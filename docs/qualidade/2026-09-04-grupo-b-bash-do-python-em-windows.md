# Grupo B — por que o `bash` do Python devolve exit 1 uniforme no Windows

> Autor: `artemis-tf` (QA) | Data: 2026-09-04
> Roadmap: `docs/roadmaps/wip/ROADMAP-2026-09-03-fechar-os-grupos-de-falha-de-windows-por-causa-raiz.md`, Wave 0, ML-0A
> Escopo: **investigação**. Nenhuma correção aplicada, nenhum arquivo de produto/teste/CI alterado,
> nenhuma operação de git.
> Evidência primária: run `33810452454`, jobs `windows-full-suites` (`100830781056`) e
> `windows-defect-reproduction` (`100830781156`).

## Veredito

**O mecanismo NÃO está identificado.** O que está identificado é o **espaço restante**, reduzido de
"desconhecido" para **duas ramificações mutuamente exclusivas**, com uma medição de uma linha que as
separa. Uma delas é fortemente favorecida pela evidência; a outra não foi falsificada e por isso
**não é apresentada como causa**.

- **(A) o `bash` que o Python lança nunca executa o script** — o processo que roda é outro, sai 1 e
  fala (se fala) em `stdout`, que **todo** teste do grupo descarta.
- **(B) o script começa e morre no cabeçalho**, antes da guarda de projeto, sob `set -euo pipefail`
  — na prática só há um candidato ali: `INPUT=$(cat)`.

Nenhuma das duas foi provada. O que foi **provado por medição** está abaixo: a população real, o
caso `exit 0` isolado, a comparação de três braços (Python × Node × **Go**), e cinco mecanismos
eliminados com a assinatura que cada um produz.

---

## 1. Duas premissas do briefing corrigidas antes de qualquer conclusão

### 1.1 O `exit 0` **não** é a segunda linha do script

O briefing (e o ML) descrevem o caso como "o que deveria sair 0 na segunda linha". O script gerado
(`pypi/trackfw/generators/init_gen.py`, `_CG_HEADER` + `_CG_PROJECT_GUARD`) é:

```sh
#!/usr/bin/env bash
# trackfw credential guard — PreToolUse/PostToolUse hook
set -euo pipefail

INPUT=$(cat)

# Script is intentionally a no-op when executed outside the project root
[ -f "trackfw.yaml" ] || exit 0
```

A guarda é a **linha 8**, e **duas coisas rodam antes dela**: `set -euo pipefail` e `INPUT=$(cat)`.
Isso importa porque é exatamente o intervalo onde a ramificação (B) pode viver: sob `set -e`,
qualquer comando que devolva 1 **sem escrever em stderr** encerra o script com 1 e silêncio — a
assinatura observada. Tratar a guarda como "segunda linha" fecharia (B) por engano.

### 1.2 A população é **50 testes**, não ~56 — e 3 das 52 falhas dos quatro arquivos são de outros grupos

Enumerando os métodos que de fato lançam `bash` e cruzando com a lista `FAILED`/`SUBFAILED` do log
(linhas 15382–15658 do job):

| Arquivo | Falhas no CI | Lançam `bash` | Observação |
|---|---:|---:|---|
| `pypi/tests/test_credential_guard.py` | 24 | **22** | 2 são bit de execução (`test_gera_script_executavel`, `..._sem_guarda_de_projeto`) → grupo do NTFS |
| `pypi/tests/test_credential_guard_sabotage.py` | 10 | **10** | os 3 `test_wiring_referencia_script_real` (sem spawn) **passaram** |
| `pypi/tests/test_git_branch_guard.py` | 14 (+2 métodos só em `SUBFAILED`) | **15** | 1 é bit de execução |
| `pypi/tests/test_git_branch_guard_dedup.py` | 4 | **3** | `test_claude_tolerates_double_slash_in_stored_command` não lança `bash` — é `//` vs `\` → grupo do separador (ML-2A) |
| **Total** | 52 | **50** | |

Dois métodos aparecem como `SUBFAILED` e não como `FAILED` (`TestGitBranchGuardML1AFalsePositive
AndSwitchC::test_quoted_message_then_real_chained_command_still_blocks` e
`TestGitBranchGuardNoOpOutsideProject::test_commit_checkout_branch_switch_without_trackfw_yaml_are_noop`,
4 subtests) — contá-los pela linha `FAILED` subestima o grupo.

🔴 **O que essa enumeração vale além da contagem:** ela é um discriminante, não bookkeeping.
**Nenhum** teste Python que lança `bash` passou nesta execução — 50 de 50 falharam — e dos testes
desses mesmos quatro arquivos que **não** lançam `bash`, **todos passaram exceto três**, cada um com
causa já classificada em outro grupo: `test_gera_script_executavel` e
`test_gera_script_executavel_sem_guarda_de_projeto` (bit de execução em NTFS) e
`test_claude_tolerates_double_slash_in_stored_command` (`AssertionError: True is not false` —
separador `//` vs `\`). A falha é uma função de "lançou `bash` pelo Python", e de mais nada.

