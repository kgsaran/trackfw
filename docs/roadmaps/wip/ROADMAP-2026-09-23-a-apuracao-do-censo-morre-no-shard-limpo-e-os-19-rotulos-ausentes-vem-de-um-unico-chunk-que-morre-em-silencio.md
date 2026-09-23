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
**Status:** ⬜ Pendente
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
- [ ] Mecanismo da causa B escrito **com a medição**, ou a lista do que foi eliminado
- [ ] Enumeração da causa A confirmada ou refutada, com o comando que produziu a lista
- [ ] Tabela garantia-sem-prova para os 12 controles de segurança, com veredito de cobertura
      equivalente por controle
- [ ] As 2 frases de fechamento escritas
- [ ] 🔴 **Nenhuma linha de implementação neste ML** — é medição
- [ ] 🔴 **NÃO rodar `make quality`** — a barreira é do arquiteto

**Comandos de validação:** `trackfw barrier <roadmap> --wave 0`

---

## Wave 1 — Causa A: a captura, e o gate que impede a volta (2 MLs em paralelo)
> Dependências: **Wave 0 auditada.** Os dois MLs tocam arquivos disjuntos.

### ML-1A — A forma de captura, nos sítios que a Wave 0 confirmar
**Owner:** `ares-tf`
**Status:** ⬜ Pendente
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
- [ ] Cada sítio (a) corrigido pela forma de captura
- [ ] Replay nas duas direções, contra os artefatos reais, com os dois números colados no relatório
- [ ] Nenhuma comparação nova acrescentada sobre captura quebrada
- [ ] 🔴 Uma frase por teste novo, dizendo qual conclusão deste ML ele afirma
- [ ] 🔴 **NÃO rodar `make quality`**

### ML-1B — Gate anti-reintrodução, falsificável, com guarda de não-vacuidade
**Owner:** `artemis-tf`
**Status:** ⬜ Pendente
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
- [ ] Gate reprova cada forma coberta — provado por injeção, uma a uma
- [ ] Gate passa na árvore correta, com piso de não-vacuidade e o comando que o produziu
- [ ] Formas não cobertas **declaradas** no cabeçalho, com a razão
- [ ] Falsificação com rótulos literais
- [ ] 🔴 Uma frase por teste novo
- [ ] 🔴 **NÃO rodar `make quality`**

---

## Wave 2 — Causa B: o chunk que morre em silêncio (1 ML)
> Dependências: **Wave 0 auditada** (o mecanismo vem de lá). Detalhamento desta wave é escrito
> **depois** do ML-0A — ML detalhado sobre mecanismo não medido é o erro do `IsAbs`.

### ML-2A — a definir pelo ML-0A
**Status:** ⬜ Pendente

**Critério que já vale, qualquer que seja o mecanismo:**
- [ ] O shard passa a **denunciar** a própria morte — hoje ele para sem emitir nem contador nem
      sentinela, e a guarda local só percebe pela ausência de rótulo
- [ ] Os 19 rótulos voltam a ser emitidos **ou** cada ausência remanescente tem razão escrita

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
