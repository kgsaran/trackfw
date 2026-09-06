# Dos 6 blocos sem cenário de falsificação, 5 fecharam; 1 (`gbg-cursor-relativo-present`) não tem lever independente do bloco 8 — masking medido ao vivo, não presumido

**Data:** 2026-09-06
**Onde:** `scripts/check-gates-falsify.sh` (Cenários 189/190/191), `internal/validator/validator_git_branch_guard.go`, `internal/validator/validator_credential_guard.go`
**Achado por:** artemis-tf (QA), ML-1F (ROADMAP-2026-09-06-fecha-o-fail-open-do-guard-config-ilegivel-deixa-de-ser-silencio)

## Contexto

Sequência: [[check-validate-parity-9-blocos-4-sem-cenario-de-falsificacao-2026-09-06]] (ML-1E)
enumerou 9 blocos cross-CLI em `check-validate-parity.sh`, dos quais 6 nomes/fixtures ficavam sem
NENHUM cenário provando reprova em regressão: GVP (`git_branch_guard_script_integrity` GLOBAL),
GVMT (`git_branch_guard_hook_resolvable` GLOBAL "missing type"), `gbg-claude-relativo`,
`gbg-cursor-relativo-present` (2 fixtures próprios do bloco 9), e `cg-claude-invalid-json`/
`cg-claude-unreadable`/`cg-claude-utf16` (3 fixtures compartilhados entre blocos 8 e 9, criados
ML-1A/1B desta mesma REQ). O usuário pediu: fechar TODOS, não só os 3 originais do ML-1C.

## O que fechou (Cenários 189, 190, 191 — as duas direções medidas)

- **189 (GVP):** `validator_git_branch_guard.go`, mensagem "content diverges from the template"
  (escopo GLOBAL, linha ~338) corrompida SÓ no texto (não no branch condicional). Reprova com
  `git_branch_guard_script_integrity GLOBAL-scope warning message text differs between runtimes`.
- **190 (GVMT):** mesma função-arquivo, mensagem "missing type" GLOBAL (linha ~252) corrompida só
  no texto. Reprova com o diagnóstico correspondente.
- **191 (bloco 9, `gbg-claude-relativo`):** `validator_credential_guard.go`,
  `validateGitBranchGuardHookResolvable()` (linha 560) — trocado `gitBranchGuardScriptMarker` por
  `credentialGuardScriptMarker` no `return`. `collectCommandsWithMarker` passa a procurar o
  marcador ERRADO; nenhum comando do fixture (que referencia
  `trackfw-git-branch-guard.sh`) casa, a regra fica muda SÓ para este wrapper.

As 3 mensagens compartilhadas (`cg-claude-invalid-json`/`unreadable`/`utf16`) já ficam cobertas
TRANSITIVAMENTE: são geradas por `validateGuardHookResolvable`, cujos braços de leitura/parse
disparam ANTES da filtragem por marcador — qualquer sabotagem realista nesses braços (ex.: Cenários
186-188, que já existiam) já quebra ambos os wrappers (credential_guard e git_branch_guard) ao mesmo
tempo, então não precisam de cenário PRÓPRIO no bloco 9 — o que faltava era só a metade
`gbg-claude-relativo`/`gbg-cursor-relativo-present`, específica da filtragem por marcador.

Cirurgia comprovada (não presumida) rodando `check-validate-parity.sh` inteiro sob cada `GO_BIN`
sabotado e contando linhas `... passed ...` antes da falha: 189 falha no bloco 2 (1 linha antes),
190 no bloco 6 (5 linhas antes: bloco1+GVP+186+187+188), 191 no bloco 9 (10 linhas antes: blocos
1 a 8 completos). Árvore íntegra (binário limpo, mesmo `check-validate-parity.sh`): `exit=0`,
todos os 9 blocos imprimem "passed".

## O que NÃO fechou — `gbg-cursor-relativo-present`, e por quê (medido, não deduzido)

