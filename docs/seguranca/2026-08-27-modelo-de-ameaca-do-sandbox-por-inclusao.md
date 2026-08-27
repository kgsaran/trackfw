# Modelo de ameaca: sandbox do update --dry-run por lista de inclusao

> Wave 0 do roadmap `ROADMAP-2026-08-27-sandbox-do-update-dry-run-por-lista-de-inclusao-dos-destinos-declarados.md`
> Agente: hades-tf | Data: 2026-08-27

---

## Completude de enumeracao dos destinos declarados

**Pergunta:** a lista de destinos declarados por target esta completa? Qualquer caminho que um target escreve e nao aparece em sua lista de relPaths e invisivel para o dry-run, que entao mente por omissao.

### Metodo de enumeracao

A busca partiu de `grep -rn "os.WriteFile\|os.Create\|afero.WriteFile\|fs.writeFileSync\|open.*'w'" internal/ npm/src/ pypi/trackfw/ --include='*.go' --include='*.js' --include='*.py'` e foi refinada lendo cada sink de escrita e rastreando de qual target ele e chamado. A lista de relPaths de cada target foi comparada diretamente contra os sinks encontrados.

### Destinos declarados (Go e Node.js) versus destinos reais

| Target | relPaths declarados | Destinos adicionais escritos (nao declarados) |
|---|---|---|
| agent-rules | CLAUDE.md, AGENTS.md, GEMINI.md, .github/copilot-instructions.md, .windsurfrules, .amazonq/developer/guidelines.md, .cursor/rules/trackfw.mdc | nenhum |
| agent-hooks | .claude/settings.json, .codex/hooks.json, .gemini/settings.json, .kiro/hooks/trackfw-attention.json, .github/hooks/trackfw-attention.json, .cursor/hooks.json, scripts/trackfw-attention-signal.sh, scripts/trackfw-attention-cleanup.sh, scripts/trackfw-credential-guard.sh, scripts/trackfw-git-branch-guard.sh | **Gap A:** `.windsurf/hooks.json` / **Gap B:** `.amazonq/cli-agents/q_cli_default.json` |
| codex-project-agents | nenhum (nao usa runFileTarget) | destinos em runtime via catalog — ver Gap D |
| validate-script | scripts/trackfw-validate.sh | nenhum |
| ci-workflow | .github/workflows/trackfw-gate.yml, .gitlab-ci-trackfw.yml | nenhum |
| git-hooks | .husky/pre-commit OU lefthook.yml | nenhum |
| claude-commands | .claude/commands/trackfw | nenhum |

### Gap A — .windsurf/hooks.json (MEDIDO)

`InjectWindsurfHooks` (agentfiles.go, funcao `InjectWindsurfHooks`) e chamada pelo apply de `agent-hooks` quando `.windsurfrules` e detectado no root. Ela escreve `.windsurf/hooks.json`. Este caminho nao consta em nenhuma das tres CLIs no relPaths de `agent-hooks`:
- Go `runProjectTarget` case "agent-hooks": lista de 10 caminhos — `.windsurf/hooks.json` ausente.
- Node.js `update.js` linhas 208-232: lista de 10 caminhos — `.windsurf/hooks.json` ausente.
- Python `AGENT_HOOKS_RELATIVE_PATHS` (update.py linhas 68-78): lista de 9 caminhos — `.windsurf/hooks.json` ausente.

Efeito no sandbox por inclusao: se o sandbox copia apenas relPaths, o apply de `agent-hooks` roda sem `.windsurfrules` disponivel (nao esta em relPaths de `agent-hooks`). `InjectHooksDetected` nao detecta windsurf → nao chama `InjectWindsurfHooks` → `.windsurf/hooks.json` nao e escrito no sandbox. Dry-run nao reporta este caminho; real run escreve. A omissao e silenciosa.

### Gap B — .amazonq/cli-agents/q_cli_default.json (MEDIDO)

`InjectAmazonQHooks` (agentfiles.go) e chamada quando `.amazonq/` e detectado. Escreve `.amazonq/cli-agents/q_cli_default.json`. Nao declarado em nenhuma CLI. Mesmo mecanismo do Gap A.

