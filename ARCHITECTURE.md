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

  newInPrefix = processResults(res.Repositories)
  collect remaining pages [2 .. min(ceil(res.count/100), 100)]

  if res.count >= 10.000 OR len(prefix) == 1:
    tokenPlateau = (newInPrefix == 0 && res.count >= 10000
                    && strings.Contains(prefix, "-") && len(prefix) > 1)
    lastChar = prefix[-1]
    isSep = (lastChar == '-' || lastChar == '_')

    for char in [a-z, 0-9, -, _]:
      if isSep && (char == '-' || char == '_'): skip   // deduplicação de separadores
      child = prefix + char
      priority = calcPriority(child, newInPrefix, tokenPlateau)
      UPSERT {_id: child, status: "pending", priority: priority} IF NOT EXISTS

  updateTaskStatus(prefix, "done")

calcPriority(child, newInPrefix, tokenPlateau):
  if tokenPlateau:             return -1  // plateau: depriorizados, mas sem perda
  if !contains(child, "-"):    return 2   // sem hifén = substring genuína
  if newInPrefix > 0:          return 1   // pai achou novos repos
  return 0                                // padrão
```

**Ciclo de vida de um worker** (`worker`):

```
worker(id):
  emptyCount = 0
  loop:
    prefix = getNextTask()    // FindOneAndUpdate: pending → processing
    if prefix == "":
      emptyCount++
      if emptyCount % 6 == 0:
        if CountDocuments({status: "pending"}) == 0: break   // fila confirmada vazia
      sleep 5s; continue

    emptyCount = 0
    success = processTask(prefix, ...)
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

- **Atomicidade:** `getNextTask` usa `FindOneAndUpdate` com filtro `{status: "pending"}` e ordenação `{priority: -1, _id: 1}`. Múltiplos workers (inclusive em nós distintos do cluster) nunca processam o mesmo prefixo simultaneamente — garantia de exclusão mútua pelo MongoDB.
- **Resumibilidade após crash:** ao reiniciar, todas as tarefas no estado `processing` são revertidas para `pending` automaticamente. Somente tarefas com `done` são permanentemente ignoradas.
- **Priorização dinâmica:** `getNextTask` ordena por `{priority: -1, _id: 1}`. Prefixos com maior probabilidade de retornar repositórios novos são processados antes. Workers nunca encerram prematuramente: só param quando `CountDocuments({status: "pending"}) == 0` confirma fila genuinamente vazia.

### 2.3. Priorização da Fila DFS e Token-Match Plateau

O campo `priority int` em cada documento `crawler_keywords` controla a ordem de processamento. A tabela abaixo define os valores e sua semântica:

| Valor | Condição de atribuição | Semântica |
|-------|------------------------|-----------|
| `2` | Filho sem hifén (`!strings.Contains(child, "-")`) | Correspondência genuína de substring no ElasticSearch (sem tokenização). Alta probabilidade de repositórios distintos. |
| `1` | Pai encontrou repositórios novos (`newInPrefix > 0`) | Aprofundamento produtivo — a região tem dados. |
| `0` | Padrão | Pai não encontrou repositórios novos, mas não é plateau. |
| `-1` | Token-match plateau | Plateau detectado: prefixo com hifén, ≥ 10.000 resultados, zero novos. Depriorizado mas preservado. |

**Token-match plateau:** o Docker Hub indexa nomes de imagens com o ElasticSearch usando hifén como separador de token. A query `tcp-client` corresponde ao token `tcp-client` inteiro — e todos os repositórios que contêm esse token exato como substring já foram coletados. Expandir `tcp-client-a`, `tcp-client-b`, etc. retorna os mesmos 10.000 resultados com zero novidade. Em vez de podar esses filhos (o que causaria perda de dados em regiões densas legítimas), eles são inseridos com `priority=-1` e ficam para o final da fila, após todas as regiões de maior retorno.

**Deduplicação de separadores consecutivos:** se o prefixo termina em `-` ou `_`, os filhos `-` e `_` são omitidos ao gerar a expansão. O Docker Hub trata `--`, `-_`, `_-`, `__` como equivalentes ao separador simples; gerar esses filhos causaria recursão ilimitada sem dados novos.

### 2.4. Aquecimento de Cache em RAM

Antes de iniciar o DFS, `PreloadExistingRepos` carrega todos os nomes de repositórios já presentes no MongoDB para a `seenRepos sync.Map` em RAM:

```
PreloadExistingRepos():
  cursor = Find(RepoColl, {}, projection={namespace, name})
  for doc in cursor:
    seenRepos.Store(doc.namespace + "/" + doc.name, true)
```

**Motivação:** nós secundários do cluster conectam-se ao MongoDB do nó primário via rede. Sem o cache, cada repositório descoberto seria verificado contra o banco remoto — saturando a banda com duplicatas. Com o cache em RAM, a deduplicação ocorre em microssegundos localmente.

**Escala:** 5,2 milhões de registros ocupam aproximadamente 300 MB de RAM. O sistema suporta até 100 milhões de registros com consumo inferior a 6 GB, dentro dos limites de servidores de pesquisa típicos.

### 2.5. Estratégia Anti-Detecção — Modelo de Ameaças e Contramedidas

O Docker Hub opera atrás do Cloudflare com inspeção comportamental em múltiplas camadas. O sistema implementa uma pilha de defesa correspondente, com uma contramedida por vetor de detecção.

#### 2.5.1. Vetor: Fingerprint TLS (JA3)

**Ameaça:** a stack `net/http` padrão do Go negocia cifras TLS em ordem diferente de qualquer navegador real. O hash JA3 resultante é instantaneamente identificável como script automatizado, independentemente do User-Agent declarado.

**Contramedida:** o `http.Transport` é configurado para emular o Chrome 121:

```go
&tls.Config{
    MinVersion:               tls.VersionTLS12,
    PreferServerCipherSuites: false, // deixa o servidor ordenar as cifras
}
```

O HTTP/2 é deliberadamente desabilitado. Além da diferença de fingerprint, o multiplexing HTTP/2 cria um vetor adicional: quando o servidor aplica tarpit (seção 6.1), conexões half-open bloqueiam workers indefinidamente. Com HTTP/1.1, cada conexão é atômica — timeouts funcionam deterministicamente.

#### 2.5.2. Vetor: Headers Anômalos

**Ameaça:** requisições Go sem configuração explícita omitem headers presentes em todo navegador real (`Sec-Fetch-*`, `Referer`, `Accept-Language`). A ausência desses campos é um sinal de baixa fidelidade no scoring do WAF.

**Contramedida:** `HubClient.setHeaders` (em `myutils/hubclient.go`) injeta o conjunto completo de headers de uma requisição XHR do Chrome 145 para a API do Docker Hub. A função foi centralizada aqui a partir de `crawler/crawler.go` (onde existia como `setBrowserHeaders`), eliminando a duplicação entre os Estágios I e II:

```go
req.Header.Set("Accept",          "application/json, text/plain, */*")
req.Header.Set("Accept-Language", "pt-BR,pt;q=0.9,en-US;q=0.8,en;q=0.7")
req.Header.Set("Referer",         "https://hub.docker.com/")
req.Header.Set("Sec-Ch-Ua-Mobile","?0")
req.Header.Set("Sec-Fetch-Dest",  "empty")
req.Header.Set("Sec-Fetch-Mode",  "cors")
req.Header.Set("Sec-Fetch-Site",  "same-origin")
req.Header.Set("Connection",      "keep-alive")
```

O `Referer` (`https://hub.docker.com/`) e os campos `Sec-Fetch-*` reconstituem o contexto de navegação esperado de um usuário na interface principal do Docker Hub. A versão atualizada do `Accept-Language` reflete uma sessão de navegador com locale `pt-BR`, mais próxima do perfil demográfico típico de operadores de pesquisa brasileiros.

#### 2.5.3. Vetor: Correlação de Identidade entre Requisições

**Ameaça:** um WAF com estado pode correlacionar múltiplas contas vindas do mesmo IP com o mesmo User-Agent. Se conta A e conta B nunca coexistem no tráfego de um navegador real com o mesmo UA, a combinação é detectável.

**Contramedida:** identidade persistente por conta (Sticky UA). No carregamento, cada conta recebe um User-Agent fixo e exclusivo via round-robin sobre o pool de 7 strings:

```
Chrome 121 / Windows
Chrome 121 / Mac
Chrome 121 / Linux
Edge 121   / Windows
Firefox 122 / Windows
Safari 17  / Mac
Chrome 119 / Windows (versão ligeiramente defasada)
```

