# Barreira final de segurança — `fix/saida-nao-ascii-declara-codificacao-em-script-gerado-e-em-gate`

> Revisor: `hades-tf` (Security). Diff auditado: `git diff origin/main...HEAD` (57 arquivos;
> ML-1A `5b5391e`, ML-1B `6721078`, ML-2A `486b5a0` + commits de governança).
> Escopo de código: `internal/generators/scaffold.go`, `npm/src/generators/hooks.js`,
> `pypi/trackfw/generators/init_gen.py`, `scripts/trackfw-attention-signal.sh`,
> 37 × `scripts/check-*.sh`, `scripts/check-output-encoding-declared.sh`.
> Verificação **por execução** nas duas direções, conforme `feedback_verify_by_execution`.
> Vault lido antes de investigar: `vault/notes/index.md` + as três notas de 2026-09-02.

## Veredito: **APROVA COM RESSALVAS**

**Nada bloqueia o merge.** A mudança de decodificação **não abre nenhum primitivo de injeção novo**
— eu procurei um e não achei, e abaixo está o que testei e por que o resultado discrimina. Ao
contrário: medido em bytes, o estado **anterior** ao ML-1A produzia um `.trackfw-attention.json`
**inválido em UTF-8** sob console cp1252, e o ML-1A elimina essa corrupção. O defeito era rotulado
como de disponibilidade; na medição ele também era de **integridade do artefato**, e o fix corrige
os dois.

**Uma ressalva vira REQ de acompanhamento (S3):** a justificativa escrita do gate novo para aceitar
`PYTHONIOENCODING=utf-8:<handler>` é **empiricamente falsa**, e duas das formas que ele aceita hoje
(`utf-8:surrogatepass`, `utf-8:surrogateescape`) reintroduzem exatamente o byte inválido que o gate
existe para impedir. Provado por execução nas duas direções. Não bloqueia porque a árvore usa a
forma canônica e nada hoje escreve um handler — é um controle que **greenlightaria a regressão que
ele foi construído para pegar**.

---

## 0. Aparato de medição

O ramo `python3` do hook só executa quando `jq` está ausente. Montei um `PATH` isolado sem `jq`
(`env -i PATH=<bin-só-com-symlinks> HOME=… bash hook.sh < payload.json`), confirmando
`command -v jq` vazio antes de confiar em qualquer resultado.

Console cp1252 **não é reproduzível no macOS por locale** (a nota
`gate-em-cp1252-tem-duas-falhas-distintas…-2026-09-02` já registra isso: PEP 540 sob `LC_ALL=C`).
O braço "antes" foi reproduzido de forma determinística com `PYTHONIOENCODING=cp1252` sobre a **forma
antiga** da linha — que é exatamente o que um console Windows cp1252 faz ao stdio do Python. Isso dá
duas pontas comparáveis em vez de um locale que não discrimina.

Duas variantes do hook: `hook_after.sh` = cópia fiel de `scripts/trackfw-attention-signal.sh`;
`hook_before.sh` = a mesma com `PYTHONIOENCODING=utf-8 python3` → `python3` (só as 2 linhas do ML-1A).

---

## 1. `PYTHONIOENCODING=utf-8` muda a superfície de entrada não confiável? — **Sim, e para melhor**

