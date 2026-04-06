# Fase II — Construção do Grafo de Dependências

Este documento descreve em detalhes a arquitetura, o funcionamento interno e os mecanismos de suporte da Fase II do DITector. A Fase II consome os repositórios coletados pela Fase I no MongoDB, busca metadados de tags e imagens no Docker Hub e insere a cadeia de camadas de cada imagem no Neo4j como um grafo de dependências.

---

## Índice

1. [Visão geral do pipeline](#1-visão-geral-do-pipeline)
2. [Concorrência e dimensionamento de workers](#2-concorrência-e-dimensionamento-de-workers)
3. [Reivindicação atômica de repositórios](#3-reivindicação-atômica-de-repositórios)
4. [Estratégia de seleção de tags](#4-estratégia-de-seleção-de-tags)
5. [Camada de cache MongoDB](#5-camada-de-cache-mongodb)
6. [Modelo de grafo no Neo4j](#6-modelo-de-grafo-no-neo4j)
7. [Cálculo do ID de camada (cadeia SHA256)](#7-cálculo-do-id-de-camada-cadeia-sha256)
8. [Inserção no Neo4j: transação única por imagem](#8-inserção-no-neo4j-transação-única-por-imagem)
9. [Checkpoint e retomada após crash](#9-checkpoint-e-retomada-após-crash)
10. [Métricas e interpretação do log](#10-métricas-e-interpretação-do-log)
11. [Índices MongoDB para a Fase II](#11-índices-mongodb-para-a-fase-ii)
12. [Autenticação, rate limiting e jitter](#12-autenticação-rate-limiting-e-jitter)
13. [Operação multi-nó](#13-operação-multi-nó)
14. [Consultas Cypher úteis](#14-consultas-cypher-úteis)

---

## 1. Visão geral do pipeline

```
MongoDB (repositories_data)
        │  ClaimNextBuildRepo() — atômico, pull_count DESC
        ▼
  repoWorker (N workers, controlado por --workers)
        │  getTags()      → tags API (ou cache MongoDB)
        │  getImages()    → images API (ou cache MongoDB)
        ▼
  batchChan (buffer 10 000)
        │
        ▼
  graphWorker (2×NumCPU, mínimo 8)
        │  InsertBatch()
        │  uma sessão Neo4j por batch, uma transação por imagem
        ▼
  Neo4j (Layer, RawLayer, IS_BASE_OF, IS_SAME_AS)
        │
        ▼
  MongoDB: MarkRepoGraphBuilt() — seta graph_built_at
  build_checkpoint.jsonl — append-only por repo concluído
  build_metrics.log      — snapshot a cada 60 s
```

O pipeline é bifurcado: `repoWorker` é limitado pela taxa de requisições ao Docker Hub, enquanto `graphWorker` é limitado pela latência do Neo4j. O canal `jobChan` desacopla os dois estágios para que nenhum bloqueie o outro.

---

## 2. Concorrência e dimensionamento de workers

### repoWorkers (busca de dados)

O número de `repoWorker` é controlado pela flag `--workers` (padrão: 1). Cada worker usa um `HubClient` exclusivo com identidade própria. O número recomendado é igual ao número de contas Docker Hub disponíveis — um worker por conta evita colisões de token entre goroutines.

```
numRepo = --workers   # flag em cmd/cmd.go; default 1
```

Com 6 contas e `--workers 6`, há 6 workers de repositório em paralelo. Cada um obtém ~1,8 req/s em média (limitação do Hub por conta), totalizando ~10,8 req/s de tags + imagens.

### graphWorkers (inserção no Neo4j)

```go
numGraph := runtime.NumCPU() * 2
if numGraph < 8 {
    numGraph = 8
}
```

O número mínimo é 8 para garantir utilização adequada em máquinas com poucos núcleos. Em máquinas com 4 núcleos, serão 8 workers de grafo; em máquinas com 16 núcleos, serão 32.

---

## 3. Reivindicação atômica de repositórios

`ClaimNextBuildRepo` usa `findAndModify` com filtro e update atômicos:

```
Filtro:
  pull_count  ≥ threshold
  graph_built_at  = null        (ainda não processado)
  build_claimed   ≠ true        (não está sendo processado agora)

Update:
  SET build_claimed = true
  SET build_started_at = now()
```

A operação é atômica no MongoDB: dois workers nunca reivindicam o mesmo repositório simultaneamente. Repositórios são ordenados por `pull_count` descendente, ou seja, os mais populares são processados primeiro.

### Recuperação de claims travados

Na inicialização, antes de qualquer trabalho, a Fase II chama `ResetStaleBuildClaims`:

```
Filtro:  build_claimed = true  AND  graph_built_at = null
Update:  UNSET build_claimed,  UNSET build_started_at
```

Isso recupera repositórios que tinham `build_claimed = true` mas o worker morreu antes de concluir. Eles voltam ao pool de pendentes e serão reprocessados normalmente. O repositório só sai definitivamente do pool quando `graph_built_at` é setado, o que acontece apenas após a inserção no Neo4j ser confirmada e `MarkRepoGraphBuilt` retornar sem erro.

---

## 4. Estratégia de seleção de tags

A Fase II não processa todas as tags de um repositório — isso seria inviável para repositórios como `library/ubuntu` com centenas de tags históricas. A estratégia é:

1. **Tag mais recente**: busca a página 1 com `page_size=1` ordenada por `last_updated` DESC. Retorna sempre a tag modificada mais recentemente.
2. **Tag `latest`**: buscada explicitamente via `GET /v2/repositories/{ns}/{name}/tags/latest`. Representa o que os usuários obtêm com `docker pull {image}` sem especificar tag.
3. **Deduplicação**: se a tag mais recente já for `latest`, não são feitas duas chamadas. Se forem diferentes, ambas são processadas.

```go
recent, _ := hub.GetTags(ns, name, 1, 1)        // page 1, size 1 → mais recente
tags := recent
if len(recent) == 0 || recent[0].Name != "latest" {
    latest, _ := hub.GetTag(ns, name, "latest")  // busca explícita
    if latest != nil {
        tags = append(tags, latest)
    }
}
```

**Justificativa**: A tag mais recente captura o estado atual de desenvolvimento (e.g., `ubuntu:24.04`). A tag `latest` captura o que a maioria dos usuários instala por padrão. Juntas, cobrem os dois vetores de ataque mais relevantes para análise de segurança sem explodir o volume de dados.

---

## 5. Camada de cache MongoDB

Todas as respostas da API do Docker Hub são persistidas no MongoDB para evitar chamadas redundantes em execuções posteriores (ou em caso de retomada).

### Cache de tags

Coleção: `tags_data`

Antes de chamar a API, `getTags` verifica se o repositório já tem tags salvas com imagens:

```go
tags, err := mongo.FindAllTagsByRepoName(ns, name)
if err == nil && allTagsHaveImages(tags) {
    m.TagCacheHits.Add(1)
    return tags  // cache hit
}
// cache miss → chama API
```

A validação `allTagsHaveImages` garante que o cache só é usado se todas as tags tiverem a lista `images` preenchida (digest + metadados de arquitetura). Tags salvas sem imagens foram provavelmente inseridas pela Fase I com dados incompletos.

### Cache de imagens

Coleção: `images_data`

A struct `Tag` contém `Images []ImageInTag` com os digests de cada imagem por arquitetura. `getImages` usa esses digests para fazer lookup no MongoDB:

```go
if len(t.Images) > 0 {
    imgs, ok := loadImagesFromCache(t.Images)
    if ok {
        m.ImageCacheHits.Add(1)
        return imgs, nil
    }
}
// cache miss → chama API /tags/{tag}/images
```

`loadImagesFromCache` busca todos os digests via `FindImagesByDigests` e valida que cada imagem tem `Layers` preenchido. Se qualquer imagem estiver faltando ou com layers vazios, vai para a API.

### Persistência após API

Após cada chamada à API, as respostas são enfileiradas para persistência assíncrona via `writesCh chan func()`. Uma goroutine dedicada drena o canal em background:

```go
// repoWorker nunca bloqueia em writes de cache:
writesCh <- func() { mongo.UpdateImage(img) }
writesCh <- func() { mongo.UpdateTag(t) }
```

A persistência é assíncrona pois os writes são apenas cache — uma falha (crash antes de drenar o canal) causa apenas um cache miss na próxima execução, sem impacto na integridade do grafo Neo4j ou no mecanismo de retomada (`graph_built_at`).

Isso constrói o cache progressivamente: execuções subsequentes com threshold diferente ou em caso de recuperação de falhas aproveitam os dados já buscados.

---

## 6. Modelo de grafo no Neo4j

### Nó `Layer` — contexto-dependente

Um nó `Layer` representa **uma camada em um contexto específico de construção**. O mesmo digest de filesystem pode gerar nós `Layer` diferentes dependendo da história de camadas que o precede.

Propriedades:

| Propriedade   | Tipo       | Descrição |
|---------------|------------|-----------|
| `id`          | string     | SHA256 encadeado da cadeia de camadas até este ponto (ver seção 7) |
| `digest`      | string     | Digest da camada de filesystem (sha256:...), vazio para instruções de configuração |
| `size`        | int64      | Tamanho da camada em bytes |
| `instruction` | string     | Instrução Dockerfile que gerou esta camada (e.g., `RUN apt-get install nginx`, `EXPOSE 80/tcp`, `CMD ["/bin/bash"]`) |
| `images`      | []string   | Lista de imagens que terminam neste nó (formato: `registry/ns/repo:tag@digest`) |

O campo `images` só é preenchido na **última camada** de cada imagem inserida (via `addImageToLayerFunc`). Para camadas intermediárias, `images` fica como lista vazia.

### Nó `RawLayer` — conteúdo-endereçável

Um nó `RawLayer` representa o **conteúdo físico de uma camada** independente de contexto. Dois nós `Layer` com o mesmo `digest` apontam para o mesmo `RawLayer`.

Propriedades:

| Propriedade   | Tipo   | Descrição |
|---------------|--------|-----------|
| `digest`      | string | Digest SHA256 da camada (chave primária, único no grafo) |
| `size`        | int64  | Tamanho em bytes |
| `instruction` | string | Instrução Dockerfile associada |

**Importante**: `RawLayer` só é criado para camadas com `digest` preenchido (camadas de filesystem com `tar.gz`). Camadas de configuração pura — como `EXPOSE`, `CMD`, `ENV`, `LABEL` — têm `digest == ""` e geram apenas nós `Layer`, sem `RawLayer` correspondente.

### Relacionamento `IS_BASE_OF`

```
(Layer A) -[:IS_BASE_OF]-> (Layer B)
```

Indica que a camada A é base da camada B na cadeia de construção. A relação é direcionada da base para o topo: camada raiz → ... → camada folha.

Para reconstruir a ordem de construção de uma imagem, traversa-se de A até o nó com `images` não vazio seguindo `IS_BASE_OF`.

### Relacionamento `IS_SAME_AS`

```
(Layer) -[:IS_SAME_AS]- (RawLayer)
```

Relaciona um contexto específico ao conteúdo físico. É não-direcionado (bidirecional no grafo). Permite responder: "em quais contextos de construção este filesystem layer foi usado?"

### Diagrama de exemplo

Para uma imagem com 3 camadas (FROM ubuntu, RUN apt install, EXPOSE 80):

```
(RawLayer digest=sha256:abc)
        |
   IS_SAME_AS
        |
(Layer id=H1, digest=sha256:abc, instruction="FROM ubuntu:22.04")
        |
   IS_BASE_OF
        |
(Layer id=H2, digest=sha256:def, instruction="RUN apt-get install -y nginx")
        |   \
   IS_BASE_OF  IS_SAME_AS
        |         \
        |     (RawLayer digest=sha256:def)
        |
(Layer id=H3, digest="", instruction="EXPOSE 80/tcp", images=["docker.io/library/nginx:latest@sha256:..."])
```

---

## 7. Cálculo do ID de camada (cadeia SHA256)

O `id` de um nó `Layer` é uma função da **cadeia completa de camadas** até aquele ponto. É calculado localmente (sem I/O) antes de qualquer chamada ao Neo4j:

```
dig_i = SHA256(layer.digest)        se layer.digest != ""
dig_i = SHA256(layer.instruction)   se layer.digest == ""

id_0 = SHA256("" + dig_0)           # primeira camada: prev_id = ""
id_1 = SHA256(id_0 + dig_1)
id_2 = SHA256(id_1 + dig_2)
...
id_N = SHA256(id_{N-1} + dig_N)     # id da última camada = ID da imagem
```

**Consequência direta**: duas imagens que compartilham as mesmas N primeiras camadas terão IDs iguais para essas camadas. O Neo4j usará `MERGE` e não duplicará esses nós — as cadeias são conectadas ao grafo existente. Isso é o mecanismo que torna o grafo um DAG (grafo acíclico dirigido) de dependências reais, não uma floresta de árvores independentes.

**Por que SHA256 duplo?** O `dig_i` é `SHA256(digest_string)`, não o digest raw. Isso normaliza entradas de comprimentos diferentes (digests de 64 chars vs instruções longas) e evita colisões de concatenação.

---

## 8. Inserção no Neo4j: transação única por imagem

Toda a cadeia de camadas de uma imagem é inserida em **uma única transação** no Neo4j:

```
BEGIN
  MERGE Layer_0 + IS_SAME_AS RawLayer_0
  MERGE Layer_1 + IS_SAME_AS RawLayer_1 + IS_BASE_OF(Layer_0 → Layer_1)
  ...
  MERGE Layer_N + IS_SAME_AS RawLayer_N + IS_BASE_OF(Layer_{N-1} → Layer_N)
  MATCH Layer_N SET Layer_N.images += [imageName]
COMMIT
```

Isso reduz a latência de O(N) round-trips para O(1) por imagem. Para uma imagem típica com 10 camadas, a latência passa de ~100ms (10 × 10ms) para ~15ms (1 round-trip).

O Cypher usa `MERGE` em vez de `CREATE`, o que garante idempotência: reinserir a mesma imagem duas vezes não duplica nós nem relacionamentos.

`InsertBatch` reutiliza uma única sessão Neo4j para todas as imagens do batch (uma por repo), criando uma nova sessão apenas por batch — não por imagem. Imagens com zero camadas após o cálculo de IDs são ignoradas silenciosamente.

---

## 9. Checkpoint e retomada após crash

### build_checkpoint.jsonl

Arquivo append-only no `data_dir` (montado em `/app` no container). Cada linha é um JSON:

```json
{"ns":"library","name":"ubuntu","built_at":"2025-04-05T14:23:01Z","tags":2}
```

Escrito por uma goroutine única (`checkpointWriter`) consumindo um canal — sem mutex, sem contenção. O arquivo sobrevive a reinicializações do container porque está no volume host-mounted.

### Retomada após crash

Ao reiniciar, a Fase II não relê o `build_checkpoint.jsonl`. A retomada é feita via MongoDB:

1. `ResetStaleBuildClaims` libera repos com `build_claimed=true` e `graph_built_at=null` (worker morreu no meio do trabalho).
2. `ClaimNextBuildRepo` só retorna repos com `graph_built_at=null`. Repos marcados como concluídos são automaticamente ignorados.

O checkpoint JSONL serve como auditoria externa e para estatísticas post-mortem, não como mecanismo de controle de fluxo.

### build_metrics.log

Arquivo append-only no `data_dir`. Snapshot a cada 60 segundos. Também escrito no log estruturado do processo.

---

## 10. Métricas e interpretação do log

Formato da linha de métricas:

```
[METRICS 14:23:01] progresso=1234/50000 (2.5%) | taxa=18.3 repos/min | ETA=44h0m0s | cache tags=72% imgs=85% | neo4j=9870 | erros=3 | uptime=1h7m23s
```

| Campo | Significado |
|-------|-------------|
| `progresso=A/B` | A = repos concluídos nesta execução; B = total pendente ao iniciar (capturado uma vez na inicialização) |
| `taxa` | repos por minuto desde o início desta execução |
| `ETA` | estimativa linear baseada na taxa atual e no total restante |
| `cache tags` | % de repositórios cujas tags foram lidas do MongoDB em vez da API |
| `cache imgs` | % de imagens cujos layers foram lidos do MongoDB em vez da API |
| `neo4j` | total de imagens inseridas no Neo4j nesta execução |
| `erros` | total de erros não-fatais (getImages falhou, tag não encontrada, etc.) |
| `uptime` | tempo desde a inicialização do processo |

**Nota sobre `progresso=X/0`**: o total `B` é calculado por `CountPendingBuildRepos` com timeout de 30s na inicialização. Se a query demorar mais que 30s (raro, mas possível sob carga), B fica em 0 e o ETA fica indefinido. Isso não afeta o processamento.

A linha `[FINAL]` é emitida quando todos os workers terminam (queue vazia confirmada).

---

## 11. Índices MongoDB para a Fase II

### `pull_count_desc`

```javascript
db.repositories_data.createIndex(
  { pull_count: -1 },
  { name: "pull_count_desc" }
)
```

Usado por `ClaimNextBuildRepo` para ordenar repositórios por popularidade e aplicar o filtro de threshold. Cobre também `CountPendingBuildRepos`.

### `stage2_partial`

```javascript
db.repositories_data.createIndex(
  { pull_count: -1 },
  {
    name: "stage2_partial",
    partialFilterExpression: { graph_built_at: null }
  }
)
```

Índice parcial que cobre apenas documentos onde `graph_built_at` é `null` (ausente ou explicitamente nulo). Uma vez que `graph_built_at` é setado em um documento, esse documento **sai do índice** automaticamente. Isso faz o índice encolher progressivamente conforme a Fase II avança, mantendo as queries rápidas mesmo com 12M de documentos.

**Por que `null` e não `{$exists: false}`?** O MongoDB não suporta `$not` em `partialFilterExpression`. A expressão `{graph_built_at: null}` no MongoDB corresponde a documentos onde o campo está ausente **ou** é explicitamente `null` — o comportamento desejado.

---

## 12. Autenticação, rate limiting e jitter

### Rotação de identidade

`HubClient` encapsula rotação automática de JWT. Em cada requisição, os headers incluem:

- `Authorization: JWT <token>` (se autenticado)
- `User-Agent`, `Sec-Ch-Ua`, `Referer`, etc. — fingerprint de browser Chrome real
- `Accept-Language: pt-BR,...` — consistente com a localização da conta

Em resposta a erros HTTP:

| Código | Ação |
|--------|------|
| 401    | Invalida o token atual (`ClearToken`), rotaciona para próxima identidade |
| 429    | Aguarda 15s, depois rotaciona |
| 403    | Rotaciona identidade imediatamente |
| outros | Retorna ao caller sem retry (e.g., 404 = tag não existe) |

Até 3 tentativas por URL antes de retornar erro.

### Jitter anti-fingerprint

```go
// HubClient.Get — antes de cada chamada à API (tags, tag latest, imagens)
time.Sleep(time.Duration(200+rand.Intn(200)) * time.Millisecond)  // 200–400ms, média 300ms
```

O jitter é aplicado **antes de cada chamada HTTP**, não apenas entre repositórios. Com ~3 chamadas por repo (GetTags + GetTag("latest") + GetImages), o delay total por repo é 600–1200ms apenas em jitter.

O intervalo 200–400ms foi calibrado para evadir o tarpit do Cloudflare sem overhead excessivo: valores abaixo de 200ms aumentam a taxa de repostas 429/captcha; acima de 600ms desperdiçam throughput sem ganho proporcional de evasão.

---

## 13. Operação multi-nó

Múltiplas máquinas podem executar a Fase II simultaneamente contra o mesmo MongoDB e Neo4j. O mecanismo de exclusão mútua é `ClaimNextBuildRepo` — atômico no MongoDB.

**Configuração típica (3 máquinas):**

| Máquina | Papel | Compose file |
|---------|-------|--------------|
| gpu1    | Fase I (crawler) | `docker-compose.yml` |
| a9      | Fase I (crawler) | `docker-compose.yml` |
| gpu2    | Fase II (builder) | `docker-compose.node3.yml` |

A máquina executando a Fase II precisa de acesso de rede ao MongoDB e Neo4j da máquina primária:

```env
MONGO_URI=mongodb://<PRIMARY_IP>:27017
NEO4J_URI=neo4j://<PRIMARY_IP>:7687
```

Para adicionar uma segunda máquina na Fase II, basta rodar `make start-build` com as variáveis de ambiente apontando para a máquina primária. As contas `accounts_builder.json` em cada máquina devem ser diferentes para maximizar o paralelismo sem conflito de tokens.

---

## 14. Consultas Cypher úteis

### Encontrar imagens que expõem uma porta específica

```cypher
MATCH (l:Layer)
WHERE l.instruction STARTS WITH 'EXPOSE 80'
  AND size(l.images) > 0
RETURN l.images
```

Para encontrar **qualquer** camada com EXPOSE (não necessariamente a última):

```cypher
MATCH (exposed:Layer)
WHERE exposed.instruction STARTS WITH 'EXPOSE'
WITH exposed
MATCH (exposed)-[:IS_BASE_OF*0..]->(leaf:Layer)
WHERE size(leaf.images) > 0
RETURN DISTINCT exposed.instruction AS porta, leaf.images AS imagens
LIMIT 100
```

### Encontrar imagens que compartilham uma camada base

```cypher
MATCH (base:RawLayer {digest: "sha256:abc123..."})
-[:IS_SAME_AS]-(l:Layer)
-[:IS_BASE_OF*]->(leaf:Layer)
WHERE size(leaf.images) > 0
RETURN DISTINCT leaf.images
```

### Encontrar a cadeia de construção de uma imagem

```cypher
MATCH path = (root:Layer)-[:IS_BASE_OF*]->(leaf:Layer)
WHERE size(leaf.images) > 0
  AND ANY(img IN leaf.images WHERE img CONTAINS 'library/nginx:latest')
  AND NOT EXISTS { MATCH (x:Layer)-[:IS_BASE_OF]->(root) }
RETURN path
```

### Contar nós e relacionamentos

```cypher
MATCH (l:Layer) RETURN count(l) AS total_layers
MATCH (rl:RawLayer) RETURN count(rl) AS total_rawlayers
MATCH ()-[r:IS_BASE_OF]->() RETURN count(r) AS total_edges
```
