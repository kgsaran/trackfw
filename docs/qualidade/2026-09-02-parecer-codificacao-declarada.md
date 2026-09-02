# Barreira final de qualidade — codificação de saída declarada (item 4 do #216)

> Autor: hefesto-tf (Code Quality) | Data: 2026-09-02
> Escopo: `git diff origin/main...HEAD` da branch
> `fix/saida-nao-ascii-declara-codificacao-em-script-gerado-e-em-gate` — 57 arquivos,
> ML-1A (prefixo por invocação nos 3 geradores), ML-1B (`export` em 37 gates),
> ML-2A (gate `check-output-encoding-declared.sh` + `Makefile` + `quality.yml` +
> `docs/cli-parity.md` + regeneração de `scripts/trackfw-attention-signal.sh`).
> Governança: `trackfw validate` → `exit 0`, 18 warnings pré-existentes, 0 violations;
> roadmap em `wip/`.

## Veredito

**APROVA COM RESSALVAS.**

O lote está substancialmente correto: os dois modos de falha sob cp1252 estão bem entendidos
(notas de vault de 2026-09-02), o remédio de ML-1A/ML-1B é o certo para cada sítio
(prefixo por invocação no literal gerado, `export` no gate), o gate novo tem guardas de
vacuidade reais, se auto-aplica, e a allowlist é verificada em três frentes em vez de ser um
skip silencioso. Não encontrei nenhum defeito no **produto** — `internal/`, `npm/src/`,
`pypi/trackfw/` estão corretos e em paridade byte-idêntica no sítio tocado.

**Duas ressaltas bloqueiam o merge**, ambas no mesmo bloco de regex/parsing do mesmo arquivo
(`scripts/check-output-encoding-declared.sh`), portanto **um único handoff**. Ambas são
*fail-open*: o gate devolve `exit 0` sobre uma regressão real, reproduzida por execução, com
controle negativo. As duas juntas custam menos de 15 linhas de correção.

O resto — inclusive os itens 2, 4 e 5 do pedido — **não bloqueia**.

---

## BLOQUEIA O MERGE

### B1 — Comentário inline citando `<<` derruba o arquivo da população, em silêncio, e o gate fica cego a uma regressão real

**Arquivo:** `scripts/check-output-encoding-declared.sh:152` (`HEREDOC_RE`) e `:155-174`
(`code_lines`), interagindo com `:221-227` (guarda de vacuidade *b*).
**Severidade:** ALTA — *fail-open* com gatilho acidental.

`HEREDOC_RE` procura o delimitador na linha inteira. `COMMENT_RE` protege a linha
**inteiramente** comentada (é testado antes, em `:168`), mas **não** o comentário *inline* numa
linha de código. Uma linha como

```bash
true  # exemplo de heredoc: <<EOF ... EOF
```

liga o estado de heredoc e faz `code_lines` **descartar todo o resto do arquivo**. O arquivo
some da população de invocadores — e nenhuma das três guardas de vacuidade pega isso: a (a) só
mede o glob, a (b) só dispara se a população ficar **totalmente** vazia, e a (c) só assevera a
auto-inclusão do próprio gate.

**Falsificado por execução, com controle negativo** (cópias em scratchpad, `scripts/` inteiro):

| Árvore | Mutação | População | Veredito |
|---|---|---|---|
| baseline | nenhuma | 38 invocadores, 37 checados | `OK`, exit 0 |
| `t2` | comentário inline com `<<EOF` inserido em `check-tty-detection.sh` antes do `export` | **37** invocadores, 36 checados | `OK`, exit 0 |
| `t2` (composto) | idem **+ remoção do `export PYTHONIOENCODING=utf-8`** do mesmo arquivo | 37, 36 | **`OK`, exit 0** ❌ |
| `t3` (controle) | apenas a remoção do `export`, sem o comentário | 38, 37 | `FAIL`, exit 1 ✅ |

