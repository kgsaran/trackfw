---
status: wip
date: 2026-08-17
req: "docs/req/REQ-2026-08-17-guard-global-e-instalado-sem-fiacao-e-sua-integridade-nunca-e-verificada.md"
adr: "docs/adr/ADR-2026-08-17-guard-global-cabeado-com-no-op-fora-de-projeto-trackfw.md"
squad: "apolo-tf, hades-tf"
---

# Roadmap: guard global cabeado com no-op fora de projeto, e integridade independente de fiação

> Created: 2026-08-17 | Status: wip

## Context

REQ: `docs/req/REQ-2026-08-17-guard-global-e-instalado-sem-fiacao-e-sua-integridade-nunca-e-verificada.md`
ADR: `docs/adr/ADR-2026-08-17-guard-global-cabeado-com-no-op-fora-de-projeto-trackfw.md` (decisão de KG, 2026-08-17)

Medido: o `git-branch-guard` global é **escrito** (`update.go:493`) e **não é cabeado** em nenhum dos
6 configs globais — o `credential-guard` tem 2 refs em cada um dos 4 existentes, ele tem zero. A
regra de integridade global só avalia configs que **referenciam** o script, então nunca roda para
ele: ficou **3 versões atrasado** (123 linhas contra 369) com `validate` verde.

Medido também, e é o que define a ordem das waves: o script **não tem no-op fora de projeto
trackfw** — num repo sem `trackfw.yaml`, `git push` → `exit 2`. **Cabear antes do no-op quebraria
todos os repositórios da máquina.**

Esta REQ cobre **escopo global**. O escopo de projeto (`validate` não detectar hook na forma relativa
antiga) é a `REQ-2026-08-17-validate-nao-detecta-hook-de-guard-na-forma-relativa-antiga-que-falha-fora-da-raiz.md`,
com roadmap próprio **depois** deste — as duas tocam os mesmos arquivos de validador.

## Acceptance Criteria

- [x] AC1 — Script é **no-op** (exit 0) fora de projeto trackfw, e mantém o comportamento atual dentro.
- [x] AC2 — `git-branch-guard` cabeado no escopo global nos mesmos CLIs do `credential-guard`.
- [ ] AC3 — Integridade de script global escrito pelo trackfw é verificada **independentemente** de
      haver config referenciando-o.
- [ ] AC4 — Script defasado/adulterado em `~/.trackfw/scripts/` é **acusado**; hoje passa limpo.
- [ ] AC5 — Não-regressão: verificação do `credential-guard` global, que hoje funciona, segue
      funcionando e não acusa em dobro.
- [ ] AC6 — Paridade nos 3 CLIs, com gate; conteúdo do script byte-idêntico entre escopos.
- [ ] AC7 — Cenários de falsificação (P4) com baseline **e** detecção para cada ML.
- [ ] AC8 — `make quality` verde; `trackfw validate` sem novas violações.

## 🔴 Riscos que valem para TODOS os MLs deste roadmap

1. **O template do guard é a fonte do `corrupt_literal` dos Cenários 60/61/62/63.** Mexer no
   template faz o literal deixar de casar e os cenários viram **inertes** — foi exatamente o que
   derrubou o Cenário 58 no rebase de 2026-08-16, e o gate acusou. Depois de tocar o template, rode
   `make quality` e **cole as linhas `OK [falsify/...]` dos braços de detecção**, provando que ainda
   reprovam. Exit code verde não basta.
2. **São 7 cópias do script.** Tudo sai **do gerador**, byte-idêntico. Nunca editar cópia a cópia.
3. **`check-gates-falsify.sh` é arquivo compartilhado por todos os MLs** — por isso as waves são
   **sequenciais**, não paralelas. Nenhum ML aqui roda em paralelo com outro.
4. **Falso-positivo em escopo global afeta todos os repositórios da máquina de uma vez.** É o risco
   dominante do roadmap inteiro.
5. **Não usar o binário do `PATH`** — pode estar velho, e `--version` não distingue o build. Compilar
   e usar `./bin/trackfw`.

---

## Wave 1 — No-op (bloqueia tudo o mais)

### ML-1A — Script vira no-op fora de projeto trackfw
**Status:** ✅ Concluído — reprovado na 1ª auditoria (EPIPE), fechado pelo ML-1B
· **Agente:** `apolo-tf` (`subagent_type: apolo-tf`)
**Arquivos:** template do guard no gerador Go + espelhos Node/Python + referência em `scripts/`
(7 cópias, todas pelo gerador), testes dos 3, `scripts/check-gates-falsify.sh`.

**Ação:** o script passa a sair com **0**, sem bloquear nada, quando não houver `trackfw.yaml` na
raiz do repositório corrente. Dentro de projeto trackfw, comportamento **inalterado**.

**Decisões que são suas, registre no relatório:** como localizar a raiz (subir diretórios até achar
`trackfw.yaml`? usar `git rev-parse --show-toplevel`?) e o custo disso, que roda em **toda** chamada
de ferramenta. Meça, não presuma — se subir diretórios custar caro, diga.

