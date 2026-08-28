---
status: wip
date: 2026-08-28
req: "REQ-2026-08-28-gate-de-ci-gerado-instala-versao-nao-pinada-do-trackfw-e-nao-ha-como-pinar.md"
squad: "hades-tf, ares-tf, apolo-tf, artemis-tf"
---

# Roadmap: Gate de CI pinado na versão geradora e `install.sh` honrando `TRACKFW_VERSION`

> Created: 2026-08-28 | Status: wip

## Context

REQ: `REQ-2026-08-28-gate-de-ci-gerado-instala-versao-nao-pinada-do-trackfw-e-nao-ha-como-pinar.md`
ADR: `ADR-2026-08-28-gate-de-ci-gerado-nasce-pinado-na-versao-que-o-gerou-e-o-install-sh-honra-trackfw-version.md`

`scripts/install.sh:33-44` resolve a versão via API de `releases/latest`, ignorando de qual tag foi
baixado, e não aceita versão por nenhum meio. O workflow gerado nos 3 CLIs usa exatamente esse
script, então o gate bloqueante de PR não é reprodutível e ninguém consegue pinar. Duas frentes:
o script passa a honrar `TRACKFW_VERSION` (com validação, porque o valor entra numa URL), e os
templates gerados nascem pinados na versão do binário gerador.

## Acceptance Criteria

Consolidado — AC1 a AC15 da REQ. Detalhe por ML abaixo.

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Modelo de ameaça deste roadmap
**Status:** ⬜ Pendente
**Agente:** `hades-tf`
**Files affected:** apenas este roadmap (seção de resultado abaixo do ML). Nenhum arquivo de produto.
**Actions:**
1. **Completude de enumeração.** A lista de superfícies abaixo está fechada? Não se limite aos
   arquivos nomeados pela REQ: faça `grep -rn "releases/latest" . --exclude-dir=.git
   --exclude-dir=node_modules` e `grep -rn "install.sh" .` nas três árvores (`internal/`,
   `npm/src/`, `pypi/trackfw/`) **e** em `docs/`, `scripts/`, `.github/`. Superfícies já conhecidas:
   `scripts/install.sh`; `internal/generators/scaffold.go:1908` (GH Actions) e `:1932` (GitLab);
   `npm/src/generators/init.js` (7 ocorrências: 227, 242, 800, 812, 824, 836, 851);
   `pypi/trackfw/generators/init_gen.py:541,571`;
   `npm/src/integrations/scaffold_doctor.js` e `pypi/trackfw/integrations/scaffold_doctor.py`
   (comparação com template). Reporte o que faltou ou demonstre que a lista fecha.
2. **Modelo de ameaça.** `TRACKFW_VERSION` é interpolada em `URL=".../releases/download/${VERSION}/
   ${FILENAME}"` e depois passada a `curl`/`tar` num script `sh` executado em CI. Quem esvazia a
   validação de AC3/AC4 sem quebrar nenhuma regra escrita? Cubra no mínimo: substituição de comando,
   separador de shell, path traversal no nome do asset, newline embutida, valor com espaços, valor
   que passa no regex mas aponta para release inexistente (falha aberta ou fechada?), e o caso de
   `TRACKFW_VERSION` vinda de `github.event.pull_request` num workflow de terceiro.
3. **Alvos de falsificação nas duas direções.** Para cada superfície: o que quebra se o
   comportamento regredir (volta a não pinar / validação some), **e** o que quebra se regredir para
   o lado oposto (validação estrita demais rejeita `v7.3.0`; pin obrigatório impede resolver
   `latest`; `update` deixa de bumpar o pin e congela o projeto).
4. **Residual declarado.** O que este desenho aceita não cobrir. Inclua, no mínimo: a lacuna do
   alvo `ci-workflow` no `update` do Python; o pin que envelhece em silêncio; e o `install.sh`
   publicado numa release antiga que não conhece a variável.
**Acceptance criteria:**
- [ ] As quatro seções respondidas com evidência (comando rodado + saída), não asserção de uma linha
- [ ] Nenhuma linha de implementação escrita neste ML
- [ ] Se a enumeração encontrar superfície fora da lista, o roadmap é atualizado antes da Wave 1

**Gates da wave:**
```bash
# Wave 0 gate: a enumeração declarada tem que bater com a busca real.
# Falha fechada enquanto ML-0A não tiver registrado a contagem medida abaixo.
grep -rn "releases/latest" scripts/ internal/ npm/src/ pypi/trackfw/ 2>/dev/null | wc -l
# ML-0A substitui esta linha por uma asserção sobre o número medido.
exit 1  # placeholder — ML-0A troca o comando, não remove o gate
```

