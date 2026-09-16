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
**Status:** ✅ Concluído
**Arquivos afetados:** somente este roadmap.

**Critérios de aceite:**
- [x] As quatro seções com evidência, não asserção de uma linha
- [x] 🔴 A lista de sítios inclui pelo menos uma categoria **fora** de `npm/` e `pypi/`
- [x] Nenhuma linha de implementação neste ML

---

#### Seção 1 — Completude da enumeração

**Busca usada:** `/usr/bin/grep` e `git grep` em todo o worktree (o `grep` do ambiente é `ugrep -I`
e omite em silêncio arquivos com NUL byte — `npm/src/validator/index.js` e
`npm/src/integrations/doctor.js` foram confirmados com NUL byte; toda busca abaixo usou o binário
cru do sistema). Lista fechada:

**Categoria A — Go produto (`internal/`):**
A1. `internal/commands/release.go` linhas 95-108: array `releaseVersionFiles` hardcoda cinco caminhos
lidos por `trackfw release tag`, incluindo `pypi/trackfw/__init__.py` (duas vezes) e
`npm/package.json`. Após a Wave 3 (AC5 apaga `pypi/trackfw/`), `trackfw release tag` recusa com
erro de leitura — o CLI Go próprio do projeto perde a capacidade de publicar releases.
A2. `internal/commands/commit.go` linhas 188-191: `commitCommandDirs` lista `npm/src/commands/` e
`pypi/trackfw/commands/` na heurística de tipo de commit. Após Wave 3, a heurística deixa de
disparar para esses caminhos silenciosamente (mudança comportamental, não crash).

**Categoria B — CI workflows (`.github/workflows/`):**
B1. `quality.yml` job `node` (linha 62): `npm ci`, `npm test`, `npm run smoke`, `npm pack --dry-run`
contra `npm/`.
B2. `quality.yml` job `python` (linha 90): matrix [3.10, 3.12], `pytest pypi/tests -q`.
B3. `quality.yml` job `windows-full-suites` (linha 150): executa npm test e pytest.
B4. `quality.yml` job `windows-integrations-resolve` (linha 117):
`pytest pypi/tests/test_integrations_resolve.py`.
B5. `quality.yml` job `consumer-smoke-by-agent` (linha ~1596): instala os 3 CLIs e invoca
`check-consumer-smoke-by-agent.sh`.
B6. `release.yml` job `quality` (linhas 21-41): `npm test` e `pytest pypi/tests -q` como pré-check
de release.
B7. `release.yml` job `publish-npm` (linhas 83-104): faz bump em `npm/package.json` e publica a
estrutura atual de `npm/` — após v8 a estrutura muda completamente (casquinha + optionalDeps).
B8. `release.yml` job `publish-pypi` (linhas 107-135): `python -m build pypi` e publica
`pypi/dist/` — toda a estrutura muda para wheels `py3-none-<platform>`.

**Categoria C — Required status checks (`.github/required-status-checks.txt`):**
C1. Declara como checks obrigatórios (bloqueiam merge no GitHub): `node`, `python (3.10)`,
`python (3.12)`, `package-smoke`, `windows-integrations-resolve`, `windows-full-suites`. Esses
nomes vivem **fora do repositório** em branch protection (conjunto R). CI não tem acesso a R —
`check-required-status-checks.py` roda só com `--scope dw`. Se Wave 3 remove os jobs `node` e
`python` sem atualizar R, o check-name não é mais emitido por nenhum workflow e **todo PR seguinte
fica pendente para sempre** (nota do vault: `matriz-em-job-required-por-nome-fica-pendente-para-sempre-2026-09-08.md`).

**Categoria D — Makefile:**
D1. Targets `.PHONY`: `test-node` (`cd npm && npm test`) e `test-python`
(`python3 -m pytest pypi/tests -q`). Linha 9 e 17-21.
D2. `quality: test test-node test-python lint parity` (linha 174). Após Wave 3,
`make quality` invoca targets cujos objetos não existem.

**Categoria E — Scripts de gate não-paridade que inspecionam source files:**
E1. `scripts/check-raw-read-ban.sh` (linhas 100-141): lê `npm/src/validator/index.js` e
`pypi/trackfw/validator.py`.
E2. `scripts/check-output-encoding-declared.sh` (linhas 145-146): lê
`npm/src/generators/hooks.js` e `pypi/trackfw/generators/init_gen.py`.
E3. `scripts/check-atomic-write-anti-divergence.sh` (linhas 95-97): lê três arquivos em
`pypi/trackfw/`.
E4. `scripts/check-ref-separator-portability.sh` (linhas 91-206): lista específica de arquivos
em `npm/src/` e `pypi/trackfw/`.
E5. `scripts/check-static-assets.sh` (linhas 60-61): `npm/src/serve/static` e
`pypi/trackfw/serve/static`. **Nota:** Go embeds `internal/serve/static` via `go:embed`
(`internal/serve/serve.go:17`); as cópias npm/pypi foram mantidas pela paridade tripla, não
como fonte. Após Wave 3 o gate perde seu objeto.
E6. `scripts/check-integration-assets.sh` (linhas 58-59): `npm/src/integrations/assets` e
`pypi/trackfw/integrations/assets`. Mesma situação — Go embeds via `go:embed assets`
(`internal/integrations/catalog.go:15`).
E7. `scripts/sync-integration-assets.sh` (linhas 26-27): sincroniza PARA esses diretórios. Após
Wave 3 o destino some.
E8. `scripts/check-python-writes-lf.sh`: percorre `pypi/trackfw/` — tem vacuity guard que faz
exit 1 em zero arquivos, então falha ruidosamente após Wave 3.
E9. `scripts/check-tty-detection.sh` (linha 66): percorre `pypi/trackfw/` — mesma vacuity, mesma
falha ruidosa.
E10. `scripts/check-serve-api-file-security.sh` (linhas 38-40): lê quatro arquivos em npm/pypi.
**Silente:** linha 122 usa `grep -q ... 2>/dev/null` — se o arquivo some, o check passa sem
coverage (falso positivo de gate).
E11. `scripts/check-serve-browser-security.sh` (linhas 62-66): lê `npm/src/commands/serve.js`,
`pypi/trackfw/commands/serve.py`, `pypi/trackfw/__main__.py`.

**Categoria F — Falsify sharding (`scripts/check-gates-falsify.sh`):**
F1. Linhas 392, 1177-1194, 2373-3030: copia `npm/bin/trackfw`, `npm/src/`, `pypi/` como fixtures
de teste e invoca diretamente os CLIs Node e Python em 144 referências. Após Wave 3 os fixtures
não existem, a contagem de cenários muda e o shard coverage (`check-falsify-shard-coverage.sh`)
pode passar sobre um corpus reduzido.

**Categoria G — `docs/cli-parity.md` anotações com gate= apontando para test files:**
G1. 192 anotações `gate=` em 270 seções. Exemplos: `gate=npm/tests/credential_guard_integrity.test.js,pypi/tests/test_credential_guard_integrity.py`. `check-parity-contract-coverage.sh` critério 2 exige que o caminho em `gate=` exista no disco — após ML-3B remover `npm/tests/` e `pypi/tests/`, essas anotações fazem o gate falhar.

**Fechamento da lista:** busca com `/usr/bin/grep -rn "npm/src\|pypi/trackfw\|npm/bin\|npm/tests\|pypi/tests" .github/ Makefile scripts/ internal/ docs/cli-parity.md` enumera os sítios acima. Não foram encontradas categorias adicionais em templates de artefato (`docs/roadmaps/`, `docs/req/`) nem em scripts de instalação (`scripts/install.sh`). A lista inclui categorias A (Go produto) e C (required-status-checks) que estão **fora de `npm/` e `pypi/`**, satisfazendo o critério de aceitação.

---

#### Seção 2 — Threat model

**Adversário:** o implementador que executa a Wave 3 sem atualizar as dependências fora de `npm/` e `pypi/`. Não é um atacante externo — é o desenvolvedor apressado ou o agente que recebe um handoff incompleto.

**Como esvaziar a migração sem quebrar regra escrita:**

**Vetor 1 — Release quebrado (A1, severidade: crítica).**
O agente executa Wave 3 (apaga `pypi/trackfw/`) antes que ML-1A atualize `internal/commands/release.go`. A regra escrita exige ML-1A antes de ML-3A (dependência implícita no roadmap), mas não há gate que impeça Wave 3 de começar com `release.go` intacto. Resultado: `trackfw release tag` lança erro de leitura a cada invocação. O projeto perde a capacidade de publicar releases até que alguém edite product code — e isso não é reversível pela branch protection.

**Vetor 2 — Branch protection trava o repositório (C1, severidade: crítica).**
Wave 3 remove os jobs `node` e `python` dos workflows. `make quality` passa, `trackfw validate` passa, o PR mergeia. O conjunto R (GitHub branch protection) ainda exige check-names que nenhum workflow emite. Todo PR subsequente fica bloqueado indefinidamente — inclusive o PR que consertaria o problema. O repositório entra em deadlock. Esta é a mesma falha documentada na nota de vault `matriz-em-job-required-por-nome-fica-pendente-para-sempre-2026-09-08.md`.

**Vetor 3 — Gate silente passa sem coverage (E10, severidade: alta).**
`check-serve-api-file-security.sh` usa `grep -q ... 2>/dev/null` ao verificar `pypi/trackfw/commands/serve.py`. Após Wave 3, o arquivo some e o grep retorna não-zero em silêncio — o check continua passando (exit 0) sem ter verificado nada. A auditoria de segurança de serve desaparece sem deixar rastro.

**Vetor 4 — Falsify corpus reduzido, shard coverage verde (F1, severidade: média).**
Wave 3 remove `npm/src/` e `pypi/`. `check-gates-falsify.sh` copia esses diretórios como fixtures. Com os fixtures ausentes, os cenários que os invocam são ignorados ou falham silenciosamente. `check-falsify-shard-coverage.sh` verifica cobertura relativa ao corpus presente — um corpus reduzido produz cobertura de 100% sobre menos cenários. Gate verde, coverage real reduzida.

**Vetor 5 — Commit heuristic muda silenciosamente (A2, severidade: baixa).**
`trackfw commit` para de classificar corretamente commits em caminhos `npm/src/commands/` e `pypi/trackfw/commands/`. Não gera erro, não quebra gate — apenas classifica commits de forma diferente. Comportamento observável só por quem conhece a heurística anterior.

---

#### Seção 3 — Falsificação nas duas direções

Para cada superfície: onde a sabotagem entra, qual gate deveria capturar, em qual direção.

**Direção regressão (a migração não se completa — Wave 3 foi revertida ou não executada):**

