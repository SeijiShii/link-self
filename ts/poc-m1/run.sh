#!/usr/bin/env bash
# M1 interop PoC runner: starts the Go echo/relay/leaf nodes, then runs the
# js-libp2p client against them. Exits 0 iff the full round-trips pass.
set -euo pipefail
cd "$(dirname "$0")"

CORE_DIR=../../core
OUT=$(mktemp -d)
trap 'kill $GO_PID 2>/dev/null || true; rm -rf "$OUT"' EXIT

echo "[run] building Go PoC node..."
(cd "$CORE_DIR" && go build -o "$OUT/poc-wsnode" ./cmd/poc-wsnode)

echo "[run] starting Go nodes (echo + relay + leaf)..."
"$OUT/poc-wsnode" -relay > "$OUT/nodes.jsonl" 2> "$OUT/go.log" &
GO_PID=$!

# Wait for all three node-info lines.
for i in $(seq 1 50); do
  [ "$(wc -l < "$OUT/nodes.jsonl" 2>/dev/null || echo 0)" -ge 3 ] && break
  sleep 0.2
done

ECHO_ADDR=$(node -e "const l=require('fs').readFileSync('$OUT/nodes.jsonl','utf8').trim().split('\n').map(JSON.parse); console.log(l.find(x=>x.role==='echo').wsAddr)")
CIRCUIT_ADDR=$(node -e "const l=require('fs').readFileSync('$OUT/nodes.jsonl','utf8').trim().split('\n').map(JSON.parse); console.log(l.find(x=>x.role==='leaf').circuitVia)")

echo "[run] echo addr:    $ECHO_ADDR"
echo "[run] circuit addr: $CIRCUIT_ADDR"

# Give the leaf a moment to complete its relay reservation.
sleep 1

npx tsx src/poc.ts "$ECHO_ADDR" "$CIRCUIT_ADDR"
STATUS=$?

echo "[run] --- go node log ---"
cat "$OUT/go.log"
exit $STATUS
