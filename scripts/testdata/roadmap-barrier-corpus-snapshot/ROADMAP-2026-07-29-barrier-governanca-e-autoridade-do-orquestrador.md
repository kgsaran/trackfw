---
status: done
date: 2026-07-29
req: "REQ-2026-07-29-barrier-governanca-e-autoridade-do-orquestrador"
squad: ""
---

# Roadmap: Barrier de governança e autoridade exclusiva do orquestrador

> Criado em: 2026-07-29 | Status: done
REQ: `docs/req/REQ-2026-07-29-barrier-governanca-e-autoridade-do-orquestrador.md`
ADR: `docs/adr/ADR-2026-07-29-barrier-governanca-e-autoridade-do-orquestrador.md`
Branch: `feat/barrier-governanca-e-autoridade-do-orquestrador`

## Diagnóstico / Contexto

O agente arquiteto já menciona barrier entre waves e auditoria pós-ML, mas não existe uma operação
do produto que reúna os checks, registre evidências e impeça o avanço quando uma etapa falha. O
contrato também precisa refletir a política de segurança operacional: agentes especialistas não
executam Git; somente o `trackfw_architect` possui autoridade para branch, commit e push.

O comando nativo será agnóstico de stack. Build, testes, E2E e gates são comandos declarados pelo
projeto ou pelo roadmap. A invocação dos agentes `code-quality` e `security` pertence ao slash
command e ao fluxo do orquestrador, não ao binário autônomo.

## Definição de pronto da barrier

Uma wave só pode ser liberada quando todos os itens abaixo estiverem verdes:

1. Todos os MLs da wave estão presentes e marcados como concluídos.
2. Testes unitários e E2E aplicáveis foram executados.
3. Build aplicável foi executado sem erros.
4. Cada critério de aceite foi inspecionado e possui evidência.
5. O agente de qualidade de código reportou conformidade, performance, robustez e clareza.
6. O agente de segurança reportou a análise aplicável de SAST, privilégios, controle de acesso e
   demais camadas relevantes.
7. Todos os gates pré-commit declarados pelo projeto passaram.
8. `trackfw validate --json` passou.
9. O diff foi auditado contra o escopo e não contém alterações de agentes concorrentes ou arquivos
   proibidos.
10. O resultado foi registrado antes de liberar a próxima wave.

## Critérios de Aceite

Este roadmap só é considerado concluído quando todos os itens abaixo forem verdadeiros:

- [x] `trackfw barrier <roadmap> --wave <n>` existe e é funcionalmente equivalente nos três CLIs
      (Go, Node.js e Python), com paridade de flags, estados, exit codes e saída JSON.
- [x] A barrier bloqueia a liberação de wave quando qualquer item da "Definição de pronto" falha,
      retornando exit code não-zero e `status: blocked`.
- [x] Uma wave integralmente verde retorna exit code 0 e `status: passed`.
- [x] O CLI executa somente gates declarados pelo projeto; nenhuma regra específica do trackfw é
      tratada como universal (sem paridade hardcoded).
- [x] O slash command `/trackfw:barrier` existe e contém o checklist operacional completo da REQ.
- [x] Nenhum agente especialista possui protocolo autorizando operações Git; `trackfw_architect` é
      a única autoridade de branch, commit e push.
- [x] `trackfw update` opera exclusivamente no projeto e `trackfw update harness` no escopo global.
- [x] Os cinco aliases deprecated de integração foram removidos e há uma única superfície `help`.
- [x] `make quality` passa e `trackfw validate --json` retorna 0 violações.

## Wave 1 — Contrato e caracterização (1 ML)
> Dependências: nenhuma

**Gates da wave:**
```bash
make quality
bin/trackfw validate --json
```

### ML-1A — Congelar o contrato universal da barrier
**Status:** ✅ Concluído
**Arquivos afetados:**
- `docs/adr/ADR-2026-07-29-barrier-governanca-e-autoridade-do-orquestrador.md`
- `docs/req/REQ-2026-07-29-barrier-governanca-e-autoridade-do-orquestrador.md`
- `docs/roadmaps/wip/ROADMAP-2026-07-29-barrier-governanca-e-autoridade-do-orquestrador.md`
- `docs/cli-parity.md`
- testes de contrato nos **três runtimes**: `internal/commands/barrier_contract_test.go`,
  `npm/tests/barrier-contract.test.js`, `pypi/tests/test_barrier_contract.py`

**Divisão de responsabilidade (autoridade de artefatos):**
- O `trackfw_architect` é o **único** autor de `docs/adr/`, `docs/req/`, `docs/roadmaps/` e
  `docs/cli-parity.md`. Especialistas **não editam** esses caminhos.
- O especialista designado para o ML-1A implementa **exclusivamente** os testes negativos/xfail
  dos três runtimes, a partir do contrato já congelado pelo orquestrador.

**Escopo dos testes negativos (decisão congelada — não delegável ao agente):**
Os testes de contrato são criados nos **três CLIs** já na Wave 1, para que os MLs 2A/2B/2C partam
de uma baseline vermelha idêntica e a regra dura de paridade seja verificável no barrier da Wave 2.
Mecanismo de pendência por runtime: Go → `t.Skip` com motivo explícito; Node.js → `{ skip: true }`
do `node:test`; Python → `@pytest.mark.xfail(strict=True)`.

