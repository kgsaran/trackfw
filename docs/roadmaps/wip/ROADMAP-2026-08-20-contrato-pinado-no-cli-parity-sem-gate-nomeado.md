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
**Status:** ✅ Concluído · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`)
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
- [x] 3 seções-piloto anotadas, cobrindo os **três** casos: com gate, sem gate, não-contrato
- [x] A escolha de cada piloto é **justificada** — piloto fácil demais não prova nada
- [x] Nenhuma mudança de comportamento de CLI, nenhum gate criado

> **Achado do executor (Apolo), pendente de decisão do arquiteto antes do ML-2A:** o formato do
> ADR só define `gate=<caminho>` e `none reason=<motivo>` — não há forma explícita para
> "contrato sem gate", o caso mais valioso da REQ. Anotado como `gate=` (chave documentada, valor
> vazio) por não inventar sintaxe nem fabricar caminho de script inexistente. Ver
> `docs/agents-working-context.md`, sessão 2026-08-20 (Apolo), para a medição completa
> (`####` — 17 headers não contados na REQ/roadmap — e o exemplo de gate parcial em
> `## Vault de conhecimento`, que também revelou `note_orphan` ausente no validator do Node.js).

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

### Auditoria do ML-1A — aprovada, e o piloto **pagou-se** antes de escalar

Era exatamente para isto que o lote existia: descobrir barato que o formato não aguentava. Descobriu.

**Confirmei as três medições por conta própria:**

```
niveis de titulo:  ## 53 · ### 122 · #### 17      <- o #### nao existia na REQ nem no roadmap
note_orphan:       Go 3 ocorrencias · Python 4 · Node ZERO
```

**Achado 1 — o formato tinha dois estados e o caso central da REQ é um terceiro.** Não havia forma
para *"isto é contrato e nada o protege"*. O executor contornou com `gate=` vazio e **sinalizou a
decisão em vez de escondê-la** — escolha certa diante da alternativa de inventar caminho de script,
que a própria ADR chama de carimbo. Mas valor vazio é indistinguível de **omissão**, e o checker não
separaria "declarei a lacuna" de "esqueci de preencher". Resolvido na **Emenda 1**: estado `gap`
próprio, greppável e **contável** — a contagem de `gap` é o produto da REQ e precisa ser um número
que se acompanhe cair.

**Achado 2 — três níveis de título, não dois.** E o estado de contrato **não acompanha a
profundidade**: há `####` de não-contrato dentro de `##` de contrato. O universo da triagem é **~192,
não 175**. O ML-2A está subdimensionado no roadmap e precisa ser refatiado.

**Achado 3 — cobertura parcial não era expressável.** Medido no piloto 2: o gate cobre a mecânica de
criação de nota mas não a semântica da regra. Colapsava em vazio. Emenda 1 acrescenta `partial=`.

**Achado 4 — regra de desempate.** Seção que se autodeclara não-contrato e mesmo assim fixa fato
falsificável. Emenda 1: **fato falsificável sobre comportamento de CLI ⇒ é contrato**; a
autodeclaração não prevalece.

#### O achado lateral é a melhor evidência que esta REQ podia ter

`note_orphan` existe em Go e Python e **está ausente do CLI Node**, com `cli-parity.md:147`
documentando-a como contrato. Violação viva da regra dura de paridade.

E o modo como apareceu é o argumento: **bastou alguém perguntar "qual gate protege esta seção?"**.
Não houve investigação — a pergunta que o mecanismo faz produziu a descoberta antes de o mecanismo
existir. Aberta a `REQ-2026-08-20-note-orphan-existe-em-go-e-python-e-esta-ausente-do-cli-node`
(backlog), com escopo negativo explícito: **não** varrer as outras regras à mão, porque é justamente
isso que o ML-2A vai fazer de forma sistemática.

`make quality` exit 0 · `validate` exit 0.

---

### ML-1A-bis — Reaplicar os 3 pilotos no formato da Emenda 1
**Status:** ⬜ Pendente · **Agente:** `apolo-tf` · **Dependência:** Emenda 1 (feita).
Trocar o `gate=` vazio do piloto 2 por `gap reason=...` (ou `gate=... partial=...`, o que a medição
dele indicar), e revisar o piloto 3 sob a regra de desempate. Lote de minutos; existe para o ML-1B
codificar contra o formato **final**, não contra o provisório.

---

## Wave 2 — Triagem (o grosso do trabalho)

### ML-2A — Triagem das seções
**Status:** ⬜ Pendente · **Agente:** `hefesto-tf` (`subagent_type: hefesto-tf`) · **Dependência:** ML-1B
**Escreve:** anotações em `docs/cli-parity.md` e o relatório de triagem.

**Ação:** classificar **cada** seção nos **três** estados da Emenda 1 (`gate=`, `gap`, `none`),
mais `partial=` onde couber. Refazer a contagem: a da REQ (18 de 52) está defasada.

🔴 **Refatiar antes de começar.** O universo medido é **~192** (`##` 53 · `###` 122 · `####` 17), não
175. Triagem de 192 seções num ML só é grande demais para auditar bem — dividir por faixas do
documento, com cada lote auditável de forma independente.

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
