---
status: Open
date: 2026-09-12
author: ""
adr: ""
roadmap: ""
---

# REQ: v8 — um binário, muitos canais

> Date: 2026-09-12 | Status: Open

## Contexto

`ADR-2026-09-12-estrategia-de-distribuicao` — **Accepted, adotada para a v8**.

Uma implementação em **Go**, distribuída por npm e pip com o **binário nativo dentro do pacote**, no
lugar das reimplementações. Molde medido em produção: **esbuild** (Go, npm, `optionalDependencies`
por plataforma) e **gh-bin** (o `cli/cli`, PyPI, wheels `py3-none-*` com **zero arquivo Python**).

Validado empiricamente em `ROADMAP-2026-09-12-validar-um-binario-muitos-canais` — incluindo o AC6
(lockfile do macOS instalando em Windows real) e byte-identidade nos dois SOs testados.

## O que sai

```
npm/src/         26.272 linhas
pypi/trackfw/    27.472 linhas
                 ─────────────
                 53.744 linhas de reimplementação

31 dos 61 gates existem só para vigiar paridade
```

## Acceptance Criteria

### Distribuição

- [ ] **AC1** — pacote npm: casquinha (~80 linhas, medida no protótipo) + N pacotes de plataforma com
      `os`/`cpu`, resolvidos por `optionalDependencies`. 🔴 **Resolução dinâmica**
      (`@trackfw-bin/${platform}-${arch}`), nunca mapa hardcoded — foi o defeito que a VM pegou.
- [ ] **AC2** — pacote PyPI: wheels `py3-none-<plataforma>` no formato `gh-bin`, binário em
      `.data/scripts/`, **zero arquivo Python**. Tags: `manylinux_2_17_*`, `musllinux_1_2_*`,
      `macosx_*`, `win32`/`win_amd64`/`win_arm64`. 🔴 `linux_x86_64` puro é rejeitado pelo PyPI.
- [ ] **AC3** — 🔴 **os manifests de plataforma são GERADOS**, não escritos à mão. Isso mantém **1**
      sítio de versão em vez de 11+, e é o que faz o **#338 ser resolvido** por esta REQ em vez de
      agravado. Gate verifica que a geração aconteceu e bate com o `CHANGELOG`.
- [ ] **AC4** — workflow de release cross-compila e publica N+1 pacotes npm e N wheels, atomicamente
      o quanto o registry permitir. Falha parcial precisa ser **detectável** — a v7.6.0 publicou
      GitHub e PyPI e deixou o npm para trás por credencial.

### Remoção

- [ ] **AC5** — `npm/src/` e `pypi/trackfw/` removidos.
- [ ] **AC6** — suítes `npm/tests/` e `pypi/tests/` removidas ou convertidas: elas testam a
      reimplementação. O que resta é **smoke** — a casquinha lança o binário, repassa argv, devolve
      exit code.
- [ ] **AC7** — os gates de paridade que **perdem objeto** são removidos, **nomeados um a um**.
      🔴 Remover gate exige a mesma disciplina de criar: dizer o que ele media e por que não mede
      mais nada.
- [ ] **AC8** — `docs/cli-parity.md` vira documento de **canais**, não de comportamento.
      `CLAUDE.md` — a regra dura de paridade muda de natureza e precisa ser reescrita, não apagada.

### O que não pode se perder

- [ ] **AC9** — 🔴 **byte-identidade com o binário nativo**, nos três SOs do CI, incluindo **x64**
      (a validação rodou em ARM64; x64 ficou por extrapolação).
- [ ] **AC10** — instala sob restrição: registry alternativo · `--ignore-scripts` · **sem rota para
      github.com** · lockfile de um SO instalando em outro. São os ACs que representam o motivo dos
      canais existirem.
- [ ] **AC11** — **break declarado no CHANGELOG**: `require('trackfw')` deixa de resolver.
      `main: ./src/commands/index.js` é superfície acidental — sem `exports`, sem `types`, sem
      documentação —, mas é break e vai escrito.
      *(O PyPI não tem API: só console script e `__version__`. Medido.)*

### O que a mudança apaga do backlog

- [ ] **AC12** — 🔴 **a medição realocada.** Classificar as REQs e os issues abertos em
      **desaparece / barateia / indiferente**, pelo mesmo critério, e **fechar os que a causa
      removeu**. Sem isto, adotamos a v8 e o backlog fica em limbo.
      ⚠️ A estimativa de *"8 de 16 issues"* é **classificação preliminar por leitura**, não medição —
      **não usar como resultado**.

- [ ] **AC13** — 🔴 **os pacotes de plataforma NÃO declaram `bin`.** Medido no protótipo em
      2026-09-12: shim e pacote de plataforma declaravam ambos `bin: {trackfw: ...}` e **colidem em
      `node_modules/.bin/trackfw`** — quem instala por último vence, de forma não determinística, e
      a invocação por `.bin` ou `npx` pode **passar por cima da casquinha**, que é justamente quem
      resolve a plataforma e emite o erro nomeado do AC7.
      Confirmado contra o modelo de referência: `@esbuild/darwin-arm64` **não declara `bin`**.
      Falsificação: pacote de plataforma com `bin` ⇒ gate reprova.

## Escopo negativo

- **Não** mudar comportamento do CLI. Esta REQ move **distribuição**, não semântica. Qualquer
  divergência encontrada é achado, e o Go é a referência.
- **Não** publicar em registry real antes do AC9 e do AC10 verdes.
- **Não** remover capacidade do Go para simplificar a casquinha.

## Linked ADR
ADR: docs/adr/ADR-2026-09-12-estrategia-de-distribuicao-o-custo-da-paridade-tripla-e-quem-ela-atende.md
