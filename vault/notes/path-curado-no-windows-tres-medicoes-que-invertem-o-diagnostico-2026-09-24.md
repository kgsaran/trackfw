# PATH curado no Windows: três medições que invertem o diagnóstico

> Data: 2026-09-24 · Autor: `ares-tf` (ML-3A, REQ-2026-09-24 — cluster de Windows, grupos G1 e G2)
> Origem: issue #307 (consumidor externo) + censo `36036473391` · Medições na VM
> `powershell-vm` (Windows 11 ARM64, Git for Windows 2.55, MSYS sem `winsymlinks`).

Três fatos medidos que fazem um gate de Windows **acusar o sujeito errado**. Os três custam
mais de 10 minutos a quem os encontra pela primeira vez, e dois deles **contaminam a própria
medição** de quem investiga.

---

## 1. 🔴 `subprocess.run(env={"PATH": curado})` NÃO cura a resolução no Windows

No Windows o `CreateProcess` resolve o executável pelo `PATH` do processo **CHAMADOR**, não pelo
bloco de ambiente entregue ao filho. Consequência prática: um probe escrito assim

```python
env = dict(os.environ); env["PATH"] = curado
subprocess.run(["gh", "--version"], capture_output=True, env=env)   # ❌ não prova nada
```

**prova o contrário do que afirma**. Medido: o probe entregou ao filho um PATH curado **sem gh** e
mesmo assim obteve `gh version 2.100.0`, enquanto `shutil.which("gh", path=curado)` devolvia
`<não resolvido>` — a resolução veio do PATH do **pai**.

**A forma correta** é pôr o PATH curado no **processo pai** e invocar o interpretador por **caminho
absoluto** (para que ele próprio não dependa do PATH curado — as DLLs dele ficam ao lado do `.exe`):

```bash
PATH="$curado" "$REAL_PYTHON3" - "$curado" "$@" <<'PY'   # ✅
```

Sítio de referência: `native_child_probe`, em `scripts/check-release-tag-parity.sh`.

⚠️ Esta é a mesma classe de contaminação que o autor do #307 registrou ter sofrido: *"a minha
primeira medição disse 'a cópia inicia' e estava contaminada — o PATH do ambiente ainda tinha o
diretório do Python"*. Se a sua medição de PATH curado deu o resultado **confortável**, desconfie
do pai.

---

## 2. 🔴 `git fetch`/`git ls-remote` de remoto LOCAL morre com `0xC0000005` quando falta `sh` no PATH

O Git for Windows spawna o transporte de um remoto de **caminho local** através de um **shell**
(`git-upload-pack '<caminho>'`, visível em `GIT_TRACE=2`). `sh` mora em `/usr/bin`. Sem `sh` no
PATH, o git **não reporta "shell não encontrado"**: ele morre com **violação de acesso** —
`3221225477` (`0xC0000005`) para um filho nativo, `139` (SIGSEGV) sob bash.

Quatro células medidas, com o mesmo repositório e o mesmo git:

| PATH | `git fetch origin --prune` |
|---|---|
| `$RUNTIME_BIN:$GIT_BIN_DIR` | **139 / 0xC0000005** |
| idem **+ diretório só com `ls.exe`** (controle: outro diretório, sem `sh`) | **139** — não é "faltava um diretório qualquer" |
| idem **+ diretório com `sh.exe` + `msys-2.0.dll`** | **0** |
| idem **+ `/usr/bin`** | **0** |

`git --version` passa em **todas** — por isso uma guarda que só roda `git --version` aprova um PATH
onde o `fetch` morre.

**O que isso elimina:** não é carregamento de DLL do próprio `git.exe` (seria `0xC0000135`,
`STATUS_DLL_NOT_FOUND` — código diferente, e o comentário do gate previa esse). Não é resolução do
`git`. É o **filho que o git spawna**. `git ls-remote` morre igual: é o transporte local, não o
`fetch`.

**Regra prática:** um `PATH` curado no Windows que precise rodar `git` contra remoto local tem de
conter `/usr/bin` (MSYS). No trackfw isso é seguro por construção **e por guarda**: o `/usr/bin` do
MSYS não contém `gh`, `glab` nem `az` (medido), e a guarda de vacuidade do gate reprova alto se um
dia contiver.

---

## 3. `ln -s` degrada para CÓPIA; `MSYS=winsymlinks:nativestrict ln -s` cria symlink de verdade

Sem `winsymlinks`, `ln -s` **copia** o alvo. Com alvo **inexistente de propósito** (fixture de
symlink pendurado), a cópia reprova com `No such file or directory` — e sob `set -euo pipefail`
isso **mata o gate inteiro na construção da fixture**. Medido:

```
ln -sf /nonexistent-target link                 -> rc=1, [ -L ] falso
MSYS=winsymlinks:nativestrict ln -sf … link2    -> rc=0, [ -L ] VERDADEIRO
```

Foi o que matou `scripts/check-update-parity.sh` no Cenário 9 do censo (shards 3 e 4): a linha do
`ln` era a **última** que o gate imprimia; os Cenários 9 a 14 nunca rodavam.

**Forma da correção** (`make_dangling_symlink`, em `scripts/check-update-parity.sh`): tenta
`ln -s`, depois `MSYS=winsymlinks:nativestrict ln -s`, e o veredito é `[[ -L ]]` — **nunca** o `$?`
de um comando com cano. Se nenhuma tentativa produzir symlink, o cenário emite **`FAIL` nomeando a
garantia não exercitada** e o gate **segue**. 🔴 `SKIP` não serve: o driver do falsify colhe rótulos
por `^(OK|FAIL|PROOF)` e exige cada literal — uma linha `SKIP` deixa o rótulo como "esperado
AUSENTE" (diagnóstico de chunk morto) **e sai 0**.

---

## 4. A forma comum aos três, e por que ela importa

Nos três casos **um único observável cobre dois estados distintos**, e o diagnóstico publicado
escolhe o errado:

| observável | o que o gate dizia | o que também significa |
|---|---|---|
| probe sai != 0 | "git não resolve no PATH curado" | o **interpretador** do probe não iniciou |
| probe sai != 0 | "gh não está no PATH" (guarda **passava**) | idem — e aqui o sinal era **invertido**: vacuidade em silêncio |
| `stderr` sem a recusa | "o produto não recusou" | um **filho nativo morreu** antes de o produto chegar à recusa |
| `ln` reprova | (nada — o gate morria) | o FS **não representa symlink** |

Quando for consertar um destes, conserte a **mensagem** também: ela tem valor próprio mesmo que o
cenário volte a passar. Foi o que o #307 pediu, e é o que economiza a próxima investigação.
