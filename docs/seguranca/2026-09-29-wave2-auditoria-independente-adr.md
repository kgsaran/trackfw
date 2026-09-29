# Wave 2 — Auditoria Independente (Reimplementação): REQ-2026-09-29

> Data: 2026-09-29 | Hades | Branch: fix/context-reporta-zero-adrs-onde-status-reporta-145

---

## Método

Leitura de ADR, REQ e Wave 0. Derivação independente do esperado. Medição caixa-preta do binário
compilado (`go build -o bin/trackfw ./cmd/trackfw`, RC=0). Código-fonte consultado somente quando
resultado divergiu do esperado ou quando a medição caixa-preta era impossível sem TTY.

---

## 1. Cenário do #450 — concordância entre context e status

**O que ADR/REQ esperam:** `context` e `status` reportam o mesmo número de ADRs; `context` não
exibe `## ADRs (0)` junto de warning nomeando um ADR; `adr list` lista os ADRs.

**Fixture:** `adr_dirs: [docs/adr/zeus]`, 3 ADRs em `done/`, 1 em `wip/`. Total = 4.

**Comando e saída literal:**

```
$ trackfw context
Governance score: 60/100
## ADRs (4)
- ADR-2026-09-01-teste.md [Accepted]
- ADR-2026-09-02-teste-b.md [Accepted]
- ADR-2026-09-03-teste-c.md [Accepted]
- ADR-2026-09-04-teste-d.md [Draft]
## Warnings (4)
- adr "ADR-2026-09-01-teste.md" is not referenced by any REQ
...

$ trackfw status
ADRs  4

$ trackfw adr list
ADR-2026-09-01-teste.md    unknown
ADR-2026-09-02-teste-b.md  unknown
ADR-2026-09-03-teste-c.md  unknown
ADR-2026-09-04-teste-d.md  unknown
```

**Veredito:** CORRETO. Contradição interna desapareceu. `context` e `status` concordam em 4.
`adr list` enumera todos os 4 ADRs.

Nota: `adr list` mostra status `unknown` para todos — `parseADRMeta` não reconheceu o frontmatter
mínimo da fixture. Comportamento pré-existente, fora do escopo desta REQ.

---

## 2. Governance score — delta exato

**O que ADR/REQ esperam:** diferença de exatamente 20 pontos entre ausência e presença de ADRs.
"Não 40" — se fosse 40, outra categoria foi tocada.

**Medição:**

```
Fixture sem ADRs (dir vazio):    Governance score: 40/100
Fixture com 4 ADRs em subpastas: Governance score: 60/100
Delta: 20 pontos.
```

**Veredito:** CORRETO. Delta = 20. Nenhuma outra categoria foi alterada.

---

## 3. Dedup D4 — direção aninhada

**O que ADR/REQ esperam:** `adr_dirs: [zeus, zeus/done]` com N ADRs reais reporta N, não 2N.

**Fixture:** 3 ADRs todos em `docs/adr/zeus/done/`. `adr_dirs: [docs/adr/zeus, docs/adr/zeus/done]`.

```
$ trackfw context
## ADRs (3)          ← dedup correto, não 6
$ trackfw status
ADRs  3              ← dedup correto
```

**Veredito:** CORRETO. A contagem é N=3.

**Achado — warnings duplicados (F1, ver Seção 9):**

A mesma execução exibiu 6 warnings (2×3) em vez de 3. Detalhe na Seção 9.

---

## 4. Dedup D4 — direção oposta

**O que ADR/REQ esperam:** dois ADRs com mesmo basename em diretórios distintos e não-aninhados
contam como 2, não 1.

**Fixture:** `docs/adr/zeus/done/ADR-2026-09-01-foo.md` e `docs/adr/athena/done/ADR-2026-09-01-foo.md`.
`adr_dirs: [docs/adr/zeus, docs/adr/athena]`.

