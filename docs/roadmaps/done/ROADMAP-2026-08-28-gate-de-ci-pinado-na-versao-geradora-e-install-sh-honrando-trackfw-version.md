---
status: done
date: 2026-08-28
req: "docs/req/REQ-2026-08-28-gate-de-ci-gerado-instala-versao-nao-pinada-do-trackfw-e-nao-ha-como-pinar.md"
squad: "hades-tf, ares-tf, apolo-tf, artemis-tf"
---

# Roadmap: Gate de CI pinado na versão geradora e `install.sh` honrando `TRACKFW_VERSION`

> Created: 2026-08-28 | Status: done

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
**Critérios de aceite:**
- [x] As quatro seções respondidas com evidência (comando rodado + saída), não asserção de uma linha
- [x] Nenhuma linha de implementação escrita neste ML
- [x] Se a enumeração encontrar superfície fora da lista, o roadmap é atualizado antes da Wave 1
      (achado do AC12: alvo real é `scaffold.go:1906/1931`, não `scaffold_doctor.go:62` — registrado
      na seção 1 do resultado abaixo; nenhum novo arquivo de produto precisou ser adicionado às
      "Files affected" de nenhuma Wave, porque `scaffold.go` já estava listado no ML-2A)

**Gates da wave:**
```bash
# Wave 0 gate — nenhum mecanismo de instalação não pinado sobra em código de produto.
#
# CORREÇÃO 3 (arquiteto, 2026-08-28, após ML-2D/2E/2F): o invariante "conjunto de
# arquivos que contêm o padrão" virou ruído. Depois que os MLs corretivos pinaram
# tudo, os arquivos que ainda casam com "@latest" casam por CITAREM a string em
# comentário ou em asserção negativa de teste — prosa, não superfície. Um gate que
# não distingue as duas coisas volta a ser inerte.
#
# O invariante certo é DIRETO: zero ocorrência do defeito em código de produto.
# Testes que asseguram a ausência, README e site (instrução de instalação para
# humano, onde @latest é o correto) ficam fora por construção.
#
# Histórico das correções anteriores, mantido porque cada uma foi um controle que
# falhou: (1) contava com `grep -rn` nos diretórios e media um .pyc de __pycache__,
# não hermético; (2) buscava só "releases/latest" e era cega para o segundo
# mecanismo, `go install ...@latest` — a Wave 0 declarou enumeração fechada sobre
# um padrão incompleto.
set -eu

# 1. O defeito: instalação por go install sem pin, em código de produto.
prod=$(git ls-files -z scripts internal npm/src pypi/trackfw \
  | xargs -0 grep -l "trackfw@latest" 2>/dev/null \
  | grep -v -e "_test\.go$" -e "\.test\.js$" -e "/tests/" || true)
if [ -n "$prod" ]; then
  echo "Wave 0: go install sem pin ainda presente em codigo de produto:" >&2
  echo "$prod" >&2
  exit 1
fi

# 2. O fetch do install.sh continua apontando para releases/latest — INTENCIONAL:
#    o script e sempre o mais recente, e quem pina o binario e TRACKFW_VERSION.
#    O conjunto e fechado nos 3 geradores; superficie nova aqui exige revisao.
esperado="internal/generators/scaffold.go
npm/src/generators/init.js
pypi/trackfw/generators/init_gen.py"
medido=$(git ls-files -z scripts internal npm/src pypi/trackfw \
  | xargs -0 grep -l "releases/latest/download/install.sh" 2>/dev/null | sort)
if [ "$medido" != "$esperado" ]; then
  echo "Wave 0: conjunto de geradores que buscam install.sh mudou." >&2
  echo "esperado:"; echo "$esperado" >&2
  echo "medido:";   echo "$medido"   >&2
  exit 1
fi
[ -n "$medido" ] || { echo "guarda de vacuidade: nenhum arquivo varrido" >&2; exit 1; }
echo "Wave 0 gate OK — zero go install sem pin; 3 geradores buscando install.sh."
```

#### Resultado do ML-0A (hades-tf, 2026-08-28)

**Sobre o escopo:** análise pura, nenhuma linha de `scripts/install.sh`, `internal/`, `npm/`,
`pypi/`, `Makefile` foi tocada. A única escrita é este bloco e o gate acima.

##### 1. Completude de enumeração

> **FALSIFICADA PELO ARQUITETO (2026-08-28), após o ML-2B.** A conclusão "a lista fechou" está
> **errada**, e o erro é meu: eu dei ao ML-0A o padrão de busca `releases/latest`, e a enumeração
> herdou a cegueira do padrão. Existe um **segundo mecanismo de instalação** no produto —
> `go install github.com/kgsaran/trackfw/cmd/trackfw@latest` — usado pelo workflow
> `.github/workflows/trackfw-validate.yml`, que o `discover` escreve nos 3 CLIs
> (`internal/discover/discover.go:274`, `npm/src/commands/discover.js:320,564`,
> `pypi/trackfw/commands/discover.py:463`). Ele é tão não pinado quanto o outro.
>
> A superfície real é de **8 arquivos**, não 4. O gate da wave foi corrigido para varrer os dois
> mecanismos. Fica registrado como está: uma Wave 0 que declarou enumeração fechada sobre um
> padrão incompleto é o mesmo defeito que a REQ combate, cometido pelo controle da própria REQ.



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

> **CORREÇÃO DO ARQUITETO (2026-08-28) — o residual seguinte está factualmente incorreto e fica
> registrado apenas como rastro da investigação.** A premissa de que o Python "gera o workflow
> pinado no `init` e nunca recebe o bump" veio do meu próprio prompt para o ML-0A, não de medição.
> O CLI Python **nunca escreveu workflow de CI**: não há `--ci` em `pypi/trackfw/commands/init.py`,
> não há gerador em `pypi/trackfw/generators/`, e as 2 ocorrências de `releases/latest` em
> `init_gen.py:541,571` são texto de ajuda. Logo o Python não pina uma vez — ele nunca pina.
> Por decisão de KG, a lacuna deixou de ser residual e virou escopo: o ML-2C acima foi reescrito
> para fechá-la (gerar + `update` + `doctor`), e a exclusão em `cli-parity.md` será apagada na
> Wave 3 (AC16).

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
**Status:** ✅ Concluído
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

**Critérios de aceite:**
- [x] AC1, AC2, AC3, AC4, AC5 da REQ verificáveis pelo gate novo
- [x] `sh -n scripts/install.sh` → exit 0 (sintaxe POSIX válida)
- [x] `bash scripts/check-install-version-pin.sh` → exit 0, com contagem de cenários impressa
- [x] Guarda de vacuidade provada: rodar o gate com a lista de cenários vazia faz o gate falhar
- [x] Nenhum download real disparado durante o gate
- [x] `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 make quality` → **bloqueado por defeito pré-existente
      fora do escopo deste ML** (ver seção de resultado abaixo) — todos os passos anteriores ao
      ponto de bloqueio, e os gates individuais substitutos após o ponto de bloqueio, passam.
**Comandos de validação:**
```bash
sh -n scripts/install.sh
bash scripts/check-install-version-pin.sh
TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 make quality
```

#### Resultado do ML-1A (ares-tf, 2026-08-28)

