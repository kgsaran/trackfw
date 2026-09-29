---
status: wip
date: 2026-09-28
req: "docs/req/REQ-2026-09-28-trackfw-init-reexecutado-destroi-a-configuracao-do-consumidor-e-o-produto-passa-a-reprovar-um-repositorio-conforme.md"
squad: ""
---

# Roadmap: `trackfw init` reexecutado destrói a configuração do consumidor

> Created: 2026-09-28 | Status: wip

## Context
<!-- Derived from REQ -->
REQ: docs/req/REQ-2026-09-28-trackfw-init-reexecutado-destroi-a-configuracao-do-consumidor-e-o-produto-passa-a-reprovar-um-repositorio-conforme.md
ADR: docs/adr/ADR-2026-09-28-trackfw-init-reexecutado-preserva-a-configuracao-autorada-pelo-consumidor.md
Origem: **#445** + ocorrência real neste repositório (2026-09-27), `validate` 170 warnings → 156 violations.

## Acceptance Criteria
<!-- Consolidados; detalhe por ML nas waves abaixo. -->
- [x] Os 22 sítios enumerados e classificados em (a)/(b)/(c), com a razão escrita por sítio
- [x] `init` reexecutado preserva `governance_mode`, `lenient_until` e `agent_models` — e a saída de
      `trackfw validate` é **byte-idêntica** antes e depois
- [x] Comentários preservados, incluindo a justificativa de cota do bloco `agent_models`
- [x] Zero diff nas linhas pré-existentes; chave nova de versão nova **É** acrescentada (contra-braço)
- [x] Gate falsificável nas duas direções: reprova sítio (a) novo que trunque; **não** reprova (b) legítimo
- [x] `make quality` **RC=0** — verificado pelo arquiteto, não aceito do relatório:
      1380 `OK`, 0 FAIL real (os 6 `FAIL` do log vêm de `/var/folders/.../arm1.go`, fixtures dos
      braços de self-test, e os 3 braços dão `PASS`). Os 4 sítios da árvore passam **via
      `os.ReadFile`**, nenhum por marcador. CI: ver PR

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat model: medir o que é defeito e o que é comportamento desejado
> Dependências: nenhuma. 🔴 **Bloqueia toda a implementação.**

**Gates da wave:**

🔴 **O número deste gate foi 22 e passou a 23 — e a razão está escrita, porque mudar número de gate
para fazer passar é anti-padrão.** O gate bloqueou legitimamente no fim da Wave 1, fazendo o que foi
desenhado para fazer: *"a população mudou, reclassifique"*.

**Causa medida:** o ML-1A dividiu a escrita de `writeTrackfwConfig` em **dois** caminhos —
`os.WriteFile` na linha **987** (arquivo ausente → template completo) e na linha **1025** (arquivo
presente → append dos blocos ausentes) — onde antes havia um. O crescimento é **dentro da classe já
tratada**, e o sítio novo está **classificado e guardado**:

```
OK   [trackfw.yaml] line  987 in writeTrackfwConfig — os.ReadFile precedes write in same function
OK   [trackfw.yaml] line 1025 in writeTrackfwConfig — os.ReadFile precedes write in same function
Sites examined: 4   ·   PASS
```

Não é afrouxamento: é nova linha de base **pós-implementação**, com os 23 sítios cobertos — 3 da
classe (a), todos guardados, e o gate do ML-1B os verifica continuamente.

```bash
n=$(grep -cE '^[[:space:]]*(if err := )?os\.(WriteFile|OpenFile)' internal/generators/scaffold.go); test "$n" = "23" && echo "Gate W0: $n sitios reais de escrita (regua com ancora de inicio de linha)" || { echo "GATE FALHOU: esperava 23 sitios, contou $n — a populacao mudou, reclassifique antes de implementar" >&2; exit 1; }
```

### ML-0A — enumerar e classificar os 22 sítios de escrita
**Owner:** `hades-tf`
**Status:** ✅ Concluído — auditado em 2026-09-28 · 🔴 **refutou a premissa da REQ**
**Arquivos:** `internal/generators/scaffold.go` (leitura), `docs/seguranca/2026-09-28-wave0-init-destroi-config.md` (escrita)

**Tarefa:** para cada um dos 22 sítios reais de `os.WriteFile`/`os.OpenFile`, classificar em:
- **(a)** config/declaração **autorada pelo consumidor** → truncar é **defeito**
- **(b)** artefato **gerado pelo produto** → truncar é **correto**, é como a correção chega
- **(c)** **híbrido** (bloco gerado dentro de arquivo do usuário) → verificar se o ramo de merge está completo

