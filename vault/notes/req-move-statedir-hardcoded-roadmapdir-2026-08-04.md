---
title: "req-move-statedir-hardcoded-roadmapdir"
tags: [req, roadmap, config, bug, parity, cross-cli]
date: 2026-08-04
related: []
---

# req-move-statedir-hardcoded-roadmapdir

## Problem

`ROADMAP-2026-08-04-req-move-list-subpastas-e-move-fisico.md` (ML-1A/1B/1C) instrui os 3 CLIs a
reaproveitar as funções de resolução de diretório-alvo já usadas por `MoveRoadmap`/`moveRoadmap`/
`move_roadmap` para implementar o move físico condicional de `MoveREQ`:

- Go: `stateDir`/`agentStateDir` (`internal/generators/roadmap.go:30,40`)
- Node: equivalentes em `npm/src/generators/roadmap.js` (ML-1B)
- Python: `_state_dir`/`_agent_state_dir` (`pypi/trackfw/generators/roadmap.py:109,116`, ML-1C)

Essas três funções resolvem o caminho contra `cfg.RoadmapDir`/`cfg.roadmapDir`/`cfg["roadmap_dir"]`
**hardcoded** — são funções de roadmap, escritas antes de REQ precisar de layout por-estado. Usá-las
literalmente para mover uma REQ grava o arquivo dentro da árvore de roadmaps (`docs/roadmaps/<estado>/
REQ-....md`), não em `docs/req/<estado>/`.

## Root cause

`stateDir(state)` (Go):

```go
func stateDir(state string) (string, bool) {
	cfg := config.Load()
	if !roadmapValidStateNames[state] {
		return "", false
	}
	return cfg.RoadmapDir + "/" + state, true  // ← RoadmapDir, não o dir do chamador
}
```

`agentStateDir` tem o mesmo problema com `cfg.RoadmapDir`. Nenhuma das duas aceita um parâmetro de
diretório-base — foram desenhadas assumindo que o único chamador seria código de roadmap.

## Impact

Testes com asserts frouxos (ex.: só checar `err == nil` do `MoveREQ`, sem checar o caminho final do
arquivo) não capturam o bug — ele só aparece com um assert explícito de que o arquivo apareceu em
`docs/req/<novo-estado>/`, como os testes `TestMoveREQ_PhysicallyMovesInStateLayout` e
`TestMoveREQ_PhysicallyMovesInByAgentLayout` (`internal/generators/req_test.go`) fazem.

## Correção aplicada (Go, ML-1A)

Em vez de chamar `stateDir`/`agentStateDir`, `MoveREQ` (`internal/generators/req.go`) constrói o
`targetDir` diretamente com `filepath.Join(cfg.REQDir, status)` (por-estado) ou
`filepath.Join(cfg.REQDir, agent, status)` (by_agent) — **ainda reaproveitando**
`roadmapValidStateNames` para validar o nome do estado, só não reaproveitando a resolução de
diretório em si.

## Débito remanescente / atenção para Wave 1 (ML-1B, ML-1C)

Node e Python ainda não foram auditados neste ponto — a instrução do roadmap para eles é a mesma
("reaproveitar `stateDir`/`agentStateDir`" / `_state_dir`/`_agent_state_dir`"), então é provável que
caiam na mesma armadilha se seguirem a instrução ao pé da letra. Verificar, ao revisar ML-1B/ML-1C,
se o `targetDir` do `moveREQ`/`move_req` usa `reqDir`/`req_dir`, não `roadmapDir`/`roadmap_dir`.
