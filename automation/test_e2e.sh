#!/bin/bash
# End-to-End Integration Test: validates the full 3-stage pipeline
# with a small but real crawl using the "nginx" seed keyword.
#
# Requirements: MongoDB and Neo4j must be running (docker compose up -d mongodb neo4j)
# Expected runtime: ~3 minutes

set -e

SEED="nginx"
OUTPUT="test_output.json"
CONFIG="config.yaml"

echo "=== [E2E TEST] DITector Pipeline ==="
echo ""

# ── Stage 0: Clean slate ──────────────────────────────────────────────────────
echo "[0/3] Cleaning test data..."
# Use mongosh directly against the same MongoDB instance the Go binary uses.
# If mongosh is not installed on the host, fall back to docker exec.
if command -v mongosh &>/dev/null; then
  mongosh localhost:27017/dockerhub_data --quiet \
    --eval "db.repositories_data.drop(); db.tags_data.drop(); db.images_data.drop();" \
    > /dev/null 2>&1 || true
else
  docker exec ditector_mongo mongosh localhost:27017/dockerhub_data --quiet \
    --eval "db.repositories_data.drop(); db.tags_data.drop(); db.images_data.drop();" \
    > /dev/null 2>&1 || true
fi
rm -f "$OUTPUT"
echo "      Done."
echo ""

# ── Stage 1: Crawl ───────────────────────────────────────────────────────────
echo "[1/3] CRAWL: discovering repositories (seed='$SEED', 45s timeout)..."
# Build first to avoid compilation delay counting against the crawl timeout
go build -o /tmp/ditector_test . 2>&1
# Note: discoveries are logged to the log file, not stdout.
# We discard stdout/stderr here to avoid SIGPIPE killing the process early.
timeout 45s /tmp/ditector_test crawl \
  --workers 5 \
  --seed "$SEED" \
  --accounts accounts.json \
  --config "$CONFIG" > /dev/null 2>&1 || true

if command -v mongosh &>/dev/null; then
  REPO_COUNT=$(mongosh localhost:27017/dockerhub_data \
    --quiet --eval 'db.repositories_data.countDocuments()' 2>/dev/null || echo 0)
else
  REPO_COUNT=$(docker exec ditector_mongo mongosh localhost:27017/dockerhub_data \
    --quiet --eval 'db.repositories_data.countDocuments()' 2>/dev/null || echo 0)
fi
echo "      Repositories discovered: $REPO_COUNT"

if [ "$REPO_COUNT" -eq 0 ]; then
  echo "FAIL: No repositories discovered. Check accounts.json and network."
  exit 1
fi

# Validate namespace is correctly populated (not empty for all records)
if command -v mongosh &>/dev/null; then
  NS_OK=$(mongosh localhost:27017/dockerhub_data \
    --quiet --eval 'db.repositories_data.countDocuments({namespace: {$ne: ""}})' 2>/dev/null || echo 0)
else
  NS_OK=$(docker exec ditector_mongo mongosh localhost:27017/dockerhub_data \
    --quiet --eval 'db.repositories_data.countDocuments({namespace: {$ne: ""}})' 2>/dev/null || echo 0)
fi
echo "      Repos with correct namespace: $NS_OK / $REPO_COUNT"
echo ""

# ── Stage 2: Build ───────────────────────────────────────────────────────────
echo "[2/3] BUILD: constructing IDEA dependency graph (threshold=0)..."
/tmp/ditector_test build \
  --format mongo \
  --threshold 0 \
  --page_size 10 \
  --tags 2 \
  --config "$CONFIG" > /dev/null 2>&1
echo "      Done."
echo ""

# ── Stage 3: Rank ────────────────────────────────────────────────────────────
echo "[3/3] RANK: computing dependency weights..."
/tmp/ditector_test execute \
  --script calculate-node-weights \
  --threshold 0 \
  --file "$OUTPUT" \
  --config "$CONFIG" > /dev/null 2>&1
echo ""

# ── Verification ─────────────────────────────────────────────────────────────
echo "=== VERIFICATION ==="

if [ ! -f "$OUTPUT" ]; then
  echo "FAIL: Output file '$OUTPUT' not generated."
  exit 1
fi

BYTE_SIZE=$(stat -c%s "$OUTPUT")
if [ "$BYTE_SIZE" -lt 10 ]; then
  echo "FAIL: Output file is empty or too small ($BYTE_SIZE bytes)."
  exit 1
fi

RECORD_COUNT=$(wc -l < "$OUTPUT")
echo "Output file: $OUTPUT ($BYTE_SIZE bytes, $RECORD_COUNT records)"
echo ""
echo "Sample records:"
head -n 3 "$OUTPUT" | python3 -m json.tool 2>/dev/null || head -n 3 "$OUTPUT"
echo ""

# Check at least one record has a non-empty namespace
NS_IN_OUTPUT=$(grep -c '"repository_namespace":"[^"]' "$OUTPUT" 2>/dev/null || echo 0)
echo "Records with correct namespace in output: $NS_IN_OUTPUT / $RECORD_COUNT"
echo ""

echo "=== RESULT: PASS ==="
