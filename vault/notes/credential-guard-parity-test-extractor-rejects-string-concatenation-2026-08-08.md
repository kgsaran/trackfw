# credential-guard: o extrator do gate de paridade Node/Python exige constante literal única (`const NAME = \`...\``/`NAME = r"""..."""`), sem suporte a concatenação — 2026-08-08

## Contexto

ML-1B (`ROADMAP-2026-08-08-credential-guard-modo-block-por-padrao-cobertura-de-read-write-e-resolucao-de-arquivo-referenciado.md`)
portou a mudança de fallback de MODE (Go, ML-1A) para Node: `credentialGuardModeResolution`, uma
constante Go compartilhada entre `credentialGuardProjectTail`/`credentialGuardGlobalTail`,
parametrizada por `$DEFAULT_MODE` e concatenada via `+` no Go (`internal/generators/scaffold.go`).

Tentativa inicial em Node: extrair a mesma lógica para uma constante `CG_MODE_RESOLUTION` e montar
`CG_PROJECT_TAIL`/`CG_GLOBAL_TAIL` via concatenação de template literals
(`` `DEFAULT_MODE="warn"\n` + CG_MODE_RESOLUTION + `if [...` ``), espelhando o padrão Go 1:1.

Isso quebra o gate de paridade Go↔Node↔Python
(`internal/generators/credential_guard_test.go:getNodeSourceBlock`/`getPythonSourceBlock`): o
extrator lê o arquivo-fonte JS/Python como texto e captura, via regex, um único bloco
`` `const NAME = \`...\`` `` (Node) ou `NAME = r"""..."""` (Python) — **não avalia o módulo**, só
faz parsing textual da declaração. Uma constante montada por concatenação (`const X = \`a\` + Y +
\`b\``) não casa com esse regex — o extrator retorna vazio/errado silenciosamente para essa
constante específica, e o teste falha comparando "Go completo" com "Node com um bloco faltando",
não porque o *conteúdo* diverge, mas porque a *forma sintática* não é reconhecida.

## Decisão

Em Node (e, por extensão, Python — mesma restrição do extrator `getPythonSourceBlock`), a resolução
de MODE (grep de `credential_guard.mode` + `case`/fallback) é **replicada como texto literal
idêntico** dentro de cada uma das duas constantes (`CG_PROJECT_TAIL` com `DEFAULT_MODE="warn"`,
`CG_GLOBAL_TAIL` com `DEFAULT_MODE="block"`), cada uma um único template literal sem concatenação.
Resultado funcional idêntico ao Go (mesmo texto de shell gerado, confirmado por
`TestCredentialGuardScript_ParityAcrossStacks`/`TestGlobalCredentialGuardScript_ParityAcrossStacks`
Go↔Node), só a forma de organização do código-fonte diverge (Go pode usar uma função/constante
Go real porque o teste Go roda o gerador de verdade — `getGoCredentialGuardScript`/
`getGoGlobalCredentialGuardScript` chamam `GenerateCredentialGuardScript`/
`GenerateGlobalCredentialGuardScript` e leem o arquivo escrito, não fazem parsing de texto-fonte).

## Por que importa para ML-1C (Python)

`_CG_PROJECT_TAIL`/`_CG_GLOBAL_TAIL` em `pypi/trackfw/generators/init_gen.py` devem seguir o mesmo
padrão: **não** extrair uma `_CG_MODE_RESOLUTION` compartilhada com concatenação de string — replicar
o texto (com `DEFAULT_MODE` local a cada bloco) como um único literal `r"""..."""` por constante,
senão `getPythonSourceBlock` também vai falhar silenciosamente em achar o conteúdo certo.

## Referências

- `internal/generators/scaffold.go` (`credentialGuardModeResolution`, referência canônica Go)
- `npm/src/generators/hooks.js` (`CG_PROJECT_TAIL`, `CG_GLOBAL_TAIL` — implementação Node final)
- `internal/generators/credential_guard_test.go` (`getNodeSourceBlock`, `getPythonSourceBlock`,
  `TestCredentialGuardScript_ParityAcrossStacks`, `TestGlobalCredentialGuardScript_ParityAcrossStacks`)
- `vault/notes/credential-guard-second-layer-cmd-extraction-json-not-raw-token-2026-08-08.md` (nota
  irmã do ML-1A sobre outra armadilha de portabilidade na mesma feature)
