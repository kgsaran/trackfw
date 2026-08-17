# git-branch-guard: segmentação quote-aware resolve o falso-positivo de linha de mensagem, sem abrir evasão

> 2026-08-16 — Apolo (ML-1A, `ROADMAP-2026-08-16-higiene-sete-debitos-acumulados-da-entrega-de-plugins-e-da-release-7-0-0.md`)
> Resolve a limitação residual documentada em
> [[git-branch-guard-self-blocking-quote-unaware-splitter-2026-08-14]] ("uma linha dentro de um
> heredoc multi-linha que começa com o token `git` ainda bloqueia") e a causa raiz descrita em
> `vault/notes/git-branch-guard-falso-positivo-em-linha-de-mensagem-de-commit-2026-08-16.md`.

## Problema

`match_subcommand()` segmentava o comando bruto por `;`/`&&`/`||`/`|`/quebra-de-linha **sem
nenhuma noção de aspas de shell**. Uma mensagem de commit multi-linha (via
`-m "$(cat <<'EOF' ... EOF)"`, a própria convenção deste CLAUDE.md) cuja PRIMEIRA linha do
corpo começa com `git checkout -b` virava, depois da quebra de linha real, um pseudo-segmento
cujo primeiro token era literalmente `git` — bloqueando um `trackfw commit` legítimo.

## Solução: segmentação quote-aware, não uma lista de flags a ignorar

A tentação óbvia seria "descartar o conteúdo de `-m`/`--message`" (sugestão original do vault
note do incidente) — mas isso exige reconhecer o nome da flag, e não cobre `-F`/`--file`, nem
generaliza para heredocs soltos. A solução adotada foi generalizar o INVARIANTE: **nunca tratar
`;`/`&&`/`||`/`|`/quebra-de-linha como separador de comando enquanto dentro de uma string entre
aspas simples ou duplas** — independente de qual flag introduziu a string. Implementada como um
scanner char-a-char em `awk` (`quote_aware_split`, ver `scripts/trackfw-git-branch-guard.sh`),
mais um pré-passo line-mode (`strip_heredoc_bodies`) para heredocs SEM aspas ao redor (ex.:
`git commit -F- <<'EOF' ... EOF`).

## Por que a versão mais permissiva não abriu evasão (a pergunta que importa)

O risco explícito do ML era: "esconder um comando dentro de algo que passe por `-m`". Duas
proteções, testadas em `internal/generators/git_branch_guard_test.go`:

1. **Aspas não fechadas nunca "vazam" o texto seguinte como comando novo** — mesma semântica do
   shell real: se a aspa não fecha até o fim da entrada, TUDO depois continua "dentro" da
   string (nunca vira segmento novo). Não há como um atacante escapar de dentro de uma aspa
   aberta sem fechá-la.
2. **Heredoc mal-formado cai para o texto ORIGINAL sem nenhuma alteração** — se
   `strip_heredoc_bodies` não encontra a linha terminadora, ele devolve o texto de entrada
   intocado (nunca "meio processado"), preservando a segmentação de linha real do resto do
   comando. Testado explicitamente: `git status <<'EOF'\nwhatever\nNOTEOF\ngit push origin main`
   continua bloqueando o `git push` (o heredoc nunca fechou, então nada foi escondido).
3. Separador REAL logo após uma aspa de fechamento continua sendo separador: `git commit -m
   "x"; git push` e `git commit -m "x" && git push` continuam bloqueando — só o que está
   *dentro* das aspas é neutralizado, nunca o que vem depois delas.

## Decisão de design: por que não `RS="\0"` no awk

`quote_aware_split` acumula o input multi-linha em uma variável (`s = (NR==1) ? $0 : s nl $0`)
em vez de usar o idiom `RS="\0"` (slurp de stdin inteiro num record só). Funciona igual, mas
evita depender de um caractere de record-separator não-padrão que varia sutilmente entre
awk/mawk/busybox awk — mais portátil entre os ambientes onde este script roda (macOS, Linux CI,
containers minimalistas).

## Decisão de design: por que `sprintf("%c", N)` em vez de literais de aspas no awk

O programa awk inteiro evita literais `'` e `"` (usa `sq = sprintf("%c", 39)`,
`dq = sprintf("%c", 34)`) — não por elegância, mas porque esse mesmo texto é embutido
VERBATIM em 6 lugares (`scripts/trackfw-git-branch-guard.sh`, `internal/generators/scaffold.go`
via raw string Go, `npm/src/generators/hooks.js` via template literal, `pypi/trackfw/
generators/init_gen.py` via raw string Python, e as cópias `_REFERENCE`/`_SCRIPT_REFERENCE`
usadas pelo validator em `internal/validator/validator_git_branch_guard_reference.go`,
`npm/src/validator/index.js` e `pypi/trackfw/validator.py`). Evitar aspas literais dentro do
programa awk elimina a interação mais perigosa de escaping (aspa dentro de string
single-quoted do bash dentro de string entre backticks do Go/JS) — só backslashes (`\t`, `\n`)
e backticks (nos comentários) precisam de tratamento por stack, e isso é mecânico/diffável.

## Achado durante a implementação: 6 cópias do template, não 4

O roadmap listava 4 arquivos ("Arquivos afetados"), mas existem **6 cópias** do template do
guard usadas por `git_branch_guard_script_integrity`/`git_branch_guard_hook_resolvable`:
- `scripts/trackfw-git-branch-guard.sh` (referência canônica em disco)
- `internal/generators/scaffold.go::gitBranchGuardScript` (gerador Go)
- `internal/validator/validator_git_branch_guard_reference.go::gitBranchGuardScriptReference`
  (cópia do validator Go — existe por causa de um ciclo de import Go/validator não pode
  importar generators)
- `npm/src/generators/hooks.js::GIT_BRANCH_GUARD_SCRIPT` (gerador Node)
- `npm/src/validator/index.js::GIT_BRANCH_GUARD_SCRIPT_REFERENCE` (cópia do validator Node,
  usa `GBG_REF_BACKTICK` em vez de `GBG_BACKTICK` — nome de variável diferente do gerador,
  mesma técnica)
- `pypi/trackfw/generators/init_gen.py::_GIT_BRANCH_GUARD_SH` (gerador Python)
- `pypi/trackfw/validator.py::_GIT_BRANCH_GUARD_SCRIPT_REFERENCE` (cópia do validator Python)

**Esquecer qualquer uma delas faz `pypi/tests/test_git_branch_guard_validator.py::
test_reference_e_byte_identico_ao_gerador_real` (e o teste Node equivalente) falhar
silenciosamente até rodar o test suite completo** — `./bin/trackfw validate` sozinho só
detecta divergência entre o Go binário e o arquivo em disco, não entre as cópias
Node/Python-validator entre si. Antes de declarar "as 3 cópias sincronizadas" num ML futuro que
toque este guard, rodar os 3 test suites completos (`go test ./...`, `node tests/git_branch_guard_hook_integrity.test.js`, `python3 -m unittest tests.test_git_branch_guard_validator`), não só o diff manual dos 4 arquivos óbvios.

## Ver também
- [[git-branch-guard-self-blocking-quote-unaware-splitter-2026-08-14]]
- [[git-branch-guard-pipe-into-while-loses-return-status-2026-08-14]]
