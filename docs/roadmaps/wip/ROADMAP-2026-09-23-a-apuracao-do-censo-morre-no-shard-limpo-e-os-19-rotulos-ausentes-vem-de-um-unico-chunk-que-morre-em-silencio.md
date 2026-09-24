---
status: wip
date: 2026-09-23
req: "docs/req/REQ-2026-09-23-a-apuracao-do-censo-morre-no-shard-limpo-e-os-19-rotulos-ausentes-vem-de-um-unico-chunk-que-morre-em-silencio.md"
squad: [hades-tf, ares-tf, artemis-tf]
---

# Roadmap: a apuração do censo morre no shard limpo, e os 19 ausentes vêm de um chunk só

> Criado em: 2026-09-23 | Status: wip
REQ: `docs/req/REQ-2026-09-23-a-apuracao-do-censo-morre-no-shard-limpo-e-os-19-rotulos-ausentes-vem-de-um-unico-chunk-que-morre-em-silencio.md`

## Diagnóstico

Duas causas, **ambas do instrumento**, nenhuma delas é o que o relatório do censo diz:

| | causa | evidência | população |
|---|---|---|---|
| **A** | `$(grep -ac … \|\| echo 0)` captura `$'0\n0'` no shard **sem falha** → aritmética quebra → `2/8` | replay dos 8 artefatos: hoje `SHARDS_FOUND=2`, corrigido `8/8 · OK=225 · FAIL=9` | 4 sítios, 1 arquivo |
| **B** | shard 1 morre **em silêncio** (sem `CHUNK_COMPLETE` e sem `N cenário(s) reprovaram`) | **19/19** dos rótulos ausentes são do shard 1; `setup-s62*` não aparece em log nenhum | 1 chunk |

⚠️ **VM investiga, CI mede.** O replay da causa A roda em qualquer máquina, contra os artefatos
reais do run `35872779844` — não espere runner de Windows para falsificar A. Só o AC6 (linha de
base) exige CI.

⚠️ **Custo de CPU:** `TRACKFW_FALSIFY_JOBS=4` na barreira local; teto de **2 agentes** simultâneos
quando cada um compila Go; `go test ./internal/<pacote>/`, nunca `./...` —
`vault/notes/carga-de-cpu-vem-da-suite-de-falsificacao-vezes-agentes-paralelos-2026-09-22.md`.

---

## Wave 0 — Threat model e medição (1 ML, bloqueia tudo)
> Dependências: nenhuma. **Nenhuma wave de implementação é despachada antes desta ser auditada.**

### ML-0A — Mecanismo da causa B, enumeração da causa A, e o que o chunk morto deixa sem prova
**Owner:** `hades-tf`
**Status:** ✅ Concluído — auditado pelo arquiteto em 2026-09-23
**Entregue:** `docs/seguranca/2026-09-23-censo-chunk-morto.md`
**Arquivos afetados:** nenhum de produto — entrega `docs/seguranca/2026-09-23-censo-chunk-morto.md`

**Ações:**
1. **Causa B — por que o chunk 1 morre.** Reproduza localmente: rode o chunk 1 com
   `TRACKFW_FALSIFY_ENUMERATE=1` e o mesmo particionamento (`scripts/gen-falsify-chunks.py`,
   `run-gates-falsify-shard.sh`). A última linha viva no artefato é
   `OK   [falsify/barrier/blocked-not-detected]`. Identifique o comando seguinte e por que ele mata
   o shard **sem** acionar nem o contador de falha nem o sentinela.
   🔴 Se não for identificável, **escreva o que foi eliminado**. "Não sei ainda" é resultado válido;
   hipótese apresentada como causa, não.
2. **Causa A — confirme ou refute a enumeração** da REQ (4 defeituosos / 3 corretos / 2 fora) pelo
   critério *"o comando já emite saída no caminho de falha"*. Varra `.github/workflows/`, `scripts/`
   e `Makefile`. Se achar sítio que a forma `grep -rn '|| echo'` não pega (ex.: `|| printf`,
   `|| true; echo`), **a enumeração da REQ estava estreita** — registre.
3. **Threat model.** Os 19 rótulos não exercitados incluem **12 controles de segurança**. Para cada
   um, escreva **qual garantia fica sem prova no Windows** — e se a garantia tem cobertura
   equivalente por outro caminho (teste Go, outro cenário, outro job). O que **não** tiver é o risco
   real desta REQ.
4. **Teste de fechamento por causa:** escreva, para A e para B, a frase
   *"corrijo esta causa, exatamente estes rótulos/efeitos fecham, e nenhum outro"*.

**Critérios de aceite:**
- [x] Mecanismo da causa B escrito **com a medição** — e **desdobrado em duas**: a camada de
      silêncio está identificada; a falha do `git` **não**, com as eliminações escritas
- [x] Enumeração **refutada em duas frentes** — população 12 (não 9) e um (c) que era (b); os 4
      sítios (a) sobreviveram
- [x] Tabela garantia-sem-prova — **são 15, não 12** (o `12` da REQ estava subcontado): 3 cobertos,
      1 parcial, **11 SEM PROVA**, por leitura do corpo dos testes Go, nunca por nome
- [x] Frases de fechamento — **3 escritas, 1 recusada**: B2 (falha do `git`) não tem causa
      identificada, então nenhuma frase foi escrita para ela. Recusa correta
- [x] 🔴 Nenhuma linha de implementação — `git status` confirmou 1 arquivo novo, o declarado
- [x] 🔴 `make quality` não rodado

**Comandos de validação:** `trackfw barrier <roadmap> --wave 0`

**Auditoria do arquiteto (verificada na árvore, não no relatório):**

| afirmação do ML-0A | como confirmei |
|---|---|
| `HDR_PAT` não casa o cabeçalho do Cenário 18 | reimplementei a regex contra o arquivo: **75** cabeçalhos `# Cenário <n>`, **17** não casam, e o `:1478` é o único cenário real entre eles |
| `extract_expected_labels` só colhe rótulo de `assert_*` | lido em `gen-falsify-chunks.py:314-336`; `ECHO_SIGNAL_PAT` aparece **só** na linha 134, para classificar bloco |
| `:1559` e `:1583` são `git … add -A 2>/dev/null` | `grep -n 'add -A'` confirma os dois |
| nenhuma implementação | `git status --porcelain` = 1 arquivo novo, o declarado |

