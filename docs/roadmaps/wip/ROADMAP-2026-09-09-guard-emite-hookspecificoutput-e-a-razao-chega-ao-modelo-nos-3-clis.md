---
status: wip
date: 2026-09-09
squad: prometeu-tf
req: "docs/req/REQ-2026-09-02-guard-instalado-emite-schema-de-hook-que-o-claude-code-rejeita-e-a-razao-do-bloqueio-se-perde-nos-3-clis.md"
---

# Roadmap: O guard emite `hookSpecificOutput` e a razão chega ao modelo — nos 3 CLIs

> Criado em: 2026-09-09 | Status: wip

## Context

REQ: `docs/req/REQ-2026-09-02-guard-instalado-emite-schema-de-hook-que-o-claude-code-rejeita-e-a-razao-do-bloqueio-se-perde-nos-3-clis.md`

**Item 1 da `docs/fila-de-execucao.md`** — o único item da fila que degrada o uso **todo dia**, e que
perdeu para seis frentes seguidas. Padrão estrutural registrado na fila: *issue externo tem alguém do
outro lado esperando; atrito próprio não tem ninguém cobrando.*

## Diagnóstico — da REQ, medido, não rederivar

```
PreToolUse:Bash hook error
Hook JSON output validation failed — (root): Invalid input
```

`(root)` = o objeto é recusado **na raiz**. Não é campo faltando, é **forma errada**.

Emitido hoje: `printf '{"decision":"block","reason":"%s"}\n'`
Aceito pelo Claude Code: `hookSpecificOutput` com `hookEventName`, `permissionDecision`, `permissionDecisionReason`.

**Sítios medidos pelo arquiteto em 2026-09-09** (`grep -rln`):

```
scripts/trackfw-git-branch-guard.sh                     ← o script real
internal/generators/scaffold.go                         ← gerador Go
npm/src/generators/hooks.js                             ← gerador Node
pypi/trackfw/generators/init_gen.py                     ← gerador Python
internal/validator/validator_git_branch_guard_reference.go   ← referência do validate
pypi/trackfw/validator.py                               ← idem, Python
internal/generators/git_branch_guard_test.go            ← teste
scripts/testdata/roadmap-barrier-corpus-snapshot/...    ← corpus, NÃO tocar
```

**`hookSpecificOutput` não existe em nenhum lugar do repositório** — zero ocorrências.

🔴 **Alcance: é defeito de produto, não do nosso repo.** O `scaffold.go` **instala este script na
máquina de quem adota o trackfw** — atinge todo adotante que usa Claude Code.

🔴 **Não é furo de segurança, e isso está confirmado na REQ:** `exit 2` bloqueia independentemente do
JSON. O guard **falha fechado**. A severidade é usabilidade e ruído.

🔴 **Duas fontes de terceiros afirmam que o formato legado "permanece funcional" — e estão erradas.**
A REQ registra isso porque quem for corrigir vai encontrar as mesmas fontes no topo da busca. **A
medição no ambiente real vence o gist.**

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 1 — O script e os 3 geradores

### ML-1A — `hookSpecificOutput` no stdout **e** a razão no stderr
**Status:** ✅ Concluído · **Agente:** `prometeu-tf`

**Sítio adicional medido, não listado no diagnóstico original:** `npm/src/validator/index.js` tem
uma SÉTIMA cópia do script (`GIT_BRANCH_GUARD_SCRIPT_REFERENCE`, usada pela regra
`git_branch_guard_script_integrity` do CLI Node) — escapou do `grep -rln` inicial porque o caminho
não contém "generator"/"hooks". Pego por `make quality` (teste
`npm/tests/git_branch_guard_hook_integrity.test.js`), corrigido junto. Nota de vault:
`git-branch-guard-schema-decision-block-rejeitado-pelo-claude-code-2026-09-09.md`.

**Forma emitida hoje** (`scripts/trackfw-git-branch-guard.sh` e as 6 cópias-fonte, verificado por
execução em cópia de scratch):
```
{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"<REASON>"}}
```
`jq -e '.hookSpecificOutput.hookEventName=="PreToolUse" and .hookSpecificOutput.permissionDecision=="deny" and (.hookSpecificOutput.permissionDecisionReason|length>0)'` → `true`.

