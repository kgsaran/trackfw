---
status: done
date: 2026-08-11
author: "Hades (Segurança)"
---

# Revisão de segurança — resolução de caminho dos hooks de agente independente do cwd (ML-8B)

> ML-8B do roadmap
> `docs/roadmaps/wip/ROADMAP-2026-08-11-resolucao-de-caminho-dos-hooks-de-agente-independente-do-cwd.md`
> Branch: `fix/resolucao-de-caminho-dos-hooks-de-agente-independente-do-cwd`
> ADR de referência:
> `docs/adr/ADR-2026-08-11-resolucao-de-caminho-dos-hooks-de-projeto-por-cli-mecanismo-especifico-do-fornecedor-sem-caminho-absoluto.md`
> (inclusive Emenda 1). Nota de vault:
> `vault/notes/codex-hooks-de-projeto-so-rodam-em-projeto-trusted-2026-08-11.md`.

Esta revisão é **puramente de leitura**: nenhum arquivo de código foi tocado por este agente.
Achados são reportados a Zeus, que decide e despacha correção.

## Escopo revisado

`git diff main...HEAD -- internal/ npm/src/ pypi/trackfw/` — 6 arquivos, 600 inserções/144 remoções:
`internal/generators/agentfiles.go`, `internal/generators/agentfiles_test.go`,
`internal/generators/credential_guard_dedup_test.go`, `internal/generators/hooks_test.go`,
`npm/src/generators/hooks.js`, `pypi/trackfw/generators/hooks.py`. Nenhuma mudança em
`scripts/trackfw-*.sh`, em `internal/generators/update.go`/`update-harness.js`/`update_harness.py`
(escopo global), nem em nenhum arquivo fora de geradores/testes — confirmado por `git diff --stat`.

---

## Q1 — Injeção via substituição de shell no Codex

**Veredito: OK.**

A string emitida é `"$(git rev-parse --show-toplevel)/scripts/trackfw-<script>.sh"`, com aspas
duplas literais envolvendo toda a expansão. Análise:

- O resultado de uma substituição de comando (`$(...)`) é inserido como **dado literal** no ponto de
  execução do shell; ele **não é re-escaneado** em busca de limites de aspas, `$`, backticks, `;`
  ou novas linhas. A decisão de onde a aspa dupla que abre `"$(...)/..."` fecha é tomada pelo shell
  ao **parsear o texto-fonte estático** do comando (antes de qualquer expansão) — não ao processar a
  saída dinâmica do `git rev-parse`. Isso vale mesmo que o nome de um diretório contenha `"`,
  `$(rm -rf ~)`, ou qualquer outro metacaractere: o conteúdo entra como texto opaco no caminho de
  arquivo resultante, na pior hipótese produzindo um caminho inválido (arquivo não encontrado), nunca
  uma segunda rodada de interpretação de shell.
- **Prova empírica, não só inferência**: o ML-3A rodou o `codex-cli 0.147.0` real (não um shell
  isolado) a partir de um subdiretório de um repositório de fixture e confirmou que o script disparou
  — ou seja, o executor do Codex de fato invoca isso via shell e tolera as aspas literais embutidas
  (ADR, Emenda 1, "Prova obtida"). Isso responde à pergunta "as aspas literais são suficientes?": sim,
  e a prova não é apenas teórica.
- **`GIT_DIR`/`GIT_WORK_TREE`, worktree, submódulo**: podem fazer `git rev-parse --show-toplevel`
  resolver para uma raiz diferente da esperada (ver Q3 abaixo — é um vetor de **disponibilidade**,
  não de injeção; a saída continua sendo um caminho de arquivo, nunca código executado).
