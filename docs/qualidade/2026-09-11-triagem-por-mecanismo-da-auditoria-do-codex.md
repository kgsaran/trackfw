# Triagem por mecanismo — auditoria do Codex

> 2026-09-11 · entrada: `docs/qualidade/2026-09-11-auditoria-ampla-dos-fontes-orientada-pelos-issues.md`
> Regra de entrada combinada antes dos achados chegarem: **mesma causa ⇒ ML na REQ existente, nunca
> REQ nova.**

## O resultado, e ele não é o que se esperava

**5 achados. 5 já tinham REQ aberta. ZERO REQs novas.**

| # | achado do Codex | REQ que já existia | estado |
|---|---|---|---|
| A1 | `status` Python ignora REQs em subpastas | `REQ-2026-08-30-consumidores-que-nao-conhecem-by-agent-...` | **Open**, com roadmap |
| A2 | `serve` Python perde aresta com separador Windows | `REQ-2026-09-01-api-chain-do-serve-nao-desenha-aresta-...` | **Open**, sem roadmap |
| A3 | título com controle forja estrutura de REQ/ADR/note | `REQ-2026-08-30-titulo-de-roadmap-com-newline-forja-...` (**AC2**) | **Open**, sem roadmap |
| A4 | `check-referential-integrity` verde em árvore vazia | `REQ-2026-09-03-check-referential-integrity-diz-ok-...` | **Open**, sem roadmap |
| A5 | gate B2 Node desalinhado bloqueia o quality | — em correção **agora**, ML-1E-b | wip |

E mais: **A1, A2 e A3 já constavam da triagem de `2026-09-05`** (linhas 9, 16 e 13), cada uma marcada
**"AINDA VÁLIDA (verificado)"**, com arquivo:linha. O A3 é literalmente o **AC2 não implementado** de
uma REQ cuja AC1 foi entregue.

## 🔴 A conclusão desconfortável — e ela muda a estratégia

**O gargalo não é detecção. É fechamento.**

Passamos o dia perguntando *"por que não achamos o que o consumidor externo acha?"*. A resposta medida
é pior e mais simples: **achamos.** Está escrito, verificado, com arquivo e linha. E continua em
produção.

Três das quatro REQs **não têm roadmap nenhum**. Elas não estão sendo trabalhadas — estão registradas.

> **Registro não é correção.** É a frase que já está no `CLAUDE.md`, na Regra Dura de Causa Raiz, e é
> exatamente o que a medição do Codex acabou de provar sobre nós.

## O que isto faz com o plano de varredura

A REQ de varredura sistemática **continua valendo**, mas deixa de ser prioridade 1. Uma varredura que
produza mais achados sobre uma fila que já não é drenada **piora** o problema — é a "enxurrada de REQs"
nomeada pelo usuário em 2026-09-10, agora com evidência de causa.

**Prioridade correta, nesta ordem:**

1. **Dar roadmap e fechar A2, A3, A4** — três REQs `Open` sem roadmap, com causa medida e localizada
   duas vezes (por nós em 05/09, pelo Codex em 11/09). São as mais baratas do backlog inteiro:
   **a análise já foi paga, duas vezes.**
2. **A1** — já tem roadmap; verificar por que não avançou.
3. **Só então** a varredura sistemática, e com a régua "o que achar vira gate".

## O que a auditoria do Codex provou de fato

Ela **não** provou que temos um ponto cego de detecção. Provou o contrário:

🔴 **Um auditor independente, sem o nosso contexto, olhando o mesmo código, chegou às MESMAS
conclusões que nós já tínhamos escrito.** A nossa capacidade de achar está calibrada. O que falha é o
que acontece **depois** do achado.

Isso também recalibra o consumidor externo: ele não enxerga o que não enxergamos. Ele **corrige o que
não corrigimos** — abrindo issue, o que força a fila a andar.

## Mérito do artefato do Codex

Vale registrar o que ele fez melhor que os nossos relatórios:

- **reproduziu cada achado** com saída colada, não com asserção;
- escreveu **"como detectar antes"** por achado — que é a régua da varredura, entregue de graça;
- listou **o que descartou como falso positivo**, com motivo. Nós raramente escrevemos o que
  *não* é defeito, e é isso que permite confiar no zero de uma varredura;
- no A5, chegou por conta própria à mesma decisão do arquiteto: **não remover o cenário nem
  transformá-lo em skip.**
