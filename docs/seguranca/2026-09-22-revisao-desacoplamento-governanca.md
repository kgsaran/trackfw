# Revisão de Segurança — Barreira Final da REQ #396

> Role: hades-tf | Data: 2026-09-22
> Branch: `fix/teste-e-gate-leem-a-arvore-de-governanca`

REQ: `docs/req/REQ-2026-09-22-teste-e-gate-leem-a-arvore-de-governanca-do-repositorio-onde-rodam-e-o-consumidor-nao-consegue-rodar-a-suite.md`

---

## Veredito

**APROVADO COM RESSALVAS**

Os 3 sítios (a) foram corrigidos. O gate novo detecta a regressão central (pin reintroduzido em
`parity-rest`). Três residuais persistem — todos de baixo impacto prático, dois deles declaráveis
como design deliberado. O único com potencial de mordida real (Sítio 3 removido do CI sem ser
detectado) é residual estrutural da arquitetura Makefile↔CI e está detalhado abaixo com a
evidência de por que é aceitável no presente.

---

## Pergunta 1 — O desacoplamento virou desligamento em algum lugar?

### Sítio 1: `roadmapdoc_test.go` — `t.Skip` quando `docs/roadmaps/done` não existe

**O que a correção faz:** substitui `t.Fatalf` por `t.Skipf` quando `errors.Is(err, fs.ErrNotExist)`.
Para qualquer outro erro de I/O, faz `t.Logf` + `return` (sem fail).

**Verificação do braço upstream (não virou skip):**

```
$ ls /Users/kgsaran/.../trackfw/docs/roadmaps/done/ | wc -l
194
```

O diretório existe com 194 arquivos no upstream. O `ReadDir` sucede. O teste mede e loga. O braço
de skip não é alcançado.

**Caminhos em que o upstream cai no skip:**

Testei duas configurações:

```
# done/ ausente (sparse-checkout excluindo o subdiretório)
$ cd fake-sparse/internal/roadmapdoc
$ ./roadmapdoc.test -test.run TestCorpusMeasurement_ReportOnly -test.v
--- SKIP: TestCorpusMeasurement_ReportOnly (0.00s)
RC=0
```

→ Visível no CI log como `--- SKIP`. Não silencioso.

**Gap encontrado: `done/` existindo mas vazia.**

```
# done/ existe mas vazia (sparse-checkout que cria o dir sem os arquivos)
$ mkdir -p fake-upstream/docs/roadmaps/done
$ cd fake-upstream/internal/roadmapdoc
$ ./roadmapdoc.test -test.run TestCorpusMeasurement_ReportOnly -test.v
--- PASS: TestCorpusMeasurement_ReportOnly (0.00s)
    roadmapdoc_test.go:299: done/ corpus: total=0, unfinished(...)=0, unfinished(...)=0
RC=0
```

→ O teste **passa com `total=0`**, sem skip, sem fail. A mensagem de log existe, mas é fácil de
ignorar em CI. Qualquer sparse-checkout que materialize o diretório mas não os arquivos produz
essa saída silenciosa.

**Avaliação:** O CI do upstream usa checkout completo com `actions/checkout@v7` sem `sparse-checkout`.
Os 194 arquivos existem e são commitados. Este cenário não ocorre no CI atual. É um residual
documentado (ver Seção Residual).

### Sítio 2: `validator_test.go` — `TestExtractRefPath_CorpusBacktickREF`

**Quando silencia sem falhar:**

O teste usa `t.Logf + continue` para cada arquivo que falhar no `ReadFile`, e `t.Skip` se `found == 0`.
Portanto: se todos os 3 arquivos retornarem erro de I/O (não apenas ENOENT), o teste SKIPS.

Testei o caso de arquivo presente com `chmod 000`:

```
$ chmod 000 fake-perm/docs/req/REQ-2026-07-27-roadmap-move-sincroniza-o-status-do-artefato.md
$ ./validator.test -test.run TestExtractRefPath_CorpusBacktickREF -test.v
    validator_test.go:2323: corpus: "docs/req/REQ-2026-07-27-roadmap-move-sincroniza-o-status-do-artefato.md" ausente — contexto de consumidor, ignorado
--- SKIP: TestExtractRefPath_CorpusBacktickREF (0.00s)
RC=0
```