- **Item genuinamente novo introduzido por esta mudança**: antes, o hook invocava diretamente um
  caminho; agora, toda disparada do hook invoca **`git`** (que por sua vez lê `.git/config`,
  possivelmente `core.fsmonitor`, hooks de núcleo do git, etc.) antes de invocar o script do trackfw.
  Um `.git` fornecido fora de um `git clone` normal (ex.: um tarball com um `.git/config` malicioso
  contendo `core.fsmonitor` ou similar apontando para um binário) é um vetor de execução conhecido do
  próprio git, não deste roadmap. **Este vetor é dominado**: para ele disparar, o usuário já precisa
  ter um `.codex/hooks.json` versionado daquele repositório **e** o projeto marcado como
  `trust_level = "trusted"` em `~/.codex/config.toml` (Q3) — nesse ponto, comandos arbitrários
  escritos pelo autor do repositório em `.codex/hooks.json` já rodam sem depender de nenhum truque de
  `git rev-parse`. Não é uma superfície nova de ataque relevante; registrado por completude.

**Conclusão Q1**: sem cenário viável de execução de comando arbitrário além do que já é inerente a
versionar `.codex/hooks.json` (que já era verdade antes deste ML, com o caminho relativo).

---

## Q2 — Expansão de variável em Claude/Gemini

**Veredito: OK**, com um residual de baixa severidade registrado.

- `$CLAUDE_PROJECT_DIR` e `$GEMINI_PROJECT_DIR` são definidos pelo **processo do CLI**, não pelo
  conteúdo do repositório — confirmado pela pesquisa do ML-0A (Claude: "both export them as the
  environment variables... on the spawned process"; Gemini: `GEMINI_PROJECT_DIR` documentado como
  "The absolute path to the project root", distinto de `GEMINI_CWD`). Não há caminho em que o
  **repositório** influencie o valor dessas variáveis — diferente do caso Codex (Q1), onde a raiz é
  derivada de `.gitmodules`/estrutura do próprio repo em cenário de submódulo.
- **Degradação sob variável indefinida**: se `$CLAUDE_PROJECT_DIR` ou `$GEMINI_PROJECT_DIR`
  expandirem para vazio, o comando vira `/scripts/trackfw-<script>.sh` — um caminho absoluto na raiz
  do **sistema de arquivos**, não do projeto. Isso não é "algo perigoso" no sentido de apontar para um
  script controlável por terceiro: nenhuma parte não-privilegiada consegue plantar um arquivo em
  `/scripts/`. A degradação é sempre **fail-to-run** (arquivo não encontrado), nunca
  fail-to-wrong-script. Ver Q3 para a consequência disso no guard.
- **Assimetria observada na pesquisa do ML-0A, vale registrar**: a doc do Cursor marca
  `CURSOR_PROJECT_DIR` explicitamente como `"Always Present": Yes`. Não há citação equivalente de
  "sempre presente" para `$CLAUDE_PROJECT_DIR` nem para `$GEMINI_PROJECT_DIR` na pesquisa do ML-0A —
  o mecanismo de expansão está confirmado (com exemplos oficiais), mas a garantia de que a variável
  está **sempre** setada em toda invocação de hook não foi citada literalmente para esses dois CLIs.
  Risco residual baixo: mesmo se a garantia não for absoluta, o modo de falha (visto acima) é
  fail-to-run, e não é pior em natureza do que o caminho relativo puro que ele substitui — que também
  falhava (de forma mais frequente: qualquer `cd`) antes desta mudança.

**Conclusão Q2**: nenhum caminho em que o repositório controle o valor; degradação sob variável
indefinida é sempre falha de execução, nunca resolução para script de terceiro.

---

## Q3 — Falha silenciosa do credential-guard, por CLI (pergunta mais importante)

**Veredito consolidado: RISCO ACEITÁVEL — nenhuma regressão em relação à `main`, com dois residuais
documentados e um gap de verificação explicitamente não fechado por este roadmap.**