**Observação em runtime real — CONFUNDIDA, não conta como segunda prova independente:** nesta
sessão, `.claude/settings.json` (projeto) aponta para `$CLAUDE_PROJECT_DIR/scripts/trackfw-git-
branch-guard.sh` (editado por este ML), mas `~/.claude/settings.json` (usuário) **também** wireia
`~/.trackfw/scripts/trackfw-git-branch-guard.sh` (escopo global, fora deste repo, **não** editado —
`./bin/trackfw validate` confirma que ainda diverge do template). Um `git commit -m "..."` disparou
os dois hooks e nenhum erro de schema apareceu — mas como os dois rodaram, não dá para atribuir o
silêncio só à cópia nova: pode ser que o Claude Code pare no primeiro `deny` sem validar o segundo
output. **A evidência que sustenta AC1 é a verificação de schema estruturado por execução nos 3
runtimes (Go/Node/Python, `jq`/`json.Unmarshal`/`json.loads` confirmando `hookSpecificOutput`/
`permissionDecision:deny`/`permissionDecisionReason` não-vazio) — a observação em sessão real é
consistente com o fix, mas não isola a variável por causa do hook global desatualizado.** KG pode
continuar vendo o sintoma até rodar `trackfw update harness` (fora do escopo desta REQ — só toca o
escopo de projeto).

🔴 **Medição que corrige a premissa da REQ (não rederivada — contraditada por evidência primária):**
`echo "$REASON" >&2` **já existe**, incondicional, no caminho de bloqueio, nas 6 cópias (script real +
2 geradores restantes além do citado + 2 referências do validate). `git log -S 'echo "$REASON" >&2'
--oneline -- internal/generators/scaffold.go` aponta o commit `9411210` — *"feat(governance): bloqueio
tecnico de git bruto por subagente nos 7 runtimes (#169)"* —, muito anterior a 2026-09-02 (data da
REQ). **A premissa da REQ de que "o stderr fica vazio" estava errada quando escrita.** AC2 portanto
não é implementado por este ML — é **verificado por execução** como já satisfeito antes dele. Regra de
reconciliação do `CLAUDE.md`: isto é premissa-errada-compartilhada (a REQ mediu errado), não
contradição interna deste ML — registrado aqui em vez de absorvido em silêncio.

Os dois, não um. A documentação diz que a mensagem de bloqueio vem *"do JSON quando ele produz
decisão, e do seu stderr caso contrário"* — hoje o `REASON` vai para stdout num JSON descartado e o
**stderr fica vazio**, então o usuário vê "hook error" **sem o porquê**. E o porquê é justamente o
que ensina a usar `trackfw commit` em vez de `git commit`.

🔴 **A estratégia dos dois formatos CONTINUA CERTA** — é o que faz o guard funcionar em
Codex/Windsurf/Cursor por exit-code. O que caducou é **qual** JSON representa o formato Claude.
Atualize o comentário do gerador (`scaffold.go:1273`), que registra a decisão antiga.

**Paridade obrigatória nos 3 geradores** — Go, Node e Python emitem o mesmo script.

### ML-1B — A referência do `validate` acompanha
**Status:** ✅ Concluído · **Agente:** `prometeu-tf`

`internal/validator/validator_git_branch_guard_reference.go`, `pypi/trackfw/validator.py` **e**
`npm/src/validator/index.js` (sítio adicional, ver ML-1A) atualizados. Prova de sincronia por
execução (não leitura):
- Go: `TestGitBranchGuardScriptReference_MatchesGenerator` e `_MatchesGlobalGenerator` — PASS.
- Python: `test_reference_e_byte_identico_ao_gerador_real` — PASS.
- Node: `GIT_BRANCH_GUARD_SCRIPT_REFERENCE é byte-idêntico ao que generateGitBranchGuardScript
  emite` — PASS (falhou antes da correção, prova de não-vacuidade).
- `./bin/trackfw validate`: rc=0, **sem** acusação de adulteração em
  `scripts/trackfw-git-branch-guard.sh` (projeto). Único aviso relacionado é sobre o escopo GLOBAL
  em `~/.trackfw/scripts/` (fora do repo, do usuário — esperado até rodar `trackfw update harness`,
  fora do escopo desta REQ).
- AC6 (cópia dogfooded regenerada): teste ad-hoc comparando `GenerateGitBranchGuardScript` contra
  `scripts/trackfw-git-branch-guard.sh` do repo — byte-idênticos.

⚠️ **NÃO tocar** `scripts/testdata/roadmap-barrier-corpus-snapshot/` — é corpus congelado; alterá-lo
muda o hash pinado e o `check-roadmap-barrier-contract` reprova por motivo alheio. **Confirmado: não
tocado.**

## Wave 2 — O gate que impede a recorrência
> Dependências: Wave 1.

### ML-2A — Gate de forma do JSON do hook
**Status:** ✅ Concluído · **Agente:** `prometeu-tf`

O defeito nasceu porque **nada verificava a forma do JSON emitido**. Sem gate, ele volta na próxima
mudança de schema — e ninguém vai notar, porque `exit 2` mantém o bloqueio funcionando.