| Superfície | Onde entra a sabotagem | Gate que deveria capturar | Resultado esperado |
|---|---|---|---|
| npm/src/ permanece | Wave 3 não apaga `npm/src/` | `check-gates-falsify.sh` invoca Node — mas se Node passa, gate verde; nenhum gate detecta "npm/src/ ainda existe" | Falso negativo: gate verde, fonte mantida |
| pypi/trackfw/ permanece | idem | idem para Python | Falso negativo |
| `release.go` não atualizado | ML-1A não entregue antes de Wave 3 | Nenhum gate — só falha em runtime ao rodar `trackfw release tag` | Não capturado pelo CI |
| required-status-checks não atualizado | Wave 3 remove jobs sem atualizar R | `make check-required-full` detecta D\\R — mas só é obrigatório na release, não na PR | Falso negativo: PR mergeia, repositório trava depois |

**Direção longe demais (removeu algo que ainda tem consumidor):**

| Superfície | Onde entra a sabotagem | Gate que deveria capturar | Resultado esperado |
|---|---|---|---|
| `cli-parity.md` anotações gate= | ML-3B remove `npm/tests/` e `pypi/tests/`; anotações ficam apontando para paths inexistentes | `check-parity-contract-coverage.sh` critério 2 — path em `gate=` deve existir no disco | Capturado — gate falha ruidosamente |
| `check-python-writes-lf.sh` | pypi/trackfw/ removida antes do gate ser desativado | Vacuity guard do próprio script: exit 1 em zero arquivos | Capturado — gate falha ruidosamente |
| `check-tty-detection.sh` | idem | idem | Capturado — gate falha ruidosamente |
| `check-serve-api-file-security.sh` | pypi/trackfw/commands/serve.py removida | `2>/dev/null` suprime o erro — gate passa sem coverage | **Não capturado** — falso negativo silente |
| falsify corpus | npm/src/ e pypi/ removidas antes dos cenários serem convertidos | `check-falsify-shard-coverage.sh` — mas mede cobertura relativa ao corpus presente | **Não capturado** — cobertura aparece 100% sobre corpus menor |
| `trackfw release tag` | Wave 3 antes de ML-1A atualizar release.go | Nenhum gate em CI — só falha ao executar o comando | **Não capturado** — descoberto em produção |

---

#### Seção 4 — Residual declarado

O que este design aceita não cobrir, dito explicitamente:

**R1 — Audiência que recebe binário onde antes recebia código-fonte puro (da ADR).**
A ADR documenta explicitamente: "Não prova que um administrador que proíbe executáveis em geral
aceite um binário Go dentro de um tarball npm. Hoje esse usuário recebe Node e Python puros; com a
opção D receberia um binário, e estaria pior." Esta audiência é aceita como perda intencional.

**R2 — Remoção de um detector de defeito independente.**
Os três CLIs foram a principal fonte de achados na campanha de paridade de 2026-09. Remover as
reimplementações Node e Python remove a capacidade de detectar divergências de comportamento por
execução independente. Após v8, nenhuma ferramenta no repositório verifica que o binário Go se
comporta como os contratos documentados em `docs/cli-parity.md` prometem — só os smoke tests de
consumidor fazem isso, em granularidade muito mais grossa. Este residual não está declarado na ADR.

**R3 — `check-serve-api-file-security.sh` silente após Wave 3.**
Identificado na Seção 3. O gate usa `2>/dev/null` em pelo menos uma verificação de arquivo Python.
Após Wave 3 a verificação desaparece sem falha visível. A cobertura de segurança de serve reduz
sem alarme. Requer correção no ML que remove o arquivo (AC7 deve nomear este gate explicitamente).

**R4 — Falsify corpus encolhe sem alarme proporcional.**
`check-falsify-shard-coverage.sh` mede cobertura relativa ao corpus presente. Com 144 referências
Node/Python ausentes, a cobertura aparece completa sobre um corpus menor. Requer que ML-3B converta
ou descarte os cenários Node/Python antes de remover os CLIs, e que `gen-falsify-chunks.py` seja
re-calibrado com os pesos atualizados.

**R5 — Dependência implícita ML-1A → ML-3A não tem gate.**
A restrição "release.go deve ser atualizado antes de pypi/trackfw/ ser apagado" é uma dependência
de ordem entre waves que o roadmap implica mas não impõe mecanicamente. Nenhum gate em CI detecta
que `release.go` ainda referencia `pypi/trackfw/__init__.py`. A dependência existe apenas como
texto neste roadmap — um agente que não leia esta seção pode executar Wave 3 fora de ordem.

---

### Auditoria do arquiteto ao ML-0A — 2026-09-12

Verifiquei os quatro achados **por execução**, não pelo relatório.

**CRÍTICO 1 — CONFIRMADO.** `internal/commands/release.go:102-108`:

```go
{"pypi/pyproject.toml",                                  "pypi/pyproject.toml", ...},
{"pypi/trackfw/__init__.py (importlib.metadata fallback)","pypi/trackfw/__init__.py", ...},
{"pypi/trackfw/__init__.py (except fallback)",            "pypi/trackfw/__init__.py", ...},
```

🔴 O AC5 apaga `pypi/trackfw/`. Depois disso, **`trackfw release tag` recusa em toda invocação** — o
CLI do próprio projeto perde a capacidade de publicar release, e nenhum gate detecta antes do
runtime. Vira **dependência dura ML-1A → ML-3A**, registrada nos dois MLs.

**CRÍTICO 2 — CONFIRMADO.** `.github/required-status-checks.txt` declara `node`, `python (3.10)` e
`python (3.12)` — jobs que a Wave 3 remove.

🔴 **O conjunto R (branch protection) vive fora do repositório e o CI não consegue lê-lo.** Se os
jobs saírem sem R ser atualizado, **todo PR fica pendente para sempre — inclusive o PR que
consertaria isso.** Deadlock de repositório.

O próprio arquivo documenta o procedimento: *"Para remover um check: o processo inverso (…) As três
operações devem ocorrer no mesmo PR."* E `make check-required-full` detecta — mas exige credencial de
mantenedor e **só é obrigatório na release, não no PR**.

**ALTA — 🔴 NÃO CONFERE COMO DESCRITO. Rebaixada.** O relatório diz que
`check-serve-api-file-security.sh:122` ficaria *"silente, com exit 0 sem ter verificado nada"*. O
código tem `else fail`:

```bash
if grep -q "os\.path\.realpath" ".../pypi/trackfw/commands/serve.py" 2>/dev/null; then
  ok "..."
else
  fail "Python serve.py pode estar sem realpath — revisar AC7"
fi
```

Arquivo ausente ⇒ `grep` retorna 2 ⇒ `else` ⇒ **`fail`**. O gate fica **vermelho, não silencioso** —
que é o comportamento correto. Varri o arquivo: é o **único** sítio com esse padrão, e ele tem
guarda.

**O que sobra do achado, e continua válido:** a Wave 3 precisa **atualizar** esse gate ao remover o
arquivo, senão ele reprova por motivo alheio. É trabalho do ML-3C, não risco de cobertura silenciosa.

**MÉDIA — aceita sem reverificação.** O corpus de falsify encolher sem alarme proporcional é
plausível e o remédio (converter ou descartar cenários antes de remover, recalibrar pesos) é barato.
Registrado no ML-3B.

**Residual R2, acrescentado pelo ML-0A e que não estava na ADR:** depois da v8, **nenhuma ferramenta
verifica divergência de comportamento do binário Go por execução independente** — a paridade era, de
graça, um detector de defeito. Isso é perda real e vai declarada.

---

## Wave 1 — Construir o canal novo, sem remover nada
> Dependências: Wave 0. **Tudo reversível.** Os MLs 1A–1C são paralelos entre si; 1D e 1E dependem deles.

### ML-1A — **AC3 + AC4** — geração dos manifests e do release
**Status:** ✅ Concluído
**Adendo — auditoria 2026-09-13 (D6-write):** `build_wheel.py` recebia a versão crua do goreleaser
(`8.0.0-rc1`) e a usava em todos os campos internos da wheel (nome do arquivo, dist-info, .data,
METADATA `Version:`). `packaging.utils.parse_wheel_filename` rejeita `8.0.0-rc1` com
`InvalidWheelFilename: Invalid build number: rc1`. Corrigido: `normalize_version()` adicionada ao
início de `build_wheel.py`; gate `scripts/check-wheel-filename.sh` adicionado ao `parity-rest`
(usa versão de pré-lançamento — versão limpa `8.0.0` é idêntica nas duas grafias e passaria com
o defeito intacto). Falsificação em duas direções em `check-gates-falsify.sh` (cenários 182+183).
**Arquivos afetados:** `.github/workflows/release.yml`, novo script de geração, `Makefile`.
**Contexto:** hoje há **5 sítios de versão** e o cruzamento com o `CHANGELOG` só roda no
`release tag` — é o **issue #338**. Com N pacotes de plataforma seriam **5+N**.
🔴 **Gerar em vez de vigiar:** os `package.json` de plataforma **não existem no repositório**; são
artefato de build, como no esbuild. Sítios caem para **1**, e o #338 é **resolvido por esta REQ**.
🔴 **DEPENDÊNCIA DURA — ML-1A precede ML-3A.** `internal/commands/release.go:102-108` hardcoda
`pypi/trackfw/__init__.py` (duas vezes) e `npm/package.json` em `releaseVersionFiles`. Se o ML-3A
apagar `pypi/trackfw/` antes de este ML reescrever a lista, **`trackfw release tag` recusa em toda
invocação** e o projeto perde a capacidade de publicar. Nenhum gate pega antes do runtime.

**Critérios de aceite:**
- [ ] 🔴 `releaseVersionFiles` deixa de hardcodar caminhos que a Wave 3 remove — derivado do sítio único
- [ ] Falsificação: simular a árvore pós-Wave-3 e provar que `release tag` **ainda funciona**
- [ ] Um único sítio de versão; os manifests são gerados a partir dele
- [ ] Gate verifica que a geração aconteceu **e** bate com o `CHANGELOG`
- [ ] Falsificação: divergir o sítio único do CHANGELOG ⇒ reprova
- [ ] 🔴 Publicação parcial é **detectável** — a v7.6.0 publicou GitHub e PyPI e deixou o npm para
      trás por credencial, e o job saiu verde para os dois primeiros

