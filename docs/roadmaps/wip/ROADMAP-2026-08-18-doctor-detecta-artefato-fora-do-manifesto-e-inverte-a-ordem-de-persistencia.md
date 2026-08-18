---
status: wip
date: 2026-08-18
req: "docs/req/REQ-2026-08-17-doctor-detecta-artefato-em-disco-ausente-do-manifesto-apos-janela-de-gravacao-parcial.md"
adr: "docs/adr/ADR-2026-08-18-ordem-de-persistencia-inverte-para-manifesto-antes-dos-artefatos.md"
squad: "apolo-tf, hades-tf"
---

# Roadmap: `doctor` detecta artefato fora do manifesto, e a ordem de persistência inverte

> Created: 2026-08-18 | Status: wip — **Wave 1 entregue em PR próprio; Waves 2 e 3 pendentes**

> 🔴 **Entrega parcial deliberada.** A Wave 1 (inversão da ordem) foi para PR sozinha porque é
> autocontida e impede casos novos desde já. **A REQ NÃO está fechada:** o `doctor` (AC1–AC4) e a
> barreira ainda não existem, e instalações que já estão no estado ruim seguem sem detecção.

## Context

REQ: `docs/req/REQ-2026-08-17-doctor-detecta-artefato-em-disco-ausente-do-manifesto-apos-janela-de-gravacao-parcial.md`
ADR: `docs/adr/ADR-2026-08-18-ordem-de-persistencia-inverte-para-manifesto-antes-dos-artefatos.md`

Origem: bug de KG no CMDB — 12 arquivos em disco, 10 no manifesto, e o `agents update --force`
recusando com `unmanaged artifact`. O comportamento estava certo; o **estado** é que não deveria
existir.

Duas frentes, e a **ordem importa**: a inversão da frente 2 impede casos novos, mas **não** conserta
instalações que já estão no estado ruim. O `doctor` é o que revela essas.

## Acceptance Criteria
- [ ] AC1 — Detecta **arquivo em disco ausente do manifesto** e o distingue de **arquivo modificado à mão**.
- [ ] AC2 — A saída **nomeia o remédio**, com comando pronto para copiar.
- [ ] AC3 — Paridade nos 3 CLIs, com **gate comparando saídas reais** — não por leitura de fonte.
- [ ] AC4 — Cenário P4 reproduzindo a janela: artefato em disco sem registro, e prova de que acusa.
- [ ] AC5 — Não-regressão: `update` **continua recusando** bytes unmanaged mesmo com `--force`.
- [ ] AC6 — Decisão sobre a janela registrada em ADR. ✅ **feito** — `ADR-2026-08-18`, inverter a ordem.
- [x] AC7 — Inversão implementada, com rollback preservado em erro normal. Evidência: ML-1A.
- [ ] AC8 — `make quality` verde **e CI verde**.

## 🔴 Riscos que valem para todos os MLs

1. **Falso-positivo é o risco dominante do `doctor`.** Acusar artefato legítimo treina o usuário a
   ignorar a saída. A comparação é **por conteúdo contra o template do catálogo**, não por presença
   de arquivo.
2. **A frente 2 é o caminho de escrita de TODO `install`/`update`.** Qualquer regressão afeta tudo.
3. **Fixture com manifesto de fato incompleto**, nunca mock — é o estado que se quer detectar.
4. **`make quality` verde localmente não fecha AC** — o AC8 exige CI. Já errei isso nesta série.
5. **AC3 não se fecha com teste por stack.** Exige gate comparando as três saídas reais; foi
   exatamente a lacuna que virou ML corretivo nas duas REQs anteriores.

---

## Wave 1 — Inversão da ordem (impede casos novos)

