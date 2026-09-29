---
status: done
date: 2026-09-29
req: "docs/req/REQ-2026-09-29-trackfw-context-reporta-adrs-zero-onde-status-reporta-145-e-o-agente-que-roda-context-primeiro-conclui-que-nao-ha-decisoes-arquiteturais.md"
squad: ""
---

# Roadmap: `trackfw context` reporta `ADRs (0)` onde `status` reporta 145

> Created: 2026-09-29 | Status: done

## Context
<!-- Derived from REQ -->
REQ: docs/req/REQ-2026-09-29-trackfw-context-reporta-adrs-zero-onde-status-reporta-145-e-o-agente-que-roda-context-primeiro-conclui-que-nao-ha-decisoes-arquiteturais.md
ADR: docs/adr/ADR-2026-09-29-a-enumeracao-de-adr-e-uniao-de-layouts-e-context-e-status-consomem-o-mesmo-ponto-unico.md
Origem: **#450**. Causa: `context.go:39` faz `os.ReadDir(adrDir)` sem descer nas subpastas de estado.

## Acceptance Criteria
<!-- Consolidados; detalhe por ML nas waves. -->
- [x] Todos os sítios que enumeram ADR levantados e classificados
- [x] `context` e `status` consomem o **mesmo** resolvedor (ADR D3)
- [x] Layout plano continua funcionando **e** subpastas de estado passam a ser enumeradas
- [x] A saída do `context` deixa de poder dizer `ADRs (0)` e nomear um ADR num warning
- [x] `Governance score` medido antes/depois: diferença de **exatamente 20 pontos**
- [x] Gate falsificável impedindo enumerador novo fora do ponto único
- [x] `make quality` **RC=0** — rodado pelo arquiteto, 1381 `OK` (os `FAIL` do log são fixtures
      dos braços de self-test). CI: ver PR

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat model: quantos enumeradores de ADR existem, e quais estão errados
> Dependências: nenhuma. 🔴 **Bloqueia a implementação.**

**Gates da wave:**

```bash
n=$(grep -rn 'os.ReadDir\|filepath.WalkDir\|filepath.Glob' --include='*.go' internal/ | grep -v _test | wc -l | tr -d ' '); test "$n" -gt 0 && echo "Gate W0: $n sitios de enumeracao de diretorio no produto — a triagem parte deste universo" || { echo "GATE FALHOU: zero sitios de enumeracao encontrados — a regua esta quebrada, nao o produto" >&2; exit 1; }
```

### ML-0A — enumerar os sítios que leem ADR e medir a divergência
**Owner:** `hades-tf`
**Status:** ✅ Concluído — auditado em 2026-09-29 · 🔴 **refutou a ADR em dois pontos**
**Arquivos:** leitura de `internal/`; escrita em `docs/seguranca/2026-09-29-wave0-context-adr-zero.md`

**Tarefa:**
1. Levantar **todos** os sítios que enumeram ADR — não só `context` e `status`. Inclua `serve`,
   `validate`, `discover`, `metrics` e o que mais existir.
2. Classificar cada um: **(i)** usa o ponto único · **(ii)** implementação própria **correta** ·
   **(iii)** implementação própria **errada**.
3. 🔴 **Medir a divergência entre comandos numa mesma fixture** — é o sintoma que denuncia
   implementação duplicada. Rode todos os que reportam contagem de ADR e cole os números lado a lado.
4. 🔴 **Refutar ou confirmar a premissa da ADR** de que existe (ou é viável construir) **um** ponto
   único que sirva aos dois. Se `status` e `context` precisarem de dados diferentes (ex.: um precisa
   do status do frontmatter e o outro não), **diga agora** — isso muda o desenho do ML-1A.

