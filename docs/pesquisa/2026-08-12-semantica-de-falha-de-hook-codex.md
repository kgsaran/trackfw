# Pesquisa: semântica de falha de hook no Codex CLI — fail-open vs fail-closed

> ML-1A do roadmap
> `ROADMAP-2026-08-12-semantica-de-falha-de-hook-fail-open-vs-fail-closed-nucleo-empirico-no-codex.md`.
> Prova empírica, não documental: nenhuma doc primária do Codex (`developers.openai.com/codex/hooks`)
> afirma o que acontece quando um hook `PreToolUse` falha em vez de reprovar (contrato `exit 2`
> documentado). Este documento fecha essa lacuna rodando o Codex real contra um fixture controlado.

## Pergunta

O `scripts/trackfw-credential-guard.sh` é um controle de negação. Hoje existem caminhos documentados
(`docs/cli-parity.md`, "Pré-condições do fix do Codex") em que o `command` do hook do Codex **não
resolve** (script ausente/caminho inválido) — caso diferente do contrato conhecido de bloqueio
(`exit 2` + stderr). Se a falha de execução do hook for tratada como fail-open, esses caminhos são uma
forma de contornar o guard. Se for fail-closed, são apenas degradação de disponibilidade.

## Método

### Fixture

- Repositório git novo em `/private/tmp/.../scratchpad/codex-foh/fixture` (fora do repo trackfw),
  inicializado com `git init` + 1 commit.
- `$HOME` isolado: `CODEX_HOME` apontado para
  `/private/tmp/.../scratchpad/codex-foh/home/.codex` (diretório novo, nunca usado antes).
  Único conteúdo copiado para lá: `auth.json` (cópia de leitura do `auth.json` real, necessária para
  autenticação não-interativa; **nenhuma escrita** foi feita no `auth.json` original — confirmado por
  checksum idêntico entre origem e cópia após todo o experimento). Nenhum `config.toml` foi criado ou
  copiado — o Codex roda com config totalmente vazio/default neste `CODEX_HOME`.
- Flag `--dangerously-bypass-hook-trust` em toda invocação, para não confundir "hook ignorado por
  projeto não-trusted" (achado do ML anterior, `vault/notes/codex-hooks-de-projeto-so-rodam-em-projeto-trusted-2026-08-11.md`)
  com a pergunta deste experimento.
- `.codex/hooks.json` no fixture, hook `PreToolUse` no matcher `Bash`, no mesmo formato que o
  gerador do trackfw emite (`internal/generators/agentfiles.go`, `migrateHookCommand(hooks["PreToolUse"], "Bash", ...)`):

  ```json
  {
    "hooks": {
      "PreToolUse": [
        {
          "matcher": "Bash",
          "hooks": [
            { "type": "command", "command": "$(git rev-parse --show-toplevel)/.codex/hooks/hook.sh" }
          ]
        }
      ]
    }
  }
  ```

  (Caso A varia esse `command` para um caminho inexistente; casos B1/B2/controle variam o conteúdo de
  `hook.sh`.)

### Discriminante

O prompt do `codex exec` pede **um único** comando `Bash`: `echo ran > <marker>`. Se o `<marker>`
existir depois da rodada, a ferramenta prosseguiu apesar do hook → **FAIL-OPEN**. Se não existir →
**FAIL-CLOSED**. O marker é apagado entre braços.

### Comando base (varia só `.codex/hooks.json`/`hook.sh` e o env `CODEX_HOME` fica fixo)

```bash
CODEX_HOME="$HOME_ISO/.codex" codex exec \
  --dangerously-bypass-hook-trust \
  --dangerously-bypass-approvals-and-sandbox \
  -C "$FIX" \
  "Run exactly this shell command and nothing else: echo ran > $MARKER"
```

