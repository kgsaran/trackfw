---
status: wip
date: 2026-08-31
req: "docs/req/REQ-2026-08-31-guarda-de-folha-faz-lstat-so-no-ultimo-componente-e-nunca-inspeciona-ancestral-escrita-fora-do-projeto-em-todo-so-e-todo-runtime.md"
squad: "hades-tf, apolo-tf, artemis-tf"
---

# Roadmap: Guarda de folha resolve o caminho e afirma contenção antes de escrever

> Created: 2026-08-31 | Status: wip

## Context

REQ: `docs/req/REQ-2026-08-31-guarda-de-folha-faz-lstat-so-no-ultimo-componente-e-nunca-inspeciona-ancestral-escrita-fora-do-projeto-em-todo-so-e-todo-runtime.md`

`Lstat` só deixa de seguir o **último** componente do caminho — ancestrais são sempre seguidos. As
guardas de link do projeto checam só a folha, então um symlink num diretório ancestral redireciona a
escrita para fora do projeto sem que nada olhe. **Todo SO, todo runtime, sem mitigação acidental.**

Descoberto a reboque da investigação de junction, mas independente dela: não precisa de junction, de
`ModeIrregular` nem de privilégio — **um symlink de diretório comum basta**.

**Reproduzível localmente em macOS/Linux**, sem runner Windows e sem a sonda. É o que separa esta REQ
da irmã.

## Acceptance Criteria

- [x] Enumeração real, pelos primitivos de **escrita**, não por `ModeSymlink` — feita no ML-1A.
      🔴 **O número "228 candidatos brutos" desta linha está morto** e foi removido do AC: é de
      2026-08-31, cobria **três runtimes** (Go + `npm/src` + `pypi/trackfw`) e uma lista de
      primitivos diferente (`writeFileSync`, `write_text`, `open(...,'w')`). Node e Python deixaram
      de existir no commit `2eae0a44` (v8). O número vigente é o que o **gate do ML-2A** produzir
      na primeira execução — medido pela regra, não por `grep` avulso.
- [x] Escrita através de ancestral symlink recusada, forma resolver-e-afirmar-contenção (Wave 1)
- [x] Falsificação nas duas direções, **incluindo o controle** de que operação legítima segue
      funcionando — **satisfeito na Wave 2**. Forma executável e permanente:
      `scripts/check-write-containment.sh` com 3 braços de falsificação (`unguarded-write`,
      `marker-accepted`, `vacuous-scan`), rótulos **literais** colhidos pela guarda de conjunto.
      Braços (a) e (b) também provados por **execução real** pelo arquiteto — ver ML-2B.
- [x] Recusa audível em stderr (verificado por execução real no ML-1C)
- [N/A] ~~Paridade exata nos 3 CLIs — aqui a paridade vale normalmente~~ — 🔴 **inaplicável pós-v8.**
      Este AC é de 2026-08-31. A partir da v8.0.0 existe **uma** implementação, em Go, entregue por
      três canais; não há mais dois artefatos para manter em paridade. Deixar este AC aberto tornaria
      a REQ permanentemente infechável; marcá-lo ✅ seria falso. Fica **N/A com a razão inline**.
- [x] `make quality` e **CI** verdes — **Wave 1: SIM, medido.** PR **#397** (draft) aberto em
      2026-09-21: **21 checks verdes**, incluindo `windows-full-suites` (4m23s),
      `windows-symlink-unprivileged`, `windows-integrations-resolve`, `windows-gates-cp1252`,
      `windows-defect-reproduction`, os 4 `parity-falsify-shard` e o agregador `parity`.
      **Repetido após a Wave 2 e verde de novo: 21 checks, 0 falhas**, com os gates novos
      (`check-write-containment` + os 3 braços de falsificação) já dentro da suite.
      **Barreira local final:** RC=0, 792 `^OK `, 0 `: FALHA`, 1664 linhas, guarda de conjunto OK.

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat Model
> Dependências: nenhuma. Bloqueia toda a implementação.

### ML-0A — Enumeração real e modelo de ameaça
**Status:** ✅ Concluído
**Agente:** `hades-tf`
**Files affected:** nenhum (documento em `docs/seguranca/`)
**Actions:**
1. **Completude da enumeração — o entregável principal deste ML.** A lista de 3 guardas
   (`update.go:1869`, `:1894`, `discover.go:268`) veio de `grep` por `ModeSymlink` e é
   **conhecidamente incompleta**: esse grep é cego para todo ponto que escreve **sem checar link
   nenhum**, que é a população em risco. Varra os **primitivos de escrita** (`os.WriteFile`,
   `os.Create`, `writeFileSync`, `write_text`, `open(...,'w')`) e determine quais escrevem em
   caminho derivado de `root`. Mostre a busca. Na Wave 0 anterior você achou `copyPath`/`_copy_path`
   e o código morto `writeCIWorkflowForce` fora da minha lista — assuma que esta também está.
2. **Modelo de ameaça** — quem esvazia esta Wave 0 sem quebrar regra escrita? Quem planta o
   ancestral, com que capacidade, e o que ganha?
3. **Falsificação nas duas direções** — para cada superfície: o que quebra quando regride, e o que
   quebra quando regride ao contrário (a guarda super-disparar e recusar operação legítima é o
   risco simétrico, e é o que transforma correção em quebra).
4. **Residual declarado.**
**Critérios de aceite:**
- [x] As quatro seções com evidência, não asserção de uma linha
- [x] A enumeração distingue "escreve sob `root` controlável" de "escreve em caminho fixo"
- [x] Nenhuma linha de implementação escrita
- [x] Parecer em `docs/seguranca/2026-08-31-modelo-de-ameaca-da-guarda-de-ancestral.md`

**Gates da wave:**
```bash
test -f docs/seguranca/2026-08-31-modelo-de-ameaca-da-guarda-de-ancestral.md
! grep -qi "placeholder" docs/seguranca/2026-08-31-modelo-de-ameaca-da-guarda-de-ancestral.md
grep -q "Residual" docs/seguranca/2026-08-31-modelo-de-ameaca-da-guarda-de-ancestral.md
```

#### Resultado do ML-0A (hades-tf, 2026-08-31) — auditado pelo arquiteto

**A enumeração estava subcontada em uma ordem de grandeza — e o defeito foi EXPLORADO ao vivo,
não inferido.**

**Duas PoCs com o binário Go real, fora do repositório:**

| comando | resultado |
|---|---|
| `trackfw req new`, com `docs/req` → symlink para fora | escreveu a REQ **inteiramente fora da árvore**, `exit 0`, **sem aviso** |
| `trackfw update harness --targets claude-skill --install-missing`, com `$HOME/.claude` → symlink | escreveu `SKILL.md` **fora do `$HOME`**, `updated=1 failed=0`, **sem aviso** |

Isto deixa de ser risco teórico. E o segundo caso é pior que o primeiro: escopo **global**, no `$HOME`
do usuário.

