# PADS — Policy-Augmented Deterministic System

**PADS** est un moteur d’évolution auto‑protégé conçu pour les pipelines CI/CD et les systèmes autonomes.
Il applique des politiques d’évaluation, détecte l’instabilité, journalise chaque décision de manière déterministe,
et peut rejouer l’intégralité de son historique (event‑sourcing).

---

## 🧠 Architecture (v3)

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

## ⚡ Commandes

### API de pilotage (dashboard web + endpoints JSON)

```bash
go run ./cmd/evolution-api
# ou
./evolution-api
# Puis ouvrez http://127.0.0.1:8080
```

L’API écoute sur 127.0.0.1:8080 et propose :

       GET  /         → dashboard HTML interactif
       GET  /health   → état de santé
       GET  /state    → état système (JSON, nécessite token)
       POST /evolve   → soumettre une nouvelle évolution (JSON, nécessite token)

Le token d’authentification est généré au lancement et affiché dans le terminal.

Replay (time‑travel debugging)

```bash
go run ./cmd/evolution-replay -- -file evolution.log
./evolution-replay -file evolution.log -full
./evolution-replay -file evolution.log -state-at 5
```

Options :

       -file   : chemin du journal d’événements
       -full   : affiche chaque étape de l’historique
       -step   : mode pas‑à‑pas (appuyez sur Entrée)
       -state-at N : état du système à la séquence N

---


```bash
go test ./... -v
```

Plus de 100 tests couvrent l’intégralité des composants (évaluateur, gate, détecteur, rollback, WAL, worker, replay, API).

---


       Évaluation pondérée : comparaison de configurations candidates.
       Stability Gate adaptatif : seuil dynamique basé sur l’écart‑type historique.
       Anti‑collapse : détection de variance, oscillation, drift.
       Rollback automatique : restauration de l’état stable en cas d’instabilité.
       WAL (Write‑Ahead Log) : journal horodaté et chaîné (SHA‑256).
       Event‑sourcing : chaque décision est un événement immuable, rejouable à l’identique.
       Bandit multi‑bras : exploration déterministe (seedable) pour l’apprentissage.
       API HTTP sécurisée : token Bearer, rate limiting, écoute locale, TLS optionnel.
       Supervision en temps réel : dashboard web avec formulaire de pilotage, graphique d’historique, métriques de stabilité.

---


       Token d’accès obligatoire pour les endpoints sensibles.
       Rate limiting configurable.
       Écoute restreinte à localhost par défaut.
       TLS supporté (-cert et -key).

---


```
cmd/
  evolution-api/      # Serveur HTTP + dashboard
  evolution-replay/   # Outil de replay CLI
internal/
  policy/
    evolution/        # Moteur d'évolution (cœur du système)
    learner/          # Détection d'anomalies et apprentissage
    shadow/           # Évaluation parallèle et A/B testing
    change/           # Validation des propositions de changement
    wal/              # Journal d'audit
```

---


Le système est opérationnel, testé, et prêt à être intégré dans un pipeline CI/CD réel.
La prochaine étape est l’intégration continue avec décision automatique de déploiement.

---

Projet maintenu par LaPurge404.
