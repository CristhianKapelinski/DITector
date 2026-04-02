# DITector — Large-Scale Docker Hub Security Research Pipeline

> Fork de [NSSL-SJTU/DITector](https://github.com/NSSL-SJTU/DITector), estendido para suportar crawling distribuído em larga escala, construção paralela do grafo de dependências e geração de datasets priorizados para scanning de segurança com OpenVAS.

---

## Índice

1. [Contexto e Motivação](#1-contexto-e-motivação)
2. [Arquitetura da Pipeline](#2-arquitetura-da-pipeline)
3. [Metodologia Científica (paper Dr. Docker)](#3-metodologia-científica)
4. [O que este fork modifica](#4-o-que-este-fork-modifica)
5. [Bugs corrigidos neste fork](#5-bugs-corrigidos-neste-fork)
6. [Pré-requisitos e Configuração](#6-pré-requisitos-e-configuração)
7. [Configuração do `config.yaml`](#7-configuração-do-configyaml)
8. [Estágio I — Crawling (Descoberta)](#8-estágio-i--crawling-descoberta)
9. [Estágio II — Build (Grafo IDEA)](#9-estágio-ii--build-grafo-idea)
10. [Estágio III — Rank (Priorização)](#10-estágio-iii--rank-priorização)
11. [Integração com OpenVAS](#11-integração-com-openvas)
12. [Automação da Pipeline](#12-automação-da-pipeline)
13. [Monitoramento](#13-monitoramento)
14. [Referência de Comandos](#14-referência-de-comandos)
15. [Decisões de Design e Trade-offs](#15-decisões-de-design-e-trade-offs)

---

## 1. Contexto e Motivação

Este projeto implementa a coleta e priorização de imagens Docker para scanning de segurança dinâmico com **OpenVAS**. O objetivo é selecionar ~100.000 containers do Docker Hub de forma inteligente — não aleatoriamente — priorizando imagens com:

- **Alto Pull Count** (amplamente usadas, impacto direto em usuários)
- **Alto Dependency Weight** (imagens base cujas vulnerabilidades se propagam para imagens filhas)
- **Exposição de rede** (containers com serviços de rede configurados via `EXPOSE`, candidatos ao scan OpenVAS)

A base científica é o paper **"Dr. Docker: A Large-Scale Security Measurement of Docker Image Ecosystem"** (WWW '25, Shi et al., Shanghai Jiao Tong University), que propõe o framework **DITector** para medir a segurança do ecossistema Docker em larga escala.

---

## 2. Arquitetura da Pipeline

```
┌──────────────────────────────────────────────────────────────────────────┐
│                        DITector Research Pipeline                        │
└──────────────────────────────────────────────────────────────────────────┘

  ┌──────────────┐     ┌──────────────────┐     ┌──────────────────────┐
  │  Docker Hub  │────▶│   Stage I        │────▶│     MongoDB          │
  │  V2 API      │     │   CRAWL          │     │  (repositories_data) │
  │  /v2/search/ │     │  (DFS + Workers) │     │  namespace, name,    │
  └──────────────┘     └──────────────────┘     │  pull_count          │
                                                 └──────────┬───────────┘
                                                            │
                                                 ┌──────────▼───────────┐
  ┌──────────────┐     ┌──────────────────┐     │     Stage II         │
  │  Docker Hub  │────▶│  Tag/Layer API   │────▶│     BUILD            │
  │  Tag+Image   │     │  (live API calls │     │  Network heuristic   │
  │  Metadata    │     │   during build)  │     │  filter + Neo4j IDEA │
  └──────────────┘     └──────────────────┘     └──────────┬───────────┘
                                                            │
                                                 ┌──────────▼───────────┐
                                                 │     Neo4j            │
                                                 │  (Layer IDEA graph)  │
                                                 │  IS_BASE_OF edges    │
                                                 └──────────┬───────────┘
                                                            │
                                                 ┌──────────▼───────────┐
                                                 │     Stage III        │
                                                 │     RANK             │
                                                 │  Dependency Weight   │
                                                 │  + Pull Count sort   │
                                                 └──────────┬───────────┘
                                                            │
                                                 ┌──────────▼───────────┐
                                                 │  final_prioritized_  │
                                                 │  dataset.json        │
                                                 │  (JSONL, one record  │
                                                 │   per image)         │
                                                 └──────────┬───────────┘
                                                            │
                                                 ┌──────────▼───────────┐
                                                 │  OpenVAS Scanning    │
                                                 │  (seu script externo)│
                                                 └──────────────────────┘
```

---

## 3. Metodologia Científica

O paper Dr. Docker (WWW '25) define:

### 3.1 Coleta de Dados

O Docker Hub fornece dois tipos de repositório:
- **Official images**: listadas via arquivo de índice público (`docker-library/official-images`)
- **Community images**: acessíveis pela API `GET /v2/search/repositories/?query=<keyword>`

A API aceita queries de 2–255 caracteres e retorna até **10.000 resultados** por query. Para cobrir os 12M+ repositórios, o paper implementa um **DFS keyword generator**:

```
Se count(keyword) >= 10.000 → aprofundar: enfileirar keyword+"a", keyword+"b", ..., keyword+"z", keyword+"0", ..., keyword+"9", keyword+"-", keyword+"_"
Se count(keyword) < 10.000  → scrape: pegar todas as páginas disponíveis
```

### 3.2 Construção do Grafo IDEA (Image DEpendency grAph)

O grafo modela herança entre imagens através de **Layer nodes**. Cada node representa uma camada no ponto de vista da cadeia de dependência.

**Cálculo do node ID:**

Para uma **content layer** (possui `digest`):
```
dig_i      = SHA256(layer_i.digest)
Layer_i.id = SHA256(Layer_{i-1}.id + dig_i)
```

Para uma **config layer** (só possui instrução Dockerfile, sem conteúdo de arquivo):
```
dig_i      = SHA256(layer_i.instruction)
Layer_i.id = SHA256(Layer_{i-1}.id + dig_i)
```

O node ID do **bottom layer** (i=0) é calculado com `preID = ""`:
```
Layer_0.id = SHA256("" + SHA256(layer_0.digest_or_instruction))
```

**Por que este esquema funciona:** Se duas imagens compartilham as mesmas N primeiras camadas na mesma ordem, elas compartilharão o mesmo `Layer_N.id`. Isso permite identificar upstream/downstream via grafo, sem precisar comparar todos os layers.

**Relações no grafo:**
- `(Layer)-[:IS_BASE_OF]->(Layer)` — relação de herança entre camadas
- `(Layer)-[:IS_SAME_AS]-(RawLayer)` — associação de uma posição de layer ao conteúdo físico

### 3.3 Identificação de Imagens Críticas

O paper define dois conjuntos de imagens de alto impacto:

| Tipo | Critério | Qtd no paper |
|------|----------|--------------|
| **High-Pull-Count** | Pull count ≥ 1.000.000, top 3 tags mais recentes | 20.673 imagens |
| **High-Dependency-Weight** | Dependency weight ≥ 10 (≥10 imagens dependem diretamente) | 25.924 imagens |

**Dependency Weight (Out-Degree):** número de imagens filhas que herdam desta imagem.
**Dependent Weight (In-Degree):** número de imagens das quais esta imagem depende.

### 3.4 Achados do Paper

- **93,7%** das imagens analisadas contêm vulnerabilidades conhecidas
- **4.437** imagens com secret leakage (chaves privadas, tokens de API, URIs)
- **50** imagens com misconfigurações (MongoDB, Redis, Elasticsearch, CouchDB)
- **24** imagens maliciosas (crypto miners: XMR, PKT, CRP)
- **334** imagens downstream afetadas por imagens maliciosas (propagação via supply chain)

---

## 4. O que este fork modifica

O upstream original (`NSSL-SJTU/DITector`) focava em análise de segurança de imagens já coletadas. Este fork adiciona a **pipeline de coleta e priorização** completa, necessária para o objetivo de scanning dinâmico em larga escala.

### 4.1 Novo pacote `crawler/`

**Arquivo:** `crawler/crawler.go`

Implementação nativa em Go do crawler distribuído descrito no paper. O upstream original usava Python/Scrapy; este fork usa Goroutines e Channels para máxima performance de I/O.

**Design:**
- `ParallelCrawler` gerencia N workers concorrentes via `sync.WaitGroup`
- Keywords são distribuídas via channel buffered (capacidade 1M)
- DFS: se `count >= 10.000`, aprofunda para `keyword+char` (todo o alfabeto `[a-z0-9-_]`)
- Se `count < 10.000`, coleta todas as páginas disponíveis (max 100 páginas × 100 resultados = 10.000 itens)
- Rate limit HTTP 429 tratado com backoff de 60s e **re-enfileiramento** da keyword (sem perda)

**Arquivo:** `crawler/auth_proxy.go`

`IdentityManager` centraliza autenticação e rotação de proxies:
- Carrega contas Docker Hub de `accounts.json` (array de `{username, password}`)
- Carrega proxies de arquivo texto (uma URL por linha, ex: `http://user:pass@host:port`)
- Auto-login JWT via `POST /v2/users/login/` com mutex para evitar login paralelo para a mesma conta
- `GetNextClient()` retorna `(*http.Client, token)` com proxy rotacionado round-robin

### 4.2 Novo arquivo `buildgraph/from_mongo.go`

Reengenharia do estágio `build` original para operar em **Worker Pool paralelo**:

```
MongoDB (repositories) 
    │
    ▼ loadReposToChannel (goroutine única, paginação)
    │
    ├──▶ repoWorker × NumCPU*2  (chamam API Docker Hub para tags+layers, filtro de rede)
    │                                    │
    │                                    ▼
    │                              jobChan (buffer 5000)
    │                                    │
    └──▶ buildGraphWorker × NumCPU  (inserem no Neo4j via InsertImageToNeo4j)
```

**Filtro heurístico de rede:** `repoWorker` aplica `isNetworkContainer(repo.Name)` — lista de 30+ keywords como `nginx`, `postgres`, `redis`, `proxy`, `gateway`, etc. — para descartar containers que provavelmente não expõem serviços de rede antes mesmo de chamar a API.

### 4.3 Modificação em `myutils/urls.go`

Adicionado template e função para a V2 Search API:

```go
V2SearchURLTemplate = `https://hub.docker.com/v2/search/repositories/?query=%s&page=%d&page_size=%d`

func GetV2SearchURL(query string, page, size int) string
```

O upstream usava a API antiga `content/v1/products/search` que possui limitações maiores. A V2 é a API oficial.

### 4.4 Novo `docker-compose.yml`

Infraestrutura completa para rodar a pipeline:

| Serviço | Imagem | Porta | Propósito |
|---------|--------|-------|-----------|
| `ditector_mongo` | `mongo:latest` | 27017 | Persistência de repos, tags, images |
| `ditector_neo4j` | `neo4j:latest` | 7474/7687 | Grafo IDEA de dependências |
| `ditector_crawler` | `golang:1.22` | — | Executa o crawl com seed configurável |

A variável de ambiente `SEED` permite rodar múltiplas instâncias do crawler com sementes diferentes (estratégia meet-in-the-middle):
```bash
SEED=a docker compose up -d crawler   # Máquina 1: a-m
SEED=n docker compose up -d crawler   # Máquina 2: n-z
```

### 4.5 Novos scripts de automação (`automation/`)

- `pipeline_autopilot.sh` — executa os 3 estágios sequencialmente com configuração parametrizada
- `test_e2e.sh` — teste de integração end-to-end: crawl com seed `nginx`, build, rank, verifica output

---

## 5. Bugs corrigidos neste fork

Lista completa de bugs encontrados e corrigidos (em ordem de severidade):

### Bug 1 — `myutils/neo4j.go`: Cypher query com property errada (CRÍTICO)

**Localização:** `FindSrcImgNamesByDigest` → `findLayerNodesByRawLayerDigestFunc`

**Problema:** A query usava `{id: $digest}` para matchar um nó `RawLayer`, mas os nós `RawLayer` são criados com a propriedade `digest`, não `id`. A query nunca retornaria resultados, quebrando silenciosamente toda a funcionalidade de rastreamento de imagens upstream.

```cypher
-- ANTES (errado):
MATCH (l:Layer)-[:IS_SAME_AS]-(rl:RawLayer {id: $digest})

-- DEPOIS (correto):
MATCH (l:Layer)-[:IS_SAME_AS]-(rl:RawLayer {digest: $digest})
```

### Bug 2 — `automation/test_e2e.sh`: Syntax error no shell (CRÍTICO)

**Problema:** `[ [` é inválido em bash e faz o script de teste sempre falhar.

```bash
# ANTES (erro de sintaxe):
if [ -f "test_output.json" ] && [ [ $(stat -c%s test_output.json) -gt 10 ]; then

# DEPOIS (correto):
if [ -f "test_output.json" ] && [ "$(stat -c%s test_output.json)" -gt 10 ]; then
```

### Bug 3 — `crawler/auth_proxy.go`: Proxy loading era stub (ALTO)

**Problema:** A função `LoadIdentities` lia o arquivo de proxies mas nunca o parseava. O campo `im.Proxies` permanecia sempre vazio. Nenhum proxy era usado, tornando toda a funcionalidade de rotação de IP inoperante.

**Correção:** Parse linha a linha com `strings.Split`, trim de espaços em branco, ignorando linhas vazias.

### Bug 4 — `buildgraph/from_mongo.go`: Threshold hardcoded ignora parâmetro (MÉDIO)

**Problema:** O parâmetro `pullCountThreshold` era corretamente usado no filtro MongoDB (`$gte: threshold`), mas dentro de `repoWorker` havia um segundo check hardcoded `repo.PullCount > 10000` que ignorava o threshold configurado via CLI. Isso criava comportamento inconsistente.

**Correção:** Removido o check secundário — a filtragem é responsabilidade exclusiva da query MongoDB.

### Bug 5 — `scripts/calculate_node_dependent_weights.go`: Dead code na branch `library` (MÉDIO)

**Problema:** O branch `if repoDoc.Namespace == "library"` tinha `continue` como primeira instrução, tornando todo o código abaixo (`FindAllTagsByRepoName`, etc.) inalcançável. Imagens `library` (imagens oficiais Docker) eram simplesmente ignoradas no cálculo de pesos.

**Correção:** Removido o `continue` prematuro. Imagens `library` agora fazem fetch de todos os tags (comportamento correto conforme o paper).

### Bug 6 — `crawler/crawler.go`: Keyword com 429 descartada permanentemente (BAIXO)

**Problema:** Quando uma keyword recebia status HTTP 429, o crawler aguardava 30s e depois simplesmente retornava sem re-enfileirar a keyword. A keyword era perdida definitivamente.

**Correção:** Após o backoff de 60s, a keyword é re-enfileirada via `pc.KeywordChan <- keyword`.

---

## 6. Pré-requisitos e Configuração

### Software necessário

```bash
# Go 1.21+
go version

# Docker e Docker Compose
docker --version
docker compose version
```

### Infraestrutura

Suba MongoDB e Neo4j antes de qualquer comando:

```bash
docker compose up -d mongodb neo4j
```

Aguarde ~10s para os serviços iniciarem. Verifique:

```bash
# MongoDB
mongosh localhost:27017 --eval "db.runCommand({ping: 1})"

# Neo4j
curl -s http://localhost:7474 | head -5
```

### Contas Docker Hub (necessárias para o crawl)

Crie `accounts.json` na raiz do projeto (NÃO commitar):

```json
[
  {"username": "usuario1", "password": "senha1"},
  {"username": "usuario2", "password": "senha2"}
]
```

> Contas gratuitas do Docker Hub são suficientes. Múltiplas contas aumentam o limite de rate e permitem rotação de tokens JWT.

### Proxies (opcional)

Crie `proxies.txt` na raiz (uma URL por linha):

```
http://user:pass@proxy1.example.com:8080
http://user:pass@proxy2.example.com:8080
socks5://proxy3.example.com:1080
```

---

## 7. Configuração do `config.yaml`

Copie o template e ajuste:

```bash
cp config_template.yaml config.yaml
```

Campos principais:

```yaml
max_thread: 0              # 0 = usa todos os CPUs disponíveis

log_file: "ditector.log"   # caminho relativo à raiz do projeto

mongo_config:
  uri: "mongodb://localhost:27017"
  database: "dockerhub_data"
  collections:
    repositories: "repositories_data"
    tags: "tags_data"
    images: "images_data"
    image_results: "image_results"
    layer_results: "layer_results"
    user: "user_data"

neo4j_config:
  neo4j_uri: "neo4j://localhost:7687"
  neo4j_username: "neo4j"
  neo4j_password: ""       # vazio se NEO4J_AUTH=none (docker-compose default)

proxy:
  http_proxy: ""           # deixe vazio se não usar proxy global
  https_proxy: ""
```

> **Importante:** Para o Neo4j do docker-compose (configurado com `NEO4J_AUTH=none`), deixe `neo4j_password` vazio.

---

## 8. Estágio I — Crawling (Descoberta)

### Representação de nomes de repositório no Docker Hub

O Docker Hub organiza imagens em dois níveis hierárquicos: `namespace/name`. Não existem namespaces aninhados (diferente do GitHub). A API V2 retorna o campo `repo_name` em dois formatos possíveis:

| Tipo | `repo_name` na API | Namespace real | Nome real |
|------|--------------------|----------------|-----------|
| Imagem oficial (`library`) | `"nginx"` | `library` | `nginx` |
| Imagem oficial (`library`) | `"postgres"` | `library` | `postgres` |
| Imagem community | `"cimg/postgres"` | `cimg` | `postgres` |
| Imagem community | `"redis/redis-stack"` | `redis` | `redis-stack` |

O campo `repo_owner` presente na resposta da API é **sempre vazio** (`""`) para todos os tipos de repositório — não deve ser utilizado. O `namespace` correto é extraído exclusivamente do `repo_name` via `parseRepoName()` em `crawler/crawler.go`:

```go
func parseRepoName(repoName string) (namespace, name string) {
    parts := strings.SplitN(repoName, "/", 2)
    if len(parts) == 2 {
        return parts[0], parts[1]  // community: "nginx/nginx-ingress" → ("nginx", "nginx-ingress")
    }
    return "library", repoName    // oficial: "nginx" → ("library", "nginx")
}
```

**Por que isso é crítico para o `docker pull` e o OpenVAS:**

- Imagens `library/`: o namespace pode ser omitido. `docker pull nginx` equivale a `docker pull library/nginx`.
- Imagens community: o namespace é **obrigatório**. `docker pull cimg/postgres` não funciona sem o prefixo `cimg/`. Sem ele, o Docker interpreta como `library/postgres` — imagem diferente, resultado de scan inválido.

O formato correto para gerar o nome de pull a partir do dataset exportado:

```python
ns  = record["repository_namespace"]
img = record["repository_name"]
tag = record["tag_name"]

# Para imagens library, o namespace é omitido no pull (convenção Docker)
image_ref = f"{img}:{tag}" if ns == "library" else f"{ns}/{img}:{tag}"
# docker pull nginx:latest        ← library
# docker pull cimg/postgres:15    ← community
```

**Verificação empírica:** Em amostragem de 1.000 resultados da API V2 cobrindo 10 queries distintas (`nginx`, `redis`, `postgres`, `mysql`, `debian`, `ubuntu`, `python`, `node`, `go`, `java`), nenhum `repo_name` apresentou mais de uma barra. O formato `namespace/name` é o teto estrutural do Docker Hub.

### O que faz

O crawler varre o Docker Hub usando a estratégia **DFS (Depth-First Search)** sobre o espaço de keywords, descobrindo repositórios e persistindo `namespace`, `name` e `pull_count` no MongoDB.

**Fluxo interno:**

```
seed keyword
    │
    ▼
GET /v2/search/repositories/?query=<keyword>&page=1&page_size=100
    │
    ├─ count >= 10.000? → enfileirar keyword+[a-z0-9-_] (aprofundar DFS)
    ├─ count > 0?       → scrapeAllPages: coletar todas as páginas
    └─ count == 0?      → keyword sem resultados, avançar
```

### Como executar

**Modo simples (uma máquina):**
```bash
go run main.go crawl \
  --workers 20 \
  --accounts accounts.json \
  --config config.yaml
```

**Modo acelerado (múltiplas máquinas / meet-in-the-middle):**
```bash
# Máquina 1: sementes a-m
go run main.go crawl --workers 30 --seed 'a' --accounts accounts.json --config config.yaml

# Máquina 2: sementes n-z
go run main.go crawl --workers 30 --seed 'n' --accounts accounts.json --config config.yaml
```

**Com proxies:**
```bash
go run main.go crawl --workers 20 --proxies proxies.txt --accounts accounts.json --config config.yaml
```

### Parâmetros

| Flag | Padrão | Descrição |
|------|--------|-----------|
| `--workers` / `-w` | 10 | Número de goroutines trabalhadoras paralelas |
| `--seed` | — | Keyword inicial para DFS (sem seed = começa por todo o alfabeto) |
| `--accounts` | — | Caminho para `accounts.json` |
| `--proxies` | — | Caminho para arquivo de proxies (uma URL por linha) |
| `--config` / `-c` | `config.yaml` | Caminho para o arquivo de configuração |

### Verificar progresso

```bash
# Contagem de repositórios descobertos
mongosh localhost:27017/dockerhub_data --eval 'db.repositories_data.countDocuments()'

# Acompanhar descobertas em tempo real
tail -f *.log | grep "Discovered repository"

# Top 10 por pull_count
mongosh localhost:27017/dockerhub_data --eval '
  db.repositories_data.find({}, {name:1, pull_count:1, _id:0})
    .sort({pull_count: -1}).limit(10).pretty()
'
```

### Volume esperado

Com 1 máquina e 20 workers rodando por 24h, espera-se descobrir entre 500.000 e 2.000.000 repositórios, dependendo da velocidade da conexão e dos rate limits. O Docker Hub contém 12M+ repositórios no total.

---

## 9. Estágio II — Build (Grafo IDEA)

### O que faz

Para cada repositório no MongoDB com `pull_count >= threshold`:

1. Aplica **filtro heurístico de rede** — descarta repositórios cujo nome não contém keywords de serviços de rede (nginx, postgres, redis, api, etc.)
2. Chama a API Docker Hub para buscar as N tags mais recentes do repositório
3. Para cada tag, busca os metadados de imagem (layers: digest, instruction, size)
4. Filtra imagens Windows
5. Insere no **Neo4j** o grafo IDEA com o algoritmo de hashing das seções 3.2

**Por que o filtro de rede aqui:** Containers sem serviços de rede não são candidatos ao scan OpenVAS. Filtrá-los no estágio II evita construir um grafo desnecessariamente grande e reduz drasticamente o tempo do estágio III.

### Como executar

```bash
go run main.go build \
  --format mongo \
  --threshold 1000 \
  --page_size 50 \
  --tags 3 \
  --config config.yaml
```

### Parâmetros

| Flag | Padrão | Descrição |
|------|--------|-----------|
| `--format` | `mongo` | Fonte de dados (só `mongo` suportado atualmente) |
| `--threshold` | 1.000.000 | Pull count mínimo para processar um repositório |
| `--page` | 1 | Página inicial para retomar processamento interrompido |
| `--page_size` | 5 | Repositórios processados por lote (página MongoDB) |
| `--tags` | 10 | Número de tags mais recentes a processar por repositório |

**Recomendações para pesquisa:**
- `--threshold 1000` — cobre a maior parte dos repos com atividade real
- `--tags 3` — as 3 tags mais recentes são suficientes para detecção (alinhado com o paper)
- `--page_size 50` — tamanho de lote adequado para a maioria das máquinas

### Verificar progresso

```bash
# Nodes no Neo4j
cypher-shell -u neo4j -p "" "MATCH (l:Layer) RETURN count(l) AS total_layers"

# Edges no Neo4j
cypher-shell -u neo4j -p "" "MATCH ()-[r:IS_BASE_OF]->() RETURN count(r) AS total_edges"
```

---

## 10. Estágio III — Rank (Priorização)

### O que faz

Para cada imagem processada no grafo Neo4j, calcula o **Dependency Weight** (Out-Degree no IDEA) — número de imagens downstream que herdam desta imagem — e exporta um arquivo JSONL com os resultados.

**Schema de saída (um JSON por linha):**

```json
{
  "repository_namespace": "library",
  "repository_name": "nginx",
  "tag_name": "latest",
  "image_digest": "sha256:abc123...",
  "weights": 1847,
  "downstream_images": ["user1/app:latest", "user2/service:v2", ...]
}
```

### Como executar

```bash
go run main.go execute \
  --script calculate-node-weights \
  --threshold 1000 \
  --file final_prioritized_dataset.json \
  --config config.yaml
```

### Pós-processamento para OpenVAS

Ordene por dependency weight (descrescente) e pull count para priorização:

```bash
# Top 100 por dependency weight
jq -s 'sort_by(-.weights) | .[0:100]' final_prioritized_dataset.json

# Extrair nomes de imagem para scanning
jq -r '"\(.repository_namespace)/\(.repository_name):\(.tag_name)"' final_prioritized_dataset.json \
  | sort -u \
  > images_for_openvas.txt
```

---

## 11. Integração com OpenVAS

O objetivo final da pipeline é alimentar um scanner OpenVAS com containers de rede. O fluxo é:

```
images_for_openvas.txt
        │
        ▼
[seu script de scanning]
  1. docker pull <image>
  2. docker run -d --name scan_target <image>
  3. docker inspect scan_target → pegar IP do container
  4. openvas-cli --target <IP> --scan-config "Full and Fast"
  5. coletar relatório
  6. docker rm -f scan_target
  7. próxima imagem
```

**Por que containers que não são de rede falham:** Se o container não expõe portas ou não roda um daemon de rede, o OpenVAS simplesmente não encontrará serviços. Seu script externo já trata isso: tenta o setup e, em caso de falha, avança para o próximo. O filtro heurístico desta pipeline reduz drasticamente o número de tentativas falhas.

**Heurística de rede atual** (keywords no nome do repositório):
```
nginx, apache, http, https, server, web, api, rest, grpc, db, database,
mysql, postgres, sql, redis, mongo, elastic, kafka, rabbitmq, proxy,
gateway, lb, balancer, vpn, ssh, ftp, smtp, imap, ldap, app, service, svc
```

> Esta lista é conservadora por design. Containers que passam na heurística têm alta probabilidade de expor algum serviço de rede. Containers sem keywords no nome ainda podem ser candidatos — seu script de scanning trata esses casos com a lógica de fallback.

---

## 12. Automação da Pipeline

### Pipeline Autopilot

Executa os 3 estágios sequencialmente:

```bash
./automation/pipeline_autopilot.sh "a"
```

Configurações no próprio script:

```bash
WORKERS=20          # workers de crawl
CRAWL_DURATION="30s" # tempo de crawl (ajuste para pesquisa real: "6h", "24h")
PULL_THRESHOLD=1000  # pull count mínimo
OUTPUT_FILE="final_prioritized_dataset.json"
```

### Teste de Integração E2E

Valida que toda a pipeline funciona end-to-end com dados reais (seed `nginx`):

```bash
chmod +x automation/test_e2e.sh
./automation/test_e2e.sh
```

O que o teste verifica:
1. Crawl com seed `nginx` por 20s → descobre repositórios relacionados a nginx
2. Build com threshold=0 → processa todos os repositórios descobertos
3. Rank → gera `test_output.json`
4. Verifica que `test_output.json` existe e tem tamanho > 10 bytes

---

## 13. Monitoramento

### MongoDB

```bash
# Total de repositórios descobertos
mongosh localhost:27017/dockerhub_data --eval \
  'db.repositories_data.countDocuments()'

# Repositórios com pull_count >= 1M
mongosh localhost:27017/dockerhub_data --eval \
  'db.repositories_data.countDocuments({pull_count: {$gte: 1000000}})'

# Top 20 repos por pull count
mongosh localhost:27017/dockerhub_data --eval \
  'db.repositories_data.find({},{name:1,namespace:1,pull_count:1,_id:0}).sort({pull_count:-1}).limit(20)'
```

### Neo4j (Browser em http://localhost:7474)

```cypher
// Total de nodes Layer
MATCH (l:Layer) RETURN count(l)

// Total de edges IS_BASE_OF (arestas de dependência)
MATCH ()-[r:IS_BASE_OF]->() RETURN count(r)

// As 10 imagens com mais dependentes
MATCH (l:Layer)-[:IS_BASE_OF*]->(down:Layer)
WHERE size(l.images) > 0
RETURN l.images[0] AS image, count(down) AS downstream
ORDER BY downstream DESC LIMIT 10

// Verificar propagação de ameaças: downstream de nginx:latest
MATCH (src:Layer {id: '<node_id_do_nginx>'})
MATCH (src)-[:IS_BASE_OF*]->(down:Layer)
WHERE size(down.images) > 0
RETURN down.images
```

### Logs

```bash
# Descobertas em tempo real
tail -f *.log | grep "Discovered repository"

# Erros de build
tail -f *.log | grep "ERROR"

# Taxa de inserção no Neo4j
tail -f *.log | grep "Inserido no Neo4j" | wc -l
```

---

## 14. Referência de Comandos

### Subcomandos disponíveis

```
docker-scan crawl      — Fase I: descoberta de repositórios
docker-scan build      — Fase II: construção do grafo IDEA
docker-scan analyze    — Análise de segurança de uma imagem específica
docker-scan execute    — Executa scripts de processamento em lote
docker-scan calculate  — Calcula o node ID de uma imagem pelo digest
```

### Flags globais

| Flag | Padrão | Descrição |
|------|--------|-----------|
| `--config` / `-c` | `config.yaml` | Arquivo de configuração |
| `--log_level` / `-l` | `debug` | Nível de log: debug, info, warn, error, critical |

### `execute --script`

| Script | Descrição |
|--------|-----------|
| `calculate-node-weights` | Calcula Dependency Weight de cada imagem e exporta JSONL |
| `analyze-threshold` | Analisa imagens com pull_count acima de threshold |
| `analyze-all` | Analisa todas as imagens no MongoDB |
| `count-images-with-upstream` | Conta imagens com upstream (In-Degree > 0) |
| `count-images-with-downstream` | Conta imagens com downstream (Out-Degree > 0) |
| `export-mongo-result-docs` | Exporta resultados de análise do MongoDB para JSON |
| `check-same-node-as-high-dependent-images` | Identifica interseções entre conjuntos high-PC e high-DW |

---

## 15. Decisões de Design e Trade-offs

### Por que Go em vez de Python/Scrapy (como o upstream)?

O upstream original usa Python/Scrapy para o crawler. Este fork reimplementou em Go por:
- **Goroutines vs threads**: Go escala para centenas de workers de I/O com custo de memória mínimo (~2KB/goroutine vs ~1MB/thread OS)
- **Channels**: comunicação entre stages é type-safe e não requer locks manuais
- **Único binário**: sem dependências de runtime, deploy trivial em múltiplas máquinas

### Por que o Build chama a API live em vez de ler do MongoDB?

O crawler (Estágio I) armazena apenas `namespace`, `name` e `pull_count`. Tags e layers são buscados no Estágio II via API live. Isso é um trade-off deliberado:
- **Prós**: Volume de dados no MongoDB é menor; o crawler é mais rápido
- **Contras**: O build stage é dependente de disponibilidade da API; repositórios deletados entre crawl e build geram erros

Alternativa (não implementada): o crawler poderia armazenar tags/layers diretamente, tornando o build stage totalmente offline.

### Por que filtro heurístico por nome e não por `EXPOSE` do Dockerfile?

O campo `EXPOSE` só é acessível após baixar a imagem (análise de conteúdo). O filtro por nome de repositório funciona em metadados já coletados, sem custo adicional de download. Para a finalidade de scan com OpenVAS, o filtro por nome é suficientemente preciso como etapa de pré-seleção.

### Limitações conhecidas

1. **JWT expiry**: Tokens JWT do Docker Hub expirem (~24h). Não há lógica de refresh automático. Em runs longas, os tokens precisam ser renovados manualmente reiniciando o crawler.
2. **Build live API**: Se um repositório for deletado entre o crawl e o build, erros são logados mas não impedem o progresso.
3. **Throughput do Neo4j**: A inserção no Neo4j é síncrona por imagem. Para volumes muito grandes (>1M imagens), considere aumentar `NEO4J_dbms_memory_heap_initial__size`.

---

*Baseado no paper: Hequan Shi et al., "Dr. Docker: A Large-Scale Security Measurement of Docker Image Ecosystem", WWW '25.*
