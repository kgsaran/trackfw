---
status: wip
date: 2026-08-11
req: "docs/req/REQ-2026-08-11-resolucao-de-caminho-dos-hooks-de-agente-independente-do-cwd-attention-signal-cleanup-e-os-5-clis-nao-claude.md"
squad: "Prometeu, Apolo, Ártemis, Hefesto, Hades"
---

# Roadmap: Resolucao de caminho dos hooks de agente independente do cwd

> Created: 2026-08-11 | Status: wip

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
3. **Gap crítico de migração.** O helper de reescrita in-place (`migrateClaudeHookCommand` /
   `_migrate_claude_hook_command`) existe **só para o Claude**. Codex, Gemini e Cursor também são
   injectors *merge-based* (leem e mesclam o arquivo existente do usuário) e **não têm migração** —
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

- [ ] Tabela de verificação dos 6 CLIs com **uma citação de doc primária por célula** (cwd do hook;
      estável × dinâmico; placeholders/env vars de raiz; relativo resolve contra cwd ou contra o
      arquivo de settings) — entregue como arquivo versionado.
- [ ] ADR aceito decidindo o mecanismo **por CLI**, admitindo mecanismos distintos, e nomeando
      explicitamente os CLIs em que **nenhuma mudança é necessária**.
- [ ] Todo CLI provado quebrado emite comandos que resolvem para a raiz do projeto independentemente
      do cwd, nos 3 stacks (Go, Node.js, Python).
- [ ] Todo injector *merge-based* alterado (Claude, Codex, Gemini, Cursor) tem migração in-place; um
      `trackfw update` sobre settings de versão antiga **reescreve** a entrada, não duplica.
- [ ] Testes nos 3 stacks cobrem, por CLI alterado: comando novo emitido, migração de entrada
      antiga, e idempotência (`update` duas vezes → nenhuma entrada duplicada).
- [ ] `docs/cli-parity.md` atualizado com a tabela de mecanismo por CLI.
- [ ] `go test ./...`, `npm test`, `pytest`, `make quality` verdes; `trackfw validate` sem violações.

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
**Status:** ⬜ Pendente
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

## Barreira B0 — ADR do mecanismo (Zeus)
> Dependências: ML-0A concluído e auditado.

Zeus escreve o ADR decidindo o mecanismo de resolução **por CLI**, com base na tabela do ML-0A. O ADR
deve: (i) admitir mecanismos diferentes por CLI quando a verificação mostrar que não há um único;
(ii) nomear os CLIs em que nada muda; (iii) registrar a restrição "sem caminho absoluto em arquivo
versionado"; (iv) definir a **string exata** a ser emitida por CLI, referenciada abaixo como
`<CMD_<CLI>>`. **As waves 1–7 não são liberadas antes disso** — os MLs abaixo estão com a string
propositalmente parametrizada e um agente leve não deve adivinhá-la.

Zeus também decide aqui, à luz do ML-0A, quais das waves 3–7 são **canceladas** por veredito `OK`.

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

### ML-1A — Generalizar o helper de migração para Codex, Gemini e Cursor
**Status:** ⬜ Pendente
**Agente:** Apolo (`apolo-tf`)
**Arquivos afetados:**
- `internal/generators/agentfiles.go` (helper em `:946`)
- `npm/src/generators/hooks.js` (helper em `:95`)
- `pypi/trackfw/generators/hooks.py` (helper em `:236`)
- testes: `internal/generators/agentfiles_test.go`, `npm/tests/generators.test.js`,
  `pypi/tests/test_generators_init.py`

**Ações:**
1. Generalizar `migrateClaudeHookCommand` / `_migrate_claude_hook_command` para uma forma reusável
   pelos formatos de Codex, Gemini e Cursor — **sem alterar o comportamento atual para o Claude**.
   Os formatos JSON alvo estão documentados no inventário: Codex e Gemini usam a forma
   `matcher` + `hooks[].command`; Cursor usa `hooks.<evento>[].command` (com `matcher` opcional) e
   `beforeShellExecution`/`afterShellExecution`.
2. Manter a ordem de chamada: migração **antes** do merge, para que o dedup por string exata do
   merge não acrescente duplicata.
3. **Não** alterar nenhuma string de comando emitida neste ML — este ML só adiciona capacidade.
4. **Wiring obrigatório:** o helper generalizado tem de ser **efetivamente chamado** pelos injectors
   de Codex, Gemini e Cursor — inicialmente com `old == new` (a string atual), o que é um no-op
   funcional mas prova que o ponto de chamada existe e está na ordem certa (antes do merge). Um
   helper generalizado e nunca ligado passaria em todos os gates e reabriria o buraco lá na frente.
5. Testes novos: dado um settings file com a string antiga, após a migração a entrada é
   **reescrita** (não duplicada), para cada um dos 3 formatos.