`--dangerously-bypass-approvals-and-sandbox` foi usado nos quatro braços principais para remover a
restrição de escrita do sandbox padrão para um caminho de marca fora do workspace. **Achado do
experimento, não suposição prévia:** `codex exec` roda com `approval: never` por padrão
independentemente do modo de sandbox — não houve nenhum prompt de aprovação interativa em nenhuma
rodada, com ou sem o bypass (ver a seção "Verificação de escopo" abaixo, que reproduz controle
positivo e caso A sob `-s workspace-write` **sem** esse flag, com marca escrita dentro do fixture, e
obtém o mesmo resultado). A configuração de fato não testada é uma **política de aprovação diferente
de `never`** (ex.: sessão interativa com humano no loop) — não o flag de bypass em si.

### Achado incidental de execução (não sobre o veredito)

A primeira tentativa do caso A, capturada com `--json` redirecionado a arquivo e monitorada em
background, levou bem mais que os 2 minutos default de timeout do shell interativo do agente — por
isso as rodadas subsequentes foram lançadas em background (`nohup ... &`) e monitoradas por polling.
O tempo de execução em si não é evidência de nada sobre fail-open/fail-closed; é só uma nota
operacional para quem for reproduzir isso.

## Braços executados

### Controle positivo — hook `exit 0`

**Setup:** `.codex/hooks/hook.sh` = `#!/bin/bash\nexit 0`. `command` no `hooks.json` aponta para esse
script existente via `$(git rev-parse --show-toplevel)/.codex/hooks/hook.sh`.

**Saída observada (trecho relevante):**

```
warning: `--dangerously-bypass-hook-trust` is enabled. Enabled hooks may run without review for this invocation.
hook: PreToolUse
hook: PreToolUse Completed
exec
/bin/zsh -lc 'echo ran > .../marker' in .../fixture
 succeeded in 0ms:
```

**Marca:** presente (`cat marker` → `ran`).

**Veredito: passa.** O hook disparou (`hook: PreToolUse` / `hook: PreToolUse Completed`) e a
ferramenta executou. Confirma que o experimento está de fato disparando o evento `PreToolUse` e que a
marca é um discriminante válido — sem isso nenhum resultado dos outros três braços seria confiável.

### Caso A — `command` aponta para caminho inexistente

**Setup:** `command` = `$(git rev-parse --show-toplevel)/.codex/hooks/does-not-exist.sh` (arquivo
nunca criado).

**Saída observada (trecho relevante):**

```
warning: `--dangerously-bypass-hook-trust` is enabled. Enabled hooks may run without review for this invocation.
hook: PreToolUse
hook: PreToolUse Failed
exec
/bin/zsh -c 'echo ran > .../marker' in .../fixture
 succeeded in 0ms:
```

**Marca:** presente (`cat marker` → `ran`).

**Veredito: FAIL-OPEN.** O Codex registra explicitamente `hook: PreToolUse Failed` — ele sabe que o
hook não executou (comando não resolve) — e mesmo assim prossegue com a ferramenta. Não há erro do
tipo `codex_core::tools::router` bloqueando nada; a linha de erro de bloqueio (vista no caso B2 abaixo)
está ausente aqui.

### Caso B1 — script existe, sai com `exit 1`

**Setup, primeira tentativa:** `.codex/hooks/hook.sh` = `#!/bin/bash\nexit 1` (sem stderr).

**Saída observada (trecho relevante):**

```
warning: `--dangerously-bypass-hook-trust` is enabled. Enabled hooks may run without review for this invocation.
codex
I'll run the exact command provided.
hook: PreToolUse
hook: PreToolUse Failed
exec
/bin/zsh -lc 'echo ran > .../marker' in .../fixture
 succeeded in 0ms:
```

**Marca:** presente (`cat marker` → `ran`). **Veredito preliminar: FAIL-OPEN.**

**Confundidor identificado e fechado:** essa primeira tentativa de B1 difere de B2 (abaixo) em duas
variáveis simultâneas — código de saída (1 vs 2) **e** presença de stderr (nenhum vs
`"blocked by policy"`). Isso deixaria em aberto se o discriminador real é o código ou o stderr.
Segunda tentativa isolando só a variável de interesse:

**Setup, segunda tentativa (confundidor fechado):** `.codex/hooks/hook.sh` =
`#!/bin/bash\necho "blocked by policy" >&2\nexit 1` — mesmo stderr do caso B2, só o código de saída
muda (1 em vez de 2).

**Saída observada (trecho relevante):**

```
warning: `--dangerously-bypass-hook-trust` is enabled. Enabled hooks may run without review for this invocation.
hook: PreToolUse
hook: PreToolUse Failed
exec
/bin/zsh -lc 'echo ran > .../fixture/marker' in .../fixture
 succeeded in 0ms:
```

**Marca:** presente (`cat marker` → `ran`).

**Veredito: FAIL-OPEN**, confirmado com o confundidor fechado. Com stderr idêntico ao de B2 e só o
código de saída variando, o resultado não muda: `exit 1` continua fail-open, `exit 2` continua
fail-closed (braço abaixo). O discriminador é **especificamente** o código de saída `2` — não a
presença de mensagem em stderr. Mesmo rótulo do Codex (`hook: PreToolUse Failed`) do caso A: o Codex
trata "hook rodou e saiu com código fora do contrato de bloqueio" da mesma forma que "hook não
rodou" — a ferramenta prossegue nos dois.

### Caso B2 — script existe, sai com `exit 2` (contrato de bloqueio documentado)

**Setup:** `.codex/hooks/hook.sh` = `#!/bin/bash\necho "blocked by policy" >&2\nexit 2`.

**Saída observada (trecho relevante):**

```
warning: `--dangerously-bypass-hook-trust` is enabled. Enabled hooks may run without review for this invocation.
hook: PreToolUse
2026-08-12T14:38:18.557120Z ERROR codex_core::tools::router: error=Command blocked by PreToolUse hook: blocked by policy. Command: echo ran > .../marker
hook: PreToolUse Blocked
codex
Command blocked by policy.
```

**Marca:** ausente (`cat marker` → `No such file or directory`).

**Veredito: FAIL-CLOSED.** Rótulo distinto dos dois braços anteriores (`hook: PreToolUse Blocked`, não
`Failed`), erro explícito do router (`Command blocked by PreToolUse hook: ...`), e a ferramenta não
executa. Confirma o contrato documentado (`exit 2` + stderr = bloqueio).

### Verificação de escopo — o veredito depende de `--dangerously-bypass-approvals-and-sandbox`?

Os quatro braços acima usaram `--dangerously-bypass-approvals-and-sandbox` para viabilizar execução
não-interativa. Hipótese alternativa a excluir: talvez esse flag seja o que produz o fail-open — em
configuração default (`approval: never` mas com sandbox real, sem bypass total), um hook que falha
poderia escalar para bloqueio em vez de prosseguir silenciosamente, tornando o risco real menor do
que "contornável".

**Setup:** mesmo fixture, `-s workspace-write` (sandbox restrito real do Codex, sem
`--dangerously-bypass-approvals-and-sandbox`) em vez de `danger-full-access`; marca escrita **dentro**
do fixture (`$FIX/marker`) para não ser barrada pelo próprio sandbox de escrita.

**Controle positivo sob `workspace-write`** (hook `exit 0`): `hook: PreToolUse Completed`, marca
presente. Passa — confirma que o rig também é válido nesta configuração.

**Caso A sob `workspace-write`** (`command` para caminho inexistente): `hook: PreToolUse Failed`,
marca presente.

**Veredito: FAIL-OPEN também sob `workspace-write`.** O resultado não é um artefato do bypass total —
o mesmo comportamento (`Failed` → ferramenta prossegue) se repete com o sandbox restritivo real
ligado. Ressalva que permanece: `codex exec` (modo não-interativo) já roda com `approval: never` por
padrão em ambas as configurações testadas — não foi possível, neste experimento, forçar um modo em
que uma falha de hook dispare um **prompt de aprovação interativo** (isso exigiria uma sessão TTY
interativa, fora do escopo de automação deste ML). O veredito fail-open está estabelecido para
`codex exec` não-interativo, com e sem bypass de sandbox; não foi verificado para o modo interativo
com aprovação humana no loop.

