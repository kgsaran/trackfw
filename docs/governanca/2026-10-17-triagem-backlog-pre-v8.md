# Triagem medida do backlog anterior à v8 — 2026-10-17

> REQ-2026-10-17 · zeus-tf · código medido: `main` em `f8870c79` (v9.7.0).
> Triagem por três `hefesto-tf` em paralelo (ML-1A/1B/1C, só leitura); o arquiteto reconferiu por
> amostra os vereditos de "vivo" com o binário (guard e `git add -A`, `check-referential-integrity`
> sobre árvore vazia, `validate --json` sujando o stderr, `discover.go:119` sem `Chmod`).

## Resultado

**16 roadmaps → 10 seguem no backlog**, todos com escopo reescrito: 4 vivos como estavam e 6 que mudaram de forma (encolheram). Saem 6: 4 abandonados, 2 em `done`; o 12º foi absorvido pelo 11 (mesma causa).

| # | roadmap (resumo) | veredito | evidência | REQ → |
|---|---|---|---|---|
| 1 | resíduos dos pareceres do barrier e do serve | **sumiu com a v8** (5/5 itens) | todos eram portes ou cenários Node/Python; `barrier.go:364` já recebe o buffer lido (#316) | REQ-2026-09-11 → Abandoned |
| 2 | gates de paridade provam concordância, não completude | **sumiu com a v8** | `check-cli-parity.sh` e `check-atomic-write-anti-divergence` não existem; as cópias foram unificadas (`pathguard.go:128`) | REQ-2026-09-01 (anti-divergência) → Abandoned |
| 3 | gate de shell só checa grafia literal | **sumiu com a v8** | o defeito estava em `npm/src/…/barrier.js` e `pypi/…/barrier.py`; gate removido | REQ-2026-09-01 (shell) → Abandoned |
| 4 | byte NUL literal no fonte + gate | **já corrigido** | varredura sem NUL em `.go/.sh/.py/.js`; gate `scripts/check-no-literal-nul-in-source.sh` existe; REQ já Done | roadmap → done |
| 5 | triagem das REQs de paridade (era `blocked`) | **condição de bloqueio caiu** | v8 adotou um binário; ML-2A/2A-b/2B (gates cross-runtime) perdem o objeto | REQ-2026-09-12 (paridade) → Done |
| 6 | 🔴 guard bloquear staging com escopo implícito | **vivo** | ADR-2026-09-05 decidiu; `internal/guard/gitbranch.go` não tem a regra; `git add -A` → exit 0 (e `git commit` → exit 2) | Open |
| 7 | 🔴 `check-referential-integrity` vácuo | **vivo** | árvore sem REQ → "Referential integrity OK", exit 0; está no `make quality` | Open |
| 8 | `branch new` cria do HEAD atual | **vivo** | `branch.go:143` faz `checkout -b` sem start point nem `fetch` | Open |
| 9 | harness do arquiteto não proíbe sobrepor `model` | **vivo** | "Dispatch contract" de `assets/agents/architect.md` (e golden) só fala de `subagent_type` | Open |
| 10 | `validate` usage em erro / `--json` | **mudou de forma** | usage e gate corrigidos (#413); vivo: com violações, stderr recebe "2 violation(s) found" fora do JSON (`root.go:123`) | Open, só AC1 |
| 11 | valor de `agent_models` sem validação | **vivo** — **funde com o #12** | `render.go:587` só rejeita caractere de controle: `"5 && rm -rf /"` vira artefato com WARN e exit 0; mensagens de `~user/` já corrigidas (#271) | Open, AC1–AC2 |
| 12 | `update harness` lê `trackfw.yaml` do cwd | **mudou de forma** — **mesma causa do #11** | leitura do cwd corrigida (#207); sobra o valor de `model` com `:`/`"` gerando YAML inválido | REQ-2026-08-21 → absorvida pela REQ-2026-08-30; roadmap → abandoned |
| 13 | reconciliação dos #238/#240 e `.trackfw-log` | **mudou de forma** | `merge=union` feito (#251), roadmaps em done; vivo: `errors="replace"` sem decisão, allowlist obsoleta (diz "#238 ABERTO") e `DECL_RE` cego a `sys.stdout.reconfigure` — o gate diz OK | Open, AC1–AC4 |
| 14 | suítes não distinguem ambiente incompleto | **mudou de forma** | Node/Python sumiram; vivo: 3 testes de `validator_git_exec_test.go` chamam `git` sem `LookPath`+`Skip`; `make quality` sem checagem de pré-requisitos | Open, AC3–AC4 |
| 15 | débitos de higiene | **mudou de forma** | lock e `pypi/build` resolvidos; vivo: `discover.go:119` `os.WriteFile(…,0755)` sem `Chmod` (arquivo existente mantém o modo) | Open, só AC3 |
| 16 | instalação no Windows | **mudou de forma** | `install.sh` aceita Windows (#373), README tem a seção; vivo: hooks com Cursor/Windsurf/Gemini/Kiro/Copilot/Amazon Q, `pwsh`, caminhos com espaço/acento — **não medidos** | Open, escopo reduzido |

## Prioridade proposta dos vivos

1. **#6 guard de staging** — controle de segurança decidido em ADR e ausente; o erro que o originou ainda reproduz.
2. **#7 gate vácuo no `make quality`** — afirma integridade sem verificar nada.
3. **#11+#12 valor de `model`** — uma causa, uma REQ: validar formato e rejeitar em vez de avisar.
4. **#13 gate de encoding cego** — gate dizendo OK sobre allowlist obsoleta.
5. **#8 base do `branch new`** · **#9 contrato de despacho** · **#10 `--json`** · **#14 testes sem `git`** · **#15 `Chmod`**.
6. **#16 Windows** — medição, não código: os hooks dos demais CLIs, `pwsh`, caminhos com espaço/acento.

## Resíduos de documentação achados

- `docs/cli-parity.md:2986` cita `scripts/check-shell-posix-portability.sh`, que não existe.
- `.gitattributes:27-28` comenta bytes NUL em `npm/src/…`, que não existe.

## Limites

- Cada veredito de "sumiu"/"corrigido" foi feito pelo agente com evidência de arquivo ou PR; o
  arquiteto reconferiu os de "vivo" por execução, não todos os de "sumiu".
- Nada foi medido em Windows nesta triagem.