→ O arquivo existe no disco, ReadFile retorna `EACCES`, o teste loga "ausente" e skips. A mensagem
é tecnicamente imprecisa (não está ausente, está inacessível), mas o comportamento é aceitável para
o uso declarado: o teste existe para garantir que arquivos de corpus acessíveis continuam satisfazendo
a propriedade. Um arquivo inacessível não pode ser verificado.

**Quando reprova (confirmado):**

```
# Arquivo presente mas conteúdo sem backtick ADR ref:
$ ./validator.test -test.run TestExtractRefPath_CorpusBacktickREF -test.v
--- FAIL: TestExtractRefPath_CorpusBacktickREF (0.00s)
    validator_test.go:2331: CORPUS REGRESSION: extractRefPath não resolveu o ADR de "..."
RC=1
```

→ O caminho de regressão está protegido.

**O `t.Logf + continue` é o design correto** para um controle de corpus em contexto misto
(upstream tem arquivos, consumidor não). A alternativa (t.Fatalf em EACCES) quebraria o
consumidor em umask restritivo. O skip declarado é visível no CI log. Não é uma desconexão.

### Sítio 3: `check-roadmap-barrier-contract.sh` — tripwire de disco

**Verificação: `make quality` não ativa a tripwire.**

```
$ make -n quality 2>/dev/null | grep "check-roadmap-barrier-contract"
GO_BIN=bin/trackfw HASH_CMD_BIN="sha256sum" scripts/check-roadmap-barrier-contract.sh
```

→ `TRACKFW_SELF_GOVERNED` não aparece na linha. A tripwire não é ativada no caminho do consumidor.

**Verificação: `make self-governance` ativa a tripwire.**

```
$ make -n self-governance 2>/dev/null | grep "check-roadmap-barrier-contract"
TRACKFW_SELF_GOVERNED=1 GO_BIN=bin/trackfw HASH_CMD_BIN="sha256sum" scripts/check-roadmap-barrier-contract.sh
```

→ O pin está presente.

**Verificação: CI chama `make self-governance` num step com nome explícito:**

```yaml
# .github/workflows/quality.yml:834-835
- name: ML-1B-bis — self-governance (tripwire do upstream)
  run: make self-governance
```

→ O passo existe e tem nome descritivo. Aparece no job `parity-other-gates`, que está em `needs`
da agregação `parity`.

**Gap: remoção do step não é detectada pelo gate.**

Se o step `ML-1B-bis — self-governance` for removido do CI yaml mas o alvo `self-governance`
permanecer no Makefile, `check-parity-call-site-pins.sh` continua passando (ele verifica o
Makefile, não o CI yaml). A tripwire simplesmente não executa.

Nenhum gate verifica que `self-governance` é invocado do CI:

```
$ grep -rn "self-governance" scripts/*.sh | grep -v "^.*:#\|MAKEOF\|comment"
(apenas referências em check-parity-call-site-pins.sh — verificam o Makefile, não o CI)
```

Avaliação: a remoção do step exige edição de `.github/workflows/quality.yml`, que passa por PR
e revisão. O único vetor silencioso seria um PR que remove o step com mensagem de commit
enganosa. Aceitável como residual estrutural (ver Seção Residual).

---

## Pergunta 2 — O gate novo pode ser contornado?

O gate `check-parity-call-site-pins.sh` usa `make -n quality` como discriminante e verifica
que `TRACKFW_SELF_GOVERNED=` NÃO aparece em nenhuma linha de recipe que invoca o script
consumidor na cadeia `quality`.

**Bypass por `env:` no CI workflow (confirmado):**

Se `TRACKFW_SELF_GOVERNED: 1` for adicionado à seção `env:` do job `parity-other-gates`,
a variável é herdada pelo runner quando `make parity-rest` executa. O script verifica
`${TRACKFW_SELF_GOVERNED:-0}` — ativaria a tripwire para o consumidor.

