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
- [ ] AC2 — `git-branch-guard` cabeado no escopo global nos mesmos CLIs do `credential-guard`.
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
**Status:** ⬜ Pendente · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`)
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

**Critérios de aceite:**
- [ ] Fiação global presente nos mesmos CLIs do `credential-guard`, com paridade estrutural nos 3 runtimes.
- [ ] `check-harness-hooks-parity.sh` cobre a fiação nova e passa.
- [ ] `update harness` é **idempotente**: rodar duas vezes não duplica entradas.
- [ ] Não-regressão: fiação do `credential-guard` inalterada.
- [ ] Cenário de falsificação novo (baseline + detecção).
- [ ] `make quality` verde.

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
- [ ] `make quality` verde.

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