### Gap C — .github/copilot-instructions.md como sinal de deteccao para destino declarado (MEDIDO + RACIOCINADO)

Medido: `InjectHooksDetected` verifica a existencia de `.github/copilot-instructions.md` para decidir se chama `InjectCopilotHooks`. `InjectCopilotHooks` escreve `.github/hooks/trackfw-attention.json`, que ESTA em relPaths de `agent-hooks`.

`.github/copilot-instructions.md` NAO esta em relPaths de `agent-hooks`.

Raciocinado: em um sandbox por inclusao (somente relPaths copiados), `.github/copilot-instructions.md` nao estara presente no sandbox de `agent-hooks`. Consequencia: `InjectHooksDetected` nao detecta copilot → `InjectCopilotHooks` nao e chamada → `.github/hooks/trackfw-attention.json` nao e escrito. Dry-run reporta `missing` ou `skipped`. Real run, com `.github/copilot-instructions.md` presente no projeto, escreve o arquivo e reporta `updated`. Discrepancia mensuravel.

### Gap D — codex-project-agents nao usa runFileTarget (MEDIDO)

update.go linhas 1907-1917: o case `codex-project-agents` nao chama `runFileTarget`. Chama `codexProjectAgentsApply(root, opts)` diretamente e retorna `TargetUpdated` incondicionalmente se nenhum erro. Nao ha lista de relPaths; nao ha comparacao de hashes antes/depois. O target reporta sempre `updated`, mesmo que nada tenha mudado.

Adicionalmente, `codexProjectAgentsApply` faz `withChdir(root, func() { projectAgentModels = config.Load().AgentModels })` (update.go:1967-1969): le `trackfw.yaml` do root para extrair `agent_models`. `trackfw.yaml` nao esta em nenhum relPaths. Se a lista de inclusao for aplicada ao sandbox global, `trackfw.yaml` precisa ser copiado separadamente; se nao for copiado, `agent_models` vira vazio.

### Gap E — trackfw.yaml como entrada de conteudo para agent-rules (MEDIDO)

agentfiles.go linha 111: `block := trackfwRulesBlock(config.ReadAgentConventions(cwd))`. Esta linha esta dentro de `injectOrUpdateRules`, que e o apply de `agent-rules`. `ReadAgentConventions` le `trackfw.yaml` via `config.Load()` com cwd setado no sandbox. `trackfw.yaml` nao esta em agent-rules relPaths.

Se `agent_conventions` estiver configurado no projeto e `trackfw.yaml` nao for copiado para o sandbox, o bloco `agent_conventions` sera omitido do CLAUDE.md gerado no dry-run. Hash diverge do real run. Relatorio mente.

### Gap F — parity gap Python (MEDIDO)

Python `AGENT_HOOKS_RELATIVE_PATHS` (update.py linhas 68-78): 9 entradas — `scripts/trackfw-git-branch-guard.sh` ausente. Go e Node.js tem 10. Defeito pre-existente independente da lista de inclusao; a correcao do sandbox vai propagar a discrepancia se nao for corrigida junto.

### Resultado da enumeracao

A lista esta INCOMPLETA. Gaps A e B sao caminhos escritos nao declarados. Gap C e um sinal de deteccao para um destino declarado que nao estara no sandbox. Gaps D e E envolvem `trackfw.yaml` e o target que nao usa runFileTarget. Gap F e paridade Python pre-existente.

---

## Modelo de ameaca

O adversario desta Wave 0 e o implementador que entende a falha superficialmente e aplica o remendo minimo. As tres rotas de esvaziamento sao:

### Rota 1 — Copiar relPaths e parar (implementador otimista)

O implementador substitui `copyProjectTree` por um loop sobre relPaths de cada target. Testa contra um projeto simples sem `.windsurfrules`, `.amazonq/` ou `agent_conventions`. Testes passam. Gaps A, B, C e E permanecem silenciosos porque as condicoes de disparo nao estao presentes no fixture de teste.

Nenhuma regra escrita proibe isso. `make quality` nao testa comportamento diferencial dry-run x real. A Wave 0 so fecha se o AC4 da REQ for verificado contra um fixture com todas as condicoes de deteccao ativas.

