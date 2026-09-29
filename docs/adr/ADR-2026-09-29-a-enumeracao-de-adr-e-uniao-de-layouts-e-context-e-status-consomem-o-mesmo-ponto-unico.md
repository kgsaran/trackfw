---
status: Accepted
date: 2026-09-29
author: "trackfw_architect"
---

# ADR: a enumeração de ADR é união de layouts, e `context` e `status` consomem o mesmo ponto único

> Date: 2026-09-29 | Status: Accepted

## Context

No mesmo repositório e na mesma versão, dois comandos discordam sobre o mesmo dado. Medido em
fixture pelo arquiteto em 2026-09-29, com `adr_dirs: [docs/adr/zeus]` e os ADRs em subpastas de
estado:

```
ADRs no disco:  4
trackfw status:   ADRs 4        ← correto
trackfw context:  ## ADRs (0)   ← zero
```

**Causa localizada:** `internal/generators/context.go:39` faz `os.ReadDir(adrDir)` — lê **apenas a
raiz** de cada entrada de `adr_dirs`, sem descer nas subpastas de estado.

### 🔴 A contradição está dentro da mesma saída

```
## ADRs (0)
- (none)
...
## Warnings (6)
- adr "ADR-2026-09-01-teste.md" is not referenced by any REQ
```

O mesmo comando, na mesma execução, **declara zero ADRs e nomeia um ADR**. O validator enxerga as
subpastas; o enumerador do `context` não. Isso não é ambiguidade de layout — é **duas implementações
da mesma pergunta**, e uma está errada.

### Por que pesa mais que um número errado

O `context` é o comando que a própria documentação instalada manda o agente rodar **primeiro**
(*"always run first"*). Um agente que obedece recebe `ADRs: (none)` e conclui que o projeto **não tem
decisões arquiteturais registradas** — quando tem 145. 🔴 **É a pior forma de dado errado: a que
induz confiança na ausência.** Um agente assim não viola uma ADR: ele nem sabe que ela existe.

### O score também, e isso foi medido — não inferido

O reportante marcou como hipótese não confirmada. Confirmado em `context.go:121`:

```go
if len(adrs) > 0 { score += 20 }
```

`adrs` vem do enumerador quebrado. O defeito custa **exatamente 20 pontos** de `Governance score`.

### O precedente que ficou pela metade

`context.go:56` traz o comentário: *"REQs — pelo PONTO ÚNICO de leitura (ADR-2026-09-03, D3/D4)"*.
As REQs **já** foram migradas para ponto único naquela ADR. **Os ADRs ficaram para trás.** É o achado
A1 da auditoria externa de 2026-09-05 outra vez: ADR de ponto único marcada satisfeita com sítio
sobrando.

## Decision

**D1 — A enumeração de ADR é união de layouts.** O resolvedor aceita, concatenando:

```
adr_dir/*.md                  ← raiz (layout plano, legado)
adr_dir/<estado>/*.md         ← subpastas de estado (backlog, wip, done, …)
```

**D2 — ADR TEM dimensão de estado; REQ não.** É a diferença explícita em relação à
**ADR-2026-09-03**, cujo invariante é *"REQ não tem dimensão de estado"*. Aquela decisão **não se
estende por analogia** a ADRs, e é por isso que esta existe em vez de uma emenda. Um ADR vive em
`done/` ou `wip/`, e o estado é informação legítima — não ruído de layout.

**D2-bis — A população de sítios é MAIOR do que esta ADR supôs.** A primeira versão nomeava **2**
sítios (`context` e `status`). A Wave 0 mediu **9 sítios** que leem ou contam ADR, dos quais **3 são
classe (iii) — implementação própria errada**:

| sítio | mecanismo | efeito |
|---|---|---|
| `context.go:38` `GetContext` | `os.ReadDir` raiz | `ADRs (0)` + score −20 |
| `adr.go:189` `ListADRs` | `filepath.Glob` raiz | 🔴 `trackfw adr list` responde **"No ADRs found"** |
| `adr.go:316` `NewADRDraft` | `filepath.Glob` raiz | 🔴 cria **rascunho duplicado** quando o twin está em subpasta |

**Mesma causa, mesma REQ** (Regra Dura): os três entram no mesmo ML. `adr list` silenciar **todos**
os ADRs é, para o usuário, tão grave quanto o `context` — e seria descoberto depois, numa fila.

