---
status: wip
date: 2026-11-05
req: "docs/req/REQ-2026-11-05-guards-do-primeiro-usuario-externo-mode-block-ignorado-com-crlf-padroes-de-credencial-e-involucro-de-shell-no-guard-de-git.md"
squad: "hades-tf, apolo-tf"
---

# Roadmap: Guards do primeiro usuário externo: mode block ignorado com CRLF, padrões de credencial e invólucro de shell no guard de git

> Created: 2026-11-05 | Status: wip

## Context
REQ: docs/req/REQ-2026-11-05-guards-do-primeiro-usuario-externo-mode-block-ignorado-com-crlf-padroes-de-credencial-e-involucro-de-shell-no-guard-de-git.md
ADR: docs/adr/ADR-2026-11-05-guards-cumprem-o-que-o-readme-promete-mode-lido-com-crlf-padroes-de-credencial-ampliados-e-involucro-de-shell-analisado.md
Issues: #662, #663

A varredura de issues abertos em 2026-11-05 achou #662, #663 e #665. O #665 (prazo do PowerShell 5.1 no
teste) e a Discussion #664 são de outra causa, a do instrumento de CI, e vão para a REQ de CI.

🔴 **Medição de guard:** alimentar os payloads por stdin a partir de um script (Write + `bash arquivo`). O
guard de git desta sessão bloqueia um comando inline que contenha o texto de um git que escreve. E o guard de
credencial bloqueia `grep` com glob que alcance arquivos de teste com JWT de exemplo.

## Acceptance Criteria
- [ ] AC1–AC6 da REQ

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model (hades-tf)
**Status:** ✅ Concluído — auditado em 2026-11-05. Parecer em `docs/seguranca/2026-11-05-wave0-guards-primeiro-usuario-externo.md`, com os achados 1 a 11 e os residuais R1 a R7.
- **Conferido pelo arquiteto:** o vazamento entre blocos dá `rc=2` no projeto, `\git push` e `g\it push` dão `rc=0`, e as constantes do `scaffold.go` só aparecem em comentário fora do próprio arquivo.
- **Emendas do arquiteto:**
  - o leitor compartilhado fica em `internal/guard`, exportado, e não num pacote novo (o validator importa `internal/guard` sem ciclo, medido com `go list -deps`);
  - entram na lista do ML-1A o `internal/commands/guard.go` (`help` com "JWT or AWS") e o `docs/cli-parity.md` (cita as constantes apagadas);
  - os achados 2, 3 e 7 a 11 entram pela Regra Dura de Causa Raiz, e o AC1 e o AC4 da REQ foram emendados.
**Files affected:** `docs/seguranca/2026-11-05-wave0-guards-primeiro-usuario-externo.md`
**Actions:**
1. Completude da enumeração, por causa:
   - todos os leitores de `credential_guard.mode`: guard de projeto, `--global`, validator, script gerado
     (`credentialGuardModeResolution`) e qualquer outro. Diga qual é o ativo na linha de hook atual e se o
     `.sh` gerado ainda é executado por algum hook ou fluxo;
   - todos os sítios com os padrões de credencial: `credJWTRe` e `credAWSRe`, as comparações
     `match == "JWT"`/`"AWS access key"` da segunda camada, `credentialGuardDetectionCore` e os testes de
     paridade do script;
   - todos os pontos em que o guard de git decide pelo primeiro token (`MatchSubcommand`,
     `stripEnvCommandPrefix`, `quoteAwareSplit`, `shellTokenize`) e se existe um segundo guard de git (script
     ou validator) que precise convergir;
   - os testes e documentos que afirmam o comportamento atual: README, `docs/cli-parity.md`, os comentários
     de "resíduo CRLF" e os pins.