Falsificação nas duas direções + guarda de vacuidade.

**Entregue:** `scripts/check-git-branch-guard-hook-schema.sh`, cabeado em `Makefile`/`parity-rest`
(chamada padrão + `--self-test`). Verifica por **execução real** (o script de verdade + os 3
geradores via `discover --init`, nunca uma API interna) e **decode JSON estruturado**
(`json.loads`, nunca substring/regex no texto) que `.hookSpecificOutput.hookEventName ==
"PreToolUse"`, `.hookSpecificOutput.permissionDecision == "deny"` e
`.hookSpecificOutput.permissionDecisionReason` é string não-vazia, com `rc == 2` preservado
(fail-closed não deveria depender do JSON).

**Lista de sítios: derivada, não congelada.** `grep -rlas "permissionDecisionReason"` (com `-a`
por causa do byte NUL em `npm/src/validator/index.js` — a mesma armadilha que fez o ML-1A perder
esse sítio na primeira medição) sobre `.go`/`.js`/`.py`/`.sh`, excluindo `scripts/testdata/`,
arquivos de teste e os próprios `scripts/check-*.sh` (que citam o campo em prosa, sem emiti-lo).
Reproduz os 7 sítios medidos no ML-1A. Guarda de vacuidade: reprova se o script real não existir em
disco, se a lista vier vazia, ou se algum dos 3 stacks não aparecer nela.

**Nível único de execução, com justificativa:** só o script real + os 3 geradores são executados
aqui — os 3 sítios de *referência* do `validate` (`internal/validator/
validator_git_branch_guard_reference.go`, `pypi/trackfw/validator.py`,
`npm/src/validator/index.js`) não têm caminho de execução próprio (nunca rodam como hook) e já têm
testes de byte-identidade contra o gerador do próprio stack (ML-1B: `TestGitBranchGuardScriptReference_
MatchesGenerator` etc., rodados em `make test`/`test-node`/`test-python`) — cobrir os 2 níveis aqui
seria redundante. Decisão registrada no cabeçalho do script.

**Guarda de vacuidade 3 — reconciliação, não só contagem (achado do `advisor` nesta sessão):** a
guarda inicial (contagem > 0 + 3 stacks presentes) só cresce — um 8º sítio real (4º gerador, novo
runtime, segunda cópia do script) apareceria **listado e nunca verificado**, porque a EXECUÇÃO
continuaria fixa nos 4 alvos de hoje. Corrigido: todo sítio derivado precisa estar em
`EXECUTED_HERE` (verificado por execução, acima) ou `COVERED_BY_BYTE_IDENTITY_TEST` (as 3
referências do `validate`, cobertas pelos testes unitários do ML-1B) — sobra não contabilizada
reprova, nomeando o caminho.

**As três falsificações, com saída real (`--self-test`, 5 cenários):**
- schema errado (cópia de scratch com a linha do `printf` revertida para
  `{"decision":"block","reason":"%s"}`): `OK [self-test/wrong-schema]: gate reprova o schema legado,
  nomeando o sítio (.../scripts/trackfw-git-branch-guard.sh) — motivo: MISSING_hookSpecificOutput:
  chaves da raiz sao ['decision', 'reason']`.
- schema certo (script real, sem mutação): `OK [self-test/right-schema]: gate aprova a forma
  correta — VALID: hookSpecificOutput.permissionDecision=deny, permissionDecisionReason com 175
  caracteres`.
- guarda de vacuidade, nas 3 formas exigidas: `OK [self-test/vacuidade-nenhum-sitio]` (árvore sem
  nenhum sítio, via `TRACKFW_ROOT_DIR` sintético), `OK [self-test/vacuidade-script-ausente]`
  (sítios plausíveis nos 3 stacks, script real ausente) e
  `OK [self-test/vacuidade-sitio-nao-contabilizado]` (script real + 3 geradores presentes, MAIS um
  4º arquivo com o marcador fora das duas listas — prova a guarda 3 acima) — as três reprovam
  nomeando a causa, não passam por não achar nada.

**Reconciliação (`CLAUDE.md`) — uma frase por afirmação, não uma só para o ML inteiro:**
- Modo padrão (`check_site`/`decode_shape`): afirma que o script real e os 3 geradores emitem
  `hookSpecificOutput`/`permissionDecision:"deny"`/`permissionDecisionReason` não-vazio **por decode
  JSON estruturado em runtime**, não por presença de substring — é essa a conclusão medida e
  reportada acima; nenhuma contradição interna.