**Sobre o escopo:** só `scripts/install.sh`, `scripts/check-install-version-pin.sh` (novo) e
`Makefile` foram tocados. Nenhuma linha de `internal/`, `npm/`, `pypi/`, `docs/`, `.github/`.

**Decisão de implementação da validação.** `case`, não `grep -E`, como o roadmap e o ML-0A exigem.
Em vez de tentar expressar "um ou mais dígitos" diretamente num glob (impossível: `*` em glob casa
QUALQUER sequência de caracteres, não só dígitos — um padrão como `[0-9]*.[0-9]*.[0-9]*` deixaria o
`*` final engolir `; rm -rf /` inteiro), a validação em `scripts/install.sh` faz em duas etapas:

```sh
VERSION=""
if [ -n "${TRACKFW_VERSION:-}" ]; then
  _tv_raw="$TRACKFW_VERSION"
  case "$_tv_raw" in
    v*) _tv_body="${_tv_raw#v}" ;;
    *)  _tv_body="$_tv_raw" ;;
  esac
  _tv_valid=1
  case "$_tv_body" in
    *[!0-9.]*|.*|*.|*..*|"")
      _tv_valid=0
      ;;
  esac
  if [ "$_tv_valid" = "1" ]; then
    _tv_dots=$(printf '%s' "$_tv_body" | tr -cd '.' | wc -c | tr -d ' ')
    [ "$_tv_dots" = "2" ] || _tv_valid=0
  fi
  if [ "$_tv_valid" != "1" ]; then
    echo "Erro: TRACKFW_VERSION invalida: '${_tv_raw}'" >&2
    echo "Formato esperado: v?MAJOR.MINOR.PATCH (ex.: 7.3.0 ou v7.3.0)" >&2
    exit 1
  fi
  VERSION="v${_tv_body}"
fi
```

1. `*[!0-9.]*` (classe negada) rejeita qualquer caractere fora de `0-9`/`.` — isto é a barreira
   real contra os payloads de injeção (`;`, `$`, `(`, `)`, backtick, espaço, `/`, `&`, `|` não
   pertencem ao alfabeto permitido).
2. `.*`/`*.`/`*..*`/`""` rejeitam ponto na ponta, grupo vazio (`7..0`) e string vazia.
3. Contar os pontos (`tr -cd '.' | wc -c`) e exigir exatamente 2 fecha a estrutura
   MAJOR.MINOR.PATCH (3 grupos), sem depender de quantificador de dígitos em glob.

