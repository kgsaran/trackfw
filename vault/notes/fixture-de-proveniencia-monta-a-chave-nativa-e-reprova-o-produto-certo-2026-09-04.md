---
title: A fixture de proveniência montava a chave com separador nativo — no Windows ela reprovava o produto CERTO, e no Node passava por acidente
date: 2026-09-04
author: apolo-tf
tags: [windows, separador, fixture, paridade, thirdparty, provenance]
---

# Fixture de proveniência: chave nativa vs. produção que grava `/`

## O que parecia

O `ADR-2026-09-04` e o handoff do ML-2A diziam: *"`provenanceKey` — Go e Python já normalizam; só o
Node não — e por isso ele **passa por acidente**, com fixture nativa e produto sem normalização
casando entre si."*

A leitura natural disso é: **o Node é quem falha no Windows**, e corrigir o produto do Node fecha o
grupo.

## O que é

**Invertido.** Medido nos três arquivos de teste:

```
internal/validator/validator_thirdparty_provenance_test.go:62,355   filepath.Rel(root, destination)
npm/tests/validator.test.js:1026,1188                                path.relative(root, destination)
pypi/tests/test_validator_thirdparty_provenance.py:69,237            os.path.relpath(destination, root)
```

As **três** fixtures montam a chave com separador **nativo**. E o produto:

```
Go   validator_thirdparty_provenance.go:151,160   filepath.Rel(...)  -> normalizeRefSeparator   NORMALIZA
Py   validator.py:3530                            _normalize_ref_separator(os.path.relpath(...))  NORMALIZA
Node validator/index.js:3184                      path.relative(root, destination)                nao normalizava
```

Logo, em Windows:

| | chave da fixture | chave que o produto procura | resultado |
|---|---|---|---|
| Go | `skills\thirdparty\example.md` | `skills/thirdparty/example.md` | **FALHA** — a fixture reprova o produto certo |
| Python | `skills\thirdparty\example.md` | `skills/thirdparty/example.md` | **FALHA** — idem |
| Node | `skills\thirdparty\example.md` | `skills\thirdparty\example.md` | **passa** — os dois errados, casando |

Simulação (`ntpath`, sem precisar de Windows):

```
dest        = C:\proj\skills\thirdparty\example.md
fixture_key = 'skills\\thirdparty\\example.md'
product_key = 'skills/thirdparty/example.md'
casa?       = False
```

## Por que a produção é `/`, e a fixture não representava a produção

Quem monta o destino real é `ResolveThirdPartySkillDestination`
(`internal/integrations/render.go:821`) — e ele usa **concatenação explícita com `/`**, não
`filepath.Join`:

```go
return baseDir + "/thirdparty/" + slug + ".md", surface.ID, nil
```

O `install` grava a chave com esse valor (`UpsertProvenanceEntry(root, rt.destination, …)`). Ou
seja: **a chave gravada em disco é `/` em qualquer SO**. A fixture, ao usar `filepath.Rel`,
inventava uma forma que a produção nunca produz.

## A armadilha operacional

Corrigir **só** o produto do Node (que é o que o handoff pedia) teria virado o Node de **verde para
vermelho** no Windows — aumentando a contagem de falhas enquanto se "corrige" o defeito. O conserto
completo exige normalizar a chave **nas 3 fixtures** junto com o produto do Node.

Generalização: quando gerador e verificador de um par são **ambos** de teste (fixture escreve, regra
lê), "o teste passa" não discrimina "os dois certos" de "os dois errados". O discriminante é
comparar a fixture contra o **terceiro** elemento — a produção que realmente grava o artefato.

## O que fica protegido

`scripts/check-ref-separator-portability.sh` ganhou 3 checagens nomeadas para as fixtures (além das
19 dos sítios de emissão). Sem elas, uma reversão só apareceria no runner de Windows — que roda uma
vez por push, na melhor hipótese.

## Relacionado

- `docs/adr/ADR-2026-09-04-separador-posix-nos-artefatos-autorados-cujo-consumidor-nao-e-o-sistema-de-arquivos.md`
- `docs/seguranca/2026-09-01-modelo-de-ameaca-do-separador-em-artefato.md`
- `vault/notes/serve-validator-index-detectado-como-binario-grep-silencioso-2026-08-29.md`
- `vault/notes/ciclo-fechado-por-artefato-uma-sabotagem-nao-falsifica-tres-fronteiras-2026-09-03.md`