**A população real:** além das ~10 guardas de folha que eu conhecia (classe *"checa a folha, ignora o
ancestral"*), existem **187 pontos que não checam link algum** — nem folha, nem ancestral —
distribuídos em quatro famílias, cada uma com o grep que a sustenta:

| família | sites |
|---|---|
| `trackfw update harness` (escopo **global**: `$HOME/.claude`, `.codex`, `.gemini`, `.cursor`, `.copilot`, `.kiro`) | 69 |
| geradores de artefato (`req new`, `roadmap new`, `adr new`, `note new`) | 34 |
| geradores de hook/script (credential-guard, git-branch-guard, husky, lefthook, `init`) | 75 |
| diversos (sync, metrics, configure, quarantine) | 9 |

O caminho de instalação de catálogo (`manager.*`) é **classe (c)** — genuinamente caminha todos os
ancestrais via `rejectSymlinks`, com call sites verificados nos 3 CLIs. Ou seja: **o projeto já sabe
fazer certo num lugar** e não replicou nos outros.

### A correção mais valiosa: a forma está certa, a grafia estava errada

Ele confirmou *resolver-e-afirmar-contenção* como forma, e **derrubou a grafia** em três pontos que
eu não teria pego:

1. **`path.resolve()` do Node é puramente léxico — nunca segue symlink.** Minha REQ lista
   `filepath.EvalSymlinks` / `fs.realpathSync` / `Path.resolve()`, mapeados a Go/Node/Python nessa
   ordem. Se a Wave 1 ler isso de forma solta e usar `path.resolve()` no Node, **a guarda vira no-op
   que passa em todo teste escrito contra o comportamento do Go**. É a armadilha perfeita: verde,
   paritário e inútil. O primitivo correto no Node é `fs.realpathSync`.
2. **`EvalSymlinks`/`realpathSync` falham com folha inexistente** — e o caso dominante aqui é
   *criar* arquivo novo. Tem de resolver o **diretório pai** e concatenar a folha, nunca resolver o
   destino completo. O `Path.resolve(strict=False)` do Python tolera nativamente — o que confirma
   que nomear três primitivas diferentes estava certo, e uma fórmula única estaria errada.
3. **Comparar destino resolvido contra `root` NÃO resolvido gera falso positivo** — medido com
   `/tmp` → `/private/tmp` no macOS, um symlink de ancestral real e nada malicioso. **Os dois lados
   precisam ser resolvidos.**

### A quebra de comportamento que precisa de decisão do KG

Respondendo ao meu próprio exemplo: `.github` apontando para diretório compartilhado **dentro** do
mesmo `root` continua funcionando. Mas a forma real mais comum — um diretório de templates
compartilhado **fora** da árvore de cada projeto — é **indistinguível do ataque e será recusada**.

Isto é quebra declarada, não efeito colateral escondido. A Wave 1 precisa escolher entre **recusa
audível nomeando o caminho resolvido** (a AC4 já exige) e um **opt-out explícito** (escopo novo).
**Decisão do KG, registrada antes da Wave 1.**

### Residual declarado

TOCTOU entre resolver e escrever **não** é eliminado (aceito — mesma janela que a REQ já aceitou ao
rejeitar `Lstat` por componente); o `root` em si nunca é `Lstat`ado no padrão atual; delta de
contagem em Go (80 medidos vs 85 na REQ, não reconciliado); enumeração por família, não site a site
nos 228 brutos; PoCs só em macOS.

**Nota de vault:** `resolve-symlinks-primitivas-divergem-nos-3-runtimes-folha-inexistente-2026-08-31.md`.

**Resíduo de PoC removido pelo arquiteto:** a execução deixou `.agents/` na raiz do repositório
(não versionado, criado pela PoC do `update harness`). Apagado, não commitado.

## Wave 1 — A correção
> Dependências: Wave 0 completa e auditada (gate verde, verificado em 2026-09-18).
> 🔴 **A enumeração da Wave 0 é de 2026-08-31, PRÉ-v8.** Os 187 sítios incluem Node e Python, que
> deixaram de existir no commit `2eae0a44`. Medição minha de 2026-09-18 no Go: **102 ocorrências de
> `os.WriteFile`/`os.Create`/`os.Rename` em 19 arquivos** de `internal/`. Revalidar é a primeira
> ação do ML-1A — **não** reusar o número antigo.

### ML-1A — extrair a contenção para ponto único e revalidar a enumeração pós-v8
**Status:** ✅ Concluído (auditado por Zeus em 2026-09-18: extração sem mudança de fixture, `pathguard` sem ciclo, barreira 630 gates / 0 FAIL)
**Files affected:** cria/estende um pacote folha (avaliar `internal/pathanchor`, que já existe),
`internal/integrations/manager.go` (passa a consumir o extraído), testes correspondentes
**Actions:**
1. 🔴 **Revalidar a enumeração no Go**, com o método que o AC1 exige: varrer **primitivos de escrita**
   (`os.WriteFile`, `os.Create`, `os.OpenFile`, `os.Rename`, `os.MkdirAll`), **não** `ModeSymlink` —
   o `grep` por `ModeSymlink` é cego justamente para quem não checa nada, que é a população em risco.
   Classificar cada sítio em: **(a)** escreve sob `root` controlável pelo usuário · **(b)** escreve em
   caminho fixo · **(c)** já protegido. Reportar a tabela.
2. **Extrair** `rejectSymlinks` e `beneath` de `internal/integrations/manager.go:755-777` para o pacote
   folha. 🔴 **Extração, não reescrita** — a semântica é a que já está em produção na classe (c). O
   `manager.go` passa a consumir o extraído; o comportamento dele **não pode mudar**.
3. Garantir que o pacote folha **não importe** `internal/commands` nem `internal/validator` (ciclo).
**Acceptance criteria:**
- [x] Tabela de enumeração revalidada, com o comando que a produziu e a classificação (a)/(b)/(c)
- [x] 🔴 **Não-regressão do `manager`:** os testes de `internal/integrations` continuam verdes **sem
      alteração de fixture**; se algum precisar mudar, **pare e relate** — seria mudança de semântica
- [x] Pacote folha sem importar `commands`/`validator`
- [x] `go build ./...` RC=0 · `go test ./...` RC=0
- [x] Uma frase por teste novo (AC11)

### ML-1B — aplicar a contenção na família de **escopo global** (a mais grave)
**Status:** ✅ Concluído **na aplicação** (PoCs verificadas por mim), com corretivo no ML-1B-bis — a barreira reprova 2 sítios de teste do ML-1A
**Files affected:** `internal/generators/update.go` (53 sítios), `internal/generators/agentfiles.go` (21),
`internal/identity/identity.go` (3, cópia não-guarded de `atomicWrite`), `internal/thirdparty/quarantine.go`
(3, idem), e os testes correspondentes
**Por que esta família primeiro:** é a única cujo dano **sai do projeto e atinge o ambiente do
usuário**. A PoC da Wave 0 escreveu `SKILL.md` **fora do `$HOME`** com `updated=1 failed=0`, sem aviso.
**Actions:**
1. Aplicar `pathguard.RejectSymlinks(root, destino)` antes de cada escrita, com `root` **absoluto**.
2. **Convergir as duas cópias de `atomicWrite`** (`identity.go:88`, `quarantine.go:141`) para o
   `pathguard` — os comentários delas já declaram a duplicação.
3. Recusa **audível** (decisão 3 da ADR): stderr nomeando caminho e motivo. Silêncio aqui vira
   *"o update não atualizou meu arquivo e não disse nada"*.
**Acceptance criteria:**
- [x] 🔴 **Braço (a):** com ancestral symlink, a escrita é recusada e **nada** é criado fora
- [x] 🔴 **Braço (b) — inegociável:** `update harness` legítimo, sem link algum, **continua
      funcionando** em todos os alvos. Sem este braço, trocamos um buraco por uma quebra
- [x] A PoC da Wave 0 (`SKILL.md` fora do `$HOME`) passa a **falhar**
- [x] `go build ./...` RC=0 · `go test ./...` RC=0 · `make quality` **630 gates / 0 FAIL**
- [x] Uma frase por teste novo (AC11)

### ML-1B-bis — corretivo: os testes do `pathguard` não usam guarda de capacidade
**Status:** ✅ Concluído (auditado por Zeus: gate OK com **143** arquivos, barreira **630 gates / 0 FAIL**)
**Files affected:** `internal/pathguard/pathguard_test.go` — **só este**
**Contexto:** o ML-1B está **aprovado na aplicação** — reproduzi os dois braços por execução real:
`(a)` `$HOME/.claude` como symlink → `failed=1`, **nada criado fora**; `(b)` `$HOME` limpo →
`updated=35 failed=0`, e idempotente na segunda execução (`skipped=35`). `atomicWrite` convergiu
para **1** ocorrência.

🔴 **A barreira reprova**, e o defeito vem do **ML-1A**:
```
check-symlink-privilege-guard: FALHA — sitios sem guarda de capacidade:
  internal/pathguard/pathguard_test.go:70
  internal/pathguard/pathguard_test.go:90
```
O gate existe para impedir *"a décima-primeira instância da issue #315"*: teste que cria symlink deve
usar `symlinkOrSkip`, para distinguir **"sem privilégio" (skip)** de **"falhou por outro motivo" (fail)**.

🔴 **Por que a barreira do ML-1A não pegou — e a falha é do meu processo.** O gate enumera por
**`git ls-files`** (`check-symlink-privilege-guard.sh:125-127`). Quando rodei a barreira do ML-1A,
`internal/pathguard/` estava **untracked** (`?? internal/pathguard/`), então o gate varreu **142**
arquivos e **não viu** o pacote novo. Depois do commit ele passou a ver **143** e reprovou.
**Rodar a barreira antes de commitar é vácuo para arquivo novo** em qualquer gate baseado em
`git ls-files`. Correção de processo: `git add -N` (ou `git add`) **antes** da barreira, sempre que o
ML criar arquivo.
**Actions:**
1. Usar o helper existente — há dois modelos no repo: `internal/discover/symlink_helper_test.go:30` e
   `internal/validator/symlink_helper_test.go:32`. 🔴 **Reuse, não reescreva.**
2. Aplicar nos dois sítios (linhas ~70 e ~90).
**Acceptance criteria:**
- [x] `bash scripts/check-symlink-privilege-guard.sh` → OK, com a contagem de arquivos varridos (não vácuo)
- [x] `go test ./internal/pathguard/` RC=0
- [x] 🔴 `make quality` **630 gates / 0 FAIL**, medido com `/usr/bin/grep -c "^OK "` **e** com
      `/usr/bin/grep -ciE "^FAIL|FALHA"` → **0**. Ver nota de instrumento abaixo.
- [x] Uma frase por teste alterado (AC11)

### ML-1C — geradores de artefato e de hook/script (inclui o **AC9**)
**Status:** ✅ Concluído (auditado por Zeus: PoCs 1 e 2 fechadas por execução real, recusa audível, braço (b) íntegro, barreira 630/0)
**Files affected:** `internal/discover/discover.go` (13), `internal/generators/scaffold.go` (33),
`internal/generators/req.go` (7), `internal/generators/roadmap.go` (7 — **AC9**),
`internal/generators/adr.go` (4), `internal/generators/note.go` (4), e os testes correspondentes
**Por que esta família agora:** contém as **duas PoCs que eu reproduzi pessoalmente** contra o binário
da `main` — `discover --init` gravando 6 arquivos fora da árvore, e `roadmap move` seguindo symlink de
folha para reescrever arquivo externo (**AC9**, absorvido da REQ irmã).
**Actions:**
1. Aplicar `pathguard.RejectSymlinks(root, destino)` / `pathguard.GuardedWrite` antes de cada escrita,
   com `root` **absoluto**.
2. **AC9:** `MoveRoadmap` recusa quando origem **ou** destino escapa da árvore. 🔴 Atenção ao
   `os.Rename` — não é só escrita: a **origem** também pode ser symlink.
3. Recusa audível nomeando caminho e motivo (decisão 3 da ADR).
**Acceptance criteria:**
- [x] 🔴 **PoC 1 fecha:** `.github` e `scripts` como symlink → `discover --init` **recusa**, e **nada**
      é criado fora. Demonstre antes/depois.
- [x] 🔴 **PoC 2 (AC9) fecha:** `ln -s /fora/vitima.md docs/roadmaps/backlog/ROADMAP-isca.md &&
      roadmap move ROADMAP-isca wip` → **recusa**, e a vítima **não é alterada**.
- [x] 🔴 **Braço (b), inegociável:** `discover --init`, `req new`, `roadmap new`, `adr new`,
      `note new` e `roadmap move` **legítimos continuam funcionando**. Execução real, não só unitário.
- [x] `make quality` **630 gates / 0 FAIL** — métrica de falha: `/usr/bin/grep -cE ": FALHA"` → **0**
- [x] Uma frase por teste novo (AC11)

### ML-1D — diversos e wrappers (fecha a Wave 1)
**Status:** ✅ Concluído (auditado por Zeus em 2026-09-21: aplicação aprovada em 2026-09-20; corretivo **ML-1D-bis** fechado — gate 150 arquivos/OK dentro da barreira, 630 `^OK ` / 0 `: FALHA`). **Fecha a Wave 1 — verde local; CI ainda não exercido.**
`configure.go`, `java.go`, `config_agents_register.go`, `metrics.go`, `sync.go`, `validator.go`
(**caminho relativo puro** — exigem resolver `root` absoluto antes, precondição do ML-1A), e os
**wrappers** `manifest.go:81` / `render.go:722`, que chamam `atomicWrite` sem contenção e **não
aparecem** no grep de primitivos.

**Acceptance criteria:** *(bloco acrescentado por mim em 2026-09-21 — o barrier acusava
`ML-1D: no acceptance block`. Os itens abaixo são os que eu **de fato** verifiquei na auditoria de
2026-09-20/21, não uma racionalização retroativa.)*
- [x] 9 sítios cobertos — 7 de caminho relativo puro + os 2 wrappers `atomicWrite`
- [x] Guard **antes** da escrita em cada sítio, com recusa audível em stderr
- [x] `go build ./...` RC=0 · `go test ./...` RC=0
- [x] Braço (b) conferido por mim por execução real: `init`, `adr new`, `req new`, `baseline`
- [x] `check-symlink-privilege-guard` OK com **150** arquivos (não 143 — a contagem menor é vácuo),
      após o corretivo do **ML-1D-bis**
- [x] `make quality` sozinho: 630 `^OK ` / 0 `: FALHA`

#### 🔴 Enumeração revalidada pelo ML-1A, auditada por mim

**160 linhas de chamada em 19 arquivos** — `(a)` em risco **156 linhas / 18 arquivos**, `(b)` fora de
risco 1, `(c)` já protegido 3. A comparação com os **187** da Wave 0 mede a **remoção de Node e
Python pela v8**, não redução de risco no Go. Os maiores: `generators/update.go` **53** (escopo
**global**: `$HOME/.trackfw`, `.claude`, `.codex`, `.gemini`, `.cursor`, `.copilot`, `.kiro`),
`generators/scaffold.go` **33**, `generators/agentfiles.go` **21**, `discover/discover.go` **13**.

#### 🔴 Dois achados do ML-1A que mudam o ML-1B

**1. Três cópias de `atomicWrite`, só uma protegida.** Confirmado por mim:
`integrations/manager.go:764` (guarded), `identity/identity.go:88` (**não**), `thirdparty/quarantine.go:141`
(**não**). Os comentários das duas cópias **declaram** a duplicação — *"Mirrors the pattern used by
internal/integrations/manager.go"* e *"replicated here rather than imported"*. Mesma causa, mesmo
roadmap: as três convergem para o `pathguard`.

**2. 🔴 A enumeração por primitivos é necessária mas INSUFICIENTE.** `manifest.go:81` e
`render.go:722` chamam `atomicWrite` com caminho derivado de `root` **sem** passar por
`rejectSymlinks` — e **não aparecem** no grep de `os.WriteFile`/`os.Create`/`os.Rename`, porque
chamam o **wrapper**. Verificado por mim: dos 4 callers de `atomicWrite` em `integrations/`, só os de
`manager.go:308,374` passam por `resolve()→rejectSymlinks()`.
**Consequência para o ML-1B:** enumerar também os **wrappers de escrita** e seus callers, não só os
primitivos. Um sítio protegido por wrapper não-protegido é indistinguível de sítio seguro no grep.

#### Precondição declarada pelo ML-1A

Sítios que escrevem em **caminho relativo puro** — `configure.go:140` (`"trackfw.yaml"`),
`java.go:63` (`"pom.xml"`), `validator.go:54` (`".trackfw-baseline.json"`) — exigem que o caller
**resolva um `root` absoluto antes**; passar caminho relativo a `RejectSymlinks` dá "escapes root"
imediato. **O ML-1B não é inserção mecânica** nesses casos.

Famílias, em ordem de gravidade medida:
As famílias da Wave 0, **a reconfirmar pós-v8** no ML-1A, em ordem de gravidade medida:
1. 🔴 **`update harness`** — escopo **global** (`$HOME/.claude`, `.codex`, `.gemini`, ...). A PoC da
   Wave 0 escreveu `SKILL.md` **fora do `$HOME`** com `updated=1 failed=0`, sem aviso. É o pior caso:
   o dano sai do projeto e atinge o ambiente do usuário.
2. **geradores de artefato** (`req new`, `roadmap new`, `adr new`, `note new`) — a PoC gravou a REQ
   inteiramente fora da árvore, `exit 0`, sem aviso.
3. **geradores de hook/script** (credential-guard, git-branch-guard, husky, lefthook, `init`) — é o
   caminho do `discover --init` que **eu reproduzi** em 2026-09-18: 6 arquivos fora da árvore.
4. **diversos** (`sync`, `metrics`, `configure`, quarantine) — inclui o `roadmap move` do **AC9**,
   absorvido da REQ irmã.
🔴 Cada ML aplica o **braço (b) da ADR** (operação legítima continua funcionando) além do braço (a).

## Wave 2 — Gate falsificável
> Dependências: Wave 1 completa (fechada em 2026-09-21). Detalhada nesta data.
> **Sem paralelismo:** o ML-2B depende do resultado de medição do ML-2A, e ambos tocariam
> `Makefile` e `scripts/check-gates-falsify.sh`. Sequenciais, e a razão fica escrita.

### Por que esta Wave existe

A Wave 1 corrigiu os sítios **conhecidos em 2026-09**. Nada impede que o sítio 157 nasça amanhã sem
contenção — e o custo de descobrir isso é o mesmo que esta REQ acabou de pagar. O entregável da
Wave 2 é o **instrumento que torna a regressão impossível de passar despercebida**.

🔴 **Achado que reorienta o desenho — medido por mim em 2026-09-21:**

```
$ grep -c "symlink-privilege" scripts/check-gates-falsify.sh        -> 0
$ grep -n "symlink-privilege" Makefile .github/workflows/*.yml
Makefile:58:	scripts/check-symlink-privilege-guard.sh                 (invocação NUA)
```

O `check-symlink-privilege-guard.sh` — o gate de que os ACs dos ML-1B-bis e ML-1D-bis dependeram —
**não tem um único caso de falsificação**, e o `--self-test` que ele já traz escrito (3 braços
documentados no cabeçalho) **nunca é invocado**. Outros gates são declarados em **duas** linhas no
Makefile, com o `--self-test` (`Makefile:46, 52, 75, 81`); este, em uma. Nada ficou vermelho porque
`check-orphan-gates.sh` exige apenas que o gate tenha **consumidor**, não que seja falsificável.

Ou seja: a Wave 1 foi auditada com um instrumento cuja capacidade de reprovar nunca foi provada.
E ele **já falhou por vácuo duas vezes nesta campanha** — 143 arquivos em vez de 150, porque os
guard tests nasceram untracked e o gate enumera por `git ls-files`.

**Verificado antes de escrever esta Wave:** rodei `bash scripts/check-symlink-privilege-guard.sh
--self-test` → `self-test: 3/3 braços OK`, RC=0. O código existe e funciona; falta **uma linha** no
Makefile. Por isso essa linha entra aqui, e não numa fila de 11 issues abertas.

### ML-2A — gate de contenção de escrita, nascido falsificável
**Status:** ✅ Concluído (auditado por Zeus em 2026-09-21) · **Papel:** `artemis-tf`

**Files affected:**
- **cria** `scripts/check-write-containment.sh`
- `Makefile` — **3 linhas** no recipe `parity-rest`
- `scripts/check-gates-falsify.sh` — 1 bloco `# Cenário` novo
- `internal/pathguard/pathguard.go` — marcadores de auto-isenção
- sítios de produção que exijam marcador (lista produzida pelo próprio gate, ver Ação 2)

#### Ação 1 — o gate, no molde do `check-raw-read-ban.sh`

🔴 **Copie a forma de `scripts/check-raw-read-ban.sh` (121 linhas). NÃO copie a forma de
`check-symlink-privilege-guard.sh`.** A diferença não é estilística:

| | `check-symlink-privilege-guard.sh` | `check-raw-read-ban.sh` |
|---|---|---|
| como aceita um sítio | **janela de ±5 linhas** — inferência de proximidade | **marcador explícito** `raw-read-allowed: <razão>` na linha ou na imediatamente acima |
| pode reprovar código correto? | 🔴 **sim, e reprovou** | não — o marcador é declaração do autor |

O ML-1D-bis inteiro existiu porque `symlinkOrSkipMetrics` estava **certo** — detectava por condição,
`IsPermission` + errno `1314` — e o gate o reprovou por **geometria**: o nome da guarda estava 13
linhas acima do sítio, fora da janela. Um ML pago para mover um comentário. **Não reproduza isso.**

O gate deve:
1. Varrer `internal/**/*.go` **de produção** (excluir `_test.go`) pelos primitivos
   `os.WriteFile`, `os.Create`, `os.CreateTemp`, `os.OpenFile`, `os.Rename`, `os.MkdirAll`.
   ⚠️ `os.Create\(` **não** casa `os.CreateTemp(` — inclua os dois explicitamente.
2. Aceitar um sítio **apenas** se ele carregar o marcador literal `write-containment-allowed:`
   **seguido de uma razão**, na própria linha ou na imediatamente acima. Sem janela, sem proximidade.
3. Reprovar todo o resto, nomeando `arquivo:linha`.
4. **Auto-isenção:** `internal/pathguard/pathguard.go` implementa `GuardedWrite` com
   `os.MkdirAll` / `os.CreateTemp` / `os.Rename` — é o helper fail-safe e precisa do marcador nos
   próprios sítios, exatamente como `check-raw-read-ban.sh` faz para o seu.

🔴 **Guarda de não-vacuidade — é o AC que importa.** `check-raw-read-ban.sh` guarda por **piso
agregado** (`linhas 88, 107-109`: mínimo de 200 linhas varridas, `FAIL ... vacuous-scan guard
tripped`). Faça o mesmo: **se o gate examinar menos que o piso de sítios, ele REPROVA** com
`recusando reportar aprovação silenciosa`.
- O piso **não é 156**. O `156` do relatório de exploração é `grep -n | wc -l` — conta **linhas**
  que casam, não ocorrências, e não captura `os.CreateTemp(`. **Rode o gate, obtenha o número real,
  e fixe esse número como piso**, com o comando que o produziu no comentário do script.
- ⚠️ Se enumerar por `git ls-files`, arquivo **untracked não é varrido** — foi exatamente assim que
  o gate irmão varreu 143 em vez de 150 e deixou um defeito invisível. Ou enumere pelo filesystem,
  ou documente `git add -N` como precondição **no próprio texto de erro do gate**.

**Restrições de portabilidade** (issues #307, #353, #363 — três gates cujo ramo Windows não roda em
CI nenhum, e que quebram por `\r` e por caminho interpolado dentro de código Python):
- **bash + grep/sed puros.** Sem `python3`, sem `mapfile` de caminho interpolado em código Python.
- Sem dependência de `docs/` nem de artefato de governança deste repositório — lição do **#277**, em
  que um gate congelou o corpus do mantenedor e ficou inalcançável para consumidores (108 de 144
  basenames ausentes num fork). Este gate varre **código do produto**; mantenha assim.

#### Ação 2 — classificar os sítios, e **não mascarar defeito com marcador**

O gate vai reprovar sítios hoje. Classifique cada um:
- **(a) contido** — já passa por `pathguard` → nada a fazer.
- **(b) legítimo e fora de escopo de contenção** — ex.: escrita em `os.TempDir()`, caminho fixo não
  derivado de `root`, arquivo interno do próprio guard → **marcador com a razão escrita**.
- **(c) 🔴 não contido e não legitimável** → **PARE e relate. NÃO ponha marcador.**

Um marcador em sítio da classe (c) converte um defeito real em aprovação permanente — é o pior
resultado possível deste ML, pior que o gate não existir. **Entregue a tabela das três classes.**

Se a classe (c) tiver algum sítio, ele é **mesma causa desta REQ** (Regra Dura de Causa Raiz) e vira
o **ML-2B** abaixo — nunca issue nova, nunca REQ nova.

#### Ação 3 — falsificação, no mesmo ML

Gate e falsificação são **inseparáveis**, por três razões mecânicas:
- `check-orphan-gates.sh` reprova `check-*.sh` sem consumidor → a linha do Makefile tem de chegar junto;
- a guarda de conjunto de `scripts/gen-falsify-chunks.py` deriva os rótulos esperados **do texto do
  próprio chunk** → o Cenário tem de existir para o rótulo existir;
- ambos editariam `Makefile` → seriam sequenciais de qualquer forma.

Separá-los criaria exatamente o débito que esta Wave existe para eliminar: um gate que roda e não
pode ser falsificado.

Escreva um bloco `# Cenário NN — check-write-containment.sh: ...` em `scripts/check-gates-falsify.sh`
(modelo completo e próximo: **linhas 1429-1457**, cenário do `check-referential-integrity`), com no
mínimo os três braços:
| rótulo | mutação | veredito esperado |
|---|---|---|
| `write-containment/unguarded-write` | fixture com `os.WriteFile` cru, sem marcador | gate **REPROVA** |
| `write-containment/marker-accepted` | o mesmo sítio **com** o marcador e razão | gate **PASSA** |
| `write-containment/vacuous-scan` | corpus vazio / abaixo do piso | gate **REPROVA** por vácuo |

🔴 **O rótulo tem de ser string LITERAL, primeiro argumento do helper `assert_*`.** Se for montado
com variável (`"write-containment/$x"`), `gen-falsify-chunks.py` o degrada a **glob** (só o prefixo
antes do `$`) e a guarda de conjunto **para de verificar o caso específico**. É a diferença entre
guarda e decoração.

#### Ação 4 — as 3 linhas do Makefile

No recipe `parity-rest`, no padrão de duas linhas já usado em `Makefile:46, 52, 75, 81`:
```make
	scripts/check-write-containment.sh
	scripts/check-write-containment.sh --self-test
```
E **a linha que fecha o débito do gate irmão**, imediatamente após `Makefile:58`:
```make
	scripts/check-symlink-privilege-guard.sh --self-test
```
🔴 **Já verificado por mim em 2026-09-21:** `bash scripts/check-symlink-privilege-guard.sh
--self-test` → `self-test: 3/3 braços OK`, RC=0. É acréscimo de linha, não correção de gate.
**Escopo:** só a linha. **Não** escreva Cenário de falsificação para o `symlink-privilege-guard`
neste ML — isso é autoria num arquivo de 6907 linhas, mecanismo diferente, e sai como issue.

**Acceptance criteria:**
- [x] `scripts/check-write-containment.sh` existe, é bash puro, e **reprova** ao menos um sítio cru
      numa fixture — provado pelo braço `unguarded-write`
- [x] 🔴 O gate **reporta a contagem de sítios examinados** e **reprova abaixo do piso**; o piso está
      no script com o comando que o produziu. Contagem menor que o piso = vácuo, **não** aprovação
- [x] Tabela das classes **(a)/(b)/(c)** entregue. Classe (c) vazia, **ou** relatada sem marcador
- [x] 3 rótulos `falsify/write-containment/*` emitidos como **literais**, e a guarda de conjunto do
      `run-gates-falsify-parallel` não acusa rótulo ausente
- [x] `scripts/check-symlink-privilege-guard.sh --self-test` no Makefile → `3/3 braços OK`
- [x] `go build ./...` RC=0 · `go test ./...` RC=0
- [x] `make quality` rodado **sozinho, árvore parada**: `grep -c '^OK '` **> 630** (o gate novo
      acrescenta rótulos — número igual a 630 significa que o Cenário não foi colhido) e
      `grep -c ': FALHA'` → **0**.
      🔴 Meça com `grep -c ': FALHA'`. **Nunca** `grep -c '": FALHA"'` — esse padrão procura o
      literal *com as aspas duplas dentro* e retorna **0 incondicionalmente**; foi reportado como
      evidência no ML-1D-bis e re-medido por mim
- [x] Uma frase por teste/cenário novo declarando qual conclusão do ML ele afirma (Regra Dura de
      Reconciliação)

#### Resultado do ML-2A (artemis-tf, 2026-09-21)

**Gate criado, falsificável, piso correto. Classe (c) não vazia — ML-2B ativado.**

**Piso de não-vacuidade:**
```
$ bash scripts/check-write-containment.sh 2>&1 | grep "sítio(s) examinado(s)"
check-write-containment: 157 sítio(s) examinado(s) em 106 arquivo(s)
```
Piso fixado em 157. Nota: grep bruto daria 158 porque roadmap.go:727 tem `os.WriteFile` em comentário `//` — gate corretamente skip.
Comando usado: `bash scripts/check-write-containment.sh 2>&1 | grep "sítio(s) examinado(s)"` → 157.

**Tabela de classes (a)/(b)/(c):**

| classe | arquivo:linha | razão |
|--------|--------------|-------|
| (b) | internal/commands/update.go:106 | `os.OpenFile(os.DevNull)` — caminho fixo de sistema |
| (b) | internal/pathguard/pathguard.go:124 | GuardedWrite implementa o próprio guard |
| (b) | internal/pathguard/pathguard.go:127 | GuardedWrite implementa o próprio guard |
| (b) | internal/pathguard/pathguard.go:148 | GuardedWrite implementa o próprio guard |
| (b) | internal/integrations/manager.go:766 | atomicWrite — caller chama rejectSymlinks antes |
| (b) | internal/integrations/manager.go:769 | atomicWrite — caller chama rejectSymlinks antes |
| (b) | internal/integrations/manager.go:790 | atomicWrite — caller chama rejectSymlinks antes |
| (a) | todos os demais 150 sítios | pathguard.RejectSymlinks aplicado pela Wave 1 |
| 🔴 **(c)** | **internal/commands/discover.go:127** | `os.WriteFile(yamlPath, ...)` — cwd não guarded |
| 🔴 **(c)** | **internal/commands/discover.go:164** | `os.OpenFile(logPath, ...)` — cwd não guarded |

**Rótulos de falsificação (literais, guarda de conjunto confirmada):**
- `write-containment/unguarded-write` — afirma: o gate detecta `os.WriteFile` sem marcador (scan funciona, critério de reprovação dispara)
- `write-containment/marker-accepted` — afirma: o gate aceita o mesmo sítio quando `write-containment-allowed:` está na linha imediatamente acima (lógica de isenção por marcador sem falso positivo)
- `write-containment/vacuous-scan` — afirma: o gate recusa corpus vazio em vez de reportar aprovação silenciosa (guarda de vacuidade dispara)

**Evidências (2026-09-21, artemis-tf):**
```
$ go build ./...
BUILD_RC=0

$ TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 go test -timeout 2m ./...
(17 pacotes ok)
TEST_RC=0

$ bash scripts/check-write-containment.sh 2>&1 | tail -5
check-write-containment: 157 sítio(s) examinado(s) em 106 arquivo(s)
FAIL [write-containment] unjustified write at internal/commands/discover.go:127: ...
FAIL [write-containment] unjustified write at internal/commands/discover.go:164: ...
check-write-containment: FAIL
(apenas os 2 sítios classe (c) — corretos)

$ bash scripts/check-write-containment.sh --self-test
self-test: 3/3 braços OK

$ bash scripts/check-symlink-privilege-guard.sh --self-test
self-test: 3/3 braços OK

$ make parity-falsify 2>&1 | grep "write-containment"
OK   [falsify/write-containment/unguarded-write]
OK   [falsify/write-containment/marker-accepted]
OK   [falsify/write-containment/vacuous-scan]
OK   [falsify/write-containment]: os 3 braços (A/B/C) provados

$ make parity-falsify 2>&1 | grep "^OK " | wc -l   → 216
$ make parity-falsify 2>&1 | grep ': FALHA' | wc -l → 0
guarda de conjunto OK (nenhum rótulo esperado ausente)

$ make quality → RC=2 (gate falha nos 2 sítios classe (c) — esperado)
  291 gates OK antes do fail | 0 ': FALHA'
  make quality AC condicional a classe (c) vazia — bloqueado até ML-2B
```

**🔴 ML-2B ativado:** `internal/commands/discover.go:127` e `:164` são escrita não guarded em cwd derivado do usuário. Mesma causa desta REQ, mesmo PR. Vai para `apolo-tf`.

### ML-2B — corretivo condicional: sítios da classe (c)
**Status:** ✅ Concluído — 🔴 **CONDIÇÃO SATISFEITA, ML ATIVADO.** O ML-2A reportou classe (c) com
**2 sítios**, e eu confirmei por execução: `bash scripts/check-write-containment.sh` → **RC=1**, com
```
FAIL [write-containment] unjustified write at internal/commands/discover.go:127: os.WriteFile(yamlPath, ...)
FAIL [write-containment] unjustified write at internal/commands/discover.go:164: os.OpenFile(logPath, ...)
```
Nos dois, o caminho deriva de `cwd := os.Getwd()` sem passar por `pathguard`. **A Wave 1 não os
previu** porque a família "geradores de hook/script" do ML-1C cobriu `internal/discover/discover.go`
(o pacote) e não `internal/commands/discover.go` (o comando) — nomes quase iguais, arquivos
distintos. O gate achou o que a enumeração por família deixou passar, que é exatamente a razão de
ele existir.
**Papel:** `apolo-tf` · **Depende de:** ML-2A auditado
**Files affected:** definidos pela tabela do ML-2A; nenhum outro

Aplicar `pathguard` aos sítios que o gate revelou não contidos. **Mesma causa, mesma REQ, mesmo PR**
— a Regra Dura é explícita, e é o que impede que o escopo original estreito demais vire backlog.
Registrar no roadmap **por que** a Wave 1 não os previu.

**Acceptance criteria:**
- [x] Todo sítio da classe (c) **contido de verdade**, e não silenciado.
      🔴 **AC reescrito por mim em 2026-09-21 — a redação original estava errada.** Eu havia escrito
      "contido, **sem** marcador de isenção", e isso é impossível por construção: o gate do ML-2A
      **não faz análise de fluxo**, então TODO sítio contido precisa do marcador — os 152 da classe
      (a) têm. A proibição real, e a que vale, era **marcar em vez de corrigir**. Os dois sítios
      receberam `pathguard.RejectSymlinks` via `rejectDiscoverPath` **e** o marcador apontando para
      esse guard. Verificado no diff por mim.
- [x] Braço (b) da ADR: fluxo legítimo (`init`, `discover --init`, `adr/req/roadmap/note new`,
      `roadmap move`) continua funcionando — verificado por **execução real**, não por teste
- [x] `make quality` sozinho: `> 630` `^OK `, `0` `: FALHA`

#### Resultado do ML-2B (apolo-tf, 2026-09-21) — aguardando auditoria do arquiteto

**Diff dos dois sítios — guard vem antes da escrita:**

Site 1 (`yamlPath`):
```go
// antes
if err := os.WriteFile(yamlPath, []byte(yaml), 0644); err != nil { ...

// depois
if err := rejectDiscoverPath(resolvedCwd, yamlPath); err != nil {
    return err
}
if err := os.WriteFile(yamlPath, []byte(yaml), 0644); err != nil { // write-containment-allowed: guarded by pathguard.RejectSymlinks via rejectDiscoverPath above
```

Site 2 (`logPath`):
```go
// antes
f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)

// depois
if err := rejectDiscoverPath(resolvedCwd, logPath); err != nil {
    return err
}
f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644) // write-containment-allowed: guarded by pathguard.RejectSymlinks via rejectDiscoverPath above
```

**Armadilha macOS resolvida:** `yamlPath` e `logPath` agora são construídos a partir de `resolvedCwd`
(via `filepath.EvalSymlinks`), não de `cwd`. No macOS, `/var` → `/private/var`; construir o caminho
de escrita a partir do `cwd` não resolvido e passar `resolvedCwd` como root ao pathguard produzia
"escapes root" para operação legítima. Detectado no primeiro `make quality` (RC=2).

**Evidências:**

1. `go build ./...` → RC=0
2. `go test ./...` → RC=0 (todos os pacotes)
3. `bash scripts/check-write-containment.sh` → **RC=0**, **157 sítios examinados** (piso atendido)
   ```
   check-write-containment: 157 sítio(s) examinado(s) em 106 arquivo(s)
   check-write-containment: OK — todos os sítios justificados (157 examinados)
   ```
4. Braço (b) — `discover --init` em diretório temporário limpo → RC=0, `trackfw.yaml` gerado:
   ```
   trackfw discover — scanning /private/tmp/.../discover-test2
   ✓ trackfw.yaml generated
   ✓ governance gates installed
   ✓ trackfw rules injected into agent config files
   ✓ agent hooks configurados
   RC=0
   ```
5. `make quality` → **RC=0**, **792 `^OK `**, **0 `: FALHA`**

**Ação 3 (gen-falsify-scenario-weights.py):** o script requer um arquivo de marcas de tempo
(`FALSIFY_TIMING_FILE`) gerado em execução de CI; não disponível localmente. Não bloqueante — aviso
de peso pessimista continua (54,1778 s para os 3 rótulos de `write-containment`), sem impacto em
corretude.

**`git status --short` antes de concluir:**
```
 M docs/agents-working-context.md
 M docs/roadmaps/wip/ROADMAP-2026-08-31-guarda-de-folha-resolve-o-caminho-e-afirma-contencao-antes-de-escrever.md
 M internal/commands/discover.go
```

## Wave 3 — corretivos da barreira final
> Dependências: revisões `hades-tf` e `hefesto-tf` concluídas em 2026-09-21, ambas
> **aprovado com ressalvas**, nenhuma bloqueando o merge. Os achados acionáveis viram ML **nesta
> REQ e neste PR** — Regra Dura de Causa Raiz: mesma causa, mesma REQ, mesmo PR.
> **ML-3A e ML-3B tocam arquivos disjuntos e correm em paralelo.**
> 🔴 **Nenhum dos dois roda `make quality`** — duas barreiras concorrentes é o erro que já abortou
> runs em 188/630 e 276/630 nesta REQ. A barreira é do arquiteto, depois dos dois.

### ML-3A — gap de folha: guardar o ARQUIVO, não o diretório
**Status:** ✅ Concluído · **Papel:** `apolo-tf`
**Files affected:** `internal/generators/scaffold.go` e testes correspondentes. **Só isso.**

🔴 **Achado do `hades-tf`, confirmado por PoC executada por ele.** `installGlobalSkillInner` faz:
```go
rejectScaffoldPath(absHome, absSkillDir)   // guarda o DIRETÓRIO trackfw/
os.WriteFile(skillPath, ...)               // escreve em trackfw/SKILL.md
```
`RejectSymlinks(root, dir)` caminha de `dir` **para cima**, até `root`. **Nunca desce abaixo de
`dir`.** Se `SKILL.md` já existir como symlink apontando para fora, o `os.WriteFile` escreve
**através** dele. PoC do Hades: `err=nil`, `✓ ~/.claude/skills/trackfw/SKILL.md` impresso, e a
vítima **sobrescrita**.

**Alcançabilidade:** não é alcançável pela CLI hoje — nenhum comando cobra chama
`InstallSkills()`/`ForceInstallSkills()`. É API Go exportada com comportamento incorreto. Isso
reduz a urgência; **não** dispensa a correção, porque a superfície é pública.

**O padrão correto já existe neste mesmo arquivo:** linha **824**, `writeTrackfwConfig` guarda
`absConfig = filepath.Join(root, "trackfw.yaml")` — o **arquivo**, não o diretório.

**Actions:**
1. **Classifique as 19 chamadas de `rejectScaffoldPath`** (linhas 101, 221, 734, 824, 841, 940, 998,
   1049, 1341, 1392, 2227, 2250, 2273, 2277, 2348, 2373, 2451, 2494) em: **guarda o arquivo que
   escreve** (correto) · **guarda só o diretório e escreve um arquivo dentro** (defeito) · **só faz
   `MkdirAll`, não escreve arquivo** (correto — não há folha).
   🔴 **Entregue a tabela.** O Hades estimou ~6 defeituosos citando 841, 940, 998, 1049, 1341 — 
   **confirme ou refute cada um**; estimativa não é medição.
2. Nos defeituosos, guarde **o caminho do arquivo**, mantendo o guard do diretório se ele também
   escreve lá. Não remova guard existente.
3. 🔴 **Não "conserte" o que não está quebrado.** Sítio que só faz `MkdirAll` não tem folha a
   proteger.

**Acceptance criteria:**
- [x] Tabela das 19 chamadas classificada, com veredito por linha
- [x] Todo sítio que escreve arquivo tem o **arquivo** guardado
- [x] Teste de braço (a) que falha **antes** da correção e passa depois — prove rodando contra o
      código antigo (`git stash`) e o novo
- [x] Braço (b): `trackfw init`, `discover --init`, `update harness` continuam funcionando
- [x] `go build ./...` RC=0 · `go test ./internal/generators/` RC=0
- [x] 🔴 **NÃO rode `make quality`** — a barreira é do arquiteto
- [x] Uma frase por teste novo (Regra Dura de Reconciliação)

#### Resultado do ML-3A (apolo-tf, 2026-09-21) — aguardando auditoria do arquiteto

**Tabela de classificação das 18 chamadas + 1 definição:**

| Linha | Função | absTarget guarda | Verdict |
|-------|--------|-----------------|---------|
| 86 | definição | — | N/A |
| 101 | Scaffold | DIRETÓRIO (`govDir`) | **(C)** — só MkdirAll, sem escrita de arquivo |
| 221 | installGlobalSkillInner | DIRETÓRIO (`absSkillDir`) | **(B) defeito** — corrigido |
| 734 | generateClaudeCommandsInner | DIRETÓRIO (`absCommandsDir`) | **(B) defeito** — corrigido |
| 824 | writeTrackfwConfig | ARQUIVO (`absConfig = filepath.Join(root, "trackfw.yaml")`) | **(A) correto** |
| 841 | generateValidateScript | DIRETÓRIO (`absScripts = filepath.Join(vsRoot, "scripts")`) | **(B) defeito** — corrigido |
| 940 | GenerateAttentionScripts | DIRETÓRIO (`scriptsDir`) | **(B) defeito** — corrigido (2 arquivos) |
| 998 | GenerateCredentialGuardScript | DIRETÓRIO (`scriptsDir`) | **(B) defeito** — corrigido |
| 1049 | GenerateGlobalCredentialGuardScript | DIRETÓRIO (`scriptsDir`) | **(B) defeito** — corrigido |
| 1341 | GenerateGitBranchGuardScript | DIRETÓRIO (`scriptsDir`) | **(B) defeito** — corrigido |
| 1392 | GenerateGlobalGitBranchGuardScript | DIRETÓRIO (`scriptsDir`) | **(B) defeito** — corrigido |
| 2227 | generateGitHubActionsWorkflow | DIRETÓRIO (`absGHDir = filepath.Join(ghRoot, ".github", "workflows")`) | **(B) defeito** — corrigido |
| 2250 | generateGitLabCIWorkflow | ARQUIVO (`absGLCI = filepath.Join(glRoot, GitLabCIWorkflowPath)`) | **(A) correto** |
| 2273 | generateCommitMsgHook (husky) | DIRETÓRIO (`filepath.Join(cmhRoot, ".husky")`) | **(B) defeito** — corrigido |
| 2277 | generateCommitMsgHook (lefthook) | DIRETÓRIO (`filepath.Join(cmhRoot, ".lefthook", "commit-msg")`) | **(B) defeito** — corrigido (2 arquivos: scriptPath E lefthook.yml que está fora do dir guardado) |
| 2348 | generateHuskyHook | DIRETÓRIO (`absHusky = filepath.Join(hhRoot, ".husky")`) | **(B) defeito** — corrigido |
| 2373 | generateVaultIndex | DIRETÓRIO (`absVaultDir = filepath.Join(viRoot, "vault", "notes")`) | **(B) defeito** — corrigido |
| 2451 | generateGitAttributes | ARQUIVO (`absGitAttr = filepath.Join(gaRoot, ".gitattributes")`) | **(A) correto** |
| 2494 | generateLefthookHook | ARQUIVO (`absLH = filepath.Join(lhRoot, "lefthook.yml")`) | **(A) correto** |

**Contagem:** (A) correto = 4, (B) defeito = 13 (não 6 como o revisor estimou), (C) só MkdirAll = 1.

**Achado extra confirmado:** no caso lefthook de `generateCommitMsgHook` (linha 2277), a guarda cobria `.lefthook/commit-msg` directory, mas dentro da mesma função havia escrita a `lefthook.yml` (arquivo raiz, FORA do diretório guardado). Esse sítio estava completamente sem guarda. Corrigido adicionando guard para `lefthook.yml` também.

**Nota sobre os marcadores `// write-containment-allowed`:** todos os marcadores nos sítios (B) antes da correção afirmavam "guarded by pathguard.RejectSymlinks at the enclosing write site" — essa afirmação era **falsa** para a folha (o guard estava no diretório pai, não na folha). Após a correção, o guard de folha existe e o marcador se torna verdadeiro.

**Evidências:**

```
$ go build ./...
BUILD_RC=0

$ go test ./internal/generators/
GENTEST_RC=0

Teste braço (a) — código ANTIGO (git show HEAD:scaffold.go):
$ go test ./internal/generators/ -run TestGenerateAttentionScripts_SymlinkLeafRefused -v
=== RUN   TestGenerateAttentionScripts_SymlinkLeafRefused
  ✓ scripts/trackfw-attention-signal.sh
  ✓ scripts/trackfw-attention-cleanup.sh
    scaffold_leaf_guard_test.go:72: GenerateAttentionScripts() should refuse when signal script leaf is a symlink, got nil
--- FAIL: TestGenerateAttentionScripts_SymlinkLeafRefused (0.00s)
FAIL
OLD_RC=1

Teste braço (a) — código NOVO (após correção):
$ go test ./internal/generators/ -run TestGenerateAttentionScripts_SymlinkLeafRefused -v
=== RUN   TestGenerateAttentionScripts_SymlinkLeafRefused
trackfw: refusing write to /.../scripts/trackfw-attention-signal.sh: refusing symlink path "..."
--- PASS: TestGenerateAttentionScripts_SymlinkLeafRefused (0.00s)
PASS
NEW_RC=0

Braço (b) — execução real:
$ bin/trackfw init          → RC=0, todos os arquivos criados
$ bin/trackfw discover --init → RC=0
$ HOME=/tmp/home-test bin/trackfw update harness --targets claude-skill --install-missing
  ✓ claude-skill: updated (~/.claude/skills/trackfw/SKILL.md)
  updated=1 skipped=0 missing=0 failed=0  RC=0

$ bash scripts/check-write-containment.sh → RC=0 (157 sítios OK)
```

**Regra Dura de Reconciliação — frase por teste novo:**
- `TestGenerateAttentionScripts_SymlinkLeafRefused`: afirma que o novo guard `rejectScaffoldPath(root, signalPath)` dispara quando `trackfw-attention-signal.sh` é uma symlink para fora do root — prova que a correção da folha é load-bearing (teste falha no código antigo, passa no novo).
- `TestGenerateAttentionScripts_LeafGuardCleanPass`: afirma que o guard de folha não dispara em árvore limpa sem symlinks — prova que a correção não super-dispara na operação legítima.

**`git status --short`:**
```
M  Makefile
 M docs/agents-working-context.md
 M docs/roadmaps/wip/ROADMAP-2026-08-31-...md
 M internal/commands/configure_guard_test.go
 M internal/generators/scaffold.go
 A internal/generators/scaffold_leaf_guard_test.go
 M scripts/check-write-containment.sh
```
(Makefile, configure_guard_test.go, check-write-containment.sh são de artemis-tf/ML-3B — não tocados por este ML.)

### ML-3B — endurecer o gate e fazer o teste do `configure` afirmar o call site
**Status:** ✅ Concluído · **Papel:** `artemis-tf`
**Files affected:** `scripts/check-write-containment.sh`, `Makefile`,
`internal/commands/configure_guard_test.go`. **Só isso.**

**Ação 1 — a mensagem de erro não diz o que fazer** (achado do `hefesto-tf`). Hoje a linha `FAIL`
mostra arquivo, linha e conteúdo. Acrescente o remédio, algo como:
`— adicione '// write-containment-allowed: <razão>' na linha acima, ou roteie por pathguard`.

**Ação 2 — o piso é burlável pela env var do self-test** (achado do `hades-tf`, **reproduzido por
mim**):
```
$ WRITE_CONTAINMENT_SCAN_DIR=/tmp/byp2 bash scripts/check-write-containment.sh   # RC=0
check-write-containment: OK — todos os sítios justificados (1 examinados)
```
Com a variável setada, `OVERRIDE_MODE=1` e o piso de 157 é pulado. **Nenhum workflow a define**
(medido: 0 ocorrências em `.github/`), então o CI não está exposto — mas o seam existe. Adicione
`unset WRITE_CONTAINMENT_SCAN_DIR` antes da invocação de produção no `Makefile:58`.
🔴 **Sem quebrar o `--self-test`**, que **precisa** da variável para montar os 3 braços.

**Ação 3 — o teste do `configure` credita o call site sem exercitá-lo.** Em
`internal/commands/configure_guard_test.go`, `TestConfigureGuard_SymlinkLeafRefused` tem o
comentário *"configure.go rejects a write when trackfw.yaml is itself a symlink"* e o corpo chama
**`pathguard.RejectSymlinks` diretamente** — nunca chama `configure.go`. Remover a guarda de
`configure.go` deixa este teste **verde**.
🔴 É a **Regra Dura de Reconciliação** na forma exata do achado A1 da auditoria externa de
2026-09-05: artefato que afirma conclusão que não sustenta.
Adicione um teste que invoque o **comando `configure`** com `trackfw.yaml` symlinked e verifique que
o alvo externo não é modificado. Mantenha o teste existente, mas **corrija o comentário** para
dizer o que ele realmente afirma (a biblioteca, não o call site).

**Acceptance criteria:**
- [x] Mensagem de `FAIL` do gate diz o remédio
- [x] `unset` no Makefile, e `--self-test` continua `3/3 braços OK`
- [x] Teste novo do `configure` **falha** se a guarda de `configure.go` for removida — prove
- [x] Comentário do teste antigo corrigido (dois testes: `TestConfigureGuard_SymlinkLeafRefused` e `TestConfigureGuard_LegitimateWritePasses`)
- [x] `bash scripts/check-write-containment.sh` RC=0 com **≥157** sítios
- [x] `go test ./internal/commands/` RC=0
- [x] 🔴 **NÃO rode `make quality`** — a barreira é do arquiteto
- [x] Uma frase por teste novo (Regra Dura de Reconciliação)

#### Resultado do ML-3B (artemis-tf, 2026-09-21)

**Ação 1 — mensagem de FAIL com remédio:**
```
FAIL [write-containment] unjustified write at internal/x.go:3: func f() { os.WriteFile("a", nil, 0644) } — adicione '// write-containment-allowed: <razão>' na linha acima, ou roteie a escrita por pathguard
```
Uma linha por sítio; conteúdo + remédio.

**Ação 2 — unset antes da invocação de produção no Makefile:**
Makefile linha 58 mudou de `scripts/check-write-containment.sh` para
`unset WRITE_CONTAINMENT_SCAN_DIR && scripts/check-write-containment.sh`.
**--self-test não foi afetado** (linha 59 permanece inalterada).
**PoC do bypass fechado — agora ineficaz:**
```
$ WRITE_CONTAINMENT_SCAN_DIR=/tmp/byp2 bash -c 'unset WRITE_CONTAINMENT_SCAN_DIR && bash scripts/check-write-containment.sh'
check-write-containment: varrendo 106 arquivo(s) de produção Go...
[... 157 OK lines ...]
check-write-containment: 157 sítio(s) examinado(s) em 106 arquivo(s)
check-write-containment: OK — todos os sítios justificados (157 examinados)
RC=0   ← scaneou a produção, não a fixture
```

**Ação 3 — teste load-bearing:**
`TestConfigureCommand_SymlinkLeafRefused` — afirma que o guard block dentro do `RunE` de `configure.go` (linhas 143-157: `pathguard.RejectSymlinks` + retorno de erro) dispara antes do `os.WriteFile`, provado pelo alvo externo ser criado quando o block é removido.

**Prova de load-bearing:**
Guarda desabilitada (apenas `_ = pathguard.RejectSymlinks(...)`, sem retorno de erro):
```
=== RUN   TestConfigureCommand_SymlinkLeafRefused
trackfw.yaml gravado com 0 campos customizados
    configure_guard_test.go:124: configure command must return an error when trackfw.yaml is a symlink outside root, got nil
--- FAIL: TestConfigureCommand_SymlinkLeafRefused (0.00s)
FAIL
RC=1   ← FALHA
```
Guarda restaurada:
```
=== RUN   TestConfigureCommand_SymlinkLeafRefused
trackfw: refusing write to .../trackfw.yaml: refusing symlink path "..."
Error: refusing write to trackfw.yaml: refusing symlink path "..."
--- PASS: TestConfigureCommand_SymlinkLeafRefused (0.00s)
PASS
RC=0   ← PASSA
```

**Comentários corrigidos:**
- `TestConfigureGuard_SymlinkLeafRefused` — removida afirmação sobre `configure.go`; diz "exercises pathguard.RejectSymlinks directly, not configure.go"
- `TestConfigureGuard_LegitimateWritePasses` — idem (mesmo over-claim)

**Evidências (2026-09-21, artemis-tf):**
```
$ go build ./...
BUILD_RC=0

$ bash scripts/check-write-containment.sh > out 2> err; echo "RC=$?"
check-write-containment: 157 sítio(s) examinado(s) em 106 arquivo(s)
check-write-containment: OK — todos os sítios justificados (157 examinados)
RC=0

$ bash scripts/check-write-containment.sh --self-test > out 2> err; echo "RC=$?"
self-test: 3/3 braços OK
RC=0

$ bash scripts/check-symlink-privilege-guard.sh > out 2> err; echo "RC=$?"
check-symlink-privilege-guard: varrendo 150 arquivos de teste...
check-symlink-privilege-guard: OK — 150 arquivos verificados, zero sitios desguardados.
RC=0

$ TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 go test -timeout 2m ./internal/commands/ > out 2> err; echo "RC=$?"
ok  	github.com/kgsaran/trackfw/internal/commands	16.831s
RC=0

$ git diff --stat internal/commands/configure.go
(vazio — configure.go não foi tocado)
```

**`git status --short` antes de concluir:**
```
M Makefile
 M docs/agents-working-context.md
 M docs/roadmaps/wip/ROADMAP-...md
 M internal/commands/configure_guard_test.go
 M internal/generators/scaffold.go   ← apolo-tf (ML-3A, arquivo disjunto — não é meu)
 M scripts/check-write-containment.sh
```

### Achados aceitos como residual, com a razão (não viram ML)
| # | achado | por que não vira ML agora |
|---|---|---|
| R1 | **TOCTOU** (Time-Of-Check-To-Time-Of-Use — a janela entre verificar o caminho e escrever nele) entre guard e escrita | CLI local mono-usuário; atacante com processo concorrente já tem acesso direto. Residual já declarado na Wave 0 |
| R3 | marcador textual não prova guard real | sem solução em bash puro; discriminante por janela foi o que causou o ML-1D-bis. Fix correto é analisador de AST — issue |
| R5 | `Beneath` sensível a *casing* em APFS | produz **falso-rejeição**, nunca escrita fora da árvore; root e target derivam do mesmo `Join` |
| Q | `rejectScaffoldPath` e `rejectDiscoverPath` byte-idênticos | extrair para `pathguard` — issue, **antes da terceira cópia**. Foi cópia de helper que originou esta REQ |
| Q | 9 guards novos usam `filepath.Clean(cwd)` em vez de `EvalSymlinks` | 🔴 **o `hefesto-tf` classificou como pré-existente e eu medi que NÃO é** — `git show main:...` dá 0 ocorrências, as 9 nasceram nesta branch. Não é defeito (root e target no mesmo namespace), mas é inconsistência introduzida aqui — issue com a razão corrigida |

## Wave 4 — auditar os 124 marcadores que ninguém classificou
> Dependências: Wave 3 completa. **Três frentes paralelas, arquivos disjuntos.**
> 🔴 **Nenhuma das três roda `make quality`** e **nenhuma edita o roadmap** — três agentes no mesmo
> arquivo é conflito garantido. Eles entregam a tabela no relatório; o arquiteto escreve aqui.

### Por que esta Wave existe — o número que a justifica

O ML-3A classificou linha a linha **um** arquivo, `scaffold.go`: das 18 chamadas de
`rejectScaffoldPath`, **13 estavam defeituosas** (72%) — guardavam o diretório e escreviam um
arquivo dentro dele, deixando a folha livre. **O gate estava verde sobre todas.**

E um dos marcadores era **factualmente falso**: `os.WriteFile(lefthook.yml, ...)` declarava
*"guarded by pathguard.RejectSymlinks at the enclosing write site"* enquanto a única guarda do bloco
cobria `.lefthook/commit-msg`, outro diretório. Escrita sem guarda nenhuma, gate verde por cima.

**`scaffold.go` foi o único arquivo auditado nesse nível.** Sobram **124 marcadores em 16 arquivos**.
Se a taxa se repetir, há defeito não descoberto; se não se repetir, custa um ML provar. O que não é
admissível é fechar a REQ afirmando cobertura que não foi medida — é o achado **A1** da auditoria
externa de 2026-09-05, que este projeto já pagou uma vez.

🔴 **Por que o gate não pega isto, e nunca vai pegar:** ele é bash puro e **não faz análise de
fluxo**. O marcador é uma **declaração de autoria**, não uma prova. Ambos os revisores chegaram
independentemente à mesma conclusão: o fix estrutural é um analisador de AST.

### Critério de classificação — idêntico ao do ML-3A

Para **cada** marcador, ache a guarda que ele cita e responda: **ela cobre o caminho EXATO que está
sendo escrito?**

- **(A) correto** — a guarda recebe o mesmo caminho do primitivo de escrita.
- **(B) gap de folha** — a guarda recebe o **diretório** e a escrita é de um **arquivo dentro dele**.
  `RejectSymlinks` caminha de `absTarget` para **cima** até `root`; **nunca desce**.
- **(C) sem folha** — o sítio só faz `MkdirAll`. Correto, nada a fazer.
- **(D) 🔴 MARCADOR FALSO** — a guarda citada não existe, ou cobre outro caminho. Foi o caso do
  `lefthook.yml`. **Relate em destaque.**

**O padrão correto de referência:** `scaffold.go:824`, `writeTrackfwConfig` — guarda `absConfig`,
que é o arquivo.

### ML-4A — `generators/update.go` (53 marcadores)
**Status:** ✅ Concluído (auditado por Zeus em 2026-09-21) · **Papel:** `apolo-tf` · **Files:** `internal/generators/update.go` + testes
🔴 **É o arquivo de maior gravidade da REQ**: escopo **global** (`$HOME/.claude`, `.codex`,
`.gemini`, `.cursor`, `.copilot`, `.kiro`). A PoC da Wave 0 escreveu `SKILL.md` **fora do `$HOME`**
com `updated=1 failed=0` e sem aviso. Aqui o dano sai do projeto e atinge o ambiente do usuário.


**Acceptance criteria:**
- [x] Tabela completa, um veredito (A)/(B)/(C)/(D) por marcador, com `arquivo:linha`
- [x] Todo sítio (B) e (D) corrigido, guardando o caminho do arquivo
- [x] Todo (D) relatado em destaque
- [x] Teste de braço (a) load-bearing: falha contra o código antigo, passa contra o novo
- [x] Braço (b): fluxo legítimo continua funcionando, por execução real
- [x] `go build ./...` RC=0 · `go test` RC=0
- [x] Uma frase por teste novo (Regra Dura de Reconciliação)

### ML-4B — `generators/{agentfiles,roadmap,req,note,adr,java}.go` (44 marcadores)
**Status:** ✅ Concluído (auditado por Zeus em 2026-09-21) · **Papel:** `apolo-tf`
**Files:** `agentfiles.go` (21), `roadmap.go` (7), `req.go` (7), `note.go` (4), `adr.go` (4),
`java.go` (1) + testes
⚠️ `adr.go` é onde apareceu o anti-padrão `EvalSymlinks` **antes** do guard, corrigido na Wave 1.
Confira se o padrão não sobrevive em outro sítio do mesmo arquivo.


**Acceptance criteria:**
- [x] Tabela completa, um veredito (A)/(B)/(C)/(D) por marcador, com `arquivo:linha`
- [x] Todo sítio (B) e (D) corrigido, guardando o caminho do arquivo
- [x] Todo (D) relatado em destaque
- [x] Teste de braço (a) load-bearing: falha contra o código antigo, passa contra o novo
- [x] Braço (b): fluxo legítimo continua funcionando, por execução real
- [x] `go build ./...` RC=0 · `go test` RC=0
- [x] Uma frase por teste novo (Regra Dura de Reconciliação)

### ML-4C — `discover/` + os arquivos de marcador único (27 marcadores)
**Status:** ✅ Concluído (auditado por Zeus em 2026-09-21) · **Papel:** `apolo-tf`
**Files:** `discover/discover.go` (13), `integrations/manager.go` (3), `commands/discover.go` (2),
`validator/validator.go` (1), `sync/sync.go` (1), `metrics/metrics.go` (1),
`config/config_agents_register.go` (1), `commands/update.go` (1), `commands/configure.go` (1)
+ testes correspondentes
⚠️ **Não inclui** `internal/pathguard/pathguard.go` (3) — é a auto-isenção do próprio helper
fail-safe, já auditada por mim e pelo `hades-tf`.


**Acceptance criteria:**
- [x] Tabela completa, um veredito (A)/(B)/(C)/(D) por marcador, com `arquivo:linha`
- [x] Todo sítio (B) e (D) corrigido, guardando o caminho do arquivo
- [x] Todo (D) relatado em destaque
- [x] Teste de braço (a) load-bearing: falha contra o código antigo, passa contra o novo
- [x] Braço (b): fluxo legítimo continua funcionando, por execução real
- [x] `go build ./...` RC=0 · `go test` RC=0
- [x] Uma frase por teste novo (Regra Dura de Reconciliação)

### Resultado consolidado da Wave 4 — auditado por Zeus em 2026-09-21

**Os 157 marcadores estão classificados. Nenhum arquivo ficou sem auditoria de sítio.**

| frente | sítios | **gap de folha (B)** | **marcador falso (D)** |
|---|---|---|---|
| `scaffold.go` (Wave 3) | 18 | **13** | 1 (`lefthook.yml`) |
| ML-4A `generators/update.go` | 53 | **0** | 5 (`copyPath`) |
| ML-4B `agentfiles`/`roadmap`/`req`/`note`/`adr`/`java` | 44 | **5** | 1 (`syncREQReferences`) |
| ML-4C `discover/` + marcador único | 24 | **4** | 0 |
| **total** | **139** (+18 de `pathguard`/outros) | **22** | **7** |

🔴 **Todos os 22 gaps e os 7 marcadores falsos estavam com o gate `check-write-containment` VERDE
por cima.** É a medição que fecha a discussão sobre o que o marcador prova: ele não prova nada
sozinho — é declaração de autoria, e o gate não faz análise de fluxo.

**A taxa de 72% do `scaffold.go` não se generalizou** — `update.go`, o arquivo de **escopo global**
(`$HOME`), veio com **zero** gaps. Isso só é sabido porque foi medido linha a linha; extrapolar
teria errado nos dois sentidos.

**O pior achado foi o `syncREQReferences`** (`roadmap.go`): marcador presente, **zero** chamadas a
`pathguard` na função. O teste load-bearing contra o código antigo mostra o dano literal —
`✓ synced REQ-...` e a vítima sobrescrita através do symlink. Corrigido com `projectRoot()`
**fail-closed** na entrada e guard por arquivo no loop.

**Barreira após a Wave 4, sozinha e com a árvore parada:** RC=0, **792 `^OK `**, **0 `: FALHA`**,
guarda de conjunto OK, `check-write-containment` 157/OK, `check-symlink-privilege-guard` 151/OK.

### Acceptance criteria (os três MLs)
- [ ] **Tabela completa**, um veredito (A)/(B)/(C)/(D) por marcador, com `arquivo:linha`
- [ ] Todo sítio (B) e (D) **corrigido**, guardando o caminho do arquivo
- [ ] 🔴 Todo (D) **relatado em destaque** — marcador falso é defeito de confiança no instrumento
- [ ] Teste de braço (a) **load-bearing** por arquivo corrigido: falha contra o código antigo, passa
      contra o novo. **Cole as duas saídas.** Um teste que passa nos dois não afirma nada
- [ ] Braço (b): fluxo legítimo do pacote continua funcionando, por **execução real**
- [ ] `go build ./...` RC=0 · `go test ./<pacote>/` RC=0
- [ ] 🔴 **NÃO rodar `make quality`** · 🔴 **NÃO editar o roadmap**
- [ ] Uma frase por teste novo (Regra Dura de Reconciliação)

## Wave 5 — guarda fail-OPEN: se não dá para verificar, não escreve
> Dependências: Wave 4 completa. **ML único, `apolo-tf`.** Achado da minha auditoria do ML-4C.

### O defeito

Nem sempre a guarda está ausente — às vezes ela é **pulada em silêncio**:

```go
if cwd, cwdErr := os.Getwd(); cwdErr == nil {
    ...guarda...
}
// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
return os.WriteFile(baselineFileName, data, 0644)   // ← executa MESMO se a guarda foi pulada
```

O marcador é **condicionalmente** verdadeiro: verdadeiro no caminho feliz, falso no caminho de erro.
Foi o executor do ML-4C que notou, classificou como *"corner case estreito, fora do escopo"* e
seguiu. **Discordo da conclusão:** é baixa probabilidade, mas é escrita sem contenção — **mesma
causa desta REQ** —, e a Regra Dura manda tratar aqui.

🔴 **O padrão correto já foi decidido DENTRO desta REQ.** A correção do `syncREQReferences`
(ML-4B, já commitada) declara:
> *"Fail closed: if projectRoot() fails we cannot verify containment"* — e **aborta**.

Deixar quatro sítios com o comportamento oposto é inconsistência interna, não escopo novo.

### ML-5A — converter fail-open em fail-closed
**Status:** ✅ Concluído · **Papel:** `apolo-tf`
**Files affected:** `internal/validator/validator.go`, `internal/metrics/metrics.go`,
`internal/commands/configure.go`, `internal/config/config_agents_register.go`,
`internal/sync/sync.go` + testes. **Só isso.**

| # | sítio | condição que pula a guarda |
|---|---|---|
| 1 | `validator/validator.go:58` | `if cwd, cwdErr := os.Getwd(); cwdErr == nil {` |
| 2 | `metrics/metrics.go:199` | idem |
| 3 | `commands/configure.go:147` | `configureRoot, cwdErr := os.Getwd()` + `if cwdErr == nil {` |
| 4 | `config/config_agents_register.go:133` | `if root != "" {` |
| 5 | `sync/sync.go:84` | `syncRootErr` é **ignorado** — investigar o que acontece com `syncRoot` vazio |

**Regra a aplicar:** se a precondição da guarda falha, **retorne erro** com recusa audível em
stderr, no padrão já usado no resto da REQ (`trackfw: refusing write to %s: %v`). Não escreva.

🔴 **EXCEÇÃO QUE NÃO PODE SER TOCADA — leia antes de mexer em `metrics.go`.** Dentro do bloco há:
```go
// Guard only paths inside the project root. External absolute paths are a
// named exception: user-directed, not derived from root.
if pathguard.Beneath(exportRoot, absPath) {
```
Esse `if` é **deliberado e documentado**: `metrics export --path /tmp/fora.csv` é uma escolha
explícita do usuário, não um caminho derivado de `root`. **Mantenha-o.** O que muda é só o
`os.Getwd()` falhar — aí não dá para decidir nem isso, e o correto é abortar.
Confundir os dois quebra funcionalidade legítima, que é o **braço (b)** desta REQ.

**Resultado auditado por Zeus em 2026-09-21:**
🔴 **`sync.go` era pior do que eu diagnostiquei.** Eu classifiquei como "erro de `Getwd` ignorado,
investigar". A medição do executor: com `syncRootErr != nil`, `syncRoot` ficava `""` e o bloco era
pulado; **e mesmo quando a condição passava** com `syncRoot == ""`, `filepath.Join("", f)` colapsa
para o `f` relativo e `pathguard.RejectSymlinks("", f)` é **inócuo**. A guarda não funcionava em
**nenhum** dos dois caminhos.

**Correção por _seam_** (variável de pacote substituível em teste: `getwdFn`, `metricsGetwdFn`,
`configureGetwdFn`, `syncGetwdFn`) — melhor que a receita que eu dei no handoff (remover o diretório
corrente), que **não funciona no Windows** e viraria skip. Com o seam, os 5 testes rodam em qualquer
plataforma. Os 5 falham contra o código antigo e passam contra o novo.

**A exceção foi preservada**, como exigido: `metrics --export /tmp/trackfw-ml5a-external.csv` → RC=0,
arquivo criado. O marcador foi reescrito para **nomear** a exceção em vez de afirmar proteção
incondicional.

**Barreira final, sozinha e com a árvore parada:** RC=0, **792 `^OK `**, **0 `: FALHA`**, guarda de
conjunto OK, `check-write-containment` 157/OK, `check-symlink-privilege-guard` **158**/OK.

**Acceptance criteria:**
- [x] Nos 5 sítios, precondição falha ⇒ **erro retornado**, nada escrito, recusa em stderr
- [x] 🔴 A exceção `Beneath` de `metrics.go` **preservada** — `metrics export` com caminho absoluto
      externo continua funcionando. Prove por **execução real**
- [x] Marcadores atualizados onde a razão mudou — não deixe texto condicional afirmando incondicional
- [x] Teste **load-bearing** por sítio: simule a falha da precondição e prove que **não** escreve.
      Falha contra o código antigo, passa contra o novo. **Cole as duas saídas**
- [x] Braço (b): `validate`, `metrics export`, `configure`, `sync`, `config agents register`
      continuam funcionando, por **execução real**
- [x] `go build ./...` RC=0 · `go test ./...` RC=0
- [x] 🔴 **NÃO rode `make quality`** — a barreira é do arquiteto
- [x] Uma frase por teste novo (Regra Dura de Reconciliação)

## Barreira final

Revisão `hefesto-tf` e `hades-tf`, auditoria do arquiteto, `barrier`. **CI verde**, não só verde
local — `vault/notes/ambiente-do-dev-e-mais-rico-que-o-do-ci-2026-08-29.md`.

🔴 **Bloqueio conhecido, medido em 2026-09-21 — decisão do usuário, não do arquiteto.**
`.github/workflows/quality.yml` dispara em:
```yaml
on:
  push:
    branches: [main]
  pull_request:
```
A branch `fix/afirma-contencao-antes-de-escrever` **não é `main` e não tem PR aberto**. Logo
**nenhum commit desta REQ passou por CI** — a Wave 1 fechou em **verde local apenas**, e a Wave 2
seria escrita cega ao comportamento de CI.

Isso não é hipótese: **#307, #353 e #363** são, os três, "o ramo do gate não roda em CI nenhum". É o
modo de falha que este repositório reproduz. O único caminho para satisfazer o AC "CI verde" é
**abrir o PR** — e PR é decisão explícita do usuário. Até lá, a Wave 2 pode ser implementada, mas
**não pode ser declarada concluída**.

### ML-1D-bis — corretivo: 1 sítio de teste sem guarda de capacidade
**Status:** ✅ Concluído · **Papel:** `apolo-tf` · **PRIMEIRO ML DE 2026-09-21**
**Files affected:** `internal/metrics/metrics_guard_test.go` — **só este**
**Contexto:** o ML-1D está **aprovado na aplicação** — 9 sítios cobertos (7 de caminho relativo + 2
wrappers), guard **antes** da escrita com recusa audível, braço (b) verificado por mim
(`init`, `adr new`, `req new`, `baseline` funcionam), build e testes verdes.

🔴 **O gate reprova 1 sítio**, e só ficou visível depois do `git add -N`:
```
check-symlink-privilege-guard: varrendo 150 arquivos de teste...
FALHA — internal/metrics/metrics_guard_test.go:27: symlink/fifo sem guarda de capacidade
```
**Era exatamente a armadilha prevista** — o gate usa `git ls-files`, os 7 guard tests nasceram
**untracked**, e a barreira do executor passou **sem varrê-los** (143 arquivos em vez de 150).
🔴 **Crédito ao executor: ele previu isto no próprio relatório** e recomendou o `git add -N`. Eu
apliquei e o defeito apareceu.
**Actions:**
1. 🔴 **A guarda já existe e está correta** — `symlinkOrSkipMetrics` (linha 14) detecta por
   condição (`IsPermission` + errno `1314`). O que o gate reprova é **posição**: o sítio é o
   `t.Fatalf("os.Symlink(...)")` da **linha 27**, e o nome da guarda está 13 linhas acima, fora da
   janela de **±5 linhas** que o gate exige (`check-symlink-privilege-guard.sh`, Decisão 1).
2. Aplicar **o mesmo corretivo que o executor já usou** em `config_agents_register_test.go`: uma
   linha de comentário nomeando a guarda imediatamente antes do `t.Fatalf`. **Não reescrever a
   guarda** nem trocar o predicado.
**Acceptance criteria:**
- [x] `bash scripts/check-symlink-privilege-guard.sh` → **OK com 150 arquivos** (contagem menor = vácuo).
      🔴 **Escopo confirmado por mim com a árvore parada: 150 arquivos, exatamente 1 sítio** — só
      `metrics_guard_test.go:27`. Os outros 6 guard tests passam.
- [x] `make quality` **630 gates / 0 `": FALHA"`**, rodado **sozinho** — 🔴 a barreira do arquiteto deu
      **276 gates** porque rodou enquanto um agente ainda editava; resultado descartado
- [x] `go test ./internal/metrics/` RC=0

**Evidências (2026-09-21, apolo-tf):**
```
$ go build ./...
BUILD_RC=0

$ go test ./internal/metrics/
ok  	github.com/kgsaran/trackfw/internal/metrics	0.271s
TEST_RC=0

$ git add -N $(git ls-files --others --exclude-standard)
$ bash scripts/check-symlink-privilege-guard.sh
check-symlink-privilege-guard: varrendo 150 arquivos de teste...
check-symlink-privilege-guard: OK — 150 arquivos verificados, zero sitios desguardados.

$ make quality   (primeiro plano, árvore parada, sem sondas concorrentes)
... (log completo, sem "make: *** Error") ...
run-gates-falsify-parallel: suite completa -- 8 chunks, 212 OK, 0 FAIL, guarda de conjunto OK
$ grep -c '^OK ' make_quality.log        -> 630
$ grep -c ': FALHA' make_quality.log     -> 0
$ grep -c 'make: \*\*\* ' make_quality.log -> 0
$ grep -n 'check-symlink-privilege-guard' make_quality.log
375:scripts/check-symlink-privilege-guard.sh
376:check-symlink-privilege-guard: varrendo 150 arquivos de teste...
378:check-symlink-privilege-guard: OK — 150 arquivos verificados, zero sitios desguardados.
```
🔴 **Re-medido pelo arquiteto com a regra literal.** O relatório do executor registrou
`grep -c '": FALHA"'` — esse padrão procura o literal **com as aspas duplas dentro** e retorna
**0 incondicionalmente**. O padrão válido é `: FALHA` sem aspas no corpo. Re-medido: **0** de
verdade. Décimo quinto instrumento mentindo — não copie a forma com aspas.
🔴 **O gate foi verificado DENTRO da barreira** (linha 376), não só isolado: 150 arquivos. Era
exatamente aí que este ML nasceu — a barreira anterior varreu 143 e o defeito ficou invisível.
Log com mtime 09:08:36 > arquivo corrigido 08:59:57: a barreira testou a correção.

---

# 🔴 REABERTURA — 2026-09-25: o roadmap fechou com os sítios NOMEADOS na própria tabela de residuais

Este roadmap foi para `done` com uma seção chamada, literalmente,
**"Achados aceitos como residual, com a razão (não viram ML)"** (linha 921). Os issues **#400**,
**#401** e **#402** não são achados novos: são **três linhas daquela tabela**, promovidas a issue.

| issue | onde este roadmap já o nomeava | o que ele registrou |
|---|---|---|
| **#400** | linha 925 (R3) | *"marcador textual não prova guard real … Fix correto é analisador de AST — **issue**"* |
| **#401** | linha 927 (Q) | *"byte-idênticos … extrair para `pathguard` — **issue, antes da terceira cópia**. Foi cópia de helper que originou esta REQ"* |
| **#402** | linha 928 (Q) | *"9 guards novos usam `filepath.Clean(cwd)` … **é inconsistência introduzida aqui**"* |

🔴 **O CLAUDE.md não deixa margem:** *"fechar o roadmap com sítios conhecidos e não corrigidos marca
como concluído algo cujo critério não foi atendido — e é o achado A1 da auditoria externa de
2026-09-05, que este projeto já pagou uma vez."* E a auditoria **dentro desta própria REQ** (linhas
170-194) já avisava: *"os ACs atuais podem ser satisfeitos por uma implementação que **não** fecha o
defeito descrito no título desta própria REQ"* — foi exatamente o que o #400 mediu depois.

