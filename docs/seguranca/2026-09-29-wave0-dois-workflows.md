# Wave 0 — Por que existem dois workflows, e o que quebra se um sair

> Domínio: security · Data: 2026-09-29 · Branch: `fix/dois-workflows-rodam-a-mesma-validacao`
> Owner: hades-tf · REQ: REQ-2026-09-02 · Roadmap: ML-0A

---

## Pergunta 1 — Existe caminho de adoção que recebe `trackfw-validate.yml` e NUNCA receberia `trackfw-gate.yml`?

**O que a REQ espera:** identificar se os dois workflows servem públicos distintos ou se a
coexistência é sedimentação histórica. Se houver razão legítima, o AC1 fecha documentando-a.

### Medição

**Fixture:** diretório no scratchpad com `.github/workflows/existing-ci.yml` preexistente e sem
`trackfw.yaml`. Representa o caso mais comum de adoção brownfield.

```
$ mkdir -p fixture/.github/workflows
$ cat fixture/.github/workflows/existing-ci.yml  # workflow CI preexistente
$ trackfw discover --init   (cwd=fixture, HOME=scratchpad)
```

**Saída literal:**

```
trackfw discover — scanning /scratchpad/fixture-brownfield
✓ CI: github-actions
✓ trackfw.yaml generated
...
✓ governance gates installed
```

**Após `discover --init`:**

```
$ ls fixture/.github/workflows/
  existing-ci.yml
  trackfw-validate.yml          ← escrito
$ grep ^ci: fixture/trackfw.yaml
ci: github-actions
$ ls fixture/.github/workflows/trackfw-gate.yml
  NOT FOUND
```

**`trackfw-gate.yml` ausente.** O `discover --init` escreve `trackfw.yaml` com `ci: github-actions`
e chama `InstallGates`, que escreve `trackfw-validate.yml`. Não chama `generateCIWorkflow` — este só
é chamado por `init`/`update`.

**Após `trackfw update` (mesma fixture):**

```
$ trackfw update
  ✓ .github/workflows/trackfw-gate.yml
  ✓ CI workflow atualizado
  ✓ CI workflow (discover) atualizado

$ ls fixture/.github/workflows/
  existing-ci.yml
  trackfw-gate.yml              ← escrito agora
  trackfw-validate.yml
```

**`trackfw-gate.yml` aparece.** O `update` chama `generateCIWorkflow(cfg)` onde `cfg.CI =
"github-actions"` (do `trackfw.yaml` gerado pelo `discover --init`), e escreve `trackfw-gate.yml`.

### Variante com `trackfw.yaml` preexistente (`ci: none`)

Quando `trackfw.yaml` já existe, `discover --init` sai imediatamente:

```go
// internal/commands/discover.go:138-141
if _, statErr := os.Stat(yamlPath); statErr == nil {
    fmt.Fprintln(out, "⚠ trackfw.yaml already exists — remove it first...")
    return nil       // ← early return, InstallGates NÃO é chamado
}
```

**Saída literal:**

```
$ trackfw discover --init   (fixture com trackfw.yaml ci:none)
⚠ trackfw.yaml already exists — remove it first if you want to regenerate
$ ls fixture/.github/workflows/
  existing-ci.yml              ← validate.yml NÃO escrito
```

Neste caso, `trackfw-validate.yml` não é escrito de forma alguma.

### Veredito pergunta 1

**O caminho exclusivo existe, mas é transiente.**

`discover --init` (sem `trackfw.yaml` preexistente) abre uma janela onde apenas
`trackfw-validate.yml` existe. O `trackfw update` subsequente fecha essa janela escrevendo
`trackfw-gate.yml`, porque `trackfw.yaml` agora tem `ci: github-actions`.

O fenômeno descrito no issue #451 ("v9.0.0 instala `trackfw-gate.yml` ao lado do
`trackfw-validate.yml` existente") **é exatamente esta janela a ser fechada pelo update**: o
consumidor rodou `discover --init` em versão anterior, ficou com `trackfw-validate.yml` e
`ci: github-actions` no yaml, e o update da v9.0.0 escreveu `trackfw-gate.yml`.

**O caminho exclusivo persistente** existiria apenas se o usuário:
1. Rodasse `discover --init` (escrevendo `ci: github-actions` + `trackfw-validate.yml`), E
2. Nunca rodasse `trackfw update`.

O único caminho onde `trackfw-validate.yml` existe e `trackfw-gate.yml` jamais seria escrito é
aquele onde `trackfw.yaml` tem `ci: none` explícito E `discover --init` foi rodado manualmente
escrevendo o arquivo — o que contradiz o fluxo de `discover --init`, que escreve `ci: github-actions`
ou não escreve nada (se `trackfw.yaml` preexiste).

