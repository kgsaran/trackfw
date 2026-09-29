# Wave 2 — Auditoria Independente: `trackfw init` reexecutado preserva configuração do consumidor

> Hades (Security) · 2026-09-28 · Branch: `fix/init-reexecutado-destroi-config-do-consumidor`
> Método: reimplementação independente — leitura de ADR+REQ+Wave0, derivação própria do que o produto deve fazer, medição caixa-preta do binário compilado nesta sessão.
> NÃO foram lidos: git diff, testes entregues pelos executores, relatórios anteriores dos MLs.

---

## Compilação

```
go build -o bin/trackfw ./cmd/trackfw
```

BUILD OK — binário compilado antes de qualquer medição.

---

## 1. Cenário do dano real reconstruído do zero

**O que a ADR exige:** `init` reexecutado sobre `trackfw.yaml` com `governance_mode: lenient`, `lenient_until`, e bloco `agent_models` com comentário de justificativa deve preservar os três. A saída de `trackfw validate` deve ser byte-idêntica antes e depois.

**Setup:** diretório temporário em scratchpad com `trackfw.yaml` contendo:
```yaml
# trackfw configuration — gerado por trackfw discover
# governance_mode: lenient permite validação não-bloqueante durante onboarding

governance_mode: lenient
lenient_until: "2027-12-31"

adr_dirs:
  - docs/adr
req_dir: docs/req
roadmap_dir: docs/roadmaps
roadmap_namespacing: flat
hooks: none
ci: github-actions
forge: github

# Versão do modelo por tier (ADR-2026-08-21).
# Justificativa de cota: orçamento mensal aprovado em board 2026-08-01.
agent_models:
  default: claude-3-5-haiku-20241022
  orchestrator: claude-sonnet-4-5
```

**Validate antes do init:**
```
[LENIENT MODE] Governance violations treated as warnings until 2027-12-31
✓ No violations found.
```
Exit code: 0

**Init executado:** RC=0

**Diff trackfw.yaml antes vs depois:**
```
20a21,32
> 
> frontend: 
> backend: 
> backend_framework: 
> pkg_manager: npm
> wip_limit: 1
> wip_by_squad: false
> require_req_in_commit: false
> 
> # validator rules (off / warning / error)
> rules:
>   branch_has_wip_roadmap: error
```

Blocos novos acrescentados; zero diff nas linhas pré-existentes. `governance_mode`, `lenient_until`, `agent_models` (incluindo comentário de justificativa) intactos.

**Validate depois do init:**
```
[LENIENT MODE] Governance violations treated as warnings until 2027-12-31
✓ No violations found.
```
diff das duas saídas: **(byte-identical)**

**Segundo init (idempotência):** diff trackfw.yaml após run1 vs run2 = vazio.

**Veredito:** CORRETO. O cenário do dano real está corrigido.

---

## 2. Segundo sítio (a) — lefthook.yml

**O que a ADR exige:** `generateLefthookHook` (linha 2806, agora renumerada como linha ~2952) não deve sobrescrever incondicionalmente `lefthook.yml`. Deve preservar hooks do consumidor e só acrescentar o bloco `trackfw-validate:` se ausente.

**Alcançabilidade:** a CLI não-interativa usa `Hooks: "none"` (init.go:110) e não atinge `generateLefthookHook`. Caminho usado: leitura direta do código-fonte + derivação (sem execução independente via CLI). Os Go tests executados são os do executor ML-1C — não são evidência independente para esta wave.

**O que a leitura independente do código encontrou:**

A função `generateLefthookHook` (scaffold.go:2952) tem DOIS predicados de guarda:

1. **`presentTopLevelKeys(existingStr)["pre-commit"]`** (linha 2975) — usa a mesma função P1+P3 de `writeTrackfwConfig`. Correto.

2. **`strings.Contains(existingStr, "trackfw-validate:")`** (linha 2970) — predicado de idempotência. **Bare substring, sem P3.**

### 🔴 DEFEITO: P3 violado no predicado de idempotência (linha 2970)