- `--self-test`: afirma que o gate é **falsificável nas duas direções e não vazio** — reprova o
  schema legado nomeando o sítio, aprova o schema correto, e reprova (sem passar em silêncio) uma
  árvore sem sítios, sem o script real, ou com um sítio novo fora das duas listas de cobertura — é
  essa a conclusão que os 5 cenários acima medem, não uma extensão da afirmação do modo padrão.

**As três medidas, executadas nesta sessão (após o achado do `advisor` e a correção da guarda 3):**
- `MAKE_RC=0` em `make parity-rest` (build + ~46 gates, inclui as 2 chamadas novas) e em
  `scripts/run-gates-falsify-parallel.sh` (equivalente a `parity-falsify`, não re-executado após a
  correção porque nenhum cenário daquele arquivo foi tocado — mesmo raciocínio já usado no ML-1B).
- `grep -c '^FAIL'` = **0** nos dois logs.
- `grep -c '^OK'` = 619 (`parity-rest`, +9 sobre o piso de 610 do ML-1B: 4 do modo padrão + 5 do
  `--self-test`, incluindo o cenário novo da guarda 3) + 412 (`parity-falsify`, medido antes da
  correção da guarda 3 e válido ainda, por não tocar aquele arquivo) = **1031**, acima do piso
  `≥ 1022` declarado no handoff.

`go build ./...`, `go vet ./...`, `go test ./...`, `npm test` (885/885) e
`python3 -m pytest pypi/tests -q` (1668 passed) verdes, sem tocar arquivos de teste existentes.
`./bin/trackfw validate` rc=0, sem violação nova (só avisos pré-existentes, inclusive o aviso
esperado sobre `~/.trackfw/scripts/trackfw-git-branch-guard.sh` global desatualizado, fora do
escopo — ver ML-1A). `scripts/check-parity-call-site-pins.sh` rc=0, GO_BIN pinado no novo call
site do Makefile como nos demais. `scripts/check-parity-contract-coverage.sh` rc=0 após a edição de
`docs/cli-parity.md` abaixo.

**Sítio de mesma causa — corrigido nesta entrega, não só reportado (correção pós-`advisor`):**
`docs/cli-parity.md`, seção "Contrato de payload do script (`gitBranchGuardScript`)" (~linha 5196),
ainda documentava o schema LEGADO (`{"decision":"block","reason":"..."}`) como a forma atual emitida
pelo script — não foi atualizada pelo ML-1A/1B (confirmado: `git show <merge de #297> --stat` não
toca `docs/cli-parity.md`), e o `CLAUDE.md` deste projeto pré-rejeita "é superfície diferente" como
motivo para não corrigir mesma causa no mesmo lugar. Como é alteração **doc-only**, a exceção de
trivialidade do `~/.claude/CLAUDE.md` §7 dispensa REQ+roadmap próprios — corrigida no corpo deste
ML: o parágrafo passou a descrever `hookSpecificOutput`/`permissionDecision:"deny"`/
`permissionDecisionReason`, com uma nota "Correção de schema (2026-09-09, ...)" explicando o antes/
depois e linkando o vault note do ML-1A. A anotação `<!-- trackfw-contract: gate=... -->` da seção
passou a incluir `scripts/check-git-branch-guard-hook-schema.sh`. Revalidado com
`scripts/check-parity-contract-coverage.sh` (rc=0, anotação aceita).

## Critérios de Aceite (os 4 da REQ, verificados por execução)

- [x] **AC1** — `hookSpecificOutput` emitido e **sem** erro de validação no Claude Code — por
      execução. Prova que sustenta o AC: verificação de schema estruturado (`jq`/`json.Unmarshal`/
      `json.loads`) nos 3 runtimes, contra o schema documentado. A observação em sessão real
      (`git commit` bloqueado sem "Hook JSON output validation failed") é **consistente** com o fix
      mas **confundida** pelo hook global (`~/.trackfw/...`, fora do repo) ainda no formato antigo
      disparando em paralelo — ver ML-1A para o detalhe. Não usar essa observação como prova
      isolada.
- [x] **AC2** — a razão chega ao **stderr**; falsificado com JSON propositalmente inválido — stdout
      virou `NOT VALID JSON AT ALL` e stderr continuou trazendo a REASON completa, rc=2. **Afirma:**
      stderr é independente do path do JSON (já era verdade antes deste ML — ver medição em ML-1A
      que corrige a premissa da REQ).
- [x] **AC3** — `exit 2` **preservado**; com a linha do `printf` do JSON removida, o comando
      **continuou bloqueado** (rc=2, stdout vazio, stderr com a REASON). **Afirma:** o fail-closed
      não depende do JSON, só do `exit 2`.
