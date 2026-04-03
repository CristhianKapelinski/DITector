# Crawler Production Results & Final Architecture

**Data:** 04/04/2026
**Objetivo:** Registro empírico da estabilização e dos resultados de produção do cluster de extração (Node 1 + Node 2) operando na Fase 1 (Discovery) do DITector.

---

## 1. Arquitetura Definitiva (Python-Mimic & High-Throughput)

Após extensos testes empíricos e identificação de gargalos (como *Half-Open Sockets* no HTTP/2 e bloqueios de I/O no MongoDB), a arquitetura final consolidou-se nas seguintes diretrizes:

### 1.1. Scraping Sequencial Cadenciado (Anti-429)
*   A tentativa de paralelizar páginas de uma mesma *keyword* saturava as conexões TCP, resultando em bloqueios silenciosos pelo balanceador do Docker Hub.
*   **A Solução:** Adotou-se o modelo do script Python original. Cada *Worker* processa as páginas de 1 a 100 de forma **sequencial** com um *delay* de 200ms entre requisições. 
*   **Paralelismo Real:** O paralelismo é alcançado horizontalmente: `W=3` no Node 1 e `W=4` no Node 2, garantindo 7 fluxos simultâneos ininterruptos que não engatilham rate limits severos.

### 1.2. Deduplicação em RAM $O(1)$
*   **O Problema:** A árvore DFS inerentemente gera sobreposição (ex: resultados de `aa` já estavam em `a`). Deixar o MongoDB lidar com toda a deduplicação via `Upsert` saturava a rede.
*   **A Solução:** Implementado um `seenRepos sync.Map` na memória do Crawler. Repositórios repetidos são descartados instantaneamente em RAM antes de qualquer I/O de rede ou banco de dados.

### 1.3. Bypass do ElasticSearch (1-Char Deepening)
*   **O Edge Case Crítico:** Buscas de 1 caractere (ex: `t`, `w`) acionam regras de *Min-Gram/Stopwords* no ElasticSearch do Docker Hub, que retorna contagens falsamente baixas (ex: 500 resultados). Isso podaria a árvore prematuramente, perdendo milhões de imagens.
*   **A Solução:** O código foi alterado para forçar o aprofundamento (DFS *Fan-out*) em todos os prefixos de 1 caractere (`len(prefix) == 1`), ignorando a contagem relatada pelo servidor e garantindo que ramificações densas (ex: `ubuntu`, `kapelinsky`) sejam eventualmente alcançadas.

### 1.4. Liveness e Timeouts Rígidos
*   **A Solução:** Todos os métodos de comunicação com o MongoDB (`BulkWrite`, `UpdateOne`, `CountDocuments`) receberam `context.WithTimeout` de 10 a 30 segundos. Isso previne que instabilidades de rede entre o Nó 2 e o Nó 1 causem *deadlocks* infinitos nas goroutines.

---

## 2. Resultados Empíricos (Validação de Produção)

Durante a janela de observação em ambiente de produção distribuído (Node 1 cobrindo o Shard 0/2 e Node 2 cobrindo o Shard 1/2), os seguintes marcos foram atingidos e validados diretamente no MongoDB:

1.  **Crescimento Sustentado:** O banco de dados registrou a inserção de mais de **100.000 repositórios únicos e inéditos em um intervalo inferior a 10 minutos** de operação conjunta dos dois nós.
2.  **Marca Histórica:** O contador total ultrapassou **750.000 repositórios únicos** (`751.149`) de forma limpa, garantida pelo índice único `{namespace: 1, name: 1}` do MongoDB.
3.  **Projeção Científica:** Com os nós operando sem bloqueios em ramificações profundas da árvore (ex: `cache-aaaaaa`, `ubuntu-aaaa`), a vazão projetada ultrapassa a marca de **11 Milhões de repositórios por dia** (mais que o dobro da meta original de 5M/dia).

**Conclusão:** O sistema atingiu a maturidade arquitetural e de rede. A Fase 1 opera atualmente no limite ótimo de extração tolerado pela API do Docker Hub.