**Decisão registrada:** caminhada por diretórios a partir do cwd FÍSICO (`pwd -P`) usando só
parameter expansion (`${_dir%/*}`) e `test -f` — **sem** `git rev-parse --show-toplevel`. Medido:
≈0,77 ms/chamada (builtins, sem fork) contra ≈16 ms/chamada do `git rev-parse` (fork+exec) — ~21x
mais caro; `git rev-parse` também sai 128 fora de um repositório git e resolve a raiz do **git**,
não a de `trackfw.yaml` (resposta errada em submódulo/repo aninhado). Guard roda em toda chamada de
ferramenta — o custo do fork por chamada foi o discriminante decisivo.

**Critérios de aceite:**
- [x] Repo **sem** `trackfw.yaml`: `git push`, `git commit`, `git branch nova` → **exit 0**.
- [x] Repo **com** `trackfw.yaml`: bateria completa inalterada — `git push`, `git commit`,
      `switch -c/-C/--create`, `checkout -b/-q -b/--no-track -b/--orphan`, `git branch nova`,
      `worktree add -b`, `env FOO=bar git push`, `env git`, `command git` → **exit 2**;
      leitura (`git branch`, `-a`, `-r`, `--list`, `-v`, `--show-current`, `-d`, `-D`) → **exit 0**;
      prosa (`trackfw commit -m "veja: git status; git push é bloqueado"`) → **exit 0**.
- [x] Subdiretório de projeto trackfw **continua** protegido (a raiz é encontrada subindo).
- [x] 7 cópias byte-idênticas; `trackfw validate` sem divergência de integridade.
- [x] Cenário de falsificação novo (baseline + detecção) para o no-op — Cenário 64, 4 braços
      (baseline sem/com trackfw.yaml + detecção + auto-discriminação).
- [x] **Cenários 60/61/62/63 continuam reprovando** nos braços de detecção — cole as linhas
      (ver relatório em `docs/agents-working-context.md`, entrada `apolo-tf — ML-1A (2026-08-17) —
      CONCLUÍDO`, e a saída bruta do gate).
- [x] `make quality` verde.

**Evidência (colada, ver relatório completo em `docs/agents-working-context.md`):**
```
OK   [falsify/git-branch-guard/switch-c/detection-catches-bypass]: exit 0
OK   [falsify/git-branch-guard/prose-in-message/detection-catches-regression]: exit 2
OK   [falsify/git-branch-guard/env-command-prefix/detection-catches-bypass-env]: exit 0
OK   [falsify/git-branch-guard/env-command-prefix/detection-catches-bypass-command]: exit 0
OK   [falsify/git-branch-guard/checkout-flag-position/detection-catches-bypass-q-b]: exit 0
OK   [falsify/git-branch-guard/checkout-flag-position/detection-catches-bypass-no-track]: exit 0
OK   [falsify/git-branch-guard/branch-create/detection-catches-bypass-positional]: exit 0
OK   [falsify/git-branch-guard/worktree-add-b/detection-catches-bypass]: exit 0
OK   [falsify/git-branch-guard/env-var-assignment/detection-catches-bypass-single]: exit 0
OK   [falsify/git-branch-guard/env-var-assignment/detection-catches-bypass-multiple]: exit 0
OK   [falsify/git-branch-guard/no-op-outside-project/baseline-noop-without-trackfw-yaml]: exit 0
OK   [falsify/git-branch-guard/no-op-outside-project/baseline-blocks-with-trackfw-yaml]: exit 2
OK   [falsify/git-branch-guard/no-op-outside-project/detection-catches-bypass-without-trackfw-yaml]: exit 2
OK   [falsify/git-branch-guard/no-op-outside-project/detection-does-not-break-inside-project]: exit 2
Falsification checks passed (all 125 scenarios, ...)
```
`go build ./...` limpo · `go test ./...` verde · `npm test`: 611 passed, 0 failed ·
`PYTHONPATH=pypi python3 -m pytest pypi/tests/`: 1290 passed, 14 subtests · `make quality`: exit 0 ·
`bin/trackfw validate` (binário local): exit 0, 17 warnings pré-existentes não relacionados.

---

### 🔴 Auditoria do arquiteto — REPROVADA por regressão não testada

Tudo o que o ML-1A prometeu **confere**, medido por mim: no-op fora de projeto (7 payloads → exit 0),
bateria completa dentro do projeto inalterada (10 bloqueios + 4 leituras + prosa), subdiretório
profundo continua protegido, 7 cópias sem divergência de integridade.

**Mas o ML introduziu uma regressão que ele não testou.** O no-op sai em `exit 0` na **linha 42**,
**antes** de o script ler o stdin (`CMD_RAW`, linha 79). Quem escreve o JSON no pipe recebe **EPIPE**:

```
guard desta branch, fora de projeto:   guard=0  escritor=ERRO   (5/5 rodadas)
guard da main (antes do ML-1A):        guard=2  escritor=limpo  (3/3 rodadas)
```

