# Changelog — DITector Research Fork

---

## [3.0.0] — 2026-04-06

### Adicionado

**`myutils/hubclient.go` — cliente HTTP autenticado compartilhado (novo arquivo):**
- Interface `IdentityProvider` com métodos `GetNextClient() (*http.Client, string, string)` e `ClearToken(string)` — abstração que permite usar o mesmo `HubClient` nos Estágios I e II sem dependência circular de pacotes.
- Struct `HubClient` — encapsula lógica de requisições autenticadas, reutilizável por qualquer goroutine que receba um `IdentityProvider`. Cada goroutine cria sua própria instância.
- `NewHubClient(ip IdentityProvider) *HubClient` — construtor.
- `Get(url string) ([]byte, int, error)` — 3 tentativas com rotação automática de identidade em respostas 401, 429 e 403; headers de navegador embutidos.
- `GetInto(url string, dest interface{}) error` — `Get` seguido de `json.Unmarshal`.
- `GetTags(ns, name string, pageNum, size int)` — busca paginada de tags com autenticação.
- `GetImages(ns, name, tag string)` — busca de manifests de imagem com autenticação.
- `setHeaders(req *http.Request)` — injeta headers de navegador Chrome 145 (centralizado; antes duplicado em `crawler.go`).
- `rotate()` — rotaciona identidade internamente, chamado após respostas de erro retriáveis.

**`buildgraph/metrics.go` — rastreamento de progresso do Estágio II (novo arquivo):**
- Struct `BuildMetrics` com contadores atômicos: `Processed`, `TagCacheHits`, `TagAPIFetches`, `ImageCacheHits`, `ImageAPIFetches`, `Neo4jInserts`, `Errors`.
- `newBuildMetrics(threshold int64) *BuildMetrics` — captura `reposTotal` via `CountPendingBuildRepos` após `ResetStaleBuildClaims`.
- `startReporter(dataDir string, done <-chan struct{})` — goroutine que registra métricas em log e em arquivo `build_metrics.log` a cada 60 segundos.
- Linha de log formatada: `[METRICS HH:MM:SS] progresso=N/Total (%) | taxa=X repos/min | ETA=Xh | cache tags=% imgs=% | neo4j=N | erros=N | uptime=Xs`.
- ETA calculado após 30 segundos de dados coletados: `ETA = (total−processed) / (processed / elapsed_minutes)`.

**`docker-compose.node3.yml` — orquestração do Nó 3 (novo arquivo):**
- Serviço `builder` para execução exclusiva do Estágio II em máquina auxiliar.
- Variáveis de ambiente: `MONGO_URI`, `NEO4J_URI`, `ACCOUNTS`, `THRESHOLD`, `TAGS`.
- Comando de entrada: compilação e execução do subcomando `build` com flags `--accounts`, `--threshold`, `--tags`, `--data_dir`.

**`myutils/mongo.go` — novos métodos:**
- `ClaimNextBuildRepo(threshold int64) (*Repository, error)` — `FindOneAndUpdate` atômico: aplica `{build_claimed: true, build_started_at: now}` no documento. Filtro: `{pull_count >= threshold, graph_built_at: {$exists: false}, build_claimed: {$ne: true}}`. Ordenação: `{pull_count: -1}`.
- `ResetStaleBuildClaims()` — liberação de claims órfãos na inicialização: documentos com `{build_claimed: true, graph_built_at: {$exists: false}}` têm `build_claimed` e `build_started_at` removidos via `$unset`.
- `CountPendingBuildRepos(threshold int64) (int64, error)` — contagem de repositórios sem claim e sem `graph_built_at`, utilizada pelo immortal worker pattern.
- `FindImagesByDigests(digests []string) (map[string]*Image, error)` — query em lote com operador `$in`; substitui N chamadas individuais a `FindImageByDigest`, reduzindo o padrão N+1 a uma única query.

**`cmd/cmd.go` — novos flags do subcomando `build`:**
- `--accounts` — caminho para `accounts.json` (contas Docker Hub para autenticação JWT).
- `--proxies` — caminho para arquivo de proxies.
- `--data_dir` — diretório de destino para `build_checkpoint.jsonl` e `build_metrics.log`.

