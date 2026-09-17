---
status: Open
date: 2026-09-17
author: ""
adr: ""
roadmap: "docs/roadmaps/done/ROADMAP-2026-09-17-gerador-aplica-o-template-de-consumidor-ao-proprio-produtor-e-o-doctor-prescreve-desfazer-a-correcao.md"
---

# REQ: gerador aplica o template de consumidor ao proprio produtor e o doctor prescreve desfazer a correcao

> Date: 2026-09-17 | Status: Open
| Linear Issue: 
| Jira Issue: 

## Motivation

Issue **#376**. Medido rodando `trackfw update harness` com o binário v8 **dentro do repositório do
próprio trackfw**, e confirmado por leitura do gerador.

### O que o comando escreve

`internal/generators/scaffold_doctor.go:53` — `BuildDiscoverGitHubActionsWorkflowContent()` devolve,
**incondicionalmente**:

```yaml
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.22"
      - run: go install github.com/kgsaran/trackfw/cmd/trackfw@v<versão>
      - run: trackfw validate
```

### 🔴 Por que isso inverte o sentido do check neste repositório

Para um **consumidor**, `go install …@vX` é o certo: ele valida a governança dele com o trackfw
publicado.

Aqui é o oposto. O `governance-go-install` é **required status check** e existe para dizer se **o PR**
mantém a governança consistente. Com a forma gerada, ele valida com o binário **já publicado** — e
**um PR que quebrasse o `trackfw validate` passaria**, porque o código do PR nunca é executado. O
verificador deixa de ver a mudança que deveria verificar.

### 🔴 O agravante: o `doctor` prescreve desfazer a correção

Três consumidores compartilham o mesmo builder:

| Sítio | Papel |
|---|---|
| `internal/discover/discover.go:276` | escreve o workflow |
| `internal/generators/update.go:1976` | reescreve no `update` — sem condição além de "existe e não é symlink" |
| `internal/generators/scaffold_doctor.go:269` | **compara o disco contra o builder** |

Como a `main` diverge **de propósito** (compila do fonte), o terceiro produz falso-positivo
**permanente**. Medido num worktree limpo da `main`:

```
[scaffold-divergent] .github/workflows/trackfw-validate.yml
  remedy: trackfw update   # resync ... content differs from the template trackfw v8.0.0 generates
```

**A ferramenta imprime, como remediação, o comando que reintroduz o defeito.** Não é só "o próximo
update reverte": é **pressão contínua e automatizada** contra um comentário no arquivo que pede o
contrário. Quem seguir o conselho da própria ferramenta quebra o required check.

### Efeitos secundários no mesmo template

- `go-version: "1.22"` fixo, enquanto o `go.mod` deste repositório declara `go 1.25.2`. O pin sai da
  fonte de verdade e passa a viver no template — **e para o consumidor também é frágil**: instalar um
  binário construído com toolchain mais nova sob Go 1.22 depende do download automático de toolchain.
- `actions/checkout` e `setup-go` **regrediram** de `v7`/`v7` para `v4`/`v5`. O
  `check-ci-workflow-pin-parity.sh` **não reprova**: ele verifica que **existe** pin, não que o pin
  não retrocedeu.

### O que salvou desta vez, e por acaso

Os **nomes dos jobs** foram preservados. Se o gerador tivesse renomeado um deles, todo PR ficaria
pendente para sempre — a falha da nota de vault
`matriz-em-job-required-por-nome-fica-pendente-para-sempre-2026-09-08.md`. E quem pegou o dano fui eu
lendo o diff; nenhum gate acusou.

## Acceptance Criteria

- [ ] **AC1** — 🔴 O gerador **distingue produtor de consumidor**. O sinal existe e é barato: o
      `go.mod` do repositório declara `module github.com/kgsaran/trackfw`. Quando o módulo **é** o
      trackfw, o workflow gerado compila do fonte (`go build -o ... ./cmd/trackfw`); caso contrário,
      instala do release. A decisão fica **no gerador**, não numa lista de exceções.
- [ ] **AC2** — 🔴 O `doctor`/`discover` compara o disco contra o template **do contexto correto**.
      Sem isto o AC1 sozinho não resolve: o falso-positivo permanece e a pressão para reverter
      continua. Os três sítios usam o mesmo builder — a correção tem de valer para os três.
