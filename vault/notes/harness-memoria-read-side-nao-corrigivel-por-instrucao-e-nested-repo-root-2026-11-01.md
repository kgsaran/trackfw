---
name: harness-memoria-read-side-nao-corrigivel-por-instrucao-e-nested-repo-root
description: Harness injeta MEMORY.md loaded do cwd de dispatch (incontornável por instrução no body); git rev-parse em nested repo retorna inner root; AC4 deve excluir nested repos de "stray"
metadata:
  type: project
---

## Fato 1 — Lado de leitura é incontornável por instrução no body do agente

O Claude Code injeta "Persistent Agent Memory" **após** o corpo do agente. A seção diz:

> "MEMORY.md is always loaded into your conversation context"

Esse carregamento ocorre no momento do dispatch, usando o path resolvido pelo cwd de dispatch. Uma instrução no body do agente pode influenciar o lado de **escrita** (onde gravar), mas não o lado de **leitura automática** — o harness já carregou o MEMORY.md errado antes de qualquer instrução do agente ser executada.

Mitigador disponível (não solução): instruir o agente a executar `Read(<raiz>/.claude/agent-memory/<nome>/MEMORY.md)` explicitamente no início da sessão, quando o path do harness diferir da raiz git. Mas isso requer que o agente detecte a discrepância e execute a Read extra.

**Por que importa:** AC1 do REQ #646 aborda só o lado de escrita. A emenda P-2 amplia para leitura explícita. Sem ela, um agente que escreve corretamente na raiz mas lê do path errado começa cada sessão sem memória.

## Fato 2 — `git rev-parse --show-toplevel` retorna inner root em nested repos

Medido no scratchpad:

```bash
# outer/ (git repo) + outer/nested/ (git repo separado)
git -C outer/nested/x rev-parse --show-toplevel
# → outer/nested    (não outer/)
```

Consequência para AC1: um agente em `outer/nested/` que usa `git rev-parse --show-toplevel` vai ancorar memória em `outer/nested/.claude/agent-memory/`, que é a raiz correta para aquele contexto. O doctor do repositório pai `outer/` classificaria isso como stray (falso positivo).

## How to apply

- **Wave 1 de #646, P-1/P-2:** o texto AC1 não deve pedir `git rev-parse` a partir do cwd de despacho do agente — em nested repos, resolve a raiz errada. Alternativa: comparar o path do harness com o `projectRoot` do harness (disponível na sessão Claude Code) e usar o sufixo do harness com o prefixo correto.
- **AC4 (doctor, P-5):** ao encontrar `<subdir>/.claude/agent-memory/`, verificar se `<subdir>` é raiz de git repo com `os.Lstat(filepath.Join(subdir, ".git"))` — retorna qualquer tipo, incluindo arquivo regular (submódulo). Se existir, não é stray.
- **Definição de "raiz":** deve ser `git rev-parse --show-toplevel` do processo chamador, não `os.Getwd()`. `integrationsManager().ProjectRoot = os.Getwd()` — se doctor chamado de subpasta, raiz fica errada.

**Why:** essa distinção custaria mais de 10 minutos para redescobrir — sem a nota, o implementador de AC4 usa `manager.ProjectRoot` (natural) e introduz falso negativo para doctor chamado de subpasta, além de falso positivo para nested repos.
