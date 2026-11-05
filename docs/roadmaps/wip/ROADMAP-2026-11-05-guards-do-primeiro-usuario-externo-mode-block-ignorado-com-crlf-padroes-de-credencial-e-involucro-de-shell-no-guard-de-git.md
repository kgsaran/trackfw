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
**Status:** ⬜ Pendente
**Files affected:** `internal/guard/gitbranch.go`, `internal/guard/gitbranch_wrapper_test.go`, `internal/guard/credential.go`, `internal/guard/credential_mode_test.go`, `internal/validator/validator_credential_guard_integrity_test.go`, `README.md`, `docs/cli-parity.md`
**Acceptance criteria:**
- [ ] B1 a B3, C4 a C9, F1-A, F1-B, F2 a F5 e F7 corrigidos, cada um com vetor na tabela de teste e contra-braço
- [ ] os 47 vetores e as 16 leituras do ML-1B continuam com o mesmo `rc`
- [ ] pacotes tocados verdes com PATH sem trackfw; `make parity-rest` verde com PATH normal

### ML-2D — Reverificação do red team (hades-tf)
**Status:** ⬜ Pendente
**Files affected:** `docs/seguranca/2026-11-05-redteam-guards-primeiro-usuario-externo.md`
**Acceptance criteria:**
- [ ] os 109 vetores medidos de novo, só pelo guard, com o `rc` de cada um no parecer
- [ ] nenhum bloqueante ou "corrigir no PR" em aberto
- [ ] nenhum interpretador executado com payload de git que escreve