**Ações:**
1. Definir o schema lógico de um resultado de barrier: `roadmap`, `wave`, `status`, `checks`,
   `evidence`, `failures` e timestamps.
2. Definir estados `pending`, `running`, `passed` e `blocked`.
3. Definir que o CLI executa somente gates declarados pelo projeto e não contém paridade hardcoded.
4. Definir a diferença entre `trackfw barrier` (validação determinística) e
   `/trackfw:barrier` (orquestração de agentes).
5. Adicionar testes negativos/xfail para ML incompleto, evidência ausente e barrier bloqueada,
   sem implementar a produção neste ML.

**Critérios de aceite:**
- [x] Contrato textual e JSON documentado em `docs/cli-parity.md`.
- [x] Testes negativos reproduzem cada falha obrigatória nos três runtimes.
- [x] Nenhum arquivo de `docs/adr/`, `docs/req/` ou `docs/roadmaps/` foi alterado pelo especialista.
- [x] Nenhum gate específico do trackfw é tratado como regra universal.
- [x] `trackfw validate --json` permanece verde.

**Comandos de validação:**
```bash
git diff --check
bin/trackfw validate --json
```

## Wave 2 — Núcleo nativo do comando (3 MLs em paralelo)
> Dependências: Wave 1 concluída e auditada. Os MLs tocam árvores de runtime disjuntas.

**Gates da wave:**
```bash
make quality
bin/trackfw validate --json
```

### ML-2A — Implementar `trackfw barrier` no Go
**Status:** ✅ Concluído
**Arquivos afetados:**
- `internal/commands/barrier.go`
- `internal/commands/root.go`
- `internal/commands/*barrier*_test.go`
- `internal/validator/` somente se a integração exigir novo contrato compartilhado

**Ações:**
1. Adicionar `trackfw barrier <roadmap> --wave <n>` e `--json`.
2. Descobrir a wave e os MLs pelo roadmap canônico, incluindo layout `by_agent`.
3. Verificar status concluído, evidências e gates declarados.
4. Executar `trackfw validate` e retornar exit code não-zero em falha.
5. Emitir resultado JSON estável conforme o contrato da Wave 1.

**Critérios de aceite:**
- [x] ML pendente impede a passagem.
- [x] Falta de evidência impede a passagem.
- [x] Wave verde retorna exit code 0 e `status: passed`.
- [x] Saída textual e JSON são determinísticos.
- [x] Os testes de contrato criados no ML-1A estão ativos neste runtime (sem `t.Skip`, sem
      `{ skip: true }`, sem `xfail`) e passam.
- [x] `go build ./...`, `go test ./...` e `go vet ./...` passam.

**Comandos de validação:**
```bash
go build ./...
go test ./...
go vet ./...
```

### ML-2B — Implementar `trackfw barrier` no Node.js
**Status:** ✅ Concluído
**Arquivos afetados:**
- `npm/src/commands/barrier.js`
- `npm/src/commands/index.js`
- `npm/tests/barrier.test.js`

**Ações:** Espelhar o contrato do ML-2A sem introduzir comportamento específico de Node.js além
   da implementação necessária.

**Critérios de aceite:**
- [x] Paridade de flags, estados, exit codes e JSON com Go.
- [x] Casos verde, ML pendente, evidência ausente e `validate` falho cobertos.
- [x] Os testes de contrato criados no ML-1A estão ativos neste runtime (sem `t.Skip`, sem
      `{ skip: true }`, sem `xfail`) e passam.
- [x] `npm test` passa.

**Comandos de validação:**
```bash
cd npm && npm test
```

### ML-2C — Implementar `trackfw barrier` no Python
**Status:** ✅ Concluído
**Arquivos afetados:**
- `pypi/trackfw/commands/barrier.py`
- `pypi/trackfw/cli.py`
- `pypi/tests/test_barrier.py`

**Ações:** Espelhar o contrato dos MLs 2A/2B usando argparse e as convenções existentes do CLI
   Python.

**Critérios de aceite:**
- [x] Paridade de flags, estados, exit codes e JSON com Go e Node.
- [x] Casos verde, ML pendente, evidência ausente e `validate` falho cobertos.
- [x] Os testes de contrato criados no ML-1A estão ativos neste runtime (sem `t.Skip`, sem
      `{ skip: true }`, sem `xfail`) e passam.
- [x] Suíte Python passa.

**Comandos de validação:**
```bash
python3 -m pytest pypi/tests/test_barrier.py -q
```

### ML-2D — Alinhar as strings divergentes entre os três runtimes (corretivo)
**Status:** ✅ Concluído
**Origem:** reprovação da barrier da Wave 2. Os MLs 2A/2B/2C passaram nos próprios testes mas
produziram textos diferentes em dois pontos não cobertos pelos 8 cenários de contrato.
**Arquivos afetados:**
- `internal/commands/barrier.go`
- `npm/src/commands/barrier.js`
- `pypi/trackfw/commands/barrier.py`
- testes correspondentes nos três runtimes

**Ações:**
1. Alinhar a falha de wave sem MLs para exatamente `wave <n>: no ML found` nos três runtimes.
2. Alinhar as duas mensagens de exit 2 aos textos literais fixados em `docs/cli-parity.md`.
3. Adicionar teste de regressão por runtime para os dois casos, que hoje não têm cobertura.

