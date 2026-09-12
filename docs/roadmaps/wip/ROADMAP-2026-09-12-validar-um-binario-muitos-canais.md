---
status: wip
date: 2026-09-12
req: "docs/req/REQ-2026-09-12-validar-a-opcao-d-um-binario-muitos-canais-antes-de-decidir-se-entra-na-v8.md"
squad: ""
---

# Roadmap: Validar um binário, muitos canais — antes de decidir a v8

> Created: 2026-09-12 | Status: wip

## Context
REQ: docs/req/REQ-2026-09-12-validar-a-opcao-d-um-binario-muitos-canais-antes-de-decidir-se-entra-na-v8.md
ADR: docs/adr/ADR-2026-09-12-estrategia-de-distribuicao-o-custo-da-paridade-tripla-e-quem-ela-atende.md (Accepted)

Duas trilhas **independentes**, executáveis em paralelo: a **prova técnica** (o pacote funciona sob a
restrição real) e a **medição de retorno** (quanto do backlog D apaga). A decisão da v8 precisa das
duas — a primeira diz *se dá*, a segunda diz *se vale*.

## Acceptance Criteria
- [ ] AC1–AC5 — instala sob a restrição real: registry alternativo, `--ignore-scripts`, sem GitHub, FS read-only, sem toolchain
- [ ] AC6 — lockfile gerado no macOS instala no Windows
- [ ] AC7 — contra-braço: sem pacote de plataforma, aborta nomeando a plataforma
- [ ] AC8–AC9 — saída e exit code byte-idênticos ao nativo, nos 3 SOs
- [ ] AC10–AC12 — quanto do backlog desaparece, barateia, e o que passa a existir

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

---

## Trilha 1 — Prova técnica (protótipo descartável)

### ML-1A — **AC1–AC4** — pacote npm com `optionalDependencies`, sob restrição
**Status:** ⬜ Pendente
**Arquivos afetados:** somente `prototype/` (novo, descartável). 🔴 **Nada de `npm/src/`.**
**Ações:**
1. Cross-compilar o binário Go para as plataformas do CI.
2. Montar pacote-casquinha + pacotes de plataforma com `os`/`cpu`, no molde do esbuild.
3. Subir **verdaccio local** e publicar lá. 🔴 **Nunca no npmjs.org** — nome e versão são
   permanentes após 72 h.
4. Provar as quatro condições, **cada uma como cenário nomeado**: registry alternativo ·
   `--ignore-scripts` · **sem rota para github.com** · `$HOME`/cache somente-leitura.
**Critérios de aceite:**
- [ ] Os 4 cenários passam, cada um com a saída registrada
- [ ] 🔴 O cenário "sem github.com" **prova o bloqueio**: mostrar que a rota está cortada, senão o
      cenário é vácuo e passa por não ter sido exercitado
**Reconciliação:** cada cenário declara qual conclusão do relatório ele afirma.

### ML-1B — **AC5** — wheel de plataforma no PyPI local
**Status:** ⬜ Pendente
**Arquivos afetados:** somente `prototype/`.
**Contexto:** `gh-bin` já faz isto com o `cli/cli` (Go) — 4 wheels `py3-none-<plataforma>`. Existe
`go-to-wheel` (fev/2026) que cross-compila Go e emite as tags certas; avaliar antes de escrever do
zero.
🔴 **Tags:** `manylinux_2_17_*` / `musllinux_1_2_*` / `macosx_*` / `win_amd64`. **`linux_x86_64`
puro é rejeitado pelo PyPI.**
**Critérios de aceite:**
- [ ] `pip install` do wheel local, **sem rede para o GitHub** e **sem toolchain Go**
- [ ] Nenhum publish em pypi.org — 🔴 o PyPI **não permite reusar nome de arquivo**

### ML-1C — **AC6 + AC7** — os dois modos de falha conhecidos
**Status:** ⬜ Pendente
**Ações:**
1. **Braço do lockfile:** gerar `package-lock.json` no macOS, instalar no **Windows do CI**, exigir
   que o binário esteja lá. É o defeito conhecido de `optionalDependencies` + lockfile.
2. **Contra-braço:** remover todos os pacotes de plataforma ⇒ a casquinha **aborta nomeando a
   plataforma**, nunca `MODULE_NOT_FOUND`.
**Critérios de aceite:**
- [ ] Lock do macOS instala no Windows
- [ ] Ausência de plataforma ⇒ erro que nomeia a plataforma

### ML-1D — **AC8 + AC9** — equivalência com o nativo nos 3 SOs
**Status:** ⬜ Pendente
**Ações:** comparar `version`, `validate --json`, `status`, `context --json` entre casquinha e
binário nativo, em Linux, macOS e Windows do CI.
**Critérios de aceite:**
- [ ] Saída **byte-idêntica** (não "equivalente")
- [ ] Exit codes idênticos, **incluindo os de violação**
- [ ] Divergência encontrada é **reportada**, não normalizada no comparador

---

## Trilha 2 — Medição de retorno (independente da Trilha 1)

### ML-2A — **AC10 + AC11 + AC12** — quanto D apaga, barateia e acrescenta
**Status:** ⬜ Pendente
**Arquivos afetados:** somente `docs/qualidade/2026-09-12-quanto-a-opcao-d-apaga-do-backlog.md` (novo).
**Ações:**
1. Classificar as **33 REQs abertas** em: **(a) desaparecem** — o defeito é "existe num runtime e
   falta noutro"; **(b) barateiam ~3×** — defeito real, hoje corrigido três vezes;
   **(c) indiferentes**.
   🔴 Medição preliminar do arquiteto: **18 das 33 têm componente de paridade.** Mas *"tem
   componente"* **não é** *"desaparece"* — separar é o trabalho deste ML, e o número preliminar
   **não serve como resultado**.
2. Mesmo exercício para os **31 gates de paridade**: quantos deixam de ter objeto.
3. O que **aparece**: `trackfw serve` é exclusivo do Go hoje e passaria a existir em npm e pip.
**Critérios de aceite:**
- [ ] Três listas nomeadas, REQ a REQ, com o critério de cada classificação
- [ ] Contagem de gates que perdem objeto, nomeados
- [ ] O ganho do `serve` quantificado
- [ ] 🔴 Nenhuma REQ classificada sem justificativa de uma linha

---

## Escopo negativo

- 🔴 **Nenhum publish em npmjs.org ou pypi.org.** Nome/versão no npm são permanentes após 72 h; o
  PyPI não reusa nome de arquivo. É o único erro irreversível desta REQ.
- **Não** migrar produção, **não** remover `npm/src/` nem `pypi/trackfw/`. A regra dura de paridade
  continua valendo até a decisão da v8.
- **Não** tocar nas frentes em execução (`fix/req-nasce-orfa`, `fix/triagem-medida-das-reqs-de-paridade`).
- Todo código novo vive em `prototype/`, e é **removido antes do PR de produção**.
