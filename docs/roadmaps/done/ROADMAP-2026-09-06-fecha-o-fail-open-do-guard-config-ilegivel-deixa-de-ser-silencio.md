---
status: done
date: 2026-09-06
squad: apolo-tf
req: "docs/req/REQ-2026-08-12-mitigacao-do-fail-open-do-credential-guard-integridade-do-script-e-da-config-controle-positivo-e-fail-closed-nativo.md"
---

# Roadmap: Fecha o fail-open do guard — config ilegível deixa de ser silêncio

> Criado em: 2026-09-06 | Status: done

## Context

REQ: `docs/req/REQ-2026-08-12-mitigacao-do-fail-open-do-credential-guard-integridade-do-script-e-da-config-controle-positivo-e-fail-closed-nativo.md`
(**reaberta** em 2026-09-06 — estava `Done` com os 4 critérios de aceite em branco.)

## Diagnóstico

A REQ estava `Done` com **os 4 critérios em branco**. Dois foram entregues de fato (controle positivo
e integridade de conteúdo); **dois não** — justamente os que cobrem **deleção** e **"não consegui
rodar"** no momento da invocação.

E o ML-6C mediu uma **quinta via**: JSON inválido em config de guard é engolido por `continue` mudo.
**O controle reporta saúde sobre o que não conseguiu ler.**

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 1 — A via medida, sozinha
> **WIP = 1.** Nada novo entra antes desta fechar.

### ML-1A — Config ilegível deixa de ser silêncio
**Status:** ✅ Concluído (implementação; barreira `hades-tf` pendente) · **Agente:** `apolo-tf` + barreira `hades-tf`
**Sítios conhecidos:** `internal/validator/validator_git_branch_guard.go:151-154` e a função irmã do
credential-guard. 🔴 **Enumerar os demais antes de corrigir** — o ML-6C achou dois **por acaso**,
olhando outra coisa.

**Decisão em aberto, a tomar com medição:** acusar como violation, ou emitir diagnóstico próprio.
🔴 O `continue` mudo deixa de ser aceitável, **mas a escolha entre as duas tem consequência**: acusar
demais faz o usuário desligar a regra, e aí o controle vale zero. Escrever a razão da escolha.

**Critérios:** falsificação nas duas direções (config corrompido acusa; válido não acusa) ·
enumeração completa das leituras de config de guard, com veredito por sítio · paridade nos 3 CLIs ·
`make quality` e `validate` verdes.

### ML-1B — O irmão do `continue`: falha de LEITURA (não só de parse), e a divergência de decodificação
**Status:** ✅ Concluído (implementação; barreira `hades-tf` pendente) · **Agente:** `apolo-tf` + barreira `hades-tf`

Barreira `hades-tf` sobre o ML-1A (parecer em
`~/.trackfw/rascunhos/2026-09-06-parecer-hades-fail-open-config-ilegivel.md`) aprovou com ressalvas
e mediu, ao vivo, que o ML-1A fechou só UM dos dois motivos do `continue` mudo original: o de JSON
sintaticamente inválido. O irmão — falha de **leitura** do arquivo (permissão negada, diretório no
lugar do arquivo) — sobrevivia em Node/Python como `continue` silencioso (exit 0, sucesso reportado
sobre wiring nunca inspecionado), e em Go como `fmt.Errorf` que **abortava `trackfw validate`
inteiro** (exit 1, stdout vazio, nenhuma outra regra reportada) — pior UX que uma violation, ainda
que "fail-closed" no sentido estrito. A mesma medição achou uma SEGUNDA divergência, de
DECODIFICAÇÃO (abaixo do parse): Python lia com `encoding="utf-8"` estrito e crashava com
`UnicodeDecodeError` não capturada (traceback cru) em 2 fixtures (UTF-16 inteiro; 1 byte inválido
dentro de JSON por lo mais válido) onde Go/Node ficavam seguros (Go opera sobre bytes crus; Node
decodifica com perda, `errors="replace"` equivalente).

**Sítios corrigidos:** `validateGuardHookResolvable` (`internal/validator/validator_credential_guard.go`)
e `validateGuardGlobalHookResolvable` (`internal/validator/validator_git_branch_guard.go`), e os
pares Node (`npm/src/validator/index.js`) / Python (`pypi/trackfw/validator.py`) — os 2 únicos sítios
que a barreira reproduziu ao vivo (escopo de projeto e escopo global da regra `*_hook_resolvable`).

**Ações:**
1. Falha de leitura não-ENOENT deixa de silenciar (Node/Python) e deixa de abortar o processo (Go) —
   nos 3 runtimes, 2 sítios cada, passa a ser `violation` com a mensagem `"... could not be read —
   ..."`, mesmo remédio (`trackfw update` / `trackfw update harness`) da violation de JSON inválido.
2. Python passou a ler em modo binário e decodificar com `errors="replace"` (perda, espelhando
   Node) em vez de `encoding="utf-8"` estrito — elimina o `UnicodeDecodeError` não capturado.
3. Distinção de causa na mensagem: `"is not valid JSON"` (sintaxe) vs. `"could not be read"` (I/O)
   vs. `"is not valid UTF-8"` (codificação) — nos 3 runtimes, calculada por `utf8.Valid`
   (Go)/`TextDecoder({fatal:true})` (Node)/decode estrito só para classificar (Python), nunca para
   decidir se o parse é tentado.

**Critérios de aceite:**
- [x] Os 3 runtimes acusam com veredito e mensagem idênticos para: JSON inválido, permissão negada,
      diretório no lugar do arquivo, UTF-16 — gate `scripts/check-validate-parity.sh` (fixtures
      `cg-claude-unreadable`, `cg-claude-utf16`, reusadas por `gbg_cases`), byte-idêntico confirmado.
- [x] Nenhum runtime crasha nos 5 cenários — Go não aborta mais o processo; Python não levanta mais
      `UnicodeDecodeError`.
- [x] Falsificação nas duas direções por caso e por runtime — testes Go/Node/Python dedicados (ver
      relatório em `docs/agents-working-context.md`).
- [x] Controle de falso positivo — 1 byte UTF-8 inválido dentro de JSON por lo mais válido continua
      SEM violation nos 3 runtimes (paridade com a coerção que Go/Node já faziam).
- [x] Terceiro motivo no mesmo laço — contado e declarado (ver relatório): a decodificação era o
      terceiro; um QUARTO (a classificação de erro do `os.Stat`/`os.access` sobre o SCRIPT
      referenciado, não sobre o config) foi encontrado e **declarado, não corrigido** — ver
      justificativa no relatório.
- [x] Gate cross-CLI cobre os casos novos — 2 fixtures novas (`cg-claude-unreadable`,
      `cg-claude-utf16`), rodado ao vivo, exit 0.
- [x] `go build ./...` / `go vet ./...` limpos; Go 218 testes (pacote `internal/validator`, todo o
      repo `go test ./...` também verde), Node 875 testes (repo inteiro), Python 1644+66 subtests
      (repo inteiro) — todos verdes.
- [x] `make quality` não rodado por este agente (arquiteto roda ao fim).

