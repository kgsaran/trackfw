---
status: wip
date: 2026-08-20
req: "docs/req/REQ-2026-08-18-contrato-pinado-no-cli-parity-sem-gate-nomeado-e-contrato-nao-aplicado.md"
adr: "docs/adr/ADR-2026-08-20-anotacao-de-cobertura-de-contrato-no-cli-parity.md"
squad: "apolo-tf, hefesto-tf"
---

# Roadmap: contrato pinado no `cli-parity.md` sem gate nomeado

> Created: 2026-08-20 | Status: wip

## Context

REQ: `docs/req/REQ-2026-08-18-contrato-pinado-no-cli-parity-sem-gate-nomeado-e-contrato-nao-aplicado.md`

A regra que falta, por analogia com o **P4** que o projeto já sustenta (*gate sem cenário de
falsificação é gate não-verificado*):

> **Contrato pinado sem gate nomeado é contrato não-aplicado.**

### Por que agora, e não como higiene de fim de sprint

A REQ foi aberta em 2026-08-18 com **duas** instâncias medidas. Entre aquela data e hoje, a lacuna
produziu mais evidência do que a própria REQ tinha quando foi escrita:

| evidência acumulada | onde |
|---|---|
| `--json` do `doctor`: Go emitia `null`, Node/Python `[]` | ML-2B do `doctor` |
| relatório de texto do `doctor`: linha em branco só no Go | ML-2B do `doctor` |
| `exec.Command().Output()` do Go descartava o stderr do filho | ML-1B do force-push |
| erro de git no fallback do Python divergente | ML-2A do `release tag` |
| timestamp com milissegundos no Node | ML-2A do `release tag` |

**Cinco divergências reais em três dias**, nenhuma detectável por teste por stack — cada runtime
concorda consigo mesmo. Todas apareceram só quando alguém escreveu um gate comparando as **três
saídas reais**. Enquanto não houver mecanismo que force a existência do gate, isso depende de alguém
lembrar.

### Medição de hoje (2026-08-20), refeita — não a da REQ

```
seções de topo (##) no cli-parity.md : 53
subseções (###)                      : 122
scripts check-*.sh                   : 27
```

A contagem de "seções que nomeiam gate" da REQ (18 de 52) precisa ser **refeita** pelo executor: o
documento cresceu desde então, e três seções novas entraram já nomeando o gate.

## 🔴 Riscos que valem para todos os MLs

1. **O modo de falha previsível é silenciar o checker** marcando tudo como não-contrato. Nenhuma
   mitigação impede o abuso; elas o tornam **visível**. É a mesma postura do `credential-guard`:
   detecção ancorada, não prevenção.
2. **Super-marcar como contrato** gera lacunas falsas e ruído que treina o leitor a ignorar. Em
   dúvida, marcar como contrato-sem-gate e deixar visível é a opção conservadora — mas dúvida
   sistemática é sinal de que o critério está mal definido, não de que se deve chutar.
3. **A triagem é julgamento, não mecânica.** É o grosso do trabalho e o produto mais valioso.
4. **Não testar por leitura.** O checker precisa ser exercitado contra seções reais do documento.
5. **O checker é um gate** — logo, ele mesmo precisa de cenário P4. Meta-checker sem falsificação
   seria a própria ironia.

---

## Wave 1 — Formato e mecanismo (2 MLs, sequenciais)