Duas coisas diferentes estão em jogo e não devem ser confundidas: (i) o **guard nunca ser
disparado** porque o hook não está cadastrado/carregado, e (ii) o **guard ser disparado mas falhar
ao executar** (comando não encontrado) e o CLI, mesmo assim, permitir a ação — isto é, se a falha de
execução do hook é fail-*aberto* ou fail-*fechado* no runtime de cada fornecedor. Nenhuma fonte
consultada neste roadmap (pesquisa do ML-0A, ADR, doc primária) responde a semântica de falha de
hook por CLI — ela documenta cwd e placeholders, não o que acontece quando o comando do hook não
roda. **Não vou inferir essa semântica.** A resposta fundamentada, por critério de aceite, é:
**este roadmap não altera o tratamento de falha de hook de nenhum CLI** — o comportamento
fail-aberto/fail-fechado de cada fornecedor é uma propriedade do runtime dele, idêntica antes e
depois desta mudança, e permanece **não verificada** por esta revisão. Registro isso como item de
follow-up, não como bloqueio desta branch (ver `docs/cli-parity.md`, que já documenta o mecanismo
mas não a semântica de falha — vale uma nota lá também, fora do escopo deste ML de segurança que não
edita código nem essa doc).

Por CLI:

**Claude Code — sem mudança de risco.** O credential-guard do Claude já usava
`$CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh` desde `0c66ecb` (v6.7.1), **antes** deste
roadmap. Este ML só estende o mesmo mecanismo, já provado em produção, aos hooks de
attention-signal/cleanup — que **não** são controle de segurança (sinalização de UI para o board).
Nenhum entry de credential-guard do Claude foi alterado (confirmado por `git diff`: as linhas do
guard mudam apenas de `migrateClaudeHookCommand` para `migrateHookCommand`, mesmo comportamento,
generalização de nome). **Sem regressão.**

**Codex — sideways move, dois casos de falha novos, ambos estreitos e um deles já documentado.**
Antes: caminho relativo puro, que já falhava sempre que a sessão Codex era iniciada de um
subdiretório (mesma classe de bug do Claude pré-`0c66ecb`) — ou seja, o guard **já podia deixar de
rodar** silenciosamente sob essa condição. Depois: `"$(git rev-parse --show-toplevel)/..."` corrige
esse caso, mas introduz dois casos que não existiam antes:
1. **Projeto Codex sem `.git`** (raro para o caso de uso do trackfw, mas possível): `git rev-parse`
   falha, a substituição vira vazio, o comando degrada para `/scripts/trackfw-credential-guard.sh`
   (Q2) → guard não executa. Este caso **já está documentado** — `docs/cli-parity.md` (edição em
   andamento do ML-8A) registra: *"O comando falha fora de um repositório git — aceitável, pois o
   trackfw governa repositórios por definição."* Não é um gap não coberto, é um risco aceito e
   registrado pelo próprio roadmap.
2. **`GIT_DIR`/`GIT_WORK_TREE` setados no ambiente da sessão**: podem redirecionar
   `--show-toplevel` para fora do projeto, produzindo o mesmo efeito (caminho não encontrado →
   guard não roda). Não documentado explicitamente em `docs/cli-parity.md` hoje; registro aqui como
   residual a acrescentar. Requer controle do ambiente da sessão Codex, que já é um nível de acesso
   maior que "controla o repositório" (fora do modelo de ameaça deste ADR, que trata de repositório
   hostil, não de máquina/ambiente hostil).
🔴 **Achado separado, já conhecido e aceito pelo ADR — fail-aberto por design do fornecedor, não
deste roadmap**: hooks de projeto do Codex só carregam se o projeto estiver `trusted` em
`~/.codex/config.toml` (Emenda 1). Em projeto não-trusted, **nenhum** hook roda — nem o antigo, nem
o novo, nem o de attention, nem o de guard — silenciosamente. Isso é **anterior** a este roadmap e
**não piora** com a mudança (o antigo caminho relativo também dependia de trust); registrado no ADR
Emenda 1 e no vault. Não é um "caminho novo" no sentido pedido pelo critério de aceite — é uma
pré-condição do fornecedor que já existia.

**Gemini — sideways move na mesma direção que Claude, sem prova empírica (aceito pelo ADR por
argumento de assimetria).** Antes: caminho relativo puro (mesma classe de bug potencial). Depois:
`$GEMINI_PROJECT_DIR/...`. Diferente do Codex, este mecanismo **não pode piorar**: `$GEMINI_PROJECT_DIR`
resolve para a raiz do projeto independentemente de o cwd ter ou não derivado (não há pré-condição de
"estar em repo git" nem de "trust" documentada). O único residual é a assimetria de Q2 (variável
"sempre presente" não citada explicitamente) — baixo risco, mesma classe de degradação fail-to-run.

