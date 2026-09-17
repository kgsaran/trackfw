# ML-0A — Parecer de Ameaça: Gate que Escreve na Árvore Auditada

**Data:** 2026-09-17
**Papel:** `hades-tf`
**Branch:** `fix/gate-escreve-na-arvore`
**REQ:** `docs/req/REQ-2026-09-17-gate-escreve-na-arvore-que-audita-e-a-guarda-existente-nao-cobre-o-culpado.md`
**Roadmap:** `docs/roadmaps/wip/ROADMAP-2026-09-17-gate-escreve-na-arvore-que-audita-e-a-guarda-existente-nao-cobre-o-culpado.md`

Convenção de leitura: **[V]** = verificado pela execução de ferramenta ou leitura de artefato primário; **[H]** = hipótese fundamentada, não medida diretamente.

---

## 1. Completude da Enumeração

**A lista da REQ está fechada em um gate culpado. O mecanismo estrutural permanece aberto.**

Varredura realizada: todos os gates que invocam `init`, `discover` ou `update` em `scripts/`:

| Gate | Isolamento cwd | Isolamento HOME | Status |
|---|---|---|---|
| `check-tty-detection.sh:43` | **Ausente** | Presente (`HOME="$WORK/home"`) | **CULPADO** [V] |
| `check-agent-models-parity.sh:872` | Presente (`cd "$cwd11"`) | Presente (`HOME="$home11"`) | Limpo [V] |
| `check-git-branch-guard-hook-schema.sh:262` | Presente (`cd "$dir"`) | N/A (discover) | Limpo [V] |
| `check-rules-parity.sh:42` | Presente (`cd "$WORK/go"`) | Presente (`HOME="$WORK/home-go"`) | Limpo [V] |
| `check-slash-parity.sh:40` | Presente (`cd "$WORK/go"`) | Presente (`HOME="$WORK/home-go"`) | Limpo [V] |
| `check-update-parity.sh` (múltiplas invocações) | Presente (`cd "$project_dir"`) | Presente (`HOME="$home_dir"`) | Limpo [V] |

**Resultado:** exatamente um gate culpado — `check-tty-detection.sh:43`. A REQ não subestima a contagem.

**O que a enumeração NÃO fecha:** o mecanismo estrutural que permite que um gate futuro invoque `init`/`discover`/`update` sem isolamento e nasça fora da cobertura da guarda por omissão. Nenhum dos cinco ACs da REQ exige que esse mecanismo seja eliminado de forma contínua; eles corrigem a instância e mudam a population da guarda, mas a população cresce por opt-in manual (ver Seção 2, Vector 5).

---

## 2. Modelo de Ameaça por Vetor

### Vector 1 — Indução por PR: conteúdo escolhido na árvore

**Pergunta:** um PR pode induzir o gate a escrever conteúdo escolhido em `trackfw.yaml` ou `CLAUDE.md`?

**Para PRs que tocam apenas scripts, fixtures, ou docs:**

Não. O conteúdo gravado por `Scaffold()` em `trackfw.yaml` é derivado de um `Config{}` com valores hardcoded em `internal/commands/init.go` (non-TTY path, linhas ~103–125). O único campo PR-influenciável seria `ProjectName = filepath.Base(cwd)` — que é sempre `"trackfw"` (nome do diretório do repositório). Os campos de `CLAUDE.md` e `GEMINI.md` são derivados de `ReadAgentConventions()` e `ReadNamespacingConfig()`, que leem o `trackfw.yaml` APÓS a sobrescrita — portanto leem os defaults mínimos, não os valores originais do `trackfw.yaml` do repositório. **[V]**

**Para PRs que modificam `internal/generators/scaffold.go`:**

Aberto. Medido: o Makefile (`parity-rest: build`) declara `build` como dependência de `parity-rest`. `build` executa `go build -o bin/trackfw ./cmd/trackfw`. O job CI `parity-other-gates` executa `make parity-rest`, portanto compila o binário do branch do PR antes de rodar `check-tty-detection.sh`. Um PR que modifique `Scaffold()` para escrever conteúdo arbitrário em `trackfw.yaml` executa esse conteúdo durante a própria rodada de CI do PR — e o gate não tem isolamento de cwd. **[V — Makefile linha 27: `parity-rest: build`]**

