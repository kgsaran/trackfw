---
status: done
date: 2026-08-11
req: "docs/req/REQ-2026-08-11-resolucao-de-caminho-dos-hooks-de-agente-independente-do-cwd-attention-signal-cleanup-e-os-5-clis-nao-claude.md"
squad: "Prometeu, Apolo, Ártemis, Hefesto, Hades"
---

# Roadmap: Resolucao de caminho dos hooks de agente independente do cwd

> Created: 2026-08-11 | Status: done

## Context

REQ: docs/req/REQ-2026-08-11-resolucao-de-caminho-dos-hooks-de-agente-independente-do-cwd-attention-signal-cleanup-e-os-5-clis-nao-claude.md

O commit `0c66ecb` (v6.7.1) corrigiu **um** comando de hook — o `trackfw-credential-guard.sh` no
wiring do **Claude Code** — trocando o caminho relativo puro por `$CLAUDE_PROJECT_DIR/scripts/...`,
porque o Claude Code resolve o `command` do hook contra o **cwd dinâmico** do hook e não contra a
raiz do projeto (doc primária: <https://code.claude.com/docs/en/hooks>, "Handlers run in the current
directory"). O próprio commit registrou o restante como fora de escopo.

### Inventário do estado atual (auditoria de 2026-08-11, linha a linha)

| Kind | Onde |
|---|---|
| `$CLAUDE_PROJECT_DIR/...` | **apenas** os 6 entries de credential-guard do Claude Code |
| **Relativo puro** | attention-signal/cleanup dos **6** CLIs (inclusive Claude) + **todos** os entries de credential-guard de Codex, Gemini, Kiro, Copilot e Cursor |
| Absoluto | apenas os hooks de **escopo global** (`trackfw update harness`) — correto por design, fora do repo |

### Cinco fatos que moldam o plano

1. **Não é port mecânico do `0c66ecb`.** Não há mecanismo uniforme entre os 6 CLIs. Indício forte de
   que o Codex CLI não expõe env var de project-dir, e as fontes públicas sobre o Cursor se
   contradizem ("cwd = project root" × "caminho relativo ao `hooks.json`"). Por isso a Wave 0 é de
   **pesquisa bloqueante** e o ADR vem **depois** dela.
2. **Caminho absoluto está proibido no escopo de projeto.** Os arquivos de settings são versionados
   no repo do usuário; gravar o path da máquina que rodou `trackfw init/update` quebra o hook para
   qualquer outro checkout.
3. **Gap crítico de migração.** ✅ **RESOLVIDO no ML-1A.** O helper de reescrita in-place existia
   **só para o Claude**; foi generalizado (`migrateHookCommand` / `_migrate_hook_command`) e ligado
   aos injectors de **Codex e Gemini**. ⚠️ **Correção pós-Barreira B0:** o **Cursor saiu** desta
   lista — veredito `OK`, não muda de string, logo não precisa de migração. Codex e Gemini são
   *merge-based* (leem e mesclam o arquivo existente do usuário) e sem migração —
   trocar a string deles sem migração faria o `trackfw update` **acrescentar** a entrada nova ao lado
   da antiga quebrada, exatamente o bug que a migração do Claude foi escrita para evitar.
   **Kiro e Copilot são isentos**: seus arquivos são regravados por inteiro a cada execução.
4. **As waves são sequenciais, não paralelas.** Todo ML toca os mesmos 3 arquivos
   (`internal/generators/agentfiles.go`, `npm/src/generators/hooks.js`,
   `pypi/trackfw/generators/hooks.py`). Além disso, `scripts/check-agent-hooks-parity.sh` faz diff
   **estrutural** do JSON parseado entre Go×Node×Python — os 3 stacks têm de mudar **no mesmo
   commit** ou o gate falha. Não há microlote paralelizável neste roadmap.
5. **Armadilhas de edição já mapeadas** (obrigatório respeitar):
   - **Go**: ~40 literais inline, um por emissão. Cada linha é uma edição.
   - **Node**: 4 constantes de módulo (`hooks.js:437/438/439/449`) — trocar a constante muda todas
     as emissões de uma vez.

     🔴 **REGRA DURA (vale para os MLs 2A a 7A):** `SIGNAL_CMD` (437), `CLEANUP_CMD` (438) e
     `GUARD_CMD` (439) são **compartilhadas pelos 6 CLIs**. Mutar uma delas em um ML de um CLI
     altera silenciosamente a emissão dos outros 5 — trabalho fora de escopo, no mesmo commit.
     Portanto: **antes de trocar qualquer string, quebrar a constante compartilhada em constantes
     por CLI** (ex.: `SIGNAL_CMD_CLAUDE`, `SIGNAL_CMD_CODEX`, …) e só então alterar a do CLI daquele
     ML. Isso é **incondicional**, não depende do que o ADR decidir.

     ⚠️ **Por que o gate não te protege aqui:** `check-agent-hooks-parity.sh` faz diff
     Go×Node×Python. Como o Go tem literais inline por emissão, mutar a constante compartilhada no
     Node faz o gate falhar apontando **divergência Go×Node nos outros CLIs** — e a "correção"
     intuitiva (mexer no Go para casar) executa as waves 3–7 dentro do ML errado. Se esse `FAIL`
     aparecer, a causa é a constante compartilhada; a correção é dividi-la, nunca alinhar o Go.
   - **Python**: **misto** — só `_GUARD_CMD_CLAUDE` (`hooks.py:268`) é constante; o resto é inline.
   - **Python/Cursor**: o literal aparece **duas vezes por hook**, uma no predicado de dedup e outra
     no `append` (linhas 741/742, 746/747, 756/757, 760/761, e predicado 774 × appends
     780/782/784/786). Editar só o `append` **desliga o dedup** e o injector passa a acrescentar
     entrada nova a cada execução.
   - **Python/Copilot**: o comando aparece **uma vez** em `guard_entry` (`hooks.py:630`) e é
     espalhado em 6 entries via `dict(guard_entry, ...)` (638–639, 646–651).
   - **Node/testes**: `npm/tests/generators.test.js:339–349` e `766–772` asseveram por **índice de
     array** (`PreToolUse[1]`, `[2]`…) — quebram se a ordem ou a contagem de entries mudar, mesmo
     com as strings certas.
   - **`scripts/check-gates-falsify.sh:3530–3531`** fixa byte-a-byte o bloco Kiro do Node
     (`npm/src/generators/hooks.js` ~761–765). Não reformatar esse bloco.

## Acceptance Criteria

- [x] Tabela de verificação dos 6 CLIs com **uma citação de doc primária por célula** (cwd do hook;
      estável × dinâmico; placeholders/env vars de raiz; relativo resolve contra cwd ou contra o
      arquivo de settings) — entregue como arquivo versionado.
- [x] ADR aceito decidindo o mecanismo **por CLI**, admitindo mecanismos distintos, e nomeando
      explicitamente os CLIs em que **nenhuma mudança é necessária**.
- [x] Todo CLI provado quebrado emite comandos que resolvem para a raiz do projeto independentemente
      do cwd, nos 3 stacks (Go, Node.js, Python).
- [x] Todo injector *merge-based* alterado (Claude, Codex, Gemini) tem migração in-place; um
      `trackfw update` sobre settings de versão antiga **reescreve** a entrada, não duplica.
- [x] Testes nos 3 stacks cobrem, por CLI alterado: comando novo emitido, migração de entrada
      antiga, e idempotência (`update` duas vezes → nenhuma entrada duplicada).
- [x] `docs/cli-parity.md` atualizado com a tabela de mecanismo por CLI.
- [x] `go test ./...`, `npm test`, `pytest`, `make quality` verdes; `trackfw validate` sem violações.

### Escopo negativo

Ver a REQ (§"Escopo negativo"). Em resumo: **não** mexer no credential-guard do Claude (já
corrigido), **não** mexer nos hooks de escopo global (absolutos por design), **não** alterar o
conteúdo dos `scripts/trackfw-*.sh`, **não** adicionar/alterar matchers ou eventos, **não** endurecer
o guard de vacuidade P4 de `check-agent-hooks-parity.sh`, **não** corrigir o `settings.json` de
projetos consumidores.

---

## Wave 0 — Verificação em doc primária (1 ML, bloqueante)
> Dependências: nenhuma. **Bloqueia todas as waves seguintes.**

### ML-0A — Semântica de cwd e placeholders de caminho nos 6 CLIs
**Status:** ✅ Concluído (auditado por Zeus em 2026-08-11)
**Resultado:** `docs/pesquisa/2026-08-11-hook-cwd-e-placeholders-por-cli.md`. Vereditos:
Claude `QUEBRADO` · Codex `QUEBRADO` · Gemini `QUEBRADO` · **Cursor `OK`** · **Copilot `OK` (já
correto no código atual)** · **Kiro `INDETERMINADO`**. Achado auditado por Zeus contra o código nos
3 stacks: `InjectCopilotHooks` já emite `"cwd": "."` em todas as entradas
(`agentfiles.go:698–762`, `hooks.js:837–849`, `hooks.py:610/618/631`) — Copilot nunca esteve
quebrado. **Escopo do roadmap caiu de 6 CLIs para 3.**
**Agente:** Prometeu (`prometeu-tf`)
**Arquivos afetados:** cria `docs/pesquisa/2026-08-11-hook-cwd-e-placeholders-por-cli.md` (novo).
**Nenhum arquivo de código é tocado neste ML.**

**Ações:**
1. Para cada um dos 6 CLIs — Claude Code, Codex CLI, Gemini CLI, Kiro, GitHub Copilot CLI, Cursor —
   responder, **exclusivamente contra documentação primária do fornecedor**:
   - (a) Qual é o diretório de trabalho em que o `command` do hook é executado?
   - (b) Esse cwd é fixo na raiz do projeto ou acompanha os `cd` do agente durante a sessão?
   - (c) Que placeholders/env vars de raiz de projeto existem (nome exato, forma de expansão:
     `$VAR`, `${VAR}`, `${workspaceFolder}`…) e em quais campos são expandidos?
   - (d) Um caminho relativo no campo `command` é resolvido contra o cwd ou contra a localização do
     próprio arquivo de settings?
2. Fontes primárias de partida (usar **estas**, não blogs nem resumos de busca):
   - Claude: <https://code.claude.com/docs/en/hooks>
   - Gemini: <https://github.com/google-gemini/gemini-cli/blob/main/docs/hooks/reference.md>
   - Cursor: <https://cursor.com/docs/hooks>
   - Codex: <https://developers.openai.com/codex/config-advanced>
   - Kiro e Copilot: localizar a doc oficial de hooks; se **não existir** doc pública que responda
     (a)–(d), registrar `INDETERMINADO` com a evidência da busca — **não inferir**.
3. Entregar uma tabela 6×4 em que **cada célula traz a URL e a citação literal** que a sustenta.
   Células sem citação são `INDETERMINADO`, nunca inferência.
4. Fechar com uma coluna de veredito por CLI: `QUEBRADO` (cwd dinâmico e caminho relativo resolve
   contra cwd) · `OK` (cwd fixo na raiz, ou relativo resolve contra o settings file) ·
   `INDETERMINADO`.
5. Para cada CLI `QUEBRADO`, listar os mecanismos de correção **disponíveis segundo a própria doc**,
   lembrando que caminho absoluto está vetado (arquivo versionado).

**Critérios de aceite:**
- [ ] Arquivo `docs/pesquisa/2026-08-11-hook-cwd-e-placeholders-por-cli.md` existe e cobre os 6 CLIs.
- [ ] Toda célula preenchida tem URL + citação literal; nenhuma afirmação sem fonte.
- [ ] Toda lacuna está marcada `INDETERMINADO` com a evidência da busca, não preenchida por inferência.
- [ ] Existe veredito explícito por CLI (`QUEBRADO`/`OK`/`INDETERMINADO`).
- [ ] Nenhum arquivo fora de `docs/pesquisa/` foi modificado (`git status` confirma).

**Comandos de validação:**
```bash
git status --porcelain   # só o arquivo novo em docs/pesquisa/
```

---

## Barreira B0 — ADR do mecanismo (Zeus) — ✅ CONCLUÍDA
> Dependências: ML-0A concluído e auditado.

**ADR aceito:**
`docs/adr/ADR-2026-08-11-resolucao-de-caminho-dos-hooks-de-projeto-por-cli-mecanismo-especifico-do-fornecedor-sem-caminho-absoluto.md`

**Strings decididas** (substituem os placeholders `<CMD_*>` dos MLs abaixo):

| CLI | Decisão | String emitida |
|---|---|---|
| Claude | alterar attention-signal/cleanup | `$CLAUDE_PROJECT_DIR/scripts/trackfw-attention-{signal,cleanup}.sh` |
| Codex | alterar todos os comandos | `"$(git rev-parse --show-toplevel)/scripts/trackfw-<script>.sh"` |
| Gemini | alterar todos os comandos | `$GEMINI_PROJECT_DIR/scripts/trackfw-<script>.sh` |
| Cursor | **não alterar** (veredito `OK`) | — |
| Copilot | **não alterar** (já correto via campo nativo `"cwd": "."`) | — |
| Kiro | **não alterar** (`INDETERMINADO`) | — |

**Waves canceladas por esta barreira: ML-5A (Kiro), ML-6A (Copilot), ML-7A (Cursor).** Mantidas no
documento com a razão registrada — não apagar, o registro de *por que* Copilot já estava certo tem
valor.

**Consequência para o ML-1A:** a generalização da migração passa a cobrir **apenas Codex e Gemini**.
Cursor sai do escopo (não muda de string, logo não precisa migrar).

**Consequência que aumenta a importância da regra dura das constantes do Node:** Cursor e Copilot
continuam usando `SIGNAL_CMD`/`CLEANUP_CMD`/`GUARD_CMD`. Como agora são CLIs **verificados
corretos**, mutar a constante compartilhada deixa de ser "vazamento de escopo" e passa a ser
**regressão em wiring comprovadamente bom**. Dividir as constantes é requisito de não-regressão.

Zeus escreve o ADR decidindo o mecanismo de resolução **por CLI**, com base na tabela do ML-0A. O ADR
deve: (i) admitir mecanismos diferentes por CLI quando a verificação mostrar que não há um único;
(ii) nomear os CLIs em que nada muda; (iii) registrar a restrição "sem caminho absoluto em arquivo
versionado"; (iv) definir a **string exata** a ser emitida por CLI, referenciada abaixo como
`<CMD_<CLI>>`. **As waves 1–7 não são liberadas antes disso** — os MLs abaixo estão com a string
propositalmente parametrizada e um agente leve não deve adivinhá-la.

**Default para `INDETERMINADO` (definido agora, para a barreira não travar).** Kiro e Copilot
provavelmente não têm doc de hooks comparável à do Claude, então `INDETERMINADO` é o resultado
esperado, não a exceção. Regra: **`INDETERMINADO` → não alterar o CLI**, e registrar em
`docs/cli-parity.md` como *"mecanismo de resolução não verificável em doc primária — mantido
relativo"*, com a data e o que foi procurado. O ADR pode sobrepor esse default para um CLI
específico se houver evidência empírica direta (teste reproduzível no CLI real), mas nunca por
inferência a partir de outro CLI.

---

## Wave 1 — Migração in-place para os injectors merge-based (1 ML)
> Dependências: Barreira B0. **Pré-requisito de todas as waves de emissão** — sem ele, trocar a
> string em Codex/Gemini/Cursor duplica entradas em vez de corrigir.

### ML-1A — Generalizar o helper de migração para Codex e Gemini
**Status:** ✅ Concluído (auditado por Zeus em 2026-08-11)
> **Escopo reduzido pela Barreira B0:** o Cursor saiu — veredito `OK`, não muda de string, logo não
> precisa de migração. Restam os dois injectors merge-based que vão mudar: **Codex e Gemini**.
**Agente:** Apolo (`apolo-tf`)
**Arquivos afetados:**
- `internal/generators/agentfiles.go` (helper em `:946`)
- `npm/src/generators/hooks.js` (helper em `:95`)
- `pypi/trackfw/generators/hooks.py` (helper em `:236`)
- testes: `internal/generators/agentfiles_test.go`, `npm/tests/generators.test.js`,
  `pypi/tests/test_generators_init.py`

**Ações:**
1. Generalizar `migrateClaudeHookCommand` / `_migrate_claude_hook_command` para uma forma reusável
   pelos formatos de Codex e Gemini — **sem alterar o comportamento atual para o Claude**.
   Ambos usam a forma `matcher` + `hooks[].command`, igual à do Claude — o formato do Cursor
   (`hooks.<evento>[].command`, `beforeShellExecution`/`afterShellExecution`) **não** precisa ser
   suportado, pois o Cursor não muda de string.
2. Manter a ordem de chamada: migração **antes** do merge, para que o dedup por string exata do
   merge não acrescente duplicata.
3. **Não** alterar nenhuma string de comando emitida neste ML — este ML só adiciona capacidade.
4. **Wiring obrigatório:** o helper generalizado tem de ser **efetivamente chamado** pelos injectors
   de Codex e Gemini — inicialmente com `old == new` (a string atual), o que é um no-op
   funcional mas prova que o ponto de chamada existe e está na ordem certa (antes do merge). Um
   helper generalizado e nunca ligado passaria em todos os gates e reabriria o buraco lá na frente.
5. Testes novos: dado um settings file com a string antiga, após a migração a entrada é
   **reescrita** (não duplicada), para Codex e Gemini.

**Critérios de aceite:**
- [x] Nenhuma string de comando emitida mudou (`git diff` não mostra alteração de literal de comando).
- [x] Helper cobre os formatos de Codex e Gemini além do já suportado Claude, nos 3 stacks.
- [x] ~~Cada formato tem teste que invoca o **injector real** provando reescrita in-place~~ —
      **DEFERIDO para ML-3A/ML-4A por impossibilidade estrutural, aceito por Zeus.** Com `old == new`
      (mandato deste ML) a chamada é um no-op funcional e **nenhum teste consegue distinguir
      "migração ligada" de "migração ausente"**. Apolo provou empiricamente: desabilitou as 6
      chamadas em `InjectCodexHooks` e a suíte inteira continuou verde, inclusive o teste novo.
      Zeus auditou o ponto de chamada **por leitura de código** (única prova possível hoje) e
      confirmou nos 3 stacks que a migração roda **antes** do merge: Go `agentfiles.go:346–351`
      (Codex) e `:466–473` (Gemini); Node `hooks.js:648–653` e `:710–717`; Python `hooks.py:388–412`
      e `:472–499`. A prova comportamental vira critério **bloqueante** do ML-3A/ML-4A (abaixo), onde
      `oldCommand != newCommand` torna a migração observável.
- [ ] Cada formato tem teste que invoca o **injector real** (`InjectCodexHooks` / `InjectGeminiHooks`
      e equivalentes Node/Python) contra um fixture com a string antiga e
      assevera reescrita in-place — **não** teste unitário do helper isolado. Este critério é o que
      distingue "helper existe" de "migração funciona".
- [ ] `go test ./...`, `npm test`, `pytest` verdes.
- [ ] `bash scripts/check-agent-hooks-parity.sh` sem `FAIL`.

**Comandos de validação:**
```bash
go build ./... && go test ./...
npm --prefix npm test
python -m pytest pypi/tests -q
bash scripts/check-agent-hooks-parity.sh
```

---

## Wave 2 — Claude Code: attention-signal e attention-cleanup (1 ML)
> Dependências: Wave 1. Único CLI já **provado** quebrado (mesmo mecanismo do `0c66ecb`; difere só
> na frequência, pois os hooks de attention casam apenas o matcher `AskUserQuestion`).

### ML-2A — Emitir `$CLAUDE_PROJECT_DIR/...` para attention-signal/cleanup + migração
**Status:** ✅ Concluído (Apolo; auditado e aprovado por Zeus em 2026-08-11)
**Agente:** Apolo (`apolo-tf`)
**Arquivos afetados:**
- `internal/generators/agentfiles.go` — linhas **211** (signal) e **265** (cleanup)
- `npm/src/generators/hooks.js` — constantes **437** (`SIGNAL_CMD`) e **438** (`CLEANUP_CMD`)
  🔴 **passo obrigatório e incondicional, antes de qualquer troca de string:** dividir `SIGNAL_CMD` e
  `CLEANUP_CMD` em constantes por CLI (as 6 mantendo o valor atual), religar cada injector à sua, e
  **só então** alterar a do Claude. Sem isso, este ML altera silenciosamente a emissão dos outros 5
  CLIs. Ver a REGRA DURA no §Context.
- `pypi/trackfw/generators/hooks.py` — linhas **279** (signal) e **303** (cleanup)
- testes: `internal/generators/agentfiles_test.go` (69/72/75/78, 112/115/118/121),
  `npm/tests/generators.test.js` (339–349 — **asserções por índice de array**),
  `pypi/tests/test_generators_init.py` (484, 508–512, 529)

**Ações:**
0. **Primeiro**, dividir as constantes compartilhadas do Node (§Context, REGRA DURA) preservando o
   valor atual para os 6 CLIs. Confirmar com `git diff` que nenhuma emissão mudou ainda.
1. Trocar a string emitida para `$CLAUDE_PROJECT_DIR/scripts/trackfw-attention-signal.sh` e
   `$CLAUDE_PROJECT_DIR/scripts/trackfw-attention-cleanup.sh` (Barreira B0) nas linhas acima.
2. Estender a chamada de migração (`agentfiles.go:231–232`, `hooks.js:588–589`, `hooks.py:287–288`)
   para reescrever também `scripts/trackfw-attention-signal.sh` e
   `scripts/trackfw-attention-cleanup.sh` no matcher `AskUserQuestion`.
3. Atualizar os testes listados. Nas asserções por índice do Node, conferir que a ordem de emissão
   **não** mudou.

**Critérios de aceite:**
- [ ] Os 3 stacks emitem `<CMD_CLAUDE>` para signal e cleanup; `git grep` não encontra mais
      `"scripts/trackfw-attention-signal.sh"` como comando emitido no wiring do Claude.
- [ ] Migração reescreve entrada antiga; `trackfw update` duas vezes → nenhuma duplicata.
- [ ] 🔴 **Prova negativa da migração (aqui já é possível, pois `old != new`).** Não basta o teste
      passar: **remover a chamada de migração dos scripts de attention tem de fazer um teste
      falhar.** Comente a chamada, rode a suíte, confirme a falha, restaure, e reporte o resultado.
      Suíte verde sem a migração = teste que não prova nada.
- [ ] `go test ./...`, `npm test`, `pytest` verdes.
- [ ] `bash scripts/check-agent-hooks-parity.sh` sem `FAIL` (prova que os 3 stacks mudaram juntos).
- [ ] Nenhum entry de credential-guard do Claude foi alterado.
- [ ] **Não-regressão dos outros 5 CLIs:** as emissões de Codex, Gemini, Kiro, Copilot e Cursor são
      **byte-idênticas** antes e depois deste ML. Provar rodando o injector sobre um fixture limpo
      antes e depois e comparando os arquivos gerados (`.codex/hooks.json`, `.gemini/settings.json`,
      `.kiro/hooks/trackfw-attention.json`, `.github/hooks/trackfw-attention.json`,
      `.cursor/hooks.json`) — diff vazio nos 5.

**Comandos de validação:** idem ML-1A.

---

## Waves 3–7 — Um CLI por wave, **sequenciais** (1 ML cada)
> ⚠️ **Após a Barreira B0 restam apenas ML-3A (Codex) e ML-4A (Gemini).** ML-5A, ML-6A e ML-7A foram
> canceladas — motivos registrados em cada uma.
> Dependências: Wave 2 (encadeadas: 3 → 4 → 5 → 6 → 7).
> **Cada wave só é executada se o ML-0A tiver dado veredito `QUEBRADO` para aquele CLI.** Vereditos
> `OK` cancelam a wave; `INDETERMINADO` bloqueia e volta para Zeus.
> Motivo da sequencialidade: todos os MLs editam os mesmos 3 arquivos, e o gate de paridade compara
> estruturalmente o JSON dos 3 stacks.

Estrutura idêntica em todas: trocar a string emitida por `<CMD_<CLI>>` **em todas** as linhas
inventariadas abaixo, adicionar/ajustar a migração (quando merge-based), atualizar os testes
listados, respeitando as armadilhas de edição do §Context.

### ML-3A — Codex (`.codex/hooks.json`) — merge-based, **precisa de migração**
**Status:** ✅ Concluído (Apolo; auditado e aprovado por Zeus em 2026-08-11)
**String a emitir:** o valor JSON do campo `command` deve conter as **aspas literais** em torno da
substituição, exatamente como nos exemplos oficiais do fornecedor — no arquivo gerado isso aparece
JSON-escapado:
```
"command": "\"$(git rev-parse --show-toplevel)/scripts/trackfw-attention-signal.sh\""
```
⚠️ Errar isso **não é pego pelo gate de paridade**: os 3 stacks errariam de forma idêntica e o diff
estrutural passaria.

🔴 **PASSO 0 obrigatório, mesma disciplina do ML-2A:** `GUARD_CMD` (`npm/src/generators/hooks.js`)
**ainda é constante compartilhada pelos 6 CLIs** — o ML-2A dividiu só `SIGNAL_CMD`/`CLEANUP_CMD`.
Divida `GUARD_CMD` em constantes por CLI, **todas com o valor atual**, religue os injectors, confirme
por `git diff` que nenhuma emissão mudou, e **só então** altere a variante do Codex. Sem isso, este
ML altera silenciosamente os entries de credential-guard de Gemini, Kiro, Copilot e Cursor — e
Copilot/Cursor são CLIs **verificados corretos**.
🔴 **Critério de aceite EXTRA e bloqueante (só deste ML) — PROVA DO MODELO DE EXECUÇÃO DO CODEX.**
O Codex é o único caso em que a correção pode **piorar** a situação: se o `command` não for executado
via shell, o `$(...)` não expande, e o hook passa a falhar **sempre** — não só sob cwd derivado.

⛔ **NÃO conta como prova** rodar a string em `bash` a partir de um subdiretório. Isso só demonstra
que o `bash` expande `$(...)`, coisa que nunca esteve em dúvida. A pergunta é sobre **o executor do
Codex**, não sobre o shell.

O Codex CLI **está instalado nesta máquina** (`/opt/homebrew/bin/codex`, `codex-cli 0.147.0`) —
confirmado por Zeus. Portanto a prova É executável e é obrigatória:

1. Criar um repositório git de fixture com `scripts/trackfw-attention-signal.sh` (que apenas escreve
   uma marca em arquivo, ex.: `echo fired > /tmp/trackfw-codex-proof`) e o `.codex/hooks.json`
   gerado pelo injector deste ML.
2. Rodar o `codex` **a partir de um subdiretório** desse repo, disparando o evento que ativa o hook.
3. Confirmar que a marca foi escrita — ou seja, que o Codex expandiu o `$(...)` e executou o script.
4. Reportar a Zeus o comando exato, a saída e o resultado.

Se, após tentativa real, a verificação se mostrar impraticável (ex.: o Codex exigir autenticação
interativa indisponível), **não altere o Codex**: reverta as mudanças deste CLI, reporte a Zeus, e o
ML vira "não alterado, registrado como não verificável" — mesmo default do Kiro (ADR §Consequences).
Essa é uma saída legítima e prevista; forçar a mudança sem a prova, não.
**Linhas:** Go `344, 356, 361, 368, 374, 379` · Node `636, 642, 643, 645, 647, 648` (via constantes
437/438/439) · Python `378, 389, 392, 397, 401, 404`
**Testes:** Go `agentfiles_test.go` 285/288/291/294/341 · Node `generators.test.js` 410–420, 455 ·
Python `test_generators_init.py` 546, 552

### ML-4A — Gemini (`.gemini/settings.json`) — merge-based, **precisa de migração**
**Status:** ✅ Concluído (Apolo; auditado e aprovado por Zeus em 2026-08-11)
**String a emitir:** `$GEMINI_PROJECT_DIR/scripts/trackfw-<script>.sh`
**Linhas:** Go `457, 470, 475, 480, 487, 493, 498, 503` · Node `685, 691, 692, 693, 695, 697, 698,
699` · Python `450, 459, 466, 467, 470, 472, 473, 474`
**Testes:** Go `agentfiles_test.go` 258–267, 367–376, 422 · Node `generators.test.js` 466–479, 518 ·
Python `test_generators_init.py` 595, 614, 653

### ML-5A — Kiro — ❌ **CANCELADO pela Barreira B0**
**Status:** ❌ Cancelado
**Motivo:** veredito `INDETERMINADO` no ML-0A — as 4 páginas oficiais de hooks do Kiro nunca
mencionam o cwd de execução da "Shell Command action" nem expõem env var de raiz de projeto.
Aplica-se o default do roadmap: **não alterar**, registrar em `docs/cli-parity.md` como não
verificável (feito no ML-8A). Linhas mantidas abaixo apenas como registro do que *seria* editado se
surgir doc primária no futuro.
**Linhas:** Go `575, 582, 598, 605, 615, 622, 629, 636` · Node `735, 742, 756, 763, 772, 779, 786,
793` · Python `511, 518, 531, 538, 548, 555, 562, 569`
**Testes:** Node `generators.test.js` 533–547 · Python `test_generators_init.py` 678
⚠️ **`scripts/check-gates-falsify.sh:3530–3531` fixa byte-a-byte o bloco Kiro do Node
(`hooks.js` ~761–765).** Alterar apenas o valor do campo `command`; **não** reformatar, reordenar
nem reindentar esse bloco. Se a sabotagem quebrar, é regressão do ML, não do gate.

### ML-6A — Copilot — ❌ **CANCELADO pela Barreira B0: já estava correto**
**Status:** ❌ Cancelado
**Motivo:** veredito `OK`. `InjectCopilotHooks` **já** emite o campo nativo `"cwd": "."` em todas as
entradas nos 3 stacks (`agentfiles.go:698–762`, `hooks.js:837–849`, `hooks.py:610/618/631`) —
verificado por Zeus lendo o código, não só pela doc. A doc do GitHub define o campo como *"Working
directory for the command (relative to repository root or absolute)"*, então o caminho relativo
dentro de `bash` resolve certo **por causa** desse campo. Copilot nunca esteve quebrado, e é o
precedente que o ADR registra: campo estruturado de cwd > placeholder em string.
**Linhas:** Go `697, 705, 726, 733, 740, 747, 754, 761` · Node `837, 838, 844–849` · Python `609,
617` + **`630` (`guard_entry`, espalhado em 6 entries via `dict(guard_entry, ...)` em 638–639 e
646–651 — uma edição só)**
**Testes:** Go `agentfiles_test.go` 566–587, `copilot_hooks_parity_test.go` 169–172 · Node
`generators.test.js` 586–608 · Python `test_generators_init.py` 724–746

### ML-7A — Cursor — ❌ **CANCELADO pela Barreira B0**
**Status:** ❌ Cancelado
**Motivo:** veredito `OK`. Doc primária: *"Project hooks (`.cursor/hooks.json` in a repository): Run
from the project root"*, e o exemplo canônico ensina exatamente a usar caminho relativo à raiz
(`.cursor/hooks/script.sh`, não `./hooks/script.sh`). Emitir `$CURSOR_PROJECT_DIR/...` seria mudança
sem defeito correspondente. **Consequência:** a armadilha do dedup duplo no Python/Cursor e a
necessidade de migração para o Cursor deixam de ser restrições ativas deste roadmap.
**Linhas:** Go `877, 878, 888, 889, 900, 901, 902, 903` (+ purga de legado `879, 880`) · Node
`935/936/938, 941/942/944, 951/952, 956/957, 968, 971, 974, 977, 980` · Python **`741+742, 746+747,
756+757, 760+761`** e **predicado `774` × appends `780, 782, 784, 786`**
**Testes:** Go `agentfiles_test.go` 632–697 · Node `generators.test.js` 630–678,
`credential_guard.test.js` 370, 372 · Python `test_generators_init.py` 774–854
⚠️ **Armadilha Python/Cursor:** cada comando aparece **duas vezes** — no predicado de dedup e no
`append`. Editar **os dois**. Editar só o `append` desliga o dedup e o injector passa a acrescentar
uma entrada nova a cada execução. Critério de aceite dedicado abaixo.

**Critérios de aceite (idênticos para ML-3A e ML-4A — únicos MLs vivos desta faixa):**
- [ ] Os 3 stacks emitem a string decidida na Barreira B0 em **todas** as linhas inventariadas.
- [ ] **Não-regressão:** as emissões de Cursor, Copilot e Kiro são **byte-idênticas** antes e depois
      (são CLIs verificados corretos — alterá-los é regressão, não melhoria).
- [ ] `git grep` não encontra mais o caminho relativo puro no wiring daquele CLI, em nenhum stack.
- [ ] Migração reescreve a entrada antiga in-place (ambos são merge-based).
- [ ] 🔴 **Prova comportamental da migração (deferida do ML-1A, agora bloqueante).** Não basta o
      teste passar: **remover a chamada de migração tem de fazer um teste falhar.** Verifique
      explicitamente — comente as chamadas `migrateHookCommand`/`_migrate_hook_command` deste CLI,
      rode a suíte, confirme que **falha**, e restaure. Se a suíte continuar verde sem a migração, o
      teste não prova nada e o ML **não** está concluído. Reporte o resultado dessa checagem a Zeus.
- [ ] **Idempotência**: rodar o injector duas vezes sobre o mesmo arquivo produz JSON idêntico
      (prova que o dedup continua funcionando — critério que captura a armadilha Python/Cursor).
- [ ] Testes dos 3 stacks atualizados e verdes.
- [ ] `bash scripts/check-agent-hooks-parity.sh` sem `FAIL`.
- [ ] `bash scripts/check-gates-falsify.sh` sem regressão.
- [ ] Nenhum arquivo em `internal/generators/update.go`, `npm/src/commands/update-harness.js` ou
      `pypi/trackfw/commands/update_harness.py` foi tocado (escopo global é fora de escopo).

**Comandos de validação (ML-3A e ML-4A):**
```bash
go build ./... && go test ./...
npm --prefix npm test
python -m pytest pypi/tests -q
bash scripts/check-agent-hooks-parity.sh
bash scripts/check-gates-falsify.sh
```

---

## Wave 8 — Barreira de qualidade, segurança e documentação (2 MLs)
> Dependências: última wave de emissão executada.

### ML-8A — Documentação de paridade + gate final
**Status:** ✅ Concluído (auditado e aprovado por Zeus em 2026-08-11)
**Agente:** Hefesto (`hefesto-tf`)
**Arquivos afetados:** `docs/cli-parity.md` (somente). **Não modifica código de produto.**
**Ações:** adicionar seção "Mecanismo de resolução de caminho dos hooks de projeto, por CLI" com a
tabela do ADR (CLI → mecanismo → string emitida → tem migração? sim/não e por quê), citando o ADR.
Rodar `make quality` e reportar.
**Critérios de aceite:**
- [ ] `docs/cli-parity.md` tem a tabela dos 6 CLIs, coerente com o ADR e com o código.
- [ ] `docs/cli-parity.md` registra as duas **pré-condições do fix do Codex**, descobertas
      empiricamente no ML-3A e ausentes da doc do fornecedor: (i) o hook só roda em projeto marcado
      como *trusted* em `~/.codex/config.toml`; (ii) `git rev-parse --show-toplevel` exige
      repositório git e retorna a raiz do submódulo/worktree quando aplicável. Ver ADR Emenda 1 e
      `vault/notes/codex-hooks-de-projeto-so-rodam-em-projeto-trusted-2026-08-11.md`.
- [ ] `docs/cli-parity.md` registra Kiro como *"mecanismo de resolução não verificável em doc
      primária — mantido relativo"*, com data 2026-08-11 e as URLs consultadas (ADR, seção Kiro).
- [ ] `make quality` exit 0, 0 `FAIL`.
- [ ] `internal/`, `npm/src/`, `pypi/trackfw/` intocados neste ML.

### ML-8B — Revisão de segurança do wiring alterado
**Status:** ✅ Concluído (auditado e aprovado por Zeus em 2026-08-11)
**Resultado:** `docs/seguranca/2026-08-11-revisao-hooks-cwd.md`. Vereditos: Q1 (injeção shell no
Codex) `OK`, ancorado na prova empírica do ML-3A; Q2 (expansão de variável Claude/Gemini) `OK`,
degradação sempre fail-to-run; Q3 (falha silenciosa do guard, 6 CLIs) `RISCO ACEITÁVEL` — sem
regressão frente à `main`, dois casos de falha novos e estreitos no Codex (um já documentado em
`docs/cli-parity.md`), semântica de fail-aberto/fail-fechado por hook não alterada por este roadmap
e registrada como gap de verificação não fechado (follow-up, não bloqueio); Q4 (migração in-place)
`RISCO ACEITÁVEL` — mesmo modelo de match pré-existente, sem mudança de estratégia; Q5 (supply
chain) `OK`. **Recomendação: seguir para PR.** Nenhum controle foi enfraquecido em relação à `main`.
**Agente:** Hades (`hades-tf`)
**Arquivos afetados:** nenhum código. `docs/seguranca/2026-08-11-revisao-hooks-cwd.md` (novo),
`docs/agents-working-context.md`, este campo Status.
**Ações:** revisar se o novo mecanismo de resolução introduz superfície de ataque — em especial:
expansão de variável em campo de comando executado por shell, possibilidade de a variável ser
controlada pelo repositório em vez do CLI, e se a mudança pode fazer o credential-guard **deixar de
executar** silenciosamente (falha aberta) em algum CLI.
**Critérios de aceite:**
- [x] Parecer escrito cobrindo os 6 CLIs.
- [x] Confirmação explícita de que nenhum caminho novo permite o guard falhar em silêncio de forma
      pior que a `main` — dois casos estreitos no Codex identificados e documentados, não
      bloqueantes; semântica de falha de hook por CLI registrada como não verificada (follow-up).
- [x] Nenhum arquivo modificado por este agente além dos 3 permitidos (achados, working-context,
      status deste ML).

---

### ML-8C — Adendo: `GIT_DIR`/`GIT_WORK_TREE` na doc de pré-condições do Codex
**Status:** ✅ Concluído (auditado e aprovado por Zeus em 2026-08-11)
**Agente:** Hefesto (`hefesto-tf`) · **Origem:** achado Q3 do ML-8B (Hades)
**Microlote corretivo** despachado pela barreira: a revisão de segurança identificou um **terceiro**
caso da mesma família dos dois já documentados — `GIT_DIR`/`GIT_WORK_TREE` no ambiente redirecionam a
resolução de `git rev-parse --show-toplevel`, com a mesma consequência prática: o
`trackfw-credential-guard.sh` **pode deixar de executar em silêncio**. Documentado no item 2 da
subseção "Pré-condições do fix do Codex" em `docs/cli-parity.md`, citando o parecer como origem.
Classificado pelo parecer como **não bloqueante e sem regressão contra a `main`**.

---

## Verificações finais de Zeus na árvore fechada (2026-08-11)

Feitas **na árvore final**, não por ML, porque alguns critérios consolidados só são verificáveis
depois de todos os MLs:

- **Idempotência** (critério que nenhuma sabotagem por-ML conseguiria revelar): `trackfw update`
  rodado **3×** sobre o mesmo fixture → `.codex/hooks.json`, `.gemini/settings.json`,
  `.claude/settings.json` e `.cursor/hooks.json` **byte-idênticos** entre a 1ª e a 3ª execução.
  Importava porque Codex e Gemini agora emitem strings com `$` e `$(...)`: se o dedup por string
  exata do merge normalizasse ou reexpandisse, a 2ª execução acrescentaria entradas duplicadas — e
  nenhum gate pegaria.
- **Credential-guard do Claude intacto na árvore final** (não só na do ML-2A, já que ML-3A/4A
  mexeram nos blocos de constantes): `.claude/settings.json` gerado tem **6** entries de
  credential-guard, todos `$CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh`.

## Follow-ups abertos como REQ, para não evaporarem no fechamento

- `docs/req/REQ-2026-08-11-semantica-de-falha-de-hook-fail-open-vs-fail-closed-por-cli-*.md` —
  achado Q3 do parecer de segurança: **nenhuma fonte** consultada estabelece se a falha de um hook é
  fail-open ou fail-closed, por CLI. O credential-guard é controle de **negação** e hoje há 3
  caminhos documentados que terminam em "guard não roda em silêncio". Este roadmap **não** alterou
  essa semântica — por isso follow-up, não bloqueio.
- `docs/req/REQ-2026-08-11-prova-negativa-dedicada-para-o-guard-de-vacuidade-credential-guard-present-*.md` —
  carregado por Hefesto em **duas** sessões (2026-08-08 e 2026-08-11) sem endereçamento: o guard de
  vacuidade `credential-guard-present` não tem prova negativa própria; o Cenário 44 falsifica só o
  comparador estrutural.

---

## Notas de execução

- **Autoridade de Git:** apenas Zeus cria branch, commita e faz push. Todo especialista entrega o
  trabalho **sem commit**.
- **Ordem obrigatória:** Wave 0 → Barreira B0 (ADR) → Wave 1 → Wave 2 → Waves 3–7 (encadeadas) →
  Wave 8. Nenhum paralelismo: todos os MLs de emissão editam os mesmos 3 arquivos e o gate de
  paridade exige que os 3 stacks mudem no mesmo commit.
- **Regra de paridade dos 3 CLIs do trackfw** (Go + Node.js + Python) vale para todos os MLs de
  código — nenhum ML é considerado concluído com apenas um stack alterado.
