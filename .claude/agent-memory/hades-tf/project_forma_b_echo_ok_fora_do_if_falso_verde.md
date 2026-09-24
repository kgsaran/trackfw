---
name: forma-b-echo-ok-fora-do-if-falso-verde-check-gates-falsify
description: Nos Cenários 87 e 158 do check-gates-falsify.sh o `echo OK` da baseline está fora do `if` — a baseline reprova e o rótulo imprime OK na linha seguinte; não corrigido
metadata:
  type: project
---

Medido em 2026-09-25 (ML-0A, censo `36036473391`). `scripts/check-gates-falsify.sh` tem **duas
formas** para o mesmo braço de baseline:

- **Forma A (correta)** — Cenários 75, 76, 175: `if ! gate; then FAIL; falsify_fail_point; else
  falsify_count_success; echo OK; fi`
- **Forma B (falso verde)** — Cenários 87 (`:6066`) e 158 (`:6123`): `if ! gate; then FAIL;
  falsify_fail_point; fi` e o **`echo OK` fora do `if`**, sem `falsify_count_success`

No log, linhas adjacentes: `1457 FAIL [setup-s87-baseline]` → `1458 OK [release-tag-parity/
content-from-commit-baseline]`; e `2343 FAIL [setup-s158-baseline]` → `2344 OK [.../
refs-replace-bypass-baseline]`.

**Why:** é o **único falso verde** do cluster de Windows — todos os outros achados reprovam alto. O
que se perde não é a asserção (os braços de detecção casaram a mensagem e dispararam de verdade); é o
**discriminante de delta único**: sem baseline válida, o `OK` da detecção não distingue "a sabotagem
foi detectada" de "o gate já reprovava por outro motivo e a mensagem coincidiu".

**How to apply:** o detector é mecânico e não exige ler intenção — a Forma B imprime `echo OK` **sem**
`falsify_count_success`, logo a **contagem de linhas `^OK` do log diverge do arquivo de tally**
exatamente nos sítios afetados. É o formato do gate que fecha essa forma. Ao auditar qualquer braço de
baseline novo neste script, confira se o `echo OK` está no `else`.

🔴 Não corrigido em 2026-09-25, e **não está em nenhuma REQ** — não é um dos 14 rótulos do censo.
Ver [[censo-windows-13-rotulos-sao-11-dois-fantasmas-proof]].