Reprodutível em **100%** das rodadas, inclusive com payload pequeno — não é corrida de timing. Antes
deste ML **todos** os caminhos de saída ficavam depois da leitura do stdin (`:79`, `:364`); este é o
primeiro `exit` pré-leitura, logo a regressão é dele.

**Por que bloqueia:** o sintoma que originou toda esta frente foi justamente **ruído de hook** no
terminal do usuário (`hook error ... non-blocking status code` no CMDB). Trocar um erro de hook por
outro seria fechar o ciclo no lugar errado.

**Não verificado por mim:** a alegação de custo (~0,77 ms/chamada contra ~16 ms do `git rev-parse`).
Minha medição foi dominada pelo startup do Python e não discrimina. A **decisão** de usar builtins em
vez de `git rev-parse` está bem fundamentada por outros motivos que confirmei — `git rev-parse` sai
128 fora de repositório git e resolve a raiz do **git**, não a do `trackfw.yaml`, dando resposta
errada em submódulo e repo aninhado.

---

### ML-1B — Consumir o stdin antes do no-op
**Status:** ✅ Concluído — auditado por medição própria · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`)
**Arquivos:** template do guard no gerador (7 cópias), `scripts/check-gates-falsify.sh`, testes.

**Ação:** garantir que o script **consuma o stdin** antes de qualquer saída antecipada — mover a
checagem do no-op para depois da leitura, ou drenar o stdin antes do `exit 0` da linha 42. A escolha
é sua; o critério é o escritor nunca receber EPIPE.

**Cuidado:** não desfaça o ganho de custo. A parte cara era o `fork` do `git rev-parse`, não ler o
stdin — mas confirme, não presuma.

**Critérios de aceite:**
- [x] Fora de projeto trackfw: guard → **exit 0** e **escritor sem erro**, em 5 rodadas seguidas.
- [x] Payload grande (>64 KB, estoura o buffer do pipe): escritor sem erro.
- [x] Dentro do projeto: bateria completa inalterada (bloqueios, leitura, prosa) — não regrida o que
      o ML-1A acertou.
- [x] Cenário de falsificação cobrindo **o escritor não receber EPIPE** — é o que ninguém testou.
- [x] Cenários 60–64 continuam reprovando nos braços de detecção; cole as linhas.
- [x] `make quality` verde.

---

### Auditoria do ML-1B pelo arquiteto — aprovada

```
EPIPE fora de projeto     5/5 rodadas: guard=0, escritor=limpo   (antes: 5/5 ERRO)
payload de 200 KB         guard=0, escritor=limpo
dentro do projeto         11 bloqueios · 8 leituras · prosa — todos inalterados
subdiretorio profundo     git push -> exit 2
make quality              exit 0 · 126 cenarios · validate exit 0
```

Cenário 65 tem os três braços e a sabotagem é por `corrupt_literal` em
`internal/generators/scaffold.go` — **na implementação, nunca na asserção**. O braço de detecção
prova `escritor_erro=1` com o dreno removido, isolando a regressão ao dreno em si.

Cenários 60–64 continuam reprovando nos braços de detecção — o modo de falha que derrubou o
Cenário 58 no rebase de anteontem não se repetiu.

---

## Wave 2 — Fiação global (depende da Wave 1)

### ML-2A — Cabear o `git-branch-guard` no escopo global
**Status:** ✅ Concluído — pendente de auditoria do arquiteto · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`)
**Dependência:** ML-1A concluído e auditado. **Cabear antes do no-op quebra todos os repositórios.**
**Arquivos:** alvos/geradores de harness nos 3 CLIs, `scripts/check-harness-hooks-parity.sh`,
`scripts/check-gates-falsify.sh`.

**Ação:** acrescentar a fiação global do `git-branch-guard` nos mesmos CLIs em que o
`credential-guard` já é cabeado, seguindo **exatamente** o padrão dele — é o modelo de referência,
já validado por gate de paridade estrutural.

**Estado atual medido (refs por config):**
```
~/.claude/settings.json    credential-guard=2   git-branch-guard=0
~/.codex/hooks.json        credential-guard=2   git-branch-guard=0
~/.gemini/settings.json    credential-guard=2   git-branch-guard=0
~/.copilot/settings.json   credential-guard=2   git-branch-guard=0
~/.cursor/hooks.json       ausente
~/.kiro/hooks/...json      ausente
```

**Decisão registrada (Kiro, exceção estrutural):** `harnessCredentialGuardTargetKiro` reescreve
`~/.kiro/hooks/trackfw-credential-guard.json` por INTEIRO a cada run (wholesale, nunca merge).
Compartilhar esse arquivo com um segundo writer também-wholesale para o git-branch-guard faria os
dois targets flapar eternamente (falha de idempotência). Kiro recebe arquivo dedicado separado,
`~/.kiro/hooks/trackfw-git-branch-guard.json`, mesmo schema, hooks
`trackfw-git-branch-guard-global-pre`/`-global-post`. Os outros 5 CLIs reusam os mesmos merge
helpers do credential-guard (só troca o `scriptPath`) — os dois guards coexistem como 2 comandos
distintos no mesmo array `hooks` do mesmo arquivo.