⚠️ **Divergência de contagem registrada, não resolvida:** medi **99** rótulos literais emitidos por
`echo "OK/FAIL [falsify/…]"`; o ML-0A relata **69** após descontar `setup*` e globs. Os métodos
diferem; a conclusão estrutural (existem rótulos por `echo` que a guarda **não** exige) está provada
independentemente pelo código de `extract_expected_labels`. **O número exato é medido no ML-2C**, com
o comando colado — não herdado de nenhum dos dois.

🔴 **Achado B3 entra nesta REQ, não numa nova.** É a mesma causa — cegueira da guarda de conjunto — e
a Regra Dura de Causa Raiz proíbe empurrá-lo para a fila.


---

## Wave 1 — Causa A: a captura, e o gate que impede a volta (2 MLs em paralelo)
> Dependências: **Wave 0 auditada.** Os dois MLs tocam arquivos disjuntos.

### ML-1A — A forma de captura, nos sítios que a Wave 0 confirmar
**Owner:** `ares-tf`
**Status:** ✅ Concluído — auditado em 2026-09-23
**Arquivos afetados:** `.github/workflows/windows-census.yml` (e outros que o ML-0A confirmar)
⚠️ **NÃO** toque em `scripts/` — é o ML-1B, em paralelo.

**Ações:**
1. Corrija cada sítio **(a)** trocando a forma de captura. A correção medida:
   `CNT=$( { grep -ac '^FAIL' "$LOG" || true; } )` — devolve `0`, uma linha.
   🔴 **Não** acrescente comparação por cima: a contagem cruzada com `awk` já existe e **foi
   derrotada** por este defeito (`grep=0\n0` vs `awk=0`, divergência inexistente). O que se corrige
   é a captura.
2. **Replay obrigatório, sem runner de Windows.** Baixe os artefatos e rode o laço de apuração nas
   duas formas:
   ```
   gh run download 35872779844 -D /tmp/censo
   # forma atual  -> SHARDS_FOUND=2      (reproduz o 2/8 do CI)
   # forma nova   -> SHARDS_FOUND=8, OK=225, FAIL=9
   ```
3. Se a mensagem `AUSENTE (artefato não baixado)` puder ser emitida quando o artefato **existe**,
   corrija o texto: ela mandou a investigação para o lado errado.

**Critérios de aceite:**
- [x] Os 4 sítios corrigidos para `|| true` — a forma que a Wave 0 mediu como (b), não um terceiro idioma
- [x] Replay: forma atual `SHARDS_FOUND=2`, nova `8 · OK=225 · FAIL=9`; confirmado por `awk` e `python3` independentes
- [x] Nenhuma comparação acrescentada — o `awk` cruzado que existia voltou a **não** acusar discrepância
- [x] 🔴 Frases escritas — e **nenhum teste foi commitado**: o replay de dois braços e a
      injeção de shard ausente são instrumento de medição, ficaram fora da árvore. A frase existe
      para os dois, e a ausência está declarada
- [x] 🔴 `make quality` não rodado pelo executor — barreira do arquiteto: **RC=0, 1021 `^OK `, 0 `: FALHA`**

### ML-1B — Gate anti-reintrodução, falsificável, com guarda de não-vacuidade
**Owner:** `artemis-tf`
**Status:** ✅ Concluído — auditado em 2026-09-23
**Entregue:** `scripts/check-emitting-capture-fallback.sh` · Cenário 198 (11 braços)
**Arquivos afetados:** `scripts/check-<nome>.sh` (novo), `Makefile`, `scripts/check-gates-falsify.sh`
⚠️ **NÃO** toque em `.github/workflows/` — é o ML-1A, em paralelo.

**Ações:**
1. Gate que reprova `$(cmd … || echo N)` quando `cmd` **já emite no caminho de falha**. O
   discriminante mínimo é `grep -c`/`grep -ac`; **declare no cabeçalho as formas que NÃO cobre** —
   "não medido" não é aceitável, o limite é medido e documentado.
2. **Guarda de não-vacuidade:** piso de sítios examinados, contando só o que o discriminante **já
   enxerga**. Cole o comando que produziu o número.
3. **Prove injetando**, forma a forma: cada forma coberta reprova de verdade.
4. Braços de falsificação em `check-gates-falsify.sh`, com **rótulos literais**, colhidos pela
   guarda de conjunto.

**Critérios de aceite:**
- [x] Gate reprova as 6 formas cobertas, por injeção; **auditei contra `HEAD` pinado: RC=1, exatamente os 4 sítios**
- [x] Árvore atual RC=0; vacuidade auditada (`MIN_CANDIDATES=999` → RC=1, 87 candidatos, piso 50)
- [x] 7 formas não cobertas declaradas, **cada uma com o comando e a contagem de sítios reais** (0 em quase todas)
- [x] Cenário 198, 11 braços, rótulos literais colhidos pela guarda de conjunto
- [x] 🔴 **11 frases, uma por braço** — inclusive as que afirmam cobertura de scanner (`workflow-yml`)
      e não classe de defeito, e a que encoda o defeito da REQ anterior (`vacuous-scan`)
- [x] 🔴 `make quality` não rodado pelo executor — barreira do arquiteto: **RC=0, 1021 `^OK `, 0 `: FALHA`**


**Auditoria do arquiteto (medida por mim, não lida do relatório):**

| afirmação | como confirmei |
|---|---|
| gate reprova a forma velha | contra `HEAD` pinado: **RC=1**, 4 `FAIL`, exatamente `:485,:486,:564,:565` |
| gate aprova a forma nova | árvore atual: **RC=0** |
| guarda de não-vacuidade funciona | `MIN_CANDIDATES=999` → **RC=1**, com diagnóstico próprio, 87 candidatos |
| não reprova os (b) legítimos | nenhuma violação em `wc -l`/`jq` |
| erro aritmético não derruba o script | reprodução própria em `bash 5.3`: o laço morre, o script segue, **`rc=0`** — e o job real ficou `apuracao → success` |

