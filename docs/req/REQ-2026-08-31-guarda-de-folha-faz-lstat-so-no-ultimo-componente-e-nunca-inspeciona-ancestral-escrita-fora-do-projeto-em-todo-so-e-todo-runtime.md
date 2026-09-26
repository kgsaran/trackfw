---
status: Done
date: 2026-08-31
author: "zeus-tf"
adr: ""
roadmap: "docs/roadmaps/done/ROADMAP-2026-08-31-guarda-de-folha-resolve-o-caminho-e-afirma-contencao-antes-de-escrever.md"
---

# REQ: Guarda de folha faz `Lstat` só no último componente e nunca inspeciona ancestral — escrita fora do projeto em todo SO e todo runtime

> Date: 2026-08-31 | Status: Done

## Motivation

As guardas de link do projeto verificam **apenas a folha** do caminho:

```go
info, err := os.Lstat(filepath.Join(root, DiscoverGitHubActionsWorkflowPath))
if info.Mode()&os.ModeSymlink != 0 { /* recusa */ }
os.WriteFile(path, ...)
```

**`Lstat` só deixa de seguir o *último* componente do caminho. Ancestrais são sempre seguidos.** Logo
um symlink num diretório ancestral (`.github/`, `.github/workflows/`) redireciona a escrita para fora
do projeto **sem que a guarda olhe**. A folha não é symlink; a checagem passa; a escrita sai da
árvore.

### Por que isto é a descoberta que importa

Este defeito veio a reboque da investigação de junction no Windows
(`REQ-2026-08-30-sonda-nao-responde-a-pergunta-7-...`), mas **não tem nada de Windows**:

- Vale em **Linux, macOS e Windows**.
- Vale nos **três runtimes** — aqui a paridade se aplica normalmente, ao contrário da detecção de
  junction, onde os três divergem.
- Não depende de junction, de `ModeIrregular` nem de privilégio: **um symlink de diretório comum
  basta**, e criar symlink de diretório não exige privilégio em Linux/macOS.

A medição de junction (run `33447191373`) atenuou as outras duas classes — o `rmdir` remove a
junction e não o alvo, sem destruição de dados, e o Node já enxerga junction. **Esta classe não foi
atenuada por nada.**

### Precondição, declarada honestamente

Exige quem consiga plantar um symlink na árvore do projeto — mesma precondição das outras classes.
Não é escalonamento remoto. O que a distingue não é ser *mais grave*, é ser **universal**: mesma
forma, três runtimes, três sistemas operacionais, sem mitigação acidental.

### A enumeração conhecida está errada, e sei disso

As três guardas que eu havia nomeado (`internal/generators/update.go:1869`, `:1894`,
`internal/discover/discover.go:268`) saíram de um `grep` por `ModeSymlink`/`Lstat`. **Esse grep é
cego para todo ponto que escreve sem checar link nenhum** — que é exatamente a população em risco.
Contagem bruta dos primitivos de escrita:

| runtime | ocorrências |
|---|---|
| Go (`os.WriteFile`/`os.Create`) | 85 |
| Node (`writeFileSync`) | 78 |
| Python (`write_text` / `open(...,'w')`) | 65 |

**228 no total** — limite superior, não a lista. A maioria não escreve em caminho derivado de `root`
controlável por terceiro. **Descobrir quais escrevem é a Wave 0, não premissa desta REQ.** A Wave 0
anterior já provou o ponto: `hades-tf` encontrou duas superfícies fora da minha lista
(`copyPath`/`_copy_path` e o código morto `writeCIWorkflowForce`).

### Forma do remédio — nomeada aqui de propósito

*"Inspecionar ancestrais"* admite pelo menos três formas incompatíveis: `Lstat` de cada componente até
o `root`; resolver com `filepath.EvalSymlinks`/`fs.realpathSync`/`Path.resolve()` e **afirmar que o
resultado continua sob o `root`**; ou descida incremental estilo `openat`. Sem escolher uma, três
especialistas inventam três.