O argumento correto aqui não é um cenário, é o **conjunto-delta**: quais caracteres eram rejeitados
antes e são aceitos agora, e se esse conjunto contém `"`, `\` ou algum byte C0. `PYTHONIOENCODING`
move **dois** canais — o *decode* de stdin e o *encode* de stdout — e a nota
`cp1252-roundtrip-mascara-o-defeito-o-discriminante-e-decode-de-stdin-2026-09-02` diz que o
discriminante é o primeiro. Medi os dois separadamente: §1a o encode, §1b o decode.

### 1a. Canal de *encode* (stdout do `python3` → escape do bash)

Medição (`scripts/trackfw-attention-signal.sh:15-16`), payload
`{"tool_name":"Bash","tool_input":{"question":"confirmar Área \"x\" \\ fim"}}`:

```
hook_before  PYTHONIOENCODING=cp1252  -> {"tool":"Bash","message":"confirmar <0xC1>rea \"x\" \\ fim",…}   INVÁLIDO em UTF-8
hook_after   PYTHONIOENCODING=cp1252  -> {"tool":"Bash","message":"confirmar Área \"x\" \\ fim",…}        utf8-ok, json-ok
hook_before  LC_ALL=en_US.UTF-8       -> {"tool":"Bash","message":"confirmar Área \"x\" \\ fim",…}        utf8-ok  (controle: não discrimina)
hook_after   LC_ALL=en_US.UTF-8       -> {"tool":"Bash","message":"confirmar Área \"x\" \\ fim",…}        utf8-ok  (controle)
```

O braço `LC_ALL=en_US.UTF-8` é o **controle**: antes e depois são idênticos, ou seja, em macOS/Linux
o ML-1A é um no-op comportamental. Toda a diferença está no braço cp1252.

Os três regimes do conjunto-delta, medidos:

| classe | exemplo | antes (cp1252) | depois |
|---|---|---|---|
| não representável em cp1252 | `✓` U+2713 | `UnicodeEncodeError` → `2>/dev/null \|\| echo` → `"Agent needs attention"` (fail-safe) | `"confirma ✓ ok"` |
| representável, mas **transcodificado** | `—` U+2014 → byte `0x97` | `{"message":"antes \x97 depois"}` → **arquivo inválido em UTF-8** | `"antes — depois"` |
| representável, transcodificado | `Á` U+00C1 → byte `0xC1` | idem, inválido em UTF-8 | correto |

Nota metodológica: os payloads acima são **ASCII puro no fio** (`json.dump` usa
`ensure_ascii=True`; o `Á` viaja como o escape `\u00c1`). Portanto este bloco isola **só o encode** —
o decode de stdin teve sucesso trivial nos dois braços. O decode está medido em §1b.

### 1b. Canal de *decode* (stdin não confiável → `json.load`) — o discriminante

Dois payloads com **bytes crus** no fio, um em cada direção:

```
A. bytes UTF-8 válidos contendo 0x81, INDEFINIDO em cp1252:  "conf \xc3\x81rea"  (Á)
   hook_before  cp1252 -> message='Agent needs attention'   <- UnicodeDecodeError, fallback
   hook_after   cp1252 -> message='conf Área'               <- aceito agora
   hook_after   UTF-8  -> message='conf Área'               (controle: idêntico)

B. bytes cp1252/latin-1 (0xE9), INVÁLIDOS em UTF-8:          "caf\xe9"
   hook_before  cp1252 -> INVALID-UTF8 b'{"…","message":"caf\xe9",…}'  <- passava, e corrompia
   hook_after   cp1252 -> message='Agent needs attention'               <- rejeitado agora, fail-safe
   hook_after   UTF-8  -> message='Agent needs attention'               (controle: idêntico)
```

**A superfície de decode mudou nas duas direções, e as duas são benignas.** O caso A é o
"antes rejeitava, agora aceita" que a revisão pediu para medir — e o que passa a ser aceito é texto
**correto**, escapado normalmente (§2). O caso B é o inverso: entrada latin-1 do harness que antes
chegava ao banner **agora cai no fallback genérico** — perda de fidelidade da mensagem
(disponibilidade), e note que o que ela produzia antes era justamente um artefato **inválido em
UTF-8**. Registrado como observação em S5.5, sem consequência de segurança.

### 1c. Conclusão do conjunto-delta

O conjunto-delta — nos dois canais — é composto exclusivamente de caracteres **não-ASCII**. `"`
(0x22), `\` (0x5C) e todos os bytes C0 codificam **identicamente** em cp1252 e UTF-8. E a afirmação
estrutural que sustenta isso está **medida**, não assumida — varredura exaustiva dos 1.114.112
pontos de código:

```
python3 -c "bad=[hex(c) for c in range(0x80,0x110000)
             if (lambda b: b in (b'\"', b'\\') or (b and b[0]<0x20))(safe_cp1252(c))]"
