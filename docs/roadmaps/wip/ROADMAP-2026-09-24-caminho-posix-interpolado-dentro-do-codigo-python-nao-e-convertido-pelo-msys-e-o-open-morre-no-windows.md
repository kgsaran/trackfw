---
status: wip
date: 2026-09-24
req: "docs/req/REQ-2026-09-24-caminho-posix-interpolado-dentro-do-codigo-python-nao-e-convertido-pelo-msys-e-o-open-morre-no-windows.md"
squad: [hades-tf, ares-tf, artemis-tf]
---

# Roadmap: caminho POSIX interpolado no código Python não é convertido pelo MSYS

> Criado em: 2026-09-24 | Status: wip

REQ: `docs/req/REQ-2026-09-24-caminho-posix-interpolado-dentro-do-codigo-python-nao-e-convertido-pelo-msys-e-o-open-morre-no-windows.md`

## Diagnóstico

O Git Bash converte caminho POSIX em `argv`, **não** dentro de string de código. O Python do Windows
lê `/tmp` como `C:\tmp` e o `open()` morre. Origem: #363 + PR #417 (`dc95ff34`), que corrigiu 5
sítios de 1 arquivo e **declarou** os restantes.

🔴 **Duas medições de partida discordam** — 15/10 (reportante) vs 7 candidatos (arquiteto), por
critérios diferentes. Nenhuma é o veredito; reconciliá-las é entregável da Wave 0.

⚠️ **VM investiga, CI mede.** A reprodução mínima roda em qualquer Git Bash; o número que vira
afirmação sai do CI.

⚠️ **Custo de CPU:** teto de 2 agentes simultâneos, `go test` só do pacote tocado, `make quality`
apenas na barreira do arquiteto.

---

## 🔴 Entrada nova, apontada pelo próprio censo (2026-09-24)

O censo consertado pela REQ-2026-09-23 **apontou um sítio desta REQ no primeiro uso**. Run
`36017761462` em `main`:

```
CHUNK_ABORT rc=1 line=3609 src=…/chunk_0.sh cmd=python3 -c "
```

Rastreado até `scripts/check-gates-falsify.sh:6745` (era `:6493` quando enumerei):

```python
python3 -c "
import json, sys
with open('$ROOT_DIR/npm/package.json') as f:
```

**Isto muda duas coisas para a Wave 0:**

1. 🔴 **Há um sítio com efeito medido em produção**, não só por varredura estática — ele **mata o
   `chunk_0` inteiro** no Windows e leva 4 rótulos junto. Comece por ele.
2. O sítio roda em POSIX (`rc=0` local, medido) e morre no Windows — o que **confirma o mecanismo**
   desta REQ sem depender de nova reprodução.

⚠️ **E ele muda a prioridade dentro do (a):** um sítio que derruba um chunk do censo tem
consequência maior que um que só reprova o próprio gate. A enumeração deve **registrar essa
distinção**, não só o veredito (a)/(b)/(c).

---

## Wave 0 — Enumeração e reconciliação (1 ML, bloqueia tudo)

### ML-0A — A enumeração real, e por que os dois números de partida divergem
**Owner:** `hades-tf`
**Status:** ✅ Concluído — auditado em 2026-09-24 · **6 sítios (a)** em 5 blocos, 3 arquivos
**Entregue:** `docs/seguranca/2026-09-24-caminhos-interpolados-no-codigo-python.md`
**Arquivos afetados:** nenhum de produto

**Ações:**
1. Enumere pelo critério **"interpola caminho dentro do texto do programa Python E usa esse caminho
   para abrir/ler/escrever arquivo"**. Classifique **(a)** defeito · **(b)** correto · **(c)** o `$`
   não é caminho.
2. 🔴 **Reconcilie os dois números de partida** — 15/10 do #417 e 7 do arquiteto. Um dos dois (ou os
   dois) usou critério mais largo ou mais estreito; diga qual e por quê. Não escolha o maior por
   precaução nem o menor por conveniência.
3. Threat model: algum sítio (a) está em gate de **segurança**? Se sim, a garantia fica sem prova no
   Windows — nomeie qual.
4. Frase de fechamento: *"corrijo esta causa, exatamente estes sítios fecham, e nenhum outro."*

**Critérios de aceite:**
- [x] Tabela com consequência medida por sítio:

| # | sítio | consequência |
|---|---|---|
| a1 | `check-gates-falsify.sh:6745` | 🔴 aborta o `chunk_0` — **remove medição** |
| a2/a3 | `check-serve-api-file-security.sh:87,92` | nenhuma no CI (gate só em `ubuntu-latest`) |
| a4 | `check-update-parity.sh:354` | **3 dos 11 FAIL** do run do censo |
| a5/a6 | `check-update-parity.sh:379,408` | latentes — o `set -e` mata em a4 antes |

      🔴 **Um (a) está em gate de segurança** (`check-serve-api-file-security`), e o risco é **menor
      do que eu supus**: no Windows ele **reprova alto** (`fail "AC6 … resultado inesperado"`), não
      silencia. O residual real é **meta-prova** — a prova de não-vacuidade do AC6 não roda no
      Windows. E a frase de fechamento nomeia os 6 sítios.
      censo? reprova só o próprio gate? nenhuma?) — o `:6745` é o caso com efeito já observado
- [x] Critério aplicável por terceiro; correspondência `chunk_0.sh:3609` ↔ `:6745` verificada
      **byte a byte** regerando os chunks
- [x] **Reconciliado em 6, e nenhum dos dois errou — contavam objetos diferentes.** O #417 foi
      **largo no predicado** (casava `open(` e `$` no mesmo *arquivo*) e **estreito na unidade**
      (1 por arquivo, onde há 3 e 2). A minha exigia `$` e `open(` na **mesma linha** — correta
      nesta árvore, frágil como piso
- [x] 🔴 Nenhuma implementação — `git status` confirmou 3 arquivos, todos de medição
- [x] 🔴 `make quality` não rodado

**Auditoria do arquiteto (medida por mim):**

| afirmação | como confirmei |
|---|---|
| 🔴 `check-doctor-parity.sh` **não existe** | `ls` → *No such file or directory*; deletado em `2eae0a44` |
| o precedente vivo passa por `argv` | `check-thirdparty-parity.sh:167` → `json.load(open(sys.argv[1]))` |
| os 6 sítios interpolam caminho | li as 6 linhas: `open('$ROOT_DIR/…')`, `open('$GO_API_FILE')`, `open('$VULN_GO')`, `open('$manifest')` ×3 |

🔴 **Ele corrigiu dois erros meus, e o segundo é da família que a minha memória já registra:**
*artefato de julho/agosto cita caminho que não existe mais na v8 — confirme antes de copiar.*
Copiei o precedente do issue #363 sem `ls`.

**O primeiro:** afirmei no handoff que o `:6745` leva **4 rótulos** junto. São **1**
(`integration-assets/direction-b-shim-absent`). O "4" era o total de ausentes **do run inteiro** —
grandeza diferente. E o shard 0 **não** caiu em `LOG SEM VEREDITO`: emitiu `OK=50 FAIL=1`.

**Ele manteve a1 em primeiro e trocou a justificativa** — não é volume (a4 produz o triplo), é
**natureza**: a1 é o único que **remove medição** em vez de produzir vermelho. Leitura certa,
logo depois da REQ que fechamos.

**Dois achados que nenhuma enumeração previa:**
1. Um sítio da #363 **fechou por deleção**, não por correção.
2. 🔴 **`internal/generators` NÃO emite o padrão** — `pathlib.Path('.')` literal ou
   `json.load(sys.stdin)`. **O defeito não chega ao consumidor**, o que reduz o alcance da REQ e
   precisava ser medido, não presumido.

E o fechamento não foi por forma: extraiu **as 39 linhas** com `$` em corpo Python fora de `.md`,
sem filtro de I/O, e **leu uma a uma**. Universo enumerado, não amostrado — depois de três
enumerações que se revelaram limite inferior nesta campanha.

---

## Wave 1 — Correção e gate (2 MLs em paralelo, arquivos disjuntos)
> Dependências: Wave 0 auditada.

### ML-1A — Sítios (a) passam o caminho por `argv`
**Owner:** `ares-tf`
**Status:** ✅ Concluído — auditado em 2026-09-24