**O que falsificaria este veredito:** um caminho de instalação que escreve `trackfw-validate.yml`
sem escrever `ci: github-actions` no `trackfw.yaml`. Nenhum foi encontrado na medição acima.

---

## Pergunta 2 — Os dois arquivos gerados são funcionalmente equivalentes?

**O que a REQ espera:** tabela de equivalência campo a campo dos dois workflows **gerados** (não das
intenções). Destaque especial para `required_status_checks` — o job ID é contrato.

### Medição — conteúdo gerado para um projeto consumidor

Conteúdo literal de cada arquivo na fixture após `discover --init` + `update` (consumer = projeto
que não é o próprio trackfw):

```yaml
# trackfw-gate.yml — gerado por generateCIWorkflow (scaffold.go:2553)
name: trackfw-gate
on:
  pull_request:
    branches: [main]

jobs:
  governance-install-script:
    runs-on: ubuntu-latest
    timeout-minutes: 10
    env:
      TRACKFW_VERSION: "9.1.0"
    steps:
      - uses: actions/checkout@v7
      - name: Install trackfw
        run: |
          curl -sSfL https://github.com/kgsaran/trackfw/releases/latest/download/install.sh | sh
      - name: Governance gate
        run: trackfw validate
```

```yaml
# trackfw-validate.yml — gerado por BuildDiscoverGitHubActionsWorkflowContent (scaffold_doctor.go:59)
name: trackfw validate
on:
  push:
    branches: [main]
  pull_request:
jobs:
  governance-go-install:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v7
      - uses: actions/setup-go@v7
        with:
          go-version: "1.25"
      - run: go install github.com/kgsaran/trackfw/cmd/trackfw@v9.1.0
      - run: trackfw validate
```

### Tabela de equivalência (consumer branch)

| Propriedade | `trackfw-gate.yml` | `trackfw-validate.yml` |
|---|---|---|
| `name:` | `trackfw-gate` | `trackfw validate` |
| Trigger `pull_request:` | `branches: [main]` | irrestrito |
| Trigger `push:` | ausente | `branches: [main]` |
| Job ID | `governance-install-script` | `governance-go-install` |
| `timeout-minutes:` | `10` | ausente |
| `env.TRACKFW_VERSION:` | `"9.1.0"` | ausente |
| Instalação | `curl|sh` + variável `TRACKFW_VERSION` | `go install @v9.1.0` via `setup-go@v7` |
| Toolchain Go exigida do runner | não | não (`setup-go` provisiona) |
| ML-1A fetch origin/main | ausente (consumer) | ausente (consumer) |
| Comando de validação | `trackfw validate` | `trackfw validate` |
| `required_status_checks` exposto | `governance-install-script` | `governance-go-install` |

### Observações críticas

**Job IDs diferentes = dois contratos de `required_status_checks` distintos.**
`governance-install-script` e `governance-go-install` são dois nomes diferentes para o mesmo
veredito. Qualquer projeto que configure `required_status_checks: [governance-go-install]` (o único
gerado pelo caminho `discover --init`) teria sua branch protection quebrada se `trackfw-validate.yml`
fosse removido sem migração — o `governance-install-script` do `trackfw-gate.yml` não satisfaz a
regra exigindo `governance-go-install`.

**Triggers não equivalentes.** `trackfw-gate.yml` dispara apenas em `pull_request: branches: [main]`.
`trackfw-validate.yml` dispara em `push: branches: [main]` e em `pull_request:` irrestrito. Em
particular: `trackfw-gate.yml` não valida pushes diretos na main — `trackfw-validate.yml` é o único
que o faz.

**Argumento do instalador é mais fraco do que parece.** Embora `go install` exija Go e `curl|sh`
não, `actions/setup-go@v7` provisiona o toolchain Go automaticamente. Na prática, ambos funcionam
em `ubuntu-latest` sem configuração prévia do runner. A diferença de instalador não discrimina casos
de uso na maioria dos environments de CI gerenciados.

### Veredito pergunta 2

**Os arquivos NÃO são funcionalmente equivalentes.** Diferem em job ID (contrato de branch
protection), em cobertura de trigger (gate não cobre push em main), e em mecanismo de instalação
(curl|sh vs go install). A distinção mais consequente é o job ID: são dois checks diferentes,
não duas cópias do mesmo check.

---

## Pergunta 3 — Instalador: existe adotante atendido só por um deles?