**`Makefile` — novos alvos:**
- `start-build` — inicia o Estágio II via `docker-compose.node3.yml`.
- `logs-build` — exibe e acompanha os logs do container `ditector_builder`.

### Modificado

**`crawler/crawler.go`:**
- Método `setBrowserHeaders` removido; a lógica foi centralizada em `HubClient.setHeaders` (`myutils/hubclient.go`).
- `worker()` instancia `hub := myutils.NewHubClient(pc.IM)` localmente; os parâmetros `client`, `token` e `ua` foram removidos da assinatura das funções internas.
- `fetchPage(hub, query, page)` simplificada: delega retry e rotação de identidade ao `HubClient.Get`; mantém apenas o cooloff de 4 minutos para erros HTTP não retriáveis (comportamento específico do Estágio I).
- `processTask(hub, prefix)` não recebe nem retorna mais `client`, `token` ou `ua`.
- Arrays `uaWindows` e `uaLinuxMac` removidos; seleção de User-Agent por ROLE removida — o UA é gerenciado exclusivamente pelo `IdentityManager`.
- Immortal worker: `emptyCount` agora usa `CountDocuments` para confirmar que a fila está efetivamente vazia antes de encerrar o worker.
- Timeout de `getNextTask`: 10s → 30s.

**`buildgraph/from_mongo.go` — reescrita completa:**
- `repoWorker` migrado de cursor MongoDB para `ClaimNextBuildRepo` (claim atômico), idêntico ao padrão `getNextTask` do Estágio I.
- Cada goroutine `repoWorker` cria seu próprio `HubClient` autenticado via `IdentityProvider`.
- `processRepo` usa `defer markBuilt` — repositório marcado como concluído em qualquer caso, inclusive aqueles com 0 tags, eliminando reprocessamento infinito.
- `getTags` e `getImages` recebem e incrementam `*BuildMetrics`.
- `graphWorker(jobChan, m)` incrementa `m.Neo4jInserts` a cada inserção no Neo4j.
- Goroutine `checkpointWriter` — escritor single-writer que persiste linhas JSONL em `dataDir/build_checkpoint.jsonl`.
- Funções renomeadas: `fetchTags` → `getTags`, `fetchImages` → `getImages`.
- Cache de imagens: padrão N+1 queries substituído por uma única query `$in` via `FindImagesByDigests`.
- `persistImages` extraída como função independente.

**`myutils/mongo.go`:**
- `MarkRepoGraphBuilt` atualizada: além de gravar `graph_built_at`, agora remove `build_claimed` e `build_started_at` via `$unset`.

**`buildgraph/build.go`:**
- Assinatura de `Build` atualizada: `Build(format, tagCnt, threshold, ip IdentityProvider, dataDir)` — parâmetros `page` e `pageSize` removidos (eram ignorados na implementação anterior).

**`myutils/urls.go`:**
- `V2SearchURLTemplate`: parâmetro `&ordering=-pull_count` removido. O Docker Hub utiliza `best_match` como modo de ordenação padrão, que prioriza correspondências exatas antes de resultados por popularidade.

**`docker-compose.yml`:**
- Volume do Neo4j alterado de named Docker volume (`neo4j_data:/data`) para volume de host (`./neo4j_data:/data`), evitando perda de dados por `docker system prune -a --volumes`.
- Seção `volumes:` (named volumes) removida do arquivo.

### Corrigido

- **Repositórios com 0 tags reprocessados indefinidamente:** o Estágio II anterior não marcava `graph_built_at` em repositórios que retornavam 0 tags da API, causando reprocessamento infinito desses registros. O uso de `defer markBuilt` em `processRepo` garante que o campo seja gravado independentemente do resultado da busca de tags.
- **Duplicação de headers de navegador entre estágios:** `setBrowserHeaders` estava duplicada em `crawler/crawler.go` e em chamadas do Estágio II. A centralização em `HubClient.setHeaders` elimina a divergência.
- **Claims órfãos após crash do Stage II:** ao reiniciar, `ResetStaleBuildClaims` libera automaticamente todos os documentos com `build_claimed=true` sem `graph_built_at`, garantindo retomada sem intervenção manual.
- **N+1 queries para cache de imagens no Stage II:** substituído por uma única query MongoDB com operador `$in` em `FindImagesByDigests`.