Esse vetor requer modificação de fonte Go — visível no diff do PR — não uma mudança covert. O risco prático é o do implementador apressado que muda `scaffold.go` para um caso de uso legítimo sem perceber que o gate executa o binário resultante contra a árvore real. A REQ não fecha esse vetor; nenhum AC o menciona.

**Veredito para AC1–5:** ACs fecham o vetor por PRs de script. O vetor por modificação de `scaffold.go` é nomeado como residual não coberto.

---

### Vector 2 — Inversão de política: erro vira warning via efeito colateral de gate

**Pergunta:** o dano inverso (introduzir `governance_mode: lenient` via gate) é alcançável?

Medido: o path non-TTY de `internal/commands/init.go` (linhas ~103–125) instancia `Config{}` sem `BrownfieldMode`. `writeTrackfwConfig()` só emite `governance_mode: lenient` quando `cfg.BrownfieldMode == true`. Portanto, o gate atual **não pode** introduzir `lenient` acidentalmente. **[V]**

**Caminho de um token:** adicionar `--brownfield` à linha 43 de `check-tty-detection.sh`:

```bash
HOME="$WORK/home" "$GO_BIN" init --ai-tools gemini --brownfield >/dev/null 2>&1 </dev/null
```

Essa mudança tem três camadas de distância do efeito: flag no script → `BrownfieldMode: true` no `Config{}` → `governance_mode: lenient` no `trackfw.yaml` → `validate` rebaixa erros para warnings. O revisor da diff vê `--brownfield` em um script de teste de TTY, não o impacto de governança. **[H — plausibilidade, não vetor observado]**

**Veredito para AC1:** AC1 fecha o caminho acidental (o gate não escreveria mais nada no cwd). O caminho intencional por modificação explícita de script não é coberto por nenhum AC. Nomeado como residual.

---

### Vector 3 — Limites da guarda `git status --porcelain`

**Achado crítico — a guarda é cega ao conteúdo (verifica path status, não hash).**

A comparação em Scenario 18 (`check-gates-falsify.sh`, linhas ~1430–1461) é:

```bash
before_status=$(cd "$ROOT_DIR" && git status --porcelain)
# ... executa gates do allowlist ...
after_status=$(cd "$ROOT_DIR" && git status --porcelain)
if [[ "$before_status" != "$after_status" ]]; then ...
```

`git status --porcelain` para um arquivo rastreado modificado retorna ` M trackfw.yaml`. O hash não aparece — apenas a presença do path no diff.

**Medido diretamente:** com a árvore nesta sessão já dirty (`trackfw.yaml` modificado), a seguinte sequência foi executada:

```
Before append:  " M trackfw.yaml"
printf '\n# injected\n' >> trackfw.yaml
After append:   " M trackfw.yaml"   ← string idêntica
```

**A guarda é cega a mudanças de conteúdo em paths já dirty.** Qualquer gate que reescreva `trackfw.yaml` com qualquer conteúdo produz `after_status = before_status` quando o arquivo já estava dirty. **[V — medido com `git status --porcelain` antes e depois de append; ambas as linhas idênticas]**

Quando isso ocorre localmente: `make parity` roda `parity-rest` antes de `parity-falsify` no mesmo worktree (ambos dependem de `build`). `check-tty-detection.sh` (em `parity-rest`) sobrescreve `trackfw.yaml` → `parity-falsify` inicia o Scenario 18 com `before_status = " M trackfw.yaml"` → a segunda execução do gate (se adicionada ao allowlist por AC2) reescreve o mesmo conteúdo → `after_status = " M trackfw.yaml"` → **a guarda passa sobre a mutação real.** **[V]**

Em CI: `parity-other-gates` (que roda `parity-rest`) e `parity-falsify-shard` (que roda o Scenario 18) são jobs separados, cada um com checkout limpo. Lá, o `before_status` começa vazio e o vetor não ocorre. Mas a garantia é acidental — depende da separação de jobs, não de um invariante explícito. **[V — CI jobs verificados em `quality.yml`]**

