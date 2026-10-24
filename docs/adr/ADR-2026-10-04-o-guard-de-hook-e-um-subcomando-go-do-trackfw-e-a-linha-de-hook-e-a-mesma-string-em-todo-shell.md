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

---

## Adendo — 2026-10-04 (Wave 0): a linha de hook tem duas formas, por família de shell

Medido na VM (Windows 11 ARM64, PowerShell **5.1.26100**), por um `.cmd` que lê `%ERRORLEVEL%` linha a
linha:

```
cmd /c exit 2                                                → 2   (controle)
powershell -NoProfile -Command "cmd /c exit 2"               → 1   ← o exit 2 vira 1
powershell -NoProfile -Command "cmd /c exit 2; exit $LASTEXITCODE" → 2
```

E o mesmo sufixo em `sh`, `bash` (macOS) e Git Bash (VM): `<cmd que sai 2>; exit $LASTEXITCODE` → 2,
e `<cmd que sai 0>; …` → 0, porque `$LASTEXITCODE` é vazio e `exit` sem argumento devolve o status do
último comando.

🔴 Sem o sufixo, **todo CLI que roda o hook por `powershell -Command` recebe 1 no lugar de 2**, e só
o exit 2 bloqueia nos CLIs de PowerShell. O guard decidiria bloquear, e o CLI deixaria passar.

### D2 revista — duas formas, nenhuma escolhida pelo SO

| família | CLIs | linha emitida |
|---|---|---|
| PowerShell **ou** POSIX | Claude Code, Codex, Gemini, Cursor, Copilot (`command`), Windsurf | `trackfw guard <nome>; exit $LASTEXITCODE` |
| `cmd.exe` | Kiro, Amazon Q | `trackfw guard <nome>` |

A escolha é **por CLI** (um fato do fornecedor, fixo), não por SO. Isso preserva o argumento central
da D2: a config versionada serve ao time misto. O Claude Code, que alterna entre Git Bash e
PowerShell na mesma máquina, funciona nos dois com a mesma linha.

### D7 — Todo erro sob `trackfw guard` sai com 2

Erro de cobra sob `guard` (subcomando desconhecido, flag inválida, argumento posicional) sai com
**exit 2** e motivo no stderr, nunca 1. Motivo: no `cmd.exe`, `;` não separa comandos, então se a
forma POSIX/PowerShell cair num `cmd` (fallback de borda do Codex), o binário recebe `git-branch;` e
deve **negar** em vez de liberar.

### D8 — O comando por argumento vira flag

O `.sh` aceita o comando em argv (`CMD_RAW="$*"`). No Go isso vira `--command "<cmd>"`; argumento
posicional é erro (D7). O invólucro `.sh` traduz `"$@"` para `--command`.

### D9 — Porta fiel onde a Wave 0 achou assimetria

- git-branch sobe até achar `trackfw.yaml`; credential só olha o cwd e lê o stdin **antes** de olhar
  o projeto. Os dois preservados como estão.
- O dreno de stdin do git-branch é em **janelas de 2 s** (não `io.ReadAll`): `(payload; sleep 3)`
  libera e `(payload; sleep 6)` nega em cerca de 4 s. O Go reproduz isso.
- O credential lê sem timeout (`cat`). Porta fiel; **resíduo declarado**: um pipe que nunca fecha
  deixa a decisão para o timeout do CLI.
- O parser JSON lê por `map[string]json.RawMessage`, não por struct. O `encoding/json` casa chave
  sem diferenciar maiúsculas em struct (`"Command"` sobrescreveria `"command"`, medido pela Wave 0).

### Riscos da D5 depois da Wave 0

- **Binário velho:** continua sendo falha aberta em 6 de 8 CLIs. Mitigação: o `validate` exige
  versão mínima e o subcomando; neste repositório, `make install` antes de migrar as próprias configs
  (gate `trackfw guard --help` = 0 antes do ML-2A).