**Critérios de aceite:**
- [x] Wave sem MLs produz a mesma string de falha nos três runtimes.
- [x] As duas mensagens de exit 2 são byte-idênticas nos três runtimes.
- [x] Os dois casos passam a ter teste de regressão em cada runtime.
- [x] `make quality` passa e `bin/trackfw validate --json` retorna 0 violações.

**Comandos de validação:**
```bash
make quality
bin/trackfw validate --json
```

### ML-2E — Corrigir a ordem de chaves do check `gates` no Go (corretivo)
**Status:** ✅ Concluído
**Origem:** detectado pelo cenário de paridade do ML-4A. A auditoria da Wave 2 comparou os JSONs
dos três runtimes com `sort_keys=True`, o que normalizou a ordem das chaves e mascarou a
divergência. Falha do método de auditoria do orquestrador, não dos MLs 2A/2B/2C.
**Arquivos afetados:**
- `internal/commands/barrier.go`
- `internal/commands/barrier_test.go`

**Diagnóstico:** o struct `barrierCheck` em Go declara os campos na ordem
`Name, Status, Evidence, Failures, Commands`, e `encoding/json` serializa por ordem de declaração.
Resultado observado no check `gates`:

- Go:     `name, status, evidence, failures, commands`
- Node:   `name, status, commands, evidence, failures`
- Python: `name, status, commands, evidence, failures`

O contrato em `docs/cli-parity.md` fixa a ordem do exemplo, na qual `commands` vem em terceiro.
Node e Python estão corretos; o Go diverge.

**Ações:**
1. Reordenar os campos do struct para `Name, Status, Commands, Evidence, Failures`.
2. Adicionar teste que asseverre a **ordem literal** das chaves no JSON serializado, não apenas a
   presença — a ausência desse teste é o motivo de a divergência ter sobrevivido à Wave 2.

**Critérios de aceite:**
- [x] Os três runtimes emitem as chaves do check `gates` na mesma ordem.
- [x] Existe teste que falha se a ordem das chaves regredir.
- [x] `scripts/check-barrier.sh` passa integralmente, incluindo o cenário de paridade.
- [x] `make quality` passa e `bin/trackfw validate --json` retorna 0 violações.

**Comandos de validação:**
```bash
scripts/check-barrier.sh
make quality
```

## Wave 3 — Orquestração e autoridade dos agentes (1 ML)
> Dependências: Wave 2 concluída e auditada.

**Gates da wave:**
```bash
make quality
bin/trackfw validate --json
```

### ML-3A — Atualizar agentes e gerar `/trackfw:barrier`
**Status:** ✅ Concluído
**Arquivos afetados:**
- Os **12** assets de agente em `internal/integrations/assets/agents/`, enumerados: `architect.md`
  (autoridade Git) e os 11 especialistas `backend.md`, `code-quality.md`, `data.md`, `dba.md`,
  `frontend.md`, `iac.md`, `infra.md`, `qa.md`, `security.md`, `tooling.md`, `ux.md`.
- `internal/generators/scaffold.go` — novo slash command `barrier.md` no mapa `commands`
- `npm/src/generators/init.js` e `pypi/trackfw/generators/init_gen.py` — gêmeos manuais do slash
  command e da tabela de slash commands do CLAUDE.md gerado
- `internal/generators/claudemd.go` + testes — anunciar `/trackfw:barrier` na tabela gerada
- testes de generators nos três runtimes
- árvores `npm/src/integrations/assets/agents/` e `pypi/trackfw/integrations/assets/agents/`
  **apenas** via `scripts/sync-integration-assets.sh`

**Duas superfícies distintas — não confundir:**
1. Assets de agente: editar a árvore Go e propagar por `scripts/sync-integration-assets.sh`.
   `check-static-assets.sh` e `check-integration-assets.sh` provam ausência de byte-drift.
2. Slash commands: literais de string em `internal/generators/scaffold.go`, com gêmeos
   **mantidos à mão** em `npm/src/generators/init.js` e `pypi/trackfw/generators/init_gen.py`.
   Nenhum script sincroniza esses três. `check-artifact-parity.sh` cobre apenas `slash_roadmap` —
   o novo `barrier.md` **não terá gate automático**, então a prova de equivalência é manual e
   obrigatória no retorno do ML.

**Ações:**
1. Documentar `trackfw_architect` como única autoridade Git.
2. Remover dos especialistas qualquer instrução que permita commit, push, checkout, branch, merge
   ou rebase.
3. Instruir especialistas a atuar somente por handoff autocontido do `trackfw_architect`.
4. Fazer especialistas recusarem implementação direta quando não houver handoff válido.
5. Criar o slash command `/trackfw:barrier` com o checklist completo da REQ.
6. Instruir o orquestrador a invocar `code-quality` e `security` quando aplicável.
7. Instruir o orquestrador a bloquear a próxima wave após qualquer falha e despachar ML corretivo.
8. Instruir o orquestrador a auditar e somente então executar commit/push.
9. Manter o nome público abstrato `trackfw_architect`; nunca depender de `zeus-tf`.

