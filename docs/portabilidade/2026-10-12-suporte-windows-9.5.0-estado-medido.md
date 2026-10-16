# Suporte a Windows no trackfw 9.5.0 — estado medido

> 2026-10-12 · zeus-tf · consolida `2026-10-12-censo-windows-x64-9.5.0.md` (ares-tf, CI x64) e
> `2026-10-12-medicao-vm-windows-9.5.0.md` (artemis-tf, VM ARM64), com recontagens do arquiteto que
> **corrigem** números dos dois relatórios (ver "Correções").

## Como ler

- **VM investiga, CI mede.** Número que vira afirmação sai do `windows-latest` x64. A VM é ARM64.
- Estados: **vivo** · **fechado** · **sumiu com a v8** · **não medido** (com o que falta).

## 🔴 O que o "verde" do CI de Windows significa

`windows-full-suites` roda com `continue-on-error` (allowlist em `check-job-annotations`) e um
**ratchet de nomes** (`.github/windows-known-failures.json`, `scripts/check-windows-known-failures.py`):
o job reprova só se aparecer falha **nova**. No run `38316881566` (`main`, 9.5.0) **14 testes Go
falham** — exatamente os 14 da lista. Verde = "nenhuma falha nova", **não** "zero falhas". As causas
na lista foram escritas em 2026-09-10, e **5 delas são inferidas** (`INFER`), não medidas.

## Correções aos relatórios dos agentes

- **Censo, contagem estrita** (rótulo no início da linha, depois do prefixo de job e timestamp; mesma
  regra nos dois logs): base `36036473391` (24/09) = 306 rótulos, 11 FAIL; hoje `38319939952`
  (`a1cc6e93`) = 303 rótulos, **4 FAIL**. Sumiram **16**, novos **13** — não 42: a primeira contagem
  usou grep sem âncora e contou texto aninhado de `PROOF` (`go-behavioral-pin`) como rótulo.
- Os 16 que sumiram: `update-harness/*` (6) e `skip-parity/*` (2) — scripts apagados; `setup-sXX`
  (5) — estavam FAIL na base; `structural-marker-value/go/roadmap-prose-*` (2) — renomeados para
  `roadmap-prose-md-*` (aparecem entre os novos); `e2e/init-outdated-global`, `update-project/json` —
  **causa não verificada**.