- **npm sob `Restricted`:** decidido pelo KG em 2026-10-04 (ML-1D medido). O `validate` trata como
  violation o `trackfw` resolvido para o shim `.ps1` com a política efetiva `Restricted`, e orienta
  `Set-ExecutionPolicy -Scope CurrentUser RemoteSigned`. O shim não tem Mark-of-the-Web e, sob
  `RemoteSigned`, roda e devolve 2. É a mesma exigência que o próprio `npm.ps1` já impõe para rodar
  no PowerShell. 🔴 Sob `Restricted`, com o sufixo, a linha sai 0: falha aberta, inclusive no
  Copilot. Por isso é violation, e não aviso.
- **`cmd.exe` procura no cwd antes do PATH:** um `trackfw.exe`/`trackfw.cmd` na raiz do repositório
  é executado pelo Kiro e pelo Amazon Q. O `validate` denuncia (ML-2B).

Parecer completo: `docs/seguranca/2026-10-04-wave0-guard-em-go.md`.

---

## Adendo — 2026-10-06 (CI do PR #527): a palavra de comando perde as aspas

Medido com o binário da branch: `"git" push`, `git "push"`, `'git' push` e `git p""ush` saem **0**
(o `.sh` da `main` também). No Windows isso não é ofuscação: `"C:\Program Files\Git\bin\git.exe" push`
é a forma normal de chamar o git pelo caminho completo, e o guard não a via.

### D10 — Remoção de aspas na tokenização das palavras de comando

Decisão do KG em 2026-10-06. Ao dividir um segmento em palavras, o guard em Go aplica a remoção de
aspas do shell: espaço dentro de `'…'` ou `"…"` não separa palavra, e as aspas saem da palavra
(`g""it` → `git`, `"C:\Program Files\Git\bin\git.exe"` → um token, base `git`). Vale para a palavra
de comando, para o subcomando e para os prefixos `env`/`command`.

**Revê em parte a ADR-2026-08-12:** a classe "evasão por citação" (`g""it push`) deixa de ser resíduo.
Continuam abertas, por decisão daquela ADR: `git${IFS}push`, expansão de chaves (`{git,push}`), barra
invertida (`g\it`), variáveis e subshell (`$(echo git) push`), `env`/`command` com flags.

Divergência deliberada da fixture `.sh` congelada, no sentido seguro (nega onde o `.sh` liberava).

---

## Adendo — 2026-10-07 (issue #535): a linha de hook falha fechada quando o `trackfw` falta

Com o `trackfw` ausente do PATH de quem executa o hook (app GUI no macOS recebe `/usr/bin:/bin:/usr/sbin:/sbin`;
máquina nova; binário removido), a linha da D2 revista saía **127** no sh/bash — erro não bloqueante — e o guard
desligava sem aviso. O `validate` não vê esse caso: ele roda no PATH do terminal. Medição completa em
`docs/portabilidade/2026-10-07-linha-de-hook-fail-closed-sem-trackfw.md` (ML-6A, duas rodadas, macOS e VM Windows).

### D11 — Linha fail-closed por família, ainda uma string só por CLI

Decisão do KG em 2026-10-07 ("tentar silenciar; se não houver variante silenciosa, adotar a de menor ruído").

| família | CLIs | linha emitida |
|---|---|---|
| PowerShell **ou** POSIX | Claude Code, Codex, Gemini, Cursor, Copilot (`command`), Windsurf | `$LASTEXITCODE=2 2>${null-/dev/null}; trackfw guard <nome>; LASTEXITCODE=$((2*!!$?)); exit $LASTEXITCODE` |
| `cmd.exe` | Kiro, Amazon Q | `trackfw guard <nome> \|\| exit 2` |

Medido (binário ausente / guard nega / guard libera): **2 / 2 / 0** em sh e bash (macOS), Git Bash, PowerShell 5.1
(`-Command`) e `cmd /c`.

