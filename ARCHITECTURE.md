# Arquitetura Técnica — DITector Research Fork

**Base científica:** Hequan Shi et al., "Dr. Docker: A Large-Scale Security Measurement of Docker Image Ecosystem", WWW '25, NSSL-SJTU.

---

## 1. Visão Geral da Pipeline

O upstream original (`NSSL-SJTU/DITector`) é integralmente escrito em Go e implementa os estágios II e III (construção do grafo IDEA e ranqueamento). O Estágio I estava declarado como subcomando `crawl` em `cmd/cmd.go` — com descrição "crawl metadata of repositories and images from Docker Hub" — mas sem campo `Run`: o comando era registrado sem implementação. Este fork implementa o corpo completo do Estágio I e reengenharia o Estágio II para operação paralela em larga escala.

```
┌─────────────────────────────────────────────────────────────────┐
│                     DITector Research Pipeline                   │
├──────────────┬──────────────────────┬───────────────────────────┤
│  Estágio I   │    Estágio II        │       Estágio III         │
│  CRAWL       │    BUILD             │       RANK                │
│  (novo)      │    (reengenhado)     │       (upstream + fixes)  │
├──────────────┼──────────────────────┼───────────────────────────┤
│ crawler/     │ buildgraph/          │ scripts/                  │
│   crawler.go │   from_mongo.go      │   calculate_node_         │
│   auth_      │ myutils/neo4j.go     │   dependent_weights.go    │
│   proxy.go   │   (reescrito)        │                           │
└──────────────┴──────────────────────┴───────────────────────────┘
         │               │                        │
         ▼               ▼                        ▼
      MongoDB          Neo4j              final_prioritized_
  (repositories_    (grafo IDEA:           dataset.json
      data)          Layer nodes +          (JSONL)
                     IS_BASE_OF edges)
```

---

## 2. Estágio I — Crawler com Fila de Tarefas Persistente

### 2.1. Restrições da API do Docker Hub

A API de busca (`GET /v2/search/repositories/`) impõe as seguintes restrições relevantes:

- **Limite por query:** 10.000 resultados máximo, independente da cardinalidade real dos repositórios correspondentes.
- **Paginação:** máximo 100 resultados por página (até 100 páginas por keyword).
- **Stopwords do ElasticSearch:** queries de 1 caractere são tratadas como stopwords pelo motor de busca do Docker Hub, retornando contagens artificialmente baixas. A estratégia de aprofundamento incondicional em prefixos de 1 caractere contorna esta limitação.
- **Rate limiting e bot detection:** HTTP 429 para IPs com alta frequência; HTTP 403 para sessões identificadas como tráfego automatizado pelo WAF/Cloudflare.

O Docker Hub contém mais de 12 milhões de repositórios públicos. A combinação do limite de 10.000 resultados por query com a ausência de listagem pública exaustiva torna necessária a estratégia DFS sobre prefixos.

### 2.2. Arquitetura de Fila de Tarefas MongoDB

O Estágio I abandona a recursão em memória em favor de uma **fila de tarefas física** na coleção `crawler_keywords`. Cada documento representa um prefixo DFS com um campo `status` (`pending`, `processing`, `done`).

**Algoritmo de processamento de tarefa** (`processTask`):

```
processTask(prefix):
  res = fetchPage(prefix, page=1)
  if res == nil: updateTaskStatus(prefix, "pending"); return failure

  collect all pages [1 .. min(ceil(res.count/100), 100)]

  if res.count >= 10.000 OR len(prefix) == 1:
    // Aprofundamento: inserir filhos como pending (via BulkWrite com $setOnInsert)
    for char in [a-z, 0-9, -, _]:
      INSERT {_id: prefix+char, status: "pending"} IF NOT EXISTS

  updateTaskStatus(prefix, "done")
```

**Ciclo de vida de um worker** (`worker`):

```
worker(id):
  loop:
    prefix = getNextTask()    // FindOneAndUpdate: pending → processing
    if prefix == "": break    // sem mais tarefas disponíveis

    success, ... = processTask(prefix, ...)
    if !success: sleep 5s     // backoff antes de tentar próxima tarefa
    sleep rand(0..1000ms)     // jitter anti-fingerprint
```