O predicado `strings.Contains(existingStr, "trackfw-validate:")` retorna `true` para um arquivo `lefthook.yml` que contém `trackfw-validate:` apenas em comentário:

```
# trackfw-validate: (removed temporarily)
```

**Medição direta do predicado:**

```go
// Replica da linha 2970 de generateLefthookHook
commentOnly := "pre-commit:\n  commands:\n    lint:\n      run: golangci-lint run\n# trackfw-validate: (removed temporarily)\n"
strings.Contains(commentOnly, "trackfw-validate:") → true  // P3 VIOLADO: comentário = presente
```

Saída do programa de medição:
```
Comment-only: true (want false for P3, actual: true — P3 VIOLATED: true)
Actually present: true (want true)
Absent: false (want false)
```

**Efeito:** consumidor que teve `trackfw-validate:` e o comentou nunca mais recebe o hook via `init`. Silenciosamente. O `init` reporta `✓ lefthook.yml` mas não instala nada.

**Contexto:** ML-1D fechou o mesmo defeito P3 para o gate de `writeTrackfwConfig` (a ADR exige chave comentada = ausente). O predicado de idempotência de `generateLefthookHook` usa `strings.Contains` em vez de `presentTopLevelKeys`, não herdando a correção P3.

**Severidade:** menor do que o sítio principal (`trackfw.yaml`) — não afeta `trackfw validate`. Mas viola a ADR: "chave comentada = chave ausente" deve valer em todos os predicados de presença que governam escrita.

**Ação requerida:** corrigir o predicado na linha 2970. Em vez de `strings.Contains`, usar uma função que exclua linhas de comentário — ou `presentTopLevelKeys`-like para o contexto de submapping YAML do lefthook.

**Esta correção é responsabilidade de Apolo (implementação). Hades não modifica código de produto.**

---

**Go tests do executor (ML-1C):** 5/5 PASS — mas não são evidência independente (são os artefatos que esta wave foi construída para não herdar). Eles PASSAM porque não testam o caminho do comentário. Confirmar: os testes T1-T5 entregues não incluem um `lefthook.yml` com `# trackfw-validate:` como estado inicial. O defeito P3 está fora do corpus dos testes entregues.

---

## 3. Gate que mede os sítios (a)

**Direção positiva (PASS no código real):**

```
bash scripts/check-init-preserves-user-config.sh
```

Saída:
```
OK   [trackfw.yaml] line 987 in writeTrackfwConfig — os.ReadFile precedes write in same function
OK   [trackfw.yaml] line 1025 in writeTrackfwConfig — os.ReadFile precedes write in same function
OK   [lefthook.yml[literal]] line 3016 in generateLefthookHook — os.ReadFile precedes write in same function
OK   [lefthook.yml[via-lefthookPath]] line 2659 in generateCommitMsgHook — os.ReadFile precedes write in same function
Sites examined: 4
PASS: all 4 consumer-config write site(s) are guarded
```

RC=0.

---

### 🔴 FALSO NEGATIVO: gate aprova leitura-decorativa + truncamento incondicional

**O REQ AC exige falsificabilidade em dois sentidos.** O gate não foi falsificado na direção FAIL→PASS até agora. Medido agora:

**Decoy:** função com `os.ReadFile("trackfw.yaml")` cujo resultado é ignorado (`_ = len(existing)`) seguida de `os.WriteFile("trackfw.yaml", []byte("# overwritten"))` incondicional.

```
SCAFFOLD_FILE=<decoy> bash scripts/check-init-preserves-user-config.sh
```

Saída:
```
OK   [trackfw.yaml] line 13 in decoyWriteTrackfwConfig — os.ReadFile precedes write in same function
Sites examined: 3
PASS: all 3 consumer-config write site(s) are guarded
```

RC=0 — **FALSE NEGATIVE MEDIDO.** O gate aprova uma função que lê e descarta, depois trunca.

