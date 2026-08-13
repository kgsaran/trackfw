---
title: check-integration-assets.sh so verifica paridade entre stacks, nao conteudo; o caminho de geracao (incl. identidade) propaga o asset fielmente
date: 2026-08-13
tags: [agents, integrations, generation-path, identity, verificacao]
---

## Contexto

REQ `docs/req/REQ-2026-08-13-fronteira-de-escrita-dos-agentes-auditores-e-coerente-com-as-ferramentas-concedidas.md`,
roadmap `ROADMAP-2026-08-13-fronteira-de-escrita-dos-agentes-auditores.md`. ML-1A editou os 3 assets
de agentes auditores (`code-quality`, `security`, `ux`) em `{internal,npm/src,pypi/trackfw}/integrations/
assets/agents/`. ML-2A verificou se o artefato **gerado** (o que o usuario recebe em `~/.claude/agents/`)
reflete a mudanca.

## Achado nao obvio (duas metades, um so risco)

**Metade 1 (achado de Zeus na auditoria do ML-1A):** `check-integration-assets.sh` — o gate que roda
em `make quality` — verifica **paridade byte-a-byte entre os 3 stacks**, nao **conteudo**. E possivel
alterar o texto de um agente sem nenhum gate acusar, desde que os 3 stacks (`internal/`, `npm/src/`,
`pypi/trackfw/`) mudem juntos. Nao ha teste que fixe o conteudo esperado do asset — so testes de
sincronia entre stacks.

**Metade 2 (achado deste ML-2A):** para o alvo `claude` (`surface: cli`, `representation: subagent`,
`support_level: native`), o **caminho de geracao** (`trackfw agents install`/`update`), incluindo o
caminho de **identidade aplicada** (`~/.trackfw/identity.json` → renomeia `name`/`description` e
insere uma linha de saudacao no corpo), **propaga o asset fielmente** — nao ha camada de substituicao
de template que introduza drift entre o asset-fonte e o arquivo instalado, para esse alvo/
representacao. Confirmado com `diff` entre o arquivo recem-gerado (com a identidade real da maquina)
e o arquivo ja instalado antes da mudanca: a unica diferenca sao exatamente as 4 linhas do ML-1A
(`tools:`, `Reporting boundary`, `Governance prerequisite`, `Git authority`) — front-matter (`name`,
`description`, `model`, `memory`) identico.

**Escopo nao coberto por este achado:** o catalogo tem ~10 targets, e `support_level`/
`representation` sao campos por-target justamente porque nem todo alvo e um subagente nativo — alguns
tem representacao transformada (ex.: regras/prompt embutido em vez de arquivo de agente separado).
Este achado **nao** cobre esses mapeamentos; so foi verificado `claude`/`cli`/`subagent`. Para um alvo
nao-nativo, reverificar a fidelidade da geracao antes de assumir que ela se aplica igual.

**Consequencia pratica:** as duas metades juntas significam que, para `claude`/`cli`/`subagent`,
**verificar o asset-fonte e suficiente** — nao e preciso auditar separadamente o pipeline de geracao/
identidade a cada mudanca de asset nesse alvo, porque ele nao introduz nem esconde divergencia. Mas
tambem significa que **o unico gate real de conteudo e a leitura humana/agente do diff** — `make
quality` passa mesmo se o texto de um agente regredir, contanto que regrida igual nos 3 stacks.

**Bonus confirmado (`agents update`, sem `--force`):** um `$HOME` isolado com o arquivo antigo
instalado (nao modificado pelo usuario, hash batendo com o manifesto) + `trackfw agents update`
(sem `--force`) **substitui** o arquivo pelo novo conteudo normalmente — nao exige `--force`.
`--force` so e necessario se o usuario tiver editado manualmente o arquivo instalado (ai o manifesto
acusa "modified managed artifact" e bloqueia por seguranca). Ou seja: nesta maquina, um simples
`trackfw agents update` (ou `install`) ja refresca os 3 auditores para o texto novo — nao ha passo
manual extra alem de rodar o comando.

## Como foi confirmado

`$HOME` isolado com `~/.trackfw/identity.json` real copiado (nunca o `~/.claude/` real):

```bash
H3=<tmp>/home3
mkdir -p "$H3/.trackfw" && cp ~/.trackfw/identity.json "$H3/.trackfw/"
HOME="$H3" ./bin/trackfw agents install --scope global --targets claude --items code-quality --json
diff "$H3/.claude/agents/trackfw-code-quality.md" ~/.claude/agents/trackfw-code-quality.md
```

`md5`+`mtime` do `~/.claude/agents/*.md` real capturados antes e depois de todas as instalacoes
isoladas — identicos, confirmando que nenhuma escrita vazou para fora do `$HOME` isolado.

## Licao para quem editar assets de agente no futuro

- `make quality` verde **nao** prova que o texto do agente esta correto — so que os 3 stacks
  concordam entre si. A leitura do diff continua sendo o gate de conteudo.
- Para verificar se uma mudanca de asset chega ao usuario, **nao** e preciso testar o pipeline de
  identidade a parte todo ML — ele so precisa ser reverificado se o proprio codigo de renomeacao/
  template (`internal/integrations/*identity*`, equivalentes em `npm/`/`pypi/`) mudar. Se so o texto
  do asset mudou, comparar o asset-fonte com o gerado num `$HOME` isolado (sem preset de identidade)
  ja e evidencia suficiente.
- Sempre usar `$HOME` isolado (nunca escrever no `~/.claude/` real) e confirmar com `md5`+`mtime`
  antes/depois — padrao ja usado em
  [vies-do-tmp-ao-medir-sandbox-de-agente-2026-08-12](vies-do-tmp-ao-medir-sandbox-de-agente-2026-08-12.md)
  e
  [check-agent-hooks-parity-unisolated-home-false-failure-2026-08-08](check-agent-hooks-parity-unisolated-home-false-failure-2026-08-08.md).
