---
status: wip
date: 2026-09-07
squad: ares-tf
req: "docs/req/REQ-2026-09-07-os-gates-chamam-python3-e-binario-hardcoded-e-nenhum-job-do-ci-os-exercita-no-windows.md"
---

# Roadmap: Os gates rodam no Windows — resolução de interpretador e de binário

> Criado em: 2026-09-07 | Status: wip

## Context

REQ: `docs/req/REQ-2026-09-07-os-gates-chamam-python3-e-binario-hardcoded-e-nenhum-job-do-ci-os-exercita-no-windows.md`

## Diagnóstico — medido na VM Windows 10 Pro ARM64, 2026-09-07

```
python   → .../Programs/Python/Python312-arm64/python    ✅ real
python3  → .../Microsoft/WindowsApps/python3             ❌ stub da Microsoft Store
py       → .../Programs/Python/Launcher/py               ✅ launcher

chamadas hardcoded nos gates:  python3 → 396   ·   python → 312
configurável:                  só smoke-integration-packages.sh (PYTHON_BIN)
```

O stub imprime **"Python was not found"** com o Python instalado e funcionando.

**Por que nunca apareceu:** `quality.yml:522-523` — o job `parity` roda em `ubuntu-latest`. Nenhum
job do CI executa estes scripts no Windows. Não é regressão; é **superfície nunca exercitada**.

🔴 **Duas hipóteses do arquiteto caíram por medição** — não re-derivar:
1. *"88 ocorrências de `$ROOT_DIR/bin/trackfw` sem `.exe`"* — patchei as 88 na VM, **resultado
   idêntico**. A falha era o binário **não existir**.
2. *"O `cp -r` do Cenário 8 trava o Windows"* — correto, mas **não é o primeiro** obstáculo.

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 1 — Fazer o gate chegar ao fim no Windows

### ML-1A — Resolução do interpretador Python
**Status:** ✅ Concluído · **Agente:** `ares-tf`

Ponto único que escolhe um interpretador **funcional**, rejeitando o stub da Store.

🔴 **Não** trocar `python3` por `python` em massa por `sed`: no Linux `python` pode não existir ou ser
Python 2. Tem de ser **detecção**, não substituição literal.

**Critérios:**
- [ ] rejeita o stub da Store — provado, não só "acha algum python3"
- [ ] conjunto de cenários em Linux/macOS **idêntico** (igualdade de conjunto, não contagem)
- [ ] `make quality` para arquivo, `grep -c '^FAIL'` sobre a saída inteira = 0

### ML-1B — Resolução do binário do CLI
**Status:** ✅ Concluído · **Agente:** `ares-tf`

Honrar o sufixo da plataforma **e falhar alto se o binário não existir**, em vez de cair em
`go build` a partir de fixture sem `go.mod`.

🔴 **A mensagem de erro tem de nomear a causa real.** O sintoma observado foi
`go: go.mod file not found` — que não tem nada a ver com a causa e me levou a duas hipóteses erradas.

**Critérios:**
- [ ] binário ausente ⇒ erro que diz *"binário ausente"*, não erro de módulo
- [ ] sufixo de plataforma honrado
- [ ] comportamento em Linux/macOS inalterado

## Wave 2 — Medir o que existe atrás
> Dependências: Wave 1.

### ML-2A — Rodar até o fim no Windows e escrever a lista
**Status:** 🔄 Em andamento (achados escritos; decisão de escopo já resolvida pelo arquiteto — ver seção
"Decisão do arquiteto sobre o AC3"; lista enumerada REPRODUZÍVEL agora entregue pelo ML-2B, seção
"Lista enumerada do Windows (VM, 2026-09-08)" abaixo — 8/8 chunks, sem depender mais da sonda
descartável) · **Agente:** `ares-tf`

**Método:** VM sincronizada na HEAD desta branch (`4456c95`), `bin/trackfw.exe` recompilado.
`scripts/check-gates-falsify.sh` rodado **serial**, sem chunking, com `FALSIFY_GO_BIN` apontando pro
exe — mesmo método usado para o conjunto de Linux/macOS abaixo (ver seção "Conjunto Linux/macOS").
**Nenhum arquivo de `scripts/` ou código de produção foi alterado nesta sessão** — só governança
(este roadmap) e vault. `git status --porcelain` confirma: só o roadmap e uma nota nova de vault.

#### Obstáculo 0 — pré-condição de ambiente, corrigida (não é o gate)

1º cenário abortava com `ln: failed to create symbolic link '.../npm/node_modules': No such file or
directory` — `npm install` nunca tinha rodado no clone da VM. Rodei `npm install` (ambiente, não
código) e segui. **Candidato a guarda para o arquiteto triar:** o gate assume `npm/node_modules`
pré-existente; `make quality` cobre isso via `npm ci` antes, mas rodar o gate isolado (como uma VM
de CI faria antes de outro job instalar deps) expõe essa precondição silenciosa.

#### Obstáculo 1 — o gate COMMITADO aborta no Cenário 17 (`update-parity/dry-run-write-leak`)

