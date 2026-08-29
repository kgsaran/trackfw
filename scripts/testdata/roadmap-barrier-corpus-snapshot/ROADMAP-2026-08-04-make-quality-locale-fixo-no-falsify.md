---
status: done
date: 2026-08-04
req: "docs/req/REQ-2026-08-04-make-quality-falha-sob-locale-pt-br-teste-fixa-literal-em-ingles-no-violations-found.md"
squad: ""
---

# Roadmap: make quality — força locale fixo no gate de falsificação em vez de pin em inglês

> Created: 2026-08-04 | Status: done

REQ: REQ-2026-08-04-make-quality-falha-sob-locale-pt-br-teste-fixa-literal-em-ingles-no-violations-found.md

## Diagnóstico / Contexto

`scripts/check-gates-falsify.sh` Cenário 29 (linha ~1947) pina byte-a-byte a mensagem de sucesso do
`validate` contra o literal em inglês `"✓ No violations found.\n"`, mas os 3 CLIs imprimem essa mensagem
via `i18n_t("validate.ok")`, resolvida pelo locale ativo do processo. Sob `pt_BR.UTF-8`, a saída real
diverge do literal pinado e o gate falha sem regressão real. Decisão de design em
`docs/adr/ADR-2026-08-04-make-quality-forca-locale-fixo-no-gate-de-falsificacao-em-vez-de-pin-em-ingles.md`:
fixar `LANG=en_US.UTF-8 LC_ALL=en_US.UTF-8` no ambiente dos subprocessos comparados pelo próprio script,
em vez de depender do locale externo ou ler a expectativa dinamicamente (o que enfraqueceria a prova de
regressão).

## Acceptance Criteria

- [x] AC1 — `scripts/check-gates-falsify.sh` Cenário 29 passa deterministicamente sob `pt_BR.UTF-8` e
      `en_US.UTF-8`
- [x] AC2 — A prova de detecção de regressão (Python reintroduzindo `"✓ Governance OK"` hardcoded)
      continua reprovando corretamente
- [x] AC3 — `make quality` roda verde numa máquina com `LANG=pt_BR.UTF-8` sem o desenvolvedor precisar
      trocar o locale manualmente

## Wave 1 — Fix do gate (1 ML)
> Dependências: Independente

### ML-1A — Fixar locale no Cenário 29 (e cenários irmãos que comparam saída textual i18n)
**Status:** ✅ Concluído
**Arquivos afetados:**
- `scripts/check-gates-falsify.sh` (Cenário 29, linhas ~1923-1985; auditar se outros cenários do script
  também comparam saída textual dependente de i18n e sofrem do mesmo problema — ex. Cenários 30/31 que
  mencionam "Status Inventory block" pinado)

**Ações:**
1. No início do script (ou localmente nos subshells que invocam os 3 binários no Cenário 29), fixar
   `LANG=en_US.UTF-8 LC_ALL=en_US.UTF-8` no ambiente antes de `s29_go_out=$(...)`, `s29_node_out=$(...)`,
   `s29_python_out=$(...)` e na prova de detecção (`s29c_python_out=$(...)`).
2. Auditar os demais `falsify/status-inventory*` (linhas ~7-8 do output de `make quality`, cenários 30/31)
   por dependência de locale semelhante — se houver, aplicar a mesma correção.
3. Reproduzir o bug antes da correção (`LANG=pt_BR.UTF-8 bash scripts/check-gates-falsify.sh` falhando no
   Cenário 29) e confirmar que passa a verde depois da correção, no mesmo locale.

**Critérios de aceite:**
- [x] `LANG=pt_BR.UTF-8 LC_ALL=pt_BR.UTF-8 bash scripts/check-gates-falsify.sh` verde
- [x] `LANG=en_US.UTF-8 LC_ALL=en_US.UTF-8 bash scripts/check-gates-falsify.sh` verde
- [x] `make quality` verde nos dois locales

**Comandos de validação:** `LANG=pt_BR.UTF-8 make quality && LANG=en_US.UTF-8 make quality`