### Rota 2 — Replicar Node.js sem auditar as diferencas

Node.js ja usa inclusao por relPaths (`update-engine.js:runFileTarget`, linhas 97-128). O implementador do Go/Python copia o padrao. Correto para o caso basico. Porem: (a) Node.js tem os mesmos Gaps A e B; (b) Node.js cria um sandbox por-target, o Go atual cria um sandbox compartilhado — a transicao muda o comportamento de `codex-project-agents`.

### Rota 3 — Usar os.ReadFile no loop de inclusao (implementador apressado)

O implementador copia apenas os relPaths mas ainda usa `os.ReadFile(path)` para ler e escrever cada arquivo. Se algum destino declarado for ele mesmo um symlink quebrado (ex: CLAUDE.md apontando para um link morto), o abort ressurge — agora com blast radius menor (so destinos declarados), mas a classe nao fecha.

Node.js evita isso: `copyPath` usa `fs.existsSync` (que segue symlinks — symlink quebrado retorna false e o arquivo e simplesmente omitido) e `fs.lstatSync` para classificar sem seguir. Go precisa de `os.Lstat` + decisao explicita.

---

## Alvos de falsificacao nas duas direcoes

Para cada gap identificado: como o comportamento regride (falso negativo do dry-run) e como o gate deve pegar; e o que quebra na direcao oposta (falso positivo).

### Gap A e B — destinos nao declarados (.windsurf/hooks.json, .amazonq/cli-agents/q_cli_default.json)

**Direcao A — FN (dry-run silencioso, real run escreve):**
- Fixture: projeto com `.windsurfrules` presente (ou `.amazonq/` presente).
- Esperado apos fix correto: dry-run reporta estes caminhos mesmo que nao estejam em relPaths, OU relPaths e atualizado para inclui-los.
- Sintoma de regressao: dry-run nao menciona estes caminhos; real run os cria.
- Gate: `trackfw update --dry-run --json | jq '.targets[] | select(.id=="agent-hooks") | .state'` deve ser `updated` no fixture, nao `skipped`, se `.windsurf/hooks.json` seria criado no real run.

**Direcao B — FP (dry-run diz updated, real run nao muda nada):**
- Fixture: projeto com `.windsurf/hooks.json` ja no estado correto.
- Sintoma: dry-run diz `updated` mas real run diz `skipped`.
- Mais improvavel dado que estes caminhos nao estao em relPaths para comparacao.

### Gap C — sinal de deteccao do copilot (.github/copilot-instructions.md)

**Direcao A — FN (dry-run skipped/missing, real run updated):**
- Fixture: `.github/copilot-instructions.md` presente; `.github/hooks/trackfw-attention.json` ausente.
- Dry-run (sandbox sem .github/copilot-instructions.md): copilot nao detectado → hook nao escrito → estado `missing`.
- Real run: copilot detectado → hook escrito → estado `updated`.
- Gate: fixture + comparar relatorio dry-run x real em /tmp isolado.

**Direcao B — FP (dry-run updated, real run skipped):**
- Fixture: copilot-instructions.md ausente do projeto (copilot nao instalado). Sandbox sem o arquivo. Dry-run: hook nao escrito → `missing`. Real run idem → `missing`. Nao ha FP neste caso.
- O FP emerge se o sandbox incluir copilot-instructions.md acidentalmente (ex: copiado como parte de outro target).

### Gap D — codex-project-agents retorna sempre updated

**Direcao A — FN:** nao e aplicavel porque o target nunca reporta `skipped`.

**Direcao B — FP (sempre updated mesmo sem mudanca):** o comportamento atual ja e FP por construcao — o target nao faz hash comparison. Fora do escopo da lista de inclusao mas e residual declarado.

### Gap E — trackfw.yaml ausente do sandbox (agent-rules e agent_conventions)

**Direcao A — FN (dry-run skipped, real run updated):**
- Fixture: `agent_conventions` configurado em trackfw.yaml; CLAUDE.md pre-existente sem o bloco.
- Dry-run sem trackfw.yaml no sandbox: bloco ausente da geracao → hash igual ao CLAUDE.md existente → `skipped`.
- Real run: le trackfw.yaml → gera bloco → hash difere → `updated`.

