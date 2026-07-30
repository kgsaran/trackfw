---
status: done
date: 2026-07-29
req: "REQ-2026-07-29-install-pula-artefato-desatualizado-em-vez-de-abortar"
squad: ""
---

# Roadmap: install pula artefato desatualizado em vez de abortar

> Created: 2026-07-29 | Status: done

## Contexto

REQ: `docs/req/REQ-2026-07-29-install-pula-artefato-desatualizado-em-vez-de-abortar.md`

`trackfw init --ai-tools gemini` aborta o scaffold de um projeto novo quando o harness global contém
um artefato trackfw desatualizado. Causa: o preflight de `mutationInstall` retorna erro para artefato
`outdated` + `owned`, e `mutate` é um lote atômico com rollback — o erro descarta a operação inteira.

**Escopo negativo explícito:** este roadmap **não** altera o escopo de instalação. As decisões D1 e D4
de `ADR-2026-07-25-escopo-de-instalacao-selecionavel-para-agents-e-skills.md` permanecem em vigor
(`init --ai-tools` sem TTY instala em **global**). A premissa original da REQ — de que `init` não
deveria alcançar o HOME — foi verificada e refutada; ver a seção "Premissa anterior invalidada" na
REQ. Nenhum ML abaixo toca `internal/commands/init.go`, `npm/src/commands/init.js` ou
`pypi/trackfw/commands/init.py` na resolução de escopo.

## Critérios de Aceite

- [x] `install` sobre `outdated` + `owned` sem `--force` pula o artefato, preserva bytes, aplica o
      resto do lote e retorna exit 0 — MLs 2A/2B/2C, testes de manager nos três runtimes.
- [x] `install` sobre `modified` continua erro sem `--force` — inalterado. Guardado por
      `TestManagerInstallOwnedModifiedRemainsError` (Go) e equivalentes.
- [x] `trackfw init --ai-tools <tool>` completa com exit 0 com artefato global desatualizado, provado
      em teste com HOME isolado nos três runtimes — cenários `e2e/init-outdated-global/{go,node,py}`,
      que também afirmam `trackfw.yaml` criado, bytes do artefato desatualizado preservados e artefato
      irmão gravado (skip ≠ abort).
- [x] Aviso em stderr, tilde-abreviado, comando de remediação correto por escopo — derivado de
      `plan.claim.scope` **por artefato** nos três, com teste de lote de escopo misto em cada runtime.
- [x] Strings de aviso byte-idênticas entre os três runtimes — cenários
      `skip-parity/{global,project}-scope/three-runtimes-identical`, com vacuity-guard.
- [x] `make quality` passa (exit 0, 19 cenários de falsificação, 12 gates não-vacuosos) e
      `bin/trackfw validate --json` retorna 0 violações e 0 warnings. Verificado pelo orquestrador.

## Mapa de dependências

```
Wave 1 — ML-1A (contrato, orquestrador)
   ↓ barrier — os três runtimes implementam contra o contrato congelado
Wave 2 — ML-2A (Go) ‖ ML-2B (Node.js) ‖ ML-2C (Python)   ← spawn simultâneo, arquivos disjuntos
   ↓ barrier — auditoria cruzada revelou divergência de forma → emenda do contrato
Wave 3 — ML-2D (Node.js) ‖ ML-2E (Python)                ← corretivo; Go já era canônico
   ↓ barrier — exige Waves 2 e 3 completas
Wave 4 — ML-3A (auditoria de paridade byte-a-byte + E2E)
```

Justificativa da barrier em ML-1A: o ML-6F do roadmap da barrier documentou empiricamente que um
contrato incompleto produz três respostas divergentes (Go=3, Node=19, Python=19 targets). O contrato
precede a implementação por lição medida, não por preferência. A Wave 3 existe porque o contrato do
ML-1A **ainda estava incompleto** — pinou nomes de parâmetros e não seus valores. A lição foi apenas
parcialmente aprendida na primeira tentativa.

**Nota de numeração:** os ids dos MLs (`ML-2D`, `ML-2E` na Wave 3; `ML-3A` na Wave 4) não seguem o
número da wave. Isso é deliberado: a Wave 3 foi acrescentada após a Wave 2 já ter sido executada e
commitada, e renumerar os ids quebraria a rastreabilidade das mensagens de commit já publicadas. A
wave corretiva nasceu como "Wave 2-bis", nomenclatura que o `trackfw barrier` **rejeita** — ele exige
número inteiro na heading (`malformed wave heading: "2-bis" is not a valid wave number`). Renumerada
para inteiro; os ids ficaram.

