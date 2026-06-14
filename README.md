# PADS v3 — Exécution déterministe et système de débogage résistant au chaos

PADS v3 est un système expérimental basé sur Go conçu pour valider la reconstruction
d'état déterministe dans des environnements sujets aux défaillances à l'aide de la
relecture d'événements, des boucles de réduction et des tests d'injection de chaos.

L'idée de base : le système doit toujours converger vers le même état final, même
sous la corruption, les crashs ou les défaillances randomisées.

## 🎯 Objectifs fondamentaux

PADS v3 valide qu'un système peut :
- Reconstruire l'état de manière déterministe à partir des journaux d'événements
- Maintenir la cohérence de l'état du graphe L3 à travers les reconstructions
- Survivre à la corruption WAL et à la perte de fichiers
- Gérer les exécutions randomisées et les échecs IO
- Converger vers un état final stable sous des boucles de réduction répétées
- Exécuter du code Go réel dans un environnement sandboxé et reproductible

## 🏗️ Architecture du système

Le système est composé de cinq couches principales :

1. **Couche d'événement** : stocke les événements d'exécution brute et les relations.
   `events` → résultats d'exécution, `event_nodes` → mappage entre événements et nœuds.

2. **Couche de stockage (SQLite)** : moteur de persistance responsable de la gestion
   du schéma, de la cohérence transactionnelle et du stockage d'état graphique.
   Backend : `modernc.org/sqlite`.

3. **Moteur de réduction (Core Logic)** : système de reconstruction de l'état
   déterministe. Rejoue les journaux d'événements, construit `graph_state`,
   assure la convergence vers `STABLE` ou `BROKEN`. Cette couche est purement
   déterministe.

4. **Execution Engine** : moteur d'exécution sandboxé qui exécute `go test` sur
   les fichiers source associés aux nœuds `UNTESTED`. Il initialise un module Go
   temporaire (`go mod init tempmod`) pour garantir une exécution reproductible
   et isolée. Les résultats sont injectés dans L2 sous forme d'événements
   `OS_EXEC_RESULT`.

5. **Couche d'injection de faute (Chaos Engine)** : simule les défaillances du
   monde réel (latence, erreurs IO, échecs d'écriture, conditions SQLITE_BUSY).
   Utilisé exclusivement pour la validation de la robustesse.

## 🔁 Flux de système

```

Événements (L2) → Stockage SQLite → Moteur de réduction → Graph State (L3)
↑
Execution Engine
↑
Code source Go

```

## 🧪 Stratégie de test

PADS v3 utilise la validation pilotée par le chaos.

### Tests Déterministes
- Reconstruction complète à partir de zéro
- Duplication d'événements
- Stabilité de l'ordre des événements
- Validation de la convergence par rejeu

### Tests de chaos
- Simulation de suppression de fichiers WAL
- Validation de récupération après crash
- Écritures partielles et état corrompu
- Injection de défaillance IO aléatoire
- Validation de convergence multi-exécutions

## ⚙️ Tech Stack

- Go (moteur de base)
- SQLite (`modernc.org/sqlite`)
- Primitives de concurrence de la bibliothèque standard
- Driver d'injection de fautes personnalisé

## 📁 Structure du projet

```

internal/
├── chaos/     → tests de chaos & récupération
├── storage/   → couche d'abstraction SQLite
├── reducer/   → moteur de réduction déterministe
├── fault/     → driver d'injection de fautes
├── resolver/  → couche de résolution d'événements
├── symbol/    → utilitaires symboliques
├── compiler/  → ingestion AST → graphe L1
└── engine/    → moteur d'exécution sandboxé

```

## 🧬 Propriété clé

Le système garantit que l'exécution répétée sur le même journal d'événements
converge toujours vers le même état de graphe final, même sous échecs IO,
pics de latence, erreurs d'exécution forcées ou suppression de fichier
pendant l'exécution.

## 🧪 Garantie de convergence

Malgré les conditions de chaos, le système converge toujours vers `STABLE`
ou `BROKEN`. C'est l'invariant principal de PADS v3.

## 📌 Statut

- ✔ Moteur de réduction de noyau stable
- ✔ Tests de chaos validés
- ✔ Injection de fautes opérationnelle
- ✔ Mécanismes de récupération vérifiés
- ✔ Execution Engine avec sandbox Go reproductible
- ✔ Boucle de réalité fermée (ingestion → exécution → feedback → convergence)

## 📜 Licence

Système expérimental / de recherche — aucune garantie de production.