Não no teste que o cenário pretende provar — **antes**, no `for h in go node py` do Cenário 6 interno
de `scripts/check-update-parity.sh:560`, que roda como fixture. `install_agent_global go` roda,
imprime "install complete" (Go achou que funcionou), e o `python3 -c
"...json.load(open('$manifest'))..."` seguinte reprova com `FileNotFoundError` — mesmo literal de
caminho, mesmo processo pai, dois runtimes discordando sobre onde o arquivo está.

**Causa raiz medida (não suposta) — MSYS/Git-Bash só converte caminho POSIX→Windows quando ele é o
TOKEN INTEIRO, não quando embutido numa string maior.** Nota do vault:
[msys-nao-converte-caminho-embutido-em-string-maior-2026-09-07](../../../vault/notes/msys-nao-converte-caminho-embutido-em-string-maior-2026-09-07.md).
Prova mínima, **sem nenhum código do trackfw envolvido, sem o shim de PATH deste ML**:
```bash
touch /tmp/probe-file
python.exe -c "print(open('/tmp/probe-file').read())"                        # FileNotFoundError
python.exe -c "import sys; print(open(sys.argv[1]).read())" /tmp/probe-file  # "hello" — OK
```
`HOME="$home_dir" "$GO_BIN" ...` passa o caminho como **valor inteiro de env var** → convertido →
Go escreve certo (confirmado: `find /tmp/trackfw-update-parity.XXX -name integrations-manifest.json`
acha o arquivo pelo caminho bash-resolvido). O `python3 -c "...open('$manifest')..."` embute o mesmo
literal **dentro do script** → não convertido → tenta abrir sob `C:\tmp\...` (drive-relativo ao drive
corrente), que não existe (`ls /c/tmp` → "No such file or directory").

**Classificação: "separador de caminho" — categoria explicitamente excluída deste ML (interpretador/
binário). Não corrigido.** Confirmado shim-agnóstico (idêntico com e sem `FALSIFY_PY_SHIM_DIR`,
com `python.exe` chamado direto por caminho absoluto) — não é efeito colateral do ML-1A/2A anteriores.
Padrão presente em 3 sítios de `check-update-parity.sh`: linhas 560, 617, 685.

#### Sondagem além do Cenário 17 (probe instrumentado, NUNCA commitado — resultado não-oficial)

`assert_fails_with` (e 7 helpers de asserção irmãos) fazem `exit 1` sob `set -euo pipefail` — **o
gate é fail-fast em QUALQUER plataforma**, não só Windows; em Linux/macOS ele só "roda até o fim"
porque nada falha lá. Isso significa que **AC3 ("roda até o fim, abortar no meio não é aceitável")
é, como está escrito, insatisfazível sem corrigir TODO obstáculo Windows encontrado** — inclusive os
de categoria explicitamente fora de escopo deste ML. Não redefini o critério silenciosamente:
registro o conflito aqui para o arquiteto decidir.

Para dar visibilidade do que existe **depois** do Cenário 17 (sem prometer exaustão), fiz uma cópia
`scripts/check-gates-falsify-PROBE.sh` **só na VM, nunca copiada pro repo, nunca commitada** —
troquei os `exit 1` das 8 funções de asserção (`assert_fails_with`, `assert_would_now_fail`,
`assert_output_contains`, `assert_output_lacks`, `assert_guard_exit`, `assert_writer_no_epipe`,
`assert_lacks_pattern`, `assert_succeeds`) e mais 3 `exit 1` inline (Cenário 18 "no-repo-mutation",
Cenário 29 "validate-ok-message") por `FAIL=1` (sem abortar), pra enumerar em vez de morrer no
primeiro. Rodei até a linha ~246 do log (por volta do Cenário 46 de ~194), quando o tempo desta
sessão se esgotou — **não é exaustivo**, é uma amostra maior que o mínimo.

**Achados adicionais do probe** (cada um com o mesmo teste de shim-agnosticismo não repetido por
tempo — marcados como "achado, não totalmente reconciliado"):

