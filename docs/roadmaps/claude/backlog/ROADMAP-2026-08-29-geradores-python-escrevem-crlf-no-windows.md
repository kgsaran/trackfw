---
status: backlog
date: 2026-08-29
req: docs/requisições/claude/REQ-2026-08-29-geradores-python-escrevem-crlf-no-windows.md
squad: ""
---

# Roadmap: Geradores Python escrevem CRLF no Windows

> Created: 2026-08-29 | Status: done

## Context

O CLI Python grava todo arquivo com CRLF no Windows; Go e Node gravam LF. Viola a Regra Dura de
Paridade e quebra os `scripts/*.sh` gerados em qualquer sistema POSIX. Bloqueia o ML-2A do roadmap
do slug.

REQ: docs/requisições/claude/REQ-2026-08-29-geradores-python-escrevem-crlf-no-windows.md

## Acceptance Criteria

- [ ] CLI Python grava LF em todo arquivo que produz, medido por varredura de bytes
- [ ] `scripts/*.sh` com o mesmo shebang nos tres runtimes
- [ ] `check-artifact-parity.sh` sem os 8 drifts `go vs python`
- [ ] Gate **falha** com o CRLF reintroduzido — nao-vacuidade verificada
- [ ] Suite pypi sem regressao contra 198 failed / 1294 passed

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model for this roadmap
**Status:** done

#### 1. Completude da enumeracao

Enumerei **por efeito**, nao por call site: rodei `init` e os quatro `new` num diretorio limpo por
runtime e varri os bytes de tudo que sobrou.

```
py    CRLF=23 arquivos   LF=0
go    CRLF=0             LF=22
```

Zero excecao dos dois lados, o que fecha a questao melhor do que qualquer grep. A contagem estatica
(48 `open(w|a)` sem `newline`, 20 `write_text`, 2 `json.dump`, 18 `print(file=)`) serve para achar
os sites a corrigir, mas **nao** para provar cobertura — `write_text` e `print(file=)` tambem
traduzem, e sao faceis de esquecer numa varredura que so procura `open`.

Os 23 arquivos cobrem cinco familias: os 9 slash commands em `.claude/commands/trackfw/`,
`.claude/settings.json`, `CLAUDE.md`, os 4 artefatos de governanca e os 5 `scripts/*.sh`.

#### 2. Quem esvazia esta Wave 0 sem quebrar regra escrita

1. **Corrigir so os `open()` e deixar `write_text` e `print(file=)`.** Nada proibe, e a varredura
   ingenua por `open` da a sensacao de completude. **Coberto:** o criterio exige varredura de bytes
   do resultado, nao contagem de call site.
2. **Deixar o gate insensivel a EOL** (`diff --strip-trailing-cr`) e declarar resolvido. Some o
   sintoma, fica o `.sh` quebrado em POSIX. **Nao coberto por gate** — fica como proibicao escrita
   aqui: a comparacao continua byte a byte.
3. **Merge futuro do upstream reintroduz.** A CI deles e Linux e nunca ve isso, entao todo arquivo
   novo que eles escreverem vem sem `newline`. **Coberto:** o gate da Wave 0 e estatico e falha em
   qualquer site novo sem `newline`.

#### 3. Alvos de falsificacao, nas duas direcoes

| Regride para | Quebra o que |
|---|---|
| CRLF (hoje) | `.sh` com `bad interpreter: bash^M` em POSIX; 8 drifts no gate de artefato |
| LF via modo binario | some a traducao mas quebra encoding — o `_force_utf8_output` cuida de stdout, nao de arquivo |
| gate insensivel a EOL | verde com o defeito vivo; e o pior estado, porque parece resolvido |
| `newline=""` em uns e `newline="
"` em outros | os dois funcionam para escrita, mas a inconsistencia convida alguem a "padronizar" de volta para `None` |

#### 4. Residual declarado

- **Divergencia de shebang** (`bash` no Python contra `sh` em Go e Node) foi medida aqui mas nao e
  fim de linha. Vai para ML-1B, separada, para nao misturar as duas.
- Leitura de arquivo nao entra: `open(..., "r")` com `newline=None` faz universal newlines, que ja
  aceita os dois. So a escrita diverge.
- Arquivo binario nao entra.

**Acceptance criteria:**
- [x] As quatro secoes respondidas com evidencia medida
- [x] Nenhuma linha de implementacao escrita nesta ML

**Gates da wave:**
```bash
bash scripts/check-python-writes-lf.sh
```

## Wave 1 — A correcao
> Dependencies: ML-0A

### ML-1A — Escrita de arquivo em LF no Python
**Status:** done
**Files affected:** 15 arquivos de `pypi/trackfw/`, 68 sites

**O que mudou:** `newline="
"` explicito em toda escrita de texto — `open(w|a)` e
`write_text`. Nao `newline=""`: os dois evitam a traducao, mas `"
"` declara a intencao. Modo
binario nao foi tocado. Aplicado com parser de parenteses balanceados, nao regex, porque ha
chamadas multilinha.

**Medicao por efeito, o criterio de aceite:**

