# Auditoria de segurança dos fontes — Hades / Codex

> Data: 2026-09-11  
> Agente: `hades-tf`  
> Orquestração: Codex / Zeus Architect  
> Escopo: análise de segurança somente leitura dos três CLIs, servidor `serve`, instalador e gates  
> Estado: concluída com achados pendentes de correção

## Objetivo

Revisar o repositório a partir de uma perspectiva adversarial, procurando caminhos em que um valor controlável pelo projeto, pelo consumidor ou pelo ambiente atravesse uma fronteira de confiança e produza leitura indevida, escrita fora do escopo, execução de comando, perda de controle de segurança ou falso verde de um gate.

A revisão foi independente da auditoria ampla do Zeus. O relatório Zeus foi usado somente como contexto para evitar repetir verificações já documentadas; os achados abaixo foram rechecados diretamente no código e, quando possível, reproduzidos em execução.

## Pré-condições e evidências gerais

- `trackfw context`: estado de governança disponível e score 100/100.
- `trackfw validate --json`: 0 violações; 172 warnings em modo lenient.
- `go test ./...`: passou.
- `node --test tests/*.test.js`: 897 testes passaram.
- `pytest pypi/tests -q`: 1724 testes passaram e 66 subtests passaram.
- `git diff --check`: passou.
- `make quality`: falhou em `scripts/check-agent-namespace-union.sh`, direção B2 Node. O cenário de falsificação ainda espera que um symlink externo seja seguido; o produto Node agora recusa esse caminho. O resultado é um bloqueio de qualidade do gate, não evidência de que a proteção atual esteja permissiva.

A tentativa de executar o servidor Go em um projeto temporário foi impedida pelo sandbox ao gravar no cache global do Go (`operation not permitted`); a análise do caminho Go abaixo é estática e baseada no handler efetivamente usado por `serve`. A reprodução dinâmica equivalente foi executada no handler Node.

## Modelo de ameaça

O servidor `trackfw serve` é local por padrão, mas aceita exposição explícita em interfaces não loopback e informa que o conteúdo ficará acessível sem autenticação. Um consumidor que controla arquivos dentro do projeto pode criar symlinks, nomes e conteúdo de artefatos. Um atacante de supply chain pode influenciar um release ou o transporte do artefato. Um teste ou gate pode estar errado nas duas direções: deixar passar um defeito ou falhar por uma causa que não mede.

## Achados ordenados por severidade

### H-01 — `/api/file` permite escapar por symlink em Go e Node

**Severidade:** alta quando `serve` é exposto em rede; média em loopback com outro processo local não confiável.

**Evidência:**

- Node: `npm/src/serve/api_file.js:46-55` usa `path.resolve()` e testa apenas o caminho textual contra as raízes autorizadas; `npm/src/serve/api_file.js:61-69` abre esse caminho com `fs.readFileSync()`.
- Go: `internal/serve/api_file.go:32-52` usa `filepath.Clean`/`filepath.Join` e testa o prefixo textual; `internal/serve/api_file.go:62` lê com `os.ReadFile`.
- Python: `pypi/trackfw/serve/api_file.py:11-22` usa `os.path.realpath()` para a raiz e para o arquivo antes da comparação; este runtime contém a defesa correta para o mesmo vetor.

**Reprodução Node:** criei uma raiz temporária com `docs/req/link.md` apontando para `secret.txt` fora da raiz autorizada. Chamando `handleFile()` com `path=.../docs/req/link.md`, a resposta foi `200` e o corpo foi `HADES_SECRET`.

**Impacto:** um arquivo aparentemente autorizado pode ser um link para tokens, arquivos de configuração, material de CI ou qualquer outro arquivo legível pelo processo. Se o operador usar `--host 0.0.0.0`, o endpoint sem autenticação amplia a exposição para qualquer dispositivo alcançável.

**Causa:** a verificação ocorre antes da resolução física do symlink. O código valida o nome que será aberto, enquanto o sistema operacional abre o destino físico.