**Critérios de aceite:**
- [x] **9 sítios**, 3 classe (iii) — a ADR supunha 2. `context.go:38`, `adr.go:189` (`adr list`), `adr.go:316` (`NewADRDraft`)
- [x] Fixture única, 4 ADRs em subpastas: `status`=**4** · `context`=**0** · `adr list`=**0** · `validate`=**4 nomeados**
- [x] **D3 CONFIRMADA** — `walkADRFilePaths(dir)` (primitivo, já existe) + `ResolveADRFiles(cfg)` (wrapper com dedup). Precedente direto: `context.go:60` já usa `validator.ResolveREQFiles(cfg)` para REQ
- [x] Layout plano (74 ADRs, sem subpastas): ambos os comandos corretos hoje, e o fix **não o afeta**

**Os dois pontos em que a Wave 0 me refutou:**

1. 🔴 **Afirmação FALSA na minha ADR.** Escrevi que *"o filtro por prefixo `ADR-` já é o usado pelo
   `status`"*. **Inventei — não medi.** `validator.go:3017` e `context.go:44` usam ambos
   `HasSuffix(".md")`, **sem prefixo**. O risco era concreto: o implementador acrescentaria o filtro
   *"para preservar comportamento"* e mudaria contagens em silêncio. Retratado na ADR.

2. **População 4,5× maior.** Eu nomeei 2 sítios; são 9, com 3 errados. `adr list` responde
   **"No ADRs found"** e `NewADRDraft` cria **rascunho duplicado** — ambos mesma causa, entram no
   mesmo ML pela Regra Dura.

**E um achado que o AC não pegaria:** `adr_dirs` **aninhadas** fazem o `status` reportar **7** onde o
correto é **4** (confirmado por mim). 🔴 Sem dedup, a correção faria os dois comandos **concordarem
no número errado** — e o AC *"delta de 20 pontos"* passaria assim mesmo, porque sem `adr_dirs`
aninhadas ele não discrimina. **Um AC que não distingue o certo do errado não é AC.**

**Resíduo declarado:** um `NOTAS.md` na raiz de `adr_dir` **é contado como ADR** (4 reais → 8
reportados). Causa distinta — critério de identificação, não alcance —, fora desta REQ, candidato a
issue própria.

## Wave 1 — o ponto único, e o gate que o sustenta
> Dependências: **Wave 0 auditada.** Os dois MLs tocam arquivos distintos → paralelos, **se** a
> Wave 0 confirmar que o resolvedor vive em arquivo próprio.

### ML-1A — resolvedor único de ADR, consumido por `context` e `status`
**Owner:** `apolo-tf`
**Status:** ✅ Concluído — auditado em 2026-09-29

**Forma decidida pela Wave 0:** `walkADRFilePaths(dir) []string` (primitivo, já existe) +
`ResolveADRFiles(cfg) []string` (wrapper: loop em `ADRDirs` + **dedup por caminho absoluto**).
Precedente a copiar: `context.go:60`, que já usa `validator.ResolveREQFiles(cfg)` para REQ.

**Critérios de aceite:**
- [x] Layout plano (`adr_dir/*.md`) **continua** funcionando — é o layout deste repositório
- [x] Layout com subpastas de estado passa a ser enumerado
- [x] `context` e `status` chamam o **mesmo** resolvedor
- [x] 🔴 **Os TRÊS sítios (iii)**: `context.go:38`, `adr.go:189` (`ListADRs`), `adr.go:316`
      (`NewADRDraft`). Deixar `adr list` de fora entregaria meia correção
- [x] 🔴 **Dedup**, com fixture de `adr_dirs` **aninhadas**: `[docs/adr/zeus, docs/adr/zeus/done]`
      com 4 ADRs reais tem que reportar **4**, não 7
- [x] 🔴 **O critério de identificação NÃO muda** — `HasSuffix(".md")`, **sem** filtro de prefixo.
      Teste que fixe isso, senão um refator futuro "melhora" o filtro e muda contagens em silêncio
- [x] 🔴 **O teste que mede o efeito:** numa fixture com ADRs em subpastas, a saída do `context`
      **não** contém `## ADRs (0)` junto de um warning que nomeia um ADR
