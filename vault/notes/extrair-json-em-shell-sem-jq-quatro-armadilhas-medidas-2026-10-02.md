# Extrair valor de JSON em shell sem `jq`: quatro armadilhas medidas

> 2026-10-02 | REQ-2026-10-02 (guard de branch falha aberto sem jq) | issue #507 | Wave 0 do hades-tf

O fallback por `sed` do `trackfw-git-branch-guard.sh` falhava aberto porque não interpretava JSON.
Ao desenhar o substituto em `awk`, a Wave 0 mediu quatro armadilhas que valem para **qualquer** script
que extraia JSON sem `jq`:

1. **`RS=""` em awk é modo parágrafo, não "ler tudo".** Uma linha em branco dentro do JSON parte o
   payload em dois registros, e o segundo se perde. Acumule as linhas e processe no `END`.
2. **`strtonum()` não existe no awk do macOS** (BWK, `version 20200816`): erro de execução. Decodificar
   `\uXXXX` exige uma função hexadecimal própria.
3. **O macOS traz `jq` em `/usr/bin/jq`.** Um teste "sem jq" que monta o `PATH` curado incluindo
   `/usr/bin` roda **com** jq e passa vácuo. Afirme `command -v jq` vazio dentro do ambiente curado.
4. **`$()` descarta NUL.** `git push\u0000origin main` vira `git pushorigin main`, e o guard deixa
   passar **inclusive no caminho com `jq`**. Nenhum comando legítimo tem NUL: negue.

E duas regras de equivalência entre os caminhos `jq` e fallback: **chave duplicada, a última vence**
(é o que o `jq` faz), e **a mesma ordem de prioridade entre chaves alternativas**. Se o fallback diverge
do `jq` em qualquer um dos dois, o mesmo payload passa num caminho e é bloqueado no outro.

Também: `\t` entre `git` e `push` (JSON `git\tpush`) não era reconhecido pelo `sed` antigo, outra
forma de falha aberta da mesma causa.

Onde está: `docs/seguranca/2026-10-02-wave0-extrator-json-do-guard.md` (casos C01–C21).