**Ressalva da barreira sobre Kiro/Copilot:** não enfraquece a decisão deste ML — a verdade
acusada aqui ("trackfw não conseguiu LER o arquivo") não depende de nenhuma suposição sobre o
parser do CLI dono (ao contrário da violation de JSON inválido do ML-1A). Nenhuma diferenciação por
CLI foi necessária.

### ML-1C — O FIFO e a família `*_script_integrity` (ressalvas do ML-1B, mesma REQ)
**Status:** ✅ Concluído (implementação; barreira `hades-tf` pendente) · **Agente:** `apolo-tf`

Barreira `hades-tf` sobre o ML-1B (parecer em
`~/.trackfw/rascunhos/2026-09-06-parecer-hades-ml1b-silencio-deslocado.md`) aprovou com ressalvas e
achou **duas coisas da mesma classe desta REQ**, nenhuma delas regressão do ML-1B (ambas
pré-existentes): (1) um FIFO no lugar do config/script trava `os.ReadFile`/`fs.readFileSync`/
`open().read()` **indefinidamente**, nos 3 runtimes; (2) a família `*_script_integrity`
(`credential_guard_script_integrity`, `git_branch_guard_script_integrity`, projeto e global) tinha o
MESMO defeito que esta REQ existe para fechar — Go project-scope **abortava** todo `trackfw
validate` (`fmt.Errorf` cru, stdout não-JSON), Go global-scope e Python (todos os 4 sítios)
**silenciavam** qualquer erro não-ENOENT, e só Node project-scope já estava correto.