🔴 **A varredura que eu escrevi na REQ era mais estreita do que eu afirmei.** Medido:
`$(grep --count x f \|\| echo 0)` **não casa** o ERE `grep +-[a-z]*c[a-z]* `; `-ac` casa. A forma
longa ficaria fora das duas enumerações. Quem fecha é o gate, que trata `--count` como contagem.
Corrigido na REQ.

⚠️ **Limitação aceita, não guardada:** `|| true` devolve string **vazia** se o `grep` sair `2`
(arquivo ilegível), e `$((0 + ))` também quebra. Inalcançável aqui — o `[[ -f ]]` a montante barra.
**Não empilhar `${VAR:-0}`**: guarda sobre captura é o padrão que produziu o defeito.

🔴 **Herança para a Wave 3:** `QUEDA = 512 − 9 = 503` cai fora do intervalo `[420,465]`, então o
censo passa a tomar o ramo *"queda fora do esperado"*. É o comportamento **correto** sobre este
corpus truncado — não é regressão, e não deve ser "consertado" alargando o intervalo.

---

## Wave 2 — Causa B e o achado B3 (3 MLs)
> Dependências: **Wave 0 auditada** ✅. ML-2A e ML-2C tocam arquivos distintos e rodam em paralelo.
> ML-2B é do arquiteto e depende do ML-2A estar mergeado.

### ML-2A — O chunk para de morrer em silêncio
**Owner:** `ares-tf`
**Status:** ✅ Concluído — auditado em 2026-09-24
**Arquivos afetados:** `scripts/check-gates-falsify.sh` (região do Cenário 18, ~1478–1603)
⚠️ **NÃO** toque em `scripts/gen-falsify-chunks.py` — é o ML-2C, em paralelo.

**Medido pelo ML-0A, não redescubra:** o chunk 1 morre com `chunk_rc=128` (erro fatal do `git`)
dentro do Cenário 18, num `git … add -A 2>/dev/null` (`:1559` ou `:1583`). O `2>/dev/null` engole o
diagnóstico e o `set -e` mata antes do epílogo — por isso não sai nem `CHUNK_COMPLETE` nem
`N cenário(s) reprovaram`.

**Ações:**
1. Faça o chunk **denunciar a própria morte**: o shard precisa dizer **onde** e **com que rc** parou,
   em vez de terminar mudo. 🔴 A guarda de conjunto hoje só percebe pela **ausência** de rótulo — é
   inferência, não diagnóstico.
2. Nos dois `git … add -A 2>/dev/null`, pare de descartar o stderr no caminho de falha. Descartar
   ruído esperado é legítimo; descartar o **rc 128** não é.
3. **Não adivinhe por que o `git` falhou** — isso é o ML-2B, e o ML-0A recusou escrever a causa por
   falta de medição. Sua entrega é o diagnóstico ficar visível quando acontecer.

**Critérios de aceite:**
- [x] `trap ERR` emite `CHUNK_ABORT rc= line= src= cmd=`; **6 injeções pareadas**, incluindo INJ-3,
      a assinatura exata medida (rc=128 com stderr **vazio**) — a linha de `rc=` é incondicional
- [x] Stderr vai para arquivo: invisível no sucesso, impresso com o rc na falha
- [x] Log da árvore limpa **byte-idêntico** ao pristino; zero `CHUNK_ABORT`, por dois caminhos
- [x] 🔴 **Nenhum teste novo** — a prova é por injeção pareada, e a ausência está declarada; há
      uma frase por injeção. E nenhuma hipótese sobre o `rc=128` foi escrita como causa
- [x] 🔴 `make quality` não rodado pelo executor

### ML-2C — A guarda de conjunto enxerga rótulo emitido por `echo`, e o Cenário 18 volta a ser corte
**Owner:** `artemis-tf`
**Status:** ✅ Concluído — auditado em 2026-09-24
**Arquivos afetados:** `scripts/gen-falsify-chunks.py`
⚠️ **NÃO** toque em `scripts/check-gates-falsify.sh` — é o ML-2A, em paralelo.

**Medido pelo ML-0A e confirmado pelo arquiteto:** `HDR_PAT` (`:81`) exige `# Cenário <n> --|—` e o
cabeçalho do 18 é `# Cenário 18 (AC1/AC2/AC3 — REQ #366) —` → **não é ponto de corte**.
`extract_expected_labels` (`:314-336`) itera **só** `ASSERT_CALL_PAT`; `ECHO_SIGNAL_PAT` existe mas
é usado só para classificar bloco (`:134`). Rótulo emitido por `echo` não vira exigência.

🔴 **A direção que mais importa:** apagar o `echo` e as asserções acima dele **não é detectado por
guarda nenhuma** — gate verde com controle de segurança removido.

**Ações:**
1. Corrija `HDR_PAT` para aceitar parêntese/anotação entre o número e o travessão. **Meça** quantos
   cabeçalhos passam a casar, antes e depois.
2. Faça `extract_expected_labels` colher também `echo "OK|FAIL [falsify/<literal>]"`, com a mesma
   regra de literal vs glob (`$` → prefixo glob).
3. 🔴 **Meça o número de rótulos que passam a ser exigidos, e cole o comando.** Há divergência
   registrada entre duas medições anteriores (99 do arquiteto, 69 do ML-0A) — **não herde nenhuma
   das duas**; meça e explique a diferença de método.
4. Prove nas duas direções: apagar um `echo` de controle passa a **reprovar**; a árvore correta
   continua passando, com guarda de não-vacuidade.

**Critérios de aceite:**
- [x] `HDR_PAT` 59 → 60 — **auditei reimplementando a regex**: entra só a linha 1478, nenhum sai
- [x] 165 → **232** exigidos (+67), conferido por segundo caminho; a divergência 99 vs 69 explicada:
      **nenhum dos dois media "rótulo que vira exigência"**
- [x] Apagar `no-repo-mutation` e `git-branch-guard-dedup/*`: gerador novo RC=1 nomeando o rótulo;
      gerador antigo RC=0 — **cego**
- [x] Direção verde com dado real: os 220 exigidos foram **todos** emitidos no run verde `35987929640`
- [x] 🔴 Nenhum teste commitado (instrumento fora da árvore); frase por medição
- [x] 🔴 `make quality` não rodado pelo executor

