---
title: exit 1 com stderr vazio é assinatura de "não foi o script que rodou" — e o Python lança `bash` por uma ordem de busca diferente de Go e Node no Windows
tags: [windows, teste, subprocess, bash, assinatura, triagem]
date: 2026-09-04
related: [[isolacao-de-home-no-windows-o-defeito-e-divergencia-de-canal-nao-vacuidade-2026-09-03]], [[exited-with-null-e-enoent-e-a-fixture-substitui-o-path-nao-o-job-2026-09-03]], [[job-de-windows-largo-so-reproduz-2-dos-11-defeitos-2026-08-30]]
---

## O que economiza tempo aqui

Toda vez que um teste lança um script shell por subprocesso e o resultado é **exit 1 com `stderr`
vazio**, o instinto (CRLF, caminho errado, bit de execução, `$HOME`) está errado — **cada um desses
mecanismos tem outra assinatura**, e dá para descartá-los sem ter a plataforma onde o defeito
aparece.

Medido localmente com o script real gerado por `_generate_credential_guard_script`, chamado por
`subprocess.run(["bash", path], cwd=…, input=…, capture_output=True, text=True)`:

```
controle (LF, existe):        rc=0    stdout=''  stderr=''
script com CRLF:              rc=2    stdout=''  stderr="line 3: set: pipefail\n: invalid option name"
script ausente:               rc=127  stdout=''  stderr="bash: …: No such file or directory"
sem bit de execução:          rc=0    stdout=''  stderr=''      # `bash <script>` ignora o bit
HOME inexistente / HOME=C:\…: rc=0    stdout=''  stderr=''
stdin vazio / stdin com CRLF: rc=0    stdout=''  stderr=''
stub que sai 1 falando em stdout: rc=1 stdout='…'  stderr=''    # <- a assinatura procurada
```

**Só a última reproduz `rc=1` + `stderr` vazio.** Ou seja: exit 1 silencioso exige um canal que
ninguém está lendo. Se os helpers de teste descartam `stdout` (`code, _out, _err = …`), a mensagem
que explicaria tudo já foi jogada fora — **imprimir `stdout` é o primeiro passo da sonda, não o
último**.

## O contexto onde isso apareceu

Run `33810452454`, job `windows-full-suites`: **50 testes Python** que lançam `bash` sobre os scripts
de guarda falham, todos com `rc=1` e `stderr` vazio, **inclusive** o caminho no-op (`exit 0`) e o
caminho de bloqueio (`exit 2`). Os dois extremos do script devolvem o mesmo 1 — o script não chega a
decidir nada.

A invocação com **stdin diferente** dá o mesmo resultado: `test_git_branch_guard_dedup.py:327`
embute o `rc` na mensagem (`rc=1 stderr=`) e é a única do grupo que **não** passa `input=` (o
comando vai por argumentos, stdin é o herdado do pytest). Ou seja, "o pipe do `input=`" não explica
sozinho.

E há **três** braços, não dois: **Node e Go lançam `bash` sobre os mesmos scripts, no mesmo job, e
executam de verdade** (Node imprime `git push bruto bloqueado` e obtém exit 2; o Go
`TestAttentionScripts_FallbackWithoutJQ` falha exibindo `"Agent needs attention"`, que é o **fallback
interno do próprio script** — prova de que rodou até o `printf` final; e `grep -c "t.Skip("` nos quatro arquivos de teste Go que lançam `bash` dá **0** em todos, então "ausência de `--- FAIL:`" ali significa mesmo passou, não pulado). Logo o ambiente não é a
causa; a diferença está no **ato de lançar**:

| runtime | como resolve `bash` no Windows |
|---|---|
| Go | `exec.LookPath` varre `%PATH%` com `PATHEXT` e chama `CreateProcess` com caminho **absoluto** |
| Node | libuv faz busca própria e também entrega caminho resolvido |
| **CPython** | `subprocess.run(list)` → `CreateProcess` com `lpApplicationName = NULL` → **ordem implícita do Windows** (dir do exe, dir corrente, `System32`, `Windows`, e só então `%PATH%`) |

Essa assimetria é a hipótese sobrevivente — **não medida no Windows** até esta data. `shutil.which`
(que honra `PATHEXT`, precedente em `pypi/tests/test_barrier.py:491-494`) é o equivalente Python de
`LookPath`, e passar o caminho absoluto é o remédio proposto **se** a sonda confirmar.

## Duas armadilhas de triagem nesse mesmo lote

1. **`assertEqual(a, b, "texto")` imprime o literal, não o `stderr`.** As falhas de
   `TestGlobalCredentialGuardScriptBehavior` exibem `"modo block (fallback sem trackfw.yaml)"` — é
   mensagem escrita no teste. Os sítios que provam `stderr` vazio são os que passam `proc.stderr`
   como mensagem (`test_git_branch_guard.py:240` e `:249`, que renderizam `1 != 0 :`).
2. **Contar falhas por arquivo infla o grupo.** Dos 52 vermelhos dos quatro arquivos de guarda,
   3 são de outros grupos (2 de bit de execução em NTFS, 1 de separador `//` vs `\`) e 2 métodos
   aparecem só como `SUBFAILED`, não `FAILED`. A população real é 50, e o critério que a define é
   "lança `bash`" — **nenhum** teste Python que lança `bash` passou, e dos que não lançam só falharam
   esses três, cada um com causa em outro grupo.

Diagnóstico completo, com a sonda de uma linha que separa "não é o `bash`" de "o script morre no
cabeçalho": `docs/qualidade/2026-09-04-grupo-b-bash-do-python-em-windows.md`.
