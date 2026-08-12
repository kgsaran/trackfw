---
status: wip
date: 2026-08-12
req: "docs/req/REQ-2026-08-12-mitigacao-do-fail-open-do-credential-guard-integridade-do-script-e-da-config-controle-positivo-e-fail-closed-nativo.md"
squad: "Apolo, Ártemis, Hades, Hefesto"
---

# Roadmap: Mitigacao do fail-open do credential-guard — wave 1 controle positivo e failClosed

> Created: 2026-08-12 | Status: wip

## Context

REQ: `docs/req/REQ-2026-08-12-mitigacao-do-fail-open-do-credential-guard-integridade-do-script-e-da-config-controle-positivo-e-fail-closed-nativo.md`

O `ROADMAP-2026-08-12-semantica-de-falha-de-hook` **mediu**: quando o `command` de um hook não
resolve, a ferramenta **prossegue**. O `trackfw-credential-guard.sh` é um controle de **negação** —
se não roda, nada bloqueia, e o agente não trata isso como erro fatal. Falha aberto em **4 dos 6**
CLIs (Claude, Codex, Cursor por padrão, Gemini provável; Copilot é a exceção fail-closed).

**Este roadmap implementa apenas os dois primeiros itens da REQ** — os de custo baixo e risco
conhecido. Os itens 3 (wrapper) e 4 (integridade de conteúdo) ficam para uma **barreira de ADR** no
fim, porque envolvem decisão de arquitetura ainda não tomada.

### Por que só os itens 1 e 2 agora

- **Item 3 (wrapper `test -x … || exit 2`)** tem um bloqueador confirmado no código: o script é
  **gerado** por `trackfw init`/`update harness` (`internal/generators/scaffold.go:779-837`), **não
  faz parte do binário**. Um clone fresco com os hooks já commitados, **antes** do `init`, teria
  **toda chamada de ferramenta travada**. Precisa de resposta de arquitetura, não de implementação.
- **Item 4 (integridade de conteúdo)** precisa de um valor de referência guardado **fora** do arquivo
  gerado — e precisa cobrir também `credential_guard.mode`, que é lido em **runtime** de
  `trackfw.yaml` (`scaffold.go:1005`). Onde guardar essa referência é decisão de arquitetura.
- **Existe uma terceira via que ninguém explorou:** o credential-guard de **escopo global** vive em
  `~/.trackfw/`, **fora do repositório** em que o agente trabalha. Um agente restrito ao workspace
  **não alcança** esse arquivo. Talvez a resposta certa seja **preferir o escopo global** em vez de
  construir integridade no escopo de projeto. Isso precisa ser avaliado **antes** de implementar 3 ou
  4 — construir a solução errada é mais caro que adiar.

### O que os itens 1 e 2 cobrem — e o que não cobrem

| Vetor | Item 1 (controle positivo) | Item 2 (`failClosed` Cursor) |
|---|---|---|
| Caminho que não resolve (o incidente original) | ✅ detecta | ✅ bloqueia (só Cursor) |
| Script **apagado** | ⚠️ detecta **quando `validate` roda**, não no momento da invocação | ✅ bloqueia (só Cursor) |
| Script **sobrescrito** com `exit 0` | ❌ não cobre | ❌ não cobre |
| `credential_guard.mode` rebaixado via YAML | ❌ não cobre | ❌ não cobre |

Isso é intencional e precisa estar escrito: os itens 1 e 2 **não fecham** a classe de ameaça. Eles
cobrem a classe do **incidente real já observado** com custo baixo e zero risco de travar projeto.

### ⚠️ Riscos de regressão já mapeados

1. **A regra nova não pode disparar neste repositório.** O `.claude/settings.json` do trackfw **não**
   referencia o credential-guard (o guard **global** está instalado, e o dedup
   `globalCredentialGuardInstalled*()` pula as entradas de projeto de propósito). Se a regra disparar
   aqui, `make quality` quebra e o Cenário 29 do falsify (mensagem de sucesso do `validate` fixada e
   byte-idêntica nos 3 CLIs) falha junto. **A regra só deve avaliar entradas que existem.**
2. **A mensagem de sucesso do `validate` é fixada byte-a-byte** nos 3 CLIs (Cenário 29). Não alterar.
3. **`check-agent-hooks-parity.sh` faz diff estrutural** do JSON gerado entre Go×Node×Python — o item
   2 altera o `.cursor/hooks.json`, então os 3 stacks mudam no mesmo commit ou o gate falha.
4. **Constantes compartilhadas do Node** já foram divididas por CLI nos roadmaps anteriores
   (`SIGNAL_CMD_*`, `CLEANUP_CMD_*`, `GUARD_CMD_*`). O item 2 mexe só no wiring do Cursor.

