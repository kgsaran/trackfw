---
status: Open
date: 2026-08-18
author: "Zeus (Arquiteto)"
adr: ""
roadmap: ""
---

# REQ: contrato pinado no `cli-parity.md` sem gate nomeado é contrato não-aplicado

> Date: 2026-08-18 | Status: Open (backlog, sem roadmap)
| Linear Issue:
| Jira Issue:

## Motivation

A mesma lacuna reapareceu em **duas REQs seguidas**, e a segunda vez foi numa REQ escrita
justamente para não repetir a primeira.

- **2026-08-17, REQ de higiene, ML-2C:** a mensagem de artefato *unmanaged* é byte-idêntica nos 3
  CLIs — mas isso foi provado por **leitura do fonte**. Declarei como lacuna na tabela de
  não-correção e recomendei um gate. Não criei.
- **2026-08-18, REQ do `branch prune`, ML-2A:** a convergência do `ship` foi provada por **teste
  unitário por stack**, cada um com fixture git real. Prova comportamento **por runtime**, não que
  os três produzem a mesma saída. O AC8 daquela REQ existia exatamente para evitar isso, e mesmo
  assim precisou de um ML corretivo.

**A repetição é o dado.** Não foi desatenção pontual: não há mecanismo que force a existência do
gate, então ele depende de alguém lembrar — e a memória falha inclusive de quem escreveu o critério.

### O que a medição mostra

| | |
|---|---|
| comandos do CLI | 26 |
| scripts de paridade | 19 |
| comandos com gate **dedicado** | ~8 |
| seções de topo no `cli-parity.md` | 52 |
| seções que **nomeiam** o gate que as protege | **18** |

A cobertura cresceu **reativamente** — existe gate onde alguém já se queimou. E o documento de
contrato não sabe dizer quais dos seus próprios contratos estão de fato protegidos.

### A regra que falta, por analogia com uma que o projeto já aplica

O projeto já sustenta, no princípio **P4**: *gate sem cenário de falsificação é gate
não-verificado* — e por isso todo gate tem braço de detecção.

Falta a análoga, um nível acima:

> **Contrato pinado sem gate nomeado é contrato não-aplicado.**

## Escopo

1. **Cada seção de contrato do `cli-parity.md` declara qual gate a protege** — de forma legível por
   máquina, não em prosa livre.
2. **Meta-checker** que reprova quando uma seção de contrato não nomeia gate, ou nomeia script que
   não existe.
3. **Seção que não é contrato** (justificativa, histórico, nota de contexto) pode ser marcada como
   tal — **com motivo escrito**. Isso é ganho por si: força a distinção contrato/prosa a ser
   declarada em vez de presumida.

### O que **não** é escopo

- **Criar os gates faltantes.** Esta REQ cria o *mecanismo que revela* a ausência. Fechar cada
  lacuna é trabalho subsequente, priorizável, e provavelmente não vale para todas.
- **Exigir gate para tudo.** Algumas seções descrevem exceções intencionais ou contexto; a resposta
  correta ali é marcar como não-contrato, não inventar gate.
- Mudar comportamento de qualquer CLI.

## 🔴 O modo de falha previsível, nomeado

Alguém silencia o checker marcando tudo como não-contrato. É a saída fácil e destruiria o valor.

Mitigações a avaliar no roadmap: a marcação **exige motivo escrito**; a contagem de seções
não-contrato é reportada; e mudanças nessa marcação ficam visíveis no diff. Nenhuma impede o abuso
— tornam-no **visível**, que é a mesma postura que o projeto adota para o `credential-guard`
(detecção ancorada, não prevenção).

## Acceptance Criteria

- [ ] AC1 — Cada seção de contrato do `cli-parity.md` nomeia o gate que a protege, em formato
      legível por máquina.
- [ ] AC2 — Meta-checker reprova seção de contrato **sem** gate nomeado.
- [ ] AC3 — Meta-checker reprova gate nomeado que **não existe** no disco (aponta para o vazio).
- [ ] AC4 — Seção pode ser marcada **não-contrato** com motivo; sem motivo, reprova.
- [ ] AC5 — **Triagem das 52 seções** feita e registrada: quais são contrato, quais não, e quais são
      contrato **sem** gate — esta última lista é o produto mais valioso da REQ.
- [ ] AC6 — O checker entra no `make quality`.
- [ ] AC7 — Cenário de falsificação (P4) para o próprio checker: provar que ele reprova seção sem
      gate e gate inexistente. Um meta-checker sem falsificação seria a própria ironia.
- [ ] AC8 — `make quality` verde **e CI verde**.

## Riscos para quem executar

- **Triagem de 52 seções é o grosso do trabalho**, e é julgamento, não mecânica. Errar para
  "não-contrato" esvazia a REQ; errar para "contrato" gera lacunas falsas. Em caso de dúvida,
  marcar como contrato-sem-gate e deixar visível é a opção conservadora.
- **Não testar por leitura.** O checker precisa ser exercitado contra seções reais do documento.
- **Cuidado com o binário do `PATH`** — pode estar velho, e `--version` não distingue o build.

## Linked ADR
ADR: <!-- avaliar: a regra "contrato sem gate é contrato não-aplicado" pode merecer ADR próprio -->

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: <!-- sem roadmap; backlog -->