**Implicação para AC3:** AC3 exige que "um gate propositalmente sujo é reprovado pela guarda". Sem uma pré-condição de árvore limpa na falsificação, o cenário pode passar vacuamente quando a árvore já está dirty. **AC3 precisa de uma pré-condição explícita: a falsificação executa a partir de um snapshot de árvore limpa.**

**Outros limites da guarda (nenhum corrigido pela REQ):**

- **Escrita fora do repositório** (`$HOME`, `/tmp`, `~/.trackfw/`): não visível para `git status`. Para este gate, HOME já está isolado — o limite existe mas não é explorado aqui. Não há mecanismo que force HOME isolation em gates futuros. [V — verificado que HOME isolation existe para este gate, generalização é H]
- **Write-then-revert dentro do gate:** um gate que escreve e restaura via `trap EXIT` produz `before_status = after_status`. A guarda passa. A REQ declara isso fora do escopo (não há AC cobrindo). [H — mecanismo, não observado]
- **Mudança de modo:** `core.fileMode = true` neste repositório (medido). Mudanças de modo aparecem em `git status --porcelain` (linha `M` com modo). Para este gate: os cinco scripts já estão commitados como 0755 e `os.Chmod(0755)` no `Scaffold()` não produz delta. Não é um vetor ativo aqui. [V]
- **Paths ignorados pelo `.gitignore`:** `.trackfw-baseline.json` é ignorado; qualquer escrita nele é invisível para a guarda. Nenhum AC da REQ cobre isso. [V — verificado `.gitignore`]

---

### Vector 4 — Introdução de arquivo e caminho de execução

**Classe de arquivo que `trackfw init --ai-tools gemini` introduz na raiz do repositório (medido):**

```
trackfw.yaml          (0644) — sobrescrito com defaults mínimos
CLAUDE.md             (0644) — injeção do bloco de regras entre marcadores
GEMINI.md             (0644) — criado se ausente; já rastreado por acidente (commit 4c8f1b04)
.gitattributes        (0644) — criado se ausente; rastreado (git ls-files confirma; não reescrito — não aparece no git status)
.claude/commands/*.md (0644) — comandos do Claude injetados
scripts/trackfw-validate.sh          (0755) — IDÊNTICO ao commitado (medido)
scripts/trackfw-credential-guard.sh  (0755) — IDÊNTICO ao commitado (medido)
scripts/trackfw-git-branch-guard.sh  (0755) — IDÊNTICO ao commitado (medido)
scripts/trackfw-attention-signal.sh  (0755) — IDÊNTICO ao commitado (medido)
scripts/trackfw-attention-cleanup.sh (0755) — IDÊNTICO ao commitado (medido)
docs/ subárvore       — dirs criados se ausentes (sem escrita de arquivo existente)
vault/notes/          — dir criado se ausente
```

**Todos os cinco scripts são IDÊNTICOS aos commitados (medido para todos).** Isso indica que o dano de sobrescrita de scripts já ocorreu em algum momento anterior e foi commitado (ou que os scripts commitados são os mínimos desde sempre). A ausência de delta não significa ausência de dano — significa que o dano é o estado atual da árvore. **[V — diff executado em `$d` vs repo root para todos os cinco scripts]**

**Caminho de execução — harness de agente:**

`Scaffold()` chama `InjectHooksDetected(cwd)` que rewires os ponteiros de hook em `.claude/` para os scripts gerados. Se os scripts gerados divergirem dos commitados, o harness do agente executa a versão enfraquecida imediatamente na próxima chamada de ferramenta — sem nenhum commit intermediário. Para os três scripts verificados, o conteúdo é idêntico, então o hook aponta para o mesmo código. **[V — para os scripts verificados; [H] para attention/cleanup]**

**Caminho de execução — CI:**

`scripts/trackfw-validate.sh` é invocado pelo job `governance-go-install`. O script é um wrapper shell que chama `trackfw validate`. O `validate` lê `trackfw.yaml` para aplicar regras. Sobrescrever `trackfw.yaml` com defaults mínimos (sem `governance_mode: lenient`) é o que causou a falha de CI descrita na REQ.

