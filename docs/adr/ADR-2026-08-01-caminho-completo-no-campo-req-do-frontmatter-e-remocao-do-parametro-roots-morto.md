---
status: Accepted
date: 2026-08-01
author: "Zeus"
---

# ADR: Caminho completo no campo req do frontmatter e remocao do parametro roots morto

> Date: 2026-08-01 | Status: Accepted

## Context

`trackfw roadmap new --from-req <req>` produz um roadmap que o `validate` reprova, mesmo com a
REQ existindo e tudo corretamente linkado. Reproduzido em diretório limpo em 2026-08-01:

```
$ trackfw roadmap new --from-req docs/req/REQ-2026-08-01-fonte-de-teste.md
✓ created docs/roadmaps/backlog/ROADMAP-2026-08-01-fonte-de-teste.md

frontmatter → req: "REQ-2026-08-01-fonte-de-teste.md"          ← basename
corpo       → REQ: docs/req/REQ-2026-08-01-fonte-de-teste.md   ← correto

$ trackfw roadmap move ... wip && trackfw validate
✗ roadmap "..." links to REQ "REQ-2026-08-01-fonte-de-teste.md" which does not exist
```

O caminho `roadmap new --title --req <path>` **não** é afetado — só o `--from-req`.

Três causas independentes se somam:

1. **O gerador grava só o basename.** `internal/generators/roadmap.go:175` usa
   `filepath.Base(reqPath)` no campo `req:` do frontmatter, enquanto o corpo recebe o caminho
   completo. Espelhado em `npm/src/generators/roadmap.js:517` e
   `pypi/trackfw/generators/roadmap.py:337`.
2. **`extractRefPath` retorna no primeiro match.** `internal/validator/validator.go:1426` varre
   linha a linha e devolve o primeiro valor terminado em `.md` cujo campo case-insensitive bata
   com `REQ`. O `req:` do frontmatter **precede** o `REQ:` do corpo — então o valor lido é sempre
   o basename, independentemente do que o corpo diga.
3. **`referenceExists` ignora o parâmetro `roots`.** `internal/validator/validator.go:1491`
   recebe `roots` e nunca o usa: faz apenas `os.Stat(ref)` relativo ao cwd. Como `ref` chega como
   basename, o `Stat` sempre falha. Os três chamadores passam `roots` (`cfg.REQDir`,
   `cfg.ADRDirs`, `cfg.RoadmapDir`) acreditando que serve para algo. Mesmo padrão em
   `npm/src/validator/index.js:781` e `pypi/trackfw/validator.py:968`.

Diagnóstico registrado em
`vault/notes/roadmap-from-req-ref-targets-exist-falso-positivo-2026-08-01.md`, depois de dois
agentes reportarem o sintoma sem causa raiz.

A questão a decidir: **qual é o contrato do campo `req:`** — caminho completo, ou basename que o
validador resolve contra as raízes configuradas?

## Decision

**O contrato é caminho relativo completo. O parâmetro `roots` morto é removido.**

1. Os três geradores gravam `reqPath` completo no campo `req:` do frontmatter, não o basename.
2. `referenceExists` / `reference_exists` **perde o parâmetro `roots`** nos três CLIs, e os
   chamadores são ajustados. A validação permanece **estrita**: um `req:` com basename escrito à
   mão continua reprovando, e isso é o comportamento desejado.
3. `extractRefPath` **não muda**. A precedência do frontmatter sobre o corpo está correta — o
   frontmatter é o campo canônico. Corrigido o item 1, ele passa a ler o valor certo.

## Consequences

**Positivas**

- `roadmap new --from-req` passa a produzir artefato que valida limpo, fechando o terceiro caso
  seguido de "a ferramenta reprova o que ela mesma gerou".
- Remove uma armadilha real de leitura: hoje o código *parece* resolver contra as raízes, e três
  chamadores passam `roots` de boa-fé. Quem lê acredita que funciona; só rastreando o valor de
  `ref` se descobre que não.
- Contrato único e explícito para o campo `req:`, sem ambiguidade entre basename e caminho.

**Negativas / aceitas**

- Mexe na assinatura de uma função de validação nos três CLIs, com ajuste de três chamadores em
  cada. Mudança mecânica, mas amplia o diff.
- Fecha a porta para tolerar basename escrito à mão. Aceito: tolerância implicaria busca em
  múltiplas raízes com resolução ambígua quando o mesmo nome existir em mais de uma.
- Roadmaps já existentes com `req:` em basename passariam a reprovar. **Verificado:** não há
  nenhum neste repositório — `validate` está verde. Projetos downstream que tenham o artefato
  gerado pelo caminho quebrado precisarão corrigir o campo; é o custo de tornar o contrato
  explícito, e a mensagem de violação já aponta o arquivo.

**Ponto de atenção herdado**

O cenário `roadmap-acceptance-heading/*/from-req` de `scripts/check-gates-falsify.sh` roda hoje
com `ref_targets_exist` **co-ocorrendo** com a violação que ele de fato verifica. O
`assert_fails_with` casa a substring de `wip_acceptance`, não a ausência de outras violações —
logo esta correção não deve quebrá-lo, apenas reduzir o cenário corrompido de duas violações para
uma. **Precisa ser confirmado empiricamente na barreira**, não presumido.

## Alternatives Considered

**Corrigir só o gerador, deixando `roots` morto** — menor superfície. **Rejeitado:** resolve o
sintoma e deixa a armadilha. O próximo agente que ler `referenceExists(ref, roots)` vai assumir
que a resolução por raiz existe, e pode construir em cima de uma garantia inexistente.

**Corrigir o gerador e fazer `referenceExists` usar `roots` de fato** — defesa em profundidade,
tolerando basename escrito à mão. **Rejeitado por decisão do usuário:** afrouxa a validação e
introduz ambiguidade quando o mesmo basename existe em mais de uma raiz (`ADRDirs` é plural).
Preferiu-se contrato estrito e explícito.

**Mudar `extractRefPath` para preferir o `REQ:` do corpo** — o corpo já tinha o valor certo.
**Rejeitado:** trata o sintoma no lugar errado e inverte a precedência natural. O frontmatter é o
campo estruturado e canônico; o corpo é prosa. Corrigir o gerador é a correção na origem.