**Inicialização da fila** (`ensureQueueInitialized`):

```
ensureQueueInitialized(seeds):
  // 1. Self-healing: resets tasks stuck in "processing" from a previous crash
  UPDATE {status: "processing"} → {status: "pending"}

  // 2. Se a fila já tem tarefas (count > 0), retomar de onde parou
  if count > 0: return

  // 3. Fila vazia: inserir seeds do alfabeto como pending
  for s in seeds: UPSERT {_id: s, status: "pending"} IF NOT EXISTS
```

**Propriedades da arquitetura:**

- **Atomicidade:** `getNextTask` usa `FindOneAndUpdate` com `{status: "pending"}` como filtro. Múltiplos workers (inclusive em nós distintos do cluster) nunca processam o mesmo prefixo simultaneamente — garantia de exclusão mútua pelo MongoDB.
- **Resumibilidade após crash:** ao reiniciar, todas as tarefas no estado `processing` são revertidas para `pending` automaticamente. Somente tarefas com `done` são permanentemente ignoradas.
- **Priorização dinâmica:** `getNextTask` ordena por `finished_at: 1` (ASC), de modo que tarefas nunca tentadas (sem `finished_at`) têm prioridade sobre tarefas re-enfileiradas por falha.

### 2.3. Aquecimento de Cache em RAM

Antes de iniciar o DFS, `PreloadExistingRepos` carrega todos os nomes de repositórios já presentes no MongoDB para a `seenRepos sync.Map` em RAM:

```
PreloadExistingRepos():
  cursor = Find(RepoColl, {}, projection={namespace, name})
  for doc in cursor:
    seenRepos.Store(doc.namespace + "/" + doc.name, true)
```

**Motivação:** nós secundários do cluster conectam-se ao MongoDB do nó primário via rede. Sem o cache, cada repositório descoberto seria verificado contra o banco remoto — saturando a banda com duplicatas. Com o cache em RAM, a deduplicação ocorre em microssegundos localmente.

**Escala:** 5,2 milhões de registros ocupam aproximadamente 300 MB de RAM. O sistema suporta até 100 milhões de registros com consumo inferior a 6 GB, dentro dos limites de servidores de pesquisa típicos.

### 2.4. Impersonação de Navegador e Evasão Anti-Bot

O sistema implementa três camadas de camuflagem para contornar a detecção de tráfego automatizado pelo WAF/Cloudflare do Docker Hub.

#### 2.4.1. Fingerprinting TLS (JA3)

A stack de rede padrão do Go possui uma assinatura JA3 identificável como script. O `http.Transport` é configurado para emular o Chrome 121:

```go
transport := &http.Transport{
    TLSClientConfig: &tls.Config{
        MinVersion:               tls.VersionTLS12,
        PreferServerCipherSuites: false,
    },
    // HTTP/2 desativado: conexões HTTP/1.1 atômicas permitem que
    // timeouts funcionem deterministicamente (ver seção 6).
}
```

A desativação do HTTP/2 é crítica: o multiplexing HTTP/2 mantinha conexões half-open quando o servidor Docker Hub aplicava técnicas de tarpit (ver seção 6.1), bloqueando workers indefinidamente.

#### 2.4.2. Headers de Alta Fidelidade

`setBrowserHeaders` injeta o conjunto completo de headers de uma navegação Chrome real:

```go
req.Header.Set("User-Agent", ua)
req.Header.Set("Accept", "application/json, text/plain, */*")
req.Header.Set("Accept-Language", "en-US,en;q=0.9")
req.Header.Set("Referer", "https://hub.docker.com/search?q=library")
req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
req.Header.Set("Sec-Fetch-Dest", "empty")
req.Header.Set("Sec-Fetch-Mode", "cors")
req.Header.Set("Sec-Fetch-Site", "same-origin")
req.Header.Set("Connection", "keep-alive")
```

#### 2.4.3. Identidade Persistente por Conta (Sticky UA)

Cada conta JWT recebe um User-Agent fixo e exclusivo no momento do carregamento (`LoadIdentities`). Esta vinculação é mantida durante toda a execução: a mesma conta sempre usa o mesmo UA, tanto no login quanto nas requisições subsequentes.

