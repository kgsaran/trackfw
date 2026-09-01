---
status: wip
date: 2026-09-01
req: "docs/req/REQ-2026-09-01-mesmo-gate-de-wave-da-vereditos-diferentes-conforme-o-cli-que-executa-o-barrier.md"
squad: "hades-tf, apolo-tf, artemis-tf"
---

# Roadmap: Os 3 CLIs executam gate de wave com `sh -c`

> Created: 2026-09-01 | Status: wip

## Context

REQ: `docs/req/REQ-2026-09-01-mesmo-gate-de-wave-da-vereditos-diferentes-conforme-o-cli-que-executa-o-barrier.md`
ADR: `docs/adr/ADR-2026-09-01-gate-de-wave-e-contrato-portavel-em-shell-posix-nao-script-do-sistema-operacional.md`

**Item 7 do issue #216** — o mais grave da lista, e o único que quebra a correção da **própria
ferramenta de governança**: `trackfw barrier` pode aprovar uma wave para quem usa um CLI e reprová-la
para quem usa outro, no mesmo repositório e no mesmo commit.

Medição que decidiu o ADR: **83 comandos** de gate em todos os roadmaps, e **nenhum idioma existe no
`cmd.exe`** (35 `grep`/`sed`/`awk`, 14 `test`/`[`, 8 negações com `!`, 3 `&&`/`||`, 3 `$( )`).
No Windows, Node e Python **não avaliam diferente — falham em avaliar**.

## Acceptance Criteria

- [ ] Os 3 CLIs executam gate com `sh -c`
- [ ] Mesmo gate, mesmo veredito nos 3 — **e o controle**: gate que deve reprovar continua reprovando
- [ ] `sh` ausente falha nomeando o remédio, com mensagem byte-idêntica nos 3
- [ ] **"Não pôde ser avaliado" é distinto de "reprovou"**
- [ ] Gate contra regressão para `shell: true`/`shell=True`
- [ ] Item 7 sai de `REPRODUCED` (camada 2 de 4 → 3), com a transição explicada
- [ ] `make quality` e **CI** verdes

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Modelo de ameaça da troca de shell
> Dependências: nenhuma. Bloqueia a implementação.

### ML-0A — Superfície de execução e a semântica de "não pôde medir"
**Status:** ✅ Concluído
**Agente:** `hades-tf`
**Files affected:** nenhum (documento em `docs/seguranca/`)
**Por que esta Wave 0 importa mais que a média:** estamos mudando **como conteúdo de artefato
versionado vira processo**. O gate já executava comando arbitrário — não é superfície nova — mas o
**shell muda**, e com ele o parsing, o quoting e o tratamento de erro.
**Actions:**
1. **A troca amplia superfície?** `sh -c` versus shell do SO: um gate malicioso num roadmap de PR de
   terceiro ganha capacidade nova? (Lembrar: o `barrier` já executa esses comandos hoje no Go.)
   Considere o `roadmapTrustForGates`, que **já tem REQ aberta por fail-open**.
2. 🔴 **A semântica de "não pôde ser avaliado".** A AC4 exige distinguir isso de "reprovou". **Julgue
   qual é o lado seguro** e por quê: um `sh` ausente que resulte em *"wave não passou"* é falso
   negativo que bloqueia trabalho legítimo; que resulte em *"passou"* é falso positivo que libera
   trabalho não verificado. **Nenhum dos dois é obviamente certo — quero o argumento.**
3. **Falsificação nas duas direções**, com atenção ao simétrico: um `barrier` que passe a recusar
   ambiente legítimo (contêiner mínimo sem `sh`? CI de terceiro?) troca um defeito por outro.
4. **Enumeração:** só os dois pontos (`barrier.js:561`, `barrier.py:582`)? Varra por outros lugares
   nos 3 CLIs onde conteúdo de artefato vira processo — `shell: true`, `shell=True`, `exec`, `spawn`,
   `system`. **Assuma que minha lista de dois está incompleta**; nesta sessão isso se confirmou
   repetidamente.
5. **Residual declarado.**
**Critérios de aceite:**
- [x] Veredito sobre ampliação de superfície, com vetor concreto se houver
- [x] Argumento explícito sobre o lado seguro de "não pôde medir"
- [x] Enumeração de pontos onde artefato vira processo, nos 3 CLIs
- [x] Nenhuma linha de implementação escrita
- [x] Parecer em `docs/seguranca/2026-09-01-modelo-de-ameaca-do-shell-de-gate.md`

**Gates da wave:**
```bash
test -f docs/seguranca/2026-09-01-modelo-de-ameaca-do-shell-de-gate.md
! grep -qi "placeholder" docs/seguranca/2026-09-01-modelo-de-ameaca-do-shell-de-gate.md
grep -q "Residual" docs/seguranca/2026-09-01-modelo-de-ameaca-do-shell-de-gate.md
```

#### Resultado do ML-0A (hades-tf, 2026-09-01) — auditado pelo arquiteto

### 1. Ele corrigiu a própria primeira conclusão, e o achado inverte minha premissa

A primeira passagem concluiu *"no-op em POSIX"* — por **leitura de código**. Uma PoC com `sh` falso
no `$PATH` provou o contrário:

| forma | resolve `sh` como |
|---|---|
| `shell: true` / `shell=True` (hoje) | **preso a `/bin/sh`** |
| `spawnSync('sh', [...])` / `subprocess.run(["sh", ...])` (Wave 1) | **via `$PATH`**, como o Go sempre fez |

