---
status: backlog
date: 2026-09-09
squad: prometeu-tf
req: "docs/req/REQ-2026-09-02-guard-instalado-emite-schema-de-hook-que-o-claude-code-rejeita-e-a-razao-do-bloqueio-se-perde-nos-3-clis.md"
---

# Roadmap: O guard emite `hookSpecificOutput` e a razão chega ao modelo — nos 3 CLIs

> Criado em: 2026-09-09 | Status: backlog

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
**Status:** ⬜ Pendente · **Agente:** `prometeu-tf`

Os dois, não um. A documentação diz que a mensagem de bloqueio vem *"do JSON quando ele produz
decisão, e do seu stderr caso contrário"* — hoje o `REASON` vai para stdout num JSON descartado e o
**stderr fica vazio**, então o usuário vê "hook error" **sem o porquê**. E o porquê é justamente o
que ensina a usar `trackfw commit` em vez de `git commit`.

🔴 **A estratégia dos dois formatos CONTINUA CERTA** — é o que faz o guard funcionar em
Codex/Windsurf/Cursor por exit-code. O que caducou é **qual** JSON representa o formato Claude.
Atualize o comentário do gerador (`scaffold.go:1273`), que registra a decisão antiga.

**Paridade obrigatória nos 3 geradores** — Go, Node e Python emitem o mesmo script.

### ML-1B — A referência do `validate` acompanha
**Status:** ⬜ Pendente · **Agente:** `prometeu-tf`

`internal/validator/validator_git_branch_guard_reference.go` e `pypi/trackfw/validator.py` contêm a
forma esperada do script. Se não acompanharem, o `validate` acusa adulteração num script correto —
e a regra de integridade de guard passa a mentir.

⚠️ **NÃO tocar** `scripts/testdata/roadmap-barrier-corpus-snapshot/` — é corpus congelado; alterá-lo
muda o hash pinado e o `check-roadmap-barrier-contract` reprova por motivo alheio.

## Wave 2 — O gate que impede a recorrência
> Dependências: Wave 1.

### ML-2A — Gate de forma do JSON do hook
**Status:** ⬜ Pendente · **Agente:** `prometeu-tf`

O defeito nasceu porque **nada verificava a forma do JSON emitido**. Sem gate, ele volta na próxima
mudança de schema — e ninguém vai notar, porque `exit 2` mantém o bloqueio funcionando.

Falsificação nas duas direções + guarda de vacuidade.

## Critérios de Aceite (os 4 da REQ, verificados por execução)

- [ ] **AC1** — `hookSpecificOutput` emitido e **sem** erro de validação no Claude Code — por execução
- [ ] **AC2** — a razão chega ao **stderr**; falsificado com JSON propositalmente inválido
- [ ] **AC3** — `exit 2` **preservado**; com o JSON removido, o comando **continua bloqueado**
- [ ] **AC4** — 🔴 **controle:** comando permitido continua `rc=0` e stdout vazio. *Guard ruidoso
      demais é desligado pelo usuário, e aí não guarda nada.*
- [ ] os 3 geradores emitem script idêntico (`check-cli-parity.sh` rc=0)
- [ ] `validate` não acusa adulteração no script novo
- [ ] gate local pelas **três medidas**: `MAKE_RC`, `grep -c '^FAIL'`, e `grep -c '^OK'` ≥ baseline

## Escopo negativo

- **Não** remove o `exit 2` nem a estratégia de dois formatos.
- **Não** altera o corpus congelado de testdata.
- **Não** amplia o conjunto de comandos bloqueados — é mudança de **forma de saída**, não de política.
