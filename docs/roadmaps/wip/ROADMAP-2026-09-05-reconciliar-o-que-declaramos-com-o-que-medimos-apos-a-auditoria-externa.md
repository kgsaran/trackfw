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
**Status:** ⬜ Pendente · **Agente:** `apolo-tf` (3 CLIs)
`ADR: <!-- preencher depois -->` e `veja a secao ADR: mais abaixo` contam como vínculo hoje.
Falsificação nas duas direções, com esses dois casos nomeados.
🔴 **A contagem do acervo vai subir acima de 67 — e continua sendo acerto.** Declarar antes e depois.
🔴 **Não** corrigir o acervo. Baseline é trabalho próprio.

## Wave 3 — A guarda contra a repetição
> Dependências: Waves 1 e 2. **É o entregável que importa** — as outras corrigem instâncias.

### ML-3A — Todo ML com teste novo declara qual conclusão aquele teste afirma
**Status:** ⬜ Pendente · **Agente:** `trackfw_architect`
Instrumentar o contrato de handoff e o de auditoria: quem entrega teste novo declara **qual conclusão
do próprio ML aquele teste afirma**, e o arquiteto verifica **esse cruzamento** na auditoria.

Foi o passo que faltou nos três achados. Sem ele, as Waves 1 e 2 corrigem sintomas.

### ML-3B — REQs com `adr:` vazio apontando para ADRs aceitos
**Status:** ⬜ Pendente · **Agente:** `apolo-tf`
Achado de governança da mesma auditoria. Depende do ML-2A para não misturar com a mudança de gate.

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