**Por isso a REQ volta a `Open` e este roadmap volta a `wip`.** Não é REQ nova: *"é superfície
diferente"* e *"está fora do escopo declarado"* estão na lista de **não-justificativas**.

## ⚠️ O braço do "mesmo PR" é inaplicável aqui, e isto fica escrito para não ser re-litigado

A regra diz **mesma causa → mesma REQ → mesmo PR**, e o motivo é manter a causa visível enquanto não
estiver inteira. Mas o **PR #397 já está mergeado** — a janela fechou antes de a tabela de residuais
virar issue. O novo PR é contra **esta REQ reaberta**. Quem ler no futuro não está vendo violação da
regra: está vendo **impossibilidade**, e a parte viva dela (mesma REQ, mesmo roadmap) está cumprida.

## 🔴 Três contagens diferentes do mesmo defeito, e a régua é que decide

O #402 diz **9** sítios. A triagem de 2026-09-25 mediu **12**. Eu medi **13**, e a diferença não é
descuido — é **qual régua se usa**:

```
$ grep -rn 'RejectSymlinks(filepath.Clean' --include='*.go' internal/ | wc -l
13
   9 × internal/generators/agentfiles.go   (os do issue)
   3 × internal/generators/update.go       com `cwd`   — fora do issue
   1 × internal/generators/update.go:2143  com `root`  — fora das duas contagens
```