**Critérios de aceite:**
- [x] `/trackfw:barrier` contém o checklist operacional completo.
- [x] Assets Go/Node/Python permanecem byte-equivalentes após sincronização.
- [x] Especialistas não possuem protocolo autorizando operações Git.
- [x] `trackfw_architect` possui protocolo explícito de auditoria, commit e push.
- [x] Testes cobrem a presença da barrier, a autoridade do orquestrador e a ausência de regras de
      paridade universal.
- [x] `make quality` passa.

**Comandos de validação:**
```bash
scripts/sync-integration-assets.sh
make quality
bin/trackfw validate --json
```

## Wave 4 — Auditoria final e documentação (1 ML)
> Dependências: Wave 3 concluída e auditada.

**Gates da wave:**
```bash
make quality
bin/trackfw validate --json
```

### ML-4A — Provar o fluxo E2E da barrier
**Status:** ✅ Concluído
**Arquivos afetados:**
- `scripts/check-barrier.sh`
- `scripts/check-gates-falsify.sh`
- `docs/cli-parity.md`
- `README.md`
- `site/guide/` e `site/en/guide/` quando aplicável
- roadmap e REQ vinculados

**Ações:**
1. Criar fixture temporária com duas waves e MLs concluídos/pendentes.
2. Provar passagem da primeira wave e bloqueio da segunda quando qualquer check falhar.
3. Provar reexecução após correção e liberação somente com todos os checks verdes.
4. Provar que gates definidos pelo projeto são executados e gates não definidos não são inventados.
5. Provar que a execução do especialista não cria commit ou push.
6. Documentar uso, saída JSON, estados e fluxo de correção.

**Critérios de aceite:**
- [x] Cenários positivos e negativos são não-vacuous.
- [x] O fluxo E2E demonstra `passed` e `blocked`.
- [x] Nenhuma operação Git é executada por especialista.
- [x] Documentação em inglês e português mantém o contrato consistente.
- [x] `make quality` passa.
- [x] `trackfw validate --json` passa sem violações.

**Comandos de validação:**
```bash
scripts/check-barrier.sh
scripts/check-gates-falsify.sh
make quality
bin/trackfw validate --json
git diff --check
```

## Wave 5 — Limpeza de superfície pública (2 MLs sequenciais)
> Dependências: Wave 4 concluída e auditada. O ML-5B vem depois do ML-5A porque a documentação
> compartilhada de paridade e help precisa refletir a remoção dos aliases antes da consolidação.

**Gates da wave:**
```bash
make quality
bin/trackfw validate --json
```

### ML-5A — Remover aliases deprecated de integração
**Status:** ✅ Concluído
**Arquivos afetados:**
- `internal/commands/copilot.go`
- `internal/commands/cursor.go`
- `internal/commands/gemini.go`
- `internal/commands/windsurf.go`
- `internal/commands/amazonq.go`
- `internal/commands/root.go`
- testes de comandos e integração correspondentes
- `README.md`, `site/guide/` e `site/en/guide/` quando houver referência

**Ações:**
1. Remover os cinco aliases deprecated do registro do CLI Go e apagar implementações sem callers.
2. Remover testes que esperam a presença dos aliases e adicionar testes que confirmem a mensagem de
   comando desconhecido/ausente.
3. Atualizar documentação para usar exclusivamente `trackfw agents|skills`.
4. Preservar superfícies de instalação marcadas como `legacy`; elas não são aliases CLI e continuam
   necessárias para migração segura.
5. Registrar a remoção como breaking change no changelog da versão de release.

**Critérios de aceite:**
- [x] Nenhum dos cinco aliases aparece em `trackfw --help`.
- [x] Os comandos `trackfw agents|skills` continuam funcionando.
- [x] Superfícies `legacy` do catálogo continuam listáveis e atualizáveis explicitamente.
- [x] Nenhuma documentação orienta os aliases removidos.
- [x] `go build ./...`, `go test ./...` e `go vet ./...` passam.

**Comandos de validação:**
```bash
go build ./...
go test ./...
go vet ./...
```

### ML-5B — Consolidar a superfície de ajuda
**Status:** ✅ Concluído
**Arquivos afetados:**
- `internal/commands/help.go`
- `internal/commands/root.go`
- `npm/src/commands/help.js`
- `npm/src/commands/index.js`
- `pypi/trackfw/commands/help_cmd.py`
- `pypi/trackfw/cli.py`
- testes de help dos três CLIs
- `docs/cli-parity.md`, `README.md`, `site/guide/` e `site/en/guide/`

**Ações:**
1. Manter uma única superfície explícita `trackfw help [assunto|chave]` para documentação de
   comandos e chaves de configuração.
2. Preservar `trackfw --help` e `<comando> --help` como flags nativas, encaminhando para a ajuda
   contextual sem registrar um segundo comando `help`.
3. Definir resolução determinística: comando conhecido → ajuda do comando; chave de configuração
   conhecida → documentação da chave; valor desconhecido → erro com sugestão.
4. Alinhar saída, exit codes e comportamento entre Go, Node.js e Python.
5. Remover a duplicação de registro/renderer sem apagar a documentação das chaves existentes.

**Critérios de aceite:**
- [x] Existe uma única entrada explícita `help` em cada CLI.
- [x] `trackfw help`, `trackfw help <comando>` e `trackfw help <chave>` funcionam.
- [x] `trackfw --help` e `<comando> --help` continuam funcionando.
- [x] Chave desconhecida retorna erro não-zero e sugestão útil.
- [x] Saída e exit codes são equivalentes nos três CLIs.
- [x] Testes dos três runtimes passam.