-> pontos nao-ASCII que cp1252 codifica para 0x22/0x5C/C0: NENHUM
```

No sentido do decode, a simetria também vale: cp1252 é single-byte e mapeia 0x22/0x5C só para
U+0022/U+005C, e o UTF-8 estrito do Python recusa formas overlong (`C0 A2`), então nenhum dos dois
regimes fabrica uma aspa ou barra a partir de byte não-ASCII.

**Logo a superfície ampliada não contém nenhum primitivo de injeção novo.** Negativo *medido*, nos
dois canais, não deduzido.

O que a superfície ampliada de fato faz é o inverso do temido: o segundo regime da tabela de §1a — o
modo que a nota de vault chama de "mismatch silencioso" — **deixa de escrever byte inválido no
artefato**.

---

## 2. Injeção no `.trackfw-attention.json` → banner do `trackfw serve` — **não encontrada**

### 2a. O sink do browser é inerte

`internal/serve/static/app.js:363-368` — os quatro campos renderizados usam `textContent`
(`attention-icon`, `attention-label`, `attention-context`, `attention-message`). Não há `innerHTML`,
`insertAdjacentHTML` nem escrita de atributo a partir do valor.

O único sink que **não** é `textContent` é `markCardAttention(data.roadmap)`
(`internal/serve/static/app.js:388-402`), e ele é igualmente inerte: percorre
`querySelectorAll('.kanban-card[data-file]')` — seletor **constante** — e compara
`c.getAttribute('data-file') !== roadmapFile` por **igualdade de string**. O valor nunca entra na
construção de um seletor nem em HTML. Mesmo com controle total de `roadmap`, o pior efeito é marcar
um card existente com uma classe CSS.

Isso fecha a pergunta antes mesmo do escape: um bypass do escape seria **cosmético**, não XSS.

### 2b. O escape resiste mesmo assim

`scripts/trackfw-attention-signal.sh:28-29` — `tr -d '\000-\037' | sed 's/\\/\\\\/g; s/"/\\"/g'`.
Ordem correta (barra antes de aspa). Medido com payload de fechamento de string, nos três braços:

```
payload: 'a","level":"info","roadmap":"X.md","z":"b'

hook_before  cp1252  -> keys=['tool','message','level','timestamp']  message='a","level":"info","roadmap":"X.md","z":"b'
hook_after   cp1252  -> keys=['tool','message','level','timestamp']  message=<idem, literal>
hook_after   UTF-8   -> keys=['tool','message','level','timestamp']  message=<idem, literal>
```

Quatro chaves exatamente, em todos os braços: **nenhum campo injetado**. Controle-negativo do
mecanismo: com `a\nb\r"c` o resultado é `message='ab"c'` — `\n`/`\r` removidos pelo `tr` (C0), a aspa
escapada. O mecanismo dispara no caso adversarial e não dispara no legítimo (§1, `Área \"x\" \\ fim`
chega íntegro e escapado).

---

## 3. Truncamento em 300 code points — **sem corte no meio de sequência, e o corte é defensivo**

`[:300]` fatia um `str` do Python (pontos de código já decodificados), então `print` só pode emitir
sequências UTF-8 **completas** — um corte no meio de multibyte não é construível. Medido com 298 ×
`A` + `Á` na posição 299 + carga de injeção depois do limite:

```
hook_after  cp1252 / UTF-8 -> utf8-ok, json-ok, 4 chaves, carga de injeção descartada pelo corte
hook_before cp1252         -> INVÁLIDO em UTF-8 (o Á virou 0xC1 antes do corte)
```

Vale notar que o mesmo vale para o ramo `jq` (`.[0:300]` também opera em pontos de código), então os
dois ramos concordam. **Sem achado.**