### ML-2B — Sonda: por que o `git` sai 128 no Windows ✅ Concluído — 2026-09-24
**Owner:** arquiteto (mediu na VM, não no CI)

🔴 **Causa identificada: MAX_PATH.** Falsificada nas duas direções, no **mesmo** comprimento de
caminho, na VM Windows 11 (Git 2.55.0.windows.3):

```
longpaths=false  rc=128  len_win=66   error: open("internal/roadmapdoc/testdata/corpus/…"):
                                      Filename too long
longpaths=true   rc=0    len_win=66
```

**65 + 1 + 195 = 261 > 260.** O corpus de `internal/roadmapdoc/testdata/` tem caminhos relativos de
**196, 195 e 190** chars — nomes de roadmap usados como fixture.

**Por que ninguém achou antes, e por que a Wave 0 acertou em recusar a causa:**

- 🔴 **A falha é marginal e o nome do `mktemp` decide.** No meu próprio teste, `lp.cW227I` (9 chars)
  **passou** e `sonda-rc128.fEHVaE` (18) **falhou** — mesmo comando, mesmo repositório. A Wave 0
  mediu "240-255, abaixo de 260" e classificou a hipótese como enfraquecida: estava certa para o
  prefixo que ela usou.
- **Não é o MSYS:** o `bash` cria e escreve em caminho de **916 chars** sem reclamar. Quem recusa é
  o `git` nativo, via API Win32 com `MAX_PATH=260`.
- **A "assinatura com stderr vazio" era artefato do próprio código.** O `git` escreve 3 linhas; o
  `2>/dev/null` as apagava. 🔴 **A correção do ML-2A teria revelado a causa sozinha.**

**Eliminado com medição:** `safe.directory` (o `rev-parse` funciona) e **arquitetura** — o `uname`
da VM é `…ARM64 3.6.9-….x86_64 … x86_64 Msys`: o runtime MSYS é **x86_64 emulado**, o mesmo build do
runner.

**Critérios de aceite:**
- [x] A causa do `rc=128` **escrita com a medição**, falsificada nas duas direções
- [x] 🔴 Nenhuma hipótese apresentada como causa — e as duas eliminadas têm a medição que as elimina
- [x] Nota de vault: `o-git-sai-128-no-windows-porque-o-corpus-de-testdata-estoura-max-path-2026-09-24.md`

⚠️ **Medido na VM, não no CI — e isso é deliberado.** *"VM investiga, CI mede"* vale para **contagem**;
o mecanismo é mecanismo. O número que a Wave 3 vai afirmar continua saindo do `windows-census.yml`.

### ML-2D — Emissão de sucesso condicional à checagem ter passado ✅ Concluído — auditado em 2026-09-24
**Owner:** `artemis-tf`

🔴 **A premissa do meu handoff estava errada, e a medição do executor a refutou.** Eu escrevi que os
6 rótulos "passam em silêncio, não há emissão de sucesso para cobrar". **Falso para 5 dos 6:** cada
um tem emissão de sucesso no mesmo braço, sob **outro nome**, e esses rótulos já estavam no
manifesto.

**O defeito real, medido, é maior e mais grave:**

```bash
if [[ <falha> ]]; then echo "FAIL [...]"; falsify_fail_point; fi
falsify_count_success          # <- incondicional
echo "OK   [falsify/...]"      # <- incondicional
```

Sob `TRACKFW_FALSIFY_ENUMERATE=1` — **o modo do censo de Windows** — `falsify_fail_point` faz
`return 0`, e o `OK` do **mesmo braço** saía logo depois do `FAIL`. O comentário do próprio
`falsify_fail_point` (`:265-274`) já descrevia o problema e já o resolvera **para os helpers**; os
blocos inline nunca receberam.

**População: 39 sítios**, não 5 — 30 da forma A (sucesso incondicional após o `fi`) e 9 da forma B
(irmão anterior reprova sem gatear o sucesso). Todos corrigidos; varredura final 0 de cada.

**Critérios de aceite:**
- [x] Lista remedida — **idêntica** à do ML-2C, item a item; nenhuma diferença a registrar
- [x] Correção provada por **injeção de falha** nos 9 pares, com o particionamento conferido igual
- [x] 🔴 Nenhum `OK` decorativo — `vacuity-guard` ficou **declarado SEM prova**, com a razão medida
- [x] Família `setup*` declarada fora, confirmada quebrando o harness do Cenário 69
- [x] A regra do ML-2C não foi invertida
- [x] 🔴 Nenhum teste commitado (instrumento no scratchpad); uma frase por par de injeção

**Auditoria do arquiteto:**

| afirmação | como confirmei |
|---|---|
| o ML-2D não tinha seção no roadmap | **verdade, e o erro é meu** — um `str.replace` sem `assert` falhou em silêncio e eu não conferi |
| o sítio `:5109` mata o chunk | reproduzi: `n=$(grep -oF ausente <<<"$out" \| wc -l)` sob `set -euo pipefail` → **rc=1, a linha seguinte nunca executa** |
| o trap do ML-2A está no bloco do Cenário 18 | `trap … ERR` na linha **1546**; o sítio que mata está na **5109** — outro bloco, outro chunk |

🔴 **Dois limites que a medição impôs, e que o executor não escondeu:**

1. **A guarda de conjunto só discrimina quando o rótulo de `FAIL` difere do de `OK`.** O `.actual` é
   montado com `grep -oE '^(OK|FAIL|PROOF)…'` — **um `FAIL` satisfaz a exigência do rótulo**. Nos 33
   sítios em que os dois nomes coincidem, o ganho é a consistência da apuração `^OK`/`^FAIL` do
   censo, **não** a guarda. Separar `FAIL` de `OK` no `.actual` é mudança de contrato — fica fora
   deste ML, **registrada**, não esquecida.
2. **`vacuity-guard` não pode ser exigido:** o epílogo (`:7490`) está sob
   `if ! declare -f __falsify_timing_mark`, e essa função é injetada em **todo** chunk — o bloco
   **nunca executa em chunk**. Testado: o rótulo entra no manifesto e nunca é emitido → gate
   permanentemente vermelho. Mesma armadilha do ML-2C, por outro caminho.

