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
**Status:** ⬜ Pendente
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
- [ ] Morte de chunk passa a emitir sítio e rc; provado por injeção de uma falha artificial
- [ ] Os dois sítios deixam de engolir stderr no caminho de falha
- [ ] Braço POSIX: nada muda no que já passa
- [ ] 🔴 Uma frase por teste novo, dizendo qual conclusão deste ML ele afirma
- [ ] 🔴 **NÃO rodar `make quality`**

### ML-2C — A guarda de conjunto enxerga rótulo emitido por `echo`, e o Cenário 18 volta a ser corte
**Owner:** `artemis-tf`
**Status:** ⬜ Pendente
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
- [ ] `HDR_PAT` casa o Cenário 18; contagem de cabeçalhos antes/depois colada
- [ ] Rótulos por `echo` viram exigência; número medido, com o comando
- [ ] Apagar um `echo` de controle reprova — provado por injeção
- [ ] Guarda de não-vacuidade com piso e o comando que o produziu
- [ ] 🔴 Uma frase por teste novo
- [ ] 🔴 **NÃO rodar `make quality`**

### ML-2B — Sonda: por que o `git` sai 128 no Windows
**Owner:** arquiteto (dispara e lê), implementação por `ares-tf`
**Status:** ⬜ Pendente — **depende do ML-2A mergeado**

Job `windows-latest` que roda **verbatim** `check-gates-falsify.sh:1553-1572` com o `2>/dev/null`
removido, ecoando rc e stderr por comando. Responde **qual sítio** e **por que**, de uma vez.

**Critérios de aceite:**
- [ ] A causa do `rc=128` fica **escrita com a medição**, ou a lista do que a sonda eliminou
- [ ] 🔴 Nenhuma hipótese apresentada como causa

## Wave 3 — Linha de base (1 ML, do arquiteto)
> Dependências: Waves 1 e 2 mergeadas.

- [ ] Censo disparado em `main` pós-correção termina **sem** `TOTAL INCOMPLETO`
- [ ] Total por shard registrado aqui como **linha de base pós-v8**
- [ ] 🔴 O número **não** é usado para triar o cluster de Windows nesta REQ (escopo negativo)

> ⚠️ O run `35920654658` (main, 2026-09-23 pré-correção) é **nulo como linha de base** — ele tem a
> causa A. Não citar o total dele.

## Barreira final

Revisão `hefesto-tf` e `hades-tf`, auditoria do arquiteto, `trackfw barrier`, CI verde.
