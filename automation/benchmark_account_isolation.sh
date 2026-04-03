#!/bin/bash
# benchmark_account_isolation.sh — Testa 1 worker por conta com PCs altos.
#
# Uso: bash automation/benchmark_account_isolation.sh

set -e
cd "$(dirname "$0")/.."

MEASURE_SECS=180
COMPILE_WAIT=45
RESULTS_FILE="benchmark_isolation_results_$(date +%Y%m%d_%H%M%S).txt"
ACCOUNTS_COUNT=$(jq length accounts_node1.json)

mongo_count() {
    docker exec ditector_mongo mongosh --quiet \
        --eval 'db.getSiblingDB("dockerhub_data").repositories_data.countDocuments()' 2>/dev/null || echo 0
}

wait_for_crawler() {
    local max=120
    local i=0
    echo -n "    Aguardando crawler..."
    while ! docker logs ditector_crawler 2>&1 | grep -q "Connect to MongoDB"; do
        sleep 3; i=$((i+3))
        if [ $i -ge $max ]; then echo " timeout"; return 1; fi
        echo -n "."
    done
    echo " OK"
}

run_config() {
    local label="$1"
    local workers="$2"
    local page_conc="$3"

    echo ""
    echo "========================================"
    echo "ISOLATION: $label  (W=$workers per account, PC=$page_conc)"
    echo "========================================"

    docker-compose stop crawler 2>/dev/null
    docker-compose rm -f crawler 2>/dev/null
    WORKERS=$workers PAGE_CONCURRENCY=$page_conc docker-compose up -d crawler 2>/dev/null

    echo "    Iniciando..."
    sleep $COMPILE_WAIT
    wait_for_crawler || { echo "    ERRO: crawler não iniciou"; return; }
    sleep 15

    local t0=$(date +%s)
    local c0=$(mongo_count)
    echo "    [t=0s] repos: $c0"

    for i in 60 120 180; do
        sleep 60
        local c=$(mongo_count)
        local elapsed=$(( $(date +%s) - t0 ))
        local added=$((c - c0))
        local rate=$(echo "scale=1; $added * 60 / $elapsed" | bc 2>/dev/null || echo "?")
        echo "    [t=${elapsed}s] repos: $c  (+${added})  taxa: ${rate} repos/min"
    done

    local c1=$(mongo_count)
    local total_added=$((c1 - c0))
    local rate_avg=$(echo "scale=1; $total_added * 60 / $MEASURE_SECS" | bc 2>/dev/null || echo "?")

    echo "    RESULTADO: +${total_added} repos em ${MEASURE_SECS}s = ${rate_avg} repos/min"
    echo "$label | W=$workers | PC=$page_conc | repos/min=$rate_avg" >> "$RESULTS_FILE"
}

echo "Benchmark Account Isolation DITector" > "$RESULTS_FILE"
echo "Data: $(date)" >> "$RESULTS_FILE"
echo "Total Accounts: $ACCOUNTS_COUNT" >> "$RESULTS_FILE"
echo "" >> "$RESULTS_FILE"

# Testa 1 worker para cada conta disponível no node1 (3 no total)
run_config "W3_PC20"  $ACCOUNTS_COUNT  20
run_config "W3_PC40"  $ACCOUNTS_COUNT  40
run_config "W3_PC60"  $ACCOUNTS_COUNT  60
run_config "W3_PC80"  $ACCOUNTS_COUNT  80

echo ""
echo "========================================"
echo "RESULTADOS FINAIS DE ISOLATION"
echo "========================================"
cat "$RESULTS_FILE"