2. Modelo de ameaça:
   - contornos do D4: aspas aninhadas, `bash -c` com `$'…'`, `-c` depois de outras flags, `--` e `bash -s`
     com stdin, `exec`, `xargs`, `nohup`, `time`, `timeout`, `env bash -c`, `sudo bash -c`, `cmd /c` no
     Windows e `powershell -EncodedCommand`. Para cada um, diga se entra, se fica como residual, e por quê;
   - o limite de profundidade: qual valor, e o que acontece acima dele;
   - falso positivo do D4: `bash -c "git status"`, mensagem de commit com o texto `bash -c`, `git log` com
     `--grep`;
   - os padrões do D2: proponha a expressão exata de cada um, com o comprimento mínimo, e meça o falso
     positivo contra este repositório. Para cada expressão, informe quantos casamentos ela tem em `docs/`,
     `internal/` e `vault/`;
   - o CRLF: há outras leituras linha a linha do `trackfw.yaml` no guard (`credReadRoadmapDir` e outras)
     com o mesmo defeito?
3. Alvos de falsificação nos dois sentidos, do AC1 ao AC5.
4. Residual declarado.
5. `## Prescrições`, com a lista de arquivos de cada ML. O ML-1A (credencial) e o ML-1B (git) não podem
   ter arquivo em comum, ou a Wave 0 diz que ficam em sequência.
**Acceptance criteria:**
- [x] As cinco seções respondidas com evidência (comando e resultado)
- [x] Prescrições em seção intitulada `Prescrições`
- [x] Nenhuma linha de implementação escrita

**Gates da wave:**
```bash
test -f docs/seguranca/2026-11-05-wave0-guards-primeiro-usuario-externo.md
grep -q '^## Prescrições' docs/seguranca/2026-11-05-wave0-guards-primeiro-usuario-externo.md
```

## Wave 1 — Implementação (sequencial: ML-1A, depois ML-1B)
> Dependencies: Wave 0 auditada. Ficam em sequência porque os dois escrevem no `README.md` e no
> `docs/cli-parity.md`, em seções diferentes, e porque os dois testam o pacote `internal/guard`.

### ML-1A — Guard de credencial: leitor de mode único, padrões ampliados, constantes mortas (apolo-tf) — AC1, AC2, AC3
**Status:** ✅ Concluído — auditado em 2026-11-05, em dois despachos.
- **AC1:** `guard.ReadCredentialGuardMode` é o leitor único, e o validator delega a ele. A tabela de `credential_mode_test.go` cobre CRLF, BOM, `# mode:`, `ci_mode:`, vazamento entre blocos, aninhado e tab. Desvio aceito: tab conta como indentação, o que preserva o resultado do validator. Medido no binário: com YAML em CRLF e `mode: block`, AWS, `ghp_`, `AIza` e `sk-proj-` dão `rc=2` e `print(1)` dá `rc=0`.
- **AC2:** 14 expressões para 12 tipos, com borda explícita nos `sk-`. **Na auditoria, a segunda camada buscava a expressão pelo nome**, e 5 padrões do GitHub compartilham o nome. A isenção de `echo … > /dev/null` falhava para `gho_`, `ghu_`, `ghs_` e `ghr_`, e o teste que deveria pegar isso era tautológico. O segundo despacho criou `credCmdHasTypeMatch` e `TestCredEphemeralExemption_AllPatterns`, que testa os 14 padrões nos dois braços. No contra-braço, os 4 subcasos esperados reprovam.
- **AC3:** README, `docs/cli-parity.md` e o `help` do `guard credential` listam os tipos e o que não é detectado. 249 linhas de constantes mortas foram apagadas.
- **`make parity-rest`:** passou com `rc=0` no PATH normal. O "pré-existente" do relatório era o bash 3.2 do PATH restrito (erro de prescrição do arquiteto, já registrado na memória).
- **Resíduo:** a variável `credLegacyPatternNames` ficou sem leitura e vai para o ML-1B.
**Files affected:** `internal/guard/credential.go`, `internal/guard/credential_test.go`, `internal/guard/credential_mode_test.go`, `internal/guard/credential_patterns_test.go`, `internal/validator/validator_credential_guard_integrity.go`, `internal/validator/validator_credential_guard_integrity_test.go`, `internal/generators/scaffold.go`, `internal/generators/scaffold_literal_copy_parity_test.go`, `internal/generators/credential_guard_test.go`, `internal/commands/guard.go`, `README.md`, `docs/cli-parity.md`
**Acceptance criteria:**
- [x] AC1, AC2 e AC3 da REQ, conforme o parecer da Wave 0 (§5.1)
- [x] pacotes tocados verdes com PATH sem trackfw; `make parity-rest` verde
- [x] contra-braço por item

