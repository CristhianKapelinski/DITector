# Metodologia de Pesquisa — Análise de Segurança em Larga Escala do Ecossistema Docker Hub

**Base científica:** Hequan Shi et al., "Dr. Docker: A Large-Scale Security Measurement of Docker Image Ecosystem", WWW '25, NSSL-SJTU.

---

## 1. Escopo e Objetivos

Esta metodologia visa a seleção qualificada de containers do Docker Hub para submissão a scans dinâmicos de rede via OpenVAS. A seleção é orientada por dois critérios de impacto sistêmico:

- **Pull Count:** indicativo de popularidade e amplitude de implantação. Vulnerabilidades em imagens de alto pull count afetam diretamente uma base ampla de infraestruturas.
- **Dependency Weight (Out-Degree no grafo IDEA):** número de imagens downstream que herdam layers desta imagem. Uma vulnerabilidade em uma imagem base propaga-se pela cadeia de suprimentos para todas as imagens derivadas — o impacto é multiplicado pelo peso de dependência.

O objetivo não é a enumeração exaustiva do Docker Hub, mas a identificação dos containers com maior potencial de impacto em segurança para priorização de scans.

---

## 2. Fase I — Descoberta Exaustiva (DFS Crawling)

### 2.1. Problema: Ausência de Listagem Pública

O Docker Hub não disponibiliza uma listagem pública exaustiva de seus repositórios. A API de busca (`GET /v2/search/repositories/`) retorna no máximo 10.000 resultados por query, com paginação de até 100 resultados por página.

### 2.2. Solução: DFS sobre Espaço de Prefixos

Aplica-se o algoritmo **Depth-First Search (DFS)** sobre o espaço de prefixos alfabéticos, recursivamente:

```
Se count(keyword) >= 10.000 → aprofundar: enfileirar keyword+a, keyword+b, ..., keyword+_ (38 chars)
Se count(keyword) < 10.000  → coletar: scrape de todas as páginas disponíveis
```

O aprofundamento é forçado para prefixos de 1 caractere independente da contagem reportada. A API do Docker Hub aceita queries de 1 caractere, mas o motor ElasticSearch subjacente trata esses termos como stopwords, retornando contagens artificialmente baixas. Sem o aprofundamento forçado, a árvore DFS seria podada prematuramente nesses nós, resultando na perda de toda a sub-árvore correspondente.

### 2.3. Representação de Nomes de Repositório

O Docker Hub organiza repositórios em dois níveis: `namespace/name`. A API V2 retorna o campo `repo_name` em dois formatos:

| Tipo | `repo_name` na API | Namespace canônico | Nome |
|------|--------------------|--------------------|------|
| Imagem oficial | `"nginx"` | `library` | `nginx` |
| Imagem community | `"cimg/postgres"` | `cimg` | `postgres` |

O campo `repo_owner` retornado pela API está sempre vazio e não deve ser utilizado. O namespace é extraído exclusivamente do `repo_name` via `parseRepoName()`:

```go
func parseRepoName(repoName string) (namespace, name string) {
    parts := strings.SplitN(repoName, "/", 2)
    if len(parts) == 2 {
        return parts[0], parts[1]
    }
    return "library", repoName
}
```

Para geração do nome de pull a partir do dataset exportado:

```python
ns  = record["repository_namespace"]
img = record["repository_name"]
tag = record["tag_name"]
image_ref = f"{img}:{tag}" if ns == "library" else f"{ns}/{img}:{tag}"
```

Imagens `library/` seguem a convenção Docker: `docker pull nginx:latest` equivale a `docker pull library/nginx:latest`. Para imagens community, o namespace é obrigatório no comando `pull`.

### 2.4. Saída do Estágio I

MongoDB, coleção `repositories_data`: um documento por repositório com `namespace`, `name` e `pull_count`.

---

## 3. Fase II — Construção do Grafo IDEA

### 3.1. Escopo de Processamento

A Fase II processa **todos os repositórios** com `pull_count ≥ threshold`, sem filtro heurístico por nome. O objetivo é a cobertura completa do ecossistema Docker Hub para análise de dependências: repositórios que não expõem portas de rede são igualmente relevantes como imagens base (upstream) no grafo IDEA.

O filtro por relevância de segurança é aplicado a posteriori, na Fase III, via Dependency Weight: repositórios que nada depende e que não expõem portas relevantes obtêm score baixo e são naturalmente despriorizados no dataset final.

### 3.2. Construção do Grafo IDEA (Image DEpendency grAph)