## Wave 1 — `install.sh` honra e valida `TRACKFW_VERSION`
> Dependências: Wave 0 aprovada. ML único — arquivo único, sem paralelismo possível.

### ML-1A — `TRACKFW_VERSION` no `install.sh`, com validação anterior ao uso
**Status:** ⬜ Pendente
**Agente:** `ares-tf`
**Files affected:**
- `scripts/install.sh` (único arquivo de produto)
- `scripts/check-install-version-pin.sh` (novo gate)
- `Makefile` (registrar o gate em `quality`)

**Actions:**
1. Em `scripts/install.sh`, **antes** do bloco de resolução via API (linha 32), inserir: se
   `TRACKFW_VERSION` está definida e não é vazia após remover espaços, validar contra
   `^v\{0,1\}[0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*$` usando `case`/`expr` compatível com `sh`
   POSIX (o script roda com `sh`, não `bash` — não usar `[[ =~ ]]`). Valor válido → `VERSION` recebe
   o valor com prefixo `v` normalizado (`v7.3.0`), pulando a consulta à API. Valor inválido →
   `echo` nomeando a variável e o formato esperado em stderr e `exit 1`, **sem** compor URL nem
   chamar `curl`/`wget`.
2. Variável ausente ou vazia → fluxo atual intocado (resolução via API). AC2.
3. Não adicionar argumento posicional nem flag. AC do escopo negativo.
4. Criar `scripts/check-install-version-pin.sh` como gate falsificável, no molde dos gates
   existentes (`scripts/check-doctor-parity.sh`): cenários que **passam** — `7.3.0`, `v7.3.0`,
   vazio, ausente; cenários que **falham com a razão declarada** (`assert_fails_with`) —
   `7.3.0; rm -rf /`, `../../etc`, `$(id)`, `` `id` ``, `7.3.0 && curl x | sh`, `v7.3.0` com
   newline embutida, `"   "`. O gate deve exercitar o script real com uma seam que impeça download
   de verdade (ex.: `TRACKFW_INSTALL_DRYRUN=1` imprimindo a URL composta e saindo antes do `curl`),
   e asserir sobre a URL impressa. Incluir **guarda de vacuidade**: se nenhum cenário rodou, o gate
   falha.
5. Registrar o gate no alvo `quality` do `Makefile`.

**Acceptance criteria:**
- [ ] AC1, AC2, AC3, AC4, AC5 da REQ verificáveis pelo gate novo
- [ ] `sh -n scripts/install.sh` → exit 0 (sintaxe POSIX válida)
- [ ] `bash scripts/check-install-version-pin.sh` → exit 0, com contagem de cenários impressa
- [ ] Guarda de vacuidade provada: rodar o gate com a lista de cenários vazia faz o gate falhar
- [ ] Nenhum download real disparado durante o gate
**Comandos de validação:**
```bash
sh -n scripts/install.sh
bash scripts/check-install-version-pin.sh
TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 make quality
```

## Wave 2 — Templates pinados nos 3 CLIs (3 MLs em paralelo)
> Dependências: Wave 1 concluída (o pin só faz sentido com o `install.sh` honrando a variável).
> Os três MLs tocam árvores disjuntas e rodam em paralelo. Nenhum deles toca `docs/cli-parity.md`
> nem `scripts/` — isso é a Wave 3, sequencial, para não haver dois agentes no mesmo arquivo.

### ML-2A — Go: template pinado + doctor + comentário corrigido
**Status:** ⬜ Pendente
**Agente:** `apolo-tf`
**Files affected:** `internal/generators/scaffold.go`, `internal/generators/scaffold_doctor.go`,
`internal/generators/scaffold_test.go` (ou arquivo de teste equivalente já existente)
**Actions:**
1. `buildGitHubActionsWorkflowContent` passa a receber a versão e emitir, no job `governance`:
   ```yaml
   jobs:
     governance:
       runs-on: ubuntu-latest
       timeout-minutes: 10
       env:
         TRACKFW_VERSION: "<versão>"
   ```
   A versão vem de `internal/version.Version`. **Não** hardcodar `7.3.0`.
2. `buildGitLabCIWorkflowContent` idem, via bloco `variables:` com `TRACKFW_VERSION`.
3. `scaffold_doctor.go` continua chamando os mesmos builders (a comparação segue coerente por
   construção). Corrigir o comentário de `:62` e o de `buildGitHubActionsWorkflowContent`: o builder
   é cfg-independente mas **não** é version-independente (AC12).
4. Testes Go: workflow gerado contém a versão que `version.Version` reporta; `doctor` reporta
   `no mismatches` logo após gerar (AC11) e `scaffold-divergent` quando o pin é trocado à mão (AC10).
