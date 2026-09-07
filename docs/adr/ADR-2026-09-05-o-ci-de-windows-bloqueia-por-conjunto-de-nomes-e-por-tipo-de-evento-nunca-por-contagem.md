---
status: Proposed
date: 2026-09-05
author: ""
---

## Contexto

Três medições independentes, todas de 2026-09-05, dizem a mesma coisa por caminhos diferentes.

**1. A contagem esconde regressão** (`#275`, medido pelo consumidor externo em Windows 11 real):

```
#269 + #270    51 → 33    corrigidas 20 · NOVAS 0
#271           33 → 32    corrigidas  2 · NOVAS 1   ←
#272           32 → 12    corrigidas 20 · NOVAS 0
```

*"Uma queda de 1 pode conter 2 correções e 1 regressão."* Nós **detectamos** aquele vermelho novo —
diffando conjuntos de nomes — mas dependeu de o arquiteto **lembrar de fazer isso**. O CI não
obrigava.

**2. A contagem é frágil no instrumento.** O arquiteto reportou **Go = 14 onde havia 46** porque o
`grep` não casava o prefixo por linha do `gh run view --log` — sem erro, exit 0, número plausível
(`vault/notes/contagem-de-falhas-de-windows-do-go-medida-por-padrao-frouxo-2026-09-04.md`).

**3. E a prova final veio de nós mesmos:** o PR #281 introduziu um teste que **reprova no Windows**
(`TestDetectNameCollision_ENOTDIRIsReportedNotSwallowed`), mergeado sem que nada bloqueasse. O
`continue-on-error: true` do job existe justamente porque a dívida não é zero — e, enquanto existe,
**nenhuma regressão bloqueia**.

### E o discriminante que propusemos para o `#274` também não discrimina

Medido:

```
teste que roda e reprova   → # tests 1 · pass 0 · fail 1
import quebrado            → # tests 1 · pass 0 · fail 1
```

Comentamos publicamente que `pass 0 / fail 1` distingue "a suíte não carregou". **Não distingue.**

## Decisão

### D1 — O gate de Windows bloqueia por **conjunto de nomes**, não por contagem

Lista versionada de vermelhos conhecidos, por nome de teste. O job **falha** se aparecer um nome fora
da lista; **avisa** quando um nome da lista deixa de falhar, pedindo remoção.

Isso remove a única razão do `continue-on-error`: passa a bloquear **regressão** sem exigir que a
dívida chegue a zero primeiro. É o mesmo princípio do baseline/ratchet que o produto já aplica a
violações de governança — aplicado ao CI.

### D2 — A lista nasce de um run do CI, nunca de uma máquina

🔴 O próprio autor do `#275` declara que o Windows dele **não é o runner** (sem WSL, sem `bash` no
`%PATH%` nativo). A lista dele sustenta a **necessidade** do ratchet, **não** o conteúdo.

### D3 — Suíte que não executa é falha, e é falha de CLASSE PRÓPRIA

Um estado sem nomes não é coberto por D1 — e é o pior modo de falha, porque a contagem **cai** e a
queda parece progresso.

O discriminante é o **tipo do evento** (erro de carregamento vs. asserção), nunca a contagem.
🔴 Quem implementar **precisa medir o discriminante nos dois cenários antes de escrevê-lo** — foi
pular esse passo que produziu a nossa afirmação errada no `#274`.

### D4 — Remoção de nome da lista exige justificativa explícita

Nome que some pode ter sido **corrigido**, **renomeado** num refactor, ou **deixado de executar**. O
ratchet não distingue os três sozinho. Toda alteração da lista declara qual é o caso.

Sem isto a lista vira cemitério — que é a objeção óbvia ao ratchet, e a única forma de ele fracassar
em silêncio.

### D5 — 🔴 Nomes instáveis são um limite conhecido, não um detalhe

Testes parametrizados e subtestes têm nome que muda com a tabela. Quem implementar **declara** como
trata isso (nome do teste de topo, normalização, ou exclusão explícita) — e a decisão fica escrita,
não implícita no código.