### ML-1B — **AC1 + AC13** — casquinha npm de produção
**Status:** ✅ Concluído
**Adendo — 2026-09-13 (o que faltava):** `npm/package.json` ainda apontava para a implementação Node v7 (`bin/trackfw`, `main: src/commands/index.js`, `files: ["bin/", "src/"]`, deps de runtime). Corrigido: `bin.trackfw` → `./bin/trackfw.js`, `files` → `["bin/trackfw.js"]`, deps movidas para `devDependencies`, `main` removido (break de `require('trackfw')` — documentado para ML-3D), `scripts.smoke` → `node --check bin/trackfw.js`. O discriminante do release.yml que verificava `npm/src/` na árvore também foi corrigido (sempre inspeciona o tarball via `check-channels-content.sh --local`). `smoke-integration-packages.sh` npm arm atualizado para provar shim + platform package sem `bin` (AC13), delegação ao binário Go que embeds assets (go:embed). `check-integration-assets.sh` e `check-channels-content.sh` atualizados.
**Arquivos afetados:** `npm/bin/`, `npm/package.json`, `npm/package-lock.json`, `scripts/smoke-integration-packages.sh`, `scripts/check-integration-assets.sh`, `scripts/check-channels-content.sh`, `.github/workflows/release.yml`, `Makefile`.
🔴 **Não remover `npm/src/` ainda** — é Wave 3.
**Molde medido:** `prototype/packages/trackfw-shim/bin/trackfw.js`, **80 linhas**, já auditado.
**Critérios de aceite:**
- [x] Resolução **dinâmica** (`@trackfw-bin/${platform}-${arch}`), nunca mapa hardcoded — foi o
      defeito que a VM pegou
- [x] Pacotes de plataforma **sem `bin`** (AC13) **e** o shim resolvendo mesmo assim — 🔴 **as duas
      propriedades provadas juntas**; foi trocar uma pela outra que quebrou o CI no protótipo
- [x] Sem pacote de plataforma ⇒ aborta **nomeando a plataforma**, nunca `MODULE_NOT_FOUND`
- [x] Porte de `npm/tests/shim_packaging.test.js`, que hoje testa o protótipo

**Reconciliações (Regra Dura — CLAUDE.md):**
- `npm/tests/shim_packaging.test.js` Arm B: "sem campo `bin` no pacote de plataforma, o shim resolve pelo subcaminho calculado." (AC1+AC13 juntos — as duas propriedades provadas pela MESMA condição inicial)
- `npm/tests/shim_packaging.test.js` Arm C: "sem pacote de plataforma instalado, o shim aborta nomeando a plataforma, sem MODULE_NOT_FOUND." (AC1 + a propriedade de erro amigável)
- `smoke-integration-packages.sh` — `test -f .../bin/trackfw.js`: "shim presente no pacote instalado" afirma que `files: ["bin/trackfw.js"]` está correto e `bin.trackfw` aponta para ele.
- `smoke-integration-packages.sh` — `test ! -d .../src`: "src/ ausente do pacote instalado" afirma que a mudança em `files` remove a v7 da v8 — D7 fechado.
- `check-channels-content.sh` (shim): "no src/ directory" asserts D7 is closed — v7 tree would have src/. Falsificação: adicionar `src/` a `files` → exit 1 (provado).
- `check-channels-content.sh` (shim): "no bin/trackfw v7 entry" afirma que o entry-point Node v7 não vazou para o tarball v8.

🔴 **Nota para ML-3D:** a remoção de `main` de `npm/package.json` quebra `require('trackfw')` — este break é real e já está acontecendo desde este ML. ML-3D deve declarar no CHANGELOG.

### ML-1C — **AC2** — wheels PyPI
**Status:** ✅ Concluído
**Arquivos afetados:** `pypi/pyproject.toml`, build de wheels. 🔴 **Não remover `pypi/trackfw/`** — Wave 3.
**Molde medido:** wheel do `gh-bin` — `dist-info` + `.data/scripts/<bin>`, **zero arquivo Python**.
🔴 Tags: `manylinux_2_17_*` · `musllinux_1_2_*` · `macosx_*` · `win_amd64`. **`linux_x86_64` puro é
rejeitado pelo PyPI.**
**Critérios de aceite:**
- [ ] Wheel sem nenhum `.py`; binário em `.data/scripts/`
- [ ] `pip install` local sem rede e sem toolchain Go
- [ ] Avaliar `go-to-wheel` antes de escrever do zero — 🔴 ele **não** suporta layout `cmd/<nome>/`, medido no protótipo

### ML-1D — **AC9** — byte-identidade vira gate permanente
**Status:** ✅ Concluído
**Arquivos afetados:** novo `scripts/check-shim-byte-identity.sh`, `.github/workflows/quality.yml`.
**Contexto:** hoje isso é a **Pergunta 15 de uma sonda sob demanda**. Vira **gate**, nos 3 SOs.
**Critérios de aceite:**
- [ ] Roda em Linux, macOS e Windows **x64** do CI
- [ ] Compara saída **e** exit code, incluindo o de violação
- [ ] 🔴 Guarda de vacuidade: cenário que não monta ⇒ **reprova**, com causa nomeada
- [ ] 🔴 Distingue **ambiente incompleto** de defeito — o teste do protótipo derrubou 3 checks
      obrigatórios por não fazer isso

### ML-1E — **AC10** — instalação sob restrição vira gate
**Status:** ✅ Concluído
**Arquivos afetados:** novo gate, `Makefile`.
**Cenários, todos já provados no protótipo:** registry alternativo · `--ignore-scripts` · **sem rota
para github** · lockfile de um SO instalando em outro.
🔴 Use `npm install --offline`, **não** registry morto: a porta 1 não recusa, fica em `SYN_SENT` até
o timeout — custou 11 min por execução no probe.
**Critérios de aceite:**
- [ ] Os 4 cenários, cada um com prova de que a restrição estava **ativa**
- [ ] O `--offline` tem braço próprio provando que **falha quando precisa de rede**

### ML-1F — gate de byte NUL literal em fonte
**Status:** ✅ Concluído
**Origem:** `REQ-2026-09-12-byte-nul-literal-no-fonte-...` — 🔴 **só o gate entra aqui.** Os dois
arquivos com NUL são fontes do Node que a Wave 3 deleta; o defeito some sozinho. O **gate**
sobrevive e vale para qualquer fonte, inclusive Go.
**Critérios de aceite:**
- [x] Reprova se qualquer fonte rastreado contiver NUL literal, nomeando arquivo e offset
- [x] 🔴 **Não usa `grep`** para procurar o NUL — usa `LC_ALL=C tr -d -c '\000' | wc -c`
- [x] Guarda de vacuidade: varredura que não examina arquivo nenhum ⇒ reprova
- [x] Falsificação: 5 arms no --self-test (limpa/NUL não-declarado/exc-ausente/exc-zero-NUL/exc-contagem-divergente)
- [x] Decisão escolhida: Opção 1 (lista de exceção com prazo **estrutural**) — três modos de obsolescência, não calendário
- [x] Achado de varredura: 651 fontes texto examinados; NUL em exatamente 2 (os declarados) — sem terceiro

---

## Wave 2 — Publicar e provar em registry real
> Dependências: **Wave 1 inteira verde.** 🔴 Primeira etapa irreversível.

### ML-2A — publicação sob `v8.0.0-rc`
**Status:** ✅ Concluído
🔴 **Nome e versão no npm são permanentes após 72 h; o PyPI não reusa nome de arquivo.** Por isso
`-rc`, e por isso esta wave não começa antes da Wave 1 fechar.

**Publicado em 2026-09-13** — tag `v8.0.0-rc1`, commit `c4cd9f71`, run `34779379955`.

**Critérios de aceite:**
- [x] `npm install trackfw@rc` funciona em máquina limpa — medido em **Windows arm64** (VM real) e
      **macOS arm64**. 🔴 **Limite declarado:** as duas máquinas são arm64; **nenhum teste real
      cobriu x64 em nenhum SO**, e o `pip install` não foi exercido por falta de máquina limpa.
- [x] Instalação de versão anterior continua funcionando — `latest` permanece `7.6.0`;
      `npm install trackfw` → `7.6.0`, `npm install trackfw@rc` → `8.0.0-rc1` (D3 confirmado no registry)
- [x] 🔴 Falha parcial entre canais é detectada e reportada — **provado nas duas direções no mesmo
      run**: na 1ª tentativa `verify-channels` reprovou nomeando o canal (`FAIL: npm trackfw@8.0.0-rc1
      not found`); após o rerun, passou. É a correção do acidente da v7.6.0 demonstrada em produção.

**Estado publicado:**

| Canal | Resultado |
|---|---|
| GitHub | `v8.0.0-rc1`, 6 binários (windows/arm64 pela primeira vez) |
| npm | shim + 6 `@trackfw-bin`, dist-tag `rc`; `latest` intacto em 7.6.0 |
| PyPI | 8 wheels em `8.0.0rc1`, nenhum sdist |
| conteúdo | `check-channels-content --published`: 2 passed, 0 failed, **0 skipped** |

**Medição em máquina real (o que o CI não podia afirmar):**

| | Windows arm64 | macOS arm64 |
|---|---|---|
| pacote de plataforma baixado | `@trackfw-bin/win32-arm64` | `@trackfw-bin/darwin-arm64` |
| campo `bin` do pacote de plataforma | `undefined` (AC13) | `undefined` (AC13) |
| shim resolve mesmo assim (AC1) | ✅ `trackfw 8.0.0-rc1` | ✅ `trackfw 8.0.0-rc1` |
| idêntico ao binário nativo | ✅ | ✅ stdout+stderr e exit **também no caso de violação** |
| `src/` no pacote instalado | ausente | ausente |

🔴 **Achado que justifica retroativamente a regra "fechar por adição".** A VM é `win32 arm64` — a
plataforma que o `.goreleaser.yaml` **ignorava** até o D2 ser fechado hoje. Se o D2 tivesse sido
fechado **por remoção** (tirando o slug do gerador em vez de acrescentar o build), esta máquina
teria recebido `added 1 package` sem erro e falhado só na execução: instalação silenciosamente
incompleta. A regra do projeto pagou na primeira máquina real em que o pacote rodou.

**Causa da falha da 1ª tentativa (registrada para o ML-4A):** o escopo `@trackfw-bin` não existia no
npmjs.org (`404 Scope not found`), nos 6 jobs de plataforma. O shim foi corretamente **pulado** pelo
`needs:`, provando a ordem que o D4 estabeleceu. Classe de defeito que **nenhuma validação local
podia pegar** — não é código, workflow nem ambiente de runner, é estado de conta em serviço externo.
Custo total: um nome de prerelease no PyPI e ~20 min. Com a `8.0.0` final seria a versão definitiva
saindo pela metade — é exatamente para isso que a Wave 2 publica um `rc`.

**Pendência herdada pela Wave 3:** o bit de execução da wheel no `pip install` (o `external_attr` que
o ML-1C tratou) não foi exercido em máquina limpa. Sem ele o `pip install` passa e o comando falha
com permissão negada. Não se aplica a Windows; aplica-se a Linux e macOS.

**Testes alterados e conclusão que cada um afirma (regra de reconciliação):**