A mesma conta sempre usa o mesmo UA em todas as requisições — login, busca, manifests. Do ponto de vista do servidor, cada conta é um "navegador" distinto e coerente. A diferenciação entre nós (Nó 1 emula Windows, Nó 2 emula Linux/Mac) dilui a correlação estatística no nível do cluster.

#### 2.5.4. Vetor: Padrão de Abertura de Sockets

**Ameaça:** bots sem controle de conexão abrem um novo socket TCP por requisição. A taxa de handshakes TLS por IP (observável pelo servidor) é um sinal de bot forte e independente dos headers.

**Contramedida (a):** pool de conexões ativo. `DisableKeepAlives` foi removido do `http.Transport` do upstream. `MaxIdleConns=100`, `IdleConnTimeout=90s` mantêm sockets abertos entre requisições do mesmo worker.

**Contramedida (b):** body draining. Em Go, um socket só é devolvido ao pool se o corpo da resposta for lido até o fim antes do `Close`. `fetchPage` usa `io.ReadAll(resp.Body)` em vez de `resp.Body.Close()` diretamente — garante que cada socket seja reutilizado na próxima requisição do mesmo worker.

#### 2.5.5. Vetor: Padrão Temporal Rítmico

**Ameaça:** intervalos fixos entre requisições são estatisticamente improváveis para navegação humana e detectáveis por análise de frequência.

**Contramedida:** dois níveis de jitter aleatório:
- Entre páginas de um mesmo prefixo: `400 + rand.Intn(500)` ms → intervalo uniforme em [400, 900] ms
- Entre tarefas consecutivas do worker: `rand.Intn(1000)` ms → [0, 1000] ms

---

### 2.6. Tratamento de Erros HTTP — Semântica de Re-enfileiramento

Com a introdução do `HubClient` (seção 2.7), a lógica de retry e rotação de identidade foi movida para dentro do cliente compartilhado. `fetchPage` em `crawler/crawler.go` delega toda a tentativa HTTP ao `HubClient.Get`, que realiza até 3 tentativas com rotação automática. O único comportamento específico do Estágio I que permanece em `fetchPage` é o cooloff de 4 minutos para erros HTTP não retriáveis (respostas inesperadas fora do conjunto 401/429/403).

```
// Dentro de HubClient.Get (myutils/hubclient.go):
Get(url):
  for attempts in [0, 3):
    setHeaders(req)
    resp = client.Do(req)
    io.ReadAll(resp.Body)  // body draining obrigatório

    401 → rotate() → continue
    429 → rotate() → continue
    403 → rotate() → continue
    200 → return body, 200, nil
    outro → return nil, status, err

// Dentro de fetchPage (crawler/crawler.go — Stage I-specific):
fetchPage(hub, query, page):
  body, status, err = hub.Get(url)
  if err != nil:
    sleep 4 min  // cooloff para erros não-retriáveis
    return nil, false
  json.Unmarshal(body); return response, true
```

**401 — JWT expirado:** `rotate()` chama `ClearToken` no `IdentityProvider`, zerando o campo `Token` da conta correspondente. Na próxima chamada a `GetNextClient`, a conta dispara `LoginDockerHub` automaticamente.

**403 — Bot score alto:** indica IP ou conta com pontuação de bot elevada no scoring do Cloudflare. A rotação de identidade ocorre imediatamente dentro do `HubClient`; o cooloff de 4 minutos no nível do `fetchPage` (Stage I) permite que o IP "esfrie" antes da próxima tarefa.

**429 — Rate limit:** taxa de requisições excedida para a conta ou IP. O `HubClient` rotaciona para a próxima identidade disponível, distribuindo a pressão entre contas.

A tarefa nunca é perdida: em qualquer falha que resulte em `nil` retornado de `fetchPage`, `processTask` chama `updateTaskStatus(prefix, "pending")` antes de retornar.

### 2.7. Gestão de Identidades — `crawler/auth_proxy.go`

`IdentityManager` centraliza autenticação, proxies e User-Agents:

- Carrega contas de `accounts.json` (`[{username, password}]`)
- Atribui `UserAgent` exclusivo a cada conta no carregamento (round-robin sobre `globalUAPool`)
- Auto-login JWT via `POST /v2/users/login/` protegido por `loginMu sync.Mutex` (previne login paralelo da mesma conta)
- `GetNextClient()` retorna `(*http.Client, token, ua)` — o UA é propagado junto com o cliente e token para garantir que a identidade seja consistente ao longo de toda a sessão de uma tarefa