### ML-1A — Aplicar o formato em 3 seções-piloto
**Status:** ⬜ Pendente · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`)
**Arquivos:** `docs/cli-parity.md` (apenas 3 seções-piloto).

> **O ADR foi escrito por mim, não delegado** — decisão de formato é do arquiteto, e o roadmap
> original o atribuía ao executor por engano. Formato decidido em
> `ADR-2026-08-20-anotacao-de-cobertura-de-contrato-no-cli-parity.md`:
> ```
> <!-- trackfw-contract: gate=scripts/check-doctor-parity.sh -->
> <!-- trackfw-contract: none reason=<motivo em uma linha> -->
> ```
> Resta ao ML-1A **aplicar** e provar que o formato aguenta os três casos.

**Ação:** decidir e registrar em **ADR** o formato pelo qual uma seção declara o gate que a protege,
e pelo qual uma seção se declara **não-contrato com motivo**. Aplicar em **3 seções-piloto** de
naturezas diferentes — uma com gate óbvio, uma sem gate, uma que é prosa — para provar que o formato
aguenta os três casos antes de virar 53.

**Critérios de aceite:**
- [x] Formato decidido em ADR, com o motivo da escolha — feito por mim
- [ ] 3 seções-piloto anotadas, cobrindo os **três** casos: com gate, sem gate, não-contrato
- [ ] A escolha de cada piloto é **justificada** — piloto fácil demais não prova nada
- [ ] Nenhuma mudança de comportamento de CLI, nenhum gate criado

### ML-1B — Meta-checker
**Status:** ⬜ Pendente · **Agente:** `apolo-tf` · **Dependência:** ML-1A
**Arquivos:** `scripts/check-parity-contract-coverage.sh` (novo), `Makefile` (alvo `parity`),
`scripts/check-gates-falsify.sh`.

**Ação:** checker que reprova quando (a) seção de contrato **não nomeia** gate, (b) nomeia gate que
**não existe** no disco, (c) marcação de não-contrato **sem motivo**.

Enquanto a triagem da Wave 2 não terminar, o checker roda em **modo relatório** — conta e lista, sem
reprovar. Vira bloqueante no ML-3A. Sem isso o `make quality` fica vermelho durante toda a triagem, e
gate vermelho por semanas é gate que se aprende a ignorar.

**Critérios de aceite:**
- [ ] Reprova seção de contrato sem gate nomeado
- [ ] Reprova gate nomeado inexistente — **aponta para o vazio**
- [ ] Reprova não-contrato sem motivo
- [ ] Modo relatório enquanto a triagem não fecha; conta e lista
- [ ] Cenário P4 do próprio checker: baseline + detecção para os três casos
- [ ] Exercitado contra seções **reais** do documento, não fixture sintético
- [ ] `make quality` verde

---

## Wave 2 — Triagem (o grosso do trabalho)

### ML-2A — Triagem das seções
**Status:** ⬜ Pendente · **Agente:** `hefesto-tf` (`subagent_type: hefesto-tf`) · **Dependência:** ML-1B
**Escreve:** anotações em `docs/cli-parity.md` e o relatório de triagem.

**Ação:** classificar **cada** seção em contrato-com-gate, contrato-sem-gate, ou não-contrato-com-motivo.
Refazer a contagem: a da REQ (18 de 52) está defasada.

**O produto mais valioso desta REQ é a lista de contratos SEM gate.** Ela não é subproduto da
triagem — é o entregável.

**Critérios de aceite:**
- [ ] Todas as seções classificadas
- [ ] Lista de contrato-sem-gate produzida e registrada, ordenada por risco
- [ ] Cada não-contrato tem motivo escrito
- [ ] Contagem de não-contratos reportada pelo checker — o abuso fica visível
- [ ] `make quality` verde

---

## Wave 3 — Tornar bloqueante

### ML-3A — Checker vira bloqueante
**Status:** ⬜ Pendente · **Agente:** `apolo-tf` · **Dependência:** ML-2A
**Critérios de aceite:**
- [ ] Checker reprova de verdade; `make quality` verde porque a triagem fechou, não porque o checker
      é permissivo
- [ ] Seção nova sem anotação **reprova** — provado por cenário
- [ ] CI verde

---

## Notas
- **Fora de escopo, declarado:** criar os gates faltantes. Esta REQ cria o **mecanismo que revela** a
  ausência; fechar cada lacuna é trabalho subsequente e priorizável, e provavelmente não vale para
  todas.
- **Fora de escopo:** exigir gate para tudo. Seção que descreve exceção intencional ou contexto deve
  ser marcada como não-contrato, não ganhar gate inventado.
- Commits e branch são exclusivos do `trackfw_architect`.
