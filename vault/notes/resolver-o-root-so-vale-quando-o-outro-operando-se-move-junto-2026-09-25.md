# Resolver o root só vale quando o **outro operando** se move junto

> 2026-09-25 · REQ-2026-08-31 (contenção de escrita) · ML-8A, fecha o issue #402

## O que esta nota economiza

Quem for corrigir "root não resolvido" em mais algum sítio vai tentar duas coisas que **parecem**
certas e são medidamente erradas: (1) trocar `filepath.Clean(root)` por `EvalSymlinks(root)` e
parar aí; (2) confiar que o idioma "resolve com fallback em duas etapas" satisfaz o analisador de
P2. Ambas custaram medição nesta ML.

## Achado 1 — o P2 casa pela **primeira ligação** do identificador, não pelo valor final

`buildEnv` (`internal/pathguard/containment_analyzer_test.go`) grava `env[nome]` **só na primeira
vez** que vê uma atribuição (`if _, seen := env[ident.Name]; !seen`). Logo o idioma canônico do
projeto:

```go
resolvedCwd := cwd                                  // ← esta é a ligação que fica
if rc, rcErr := filepath.EvalSymlinks(cwd); rcErr == nil {
    resolvedCwd = rc                                // ← esta o analisador NÃO vê
}
```

é **correto em runtime** e **não resolvido** para o P2.

🔴 Consequência concreta: `internal/commands/discover.go:31-38`, citado no handoff do ML-8A como
*"o padrão correto já existe dentro desta mesma REQ"*, **não satisfazia o P2** — ele era um dos 22
`root-unresolved`. Quem satisfazia era `scaffold.go`, por outro motivo: `projectRoot()` é uma
**chamada** listada em `approvedResolvers`, e chamada não depende de `env`.

**Regra de bolso:** o root de uma guarda tem de vir de **uma** atribuição a partir de **uma**
chamada de resolvedor. Reatribuição condicional é invisível. Foi por isso que o ML-8A criou
`pathguard.ResolveRoot` e colapsou nele as 5 cópias manuais do idioma (`scaffoldRoot`, 3× `absHome`
em `scaffold.go`, `resolvedCwd` em `discover.go`).

## Achado 2 — o discriminante que decide se um sítio é corrigível

**Resolver o root sozinho troca o falso-negativo por um falso-positivo.** `RejectSymlinks` para
por **igualdade de string** com o root; se o alvo ficou no namespace lógico e o root foi para o
resolvido, a caminhada nunca chega à condição de parada e recusa uma escrita legítima.

O teste, aplicado sítio a sítio:

> **O outro operando se move junto?** Se o alvo é construído por `filepath.Join` a partir do root,
> dentro da mesma função, sim → corrigível, mecanicamente. Se o alvo vem de fora (parâmetro do
> usuário, caminho relativo que o resto da função usa, conteúdo gravado em artefato), não → a
> correção é uma **decisão**, não um conserto de argumento.

🔴 E o alvo **nunca** pode ser resolvido por `EvalSymlinks` para casar: isso é a autorrefutação do
ML-6A — resolver o alvo apaga exatamente o symlink que a guarda existe para recusar. `Join` é
textual e por isso é o único jeito de mover o alvo.

### Os dois sítios que reprovaram o teste, com a medição

| sítio | por que não move |
|---|---|
| `internal/integrations/manager.go` `Manager.resolve` | no ramo `IsAnchored \|\| IsAbs` o destino é um caminho **absoluto do usuário**, aceito verbatim, não derivado do root. Com `ResolveRoot(root)`: **7 testes reprovam**, ex. `TestClaimOrigin_LegacyManifestReadsAsCatalog` → `destination "/var/folders/….claude/agents/trackfw-backend.md" is outside project root` |
| `internal/generators/update.go` `UpdateHarness` | `home` não é só o root da guarda: ele é **gravado verbatim** dentro de `.claude/settings.json`, `.codex/hooks.json`, `.gemini/settings.json`, `.cursor/hooks.json`. Resolver muda o **conteúdo** gerado (`/var/…` → `/private/var/…`); **15 testes** fixam a forma lógica |

Nos dois casos a contenção **não** está degradada: root e alvo compartilham o namespace. Armadilha 3
é o **descasamento** entre namespaces, não a escolha de um deles.

## Achado 3 — `root-aliased` é a metade da Wave 8 que ninguém contou

`filepath.Clean(root)` no argumento era 16 sítios. Mas o analisador media também 16 escritas
**guardadas sob outra grafia do root** (`kindRootAliased`): `internal/discover/discover.go` calculava
`root := resolveRoot(rootDir)`, guardava `Join(root, …)` e **escrevia** `Join(rootDir, …)`. O fluxo
passa pela guarda; o argumento não. 13 dos 16 caíram com uma troca de variável. Os 3 restantes
(`adr.go NewADRDraft`, `roadmap.go MoveRoadmap` ×2) reprovam o teste do Achado 2 e estão fixados por
nome em `containment_live_test.go` com a razão medida.

## Medição de fechamento (ML-8A)

| medida | antes | depois |
|---|---|---|
| root literalmente `filepath.Clean(…)` | 16 | **0** |
| root sem proveniência de resolvedor | 22 | **3** |
| escritas sob outra grafia de root | 16 | **3** |
| findings P1 | 15 | 15 (todos ponto cego) |
| `liveKnownFailOpen` | vazia | vazia |

Falsificação da armadilha 3 em `internal/thirdparty/resolved_root_test.go`: com `quarantine.go`
revertido à forma pré-fix, `WriteQuarantine` **recusa** um root alcançado por symlink —
`refusing symlink path ".../link"` — e o teste nomeia o sítio.

## Relacionadas

- [[beneath-entre-namespaces-degrada-escopo-da-guarda-em-vez-de-recusar-2026-09-25]] — a mesma
  fronteira de namespace, vista pelo lado do escopo da guarda
- [[recusa-de-root-em-funcao-void-o-analisador-exige-que-a-recusa-seja-acionada-2026-09-25]] — por que
  `updateHooksSurgical` ganhou o par `…Entry` em vez de descartar a recusa
- [[analisador-de-contencao-a-populacao-e-a-decisao-cara-nao-o-predicado-2026-09-25]]