## Acceptance Criteria

- [ ] Regra nova de validação detecta hook de credential-guard registrado cujo script **não existe**
      ou **não é executável**, nos 3 CLIs do trackfw (Go, Node.js, Python).
- [ ] A regra **não dispara** quando não há entrada de credential-guard registrada (estado legítimo
      com guard global instalado).
- [ ] A regra é configurável por `rules:` no `trackfw.yaml`, como as demais.
- [ ] Cenário de falsificação provando que a regra **não é vácua**.
- [ ] `failClosed: true` emitido **apenas** nas entradas de credential-guard do Cursor — nunca nas de
      attention — nos 3 stacks.
- [ ] `make quality` exit 0; `trackfw validate` sem violações **neste repositório**.
- [ ] ADR decidindo os itens 3 e 4, ou registrando explicitamente que ficam adiados e por quê.

### Escopo negativo

- **Não** implementa o wrapper (item 3) nem a verificação de integridade de conteúdo (item 4) — são
  saída da barreira de ADR.
- **Não** altera `scripts/trackfw-credential-guard.sh`.
- **Não** altera o wiring de **caminho** dos hooks (encerrado no `ROADMAP-2026-08-11`).
- **Não** altera a mensagem de sucesso do `validate`.
- **Não** mexe nos hooks de **escopo global** (`trackfw update harness`).

---

## Wave 1 — Controle positivo no `validate` (1 ML)
> Dependências: nenhuma.

### ML-1A — Regra `credential_guard_hook_resolvable`
**Status:** 🔄 Em andamento
**Agente:** Apolo (`apolo-tf`)

**Arquivos afetados:** `internal/validator/` (Go), equivalente Node em `npm/src/`, equivalente Python
em `pypi/trackfw/` — **os 3 stacks**, mais os testes de cada um.

**Ações:**
1. Nova regra que, para cada arquivo de hook de projeto que **existir**
   (`.claude/settings.json`, `.codex/hooks.json`, `.gemini/settings.json`, `.cursor/hooks.json`,
   `.github/hooks/trackfw-attention.json`, `.kiro/hooks/trackfw-attention.json`), extrai os comandos
   que referenciam `trackfw-credential-guard.sh`, **resolve o caminho** e verifica que o script
   **existe** e é **executável**.
2. **Resolução dos prefixos** — as 3 formas que o trackfw emite hoje (ver `docs/cli-parity.md`,
   "Mecanismo de resolução de caminho dos hooks de projeto, por CLI"):
   - `$CLAUDE_PROJECT_DIR/…` e `$GEMINI_PROJECT_DIR/…` → substituir pela raiz do projeto;
   - `"$(git rev-parse --show-toplevel)/…"` → substituir pela raiz do projeto, **removendo as aspas
     literais**;
   - caminho relativo puro (Cursor/Copilot/Kiro) → resolver contra a raiz do projeto.
   Comando que **não casar** nenhuma dessas formas: **não** violar — emitir nada e seguir. Não é
   função desta regra adivinhar formatos que o trackfw não gera.
3. Usar `applyRule`/`applyRuleTagged` (Go, `internal/validator/validator.go:120` e `:136`) para a
   regra ser configurável por `rules:` no `trackfw.yaml`, com default **`error`**. Equivalentes nos
   outros dois stacks.
4. Mensagem de violação **acionável**: qual arquivo de hook, qual CLI, qual caminho resolvido, e a
   ação (`trackfw update` regenera o script).

**Critérios de aceite:**
- [ ] A regra dispara quando existe entrada de guard e o script está ausente/não executável.
- [ ] A regra **não** dispara quando não há entrada de guard (estado legítimo — guard global).
- [ ] A regra **não** dispara neste repositório: `trackfw validate` continua sem violações.
- [ ] Configurável por `rules:` (`off`/`warning`/`error`), default `error`.
- [ ] Paridade nos 3 CLIs — mensagem e comportamento idênticos.
- [ ] A **mensagem de sucesso** do `validate` não mudou (Cenário 29 do falsify continua passando).
- [ ] `go test ./...`, `npm test`, `pytest` verdes; `make quality` exit 0.

**Comandos de validação:**
```bash
go build ./... && go test ./...
npm --prefix npm test
python3 -m pytest pypi/tests -q
make quality
```

---

## Wave 2 — Prova negativa da regra (1 ML)
> Dependências: Wave 1.

### ML-2A — Cenário de falsificação da regra nova
**Status:** ⬜ Pendente
**Agente:** Ártemis (`artemis-tf`)

**Arquivos afetados:** `scripts/check-gates-falsify.sh` (cenário novo + string de resumo final).

**Ações:** cenário baseline + detecção provando que a regra `credential_guard_hook_resolvable` acusa
um hook de guard cujo script não existe, e que **removê-la** faz o cenário falhar.

