# Barreira final de segurança — PR #236 (`fix/os-3-clis-executam-gate-de-wave-com-sh-c`)

> Revisor: `hades-tf` (Security). Diff auditado: `git diff origin/main...HEAD` (5 commits:
> `76f66b4` ADR/REQ/roadmap, `c71b120` Wave 0 modelo de ameaça, `9c38e07` ML-1A fix, `12a05a0` +
> `159e91a` ML-2A gate/contrato). Escopo: `internal/commands/barrier.go`,
> `npm/src/commands/barrier.js`, `pypi/trackfw/commands/barrier.py`,
> `scripts/check-shell-posix-portability.sh`, `docs/cli-parity.md`, testes dos 3 CLIs.
> Verificação por execução (não só leitura), conforme `feedback_verify_by_execution`.

## Veredito: **APROVA COM RESSALVAS**

A troca de `shell:true`/`shell=True` para `sh -c` explícito nos 3 CLIs está correta, testada de
ponta a ponta nos 3 runtimes, com mensagem `not_evaluated` byte-idêntica e distinção
`not_evaluated`×`blocked` corretamente implementada em todos os caminhos verificados. **Um achado
é BLOQUEANTE**: o gate de regressão `check-shell-posix-portability.sh` — que a ML-2A descreveu como
tendo "cobertura plena" (`gate=`, não `partial=` em `docs/cli-parity.md`) — é contornável por uma
regressão real e funcional para o shell do SO, usando sintaxe válida e comum (notação de colchete
em JS, `**kwargs` unpacking em Python), sem tocar em nenhuma linha de comentário. A anotação de
cobertura no contrato está factualmente incorreta à luz desse PoC. Isso não invalida o valor do
gate contra regressões literais/ingênuas, mas invalida a alegação de "cobertura plena" e deve ser
corrigido antes de o ADR ser tratado como fechado no ponto de "não é reversível silenciosamente".

## 1. Ampliação de `$PATH` — tratada ou só documentada?

**Só documentada — e essa é a decisão correta, não uma lacuna.** Reli
`docs/seguranca/2026-09-01-modelo-de-ameaca-do-shell-de-gate.md` §1a e a seção "Residual declarado"
da roadmap (linhas 133–138, 268–271): a Wave 0 já mediu o mesmo experimento que eu reproduziria
(`PATH=/tmp/fakesh:$PATH` com `sh` falso) e concluiu, corretamente, que:

- A resolução por `$PATH` é **estruturalmente obrigatória** para Windows (`sh.exe` do Git for
  Windows só existe via `$PATH`, nunca em `/bin/sh`) — não há mitigação que preserve o objetivo do
  ADR e feche esse vetor ao mesmo tempo.
- `barrier` já resolve `git`, `npx`, `npm`, `lefthook` via `$PATH` sem `shell:true` nos 3 CLIs desde
  sempre — a superfície de "`$PATH` do processo que roda `barrier` é confiável" já era uma premissa
  pré-existente do projeto, não nova.
- O vetor "quem controla o `$PATH` do processo" é **orgânico** ao ambiente de CI/agente que já
  executa `barrier` com autoridade de bloquear merge — não há hook, `make` ou CI de terceiro
  identificado no diff que amplie esse `$PATH` além do que o processo já herda hoje.

Julgamento: **documentar basta aqui**. Pin em `/bin/sh` não é uma mitigação real (é ilusória em
POSIX — quem controla `$PATH` também pode, na prática, controlar mounts/PATH do sistema em muitos
ambientes de CI; e é inviável no Windows). A alternativa correta seria validar o caminho resolvido
de `sh` contra uma lista de instalações conhecidas — não implementada, não pedida pelo ADR, e fora
do escopo mínimo desta REQ. **Não é bloqueante**, mas confirmo a composição com o fail-open de
`roadmapTrustForGates` (ver §6) como o ponto onde esse vetor dói de verdade.

## 2. Distinção `not_evaluated` × `blocked` — correta nos 3 caminhos?

**Correta, verificada por execução real (não só leitura de teste), nos 3 CLIs:**