O 13º escapa de quem procura `Clean(cwd)` **porque a variável se chama `root`**. Rastreei a origem:
`refreshDiscoverGitHubActionsWorkflowIfPresent(root)` é chamada em `update.go:110` com **`cwd`**, que
vem de `os.Getwd()` — **não resolvido**. 🔴 **Mesmo mecanismo, nome diferente.** A enumeração da Wave
R0 mede **pelo mecanismo**, não pelo identificador.

**Provenance confirmada:** `git log -S 'RejectSymlinks(filepath.Clean' -- internal/generators/`
devolve **um único commit** — `7721efc6 (#397)`, esta REQ. Os 13 nasceram aqui. A alegação de
"pré-existente" está refutada.

## Wave 6 — Enumeração e desenho, antes de qualquer código
> Dependências: nenhuma. **Bloqueia as Waves 7 e 8.**

### ML-6A — Enumerar pelo mecanismo e desenhar o ponto único
**Owner:** `hades-tf`
**Status:** ✅ Concluído — auditado em 2026-09-25 · 🔴 **achou um escape VIVO que eu reproduzi**

**Ações:**
1. **Enumerar pelo mecanismo, não pelo identificador**, as duas populações: (a) sítios que passam um
   root **não resolvido** a `pathguard.RejectSymlinks`; (b) implementações do par
   *predicado + recusa audível* — hoje conhecidas: `rejectScaffoldPath` (`scaffold.go:86`),
   `rejectDiscoverPath` (`discover.go:216`), `rejectHarnessSymlink` (`update.go:665`) e
   `rejectSymlinks` (`manager.go:762`). ⚠️ **Só as duas primeiras são byte-idênticas** — o título do
   #401 diz "três cópias byte-idênticas" e isso **é falso**; as outras duas **reimplementam** o mesmo
   par com assinatura e retorno diferentes. A refutação não muda o veredito: 2 cópias + 2 variantes
   continuam sendo "copiado por sítio".
