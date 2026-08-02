# Cenário de falsificação para regressão de "parser de config volta a ser artesanal" só discrimina se a fixture usar um escalar CITADO (`"3"`) — sem aspas o cenário é vácuo

> Data: 2026-08-02 | Autor: Ártemis (ML-3B, decorrente da auditoria do ML-3A do ROADMAP-substituir-os-parsers-artesanais-de-config-por-biblioteca-yaml-nos-tres-clis) | Domínio: `scripts/check-gates-falsify.sh` / validator — os 3 CLIs

## Contexto

`vault/notes/config-legacy-line-reader-sombreia-yaml-lib-no-validate-2026-08-02.md`
documentou que `internal/validator/validator.go` (e gêmeos Node/Python) tinham um
segundo parser artesanal de `trackfw.yaml` (`readWIPConfig`/`readGovernanceMode`),
independente da biblioteca YAML já adotada em `config.Load()`. O ML-3A eliminou esses
leitores nos 3 CLIs (commit `74d70ee`) — `wipConfigFrom`/`_wip_config_from` agora só
derivam de `cfg.WipLimit` já normalizado.

## O achado (nesta auditoria)

Nenhum teste do repositório fixava essa classe de regressão. O teste unitário
`TestValidateWIPLimit_Global_HighLimit` (`internal/validator/validator_test.go`) usa
`wip_limit: 3` — **sem aspas**. Um leitor artesanal (`fmt.Sscanf("%d", ...)` em Go,
`parseInt(...)` em Node, `int(...)` em Python) lê `3` sem aspas corretamente, IGUAL à
biblioteca YAML. Se o leitor artesanal fosse reintroduzido, esse teste continuaria
verde — ele não discrimina os dois caminhos, é estruturalmente vácuo para essa
classe de bug.

## A fixture que discrimina

`wip_limit: "3"` — **com aspas**. YAML resolve a string tipada normalmente para `3`;
um `Sscanf("%d", ...)`/`parseInt`/`int()` naive recebe o texto `"3"` (aspas inclusas)
e falha ao converter, caindo no default (`1`).

Prova medida (Cenário 38, `scripts/check-gates-falsify.sh`), 4 roadmaps em `wip/`:

| trackfw.yaml           | binário correto (pós-74d70ee) | binário com leitor artesanal reintroduzido |
|-------------------------|-------------------------------|---------------------------------------------|
| `wip_limit: "3"` (citado)   | `(limit: 3)`               | `(limit: 1)` ← discrimina                    |
| `wip_limit: 3` (sem aspas)  | `(limit: 3)`               | `(limit: 3)` ← **vácuo, não discrimina**     |

Confirmado empiricamente nos 3 CLIs (Go/Node/Python) — o Cenário 38 usa só a fixture
citada, com corrupção determinística de `wipConfigFrom` (Go/Node) e `_wip_config_from`
(Python) reintroduzindo exatamente o padrão de `readWIPConfig` eliminado por
`74d70ee`.

## Lição generalizável

Qualquer fixture de falsificação para "biblioteca YAML vs. parser artesanal" (chaves
numéricas, booleanas, listas) só é não-vácua se o valor escolhido tiver uma
representação onde os dois parsers **discordam**. Para inteiros/booleanos, isso é
tipicamente um escalar citado (`"3"`, `"true"`) — o leitor artesanal (regex/Sscanf de
linha única) trata o texto bruto, a biblioteca YAML resolve o tipo. Um valor "normal"
(sem aspas, sem formatação especial) tende a ser lido identicamente pelos dois
caminhos e não prova nada — o mesmo padrão já observado para `agents:` (Cenários
34/35, listas com indentação/vírgula-dentro-de-aspas) e para escalares tipados
(Cenário 36, octal/data/`yes`).

## Como reconhecer se isso regride

- Se `readWIPConfig`/`readGovernanceMode` (ou equivalente) for reintroduzido em
  qualquer um dos 3 CLIs, o Cenário 38 deve falhar no braço de detecção
  correspondente (`falsify/wip-limit-quoted/{go,node,python}-detects-artisanal-reader-reintroduced`).
- Se uma fixture de falsificação futura para `governance_mode`/`lenient_until` (que
  compartilham o padrão `wipConfigFrom`/`governanceModeFrom`) usar um valor sem
  citação especial, revisar se ela de fato discrimina antes de confiar nela como
  regression guard — mesma armadilha.

Relacionado:
`vault/notes/config-legacy-line-reader-sombreia-yaml-lib-no-validate-2026-08-02.md`
(achado original, causa raiz),
`vault/notes/falsificacao-fixture-vacua-contra-reversao-total-vs-parcial-2026-08-02.md`,
`vault/notes/cenarios-de-falsificacao-quebram-em-refactor-do-alvo-2026-08-02.md`.