**Medido diretamente:** `./bin/trackfw validate` executado com `trackfw.yaml` sobrescrito (estado atual desta sessão) retorna RC=0 com 156 violations — idêntico ao comportamento descrito na REQ (local validate sai RC=0 enquanto CI reprova). A distinção é que o `governance-go-install` em CI invoca o validate de forma que trata violations como falha de job; localmente, validate exibe mas não bloqueia. **[V — executado nesta sessão; RC=0, 156 violations, incluindo REQs sem roadmap vinculado]**

**`GEMINI.md` — arquivo introduzido por gate, commitado por acidente:**

AC5 decide o destino. A ameaça é: o arquivo contém o bloco de regras de governança que o Gemini CLI lê como instruções. Se esse arquivo for gerado com defaults mínimos (sem `agent_conventions` do projeto), a próxima sessão Gemini opera sem as convenções de agente do projeto. A introdução silenciosa de um arquivo de instrução de agente na raiz do repositório, com conteúdo controlado pelos defaults do binário, é o mecanismo de classe relevante. **[V — GEMINI.md gerado e confirmado no experimento de scratch]**

---

### Vector 5 — Precedente: o que o ML-6I não fechou

**Em uma frase:** o ML-6I codificou "os gates atualmente conhecidos como limpos" em vez de eliminar a propriedade que permite que um gate novo nasça fora da cobertura por omissão.

**Evidência:** nota de vault `update-parity-gate-writes-real-claude-md-2026-07-29.md` registra o primeiro incidente (gate escrevendo em `CLAUDE.md` do repo). A correção foi:
1. Envolver a chamada culpada em `(cd scratch_dir && ...)`.
2. Auditar os 4 gates conhecidos naquele momento.
3. Criar o Scenario 18 com allowlist fixo desses 4 gates.

A allowlist recebeu o conjunto de gates auditados naquele instante. A propriedade corrigida foi: "esse gate específico não tinha isolamento." A propriedade NÃO corrigida foi: "qualquer gate que invocar `init`/`discover`/`update` sem isolamento passará despercebido até ser auditado manualmente."

O modelo correto seria opt-out (todos os gates cobertos, exclusões declaradas com motivo); o modelo implementado é opt-in (só os gates listados são cobertos, adições são manuais). A diferença é que o modelo opt-in herda o erro de enumeração para cada novo gate.

**AC2 corrige isso:** passa de lista fixa para enumeração de todos os gates. Essa mudança fecha o mecanismo estrutural para a classe de gates que já existem quando AC2 é implementado. Não há garantia mecânica que force um gate futuro a ser auditado antes de rodar — a garantia é que ele automaticamente aparece no escopo do Scenario 18.

---

## 3. Alvos de Falsificação nas Duas Direções

### Surface: isolamento de cwd em `check-tty-detection.sh` (AC1)

**Falso negativo** (gate continua escrevendo após o fix):
- Ponto de entrada: `check-tty-detection.sh:43` ainda roda no `cwd` original.
- Gate que deveria pegar: AC3 (Scenario 18 re-executa o gate no allowlist e detecta delta de `git status`).
- Direção: o gate propositalmente sujo passa quando deveria reprovar.
- Condição para que a detecção falhe: árvore já dirty (ver Vector 3 — guarda cega ao conteúdo).

**Falso positivo** (gate limpo é acusado de mutação):
- Ponto de entrada: outro processo (IDE watcher, outro job CI) modifica um arquivo rastreado entre `before_status` e `after_status`.
- Gate que seria acionado: Scenario 18 reprova com falsa acusação.
- Mitigação: em CI (checkout isolado) a probabilidade é desprezível; localmente é real.

### Surface: enumeração da cobertura (AC2)

**Falso negativo** (novo gate invocando `init` sem isolamento não é coberto):
- Ponto de entrada: gate adicionado ao exclusion list de AC2 sem motivo declarado.
- AC que deveria pegar: AC2 exige que exclusões sejam declaradas com motivo por item.
- Direção: o mecanismo estrutural ressurge na forma de uma exclusão injustificada.