```
$ trackfw context
## ADRs (2)
$ trackfw status
ADRs  2
```

**Veredito:** CORRETO. Dedup por caminho absoluto, não por basename.

---

## 5. Layouts adversariais

| Caso | Configuração | Esperado | Medido | Veredito |
|------|-------------|----------|--------|----------|
| A1 | adr_dir inexistente | 0, sem crash | 0 | CORRETO |
| A2 | adr_dir vazio | 0 | 0 | CORRETO |
| A3 | subdir sem .md | 0 | 0 | CORRETO |
| A4 | NOTAS.md sem prefixo ADR- | contado por HasSuffix (residual declarado), violation | 2 ADRs, violation "no frontmatter block", score 20 | CORRETO conforme ADR |
| A5 | ADR 2 níveis abaixo (done/sub/deep) | 1 | 1 | CORRETO |
| A6 | adr_dir é symlink de diretório | não especificado | 0 — WalkDir não segue dir symlinks | INFORMACIONAL (F2) |
| A7 | espaço no nome do dir | 1 | 1 | CORRETO |
| A8 | arquivo .md via symlink dentro de done/ | 1 | 1 | CORRETO |
| A9 | barra final em adr_dirs | 1 | 1 | CORRETO |
| A10 | caminho relativo "./" | 1 | 1 | CORRETO |

**A4:** o score 20 (não 60) é porque a violation penaliza o score — comportamento correto e
esperado pelo gate de validate.

---

## 6. Diretório global via HOME sintético

**Proibição respeitada:** HOME aponta para `$SCRATCHPAD/fake-home-a11`. Zero escrita em
`~/.trackfw/` real.

**Fixture:** ADR em `$FAKE_HOME/.trackfw/adr/done/ADR-2026-09-01-global.md`.
`adr_dirs: [docs/adr/local, ~/.trackfw/adr]`.

```
$ HOME=$FAKE_HOME trackfw context
## ADRs (2)
- ADR-2026-09-01-local.md [Accepted]
- ADR-2026-09-01-global.md [Accepted]

$ HOME=$FAKE_HOME trackfw status
ADRs  2
```

**Veredito:** CORRETO. `~` expandido. ADR em subpasta de `~/.trackfw/adr` encontrado.

---

## 7. `NewADR` (adr new) como potencial 10º sítio

Wave 0 enumerou 9 sítios. `NewADR` — chamada por `trackfw adr new` — não está na lista.
Durante a medição observei `adr new "teste"` criar um arquivo novo com twin existente em `done/`.
Investiguei para determinar se `NewADR` enumera ADRs existentes.

**Medição:**

```bash
$ sed -n '/^func NewADR(/,/^}/p' internal/generators/adr.go | grep -nE 'ReadDir|Glob|WalkDir'
(saída vazia)
```

`NewADR` não contém ReadDir, Glob nem WalkDir — é um caminho de escrita pura. Não verifica se
existe ADR com mesmo slug. Por isso cria o arquivo mesmo com twin em subpasta.

**Veredito para o Wave 0:** a lista de sítios do Wave 0 está correta para enumeradores. `NewADR`
não é um enumerador — é um gerador sem verificação de idempotência. A ADR de ponto único governa
*leitura* de ADRs, não *criação*. O comportamento de `adr new` sempre criando é by design (o
comando `adr new` é para criar deliberadamente, não para dedup).

Observação: isso implica que `trackfw adr new` pode criar um duplicado de nomes quando um ADR
com slug similar existe. Isso é comportamento esperado de um criador deliberado, diferente de
`req new` (que cria draft e verifica idempotência). Documentado, não é defeito desta REQ.

---

## 8. Veredito do ponto único (D3)

**O que ADR/REQ esperam:** todos os sítios que enumeram ADRs passam pelo ponto único.

**Verificação por código (consultado após medições de comportamento):**

