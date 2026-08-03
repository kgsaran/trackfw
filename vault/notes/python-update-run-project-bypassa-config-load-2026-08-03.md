# `trackfw update` (Python) — variante `_run_project` nunca chama `config.load()`

> Data: 2026-08-03 | Autor: Hefesto | Domínio: config / update / falsificação

## Sintoma

`trackfw update --dry-run` (ou `--json`/`--targets`/`--install-missing`) com um `trackfw.yaml`
malformado **não falha** no CLI Python — devolve relatório normal, exit 0. Nos CLIs Go e Node.js,
a mesma invocação falha com a mensagem fatal padrão e exit 1.

```
$ trackfw update --dry-run        # Go/Node: exit 1, mensagem de YAML malformado
$ python -m trackfw update --dry-run   # Python: exit 0, relatório normal
```

A variante sem flags (`trackfw update`, dispatch para `_run`) **é** fatal nos 3 CLIs — só a
variante project-scope com flags (`_run_project`) diverge.

## Causa

`pypi/trackfw/commands/update.py` tem dois caminhos de dispatch (`_dispatch`, ML-2A):
- `_run(args)` — sem flags — chama `_load_update_config(cwd)` → `config.load()`, que é fatal em
  YAML malformado (mesmo contrato de `validate`/`status`).
- `_run_project(args)` — `--dry-run`/`--json`/`--targets`/`--install-missing` — **nunca** chama
  `config.load()`. Resolve `target_ids` de uma lista fixa (`_resolve_project_targets`) e nunca lê
  `hooks`/`ci`/`backend`/`frontend`/`pkg_manager`.

Isso é estrutural, não um bug do ML-2A: `docs/cli-parity.md` documenta que o Python tem uma lista
de targets **fixa e menor** (5 ids: agent-rules, agent-hooks, codex-project-agents,
validate-script, claude-commands) — nunca inclui `ci-workflow`/`git-hooks`, então nunca precisa
saber `cfg.CI`/`cfg.Hooks` para decidir o que incluir. Confirmado por `git show main:...` que
`_run_project` já não lia `trackfw.yaml` antes do ML-2A.

O Go equivalente (`UpdateProject`, `internal/generators/update.go`) **precisa** chamar
`loadUpdateConfig()` mesmo em `--dry-run` porque `ProjectTargetIDs(cfg)` decide dinamicamente se
inclui `ci-workflow` (`cfg.CI != ""`) e `git-hooks` (`cfg.Hooks == "husky"/"lefthook"`) — por isso
falha em YAML malformado nesse caminho, e o Python não.

## Por que importa para a Wave 3 (`scripts/check-gates-falsify.sh`)

O ML-3A deste roadmap pede um cenário de falsificação por CLI que prove "scanner artesanal
reintroduzido → detectado". Se o braço Python desse cenário invocar `trackfw update --dry-run`
ou `--json` (escolha natural para um script shell, porque não muta a árvore do projeto), o
cenário **nunca alcança `config.load()`** — passa idêntico com ou sem o scanner reintroduzido.
Cenário verde, mas morto (mesmo sintoma de
`vault/notes/cenarios-de-falsificacao-quebram-em-refactor-do-alvo-2026-08-02.md`, causa
diferente: aqui não é literal ausente, é caminho de código que nunca lê o arquivo).

**Regra para quem escrever o cenário Python de `update`:** invocar a variante sem flags
(`trackfw update`) ou `trackfw sync --to <provider>` — ambos chamam `config.load()`
incondicionalmente. Não usar `--dry-run`/`--json`/`--targets`/`--install-missing` como veículo de
detecção nesse CLI.

## AC6 — cobertura parcial, decisão de Zeus

A REQ deste roadmap (AC6) diz "`update` do Python lê e age sobre os 5 campos, como Go e Node".
Isso é verdade para `_run` e **falso** para `_run_project` (nunca lê os 5 campos). Se cobertura
parcial satisfaz a redação do AC6 é chamada de auditoria do Zeus, não achado de qualidade — só
registrado aqui para não se perder entre as sessões.

Relacionado: `vault/notes/cenarios-de-falsificacao-quebram-em-refactor-do-alvo-2026-08-02.md`.