**Critérios de aceite:**
- [x] Fiação global presente nos mesmos CLIs do `credential-guard`, com paridade estrutural nos 3 runtimes.
- [x] `check-harness-hooks-parity.sh` cobre a fiação nova e passa.
- [x] `update harness` é **idempotente**: rodar duas vezes não duplica entradas.
- [x] Não-regressão: fiação do `credential-guard` inalterada.
- [x] Cenário de falsificação novo (baseline + detecção) — Cenário 66, com prova adicional de
      não-regressão do Cenário 45 (label original continua passando na mesma árvore corrompida).
- [x] `make quality` verde.

**Evidência (colada, bruta):**
```
go build ./...                    → limpo
go test ./internal/commands/... ./internal/generators/... → ok (inclui os novos)
node --test npm/tests/update-harness.test.js → 66 tests, 0 fail
PYTHONPATH=pypi python3 -m pytest pypi/tests/test_update_harness.py -q → 72 passed
bash scripts/check-harness-hooks-parity.sh → 14 OK (12 credential-guard + 2 kiro git-branch-guard)
make quality (completo)           → exit 0
  go test ./...                   → todos os pacotes ok
  node --test (todos)             → 637 passed, 0 failed
  PYTHONPATH=pypi python3 -m pytest pypi/tests -q → 1316 passed, 14 subtests, 0 failed
  scripts/check-gates-falsify.sh  → 127 cenários (126→127), 0 FAIL
bin/trackfw validate (binário local recompilado) → exit 0, 17 warnings pré-existentes,
  0 novos relacionados a este ML
```

Alvos filtrados (`--targets claude-credential-guard` isolado etc.) confirmados INALTERADOS — testes
pré-existentes de credential-guard passam sem edição de expectativa.

Lista de targets cresceu de 27 para 33 ids, byte-idêntica nos 3 runtimes — confirmado por
`check-update-parity.sh` (`update-harness/target-list/three-runtimes-identical`).

Duas observações reportadas (não corrigidas, fora do escopo desta ML): (1) `validate` passa a poder
acusar `git_branch_guard_script_integrity`/`_hook_resolvable` em escopo global "de graça" assim que
o usuário rodar `trackfw update harness` de verdade — a infraestrutura já existe de um ML anterior,
este ML só a alimenta pela primeira vez; a Wave 3 formaliza isso para todo artefato global. (2) não
existe dedup projeto+global para git-branch-guard (`globalGitBranchGuardInstalled<Tool>`, análogo ao
que já existe para credential-guard) — um projeto trackfw com fiação global E de projeto rodaria o
guard duas vezes por chamada; é fiação de projeto, fora do escopo declarado desta ML. Detalhes
completos em `docs/agents-working-context.md`, entrada `apolo-tf — ML-2A (2026-08-17) — CONCLUÍDO`.

---

### Auditoria do ML-2A pelo arquiteto

```
alvos de harness      27 -> 33, lista byte-identica nos 3 runtimes
dry-run               nao escreve no HOME real (conferido: 0 refs apos dry-run)
idempotencia          HOME controlado, 2 rodadas -> 2 refs nas duas   IDEMPOTENTE
credential-guard      inalterado (2 refs, como antes)
paridade do harness   14/14 OK
Cenario 45            continua detectando, e ganhou braco provando que o
                      credential-guard nao e afetado pela corrupcao do git-branch-guard
make quality          exit 0 · 127 cenarios · validate exit 0
```

**Ratificada** a decisão do agente de dar ao Kiro um arquivo dedicado
(`~/.kiro/hooks/trackfw-git-branch-guard.json`) em vez de compartilhar o do `credential-guard`: o
escritor do Kiro reescreve o documento inteiro em vez de mesclar, então compartilhar faria os dois
alvos oscilarem para sempre. É a mesma classe de raciocínio que motivou o arquivo separado no
catálogo original.

---

