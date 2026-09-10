---
status: wip
date: 2026-09-06
squad: ares-tf
req: "docs/req/REQ-2026-09-06-o-ci-de-windows-nao-bloqueia-regressao-e-nao-distingue-suite-que-nao-carregou-de-teste-que-reprovou.md"
---

# Roadmap: Ratchet por nome, e classe própria para suíte que não carrega

> Criado em: 2026-09-06 | Status: wip

## Context

REQ: `docs/req/REQ-2026-09-06-o-ci-de-windows-nao-bloqueia-regressao-e-nao-distingue-suite-que-nao-carregou-de-teste-que-reprovou.md`
ADR: `docs/adr/ADR-2026-09-05-o-ci-de-windows-bloqueia-por-conjunto-de-nomes-e-por-tipo-de-evento-nunca-por-contagem.md` (`Accepted`)

> 🔴 **Correção de auditoria (arquiteto, 2026-09-10):** este roadmap afirmava `Accepted` desde
> 2026-09-06, mas a ADR estava **`Proposed`** — e a REQ estava **órfã** (`roadmap: ""`), apesar de o
> roadmap apontar para ela. Vínculo de mão única. As duas coisas foram corrigidas agora, **antes** do
> despacho: ADR lida e **aceita**, REQ vinculada. Trabalho sobre ADR não aceita é decisão
> arquitetural tomada por omissão.
Fecha: **#275** e **#274**

## Diagnóstico

`windows-full-suites` roda com `continue-on-error: true` — **nenhuma regressão de Windows reprova um
PR**. E a contagem esconde regressão: medido 3x pelo consumidor externo, e **uma 4ª vez conosco**, no
PR #285, que baixou o total e introduziu 6 falhas.

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 1 — O discriminante, antes do ratchet
> Sequencial. A D3 da ADR exige medir antes de escrever.

### ML-1A — Distinguir "suíte não carregou" de "teste reprovou"
**Status:** 🔄 Em andamento · **Agente:** `ares-tf`
🔴 **Medir o discriminante nos DOIS cenários antes de escrevê-lo.** Foi pular esse passo que produziu
a nossa afirmação pública errada no `#274` — `pass 0 / fail 1` é idêntico nos dois casos.
Cobrir também `tests == 0`. **Falha de classe própria**, não linha na lista de nomes.

**Arquivos afetados:** `.github/workflows/quality.yml`

**Parity note:** a implementação é em CI script/YAML (infra), não em comando CLI.
O contrato de paridade dos 3 CLIs cobre COMANDOS públicos do trackfw, não scripts de CI.
Referência: `docs/cli-parity.md` (regra cobre `public commands`).

**Medições realizadas antes da implementação (2026-09-10):**

| Runtime | Cenário | Exit code | Sinal de evento | Observável |
|---|---|---|---|---|
| Go 1.25.2/macOS | (a) load failure — sintaxe | 1 | `[setup failed]` | Linha `FAIL\t<pkg> [setup failed]` |
| Go 1.25.2/macOS | (b) load + assert mistos | 1 | `[setup failed]` + `--- FAIL:` | Ambos na mesma saída |
| Go 1.25.2/macOS | (c) assertion failure | 1 | `--- FAIL: TestName` | Sem `[setup failed]` |
| Go 1.25.2/macOS | (d) zero tests | **0** | `[no test files]` | 🔴 exit 0 — indistinguível de sucesso |
| Python 3.14.7/macOS | (a) conftest error | **4** | exit code | Protocolo pytest estável |
| Python 3.14.7/macOS | (b) import error collection | **2** | exit code | Collection interrupted |
| Python 3.14.7/macOS | (c) assertion failure | 1 | exit code | Normal |
| Python 3.14.7/macOS | (d) zero tests | **5** | exit code | `no tests ran` |
| Node 26.8.1/macOS + TAP | (a) require quebrado | 1 | `exitCode:` presente no TAP | D3-bis confirmado |
| Node 26.8.1/macOS + TAP | (c) assertion failure | 1 | `exitCode:` **ausente** | `code: 'ERR_ASSERTION'` apenas |
| Node 26.8.1/macOS + TAP | (d) zero tests | **0** | `# tests 0`, `1..0` | 🔴 exit 0 — pior modo |