Verificação de que a população está fechada: os únicos arquivos de `pypi/tests/` que lançam `bash`
são esses quatro (`grep -rn "\['bash'\|\[\"bash\""`). O quinto hit,
`pypi/tests/test_credential_guard_dedup.py`, é a **chave JSON** `"bash"` do formato de hooks do
Copilot/Kiro, não um spawn — e esse arquivo tem **zero** falhas no run.

---

## 2. O caso `exit 0`, medido isoladamente

`pypi/tests/test_credential_guard.py:121` — `trackfw.yaml` presente no `cwd`, payload sem nenhum
padrão de credencial. O caminho esperado é `[ -n "$MATCH" ] || exit 0`; nem o modo é lido.

```
    def test_sem_match_e_no_op_silencioso(self):
        self._write_yaml()
        code, _out, _err = self._run({"tool_name": "Bash", "tool_input": {"command": "echo hello"}})
>       self.assertEqual(code, 0)
E       AssertionError: 1 != 0
pypi\tests\test_credential_guard.py:121: AssertionError
```

O gêmeo sem projeto, `TestGitBranchGuardNoOpOutsideProject::test_git_push_without_trackfw_yaml_is_noop`
(cwd sem `trackfw.yaml` em nenhum ancestral, caminho `exit 0` da guarda de projeto), também dá
`1 != 0`.

**O que isso discrimina:** o `1` aparece no caminho em que o script não faz nada — não há regex, não
há leitura de YAML, não há `date`, não há `mkdir`. Logo o defeito **não** está na lógica do script.
E aparece igualmente no caminho oposto (`test_git_push_with_trackfw_yaml_still_blocks` esperava 2,
obteve 1). Os dois extremos do script devolvem o mesmo `1`: **o script não chega a decidir nada**.

### 2.1 `stderr` está vazio — e isso é medição, não inferência

A maioria dos testes descarta `_err`. Os dois sítios que **passam `proc.stderr` como mensagem da
asserção** são os que provam:

```
pypi/tests/test_git_branch_guard.py:240 →  E   AssertionError: 1 != 0 :
pypi/tests/test_git_branch_guard.py:249 →  E   AssertionError: 1 != 2 :
```

O `:` final é a mensagem — vazia. (Cuidado: as falhas de `TestGlobalCredentialGuardScriptBehavior`
exibem textos como `"modo block (fallback sem trackfw.yaml)"`; **são literais escritos no teste**,
não `stderr`, e não provam nada sobre o canal de erro.)

Confirmação por varredura: em **todo** o step Python do job, nenhuma linha contém
`trackfw-credential-guard:` ou `bloqueado` fora de trechos de código-fonte ecoados pelo traceback.
Os scripts de guarda **nunca falaram** durante a suíte Python.

### 2.2 A terceira testemunha — e ela vem de uma invocação com **stdin diferente**

`pypi/tests/test_git_branch_guard_dedup.py:327` embute `rc` **e** `stderr` no texto da falha, e as 3
falhas de `TestGBGDedupMessageOnce` no log mostram:

```
E   AssertionError: unexpected script outcome for
    C:\Users\RUNNER~1\AppData\Local\Temp\tmpuf4yd2si/scripts/trackfw-git-branch-guard.sh: rc=1 stderr=
```