---

## 4. `export` do ML-1B alcança processos-filho — **confirmado por leitura, o argumento do `artemis-tf` procede, e por um motivo a mais**

Não aceitei o argumento; verifiquei os dois arquivos nomeados e um terceiro que ele não citou:

- `pypi/tests/test_cli_encoding.py:74-76` — `TestCliEmConsoleCp1252._run` faz
  `env = dict(os.environ)` e então **`env["PYTHONIOENCODING"] = "cp1252"`** + `env.pop("PYTHONUTF8", None)`.
  A atribuição explícita **vence** sobre qualquer valor herdado. O teste continua medindo o que
  pretende medir.
- `scripts/windows-repro/python/checks.py:34-35` e `:78-79` — `env.pop("PYTHONUTF8", None)` e
  **`env.pop("PYTHONIOENCODING", None)`** antes de cada medição "sem máscara"; nas linhas 163 e 233
  a variável é **setada de propósito** (`= "utf-8"`), documentado no docstring como neutralização do
  item 1. Nenhuma leitura implícita do valor herdado.
- **Terceiro ponto, que fecha o resto:** `pypi/trackfw/cli.py:47` `_force_utf8_output()` chama
  `reconfigure(encoding="utf-8", errors="replace", newline="\n")` **antes de qualquer escrita**
  (invocado em `:80`). `reconfigure()` sobrescreve o que `PYTHONIOENCODING` tiver estabelecido —
  então o `export` dos gates **não tem efeito algum** sobre o stdout do CLI Python do produto. Não
  há como cegar uma verificação por essa via.

**Sem achado.** O `export` alcança os filhos, sim, mas não existe consumidor cujo veredito dependa do
valor herdado.

---

## 5. O gate novo executa conteúdo de arquivo? — **não, e verifiquei especificamente**

`scripts/check-output-encoding-declared.sh:120` — heredoc **citado** (`<<'PYEOF'`), logo sem expansão
de shell no corpo. Argumentos passados como `"$SELF_BASENAME" "${#ALLOWLIST[@]}" "${ALLOWLIST[@]}"
"${ATTENTION_SOURCES[@]}"` — todos entre aspas, sem glob nem word-splitting. Dentro do Python só
`glob`, `os.path`, `open(..., errors="replace")` e `re`; `grep -n "eval\|exec(\|os.system\|subprocess\|popen"`
retorna **apenas ocorrências de crase em linhas de comentário** — nenhuma chamada. Caminhos e
conteúdo entram nas mensagens como **argumentos de `%`**, nunca como format string. Conteúdo de
arquivo nunca chega a um shell.

Executado limpo na árvore: `exit=0`, "45 scripts/check-*.sh enumerados, 38 invocam python3, 1 na
allowlist". **Sem achado.**

---

## 6. Regeneração de `scripts/trackfw-attention-signal.sh` — **byte-idêntica aos três literais**

Comparação **byte a byte**, e contra a **transformação do próprio escritor** — não contra o literal
cru — porque é essa a forma que chega ao disco do adopter e que
`scaffold_doctor` compara para acusar adulteração:

```
Go     scaffold.go:833   os.WriteFile([]byte(attentionSignalScript))     -> 1583 == 1583  IDÊNTICO
Node   hooks.js:1496     fs.writeFileSync(SIGNAL_SCRIPT, 'utf8')         -> 1583 == 1583  IDÊNTICO
Python init_gen.py:1857  f.write(_ATTENTION_SIGNAL_SH.lstrip('\n'))      -> 1583 == 1583  IDÊNTICO
```

A comparação Node foi feita carregando o **módulo real** (`require('./npm/src/generators/hooks.js')`
→ `SIGNAL_SCRIPT`) e a Python importando `trackfw.generators.init_gen`, não re-extraindo o literal
por regex — assim a asserção cobre também eventual escape de template/`r"""`. Isso importa: um
descasamento de newline aqui não seria cosmético, faria
`pypi/trackfw/integrations/scaffold_doctor.py:459` (e o equivalente Go
`internal/generators/scaffold_doctor.go:205`) acusar **adulteração falsa** no projeto de quem adota.

