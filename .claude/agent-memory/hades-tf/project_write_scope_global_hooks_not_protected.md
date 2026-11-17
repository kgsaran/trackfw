---
name: write-scope-global-hooks-not-protected
description: write-scope protege .codex/hooks.json relativo ao project root; paths globais (fora de trackfw project root) → allow
metadata:
  type: project
---

`writescope.go:65-70` lista `.codex/hooks.json` como frozen path relativo ao root de um trackfw project.

`RunWriteScope` chama `findProjectRoot(resolved)` para o target do Write tool. Se o target (`~/.codex/hooks.json`, `~/.claude/settings.json`) não está dentro de um project com `trackfw.yaml` → `found=false` → `return 0` (allow).

Portanto:
- Escrita AI via Write tool para `~/.codex/hooks.json` global → NOT blocked by write-scope
- Escrita AI via Write tool para diretório em PSModulePath (fora de project) → NOT blocked

Consequência para 5-A: PSModulePath injection não oferece capability além da escrita direta a paths globais — ambos passam pelo mesmo controle (ou falta dele).

**Why:** write-scope é por-projeto (scope = dentro de um trackfw project). Governa o projeto, não o perfil global do usuário.
**How to apply:** ao analisar findings que envolvem paths globais (home, perfil de AI), não assumir que write-scope os protege sem verificar findProjectRoot.
