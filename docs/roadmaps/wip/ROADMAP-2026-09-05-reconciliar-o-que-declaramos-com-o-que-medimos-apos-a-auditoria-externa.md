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
**Status:** 🔄 Em andamento · **Agente:** `apolo-tf`
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
**Status:** 🔄 Em andamento · **Agente:** `trackfw_architect`
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
