# PADS — Policy-Augmented Deterministic System

[![Build](https://github.com/LaPurge404/pads-v3/actions/workflows/pads-ci.yml/badge.svg)](https://github.com/LaPurge404/pads-v3/actions/workflows/pads-ci.yml)

**PADS** est un moteur d'évolution auto‑protégé conçu pour les pipelines CI/CD et les systèmes autonomes.
Il applique des politiques d'évaluation, détecte l'instabilité, journalise chaque décision de manière déterministe,
et peut rejouer l'intégralité de son historique (event‑sourcing).

---

## Architecture (v3)

```

     ┌──────────────┐     ┌───────────────────┐
  HTTP API   │────▶│ Event Queue  │────▶│ Async Worker      │
 (dashboard) │     │ (persistée)  │     │ (reprise crash)   │
     └──────────────┘     └─────────┬─────────┘

                })

 SafeEvolutionLoopV3  │
  • MultiCycleEvaluator
  • StabilityGate (adaptatif)
  • AntiCollapseDetector
  • RollbackManager
  • WAL + EventStore   │


                })

  ReplayEngine        │
  (time‑travel debug) │


```

---

## Variables d'environnement

| Variable | Description |
|----------|-------------|
| `PADS_TOKEN` | Token Bearer pour l'authentification API. |
| `NVIDIA_API_KEY` | Clé API NVIDIA NIM (fournisseur LLM par défaut). |
| `OPENAI_API_KEY` | Clé API OpenAI pour `OpenAIClient`. |
| `ANTHROPIC_API_KEY` | Clé API Anthropic pour `ClaudeClient`. |

---

## Démarrage rapide

```bash
# Cloner le dépôt
git clone https://github.com/LaPurge404/pads-v3.git
cd pads-v3

# Lancer l'API
go run ./cmd/evolution-api

# Ouvrir http://127.0.0.1:8080
```

Au premier lancement, un token Bearer est généré automatiquement et affiché dans le terminal. Utilisez-le pour les requêtes authentifiées.

---

## Endpoints HTTP

L'API écoute sur `127.0.0.1:8080` :

| Méthode | Path | Auth | Description |
|---------|------|------|-------------|
| `GET` | `/` | Non | Dashboard HTML interactif |
| `GET` | `/health` | Non | Sonde de liveness — renvoie `OK` |
| `GET` | `/dashboard/enriched` | Non | Dashboard enrichi avec UCB |
| `POST` | `/evolve` | Oui | Soumettre un candidat (JSON) |
| `GET` | `/state` | Oui | État système (rebuild via ReplayEngine) |
| `POST` | `/rotate` | Oui | Rotation du token Bearer |
| `GET` | `/select` | Oui | Bras UCB actuellement sélectionné |
| `GET` | `/workspace` | Oui | Statut git et tests |
| `POST` | `/agent/evolve` | Oui | Évolution via CodeAgent |
| `GET` | `/agent/status` | Oui | Statistiques UCB des agents |
| `GET` | `/agent/strategies` | Oui | Stratégies disponibles |

Tous les endpoints protégés nécessitent le header :
```
Authorization: Bearer <token>
```

### Exemple curl

```bash
# Soumettre une évolution
curl -s -X POST http://127.0.0.1:8080/evolve \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"candidate":12,"current":7,"weight":1.5,"mode":"stable"}'

# Lire l'état système
curl -s -H "Authorization: Bearer YOUR_TOKEN" \
  http://127.0.0.1:8080/state

# Rotater le token
curl -s -X POST http://127.0.0.1:8080/rotate \
  -H "Authorization: Bearer YOUR_TOKEN"
```

---

## Replay (debug time‑travel)

```bash
go run ./cmd/evolution-replay -- -file evolution.log
./evolution-replay -file evolution.log -full
./evolution-replay -file evolution.log -state-at 5
```

---

## Tests

```bash
# Tous les tests
go test ./... -v

# Couverture
go test ./... -cover -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html
```

Plus de 100 tests couvrent l'intégralité des composants (évaluateur, gate, détecteur, rollback, WAL, worker, replay, API).

**Note** : Le flag `-race` n'est pas supporté sur Android/arm64. Les tests sur cette plateforme s'exécutent sans.

---

## Fonctionnalités

- **Évaluation pondérée** : comparaison de configurations candidates.
- **Stability Gate adaptatif** : seuil dynamique basé sur l'écart‑type historique.
- **Anti‑collapse** : détection de variance, oscillation, drift.
- **Rollback automatique** : restauration de l'état stable en cas d'instabilité.
- **WAL persistant** : journal horodaté et chaîné (SHA‑256), recovers from crash.
- **Event‑sourcing** : chaque décision est un événement immuable, rejouable à l'identique.
- **Bandit multi‑bras** : exploration déterministe (seedable) pour l'apprentissage.
- **CodeAgent** : agent autonome utilisant un LLM pour générer des corrections de code.
- **Sandbox isolé** : validation des modifications avant application (copy-test-apply).
- **API HTTP sécurisée** : token Bearer, rate limiting, écoute locale, TLS, rotation de token.
- **Timeout HTTP** : chaque requête est limitée à 30s par défaut (configurable).
- **Headers de sécurité** : `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`.
- **Supervision en temps réel** : dashboard web avec formulaire de pilotage et métriques.

---

## Sécurité

- Token d'accès obligatoire pour les endpoints sensibles.
- Rate limiting : 10 requêtes/minute par token (configurable).
- Middleware d'authentification exécuté **avant** le rate limiting.
- Écoute restreinte à localhost par défaut.
- TLS supporté (`-cert` et `-key`).
- Rotation du token via `POST /rotate`.
- Aucune clé API ou secret dans le code — tout via variables d'environnement.

---

## Structure du projet

```
cmd/
  evolution-api/      # Serveur HTTP + dashboard
  evolution-replay/   # Outil de replay CLI
  pads/              # CLI principal
  pads-ci/          # Client d'intégration CI
internal/
  agent/            # CodeAgent + Sandbox + LLM
  policy/
    evolution/      # Moteur d'évolution (cœur)
    learner/        # Détection d'anomalies et apprentissage
    shadow/         # Évaluation parallèle et A/B testing
    change/         # Validation des propositions de changement
    wal/            # Journal d'audit
docs/
  DESIGN.md         # Architecture détaillée
  DEPLOY.md         # Guide de déploiement
  ci/               # Documentation CI
```

---

## CI/CD

Le pipeline GitHub Actions (`.github/workflows/pads-ci.yml`) exécute :
1. `go build ./...`
2. `go test ./... -count=1`
3. Lancement de l'API en arrière-plan
4. Exécution de `pads-ci` pour vérifier la stabilité

Le token est passé via `secrets.PADS_TOKEN` — jamais en clair.

---

Projet maintenu par LaPurge404.