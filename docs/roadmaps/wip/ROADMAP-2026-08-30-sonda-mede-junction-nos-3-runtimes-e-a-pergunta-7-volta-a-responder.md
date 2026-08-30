---
status: wip
date: 2026-08-30
req: "docs/req/REQ-2026-08-30-sonda-nao-responde-a-pergunta-7-e-nao-mede-junction-em-node-e-python-a-guarda-de-symlink-pode-estar-furada-nos-3-clis-no-windows.md"
squad: "hades-tf, ares-tf, zeus-tf"
---

# Roadmap: Sonda mede junction nos 3 runtimes e a pergunta 7 volta a responder

> Created: 2026-08-30 | Status: wip

## Context

REQ: `REQ-2026-08-30-sonda-nao-responde-a-pergunta-7-e-nao-mede-junction-em-node-e-python-a-guarda-de-symlink-pode-estar-furada-nos-3-clis-no-windows.md`

A sonda mediu, na primeira execução pós-merge (run `33338382066`), que **o `os.Lstat` do Go não
marca junction como `ModeSymlink` — marca como `ModeIrregular`** —, e que junction é criada por
`mklink /J` **sem privilégio algum**. Toda guarda que testa `Mode()&os.ModeSymlink` é cega para ela.

Duas lacunas impedem escrever a correção: a **pergunta 7 falhou** (vírgula do PowerShell virou
array em `git update-index --cacheinfo`), e **Node e Python nunca foram medidos**. Sob a regra dura
de paridade, "defeito só do Go" e "divergência entre os três" são correções diferentes.

**Este roadmap não corrige guarda nenhuma.** Ele produz o número que decide a correção.

## Acceptance Criteria

- [ ] A pergunta 7 responde, com prova de que o argumento chega íntegro (não só que parou de errar)
- [ ] Junction medida em Node e em Python, com valores crus
- [ ] Tabela comparativa `runtime × (arquivo | symlink | junction)` legível sem cruzar logs
- [ ] A sonda continua **sem veredito** — nenhum `exit 1` por causa do valor medido
- [ ] Nota de correção na `REQ-2026-08-29` e nota de vault
- [ ] `actionlint` limpo, `make quality` verde, `quality.yml` **não** alterado

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat Model
> Dependências: nenhuma. Bloqueia toda a implementação.

### ML-0A — Modelo de ameaça da extensão da sonda
**Status:** ✅ Concluído
**Agente:** `hades-tf`
**Files affected:** nenhum (documento em `docs/seguranca/`)
**Contexto que importa:** a sonda **cria** links no runner (symlink via `os.Symlink`, junction via
`mklink /J`, e agora um symlink versionado via plumbing do git). Ela vai passar a criar mais. Roda
em `workflow_dispatch` com **log público**.
**Actions:**
1. **Completude da enumeração** — a lista de superfícies deste roadmap está completa? Não se limite
   aos arquivos citados pela REQ: procure no repositório outros pontos que criem reparse point,
   symlink ou junction, e outros que decidam por `ModeSymlink`/`isSymbolicLink`/`islink`. Mostre a
   busca, não a conclusão.
2. **Modelo de ameaça** — quem esvazia esta Wave 0 sem quebrar regra escrita, e como? Em especial:
   a sonda pode ser levada a criar link **fora** de `RUNNER_TEMP`/workspace? O log público passa a
   revelar algo novo?
3. **Alvos de falsificação nas duas direções** — para cada superfície, o que quebra quando o
   comportamento regride, e o que quebra quando regride ao contrário (ex.: a sonda ganhar veredito,
   ou passar a esconder um valor cru atrás de interpretação).
4. **Residual declarado** — o que este desenho aceita não cobrir.
**Critérios de aceite:**
- [x] As quatro seções respondidas com evidência, não asserção de uma linha
- [x] Nenhuma linha de implementação escrita neste ML
- [x] Parecer em `docs/seguranca/2026-08-30-modelo-de-ameaca-da-extensao-da-sonda.md`

**Gates da wave:**
```bash
test -f docs/seguranca/2026-08-30-modelo-de-ameaca-da-extensao-da-sonda.md
! grep -qi "placeholder" docs/seguranca/2026-08-30-modelo-de-ameaca-da-extensao-da-sonda.md
grep -q "Residual" docs/seguranca/2026-08-30-modelo-de-ameaca-da-extensao-da-sonda.md
```

#### Resultado do ML-0A (hades-tf, 2026-08-30) — auditado pelo arquiteto

**A Wave 0 fez o que existe para fazer: derrubou uma classificação minha antes que virasse roadmap.**