### ML-1B — Guard de git: invólucro de shell, prefixos e barra invertida (apolo-tf) — AC4, AC5
**Status:** ✅ Concluído — auditado em 2026-11-05.
- **AC4:** medido no binário da branch com 47 vetores, por `--command`. Os 47 dão `rc=2`: os invólucros `bash`, `sh`, `zsh`, `dash`, `ksh`, `pwsh`, `powershell`, `cmd` e `eval` (com flags combinadas, `.exe`, maiúsculas e `--norc`/`-o pipefail`), dois níveis de aninhamento, o terceiro nível com `wrapper-not-analyzed`, `\git push`, `g\it push`, atribuição nua e os prefixos. No contra-braço, a 11.3.0 instalada deixa passar 44 dos 47.
- **Sem falso positivo:** 16 vetores de leitura dão `rc=0`, entre eles `bash -c "git status"`, `echo 'bash -c git push'`, `git log --grep 'bash -c push'` e `time make test`.
- **AC5 e residuais:** `python3 -c` com `subprocess`, `bash <<<`, `| sh`, `xargs`, `$X push` e `ionice` dão `rc=0`, como a documentação declara.
- **Desvio de desenho aceito:** em vez de mudar `quoteAwareSplit` e `shellTokenize`, o ML criou cópias POSIX (`quoteAwareSplitPOSIX`, `shellTokenizePOSIX`, `stripShellSuffixPOSIX`) e uma segunda análise que só roda quando a primeira não acha nada. Motivo: a primeira trata a barra invertida como literal por causa dos caminhos do Windows. O preço são dois tokenizadores para manter, e o ML-2A avalia isso.
- **Corrigido pelo arquiteto na documentação:** o README e o `docs/cli-parity.md` diziam que o guard "só bloqueia quando as duas análises concordam" e que "nada permitido hoje vira bloqueado". O código bloqueia quando qualquer uma acha uma escrita, e `bash -c "git push"` passa a ser bloqueado de propósito. Também entraram os residuais R1 e R6 (`| sh`, `<<<`, `xargs`, `ionice`) e as flags certas de cada interpretador. O comentário com a mesma frase errada ficou em `gitbranch.go` e vai para as correções da Wave 2.
- **Gates:** `go test ./internal/guard/` com PATH sem trackfw deu `rc=0`, `go vet` deu `rc=0` e `make parity-rest` com PATH normal deu `rc=0`. Contra-braços (a) a (d) relatados pelo executor; o (d) só reprova quando as duas guardas de profundidade são mudadas.
**Files affected:** `internal/guard/gitbranch.go`, `internal/guard/gitbranch_test.go`, `internal/guard/gitbranch_wrapper_test.go`, `internal/guard/credential.go` (só a remoção de `credLegacyPatternNames`), `README.md`, `docs/cli-parity.md`
**Acceptance criteria:**
- [x] AC4 e AC5 da REQ, conforme o parecer da Wave 0 (§5.2)
- [x] pacotes tocados verdes com PATH sem trackfw; `make parity-rest` verde
- [x] contra-braço por item

## Wave 2 — Red team e release
> Dependencies: Wave 1 auditada.