**Mecanismo:** o predicado do gate verifica "os.ReadFile aparece antes do os.WriteFile na mesma função". É estrutural, não semântico. Não verifica se o resultado da leitura é USADO no cálculo do que escrever.

**Classe do falso negativo:** `os.ReadFile` por qualquer motivo (logging, size check, stat-like) satisfaz condição 1. Um implementador que refatora `writeTrackfwConfig` para, por exemplo, logar o tamanho do arquivo antes de reescrever, satisfaz o gate mesmo truncando.

**Impacto no estado atual:** o código REAL de `writeTrackfwConfig` usa o resultado da leitura para o merge — o falso negativo não existe na implementação atual. Mas o gate não impede reintrodução do defeito por refator futuro que preserve o `os.ReadFile` com uso diferente. O AC "reprova quando um sítio (a) novo nasce truncando" só é satisfeito se o site não tiver nenhum `os.ReadFile` anterior. Um site refatorado que lê-e-ignora passa silenciosamente.

**Ação requerida:** o gate precisa de um predicado semântico, não apenas estrutural. Opções: (1) exigir marcador explícito `consumer-config-merge-allowed:` em TODOS os sites (a), abandonando condição 1 como suficiente sozinha; (2) verificar que o resultado de `os.ReadFile` é usado como base para o valor escrito (análise de data-flow, mais complexo). Opção 1 é a mais simples e já está suportada pelo gate (condição 2). **Hades não modifica o gate — reporta para Apolo.**

---

## 4. Contra-braço e --brownfield

### 4a. Sub-key delivery failure (rules: block presente)

**AC da REQ:** "Chave nova de versão nova É acrescentada."

**Setup:** consumer com `rules:\n  some_other_rule: warning` (bloco `rules:` existente mas sem `branch_has_wip_roadmap`).

**Resultado medido:**
```
(no diff) — nenhuma chave acrescentada
branch_has_wip_roadmap present: 0
some_other_rule present: 1
```

O merge detecta `rules:` como chave de nível 0 presente → o bloco inteiro é pulado → `branch_has_wip_roadmap: error` nunca chega ao consumidor.

**Qualificação como defeito vs. residual:** a ADR declarou como residual aceito "Uma chave cujo default mudou entre versões não é atualizada — o valor do consumidor vence." Esse enunciado cobre escalares. O caso aqui é diferente: uma SUB-CHAVE nova sob bloco existente — `branch_has_wip_roadmap` não é uma chave cujo default mudou; é uma regra nova que nunca existia.

A ADR **não declarou explicitamente** este caso como residual. O AC da REQ ("chave nova de versão nova É acrescentada") passa para top-level scalars (medido nas demais cases) mas **falha para sub-keys** de blocos já presentes.

**Impacto prático:** qualquer consumidor que customizou `rules:` com ao menos uma regra nunca receberá novas regras de validação via `init`. Para `branch_has_wip_roadmap: error`, o efeito é que o novo validador fica silenciosamente desativado no projeto deles.

**Ação requerida:** o arquiteto deve decidir: (1) declarar explicitamente como residual na ADR (com as consequências enunciadas), ou (2) implementar merge recursivo para blocos aninhados. Hades não escolhe — reporta a lacuna.

---

### 4b. --brownfield contra governance_mode: strict

**Setup:** consumer tem `governance_mode: strict` + todos os demais keys completos. Roda `trackfw init --brownfield`.

**Resultado medido:** diff vazio. `governance_mode: strict` preservado (count=1, não duplicado, não sobrescrito com `lenient`).

**Veredito:** CORRETO. O `--brownfield` não força `governance_mode: lenient` sobre um arquivo com todos os keys já presentes.

---

## 8. Ataques às 4 regras da spec (P1-P4)

### Case A — chave-de-nível-0 dentro de bloco literal (indentada, como YAML requer)

**Setup:** `agent_conventions: |` com corpo `  wip_limit: 3` e `  frontend: react` (2 espaços).

**O que P1 deveria fazer:** linhas indentadas não são chaves de nível 0 → não contadas como presentes → template acrescenta `wip_limit:` e `frontend:` no nível 0.