Eu pedi que ele *contestasse* minha separação das guardas em três classes, não que a confirmasse.
Ele confirmou duas e **corrigiu a terceira** — e a correção é a que mais importa:

| CLI | Freio contra junction em `removeEmptyAncestors`/`cleanEmpty`/`_remove_empty` |
|---|---|
| Go `manager.go:582` | `if !info.IsDir() { return nil }` → **para** (`ModeDir=false` para junction) |
| Node `manager.js:420` | **nenhum teste de `isDirectory()`** — depende de `readdirSync(dir).length` |
| Python `manager.py:589` | **nenhum teste de `IsDir` nem de vazio** — só `except OSError` em volta do `rmdir()` |

Verifiquei lendo os três. O freio acidental **existe só no Go**. Consequência: no Go o remédio é
tornar intencional um freio que já existe; em Node e Python pode ser preciso **adicionar** um freio
que hoje não existe. Eu teria escrito o roadmap de correção errado em dois dos três CLIs — e ele
passaria pelos gates de paridade, porque paridade mede se as implementações concordam entre si, não
se o contrato está correto. A REQ foi corrigida; a tabela original ficou registrada como errada em
vez de reescrita.

**Segundo achado, contra um AC meu:** `probe.go:117,147` usa `os.MkdirTemp("", ...)`, que resolve
para `%TEMP%` e **não** para `RUNNER_TEMP` — o log do run `33338382066` mostra
`C:\Users\RUNNER~1\AppData\Local\Temp\...` enquanto `RUNNER_TEMP` é `D:\a\_temp`. O AC que eu
escrevi (*"todo link fica dentro de `RUNNER_TEMP`/workspace"*) **já era falso quando foi escrito**, e
um agente diligente o teria "satisfeito" por asserção. Reescrito para exigir **medição impressa**.

**Duas superfícies fora da minha enumeração:** `update.go:2323` `copyPath` / `update.py:667`
`_copy_path` (cegueira de junction no sandbox do `--dry-run`; classe DoS-local, inferida e não
medida ao vivo) e `discover.js:593` `writeCIWorkflowForce`, **código morto sem chamador nos 3 CLIs**.
Nenhuma bloqueante; ambas entram na REQ de correção.

**Residual aceito:** `%TEMP%` ≠ `RUNNER_TEMP`; o comportamento de libuv e CPython sobre junction é o
que a Wave 1 existe para medir, não para prever; a verificação é estruturalmente pós-merge.

**Nota de processo:** ele não escreveu em `docs/agents-working-context.md` porque meu prompt
restringiu a escrita ao documento de segurança, e sinalizou o conflito em vez de silenciosamente
desobedecer uma das duas instruções. Registro eu, na auditoria — mas o prompt estava mal formulado:
restrição de escrita não deveria colidir com obrigação de protocolo do role.

## Wave 1 — A medição (ML único)
> Dependências: Wave 0 completa. **ML único porque as três ações tocam `windows-probe.yml`** — dois
> agentes no mesmo arquivo é a colisão que este projeto proíbe.

### ML-1A — Pergunta 7 responde e junction é medida em Node e Python
**Status:** ⬜ Pendente
**Agente:** `ares-tf`
**Files affected:** `.github/workflows/windows-probe.yml`, `scripts/windows-repro/node/` (arquivo
novo de sonda), `scripts/windows-repro/python/` (arquivo novo de sonda). **Não tocar**
`quality.yml`, nem `run.ps1`, nem `checks.go`/`checks.js`/`checks.py` (esses são da camada 2, que é
regressão, não sonda).
**Actions:**
1. **Corrigir a pergunta 7.** `git update-index --add --cacheinfo 120000,$blob,mylink` falhou porque
   **em PowerShell a vírgula constrói array** — chegaram três argumentos ao git. Passar como
   **string única**. Provar que o argumento chega íntegro, não apenas que o comando parou de errar.
2. **Junction em Node**: `lstatSync()` sobre junction, symlink real e arquivo comum, imprimindo
   `isSymbolicLink()`, `isDirectory()`, `isFile()` **crus**. Mesmo formato comparativo do braço Go.
3. **Junction em Python**: `os.path.islink()`, `os.lstat().st_mode`, `stat.S_ISLNK()` e
   `os.readlink()` (com o erro, se levantar), sobre os mesmos três alvos.
