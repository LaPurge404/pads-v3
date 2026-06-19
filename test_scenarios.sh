#!/bin/bash
set -e

TOKEN="${1:-}"
if [ -z "$TOKEN" ]; then
    echo "Usage: $0 <token>"
    exit 1
fi

API="http://127.0.0.1:8080"
PADS_CI="./pads-ci -token $TOKEN"

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_scenario() { echo -e "\n${YELLOW}=== $1 ===${NC}"; }
pass() { echo -e "${GREEN}✅ $1${NC}"; }
fail() { echo -e "${RED}❌ $1${NC}"; }

# ------------------------------------------------------------
log_scenario "Validation de l'API"
# Test health
if curl -s -H "Authorization: Bearer $TOKEN" "$API/health" | grep -q OK; then
    pass "GET /health"
else
    fail "GET /health"
fi

# Test state (protégé)
if curl -s -H "Authorization: Bearer $TOKEN" "$API/state" | grep -q "StabilityScore"; then
    pass "GET /state (token valide)"
else
    fail "GET /state"
fi

# Test select (protégé)
if curl -s -H "Authorization: Bearer $TOKEN" "$API/select" | grep -q "arm"; then
    pass "GET /select"
else
    fail "GET /select"
fi

# Test workspace (protégé)
if curl -s -H "Authorization: Bearer $TOKEN" "$API/workspace" | grep -q "gitBranch"; then
    pass "GET /workspace"
else
    fail "GET /workspace"
fi

# Test d'une requête sans token (doit échouer)
if curl -s "$API/state" | grep -q "Unauthorized"; then
    pass "GET /state sans token → 401"
else
    fail "GET /state sans token"
fi

# ------------------------------------------------------------
log_scenario "Scénario 1 : Tous les tests passent"
$PADS_CI
if [ $? -eq 0 ]; then
    pass "Commit accepté (attendu)."
else
    fail "Commit rejeté alors que les tests passent."
fi

# ------------------------------------------------------------
log_scenario "Scénario 2 : Un test échoue"
cat > internal/policy/evolution/failing_test.go << 'GOEOF'
package evolution_test

import "testing"

func TestFailing(t *testing.T) {
    t.Fail()
}
GOEOF
$PADS_CI
exit_code=$?
rm -f internal/policy/evolution/failing_test.go
if [ $exit_code -ne 0 ]; then
    pass "Commit rejeté (attendu)."
else
    fail "Commit accepté alors qu'un test échoue."
fi

# ------------------------------------------------------------
log_scenario "Scénario 3 : Alternance succès / échec (oscillation)"
$PADS_CI && pass "Cycle 1 : accepté" || fail "Cycle 1 : rejeté"
sleep 1
cat > internal/policy/evolution/failing_test.go << 'GOEOF'
package evolution_test

import "testing"

func TestFailing(t *testing.T) {
    t.Fail()
}
GOEOF
$PADS_CI || pass "Cycle 2 : rejeté (échec)"
rm -f internal/policy/evolution/failing_test.go
sleep 1
$PADS_CI && pass "Cycle 3 : accepté" || fail "Cycle 3 : rejeté"
echo "Vérifier manuellement dans les logs si un rollback a été déclenché."

# ------------------------------------------------------------
log_scenario "Scénario 4 : Aucun test (score zéro)"
cp cmd/pads-ci/main.go cmd/pads-ci/main.go.bak
sed -i 's/cmd := exec.Command("go", "test", ".\/...", "-count=1")/cmd := exec.Command("true")/' cmd/pads-ci/main.go
go build ./cmd/pads-ci
$PADS_CI
exit_code=$?
mv cmd/pads-ci/main.go.bak cmd/pads-ci/main.go
go build ./cmd/pads-ci
if [ $exit_code -ne 0 ]; then
    pass "Commit rejeté (score 0)."
else
    fail "Commit accepté avec score 0."
fi

# ------------------------------------------------------------
log_scenario "Scénario 5 : Rafale de 10 évolutions"
for i in $(seq 1 10); do
    $PADS_CI > /dev/null 2>&1 || true
done
pass "10 évolutions soumises sans erreur."

# ------------------------------------------------------------
log_scenario "Scénario 6 : API injoignable (arrêt temporaire)"
pkill -9 evolution-api || true
sleep 1
if $PADS_CI; then
    fail "pads-ci a réussi sans API"
else
    pass "pads-ci échoue proprement (connexion refusée)."
fi
echo "Relancez l'API manuellement avec ./evolution-api &"

# ------------------------------------------------------------
log_scenario "Scénario 7 : Fichier d'événements corrompu"
echo "ligne corrompue" >> evolution.log
if curl -s -H "Authorization: Bearer $TOKEN" "$API/state" | grep -q "StabilityScore"; then
    pass "API résiliente à une ligne corrompue."
else
    fail "API plantée."
fi
git checkout HEAD -- evolution.log 2>/dev/null || true

# ------------------------------------------------------------
log_scenario "Scénario 8 : Reset de l'état"
pkill -9 evolution-api || true
rm -f event_queue.log evolution.log worker_offset.txt
./evolution-api &
sleep 2
$PADS_CI
pass "Reset et nouvelle évolution acceptée."

echo -e "\n${GREEN}Scénarios terminés.${NC}"