Como funciona a linha PS/POSIX:
- **sh/bash:** `$LASTEXITCODE` é vazio, então a semente vira o comando `=2`, que falha em silêncio (`${null-/dev/null}`
  expande para `/dev/null`); `LASTEXITCODE=$((2*!!$?))` vale 2 para qualquer saída ≠ 0 do `trackfw` (127 se ausente,
  2 se negou) e 0 se liberou.
- **PowerShell:** a semente atribui 2 a `$LASTEXITCODE`; se o `trackfw` não existe, o valor não muda e sai 2; se existe,
  vale a saída dele. O trecho `LASTEXITCODE=…` não é atribuição no PS (vira comando inexistente) e não altera nada.

**Custos aceitos (medidos):**
- No PowerShell 5.1, uma linha de erro `CommandNotFound` no stderr em toda execução (o trecho `LASTEXITCODE=…`); no
  sh/bash, stderr limpo nos casos B e C.
- Dentro de bloco `& { … }`, `-File` ou `.ps1`, a semente vira variável local e a linha **nega tudo** — falha no sentido
  seguro. Os CLIs medidos rodam o hook como string de comando (`-Command`/`bash -c`).
- A string é críptica. Fica num sítio único no gerador e é afirmada por teste de concordância gerador↔validator.

Revê o registro da Wave 0 de que uma string fail-closed universal seria impossível no PS 5.1: o `|| exit 2` de fato
não parseia lá, mas a semente em `$LASTEXITCODE` resolve sem ele.

### D11 revista (2026-10-07, issue #538) — toda saída ≠ 0 vira 2 também no PowerShell

Na D11, o PowerShell repassava a saída do `trackfw` íntegra: um binário presente que sai 1 ou 3 (ex.: versão antiga
sem `guard`) fazia a linha sair 1 ou 3 — erro não bloqueante. Medido no ML-6C com um `trackfw` falso (0/1/2/3/ausente).
A linha da família PS/POSIX ganha um quarto trecho, que o PowerShell executa como atribuição e o sh descarta:

`$LASTEXITCODE=2 2>${null-/dev/null}; trackfw guard <nome>; LASTEXITCODE=$((2*!!$?)); $LASTEXITCODE=2*!!$LASTEXITCODE 2>${null-/dev/null}; exit $LASTEXITCODE`

Medido: saída **0 só quando o `trackfw` sai 0** e **2 em todos os outros casos** (1, 2, 3, ausente) em sh, bash, Git
Bash e PowerShell 5.1; o ruído no stderr do PowerShell é o mesmo da D11 (o trecho novo é atribuição válida). Família
`cmd` inalterada (`trackfw guard <nome> || exit 2`, já 0/2/2/2/2). Medição em
`docs/portabilidade/2026-10-07-linha-de-hook-fail-closed-sem-trackfw.md`, seção ML-6C.


## Adendo — 2026-10-21: correção de falha sob zsh — prefixo `emulate -R sh` (REQ-2026-10-20)

A linha D11 revisada (ML-6C) falhava silenciosamente quando o hook era executado sob zsh com as
opções padrão ativas. Dois mecanismos distintos causavam abort antes de `trackfw guard` rodar:

1. **EQUALS expansion** (`setopt EQUALS` ativo por padrão em zsh interativo): `$LASTEXITCODE` vazio
   → o token `=2` aciona *equals expansion* → zsh tenta `which 2` → falha com exit 1, encerrando o
   hook imediatamente. A proteção nunca chegava a executar.

2. **NOMATCH** (`setopt NOMATCH` ativo por padrão em zsh): o 4º segmento
   `$LASTEXITCODE=2*!!$LASTEXITCODE` contém `*` não quotado → glob expansion → nenhum arquivo casa
   → zsh aborta com `zsh: no matches found: =2*!!`.