E o executor registrou uma **previsão própria refutada** (esperava que o fonte antigo fosse cego no
caso do `setup` e ele não era) — é o comportamento que esta casa pede.

---

## Wave 2-bis — O que a auditoria do ML-2D descobriu (2 MLs)
> Dependências: nenhuma. Mesma causa, mesma REQ: chunk que morre sem produzir número.

### ML-2E — O trap em todos os chunks, o `longpaths`, e a família `grep | wc`
**Owner:** `ares-tf`
**Status:** ✅ Concluído — auditado em 2026-09-24

🔴 **Absorve a correção do ML-2B** (mesmo arquivo, mesmo sintoma): `core.longpaths=true` nos `git`
que operam sobre cópia do repositório dentro de `$WORK`. **Todos os sítios**, não o do Cenário 18 —
a distância até os 260 depende do `mktemp` daquele dia.

🔴 **Falha de auditoria minha:** marquei o AC do ML-2A *"morte de chunk passa a emitir sítio e rc"*
como atendido. O ML-2A **declarou** o limite — "do ponto de instalação até o fim do processo" — e eu
li isso como ressalva, sem medir o alcance. O ML-2D mediu: o `trap ERR` está em `:1546`, dentro do
bloco do Cenário 18; **num particionamento de 8 chunks, 7 não têm o trap em lugar nenhum do texto**.
E mediu um caso real: **chunk_0 morreu por `set -e` sem emitir uma linha de `CHUNK_ABORT`** — o
sintoma exato que o ML-2A existe para eliminar.

**Ações:**
1. Leve a instalação do trap para o **prelúdio** — a região que `gen-falsify-chunks.py` copia para
   **todo** chunk. Verifique com o próprio gerador qual é o `prelude_end_line`, não por inspeção
   visual.
2. 🔴 Preserve as duas decisões do ML-2A, que foram medidas: `ERR` e **não** `EXIT` (há
   `trap 'rm -rf "$WORK"' EXIT` em `:34`, que seria substituído); e **sem `set -E`**, porque com
   errtrace toda falha *esperada* de gate escreveria `CHUNK_ABORT` dentro do log do subshell.
3. **Corrija `:5109`** e varra a família: `$(grep … | wc -l)` e afins, onde o `grep` sem casar sai 1
   e o `pipefail` transforma "zero ocorrências" em morte do chunk. **É a mesma raiz da Wave 1** —
   `grep` que sai 1 no caminho de não-casamento — em outra forma. Enumere antes de corrigir.
4. Verifique se o gate do ML-1B (`check-emitting-capture-fallback.sh`) cobre esta forma. Se não
   cobrir, **diga se deve cobrir** — e se a resposta for sim, é ML novo, não remendo aqui.

**Critérios de aceite:**
- [x] Trap no prelúdio: **auditei gerando as partições — N=8 → 8/8, N=16 → 16/16**. Provado matando
      o `chunk_6`, que não contém o Cenário 18
- [x] `git config core.longpaths true` **no repositório da cópia** — cobre os 11 `git` e também os
      gates candidatos que rodam com `cd` na cópia; VM em `len_win=69`: `false`→rc=128, `true`→rc=0
- [x] 🔴 `git diff --name-only | grep -c testdata` → **0**
- [x] 10 substituições com `grep` no arquivo → 6 prosa, **3 defeituosas corrigidas**, 1 isenta
      (roda por `bash -c` sem `pipefail`, e `SHELLOPTS` não é exportado)
- [x] Sequência de rótulos **byte-idêntica ao pristino** em 3 chunks; ⚠️ a guarda de conjunto **em
      execução** ficou devendo — cobrada na barreira do arquiteto
- [x] 🔴 Nenhum teste commitado; **14 frases por medição**
- [x] 🔴 `make quality` não rodado pelo executor

**Auditoria do arquiteto (medida por mim):**

| afirmação | como confirmei |
|---|---|
| trap cobre todos os chunks | gerei as partições: **N=8 → 8/8**, **N=16 → 16/16** com `__falsify_abort_report` |
| 🔴 o `ERR` dispara com `set +e` | reproduzi: `trap … ERR; set +e; false` → **dispara com flags `hBc`** (sem `e`) e o script **segue** |
| a guarda existe | `case "$-" in *e*) ;; *) return 0` no handler, antes do `echo` |
| corpus intocado | `git diff --name-only \| grep -c testdata` → **0** |

🔴 **O achado próprio do executor é o mais valioso deste ML, e não estava no meu handoff.** Ao levar
o trap para o prelúdio, ele mediu o efeito colateral em vez de presumi-lo: o `ERR` do bash **não**
depende do errexit estar ligado, e este arquivo usa `set +e … set -e` em dezenas de blocos para
capturar saída de comando que **deve** falhar. Resultado: **2 `CHUNK_ABORT` falsos** num chunk que
terminou `rc=0, 36 OK, 0 FAIL`. A linha afirmaria *"o shell abortou por `set -e`"* com errexit
desligado — **diagnóstico que mente**, que é exatamente a classe de defeito desta REQ. Ele guardou
com `case "$-"`.

⚠️ **E ele reconciliou uma contradição em vez de escolher um lado:** a cobertura do trap era
**função da partição** — em N=4 e N=8 o Cenário 18 e o sítio do `chunk_0` caem no mesmo chunk; em
N=12/16/60, não. A minha medição (N=8, via ML-2D com N=60) e a dele discordavam por isso. A tese
fica **mais forte** que "7 em 8".

---

### ML-2G — Os 10 sítios da mesma causa fora do `check-gates-falsify.sh`
**Owner:** `ares-tf`
**Status:** ✅ Concluído — auditado em 2026-09-24 · **5 corrigidos · 5 isentos com razão escrita**

O ML-2E varreu o corpus inteiro (`scripts/*.sh` + `.github/workflows/` + `Makefile`): **73 brutos**
→ filtrando `pipefail` ativo, sem guarda e fora de comentário, e descartando 13 da forma correta,
sobram **10 candidatos da mesma causa** — `grep` em posição **não-final** de pipeline dentro de
`$( … )`, onde não casar mata o script:

