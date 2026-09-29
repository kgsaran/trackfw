---
status: wip
date: 2026-09-29
req: "docs/req/REQ-2026-09-29-trackfw-context-reporta-adrs-zero-onde-status-reporta-145-e-o-agente-que-roda-context-primeiro-conclui-que-nao-ha-decisoes-arquiteturais.md"
squad: ""
---

# Roadmap: `trackfw context` reporta `ADRs (0)` onde `status` reporta 145

> Created: 2026-09-29 | Status: wip

## Context
<!-- Derived from REQ -->
REQ: docs/req/REQ-2026-09-29-trackfw-context-reporta-adrs-zero-onde-status-reporta-145-e-o-agente-que-roda-context-primeiro-conclui-que-nao-ha-decisoes-arquiteturais.md
ADR: docs/adr/ADR-2026-09-29-a-enumeracao-de-adr-e-uniao-de-layouts-e-context-e-status-consomem-o-mesmo-ponto-unico.md
Origem: **#450**. Causa: `context.go:39` faz `os.ReadDir(adrDir)` sem descer nas subpastas de estado.

## Acceptance Criteria
<!-- Consolidados; detalhe por ML nas waves. -->
- [ ] Todos os sítios que enumeram ADR levantados e classificados
- [ ] `context` e `status` consomem o **mesmo** resolvedor (ADR D3)
- [ ] Layout plano continua funcionando **e** subpastas de estado passam a ser enumeradas
- [ ] A saída do `context` deixa de poder dizer `ADRs (0)` e nomear um ADR num warning
- [ ] `Governance score` medido antes/depois: diferença de **exatamente 20 pontos**
- [ ] Gate falsificável impedindo enumerador novo fora do ponto único
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

## Wave 2 — auditoria independente
> Dependências: Wave 1 completa e auditada.

### ML-2A — revisão por reimplementação
**Owner:** `hades-tf`
**Status:** ⬜ Pendente
**Método:** 🔴 **não conferir o diff.** Ler ADR e REQ, derivar o esperado, medir o binário como
caixa-preta. A Regra Dura de Reconciliação pega contradição interna; **não** pega premissa errada
compartilhada entre implementação e teste.

**Critérios de aceite:**
- [ ] Cenário do #450 reconstruído do zero (`adr_dirs` com subpastas), `context` e `status` concordando
- [ ] Ataque a layouts adversariais: `adr_dir` inexistente · vazio · com subpasta sem `.md` ·
      `.md` que não começa com `ADR-` · symlink · subpasta aninhada em dois níveis
- [ ] Veredito explícito: sobrou enumerador de ADR fora do ponto único? A ADR **não** está satisfeita
      enquanto sobrar sítio