| Arquivo | Regex/assertion alterada | Conclusão afirmada por este ML |
|---|---|---|
| `internal/commands/version_test.go` | `versionLineRE` — adicionado grupo `(-[a-zA-Z0-9]+(\.[a-zA-Z0-9]+)*)?` | `trackfw version` e `trackfw --version` emitem `trackfw 8.0.0-rc1` (formato semver com pre-release aceito pelo contrato) |
| `npm/tests/version.test.js` | `VERSION_RE` — mesmo sufixo opcional | `trackfw version` no Node emite `trackfw 8.0.0-rc1` |
| `pypi/tests/test_commands_basic.py` | `_CANONICAL_RE` — mesmo sufixo opcional | `trackfw version` no Python emite `trackfw 8.0.0-rc1` |
| `scripts/check-cli-parity.sh` | `_VERSION_RE` — mesmo sufixo opcional | o gate de paridade valida `trackfw 8.0.0-rc1` como formato canônico e compara os 3 runtimes byte-a-byte |
| `scripts/check-gates-falsify.sh` | liveness probe s23 — mesmo sufixo opcional | o binário corrompido que aceita `-v` imprime versão no formato correto (incluindo pre-release), provando que o seam está ativo antes de rodar o gate |
| `scripts/check-doctor-parity.sh` | normalização `v[\w.]+` → `v[\w.-]+` | strings de versão com hífen (e.g. `v8.0.0-rc1`) são completamente normalizadas para `vTEST` antes da comparação byte-a-byte |
| `scripts/check-manifest-version-gate.sh` | awk — suporte a formato simples `__version__ = "X"` | `pypi/trackfw/__init__.py` com valor hardcoded (sem try/except) é verificado como literal único; 10 assertions (antes 11: try-branch + except-branch eram 2 assertions para o mesmo arquivo; agora 1 literal = 1 assertion, sem perda de cobertura) |
| `npm/tests/ci_workflow_version_pin.test.js` | `semverLiteral` — adicionado grupo `(-[a-zA-Z0-9]+(\.[a-zA-Z0-9]+)*)?` + 2 arms de falsificação (direction-A: literal injetado é detectado; direction-B: bloco limpo passa) | o guard que proíbe literal de versão no bloco gerador de workflows captura `"8.0.0-rc1"` e `"v8.0.0-rc1"` — literais de prerelease que a regex anterior (`\d+\.\d+\.\d+` somente) deixava escapar sem detecção |

**Mudança de comportamento em `pypi/trackfw/__init__.py`:** `importlib.metadata.version('trackfw')` substituído por `__version__ = "8.0.0-rc1"` hardcoded. Um pacote instalado agora reporta a versão compilada na fonte, não a versão do pacote instalado. Deriva é detectada pelo gate. Wave 3 remove o arquivo inteiro; esta constante é transitória.

---

## Wave 3 — Remover
> Dependências: **Wave 2 verde.** Nada aqui começa antes de o canal novo estar publicando.

### ML-3A — **AC5** — remover as reimplementações
**Status:** ✅ Concluído

**Auditoria do arquiteto (2026-09-16):** `npm/src/` e `pypi/trackfw/` ausentes da árvore.
Referências residuais em `internal/**` são **comentários históricos e testes-guarda** — em
particular `internal/commands/release_test.go:237` e `TestReleaseVersionFiles_NoWave3DeletedPaths`
**impedem a reintrodução** de caminhos sob `pypi/trackfw/` ou `npm/src/` em `releaseVersionFiles`.
Nenhuma referência operacional em `Makefile`, workflows ou código. `go build ./...` RC=0.

`npm/src/` (26.272 linhas) e `pypi/trackfw/` (27.472).

### ML-3B — **AC6** — suítes
**Status:** ✅ Concluído

**Auditoria do arquiteto (2026-09-16):** `npm/tests/` (87 arquivos) e `pypi/tests/` (89) removidos.
🔴 O ponto crítico do ML foi cumprido: `shim_packaging.test.js` **foi movido**, não apagado — vive
em `npm/shim_packaging.test.js` e entrou no commit de preservação como arquivo novo (a árvore antiga
aparece como deleção; o conteúdo sobreviveu). Verificado por leitura do cabeçalho do arquivo.

`npm/tests/` e `pypi/tests/` testam a reimplementação. O que resta é **smoke**: a casquinha lança o
binário, repassa argv, devolve exit code.
🔴 `npm/tests/shim_packaging.test.js` **não é da reimplementação** — é do empacotamento, e sobrevive.
**Mover, não apagar** (está escrito no topo do arquivo).

### ML-3C — **AC7** — gates que perdem objeto
**Status:** ✅ Concluído — segunda metade implementada; defeito de autorrelato corrigido em ML-3C-bis (ver seção abaixo e "Auditoria do arquiteto — 2026-09-16")

#### Primeira metade concluída — extração de pins (PR separado, sem deleções)

**Gate novo extraído:** `scripts/check-validate-rule-pins.sh`
— 25 pins comportamentais extraídos de `check-validate-parity.sh` antes da deleção.
— Wired em `Makefile` (parity-rest) e `.github/workflows/release.yml`.
— `check-orphan-gates.sh` verde; `check-workflow-yaml.py` 8/8; build e testes Go verdes.

**Falsificação em duas direções (3 pins representativos):**
- PIN1 (rule-set): renomear `adr_accepted_when_req_done` → `adr_accepted_when_req_MUTATED` em `validator.go` → gate reprova com "missing rule(s) ['adr_accepted_when_req_done']". Restore → verde.
- PIN5 (--agent guidance): mudar `--agent <agent>` → `--scope <agent>` em `validator.go` → gate reprova com "orientation message must contain '--agent'". Restore → verde.
- PIN13 (invalid JSON): mudar "is not valid JSON" → "has invalid JSON syntax" em `validator_credential_guard.go` → gate reprova com "message does not contain 'is not valid JSON'". Restore → verde.

#### Lista nomeada — 67 gates (classificação por balde)

**DELETAR** — só compara runtimes (comparação morre com Node/Python), sem pin próprio:

| Gate | O que media |
|---|---|
| check-agent-hooks-parity.sh | Byte-identidade de hooks gerados (Go/Node/Py) |
| check-agents-install-yaml-parity.sh | trackfw.yaml pós `agents install` nos 3 runtimes |
| check-artifact-closed-cycle.sh | Ciclo fechado de artefatos nos 3 runtimes |
| check-artifact-parity.sh | Byte-identidade de artefatos gerados |
| check-attention-scripts-parity.sh | Scripts de atenção byte-idênticos nos 3 runtimes |
| check-audit-surface.sh | Surface de auditoria nos 3 runtimes |
| check-branch-new-parity.sh | `trackfw branch new` nos 3 runtimes |
| check-branch-prune-parity.sh | `trackfw branch prune` nos 3 runtimes |
| check-cli-parity.sh | `--help`, `version`, `-v` nos 3 runtimes |
| check-commit-parity.sh | `trackfw commit` nos 3 runtimes |
| check-consumer-smoke-by-agent.sh | Smoke test by-agent nos 3 runtimes |
| check-doctor-parity.sh | `trackfw doctor` nos 3 runtimes |
| check-doctor-remote-parity.sh | `trackfw doctor --remote` nos 3 runtimes |
| check-harness-hooks-parity.sh | Hooks do harness nos 3 runtimes |
| check-homedir-parity.sh | Comportamento de $HOME nos 3 runtimes |
| check-identity-parity.sh | Identidade de arquivos gerados nos 3 runtimes |
| check-integration-cli-parity.sh | CLI de integração nos 3 runtimes |
| check-push-force-parity.sh | `trackfw push --force` nos 3 runtimes |
| check-push-parity.sh | `trackfw push` nos 3 runtimes |
| check-roadmap-barrier-contract.sh | Contrato de barreira de roadmap nos 3 runtimes |
| check-roadmap-move-parity.sh | `trackfw roadmap move` nos 3 runtimes |
| check-rules-parity.sh | Bloco de regras gerado nos 3 runtimes |
| check-ship-force-parity.sh | `trackfw ship --force` nos 3 runtimes |
| check-ship-parity.sh | `trackfw ship` nos 3 runtimes |
| check-unknown-command-parity.sh | Comando desconhecido nos 3 runtimes |
| check-update-parity.sh | `trackfw update` nos 3 runtimes |
| check-atomic-write-anti-divergence.sh | Anti-divergência entre 3 cópias Python do _atomic_write — objeto desaparece (sem Python) |
| check-python-writes-lf.sh | Saída Python em LF — objeto desaparece (sem Python) |
| check-shell-posix-portability.sh | barrier.js/barrier.py usam `sh -c` — objeto desaparece (sem npm/pypi fontes) |

**PIN EXTRAÍDO** — tinha pin próprio embutido, extraído para gate novo:

| Gate original | Gate extraído | Pins |
|---|---|---|
| check-validate-parity.sh | check-validate-rule-pins.sh | 25 pins (rule-set, bhr-messages, credential-guard, git-branch-guard) |

**SOBREVIVE** — sem dependência de runtime Node/Python, continua como está:

| Gate | Por que sobrevive |
|---|---|
| check-ci-workflow-job-id-collision.sh | Lê workflow YAML, não invoca CLI |
| check-falsify-shard-coverage.sh | Analisa scripts/check-*.sh, sem runtime |
| check-install-checksum.sh | Verifica checksum de artefato Go |
| check-install-version-pin.sh | Verifica pin de versão de instalação |
| check-no-literal-nul-in-source.sh | Scan de NUL em código-fonte, sem runtime |
| check-orphan-gates.sh | Meta-gate: referência de gates no Makefile/workflow |
| check-parity-call-site-pins.sh | Só Go, pins de call-site |
| check-parity-contract-coverage.sh | Meta-checker docs/cli-parity.md |
| check-platform-matrix-parity.sh | Verifica matrix de plataformas no CI |
| check-pr-closing-keyword.sh | Verifica keyword de fechamento de PR |
| check-referential-integrity.sh | Integridade referencial de artefatos |
| check-job-annotations.py | Verifica annotations nível failure em jobs success via GitHub API (workflow_run); sem dependência de CLI; sobrevive sem mudança |

**REESCREVER** — mede algo real mas invoca Node/Python CLI ou lê fontes npm/pypi:

| Gate | O que sobrevive (Go/shell) | O que morre (npm/pypi) |
|---|---|---|
| check-agent-models-parity.sh | Composição de model IDs por tier (Go) | Comparação Node/Py |
| check-agent-namespace-union.sh | União namespace disk+declarado (Go) | Comparação Node/Py |
| check-barrier.sh | Exit code e msg de barrier (Go) | Comparação Node/Py |
| check-channels-content.sh | Conteúdo de canais Go (bin/) | npm/pypi channels |
| check-ci-workflow-pin-parity.sh | Pins de workflow CI (sem runtime) | Invocação Python CLI |
| check-git-branch-guard-hook-schema.sh | Schema hookSpecificOutput (Go + shell) | Geradores Node/Py |
| check-integration-assets.sh | Assets Go binário | Assets npm/pypi |
| check-install-restriction.sh | Restrição de instalação Go | npm/pypi restriction |
| check-manifest-version-gate.sh | Versão em version.go e go.mod | npm/package.json, pypi |
| check-output-encoding-declared.sh | PYTHONIOENCODING em gates (tool) | attentionSignalScript em npm/pypi |
| check-raw-read-ban.sh | Ban de raw read em internal/*.go | Ban em npm/src, pypi |
| check-ref-separator-portability.sh | normalizeRefSeparator em internal/ | npm/src, pypi |
| check-release-tag-parity.sh | 9 refusal literals + SHA linkage (Go) | Comparação Node/Py |
| check-serve-address-parity.sh | Bind address 0.0.0.0 (Go) | Comparação Node/Py |
| check-serve-api-file-security.sh | Segurança de API serve (Go) | pypi/trackfw/commands/serve.py |
| check-serve-browser-security.sh | Segurança browser (Go) | Node/Py |
| check-shim-byte-identity.sh | Shim Go binary identity | Node shim identity |
| check-slash-parity.sh | EXPECTED_COMMANDS (Go) | Comparação Node/Py |
| check-static-assets.sh | Assets estáticos Go | Assets npm/pypi |
| check-symlink-privilege-guard.sh | Guarda symlink em sources Go | npm/src, pypi |
| check-thirdparty-parity.sh | Test coverage Go thirdparty_test.go | Node/Py tests |
| check-tty-detection.sh | TTY detection Go | Python tty |
| check-wheel-filename.sh | Nome do wheel PyPI (CI-only) | pypi wheel |
| check-windows-known-failures.py | Ratchet de falhas Windows conhecidas (D1/D2 core sobrevive) | D4 `removal_note='corrected'` lê output Go/Node/Py — Node/Py arms morrem |

**31 dos 61** são de paridade. 🔴 **Nomear um a um**, dizendo o que cada um media e por que não mede
mais nada. Remover em bloco é como se perde cobertura sem perceber.
🔴 **`.github/required-status-checks.txt` declara `node`, `python (3.10)` e `python (3.12)`** — jobs
que esta wave remove. O conjunto **R** (branch protection) vive **fora do repositório** e o CI não o
lê. Remover os jobs sem atualizar R deixa **todo PR pendente para sempre, inclusive o que
consertaria** — deadlock de repositório.

**As três operações no MESMO PR**, como o próprio arquivo prescreve: remover do workflow, remover da
declaração, remover de `required_status_checks` via API. 🔴 A terceira **exige credencial de
mantenedor** — não é operação de agente.

**Critérios de aceite:**
- [ ] 🔴 `make check-required-full` verde **antes** do merge do PR que remove os jobs
- [ ] `check-serve-api-file-security.sh` atualizado ao remover `pypi/trackfw/commands/serve.py` —
      ele **reprova alto** com o arquivo ausente (verificado), então precisa ser reescrito, não só observado
- [ ] Lista nomeada, com justificativa por gate
- [ ] Gate que ainda mede algo **fica** — paridade não é o único motivo de um gate existir

#### 🔴 Medição prévia do arquiteto — os 31 NÃO são um bloco

Feita em 2026-09-12, para quem executar este ML não redescobrir. **Não é o resultado do AC — é o
ponto de partida**, e o AC continua exigindo nomear um a um.

**Prova de que tratar como bloco perde cobertura**, `scripts/check-validate-parity.sh:200`:

```python
expected_rules = {"adr_accepted_when_req_done", "blocked_by_draft_adr"}
missing = expected_rules - got_rules
```

🔴 Isso **não compara runtimes** — é um *pin* no conjunto de regras esperado, afirmado por runtime.
Com um runtime só, *"o validate tem que emitir estas regras para esta fixture"* **continua sendo
contrato válido**. O arquivo carrega **duas coisas**: a comparação cross-runtime (morre) e um pin de
comportamento (sobrevive).

**Deletar o arquivo inteiro perde o pin sem ninguém perceber** — e é o tipo de perda que só aparece
meses depois, quando o defeito que ele pegava volta.

**Quatro baldes, medidos por quantos runtimes cada gate invoca:**

| balde | quantos | o que fazer |
|---|---|---|
| comparam os 3 runtimes | ~27 | a comparação morre — **mas extrair pins antes** |
| carregam pin próprio junto | ≥1 confirmado | 🔴 **extrair para gate próprio antes de deletar** |
| não invocam CLI nenhum | 3 | sobrevivem com reescrita |
| **novo** | +1 | shim ↔ binário nativo (ML-1D) |

**Os três que não invocam runtime** (`go=0 node=0 py=0` ou só Go):

```
check-parity-contract-coverage   meta-checker de docs/cli-parity.md — verifica que cada
                                 contrato NOMEIA um gate. Vira doc de canais, mas a ideia
                                 de "todo contrato nomeia seu gate" sobrevive intacta
check-parity-call-site-pins      só Go
check-ci-workflow-pin-parity     compara arquivos de workflow, não runtimes
```

**O saldo real não é "31 deletados".** É *31 gates de comparação tripla trocados por 1 de comparação
shim↔nativo, mais os pins extraídos antes*.

**Critério de aceite acrescentado:**
- [ ] 🔴 **Todo gate deletado é lido antes**, procurando asserção que **não** seja comparação entre
      runtimes. O que for pin de comportamento é **extraído para gate próprio**, não perdido junto.
      Falsificação: mutar o comportamento que o pin protegia ⇒ algum gate ainda reprova.

#### Tabela de auditoria — check-gates-falsify.sh (Ártemis, 2026-09-14)

Decisão por cenário. Regra: REMOVE exige declaração escrita de por que o cenário não afirma NADA sobre Go. Default é MANTÉM.

| Cenário | Rótulo da asserção (linha) | Invoca Node/Py? | Decisão | Justificativa |
|---------|---------------------------|-----------------|---------|---------------|
| 1 | `static-assets/byte-drift` (L1423) | Não | REMOVE | `check-static-assets.sh` foi reescrito; `check_destination()` existe mas nunca é chamada — mensagem "byte drift" jamais é emitida |
| 2 | `integration-assets/byte-drift` (L1445) | Não | REMOVE | `check-integration-assets.sh` idem: `check_destination()` definida, não chamada — mensagem inalcançável |
| 3 | `identity-parity/slug-drift` (L1468) | Sim (node+py) | REMOVE | `check-identity-parity.sh` AUSENTE em disco; setup_npm_tree() falha |
| 3b | `identity-parity/catalog-target-missing` (L1534) | Sim (node+py) | REMOVE | mesmo gate AUSENTE |
| 4 | `validate-parity/rule-removed` (L1562) | Sim (node+py) | REMOVE | `check-validate-parity.sh` AUSENTE; setup_npm_tree() falha |
| 5 | `cli-parity/missing-command` (L1586) | Sim (node+py) | REMOVE | `check-cli-parity.sh` AUSENTE |
| 6 | `integration-cli-parity/missing-agents` (L1608) | Sim (node+py) | REMOVE | `check-integration-cli-parity.sh` AUSENTE |
| 7 | `artifact-parity/req-content-drift` (L1645) | Sim (node+py) | REMOVE | `check-artifact-parity.sh` AUSENTE |
| 8 | `artifact-parity/req-name-drift` (L1707) | Sim (node+py) | REMOVE | mesmo gate AUSENTE |
| 9 | `artifact-parity/slash-roadmap-content-drift` (L1736) | Sim (node+py) | REMOVE | mesmo gate AUSENTE |
| 10 | `cli-parity/roadmap-new-flag-drift` (L1773) | Sim (py) | REMOVE | `check-cli-parity.sh` AUSENTE |
| 11 | `artifact-parity/by-agent-log-drift` (L1803) | Sim (node+py) | REMOVE | `check-artifact-parity.sh` AUSENTE |
| 12 | `referential-integrity/missing-roadmap` (L1832) | Não | MANTÉM | `check-referential-integrity.sh` PRESENTE (SOBREVIVE); asserção Go pura |
| 13 | `barrier/blocked-not-detected` (L1849) | Não | MANTÉM | `check-barrier.sh` PRESENTE (REESCREVER Go-only); seam `BARRIER_SELFTEST_BREAK=1` Go puro |
| 14 | `slash-parity/status-content-drift` (L1887) | Sim (node) | REMOVE | Gate reescrito Go-only em ML-3A; mensagem "go vs node" não existe mais; setup corrompe npm/src (AUSENTE) |
| 15 | `slash-parity/status-name-drift` (L1919) | Sim (node) | REMOVE | idem cenário 14 |
| 16 | `rules-parity/content-drift` (L1952) | Sim (node) | REMOVE | setup_npm_tree() falha (npm/src AUSENTE); afirma comportamento Node |
| 17 | `update-parity/dry-run-write-leak` (L1988) | Sim (node) | REMOVE | setup_npm_tree() falha; afirma comportamento Node de dry-run |
| 18 | (inline) `falsify/no-repo-mutation` | Não (Go gates) | PODA O BRAÇO | Remover `check-roadmap-move-parity.sh` de GATES_MUTATION_CHECK (gate AUSENTE); os outros 4 gates permanecem: check-update-parity.sh, check-barrier.sh, check-slash-parity.sh, check-rules-parity.sh |
| 19 | `barrier/early-break-after-target-not-detected` (L2054) | Não | MANTÉM | `check-barrier.sh` Go-only; seam `BARRIER_BIS_SELFTEST_BREAK=1` |
| 20 | `roadmap-move-parity/discriminant-wrong-order-not-detected` (L2094) | Sim (node+py) | REMOVE | `check-roadmap-move-parity.sh` AUSENTE |
| 21 | `cli-parity/version-v-prefix` (L2127) | Sim (node+py) | REMOVE | `check-cli-parity.sh` AUSENTE |
| 22 | `cli-parity/version-byte-mismatch` (L2163) | Sim (node+py) | REMOVE | mesmo gate AUSENTE |
| 23 | `cli-parity/v-flag-accepted` (L2238) | Sim (node+py) | REMOVE | mesmo gate AUSENTE |
| 24 | `roadmap-acceptance-heading/go/*` (L2350), `node/*` (L2372), `python/*` (L2394) | Sim (node+py) | PODA O BRAÇO | Remover braços node e python do loop; braço go (assert_fails_with no loop para Go) afirma corretamente |
| 25 | `roadmap-req-frontmatter-path/go/from-req-baseline` (L2454), `go/from-req` (L2479), `node/from-req-baseline` (L2490), `node/from-req` (L2506), `python/from-req-baseline` (L2517), `python/from-req` (L2533) | Sim (node+py) | PODA O BRAÇO | Remover braços node e python; braços Go afirmam comportamento real (`req_path` em frontmatter) |
| 26 | `roadmap-req-frontmatter-path/go/simple-baseline` (L2595), `go/simple-detects-regression` (L2619), `node/simple-baseline` (L2628), `node/simple-detects-regression` (L2643), `python/simple-baseline` (L2652), `python/simple-detects-regression` (L2667) | Sim (node+py) | PODA O BRAÇO | Remover braços node e python; braços Go afirmam AC2b |
| 27 | `adr-not-accepted/go/*` (L2715-2761), `node/*` (L2775-2809), `python/*` (L2824-2860) | Sim (node+py) | PODA O BRAÇO | Remover braços node e python; braços Go afirmam `adr_accepted_when_req_done` e `blocked_by_draft_adr` em Go |
| 28 | `backtick-ref/go/*` (L2904-2925), `node/*` (L2937-2952), `python/*` (L2965-2989) | Sim (node+py) | PODA O BRAÇO | Remover braços node e python; braço Go afirma backtick-ADR sem campo frontmatter adr: |
| 29–38 | `unpaired-delimiter`, `status-*`, `wip-limit-*`, `config-*`, `roadmap-cycle-*` (L3250–L4190) | Sim (node+py) | PODA O BRAÇO | Bloco de cenários 29-38 tem braços Go + Node + Python em cada; remover braços node e python; braços Go afirmam contrato de validate/status em Go |
| 39 | (inline) `update-config-loader/go-baseline`, `go-detects-artisanal-scanner-reintroduced` | Não | MANTÉM | Cenário Go puro; testa loadUpdateConfig() com Go binary direto |
| 40 | (inline) `update-config-loader/node-baseline`, `node-detects-artisanal-scanner-reintroduced` | Sim (node) | REMOVE | Invoca `node npm/bin/trackfw update`; setup_npm_tree() falha (npm/src AUSENTE) |
| 41 | (inline) `update-config-loader/python-baseline`, `python-detects-artisanal-scanner-reintroduced` | Sim (py) | REMOVE | Invoca `PYTHONPATH=$ROOT_DIR/pypi python3 -m trackfw update`; pypi/trackfw AUSENTE |
| 42 | `branch-new-parity/no-match/go-vs-node/err-message-reformatted-not-detected` (L4540) | Sim (node) | REMOVE | `check-branch-new-parity.sh` AUSENTE |
| 43 | `attention-scripts-parity/trackfw-attention-cleanup.sh/go-vs-py-comment-drift-not-detected` (L4581) | Sim (py) | REMOVE | `check-attention-scripts-parity.sh` AUSENTE |
| 44 | `agent-hooks-parity/kiro/go-vs-node-matcher-drift-not-detected` (L4627) | Sim (node) | REMOVE | `check-agent-hooks-parity.sh` AUSENTE |
| 45 | `harness-hooks-parity/kiro/go-vs-py-matcher-drift-not-detected` (L4676) | Sim (py) | REMOVE | `check-harness-hooks-parity.sh` AUSENTE |
| 46 | (inline) `credential-guard-hook-resolvable/detected` (L5039) | Não | MANTÉM | Invoca Go binary diretamente; testa `credential_guard_hook_resolvable` sem Node/Py |
| 47 | `attention-scripts-parity/trackfw-credential-guard.sh/go-vs-node-composition-reordered-not-detected` (L5089) | Sim (node) | REMOVE | `check-attention-scripts-parity.sh` AUSENTE |
| 48 | (inline) `credential-guard-script-integrity/detected` (L5198) | Não | MANTÉM | Go binary direto; testa integridade do script de credential guard |
| 49 | `credential-guard-mode-downgrade/detected` (L5333), `non-vacuity` (L5360) | Não | MANTÉM | Go binary direto; sem Node/Py |
| 50 | `credential-guard-anchoring-combined-edit/detected` (L5390), `legitimate-committed-off-silences` (L5409) | Não | MANTÉM | Go binary direto |
| 51 | `credential-guard-anchoring-non-regression/filename-uniqueness-baseline` (L5577), `off-uncommitted-still-silences` (L5598) | Não | MANTÉM | Go binary direto |
| 52–54 | `credential-guard-git-env-bypass/redirect-detected` (L5746), `config-count-detected` (L5755), `worktree-legitimate-detection` (L5791) | Não | MANTÉM | Go binary direto |
| 55 | `unknown-command-parity/text-drift/python-baseline` (L5844), `python-detects-regression` (L5859) | Sim (py) | REMOVE | `check-unknown-command-parity.sh` AUSENTE |
| 56 | `unknown-command-parity/exit-code-drift/node-baseline` (L5879), `node-detects-regression` (L5894) | Sim (node) | REMOVE | mesmo gate AUSENTE |
| 57 | `unknown-command-parity/missing-suggestion/go-baseline` (L5922), `go-detects-regression` (L5948) | Não | REMOVE | Gate `check-unknown-command-parity.sh` AUSENTE; mesmo com label /go, o cenário invoca o gate AUSENTE como orquestrador — sem o gate, a asserção nunca roda |
| 58 | (inline) Node+Python error handling | Sim (node+py) | REMOVE | Sem asserção Go; testa comportamento de erro de Node e Python que foram deletados |
| 59 | `serve-address-parity/wildcard-bind-regression/python-baseline` (L6191), `python-detects-regression` (L6206) | Sim (node+py) | REMOVE | setup_npm_tree() falha (npm/src AUSENTE); corrompe pypi/trackfw/commands/serve.py (AUSENTE) |
| 60–65 | `trackfw-git-branch-guard/…` (L6250–L6724 aprox.) | Não | MANTÉM | `scripts/trackfw-git-branch-guard.sh` Go shell script; sem Node/Py |
| 66 | `harness-hooks-parity/kiro/git-branch-guard/go-vs-py-matcher-drift-not-detected` (L6912) | Sim (py) | REMOVE | `check-harness-hooks-parity.sh` AUSENTE |
| 67 | (inline) dedup projeto+global para git-branch-guard | Não | MANTÉM | Go binary direto; testa deduplificação de escopo global |
| 68 | (inline) `git-branch-guard-global-script-integrity/detected-without-wiring` (L7224) | Não | MANTÉM | Go binary direto |
| 69 | `git-branch-guard-global-hook-resolvable/kiro-dedicated-file/detected` (L7391) | Não | MANTÉM | Go binary direto |
| 70 | `ship-parity/squash-merge-warning-false-positive` (L7450) | Sim (node+py) | REMOVE | `check-ship-parity.sh` AUSENTE |
| 71 | `doctor-parity/registered-under-different-claim-false-positive` (L7492) | Sim (node+py) | REMOVE | `check-doctor-parity.sh` AUSENTE |
| 72 | `doctor-parity/registered-under-different-claim-content-drifted-false-positive` (L7534) | Sim (node+py) | REMOVE | mesmo gate AUSENTE |
| 73 | `ship-force-parity/remote-advanced-lease-mismatch-raw-force-false-negative` (L7578) | Sim (node+py) | REMOVE | `check-ship-force-parity.sh` AUSENTE |
| 74 | (inline) `trackfw-git-branch-guard/…` (L7584–L7879 aprox.) | Não | MANTÉM | Go shell script; sem Node/Py |
| 75 | `release-tag-parity/success-lightweight-tag-false-negative` (L7918) | Não | MANTÉM | `check-release-tag-parity.sh` PRESENTE, reescrito Go-only em ML-3A |
| 76 | `release-tag-parity/forge-commit-diverges-false-negative` (L7961) | Não | MANTÉM | mesmo gate Go-only |
| 77 | `parity-contract-coverage/*` (L8037–L8236) | Não | MANTÉM | `check-parity-contract-coverage.sh` PRESENTE (SOBREVIVE); sem Node/Py |
| 78 | `agent-hooks-parity/amazonq/go-vs-node-tools-drift-not-detected` (L8275) | Sim (node) | REMOVE | `check-agent-hooks-parity.sh` AUSENTE |
| 79 | `validate-parity/branch-has-wip-roadmap-done-acceptance-not-detected` (L8342) | Sim (node+py) | REMOVE | `check-validate-parity.sh` AUSENTE |
| 80 | `validate-parity/credential-guard-hook-resolvable-not-detected` (L8418) | Sim (node+py) | REMOVE | mesmo gate AUSENTE |
| 81 | `validate-parity/credential-guard-noexec-not-detected` (L8473) | Sim (node+py) | REMOVE | mesmo gate AUSENTE |
| 82 | `validate-parity/credential-guard-notype-not-detected` (L8520) | Sim (node+py) | REMOVE | mesmo gate AUSENTE |
| 83 | `agent-hooks-parity/amazonq/denied-commands-not-detected` (L8582) | Sim (node) | REMOVE | `check-agent-hooks-parity.sh` AUSENTE |
| 84 | `artifact-parity/claude-md-architect-responses-section-node` (L8619) | Sim (node) | REMOVE | `check-artifact-parity.sh` AUSENTE |
| 85 | `nil-map-init/parse-missing-causes-panic-on-agent-models` (L8673) | Não | MANTÉM | Invoca Go binary com fixture; sem Node/Py |
| 86 | `agent-models-parity/namespace-guard-removed-causes-gemini-leak` (L8765) | Não | MANTÉM | Go binary direto; muta internal/render.go; sem Node/Py |
| 87 | `release-tag-parity/content-from-commit-false-negative` (L8828) | Não | MANTÉM | `check-release-tag-parity.sh` Go-only |
| 158 | `release-tag-parity/refs-replace-bypass-false-negative` (L8888) | Não | MANTÉM | mesmo gate Go-only |
| 159 | `validate-parity/credential-guard-bare-relative-not-detected` (L8944) | Sim (node+py) | REMOVE | `check-validate-parity.sh` AUSENTE |
| 160 | `validate-parity/credential-guard-copilot-false-positive-detected` (L8995) | Sim (node+py) | REMOVE | mesmo gate AUSENTE |
| 161 | `push-parity/feat-governance-blocked/exit-code` (L9043) | Sim (node+py) | REMOVE | `check-push-parity.sh` AUSENTE |
| 162 | `push-parity/feat-governance-ok-no-upstream/go` (L9100) | Não | REMOVE | Gate `check-push-parity.sh` AUSENTE; label /go não basta — o gate que orquestra é AUSENTE |
| 163 | `push-force-parity/pr-open-gate-removed/go` (L9143) | Não | REMOVE | `check-push-force-parity.sh` AUSENTE; mesmo argumento do cenário 162 |
| 164 | `validate-parity/credential-guard-pwd-not-detected` (L9194) | Sim (node+py) | REMOVE | `check-validate-parity.sh` AUSENTE |
| 165 | `validate-parity/credential-guard-absolute-path-accused` (L9265) | Sim (node+py) | REMOVE | mesmo gate AUSENTE |
| 166 | `artifact-parity/wave0-removed-synced-detected` (L9313) | Sim (node+py) | REMOVE | `check-artifact-parity.sh` AUSENTE |
| 167 | `barrier/wave-zero-rejected-again-detected` (L9358) | Não | MANTÉM | `check-barrier.sh` Go-only; muta barrier.go via Go source |
| 168 | `barrier/wave-zero-flag-guard-rejected-again-detected` (L9404) | Não | MANTÉM | idem; segundo guarda AC9 |
| 169 | `global-scope/direction-a-reads-cwd-detected` (L9446) | Não | MANTÉM | `check-agent-models-parity.sh` PRESENTE, reescrito Go-only; muta integrations_flags.go |
| 170 | `global-scope/direction-b-reads-global-detected` (L9485) | Não | MANTÉM | idem; direção B |
| 171 | `ac2-sanitization/direction-a-detected` (L9525) | Não | MANTÉM | `check-roadmap-barrier-contract.sh` PRESENTE Go-only (python3 usado apenas como ferramenta de parsing JSON, não como CLI trackfw) |
| 172 | `trust-check/direction-b-detected` (L9563) | Não | MANTÉM | idem; muta barrier.go |
| 173 | `audit-surface/direction-a-detected` (L9584) | Não | REMOVE | `check-audit-surface.sh` AUSENTE; baseline check falha na abertura do script |
| 174 | `audit-surface/direction-b-detected` (L9601) | Não | REMOVE | mesmo gate AUSENTE; reusa baseline do 173 que já falhou |
| 175 | `sandbox-gap-e/direction-a-detected` (L9641) | Não | MANTÉM | `check-update-parity.sh` PRESENTE, Go-only; muta update.go |
| 176 | `sandbox-walkdir-reintroduced/direction-b-detected` (L9710) | Não | MANTÉM | idem; direção B |
| 177 | `scaffold-divergent-silenced/direction-a-detected` (L9755) | Não | REMOVE | `check-doctor-parity.sh` AUSENTE; baseline check falha |
| 178 | `scaffold-intact-accused/direction-b-detected` (L9797) | Não | REMOVE | mesmo gate AUSENTE |
| 179 | `scaffold-mode-check-silenced/direction-a-detected` (L9844) | Não | REMOVE | mesmo gate AUSENTE; baseline em check-doctor-parity.sh |
| 180 | (inline) `scaffold-update-chmod-removed/direction-b` | Não | MANTÉM | Invoca `trackfw doctor` Go binary direto; sem gate intermediário; python3 NÃO usado |
| 181 | (inline) `scaffold-update-chmod-removed/direction-c-baseline`, `direction-c-detected` | Não | MANTÉM | Invoca `trackfw update` Go binary; python3 usado apenas para editar arquivo Go fonte (ferramenta de scripting, não CLI trackfw) |
| 182 | `pr-closing-keyword/isencao-por-numero-baseline` (L10087), `vacuidade-corpo-vazio` (L10120), `vacuidade-fora-de-pull-request` (L10125) | Não | MANTÉM | `check-pr-closing-keyword.sh` PRESENTE (SOBREVIVE); sem Node/Py |
| 183 | `closed-cycle/req-resolver-sem-caso-canonico-reprova` (L10230) | Sim (node+py) | REMOVE | `check-artifact-closed-cycle.sh` AUSENTE; baseline falha imediatamente; setup_npm_tree() chamado |
| 184 | `closed-cycle/note-link-do-gerador-nao-reconhecido-reprova` (L10252) | Sim (node) | REMOVE | mesmo gate AUSENTE; afirma comportamento de gerador Node (npm/src AUSENTE) |
| 185 | `closed-cycle/vocabulario-de-status-do-adr-em-portugues-reprova` (L10285) | Sim (py) | REMOVE | mesmo gate AUSENTE; afirma comportamento de gerador Python (pypi AUSENTE) |
| 186–188 | `validate-parity/script-integrity-unreadable-project-not-detected` (L10336), `unreadable-global-not-detected` (L10375), `fifo-hang-not-detected` (L10417) | Sim (node+py) | REMOVE | `check-validate-parity.sh` AUSENTE; baseline compartilhado falha |
| 189–191 | `validate-parity/gvp-global-script-integrity-message-text-diverges` (L10464), `gvmt-global-missing-type-message-text-diverges` (L10506), `gbg-claude-relativo-bare-relative-path-not-detected` (L10564) | Sim (node+py) | REMOVE | mesmo gate AUSENTE |
| 192 | `structural-marker-value/go/*` (L10626–L10669), `node/*` (L10692–L10721), `python/*` (L10746–L10777) | Sim (node+py) | PODA O BRAÇO | Remover braços node e python (setup_npm_tree falha; pypi AUSENTE); braços Go (4 assertions: 2 baselines + 2 detections) afirmam contrato de marcador estrutural em Go |
| 193 | `roadmap-ref-stale-state/go/*` (L10866–L10926), `node/*` (L10955–L10996), `python/*` (L11026–L11069) | Sim (node+py) | PODA O BRAÇO | Remover braços node e python (setup_npm_tree falha; pypi AUSENTE); braços Go (8 assertions) afirmam resolução de referência stale em Go |
| 194 | `serve-chain-canonical-link/node/*` (L11163–L11182), `python/*` (L11187–L11205) | Sim (node+py) | REMOVE | Sem braço Go; cenário afirma SOMENTE comportamento Node e Python de api_chain (npm/src e pypi AUSENTES); comentário do próprio cenário: "Go tem cobertura [de outro cenário]" |
| 195 | `python-writes-lf/wrong-newline-value` (L11238) | Sim (py) | REMOVE | `check-python-writes-lf.sh` AUSENTE |

**Contagem:** 67 cenários no total (incluindo grupos numerados como 3b, 39/40/41, 186-191). REMOVE: 53. PODA O BRAÇO: 7 (cenários 18, 24, 25, 26, 27-28, 29-38, 192, 193). MANTÉM: ~28.

**Verificação de integridade:** toda linha REMOVE cita um gate AUSENTE ou ausência de asserção Go. Cenário 57 (label /go mas gate AUSENTE): classificado REMOVE porque o gate `check-unknown-command-parity.sh` orquestra a asserção — sem ele, nenhum assert_fails_with executa. Cenários 162, 163 (labels /go): idem — gates `check-push-parity.sh` e `check-push-force-parity.sh` AUSENTES.

#### Auditoria do arquiteto — 2026-09-16

**Contagem medida:** `main` tinha 66 `scripts/check-*.sh`; a branch tem 39; 27 deletados
(26 do balde DELETAR + `check-validate-parity.sh`, cujo pin foi extraído antes).

**Reclassificação DELETAR → REESCREVER — três gates, auditada e APROVADA.** O balde DELETAR listava
29 gates; três **sobreviveram reescritos**, e a reclassificação está correta porque o critério do
balde ("só compara runtimes, sem pin próprio") não se aplicava a eles:

| Gate | Por que não era DELETAR | Diff |
|---|---|---|
| `check-rules-parity.sh` | O objeto não é comparar runtimes: é a identidade byte-a-byte do bloco de governança **entre os 4 arquivos de AI tool** (GEMINI.md, copilot-instructions, .windsurfrules, amazonq). Essa comparação sobrevive sem Node/Python. | −73/+54 |
| `check-update-parity.sh` | Restam 320 linhas de asserção comportamental do binário Go (exit codes, forma do JSON, dry-run, skip warnings, contrato de sandbox) — pin próprio, não comparação. | −730/+320 |
| `check-roadmap-barrier-contract.sh` | Contrato gerador↔`barrier` com snapshot de corpus pinado por SHA-256 + cenários `assert_fails_with`. O braço cross-runtime era uma das três partes; as outras duas são Go-only e independentes. | −26/+20 |

🔴 **Defeito aberto — a mensagem de sucesso do `check-gates-falsify.sh` não cobre o fim da suíte.**
O arquivo tem 6641 linhas; o único `echo "Falsification checks passed (all 183 scenarios…"` está na
linha 6272. Os cenários 182–185 (~370 linhas, incluindo `integration-assets/direction-b-shim-absent`)
**executam depois da mensagem de sucesso**. Como `assert_fails_with` faz `exit 1` na falha, uma
execução pode **imprimir "Falsification checks passed" e mesmo assim sair com código 1** — quem lê o
fim do log vê sucesso. Além disso, o número `183` e a enumeração em prosa são **digitados, não
medidos**, e estão obsoletos após as deleções desta wave.

**Consequência de auditoria:** a evidência "Falsification checks passed" reportada pelos agentes
desta wave é **parcial por construção** e não conta como prova até o defeito ser corrigido.
Correção atribuída a Ártemis (ML-3C-bis, mesma REQ, mesma causa).

#### ML-3C-bis — defeito de autorrelato em `check-gates-falsify.sh`
**Status:** ✅ Concluído (2026-09-16 — Ártemis)

**Defeito:** `echo "Falsification checks passed (all 183 scenarios, ..."` estava na linha 6272 de 6641;
~370 cenários executavam após ela. Falha nesses cenários: exit 1, mas a mensagem era impressa antes.
Contador "183" hardcoded e obsoleto.

**Fix entregue:**
- `$FALSIFY_SUCCESS_TALLY` (arquivo, sobrevive subshell boundary — padrão do `$FALSIFY_ENUM_TALLY` existente)
- `falsify_count_success()` instrumentada em 65 pontos: 7 helpers + 39 body non-indented + 19 body indented
- Mensagem hardcoded removida; mensagem medida adicionada na última linha do script
- Contador reporta `201 scenarios` na execução de 2026-09-16

**Falsificações provadas:**
- Direção A: injeção de mismatch em `integration-assets/direction-b-shim-absent` (após linha 6272) → exit 1, sem mensagem de sucesso. Restauração → exit 0, 201 scenarios.
- Direção B: `integration-assets/baseline` comentado → 200 scenarios. Restauração → 201 scenarios.

**Gates verdes:** `go build` RC=0 · `go test` RC=0 · `make quality` RC=0 (212 OK, 0 FAIL) · `trackfw validate` RC=0 (176 violações pré-existentes) · `check-orphan-gates.sh` RC=0.

**Regra Dura de Reconciliação:**
- `make quality` reporta 212 OK: confirma que a suíte completa passa com o fix aplicado.
- `trackfw validate` RC=0: confirma que nenhuma violação nova foi introduzida.
- Contador 201: mede exatamente os `echo "OK   [falsify/..."` instrumentados neste script; ~11 linhas adicionais vêm de sub-scripts externos não instrumentados (documentado no comentário do script e na nota de vault).

---

### ML-3D — **AC8 + AC11** — documentação e o break
**Status:** ✅ Concluído
`docs/cli-parity.md` vira documento de **canais**. O `CLAUDE.md` tem a regra dura de paridade
**reescrita, não apagada** — o Go continua sendo a expressão da verdade, agora por construção.
🔴 CHANGELOG declara o break: `require('trackfw')` deixa de resolver.

---

### ML-3C-ter — guarda de piso no contador do falsify
**Status:** ✅ Concluído — auditado pelo arquiteto (2026-09-16)

O ML-3C-bis trocou o literal por um contador medido, mas deixou o fecho sem piso: um tally zerado, ou
a remoção das chamadas de contagem por um refator futuro, faria o gate imprimir
`Falsification checks passed (0 scenarios)` e **sair com 0** — a mesma classe de defeito que o
ML-3C-bis existia para eliminar, uma camada abaixo.

🔴 **Erro de handoff do arquiteto, registrado:** pedi um piso fixo sem verificar que
`check-gates-falsify.sh` **não roda só em série**. O `gen-falsify-chunks.py` materializa preâmbulo +
fatia do corpo em ~7 chunks paralelos, e **é esse o caminho do CI**. Um piso de corpus inteiro é falso
por construção para qualquer chunk: medido, `chunk_6` conta ~10 cenários. A guarda pedida para
impedir falso-negativo produziu **falso-positivo**, que é pior — ensina a ignorar o gate.

**Desenho final (auditado no artefato):** o piso só vale na execução íntegra. O discriminante é
explícito, não inferido de contagem — `gen-falsify-chunks.py:502` injeta `__falsify_timing_mark` no
preâmbulo de **cada** chunk, e essa função **nunca** existe no script completo; a guarda roda sob
`if ! declare -f __falsify_timing_mark`.

**Por que não há lacuna no CI.** Sob sharding a guarda de piso não dispara, mas a garantia equivalente
já existe no caminho do CI e é **mais forte que contagem**: `run-gates-falsify-parallel.sh` faz guarda
de conjunto **por rótulo** (linha 190-208 — reprova se qualquer rótulo esperado estiver ausente), e
`check-falsify-shard-coverage.sh` roda no job `parity` (`quality.yml:924`). Contagem é um proxy;
conjunto de rótulos é o invariante. O piso é o complemento da execução serial local.

---

### ML-3E — tag de pré-release publicando como release estável
**Status:** ✅ Concluído — auditado pelo arquiteto (2026-09-16)

**Reportado pelo usuário e confirmado por medição:** o `install.sh` estava servindo `8.0.0-rc1` como
versão estável, havia 3 dias.

| Canal | O que servia como estável | Estado |
|---|---|---|
| GitHub (`scripts/install.sh`) | 🔴 `v8.0.0-rc1` | corrigido |
| npm | `7.6.0` (rc sob dist-tag `rc`) | já correto |
| PyPI | `7.6.0` (PEP440 trata pré-release) | já correto |

**Causa raiz:** o bloco `release:` do `.goreleaser.yaml` não declarava `prerelease`, e o default do
GoReleaser é `false` (documentação oficial de `customization/release`). Logo **toda** tag `-rc`
publicava como release estável e virava o `latest` do GitHub. Não foi descuido no rc1 — era o
comportamento configurado, e reincidiria no rc2.

**Correção em duas camadas:** (1) ação de mantenedor — o release `v8.0.0-rc1` foi marcado
`prerelease: true` / `make_latest: false` via API, e `/releases/latest` voltou a devolver `v7.6.0`
(verificado, não presumido; o `install.sh` resolve por esse endpoint, que honra a flag);
(2) origem — `prerelease: auto` no `.goreleaser.yaml` + `scripts/check-goreleaser-prerelease.sh` com
5 braços de auto-falsificação (chave ausente, valor `false`, config correta, arquivo ausente, bloco
ausente), ligado em `parity-rest`, que o CI já executa em `quality.yml:839` sob o required check
`parity`. Auto-teste rodado pelo arquiteto: 5 passed, 0 failed.

---

### Ação de mantenedor executada — R1 fechado (2026-09-16)

`make check-required-full` reprovava: o `required_status_checks` do branch protection (conjunto R,
que vive **fora do repositório**) ainda exigia `node`, `python (3.10)` e `python (3.12)` — checks que
nenhum workflow emite depois desta wave. Mergear o PR antes de corrigir R deixaria **todo PR
subsequente pendente para sempre, inclusive o que consertaria o problema** (vault:
`matriz-em-job-required-por-nome-fica-pendente-para-sempre-2026-09-08.md`).

**Ordem executada — R primeiro, PR depois.** R foi reduzido aos 7 checks de D nesta branch
(`go`, `package-smoke`, `windows-integrations-resolve`, `parity`, `governance-install-script`,
`governance-go-install`, `windows-full-suites`), e cada um foi verificado contra um job real nos
workflows da branch. D = R = W.

🔴 **Afrouxamento declarado e com prazo, atado ao merge deste PR.** Enquanto este PR não mergear, a
`main` ainda emite os jobs `node` e `python` **sem que sejam obrigatórios** — uma regressão nesses
dois runtimes não bloquearia merge nessa janela. O `D` da `main` (`.github/required-status-checks.txt`)
ainda lista os 10 e só converge para 7 quando este PR entrar. Se este PR estagnar, a janela fica
aberta em silêncio: registrado aqui para não ser redescoberto depois como drift de origem
desconhecida.

---

## Wave 4 — Fechar o backlog que a mudança apagou
> Dependências: Wave 3.

### ML-4A — **AC12** — a medição realocada
**Status:** 🔄 Classificação concluída e auditada (2026-09-16) — fechamento dos issues pendente de confirmação do usuário

**Corpus medido:** 18 issues abertos (não 16 — a estimativa do roadmap era anterior a #362 e #363).

🔴 **A estimativa preliminar de "8 de 16 desaparecem" era otimista. O medido é 6 de 18.**

🔴 **Erro do arquiteto que quase fechou defeito vivo, registrado porque o método errado é sedutor.**
A primeira passada cruzou mecanicamente os caminhos citados em cada issue contra a árvore e
classificou como "objeto removido" quando `npm/src/...` e `pypi/trackfw/...` estavam ausentes. O
regex só procurava esses dois prefixos — então **todo issue que também tinha sítio em `internal/`
foi rotulado DESAPARECE por omissão**. O #327 é a prova: cita `npm/src/generators/req.js:193` e
`pypi/trackfw/generators/req.py:303`, ambos ausentes, mas o sítio Go `internal/generators/req.go`
está vivo (a linha moveu de 299 para 343; o código é o mesmo). É INDIFERENTE.
A pergunta decisiva não é "o caminho citado sumiu?" e sim **"existe sítio Go sobrevivente que ainda
reproduza o mecanismo?"**. Reclassificado com essa pergunta.

(Uma segunda medição desta mesma sessão saiu errada por rodar em `zsh` com variável não citada —
`zsh` não faz word-splitting, e o teste comparou a lista inteira de artefatos como um caminho só,
devolvendo "ausente" para tudo. Refeita em `bash`. É a mesma classe de erro que custou o apagamento
de uma branch em 2026-09-12.)

#### Classificação medida

| Classificação | Issues | n |
|---|---|---|
| **DESAPARECE** — objeto removido, nada a corrigir | #261, #286, #298, #309, #310, #329 | 6 |
| **BARATEIA** — superfície encolheu, resta parte | #268, #307, #363 | 3 |
| **INDIFERENTE** — v8 não muda nada | #258, #273, #277, #290, #308, #327, #353 | 7 |
| **JÁ CORRIGIDO** (por trabalho desta wave, não pela remoção) | #362 | 1 |
| **PREMISSA FALSIFICADA** — não é defeito | #359 | 1 |

#### O que resta nos BARATEIA — e é isto que não pode se perder no fechamento

- **#268** — some a metade do `status` do Python; **resta a metade que escreve**:
  `internal/sync/sync.go:43` usa `filepath.Glob("docs/req/*.md")` literal e ignora o `req_dir`
  configurado. Num consumidor com `req_dir: docs/requisições`, o `sync` enxerga 0 REQ real e pode
  criar issue no PM para REQ alheia. Mais grave que a metade que desaparece.
- **#307** — somem 2 dos 3 gates; resta `check-release-tag-parity.sh` inteiro.
- **#363** — some o sítio de `check-doctor-parity.sh`; resta
  `check-validate-rule-pins.sh:367,376,385`, no gate mais central de `parity-rest`. 13 gates ainda
  usam a forma `python3 -c "` com aspas duplas e não foram varridos.

#### Ressalvas de fechamento (registradas antes de fechar, para não sumirem com o issue)

- **#329** — o que o autor declarou acionável não foi o nome velho na lista, e sim que **o sinal
  `[-1 resolvido]` não tem destino: não reprova nem fecha**. Essa observação é de desenho do ratchet,
  não é tocada pela v8, e morreria junto com o issue.
- **#359** — a flag `--no-pr` existe desde o PR #73 e está honrada (`internal/commands/ship.go:140`,
  verificado rodando o binário construído desta branch). O relato nasceu de um `--help` colado
  truncado — o cobra ordena alfabeticamente e `--no-pr` é a última linha. **Mas o dano relatado
  (quatro PRs abertos por agentes) é real** e é de default e orquestração, não de flag ausente.
  Fechar como "não reproduz" sem recapturar isso perde o problema verdadeiro.

---

### ML-4B — dependência morta de `node` e `python3` num gate sobrevivente
**Status:** ⬜ Pendente — descoberto na triagem do ML-4A, mesma causa, mesma REQ

`scripts/check-release-tag-parity.sh` já loopa **só** `for runtime in go` (linhas 637, 669, 707,
comentadas `ML-3A (v8): node py removed`), mas o setup **continua exigindo os dois interpretadores**:
`exit 1` com `node not found in PATH` (linhas 89-93) e `python3 not found in PATH`, e `ln -s` de
ambos para o `RUNTIME_BIN` (118-119). Verificado no fonte pelo arquiteto.

**Por que é bloqueante para a v8 e não cosmético:** a v8 declara que o produto é um binário Go. Um
contribuidor sem `node` instalado **não consegue rodar `make quality`** — o gate aborta no setup por
uma dependência que o produto não usa mais. É a Wave 3 incompleta: o braço morreu, o andaime ficou.

Também é o mecanismo remanescente do **#307**: a guarda de vacuidade conclui `git does not resolve`
quando quem não inicia é o `python3` copiado.
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
