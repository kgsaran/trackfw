---
name: migrar-leitor-de-regra-esvazia-falsify
description: Trocar o leitor de uma regra do validator (ex. contentHasMarkerValue → extractRefPath) esvazia os cenários de check-gates-falsify que sabotavam o leitor antigo por conta daquela regra — procure pela MENSAGEM, não pelo nome da função
metadata:
  type: feedback
---

Antes de trocar o mecanismo de leitura de uma regra do validator, **grepe
`scripts/check-gates-falsify.sh` pela MENSAGEM da regra** (`S192_MSG_ROADMAP='has no linked Roadmap'`),
não pelo nome da função. Todo cenário que sabota o leitor **antigo** e exige que aquela mensagem
desapareça fica **vácuo** — a sabotagem deixa de mudar o veredito, a violação sobrevive e
`assert_lacks_pattern` reprova **sem que nada esteja quebrado**.

**Why:** medido no ML-1D de 2026-09-26 (REQ-2026-09-09). `req_has_roadmap` migrou de
`contentHasMarkerValue` para `extractRefPath`; a Direção B do Cenário 192 sabotava a ancoragem por
linha de `contentHasMarkerValue` e esperava `has no linked Roadmap` desaparecer. E **nenhuma
composição salva**: com o `.md` exigido, a prosa passa a ser recusada por duas razões independentes
(sem ancoragem **e** sem `.md`), então neutralizar só a ancoragem nunca vira verde.

**How to apply:** (1) se ainda houver regra no leitor antigo, mover o cenário para ela; (2)
acrescentar cenário que sabota o leitor **novo** com fixture que só ele discrimina.

🔴 **Atualizado no ML-1E (2026-09-26): a perna (1) ESGOTOU.** As três últimas regras
(`req_has_adr`, `wip_has_req`, `blocked_has_req`) migraram, `contentHasMarkerValue` ficou sem chamador
de produção e as Direções **A e B** do Cenário 192 ficaram as duas vácuas de uma vez. O conserto que
funcionou: **A** passou a sabotar a exigência de `.md` dentro de `extractRefPath`
(`if strings.HasSuffix(v, ".md")` → `if v != ""`), e **B** reusa o binário corrompido da **Direção C**
(mesmo seam, campo diferente) em vez de compilar um quarto binário. ⚠️ Sabotar `extractRefPath` tem um
efeito colateral inevitável: `ref_targets_exist` passa a enxergar a referência lixo e emite
`links to ADR "<!--" which does not exist`, então **exit 0 é inalcançável** naquele braço — use
`assert_output_lacks` (não exige rc 0) **mais** um liveness anchor com `assert_output_contains` sobre a
mensagem colateral, ou o braço fica infalsificável.

🔴 **E a régua da MENSAGEM tem um ponto cego, medido no mesmo ML:** ela acha os cenários que
**asseveram** a mensagem da regra migrada; **não** acha os que passam a emiti-la como **efeito
colateral** e morrem no `rc != 0` de `assert_lacks_pattern`. Foi o Cenário 28 (backtick): a fixture
referencia o ADR só pelo corpo e só entre backticks, então a corrupção que esconde a referência de
`adr_accepted_when_req_done` esconde também o vínculo de `req_has_adr`. Só o `make quality` achou.
**Rastreio complementar:** liste todo `corrupt_literal` cujo alvo esteja DENTRO do extrator
compartilhado (`extractRefPath`) e reveja cada braço — foram 3 dos 3 nesta campanha.

Dois detalhes já pagos: a fixture de prosa precisa citar o **caminho** do artefato (`adr_orphan` casa por
`strings.Contains` do basename e reprovaria os dois braços), e sabotar
`EqualFold(TrimSpace(key), field)` **só compila como disjunção** (substituir deixa `key` sem uso).
Relacionado: [[grep-scripts-inclui-lista-de-argumentos]], [[metrica-por-artefato]].
