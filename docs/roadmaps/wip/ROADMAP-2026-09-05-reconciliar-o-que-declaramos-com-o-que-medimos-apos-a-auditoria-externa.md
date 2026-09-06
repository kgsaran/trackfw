---
status: wip
date: 2026-09-05
squad: apolo-tf
req: "docs/req/REQ-2026-09-05-auditoria-externa-aponta-que-declaramos-correcao-onde-a-nossa-propria-medicao-dizia-o-contrario.md"
---

# Roadmap: Reconciliar o que declaramos com o que medimos, após a auditoria externa

> Criado em: 2026-09-05 | Status: wip

## Context

REQ: `docs/req/REQ-2026-09-05-auditoria-externa-aponta-que-declaramos-correcao-onde-a-nossa-propria-medicao-dizia-o-contrario.md`
Registro da auditoria: `docs/portabilidade/2026-09-05-auditoria-externa-astra-achados-e-verificacao.md`
ADR irmã (previne a recorrência): `docs/adr/ADR-2026-09-05-o-ci-de-windows-bloqueia-por-conjunto-de-nomes-e-por-tipo-de-evento-nunca-por-contagem.md`

## Diagnóstico

Três achados verificados por mim, todos procedentes. O padrão que os une: **produzimos a evidência e
não a cruzamos com a nossa própria declaração.**

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 1 — Desfazer o dano que introduzimos (primeiro, e sozinho)
> Dependências: nenhuma. Vem antes de tudo porque é vermelho **na `main`**.

### ML-1A — O teste do `ENOTDIR` afirma o que foi medido, ou sai
**Status:** ✅ Concluído · **Agente:** `apolo-tf`
**Arquivos:** `internal/integrations/manager_collision_enotdir_test.go` (+ `manager.go` se a
reconciliação exigir).

O ML-1C mediu que `ENOTDIR = ERROR_PATH_NOT_FOUND` no Windows e escreveu teste afirmando o
contrário. Reprova no CI.

🔴 **Reconciliar, não apagar.** Ou o teste afirma o comportamento **medido** (no Windows, `ENOTDIR`
**é** ausência, por decisão do SO; em POSIX, não), ou é removido com a razão escrita no lugar.
🔴 **`skip` não é saída** — esconderia a contradição. E foi o `#279` que nos ensinou isso.
🔴 **A prova é no CI de Windows**, não em macOS: confiar em suíte verde local foi o que produziu o
defeito.

### ML-1B — Corrigir publicamente o comentário do `#274`
**Status:** ✅ Concluído · **Agente:** `trackfw_architect`
**Novo** comentário na issue, com a medição que derruba o anterior. Não editar o original em
silêncio: quem leu a afirmação errada precisa encontrar a correção.

## Wave 2 — Fechar o resíduo do `#278`
> Dependências: Wave 1.

### ML-2A — `req_has_adr` deixa de aceitar placeholder e prosa
**Status:** ✅ Concluído · **Agente:** `apolo-tf` (3 CLIs)
`ADR: <!-- preencher depois -->` e `veja a secao ADR: mais abaixo` contam como vínculo hoje.
Falsificação nas duas direções, com esses dois casos nomeados.
🔴 **A contagem do acervo vai subir acima de 67 — e continua sendo acerto.** Declarar antes e depois.
🔴 **Não** corrigir o acervo. Baseline é trabalho próprio.

## Wave 3 — A guarda contra a repetição
> Dependências: Waves 1 e 2. **É o entregável que importa** — as outras corrigem instâncias.

### ML-3A — Todo ML com teste novo declara qual conclusão aquele teste afirma
**Status:** ✅ Concluído · **Agente:** `trackfw_architect`
Instrumentar o contrato de handoff e o de auditoria: quem entrega teste novo declara **qual conclusão
do próprio ML aquele teste afirma**, e o arquiteto verifica **esse cruzamento** na auditoria.

Foi o passo que faltou nos três achados. Sem ele, as Waves 1 e 2 corrigem sintomas.

### ML-3B — O vínculo guarda o caminho com a pasta de estado, e `roadmap move` quebra o vínculo
**Status:** ⬜ Pendente · **Agente:** `apolo-tf`

**Medição que originou o ML** (arquiteto, 2026-09-06, `./bin/trackfw validate`):

```
req X links to Roadmap "docs/roadmaps/wip/<nome>.md" which does not exist   (2 ocorrências)
    → os dois roadmaps existem, em docs/roadmaps/done/
```