**Correção recomendada:** canonicalizar as raízes autorizadas e o arquivo solicitado com resolução física antes da comparação; rejeitar qualquer destino cuja canonicalização não esteja sob uma raiz autorizada. Manter a regra de prefixo com separador. Adicionar o mesmo caso de symlink externo aos testes dos três runtimes e ao gate de segurança do servidor, exigindo `403` e ausência do conteúdo secreto.

**Proprietário sugerido:** backend / segurança, com paridade Go, Node e Python.

### H-02 — Instalador baixa release sem verificar o checksum publicado

**Severidade:** alta para a cadeia de distribuição; depende de comprometimento ou substituição do artefato servido.

**Evidência:**

- `.goreleaser.yaml:28-30` configura a geração de `checksums.txt` com SHA-256.
- `scripts/install.sh:115-123` baixa somente o tarball com `curl` ou `wget` e extrai diretamente com `tar`; não baixa `checksums.txt`, não calcula hash e não compara o resultado antes da extração.

**Impacto:** HTTPS autentica o transporte enquanto a conexão e o endpoint permanecem confiáveis, mas o instalador não confirma que o conteúdo recebido corresponde ao release publicado. Uma alteração no artefato, no endpoint ou na cadeia de publicação pode levar o usuário a instalar um binário adulterado.

**Causa:** o checksum é produzido no release, mas não participa do fluxo de instalação.

**Correção recomendada:** baixar o checksum correspondente à mesma tag, validar o nome esperado e executar `sha256sum -c` ou equivalente disponível para a plataforma antes de `tar -xzf`. Falhar fechado quando o checksum estiver ausente, duplicado ou divergente. Testar com um stub de download que troca um byte do tarball e confirmar que nenhum binário é instalado.

**Proprietário sugerido:** release / infraestrutura.

### M-03 — Arquivos estáticos Node seguem symlink criado dentro do diretório embutido de distribuição

**Severidade:** média como defesa em profundidade; baixa no pacote normal, em que os assets são parte do próprio pacote.

**Evidência:** `npm/src/commands/serve.js:141-164` verifica o caminho lexical com `path.resolve()` e depois lê com `fs.readFileSync()`. Diferentemente do Python, não usa `realpath` para o arquivo final. Um symlink criado dentro de `npm/src/serve/static` poderia apontar para fora e ser servido por `/static/...`.

**Impacto:** adulteração local do pacote Node pode transformar um asset permitido em leitura de outro arquivo do sistema. O risco é menor que H-01 porque o diretório normalmente vem do pacote instalado e não do diretório de governança do projeto, mas o controle deveria ser consistente.

**Correção recomendada:** usar `realpath` do arquivo existente e comparar o destino físico com a raiz física antes de ler; adicionar teste de symlink externo.

**Proprietário sugerido:** backend / segurança.

### M-04 — `serve` exposto em rede entrega a cadeia sem autenticação e com CORS universal

**Severidade:** média por desenho; comportamento explicitamente opt-in, mas o aviso não é uma barreira.

**Evidência:**

- `internal/serve/serve.go:152-154` apenas imprime aviso quando o host não é loopback.
- `npm/src/commands/serve.js:211-213` e `pypi/trackfw/commands/serve.py:287-288` fazem o equivalente.
- `npm/src/commands/serve.js` define `Access-Control-Allow-Origin: *` para todas as respostas, e os handlers Go/Python também usam CORS permissivo.
- Os endpoints `/api/board`, `/api/chain`, `/api/metrics`, `/api/file` e `/api/attention` não exigem autenticação.

**Avaliação:** o comportamento está documentado e o padrão é loopback. Não classifico isto como defeito novo oculto sem mudar o contrato: é um residual de exposição que precisa permanecer explícito na documentação e na interface.