O gate, porém, verifica apenas a saída de `make -n quality`:

```
$ TRACKFW_SELF_GOVERNED=1 make -n quality 2>/dev/null | grep "check-roadmap-barrier-contract"
GO_BIN=bin/trackfw HASH_CMD_BIN="sha256sum" scripts/check-roadmap-barrier-contract.sh

$ TRACKFW_SELF_GOVERNED=1 make -n quality 2>/dev/null | grep -E "TRACKFW_SELF_GOVERNED="
(vazio — RC=1)
```

→ `make -n` não mostra variáveis herdadas do ambiente na linha de recipe. O gate passa. A
tripwire estaria ativa para o consumidor. **Bypass confirmado.**

Condição necessária: editar `.github/workflows/quality.yml` adicionando `TRACKFW_SELF_GOVERNED: 1`
na seção `env:`. Essa edição é visível em PR. Não há caminho silencioso.

**Bypass por `export VAR = 1` no Makefile (confirmado):**

```
# Makefile com: export TRACKFW_SELF_GOVERNED = 1 no topo
$ make -n quality 2>/dev/null | grep "check-roadmap-barrier-contract"
GO_BIN=bin/trackfw scripts/check-roadmap-barrier-contract.sh

$ make -n quality 2>/dev/null | grep -E "(^|[[:space:]])TRACKFW_SELF_GOVERNED="
(vazio — gate não detectaria)
```

→ A variável seria exportada para o ambiente do script sem aparecer na linha de recipe do
`make -n` output. Gate passa. Tripwire ativa. **Bypass confirmado.**

Condição necessária: editar o `Makefile` — visível em PR.

**O que o gate captura corretamente (prova de não regressão):**

Pin reintroduzido inline em `parity-rest`:

```
$ bash regression-test/scripts/check-parity-call-site-pins.sh regression-test/ 2>&1 | grep "forbidden-in-quality"
FAIL [call-site-pin/TRACKFW_SELF_GOVERNED/forbidden-in-quality]: make quality alcança ao menos
uma invocação de scripts/check-roadmap-barrier-contract.sh que pina TRACKFW_SELF_GOVERNED= --
o pin pertence exclusivamente ao alvo upstream (self-governance), não ao caminho do consumidor:
TRACKFW_SELF_GOVERNED=1 GO_BIN=bin/trackfw HASH_CMD_BIN="sha256sum" scripts/check-roadmap-barrier-contract.sh
RC=1
```

→ A regressão exata da Wave 1 é detectada.

---

## Pergunta 3 — `TRACKFW_SELF_GOVERNED` como superfície

**Estado atual:**

```
$ grep -rn "TRACKFW_SELF_GOVERNED" .github/workflows/quality.yml
(vazio — não está nos env: de nenhum job ou step)

$ grep -n "TRACKFW_SELF_GOVERNED" Makefile
75:  # ML-1B-bis: a tripwire de disco (TRACKFW_SELF_GOVERNED=1) foi movida...
142: # TRACKFW_SELF_GOVERNED=1: pin verificado...
145: TRACKFW_SELF_GOVERNED=1 GO_BIN=... scripts/check-roadmap-barrier-contract.sh

$ ls .env 2>/dev/null
(arquivo .env não existe)
```

A variável existe em exatamente um lugar: a linha de recipe do alvo `self-governance` no Makefile.

**Quem consegue desligá-la e o que ganha:**

- Remover a linha do recipe → `VARS_PIN_ANY` do gate detecta (zero call sites pinando) → RC=1.
- Remover o alvo `self-governance` inteiro → mesmo: `VARS_PIN_ANY` verifica que o script é chamado
  com o pin em ao menos uma linha → RC=1 na ausência.
- Ambas as remoções são visíveis em PR.

O que se ganha ao desligar: corpus pode divergir do disco (basenames apagados do `docs/roadmaps/`
sem alerta). Impacto: integridade do snapshot do harness de falsificação fica sem verificação viva.