- [ ] **AC3** — A versão do Go no template deixa de ser literal desalinhado. Para o produtor,
      `go-version-file: go.mod`. Para o consumidor, declare a escolha e o porquê — o pin atual
      (`1.22`) é menor que o `go` do `go.mod` deste projeto.
- [ ] **AC4** — As versões das actions no template deixam de regredir. E o
      `check-ci-workflow-pin-parity.sh` passa a detectar **retrocesso**, não só ausência de pin —
      hoje ele é verde diante de uma regressão `v7` → `v4`.
- [ ] **AC5** — 🔴 **Reescrito na auditoria do ML-1A (2026-09-17). A redação anterior nomeava o
      mecanismo, não a classe, e foi satisfeita ao pé da letra com a classe aberta.**
      Redação anterior: *"nenhum workflow deste repositório obtém o binário do trackfw por
      `go install …@v<versão>`"*. Um segundo sítio obtém o binário por **`install.sh` do release**, o
      que não é `go install` e passava.
      Redação vigente: **nenhum workflow deste repositório valida com um binário do trackfw que não
      seja compilado do código do próprio PR** — qualquer que seja o mecanismo de obtenção
      (`go install …@v`, `curl …/releases/latest/download/install.sh | sh`, download de artefato,
      imagem pré-construída). O que define a classe é a **procedência do binário**, não o comando.
      **Gate exigido:** varredura dos workflows **commitados** em `.github/workflows/*.yml` deste
      repositório. O `check-ci-workflow-pin-parity.sh` verifica o **template** que o builder emite,
      não o arquivo em disco — e foi por isso que o sítio sobreviveu.
- [ ] **AC6** — Falsificação em duas direções: num fixture cujo `go.mod` é o do trackfw, o gerado
      compila do fonte; num fixture de consumidor, instala do release. 🔴 **Com contra-braço:** a
      versão sem a correção produz `go install` **nos dois** — provando que o cenário discrimina.
- [ ] **AC7** — Depois da correção, `trackfw doctor` na raiz deste repositório **não** reporta
      `scaffold-divergent` para `.github/workflows/trackfw-validate.yml`. É a medição que fecha o
      issue; qualquer outra é indireta.
- [ ] **AC8** — 🔴 **Bloqueante, descoberto na Wave 0 e verificado no script.**
      `scripts/check-ci-workflow-pin-parity.sh` **reprova a correção do AC1**: a função
      `check_discover_pin` exige literalmente `@v<versão>` no template do `discover` e recusa qualquer
      variante sem ele. Um template de produtor que compila do fonte **não contém** essa string, logo
      o gate falha — e o gate é obrigatório no `quality`.
      **O gate tem de passar a distinguir os dois contextos, na mesma entrega do AC1**: exigir
      `@v<versão>` no template **de consumidor** (que é onde a pinagem importa, e é o que a
      REQ-2026-08-28 AC8 protege) e exigir a **ausência** de `go install` no template **de produtor**.
      Sem isto, o AC1 e o AC5 são mutuamente inconsistentes: o AC5 proíbe `go install @v` nos
      workflows deste repositório e o gate atual o exige.
      ⚠️ Nem o AC1 nem o AC5 podem ser declarados atendidos com este gate intocado.

## Negative Scope

- ❌ **Não** alterar `.github/required-status-checks.txt` nem branch protection. O conjunto R está em
  `D = R = W = 8` e foi fechado em 2026-09-16; mexer fora de ordem reabre risco de deadlock.
- ❌ **Não** renomear job de workflow. Nome de job é contrato com o `required_status_checks`, e
  renomear deixa todo PR pendente para sempre.
- ❌ **Não** redesenhar o `discover`/`doctor` — o escopo é **qual template** e **contra o que** se
  compara.
- ❌ **Não** tratar aqui o `trackfw.yaml` reescrito por gate (#366, já corrigido) — outra causa.

## Linked ADR
<!-- Reference the ADR that governs this requirement -->
ADR: 

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: docs/roadmaps/done/ROADMAP-2026-09-17-gerador-aplica-o-template-de-consumidor-ao-proprio-produtor-e-o-doctor-prescreve-desfazer-a-correcao.md
<!-- Reference the roadmap that implements this requirement -->
Roadmap: 