**A correção amplia superfície — pequena, mas real.** Node e Python saem de interpretador fixo para
interpretador resolvido por `$PATH`. É **necessário** para Windows, onde o `sh.exe` do Git Bash nunca
está em `/bin/sh`; mas quem controla o `$PATH` do processo passa a controlar o interpretador do gate
nos três, e não só no Go.

Eu teria assumido no-op. **Ele mediu.**

E foi honesto sobre o limite: no Windows a premissa do ADR é **inferida, não medida** — não há máquina
disponível. Nomeou como inferência em vez de afirmar como fato.

### 2. A resposta sobre "não pôde medir" veio com precedente interno

Eu pedi o argumento, não a conclusão, e sugeri considerar um terceiro estado. A resposta é melhor que
a sugestão: **o projeto já resolveu este exato problema.**

O `roadmapTrustForGates` tem um terceiro status — **`not_evaluated`**, distinto de `passed` e
`blocked` — nos três CLIs (`barrier.go:872`, `barrier.js:592`, `barrier.py:688`/`:747`). Ele
**reprova a wave** e **nomeia o remédio**.

**Fail-closed, reusando padrão existente**, em vez de inventar código de saída. E ele mediu algo que
teria quebrado a implementação: `sh -c 'nosuchtool'` devolve **exit 127** — *ferramenta interna
ausente*, **não** *`sh` ausente*. A AC4 **não pode** se apoiar em 127; o sinal de `sh` ausente é
falha no nível do spawn.

### 3. A enumeração confirmou o escopo do ADR — e achou uma vulnerabilidade fora dele

Só os três pontos do `barrier` levam conteúdo de **artefato versionado** para um shell. O ADR está
correto.

**Mas apareceu outra coisa**, porque pedi *todo* ponto onde conteúdo vira processo:

```
--host  'x" ; id > /tmp/INJETADO ; echo "'
   ↓
exec:   open "http://x" ; id > /tmp/INJETADO ; echo ":4080"
```

`serve.js:46-56` devolve `http://${host}:${port}` **sem sanitização** para qualquer string que não
seja `localhost` nem IP, e o resultado é interpolado numa string de shell passada a `exec()`.
**Reproduzido pelo arquiteto.** `serve.py:196` tem a variante Windows (`shell=True`); os ramos Darwin
e Linux do Python usam argv e **estão corretos** — precedente interno para a correção.

**REQ própria aberta.** Fora do escopo desta; não entra na Wave 1.

### Residual declarado

Composição com a REQ aberta de fail-open do `roadmapTrustForGates`; o vetor de adulteração de
`$PATH` de §1a; e a recomendação de que AC2/AC3/AC7 sejam verificadas no job `windows-full-suites`,
já que runner POSIX **não consegue falsificar** um defeito específico de Windows.

## Wave 1 — A correção (ML único)
> Dependências: Wave 0. ML único: os dois arquivos são pequenos e a semântica de erro tem de ser
> idêntica entre eles — separar convidaria à divergência que a REQ existe para fechar.

### ML-1A — `sh -c` nos dois CLIs, com `not_evaluated` para `sh` ausente
**Status:** ⬜ Pendente
**Agente:** `apolo-tf`
**Files affected:** `npm/src/commands/barrier.js`, `pypi/trackfw/commands/barrier.py`
**Actions:**
1. Trocar `spawnSync(cmd, { shell: true })` e `subprocess.run(cmd, shell=True)` por invocação
   explícita de `sh -c`, resolvida por `$PATH` — como o Go já faz.
2. 🔴 **`sh` ausente vira `not_evaluated`, não falha de gate.** **Reuse o padrão que já existe** no
   `roadmapTrustForGates` (`barrier.go:872`, `barrier.js:592`, `barrier.py:688`/`:747`): terceiro
   status, distinto de `passed`/`blocked`, que **reprova a wave** e **nomeia o remédio**. Não invente
   código de saída novo.
3. 🔴 **Não use `exit 127` como sinal de `sh` ausente.** Medido no ML-0A: `sh -c 'nosuchtool'`
   devolve 127 — *ferramenta interna ausente*, não *`sh` ausente*. O sinal correto é falha no nível
   do **spawn**.
**Critérios de aceite:**
- [ ] Gate com idioma POSIX (`! grep -q`, `test -f`, `$( )`) dá **o mesmo veredito nos 3 CLIs**
- [ ] 🔴 **Controle:** gate que **deve reprovar** continua reprovando nos três — uniformizar para
      "passa" seria pior que o defeito
- [ ] `sh` ausente → `not_evaluated`, mensagem **byte-idêntica** nos 3, nomeando o remédio
- [ ] `make quality` verde

## Wave 2 — Gate e contrato
> Dependências: Wave 1. 🔴 Nasce ligado, com guarda de vacuidade ancorada no mesmo cwd, `python3`
> nunca `python`. E **prefira `assert_count` a `assert_has`** onde a assinatura puder repetir — o
> gate precisa reprovar se **um** dos dois CLIs regredir, não só se ambos.

## Verificação que só o CI fecha

Item 7 saindo de `REPRODUCED`: camada 2 de **4 para 3**. O check compara o veredito do mesmo gate nos
3 runtimes — **comportamento de produto**, então deve genuinamente virar. Verificado o que ele mede
**antes** de fixar o número.

## Barreira final

`hefesto-tf` e `hades-tf`, auditoria do arquiteto, `barrier`. **CI verde**, não só verde local.
