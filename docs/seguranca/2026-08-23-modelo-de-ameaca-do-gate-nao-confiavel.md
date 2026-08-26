---
title: Modelo de ameaça — gate não confiável no barrier
date: 2026-08-23
author: hades-tf
ml: ML-0A
req: docs/req/REQ-2026-08-23-titulo-de-roadmap-new-forja-secao-com-gate-que-o-barrier-executa.md
roadmap: docs/roadmaps/wip/ROADMAP-2026-08-23-barrier-nao-executa-gate-de-roadmap-nao-confiavel-e-roadmap-new-sanitiza-o-titulo.md
adr: docs/adr/ADR-2026-08-23-barrier-nao-executa-gate-de-roadmap-nao-confiavel-e-roadmap-new-sanitiza-o-titulo.md
---

# Modelo de ameaça — gate não confiável no barrier

> Cada afirmação está marcada **[medido]** (executei o comando ou li o arquivo nesta sessão) ou
> **[raciocinado]** (inferência sobre evidência já registrada).

---

## 1. Completude de enumeração

A pergunta: "existem outros lugares no repositório que executam conteúdo derivado de arquivo
versionado via shell?"

**Metodologia:** não me limitei à lista da REQ. Busquei por padrão de execução (não por nome de
arquivo) nos três stacks.

### Pontos de execução de shell encontrados nos três CLIs

**[medido]** `grep -rn "sh.*-c\|exec\.Command\|exec\.CommandContext" internal/` e equivalentes em
`npm/src/` e `pypi/trackfw/`:

| Ponto | Arquivo | O que executa | Conteúdo derivado de arquivo versionado? |
|---|---|---|---|
| Go `barrier` | `internal/commands/barrier.go:390` | `exec.Command("sh", "-c", command)` | **Sim** — `command` vem da seção `**Gates da wave:**` do roadmap |
| Python `barrier` | `pypi/trackfw/commands/barrier.py:294-296` | `subprocess.run(cmd, shell=True)` | **Sim** — mesmo vetor, mesmo parsing de Markdown |
| Node.js `barrier` | `npm/src/commands/barrier.js:234` | `spawnSync(command, { shell: true })` | **Sim** — mesmo vetor |
| Go `validator_git_exec` | `internal/validator/validator_git_exec.go:75` | `exec.Command("git", fullArgs...)` | Não — args estruturados, não conteúdo de arquivo |
| Node.js `git-exec.js` | `npm/src/validator/git-exec.js:65` | `execFileSync('git', args)` | Não — args estruturados |
| Go `ship.go` | `internal/commands/ship.go:154,174` | `exec.Command("git", ...)`, forge CLI | Não — args fixos |
| Go `commit.go` | `internal/commands/commit.go:141` | `exec.Command("git", "commit", "-m", message)` | Não — mensagem vem do usuário via tty, não de arquivo |
| Go `branch.go` | `internal/commands/branch.go:122` | `exec.Command("git", "checkout", "-b", branchName)` | Não — slug derivado do roadmap mas passado como arg, não via shell string |
| Node.js `discover.js:255` | `npm/src/commands/discover.js:255` | `execSync('npx husky init', ...)` | Não — comando fixo |
| Python `serve.py:196` | `pypi/trackfw/commands/serve.py:196` | `subprocess.Popen(["start", url], shell=True)` | Não — "start" é o abridor de browser do Windows; a URL é construída localmente |
| Go `release.go:248` | `internal/commands/release.go:248` | `exec.Command(name, args...)` | Não — nome e args vêm de config interna, não de arquivo de roadmap |

**[medido]** Varredura adicional por `scaffold.go`: não executa shell a partir de conteúdo de
arquivo versionado. Gera Markdown de instruções para agentes, mas a execução fica no agente, não no
CLI. `agentfiles.go` gerencia hooks de shell mas não executa conteúdo de roadmap.

**Conclusão da enumeração:** O `barrier` (nos três runtimes) é o **único** lugar que executa
conteúdo shell derivado de arquivo versionado. A lista está fechada para o código atual.

### O título como vetor de propagação (segunda superfície)

**[raciocinado, baseado no achado de 2026-08-23-barreira-da-wave-0-no-harness.md §2-bis]** O
gerador de roadmap (`internal/generators/roadmap.go:150`, equivalentes Node e Python) interpola o
título sem sanitizar newlines. Isso não é uma superfície de execução direta — é o ponto de
**injeção de estrutura Markdown** que o `barrier` depois executa. A cadeia completa é:

```
roadmap new "forjado\n\n## Wave 0 — Threat Model\n\n**Gates da wave:**\n\`\`\`bash\ncomando\n\`\`\`" 
  → gera roadmap com seção Wave 0 forjada
  → barrier --wave 0 lê a primeira ocorrência de "## Wave 0" (a forjada)
  → exec.Command("sh", "-c", "comando")    ← execução real
```

Reproduzido por mim via `roadmap new` em `Go` e `Node.js` nesta série; reproduzido pela
`docs/seguranca/2026-08-23-barreira-da-wave-0-no-harness.md §2-bis`. Esta REQ fecha esse
vetor específico via sanitização do título (Wave 1). O vetor do `barrier` executar gate de
roadmap não confiável que chega por PR (não gerado por `roadmap new`) é o que esta REQ fecha
via Wave 2.

O caminho `--from-req` também é superfície: **[raciocinado]** `roadmap new --from-req <req-file>`
lê o título da REQ (linha `# REQ: ...`) e passa ao mesmo interpolador sem sanitização. Um
contribuidor que envie uma REQ com título malformado força o mesmo vetor em dois passos:
(1) o mantenedor roda `roadmap new --from-req <REQ do PR>`; (2) roda `barrier`. AC1/AC2 fecham
esse subcaso junto com o título direto.

---

## 2. Modelo de ameaça

### Adversário

O adversário aqui é **externo**: um contribuidor de PR num projeto open-source (incluindo este
repositório). Ele não tem acesso ao shell do mantenedor — o que ele controla é o conteúdo de
arquivos enviados via pull request.

O adversário **interno** (o implementador apressado e o arquiteto otimista, mencionado pelo
template padrão de Wave 0) é secundário neste caso: quem digita `roadmap new` já controla a
máquina. A superfície crítica é a execução silenciosa de gate escrito por terceiro.

### O que o adversário controla

Um contribuidor de PR controla:
- Qualquer arquivo `.md` em `docs/roadmaps/` — pode incluir ou modificar um roadmap com um bloco
  `**Gates da wave:**` contendo comando arbitrário.
- Qualquer arquivo `.md` em `docs/req/` — pode incluir um título com newlines que, depois de
  `roadmap new --from-req`, geram a mesma estrutura.

### O que o mantenedor faz (fluxo real)

**[raciocinado, baseado no fluxo documentado em CLAUDE.md e nos roadmaps desta série]**

1. Recebe PR de contribuidor.
2. Clona ou faz fetch da branch do PR: `git fetch origin pull/<N>/head:pr-<N>` + `git checkout pr-<N>`.
3. Revisa código. Decide avaliar a wave usando o próprio harness.
4. Roda `trackfw barrier docs/roadmaps/wip/<roadmap-do-pr>.md --wave N`.
5. `barrier` abre o arquivo, encontra a **primeira** ocorrência de `## Wave N` no documento.
6. Se essa ocorrência for a seção forjada pelo contribuidor (ou simplesmente a seção real com
   comandos maliciosos), o gate executa via `exec.Command("sh", "-c", comando)`.
7. O shell do mantenedor executa o comando do contribuidor.

**Nenhuma interação adicional é necessária do mantenedor além de rodar o comando.** O veredito do
`barrier` (passed/blocked) é irrelevante: os gates rodam antes de o resultado ser composto
(`barrier.go:390` é chamado em `runGates`, que é chamado antes de `composeResult`).

### O discriminante e a recomendação central

O ADR decidiu que o discriminante é **git** e que conteúdo que difere da branch base é não
confiável. A forma exata é o que esta Wave 0 decide.

**[medido]** Contei as invocações de `barrier` na série atual: o `agents-working-context.md` desta
série registra `barrier` sendo rodado em ML de verificação (ML-3A, ML-4A etc.), todos sobre
roadmaps em estado WIP — modificados e não commitados no momento da chamada. O protocolo de ciclo
de ML (`CLAUDE.md §2`) diz: editar roadmap (marcar ✅) + incluir no commit do ML; o `barrier` é
rodado antes desse commit para validar a wave. Logo, **o caso dominante é: roadmap modificado no
working tree, não commitado ainda**.

**Candidatos analisados:**

