---
status: Accepted
date: 2026-10-04
author: "trackfw_architect"
---

# ADR: o guard de hook é um subcomando Go do trackfw, e a linha de hook é a mesma string em todo shell

> Date: 2026-10-04 | Status: Accepted

REQ: `docs/req/REQ-2026-09-05-os-hooks-de-guard-nao-executam-no-windows-na-maioria-dos-clis-de-agente-e-o-validate-reporta-instalado.md`
Emenda: `docs/adr/ADR-2026-09-05-hook-de-windows-roda-no-windows-geracao-nativa-por-cli-de-agente-em-vez-de-exigir-git-bash.md` — **substitui D6 e D7**, reforma D3. D1, D2, D4 e D5 continuam valendo.

## Context

A ADR-2026-09-05 decidiu que o hook de Windows roda no Windows (D1) e, no adendo de 2026-09-06,
que a forma seria um `.ps1` invocado por `powershell -NoProfile -ExecutionPolicy Bypass -File <…>.ps1`
(D7). Ao abrir a implementação, em 2026-10-04, medimos uma restrição que a D7 não considera:

**As configs de hook são versionadas.** `git ls-files` neste repositório lista `.claude/settings.json`,
`.codex/hooks.json` e `.gemini/settings.json`. O mesmo arquivo commitado é lido pelo dev de macOS e
pelo dev de Windows do mesmo time.

Consequências diretas:

1. **Nem o `trackfw init` pode escolher `.sh` ou `.ps1` pelo SO.** Ele roda numa máquina e escreve um
   arquivo que vale para todas. Um `init` feito em macOS emite `.sh` e deixa o colega de Windows sem
   guard; um feito em Windows emite a linha da D7 e quebra o colega de macOS.
2. **Nem o hook pode escolher.** Quem interpreta a string é o CLI de agente, com o shell que **ele**
   escolhe: PowerShell (Gemini, Codex, Cursor), `cmd.exe` (Kiro), Git Bash **ou** PowerShell conforme
   a máquina (Claude Code), campo por plataforma (Copilot). O Git Bash no Windows é só um caso disso.
3. **A D7 é uma linha de PowerShell.** Commitada, ela falha em `sh`.

Além disso, medido no código em 2026-10-04:

- o `trackfw-git-branch-guard.sh` tem **756 linhas** (a REQ citava 561) e existe em **4 cópias** que
  precisam mudar juntas (ADR-2026-10-02, D4);