A detecção "falso-positivo do Cursor" depende de UM booleano compartilhado
(`credentialGuardHookFile.requiresVarOrShellPrefix`, `validator_credential_guard.go`, tabela
`credentialGuardHookFiles`) usado IDENTICAMENTE pelos dois wrappers
(`validateCredentialGuardHookResolvable`/`validateGitBranchGuardHookResolvable`) — não há nenhum
branch condicionado por `scriptMarker` que trate o Cursor diferente para uma regra e não para a
outra. Toda sabotagem realista desse booleano (`false`→`true` na linha do Cursor) é, por
construção, marker-agnóstica.

Medido ao vivo (não presumido): sabotei a entrada `{".cursor/hooks.json", "Cursor", false, false}`
→ `..., true}` e rodei `check-validate-parity.sh` inteiro. Resultado: falha no **bloco 8**
(`cg-cursor-absent`, "mensagem inesperada — esperava 'but the script does not exist'... [bare
relative path]"), ANTES de alcançar o bloco 9 — os 7 blocos anteriores (1, GVP, 186, 187, 188,
GVMT, `branch_has_wip_roadmap`) passam limpos, e o script aborta ali (`set -euo pipefail` +
`run_cg`/`assert_fails_with` sobre o processo inteiro). `gbg-cursor-relativo-present` nunca é
alcançado — mascarado pelo bloco 8, que roda primeiro no arquivo.

Não existe hoje, no código de produção, nenhum ponto onde eu possa injetar uma sabotagem REALISTA
(i.e., que corresponda a uma classe de regressão plausível, não a lógica nova inventada só para
discriminar o teste) que quebre SÓ `gbg-cursor-relativo-present` sem também quebrar o bloco 8
primeiro. Escrever esse cenário exigiria ou (a) alterar `check-validate-parity.sh` para isolar o
bloco 9 (proibido pelo handoff) ou (b) inserir lógica nova condicionada por marcador que não existe
em produção (não seria falsificação de uma regressão real, seria uma prova fabricada — o "vácuo por
simetria" que o handoff pede para evitar). Decisão: não escrever cenário para este fixture; ele
permanece coberto INDIRETAMENTE (qualquer regressão real do booleano compartilhado já reprova o
gate, só que atribuída ao diagnóstico do bloco 8, não ao do bloco 9).

## Enumeração final dos 9 blocos (pós-ML-1F)

| # | Bloco | Cenário |
|---|---|---|
| 1 | Contrato bare ADR/REQ | Cenário 4 |
| 2 | GVP — `git_branch_guard_script_integrity` GLOBAL | **Cenário 189 (novo)** |
| 3 | SIU project | Cenário 186 |
| 4 | SIU global | Cenário 187 |
| 5 | FIFO | Cenário 188 |
| 6 | GVMT — `git_branch_guard_hook_resolvable` GLOBAL "missing type" | **Cenário 190 (novo)** |
| 7 | `branch_has_wip_roadmap` done/ | Cenário 79 |
| 8 | `credential_guard_hook_resolvable` (22 fixtures) | Coberto (inclui `cg-claude-invalid-json`/`unreadable`/`utf16` transitivamente via 186-188) |
| 9 | `git_branch_guard_hook_resolvable` (5 fixtures) | `gbg-claude-relativo`: **Cenário 191 (novo)**; `gbg-cursor-relativo-present`: **sem cenário — não falsificável independentemente, ver acima**; os 3 fixtures reaproveitados: cobertos transitivamente |

8 de 9 blocos têm cenário próprio; o 9º (bloco 9) tem cobertura parcial (1 de 2 fixtures únicos) e
cobertura transitiva para os 3 fixtures reaproveitados — a lacuna residual é INFORMAÇÃO, não
fracasso: é uma restrição estrutural do código de produção (booleano compartilhado por 2 regras),
não uma omissão do teste.

## Ver também

- [[check-validate-parity-9-blocos-4-sem-cenario-de-falsificacao-2026-09-06]] — enumeração original (ML-1E)
- [[credential-guard-marker-const-compartilhada-entre-3-consumidores-quebra-cenario-80-2026-09-06]] — achado irmão: mesma classe de acoplamento (código compartilhado entre credential_guard e git_branch_guard) já havia quebrado um cenário existente
- `docs/roadmaps/wip/ROADMAP-2026-09-06-fecha-o-fail-open-do-guard-config-ilegivel-deixa-de-ser-silencio.md` — ML-1F
