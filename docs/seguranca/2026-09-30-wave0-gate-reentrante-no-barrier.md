---
date: 2026-09-30
roadmap: "ROADMAP-2026-09-30-gate-de-wave-que-reentra-no-barrier-recursa-sem-limite-e-um-roadmap-vira-fork-bomb.md"
ml: ML-0A
agente: hades-tf
---

# Modelo de Ameaça — Gate Reentrante no `barrier`

> ML-0A · Wave 0 · 2026-09-30

---

## 1. Completude de enumeração

**Pergunta:** quais caminhos do produto executam conteúdo de roadmap? A lista é fechada?

### Método

Busca de `runGateCommand`, `evalGateCommands`, `parseGates`, `sh -c`, e todo `exec.Command` em
`internal/commands/` (excluindo `_test.go`), seguida de leitura de `roadmap.go`, `commit.go`,
`ship.go`, `validate.go` e do slash command gerado.

### Executores identificados

| Caminho | Arquivo:linha | Executa gate? | Nota |
|---|---|---|---|
| `runGateCommand` | `barrier.go:371–381` | **SIM** | `sh -c` arbitrário por desenho |
| `evalGateCommands` | `barrier.go:389–407` | **SIM** | chama `runGateCommand` em loop |
| `/trackfw:barrier` slash command | `.claude/commands/trackfw/barrier.md:13` | indiretamente | instrui LLM a chamar `trackfw barrier --trust-local-gates`; não executa diretamente, delega ao binário |
| `roadmap move` | `roadmap.go:145–` | **não** | apenas move arquivo e atualiza frontmatter |
| `commit` | `commit.go` | **não** | git commit + push; sem execução de gate |
| `ship` | `ship.go` | **não** | governance check + push; sem execução de gate |
| `validate` | `validate.go` | **não** | inspeção estática; sem `sh -c` |
| `check-barrier.sh` | `scripts/check-barrier.sh:181–188` | sobre fixtures | não é executor de produto; é gate/script de teste |
| `check-roadmap-barrier-contract.sh` | `scripts/check-roadmap-barrier-contract.sh` | sobre fixtures | idem |
| `barrier_contract_test.go` | `~:258 runBarrierCLI` | sobre fixtures | chama binário como subprocess, sem `cmd.Env` |

**Conclusão:** o único executor de gate de produto é `barrier.go:371 runGateCommand`, acessado
exclusivamente por `evalGateCommands`, invocado por `runBarrier`. Nenhum outro comando do produto
executa conteúdo de roadmap via `sh -c`. A enumeração está fechada.

**Consequência para AC5:** se apenas `barrier` executa gates, não há outro executor que precise
da mesma contenção de reentrada como produto — qualquer script que chame `barrier` sobre fixture
(check-barrier.sh, testes) opera em fixtures independentes, não no mesmo roadmap/wave.

### Verificação do slash command

O `/trackfw:barrier` chama `trackfw barrier <roadmap> --wave <n> --trust-local-gates --json`
(`barrier.md:13`). A execução de gate acontece dentro do binário, não no slash command. A contenção
de reentrada precisa estar no binário.

---

## 2. Modelo de ameaça

**Adversário:** o implementador apressado e o arquiteto otimista — quem escreve um gate natural mas
recursivo, não um atacante externo.

### Superfície

O bloco `**Gates da wave:**` é conteúdo de arquivo de texto lido e executado linha a linha via
`sh -c`. Não há validação do conteúdo das linhas além de parsing de cerca (fenceMask). O trust check
(`roadmapTrustForGates`) protege contra execução de gates de roadmaps *chegados por PR de terceiro*,
mas não contra recursão em roadmap do próprio repositório — o `origin/main` que prova a confiança
já contém o gate problemático.

### Cenário de ameaça principal

1. Autor escreve na Wave N de um roadmap: `trackfw barrier <este-roadmap> --wave N` como gate.
   Intenção: "o gate desta wave é passar na barreira". Erro natural.