- ele depende de `jq`, com um extrator `awk` de fallback que a ADR-2026-10-02 teve de criar porque o
  Windows sem `jq` falhava aberto (#507);
- os dois guards **já chamam o `trackfw`** (44 e 8 ocorrências): a dependência do binário no `PATH`
  já existe hoje;
- a emissão não se limita aos 6 CLIs da tabela: `agentfiles.go` também emite para Windsurf
  (`bash scripts/…`) e Amazon Q.

Portar 756 linhas para PowerShell é exatamente a "segunda implementação de controle de segurança"
que a D3 da ADR anterior classifica como dívida, e repete o problema que a Regra Dura do Go
eliminou entre runtimes na v8.

## Decision

### D1 — O guard é um subcomando do binário: `trackfw guard git-branch` e `trackfw guard credential`

A lógica dos dois guards de segurança é portada para Go, **com comportamento igual** ao do `.sh` de
hoje (nenhuma melhoria junto: é o que torna a paridade verificável). O payload continua chegando
pelo stdin; exit 2 e stderr continuam sendo o sinal de bloqueio; a saída `hookSpecificOutput` de
cada CLI é preservada.

### D2 — A linha de hook é a mesma string em todo shell

Toda config emitida passa a usar `trackfw guard <nome>` (e os argumentos que o guard precisar, como
palavras simples, sem `$VAR`, sem aspas e sem caminho). Essa string é sintaticamente idêntica em
`sh`, `bash`, Git Bash, PowerShell 5/7 e `cmd.exe`.

**Resposta à pergunta "como o CLI sabe se roda `.ps1` ou `.sh`?": ele não precisa saber.** Não há
script a escolher. O shell que o CLI de agente escolher resolve `trackfw` no `PATH` e executa o
mesmo binário. Isso vale para o Git Bash no Windows e para o time misto com a config commitada.

Ganhos que vêm junto, todos já medidos como defeito em algum momento:

| problema | por que some |
|---|---|
| `ExecutionPolicy Restricted` bloqueia `.ps1` (D6 anterior) | a política vale para script, não para `.exe` |
| `$CLAUDE_PROJECT_DIR` vs `$env:…` no PowerShell | a string não tem variável; a raiz vem do cwd/subida até `trackfw.yaml` |
| caminho com espaço ou acento (não medido na ADR anterior) | a string não tem caminho |
| `jq` ausente no Windows (#507) | o parser JSON é o `encoding/json` do Go |
| 4 cópias do guard que mudam juntas | uma implementação |

### D3 — O `.sh` vira invólucro fino, não some

`scripts/trackfw-*-guard.sh` passam a ser `exec trackfw guard <nome> "$@"`. Motivo: projetos que já
têm config apontando para o `.sh` continuam protegidos sem precisar regenerar. A config nova não
aponta mais para eles.

### D4 — Copilot usa o campo `command`

Com a mesma string servindo aos dois mundos, o Copilot recebe o campo `command` (o fallback
cross-platform do schema dele), e não mais só `bash`. Isso fecha o AC1 da REQ sem script novo.

### D5 — Os riscos novos vão para a Wave 0, não para a fé

A mudança troca uma confiança por outra: antes o controle confiava num **script no repositório**
(com integridade verificada por `credential_guard_script_integrity`); agora confia no **`trackfw`
resolvido pelo `PATH`**. Três riscos são do desenho e precisam de medição antes de implementar:

1. **Shim do npm no PowerShell.** O canal npm instala um lançador Node (`bin/trackfw.js`); no
   Windows o npm cria os shims `trackfw.cmd` e `trackfw.ps1`. Se o PowerShell resolver o `.ps1` sob
   `Restricted`, o hook falha. **É a medição que pode derrubar esta ADR para o canal npm.**
2. **Binário velho → falha aberta.** Um `trackfw` sem o subcomando `guard` sai com código ≠ 2, e
   para os CLIs de agente isso é "permitir". O desenho precisa falhar fechado ou, no mínimo,
   **avisar alto**, e o `validate` precisa denunciar a versão incompatível.
3. **`trackfw` ausente do `PATH` do processo do CLI de agente.** Mesmo efeito do item 2.

### D6 — O `validate` passa a relatar se o hook **pode executar**, não se o arquivo existe

Mantém a D5 da ADR anterior. O relato muda de objeto: com o guard no binário, "pode executar" passa
a significar "a config aponta para `trackfw guard …` **e** o `trackfw` resolvido tem esse
subcomando". O arquivo `.sh` presente deixa de ser evidência de proteção.

## Consequences

- **Some a dívida da D3 anterior.** Não há duas implementações do controle; a paridade passa a ser
  `.sh` de hoje ↔ Go, verificada uma vez na migração, com o corpus de testes existente, e não para
  sempre.
- **As ADRs que governam detalhes do `.sh` passam a governar o Go.** A ADR-2026-10-02 (parser JSON
  falha fechado, NUL, chave duplicada) vira o contrato do parser em Go; o `awk` de fallback deixa de
  estar no caminho padrão.
- **O guard passa a custar o startup do binário por chamada de ferramenta.** No canal npm, também o
  do Node. Fica medido na Wave 0, não presumido.
- **A integridade do guard muda de objeto.** A regra `credential_guard_script_integrity` deixa de
  proteger o caminho padrão, e a confiança vai para o binário (D5).

## Alternatives Considered

- **Port para `.ps1` (D7 anterior).** Recusada: quebra o time misto com a config commitada e cria uma
  segunda implementação de controle de segurança, com 756 linhas.
- **String poliglota sh/PowerShell chamando `.sh` ou `.ps1`.** Recusada: não cobre `cmd.exe` (Kiro);
  é frágil em quoting e ainda exige as duas implementações.
- **Escolher pelo SO no `init`.** Recusada: o arquivo é versionado (Context, item 1).
- **Exigir Git Bash.** Já recusada na D1 da ADR anterior.

## Linked REQ

`docs/req/REQ-2026-09-05-os-hooks-de-guard-nao-executam-no-windows-na-maioria-dos-clis-de-agente-e-o-validate-reporta-instalado.md`
