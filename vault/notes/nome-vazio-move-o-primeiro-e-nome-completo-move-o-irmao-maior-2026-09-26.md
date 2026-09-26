# `roadmap move`/`req move`: o nome vazio movia o primeiro — e o nome COMPLETO movia o irmão maior

> Criada em 2026-09-26 · ML-1C da REQ-2026-09-09 · `internal/generators/{roadmap.go,req.go}`

## O que se sabia

`containsIgnoreCase(basename, name)` sobre `strings.Contains`: `Contains(x, "")` é **sempre
verdadeiro**, logo `trackfw roadmap move "" wip` casava o **primeiro** arquivo do **primeiro**
diretório de estado e o movia. Aconteceu de verdade em 2026-09-12, por variável de shell vazia.

## 🔴 O que a medição acrescentou, e que ninguém previu

O nome **COMPLETO e exato** também movia o arquivo errado. Medido no corpus real (228 roadmaps,
231 REQs), com binário de `HEAD` contra binário corrigido, sobre cópia do corpus:

```
$ trackfw roadmap move ROADMAP-2026-07-19-global-adrs-governance analyzing     # ANTES
✓ moved ROADMAP-2026-07-19-global-adrs-governance-ML-1B.md → docs/roadmaps/analyzing
```

Causa: quando o stem do arquivo é **prefixo** de um irmão maior (`...-governance` contra
`...-governance-ML-1B.md`), o substring casa os dois e a ordem de varredura escolhe. São **3 pares
assim nos roadmaps e os mesmos 3 nos REQs** — e o nome completo é justamente a forma que o usuário
digita achando que é inequívoca. `req move` é pior que `roadmap move` no mesmo mecanismo: além de
mover, ele **reescreve o `status:` dentro do arquivo** errado.

**Consequência de projeto:** recusar ambiguidade **sem** precedência de casamento exato tornaria
esses 3 nomes **impossíveis de endereçar** — trocaria mover-o-errado por não-mover-nenhum. A ordem
correta é: exato (com ou sem `.md`) vence → depois parcial único → só então recusa.

## Como medir isto em outro corpus (a régua que separa "parcial" de "ambíguo")

Parcial **não** é o defeito; **ambíguo** é. No corpus deste projeto:

| query | únicos | ambíguos |
|---|---|---|
| nome completo sem `.md` | 225 / 228 | **3** (os prefixos acima) |
| 20 primeiros caracteres | 151 / 228 | **77** |
| vazio | — | **228** (casa tudo) |

Os 77 são casamentos que hoje **escolhem em silêncio**. Proibir "parcial" mataria os 151 — que é o
uso real de todo dia. A régua é a **cardinalidade dos candidatos**, nunca o comprimento da query.

## Duas armadilhas de implementação

1. 🔴 **`findRoadmap` tinha DUAS cópias do laço primeiro-vence** (layout `flat` e `by_agent`), e o
   corpus real deste projeto é `flat` — corrigir só o `flat` deixa o defeito vivo com `make quality`
   verde. Ponto único de enumeração (`roadmapCandidateFiles`) + teste `by_agent` explícito.
2. **Coleta-de-todos exige filtrar `.md`.** Sob primeiro-vence, um `.DS_Store` ou `ROADMAP-x.md~` ao
   lado era inofensivo; coletando todos, ele viraria um segundo candidato e transformaria um
   casamento **único** em **recusa**. Medido antes de filtrar: os únicos não-`.md` do corpus
   (`.trackfw-log`, `.DS_Store`) vivem na raiz de `docs/roadmaps`, zero dentro das pastas de estado.

## Sítio de mesma classe deixado de fora, e por quê

`validator.BranchSlugMatchesRoadmap` (`validator.go:3493`) é o mesmo `strings.Contains` e com
`branchSlug` vazio **casa qualquer roadmap** (validação passa vaziamente). Fica para o `ML-3A` por
decisão escrita da **D4** do `ADR-2026-09-26`: apertar esse matcher **não** é aditivo e pode
recusar a própria branch que entrega o fix (`trackfw commit` falha, `git commit` cru é bloqueado
pelo guard). Recusar o nome vazio no `move`, ao contrário, é aditivo em segurança — não existe
consumidor legítimo de `move ""`. 🔴 Quando o ML-3A chegar, o braço do **slug vazio** tem de estar
na lista dele.

> **Fechado em 2026-09-26 pelo `ML-3A`:** o braço do slug vazio de `BranchSlugMatchesRoadmap` foi
> recusado, e a relação ganhou o segundo braço (sobreposição de tokens). O que a medição daquele ML
> acrescentou está em
> [[o-prefixo-roadmap-e-token-de-todo-roadmap-e-o-contains-rejeita-29-das-205-2026-09-26]] — inclusive
> a refutação de que o `Contains` só errava em ~2 de 111 casos.

