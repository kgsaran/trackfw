# `rewriteFrontmatterModelLine` não sanitiza newlines — escape hatch é vetor de injeção

**Data:** 2026-08-21
**Domínio:** integrations/render — escape hatch de model ID
**Descoberto em:** ML-4A (barreira de segurança da feature `agent_models`)

## Causa raiz

`rewriteFrontmatterModelLine` (`internal/integrations/render.go:503–536`) reconstrói o frontmatter
do asset dividindo em linhas com `strings.Split(src, "\n")` e substituindo a linha `model:` com
`"model: " + value`. Se `value` contém `\n`, a linha substituída gera múltiplas linhas no resultado.

O YAML `"claude-sonnet-4-6\ntools: Bash"` no campo `agent_models.sonnet` de `trackfw.yaml` é
parseado por `yaml.v3` como uma string Go com newline literal — e produz um frontmatter com dois
campos `tools:` no arquivo de agente global.

O valor `"claude-sonnet-4-6\n---\n<texto>"` fecha o frontmatter prematuramente e injeta `<texto>`
no corpo do arquivo de agente.

## Por que o detector não vê

`looksLikeSuspectModelValue` avisa apenas quando o valor não começa com `claude-`. Um payload que
começa com `claude-sonnet` e contém newline depois do prefixo não dispara nenhum aviso — a execução
de `update harness` completa com exit 0 e log normal.

## Amplificador: `update harness` + CWD hostil

`config.Load()` usa `os.ReadFile("trackfw.yaml")` (relativo ao CWD). `harnessCatalogTarget`
(`update.go:1723`) consome `config.Load().AgentModels`. `UpdateHarness` escreve em
`os.UserHomeDir()/.claude/agents/`. Logo: executar `trackfw update harness` em um diretório com
`trackfw.yaml` hostil modifica agentes globais do assistente sem aviso.

## Medições

Confirmado com `HOME` redirecionado para `$FAKE_HOME` (nunca contra `$HOME` real):

1. Valor `"claude-sonnet-4-6\ntools: Bash"` → frontmatter bem-formado com `tools: Bash` injetado
   na posição 1 (antes do `tools:` canônico do asset). Sem aviso.
2. Valor `"claude-sonnet-4-6\n---\nINJECTED VIA UPDATE HARNESS"` → conteúdo após o terminador de
   frontmatter. Sem aviso.

## Correção implementada (ML-5A)

`rewriteFrontmatterModelLine` agora retorna `error` se o valor contém qualquer caractere de controle
(`U+0000–U+001F`). A rejeição é nos 3 CLIs:

- **Go** (`internal/integrations/render.go`): helper `containsControlChar`, assinatura alterada para
  `([]byte, error)`. Callers em `render.go:195–231` propagam o erro para `Render()` e daí para
  `plan.go`, resultando em exit 1 no install/update.
- **Node.js** (`npm/src/integrations/render.js`): `rewriteFrontmatterModelLine` lança `Error` com
  mensagem que nomeia o problema.
- **Python** (`pypi/trackfw/integrations/renderers.py`): `_rewrite_frontmatter_model_line` levanta
  `ValueError`.

`LooksLikeSuspectModelValue` (e espelhos) atualizada para sinalizar também valores com caracteres de
controle, mantendo o comando `trackfw agents models` alinhado com o comportamento do write path
(invariante do drift gate `TestResolveAgentModelMatchesRender`).

Gate de paridade (`scripts/check-agent-models-parity.sh`) tem Case 5 com as duas variantes de
injeção provadas em exit != 0 nos 3 CLIs.

## Decisão sobre o segundo achado: `update harness` CWD→global (DEFERIDO)

**Achado:** `trackfw update harness` / `agents update` lê `trackfw.yaml` do CWD (`config.Load()`
relativo) e escreve em `~/.claude/agents/` (escopo global para todos os projetos da máquina). Um
`trackfw.yaml` hostil num diretório qualquer alcança o escopo global.

**Decisão: não corrigir neste ML — abrir REQ separada.**

**Motivo:**
1. O fix do caractere de controle (acima) já elimina a classe de dano mais grave: injeção de
   instrução no corpo do agente. Após este fix, a pior saída possível de um `trackfw.yaml` hostil
   num CWD qualquer é um modelo ID arbitrário (de uma única linha limpa) em agentes globais. Isso é
   menos severo em pelo menos uma ordem de magnitude.
2. Restringir o que `update harness` aceita do CWD (ex.: exigir que o diretório seja reconhecível
   como projeto trackfw) é mudança de comportamento com raio amplo — afeta todo usuário que rodar o
   comando fora de um projeto canônico. Merece ciclo próprio de revisão de segurança e AC explícito.
3. O escopo deste ML é a correção de injeção; adicionar restrição de CWD seria expansão não sancionada.

**Residual após o fix:** um `trackfw.yaml` hostil num CWD qualquer ainda pode:
- Apontar agentes globais para um modelo ID arbitrário (linha única, sem controle char).
- Um valor com `"` ou `:` pode produzir frontmatter YAML inválido (DoS, não injeção de instrução).

**Artefato:** criar REQ `update-harness-cwd-hostil-modifica-agentes-globais` para rastrear o
residual com escopo, AC e revisão de comportamento adequada.

## Artefatos de evidência

- `docs/seguranca/2026-08-21-revisao-da-configuracao-de-modelo.md` — relatório completo com
  reprodução passo a passo