**A forma escolhida é resolver-e-afirmar-contenção**, porque **compõe com o que já existe**: o
`cleanEmpty` (`manager.js:420`) e o `_remove_empty` (`manager.py:589`) já fazem contenção geográfica
(`path.isAbsolute(rel)`, `root in directory.parents`). Adotar a mesma forma evita uma segunda
checagem conflitante com a primeira.

## Decisões tomadas após a Wave 0 (2026-08-31)

### Decisão 1 — quebra de comportamento: **recusa audível, sem opt-out**

A Wave 0 mostrou que um diretório compartilhado **fora** da árvore do projeto (padrão real: um
`.github`/templates central apontado por vários repositórios) é **indistinguível do ataque** e será
recusado. KG decidiu: **recusa audível nomeando o caminho resolvido**, sem opt-out.

Razão: o opt-out é escopo novo, e um opt-out mal desenhado vira a própria porta que a guarda existe
para fechar. Se aparecer usuário real com esse padrão legítimo, a recusa audível dá o diagnóstico
imediato e a REQ de opt-out nasce com caso de uso concreto em vez de hipótese.

### Decisão 2 — grafia do remédio, corrigida pela Wave 0

**A forma segue sendo resolver-e-afirmar-contenção. A grafia que eu escrevi estava errada em três
pontos**, todos medidos:

| # | armadilha | correto |
|---|---|---|
| 1 | **`path.resolve()` do Node é léxico — nunca segue symlink.** Usá-lo faria a guarda virar **no-op que passa em todo teste escrito contra o comportamento do Go**: verde, paritária e inútil | **`fs.realpathSync`** |
| 2 | `EvalSymlinks`/`realpathSync` **falham com folha inexistente**, e o caso dominante é *criar* arquivo | resolver o **diretório pai** e concatenar a folha. `Path.resolve(strict=False)` (Python) tolera nativamente |
| 3 | comparar destino resolvido contra `root` **não** resolvido → falso positivo (medido: `/tmp` → `/private/tmp` no macOS) | **resolver os dois lados** antes de comparar |

A armadilha 1 é a mais perigosa das três porque **não falha** — produz uma guarda que passa nos
gates. Qualquer ML da Wave 1 tem de nomear a primitiva por runtime, nunca "resolver o caminho".

### Escopo real, revisado

A enumeração original (3 guardas) estava subcontada em uma ordem de grandeza: **187 pontos que não
checam link algum**, em 4 famílias. A família de maior severidade é `trackfw update harness`
(**escopo global**, escreve no `$HOME`), com PoC executada. O particionamento da Wave 1 sai daí.

## Acceptance Criteria

- [x] **AC1** — 🔴 **Wave 0 enumera de verdade.** A lista de pontos que escrevem em caminho derivado
      de `root` **sem** inspeção de ancestral, no **único runtime (Go)**, obtida varrendo os **primitivos de escrita** — não `ModeSymlink`. A lista de 3 guardas é ponto de partida conhecido-incompleto.
      → **Wave 6** (`ML-6A`, `hades-tf`). E a enumeração do fechamento original estava **incompleta pela régua**: 4 identificadores `reject*` contra **53** implementações do par inline; 9/12/13 sítios por grep contra **16** por provenância no AST.
- [x] **AC2** — Escrita através de **ancestral** symlink é recusada, com a forma
      resolver-e-afirmar-contenção.
      → `ML-7A`. 🔴 **E a Wave 6 achou um escape VIVO que o fechamento anterior não tinha visto:** `trackfw adr new` escrevia **fora do projeto** com `RC=0`, porque `Beneath` comparava namespaces diferentes e o código **caía no escopo global** em silêncio.