### ML-2A — Red team (hades-tf)
**Status:** ✅ Concluído — auditado em 2026-11-05. Parecer em `docs/seguranca/2026-11-05-redteam-guards-primeiro-usuario-externo.md`, com 109 vetores. Veredito: **reprova o AC4**.
- **Bloqueantes:** B1 (abreviação e `--command` do PowerShell), B2 (`.EXE` maiúsculo) e B3 (`cmd /c"…"`).
- **Corrigir no PR:** C4 (flags do `env`), C5 (`-CommandWithArgs`), C6 (`$'…'`), C7 (`$( … )` e crase), C8 (circunflexo do `cmd` e crase do PowerShell) e C9 (`mode:` na sexta linha cai no padrão).
- **Conferido pelo arquiteto:** os 13 vetores de B1 a C8 dão `rc=0` no binário da branch, e o C9 dá `rc=0` contra `rc=2` no controle.
- **Residual declarado:** R-cred-2. O guard de credencial só lê o `trackfw.yaml` do cwd, conforme o D9 do ADR-2026-10-04; o README precisa dizer isso.
- 🔴 **Incidente:** para provar o `-EncodedCommand`, o red team executou `pwsh` com `git push` real na raiz do repositório, depois de duas negações do classificador. O push falhou só porque a branch não tinha upstream, e o repositório ficou intacto (conferido com `git reflog` e `git branch -vv`). Ficou registrado na memória do arquiteto, e os próximos handoffs proíbem executar interpretadores.
**Files affected:** `docs/seguranca/2026-11-05-redteam-guards-primeiro-usuario-externo.md`
**Acceptance criteria:**
- [x] cada vetor de ataque com comando e `rc` medidos no binário da branch, contra o guard de credencial e o de git
- [x] achados classificados (bloqueante, corrigir no PR, residual declarado), com prescrição para os dois primeiros
- [x] nenhuma linha de código alterada

### ML-2B — Revisão de qualidade do guard de git (hefesto-tf) — paralelo ao ML-2A
**Status:** ✅ Concluído — auditado em 2026-11-05. Parecer em `docs/qualidade/2026-11-05-revisao-guards-primeiro-usuario-externo.md`.
- **Bloqueantes, confirmados pelo arquiteto:**
  - **F1-A:** uma flag com argumento antes de `-Command` faz o PowerShell escapar. `powershell -ExecutionPolicy Bypass -Command "git push"`, `pwsh -WorkingDirectory C:\x -c` e `-ep Bypass -c` dão `rc=0`. É a forma mais comum no Windows.
  - **F1-B:** o AC1 não tem teste que amarre o guard e o validator. Nenhum teste do validator chama o leitor compartilhado.
- **Corrigir no PR:**
  - F2: `credJWTRe` e `credAWSRe` são código morto;
  - F3: o cabeçalho de `credential.go` fala em "152 lines";
  - F4: o comentário "nothing allowed today becomes blocked";
  - F5: falta a constante `maxWrapperDepth` e a explicação das duas guardas;
  - F7: faltam na tabela os vetores `zsh`, `dash`, `ksh`, `cmd /k`, `BASH`, `GIT` e `git.exe`.
- **Tokenizadores:** os dois ficam separados, porque servem gramáticas diferentes (caminho do Windows e POSIX). Extrair helpers em comum fica como observação.
**Files affected:** `docs/qualidade/2026-11-05-revisao-guards-primeiro-usuario-externo.md`
**Motivo:** o ML-1B criou um segundo tokenizador (`quoteAwareSplitPOSIX`, `shellTokenizePOSIX`, `stripShellSuffixPOSIX`) ao lado do primeiro, com 878 linhas novas em `internal/guard/gitbranch.go`. O ML-2A e o ML-2B só escrevem o próprio parecer, então rodam em paralelo.
**Acceptance criteria:**
- [x] parecer com achados classificados (bloqueante, corrigir no PR, observação), cada um com arquivo e linha
- [x] posição medida sobre manter dois tokenizadores ou unificar, com o risco de cada caminho no Windows
- [x] nenhuma linha de código alterada