- [x] 🔴 **Score medido antes/depois:** diferença de **exatamente 20 pontos**. Nem mais — se subir
      40, a mudança tocou outra categoria e isso precisa ser explicado
- [x] Reconciliação: uma frase por teste, dizendo o que **mediu**

### ML-1B — gate que impede enumerador de ADR fora do ponto único
**Owner:** `apolo-tf`
**Status:** ✅ Concluído — auditado em 2026-09-29

**Critérios de aceite:**
- [x] 🔴 Falsificável nas **duas** direções, com as execuções coladas
- [x] Anti-vacuidade: declara quantos sítios examinou e **reprova se examinar zero**
- [x] 🔴 **O discriminante ignora comentários** — este projeto já pagou por gate que aceitava token
      em comentário **duas vezes** (`check-crlf-normalize-capture.sh` e, em 2026-09-28,
      `check-init-preserves-user-config.sh`, nas duas direções). Falsifique esse caso explicitamente.

### ML-1C — 🔴 `ensureGlobalADRDirRegistered` lê `~/.trackfw/adr` com `Glob` raiz, e o diretório global nunca é registrado
**Owner:** `apolo-tf`
**Status:** ✅ Concluído — auditado em 2026-09-29
**Arquivos:** `internal/generators/update.go` (~linha 302), teste; e a **isenção** correspondente em
`scripts/check-adr-enumeration-single-point.sh`

**Achado da auditoria do arquiteto ao ML-1B.** O executor isentou este sítio classificando-o como
*"diretório global plano **por design**"*, e marcou a isenção como **provisória**. Investiguei a
premissa e ela **não se sustenta**:

```go
// update.go:302 — dentro de ensureGlobalADRDirRegistered
matches, globErr := filepath.Glob(filepath.Join(globalDir, "ADR-*.md"))
if len(matches) == 0 { return nil }   // no-op
```

🔴 **A função REGISTRA `~/.trackfw/adr` dentro de `adr_dirs`.** Ou seja, o mesmo diretório passa a ser
varrido **recursivamente** por `ResolveADRFiles(cfg)` — enquanto **este** sítio o lê com `Glob` raiz.
São **dois leitores do mesmo diretório com alcances diferentes**: exatamente a divergência que esta
REQ existe para eliminar.

**Efeito medido por leitura:** um usuário com ADRs globais **apenas em subpastas** obtém
`len(matches) == 0` → **no-op** → `~/.trackfw/adr` **nunca entra** em `adr_dirs` → os ADRs globais
ficam **invisíveis para todos os comandos**. Mesmo sintoma do #450, outra porta.

**Por que ML e não issue nova:** Regra Dura — mesma causa (leitura raiz onde deveria ser recursiva),
mesmo mecanismo, mesma ADR. Um sítio conhecido e não corrigido deixaria a ADR de ponto único
insatisfeita, que é o achado A1 que este projeto já pagou duas vezes.

**Critérios de aceite:**
- [x] A verificação de existência passa a ser **recursiva**, preservando a semântica *"há algum ADR
      neste diretório?"* — não transforme em enumeração para contagem
- [x] 🔴 **Teste no braço do achado:** `~/.trackfw/adr` com ADRs **apenas em subpasta** → o diretório
      **É** registrado em `adr_dirs`
- [x] **Contra-braço:** `~/.trackfw/adr` **vazio** ou inexistente → continua no-op, sem registrar
- [x] A **isenção deste sítio sai** do `check-adr-enumeration-single-point.sh`, e o gate continua
      `RC=0` — se a isenção precisar ficar, a razão tem que ser outra, escrita
- [x] Reconciliação: uma frase por teste, dizendo o que **mediu**

### ML-1D — 🔴 os warnings duplicam com `adr_dirs` aninhadas (mesma causa do D4)
**Owner:** `apolo-tf`
**Status:** ✅ Concluído — auditado em 2026-09-29
**Arquivos:** `internal/validator/` (regras `adr_orphan` e presença de frontmatter), teste
**⚠️ Frente paralela ML-1E em `scripts/` — não toque lá.**

