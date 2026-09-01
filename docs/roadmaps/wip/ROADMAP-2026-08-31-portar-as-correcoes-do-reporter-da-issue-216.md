---
status: wip
date: 2026-08-31
req: "docs/req/REQ-2026-08-31-portar-as-correcoes-dos-prs-222-225-do-reporter-da-issue-216-defeitos-1-2-3-5-e-6-de-windows.md"
squad: "hades-tf, apolo-tf, hefesto-tf"
---

# Roadmap: Portar as correções do reporter da issue #216

> Created: 2026-08-31 | Status: wip

## Context

REQ: `docs/req/REQ-2026-08-31-portar-as-correcoes-dos-prs-222-225-do-reporter-da-issue-216-defeitos-1-2-3-5-e-6-de-windows.md`

`lourivalgarciajunior` reportou o issue #216 e enviou quatro PRs com as correções. Fechados para não
colidir com nosso ciclo; **a análise concluiu que os quatro valem inteiros**
(`docs/analises/2026-08-31-aproveitamento-dos-prs-222-225.md`).

**Duas regras valem para todos os MLs:**

1. 🔴 **Atribuição:** todo commit carrega `Co-Authored-By: lourivalgarciajunior <lourival.garcia@gmail.com>`.
2. 🔴 **Porte fiel.** Estes diffs foram revisados **como estão escritos**. "Melhorar" em trânsito
   destrói a revisão que justificou aceitá-los. Se achar que algo deveria mudar, **pare e avise**.

Acesso ao conteúdo: os PRs estão `CLOSED` mas `gh pr diff 222|223|224|225` continua funcionando.
**Não faça `gh pr checkout`** — worktree compartilhado.

## Acceptance Criteria

- [ ] Defeitos 1, 2, 3, 5 e 6 corrigidos, portados fielmente, com os testes
- [ ] Atribuição em todo commit
- [ ] Contagem cai para **3 `REPRODUCED`** (itens 4, 7, 10) em runner Windows real
- [ ] Cada item que sai de `REPRODUCED` explicado no roadmap, citando o run
- [ ] `docs/cli-parity.md` ganha os dois contratos (LF na escrita, UTF-8 na saída)
- [ ] `make quality` e **CI** verdes

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat Model (escopo estreito, de propósito)
> Dependências: nenhuma. Bloqueia **apenas** a Wave 2.

### ML-0A — Mudança de âncora de confiança do #222 Grupo A
**Status:** ✅ Concluído
**Agente:** `hades-tf`
**Files affected:** nenhum (documento em `docs/seguranca/`)
**Escopo estreito de propósito:** **não** remodelar os outros três PRs. `hefesto-tf` os avaliou e o
arquiteto auditou essa avaliação. A única pergunta de confiança genuinamente nova é esta.
**Actions:**
1. O #222 Grupo A muda a âncora de `validateGuardGlobalHookResolvable` de **API nativa do SO** para
   **`$HOME` env-var-first**. Env var é controlável pelo processo pai; API nativa não é. Avalie: quem
   controla `$HOME` no momento da validação, o que ganha, e se isso degrada a garantia do guard.
2. Falsificação nas duas direções: o que quebra se a âncora regride, e o que quebra se a validação
   ficar **estrita demais** e recusar ambiente legítimo (Windows sem `$HOME`, CI, contêiner).
3. Residual declarado.
**Critérios de aceite:**
- [x] Veredito explícito sobre a troca de âncora, com vetor concreto se houver
- [x] Nenhuma linha de implementação escrita
- [x] Parecer em `docs/seguranca/2026-08-31-ancora-de-confianca-do-guard-global-home-first.md`

**Gates da wave:**
```bash
test -f docs/seguranca/2026-08-31-ancora-de-confianca-do-guard-global-home-first.md
! grep -qi "placeholder" docs/seguranca/2026-08-31-ancora-de-confianca-do-guard-global-home-first.md
grep -q "Residual" docs/seguranca/2026-08-31-ancora-de-confianca-do-guard-global-home-first.md
```

#### Resultado do ML-0A (hades-tf, 2026-08-31) — auditado pelo arquiteto

**Veredito: (b) — segura no eixo perguntado, com ressalva concreta e diferente.**

**Ele corrigiu a premissa do meu próprio handoff.** Eu descrevi a mudança como *"API nativa do SO →
`$HOME` env-var-first"*. **Errado**, e verifiquei no fonte antes de aceitar:

```go
// $(go env GOROOT)/src/os/file.go — os.UserHomeDir()
case "windows":
    env, enverr = "USERPROFILE", "%userprofile%"
if v := Getenv(env); v != "" { return v, nil }
```

**A âncora JÁ era env var** nos três runtimes, antes e depois do PR — Go e Python resolvem home no
Windows por `USERPROFILE`, sem chamar API do Windows em lugar nenhum. O PR só muda **qual variável
tem prioridade**, e só no Windows.

**Consequência:** um processo pai que controla o ambiente **não ganha capacidade nova** — já podia
redirecionar a resolução via `USERPROFILE` antes. Isso já é modelo de ameaça aceito pelo
`ADR-2026-08-12` e pela revisão de 2026-08-18. Eu herdei a formulação imprecisa da análise e a
repeti sem checar; ele foi ao fonte.

### O vetor real, e é outro: **instalação fantasma**

`UpdateHarness` (escritor) e `validateGuardGlobalHookResolvable` (auditor) passam a usar a mesma
`homedir.Dir()` — **consistentes entre si**, mas não necessariamente com o **CLI de agente real**
(Claude Code, Codex, Gemini), que é binário de terceiro e continua resolvendo home pelo mecanismo
nativo dele, tipicamente `USERPROFILE`-first.

Se `$HOME` e `%USERPROFILE%` divergirem num Windows legítimo — **cenário plausível sob Git Bash, que
é o próprio ambiente do reporter da issue #216** —, o `trackfw` escreve e audita um guard global
saudável **num caminho que o agente real nunca lê**.

**Isso é pior que silêncio: é falso positivo de saúde.** O guard é controle de negação; um controle
que se reporta saudável enquanto está inerte é a forma de falha mais cara que existe — e é a mesma
que já nos mordeu duas vezes nesta REQ (o `success()` implícito e o `VERDICT=ABSENT` por vacuidade).

Ele **não fechou sozinho** por falta de runner Windows, e nomeou a lacuna em vez de assumir.

### Decisão do arquiteto sobre o escopo

O port do Grupo A **segue na Wave 2 como está** — é seguro no eixo perguntado, e o reporter escolheu
deliberadamente o fix de produção por objetivo legítimo.

**A detecção de instalação fantasma NÃO entra aqui.** Adicionar finding novo ao `trackfw validate`
é *feature*, não *port*, e violaria a regra de porte fiel que governa esta REQ. **Vira REQ própria**,
aberta em seguida para não se perder — com a medição certa, que não é comparar as duas env vars entre
si, mas **medir onde o consumidor real lê o `settings.json`** a partir de cada terminal.

**Restrição que passa para o ML-2A:** não carregar a alegação *"API nativa → env var"* para lugar
nenhum. É imprecisa e eu a coloquei em circulação.

## Wave 1 — Bit de execução e CRLF (2 MLs em paralelo)
> Dependências: nenhuma. **Arquivos disjuntos entre os dois MLs** — verificado na análise.

### ML-1A — #222 Grupo B: bit de execução no validator (item 3)
**Status:** ✅ Concluído
**Agente:** `apolo-tf`
**Fonte:** `gh pr diff 222`, **só o grupo do bit de execução** (não o grupo `$HOME`, que é a Wave 2).
**Atenção:** distinto da `REQ-2026-08-28`, que cobriu apenas `scaffold_doctor.go`. Este é o
**validator**. Confirme que não há sobreposição antes de editar.
**Critérios de aceite:**
- [ ] Port fiel do grupo do bit de execução, com os testes
- [ ] `Co-Authored-By: lourivalgarciajunior <lourival.garcia@gmail.com>` — o commit é meu, mas a
      autoria é dele; me devolva a mensagem sugerida
- [ ] Não toca os arquivos do Grupo A (`$HOME`)
- [x] `make quality` verde

### ML-1B — #225: geradores Python escrevem LF, não CRLF (item 5)
**Status:** ✅ Concluído
**Agente:** `apolo-tf`
**Fonte:** `gh pr diff 225`.
**Inclui** o gate `check-python-writes-lf.sh` que o PR introduz.
**Critérios de aceite:**
- [ ] Port fiel, com testes e com o gate
- [ ] O gate falsifica nas duas direções e tem guarda de vacuidade
- [x] `make quality` verde

**Gates da wave:**
```bash
make quality
```