---

## Wave 1 — Congelar o contrato (1 ML)
> Dependências: nenhuma

### ML-1A — Fixar o contrato de skip de artefato desatualizado
**Status:** ✅ Concluído (contrato autorado pelo orquestrador)
**Arquivos afetados:**
- `docs/cli-parity.md` — nova seção `## install sobre artefato gerenciado desatualizado — skip, não erro fatal`

**Divisão de autoridade:** contrato de autoria exclusiva do `trackfw_architect`, como no ML-6A do
roadmap da barrier. Os MLs 2A/2B/2C implementam contra ele e não o alteram.

**Contrato congelado:**
1. `outdated` + `owned` + sem `--force` → **skip**: bytes preservados, lote continua, exit **0**.
2. `modified` → **erro** sem `--force`, inalterado. Não simetrizar os dois casos.
3. `outdated` + **não** `owned` → adoção, inalterado.
4. Observador de skip é a **única** superfície do sinal: `Manager.OnSkip func(destination, reason string)`
   (Go), `new IntegrationManager(dirs, { onSkip })` (Node), `IntegrationManager(root, on_skip=None)`
   (Python). Ausente → no-op. Chamado uma vez por artefato pulado, na ordem de `resolved`.
5. O retorno existente de `mutate` no Node (`this.inspect(plans)`) **não** carrega skips.
6. Aviso em **stderr**, uma linha por skip, caminho **tilde-abreviado**, remediação por escopo:
   `update harness` para global, `update` para projeto.

**Critérios de aceite:**
- [x] O contrato distingue `outdated`+`owned`, `outdated`+não-owned e `modified`.
- [x] A superfície do sinal é única e idêntica nos três runtimes.
- [x] As strings de aviso estão pinadas literalmente, com regra de escopo.
- [x] O escopo negativo (D1/D4 preservados) está registrado.

---

## Wave 2 — Implementar nos três runtimes (3 MLs em paralelo)
> Dependências: ML-1A completo. Os três MLs tocam arquivos disjuntos — **spawn simultâneo**.

**Gates da wave (cada ML roda o seu):** ver "Comandos de validação" por ML.

### ML-2A — Implementar o skip no Go
**Status:** ✅ Concluído
**Agente:** Apolo
**Arquivos afetados:**
- `internal/integrations/manager.go` — `preflight` (linha ~219), `mutate` (linha ~102), struct `Manager`
- `internal/commands/init.go` — apenas ligar `OnSkip` ao writer de stderr em `installAITools` (~linha 430)
- `internal/commands/integrations_flags.go` — apenas ligar `OnSkip` (~linha 213)
- `internal/integrations/manager_test.go` — novo teste

**Ações:**
1. Adicionar campo `OnSkip func(destination, reason string)` a `integrations.Manager`.
2. Alterar `preflight` para sinalizar skip em vez de erro no caso `StateOutdated && owned && !force`
   de `mutationInstall`. Sugestão de assinatura: `preflight(...) (skip bool, err error)`.
   O caso `StateModified` na linha ~217 permanece erro — **não alterar**.
3. Em `mutate`, filtrar os itens pulados de `resolved` **antes** das fases de snapshot e
   `applyMutation`, de modo que o artefato pulado não entre no rollback nem no manifest write.
   Invocar `m.OnSkip` (nil-safe) uma vez por item pulado, na ordem de `resolved`.
4. Ligar `OnSkip` nos dois callers de `Install` acima, imprimindo em **stderr** a string pinada no
   contrato, com caminho tilde-abreviado. Reutilizar o helper de tilde já existente do `update`
   (procurar por `tildeify` ou equivalente em `internal/generators/update.go`) — **não** duplicar.
5. Novo teste em `manager_test.go`: instalar artefato, adulterar para bytes de template anterior
   declarado em `legacy_hashes`, garantir `owned` via manifest, chamar `Install` e afirmar
   (a) sem erro, (b) bytes preservados, (c) `OnSkip` chamado exatamente uma vez com caminho
   tilde-abreviado, (d) um segundo artefato no mesmo lote foi aplicado normalmente.

**Critérios de aceite:**
- [x] `install` em `outdated`+`owned` não retorna erro e preserva os bytes.
- [x] Demais itens do lote são aplicados; artefato pulado ausente do manifest write.
- [x] `modified` continua erro sem `--force` (teste existente ou novo comprova).
- [x] `OnSkip` nil não causa panic.
- [x] Aviso em stderr, byte-idêntico ao contrato, caminho tilde-abreviado.
- [x] `go build ./...`, `go test ./...` e `go vet ./...` passam.