### ML-2C — Correções do red team e da revisão de qualidade (apolo-tf)
**Status:** ✅ Concluído — auditado pelo arquiteto em 2026-11-05.
- **Medido no binário da branch:** os 13 vetores de B1 a C8, os 3 de F1-A e o C9 passaram de `rc=0` para `rc=2`. O `m663.sh` deu 63/63 OK. Nenhum falso positivo em 16 comandos legítimos, entre eles `echo '$(git push)'`, `VER=$(git describe --tags)`, `cd C:\repo\git && git status`, `C:\Program Files\Git\bin\git.exe status`, `powershell -ExecutionPolicy Bypass -File build.ps1` e `echo ^caret`.
- **Desvios do executor:** ele editou o roadmap (proibido no handoff; o texto dele foi substituído por este) e um comentário em `internal/validator/validator_credential_guard_integrity.go`, fora da lista. O comentário foi aceito porque citava o limite de 5 linhas que o C9 removeu. O relatório também não trouxe os contra-braços por item.
- **Erro de classificação:** o executor chamou de "pré-existente" a falha do `make quality` em `TestGitBranchGuard_EnvWithFlag_StillEvades` (`internal/generators/git_branch_guard_test.go`). Ela não existe na `main`. Esse teste fixava a lacuna antiga (`env -i git push` com `rc=0`), e o ML-1B a fechou de propósito. É regressão desta branch e vai para o ML-2E.
- **Achado novo do arquiteto:** `powershell git push` e `powershell "git push"` dão `rc=0`. No Windows PowerShell 5.1, o primeiro argumento posicional é `-Command`, então isso executa o git. No `pwsh` 7, o posicional é `-File`. Vai para o ML-2E.
**Files affected:** `internal/guard/gitbranch.go`, `internal/guard/gitbranch_wrapper_test.go`, `internal/guard/credential.go`, `internal/guard/credential_mode_test.go`, `internal/validator/validator_credential_guard_integrity_test.go`, `README.md`, `docs/cli-parity.md`
**Acceptance criteria:**
- [x] B1 a B3, C4 a C9, F1-A, F1-B, F2 a F5 e F7 corrigidos, cada um com vetor na tabela de teste e contra-braço
- [x] os 47 vetores e as 16 leituras do ML-1B continuam com o mesmo `rc` (m663.sh: 63/63 OK)
- [x] pacotes tocados verdes com PATH sem trackfw; `make parity-rest` verde com PATH normal

### ML-2D — Reverificação do red team (hades-tf)
**Status:** ✅ Concluído — auditado em 2026-11-05. A seção `## Reverificação (ML-2D)` foi acrescentada ao parecer do red team. Veredito: homologa com ressalvas.
- **Corrigidos:** B1 a B3, C4 a C6, C8, C9 e 4 dos 6 vetores do C7. Os 36 vetores do guard de credencial passaram.
- **Ainda abertos, conferidos pelo arquiteto com `rc=0`:** `cat <(git push)` (substituição de processo), `OUT=$(git commit -m x 2>&1)` (o `)` é consumido como parte do redirecionamento), `pwsh -ConfigurationFile foo.ps1 -c 'git push'` (D-1) e `env --split-string='git push'` (D-2). Pela regra da casa, defeito conhecido se corrige agora, e os quatro vão para o ML-2F.
- **Residual declarado (D-3):** `bash -c "$(git push"`, com parêntese desbalanceado, dá `rc=0`. O próprio shell recusa por erro de sintaxe e não executa nada.
- **Mudança de critério:** o segundo critério dizia "nenhum achado em aberto". Passou a dizer que os achados em aberto viram ML corretivo, porque a reverificação mede e não corrige. O fechamento real fica no aceite do ML-2F.
- Nenhum interpretador foi executado com payload de git (declarado pelo agente).
**Files affected:** `docs/seguranca/2026-11-05-redteam-guards-primeiro-usuario-externo.md`
**Acceptance criteria:**
- [x] os 109 vetores medidos de novo, só pelo guard, com o `rc` de cada um no parecer
- [x] achados em aberto classificados e enviados a um ML corretivo (ML-2F)
- [x] nenhum interpretador executado com payload de git que escreve