E `git diff origin/main...HEAD -- scripts/trackfw-attention-signal.sh` mostra **exatamente as 2
linhas** do ML-1A, nada mais. O ML-2A **resolveu** o achado 1 da entrada anterior do
`agents-working-context.md` (cópia obsoleta). O que **não** foi resolvido é a ausência de guarda —
ver §8.

---

## Achados

### 🟠 S3 — A justificativa do gate para aceitar `utf-8:<handler>` é falsa, e duas formas aceitas reintroduzem o defeito
**Arquivo:** `scripts/check-output-encoding-declared.sh:45-49` (comentário), `:141-146` (`DECL_RE`),
`:283-287` (`PREFIX_RE`) — o sufixo opcional `(?::[A-Za-z0-9_]+)?` em ambas as regex.
**Classificação:** não bloqueia o merge → **REQ de acompanhamento**.

**Mecanismo.** O comentário justifica a aceitação do handler assim:

> `export PYTHONIOENCODING=utf-8:replace` … Aceito porque, **com encoding utf-8, nenhum str do
> Python e inencodavel**: o handler nunca dispara e nao ha dado escondido.

O universal é falso. `json.load` **preserva** surrogates isolados vindos de escapes `\udXXX` (JSON
permite; `json` não valida pareamento), e surrogates isolados **são** inencodáveis em UTF-8. O
handler, portanto, dispara — e o que ele faz depende de qual handler é.

**Prova de execução — direção que quebra.** Variantes do hook com handler, todas aceitas pelas duas
regex do gate; payload `{"tool_input":{"question":"x\ud800y"}}` e `"a\udcffb"`:

```
utf-8:surrogatepass    x\ud800y -> b'{"…","message":"x\xed\xa0\x80y",…}'   ← ED A0 80 cru no artefato
utf-8:surrogatepass    a\udcffb -> b'{"…","message":"a\xed\xb3\xbfb",…}'
utf-8:surrogateescape  a\udcffb -> b'{"…","message":"a\xffb",…}'           ← 0xFF cru no artefato
```

**Prova de execução — direção que NÃO dispara (controle).** A forma canônica da árvore, mesmo
payload:

```
utf-8 (canônico)       x\ud800y -> UnicodeEncodeError -> fallback -> message="Agent needs attention"
utf-8:backslashreplace x\ud800y -> message="\\ud800"  (barra escapada pelo sed; inofensivo)
```

Ou seja: o mecanismo dispara nas formas alternativas e **não** dispara na canônica — o teste
discrimina.

**E o gate aceita, hoje.** Reproduzido em árvore-sandbox com os 3 geradores e o próprio gate
reescritos para `utf-8:surrogatepass`:

```
ALVO 2: internal/generators/scaffold.go — 2/2 invocacoes com prefixo.
ALVO 2: npm/src/generators/hooks.js — 2/2 invocacoes com prefixo.
ALVO 2: pypi/trackfw/generators/init_gen.py — 2/2 invocacoes com prefixo.
check-output-encoding-declared: OK      exit=0
```

**Consequência no consumidor — divergência de 3 runtimes, medida.** Alimentei o artefato inválido
aos três handlers de `/api/attention`:

| runtime | arquivo | comportamento |
|---|---|---|
| Go (`internal/serve/api_attention.go:33-38`) | `ED A0 80` / `0xFF` | `json.Unmarshal` **não erra**; substitui por U+FFFD; responde `active:true` com `message:"x���y"` |
| Node (`npm/src/serve/api_attention.js:9-11`) | idem | `readFileSync(…, 'utf8')` → U+FFFD; `active:true` (idêntico ao Go) |
| Python (`pypi/trackfw/serve/api_attention.py:13-16`) | idem | `UnicodeDecodeError` é subclasse de `ValueError` → capturado → **`{"active": False}`** |

