# `roadmap new --from-req` sempre reprova `ref_targets_exist`, mesmo com REQ existente

> Data: 2026-08-01 | Autor: Ártemis | Domínio: generators / validator

## Sintoma

`trackfw roadmap new --from-req docs/req/REQ-flag-source.md`, seguido de
`roadmap move ... wip` e `trackfw validate`, reprova mesmo quando a REQ existe e o
gerador está correto (sem nenhuma corrupção):

```
✗ roadmap "ROADMAP-....md" links to REQ "REQ-flag-source.md" which does not exist
```

O caminho `roadmap new --title --req <path>` (sem `--from-req`) NÃO tem esse problema —
só o caminho `--from-req` está afetado. Reproduzido nos três CLIs (Go, Node, Python);
mesma causa raiz nos três, mesmo texto de mensagem.

## Causa raiz (duas partes)

**Parte 1 — `NewRoadmapFromREQ` grava só o basename no frontmatter.**
Em `internal/generators/roadmap.go:175` (`req: "%s"` com `filepath.Base(reqPath)`), o
frontmatter do roadmap recebe `req: "REQ-flag-source.md"` — sem o diretório — enquanto o
corpo (`## Context` → `REQ: docs/req/REQ-flag-source.md`) recebe o caminho completo,
correto. Espelhado em `npm/src/generators/roadmap.js:517` (`req: "${basename}"`) e
`pypi/trackfw/generators/roadmap.py` (equivalente). Já documentado como achado colateral
na auditoria da Wave 1 de
`ROADMAP-2026-07-31-alinhar-marcador-de-criterios-de-aceite-do-gerador-de-roadmap.md` —
mas sem a causa raiz abaixo, que explica por que o basename (e não o caminho do corpo)
é o valor que o validador efetivamente usa.

**Parte 2 — `extractRefPath` retorna no primeiro match, e o frontmatter vem antes do corpo.**
`internal/validator/validator.go` (`extractRefPath`, ~1425) varre o arquivo linha a linha
procurando um campo `REQ:` (case-insensitive) e retorna assim que encontra a primeira
linha cujo valor termine em `.md`. O frontmatter YAML tem `req: "REQ-flag-source.md"` —
que casa com o campo `REQ` por `EqualFold` — e aparece ANTES da linha `REQ:` do corpo
(`## Context`). Por isso `extractRefPath` sempre devolve o basename do frontmatter, nunca
o caminho completo do corpo, independentemente do que o corpo diga.

**Parte 3 — `referenceExists` ignora o parâmetro `roots`.**
`internal/validator/validator.go:1491-1497`:

```go
func referenceExists(ref string, roots []string) bool {
	expandedRef := config.ExpandPath(ref)
	if _, err := os.Stat(expandedRef); err == nil {
		return true
	}
	return false
}
```

`roots` (que seria `[]string{cfg.REQDir}`, ou seja `docs/req`) nunca é usado. A checagem é
sempre `os.Stat(ref)` relativo ao cwd. Como `ref` é o basename (Parte 1+2), `os.Stat`
sempre falha — o arquivo real está em `docs/req/REQ-flag-source.md`, não em
`./REQ-flag-source.md`.

O bug tem DUAS causas independentes que se mascaram uma à outra: corrigir só a Parte 1
(gravar o caminho completo no frontmatter) já resolveria o sintoma seguindo o caminho
atual de `extractRefPath` (que pegaria o frontmatter mesmo assim, mas agora com o valor
certo); corrigir só a Parte 3 (usar `roots` de fato) não resolveria nada sozinha, porque
`ref` já chega errado (basename) da Parte 1+2.

## Impacto

Todo agente que use `roadmap new --from-req` — o caminho recomendado quando já existe uma
REQ com critérios de aceite prontos — vai bater nessa violação na primeira transição para
`wip`, mesmo com REQ, ADR e Roadmap corretamente linkados. Reportado de forma independente
por dois agentes na Wave 1 do roadmap de 2026-07-31 sem diagnóstico — apenas o sintoma foi
registrado. Sem esta nota, a Parte 3 (o `roots` morto) é fácil de não notar porque o código
parece correto à primeira leitura — só falha ao rastrear o valor real de `ref`.

## Por que não foi corrigido aqui

Fora de escopo do ML-2A (ROADMAP-2026-07-31-...) — o ML cobre só o heading de critérios de
aceite, não o link de REQ. Candidato a REQ própria. **Atenção para quem for corrigir**: o
cenário de falsificação `roadmap-acceptance-heading/*/from-req` em
`scripts/check-gates-falsify.sh` (ML-2A) depende deste bug continuar produzindo a violação
`ref_targets_exist` como violação CO-OCORRENTE (não como a única) — o padrão buscado por
`assert_fails_with` é especificamente a substring `is in wip but has no acceptance
criteria block` (`wip_acceptance`), não a ausência de outras violações. Corrigir este bug
NÃO quebra aquele cenário: só reduz o cenário `from-req` corrompido de 2 violações para 1
(a que o cenário verifica). Confirmado empiricamente nesta sessão — ver o roadmap ML-2A
para os dois outputs (limpo vs. corrompido).

## ✅ CORRIGIDO em 2026-08-01

Entregue em `ROADMAP-2026-08-01-corrigir-falso-positivo-ref-targets-exist-em-roadmap-new-from-req`
(ADR `ADR-2026-08-01-caminho-completo-no-campo-req-...`). O contrato do campo `req:` é **caminho
relativo completo**; o parâmetro `roots` foi **removido** (não implementado) — a validação segue
**estrita**. `extractRefPath` não mudou.

Um **quarto** defeito foi descoberto durante o setup e corrigido junto: `roadmap new --req <path>`
gravava `req: ""` vazio no frontmatter, produzindo falso-**negativo** silencioso (o early-return
para valor vazio faz nenhuma violação disparar).

Proteção de CI: cenários `roadmap-req-frontmatter-path/*` em `scripts/check-gates-falsify.sh`,
cobrindo os 3 CLIs e os dois caminhos de geração, com braços de baseline e de detecção.
Contador 30 → 42.

A previsão registrada abaixo — de que a correção **não** quebraria os cenários
`roadmap-acceptance-heading/*/from-req` — **confirmou-se empiricamente**.

## Correção sugerida na época (histórico)

- Parte 1: gravar o caminho relativo completo (`reqPath`, não `filepath.Base(reqPath)`) no
  campo `req:` do frontmatter, nos três CLIs.
- Parte 3: usar `roots` em `referenceExists` (resolver `ref` relativo a cada root antes de
  desistir), ou remover o parâmetro morto se a Parte 1 já bastar.
