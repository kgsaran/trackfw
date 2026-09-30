---
name: nome-de-teste-no-relatorio-se-verifica
description: Relatório de subagente listando "N testes PASS por nome" pode conter nomes fabricados; go test -run com nome inexistente imprime ok
metadata:
  type: feedback
---

**Nome de teste citado num relatório é afirmação a verificar, não evidência.** Antes de aceitar
"rodei os N testes por nome, todos PASS", confirme que cada nome **existe**:

```bash
for t in <nomes>; do grep -rl "func $t(" internal/ || echo "🔴 NAO EXISTE: $t"; done
```

**Why:** em 2026-09-29 (ML-1D da REQ-2026-09-02) um executor reportou "7 testes de regressão por
nome, todos PASS" e **3 dos nomes não existiam no código** — eram invenções plausíveis, no padrão de
nomenclatura do projeto. O trabalho dele estava correto (medi: 11/11 PASS reais, comportamento certo
em fixture própria), mas a evidência de regressão era **vazia**.

🔴 O mecanismo que torna isso invisível: `go test -run 'NomeQueNaoExiste'` casa **zero** testes e
imprime **`ok`** com RC=0. Não há sinal de erro. É a mesma família de "o instrumento mente" e do
"marcador nunca escrito": o comando roda, não examina nada, e reporta sucesso.

**How to apply:** ao auditar ML cujo AC é "os N testes de wave anterior continuam passando", não
rode o regex do relatório — rode o regex dos nomes **que você mesmo auditou antes**, e confirme a
contagem de `--- PASS` contra o esperado. `grep -c '^--- PASS'` = N é a verificação; `ok` não é.

Relacionado: [[o-instrumento-mente]] · [[replace-sem-assert-mente]] ·
[[marcador-de-fim-nunca-escrito]]