```
check-agent-namespace-union.sh:632,900,901
check-ci-workflow-pin-parity.sh:192,211
check-manifest-version-gate.sh:113
check-parity-call-site-pins.sh:261
check-serve-address-parity.sh:229
check-serve-api-file-security.sh:76
.github/workflows/check-annotations.yml:80
```

🔴 **Mesma causa, mesma REQ** — não vira REQ nova, que é onde os defeitos medidos se perdem.

**A pergunta a responder por sítio:** *"não casar é resultado válido da medição?"* Se for, falta
`|| true`. Se não for, o script **deve** morrer ali — e isso fica escrito.

**Critérios de aceite:**
- [x] Veredito por sítio. **5/5** — exatamente o que eu pedi que não fosse automático: 5 defeituosos
      e 5 isentos, **3 por razão estrutural e 1 por razão semântica**
- [x] Auditei o diff: **9 ocorrências** de `|| true; }`, **0** de `|| echo 0`
- [x] 🔴 Nenhum teste commitado; **14 frases por medição**
- [x] 🔴 `make quality` não rodado pelo executor

**Auditoria do arquiteto (medida por mim, em `bash 5.3`):**

| isenção | como confirmei |
|---|---|
| substituição em **posição de argumento** descarta o rc | `f "$(echo x \| grep -o ausente \| head -1)"` sob `set -euo pipefail` → `ARG=[]`, **sobreviveu** |
| `v=$( … ) \|\| true` guarda de fora | `rv=$(echo x \| grep -o ausente) \|\| true` → **sobreviveu**, `rv=[]` |
| `grep` **final** mata mesmo sem `pipefail` | `set +o pipefail; v=$(grep -o ausente <<<"x")` → **rc=1**, a linha seguinte não executa |
| o **segundo** `grep` do manifest precisava de guarda | CHANGELOG só com `## [Unreleased]`: antes **rc=1** (morre), depois **rc=0**, `v=[]` |

🔴 **O achado mais importante deste ML é sobre a minha própria enumeração:** o filtro exige `|` no
corpo, então a **forma irmã sem cano** — `v=$(grep … arquivo)`, que mata pelo rc do `grep` **sozinho,
sem depender de `pipefail`** — é invisível às duas listas. **5 sítios vivos**, nomeados. A enumeração
do ML-2E era um **limite inferior**, não a população.

E ele encontrou um defeito no próprio instrumento: o primeiro braço positivo de `:229` deu vazio por
**escape de heredoc**, não pela correção. Conferiu por segundo caminho que o `grep` BSD honra `\S` e
refez. *"O instrumento mentiu; a correção estava certa."*

⚠️ **Erro meu nesta auditoria, registrado:** medi o braço C com `… | head -2; echo $?` e li **rc=0**
— que é o rc do `head`. É a armadilha que este projeto já pagou quatro vezes. Refiz sem cano:
**rc=1**. A conclusão não mudou; o instrumento, sim.

---

### ML-2I — A forma irmã sem cano: `v=$(grep … arquivo)`
**Owner:** `ares-tf`
**Status:** ✅ Concluído — auditado em 2026-09-24 · **5 corrigidos · 7 isentos · 1 falso positivo**

```
check-ci-workflow-pin-parity.sh:230
check-install-version-pin.sh:210,213,246,249
```

Mata pelo rc do `grep` **sozinho** — não depende de `pipefail`, como o braço C mediu. Mesma causa,
mesma REQ.

- [x] 13 sítios (não 5): a forma estava certa, **a família é que estava estreita** — 4 `command -v`,
      3 `find`, 1 falso positivo de substituição multi-linha
- [x] Corrigidos com `{ … || true; }` **mais guarda de não-vacuidade** nos AC5 — ver o achado abaixo;
      7 isentos com razão, **5 estruturais e 2 semânticas**
- [x] Duas direções, e **três** nos AC5 — o terceiro braço falsifica a alternativa que ele não escolheu
- [x] 🔴 Nenhum teste commitado; 9 frases por medição
- [x] 🔴 `make quality` não rodado pelo executor

**Auditoria do arquiteto (medida por mim, em `bash 5.3`):**

| afirmação | como confirmei |
|---|---|
| `local v=$(cmd)` na mesma linha **mascara o rc** | `local v=$(grep -o zzz <<<x)` sob `set -e` → **rc=0**, `ALIVE`; com `local v;` separado → **rc=1** |
| `sed -n` sem casar **sai 0** | `v=$(sed -n 's/^DEST: //p' <<<"nada")` → **rc=0**, `v=[]` |
| a guarda de não-vacuidade reprova **com rótulo** | `FAIL [install-version-pin/ac5-same-asset]: … medicao vacua, nao comparacao` |

🔴 **O achado deste ML é o mais importante da Wave 2-bis, e ele contraria a receita que eu vinha
repetindo.** Nos 4 sítios do `check-install-version-pin.sh` a linha seguinte **compara duas
capturas**:

```bash
URL_BARE=$(grep '^URL: ' <<<"$OUT")
URL_PREFIXED=$(grep '^URL: ' <<<"$OUT")
if [ "$URL_BARE" != "$URL_PREFIXED" ]; then … exit 1; fi
echo "OK   [install-version-pin/ac5-same-asset]"
```

Se o seam parar de emitir `URL: `, as duas ficam **vazias**, comparam **iguais**, e o cenário emite
**`OK`**. Medido nas três variantes:

| variante | não casa |
|---|---|
| antes (sem guarda) | rc=1, **morte muda** |
| 🔴 **só `\|\| true`** | **rc=0 e `OK`** — **aprovação vácua** |
| rc **+** não-vacuidade | rc=1 **com `FAIL` nomeado** |

**`{ … \|\| true; }` isolado ali teria trocado um falso-negativo barulhento por um falso-positivo
silencioso** — pior. Ele falsificou a alternativa em vez de supô-la.

> **Teste de bolso corrigido, que passa a valer para a forma sem cano:** *"o código **depois** da
> atribuição distingue **vazio** de valor legitimamente medido?"* Distingue → guarda de rc basta.
> Não distingue → rc **+** não-vacuidade, ou o sítio fica como está com a razão escrita.