**O que a REQ espera:** verificar se `go install` exige toolchain Go e se isso discrimina um subconjunto
de adotantes que só pode usar `validate.yml`; e o inverso.

### Medição

`trackfw-validate.yml` inclui `actions/setup-go@v7` com `go-version: "1.25"` — o runner provisiona
Go se não tiver. Não há situação em GitHub Actions (ubuntu/windows/macos latest) onde `go install`
falhe por ausência de toolchain.

`trackfw-gate.yml` usa `curl|sh` com download de `github.com`. Em ambientes com bloqueio de
tráfego externo ou runners sem curl, o gate.yml falha onde validate.yml não falharia.

**Situação prática encontrada:** a tabela de diferença real entre os dois é o job ID exposto como
check name e a cobertura de trigger — não o instalador.

### Veredito pergunta 3

O argumento de "públicos distintos por instalador" não se sustenta na maioria dos ambientes de CI
gerenciados. A distinção real é o check name para `required_status_checks` e a cobertura de trigger.
Um adotante que configurou `governance-go-install` como required check depende do
`trackfw-validate.yml` por razão de contrato, não por razão de instalador.

---

## Pergunta 4 — O que o `doctor` cobra de cada um, e o que passa a cobrar errado se um sumir?

**O que a REQ espera:** entender a dependência entre `doctor` e cada workflow.

### Medição — scaffold_doctor.go

```go
// scaffold_doctor.go:307-323 — gate.yml: condicional em cfg.CI
switch cfg.CI {
case "github-actions":
    // checkScaffoldArtifact — expected, reporta se ausente
}

// scaffold_doctor.go:333-344 — validate.yml: condicional em presença no disco
if _, err := os.Stat(discoverWorkflowPath); err == nil {
    // checkScaffoldArtifact — opcional, reporta se divergente (nunca se ausente)
}
```

**`trackfw-gate.yml`:** o `doctor` ESPERA o arquivo quando `cfg.CI == "github-actions"`. Se o
arquivo sumir, o doctor reporta `scaffold-missing` (achado verdadeiro).

**`trackfw-validate.yml`:** o `doctor` só verifica se o arquivo EXISTE NO DISCO. Se o arquivo não
existir, o doctor não reporta nada. Se existir mas estiver divergente do template, reporta
`scaffold-divergent`.

**ProjectTargetIDs inclui `ci-workflow` quando `discoverWorkflowPresent || cfg.CI != "" && cfg.CI != "none"`** —
o `update` gerencia o validate.yml via `refreshDiscoverGitHubActionsWorkflowIfPresent` se o arquivo
existir, mas nunca o cria.

### Impacto se `trackfw-validate.yml` deixasse de ser escrito

1. O `doctor` não reportaria achado novo (ele só verifica se o arquivo existe).
2. O `update` deixaria de incluir `ci-workflow` via `discoverWorkflowPresent` para projetos sem
   `ci: github-actions` no yaml.
3. Projetos que configuraram `governance-go-install` em `required_status_checks` teriam branch
   protection quebrada (o check nunca mais seria produzido).
4. A cobertura de push em main desapareceria para esses projetos.

### Impacto se `trackfw-gate.yml` deixasse de ser escrito

1. O `doctor` reportaria `scaffold-missing` para todo projeto com `ci: github-actions`.
2. O `update` target `ci-workflow` precisaria ser atualizado ou o doctor produziria achado falso
   permanente.
3. Projetos com `governance-install-script` em `required_status_checks` teriam branch protection
   quebrada.

### Veredito pergunta 4

**O `doctor` trata os dois assimetricamente por design**: gate.yml é "esperado quando configurado";
validate.yml é "verificado quando presente". Remover qualquer um sem ajustar o doctor cria achados
falsos (gate) ou silêncio sobre branch protection quebrada (validate). A dependência é explícita no
código e constitui um requisito de co-entrega para qualquer mudança no comportamento do gerador.

---

## Inventário dos sítios que citam a decisão inexistente

**Citação:** "both can coexist in the same project (ADR-2026-08-28)" ou equivalente.

**ADR real:** `ADR-2026-08-28-gate-de-ci-gerado-nasce-pinado-na-versao-que-o-gerou-e-o-install-sh-honra-trackfw-version.md`
— zero ocorrências de `trackfw-validate.yml`; decide apenas versão pinada e `TRACKFW_VERSION`.

```
$ grep -c "trackfw-validate.yml" docs/adr/ADR-2026-08-28-*.md
0
```

### Sítios encontrados (5 — o roadmap dizia 2)

