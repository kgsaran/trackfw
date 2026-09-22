---
status: Done
date: 2026-09-17
author: ""
adr: ""
roadmap: "docs/roadmaps/done/ROADMAP-2026-09-17-jira-base-url-do-repositorio-vira-destino-de-post-autenticado-e-um-pr-que-edita-so-a-config-exfiltra-a-credencial-do-ci.md"
---

# REQ: jira_base_url do repositorio vira destino de post autenticado e um PR que edita so a config exfiltra a credencial do ci

> Date: 2026-09-17 | Status: Done
| Linear Issue: 
| Jira Issue: 

## Motivation

Issue **#380**, achado da Wave 0 da REQ do `sync` e verificado linha a linha pelo arquiteto.

### A cadeia

1. `jira_base_url` é lido do **`trackfw.yaml`** — arquivo **versionado do repositório**
   (`internal/config/config.go:507-508`).
2. Chega ao cliente sem validação (`internal/sync/jira.go:29`, `:60`).
3. A URL do POST é montada por **concatenação de string**:
   `url := c.BaseURL + "/rest/api/3/issue"` (`jira.go:100-101`).
4. E recebe a credencial:
   `req.Header.Set("Authorization", "Basic "+base64(email+":"+token))` (`jira.go:107-109`).

Não há `url.Parse`, verificação de esquema nem allowlist de host — `grep` por
`url.Parse|scheme|TLS` em `internal/sync/jira.go` volta vazio.

### 🔴 O que transforma isto de "config errada" em exfiltração

A precedência é **`trackfw.yaml` primeiro, ambiente depois** (`jira.go:29-31`, `:38-45`):

```go
baseURL := sc.JiraBaseURL           // vem do repositório
if baseURL == "" { baseURL = os.Getenv("JIRA_BASE_URL") }

token := sc.JiraToken
if token == "" { token = os.Getenv("JIRA_TOKEN") }
```

Num CI onde o `JIRA_TOKEN` é **secret do runner** e o `jira_base_url` vem do **repositório**, um PR
que altera **apenas uma linha de configuração** redireciona o POST autenticado para um host
escolhido, entregando o `Authorization: Basic base64(email:token)`. O atacante não toca em código,
não lê o secret e não precisa de permissão especial: faz o CI enviá-lo.

Sem verificação de esquema, `jira_base_url: http://…` também funciona — credencial em texto claro.

### O contraste interno que mostra que a forma segura já existe aqui

`internal/sync/linear.go:69` **fixa** o endpoint:
`http.NewRequest("POST", "https://api.linear.app/graphql", …)`.

O Linear é imune à classe pela mesma razão que o Jira é vulnerável. A diferença não foi decidida —
é consequência de o Jira ser self-hostable.

### 🔴 A tensão que torna esta REQ não-trivial

**Quem hospeda Jira próprio configura `jira_base_url` legitimamente** — é o caso de uso normal, e é
por isso que o campo existe. Uma correção que simplesmente proíba a chave no `trackfw.yaml` quebra
usuários legítimos e será revertida.

O problema não é o campo ser configurável. É **a mesma fonte controlar o destino e a credencial ir
junto sem que ninguém confirme**. A correção tem de separar essas duas coisas sem tirar a capacidade.

## Acceptance Criteria

- [ ] **AC1** — 🔴 **O destino deixa de ser determinado por arquivo do repositório quando a credencial
      vem do ambiente.** A regra precisa ser escrita de forma que um PR que edite só o `trackfw.yaml`
      **não** consiga mudar para onde uma credencial de CI é enviada. A forma (precedência invertida,
      exigência de par, allowlist fora da árvore) é decisão do ML — o invariante é este.