| Cenário | Sintoma | Causa provável | Categoria |
|---|---|---|---|
| 17 (real) | `FileNotFoundError` no `python3 -c open()` | MSYS não converte caminho embutido | separador de caminho — **fora de escopo, não corrigir** |
| 18 (no-repo-mutation), sítio `check-update-parity.sh` | `FileNotFoundError` idêntico ao do 17 | **causa medida igual ao 17** (mesmo `FileNotFoundError` no mesmo `.trackfw/integrations-manifest.json`, log conferido linha a linha) | mesma causa — **fora de escopo** |
| 18 (no-repo-mutation), sítio `check-roadmap-move-parity.sh` | `stdout diverges`: Go emite `docs/roadmaps/wip`, Python emite `docs/roadmaps\wip` | **causa DIFERENTE, medida** (não é a mesma do 17 — conferi o log: nenhum `FileNotFoundError`, é Python usando separador nativo do SO na mensagem de saída enquanto Go usa `/` fixo) | separador de caminho — **categoria igual à do 17, sítio/causa concreta distinta; fora de escopo, mas é REQ própria se algum dia corrigida (Regra Dura de Causa Raiz: causa diferente do 17)** |
| `update-harness/populated-harness/go-vs-python` (fixture do Cenário 17) | `claude-agents` state `skipped` (Go) vs `updated` (Python) | **divergência real entre runtimes**, não vista em Linux/macOS (gate nunca rodou lá) — carece de investigação própria | **não é causa Windows** — candidato a REQ própria ou ML-6F revisitado |
| `validate-ok-message/baseline-byte-identical-and-pinned` (Cenário 29) | Node emite "Nenhuma violação encontrada" mesmo com `LANG=en_US.UTF-8 LC_ALL=en_US.UTF-8`; Go/Python respeitam e emitem inglês | Windows não tem `LANG`/`LC_ALL` como convenção de locale — Node parece resolver locale pela UI do SO (pt-BR nesta VM), não pelas env vars POSIX | **locale/i18n do SO — categoria nova, fora de escopo (não é interpretador/binário)** |
| `agent-hooks-parity/credential-guard-present-vacuity/baseline` | "árvore íntegra deveria passar, saiu com 1" | não diagnosticado (tempo) | **não diagnosticado — deixado para o arquiteto/próxima sessão** |
| `agent-hooks-parity/amazonq/go-vs-py` | `structural drift` no campo `description`: `"...guard hook/denylist"` com `â€”` (Go) vs `—` (Python) | mojibake — bytes UTF-8 do Go reinterpretados como cp1252/latin1 em algum ponto da captura no console Windows | **encoding — categoria já conhecida do projeto (ver `PYTHONIOENCODING=utf-8` no cabeçalho de `check-update-parity.sh`), fora de escopo deste ML** |

🔴 **Ressalva de fidelidade do probe, a partir da linha do Cenário 29 (`FAIL=1` no lugar do `exit 1`
de linha 2914 do script instrumentado):** ao contrário dos outros `exit 1` neutralizados (que
terminam a scenario/loop ali mesmo), este `FAIL=1` deixou a execução **cair adiante** no bloco
seguinte do próprio Cenário 29 ("--- Python: prova de detecção ---"), que assume a baseline
anterior como válida. **Todos os rótulos depois do Cenário 29 no log do probe (inclusive as duas
últimas linhas desta tabela — `credential-guard-present-vacuity` e o mojibake do `amazonq`) rodam
sob uma premissa quebrada e podem ser artefato do instrumento, não do gate real.** Não tratar como
achado limpo sem re-rodar isolado (ex.: reproduzir cada um sozinho, fora do probe, como fiz para o
Cenário 17).

**O que isso significa para AC3:** o gate committado para no Cenário 17 hoje. Categorias adicionais
de obstáculo Windows (locale/i18n, encoding, mais um caso não diagnosticado de vacuity-guard) foram
confirmadas existirem mais à frente, então **mais haverá** se algum dia o 17 for corrigido — o
"roda até o fim" não é alcançável neste ML sem expandir o escopo para essas categorias também.
**Recomendação:** Wave 3 (ML-3A, do arquiteto) decide não só "rodar o parity no CI" mas também se
vale abrir REQ(s) próprias para separador-de-caminho, locale/i18n do Windows e encoding — cada uma é
causa distinta, então cada uma é REQ própria (Regra Dura de Causa Raiz).

#### Reconciliação com as "39 falhas residuais"

🔴 **Correção após revisão:** o que segue reconcilia **superfícies** (jobs de CI), não **causas** —
não medi se alguma causa encontrada aqui já está representada nas 39. Não afirmar sobreposição zero.

**Superfícies são disjuntas, medido:** as 39 residuais vêm de `windows-full-suites` /
`windows-defect-reproduction` (`.github/workflows/quality.yml:139,437`), que rodam `go test` /
`pytest` / `npm test` no Windows. O `parity` (`check-gates-falsify.sh`) roda **só em
`ubuntu-latest`** (`quality.yml:522`) e **nunca rodou no Windows antes desta sessão** (confirmado na
REQ) — então nenhum dos achados desta sessão pode já estar CONTADO nas 39 (jobs diferentes, gate
nunca executado lá).

**Se as CAUSAS se sobrepõem não foi medido.** Candidato concreto de sobreposição de causa (não
confirmado): o achado de mojibake (`agent-hooks-parity/amazonq/go-vs-py`, `â€”` vs `—`) é da mesma
família já rastreada no projeto — `REQ-2026-09-03-setenta-e-tres-das-duzentas-e-quarenta-e-seis-falhas-de-windows-nao-medem-nada-e-contaminam-qualquer-estimativa.md`
e as notas de vault `cp1252-roundtrip-mascara-o-defeito-o-discriminante-e-decode-de-stdin-2026-09-02`
e `gate-em-cp1252-tem-duas-falhas-distintas-crash-de-print-e-mismatch-por-transcodificacao-2026-09-02`.
Confirmar isso exige cruzar contra a lista real das 39, o que não fiz nesta sessão. **O número certo
é: 39 residuais (inalterado, superfície diferente, contagem não afetada) + os achados desta sessão
(1 causa medida e fora de escopo — MSYS path-embedding — mais um segundo sítio de causa distinta no
mesmo cenário 18, mais 2 categorias adicionais achadas no probe não-oficial — locale/i18n e
encoding/mojibake, esta última candidata a MESMA causa de um item já tracked, não confirmado) — subir
é esperado (a REQ já avisa isso), não regressão.**