- `context.go:42`: usa `validator.ResolveADRFiles(cfg)` ✓
- `validator.go:1418`: usa `len(ResolveADRFiles(cfg))` ✓
- `adr.go:192`: `ListADRs` usa `validator.WalkADRFilePaths(dir)` ✓
- `adr.go:319`: `NewADRDraft` usa `validator.WalkADRFilePaths(adrDir)` ✓

**Gate `check-adr-enumeration-single-point.sh` — saída literal:**

```
--- Gate: adr-enumeration-single-point ---
Files examined: 110
PASS: no ADR enumeration violations
```

Self-test (4 arms): todos passam — arm1 (Pattern A), arm2 (correto), arm3 (comentário em ambas
as direções), arm4 (Pattern B). Saída examinada linha a linha.

**Achado de segurança do gate — F3 (ver Seção 9):**

Construí dois padrões de evasão que o gate não detecta:

**Evasão 1** — variável intermediária:
```go
dirs := cfg.ADRDirs
for _, d := range dirs {
    entries, _ := os.ReadDir(d)  // Pattern A': d não é de `range cfg.ADRDirs`
    _ = entries
}
```
Resultado: gate PASS (deveria FAIL). Pattern A' exige `for _, VAR := range.*ADRDirs`
literalmente na mesma linha — `range dirs` não casa.

**Evasão 2** — indireção através de helper:
```go
func readHelper(dir string) { entries, _ := os.ReadDir(dir); ... } // `dir` sem "adr"
for _, adrDir := range cfg.ADRDirs { readHelper(adrDir) }
```
Resultado: gate PASS (deveria FAIL). Pattern A' cobre o loop mas o `os.ReadDir` está em
outro escopo (helper). Pattern A não casa porque o arg da helper não tem "adr".

Ambos os evasores foram medidos com `ADR_ENUM_SCAN_DIR=` e o gate retornou RC=0.

O gate passa o AC positivo ("reprova quando enumerador direto é introduzido") mas falha o AC
negativo na sua forma mais ampla: um implementador que conhece as heurísticas do gate pode
introduzir root-only enumeration sem ser detectado. O gate documenta explicitamente essa
limitação no cabeçalho ("Pattern A only catches ADR-named variables"), mas o AC da REQ diz
"gate que impede a reintrodução" sem restringir ao caso nominal.

---

## 9. Achados consolidados

### Corretos
- Cenário do #450: CORRETO
- Score delta = 20: CORRETO
- Dedup aninhado e oposto: CORRETO (contagem)
- Layouts adversariais A1-A5, A7-A10: CORRETO
- Global dir via HOME sintético: CORRETO
- Ponto único (110 arquivos, 0 violações): CORRETO no caso nominal

### S7 — NewADRDraft dedup

**Verificação:** interna ao módulo Go (`internal/generators`), impossível via import externo.
Medida por: (a) inspeção de código: `NewADRDraft` usa `validator.WalkADRFilePaths(adrDir)` com
`filepath.Match("ADR-*-"+slug+".md")` antes de criar; (b) execução do teste entregue
`TestNewADRDraft_NaoCriaDuplicadoSubpasta` via `go test`, que produziu a saída literal
`skipped ADR-2026-09-01-minha-decisao.md (already exists)` — mensagem gerada pela implementação,
não pelo harness.

**Limitação:** o setup do cenário foi feito pelo executor, não por mim. Não é medição
completamente independente. A inspeção de código confirma a correção. Não encontrei motivo para
desconfiar do resultado, mas registro a limitação.

---

### F1 — Warnings duplicados com adr_dirs aninhadas (mesmo mecanismo que D4)

**Superfície:** `validateFrontmatterPresence` e `adr_orphan` iteram `cfg.ADRDirs` independentemente
sem dedup, gerando warnings duplicados quando dirs são aninhadas.

**Medido:** com `adr_dirs: [zeus, zeus/done]`, 3 ADRs únicos → 6 warnings na saída de `context`.