**Comandos de validação:**
```bash
go test ./internal/commands -run Help -v
cd npm && npm test -- --test-name-pattern='help'
python3 -m pytest pypi/tests -k help -q
```

### ML-5C — Eliminar o mapa duplicado de slash commands no Node.js
**Status:** ✅ Concluído
**Origem:** defeito pré-existente detectado durante a auditoria do ML-3A.
**Arquivos afetados:**
- `npm/src/generators/init.js`
- testes de generators do Node.js

**Diagnóstico:** o Node.js mantém dois mapas de slash commands. `generateClaudeCommands` lista os
9 comandos; `generateClaudeCommandsForce` lista apenas 6 — faltam `roadmap.md`, `implement.md` e
`barrier.md`. Consequência: `trackfw skills --force` no Node instala menos comandos do que a
instalação normal, e menos do que Go e Python, que usam um único mapa com flag de força.

**Ações:**
1. Eliminar a duplicação: `generateClaudeCommandsForce` deve reusar o mesmo mapa de
   `generateClaudeCommands`, variando apenas o comportamento de sobrescrita — o padrão já adotado
   por `installSkillsInner(force)` no Go.
2. Adicionar teste que prove que os caminhos normal e forçado instalam exatamente o mesmo conjunto
   de comandos, para que a divergência não possa voltar.

**Critérios de aceite:**
- [x] Existe um único mapa de slash commands no Node.js.
- [x] Os caminhos normal e `--force` instalam o mesmo conjunto nos três runtimes.
- [x] Teste de regressão prova a equivalência entre os dois caminhos.
- [x] `make quality` passa e `bin/trackfw validate --json` retorna 0 violações.

**Comandos de validação:**
```bash
cd npm && npm test
make quality
```

### ML-5D — Criar gate de paridade do conjunto de slash commands
**Status:** ✅ Concluído
**Origem:** lacuna estrutural apontada nas auditorias do ML-3A e do ML-5C.
**Arquivos afetados:**
- `scripts/check-artifact-parity.sh` (ou novo `scripts/check-slash-parity.sh`)
- `Makefile` (alvo `parity`)
- `npm/src/generators/init.js`

**Diagnóstico:** o conjunto de slash commands é mantido à mão em três geradores
(`internal/generators/scaffold.go`, `npm/src/generators/init.js`,
`pypi/trackfw/generators/init_gen.py`) e **nenhum gate compara os três**.
`check-artifact-parity.sh` verifica apenas o conteúdo de `roadmap.md`
(`slash-roadmap-content-drift`). Consequência: os dois defeitos desta wave — `barrier.md` ausente
do mapa forçado do Node e o mapa duplicado com 6 de 9 comandos — só foram encontrados por
inspeção manual. A prova de equivalência do `barrier.md` no ML-3A também foi manual.

**Ações:**
1. Criar gate que compare, entre os três runtimes, **o conjunto de nomes** de slash commands e
   **o conteúdo** de cada um, não apenas o de `roadmap.md`.
2. Encadear o gate no alvo `parity` do Makefile.
3. Adicionar cenário de falsificação em `scripts/check-gates-falsify.sh` provando que o novo gate
   acusa quando um comando é removido de um runtime ou tem o texto alterado.
4. Corrigir o defeito latente reportado no ML-5C: `generateClaudeCommands(root)` em
   `npm/src/generators/init.js` recebe `root` mas escreve sempre relativo ao `cwd`, descartando o
   argumento silenciosamente. O gêmeo forçado honra `rootDir` corretamente.

**Critérios de aceite:**
- [x] O gate compara nomes e conteúdo de todos os slash commands nos três runtimes.
- [x] O gate está encadeado no alvo `parity`.
- [x] Cenário de falsificação prova que o gate não é vacuoso.
- [x] `generateClaudeCommands` honra o diretório recebido.
- [x] `make quality` passa e `bin/trackfw validate --json` retorna 0 violações.

**Comandos de validação:**
```bash
make quality
scripts/check-gates-falsify.sh
```

### ML-5E — Restaurar a criação dos arquivos auxiliares de regras (corretivo)
**Status:** ✅ Concluído (desbloqueado pelo ML-5G)
**Dependência:** o código já está implementado no working tree, mas só pode ser concluído após o
ML-5G reconciliar o bloco de regras. Ligar a injeção nos três runtimes exige conteúdo idêntico,
porque `check-identity-parity.sh` compara sha256 por artefato.
**Origem:** regressão funcional detectada na auditoria do ML-5A.
**Arquivos afetados:**
- `internal/generators/agentfiles.go`
- o caminho de instalação baseado em catálogo (`installAITools` e chamadores)
- equivalentes em `npm/src/` e `pypi/trackfw/` se a superfície existir nesses runtimes
- testes correspondentes

**Diagnóstico:** os quatro arquivos auxiliares de regras — `GEMINI.md`,
`.github/copilot-instructions.md`, `.windsurfrules` e `.amazonq/developer/guidelines.md` — só eram
**criados pela primeira vez** pelo alias deprecated removido no ML-5A, via `InjectRulesForTool`.
Os demais chamadores (`trackfw discover` e `trackfw update`) usam `InjectRulesDetected`, que só
injeta em arquivo **já existente** (exceto `cursor`, que dispara pela existência do diretório
`.cursor/`). O caminho de instalação por catálogo nunca chama `InjectRulesForTool`.