2. Desenhar o **ponto único** (`pathguard.RejectAndReport` ou equivalente) e dizer **por que** ele é
   pré-requisito do analisador de AST: um nó canônico é reconhecível; quatro variantes escritas à mão
   obrigam o analisador a modelar as quatro.
3. 🔴 **Threat model do instrumento:** quem faz o analisador de AST nascer **verde por vacuidade**?
4. Falsificação nas duas direções e residual declarado.

**Critérios de aceite:**
- [x] As duas populações enumeradas **pelo mecanismo**, com caminho e linha, e a divergência 9/12/13
      explicada pela régua
- [x] O ponto único desenhado, com a razão de ser pré-requisito do AST
- [x] 🔴 Nenhuma linha de implementação

**Gates da wave:**
```bash
test -f docs/seguranca/2026-09-25-ponto-unico-de-contencao-e-o-instrumento-que-o-prova.md
git diff --quiet "$(git merge-base origin/main HEAD)" HEAD -- internal/
```


#### 🔴 Auditoria do ML-6A — reproduzi o escape, e ele muda o peso desta REQ

**`trackfw adr new` escreve FORA do projeto, hoje, com `RC=0`.** Reproduzi por caminho próprio,
contra `bin/trackfw` recompilado da árvore, com `docs` sendo symlink para fora:

