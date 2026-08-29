---
status: abandoned
date: 2026-08-15
req: "docs/req/REQ-2026-08-15-gate-de-seguranca-para-trackfw-plugins-install-download-de-binario-de-terceiro-sem-parecer-previo.md"
squad: "hades-tf, apolo-tf, hefesto-tf"
---

# Roadmap: gate de segurança para `trackfw plugins add` (binário de terceiro)

> ⛔ **ABANDONADO em 2026-08-15, com a Wave 0 concluída.** KG propôs remover o subsistema de plugins
> em vez de cercá-lo, e a proposta ganha. Substituído por
> `docs/roadmaps/wip/ROADMAP-2026-08-15-remocao-do-subsistema-de-plugins-do-trackfw.md`.
> A Wave 0 **não foi desperdiçada**: foi o parecer do `hades-tf`
> (`docs/seguranca/2026-08-15-gate-de-plugins-binario.md`) que demonstrou o quanto o gate entregaria
> a menos, e é essa análise que sustenta a remoção.

> Created: 2026-08-15 | Status: wip

## Context

REQ: `docs/req/REQ-2026-08-15-gate-de-seguranca-para-trackfw-plugins-install-download-de-binario-de-terceiro-sem-parecer-previo.md`
ADR de origem (débito D8e): `docs/adr/ADR-2026-08-15-gate-de-duas-fases-para-artefatos-de-terceiro-...md`
Parecer que classificou a severidade: `docs/seguranca/2026-08-15-skills-de-terceiro-via-url.md` (Q8e)

Estender ao caminho que baixa **binário** a propriedade já provada para markdown: *nenhum caminho
de código instala artefato de terceiro sem parecer prévio*. **Markdown influencia; binário
executa** — por isso o parecer classificou este vetor como de severidade **maior**.

### Mapa apurado no código (2026-08-15) — base factual

| | Go | Node | Python |
|---|---|---|---|
| Comando | `internal/commands/plugins.go:46` `plugins add` | `npm/src/commands/plugins.js:136` `add` | **não existe** |
| Download | `internal/plugins/plugins.go:175` `Install(repo)` | `npm/src/commands/plugins.js:82` `installPlugin` | **não existe** |
| Destino | `~/.trackfw/plugins` (`Dir()`, :146) | idem (`pluginsDir()`, :61) | — |
| Testes | `internal/plugins/plugins_test.go` (parcial) | **nenhum** | `list`/`run` apenas |

**Fatos que condicionam o desenho:**

1. 🔴 **Python não tem caminho de instalação.** `pypi/trackfw/commands/plugins.py` só tem `list` e
   `run` sobre executáveis `trackfw-*` já no `PATH`. **Não há o que gatear.**
2. 🔴 **O binário nasce executável, antes de qualquer aprovação.** Go: `os.Chmod(tmpPath, 0755)`
   **antes** do rename (`plugins.go:246`). Node: `mode: 0o755` no próprio `writeFileSync`
   (`plugins.js:108`).
3. 🔴 **Node tem fraquezas pré-existentes** que AC3 obriga a tocar: `fetchRegistry()` usa
   `https.get` cru — **sem checagem de status, sem timeout, sem teto de tamanho**; `installPlugin`
   faz `res.arrayBuffer()` e só **depois** checa o tamanho (bufferiza o corpo inteiro);
   **zero testes** de plugins; e **não existe `ResolveRepo`** — nome puro se comporta diferente do Go.
4. 🟡 **Escopo é sempre global** (`~/.trackfw/plugins`), sem `--scope`. KG decidiu **manter**.
   Consequência: fica fora do perímetro da detecção, como em D4-bis.
5. ✅ **Referência a reusar** (mergeada e provada): `internal/thirdparty/` — `Fetch`, `Checksum`,
   `RedactURL`, `NewQuarantineEntry`/`WriteQuarantine`/`ReadQuarantine`,
   `LoadProvenance`/`UpsertProvenanceEntry`/`VerifyApproval` (todas com `root` como 1º parâmetro),
   e a regra `thirdparty_artifact_has_provenance` nos 3 CLIs.

### Decisões já tomadas (não reabrir)

- **KG:** apenas o **handshake**; verificação de origem (checksum publicado, assinatura, pinagem)
  fica **fora de escopo**, com o limite declarado no ADR — é gate de **revisão**, não de
  **supply-chain**.
- **KG:** `chmod 0755` **só após aprovação**; em quarentena o binário fica `0600`. Escopo atual
  mantido.