**D4 — O resolvedor de nível-`cfg` DEDUPLICA por caminho absoluto.** Medido na Wave 0 e confirmado
pelo arquiteto: com `adr_dirs: [docs/adr/zeus, docs/adr/zeus/done]` — entradas **aninhadas** —, o
`status` reporta **7** onde o correto é **4**. O defeito **já existe hoje** e é independente das
subpastas.

🔴 **Por que isto entra aqui e não vira resíduo:** sem dedup, a correção faria `context` e `status`
passarem a **concordar no número errado** — e o AC *"delta de exatamente 20 pontos"* **passaria assim
mesmo**, porque num repositório sem `adr_dirs` aninhadas ele não discrimina. Um AC que não distingue
o certo do errado não é AC.

**D3 — `context` e `status` consomem o MESMO ponto único.** Não é "corrigir o `context` para
concordar com o `status`": é os dois passarem a perguntar ao mesmo resolvedor. Duas implementações
que concordam hoje divergem amanhã, e a divergência entre comandos é o sintoma mais confiável de que
existem duas.

## Consequences

**Positivas**
- O agente que roda `context` primeiro passa a ver as decisões que deve respeitar.
- O `Governance score` deixa de ser deflacionado em 20 pontos por um defeito de enumeração.
- A contradição interna da saída (`ADRs (0)` + warning nomeando ADR) desaparece por construção.

**Negativas e aceitas**
- 🔴 **RETRATAÇÃO (2026-09-29, refutada pela Wave 0).** A primeira versão desta ADR afirmava que
  *"o filtro por prefixo `ADR-` já é o usado pelo `status`"*. **É FALSO, e eu não havia medido.**
  Medido: `validator.go:3017` e `context.go:44` usam ambos `strings.HasSuffix(path, ".md")` —
  **sem filtro de prefixo nenhum**.

  O risco de deixar a frase de pé era concreto: o implementador leria a ADR, acrescentaria filtro por
  prefixo *"para preservar o comportamento"*, e mudaria **silenciosamente** a contagem de todo
  repositório com `.md` não-ADR em `adr_dirs`. A regressão seria invisível **aqui** — neste
  repositório não há `.md` fora do padrão na raiz de `docs/adr`.

  **Decidido:** o resolvedor mantém `HasSuffix(".md")`, **sem** prefixo — idêntico ao comportamento
  atual dos dois comandos. Esta REQ corrige **alcance da varredura**, não **critério de
  identificação**; mudar os dois ao mesmo tempo tornaria impossível atribuir qualquer variação de
  contagem a uma das causas.

- 🔴 **Resíduo declarado, medido e NÃO corrigido aqui:** com `HasSuffix(".md")`, um `NOTAS.md` na
  raiz de `adr_dir` **é contado como ADR**. Medido: 4 ADRs reais → `status` reporta **8** com um
  `NOTAS.md` presente e `adr_dirs` aninhadas. É **causa distinta** (critério de identificação), fica
  fora desta REQ por decisão, e é **candidato a issue própria**.

- A contagem de ADRs de alguns repositórios **vai subir** após o upgrade. Isso é a correção
  aparecendo, não regressão — e deve constar nas release notes, porque número que muda sozinho entre
  versões gera issue.
- A contagem de ADRs de alguns repositórios **vai subir** após o upgrade. Isso é a correção
  aparecendo, não regressão — e deve constar nas release notes, porque um número que muda sozinho
  entre versões é exatamente o tipo de coisa que gera issue.

## Alternatives Considered

**Corrigir só o `context` para descer nas subpastas** — rejeitado. Fecha o sintoma medido e deixa
**duas** implementações da mesma pergunta, que é a causa. A próxima divergência nasce no próximo
comando que enumerar ADR.

**Emendar a ADR-2026-09-03 estendendo D3/D4 a ADRs** — rejeitado por **D2**: o invariante daquela
ADR é que REQ **não** tem dimensão de estado, e ADR tem. Estendê-la por analogia escreveria uma
decisão falsa no documento que governa outra família de artefato.

**Fazer `context` chamar `status` internamente** — rejeitado. Acopla dois comandos pela saída em vez
de pelo dado, e a saída do `status` é formatada para humano.