`ClearToken(token)` percorre as contas e zera o token da conta correspondente, forçando re-autenticação na próxima chamada a `GetNextClient`.

`IdentityManager` implementa a interface `IdentityProvider` (seção 2.7), o que permite que o mesmo `HubClient` seja utilizado tanto pelo Estágio I quanto pelo Estágio II sem dependência circular entre pacotes. A dependência flui apenas de `myutils` (que define `IdentityProvider`) para `crawler` e `buildgraph` (que implementam e consomem a interface), e nunca no sentido inverso.

### 2.8. HubClient — Cliente HTTP Autenticado Compartilhado

O `HubClient` (em `myutils/hubclient.go`) é a abstração central que elimina a duplicação de lógica de requisições autenticadas entre os Estágios I e II.

#### Interface IdentityProvider

```go
type IdentityProvider interface {
    GetNextClient() (*http.Client, string, string) // client, token, userAgent
    ClearToken(token string)
}
```

A interface é o ponto de desacoplamento entre `myutils` (que define `HubClient`) e `crawler` (que implementa `IdentityManager`). Qualquer struct que implemente esses dois métodos pode ser usada como fonte de identidade para um `HubClient`, permitindo substituição em testes ou extensão futura para outros provedores de autenticação.

#### Ciclo de vida do HubClient

O padrão de uso é **uma instância por goroutine**. No Estágio I, cada `worker()` chama `myutils.NewHubClient(pc.IM)` no início do seu loop. No Estágio II, cada `repoWorker` faz o mesmo. Instâncias não são compartilhadas entre goroutines; o estado interno (cliente HTTP ativo, token atual) é exclusivo de cada instância, eliminando a necessidade de sincronização adicional.

```
HubClient.Get(url):
  for attempts in [0, 3):
    req = buildRequest(url)
    setHeaders(req)             // Chrome 145 headers
    resp = client.Do(req)
    body = io.ReadAll(resp.Body)

    401/429/403 → rotate()     // troca identidade; próxima tentativa
    200         → return body, nil
    outro       → return nil, err  // falha não retriável
```

#### Métodos de alto nível

| Método | Descrição |
|--------|-----------|
| `Get(url)` | Requisição GET com 3 tentativas e rotação em 401/429/403 |
| `GetInto(url, dest)` | `Get` + `json.Unmarshal` no destino fornecido |
| `GetTags(ns, name, pageNum, size)` | Busca paginada de tags de um repositório |
| `GetImages(ns, name, tag)` | Busca manifests de imagem para uma tag específica |

#### Eliminação de duplicação

Antes desta versão, `crawler/crawler.go` e `buildgraph/from_mongo.go` mantinham lógicas paralelas e divergentes para: injeção de headers, retry em erros HTTP, rotação de identidade e body draining. O `HubClient` centraliza todas essas responsabilidades, reduzindo a superfície de manutenção e garantindo comportamento idêntico entre estágios.

### 2.9. Monitoramento e Telemetria

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

### 2.10. Distribuição Multi-Nó

O comando `crawl` suporta dois modos de particionamento:

| Modo | Flags | Comportamento |
|------|-------|--------------|
| Shard automático | `--shard N --shards M` | Divide o alfabeto igualmente entre M shards; shard N processa a fração correspondente. Implementado em `crawler.ShardSeeds(shard, total)` |
| Seeds manuais | `--seed a,b,c` | Seeds explícitas separadas por vírgula |
| Alfabeto completo | (nenhuma flag) | Semeia todo o alfabeto `[a-z, 0-9, -, _]` |

Dado que a fila de tarefas reside no MongoDB compartilhado, ambos os nós interagem com a mesma coleção `crawler_keywords`. O `FindOneAndUpdate` atômico garante que cada prefixo seja processado por exatamente um nó.

### 2.11. Resultados Empíricos (Produção)

Configuração: Node 1 (shard 0/2, 3 workers) + Node 2 (shard 1/2, 4 workers), 7 contas Docker Hub, MongoDB no Node 1, conexão remota Node 2 → Node 1.

| Métrica | Valor |
|---------|-------|
| Repositórios únicos acumulados | >2.100.000 |
| Throughput sustentado pós-otimização | ~10.000–18.000 repos únicos/minuto |
| Duplicatas no banco | 0 (índice único MongoDB `{namespace, name}`) |