- **Arquiteto:** **Python é exceção intencional documentada** para o *gate* (não há caminho de
  instalação; criá-lo só para gatear inverteria a REQ). A **regra de detecção**, essa sim, é
  portada nos **3 CLIs**.
- **Arquiteto:** a quarentena persiste o **repo e a URL de asset RESOLVIDOS**, nunca a string que o
  usuário digitou — `ResolveRepo` traduz nome puro via registry não-pinado, e sem isso o revisor
  não vê de onde os bytes vieram. É também o que mantém a confiança no registry fora de escopo de
  forma honesta: o gate revisa o **artefato resolvido**.

## Acceptance Criteria
- [ ] AC1 — Nenhum caminho de código instala plugin sem parecer prévio: `plugins add` para em
      quarentena; a instalação só consuma com aprovação vinculada por checksum.
- [ ] AC2 — Reuso do handshake de `internal/thirdparty/` (quarentena, proveniência, `VerifyApproval`),
      não reimplementação. Divergências justificadas no ADR.
- [ ] AC3 — Política de rede explícita e **idêntica entre Go e Node**: HTTPS-only, timeout, teto de
      tamanho aplicado por **stream** (não pós-buffer), limite de redirect com revalidação de
      esquema por hop, checagem de status.
- [ ] AC4 — Binário em quarentena **sem bit de execução** (`0600`); `chmod 0755` **somente** após
      aprovação. Teste que verifica o modo do arquivo em quarentena.
- [ ] AC5 — Limite de origem/integridade **declarado no ADR**: o checksum prova que o binário
      instalado é o revisado, **não** que o autor publicou aquilo.
- [ ] AC6 — **Corrigido pela Wave 0 (D2 do ADR):** detecção de **adulteração pós-aprovação**
      (ramo ii) via índice versionado no projeto chaveado por nome de plugin, portada nos **3 CLIs**.
      Detecção de **instalação sem aprovação** (ramo i) é **declarada ausente** — impossível com
      escopo global sem gerar falso-positivo entre projetos.
- [ ] AC7 — Parecer do `hades-tf` **antes** do primeiro ML de implementação.
- [ ] AC8 — Gate em **Go e Node**; **Python declarado exceção intencional** em `docs/cli-parity.md`.
      `make quality` verde.
- [ ] AC9 — Quarentena registra **repo e URL de asset resolvidos** (não a string digitada), com a
      query redigida (`RedactURL`).

---

## Wave 0 — Parecer de segurança (BARREIRA BLOQUEANTE)
> ⛔ Nenhuma Wave posterior inicia antes de ML-0A e ML-0B ✅.

### ML-0A — Parecer do `hades-tf`
**Status:** ✅ Concluído (2026-08-15) — P1–P7 respondidas; flag de AC6 inimplementável acatada · **Agente:** `hades-tf` (`subagent_type: hades-tf`)
**Escreve apenas:** `docs/seguranca/2026-08-15-gate-de-plugins-binario.md` (novo)

**Perguntas com veredito obrigatório:**
1. **P1 — Onde mora a proveniência do plugin?** ⭐ **Pergunta central.** O binário fica em
   `~/.trackfw/plugins` (global, decisão de KG). Proveniência fora do repo não aparece em
   `git status`/diff/PR, e o `ADR-2026-08-12` afirma que **detecção é a defesa**. Avaliar,
   no mínimo: (a) índice versionado **no projeto** listando plugins esperados + checksums, com o
   binário seguindo no home; (b) proveniência no home, aceitando que não há detecção;
   (c) outra saída. Veredito explícito sobre **se existe camada de detecção para plugins ou se ela
   é declarada ausente**.
2. **P2 — Modelo de ameaça do binário**, e o que muda em relação ao markdown: execução direta,
   persistência, `RunPlugin` (`internal/commands/plugins.go:111`) executando sem revalidar nada.
3. **P3 — `chmod` tardio é suficiente?** O binário em quarentena é `0600` mas continua sendo bytes
   executáveis a um `chmod` de distância. Dizer o que isso compra de fato.
4. **P4 — O que o revisor precisa ver** para dar parecer sobre um **binário** (não pode ler o
   conteúdo como faz com markdown): repo resolvido, URL de asset, tamanho, checksum, plataforma.
5. **P5 — Registry não-pinado** (`RegistryURL` → branch `main` de repo externo, parser YAML
   caseiro). Está fora do escopo da REQ; confirmar se essa exclusão se sustenta **dado** que a
   quarentena passa a registrar o alvo resolvido.
