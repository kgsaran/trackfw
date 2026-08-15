# credential_guard_hook_resolvable não detectou script ausente em teste real

**Data:** 2026-08-15
**Onde:** `internal/validator/validator_credential_guard.go` (`validateCredentialGuardHookResolvable`)
**Achado por:** usuário (Kleber), durante discussão sobre `.gitignore` dos scripts gerados

## Sintoma

Removidos manualmente `scripts/trackfw-credential-guard.sh` e
`scripts/trackfw-git-branch-guard.sh` (simulando um clone fresco do repo sem
`trackfw update` rodado) com `.claude/settings.json` já registrando os hooks
apontando para esses caminhos. Rodado `trackfw validate` — **zero violação, zero
aviso** sobre os scripts ausentes, apesar de existir uma regra
(`credential_guard_hook_resolvable`) cujo propósito documentado é exatamente esse:
"para cada arquivo de hook de PROJETO que existir, extrai os comandos que referenciam
trackfw-credential-guard.sh, resolve o caminho e verifica que o script existe e é
executável."

## Por que isso importa

Isso não é hipotético: `scripts/trackfw-credential-guard.sh` nunca foi versionado
neste repo antes desta sessão (confirmado via `git log --all -- <path>` vazio) — ou
seja, todo clone fresco do próprio trackfw, sem rodar `trackfw update` primeiro,
tinha hooks de credential-guard registrados apontando para um arquivo inexistente,
e `trackfw validate`/CI (`trackfw-gate.yml`, que roda `trackfw validate` numa branch
recém-clonada) nunca acusou isso. Corrigido via commit `d7e88cf` (versionar os
scripts em vez de ignorá-los), mas a regra `credential_guard_hook_resolvable` em si
continua com esse coverage gap — se alguém deletar o script localmente de novo, ou
se o mesmo padrão se repetir para outro hook novo no futuro, `validate` não vai
pegar.

## Hipótese não investigada a fundo (próximo passo, se alguém for atrás)

`validateCredentialGuardHookResolvable` resolve o path do script a partir do
comando registrado no JSON do hook — que no caso real é
`$CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh` (variável de ambiente
literal na string, não expandida). Suspeita: a função pode estar tentando resolver
esse path literalmente (com o `$CLAUDE_PROJECT_DIR` ainda embutido como texto) em
vez de substituir pela raiz do projeto antes de checar `os.Stat` — o que faria a
checagem de existência sempre "falhar silenciosamente" por caminho malformado, não
por ausência real do arquivo (ver o comentário da função: "Arquivo de hook ausente
é pulado em silêncio" — mas aqui o arquivo de HOOK existe, `.claude/settings.json`
existe; é o SCRIPT referenciado dentro dele que está ausente, cenário diferente do
que esse comentário descreve).

## Ver também

- Commit `d7e88cf` (branch `feat/trackfw-ship-gera-corpo-de-pr-minimo`) — fix
  aplicado (versionar os scripts, não corrigir a regra).
- `internal/validator/validator_credential_guard.go` — função a investigar.
