# Guia de Operação: Pipeline DITector Research

Este guia descreve o fluxo completo para gerar o dataset de 100.000 containers priorizados.

## 1. Pré-requisitos
*   **Docker & Docker Compose** instalados.
*   **Contas Docker Hub:** Criar o arquivo `accounts.json` na raiz:
    ```json
    [{"username": "seu_user", "password": "seu_password"}]
    ```

## 2. Setup do Ambiente
Suba a infraestrutura de banco de dados (MongoDB e Neo4j):
```bash
docker compose up -d mongodb neo4j
```

## 3. Execução da Pipeline (Os 3 Estágios)

### Estágio I: Descoberta (Crawl)
Inicie a descoberta de repositórios. Use diferentes `SEED` para cada máquina:
```bash
SEED=a go run main.go crawl --workers 20 --seed $SEED --accounts accounts.json
```

### Estágio II: Extração de Layers e Filtro de Rede (Build)
Após coletar uma massa de dados, processe-os para construir o grafo de dependências. O sistema filtrará automaticamente containers de rede:
```bash
go run main.go build --format mongo --threshold 1000 --page_size 50
```

### Estágio III: Ranqueamento e Exportação (Rank)
Gere o dataset final priorizado por Pull Count e Dependency Weight:
```bash
go run main.go execute --script calculate-node-weights --threshold 1000 --file dataset_final.json
```

## 4. Automação (Autopilot)
Para rodar tudo de forma sequencial automática:
```bash
./automation/pipeline_autopilot.sh "a"
```
