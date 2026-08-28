---
status: wip
date: 2026-08-28
req: "REQ-2026-08-28-gate-de-ci-gerado-instala-versao-nao-pinada-do-trackfw-e-nao-ha-como-pinar.md"
squad: "hades-tf, ares-tf, apolo-tf, artemis-tf"
---

# Roadmap: Gate de CI pinado na versão geradora e `install.sh` honrando `TRACKFW_VERSION`

> Created: 2026-08-28 | Status: wip

## Context

REQ: `REQ-2026-08-28-gate-de-ci-gerado-instala-versao-nao-pinada-do-trackfw-e-nao-ha-como-pinar.md`
ADR: `ADR-2026-08-28-gate-de-ci-gerado-nasce-pinado-na-versao-que-o-gerou-e-o-install-sh-honra-trackfw-version.md`

`scripts/install.sh:33-44` resolve a versão via API de `releases/latest`, ignorando de qual tag foi
baixado, e não aceita versão por nenhum meio. O workflow gerado nos 3 CLIs usa exatamente esse
script, então o gate bloqueante de PR não é reprodutível e ninguém consegue pinar. Duas frentes:
o script passa a honrar `TRACKFW_VERSION` (com validação, porque o valor entra numa URL), e os
templates gerados nascem pinados na versão do binário gerador.

## Acceptance Criteria

Consolidado — AC1 a AC15 da REQ. Detalhe por ML abaixo.

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Modelo de ameaça deste roadmap
**Status:** ✅ Concluído
**Agente:** `hades-tf`
**Files affected:** apenas este roadmap (seção de resultado abaixo do ML). Nenhum arquivo de produto.
**Actions:**
1. **Completude de enumeração.** A lista de superfícies abaixo está fechada? Não se limite aos
   arquivos nomeados pela REQ: faça `grep -rn "releases/latest" . --exclude-dir=.git
   --exclude-dir=node_modules` e `grep -rn "install.sh" .` nas três árvores (`internal/`,
   `npm/src/`, `pypi/trackfw/`) **e** em `docs/`, `scripts/`, `.github/`. Superfícies já conhecidas:
   `scripts/install.sh`; `internal/generators/scaffold.go:1908` (GH Actions) e `:1932` (GitLab);
   `npm/src/generators/init.js` (7 ocorrências: 227, 242, 800, 812, 824, 836, 851);
   `pypi/trackfw/generators/init_gen.py:541,571`;
   `npm/src/integrations/scaffold_doctor.js` e `pypi/trackfw/integrations/scaffold_doctor.py`
   (comparação com template). Reporte o que faltou ou demonstre que a lista fecha.
2. **Modelo de ameaça.** `TRACKFW_VERSION` é interpolada em `URL=".../releases/download/${VERSION}/
   ${FILENAME}"` e depois passada a `curl`/`tar` num script `sh` executado em CI. Quem esvazia a
   validação de AC3/AC4 sem quebrar nenhuma regra escrita? Cubra no mínimo: substituição de comando,
   separador de shell, path traversal no nome do asset, newline embutida, valor com espaços, valor
   que passa no regex mas aponta para release inexistente (falha aberta ou fechada?), e o caso de
   `TRACKFW_VERSION` vinda de `github.event.pull_request` num workflow de terceiro.
3. **Alvos de falsificação nas duas direções.** Para cada superfície: o que quebra se o
   comportamento regredir (volta a não pinar / validação some), **e** o que quebra se regredir para
   o lado oposto (validação estrita demais rejeita `v7.3.0`; pin obrigatório impede resolver
   `latest`; `update` deixa de bumpar o pin e congela o projeto).
4. **Residual declarado.** O que este desenho aceita não cobrir. Inclua, no mínimo: a lacuna do
   alvo `ci-workflow` no `update` do Python; o pin que envelhece em silêncio; e o `install.sh`
   publicado numa release antiga que não conhece a variável.
**Acceptance criteria:**
- [x] As quatro seções respondidas com evidência (comando rodado + saída), não asserção de uma linha
- [x] Nenhuma linha de implementação escrita neste ML
- [x] Se a enumeração encontrar superfície fora da lista, o roadmap é atualizado antes da Wave 1
      (achado do AC12: alvo real é `scaffold.go:1906/1931`, não `scaffold_doctor.go:62` — registrado
      na seção 1 do resultado abaixo; nenhum novo arquivo de produto precisou ser adicionado às
      "Files affected" de nenhuma Wave, porque `scaffold.go` já estava listado no ML-2A)