2. `barrier` lê o gate, chama `runGateCommand`, que executa `sh -c "trackfw barrier ..."`.
3. O processo filho chama `runBarrier` com o mesmo roadmap e wave.
4. O filho lê o mesmo gate e inicia o neto. Sem limite: cadeia linear de PIDs.
5. Medido: 3469 processos em ~6 min, load 12.9. Contenção: `pkill -9 -x trackfw`.

### Quem aciona

- Qualquer autor que escreva o gate: direto (`trackfw barrier roadmap.md --wave N`),
  via script (`make wave-check` que chama barrier), via outra ferramenta.
- Contexto de CI: o acervo tem **4 roadmaps `done/`** com `make quality` em bloco de gate
  (10 blocos de gate no total — ROADMAP-2026-07-29-barrier-governanca tem 6 waves com este
  gate; medição: Python, extração do 1º bloco cercado após `**Gates da wave:**`, 2026-09-30).
  `make quality` → `parity` → `scripts/check-barrier.sh` → `trackfw barrier fixture
  --trust-local-gates`. Se `make quality` for gate de um roadmap e `check-barrier.sh`
  for modificado para chamar barrier no roadmap do usuário (hipotético), a reentrada ocorre
  por indireção. Na configuração atual, check-barrier.sh só chama barrier sobre fixtures
  — não é vulnerabilidade ativa, mas mostra o caminho de indireção.

### O que NÃO é esta ameaça

O gate executa `sh -c` arbitrário por desenho. Um gate hostil pode ser `:(){ :|:& };:` sem
precisar de recursão. O trust check não protege clone hostil (o `origin/main` do clone já
contém o gate). Esta REQ não promete sandbox. O defeito aqui é de **robustez**, não de
confinamento: um gate **benigno e bem-intencionado** se amplifica sem limite.

---

## 3. Alvos de falsificação em ambas as direções

Para cada candidato de discriminante: onde o sabotador entra, qual gate deveria capturar, e
em qual direção — falso negativo e falso positivo.

---

### Candidato (a): contador de profundidade por env (`TRACKFW_BARRIER_DEPTH`)

**Mecanismo:** `barrier` incrementa a variável ao iniciar e recusa se já estiver em 1.

**Contra-braço medido:** `make quality` aparece em gate blocks de **4 roadmaps `done/`** do
acervo (10 blocos de gate; ver nota de contagem em § 2 acima). `make quality` chama
`scripts/check-barrier.sh` via o alvo `parity`. `check-barrier.sh:181–188` executa
`trackfw barrier fixture --trust-local-gates`. Com um contador de profundidade cego:

```
Outer barrier (DEPTH=0→1) → gate "make quality" → sh herda DEPTH=1 →
make quality → check-barrier.sh → trackfw barrier fixture (DEPTH=1→2) → RECUSADO
```

Resultado: AC3 violada. Gate legítimo (`make quality`) quebrado.

`barrier_contract_test.go:258 runBarrierCLI` confirma: `cmd.Env` não é setado, o processo filho
herda o env do pai. Então qualquer gate que transite por `go test ./internal/commands/...`
também herda o contador.

**Veredito:** FALSIFICADO. O contador cego quebra gates legítimos que aninham barrier sobre
fixtures distintas.

---

### Candidato (b): inspeção estática do gate procurando o subcomando `barrier`

**Mecanismo:** antes de executar, verificar se a string do gate contém `barrier`. Se sim, recusar.

**Falso negativo (não detecta a ameaça real):**
- Gate: `make quality` ou `./check-barrier.sh` — não contém a palavra `barrier` no texto do gate,
  mas chama `trackfw barrier` internamente. A inspeção estática não enxerga indireção.
- Gate: `sh -c "$(./run-checks.sh)"` — qualquer wrapper opaco.

**Falso positivo (bloqueia uso legítimo):**
- Gate: `trackfw barrier outro-roadmap --wave 1` — gate que verifica outra wave de outro roadmap.
  Legítimo e correto, bloqueado sem necessidade.
- Gate: `grep -q 'barrier' docs/cli-parity.md` — checagem de doc que menciona a palavra.

**Veredito:** INADEQUADO. Ambas as direções falham. A inspeção estática de string não tem
semântica suficiente para este problema.

---

### Candidato (c): pilha de chaves `(roadmap, wave)` herdada por env, com backstop de profundidade