- **Os 4 FAIL do censo de hoje não são regressão do produto.** Três estavam OK em 24/09, mas a causa é
  do **instrumento**: o censo executa `python -m pip install pypi/` (`windows-census.yml:399`, passo
  de antes da v8), que põe um `trackfw.exe` sem o subcomando `guard` no PATH; desde a 9.3.0 o
  `validate` acusa isso ("does not have the guard subcommand — the hook will fail open") e o ciclo
  limpo sai com 1. Afeta `kiro-dedicated-file/baseline` e `roadmap-req-frontmatter-path/go/from-req-baseline`.
  Os outros dois são o NTFS sem ACL (#421).
- **"28 FAIL na VM = artefato"**: confirmado para as famílias `TestADRPrefixE2E_` (5), `TestBarrierContract_`
  (12), `TestBarrierFragment_` (9) e `TestBranchStateE2E_` (10), todas PASS no x64.
  `TestShip_Integration_GracefulDegradation_RealBinary` **falha também no x64** (está na lista conhecida).

## Tabela

| # | Item | Estado na 9.5.0 | Evidência | Segurança? | Próximo passo |
|---|---|---|---|---|---|
| 1 | Guard global gravado/auditado sob `$HOME` quando o agente lê `%USERPROFILE%` (REQ-2026-09-01) | **vivo — confirmado em 2026-10-14** | VM, Claude Code 2.1.299 nativo: com `HOME`≠`USERPROFILE`, `update harness` grava em `HOME\.claude\settings.json` (`homedir.Dir()`, `HOME` primeiro), o `validate` audita o mesmo lugar e não acusa nada, e o `claude --debug-file` lê `C:\Users\Lab\.claude\settings.json` — nos três terminais (Git Bash, PowerShell, cmd). Falso positivo de saúde. No padrão da VM `HOME`=`USERPROFILE` e não diverge. `docs/portabilidade/2026-10-14-guard-global-home-userprofile-vm.md` | 🔴 sim | Implementar AC3/AC4 da REQ-2026-09-01; número de CI (AC2) pelo `windows-probe.yml` |
| 2 | Proveniência de terceiros no Windows (`thirdparty_artifact_has_provenance`) | **vivo** | x64: install aprovado **acusa violação** (`integrations_thirdparty_validate_test.go:74`); no caso adulterado a violação esperada (D2 ramo ii) **não** aparece (`:132`). Causa listada como `INFER` (nome curto 8.3) | 🔴 sim — a regra não discrimina íntegro de adulterado no Windows (falha fechada no íntegro) | Medir a causa; corrigir |
| 3 | ML-R3 — guard de entrada cego a `\` (`internal/integrations/manager.go:756`) | **vivo, contido** | VM: `..\outside.md` passa o guard de entrada e é barrado por `beneath()`. macOS: vira arquivo chamado `..\outside.md` dentro da raiz (não é travessia) | 🔴 sim (defesa em uma camada) | ML com teste que reprova sem Windows |
| 4 | Permissão 0600 ignorada no NTFS (`TestSave_WritesAtomicallyWithPermissions`, `internal/identity`) | **vivo** | x64, lista conhecida (grupo A, medido) | possível — arquivo de identidade fica 0666 | Decidir: ACL do Windows ou declarar |
| 5 | Demais 11 falhas conhecidas do x64 | **vivo** | lista: NTFS sem ACL (3), CRLF no renderizador (2 + 2 `INFER`), symlink sem privilégio (1), `shasum` com `\` (1), fake `git.exe` vs defesa do runner (1), separador no manifesto do `update` (1, `INFER`) | não | Remedir as `INFER`; corrigir as de produto (CRLF, separador) |
| 6 | Censo: instrumento instala o pacote Python pré-v8 | **vivo, no instrumento** | `windows-census.yml:399`; 2 FAIL de hoje | não | Corrigir o passo de setup do censo |
| 7 | NTFS sem ACL de execução (#421) no censo | **vivo** (2 rótulos) | censo | não | Já tem issue |
| 8 | ML-R2b2 — 76 rótulos sem triagem | **fechado em grande parte** | 4 FAIL hoje, todos com causa (itens 6 e 7) | — | Reclassificar no roadmap bloqueado |
| 9 | ML-R2b1b — `{owner}/{repo}` some no shim do `gh` | **não medido** | cenários em blocos PROOF ou com `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1` | não | Sonda dedicada, se ainda houver consumidor |
| 10 | Prazo de rede: `ssh` neto órfão no estouro (RS-2) | **vivo, declarado** | VM (Git Bash): retorna em ~10 s com `git fetch timed out after 5s (TRACKFW_NETWORK_TIMEOUT)`; o `ssh` falso segue vivo | não | Resíduo aceito no ADR; PowerShell não medido |
| 11 | SKIPs no `windows-full-suites` | **20** | censo §5 | — | Dois protegem critério de aceite: `TestGitExec_AC2_Bite_NoWaitDelay` e `TestGitExec_WaitDelay_ExitZeroWithHangingGrandchild` |
| 12 | Instalação (REQ-2026-09-05) | **em grande parte fechado; REQ desatualizada** | `install.sh` aceita MINGW/MSYS/Cygwin em amd64/arm64; README "Windows support (partial)"; Claude Code e Codex provados ponta a ponta (2026-10-07) | não | Reescrever a REQ com o que sobra (13–14) |
| 13 | Hooks ponta a ponta com agente real: Cursor, Windsurf, Gemini, Kiro, Copilot, Amazon Q | **não medido** | README: linha do hook medida no shell; agente não rodado | sim (guard) | Cada CLI instalado (e conta, para Copilot/Amazon Q/Kiro) |
| 14 | PowerShell 7 (`pwsh`) | **não medido** | README | sim (guard) | Instalar `pwsh` na VM |

## Censo pós-ML-W4 (2026-10-16)

> ares-tf · branch `fix/fechar-os-grupos-de-falha-de-windows-ml-w4-censo` · run `38941248253`

### Commits do ML-W4

| Hash | Descrição |
|---|---|
| `10613e10` | fix(ci): remover pip install pypi/ e npm ci do job censo |
| `b88b532b` | fix(ci): usar go env GOEXE no build do censo Windows para que exec.LookPath encontre trackfw.exe |

### Contagens

| Censo | Run | Total | OK | FAIL |
|---|---|---|---|---|
| Referência (v9.5.0 main) | 38319939952 | 278 | 274 | 4 |
| Pós-ML-W4 | 38941248253 | 278 | 274 | 4 |

### FAILs pós-ML-W4

| Rótulo | Causa | Previsto? |
|---|---|---|
| `scaffold-update-chmod-removed/direction-c-baseline` | NTFS sem ACL (#421) | ✅ previsto |
| `scaffold-update-chmod-removed/direction-c-detected` | NTFS sem ACL (#421) | ✅ previsto |
| `git-branch-guard-dedup/baseline-skips-project-entry` | Regressão de dedup introduzida pelo ML-R3 (manager.go) | ❌ novo |
| `git-branch-guard-dedup/double-slash-tolerance` | Regressão de dedup introduzida pelo ML-R3 (manager.go) | ❌ novo |

### Rótulos que saíram de FAIL

| Rótulo | Causa original (referência) | Causa intermediária (run 38937863655) | Correção |
|---|---|---|---|
| `git-branch-guard-global-hook-resolvable/kiro-dedicated-file/baseline` | pip instalava trackfw.exe antigo (sem guard) | exec.LookPath não encontrava bin/trackfw (sem .exe) | go build com $(go env GOEXE) |
| `roadmap-req-frontmatter-path/go/from-req-baseline` | pip instalava trackfw.exe antigo (sem guard) | exec.LookPath não encontrava bin/trackfw (sem .exe) | go build com $(go env GOEXE) |

### Causa raiz do `exec.LookPath` — por que `export PATH` não basta

No CI Windows, o `check-gates-falsify.sh` compila com `go build -o bin/trackfw` e depois
faz `export PATH="$ROOT_DIR/bin:$PATH"`. O arquivo produzido é `bin/trackfw` **sem `.exe`**.

O `exec.LookPath("trackfw")` do Go procura por PATHEXT (`.COM`, `.EXE`, …). O arquivo nu não
casa — "not found in PATH". Antes do ML-W4, o `pip install pypi/` colocava um `trackfw.exe`
em `C:\...\Python\Scripts\` via `$GITHUB_PATH` (mecanismo persistente de PATH do Actions);
esse binário era encontrado pelo LookPath, mas era a versão antiga sem o subcomando `guard`.

Correção: `go build -o "bin/trackfw$(go env GOEXE)"` — produz `bin/trackfw.exe` no Windows.
Ver vault: `go-build-no-exe-suffix-exec-lookpath-falha-no-windows-2026-10-16`.

### Status do aceite do ML-W4

- ✅ `kiro-dedicated-file/baseline` → OK
- ✅ `roadmap-req-frontmatter-path/go/from-req-baseline` → OK
- ✅ NTFS noacl (#421) continuam FAIL (esperado)
- ❌ 2 rótulos novos em FAIL: `git-branch-guard-dedup/baseline-skips-project-entry` e
  `git-branch-guard-dedup/double-slash-tolerance` — regressão do ML-R3, fora do escopo do ML-W4.
  Referência (v9.5.0): ambos OK. Introduzidos pelos commits do ML-R3 em `internal/integrations/manager.go`.

O critério "nenhum rótulo novo em FAIL" **NÃO está atendido** por causa da regressão do ML-R3.
O ML-W4 resolveu sua causa específica; a regressão do dedup deve ser investigada e corrigida no
escopo do ML-R3 antes de fechar o roadmap.

## O que **não** dá para afirmar hoje

- "Suporte total ao Windows": há 14 falhas conhecidas no x64 (2 com efeito de segurança: itens 2 e
  4), o item 1 (guard global) está confirmado vivo, e os itens 13 e 14 — guard — estão sem medição.
- Que as 5 causas `INFER` da lista conhecida são as causas reais.
