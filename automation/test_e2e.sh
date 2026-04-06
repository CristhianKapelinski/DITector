#!/bin/bash
# End-to-End Integration Test — validates the full 3-stage pipeline with a
# minimal real crawl using an isolated test database.
#
# Arquitetura de rede (sem "expose"):
#   - Este script e o binário Go rodam diretamente no host.
#   - MongoDB roda nativo em 127.0.0.1:27017 (sem container).
#   - Neo4j roda (ou é iniciado) em Docker com port-map localhost:7687.
#   - O binário de teste usa o banco 'dockerhub_e2e' (isolado do crawler em prod).
#
# Tempo esperado: ~60 s (+ até 60 s de startup do Neo4j na primeira vez)

set -euo pipefail

# ── config ────────────────────────────────────────────────────────────────────
SEED="nginx"
THRESHOLD="100000000"   # 100 M+ pulls → limita Stage II a ~2 repos
WORKERS="3"
CRAWL_TIMEOUT="20"
BUILD_TIMEOUT="60"
OUTPUT="/tmp/ditector_e2e_output.json"
BINARY="/tmp/ditector_e2e"
E2E_DB="dockerhub_e2e"                   # banco ISOLADO — não conflita com o crawler
E2E_CONFIG="/tmp/ditector_e2e_config.yaml"

_step() { echo ""; echo "[${1}] ${2}"; }
_ok()   { echo "      ✓ ${1}"; }
_fail() { echo "      ✗ ${1}"; exit 1; }

echo "=== DITector E2E Test  seed='${SEED}'  threshold=${THRESHOLD} ==="

# Monta config temporário apontando para o banco de teste isolado.
# Lê a config real só para herdar caminhos de regras/contas; substitui o DB.
DB_LINE=$(grep -A2 'mongo_config:' config.yaml | grep 'uri:' | sed "s/'[^']*'/'mongodb:\/\/localhost:27017'/" || echo "  uri: 'mongodb://localhost:27017'")
cat > "$E2E_CONFIG" <<YAML
max_thread: 0
log_file: '/tmp/ditector_e2e'
repo_with_many_tags_file: '/tmp/ditector_e2e_repos.txt'
tmp_dir: '/tmp'
proxy:
  http_proxy: ''
  https_proxy: ''
mongo_config:
  uri: 'mongodb://localhost:27017'
  database: '${E2E_DB}'
  collections:
    repositories: 'repositories_data'
    tags: 'tags_data'
    images: 'images_data'
    image_results: 'image_results'
    layer_results: 'layer_results'
    user: 'user'
neo4j_config:
  neo4j_uri: 'neo4j://localhost:7687'
  neo4j_username: 'neo4j'
  neo4j_password: ''
rules_config:
  secret_rules_file: 'rules/secret_rules.yaml'
  sensitive_param_rules_file: 'rules/sensitive_param_rules.yaml'
trufflehog_config:
  filepath: ''
  verify: false
anchore_config:
  filepath: ''
YAML

# ── 0. prerequisites ──────────────────────────────────────────────────────────
_step "0/4" "Prerequisites..."

[ -f accounts.json ] || _fail "accounts.json not found in $(pwd)"

mongosh --eval "db.runCommand({ping:1})" --quiet &>/dev/null \
    || _fail "MongoDB não acessível em localhost:27017"
_ok "MongoDB up (nativo)"

if ! nc -z localhost 7687 &>/dev/null 2>&1; then
    echo "      Neo4j não está rodando — iniciando container..."
    docker compose --profile db up -d neo4j 2>&1 | grep -v "^time=" || true
    echo -n "      Aguardando porta 7687 (até 60 s)"
    for _ in $(seq 1 30); do
        sleep 2
        nc -z localhost 7687 &>/dev/null && break
        echo -n "."
    done
    echo ""
    nc -z localhost 7687 &>/dev/null || _fail "Neo4j não ficou pronto em 60 s"
fi
_ok "Neo4j up (docker, localhost:7687)"

# Limpa o banco de teste isolado
mongosh "localhost:27017/${E2E_DB}" --quiet \
    --eval "db.dropDatabase();" &>/dev/null || true
rm -f "$OUTPUT"
_ok "Banco '${E2E_DB}' limpo"

# ── compila ───────────────────────────────────────────────────────────────────
echo ""
echo "[pre] Compilando..."
go build -o "$BINARY" . 2>&1 || _fail "go build falhou"
_ok "Binário: $BINARY"

# ── 1. Stage I — Crawl ───────────────────────────────────────────────────────
_step "1/4" "CRAWL  seed='${SEED}'  workers=${WORKERS}  timeout=${CRAWL_TIMEOUT}s..."

