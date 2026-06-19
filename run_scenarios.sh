#!/bin/bash

# Nettoyage
pkill -9 evolution-api 2>/dev/null || true
sleep 1

# Lancement de l'API avec logs capturés
LOG=$(mktemp)
./evolution-api > "$LOG" 2>&1 &
API_PID=$!

# Attente que l'API démarre (max 10 secondes)
for i in $(seq 1 10); do
    if grep -q "API addr=127.0.0.1:8080" "$LOG"; then
        break
    fi
    sleep 1
done

# Extraction du token
TOKEN=$(grep "Token généré" "$LOG" | head -1 | sed 's/.*token=//')
if [ -z "$TOKEN" ]; then
    echo "Impossible d'extraire le token. Vérifiez $LOG"
    kill $API_PID
    exit 1
fi
echo "Token : $TOKEN"

# Vérification rapide de l'API avant les scénarios
echo "Vérification de l'API..."
if curl -s -H "Authorization: Bearer $TOKEN" http://127.0.0.1:8080/health | grep -q OK; then
    echo "✅ API opérationnelle"
else
    echo "❌ L'API ne répond pas correctement"
    kill $API_PID
    exit 1
fi

# Lancer les scénarios
./test_scenarios.sh "$TOKEN"

# Nettoyage
kill $API_PID
rm -f "$LOG"
