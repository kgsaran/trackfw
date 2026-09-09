# O guard emitia um JSON válido, mas com o schema errado — e a premissa da REQ sobre o stderr estava desatualizada

> ROADMAP-2026-09-09-guard-emite-hookspecificoutput-e-a-razao-chega-ao-modelo-nos-3-clis.md, ML-1A.

## O sintoma

```
PreToolUse:Bash hook error
Hook JSON output validation failed — (root): Invalid input
```

`(root)` significa que o objeto inteiro é recusado — não é campo faltando, é forma errada.

## A causa

`trackfw-git-branch-guard.sh` (script real + 3 geradores + 2 referências de `validate` + 1 referência
Node de `validate` embutida em `npm/src/validator/index.js`) emitia, no caminho de bloqueio:

```bash
printf '{"decision":"block","reason":"%s"}\n' "$REASON"
```

O schema que o Claude Code aceita hoje para `PreToolUse` é diferente — a decisão vem aninhada:

```json
{
  "hookSpecificOutput": {
    "hookEventName": "PreToolUse",
    "permissionDecision": "deny",
    "permissionDecisionReason": "texto"
  }
}
```

`{"decision":"block","reason":"..."}` nunca validou contra esse schema. Corrigido para:

```bash
printf '{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"%s"}}\n' "$REASON"
```

## Duas fontes de terceiros afirmam que o formato legado "permanece funcional" — estão erradas

Uma consulta de apoio devolveu um gist e um blog afirmando que `{"decision":"approve"|"block","reason":"..."}`
continua válido. A documentação oficial (`code.claude.com/docs/en/hooks`) não sustenta isso, e a
evidência primária (o hook real sendo rejeitado, reproduzido nesta sessão com `git commit` bloqueado
sem erro de schema após o fix) o contradiz diretamente. **A medição no ambiente real vence o gist.**

## `exit 2` é o que garante fail-closed — independente do JSON

A documentação oficial: *"Exit 2 blocks whether or not you print JSON: even a JSON `permissionDecision`
of `"allow"` can't override it."* O defeito acima nunca foi um furo de segurança — o guard sempre
bloqueou via `exit 2`, com ou sem JSON válido. A severidade era usabilidade/ruído: o usuário via "hook
error" sem a razão explicando por que usar `trackfw commit` em vez de `git commit`.

## Achado que corrige a premissa da própria REQ (não uma contradição deste ML)

A REQ (2026-09-02) afirmava que "o `REASON` vai para o stdout... o stderr fica vazio". Medido nesta
sessão: `echo "$REASON" >&2` já existe, incondicional, no caminho de bloqueio, em TODAS as 6+1 cópias
(script real, 3 geradores, 2 referências de validate Go/Python, e a referência embutida no
`npm/src/validator/index.js`). `git log -S 'echo "$REASON" >&2' -- internal/generators/scaffold.go`
aponta o commit `9411210` ("feat(governance): bloqueio tecnico de git bruto por subagente nos 7
runtimes (#169)"), bem anterior à data da REQ. **A premissa estava errada quando a REQ foi escrita** —
não é um caso de contradição interna deste ML (regra de reconciliação do `CLAUDE.md`), é premissa
compartilhada errada. AC2 foi verificado por execução como já satisfeito, não implementado por este ML.

## Sítio que a busca por `"decision":"block"` quase não pegou

Além dos 6 sítios listados no roadmap, existe uma SÉTIMA cópia da referência do script para o
`validate` do CLI Node: a constante `GIT_BRANCH_GUARD_SCRIPT_REFERENCE` dentro de
`npm/src/validator/index.js` (não em `npm/tests/`, que só a IMPORTA). O teste
`npm/tests/git_branch_guard_hook_integrity.test.js` (`GIT_BRANCH_GUARD_SCRIPT_REFERENCE é
byte-idêntico ao que generateGitBranchGuardScript emite`) pegou a divergência imediatamente — mas só
depois de rodar `make quality` inteiro. Um `grep` estreito por `"decision":"block"` em
`npm/src/generators/` (onde o script é COMPOSTO) não alcança `npm/src/validator/` (onde ele é
DUPLICADO para a checagem de integridade) — mesmo padrão de risco já registrado pela regra dura de
paridade nos 3 CLIs deste `CLAUDE.md`: a duplicação da referência para `validate` existe em Go
(`internal/validator/validator_git_branch_guard_reference.go`), Python (`pypi/trackfw/validator.py`)
E Node (`npm/src/validator/index.js`) — mas só os dois primeiros têm nome de arquivo óbvio.

## Como não cair nisso de novo

Antes de declarar completo qualquer ML que edita `gitBranchGuardScript`/`_GIT_BRANCH_GUARD_SH`/
`GIT_BRANCH_GUARD_SCRIPT`, rodar `grep -rn '"decision":"block"' --include='*.go' --include='*.js'
--include='*.py' --include='*.sh' .` (excluindo `scripts/testdata/`) e conferir que a lista bate com
os 3 pares gerador+referência (Go, Node, Python) — não só os arquivos com "generator" ou "hooks" no
caminho.

## Ver também

- [reason-do-guard-diverge-por-escaping-de-aspas-entre-python-e-go-node-2026-08-22](reason-do-guard-diverge-por-escaping-de-aspas-entre-python-e-go-node-2026-08-22.md)