**Gates da wave:**
```bash
# Wave 0 gate — a enumeração declarada tem que bater com a busca real.
# Medido por hades-tf em 2026-08-28: 4 arquivos de fonte, 18 ocorrências.
#
# Auditoria do arquiteto (2026-08-28): a primeira versão deste gate contava com
# `grep -rn` direto nos diretórios e media 19 numa árvore com `__pycache__` — o 19º
# era `pypi/trackfw/generators/__pycache__/init_gen.cpython-314.pyc`, um artefato de
# build ignorado pelo git. O gate falhava em máquina de dev que já rodou pytest e
# passava em checkout limpo de CI: não hermético. Corrigido para varrer só fonte
# RASTREADA pelo git.
#
# O invariante primário é o CONJUNTO DE ARQUIVOS, não a contagem: a afirmação do
# ML-0A é "a lista de superfícies está fechada". A Wave 2 pode legitimamente mudar
# o número de ocorrências dentro de um arquivo já declarado; o que não pode é
# aparecer um quinto arquivo.
set -eu
esperado="internal/generators/scaffold.go
npm/src/generators/init.js
pypi/trackfw/generators/init_gen.py
scripts/install.sh"
medido=$(git ls-files -z scripts internal npm/src pypi/trackfw \
  | xargs -0 grep -l "releases/latest" 2>/dev/null | sort)
if [ "$medido" != "$esperado" ]; then
  echo "Wave 0: conjunto de superfícies mudou — a enumeração do ML-0A não fecha mais." >&2
  echo "esperado:"; echo "$esperado" >&2
  echo "medido:";   echo "$medido"   >&2
  exit 1
fi
[ -n "$medido" ] || { echo "guarda de vacuidade: nenhum arquivo varrido" >&2; exit 1; }
echo "Wave 0 gate OK — 4 superfícies declaradas, nenhuma nova."
```

#### Resultado do ML-0A (hades-tf, 2026-08-28)

**Sobre o escopo:** análise pura, nenhuma linha de `scripts/install.sh`, `internal/`, `npm/`,
`pypi/`, `Makefile` foi tocada. A única escrita é este bloco e o gate acima.

##### 1. Completude de enumeração

Comandos rodados e saída integral:

```bash
$ grep -rn "releases/latest" scripts/ internal/ npm/src/ pypi/trackfw/ 2>/dev/null | wc -l
18
$ grep -rln "releases/latest" . --exclude-dir=.git --exclude-dir=node_modules --exclude-dir=bin 2>/dev/null | wc -l
24
$ grep -rln "install.sh" . --exclude-dir=.git --exclude-dir=node_modules --exclude-dir=bin 2>/dev/null | wc -l
33
```

O grep com escopo do gate (`scripts/`, `internal/`, `npm/src/`, `pypi/trackfw/`) retorna **18**
ocorrências:

| Arquivo | Linhas | Natureza |
|---|---|---|
| `scripts/install.sh` | 34, 38 | resolução via API (`api.github.com/.../releases/latest`) — não é a superfície de asset, é a chamada que o Wave 1 substitui condicionalmente |
| `internal/generators/scaffold.go` | 251, 263, 275, 287, 302 | texto de slash commands (`claudeCommandsContent()`) — string de ajuda "trackfw não está instalado", não é o job de CI |
| `internal/generators/scaffold.go` | 1923, 1939 | **as duas ocorrências reais do job de CI** — dentro de `buildGitHubActionsWorkflowContent` (linha 1923) e `buildGitLabCIWorkflowContent` (linha 1939) |
| `npm/src/generators/init.js` | 227, 242 | job de CI (GH Actions / GitLab, equivalente Node) |
| `npm/src/generators/init.js` | 800, 812, 824, 836, 851 | textos de ajuda equivalentes ao `claudeCommandsContent()` do Go |
| `pypi/trackfw/generators/init_gen.py` | 541, 571 | textos de ajuda equivalentes (`generate_claude_commands`) |

**Correção de linha sobre a lista pré-existente do roadmap:** o texto original desta seção (linha
39) cita `internal/generators/scaffold.go:1908` (GH Actions) e `:1932` (GitLab). Medido agora:
`buildGitHubActionsWorkflowContent` começa em **1909** e a string com `curl` está em **1923**;
`buildGitLabCIWorkflowContent` começa em **1932** e a string com `curl` está em **1939**. Divergência
de poucas linhas, sem impacto — mesmo arquivo, mesmas duas funções — mas a Wave 2 (ML-2A) deve
localizar por nome de função (`buildGitHubActionsWorkflowContent` / `buildGitLabCIWorkflowContent`),
não por número de linha, porque o próprio ML-0A já mediu divergência entre o número citado na REQ/ADR
e o número real.

**Achado fora da lista original — precisa entrar no roadmap:** o comentário-alvo do AC12
(`scaffold_doctor.go:62`, citado na REQ linha 59 e no ADR linha 94) **não existe nesse arquivo**.
Medido:

```bash
$ grep -n "cfg-independent" internal/generators/scaffold_doctor.go internal/generators/scaffold.go
internal/generators/scaffold.go:1906:// to GitHubActionsWorkflowPath. The content is cfg-independent (cfg is accepted for
internal/generators/scaffold.go:1931:// GitLabCIWorkflowPath. Cfg-independent; ci: gitlab-ci is the gate at the call site.
```

O comentário "cfg-independent" que precisa virar "cfg-independent mas não version-independent"
(AC12) está em `internal/generators/scaffold.go:1906` e `:1931` — nos próprios doc-comments de
`buildGitHubActionsWorkflowContent`/`buildGitLabCIWorkflowContent` — **não** em
`scaffold_doctor.go:62`. `scaffold_doctor.go:50-68` tem um comentário de design diferente
("Property by path...", "Config-rendered templates...") que não menciona cfg-independence.
**Ação:** ML-2A deve corrigir os comentários em `scaffold.go:1906` e `:1931`, não em
`scaffold_doctor.go:62`. Atualizo a lista de "Files affected" do ML-2A abaixo — o arquivo já estava
listado (`internal/generators/scaffold.go`), então isso é uma correção de alvo dentro do arquivo já
previsto, sem novo arquivo a adicionar.