**Critérios de aceite:**
- [ ] Nenhuma string de comando emitida mudou (`git diff` não mostra alteração de literal de comando).
- [ ] Helper cobre os 3 formatos (Codex/Gemini/Cursor) além do já suportado Claude, nos 3 stacks.
- [ ] Cada formato tem teste que invoca o **injector real** (`InjectCodexHooks` / `InjectGeminiHooks`
      / `InjectCursorHooks` e equivalentes Node/Python) contra um fixture com a string antiga e
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

### ML-2A — Emitir `<CMD_CLAUDE>` para attention-signal/cleanup + migração
**Status:** ⬜ Pendente
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
1. Trocar a string emitida para `<CMD_CLAUDE>` (definida na Barreira B0) nas linhas acima.
2. Estender a chamada de migração (`agentfiles.go:231–232`, `hooks.js:588–589`, `hooks.py:287–288`)
   para reescrever também `scripts/trackfw-attention-signal.sh` e
   `scripts/trackfw-attention-cleanup.sh` no matcher `AskUserQuestion`.
3. Atualizar os testes listados. Nas asserções por índice do Node, conferir que a ordem de emissão
   **não** mudou.

**Critérios de aceite:**
- [ ] Os 3 stacks emitem `<CMD_CLAUDE>` para signal e cleanup; `git grep` não encontra mais
      `"scripts/trackfw-attention-signal.sh"` como comando emitido no wiring do Claude.
- [ ] Migração reescreve entrada antiga; `trackfw update` duas vezes → nenhuma duplicata.
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
> Dependências: Wave 2 (encadeadas: 3 → 4 → 5 → 6 → 7).
> **Cada wave só é executada se o ML-0A tiver dado veredito `QUEBRADO` para aquele CLI.** Vereditos
> `OK` cancelam a wave; `INDETERMINADO` bloqueia e volta para Zeus.
> Motivo da sequencialidade: todos os MLs editam os mesmos 3 arquivos, e o gate de paridade compara
> estruturalmente o JSON dos 3 stacks.

Estrutura idêntica em todas: trocar a string emitida por `<CMD_<CLI>>` **em todas** as linhas
inventariadas abaixo, adicionar/ajustar a migração (quando merge-based), atualizar os testes
listados, respeitando as armadilhas de edição do §Context.

### ML-3A — Codex (`.codex/hooks.json`) — merge-based, **precisa de migração**
**Status:** ⬜ Pendente · **Agente:** Apolo (`apolo-tf`)
**Linhas:** Go `344, 356, 361, 368, 374, 379` · Node `636, 642, 643, 645, 647, 648` (via constantes
437/438/439) · Python `378, 389, 392, 397, 401, 404`
**Testes:** Go `agentfiles_test.go` 285/288/291/294/341 · Node `generators.test.js` 410–420, 455 ·
Python `test_generators_init.py` 546, 552

### ML-4A — Gemini (`.gemini/settings.json`) — merge-based, **precisa de migração**
**Status:** ⬜ Pendente · **Agente:** Apolo (`apolo-tf`)
**Linhas:** Go `457, 470, 475, 480, 487, 493, 498, 503` · Node `685, 691, 692, 693, 695, 697, 698,
699` · Python `450, 459, 466, 467, 470, 472, 473, 474`
**Testes:** Go `agentfiles_test.go` 258–267, 367–376, 422 · Node `generators.test.js` 466–479, 518 ·
Python `test_generators_init.py` 595, 614, 653

### ML-5A — Kiro (`.kiro/hooks/trackfw-attention.json`) — **arquivo regravado, sem migração**
**Status:** ⬜ Pendente · **Agente:** Apolo (`apolo-tf`)
**Linhas:** Go `575, 582, 598, 605, 615, 622, 629, 636` · Node `735, 742, 756, 763, 772, 779, 786,
793` · Python `511, 518, 531, 538, 548, 555, 562, 569`
**Testes:** Node `generators.test.js` 533–547 · Python `test_generators_init.py` 678
⚠️ **`scripts/check-gates-falsify.sh:3530–3531` fixa byte-a-byte o bloco Kiro do Node
(`hooks.js` ~761–765).** Alterar apenas o valor do campo `command`; **não** reformatar, reordenar
nem reindentar esse bloco. Se a sabotagem quebrar, é regressão do ML, não do gate.

### ML-6A — Copilot (`.github/hooks/trackfw-attention.json`) — **arquivo regravado, sem migração**
**Status:** ⬜ Pendente · **Agente:** Apolo (`apolo-tf`)
**Linhas:** Go `697, 705, 726, 733, 740, 747, 754, 761` · Node `837, 838, 844–849` · Python `609,
617` + **`630` (`guard_entry`, espalhado em 6 entries via `dict(guard_entry, ...)` em 638–639 e
646–651 — uma edição só)**
**Testes:** Go `agentfiles_test.go` 566–587, `copilot_hooks_parity_test.go` 169–172 · Node
`generators.test.js` 586–608 · Python `test_generators_init.py` 724–746