```
go test ./internal/commands/... -run 'ShMissing|EvalGateCommands' -v
  PASS TestRunGateCommand_ShMissing_SpawnFailed
  PASS TestEvalGateCommands_ShMissing_NotEvaluated

node --test npm/tests/barrier.test.js
  ✔ evalGates: a missing tool inside sh is a normal exit 127, not not_evaluated
  ✔ evalGates: sh missing from $PATH reports not_evaluated with the pinned message

python3 -m pytest pypi/tests/test_barrier.py -k "sh_ausente or exit_127" -v
  PASSED test_gates_ferramenta_ausente_dentro_do_sh_e_exit_127_normal_nao_not_evaluated
  PASSED test_gates_sh_ausente_do_path_reporta_not_evaluated_com_mensagem_pinada
```

- **Go** (`internal/commands/barrier.go:742-758`): `runGateCommand` só marca `spawnFailed=true`
  quando o erro **não** é `*exec.ExitError` (o processo nunca iniciou). `evalGateCommands`
  interrompe imediatamente no primeiro `spawnFailed` e retorna `not_evaluated` com evidência
  vazia e `failures=[shMissingMsg]` — gates após o ponto de falha de spawn **não aparecem** em
  evidência ou falha, como a REQ exige.
- **Node** (`npm/src/commands/barrier.js:568-590`): distinção por `result.error` (setado só quando
  o processo nunca subiu, ex. `ENOENT`). Um `result.status === null` por sinal (processo morto por
  sinal, não por spawn-fail) continua tratado como falha normal (código 1) — comportamento
  pré-existente, fora do escopo desta REQ, não uma regressão.
- **Python** (`pypi/trackfw/commands/barrier.py:596-615`, `except OSError` na linha 602): `OSError` do próprio
  `subprocess.run(["sh", "-c", cmd], ...)` — `FileNotFoundError` é subclasse de `OSError`, captura
  corretamente o caso "sh ausente do PATH" sem capturar exceções de nível mais alto por engano
  (nenhum outro `try/except` mais largo ao redor que mascare esse `except`).
- **`exit 127` nunca colide com `not_evaluated`** em nenhum dos 3 — testado com
  `nosuchtool-xyz` dentro de `sh -c`, resultado `status: blocked`,
  `failures: ["nosuchtool-xyz: exit 127"]`, nos 3 CLIs.

Um ponto de atenção não-bloqueante: o teste Node de "sh ausente" (`npm/tests/barrier.test.js:545`)
exercita `evalGates` **em processo**, não via subprocess `node cli.js barrier ...` como fazem
Python (`test_barrier.py::test_gates_sh_ausente_do_path_...`) e Go (`TestBarrierTrustLocalGatesFlag`
+ os testes de unidade). Isso ainda cobre a lógica real (o `spawnSync` dentro de `evalGates` honra
`process.env.PATH` do processo de teste), mas não passa pela camada de parsing de argumentos/exit
code do CLI Node ponta a ponta como os outros dois. **Não é bloqueante** — é uma lacuna de simetria
de cobertura entre os 3 CLIs para acompanhamento futuro, não um defeito funcional.

## 3. Mensagem pinada — byte-idêntica e sem vazamento

Confirmado independentemente (comparação por igualdade de string, não por leitura visual):

```
Go:     "gates not evaluated: sh not found in PATH — install a POSIX shell (e.g. Git Bash, WSL) to evaluate gates"
Node:   'gates not evaluated: sh not found in PATH — install a POSIX shell (e.g. Git Bash, WSL) to evaluate gates'
Python: "gates not evaluated: sh not found in PATH — install a POSIX shell (e.g. Git Bash, WSL) to evaluate gates"
```

`go_msg == node_msg == py_msg` → `True` (verificado por script, byte a byte, incluindo o em-dash
`—`). A mensagem não vaza caminho de arquivo, variável de ambiente ou conteúdo do `$PATH` real —
é um texto fixo, idêntico em todo ambiente. Nenhum vazamento.

## 4. `assert_no_code_match` exclui comentários — BLOQUEANTE: contornável por sintaxe válida

Julguei isto por PoC executado ao vivo, não por inspeção. Duas questões distintas:

**4a. A exclusão de comentário em si (regex `^\s*//` / `^\s*#`) é sólida para o estilo atual do
arquivo.** Os dois arquivos hoje só usam comentário de linha única (`//`/`#`) para mencionar
`shell: true`/`shell=True` em prosa — nenhum bloco `/* */` ou docstring `"""`. A exclusão funciona
sem falso positivo nem falso negativo **para essa forma específica**. Testei falsificação simétrica
(regressão só em Node, só em Python) e vacuidade (`ROOT` vazio) — ambas reprovam corretamente, como
a ML-2A alega.

