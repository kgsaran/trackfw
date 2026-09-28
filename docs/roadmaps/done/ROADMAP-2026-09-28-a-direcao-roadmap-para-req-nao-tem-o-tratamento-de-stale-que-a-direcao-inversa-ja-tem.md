---
status: done
date: 2026-09-28
req: "docs/req/REQ-2026-09-28-a-direcao-roadmap-para-req-nao-tem-o-tratamento-de-stale-que-a-direcao-inversa-ja-tem.md"
squad: ""
---

# Roadmap: a direcao roadmap para REQ nao tem o tratamento de stale que a direcao inversa ja tem

> Created: 2026-09-28 | Status: done

## Context
<!-- Derived from REQ: REQ-2026-09-28-a-direcao-roadmap-para-req-nao-tem-o-tratamento-de-stale-que-a-direcao-inversa-ja-tem.md -->
REQ: docs/req/REQ-2026-09-28-a-direcao-roadmap-para-req-nao-tem-o-tratamento-de-stale-que-a-direcao-inversa-ja-tem.md

## Acceptance Criteria
<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->
- [x] A direção Roadmap → REQ **classifica** referência stale, com o predicado certo (ML-1A)
- [x] `ref_targets_exist` varre `backlog` e `analyzing`; `done`/`abandoned` **declarados fora**, com
      a razão medida em `docs/cli-parity.md` (ML-1B)
- [x] `FindRoadmapLinkingREQ` **não cruza namespace** de agente em `by_agent`, e em `flat` o vínculo
      por basename **continua funcionando** (ML-1C)
- [x] `roadmapCandidateFiles` devolve separador **POSIX por contrato**, fixado por gate falsificado
      nas duas direções (ML-1D)
- [x] Nenhum teste afirma o contrato de separador **antigo** (ML-1E)
- [x] `make quality` local e **CI verdes**: 20/20 checks SUCCESS no PR #464

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat model: medir o que NÃO precisa ser construído
> Dependências: nenhuma. **Bloqueia a implementação.**

**Gates da wave:**

```bash
n=$(git ls-files 'docs/req/*.md' | xargs -n1 basename | sort -u | wc -l | tr -d ' '); t=$(git ls-files 'docs/req/*.md' | wc -l | tr -d ' '); test "$n" = "$t" && echo "Gate W0: $t REQs, $n basenames unicos — zero colisao" || { echo "GATE FALHOU: $t REQs mas $n basenames unicos — ha colisao, o ramo de ambiguidade e alcancavel" >&2; exit 1; }
```

⚠️ Este gate **passa hoje** e é de vigilância: se algum dia houver colisão de basename, o ramo de
ambiguidade deixa de ser inalcançável e a decisão do `ML-0A` precisa ser revista.

### ML-0A — o ramo de ambiguidade é alcançável?
**Owner:** `hades-tf`
**Status:** ✅ Concluído — auditado em 2026-09-28 · 🔴 **refutou duas premissas minhas, e uma é bloqueante**
**Parecer:** `docs/seguranca/2026-09-28-wave0-req-stale-direction.md`

**Veredito resumido:**

1. **Ramo `default:` é alcançável** — `req new --agent hades "x"` + `req new --agent apolo "x"` no mesmo dia produz dois arquivos de basename idêntico sem obstáculo. Medido em projeto temporário com o binário desta branch. O ramo entra na Wave 1, condicionado ao predicado correto (ver ponto 3 abaixo).

2. **AC4 — ampliar para `backlog`**, declarar `done`/`abandoned` fora. Custo medido: 14 violações de artefatos pré-padronização em done/abandoned que ninguém vai corrigir; 0 custo adicional em backlog que não seja acionável. Opção B (backlog+analyzing, sem done/abandoned) fecha o #452 sem ruído histórico.

3. **Bloqueio crítico para Wave 1:** o predicado de entrada do mecanismo existente (`isStaleRoadmapStateRef`) usa `agentNamespaceStateNames` — e para a ref do #452 (`docs/req/hefesto/REQ-X.md`), `base_parent = "hefesto"` não está no mapa. Um mirror literal NÃO corrige o #452. O predicado correto para REQs é `filepath.Dir(ref) != "."`. O Scenario 25 é preservado com este predicado (`REQ-flag-source.md` tem dir=`.`).

4. **Wave 0 gate é cego em `by_agent`**: o glob `docs/req/*.md` não alcança `docs/req/<agente>/*.md`. Passes vacuamente em projeto by_agent puro. Corrigi-lo está fora do escopo desta REQ.

