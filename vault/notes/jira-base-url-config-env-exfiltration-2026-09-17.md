# jira_base_url from config + token from env = indistinguishable from legitimate use

> Hades / Apolo · 2026-09-17 · REQ #380 / ML-0A + ML-1A · Branch `fix/jira-base-url`

---

## O que acontece

`jira_base_url` vem do `trackfw.yaml` (arquivo versionado). `JIRA_TOKEN` vem do ambiente
(secret do runner de CI). O POST autenticado vai para o destino configurado pelo repositório.

Um PR que edita **somente uma linha de config** redireciona o header
`Authorization: Basic base64(email:token)` para qualquer host escolhido pelo atacante.
O token nunca precisa ser lido diretamente — o CI o envia.

Reproduzido por execução em Wave 0: `bin/trackfw sync --to=jira` com
`JIRA_TOKEN=FAKE_CI_TOKEN_DO_NOT_USE` e `jira_base_url: http://127.0.0.1:18080` no YAML →
`Authorization: Basic <REDACTED>` chegou ao listener.

## Por que é não-óbvio

**A combinação vulnerável é a mesma de boa prática.**

Quem hospeda Jira próprio commita a URL (que não é segredo) e mantém o token fora do
repositório. Não existe sinal que distingue "config confiável" de "config alterada por PR":
a origem é idêntica nos dois casos.

Uma solução que proibisse `jira_base_url` no `trackfw.yaml` quebraria usuários legítimos.
Uma solução que só adicionasse validação de URL (`url.Parse` + `https`) não fecha o vetor —
a URL válida e legítima pode ser o próprio servidor do atacante.

## A solução: opt-in por variável de ambiente, não por config

A recusa é da combinação, não do campo. A liberação precisa ser um ato de quem controla o CI
(não de quem abre o PR). Portanto:

| `base_url` | `token` | Comportamento |
|---|---|---|
| config | config | funciona |
| env | env | funciona |
| **config** | **env** | 🔴 recusa por padrão |
| env | config | funciona |

Opt-in: `TRACKFW_JIRA_ALLOW_MIXED_ORIGIN=1` no ambiente do CI (não no `trackfw.yaml`).

A mensagem de recusa nomeia as duas origens e a variável que libera — recusa sem saída vira
issue de suporte e depois um `--force` genérico.

**Critério do nome da variável:** prefixo `TRACKFW_` evita colisão com variáveis do vendedor
Jira; `ALLOW_MIXED_ORIGIN` nomeia a combinação permitida, não um skip genérico.

## Comportamento do stdlib Go em redirects

Medido por execução na Wave 0 (§5 do threat model):

- **Redirect cross-hostname** (`jira.example.com` → `attacker.com`): `shouldCopyHeaderOnRedirect`
  do stdlib **remove** o `Authorization`. Não há vetor. `url.Parse` + `https` é suficiente aqui.

- **Redirect mesmo hostname, porta diferente** (`127.0.0.1:18081` → `127.0.0.1:18082`):
  `Authorization` **é preservado**. Há vetor. Fechado por `CheckRedirect` que compara
  `jiraNormalizeHost(req.URL)` com o host original (normalizado para `hostname:port`).

- **http → https no mesmo host**: inacessível porque `validateJiraURL` exige `https` na URL
  inicial; uma downgrade (https → http) é bloqueada pelo check de esquema no `CheckRedirect`.

## Padrão reutilizável: fetch.go

`internal/thirdparty/fetch.go:30-57` já faz `url.Parse` + `https` + `CheckRedirect` para
revalidar esquema. O código de Jira segue o mesmo padrão. Quando precisar fechar a mesma
classe em outro ponto do código, buscar `fetch.go` primeiro.

## Raio da classe (medido em Wave 0)

**1 chave** (`jira_base_url`) em **1 comando** (`sync --to=jira`). As demais chaves Jira
(`jira_email`, `jira_token`, `jira_project`) influenciam credencial ou conteúdo, não destino.
O Linear tem endpoint fixo (`https://api.linear.app/graphql`). `doctor --remote`, `release` e
`ship` delegam ao `gh` CLI com forge validado contra allowlist. `thirdparty fetch` já valida.
Scripts não leem `trackfw.yaml` para montar URL com credencial.

## Gate anti-reintrodução

`scripts/check-jira-url-concat.sh` detecta conjunção: `Authorization` header + concatenação
de string com `\.BaseURL` no mesmo arquivo Go. Wired em `parity-rest`.

## Artefatos

- Threat model: `docs/portabilidade/2026-09-17-threat-model-jira-base-url.md`
- REQ: `docs/req/REQ-2026-09-17-jira-base-url-*.md`
- Implementação: `internal/sync/jira.go` (funções `newJiraClientFromSources`, `validateJiraURL`,
  `jiraNormalizeHost`, `newJiraHTTPClient`)
- Testes: `internal/sync/jira_security_test.go`
- Gate: `scripts/check-jira-url-concat.sh`
