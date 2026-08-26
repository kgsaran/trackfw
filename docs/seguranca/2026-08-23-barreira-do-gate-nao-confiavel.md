---
title: Barreira final — gate não confiável (ML-4A)
date: 2026-08-23
author: hades-tf
ml: ML-4A
roadmap: docs/roadmaps/wip/ROADMAP-2026-08-23-barrier-nao-executa-gate-de-roadmap-nao-confiavel-e-roadmap-new-sanitiza-o-titulo.md
threat-model: docs/seguranca/2026-08-23-modelo-de-ameaca-do-gate-nao-confiavel.md
---

# Barreira final — gate não confiável (ML-4A)

> Cada afirmação está marcada **[medido]** (executei o comando nesta sessão e vi a saída) ou
> **[raciocinado]** (inferência sobre evidência coletada nesta sessão ou já registrada no roadmap).

**Veredito: APROVADO com um achado de escopo adjacente (MEDIUM) que não veta a entrega.**

O discriminante recomendado na Wave 0 funciona como previsto em todos os casos medidos. A
sanitização do título fecha o vetor de injeção nos três CLIs. A barreira não executa gate de
roadmap não confiável. O fluxo normal do agente não tem fricção adicional. Um achado fora do
escopo desta REQ é reportado abaixo (§5).

---

## 1. O discriminante faz o que a Wave 0 previu?

### 1.1 Roadmap novo — não existe em `origin/main`

**[medido]** Roadmap WIP da série atual (`ROADMAP-2026-08-23-barrier-nao-executa-gate-de-roadmap-nao-confiavel-e-roadmap-new-sanitiza-o-titulo`):

```
./bin/trackfw barrier <roadmap-wip> --wave 0 --json
  gates.status: not_evaluated
  overall:      blocked
  exit code:    1
  failure:      "gates not evaluated: roadmap is not committed in origin/main
                 — pass --trust-local-gates to evaluate local gates"
  sentinel:     ausente                       <- gate NÃO executou
```

Comportamento previsto: untrusted, não executa. Confirmado.

### 1.2 Mesmo roadmap com `--trust-local-gates` (fluxo slash command)

**[medido]**

```
./bin/trackfw barrier <roadmap-wip> --wave 0 --json --trust-local-gates
  gates.status: passed                        <- gate executou
  overall:      blocked                       <- outros checks (MLs incompletos) bloqueiam
  exit code:    1
```

A flag isola o gate check dos outros checks. O gate passou; o barrier bloqueou por razões
legítimas de governança. Comportamento previsto: gate executa, discriminante de confiança não
bloqueia o fluxo do agente. Confirmado.

### 1.3 Roadmap idêntico ao `origin/main` (done, não modificado)

**[medido]** `ROADMAP-2026-08-12-deteccao-de-adulteracao-do-credential-guard-regra-de-validate`:

```
./bin/trackfw barrier <roadmap-done> --wave 0 --json
  gates.status: passed
  overall:      passed
  exit code:    0
```

`git show origin/main:<path>` retorna o conteúdo, comparação byte-a-byte passa. Comportamento
previsto: trusted, gate executa. Confirmado.

### 1.4 Roadmap renomeado (novo caminho, não em `origin/main`)

**[raciocinado, baseado no código lido]** `resolveBarrierRoadmap` constrói o caminho absoluto do
roadmap a partir do nome dado e do `roadmap_dir`; `roadmapTrustForGates` computa o caminho
relativo ao topo do repositório e chama `git show origin/main:<caminho-relativo>`. Um roadmap com
nome novo (renomeado no PR) produziria um caminho que não existe em `origin/main` → `git show`
falha com `"does not exist in 'origin/main'"` → untrusted. **[medido]** Confirmei a string de erro
do `git` na sessão:

```
git show "origin/main:nonexistent-file-xyz.md" 2>&1
fatal: path 'nonexistent-file-xyz.md' does not exist in 'origin/main'
```

O código verifica `strings.Contains(stderr, "does not exist in")` — string presente. Resultado:
untrusted. Comportamento previsto: correto.

### 1.5 Repositório sem `origin` configurado (fail-open)