O `[ -n "${TRACKFW_VERSION:-}" ]` (sem trim) decide se entra no ramo de validação — `"   "`
(só espaços) é não-vazio como string literal, entra no ramo, e o charset já rejeita o espaço; não
foi necessário nenhum trim explícito para satisfazer o cenário "só espaços" da lista de falha
(consistente com a leitura do ML-0A/seção 2: "o trim citado na REQ é sobre a condição 'está
definida', não sobre limpar espaços do meio do regex").

**Seam de teste.** `TRACKFW_INSTALL_DRYRUN=1` imprime `URL:` (já existia) e `DEST:` (novo — o
argumento exato do `-o` do curl de download) e sai com 0 antes de qualquer rede, com `rm -rf
"${TMP_DIR}"` antes do `exit 0` para não vazar diretório temporário a cada invocação do gate.

**Por que o gate testa `DEST`, não só `URL`.** Auditoria do arquiteto (advisor) apontou que a
primeira versão do gate só lia a `URL` impressa — se a validação fosse movida para depois da
composição de `URL`/`FILENAME`, o gate continuaria verde porque o dryrun sai antes do `curl`
mesmo assim. Corrigido: `pass_pinned` agora extrai `DEST` da saída e afirma (a) ausência de `..`
e (b) que o basename bate exatamente `trackfw_<bare>_<os>_<arch>.tar.gz` — a propriedade que um
`/`/`..` vazando de `VERSION_BARE` quebraria. `assert_fails_with` afirma a ausência da linha
`DEST:` (a rejeição tem que ocorrer antes de qualquer composição). Cenário novo
`path-traversal-targets-dash-o-dest` (`7.3.0/../../tmp/evil`) nomeia esse alvo explicitamente.
Prova empírica da sensibilidade da asserção: uma cópia de `install.sh` com a validação removida
(simulando a regressão) produziu, para o mesmo payload, `DEST:
.../trackfw_7.3.0/../../tmp/evil_darwin_arm64.tar.gz` e `exit 0` — exatamente o que a asserção
`*..*` e o `EC -eq 0` de `assert_fails_with` capturam.

**Evidência medida:**

```
$ sh -n scripts/install.sh
(exit 0)

$ bash scripts/check-install-version-pin.sh
OK   [install-version-pin/pinned-bare]
OK   [install-version-pin/pinned-v-prefixed]
OK   [install-version-pin/pinned-multi-digit-minor]
OK   [install-version-pin/pinned-multi-digit-major]
OK   [install-version-pin/pinned-pre-1.0-no-v]
OK   [install-version-pin/ac5-same-asset]
OK   [install-version-pin/unset-resolves-via-api]
OK   [install-version-pin/empty-resolves-via-api]
OK   [install-version-pin/command-separator-semicolon]
OK   [install-version-pin/command-substitution-dollar]
OK   [install-version-pin/command-substitution-backtick]
OK   [install-version-pin/command-separator-and-pipe]
OK   [install-version-pin/path-traversal]
OK   [install-version-pin/path-traversal-targets-dash-o-dest]
OK   [install-version-pin/whitespace-only]
OK   [install-version-pin/embedded-newline-with-trailing-content]
install-version-pin: 16 cenarios OK
(exit 0)
```

Guarda de vacuidade provada empiricamente: uma cópia do gate com o bloco de invocação de
cenários removido (via script Python, nunca commitada) sai com
`FAIL [install-version-pin]: guarda de vacuidade — nenhum cenario rodou` e `exit 1`. O arquivo
real do repositório nunca teve os cenários removidos.

**`TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 make quality` — bloqueado, fora deste escopo.** O alvo
`parity` do `Makefile` roda `scripts/check-referential-integrity.sh` antes de chegar em
`scripts/check-install-version-pin.sh` (linha adicionada por este ML é a última do alvo). Esse
gate (pré-existente, não tocado por este ML) falha com:

```
referential integrity failed: docs/req/REQ-2026-08-28-gate-de-ci-gerado-instala-versao-nao-
pinada-do-trackfw-e-nao-ha-como-pinar.md adr "ADR-2026-08-28-gate-de-ci-gerado-nasce-pinado-na-
versao-que-o-gerou-e-o-install-sh-honra-trackfw-version.md" does not exist
```

Causa raiz (confirmada por leitura direta): o ADR **existe e está commitado**
(`docs/adr/ADR-2026-08-28-...md`, commit `bb7bf72`), mas o frontmatter do REQ referencia
`adr: "ADR-2026-08-28-....md"` como nome de arquivo **sem** o prefixo `docs/adr/` — diferente do
campo `roadmap:` no mesmo frontmatter, que usa o caminho completo
(`docs/roadmaps/wip/ROADMAP-...md`). `scripts/check-referential-integrity.sh` resolve o valor de
`adr:`/`roadmap:` relativo à raiz do repo (`[[ ! -f "$value" ]]` com cwd em `ROOT_DIR`), então um
valor sem o diretório nunca resolve. Este é um defeito de autoria do **REQ**
(`docs/req/`), fora dos arquivos permitidos a este ML e fora do papel de Infrastructure —
reportado ao arquiteto/trackfw_architect para correção do frontmatter, não corrigido aqui.

Evidência de que o defeito é anterior a este ML e não introduzido por ele: `git status --short`
mostra só `M Makefile`, `M scripts/install.sh`, `?? scripts/check-install-version-pin.sh` — nenhum
arquivo em `docs/req/` ou `docs/adr/` foi tocado nesta sessão.

Como o defeito interrompe `make parity` antes de alcançar meu gate, rodei os passos restantes de
`make quality` individualmente para substituir a evidência que `make quality` teria produzido:

```
$ TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 go test -timeout 2m ./...        # via `make test`
98 passed, 0 failed, 0 xfail  (Node.js embedded no runner Go, ver saída completa)

$ cd npm && npm test                                                    # via `make test-node`
tests 778, pass 778, fail 0

$ python3 -m pytest pypi/tests -q                                       # via `make test-python`
1490 passed, 28 subtests passed

$ go vet ./...                                                          # via `make lint`
(sem saída, exit 0)

$ GO_BIN=bin/trackfw scripts/check-cli-parity.sh          → OK
$ scripts/check-validate-parity.sh                        → OK
$ scripts/check-referential-integrity.sh                  → FALHA (defeito pré-existente acima)
$ GO_BIN=bin/trackfw scripts/check-parity-contract-coverage.sh → exit 0
$ scripts/check-static-assets.sh                           → "Static assets are synchronized"
$ scripts/check-integration-assets.sh                       → "Integration assets are synchronized"
$ bash scripts/check-install-version-pin.sh                 → exit 0, 16 cenarios (ver acima)
```

`scripts/check-gates-falsify.sh` (suíte grande, minutos de execução) foi disparado em background
para não bloquear a entrega; resultado a ser conferido pelo arquiteto na auditoria — este ML não
toca nenhum código exercitado por essa suíte (ela cobre hooks/guards, não `install.sh`), então o
risco de regressão cruzada é baixo, mas não fechei essa evidência antes de reportar por
disciplina de tempo.

**Não fiz:** não criei nem editei nenhum arquivo em `docs/adr/` ou `docs/req/` para corrigir o
defeito de referência acima — está fora dos arquivos permitidos a este ML.

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
**Critérios de aceite:**
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
**Critérios de aceite:**
- [ ] AC6, AC7 (Node), AC10, AC11, AC13 (parte Node)
- [ ] `npm test --prefix npm` → exit 0
- [ ] Nenhuma versão literal no template

### ML-2C — Python: fecha a paridade do workflow de CI (gerar, gerenciar no `update`, cobrir no `doctor`)
**Status:** ⬜ Pendente
**Agente:** `apolo-tf`

> **Este ML foi reescrito em 2026-08-28, depois da Wave 0.** A versão original mandava "pinar o
> template Python". Medição do arquiteto: **não existe template Python.** O CLI Python nunca gerou
> workflow de CI nenhum — não há `--ci` no `init.py`, não há gerador em `pypi/trackfw/generators/`,
> e as 2 ocorrências de `releases/latest` em `init_gen.py:541,571` são **texto de ajuda** do
> "trackfw não está instalado", não template de workflow. O residual do ML-0A que diz que o Python
> "pina uma vez e nunca mais bumpa" descende dessa mesma premissa errada e **está factualmente
> incorreto**: o Python não pina porque não escreve o arquivo.
>
> Decisão de KG (regra dura de paridade — os 3 CLIs têm as mesmas funcionalidades e funcionam
> exatamente igual): a lacuna é **fechada**, não estreitada. `pypi/trackfw/config.py:431` **já lê a
> chave `ci:`** do `trackfw.yaml`, então a exclusão documentada em `cli-parity.md` como "fundamentada
> em propriedade" não é limite de capacidade — é escolha.

**Files affected:** `pypi/trackfw/generators/init_gen.py`,
`pypi/trackfw/integrations/scaffold_doctor.py`, `pypi/trackfw/commands/update.py`, `pypi/tests/`

**Actions:**
1. **Criar os dois builders no Python**, byte-idênticos aos de Go e Node para a mesma versão:
   `build_github_actions_workflow_content` e `build_gitlab_ci_workflow_content`, já emitindo
   `TRACKFW_VERSION: "<versão>"` e `timeout-minutes: 10`. A versão vem de `trackfw.__version__` —
   **nunca literal**.
2. **Gerar o workflow** quando `cfg["update"]["ci"]` for `github-actions` ou `gitlab-ci`, no mesmo
   ponto do fluxo em que Go e Node geram.
3. **Adicionar `ci-workflow` ao `PROJECT_TARGET_IDS`** de `pypi/trackfw/commands/update.py:107`, na
   **mesma posição** da lista de Go e Node, com o mesmo relatório de estado
   (`updated`/`skipped`/`missing`).
4. **Cobrir o workflow no `scaffold_doctor.py`**, removendo a exclusão de `:23-25` e `:382`. O
   remédio impresso passa a ser `trackfw update`, que agora é verdade neste runtime.
5. Cobrir `init_gen.py:541` e `:571` conforme AC13 — são texto de ajuda; atualizar ou declarar
   explicitamente fora do pin, sem deixar instrução contraditória entre os 3 CLIs.
6. Testes Python cobrindo AC6, AC7, AC9, AC10, AC11.

**Critérios de aceite:**
- [ ] AC6, AC7 (Python), AC9 (Python passa a bumpar o pin), AC10, AC11, AC13 (parte Python)
- [ ] `python -m pytest pypi/tests` → exit 0
- [ ] Nenhuma versão literal no template
- [ ] `ci-workflow` aparece no relatório de alvos do `update` do Python, na mesma posição de Go/Node
- [ ] A seção "CI workflow exclusion — Python" de `docs/cli-parity.md` é **apagada** na Wave 3, não
      estreitada — e a tabela de cobertura por runtime passa a marcar "sim" para os 3 CLIs nas duas
      linhas de workflow

**Assimetria que PERMANECE e não é fechada aqui:** o `init` do Python continua sem `--ci` e sem
`--hooks`, então ele gerencia o workflow de um projeto cujo `trackfw.yaml` já declara `ci:`, mas não
*escolhe* CI na criação. Coberto por
`REQ-2026-08-28-cli-python-nao-oferece-superficie-de-ci-e-git-hooks-no-init-e-nao-declara-git-hooks-como-alvo-do-update.md`,
que declara dependência desta REQ.

## Corretivas da Wave 2 (ML-2D a ML-2H)
> Dependências: Wave 2 auditada. Todas nasceram da auditoria do arquiteto ou de achado de agente,
> e todas já estão concluídas e commitadas.

### ML-2D — `timeout: 10 minutes` no GitLab de Go e Node
**Status:** ✅ Concluído · `apolo-tf`
O Python emitia a linha, Go e Node não. Alinhado **para cima**: o análogo GitHub
(`timeout-minutes: 10`) já existia, e foi a perda desse controle que causou o incidente de
2026-08-27 no cmdb. Verificado por diff byte a byte dos 3.

### ML-2E — retarget do alvo `ci-workflow` do `update` do Node
**Status:** ✅ Concluído · `apolo-tf`
O alvo apontava para `trackfw-validate.yml` via `discover.writeCIWorkflowForce` — arquivo
diferente do que o Go gerencia. O pin desta REQ nunca receberia bump no Node: **AC9
insatisfazível**. Retargetado para o mesmo par do Go, e passou a cobrir `gitlab-ci`, que estava
fora da condição.

### ML-2F — segundo mecanismo de instalação (`go install …@latest`)
**Status:** ✅ Concluído · `apolo-tf`
`.github/workflows/trackfw-validate.yml`, escrito pelo `discover` nos 3 CLIs, instalava por
`go install …@latest` — tão não pinado quanto o `install.sh` era. Passou a `@v<versão do binário>`
e ganhou cobertura no `doctor` dos 3, que não existia em nenhum. **A Wave 0 não tinha visto porque
o padrão de busca que eu dei a ela (`releases/latest`) nunca casa com `@latest`** — ver a nota de
falsificação na seção 1 do ML-0A.

### ML-2G — `update` gerencia o `trackfw-validate.yml`
**Status:** ❌ Reprovado na auditoria, corrigido pelo ML-2H · `apolo-tf`
Implementou o AC17 no caminho de **alvos** (`runFileTarget`, usado por `--targets`/`--json`) e
declarou a prova ponta-a-ponta feita. Mas o `trackfw update` tem dois caminhos, e o que o usuário
digita — e que o `doctor` manda rodar — é o outro. Os testes dele asseriam pelo caminho de alvos
enquanto alegavam provar o remédio do `doctor`. **Registrado como reprovado, não apagado:** é a
mesma família dos cinco defeitos da 7.3.0, cometida dentro da REQ que os combate.

### ML-2H — o caminho simples do `update`
**Status:** ✅ Concluído · `apolo-tf`
`Update()` (`update.go:37`) e `_run()` (`update.py:420`) chamavam o gerador direto, sem passar pelo
caminho de alvos; o Node nunca teve essa cisão. Corrigido **reaproveitando o helper** que o caminho
de alvos já usava, em vez de copiar a regra — duplicação foi como o bug nasceu. Testes do 2G
reescritos para exercitar o comando nu.

**Auditoria do arquiteto — os 3 CLIs reais, não chamada de função:**

| | doctor→update→doctor | não cria | `ci: none` refresca |
|---|---|---|---|
| Go (`bin/trackfw`) | acusa → `@v7.3.0` → limpo | não cria | `@v7.3.0` |
| Node (`npm/bin/trackfw`) | acusa → `@v7.3.0` → limpo | não cria | `@v7.3.0` |
| Python (`-m trackfw`) | acusa → `@v7.3.0` → limpo | não cria | `@v7.3.0` |

**Byte-identidade dos 9 builders (3 templates × 3 CLIs):** idênticos.
**`make quality` completo:** exit 0, 181 cenários de falsificação.

## Wave 3 — Gate de paridade, contrato e evidência
> Dependências: Wave 2 completa nos três. ML único — toca arquivos compartilhados pelos 3 stacks.

### ML-3A — Gate falsificável de paridade do pin + `docs/cli-parity.md`
**Status:** ✅ Concluído
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
**Critérios de aceite:**
- [x] AC8, AC14, AC16 — medidos pelo arquiteto, ver evidência abaixo
- [x] `bash scripts/check-ci-workflow-pin-parity.sh` → exit 0, **15 cenários**
- [x] `bash scripts/check-parity-contract-coverage.sh` → exit 0 (217 seções, 0 sem anotação)
- [x] AC15: `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 make quality` → exit 0
- [x] Falsificação real (não pelo helper interno): apagar `timeout-minutes: 10` de
      `internal/generators/scaffold.go` faz o gate reprovar com dois `FAIL` — um nomeando os pares
      divergentes, outro a linha ausente. Restaurado, gate volta a exit 0.
- [x] Nenhum resíduo `zz_dump*` na árvore, **inclusive após falha** — o `trap` segurou
- [x] `docs/cli-parity.md`: seção "CI workflow exclusion — Python (principled)" apagada; tabela de
      cobertura marca `sim` nos 3; seção nova anotada com `gate=`; `gap reason=` para o `init` do
      Python

**Evidence:**
```
bash scripts/check-ci-workflow-pin-parity.sh        exit 0 — 15 cenários OK
  regressão injetada (timeout-minutes removido)     exit != 0
    FAIL [pin-parity/github-actions/byte-identical]: go vs node divergem; go vs py divergem;
    FAIL [pin-parity/github-actions/timeout-minutes]: timeout-minutes: 10 ausente
  find . -name "zz_dump*"                           vazio
bash scripts/check-parity-contract-coverage.sh      exit 0
bash scripts/check-doctor-parity.sh                 exit 0 — 36 cenários (ML-3B, diff só comentário)
TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 make quality    exit 0
grep "CI workflow exclusion — Python"               sem resultado (apagada)
```
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

#### Resultado da Wave 3 (artemis-tf, 2026-08-28)

**ML-3A — ✅ Concluído.** `scripts/check-ci-workflow-pin-parity.sh`, 15 cenários, exit 0. Extrai os
9 builders (3 templates × 3 runtimes) e compara byte a byte; os dois builders Go do primeiro par não
são exportados e saem por um `_test.go` temporário removido por `trap` — verificado que não sobra
resíduo nem quando o gate falha.

Auditoria do arquiteto, falsificando de verdade: apagar `timeout-minutes: 10` de `scaffold.go` faz o
gate reprovar com dois `FAIL` — um nomeando os pares divergentes, outro nomeando a linha ausente.

`docs/cli-parity.md`: a seção "CI workflow exclusion — Python (principled)" foi **apagada** (AC16),
a tabela de cobertura marca `sim` para os 3, e há seção nova com o contrato do pin anotada com
`gate=`. A assimetria que permanece — `init` do Python sem `--ci`/`--hooks` — está anotada como
`gap reason=` apontando para a REQ dependente.

**Correção da própria agente, digna de registro:** no primeiro passe ela removeu o `partial=` inteiro
da anotação da tabela, o que passaria a alegar que o `check-doctor-parity.sh` exercita aquelas linhas
cross-CLI. Não exercita — as fixtures dele nunca definem `ci:`, então o check de CI workflow não
dispara em runtime nenhum. Reintroduziu o `partial=` nomeando a limitação exata.

**ML-3B — ✅ Concluído.** A mesma afirmação falsa que o AC16 apagou do `cli-parity.md` sobrevivia no
cabeçalho de `scripts/check-doctor-parity.sh:404` ("Python's `update` does not manage ci-workflow").
Reescrita para dizer o que é verdade — os 3 gerenciam; o motivo de este gate não exercitar o caminho
é que as fixtures não definem `ci:` — e apontando para onde a cobertura existe. Diff **só de
comentário**, verificado; 36 cenários, mesma contagem de antes.

#### Parecer de qualidade da barreira final (hefesto-tf, 2026-08-28)

**Veredito: REPROVA**, por um único achado bloqueante — comentário obsoleto em código de produto,
corretivo estritamente comment-only, sem risco de regressão. Os demais achados são dívida aceitável
ou observações fora de escopo, registrados abaixo.

##### O que bloqueia o PR

**`internal/generators/update.go:1811-1819`** — o bloco `// NOTE ON CROSS-RUNTIME PARITY:` afirma:

> "the Python CLI intentionally implements a reduced project-scope surface (agent rules + hooks +
> Codex project agents only — see pypi/trackfw/commands/update.py's own docstring, which points
> users to the Go/Node.js CLIs for CI/git-hooks/Claude commands)"

