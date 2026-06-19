# Changelog

## [Unreleased] - 2026-06-15

### Added
- Moteur DAG déterministe (`internal/dag`) : nœuds, résolveur topologique, exécuteur avec ready queue
- Scheduler single-thread, workers stateless
- Événements produits par l'exécuteur et collectés dans l'ordre topologique
- CacheSnapshot pour figer l'état du cache avant la construction du plan
- Compatibilité avec l'ancien pipeline CI via shims (Plan, PlannedStep, executePlan)

### Changed
- Remplacement de l'ancien scheduler par vagues par un DAG incrémental
- `internal/ci/scheduler.go` utilise désormais `dag.Executor`
- `internal/ci/replay_verifier.go` adapté au nouveau DAG
- Mise à jour du README pour refléter la nouvelle architecture

### Fixed
- Élimination du non-déterminisme lié à l'ordre d'écriture concurrent dans le WAL
- Correction des divergences de cache entre runs
- Stabilisation de la boucle adaptative v2