---

## [2.5.0] — 2026-04-04

### Adicionado

**Fila de tarefas persistente com prioridade (`crawler/crawler.go`):**
- `ensureQueueInitialized(seeds)` — inicializa `crawler_keywords` com seeds do alfabeto se vazia; reverte automaticamente tarefas no estado `"processing"` para `"pending"` ao reiniciar (auto-healing de crashes anteriores). Se a fila já contém tarefas, retoma de onde parou sem re-inserção.
- `getNextTask()` — `FindOneAndUpdate` atômico com filtro `{status: "pending"}` e ordenação composta `{priority: -1, _id: 1}`: prefixos de maior prioridade são processados primeiro; empates resolvidos lexicograficamente.
- `updateTaskStatus(id, status)` — atualiza status e `finished_at` de uma tarefa; usado para transitar para `"done"` ou reverter para `"pending"` em falha.
- Campo `priority int` no documento de tarefa `crawler_keywords`, com semântica definida por tipo de correspondência:
  - `priority=2` — filho sem hifén: correspondência genuína de substring, sem tokenização pelo ElasticSearch. Alta probabilidade de novos repositórios distintos.
  - `priority=1` — filho quando o pai encontrou repositórios novos (`newInPrefix > 0`).
  - `priority=0` — padrão (pai não encontrou novos repositórios).
  - `priority=-1` — filho de "token-match plateau" (depriorizados; processados por último, mas sem perda de dados).

**Detecção de token-match plateau e depriorização (`crawler/crawler.go`):**
- Condição de plateau: `newInPrefix == 0 && res.Count >= 10000 && strings.Contains(prefix, "-") && len(prefix) > 1`
- Semântica: o Docker Hub usa hifén como separador de token no ElasticSearch. Um prefixo com hifén retornando 10.000 resultados mas zero repositórios novos é uma correspondência de token completo sem novidade para o dataset. Aprofundar nessa direção com prioridade normal saturaria a fila com tarefas de baixo retorno.
- Ação: todos os filhos são inseridos com `priority=-1` em vez de `priority=0`. Depriorização sem remoção — os dados não se perdem, apenas ficam para o final da fila.
- Log explícito: `>>> DEPRIORITIZING [prefix]: token-match plateau (N results, 0 new). Children set to priority=-1.`

**Deduplicação de separadores consecutivos (`crawler/crawler.go`):**
- Se o último caractere do prefixo é `-` ou `_`, os filhos `-` e `_` são omitidos ao gerar a expansão.
- Motivação: o ElasticSearch do Docker Hub trata `--`, `-_`, `_-`, `__` como equivalentes ao separador simples. Gerar esses filhos causaria DFS de profundidade ilimitada retornando resultados idênticos ao pai, sem nenhum dado novo.

**Aquecimento de cache em RAM (`crawler/crawler.go`):**
- `PreloadExistingRepos()` — executa ao iniciar, antes de qualquer worker. Faz `Find` em `RepoColl` com projeção `{namespace, name}` e popula `seenRepos sync.Map` com todos os repositórios já persistidos.
- Impede chamadas à API e round-trips ao banco para repositórios já coletados em execuções anteriores. Deduplicação ocorre em microssegundos na RAM local em vez de sub-milissegundos via rede.
- Escala testada: 5,2 milhões de registros em ~300 MB de RAM. Log de progresso a cada 250.000 registros; log final com contagem e duração total.

**Logging de eficiência por prefixo (`crawler/crawler.go`):**
- `efficiency = (newInPrefix / pages×100) × 100%`
- Log ao final de cada tarefa: `[DONE] Prefix [prefix]: +N unique | Eff: X.X% | Found total: N`
- Métrica de diagnóstico: prefixos com eficiência próxima de 0% indicam regiões do espaço DFS já saturadas pelo cache.

### Modificado