### ML-2B — Dedup projeto+global para o `git-branch-guard`
**Status:** ✅ Concluído — pendente de auditoria do arquiteto · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`)
**Arquivos:** `internal/generators/agentfiles.go` + espelhos Node/Python, `scripts/check-gates-falsify.sh`, testes.

**O gap, medido:** o `credential-guard` tem `globalCredentialGuardInstalled<Tool>()` e o escopo de
projeto **pula** a fiação quando a global existe — padrão usado em **19 pontos** de
`agentfiles.go`. O `git-branch-guard` **não tem equivalente**. Com a fiação global do ML-2A somada à
de projeto, o guard roda **duas vezes** por chamada de ferramenta.

Impacto medido — o usuário vê a mensagem duplicada:
```
trackfw: git push bruto bloqueado. Use `trackfw ship`. Ver CLAUDE.md §1.
trackfw: git push bruto bloqueado. Use `trackfw ship`. Ver CLAUDE.md §1.
```

**Por que não aceito como "fora de escopo", que foi como o agente classificou:** o ML-2A pediu
seguir *"exatamente o padrão do `credential-guard`"*, e o dedup **é** parte desse padrão. Mais: esta
frente inteira nasceu de **ruído de hook** no terminal do usuário. Fechá-la introduzindo ruído novo
seria contraditório — e o ADR desta REQ argumenta que incômodo leva o usuário a desligar o guard, e
guard desligado protege zero.

**Critérios de aceite:**
- [x] Com fiação global presente, o escopo de projeto **não** cabea o `git-branch-guard`.
- [x] Sem fiação global, o escopo de projeto cabea normalmente — não-regressão.
- [x] Mensagem aparece **uma vez só** com ambos os escopos instalados; prove executando.
- [x] `credential-guard` inalterado (é o modelo, não pode mudar).
- [x] `$HOME` do teste controlado pelo fixture, nunca o real (precedente: Cenário 46).
- [x] Cenário de falsificação com baseline e detecção.
- [x] `make quality` verde.

**Escopo real (medido, 5 dos 6 alvos de `credential-guard`):** dedup adicionado em
Claude/Codex/Gemini/Cursor/Copilot — os mesmos 5 CLIs que já tinham a fiação de projeto do
`git-branch-guard` **e** ganharam alvo global no ML-2A. **Kiro não tem fiação de projeto do
`git-branch-guard`** (`InjectKiroHooks`/`inject_kiro_hooks` nunca a escreveram — confirmado por
leitura, não presumido), logo não há nada a deduplicar ali; documentado no código, não é gap.
Windsurf/AmazonQ têm fiação de projeto mas **nenhum alvo global** (ML-2A não os cobriu, mesmo
padrão do `credential-guard`, que também não tem dedup para esses dois) — consistente, não é gap
novo introduzido por este ML.

**Armadilha do "empty array vs absent key" (Cursor):** quando os dois guards (`credential-guard`
E `git-branch-guard`) têm fiação global instalada, `hooks.beforeShellExecution` não pode virar um
array vazio *presente* — tem de ficar **ausente** do JSON, replicando o que o Go já fazia para o
`credential-guard` sozinho. Os 3 stacks foram alinhados para só tocar essa chave dentro do `if`
de dedup correspondente, nunca fora dele; teste dedicado
(`TestGBGDedup_Cursor_BothGloballyInstalled_KeyAbsentNotEmpty` e espelhos Node/Python) prova a
ausência da chave, não um array de tamanho 0.

**Prova por execução (AC3), não contagem de JSON:** `TestGBGDedup_MessageAppearsOnceWhenBothScopesInstalled`
(Go) e espelhos Node/Python geram o script real (`GenerateGitBranchGuardScript`/
`GenerateGlobalGitBranchGuardScript`), escrevem a fiação global E rodam o injetor de projeto,
depois **leem e combinam** as entradas de `PreToolUse`/`Bash` dos dois arquivos
(`.claude/settings.json` do projeto E `~/.claude/settings.json`) — exatamente como o Claude Code
faz na prática — e **executam** cada script registrado com `git push`, contando quantas vezes o
processo bloqueia (`exit 2` + a razão no stderr). Braço de não-vacuidade
(`TestGBGDedup_MessageAppearsOnceWhenBothScopesInstalled_NonVacuous`) simula o estado pré-ML-2B
(as duas entradas presentes, sem dedup) e prova que a mesma metodologia de contagem por execução
reporta 2 bloqueios — não 1 — fechando a lacuna que uma asserção só-de-JSON deixaria aberta.

**Cenário de falsificação (P4):** implementado como testes de unidade nos 3 stacks
(`internal/generators/git_branch_guard_dedup_test.go`,
`npm/tests/git_branch_guard_dedup.test.js`, `pypi/tests/test_git_branch_guard_dedup.py`) — não
como cenário novo em `scripts/check-gates-falsify.sh`. Motivo: `check-agent-hooks-parity.sh`
sempre roda com `$HOME` isolado e **vazio** (nenhum guard global instalado), então o caminho
"global instalado" nunca é exercitado por esse gate — nem para o `credential-guard`, cujo próprio
dedup nunca ganhou um cenário de falsificação em nível de shell além do Cenário 46 (que ataca o
guard de vacuidade de OUTRO gate, não o mecanismo de dedup em si). Os testes de unidade cobrem
baseline (skip com global instalado), fail-open (sem global/`$HOME` corrompido, 2 casos) e
não-vacuidade por execução real de processo — mesmo padrão de rigor do Cenário 46 (baseline +
detecção + prova de não-vacuidade), só que no nível de teste que já teria pego a regressão
original do `credential-guard` em 2026-08-08.

**Débito residual explícito (não fechado, decisão do arquiteto):** o dedup do `credential-guard` é
*só de adição* — nunca remove uma entrada de projeto já escrita por um `trackfw init`/`update`
anterior. Um projeto que **já tinha** a fiação de projeto do `git-branch-guard` escrita antes do
`trackfw update harness` instalar o guard global mantém as duas entradas até o próximo
`trackfw init`/`update` **depois** de o global já estar presente — só então o dedup entra em
efeito e a entrada de projeto para de ser reescrita (ela nunca é apagada ativamente; simplesmente
deixa de ser reafirmada a cada rodada, e como `mergeClaudeHookArray`/equivalentes não removem
entradas, ela sobrevive até alguém tocar o arquivo manualmente ou até um ML futuro adicionar
remoção ativa). Não há helper de remoção no padrão do `credential-guard` para copiar. Não fechado
aqui — mesma classe de gap, mesma decisão do arquiteto sobre corrigi-lo ou não.

---

### Diagnóstico do arquiteto — o dedup está certo, a comparação de caminho é que é frágil

**O dedup funciona.** Medido com fixture próprio:
```
sem fiação global  -> projeto cabeia   (1 ref)
com fiação global  -> projeto pula     (0 refs)     credential-guard também pula
```

**Mas `make quality` reprova**, no baseline do próprio Cenário 67:
`entrada de git-branch-guard presente ... com a fiação global instalada`.

**Causa raiz, provada.** O `TMPDIR` do macOS termina em `/`, então `$WORK` do falsify vira
`/var/folders/.../T//trackfw-falsify.XXX` — com **barra dupla**. O fixture escreve o comando do hook
com `//`; o Go monta o caminho esperado com `filepath.Join`, que **normaliza** e remove a barra
duplicada. A comparação é de **string crua**:

```go
// agentfiles.go:1616
if ok && hObj["command"] == command {
```

Discriminante medido:
```
HOME sem '//'  -> refs=0   dedup dispara
HOME com '//'  -> refs=1   dedup NAO dispara
```

**Não é só bug de fixture.** `hookArrayHasCommand` é compartilhado, e o
`globalCredentialGuardInstalled*` usa o mesmo comparador — a fragilidade é **pré-existente** no
`credential-guard`, e o ML-2B apenas a herdou. Qualquer divergência inócua de forma no caminho
(barra dupla, barra final, `/tmp` vs `/private/tmp` por symlink, config escrita à mão) faz o dedup
falhar em silêncio e o guard voltar a rodar duas vezes.

É a mesma classe que atravessa esta REQ inteira: **comparação por igualdade exata onde a
comparação correta é normalizada** — igual ao ponto cego de resolvibilidade do hook, que tratava
caminho como propriedade absoluta em vez de relativa ao cwd.

**Corretivo (ML-2C):** normalizar os dois lados antes de comparar **e** corrigir o fixture para não
depender de `//`. Corrigir só o fixture deixaria o produto frágil e o gate verde — o pior par.

---

### ML-2C — Comparar caminho de hook normalizado, não string crua
**Status:** ✅ Concluído — pendente de auditoria do arquiteto · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`)
**Arquivos:** `internal/generators/agentfiles.go` (`hookArrayHasCommand`, `simpleArrayHasValue`,
`normalizeGuardPath`, `samePathCommand` novos) + espelhos `npm/src/generators/hooks.js`
(`normalizeGuardPath`, `samePathCommand`, `hasEntryPath` novos) e
`pypi/trackfw/generators/hooks.py` (`_normalize_guard_path`, `_same_path_command` novos,
`_hook_array_has_command`/`_simple_array_has_value` editados), `scripts/check-gates-falsify.sh`
(fixture do Cenário 67 + braço 4 novo), testes novos
`internal/generators/guard_path_normalize_test.go`, `npm/tests/guard_path_normalize.test.js`,
`pypi/tests/test_guard_path_normalize.py`, mais um teste de regressão em cada um dos 3 arquivos de
dedup existentes.

**Decisão de escopo (consultada e ratificada por revisão externa antes de implementar):** o mesmo
bug de comparação por string crua também afeta `simpleArrayHasValue` (Go)/`hasEntry` (Node)/
`_has_entry` (Python), usados pelo dedup de Cursor/Copilot — não só `hookArrayHasCommand`. Corrigido
nos 5 alvos (Claude/Codex/Gemini via `hookArrayHasCommand`, Cursor/Copilot via a variante read-only),
não só no citado no roadmap. **Cuidado tomado:** em Node/Python esses helpers são compartilhados
com o lado de ESCRITA (idempotência de `injectCursorHooks`/merge helpers) — normalizar ali mudaria
o comportamento de escrita e quebraria paridade com o Go (que já tinha os 4 call-sites de
`simpleArrayHasValue` 100% read-only). Solução: `hasEntryPath`/`_simple_array_has_value` novos,
usados **só** nos 4 call-sites read-only de dedup global; os helpers de escrita (`hasEntry`/
`_has_entry`) ficaram **byte-intocados**.

**Normalização implementada (`normalizeGuardPath`/`_normalize_guard_path`):** colapsa barras
duplicadas em qualquer posição (inclusive líder) e remove barra final — **não** resolve `.`/`..`
nem symlinks. Hand-rolled nos 3 stacks (não `filepath.Clean`/`path.normalize`/`os.path.normpath`)
porque **medi** que os três divergem entre si: `os.path.normpath('//a/b')` preserva a barra dupla
líder (regra POSIX), `filepath.Clean`/`path.normalize` a colapsam; `path.normalize('/a/b/')`
mantém a barra final, `filepath.Clean`/`os.path.normpath` a removem — usar os builtins teria
quebrado a paridade entre os 3 CLIs mesmo corrigindo o bug em cada um isoladamente.

**Symlink — decisão registrada:** comparação puramente lexical, sem `EvalSymlinks`/`realpath`.
Dois motivos: (1) toda função aqui responde "o guard global está fiado?" **antes** de o artefato
necessariamente existir — resolver symlink falha em caminho inexistente, e todo caller é fail-open,
então o erro viraria um `false` silencioso, exatamente a classe de falha que esta ML existe para
fechar; (2) o custo de falhar é assimétrico — sub-normalizar faz o guard rodar 2x (ruído), sobre-
normalizar faz o dedup disparar para um arquivo que não é o mesmo (guard some, falha de segurança
silenciosa). Vieso para a transformação mais estreita: `normalizeGuardPath` é puramente sintática e
só colapsa formas que lexicamente denotam o mesmo caminho.

**Fixture do Cenário 67 corrigida sem tocar `$WORK` global:** `T67_FAKE_HOME` agora deriva de
`WORK67_CLEAN` (uma forma de `$WORK` com barras colapsadas via `sed`), não de `$WORK` bruto — o
baseline deixa de depender de `$TMPDIR` terminar em `/` (macOS). A tolerância a `//` em si passou a
ser exercitada **explicitamente** por um braço 4 novo, com `HOME` sintético carregando `//`
deliberadamente embutido no meio do caminho e o comando gravado no JSON usando essa forma crua
(simulando config editado à mão ou capturado antes da normalização) — o binário real deve continuar
deduplicando (entrada de projeto ausente).