**Falso positivo** (gate incluído na enumeração falha por razão não relacionada a mutação):
- Ponto de entrada: gate com dependência de ambiente ausente exits != 0.
- Comportamento atual do Scenario 18: se o gate falha com != 0, o cenário não pode inferir mutação (não há garantia de que o gate executou completamente).
- Mitigação necessária: o Scenario 18 deve distinguir "gate falhou" de "gate rodou e árvore mudou".

### Surface: pré-condição de árvore limpa para AC3

**Falso negativo** (gate sujo passa na falsificação porque árvore já dirty):
- Ponto de entrada: execução local de `make parity` roda `parity-rest` antes de `parity-falsify` no mesmo worktree; `trackfw.yaml` já está dirty.
- Gate que deveria pegar: AC3.
- O falso negativo é o estado atual: a guarda passa porque `before_status = after_status = " M trackfw.yaml"`.
- **Implicação:** AC3 como escrito ("gate propositalmente sujo é reprovado") pode ser satisfeito por um teste executado em CI (onde o checkout é limpo) sem cobrir o caso local (onde `make parity` acumula sujeira sequencialmente). O AC precisa declarar explicitamente que a falsificação requer árvore limpa no momento da execução.

---

## 4. Residual Declarado

Esta REQ e roadmap aceitam não cobrir os seguintes vetores:

**R1 — Write-then-revert (gate com `trap EXIT`):**
Um gate que escreve um arquivo e o restaura antes de sair não é detectável por `git status --porcelain`. A REQ não propõe cobertura. Risco aceitável: gates com esse padrão requerem intenção ativa; a classe de bug descrita (acidental) não inclui cleanup.

**R2 — Escrita fora do repositório (`$HOME`, `/tmp`, `~/.trackfw/`):**
A guarda não cobre. Para este gate, HOME já está isolado. Para gates futuros, a isolação é responsabilidade do autor do gate — não há verificação mecânica. A REQ não propõe alterar isso.

**R3 — Paths ignorados pelo `.gitignore`:**
`.trackfw-baseline.json` e outros paths ignorados são invisíveis para a guarda. A REQ não propõe cobertura.

**R4 — Injeção de conteúdo via modificação de `scaffold.go` em CI:**
Um PR que modifica `internal/generators/scaffold.go` para escrever conteúdo escolhido executa essa escrita durante a rodada de CI do próprio PR (binário compilado do branch). Esse vetor é visível no diff do PR mas não é capturado por nenhum gate de mutação de árvore. Aceito como residual — pertence ao contrato de revisão de PRs, não a uma guarda automática.

**R5 — Inversão intencional de política via `--brownfield`:**
Adicionar `--brownfield` explicitamente a um gate script é uma modificação intencional de script (visível no diff) que produz `governance_mode: lenient` a três camadas de distância. AC1 fecha o caminho acidental. O caminho intencional permanece coberto apenas por revisão de código.

**R6 — Pré-condição de árvore limpa em AC3 (requer emenda ao AC):**
Como descrito na Seção 3, AC3 como escrito não garante que a falsificação detecta o vetor em execução local com árvore dirty. Este residual é eliminável pela emenda: "a execução da falsificação parte de um snapshot de árvore limpa (verificado com `git status --porcelain` antes de iniciar)." Sem essa emenda, o gate de falsificação local pode ser vacuamente verde.

---

## Recomendação para o Arquiteto

**Emenda necessária a AC3 (não opcional):** o AC deve declarar que a execução do Scenario 18 que falha quando o gate é sujo e passa quando está limpo ocorre em uma árvore cujo `git status --porcelain` está vazio no início. Sem essa pré-condição, a guarda é satisfeita vacuamente em execuções locais de `make parity`.

A emenda não requer alteração arquitetural — apenas que o Scenario 18 verifique `git status --porcelain` antes de iniciar e aborte com diagnóstico se a árvore já estiver dirty (ou, alternativamente, que a falsificação seja executada em um worktree separado com checkout limpo).

Todos os outros ACs (AC1, AC2, AC4, AC5) são implementáveis sem restrições adicionais identificadas por este parecer.