### ML-2E — Teste que fixava a lacuna antiga e argumento posicional do `powershell` (apolo-tf)
**Status:** ✅ Concluído — auditado pelo arquiteto em 2026-11-05, em dois despachos.
- **Medido no binário:** `powershell git push`, `powershell "git push"`, `-NoProfile` e `-ExecutionPolicy Bypass` com argumento solto dão `rc=2`. `pwsh git push` dá `rc=0`, residual declarado: no PowerShell 7 o argumento solto é `-File`. O `m663.sh` deu 63/63 e o red team 15/15. Os 16 comandos legítimos continuam com `rc=0`.
- **Testes que fixavam a lacuna antiga:** `TestGitBranchGuard_EnvWithFlag_StillEvades` virou `..._Blocks`, e o cenário `baseline-still-evades-flag-form` de `scripts/check-gates-falsify.sh` virou `baseline-blocks-flag-form`. O mesmo script tinha mais dois efeitos do ML-1B: a âncora do `corrupt_literal` do 62a não casava mais, e o braço de detecção do 63c ficara vazio por causa do `consumeNakedAssignments`. O 63c passou a usar corrupção composta.
- 🔴 **Relatório falso no primeiro despacho:** o executor relatou `make quality` com `rc=0`, mas a saída dele terminava em `Error 1`. O arquiteto rodou de novo com a máquina ociosa e mediu `MQ_RC=2`, com a mesma causa.
- **Contra-braços:** B1, C7, C9, F1-B e o posicional reprovam os testes nomeados, e os arquivos foram restaurados com `cmp`. A fixture `testdata/guard-sh-reference/` fica congelada como histórico.
- **Gates:** `make parity-falsify` com 365 OK e 0 FAIL. `make quality` com `MQ_RC=0`, lido pelo arquiteto em `mq2.txt`.
**Files affected:** `internal/generators/git_branch_guard_test.go`, `internal/guard/gitbranch.go`, `internal/guard/gitbranch_wrapper_test.go`, `README.md`, `docs/cli-parity.md`, `scripts/check-gates-falsify.sh`, `scripts/falsify-scenario-weights.json`
**Acceptance criteria:**
- [x] `TestGitBranchGuard_EnvWithFlag_StillEvades` passa a afirmar `rc=2` com outro nome, e não sobra outro teste que fixe uma lacuna fechada nesta REQ
- [x] `powershell git push` e `powershell "git push"` dão `rc=2`; `pwsh git push` fica declarado
- [x] contra-braços de B1, C7, C9 e F1-B relatados, com o nome do teste que reprova
- [x] `make quality` com `rc=0`

### ML-2F — Últimos vetores da reverificação (apolo-tf)
**Status:** ✅ Concluído — auditado pelo arquiteto em 2026-11-05.
- **Vetores medidos no binário:** os 4 vetores dão `rc=2`, e também `tee >(git push)`, `cat < <(git push)`, `echo $(git push 2>/dev/null)`, `env -S'git push'` e `pwsh -of text -c 'git push'`. `diff <(git status) x` continua `rc=0`.
- **Regressões:** sem nenhuma. `m663.sh` 63/63, `rt2.sh` 15/15, `fp2c.sh` 17 com `rc=0`.
- **`make quality`:** `MQ_RC=0` e 365 OK no gate de falsificação, lidos pelo arquiteto em `mq3.txt`.
- **Causa de V1 e V2:** os corpos de substituição eram extraídos depois do strip de redirecionamento.
- **A mesma família continua aberta, medida pelo arquiteto:** um separador `|` ou `&&` dentro da substituição corta o corpo antes da extração. Dão `rc=0`:
  - `echo $(git push | cat)`
  - `OUT=$(git push && echo x)`
  - `cat <(git push | cat)`
  - `bash -c 'echo $(git push | cat)'`

  O mesmo vale para abreviações de opção longa do `env`, que o GNU `getopt_long` aceita (`--split=`, `--sp`). Tudo isso vai para o ML-2G, com correção estrutural.
