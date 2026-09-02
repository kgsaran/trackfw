# Um stub `gh` com `#!/usr/bin/env bash` falha silenciosamente numa `PATH` restrita sem `/usr/bin:/bin` — e o erro tem a FORMA certa pela razão errada

**Data:** 2026-09-02 · **Achado por:** Apolo (ML-3A, `docs/roadmaps/wip/ROADMAP-2026-09-01-o-repositorio-do-trackfw-sob-os-cuidados-do-trackfw.md`)

## O problema

`scripts/check-doctor-remote-parity.sh` (e o padrão que `check-release-tag-parity.sh` já usava)
constrói um executável `gh` STUB e o coloca no início de uma `PATH` restrita, para provar
`trackfw doctor --remote` sem rede real. A `PATH` restrita carregava só os interpretes
(`node`, `python3`, `git`) — sem `/usr/bin:/bin`.

O stub começa com `#!/usr/bin/env bash`. O kernel resolve `/usr/bin/env` por caminho absoluto
(fora da `PATH` do processo), mas `env` por sua vez usa a `PATH` **herdada do processo chamador**
para resolver `bash`. Sem `/usr/bin:/bin` na `PATH`, `env` não encontra `bash`, e o exec do stub
falha — **antes** de qualquer lógica do stub rodar.

## Por que isso não foi óbvio

O erro não apareceu como "comando não encontrado". `execForgeAPI` captura stderr e trata
QUALQUER falha do processo `gh` como "sem credencial" — exatamente o resultado que a checagem
deveria produzir por rede/token realmente ausentes. **O finding tinha a forma certa
(`not-evaluated`, remédio "authenticate") pela razão errada** (o stub nunca rodou, não porque a
`auth status` simulada tenha falhado de propósito). Sem os asserts de não-vacuidade do próprio
gate (checar a presença do finding certo E ausência de `not-evaluated` nos cenários que deveriam
evidenciar findings reais), esse bug teria passado despercebido — todos os cenários "produziriam"
`not-evaluated`, inclusive os que deveriam mostrar findings reais.

`check-release-tag-parity.sh` já resolvia isso com `BASE_PATH="$RUNTIME_BIN:/usr/bin:/bin"` — a
lição já estava registrada em código, só não generalizada em texto.

## A correção

`BASE_PATH` (ou equivalente) para QUALQUER gate que gere um binário-stub com shebang
`#!/usr/bin/env <interprete>` precisa incluir `/usr/bin:/bin` (onde `env`/`bash`/`sh` moram),
mesmo quando a intenção da `PATH` restrita é isolar só um binário específico (aqui, esconder um
`gh` real do host). Isolar não é "só o essencial" — é "essencial MENOS o que o mecanismo do
próprio stub silenciosamente depende".

## Como generalizar a lição

Qualquer gate futuro que:
1. constrói um binário-stub via heredoc com shebang `#!/usr/bin/env <algo>`, E
2. executa esse stub sob uma `PATH` restrita/customizada (para simular "ferramenta ausente" ou
   isolar de binários reais do host)

precisa incluir `/usr/bin:/bin` nessa `PATH`, e um guard de não-vacuidade que distinga "o
resultado X apareceu porque o cenário simulou X" de "o resultado X apareceu porque o mecanismo de
simulação falhou de um jeito que também produz X".

## Ver também

`docs/cli-parity.md` § `trackfw doctor --remote — modalidade remota opcional (ADR-2026-09-02,
ML-3A)`, `scripts/check-doctor-remote-parity.sh` (comentário sobre `BASE_PATH`).