O CLI Python **apaga o banner** onde Go e Node o exibem com caracteres de substituição. Não é crash
nem 500 — mas é um sinal de atenção que **desaparece silenciosamente** em um dos três runtimes.
Vale registrar que essa divergência **é alcançável hoje, sem handler nenhum**: é exatamente o que o
braço "antes / cp1252" do §1 produzia (byte `0x97` do em-dash). Ela é **pré-existente** e o ML-1A a
torna muito menos provável; é por isso que ela é ressalva e não bloqueio.

**Remédio (para quem detém o código dos gates — `hefesto-tf`/`artemis-tf`, não eu):**
1. Restringir o sufixo em `DECL_RE` e `PREFIX_RE` a uma allowlist de handlers que **não podem
   emitir byte inválido** — `strict`, `replace`, `backslashreplace`, `xmlcharrefreplace`,
   `namereplace` — e **recusar** `surrogatepass` e `surrogateescape` explicitamente. Uma linha de
   regex: `(?::(?:strict|replace|backslashreplace|xmlcharrefreplace|namereplace))?`.
2. Corrigir o comentário `:45-49`: a afirmação "nenhum str do Python é inencodável em utf-8" é falsa
   para surrogates isolados vindos de `json.load`. Trocar pela razão real (os handlers permitidos são
   os que garantem saída UTF-8 bem-formada).
3. Registrar o vetor `\udXXX` no corpo de teste do gate, com os dois braços acima.

### 🟡 S4 — O gate anti-reintrodução não cobre a cópia versionada que ele próprio descreve
**Arquivo:** `scripts/check-output-encoding-declared.sh:112-117` (`ATTENTION_SOURCES` = só os 3
geradores) vs. `scripts/trackfw-attention-signal.sh`.
**Classificação:** REQ de acompanhamento.

**Mecanismo.** O ALVO 2 assere os 3 literais-fonte. A cópia versionada em `scripts/` — que o ML-2A
acabou de regenerar e que eu confirmei byte-idêntica (§6) — **não está na população**. E nada mais a
compara: `grep -rn "scripts/trackfw-attention-signal.sh" Makefile scripts/ .github/` só encontra
`check-doctor-parity.sh:553,650`, que **cria sua própria cópia em `mktemp -d`** e a corrompe de
propósito — não olha para a do repositório. `internal/generators/scaffold_doctor.go:205` compara
disco-vs-literal **no projeto de quem adota o trackfw**, não no nosso repo durante `make quality`.

É exatamente a condição de órfão registrada em
`vault/notes/copia-versionada-do-attention-signal-esta-obsoleta-e-sem-guarda-2026-09-02.md`: o ML-2A
corrigiu o **estado**, não a **ausência de guarda**. A mesma divergência pode reaparecer no próximo
ML que tocar o literal, e o gate anti-reintrodução — cuja razão de existir é justamente essa —
ficará verde.

**Por que S4 e não S3:** a cópia do repositório não é distribuída como produto (os adopters recebem
o literal, escrito pelo gerador). O dano é auditoria enganosa — um revisor que abrir o arquivo de
nome óbvio pode dar veredito errado, que é precisamente o que a nota de vault previne.

**Remédio:** acrescentar ao ALVO 2 uma asserção de identidade `scripts/trackfw-attention-signal.sh`
== literal extraído de `scaffold.go` (com guarda de vacuidade se a âncora do literal quebrar).

### 🔵 S5 — Observações, sem ação exigida

1. **`internal/generators/scaffold.go:1873`** emite um segundo `python3 -c` (build-check
   `py_compile`) **sem** prefixo. Fora do contrato por decisão explícita (a âncora do ALVO 2 usa
   `json.load(sys.stdin)` para não arrastá-lo). Sem consequência de segurança: não lê stdin não
   confiável e o caminho é **fail-closed** — sob cp1252 um erro de compilação com caractere fora do
   codepage faria o build-check morrer ruidosamente, nunca passar em silêncio.