---

## 3. Estágio II — BuildGraph

### 3.1. Arquitetura de Fila Distribuída com Claim Atômico

O Estágio II foi redesenhado com o mesmo padrão de fila persistente do Estágio I: em vez de um cursor MongoDB centralizado (vulnerável a partições e reinicializações), cada worker goroutine reivindica atomicamente o próximo repositório disponível via `ClaimNextBuildRepo`.

**Operação de claim (FindOneAndUpdate atômico):**

```
ClaimNextBuildRepo(threshold):
  filter = {
    pull_count:    {$gte: threshold},
    graph_built_at: {$exists: false},
    build_claimed:  {$ne: true}
  }
  update = {
    $set: {build_claimed: true, build_started_at: now()}
  }
  sort = {pull_count: -1}   // repositórios mais populares têm prioridade
  return collection.FindOneAndUpdate(filter, update, {sort: sort})
```

O `FindOneAndUpdate` atômico do MongoDB garante exclusão mútua: dois workers (em goroutines distintas ou em máquinas distintas) nunca processam o mesmo repositório simultaneamente.

**Inicialização e auto-cura (ResetStaleBuildClaims):**

No startup do Estágio II, antes de iniciar qualquer worker, `ResetStaleBuildClaims` libera claims órfãos de execuções anteriores que foram interrompidas sem concluir:

```
ResetStaleBuildClaims():
  filter = {build_claimed: true, graph_built_at: {$exists: false}}
  update = {$unset: {build_claimed: "", build_started_at: ""}}
  UpdateMany(filter, update)
```

Essa operação garante que o Stage II retome de onde parou sem reprocessar nem ignorar repositórios após qualquer tipo de interrupção — crash, OOM, reinicialização de container.

**Immortal worker — CountPendingBuildRepos:**

Após `ClaimNextBuildRepo` retornar "não encontrado", o worker não encerra imediatamente. Antes de parar, verifica com `CountPendingBuildRepos(threshold)` se há repositórios pendentes genuinamente:

```
CountPendingBuildRepos(threshold):
  filter = {
    pull_count:    {$gte: threshold},
    graph_built_at: {$exists: false},
    build_claimed:  {$ne: true}
  }
  return collection.CountDocuments(filter)
```

O worker encerra somente quando a contagem retorna zero. Isso evita terminação prematura quando todos os repositórios disponíveis estão temporariamente claimed por outros workers.

**Distribuição multi-nó:** múltiplas máquinas podem executar o Estágio II simultaneamente apontando para o mesmo MongoDB. O claim atômico garante que cada repositório seja processado por exatamente uma máquina. Não é necessária nenhuma coordenação adicional entre nós.

### 3.2. Pipeline de Workers

```
ClaimNextBuildRepo (por goroutine)
    │
    ▼ repoWorker × max(NumCPU × 8, 32)   [I/O bound — espera HTTPS]
    │   1. getTags: busca N tags mais recentes (cache MongoDB + fallback API)
    │   2. getImages: busca manifests por tag (cache MongoDB + fallback API)
    │   3. descartar imagens Windows
    │   4. defer markBuilt → MarkRepoGraphBuilt sempre executado
    │
jobChan (buffer)
    │
    ▼ graphWorker × max(NumCPU × 2, 8)   [DB bound — Bolt/TCP → Neo4j]
    │   1. SHA256 chain de IDs (local, CPU)
    │   2. InsertImageToNeo4j (transação única por imagem)
    │   3. m.Neo4jInserts++ (contador atômico)
    │
Neo4j (Layer nodes + IS_BASE_OF edges + IS_SAME_AS → RawLayer nodes)

checkpointWriter (goroutine única)
    │
    ▼ dataDir/build_checkpoint.jsonl   [append-only, single-writer]
```

**Dimensionamento dos workers:**
- `repoWorkers`: `max(NumCPU × 8, 32)` — goroutines aguardam respostas HTTPS sem consumir CPU. Mínimo de 32 assegura paralelismo em máquinas compactas.
- `graphWorkers`: `max(NumCPU × 2, 8)` — escrita Neo4j via Bolt é o gargalo; excesso de conexões simultâneas degrada o throughput. O fator 2 equilibra paralelismo com estabilidade.

### 3.3. Autenticação e Headers (HubClient)