A última linha é o defeito: exatamente a regressão que esta REQ existe para impedir passa
verde, porque uma linha de **prosa** desarmou a varredura. Reproduzi só a direção
**falso-negativo**; a direção falso-positivo é plausível pelo mesmo mecanismo, mas não a
demonstrei.

**Por que isto bloqueia e o achado análogo de 2026-09-01 (regex literal evadível) não bloqueou:**

1. **Distância até o gatilho.** Aquele exigia um autor escrever sintaxe equivalente
   deliberadamente. Este exige um autor escrever um **comentário contendo `<<`**. A árvore já
   tem cinco desses comentários — `scripts/check-gates-falsify.sh:227,246,5600`,
   `scripts/check-doctor-parity.sh:86`, `scripts/trackfw-git-branch-guard.sh:96-97`. Estão
   inertes hoje **apenas** porque são de linha inteira. Uma reflow de parágrafo os arma.
2. **O repositório já tem o idioma que fecha isto.** `assert_count` existe exatamente para
   impedir encolhimento silencioso de população — foi o ponto que este mesmo papel elogiou no
   parecer de 2026-09-01 sobre `check-shell-posix-portability.sh`. Um gate que assevera a
   própria auto-inclusão mas **não** o tamanho da população é inconsistente com o padrão que o
   projeto já aplica.

**Remédio proposto — e verificado contra a árvore atual.** A exclusão de heredoc é necessária
**só do lado da DECLARAÇÃO** (menção morta dentro de corpo de heredoc não pode contar como
declaração). Do lado da **POPULAÇÃO** ela é o que abre o buraco. Separe os dois predicados:

- **população** (`first_py3` em `:192`, e a asserção (c) da allowlist em `:264` — "continua
  invocando python3?", que é pergunta de população, não de declaração): linhas sem comentário de
  linha inteira, **sem** estado de heredoc;
- **declaração** (`DECL_RE` em `:198`, `ANY_ASSIGN_RE` em `:200`, e as asserções (a)/(b) da
  allowlist em `:246`/`:256`): `code_lines` como está hoje, com exclusão de heredoc.

A separação da asserção (c) importa: sob o predicado estrito, se
`check-roadmap-barrier-contract.sh` ganhasse um comentário inline com `<<`, o gate dispararia
`"ALLOWLIST SEM OBJETO"` sem motivo. Falha fechada e ruidosa, não é risco — mas é ruído evitável,
e explicitar isso poupa uma decisão do especialista.

Medi as duas populações sobre a árvore atual: **strict 38, loose 38, delta vazio nas duas
direções**. O remédio é drop-in, não quebra o build, e não muda a contagem reportada. Se um dia
divergirem, o comportamento correto é reprovar apontando a divergência, e não escolher em
silêncio.

Alternativa complementar (barata, cabe junto): fixar a população esperada
(`assert_count`-style — hoje 38 enumerados / 38 invocadores) e reprovar quando o número mudar
sem que o gate tenha sido atualizado. Isso cobre também qualquer outro mecanismo futuro de
encolhimento.

**Handoff:** especialista de infra de gate / shell, despachado pelo arquiteto. Merece **nota de
vault** — o mecanismo (comentário inline arma rastreador de heredoc → perda silenciosa de
população) não é derivável do diff e custaria bem mais de dez minutos a quem topar com ele
amanhã. Não a escrevi: está fora da minha fronteira de escrita neste ciclo.

---

### B2 — `re.IGNORECASE` sobre o **nome** da variável aceita `pythonioencoding=utf-8`, que em bash é outra variável — nos **dois** alvos

**Arquivo:** `scripts/check-output-encoding-declared.sh:140-148` (`DECL_RE`, `ANY_ASSIGN_RE`) e
`:285-289` (`PREFIX_RE`) — as **três** regexes carregam `re.IGNORECASE`.
**Severidade:** MÉDIA-ALTA no alvo 1; **ALTA no alvo 2**, que incide sobre o produto.

O alias de codec do Python (`utf-8`/`utf8`/`utf_8`/`u8`) é case-insensitive — daí o
`IGNORECASE` fazer sentido **para o valor**. Mas ele está aplicado à regex inteira, incluindo o
**nome da variável de ambiente**, que em POSIX shell é case-**sensitive**. `pythonioencoding` é
uma variável distinta; o Python nunca a lê.

**B2a — alvo 1** (árvore `t5`, `check-tty-detection.sh`):

```
export PYTHONIOENCODING=cp1252     → FAIL, exit 1  (mensagem de "forma nao e aceita")   ✅
export pythonioencoding=utf-8      → OK,   exit 0                                       ❌
( export PYTHONIOENCODING=utf-8 )  → FAIL, exit 1  (subshell — recusa correta)          ✅
```

A segunda linha é uma declaração **sem nenhum efeito** aceita como conforme. Pior: por
`ANY_ASSIGN_RE` carregar o mesmo `IGNORECASE`, o diagnóstico de "forma não aceita" — que seria a
mensagem certa aqui — também não dispara.

**B2b — alvo 2, e é o que tem raio de alcance maior.** `PREFIX_RE` (`:285-289`) tem o mesmo
defeito, e o alvo 2 é o **produto**: o literal `attentionSignalScript` que é escrito na máquina
de quem adota o trackfw. Falsificado (árvore `t6`): trocando as duas invocações de
`npm/src/generators/hooks.js` para `pythonioencoding=utf-8 python3 -c …`, o gate reporta
**`2/2 invocacoes com prefixo`** e devolve **`OK`/exit 0** — uma reversão silenciosa e completa
do ML-1A naquele runtime, tratada como conforme. Um prefixo de comando em minúsculas seta uma
variável que o Python nunca lê.

Registro isto como sub-achado do mesmo defeito, e não como achado separado, porque a causa e a
correção são a mesma linha de raciocínio — mas ele **precisa estar no handoff**: quem aplicar só
o remédio do alvo 1 deixa o furo do produto aberto.

**Remédio — verificado por execução, nas quatro direções.** Tirar `re.IGNORECASE` das **três**
regexes e mover a case-insensitividade para dentro do grupo de aliases, que é o único lugar onde
ela é correta:

```python
UTF8_ALIASES = r"(?i:utf-8|utf8|utf_8|u8)"

DECL_RE = re.compile(
    r'^[ \t]*export[ \t]+PYTHONIOENCODING=(?P<q>["\']?)' + UTF8_ALIASES
    + r'(?::[A-Za-z0-9_]+)?(?P=q)[ \t]*(?:#.*)?$')          # sem re.IGNORECASE

ANY_ASSIGN_RE = re.compile(r'^[ \t]*(?:export[ \t]+)?PYTHONIOENCODING=')   # sem re.IGNORECASE

PREFIX_RE = re.compile(
    r'PYTHONIOENCODING=(?P<q>["\']?)' + UTF8_ALIASES
    + r'(?::[A-Za-z0-9_]+)?(?P=q)[ \t]+python3[ \t]+-c\b')  # sem re.IGNORECASE
```

Apliquei este patch a uma cópia e rodei os quatro cenários:

| Árvore | Estado | Com o patch |
|---|---|---|
| atual (sem mutação) | conforme | `OK`, exit 0, `38 invocam / 37 checados`, `2/2` nos 3 runtimes — **sem falso positivo** |
| `t5` — `export pythonioencoding=utf-8` | B2a | **`FAIL`**, exit 1, `"NAO declara"` ✅ |
| `t6` — prefixo minúsculo no ALVO 2 | B2b | **`FAIL`**, exit 1, nomeando `hooks.js` linhas 147, 148 ✅ |
| `export PYTHONIOENCODING=UTF-8` (valor em caixa alta) | forma legítima documentada em `:41` | `OK`, exit 0 — **a tolerância de caixa do valor é preservada** ✅ |

**Handoff:** mesmo especialista, mesmo arquivo, mesmo commit que B1.

---

## NÃO BLOQUEIA — vira REQ/ML de acompanhamento

### S1 — `ALLOWLIST` vazia + `bash` 3.2 sob `set -u` → `unbound variable`

**Arquivo:** `scripts/check-output-encoding-declared.sh:109-111` e `:120`.
**Severidade:** BAIXA. Falha **fechada** e ruidosa; não mascara nada.

`"${ALLOWLIST[@]}"` sob `set -u` estoura em bash < 4.4 quando o array está vazio. Reproduzi com
`/bin/bash` 3.2.57 (o bash de sistema do macOS): `line 118: ALLOWLIST[@]: unbound variable`,
exit 1. Sob bash 5.3 (que é o que o shebang `env bash` resolve nesta máquina e o que roda no
`ubuntu-latest` do job `parity`) o cenário passa normalmente e reprova corretamente o
`check-roadmap-barrier-contract.sh`.

O que torna isto digno de nota não é o risco hoje, e sim que o próprio comentário do arquivo
(`:106`) instrui **"Quando o #238 fechar: remover desta lista"** — ou seja, o caminho de limpeza
documentado é exatamente o gatilho. Endurecimento de uma linha, para colar junto do comentário:

```bash
python3 - "$SELF_BASENAME" "${#ALLOWLIST[@]}" ${ALLOWLIST[@]+"${ALLOWLIST[@]}"} "${ATTENTION_SOURCES[@]}" <<'PYEOF'
```

### S2 — `ATTENTION_SOURCES` pode encolher sem que nada reclame

**Arquivo:** `scripts/check-output-encoding-declared.sh:113-118` e `:332-333`.
**Severidade:** BAIXA.

O ALVO 2 valida cada fonte listada, mas **não** assevera que as três estão listadas. Reduzir a
lista a um runtime deixa o gate verde cobrindo um terço. A cobertura combinada sobrevive na
prática, porque `check-attention-scripts-parity.sh` força os três a concordarem entre si — mas
essa é uma dependência implícita entre dois gates, não uma asserção. Endurecimento de uma linha:
`if len(attention_sources) != 3: failures.append(...)`, que substitui com vantagem o
`if not attention_sources` de `:332`.

Sobre `:332`: **não é código morto** — é alcançável esvaziando o array bash (em bash ≥ 4.4). É
só uma guarda mais fraca do que precisava ser, e posicionada depois do laço que ela deveria
proteger.

### S3 — `scripts/trackfw-attention-signal.sh` (cópia no repo) não é guardada contra o literal

**Arquivos:** `scripts/trackfw-attention-signal.sh:14-15`; `scripts/check-attention-scripts-parity.sh:150-190`.
**Severidade:** BAIXA. **Pré-existente — não é regressão deste lote.**

O ML-2A regenerou essa cópia corretamente. Mas nenhum gate a compara com o literal: o
`check-attention-scripts-parity.sh` gera os três runtimes em tempdirs e faz diff
**gerado-vs-gerado**, nunca contra a cópia versionada; e o ALVO 2 do gate novo olha só as três
fontes. Falsifiquei: removendo o prefixo **só** de `scripts/trackfw-attention-signal.sh`, o gate
novo continua `OK`/exit 0. O impacto é limitado ao hook do próprio repositório, não ao produto
distribuído.

### S4 — Sobra-declaração em `check-shell-posix-portability.sh`: a decisão está certa, o comentário está errado

**Arquivo:** `scripts/check-shell-posix-portability.sh:44-56` (bloco do ML-1B) — `python3`
aparece só nas linhas 29 e 46, **ambas comentário**.
**Severidade:** BAIXA (comentário enganoso). Item 6 do pedido.

**A assimetria do gate está certa**, e recomendo mantê-la: ele assevera "quem invoca, declara"
sem assevera "quem não invoca, não declara". Exigir o inverso criaria manutenção reativa
(qualquer arquivo que ganhe uma invocação amanhã já estaria pronto hoje) sem fechar nenhum
buraco de segurança. E o predicado de população está semanticamente correto: como `code_lines`
exclui comentários também para `PY3_RE`, este arquivo é **corretamente excluído** dos 38
invocadores — foi isso que reconciliou minha contagem (ver item 4 abaixo).

O defeito é textual: o bloco inserido afirma *"forca UTF-8 no stdio de todo python3 deste gate"*
quando **não há nenhum** `python3` neste gate. Um leitor futuro conclui que o arquivo invoca
Python. Remédio: emendar a frase (`"…caso este gate passe a invocar python3"`) ou remover o bloco
deste arquivo — as duas são aceitáveis; a primeira é mais barata e não requer tocar a contagem
declarada em `docs/cli-parity.md`.

### S5 — Duplicação de 11 linhas de racional × 37 arquivos: a decisão está certa, o tamanho não

**Arquivos:** 37 × `scripts/check-*.sh`, bloco `# Codificacao de saida (ML-1B…)` + `export`.
**Severidade:** BAIXA. Item 2 do pedido.

**A justificativa de não centralizar se sustenta — verifiquei no código, não no relato.**
`scripts/check-gates-falsify.sh` copia gates **individualmente** para sandboxes
(`cp "$ROOT_DIR/scripts/<gate>.sh" "$T<n>/scripts/"`, ~26 ocorrências entre as linhas 289 e
3855), sem irmãos. Um `source scripts/_lib/encoding.sh` quebraria toda cópia sandboxada. A
autocontenção do arquivo é a propriedade que carrega o peso, e a linha `export` **tem** de ser
duplicada. Isso é custo certo.

O que **não** precisa ser duplicado é o racional de 11 linhas. A duplicação já drifta: medi os
blocos por hash — **37 idênticos + 1 variante** (a do próprio gate novo, que acrescenta a nota de
auto-aplicação). Com n = 38 e duas variantes já no primeiro dia, qualquer revisão futura do
trade-off (a frase *"mojibake em vez de crashar"* é uma decisão de produto, não um detalhe) exige
editar 407 linhas em lockstep, ou aceitar divergência.

**Forma melhor que preserva a propriedade:** manter o `export` autocontido e reduzir o
comentário a um ponteiro de uma linha —
`# UTF-8 no stdio do python3 deste gate; racional em docs/cli-parity.md § "Codificação de saída declarada".`
Custo: um salto de leitura. Ganho: uma única fonte de verdade para o trade-off. Não bloqueia, e
faz sentido como ML transversal barato depois de B1/B2.

---

## Itens do pedido — resposta direta

### 1. O gate novo é manutenível? Um leitor futuro entende por que cada forma é aceita ou recusada, sem arqueologia?

**Sim, e isto é o ponto mais forte do lote.** As linhas 31-80 enumeram nominalmente as formas
aceitas *e* as recusadas, **com o motivo de cada recusa** — inclusive as duas recusas que são
decisão e não descuido (assignment sem `export`; forma de prefixo por invocação no alvo 1, com a
inversão explicada para o alvo 2). Nenhuma exige abrir o roadmap ou o git log. As três guardas
de vacuidade estão nomeadas (a)/(b)/(c) e comentadas no ponto de uso; a allowlist declara as três
asserções e a condição de remoção. É a documentação in-loco mais completa entre os gates que
li neste repositório.

Acoplamento: baixo e explícito — o gate depende de `scripts/check-*.sh` (glob), de três caminhos
de fonte hardcodados e de nada mais; não invoca binário, não faz git, não escreve disco, roda em
0,05 s. Legibilidade do heredoc Python: boa; funções curtas, uma responsabilidade por bloco,
mensagens de falha que dizem **o que fazer** e não só o que quebrou.

A ressalva de manutenibilidade não é sobre o texto — é que o texto documenta corretamente uma
regra que o **parser** implementa com dois desvios (B1 e B2). O leitor futuro vai confiar no
comentário; é o código que precisa alcançá-lo.

### 2. Repetição × 37 — ver S5. Justificativa **se sustenta**; o tamanho do bloco é o que vale reduzir.

### 3. Risco do rastreador heurístico de heredoc

**O risco deixa de ser inerte com uma linha de prosa** — demonstrado em B1, com controle
negativo. Confirmo a inertness de hoje pelo mesmo caminho aritmético do ML: contagem
independente por *strip* de comentário (predicado loose) dá **38**, idêntica à do gate
(**38**), delta vazio nas duas direções. Mas "inerte hoje" é uma propriedade do conteúdo dos
comentários da árvore, não uma propriedade do gate; e as cinco menções `<<` já presentes só estão
salvas por serem de linha inteira. **A declaração não é suficiente** — o encolhimento é
silencioso, e a correção custa poucas linhas.

### 4. A anotação `partial=` em `docs/cli-parity.md` é honesta ou desculpa?

**Honesta, e a fronteira declarada bate com o que o código faz.** Confirmei ponto a ponto:

- "asserção ESTÁTICA sobre o texto-fonte" — verdade: nenhuma execução de gate, nenhum
  `subprocess`, nenhum oráculo de runtime.
- "existe, é exportada, tem valor alias de utf_8 e precede a primeira invocação" — verdade:
  `DECL_RE` exige `export`, o alias vem de `UTF8_ALIASES`, e a ordem é `decl > first_py3` em
  `:214`. (A ressalva B2 é sobre a **caixa do nome**, que a anotação não afirma cobrir nem
  deixar de cobrir.)
- "formas recusadas por decisão e não por descuido" — verdade, e as duas estão nomeadas.
- "`HEREDOC_RE` procura o delimitador na linha inteira, então um comentário inline citando `<<`
  iniciaria estado de heredoc" — **descrição exata do mecanismo que reproduzi**. A anotação nomeia
  o próprio furo.
- "hoje é comprovadamente inerte — contagem independente (37) mais o próprio gate bate com os 38"
  — a aritmética fecha: **45** globados → **38** invocadores → **37** checados + **1** na
  allowlist, com `check-shell-posix-portability.sh` corretamente **fora** dos 38 por só mencionar
  `python3` em comentário. É isso que torna o "37 dos 38" do texto preciso, e não impreciso.

Ou seja: a anotação não vendeu o que não entrega — ela **nomeia** o defeito. O que peço em B1 não
é honestidade adicional; é que o defeito nomeado seja corrigido, porque a distância até o gatilho
é uma linha de comentário e o repositório já tem o idioma que fecha isso.

### 5. Sobra-declaração — ver S4. Decisão certa, comentário enganoso.

### 6. Código morto / duplicação / nome enganoso

- **Nada de código morto.** `:332` (`if not attention_sources`) é alcançável, só está mal
  posicionada e é mais fraca do que devia (S2).
- **Duplicação:** a de S5 (racional × 37), única do lote. A duplicação executável (`export`) é
  necessária.
- **Nome enganoso:** o comentário de `check-shell-posix-portability.sh` (S4). Nomes de variável e
  de função no gate são precisos — `ALVO 1`/`ALVO 2`, `first_py3`, `code_lines`, `invokers`,
  `ANY_ASSIGN_RE` dizem exatamente o que fazem. `code_lines` tem docstring que descreve a
  exclusão dupla e o porquê.
- O `2>/dev/null` nas duas invocações do `attentionSignalScript` engole um eventual crash e cai
  no fallback `"Agent needs attention"` — **pré-existente**, não introduzido aqui, e é
  exatamente o que torna o modo *mismatch silencioso* das notas de vault mais perigoso que o
  crash. Registro como observação; não é achado deste lote.

### 7. `make quality`

Iniciado por mim em background sobre a árvore da branch, sem pipe, `MAKE_EXIT` capturado. O
resultado observado está no adendo abaixo. Não presumi o `exit 0` relatado pelo arquiteto — o
gate central deste lote foi, além disso, exercitado de forma direta e independente (baseline +
quatro árvores mutadas), o que sustenta o veredito sem depender de uma janela de `make quality`
estática.

`bash scripts/check-output-encoding-declared.sh` → `exit 0`, 0,05 s:
`45 enumerados, 38 invocam python3, 1 na allowlist, 37 checados` + `2/2` nos três runtimes.

---

## Resumo

| # | Item | Resultado |
|---|---|---|
| B1 | `HEREDOC_RE` × comentário inline → perda silenciosa de população | **BLOQUEIA** — fail-open falsificado com controle negativo; remédio verificado (strict 38 = loose 38) |
| B2 | `re.IGNORECASE` sobre o nome da variável, nas **3** regexes | **BLOQUEIA** — B2a: `export pythonioencoding=utf-8` aceito no alvo 1. **B2b: `PREFIX_RE` aceita prefixo minúsculo no alvo 2 (produto), reportando `2/2` e exit 0.** Remédio verificado nas 4 direções |
| S1 | `ALLOWLIST` vazia sob bash 3.2 | Follow-up — falha fechada; gatilho é o caminho de limpeza documentado |
| S2 | `ATTENTION_SOURCES` pode encolher | Follow-up — coberto na prática pelo gate de paridade, não por asserção |
| S3 | Cópia versionada do attention-signal sem guarda | Follow-up — **pré-existente** |
| S4 | Sobra-declaração em `check-shell-posix-portability.sh` | Follow-up — decisão certa, comentário enganoso |
| S5 | Racional de 11 linhas × 37 arquivos | Follow-up — justificativa **se sustenta**; reduzir a ponteiro de 1 linha |
| 1 | Manutenibilidade do gate | Alta — formas aceitas/recusadas documentadas in-loco, sem arqueologia |
| 4 | Anotação `partial=` | **Honesta** — fronteira declarada bate com o código; ela nomeia o próprio furo de B1 |

**Bloqueantes: 2**, ambos em `scripts/check-output-encoding-declared.sh`, um único handoff,
< 20 linhas, ambos com remédio verificado por execução.

**Produto (`internal/`, `npm/src/`, `pypi/trackfw/`): sem achados** — o literal
`attentionSignalScript` está correto e byte-idêntico nos três runtimes. A distinção importa:
o defeito B2b não é no produto, é na **barreira que deveria protegê-lo** — hoje ela deixa passar
uma reversão completa do ML-1A num runtime, reportando `2/2` e `exit 0`.

---

## Adendo — `make quality`

**`MAKE_EXIT=0`** — observado, não presumido. Log completo com 3.613 linhas.

Varredura por `FAIL|not ok|^Error:|panic:` devolve **3 ocorrências, todas benignas**, e
inspecionei as três: são linhas de prosa dentro de blocos `PROOF [falsify/…/non-vacuity]`
(linhas 3180 e 3289) que **citam** a mensagem de falha esperada da árvore desligada — é o texto
da prova de não-vacuidade, não uma falha — e a linha de sumário 3488
(`Falsification checks passed (all 181 scenarios, 23 gates + 11 generator/validator contracts…)`).
Nenhuma falha real.

Últimas linhas do log, confirmando que o gate novo entrou pelo caminho de `make parity`:

```
  ALVO 2: pypi/trackfw/generators/init_gen.py — 2/2 invocacoes com prefixo.
check-output-encoding-declared: OK
MAKE_EXIT=0
```

Nota: o `make quality` verde **não contradiz** B1 e B2. Os dois são fail-open — o gate devolve
`exit 0` sobre a mutação, que é exatamente o que o `make quality` mede. Só as árvores mutadas
com controle negativo os expõem, e é por isso que a auditoria não parou no `exit 0`.