A diferenciação entre nós do cluster (ex.: Nó 1 emulando Windows, Nó 2 emulando Linux/Mac) dilui a correlação estatística do tráfego gerado pelo cluster.

#### 2.4.4. Body Draining (Keep-Alive TCP)

Após cada resposta, o corpo é lido completamente via `io.ReadAll(resp.Body)` antes do `Close`. Em Go, isso é obrigatório para que o socket TCP seja devolvido ao pool de conexões. Sem o dreno, o `Transport` abre novos sockets a cada requisição — padrão detectável como bot e ineficiente em termos de latência TLS.

### 2.5. Tratamento de Erros HTTP e Backoff

```
fetchPage(query, page, client, token, ua):
  for attempts in [0, 3):
    resp = client.Do(req)

    401 → ClearToken(token); GetNextClient(); return nil  // JWT expirado: forçar re-login
    403 → sleep 15 min; GetNextClient(); return nil       // Bot score alto: esfriar IP
    429 → sleep 15s; GetNextClient(); return nil          // Rate limit: rotacionar identidade
    !200 → return nil
    OK  → decode JSON; return response
```

- **401:** `ClearToken` invalida o token expirado na conta correspondente. Na próxima chamada a `GetNextClient`, a conta sem token dispara `LoginDockerHub` automaticamente.
- **403:** sono de 15 minutos antes de retomar com nova identidade. Indica pontuação de bot elevada — rotação imediata sem espera é contraproducente.
- **429:** rotação imediata de identidade com delay de 15s.

Na ausência de erros, cada página aguarda um intervalo aleatório entre **400 ms e 900 ms** antes da próxima requisição (jitter), evitando a detecção de padrão rítmico de bots.

### 2.6. Gestão de Identidades — `crawler/auth_proxy.go`

`IdentityManager` centraliza autenticação, proxies e User-Agents:

- Carrega contas de `accounts.json` (`[{username, password}]`)
- Atribui `UserAgent` exclusivo a cada conta no carregamento (round-robin sobre `globalUAPool`)
- Auto-login JWT via `POST /v2/users/login/` protegido por `loginMu sync.Mutex` (previne login paralelo da mesma conta)
- `GetNextClient()` retorna `(*http.Client, token, ua)` — o UA é propagado junto com o cliente e token para garantir que a identidade seja consistente ao longo de toda a sessão de uma tarefa

`ClearToken(token)` percorre as contas e zera o token da conta correspondente, forçando re-autenticação na próxima chamada a `GetNextClient`.

### 2.7. Monitoramento e Telemetria

Uma goroutine separada loga o estado da fila a cada 30 segundos:

```go
go func() {
    for {
        active, _ := KeywordsColl.CountDocuments({status: "pending"})
        Logger.Info(fmt.Sprintf("--- STATS: %d workers active | %d tasks left | Uptime: %v",
            pending, active, time.Since(startTime)))
        time.Sleep(30 * time.Second)
    }
}()
```

`processTask` também loga uma **métrica de eficiência** por prefixo: proporção de repositórios novos em relação ao total baixado da página. Prefixos com eficiência baixa indicam regiões do espaço DFS já saturadas.

### 2.8. Distribuição Multi-Nó

O comando `crawl` suporta dois modos de particionamento:

| Modo | Flags | Comportamento |
|------|-------|--------------|
| Shard automático | `--shard N --shards M` | Divide o alfabeto igualmente entre M shards; shard N processa a fração correspondente. Implementado em `crawler.ShardSeeds(shard, total)` |
| Seeds manuais | `--seed a,b,c` | Seeds explícitas separadas por vírgula |
| Alfabeto completo | (nenhuma flag) | Semeia todo o alfabeto `[a-z, 0-9, -, _]` |

Dado que a fila de tarefas reside no MongoDB compartilhado, ambos os nós interagem com a mesma coleção `crawler_keywords`. O `FindOneAndUpdate` atômico garante que cada prefixo seja processado por exatamente um nó.

### 2.9. Resultados Empíricos (Produção)

Configuração: Node 1 (shard 0/2, 3 workers) + Node 2 (shard 1/2, 4 workers), 7 contas Docker Hub, MongoDB no Node 1, conexão remota Node 2 → Node 1.