4. **`rmdir` sobre junction vazia nos 3 runtimes** — `os.Remove` (Go), `fs.rmdirSync` (Node),
   `Path.rmdir()` (Python) sobre uma junction cujo alvo está vazio. Imprimir sucesso/erro **cru** e,
   depois, se a **junction** sumiu e se o **alvo** sobreviveu. Entrou no escopo por causa do achado
   do ML-0A: `manager.py:589` depende **só** de `except OSError` para parar, então *"o `rmdir` teve
   sucesso?"* é literalmente o discriminante entre "Python para" e "Python sobe removendo
   ancestrais". A fixture de junction já existirá — custo marginal baixo, valor alto.
5. **Tabela final** `runtime × (arquivo | symlink | junction)` — é o artefato que a REQ de correção
   vai citar. Sem ela, comparar exige cruzar log à mão.
**Critérios de aceite:**
- [ ] AC1, AC2, AC3, AC4, AC5 da REQ
- [ ] 🔴 **AC6 da REQ — a sonda continua SEM veredito.** Nenhuma pergunta nova emite pass/fail nem
      `exit 1` por causa do *valor* medido. Sonda com veredito vira job de regressão disfarçado.
- [ ] 🔴 **Medir, não afirmar, onde os links são criados.** O AC original dizia *"todo link fica
      dentro de `RUNNER_TEMP`/workspace"* — **já era falso quando eu o escrevi**: `probe.go:117,147`
      usa `os.MkdirTemp("", ...)`, que resolve para `%TEMP%`
      (`C:\Users\RUNNER~1\AppData\Local\Temp`), enquanto `RUNNER_TEMP` é `D:\a\_temp`. Achado
      do `hades-tf`, confirmado no log do run `33338382066`. O critério passa a ser: **cada braço
      imprime o tempdir resolvido ao lado de `$env:RUNNER_TEMP`**, para que a diferença seja lida no
      log em vez de asseverada num AC. Consequência prática é baixa (diretório efêmero e vazio); o
      problema era o AC não ser verificável por construção.
- [ ] `actionlint` limpo; `make quality` verde; `quality.yml` byte-idêntico

**Gates da wave:**
```bash
actionlint .github/workflows/windows-probe.yml
git diff --quiet origin/main -- .github/workflows/quality.yml
! grep -nE "^\s*exit 1" .github/workflows/windows-probe.yml
```

## Wave 2 — Governança (ML único)
> Dependências: Wave 1 completa.

### ML-2A — Nota de correção na REQ-2026-08-29 e nota de vault
**Status:** ⬜ Pendente
**Agente:** arquiteto (`zeus-tf`)
**Files affected:**
`docs/req/REQ-2026-08-29-namespace-de-agente-nao-declarado-em-agents-fica-invisivel-e-o-validate-reporta-limpo-sem-olhar.md`,
`vault/notes/` (nota nova), `vault/notes/index.md`
**Actions:**
1. **Nota de correção** na `REQ-2026-08-29`, cujo **AC12** está `Done` afirmando *"a enumeração não
   segue symlink, verificável nos 3 CLIs"* — verdadeiro no Linux, **falso no Windows para
   junction**. Anexar nota com link para o run que mediu. **Não reabrir a REQ, não reescrever o AC
   original** — o histórico fica.
2. **Nota de vault** sobre `Lstat`/junction/`ModeIrregular`, contendo a separação que custou a
   primeira leitura errada: guarda de **diretório** (furada), guarda salva por acidente via
   `IsDir()`, e guarda de **folha que nunca olha ancestral** — esta última independente de
   plataforma, porque `Lstat` só não segue o **último** componente.
**Critérios de aceite:**
- [ ] AC7 e AC8 da REQ
- [ ] A nota separa as três classes de guarda — a versão ampla demais ("todas furadas") é errada
- [ ] Nota linkada em `vault/notes/index.md`

**Gates da wave:**
```bash
grep -q "junction" docs/req/REQ-2026-08-29-namespace-de-agente-nao-declarado-em-agents-fica-invisivel-e-o-validate-reporta-limpo-sem-olhar.md
grep -q "junction" vault/notes/index.md
```

## Verificação diferida para pós-merge — NÃO é critério de aceite de nenhum ML

`workflow_dispatch` só é acionável a partir da branch default, então **a sonda estendida não pode
ser executada nesta branch**. A AC9 da REQ é estruturalmente inverificável antes do merge.

| Ação | Gatilho | Dono | O que fecha |
|---|---|---|---|
| Disparar `windows-probe.yml` | merge deste PR em `main` | arquiteto | AC9 — produz a tabela `runtime × alvo` e decide se a correção é só do Go ou divergência dos 3 |

## Barreira final

Revisão `hefesto-tf` e `hades-tf`, auditoria do arquiteto, `barrier --wave 2`. **A medição só existe
depois do merge** — este PR entrega o instrumento, não o número.
