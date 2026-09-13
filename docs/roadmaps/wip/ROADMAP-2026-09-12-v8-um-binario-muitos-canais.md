---
status: wip
date: 2026-09-12
req: "docs/req/REQ-2026-09-12-v8-um-binario-muitos-canais-casquinha-npm-e-wheels-de-plataforma-substituem-as-reimplementacoes-node-e-python.md"
squad: ""
---

# Roadmap: v8 — um binário, muitos canais

> Created: 2026-09-12 | Reescrito: 2026-09-12 | Status: wip

## Context
REQ: docs/req/REQ-2026-09-12-v8-um-binario-muitos-canais-casquinha-npm-e-wheels-de-plataforma-substituem-as-reimplementacoes-node-e-python.md
ADR: docs/adr/ADR-2026-09-12-estrategia-de-distribuicao-o-custo-da-paridade-tripla-e-quem-ela-atende.md (**Accepted**)

Substituir **53.744 linhas** de reimplementação (Node 26.272 + Python 27.472) por uma casquinha de
~80 linhas e wheels sem código Python, mantendo os três canais. **31 dos 61 gates** deixam de ter
objeto.

Validado empiricamente em `ROADMAP-2026-09-12-validar-um-binario-muitos-canais`: AC1–AC9 verdes,
byte-identidade em darwin/arm64, win32/arm64 e win32/x64.

> 🔴 Este bloco de ACs foi preenchido **à mão**. O `roadmap new --from-req` gerou `## Wave 1 — <name>`
> e **um** ML genérico para 13 critérios, com o bloco de ACs vazio. **Terceira ocorrência hoje** — é o
> AC7 da REQ de REQ órfã, ainda aberto.

## 🔴 O princípio que ordena as waves: construir antes de remover

Nada é deletado enquanto o canal novo não estiver **provado publicando**. As Waves 0 e 1 são
inteiramente reversíveis; a Wave 2 é a primeira coisa irreversível; a Wave 3 só começa depois dela.

**Motivo:** se removermos `npm/src/` e o canal novo falhar em produção, não há para onde voltar sem
um revert grande sob pressão. A ordem inversa custa um ciclo a mais e elimina esse cenário.