**Comandos de validação:**
```bash
go build ./... && go test ./... && go vet ./...
```

### ML-2B — Implementar o skip no Node.js
**Status:** ✅ Concluído
**Agente:** Apolo
**Arquivos afetados:**
- `npm/src/integrations/manager.js` — `preflight` (linha ~149), `mutate` (linha ~125), construtor
- `npm/src/integrations/index.js` — `execute()` repassa `options.onSkip` ao construtor do manager
- `npm/src/generators/init.js` — `installIntegrationTarget` aceita e repassa `{ onSkip }`
- `npm/src/commands/init.js` — ligar `onSkip` (modos TTY e não-TTY)
- `npm/src/commands/integrations.js` — ligar `onSkip`
- `npm/tests/agents-skills.test.js` — **inverter** a asserção da linha 193

**Ações:**
1. Aceitar `onSkip` no segundo parâmetro de opções do construtor de `IntegrationManager`.
2. Em `preflight`, o caso `status.state === 'outdated' && owned && !force` de `install`
   (linha 156) deixa de lançar e passa a sinalizar skip. A linha 155 (`modified`) permanece
   lançando — **não alterar**.
3. Em `mutate`, filtrar os pulados antes de `snapshot`/`apply`, invocar `onSkip` (guardado contra
   `undefined`) uma vez por item, na ordem de `resolved`.
4. **O retorno `this.inspect(plans)` da linha 146 não deve carregar o sinal de skip** — o contrato
   pina o callback como única superfície, para não divergir de Go e Python.
5. **Reescrever `npm/tests/agents-skills.test.js:193`.** A linha atual é
   `assert.throws(() => manager.install([plan]), /outdated.*update/i)` e codifica o contrato
   antigo. Substituir por asserção de que `install` **não** lança, os bytes são preservados e
   `onSkip` foi observado uma vez. Manter intacto o resto do teste (linhas 181–192), que valida
   `current`/não-owned e a adoção de legacy.
6. Ligar `onSkip` nos callers, imprimindo em stderr a string pinada, com tilde. Reutilizar o
   `tildeify` existente em `npm/src/lib/update-engine.js` — **não** duplicar (ele já foi corrigido
   para `$HOME` com barra dupla no ML-6H; reimplementar reintroduz o bug).

**Critérios de aceite:**
- [x] Comportamento, estados e exit codes equivalentes ao Go.
- [x] Linha 193 do teste invertida; demais asserções do teste preservadas.
- [x] `onSkip` ausente não causa erro.
- [x] Aviso em stderr byte-idêntico ao Go.
- [x] `cd npm && npm test` passa.

**Comandos de validação:**
```bash
cd npm && npm test
```

### ML-2C — Implementar o skip no Python
**Status:** ✅ Concluído
**Agente:** Apolo
**Arquivos afetados:**
- `pypi/trackfw/integrations/manager.py` — `preflight` (linha ~213), `mutate`, `__init__`
- `pypi/trackfw/commands/init.py` — apenas ligar `on_skip`
- `pypi/trackfw/integrations/command.py` — apenas ligar `on_skip`
- `pypi/tests/test_agents_skills.py` — novo teste

**Ações:**
1. Aceitar `on_skip=None` em `IntegrationManager.__init__`.
2. Em `preflight`, o caso `outdated` + `owned` + sem `force` de `install` (linha ~213) deixa de
   levantar `IntegrationError` e passa a sinalizar skip. O caso `modified` permanece levantando —
   **não alterar**.
3. Em `mutate`, filtrar os pulados antes de snapshot/apply, invocar `on_skip` (guardado contra
   `None`) uma vez por item, na ordem de `resolved`.
4. Novo teste em `test_agents_skills.py`, espelhando o do ML-2A. **Não alterar** o teste da linha
   232–243, que exercita a adoção de legacy (não-owned) e permanece válido.
5. Ligar `on_skip` nos callers, imprimindo em stderr a string pinada, com tilde.

**Critérios de aceite:**
- [x] Comportamento, estados e exit codes equivalentes ao Go e Node.
- [x] Teste de adoção de legacy (linhas 232–243) permanece verde e inalterado.
- [x] `on_skip=None` não causa erro.
- [x] Aviso em stderr byte-idêntico ao Go e Node.
- [x] Suíte Python passa. (693 passed)

