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
> Dependências: Wave 0 completa. **O particionamento em MLs só é decidível depois da enumeração** —
> escrever os MLs agora seria escopo inventado. Preenchido ao fechar o ML-0A.

## Wave 2 — Gate falsificável
> Dependências: Wave 1 completa. `artemis-tf`. Detalhado após a Wave 1.

## Barreira final

Revisão `hefesto-tf` e `hades-tf`, auditoria do arquiteto, `barrier`. **CI verde**, não só verde
local — `vault/notes/ambiente-do-dev-e-mais-rico-que-o-do-ci-2026-08-29.md`.