```
antes    py  CRLF=23  LF=0
depois   py  CRLF=0   LF=23
```

**Efeito no gate de artefato:** `check-artifact-parity.sh` sai de **8 drifts `go vs python`** para
**0**. Era tudo isto.

**Regressao — medida por lista nomeada, nao por contagem.** As duas corridas na **mesma arvore de
trabalho**, revertendo os 15 arquivos para o `HEAD` e restaurando depois:

```
antes    200 failed / 1292 passed
depois   199 failed / 1293 passed

novas falhas:     nenhuma
falha que sumiu:  test_validator.py::TestValidateStaleWip::test_stale_wip_warning_arquivo_antigo
```

O teste que oscila e o de skew de relogio ja caracterizado nesta sessao — `time.time()` no teste
contra `datetime.now().timestamp()` na producao. Explica as tres contagens diferentes (198, 199,
200) para o mesmo codigo.

> Uma primeira tentativa comparou contra uma **copia isolada** do `pypi/` em tempdir e deu 205
> falhas — 6 a mais, todas por a copia estar fora do repo (`test_thirdparty` le
> `../../docs/seguranca/`, `test_documentation_contract` le o site). Baseline contaminada,
> descartada. So a medicao na propria arvore compara o que precisa ser comparado.

**Acceptance criteria:**
- [x] `py CRLF=0` na mesma medicao que antes dava 23
- [x] Suite pypi sem regressao — zero falhas novas, verificado por diff de lista nomeada

### ML-1B — Shebang identico nos tres runtimes
**Status:** abandoned — **eu estava errado**

Medi os cinco `scripts/*.sh` nos tres runtimes:

```
trackfw-validate            go sh    node sh    py bash    <- diverge
trackfw-attention-signal    go bash  node bash  py bash
trackfw-attention-cleanup   go bash  node bash  py bash
trackfw-credential-guard    go bash  node bash  py bash
trackfw-git-branch-guard    go bash  node bash  py bash
```

So o `trackfw-validate.sh` diverge — e essa divergencia e **deliberada, arquitetada e
documentada**: decisao de arquiteto de 2026-08-27, registrada em `docs/cli-parity.md` sob
"validate.sh — pertencimento a conjunto", com um check de set-membership construido no
`scaffold_doctor` dos tres runtimes so para acomoda-la
(`internal/generators/scaffold_doctor.go:224`, `pypi/trackfw/integrations/scaffold_doctor.py:5`).

Eu tinha medido a divergencia sem ter lido a decisao. "Alinhar" quebraria o contrato e a maquinaria
construida em volta dele. **ML abandonada, nao adiada.**

## Wave 2 — A guarda
> Dependencies: ML-1A, ML-1B

### ML-2A — Gate de escrita em LF
**Status:** done
**Files affected:** `scripts/check-python-writes-lf.sh` (novo)

Gate estatico: nenhuma escrita de texto em `pypi/trackfw/` sem `newline` explicito. Usa o mesmo
parser de parenteses balanceados da correcao, entao cobre `open(w|a)` e `write_text`, ignora modo
binario e nao se confunde com chamada multilinha.

Estatico de proposito: a CI do upstream e Linux e nunca vera este defeito, entao todo arquivo novo
que vier de la chega sem `newline`. Este gate pega no merge, que e onde a regressao vai nascer.

**Nao-vacuidade verificada** — removi o `newline` de um site e ele acusou o arquivo e a linha:

```
escrita de texto sem newline explicito em pypi/trackfw/:
  pypi/trackfw/generators/note.py:65
rc=1
```

> A primeira tentativa deste teste passou verde e era **vacuosa**: o escape do heredoc virou quebra
> de linha real e a injecao nunca aconteceu. So peguei porque conferi se o padrao tinha sido
> encontrado antes de olhar o resultado do gate. Fica registrado porque e a segunda vez nesta
> sessao que esse heredoc engana.

**Acceptance criteria:**
- [x] Gate passa depois de ML-1A
- [x] Gate **falha** com um site sem `newline` reintroduzido — saida colada acima
- [x] `check-artifact-parity.sh` sem os 8 drifts

---

## Passivo aberto — o gate de artefato ainda nao passa no Windows

Os 8 drifts sumiram, mas o `check-artifact-parity.sh` continua saindo 1, agora por outro motivo:
ele roda `validate --json` num fixture e o **Node** devolve nao-zero, com `set -euo pipefail`
abortando o gate. As violacoes citam a home **real** do usuario (`~/.trackfw/scripts/`, caminho
absoluto em `C:/Users/...`) apesar de o gate exportar `HOME="$WORK/home"`.

E o mesmo problema de isolamento de home que a migracao corrigiu no Go com `internal/homedir`,
**nao corrigido em Node nem em Python**. Some o defeito de CRLF, aparece a proxima parede.

Consequencia: o **ML-2A do roadmap do slug segue bloqueado**, agora por esta e nao mais pelo CRLF.
Precisa de REQ propria. A superficie ja e conhecida: `os.homedir()` no Node e
`os.path.expanduser` no Python leem `%USERPROFILE%`, nao `$HOME`.