**Candidato descartado — `unsetopt EQUALS NOMATCH`:** O prefixo `unsetopt EQUALS NOMATCH 2>${null-/dev/null};`
resolve EQUALS e NOMATCH mas é insuficiente sob variantes mais restritivas do zsh. Medição ML-1B:

| Prefixo              | zsh padrão | zsh -o nounset | zsh -o errexit | zsh -o cshnullglob |
|----------------------|------------|----------------|----------------|--------------------|
| sem prefixo          | 127        | 127            | 127            | 127                |
| `unsetopt EQUALS NOMATCH` | 2     | 1              | 127            | 127                |
| `emulate -R sh`      | 2          | 2              | 2              | 2                  |

Sob `zsh -o nounset`, `$LASTEXITCODE` não inicializado aborta com `unbound variable` (exit 1).
Sob `zsh -o errexit`, qualquer saída não-zero antes de `trackfw guard` encerra o hook (exit 127).
O `unsetopt` não foi publicado em nenhum config versionado, portanto não vira forma legada.

**Solução final:** prefixo `emulate -R sh 2>${null-/dev/null};` antes da semente. O `emulate -R sh`
reseta TODAS as opções do zsh para os padrões POSIX sh — incluindo nounset, errexit, cshnullglob e
qualquer combinação futura. Em sh/bash é `command not found` com stderr descartado pelo redirecionamento
— inerte. Em PS 5.1 o `emulate` é CommandNotFound não-terminante: a execução segue e o `$LASTEXITCODE`
final não muda (medido na VM, `docs/portabilidade/2026-10-21-guard-zsh-candidato-windows.md`, rodada 2).
🔴 **Mas o stderr NÃO é descartado no PS 5.1:** `2>${null-/dev/null}` não suprime o registro de
CommandNotFound (achado A6, medido com e sem o redirect). A linha continua poliglota em todos os
ambientes.

**Custo aceito (decisão do KG, 2026-10-21):** no PowerShell 5.1, cada execução do hook passa a emitir
**duas** linhas de erro `CommandNotFound` no stderr (`emulate` + o trecho `LASTEXITCODE=…` já
registrado no adendo de 2026-10-07), e no deny elas aparecem antes do motivo do guard. Aplicando a
regra de 2026-10-07 ("tentar silenciar; se não houver variante silenciosa, adotar a de menor ruído"):
toda forma que corrige o zsh medida até aqui custa essa linha (o `unsetopt` também custava), e a
alternativa sem ruído é o fail-open do zsh. Nenhuma variante silenciosa foi pesquisada a fundo; a
candidata esboçada (`${=null-emulate -R sh}`) quebraria sh/bash com "bad substitution" e foi descartada sem
medição. Se alguém trouxer uma variante silenciosa medida na matriz inteira, ela substitui esta.

Forma canônica atualizada (REQ-2026-10-20, família PS/POSIX):

`emulate -R sh 2>${null-/dev/null}; $LASTEXITCODE=2 2>${null-/dev/null}; trackfw guard <nome>; LASTEXITCODE=$((2*!!$?)); $LASTEXITCODE=2*!!$LASTEXITCODE 2>${null-/dev/null}; exit $LASTEXITCODE`

A D11 revisada (pre-REQ-2026-10-20: sem o prefixo `emulate -R sh`) não gera violation no `trackfw validate` —
é aceita como forma legada —, mas emite aviso solicitando `trackfw update harness` para migrar
para a nova forma. Os arquivos de configuração deste repositório foram migrados para REQ-2026-10-20 no mesmo
ML. O único sítio canônico continua `guardExpectedLine` em `validator_guard_binary_probe.go`.

`docs/portabilidade/` (Artemis, em andamento) documentará a medição de cada shell.

## Adendo — 2026-10-20: atenção nativa (ML-1H da instalação Windows)