**Causa**, lida no código e idêntica nos 3 CLIs: `referenceExists(ref)` faz `os.Stat` **literal** no
caminho armazenado, e o caminho armazenado **inclui a pasta de estado**. Mas, pelo `CLAUDE.md`, a
pasta *é* o estado. Logo **todo `trackfw roadmap move` quebra, por construção, todo vínculo que
guarda caminho completo**. Não são 2 vínculos quebrados: são 2 que já dispararam.

🔴 Segundo efeito, pior porque é silencioso: `validateREQRoadmapLifecycle` (`req_roadmap_lifecycle`)
faz `os.Stat` no mesmo caminho e, **em erro, faz `continue`** — a regra que existe para achar REQ
aberta com roadmap em `done/` **desliga exatamente no caso em que o roadmap foi para `done/`**. É
fail-open na regra, não só ruído no relatório.

**Arquivos afetados (os 3 CLIs — regra dura de paridade):**
- `internal/validator/validator.go` — `referenceExists`, `validateRefTargetsExist`, `validateREQRoadmapLifecycle`
- `npm/src/validator/index.js` — `referenceExists`, `extractRefPath` e chamadores
- `pypi/trackfw/validator.py` — `_reference_exists`, `validate_ref_targets_exist`, ciclo de vida

**Ações:**
1. Resolver o vínculo **pelo basename**, procurando o arquivo nos diretórios de estado configurados
   (`backlog/analyzing/wip/blocked/done/abandoned`, honrando `roadmap_namespacing`), com o caminho
   literal ainda aceito quando existir. Existe em qualquer estado ⇒ o vínculo **não** está quebrado.
2. `req_roadmap_lifecycle` passa a derivar o estado **de onde o arquivo foi encontrado**, não do
   caminho gravado — fim do `continue` que desliga a regra.
3. Ambiguidade (mesmo basename em dois estados) **não** é resolvida em silêncio: emitir aviso próprio.

**Falsificação nas duas direções** (`scripts/check-gates-falsify.sh`), por CLI:
- roadmap movido de `wip/` para `done/` com a REQ intacta ⇒ **sem** aviso de vínculo quebrado (hoje avisa);
- REQ apontando para basename que não existe em estado algum ⇒ **avisa** (guarda de vacuidade);
- REQ `Open` com roadmap em `done/` ⇒ **avisa**, mesmo com o caminho gravado desatualizado.

**Reconciliação obrigatória:** para **cada** teste novo, uma frase dizendo qual conclusão deste ML
aquele teste afirma (regra dura do `CLAUDE.md`).

**Critérios de aceite:**
- [ ] `go build ./...` e `go vet ./...` limpos
- [ ] `make quality QUALITY_EXIT=0` sem `FAIL`
- [ ] `./bin/trackfw validate` deixa de emitir os 2 avisos `links to Roadmap ... does not exist`
- [ ] comportamento idêntico nos 3 CLIs (`scripts/check-cli-parity.sh`)
- [ ] uma frase de reconciliação por teste novo, no relatório

### ML-3C — O acervo: medido, declarado e sob ratchet, não corrigido à mão
**Status:** ⬜ Pendente · **Agente:** `trackfw_architect`

O ML-3B original dizia "REQs com `adr:` vazio apontando para ADRs aceitos", o que soa delimitado. A
medição diz outra coisa:

| medido em 2026-09-06 | valor |
|---|---|
| REQs com `adr: ""` | **128** de 201 |
| avisos `req_has_adr` | **103** |
| avisos `req_has_roadmap` | **54** |
| roadmaps de `done/` que falhariam se voltassem a `wip/` | **~43** (o ML-2A relatou 13 — **sub-medido**) |

Decidir caso a caso qual das 103 tem ADR real é arqueologia por arquivo, sem critério de parada, e
produz um diff que **não** é auditável contra critério de aceite. O acervo entra no **ratchet**
(teto declinante, 103 grandfathered), que é a ADR já escrita — não em correção manual nesta sessão.

**Critérios de aceite:**
- [ ] os números acima registrados no roadmap e no `docs/agents-working-context.md`
- [ ] o bloco `## Critérios de Aceite` presente neste roadmap (hoje `validate` acusa a ausência)
- [ ] a sub-medição do ML-2A (13 vs ~43) corrigida por escrito, na auditoria

## Critérios de Aceite

