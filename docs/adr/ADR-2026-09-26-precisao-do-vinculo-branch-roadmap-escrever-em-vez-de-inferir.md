---
status: Proposed
date: 2026-09-26
author: "trackfw_architect"
---

# ADR: precisão do vínculo branch↔roadmap — escrever em vez de inferir

> Date: 2026-09-26 | Status: Proposed

REQ: `docs/req/REQ-2026-09-09-req-nasce-orfa-porque-criar-req-e-criar-roadmap-sao-dois-comandos-e-o-segundo-se-esquece.md` (AC11)
Origem: **#273** (consumidor externo, com medição) · REQ-2026-08-20, absorvida em 2026-09-12

## Context

O trackfw decide se uma branch está **governada** comparando o slug da branch com o **nome do arquivo**
do roadmap, por substring. Isso vive em **dois sítios**, com implementações separadas:

| sítio | função | consumido por |
|---|---|---|
| `internal/validator/validator.go:3493` | `strings.Contains(normalizeBranchSlug(name), branchSlug)` | `validate` · `branch new` · `commit` · `ship` |
| `internal/generators/roadmap.go:791,805` | `containsIgnoreCase(e.Name(), name)` | `roadmap move` |

🔴 **A regra erra nas duas direções, e isso está medido.**

**Frouxa demais** — 185 roadmaps e 111 branches históricas deste repositório:

```
fix/roadmap   casa 159 de 185 roadmaps   (86% do corpus)
fix/ci        casa  28
strings.Contains(x, "")  é SEMPRE verdadeiro  →  `roadmap move ""` move um roadmap arbitrário
```

**Restrita demais** — 64 roadmaps do fork do reportante, 22 branches governadas:

```
2 de 22 branches legitimamente governadas foram REJEITADAS  (~9%)
caso real: feat/adrs-retroativas-da-divida-do-acervo
           contra ROADMAP-2026-09-05-divida-de-governanca-do-acervo-…
           → o autor teve de RENOMEAR a branch para caber na regra
```

A causa das duas direções é **uma só**: substring exige que um nome esteja **literalmente dentro** do
outro, enquanto os dois nomes descrevem o mesmo trabalho por perspectivas diferentes — a branch nomeia
**o trabalho**, o roadmap nomeia **o título da REQ**.

### O risco que domina qualquer decisão aqui

Este portão é atravessado por **todo** `branch new`, `commit` e `ship`. 🔴 **Falso-positivo aqui não
irrita: paralisa.** E há um caminho terminal, declarado no roadmap como *deadlock de bootstrap*:

```
branch nomeada corretamente → validator rejeita → `trackfw commit` falha
→ `git commit` cru é bloqueado pelo guard → o fix do matcher não pode ser mergeado
→ intervenção manual de git
```

**Isso já acontece em ~9% dos casos medidos. Não é hipótese.**

## Decision

### D1 — O vínculo passa a ser ESCRITO. A inferência vira fallback, não fonte.

🔴 **O `trackfw branch new` sabe exatamente qual roadmap está em `wip/` no instante em que cria a
branch.** Inferir depois, por nome, é **reconstruir uma informação que existia e foi jogada fora** —
e todos os candidatos de inferência (substring, fronteira, sobreposição de tokens) são aproximações
de algo que **não precisa ser aproximado**.

O vínculo escrito é a **fonte de verdade**. A inferência continua existindo **apenas** para branches
que não passaram pelo `branch new` (criadas com `git checkout -b`, ou vindas de clone/fork).

### D2 — A relação da inferência é SOBREPOSIÇÃO DE TOKENS, não substring nem fronteira.

**O candidato "fronteira" está falsificado, e por aritmética, não por amostra:** fronteira é
**subconjunto estrito** de `Contains`, logo **não pode corrigir nenhum falso-negativo** — só criar
mais. A medição confirma que ele também quase não ajuda na direção frouxa:

```
             substring   fronteira
fix/roadmap      159        159      ← 86% do corpus, INALTERADO
fix/guard         16         14
fix/ci            28          2      ← só aqui funciona
20 slugs curtos  326        266      (−18%)
casamentos das 111 branches reais: 109 → 109, regressão 0
```

⚠️ **O número mínimo de tokens NÃO é decidido aqui.** O reportante propôs ≥2 e **declarou que não o
estava propondo como valor calibrado**. A calibração contra os 185 roadmaps é entregável do `ML-3A`,
e o `AC14` exige que ela vire **gate**, não nota de rodapé.

### D3 — `validator.go` é a FONTE do matcher. `generators/roadmap.go` delega.

Razão: `BranchSlugMatchesRoadmap` já é a função nomeada e exportada, e é a que serve o portão
crítico (`branch new`/`commit`/`ship`). `containsIgnoreCase` é um helper local de 3 linhas.