**Medido:**
```
wip_limit: 1     ← acrescentado (correto: a linha indentada não era topo-nível)
frontend:        ← acrescentado (idem)
```
Corpo do bloco literal intacto (`  wip_limit: 3`, `  frontend: react` preservados).

**Veredito:** CORRETO. P1 (âncora em coluna 0) previne confusão entre conteúdo de bloco e chave de nível 0.

---

### Case B — pares com prefixo compartilhado (roadmap_dir vs roadmap_namespacing)

**Setup:** consumer tem `roadmap_dir: docs/roadmaps` mas não `roadmap_namespacing`.

**O que P2 deveria fazer:** `roadmap_dir:` e `roadmap_namespacing:` diferem após o `:` — P2 os distingue corretamente, acrescenta `roadmap_namespacing: flat`, não duplica `roadmap_dir`.

**Medido:**
```
roadmap_dir count: 1    (preservado, não duplicado)
roadmap_namespacing: 1  (acrescentado)
```

**Veredito:** CORRETO. P2 (dois-pontos no padrão) elimina colisão de prefixo.

---

### Case C — chave duplicada já no arquivo do consumidor

**Setup:** consumer tem `wip_limit: 5` e `wip_limit: 3` no mesmo arquivo.

**O que o merge deveria fazer:** a chave está presente (pelo menos uma ocorrência) → não acrescenta terceira instância. Conteúdo existente preservado intacto.

**Medido:**
```
wip_limit count: 2    (as duas do consumidor, não acrescentou terceira)
wip_by_squad:         (ausente → acrescentado corretamente)
```

**Veredito:** CORRETO. A duplicata pré-existente é do consumidor; o merge não cria nova duplicata.

---

### Case D1 — arquivo sem newline final

**Setup:** `trackfw.yaml` com `forge: github` sem `\n` terminal.

**O que P4 deveria fazer:** antes de acrescentar blocos, adicionar `\n` se ausente, para que o primeiro bloco não grude na última linha existente.

**Medido:**
```
< forge: github
\ No newline at end of file
---
> forge: github
> 
> frontend: 
```
`forge: github` ficou em sua própria linha; o primeiro bloco acrescentado começa na linha seguinte.

**Veredito:** CORRETO. P4 (guarda de newline final) previne corrupção silenciosa.

---

### Case D2 — arquivo vazio

**Setup:** `trackfw.yaml` existente mas vazio (0 bytes).

**O que deveria acontecer:** `presentTopLevelKeys` retorna mapa vazio → todos os blocos do template são "ausentes" → acrescentados. Guarda P4 (`out != ""`) evita newline espúrio no início.

**Medido:** arquivo vazio → todos os blocos do template acrescentados, começando com `\nfrontend:` (blank leader do primeiro bloco). RC=0.

**Veredito:** CORRETO.

---

### Case D3 — arquivo só com comentários (P3 em ação)

**Setup:**
```yaml
# trackfw configuration
# All keys commented out
# governance_mode: lenient
# wip_limit: 3
```

**O que P3 deveria fazer:** `# wip_limit: 3` é comentário → chave ausente → `wip_limit: 1` acrescentado. `# governance_mode: lenient` é comentário → ausente, mas `governance_mode:` não está no template sem `--brownfield` → não acrescentado de qualquer forma.

**Medido:**
```
wip_limit: 1    ← acrescentado (P3 não confundiu o comentário com presença)
# governance_mode: lenient  ← preservado como comentário
```

**Veredito:** CORRETO. P3 (comentário = ausente) funciona. Comentários pré-existentes preservados.

---

### Case E — CRLF

**Setup:** arquivo com 10 linhas CRLF (`\r\n`).

**O que deveria acontecer:** P1-P3 funcionam com `\r` no final de cada segmento (após `strings.Split(content, "\n")`). Chaves corretamente detectadas. Blocos acrescentados usam LF → arquivo terá line endings mistos.