Efeito líquido: em projeto novo, nenhum comando do produto cria esses quatro arquivos — eles só
são atualizados se o usuário os criar à mão.

**Decisão do orquestrador:** isto é **regressão**, não parte do breaking change. O ADR sancionou a
remoção dos aliases de CLI, não a perda da capacidade de criar os arquivos de regras. Deve ser
corrigido, não documentado como comportamento aceito.

**Ações:**
1. Ligar a injeção de regras ao caminho de instalação baseado em catálogo, de modo que instalar o
   target correspondente crie o arquivo de regras quando ele não existir.
2. Preservar a idempotência: instalar de novo não pode duplicar o bloco de regras.
3. Adicionar teste que, a partir de projeto vazio, prove que instalar cada um dos quatro targets
   cria o arquivo de regras correspondente.
4. Verificar se Node.js e Python têm superfície equivalente e, em caso positivo, manter paridade.

**Critérios de aceite:**
- [x] Em projeto novo, instalar o target cria o arquivo de regras correspondente.
- [x] Reinstalar não duplica o bloco de regras.
- [x] Teste cobre os quatro arquivos a partir de projeto vazio.
- [x] Paridade verificada entre os runtimes que possuem a superfície.
- [x] `make quality` passa e `bin/trackfw validate --json` retorna 0 violações.

**Comandos de validação:**
```bash
go test ./internal/generators/... ./internal/commands/...
make quality
```

### ML-5F — Reconciliar o conteúdo dos slash commands entre os runtimes
**Status:** ✅ Concluído
**Origem:** o gate criado no ML-5D acusou 3 divergências pré-existentes de conteúdo.
**Arquivos afetados:**
- `internal/generators/scaffold.go`
- `npm/src/generators/init.js`
- `pypi/trackfw/generators/init_gen.py`

**Divergências e resolução:**
1. `move.md`, "Estados válidos" — Go e Node listam `analyzing`, Python não. Resolução: incluir
   `analyzing` no Python (maioria 2-1).
2. `move.md`, "Exemplo" — Go e Python usam `wip`, Node usa `analyzing`. **Decisão de produto do
   usuário: usar `analyzing`**, por ser o estado intermediário antes de `wip` e ensinar o ciclo
   completo em vez do atalho. Ajustar Go e Python. Esta é a única resolução que **não** segue a
   maioria, e é deliberada.
3. `architect.md`, frase de abertura — Python tem um parêntese extra. Resolução: remover do Python
   (maioria 2-1).

**Critérios de aceite:**
- [x] `scripts/check-slash-parity.sh` passa sem divergências.
- [x] O exemplo de `move.md` usa `analyzing` nos três runtimes.
- [x] `make quality` passa e `bin/trackfw validate --json` retorna 0 violações.

**Comandos de validação:**
```bash
scripts/check-slash-parity.sh
make quality
```

### ML-5G — Reconciliar o bloco de regras entre os runtimes (desbloqueia ML-5E)
**Status:** ✅ Concluído
**Origem:** bloqueio do ML-5E. **Decisão do usuário: reconciliar agora, nesta branch**, em vez de
adiar para REQ própria.
**Arquivos afetados:**
- `internal/generators/agentfiles.go`
- os injetores de regras equivalentes em `npm/src/` e `pypi/trackfw/`
- testes correspondentes

**Diagnóstico:** o texto do bloco de regras injetado em `GEMINI.md`,
`.github/copilot-instructions.md`, `.windsurfrules` e `.amazonq/developer/guidelines.md` diverge
entre os três runtimes. O Go omite `analyzing` na lista de estados e o item de ciclo de vida de ML,
tem uma seção `Key Commands` ausente nos outros dois, e **repete o bloco `Architecture Directives`
duas vezes dentro do próprio arquivo** (`agentfiles.go` ~52-61 e ~63-72). Node e Python divergem no
sentido oposto.

Enquanto isso não for reconciliado, o ML-5E não pode ligar a injeção nos três runtimes:
`scripts/check-identity-parity.sh` compara **sha256 por artefato**, então contagens iguais com
conteúdo diferente continuam reprovando.

**Ações:**
1. Remover a duplicação do bloco `Architecture Directives` no Go — é bug, não conteúdo.
2. Unificar o texto do bloco de regras nos três runtimes, incluindo lista de estados
   (com `analyzing`), item de ciclo de vida de ML e a seção `Key Commands`.
3. Adicionar teste que prove que os três runtimes emitem bloco byte-idêntico a partir de projeto
   vazio, para que a divergência não volte.

**Critérios de aceite:**
- [x] O bloco `Architecture Directives` aparece uma única vez no Go.
- [x] Os três runtimes emitem bloco de regras byte-idêntico.
- [x] Existe teste que falha se a divergência regredir.
- [x] `scripts/check-identity-parity.sh` passa com a injeção do ML-5E ligada nos três runtimes.
- [x] `make quality` passa e `bin/trackfw validate --json` retorna 0 violações.

**Comandos de validação:**
```bash
scripts/check-identity-parity.sh
make quality
```