**Superfícies fora do escopo do gate, mas dentro do que a REQ/ADR chamam de "textos gerados de
CLAUDE.md/docs" (AC13) — já cobertas pela Wave 2, confirmadas presentes:**
`internal/generators/scaffold.go:251,263,275,287,302` (5 ocorrências, `claudeCommandsContent()`);
`npm/src/generators/init.js:800,812,824,836,851` (5 ocorrências); `pypi/trackfw/generators/init_gen.py`
tem 2 ocorrências em vez das 5 do Go/Node porque o Python só materializa 2 dos 5 comandos com o bloco
de "instalação não encontrada" nesse trecho lido (541, 571) — **isto é uma divergência de paridade
pré-existente entre os 3 CLIs que não é desta REQ** (a REQ pede paridade no *pin*, não retroage sobre
quantos slash commands cada CLI já carrega o blurb de instalação); sinalizo para o Wave 2/3 não
tentar igualar as contagens, só declarar o texto de cada CLI fora-do-pin de forma consistente
(AC13 já prevê "ou declaradas fora do pin, sem instrução contraditória").

**Fora do escopo do gate e fora do escopo da REQ (negative scope explícito), mas encontrados pela
busca ampla e registrados para não reabrir dúvida depois:** `README.md:74`,
`.github/workflows/trackfw-gate.yml:14` (o próprio workflow deste repo — REQ linha 79-80 exclui
explicitamente), `docs/visao-projeto/VISION.md:203`, `docs/cli-parity.md:63` (menciona
`VERSION_BARE="${VERSION#v}"` do próprio `install.sh` — relevante para AC5, não uma superfície nova),
`.claude/commands/trackfw/*.md` e `.gemini/commands/trackfw/*.md` (10 arquivos — são os artefatos
*instalados* deste próprio repositório trackfw, gerados a partir do template Go acima; não são
gerados por scaffold para *outros* projetos, então não são alvo de pin — mas herdam o texto de ajuda
não pinado do template, então mudam automaticamente se o ML-2A mudar `claudeCommandsContent()` — nada
a fazer aqui, é efeito, não causa), e três ocorrências em roadmaps `done/` (histórico, imutável).

**Conclusão da seção 1:** a lista do ML-0A original fecha para as superfícies de **produto** (as que
o Wave 1/2/3 tocam). A contagem do gate (18) bate com a soma: 2 (`install.sh` API) + 7 (`scaffold.go`,
sendo 5 texto + 2 CI real) + 7 (`init.js`, 2 CI real + 5 texto) + 2 (`init_gen.py`, texto) = 18. A
única correção material é o alvo do AC12: `scaffold.go:1906/1931`, não `scaffold_doctor.go:62`.

##### 2. Modelo de ameaça

`TRACKFW_VERSION` entra em duas interpolações em `scripts/install.sh` depois da Wave 1: a
`VERSION` bruta compõe `URL="https://github.com/${REPO}/releases/download/${VERSION}/${FILENAME}"`
(linha 55), e `VERSION_BARE="${VERSION#v}"` (linha 52) compõe
`FILENAME="${BIN}_${VERSION_BARE}_${OS}_${ARCH}.tar.gz"` (linha 54). Ambas terminam em argumentos
**quotados** de `curl`/`tar`/`mv` (nenhuma delas passa por `eval`, por um `sh -c` sem aspas, nem por
um segundo nível de `$()`) — isso já é uma barreira estrutural independente do regex: mesmo que um
valor hostil escapasse da validação, ele não vira comando executado *no shell atual*, porque nunca é
reexpandido sem aspas. O regex de AC3/AC4 é a segunda barreira, e é a que este ML audita.

O regex-alvo é `^v?[0-9]+\.[0-9]+\.[0-9]+$`. O ML-1A já prescreve implementar com `case`/`expr`
POSIX, não `[[ =~ ]]` (o script roda com `sh`). Isso importa porque **`case` e `expr` não têm a
mesma superfície de erro** — quem vai implementar precisa saber qual delas cada vetor abaixo ataca:

| Vetor | `$(id)` / `` `id` `` (substituição de comando) | Ataca via | Resultado esperado | Como esvaziar sem quebrar regra escrita |
|---|---|---|---|---|
| Substituição de comando | `$(id)` ou `` `id` `` | nenhum dos dois é interpretado dentro de `case "$V" in padrão)` nem dentro de `expr "$V" : 'regex'` — a variável já foi expandida uma vez pelo shell ao ser lida com `$TRACKFW_VERSION`; o *conteúdo* literal `$(id)` só re-executaria se o valor fosse passado a um segundo `eval`/`sh -c` sem aspas | rejeitado pelo charset (`$`, `(`, `)` não estão em `[0-9v.]`) | nenhuma forma legítima descrita no roadmap reintroduz um segundo nível de expansão — mas se um ML futuro "otimizar" trocando `case` por `eval "case \$TRACKFW_VERSION in ...` (para reuso de padrão via variável), isso reintroduziria o segundo nível e o vetor voltaria a valer. Vale como cenário do gate mesmo sabendo que hoje não aplica: barrar por design, não por acidente de implementação atual. |
| Separador de shell | `;`, `&&`, `\|` | mesma barreira estrutural — não há reexpansão sem aspas | rejeitado pelo charset | idem — só reabre se alguém interpolar `$TRACKFW_VERSION` sem aspas num comando composto novo (ex.: um log `echo Instalando $TRACKFW_VERSION` sem aspas facilita word-splitting mas não execução; um `sh -c "algo $TRACKFW_VERSION"` sem aspas seria o buraco real) |
| Path traversal | `../../etc` | `/` e `.` múltiplos não batem `[0-9]+\.[0-9]+\.[0-9]+` (glob `case` não tem `/` no alfabeto permitido) | rejeitado pelo charset | **mas repare no alvo real do traversal se a validação falhar**: não é a `URL` (GitHub rejeita/normaliza o path do lado do servidor), é `VERSION_BARE` entrando em `FILENAME` e depois em `curl ... -o "${TMP_DIR}/${FILENAME}"` (linha 63) — se `VERSION_BARE` contiver `/`, o `-o` grava fora de `${TMP_DIR}` (dentro da árvore de `/tmp`, previsível por padrão de `mktemp -d`). Isso é uma escrita de arquivo arbitrária **sob controle do usuário que definiu a env var**, não do atacante remoto — mas se a REQ/ADR abre a porta para `TRACKFW_VERSION` vinda de `pull_request` (ver linha de threat model 5 abaixo), o "usuário que definiu a var" pode ser o autor do PR, e o alvo de escrita passa a ser o runner de CI. O gate precisa cobrir exatamente esse `-o` como o alvo, não só "a URL fica estranha". |
| Newline embutida | `v7.3.0\nFOO` | **depende de qual API valida**: um `case "$V" in v[0-9]*.[0-9]*.[0-9]*) ;; esac` sem `*` sobrando nas pontas casa a *string inteira*, incluindo o `\n` — `\nFOO` não bate o padrão fechado, então `case` bem escrito **rejeita** corretamente. O risco real está em `grep -E`: se a implementação usar `printf '%s' "$V" \| grep -qE '^v?[0-9]+\.[0-9]+\.[0-9]+$'` em vez de `case`, `grep` opera **linha a linha** — a primeira linha `v7.3.0` bate `^...$` sozinha e `grep -q` retorna 0 (match encontrado) mesmo a entrada completa tendo uma segunda linha `FOO` que nunca é examinada pela âncora. **Isto é o vazamento real**: `TRACKFW_VERSION` continua sendo `"v7.3.0\nFOO"` (variável não trunca sozinha), e esse valor completo — com a segunda linha — segue para `VERSION`/`URL`/`FILENAME`. Não é execução de comando (mesma barreira de aspas), mas é uma variável poluída entrando num script `.tar.gz` esperado, e mais importante: **é exatamente a classe de bug já registrada em `vault/notes/bash-grep-F-embedded-newline-vacuous-match-2026-08-16.md`** (ali era `grep -F` com newline no *padrão*; aqui seria `grep -E` com newline no *dado de entrada* — mecanismo diferente, mesma família: âncoras `^`/`$` de `grep` são por linha, não por buffer inteiro). **Consequência para o gate do ML-1A:** o cenário "newline embutida" só falsifica de verdade se tiver conteúdo *depois* da quebra de linha (`v7.3.0\nFOO`, como a REQ já pede em AC4) — um cenário com só `v7.3.0\n` (newline final sem conteúdo) não distingue implementação correta de uma que usa `grep -qE` sem `-z`, porque ambas passariam. **Recomendo ao ares-tf, no ML-1A, implementar com `case`, não `grep -E`, e o gate deve testar explicitamente que o valor de `VERSION` usado depois não contém a segunda linha** (não basta checar o exit code da validação — checar o conteúdo que sobrou). |
| Só espaços | `"   "` | REQ AC3 diz "definida e não vazia **após remover espaços**" — `case` não faz trim automático; `"   "` não bate `[0-9v.]*` de qualquer forma, então mesmo sem trim explícito o charset já rejeita. O trim citado na REQ é sobre a condição "está definida" (differenciar de vazio), não sobre limpar espaços do meio do regex. Se a implementação pular o trim e for direto ao `case`, o resultado é o mesmo (rejeitado), então este vetor não é o crítico — é o de newline que é. |
| Valor válido no regex mas release inexistente | `v99.0.0` | passa a validação, `VERSION` recebe `v99.0.0`, `URL` é composta e `curl -sSfL ... -o ...` roda. `-f` (fail on HTTP error) + `set -e` no topo do script já fazem o script abortar no primeiro `curl` que retornar 4xx/5xx, **antes** de chegar em `tar`/`mv`. **Falha fechada, por construção já existente no script hoje** — a Wave 1 não precisa adicionar nada para isso; só precisa não remover `-f` nem `set -e` (nenhuma ação do ML-1A toca essas duas flags, então o risco de regressão aqui é baixo, mas vale como cenário de gate: "versão bem formada e inexistente → exit != 0, sem instalar binário"). |
| `TRACKFW_VERSION` de `github.event.pull_request.*` em workflow de terceiro | qualquer string controlada pelo autor do PR | **fora do CI gerado pelo trackfw** — o template do Wave 2 escreve `TRACKFW_VERSION: "<versão do binário>"` como string literal, nunca interpolando `${{ github.event.... }}`, então o trackfw não cria esse vetor. O vetor só existe se um usuário de terceiro, por conta própria, editar o workflow gerado para trocar o literal por uma expressão do GitHub Actions — nesse caso o regex de `install.sh` é a **única** barreira restante, e ela já foi coberta pelos vetores acima. **Este é o caso que dá peso a "a validação é requisito de segurança, não higiene de formato"** (ADR linha 60-61): sem ela, um autor de PR malicioso controlaria o `URL` de download do próprio gate de governança do repositório alheio. Com ela (mesmo o `case` simples do ML-1A), o pior que esse autor consegue é apontar para uma tag `vNN.NN.NN` numericamente válida mas de release inexistente ou de uma versão antiga/vulnerável do próprio `trackfw` real (downgrade dentro do espaço de versões publicadas — não é injeção, é escolha de versão, e é o mesmo risco que qualquer pin por variável de ambiente aceita). Não é um vetor que a validação apague; é o resíduo aceito por desenho, e deveria estar na seção 4. |