- [x] **AC3** — 🔴 **Falsificação nas duas direções.** (a) com ancestral symlink apontando para fora,
      a escrita é recusada e **nada** é criado fora da árvore; (b) **controle**: operação legítima,
      sem link algum, **continua funcionando** — a guarda não pode super-disparar. Sem (b), trocamos
      um buraco por uma quebra.
      → braço (a) e braço (b) **em cada ML**, e o controle foi o que impediu três conserto-vazios: `req new` recusando no mesmo diretório (`ML-7A`), o caminho feliz ainda escrevendo (`ML-9A`), e o mutante da forma correta exigindo **zero** achado (`ML-7C`).
- [x] **AC4** — Recusa **audível**: mensagem em stderr nomeando o caminho e o motivo. Silêncio vira
      *"o update não atualizou meu arquivo e não disse nada"*.
      → `ML-7B`. **Estava aberto e medido:** **7** gramáticas distintas e **4** sítios que recusavam **mudos** — o quarto dentro do próprio `pathguard`, invisível ao censo porque a régua **isentava o pacote antes de medir**.
- [x] **AC5** — ~~Paridade nos 3 CLIs~~ **SEM OBJETO desde a v8.0.0** (commit `2eae0a44`): há uma
      implementação única em Go. Medido em 2026-09-18: `npm/src` não existe, `git ls-files pypi/trackfw`
      → vazio. Substituído por: a recusa e a mensagem são **idênticas em todos os sítios de escrita**
      do Go — a consistência que importa agora é **entre sítios**, não entre runtimes.
      → `ML-7B`: **53 implementações → 1 emissor**, com a identidade **por construção** (stderr e erro saem da mesma const), não por coincidência textual.
- [x] **AC6** — Gate falsificável cobrindo AC2 e AC3, com guarda de vacuidade.
      → `ML-7C`. 🔴 **Este era o AC satisfeito por um instrumento que não provava nada** — o gate bash ficou **verde por cima de 22 instâncias vivas** do defeito. Substituído por analisador de AST com **corpus pré-fix versionado**: 104 achados no corpus, 15 na árvore (todos fixados), e os dois casos que o #400 nomeia reprovando **pelo nome**.
- [x] **AC7** — Reproduzível **localmente** em macOS/Linux, sem depender de runner Windows nem da
      sonda. É o que torna esta REQ mais rápida que a de junction.
      → reproduzido localmente em macOS o tempo todo, inclusive o checkout do Windows (`git -c core.autocrlf=true checkout-index`) para o `ML-9B`.
- [x] **AC8** — `make quality` verde e **CI verde**. Verde local não é conclusão —
      ver `vault/notes/ambiente-do-dev-e-mais-rico-que-o-do-ci-2026-08-29.md`.

      → `make quality` rc 0 e **PR #441 com 21 checks, 21 `pass`**, incluindo `windows-full-suites`. ⚠️ **E o AC8 provou seu próprio ponto:** o verde local **não** era conclusão — o Windows reprovou 4 testes que passavam aqui.
- [x] **AC9** — 🔴 **Absorve a `REQ-2026-08-30-roadmap-move-segue-symlink-de-arquivo-md-e-altera-arquivo-fora-do-projeto`.**
      Mesma causa pelo teste da Regra Dura (*"se eu corrigir esta causa, exatamente estas falhas fecham"*):
      **o produto resolve um caminho e escreve sem afirmar que o destino está contido na árvore.**
      As duas são faces do mesmo defeito — lá o symlink é a **folha** (`roadmap move` o segue), aqui é um
      **ancestral** (`Lstat` não o inspeciona). Reproduzido pelo arquiteto em 2026-09-18:
      `ln -s /tmp/vitima.md docs/roadmaps/backlog/ROADMAP-isca.md && trackfw roadmap move ROADMAP-isca wip`
      alterou o `status:` da vítima **fora do projeto**.
      Falsificação exigida para este AC, além das do AC3: o mesmo comando passa a **recusar**, e
      `roadmap move` legítimo **continua funcionando**.
      → mantido; a família da folha já fechara na Wave 4, e a reabertura tratou o **ancestral** e o **argumento**.

