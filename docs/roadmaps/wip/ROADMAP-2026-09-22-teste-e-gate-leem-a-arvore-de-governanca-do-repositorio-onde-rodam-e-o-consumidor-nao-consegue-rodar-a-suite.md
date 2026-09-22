---
status: wip
date: 2026-09-22
req: "docs/req/REQ-2026-09-22-teste-e-gate-leem-a-arvore-de-governanca-do-repositorio-onde-rodam-e-o-consumidor-nao-consegue-rodar-a-suite.md"
squad: "hades-tf, apolo-tf, artemis-tf"
---

# Roadmap: teste e gate leem a árvore de governança do repositório onde rodam

> Created: 2026-09-22 | Status: wip

## Context

REQ: `docs/req/REQ-2026-09-22-teste-e-gate-leem-a-arvore-de-governanca-do-repositorio-onde-rodam-e-o-consumidor-nao-consegue-rodar-a-suite.md`
Origem: **#396** (consumidor externo, com medição). Mesma classe declarada: **#277**, e o achado 16
do #216 (fechado).

O produto se propõe a governar o projeto de quem o instala. Quando um teste ou gate lê a árvore de
governança **do repositório onde roda** e presume o layout do mantenedor, a suíte fica inalcançável
para o consumidor — a proposta de valor sendo negada pelo próprio produto.

## Acceptance Criteria

- [ ] Enumeração real da população, classificada em (a) exige do consumidor · (b) audita o upstream,
      legítimo · (c) já usa fixture própria
- [ ] Todo sítio (a) corrigido
- [ ] Nenhum artefato declara o que não sustenta (Regra Dura de Reconciliação)
- [ ] Falsificação nas duas direções, incluindo o controle
- [ ] Prova de execução num consumidor `by_agent`, sem importar governança alheia
- [ ] `make quality` e **CI** verdes

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

---

## Wave 0 — Enumeração e modelo de ameaça
> Dependências: nenhuma. **Bloqueia toda a implementação.**

### ML-0A — enumerar a população e classificar cada sítio
**Status:** ⬜ Pendente · **Papel:** `hades-tf`
**Files affected:** nenhum de produto — documento em `docs/seguranca/`

**Contexto que você NÃO precisa remedir:** o #396 já está medido pelo reportante, e eu confirmei o
sítio. O seu trabalho é a **população**, não o caso individual.

**Actions:**
1. 🔴 **Enumeração pelo critério certo.** O critério é *"lê a árvore de governança do repositório
   onde roda"* — não *"menciona `docs/`"*. Minha medição inicial é **aproximada e você deve
   refutá-la ou confirmá-la**:
   ```
   $ grep -rn "repoRoot(" --include='*_test.go' internal/ | grep -v "func repoRoot" | wc -l
   1
   $ grep -rln 'docs/roadmaps\|docs/req\|docs/adr' scripts/*.sh | wc -l
   14
   ```
   ⚠️ O `14` **não** é a resposta — a maioria pode ser legítima. E o `1` pode estar subestimado:
   um teste pode alcançar a árvore real por `os.Getwd()`, por caminho relativo `../..`, ou por
   helper com outro nome. **Procure o mecanismo, não o literal.**
2. **Classifique cada sítio** em:
   - **(a)** exige a governança do consumidor → **defeito**
   - **(b)** audita o upstream e é legítimo → mantém, mas deve declarar isso
   - **(c)** já usa fixture própria → correto
3. **Modelo de ameaça.** Quem esvazia esta Wave 0 sem quebrar regra escrita? O caso óbvio: declarar
   tudo como (b) e não corrigir nada. Qual é o teste que distingue (a) de (b) sem depender de quem
   classifica?
4. **Falsificação nas duas direções.** Para cada superfície: o que quebra quando o produto regride,
   e o que quebra quando a correção vai longe demais — um gate que deixa de auditar a governança do
   upstream é regressão silenciosa, não melhoria.
5. **Residual declarado.**

**Acceptance criteria:**
- [ ] Tabela completa, um veredito (a)/(b)/(c) por sítio, com `arquivo:linha` e o comando que
      produziu a lista
- [ ] 🔴 O critério que distingue (a) de (b) está **escrito e é aplicável por terceiro** — não
      "julgamento do revisor"