**Achado da Wave 2, confirmado pelo arquiteto.** Com `adr_dirs: [zeus, zeus/done]` e **3** ADRs reais:

```
  ## ADRs (3)          ← contagem CORRETA (o D4 funcionou)
  ## Warnings (6)      ← 🔴 cada ADR aparece DUAS vezes
- adr "ADR-...-01-t.md" is not referenced by any REQ
- adr "ADR-...-02-t.md" is not referenced by any REQ
- adr "ADR-...-03-t.md" is not referenced by any REQ
- adr "ADR-...-01-t.md" is not referenced by any REQ   ← duplicado
...
```

Medido: **6 linhas** de warning para **3 ADRs únicos**.

🔴 **Não é só UX — é uma NOVA contradição interna**, da mesma família da que originou o #450: o
comando diz `## ADRs (3)` e emite **6** avisos sobre ADRs, na mesma saída.

**Por que entra nesta REQ, e não vira issue:** as regras de validação iteram `cfg.ADRDirs`
independentemente, **sem dedup por caminho absoluto** — **exatamente o mecanismo do D4**. Eu trouxe o
D4 para esta REQ com esse argumento; recusar F1 agora seria inconsistente, e a Regra Dura é explícita
que *"está fora do escopo declarado"* **não** justifica REQ nova: se a causa é a mesma, o escopo
estava estreito demais.

**Critérios de aceite:**
- [x] As regras de ADR consomem o resolvedor deduplicado (ou deduplicam por caminho absoluto)
- [x] 🔴 **Braço do achado:** `adr_dirs` aninhadas com N ADRs reais → **N** warnings, não 2N
- [x] **Contra-braço:** dois ADRs de **mesmo basename** em dirs **distintos e não aninhados** →
      **2** warnings, não 1. O dedup não pode suprimir avisos legítimos
- [x] Contagem e score **não regridem** (continuam corretos)
- [x] Reconciliação: uma frase por teste, dizendo o que **mediu**

### ML-1E — o gate é evadido por variável intermediária, e o AC prometia demais
**Owner:** `apolo-tf`
**Status:** ✅ Concluído — auditado em 2026-09-29
**Arquivos:** `scripts/check-adr-enumeration-single-point.sh` **apenas**
**⚠️ Frente paralela ML-1D em `internal/` — 🔴 não edite nenhum `.go`.**

**Achado da Wave 2, confirmado pelo arquiteto em fixture própria:**

```go
dirs := cfg.ADRDirs          // variável intermediária
for _, d := range dirs {
    entries, _ := os.ReadDir(d)   // → gate PASSA (RC=0). EVADIU.
}
```
Controle, mesma fixture com `range cfg.ADRDirs` direto → gate **RC=1**, detecta.

**Decisão do arquiteto — cobrir uma, declarar a outra:**
- **Evasão 1 (variável intermediária): COBRIR.** É refator **plausível sem má-fé** — alguém extrai a
  lista para uma variável e o gate silencia.
- **Evasão 2 (helper em outro escopo): DECLARAR como limite.** Exigiria análise de fluxo, inviável
  em análise textual. É a mesma classe do limite já declarado no
  `check-init-preserves-user-config.sh` (*"estrutural, não semântico"*). **Limite conhecido se
  declara; não se finge corrigir.**
- **O AC da REQ foi AJUSTADO** para descrever o que o gate entrega. Um AC que promete mais do que o
  artefato faz transforma "gate passou" em evidência de uma garantia que não existe.

**Critérios de aceite:**
- [x] Evasão por **variável intermediária** passa a ser detectada
- [x] 🔴 **Falsificação nas duas direções:** a fixture da evasão → **REPROVA**; a árvore real e o
      ponto único → **PASSAM** (`RC=0`, ~110 arquivos)