## Consequências

**O `continue-on-error: true` do `windows-full-suites` sai** — mas só depois de D1, D3 e D4 estarem
de pé. Removê-lo antes tornaria a `main` imergível com a dívida atual.

**A dívida deixa de ser um número e passa a ser uma lista.** É mais verboso e é o ponto: um número
não diz *o que* piorou.

**Não cobre o que não roda em CI.** O runner não é o Windows de um consumidor: `bash` no `%PATH%`,
WSL ausente, perfil de rede. O ratchet protege contra regressão **no runner**, e continuar recebendo
relato externo segue sendo necessário.

## Verificação exigida de quem implementar

- Falsificação **nas duas direções**: teste novo que falha só em Windows, sem tocar a lista → o job
  **reprova nomeando o teste**. E: corrigir um teste da lista → **avisa**, não reprova; removê-lo da
  lista → verde.
- 🔴 **Controle de não-vacuidade:** com a lista vazia e a dívida atual, o job **tem** de reprovar. Um
  ratchet que passa sobre qualquer entrada não é ratchet.
- D3 medido nos dois cenários **antes** de escrever o discriminante, com a saída real de cada um.
- O caso do **rename** exercitado explicitamente: renomear um teste da lista sem corrigi-lo **não**
  pode virar verde silencioso.


---

## Adendo — 2026-09-06: qual campo é o "tipo do evento" (D3), medido

🔴 **A D3 dizia "tipo do evento" — e o nome natural do conceito aponta para o campo ERRADO.**

Medido pelo consumidor externo (issue #274), Node 22 / Windows 11, reporter `tap`, cinco modos:

```
caso                      exitCode  ERR_ASSERTION  failureType       subject do "not ok"
A  assert falso              0           1         testCodeFailure   nome do teste
B  modulo ausente            1           0         testCodeFailure   CAMINHO DO ARQUIVO
C  erro de sintaxe           1           0         testCodeFailure   CAMINHO DO ARQUIVO
D  throw generico            0           0         testCodeFailure   nome do teste
E  dois testes, 1 reprova    0           1         testCodeFailure   nome do teste
```

**`failureType` é `testCodeFailure` nos cinco.** Quem implementasse "discriminar por tipo de evento"
pegando esse campo entregaria um discriminante que não discrimina — pela terceira vez nesta issue.

### D3-bis — o discriminante é `exitCode` PRESENTE, não a ausência de `ERR_ASSERTION`

Dois sinais funcionam e concordam entre si:

1. **`exitCode:` presente** no bloco do `not ok` → o processo do arquivo morreu (B, C).
2. **O subject do `not ok` é o caminho do arquivo**, não o nome do teste.

🔴 **`ERR_ASSERTION` sozinho NÃO serve:** o caso **D** (`throw` genérico dentro do teste) não tem
`ERR_ASSERTION` **nem** `exitCode` — classificá-lo pela ausência de `ERR_ASSERTION` o mandaria para o
balde de "não carregou". **O robusto é a presença de `exitCode`, não a ausência do outro.**

### Limites declarados pelo autor da medição

- **Node 22, Windows 11.** Estabilidade do formato TAP entre versões do Node **não verificada** — e o
  remédio passa a depender de um campo do reporter, não de um número no sumário.
- **`pytest` e `go test` não medidos com o mesmo rigor.** O Go discrimina por `[build failed]`
  (confirmado); o `pytest` separa `errors` de `failures` no sumário, mas o caso de **arquivo com um
  único teste reprovando** não foi testado.
- 🔴 **`tests == 0` continua descoberto** — nenhum dos dois sinais o cobre, porque não há `not ok`
  para inspecionar. **Continua sendo o pior modo de falha**, e a AC5 da REQ segue exigindo medição
  própria para ele.

### O que isto muda no ML-1A da REQ

O ML **não** parte de "escolher um campo". Parte de **reproduzir esta tabela nos 3 runtimes** — e a
medição do Node já está feita e é reaproveitável. O que falta medir é `pytest`, `go test` e o caso
`tests == 0`.