**Correção recomendada:** manter loopback como padrão; considerar token efêmero ou autenticação local quando `--host` não for loopback; restringir CORS à origem do próprio servidor ou removê-lo quando não for necessário. Se a decisão for manter a exposição sem autenticação, registrar esse residual no contrato de segurança e adicionar uma confirmação explícita para a opção de rede.

### M-05 — `scripts/check-referential-integrity.sh` fica verde sobre árvore sem REQs

**Severidade:** média como falha de assurance; não é exploração direta do produto.

**Evidência:** `scripts/check-referential-integrity.sh:10-12` itera apenas em `docs/req/*.md`, ignora subpastas e não exige que ao menos um arquivo tenha sido inspecionado. Em uma árvore sem REQs, o loop não executa e o script imprime `Referential integrity OK`.

**Impacto:** remoção acidental da população testada ou mudança para layout namespaced pode produzir falso verde. O gate deixa de medir exatamente quando os arquivos migram para `docs/req/<agent>/<state>/`.

**Correção recomendada:** consumir o mesmo resolvedor de layouts dos três CLIs, ou enumerar explicitamente os layouts suportados; contar arquivos inspecionados; falhar ou emitir estado inconclusivo quando a população esperada estiver vazia. Adicionar falsificação que esvazie a população e exija falha.

**Proprietário sugerido:** qualidade / governança.

## Controles de segurança que resistiram à revisão

- O browser opener Node e Python usa argv em vez de interpolação de shell. O gate `scripts/check-serve-browser-security.sh` contém braço vulnerável, braço corrigido e contra braço legítimo.
- A validação de `--host` rejeita metacaracteres e zonas IPv6; o fluxo valida antes de fazer bind ou abrir o browser.
- O servidor Go serve assets por `embed.FS`, reduzindo a superfície de symlink no runtime Go.
- O `trackfw update` possui rejeição de symlinks nas raízes de integração em Node e Python, e os testes recentes exercitam links externos.
- A filtragem de ambiente para subprocessos Git remove variáveis da família `GIT_ENV_`; não encontrei credenciais literais hardcoded nos fontes revisados.
- Os tokens de Jira/Linear podem ser lidos de `trackfw.yaml`; o próprio código documenta que isso preserva comportamento legado e não representa uma recomendação de armazenamento. O risco operacional é que um usuário comita o arquivo com segredo, mas não encontrei materialização automática adicional neste ciclo.

## Lacunas de verificação

1. A API pública do GitHub não estava acessível durante a auditoria; a revisão usou os issues preservados no histórico, triagens, roadmaps e vault.
2. A reprodução dinâmica do handler Go foi bloqueada pelo sandbox ao iniciar um processo que precisava escrever no cache global do Go. A evidência estática é direta e o código Go usa a mesma classe de validação lexical que a reprodução Node demonstrou vulnerável.
3. A suíte está verde nos três runtimes, mas `make quality` continua bloqueado pela falsificação B2 desatualizada. Isso impede tratar o estado global de qualidade como confiável até o gate ser corrigido.

## Prioridade recomendada

1. Corrigir H-01 nos três runtimes e criar teste de symlink externo no endpoint `/api/file`.
2. Corrigir o instalador para validar o checksum antes da extração.
3. Corrigir o contra braço B2 e o gate referencial para evitar falsos verdes.
4. Fechar a defesa de symlink dos assets Node.
5. Decidir e registrar o modelo de autenticação/CORS de `serve` em rede.

## Conclusão

A revisão Hades encontrou um defeito de leitura fora da raiz autorizado em dois runtimes e uma lacuna de verificação da cadeia de distribuição. O primeiro é reproduzível com um symlink criado no layout de governança e merece correção prioritária. O segundo permite que um release baixado seja instalado sem confirmação criptográfica, apesar de o processo de release já publicar o checksum. Também há dois problemas de assurance: um gate referencial que pode ficar verde sem população e uma falsificação B2 que está desalinhada da proteção atual.

Nenhum código de produto foi alterado nesta auditoria. Os achados devem ser encaminhados para microlotes de correção separados, mantendo a paridade dos três CLIs.