## Negative Scope — o que esta REQ NÃO faz

- **Não trata detecção de junction.** Classes 1 e 2, `ModeIrregular`, troca de primitiva no Python:
  tudo isso é a REQ irmã de **junction**, que permanece separada (causa diferente: detecção de forma
  no Windows, não contenção de destino) e só
  se verifica em runner Windows. Misturar faria esta REQ herdar a verificação pós-merge de lá.
- **Não altera o Node na detecção de link.** Medido: o Node já enxerga junction.
- **Não adota `ModeSymlink|ModeIrregular`** em lugar nenhum.
- **Não mexe** em `cleanEmpty`/`_remove_empty`/`removeEmptyAncestors` — são Classe 2.

## Linked ADR

ADR: `docs/adr/ADR-2026-09-18-todo-sitio-que-escreve-em-caminho-derivado-de-root-resolve-e-afirma-contencao-antes-de-escrever.md`

<!-- Retificação de 2026-09-18: a versão original declarava "nenhum ADR — correção de bug com
tratamento idêntico nos 3 runtimes". O argumento morreu com a v8 (implementação única), e a
conclusão estava errada de todo modo: há decisão arquitetural a registrar — o predicado é
resolver-e-afirmar-contenção, não detectar symlink, e vive num ponto único em vez de copiado por
sítio. -->

## Linked Roadmap

Roadmap: `docs/roadmaps/done/ROADMAP-2026-08-31-guarda-de-folha-resolve-o-caminho-e-afirma-contencao-antes-de-escrever.md`


---

## 🔴 Achado da auditoria externa (2026-09-05) — a solução aceita é insuficiente

Registro: `docs/portabilidade/2026-09-05-auditoria-externa-astra-achados-e-verificacao.md`, item 12.

A escolha de **resolver caminhos e verificar contenção** está certa. O problema é o alcance:

**Resolver somente o pai imediato não cobre criação com vários ancestrais ainda inexistentes.** Num
`mkdir -p` de três níveis, o pai imediato não existe — e a verificação passa a inspecionar um caminho
que ainda não está lá, enquanto o **ancestral existente mais próximo** (que é quem de fato determina
onde a escrita vai cair) nunca é olhado.

**Duas superfícies que a REQ não menciona:**

1. 🔴 **O ancestral existente mais próximo** é o alvo correto da verificação de contenção, não o pai
   imediato.
2. 🔴 **Junctions do Windows** — o NTFS tem redirecionamento que não é symlink, e um `lstat` que só
   entende symlink passa por cima dele. Uma junction num ancestral leva a escrita para fora do
   projeto **sem** que nenhum componente seja um symlink.

E falta declarar o **controle de operação legítima**: a guarda precisa provar que **não** bloqueia
criação normal de diretório dentro do projeto. Guarda de escrita que aperta demais vira guarda
desligada na semana seguinte.

**Consequência:** os ACs atuais podem ser satisfeitos por uma implementação que **não** fecha o
defeito descrito no título desta própria REQ. Precisam de revisão antes do roadmap.

## 🔴 Fechamento pós-merge (PR #441, 2026-09-26) — esta REQ foi REABERTA, e a reabertura era necessária

Esta REQ já esteve `Done` uma vez. Foi reaberta em 2026-09-25 porque o roadmap tinha ido para `done`
com uma seção chamada, **literalmente**, *"Achados aceitos como residual, com a razão (não viram
ML)"* — e as issues **#400**, **#401** e **#402** eram **três linhas daquela tabela**.

🔴 **O que a reabertura encontrou justifica a regra que a obrigou.** A Wave 6 mediu um **escape vivo
que ninguém tinha reportado**: `trackfw adr new` escrevia **fora do projeto**, com `RC=0`, enquanto
`req new` recusava no mesmo diretório com a mesma isca. Era o defeito do **título desta REQ**, vivo,
**dentro da REQ que existe para fechá-lo** — e ele não estava em issue nenhuma.

