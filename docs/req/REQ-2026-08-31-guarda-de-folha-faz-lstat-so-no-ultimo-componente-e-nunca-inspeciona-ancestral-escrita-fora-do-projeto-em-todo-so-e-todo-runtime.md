---
status: Open
date: 2026-08-31
author: "zeus-tf"
adr: ""
roadmap: "docs/roadmaps/wip/ROADMAP-2026-08-31-guarda-de-folha-resolve-o-caminho-e-afirma-contencao-antes-de-escrever.md"
---

# REQ: Guarda de folha faz `Lstat` só no último componente e nunca inspeciona ancestral — escrita fora do projeto em todo SO e todo runtime

> Date: 2026-08-31 | Status: Open

## Motivation

As guardas de link do projeto verificam **apenas a folha** do caminho:

```go
info, err := os.Lstat(filepath.Join(root, DiscoverGitHubActionsWorkflowPath))
if info.Mode()&os.ModeSymlink != 0 { /* recusa */ }
os.WriteFile(path, ...)
```

**`Lstat` só deixa de seguir o *último* componente do caminho. Ancestrais são sempre seguidos.** Logo
um symlink num diretório ancestral (`.github/`, `.github/workflows/`) redireciona a escrita para fora
do projeto **sem que a guarda olhe**. A folha não é symlink; a checagem passa; a escrita sai da
árvore.

### Por que isto é a descoberta que importa

Este defeito veio a reboque da investigação de junction no Windows
(`REQ-2026-08-30-sonda-nao-responde-a-pergunta-7-...`), mas **não tem nada de Windows**:

- Vale em **Linux, macOS e Windows**.
- Vale nos **três runtimes** — aqui a paridade se aplica normalmente, ao contrário da detecção de
  junction, onde os três divergem.
- Não depende de junction, de `ModeIrregular` nem de privilégio: **um symlink de diretório comum
  basta**, e criar symlink de diretório não exige privilégio em Linux/macOS.

A medição de junction (run `33447191373`) atenuou as outras duas classes — o `rmdir` remove a
junction e não o alvo, sem destruição de dados, e o Node já enxerga junction. **Esta classe não foi
atenuada por nada.**

### Precondição, declarada honestamente

Exige quem consiga plantar um symlink na árvore do projeto — mesma precondição das outras classes.
Não é escalonamento remoto. O que a distingue não é ser *mais grave*, é ser **universal**: mesma
forma, três runtimes, três sistemas operacionais, sem mitigação acidental.

### A enumeração conhecida está errada, e sei disso

As três guardas que eu havia nomeado (`internal/generators/update.go:1869`, `:1894`,
`internal/discover/discover.go:268`) saíram de um `grep` por `ModeSymlink`/`Lstat`. **Esse grep é
cego para todo ponto que escreve sem checar link nenhum** — que é exatamente a população em risco.
Contagem bruta dos primitivos de escrita:

| runtime | ocorrências |
|---|---|
| Go (`os.WriteFile`/`os.Create`) | 85 |
| Node (`writeFileSync`) | 78 |
| Python (`write_text` / `open(...,'w')`) | 65 |

**228 no total** — limite superior, não a lista. A maioria não escreve em caminho derivado de `root`
controlável por terceiro. **Descobrir quais escrevem é a Wave 0, não premissa desta REQ.** A Wave 0
anterior já provou o ponto: `hades-tf` encontrou duas superfícies fora da minha lista
(`copyPath`/`_copy_path` e o código morto `writeCIWorkflowForce`).

### Forma do remédio — nomeada aqui de propósito

*"Inspecionar ancestrais"* admite pelo menos três formas incompatíveis: `Lstat` de cada componente até
o `root`; resolver com `filepath.EvalSymlinks`/`fs.realpathSync`/`Path.resolve()` e **afirmar que o
resultado continua sob o `root`**; ou descida incremental estilo `openat`. Sem escolher uma, três
especialistas inventam três.

**A forma escolhida é resolver-e-afirmar-contenção**, porque **compõe com o que já existe**: o
`cleanEmpty` (`manager.js:420`) e o `_remove_empty` (`manager.py:589`) já fazem contenção geográfica
(`path.isAbsolute(rel)`, `root in directory.parents`). Adotar a mesma forma evita uma segunda
checagem conflitante com a primeira.

## Acceptance Criteria

- [ ] **AC1** — 🔴 **Wave 0 enumera de verdade.** A lista de pontos que escrevem em caminho derivado
      de `root` **sem** inspeção de ancestral, nos 3 runtimes, obtida varrendo os **primitivos de
      escrita** — não `ModeSymlink`. A lista de 3 guardas é ponto de partida conhecido-incompleto.
- [ ] **AC2** — Escrita através de **ancestral** symlink é recusada nos 3 CLIs, com a forma
      resolver-e-afirmar-contenção.
- [ ] **AC3** — 🔴 **Falsificação nas duas direções.** (a) com ancestral symlink apontando para fora,
      a escrita é recusada e **nada** é criado fora da árvore; (b) **controle**: operação legítima,
      sem link algum, **continua funcionando** — a guarda não pode super-disparar. Sem (b), trocamos
      um buraco por uma quebra.
- [ ] **AC4** — Recusa **audível**: mensagem em stderr nomeando o caminho e o motivo. Silêncio vira
      *"o update não atualizou meu arquivo e não disse nada"*.
- [ ] **AC5** — Paridade exata nos 3 CLIs: mesma recusa, mesma mensagem. **Aqui a paridade vale
      normalmente** — ao contrário da detecção de junction, onde os três divergem legitimamente.
- [ ] **AC6** — Gate falsificável cobrindo AC2 e AC3 nos 3 runtimes, com guarda de vacuidade.
- [ ] **AC7** — Reproduzível **localmente** em macOS/Linux, sem depender de runner Windows nem da
      sonda. É o que torna esta REQ mais rápida que a de junction.
- [ ] **AC8** — `make quality` verde e **CI verde**. Verde local não é conclusão —
      ver `vault/notes/ambiente-do-dev-e-mais-rico-que-o-do-ci-2026-08-29.md`.

## Negative Scope — o que esta REQ NÃO faz

- **Não trata detecção de junction.** Classes 1 e 2, `ModeIrregular`, troca de primitiva no Python:
  tudo isso é a REQ irmã, que **precisa de ADR** (divergência deliberada da paridade de 3 CLIs) e só
  se verifica em runner Windows. Misturar faria esta REQ herdar a verificação pós-merge de lá.
- **Não altera o Node na detecção de link.** Medido: o Node já enxerga junction.
- **Não adota `ModeSymlink|ModeIrregular`** em lugar nenhum.
- **Não mexe** em `cleanEmpty`/`_remove_empty`/`removeEmptyAncestors` — são Classe 2.

## Linked ADR

ADR: <!-- nenhum. Correção de bug com tratamento idêntico nos 3 runtimes; não há decisão
arquitetural a registrar. O ADR pertence à REQ irmã, de junction, onde a divergência entre runtimes
é deliberada e precisa ser documentada em docs/cli-parity.md para que um gate futuro não "conserte"
o Node de volta à simetria. -->

## Linked Roadmap

Roadmap: `docs/roadmaps/wip/ROADMAP-2026-08-31-guarda-de-folha-resolve-o-caminho-e-afirma-contencao-antes-de-escrever.md`