Estende D1/D3 aos hooks informativos: `trackfw attention signal` e `trackfw attention cleanup` são a fonte única Go; scripts .sh existentes viram invólucros de compatibilidade. Todas as configs emitidas usam essas linhas simples, sem variáveis/caminhos/jq/python. Copilot usa command. Atenção não bloqueia ferramentas: erros internos resultam em no-op e exit 0. Por isso não copia D11/D11 revista dos guards. A falha da semente polyglot sob zsh está registrada em vault/notes/codex-guard-polyglot-aborta-zsh-equals-2026-10-20.md e a correção dos guards segue na sua REQ própria.

O payload é dado JSON, nunca código. Escrita e cleanup ficam confinados fisicamente ao roadmap_dir da raiz com trackfw.yaml no cwd, usando as guardas existentes; não remover o arquivo dedicado de credential guard. Migração reconhece somente comandos legados de atenção emitidos pelo produto e preserva hooks personalizados. Prova local shell não será declarada como jornada real de CLI de agente no Windows.

## Adendo — 2026-10-23 (issue #599): uma lista de gerações, e a migração termina em uma entrada por par

**Contexto medido.** No `.cursor/hooks.json` de um consumidor com a 10.1.0, depois de `trackfw update`, cada par
`preToolUse`/`postToolUse` × `Read`/`Write` tem 4 formas legadas do credential guard mais a atual. O guard roda 5 vezes por
chamada, e o `validate` continua mandando rodar o `update`, que não resolve. A causa está em `InjectCursorHooks`
(`internal/generators/agentfiles.go`): as migrações de forma legada cobrem só `beforeShellExecution`/`afterShellExecution`.
Os pares com matcher passam apenas por `mergeCursorGuardMatcherEntry`, que acrescenta a forma atual e não toca nas outras.
Além disso, a lista de gerações (`legacyBareCredGuardCmd`, `legacyD2…`, `legacyD11…`, `legacyD11Rev…`, `legacyWABash…`)
está **copiada à mão em cada sítio de migração**, e as cópias já divergem entre os CLIs. É a mesma família do achado de
2026-10-21 (`vault/notes/inject-codex-gemini-d11rev-duplicata-antes-de-migracao-2026-10-21.md`).

### D12 — As gerações conhecidas de cada guard vivem num registro único

Existe **uma** lista, por guard e por família de shell, com todas as formas que o produto já emitiu. O validador (que
classifica uma linha como legada) e todo `Inject*Hooks` e alvo do `update harness` (que migram) consomem essa lista. Nenhum
sítio escreve a própria cópia. Um teste de concordância afirma que toda forma que o validador reconhece como legada é
migrada por todo sítio que emite aquele guard.

### D13 — A migração termina com exatamente uma entrada por (evento, matcher, guard)

Depois da migração, cada par (evento, matcher) tem **exatamente uma** entrada do guard, na forma atual. Isso vale para os
dois estados reais: só formas legadas, e formas legadas convivendo com a atual. Migrar no lugar sem deduplicar não basta,
porque N legadas viram N cópias idênticas da atual. A ordem é sempre migrar, deduplicar e só depois mesclar. Entradas que
não estão no registro (hooks do usuário) nunca são removidas nem reescritas.

### D14 — Com o guard global instalado, a forma legada de projeto é removida