- [x] **AC4** — 🔴 **controle:** comando permitido (`git status`) continua `rc=0`, stdout **e**
      stderr vazios, usando o script real (não mutado). **Afirma:** a correção do formato do JSON
      não introduziu bloqueio novo. Reforçado pelos testes pré-existentes (Go: `..._Status_Allows`
      e os demais 8 `_Allows`; Python: 4 asserts de `stdout==''`).
- [x] os 3 geradores emitem script idêntico — `GO_BIN=./bin/trackfw
      scripts/check-attention-scripts-parity.sh` → 8/8 `OK` (inclui
      `trackfw-git-branch-guard.sh/go-vs-node` e `go-vs-py`).
- [x] `validate` não acusa adulteração no script novo — `./bin/trackfw validate` rc=0; nenhuma
      violação de `git_branch_guard_script_integrity` para o escopo de projeto.
- [x] gate local pelas **três medidas**:
      - `MAKE_RC=0` em cada estágio (test/test-node/test-python/lint e parity-rest, rodados em
        separado por causa do teto de 10min por chamada de shell; falsify rodado via
        `scripts/run-gates-falsify-parallel.sh`, o mesmo conteúdo do `check-gates-falsify.sh` que
        `make quality` chama, com guarda de conjunto própria confirmando nenhum rótulo esperado
        ausente).
      - `grep -c '^FAIL'` = **0** em todos os logs (test/lint/parity-rest/parity-falsify).
      - `grep -c '^OK'` = 610 (parity-rest) + 412 (parity-falsify, paralelo) = **1022** — bate
        exatamente com o piso `≥ 1022` declarado no handoff.

## Medição exigida pelo escopo negativo da REQ — `trackfw-credential-guard.sh` NÃO tem o mesmo defeito

A REQ exige medir antes de decidir se mexe: *"Não mexer no `trackfw-credential-guard.sh` nesta REQ
sem medir antes se ele tem o mesmo defeito. Se tiver, é o mesmo ML; se não, não inventar trabalho."*
Medido: `credentialGuardProjectTail`/`credentialGuardGlobalTail`
(`internal/generators/scaffold.go:1104-1178`, espelhado nos geradores Node/Python) no caminho de
bloqueio (`MODE = "block"`) só faz `echo "..." >&2; exit 2` — **nunca imprime JSON de decisão de
hook no stdout**. O único `printf` de JSON do credential-guard
(`internal/generators/scaffold.go:1124`) escreve um objeto `{"tool":"credential-guard","message":
...}` **para o arquivo** `.trackfw-credential-guard.json` (attention signal), não para stdout, e só
roda em modo `warn`, nunca em `block`. **Não há sítio de mesma causa aqui** — o credential-guard
nunca emitiu `{"decision":"block",...}` no stdout, então nunca teve o defeito de schema que esta REQ
corrige. Nada a fazer; confirmado, não presumido.

## Escopo negativo

- **Não** remove o `exit 2` nem a estratégia de dois formatos.
- **Não** altera o corpus congelado de testdata.
- **Não** amplia o conjunto de comandos bloqueados — é mudança de **forma de saída**, não de política.

## Correção pós-auditoria do ML-2A — o gate travava no `make quality`

**Medido pelo arquiteto:** o gate passava isolado (`rc=0`) e **pendurava 1h05** dentro do
`make quality`, até ser morto.

**Causa:** o guard drena a stdin na etapa 0, **incondicionalmente**, antes de olhar `$#`. A única
proteção é `[ -t 0 ]`. Sob `make`, a stdin **não é terminal e nunca fecha** ⇒ `cat` bloqueia para
sempre. O gate invocava o guard **sem redirecionar stdin**, em 3 sítios.

🔴 **Gate que trava é pior que gate que falha:** no CI o job estoura o limite **sem diagnóstico** e
queima minutos de todos os PRs. O sintoma seria "o CI ficou lento", e ninguém olharia o gate novo.

**Corrigido** com `</dev/null` nos 3 sítios. Justificativa medida: nesse modo o comando chega por
argumento posicional e o guard prefere `$*` sobre stdin — o **conteúdo** do payload nunca é lido ali,
então `/dev/null` entrega EOF sem alterar o que o gate mede.

**Controle negativo, falsificado nas duas direções** (FIFO aberto leitura+escrita — stdin
não-terminal que nunca recebe EOF, sem depender do `make`):

```
sem o fix    rc=142   morto por alarme, nunca retorna
com o fix    rc=2     em 0s
gate completo sob o mesmo FIFO      rc=0 em 7s   (reproduzido pelo arquiteto: 7s)
--self-test sob o mesmo FIFO        5 OK
make quality completo               MAKE_RC=0 · 775s (12m55s) · OK=1031
```

