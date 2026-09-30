---
status: wip
date: 2026-09-30
req: "docs/req/REQ-2026-09-30-cerca-nao-terminada-mascara-ate-o-fim-do-arquivo-em-silencio-e-o-barrier-apaga-o-ml-pendente.md"
squad: ""
---

# Roadmap: cerca nao terminada mascara ate o fim do arquivo em silencio e o barrier apaga o ML pendente

> Created: 2026-09-30 | Status: wip

## Context
<!-- Derived from REQ: REQ-2026-09-30-cerca-nao-terminada-mascara-ate-o-fim-do-arquivo-em-silencio-e-o-barrier-apaga-o-ml-pendente.md -->
REQ: docs/req/REQ-2026-09-30-cerca-nao-terminada-mascara-ate-o-fim-do-arquivo-em-silencio-e-o-barrier-apaga-o-ml-pendente.md

## Acceptance Criteria
- [ ] **AC1** — comportamento correto **por superfície**, nos 10 call sites (Wave 0)
- [ ] **AC2/AC3** — CLI: exit 2 nomeando a linha · **bem-formado continua passando**
- [ ] **AC4** — o braço D da sonda deixa de sair `passed`
- [ ] **AC5** — os 2 arquivos do acervo corrigidos **no mesmo PR**
- [ ] **AC6** — `serve` não quebra
- [ ] **AC7** — `cli-parity.md` descreve o real, por superfície
- [ ] **AC8** — `make quality` e CI verdes

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — o comportamento correto por superfície
> Dependências: nenhuma. 🔴 **BLOQUEIA toda implementação.**

### ML-0A — mapear os 10 call sites e decidir o que cada um faz
**Owner:** `hades-tf`
**Status:** ⬜ Pendente
**Arquivos de leitura:** `internal/roadmapdoc/roadmapdoc.go` (`FenceMask`:311, `ParseGates`:~674) ·
`internal/commands/barrier.go:164` · `internal/generators/roadmap.go:617` ·
`internal/generators/roadmap_show_json.go:128` · `internal/serve/api_board.go:168` ·
`docs/cli-parity.md` (§ *Roadmap parsing rules*, regra 6)
**Método:** medir. 🔴 **Não escrever implementação.** A Wave 0 tem autoridade para **bloquear**.

**Perguntas, em ordem de peso:**

1. 🔴 **O que cada uma das 10 superfícies deve fazer?** A regra 6 diz *exit 2*, mas `serve` é
   **servidor HTTP** — derrubá-lo por um roadmap malformado é pior que o defeito. Proponha o
   comportamento de cada uma, **com o motivo**. Candidatos para o `serve`: marcar o card como
   malformado no board · omitir o arquivo e registrar · servir o que der e sinalizar. **Meça o que o
   `serve` já faz hoje com o arquivo de cerca aberta que existe no acervo** — ele está lá, é o
   `ROADMAP-2026-08-22-wave-0-...` (abre na linha 460).
2. **`ParseGates` já detecta** e emite `unterminated gates fence starting at line %d`. Por que o
   `FenceMask` não? É esquecimento, ou há razão — `FenceMask` devolve `[]bool` e mudar a assinatura
   toca 10 sítios. **Qual é o menor corte que cumpre a regra 6 sem quebrar os 10?**
3. 🔴 **Os 2 arquivos do acervo: a cauda mascarada é REALMENTE só prosa?** Eu medi `### ML-`,
   `**Status:**`, `- [ ]` e `## Wave ` — **meça também** `**Critérios`, `**Gates`, `**Owner`,
   `- [x]`, e qualquer marcador que o parser leia. Se houver conteúdo governante escondido ali, o
   defeito **já** está agindo e a REQ muda de severidade.
4. **O que mais o `FenceMask` mascara em silêncio hoje?** Cerca aberta é um caso de entrada
   malformada; existem outros nessa família que a regra 6 promete e o produto não cumpre?

**Ataque as minhas premissas — duas Waves 0 desta sessão derrubaram premissas minhas e estavam certas:**

- Afirmo que o **#470/PR #475 não fecha isto**, porque rodei a sonda com o binário da `main` (que já
  tem o #475) e o braço D saiu `mls_complete: passed`. **Verifique por conta própria.** Se a causa for
  a mesma, esta REQ tem de ser absorvida, não aberta.
- Afirmo **2** arquivos com cerca aberta em 235. Reimplemente a contagem e diga se bate.
- Afirmo que **leniência não se aplica** porque isto é *usage error*, não *violation*. Se o projeto
  tiver precedente de leniência para erro de entrada, **diga** — muda o desenho.

**Critérios de aceite:**
- [ ] Tabela das 10 superfícies com o comportamento proposto e o **motivo** de cada um
- [ ] Resposta à pergunta 2 com o **menor corte** identificado
- [ ] Recontagem independente dos arquivos de cerca aberta, e o conteúdo real das caudas
- [ ] Veredito sobre o #470: mesma causa ou não, **com a medição**
- [ ] 🔴 Se a medição refutar qualquer premissa da REQ, **diga**
- [ ] Nenhuma linha de implementação escrita neste ML

**Gates da wave:**
```bash
trackfw barrier ROADMAP-2026-09-30-cerca-nao-terminada-mascara-ate-o-fim-do-arquivo-em-silencio-e-o-barrier-apaga-o-ml-pendente --wave 0 --trust-local-gates
```

## Wave 1 — a detecção
> 🔴 **Dependências: Wave 0 auditada.** Os MLs saem do veredito do ML-0A e da decisão sobre o
> comportamento do `serve` — **escritos depois, não antes.**

**Status:** ⬜ Pendente — aguardando Wave 0