**Critérios de aceite:**
- [x] Veredito escrito sobre alcançabilidade do ramo de ambiguidade, com a medição
- [x] Decisão do AC4 (ampliar × declarar), com o custo de cada lado
- [x] Nenhuma linha de implementação neste ML

## Auditoria da Wave 0 — 2026-09-28

### 🔴 Refutação 1, e é BLOQUEANTE: "espelhar" não corrigiria o #452

O AC1 diz *"espelhar `resolveRoadmapRefStatus`"*. **Verdade para a estrutura, falso para o
predicado.** O mecanismo existente entra no fallback por:

```go
func isStaleRoadmapStateRef(ref string) bool {
	dir := filepath.Base(filepath.Dir(filepath.ToSlash(ref)))
	return agentNamespaceStateNames[dir]      // wip, backlog, done, …
}
```

Para o ref do #452 — `docs/requisicoes/hefesto/REQ-X.md` — o pai é **`hefesto`**, que **não é
estado**. Verifiquei: `filepath.Base(filepath.Dir(...))` → `hefesto` → `false` → **o fallback nunca
roda**.

🔴 **Um implementador que copiar literalmente entrega código que compila, passa todos os testes
atuais, passa o Cenário 25 da falsificação — e não corrige nada.** É a pior forma de entrega: verde
por toda parte e inerte no sítio que motivou o trabalho.

**O predicado para REQ é outro:** `filepath.Dir(ref) != "."` — deixa `docs/req/hefesto/REQ-X.md`
entrar no fallback e bloqueia `REQ-flag-source.md` (sem diretório), preservando o Cenário 25.

### 🔴 Refutação 2: o ramo de ambiguidade É alcançável

Eu escrevi no AC2 que ele provavelmente seria inalcançável, com base em **0 colisões em 233 REQs**.
A medição derruba: o executor produziu a colisão com **dois comandos normais**.

```
$ trackfw req new "same title test" --agent hades   → docs/req/hades/REQ-2026-09-28-same-title-test.md
$ trackfw req new "same title test" --agent apolo   → docs/req/apolo/REQ-2026-09-28-same-title-test.md
```

`req new` **não verifica colisão entre namespaces de agente**. Não é estado fabricado — são dois
comandos que qualquer consumidor `by_agent` roda.

⚠️ **O erro de método meu:** o corpus que medi é deste repositório, que é **`flat`**. Confundi
*ausência no meu corpus* com *impossibilidade no produto*. O ramo entra.

### ✅ E uma refutação DELE que a medição derruba

Ele afirmou que o gate desta Wave 0 é *"cego em `by_agent`"*, porque `docs/req/*.md` não alcançaria
subpastas. **Medi o contrário:**

```
cenário by_agent com colisão real (hades/REQ-X.md + apolo/REQ-X.md)
  git ls-files 'docs/req/*.md'    → 2 arquivos
  o gate                          → GATE FALHOU: 2 REQs mas 1 basenames unicos
```

🔴 **`git ls-files` trata o padrão como *pathspec*, que casa recursivamente** — diferente do glob de
shell, onde `*` para no separador. A intuição dele vem do shell e é razoável; o instrumento é que se
comporta diferente. **O gate pega a colisão.**

### Decisões aceitas, com o custo medido

**AC4 — ampliar para `backlog` + `analyzing`, declarar `done`/`abandoned` fora.** Medido nos 206
roadmaps em `done`/`abandoned`:

| categoria | n |
|---|---|
| literal resolve | 149 |
| resolveria por basename (viraria *stale*) | 20 |
| não existe em lugar nenhum | **14** |
| sem campo `req:` | 23 |

As **14** são refs legados, de antes da padronização de caminho. **Ninguém vai corrigir um roadmap
`done` para atualizar um campo legado** — emiti-los seria ruído permanente no CI. `backlog` é o
oposto: trabalho futuro, acionável, e é **onde o #452 se manifesta**.

**Achado de mesma causa — `FindRoadmapLinkingREQ` cruza namespaces.** Ao criar a REQ do `apolo`, o
`chainRoadmapForREQ` encontrou o roadmap do **`hades`** por basename e vinculou os dois. Mesma causa,
mesmo mecanismo → **ML desta REQ**, não REQ nova.

---

## Wave 1 — o predicado correto, não o espelho literal

⚠️ **Terceira vez nesta campanha que o rótulo de wave duplica.** O `roadmap new` gera
`## Wave 1 — Implementation`; ao acrescentar a wave real, ficam **duas seções com o mesmo rótulo**, e
o `barrier` lê a **primeira** — que está vazia (`wave 1: no ML found`). Aconteceu no roadmap do #444
(Wave 0 e Wave 1) e aqui. 🔴 **Candidato a issue**: o gerador deveria recusar rótulo duplicado, ou o
`barrier` deveria acusar a duplicata em vez de ler a primeira em silêncio.
> Dependências: **Wave 0 auditada** ✅