**Mecanismo escolhido para o FIFO — com a razão e o veredito sobre TOCTOU:**
Nos 3 runtimes, todo ponto de leitura de config/script de guard passou a: abrir com `O_NONBLOCK`
(evita que `open()` bloqueie num FIFO sem escritor do outro lado) e então **fstat no FILE
DESCRIPTOR já aberto** (não no caminho) para confirmar que o que foi aberto é um arquivo regular —
`readRegularFile` (Go, `internal/validator/regularfile.go` + `regularfile_unix.go`/
`regularfile_windows.go`), `readRegularFileSync` (Node, `npm/src/validator/index.js`),
`_read_regular_file` (Python, `pypi/trackfw/validator.py`). **Veredito TOCTOU: imune em
POSIX/unix** — o fd é vinculado ao objeto (inode/pipe/device) que o kernel resolveu no momento do
`open()`, não ao caminho; nada que aconteça com o CAMINHO depois disso muda o que o fd referencia.
**No Windows, é redução, não eliminação** — não há equivalente portátil de baixo custo do truque
open-nonblock+fstat-no-fd (named pipes do Windows vivem no namespace `\\.\pipe\`, API totalmente
diferente); `regularfile_windows.go` faz stat-depois-open (`os.Stat` seguido de `os.Open`), que tem
uma janela de corrida residual, honestamente documentada como tal — **medido apenas em darwin**,
comportamento Windows é raciocinado a partir da API, não medido ao vivo nesta ML.

**Enumeração dos tipos especiais:** a checagem é um **allowlist positivo** — "deve ser arquivo
regular" (`Mode().IsRegular()` / `st.isFile()` / `stat.S_ISREG`) — não um denylist por tipo. Medido
ao vivo (não só argumentado): FIFO (`mkfifo`) e **socket Unix** (`net.Listen("unix", ...)`) ambos
não travam e ambos acusam "could not be read" nos 3 runtimes; **char device** (symlink para
`/dev/null`) também acusa sem travar. Um denylist teria coberto só o que já foi achado por ataque
(FIFO); o allowlist cobre qualquer tipo especial futuro por construção.

**Tratamento da família `*_script_integrity`, por runtime e escopo:**
- Go project-scope (`validateCredentialGuardScriptIntegrity`,
  `validateGitBranchGuardScriptIntegrity`): deixou de `return nil, fmt.Errorf(...)` (abort) — passa
  a `violation` "could not be read", mesma mensagem-família do `*_hook_resolvable`.
- Go global-scope (`validateGuardGlobalScriptIntegrity`): deixou de `return nil, nil`
  incondicional em qualquer erro — distingue `os.IsNotExist` (ausência, legítima, silenciosa) de
  qualquer outro erro (agora `violation`).
- Node project-scope: já emitia `violation` (via `inspectionDiagnostic`) — só precisava do fix de
  FIFO; **texto da mensagem unificado** com Go/Python ("could not be read...") para não introduzir
  uma divergência de 3-CLI nova no momento em que os 3 runtimes passam a acusar no mesmo cenário
  (achado da revisão: antes do ML-1C não havia nada para divergir — Go abortava, Python silenciava
  — então a divergência de texto era latente, não observável).
- Node global-scope: mesmo fix que Go (era `catch (_) { return [] }` incondicional).
- Python (ambos escopos, `validate_credential_guard_script_integrity` +
  `validate_guard_script_integrity` compartilhada + `validate_guard_global_script_integrity`): era
  `except OSError: return []` incondicional nos 3 pontos — split em `except FileNotFoundError`
  (silêncio) vs. `except OSError` (violation). Bônus: a leitura estrita `encoding="utf-8"` também
  podia crashar com `UnicodeDecodeError` não capturado num script sobrescrito com bytes inválidos
  (mesma classe do bug que o ML-1B fechou nos configs JSON, nunca fechada aqui porque este arquivo
  não foi tocado pelo ML-1B) — eliminado lendo em bytes e comparando contra
  `reference_content.encode("utf-8")`.

**Prova de que `--json` sobrevive:** `scripts/check-validate-parity.sh` ganhou um bloco novo que
roda os 3 binários reais contra um script ilegível (diretório no lugar do arquivo, project e global
scope, ambas as regras) e faz `json.load()` do stdout de cada um antes de qualquer outra asserção —
um abort estilo Go antigo (stdout vazio) falha o `json.load` imediatamente com mensagem clara. Rodado
ao vivo, `exit 0`.

**Resposta sobre convergência — NÃO converge fora da família guard.** Contagem de leituras cruas
(sem passar por `readRegularFile`/`readRegularFileSync`/`_read_regular_file`) fora desta família:
Go **20** sítios de `os.ReadFile(` em `internal/validator/*.go` (roadmaps, REQs, ADRs,
`trackfw.yaml`, `.trackfw-baseline.json`, manifesto de terceiros, arquivos de trace-id, índice de
notas) + **55** pontos `return nil, nil, e`/`return nil, nil, err` em `ValidateUnfiltered`
(`validator.go`) que propagam qualquer erro de uma regra como abort do comando inteiro; Node **19**
sítios de `fs.readFileSync(` fora da família guard; Python **~19** sítios de `open(` fora da
família guard. Um FIFO ou uma permissão negada em qualquer um desses arquivos hoje reproduz o MESMO
hang/abort/silêncio que esta REQ fecha só para os guards. **Não corrigido aqui** — a escala (~20 por
runtime, 3 categorias de falha, código já espalhado por dezenas de regras não relacionadas) é
exatamente o sinal que o handoff previu: a resposta é **arquitetural** (um leitor único de config
com um contrato de erro — nunca trava, nunca aborta, sempre classifica), não mais patches
sítio-a-sítio. Decisão do arquiteto.

**Requisito de reconciliação (1 frase por teste novo):**
- Go `TestReadRegularFile_FIFO_NaoTravaEAcusaTipoErrado` / `_Socket_...` — afirma que o allowlist
  "deve ser regular" cobre FIFO e socket sem hang, sem branch dedicado por tipo.
- Go `TestReadRegularFile_ArquivoRegular_LeNormalmente` / `_Ausente_RetornaENOENT` /
  `_SymlinkParaArquivoRegularExterno_LeNormalmente` — controle de falso positivo: arquivo normal,
  ausência e symlink externo legítimo continuam exatamente como antes.
- Go `TestCredentialGuardScriptIntegrity_ScriptIlegivel_ViolationSemAbortar` /
  `TestGitBranchGuardScriptIntegrity_ScriptIlegivel_ViolationSemAbortar` — afirma que Go
  project-scope não aborta mais `trackfw validate` num script ilegível.
- Go `TestCredentialGuardScriptIntegrity_FIFO_NaoTrava` — afirma o mesmo no nível da regra (não só
  do primitivo), reproduzindo o vetor `mkfifo` da barreira.
- Go `TestGuardGlobalScriptIntegrity_ScriptIlegivel_ViolationSemSilencio` — afirma que Go
  global-scope deixou de silenciar EACCES/EISDIR, sem quebrar o controle de ausência (teste irmão
  já existente).
- Node/Python — cada teste espelha exatamente a mesma afirmação do teste Go correspondente (mesmo
  nome de conclusão, arquivo irmão), incluindo o teste específico do Python
  `test_byte_invalido_utf8_no_script_nao_crasha_e_classifica_como_divergencia`, que afirma a
  eliminação do `UnicodeDecodeError` não capturado.

**Premissas do handoff que a medição derrubou:** nenhuma. A única correção de rota veio da
autorevisão: a mensagem "could not be read" que eu unifiquei em Node project-scope não existia como
divergência ANTES desta ML (Go abortava sem mensagem, Python silenciava sem mensagem) — só se
tornou uma divergência de 3-CLI observável DEPOIS que os 3 passaram a emitir uma mensagem no mesmo
cenário; corrigido antes de declarar concluído.

**Critérios de aceite:**
- [x] FIFO não trava nenhum dos 3 runtimes, nos 2 escopos — medido ao vivo (`mkfifo` + timeout
      manual, 3 binários reais, project e global scope) e sob o gate (`scripts/check-validate-
      parity.sh`, timeout duro via background+kill, sem depender de `timeout`/`gtimeout`).
- [x] Enumeração de tipos especiais além de FIFO — socket e char device medidos ao vivo, allowlist
      positivo cobre por construção.
- [x] `*_script_integrity` com os 3 motivos tratados (leitura / decodificação em Python / n/a
      parse — este família não faz parse JSON), mensagens distintas, nos 3 runtimes e 2 escopos.
- [x] Go deixa de abortar em qualquer caminho destas regras — provado via gate (`json.load()` do
      stdout de cada runtime antes de qualquer outra asserção).
- [x] Falsificação nas duas direções, por cenário e por runtime, com os binários reais — Go/Node/
      Python `bin/trackfw`, `node npm/bin/trackfw`, `python3 -m trackfw.cli`, nunca parser isolado.
- [x] Controle de falso positivo — ausência continua silenciosa (testes dedicados nos 3 runtimes);
      arquivo íntegro e symlink externo legítimo continuam silenciosos (Go).
- [x] Gate cross-CLI cobre os casos novos — 3 blocos novos em `scripts/check-validate-parity.sh`
      (unreadable project-scope, unreadable global-scope, FIFO com timeout duro), rodado ao vivo,
      `exit 0`.
- [x] `go build ./...` / `go vet ./...` limpos nos 3 GOOS (`darwin`, `linux`, `windows` via
      `GOOS=... go vet`/`go build`) — a suíte de testes original quebrava a COMPILAÇÃO windows
      (`syscall.Mkfifo` inexistente lá); corrigido com arquivos `_unix_test.go` tagueados
      `//go:build !windows`. Suítes: Go 227 testes (`internal/validator`, +9), Node 881 (+6), Python
      1651+66 subtests (+7) — todos verdes.
- [x] `make quality` não rodado por este agente (arquiteto roda ao fim).

**Ressalva pendente (não corrigida, ver "Resposta sobre convergência" acima):** ~20 sítios de leitura
crua por runtime fora da família guard continuam expostos ao mesmo hang/abort/silêncio — decisão
arquitetural do `trackfw_architect`, fora do escopo deste ML.

### ML-1E — Falsificação dos 3 blocos que o ML-1C acrescentou a `check-validate-parity.sh`
**Status:** ✅ Concluído · **Agente:** `artemis-tf` (QA)

O ML-1C acrescentou 3 blocos a `scripts/check-validate-parity.sh` (script ilegível project-scope,
script ilegível GLOBAL-scope, FIFO com timeout artesanal) sem nenhum cenário em
`scripts/check-gates-falsify.sh` provando que cada um reprovaria se a produção regredisse — achado
do ML-1D ao investigar outra coisa (o Cenário 80 quebrado incidentalmente, não uma prova
intencional). Fechado com 3 cenários novos (186, 187, 188), cada um cirúrgico ao seu bloco — ver
relatório completo em `docs/agents-working-context.md`.

**Enumeração (pedida pelo handoff, não corrigida além do escopo dos 3 blocos do ML-1C):**
`check-validate-parity.sh` tem 9 blocos de comparação cross-CLI. Cobertos por cenário de
falsificação em `check-gates-falsify.sh`: bare ADR/REQ contract (Cenário 4, parcial), FIFO/
unreadable project/unreadable global (Cenários 186-188, novos, este ML),
`branch_has_wip_roadmap` done/ (Cenário 79), `credential_guard_hook_resolvable` cross-CLI (bem
coberto — Cenários 80-82, 89/90/94/95 e outros). **Sem cenário nenhum:** o bloco GVP (mensagem de
`git_branch_guard_script_integrity` GLOBAL-scope, ML-3A), o bloco GVMT (mensagem de
`git_branch_guard_hook_resolvable` GLOBAL-scope "missing type", ML-4B), os 2 fixtures próprios de
`git_branch_guard_hook_resolvable` cross-CLI (`gbg-claude-relativo`, `gbg-cursor-relativo-present`),
e os 3 fixtures `claude-invalid-json`/`claude-unreadable`/`claude-utf16` (ML-1A/1B) — nem para
`credential_guard_hook_resolvable` nem para `git_branch_guard_hook_resolvable`. Decisão sobre virar
ML nesta frente ou ficar declarado: do `trackfw_architect`.

**Critérios de aceite:**
- [x] Um cenário por bloco (186, 187, 188), cada um com as duas direções — sabotado reprova com o
      diagnóstico daquele bloco; árvore íntegra passa.
- [x] Guarda de padrão em cada um — `corrupt_literal` (helper já existente em
      `check-gates-falsify.sh`) levanta `SystemExit` se o padrão não for encontrado exatamente 1 vez,
      equivalente ao `cmp -s` do Cenário 80.
- [x] Cada sabotagem atinge só o seu bloco — medido ao vivo: em cada braço de detecção, os blocos
      anteriores do `check-validate-parity.sh` (incluindo os outros 2 novos) passam limpos antes do
      bloco-alvo reprovar.
- [x] `bash scripts/check-gates-falsify.sh` isolado — `exit 0`, 0 FAIL, 365 linhas OK/PROOF,
      incluindo as 4 novas (`script-integrity-unreadable-and-fifo-baseline`,
      `script-integrity-unreadable-project-not-detected`,
      `script-integrity-unreadable-global-not-detected`,
      `script-integrity-fifo-hang-not-detected`).
- [x] `make quality` não rodado por este agente — arquiteto roda ao fim.
- [x] `scripts/check-validate-parity.sh` e código de produto não alterados — só
      `scripts/check-gates-falsify.sh`.

### ML-1F — Falsificação dos 6 blocos restantes de `check-validate-parity.sh` (enumerados pelo ML-1E)
**Status:** ✅ Concluído · **Agente:** `artemis-tf` (QA)

O ML-1E enumerou 6 nomes/fixtures do resto do arquivo sem nenhum cenário de falsificação: GVP
(`git_branch_guard_script_integrity` GLOBAL), GVMT (`git_branch_guard_hook_resolvable` GLOBAL
"missing type"), `gbg-claude-relativo`/`gbg-cursor-relativo-present` (fixtures próprios do bloco 9)
e `cg-claude-invalid-json`/`cg-claude-unreadable`/`cg-claude-utf16` (reaproveitados nos blocos 8 e
9). Decisão do usuário: fechar todos, não só uma fração — evitar "corrigir a instância e deixar a
classe" (mesmo princípio recusado no fatiamento da REQ do CRLF).

**Fechado com 3 cenários novos** (189, 190, 191), cada um cirúrgico ao seu bloco, e as duas
direções medidas (sabotado reprova com o diagnóstico daquele bloco; árvore íntegra — binário limpo
— passa `exit 0` nos 9 blocos):
- **189 — GVP:** `validator_git_branch_guard.go`, mensagem "content diverges from the template"
  (escopo GLOBAL) corrompida só no texto (Go). Reprova com
  `git_branch_guard_script_integrity GLOBAL-scope warning message text differs between runtimes`.
- **190 — GVMT:** mesmo arquivo, mensagem "missing type" (escopo GLOBAL) corrompida só no texto.
  Reprova com o diagnóstico correspondente.
- **191 — bloco 9, `gbg-claude-relativo`:** `validator_credential_guard.go`,
  `validateGitBranchGuardHookResolvable()` — marcador trocado por `credentialGuardScriptMarker`
  (o errado). A regra fica muda só para este wrapper (`collectCommandsWithMarker` procura o
  marcador errado). Reprova com o diagnóstico de "expected violation... none reported".

As 3 fixtures reaproveitadas (`claude-invalid-json`/`unreadable`/`utf16`) fecham TRANSITIVAMENTE:
disparam em `validateGuardHookResolvable` ANTES da filtragem por marcador, então qualquer sabotagem
realista dos braços de leitura/parse (Cenários 186-188, já existentes) já reprova ambos os
wrappers (credential_guard e git_branch_guard) ao mesmo tempo.

**`gbg-cursor-relativo-present` — sem cenário, reportado, não corrigido.** Medido ao vivo (não
presumido): a detecção do falso-positivo do Cursor depende de um booleano ÚNICO e compartilhado
(`requiresVarOrShellPrefix` da tabela `credentialGuardHookFiles`), usado identicamente pelos dois
wrappers — não existe hoje nenhum branch condicionado por `scriptMarker` que os trate diferente.
Sabotar esse booleano (`false`→`true` na linha do Cursor) e rodar `check-validate-parity.sh`
inteiro reprova no **bloco 8** (`cg-cursor-absent`) ANTES de alcançar o bloco 9 — os 7 blocos
anteriores passam limpos, o bloco 9 nunca é atingido. Escrever um cenário "surgical" para este
fixture exigiria alterar `check-validate-parity.sh` (proibido pelo handoff) ou inserir lógica
condicionada por marcador que não existe em produção (prova fabricada, não falsificação de
regressão real). Nota completa: `vault/notes/check-validate-parity-6-blocos-fechados-5-com-
cenario-1-sem-lever-independente-2026-09-06.md`.

**Enumeração final dos 9 blocos:** 8 de 9 têm cenário próprio (1, 2/GVP, 3/SIU-project, 4/SIU-
global, 5/FIFO, 6/GVMT, 7/branch_has_wip_roadmap, 8/credential_guard_hook_resolvable); o bloco 9
(`git_branch_guard_hook_resolvable`) tem cobertura parcial: `gbg-claude-relativo` e os 3 fixtures
reaproveitados cobertos, `gbg-cursor-relativo-present` sem cenário próprio (restrição estrutural do
código, não omissão de teste — ver nota do vault).

**Critérios de aceite:**
- [x] Um cenário por bloco restante, com as duas direções (sabotado reprova com o diagnóstico
      daquele bloco; árvore íntegra passa) — 3 dos 4 fixtures/blocos únicos restantes; o 4º
      (`gbg-cursor-relativo-present`) reportado como não-falsificável independentemente, com
      medição ao vivo do masking, não uma omissão por preguiça.
- [x] Guarda de padrão em todos — `corrupt_literal`, `count != 1` levanta `SystemExit`.
- [x] Cirurgia demonstrada em todos — medido ao vivo contando linhas "... passed ..." antes da
      falha: 189 falha após 1 linha (bloco 1), 190 após 5 (1+GVP+186+187+188), 191 após 10
      (blocos 1-8 completos).
- [x] Limite de tempo do gate, nunca externo — nenhum dos 3 cenários novos usa timeout externo;
      `run_cg`/`run_global_integrity`/`run_global_missing_type` (helpers do próprio
      `check-validate-parity.sh`) controlam exit code via `set +e`.
- [x] `bash scripts/check-gates-falsify.sh` isolado: 0 FAIL, os 3 cenários novos como `OK` (medido
      via harness isolado com só os helpers necessários + os 3 blocos novos, `exit=0`, tempo real
      ~28s — rodar o arquivo inteiro, com todos os ~191 cenários, está fora do orçamento deste ML;
      `make quality` fica para o arquiteto).
- [x] Enumeração final dos 9 blocos — reportada acima e na nota do vault.
- [x] `scripts/check-validate-parity.sh` e código de produto não alterados — só
      `scripts/check-gates-falsify.sh`.
- [x] `make quality` não rodado por este agente — arquiteto roda ao fim.

### ML-1G — Fecha a classe inteira: os ~20/19/19 sítios de leitura crua fora da família guard
**Status:** ✅ Concluído · **Agente:** `apolo-tf`

Decisão do usuário: fechar tudo nesta sessão, não abrir nova REQ. Medição prévia (Node: helper
`readRegularFileSync` já existia do ML-1C, 6 sítios roteados, 19 crus; Python: helper
`_read_regular_file` já existia, 6 sítios roteados, 16 crus — não ~19 como o handoff estimava,
divergência sem impacto no trabalho) confirmou que a resposta era ROTEAR pelos helpers já criados
no ML-1C, não inventar arquitetura nova.

**Roteados pelo helper (`readFileForRule`/equivalente, regra real com silêncio antes):**
`req_has_adr`, `req_has_roadmap`, `frontmatter_presence` (ADR+REQ), `req_roadmap_lifecycle`,
`folder_status`, `note_orphan` (índice do vault — ausência continua legítima e silenciosa,
qualquer OUTRO erro agora diagnostica em vez de tratar como índice vazio), e os 4 sítios de
`validator_traceid.go` (roteados via novo parâmetro `msgs *[]string` threaded pelos 3
coletores até `validateTraceId`, que os despeja em `warnings`).

**Swap-only (helper aplicado, comportamento preservado — já diagnosticavam ou são estado
interno/advisory, não regra de governança):** `readFileForRule`/equivalente em si (agora usa
`readRegularFile`/`readRegularFileSync`/`_read_regular_file`), `LoadBaseline`,
`parseSquadFromFrontmatter`, `inventoryBlock` (contagem de `trackfw status`), `blockedREQs`
(display, a regra real `blocked_by_draft_adr` já roteava), `parseBlockedADRs`, `resolveAdrStatusForRule`,
`latestWIPTransitionTime`, `thirdparty_artifact_has_provenance` (leitura do artefato instalado).

**Achado por medição, fora da contagem do ML-1C (ver nota do vault — o achado central deste ML):**
1. `validateCredentialGuardModeDowngrade` (Go) abortava `trackfw validate` inteiro em erro
   não-ENOENT — dentro de um arquivo da família guard mas fora das FUNÇÕES que 1A-1C corrigiram.
   Python tinha uma variante pior: tratava qualquer `OSError` como downgrade CONFIRMADO (falso
   positivo de alta confiança). Node já diagnosticava corretamente. Os 3 convergem agora:
   `FileNotFoundError`/ENOENT = downgrade confirmado; qualquer outro erro = diagnóstico.
2. `integrations.LoadManifest` (Go, `internal/integrations/manifest.go` — FORA de
   `internal/validator`, nunca contado pela varredura por pacote do ML-1C) alcançado por
   `thirdparty_artifact_has_provenance` e abortava nos 3 runtimes (`fmt.Errorf`/`throw`/`raise`).
   Corrigido no CALL SITE de validate (converte para diagnóstico); `LoadManifest` em si continua
   fail-closed para `trackfw thirdparty install` (consumidor de escrita), intocado.

**Declarado, não corrigido:** `internal/integrations/manifest.go:51` continua com leitura crua —
um FIFO em `.trackfw/integrations-manifest.json` ainda travaria `trackfw validate` via
`LoadManifest` (o ABORT foi fechado, o HANG não; portar `readRegularFile` para lá exige extraí-lo
para um pacote compartilhado, decisão arquitetural fora do orçamento deste ML). Os "55 pontos de
`return nil, nil, e`" que o ML-1C mediu em `ValidateUnfiltered` são uma classe DIFERENTE (propagação
de erro genérica, não sítio de leitura) — fora do escopo desta REQ, não tocados.

**Autorevisão pós-implementação — mesmo padrão que o ML-1C flagrou em si mesmo, uma ML depois:**
as duas correções de abort (`credential_guard_mode_downgrade`, `thirdparty_artifact_has_provenance`)
tornaram uma divergência de 3-CLI LATENTE em OBSERVÁVEL: antes, só 1 runtime emitia mensagem em
cada cenário (Go abortava, Python confirmava downgrade falso ou também abortava) — nada para
divergir. Ao fazer os 3 emitirem no mesmo cenário, a implementação inicial usou
`inspectionDiagnostic`/`_inspection_item`, que interpola a string de erro CRUA do SO — medido ao
vivo que ela diverge por runtime para a MESMA falha (`open trackfw.yaml: permission denied` em Go
vs. `EACCES: permission denied, open 'trackfw.yaml'` em Node vs. `[Errno 13] Permission denied:
'trackfw.yaml'` em Python). Para `credential_guard_mode_downgrade` — que está em
`CREDENTIAL_GUARD_ANCHORED_RULES` (Node) e cujas funções-irmãs (`validateGuardHookResolvable`,
`*_script_integrity`) usam texto FIXO desde o ML-1B/1C especificamente para byte-identidade —
corrigido para uma mensagem de texto fixo nova
(`credentialGuardModeDowngradeReadFailureMessage()`/equivalente), idêntica nos 3 runtimes, com
teste dedicado nos 3 (`TestCredentialGuardModeDowngrade_LeituraFalhaNaoENOENT_ViolationSemAbortarEComTextoFixo`
e espelhos Node/Python) que prova E a não-abortagem E o texto fixo E que não reusa a mensagem de
downgrade CONFIRMADO. Para `thirdparty_artifact_has_provenance`, o branch PRÉ-EXISTENTE (código de
antes do ML-1G) "destination file could not be read (%v)"/`({error})` já interpolava erro cru —
convenção diferente, já aceita para esta regra especificamente — então a correção nova (falha de
leitura do MANIFESTO) manteve `inspectionDiagnostic`/equivalente, consistente com o padrão já
estabelecido da própria regra, não uma divergência nova.

**Verificação de que `resolveREQFiles`/`walkADRFiles` nunca fabricam caminho fantasma (item que
motivaria falso-positivo nos sítios roteados):** lido o mecanismo completo — o fallback "join cego"
de `ResolveREQFiles` (comentário §4, hades-tf 2026-09-03) só decide qual DIRETÓRIO descer
(`addChild`), e a lista final de arquivos sempre vem de `ListMDFiles`/`filepath.WalkDir`, uma
listagem REAL do disco no momento da chamada. Nenhum caminho de REQ/ADR é construído sem
corresponder a um arquivo que existia no momento da listagem — o único erro possível num sítio
roteado é uma corrida TOCTOU genuína (arquivo apagado entre listagem e leitura) ou um erro real de
I/O, exatamente a classe que esta REQ existe para diagnosticar, não uma falha do resolvedor.
Verificado nos 3 runtimes, não só Go — Node (`listReqMdFiles` dentro de `resolveReqFiles`,
`npm/src/validator/index.js`) e Python (`_list_md_files` dentro de `resolve_req_files`,
`pypi/trackfw/validator.py`) usam o MESMO mecanismo: o "join cego" só decide qual diretório
descer, a lista final de arquivos sempre vem de uma listagem real do disco
(`fs.readdirSync`/`os.listdir`), nunca de um caminho construído sem checagem de existência.

**Gate de anti-reintrodução:** `scripts/check-raw-read-ban.sh` — bane `os.ReadFile`/
`fs.readFileSync`/`open(` fora de allowlist inline (`raw-read-allowed: <razão>`) em
`internal/validator/*.go` (não-teste), `npm/src/validator/index.js`, `pypi/trackfw/validator.py`.
Guarda de vacuidade agregada por runtime (200 linhas mínimas somadas, não por arquivo — evita falso
FAIL em utilitários pequenos como `regularfile_windows.go`). Padrão Python ancorado em POSIÇÃO DE
STATEMENT (`(with|=|return)[[:space:]]+open\(`), não em "qualquer `open(` isolado" — a primeira
versão do padrão casava PROSA de comentário/docstring (ex.: "replaces open(path, ...).read()"), e a
correção inicial foi reescrever 7 comentários arqueológicos do ML-1B/1C de `open(` para `open (` só
para escapar do regex — ao contrário: quebrar prosa para satisfazer o gate. Revertido; o padrão por
posição de statement não casa prosa por construção, sem reescrever nada. Resíduo declarado: um
`open(` aninhado sem `with`/`=`/`return` na mesma linha (ex. `foo(open(p))`) escaparia — não
exercitado neste código-base. Sem lookbehind PCRE porque o `grep` BSD do macOS não tem `-P` (medido
ao vivo: `-P` gerava erro engolido por `|| true`, gate vácuo).

**Falsificação nas duas direções, nos 3 runtimes, medida ao vivo (backup/sabotage/restore
manual, não integrado a `check-gates-falsify.sh` — escopo do ML-1E/1F, cobre paridade de
`trackfw validate`, não a superfície de código-fonte deste gate):** árvore limpa → `exit 0`;
`os.ReadFile` cru reintroduzido sem marcador (Go) → `FAIL` nomeando o arquivo:linha; idem
`fs.readFileSync` (Node) e `open(` (Python, padrão por statement — refalsificado após o fix do
padrão); arquivo Node esvaziado → `FAIL` na guarda de vacuidade (arquivo vazio E total de linhas
por runtime). Árvore restaurada, confirmada idêntica (`diff` limpo) e gate volta a `exit 0`.

**Requisito de reconciliação:** 3 testes novos, 1 por runtime (Go/Node/Python), cada um afirmando a
MESMA coisa: uma falha de leitura não-ENOENT em `trackfw.yaml` com HEAD em `mode: block` (i) não
aborta `trackfw validate`, (ii) emite o texto FIXO da família (não a string de erro do SO
interpolada), e (iii) não reusa a mensagem de downgrade CONFIRMADO — provando as 3 pernas da
autorevisão acima ao mesmo tempo. O restante do roteamento (os ~18 sítios sem correção de
mensagem) não ganhou teste novo — a prova ali é o gate `check-raw-read-ban.sh` (falsificado ao
vivo) mais a paridade das 3 suítes completas permanecendo verdes com a contagem +1 exata em cada
runtime (a mudança de contagem É o teste novo, e bate nos 3).

**Delta de `trackfw validate --json` neste repositório:** antes 0 violations / 117 warnings;
depois 0 violations / 117 warnings — delta zero, como esperado: todo artefato deste repositório já
era legível, então rotear muda ALCANÇABILIDADE (o que aconteceria com um arquivo ilegível), não o
resultado atual.

**`check-gates-falsify.sh` inteiro, rodado do início ao fim (não só grep pelo nome das
regras alteradas):** `bash scripts/check-gates-falsify.sh` completo, em background (tempo real —
não medido por `time` por causa do redirecionamento, mas na ordem dos ~13min já registrados no
vault para `make quality`). Saída: 443 linhas, 366 `OK`, **0 `FAIL`**, fecha com
`Falsification checks passed (all 181 scenarios, ...)` — a etiqueta "181" no texto do echo é
STALE (não foi atualizada quando os Cenários 186-191 foram adicionados pelo ML-1C/1E/1F), mas os
cenários novos RODARAM de fato: a última linha da saída é
`OK   [falsify/validate-parity/gbg-claude-relativo-bare-relative-path-not-detected]`, o Cenário 191
do ML-1F. Nenhum cenário pré-existente quebrou com o roteamento deste ML — confirmado por execução
completa, não por grep nos nomes das regras alteradas.

**Critérios de aceite:**
- [x] Medição de Node e Python entregue antes das mudanças, com veredito (ambos já tinham helper
      do ML-1C — trabalho era ROTEAR, não criar).
- [x] `readFileForRule`/equivalentes usando o leitor à prova de arquivo especial, nos 3 runtimes.
- [x] Sítios crus roteados nos 3 runtimes, ou razão escrita por sítio para os que não devem —
      enumerado acima.
- [x] Gate de anti-reintrodução com guarda de vacuidade — `scripts/check-raw-read-ban.sh`.
- [x] Falsificação nas duas direções, nos 3 runtimes — medida ao vivo acima.
- [x] Contagem de violations do `trackfw validate` antes e depois, delta explicado — zero, artefatos
      já legíveis.
- [x] `go build ./...` / `go vet ./...` limpos; Go 228 testes (+1) / Node 882 (+1) / Python 1652+66
      (+1) — todos verdes, +1 exato em cada runtime (o teste da autorevisão de byte-identidade,
      ver requisito de reconciliação).
- [x] `bash scripts/check-gates-falsify.sh` completo — ver evidência ao final deste ML.
- [x] `make quality` não rodado por este agente — arquiteto roda ao fim.

### ML-1H — CRLF em Windows: 6 falhas novas do ML-1G, medidas na VM real, uma causa em 2 direções
**Status:** ✅ Concluído · **Agente:** `apolo-tf`

O ML-1G roteou os sítios de leitura pelo leitor único (`_read_regular_file`), o que removeu uma
tradução implícita que só o Python tinha: `open(path, encoding="utf-8")` faz universal-newlines
(`\r\n` → `\n`) na leitura; `_read_regular_file` lê bytes crus, sem essa tradução. Medido ao vivo
na VM Windows ARM64 (Python 3.12.10) pelo arquiteto: 6 falhas novas, todas Python, nas duas
direções (uma regra para de detectar; um teste de silêncio passa a acusar).

**A medição revelou que a premissa do handoff era só metade do mecanismo.** As 6 falhas se
dividem em **duas causas distintas**, cada uma exigindo o remédio oposto:

**Classe 1 — normalizar no leitor (4 falhas): `test_adr_draft_validador_detecta_formato_canonico`,
`test_req_open_validador_detecta_formato_canonico`, `TestBlockedByDraftAdr...test_req_open_bloqueada_...`.**
Causa raiz real, isolada por leitura de código (não suposição): `_parse_blocked_adrs` compara
`line == "## Blocked by ADRs"` por **igualdade exata de linha**, sem `.strip()`. Uma REQ com CRLF
deixa um `"\r"` colado ao fim dessa linha depois de `content.split("\n")`, e o match nunca
dispara — a seção inteira é pulada em silêncio. Isso é exatamente o cenário que
ADR-2026-09-04 (D1) já decidiu que este runtime tolera ("o produtor do arquivo é frequentemente
outra pessoa, em outro SO"). Remédio: nova função `_read_text_normalized(path)` —
`_normalize_crlf(_read_regular_file(path).decode("utf-8", errors="replace"))`, reaproveitando o
`_normalize_crlf` já existente em `trackfw.integrations.renderers` (nenhum segundo normalizador,
D3). Roteados: `_read_file_for_rule`, `_parse_blocked_adrs`, `_adr_draft_status_for_rule`,
`_parse_squad_from_frontmatter`, `_latest_wip_transition_time` (log), `validate_note_orphan`
(índice do vault), `validate_credential_guard_mode_downgrade` (trackfw.yaml em disco).

**Classe 2 — NÃO normalizar; corrigir a fixture (2 falhas):
`test_script_identico_ao_template_silencio` (credential_guard + git_branch_guard),
`test_global_instalado_e_integro_silencio`.** Investigação inicial suspeitou que fosse a mesma
causa — errado. Causa raiz real: os helpers `_write()` dos 3 arquivos de teste
(`test_credential_guard_integrity.py`, `test_git_branch_guard_validator.py`) escrevem com
`open(path, "w", encoding="utf-8")` **sem `newline="\n"`** — geradores de produção já usam
`newline="\n"` explícito (`check-python-writes-lf.sh` garante isso), mas os HELPERS DE TESTE não.
No Windows, o modo texto padrão do Python também **traduz na ESCRITA**: cada `"\n"` embutido na
constante de referência (`_CREDENTIAL_GUARD_SCRIPT_REFERENCE`, só-LF) virou `"\r\n"` em disco —
a fixture escreveu um script CRLF-corrompido e alegou "idêntico ao template". A regra
`*_script_integrity` compara **byte-a-byte** contra o template — corretamente, por design: CRLF
num `.sh` gerado quebra o shebang em POSIX ("bad interpreter", motivo documentado no próprio
cabeçalho de `check-python-writes-lf.sh`). Normalizar essa comparação teria reintroduzido, dentro
desta mesma REQ, a classe de fail-open que ela existe para fechar — reportar saúde sobre um
script que na verdade não executa. Remédio: `newline="\n"` nos 3 helpers `_write()`, igualando ao
padrão de produção; a comparação byte-a-byte do produto **não foi tocada**. Guarda contra
regressão futura (alguém "resolver" isso de novo normalizando a regra): `test_script_crlf_dispara_divergencia`
(credential_guard e git_branch_guard) e `test_global_script_crlf_dispara_divergencia`, que escrevem
um script CRLF via `open(..., "wb")` (bytes crus, contorna o próprio bug do helper) e afirmam que a
regra continua acusando.

**`_write()` de `test_validator.py` (as 4 fixtures markdown da Classe 1) foi deixado intocado** —
no Windows ele agora exercita CRLF de verdade nessas fixtures, o que é um teste de regressão
gratuito para a tolerância adicionada; não havia fixture com CRLF intencional nesse arquivo para
quebrar.

**3928 (`thirdparty_artifact_has_provenance`, leitura do artefato instalado) não foi tocado —
critério principal do handoff.** Esse sítio compara `_thirdparty.checksum(installed)` contra
`installed_sha256` gravado em `.trackfw/thirdparty-provenance.json` no momento da aprovação —
integridade byte-a-byte de conteúdo de terceiro arbitrário, não comparação contra um template
canônico. Normalizar ali seria incorreto nas duas direções: (a) um artefato aprovado que tenha
CRLF legítimo passaria a divergir do checksum gravado (falso positivo de adulteração), e mais
grave, (b) um artefato de fato adulterado com uma mudança disfarçada de troca de EOL passaria a
bater com o checksum aprovado (falso negativo de segurança) — a garantia que esta regra existe
para dar. Ficou raw, sem decode nem normalização — consumidor byte-exato desta ML.

**Go e Node: verificados, não "assumidos como ok".** `Buffer.toString('utf8')` (Node) e leitura de
bytes crus (Go) nunca fizeram universal-newlines nem na leitura nem na escrita — nenhum dos dois
tinha tradução para perder com o roteamento do ML-1G. Confirmado por leitura de código, não só
inferência: `resolveAdrStatus`/`extractAdrHeaderStatus` (Go/Node) já usam `TrimSpace`/`.trim()` em
todo ponto de comparação, tolerantes a CRLF por construção; idem `parseSquadFromFrontmatter` e
`extractCredentialGuardMode`. **Uma exceção real, medida, não suposta:** `parseBlockedADRs` em Go
(`internal/validator/validator.go`) e Node (`npm/src/validator/index.js`) fazem o **mesmo**
`line == "## Blocked by ADRs"` por igualdade exata, sem trim — o mesmo defeito da Classe 1, nunca
antes exercitado nesses 2 runtimes só porque nenhum `os.WriteFile`/`fs.writeFileSync` de teste
jamais introduziu CRLF sozinho (ao contrário do `open()` texto do Python). Se eu tivesse corrigido
só o Python, uma REQ real com CRLF passaria a ser detectada em Python e continuaria escapando em
Go/Node — uma divergência de 3-CLI nova, criada por esta própria ML, contra a Regra Dura de
Paridade. Fechado nos 3: `integrations.NormalizeCRLF` (Go, já existe, sem novo import cycle —
`internal/validator` já importa `internal/integrations` via `validator_thirdparty_provenance.go`)
e `normalizeCRLF` (Node, `npm/src/integrations/render.js`, sem cycle — `render.js` só importa
`../identity`).

**Escopo do "não precisou de mudança" para Go/Node — restrito ao que foi de fato lido, não uma
varredura completa dos ~19 sítios crus de cada runtime.** Confirmado por leitura de código (não
inferência) apenas para: `resolveAdrStatus`/`extractAdrHeaderStatus` (status de ADR, ambos os
runtimes), `parseSquadFromFrontmatter` (ambos), `extractCredentialGuardMode` (ambos) e
`frontmatter_presence` (Go, `strings.HasPrefix(content, "---")` — não se importa com `\r` porque
só olha os 3 primeiros bytes). Os equivalentes Go/Node de `_latest_wip_transition_time`
(`.trackfw-log`, `validator.go:1771`) e `validate_note_orphan` (índice do vault,
`validator.go:2721`) — que EU roteei em Python nesta ML — não foram lidos/auditados em Go/Node;
não fazem parte da alegação de paridade fechada acima.

**Falsificação nas duas direções, na VM Windows ARM64 real (Python 3.12.10), não em raciocínio:**
- Estado final: os 6 testes-alvo passam (`6 passed in 0.08s`).
- Remover só a normalização do leitor (`_read_text_normalized` volta a decodificar sem
  `_normalize_crlf`): as 3 falhas de Classe 1 voltam (`0 != 1` nos 3 casos); as 2 de Classe 2
  continuam passando — prova que os dois remédios são independentes, não um disfarçado de outro.
- Restaurar o leitor e em vez disso reverter só o `newline="\n"` dos helpers de fixture: as 3
  falhas de Classe 2 voltam (`content diverges from the template` onde se esperava silêncio); as
  3 de Classe 1 continuam passando.
- Suíte completa (`pytest tests/ -q`) na VM antes e depois: mesmos **29 falhas pré-existentes,
  não relacionadas** (barrier, gitattributes, identity wizard tty, ship, thirdparty — gaps
  Windows já existentes, fora do escopo desta ML) nas duas rodadas; 1612 passed (+4 exato, os 4
  testes novos desta ML) na rodada final.

**Requisito de reconciliação (1 frase por teste novo):**
- Python `test_script_crlf_dispara_divergencia` (credential_guard, git_branch_guard) /
  `test_global_script_crlf_dispara_divergencia` — afirmam que a comparação byte-a-byte de
  `*_script_integrity` nunca faz fold de CRLF→LF: um script CRLF-corrompido continua reportado
  como divergente do template.
- Python `test_req_open_bloqueada_por_adr_proposed_dispara_com_crlf` — afirma que uma REQ com
  CRLF continua sendo detectada como bloqueada por ADR Proposed (a tolerância de leitura
  funciona ponta a ponta, não só na unidade).
- Go `TestBlockedByDraftADR_REQOpen_ProposedADR_CRLF_Violates` / `TestCredentialGuardScriptIntegrity_ScriptCRLF_Dispara`
  — espelham as duas afirmações acima em Go, provando a paridade fechada nesta ML.
- Node (`blocked_by_draft_adr: REQ com CRLF continua detectando bloqueio`,
  `credential_guard_script_integrity: script CRLF-corrompido dispara divergência`,
  `git_branch_guard_script_integrity: script CRLF-corrompido dispara divergência`) — mesmas duas
  afirmações, terceiro runtime.

**Premissa do handoff que a medição corrigiu:** "Python leu por engano graças ao universal
newlines" é só metade do mecanismo — a outra metade, achada por leitura de código, não suposição,
é que o **próprio helper de teste** também escrevia CRLF por engano no Windows (mesma tradução
implícita do Python, do lado da escrita). Tratar as 6 falhas como uma causa única teria produzido
o remédio errado para 2 delas: normalizar a comparação byte-a-byte do script_integrity, reabrindo
dentro desta REQ a classe de fail-open que ela existe para fechar.

**Critérios de aceite:**
- [x] As 6 falhas deixam de ocorrer — provado em Windows real (VM ARM64), não em macOS.
- [x] Nenhum consumidor que precise de bytes exatos foi afetado — `thirdparty_artifact_has_provenance`
      (linha ~3975 de `validator.py`) permanece raw; enumeração completa acima.
- [x] Falsificação nas duas direções, na VM real — ver evidência acima (2 remédios, 2 falsificações
      independentes).
- [x] Escrita continua LF (D2) — `check-python-writes-lf.sh` verde (após reescrever uma frase do
      docstring de `_read_text_normalized` que continha `open(path, "w", encoding="utf-8")` literal
      e disparava o gate por prosa, mesma classe de falso-positivo que o ML-1G já documentou para
      `check-raw-read-ban.sh`).
- [x] Suítes dos 3 runtimes verdes, com números: Go 230 testes (+2, `internal/validator`; `go build
      ./...`/`go vet ./...` limpos); Node 884 testes (+3, `npm test` completo); Python 1656 + 66
      subtests (+4, `pytest tests/` completo).
- [x] `check-raw-read-ban.sh` verde (nenhum novo sítio cru introduzido — `_read_text_normalized`
      chama `_read_regular_file`, já allowlisted).
- [x] `make quality` não rodado por este agente — arquiteto roda ao fim.

**Reconciliação da contagem Node (882 do ML-1G vs. 884 desta ML):** o diff desta ML introduz
exatamente 3 `test(` novos (`git diff npm/tests/*.test.js | grep "^+.*test("` — confirmado, não
contado de cabeça), então a baseline imediatamente anterior a este ML era 881, não 882. A
diferença veio do commit de merge já presente na branch antes deste ML
(`f5850b7 chore(merge): integra a main no fail-open, resolvendo 3 conflitos de append`, git log),
que trouxe trabalho independente da `main` — não investigado a fundo por estar fora do escopo
deste ML e por 881+3=884 já fechar a conta sem resto.

**`gofmt -l` pré-existente, não introduzido por este ML:** `internal/validator/validator_credential_guard.go`
e `internal/validator/validator_credential_guard_test.go` aparecem em `gofmt -l` (drift de
alinhamento de coluna em `credentialGuardHookFile` e nos comentários de um slice de casos de
teste). `git diff --stat` desses 2 arquivos contra este ML retorna vazio — nenhuma linha tocada
por mim. Sinalizado aqui para o `trackfw_architect` não atribuir a `make quality` a este ML.

**Warning "no acceptance criteria block" — pré-existente, não fechado por este ML.** `trackfw
validate` continua acusando este roadmap por não ter o heading consolidado que
ADR-2026-07-31-gerador-de-roadmap-emite-heading-consolidado-de-criterios-de-aceite exige (um único
bloco `## Acceptance Criteria` agregando todos os MLs, não os `**Critérios de aceite:**` por ML já
presentes em cada seção, inclusive na que acrescentei). Já valia antes desta ML (mesmo warning
aparecia em `trackfw validate` no início da sessão); geração desse heading é artefato do
`trackfw_architect`, fora do escopo de implementação deste ML.

## Fora desta wave, e declarado
Os **ACs 2 e 3** da REQ (o `failClosed` do Cursor e o wrapper) continuam não entregues. **Não entram
aqui** — o AC3 tem bloqueador declarado e não resolvido (o script é gerado, não vem no binário; um
clone fresco com hooks commitados antes do `init` teria toda chamada travada). Entram na wave
seguinte **da mesma REQ**, depois que esta fechar.


## Auditoria da frente — arquiteto, 2026-09-06

```
make quality QUALITY_EXIT=0, zero FAIL · 372 cenarios de falsificacao
trackfw validate exit 0
```

**Sete microlotes, cada um encontrado pelo anterior:**

| ML | o que fechou | quem achou o próximo |
|---|---|---|
| 1A | JSON inválido engolido em silêncio | barreira `hades-tf` achou o 2º motivo |
| 1B | leitura e decodificação | barreira achou FIFO + família inteira |
| 1C | FIFO (hang) e `script_integrity` | convergência falha fora da família guard |
| 1D | cenário 80 sabotando const compartilhada | 3 blocos do gate sem prova de detecção |
| 1E | falsificação dos 3 blocos novos | 6 blocos restantes sem cenário |
| 1F | os 6 restantes (8 de 9, o 9º com limite provado) | — |
| 1G | leitor único + gate de anti-reintrodução | 2 aborts vivos fora da família |

## 🔴 MEDIDO EM WINDOWS ARM64 REAL — não em CI, não em raciocínio

Binário cruzado (`GOOS=windows GOARCH=arm64`) da árvore desta frente, executado numa VM Windows
build `10.0.26200`, ARM64:

```
diretorio no lugar de .claude/settings.json
  → 2 violations nomeadas: "could not be read — ... cannot load any hook from a file it cannot read"

config gravado em UTF-16 (BOM medido: 255,254,123,0)
  → 2 violations: "is not valid UTF-8 — ... cannot decode"
```

**Antes desta frente, o primeiro caso era silêncio no Node e no Python, e abort no Go.** E a
**distinção entre as duas mensagens** — exigência que escrevi no handoff do ML-1B sem poder
verificar — está correta na plataforma onde importa.

🔴 **Achado colateral:** `windows/arm64` está excluído de `.goreleaser.yaml`, mas **compila e roda
nativamente** — `trackfw 7.3.0`, exit 0. A exclusão **não tem necessidade técnica**; é suposição não
testada. Insumo para a REQ da instalação em Windows.

## O que esta frente prova sobre método

A REQ estava `Done` **com os 4 critérios de aceite em branco**. Dois nunca foram entregues — os que
cobrem deleção e "não consegui rodar".

**Pelo meu padrão de ontem, isto teria virado sete REQs**, e a primeira ainda estaria na fila de 36 —
inclusive o `roadmap move` gravando CRLF no arquivo de REQ do usuário, e o `validate` reportando
saúde sobre config que não conseguiu ler.

A `Regra Dura de Causa Raiz` (`CLAUDE.md`) e a decisão do usuário de **reabrir em vez de abrir a 37ª**
são o que tornou isso possível.