**Mecanismo:** ao iniciar, `barrier` serializa a chave `(roadmap_abs_path, wave_label)` em
`TRACKFW_BARRIER_STACK` (ex.: path1:wave1|path2:wave2). Se a chave atual já está na pilha, recusa.
Backstop: recusa se a pilha já tem N entradas (N=4; profundidade máxima legítima observada: pilha
de comprimento 1 ao iniciar o nível 2; N=4 dá folga de 3 níveis — ver § 5, item 3).

**Contra-braço medido:**

```
Outer barrier roadmapA --wave 1:
  TRACKFW_BARRIER_STACK = "roadmapA:1"
  Gate: "make quality" → sh herda TRACKFW_BARRIER_STACK
  check-barrier.sh → trackfw barrier fixture1 --wave 1:
    chave=(fixture1,1) — não está na pilha → PASSA
  barrier_contract_test.go → trackfw barrier fixture2 --wave 2:
    chave=(fixture2,2) — não está na pilha → PASSA
```

Reentrada real:
```
Outer barrier roadmapA --wave 1:
  TRACKFW_BARRIER_STACK = "roadmapA:1"
  Gate: "trackfw barrier roadmapA --wave 1":
    chave=(roadmapA,1) — JÁ NA PILHA → RECUSADA, exit 2
```

**Falso negativo residual:** gate que faz `env -i trackfw barrier <mesmo roadmap> --wave N`
(strip do env) contorna a pilha. Sob `env -i` — e qualquer ambiente que apaga o env herdado:
`sudo -i`, `docker run` sem `-e`, `ssh` — `TRACKFW_BARRIER_STACK` chega vazia no processo filho.
**Tanto o check de deduplicação por chave quanto o backstop de profundidade dependem dessa
variável: ambos deixam de funcionar.** Não existe um "contador separado" no desenho recomendado.
A recursão volta a ser ilimitada.

Backstop independente de env avaliado:
- **Cadeia PPID com argv:** inspecionar os ancestrais PID-por-PID procurando outro `trackfw
  barrier` com o mesmo `(roadmap, wave)` no argv. Não bloqueia runs paralelas legítimas.
  Custo: até O(N) lookups de process-table por nível. Portabilidade: Linux via
  `/proc/<pid>/cmdline` + `/proc/<pid>/cwd`; macOS via libproc; Windows requer PEB — inviável
  sem libs extras. Matching por basename dá falso-positivo; matching pelo path absoluto exige
  o cwd do ancestral, que tem APIs distintas por OS. Um wrapper que chame `execve` com um
  novo PID derrota a inspeção sem `env -i`.
- **Lock por chave de arquivo:** lock file em diretório temporário por hash `(roadmap, wave)`.
  Independe de env. Mas bloqueia runs **paralelas e legítimas** da mesma wave (dois arquitetos
  em paralelo) e deixa locks órfãos após SIGKILL.

Nenhuma das duas alternativas vale a complexidade para este residual. Um autor benigno não
escreve `env -i trackfw barrier <mesmo-roadmap>` como gate — e `sudo -i`/`docker run`/`ssh`
não executam `barrier` sobre o mesmo roadmap salvo arquitetura deliberada.
Declarado como resíduo **sem contenção** (ver § 5, item 1).

**Veredito:** APROVADO. Survives the contra-braço central (make quality / check-barrier.sh /
barrier_contract_test.go). AC3 preservada.

**Contagem de gates e reconciliação de números (2026-09-30):**

| Universo contado | Número | Método |
|---|---|---|
| Gates com invocação direta de `trackfw barrier` (regex `^\s*(trackfw\|./bin/trackfw\|bin/trackfw)\s+barrier\b`, exclui linhas `#`) | **0** | Python, extração do 1º bloco cercado após `**Gates da wave:**` |
| Gate blocks com `make quality` (indireção que aninha `barrier` sobre fixture) | **10 blocos em 4 arquivos** | Python, mesmo extrator, busca por substring |
| Arquivos em `done/` cujos gate blocks contêm `make quality` | **4** | `done/ROADMAP-2026-07-29-barrier-governanca` (6 waves), `done/ROADMAP-2026-08-22-wave-0-ameaca`, `done/ROADMAP-2026-08-31-portar-correcoes`, `done/ROADMAP-2026-09-01-repositorio-trackfw` |
| Gates com `grep -q ... scripts/check-roadmap-barrier-contract.sh` | **3 blocos em 3 arquivos** | Inspecionam o script com grep; **não executam barrier** |

