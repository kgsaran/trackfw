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

**How to apply:** o conserto tem duas pernas — (1) mover o cenário antigo para uma regra que **ainda**
usa o leitor antigo (`req_has_adr`, `wip_has_req`, `blocked_has_req` usam `contentHasMarkerValue`), e
(2) acrescentar um cenário novo que sabota o leitor **novo** com fixture que só ele discrimina. Dois
detalhes já pagos: a fixture de prosa precisa citar o **caminho** do artefato (`adr_orphan` casa por
`strings.Contains` do basename e reprovaria os dois braços), e sabotar
`EqualFold(TrimSpace(key), field)` **só compila como disjunção** (substituir deixa `key` sem uso).
Relacionado: [[grep-scripts-inclui-lista-de-argumentos]], [[metrica-por-artefato]].