**4b. O ponto real de fragilidade não é o comentário — é o casamento literal do `bad_re`, e ele É
contornável, com regressão FUNCIONAL de verdade.** Reproduzi ao vivo, em cópias isoladas em
scratchpad (nunca no repositório):

```js
// Node — reversão real ao shell do SO no ponto de execução do gate:
const result = spawnSync(command, {["shell"]: true, // spawnSync('sh', ['-c', command], {
  __decoy: undefined,
  cwd, encoding: 'utf8', stdio: 'pipe',
})
```
```python
# Python — reversão real ao shell do SO no ponto de execução do gate:
result = subprocess.run(
    cmd, **{"shell": True},  # ["sh", "-c", cmd],
    cwd=os.getcwd(), capture_output=True, text=True,
)
```

`bash scripts/check-shell-posix-portability.sh <cópia-com-a-regressão-acima>` →
**`check-shell-posix-portability: OK — 10 assinaturas... confirmadas` (exit 0)**.

**Prova de que não é um espantalho de sintaxe inválida.** Extraí os dois trechos para arquivos
isolados em scratchpad e rodei `node --check`/`python3 -m py_compile` (sintaxe válida nos dois) e
depois **executei** as funções: o trecho JS chamou `spawnSync('echo hi_from_shell_true', {["shell"]:
true, ...})` e o processo saiu com `exit 0` e a string interpretada pelo shell do SO (prova de que
`{["shell"]: true}` ativa o mesmo comportamento de `{shell: true}` em runtime, não é inerte); o
trecho Python chamou `subprocess.run(cmd, **{"shell": True}, ...)` com o mesmo resultado. Ambos
tratam `nosuchtool-xyz` corretamente como `exit 127` via shell — comportamento indistinguível de
`shell: true`/`shell=True` legítimo, porque **é** `shell: true`/`shell=True`, só com a chave escrita
de forma sintaticamente diferente.

O que faz o bypass funcionar, em duas camadas independentes:

1. **`assert_no_code_match` (negativo)** procura literalmente `shell\s*:\s*true` / `shell\s*=\s*True`.
   `{["shell"]: true, ...}` (notação de colchete/propriedade computada, JS válido) e
   `**{"shell": True}` (desempacotamento de kwargs, Python válido) são **funcionalmente idênticos**
   a `shell: true`/`shell=True` em runtime, mas não casam a string literal — nenhuma linha precisa
   ser "escondida atrás de comentário" para isso, é sintaxe de código legítima e comum.
2. **`assert_count` (positivo)**, que exige a string `spawnSync('sh', ['-c', command], {` /
   `["sh", "-c", cmd],` presente exatamente 1 vez, **não exclui comentários** — colocar a
   assinatura antiga (agora morta) num comentário `// spawnSync('sh', ['-c', command], {` /
   `# ["sh", "-c", cmd],` satisfaz o `grep -qF`/`grep -cF` sem que o código realmente a execute.

As duas camadas em conjunto (positivo satisfeito por comentário + negativo evadido por sintaxe
alternativa) produzem um gate verde sobre uma regressão real, plena, ao shell do SO — reintroduzindo
exatamente o defeito original do ADR (idiomas POSIX interpretados por `cmd.exe`/`shell:true` fixo em
`/bin/sh`) sem qualquer sinal.

**Severidade: BLOQUEANTE quanto à alegação de cobertura.** Não é um defeito de segurança explorável
por um atacante externo em runtime hoje (não há nenhuma entrada de usuário que force esse padrão
de código a existir) — é um **defeito do gate de defesa em profundidade**: a REQ e a ML-2A tratam
este gate como a barreira que impede reversão silenciosa da correção, e ele não impede uma
reversão escrita com sintaxe levemente distinta, seja por erro humano de refatoração futura, seja
por um agente de IA comprometido/induzido reescrevendo o ponto de execução do gate (o mesmo
vetor de "agente induzido com escrita irrestrita" já nomeado em
`ADR-2026-08-12-nao-ha-prevencao-contra-agente-induzido-com-escrita-irrestrita-a-resposta-e-deteccao-ancorada-no-git.md`).
A anotação `<!-- trackfw-contract: gate=scripts/check-shell-posix-portability.sh -->` (sem
`partial=`) em `docs/cli-parity.md:1951` está **factualmente incorreta**: o gate cobre reversão
literal/ingênua, não reversão funcional por sintaxe equivalente. Deve virar `partial=` com o
achado nomeado, ou o gate precisa evoluir para casar por AST/parse em vez de regex — decisão que
cabe ao dono do código (`hefesto-tf`/`artemis-tf`), não a mim.

