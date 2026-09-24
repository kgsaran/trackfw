# Cross-check de dois instrumentos **concorda em zero sem medir** — e migrar a alegação para o marcador inline deixa a guarda de obsolescência sem fixture

> 2026-09-24 · ML-2K, REQ-2026-09-23 (a apuração do censo morre no shard limpo) ·
> medido em `bash 5.3.20` (macOS/ARM64) sobre o bloco `run:` **extraído do próprio
> `.github/workflows/windows-census.yml`** por `yaml.safe_load`, nunca recopiado à mão

---

## 1. A mesma defesa falhou três vezes na mesma REQ, por três mecanismos diferentes

O par de contadores da apuração do censo (`windows-census.yml`, laço de shards):

```bash
CNT_FAIL_GREP=$(grep -ac '^FAIL' "$LOG_FILE" 2>/dev/null || true)
CNT_FAIL_AWK=$(awk '/^FAIL/{n++} END{print n+0}' "$LOG_FILE")
if [[ "$CNT_FAIL_GREP" != "$CNT_FAIL_AWK" ]]; then ::warning:: … fi
```

| # | mecanismo | efeito |
|---|---|---|
| 1 | **causa A** — `\|\| echo 0` fazia a captura valer `$'0\n0'` | a comparação acusava `grep=0\n0 awk=0`: **divergência inexistente**, e o `$(( ))` a jusante matava o laço |
| 2 | corrigida a captura para `\|\| true` | a comparação voltou a funcionar |
| 3 | **este ML** | com log **sem nenhuma linha de veredito**, as quatro contagens valem `0`, **concordam**, e o par **não mediu nada** |

🔴 **A lição é sobre a forma, não sobre o bug:** duas capturas do **mesmo produtor** comparadas entre
si são **auto-concordantes no vazio**. Concordância só é evidência **depois** que pelo menos um
instrumento viu uma linha. Não-vacuidade é **pré-condição** da comparação, nunca resultado dela — é
a mesma frase da nota irmã do `sed -n`, aqui na roupa de "dois instrumentos".

🔴 **E a correção NÃO é um terceiro contador.** Mais instrumentos sobre o mesmo produtor vazio
continuam concordando. O que falta é **testemunha de que houve medição** — e ela sai das quatro
capturas que já existem: `CNT_FAIL_GREP + CNT_OK_GREP + CNT_FAIL_AWK + CNT_OK_AWK == 0`.

## 2. O dano não é o aviso ausente — é o **cadáver lido como melhora**

Medido no bloco **pré-ML-2K**, com o shard 3 substituído por um log de chunk que morreu no preâmbulo
(129 bytes de erro de sintaxe, zero `^OK`/`^FAIL`):

```
  shard 3: OK=0 FAIL=0  (base FAIL=113 delta=-113 ▼)
  queda de FAIL: 113
Shards completos: 8/8      ← no GITHUB_STEP_SUMMARY
rc do step = 0 · 0 avisos
```

O shard morto entra no total como **a maior melhora da tabela** (`▼ -113`), e a apuração declara
**8/8 completos**. É exatamente a patologia que dá nome à REQ, um nível acima: não é o chunk que
morre em silêncio, é a **apuração** que assina o atestado de completude sobre ele.

## 3. Quais casos são alcançáveis — medidos, para não guardar ramo morto

| caso | alcançável? | tratamento |
|---|---|---|
| log **ausente** nos dois layouts | sim | já tratado antes deste ML (`SEM LOG`) |
| log presente, **0 bytes** | sim (chunk morto antes de qualquer escrita) | **guarda nova** |
| log presente, com texto e **0 linhas de veredito** | sim, e é o caso realista (erro de sintaxe do chunk, `GUARDA LOCAL`, stub de Python) | **guarda nova** |
| log **ilegível** | 🔴 **não chega à guarda** | morte ruidosa |

Os dois do meio entram no **mesmo ramo**: o discriminante é idêntico (zero linhas de veredito), e o
tamanho em bytes vai no diagnóstico. Dois ramos ali seriam distinção sem diferença.

O caso ilegível foi **medido**, não presumido (`chmod 000` no `shard_2.log`):

```
awk: can't open file censo-artifacts/falsify-shard-2/shard_2.log
rc do step = 2
```

O `awk` da segunda captura **não tem** `|| true` nem `2>/dev/null`; sob `set -e` ele mata o step **na
própria atribuição**, antes da guarda. É morte ruidosa, não vacuidade — e por isso **não** ganhou
ramo.

## 4. Falsificação nas duas direções (três, na verdade)

| direção | resultado |
|---|---|
| log com texto sem veredito | `::error:: LOG SEM VEREDITO (apuração): 129 byte(s)…`, shard fora do total, `TOTAL INCOMPLETO — 7/8`, linha da tabela `LOG SEM VEREDITO (tabela)` |
| log de 0 bytes | idem, com `0 byte(s)` |
| 8 logs normais | 🔴 saída **byte-idêntica** à de antes do ML (console **e** `GITHUB_STEP_SUMMARY`), `stderr` vazio, `Shards completos: 8/8`, queda calculada |