- [ ] **AC2** — 🔴 **Reescrito após a Wave 0: como eu tinha redigido, este AC era impossível de
      satisfazer.**

      Eu exigia "nenhum passo extra" para o uso legítimo. A Wave 0 mostrou que a combinação
      **vulnerável** — `token` do ambiente + `base_url` da config — é **a mesma combinação de boa
      prática**: quem hospeda Jira próprio commita a URL (que não é segredo) e mantém o token fora do
      repositório. Não existe sinal que distinga "config confiável" de "config alterada por um PR":
      a origem é idêntica nos dois casos.

      Portanto o AC passa a ser **por combinação**, e o passo extra fica confinado à única que é
      perigosa:

      | `base_url` | `token` | Comportamento exigido |
      |---|---|---|
      | config | config | funciona, **sem passo extra** — segredo e destino já compartilham confiança |
      | env | env | funciona, **sem passo extra** — nenhum dos dois vem do repositório |
      | **config** | **env** | 🔴 **recusa por padrão**, com erro que diga exatamente o que fazer |
      | env | config | funciona — o destino não vem do repositório |

      A recusa da linha crítica é liberável por **opt-in explícito fora da árvore** (variável de
      ambiente), porque o ato de defini-la é uma decisão de quem controla o CI — não de quem abre o
      PR. Um usuário afetado paga **uma linha, uma vez**; um atacante não ganha nada editando o
      repositório.

      🔴 A mensagem de recusa é parte do AC, não cortesia: ela precisa nomear as duas origens em
      conflito e a variável que libera. Uma recusa que não ensina a saída vira issue de suporte e
      depois um `--force` genérico.
- [ ] **AC3** — `jira_base_url` é **validado** antes de virar destino: `url.Parse`, esquema exigido
      (`https`, com exceção declarada para `http` apenas se houver decisão explícita), e recusa
      **nomeada** para valor que não seja URL absoluta. Concatenação de string deixa de ser a forma.

      **Decisão do arquiteto após a Wave 0 — incluir `CheckRedirect`.** A Wave 0 mediu duas formas de
      redirect e elas têm vereditos distintos: hostname diferente ⇒ o próprio stdlib do Go remove o
      `Authorization` (`shouldCopyHeaderOnRedirect`), então **não há vetor**; mesmo hostname com porta
      diferente ⇒ o header **é preservado**, e aí há.

      O resíduo é estreito, mas o custo de fechá-lo é baixo **e o padrão já existe neste repositório**:
      `internal/thirdparty/fetch.go:57` faz `url.Parse` + exige `https` + revalida o esquema no
      `CheckRedirect`. Reutilize-o em vez de inventar — e, se divergir, declare por quê.

      ⚠️ Decidir se um redirect `http`→`https` no **mesmo host** deve ser permitido é parte deste AC;
      a Wave 0 não testou esse caminho e o deixou declarado.
- [ ] **AC4** — Falsificação em duas direções: (a) `jira_base_url` apontando para host diferente do
      esperado, com token vindo do ambiente ⇒ **recusa nomeada, nenhuma requisição emitida**; (b) o
      caso legítimo self-hosted ⇒ requisição emitida para o host configurado.
      🔴 A direção (a) prova ausência de tráfego, não só código de erro: o teste tem de falhar se uma
      requisição sair.
- [ ] **AC5** — 🔴 **Nenhum teste emite requisição de rede real**, e isso é garantido por construção
      (cliente injetável / servidor local), não por promessa. Um teste que aponte para host externo
      para "provar" a exfiltração seria o próprio defeito, encenado.
- [ ] **AC6** — A superfície inteira da classe está enumerada e tratada: `jira_email`, `jira_project`
      e qualquer outra chave de `trackfw.yaml` que influencie **destino** de requisição autenticada.
      O número entra no relatório. Pela Regra Dura de Causa Raiz, o que tiver a mesma causa é
      corrigido **aqui**.
- [ ] **AC7** — Gate anti-reintrodução: nenhuma URL de requisição autenticada montada por
      concatenação a partir de valor de config, sem validação. Com allowlist justificada, se houver
      sítio legítimo.

## Negative Scope

- ❌ **Não** remover a capacidade de apontar para Jira self-hosted. Ver AC2.
- ❌ **Não** redesenhar a integração com Jira/Linear — a REQ é sobre **destino e credencial**.
- ❌ **Não** tratar aqui o `req_dir` (REQ própria, já em PR #382) — mecanismo diferente, embora o
  arquivo de origem seja o mesmo.
- ❌ **Não** introduzir gerenciamento de segredos: quem guarda o token continua sendo o usuário/CI.

## Linked ADR
<!-- Reference the ADR that governs this requirement -->
ADR: 

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: docs/roadmaps/done/ROADMAP-2026-09-17-jira-base-url-do-repositorio-vira-destino-de-post-autenticado-e-um-pr-que-edita-so-a-config-exfiltra-a-credencial-do-ci.md
<!-- Reference the roadmap that implements this requirement -->
Roadmap: `docs/roadmaps/done/ROADMAP-2026-09-17-jira-base-url-do-repositorio-vira-destino-de-post-autenticado-e-um-pr-que-edita-so-a-config-exfiltra-a-credencial-do-ci.md`
