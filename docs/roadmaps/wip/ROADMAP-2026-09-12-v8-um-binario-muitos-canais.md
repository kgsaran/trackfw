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
**Status:** ⬜ Pendente
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
