---
status: Done
date: 2026-07-30
author: "trackfw_architect"
adr: "docs/adr/ADR-001-trackfw-como-trilho-de-governanca-para-agentes-ia.md"
roadmap: "docs/roadmaps/done/ROADMAP-2026-07-30-padrao-unico-de-saida-de-versao-nos-tres-clis.md"
---

# REQ: padrao unico de saida de versao nos tres CLIs

> Date: 2026-07-30 | Status: Done
| Linear Issue:
| Jira Issue:

## Motivation

A saída de versão diverge entre os três CLIs em **duas** superfícies. Medido executando os binários na
`v5.0.0`:

| Superfície | Go | Node.js | Python |
|---|---|---|---|
| `trackfw version` | `trackfw v5.0.0` | `trackfw 5.0.0` | `trackfw 5.0.0` |
| `trackfw --version` | `trackfw v5.0.0` | **`5.0.0`** | `trackfw 5.0.0` |

Duas quebras distintas:

1. **Prefixo `v`** — só o Go o emite, porque `internal/version/version.go` armazena a string
   `"v5.0.0"`. Node.js e Python derivam de `npm/package.json` e `pypi/pyproject.toml`, que **não podem**
   carregar o `v`: npm rejeita, e o SemVer especifica que `v1.2.3` não é uma versão semântica — o `v` é
   convenção de *tag Git*, não de string de versão.
2. **Prefixo `trackfw`** — o `--version` do Node.js imprime o número puro, sem o nome do programa,
   porque é o comportamento default do `.version()` do commander.

## O gate de paridade assina a divergência

`scripts/check-cli-parity.sh:108` usa **regex diferente para o Node.js**:

```bash
"$GO_BIN" --version                         | grep -Eq '^trackfw .+'
node "$ROOT_DIR/npm/bin/trackfw" --version  | grep -Eq '^([0-9]+\.){2}[0-9]+|^0\.0\.0-dev$'
PYTHONPATH=... python3 -m trackfw --version | grep -Eq '^trackfw .+'
```

O gate que existe para detectar divergência **codifica esta divergência como esperada**. É a mesma
classe de vacuidade que o projeto já combateu em outros gates: uma asserção afrouxada para um runtime
específico torna a diferença invisível e permanente. Enquanto essa linha existir, nenhuma auditoria
futura vai reportar o problema.

O `^trackfw .+` dos outros dois também é frouxo demais: aceita `trackfw v5.0.0` e `trackfw 5.0.0`
igualmente — que é exatamente por que o prefixo `v` sobreviveu a todas as auditorias até agora.

## Decisão de formato

Canônico: **`trackfw <versão>`**, com a versão em SemVer **sem prefixo `v`**, idêntico nas duas
superfícies e nos três runtimes.

```
trackfw 5.0.0
```

Escolhido por alinhamento com o padrão de versões do Python e com os manifestos `npm/package.json` e
`pypi/pyproject.toml`, que são a fonte da string nos outros dois runtimes e não admitem o `v`. Adotar o
formato com `v` obrigaria Node.js e Python a **concatenar** um prefixo na impressão, criando duas
representações da mesma versão dentro do mesmo runtime.

A **tag Git permanece `v5.0.0`** — ali o prefixo é a convenção correta e não muda.

## Fatos verificados no código

1. `internal/version/version.go` contém `var Version = "v5.0.0"`. O valor é consumido em dois pontos:
   `internal/commands/version.go:15` (`fmt.Println("trackfw", version.Version)`) e
   `internal/commands/root.go:22` (campo `Version` do cobra, que alimenta `--version`). Remover o `v` da
   constante corrige **as duas** superfícies de uma vez.
2. `scripts/install.sh:52` já faz `VERSION_BARE="${VERSION#v}"` sobre a tag do GitHub. Não depende do
   formato impresso pelo CLI e **não é afetado**.
3. `pypi/trackfw/__init__.py` deriva de `importlib.metadata` com fallback literal; ambos já sem `v`.

## Acceptance Criteria

- [x] `trackfw version` imprime `trackfw <semver>` sem prefixo `v`, byte-idêntico nos três runtimes.
- [x] `trackfw --version` imprime exatamente a mesma linha que `trackfw version`, nos três runtimes.
- [x] `internal/version/version.go` passa a armazenar a versão **sem** o `v`.
- [x] `scripts/check-cli-parity.sh` usa **a mesma asserção para os três runtimes** nas duas superfícies —
      a regex específica do Node.js é removida.
- [x] A asserção passa a exigir o formato exato (`^trackfw [0-9]+\.[0-9]+\.[0-9]+`), não `^trackfw .+`,
      que era frouxa o bastante para deixar o `v` sobreviver.
- [x] Cenário de comparação byte-a-byte das duas superfícies entre os três runtimes, encadeado em
      `make quality`, com prova de falsificação.
- [x] Protocolo de release em `CLAUDE.md` atualizado, se citar o formato com `v`.
- [x] `make quality` passa e `bin/trackfw validate --json` retorna 0 violações.

## Escopo negativo

- **Não** alterar a tag Git nem o formato de release do GitHub: `v<x.y.z>` permanece.
- **Não** alterar `npm/package.json`, `pypi/pyproject.toml` nem os valores dos manifestos — já corretos.
- **Não** incluir a flag curta `-v` nesta REQ. Verificado: `-v` funciona **apenas no Go**; o Node.js
  responde `error: unknown option '-v'` e o Python cai no `usage:`. É divergência de **quais flags
  existem**, não de **formato de saída** — e resolvê-la exige decisão própria, porque adicionar `-v` a
  dois runtimes é feature e removê-la do Go é breaking change. Registrada aqui para não se perder;
  merece REQ separada.

## Impacto observável

O CLI Go deixa de imprimir o prefixo `v`, e o `--version` do Node.js passa a incluir `trackfw `. Ambas
são mudanças de saída observável e devem constar no CHANGELOG do próximo release.

## Linked ADR
ADR: `docs/adr/ADR-001-trackfw-como-trilho-de-governanca-para-agentes-ia.md`

Não altera decisão arquitetural; aplica a regra dura de paridade a uma superfície onde o próprio gate a
havia dispensado.

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: `docs/roadmaps/done/ROADMAP-2026-07-30-padrao-unico-de-saida-de-versao-nos-tres-clis.md`