- [x] **Braço do comentário preservado** — a evasão por comentário continua não reprovando
- [x] **Limite declarado no header:** enumeração via helper em outro escopo **não** é detectada, com
      a razão (análise textual não faz análise de fluxo)
- [x] `--self-test` ganha braço para a evasão coberta, e ele é **discriminante**: rode-o contra o
      script **pré-fix** e mostre que falharia
- [x] `check-orphan-gates` OK

## Wave 2 — auditoria independente
> Dependências: Wave 1 completa e auditada.

### ML-2A — revisão por reimplementação
**Owner:** `hades-tf`
**Status:** ✅ Concluído — auditado em 2026-09-29 · **aprova com 2 ressalvas, ambas aceitas**
**Método:** 🔴 **não conferir o diff.** Ler ADR e REQ, derivar o esperado, medir o binário como
caixa-preta. A Regra Dura de Reconciliação pega contradição interna; **não** pega premissa errada
compartilhada entre implementação e teste.

**Critérios de aceite:**
- [x] Cenário do #450 reconstruído do zero (`adr_dirs` com subpastas), `context` e `status` concordando
- [x] Ataque a layouts adversariais: `adr_dir` inexistente · vazio · com subpasta sem `.md` ·
      `.md` que não começa com `ADR-` · symlink · subpasta aninhada em dois níveis
- [x] Veredito explícito: sobrou enumerador de ADR fora do ponto único? A ADR **não** está satisfeita
      enquanto sobrar sítio


**Resultado:** 10 casos adversariais corretos (dir inexistente, vazio, sem `.md`, 2 níveis, symlink de
arquivo, espaço no caminho, barra final, `./`, `NOTAS.md` contado conforme declarado). Dedup correto
**nas duas direções**: aninhadas fundem, mesmo basename em dirs distintos **não** funde. Score delta
**20** isolado. E ela verificou que `NewADR` **não** é um 10º sítio — `adr new` é escrita pura.

**Ressalva F2 (informacional, não bloqueia):** `filepath.WalkDir` **não segue symlink de diretório**;
`adr_dirs` apontando para symlink de dir acha 0 ADRs. Não está nos requisitos e não é regressão — o
`Glob`/`ReadDir` anterior também não seguia. Registrado para rastreabilidade.

---

## 🔴 Achado de PRODUTO durante esta REQ — fora do escopo, issue própria

**O `barrier` não é fence-aware.** Linhas que começam com `## ` **dentro de um bloco de código**
são lidas como headings de seção. Medido em 2026-09-29, neste próprio roadmap:

```
barrier --wave 1 → ✗ acceptance_evidence: blocked
                     - ML-1D: no acceptance block
```

O ML-1D **tem** bloco de aceite. Mas o exemplo de saída dele contém:

```
  ## ADRs (3)
  ## Warnings (6)
```

…e o parser encerra a seção do ML ali, antes de chegar em `**Critérios de aceite:**`.

**Contorno aplicado aqui:** indentar as duas linhas dentro do fence (conteúdo intacto, deixa de
parecer heading). **Não é correção** — é contorno, e está escrito para não ser confundido com uma.

🔴 **Por que NÃO entra nesta REQ:** aplicando o teste da Regra Dura — *"se eu corrigir esta causa,
exatamente estas falhas fecham"* — corrigir o parser do `barrier` **não** tem relação com enumeração
de ADR. Causa distinta, superfície distinta, ADR distinta. **Issue própria.**

**Por que vale a issue:** um roadmap que documenta saída de comando em bloco de código — que é
prática **encorajada** neste projeto, porque evidência colada vale mais que prosa — pode ter MLs
silenciosamente invisíveis ao `barrier`. O modo de falha é o pior possível: **o gate reporta
`no acceptance block` para um ML que tem o bloco**, e o autor procura no lugar errado.

⚠️ E o mesmo defeito apareceu no **meu próprio script de verificação** na mesma sessão, pela mesma
razão. A lição é do domínio, não do trackfw: **parser de markdown ingênuo confunde conteúdo de fence
com estrutura**.