**Fingerprint de navegador — identidade por conta (`crawler/auth_proxy.go`):**
- `globalUAPool`: pool de 7 User-Agents cobrindo Chrome 121/119 (Windows/Mac/Linux), Edge 121, Firefox 122, Safari 17.
- Cada conta recebe UA fixo no carregamento via round-robin: `acc.UserAgent = globalUAPool[i % len(globalUAPool)]`. A mesma conta sempre usa o mesmo UA em login, busca e manifests — coerência de identidade observável pelo servidor.
- `GetNextClient()` retorna `(*http.Client, token, ua)` — os três atributos da identidade propagados juntos para garantir que cada requisição pertença à mesma "sessão de navegador".
- Auto-login JWT protegido por `loginMu sync.Mutex` — previne login paralelo da mesma conta por múltiplos workers.
- `ClearToken(token)` zera o token da conta correspondente, forçando re-autenticação na próxima chamada a `GetNextClient`.

**Stack TLS e timeouts de rede (`crawler/auth_proxy.go`):**
- `TLSClientConfig{MinVersion: tls.VersionTLS12, PreferServerCipherSuites: false}` — emula o perfil de negociação TLS do Chrome (servidor escolhe a ordem das cifras).
- `TLSHandshakeTimeout = 5s`, `ResponseHeaderTimeout = 5s`, timeout total da requisição = 10s.
- HTTP/2 não configurado no transporte — deliberadamente evitado para prevenir bloqueio de workers por conexões half-open durante tarpit de rate limit (com HTTP/1.1 cada conexão é atômica e o timeout funciona deterministicamente).
- `MaxIdleConns = 100`, `IdleConnTimeout = 90s`, `MaxIdleConnsPerHost = 10` — pool de conexões TCP ativo.

**Semântica do código HTTP 404 (`crawler/crawler.go`):**
- HTTP 404 em `fetchPage` retorna `&V2SearchResponse{}` (vazio) com `ok=true` — não é falha.
- Interpretação: 404 indica que `starting_index` excede o total de resultados da query. O loop de paginação `for p := 2; p <= pages; p++` aborta naturalmente ao receber slice vazio em qualquer página.
- Antes desta versão, 404 causava re-enfileiramento da tarefa como `"pending"` ou cooldown de 30s — comportamento incorreto que desperdiçava tempo e re-processava tarefas válidas.

**Cooldown estendido para erros HTTP inesperados (`crawler/crawler.go`):**
- Códigos HTTP fora do conjunto `{200, 404, 401, 429, 403}`: log com os primeiros 200 bytes do body diagnóstico e cooldown de **4 minutos** antes de retornar falha.
- Antes desta versão o cooldown era de 30 a 90 segundos — insuficiente para o período de cache do WAF do Cloudflare em respostas de erro.

### Corrigido

- **DFS de profundidade infinita em separadores consecutivos:** prefixo terminado em `-` ou `_` gerava filhos `--`, `-_` cujos resultados eram idênticos ao pai, causando fan-out de profundidade ilimitada sem dados novos.
- **Terminação prematura de workers em fila temporariamente vazia:** workers encerravam após N falhas consecutivas de `getNextTask`, mesmo com tarefas genuinamente pendentes temporariamente claimed por outros workers no cluster. Corrigido: worker só encerra quando `CountDocuments({status: "pending"})` confirma zero tarefas restantes.
- **Saturação DFS em regiões densas:** sem priorização, prefixos de token-match acumulavam centenas de filhos que cobriam a mesma região lexical, bloqueando prefixos de alta utilidade de chegar à frente da fila.

---

## [2.0.0] — 2026-04-03

### Adicionado

**`crawler/` (implementação do Estágio I — stub sem `Run` no upstream):**
- `crawler/crawler.go`: `ParallelCrawler` com DFS recursivo sobre espaço de prefixos do Docker Hub. N workers independentes, cada um executando `crawlDFS` recursivamente a partir de suas seeds. Deduplicação em memória via `seenRepos sync.Map`. Aprofundamento forçado para prefixos de 1 caractere. Goroutine `repoWriter` com `BulkWrite` ao MongoDB a cada 2s ou 1.000 repositórios. Checkpointing post-order por keyword via coleção `crawler_keywords`.
- `crawler/auth_proxy.go`: `IdentityManager` — carrega contas Docker Hub de `accounts.json` e proxies de arquivo texto; auto-login JWT com `sync.Mutex`; rotação round-robin de identidades via `GetNextClient()`.