**Quem consegue ativá-la incorretamente e o que isso produz:**

Como documentado em Q2: via `env:` do CI ou `export` no Makefile. Resultado: tripwire ativa para
consumidores que rodam `make parity-rest`, quebrando CI deles. O adversário aqui é o implementador
que "move a lógica de volta" sem perceber as implicações.

**Risco do ambiente do mantenedor:**

Se o mantenedor tem `TRACKFW_SELF_GOVERNED=1` no shell (`~/.zshrc`, `~/.bashrc`), `make parity-rest`
no shell do mantenedor ativa a tripwire. Para o mantenedor isso é correto (o upstream tem os 144
arquivos). Para qualquer script de CI que herda o ambiente do shell (raro com CI cloud, comum com
runners self-hosted), seria um problema. O CI usa GitHub-hosted runners, então essa herança não
ocorre.

---

## Pergunta 4 — A asserção negativa é completa?

**O que a asserção cobre:**

O gate usa `make -n quality` como discriminante. O Makefile declara:
`quality → parity → parity-rest → check-roadmap-barrier-contract.sh`.

```
$ make -n quality 2>/dev/null | grep "check-roadmap-barrier-contract"
GO_BIN=bin/trackfw HASH_CMD_BIN="sha256sum" scripts/check-roadmap-barrier-contract.sh

$ make -n parity 2>/dev/null | grep "check-roadmap-barrier-contract"
GO_BIN=bin/trackfw HASH_CMD_BIN="sha256sum" scripts/check-roadmap-barrier-contract.sh

$ make -n parity-rest 2>/dev/null | grep "check-roadmap-barrier-contract"
GO_BIN=bin/trackfw HASH_CMD_BIN="sha256sum" scripts/check-roadmap-barrier-contract.sh
```

→ `make quality`, `make parity` e `make parity-rest` todos são cobertos pela asserção (porque
`quality` já os alcança, e o gate usa `quality` como raiz).

**O que não é coberto:**

- `bash scripts/check-roadmap-barrier-contract.sh` direto: não passa pelo Makefile. Se alguém
  chama o script diretamente com `TRACKFW_SELF_GOVERNED=1` no ambiente, o gate não vê.
  → Não é invocado pelo CI; é uso de debug/desenvolvimento. Aceitável.

**CI atual invoca `make parity-rest` e `make self-governance` corretamente:**

```yaml
# parity-other-gates job:
- run: make parity-rest         # sem TRACKFW_SELF_GOVERNED
  env:
    TRACKFW_DISABLE_EXTERNAL_COMMANDS: "1"
- name: ML-1B-bis — self-governance (tripwire do upstream)
  run: make self-governance     # sem env: extra
```

→ Separação estrutural correta. O step de `parity-rest` não herda `TRACKFW_SELF_GOVERNED` de
lugar nenhum. O step de `self-governance` seta o pin via Makefile recipe.

**Guarda de vacuidade do gate (confirmada):**

O gate verifica explicitamente que `make -n quality` produz ao menos uma invocação não-comentário
do script consumidor. Sem isso, ausência de pin seria falso-positivo:

```bash
# scripts/check-parity-call-site-pins.sh:231-237
consumer_invocations=$(grep -F "scripts/${base}" "$local_dryrun" | grep -vE '^[[:space:]]*#' || true)
if [[ -z "$consumer_invocations" ]]; then
  fail "call-site-pin/$var/forbidden-in-quality/consumer-invoked" \
    "make -n quality não produziu nenhuma invocação não-comentário..."
```

→ Se o script for removido de `parity-rest`, o gate falha fechado. Correto.

**Gate em produção, RC=0:**

```
$ scripts/check-parity-call-site-pins.sh . 2>&1 | grep -E "TRACKFW_SELF_GOVERNED|check-parity"
OK   [call-site-pin/TRACKFW_SELF_GOVERNED/check-roadmap-barrier-contract.sh]
OK   [call-site-pin/TRACKFW_SELF_GOVERNED/forbidden-in-quality]
check-parity-call-site-pins: 10 verificação(ões) -- 2 pin-all(s) + 1 pin-any(s) + 1 forbidden-in-quality(s) + 3 rastro(s) na lista
```

