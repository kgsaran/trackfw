---
status: Open
date: 2026-09-11
author: ""
adr: ""
sucede: "REQ-2026-08-30-barrier-executa-gate-de-roadmap-nao-confiavel-porque-roadmaptrustforgates-falha-aberto-em-todo-caminho-de-erro.md, REQ-2026-09-01-serve-interpola-host-em-string-de-shell-e-permite-injecao-de-comando-ao-abrir-o-browser.md"
roadmap: "docs/roadmaps/backlog/ROADMAP-2026-09-11-residuos-dos-pareceres-de-seguranca-do-barrier-e-do-serve.md"
---

# REQ: residuos dos pareceres de seguranca do barrier e do serve

> Date: 2026-09-11 | Status: Open
| Linear Issue: 
| Jira Issue: 

## Motivation
<!-- Why is this requirement needed? What problem does it solve? -->

## Acceptance Criteria
- [ ]
- [ ]

## Linked ADR
<!-- Reference the ADR that governs this requirement -->
ADR: 

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
<!-- Reference the roadmap that implements this requirement -->
Roadmap: `docs/roadmaps/backlog/ROADMAP-2026-09-11-residuos-dos-pareceres-de-seguranca-do-barrier-e-do-serve.md`
## Motivation

REQ **sucessora**, criada pela `ADR-2026-09-10-req-de-campanha-tem-escopo-congelado-e-achado-novo-vai-para-sucessora`.

As duas REQs de origem — `barrier` (fail-open) e `serve` (injeção de comando) — **atingiram o
objetivo**: o `hades-tf` emitiu **APROVA** nas duas, depois de verificar o fechamento dos próprios
achados. O que sobrou foi declarado **não-bloqueante pelos próprios pareceres**.

🔴 **Por que sucessora e não "deixar em `wip`":** roadmap com resíduo parado é como o de Windows
chegou a **34 MLs e 2.496 linhas**. REQ sem condição terminal não fecha nunca, e `wip` deixa de
significar "em andamento". Fechar duas e abrir uma dá saldo **−1** e devolve estado verdadeiro a
cada uma.

---

## R1 — `barrier`: a metade do CALLEE (cobertura sem paridade)

`TestRoadmapTrustForGates_VerifiesPassedBuffer` existe **só no Go**. Node e Python não têm
equivalente.

🔴 É o **F3 repetido dentro da correção do F1**: o código tem paridade, a **guarda que a prova** não
tem. Um gate que existe em um runtime e não nos outros protege um terço do que afirma.

**Portar para Node e Python.**

## R2 — `barrier`: a metade do CALLER (🔴 não falsificável por comportamento)

O invariante *"o buffer passado ao verificador é o buffer consumido pelo parser de gates"* **não é
verificável por fixture**: sem escritor concorrente, duas leituras devolvem bytes idênticos e
**nenhum teste determinístico distingue**.

Fechável **só estruturalmente** — guarda contra `readFile`/`open` duplicado no caller. **Ausente nos
3 CLIs.**

🔴 **É o caso raro em que guarda estrutural NÃO é o remédio fraco.** O projeto vinha preferindo
comportamental à estrutural, e com razão — mas aqui **o comportamento não discrimina por
construção**. Quando o observável não separa os estados, a estrutura é o único lugar onde a separação
existe.

## R3 — `serve`: cenários 8-9 do gate sem mensagem `FAIL` explícita

Com o módulo ilegível, o `set -euo pipefail` aborta na linha do `grep` antes da checagem de vazio. O
gate termina RC=2 pelas falhas acumuladas — **não é falso-OK**, mas a mensagem que nomeia a causa não
aparece. Cosmético; medido por execução (`chmod 000`) pelo `hades-tf`.

## R4 — 🔴 `serve`: pycache pode fazer o cenário medir código que não existe mais

Um `serve.cpython-*.pyc` válido faz o CPython **importar o cache** mesmo com `chmod 000` no fonte — a
validação é por `stat()`. Se o `.pyc` estiver correto e o fonte for trocado por código vulnerável, o
cenário **passa sobre código não medido**.

É a classe de vacuidade **escondida no interpretador**. O `hades-tf` só a viu porque removeu o cache
explicitamente antes de testar.

**O gate precisa invalidar o pycache antes de medir.**

---

## Acceptance Criteria

- [ ] **AC1** — R1: `VerifiesPassedBuffer` portado para Node e Python, com a mesma conclusão afirmada.
- [ ] **AC2** — R2: guarda **estrutural** contra leitura duplicada no caller, nos 3 CLIs. 🔴 E a
      justificativa de por que estrutural **aqui** fica escrita — senão o próximo leitor a troca por
      comportamental achando que melhora.
- [ ] **AC3** — R3: cenários 8-9 emitem mensagem que **nomeia a causa**, sem perder o RC.
- [ ] **AC4** — R4: o gate invalida o pycache antes de medir. 🔴 **Falsificação:** com `.pyc` válido e
      fonte trocado, o cenário **reprova** — provando que mede o fonte, não o cache.
- [ ] **AC5** — contra-braço em cada um: configuração correta ⇒ passa. Guarda que só reprova é
      indistinguível de guarda que reprova sempre.
- [ ] **AC6** — 🔴 **varredura:** o R4 é isolado? Outros gates nossos importam módulo Python cujo
      fonte eles pretendem medir? **Derivar com comando escrito**, não presumir.

## Proveniência

- `docs/seguranca/2026-09-10-parecer-barrier-fail-closed.md` (veredito **APROVA**, R1 e R2)
- `docs/seguranca/2026-09-11-parecer-serve-injecao-de-comando.md` (veredito **APROVA**, R3 e R4)