## Wave 6 — Separação entre update de projeto e update do harness (4 MLs)
> Dependências: Wave 5 concluída e auditada. Os MLs implementam o mesmo contrato em árvores de
> runtime disjuntas. O ML-6A define a fronteira antes dos três MLs de runtime.

**Gates da wave:**
```bash
make quality
bin/trackfw validate --json
```

### ML-6A — Fixar o contrato de escopo dos updates
**Status:** ✅ Concluído (contrato autorado pelo orquestrador)
**Arquivos afetados:**
- `docs/cli-parity.md` — seção `## trackfw update vs trackfw update harness`

**Divisão de autoridade:** o contrato é de autoria exclusiva do `trackfw_architect`, como no ML-1A.
Os MLs 6B/6C/6D implementam contra ele.

**Contrato congelado:**
1. `trackfw update` é operação de projeto e **nunca** muta estado global.
2. `trackfw update harness` é operação global e **não** exige `trackfw.yaml` nem cwd de projeto.
3. Quatro estados por target: `updated`, `skipped`, `missing`, `failed`.
4. `missing` **nunca instala** sem `--install-missing` explícito, e não é erro: um harness sem nada
   instalado reporta tudo como `missing` e sai com **exit 0**. Exit não-zero só com `failed`.
5. Flags `--dry-run`, `--json`, `--targets`, `--install-missing` nos dois comandos.
6. Documento JSON com `scope`, `dry_run`, `targets` e `summary` com os quatro contadores.

**Critérios de aceite:**
- [x] O contrato diferencia claramente projeto e global.
- [x] O contrato não permite efeito global acidental a partir de um repositório.
- [x] Dry-run, JSON e estados são documentados.
- [x] Auditoria de paridade dos documentos JSON exige ordem de chaves preservada.

**Comandos de validação:**
```bash
bin/trackfw validate --json
```

### ML-6B — Implementar updates separados no Go
**Status:** ✅ Concluído
**Arquivos afetados:**
- `internal/commands/update.go`
- `internal/commands/update_harness.go`
- `internal/generators/update.go`
- `internal/commands/root.go`
- testes correspondentes

**Ações:**
1. Manter `trackfw update` restrito ao projeto atual.
2. Mover a atualização de skill global para `trackfw update harness`.
3. Criar `trackfw update harness` sem exigir `trackfw.yaml` ou cwd de projeto.
4. Atualizar somente deployments globais já existentes por padrão.
5. Implementar `--dry-run`, `--json`, `--targets`, `--install-missing` e preservação de artefatos
   não gerenciados.

**Critérios de aceite:**
- [x] Executar `trackfw update` em 20 projetos não repete update global.
- [x] `trackfw update harness` atualiza o global uma única vez.
- [x] Saída, estados e filtros seguem o contrato do ML-6A.
- [x] `go build ./...`, `go test ./...` e `go vet ./...` passam.

**Comandos de validação:**
```bash
go build ./...
go test ./...
go vet ./...
```

### ML-6C — Implementar updates separados no Node.js
**Status:** ✅ Concluído
**Arquivos afetados:**
- `npm/src/commands/update.js`
- `npm/src/commands/update-harness.js`
- `npm/src/commands/index.js`
- `npm/src/commands/integrations.js`
- `npm/src/integrations/manager.js`
- testes correspondentes

**Ações:** Espelhar o contrato dos MLs 6A/6B. `update` permanece local; `update harness` opera no
escopo global sem depender de projeto.

**Critérios de aceite:**
- [x] Saída, flags, estados e exit codes equivalentes ao Go.
- [x] `update` não muta global.
- [x] `update harness` atualiza somente global já instalado por padrão.
- [x] `npm test` passa.

**Comandos de validação:**
```bash
cd npm && npm test
```

### ML-6D — Implementar updates separados no Python
**Status:** ✅ Concluído
**Arquivos afetados:**
- `pypi/trackfw/commands/update.py`
- `pypi/trackfw/commands/update_harness.py`
- `pypi/trackfw/cli.py`
- `pypi/trackfw/integrations/manager.py`
- `pypi/trackfw/integrations/command.py`
- testes correspondentes

**Ações:** Espelhar o contrato dos MLs 6A–6C. A limitação atual que exige Go/Node para gates, CI e
slash commands deve ser removida do update local ou formalizada como escopo separado; o novo
`update harness` deve funcionar autonomamente no Python.

**Critérios de aceite:**
- [x] Saída, flags, estados e exit codes equivalentes ao Go e Node.
- [x] `update` não muta global.
- [x] `update harness` atualiza o global já instalado.
- [x] `--dry-run` e `--json` funcionam nos dois comandos.
- [x] Suíte Python passa.

**Comandos de validação:**
```bash
python3 -m pytest pypi/tests -k update -q
```

### ML-6F — Alinhar os três runtimes à lista de targets pinada (corretivo)
**Status:** ✅ Concluído (parcial pelo agente, que falhou por erro de API; completado no ML-6H)
**Nota de auditoria:** os critérios abaixo foram atendidos em conjunto por este ML e pelo ML-6H. A
barrier da Wave 6 reprovou na primeira execução porque eu marquei este ML como concluído sem marcar
os critérios — o check `acceptance_evidence` pegou o descuido do orquestrador.
**Origem:** auditoria cruzada da Wave 6. O contrato do ML-6A fixou estados, flags e ordem de
chaves, mas **não** fixou o conjunto de targets — e os três runtimes produziram três respostas.
Falha do contrato, não dos implementadores; os três reportaram a lacuna honestamente.
**Arquivos afetados:**
- `internal/generators/update.go`, `internal/commands/update*.go`
- `npm/src/lib/update-engine.js`, `npm/src/commands/update*.js`
- `pypi/trackfw/commands/update*.py`
- testes correspondentes

