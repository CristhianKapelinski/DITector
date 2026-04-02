# Metodologia de Pesquisa: Análise de Segurança em Larga Escala

## 1. Escopo da Pesquisa
Esta metodologia visa a seleção qualificada de 100.000 containers do Docker Hub para submissão a scans dinâmicos de rede (OpenVAS), priorizando o impacto sistêmico e a popularidade.

## 2. Fase I: Descoberta Exaustiva (DFS Crawling)
Devido à ausência de uma listagem pública exaustiva no Docker Hub, aplicamos o algoritmo **Depth-First Search (DFS)** sobre o espaço de nomes de repositórios.
*   **Algoritmo:** O sistema realiza buscas recursivas por prefixos alfabéticos. Se uma query (ex: "a") retorna > 10.000 resultados, o crawler aprofunda para "aa", "ab", etc., até que a amostragem seja capturável pela paginação da API.
*   **Distribuição:** A estratégia *Meet-in-the-Middle* permite que múltiplas instâncias dividam o alfabeto, acelerando a descoberta total.

## 3. Fase II: Extração de Infraestrutura e Grafo IDEA
A construção do grafo **IDEA (Image DEpendency grAph)** no Neo4j permite mapear a cadeia de suprimentos de containers.
*   **Extração de Camadas:** Para cada container descoberto, recuperamos os *hashes* (SHA256) de suas camadas constituintes.
*   **Relacionamento Base-Filha:** Uma aresta de dependência é criada entre dois containers que compartilham camadas idênticas em sua pilha base.
*   **Cálculo de Peso (Impacto):** O "Dependency Weight" de um container é definido pelo número de nós descendentes (downstream) no grafo. Um container com alto peso indica que vulnerabilidades nele presentes se propagam para uma vasta gama de outros serviços.

## 4. Fase III: Filtragem de Serviços de Rede (Heurísticas)
Para otimizar o tempo de execução do OpenVAS, aplicamos um filtro de rede duplo no estágio de processamento:
1.  **Declaração Explícita:** Verificação da instrução `EXPOSE` nos metadados de configuração da imagem.
2.  **Identificação Heurística:** Seleção baseada em palavras-chave no nome do repositório (ex: *nginx, server, sql, api, redis, proxy*).

## 5. Fase IV: Ranqueamento e Geração do Dataset
O dataset final de 100.000 containers é gerado através de uma função de pontuação composta:
$$S = w_1 \cdot \text{PullCount} + w_2 \cdot \text{DependencyWeight}$$
Onde $w_1$ e $w_2$ são pesos atribuídos conforme o foco da pesquisa (Popularidade vs. Impacto na Cadeia).