**Gates da wave:**

```bash
grep -qE 'filepath\.Dir\(ref\) != "\."|dirOfRefIsNotDot' internal/validator/validator.go && echo "Gate W1: o predicado de REQ nao e o de roadmap" || { echo "GATE FALHOU: o predicado de REQ ainda nao existe — copiar isStaleRoadmapStateRef NAO corrige o #452" >&2; exit 1; }
```

⚠️ Reprova hoje, de propósito.

### ML-1A — a direção Roadmap → REQ classifica, com o predicado CERTO
**Owner:** `apolo-tf`
**Status:** ✅ Concluído — 2026-09-28
**Arquivos:** `internal/validator/validator.go`

🔴 **NÃO copie `isStaleRoadmapStateRef`.** Ela testa se o pai é um **estado**; o caso do #452 tem o
pai igual ao **agente**. O predicado é `filepath.Dir(ref) != "."`.

**Critérios de aceite:**
- [x] `docs/req/<agente>/REQ-X.md` movido → **stale** message (violação-class — ver refutação abaixo)
- [x] REQ **apagada** → **violação**, como hoje
- [x] 🔴 **O teste exercita `docs/req/hefesto/REQ-X.md`** (`TestRefTargetsExist_ReqStale_HefestoDir`)
- [x] Cenário 25 do `check-gates-falsify.sh` **continua passando** (ref sem diretório)
- [x] Ramo de ambiguidade implementado — **é alcançável**, medido na Wave 0
- [x] A frase da Regra Dura de Reconciliação, por teste novo

**Refutação do AC "stale, não violação":** A regra `ref_targets_exist` tem severidade "error"
(default, ausente de `ruleDefaults`). Todos os mensagens da função passam por `applyRule`, que não
distingue mensagem individual — classifica todas pelo severity da regra. O espelho (REQ→Roadmap)
também emite stale como violação. O AC como escrito é inalcançável sem mecanismo novo. Mantido
como está: stale emite violação-class com texto "stale path" (distinguível de "does not exist").

### ML-1B — **AC4** — `ref_targets_exist` varre `backlog` e `analyzing`
**Owner:** `apolo-tf`
**Status:** ✅ Concluído — 2026-09-28
**Critérios de aceite:**
- [x] `backlog` e `analyzing` entram; `done`/`abandoned` **declarados fora** em `docs/cli-parity.md`
      seção "ref_targets_exist — escopo de estados (ML-1B)", com as 14 entradas legadas como razão medida
- [x] Contagem de warnings antes/depois: **0 → 0** (delta zero; todos os backlog refs deste repo
      resolvem pelo caminho literal — confirmado com binário before/after em 2026-09-28)

### ML-1C — `FindRoadmapLinkingREQ` não cruza namespace de agente
**Owner:** `apolo-tf`
**Status:** ✅ Concluído — 2026-09-28
**Arquivos:** `internal/generators/req_chain_ml4a.go`, `internal/generators/req_chain_ml1c_test.go`
**Critérios de aceite:**
- [x] REQ de `apolo` **não** vincula ao roadmap de `hades` de mesmo basename
- [x] 🔴 Contra-braço: em `flat`, o vínculo por basename **continua funcionando**

### ML-1D — `roadmapCandidateFiles` devolve separador POSIX por contrato
**Owner:** `apolo-tf`
**Status:** ✅ Concluído — 2026-09-28
**Arquivos:** `internal/generators/roadmap.go`, `scripts/check-ref-separator-portability.sh`,
`internal/generators/req_chain_ml1c_test.go`

**Origem:** o `windows-full-suites` do PR #464 reprovou em
`TestFindRoadmapLinkingREQ_ByAgent_DoesNotCrossNamespace` — na **guarda de anti-vacuidade** do
ML-1C. 🔴 **A guarda fez o trabalho dela:** sem ela, o teste passaria **vacuamente** no Windows,
porque *"não retorna o roadmap errado"* é trivialmente verdadeiro quando a busca não acha nada.

**Medição (Windows real, `go1.27 windows/arm64`, VM `Lab@192.168.64.6`):**
```
Join        = "docs\\roadmaps\\hades\\backlog\\R.md"   ← todo backslash
ToSlash(J)  = "docs/roadmaps/hades/backlog/R.md"
Base(J)     = "R.md"   Base(slash) = "R.md"
```
Fonte primária concordante: `$GOROOT/src/path/filepath/path.go:43` — *"any occurrences of slash are
replaced by Separator"* — e `IsPathSeparator` do Windows aceita `/`. 🔴 **Retratação do arquiteto:**
afirmei antes que o separador era *"misto"* (barra no prefixo, backslash no fim). É **falso** — o
`Clean` dentro do `Join` achata a barra literal da montagem de `dirs`.