**Cursor — sem mudança, veredito `OK` do ML-0A.** cwd de hooks de projeto é fixo na raiz por design
do fornecedor ("Project hooks... Run from the project root"); caminho relativo já resolve
corretamente. Nenhum caminho novo de falha silenciosa introduzido nem preexistente quanto a
resolução de caminho — o guard do Cursor não depende de expansão nem de shell.

**GitHub Copilot CLI — sem mudança, mecanismo mais robusto dos 6.** Usa o campo nativo estruturado
`"cwd": "."`, não uma string com placeholder — não há expansão de variável nem substituição de shell
envolvida, então não há classe de falha "variável indefinida" ou "comando não encontrado por cwd
errado" aplicável aqui. Confirmado nos 3 stacks que o Copilot já emitia isso antes deste roadmap
(ML-0A, achado que reduziu o escopo de 6 para 3 CLIs a alterar).

**Kiro — sem mudança, dívida conhecida e explicitamente aceita, não deste roadmap.** Continua com
caminho relativo puro e comportamento de cwd **não verificável** em doc primária (`INDETERMINADO`).
Isso significa que, **se** o cwd de hooks do Kiro for dinâmico (hipótese não confirmada nem
descartada), o credential-guard do Kiro **já pode estar** sujeito à mesma classe de falha silenciosa
que motivou o `0c66ecb` no Claude — só que nunca verificada. Este é um gap **pré-existente**, não
introduzido por este ML, e está registrado como risco aceito no ADR (§Consequences, "Kiro fica com
dívida conhecida... Registrado, não resolvido") e no roadmap (default para `INDETERMINADO`: não
alterar). Não bloqueia esta branch — é um item de follow-up para uma futura verificação empírica do
Kiro, fora do escopo negativo explícito deste roadmap.

**Resumo Q3 (6/6 CLIs cobertos):**

| CLI | Guard alterado neste ML? | Falha silenciosa nova? | Observação |
|---|---|---|---|
| Claude | Não (já era `$CLAUDE_PROJECT_DIR` desde 0c66ecb) | Não | Só attention mudou, não é controle de segurança |
| Codex | Sim | Dois casos estreitos (não-git; `GIT_DIR`/`GIT_WORK_TREE`), um já documentado em cli-parity.md | Trust-gate do fornecedor é pré-existente, não deste ML |
| Gemini | Sim | Não (mecanismo não pode piorar por construção) | Assimetria "always present" não citada — residual baixo |
| Cursor | Não | Não | Mecanismo nativo, cwd fixo |
| Copilot | Não | Não | Mecanismo nativo `cwd:"."`, sem expansão |
| Kiro | Não | Dívida pré-existente, não verificável | Aceita explicitamente no ADR; fora do escopo negativo |

---

## Q4 — Migração in-place pode sobrescrever entrada customizada do usuário?

**Veredito: RISCO ACEITÁVEL (padrão pré-existente, não introduzido por este ML).**

`migrateHookCommand`/`_migrate_hook_command` casa por **igualdade exata** de `matcher` **e**
`command` contra a string legada literal do trackfw (ex.: `scripts/trackfw-credential-guard.sh`).
Só reescreve entradas cujo comando seja **byte-idêntico** ao literal antigo gerado pelo próprio
trackfw. Para uma entrada verdadeiramente customizada pelo usuário ser afetada, o usuário precisaria
ter escrito, por coincidência ou cópia, um hook com o mesmo matcher e exatamente o mesmo comando
relativo que o trackfw usava — nesse ponto a entrada é indistinguível de uma gerada pelo trackfw.
Este é o **mesmo modelo de confiança** já usado desde `migrateClaudeHookCommand` (a versão anterior,
específica do Claude, existente antes deste roadmap) — o roadmap generalizou o helper para Codex e
Gemini e estendeu a 2 comandos novos (signal/cleanup), mas não mudou a estratégia de match nem
introduziu um modo novo de colisão.

