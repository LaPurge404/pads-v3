# PADS v3 — DAG-Driven Deterministic CI Engine

PADS v3 est un moteur de supervision événementiel déterministe pour le
développement logiciel. Il combine un graphe de dépendances (L1), un
journal d'événements immuable (L2) et une projection d'état (L3) pour
fournir une base fiable à des agents d'intelligence artificielle.

**Version** : v0.4-dag-stable
**Licence** : Expérimental / Recherche

---

## 🎯 Objectifs fondamentaux

PADS v3 valide qu'un système peut :
- Reconstruire l'état de manière déterministe à partir des journaux d'événements
- Maintenir la cohérence de l'état du graphe L3 à travers les reconstructions
- Survivre à la corruption WAL et à la perte de fichiers
- Exécuter du code Go réel dans un environnement sandboxé et reproductible
- Converger vers un état final stable sous des boucles de réduction répétées
- **Orchestrer des pipelines CI multi-jobs avec un DAG causal déterministe**

---

## 🏗️ Architecture du système

```

Job Spec → DAG Builder → DAG Executor (deterministic) → Event Stream → WAL → Causal Engine

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
| `internal/dag` | **Moteur DAG déterministe** – exécution topologique, scheduler single-thread, workers stateless |
| `internal/ci` | Pipeline CI (Plan, Scheduler, Cache, Artifacts, Replay) |
| `internal/replay` | Hermetic Replay Engine avec isolation CI-grade |
| `internal/ci/causal` | Couche causale – instrumentation, localisation de divergence, patch engine |
| `internal/adaptive` | Boucle adaptative – diagnostic causal et correction ciblée |
| `internal/chaos` | Suite de tests de chaos (crash, corruption, désordre) |

---

## 🔁 DAG Executor (nouveau cœur)

Le moteur DAG remplace l'ancien scheduler par vagues :
1. **Construction du graphe** : BuildDAG crée un graphe acyclique dirigé (DAG)
   où chaque nœud représente une action (job start, step run, cache, artifact).
2. **Exécution topologique** : l'Executor maintient une file d'attente (ready queue)
   et exécute les nœuds dont toutes les dépendances sont satisfaites.
3. **Workers stateless** : les goroutines exécutent les commandes sans jamais
   modifier l'état du scheduler. L'ordre est garanti par le single-thread scheduler.
4. **Événements déterministes** : chaque nœud produit ses événements, collectés
   dans l'ordre topologique et écrits dans le WAL.

---

## 🧬 Propriétés clés

- **Déterminisme** : même L2 → même L3 (validé par tests de convergence)
- **Idempotence** : `INSERT OR IGNORE` partout, transactions atomiques
- **Immuabilité** : L2 est append-only
- **Herméticité** : le Replay Engine n'a aucune dépendance au système de fichiers hôte
- **Reproductibilité** : exécution stable dans un environnement Go contrôlé (CI-oriented)
- **DAG-driven** : l'ordre d'exécution est dicté par les dépendances causales, pas par le runtime

---

## 📁 Structure du projet

```

internal/
 storage/       → SQLite, schéma, wrappers
 compiler/      → AST → L1 (ingestion déterministe)
 reducer/       → L2 → L3 (fonction pure)
 dag/           → Moteur DAG (nœuds, résolveur topologique, exécuteur)
 ci/            → Pipeline CI (Plan, Scheduler, Cache, Artifacts, Replay, causal)
 replay/        → Hermetic Replay Engine
 adaptive/      → Boucle adaptative avec diagnostic causal
 chaos/         → Tests de chaos et résilience
 symbol/        → Table de symboles (lookup)
 resolver/      → Résolution des appels (non-régression)

```

---

## 🧪 Tests

```bash
go test ./... -v -timeout 120s
```

Tous les packages ont des tests unitaires. La suite de chaos (internal/chaos) valide
la résilience du système face aux crashs, corruptions et désordres.

---


       ✔ Moteur DAG déterministe
       ✔ Tests de chaos validés
       ✔ Replay Engine avec isolation CI-oriented
       ✔ CI Gate en lecture seule
       ✔ Boucle adaptative avec diagnostic causal
       🔧 Couche causale hybride en développement

---


Système expérimental / de recherche — aucune garantie de production.