**Comandos de validação:**
```bash
cd pypi && python -m pytest
```

---

## Wave 3 — Convergir a semântica do observador (2 MLs em paralelo, corretivo)
> Dependências: ML-2A, ML-2B e ML-2C concluídos. Emenda do contrato feita (ML-1A-bis).
> Arquivos disjuntos entre Node.js e Python — **spawn simultâneo**.

**Origem:** auditoria cruzada da Wave 2 pelo orquestrador. O contrato do ML-1A pinou os **nomes** dos
parâmetros do observador e a string de aviso literal, mas deixou os **valores** dos parâmetros e a
origem da remediação à interpretação. Falha do contrato, não dos implementadores — os três reportaram
seus desvios honestamente.

**Enquadramento honesto:** as strings em stderr saem **byte-idênticas hoje** nos três runtimes. Isto
não é regressão visível ao usuário. O que diverge é a forma interna e a robustez da derivação de
escopo. O trabalho é endurecimento preventivo mais um bug latente de escopo misto.

**Divergências medidas no código (não em relatórios):**
1. Valor de `reason`: linha de aviso completa (Go) · `'outdated+owned'` (Node) · `"outdated"` (Python).
2. Valor de `destination`: tilde-abreviado (Go, Python) · caminho absoluto (Node).
3. Quem compõe a linha: o manager (Go) · o caller (Node, Python).
4. Origem da remediação: `Claim.Scope` por artefato (Go) · `tilde.startsWith('~/')` (Node) · closure
   sobre o escopo de nível de comando (Python). As duas últimas acertam só com lote de escopo
   uniforme — corretas por acidente, não por construção.
5. Node compõe em **dois sites do mesmo runtime** (`init.js` e `integrations.js`), que podem divergir
   entre si sem nenhum teste de paridade entre runtimes perceber.

**Resolução (pinada em `docs/cli-parity.md`):** Go é a implementação canônica. O manager compõe a
linha completa; `destination` é o caminho tilde-abreviado; `reason` é a linha pronta para impressão;
callers imprimem verbatim e não compõem, abreviam nem derivam remediação; a remediação vem de
`plan.claim.scope` por artefato.

### ML-2D — Convergir o Node.js para a forma canônica
**Status:** ✅ Concluído (status marcado pelo orquestrador — ver nota de rastreabilidade)
**Agente:** Apolo
**Arquivos afetados:** `npm/src/integrations/manager.js`, `npm/src/commands/init.js`,
`npm/src/commands/integrations.js`, `npm/tests/agents-skills.test.js`

**Critérios de aceite:**
- [x] Manager compõe a linha; `onSkip(destinoTilde, linhaCompleta)` — `manager.js:146-149`.
- [x] Nenhum caller compõe, abrevia ou deriva remediação — `init.js:284` e `integrations.js:190`
      recebem `(_destination, reason)` e apenas escrevem em stderr; imports de `tildeify` removidos
      dos callers.
- [x] Remediação derivada de `plan.claim.scope` — `manager.js:147`. A inferência
      `tilde.startsWith('~/')` foi eliminada.
- [x] Strings de **formato** idênticas nos três runtimes, auditadas no código pelo orquestrador em
      941ac15: `warning: skipping outdated artifact %s; run '%s' to refresh it`.
- [x] Byte-identidade da saída **executada** dos três CLIs — provada pelo ML-3A nos cenários
      `skip-parity/global-scope/three-runtimes-identical` e `skip-parity/project-scope/three-runtimes-identical`
      de `scripts/check-update-parity.sh`, ambos com vacuity-guard, encadeados em `make quality`.
- [x] `npm test` → **329 passed, 0 failed** (verificado pelo orquestrador).

**Nota de rastreabilidade — desvio de processo:** os agentes do ML-2D e do ML-2E morreram por limite
de sessão de API. O agente do ML-2D commitou com os arquivos do ML-2E já staged pelo agente paralelo,
de modo que **d737b15 contém os dois MLs** embora sua mensagem descreva apenas o Node.js. O código dos
dois está presente e auditado; o defeito é de rastreabilidade, não de conteúdo. Causa: dois agentes
paralelos compartilhando o index do Git. `git add <caminhos>` explícito por ML não é suficiente quando
outro agente já fez staging — o correto seria `git commit -- <caminhos>` ou worktrees isoladas.