Para cada repositório, o sistema consulta a API do Docker Hub para obter a tag mais recentemente atualizada e a tag `latest` (se diferente), e para cada tag os metadados de imagem (lista de layers com digest, instrução e tamanho) de todas as plataformas disponíveis.

O grafo modela herança entre imagens através de um esquema de hashing de layers. Para cada layer i na pilha de uma imagem:

**Content layer** (possui digest SHA256 do conteúdo):
```
dig_i      = SHA256(layer_i.digest)
Layer_i.id = SHA256(Layer_{i-1}.id || dig_i)
```

**Config layer** (instrução Dockerfile sem conteúdo físico):
```
dig_i      = SHA256(layer_i.instruction)
Layer_i.id = SHA256(Layer_{i-1}.id || dig_i)
```

O bottom layer usa `Layer_{-1}.id = ""`. Todos os IDs são computados localmente antes de qualquer comunicação com o Neo4j.

**Propriedade fundamental:** se duas imagens compartilham as mesmas N primeiras layers na mesma ordem, seus `Layer_N.id` serão idênticos. Relações de herança são identificáveis por igualdade de ID de nó — sem comparação de conteúdo.

### 3.3. Saída do Estágio II

Neo4j com nós `Layer`, nós `RawLayer` e arestas `IS_BASE_OF` (herança entre layers consecutivas) e `IS_SAME_AS` (associação layer→conteúdo físico).

---

## 4. Fase III — Ranqueamento e Geração do Dataset

### 4.1. Dependency Weight (Out-Degree)

O **Dependency Weight** de uma imagem é o Out-Degree do nó `Layer` correspondente à sua última camada no grafo IDEA — ou seja, o número de imagens filhas que herdam diretamente desta imagem. Imagens com alto Out-Degree são imagens base amplamente utilizadas; vulnerabilidades nelas se propagam pela cadeia de suprimentos.

O paper Dr. Docker define dois conjuntos de imagens de alto impacto:

| Conjunto | Critério | Qtd. no paper |
|----------|----------|---------------|
| High-Pull-Count | Pull count ≥ 1.000.000, top 3 tags mais recentes | 20.673 imagens |
| High-Dependency-Weight | Dependency weight ≥ 10 | 25.924 imagens |

### 4.2. Dataset de Saída

O dataset exportado (`final_prioritized_dataset.json`) é um arquivo JSONL (um registro JSON por linha):

```json
{
  "repository_namespace": "library",
  "repository_name": "nginx",
  "tag_name": "latest",
  "image_digest": "sha256:...",
  "weights": 1847,
  "downstream_images": ["user1/app:latest", "user2/service:v2"]
}
```

A priorização para scan é feita por ordenação sobre os campos `weights` (impacto na cadeia de suprimentos) e `pull_count` (popularidade direta). Não há função de score composta com pesos explícitos — a escolha dos critérios de ordenação é deixada ao pesquisador conforme o foco da análise.

### 4.3. Checkpointing entre Estágios

| Estágio | Mecanismo | Campo |
|---------|-----------|-------|
| I | MongoDB coleção `crawler_keywords` | `_id = keyword`, `crawled_at` |
| II | Campo no documento de repositório | `graph_built_at: <RFC3339>` |

Em caso de interrupção e reinício, cada estágio retoma de onde parou sem reprocessar itens já concluídos.

---

## 5. Integração com OpenVAS

O dataset final alimenta um scanner OpenVAS externo. O fluxo esperado por imagem:

1. `docker pull <namespace>/<name>:<tag>` (ou `<name>:<tag>` para `library/`)
2. `docker run -d --name scan_target <image>`
3. `docker inspect scan_target` → extração do IP do container
4. Scan OpenVAS com target = IP do container
5. Coleta do relatório; `docker rm -f scan_target`

O pré-filtro heurístico do Estágio II reduz a proporção de containers sem serviços de rede no dataset de saída, diminuindo o número de tentativas de scan sem resultado para o scanner externo.

---

## 6. Achados do Paper de Referência (Dr. Docker, WWW '25)

| Métrica | Valor reportado |
|---------|----------------|
| Imagens com vulnerabilidades conhecidas (CVE) | 93,7% |
| Imagens com secret leakage | 4.437 |
| Imagens com misconfigurações de serviço | 50 |
| Imagens maliciosas (crypto miners: XMR, PKT, CRP) | 24 |
| Imagens downstream afetadas por maliciosas (supply chain) | 334 |
| High-Pull-Count (≥1M pulls, top 3 tags) | 20.673 |
| High-Dependency-Weight (Out-Degree ≥ 10) | 25.924 |