E ele **reportou um erro próprio pego na revisão**: a primeira versão dos artefatos trazia "2
corrigidos / 9 isentos", contradizendo as 5 coordenadas da mesma frase. Aritmética, não medição — e
é exatamente o que a Regra Dura de Reconciliação existe para pegar.

---

### ML-2J — `sed -n` não tem rc para propagar, e o efeito é o mesmo
**Owner:** `ares-tf`
**Status:** ✅ Concluído — auditado em 2026-09-24 · **67 sítios · 4 corrigidos · 63 isentos**

```bash
DEST_BARE=$(sed -n 's/^DEST: //p' <<<"$OUT")     # sed -n sem casar sai 0 — medido
```

Nos **mesmos** cenários AC5, dois `DEST` vazios comparam iguais e o cenário emite `OK`.

**Decisão minha:** entra nesta REQ. Mecanismo diferente (ausência de rc vs. rc não-zero), **sintoma
idêntico** — e fechar a metade `URL` do mesmo `if` deixando a metade `DEST` vácua seria fechar o
roadmap com defeito conhecido no sítio que acabamos de tocar.

- [ ] Guarda de não-vacuidade para `DEST`, no padrão já estabelecido pelo ML-2I (reprova com rótulo)
- [ ] Varra a família `sed -n`/`awk`/`cut` — **comandos que saem 0 mesmo sem produzir saída** — e dê
      veredito por sítio. 🔴 Aqui o rc **nunca** ajuda: o único discriminante é o conteúdo
- [ ] Falsificação em três braços, como no ML-2I
- [ ] 🔴 Uma frase por teste novo, ou ausência declarada
- [ ] 🔴 **NÃO rodar `make quality`**

### ML-2H — Discriminante irmão: pipeline desguarnecido sob `pipefail`
**Owner:** `artemis-tf`
**Status:** ✅ Concluído — auditado em 2026-09-24 · `scripts/check-unguarded-capture-rc.sh` · Cenário 199, 17 braços

**Veredito do ML-2E, que eu aceito:** o gate `check-emitting-capture-fallback.sh` (ML-1B) **não**
cobre esta forma e **não deve ser alargado** — ele exige coexistência de comando emissor **e**
fallback emissor; aqui não há fallback algum. Alargar a regex acusaria todo `$(a | b)` legítimo.
É **discriminante irmão**, gate próprio.

- [ ] Gate que reprova `grep` em posição não-final de pipeline dentro de `$( )` sob `pipefail`, sem
      guarda — provado por injeção, forma a forma
- [ ] Não reprova os legítimos — provado na árvore, incluindo as 13 da forma correta
- [ ] 🔴 **As 4 classes que o gate NÃO pode acusar**, medidas no ML-2G, tratadas explicitamente:
      (1) substituição em posição de argumento · (2) `|| true` depois do fecha-parênteses ·
      (3) corpo em `bash -c`/`sh -c` — **as três sintáticas e decidíveis** — e (4) não-casamento
      pré-excluído por checagem anterior que encerra o script — **semântica, NÃO decidível**: vai
      para o cabeçalho como não coberta, com alegação por sítio. O gate não finge decidir o que não
      decide
- [ ] Cobre também a **forma sem cano** (ML-2I) — senão nasce com sítios fora do alcance no dia 1
- [ ] 🔴 **São 5 classes de isenção, não 4.** O ML-2I acrescentou duas, ambas **sintáticas e
      decidíveis**: arquivo **sem `set -e`** (medido em `check-orphan-gates.sh` e
      `check-raw-read-ban.sh`) e **`local v=$(cmd)` na mesma linha**, que mascara o rc
- [ ] 🔴 **Junte continuações antes de decidir** — um gate linha-a-linha nasce com o falso positivo
      de `quality.yml:1255`, onde o `|| true` está três linhas abaixo do `$(`
- [ ] Formas não cobertas declaradas no cabeçalho
- [ ] Guarda de não-vacuidade com piso e o comando que o produziu

### ML-2F — Decidir o contrato do `.actual`: `FAIL` satisfaz a exigência do rótulo
**Owner:** arquiteto (decisão), depois `artemis-tf`
**Status:** ⬜ Pendente — **bloqueado por decisão**

Medido no ML-2D: a guarda de conjunto aceita um `FAIL` como prova de que o rótulo foi exercitado.
Consequência: quando o braço usa o **mesmo nome** em `OK` e `FAIL`, apagar a prova e deixar a falha
**passa pela guarda**. Em 33 dos 39 sítios é esse o caso.

Separar os dois no `.actual` é mudança de contrato e mexe em consumidores. 🔴 **Não fazer é aceitar
uma cegueira medida**, que é o que esta REQ inteira existe para atacar. A decisão é minha, e ela
precisa da contagem de consumidores afetados antes.


**Auditoria do arquiteto — ML-2J e ML-2H (medida por mim):**

| afirmação | como confirmei |
|---|---|
| a guarda tinha de incidir no **basename**, não na captura | `DEST="/tmp/work/"` → captura `[/tmp/work/]` **não-vazia**, basename `[]` **vazia** — o `[ -z "$DEST_BARE" ]` copiado do padrão do `URL` **não dispararia** |
| o gate novo passa na árvore | `check-unguarded-capture-rc.sh` → **rc=0**, com `OK alegacao viva: (check-agent-namespace-union.sh, zulu_ln) casa 1 sitio(s)` |
| a guarda de não-vacuidade funciona | `MIN_CANDIDATES=9999` → **rc=1**, 108 candidatos, piso 60 |
| `FALSIFY_SUCCESS_FLOOR` | 210 → **227**, e o ML-2J não acrescentou braço — sem colisão entre os dois MLs paralelos |

🔴 **O ML-2H alargou o próprio enunciado, com razão medida.** Eu escrevi "posição **não-final**". Ele
incluiu o **elo final** porque a §4 da nota do ML-2G mediu que `check-annotations.yml:80` — o sítio
que esta REQ acabou de corrigir — tem o `grep` em posição **final**, onde o `set -e` mata **sem**
`pipefail`. Um gate restrito a "não-final" deixaria o defeito voltar. Declarado no cabeçalho.

🔴 **A guarda da classe 6 é ela mesma falsificável** (braço P): alegação que não casa sítio nenhum
**reprova**. É o que separa "declarar um limite" de "declarar e seguir em frente" — uma guarda que
nunca pode reprovar não é guarda.