## Acceptance Criteria
- [ ] AC1 — casquinha npm + N pacotes de plataforma, **resolução dinâmica**
- [ ] AC2 — wheels `py3-none-<plataforma>` no formato `gh-bin`, zero Python
- [ ] AC3 — manifests **gerados**, não escritos à mão (resolve o #338)
- [ ] AC4 — release publica N+1 pacotes npm e N wheels, com falha parcial **detectável**
- [ ] AC5 — `npm/src/` e `pypi/trackfw/` removidos
- [ ] AC6 — suítes dos dois removidas ou convertidas em smoke
- [ ] AC7 — gates de paridade que perdem objeto removidos, **nomeados um a um**
- [ ] AC8 — `docs/cli-parity.md` vira documento de canais; `CLAUDE.md` reescrito
- [ ] AC9 — byte-identidade nos 3 SOs, **incluindo x64**
- [ ] AC10 — instala sob restrição: registry alternativo · `--ignore-scripts` · sem github · lockfile cruzado
- [ ] AC11 — break do `require('trackfw')` declarado no CHANGELOG
- [ ] AC12 — medição: quais REQs e issues fecham por **causa removida**
- [ ] AC13 — pacotes de plataforma **não declaram `bin`**

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

---

## Wave 0 — Threat model
> Dependências: nenhuma. **Bloqueia tudo.**

### ML-0A — Threat model desta migração
**Status:** ⬜ Pendente
**Arquivos afetados:** somente este roadmap.

🔴 **Não pule.** O threat model da REQ de REQ órfã achou a família `findREQ` — 3 sítios que o
arquiteto não tinha enumerado. Aqui o custo de enumerar errado é maior: o que não for encontrado
antes da Wave 3 vira código deletado sem substituto.

**Ações:**
1. **Enumeração:** todo sítio que **assume três implementações**. Não só `npm/src/` e
   `pypi/trackfw/` — também CI, Makefile, gates, docs, templates de artefato, `.github/`, scripts de
   release. Declare a lista fechada com a busca que a fecha.
   🔴 Use `/usr/bin/grep` ou `git grep`: o `grep` do shell é `ugrep -I` e omite arquivos com NUL byte.
2. **Threat model:** quem esvazia esta migração sem quebrar regra escrita?
3. **Falsificação nas duas direções:** o que quebra se a migração regredir, e o que quebra se ela
   **for longe demais** — removendo algo que ainda tem consumidor.
4. **Residual declarado.** Em especial: a audiência que a ADR admite estar sendo trocada
   (política corporativa que proíbe executáveis, não só rede).

**Critérios de aceite:**
- [ ] As quatro seções com evidência, não asserção de uma linha
- [ ] 🔴 A lista de sítios inclui pelo menos uma categoria **fora** de `npm/` e `pypi/`
- [ ] Nenhuma linha de implementação neste ML

---

## Wave 1 — Construir o canal novo, sem remover nada
> Dependências: Wave 0. **Tudo reversível.** Os MLs 1A–1C são paralelos entre si; 1D e 1E dependem deles.

### ML-1A — **AC3 + AC4** — geração dos manifests e do release
**Status:** ⬜ Pendente
**Arquivos afetados:** `.github/workflows/release.yml`, novo script de geração, `Makefile`.
**Contexto:** hoje há **5 sítios de versão** e o cruzamento com o `CHANGELOG` só roda no
`release tag` — é o **issue #338**. Com N pacotes de plataforma seriam **5+N**.
🔴 **Gerar em vez de vigiar:** os `package.json` de plataforma **não existem no repositório**; são
artefato de build, como no esbuild. Sítios caem para **1**, e o #338 é **resolvido por esta REQ**.
**Critérios de aceite:**
- [ ] Um único sítio de versão; os manifests são gerados a partir dele
- [ ] Gate verifica que a geração aconteceu **e** bate com o `CHANGELOG`
- [ ] Falsificação: divergir o sítio único do CHANGELOG ⇒ reprova
- [ ] 🔴 Publicação parcial é **detectável** — a v7.6.0 publicou GitHub e PyPI e deixou o npm para
      trás por credencial, e o job saiu verde para os dois primeiros

### ML-1B — **AC1 + AC13** — casquinha npm de produção
**Status:** ⬜ Pendente
**Arquivos afetados:** `npm/bin/`, `npm/package.json`. 🔴 **Não remover `npm/src/` ainda** — é Wave 3.
**Molde medido:** `prototype/packages/trackfw-shim/bin/trackfw.js`, **80 linhas**, já auditado.
**Critérios de aceite:**
- [ ] Resolução **dinâmica** (`@trackfw-bin/${platform}-${arch}`), nunca mapa hardcoded — foi o
      defeito que a VM pegou
- [ ] Pacotes de plataforma **sem `bin`** (AC13) **e** o shim resolvendo mesmo assim — 🔴 **as duas
      propriedades provadas juntas**; foi trocar uma pela outra que quebrou o CI no protótipo
- [ ] Sem pacote de plataforma ⇒ aborta **nomeando a plataforma**, nunca `MODULE_NOT_FOUND`
- [ ] Porte de `npm/tests/shim_packaging.test.js`, que hoje testa o protótipo

### ML-1C — **AC2** — wheels PyPI
**Status:** ⬜ Pendente
**Arquivos afetados:** `pypi/pyproject.toml`, build de wheels. 🔴 **Não remover `pypi/trackfw/`** — Wave 3.
**Molde medido:** wheel do `gh-bin` — `dist-info` + `.data/scripts/<bin>`, **zero arquivo Python**.
🔴 Tags: `manylinux_2_17_*` · `musllinux_1_2_*` · `macosx_*` · `win_amd64`. **`linux_x86_64` puro é
rejeitado pelo PyPI.**
**Critérios de aceite:**
- [ ] Wheel sem nenhum `.py`; binário em `.data/scripts/`
- [ ] `pip install` local sem rede e sem toolchain Go
- [ ] Avaliar `go-to-wheel` antes de escrever do zero — 🔴 ele **não** suporta layout `cmd/<nome>/`, medido no protótipo

### ML-1D — **AC9** — byte-identidade vira gate permanente
**Status:** ⬜ Pendente
**Arquivos afetados:** novo `scripts/check-shim-byte-identity.sh`, `.github/workflows/quality.yml`.
**Contexto:** hoje isso é a **Pergunta 15 de uma sonda sob demanda**. Vira **gate**, nos 3 SOs.
**Critérios de aceite:**
- [ ] Roda em Linux, macOS e Windows **x64** do CI
- [ ] Compara saída **e** exit code, incluindo o de violação
- [ ] 🔴 Guarda de vacuidade: cenário que não monta ⇒ **reprova**, com causa nomeada
- [ ] 🔴 Distingue **ambiente incompleto** de defeito — o teste do protótipo derrubou 3 checks
      obrigatórios por não fazer isso

### ML-1E — **AC10** — instalação sob restrição vira gate
**Status:** ⬜ Pendente
**Arquivos afetados:** novo gate, `Makefile`.
**Cenários, todos já provados no protótipo:** registry alternativo · `--ignore-scripts` · **sem rota
para github** · lockfile de um SO instalando em outro.
🔴 Use `npm install --offline`, **não** registry morto: a porta 1 não recusa, fica em `SYN_SENT` até
o timeout — custou 11 min por execução no probe.
**Critérios de aceite:**
- [ ] Os 4 cenários, cada um com prova de que a restrição estava **ativa**
- [ ] O `--offline` tem braço próprio provando que **falha quando precisa de rede**

### ML-1F — gate de byte NUL literal em fonte
**Status:** ⬜ Pendente
**Origem:** `REQ-2026-09-12-byte-nul-literal-no-fonte-...` — 🔴 **só o gate entra aqui.** Os dois
arquivos com NUL são fontes do Node que a Wave 3 deleta; o defeito some sozinho. O **gate**
sobrevive e vale para qualquer fonte, inclusive Go.
**Critérios de aceite:**
- [ ] Reprova se qualquer fonte rastreado contiver NUL literal, nomeando arquivo e offset
- [ ] 🔴 **Não usa `grep`** para procurar o NUL — seria a ferramenta derrotada pelo objeto medido
- [ ] Guarda de vacuidade: varredura que não examina arquivo nenhum ⇒ reprova

---

## Wave 2 — Publicar e provar em registry real
> Dependências: **Wave 1 inteira verde.** 🔴 Primeira etapa irreversível.

### ML-2A — publicação sob `v8.0.0-rc`
**Status:** ⬜ Pendente
🔴 **Nome e versão no npm são permanentes após 72 h; o PyPI não reusa nome de arquivo.** Por isso
`-rc`, e por isso esta wave não começa antes da Wave 1 fechar.
**Critérios de aceite:**
- [ ] `npm install trackfw@rc` e `pip install trackfw==8.0.0rc1` funcionam em máquina limpa, nos 3 SOs
- [ ] Instalação de versão **anterior** continua funcionando
- [ ] 🔴 Falha parcial entre canais é detectada e reportada

---

## Wave 3 — Remover
> Dependências: **Wave 2 verde.** Nada aqui começa antes de o canal novo estar publicando.

### ML-3A — **AC5** — remover as reimplementações
**Status:** ⬜ Pendente
`npm/src/` (26.272 linhas) e `pypi/trackfw/` (27.472).

### ML-3B — **AC6** — suítes
**Status:** ⬜ Pendente
`npm/tests/` e `pypi/tests/` testam a reimplementação. O que resta é **smoke**: a casquinha lança o
binário, repassa argv, devolve exit code.
🔴 `npm/tests/shim_packaging.test.js` **não é da reimplementação** — é do empacotamento, e sobrevive.
**Mover, não apagar** (está escrito no topo do arquivo).

### ML-3C — **AC7** — gates que perdem objeto
**Status:** ⬜ Pendente
**31 dos 61** são de paridade. 🔴 **Nomear um a um**, dizendo o que cada um media e por que não mede
mais nada. Remover em bloco é como se perde cobertura sem perceber.
**Critérios de aceite:**
- [ ] Lista nomeada, com justificativa por gate
- [ ] Gate que ainda mede algo **fica** — paridade não é o único motivo de um gate existir

### ML-3D — **AC8 + AC11** — documentação e o break
**Status:** ⬜ Pendente
`docs/cli-parity.md` vira documento de **canais**. O `CLAUDE.md` tem a regra dura de paridade
**reescrita, não apagada** — o Go continua sendo a expressão da verdade, agora por construção.
🔴 CHANGELOG declara o break: `require('trackfw')` deixa de resolver.

---

## Wave 4 — Fechar o backlog que a mudança apagou
> Dependências: Wave 3.

### ML-4A — **AC12** — a medição realocada
**Status:** ⬜ Pendente
Classificar REQs e issues em **desaparece / barateia / indiferente**, pelo mesmo critério, e
**fechar os que a causa removeu**. Sem isto, a v8 entra e o backlog fica em limbo.
⚠️ A estimativa de *"8 de 16 issues"* é **classificação preliminar por leitura** — não serve como
resultado.

---

## Escopo negativo

- **Não** mudar comportamento do CLI. Esta REQ move **distribuição**, não semântica. Divergência
  encontrada é achado, e o Go é a referência.
- 🔴 **Não** remover nada antes da Wave 2 verde.
- **Não** remover capacidade do Go para simplificar a casquinha.
- **Não** publicar em registry real fora da Wave 2.