2. **`npm/src/serve/api_attention.js:10`** faz `{ ...payload, active: true }` e
   `pypi/…/api_attention.py:18-19` devolve o payload inteiro — os dois **repassam ao browser
   qualquer chave** presente no arquivo, enquanto o Go filtra por struct tipada
   (`internal/serve/api_attention.go:12-19`). Não é achado deste diff: provei em §2 que não há como
   injetar chave nova pelo hook, e `app.js` só lê campos conhecidos. Registro porque é uma assimetria
   de rigor entre os 3 CLIs que vira relevante no dia em que outro produtor escrever o arquivo.
3. **Trade-off do mojibake — avaliado, sem consequência de segurança.** Sob console cp1252 os gates
   agora exibem `�` no lugar de crashar (observei isso na própria sandbox: `ALVO 2: … � 2/2`). A
   pergunta que importa é se um gate que antes falhava (fail-closed, bloqueando merge) pode agora
   **passar** com medição errada. Não pode: sob `set -euo pipefail` o crash do `python3` dava
   exit≠0, e o que o ML-1B remove é uma **falha falsa** (o mismatch por transcodificação do em-dash
   documentado na nota de vault). Depois do fix a comparação é byte-correta dos dois lados — a
   precisão aumenta. Não há caminho de fail-open introduzido.
5. **Perda de fidelidade em entrada latin-1 (caso B de §1b).** Se o harness do agente entregar o
   JSON em cp1252/latin-1, o hook antes escrevia a mensagem (corrompida, byte inválido) e agora cai
   no genérico `"Agent needs attention"`. É **fail-safe** e afeta só a fidelidade do texto do
   banner. Não é regressão de segurança: o estado anterior produzia artefato malformado, que é
   justamente o que S3 mostra ser problemático no consumidor Python. Nenhuma ação recomendada — o
   contrato do harness é JSON, e JSON é UTF-8 por especificação (RFC 8259 §8.1).
6. **`scripts/check-shell-posix-portability.sh`** declara `PYTHONIOENCODING` sem invocar `python3`
   (sobra do ML-1B, casou pela palavra em comentário). Inofensivo, o gate não reprova
   sobra-declaração.

---

## O que bloqueia o merge

**Nada.** Recomendo merge.

## O que vira REQ de acompanhamento

| # | Sev | Item | Dono sugerido |
|---|---|---|---|
| S3 | 🟠 | Restringir a allowlist de handlers em `DECL_RE`/`PREFIX_RE` (recusar `surrogatepass`/`surrogateescape`) e corrigir a justificativa falsa em `:45-49` | dono dos gates |
| S4 | 🟡 | Estender o ALVO 2 à cópia versionada `scripts/trackfw-attention-signal.sh` | dono dos gates |
| S5.2 | 🔵 | Alinhar Node/Python ao Go: filtrar campos conhecidos em `/api/attention` (paridade de rigor) | dono do `serve` |

## Falsificação do gate novo — as duas direções, executadas

Em árvore-sandbox espelhada (`scripts/` + os 3 geradores), para não tocar o repositório:

```
A. cópia fiel                                          -> exit=0  OK
B. prefixo removido dos 3 literais (ALVO 2)            -> exit=1  FAIL, nomeia scaffold.go:773-774,
                                                                  hooks.js:147-148, init_gen.py:969-970
C. export trocado para PYTHONIOENCODING=cp1252 (ALVO 1)-> exit=1  FAIL, "valor nao-utf8 nao corrige nada"
D. tudo trocado para utf-8:surrogatepass               -> exit=0  OK   <- achado S3
```

A, B e C confirmam que o gate discrimina como afirma. D é o achado.

---

**Nenhuma operação de git.** Status do ML no roadmap **não** alterado — aguarda auditoria do
arquiteto (`trackfw_architect`).