> **Substituída pela D15 (2026-10-24, #613).**

Quando o guard global do CLI está instalado, a entrada de projeto não é emitida (ADR-2026-08-06, decisão 4). Hoje, porém,
uma forma legada de projeto é **migrada** para a forma atual mesmo com o global presente, e isso duplica o guard. Passa a
ser assim: com o global instalado, uma entrada de projeto reconhecida pelo registro é **removida**; sem o global, ela é
migrada. A detecção do global continua com falha aberta (se não der para ler, conta como não instalado), então o pior caso é
o comportamento de hoje.

REQ: `docs/req/REQ-2026-10-23-update-nao-remove-geracoes-antigas-do-credential-guard-em-pares-evento-matcher-e-o-validate-manda-rodar-update-que-nao-resolve.md`.

### D14 revista (2026-10-23, auditoria da Wave 0): a remoção é por par, e só onde o global cobre

> **Substituída pela D15 (2026-10-24, #613).**

A Wave 0 (`docs/seguranca/2026-10-23-wave0-geracoes-do-guard.md`, T4 e R2) mediu que o guard global do Cursor só cobre
`beforeShellExecution`/`afterShellExecution`. Os pares `preToolUse`/`postToolUse` × `Read`/`Write` não têm guard global.
Com a D14 original, uma remoção "com o global instalado" tiraria o único guard que esses pares têm.

Passa a valer: a decisão de pular ou remover a entrada de projeto é tomada **por par (evento, matcher)**, e não por CLI.
Num par que o guard global daquele CLI cobre, a entrada de projeto reconhecida pelo registro é removida. Num par que o
global não cobre, ela é migrada para a forma atual e continua sendo emitida, haja ou não global instalado. Isso não roda o
guard duas vezes: o global não está naquele par. O efeito colateral é intencional e corrige uma lacuna de hoje, em que o
`InjectCursorHooks` com o global instalado deixa de emitir Read/Write e esses pares ficam sem guard.

### D15 (2026-10-24, issue #613): a fiação de projeto não depende do guard global — substitui a D14 e a decisão 4 do ADR-2026-08-06

**Status: substitui a D14 (original e revista) deste ADR e a decisão 4 do ADR-2026-08-06.** O dedup do git-branch-guard
global do ROADMAP-2026-08-17 também cai.

**Defeito medido.** Com o guard global instalado, o `trackfw update` 11.0.0 remove do projeto as entradas que o global
cobre (D14), e o `trackfw init` deixa de emiti-las (decisão 4 do ADR-2026-08-06). Só que `.claude/settings.json`,
`.codex/hooks.json`, `.gemini/settings.json` e equivalentes são **versionados**, e o guard global existe apenas na máquina
de quem rodou o comando. O resultado:
- o repositório fica sem guard no CI e em todo clone sem global;
- o `validate` da mesma versão reprova (`guard_wiring_removed`);
- medido no consumidor ea-cmdb, 4 violations (#613), e neste repositório, 9 violations (rc=1, CI vermelho no #612).

As duas decisões leram estado da máquina e gravaram o resultado num artefato compartilhado.

**Decisão.** Nenhum gerador de **projeto** consulta o guard global, nem para pular, nem para remover. A fiação de projeto é
sempre emitida e migrada para a forma atual, por par (D13), com ou sem global. O validador também deixa de usar o estado
global para aceitar ausência de fiação de projeto, porque o veredito do `validate` precisa ser o mesmo na máquina do
mantenedor e no CI. O `update harness` (escopo global) não muda.

**Custo aceito.** Numa máquina com o global, o guard roda duas vezes por evento coberto. Mediana medida, 20 execuções:
`credential` 12,5 ms e `git-branch` 10,6 ms por chamada. A idempotência do segundo disparo (por exemplo, o arquivo de
atenção no modo `warn`) fica medida na Wave 0 da REQ.

**Alternativas rejeitadas (#613):**
- **Dedup em tempo de execução**, em que o guard de projeto vira no-op quando o global já rodou. Rejeitada porque cada
  chamada passaria a ler o arquivo global, o que custa o mesmo que rodar o guard, e porque abre uma rota de fail-open se a
  detecção errar. O ganho, que é evitar ~11 ms, não paga o risco num controle de negação.
- **Opt-in auditável no `trackfw.yaml` para aceitar a remoção.** Rejeitada porque mantém o artefato versionado dependente
  da máquina de quem o gerou. O CI continuaria sem guard.

REQ: `docs/req/REQ-2026-10-23-update-nao-remove-geracoes-antigas-do-credential-guard-em-pares-evento-matcher-e-o-validate-manda-rodar-update-que-nao-resolve.md` (AC6 revisto, AC9).