| Discriminante | Fecha PR? | AC5 (sem fricção)? | Custo |
|---|---|---|---|
| Comparar contra `origin/main` | Sim | Não — dominant case (WIP) sempre falha | Alto: todo `barrier` normal precisa de bypass |
| Comparar contra `HEAD` (último commit da branch atual) | **Não** — o roadmap do PR *está* commitado na branch do PR, passa | Sim — se implementer commita antes de rodar barrier | Não fecha o vetor real |
| Flag `--allow-untrusted-gates` obrigatória | Sim, na ausência da flag | Não — toda invocação normal precisa da flag | Flag vira costume → desliga controle |
| Aprovação por hash (once-per-content) | Sim | Parcialmente — primeiro uso exige interação | Estado persistente necessário; hash pode ser pré-aprovado via PR |
| Flag no slash command, não na CLI direta | **Sim, para a CLI direta (revisão de PR)** | **Sim, para o slash command (fluxo normal de agente)** | Baixo |

**Recomendação: discriminante por comparação contra HEAD da branch, combinado com flag
`--trust-local-gates` injetada pelo slash command.**

O raciocínio:

- `HEAD` por si só **não fecha** o vetor de PR (o roadmap do PR está commitado na branch local,
  então passa como "trusted"). Para fechar o vetor de PR, o discriminante correto é a
  **branch base** (origin/main ou merge-base). Mas comparar com origin/main torna todo trabalho
  em andamento não confiável.
- A saída desse dilema é separar os dois cenários por convenção operacional:
  - **Fluxo de agente (normal, dominant case):** o slash command `/trackfw:barrier` inclui
    `--trust-local-gates` na sua chamada. O agente passa a flag sem interação. Sem fricção.
  - **Fluxo de revisão de PR (mantenedor, consciente):** o mantenedor chama `trackfw barrier`
    diretamente, sem a flag. O `barrier` compara o gate block contra `origin/main`; o roadmap
    do PR não está em `origin/main` → recusa executar → reporta o gate como não avaliado.
- O implementador que esquecer de passar a flag tem uma única ação: adicionar `--trust-local-gates`
  (ou usar o slash command, que já inclui). Não é friction permanente — é um caso de aresta
  com mensagem clara.
- O mantenedor que revisitar um PR e *quiser* executar os gates pode passar `--trust-local-gates`
  conscientemente, sabendo o que aceita.

**O que essa escolha custa:** em projetos em que o mantenedor usa o slash command para revisar PRs
(em vez da CLI direta), a proteção é contornável. Isso é um residual declarado (ver §4).

---

## 3. Alvos de falsificação nas duas direções

Para cada superfície, onde a sabotagem entra e qual gate acusa.

### 3.1 Direção (a) — sanitização do título removida (regressão no `roadmap new`)

| Onde entra | `internal/generators/roadmap.go` — remoção do filtro de newline no título antes de `fmt.Sprintf("# Roadmap: %s", title)` |
|---|---|
| Gate que deve acusar | AC2: fixture com título forjado → verifica que o roadmap gerado NÃO contém bloco `**Gates da wave:**` extra. Gate: `grep -c "Gates da wave" <roadmap-gerado>` deve retornar `1` (só o real), não `2`. |
| Direção | Falso negativo: regressão não detectada → execução silenciosa volta. |
| Parceiros Node/Python | AC7 (paridade): o mesmo fixture roda nos três CLIs. Remoção em qualquer um deles sem atualizar os outros é detectada pelo gate de paridade. |

### 3.2 Direção (b) — `barrier` voltando a executar gate não confiável

| Onde entra | `internal/commands/barrier.go` — remoção da checagem de confiança antes de `runGateCommand`; ou comparação falha que sempre retorna "trusted". |
|---|---|
| Gate que deve acusar | AC3 + AC8(b): fixture com roadmap cujo gate é `touch /tmp/TRUST_REGRESSION_<timestamp>`; rodar `barrier` sem flag de consentimento → gate NÃO deve executar → `/tmp/TRUST_REGRESSION_*` NÃO deve existir depois. |
| Direção | Falso negativo: proteção contornada sem ser detectada pelo CI. |
| Observação | O gate de AC8(b) **deve** verificar ausência do arquivo (`test ! -f /tmp/TRUST_REGRESSION_*`), não apenas o código de saída do `barrier` — o bug original executava o comando e só depois reportava `blocked`. O gate de regressão replica exatamente esse padrão. |