🔴 **A régua é a natureza do conteúdo destruído, NÃO a forma da chamada.** Um sítio (b) "corrigido"
para preservar congelaria scripts defeituosos na máquina de quem instalou — foi por (b) que o fix de
CRLF do #353 chegou aos consumidores.

**Critérios de aceite:**
- [x] Os 22 sítios enumerados com linha exata e classe, **nenhum sem razão escrita**
      → auditado pelo arquiteto por cruzamento: as 22 linhas reais do `grep` com âncora estão **todas**
      citadas no parecer (`comm -23` entre reais e citadas = **vazio**), cada uma com função,
      arquivo-alvo, classe e razão
- [x] A população **(a)** nomeada → **são 2, não 1**: `writeTrackfwConfig:864` (`trackfw.yaml`) e
      `generateLefthookHook:2806` (`lefthook.yml`)
- [x] Os sítios **(c)** verificados → **6, todos com ramo de merge COMPLETO**
      (`.gitattributes` e `.gitignore` com create/append/no-op; `lefthook.yml` bloco `commit-msg` com
      `strings.Contains`; `vault/notes/index.md` com skip-if-exists). Nenhuma lacuna — são o
      **precedente de implementação** da Wave 1, não trabalho
- [x] 🔴 **Premissa da ADR CONFIRMADA** — merge textual por chave ausente cobre todos os constructs
      que o produto emite. Spec derivada: **P1** âncora em coluna 0 · **P2** `key+":"` e não `key`
      (fecha a colisão `roadmap_dir` × `roadmap_namespacing`) · **P3** chave comentada = ausente ·
      **P4** guarda de newline final antes do append. Declarados fora por o produto nunca emitir:
      âncoras/aliases, merge key `<<:`, documentos múltiplos `---`

**Resultado auditado:** **(a)=2 · (b)=14 · (c)=6**.

🔴 **A Wave 0 refutou a premissa da REQ** de que a população (a) seria só o `trackfw.yaml`. O segundo
sítio é `generateLefthookHook` (`scaffold.go:2806`), que sobrescreve `lefthook.yml` incondicionalmente.

**Auditoria do arquiteto — e por que ela quase produziu o veredito errado:** reproduzi com
`trackfw init` num projeto com `lefthook.yml` autorado e **o arquivo ficou intacto**. Quase declarei o
achado falso. A medição do braço estava errada: `generateGitHooks` só chama `generateLefthookHook`
quando `cfg.Hooks == "lefthook"`, e o caminho **não-interativo** usa `Hooks: "none"`
(`init.go:110`). O **wizard** oferece `lefthook` (`init.go:226`) — é por ali que o sítio é alcançado.
O Hades acertou lendo o código; eu quase o refutei medindo o braço que não passa pelo defeito.

Os 6 sítios **(c)** têm ramo de merge **completo** (`.gitattributes` e `.gitignore` com
create/append/no-op) — são o **precedente de implementação** para o ML-1A, não trabalho a fazer.

## Wave 1 — o merge textual, e o gate que o sustenta
> Dependências: **Wave 0 auditada.** MLs 1A, 1B e 1C tocam arquivos disjuntos → paralelos.

### ML-1A — `writeTrackfwConfig` mescla em vez de truncar
**Owner:** `apolo-tf`
**Status:** ✅ Concluído — auditado em 2026-09-28
**Arquivos:** `internal/generators/scaffold.go`, teste novo em `internal/generators/`
**⚠️ NÃO toque em `scripts/`** — é do ML-1B, que roda em paralelo.

**Critérios de aceite:**
- [x] `trackfw.yaml` ausente → escreve o template inteiro (comportamento atual preservado)
- [x] `trackfw.yaml` presente → preserva **todo** valor existente e acrescenta **só** chaves ausentes
- [x] 🔴 Comentários preservados, inclusive comentário de linha ao lado de valor
- [x] 🔴 **Zero diff** nas linhas pré-existentes — medido pelo arquiteto: o diff do `trackfw.yaml`
      após `init` mostra **só chaves novas acrescentadas ao fim**, nenhuma linha anterior tocada
- [x] **Contra-braço:** chave nova de versão nova **É** acrescentada — sem ele, "preservar" degenera
      em "não escrever nada"
- [x] 🔴 **O teste que mede o efeito:** fixture com `governance_mode: lenient` + `lenient_until` +
      `agent_models` comentado; `validate` **byte-idêntico** antes e depois do `init`
