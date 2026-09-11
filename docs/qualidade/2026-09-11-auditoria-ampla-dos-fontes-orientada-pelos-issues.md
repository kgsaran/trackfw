# Auditoria ampla dos fontes orientada pelos issues do consumidor

> Data: 2026-09-11  
> Auditor: Zeus Architect  
> Escopo: revisão ampla, somente leitura, dos três CLIs e dos gates do repositório  
> Estado: concluída com achados pendentes de correção

## Objetivo

Investigar erros ocultos ou ainda não rastreados nos fontes do trackfw usando como norte o padrão de descoberta observado nos issues recentes do consumidor. A auditoria procurou reproduzir as condições que produziram os relatos, em vez de verificar apenas se os testes existentes estavam verdes.

O foco foi identificar:

- caminhos de uso real que não são exercitados pelos testes;
- divergências entre Go, Node.js e Python;
- falsos verdes causados por testes ou gates vácuos;
- fallbacks silenciosos, perda de informação e saída enganosa;
- diferenças de layout, estado, separador de caminho, symlink e encoding;
- contratos afirmados na documentação mas não verificados por execução.

## Fonte do corpus

A API de issues do GitHub estava indisponível durante a auditoria: `gh issue list` falhou ao conectar a `api.github.com`, e não havia navegador disponível na sessão. Portanto, não foi feita uma afirmação de leitura direta dos 38 corpos remotos.

O corpus local utilizado contém as triagens, roadmaps, pareceres, notas de vault e commits que preservam os diagnósticos dos issues. O histórico recente referencia 33 issues entre `#284` e `#328`, além das análises anteriores de issues como `#216`, `#268`, `#274`, `#275`, `#279`, `#288`, `#309`, `#314`, `#315`, `#319` e `#320`. Esse material foi suficiente para reconstruir os padrões de falha e testar os caminhos correspondentes no checkout atual.

## Padrão de descoberta extraído

Os relatos do consumidor encontram defeitos porque combinam quatro técnicas que o ciclo interno nem sempre aplica ao mesmo caminho:

1. Executam o comando completo em um projeto descartável, com a configuração que um usuário realmente usaria.
2. Comparam o resultado entre os três CLIs, em vez de testar cada implementação contra seu próprio helper.
3. Falsificam o teste ou o gate: aplicam uma mutação que deveria ser detectada e verificam se o veredito muda.
4. Usam ambientes que o CI local não representa: Windows sem privilégio de symlink, console `cp1252`, caminhos com separadores nativos, nomes curtos 8.3, forge ausente e configurações com mais de um agente.

O padrão recorrente é: o teste interno prova que a implementação concorda com a própria premissa; o consumidor altera a premissa e observa o comportamento real.

## Estado de validação encontrado

### Comandos executados

| Comando | Resultado |
|---|---|
| `bin/trackfw context` | score de governança 100/100 |
| `bin/trackfw validate --json` | 0 violações, 172 warnings preexistentes, modo `lenient` |
| `go test ./...` | passou |
| `node --test tests/*.test.js` em `npm/` | 897 passaram, 0 falharam |
| `python3 -m pytest pypi/tests -q` | 1724 passaram + 66 subtests |
| `git diff --check` | passou |
| `make quality` | falhou em `check-agent-namespace-union.sh` |

### Falha do `make quality`

O ponto de falha foi:

```text
FAIL [agent-namespace-union/direction-b2/node/detects-symlink-regression]
corrupted binary did not escape through the symlink
... cannot determine agent namespace ... path is outside roadmap directory ...
checagem vácua
```

O comportamento atual do Node já rejeita o caminho resolvido por symlink externo com erro explícito. O teste de falsificação continua esperando o comportamento antigo, em que o arquivo escaparia para fora do projeto. Assim, a proteção do produto está no sentido correto, mas o contra braço do gate ficou incompatível com a correção e agora reprova a qualidade ou, se ajustado sem cuidado, pode deixar de provar a propriedade relevante.

Esse é um problema de qualidade do teste/gate, não evidência de que a fuga ainda ocorra. O contra braço deve ser reescrito para exigir a recusa explícita, nomear o caminho recusado e provar que nenhum arquivo foi criado em `outside/done/`. Também deve permanecer um controle positivo para um symlink legítimo dentro de `roadmap_dir`.

## Achados

### A1 — Alta: `status` Python ignora REQs em subpastas

**Arquivo:** [pypi/trackfw/commands/status.py](/Users/kgsaran/Sistemas/Desenvolvimento/workspace/trackfw/pypi/trackfw/commands/status.py:50)

**Código observado:** `_count_reqs_by_status()` chama `_list_files(req_dir)` diretamente. `_list_files()` lê apenas os arquivos imediatamente dentro de `req_dir`.

**Reprodução:** uma árvore temporária contendo apenas `docs/req/done/REQ-x.md` produziu:

```text
{'total': 0, 'open': 0, 'done': 0, 'closed': 0, 'other': 0}
```