**Teste negativo (não estava no AC original, adicionado por indicação da revisão):** caminhos
genuinamente diferentes não podem comparar iguais — `TestSamePathCommand_DifferentPathsDoNotMatch`
(Go) e espelhos Node/Python cobrem `/home/alice/...` vs `/home/bob/...`, `/a/b` vs `/a/bb`, guard
diferente no mesmo diretório, e prefixo vs caminho completo. É a prova de não-sobre-normalização
que uma bateria só-de-`//` não fecharia.

**Critérios de aceite:**
- [x] `hookArrayHasCommand` normaliza ambos os lados antes de comparar (barra dupla, barra final).
- [x] Discriminante: `HOME` com `//` passa a deduplicar — antes não deduplicava.
- [x] Fixture do Cenário 67 deixa de depender de `//`, e o **produto** também passa a tolerá-lo
      (braço 4 novo).
- [x] **Não-regressão do `credential-guard`:** dedup dele inalterado; **Cenário 46 continua
      passando** (4 sub-braços: baseline, detected, discriminant, structural-comparator-not-reached).
- [x] Cenários 60–67 verdes; `make quality` verde; `./bin/trackfw validate` exit 0 (17 warnings
      pré-existentes não relacionados).

**Evidência (colada, bruta):**
```
go build ./...                                          → limpo
go test ./... (inclui internal/generators)               → ok, todos os pacotes
  TestNormalizeGuardPath_Table                            → PASS
  TestSamePathCommand_ToleratesDoubleSlashAndTrailingSlash → PASS
  TestSamePathCommand_DifferentPathsDoNotMatch             → PASS
  TestGBGDedup_Claude_SkipsProjectEntry_ToleratesDoubleSlashInStoredCommand → PASS
cd npm && npm test                                        → 651 passed, 0 failed
  ✔ normalizeGuardPath collapses double slashes and strips trailing slash
  ✔ samePathCommand tolerates the double-slash formatting produced by a trailing-slash $TMPDIR
  ✔ samePathCommand does not match genuinely different paths
  ✔ injectClaudeHooks skips project-scope git-branch-guard despite // formatting in stored global command
PYTHONPATH=pypi python3 -m pytest pypi/tests -q             → 1330 passed, 28 subtests passed
make quality (completo)                                   → exit 0
  scripts/check-gates-falsify.sh                            → 128 cenários, 0 FAIL
    OK   [falsify/agent-hooks-parity/credential-guard-present-vacuity/baseline]
    OK   [falsify/agent-hooks-parity/credential-guard-present-vacuity/detected]
    OK   [falsify/agent-hooks-parity/credential-guard-present-vacuity/discriminant]
    OK   [falsify/agent-hooks-parity/credential-guard-present-vacuity/structural-comparator-not-reached]
    OK   [falsify/git-branch-guard/switch-c/...] ... [falsify/git-branch-guard/no-op-outside-project/...] (Cenários 60–64, todos OK)
    OK   [falsify/git-branch-guard-dedup/baseline-skips-project-entry]
    OK   [falsify/git-branch-guard-dedup/baseline-credential-guard-unaffected]
    OK   [falsify/git-branch-guard-dedup/reverse-vacuity]
    OK   [falsify/git-branch-guard-dedup/detection-catches-regression]
    OK   [falsify/git-branch-guard-dedup/double-slash-tolerance]     ← braço 4 novo
./bin/trackfw validate (binário local recompilado)         → exit 0, 17 warnings pré-existentes,
  0 novos relacionados a este ML
```