#### Conjunto Linux/macOS

Nenhuma alteração de código nesta sessão (só doc/vault) ⇒ `diff` de conjunto de rótulos
trivialmente vazio (nada mudou no script que produz os rótulos). Método: mesmo script serial
(`bash scripts/check-gates-falsify.sh`), sem chunking, idêntico ao usado nas MLs anteriores desta
mesma REQ (ML-1A/1B) e na VM.

#### `make quality` / `check-cli-parity.sh` — evidência apresentada nesta sessão

`git status --porcelain` = 3 arquivos, todos de governança (este roadmap, 1 nota de vault nova,
`vault/notes/index.md`) — **zero arquivos de `scripts/` ou de produção alterados**. Com base nisso:

- `go build ./...` — limpo, sem erros.
- `./bin/trackfw validate` — exit 0 (só os 172 warnings pré-existentes, não relacionados a esta
  sessão, confirmados idênticos antes/depois).
- `scripts/check-cli-parity.sh` isolado — rc=0, "CLI parity smoke checks passed".
- **`make quality` completo NÃO foi rodado nesta sessão.** Justificativa: nenhum arquivo que o
  `parity`/`test`/`test-node`/`test-python`/`lint` exercitam foi tocado; o baseline 0 FAIL/4060
  linhas do ML-2E (mesma HEAD desta branch, sessão anterior) vale para este estado. Isto é uma
  decisão explícita, não uma omissão silenciosa — se o arquiteto exigir a reprodução completa antes
  de fechar a REQ, é um passo de 15-20min em batches foreground (método já documentado na entrada de
  ML-2E de `agents-working-context.md`).

#### Pendências explícitas para a próxima sessão / arquiteto
1. Diagnosticar `agent-hooks-parity/credential-guard-present-vacuity/baseline` (não investigado).
2. Continuar a sondagem do probe (parou por volta do cenário ~46/194) se o arquiteto decidir que
   vale o investimento — script instrumentado descrito acima, refazível a qualquer momento (não
   preservado em disco, só o método).
