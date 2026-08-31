---
status: wip
date: 2026-08-31
req: "docs/req/REQ-2026-08-31-guarda-de-folha-faz-lstat-so-no-ultimo-componente-e-nunca-inspeciona-ancestral-escrita-fora-do-projeto-em-todo-so-e-todo-runtime.md"
squad: "hades-tf, apolo-tf, artemis-tf"
---

# Roadmap: Guarda de folha resolve o caminho e afirma contenção antes de escrever

> Created: 2026-08-31 | Status: wip

## Context

REQ: `docs/req/REQ-2026-08-31-guarda-de-folha-faz-lstat-so-no-ultimo-componente-e-nunca-inspeciona-ancestral-escrita-fora-do-projeto-em-todo-so-e-todo-runtime.md`

`Lstat` só deixa de seguir o **último** componente do caminho — ancestrais são sempre seguidos. As
guardas de link do projeto checam só a folha, então um symlink num diretório ancestral redireciona a
escrita para fora do projeto sem que nada olhe. **Todo SO, todo runtime, sem mitigação acidental.**

Descoberto a reboque da investigação de junction, mas independente dela: não precisa de junction, de
`ModeIrregular` nem de privilégio — **um symlink de diretório comum basta**.

**Reproduzível localmente em macOS/Linux**, sem runner Windows e sem a sonda. É o que separa esta REQ
da irmã.

## Acceptance Criteria

- [ ] Enumeração real, pelos primitivos de **escrita** (228 candidatos brutos), não por `ModeSymlink`
- [ ] Escrita através de ancestral symlink recusada nos 3 CLIs, forma resolver-e-afirmar-contenção
- [ ] Falsificação nas duas direções, **incluindo o controle** de que operação legítima segue funcionando
- [ ] Recusa audível em stderr
- [ ] Paridade exata nos 3 CLIs — aqui a paridade vale normalmente
- [ ] `make quality` e **CI** verdes

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat Model
> Dependências: nenhuma. Bloqueia toda a implementação.

### ML-0A — Enumeração real e modelo de ameaça
**Status:** ⬜ Pendente
**Agente:** `hades-tf`
**Files affected:** nenhum (documento em `docs/seguranca/`)
**Actions:**
1. **Completude da enumeração — o entregável principal deste ML.** A lista de 3 guardas
   (`update.go:1869`, `:1894`, `discover.go:268`) veio de `grep` por `ModeSymlink` e é
   **conhecidamente incompleta**: esse grep é cego para todo ponto que escreve **sem checar link
   nenhum**, que é a população em risco. Varra os **primitivos de escrita** (`os.WriteFile`,
   `os.Create`, `writeFileSync`, `write_text`, `open(...,'w')`) e determine quais escrevem em
   caminho derivado de `root`. Mostre a busca. Na Wave 0 anterior você achou `copyPath`/`_copy_path`
   e o código morto `writeCIWorkflowForce` fora da minha lista — assuma que esta também está.
2. **Modelo de ameaça** — quem esvazia esta Wave 0 sem quebrar regra escrita? Quem planta o
   ancestral, com que capacidade, e o que ganha?
3. **Falsificação nas duas direções** — para cada superfície: o que quebra quando regride, e o que
   quebra quando regride ao contrário (a guarda super-disparar e recusar operação legítima é o
   risco simétrico, e é o que transforma correção em quebra).
4. **Residual declarado.**
**Critérios de aceite:**
- [ ] As quatro seções com evidência, não asserção de uma linha
- [ ] A enumeração distingue "escreve sob `root` controlável" de "escreve em caminho fixo"
- [ ] Nenhuma linha de implementação escrita
- [ ] Parecer em `docs/seguranca/2026-08-31-modelo-de-ameaca-da-guarda-de-ancestral.md`

**Gates da wave:**
```bash
test -f docs/seguranca/2026-08-31-modelo-de-ameaca-da-guarda-de-ancestral.md
! grep -qi "placeholder" docs/seguranca/2026-08-31-modelo-de-ameaca-da-guarda-de-ancestral.md
grep -q "Residual" docs/seguranca/2026-08-31-modelo-de-ameaca-da-guarda-de-ancestral.md
```

## Wave 1 — A correção
> Dependências: Wave 0 completa. **O particionamento em MLs só é decidível depois da enumeração** —
> escrever os MLs agora seria escopo inventado. Preenchido ao fechar o ML-0A.

## Wave 2 — Gate falsificável
> Dependências: Wave 1 completa. `artemis-tf`. Detalhado após a Wave 1.

## Barreira final

Revisão `hefesto-tf` e `hades-tf`, auditoria do arquiteto, `barrier`. **CI verde**, não só verde
local — `vault/notes/ambiente-do-dev-e-mais-rico-que-o-do-ci-2026-08-29.md`.