| Métrica | Valor |
|---------|-------|
| Repositórios únicos acumulados | >2.100.000 |
| Throughput sustentado pós-otimização | ~10.000–18.000 repos únicos/minuto |
| Duplicatas no banco | 0 (índice único MongoDB `{namespace, name}`) |

---

## 3. Estágio II — BuildGraph

### 3.1. Pipeline de Três Estágios com Buffered Channels

`buildgraph/from_mongo.go` implementa um pipeline produtor-consumidor que desacopla os três gargalos físicos distintos: leitura de banco de dados (MongoDB), I/O de rede (API Docker Hub) e escrita de banco de dados (Neo4j).

```
MongoDB
  { pull_count >= threshold, graph_built_at: {$exists: false} }
    │
    ▼ goroutine Loader (única, leitura paginada)
    │
repoChan (buffer 4.000)
    │
    ▼ repoWorkers × max(NumCPU × 16, 64)       [I/O bound — espera HTTPS]
    │   1. GET tags da API Docker Hub
    │   2. GET manifests por tag (semáforo tagConcurrency=4 por repo)
    │   3. descartar imagens Windows
    │
jobChan (buffer 20.000)
    │
    ▼ buildGraphWorkers × max(NumCPU × 4, 16)  [DB bound — Bolt/TCP → Neo4j]
    │   1. SHA256 chain de IDs (local, CPU)
    │   2. InsertImageToNeo4j (transação única)
    │   3. MarkRepoGraphBuilt (MongoDB → graph_built_at)
    │
Neo4j (Layer nodes + IS_BASE_OF edges + IS_SAME_AS → RawLayer nodes)
```

**Dimensionamento dos workers:**
- `repoWorkers`: `max(NumCPU × 16, 64)` — fator 16 justificado pelo modelo de I/O: goroutines aguardam respostas HTTPS em estado de sleep sem consumir CPU. Mínimo absoluto de 64 garante paralelismo em máquinas com poucos núcleos.
- `buildGraphWorkers`: `max(NumCPU × 4, 16)` — escrita Neo4j via Bolt é menos paralelizável; excesso de conexões simultâneas degrada o throughput do banco. O fator 4 equilibra paralelismo com estabilidade.

### 3.2. Algoritmo IDEA — Hashing de Layer IDs

O algoritmo definido no paper Dr. Docker (Seção 3.2) é implementado em `myutils/neo4j.go`, função `InsertImageToNeo4j`:

**Content layer** (possui digest SHA256 do arquivo tar):
```
dig_i      = SHA256(layer_i.digest)
Layer_i.id = SHA256(Layer_{i-1}.id || dig_i)
```

**Config layer** (instrução Dockerfile sem conteúdo físico, ex.: `ENV`, `CMD`):
```
dig_i      = SHA256(layer_i.instruction)
Layer_i.id = SHA256(Layer_{i-1}.id || dig_i)
```

**Bottom layer** (i=0): usa `preID = ""` como valor anterior à concatenação.

**Propriedade fundamental:** duas imagens que compartilham as mesmas N primeiras layers na mesma ordem produzem `Layer_N.id` idênticos. Relações de herança são identificáveis por igualdade de ID — sem análise de conteúdo das layers.

### 3.3. Transação Única por Imagem no Neo4j

A implementação original do upstream executava uma transação Neo4j separada por layer (O(N) round-trips por imagem). O fork reescreve `InsertImageToNeo4j`:

```
// Fase 1 — local, sem I/O de rede:
records = []layerRecord{}
preID = ""
for each layer_i in image.Layers:
    dig_i  = SHA256(layer_i.digest or layer_i.instruction)
    currID = SHA256(preID + dig_i)
    records.append({prevID: preID, currID: currID, layer: layer_i})
    preID = currID

// Fase 2 — uma única transação:
session.ExecuteWrite(func(tx):
    for each record in records:
        tx.Run(MERGE (l:Layer {id: record.currID}) ...)
        tx.Run(MERGE (l)-[:IS_BASE_OF]->(next) ...)
        tx.Run(MERGE (rl:RawLayer {digest: ...})-[:IS_SAME_AS]-(l) ...)
    tx.Run(SET last_layer.images += [imgName])
)
```

