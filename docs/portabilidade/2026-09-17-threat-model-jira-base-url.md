# Threat Model — jira_base_url como destino de POST autenticado

> Hades · 2026-09-17 · Branch `fix/jira-base-url` · ML-0A da REQ #380

---

## 1. Reprodução — listener LOCAL (127.0.0.1)

### Configuração

**Binário:** `bin/trackfw` compilado de `internal/sync/jira.go` e `internal/commands/sync.go`
via `make build` neste worktree (`fix/jira-base-url`).

**Ambiente de teste:** diretório temporário fora do worktree contendo:
- `trackfw.yaml` com `jira_base_url: http://127.0.0.1:18080`, `jira_email` e `jira_project`
- `jira_token` **ausente** do `trackfw.yaml` — fornecido **exclusivamente** via `JIRA_TOKEN=FAKE_CI_TOKEN_DO_NOT_USE`
- `docs/req/REQ-repro.md`: uma REQ com `Status: Open` e sem `jira_issue`

Isso replica o cenário CI: `jira_base_url` vem do repositório; o token vem do secret do runner.

**Listener:** servidor HTTP local em `127.0.0.1:18080` que imprime headers recebidos e retorna
`201 {"key":"TEST-1"}`.

### Invocação

```bash
cd <testdir> && JIRA_TOKEN="FAKE_CI_TOKEN_DO_NOT_USE" bin/trackfw sync --to=jira
```

### Saída do binário

```
REQ                                                     ISSUE
------------------------------------------------------  ----------
docs/req/REQ-repro.md                                   TEST-1
```

### Saída capturada pelo listener (o que chegou ao destino)

```
--- REQUEST RECEIVED ---
Method: POST
Path: /rest/api/3/issue
Header: Accept-Encoding: [gzip]
Header: User-Agent: [Go-http-client/1.1]
Header: Content-Length: [268]
Header: Accept: [application/json]
Header: Authorization: Basic <REDACTED>
Authorization-present: YES
Header: Content-Type: [application/json]
Body-length: 268
------------------------
```

**O que isso prova, verificado por execução:**

1. O `config.Load()` lê `jira_base_url` do `trackfw.yaml` e ele chega a `JiraClient.BaseURL`
   sem validação alguma.
2. O caminho `sync --to=jira` → `SyncToJira()` → `NewJiraClient()` → `CreateIssue()` emite
   a requisição com o `Authorization: Basic <REDACTED>` para o destino configurado.
3. A **precedência config-antes-de-env** funciona como afirmado: `jira_base_url` veio do
   `trackfw.yaml`; `JIRA_TOKEN` veio do ambiente. O binário aceitou o par sem aviso.
4. `http://` funciona — o listener recebeu a requisição, credencial em texto claro na rede.

O token não é transcrito. O listener foi construído para imprimir `Basic <REDACTED>` em vez
do valor; a saída acima é a saída real do listener.

---

## 2. Enumeração do raio — quais chaves influenciam destino de requisição autenticada

### Resultado: uma chave influencia destino; as demais influenciam conteúdo ou credencial

Verificado por leitura de `internal/sync/jira.go`, `internal/sync/linear.go`,
`internal/thirdparty/fetch.go`, `internal/commands/doctor_remote.go`,
`internal/forge/resolve.go`, e por busca de `http.\|NewRequest\|Authorization` em
`internal/` (excluindo `serve/`, que só recebe conexões).

**Quatro chaves Jira:**

| Chave | Campo | Papel na requisição | Influencia destino? |
|---|---|---|---|
| `jira_base_url` | `JiraClient.BaseURL` | `url := c.BaseURL + "/rest/api/3/issue"` (jira.go:101) | **SIM** — é a URL destino |
| `jira_email` | `JiraClient.Email` | `base64(email+":"+token)` — folded no `Authorization` (jira.go:108) | NÃO — credencial, não destino |
| `jira_token` | `JiraClient.Token` | `base64(email+":"+token)` — o secret em si (jira.go:108) | NÃO — credencial, não destino |
| `jira_project` | `JiraClient.Project` | `"project": {"key": c.Project}` no corpo JSON (jira.go:72) | NÃO — conteúdo do payload |