**Remédio sugerido (não implementado por mim — fora de escopo desta barreira):**
- Curto prazo: anotar `partial=` no contrato, nomeando o gap de sintaxe-equivalente como risco
  residual aceito (mesma prática já usada em dezenas de outras entradas de `cli-parity.md`).
- Médio prazo: trocar o `assert_no_code_match` por uma checagem que não dependa só de regex sobre
  texto — por exemplo, executar `evalGates`/`_check_gates` num cenário de teste onde `sh` está
  ausente do `$PATH` curado E TAMBÉM instrumentar/mockar `child_process.spawnSync`/
  `subprocess.run` para provar que **nenhuma chamada** com `shell: true`/`shell=True` (de qualquer
  forma sintática) ocorre — comportamento observado, não texto casado.

## 5. `evalGateCommands` — extração preserva semântica do ramo de trust?

**Sim, confirmado por leitura completa do fluxo pós-extração** (`internal/commands/barrier.go:930-969`).
Antes da extração, a variável `gatesOK` só controlava `gatesCheck.Status` **dentro** de cada ramo
(`trustLocalGates` e trust-fail-open) — nunca era lida fora desses dois blocos. O agregado
`overallStatus` (linha 937-946) sempre foi calculado de forma independente, iterando
`checks := []barrierCheck{mlsCheck, accCheck, gatesCheck, validateCheck}` e marcando `blocked`
sempre que `c.Status != "passed"` — o que já cobria `not_evaluated` antes e depois da extração
(`not_evaluated != "passed"` → bloqueia). A extração removeu a variável morta `gatesOK` e
centralizou a lógica de loop+status em `evalGateCommands`, chamada identicamente nos dois ramos
(`trustLocalGates` e trust-aprovado). **Nenhuma mudança de comportamento observável** — verificado
também pelos testes `TestBarrierTrustLocalGatesFlag` (permanece verde) e pela suíte completa
`go test ./internal/commands/...` (ver §7). A REQ aberta de fail-open do `roadmapTrustForGates`
**não piorou** aqui: o ramo de trust continua decidindo `trusted`/`not trusted` exatamente como
antes; a extração só afeta o que acontece **depois** de já ter decidido evaluar.

## 6. Composição e risco residual

**Cadeia de composição, do mais para o menos provável:**