- [x] Reconciliação: cada teste novo declara, em uma frase, qual conclusão do ML afirma

### ML-1B — gate que impede a reintrodução
**Owner:** `apolo-tf`
**Status:** ✅ Concluído — auditado em 2026-09-28
**Arquivos:** `scripts/check-init-preserves-user-config.sh` (novo), `Makefile` se necessário
**⚠️ NÃO toque em `internal/generators/scaffold.go`** — é do ML-1A, que roda em paralelo.

**Critérios de aceite:**
- [x] 🔴 **Falsificável nas duas direções** — braço 1: 3 escritas desguarnecidas → **FAIL nomeando
      cada sítio**; braço 2: escritas marcadas + sítio (b) intacto → **PASS**, e o sítio (b)
      `generateValidateScript` **não é sequer examinado** (o gate não proíbe o mecanismo pelo qual o
      fix de CRLF do #353 chegou aos consumidores)
- [x] Anti-vacuidade **por alvo** (mais estrita que a pedida): falha nomeando o alvo que não tiver
      nenhum sítio, além de declarar `Sites examined: N`
- [x] Isenções explícitas e comentadas no cabeçalho do script; wired em `make parity-rest`;
      `check-orphan-gates` verde

### ML-1C — `generateLefthookHook` mescla em vez de truncar
**Owner:** `apolo-tf`
**Status:** ✅ Concluído — auditado em 2026-09-28
**Arquivos:** `internal/generators/scaffold.go` (função `generateLefthookHook`, ~linha 2806)
**⚠️ Mesmo arquivo do ML-1A** → **SEQUENCIAL após o ML-1A**, nunca em paralelo com ele.

**Origem:** achado da Wave 0. Sem este ML, o AC *"todo sítio (a) corrigido por ponto único"* fica
insatisfeito e a REQ fecharia com sítio conhecido e vivo — o achado A1 da auditoria externa de
2026-09-05, que este projeto já pagou uma vez.

**Precedente a seguir:** o ramo de merge da linha 2498 (bloco `commit-msg` no mesmo `lefthook.yml`)
já lê → verifica `strings.Contains(…, "commit-msg:")` → acrescenta se ausente. **Reuse o padrão.**

**Critérios de aceite:**
- [x] `lefthook.yml` ausente → escreve o conteúdo (comportamento atual preservado)
- [x] `lefthook.yml` presente com hooks do consumidor → **preservados**. Insere **dentro** do
      `commands:` existente em vez de acrescentar um segundo `pre-commit:` — evita chave YAML
      duplicada. Recusa conservadoramente quando o layout é desconhecido
- [x] Reexecutar → **zero diff** (idempotente), afirmado por teste que compara bytes
- [x] 🔴 Os testes percorrem `cfg.Hooks = "lefthook"`, e há contra-braço `Hooks: "none"` provando
      que o caminho não-interativo **não** cria o arquivo
- [x] 5 frases de reconciliação, todas afirmando o que foi **medido** (contagem de linhas, bytes,
      `os.Stat`) — diferente do ML-1A, que deduziu
- [x] 🔴 **Correção de auditoria:** o sítio ficou com a guarda real **e** o marcador de isenção
      `consumer-config-merge-allowed:`. Como o gate é um **OR**, um refator que extraísse o
      `os.ReadFile` deixaria o marcador isentando sozinho. Marcador removido, e a correção
      **falsificada**: pré-fix (marcador, sem `ReadFile`) → **PASS falso**; pós-fix → **FAIL**

### ML-1D — 🔴 o gate do ML-1B aceita `os.ReadFile` em COMENTÁRIO como guarda
**Owner:** `apolo-tf`
**Status:** ✅ Concluído — auditado em 2026-09-28
**Arquivos:** `scripts/check-init-preserves-user-config.sh` **apenas**

**Achado da auditoria do arquiteto, medido em 2026-09-28.** O gate criado por esta REQ para impedir
a reintrodução **não impediria a reintrodução**: ele procura o token `os.ReadFile` sem excluir
linhas de comentário.

```
# cópia com a guarda REAL removida, mencionada só em comentário:
//  antes aqui havia os.ReadFile("lefthook.yml") — removido por um refator
    var existing []byte

→ OK [lefthook.yml[literal]] ... os.ReadFile precedes write in same function
→ PASS: all 4 consumer-config write site(s) are guarded        🔴 FALSO VERDE
```

Braço de controle (cópia fiel) também passa — então o falso verde é **indistinguível** do verde
legítimo. É a mesma classe de defeito que o `check-crlf-normalize-capture.sh` já pagou nesta base
(*"comentário derrotava o gate"*), e que a memória do arquiteto registra como *"medir com a régua,
não com grep"*. O executor do ML-1C **tropeçou nisto** — o primeiro rascunho do comentário dele
continha `os.ReadFile` literal e produziu falso positivo — e reportou como nota de processo, sem
perceber que era defeito do gate.

⚠️ **Nota de método para quem executar:** o gate resolve o alvo por `SCAFFOLD_FILE` (env var), e
**ignora argumento posicional**. Eu mesmo medi errado na primeira tentativa por passar o caminho
como `$1` — o gate leu a árvore real e eu quase reportei o resultado do arquivo errado. Use
`SCAFFOLD_FILE=<caminho> bash scripts/...`.

**Critérios de aceite:**
- [x] O discriminante ignora comentário de linha **e** de fim de linha (`sub(/[[:space:]]*\/\/.*$/)`),
      com o caveat de truncar em `//` dentro de string literal **declarado** no header do script
- [x] 🔴 **Falsificação nas duas direções**, e **reverificada pelo arquiteto com a fixture original
      do achado**: a cópia que dava falso verde agora → **FAIL**; cópia fiel → **PASS**
- [x] `--self-test` ganhou o **braço 3**, e ele é **discriminante, não decorativo**: o executor rodou
      a fixture do braço 3 contra o script **pré-fix** e obteve `PASS/RC=0` — prova de que o braço
      falharia antes e passa depois
- [x] Contagem e anti-vacuidade por alvo preservadas; `RC=0` na árvore real, `SELFTEST_RC=0` (3/3
      braços), `check-orphan-gates` OK sobre 50 scripts
- [x] Reconciliação escrita, e ela afirma o discriminante: numa mesma execução sobre a mesma
      fixture, o gate **reprova** o sítio cuja guarda está só em comentário e **aceita** o sítio
      adjacente cuja guarda é código executável
- [x] 🔴 **Defeito ESPELHO achado e corrigido no mesmo ML** (Regra Dura, mesma causa): a detecção do
      *write-site* por `grep -nE` também não filtrava comentários — um `os.WriteFile` comentado
      satisfazia a anti-vacuidade e produzia falso verde pelo outro lado. Corrigido com
      `grep -Ev '^[0-9]+:[[:space:]]*//'`, e falsificado

### ML-1E — o predicado de idempotência do lefthook pergunta "o texto aparece?" em vez de "o hook está no lugar?"
**Owner:** `apolo-tf`
**Status:** ✅ Concluído — auditado em 2026-09-28 · **a auditoria achou um caso não testado**
**Arquivos:** `internal/generators/scaffold.go` (`generateLefthookHook`), `scripts/check-init-preserves-user-config.sh` (só o header)

**Achado da Wave 2, e o arquiteto ampliou o escopo ao medir.** O Hades reportou o defeito pelo braço
do **comentário** (`# trackfw-validate:` fazendo o `init` virar no-op). Medi e o defeito é **maior**:

```go
if strings.Contains(existingStr, "trackfw-validate:") { return nil }   // linha ~2970
```

`strings.Contains` casa **qualquer ocorrência em qualquer lugar**. Medido com um `lefthook.yml`
legítimo, **sem comentário nenhum** — consumidor que roda o validate no **push** em vez do commit:

```
pre-commit:
  commands:
    lint: {run: golangci-lint run}
pre-push:
  commands:
    trackfw-validate: {run: trackfw validate}     ← configuração legítima

strings.Contains(arquivo, "trackfw-validate:") = true
→ o init declara idempotência e NUNCA instala o hook em pre-commit:
```

🔴 O predicado pergunta *"o texto aparece no arquivo?"*; deveria perguntar *"o hook está instalado no
lugar certo?"*. Corrigido isso, o caso do comentário cai por consequência — e a questão filosófica
*"comentar é desabilitar deliberadamente?"* **deixa de precisar de resposta**.

**Critérios de aceite:**
- [x] O predicado verifica presença **dentro** de `pre-commit:` → `commands:`, não no arquivo inteiro.
      Reuse o caminho que o merge já percorre.
- [x] 🔴 **Teste no braço do `pre-push:`** (sem comentário): hook sob `pre-push:` → `trackfw-validate`
      **É** instalado em `pre-commit:`, e o `pre-push:` do consumidor é **preservado**
- [x] Contra-braço: hook **já** em `pre-commit:` → no-op, zero diff (idempotência real preservada)
- [x] Braço do comentário: `# trackfw-validate:` fora de `commands:` → tratado como ausente
- [x] **Header do gate ganha o limite declarado:** *"este gate é estrutural — detecta ausência de
      leitura, não uso semântico dela; um `os.ReadFile` decorativo o satisfaz"*.
      🔴 **NÃO** tornar o marcador obrigatório: o decoy da Wave 2 passaria igual, bastando colar o
      marcador. O ML-1C já removeu o marcador deste sítio **por medição**, e a falsificação provou que
      fortaleceu o gate. Limite conhecido se **declara**; não se finge corrigir.
- [x] Reconciliação: cada teste novo declara, em uma frase, o que **mediu**

**Entregue:** `lefthookValidatePresent(content)`, que rastreia seção de nível 0 e só conta
`trackfw-validate:` **dentro** de `pre-commit:`. Gate com **só comentários** alterados (verificado por
`git diff` filtrando linhas `#` — saída vazia). Testes T6 (braço `pre-push:`) e T7 (comentário).

### ML-1F — 🔴 comentário inline após a chave faz o merge DUPLICAR a entrada
**Owner:** `apolo-tf`
**Status:** ✅ Concluído — auditado em 2026-09-28 · **com resíduo DECLARADO, não perseguido**
**Arquivos:** `internal/generators/scaffold.go` (`lefthookValidatePresent`), teste em
`internal/generators/lefthook_hook_merge_test.go`

**Achado da auditoria do arquiteto ao ML-1E, por falsificação de um caso que nenhum teste cobriu.**
Replicei a função entregue e medi 4 formas do hook já instalado:

```
hook instalado, limpo            presente=true    ✅
hook instalado + comentário inline presente=false  🔴
hook só sob pre-push             presente=false   ✅ (é o fix do ML-1E)
hook com espaço no fim da linha  presente=true    ✅
```

A comparação é `strings.TrimSpace(line) == "trackfw-validate:"` — **exata**. Um
`trackfw-validate: # instalado pelo trackfw` é **YAML válido** e não casa. Efeito: o predicado diz
"ausente", o merge insere **uma segunda** entrada `trackfw-validate:` dentro do mesmo `commands:`, e
o `lefthook.yml` do consumidor fica com **chave YAML duplicada** — exatamente a corrupção que o ML-1C
evitou ao inserir dentro do bloco em vez de acrescentar outro `pre-commit:`.

🔴 **Severidade: é corrupção de arquivo do consumidor** — a classe mais grave desta REQ, não cosmético.

**A lição já estava na base:** o ML-1D aprendeu a **filtrar comentário** no gate (`awk` truncando
`//`). A mesma operação falta no produto, com `#`. O defeito atravessou gate e produto na mesma
sessão, e só o gate recebeu a correção.

**Critérios de aceite:**
- [x] `lefthookValidatePresent` trunca comentário inline (`#`) antes de comparar
- [x] 🔴 **Os 4 braços medidos acima entram como teste**, com o resultado esperado de cada um
- [x] 🔴 **Contra-braço de corrupção:** partindo de `trackfw-validate: # cmt` já instalado, rodar o
      merge **não** produz uma segunda entrada — conte as ocorrências de `trackfw-validate:` dentro
      de `commands:` e exija **exatamente 1**
- [x] `#` dentro de valor entre aspas não é tratado como comentário — **ou**, se for, o caveat é
      **declarado** no comentário da função (o gate declarou o seu; faça o mesmo)
- [x] Reconciliação: uma frase por teste, dizendo o que **mediu**

**Entregue:** truncamento de `" #"` antes da comparação, caveat de `#` em valor citado declarado no
docblock, 4 braços como teste (T8) e contra-braço de corrupção com **contagem literal = 1** (T9).

**Auditoria do arquiteto — repliquei e medi 6 variantes, não as 4 entregues:**

| variante | presente | veredito |
|---|---|---|
| limpo | `true` | ✅ |
| comentário inline (espaço) | `true` | ✅ — é o fix |
| dois espaços antes do `#` | `true` | ✅ |
| espaço no fim da linha | `true` | ✅ |
| só sob `pre-push:` | `false` | ✅ — fix do ML-1E não regrediu |
| 🔴 **TAB antes do `#`** | `false` | **resíduo** |

### 🔴 Resíduo DECLARADO — e a decisão de parar

`trackfw-validate:\t# cmt` (TAB entre `:` e `#`) não casa o `strings.Index(trimmed, " #")` e
produziria a duplicação. **Decidido: não corrigir, e declarar.**

**Razão:** YAML **proíbe TAB como indentação**; TAB depois de `:` é forma patológica que nenhum
editor ou ferramenta emite. O custo de mais um ciclo de ML excede o risco.

🔴 **E a razão de método, que importa mais:** perseguir variantes de whitespace indefinidamente é
cavar sem retorno. Cada rodada desta REQ encontrou um caso novo — `pre-push:`, comentário inline,
TAB — e **sempre encontrará**, porque o espaço de entradas malformadas é infinito. O ponto de parada
correto é quando o próximo caso deixa de ser plausível no mundo real. **Este é esse ponto.** O
resíduo fica escrito aqui, para que quem o encontrar amanhã saiba que foi **medido e decidido**, não
esquecido.

### ML-1G — o marcador `write-containment-allowed:` se perdeu na cadeia ML-1C → ML-1D
**Owner:** `apolo-tf`
**Status:** ✅ Concluído — auditado em 2026-09-28
**Arquivos:** `internal/generators/scaffold.go` (só comentário)

🔴 **Regressão introduzida pelo arquiteto, e registrada como tal.** Na `main`, o `os.WriteFile` do
`lefthook.yml` tinha `// write-containment-allowed: guarded by pathguard.RejectSymlinks…`. O ML-1C
substituiu esse comentário por `// consumer-config-merge-allowed: …`; eu então pedi a **remoção** do
`consumer-config-merge-allowed:` (porque enfraquecia *outro* gate), e o `write-containment-allowed:`
— de **escopo diferente** (pathguard) — foi junto.

```
FAIL [write-containment] unjustified write at internal/generators/scaffold.go:3056
check-write-containment: FAIL   ·   make: *** [parity-rest] Error 1
```

🔴 **Por que só apareceu no fim: eu proibi `make quality` em todos os handoffs** (custo de CPU,
árvore em movimento). Nenhum executor podia ver. A proibição é defensável para frentes paralelas —
mas ela **cega o executor para regressão em gate vizinho**, e o custo reapareceu inteiro no fim.
Esta tarefa foi a única em que autorizei `make quality`, e foi ela que fechou.

**Critérios de aceite:**
- [x] Marcador restaurado, com frase distinguindo os **dois** gates (contenção de escrita × preservação de config)
- [x] As 4 linhas de comentário do ML-1D **preservadas** — documentam decisão medida
- [x] `grep -c 'consumer-config-merge-allowed'` → **0**
- [x] O gate de config continua passando por `os.ReadFile precedes write`, **não** por marcador
- [x] `make quality` **RC=0**

## Wave 2 — auditoria independente
> Dependências: Wave 1 completa e auditada.

### ML-2A — revisão independente por reimplementação
**Owner:** `hades-tf`
**Status:** ✅ Concluído — auditado em 2026-09-28 · **3 achados, 1 toca produto**
**Tarefa:** reimplementar a verificação **a partir da leitura da ADR e da REQ**, não conferindo o
diff. A Regra Dura de Reconciliação pega contradição interna; **não** pega premissa errada
compartilhada entre teste e implementação. Esta wave existe para isso.

**Critérios de aceite:**
- [x] O cenário do dano real reproduzido do zero: `governance_mode` + `lenient_until` + `agent_models`
      com comentário → `init` → `validate` byte-idêntico
- [x] Veredito explícito sobre se algum sítio **(a)** ficou sem correção — a ADR de ponto único
      **não** está satisfeita enquanto sobrar sítio

**Resultado — 10 ataques a P1–P4 todos corretos** (bloco literal `|`, prefixo compartilhado, chave
duplicada, sem newline final, arquivo vazio, só comentários, CRLF, TAB, indentação de 1 espaço, chave
em coluna 0 após `|`). O cenário do dano real reconstruído do zero: `validate` **byte-idêntico**.

⚠️ **Nota de leitura:** o parecer usa `D1`/`D2` em **duas** numerações diferentes (defeitos *e* casos
de ataque). Aqui os achados são citados por **nome**, para o próximo leitor não cruzar as tabelas.

**Achado 1 — predicado de idempotência do lefthook pergunta a coisa errada** → vira **ML-1E**.
**Achado 2 — o gate não distingue leitura usada de leitura decorativa** → **limite declarado**, não corrigido.
**Achado 3 — sub-chave nova sob bloco existente não é entregue** → **decisão**, ADR Emenda 1.
