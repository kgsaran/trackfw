---
status: Done
date: 2026-08-08
author: "kg.saran@gmail.com"
adr: "docs/adr/ADR-2026-08-06-hooks-de-credential-guard-em-escopo-global-via-trackfw-update-harness.md"
roadmap: "docs/roadmaps/done/ROADMAP-2026-08-08-credential-guard-modo-block-por-padrao-cobertura-de-read-write-e-resolucao-de-arquivo-referenciado.md"
---

# REQ: credential-guard — modo block por padrão, cobertura de Read/Write e resolução de arquivo referenciado

> Date: 2026-08-08 | Status: Done

## Motivation

Análise feita em 2026-08-08 (sessão real de uso, projeto CMDB) após um subagente extrair um access
token real do Entra ID (`e2e/.auth/auth-state.json`) durante uma investigação de bug, escrever o
valor completo em `/tmp/token.txt` e num arquivo de scratchpad, e imprimir trechos (`head -c 50`,
`tail -c 60`) no stdout/transcript — episódio sinalizado pelo harness como security warning. O hook
`trackfw-credential-guard.sh` (global, `~/.trackfw/scripts/`, instalado via `trackfw update
harness` — ver `ADR-2026-08-06-...` e `REQ-2026-08-06-hooks-de-credential-guard-...`) **estava
corretamente instalado e ativo**, e de fato disparou nesta mesma sessão para um subagente
diferente que colou um JWT literal dentro de um comando `curl` — mas não capturou o incidente
acima. Investigação confirmou 3 lacunas estruturais no desenho atual do script/wiring, não um bug
de instalação (isso já foi corrigido pela REQ de 2026-08-08 anterior):

1. **Modo default é `warn`, nunca `block`.** O script só assume `mode: block` se o
   `trackfw.yaml` do projeto tiver um bloco `credential_guard: mode: block` explícito
   (`grep -A 5 '^credential_guard:'`). Sem essa config (caso comum — nenhum dos projetos
   observados a define), o hook **nunca impede** a ação, só loga um aviso em stderr e grava
   `<roadmap_dir>/.trackfw-credential-guard.json`. Para um mecanismo cujo propósito é evitar
   materialização de credencial real, um default permissivo esvazia boa parte do valor.

2. **O wiring só registra o matcher `Bash`** (confirmado em `~/.claude/settings.json`:
   `PreToolUse`/`PostToolUse` com `"matcher": "Bash"`, gerado pelo `update harness`). Extração de
   credencial via os tools `Read` (ler o arquivo que contém o segredo diretamente) ou `Write`/`Edit`
   (materializar o segredo num arquivo novo) nunca passa pelo hook — ele literalmente não é
   invocado para essas chamadas. No incidente analisado, é plausível que boa parte da
   materialização tenha ocorrido por esse caminho (o `head -c 50`/`tail -c 60` finais, via Bash,
   também não bateram — ver item 3).

3. **É um regex estático sobre o texto literal do payload do comando, não um resolvedor de
   dataflow.** `JWT_PATTERN`/`AWS_KEY_PATTERN` só casam se a credencial aparecer *literalmente*
   dentro do `tool_input.command` (ex.: colar o JWT inteiro num header `curl -H "Authorization:
   Bearer eyJ..."`). Comandos que referenciam a credencial por caminho de arquivo ou substituição
   de comando — `head -c 50 /tmp/token.txt`, `jq -r .access_token auth-state.json >
   /tmp/token.txt`, `cat auth-state.json | ...` — nunca contêm a string do segredo no texto do
   comando em si; o regex não tem o que casar e o hook passa em silêncio, mesmo que o comando
   claramente esteja manipulando/expondo a credencial.

Nenhuma dessas 3 lacunas é específica do CLI Claude Code — a lógica central do script
(`is_ephemeral_target`, checagem de `MODE`, escopo do matcher no wiring gerado por `update
harness`) é compartilhada pelos 6 alvos (Go/Node/Python × Claude/Codex/Gemini/Copilot/Cursor/Kiro),
então a correção deve valer para os 3 stacks e os CLIs que suportam os tools/matchers equivalentes
a `Read`/`Write`/`Edit`.

## Acceptance Criteria
- [x] Decisão de design registrada (ADR novo ou emenda ao ADR-2026-08-06) sobre o novo default de
      `mode` — recomendação: `block` por padrão para o script global (o usuário optou
      explicitamente por instalar esse hook via `trackfw update harness`; um guard de segurança
      opt-in que não bloqueia por padrão é uma armadilha de falsa sensação de proteção). Se
      `block` quebrar fluxos legítimos de algum CLI/wave, documentar o trade-off e a mitigação
      (ex.: allowlist de padrões conhecidos-seguros, não just "warn").
- [x] Wiring gerado por `update harness` (Go/Node/Python, todos os alvos `<cli>-credential-guard`)
      passa a registrar o hook também para os tools de leitura/escrita de arquivo equivalentes a
      `Read`/`Write`/`Edit` em cada CLI suportado — ou, onde o CLI não expuser hook por-tool para
      esses casos, documentar a limitação explicitamente (não silenciosamente).
- [x] Script central ganha uma segunda camada de detecção: quando o comando contém um redirect ou
      substituição referenciando um caminho de arquivo (não `mktemp`, não `/dev/null`), resolver o
      caminho e escanear o **conteúdo do arquvo referenciado** (quando acessível de forma síncrona
      e barata) pelo mesmo `JWT_PATTERN`/`AWS_KEY_PATTERN`, além do payload do comando em si — cobre
      o padrão `head`/`tail`/`cat`/`jq ... > arquivo` sem precisar de um resolvedor de dataflow
      completo.
- [x] Testes novos (Go/Node/Python) cobrindo os 3 cenários que hoje escapam: (a) `mode` ausente no
      `trackfw.yaml` deve bloquear, não só avisar; (b) chamada de tool `Read`/`Write`/`Edit` com
      payload contendo JWT/AWS key é capturada; (c) comando Bash que referencia um arquivo com
      segredo por caminho (sem o literal no comando) é capturado.
- [x] `make quality`/paridade Go-Node-Python sem regressão.
- [x] Nenhuma mudança de comportamento para quem já definiu `credential_guard: mode: warn`
      explicitamente no próprio `trackfw.yaml` — a mudança de default só afeta ausência de config.

## Linked ADR
ADR: `docs/adr/ADR-2026-08-06-hooks-de-credential-guard-em-escopo-global-via-trackfw-update-harness.md`
— pode precisar de emenda para o novo default de `mode` e para a cobertura de tools além de `Bash`.

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: `docs/roadmaps/done/ROADMAP-2026-08-08-credential-guard-modo-block-por-padrao-cobertura-de-read-write-e-resolucao-de-arquivo-referenciado.md`