A terceira direção é a que evita "corrigir" trocando falso-negativo por ruído: a igualdade é
**byte-a-byte contra o bloco extraído de `git show HEAD`**, não uma inspeção visual.

**Misto** (shard 5 sem log + shard 3 sem veredito) imprime as duas listas separadas —
`sem log: 5` / `log sem veredito: 3` — e `6/8`. Contar um shard como ausente **sem dizer qual** era
um diagnóstico que se autocontradiz (`apenas 7/8` com lista vazia); por isso a lista nova aparece
nos **três** lugares que falam de completude.

**Segundo sítio da mesma causa:** o laço que monta a tabela `### Por shard` do summary recomputa
`CNT_*_R` e **não tem par** (não existe `awk` ali) — a testemunha é só a soma. O rótulo é
**deliberadamente distinto** (`(tabela)` vs `(apuração)`) para que o log do job diga qual dos dois
disparou.

## 5. 🔴 A tabela `ALLEGATIONS` era a **única fixture** do braço P — esvaziá-la desarma a guarda

O ML-2H registrou as duas isenções de classe 6 (`check-agent-namespace-union.sh` `alfa_ln`/`zulu_ln`)
na tabela `ALLEGATIONS` do gate porque **estava proibido de editar arquivo de produto**. O ML-2K
migrou-as para a forma preferida — marcador inline no sítio — e a tabela ficou **vazia**. Duas
consequências, ambas medidas:

**(a) O braço P do Cenário 199 deixa de falsificar.** Ele afirma que uma alegação que não casa sítio
nenhum **reprova**, e a alegação que ele usa como fixture é a **da tabela real**, vista de uma árvore
sintética onde o sítio não existe:

```bash
# tabela vazia + FORCE_ALLEGATION_GUARD=1 sobre a árvore do braço M:
=== guarda de obsolescencia das alegacoes (classe 6) ===
check-unguarded-capture-rc: OK …            rc=0     ← o braço P espera "alegacao obsoleta"
```

**(b) Na árvore real a guarda fica verde porque itera ZERO entradas.** Isto é, literalmente, "o gate
que examina zero e reporta sucesso" — a classe de defeito que o cabeçalho do próprio gate nomeia
como razão de existir. 🔴 **"Guarda de obsolescência verde" passou a ser verdade vácua**, e declarar
o critério atendido sem dizer isso seria a mesma contradição interna que a Regra Dura de
Reconciliação deste projeto existe para pegar.

**Por que não foi corrigido aqui:** as duas saídas possíveis — fixture sintética injetável no gate,
ou braço P construindo a própria alegação — são **lógica de gate** e **`check-gates-falsify.sh`**,
os dois fora da fronteira de escrita do ML-2K. Fica como achado para o arquiteto, registrado também
em comentário no corpo da tabela vazia.

**Nota lateral, medida:** a migração **não** podia manter as duas formas ao mesmo tempo. O marcador
inline é consultado **antes** da tabela e retorna, então `ALLEGATION_HITS` ficaria em 0 e a guarda de
obsolescência **reprovaria na árvore real**. Marcador e entrada de tabela para o mesmo sítio são
mutuamente exclusivos por construção.

**Nota lateral 2:** o marcador precisa estar **imediatamente acima de cada** sítio. `alfa_ln` e
`zulu_ln` são linhas **adjacentes**: um único marcador acima do primeiro deixaria o segundo com um
predecessor que não é comentário, e ele voltaria a ser **violação**. Conferido lendo as duas linhas
`OK   [exempt/class6-alleged-inline/…]` — não pelo `rc=0`, que também seria compatível com isenção
pela classe errada.

**Nota lateral 3:** o gate exige `bash` ≥ 4 de qualquer forma (`mapfile`, `declare -A`) — o `/bin/bash`
3.2 do macOS não o roda, medido. Resta o intervalo **4.0–4.3**, onde `"${ARRAY[@]}"` de array
**vazio** sob `set -u` é erro de variável não vinculada: não medido, declarado. Em `bash` 5.3 a
expansão da tabela vazia é inócua.

## 6. Regra operacional que sai daqui

> **Comparar dois instrumentos do mesmo produtor não é medição — é medição *mais* uma testemunha de
> que houve o que medir.** Sem a testemunha, o par é um `assertEquals(0, 0)` com dois nomes
> diferentes.

E o corolário de instrumento, que esta campanha já pagou nove vezes: **o bloco `run:` de um workflow
se falsifica extraindo o YAML programaticamente**. Recopiar o bloco à mão para um harness é criar um
décimo instrumento que mente.