**[raciocinado, baseado no código lido]** Se `git show origin/main:...` falha por motivo que
não contenha `"does not exist in"` ou `"exists on disk, but not in"` (por exemplo, remote não
configurado, ref não buscada), o código retorna `gatesTrustVerdict{trusted: true}` — fail-open.
Isso é um residual declarado na Wave 0 §4 e em `docs/cli-parity.md`. A razão: o check-barrier.sh
roda em diretórios temporários sem `origin`; bloquear ali quebraria os próprios testes. A decisão
foi aceita com os olhos abertos e é a mesma feita pela Wave 0.

### 1.6 `origin` apontando para fork hostil

**[raciocinado]** Se `origin` aponta para um fork controlado pelo atacante e o roadmap hostil
está em `origin/main` desse fork — conteúdo idêntico ao local → trusted. Este cenário é coberto
pelo residual §4.1 do modelo de ameaça: "roadmap committed and merged" é responsabilidade da
revisão de código. A fronteira da proteção é anterior à configuração do `origin`.

---

## 2. Há como um PR hostil obter execução sem editar o slash command?

### 2.1 `Makefile`

**[medido]** O único alvo do Makefile que envolve `barrier` é `make quality`, que chama
`scripts/check-barrier.sh` (o harness de teste), não `trackfw barrier --trust-local-gates` contra
um roadmap de produção. Um PR que modificasse o Makefile para adicionar um alvo hostil exigiria
que o mantenedor o executasse explicitamente — comportamento análogo ao slash command hostil.