O Estágio II utiliza o `HubClient` (seção 2.7) com o mesmo padrão do Estágio I: uma instância por goroutine `repoWorker`, criada via `myutils.NewHubClient(ip)` onde `ip` é o `IdentityProvider` passado por `buildgraph.Build`.

```go
func repoWorker(ip myutils.IdentityProvider, ...) {
    hub := myutils.NewHubClient(ip)
    for {
        repo := ClaimNextBuildRepo(threshold)
        if repo == nil { break }
        processRepo(hub, repo, ...)
    }
}
```

**Cache MongoDB para tags e imagens:** antes de chamar a API, `getTags` e `getImages` consultam a coleção MongoDB correspondente. A taxa de acerto do cache (registrada em `BuildMetrics`) tipicamente supera 80% após o Stage I, pois muitos repositórios já tiveram suas tags e imagens armazenadas. O fallback para a API só ocorre quando o cache não contém o dado.

**Rotação JWT em 401/429/403:** herdada do `HubClient.Get` — comportamento idêntico ao Estágio I, sem código duplicado.

### 3.4. Métricas e Estimativa de Tempo (BuildMetrics)

`BuildMetrics` (em `buildgraph/metrics.go`) rastreia o progresso do Estágio II com contadores atômicos, garantindo leituras e escritas seguras de múltiplas goroutines:

| Contador | Descrição |
|----------|-----------|
| `Processed` | Repositórios concluídos (independente de sucesso ou erro) |
| `TagCacheHits` | Buscas de tags satisfeitas pelo cache MongoDB |
| `TagAPIFetches` | Buscas de tags que recorreram à API Docker Hub |
| `ImageCacheHits` | Buscas de imagens satisfeitas pelo cache MongoDB |
| `ImageAPIFetches` | Buscas de imagens que recorreram à API Docker Hub |
| `Neo4jInserts` | Inserções bem-sucedidas no Neo4j |
| `Errors` | Erros fatais durante o processamento de repositórios |

`startReporter(dataDir, done)` executa em goroutine dedicada, registrando métricas em log e em `dataDir/build_metrics.log` a cada 60 segundos:

```
[METRICS 02:15:00] progresso=1234/48000 (2.6%) | taxa=45.2 repos/min | ETA=17h22m | cache tags=82% imgs=71% | neo4j=12340 | erros=3 | uptime=27m18s
```

O ETA é calculado após os primeiros 30 segundos de dados acumulados:

```
taxa = processed / elapsed_minutes
ETA  = (total - processed) / taxa
```

Antes de 30 segundos, o campo ETA exibe "calculando..." para evitar estimativas instáveis na fase de aquecimento.

### 3.5. Persistência em Disco Físico

A versão anterior do `docker-compose.yml` utilizava um named Docker volume (`neo4j_data:/data`) para o Neo4j. Named volumes são gerenciados pelo Docker Engine e podem ser destruídos por `docker system prune -a --volumes`, que é um comando de limpeza de rotina em servidores compartilhados. O grafo Neo4j representa semanas de processamento do Estágio II — a perda seria catastrófica.

**Mudança:** o volume do Neo4j foi migrado para host path explícito:

```yaml
# docker-compose.yml — antes
volumes:
  - neo4j_data:/data

volumes:  # seção raiz
  neo4j_data:

# docker-compose.yml — depois
volumes:
  - ./neo4j_data:/data
# (sem seção volumes raiz — não há named volumes)
```

O MongoDB já utilizava `./mongo_data_secure:/data/db` (host path) desde a versão 2.0.0. Agora ambos os bancos estão em caminhos explícitos no sistema de arquivos do host, imunes a comandos de limpeza do Docker.

**Checkpoint do Estágio II:** a goroutine `checkpointWriter` persiste uma linha JSONL por repositório processado em `dataDir/build_checkpoint.jsonl`. O padrão single-writer elimina a necessidade de mutex na escrita do arquivo.

### 3.6. Algoritmo IDEA e Inserção no Neo4j

#### 3.6.1. Hashing de Layer IDs

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

**Propriedade fundamental:** duas imagens que compartilham as mesmas N primeiras layers na mesma ordem produzem `Layer_N.id` idênticos. Relações de herança são identificáveis por igualdade de ID, sem análise de conteúdo das layers.

#### 3.6.2. Transação Única por Imagem no Neo4j

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