**Saída literal:**
```
## Warnings (6)
- adr "ADR-2026-09-01-teste.md" is not referenced by any REQ
- adr "ADR-2026-09-02-teste-b.md" is not referenced by any REQ
- adr "ADR-2026-09-03-teste-c.md" is not referenced by any REQ
- adr "ADR-2026-09-01-teste.md" is not referenced by any REQ  ← duplicado
- adr "ADR-2026-09-02-teste-b.md" is not referenced by any REQ
- adr "ADR-2026-09-03-teste-c.md" is not referenced by any REQ
```

**Classificação — Regra Dura:**
O mecanismo é idêntico ao D4 que foi puxado para esta REQ: iterar `adr_dirs` sem dedup por
caminho absoluto. O arquiteto decidiu trazer D4 para cá sob exatamente esse argumento. Pela
Regra Dura ("mesma causa, mesma REQ"), este sítio pertence à mesma REQ — não a uma issue nova.

**Gravidade:** os warnings duplicados são UX, não segurança. O agente que lê `context` vê
avisos redundantes, mas a contagem de ADRs está correta. Não oculta ADRs.

**Ação esperada do arquiteto:** decidir se entra como novo ML nesta REQ (conforme Regra Dura)
ou se recebe classificação explícita de por que a causa difere. O ônus está em quem quer dividir.

---

### F2 — Dir symlink em adr_dirs não encontra ADRs (informacional)

`filepath.WalkDir` não segue symlinks de diretório. Um `adr_dirs` apontando para um symlink
de dir resulta em 0 ADRs encontrados. Não é requisito da ADR/REQ. Documentado para
rastreabilidade. Severidade: baixa (configuração incomum).

---

### F3 — Gate tem dois caminhos de evasão conhecidos (achado de segurança)

**Superfície:** `scripts/check-adr-enumeration-single-point.sh`

**Evasão 1:** variável intermediária (`dirs := cfg.ADRDirs; for _, d := range dirs`) — Pattern A'
não casa com `range dirs`, apenas com `range.*ADRDirs` literal.

**Evasão 2:** helper function — o ReadDir está em escopo diferente do loop sobre `ADRDirs`.

**Impacto:** um implementador pode introduzir root-only enumeration usando um desses padrões
e o gate não detectará. O gate documenta essa limitação, mas o AC da REQ ("gate que impede a
reintrodução") não a restringe explicitamente. O AC está overstated em relação à capacidade
real do gate.

**Ação esperada:** ou refinar o gate para cobrir os padrões de evasão, ou redefinir o AC para
refletir o que o gate realmente garante ("reprova na introdução de os.ReadDir/filepath.Glob
diretamente em loop sobre cfg.ADRDirs"). Não bloqueia o ML se o AC for ajustado — o gate cobre
os casos diretos e não triviais que um implementador distraído (não malicioso) introduziria.

---

## Veredito final

**APROVA COM RESSALVAS.**

O cenário do #450 está correto. Os três sítios de classe (iii) (S1, S6, S7) estão funcionando.
O dedup D4 está correto nas duas direções. Nenhum enumerador root-only residual no caso nominal.

**Ressalvas que o arquiteto deve tratar antes do merge:**
1. **F1** — warnings duplicados com adr_dirs aninhadas têm o mesmo mecanismo que D4; Regra Dura
   coloca na mesma REQ. Arquiteto decide se entra como ML ou documenta a distinção de causa.
2. **F3** — o AC da REQ sobre o gate está overstated. O gate não "impede" evasão sofisticada;
   "detecta o padrão direto e nominal". AC deve ser corrigido ou o gate deve cobrir os padrões
   de evasão medidos.

**Ressalvas informacionais (não bloqueiam):**
- F2: dir symlinks não seguidos — comportamento stdlib, não requisito.
- S7: verificado por inspeção + teste entregue (não medição black-box pura) — limitação documentada.

**git diff trackfw.yaml:** vazio.