**Outros comandos com requisições de rede:**

| Comando | Mecanismo | Credencial de CI envolvida? | Na classe? |
|---|---|---|---|
| `sync --linear` | `linear.go:69`: URL hardcoded `https://api.linear.app/graphql` | `LINEAR_API_KEY` do env | NÃO — endpoint fixo |
| `thirdparty fetch` | `fetch.go:57`: `url.Parse` + exige `https` + `CheckRedirect` revalida scheme | Nenhuma — fetch não autentica | NÃO — sem credencial; URL validada |
| `doctor --remote` | Chama `gh api` como subprocesso; `configForge` é validado contra allowlist `{"github","gitlab","bitbucket","azure"}` (`forge/resolve.go:57`) — não vira URL. `remoteURL` (`git remote get-url origin`) é usado só para identificar o forge, não para construir endpoint (`forge/resolve.go:41-44`). Credencial gerenciada pelo `gh` CLI. | GH_TOKEN/GITHUB_TOKEN gerenciado pelo `gh`, não pelo trackfw | NÃO |
| `release tag`, `ship` | Mesmo padrão de `doctor --remote` — `gh api repos/{owner}/{repo}` com placeholder resolvido pelo `gh`, não pelo Go | Idem | NÃO |
| `serve` | Servidor HTTP que recebe conexões, não emite | N/A | NÃO |

**Scripts (`scripts/`):** `curl` e `wget` presentes em `install.sh` e `verify-npm-channels.sh`,
mas todos com URLs hardcoded ou derivadas de nome/versão do repositório (GitHub releases API).
Nenhum lê `trackfw.yaml` para construir URL. Nenhum envia credencial via `Authorization` header.

**Raio = 1 chave** (`jira_base_url`) em 1 comando (`sync --to=jira`).
O remédio pode ser cirúrgico.

---

## 3. Precedência — confirmação com arquivo:linha

Todas as quatro chaves Jira seguem **config-antes-de-env** sem exceção:

```go
// jira.go:29-44
baseURL := sc.JiraBaseURL          // 1. trackfw.yaml
if baseURL == "" {
    baseURL = os.Getenv("JIRA_BASE_URL")  // 2. env (fallback)
}
email := sc.JiraEmail
if email == "" {
    email = os.Getenv("JIRA_EMAIL")
}
token := sc.JiraToken
if token == "" {
    token = os.Getenv("JIRA_TOKEN")
}
project := sc.JiraProject
if project == "" {
    project = os.Getenv("JIRA_PROJECT")
}
```

Não existe precedente interno de ordem inversa em `internal/sync/`.
`linear.go:26-32` segue o mesmo padrão — mas o Linear não tem URL configurável,
então a ordem não cria exposição equivalente.

**Implicação para AC1:** a direção "inverter a precedência de `jira_base_url` quando
`JIRA_TOKEN` vier do ambiente" não tem precedente interno de aplicação — nenhuma outra
chave faz essa distinção —, mas é mecanicamente simples e não afeta as outras três chaves.

---

## 4. Compatibilidade — o que cada direção do #380 preserva

O caso de uso legítimo que não pode ser quebrado: empresa com Jira self-hosted configura
`jira_base_url: https://jira.empresa.com` no `trackfw.yaml` e usa `jira_token` na mesma
origem (config local, não CI). Esse é o caso normal — é por isso que o campo existe.

### Direção 1 — separar origem de credencial e destino

> `jira_base_url` do `trackfw.yaml` só é usado quando `JIRA_TOKEN` **não** vier
> do ambiente. Quando o token vem de `JIRA_TOKEN` (env), o `jira_base_url` deve vir
> também do ambiente (`JIRA_BASE_URL`).

**Preserva o caso self-hosted?** SIM, sem passo extra — desde que o usuário use
`jira_token` no `trackfw.yaml` (par `config+config`). Esse par continua funcionando
como hoje.

**Custo:** o par `(config base_url, env token)` — o cenário CI vulnerável — para de
funcionar. Quem hoje tem esse par em CI precisará mover `jira_base_url` para variável de
ambiente.