🔴 **CORREÇÃO (achado do ML-0A):** o roadmap citava `_normalize_version_in_file` do
`check-doctor-parity.sh` como precedente. **Esse arquivo não existe** — deletado em `2eae0a44`
(v8.0.0, #365), oito dias antes do #417. Copiei do issue **#363** sem conferir na árvore
(`ls` → *No such file or directory*).

**O precedente vivo é `check-thirdparty-parity.sh:167`**, que passa por `sys.argv[1]`.

**Ordem de despacho:** a1 → a4/a5/a6 → a2/a3, **os três grupos no mesmo PR** (Regra Dura: a2/a3 têm
consequência **zero** hoje, e é por isso que alguém vai querer deixá-los de fora):

```
check-gates-falsify.sh:6745
check-update-parity.sh:354, :379, :408
check-serve-api-file-security.sh:87, :92
```

**Critérios de aceite:**
- [x] Os 6 por `argv` — **auditei**: 0 caminhos interpolados restantes, 0 `cygpath`/`os.environ` no
      diff (+9/−8). Fixtures idênticos por `cmp`, com guarda de não-vacuidade em cada comparação
- [x] 🔴 E o rótulo **voltou a existir**: `OK [falsify/integration-assets/direction-b-shim-absent]`
      — o a1 voltou a **produzir asserção**, não só deixou de abortar. Na VM: `FileNotFoundError`
      POSIX-MSYS nos 3 arquivos antes, rc=0 depois
- [x] 🔴 Nenhum teste commitado pelo ML-1A (6 frases por medição); 9 frases por braço no ML-1B
- [x] 🔴 `make quality` não rodado pelos executores

### ML-1B — Gate anti-reintrodução
**Owner:** `artemis-tf`
**Status:** ✅ Concluído — auditado em 2026-09-24
**Entregue:** `scripts/check-interpolated-path-in-python.sh` · Cenário 200, 9 braços

**Critérios de aceite:**
- [x] **Auditei nas duas direções**: corpus pré-fix (`git archive HEAD`) → **rc=1, exatamente os 6
      sítios**, linha a linha iguais à tabela do ML-0A, **e nenhum outro**; árvore pós-fix → **rc=0**
- [x] Os dois não-flag saem como `OK [literal/…]`: **0 FAILs** para `check-validate-rule-pins.sh:371`
      (heredoc **citado** — a linha que o #417 consertou) e `check-serve-browser-security.sh:97`
      (URL, cujo vetor **depende** da não-conversão). Poupados por razões **independentes**
- [x] Declaradas; e o residual 3 do ML-0A foi **fechado, não declarado** — o corpus é `git ls-files`,
      não varredura de árvore, então diretório novo nasce coberto
- [x] **Dois** pisos (corpos e expansíveis), calibrados pelo **pior dos dois** cenários. Auditei:
      `MIN_BODIES=9999` → rc=1. 🔴 O segundo existe porque o primeiro **sozinho não pega**
      classificador de citação quebrado num corpus grande — medido: 65 corpos / 5 expansíveis
- [x] 🔴 9 frases, uma por braço do Cenário 200
- [x] 🔴 `make quality` não rodado pelo executor — barreira do arquiteto: **RC=0 · 1275 `^OK ` · 0 `: FALHA`**

---

## Wave 2 — Prova no Windows (1 ML)
> Dependências: Wave 1 mergeada.

### ML-2W — Falsificação exercitada no Windows
**Owner:** `ares-tf`
**Status:** ✅ Concluído — auditado em 2026-09-24 · VM Windows 11, dois braços

**Critérios de aceite:**
- [x] **Exercitada na VM, nos dois braços** (`c79f260` × `dc9f733` — e **não** contra a `main`, que
      diverge em 10 arquivos de `scripts/`). Gate do ML-1B: **rc=1 com exatamente os 6** antes,
      **rc=0 com os dois pisos disparados** depois. O rótulo `integration-assets/direction-b-shim-absent`
      **voltou** no Windows. E os 3 rótulos novos do `check-update-parity` (`:354`/`:379`/`:408`) são a
      **primeira prova direta de que a5/a6 executam** — o ML-0A só os derivava como latentes
- [x] 🔴 **A #363 FECHA pela causa declarada**, medido: os dois sítios que ela nomeia são
      `check-doctor-parity.sh:594` (fechou por **deleção**, v8.0.0) e `check-validate-rule-pins.sh`
      (corrigido pelo **#417**). Gate inteiro na VM: **pins 1–6 `OK`, zero `FileNotFoundError`**.

      **O que sobra é outro mecanismo, medido:** `pin7-noexec` reprova porque em NTFS montado
      `noacl` o `chmod 644` **não tira o bit** (`ls -l` → `-rwxr-xr-x`, `stat %a` → `755`,
      `[ -x ]` verdadeiro). A fixture *"presente e não executável"* é **inconstruível** nesse
      Windows. É **permissão, não caminho** — fora desta REQ.

      ⚠️ Duas ressalvas para a nota de fechamento: fechar **não** significa gate verde no Windows; e
      o cenário do bit em NTFS **não tem issue** hoje (nem #307 nem #364 o cobrem)

## Barreira final

Revisão `hefesto-tf` e `hades-tf`, auditoria do arquiteto, `trackfw barrier`, CI verde.

---

## Wave 2-bis — o defeito que a correção desmascarou (1 ML)

🔴 **A Wave 2 removeu o `FileNotFoundError` e, com ele, a cortina que escondia três defeitos de
Windows.** Nenhum é regressão. Um deles é meu de decidir, e decidi.

### ML-2A — O overlay do `go test` leva caminho POSIX e é ignorado EM SILÊNCIO
**Owner:** `ares-tf`
**Status:** ✅ Concluído — auditado em 2026-09-24

`scripts/check-serve-api-file-security.sh:96` monta o overlay com `printf` de variáveis do shell —
caminhos **POSIX-MSYS** no Git Bash:

```bash
printf '{"Replace": {"%s": "%s"}}\n' "$GO_API_FILE" "$VULN_GO" > "$OVERLAY_JSON"
```

**A/B na VM, mesma máquina, mesmo minuto, única variável a grafia da chave:**

| chave do overlay | resultado |
|---|---|
| `/c/Users/Lab/trackfw/internal/serve/api_file.go` | `ok … 0.038s` — **overlay IGNORADO, sem erro** |
| `C:/Users/Lab/trackfw/internal/serve/api_file.go` | `FAIL … esperado 403, obteve 200; corpo vazou segredo` |

🔴 **`go test -overlay` com chave que não casa nenhum arquivo do build NÃO é erro.** O teste roda
contra o fonte correto, passa, e o AC6 conclui `FAIL AC6 Go falsificação: teste passou na versão
vulnerável`. O Python escreveu o arquivo vulnerável certo nas duas execuções — a diferença é só a
grafia da chave.

**Decisão minha: MESMA CAUSA, ML nesta REQ.** O teste de fechamento literal (*"corrijo a
interpolação no código Python e este sítio fecha"*) diz **não** — a remediação difere. Mas a causa
**raiz** é a mesma fronteira: **caminho POSIX atravessando para um binário Windows nativo sem
conversão**; a interpolação no código Python é **uma forma**, o JSON de overlay é **outra**. E o
CLAUDE.md nomeia *"é superfície diferente"* como exatamente o que **não** justifica REQ nova.

Pesou também o que está em jogo: o braço de segurança **vazou segredo** quando a chave casou. Fechar
esta REQ com `:96` intacto — **quatro linhas abaixo** dos sítios que acabei de corrigir, no mesmo
bloco — é correção pela metade, e o PR mergeado fecharia a janela de atenção.

⚠️ **`ubuntu-latest` NÃO é afetado** — lá POSIX é a grafia nativa e o braço passa (run
`36023336663`). Isto **não** é "o gate de segurança está quebrado"; é "a prova de não-vacuidade dele
não roda no Windows".

**Ações:**
1. Corrija `:96`. O precedente imune está na árvore: `quality.yml:1172` escreve o JSON **pelo
   Python** (`json.dumps({'Replace': {src: inject}})`), que recebe os caminhos já resolvidos.
   `cygpath -m` é a alternativa; escolha e **escreva a razão**.
2. 🔴 **Falsifique nas duas direções na VM:** com a correção, o braço vulnerável precisa **FALHAR**
   (é o que prova não-vacuidade); sem ela, passa em silêncio.
3. **Varra a família:** outros `-overlay`, e caminho shell entregue a binário Go/nativo por arquivo
   de configuração em vez de `argv`. Veredito por sítio.
4. 🔴 **O gate do ML-1B não vê este sítio** — ele varre corpo de programa Python; aqui é `printf`
   para dentro de JSON. Diga se deve ver. Se sim, é ML próprio, não remendo aqui.

**Critérios de aceite:**
- [x] O `printf` sumiu: o overlay passa a ser escrito pelo **próprio Python**, com `json.dumps`
      sobre `os.path.abspath` — a forma de `quality.yml:1172`. **Auditei: 0 ramos condicionais por
      plataforma** (as 2 menções a `cygpath` no diff são comentários explicando por que **não** foi
      usado)
- [x] A/B na VM, mesmo `mktemp`, mesmo arquivo vulnerável: **A (json.dumps)** → `FAIL … 403→200,
      corpo vazou segredo`, **rc=1** ← o resultado desejado; **B (printf)** → `ok 0.043s`, overlay
      ignorado. Gate inteiro: antes `1 falhou`, depois **`5 ok, 0 falhou`**, com `EXPECTED_PASS=5`
      inalterado
- [x] Varrida **pela forma**: `:96` era o **único** sítio no repositório. Três buckets, com razões
      **diferentes** por família — não uma razão só
- [x] Veredito escrito, e **decisão minha abaixo**
- [x] Nenhum teste Go novo; uma frase para o artefato novo (`assert count >= 1`)
- [x] 🔴 `make quality` não rodado pelo executor

**Auditoria do arquiteto (medida por mim):**

| afirmação | como confirmei |
|---|---|
| `printf` do overlay eliminado | `grep -c 'printf .*Replace'` → **0** |
| sem ramo por plataforma | as 2 menções a `cygpath` no diff são **comentários** justificando a não-escolha |
| gate passa em POSIX | `rc=0`, `5 ok, 0 falhou` |

🔴 **Dois achados do executor que valem mais que a correção em si:**

1. **O MSYS converte também o AMBIENTE, não só `argv`.** Eu e ele presumíamos o contrário. Medido:
   `GOCACHE="/tmp/…/gc" go env GOCACHE` → `C:/Users/Lab/AppData/Local/Temp/…/gc`. Isso **isenta por
   medição a maior população suspeita do repositório** (≈8 gates com `GOCACHE=`/`GOPATH=`), que de
   outro modo viraria trabalho. Regra de bolso corrigida na nota de vault.
2. **Um segundo canal de vacuidade, quatro linhas acima, fechado junto:** o `src.replace(needle, …)`
   era **no-op silencioso** se o needle mudasse — o "vulnerável" sairia byte a byte igual ao correto,
   o overlay aplicaria, o teste passaria, e o gate imprimiria **a mesma linha enganosa** por outro
   mecanismo. Entrou `assert count >= 1`. Mesmo sintoma, mecanismo diferente, mesmo bloco.

⚠️ **Consequência declarada, não escondida:** sob `set -euo pipefail` esse assert **aborta o gate**
em vez de incrementar `FAIL` — o resumo `N ok, M falhou` não chega a imprimir. É ruidoso, logo
aceitável; fica dito para não ser lido como defeito depois.

⚠️ **Ausência declarada:** `ubuntu-latest` **não foi medido** — é inferido de a correção não ter ramo
por plataforma e de o braço macOS não ter mudado (saída byte a byte idêntica). O CI do PR mede.

---

### Decisão minha: **NÃO criar** o gate da forma "caminho cru em formato estruturado"

O executor recomendou pesar o custo e **não implementou** — correto. Com `:96` corrigido, a
população restante são os **dois sítios já imunes por construção** do `quality.yml`.

🔴 **O gate nasceria vazio.** Um gate que examina zero e reporta sucesso é **exatamente o defeito que
esta campanha inteira combate** — ele só existiria sustentado por fixture sintética permanente, sem
sítio real para vigiar. Criar isso para "ficar protegido" é encenação de cobertura.

**O que fica no lugar:** a forma correta — *escrever o arquivo estruturado pelo próprio consumidor,
com serializador* — está registrada na nota de vault com o precedente (`quality.yml:1172`) e com as
duas razões (grafia **e** escape).

⚠️ **A premissa que muda esta decisão:** se aparecer um **segundo** sítio real da forma, o gate passa
a ter população e se justifica. Quem reabrir deve medir isso primeiro, não herdar a decisão.

### Os outros dois, classificados como OUTRA causa — com a medição

| achado | mecanismo | veredito |
|---|---|---|
| `ln: failed to create symbolic link '…/s9-proj/.venv/bin/python'` | venv no Windows usa `Scripts/python.exe`, não symlink em `bin/` | **outra** |
| `trackfw barrier` sai **2** onde o cenário espera **1** | código de saída do `barrier`; nenhum caminho envolvido | **outra** |

Nenhum é coberto por #307 ou #364. Ficam **registrados aqui**, não empurrados para REQ nova sem
medição — e não são regressão: estavam atrás do abort.

⚠️ **Ausência declarada, e honesta:** o `check-gates-falsify.sh` **não roda inteiro nesta VM** —
morre no Cenário 19 (`barrier/wave-label`), **~4960 linhas antes** do a1. O que rodou no lugar foi um
**mini-chunk** extraído **byte a byte** do chunk gerado (preâmbulo + segmento do a1), com
`TRACKFW_ROOT_DIR` como o driver de CI faz. É materialmente mais forte que o braço do ML-1A, e
**não** é "gate inteiro" — as duas coisas ditas.

