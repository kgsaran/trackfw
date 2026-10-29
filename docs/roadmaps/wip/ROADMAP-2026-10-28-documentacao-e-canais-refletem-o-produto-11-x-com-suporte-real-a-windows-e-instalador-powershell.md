---
status: wip
date: 2026-10-28
req: "docs/req/REQ-2026-10-28-documentacao-e-canais-refletem-o-produto-11-x-com-suporte-real-a-windows-e-instalador-powershell.md"
squad: "hades-tf, ares-tf, artemis-tf, apolo-tf, atena-tf"
---

# Roadmap: Documentação e canais refletem o produto 11.x, com suporte real a Windows e instalador PowerShell

> Created: 2026-10-28 | Status: wip

## Context
REQ: docs/req/REQ-2026-10-28-documentacao-e-canais-refletem-o-produto-11-x-com-suporte-real-a-windows-e-instalador-powershell.md
ADR: docs/adr/ADR-2026-10-28-instalador-powershell-publicado-na-release-com-o-mesmo-contrato-do-install-sh.md

A auditoria de 2026-10-28 achou 33 divergências entre a documentação de usuário e o produto
11.1.0. As altas e o risco de namespace estão na REQ. Este roadmap **absorve** o
`ROADMAP-2026-10-18-reescrita-do-cli-parity-md-para-o-contrato-de-binario-unico-pos-v8`, porque a
causa é a mesma: documentação anterior à v8. Aquele foi movido para `abandoned/` com ponteiro para
este.

**Dependências reais e motivo da ordem:**
- README, VISION e site documentam o `install.ps1`, a matriz de Windows e a linha de hook. Por
  isso a Wave 3 espera as Waves 1 e 2.
- Os MLs de cada wave tocam arquivos disjuntos. Mesmo assim, o teto é de **2 agentes
  simultâneos**, por custo de CPU.
- O `Makefile` só é tocado pelo ML-1B e o `.github/workflows/quality.yml` só pelo ML-1A.

## Acceptance Criteria
Os AC1–AC10 da REQ. O mapeamento por ML está em cada microlote.

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model (hades-tf)
**Status:** ✅ Concluído — auditado em 2026-10-28, após dois corretivos: fixture sem porta de redirecionamento; consumidores de npm/pypi; namespace fechado
**Files affected:** `docs/seguranca/2026-10-28-wave0-docs-e-install-ps1.md` (único arquivo escrito pelo hades-tf), `docs/roadmaps/backlog/ROADMAP-2026-10-18-reescrita-do-cli-parity-md-para-o-contrato-de-binario-unico-pos-v8.md`, `docs/roadmaps/abandoned/ROADMAP-2026-10-18-reescrita-do-cli-parity-md-para-o-contrato-de-binario-unico-pos-v8.md` (absorção movida pelo arquiteto no commit de governança, antes da Wave 0)
**Actions:**
1. **Completude da enumeração.** Varrer o repositório inteiro, não só os arquivos da REQ:
   - superfícies onde a documentação instrui instalação ou cola linha de hook:
     `git grep -nE 'brew install|go install|pip install|npm install -g|irm |iex|LASTEXITCODE'`;
   - leitores mecânicos de `docs/cli-parity.md` e do README;
   - consumidores de `pypi/pyproject.toml`, `npm/README.md` e `pypi/README.md`, antes de removê-los.
2. **Modelo de ameaça do `install.ps1`:**
   - execução por `irm | iex`;
   - checksum: ausente, duplicado, divergente, ou `checksums.txt` de outra tag;
   - `TRACKFW_VERSION` e `TRACKFW_ARCH` com quebra de linha, `;`, `$()` ou caminho;
   - `TRACKFW_INSTALL_DIR` com junção ou link;
   - corrida em `%TEMP%`;
   - extração por `tar.exe` com entrada `..` ou absoluta;
   - alteração do PATH do usuário: duplicação, entrada vazia, `REG_EXPAND_SZ` perdido ao regravar
     como `REG_SZ`;
   - processo x64 emulado em ARM64;
   - **decidir como o gate injeta fixture** (tarball e checksums locais) sem abrir no script de
     produção uma porta que redirecione o download.