**`buildgraph/from_mongo.go` (Estágio II reengenhado):**
- Pipeline de três estágios desacoplados por buffered channels: Loader (MongoDB → `repoChan` buf 4.000), repoWorkers (`max(NumCPU×16, 64)`) e buildGraphWorkers (`max(NumCPU×4, 16)`).
- Semáforo `tagConcurrency=4` por repositório para controle de requisições paralelas de manifest.
- Checkpointing `graph_built_at` no MongoDB após conclusão de cada repositório.

**`myutils/mongo.go`:**
- `BulkUpsertRepositories`: bulk write não-ordenado com upsert por `{namespace, name}`.
- `KeywordsColl`, `IsKeywordCrawled`, `MarkKeywordCrawled`: sistema de checkpoint para o Estágio I.
- `MarkRepoGraphBuilt`: checkpoint para o Estágio II.
- Connection pool: `MaxPoolSize=100`, `MinPoolSize=5`, `MaxConnIdleTime=5min`.
- Timeout do ping inicial: `1s → 30s`.

**`myutils/neo4j.go`:**
- `InsertImageToNeo4j` reescrito: IDs de layer pré-computados localmente via SHA256 chain; toda a cadeia de layers inserida em uma única transação `ExecuteWrite` (O(1) round-trips por imagem, em vez de O(N layers)).

**`myutils/urls.go`:**
- `V2SearchURLTemplate` e `GetV2SearchURL` — API V2 do Docker Hub com `ordering=-pull_count`.

**`myutils/config.go`:**
- Override por variáveis de ambiente `MONGO_URI` e `NEO4J_URI`.
- Localização do config por `os.Getwd()` (compatível com `go run`).
- Neo4j opcional na inicialização.

**`myutils/docker_hub_api_requests.go`:**
- Keep-alives habilitados (conexões TCP/TLS reutilizadas entre requisições).
- Connection pool: `MaxIdleConns=300`, `MaxIdleConnsPerHost=50`, `IdleConnTimeout=90s`, `Timeout=30s`.

**`cmd/cmd.go`:**
- Subcomando `crawl` com flags `--workers`, `--seed`, `--shard`, `--shards`, `--accounts`, `--proxies`, `--config`.

**Infraestrutura:**
- `docker-compose.yml`: MongoDB, Neo4j, crawler.
- `docker-compose.node2.yml`: Node 2 apontando para MongoDB do Node 1.
- `automation/pipeline_autopilot.sh`: orquestração sequencial dos 3 estágios.
- `automation/test_e2e.sh`: teste de integração end-to-end.

### Corrigido

- **`myutils/neo4j.go` — `findLayerNodesByRawLayerDigestFunc` (crítico):** query Cypher usava `{id: $digest}` para matchar nó `RawLayer`, mas a propriedade armazenada é `digest`. A propriedade `id` não existe em `RawLayer`. A query nunca retornava resultados, quebrando silenciosamente o rastreamento de imagens upstream. Corrigido para `{digest: $digest}`.

- **`scripts/calculate_node_dependent_weights.go` (médio):** branch `if repoDoc.Namespace == "library"` continha `continue` como primeira instrução, tornando todo o código subsequente inalcançável. Imagens oficiais Docker eram ignoradas no cálculo de dependency weight. `continue` removido.

- **`myutils/docker_hub_api_requests.go` (médio):** `DisableKeepAlives: true` impedia reutilização de conexões TCP, acrescentando ~100–300ms de handshake+TLS por requisição desnecessariamente. Removido.

- **`automation/test_e2e.sh` (baixo):** sintaxe `[ [` inválida em bash substituída por `[ "$(expr)" -gt N ]`.

---

## [1.0.0] — baseline upstream (NSSL-SJTU/DITector)

Pipeline original com subcomando `crawl` declarado em `cmd/cmd.go` sem campo `Run` (stub sem implementação). Estágios II e III funcionais. Implementa: `buildgraph/build.go` (inserção síncrona por layer no Neo4j), `myutils/`, `scripts/`, `analyzer/`, `cmd/`, `main.go`.