**Impacto:** `trackfw status` Python informa inventário incompleto para layouts por estado e por agente. A mesma árvore é descoberta por `resolve_req_files()` em outras partes do runtime, portanto o comando apresenta uma visão diferente da validação e de `context`.

**Causa:** o comando preservou um contador flat enquanto o restante do runtime migrou para o resolvedor único de REQs.

**Como detectar antes:** executar `status` nos três binários contra a mesma árvore temporária com REQs em `req_dir/*.md`, `req_dir/done/*.md` e `req_dir/<agent>/done/*.md`; comparar o inventário completo, não somente o exit code.

**Correção recomendada:** usar `resolve_req_files(cfg_local)` para formar a população do contador, deduplicando os caminhos antes de ler o status. Adicionar cenários de paridade para flat, por estado e `by_agent`.

### A2 — Alta: `serve` Python perde arestas quando a referência usa separador Windows

**Arquivo:** [pypi/trackfw/serve/api_chain.py](/Users/kgsaran/Sistemas/Desenvolvimento/workspace/trackfw/pypi/trackfw/serve/api_chain.py:187)

**Código observado:** `_find_node_by_ref()` faz `ref.strip()`, depois compara o valor contra `by_id` e aplica `os.path.basename(ref)`. A função importa `normalize_ref_separator`, mas não normaliza a referência antes da resolução.

**Reprodução:** com uma REQ contendo `Roadmap: docs\\roadmaps\\wip\\ROADMAP-x.md` e um roadmap real em `docs/roadmaps/wip/ROADMAP-x.md`, `get_chain()` devolveu somente a aresta do roadmap para a REQ; a aresta da REQ para o roadmap desapareceu.

**Impacto:** o dashboard `/api/chain` apresenta uma cadeia incompleta, embora o arquivo referenciado exista. O usuário pode interpretar a ausência da aresta como falta de vínculo ou como artefato órfão.

**Causa:** o identificador dos nós é normalizado para `/`, mas o valor vindo do frontmatter não passa pela mesma fronteira de formato.

**Como detectar antes:** executar `/api/chain` contra a mesma fixture três vezes, usando referências com `/`, `\\`, caminho completo e basename; comparar `edges` entre os runtimes.

**Correção recomendada:** aplicar `normalize_ref_separator(ref)` antes de tentar `by_id` ou basename. Manter a normalização somente na referência de saída, sem alterar o caminho nativo usado para ler o arquivo.

### A3 — Alta: títulos com controles forjam estrutura de REQ, ADR e note

**Arquivos:**

- [internal/generators/req.go](/Users/kgsaran/Sistemas/Desenvolvimento/workspace/trackfw/internal/generators/req.go:101)
- [internal/generators/adr.go](/Users/kgsaran/Sistemas/Desenvolvimento/workspace/trackfw/internal/generators/adr.go:65)
- [internal/generators/note.go](/Users/kgsaran/Sistemas/Desenvolvimento/workspace/trackfw/internal/generators/note.go:34)
- equivalentes em `npm/src/generators/{req,adr,note}.js` e `pypi/trackfw/generators/{req,adr,note}.py`

**Código observado:** o slug usado no nome do arquivo é normalizado, mas o título original é interpolado diretamente no corpo Markdown. No note, o título também é inserido dentro de uma string YAML entre aspas.

**Vetor:** um título contendo `\n` pode criar headings e campos adicionais. No note, uma quebra de linha seguida por conteúdo YAML pode alterar a estrutura lida pelo parser. Em REQ e ADR, o título pode criar seções artificiais que parecem parte do documento gerado.

**Impacto:** artefatos gerados deixam de ser estruturalmente confiáveis; um título fornecido como argumento passa a controlar o documento produzido. Isso afeta validação, dashboard, revisão humana e rastreabilidade.

**Causa:** a sanitização foi aplicada ao nome do arquivo, mas não à superfície de emissão do conteúdo.

**Como detectar antes:** executar cada comando com títulos contendo newline, `\r`, tab, `---`, `#`, `:` e aspas; em seguida fazer parse do frontmatter e comparar headings esperados. O teste deve ser feito contra o binário real dos três CLIs, porque testar somente `toSlug()` não cobre a interpolação do corpo.

**Correção recomendada:** rejeitar caracteres de controle na entrada de título, com mensagem e exit code alinhados entre os três CLIs. Adicionar teste negativo para cada gerador e teste positivo para acentos, aspas e pontuação legítima.

### A4 — Média: `check-referential-integrity.sh` fica verde em árvore vazia

**Arquivo:** [scripts/check-referential-integrity.sh](/Users/kgsaran/Sistemas/Desenvolvimento/workspace/trackfw/scripts/check-referential-integrity.sh:10)

**Código observado:** o gate itera `docs/req/*.md` e continua quando não há arquivos. Nenhuma guarda exige que pelo menos uma REQ tenha sido visitada.

