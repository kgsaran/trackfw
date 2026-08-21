---
status: wip
date: 2026-08-21
req: "docs/req/REQ-2026-08-21-regra-de-verbosidade-das-respostas-do-arquiteto-no-asset-e-nas-regras-semeadas.md"
adr: ""
squad: "apolo-tf"
---

# Roadmap: regra de verbosidade no asset do arquiteto e nas regras semeadas

> Created: 2026-08-21 | Status: wip

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
**Status:** ⬜ Pendente · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`)
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