**Medido:**
```
governance_mode count: 1  (CRLF line corretamente detectada como presente — não duplicada)
frontend count: 1         (ausente → acrescentado com LF)
CRLF lines count: 10      (as originais, preservadas)
Total lines: 22           (10 originais + 12 novas em LF)
```

**Observação de qualidade** (não é corrupção): o arquivo resultante tem mixed line endings (CRLF + LF). Isso é uma consequência de preservar o arquivo do consumidor "tal como está" — a filosofia da ADR. O produto não normaliza line endings do arquivo do consumidor, o que é intencional.

**Veredito:** CORRETO (sem corrupção ou perda de dados). Mixed endings são uma consequência intencional da estratégia "zero diff nas linhas existentes".

---

### Case F — indentação com TAB

**Setup:** linha `\t- docs/adr` (TAB) dentro de bloco `adr_dirs:`.

**O que P1 deveria fazer:** `line[0] == '\t'` → linha ignorada → não contada como chave de nível 0.

**Medido:**
```
Tab line preserved: 1   (preservada intacta)
Diff: apenas blocos novos acrescentados
```

**Veredito:** CORRETO. P1 trata TAB explicitamente.

---

### Case G — chave com indentação de 1 espaço

**Setup:** ` wip_limit: 5` (1 espaço inicial) no arquivo do consumidor.

**O que P1 deveria fazer:** `line[0] == ' '` → ignora → chave considerada ausente → `wip_limit: 1` acrescentado.

**Resultado:**
```
 wip_limit: 5    ← do consumidor (preservado)
wip_limit: 1     ← acrescentado pelo merge
```

O arquivo resultante tem duas entradas para `wip_limit`. O comportamento é tecnicamente correto (o conteúdo do consumidor foi preservado sem alteração), mas a indentação de 1 espaço no YAML já era inválida/ambígua antes do merge. O merge não agrava a situação nem a corrige.

**Veredito:** CORRETO para as garantias da ADR (zero diff nas linhas existentes). A semântica YAML do arquivo resultante é responsabilidade do consumidor.

---

### Case H — aparente chave na coluna 0 após indicador de bloco literal

**Setup:**
```yaml
agent_conventions: |
wip_limit: this looks like it terminates the block
```

Em YAML válido, `wip_limit:` SEM indentação após `|` termina o bloco literal (que fica vazio) e é uma chave de nível 0. O merge e o parser YAML concordam: `wip_limit` é uma chave de nível 0.

**Medido:** `wip_limit` não foi re-adicionado (já presente em coluna 0). `agent_conventions: |` preservado.

**Veredito:** CORRETO. A construção adversarial é YAML válido; merge e parser concordam.

---

## 6. Veredito sobre o ponto único — ADR satisfeita?

A ADR diz: "ponto único por defeito são dois após a enumeração da Wave 0". Os dois sítios (a) são:
- Linha 864: `writeTrackfwConfig` — corrigido em ML-1A
- Linha 2806: `generateLefthookHook` — corrigido em ML-1C

O gate (`check-init-preserves-user-config.sh`) confirma RC=0 com 4 sites guardados.

**A ADR de ponto único está satisfeita.** Não sobrou sítio (a) sem correção.

---

## 7. Verificação: git diff trackfw.yaml desta árvore

```
git diff trackfw.yaml
(saída vazia — confirmado antes de escrever este parecer)
```

---

## 9. Tabela de achados

