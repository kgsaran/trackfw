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

- [ ] Enumeração real, pelos primitivos de **escrita** (228 candidatos brutos), não por `ModeSymlink`
- [ ] Escrita através de ancestral symlink recusada nos 3 CLIs, forma resolver-e-afirmar-contenção
- [ ] Falsificação nas duas direções, **incluindo o controle** de que operação legítima segue funcionando
- [ ] Recusa audível em stderr
- [ ] Paridade exata nos 3 CLIs — aqui a paridade vale normalmente
- [ ] `make quality` e **CI** verdes

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
- [ ] Tabela de enumeração revalidada, com o comando que a produziu e a classificação (a)/(b)/(c)
- [ ] 🔴 **Não-regressão do `manager`:** os testes de `internal/integrations` continuam verdes **sem
      alteração de fixture**; se algum precisar mudar, **pare e relate** — seria mudança de semântica
- [ ] Pacote folha sem importar `commands`/`validator`
- [ ] `go build ./...` RC=0 · `go test ./...` RC=0
- [ ] Uma frase por teste novo (AC11)

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
- [ ] 🔴 **Braço (a):** com ancestral symlink, a escrita é recusada e **nada** é criado fora
- [ ] 🔴 **Braço (b) — inegociável:** `update harness` legítimo, sem link algum, **continua
      funcionando** em todos os alvos. Sem este braço, trocamos um buraco por uma quebra
- [ ] A PoC da Wave 0 (`SKILL.md` fora do `$HOME`) passa a **falhar**
- [ ] `go build ./...` RC=0 · `go test ./...` RC=0 · `make quality` **630 gates / 0 FAIL**
- [ ] Uma frase por teste novo (AC11)

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
- [ ] `bash scripts/check-symlink-privilege-guard.sh` → OK, com a contagem de arquivos varridos (não vácuo)
- [ ] `go test ./internal/pathguard/` RC=0
- [ ] 🔴 `make quality` **630 gates / 0 FAIL**, medido com `/usr/bin/grep -c "^OK "` **e** com
      `/usr/bin/grep -ciE "^FAIL|FALHA"` → **0**. Ver nota de instrumento abaixo.
- [ ] Uma frase por teste alterado (AC11)

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
- [ ] 🔴 **PoC 1 fecha:** `.github` e `scripts` como symlink → `discover --init` **recusa**, e **nada**
      é criado fora. Demonstre antes/depois.
- [ ] 🔴 **PoC 2 (AC9) fecha:** `ln -s /fora/vitima.md docs/roadmaps/backlog/ROADMAP-isca.md &&
      roadmap move ROADMAP-isca wip` → **recusa**, e a vítima **não é alterada**.
- [ ] 🔴 **Braço (b), inegociável:** `discover --init`, `req new`, `roadmap new`, `adr new`,
      `note new` e `roadmap move` **legítimos continuam funcionando**. Execução real, não só unitário.
- [ ] `make quality` **630 gates / 0 FAIL** — métrica de falha: `/usr/bin/grep -cE ": FALHA"` → **0**
- [ ] Uma frase por teste novo (AC11)

### ML-1D — diversos e wrappers
**Status:** ⬜ Pendente (após o ML-1C)
`configure.go`, `java.go`, `config_agents_register.go`, `metrics.go`, `sync.go`, `validator.go`
(**caminho relativo puro** — exigem resolver `root` absoluto antes, precondição do ML-1A), e os
**wrappers** `manifest.go:81` / `render.go:722`, que chamam `atomicWrite` sem contenção e **não
aparecem** no grep de primitivos.

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
> Dependências: Wave 1 completa. `artemis-tf`. Detalhado após a Wave 1.

## Barreira final

Revisão `hefesto-tf` e `hades-tf`, auditoria do arquiteto, `barrier`. **CI verde**, não só verde
local — `vault/notes/ambiente-do-dev-e-mais-rico-que-o-do-ci-2026-08-29.md`.