Vale mais do que uma terceira confirmação de "stderr vazio": **`_run_entries` não passa `input=`** —
o comando vai por **argumentos** (`['bash', script, 'git', 'push']`) e o `stdin` do filho é o
herdado do pytest, não um pipe fechado após a escrita. É a única variação de fiação de stdin dentro
dos 50, e o resultado é **o mesmo `rc=1` silencioso**. Consequência: a ramificação (B) fica mais
estreita — se o cabeçalho morre em `INPUT=$(cat)`, morre **independentemente de como o stdin foi
ligado**, o que torna "o pipe do `input=` do Python" uma explicação insuficiente por si só.

Observação adicional, não conclusiva: o caminho exibido mistura separadores e nome curto 8.3
(`C:\Users\RUNNER~1\…\tmpuf4yd2si/scripts/…`). `bash` aceita as duas formas, e as outras duas falhas
do mesmo teste usam caminho com `\` puro e dão o mesmo `rc=1` — então a mistura **não** é o
discriminante. Fica registrado só para não ser "descoberto" de novo como pista falsa.

🔴 **`stdout` nunca foi lido.** Os helpers capturam (`capture_output=True`) e descartam (`_out`).
Exit 1 + `stderr` vazio + `stdout` não lido é exatamente a forma de uma mensagem enviada a
**stdout**. É o buraco central da medição atual e o primeiro item da sonda proposta na §5.

---

## 3. Comparação lado a lado — e ela tem **três** braços, não dois

O briefing aponta o Node como discriminante. Existe um terceiro, que ninguém tinha olhado: **o Go
também lança `bash` sobre os mesmos scripts gerados, no mesmo job, e passa.**

| | Python | Node | Go |
|---|---|---|---|
| Chamada | `subprocess.run(["bash", script], cwd=…, input=json, capture_output=True, text=True)` | `spawnSync('bash', [script], { cwd, input, encoding: 'utf8' })` | `exec.Command("bash", script)` |
| Sítios | `test_credential_guard.py:106,330`; `test_credential_guard_sabotage.py:69`; `test_git_branch_guard.py:126,165`; `test_git_branch_guard_dedup.py:316` | `npm/tests/credential_guard.test.js:80,171`; `credential_guard_sabotage.test.js:63`; `git_branch_guard.test.js:102` | `internal/generators/credential_guard_test.go:447`; `git_branch_guard_test.go:490`; `git_branch_guard_dedup_test.go:440`; `scaffold_test.go:225,398` |
| Script | gerado pelo gerador **Python** | gerado pelo gerador **Node** | gerado pelo gerador **Go** |
| Resultado no run `33810452454` | **50/50 falham**, exit 1, `stderr` vazio | **suíte de sabotagem 13/13 verde** (`ok 264`–`ok 276`) | **verde** nos testes que executam o script |

Evidências de que Node e Go realmente **executaram** o script (não passaram por vacuidade):

- Node, `git_branch_guard.test.js`, saída do próprio script no log:
  `# trackfw: git push bruto bloqueado. Use \`trackfw push\` …` — a mensagem só existe no fim do
  script, depois do parser de comando.
- Node, sabotagem: `ok 266 - Sabotage/ClaudeCode: JWT sintético no comando Bash -- modo block sai
  com exit 2` — o **exit 2** vem do `exit 2` do script.
- Go, `TestAttentionScripts_FallbackWithoutJQ` **falhou**, e a falha prova execução: obteve
  `Message = "Agent needs attention"`, que é o **fallback interno do script** quando o `python3` do
  ramo sem `jq` não resolve. O script rodou até o `printf` final e escreveu o JSON. (Essa falha é de
  outro grupo — `python3` ausente no PATH do runner — e não deve ser triada aqui.)
- Go, os demais testes que executam o script (`TestGitBranchGuard_EnvVarFallback_Blocks`,
  `TestGBGDedup_FailOpen_CorruptedGlobalFile`,
  `TestGlobalCredentialGuardScript_WritesAttentionOnlyWhenRoadmapsDirExists`,
  `TestAttentionScripts_ExecutionContract`) **não aparecem em nenhum `--- FAIL:`** do pacote
  `internal/generators` (que como um todo reprovou — logo os testes rodaram). ⚠️ `go test` sem `-v`
  também não imprime `SKIP`, então "ausência de FAIL" só vale se eles não forem pulados: verificado —
  `grep -c "t.Skip("` nos quatro arquivos dá **0, 0, 0, 0**, e não há `runtime.GOOS` nem
  `testing.Short` em nenhum deles (o único `runtime.GOOS == "windows"` do pacote está em
  `scaffold_doctor_test.go`, que não lança `bash`).