- [ ] As quatro seções com evidência, não asserção de uma linha
- [ ] Nenhuma linha de implementação escrita neste ML

**Gate da wave:** `trackfw barrier <roadmap> --wave 0`, auditado por mim antes de qualquer despacho.

---

## Wave 1 — A correção
> Dependências: Wave 0 completa e **auditada**. Escopo definido pela tabela do ML-0A.

### ML-1A — `TestCorpusMeasurement_ReportOnly` cumpre o que declara (#396)
**Status:** ⬜ Pendente · **Papel:** `artemis-tf`
**Files affected:** `internal/roadmapdoc/roadmapdoc_test.go` — e só este

**O defeito, já medido:** o cabeçalho da seção (linha 242) diz **"report-only, never fails"** e a
linha 253 faz `t.Fatalf`. Contradição interna literal — a **Regra Dura de Reconciliação** na forma
exata do achado A1 da auditoria externa de 2026-09-05.

Num consumidor `by_agent`, `docs/roadmaps/done` não existe e a cascata derruba `go`,
`windows-full-suites`, e pula `parity-falsify-shard` e `parity-other-gates`.

🔴 **Medição do reportante que decide o desenho:** com `docs/roadmaps/done/` **vazio** (só
`.gitkeep`), o teste **passa** e o `validate` segue com 0 violações. **Então ele não mede nada do
produto quando o diretório está vazio** — ele mede o corpus do mantenedor.

**Duas saídas possíveis, e a escolha precisa de justificativa escrita:**
- **skip declarado** quando o diretório não existe — cumpre o "never fails" do próprio docstring;
- **resolver o diretório pela config** (`roadmap_dir` + `roadmap_namespacing`) — mede o corpus de
  quem roda, seja qual for o layout.

O reportante sugere as duas e diz *"a escolha é sua"*. **Escolha uma, escreva por quê, e diga o que
a outra daria.** Se escolher skip: o teste passa a não medir nada no consumidor — isso é aceitável?
Se escolher config: ele passa a medir corpus alheio — o número resultante significa o quê?

**Acceptance criteria:**
- [ ] O cabeçalho e o corpo **concordam** — se diz "never fails", não há `t.Fatalf`
- [ ] Teste **load-bearing**: prove com um repositório sem `docs/roadmaps/done` que o comportamento
      novo difere do antigo. Cole as duas saídas
- [ ] 🔴 **Braço (b):** no layout plano deste repositório, a medição **continua acontecendo** — a
      correção não pode transformar o teste em no-op para o upstream
- [ ] `go test ./internal/roadmapdoc/` RC=0
- [ ] 🔴 **NÃO rodar `make quality`** — barreira é do arquiteto
- [ ] Uma frase declarando qual conclusão do ML o teste afirma

### ML-1B — sítios (a) restantes
**Status:** ⬜ Pendente · **Papel:** a definir pela tabela do ML-0A
**Files affected:** definidos pelo ML-0A

Inclui, se a Wave 0 confirmar: o corpus do `check-roadmap-barrier-contract` (**#277**), cuja proposta
do reportante é separar as **fontes** — corpus vira fixture em `scripts/testdata/` (roda em qualquer
clone) e a tripwire de disco vira gate separado, executado só no upstream ou atrás de
`TRACKFW_SELF_GOVERNED=1`.

🔴 **Não presuma que #277 entra.** Ele entra se a Wave 0 medir que é a mesma causa. Se a medição
disser que a causa é outra, ele sai — **com a medição escrita**, como manda a Regra Dura.

**Acceptance criteria:** definidos após o ML-0A.

---

## Barreira final

Revisão `hefesto-tf` e `hades-tf`, auditoria do arquiteto, `trackfw barrier`.
🔴 **CI verde, não só verde local** — e, neste roadmap especificamente, a prova que importa é a
**execução num consumidor `by_agent`**, porque é exatamente o ambiente que o defeito quebra e que o
CI do mantenedor **não** exercita.

⚠️ Custo de CPU: ver `vault/notes/carga-de-cpu-vem-da-suite-de-falsificacao-vezes-agentes-paralelos-2026-09-22.md`
— teste do pacote tocado nos handoffs, `TRACKFW_FALSIFY_JOBS=4` na barreira local.
