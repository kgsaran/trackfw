---
status: wip
date: 2026-07-29
req: "REQ-2026-07-29-install-pula-artefato-desatualizado-em-vez-de-abortar"
squad: ""
---

# Roadmap: install pula artefato desatualizado em vez de abortar

> Created: 2026-07-29 | Status: wip

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

- [ ] `install` sobre `outdated` + `owned` sem `--force` pula o artefato, preserva bytes, aplica o
      resto do lote e retorna exit 0.
- [ ] `install` sobre `modified` continua erro sem `--force` — inalterado.
- [ ] `trackfw init --ai-tools <tool>` completa com exit 0 com artefato global desatualizado, provado
      em teste com HOME isolado nos três runtimes.
- [ ] Aviso em stderr, tilde-abreviado, comando de remediação correto por escopo.
- [ ] Strings de aviso byte-idênticas entre os três runtimes.
- [ ] `make quality` passa e `bin/trackfw validate --json` retorna 0 violações.

## Mapa de dependências

```
ML-1A (contrato, orquestrador)
   ↓ barrier — os três runtimes implementam contra o contrato congelado
ML-2A (Go) ‖ ML-2B (Node.js) ‖ ML-2C (Python)     ← spawn simultâneo, arquivos disjuntos
   ↓ barrier — exige os três concluídos
ML-3A (auditoria de paridade byte-a-byte)
```

Justificativa da barrier em ML-1A: o ML-6F do roadmap da barrier documentou empiricamente que um
contrato incompleto produz três respostas divergentes (Go=3, Node=19, Python=19 targets). O contrato
precede a implementação por lição medida, não por preferência.

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
**Status:** ⬜ Pendente
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
- [ ] `install` em `outdated`+`owned` não retorna erro e preserva os bytes.
- [ ] Demais itens do lote são aplicados; artefato pulado ausente do manifest write.
- [ ] `modified` continua erro sem `--force` (teste existente ou novo comprova).
- [ ] `OnSkip` nil não causa panic.
- [ ] Aviso em stderr, byte-idêntico ao contrato, caminho tilde-abreviado.
- [ ] `go build ./...`, `go test ./...` e `go vet ./...` passam.

**Comandos de validação:**
```bash
go build ./... && go test ./... && go vet ./...
```

### ML-2B — Implementar o skip no Node.js
**Status:** ⬜ Pendente
**Agente:** Apolo
**Arquivos afetados:**
- `npm/src/integrations/manager.js` — `preflight` (linha ~149), `mutate` (linha ~125), construtor
- `npm/src/commands/init.js` — apenas ligar `onSkip`
- `npm/src/commands/integrations.js` — apenas ligar `onSkip`
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
- [ ] Comportamento, estados e exit codes equivalentes ao Go.
- [ ] Linha 193 do teste invertida; demais asserções do teste preservadas.
- [ ] `onSkip` ausente não causa erro.
- [ ] Aviso em stderr byte-idêntico ao Go.
- [ ] `cd npm && npm test` passa.

**Comandos de validação:**
```bash
cd npm && npm test
```

### ML-2C — Implementar o skip no Python
**Status:** ⬜ Pendente
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
- [ ] Comportamento, estados e exit codes equivalentes ao Go e Node.
- [ ] Teste de adoção de legacy (linhas 232–243) permanece verde e inalterado.
- [ ] `on_skip=None` não causa erro.
- [ ] Aviso em stderr byte-idêntico ao Go e Node.
- [ ] Suíte Python passa.

**Comandos de validação:**
```bash
cd pypi && python -m pytest
```

---

## Wave 3 — Auditoria de paridade (1 ML)
> Dependências: **barrier** — ML-2A, ML-2B e ML-2C todos concluídos.

### ML-3A — Auditar paridade e provar o cenário de ponta a ponta
**Status:** ⬜ Pendente
**Agente:** Artemis
**Arquivos afetados:**
- testes de paridade dos três runtimes
- `docs/cli-parity.md` — apenas se uma divergência exigir emenda ao contrato

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
- [ ] Avisos byte-idênticos nos três runtimes, nos dois escopos.
- [ ] `init --ai-tools` com artefato global desatualizado → exit 0 e scaffold completo, nos três.
- [ ] `modified` continua erro nos três.
- [ ] Nenhuma mudança na resolução de escopo de `init` (D1/D4 preservados).
- [ ] `make quality` passa e `bin/trackfw validate --json` retorna 0 violações.

**Comandos de validação:**
```bash
make quality
bin/trackfw validate --json
```