**Critérios de aceite:**
- [ ] Baseline (árvore íntegra passa) + detecção (árvore sabotada reprova).
- [ ] 🔴 **Prova de não-vacuidade:** desabilitar a regra no validador faz o cenário **falhar**.
      Restaurar. Reportar a saída. *(Este projeto já produziu um cenário de prova negativa que ele
      próprio não provava — ver ML-1A/ML-1B do `ROADMAP-2026-08-12-prova-negativa-...`.)*
- [ ] `$HOME` isolado, se o cenário depender de estado global.
- [ ] Comentário do cenário documenta a **âncora de manutenção** e traz instrução `RETARGET`.
- [ ] `make quality` exit 0 com total de cenários incrementado.

---

## Wave 3 — `failClosed: true` no Cursor (1 ML)
> Dependências: Wave 2.

### ML-3A — Opt-in nativo do Cursor nas entradas do guard
**Status:** ⬜ Pendente
**Agente:** Apolo (`apolo-tf`)

**Arquivos afetados:** `internal/generators/agentfiles.go`, `npm/src/generators/hooks.js`,
`pypi/trackfw/generators/hooks.py` + testes dos 3 stacks.

**Ações:** emitir `failClosed: true` **somente** nas entradas de credential-guard do
`.cursor/hooks.json` (`beforeShellExecution`, `afterShellExecution`, e os matchers `Read`/`Write` de
`preToolUse`/`postToolUse`).

⚠️ **Nunca** nas entradas de attention-signal/cleanup: travar o agente porque um hook de sinalização
de UI falhou seria pior que o problema. O escopo é **só o controle de segurança**.

⚠️ Confirme na doc primária do Cursor a **forma exata** do campo (nome, tipo, onde no objeto) antes
de emitir — não inferir a partir de outro CLI.

**Critérios de aceite:**
- [ ] `failClosed: true` presente **apenas** nas entradas de guard do Cursor, nos 3 stacks.
- [ ] Entradas de attention do Cursor **byte-idênticas** antes e depois.
- [ ] Emissões dos outros 5 CLIs **byte-idênticas** antes e depois.
- [ ] Forma do campo confirmada em doc primária (cite URL no comentário do código).
- [ ] `check-agent-hooks-parity.sh` sem `FAIL`; `make quality` exit 0.

---

## Barreira B1 — ADR dos itens 3 e 4 (Zeus)
> Dependências: Wave 3.

Zeus escreve ADR decidindo — ou adiando com justificativa — os dois itens que sobraram, à luz de
três opções que **não** são mutuamente exclusivas:

1. **Wrapper** (`test -x … || exit 2`) — resolve deleção no momento da invocação, mas precisa de
   resposta para "projeto legitimamente sem o script" (risco de *bricking* confirmado).
2. **Integridade de conteúdo** do script **e** de `credential_guard.mode`, com referência fora do
   arquivo gerado — único caminho que cobre **sobrescrita** e **downgrade por config**.
3. **Preferir o escopo global** (`~/.trackfw/`, fora do repositório, inalcançável por agente restrito
   ao workspace) em vez de endurecer o escopo de projeto.

A opção 3 pode tornar 1 e 2 desnecessários — **avaliá-la primeiro**.

---

## Wave 4 — Barreira de segurança e documentação (2 MLs, **paralelos**)
> Dependências: Barreira B1. Arquivos disjuntos.

### ML-4A — Revisão de segurança
**Status:** ⬜ Pendente · **Agente:** Hades (`hades-tf`) · **Entregável:** `docs/seguranca/` (novo).
Avaliar se os itens 1 e 2 entregam a redução de risco pretendida, e se a regra nova cria falso senso
de segurança (ela verifica no momento do `validate`, não no da invocação). **Não modifica código.**

### ML-4B — Documentação
**Status:** ⬜ Pendente · **Agente:** Hefesto (`hefesto-tf`) · **Arquivos:** `docs/cli-parity.md`.
Registrar a regra nova, o `failClosed` do Cursor, e **explicitamente o que continua descoberto**
(sobrescrita e downgrade por config). **Não modifica código de produto.**

---

## Notas de execução

- **Autoridade de Git:** apenas Zeus cria branch, commita e faz push.
- **Sequencialidade:** Waves 1→2→3 são sequenciais. A Wave 4 tem 2 MLs paralelos (arquivos
  disjuntos). Motivo da sequencialidade das Waves 1–3: cada uma depende do resultado da anterior, e
  a Wave 3 altera arquivos gerados que o gate de paridade compara entre os 3 stacks.
- **Regra de paridade dos 3 CLIs é inviolável** nas Waves 1 e 3 (alteram código de produto).