**Decisão — (a) defeito do PRODUTO, não do teste.** `roadmapCandidateFiles` devolvia caminho em
separador nativo **sem contrato declarado de forma**. Dos 4 consumidores, 3 são imunes **por
acidente** (`filepath.Base` em `FindRoadmapLinkingREQ`/`selectArtifactByName`/`syncRoadmapREQReference`;
`Abs`/`Rel`/`ToSlash` em `agentFromPath`) e o 4º **imprime o caminho cru ao usuário** na mensagem
`multiple roadmaps match`. Corrigir o teste preservaria a armadilha para o próximo consumidor — foi
recusado explicitamente como a saída confortável.

**Reconciliação (Regra Dura):** o `assert_has "Go: roadmapCandidateFiles devolve separador POSIX"`
afirma a conclusão de que o contrato de forma passou a ser **estrutural e não acidental** — reversão
do `ToSlash` é detectada.

**Critérios de aceite:**
- [x] `roadmapCandidateFiles` devolve `/` em toda plataforma, com o contrato no doc comment
- [x] Gate fixa o contrato; contagem de anti-vacuidade `13 → 14`, **medida** (`grep -c` real = 14)
- [x] 🔴 **Falsificação nas duas direções**, feita pelo arquiteto sobre cópia mutada: fiel → `RC=0`;
      `ToSlash` revertido → `RC=1` nomeando o assert ausente
- [x] A guarda de anti-vacuidade do ML-1C **não foi relaxada** — ganhou comentário proibindo o relaxe
- [x] `go build ./...` e `go test ./internal/generators/...` verdes; `trackfw.yaml` intacto
- [x] `windows-full-suites` verde no PR #464 — **20/20 checks SUCCESS** (mede o CI, não a VM)

### ML-1E — o teste que afirmava o contrato ANTIGO
**Owner:** `apolo-tf`
**Status:** ✅ Concluído — 2026-09-28
**Arquivos:** `internal/generators/artifact_select_ml1c_test.go`

**Origem:** o ML-1D fechou a falha que o motivou, mas **destapou outra**:
`TestMoveRoadmap_AmbiguousPartial_RefusesNamingCandidates` fixava a mensagem de ambiguidade com
`filepath.Join` — forma **nativa**, o contrato antigo.

🔴 **Falha de auditoria do arquiteto, registrada:** eu identifiquei, no ML-1D, que
`selectArtifactByName` *"imprime o caminho cru ao usuário"* — e **não procurei testes que afirmassem
a forma antiga**. Havia um. A régua da auditoria enxergou o consumidor e não os seus testes.

**Medição (Windows real):**
```
erro deveria conter "docs\roadmaps\backlog\...um.md"
got:                 docs/roadmaps/backlog/...um.md
```

**Sobre as 93 falhas da suíte na VM — o instrumento mentiu.** `go test ./internal/generators/...` na
VM acusou 87 falhas únicas, 86 delas **não** na `windows-known-failures.json`. Não são regressões:
falham por `exec: "bash": executable file not found in %PATH%` — o `cmd.exe` da VM não tem bash, o
runner do CI tem. **Unanimidade num corpus inteiro é sinal de instrumento quebrado.** A interseção
VM ∩ CI era **uma** falha, e é esta.

**Reconciliação (Regra Dura):** o teste corrigido afirma que `roadmapCandidateFiles` devolve
separador POSIX em toda plataforma — e que a mensagem de ambiguidade **exibida ao usuário** usa `/`,
não `\`, mesmo no Windows.

**Critérios de aceite:**
- [x] Literal POSIX no lugar de `filepath.Join`, com o **porquê** no comentário
- [x] Import de `filepath` **medido** como ainda necessário (8 outros usos) antes de tocar
- [x] 🔴 Verde no **Windows real** (VM, `go1.27 windows/arm64`): `TestMoveRoadmap*`,
      `TestFindRoadmapLinkingREQ*`, `TestShowRoadmap*`, `TestRoadmapCandidateFiles*` — `ok`, 0 falhas
- [x] Gate de separador segue `RC=0` com 14 assinaturas; `roadmap.go` e `trackfw.yaml` intactos
- [x] `windows-full-suites` verde no PR #464 — **20/20 checks SUCCESS**