**Assunção não verificada:** assume-se que quem configura `JIRA_TOKEN` como secret do
runner tem `JIRA_BASE_URL` disponível como variável de ambiente — em muitas organizações
isso é verdade (o endpoint do Jira é uma variável de CI padrão), mas não é universal.
O arquiteto deve confirmar se o changelog de breaking change é aceitável antes de escolher
esta direção.

**Como implementar:** em `NewJiraClient()` (jira.go), ao montar o cliente, verificar
se o token veio de `os.Getenv`; se sim, exigir que `baseURL` também venha de `os.Getenv`.

### Direção 2 — validar o destino com allowlist fora da árvore

> `jira_base_url` é validado contra uma lista de hosts aprovados em
> `~/.trackfw/jira-allowlist` ou `JIRA_ALLOWED_HOST`.

**Preserva o caso self-hosted?** SIM — mas **com passo extra**: o usuário precisa
criar o arquivo de allowlist ou a variável antes de usar. Quebra o AC2 na forma atual
("sem passo extra para quem já tem `jira_base_url` e `jira_token` na mesma origem").

**Custo:** instalação existente quebra. É a proteção mais forte, mas o custo de adoção
é real — e AC2 hoje proíbe isso.

### Direção 3 — avisar e confirmar em TTY

**Não é barreira.** Um atacante que saiba o endpoint do Jira legítimo configura seu servidor
no mesmo endereço, altera o `trackfw.yaml` para apontar para ele, e a confirmação não
aparece (o valor já foi "confirmado"). Útil como UX complementar, não como remédio primário.

### Recomendação

**Direção 1** + AC3 conforme detalhado em §5.

Direção 1 fecha o vetor CI, é cirúrgica, e preserva o caso self-hosted sem passo extra
para o par `(config, config)`. AC3 — `url.Parse` + https — cobre o vetor cross-hostname
(stdlib Go já o bloqueia, §5 Teste B). O gap residual de redirect same-hostname é mais
estreito: o arquiteto decide antes de ML-1A se AC3 deve exigir também o `CheckRedirect`
para esse caso.

---

## 5. Redirect — duas formas testadas, conclusões distintas

Dois testes por execução com o binário real. Os resultados divergem — e a distinção importa para AC3.

### Teste A — redirect mesmo hostname, porta diferente (confirmado por execução)

`jira_base_url: http://127.0.0.1:18081`. Listener em 18081 redireciona (HTTP 302) para
`http://127.0.0.1:18082`. Mesmo hostname (`127.0.0.1`), porta diferente.

**Resultado em 18082:**

```
--- REQUEST at :18082 ---
Method: GET Path: /rest/api/3/issue
Header: Authorization: Basic <REDACTED>
Authorization-present: YES at :18082
Header: Content-Type: [application/json]
Header: Referer: [http://127.0.0.1:18081/rest/api/3/issue]
Body-length: 0
```

**`Authorization-present: YES` — verificado por execução.** Go encaminha `Authorization`
quando o redirect permanece no mesmo hostname (independentemente da porta). A credencial
chega ao destino final. Um `jira_base_url` que aponte para um servidor que redirecione
para si mesmo em porta diferente ainda exfiltra a credencial.

Nota: Go converte POST→GET em redirect 302; o `Authorization` é o ativo exfiltrado, então
um GET com credencial é comprometimento completo independentemente do método ou body.

### Teste B — redirect hostname diferente (exfiltração AUSENTE)

`jira_base_url: http://localhost:18081`. Listener em 18081 redireciona para
`http://127.0.0.1:18082`. Hostname diferente (`localhost` vs `127.0.0.1`), ambos loopback.

**Resultado em 18082:** headers `Content-Type`, `Referer`, `Accept-Encoding`, `User-Agent`,
`Accept` — **sem `Authorization`**.

**Verificado por execução: Go STRIP o header `Authorization` em redirects cross-hostname.**
O mecanismo é `shouldCopyHeaderOnRedirect` na stdlib: só encaminha `Authorization` quando
o destino é o mesmo hostname ou subdomínio do original. Um redirect de `https://jira-minha-empresa.com`
para `https://attacker.com` teria o `Authorization` removido antes de chegar ao atacante.