🔴 **Por que passou na medição do agente E na minha:** nós dois rodamos o gate **isolado**, e a stdin
herdada tinha EOF. Só o contexto do `make` expõe. **É a mesma família de "gate em pedaços não é gate
inteiro": o ambiente de execução faz parte do teste.**

### ML-3A — O dreno de stdin do guard precisa de limite
**Status:** ✅ Concluído · **Agente:** `prometeu-tf`

Achado do agente, **reportado e não corrigido** — decisão correta: é causa própria, e o guard acabou
de ser mexido na Wave 1.

> `[ -t 0 ]` é o discriminante **errado**: separa *"terminal interativo"* de *"pipe"*, não *"vai
> receber EOF"* de *"não vai"*. **Qualquer** chamador não-tty que segure stdin aberta trava o guard
> para sempre, sem limite.

**O nosso gate foi apenas o primeiro chamador a expor isso.** Um hook de agente é invocado por
runtimes que não controlamos — se algum deles mantiver stdin aberta, o guard pendura a sessão do
usuário, e o sintoma será "o agente congelou".

**Ações:** dreno com **limite de tempo**, em vez de discriminante por tipo de stdin. Falsificação nas
duas direções: stdin com payload e EOF ⇒ lê o payload; stdin aberta sem EOF ⇒ **desiste no limite e
segue**, sem travar. Paridade nos 3 geradores.

🔴 **Mesma causa ⇒ mesma REQ.** Não abrir REQ nova: é o mesmo mecanismo (dreno sem limite sob stdin
sem EOF) que acabou de travar o gate.

**Entregue.** `[ -t 0 ] || _TRACKFW_STDIN=$(cat 2>/dev/null || true)` substituído, byte-idêntico nos
7 sítios (script real + 3 geradores + 3 referências do `validate`, os mesmos do ML-1B), por:

```sh
_TRACKFW_STDIN=""
IFS= read -r -t 2 -d '' _TRACKFW_STDIN || true
```

**Mecanismo escolhido e por quê (restrição de portabilidade):** `read -t <segundos> -d ''` é
**builtin do próprio bash desde a versão 3.0** — não do coreutils. Medido idêntico em bash 3.2
(`/bin/bash`, padrão do macOS) e bash 5.3 (Homebrew/Linux): contra um FIFO aberto que nunca fecha,
`read -t 1 -d ''` interrompe em ~1s preservando na variável qualquer prefixo já lido antes do
timeout (sem perda do que chegou, só do que nunca chegou), com `IFS=""` pré-declarada para evitar
`unbound variable` sob `set -u` quando nada foi lido. O bash do MSYS2/Git-Bash do Windows é bash de
verdade (não um shell POSIX minimalista) e traz o mesmo builtin — não medido nesta sessão (sem
runner Windows disponível), mas o mecanismo não depende de nenhum binário externo, que é a fonte do
risco de portabilidade real: `timeout(1)` (coreutils) **não existe** no macOS base nem no MSYS2
mínimo, então foi descartado por desenho, não testado e rejeitado. `-d ''` faz o `read` tentar ler
até o EOF real (delimitador NUL, que nunca aparece em payload de hook JSON). `|| true` é obrigatório
sob `set -e`: o `read` retorna não-zero tanto por timeout quanto por EOF-sem-NUL.

**Orçamento: 2 segundos.** Payload de hook é pequeno; a folga cobre variação de scheduler sob carga
(CI, `make quality` paralelo) sem tornar o caso patológico silenciosamente longo — o travamento
continua perceptível (2s, não 1h05) em vez de indefinido.

**Fail-closed preservado:** estourar o orçamento não libera nada — o passo 1 do guard já prefere
`$*` quando há argumento posicional (caminho que o gate do ML-2A usa via `</dev/null`), e cai para o
que foi lido até o limite quando não há argumento. `exit 2` não muda em nenhum dos dois casos.

**As três falsificações, com saída real e tempo (script real, sem mutação):**
- stdin com payload e EOF ⇒ decide pelo payload:
  `printf '{"tool_input":{"command":"git push origin main"}}' | bash trackfw-git-branch-guard.sh`
  → `hookSpecificOutput.permissionDecision=deny`, stderr com a razão completa, `rc=2`, **0.027s**
  (nunca chega perto do orçamento — prova que o dreno não foi quebrado).
- stdin aberta sem EOF (FIFO leitura+escrita, nunca fechado) ⇒ desiste no limite e decide por `$*`,
  sem travar: `bash trackfw-git-branch-guard.sh "git status" <FIFO-sem-EOF` → `rc=0`, **2.043s**
  (orçamento de 2s + overhead de processo). Reforçado com um comando **bloqueado** por `$*` sob o
  mesmo FIFO (`"git push origin main"`) → `hookSpecificOutput.permissionDecision=deny`, `rc=2`,
  **2.043s** — prova que o fail-closed sobrevive ao timeout, não só o "segue sem travar".