---

### Auditoria do ML-2C — aprovada, e extensão de escopo ratificada

```
discriminante '//'      refs 1 -> 0     dedup passa a disparar
nao-regressao           sem global, projeto cabeia (1)   guard NAO sumiu
nao normalizou demais   caminho parecido porem diferente NAO deduplica (1)
idempotencia harness    3 rodadas -> gbg=2 cg=2 constante
idempotencia discover   3 rodadas -> gbg=1 constante
Cenario 46              4 bracos passam (credential-guard intacto)
Cenario 67              5 bracos, incluindo double-slash-tolerance
make quality            exit 0 · 128 cenarios · validate exit 0
```

**Extensão de escopo ratificada.** O ML pedia `hookArrayHasCommand`; ele estendeu para
`simpleArrayHasValue`/`hasEntry`/`_has_entry`, que tinham o **mesmo** defeito nos alvos de
Cursor/Copilot. Corrigir metade deixaria o mesmo bug vivo em dois CLIs, com gate verde — pior que
não corrigir, porque criaria a impressão de cobertura. A separação leitura/escrita que ele fez por
linguagem (só os call sites de leitura mudaram; o lado de escrita ficou intacto) é o que preserva a
idempotência, e a medição acima confirma que preservou.

**Risco inverso verificado por mim:** normalizar demais faria o dedup casar caminhos diferentes e o
guard sumir do projeto — falha silenciosa de segurança, pior que a duplicação corrigida. Testei com
caminho deliberadamente parecido e ele **não** deduplica.

---

## Wave 3 — Integridade independente de fiação (depende da Wave 2)

### ML-3A — Verificar integridade de script global mesmo sem config apontando para ele
**Status:** ⬜ Pendente · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`)
**Arquivos:** `internal/validator/validator_git_branch_guard.go` e/ou
`validator_credential_guard.go` + espelhos Node/Python, `scripts/check-gates-falsify.sh`.

**O ponto cego a fechar, com o mecanismo já provado:** `validateGuardGlobalScriptIntegrity` só avalia
os configs que **referenciam** o `scriptMarker`. Sem fiação, o laço nunca entra e a regra nunca roda
— foi assim que 3 versões passaram. **A verificação de integridade está condicionada à fiação, e
deveria estar condicionada à existência do artefato:** se o trackfw escreveu o script, o trackfw
verifica o script.

> A Wave 2 faz a regra passar a rodar **de graça** para o `git-branch-guard`. Este ML existe porque
> isso **não deve depender** da fiação — qualquer artefato global futuro cai no mesmo buraco.

**Critérios de aceite:**
- [ ] Script global presente e **divergente** do template é acusado, **mesmo sem** config referenciando.
- [ ] Script global **ausente** não é acusado (não é erro não ter instalado).
- [ ] Não-regressão do `credential-guard` global: continua sendo verificado, **sem duplicar** o aviso
      agora que há dois caminhos possíveis de disparo.
- [ ] `$HOME` do teste é **controlado pelo fixture**, nunca o real — há precedente de vazamento de
      ambiente neste tipo de cenário (Cenário 46, ML-1B de 2026-08-12).
- [ ] Cenário de falsificação (baseline + detecção), com prova de não-vacuidade.
- [x] `make quality` verde.

---

## Wave 4 — Barreira

### ML-4A — `hades-tf`: revisão do guard em escopo global
**Status:** ⬜ Pendente · **Agente:** `hades-tf` (`subagent_type: hades-tf`)
**Escreve:** `docs/seguranca/2026-08-17-revisao-do-guard-em-escopo-global.md`

**Ações:** o no-op é **superfície de ataque nova** — avaliar se dá para induzir o no-op dentro de um
projeto trackfw (cwd manipulado, `trackfw.yaml` removido/renomeado, symlink, subdiretório com
`trackfw.yaml` falso). Avaliar se a fiação global introduziu caminho de desarme. Confirmar que a
integridade nova não é vacuosa. **Veredito explícito; bloquear é saída legítima.**

---

## Notas
- **Fora deste roadmap:** o escopo de projeto (`validate` não detectar hook na forma relativa antiga)
  tem REQ e roadmap próprios, **depois** deste — mesmos arquivos de validador, não pode ser paralelo.
- **Fora de escopo, já declarado no ADR:** repositório sem `trackfw.yaml` não é protegido. Deliberado.
- Commits e branch são exclusivos do `trackfw_architect`.