O "11" dos registros anteriores era de um script distinto que não restringia a busca a blocos de gate — contava qualquer ocorrência de "make quality" no texto do roadmap. Não reproduzível com o extrator atual; retratado.

**Colateral do candidato (c) no acervo:** zero. Candidato (c) recusa apenas a chave
`(path-absoluto-roadmap, wave-label)` que já está na pilha. `make quality` → `check-barrier.sh`
→ `trackfw barrier /tmp/s.../ROADMAP-barrier-fixture --wave 1` usa um diretório temporário como
roadmap — chave diferente da do roadmap externo. Os 4 roadmaps com `make quality` como gate
continuam passando sob (c). Zero colateral é conclusão do mecanismo (chave absoluta distinta),
não contagem de invocações literais.

---

### Discriminante recomendado

**(c)** com as seguintes propriedades de implementação:

- Variável: `TRACKFW_BARRIER_STACK` — lista de pares `<path-absoluto-normalizado>:<wave-label>`
  separados por `|`.
- Ao iniciar: construir a chave com o path absoluto do roadmap resolvido e o wave label
  normalizado. Checar se a chave está na pilha. Se sim: exit 2 com mensagem
  `"trackfw barrier: reentrant call — <roadmap> wave <N> is already being evaluated"`.
- Ao executar gates: adicionar a chave à pilha, exportar, executar.
- Backstop: se `len(pilha) >= 4`, exit 2 com mensagem
  `"trackfw barrier: evaluation depth limit exceeded (likely reentrant via indirection)"`.

---

## 4. Exit code e mensagem

**Reentrada detectada:** exit **2** (usage error / `barrierUsageError`).

Motivo: exit 1 significa "barrier avaliou e a wave está bloqueada" — o gate encontrou condições
insatisfeitas. Exit 2 significa "barrier não conseguiu avaliar" — a configuração do roadmap tem
um erro de uso (ciclo). Reentrada é um erro do roadmap, não um estado de avaliação. A distinção
existe hoje (`barrierUsageError`, `usageExit`).

**Mensagem proposta (reentrada direta, AC1):**
```
trackfw barrier: reentrant call — <basename(roadmap)> wave <N> is already being evaluated
```

**Mensagem proposta (backstop por profundidade, AC2):**
```
trackfw barrier: evaluation depth limit exceeded — possible reentrant call via indirection
```

**Consistência com convenção:** a mensagem usa o prefixo `trackfw barrier:` igual a todas as
outras mensagens de usage error (ex.: `"trackfw barrier: --wave is required"`, linha 80).

---

## 5. Resíduo declarado

O que o desenho escolhido aceita não cobrir, dito explicitamente:

1. **Contorno por env-clearing:** `env -i trackfw barrier <mesmo roadmap>` (e equivalentes:
   `sudo -i`, `docker run` sem `-e`, `ssh`) descarta `TRACKFW_BARRIER_STACK`. Como tanto o
   check de deduplicação por chave quanto o backstop de profundidade dependem desta variável,
   **não há freio algum** sob env-clearing: a recursão volta a ser ilimitada. Alternativas
   independentes de env foram avaliadas (cadeia PPID; lock por chave de arquivo) e rejeitadas
   por custo de portabilidade ou efeitos colaterais piores que o defeito para este cenário.
   Resíduo **sem contenção**. Aceitável: nenhum desses ambientes executa `barrier` no mesmo
   roadmap por acidente; o autor que escrever esse gate está contornando a proteção
   deliberadamente.

2. **Conteúdo de gate arbitrário:** fork bomb (`:(){ :|:& };:`), `rm -rf`, `curl | sh` — gate é
   `sh -c` por desenho; a proteção desta REQ é contra reentrada do próprio `barrier`, não contra
   gate hostil. Trust check via `origin/main` cobre PR de terceiro, não clone hostil. Declarado
   escopo negativo da REQ.