### ML-1C — Ligar o gate do #225 e dar-lhe guarda de vacuidade (desvio autorizado)
**Status:** ✅ Concluído
**Agente:** `apolo-tf`
**Files affected:** `Makefile` (alvo `parity:`), `scripts/check-python-writes-lf.sh`
**Origem:** `apolo-tf` levantou dois pontos no ML-1B e **parou para perguntar**, como mandei. Auditei
os dois e confirmei:

1. **O gate não está ligado a lugar nenhum.** Não aparece em `Makefile` nem em workflow. Verifiquei
   `gh pr diff 225`: **o PR original também não o liga** — não é omissão do port, o gate nasceu inerte.
   `make quality` verde **não é evidência sobre ele**, porque ele nunca é invocado.
2. **Não tem guarda de vacuidade.** `set -euo pipefail` fecha o caminho do interpretador falhar, mas
   se a varredura rodar e **não visitar arquivo nenhum** (`pypi/trackfw/` movido, renomeado, filtro
   quebrado), `OFFENDERS` fica vazio e o gate passa em silêncio.

**Por que isto NÃO viola a regra de porte fiel.** A regra existe para proteger a **lógica revisada** —
o que o gate procura e como decide. Nenhuma das duas mudanças toca isso. Elas fazem a lógica
**executar** e **poder falhar**. Um gate que nunca roda e não consegue reprovar não é um controle: é
um arquivo. Portar o arquivo sem portar o controle seria porte fiel da forma e infiel da função.

E é exatamente a classe de falha que nos mordeu quatro vezes nesta sessão — **mecanismo dando sinal
verde enquanto o controle está inerte**: o `success()` implícito, o `VERDICT=ABSENT` por vacuidade,
o `tail` engolindo o exit code, o gate com base `origin/main` errada.

**Actions:**
1. Adicionar `scripts/check-python-writes-lf.sh` ao alvo `parity:` do `Makefile`, no formato dos gates
   vizinhos.
2. Guarda de vacuidade seguindo o padrão já usado em `scripts/check-agent-hooks-parity.sh` e irmãos:
   afirmar que a varredura **visitou** o corpus esperado, e reprovar se visitou zero.
**Critérios de aceite:**
- [x] O gate roda em `make quality` — provado por saída, não por asserção
- [x] 🔴 Guarda de vacuidade falsificada: com o corpus artificialmente vazio, o gate **reprova**
- [x] A lógica de detecção do PR original **não** é alterada
- [x] `make quality` verde

#### Resultado da Wave 1 (apolo-tf, 2026-08-31) — auditado pelo arquiteto

**Os três MLs entregues, e o comportamento que mais quero registrar é o de ter parado para
perguntar.** Ele encontrou dois problemas no ML-1B que a instrução de *porte fiel* o impedia de
resolver sozinho, e **trouxe a decisão em vez de decidir**. Se tivesse "consertado" em silêncio, eu
teria mergeado um gate inerte achando que estava coberto.

**Auditoria independente — verifiquei, não aceitei por relato:**

| verificação | resultado |
|---|---|
| Grupo A (`$HOME`) intocado | `grep` por `homedir\|UserHomeDir\|expanduser` no diff → **vazio** |
| gate ligado | `make -n parity` **expande** `check-python-writes-lf.sh` (linha 8) |
| vacuidade, corpus normal | `exit=0` |
| vacuidade, corpus vazio | `exit=1`, *"scan visited zero .py files — refusing to pass silently"* |
| lógica de detecção | inalterada vs. `gh pr diff 225` |

**A guarda de vacuidade quase nasceu vácua — e ele pegou sozinho.** A primeira versão ancorava em
`ROOT_DIR` via `cd`, ancoradouro que a varredura Python (`os.walk` relativo ao cwd do chamador)
**não tem**. A guarda passaria de qualquer diretório enquanto a varredura real via zero arquivos —
exatamente o silêncio que ela existe para impedir. **Uma guarda de vacuidade vácua** é a versão
recursiva do defeito que este ML corrige, e teria sido invisível em qualquer teste rodado da raiz do
repositório.

**Cinco desvios do diff literal, todos declarados, nenhum silencioso:** `python`→`python3`
(inexistente no ambiente e em todos os outros `check-*.sh`); modo 755 em vez de 644 (o `parity:`
invoca gates sem `bash` na frente — decisão que o ML-1C depois justificou); manteve **verbatim** um
par duplicado cosmético no teste do PR em vez de "limpar"; retarget do Cenário 81 em
`check-gates-falsify.sh`, cujo `sed` pinava uma linha de fonte que o guard novo quebrou (com nota de
vault); e uma **correção de precisão na análise** — o PR *guarda* `os.access(X_OK)` com checagem de
plataforma, **não** unifica o mecanismo Go/Node vs Python como estava escrito. A divergência fora do
Windows permanece, não relacionada a este port.

