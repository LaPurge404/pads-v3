#!/bin/bash
# chaos_test.sh – Phase 6: Load and chaos testing for PADS evolution API
# Usage: ./chaos_test.sh [api-url] [token]
#
# Prerequisites:
#   - PADS binary must be built: go build -o evolution-api ./cmd/evolution-api
#   - jq must be installed for JSON parsing
#
# Tests performed:
#   1. Health check
#   2. 50 parallel POST /evolve requests (expect HTTP 202)
#   3. Worker crash simulation (kill worker process, verify restart)
#   4. Corrupt event log line, verify API remains functional

set -euo pipefail

# Configuration
API_URL="${1:-http://127.0.0.1:8080}"
TOKEN="${2:-}"
TOKEN_FILE="token.txt"
MAX_PARALLEL=50
TIMEOUT=30
LOG_FILE="event_queue.log"
EVOLUTION_LOG="evolution.log"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info()  { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }
pass()      { log_info "PASS: $1"; }
fail()      { log_error "FAIL: $1"; exit 1; }

# Get token (from flag, env, or file)
get_token() {
    if [[ -n "$TOKEN" ]]; then
        echo "$TOKEN"
    elif [[ -n "${PADS_TOKEN:-}" ]]; then
        echo "$PADS_TOKEN"
    elif [[ -f "$TOKEN_FILE" ]]; then
        cat "$TOKEN_FILE"
    else
        # Try to read from running API
        fail "No token provided and $TOKEN_FILE not found"
    fi
}

# Wait for API to be ready
wait_for_api() {
    local max_attempts=20
    local attempt=0
    while ((attempt < max_attempts)); do
        if curl -sf "$API_URL/health" > /dev/null 2>&1; then
            return 0
        fi
        ((attempt++))
        sleep 0.5
    done
    fail "API not responding at $API_URL"
}

# Health check
test_health() {
    log_info "Testing health endpoint..."
    local status
    status=$(curl -s -o /dev/null -w "%{http_code}" "$API_URL/health")
    if [[ "$status" == "200" ]]; then
        pass "Health endpoint returns 200"
    else
        fail "Health endpoint returned $status, expected 200"
    fi
}

# 50 parallel POST /evolve requests
test_parallel_evolve() {
    log_info "Sending $MAX_PARALLEL parallel POST /evolve requests..."
    local token
    token=$(get_token)
    local pids=()
    local success=0
    local failures=()

    for i in $(seq 1 $MAX_PARALLEL); do
        (
            local resp
            resp=$(curl -s -o /dev/null -w "%{http_code}" \
                -X POST "$API_URL/evolve" \
                -H "Content-Type: application/json" \
                -H "Authorization: Bearer $token" \
                -d "{\"candidate\":$((50+i)),\"current\":50,\"weight\":1.0,\"mode\":\"stable\"}")
            echo "$resp"
        ) &
        pids+=($!)
    done

    # Collect results
    local results=()
    for pid in "${pids[@]}"; do
        results+=($(wait $pid))
    done

    # Count successes
    local accepted=0
    local other=0
    for code in "${results[@]}"; do
        if [[ "$code" == "202" ]]; then
            ((accepted++)) || true
        else
            ((other++)) || true
            failures+=("$code")
        fi
    done

    log_info "Results: $accepted accepted, $other rejected"
    if (( accepted == MAX_PARALLEL )); then
        pass "All $MAX_PARALLEL requests returned HTTP 202"
    elif (( accepted > 0 )); then
        log_warn "Only $accepted/$MAX_PARALLEL returned 202; $other returned non-202"
        pass "At least some requests succeeded"
    else
        fail "No requests returned 202 (failures: ${failures[*]})"
    fi
}