3. **Profundidade de backstop:** N=4 — medido, não apenas afirmado.

   Cadeia máxima legítima observada no acervo:
   - Nível 1: `barrier roadmapA --wave W` (invocação do arquiteto; pilha = `[]` → push → `["roadmapA:W"]`)
   - Nível 2: gate `make quality` → `parity` → `check-barrier.sh` (ou `go test` →
     `runBarrierCLI`) → `barrier /tmp/.../ROADMAP-barrier-fixture --wave 1`
     (pilha herdada = `["roadmapA:W"]`; len=1 < 4 → passa; push → `["roadmapA:W","fixture:1"]`)
   - Nível 3: fixture tem gates `exit 0` / `touch sentinel` / `true`/`false` — nenhum chama
     `barrier`. Cadeia termina em nível 2.
   - Corpus snapshots em `check-roadmap-barrier-contract.sh` não têm `trackfw barrier` como
     gate (verificado por grep em `scripts/testdata/roadmap-barrier-corpus-snapshot/`).

   **Comprimento máximo de pilha ao iniciar o nível 2: 1.** O backstop dispara em `len >= 4`,
   ou seja, ao iniciar o nível 5 com env intacto. Folga: 3 níveis acima do máximo observado.

   Se no futuro um fluxo legítimo precisar de pilha >= 4, o backstop precisará ser revisto.
   Deve ser configurável ou documentado em cli-parity.md.

4. **Serialização da pilha:** `TRACKFW_BARRIER_STACK` passado por env pode ser truncado por
   sistemas que limitam o tamanho do env (ex.: ARG_MAX no Linux: tipicamente 2 MB; na prática,
   um path de 200 char × 4 entradas = 800 char — sem problema). Documentado como comportamento
   conhecido, não como vulnerabilidade.

---

## Premissas da REQ — veredicto

| Premissa | Verificação | Resultado |
|---|---|---|
| (i) Nenhuma correção de profundidade fecha "DoS por arquivo de texto" | Trust check (barrier.go:219–350) não protege clone hostil; gate é `sh -c` arbitrário | **CONFIRMADA** — o escopo negativo da REQ está correto |
| (ii) `barrier_contract_test.go` herda o env do pai | `runBarrierCLI` (~:258): `exec.Command(bin, fullArgs...)` sem atribuição a `cmd.Env` | **CONFIRMADA** — nenhum isolamento de env no teste |
| (iii) 0 gates reentrantes no acervo desta branch | Script Python (extração de primeiro bloco cercado após marcador): 0 | **CONFIRMADA** — acervo limpo; a mina da `main` (ROADMAP-2026-09-22:73) foi desarmada nesta branch |

Nenhuma premissa foi refutada. Nenhuma mudança de escopo decorre desta análise.

---

## Comandos executados como evidência

```
# Contagem de gates reentrantes no acervo (Python)
python3 -c "..." → Total barrier-in-gates: 0

# Verificação de cmd.Env em barrier_contract_test.go
grep -n "cmd\.Env\|Env\s*=" internal/commands/barrier_contract_test.go → (sem saída)

# Gates por categoria (Python, extrator de 1º bloco após **Gates da wave:**, 2026-09-30)
# make quality: 10 blocos em 4 arquivos únicos
# trackfw barrier (não-comentário, regex): 0
# check-barrier (como string): 0
# check-roadmap-barrier (grep sobre script, não execução): 3 blocos em 3 arquivos
# O "11" de registros anteriores era contagem sem restrição a blocos de gate — retratado.

# make quality → parity target
grep -n "quality\|check-barrier" Makefile → linha 40: GO_BIN=... scripts/check-barrier.sh

# barrier_contract_test.go runBarrierCLI
sed -n '250,280p' internal/commands/barrier_contract_test.go → cmd sem cmd.Env

# Únicos executores de gate em internal/commands/ (não-test, não-barrier.go)
grep -rn "runGateCommand|evalGateCommands|sh -c" internal/commands/*.go (excl. barrier.go)
→ release.go:205: git show (não é gate), audit_surface.go:86: git rev-parse (não é gate)
```