- [ ] ML-3B: os 2 avisos de vínculo quebrado somem **sem** editar as REQs, nos 3 CLIs
- [ ] ML-3B: `req_roadmap_lifecycle` deixa de ser fail-open quando o caminho gravado está velho
- [ ] ML-3C: acervo medido e declarado; correção manual explicitamente fora de escopo, com motivo
- [ ] `make quality QUALITY_EXIT=0` sem `FAIL` e `./bin/trackfw validate` sem aviso novo

## Fora deste roadmap
- **Ratchet de CI** → ADR própria, já escrita.
- **CRLF em renderizadores** (não só no parser) e **jornada de instalação em Windows** → achados
  legítimos da mesma auditoria, com superfície própria. Misturar aqui repetiria o erro que esta REQ
  existe para corrigir.
- **Release**: a publicada é a v7.3.0, de 28/08 — anterior a tudo. Decisão do usuário, e é a que
  determina se qualquer uma dessas correções chega a quem instala.


## Auditoria da Wave 1 — arquiteto, 2026-09-05

```
make quality QUALITY_EXIT=0, zero FAIL · trackfw validate exit 0
go build/vet limpos · go test ./internal/integrations/... ok
```

**ML-1A — reconciliado, não apagado.** `TestDetectNameCollision_ENOTDIRIsPlatformDependent` ramifica
por `GOOS` **sem `skip`**, e **cada ramo afirma algo**: em POSIX, `err != nil`; no Windows,
`err == nil` — mas **só depois de confirmar, via `os.ReadDir` bruto, que
`errors.Is(rawErr, fs.ErrNotExist)` é verdadeiro no runner real**.

🔴 Esse último passo veio de uma correção do advisor do próprio agente: a primeira versão do ramo
Windows era **vacuamente satisfazível** — um `ReadDir` bem-sucedido por acidente também daria
`err == nil`. O teste passaria sem medir nada. **Guarda de vacuidade dentro de um teste que existe
para corrigir uma contradição** — exatamente o rigor que faltou na entrega original.

**A frase de reconciliação teste↔conclusão** (requisito novo desta wave) foi escrita para **os dois**
testes do arquivo, incluindo o que eu não pedi. **A guarda funcionou na primeira aplicação.**

**ML-1B** — correção publicada no `#274` como comentário novo, não edição do original.

## 🔴 Erro de processo do arquiteto nesta wave

Rodei `git add -A` **com o subagente editando a árvore**: o commit `228fea5`, rotulado
`chore(governance)`, carregou o teste do ML-1A. Regra existia em prosa e não impediu nada; **quem
detectou foi o agente**, por `git hash-object`.

Não reescrevi o histórico. Virou `ADR-2026-09-05-staging-com-escopo-implicito-...` + REQ: o controle
passa para o **harness e para o produto**, em vez de depender de memória.


## Auditoria dos ML-2A e ML-3A — arquiteto, 2026-09-06

```
make quality QUALITY_EXIT=0, zero FAIL · trackfw validate exit 0
acervo: req_has_adr 70 -> 103 (+33)  ·  req_has_roadmap 36 -> 54 (+18)
```

**ML-3A** — a `Regra Dura de Reconciliação` está no `CLAUDE.md`, com o **limite declarado**: ela pega
contradição interna (artefato contra conclusão do mesmo relatório), e **não pega premissa errada
compartilhada** pelos dois. Para isso serve a barreira independente.

**ML-2A** — o agente **corrigiu a mensagem no meio do trabalho, por medição**: a primeira versão dizia
"o marcador tem de iniciar a linha", mas **38 das 51 novas acusações são de placeholder**, não de
ancoragem — a mensagem mandaria a maioria a investigar a coisa errada.

🔴 **E ele esclareceu o caso que eu tinha entendido errado.** O meu `REQ (**reaberta**):` tem
decoração **entre o nome do campo e os dois-pontos** — **nem a implementação antiga nem a nova** casam
com isso. Não era o mesmo defeito que a prosa. Forma que funciona: `REQ: docs/req/x.md (reaberta)`.

**`none`/`TBD`/`N/A`/`-` seguem sendo valores reais, de propósito** — protegem as fixtures dos gates, e
nenhum dos 4 casos exigidos precisava bloqueá-los.

### Para o ML-3B, achado do ML-2A

**13 linhas `> REQ:` decoradas em `docs/roadmaps/done/`.** Hoje fora de alcance (as regras só varrem
`wip/` e `blocked/`) — 🔴 **mas falhariam se algum desses roadmaps voltasse para `wip/`**, que é
exatamente o que fizemos **duas vezes hoje** (CRLF e fail-open). Bomba-relógio de formato no acervo.