**Direcao B — FP (dry-run updated, real run skipped):**
- Fixture: `agent_conventions` configurado; CLAUDE.md pre-existente JA tem o bloco correto.
- Dry-run sem trackfw.yaml: bloco ausente → hash difere do CLAUDE.md existente → `updated`.
- Real run: le trackfw.yaml → gera o mesmo bloco → hash identico → `skipped`.

### Classe de symlink — copyPath vs os.ReadFile

**Direcao A — FN (abort ressurge no Go se os.ReadFile for usado no loop de inclusao):**
- Fixture: CLAUDE.md e um symlink quebrado.
- Fix com os.ReadFile: abort; dry-run falha com erro. Comportamento identico ao defeito original, mas so para destinos declarados.
- Fix correto (os.Lstat): symlink quebrado → arquivo tratado como `missing`.
- Gate (direcao A): `ln -sf /nao-existe CLAUDE.md && trackfw update --dry-run` deve completar sem erro.

**Direcao B — gate de regressao:**
- Fixture: sandbox por inclusao deve NAO copiar `.venv/bin/python` quebrado.
- Gate (direcao B): o sandbox NAO deve fazer WalkDir — deve copiar exatamente os relPaths e nada mais. Verificavel por mock/spy ou por auditoria do codigo: nenhuma chamada a `filepath.WalkDir` ou equivalente no caminho do dry-run apos a correcao.

---

## Residual declarado

Este design aceita nao cobrir os seguintes pontos:

**R1 — Gaps A e B nao fecham sem atualizar relPaths.**
`.windsurf/hooks.json` e `.amazonq/cli-agents/q_cli_default.json` sao escritos mas nao declarados. A lista de inclusao nao fecha este risco automaticamente: ou os relPaths sao expandidos (mudanca de contrato), ou os dois targets recebem tratamento similar ao `codex-project-agents` (fora de runFileTarget). Decisao pendente para ML-1A.

**R2 — codex-project-agents esta estruturalmente fora da garantia de inclusao.**
O target nao usa runFileTarget. Seus destinos em runtime vem de um catalog resolvido via `integrations.BuildPlans`. Nao e possivel declarar um relPaths estatico para ele. O design aceita isso: a verificacao deste target e via `manager.Inspect`, nao via hash comparison. O relatorio de dry-run para este target e sempre `updated` (nao distingue mudanca de ausencia de mudanca).

**R3 — trackfw.yaml e um arquivo lido-para-decidir que nao e um destino.**
Adiciona-lo a relPaths de todos os targets que o consomem resolveria os Gaps C e E, mas tornaria o dry-run dependente de copiar um arquivo de configuracao que ele nao escreve. O design aceita a alternativa: copiar trackfw.yaml para o sandbox raiz como prerequisito, separado dos relPaths de targets individuais. Esta decisao esta fora do escopo desta Wave 0 e deve ser registrada como AC adicional para ML-1A.

**R4 — Sinal de deteccao do copilot nao fechado estaticamente.**
Gap C (`.github/copilot-instructions.md` como sinal de deteccao) nao e detectavel por nenhum gate estatico de relPaths. O unico gate efetivo e um teste comportamental: fixture com copilot instalado, dry-run, comparar relatorio com real run no mesmo fixture em /tmp isolado.

**R5 — Parity gap Python (Gap F) e pre-existente e independente.**
`scripts/trackfw-git-branch-guard.sh` ausente de `AGENT_HOOKS_RELATIVE_PATHS` no Python. A correcao do sandbox vai propagar este gap para o sandbox por inclusao do Python. Deve ser corrigido em paralelo no ML-1A.

**R6 — Symlinks validos em destinos declarados sao copiados como arquivos regulares no Node.js.**
`copyPath` usa `fs.existsSync` (segue symlink) e `fs.copyFileSync` (copia conteudo do alvo, nao o link em si). Um destino declarado que seja symlink valido vira arquivo regular no sandbox. Se o real run escreve atraves do symlink para o mesmo conteudo, o comportamento e identico. Se o real run reescreve o link em si (improvavel com os writers atuais), o hash diverge. O design aceita este caso como nao ocorrente nos writers atuais.
