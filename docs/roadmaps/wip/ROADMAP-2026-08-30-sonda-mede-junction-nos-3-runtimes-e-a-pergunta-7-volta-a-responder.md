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
**Status:** ✅ Concluído
**Agente:** `ares-tf`
**Files affected:** `.github/workflows/windows-probe.yml`, `scripts/windows-repro/go/probe.go`
(braço Go da **sonda** — é escopo, ao contrário de `checks.go`), `scripts/windows-repro/node/`
(arquivo novo de sonda), `scripts/windows-repro/python/` (arquivo novo de sonda). **Não tocar**
`quality.yml`, nem `run.ps1`, nem `checks.go`/`checks.js`/`checks.py` — esses três são a **camada 2**,
que é regressão com veredito, não sonda. A distinção importa: `probe.go` imprime valor cru sem
veredito; `checks.go` decide `REPRODUCED`/`ABSENT`.
**Actions:**
1. **Corrigir a pergunta 7.** `git update-index --add --cacheinfo 120000,$blob,mylink` falhou.
   ⚠️ **O mecanismo escrito aqui originalmente estava errado** — ver correção na REQ: em **modo
   argumento** a vírgula **não** constrói array; o token chega como **uma** string com `$blob`
   **literal, não interpolado**. Remédio: **citar** (`"120000,$blob,mylink"`), não desmembrar.
   Provar que o argumento chega íntegro, não apenas que o comando parou de errar.
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
- [x] AC1, AC2, AC3, AC4, AC5 da REQ
- [x] 🔴 **AC6 da REQ — a sonda continua SEM veredito.** Nenhuma pergunta nova emite pass/fail nem
      `exit 1` por causa do *valor* medido. Sonda com veredito vira job de regressão disfarçado.
- [x] 🔴 **Medir, não afirmar, onde os links são criados.** O AC original dizia *"todo link fica
      dentro de `RUNNER_TEMP`/workspace"* — **já era falso quando eu o escrevi**: `probe.go:117,147`
      usa `os.MkdirTemp("", ...)`, que resolve para `%TEMP%`
      (`C:\Users\RUNNER~1\AppData\Local\Temp`), enquanto `RUNNER_TEMP` é `D:\a\_temp`. Achado
      do `hades-tf`, confirmado no log do run `33338382066`. O critério passa a ser: **cada braço
      imprime o tempdir resolvido ao lado de `$env:RUNNER_TEMP`**, para que a diferença seja lida no
      log em vez de asseverada num AC. Consequência prática é baixa (diretório efêmero e vazio); o
      problema era o AC não ser verificável por construção.
- [x] `actionlint` limpo; `make quality` verde; `quality.yml` byte-idêntico

**Gates da wave:**
```bash
actionlint .github/workflows/windows-probe.yml
git diff --quiet origin/main -- .github/workflows/quality.yml
! grep -nE "^\s*exit 1" .github/workflows/windows-probe.yml
```

#### Resultado do ML-1A (ares-tf, 2026-08-31) — auditado pelo arquiteto

**Entregue:** pergunta 7 corrigida + perguntas 8/9/10/11 novas em `windows-probe.yml`;
`rmdir-junction` e `table` em `probe.go`; `probe.js` e `probe.py` novos.

**Gates rodados por mim:** `actionlint` limpo · `quality.yml` **byte-idêntico** a `origin/main` ·
nenhum `exit 1` no workflow · `go build ./...` · **cross-build `GOOS=windows`** limpo.

**Duas autocorreções dele, ambas substantivas.**

**(1) A que salva a medição.** A primeira versão do `probe.js` criava junction com
`fs.symlinkSync(..., 'junction')` — primitiva nativa do libuv — enquanto Go e Python usam
`cmd /c mklink /J`. As duas produzem o mesmo *reparse tag*, mas conteúdo diferente no
`REPARSE_DATA_BUFFER` (`SubstituteName`/`PrintName`), que é **exatamente** o que `readlink()` lê.
Teríamos confundido *"o `lstat` do Node diverge"* com *"o objeto medido é outro"* — envenenando a
tabela comparativa que é a razão de existir deste ML. Verifiquei: os três braços agora chamam
`mklink /J`. Uma medição comparativa só vale se o **objeto** for o mesmo nos três; a variável sob
teste tem de ser o runtime, não a fixture.

**(2) A que corrige um erro meu.** Ele reproduziu a falha da pergunta 7 localmente e reportou que
**meu diagnóstico estava errado**. Confirmei com `pwsh` antes de aceitar, porque a mensagem do git
era compatível com a minha hipótese:

```
modo expressão   $arr = 120000,$blob,"mylink"   →  Object[], 3 elementos    ← vírgula constrói array
modo argumento   & exe 120000,$blob,mylink      →  1 arg, "$blob" LITERAL   ← nem array, nem interpolação
forma citada     & exe "120000,$blob,mylink"    →  1 arg, interpolado       ← o remédio
```

Eu apliquei semântica de **modo expressão** ao **modo argumento**. E o remédio **muda com o
diagnóstico**: não é "juntar os argumentos", é **citar a string** — quem lesse a versão antiga
poderia escapar vírgulas e seguir sem interpolação. REQ e roadmap corrigidos; a versão errada ficou
registrada como errada.

**AC2 satisfeita pelo mecanismo certo:** `git ls-files --stage mylink` é lido logo após o
`update-index` e **antes** do checkout, com o valor cru impresso ao lado do esperado — sem veredito.
Prova que o argumento chegou íntegro, não só que o comando devolveu `exit 0`.

**Também:** `Path(junction).rmdir()` em vez de `os.rmdir()` — a primitiva **exata** de
`manager.py:589`, que é o alvo da medição. Medir a primitiva vizinha teria respondido a pergunta
errada.

**O que ele explicitamente NÃO verificou, e está certo em não ter:** a execução real. Junction não
existe fora do Windows e `workflow_dispatch` só dispara da branch default. Ele nomeou a lacuna em
vez de simular — que era o risco que eu tinha sinalizado no handoff.

## Wave 2 — Governança (ML único)
> Dependências: Wave 1 completa.

### ML-2A — Nota de correção na REQ-2026-08-29 e nota de vault
**Status:** ✅ Concluído
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
- [x] AC7 e AC8 da REQ
- [x] A nota separa as três classes de guarda — a versão ampla demais ("todas furadas") é errada
- [x] Nota linkada em `vault/notes/index.md`

**Gates da wave:**
```bash
grep -q "junction" docs/req/REQ-2026-08-29-namespace-de-agente-nao-declarado-em-agents-fica-invisivel-e-o-validate-reporta-limpo-sem-olhar.md
grep -q "junction" vault/notes/index.md
```

#### Resultado do ML-2A (arquiteto, 2026-08-31)

**Nota de correção anexada à `REQ-2026-08-29`** (que segue `Done`, não reaberta, AC12 não reescrito).
O ponto registrado: o AC12 **não está errado sobre o que mediu — está incompleto sobre onde vale**.
*"Verificável nos 3 CLIs"* foi lido como *"verificado em toda plataforma"*, e nenhum dos três havia
sido exercitado em Windows quando aquela REQ fechou. O instrumento que torna Windows mensurável só
passou a existir depois, no #221. A mesma frase aparece em outras REQs desta família — por isso a
nota fica no artefato, não só no vault.

**Duas notas de vault**, ambas linkadas no índice:

1. `lstat-nao-ve-junction-e-guarda-de-folha-nao-olha-ancestral-2026-08-31.md` — a medição crua, a
   inversão de privilégio (`mklink /J` não exige, `os.Symlink` exige), as **três classes de guarda**
   com a tabela mostrando que o freio existe só no Go, e a Classe 3 que **não tem nada de Windows**.
   Registra também por que os gates de paridade são cegos aqui: paridade mede se as implementações
   concordam, não se o contrato está correto.
2. `powershell-modo-argumento-nao-interpola-nem-divide-2026-08-31.md` — **não estava previsto no
   ML**, e entra porque passa o critério dos dez minutos com folga. O mecanismo real (modo argumento
   não interpola *e* não divide) muda o remédio: citar, não escapar. Quem aplicasse a correção pela
   minha leitura errada escaparia vírgulas e seguiria sem interpolação, falhando por outra mensagem
   ou em silêncio.

A lição que atravessa as duas e que eu quero encontrar de novo daqui a três meses: **uma mensagem de
erro compatível com a sua hipótese não é evidência a favor dela.** `expects <mode>,<sha1>,<path>`
casava com as duas explicações; só o teste que as separa discrimina.

## Verificação diferida para pós-merge — NÃO é critério de aceite de nenhum ML

`workflow_dispatch` só é acionável a partir da branch default, então **a sonda estendida não pode
ser executada nesta branch**. A AC9 da REQ é estruturalmente inverificável antes do merge.

| Ação | Gatilho | Dono | O que fecha |
|---|---|---|---|
| Disparar `windows-probe.yml` | merge deste PR em `main` | arquiteto | AC9 — produz a tabela `runtime × alvo` e decide se a correção é só do Go ou divergência dos 3 |

## Barreira final

Revisão `hefesto-tf` e `hades-tf`, auditoria do arquiteto, `barrier --wave 2`. **A medição só existe
depois do merge** — este PR entrega o instrumento, não o número.