3. **Namespace:** confirmar com `gh api` quais namespaces citados na documentação não são nossos
   (`trackfw`, `kgsaran/homebrew-tap`) e qual o efeito de alguém registrá-los.
4. **Alvos de falsificação nas duas direções, por superfície.** Exemplos:
   - o gate da linha de hook do README falha se o README voltar à forma antiga **e** se o
     `guardforms` mudar sem o README;
   - o gate do METADATA falha com corpo vazio **e** com Content-Type ausente.
5. **Residual declarado.**

**Acceptance criteria:**
- [x] As cinco seções respondidas com evidência (comando e resultado), não com afirmação de uma linha
- [x] Nenhuma linha de implementação escrita (um arquivo solto `-ErrorAction`, resto de experimento, foi tirado da raiz pelo arquiteto)

**Gates da wave:**
```bash
test -s docs/seguranca/2026-10-28-wave0-docs-e-install-ps1.md
```

## Wave 1 — Canais (2 MLs em paralelo)
> Dependencies: Wave 0 auditada.

### ML-1A — `install.ps1` + gate Windows (ares-tf) — AC4
**Status:** ✅ Concluído — auditado em 2026-10-29. CI `windows-latest` no PR #627, run https://github.com/kgsaran/trackfw/actions/runs/40846363727 (job `windows-install-ps1`): 30 PASS e 0 SKIP sob `powershell` (Edition Desktop) e 30 PASS e 0 SKIP sob `pwsh` (Edition Core). São os 31 cenários locais menos `traversal-counter-arm-macos`, que só se aplica fora do Windows. Gate local rc=0 (14 PASS, 17 SKIP nomeados, pwsh 7.6.9 no macOS); build ok. Residual aceito: o membro absoluto não tem contra-braço com `-P`, porque escrever na raiz do drive no CI é inseguro; a asserção de ausência continua valendo.
**Files affected:**
- `scripts/install.ps1` (novo)
- `scripts/check-install-ps1.ps1` (novo)
- `.goreleaser.yaml`: `extra_files` ganha `- glob: scripts/install.ps1`
- `.github/workflows/quality.yml`: o job Windows roda o gate sob `powershell` e `pwsh`

**Actions:**
1. Implementar o contrato da ADR-2026-10-28 cláusula por cláusula, espelhando o
   `scripts/install.sh`:
   - checksum conferido antes de extrair;
   - versão e arquitetura validadas no valor inteiro;
   - destino `$env:LOCALAPPDATA\Programs\trackfw`;
   - PATH do usuário idempotente, preservando `REG_EXPAND_SZ`;
   - `TRACKFW_NO_MODIFY_PATH=1`;
   - aviso de outras cópias.
   **Prescrições da Wave 0** (`docs/seguranca/2026-10-28-wave0-docs-e-install-ps1.md` §2):
   - o download usa `Invoke-WebRequest` **não qualificado**;
   - **nenhuma** variável de ambiente ou parâmetro muda a origem do download (proibidos
     `TRACKFW_BASE_URL`, `TRACKFW_MIRROR`, `TRACKFW_INSTALL_DRYRUN`);
   - versão validada com
     `-cmatch '\Av?[0-9]+\.[0-9]+\.[0-9]+(-rc[0-9]{1,3}|-beta[0-9]{1,3})?\z'`, antes de qualquer
     download;
   - o PATH do usuário é regravado preservando `REG_EXPAND_SZ` (não usar
     `[Environment]::SetEnvironmentVariable`);
   - diretório temporário novo e aleatório, criado com `New-Item -ErrorAction Stop`;
   - fixture binária escrita com `[IO.File]::WriteAllBytes`.