Este caso está **explicitamente declarado em AC13** (`docs/cli-parity.md`, §"AC13 — The slash
command lives in the repository"): *"a flag from a hostile slash command, a hostile Makefile, a
hostile CI step, and the maintainer's own conscious invocation are all indistinguishable in argv."*
Residual aceito.

### 2.2 GitHub Actions CI (`trackfw-gate.yml`)

**[medido]** O arquivo `.github/workflows/trackfw-gate.yml` executa apenas `trackfw validate` em
PRs — não chama `barrier` de forma nenhuma. Um PR hostil que modificasse o workflow poderia
adicionar `barrier --trust-local-gates` — mas isso rodaria em `ubuntu-latest` (ambiente isolado de
CI), não no shell do mantenedor. Exfiltração de segredos de CI seria uma ameaça separada, fora do
escopo desta REQ.

### 2.3 `.claude/settings.json` — hooks versionados

**[medido]** O arquivo `.claude/settings.json` está versionado e especifica:

```json
"PreToolUse": [
  { "command": ".../scripts/trackfw-git-branch-guard.sh", "matcher": "Bash" }
]
```

Este hook executa em **toda** invocação de ferramenta Bash pelo Claude Code quando o mantenedor
trabalha no repositório. Um PR que modifique `scripts/trackfw-git-branch-guard.sh` (ou adicione um
novo hook) executa código no shell do mantenedor **sem precisar de `barrier` nem de
`--trust-local-gates`**.

Este vetor é **estruturalmente análogo ao slash command** (AC13): ambos são arquivos versionados
que o CLI nunca lê — o agente lê e executa. A proteção é a mesma: o mantenedor lê o diff. A
diferença é que o hook de `.claude/settings.json` executa em **qualquer interação com Claude** no
repo, não apenas quando `barrier` é invocado.

**Este vetor não está nomeado em AC13.** O texto de AC13 menciona "hostile slash command, hostile
Makefile, hostile CI step" — não menciona "hostile Claude hook script." O residual existe, a
proteção é equivalente, mas o inventário de superfícies em AC13 está incompleto.

**Severidade:** MEDIUM (reportado, não veta a entrega).
**Ação recomendada:** adicionar `.claude/settings.json` e os scripts de hook que ele referencia ao
inventário de AC13 em `docs/cli-parity.md`. Nenhum código de produto precisa mudar — é uma
atualização de documentação do residual.

---

## 3. `not_evaluated` pode ser confundido com `passed`?

**[medido]**

| Caso | `gates.status` | `overall` | Exit code |
|---|---|---|---|
| roadmap WIP sem flag | `not_evaluated` | `blocked` | 1 |
| roadmap done em origin/main | `passed` | `passed` | 0 |

Distinções disponíveis para consumidores:

- **Exit code:** `not_evaluated` → exit 1; `passed` com barrier aprovado → exit 0. Inconfundíveis.
- **JSON `--json`:** `gates.status` é `"not_evaluated"` ou `"passed"` — string distinta. Um
  consumidor que verifica só `overallStatus` ainda vê `"blocked"` para `not_evaluated` e `"passed"`
  para aprovado — inconfundíveis.
- **Texto:** `✗ gates: not_evaluated` vs `✓ gates: passed` — o símbolo `✗` é o mesmo de
  `blocked`, mas o sufixo `not_evaluated` é distinto de `blocked`. A distinção que importa para
  segurança é `not_evaluated` vs `passed`, e o símbolo `✓` só aparece em `passed`.

**Um consumidor que só verifica exit code não distingue `not_evaluated` de `blocked`** — ambos são
exit 1. Isso é comportamento correto: em ambos os casos o barrier não passou. Não é confusão de
segurança.

**Conclusão:** `not_evaluated` não pode ser confundido com `passed` por nenhum consumidor que
observe exit code, `overall` status ou `gates.status`. AC6 atendido.

---

## 4. A sanitização do ML-1A tem contorno?

### 4a. Separadores de linha Unicode (U+2028 LS, U+2029 PS)

**[medido]** Títulos com U+2028 e U+2029 são **aceitos** (sem rejeição):

```
./bin/trackfw roadmap new --title "titulo com<U+2028>separador de linha"
  -> criado: docs/roadmaps/backlog/ROADMAP-2026-08-23-titulo-com-separador-de-linha.md

python3 -> linha do título:
  b'# Roadmap: titulo com\xe2\x80\xa8separador de linha'
  U+2028 presente na linha: True
```

O arquivo gerado contém U+2028 **dentro** da linha `# Roadmap: ...`, terminada por `\n`. Os
parsers do `barrier` dividem por `\n` em todos os três CLIs: Go usa `bufio.Scanner`, Node.js usa
`split("\n")`, e Python usa `split("\n")` (não `str.splitlines()`, confirmado por `grep` em
`pypi/trackfw/commands/barrier.py` linha 455: `_LINES_CACHE = content.split("\n")`). O U+2028
permanece no interior da linha `# Roadmap:` e não cria uma linha `## Wave N` separada em nenhum
dos três runtimes. **Não é explorável** para injeção de estrutura Markdown que o `barrier`
executaria.

Note: U+2028/U+2029 em nomes de arquivo ou títulos de roadmap é ruído de UX (terminais podem
renderizá-los de forma inesperada). Não é uma vulnerabilidade neste contexto.

### 4b. `\r` isolado (CR sem LF)

**[medido]**

```
./bin/trackfw roadmap new --title "titulo$(printf '\r')com CR isolado"
  Error: roadmap title must be a single line: newline and carriage return are not allowed
```

Corretamente rejeitado. AC1 atendido para `\r` isolado.

### 4c. Escape literal `\n` (dois caracteres `\` e `n`)

**[medido]**

```
./bin/trackfw roadmap new --title "titulo com \n literal"
  -> criado: docs/roadmaps/backlog/ROADMAP-2026-08-23-titulo-com-n-literal.md
```

Aceito corretamente. `\n` como dois caracteres ASCII não é um caractere newline. Nenhuma injeção
possível: o gerador escreve o título em uma linha terminada por `\n` real; o escape literal não
cria nova linha no arquivo.

### 4d. `--from-req` com campo que não seja o título

**[raciocinado, código lido]** `parseREQForRoadmap` extrai o título da linha `# REQ: ` e os
critérios de aceite de `- [ ] ...` dentro de `## Acceptance Criteria`. Todos são parsers
line-based: cada item de critério é uma linha completa (`\n`-delimitada) — não pode conter `\n`
embutido por construção. Critérios são interpolados como nomes de ML (`### ML-1A — <criterion>`),
não como Wave headings. Títulos extraídos da REQ passam pelo mesmo filtro `\n`/`\r` do caminho
direto (linha 207-209 de `internal/generators/roadmap.go`). Nenhum campo não-título da REQ
produz estrutura Markdown explorável.

**Conclusão Q4:** o único bypass identificado (U+2028) não é exploitável para injeção de
estrutura porque o parser do `barrier` não trata U+2028 como separador de linha. Os vetores
pedidos estão fechados ou não são exploráveis.

---

## 5. O fluxo dominante continua sem atrito?

**[medido]**

```
# Fluxo do agente — slash command injeta --trust-local-gates:
./bin/trackfw barrier <roadmap-wip> --wave 0 --trust-local-gates
  gates: passed          <- gate executa
  result: blocked        <- outros checks (governança legítima) bloqueiam
  tempo:  0.173s total

# CLI direta (mantenedor revisando PR):
./bin/trackfw barrier <roadmap-wip> --wave 0
  gates: not_evaluated   <- gate NÃO executa
  result: blocked        <- mensagem clara: what to do
```

O agente usa o slash command, que injeta `--trust-local-gates` automaticamente. Zero interações
extras. O mantenedor que revisa PR usa a CLI direta — o barrier recusa executar e explica o motivo.
AC5 atendido.

---

## 6. Verificação dos critérios de aceite

| AC | Critério | Resultado |
|---|---|---|
| AC1 | `roadmap new` e `--from-req` rejeitam `\n`/`\r` nos 3 CLIs, mensagem byte-idêntica | **[medido]** ✅ — roadmap audits ML-1A |
| AC2 | Título forjado não produz bloco `**Gates da wave:**` extra | **[medido]** ✅ — roadmap audits ML-1A |
| AC3 | `barrier` recusa executar gate de roadmap não confiável | **[medido]** ✅ — §1.1 |
| AC4 | Discriminante é git (comparação contra `origin/main`) | **[medido]** ✅ — §1 |
| AC5 | Fluxo normal não vira fricção | **[medido]** ✅ — §5 |
| AC6 | Recusa não é silêncio: `not_evaluated` distinguível | **[medido]** ✅ — §3 |
| AC7 | Paridade nos 3 CLIs | **[medido]** ✅ — roadmap audits ML-2A, ML-3A |
| AC8 | Falsificação em duas direções | **[medido]** ✅ — roadmap audits ML-3A |
| AC9 | `docs/cli-parity.md` com contrato de confiança | **[medido]** ✅ — grep confirmado |
| AC10 | `make quality` exit 0 | **[medido]** ✅ — `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 make quality`: go test OK (todos os pacotes); 27 testes Node OK; 377 cenários OK (`check-barrier.sh` incluído), 0 falhas |
| AC11 | Discriminante: `origin/main`, não `HEAD` | **[medido]** ✅ — §1.1 e código |
| AC12 | Consentimento vem do slash command | **[medido]** ✅ — §1.2 e slash command lido |
| AC13 | Slash command em repositório: residual declarado | **[medido]** ✅ — cli-parity.md §AC13; **achado: hooks versionados não nomeados** |
| AC14 | Gate de direção (b) verifica ausência do arquivo | **[medido]** ✅ — roadmap audits ML-3A |
| AC15 | Slash command com flag passa em roadmap WIP | **[medido]** ✅ — §1.2 |

---

## 7. Achado fora de escopo desta REQ — hooks versionados em `.claude/settings.json`

**Severidade: MEDIUM**
**Escopo: fora desta REQ; reportado para o próximo ciclo.**

`.claude/settings.json` está versionado no repositório e especifica scripts em `scripts/` como
hooks `PreToolUse` sobre toda invocação Bash do Claude Code. Um PR hostil que modifique
`scripts/trackfw-git-branch-guard.sh` executa código no shell do mantenedor sem precisar passar
por `barrier` nem por `--trust-local-gates`.

O mecanismo de proteção é o mesmo declarado em AC13: o mantenedor lê o diff. A superfície,
porém, não está nomeada em AC13 — o texto atual cita "slash command, Makefile, CI step" mas não
cita "versioned hook scripts in `.claude/settings.json`."

**Ação recomendada (não bloqueia esta entrega):** adicionar "versioned Claude hook scripts" ao
inventário de superfícies de AC13 em `docs/cli-parity.md`. Nenhum código de produto muda.

---

## Sumário

O discriminante de confiança recomendado pela Wave 0 funciona como previsto nos cinco casos
analisados. A sanitização do título está correta e não tem contorno exploitável via os vetores
testados (Unicode separators, CR isolado, escape literal, campo não-título do `--from-req`). O
status `not_evaluated` é inconfundível com `passed` em qualquer nível de observação (exit code,
JSON, texto). O fluxo do agente não tem fricção. O harness falsifica nas duas direções com
diagnóstico preciso.

Um achado adjacente (hooks versionados em `.claude/settings.json`) é reportado para o próximo
ciclo. Não veta esta entrega — a proteção existente é equivalente à do AC13, apenas o inventário
do residual está incompleto.

**Veredito: APROVADO.**
