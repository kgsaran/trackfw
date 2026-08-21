---
status: done
date: 2026-08-21
req: "docs/req/REQ-2026-08-21-regra-de-verbosidade-das-respostas-do-arquiteto-no-asset-e-nas-regras-semeadas.md"
adr: ""
squad: "apolo-tf"
---

# Roadmap: regra de verbosidade no asset do arquiteto e nas regras semeadas

> Created: 2026-08-21 | Status: done

## Context

REQ: `docs/req/REQ-2026-08-21-regra-de-verbosidade-das-respostas-do-arquiteto-no-asset-e-nas-regras-semeadas.md`

Feedback de KG. O argumento é de **atenção**, não de custo: relatório longo torna o achado importante
indistinguível do resto — a mesma falha de sinal ruidoso que a série de gates combateu.

## 🔴 Risco que vale para todos os MLs

**Encurtar demais esconde bloqueio.** Os três gatilhos de escalada (bloqueio · decisão do usuário ·
erro próprio) não são decoração: são o que impede a regra de virar silêncio conveniente. Se um ML
mexer neles, é achado.

---

## Wave 1 — Texto e cobertura (1 ML)

### ML-1A — Regra nos dois lugares, com gate estendido
**Status:** ✅ Concluído · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`)
**Arquivos:** `internal/integrations/assets/agents/architect.md` + espelhos,
`internal/generators/agentfiles.go`, `npm/src/generators/init.js`,
`pypi/trackfw/generators/init_gen.py`, gate de paridade **existente**, `docs/cli-parity.md`.

**Ação:** inserir a regra da REQ (padrão curto · três gatilhos de escalada · o que nunca se corta ·
o que se corta) no asset do arquiteto e no `CLAUDE.md` semeado, byte-idêntico nos 3 CLIs.

**Antes de escrever gate novo, verificar se já existe** cobrindo o asset e o `CLAUDE.md` semeado —
nesta série um comparador paralelo quase foi criado sem necessidade, e o lote de investigação que
evitou isso se pagou.

**Critérios de aceite:**
- [ ] Regra no asset e no `CLAUDE.md` semeado, byte-idêntica nos 3
- [ ] Os três gatilhos e a lista do que nunca se corta estão explícitos
- [ ] Gate **existente** estendido, não paralelo; se não existir, criar e dizer por quê
- [ ] Cenário P4 com baseline e detecção
- [ ] Anotação `trackfw-contract` atualizada; checker de cobertura exit 0
- [ ] `make quality` verde · invocação CI-exata verde

---

## Notas
- **Fora de escopo:** verbosidade dos executores ao reportar para o arquiteto — outro canal, outro
  destinatário, e o relatório detalhado deles é o que torna a auditoria possível.
- **Fora de escopo:** controle configurável. A REQ registra o motivo: botão é ajustado uma vez e
  esquecido no valor errado.
- Commits e branch são exclusivos do `trackfw_architect`.

---

### Auditoria do ML-1A — aprovada

```
sabotagem propria: "Depth is on demand from the user." -> "Depth is always maximum."
  gate -> EXIT=1: "artifact parity drift: CLAUDE.md ## Architect responses differs
                   between go and node" (idem go/python)
restaurado -> EXIT=0
153 cenarios · make quality (CI-exata) exit 0 · cobertura exit 0 · validate exit 0
```

**Risco residual dele, resolvido:** ele declarou que o `make quality` estava rodando quando o
contexto acabou e pediu confirmação. Rodei a invocação CI-exata completa — exit 0, zero FAIL.
Declarar o que ficou por confirmar, em vez de afirmar verde, é o comportamento certo.

**Gate estendido, não paralelo** — `check-artifact-parity.sh`, que já cobria o `CLAUDE.md` semeado.
Era o que eu tinha pedido para verificar antes de criar.

**A redação passa no teste que mais me preocupava.** Eu tinha escrito no handoff: *"se a sua redação
puder ser lida como 'reporte menos', está errada"*. A dele inclui a salvaguarda de forma literal:

> *"A response that buries a blocker in paragraph seven produced the same effect as not reporting it."*

Os três gatilhos estão qualificados, não genéricos — bloqueio **que para a próxima wave**, decisão
**que não se infere do contexto**, erro **que não se autocorrige**. Sem essas qualificações, "erro do
agente" viraria escada para escalar sempre.