🔴 **Sem esta decisão, a Wave 3 entrega dois matchers novos que concordam por coincidência** — que é
exatamente o defeito do AC5 da REQ-2026-08-31 (*"a mensagem é idêntica em todos os sítios"* satisfeito
por coincidência textual, não por construção), em outra roupa, e nesta casa já custou um microlote
inteiro para desfazer.

### D4 — 🔴 ORDEM DE APLICAÇÃO: modo ADITIVO primeiro. É o que evita o deadlock.

A troca acontece em duas etapas, **e a primeira não pode apertar nada**:

| etapa | o que faz | por quê |
|---|---|---|
| **1 — aditiva** | o matcher novo aceita **tudo que `Contains` aceitava, e mais** (vínculo escrito + sobreposição de tokens) | 🔴 **reversível sem intervenção manual de git**: nenhuma branch que passava antes passa a falhar, então o fix do matcher pode ser commitado pelo próprio `trackfw commit` |
| **2 — restritiva** | remove o que `Contains` aceitava e a sobreposição não aceita (o `fix/roadmap` casando 159 de 185) | só com a calibração do `ML-3A` medida e o gate do `AC14` no ar |

**O caso do nome vazio é exceção à ordem, e por uma razão escrita:** `strings.Contains(x, "")` é
sempre verdadeiro, e **não existe consumidor legítimo** de `roadmap move ""`. Recusá-lo é
estritamente aditivo em segurança e não pode paralisar ninguém — por isso o AC12 já o põe primeiro.

### D5 — A contenção do risco de falso-positivo é medida, não prometida.

O `AC14` exige que a medição dos 185 roadmaps vire **gate**: mudança no matcher que altere o conjunto
de casamentos **reprova** até que a diferença seja declarada. E o `AC15` (acrescentado em 2026-09-26)
exige falsificação da direção **restrito demais** — sem ele, um candidato que aperta passa nos ACs
antigos e piora o que o #273 reportou.

## Consequences

**Positivas**

- O vínculo deixa de ser adivinhado onde ele é **conhecido** — o caso majoritário, porque o
  `branch new` é o caminho recomendado e o guard bloqueia o `git checkout -b` cru.
- As duas direções do erro caem com **uma** decisão, em vez de trocar uma pela outra.
- Nomes de branch voltam a poder descrever **o trabalho**, sem precisar caber no nome do roadmap.
- O `roadmap move ""` para de mover roadmap arbitrário.

**Negativas, e declaradas**

- 🔴 **Superfície nova de estado:** o vínculo escrito é um artefato que pode ficar **obsoleto**
  (branch renomeada, roadmap movido, rebase). O `ML-3A` tem de decidir o que acontece quando o
  registro aponta para um roadmap que saiu de `wip/` — e a resposta **não pode** ser "cai no
  fallback em silêncio", que reintroduz a adivinhação sem avisar.
- **Duas etapas custam mais que uma.** A etapa 2 pode ficar pendente por tempo indeterminado, e nesse
  intervalo a direção frouxa continua aberta. É preço aceito: a alternativa é o deadlock.
- Branches de fork/clone continuam dependendo de inferência — **não há vínculo escrito para importar**.

**Residual aceito**

- **Colisões em filesystem case-insensitive** (macOS, Windows): dois roadmaps `REQ-A.md` e `req-a.md`
  são o mesmo inode. Nenhum matcher distingue; exige gate de lint próprio.
- **Assimetria de protocolo `roadmap move` vs `req move`** em múltiplos casamentos — registrada no
  roadmap como residual independente do matcher.

## Alternatives Considered

**A1 — Casamento por fronteira de palavra.** 🔴 **Falsificado por aritmética:** é subconjunto estrito
de `Contains`, logo não pode corrigir falso-negativo. E a medição mostra ganho quase nulo na direção
frouxa (`fix/roadmap` inalterado em 159 de 185). A REQ-2026-08-20 o descrevia como *"provavelmente
suficiente"* — **era**, até alguém medir.

**A2 — Apenas apertar o limiar do `Contains`** (exigir N caracteres mínimos). Trata o sintoma:
o problema não é o **limiar**, é a **relação**. Um slug longo e legítimo continua rejeitado, e um
slug longo e genérico continua casando.

**A3 — Vínculo escrito sem nenhum fallback.** Quebraria toda branch criada fora do `trackfw branch
new` — incluindo forks e clones, que são o caso do reportante do #273. Rejeitado: transformaria um
falso-negativo de ~9% em 100% para essa população.

**A4 — Inferência por título da REQ em vez do nome do arquivo.** Já existe no Node (`resolveRef`,
serve board) e está declarada como residual: é first-wins em colisão e só serve UI de leitura.
Não resolve o portão de escrita.

## Linked REQ
REQ: `docs/req/REQ-2026-09-09-req-nasce-orfa-porque-criar-req-e-criar-roadmap-sao-dois-comandos-e-o-segundo-se-esquece.md`
