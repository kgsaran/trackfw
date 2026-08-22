# REASON do guard diverge por escaping de aspas entre Python e Go/Node

> 2026-08-22 · domínio: geradores / git-branch-guard · REQ-2026-08-22

## Fato

A string de REASON do `git-branch-guard` é duplicada em **7 arquivos** (não 5, como mapeamentos
anteriores registravam):

```
scripts/trackfw-git-branch-guard.sh
internal/validator/validator_git_branch_guard_reference.go
internal/generators/scaffold.go
npm/src/generators/hooks.js
pypi/trackfw/generators/init_gen.py
npm/src/validator/index.js        <- costuma ficar fora do mapa
pypi/trackfw/validator.py         <- costuma ficar fora do mapa
```

Ao introduzir **aspas duplas** dentro da REASON (`trackfw ship -m "..."`), o gerador Python emitia
`\"...\"` enquanto Go e Node emitiam `"..."`. Resultado: `check-attention-scripts-parity.sh`
falhando no eixo **go-vs-py**, com os outros gates verdes.

Correção: alinhar todos ao padrão de Go/Node (`"..."`, sem escape de barra) — no `.sh` fonte, em
`init_gen.py` e em `validator.py`.

## Por que custa tempo

O sintoma aparece **longe da causa**: o gate que quebra compara os *scripts emitidos*, não o código
do gerador. Quem edita a REASON acha que sincronizou as cópias — e sincronizou — mas o **escaping**
por linguagem introduz a divergência na emissão.

Regra prática: ao mexer nessas strings, prefira não introduzir aspas duplas; se introduzir, rode
`check-attention-scripts-parity.sh` antes de qualquer outro gate — é o que discrimina.

## Os gates provam identidade, não correção

Os 4 gates de hooks provam que as 7 cópias são byte-idênticas. **Não** provam que o conselho da
mensagem está certo. Nesta REQ, uma substituição mecânica `trackfw ship` → `trackfw push` passou
por todos eles emitindo conselho errado: dizia para reempurrar via `trackfw push` depois de
`git reset --soft HEAD~1`, estado em que as mudanças estão *staged e não commitadas* — `push` não
commita, então não há o que empurrar. O correto ali é `trackfw ship -m`.

## Efeito colateral esperado ao editar o guard

A cópia global `~/.trackfw/scripts/trackfw-git-branch-guard.sh` passa a divergir do template até que
o usuário rode `trackfw update harness`. Regra `git_branch_guard_script_integrity`, severidade
**warning** — `validate` e `make quality` seguem exit 0.

**Consequência prática:** a mensagem que o usuário realmente vê continua sendo a **antiga**, porque
`~/.claude/settings.json` também wira a cópia global e ela bloqueia primeiro. Medido em 2026-08-22:
`git push` bruto pela ferramenta Bash devolveu o texto antigo mesmo com o script do projeto já
corrigido. O guard **não** falha aberto — só ensina o caminho velho até o `update harness`.