**Impacto:** uma regressão que remove ou deixa de materializar toda a população de REQs pode passar pelo gate com `Referential integrity OK`.

**Causa:** a guarda de vacuidade não faz parte do contrato do script.

**Como detectar antes:** executar o gate em três fixtures: árvore válida com REQ, árvore com REQ inválida e árvore sem REQ. O terceiro cenário deve reprovar nomeando a ausência da população.

**Correção recomendada:** contar arquivos visitados e falhar quando a população esperada estiver vazia. A mensagem deve distinguir “nenhuma REQ existe” de “não foi possível ler as REQs”.

### A5 — Média: o gate B2 Node está desalinhado e bloqueia o quality gate

**Arquivo relacionado:** `scripts/check-agent-namespace-union.sh` e testes em `npm/tests/roadmap_move.test.js`.

**Impacto:** `make quality` retorna exit code 2 mesmo com o produto rejeitando o caminho externo. Isso impede distinguir regressão de segurança, teste obsoleto e indisponibilidade do cenário.

**Causa:** o cenário foi escrito para detectar uma fuga antiga e não foi reconciliado depois que `agentFromPath()` passou a retornar erro explícito.

**Correção recomendada:** atualizar a expectativa do contra braço para o novo contrato e provar as duas propriedades: recusa explícita e ausência de escrita externa. Não remover o cenário nem transformá-lo em skip por causa do erro esperado.

## Itens analisados e descartados como falso positivo

Alguns diagnósticos registrados nas triagens antigas não são achados atuais:

- `note_orphan` no Node já existe em `npm/src/validator/index.js:1582` e é aplicado em `:3733`; a conclusão contrária vinha de `grep` que classificava o arquivo como binário por causa de byte NUL.
- A injeção de comando via `serve --host` já foi corrigida nos três CLIs; `check-serve-browser-security.sh` passou os cenários de referência vulnerável, URL legítima, host inválido e zone ID.
- O schema de hook já usa `hookSpecificOutput.permissionDecision=deny`; o gate correspondente passou nos quatro emissores derivados.
- O trust check do `barrier` atual está fail-closed, com uma única saída `trusted: true`; os cenários de ausência de Git, ref ausente, conteúdo divergente e `--trust-local-gates` passaram.
- A ausência de `fchmod` no Windows Python já possui fallback explícito com `getattr(os, "fchmod", None)` nos três caminhos de escrita observados.

## Lacunas de cobertura que continuam relevantes

O relatório de `scripts/check-parity-contract-coverage.sh` encontrou 47 seções de contrato sem gate cross-runtime completo. As mais relevantes para prevenir a próxima série de issues são:

| Área | Lacuna |
|---|---|
| REQ | `req list` e `req move` não têm cenário cross-runtime para todos os layouts |
| `serve` | cadeia, separadores e referências stale ainda têm cobertura incompleta entre runtimes |
| `ship` | forge real, fallback sem CLI externo e distinção de `lenient` não são exercitados ponta a ponta |
| instalação | defaults de `--scope` sem flag e sem TTY não são comparados entre os CLIs |
| artefatos | schemas publicados não entram no gate de paridade |
| gates | várias verificações provam texto ou presença, mas não mutação semântica |
| Windows | console `cp1252`, symlink sem privilégio, nomes 8.3 e resolução nativa continuam exigindo ambiente específico |

Essas lacunas não foram promovidas automaticamente a bugs: cada uma precisa de reprodução concreta antes de virar correção. Elas são, porém, os pontos com maior probabilidade de repetir o padrão observado nos issues.

## Ordem recomendada de tratamento

1. Corrigir o contra braço B2 Node e fazer `make quality` voltar a um veredito confiável.
2. Corrigir o contador de REQs Python e criar paridade E2E com layouts flat, por estado e `by_agent`.
3. Corrigir a normalização de referências no `serve` Python e comparar o grafo dos três CLIs.
4. Fechar a injeção estrutural de títulos nos três geradores e testar os binários reais.
5. Adicionar guarda de vacuidade ao gate de integridade referencial.
6. Criar uma rodada periódica de falsificação dos gates antigos, especialmente os que usam regex ou arrays hardcoded para definir sua própria população.

## Conclusão

O consumidor externo não está encontrando defeitos por acaso. Ele está exercitando fronteiras que o ciclo interno frequentemente trata como detalhes: configuração não flat, caminho Windows, dados controláveis no texto, contra braços negativos e comandos completos. A suíte atual tem boa cobertura de regressões já conhecidas, mas ainda mistura três estados diferentes em vários lugares: produto correto, teste desatualizado e gate incapaz de observar a população.

Os cinco achados A1–A5 acima são os pontos acionáveis desta auditoria. A1, A2, A3 e A4 são problemas de produto ou gate reproduzíveis; A5 é o bloqueio imediato de qualidade causado pela divergência entre a correção recente e sua falsificação.