6. **P6 — Node pré-existente:** `fetchRegistry` sem status/timeout/teto e `arrayBuffer()` antes da
   checagem de tamanho. Classificar severidade e dizer se entram nesta REQ ou em outra.
7. **P7 — Exceção do Python** é defensável, ou cria um caminho de fuga?

**Critérios de aceite:**
- [ ] Arquivo existe e responde P1–P7 com veredito explícito (nunca "depende").
- [ ] P1 declara **se há ou não** camada de detecção para plugins.
- [ ] Nenhum arquivo fora de `docs/seguranca/` e `docs/agents-working-context.md` tocado.

### ML-0B — ADR + reescrita das Waves 1+
**Status:** 🔄 Em andamento — ADR escrito (D1–D9); Waves 1+ pendentes de reescrita · **Agente:** Zeus (não delegável)
**Ações:** ADR novo com decisões numeradas, uma por P1–P7; preencher `adr:` da REQ; substituir os
`<<TBD-Pn>>` das Waves 1+; declarar a exceção do Python e o limite de origem (AC5).

---

## Wave 1 — Go (contingente à Wave 0)
> Dependências: Wave 0 ✅. MLs sequenciais (mesmo pacote).

### ML-1A — Quarentena de binário + política de rede (Go)
**Status:** ⬜ Pendente · **Agente:** `apolo-tf`
**Arquivos:** `internal/plugins/plugins.go`, `internal/plugins/quarantine.go` (novo), `+ testes`
**Ações:** cliente HTTP **dedicado** (não reusar o compartilhado com `Search`), HTTPS-only,
teto por stream, redirect com revalidação de esquema; `plugins add` passa a **parar** em
quarentena `<<TBD-P1 caminho>>`, gravando **repo e URL de asset resolvidos** (AC9) com `RedactURL`,
tamanho, checksum e plataforma; arquivo em quarentena com modo **`0600`**.
**Aceite:** `plugins add` não cria nada em `~/.trackfw/plugins`; quarentena é `0600`; teste de
HTTP não-200 e de tamanho excedido **por stream**.

### ML-1B — Fase 2 + `chmod` tardio + proveniência (Go)
**Status:** ⬜ Pendente · **Agente:** `apolo-tf` · **Dep.:** ML-1A ✅
**Ações:** subcomando de consumo exigindo `VerifyApproval`; `chmod 0755` **só aqui**; proveniência
em `<<TBD-P1>>`; guardrail de sessão como em D2.
**Aceite:** instalar sem aprovação **falha**; checksum divergente **falha** (TOCTOU); modo `0755`
só existe após aprovação.

---

## Wave 2 — Node (contingente)
### ML-2A — Porte + correção das fraquezas pré-existentes
**Status:** ⬜ Pendente · **Agente:** `apolo-tf`
**Arquivos:** `npm/src/commands/plugins.js`, `npm/src/plugins/` (novo), `npm/tests/plugins.test.js` (novo)
**Ações:** porte 1:1 do Go **+** correções que AC3 obriga, **cada uma sinalizada como pré-existente
no relatório**: status/timeout/teto em `fetchRegistry`; teto por stream em vez de `arrayBuffer()`;
`resolveRepo` equivalente ao Go. **Primeiros testes de plugins do Node.**

---

## Wave 3 — Detecção, paridade e docs
### ML-3A — Regra de detecção (3 CLIs) + paridade + docs
**Status:** ⬜ Pendente · **Agente:** `apolo-tf`
**Ações:** regra `<<TBD-P1 nome>>` nos **3 CLIs** (o validator Python existe e espelha o Go);
`scripts/check-plugins-parity.sh` (novo, no alvo `parity` — hoje **não existe** paridade
comportamental de plugins, só a presença do nome no `--help` em `check-cli-parity.sh:22`);
`docs/cli-parity.md` documenta a **exceção do Python** e o limite de AC5.

---

## Wave 4 — Barreira final
### ML-4A — `hades-tf` (verificação pós-implementação) · ML-4B — `hefesto-tf` (qualidade)
**Status:** ⬜ Pendente — paralelos, escrevem em `docs/seguranca/` e `docs/qualidade/`.

---

## Notas
- **Release depois, em PR próprio.** Versão vive em 4 arquivos (`internal/version/version.go:3`,
  `npm/package.json:3`, `pypi/pyproject.toml:7`, `pypi/trackfw/__init__.py`) e
  `check-cli-parity.sh` exige saída byte-idêntica — bump atômico. `CHANGELOG.md` + bump no **mesmo**
  PR; tag após o merge. **Não misturar com o PR desta feature.**
- Commits e branch são exclusivos do `trackfw_architect`.