**Limite declarado sem simulação:** os itens 3 e 5 só se manifestam em Windows. Os testes passam
aqui como **guarda de regressão**, não como reprodução — os próprios docstrings dizem isso. A
verificação real é o CI.

## Wave 2 — `$HOME` nos 3 runtimes (item 2)
> Dependências: **Wave 0 aprovada**. Não iniciar antes do veredito do `hades-tf`.

### ML-2A — #222 Grupo A: `$HOME` nos 3 runtimes
**Status:** ⬜ Pendente
**Agente:** `apolo-tf`
**Fonte:** `gh pr diff 222`, **só o grupo `$HOME`**.
**Critérios de aceite:**
- [ ] Port fiel, com testes. A Wave 0 **liberou o port como está** — o vetor que ela achou
      (instalação fantasma) vira REQ própria, não entra aqui
- [ ] 🔴 **Não repetir a alegação "API nativa → env var"** em comentário, commit ou doc: a âncora já
      era env var nos 3 runtimes. Formulação imprecisa posta em circulação pelo arquiteto
- [ ] Paridade nos 3 runtimes — este defeito é dos três
- [x] `make quality` verde

## Wave 3 — cp1252 e isatty (sequencial: diffs empilhados)
> Dependências: Wave 1 completa. **ML-3B depois de ML-3A** — o diff do #224 **contém** o do #223.

### ML-3A — #223: UTF-8 na saída do CLI Python (item 1)
**Status:** ⬜ Pendente
**Agente:** `apolo-tf`
**Fonte:** `gh pr diff 223`.
**Correção na origem:** `_force_utf8_output()` chamada no início de `main()`, reconfigurando
`sys.stdout`/`sys.stderr` **dentro do processo**, sem depender de env var.
🔴 **Isto NÃO corrige o item 4.** O item 4 é um `print()` cru em
`scripts/check-parity-contract-coverage.sh`, que nunca entra em `main()`. Não tente corrigi-lo aqui.
**Critérios de aceite:**
- [ ] Port fiel, incluindo os testes (`TestCliEmConsoleCp1252` reproduz console cp1252 em **qualquer
      SO** via `PYTHONIOENCODING=cp1252` — roda no CI Linux todo dia)
- [ ] Item 4 **não** tocado
- [x] `make quality` verde

### ML-3B — #224: `isatty()` mente `True` para `NUL` (item 6)
**Status:** ⬜ Pendente
**Agente:** `apolo-tf`
**Fonte:** `gh pr diff 224`. **O diff dele inclui o do #223** — porte só o que for do item 6.
**Critérios de aceite:**
- [ ] Port fiel apenas da parte do item 6, sem duplicar o ML-3A
- [x] `make quality` verde

## Wave 4 — Contratos em `docs/cli-parity.md`
> Dependências: Waves 1 e 3 completas.

### ML-4A — Escrever os dois contratos que as correções passam a impor
**Status:** ⬜ Pendente
**Agente:** `hefesto-tf`
**Files affected:** `docs/cli-parity.md`
**Diagnóstico:** o #225 introduz um gate que **impõe** um contrato que **não está escrito em lugar
nenhum**. Gate sem contrato escrito é exatamente o drift que aquele documento existe para impedir.
**Actions:**
1. *"Os 3 runtimes escrevem artefato em LF"* — imposto por `check-python-writes-lf.sh`.
2. *"Os 3 runtimes escrevem UTF-8 na saída, independente da codepage do console"* — cumprido pelo
   `_force_utf8_output()`.
**Critérios de aceite:**
- [ ] Os dois contratos escritos no formato das seções existentes, apontando o gate que os impõe
- [x] `make quality` verde

## Verificação que só o CI fecha

A **contagem cair para 3 `REPRODUCED`** só se verifica em runner Windows real, no push. Verde local
não prova. Cada transição `REPRODUCED → ABSENT` é explicada aqui citando o run.

## Barreira final

`hefesto-tf` e `hades-tf` sobre o diff completo, auditoria do arquiteto, `barrier`. **CI verde** — e
aqui "verde" significa a camada 2 reportando **3**, com os cinco itens corrigidos explicados.