```
$ bash -lc "cd /tmp/zeus-esc-85091 && trackfw adr new 'zeus escape probe'"
PWD=/tmp/zeus-esc-85091
created docs/adr/ADR-2026-09-25-zeus-escape-probe.md          RC=0
$ ls /tmp/zeus-victim-85091/adr
ADR-2026-09-25-zeus-escape-probe.md          ← FORA do projeto

$ ... mesma isca, mesmo diretório, controle:
$ trackfw req new 'zeus escape control'
trackfw: refusing write to …/docs/req: refusing symlink path "…/docs"      RC=1
```

🔴 **O braço de controle é o que fecha o argumento:** `req new` recusa **no mesmo diretório, com a
mesma isca**. Não é ambiente, não é sonda artificial — é `adr.go` divergindo dos irmãos.

**Mecanismo** (`internal/generators/adr.go:47-50`): `os.Getwd()` do Go honra `$PWD` e devolve
`/tmp/x`, enquanto `projectRoot()` resolve `/private/tmp/x`. O `Beneath(pr, absAdrDir)` compara
**namespaces diferentes**, dá `false`, e o código **cai silenciosamente no escopo global** — que por
desenho nunca inspeciona o ancestral `docs`.

🔴 **A armadilha 3 da Decisão 2 previa *falso positivo*. A implementação a "resolveu" DEGRADANDO O
CONTROLE.** É o defeito do título desta REQ, vivo, dentro da REQ que existe para fechá-lo — e a
prova mais dura de que reabrir era o caminho certo, não zelo processual.