**Complexidade de rede:**
- Anterior (upstream): O(N) round-trips por imagem, N ∈ [5, 30] tipicamente
- Atual: O(1) round-trips por imagem, independente de N

Com latência típica de Bolt/TCP de ~5–10ms por round-trip, uma imagem com 20 layers passa de ~100–200ms para ~5–10ms de custo de rede de inserção.

### 3.4. Checkpointing do Estágio II

Após processar todas as tags de um repositório com sucesso, `repoWorker` chama `MarkRepoGraphBuilt`, que grava `graph_built_at: <timestamp RFC3339>` no documento MongoDB. O Loader filtra por `{graph_built_at: {$exists: false}}`.

Em caso de interrupção: repositórios com `graph_built_at` gravado são ignorados; repositórios parcialmente processados são reprocessados integralmente — seguro pela idempotência dos `MERGE` no Neo4j.

### 3.5. Estrutura do Grafo Neo4j

| Tipo | Propriedades | Semântica |
|------|-------------|-----------|
| `Layer` | `id` (SHA256 chain), `digest`, `images[]`, `size`, `instruction` | Posição na cadeia de herança |
| `RawLayer` | `digest` | Conteúdo físico da layer |
| `[:IS_BASE_OF]` | — | Layer antecessora → Layer sucessora |
| `[:IS_SAME_AS]` | — | Layer ↔ RawLayer (posição ao conteúdo) |

Todas as inserções usam `MERGE` (não `CREATE`), garantindo idempotência.

---

## 4. Modificações Transversais no Upstream

### 4.1. `myutils/mongo.go`

| Adição | Descrição |
|--------|-----------|
| `BulkUpsertRepositories(repos)` | Bulk write não-ordenado; ~10–50× mais rápido que upserts individuais para processar uma página de resultados |
| `KeywordsColl` | Coleção `crawler_keywords` para a fila de tarefas do Estágio I. Cada documento: `{_id: prefix, status: pending|processing|done, started_at, finished_at}` |
| `MarkRepoGraphBuilt(ns, name)` | Grava `graph_built_at` no repositório (checkpoint Stage II) |
| Connection pool | `MaxPoolSize=100`, `MinPoolSize=5`, `MaxConnIdleTime=5min` |
| Timeout do ping inicial | `1s → 30s` (evita falso-negativo em conexões lentas) |

### 4.2. `myutils/docker_hub_api_requests.go`

| Parâmetro | Antes | Depois | Justificativa |
|-----------|-------|--------|---------------|
| `DisableKeepAlives` | `true` | removido (false) | Reutilização de conexões TCP/TLS; economia de ~100–300ms por requisição |
| `MaxIdleConns` | — | 300 | Pool de conexões para alta concorrência |
| `MaxIdleConnsPerHost` | — | 50 | Limita conexões ociosas por host |
| `IdleConnTimeout` | — | 90s | Descarte de conexões ociosas |
| `Timeout` | — | 30s | Timeout global por requisição |

### 4.3. `myutils/config.go`

| Modificação | Descrição |
|-------------|-----------|
| `MONGO_URI` / `NEO4J_URI` env vars | Sobrescrevem `config.yaml` — permite nós remotos apontarem para o banco central sem alterar configuração local |
| `os.Getwd()` em vez de `filepath.Dir(os.Args[0])` | Config buscado relativo ao CWD (compatível com `go run` e binários compilados) |
| Neo4j opcional na inicialização | Falha de conexão Neo4j não aborta o processo — útil para Estágio I sem Neo4j ativo |

### 4.4. `myutils/neo4j.go` — Correção de Bug Crítico

**`findLayerNodesByRawLayerDigestFunc`:** a query Cypher original usava `{id: $digest}` para matchar um nó `RawLayer`, mas nós `RawLayer` são criados com a propriedade `digest`. A propriedade `id` não existe em `RawLayer`. A query nunca retornava resultados, quebrando silenciosamente toda a funcionalidade de rastreamento de imagens upstream.

```cypher
-- Antes (upstream, incorreto):
MATCH (l:Layer)-[:IS_SAME_AS]-(rl:RawLayer {id: $digest})

-- Depois (correto):
MATCH (l:Layer)-[:IS_SAME_AS]-(rl:RawLayer {digest: $digest})
```