**A diferença é o achado, e ela ficou pequena:** o script é o mesmo (§4.1 prova isso no Windows), o
`cwd` é o mesmo, o payload é o mesmo, o argumento é o mesmo caminho absoluto do Windows. Sobra **o
ato de lançar `bash`** — e é exatamente aí que os três runtimes divergem por construção:

- **Go**: `exec.Command` → `exec.LookPath("bash")` varre `%PATH%` respeitando `PATHEXT` e chama
  `CreateProcess` já com o **caminho absoluto resolvido**.
- **Node**: `spawnSync` sem `shell` → libuv faz a própria busca e também entrega caminho resolvido.
- **CPython**: `subprocess.run(["bash", …])` sem `shell` → `list2cmdline` e `CreateProcess` com
  `lpApplicationName = NULL`. A resolução passa a ser a **ordem implícita do Windows**, que consulta
  o diretório do executável do processo, o diretório corrente, `System32` e `Windows` **antes** do
  `%PATH%`.

Essa assimetria é a única diferença estrutural encontrada entre os três braços, e é a base da
ramificação (A). **Ela não foi medida no Windows** — por isso continua hipótese, não causa. A §5 diz
como medi-la em uma linha.

---

## 4. O que foi eliminado, e como

### 4.1 Eliminados por medição **no próprio Windows** (job `windows-defect-reproduction`)

| Hipótese | Medição | Resultado |
|---|---|---|
| O gerador Python escreve o script com **CRLF** no Windows | ITEM 5 da suíte de reprodução: `trackfw init` real + varredura de bytes | `scripts\trackfw-credential-guard.sh: crlf=False bytes_sample=b'#!/usr/bin/env bash\n…'`, `scripts_checked=5 scripts_with_crlf=0`, `VERDICT=ABSENT` — **morta** |
| Não há shell POSIX utilizável no runner | ITEM 7: `Evidencia auxiliar (ML-1A): sh no PATH do runner -> presente`, `sh-ran-ok output="trackfw-sh-check-ok\n"` | **morta** |

A primeira também é confirmada por leitura da fonte: `_generate_credential_guard_script`,
`_generate_git_branch_guard_script` e `generate_global_credential_guard_script` abrem com
`newline="\n"` explícito (`init_gen.py:1935, 1952, 1995`).

### 4.2 Eliminados por **assinatura** — reprodução local (macOS) do que cada mecanismo produziria

O ponto não é "funciona no macOS"; é que **cada mecanismo candidato tem uma assinatura de saída
distinta**, e nenhuma delas é a observada (`rc=1`, `stderr` vazio).

```
A control (LF, existe):        (0, stdout='',  stderr='')
B CRLF:                        (2, stdout='',  stderr="…/crlf.sh: line 3: set: pipefail\n: invalid option name\n")
C script ausente:            (127, stdout='',  stderr="bash: …/nope.sh: No such file or directory\n")
D sem bit de execução:         (0, stdout='',  stderr='')        # `bash script` ignora o bit
E stub que sai 1 via stdout:   (1, stdout='no distributions installed\n', stderr='')
```

| Hipótese | Assinatura que produziria | Observado | Veredito |
|---|---|---|---|
| CRLF no script | rc **2**, stderr com `invalid option name` | rc 1, stderr vazio | **eliminada** |
| Caminho do script errado / não encontrado | rc **127**, stderr `No such file or directory` | rc 1, stderr vazio | **eliminada** |
| Bit de execução ausente em NTFS | rc **0** (irrelevante para `bash <script>`) | — | **eliminada** (é o grupo do NTFS, e explica só 3 das 52 falhas dos 4 arquivos) |
| Tradução de newline por `text=True` no stdin | o payload tem **0** `\n` e **0** `\r` (medido: `len=62`, `payload.count("\n")==0`) — não há o que traduzir; e injetar CRLF no stdin dá rc 0 | rc 1 | **eliminada** |
| `HOME` de sessão herdado pelo filho (`conftest.py` aponta `HOME` **e** `USERPROFILE` para um dir sintético) | rc 0 com `HOME` inexistente **e** com `HOME` no formato `C:\…` | rc 1 | **eliminada como causa suficiente** |
| stdin vazio | rc 0 | rc 1 | **eliminada** |
| "o `bash` não funciona no runner" | contradiz Node e Go executando o mesmo script no mesmo job | — | **eliminada** (§3) |
| Algo escreve em stdout e sai 1 | rc **1**, stderr **vazio** | **idêntico** | **compatível — é o único candidato local que reproduz a assinatura** |