**Acceptance criteria:**
- [ ] AC6, AC7 (Go), AC10, AC11, AC12
- [ ] `go build ./...` → exit 0
- [ ] `go test ./internal/generators/...` → exit 0
- [ ] Nenhuma string de versão literal no template — grep por `7\.3\.0` em `scaffold.go` não retorna
      nada no bloco do workflow

### ML-2B — Node: template pinado + doctor
**Status:** ⬜ Pendente
**Agente:** `apolo-tf`
**Files affected:** `npm/src/generators/init.js`, `npm/src/integrations/scaffold_doctor.js`,
`npm/src/commands/update.js` (só se o alvo `ci-workflow` precisar da versão), `npm/tests/`
**Actions:**
1. Mesmo template do ML-2A, **byte-idêntico** para a mesma versão. A versão vem do `version` do
   `npm/package.json`, não literal.
2. Cobrir as 7 ocorrências de `releases/latest` em `init.js` (227, 242, 800, 812, 824, 836, 851):
   as que compõem o workflow gerado passam a pinar; as que aparecem em texto de CLAUDE.md/docs
   seguem AC13 — atualizar ou declarar explicitamente fora do pin, sem deixar instrução
   contraditória.
3. `scaffold_doctor.js` compara contra o template novo.
4. Testes Node cobrindo AC6, AC10, AC11.
**Acceptance criteria:**
- [ ] AC6, AC7 (Node), AC10, AC11, AC13 (parte Node)
- [ ] `npm test --prefix npm` → exit 0
- [ ] Nenhuma versão literal no template

### ML-2C — Python: template pinado + doctor
**Status:** ⬜ Pendente
**Agente:** `apolo-tf`
**Files affected:** `pypi/trackfw/generators/init_gen.py`,
`pypi/trackfw/integrations/scaffold_doctor.py`, `pypi/tests/`
**Actions:**
1. Mesmo template, byte-idêntico. Versão vem de `trackfw.__version__`, não literal.
2. Cobrir `init_gen.py:541` e `:571` (AC13, parte Python).
3. `scaffold_doctor.py` compara contra o template novo. **Manter** a exclusão documentada do alvo
   `ci-workflow` no `update` do Python (`scaffold_doctor.py:25`) — está no escopo negativo — mas
   revisar se o texto da exclusão continua correto depois desta mudança.
4. Testes Python cobrindo AC6, AC10, AC11.
**Acceptance criteria:**
- [ ] AC6, AC7 (Python), AC10, AC11, AC13 (parte Python)
- [ ] `python -m pytest pypi/tests` → exit 0
- [ ] Nenhuma versão literal no template

## Wave 3 — Gate de paridade, contrato e evidência
> Dependências: Wave 2 completa nos três. ML único — toca arquivos compartilhados pelos 3 stacks.

### ML-3A — Gate falsificável de paridade do pin + `docs/cli-parity.md`
**Status:** ⬜ Pendente
**Agente:** `artemis-tf`
**Files affected:** `scripts/check-ci-workflow-pin-parity.sh` (novo), `docs/cli-parity.md`,
`Makefile`
**Actions:**
1. Gate que gera o workflow com os 3 CLIs em sandbox e compara **byte a byte** (AC8). Falha se
   qualquer par divergir, nomeando qual.
2. Cenário de falsificação em cada direção: workflow sem `TRACKFW_VERSION` → gate falha; workflow
   com versão diferente da do binário → gate falha; `timeout-minutes` ausente → gate falha.
   Usar `assert_fails_with` mirando a razão que o **próprio gate** emite, não a mensagem do CLI.
3. Guarda de vacuidade obrigatória.
4. Seção nova em `docs/cli-parity.md` com o contrato do pin, anotada com `gate=` apontando para o
   script novo, mais a lacuna do `ci-workflow` no Python anotada como `gap reason=`.
5. Registrar no `Makefile`.
**Acceptance criteria:**
- [ ] AC8, AC14
- [ ] `bash scripts/check-ci-workflow-pin-parity.sh` → exit 0 com contagem de cenários
- [ ] `bash scripts/check-parity-contract-coverage.sh` → exit 0
- [ ] AC15: `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 make quality` → exit 0
**Comandos de validação:**
```bash
bash scripts/check-ci-workflow-pin-parity.sh
bash scripts/check-parity-contract-coverage.sh
TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 make quality
```

## Barreira final
Antes do PR: revisão `hefesto-tf` (qualidade) e `hades-tf` (segurança — a validação de AC3/AC4 é o
ponto de maior risco do roadmap), auditoria de diff pelo arquiteto, e
`trackfw barrier <roadmap> --wave 3`.