---

## Residual declarado

**R-A — `done/` existe mas vazia: silêncio com `total=0`**

Mecanismo: sparse-checkout que materializa o diretório sem os arquivos. O teste passa com
`total=0, unfinished=0`. Sem skip, sem fail. A mensagem de log existe mas não chama atenção.

Porque é aceitável: o CI usa `actions/checkout@v7` com checkout completo. Os 194 arquivos são
commitados. O cenário só ocorre com checkout não-padrão proposital. O teste continua sendo
"nunca falha" — que é o contrato declarado do sítio 1.

Nota: o único contra-argumento seria *adicionar um piso mínimo (`if total == 0 && err == nil`)*
como aviso de log mais visível. Não bloqueia o veredito; é sugestão de robustez futura.

**R-B — Bypass via `env:` no CI workflow ou `export` no Makefile**

Mecanismo: documentado em Q2. Requer edição de arquivo rastreado (visible em PR). Não há vetor
silencioso. O gate protege o caso mais provável (regressão inocente via inline pin).

Porque é aceitável: ambos os caminhos de bypass são mudanças intencionais a arquivos commitados,
visíveis em revisão de PR. O adversário aqui é o implementador apressado, não um ator mal-
intencionado com acesso de escrita ao repositório. Se o adversário tem acesso de escrita ao main
sem revisão, o modelo de controle já está comprometido por outra razão.

**R-C — Remoção do step `self-governance` do CI yaml sem detecção automática**

Mecanismo: documentado em Q1 (Sítio 3). O gate verifica que o Makefile contém o pin, não que
o CI invoca o alvo. Se o step for removido, a tripwire fica morta silenciosamente.

Porque é aceitável (com condição): o step tem nome explícito (`ML-1B-bis — self-governance`)
que aponta diretamente para o roadmap. Um PR que remove esse step com mensagem enganosa passaria
pelo gate — mas não passaria por uma revisão de código atenta. A condição de aceitabilidade é
que o PR process inclua revisão do diff do workflow yaml, que é o padrão do projeto.

Se o projeto crescer em complexidade ou usar merge automático, este residual deve ser reavaliado.

**R-D — `TestExtractRefPath_CorpusBacktickREF` com `chmod 000` trata arquivo como ausente**

Mecanismo: documentado em Q1 (Sítio 2). `ReadFile` retorna `EACCES`, o teste loga "ausente" e
continua. Se todos os 3 arquivos falharem por EACCES, o teste skips.

Porque é aceitável: arquivos commitados no git têm permissão padrão (644). O cenário requer
manipulação proposital de permissões fora do git. A mensagem de log ("ausente — contexto de
consumidor") é ligeiramente imprecisa para o caso de EACCES, mas o comportamento (não falhar
por inacessibilidade) está correto para o contrato do teste.

---

## O que eu bloquearia

Não há nenhum item bloqueante. Os 3 sítios (a) foram corrigidos. O gate novo detecta a regressão
primária. Os residuais documentados (R-A a R-D) são aceitáveis nas condições descritas.

Se houvesse capacidade de endereçar agora, priorizaria:

1. **R-A (Sítio 1)**: adicionar um `if total == 0 && os.IsDir-check` com `t.Logf("ATENÇÃO: done/ existe mas vazia — checkout completo?")` mais visível. Uma linha. Não é uma falha de segurança, é robustez de observabilidade.

2. **R-C (Sítio 3)**: um gate que verifica que o yaml do workflow contém `make self-governance`
   (simples grep no CI yaml dentro de `check-parity-call-site-pins.sh`). Também uma linha,
   fecharia o único vetor de remoção silenciosa. Mas exige cuidado para não falhar quando o
   step for legitimamente renomeado ou movido.

Nenhum dos dois é necessário para aprovar a entrega. São melhorias de robustez, não correções
de defeito.

---

*Hades, Security Reviewer — hades-tf — 2026-09-22*