O último não prova (A): "escreve em stdout e sai 1" descreve tanto um `bash` que não é `bash` quanto
um cenário improvável dentro do script. Prova apenas que **a assinatura observada exige um canal que
ninguém está lendo**.

### 4.3 O que sobrou de pé

- **(A)** `bash` resolve, para o Python, para um executável que não é o `bash` que Node e Go
  encontram — a ordem de busca do `CreateProcess` sem `lpApplicationName` difere da de
  `LookPath`/libuv. Compatível com tudo que foi medido; **não medida no Windows**.
- **(B)** o script arranca e morre entre `set -euo pipefail` e a guarda de projeto. O único comando
  nesse intervalo é `INPUT=$(cat)`. Sob `set -e`, um `cat` que devolva 1 sem escrever encerra com
  exit 1 e silêncio. **Não falsificável no macOS** (aqui `cat` sempre funciona) — é por isso que (B)
  continua aberta, e não porque haja evidência a favor dela. O que a §2.2 acrescenta: se (B) for a
  causa, ela **não** depende da fiação do stdin — a invocação sem `input=` (stdin herdado do pytest)
  devolve o mesmo `rc=1` silencioso.

---

## 5. O que falta medir, e exatamente como

Uma sonda, três saídas, e as duas ramificações se separam. **Ela não foi escrita nesta ML** (é
investigação; escrever no CI é mudança). Lugar natural: um **ITEM 12** em
`scripts/windows-repro/run.ps1`, seguindo o padrão `Add-Result` /
`REPRODUCED|ABSENT|INCONCLUSIVE` dos 11 itens existentes — não um arquivo novo.

🔴 **Escrito para `shell: pwsh`**: nada de continuação com `\` (o PowerShell continua linha com
**crase**, e quebraria o comando na primeira quebra), e o corpo Python vai por **arquivo temporário
via here-string**, não por `python -c` multilinha. Copiar e colar como está.

```powershell
# 12a — que 'bash' cada runtime enxerga
where.exe bash                                  # TODAS as ocorrências, na ordem de resolução
python -c "import shutil; print('py shutil.which ->', shutil.which('bash'))"
node -e "console.log('node PATH[0..] ->', process.env.PATH.split(';').slice(0,8).join(' | '))"

# 12b — controle mínimo: o Python consegue rodar QUALQUER coisa por bash? (uma linha física)
python -c "import subprocess as s; p = s.run(['bash','-c','echo PROBE_OUT; echo PROBE_ERR >&2'], capture_output=True, text=True); print('rc', p.returncode, 'out', repr(p.stdout), 'err', repr(p.stderr))"

# 12c — o script real, com stdout IMPRESSO (o canal que os 50 testes descartam)
$probe = Join-Path $env:RUNNER_TEMP "item12c.py"
@'
import json, os, subprocess, sys, tempfile
sys.path.insert(0, "pypi")
from trackfw.generators.init_gen import _generate_credential_guard_script
d = tempfile.mkdtemp()
_generate_credential_guard_script(d)
with open(os.path.join(d, "trackfw.yaml"), "w", encoding="utf-8") as f:
    f.write("roadmap_dir: docs/roadmaps\n")
payload = json.dumps({"tool_name": "Bash", "tool_input": {"command": "echo hello"}})
script = os.path.join(d, "scripts", "trackfw-credential-guard.sh")
p = subprocess.run(["bash", script], cwd=d, input=payload, capture_output=True, text=True)
print("rc ", p.returncode)
print("out", repr(p.stdout))
print("err", repr(p.stderr))
'@ | Set-Content -Path $probe -Encoding utf8
python $probe