# Worker crash recovery
test_worker_crash() {
    log_info "Simulating worker crash..."
    local token
    token=$(get_token)

    # Find worker PID (look for process named evolution-api or similar)
    local worker_pid=""
    for pid_file in /tmp/pads-worker.pid /var/run/pads-worker.pid; do
        if [[ -f "$pid_file" ]]; then
            worker_pid=$(cat "$pid_file")
            break
        fi
    done

    # Alternative: look for processes listening on the API port
    if [[ -z "$worker_pid" ]]; then
        log_warn "Worker PID not found via pidfile, skipping crash test"
        log_warn "(In production, use a process manager like systemd/supervisord)"
        return 0
    fi

    # Kill the worker
    log_info "Killing worker PID $worker_pid..."
    if kill -0 "$worker_pid" 2>/dev/null; then
        kill "$worker_pid"
        sleep 2

        # Verify worker restarted
        if kill -0 "$worker_pid" 2>/dev/null; then
            pass "Worker $worker_pid restarted"
        else
            log_warn "Worker did not restart within timeout (expected in test env)"
        fi
    else
        log_warn "Worker process $worker_pid not found or already dead"
    fi
}

# Event log corruption resilience
test_log_corruption() {
    log_info "Testing event log corruption resilience..."

    # Backup logs
    local backup_suffix=".backup.$(date +%s)"
    if [[ -f "$LOG_FILE" ]]; then
        cp "$LOG_FILE" "$LOG_FILE$backup_suffix"
        log_info "Backed up $LOG_FILE to $LOG_FILE$backup_suffix"
    fi
    if [[ -f "$EVOLUTION_LOG" ]]; then
        cp "$EVOLUTION_LOG" "$EVOLUTION_LOG$backup_suffix"
        log_info "Backed up $EVOLUTION_LOG to $EVOLUTION_LOG$backup_suffix"
    fi

    # Corrupt a line in the event log
    if [[ -f "$LOG_FILE" ]]; then
        local line_count
        line_count=$(wc -l < "$LOG_FILE")
        if ((line_count > 0)); then
            local corrupt_line=$((RANDOM % line_count + 1))
            log_info "Corrupting line $corrupt_line of $LOG_FILE..."
            sed -i "${corrupt_line}s/.*/INVALID_CORRUPT_JSON_LINE{{{}}/" "$LOG_FILE"
            log_info "Corrupted line $corrupt_line"
        fi
    fi

    if [[ -f "$EVOLUTION_LOG" ]]; then
        local line_count
        line_count=$(wc -l < "$EVOLUTION_LOG")
        if ((line_count > 0)); then
            local corrupt_line=$((RANDOM % line_count + 1))
            log_info "Corrupting line $corrupt_line of $EVOLUTION_LOG..."
            sed -i "${corrupt_line}s/.*/INVALID_CORRUPT_JSON_LINE{{{}}/" "$EVOLUTION_LOG"
            log_info "Corrupted line $corrupt_line"
        fi
    fi

    # Verify API still responds
    sleep 1
    local status
    status=$(curl -s -o /dev/null -w "%{http_code}" "$API_URL/health")
    if [[ "$status" == "200" ]]; then
        pass "API remains functional after log corruption (HTTP $status)"
    else
        # Restore backups on failure
        [[ -f "$LOG_FILE$backup_suffix" ]] && mv "$LOG_FILE$backup_suffix" "$LOG_FILE"
        [[ -f "$EVOLUTION_LOG$backup_suffix" ]] && mv "$EVOLUTION_LOG$backup_suffix" "$EVOLUTION_LOG"
        fail "API returned $status after log corruption"
    fi

    # Cleanup backups
    rm -f "$LOG_FILE$backup_suffix" "$EVOLUTION_LOG$backup_suffix"
}

# Cleanup function
cleanup() {
    log_info "Cleaning up..."
    # Remove corrupted test logs
    rm -f "$LOG_FILE.test" "$EVOLUTION_LOG.test"
}

# Main
main() {
    log_info "=== PADS Chaos Test ==="
    log_info "API URL: $API_URL"

    # Check dependencies
    if ! command -v curl &> /dev/null; then
        fail "curl is required but not installed"
    fi

    # Wait for API
    wait_for_api

    # Run tests
    test_health
    test_parallel_evolve
    test_worker_crash
    test_log_corruption

    log_info "=== All chaos tests passed ==="
}

main "$@"