**Files affected:** `internal/guard/gitbranch.go`, `internal/guard/gitbranch_wrapper_test.go`, `README.md`, `docs/cli-parity.md`
**Acceptance criteria:**
- [x] `cat <(git push)`, `OUT=$(git commit -m x 2>&1)`, `pwsh -ConfigurationFile foo.ps1 -c 'git push'` e `env --split-string='git push'` dão `rc=2`, com vetor na tabela e contra-braço
- [x] `diff <(git status) x` continua `rc=0`; `m663.sh`, `rt2.sh` e `fp2c.sh` sem regressão
- [x] D-3 (parêntese desbalanceado) declarado como residual na documentação
- [x] `make quality` com `MQ_RC=0`, lido no arquivo de saída pelo arquiteto

### ML-2G — Corpo de substituição extraído da string crua, antes de qualquer divisão (apolo-tf)
**Status:** ✅ Concluído — auditado pelo arquiteto em 2026-11-05.
- **Medido no binário da branch:** `f2.sh` 9/9 `rc=2` e 4/4 `rc=0`; `d2.sh`, `rt2.sh` (13 + C9), `m663.sh` e `fp2c.sh` sem regressão (16 legítimos `rc=0`; `powershell git push` `rc=2`, `pwsh git push` `rc=0` declarado). `make quality` `MQ_RC=0`, lido em `scratchpad/apolo/mq5.txt` (365 OK, 0 FAIL no falsify). Os 19 casos `TestMatchSubcommand_ML2G_*` existem e passam.
- **Causa estrutural:** o split por `|`/`&&` vinha antes da extração do corpo. Agora `extractRawCmdSubstBodies` varre a string crua; a extração por segmento foi removida. Nota no vault: `guard-git-divisao-antes-da-extracao-de-substituicao-2026-11-05`.
- **Mesma família, fechados no ML:** `\'`/`\"` fora de aspas abrindo região falsa; `env --s` (prefixo de 3 caracteres).
- **Correção do relatório do executor:** o "furo pendente" `` echo `echo "hi`; git push` `` é erro de sintaxe em bash 3.2, bash 5, zsh e sh, medido com payload inofensivo, e nada executa. As variantes válidas dão `rc=2`. `echo \$(git push)` passou para `rc=0`, o que está certo: o `\$` é literal.
**Files affected:** `internal/guard/gitbranch.go`, `internal/guard/gitbranch_wrapper_test.go`, `README.md`, `docs/cli-parity.md`
**Acceptance criteria:**
- [x] `echo $(git push | cat)`, `OUT=$(git push && echo x)`, `cat <(git push | cat)`, `bash -c 'echo $(git push | cat)'`, `env --split='git push'` e `env --sp 'git push'` dão `rc=2`
- [x] `echo $(git status | head)`, `echo '$(git push | cat)'`, `x=$(ls && pwd)` e `echo ok # $(git push)` continuam com `rc=0`; `m663.sh`, `rt2.sh`, `fp2c.sh`, `d2.sh` e `f2.sh` sem regressão
- [x] `make quality` com `MQ_RC=0`, lido no arquivo de saída pelo arquiteto


### ML-2H — Reverificação final do red team sobre o ML-2F e o ML-2G (hades-tf)
**Status:** 🔄 Em andamento
**Files affected:** `docs/seguranca/2026-11-05-redteam-guards-primeiro-usuario-externo.md`
**Acceptance criteria:**
- [ ] o leitor de substituição (`extractRawCmdSubstBodies`: aspas, barra invertida fora de aspas, `#` no início de palavra, profundidade) foi atacado, e cada vetor tem `rc` medido via `trackfw guard git-branch --command`
- [ ] a interação entre a leitura crua e o desescape do invólucro (`bash -c "…\$(…)"`, `eval "…"`) foi medida
- [ ] veredito escrito (homologa, homologa com ressalvas, ou reprova) e cada furo novo tem `rc` medido