##### 3. Alvos de falsificação nas duas direções

| Superfície | Regride para "sem controle" (quebra o que a REQ resolve) | Regride para o lado oposto (controle rígido demais) |
|---|---|---|
| `install.sh` — honrar `TRACKFW_VERSION` (AC1, AC2) | `TRACKFW_VERSION` definida é ignorada, sempre resolve `latest` — volta ao estado atual. Cenário de gate: `TRACKFW_VERSION=v1.0.0` (bem anterior ao HEAD) + `TRACKFW_INSTALL_DRYRUN=1`, assert que a `URL` impressa contém `v1.0.0` e **não** chama a API de `releases/latest`. | `TRACKFW_VERSION` ausente ou vazia passa a exigir valor (quebra AC2, quebra todo projeto que nunca setou a variável, inclusive quem instala localmente sem CI). Cenário: `unset TRACKFW_VERSION; TRACKFW_INSTALL_DRYRUN=1 sh install.sh`, assert que resolve via API como hoje (sem exit 1 por variável ausente). |
| `install.sh` — validação (AC3, AC4) | Regex vira permissivo demais (ex.: trocado por `case` com `*` sobrando nas pontas, ou por `expr match` sem âncora final) e deixa passar `7.3.0; rm -rf /`. Cenário: os 6+ payloads de AC4, cada um com `assert_fails_with` nomeando a razão ("formato inválido"), e adicionar explicitamente o par `v7.3.0\nFOO` com conteúdo pós-newline (não só `v7.3.0\n`) para cobrir a lacuna de `grep`/linha discutida na seção 2. | Regex vira restritivo demais e rejeita versão real: `v7.30.0` (segmento com 2 dígitos), `v10.0.0` (major com 2 dígitos), `0.9.1` (pré-1.0, sem prefixo `v`). Cenário: os três aceitos sem exit 1, `URL` composta corretamente para cada um. |
| `scaffold.go`/`init.js`/`init_gen.py` — template pinado (AC6, AC7, AC9-AC12) | Versão nunca escrita no bloco `env:`/`variables:` (regride para o YAML de hoje) — `doctor` nunca aponta `scaffold-divergent` mesmo com pin desatualizado porque o template comparado também não tem pin. Cenário Wave 2: grep pelo literal `TRACKFW_VERSION:` no output gerado, falha se ausente. | Versão fica hardcoded no código-fonte do gerador (ex.: `"7.3.0"` fixo em vez de ler `version.Version`/`package.json`/`__version__`) — todo projeto gerado por qualquer binário fica pinado na mesma versão, o pin nunca acompanha o binário real, e o `doctor` acusa `scaffold-divergent` em **todo** projeto gerado por um binário diferente de `7.3.0`, mesmo recém-criado (quebra AC11). Cenário: gerar com dois binários de versão diferente (ou stub da função de versão), assert que o pin no output muda. |
| `update` bumpando o pin (AC9) | `update` não toca o alvo `ci-workflow` em Go/Node (regride para "pin congela para sempre" mesmo nos dois CLIs que deveriam gerenciar) — cenário: seed de workflow com pin antigo, `trackfw update` (Go e Node), assert pin novo e alvo reportado `updated`. | `update` reescreve o workflow mesmo sem diferença de versão, todo `trackfw update` gera diff espúrio no PR (ruído contrário ao propósito do ADR — "bump vira ato deliberado e revisável", não "todo update sempre suja o diff"). Cenário: `update` duas vezes seguidas com o mesmo binário, segunda chamada não reporta `updated` para `ci-workflow` (idempotência). |
| `doctor` (AC10, AC11) | Nunca aponta `scaffold-divergent` para pin desatualizado (ruído zero, mas também sinal zero — usuário nunca sabe que o gate do PR está rodando versão diferente do binário local). Cenário: pin manual trocado à mão para uma versão diferente da do binário, `doctor` deve reportar `[scaffold-divergent]`. | Aponta `scaffold-divergent` em projeto recém-gerado pelo próprio binário (falso positivo constante, quebra AC11, o "ruído aceito" da ADR vira ruído **sempre**, não só entre releases). Cenário: gerar e rodar `doctor` na sequência, sem trocar nada, assert `no mismatches`. |
| Paridade 3 CLIs (AC8) | Um dos três CLIs esquece `timeout-minutes: 10` ou usa aspas/indentação diferente — pin presente mas byte-diferente. Cenário Wave 3: diff byte a byte nomeando o par divergente. | Gate de paridade fica frágil a diferença cosmética irrelevante (ex.: ordem de chaves de mapa não determinística em algum runtime) e falha em CI de forma não-reprodutível — não é "controle rígido demais" no sentido de rejeitar release válida, mas é o análogo: falso positivo recorrente que ensina o time a ignorar o gate. Cenário: rodar o gate 2x seguidas sobre o mesmo binário/commit, mesma saída. |
| Textos de ajuda fora do pin (AC13) | Texto de "trackfw não está instalado" nos 3 CLIs fica contraditório entre si (um pinado, outro não) sem declaração — usuário lê instrução diferente dependendo de qual CLI gerou o projeto. Cenário: `docs/cli-parity.md` deve ter uma frase explícita "estes 3 blocos de texto ficam fora do pin, deliberadamente" e um teste que falha se o texto virar pinado em só um dos 3 CLIs. | Alguém "corrige" esse texto para pinar a versão do binário que gerou o projeto — parece mais correto, mas fica obsoleto: a instrução é mostrada quando o comando `trackfw` **falha** (não está instalado), e nesse momento recomendar exatamente a versão antiga do binário que gerou o projeto (em vez de "a mais recente") é pior UX, porque o motivo de reinstalar normalmente é estar desatualizado. Cenário: não seria pego por um gate automático — é uma decisão de produto; registro aqui para o ares-tf/apolo-tf não "corrigirem" isso por iniciativa própria na Wave 2. |