**Divergências medidas empiricamente:**
1. Contagem de targets do harness: Go=3, Node=19, Python=19.
2. Renderização de `path`: Node usa `~/...`, Python usa caminho absoluto.
3. Artefato de `claude-skills`: Node aponta `trackfw-architecture-skill`, Python `trackfw-governance`.
4. Escopo do contrato completo: Node aplicou as 4 flags e o JSON também ao `update` de projeto;
   Go e Python deixaram o `update` de projeto sem flags.

**Resolução (agora pinada em `docs/cli-parity.md`):**
1. Lista fixa de 19 targets, na ordem declarada do catálogo. Go se alinha a Node/Python.
2. `path` sempre tilde-abreviado. Python se alinha.
3. `claude-skills` resolve pelo catálogo; adotar o id que o catálogo declara.
4. O contrato diz "Applies to: both" — o `update` de projeto também expõe as 4 flags e o JSON.
   Go e Python se alinham a Node.

**Critérios de aceite:**
- [x] Os três runtimes declaram os mesmos 19 targets, na mesma ordem.
- [x] `path` é tilde-abreviado nos três.
- [x] `update` de projeto expõe `--dry-run`, `--json`, `--targets` e `--install-missing` nos três.
- [x] JSON byte-idêntico entre os três para o mesmo HOME e projeto, com ordem preservada.
- [x] `make quality` passa e `bin/trackfw validate --json` retorna 0 violações.

**Comandos de validação:**
```bash
make quality
```

### ML-6H — Alinhar o `update` de projeto entre os runtimes (corretivo)
**Status:** ✅ Concluído
**Origem:** o ML-6F falhou por erro de API no meio do trabalho, deixando o escopo de projeto
pendente. Divergências medidas: Python declarava 3 dos 5 targets; Node marcava `updated` onde Go
marcava `skipped`; `tildeify` no Node falhava com `$HOME` contendo barra dupla.
**Resolução:** lista de 5 targets pinada e adotada nos três; `updated` passa a significar mudança
real de conteúdo; `tildeify` corrigido com teste. Descobertas duas lacunas de paridade `init`↔`update`
no caminho: o `init` do Go nunca escrevia agent-hooks e o do Python nunca escrevia
`scripts/trackfw-validate.sh`.

**Critérios de aceite:**
- [x] Os três declaram os mesmos 5 targets de projeto, na mesma ordem.
- [x] `updated` reflete mudança real; idempotência produz `skipped` nos três.
- [x] JSON byte-idêntico entre os três, ordem preservada, normal e `--dry-run`.

### ML-6I — Impedir que os gates mutem o repositório (corretivo)
**Status:** ✅ Concluído
**Origem:** `scripts/check-update-parity.sh` injetava o bloco `trackfw:rules` no `CLAUDE.md` do
próprio repositório — e **passava** enquanto fazia isso. A causa era `install_claude_agents()`
redirecionar `HOME` mas não fazer `cd` para diretório descartável, herdando o `cwd` de quem invocava.
Como o gate está no alvo `parity`, `make quality` mutava a árvore de trabalho.
**Resolução:** invocação isolada em scratch dir; auditados os quatro gates novos; adicionado cenário
`falsify/no-repo-mutation` que compara `git status --porcelain` antes e depois de rodar os gates.
Transforma "eu conferi" em "o pipeline confere".

**Critérios de aceite:**
- [x] Rodar os quatro gates da raiz não altera nenhum arquivo versionado.
- [x] `make quality` passa e não muta a árvore de trabalho.
- [x] Cenário de falsificação prova que a ausência de mutação é verificada.


> **Defeito extraído.** Durante a validação da Wave 5 constatei que `trackfw init --ai-tools`
> grava em `~/.gemini` — um comando de escopo de projeto mutando o harness global, mesma classe de
> defeito que esta wave corrige em `update`. Como está fora da REQ desta entrega (que trata de
> `update`, não de `init`), foi extraído para
> `docs/req/REQ-2026-07-29-escopo-de-init-ai-tools-nao-deve-mutar-o-harness-global.md` e
> `docs/roadmaps/backlog/ROADMAP-2026-07-29-escopo-de-init-ai-tools-nao-deve-mutar-o-harness-global.md`,
> em vez de inflar um roadmap que já cresceu de 9 para 21 MLs.

## Protocolo de conclusão do roadmap

Após a auditoria verde da Wave 4, o `trackfw_architect` deve:

1. marcar todos os MLs e o roadmap como concluídos;
2. executar `trackfw validate --json`;
3. revisar o diff completo;
4. commitar os artefatos consolidados, incluindo código produzido pelos especialistas;
5. fazer push da branch;
6. sugerir a abertura de PR/MR, sem abrir automaticamente sem autorização do usuário;
7. mover este roadmap para `docs/roadmaps/done/`.