**Lacunas declaradas:**
- Go (d): `[no test files]` → exit 0. O discriminante `[setup failed]` não cobre isso; zero-test-Go é visível no log mas não bloqueia (exit 0).
- Node 20/windows-latest: não medido diretamente. Formato TAP estável entre Node 22 e 26; extrapolado.
- Node (d): `# tests 0` + exit 0 é o pior modo — o job parece ter passado. Emitido como `::error::` mesmo com exit 0.

**Discriminante implementado (tipo de evento, nunca contagem — ADR D3/D3-bis):**
- Go: `Select-String 'FAIL\t.*\[setup failed\]'` na saída capturada
- Python: exit code 2/3/4 → `suite-load-failure`; exit code 5 → `zero-test-failure`
- Node: `exitCode:` presente em bloco `not ok` do TAP → `suite-load-failure` (D3-bis)
- Node zero-test: exit 0 + `# tests 0` no TAP → `zero-test-failure`

**Falsificação (nas duas direções, 3 runtimes):**
Step `ML-1A — falsificação do discriminante` adicionado ANTES dos 3 runs de suíte.
Braço 1 (load failure → detector dispara) + Braço 2 (asserção → detector NÃO dispara).
Guarda de vacuidade por braço: verifica que a mutação foi aplicada antes de afirmar.

**Reconciliação teste↔conclusão (regra dura do projeto):**
O step de falsificação não contém testes de produto; contém PROBES que afirmam as conclusões desta seção de medição:
- `Assert-True ($goLoadFails.Count -gt 0) "..."` afirma: "Go: `[setup failed]` aparece na saída de load failure"
- `Assert-True ($goAssertFails.Count -eq 0) "..."` afirma: "Go: `[setup failed]` NÃO aparece na saída de assertion"
- `Assert-True ($rc_py_load -eq 2 -or ...-eq 4) "..."` afirma: "Python: exit code de load failure é 2 ou 4"
- `Assert-True ($rc_py_assert -eq 1) "..."` afirma: "Python: exit code de assertion é 1"
- `Assert-True ($tapLoadContent -match 'exitCode:') "..."` afirma: "Node: `exitCode:` aparece no TAP de load failure (D3-bis)"
- `Assert-True ($nodeAssertFails.Count -eq 0) "..."` afirma: "Node: `exitCode:` NÃO aparece no TAP de assertion"

## Wave 2 — O ratchet
> Dependências: Wave 1. Sem o discriminante, um estado sem nomes escapa do ratchet por construção.

### ML-2A — Lista versionada de vermelhos, por nome
**Status:** ⬜ Pendente · **Agente:** `ares-tf`
🔴 **A lista nasce de um run do CI**, nunca de máquina — o autor do `#275` declara que o Windows dele
não é o runner. Reprova nome fora da lista; **avisa** quando um nome da lista deixa de falhar.
🔴 **Guarda de não-vacuidade:** com a lista vazia e a dívida atual, o job **tem** de reprovar.

### ML-2B — Remoção de nome exige justificativa
**Status:** ⬜ Pendente · **Agente:** `ares-tf`
Corrigido, **renomeado** ou **deixou de executar** — o ratchet não distingue sozinho. Sem isto a
lista vira cemitério, que é a única forma de ele fracassar em silêncio.
**Falsificação obrigatória:** renomear um teste da lista **sem corrigi-lo** não pode virar verde.

## Wave 3 — Tirar a rede
> Dependências: Waves 1 e 2 fechadas e verdes.

### ML-3A — Remover `continue-on-error` do `windows-full-suites`
**Status:** ⬜ Pendente · **Agente:** `ares-tf`
🔴 **Só aqui.** Remover antes tornaria a `main` imergível com a dívida atual — o ponto do ratchet é
bloquear regressão **sem** exigir zero primeiro.