**Corrupção de JSON**: não. O helper só faz `hObj["command"] = newCommand` em um `map[string]interface{}`
já parseado — nenhuma reestruturação, nenhuma escrita de string bruta. Sem risco de corromper o
arquivo.

**Assimetria notada no Python (Codex)**: `_migrate_hook_command` para o guard do Codex roda
**incondicionalmente**, fora do `if not skip_cg:` que envolve o `_merge_codex_hook_entry`
correspondente (mesmo padrão em Node/Go). Isso é **benigno**: a entrada de projeto obsoleta, se
existir, já estava presente no arquivo **antes** desta mudança (nada a remove quando o guard global é
instalado — o dedup apenas evita **adicionar** uma nova). A migração rodar de qualquer forma só
corrige a string dessa entrada preexistente; não cria uma entrada nova nem duplica nada. Pior caso: o
guard dispara duas vezes (global + projeto) quando ambos coexistem — reforço redundante de um
controle de bloqueio, não enfraquecimento.

---

## Q5 — Supply chain / permissões

**Veredito: OK.**

- Nenhuma mudança em `scripts/trackfw-*.sh` (conteúdo dos scripts intocado — confirmado por
  `git diff --stat`, escopo negativo do roadmap).
- Nenhuma mudança em quem grava os arquivos de settings (mesmos geradores, mesma permissão de
  arquivo `0644`/equivalente já usada antes).
- O único componente **novo** invocado a cada disparo do hook do Codex é o binário `git` do sistema,
  resolvido via `PATH` do processo que executa o hook — mesma superfície que qualquer outro uso de
  `git` já feito pelo próprio trackfw/pelo usuário nesse ambiente; não é uma dependência nova
  introduzida pelo trackfw (o `git` já é pré-requisito do próprio modelo de governança do trackfw).
  Dominado pelo mesmo argumento de Q1: exige projeto já `trusted` e `.codex/hooks.json` já
  versionado, ponto em que o repositório já tem controle sobre o que executa.

---

## Nota de processo

`docs/cli-parity.md` aparece como modificado (`git status --porcelain`) — é o artefato do ML-8A
(Hefesto), rodando em paralelo a este ML-8B por tocar arquivos disjuntos (ML-8A só `docs/cli-parity.md`;
ML-8B não modifica nenhum arquivo de código). Não é um desvio de escopo deste agente. Os únicos
arquivos tocados por Hades neste ML são: este arquivo (`docs/seguranca/2026-08-11-revisao-hooks-cwd.md`),
`docs/agents-working-context.md`, e o campo `**Status:**` do ML-8B no roadmap.

---

## Conclusão

**Nenhum achado bloqueia o avanço desta branch para PR do ponto de vista de segurança.** Não há
enfraquecimento de controle em relação à `main`:

- Q1 (injeção Codex): `OK`, com prova empírica de que a substituição não é re-interpretada.
- Q2 (expansão Claude/Gemini): `OK`, degradação sempre fail-to-run, nunca fail-to-wrong-script.
- Q3 (falha silenciosa do guard, 6 CLIs): `RISCO ACEITÁVEL` — nenhuma regressão; dois casos de falha
  novos e estreitos no Codex (um já documentado); a semântica de fail-aberto/fail-fechado por hook
  não é alterada por este roadmap e permanece **não verificada** — registrado como follow-up, não
  como bloqueio.
- Q4 (migração in-place): `RISCO ACEITÁVEL` — modelo de match pré-existente, sem mudança de
  estratégia; assimetria `skip_cg` no Codex é benigna.
- Q5 (supply chain): `OK`.

**Recomendação: seguir para PR.** Follow-up sugerido para roadmap futuro (não bloqueante): (a)
registrar em `docs/cli-parity.md` o caso `GIT_DIR`/`GIT_WORK_TREE` do Codex ao lado do caso
"fora de repo git" já documentado; (b) uma verificação empírica dedicada da semântica de
fail-aberto/fail-fechado de hook por CLI (Claude, Codex, Gemini) — item novo, fora do escopo deste
roadmap; (c) revisitar o veredito `INDETERMINADO` do Kiro caso surja doc primária ou teste
reproduzível no futuro.