| # | Arquivo | Linha | Texto |
|---|---|---|---|
| 1 | `internal/generators/scaffold_doctor.go` | 29-31 | "Both files can coexist in the same project — ADR-2026-08-28 names this exact case as the motivation for pinning both install mechanisms" |
| 2 | `internal/generators/scaffold_doctor.go` | 334-335 | "both can coexist in the same project (ADR-2026-08-28)" |
| 3 | `internal/generators/discover_workflow_trigger_test.go` | 17 | "A coexistência dos DOIS arquivos é decidida e está escrita (ADR-2026-08-28, citada em scaffold_doctor.go)" |
| 4 | `docs/seguranca/2026-09-28-triagem-issues-abertas.md` | 221 | "a coexistência dos dois arquivos é decidida (ADR-2026-08-28)" |
| 5 | `docs/seguranca/2026-09-28-triagem-issues-abertas.md` | 245 | "coexistência dos dois workflows (decisão da ADR-2026-08-28, para cobrir dois métodos de instalação)" |

**Sítio 3 (test comment) é o mais perigoso**: a Regra Dura de Reconciliação exige que cada teste
afirme uma conclusão do mesmo ML. Este comentário afirma uma decisão de governança — e a decisão é
falsa. O teste em si protege o trigger correto (válido), mas a justificativa que ele traz é a
afirmação não verificada que travou esta REQ por três semanas.

**Sítios 4 e 5** são de autoria de hades-tf (2026-09-28) — gerados a partir do mesmo comentário de
código, não de leitura direta da ADR. Constituem propagação da citação original.

**Fonte primária:** `scaffold_doctor.go:334` (sítio 2) — é o comentário mais antigo e o que os
demais citam explicitamente.

---

## Refutação de premissa da REQ

**Premissa refutada:** "São a mesma verificação. A única diferença real é o método de instalação."
(REQ, seção Motivation)

**Medição:** os job IDs diferem (`governance-install-script` vs `governance-go-install`). Trata-se
de dois contratos distintos de `required_status_checks`, não de "o mesmo check instalado de dois
jeitos". Um projeto que configurou o ID do validate.yml como required check não pode substituí-lo
pelo gate.yml sem reconfigurar branch protection. Esta é a diferença mais consequente entre os dois
arquivos.

---

## Veredito global

**Os dois arquivos são necessários para fins distintos — mas a coexistência no mesmo projeto é o
problema, não a existência de cada um.**

| Caso | Arquivo necessário | Motivo |
|---|---|---|
| Projeto que rodou `discover --init` antes de qualquer `update` e configurou `governance-go-install` como required check | `trackfw-validate.yml` | contrato de branch protection |
| Projeto com `ci: github-actions` e required check `governance-install-script` | `trackfw-gate.yml` | contrato de branch protection |
| Push direto na main | apenas `trackfw-validate.yml` | único que cobre esse trigger |
| Pull request em `branches: [main]` | ambos (se ambos instalados) | duplicação |

**A assimetria é o defeito confirmado** (REQ, seção AMPLIADA):

- `generateCIWorkflow` (gate) grava incondicionalmente quando `cfg.CI == "github-actions"`
- `writeCIWorkflow` (validate) verifica idempotência no disco (`os.Lstat(dest); err == nil → return nil`)

O `update` fecha a janela exclusiva do `discover --init`: qualquer projeto que rodou `discover
--init` e depois `update` termina com DOIS arquivos e DOIS jobs do mesmo veredito num PR.

**O que falsificaria "necessários":** um mecanismo que, ao escrever qualquer um dos dois, verificasse
se o outro já existe e ajustasse o ID do job para convergir para um único check name. Isso tornaria
os dois funcionalmente intercambiáveis e um deles dispensável. Esse mecanismo não existe hoje.

---

## Premissa sobre a ADR-2026-08-28

A claim "ADR-2026-08-28 names this exact case as the motivation" (`scaffold_doctor.go:30`) é **mais
forte do que a leitura do ADR suporta**. A ADR decide pinagem de versão no template gerado —
incidentalmente, em dois mecanismos de instalação — mas não decide explicitamente que os dois
workflows devem coexistir. É uma interpretação retrospectiva de um fato colateral, não uma decisão
registrada.

A coexistência **nunca foi decidida** por nenhum ADR. O AC4-bis da REQ está correto.

---

*Parecer gerado para: ML-0A, Wave 0 do ROADMAP-2026-09-22.*
*Nenhuma linha de implementação foi escrita neste ML.*
*`git diff --stat trackfw.yaml`: vazio (confirmado ao final).*