### O que mudou, medido

| | fechamento anterior | agora |
|---|---|---|
| implementações do par predicado+recusa | **53** | **1** |
| gramáticas de mensagem de recusa | **7** | **1** |
| sítios que recusam **mudos** | **4** | **0** |
| sítios **fail-open** (guarda no `if`, escrita fora) | **5** | **0** |
| guard roots não resolvidos (`filepath.Clean`) | **16** | **0** |
| escritas sob outra grafia de root | **16** | **3** (nomeadas) |
| instrumento | marcador textual | **analisador de AST + corpus versionado** |

### As cinco vezes em que a régua decidiu, e o arquiteto errou junto

1. **Sítios de root não resolvido:** 9 (issue) · 12 (triagem) · **13 (arquiteto, por grep)** · **16
   (AST, por provenância)**. Acusei as duas primeiras de serem régua de identificador **e usei outra
   régua de identificador**.
2. **Implementações do par:** `^func reject` acha 4; o par está **inline** em 49 → **53**.
3. **Sítios mudos:** eram **4**, não 3 — o quarto dentro do `pathguard`, invisível porque a régua
   **isentava o pacote antes de medir**. *Isenção de censo diz onde a correção mora, não onde o
   defeito pode estar.*
4. **Gramáticas:** 7, não 5. E medir só `internal/` era estreito — o AC diz *"no binário"*, e `cmd/`
   conta.
5. 🔴 **O exemplar de "forma correta" que EU mandei copiar não satisfazia o próprio analisador.**
   `commands/discover.go` era um dos 22: o idioma de duas etapas é correto em runtime e **invisível
   ao instrumento**.

### Três coisas que o instrumento impediu, e que teriam passado

- A **correção óbvia** do escape (`EvalSymlinks` no alvo) não só o mantinha como **abria a arm que
  recusava** — e só é visível com a fixture certa: com o diretório da vítima ausente, o mutante
  **degenera no pré-fix** e o teste passa.
- A forma que prescrevi para os sítios `void` (`_ = RefuseUnverifiableRoot(…)`) foi **reprovada pelo
  analisador** com `guard-not-acted-on`. Afrouxar a regra faria uma chamada nua **dominar** a escrita
  tendo apenas *imprimido* uma mensagem.
- O executor **tentou corrigir dois sítios e reverteu com a medição** (7 e 15 testes), deixando a
  razão **no código**: em `update.go` o `home` é gravado **verbatim no conteúdo** de
  `.claude/settings.json`, logo resolver muda o **artefato gerado** — decisão de produto, não refactor.

### Residual declarado, e por que não é dívida escondida

- **T5** — `projectRoot()`/`resolveRoot()` caem para o caminho não resolvido quando `EvalSymlinks`
  falha. **Dinâmico**; a provenância é **estática**. 🔴 Nada aqui pode ser lido como cobertura dele, e
  virou dependência de ~19 ramos `RefuseUnverifiableRoot`.
- **3 roots irredutíveis** e **3 `root-aliased`**, todos **nomeados com a medição** que diz por que
  não podem mover.
- **15 pontos cegos** do analisador, fixados por sítio — entrada obsoleta vira **erro**.
- **`Render` não normaliza CRLF no caminho `subagent`** — medido no `ML-9B`, mecanismo **de produto**,
  candidato a REQ própria.
- **#403** fica fora, com a diferença de mecanismo escrita: nenhum passo desta correção escreve em
  `falsify-scenario-weights.json`.

⚠️ **E um achado de processo que é meu:** dois gates (`check-symlink-privilege-guard`,
`check-write-containment`) enumeram por `git ls-files`, logo **arquivo não commitado é invisível a
eles**. Minha barreira roda **antes** do commit — três vezes nesta campanha um teste passou na
barreira e reprovou depois. **A segunda passada pós-commit virou parte do meu protocolo.**
