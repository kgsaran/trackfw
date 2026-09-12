# Segunda varredura de segurança e paridade — Hades / Codex

> Data: 2026-09-11  
> Agente: `hades-tf`  
> Orquestração: Codex / Zeus Architect  
> Escopo: fuzzing operacional orientado a casos-limite, paridade diferencial, `serve`, release/instalação e falsificação de gates  
> Estado: concluída

## Objetivo

Executar uma segunda rodada independente depois da auditoria Hades inicial, concentrada em quatro superfícies: diferenças entre Go/Node/Python, servidor HTTP observado por casos adversariais, fluxo de distribuição e gates que podem produzir falso verde.

Esta rodada não alterou código de produto. Os arquivos temporários usados pelos gates foram criados em diretórios temporários e removidos pelos próprios scripts.

## Matriz executada

| Superfície | Comando | Resultado |
|---|---|---|
| CLI geral | `scripts/check-cli-parity.sh` | PASS |
| `validate` e hooks de guarda | `scripts/check-validate-parity.sh` | PASS |
| `update`, dry-run e sandbox | `scripts/check-update-parity.sh` | PASS |
| `roadmap move` | `scripts/check-roadmap-move-parity.sh` | PASS |
| `doctor` | `scripts/check-doctor-parity.sh` | PASS |
| hooks por agente | `scripts/check-agent-hooks-parity.sh` | PASS |
| hooks do harness | `scripts/check-harness-hooks-parity.sh` | PASS |
| artefatos gerados | `scripts/check-artifact-parity.sh` | PASS |
| resolução de HOME | `scripts/check-homedir-parity.sh` | PASS |
| third party/proveniência | `scripts/check-thirdparty-parity.sh` | PASS |
| browser opener e injeção | `scripts/check-serve-browser-security.sh` | PASS |
| pin de versão no instalador | `scripts/check-install-version-pin.sh` | PASS |
| release/tag | `scripts/check-release-tag-parity.sh` | PASS |
| integridade referencial | `scripts/check-referential-integrity.sh` | PASS, com lacuna de vacuidade conhecida |

## Resultado da análise diferencial

Os casos de JSON inválido, arquivo ilegível, UTF-16, caminhos relativos, `$PWD`, tilde, hooks ausentes, hooks presentes, symlinks dangling, dry-run, namespaces não declarados e conteúdo de terceiros produziram respostas equivalentes nos três runtimes nos gates correspondentes.

Isso reduz a probabilidade de uma nova divergência simples entre Go, Node e Python nas superfícies cobertas. Não prova equivalência total: a cobertura segue dependente da população de cenários existente e não substitui testes com os binários distribuídos em cada sistema operacional.

## Resultado da varredura do `serve`

O gate de browser confirmou novamente:

- ausência de interpolação de URL em `exec(string)` no Node;
- ausência de `shell=True` no Python;
- rejeição de hosts com metacaracteres e zonas IPv6 perigosas nos três CLIs;
- preservação do argv único para URLs legítimas;
- mensagens e códigos de erro coerentes entre runtimes.

O gate de bind (`scripts/check-serve-address-parity.sh`) não pôde medir o servidor neste ambiente: todos os processos receberam `EPERM` ao abrir sockets, inclusive loopback. Os casos de bind devem ser repetidos em CI ou em um ambiente com rede permitida.

A análise black-box também confirmou o achado H-01 da auditoria anterior: o handler Node retorna `200` e o conteúdo externo quando um arquivo autorizado é symlink para fora da raiz. O código Go usa a mesma validação lexical antes de `os.ReadFile`; portanto o caso permanece pendente nos dois runtimes. O handler Python resolve o caminho físico com `realpath` antes de autorizar.

## Resultado da cadeia de release

O pin de versão passou 16 cenários, incluindo separadores de comando, substituição de comando, backticks, espaços, newline embutido e traversal no nome do destino.

O fluxo ainda não verifica o checksum publicado: `.goreleaser.yaml` gera `checksums.txt`, mas `scripts/install.sh` baixa e extrai somente o tarball. Esse é o H-02 da auditoria Hades anterior e continua sendo a principal lacuna da cadeia de distribuição.

Não foi reproduzida uma escrita fora do diretório de instalação por um tarball normalizado pelo GNU tar. Ainda assim, a extração ocorre antes de qualquer verificação criptográfica; a recomendação continua sendo verificar checksum e validar entradas do arquivo antes de extrair.

## Resultado da falsificação dos gates

Os gates de paridade e falsificação executados passaram seus cenários negativos. O cenário referencial também passou, mas o código de `scripts/check-referential-integrity.sh` só itera sobre `docs/req/*.md` e não exige que a população seja não vazia. Portanto, o `PASS` não elimina o achado M-05; ele é precisamente o tipo de resultado que uma guarda de vacuidade deve impedir.

O gate `scripts/check-falsify-shard-coverage.sh` não foi executado como suíte porque exige três argumentos operacionais (`script-fonte`, quantidade de shards e diretório de artefatos baixados); a chamada sem argumentos apenas retornou sua mensagem de uso. Isso não é uma falha do produto, mas deixa a cobertura de shards inconclusiva nesta rodada.

## Novos achados

Nenhum novo defeito explorável foi confirmado nesta segunda rodada. O resultado mais relevante é a confirmação independente dos dois achados Hades anteriores:

1. **H-01:** leitura via symlink externo em `/api/file` no Go e Node.
2. **H-02:** ausência de verificação do checksum no instalador.

Também foram confirmados como residuais já conhecidos:

- assets estáticos Node com contenção lexical, sem canonicalização física;
- `serve` em rede sem autenticação e com CORS universal, por desenho opt-in;
- gate referencial verde sobre população vazia;
- falsificação B2 do namespace ainda desalinhada da proteção Node atual, conforme `make quality`.

## Conclusão operacional

A comparação diferencial está sólida nas superfícies que os gates cobrem. O próximo ganho de segurança não está em repetir os mesmos testes, mas em transformar H-01 e H-02 em microlotes corrigidos e em executar os casos de rede no CI. Depois disso, a melhor expansão é fuzzing de entrada contra os binários empacotados, com corpus compartilhado e comparação de exit code, stdout, stderr e arquivos criados.
