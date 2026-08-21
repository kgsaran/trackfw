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

## Extensão ML-5C: U+2028 e U+2029 (LINE/PARAGRAPH SEPARATOR)

**Descoberto em:** ML-5B (reverificação de barreira). Corrigido em ML-5C.

### Por que `< 0x20` era insuficiente

`containsControlChar` usava limite `s[i] < 0x20`. U+2028 (LINE SEPARATOR) e U+2029
(PARAGRAPH SEPARATOR) têm bytes UTF-8 `0xE2 0x80 0xA8` / `0xE2 0x80 0xA9`, todos `>= 0x80`.

### Comportamento do parser

Medido diretamente com `go run` contra `gopkg.in/yaml.v3`:

```
value configurada em trackfw.yaml: "claude-sonnet-4-6\xe2\x80\xa8tools: Bash"
valor parseado por yaml.v3 (Go string): "claude-sonnet-4-6 tools: Bash" (len=31, rune[17]=U+2028)
```

yaml.v3 **preserva** U+2028 no valor Go string (não converte para `\n` nem para espaço).
`rewriteFrontmatterModelLine` escreve `model: claude-sonnet-4-6<U+2028>tools: Bash`.

Parsers de frontmatter que dividem em `\n`/`\r\n` veem isso como uma linha válida com ID inválido.
Parsers YAML 1.2 que tratam U+2028 como separador de linha podem dividir em nova chave — injeção estrutural especulativa.

### U+0085 (NEL) — excluído, com medição

yaml.v3 **normaliza** U+0085 para espaço antes de passar o valor para Go.
O resultado é `claude-sonnet-4-6 tools: Bash` (espaço literal, não U+0085).
Sem injeção estrutural; espaço é inócuo para parsers de frontmatter baseados em linha.
U+0085 **não** foi adicionado ao check — justificativa por evidência, não por precaução.

### Correção implementada (ML-5C)

`containsControlChar` trocou o loop de byte `s[i] < 0x20` por loop de rune:

```go
for _, r := range s {
    if r < 0x20 || r == ' ' || r == ' ' {
        return true
    }
}
```

O loop de rune é equivalente ao loop de byte para ASCII: bytes UTF-8 de continuação são sempre
`>= 0x80`, então `byte < 0x20` nunca pertence a sequência multi-byte. U+2028/U+2029 adicionados
pelo valor exato do rune. Identicamente nos 3 CLIs (Node.js via `charCodeAt`, Python via `ord`).

`LooksLikeSuspectModelValue` sinaliza automaticamente (delega a `containsControlChar`).

Gate: Case 5c em `scripts/check-agent-models-parity.sh` (fixture com U+2028 literal no YAML
double-quoted, 3 runtimes × 1 variante + vacuity guard).

## Artefatos de evidência

- `docs/seguranca/2026-08-21-revisao-da-configuracao-de-modelo.md` — relatório completo com
  reprodução passo a passo
- `docs/seguranca/2026-08-21-reverificacao-da-configuracao-de-modelo.md` — reverificação ML-5B,
  gap U+2028 identificado e medido