2. Montar o gate com a **função sombra `Invoke-WebRequest`** no escopo do gate, decidida no ML-0A.
   É o análogo do stub de `curl`/`wget` no PATH usado pelo `check-install-checksum.sh`. O gate:
   - roda primeiro uma verificação estática da lista de banidos (§2 do parecer);
   - afirma que a sombra **foi chamada**, para que um bypass reprove em vez de ir à rede;
   - executa o modo `iex` em processo filho, com a sombra redefinida dentro dele.

   Cenários, cada um com contra-braço:
   - íntegro → instala;
   - adulterado, ausente ou duplicado → sem binário no destino (afirmar pela ausência do arquivo,
     não pelo exit);
   - versão com `\n` ou `;` → rejeitada **e** zero chamadas de download;
   - tarball com membro `..` ou absoluto → nada escrito fora do destino;
   - arquitetura inválida → rejeitada;
   - PATH que já contém o destino → não duplica;
   - `TRACKFW_NO_MODIFY_PATH=1` → PATH intocado.
3. Rodar o gate localmente com `pwsh` (o macOS tem `/opt/homebrew/bin/pwsh`) nos cenários que não
   dependem do SO. Os cenários de registro e de PATH do Windows rodam só no CI.

**Acceptance criteria:**
- [x] Gate verde no CI `windows-latest` sob `powershell` 5.1 **e** sob `pwsh` 7 (run 40846363727, PR #627)
- [x] Cada cenário falha quando o comportamento é removido do `install.ps1` (sabotagem medida e revertida)
- [x] `go build ./...` verde
- [x] Relatório com uma frase por teste novo, dizendo que conclusão do ML ele afirma

### ML-1B — Página do PyPI e cópias mortas (ares-tf) — AC3
**Status:** ✅ Concluído — auditado em 2026-10-28: wheel construída pelo arquiteto, METADATA com Content-Type e corpo byte-igual ao README; gate rc=0 com 5 braços. Desvio aceito: o default do `--readme` é o README da raiz (via `__file__`), não `pypi/README.md` — conteúdo idêntico, e dois gates chamam sem `--readme`. A página do PyPI só muda na próxima release.
**Files affected:**
- `pypi/scripts/build_wheel.py`: grava `Description-Content-Type: text/markdown` e o README como
  corpo do METADATA
- `scripts/check-wheel-metadata-readme.sh` (novo)
- `Makefile`: registra o gate no `quality`
- `npm/README.md` e `pypi/README.md`: removidos do git, porque a Wave 0 mediu que o release
  sobrescreve os dois (`release.yml:237` e `:309`)
- `.gitignore`: as duas cópias geradas no release não podem ser commitadas
- **`pypi/pyproject.toml` fica.** A Wave 0 mediu consumidores vivos: `internal/commands/release.go`
  (`extractPyprojectVersion`), `scripts/check-manifest-version-gate.sh`,
  `scripts/check-integration-assets.sh` e `scripts/check-gates-falsify.sh`

**Actions:**
1. Ler `.github/workflows/release.yml:230-347`. O release copia o `README.md` para `npm/` e
   `pypi/`. Confirmar que o `build_wheel.py` lê o `pypi/README.md` copiado e falha com mensagem
   quando ele não existe.
2. Gate: copia o README, constrói uma wheel `linux amd64` com um binário qualquer e afirma no
   METADATA o Content-Type e um corpo **igual** ao README. Contra-braço: a wheel sem README
   reprova.
3. Rodar `make parity-rest`, porque o ML cria script novo.

**Acceptance criteria:**
- [x] O gate falha com o README ausente e com o Content-Type removido (sabotagem medida)
- [x] `make parity-rest` verde
- [x] Relatório com uma frase por teste novo

**Gates da wave:**
```bash
test -f scripts/install.ps1
grep -q 'scripts/install.ps1' .goreleaser.yaml
sh scripts/check-wheel-metadata-readme.sh
go build ./...
```

## Wave 2 — Medição e contrato (2 MLs em paralelo)
> Dependencies: Wave 1 auditada. O motivo é o teto de CPU; não há dependência de arquivo.

### ML-2A — Linha de hook sob `pwsh` 7 e `powershell` 5.1 (artemis-tf) — AC5
**Status:** ✅ Concluído — auditado em 2026-10-29. No CI `windows-latest` (run 40882950230, job `windows-full-suites`), os 40 subtestes `TestGuardLineShellMatrix/<forma>/{pwsh,powershell}/{RC0-allow,RC2-deny,RC1-deny-normalizado,absent}` passaram nas 4 formas, sob PowerShell 5.1 e 7. As 5 falhas do job estão em `.github/windows-known-failures.json`. No macOS, `pwsh` passou e `powershell` pulou com motivo. Sabotagem: sem o envelope, deny, erro e ausente saem com exit 1. Observação: sob PowerShell, o stderr do deny traz dois blocos `CommandNotFound` junto com o motivo. O motivo é preservado. Fica registrado para a matriz do README (ML-3A).
**Files affected:** `internal/generators/guard_line_shell_test.go`

**Actions:**
1. Estender o `TestGuardLineShellMatrix` com braços `pwsh` e `powershell`. Eles ficam ativos
   quando o executável existe (`exec.LookPath`). Quando não existe, o braço é reportado como
   `t.Skip` com motivo, nunca como PASS silencioso.
2. Cobrir os casos allow, deny, erro e ausente, com o payload em stdin. Afirmar o exit (0/2/2/2) e
   a preservação do motivo do deny.
3. Rodar com um PATH sem `trackfw` instalado, para não mascarar a sonda.

**Acceptance criteria:**
- [x] Os braços rodam (não pulam) no job Windows do CI: nome do teste e linha do log citados
- [x] Localmente, o braço `pwsh` roda no macOS
- [x] Relatório com uma frase por teste novo; nome passado ao `go test -run` conferido com `-v`

### ML-2B — Reescrita do `docs/cli-parity.md` (apolo-tf) — AC7
**Status:** ✅ Concluído — auditado em 2026-10-29, depois de dois corretivos. O primeiro cuidou das 17 anotações, que agora descrevem o que cada gate prova hoje, sem afirmar mais. O segundo cuidou da prosa: cerca de 108 linhas com paridade no presente, que eram contrato, foram reescritas com o binário como sujeito ou marcadas pré-v8; as linhas sobre CLIs de agente ficaram intactas. Medido pelo arquiteto: `check-parity-contract-coverage.sh` rc=0, com contadores idênticos (324/74/72/53/125/0/0). Rodaram com rc=0: `check-barrier`, `check-agent-namespace-union`, `check-agent-models-parity`, `check-ref-separator-portability`, `check-output-encoding-declared`, `check-roadmap-barrier-contract`, `check-usage-silencing` e `check-pr-closing-keyword --self-test`. Cabeçalhos: 341 antes e depois, 0 removidos, 11 renomeados; nenhum leitor depende do texto deles. O censo final deixa 9 linhas, todas no passado, em continuação de parêntese pré-v8 ou sobre CLIs de agente.
**Files affected:** `docs/cli-parity.md` (único)

**Actions:**
1. Inventariar os leitores mecânicos:
   `grep -rln 'cli-parity.md' internal scripts Makefile .github`. Listar as anotações
   `gate=`/`partial=` e as strings fixadas que esses leitores consomem. Elas ficam intactas, byte a
   byte.
2. Classificar cada ocorrência de `three runtimes|all three|Node\.js|Python`:
   - contrato de comportamento → reescrever como "o binário …";
   - canal → mover para a seção de canais;
   - histórico → passado, marcado como tal.
3. Fazer a tabela "Command contract" bater com `internal/commands/root.go:54-86`:
   - incluir `release`, `barrier`, `commit`, `doctor`, `audit-surface`, `guard` e `completion`;
   - remover os aliases `gemini`, `cursor`, `copilot`, `windsurf` e `amazonq` (l.84).
4. Preservar a seção `### `ml_scope` check` como está. Ela foi reescrita pela REQ-2026-10-29 (ML-1A) e é o contrato
   novo. Se o exemplo de `failures` do `### JSON document` citar `ml_scope`, alinhe-o à mensagem nova:
   `<roadmap> <ML-id>: rejected: pattern "<tok>" without literal path segments ...`. Tire as referências
   `barrier.go:<linha>` da linha `ml_scope` da tabela: número de linha num contrato fica desatualizado no próximo commit.
5. Corrigir o `brew install trackfw` (l.28) para `brew install kgsaran/trackfw/trackfw`. Atualizar
   "Platforms (v8.0.0-rc1)" (l.32-39) com o estado atual do CI.

**Acceptance criteria:**
- [x] Os testes Go do inventário (os que leem o arquivo) verdes, cada um rodado por nome
- [x] Nenhuma regra de contrato perdida: diff dos cabeçalhos `^#{2,4} ` antes e depois anexado ao relatório
- [x] Nenhuma afirmação de paridade entre runtimes no presente

**Gates da wave:**
```bash
go test ./internal/generators/ -run TestGuardLineShellMatrix
go build ./...
```

## Wave 3 — Documentação de usuário (2 MLs em paralelo)
> Dependencies: Waves 1 e 2 auditadas. Elas produzem o `install.ps1`, a matriz medida e o contrato.

### ML-3A — README, CONTRIBUTING e gate-design-principles (apolo-tf) — AC1, AC2, AC6
**Status:** ✅ Concluído — auditado em 2026-10-29, depois de dois corretivos e um ajuste do arquiteto. (1) A matriz Windows afirmava ✅ em npm/pip x64 sem medição e ⚠️ em `install.ps1` x64 e `pwsh` 7, que o CI deste PR mediu. Agora toda célula ✅ cita um arquivo de `docs/portabilidade/` ou uma run de CI, e a inversão antiga do cmd.exe (medido em ARM64, não em x64) foi desfeita. (2) "ten" virou "twelve" nos presets, mas são 10 presets (`presetOrder`, `preset.go:176`) com 12 especialidades (`KnownAgentIDs`). O modo personalizado pede 12 nomes, e o arquiteto corrigiu a linha que ainda dizia "ten". (3) Comentário de `guard_generations.go:40` alinhado ao que foi medido, sem mudar constante. Gates: `TestREADMEHookLinesMatchCurrentConstants` PASS, com as sabotagens forma antiga, constante alterada e vácuo reprovando; `go test ./internal/guardforms/` rc=0. As células ⚠️ do `install.sh`/npm/pip ARM64 ganham fonte no ML-4A.
**Files affected:** `README.md`, `CONTRIBUTING.md`, `docs/gate-design-principles.md`,
`internal/guardforms/readme_line_test.go` (novo),
`internal/guardforms/guard_generations.go` (só comentário; acrescentado na auditoria de 2026-10-29: o comentário contradizia a medição citada no README)

**Actions:**
1. Corrigir as divergências do README listadas na auditoria (anexada ao handoff):
   - tap do Homebrew;
   - linha de hook com `emulate`;
   - texto de fail-closed do binário antigo (l.262-274);
   - "next release" (l.27-28, 124-125, 187);
   - OpenCode na lista de assistentes;
   - `vault-skill` → `architecture-skill`;
   - 12 agentes;
   - tabela de comandos completa;
   - credential guard atual;
   - shim resolvido na execução.
2. Nova seção **Windows**:
   - matriz rota × arquitetura × shell, com cada célula marcada "medido (fonte)" ou "não
     verificado", alimentada pelos relatórios de 1A, 2A e da auditoria;
   - o `install.ps1`;
   - as limitações vigentes: cmd.exe com a linha PS/POSIX, `sh` exigido pelo `barrier`, NTFS e
     0600, as 5 falhas conhecidas, o `~/bin` do Git Bash à frente no PATH;
   - as variáveis do `install.sh`.
3. `readme_line_test.go`: extrai do README as linhas de hook documentadas e afirma que são iguais
   às constantes de `guardforms`. O teste precisa falhar nas duas direções.
4. CONTRIBUTING: build no Windows (o `bin/trackfw` sai sem `.exe`; ver a nota do vault de
   2026-10-16). gate-design-principles: remover os gates de paridade removidos na v8 (l.17-18,
   42, 165-167, 178) ou marcá-los como históricos.

**Acceptance criteria:**
- [x] `readme_line_test.go` falha com o README antigo e com a constante alterada (sabotagem medida e revertida)
- [x] Nenhum comando de instalação no README sem execução ou conferência contra o registro (lista anexada)
- [x] Relatório com uma frase por teste novo

### ML-3B — VISION e site (atena-tf) — AC1, AC8
**Status:** ✅ Concluído — auditado em 2026-10-29. Build do site com `cd site && npm ci && npm run build` (o comando do `deploy-docs.yml`) rc=0. Verificação visual feita pelo arquiteto no Chrome headless (`--screenshot`) sobre o `vitepress preview`, nas páginas de getting-started em pt e en. Conferidos contra o binário: os 10 alvos do `--ai-tools` (`init.go:50`) e `release tag`. Corretivo: o logo nunca existiu (`/trackfw/logo.svg` dava 404, imagem quebrada no cabeçalho do site publicado desde a criação). A chave `logo` foi removida; criar uma marca fica com o KG. O plugin de gerador não existe no código, e o VISION agora o marca como intenção.
**Files affected:** `docs/visao-projeto/VISION.md`, `site/**` (pt e en)

**Actions:**
1. VISION:
   - versão e "Current State" atualizados para a 11.x;
   - tabela Distribution com um binário em três canais, o tap correto e o módulo
     `github.com/kgsaran/trackfw`;
   - vNext com a direção consolidação + adoção (#525);
   - "What trackfw is NOT" passa a incluir gestão de times;
   - o modelo de plugin de gerador só pode ser afirmado se existir no código; se não existir, vira
     intenção marcada como tal.
2. Site:
   - Go ≥ 1.25;
   - saída `trackfw 11.1.0`;
   - instalação com pip, `install.sh` e `install.ps1`;
   - dez alvos de agente;
   - página de comandos com `guard`, `attention`, `doctor`, `push`, `branch prune`, `release`,
     `audit-surface` e `agents models`;
   - seção Windows resumida, apontando para o README.
3. Conferir o build do site com o comando de `.github/workflows/deploy-docs.yml`.

**Acceptance criteria:**
- [x] Build do site verde, com o comando exato citado
- [x] Verificação visual: a página de getting-started servida e lida no navegador (screenshot no relatório)

**Gates da wave:**
```bash
go test ./internal/guardforms/ -run TestREADME
sh -c "git grep -nE 'github\.com/trackfw/|trackfw/tap|kgsaran/tap' -- README.md CONTRIBUTING.md docs/visao-projeto docs/cli-parity.md docs/gate-design-principles.md site; test \$? -eq 1"
```

## Wave 4 — Medição real e red team (2 MLs em paralelo)
> Dependencies: Wave 3 auditada.

### ML-4A — `install.ps1` em VM Windows 11 ARM64 (ares-tf) — AC9
**Status:** ✅ Concluído — auditado em 2026-10-29. `docs/portabilidade/2026-10-28-install-ps1-vm-arm64.md`, 9/14 chamadas. Na VM Windows 11 ARM64, contra a v11.1.0 real, foram medidos: `install.ps1` sob PS 5.1, `install.sh` sob Git Bash, npm e pip a partir do registro. A VM terminou com o PATH do usuário igual ao inicial. A medição achou a lacuna do aviso do `~/bin`, corrigida no ML-4C.
**Files affected:** `docs/portabilidade/2026-10-28-install-ps1-vm-arm64.md` (novo, único)

**Actions:** rodar o `install.ps1` da branch na VM ARM64, sob Windows PowerShell 5.1, contra a
release real (`TRACKFW_VERSION=v11.1.0`). Registrar:
- a arquitetura detectada;
- o binário instalado e a saída de `trackfw version`;
- o PATH do usuário antes e depois, com o tipo do valor no registro;
- a reexecução idempotente;
- o efeito de `TRACKFW_NO_MODIFY_PATH=1`;
- o aviso de outra cópia no `~/bin` do Git Bash.

Acrescentado na auditoria do ML-3A (2026-10-29): para que as células da matriz Windows do README tenham fonte citável em
`docs/portabilidade/`, o mesmo documento registra na VM ARM64 o `install.sh` sob Git Bash e o `npm install -g trackfw`
e o `pip install trackfw` a partir do registro, todos na 11.1.0. Hoje a única evidência do `install.sh` ARM64 é o
working context congelado. Depois do ML-4A, o arquiteto atualiza as células do README em linha.

O handoff traz lista fechada de ações e contador N/M de chamadas.

**Acceptance criteria:**
- [ ] Cada item com comando e resultado literal; rc lido sem pipe

### ML-4B — Red team (hades-tf)
**Status:** ✅ Concluído — o parecer reprovou (F-1: `;` no destino quebrava a idempotência do PATH), os corretivos ML-4C e ML-4D foram feitos, e a reauditoria aprovou com ressalvas. Os residuais A-1, A-2 e A-4 foram corrigidos no ML-4E. O A-3 fica aceito, com o motivo escrito no ML-4E.
**Files affected:** `docs/seguranca/2026-10-28-redteam-docs-e-install-ps1.md` (único)

**Actions:** reimplementar, a partir da leitura da ADR e da REQ, os ataques do ML-0A contra o
`install.ps1` entregue e contra o gate do README. Todo residual vira ML neste roadmap, não REQ nova.

**Acceptance criteria:**
- [x] Parecer aprovado, ou corretivo despachado e reauditado

### ML-4C — Corretivo do install.ps1: `;` no destino, `;;` no PATH e aviso do `~/bin` (ares-tf)
**Status:** ✅ Concluído — auditado em 2026-10-29, depois de um corretivo no gate. O `sab-trailing-semicolon` restaurava o registro antes de ler o PATH; a ordem foi corrigida, e os outros cenários de registro foram varridos e estavam certos. CI `windows-latest`, run 40918775429, job `windows-install-ps1`: 39 PASS, 0 FAIL e 0 SKIP sob Desktop (5.1) e sob Core (7), incluindo os 9 cenários novos (`install-dir-semicolon`, `install-dir-semicolon-no-dir`, `sab-install-dir-semicolon`, `path-trailing-semicolon`, `path-trailing-semicolon-idempotent`, `sab-trailing-semicolon`, `gitbash-bin-warning`, `gitbash-bin-no-warning`, `sab-gitbash-bin-warning`).
**Files affected:** `scripts/install.ps1`, `scripts/check-install-ps1.ps1`, `README.md`
**Actions:** achados do red team (`docs/seguranca/2026-10-28-redteam-docs-e-install-ps1.md`) e da VM
(`docs/portabilidade/2026-10-28-install-ps1-vm-arm64.md`):
1. F-1: `TRACKFW_INSTALL_DIR` com `;` é rejeitado antes de qualquer escrita. Hoje `C:\tf;%TEMP%` passa e a reexecução
   duplica o PATH.
2. F-2: o PATH existente terminado em `;` não gera `;;`.
3. Medido na VM: o aviso de outra cópia não cobre o `~/bin` do Git Bash (`%USERPROFILE%\bin\trackfw.exe`), que fica
   fora do PATH do Windows. O instalador passa a avisar quando esse arquivo existe, sem executá-lo.
4. Comentários "NAO MEDIDO" de arquitetura que a VM mediu (`PROCESSOR_ARCHITEW6432` vazia em ARM64 nativo;
   `OSArchitecture` disponível no PS 5.1) passam a citar a medição.
5. README: a limitação do `~/bin` descreve o aviso novo.
**Acceptance criteria:**
- [x] Cenários novos no gate (`;` no destino, PATH com `;` final, aviso do `~/bin`), cada um com sabotagem medida e revertida
- [x] Gate local verde e job `windows-install-ps1` verde no CI sob PS 5.1 e 7 (run citada)

### ML-4D — Corretivo do gate da linha de hook: cerca e `cli-parity.md` (artemis-tf)
**Status:** ✅ Concluído — auditado em 2026-10-29. Cerca no estilo CommonMark (mesmo caractere, comprimento ≥ ao da abertura, nada depois); `TestREADMEFenceExtractor` com 4 vetores, entre eles o B8b. `TestCLIParityHookLines` confere as 3 formas PS/POSIX do `cli-parity.md` (l.6360, 6362 e 9225) e reprova uma forma legada fora de linha marcada como histórica. Sabotagem: o toggle antigo reprova os 4 vetores. Limitação declarada: formas cmd.exe são byte-iguais a menções do subcomando e ficam fora; o critério de "histórico" é por linha. `go test ./internal/guardforms/` rc=0 (arquiteto).
**Files affected:** `internal/guardforms/readme_line_test.go`
**Actions:**
1. F-3: a cerca só fecha com o mesmo caractere e com comprimento ≥ ao da abertura. Hoje um `~~~` dentro de um bloco ```
   cega o extrator.
2. F-4: as formas PS/POSIX citadas em `docs/cli-parity.md` também são conferidas contra as constantes.
**Acceptance criteria:**
- [x] Vetor B8b do red team reprova; sabotagem de uma forma antiga em `cli-parity.md` reprova; os dois revertidos

### ML-4E — Residuais da reauditoria do install.ps1: `;` Unicode, `Join-Path` e caixa invariante (ares-tf)
**Status:** ✅ Concluído — auditado em 2026-10-29. CI `windows-latest`, run 40931646835, job `windows-install-ps1`: 52 PASS, 0 FAIL e 0 SKIP sob PS 5.1 e sob 7, incluindo os 13 cenários novos (`install-dir-semicolon-unicode-{037E,FE54,FF1B}`, `unc-userprofile*`, `trtr-*` e as sabotagens). Os dois arquivos continuam ASCII (`LC_ALL=C grep -c '[^ -~\t]'` = 0).
**Files affected:** `scripts/install.ps1`, `scripts/check-install-ps1.ps1`, `vault/notes/index.md`
**Actions:** achados A-1, A-2 e A-4 da seção "Reauditoria (2026-10-29)" de `docs/seguranca/2026-10-28-redteam-docs-e-install-ps1.md`.
O A-3 (formas cmd.exe do `cli-parity.md` fora do gate) fica como residual, com o motivo escrito: a forma cmd.exe é
byte-igual a uma menção do subcomando, então o gate não consegue distinguir as duas.
**Acceptance criteria:**
- [x] Cada achado com cenário no gate e sabotagem medida e revertida
- [x] Job `windows-install-ps1` verde no CI sob PS 5.1 e 7 (run citada)

**Gates da wave:**
```bash
test -s docs/portabilidade/2026-10-28-install-ps1-vm-arm64.md
test -s docs/seguranca/2026-10-28-redteam-docs-e-install-ps1.md
trackfw validate
```

## Fechamento
- `make quality` com a máquina ociosa, rodado pelo arquiteto.
- PR com `Closes` para as issues que houver.
- O roadmap só vai para `done/` depois do merge.