## Tabela-resumo

| Braço | Setup | Rótulo do Codex | Marca (ferramenta prosseguiu?) | Veredito |
|---|---|---|---|---|
| Controle positivo | `hook.sh` existe, `exit 0` | `hook: PreToolUse Completed` | presente | passa (experimento válido) |
| Caso A | `command` aponta para caminho inexistente | `hook: PreToolUse Failed` | presente | **FAIL-OPEN** |
| Caso B1 | `hook.sh` existe, `exit 1` | `hook: PreToolUse Failed` | presente | **FAIL-OPEN** |
| Caso B2 | `hook.sh` existe, `exit 2` + stderr | `hook: PreToolUse Blocked` | ausente | **FAIL-CLOSED** |
| B1 (confundidor fechado) | `hook.sh` existe, stderr igual a B2, `exit 1` | `hook: PreToolUse Failed` | presente | **FAIL-OPEN** (confirma que o discriminador é o código, não o stderr) |
| Controle + Caso A sob `-s workspace-write` (sem bypass total) | mesmos setups de A, sandbox restrito real | `Completed` / `Failed` | presentes | **FAIL-OPEN também sob sandbox restrito** — não é artefato do bypass |

## Conclusão — o caso que importa para o risco real

**Caso A (o produzido pelos três caminhos documentados em `docs/cli-parity.md` — fora de repo git,
submódulo/worktree com `git rev-parse --show-toplevel` apontando para outra raiz, ou
`GIT_DIR`/`GIT_WORK_TREE` redirecionando a resolução) é FAIL-OPEN no Codex CLI 0.147.0.**

Isso significa que o `scripts/trackfw-credential-guard.sh`, como controle de negação, é **contornável**
por quem consiga colocar o Codex em qualquer um desses três caminhos — não porque o hook reprova e é
ignorado, mas porque o hook **nem chega a rodar** e o Codex, ao detectar essa falha de execução
(`hook: PreToolUse Failed`), deixa a ferramenta prosseguir em vez de bloquear.

O achado adicional do caso B1 (script existe mas sai `exit 1`, fora do contrato documentado `exit 2`)
mostra que o mesmo comportamento fail-open se aplica a **qualquer** falha do hook que não seja
especificamente o contrato de bloqueio — não é uma peculiaridade só da resolução de caminho.

Medir só o caso B2 (o contrato documentado) e concluir "o Codex é fail-closed" teria respondido a
pergunta errada: o contrato documentado é robusto, mas ele só cobre o caminho em que o hook roda e
decide reprovar. O caminho onde o hook simplesmente não roda — que é exatamente o que os três cenários
de `docs/cli-parity.md` produzem — segue a semântica oposta.

**Robustez adicional quanto ao mapeamento "os três caminhos → caso A":** os três caminhos
documentados em `docs/cli-parity.md` podem, a depender da forma exata como `git rev-parse
--show-toplevel` falha no ambiente do usuário, produzir tanto o caso A (substituição de shell vazia
→ `command` não resolve, processo nem chega a rodar) quanto algo equivalente ao caso B1 (o `command`
chega a invocar um processo, mas ele sai com código diferente de 0 e diferente de 2, porque o `git`
embutido no `command` falhou e propagou um código de erro não-contratual). Este experimento mediu
**ambos** como FAIL-OPEN — a conclusão não depende de qual dos dois mecanismos ocorre de fato em cada
um dos três caminhos.

**Escopo do veredito, declarado explicitamente:** confirmado para `codex exec` não-interativo, tanto
com `--dangerously-bypass-approvals-and-sandbox` quanto com sandbox restrito real (`-s
workspace-write`) — não é um artefato do bypass. **Não verificado** para uma sessão interativa com
aprovação humana no loop (fora do alcance de automação deste ML); se a interação humana intercepta o
comando antes da falha do hook ser relevante, o risco prático em uso interativo pode diferir — questão
em aberto para quem for avaliar mitigação.