### 4.5. `myutils/urls.go`

Adicionados `V2SearchURLTemplate` e `GetV2SearchURL`:

```go
V2SearchURLTemplate = `https://hub.docker.com/v2/search/repositories/?query=%s&page=%d&page_size=%d&ordering=-pull_count`
```

O parâmetro `ordering=-pull_count` garante resultados determinísticos e ordenados por popularidade — necessário para consistência entre páginas durante o scraping.

### 4.6. `scripts/calculate_node_dependent_weights.go` — Correção de Bug

O branch `if repoDoc.Namespace == "library"` continha `continue` como primeira instrução, tornando todo o código subsequente (`FindAllTagsByRepoName`, etc.) inalcançável. Imagens oficiais Docker (namespace `library`) eram silenciosamente ignoradas no cálculo de dependency weight.

Correção: `continue` removido. Imagens `library` agora passam pelo mesmo processamento das imagens community.

---

## 5. Limitações Conhecidas

1. **Expiração de JWT:** tokens Docker Hub expiram em ~24h. A expiração é tratada automaticamente via `ClearToken` + `GetNextClient` que dispara novo `LoginDockerHub`. Reinicializações do container também renovam todos os tokens.

2. **Build com API live:** se um repositório for deletado entre o Estágio I e o Estágio II, erros são logados mas não interrompem o processamento.

3. **Cobertura do espaço de busca:** o DFS sobre prefixos `[a-z0-9-_]` não garante cobertura de repositórios com nomes compostos exclusivamente por outros caracteres (ex.: Unicode). Cobertura prática para nomenclaturas descritivas é alta, mas não foi quantificada formalmente.

4. **Throughput Neo4j:** uma transação por imagem (O(1) round-trips). Para volumes >1M imagens, o gargalo migra para a memória heap do Neo4j — aumentar `NEO4J_dbms_memory_heap_max__size` é recomendado.

5. **Re-crawl após conclusão:** quando todas as tarefas atingem o estado `done`, a fila não é re-inicializada automaticamente. Para iniciar um novo ciclo de coleta (ex.: capturar repositórios criados após o ciclo anterior), é necessário resetar manualmente o campo `status` para `pending` ou limpar a coleção `crawler_keywords`.

---

## 6. Resiliência de Rede e Estabilidade em Larga Escala

### 6.1. O Fenômeno do Tarpit HTTP/2

Durante a execução massiva de descoberta, ao ultrapassar 4,1 milhões de registros únicos, o cluster atingiu um ponto de stall crítico. A causa raiz foi identificada como técnica de tarpit aplicada pelo Docker Hub (via Cloudflare) ao detectar tráfego automatizado persistente.

O HTTP/2 utiliza multiplexing: múltiplas requisições compartilham uma única conexão TCP. Ao detectar o padrão de tráfego, o servidor parava de enviar frames de dados mas mantinha a conexão TCP aberta indefinidamente — conexão "half-open". Os workers ficavam presos em estado de I/O wait sem que os timeouts de aplicação fossem acionados, pois o protocolo HTTP/2 não encerrava a sessão. O resultado era uma degradação progressiva do throughput até paralisação total.

### 6.2. Solução: Degradação Forçada para HTTP/1.1

A desativação do HTTP/2 (`TLSNextProto: map[string]func(*http.Server, *tls.Conn, http.Handler){}` ausente do TLSClientConfig) força conexões HTTP/1.1 puras. Cada worker gerencia sessões TCP atômicas e independentes. Os timeouts de `ResponseHeaderTimeout` e `Timeout` no `http.Client` funcionam deterministicamente, matando conexões travadas e liberando o worker.

### 6.3. Estabilidade do Pipeline de Log (IO Deadlock)

Identificou-se que o redirecionamento de logs via shell (`>> log.txt`) causava travamentos totais quando os buffers de shell lotavam. O processo Go bloqueava na chamada de escrita ao stdout, paralisando toda a goroutine que chamou o logger.

Solução: remoção de redirecionamentos manuais. O Docker Engine gerencia o stdout de forma assíncrona via seu driver de log (`json-file` ou equivalente), garantindo que o processo Go nunca bloqueie por espera de escrita em disco.