#### 3.6.3. Estrutura do Grafo Neo4j

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
| `MarkRepoGraphBuilt(ns, name)` | Grava `graph_built_at` no repositório e remove `build_claimed` e `build_started_at` (checkpoint Stage II) |
| `ClaimNextBuildRepo(threshold)` | `FindOneAndUpdate` atômico para claim de repositório no Stage II; seta `{build_claimed: true, build_started_at: now}` |
| `ResetStaleBuildClaims()` | No startup do Stage II, libera claims órfãos: `{build_claimed: true, graph_built_at: {$exists: false}}` → `$unset build_claimed, build_started_at` |
| `CountPendingBuildRepos(threshold)` | Contagem de repositórios não claimed e sem `graph_built_at`; usada pelo immortal worker pattern |
| `FindImagesByDigests(digests)` | Query em lote com `$in`; substitui N chamadas individuais a `FindImageByDigest` — elimina padrão N+1 no Stage II |
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

`V2SearchURLTemplate` e `GetV2SearchURL` fornecem a URL canônica para a API de busca V2 do Docker Hub:

```go
V2SearchURLTemplate = `https://hub.docker.com/v2/search/repositories/?query=%s&page=%d&page_size=%d`
```

O parâmetro `&ordering=-pull_count` foi removido nesta versão. A remoção é motivada pela diferença semântica entre os dois modos de ordenação da API:

- **`ordering=-pull_count`** (anterior): ordena resultados por contagem de pulls decrescente dentro do conjunto de documentos que correspondem ao token da query. Isso significa que queries curtas e ambíguas (como `"a"`) retornam repositórios populares, mas não necessariamente aqueles cujo nome começa com `"a"` — o ElasticSearch inclui qualquer documento onde `"a"` apareça como token em qualquer campo.
- **`best_match`** (padrão da API, comportamento atual): prioriza correspondências exatas de prefixo antes de correspondências parciais. Para o DFS por prefixo, isso significa que `query="ngin"` retorna `nginx` antes de `some-image-with-nginx-in-description`, maximizando a relevância dos resultados coletados em cada nó da árvore DFS.

A remoção do parâmetro `ordering` não reduz o determinismo da coleta, pois a deduplicação é garantida pelo índice único MongoDB em `{namespace, name}`, não pela ordem de chegada dos resultados.

### 4.6. `scripts/calculate_node_dependent_weights.go` — Correção de Bug

O branch `if repoDoc.Namespace == "library"` continha `continue` como primeira instrução, tornando todo o código subsequente (`FindAllTagsByRepoName`, etc.) inalcançável. Imagens oficiais Docker (namespace `library`) eram silenciosamente ignoradas no cálculo de dependency weight.

Correção: `continue` removido. Imagens `library` agora passam pelo mesmo processamento das imagens community.

---

## 5. Limitações Conhecidas

1. **Expiração de JWT:** tokens Docker Hub expiram em ~24h. A expiração é tratada automaticamente via `ClearToken` + `GetNextClient` que dispara novo `LoginDockerHub`. Reinicializações do container também renovam todos os tokens.

2. **Build com API live:** se um repositório for deletado entre o Estágio I e o Estágio II, erros são logados mas não interrompem o processamento. O uso de `defer markBuilt` em `processRepo` garante que o repositório seja marcado como concluído mesmo em caso de erros de API, evitando reprocessamento infinito.

3. **Cobertura do espaço de busca:** o DFS sobre prefixos `[a-z0-9-_]` não garante cobertura de repositórios com nomes compostos exclusivamente por outros caracteres (ex.: Unicode). Cobertura prática para nomenclaturas descritivas é alta, mas não foi quantificada formalmente.

4. **Throughput Neo4j:** uma transação por imagem (O(1) round-trips). Para volumes >1M imagens, o gargalo migra para a memória heap do Neo4j — aumentar `NEO4J_dbms_memory_heap_max__size` é recomendado.

5. **Re-crawl após conclusão:** quando todas as tarefas do Estágio I atingem o estado `done`, a fila não é re-inicializada automaticamente. Para iniciar um novo ciclo de coleta — por exemplo, capturar repositórios criados após o ciclo anterior — é necessário resetar o campo `status` para `pending` via update MongoDB ou limpar a coleção `crawler_keywords`. Analogamente, um re-run do Estágio II requer a remoção dos campos `graph_built_at` dos documentos a serem reprocessados.

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