| # | Caso | Esperado pela ADR | Medido | Veredito |
|---|---|---|---|---|
| 1 | Cenário real (lenient+agent_models+comentários) | validate byte-identical, comentários preservados | IDÊNTICO | CORRETO |
| 2 | lefthook.yml — caso nominal (leitura antes de escrita) | ReadFile precede WriteFile | código fonte confirma | CORRETO |
| 🔴 2b | **lefthook.yml — P3 no predicado de idempotência (linha 2970)** | **comentário = ausente** | **strings.Contains retorna true para comentário** | **DEFEITO** |
| 3 | Gate — direção positiva | RC=0 no código real | RC=0, Sites=4 | CORRETO |
| 🔴 3b | **Gate — falso negativo (leitura decorativa + truncamento)** | **deve FAIL** | **reporta OK para decoy** | **DEFEITO no gate** |
| 4a | Contra-braço sub-key (rules: já presente) | nova sub-key entregue | branch_has_wip_roadmap nunca acrescentado | LACUNA NÃO DECLARADA |
| 4b | --brownfield com governance_mode: strict | consumer value wins | strict preservado, sem duplicata | CORRETO |
| A | Key-like dentro de literal block (indentada) | P1 ignora | chaves corretas acrescentadas em nível 0 | CORRETO |
| B | Prefixo compartilhado (roadmap_dir/namespacing) | P2 distingue | só ausente acrescentado | CORRETO |
| C | Chave duplicada no consumer | duplicatas preservadas | duplicatas intactas, não re-adicionado | CORRETO |
| D1 | Arquivo sem newline final | P4: newline + blocos | nada grudado, append correto | CORRETO |
| D2 | Arquivo vazio | todos os blocos | blocos acrescentados | CORRETO |
| D3 | Só comentários | P3: comentado = ausente | wip_limit: 1 acrescentado, comentários preservados | CORRETO |
| E | CRLF | P1-P3 funcionam | funcionam; mixed endings residual cosmético | CORRETO |
| F | TAB indentation | P1 trata TAB | TAB preservado, not counted as key | CORRETO |
| G | 1-espaço indentation | P1 ignora → duplicata | duplicata; YAML do consumer já era inválido | CORRETO |
| H | Key em coluna 0 após `\|` indicator | YAML e merge concordam | concordam | CORRETO |

### Defeitos para Apolo (implementação)

**D1 — DEFEITO PRODUTO** (`generateLefthookHook`, linha 2970):
- Predicado de idempotência usa `strings.Contains` sem P3 → comentário bloqueia instalação silenciosamente
- Arquivo: `internal/generators/scaffold.go:2970`
- Correção: substituir `strings.Contains(existingStr, "trackfw-validate:")` por predicado que exclui linhas de comentário (equivalente a P3 de `presentTopLevelKeys`)

**D2 — DEFEITO GATE** (`scripts/check-init-preserves-user-config.sh`, condição 1):
- Leitura decorativa (`_ = len(existing)`) + truncamento incondicional passa o gate
- O gate não verifica se o resultado da leitura é usado para o merge
- Correção: exigir marcador `consumer-config-merge-allowed:` em TODOS os sites (a) (já existe como condição 2 do gate — torná-la a única condição suficiente, ou condição obrigatória)

**D3 — LACUNA NÃO DECLARADA NA ADR** (sub-key delivery para blocos aninhados):
- Consumidores com `rules:` block customizado nunca recebem novas regras de validação via `init`
- A ADR não declarou este caso como residual explícito
- Não é um defeito de segurança — é uma lacuna de entrega de funcionalidade
- Ação: arquiteto decide se declara como residual na ADR ou implementa merge recursivo

---

## 10. Residuais declarados (herdados da Wave 0 + novos)

1. **Mixed line endings (CRLF+LF) em Case E:** consequência intencional da estratégia "zero diff nas linhas existentes". Não é um defeito pelos critérios da ADR.

2. **1-espaço indentation cria duplicata semântica (Case G):** o consumidor que usa 1 espaço de indentação para uma chave de nível 0 já tinha YAML inválido/ambíguo. O merge não agrava.

3. **discover --init guard:** `os.Stat` skip-if-exists em commands/discover.go:138-141. Verificado ativo. Não é sítio de defeito.

4. **Site (a) #2 — evidência de execução independente ausente:** `generateLefthookHook` é inalcançável via CLI não-interativa. A evidência de execução disponível é a dos testes entregues por ML-1C (não independente). A leitura de código independente encontrou o defeito P3 (item D1 da tabela), que os testes do executor não cobrem.