⚠️ **Erro meu nesta auditoria — o nono do instrumento.** Rodei o comando da classe 5 e obtive **5**
sítios onde o executor afirmara **0**. Investigado: os 5 são **comentários e strings do próprio gate
e do Cenário 199**, que ele acabou de criar. A medição dele era **pré-entrega** e estava certa; a
minha mediu depois e contou o código de teste como se fosse produto. **Medir a árvore depois da
entrega conta o instrumento junto com o objeto.**

**Decisão minha, que estava pendente:** o gate acusaria `check-agent-namespace-union.sh:904,905`, e
a forma **preferida** de isenção é o marcador inline — que exige editar um arquivo proibido ao
`artemis-tf`. **Aceito a tabela `ALLEGATIONS` por ora**, porque ela é falsificável e chaveada por
basename+variável (não por número de linha, que envelhece). **O marcador inline entra no ML-2K**,
que já vai tocar arquivo de produto.

---

### ML-2K — O cross-check que concorda sem medir
**Owner:** `ares-tf`
**Status:** 🔄 Entregue e auditado · ❌ **a árvore está VERMELHA** até o ML-2L — ver abaixo

Achado do ML-2J, **no arquivo que dá nome a esta REQ**:

```bash
CNT_FAIL_GREP=$(grep -ac '^FAIL' "$LOG_FILE" … || true)     # windows-census.yml:489
CNT_FAIL_AWK=$(awk '/^FAIL/{n++} END{print n+0}' "$LOG_FILE")
```

São **duas capturas do mesmo produtor**. Com log **ausente ou vazio**, as duas valem `0`,
**concordam**, e nenhum `::warning::` de discrepância sai. 🔴 **O cross-check de instrumentos
concorda sem medir** — é a mesma defesa que a causa A já tinha derrotado, agora vácua por outro
caminho.

**Ações:** guarda de não-vacuidade no par (um log legível e não-vazio é pré-condição da comparação);
e o **marcador inline** `# unguarded-capture-rc-allowed:` em `check-agent-namespace-union.sh:904,905`,
migrando da tabela `ALLEGATIONS` para a forma preferida.

- [ ] O par passa a distinguir "concordam em zero porque não há o que medir" de "concordam porque
      mediram o mesmo" — falsificado nas duas direções
- [ ] Marcador inline nos dois sítios; a entrada correspondente sai da tabela e a **guarda de
      obsolescência** continua verde
- [ ] 🔴 Uma frase por teste novo, ou ausência declarada
- [ ] 🔴 **NÃO rodar `make quality`**


**Auditoria do arquiteto (medida por mim):**

| afirmação | como confirmei |
|---|---|
| os dois sítios saem pela classe **pretendida** | `OK [exempt/class6-alleged-inline/check-agent-namespace-union.sh:905\|:907]` — isenção pela classe 6, não veredito certo por razão errada |
| 🔴 a guarda de obsolescência ficou **vácua** | `FORCE_ALLEGATION_GUARD=1` com tabela vazia → **rc=0**; a seção imprime o cabeçalho e **nada embaixo** |

🔴 **Ele entregou sabendo que a árvore reprova, e mediu em vez de prever.** É o comportamento certo:
o handoff mandou tratar como achado de relatório, não como ajuste silencioso.

**O que aconteceu é a ironia inteira desta REQ numa linha:** ao migrar as duas alegações para a forma
**preferida** (marcador inline), a tabela esvaziou — e a guarda que vigia a obsolescência da tabela
passou a **iterar zero entradas e reportar sucesso**. É **literalmente a classe de defeito que o
cabeçalho do próprio gate nomeia como razão de existir**, reproduzida por acidente, pela correção
que o tornava melhor.

E o efeito colateral é que o braço **P** do Cenário 199 — o que falsifica a guarda — deixa de
falsificar: `assert_fails_with "alegacao obsoleta"` recebe **rc=0**. A tabela era a **única fixture**
dele.

**Decisão minha — opção (1), fixture injetável.** O executor apresentou duas, honestamente:

1. o gate ganha fixture injetável, e o braço P constrói a própria alegação;
2. a tabela continua não-vazia (reverter um dos marcadores).

Escolho **(1)**. A (2) é custo zero e funciona hoje, mas deixa a guarda **falsificável apenas por
acidente** — ela só teria teste enquanto sobrasse alegação real. 🔴 **Uma guarda que só funciona
porque existe dado real é infalsificável no dia em que o dado acaba** — e este roadmap já mediu
três vezes que a defesa vácua é pior que a ausência de defesa.

---

### ML-2L — A guarda de obsolescência precisa da própria fixture
**Owner:** `artemis-tf` (autor do gate)
**Status:** ⬜ Pendente — 🔴 **bloqueia a barreira**

- [ ] `check-unguarded-capture-rc.sh` aceita alegação **sintética** (env/fixture) para que a guarda de
      obsolescência seja exercitável com a tabela **vazia**
- [ ] O braço **P** do Cenário 199 volta a falsificar — `assert_fails_with "alegacao obsoleta"` reprova
- [ ] 🔴 A guarda passa a **reprovar quando não há o que verificar**, em vez de reportar sucesso
      iterando zero — é a mesma testemunha-de-medição que o ML-2K pôs no censo
- [ ] `make quality` volta a **RC=0** (verificado pelo arquiteto)
- [ ] 🔴 Uma frase por teste novo

---

## Wave 3 — Linha de base (1 ML, do arquiteto)
> Dependências: Waves 1 e 2 mergeadas.

- [ ] Censo disparado em `main` pós-correção termina **sem** `TOTAL INCOMPLETO`
- [ ] Total por shard registrado aqui como **linha de base pós-v8**
- [ ] 🔴 O número **não** é usado para triar o cluster de Windows nesta REQ (escopo negativo)

> ⚠️ O run `35920654658` (main, 2026-09-23 pré-correção) é **nulo como linha de base** — ele tem a
> causa A. Não citar o total dele.

## Barreira final

Revisão `hefesto-tf` e `hades-tf`, auditoria do arquiteto, `trackfw barrier`, CI verde.