## Ordem dos braços — por que não foi preciso re-rodar o controle no fim

A ordem de execução foi controle positivo → caso A → caso B1 → caso B2 (mais a verificação de escopo
com sandbox restrito, também controle → caso A). O caso B2, **por último**, bloqueou corretamente
(`hook: PreToolUse Blocked`, marca ausente) — isso por si só prova que o pipeline de hooks seguia
ativo depois dos braços A e B1 terem prosseguido. Não é o experimento "parando de medir nada" depois
de A/B1; se estivesse, B2 também teria deixado a marca passar. Por isso um controle positivo de
fechamento não foi necessário.

## Confirmação de isolamento

- **Nota de precisão:** a variável redirecionada em todas as invocações foi `CODEX_HOME`, não `HOME`.
  `HOME` permaneceu o real do usuário durante todo o experimento; o que se garante é que **nada sob
  `~/.codex/` foi escrito**, que é o que os critérios de aceite exigem.
- `~/.codex/config.toml` real: mtime confirmado **inalterado** (`Aug 11 21:12:54 2026`, anterior ao
  início desta sessão) — nenhuma escrita ocorreu nele.
- `~/.codex/auth.json` real: apenas **lido e copiado** para `CODEX_HOME` isolado (necessário para
  autenticação não-interativa do `codex exec`, já que `HOME`/`CODEX_HOME` isolado não teria sessão
  autenticada própria); checksum MD5 idêntico entre o arquivo original e a cópia após todo o
  experimento (`a9d4e855b3674a0307c09be63de6ec7a` em ambos) — confirma que a cópia não foi
  modificada nem o original tocado por escrita.
- Todas as invocações usaram `CODEX_HOME` apontando para um diretório novo em
  `/private/tmp/.../scratchpad/codex-foh/home/.codex` (nunca usado antes desta sessão), o que faz o
  Codex ler config/hooks/estado de sessão desse diretório isolado em vez de `~/.codex/` real — apenas
  a leitura de credencial (`auth.json`) veio de fora dele, por cópia.
- `git status --porcelain` no repositório `trackfw` durante toda a execução deste ML não mostra
  nenhum arquivo do fixture, do `CODEX_HOME` isolado ou de `~/.codex` — só os arquivos que este ML e
  o ML-1B (Prometeu, em paralelo) produziram dentro de `docs/`.

## Reprodutibilidade

Todo o fixture roda fora do repositório trackfw, em diretório descartável. Para reproduzir:

1. Criar um diretório de scratch, `git init` dentro, 1 commit.
2. Criar um `$HOME` isolado com apenas `.codex/auth.json` copiado (leitura) de um `~/.codex/auth.json`
   válido.
3. Escrever `.codex/hooks.json` no fixture com o hook `PreToolUse`/`Bash` apontando (via
   `$(git rev-parse --show-toplevel)/...`) para um script controlado.
4. Rodar `CODEX_HOME=<home-isolado>/.codex codex exec --dangerously-bypass-hook-trust --dangerously-bypass-approvals-and-sandbox -C <fixture> "Run exactly this shell command and nothing else: echo ran > <marker>"` variando o script/`command` por braço.
5. Checar a saída por `hook: PreToolUse <Completed|Failed|Blocked>` e a presença do `<marker>`.

## Relacionado

- `docs/roadmaps/wip/ROADMAP-2026-08-12-semantica-de-falha-de-hook-fail-open-vs-fail-closed-nucleo-empirico-no-codex.md`
- `docs/seguranca/2026-08-11-revisao-hooks-cwd.md` (Q3 — origem da pergunta)
- `vault/notes/codex-hooks-de-projeto-so-rodam-em-projeto-trusted-2026-08-11.md` (pré-condição de
  trust, usada aqui via `--dangerously-bypass-hook-trust`)
- `docs/pesquisa/2026-08-11-hook-cwd-e-placeholders-por-cli.md` (os três caminhos documentados que
  produzem o caso A no Codex)
- `docs/cli-parity.md`, "Pré-condições do fix do Codex" (os três caminhos citados acima)