timeout "${CRAWL_TIMEOUT}s" "$BINARY" crawl \
    --workers  "$WORKERS" \
    --seed     "$SEED" \
    --accounts accounts.json \
    --config   "$E2E_CONFIG" &>/dev/null || true   # saída por timeout é esperada

REPO_COUNT=$(mongosh "localhost:27017/${E2E_DB}" --quiet \
    --eval 'db.repositories_data.countDocuments()' 2>/dev/null || echo 0)
_ok "Repos descobertos: $REPO_COUNT"
[ "$REPO_COUNT" -gt 0 ] || _fail "Nenhum repo descoberto — verifique accounts.json e rede"

NS_OK=$(mongosh "localhost:27017/${E2E_DB}" --quiet \
    --eval 'db.repositories_data.countDocuments({namespace:{$ne:""}})' 2>/dev/null || echo 0)
_ok "Com namespace correto: $NS_OK / $REPO_COUNT"

# ── 2. Stage II — Build ───────────────────────────────────────────────────────
_step "2/4" "BUILD  threshold=${THRESHOLD}  timeout=${BUILD_TIMEOUT}s..."

timeout "${BUILD_TIMEOUT}s" "$BINARY" build \
    --format    mongo \
    --threshold "$THRESHOLD" \
    --accounts  accounts.json \
    --config    "$E2E_CONFIG" \
    --data_dir  /tmp &>/dev/null || true

BUILT=$(mongosh "localhost:27017/${E2E_DB}" --quiet \
    --eval 'db.repositories_data.countDocuments({graph_built_at:{$exists:true}})' 2>/dev/null || echo 0)
TAG_COUNT=$(mongosh "localhost:27017/${E2E_DB}" --quiet \
    --eval 'db.tags_data.countDocuments()' 2>/dev/null || echo 0)
IMG_COUNT=$(mongosh "localhost:27017/${E2E_DB}" --quiet \
    --eval 'db.images_data.countDocuments()' 2>/dev/null || echo 0)

_ok "Repos com graph_built_at: $BUILT"
_ok "Tags salvas no MongoDB: $TAG_COUNT"
_ok "Images salvas no MongoDB: $IMG_COUNT"
[ "$BUILT"     -gt 0 ] || _fail "Nenhum repo built — verifique conexão Neo4j e accounts.json"
[ "$TAG_COUNT" -gt 0 ] || _fail "Nenhuma tag salva — regressão no getTags fix #1"
[ "$IMG_COUNT" -gt 0 ] || _fail "Nenhuma image salva — regressão no persistImages"

# Verifica que os metadados ricos do ImageInTag foram preservados (fix #2):
# ao menos um tag deve ter o campo images.status preenchido.
STATUS_OK=$(mongosh "localhost:27017/${E2E_DB}" --quiet \
    --eval 'db.tags_data.countDocuments({"images.status":{$exists:true,$ne:""}})' 2>/dev/null || echo 0)
_ok "Tags com images.status preservado: $STATUS_OK"
[ "$STATUS_OK" -gt 0 ] || _fail "images.status vazio — regressão no overwrite de ImageInTag (fix #2)"

# ── 3. Stage III — Rank ───────────────────────────────────────────────────────
_step "3/4" "RANK  threshold=${THRESHOLD}  page_size=3..."

"$BINARY" execute \
    --script    calculate-node-weights \
    --threshold "$THRESHOLD" \
    --page_size 3 \
    --file      "$OUTPUT" \
    --config    "$E2E_CONFIG" &>/dev/null || _fail "calculate-node-weights falhou"

[ -f "$OUTPUT" ] || _fail "Arquivo de saída não gerado: $OUTPUT"
RECORD_COUNT=$(wc -l < "$OUTPUT")
_ok "Saída: $OUTPUT  ($RECORD_COUNT registros)"
[ "$RECORD_COUNT" -gt 0 ] || _fail "Arquivo de saída está vazio"

# ── 4. Verifica saída ─────────────────────────────────────────────────────────
_step "4/4" "Verificando saída..."

NS_IN_OUTPUT=$(grep -c '"repository_namespace":"[^"]' "$OUTPUT" 2>/dev/null || echo 0)
_ok "Registros com repository_namespace: $NS_IN_OUTPUT / $RECORD_COUNT"
[ "$NS_IN_OUTPUT" -gt 0 ] || _fail "Nenhum registro com namespace no output"

echo ""
echo "      Amostra (primeiro registro):"
head -n 1 "$OUTPUT" | python3 -m json.tool 2>/dev/null || head -n 1 "$OUTPUT"
echo ""

# Limpa banco de teste ao fim
mongosh "localhost:27017/${E2E_DB}" --quiet --eval "db.dropDatabase();" &>/dev/null || true
rm -f "$E2E_CONFIG" "$OUTPUT"

echo "=== RESULTADO: PASSOU ==="