3. Decisão de escopo: o AC3 como escrito ("roda até o fim") conflita com o desenho fail-fast do
   gate — pedir ao arquiteto para relaxar o critério (ex.: "identifica e documenta cada ponto de
   parada, sem exigir que o gate real chegue ao fim") ou aceitar abrir REQs para as categorias
   achadas antes de fechar esta.

🔴 **"N cenários reprovam no Windows" é resultado válido.** O que não é válido é não saber.

Reconciliar com as **39 falhas residuais** da campanha — elas foram medidas **sem** que este gate
jamais rodasse no Windows.

**Critérios:**
- [ ] lista escrita dos cenários que reprovam, com o erro de cada um
- [ ] reconciliação com as 39 residuais: quantas são destas, quantas são novas
- [ ] fecha (ou não) o issue #288, com a medição que o autor pediu

## Wave 3 — Decisão de cobertura
> Dependências: Wave 2.

### ML-3A — Rodar o `parity` (ou subconjunto) em Windows no CI?
**Status:** ⬜ Pendente · **Agente:** `trackfw_architect`

🔴 **"Não vamos rodar" é decisão válida** — mas então fica **escrito** que a superfície é
declaradamente não coberta, em vez de silenciosamente.

## Fora deste roadmap
- **Paralelização do `parity`** → REQ própria, entregue no PR #291.
- **Instalação em Windows** → REQ própria já existente.

## Critérios de Aceite

- [ ] `check-gates-falsify.sh` roda até o fim no Windows
- [ ] conjunto de cenários em Linux/macOS idêntico ao de hoje
- [ ] lista dos que reprovam no Windows, escrita e reconciliada com as 39 residuais
- [ ] decisão de cobertura de CI registrada por escrito


## Decisão do arquiteto sobre o AC3 — 2026-09-07

🔴 **O AC3 estava mal escrito, e o erro é meu.** Ele exigia que o gate *"rode até o fim no Windows"*.
Mas o gate é **fail-fast** em toda plataforma (`set -euo pipefail`) — em Linux/macOS ele "chega ao
fim" apenas porque nada reprova lá. Então o AC3, como escrito, só seria satisfeito **corrigindo todos
os obstáculos de Windows**, inclusive os que este ML está **proibido** de tocar.

Critério de aceite que exige violar o escopo negativo do próprio ML é contradição interna — a mesma
família de defeito que a regra dura de reconciliação existe para pegar. O agente **recusou-se a
forçá-lo** e devolveu a contradição. Correto.

### AC3 revisado

> ~~O gate roda até o fim no Windows.~~
> **O gate avança até esgotar as causas deste ML (interpretador e binário), e os obstáculos além
> ficam enumerados por um mecanismo reproduzível.**

**Por que "reproduzível" é a palavra que importa:** a enumeração de hoje saiu de uma cópia
instrumentada (`check-gates-falsify-PROBE.sh`) que neutralizava os `exit 1` e **nunca foi commitada**.
O conhecimento existe só no relatório. Amanhã, para saber o que mudou, alguém refaz a sonda do zero —
ou, pior, confia numa lista que envelheceu.

### ML-2B — Modo de enumeração reproduzível
**Status:** 🔄 Em andamento (implementado, falsificado nas duas direções, VM Windows completa com lista
enumerada real — 8/8 chunks; `make quality` completo ainda pendente, decomposição necessária por tempo
de execução) · **Agente:** `ares-tf`

#### Relatório (2026-09-08)

**O que foi feito.** `TRACKFW_FALSIFY_ENUMERATE=1` em `scripts/check-gates-falsify.sh`: os ~199 pontos
de `exit 1` de resultado de cenário (excluídos os 2 pré-flight de interpretador/binário, linhas 98/178
— resolução de ambiente, não resultado de cenário) passam por `falsify_fail_point` (184 sites, código
FLAT fora de função) ou pelo padrão `falsify_count_failure; [[ enum==1 ]] && return 0 || exit 1` (15
sites, dentro dos helpers `assert_fails_with` e as 9 funções irmãs). Desligado (default): nenhuma linha
nova, nenhum branch novo — idêntico ao script de ontem. `scripts/gen-falsify-chunks.py` ganhou a mesma
checagem de fechamento injetada em TODO chunk gerado (não só no que herda a cauda do arquivo-fonte).

**Duas rodadas de falsificação reprovaram a primeira versão** — ver nota de vault
[enum-mode-return-1-e-variavel-de-shell-reintroduzem-o-exit-1-2026-09-08](../../../vault/notes/enum-mode-return-1-e-variavel-de-shell-reintroduzem-o-exit-1-2026-09-08.md):

1. **`return 1` num helper chamado nu sob `set -euo pipefail` aborta o call site** — byte a byte o
   mesmo efeito do `exit 1` que o ML existe para evitar, reintroduzido pela porta dos fundos.
   Sabotagem: Cenário 1 (`static-assets/byte-drift`) forçado a reprovar dentro de `assert_fails_with`;
   com `return 1`, o chunk morria ali (0 cenários depois). Corrigido para `return 0` (a contagem já
   aconteceu antes do `return`; o valor de retorno do helper só precisa ser 0 para não disparar
   `set -e`).
2. **Contador em variável de shell (`FALSIFY_ENUM_FAILURES=$((...))`) não sobrevive subshell** — boa
   parte dos ~199 pontos roda dentro de `( ... )`/`$( ... )`; a atribuição muta a cópia da subshell e
   some quando ela termina, então o fechamento do script encontra 0 e sai `exit 0` com `FAIL` no log —
   "transformar vermelho em verde" pela guarda 1. Corrigido: contagem em ARQUIVO (`$FALSIFY_ENUM_TALLY`
   sob `$WORK`, que já existe por processo/chunk) — escrita em arquivo atravessa fronteira de subshell,
   então a correção é estrutural (vale para os ~199 sites sem precisar auditar cada um por
   inspeção manual — só confirmado por amostra dirigida, ver prova 2 abaixo).

**Prova por sabotagem (as 3 guardas inegociáveis), com a versão final do código:**

| Prova | Cenário sabotado | Modo | Resultado |
|---|---|---|---|
| Guarda 1 (nunca torna o gate verde) — caminho FLAT | `static-assets/byte-drift` (Cenário 1, `assert_fails_with`) | desligado | 3 linhas, `exit 1` imediato — idêntico ao comportamento de sempre |
| Guarda 1, mesmo cenário | idem | ligado | enumera até o fim do chunk (46 OK depois do FAIL), `exit 1` no fechamento |
| Guarda 1 — caminho SUBSHELL (a amostra dirigida do Defeito 2) | Cenário 64, `git-branch-guard/no-op-outside-project/baseline-noop-without-trackfw-yaml`, dentro de `( cd ... && assert_guard_exit ... )` | desligado | 5 linhas, `exit 1` imediato |
| idem | idem | ligado | 46 OK depois do FAIL no mesmo chunk, `[falsify/enumerate] 1 cenário(s) reprovaram ... -- exit 1`, processo sai 1 |
| Guarda 1 — driver completo, 8 chunks | idem (Cenário 1) via `TRACKFW_FALSIFY_SCRIPT` | ligado | 411 OK / 1 FAIL / exit 1 no processo do driver — todos os OUTROS 7 chunks completos, só o sabotado reprova |
| Guarda 1 — driver completo | idem | desligado | fail-fast igual a antes: guarda de conjunto do driver denuncia 8 rótulos ausentes no chunk sabotado, exit 1 |
| Guarda 2 (rastro em stderr) | — | ligado | `[falsify/enumerate] modo de enumeração ATIVO (TRACKFW_FALSIFY_ENUMERATE=1, default=0) ...` emitido 1x por processo/chunk (8x no driver completo) |
| Guarda 3 (não é caminho de produção) | — | — | `grep -rn TRACKFW_FALSIFY_ENUMERATE Makefile .github/workflows/ scripts/` só acha ocorrências dentro de `check-gates-falsify.sh` e `gen-falsify-chunks.py` — nenhuma no Makefile nem nos workflows |

**Reconciliação (regra dura do CLAUDE.md — uma frase por teste novo):** este ML não adicionou teste
automatizado novo (nenhum arquivo em `internal/`, `npm/`, `pypi/` sob teste de CI) — as provas acima
são falsificações manuais, cada linha da tabela afirma exatamente a conclusão da coluna "Resultado", e
nenhuma delas se sustenta em execução de CI regular (o modo nunca roda em `make quality`/`make parity`,
guarda 3).

**Conjunto Linux/macOS — método declarado.** Driver paralelo (`scripts/run-gates-falsify-parallel.sh`,
8 chunks, JOBS auto-detectado=10→capado a 8), desligado (default), rodado duas vezes: versão
`origin/HEAD` do arquivo (via `TRACKFW_FALSIFY_SCRIPT`) e a versão final deste ML. Rótulos extraídos de
ambos os logs com `grep -oE '(OK|FAIL|PROOF)   \[falsify/[^]]*\]' | sort`: **384/384, `diff` vazio**.
Reconfirmado depois da correção dos 2 defeitos: novo run completo, desligado, 412 OK / 0 FAIL / exit 0
— mesma contagem do baseline anterior a este ML (ML-2E: 412 OK/0 FAIL — mais os OK "wrapped" de
sub-scripts, além dos rotulados `falsify/`).

**Sítios de mesma causa — reportado, sem abrir artefato novo (regra dura de causa raiz):** nenhum
sítio de mesma causa fora deste arquivo e de `gen-falsify-chunks.py` — os dois defeitos são
específicos à implementação do ML-2B (não existiam antes dele), então não há "sítio adicional" a
enumerar; ambos os pontos de correção (helper + gerador de chunks) já foram fechados nesta mesma
sessão/PR.

**Pendências explícitas (todas fechadas nesta mesma sessão, depois de escritas):**
1. ~~`make quality` completo~~ — feito, decomposto em lotes (Bash tem teto de 10min/chamada, `make
   quality` inteiro excede isso numa chamada só): `go test` (17 pacotes, cached/8.2s, 0 falhas),
   `npm test` (885 testes, 0 falhas), `python3 -m pytest pypi/tests -q` (1664 + 66 subtests, 0
   falhas), `go vet ./...` (limpo) — cada um seu próprio log em `/tmp/quality-logs/`. `make parity`
   (44 scripts) em 5 lotes sequenciais de ~9 scripts cada, mais `run-gates-falsify-parallel.sh` já
   validado antes (desligado, 412 OK/0 FAIL, ver prova por sabotagem do ML-2B) — logs concatenados em
   `/tmp/quality-logs/parity-full.log`: **`grep -c '^FAIL'` sobre o arquivo INTEIRO = 0** (nunca
   `| tail`). `GO_BIN=bin/trackfw scripts/check-cli-parity.sh` isolado — rc=0. `./bin/trackfw validate`
   — rc=0, 172 warnings pré-existentes (mesma contagem do ML-2A, não relacionados a esta sessão).
2. ~~VM Windows: rodar `bin/trackfw.exe` + `gen-falsify-chunks.py` + chunks sequenciais~~ — feito, ver
   seção "Lista enumerada do Windows (VM, 2026-09-08)" abaixo.
3. `scripts/` (infraestrutura de gate) é exceção explícita à regra de paridade 3-CLI de
   `docs/cli-parity.md` — não é um "sítio não coberto" da regra de paridade, é escopo fora dela por
   definição do projeto.

**Reconciliação — teste novo:** nenhum teste automatizado de CI foi adicionado por este ML (mudança é
em `scripts/`, exercitada por execução manual/falsificação, não por `go test`/`npm test`/`pytest`) —
consistente com a declaração já feita acima ("este ML não adicionou teste automatizado novo").

## Lista enumerada do Windows (VM, 2026-09-08)

**Método.** VM sincronizada em `27b09cc` (`git fetch` + `checkout -f origin/<branch>`); os 2 arquivos
tocados pelo ML-2B (`scripts/check-gates-falsify.sh`, `scripts/gen-falsify-chunks.py`) copiados por
`scp` — SHA256 conferido idêntico dos dois lados antes de rodar. `bin/trackfw.exe` recompilado e
confirmado existente antes de cada rodada. `gen-falsify-chunks.py` gerou 8 chunks (mesmo método do
driver paralelo); cada chunk rodado **sequencial, um processo `ssh` por chunk** com
`TRACKFW_FALSIFY_ENUMERATE=1`, log próprio por chunk — a tentativa anterior desta sessão de atribuir
log a um chunk específico do driver PARALELO por posição no stdout combinado deu falso positivo por
interleaving (ver relatório do ML-2B); log por-chunk sequencial elimina essa ambiguidade.

**Contagem por chunk (os 8 completos):**

| Chunk | OK | FAIL |
|---|---|---|
| 0 | 32 | 110 |
| 1 | 45 | 25 |
| 2 | 45 | 5 |
| 3 | 38 | 113 |
| 4 | 38 | 5 |
| 5 | 43 | 17 |
| 6 | 71 | 234 |
| 7 | 128 | 3 |
| **Total** | **440** | **512** |

🔴 **Este número não é "512 obstáculos distintos"** — a maior causa isolada (git não resolvível, ver
abaixo) sozinha produz ~250 dessas linhas por cascata (3-5 `FAIL` por cenário afetado). Contagem de
CAUSAS distintas, não de linhas: ver categorização abaixo.

**Achado dominante — NOVO, não visto pela sonda do ML-2A:** 135 das 275 reprovações de FAIL únicas nos
chunks 0-5 (49%) são a MESMA causa raiz — `git` não resolvível pelo processo filho nativo (Go
`exec.Command`, Node `spawnSync`, Python `subprocess`) invocado por `check-release-tag-parity.sh` /
`check-ship-force-parity.sh`. Causa medida, não suposta:
- Esses dois scripts constroem um `PATH` MÍNIMO e explícito para o CLI-sob-teste:
  `BASE_PATH="$RUNTIME_BIN:/usr/bin:/bin"` (`check-release-tag-parity.sh:112`,
  `check-ship-force-parity.sh:115`) — assume que `git` está em `/usr/bin` ou `/bin`, verdade na grande
  maioria de Linux/macOS/Git-for-Windows padrão.
- Nesta VM, `git` está instalado via MSYS2 `clangarm64` (`which git` → `/clangarm64/bin/git`,
  `/usr/bin/git` e `/bin/git` **não existem** — confirmado por `ls`). O Windows nativo TEM
  `C:\Program Files\Git\cmd\git.exe` no `PATH` do sistema (`Test-Path` confirma), mas essa entrada não
  aparece em `BASE_PATH` porque `BASE_PATH` é construído com convenção POSIX/MSYS, não com o `PATH`
  nativo do Windows.
- 🔴 **Ressalva declarada:** medido nesta VM específica, cujo layout de Git (MSYS2 `clangarm64`, sem
  `/usr/bin/git`) pode não ser representativo de um runner de CI Windows padrão (GitHub Actions
  `windows-latest` usa Git for Windows convencional, com `/usr/bin/git` presente). Não generalizar sem
  medir também num runner convencional — mas o padrão de código (`PATH` hardcoded assumindo
  `/usr/bin:/bin`) é, em si, uma dependência de layout não abstraída, então vale reportar
  independentemente da VM.
- Cascata: cada cenário afetado gera de 3 a 5 `FAIL` (um por runtime que chama `git`, mais os
  `go-vs-node/err`/`go-vs-py/err` de divergência de stderr) — por isso 1 causa produz ~130 linhas.

**Categorias já conhecidas, reconfirmadas nesta rodada real (não mais só pela sonda descartável):**
- **Separador de caminho / MSYS path-embedding (Cenário 17, `update-parity/dry-run-write-leak`)** —
  o obstáculo original do gate committed, presente também no modo de enumeração (linha 1 do chunk 0).
- **Dedup de guard cego a `\`** (`git-branch-guard-dedup/baseline-skips-project-entry`,
  `double-slash-tolerance`) — já documentado em
  [dedup-guard-path-cego-a-backslash-no-windows-2026-09-05](../../../vault/notes/dedup-guard-path-cego-a-backslash-no-windows-2026-09-05.md).
- **Mojibake/encoding** (`harness-hooks-parity/*/go-vs-node`, `go-vs-py`, `agent-hooks-parity/amazonq/
  go-vs-py`: "structural drift") — **14 ocorrências nos 8 chunks completos**, mesma família de
  [gate-em-cp1252-tem-duas-falhas-distintas...](../../../vault/notes/gate-em-cp1252-tem-duas-falhas-distintas-crash-de-print-e-mismatch-por-transcodificacao-2026-09-02.md).

**Cluster não triado nesta sessão — "baseline já reprova com o binário real, prova P4 inválida"**
(`setup-sXX-baseline`, **24 ocorrências distintas nos 8 chunks completos**: doctor-parity,
ship-force-parity, release-tag-parity, validate-parity, agent-hooks-parity, agent-models-parity,
push-force-parity, audit-surface, update-parity): o CICLO LIMPO (binário não-corrompido) já
reprova esses `check-*-parity.sh` no Windows — são bugs de produto/gate reais neste SO, não efeito do
modo de enumeração. Cada um pede triagem própria (causa provavelmente distinta por script) — fora do
escopo deste ML (que entrega o instrumento, não os diagnósticos individuais).

**Duas categorias que eram "não diagnosticado" nas pendências do ML-2A e agora têm causa medida
(chunk 7, dados reais — não mais sonda):**

- **Locale/i18n: Node ignora `LANG`/`LC_ALL`** (`validate-ok-message/baseline-byte-identical-and-
  pinned`) — Go e Python emitem `✓ No violations found.` (inglês, respeitando
  `LANG=en_US.UTF-8 LC_ALL=en_US.UTF-8` fixado pelo cenário); Node emite
  `✓ Nenhuma violação encontrada.` (português) — a mesma observação do probe descartável do ML-2A,
  agora reproduzida pelo mecanismo real: Node resolve locale pela UI do SO (pt-BR nesta VM), não pelas
  env vars POSIX.
- **`agent-hooks-parity/credential-guard-present-vacuity/baseline`** (o item 1 das pendências do
  ML-2A, "não investigado") — reproduzido: a MESMA checagem roda 2x no cenário (linha ~72 e ~92 do log
  do chunk 7); a primeira falha (`saiu com 1`) apesar da saída capturada mostrar só `OK` nos
  comparadores estruturais subjacentes (`agent-hooks-parity/claude/go-vs-node`, etc.) — sinal de que o
  `exit 1` não vem do MESMO comando cujo `output:` foi capturado, e sim de um passo anterior dentro do
  mesmo bloco de setup. Causa exata ainda não isolada — precisa de reprodução isolada fora do cenário
  completo (fora do escopo deste ML, que entrega enumeração, não diagnóstico linha-a-linha).

**Residuais não explicados nesta sessão** (não caem em nenhuma categoria acima, 1 ocorrência cada):
`serve-chain-canonical-link/node/edge-baseline` + `no-invented-node-baseline` (Node crash visível no
primeiro log do chunk 0: `MODULE_NOT_FOUND`, padrão `bash -c "$(declare -f fn); fn ..."` chamando
`require()` num contexto Windows — merece investigação própria);
`serve-address-parity/wildcard-bind-regression/python-detects-regression` (bind de rede, possível
diferença de semântica IPv4/IPv6 wildcard no Windows); `git-branch-guard/prose-in-message` e
`stdin-drain-before-noop/detection-catches-epipe-regression` (EPIPE não ocorre — pipe do Windows pode
não sinalizar do mesmo jeito que POSIX); `sandbox-gap-e/direction-a-detected`,
`sandbox-walkdir-reintroduced/direction-b-detected`, `scaffold-mode-check-silenced/direction-a-
detected`, `scaffold-execbit-discriminant-silenced/direction-b-detected`,
`scaffold-update-chmod-removed/direction-c-detected` (chunk 6 — braços de detecção de sabotagem que
não acusam no Windows, possivelmente relacionados a semântica de bit de execução/symlink já conhecida
como divergente no SO, ver
[nem-todo-skip-de-execbit-precisa-do-mesmo-probe-um-era-copia-colada-2026-09-05](../../../vault/notes/nem-todo-skip-de-execbit-precisa-do-mesmo-probe-um-era-copia-colada-2026-09-05.md)
— não confirmado, só levantada a hipótese); `update-harness/populated-harness/go-vs-python` (JSON
diverge — já sinalizado no ML-2A como "divergência real entre runtimes, não vista em Linux/macOS").

**Cobertura desta lista:** 8/8 chunks completos, 440 OK / 512 FAIL, todos os logs por-chunk preservados
em `C:\Users\Lab\falsify-chunks\chunk_N.enum.log` na VM (não copiados para o repo — reproduzíveis a
qualquer momento com o método acima, que é o ponto do ML-2B).

Modo explícito que continua após reprovação e **enumera** em vez de abortar no primeiro FAIL.

🔴 **Três guardas inegociáveis**, herdadas do parecer do `hades-tf` sobre `TRACKFW_FALSIFY_SCRIPT`:
1. **Nunca pode fazer o gate passar.** Havendo qualquer reprovação, o exit final é != 0 —
   independentemente do modo. Modo que transforma vermelho em verde é o pior defeito possível aqui.
2. **Rastro em `stderr`** sempre que ativado, com valor efetivo e default — como o ML-2E fez para os
   overrides do driver.
3. **Não pinável a partir do ambiente para o caminho de produção**: `make quality`/`parity` continuam
   fail-fast. O modo é ferramenta de diagnóstico, não configuração.

**Critérios:** ativado ⇒ enumera e sai != 0 · desativado ⇒ comportamento byte-idêntico ao de hoje ·
falsificação nas duas direções · conjunto em Linux/macOS inalterado.

## O que a Wave 2 mediu, e o que deliberadamente não mediu

**Obstáculo real, no gate commitado:** para no **Cenário 17** (`update-parity/dry-run-write-leak`).
Causa medida, sem código do trackfw envolvido: **o MSYS só converte caminho POSIX quando ele é o
token inteiro de `argv`/env — não quando está embutido numa string maior**
(`python3 -c "...open('$path')..."`). Confirmado idêntico com e sem o shim. É **separador de
caminho**, fora do escopo deste ML — **não corrigido**, corretamente.
Vault: `msys-nao-converte-caminho-embutido-em-string-maior-2026-09-07.md`.

**Além do 17** (sonda não-oficial, ~46 de 194): segundo bug de separador causalmente distinto (Go
emite `/`, Python emite `\` nativo), uma **divergência Go-vs-Python não relacionada a Windows**, um
achado de locale (Node ignora `LANG`/`LC_ALL`) e um de mojibake — este último **declarado não
verificado**, porque veio depois de um patch de sonda.

🔴 **Reconciliação com as 39 residuais: superfícies disjuntas confirmadas** (o `parity` nunca rodou em
Windows), **mas sobreposição de causas NÃO foi medida** — e o agente declarou isso em vez de alegar
zero. Ex.: o mojibake pode ser da família da REQ de cp1252 já existente. **Fica em aberto, escrito.**