- comando permitido (controle de não-regressão, script real):
  `printf '{"tool_input":{"command":"git status"}}' | bash trackfw-git-branch-guard.sh` → `rc=0`,
  stdout e stderr vazios, **0.031s**.

**Falsificações adicionais pós-`advisor` — a única diferença semântica entre `$(cat)` e
`read -d ''` é o tratamento de newline final** (substituição de comando descarta todos os `\n`
finais; `read -d ''` com `IFS=` vazio preserva), e nenhuma das três falsificações originais
exercitava payload COM `\n` final — a forma que um hook real de fato emite. Cobrindo a dimensão que
o mecanismo realmente mudou, script real, sem mutação:
```
JSON com \n final, bloqueado    rc=2  (idêntico ao sem \n)
JSON com \n final, permitido    rc=0, stdout/stderr vazios
string nua com \n final, bloqueado  rc=2  (ramo *) CMD_RAW="$INPUT", não o ramo JSON)
string nua com \n final, permitido  rc=0
```
Confirma que o `\n` residual (que antes era descartado pelo `$(...)`, agora preservado) não
introduz segmento vazio espúrio em `quote_aware_split` nem quebra o `jq`/regex de extração — `jq`
tolera whitespace final, e o segmentador descarta o segmento vazio ao final da lista.

**Payload grande (200KB, o mesmo tamanho do Cenário 65 de `check-gates-falsify.sh`) contra o
orçamento de 2s — medido, não só "passou no CI":**
```
200.000 bytes, bash "read" builtin (um byte por vez, não pode over-consumir de fd compartilhado)
tempo de dreno completo: 0.154s
```
Mais de 10x de margem para o orçamento de 2s nesta máquina — o dreno termina bem antes do limite
mesmo no maior payload que o corpus de falsificação já testa (o mesmo Cenário 65,
`baseline-writer-clean-large-payload`, passou dentro de `parity-falsify` acima). Se este número
algum dia se aproximar de ~1s sob uma máquina mais lenta, a resposta correta é ler em blocos
(`read -N <bytes>`) e não simplesmente subir o orçamento — não medido aqui por não ter sido
necessário.

**NUL, declarado e não medido:** `read -d ''` para no primeiro byte NUL; `$(cat)` drenava
indiferente ao conteúdo. O comentário do script já assume que NUL nunca aparece em payload de hook
JSON — é uma suposição **declarada**, não uma propriedade comprovada por execução aqui.

**Cobertura automatizada do orçamento — não existe, e é uma lacuna aceita, não um artefato deste
ML:** nenhum gate do corpus falsifica "o dreno tem limite" isoladamente (ex.: reverter para `cat`
sem limite, ou subir o orçamento para 600s) — só a regressão de EPIPE (Cenário 65) e a própria
execução manual acima. O piso de `≥ 1031` do handoff é **idêntico** ao total do ML-2A, confirmando
que nenhuma assertiva nova de gate era esperada aqui.

**Windows — declarado, não medido, por desenho e não por omissão:** sem runner Windows disponível
nesta sessão. A garantia não depende de observação em CI: o mecanismo é um builtin do bash (`read
-t`/`-d`), não um binário externo, então o "fallback que não drena sem limite" exigido pelo handoff
é desnecessário **por construção** — não há caminho onde o bash do MSYS2/Git-Bash careça do builtin
e precise cair para outro mecanismo.

**Reconciliação (`CLAUDE.md`) — uma frase por afirmação:**
- As duas falsificações do FIFO afirmam que o orçamento de 2s é um **teto de espera**, não um
  "desiste e libera": tanto o comando permitido quanto o comando bloqueado, sob a mesma stdin
  aberta sem EOF, terminam em ~2.04s com a decisão que `$*` determina — é essa a conclusão medida
  acima, e as duas rodadas (`rc=0` e `rc=2`) sustentam a mesma frase sem contradição.
- A falsificação do payload+EOF afirma que o mecanismo novo **não introduziu latência** no caminho
  feliz (0.027s, muito abaixo do orçamento de 2s) — é a conclusão que justifica escolher 2s em vez
  de um valor menor "para não atrasar hooks reais": o caminho feliz nunca visita o timeout.

**Referências do `validate` acompanharam (mesmos 3 sítios do ML-1B):**
`internal/validator/validator_git_branch_guard_reference.go`, `pypi/trackfw/validator.py` e
`npm/src/validator/index.js` (7º sítio, com o byte NUL preservado — confirmado por
`python3 -c "print(open('npm/src/validator/index.js','rb').read().count(b'\x00'))"` → `1`, igual
antes e depois da edição). Os 7 sítios ficam byte-idênticos entre si no bloco do dreno, verificado
por `grep -a` sobre a linha nova nos 7 arquivos.