### ML-2E — Convergir o Python para a forma canônica
**Status:** ✅ Concluído
**Agente:** Apolo
**Arquivos afetados:** `pypi/trackfw/integrations/manager.py`, `pypi/trackfw/commands/init.py`,
`pypi/trackfw/integrations/command.py`, `pypi/tests/test_agents_skills.py`

**Critérios de aceite:**
- [x] Manager compõe a linha; `on_skip(destino_tilde, linha_completa)`.
- [x] Nenhum caller compõe — closures de `init.py` e `command.py` reduzidas a imprimir `reason`.
- [x] Remediação derivada de `plan["claim"]["scope"]` por artefato, não da closure de comando.
- [x] Strings de **formato** idênticas nos três, auditadas no código pelo orquestrador em 941ac15.
- [x] Byte-identidade da saída **executada** dos três CLIs — provada pelo ML-3A nos cenários
      `skip-parity/*/three-runtimes-identical`. Histórico: o agente do ML-2E marcou este critério
      antes de morrer por limite de sessão, com referência circular a um ML que não havia rodado; foi
      desmarcado pelo orquestrador e a barrier da Wave 3 retornou `blocked` até a evidência existir.
- [x] Suíte Python passa — **694 passed** (verificado pelo orquestrador).
## Wave 4 — Auditoria de paridade (1 ML)
> Dependências: **barrier** — Waves 2 e 3 completas (ML-2A a ML-2E).

### ML-3A — Auditar paridade e provar o cenário de ponta a ponta
**Status:** ✅ Concluído
**Agente:** Artemis
**Arquivos afetados:**
- `internal/integrations/manager_test.go` — novo `TestManagerInstallSkipMixedScopeBatch`
- `scripts/check-update-parity.sh` — cenários 6 (global-scope), 7 (project-scope), 8 (E2E init)

**Ações:**
1. Comparar as strings de aviso dos três runtimes **byte-a-byte** para o mesmo HOME e projeto, nos
   dois escopos (global e projeto). Divergência é violação, não detalhe cosmético — o ML-6F mediu
   que a divergência dos runtimes se deu exatamente em renderização (`~/...` vs absoluto), não em
   lógica.
2. Teste de ponta a ponta com **HOME isolado**: preparar um artefato global desatualizado e owned,
   executar `init --ai-tools` num projeto novo e afirmar exit **0** + scaffold completo, nos três
   runtimes.
3. Confirmar que nenhum ML da Wave 2 alterou a resolução de escopo de `init` — D1/D4 intactos.

**Critérios de aceite:**
- [x] Avisos byte-idênticos nos três runtimes, nos dois escopos.
  - Evidência: `skip-parity/global-scope/three-runtimes-identical` e `skip-parity/project-scope/three-runtimes-identical` — OK em `make quality`.
- [x] `init --ai-tools` com artefato global desatualizado → exit 0 e scaffold completo, nos três.
  - Evidência: cenários `e2e/init-outdated-global/go`, `/node`, `/py` — todos OK.
- [x] `modified` continua erro nos três.
  - Evidência: `TestManagerInstallOwnedModifiedRemainsError` (Go, pré-existente), Node.js test line 195+, Python test line 397+ — todos verdes.
- [x] Nenhuma mudança na resolução de escopo de `init` (D1/D4 preservados).
  - Evidência: `git diff origin/main..HEAD -- internal/commands/init.go` mostra apenas adição de `OnSkip`; cenário 8 confirma instalação em `$HOME/.gemini/...` (global).
- [x] `make quality` passa e `bin/trackfw validate --json` retorna 0 violações.
  - Evidência: `make quality` exit 0, 19 cenários de falsificação; `bin/trackfw validate --json` → 0 violações.
- [x] Go ganha teste de lote de escopo misto (`TestManagerInstallSkipMixedScopeBatch`).
  - Evidência: `go test ./internal/integrations/ -run TestManagerInstallSkipMixedScopeBatch` → PASS.

**Nota sobre Lacuna 3 (project scope — macOS):** Node.js e Python resolvem `process.cwd()` via
`/private/var/...` (symlink macOS), diferente do `filepath.Abs` do Go que retorna `/var/...`. Para
o escopo de projeto, cada runtime usa seu próprio manifesto criado por si mesmo — os avisos em stderr
são byte-idênticos entre os três porque a string usa o caminho relativo (`.claude/agents/...`), não
o caminho absoluto da chave do manifesto.

**Comandos de validação:**
```bash
make quality
bin/trackfw validate --json
```

---