### ML-7A — Cursor (`.cursor/hooks.json`) — merge-based, **precisa de migração**
**Status:** ⬜ Pendente · **Agente:** Apolo (`apolo-tf`)
**Linhas:** Go `877, 878, 888, 889, 900, 901, 902, 903` (+ purga de legado `879, 880`) · Node
`935/936/938, 941/942/944, 951/952, 956/957, 968, 971, 974, 977, 980` · Python **`741+742, 746+747,
756+757, 760+761`** e **predicado `774` × appends `780, 782, 784, 786`**
**Testes:** Go `agentfiles_test.go` 632–697 · Node `generators.test.js` 630–678,
`credential_guard.test.js` 370, 372 · Python `test_generators_init.py` 774–854
⚠️ **Armadilha Python/Cursor:** cada comando aparece **duas vezes** — no predicado de dedup e no
`append`. Editar **os dois**. Editar só o `append` desliga o dedup e o injector passa a acrescentar
uma entrada nova a cada execução. Critério de aceite dedicado abaixo.

**Critérios de aceite (idênticos para ML-3A … ML-7A):**
- [ ] Os 3 stacks emitem `<CMD_<CLI>>` em **todas** as linhas inventariadas para aquele CLI.
- [ ] `git grep` não encontra mais o caminho relativo puro no wiring daquele CLI, em nenhum stack.
- [ ] (merge-based: Codex, Gemini, Cursor) migração reescreve a entrada antiga in-place.
- [ ] **Idempotência**: rodar o injector duas vezes sobre o mesmo arquivo produz JSON idêntico
      (prova que o dedup continua funcionando — critério que captura a armadilha Python/Cursor).
- [ ] Testes dos 3 stacks atualizados e verdes.
- [ ] `bash scripts/check-agent-hooks-parity.sh` sem `FAIL`.
- [ ] `bash scripts/check-gates-falsify.sh` sem regressão.
- [ ] Nenhum arquivo em `internal/generators/update.go`, `npm/src/commands/update-harness.js` ou
      `pypi/trackfw/commands/update_harness.py` foi tocado (escopo global é fora de escopo).

**Comandos de validação (todas as waves 3–7):**
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
**Status:** ⬜ Pendente
**Agente:** Hefesto (`hefesto-tf`)
**Arquivos afetados:** `docs/cli-parity.md` (somente). **Não modifica código de produto.**
**Ações:** adicionar seção "Mecanismo de resolução de caminho dos hooks de projeto, por CLI" com a
tabela do ADR (CLI → mecanismo → string emitida → tem migração? sim/não e por quê), citando o ADR.
Rodar `make quality` e reportar.
**Critérios de aceite:**
- [ ] `docs/cli-parity.md` tem a tabela dos 6 CLIs, coerente com o ADR e com o código.
- [ ] `make quality` exit 0, 0 `FAIL`.
- [ ] `internal/`, `npm/src/`, `pypi/trackfw/` intocados neste ML.

### ML-8B — Revisão de segurança do wiring alterado
**Status:** ⬜ Pendente
**Agente:** Hades (`hades-tf`)
**Arquivos afetados:** nenhum (revisão). Achados são reportados a Zeus, **não corrigidos** por este
agente.
**Ações:** revisar se o novo mecanismo de resolução introduz superfície de ataque — em especial:
expansão de variável em campo de comando executado por shell, possibilidade de a variável ser
controlada pelo repositório em vez do CLI, e se a mudança pode fazer o credential-guard **deixar de
executar** silenciosamente (falha aberta) em algum CLI.
**Critérios de aceite:**
- [ ] Parecer escrito cobrindo os 6 CLIs.
- [ ] Confirmação explícita de que nenhum caminho novo permite o guard falhar em silêncio.
- [ ] Nenhum arquivo modificado por este agente.

---

## Notas de execução

- **Autoridade de Git:** apenas Zeus cria branch, commita e faz push. Todo especialista entrega o
  trabalho **sem commit**.
- **Ordem obrigatória:** Wave 0 → Barreira B0 (ADR) → Wave 1 → Wave 2 → Waves 3–7 (encadeadas) →
  Wave 8. Nenhum paralelismo: todos os MLs de emissão editam os mesmos 3 arquivos e o gate de
  paridade exige que os 3 stacks mudem no mesmo commit.
- **Regra de paridade dos 3 CLIs do trackfw** (Go + Node.js + Python) vale para todos os MLs de
  código — nenhum ML é considerado concluído com apenas um stack alterado.