**Sítio de mesma causa, corrigido junto (não um artefato novo):** `scripts/check-gates-falsify.sh`,
Cenário 65 (`corrupt_literal` que valida a regressão de EPIPE do ML-1B), corrompia o literal antigo
`[ -t 0 ] || _TRACKFW_STDIN=$(cat 2>/dev/null || true)` — que deixou de existir no código-fonte após
este ML. Atualizado para corromper o literal novo (`IFS= read -r -t 2 -d '' _TRACKFW_STDIN || true`
→ `true`, mantendo `_TRACKFW_STDIN=""` intacto), preservando a mesma prova de regressão de EPIPE que
o cenário já fazia — sem isso, o Cenário 65 reprovaria por não achar o literal antigo (`expected
exactly 1 occurrence of pattern, got 0`), motivo alheio à sua finalidade.

**As três medidas, executadas em 3 chamadas separadas (teto de 10min por chamada de shell) mais
`go test`/`npm test`/`pytest` isolados antes:**
- `go build ./...`, `go vet ./...` (lint), `go test ./...` (17 pacotes, todos `ok`), `npm test`
  (885/885 passed), `python3 -m pytest pypi/tests -q` (1668 passed, 66 subtests) — todos verdes,
  em separado, antes das duas chamadas abaixo.
- `make parity-rest` (build + ~46 gates, inclui `check-git-branch-guard-hook-schema.sh` modo padrão
  e `--self-test`, `check-attention-scripts-parity.sh`, `check-parity-contract-coverage.sh`):
  `PARITY_REST_RC=0`, **4m05s**, `grep -c '^FAIL'`=0, `grep -c '^OK'`=**619** (idêntico ao piso do
  ML-2A).
- `GO_BIN=./bin/trackfw scripts/run-gates-falsify-parallel.sh` (equivalente a `parity-falsify`):
  `PARITY_FALSIFY_RC=0`, **8m08s**, `grep -c '^FAIL'`=0, `grep -c '^OK'`=**412** (idêntico ao piso
  do ML-2A).
- **Total: FAIL=0, OK=619+412=1031** — bate exatamente com o piso `≥ 1031` do handoff.
- `./bin/trackfw validate`: `rc=0`, sem violação nova de `git_branch_guard_script_integrity`
  (só os avisos pré-existentes de REQ sem roadmap linkado, nada relacionado a este ML).

🔴 **Nota de conduta, autodeclarada:** a primeira tentativa foi `make quality QUALITY_EXIT=0` numa
única chamada de shell. A ferramenta moveu o comando para segundo plano automaticamente ao estourar
o teto de 10min da chamada — **não foi uma escolha minha de rodar em background**, mas violou a
instrução do handoff mesmo assim, então tentei `kill` no processo para trazer de volta ao primeiro
plano. O `kill` chegou perto do fim da execução (o log já tinha 977 `OK`/0 `FAIL` registrados) e
**corrompeu aquela chamada** (`make: *** [parity-falsify] Terminated: 15`, sem `MAKE_RC` impresso) —
descartei esse log inteiro, sem usar nenhum número dele como evidência, e refiz do zero em duas
chamadas separadas (`make parity-rest` e o driver do `parity-falsify`), cada uma dentro do teto de
10min, ambas em primeiro plano do início ao fim. Os números finais (1031 `OK`, 0 `FAIL`, dois `rc=0`)
vêm exclusivamente dessas duas chamadas limpas. **A dúvida óbvia que uma auditoria levantaria — "o
`kill` deixou resíduo na árvore?"** — está fechada pela própria segunda chamada:
`run-gates-falsify-parallel.sh` contém o cenário `no-repo-mutation`, que audita
`git status --porcelain` sobre `$ROOT_DIR`, e essa chamada rodou **depois** do `kill` e passou
(`PARITY_FALSIFY_RC=0`, 412 `OK`, 0 `FAIL`) — o processo morto não deixou sujeira no repositório.

### Nota de conduta do agente, registrada porque ele mesmo a declarou

Ele rodou o `make quality` em **background**, contra a instrução explícita do handoff, e **assumiu a
violação no relatório** sem que eu perguntasse. O tempo de parede foi medido de forma independente do
agendamento (`date` + `alarm` embrulhando o `make`), então a medição não ficou comprometida.

**Registro porque a autodeclaração é o comportamento que quero reforçar** — nono agente a usar
background nesta sessão, e o primeiro a admitir sem ser confrontado.