# 12d — o mesmo script, pelo Go (controle positivo já observado; torna a comparação explícita)
go test ./internal/generators/ -run "TestAttentionScripts_ExecutionContract" -v
```

Leitura do resultado — **decide sem ambiguidade**:

| Saída de 12b | Conclusão |
|---|---|
| `rc 1`, `out` com texto qualquer (ex.: mensagem de WSL), `err ''` | **(A) confirmada.** O `bash` do Python não é um `bash`. 12a diz qual é. |
| `rc 0`, `out 'PROBE_OUT\n'`, `err 'PROBE_ERR\n'` | **(A) morta.** Vai para 12c: se lá der `rc 1`, o defeito está no script sob a invocação Python → **(B)**, e o passo seguinte é `bash -x` para ver em qual linha morre. |

**Custo:** um job de Windows já existente, sem alterar nenhum teste. Nenhuma das três linhas toca
código de produto.

Se o resultado for (A) e o remédio exigir tocar os quatro arquivos de teste, isso é **REQ própria**
— 50 testes em 4 arquivos, com um contrato ("como um teste Python lança um interpretador externo")
que hoje não está escrito em lugar nenhum.

---

## 6. Remédio **proposto**, não aplicado

Por ramificação, e nesta ordem — nada disso é para ser feito antes da sonda da §5.

**Se (A)** — resolver o interpretador explicitamente antes de lançar, como Go e Node já fazem por
construção: `shutil.which("bash")` (que **honra `PATHEXT` no Windows**, precedente já documentado em
`pypi/tests/test_barrier.py:491-494`) e passar o **caminho absoluto** como `argv[0]`, com `skip` de
mensagem explícita se `which` devolver `None`. Um helper único por arquivo — os seis sítios de spawn
estão listados na tabela da §3.

**Se (B)** — não há remédio antes de saber qual comando devolve 1. `bash -x` no 12c dá a linha.

🔴 **Nos dois casos o remédio é de harness, não de produto.** Medido: **nenhum** módulo de
`pypi/trackfw/` lança `bash` ou `sh` (`grep -rn "subprocess.run\|Popen" pypi/trackfw/` cruzado com
`bash`/`sh` → zero hits). Quem executa esses scripts em runtime real é o CLI de agente
(Claude/Cursor/Kiro), não o CLI Python. Consequência para a Wave de recontagem: **fechar o grupo B
tira ~50 vermelhos sem esconder defeito de produto** — mas isso só vale se o veredito for (A) ou um
(B) confinado ao harness. Se 12c mostrar que o **script** morre sob uma invocação legítima do
Windows, o grupo deixa de ser harness e vira defeito do produto, com severidade de segurança (o
guard viraria fail-open silencioso). **Essa distinção é o motivo de a sonda existir.**

---

## 7. Residuais declarados

- **Ordem de busca do `CreateProcess` não medida.** A descrição do comportamento de `LookPath`,
  libuv e `lpApplicationName = NULL` é leitura de contrato de plataforma, não medição neste runner.
  É o que a sonda 12a/12b converte em fato.
- **O braço Go só foi lido pela ausência de `--- FAIL:`.** `go test` sem `-v` não imprime PASS;
  "não falhou" é sólido (o pacote rodou e reprovou por outros testes), mas um `-run` dirigido com
  `-v` seria mais forte. Não rodei — não tenho Windows.
- **Node tem 2 falhas próprias** no arquivo `credential_guard.test.js` (o arquivo aparece como
  `not ok 20`). O briefing as classifica como bit de execução; **verifiquei em vez de herdar** — os
  dois subtests marcados `✗` no log são `generateCredentialGuardScript cria
  scripts/trackfw-credential-guard.sh executável` e `generateGlobalCredentialGuardScript cria
  ~/.trackfw/scripts/trackfw-credential-guard.sh executável`. São de fato o grupo do NTFS, e não
  tocam a execução do script.
- **A contagem de 50 é do run `33810452454`.** Qualquer recontagem posterior tem de repetir a
  enumeração da §1.2, não reaproveitar o número — foi exatamente reaproveitar contagem que produziu
  o "~56".
