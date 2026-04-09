# DITector — Large-Scale Docker Hub Security Research Pipeline

> Fork de [NSSL-SJTU/DITector](https://github.com/NSSL-SJTU/DITector), estendido para suportar crawling distribuído em larga escala, construção paralela do grafo de dependências e geração de datasets priorizados para scanning de segurança com OpenVAS.

---

## Índice

1. [Contexto e Motivação](#1-contexto-e-motivação)
2. [Arquitetura da Pipeline](#2-arquitetura-da-pipeline)
3. [Metodologia Científica (paper Dr. Docker)](#3-metodologia-científica)
4. [O que este fork modifica](#4-o-que-este-fork-modifica)
5. [Pré-requisitos e Configuração](#5-pré-requisitos-e-configuração)
6. [Configuração do `config.yaml`](#6-configuração-do-configyaml)
7. [Estágio I — Crawling (Descoberta)](#7-estágio-i--crawling-descoberta)
8. [Estágio II — Build (Grafo IDEA)](#8-estágio-ii--build-grafo-idea)
9. [Estágio III — Rank (Priorização)](#9-estágio-iii--rank-priorização)
10. [Integração com OpenVAS](#10-integração-com-openvas)
11. [Automação da Pipeline](#11-automação-da-pipeline)
12. [Monitoramento](#12-monitoramento)
13. [Referência de Comandos](#13-referência-de-comandos)
14. [Decisões de Design e Trade-offs](#14-decisões-de-design-e-trade-offs)


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

  NÓ 1 / NÓ 2 (Crawlers)              NÓ 1 (Bancos de Dados)
  ┌──────────────────┐                 ┌──────────────────────┐
  │  Docker Hub      │                 │     MongoDB          │
  │  V2 API          │────────────────▶│  (repositories_data) │
  │  /v2/search/     │                 │  namespace, name,    │
  │  Stage I: CRAWL  │                 │  pull_count          │
  │  (DFS + Workers) │                 └──────────┬───────────┘
  └──────────────────┘                            │
                                                  │
  NÓ 3 (Builder)                                  │
  ┌──────────────────┐                 ┌──────────▼───────────┐
  │  Docker Hub      │                 │     Stage II         │
  │  Tag+Image API   │────────────────▶│     BUILD            │
  │  (JWT authn,     │                 │  Claim atômico +     │
  │   HubClient,     │                 │  HubClient + cache   │
  │   cache MongoDB) │                 │  + Neo4j IDEA        │
  └──────────────────┘                 └──────────┬───────────┘
                                                  │
                                       ┌──────────▼───────────┐
                                       │     Neo4j            │
                                       │  (Layer IDEA graph)  │
                                       │  IS_BASE_OF edges    │
                                       │  ./neo4j_data/       │
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
                                       │                      │
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

O upstream original (`NSSL-SJTU/DITector`) declarava o subcomando `crawl` mas sem implementação (campo `Run` ausente no `cobra.Command` correspondente). Os estágios II e III estavam funcionais. Este fork implementa o Estágio I completo e reengenharia o Estágio II para operação paralela em larga escala.

### 4.1 Novo pacote `crawler/`

**Arquivo:** `crawler/crawler.go`

Implementação do crawler distribuído descrito no paper. O upstream original declarava o subcomando `crawl` em `cmd/cmd.go` mas sem campo `Run` — o comando era um stub registrado sem implementação. Este fork implementa o corpo completo do Estágio I.

**Arquitetura de fila de tarefas:**
- `ParallelCrawler` mantém N workers que consomem tarefas da coleção MongoDB `crawler_keywords`
- Cada tarefa é um prefixo DFS com campo `status`: `pending` → `processing` → `done`
- `getNextTask()` usa `FindOneAndUpdate` atômico, garantindo que dois workers (inclusive em nós distintos) nunca processem o mesmo prefixo simultaneamente
- `ensureQueueInitialized()` semeia o alfabeto `[a-z0-9-_]` como `pending` apenas se a coleção estiver vazia; na reinicialização, reverte tarefas `processing` → `pending` (self-healing após crash)
- `processTask()`: coleta todas as páginas do prefixo (máx. 100 páginas × 100 resultados), depois insere 38 filhos como `pending` se `count >= 10.000` ou `len(prefix) == 1` (stopword workaround)
- Deduplicação em memória via `seenRepos sync.Map` (O(1)); `PreloadExistingRepos()` aquece o cache no boot carregando todos os nomes do banco para RAM

**Estratégia anti-detecção — `fetchPage`:**

O Docker Hub aplica WAF/Cloudflare com detecção comportamental. A resposta é uma pilha de camuflagem em múltiplas camadas:

| Camada | Mecanismo | Implementação |
|--------|-----------|---------------|
| Fingerprint TLS | HTTP/1.1 forçado (sem HTTP/2), TLS 1.2+ | `tls.Config{MinVersion: tls.VersionTLS12}` + transporte sem HTTP/2 |
| Headers de navegador | Conjunto completo de headers Chrome 121 | `setBrowserHeaders()`: UA, Accept, Referer, Sec-Fetch-*, Connection |
| Identidade por conta | Cada conta JWT tem UA fixo e exclusivo | `acc.UserAgent` atribuído no boot via round-robin sobre pool de 7 UAs |
| Jitter entre páginas | 400–900 ms aleatório entre páginas | `rand.Intn(500) + 400` ms por requisição |
| Jitter entre tarefas | 0–1000 ms aleatório após cada tarefa | `rand.Intn(1000)` ms no loop do worker |
| Keep-Alive / body draining | Corpo lido completamente antes de fechar | `io.ReadAll(resp.Body)` — devolve socket ao pool TCP |

**Tratamento de erros HTTP — sem retry recursivo:**

| Código | Interpretação | Ação | Destino da tarefa |
|--------|---------------|------|-------------------|
| 401 | JWT expirado | `ClearToken(token)` + `GetNextClient()` | re-enfileirada como `pending` |
| 403 | Bot score alto / IP flagrado | sleep 15 min + `GetNextClient()` | re-enfileirada como `pending` |
| 429 | Rate limit por IP/conta | sleep 15 s + `GetNextClient()` | re-enfileirada como `pending` |
| outros | Erro transitório | retorna `nil` | re-enfileirada como `pending` |

A tarefa nunca é descartada: em qualquer falha, `processTask` chama `updateTaskStatus(prefix, "pending")` antes de retornar. Na próxima iteração de qualquer worker disponível, ela será retomada.

---

**Arquivo:** `crawler/auth_proxy.go`

`IdentityManager` centraliza autenticação, User-Agents e clientes HTTP:

- Carrega contas Docker Hub de `accounts.json` (`[{username, password}]`)
- Atribui `UserAgent` exclusivo a cada conta no carregamento — round-robin sobre `globalUAPool` (7 strings representando Chrome 121, Edge, Firefox 122, Safari 17 em Windows, Mac e Linux)
- `GetNextClient()` retorna `(*http.Client, token, ua)`: o UA é retornado junto com o token para ser propagado coerentemente em todas as requisições daquela identidade
- Login JWT via `POST /v2/users/login/` protegido por `loginMu sync.Mutex` — evita que dois workers loguem a mesma conta simultaneamente
- `ClearToken(token string)` percorre as contas e zera o campo `Token` da conta correspondente ao token expirado; na próxima chamada a `GetNextClient`, `LoginDockerHub` é invocado automaticamente
- O `http.Transport` por cliente configura `MaxIdleConns=100`, `IdleConnTimeout=90s` e `TLSHandshakeTimeout=10s`, mantendo um pool TCP estável e evitando a abertura massiva de sockets (sinal de bot)

### 4.2 Novo arquivo `buildgraph/from_mongo.go`

Reengenharia completa do estágio `build` para operação distribuída com claim atômico MongoDB:

```
ClaimNextBuildRepo (por goroutine — FindOneAndUpdate atômico)
    │
    ▼ repoWorker × max(NumCPU*8, 32)   ← I/O bound: espera de rede
    │   (HubClient autenticado por goroutine)
    │   (cache MongoDB → fallback API para tags e imagens)
    │   (defer MarkRepoGraphBuilt — sempre executado)
    ▼
jobChan
    │
    ▼ graphWorker × max(NumCPU*2, 8)   ← DB bound: escrita Neo4j
    │
    ▼
Neo4j (grafo IDEA) + MongoDB (graph_built_at)

checkpointWriter (goroutine single-writer)
    ▼
dataDir/build_checkpoint.jsonl
```

**Claim atômico:** cada `repoWorker` usa `ClaimNextBuildRepo` em vez de cursor compartilhado, habilitando execução distribuída em múltiplas máquinas. `ResetStaleBuildClaims` no startup libera claims órfãos de runs anteriores.

**Checkpointing:** `defer MarkRepoGraphBuilt` em `processRepo` garante que `graph_built_at` seja gravado para todos os repositórios processados, inclusive aqueles sem tags disponíveis — eliminando o reprocessamento infinito de repositórios vazios.

### 4.3 Modificação em `myutils/urls.go`

Template e função para a V2 Search API:

```go
V2SearchURLTemplate = `https://hub.docker.com/v2/search/repositories/?query=%s&page=%d&page_size=%d`

func GetV2SearchURL(query string, page, size int) string
```

O parâmetro `ordering=-pull_count` foi removido. O Docker Hub utiliza `best_match` como modo padrão de ordenação, que prioriza correspondências exatas de prefixo antes de resultados por popularidade. Para o DFS por prefixo, `best_match` é semanticamente superior: `query="ngin"` retorna `nginx` antes de repositórios que apenas mencionam "nginx" em descrições, maximizando a relevância dos resultados coletados em cada nó da árvore DFS.

A consistência entre páginas é garantida pelo índice único MongoDB em `{namespace, name}`, não pela ordem de chegada.

O upstream declarava o subcomando `crawl` como stub sem implementação e não utilizava nenhuma API de busca.

### 4.4 Novo arquivo `myutils/hubclient.go`

`HubClient` é o cliente HTTP autenticado compartilhado pelos Estágios I e II, eliminando duplicação de código:

- **Interface `IdentityProvider`** — abstração sobre `IdentityManager`; permite que `myutils` não dependa de `crawler`
- **`NewHubClient(ip IdentityProvider) *HubClient`** — uma instância por goroutine
- **`Get(url)`** — 3 tentativas com rotação em 401/429/403; headers Chrome 145 injetados automaticamente
- **`GetInto(url, dest)`** — `Get` + unmarshal JSON
- **`GetTags(ns, name, pageNum, size)`** — busca paginada de tags autenticada
- **`GetImages(ns, name, tag)`** — busca de manifests de imagem autenticada
- **`setHeaders(req)`** — injeta `Accept-Language: pt-BR,pt;q=0.9,...`, `Referer: https://hub.docker.com/`, `Sec-Fetch-*`
- **`rotate()`** — troca identidade internamente via `IdentityProvider`

### 4.5 Novo arquivo `buildgraph/metrics.go`

`BuildMetrics` fornece rastreamento de progresso em tempo real para o Estágio II:

- Contadores atômicos para `Processed`, cache hits/misses de tags e imagens, inserções Neo4j, erros
- `newBuildMetrics(threshold)` captura o total de repositórios pendentes no momento do startup
- `startReporter(dataDir, done)` loga e persiste em `build_metrics.log` a cada 60s
- ETA calculado após 30s: `taxa = processed/elapsed_min`, `ETA = (total−processed)/taxa`

### 4.6 Modificações em `myutils/mongo.go`

Adicionadas ao cliente MongoDB para suportar o crawler de alta vazão e o Estágio II distribuído:

- **`BulkUpsertRepositories(repos []*Repository)`** — bulk write atômico e não-ordenado; ~10-50× mais rápido que upserts individuais em loop para processar uma página de resultados inteira de uma vez
- **`KeywordsColl`** — nova coleção `crawler_keywords` para checkpointing do Estágio I: ao reiniciar, keywords já completamente crawleadas são ignoradas em O(1)
- **`IsKeywordCrawled(keyword)` / `MarkKeywordCrawled(keyword)`** — interface de leitura/gravação do checkpoint do Estágio I
- **`MarkRepoGraphBuilt(namespace, name)`** — grava `graph_built_at` e remove `build_claimed`/`build_started_at` (checkpoint Stage II)
- **`ClaimNextBuildRepo(threshold)`** — `FindOneAndUpdate` atômico para claim de repositório no Stage II
- **`ResetStaleBuildClaims()`** — libera claims órfãos no startup do Stage II
- **`CountPendingBuildRepos(threshold)`** — verifica fila vazia para immortal worker pattern
- **`FindImagesByDigests(digests)`** — query em lote com `$in`; substitui N queries individuais
- **Connection pool**: `SetMaxPoolSize(100)`, `SetMinPoolSize(5)`, `SetMaxConnIdleTime(5m)` — estabilidade sob carga paralela alta
- **Timeout do ping inicial**: aumentado de `1s` para `30s` — evita falso-negativo em conexões lentas

### 4.7 Modificações em `myutils/neo4j.go`

`InsertImageToNeo4j` foi reescrito para **transação única por imagem** (antes: uma transação por layer):

1. Todos os IDs de layer são computados localmente via SHA256 (puro CPU, zero I/O de rede)
2. Toda a cadeia de layers + tag de imagem é inserida em **uma única `ExecuteWrite`** — O(1) round-trips por imagem independente do número de layers

Resultado: latência de inserção cai de O(N layers × RTT) para O(1 × RTT).

**Correção em `findLayerNodesByRawLayerDigestFunc`:** a query original usava `{id: $digest}` para matchar um nó `RawLayer`, mas a propriedade armazenada é `digest`. Corrigido para `{digest: $digest}`. O bug quebrava silenciosamente o rastreamento de imagens upstream.

### 4.8 Modificações em `myutils/docker_hub_api_requests.go`

O cliente HTTP global foi reestruturado:

- **`DisableKeepAlives: true` removido** — keep-alives habilitados; conexões TCP são reutilizadas entre requisições (economia de ~100-300ms de handshake+TLS por requisição)
- **Connection pool**: `MaxIdleConns: 300`, `MaxIdleConnsPerHost: 50`, `IdleConnTimeout: 90s`
- **`Timeout: 30s`** adicionado ao cliente global

### 4.9 Modificações em `myutils/config.go`

- **Env vars de override**: `MONGO_URI` e `NEO4J_URI` sobrescrevem os valores do `config.yaml` — permite rodar Node 2 apontando para o MongoDB do Node 1 sem alterar o arquivo de configuração
- **Localização do config**: `filepath.Dir(os.Args[0])` → `os.Getwd()` — o config é buscado relativo ao diretório de trabalho, não ao binário (compatível com `go run`)
- **Neo4j opcional**: se a conexão Neo4j falhar na inicialização, o sistema não aborta — útil para rodar apenas o Estágio I sem Neo4j ativo

### 4.10 Infraestrutura Docker Compose

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

`docker-compose.node3.yml` define o serviço `builder` para o Nó 3 (Stage II):
```bash
DB_HOST=<IP_NÓ_1> NEO4J_URI=neo4j://<IP_NÓ_1>:7687 make start-build
```

O volume do Neo4j foi migrado de named Docker volume para host path `./neo4j_data:/data`, protegendo dados contra `docker system prune -a --volumes`.

### 4.11 Modificação em `scripts/calculate_node_dependent_weights.go`

O branch `if repoDoc.Namespace == "library"` continha `continue` como primeira instrução, tornando todo o código abaixo inalcançável. Imagens oficiais Docker (`library/`) eram silenciosamente ignoradas no cálculo de dependency weight. O `continue` foi removido.

### 4.12 Novos scripts de automação (`automation/`)

- `pipeline_autopilot.sh` — executa os 3 estágios sequencialmente com configuração parametrizada
- `test_e2e.sh` — teste de integração end-to-end: crawl com seed `nginx`, build, rank, verifica output

---

## 5. Pré-requisitos e Configuração

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

## 6. Configuração do `config.yaml`

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

## 7. Estágio I — Crawling (Descoberta)

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
| `--seed` | — | Keywords iniciais para DFS, separadas por vírgula (sem seed = começa por todo o alfabeto) |
| `--shard` | -1 | Índice do shard (base 0) para crawl distribuído; requer `--shards` |
| `--shards` | 1 | Total de shards para distribuição meet-in-the-middle (ex: 2 para dividir o alfabeto entre 2 máquinas) |
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

## 8. Estágio II — Build (Grafo IDEA)

### O que faz

Para cada repositório no MongoDB com `pull_count >= threshold`, o Estágio II:

1. Reivindica atomicamente o repositório via `ClaimNextBuildRepo` (MongoDB `FindOneAndUpdate`), garantindo que nenhum outro worker o processe simultaneamente
2. Consulta o cache MongoDB de tags; recorre à API Docker Hub com autenticação JWT (HubClient) apenas quando o cache não contém o dado
3. Para cada tag, consulta o cache MongoDB de imagens; acessa a API para obter layers (digest, instruction, size) quando necessário
4. Filtra imagens Windows
5. Insere no Neo4j o grafo IDEA com o algoritmo de hashing de layer IDs (seção 3.2 do paper)
6. Marca o repositório como concluído via `MarkRepoGraphBuilt` (campo `graph_built_at`) — executado via `defer`, portanto garantido inclusive para repositórios com 0 tags

O Stage II pode ser executado em múltiplas máquinas simultaneamente. O claim atômico elimina reprocessamento duplicado sem nenhuma coordenação adicional entre nós.

### Como executar

**Via Makefile (Nó 3 — recomendado):**
```bash
# Configurar variáveis e iniciar o container builder
DB_HOST=<IP_NÓ_1> NEO4J_URI=neo4j://<IP_NÓ_1>:7687 make start-build

# Acompanhar logs
make logs-build
```

**Via linha de comando (desenvolvimento / teste local):**
```bash
go run main.go build \
  --format mongo \
  --threshold 1000 \
  --tags 3 \
  --accounts accounts.json \
  --data_dir /tmp/ditector_build \
  --config config.yaml
```

### Parâmetros

| Flag | Padrão | Descrição |
|------|--------|-----------|
| `--format` | `mongo` | Fonte de dados (somente `mongo` suportado) |
| `--threshold` | 1.000.000 | Pull count mínimo para processar um repositório |
| `--tags` | 10 | Número de tags mais recentes a processar por repositório |
| `--accounts` | — | Caminho para `accounts.json` (autenticação JWT — mesmo arquivo do Estágio I) |
| `--proxies` | — | Caminho para arquivo de proxies (opcional) |
| `--data_dir` | `.` | Diretório para `build_checkpoint.jsonl` e `build_metrics.log` |

Os parâmetros `--page` e `--page_size` foram removidos: o controle de progresso é gerenciado pelo campo `graph_built_at` no MongoDB (via claim atômico), não por paginação manual.

**Recomendações para pesquisa:**
- `--threshold 1000` — cobre a maior parte dos repositórios com atividade real
- `--tags 3` — alinhado com o paper Dr. Docker; as 3 tags mais recentes são suficientes para análise de herança

### Monitoramento do progresso

```bash
# Métricas com ETA em tempo real
tail -f build_metrics.log

# Exemplo de linha de métricas:
# [METRICS 02:15:00] progresso=1234/48000 (2.6%) | taxa=45.2 repos/min | ETA=17h22m | cache tags=82% imgs=71% | neo4j=12340 | erros=3 | uptime=27m18s

# Repositórios concluídos (linhas no checkpoint)
wc -l build_checkpoint.jsonl

# Contagem direta no MongoDB
mongosh <MONGO_URI>/dockerhub_data --eval \
  'db.repositories_data.countDocuments({graph_built_at: {$exists: true}})'

# Nodes no Neo4j
cypher-shell -u neo4j -p "" "MATCH (l:Layer) RETURN count(l) AS total_layers"

# Edges no Neo4j
cypher-shell -u neo4j -p "" "MATCH ()-[r:IS_BASE_OF]->() RETURN count(r) AS total_edges"
```

### Persistência dos dados Neo4j

O Neo4j persiste em `./neo4j_data/` (host path explícito). Essa pasta é criada automaticamente pelo Docker Compose no primeiro start. Ao contrário de named Docker volumes, ela não é afetada por `docker system prune -a --volumes`. Inclua `neo4j_data/` nos seus backups regulares junto com `mongo_data_secure/`.

---

## 9. Estágio III — Rank (Priorização)

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

## 10. Integração com OpenVAS

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

**Containers sem serviços de rede:** se o container não expõe portas ou não roda um daemon de rede, o OpenVAS não encontrará serviços. O script externo de scanning deve tratar esse caso avançando para o próximo container.

---

## 11. Automação da Pipeline

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

## 12. Monitoramento

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

## 13. Referência de Comandos

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

## 14. Decisões de Design e Trade-offs

### Por que o crawler foi implementado neste fork em Go?

O upstream declarava o subcomando `crawl` em `cmd/cmd.go` sem campo `Run` — registrado mas sem implementação. O Estágio I foi implementado neste fork em Go pela consistência de stack e pelas vantagens para workloads de I/O intensivo:
- **Goroutines**: escala para centenas de workers com ~2KB/goroutine (vs ~1MB/thread OS)
- **Channels**: comunicação entre estágios type-safe sem locks manuais
- **Único binário**: deploy trivial em múltiplas máquinas, sem runtime externo

### Por que o Build chama a API live em vez de ler do MongoDB?

O crawler (Estágio I) armazena apenas `namespace`, `name` e `pull_count`. Tags e layers são buscados no Estágio II via API live. Trade-off deliberado:
- **Prós**: volume de dados no MongoDB é menor; o crawler é mais rápido
- **Contras**: o build stage depende da disponibilidade da API; repositórios deletados entre crawl e build geram erros logados

Alternativa não implementada: o crawler poderia armazenar tags/layers diretamente, tornando o build stage totalmente offline.

### Limitações conhecidas

1. **JWT expiry e re-login**: ao receber HTTP 401, `fetchPage` chama `ClearToken` para invalidar o token expirado e `GetNextClient` para obter uma nova identidade com login automático. Se todas as contas estiverem simultaneamente com token inválido, o retry pode falhar para a página em questão.
2. **Build live API**: se um repositório for deletado entre o crawl e o build, erros são logados mas não interrompem o progresso.
3. **Throughput do Neo4j**: uma transação por imagem (O(1) round-trips). Para volumes >1M imagens, o gargalo migra para a memória heap do Neo4j — aumentar `NEO4J_dbms_memory_heap_max__size` é recomendado.

---

*Baseado no paper: Hequan Shi et al., "Dr. Docker: A Large-Scale Security Measurement of Docker Image Ecosystem", WWW '25.*
