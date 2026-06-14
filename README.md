# PADS v3 — Hermetic CI Engine & Deterministic Replay System

PADS v3 est un moteur de supervision événementiel déterministe pour le
développement logiciel. Il combine un graphe de dépendances (L1), un
journal d'événements immuable (L2) et une projection d'état (L3) pour
fournir une base fiable à des agents d'intelligence artificielle.

**Version** : v0.2.1 (stable-ci)
**Licence** : Expérimental / Recherche

---

## 🎯 Objectifs fondamentaux

PADS v3 valide qu'un système peut :
- Reconstruire l'état de manière déterministe à partir des journaux d'événements
- Maintenir la cohérence de l'état du graphe L3 à travers les reconstructions
- Survivre à la corruption WAL et à la perte de fichiers
- Exécuter du code Go réel dans un environnement sandboxé et reproductible
- Converger vers un état final stable sous des boucles de réduction répétées
- **Rejouer un run de manière déterministe et reproductible (CI-grade)**

---

## 🏗️ Architecture du système

```

L0 (Code Source) → L1 (Graphe) → L2 (Event Log) → Reducer → L3 (State Projection)

Hermetic Execution Engine (CI Sandbox)

utilisé par Replay + CI Gate

```

### Couches

| Couche | Nom | Rôle |
|--------|-----|------|
| L0 | Code Source | Fichiers `.go` sur disque |
| L1 | Execution Graph | Nœuds et arêtes extraits par le Compiler |
| L2 | Event Log | Journal immuable d'événements (append-only) |
| L3 | State Projection | État dérivé de chaque nœud (STABLE/BROKEN/UNTESTED) |

### Composants

| Package | Rôle |
|---------|------|
| `internal/storage` | Accès SQLite avec PRAGMA WAL, transactions, wrappers |
| `internal/compiler` | Ingestion AST → L1 (nœuds, arêtes, hash de signature) |
| `internal/reducer` | Boucle de réduction L2 → L3 (fonction pure, point fixe) |
| `internal/engine` | Hermetic Execution Engine 3 phases (Snapshot, Execution, Commit) |
| `internal/replay` | **Hermetic Replay Engine** – exécution isolée dans un workspace temporaire |
| `internal/ci` | **CI Gate** – validateur d'invariants en lecture seule sur la projection L3 |
| `internal/chaos` | Suite de tests de chaos (crash, corruption, désordre) |
| `internal/agent` | Agent Runtime avec contrat Task/Plan/Action/Executor |
| `internal/scheduler` | Boucle daemon continue (Engine → Agent → Executor) |

---

## 🔁 Hermetic Replay Engine

Le Replay Engine garantit qu'un run peut être rejoué de manière déterministe :

1. **Capture de snapshot** : contenu des fichiers, hash, nœuds associés
2. **Workspace temporaire isolé** : création d'un module Go jetable
3. **Environnement contrôlé** : `GOCACHE`, `GOMODCACHE`, `GOPATH` isolés
4. **Exécution reproductible** : `go test -count=1 ./...`
5. **Événement L2** : un événement `REPLAY_RESULT` est émis pour audit

---

## 🧪 CI Gate

Le CI Gate est un validateur passif qui vérifie les invariants système
sur la projection L3 :

- Aucun nœud `BROKEN` ne doit persister
- Chaque nœud de L1 doit avoir un état dans L3

Il n'exécute jamais le moteur – il se contente de lire l'état.

---

## 🧬 Propriétés clés

- **Déterminisme** : même L2 → même L3 (validé par tests de convergence)
- **Idempotence** : `INSERT OR IGNORE` partout, transactions atomiques
- **Immuabilité** : L2 est append-only
- **Herméticité** : le Replay Engine n'a aucune dépendance au système de fichiers hôte
- **Robustesse** : validée par la suite de tests de chaos

---

## 📁 Structure du projet

```

internal/
 storage/       → SQLite, schéma, wrappers
 compiler/      → AST → L1 (ingestion déterministe)
 reducer/       → L2 → L3 (fonction pure)
 engine/        → Exécution 3 phases (Snapshot, Execution, Commit)
 replay/        → Hermetic Replay Engine
 ci/            → CI Gate (validation invariants)
 agent/         → Agent Runtime (Task, Plan, Action, Executor)
 executor/      → Exécuteur des plans d'action
 scheduler/     → Boucle daemon (Engine → Agent → Executor)
 chaos/         → Tests de chaos et résilience
 symbol/        → Table de symboles (lookup)
 resolver/      → Résolution des appels (non-régression)

```

---

## 🚀 Exécution

```bash
# Mode one-shot
./pads -db test_project/pads.db -ingest test_project

# Mode daemon
./pads -db test_project/pads.db -ingest test_project -daemon -interval 30s
```

---


```bash
go test ./... -v -timeout 120s
```

Tous les packages ont des tests unitaires. La suite de chaos (internal/chaos) valide
la résilience du système face aux crashs, corruptions et désordres.

---


       ✔ Moteur de réduction de noyau stable
       ✔ Tests de chaos validés
       ✔ Hermetic Execution Engine 3 phases
       ✔ Hermetic Replay Engine avec isolation CI-grade
       ✔ CI Gate en lecture seule
       ✔ Agent Runtime avec Executor
       ✔ Boucle de réalité fermée (ingestion → exécution → feedback → convergence)

---


Système expérimental / de recherche — aucune garantie de production.