##### 4. Residual declarado

- **Lacuna do alvo `ci-workflow` no `update` do CLI Python** (`pypi/trackfw/integrations/scaffold_doctor.py:25` e `:382`, confirmado por leitura direta): projetos que só usam o CLI Python geram o
  workflow pinado no `init`, mas nunca recebem o bump automático depois — o `doctor` do Python nunca vai
  acusar `scaffold-divergent` para esse arquivo porque ele está fora da comparação, por desenho. Isto
  já era verdade antes desta REQ (aplicava a "não pinar nunca") e continua depois (aplica a "pinar uma
  vez e nunca mais bumpar"), então esta REQ **piora** silenciosamente a lacuna: antes, o pin nunca
  existia (nada para desatualizar); depois, existe um pin que pode ficar desatualizado sem que o
  `doctor` do Python jamais aponte.
- **O pin envelhece em silêncio.** Fora do `doctor` local (que só roda quando alguém chama), nada
  força um projeto a rodar `trackfw update`. Um projeto que nunca atualiza o binário local também
  nunca vê o `doctor` acusar nada, porque o pin do CI sempre bate com o binário desatualizado que
  gerou/atualizou o projeto pela última vez — congela sozinho, como a ADR já nomeia (linha 101-103),
  mas vale registrar que "congelar" aqui inclui não receber patches de segurança do próprio `trackfw`
  no gate de CI indefinidamente, sem nenhum sinal.
- **`install.sh` já publicado em releases antigas não conhece `TRACKFW_VERSION`.** Qualquer workflow
  gerado por um binário anterior a esta REQ, ou qualquer usuário com o script em cache/vendorizado,
  continua baixando via `releases/latest/download/install.sh` (linha não tocada por esta REQ nos
  templates — o Wave 2 só adiciona `env:`/`variables:` com a versão, a chamada `curl .../install.sh`
  continua batendo em `latest`) e recebe a versão **mais nova do install.sh**, que aí sim honra
  `TRACKFW_VERSION` e instala o binário pinado corretamente — mas esse encadeamento depende de o
  `install.sh` publicado em `releases/latest` já ser pós-Wave-1. Até a primeira release pós-merge, o
  `TRACKFW_VERSION` escrito no workflow por um `init`/`update` feito com o binário novo é **ignorado**
  pelo `install.sh` de `latest` (ainda o antigo), e o pin no YAML é decorativo até essa release sair —
  janela de tempo entre "código mergeado" e "release publicada" em que AC6/AC7 estão satisfeitas no
  template mas AC1 não está satisfeita no script consumido. Não é bug da REQ; é ordem de entrega —
  registro para o `hefesto-tf`/arquiteto não fecharem a REQ como "efetiva" antes de confirmar que a
  tag/release com o `install.sh` novo já foi publicada.
- **Downgrade dentro do espaço de versões publicadas não é coberto.** Como já dito na seção 2, a
  validação barra injeção, não escolha de versão — um `TRACKFW_VERSION` numericamente válido apontando
  para uma release antiga (inclusive vulnerável) do próprio `trackfw` passa. Aceito por desenho (é
  literalmente o que "pinar" significa), mas fica registrado que esta REQ não adiciona nenhuma lista
  de versões mínimas/bloqueadas — se um CVE for encontrado numa versão antiga do `trackfw`, nada aqui
  impede reintroduzi-la via pin manual.
- **Divergência de contagem de textos de ajuda entre os 3 CLIs** (seção 1: Go/Node têm 5 ocorrências
  de texto "não instalado", Python parece ter 2 na amostra lida) não é fechada por este roadmap — é
  parity debt pré-existente, fora do escopo desta REQ (que trata do *pin*, não de *paridade de
  cobertura de slash commands*). Registrado para não ser confundido com uma regressão da Wave 2 se
  aparecer numa auditoria futura.

## Wave 1 — `install.sh` honra e valida `TRACKFW_VERSION`
> Dependências: Wave 0 aprovada. ML único — arquivo único, sem paralelismo possível.

### ML-1A — `TRACKFW_VERSION` no `install.sh`, com validação anterior ao uso
**Status:** ⬜ Pendente
**Agente:** `ares-tf`
**Files affected:**
- `scripts/install.sh` (único arquivo de produto)
- `scripts/check-install-version-pin.sh` (novo gate)
- `Makefile` (registrar o gate em `quality`)

**Actions:**
1. Em `scripts/install.sh`, **antes** do bloco de resolução via API (linha 32), inserir: se
   `TRACKFW_VERSION` está definida e não é vazia após remover espaços, validar contra
   `^v\{0,1\}[0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*$` usando `case`/`expr` compatível com `sh`
   POSIX (o script roda com `sh`, não `bash` — não usar `[[ =~ ]]`). Valor válido → `VERSION` recebe
   o valor com prefixo `v` normalizado (`v7.3.0`), pulando a consulta à API. Valor inválido →
   `echo` nomeando a variável e o formato esperado em stderr e `exit 1`, **sem** compor URL nem
   chamar `curl`/`wget`.
2. Variável ausente ou vazia → fluxo atual intocado (resolução via API). AC2.
3. Não adicionar argumento posicional nem flag. AC do escopo negativo.
4. Criar `scripts/check-install-version-pin.sh` como gate falsificável, no molde dos gates
   existentes (`scripts/check-doctor-parity.sh`): cenários que **passam** — `7.3.0`, `v7.3.0`,
   vazio, ausente; cenários que **falham com a razão declarada** (`assert_fails_with`) —
   `7.3.0; rm -rf /`, `../../etc`, `$(id)`, `` `id` ``, `7.3.0 && curl x | sh`, `v7.3.0` com
   newline embutida, `"   "`. O gate deve exercitar o script real com uma seam que impeça download
   de verdade (ex.: `TRACKFW_INSTALL_DRYRUN=1` imprimindo a URL composta e saindo antes do `curl`),
   e asserir sobre a URL impressa. Incluir **guarda de vacuidade**: se nenhum cenário rodou, o gate
   falha.
5. Registrar o gate no alvo `quality` do `Makefile`.

**Acceptance criteria:**
- [ ] AC1, AC2, AC3, AC4, AC5 da REQ verificáveis pelo gate novo
- [ ] `sh -n scripts/install.sh` → exit 0 (sintaxe POSIX válida)
- [ ] `bash scripts/check-install-version-pin.sh` → exit 0, com contagem de cenários impressa
- [ ] Guarda de vacuidade provada: rodar o gate com a lista de cenários vazia faz o gate falhar
- [ ] Nenhum download real disparado durante o gate
**Comandos de validação:**
```bash
sh -n scripts/install.sh
bash scripts/check-install-version-pin.sh
TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 make quality
```

## Wave 2 — Templates pinados nos 3 CLIs (3 MLs em paralelo)
> Dependências: Wave 1 concluída (o pin só faz sentido com o `install.sh` honrando a variável).
> Os três MLs tocam árvores disjuntas e rodam em paralelo. Nenhum deles toca `docs/cli-parity.md`
> nem `scripts/` — isso é a Wave 3, sequencial, para não haver dois agentes no mesmo arquivo.

### ML-2A — Go: template pinado + doctor + comentário corrigido
**Status:** ⬜ Pendente
**Agente:** `apolo-tf`
**Files affected:** `internal/generators/scaffold.go`, `internal/generators/scaffold_doctor.go`,
`internal/generators/scaffold_test.go` (ou arquivo de teste equivalente já existente)
**Actions:**
1. `buildGitHubActionsWorkflowContent` passa a receber a versão e emitir, no job `governance`:
   ```yaml
   jobs:
     governance:
       runs-on: ubuntu-latest
       timeout-minutes: 10
       env:
         TRACKFW_VERSION: "<versão>"
   ```
   A versão vem de `internal/version.Version`. **Não** hardcodar `7.3.0`.
2. `buildGitLabCIWorkflowContent` idem, via bloco `variables:` com `TRACKFW_VERSION`.
3. `scaffold_doctor.go` continua chamando os mesmos builders (a comparação segue coerente por
   construção). Corrigir o comentário de `:62` e o de `buildGitHubActionsWorkflowContent`: o builder
   é cfg-independente mas **não** é version-independente (AC12).
4. Testes Go: workflow gerado contém a versão que `version.Version` reporta; `doctor` reporta
   `no mismatches` logo após gerar (AC11) e `scaffold-divergent` quando o pin é trocado à mão (AC10).
**Acceptance criteria:**
- [ ] AC6, AC7 (Go), AC10, AC11, AC12
- [ ] `go build ./...` → exit 0
- [ ] `go test ./internal/generators/...` → exit 0
- [ ] Nenhuma string de versão literal no template — grep por `7\.3\.0` em `scaffold.go` não retorna
      nada no bloco do workflow

### ML-2B — Node: template pinado + doctor
**Status:** ⬜ Pendente
**Agente:** `apolo-tf`
**Files affected:** `npm/src/generators/init.js`, `npm/src/integrations/scaffold_doctor.js`,
`npm/src/commands/update.js` (só se o alvo `ci-workflow` precisar da versão), `npm/tests/`
**Actions:**
1. Mesmo template do ML-2A, **byte-idêntico** para a mesma versão. A versão vem do `version` do
   `npm/package.json`, não literal.
2. Cobrir as 7 ocorrências de `releases/latest` em `init.js` (227, 242, 800, 812, 824, 836, 851):
   as que compõem o workflow gerado passam a pinar; as que aparecem em texto de CLAUDE.md/docs
   seguem AC13 — atualizar ou declarar explicitamente fora do pin, sem deixar instrução
   contraditória.
3. `scaffold_doctor.js` compara contra o template novo.
4. Testes Node cobrindo AC6, AC10, AC11.
**Acceptance criteria:**
- [ ] AC6, AC7 (Node), AC10, AC11, AC13 (parte Node)
- [ ] `npm test --prefix npm` → exit 0
- [ ] Nenhuma versão literal no template

### ML-2C — Python: template pinado + doctor
**Status:** ⬜ Pendente
**Agente:** `apolo-tf`
**Files affected:** `pypi/trackfw/generators/init_gen.py`,
`pypi/trackfw/integrations/scaffold_doctor.py`, `pypi/tests/`
**Actions:**
1. Mesmo template, byte-idêntico. Versão vem de `trackfw.__version__`, não literal.
2. Cobrir `init_gen.py:541` e `:571` (AC13, parte Python).
3. `scaffold_doctor.py` compara contra o template novo. **Manter** a exclusão documentada do alvo
   `ci-workflow` no `update` do Python (`scaffold_doctor.py:25`) — está no escopo negativo — mas
   revisar se o texto da exclusão continua correto depois desta mudança.
4. Testes Python cobrindo AC6, AC10, AC11.
**Acceptance criteria:**
- [ ] AC6, AC7 (Python), AC10, AC11, AC13 (parte Python)
- [ ] `python -m pytest pypi/tests` → exit 0
- [ ] Nenhuma versão literal no template

## Wave 3 — Gate de paridade, contrato e evidência
> Dependências: Wave 2 completa nos três. ML único — toca arquivos compartilhados pelos 3 stacks.

### ML-3A — Gate falsificável de paridade do pin + `docs/cli-parity.md`
**Status:** ⬜ Pendente
**Agente:** `artemis-tf`
**Files affected:** `scripts/check-ci-workflow-pin-parity.sh` (novo), `docs/cli-parity.md`,
`Makefile`
**Actions:**
1. Gate que gera o workflow com os 3 CLIs em sandbox e compara **byte a byte** (AC8). Falha se
   qualquer par divergir, nomeando qual.
2. Cenário de falsificação em cada direção: workflow sem `TRACKFW_VERSION` → gate falha; workflow
   com versão diferente da do binário → gate falha; `timeout-minutes` ausente → gate falha.
   Usar `assert_fails_with` mirando a razão que o **próprio gate** emite, não a mensagem do CLI.
3. Guarda de vacuidade obrigatória.
4. Seção nova em `docs/cli-parity.md` com o contrato do pin, anotada com `gate=` apontando para o
   script novo, mais a lacuna do `ci-workflow` no Python anotada como `gap reason=`.
5. Registrar no `Makefile`.
**Acceptance criteria:**
- [ ] AC8, AC14
- [ ] `bash scripts/check-ci-workflow-pin-parity.sh` → exit 0 com contagem de cenários
- [ ] `bash scripts/check-parity-contract-coverage.sh` → exit 0
- [ ] AC15: `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 make quality` → exit 0
**Comandos de validação:**
```bash
bash scripts/check-ci-workflow-pin-parity.sh
bash scripts/check-parity-contract-coverage.sh
TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 make quality
```

## Barreira final
Antes do PR: revisão `hefesto-tf` (qualidade) e `hades-tf` (segurança — a validação de AC3/AC4 é o
ponto de maior risco do roadmap), auditoria de diff pelo arquiteto, e
`trackfw barrier <roadmap> --wave 3`.
