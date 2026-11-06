---
status: Open
date: 2026-11-05
author: "Zeus (trackfw_architect)"
adr: "docs/adr/ADR-2026-11-05-guards-cumprem-o-que-o-readme-promete-mode-lido-com-crlf-padroes-de-credencial-ampliados-e-involucro-de-shell-analisado.md"
roadmap: "docs/roadmaps/wip/ROADMAP-2026-11-05-guards-do-primeiro-usuario-externo-mode-block-ignorado-com-crlf-padroes-de-credencial-e-involucro-de-shell-no-guard-de-git.md"
---

# REQ: Guards do primeiro usuário externo: mode block ignorado com CRLF, padrões de credencial e invólucro de shell no guard de git

> Date: 2026-11-05 | Status: Open
| Linear Issue: 
| Jira Issue: 

## Motivation

Os issues #662 e #663 foram abertos por bruno-castiglioni, com a 11.3.0 no Windows 11, e reproduzidos de
forma independente pelo Lourival no fork. O arquiteto mediu de novo em 2026-11-05, com a 11.4.0 no macOS,
alimentando payloads ao guard por stdin.

- **#662, mode ignorado com CRLF.** Com `credential_guard.mode: block`, o `trackfw.yaml` em LF dá `rc=2` e
  em CRLF dá `rc=0`, com o texto "warning - possible AWS access key". A causa está em
  `credReadModeWithDefault` (`internal/guard/credential.go`), que não remove o `\r`. O validator
  (`extractCredentialGuardMode`) usa `TrimSpace`, então o `validate` e o guard divergem sobre o mesmo
  arquivo.
- **#662, cobertura.** Só JWT e chave AWS são detectados. `ghp_…`, `AIza…` e `sk-proj-…` dão `rc=0` sem
  aviso. O README promete mais ("block agents from materializing real credentials").
- **#663, invólucro de shell.** `bash -c "git push"`, `sh -c`, `bash -lc "git commit"`, `eval "git push"`
  e `powershell -c "git push"` dão `rc=0`. `git push` e `cd x && git push` dão `rc=2`. O `MatchSubcommand`
  (`internal/guard/gitbranch.go`) só reconhece `git` como primeiro token depois de `env`, `command` e
  `sudo`.

Decisões do KG em 2026-11-05: ampliar os padrões de conteúdo e corrigir o README (sem detecção por caminho),
e fazer os guards antes da REQ de CI. O desenho está no ADR.

## Acceptance Criteria
- [x] **AC1 (#662).** Com `credential_guard.mode: block`, o guard dá `rc=2` com o `trackfw.yaml` em LF e
  também em CRLF, nos escopos de projeto e `--global`. Um teste de tabela única afirma que o guard e o
  `extractCredentialGuardMode` do validator chegam ao mesmo `mode` para cada entrada: LF, CRLF, comentário,
  aspas, indentação, chave ausente, `# mode:` comentado, `ci_mode:`, seção vizinha com `mode:` (vazamento
  entre blocos), linha em branco CRLF, `credential_guard:` aninhado, BOM e tab. No global, `mode: warn` em
  CRLF dá `rc=0`. As constantes mortas do `scaffold.go` são apagadas, porque o script gerado é um shim, como a
  Wave 0 mediu.
- [x] **AC2 (#662).** O guard detecta os padrões do ADR D2 (o conjunto exato sai da Wave 0), com mensagem
  que nomeia o tipo. Há vetor positivo por padrão e vetor negativo de falso positivo por padrão (prefixo
  curto demais, texto de documentação). Conteúdo comum (`print(1)`) continua com `rc=0`. Os vetores de
  exemplo vêm de valores públicos de documentação, nunca de segredo real.
- [x] **AC3 (#662).** O README e o `docs/cli-parity.md` listam exatamente o que o guard de credencial
  detecta e o que não detecta (caminho de arquivo, atribuição genérica). A frase "block agents from
  materializing real credentials" passa a ter ressalva.
- [x] **AC4 (#663).** O guard de git dá `rc=2` para `bash -c`, `sh -c`, `bash -lc`, `zsh -c`, `eval`,
  `pwsh -c`, `powershell -Command` e `cmd /c` envolvendo cada subcomando hoje bloqueado (`push` e `commit`
  no mínimo, e a tabela inteira de `applyRule` por amostragem). Dá `rc=0` para `bash -c "git status"`.
  Invólucro aninhado até a profundidade 2 também bloqueia, e acima disso o guard nega. Também dão `rc=2`,
  porque são da mesma causa e foram achados na Wave 0:
  - `\git push` e `g\it push`;
  - `FOO=1 git push`, `env -i git push` e `command -p git push`;
  - `nohup`, `time`, `exec`, `timeout 10s`, `nice -n 10`, `doas -u x` e `stdbuf` antes de `git push`;
  - e, pela emenda da Wave 2: `powershell -ExecutionPolicy Bypass -Command`, `pwsh -co`, `pwsh --command`,
    `pwsh -cwa`, `CMD.EXE /c`, `BASH.EXE -c`, `cmd /c"git push"`, `cmd /c g^it push`, ``pwsh -c 'g`it push'``,
    `env --unset=HOME`, `bash -c $'git push'`, `echo $(git push)`, ``echo `git push` `` e
    `OUT=$(git commit -m x)`.
- [x] **AC5 (#663).** O limite (interpretador de linguagem, como `python -c` e `node -e`) fica declarado no
  README e no `docs/cli-parity.md`, com um teste que fixa o `rc=0` atual como limite conhecido, não como
  garantia.
- [ ] **AC6.** Os issues #662 e #663 recebem comentário com o que mudou e a versão. O PR fecha os dois com
  `Closes`.

## Negative scope

- Não detecta credencial por caminho de arquivo (`.env`, `.aws/credentials`, `.ssh/id_*`).
- Não detecta atribuição genérica (`password = "…"`) nem segredo por entropia.
- Não analisa interpretadores de linguagem (`python`, `node`, `ruby`, `perl`) nem scripts em arquivo.
- Não trata shell lido de stdin (heredoc, herestring, `| sh`, `xargs`), `powershell -EncodedCommand`,
  expansão de variável nem prefixos fora da lista (`ionice`, `taskset`, `nsenter`): são os residuais R1, R3,
  R4 e R6 da Wave 0, declarados na documentação.
- Não muda o padrão de `mode` (`warn` no projeto, `block` no global).
- Não muda a linha de hook gerada nem os eventos e matchers instalados.
- Não mexe na REQ de CI (#664, #665 e a porta fixa do `serve-address`), que vem depois, em outra branch.

## Linked ADR
<!-- Reference the ADR that governs this requirement -->
ADR: docs/adr/ADR-2026-11-05-guards-cumprem-o-que-o-readme-promete-mode-lido-com-crlf-padroes-de-credencial-ampliados-e-involucro-de-shell-analisado.md

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
<!-- Reference the roadmap that implements this requirement -->
Roadmap: docs/roadmaps/wip/ROADMAP-2026-11-05-guards-do-primeiro-usuario-externo-mode-block-ignorado-com-crlf-padroes-de-credencial-e-involucro-de-shell-no-guard-de-git.md
