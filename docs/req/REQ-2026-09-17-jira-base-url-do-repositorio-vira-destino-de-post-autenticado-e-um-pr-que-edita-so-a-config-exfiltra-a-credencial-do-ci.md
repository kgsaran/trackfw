---
status: Open
date: 2026-09-17
author: ""
adr: ""
roadmap: "docs/roadmaps/wip/ROADMAP-2026-09-17-jira-base-url-do-repositorio-vira-destino-de-post-autenticado-e-um-pr-que-edita-so-a-config-exfiltra-a-credencial-do-ci.md"
---

# REQ: jira_base_url do repositorio vira destino de post autenticado e um PR que edita so a config exfiltra a credencial do ci

> Date: 2026-09-17 | Status: Open
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
- [ ] **AC2** — Uso legítimo de Jira self-hosted **continua funcionando**, sem passo extra para quem
      já tem `jira_base_url` e `jira_token` na mesma origem. 🔴 Uma correção que quebre esse caso será
      revertida e o defeito volta — a compatibilidade é parte do remédio, não concessão.
- [ ] **AC3** — `jira_base_url` é **validado** antes de virar destino: `url.Parse`, esquema exigido
      (`https`, com exceção declarada para `http` apenas se houver decisão explícita), e recusa
      **nomeada** para valor que não seja URL absoluta. Concatenação de string deixa de ser a forma.
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
Roadmap: docs/roadmaps/wip/ROADMAP-2026-09-17-jira-base-url-do-repositorio-vira-destino-de-post-autenticado-e-um-pr-que-edita-so-a-config-exfiltra-a-credencial-do-ci.md
<!-- Reference the roadmap that implements this requirement -->
Roadmap: 
