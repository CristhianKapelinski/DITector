#!/bin/bash
# Teste de Integração End-to-End da Pipeline

echo "--- [TEST E2E] LIMPANDO DADOS DE TESTE ---"
docker exec -it ditector_mongo mongosh localhost:27017/dockerhub_data --eval "db.repositories_data.deleteMany({name: 'nginx'})" > /dev/null

echo "--- [TEST E2E] PASSO 1: CRAWL (DISCOVERY) ---"
timeout 20s go run main.go crawl --workers 2 --seed 'nginx' --accounts accounts.json --config config.yaml

echo "--- [TEST E2E] PASSO 2: BUILD (GRAPH) ---"
go run main.go build --format mongo --threshold 0 --page_size 2 --config config.yaml

echo "--- [TEST E2E] PASSO 3: RANK (EXPORT) ---"
rm -f test_output.json
go run main.go execute --script calculate-node-weights --threshold 0 --file test_output.json --config config.yaml

echo "--- [TEST E2E] VERIFICAÇÃO FINAL ---"
if [ -f "test_output.json" ] && [ [ $(stat -c%s test_output.json) -gt 10 ]; then
    echo "SUCESSO: Dataset gerado com $(cat test_output.json | wc -l) registros."
    head -n 3 test_output.json
else
    echo "ERRO: Falha ao gerar dataset de saída."
    exit 1
fi
