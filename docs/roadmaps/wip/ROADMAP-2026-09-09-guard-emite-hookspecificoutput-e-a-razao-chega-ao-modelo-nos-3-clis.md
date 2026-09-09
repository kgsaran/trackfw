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
**Status:** ⬜ Pendente · **Agente:** `prometeu-tf`

O defeito nasceu porque **nada verificava a forma do JSON emitido**. Sem gate, ele volta na próxima
mudança de schema — e ninguém vai notar, porque `exit 2` mantém o bloqueio funcionando.

Falsificação nas duas direções + guarda de vacuidade.

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
