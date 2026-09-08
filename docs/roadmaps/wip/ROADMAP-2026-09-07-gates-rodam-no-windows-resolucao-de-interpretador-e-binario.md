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
**Status:** 🔄 Em andamento (achados escritos; decisão de escopo pendente do arquiteto) · **Agente:** `ares-tf`

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
**Status:** ⬜ Pendente · **Agente:** `ares-tf`

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
