# Gerador aplica template de consumidor ao produtor; doctor prescreve desfazer a correção

**Data:** 2026-09-17 | **Issue:** #376 | **REQ:** REQ-2026-09-17-gerador-aplica-o-template-de-consumidor-ao-proprio-produtor-e-o-doctor-prescreve-desfazer-a-correcao

## Causa raiz

`BuildDiscoverGitHubActionsWorkflowContent()` em `internal/generators/scaffold_doctor.go` devolvia um template de consumidor (`go install ...@v<versão>`) **incondicionalmente**. Os três sítios que o consomem (`discover.go:276`, `update.go:1976`, `scaffold_doctor.go:269`) herdavam o template errado.

No repositório do próprio trackfw (o produtor), o doctor comparava o arquivo corrigido em disco contra o template de consumidor e reportava `scaffold-divergent` permanente, prescrevendo `trackfw update` — que reintroduziria o defeito. O sinal de "o arquivo está correto" era invisível para a ferramenta.

## Discriminante

O `go.mod` do projeto declara `module github.com/kgsaran/trackfw`. Esse é o único sinal necessário e suficiente:

- **Produtor:** compila do fonte (`go build -o /usr/local/bin/trackfw ./cmd/trackfw`). Usar `go install ...@vX` aqui validaria com o binário publicado — um PR que quebrasse `trackfw validate` passaria porque o verificador não enxerga a mudança.
- **Consumidor:** instala do release (`go install ...@v<versão>` pinada). O pin importa para que a CI do consumidor não dependa de uma versão não declarada.

## Sítios corrigidos

| Sítio | Função |
|---|---|
| `internal/generators/scaffold_doctor.go:269` | compara o disco contra o template |
| `internal/generators/update.go:1976` | reescreve o arquivo no `update` |
| `internal/discover/discover.go:276` | escreve o arquivo no `discover --init` |

Todos passam `IsProducerGoMod(root)` para `BuildDiscoverGitHubActionsWorkflowContent(isProducer bool)`.

## `IsProducerGoMod` — implementação line-wise (não bytes.Contains)

A função lê o `go.mod` e itera pelas linhas com `strings.TrimSpace`. **Não usar `bytes.Contains(data, []byte("module github.com/kgsaran/trackfw\n"))` com trailing `\n`**: checkouts CRLF no Windows fazem a linha terminar em `\r\n`, a correspondência falha, e o produtor é classificado como consumidor somente no Windows — lê como CI flaky, não como bug.

## AC8 — gate bloqueante

`check-ci-workflow-pin-parity.sh` usava `check_discover_pin` que exigia literalmente `@v<versão>` no template do discover. Um template de produtor (que compila do fonte) não tem essa string — o gate reprovaria a própria correção.

Solução: `dump_go` agora emite `dv_go_consumer.yml` e `dv_go_producer.yml` com as assinaturas `BuildDiscoverGitHubActionsWorkflowContent(false)` e `BuildDiscoverGitHubActionsWorkflowContent(true)`. A função gerada chama o builder com a aridade correta.

## Cuidado com "go install" em comentários YAML

O template de produtor inclui um comentário YAML explicativo que contém a string "go install" como parte do aviso. Usar `grep -qF 'go install'` para detectar o problema captura o comentário como falso-positivo. O padrão correto é `grep -qFe 'run: go install'` (verifica o step executável) ou `- run: go install` (mas esse começa com `-` que BSD grep interpreta como flag — usar `-e` para passar o padrão).

## Job-id imutável

`governance-go-install` é contrato com `required_status_checks`. Renomear deixa todo PR pendente para sempre (não vermelho — pendente, que é pior: não há sinal de falha, apenas ausência de aprovação). O id foi mantido em ambas as variantes do template.

## Causa separada — #387

`governance_mode: lenient` no `trackfw.yaml` deste repositório move todas as violations para warnings e faz `trackfw validate` sair 0 incondicionalmente. Isso torna `governance-go-install` e `governance-install-script` exit-0 por construção — **não são evidência de nada** enquanto #387 não for tratado. O aceite do ML-1A se apoia em `trackfw doctor` (AC7), no gate corrigido (AC8) e em `make quality`.