Medido contra o estado real do diff: `project_target_ids` (`pypi/trackfw/commands/update.py:152-168`)
já declara `ci-workflow` condicionalmente — exatamente a mesma regra de Go/Node, entregue por ML-2C
**desta REQ** — e `_run_project` já despacha `claude-commands`
(`pypi/trackfw/commands/update.py:819`, pré-existente a esta REQ). O único gap real remanescente é
`git-hooks` (sem flag `--hooks` no `init` do Python — item explicitamente fora de escopo desta REQ,
ver Negative Scope da REQ). O próprio docstring de módulo de `update.py` (linhas 17-40) já foi
atualizado corretamente e diz o oposto do que este comentário em Go afirma.

Esta é a mesma classe de defeito que o **AC16** desta REQ existe para eliminar (a seção "CI workflow
exclusion — Python (principled)" apagada de `docs/cli-parity.md`) e que o **ML-3B** corrigiu no
cabeçalho de `scripts/check-doctor-parity.sh:404` ("Python's `update` does not manage ci-workflow").
A auditoria de ambos os pontos foi feita; este terceiro ponto — no arquivo de produção mais central
desta REQ — sobreviveu. Pelo precedente que a própria REQ estabeleceu (afirmação de paridade
obsoleta é entregável, não débito), isto bloqueia.

**Microlote corretivo mínimo:**

- **Arquivo:** `internal/generators/update.go`, linhas 1811-1819 (bloco `NOTE ON CROSS-RUNTIME
  PARITY`).
- **Ação:** reescrever para refletir o estado medido — a superfície `--targets` do Python já inclui
  `ci-workflow` (condicional, ML-2C, mesma regra de Go/Node) e `claude-commands` (pré-existente);
  `git-hooks` é a única lacuna real, e por quê (sem `--hooks` no `init` do Python). Sem mudança de
  comportamento — comentário apenas, nenhum arquivo de teste precisa mudar.
- **Critério de aceite:** `grep -n "reduced project-scope surface" internal/generators/update.go`
  sem resultado; `go build ./...` e `go test ./internal/generators/...` continuam verdes (nenhuma
  mudança de comportamento esperada).

##### Dívida aceitável / observações (não bloqueiam)

1. **`internal/generators/ci_workflow_version_pin_test.go:104`** — a asserção nega a presença do
   literal `"7.3.0"` no bloco dos builders. É a versão corrente hoje; no próximo bump (7.4.0) essa
   cláusula negativa específica nunca mais dispara — não é vácuo perigoso porque a asserção
   positiva na linha seguinte (`must reference version.Version`) é quem carrega o peso real da
   prova, mas é a única das três linguagens com um literal congelado: Node usa
   `PACKAGE_VERSION` (`ci_workflow_version_pin.test.js:84`) e Python usa `TRACKFW_VERSION`
   (`test_ci_workflow_pin.py:69`), ambos dinâmicos via `assertNotIn`/`includes`. Sugestão para um
   ML futuro: trocar para `strings.Contains(block, "\""+version.Version+"\"")` negado, espelhando
   as outras duas linguagens.

2. **Duplicação do predicado `cfg.CI` — dívida registrada, não é a classe de risco do ML-2G.**
   `cfg.CI != "" && cfg.CI != "none"` aparece duas vezes em Go (`update.go:97`, gate do print; e
   `:1835`, gate da lista de alvos declarados) e o equivalente em Python
   (`update.py:420` e `:167`). À primeira vista parece o mesmo padrão que produziu o ML-2G, mas
   **não é**: `generateCIWorkflow`/`generate_ci_workflow` se auto-protegem internamente via switch
   sobre `cfg.CI` (`scaffold.go:1888-1896` e equivalente Python) — o predicado duplicado só governa
   a linha de status impressa e a lista de alvos declarados, nunca se o write acontece. O Node evita
   a duplicação por construção: não existe um "caminho simples" separado — `update.js:429-431`
   chama `buildProjectTargets` incondicionalmente até para o comando puro. Registrando aqui para que
   um leitor futuro não reabra isto como o mesmo risco do ML-2G sem primeiro checar que o gate real
   (o write) está unificado.

3. **Fora de escopo desta REQ, não é regressão dela.** `pypi/trackfw/commands/update.py:468-472`
   imprime "Para atualizar Claude commands, use: trackfw update (CLI Go)" mesmo `_run_project`
   (caminho `--targets`) já despachando `claude-commands`. Conferido via `git diff` que essa
   assimetria (o caminho simples `_run` nunca chama `generate_claude_commands`, só o caminho de
   alvos chama) é **anterior** a esta REQ — os trechos de despacho de `claude-commands` em
   `_run_project` aparecem como contexto inalterado no diff, não como adição. Não é o mesmo defeito
   do achado bloqueante acima (que é sobre um comentário no Go tornado falso pelas mudanças *desta*
   REQ); é uma assimetria pré-existente do Python, fora do escopo aqui. Sinalizando para uma REQ
   futura, sem bloquear esta.

##### Respostas às 5 perguntas dirigidas

1. **Duplicação do caminho simples vs. alvos do `update`** — a arquitetura do ML-2H está correta
   nos 3 CLIs: o caminho simples e o caminho de alvos reaproveitam o **mesmo** helper
   (`refreshDiscoverGitHubActionsWorkflowIfPresent`/`discoverWorkflowPresent` em Go,
   `refreshDiscoverGitHubActionsWorkflowIfPresent` em Node, `_refresh_discover_github_actions_
   workflow_if_present`/`_discover_workflow_present` em Python) para decidir **o quê** escrever e
   **quando**. Não há write duplicado. A única duplicação residual é o predicado `cfg.CI` (item 2
   acima), que não governa o write — não é a mesma classe de risco.

2. **Os 9 builders (3 templates × 3 runtimes)** — resposta explícita, como pedido: **não há forma
   estrutural razoável de reduzir a superfície sem violar a regra dos 3 CLIs.** São três artefatos
   de runtime independentes (binário Go, pacote npm, pacote pypi) sem asset compartilhável em tempo
   de execução — é o custo da regra de paridade dos 3 CLIs, não um problema de design deste
   roadmap. O que existe hoje (`scripts/check-ci-workflow-pin-parity.sh`) é o "golden file" possível
   nesse desenho: compara os 9 builders byte a byte, com guarda de vacuidade (linhas 319-322,
   verificada presente) e `trap` de limpeza mesmo em falha (linha 33, verificado presente). É
   exatamente por essa ausência de estrutura compartilhada que esse gate carrega mais peso que o
   normal — é o único mecanismo segurando a paridade.

3. **Testes novos** — busquei especificamente o padrão do ML-2G (asserção que passaria mesmo com a
   versão hardcoded; teste de função interna alegando cobrir comportamento de CLI) e não encontrei
   recorrência, com uma exceção cosmética (achado não bloqueante 1, acima). Os testes de pin de
   versão em Node (`ci_workflow_version_pin.test.js`) e Python (`test_ci_workflow_pin.py`) usam
   `PACKAGE_VERSION`/`TRACKFW_VERSION` dinâmicos, nunca literais. Os testes de alvo do `update` em
   Node (`update_ci_workflow_target*.test.js`) invocam o binário real via `spawnSync`, não função
   interna. E os três runtimes têm um teste dedicado, comentado citando o ML-2G explicitamente, que
   prova o caminho que escapou da primeira auditoria: `internal/generators/update_test.go:1969`
   (`TestUpdateCiWorkflowClosesDoctorFindingForDiscoverWorkflow`, chama `Update()` puro, não
   `UpdateProject()`); `pypi/tests/test_update_ci_workflow_discover_workflow.py` (`_run_bare_update`,
   linhas 70-82); Node (`update_ci_workflow_target_discover_workflow.test.js:181`,
   `run(['update'], ...)` sem `--json`/`--targets`). Essa é a evidência mais forte a favor da
   barreira: o próprio buraco do ML-2G tem regressão nomeada nos 3 CLIs.

4. **`scripts/install.sh`** — legível e defendido contra a "simplificação" óbvia. O comentário nas
   linhas 35-43 explica explicitamente por que não é `grep -E` (âncora por linha vs. buffer inteiro
   do parâmetro do shell), cita a vault note da mesma família de bug
   (`bash-grep-F-embedded-newline-vacuous-match-2026-08-16.md`) e nomeia o alvo real do path
   traversal (o `-o` do `curl`, não a URL). A validação em duas etapas (charset via `case`, depois
   contagem de pontos via `tr -cd '.' | wc -c`) tem comentário acima da média do repositório neste
   ponto específico — um mantenedor futuro tem o que precisa para não reintroduzir o bug de âncora
   por linha.

5. **Nomes e convenções entre os 3 runtimes** — consistentes, respeitando o idioma de cada
   linguagem: `DiscoverGitHubActionsWorkflowPath`/`BuildDiscoverGitHubActionsWorkflowContent`
   (Go, PascalCase)· `DISCOVER_GITHUB_ACTIONS_WORKFLOW_PATH`/`buildDiscoverGitHubActionsWorkflowContent`
   (Node) · `DISCOVER_GITHUB_ACTIONS_WORKFLOW_PATH`/`build_discover_github_actions_workflow_content`
   (Python). Nenhuma divergência de nome encontrada; a única divergência de fundo é o achado
   bloqueante acima (conteúdo do comentário, não o nome).

**Comandos executados nesta revisão (leitura apenas, nenhum gate re-rodado por instrução do
arquiteto):** `git diff origin/main...HEAD --stat`, `go build ./...`, inspeção linha a linha de
`internal/generators/update.go` (2348 linhas, lido em duas partes), `internal/generators/
scaffold_doctor.go`, `internal/discover/discover.go`, `scripts/install.sh`,
`scripts/check-ci-workflow-pin-parity.sh`, os 3 arquivos de comando `update.js`/`update.py`, e os
testes de pin de versão e de alvo `ci-workflow` nos 3 runtimes.

**Fronteiras mantidas:** nenhum arquivo de código de produto tocado; nenhum commit, push ou PR
executado; nenhum outro arquivo além desta seção apensada a este roadmap.

#### Parecer de segurança da barreira final (hades-tf, 2026-08-28)

**Veredito: REPROVA.**

Achado bloqueante: `trackfw update` (e, num sub-caso, `trackfw discover --init`) escrevem em
`.github/workflows/trackfw-validate.yml` seguindo symlink, sem `lstat`. Um repositório hostil que o
mantenedor clona e sobre o qual roda `trackfw update`/`discover --init` consegue sobrescrever, ou
até **criar**, um arquivo arbitrário fora do projeto, no caminho de sua escolha. Reproduzido ao vivo
nos 3 CLIs. O conteúdo escrito é sempre o template fixo do trackfw (não é conteúdo do atacante), então
a classe é "escrita/criação de arquivo arbitrário com conteúdo confiável", não RCE — severidade
**ALTA**, não crítica.

##### 1. Achado bloqueante — symlink-follow em `.github/workflows/trackfw-validate.yml` (ALTA)

**Mecanismo.** `refreshDiscoverGitHubActionsWorkflowIfPresent` (Go) e seus equivalentes Node/Python
decidem "o arquivo já existe" com `os.Stat`/`fs.existsSync`/`os.path.isfile` — todas essas chamadas
seguem symlink. Se `.github/workflows/trackfw-validate.yml` for um symlink para um arquivo fora do
projeto, a checagem de presença retorna verdadeiro e o passo seguinte escreve (`os.WriteFile`/
`fs.writeFileSync`/`open(...,'w')`) no alvo do link, não no link em si — todas essas APIs também
seguem symlink por padrão.

**Localização exata:**
- `internal/generators/update.go:1852` (`discoverWorkflowPresent`) e `:1865`
  (`refreshDiscoverGitHubActionsWorkflowIfPresent`, escrita em `:1871`)
- `npm/src/commands/update.js:179` (`discoverWorkflowPresent`) e `:191`
  (`refreshDiscoverGitHubActionsWorkflowIfPresent`, escrita em `:196`)
- `pypi/trackfw/commands/update.py:173` (`_discover_workflow_present`) e `:185`
  (`_refresh_discover_github_actions_workflow_if_present`, escrita em `:200`)

**Reprodução (Go, idêntico nos outros dois — comandos executados, saída real anexada):**
```bash
mkdir -p proj/.github/workflows outside
echo "PRISTINE-SENTINEL" > outside/authorized_keys
ln -s "$PWD/outside/authorized_keys" proj/.github/workflows/trackfw-validate.yml
printf 'version: 1\nproject: poc\nreq_dir: docs/req\nroadmap_dir: docs/roadmaps\nci: none\n' > proj/trackfw.yaml
cd proj && trackfw update
cat ../outside/authorized_keys
# antes: PRISTINE-SENTINEL
# depois: o template YAML completo do trackfw ("name: trackfw validate\non: [push, pull_request]...")
```
Repetido com sucesso (mesmo resultado: sobrescrita confirmada) em Node
(`node npm/bin/trackfw update`) e Python (`python3 -m trackfw.cli update`), os três com `ci: none`
no `trackfw.yaml` — nenhum dos três exigiu `ci: github-actions` para disparar a escrita.

**`--dry-run` verificado como seguro** (testado explicitamente): a sandbox de `--dry-run` copia o
conteúdo do link para dentro da árvore temporária em vez de preservar o symlink absoluto — o arquivo
fora do projeto permanece intocado nesse modo. O buraco é só no caminho real (sem `--dry-run`).

**Por que isto é um achado desta REQ, não só um defeito antigo em `runFileTarget`.** O mesmo padrão
já existia antes desta REQ para `trackfw-gate.yml`/`.gitlab-ci-trackfw.yml` (reproduzido também, ver
seção "Residual" abaixo) — mas esta REQ **amplia o alcance de forma mensurável**, em dois pontos que
o próprio diff introduz:
1. `refreshDiscoverGitHubActionsWorkflowIfPresent` é uma função nova, chamada incondicionalmente pelo
   passo 3b de `Update()` e pelo alvo `ci-workflow` — antes deste diff nenhum caminho de `update`
   escrevia `trackfw-validate.yml`. É um arquivo novo dentro da superfície de escrita-seguindo-link.
2. `ProjectTargetIDs`/`project_target_ids`/`PROJECT_TARGET_IDS` passou a incluir `ci-workflow` também
   quando `discoverWorkflowPresent` é verdadeiro, **independente de `cfg.CI`**. Um projeto com
   `ci: none` — que declarou explicitamente não querer que `update` mexa em CI — entra na superfície
   só por ter um arquivo (ou, como provado, um symlink) nesse caminho. A reprodução acima usa
   exatamente `ci: none` e disparou nos 3 CLIs: esta é a regressão de alcance atribuível a este diff,
   não o mecanismo em si (que é mais antigo).

**Sub-caso mais grave, achado durante a investigação — `discover --init` com symlink pendurado
(dangling), pré-existente, não modificado por este diff, mas na mesma família e mesmo arquivo-alvo:**
`internal/discover/discover.go:254` `writeCIWorkflow` usa `if isFile(dest) { return }` (idempotente)
antes de escrever. Com um symlink **pendurado** (aponta para um caminho que ainda não existe),
`isFile`/`os.Stat` retorna falso (arquivo não existe *no destino do link*), a guarda idempotente não
barra, e `os.WriteFile` segue o link e **cria** o arquivo no destino escolhido pelo atacante — fora do
projeto. Reproduzido:
```bash
mkdir -p proj/.github/workflows outside
ln -s "$PWD/outside/does-not-exist-yet" proj/.github/workflows/trackfw-validate.yml
cd proj && trackfw discover --init
ls ../outside/         # does-not-exist-yet agora existe
cat ../outside/does-not-exist-yet   # template completo do trackfw
```
Confirmado ao vivo (Go). Este `writeCIWorkflow` **não foi tocado por este diff** (só a função que ele
chama para gerar o conteúdo mudou, de string inline para `generators.BuildDiscoverGitHubActionsWorkflowContent()`)
— é um residual pré-existente, registrado aqui por ser a mesma família de bug no mesmo arquivo-alvo,
não como parte do achado bloqueante. Recomendo que a correção do achado bloqueante cubra os dois de
uma vez (mesmo commit/ML), já que são a mesma classe e o mesmo arquivo de destino.

##### 2. Achados não bloqueantes

- **Validação de `TRACKFW_VERSION` em `scripts/install.sh:32-72`** — robusta. Testada ao vivo com o
  gate `scripts/check-install-version-pin.sh` (16 cenários, todos OK) e com vetores adicionais fora
  do gate: CRLF (`v7.3.0\r`, rejeitado), dígito UTF-8 largo/ponto largo (rejeitado pelo charset),
  entrada de 100 KB só-dígitos (aceita, sem custo perceptível — não é DoS explorável, `case`/`tr` são
  lineares e o valor só chega via variável de ambiente, cujo tamanho já é limitado pelo próprio SO
  antes de chegar ao script). NUL byte não é vetor viável (variável de ambiente não pode conter NUL).
  O comentário do script (linhas 35-48) documenta corretamente por que `case` evita a armadilha de
  `grep -E` ancorar por linha — mesma causa-raiz já registrada em
  `vault/notes/bash-grep-F-embedded-newline-vacuous-match-2026-08-16.md`. Nenhuma regressão frente ao
  modelo de ameaça do ML-0A.
- **`TRACKFW_INSTALL_DRYRUN`** — confirmado que a variável não aparece em nenhum template gerado
  (`grep` nos 3 geradores não encontra ocorrência fora dos scripts de gate/teste); não é alcançável no
  caminho real de CI, não é um seam que um workflow gerado possa disparar por acidente.
- **Pin `go install .../cmd/trackfw@v<versão>`** — falha fechada quando a tag não existe (`go
  install` retorna erro, o step falha, `trackfw validate` nunca roda, job fica vermelho). Tag movida:
  não verificado ao vivo (sem rede neste ambiente), mas por padrão o toolchain Go usa `GOSUMDB`
  habilitado — uma tag re-apontada para conteúdo diferente do já registrado no sumdb produz
  `checksum mismatch`, também falha fechada, contanto que `GONOSUMDB`/`GOFLAGS=-insecure` não tenham
  sido desligados no runner. Não é um ponto a corrigir nesta REQ.
- **Janela de pin decorativo — confirmada como concreta, não hipotética.** `internal/version/
  version.go` tem `Version = "7.3.0"` e a tag mais nova do repositório é `v7.3.0` (medido:
  `git tag --sort=-version:refname | head -1`), ou seja, o `install.sh` publicado em
  `releases/latest` **hoje** ainda não honra `TRACKFW_VERSION` — é o script anterior a este merge.
  Um projeto gerado agora, com o binário desta branch, grava `TRACKFW_VERSION: "7.3.0"` no workflow,
  mas o `install.sh` que esse workflow baixa via `releases/latest/download/install.sh` continua
  ignorando a variável até a próxima release ser publicada. Já registrado como residual do ML-0A;
  confirmo que segue válido e que a janela está aberta neste exato commit — o arquiteto/hefesto-tf não
  devem declarar a REQ "efetiva" antes de confirmar que uma release pós-merge com o `install.sh` novo
  foi publicada.
- **`install.sh` de `releases/latest` continua não pinado (ponto 4 do prompt) — decisão defensável
  para o risco de *drift*, buraco nomeado para o risco de *conta comprometida*.** `TRACKFW_VERSION`
  pina o binário; o script que o busca continua vindo de `releases/latest/download/install.sh` e é
  `curl | sh` sem verificação de checksum/assinatura. Se a conta do repositório for comprometida, um
  atacante que substitua o `install.sh` publicado ganha execução arbitrária em todo consumidor de CI,
  independente do pin — o pin de versão do binário não mitiga esse cenário, só o de desatualização
  involuntária. Não é regressão desta REQ (comportamento inalterado), mas é a resposta correta à
  pergunta: o único fix real seria checksum/assinatura publicada e verificada antes do `sh`, o que é
  escopo de uma REQ própria, não desta.

##### 3. Residuais do ML-0A — status

- **Lacuna do `update` do Python para `ci-workflow`** — RESOLVIDA. Medido em
  `pypi/trackfw/integrations/scaffold_doctor.py:308` (`_check_ci_workflow_artifact`, usa
  `build_github_actions_workflow_content`) e `pypi/trackfw/generators/init_gen.py:559,621`
  (`build_github_actions_workflow_content`, `generate_ci_workflow`) — o Python agora gera e compara
  o workflow, como a nota "FALSIFICADA PELO ARQUITETO" já registrava para o ML-2C.
- **Pin envelhece em silêncio** — segue válido, sem mudança; é comportamento aceito por desenho
  (ADR), não uma regressão.
- **`install.sh` publicado em release antiga não conhece a variável** — segue válido, e **confirmado
  ativo agora** (ver item acima "janela de pin decorativo").
- **Downgrade dentro do espaço de versões publicadas** — segue válido, aceito por desenho.
- **Divergência de contagem de textos de ajuda entre os 3 CLIs** — fora do escopo desta REQ, não
  reavaliado (não é superfície de segurança).
- **Nenhum residual do ML-0A piorou.** O achado novo desta revisão (symlink-follow) não estava
  nomeado no ML-0A original — a seção 2 do ML-0A cobriu substituição de comando, separador de shell,
  path traversal por caractere, newline embutida e versão inexistente, todos sobre
  `TRACKFW_VERSION`/`install.sh`; não cobriu a superfície de escrita de arquivo do `update`/`discover`
  em si, que é onde este achado vive. Não é uma falsificação do ML-0A — é um vetor fora do escopo que
  o próprio ML-0A se deu (validação de `TRACKFW_VERSION`), fica registrado para um ML-0A futuro que
  cubra escrita de arquivo, não só o script `install.sh`.

##### 4. Microlote corretivo mínimo

**Arquivos exatos:**
- `internal/generators/update.go:1852,1865` — trocar `os.Stat` por `os.Lstat` em
  `discoverWorkflowPresent` e em `refreshDiscoverGitHubActionsWorkflowIfPresent`; se
  `info.Mode()&os.ModeSymlink != 0`, tratar como "não gerenciável por `update`" (não escrever, e não
  contar como presente para fins de `ProjectTargetIDs`) e reportar a divergência em vez de seguir o
  link.
- `internal/discover/discover.go:254` (`writeCIWorkflow`) e o par `writeCIWorkflowForce` — mesma
  troca (`os.Lstat` antes de decidir presença/escrever), cobrindo o sub-caso do symlink pendurado.
- `npm/src/commands/update.js:179,191` — `fs.lstatSync(dest, {throwIfNoEntry:false})` +
  `isSymbolicLink()` em vez de `fs.existsSync`/`fs.writeFileSync` direto.
- `npm/src/commands/discover.js` (equivalente de `writeCIWorkflow`/`writeCIWorkflowForce`) — mesma
  troca.
- `pypi/trackfw/commands/update.py:173,185` — `os.path.islink` antes de `os.path.isfile`/`open`.
- `pypi/trackfw/commands/discover.py` (`_write_ci_workflow`) — mesma troca.
- Um teste falsificador por runtime, no molde da reprodução acima (symlink vivo apontando para fora
  do projeto + `update`; symlink pendurado + `discover --init`), que falha antes da correção e passa
  depois.

**Não incluir no corretivo:** blindagem contra symlink em todos os outros alvos de `runFileTarget`
(`trackfw-gate.yml`, `.gitlab-ci-trackfw.yml`, agent-rules, etc.) — confirmado que sofrem do mesmo
padrão (reproduzido para `trackfw-gate.yml`), mas isso é uma REQ própria de hardening geral de
escrita de arquivo em `update`, não desta REQ de pin de versão. Registro aqui como residual a abrir,
não como bloqueio adicional deste PR.

**Comandos executados nesta revisão:** `git log origin/main..HEAD --oneline`, `git diff
origin/main...HEAD --stat`, leitura de `scripts/install.sh`, `bash scripts/check-install-version-pin.sh`
(16/16 OK), `go build ./...` e `go test ./internal/generators/... ./internal/discover/...` (verdes),
vetores manuais contra `install.sh` (CRLF, UTF-8 largo, 100 KB de dígitos), leitura de
`internal/generators/update.go`, `npm/src/commands/update.js`, `pypi/trackfw/commands/update.py`,
`internal/discover/discover.go`, e 5 reproduções ao vivo com binário compilado desta branch
(Go/Node/Python × symlink vivo em `update`, Go × `--dry-run` sobre symlink vivo, Go × symlink
pendurado em `discover --init`) em diretórios de scratchpad isolados do repositório.

**Fronteiras mantidas:** nenhum arquivo de código de produto tocado; nenhum commit, push ou PR
executado; nenhum outro arquivo além desta seção apensada a este roadmap — inclusive
`docs/agents-working-context.md`, cuja atualização normalmente seria obrigatória por este papel, mas
que fica deliberadamente pulada aqui por instrução explícita do arquiteto de não tocar em nenhum
outro arquivo (colisão com `hefesto-tf` revisando em paralelo no mesmo roadmap).

#### Resolução da barreira final (trackfw_architect, 2026-08-28)

**Os dois pareceres REPROVARAM, e cada um pegou algo que o outro não veria.**

**ML-3C** (de `hefesto-tf`) — `internal/generators/update.go:1811-1819` afirmava que o CLI Python
implementa superfície reduzida e aponta o usuário para Go/Node. Falso desde o ML-2C **desta REQ**.
Era a **terceira** aparição da mesma alegação obsoleta: já apagada de `cli-parity.md` (AC16) e
reescrita em `check-doctor-parity.sh:404` (ML-3B). Comentário obsoleto não quebra teste nenhum —
nenhum gate pega, só leitura pega. Corrigido, diff só de comentário. Junto, o literal `"7.3.0"`
congelado em `ci_workflow_version_pin_test.go:104` virou derivado de `version.Version`; ele passaria
hoje **mesmo com a versão hardcoded no gerador**.

**ML-3D + ML-3E** (de `hades-tf`) — **escrita fora do projeto através de symlink**. Reproduzido pelo
arquiteto com o binário da branch: projeto com `ci: none`, symlink vivo em
`.github/workflows/trackfw-validate.yml`, `trackfw update` sobrescrevia o alvo do link fora da
árvore. Nenhum dos 3 runtimes usava `lstat`. **O gatilho foi ampliado por uma regra de aceite minha**
— o AC17(c), que trocou a ativação do alvo de `cfg.ci` para presença em disco. Cobertura maior é
superfície maior, e nem eu nem a Wave 0 vimos: ela olhava o `install.sh`, e o vetor só nasceu dois
microlotes depois. Corrigido com `lstat` + recusa explícita nos 3.

O ML-3E foi necessário porque a primeira entrega do Node era **segura mas muda** — o alvo deixava de
ser declarado e nada era dito. Silêncio aqui vira *"o update não atualizou meu workflow e não falou
nada"*, que é a mesma classe de falha invisível que esta REQ combateu. Mensagem agora byte-idêntica
nos 3.

**Auditoria final do arquiteto, com os 3 CLIs reais:**

| | vítima do symlink | stderr |
|---|---|---|
| Go | intacta | avisa |
| Node | intacta | avisa |
| Python | intacta | avisa |

`diff` das três mensagens: idênticas. Symlink pendurado + `discover --init`: zero arquivos criados
fora do projeto nos 3.

```
make quality (TRACKFW_DISABLE_EXTERNAL_COMMANDS=1)   exit 0, 0 erros de make
barrier --wave 3                                     passed nos 4 checks
validate                                             16 warnings, 0 violations
```

**Duas notas de vault escritas e linkadas no índice:**
`vault/notes/update-segue-symlink-e-escreve-fora-do-projeto-2026-08-28.md` e
`vault/notes/barrier-so-casa-cabecalho-de-aceite-em-portugues-2026-08-28.md`.

**Achado colateral, virou REQ própria:** o `barrier` só casa `**Critérios de aceite:**` enquanto os
3 geradores escrevem `**Acceptance criteria:**` — todo roadmap que a ferramenta gera é reprovado pelo
próprio `barrier`. A paridade entre os 3 CLIs está intacta (os três erram igual), e é exatamente por
isso que nenhum gate pega: eles medem se as implementações concordam entre si, não se o contrato
gerador↔verificador fecha. Contornado neste roadmap trocando os 6 cabeçalhos.
