---
title: trackfw.yaml aninhado re-enraíza o write-scope e rebaixa o guard de credencial
date: 2026-11-24
req: REQ-2026-11-23 (write-scope com raiz aninhada)
adr: ADR-2026-11-24 (D1–D4)
---

# trackfw.yaml aninhado re-enraíza o write-scope e rebaixa o guard de credencial

**Sintoma:** um subagente escreve `.claude/trackfw.yaml` (rc=0) e depois `.claude/settings.json` (rc=0; sem a planta, rc=2).

**Causa:** `findProjectRoot` (`internal/guard/writescope.go`) devolve o ancestral mais PRÓXIMO do alvo com
`trackfw.yaml`, por `os.Lstat`. A lista de congelados é resolvida contra essa raiz, e
`.claude/settings.json` vira `.claude/.claude/settings.json`.

Variantes do mesmo mecanismo:
- um diretório chamado `trackfw.yaml`;
- `TRACKFW.YAML` em FS sem caixa;
- FIFO, que trava `os.ReadFile`.

**Mesma causa no guard de credencial:** o modo vem de `<cwd>/trackfw.yaml`. Um `mode: warn` plantado rebaixa o global
de `block` para `warn`. O guard de projeto não sobe, então com o cwd num subdiretório ele fica desligado.

**Armadilhas para quem corrige:**
- a proteção de `.claude/settings.json` vem da lista FIXA de subagente, não do `write_scope` do yaml. Unir só os
  `frozen_paths` dos ancestrais não fecha nada;
- `if err != nil { return nil, nil, nil }` transforma diretório/FIFO em "ausente". Só `os.IsNotExist` é ausente;
- `IsRegular()` sozinho não pega a caixa e quebra yaml por symlink. Use `fileutil.ReadRegularFile`;
- `credReadModeWithDefault` tem dois chamadores (projeto `warn`, global `block`). Um piso `"block"` fixo dentro da
  função eleva o escopo de projeto;
- o hook de credencial bloqueia comandos que contenham o token falso dos testes (`eyJ...`). Meça por teste Go, não
  pelo shell da sessão.

**Fonte:** `docs/seguranca/2026-11-24-wave0-write-scope-raiz-aninhada-threat-model.md`.