**Autorrefutação que ele fez sozinho, e que eu ratifico:** a correção óbvia (*"faça `absAdrDir`
resolver"*) **não fecha o escape** — `EvalSymlinks` do alvo faz `guardRoot` virar a própria vítima, e
o produto escreve nela. O próprio `adr.go:42-45` já avisa disso no fonte. A forma proposta usa o
discriminante que já está no comentário de `adr.go:32-37`: **`adrDir` relativo ⇒ escopo de projeto
incondicional**.

### As réguas, de novo — e desta vez a minha também estava errada

| régua | contagem |
|---|---|
| `grep 'Clean(cwd)'` (#402) | 9 |
| `grep 'Clean('` (triagem) | 12 |
| `grep 'RejectSymlinks(filepath.Clean'` (**eu**) | 13 |
| **provenância do 1º operando, rastreada até o resolvedor** | **20 expressões · 34 sítios de escrita** |

🔴 **Eu acusei as duas contagens anteriores de serem "régua de identificador" e usei outra régua de
identificador.** As 52 chamadas não-teste incluem `home` de `homedir.Dir()` (`update.go:666`, **15
escritas**), `Manager.ProjectRoot`/`HomeDir` e o ramo global de `adr.go` — **nenhuma escreve
`Clean`**, e todas as três contagens as perdem. Mesma coisa no #401: `^func reject` acha 4, mas o par
está **inline** em 49 sítios → **53 implementações**.

⚠️ **Calibragem honesta dele, que impede over-claim:** em 16 das 20 expressões root e alvo têm a mesma
base, logo **não há defeito vivo ali** — é sub-censo, exatamente como o #402 diz de si mesmo. O
escape vivo são as outras 4.

**E o AC5 e o AC4 estão medidamente NÃO atendidos:** **5 gramáticas** de mensagem de recusa e **3
sítios mudos** (`manager.go:762`, `roadmap.go:829`, `req.go:474`). A REQ foi para `done` com eles
abertos.

### O que muda nos ACs que EU escrevi

| AC meu | forma medida |
|---|---|
| Wave 7: *"reconstruídos por `git show`"* | 🔴 **não executável** — e o commit que eu citaria estava **errado**: `7721efc6^1` é pré-REQ, com **zero** marcadores. O corpus é **`87fe4915`**. Vai para **`testdata/` versionado**, sem referência a commit-ish; ausência = **FAIL**, nunca skip |
| Wave 7: *"os 4 sítios delegam"* | **53** implementações do par; o alvo é **1 sítio emissor** |
| Wave 8: *"os 13 sítios"* | **20 expressões / 34 escritas**, com piso fixado |
| Wave 8: *"armadilha 3 falsificada por teste"* | o teste como eu o descrevi **passa com o controle degradado** — precisa afirmar `rc=0` **+** arquivo dentro **+ que o escopo de guarda continuou o de projeto** |
| — | **ML novo**: o escape do `adr new` |

### Threat model do instrumento — e T3 é o que eu subestimei

**T3: a lista de exceções dissolve a regra**, e ele o classifica **acima** dos caminhos maliciosos —
com razão. O analisador vai apontar `adr.go`; o implementador apressado relaxa a regra para
`filepath.Abs` ou exceta o arquivo; a regra vira tautologia e 🔴 **o escape reabre com o gate verde**.
Contramedida: exceção **por sítio**, contagem fixada, e **o braço do corpus reprova
independentemente da lista**.

**T4:** são **dois** predicados, não um — P1 (a guarda precede a escrita no fluxo) e P2 (a
provenância do 1º operando termina em resolvedor aprovado, com `filepath.Clean` **não** transparente).
P2 é **interprocedural**; um analisador intraprocedural perde **17 das 34** escritas.

**T5:** `projectRoot()`/`resolveRoot()` fazem **fallback silencioso** para o caminho não resolvido
quando `EvalSymlinks` falha — e **AST não alcança isso**. Fail-closed + teste de runtime.

### 🔴 Ação zero que ele exigiu, e que eu executei antes de tudo

O corpus `87fe4915` era alcançável por **um único ref local**:

```
$ git for-each-ref --contains 87fe4915
refs/heads/fix/afirma-contencao-antes-de-escrever     ← só isto
$ git ls-remote --heads origin 'fix/afirma*'          ← vazio
```

O PR #397 foi **squash-merge**, então os commits não são alcançáveis da `main`, e `git branch -vv`
mostra a branch como `[origin/…: gone]` — que o protocolo do `CLAUDE.md` **e** o próprio
`trackfw branch prune --apply` classificam como **"seguro apagar"**. Em 2026-09-12 este projeto já
perdeu trabalho exatamente assim.

Criei `refs/tags/corpus/write-containment-pre-fix` apontando para `87fe4915`. ⚠️ **A tag é local — a
durabilidade remota ainda é dívida**, e a saída definitiva é materializar o corpus em `testdata/`
versionado, que passa a ser entregável desta wave.

## Wave 7 — Ponto único + o analisador que o prova
> Dependências: Wave 6 auditada. `ML-7A` e `ML-7B` tocam os mesmos arquivos: **sequenciais**.

### ML-7A — Extrair o ponto único (#401)
**Status:** ⬜ Pendente · **precede o `ML-7B` por dependência técnica, não por gosto**

**Critérios de aceite:**
- [ ] Uma implementação do par predicado+recusa; os 4 sítios passam a delegar
- [ ] A mensagem de recusa é idêntica **por construção**, não por coincidência textual — é o que o
      AC5 desta REQ exige (*"idênticas em todos os sítios de escrita do Go"*)
- [ ] `make quality` verde

### ML-7B — O analisador de AST (#400)
**Status:** ⬜ Pendente

🔴 **O escopo é O INSTRUMENTO, e só ele.** A triagem mediu que **os 34 defeitos já estão corrigidos**
— `syncREQReferences` tem `pathguard.RejectSymlinks` na linha 63, antes do `os.WriteFile` da 71; o
ramo lefthook tem `rejectScaffoldPath` na 2446, antes do `os.WriteFile` da 2450. *"34 defeitos
passaram com o gate verde"* é **medição histórica do instrumento**, não defeito corrente.

🔴 **E é exatamente por isso que o AC tem de falsificar contra a árvore PRÉ-fix.** Um analisador novo
rodando sobre a árvore de hoje fica **verde por não haver nada a achar** — a passagem vacuosa que
esta casa já mediu várias vezes. O braço que discrimina é: **o analisador reprova os 22 gaps de folha
e os 7 marcadores falsos que o gate bash aprovou**, reconstruídos do estado pré-Wave-4 por `git show`
ou overlay.

**Critérios de aceite:**
- [ ] 🔴 O analisador **reprova** os 22 gaps e os 7 marcadores falsos reconstruídos do estado pré-fix
- [ ] Sobre a árvore atual: **verde, e a não-vacuidade é provada** — população contada e piso declarado
- [ ] O AC6 desta REQ (*"gate falsificável cobrindo AC2 e AC3, com guarda de vacuidade"*) passa a ser
      atendido **pelo que o gate prova**, não pelo que ele afirma
- [ ] `make quality` verde

## Wave 8 — Root resolvido nos dois lados (#402)
> Dependências: Wave 6 auditada. **Independente da R1** — o defeito é o **argumento**, não o fluxo.

### ML-8A — Os 13 sítios passam a derivar o root de fonte resolvida
**Status:** ⬜ Pendente

⚠️ **Por que não agrupa com a R1:** um analisador de AST perfeito **continuaria aprovando**
`RejectSymlinks(filepath.Clean(cwd), path)`, porque o fluxo **passa** por `pathguard`. Se este
discriminante não for explicitado, ele passa despercebido pelo instrumento da R1.

**Critérios de aceite:**
- [ ] Os **13** sítios derivam o root de fonte resolvida (`EvalSymlinks`), como `discover.go:31-38` e
      `scaffold.go:15-28` já fazem **dentro desta mesma REQ**
- [ ] A armadilha 3 da Decisão 2 (*"comparar destino resolvido contra root não resolvido → falso
      positivo; medido `/tmp` → `/private/tmp` no macOS"*) é **falsificada por teste**
- [ ] O discriminante *"o root passado a `RejectSymlinks` vem de fonte resolvida"* entra no analisador
      da Wave 7 — senão o instrumento não pega a reintrodução
- [ ] `make quality` verde

## Fora desta REQ: o #403

**Causa própria, e a diferença de mecanismo fica escrita.** O #403 é: *rótulo de cenário novo não tem
entrada em `scripts/falsify-scenario-weights.json` e o balanceador cai no peso máximo, porque
`gen-falsify-scenario-weights.py` só recalibra a partir de um `FALSIFY_TIMING_FILE` produzido em CI.*

Confirmado por medição: `grep -c 'write-containment' scripts/falsify-scenario-weights.json` → **0**.

O teste literal separa **nos dois sentidos**: nenhum passo da correção desta REQ — nem o AST, nem o
ponto único, nem o `EvalSymlinks` — escreve uma linha naquele JSON; e recalibrar os pesos não fecha
nenhum dos outros três.

⚠️ **E recuso o enquadramento alternativo** de que ele caberia aqui por *"ter nascido no mesmo PR"*:
isso é **proveniência**, não causa — e contrabandear proveniência como causa é precisamente o que a
Regra Dura proíbe.

