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
- [ ] Os 22 sítios enumerados e classificados em (a)/(b)/(c), com a razão escrita por sítio
- [ ] `init` reexecutado preserva `governance_mode`, `lenient_until` e `agent_models` — e a saída de
      `trackfw validate` é **byte-idêntica** antes e depois
- [ ] Comentários preservados, incluindo a justificativa de cota do bloco `agent_models`
- [ ] Zero diff nas linhas pré-existentes; chave nova de versão nova **É** acrescentada (contra-braço)
- [ ] Gate falsificável nas duas direções: reprova sítio (a) novo que trunque; **não** reprova (b) legítimo
- [ ] `make quality` e CI verdes

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat model: medir o que é defeito e o que é comportamento desejado
> Dependências: nenhuma. 🔴 **Bloqueia toda a implementação.**

**Gates da wave:**

```bash
n=$(grep -cE '^[[:space:]]*(if err := )?os\.(WriteFile|OpenFile)' internal/generators/scaffold.go); test "$n" = "22" && echo "Gate W0: $n sitios reais de escrita (regua com ancora de inicio de linha)" || { echo "GATE FALHOU: esperava 22 sitios, contou $n — a populacao mudou, reclassifique antes de implementar" >&2; exit 1; }
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
- [ ] `trackfw.yaml` ausente → escreve o template inteiro (comportamento atual preservado)
- [ ] `trackfw.yaml` presente → preserva **todo** valor existente e acrescenta **só** chaves ausentes
- [ ] 🔴 Comentários preservados, inclusive comentário de linha ao lado de valor
- [x] 🔴 **Zero diff** nas linhas pré-existentes — medido pelo arquiteto: o diff do `trackfw.yaml`
      após `init` mostra **só chaves novas acrescentadas ao fim**, nenhuma linha anterior tocada a acrescentar
- [ ] **Contra-braço:** chave nova de versão nova **É** acrescentada — sem ele, "preservar" degenera
      em "não escrever nada"
- [ ] 🔴 **O teste que mede o efeito:** fixture com `governance_mode: lenient` + `lenient_until` +
      `agent_models` comentado; `validate` **byte-idêntico** antes e depois do `init`
- [ ] Reconciliação: cada teste novo declara, em uma frase, qual conclusão do ML afirma

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

## Wave 2 — auditoria independente
> Dependências: Wave 1 completa e auditada.

### ML-2A — revisão independente por reimplementação
**Owner:** `hades-tf`
**Status:** ⬜ Pendente
**Tarefa:** reimplementar a verificação **a partir da leitura da ADR e da REQ**, não conferindo o
diff. A Regra Dura de Reconciliação pega contradição interna; **não** pega premissa errada
compartilhada entre teste e implementação. Esta wave existe para isso.

**Critérios de aceite:**
- [ ] O cenário do dano real reproduzido do zero: `governance_mode` + `lenient_until` + `agent_models`
      com comentário → `init` → `validate` byte-idêntico
- [ ] Veredito explícito sobre se algum sítio **(a)** ficou sem correção — a ADR de ponto único
      **não** está satisfeita enquanto sobrar sítio