**Assunção não verificada:** "o Jira self-hosted não redireciona para outro hostname em uso
normal". Reverse proxies e front-ends de SSO podem introduzir redirects cross-hostname em
configurações não-padrão.

### Impacto sobre AC3

**AC3 como redigido (`url.Parse`, esquema `https`) é necessário e, para o vetor
cross-hostname, suficiente** — a stdlib já cobre esse caso.

**O gap que permanece:** redirect para o mesmo hostname em porta diferente (Teste A). Um
servidor em `https://jira.empresa.com` que redirecione para `https://jira.empresa.com:8443`
(ou para um caminho lateral no mesmo host) encaminharia o `Authorization`.

O remédio para o gap residual é um `CheckRedirect` que rejeite qualquer redirect (ou que
exija host:porta idênticos ao original). Precedente interno: `thirdparty/fetch.go:30-39`
(que rejeita redirect para esquema não-`https`). Decidir se esse nível de proteção é
exigido pelo AC3 é responsabilidade do arquiteto ao revisar o AC antes de ML-1A.

### Como implementar (se o arquiteto ampliar AC3)

Injetar um `http.Client` dedicado em `JiraClient` com `CheckRedirect` que compare
`req.URL.Host` com o host original antes de seguir qualquer redirect. Precedente interno:
`thirdparty/fetch.go:30-39`.

---

## 6. O que ficou em aberto — residual declarado

1. **Par `(config token, config base_url)` em repositório compartilhado** não é coberto
   por esta REQ: se `jira_token` estiver no `trackfw.yaml` de um repositório compartilhado,
   o segredo está exposto independentemente do destino — problema de secret management
   fora do escopo definido no Negative Scope da REQ.

2. **Indeterminado:** não foi medido se existem instalações de CI em produção que
   combinam `(config base_url, env token)` de forma legítima. A Direção 1 seria uma
   breaking change para esses usuários. Decisão de produto sobre se uma versão de aviso
   deve preceder a mudança de comportamento — não decisão de segurança.

3. **Não testado:** redirect de `http` para `https` no mesmo host (path de upgrade).
   O `CheckRedirect` proposto (verificar host) o permitiria; se isso é desejável é
   decisão do ML-1A.

---

## Resumo para o arquiteto

| Item | Resultado |
|---|---|
| Reprodução por execução do binário real | Confirmada — `Authorization: Basic <REDACTED>` chegou ao listener configurado em `jira_base_url` |
| Raio da classe | **1 chave** (`jira_base_url`), 1 comando (`sync --to=jira`); remédio pode ser pontual |
| Precedência | Config-antes-de-env para as 4 chaves Jira (jira.go:29-44); sem precedente interno de inversão |
| Direção recomendada | **Direção 1** (separar origem) + AC3 conforme §5 |
| Redirect — duas formas testadas | **Mesmo hostname, porta diferente:** `Authorization` encaminhado (confirmado). **Cross-hostname:** `Authorization` removido pela stdlib Go (confirmado). Stdlib cobre o cenário `attacker.com`; gap residual é redirect no mesmo hostname. |
| Scripts | Nenhum `curl`/`wget` usa URL derivada de config com credencial |

---

## Gate da wave 0

```bash
P=docs/portabilidade/2026-09-17-threat-model-jira-base-url.md
test -s "$P" || { echo "FAIL: parecer ausente ou vazio: $P"; exit 1; }
for termo in "127.0.0.1" "Authorization" "jira_base_url" "compatibilidade"; do
  grep -qi -- "$termo" "$P" || { echo "FAIL: parecer nao cobre: $termo"; exit 1; }
done
grep -qiE "residual|nao consegui|não consegui|indetermin" "$P" \
  || { echo "FAIL: parecer nao declara o que ficou em aberto"; exit 1; }
grep -qE "Basic [A-Za-z0-9+/]{20,}" "$P" \
  && { echo "FAIL: parecer contem credencial nao redigida"; exit 1; }
echo "OK [wave0/threat-model-jira]: parecer presente, reproduz em local e sem credencial vazada"
```