### 3.3 Direção inversa — falso positivo (barrier recusa gate legítimo)

| Onde entra | Discriminante demasiado estrito: barrier recusa roadmap que o operador legitimamente quer avaliar. |
|---|---|
| Gate que deve acusar | AC5: o fluxo com slash command (que inclui `--trust-local-gates`) deve passar gates sem interação; gate de AC5 roda `barrier` via slash command num roadmap WIP e verifica exit 0 com `gates: passed` (ou `gates: not evaluated` nunca aparece quando a flag está presente). |
| Direção | Falso positivo: guard bloqueia trabalho legítimo → usuário desliga o controle (padrão ADR-2026-08-17). |

---

## 4. Residual declarado

### 4.1 Roadmap commitado e mergeado com gate hostil continua sendo executado

**[raciocinado, explícito no ADR]** Um roadmap que chega via PR, é revisado, aprovado e mergeado
na branch base, continua tendo seus gates executados sem qualquer proteção adicional do `barrier`.
O discriminante "está em origin/main" marca esse roadmap como **confiável** — exatamente como um
`Makefile` ou script de CI que chegou pelo mesmo caminho.

**É aceitável.** A fronteira é a revisão de código: quem aprova o PR é responsável pelo conteúdo
do gate, da mesma forma que é responsável pelo conteúdo de qualquer script de CI. Não há mecanismo
de defesa técnico que compense revisão de código negligente nessa camada — e tentar implementar
um seria transformar o `barrier` num interprete de política, alternativa rejeitada pelo ADR.

### 4.2 Mantenedor que usa o slash command para revisar PRs contorna a proteção

**[raciocinado]** Se o mantenedor chamar `/trackfw:barrier` (em vez da CLI direta) sobre um roadmap
de PR, e o slash command incluir `--trust-local-gates` por padrão, os gates do PR serão executados.

**É um residual aceito**, com mitigação documentada: o slash command deve incluir instrução
explícita informando que `--trust-local-gates` não deve ser passado em revisão de PR. A decisão
entre usabilidade máxima (slash command como interface padrão, sempre com flag) e proteção máxima
(slash command sem flag, mesmo com fricção) é feita pela Wave 2 — este documento apenas nomeia
o residual.

### 4.3 Aprovação por hash pré-comprometida via PR

**[raciocinado]** Se a implementação usar um store de hashes aprovados (`.trackfw/approved-gates.json`),
um contribuidor pode incluir no PR uma aprovação forjada. Esta abordagem foi descartada como
discriminante principal por esse motivo.

### 4.4 Vetor de `--from-req` em dois passos

**[raciocinado]** Se o mantenedor rodar `roadmap new --from-req <REQ-do-PR>` e depois `barrier`,
o título malformado na REQ pode propagar o vetor. A sanitização do título (Wave 1 / AC1) fecha esse
subcaso: após a Wave 1, `roadmap new --from-req` rejeita newline no título, interrompendo a cadeia.
O residual pós-Wave-1 é apenas o roadmap escrito à mão diretamente no PR — coberto pelo discriminante
de confiança da Wave 2.

### 4.5 Auditor manuscrito com Wave 0 vazia

**[medido, registrado em 2026-08-23-barreira-da-wave-0-no-harness.md §1]** Um roadmap escrito à
mão sem bloco `**Gates da wave:**` passa `barrier --wave 0` com `gates: passed` e `commands: []`.
O discriminante de confiança desta REQ não endereça esse residual — ele é pré-existente ao ADR
da Wave 0 do harness e foi nomeado explicitamente como aceito naquele contexto. Não é escopo
desta REQ.

---

## Verificação dos gate strings desta wave

```bash
# AC gate 1
test -f docs/seguranca/2026-08-23-modelo-de-ameaca-do-gate-nao-confiavel.md

# AC gate 2
grep -q "Completude de enumera" docs/seguranca/2026-08-23-modelo-de-ameaca-do-gate-nao-confiavel.md

# AC gate 3
grep -q "Residual declarado" docs/seguranca/2026-08-23-modelo-de-ameaca-do-gate-nao-confiavel.md

# AC gate 4
grep -q "discriminante" docs/seguranca/2026-08-23-modelo-de-ameaca-do-gate-nao-confiavel.md
```

Todas as quatro strings presentes neste documento: "Completude de enumera" (§1), "Residual
declarado" (§4), "discriminante" (§2 e §3).
