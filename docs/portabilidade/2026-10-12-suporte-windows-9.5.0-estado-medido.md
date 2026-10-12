# Suporte a Windows no trackfw 9.5.0 — estado medido

> 2026-10-12 · zeus-tf · consolida `2026-10-12-censo-windows-x64-9.5.0.md` (ares-tf, CI x64) e
> `2026-10-12-medicao-vm-windows-9.5.0.md` (artemis-tf, VM ARM64), com conferências do arquiteto.

## Como ler

- **VM investiga, CI mede.** Número que vira afirmação sai do `windows-latest` x64. A VM é ARM64.
- Estados: **vivo** (reproduz na 9.5.0) · **fechado** · **sumiu com a v8** · **não medido** (com o que falta).

## 🔴 Validade do instrumento — ressalva que vale para tudo que vem do censo

O censo `38319939952` (`main` em `a1cc6e93`) rodou com a falsificação do próprio censo passando.
Mas a diferença **por nome** contra o censo de 24/09 (`36036473391`), feita pelo arquiteto, mostra
**42 rótulos que sumiram e 13 novos**:

- 21 são `release-tag-parity/*/go-behavioral-pin`; os outros incluem `update-harness/*` (6),
  `validate-rule-pins/pin1–6`, `skip-parity/*` (2) e `falsify/setup-sXX` (5).
- Os scripts `check-update-harness-parity.sh` e `check-skip-parity.sh` **não existem mais** — sumiço
  explicado.
- 🔴 **Não explicado:** `check-release-tag-parity.sh` e `check-validate-rule-pins.sh` existem, ainda têm
  os cenários `go-behavioral-pin`, e `check-release-tag-parity.sh` os emitiu no macOS hoje. No log do
  censo: 40 linhas `go-behavioral-pin` na base, **0 hoje**. Hipóteses não testadas: o driver de
  falsificação passou a capturar a saída aninhada dos gates (mudança de formato, inofensiva) **ou**
  esses cenários deixaram de rodar no Windows (gate que parou de emitir — precedente
  `release-tag-parity/dirty-tree`).

**Consequência:** a contagem de FAIL do censo (abaixo) é confiável para o que ele **emitiu**; não
prova nada sobre os 42 que sumiram até a hipótese acima ser decidida.

## Tabela

| # | Item | Estado na 9.5.0 | Evidência | Segurança? | Próximo passo |
|---|---|---|---|---|---|
| 1 | Guard global gravado/auditado sob `$HOME` quando o agente lê `%USERPROFILE%` (REQ-2026-09-01) | **não medido** | VM: medição usou `init --ai-tools`, que não escreve a fiação global; quem escreve é o `update harness` (`internal/generators/update.go`, `harnessCredentialGuardTarget*`), e o `validate` a lê por `homedir.Dir()` (`validator_git_branch_guard.go:162`). Claude Code **não instalado** na VM | 🔴 sim | Refazer com `update harness` e Claude Code instalado na VM; número final pelo `windows-probe.yml` |
| 2 | ML-R3 — guard de entrada cego a `\` (`internal/integrations/manager.go:756`) | **vivo, contido** | VM: `..\outside.md` passa o guard de entrada e é barrado por `beneath()`. macOS: não é travessia (vira arquivo chamado `..\outside.md` dentro da raiz) | 🔴 sim (defesa em uma camada) | ML de correção com teste que reprova sem Windows |
| 3 | Censo: FAIL emitidos | **4 rótulos** (base x64: 11; base ARM64 09-08: 512) | `censo-windows-x64-9.5.0.md` | não | Ver 3a–3c |
| 3a | NTFS sem ACL de execução (#421) — `scaffold-update-chmod-removed/direction-c-*` | vivo (2) | censo | não | Já tem issue; o `baseline` era OK vácuo antes e agora se declara "não exercitado" |
| 3b | `git-branch-guard-global-hook-resolvable/kiro-dedicated-file/baseline` | **vivo, novo**, causa não investigada | censo | possível (guard) | Investigar — é rótulo de guard |
| 3c | `roadmap-req-frontmatter-path/go/from-req-baseline` | **vivo, novo**, causa não investigada | censo | não | Investigar |
| 4 | ML-R2b1b — `{owner}/{repo}` some no shim do `gh` | **não medido** | censo: os cenários estão em blocos PROOF ou bloqueados por `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1` | não | Sonda dedicada, se ainda houver consumidor |
| 5 | ML-R2b2 — 76 rótulos sem triagem | **sumiu em grande parte** | 4 FAIL hoje; 10 rótulos viraram OK desde 24/09 (`git-branch-guard/stdin-drain`, `dedup`, `setup-s75/76/87/158/175`, `sandbox-*`) | — | Reclassificar no roadmap bloqueado depois de decidir a ressalva do instrumento |
| 6 | Prazo de rede: `ssh` neto órfão no estouro (RS-2) | **vivo, declarado** | VM (Git Bash): retorno em ~10 s com `git fetch timed out after 5s (TRACKFW_NETWORK_TIMEOUT)`; o `ssh` falso segue vivo | não | Resíduo aceito no ADR; PowerShell não medido (script com caminho POSIX) |
| 7 | Testes que pulam no `windows-full-suites` | **20 SKIP** | censo §5 | — | Dois protegem critério de aceite: `TestGitExec_AC2_Bite_NoWaitDelay` (prova de mordida do prazo) e `TestGitExec_WaitDelay_ExitZeroWithHangingGrandchild` (hook) |
| 8 | 28 FAIL `git init … unable to access 'NUL'` | **artefato da VM** | os 5 `TestADRPrefixE2E_*` passam no x64 | não | Nenhum no produto; a VM tem git/config diferente |
| 9 | Instalação (REQ-2026-09-05) | **em grande parte fechado, REQ desatualizada** | `install.sh` aceita MINGW/MSYS/Cygwin em amd64/arm64; README tem seção "Windows support (partial)"; Claude Code e Codex provados ponta a ponta (2026-10-07) | não | Reescrever a REQ com o que sobra (itens 10–11) |
| 10 | Hooks ponta a ponta com agente real: Cursor, Windsurf, Gemini, Kiro, Copilot, Amazon Q | **não medido** | README: linha do hook medida no shell; agente não rodado | sim (guard) | Precisa de cada CLI instalado (e conta, para Copilot/Amazon Q/Kiro) |
| 11 | PowerShell 7 (`pwsh`) | **não medido** | README | sim (guard) | Instalar `pwsh` na VM |

## O que **não** dá para afirmar hoje

- "Suporte total ao Windows" — os itens 1, 10 e 11 são guard e estão sem medição; a ressalva do
  instrumento cobre 42 rótulos.
- Que o censo está verde para os gates de `release-tag-parity` e `validate-rule-pins` no Windows.