### ML-1A — Persistir manifesto antes dos artefatos
**Status:** ✅ Concluído · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`)
**Arquivos:** `internal/integrations/manager.go` (`mutate`) + espelhos Node/Python, testes dos 3.

**Ação:** trocar a ordem dos dois laços de `mutate` — persistir os manifestos **antes** de escrever
os bytes. Ver o ADR para o raciocínio: cada escrita já é atômica, a janela é só de ordem, e a
direção invertida é auto-reparável (`StateNotInstalled`) em vez de exigir humano (`unmanaged`).

**Decisão sobre `uninstall` (registrada com justificativa, conforme pedido no handoff):**
`uninstall` foi deliberadamente **NÃO invertido** — mantém a ordem pré-existente (bytes removidos
primeiro, manifesto persistido depois). Regra geral que decide os dois casos: persistir o lado que
torna o manifesto um **superset** do disco. Para install/update isso é manifesto-primeiro (uma
interrupção deixa o manifesto declarando um artefato ainda ausente → `StateNotInstalled`,
auto-reparável). Para uninstall, inverter da mesma forma (remover a entrada do manifesto antes de
remover os bytes) produziria a direção **ruim**: uma interrupção deixaria um arquivo íntegro em
disco, com conteúdo que ainda bate com o template do catálogo, mas **sem nenhum registro no
manifesto** — resolve para `StateCurrent`/`managed=false`, um artefato órfão que parece legítimo e
que nada detecta ou repara automaticamente. É exatamente a direção "disco à frente do manifesto"
que o ADR existe para eliminar. Comentário equivalente está no código dos 3 CLIs para não ser
"simetrizado" por engano depois.

**Critérios de aceite:**
- [x] Interrupção simulada entre as fases deixa **manifesto à frente**, e `install`/`update` repara sozinho.
- [x] **Rollback preservado**: erro normal no meio do lote restaura arquivos **e** manifestos.
- [x] Não-regressão: `install`, `update` e `uninstall` inalterados no caminho feliz, nos 3 CLIs.
- [x] Cenário P4 com baseline e detecção.
- [x] `make quality` verde.

---

### Auditoria do ML-1A — a assimetria do ADR, provada nos dois sentidos

Medida por mim em projeto real e descartável, **não por leitura**:

```
manifesto a frente (direcao nova)     agents install         -> REPARA sozinho
disco a frente + deriva (direcao ANTIGA, o caso do CMDB)
                                      agents install         -> RECUSA: "is modified; use force"
                                      agents install --force -> exige decisao humana
```

É exatamente o que o ADR afirmou. A primeira tentativa da minha auditoria falhou por **erro meu**
(usei `--install-missing`, que é flag do `trackfw update`, não do `agents update`), e a segunda não
reproduziu o caso ruim porque faltava a **deriva de conteúdo** — sem ela o `install` adota o arquivo.
O caso real do CMDB exige disco-à-frente **somado** a conteúdo que deixou de bater com o template.

**Decisão sobre o `uninstall`: melhor do que eu pedi.** Eu pedi que decidisse e justificasse; ele
derivou a **regra geral** que decide os dois casos — *persistir o lado que torna o manifesto um
superset do disco*. Para install/update isso é manifesto-primeiro; para uninstall, inverter
produziria a direção **ruim** (arquivo íntegro, sem registro, parecendo legítimo e sem reparo
automático). O comentário está no código dos 3 CLIs para não ser "simetrizado" por engano depois.

**Bug pré-existente corrigido de passagem:** o laço de rollback do Python **não** engolia erro por
item, ao contrário de Go e Node. Um restore falhando abortava antes de restaurar o resto — inclusive
o manifesto. Agora espelha os outros dois. Era exatamente o risco 2 do handoff: quebrar o rollback
trocaria uma falha rara de interrupção por uma falha comum de erro.

`make quality` exit 0 · 131 cenários · `validate` exit 0.

---

## Wave 2 — `doctor` (revela o estado ruim já existente)

### ML-2A — Comando/regra que detecta artefato fora do manifesto
**Status:** ⬜ Pendente · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`)
**Dependência:** ML-1A — a inversão muda o estado que o `doctor` vai encontrar.

**Ação:** detectar **arquivo cujo conteúdo bate com o template do catálogo e que está ausente do
manifesto** — isso não é adulteração, é escrita não registrada, e o remédio é diferente. Distinguir
de **arquivo modificado à mão**, que continua sendo o caso de `install --force`.

**Critérios de aceite:**
- [ ] As duas classes são distinguidas e têm remédios diferentes; não podem ser fundidas.
- [ ] A saída nomeia o remédio com comando pronto para copiar.
- [ ] **Não acusa** artefato legítimo — risco 1.
- [ ] Gate comparando as **três saídas reais**; teste por stack não fecha o AC3.
- [ ] Cenário P4 reproduzindo a janela.
- [ ] `make quality` verde.

---

## Wave 3 — Barreira

### ML-3A — `hades-tf`: revisão da inversão e do diagnóstico
**Status:** ⬜ Pendente · **Agente:** `hades-tf` (`subagent_type: hades-tf`)
**Escreve:** `docs/seguranca/2026-08-18-revisao-do-doctor-e-da-inversao.md`

**Ações:** a inversão mexe no caminho de escrita de tudo — avaliar se abre caminho para o produto
sobrescrever bytes que não escreveu, ou para o manifesto declarar como gerenciado algo que não é.
Avaliar se o `doctor` pode ser induzido a chamar de "escrita não registrada" um artefato adulterado
de fato — o que rebaixaria adulteração a acidente. **Veredito explícito; bloquear é saída legítima.**

---

## Notas
- **Fora de escopo, declarado:** WAL/journal cross-file — rejeitado no ADR por desproporção.
- **Fora de escopo:** afrouxar o `preflight`; ele recusar bytes desconhecidos é correto.
- Commits e branch são exclusivos do `trackfw_architect`.