1. **`roadmapTrustForGates` fail-open** (nota do vault
   `barrier-trust-check-fail-open-em-tmpdir-simbolico-2026-08-29`, NÃO CORRIGIDO) + **`$PATH`
   resolvido em vez de fixo** (este PR) = um ambiente com `$TMPDIR` simbólico (macOS) *e* um
   `$PATH` adulterado agora tem **dois** graus de liberdade simultâneos: gates de roadmap não
   confiável são avaliados mesmo assim (fail-open), *e* o interpretador que os avalia pode ser
   trocado por quem controla `$PATH` — nos 3 CLIs, não só no Go como antes. Este é exatamente o
   ponto que a Wave 0 já nomeou ("ambiente onde tanto fail-open de trust quanto `sh` adulterado por
   `$PATH` se alinham não tem mais mitigação acidental de sintaxe no meio") — concordo com a
   análise, e a REQ do fail-open do trust check continua sendo o ponto certo para fechar essa
   composição, não este PR.
2. **Achado 4 (§4 acima) enfraquece a defesa em profundidade que detectaria uma reversão
   silenciosa** de volta a `shell:true` feita por um agente comprometido reescrevendo
   `barrier.js`/`barrier.py` — se isso acontecer via sintaxe equivalente, nem o `barrier` (que roda
   depois) nem o gate `make parity` acusam.
3. **`serve` injection** (`002-serve-permite-injecao-de-comando-ao-abrir-o-browser.md`, achado
   próprio nomeado na Wave 0, ML-0A finding 4.2): não tocado por este diff, `shell:true`
   legítimo e não relacionado ao ponto de execução de gate — corretamente fora de escopo,
   confirmado que `scripts/check-shell-posix-portability.sh` explicitamente não mira
   `serve.js`/`serve.py` (comentário linha 39-41 do script).

**Risco residual aceito, e a razão para eu não bloquear o merge inteiro por isso:** os dois vetores
de `$PATH` (§1) e de gate-bypass-por-sintaxe (§4) exigem, para serem explorados, controle sobre o
ambiente que já executa `barrier` com autoridade — ou sobre a própria árvore de código do CLI. Um
atacante nessa posição já tem alternativas mais diretas (editar `barrier.go` para sempre retornar
`passed`, por exemplo) que nenhum gate baseado em regex jamais vai fechar por completo. O ADR não
promete "impossível de reverter" — promete "não é um no-op silencioso, e a reversão ingênua/óbvia é
pega". Isso é verdade. O que não é verdade é a anotação de cobertura **plena** no contrato.

## Conclusão (repetida)

**Veredito: APROVA COM RESSALVAS.**

**Bloqueante:** `docs/cli-parity.md:1951` anota `gate=scripts/check-shell-posix-portability.sh`
(cobertura plena) quando o gate é, na verdade, contornável por regressão funcional real usando
sintaxe alternativa válida (`{["shell"]: true}` em JS, `**{"shell": True}` em Python) — PoC
executado, gate retorna exit 0 sobre uma árvore com shell do SO reintroduzido no ponto de execução
de gate. Corrigir a anotação para `partial=` nomeando o gap, ou endurecer o gate para checagem
comportamental (não regex), antes de tratar esta REQ como definitivamente fechada.

**Não-bloqueante (acompanhamento):**
- Node não tem teste `sh`-ausente ponta a ponta via subprocess do CLI, só em processo — Go e
  Python têm. Simetria de cobertura, não defeito funcional.
- `$PATH` como superfície de controle do interpretador de gate nos 3 CLIs (não só Go) — já
  declarado corretamente como residual pela própria Wave 0; concordo que documentar basta aqui, e
  que o ponto que dói é a composição com o fail-open de `roadmapTrustForGates` (REQ já aberta,
  não desta REQ).

Tudo mais auditado — extração de `evalGateCommands`/`evalGates`/`_check_gates`, distinção
`not_evaluated`×`blocked` nos 3 caminhos, mensagem pinada byte-idêntica, wiring em `make parity` —
está correto e verificado por execução real, não só leitura.

## Observação fora de escopo (não bloqueante)

O commit `159e91a` (junto com o bloco de aceite do ML-2A) também adiciona três arquivos novos —
`.agents/skills/source-command-trackfw-{architect,status,validate}/SKILL.md` — que não pertencem
ao escopo declarado da REQ (`internal/`, `npm/`, `pypi/`, scripts, docs de governança/segurança).
Li o conteúdo dos 3: são wrappers de prompt benignos, gerados (formato "Migrated source command"),
sem instrução injetada, sem segredo, sem referência a rede externa além do instalador oficial do
próprio projeto. Não é um achado de segurança — é uma observação de higiene de escopo de commit,
para quem audita o diff completo do PR não presumir que só arquivos de `barrier`/gate mudaram.

## `make quality` — não observado até o fim

`make quality` não completou dentro de 10 min nesta sessão (mesma observação independente feita
por `hefesto-tf` em paralelo, registrada em `docs/agents-working-context.md`). Não interpreto isso
como falha desta mudança: `go test ./internal/commands/...`, `node --test npm/tests/barrier.test.js`
e `python3 -m pytest pypi/tests/test_barrier.py pypi/tests/test_barrier_contract.py` rodaram
isoladamente e verdes (ver §2), `trackfw validate` não aponta nenhuma violação nova (só os 16
warnings pré-existentes, não relacionados a este diff), e `scripts/check-shell-posix-portability.sh`
roda em segundos contra a árvore real (`OK`). A suíte completa de `make quality` é conhecida por
ser longa neste repositório; não bloqueio por isso, mas registro que não vi o `make quality`
completo verde com meus próprios olhos nesta sessão.
