# Reverificação de Segurança — ML-5B: Configuração de Modelo por Tier com Composição por Alvo

**Data:** 2026-08-21
**Branch:** `feat/versao-do-modelo-por-tier-com-composicao-por-alvo`
**Revisor:** Hades (Security Reviewer)
**Referência:** `docs/seguranca/2026-08-21-revisao-da-configuracao-de-modelo.md` (ML-4A)
**Fix aplicado em:** ML-5A (`internal/integrations/render.go`, espelhos Node.js e Python)

---

## VEREDITO: BLOQUEIO LEVANTADO COM RESSALVAS

O bloqueio emitido no ML-4A (injeção de bytes via newlines embarcados no escape hatch) está fechado.
Os dois exploits originais, medidos no ML-4A, foram medidos novamente contra o binário atual e
produzem `exit=1` sem modificar nenhum arquivo de agente. A ressalva nomeada abaixo é residual
conhecida, não reabre o bloqueio.

---

## O que foi medido

### Medição 1 — Exploit original: injeção de chave duplicada via `\n`

Payload hostil em `trackfw.yaml`:
```yaml
agent_models:
  sonnet: "claude-sonnet-4-6\ntools: Bash"
```

Resultado com o binário atual (`./bin/trackfw`, `HOME` redirecionado para `$FAKE_HOME`):
```
✗ claude-agents: failed (~/.claude/agents)
    - model value contains control character and was rejected: model IDs never require
      newlines or other control characters (got "claude-sonnet-4-6\ntools: Bash")
updated=0 skipped=0 missing=0 failed=1
exit=1
```

Arquivo de agente: intacto (`model: claude-sonnet-4-6` preservado). Nenhum byte de origem externa
foi escrito.

### Medição 2 — Exploit original: injeção de corpo via `\n---\n`

Payload hostil:
```yaml
agent_models:
  sonnet: "claude-sonnet-4-6\n---\nINJECTED VIA UPDATE HARNESS"
```

Resultado:
```
✗ claude-agents: failed (~/.claude/agents)
    - model value contains control character and was rejected: (got "claude-sonnet-4-6\n---\nINJECTED VIA UPDATE HARNESS")
exit=1
```

Arquivo de agente: intacto. Bloqueio confirmado.

### Medição 3 — Tab (0x09, ASCII control)

Valor `"claude-sonnet-4-6\ttools: Bash"` → `exit=1`, rejeitado com mesma mensagem. Coberto por
`containsControlChar` (0x09 < 0x20).

### Medição 4 — CR sozinho (0x0D, ASCII control)

O parser Go YAML normaliza `\r` literal em scalar de aspas duplas para espaço (conforme spec YAML:
quebra de linha em flow scalar dobra-se em espaço). Valor extraído: `"claude-sonnet-4-6 tools: Bash"`
(sem caractere de controle). O harness reportou `skipped` (arquivo não gerenciado pelo manifesto);
o arquivo de agente não foi modificado. Mesmo que o harness tentasse escrever, o valor
`claude-sonnet-4-6 tools: Bash` produz a linha `model: claude-sonnet-4-6 tools: Bash` — nenhuma
injeção estrutural (espaço é inócuo para parsers de frontmatter baseados em linha).

### Medição 5 — Caracteres estruturais ASCII imprimíveis (`:`, `"`)

Valores como `"claude-sonnet-4-6: bogus-key"` e `'claude-sonnet-4-6" injected: yes'` passam em
`containsControlChar` (printable ASCII, não controles). Mas não produzem injeção estrutural: o
resultado seria `model: claude-sonnet-4-6: bogus-key` — uma linha com valor inválido de model ID,
não uma chave YAML adicional. Impacto: model ID inválido (degradação de disponibilidade), não
injeção de instrução.

### Medição 6 — Paridade 3 CLIs

Todos os três CLIs foram testados diretamente com os casos acima:

| Caso | Go | Node.js | Python |
|------|-----|---------|--------|
| `\n` injection | REJECTED | REJECTED | REJECTED |
| `\n---\n` injection | REJECTED | REJECTED | REJECTED |
| Tab (0x09) | REJECTED | REJECTED | REJECTED |
| U+2028 (ver abaixo) | WRITTEN | WRITTEN | WRITTEN |
| Valor legítimo | WRITTEN | WRITTEN | WRITTEN |

Paridade completa. A implementação de `containsControlChar` é idêntica nos três CLIs: limite `< 0x20`
(excluindo exatamente U+0000–U+001F). Nenhuma divergência.

---

## Questão 2: cobertura completa ou só o que foi testado?

### Gap identificado: Unicode Line Separator U+2028 e Paragraph Separator U+2029

**Inferido e medido:**

O parser Go YAML preserva U+2028 e U+2029 como-são no valor extraído (não converte para `\n`). O
valor produzido pelo parser para `"claude-sonnet-4-6\xe2\x80\xa8tools: Bash"` é
`"claude-sonnet-4-6 tools: Bash"` (medido com `go run`).

`containsControlChar` verifica `s[i] < 0x20`. U+2028 é 0xE2 0x80 0xA8 em UTF-8 — os bytes
individuais (0xE2, 0x80, 0xA8) são todos >= 0x80. `containsControlChar` retorna `false`. O valor
passa pelo check e `rewriteFrontmatterModelLine` escreve a linha:

```
model: claude-sonnet-4-6<U+2028>tools: Bash
```

Medição direta com `rewriteFrontmatterModelLine` reimplementada em `go run`:

```
line[3]: "model: claude-sonnet-4-6 tools: Bash"
```

A linha não é dividida pelo `strings.Split(frontmatter, "\n")` — permanece uma única entrada de
`lines[]`. Nenhuma quebra estrutural de frontmatter no nível de construção do arquivo.

**Impacto real — inferido, não mensurável neste repositório:**

- Parsers que dividem linhas em `\n`/`\r\n` (padrão): o campo `model` terá valor
  `claude-sonnet-4-6 tools: Bash` — inválido como model ID, causando falha de carregamento
  (degradação de disponibilidade).
- Parsers YAML 1.2-compatíveis que tratam U+2028 como separador de linha: o frontmatter seria
  parseado como `model: claude-sonnet-4-6` + `tools: Bash` (nova chave). Isso seria injeção
  estrutural via U+2028.

O comportamento do loader de agentes da Anthropic (Claude Code) não é mensurável a partir deste
repositório. Parsers de frontmatter convencionais (gray-matter, front-matter) dividem em `\n`. O
risco de injeção via U+2028 depende de um parser YAML 1.2 ser usado no loader — cenário especulativo.

**`LooksLikeSuspectModelValue` não flageia U+2028.** O valor começa com `claude-` e não contém
ASCII control chars — retorna `false`. O usuário não recebe aviso.

**Severidade: MÉDIO (disponibilidade garantida; instrução injection especulativa).**

Essa lacuna existe identicamente nos 3 CLIs. Não é regressão da feature em revisão — é característica
do limite `< 0x20` da verificação. A correção seria estender a verificação para incluir U+2028 e U+2029
(ou qualquer código Unicode categorizado como Line_Separator/Paragraph_Separator), em ciclo próprio.

---

## Questão 3: os 3 CLIs recusam igual?

Sim. Medido diretamente. Ver Medição 6 acima. Não há divergência de comportamento entre Go, Node.js
e Python para nenhum dos casos testados.

---

## Questão 4: o deferimento do segundo achado (CWD→global) é defensável?

**Sim, é defensável.** Fundamento:

O dano de primeira ordem do achado ALTO (ML-4A §2) era: `trackfw.yaml` hostil no CWD permite
escrever bytes de origem externa em `~/.claude/agents/` via escape hatch sem sanitização. Com a
correção do ML-5A, a escrita de bytes de origem externa só é possível para valores que passem em
`containsControlChar` — ou seja, valores ASCII imprimíveis e Unicode sem controles. Esses valores
produzem no máximo um model ID inválido (degradação de disponibilidade), não instrução injection.

A escalada estrutural (CWD→HOME) permanece — qualquer valor imprimível do `trackfw.yaml` do CWD pode
ser escrito em arquivos globais. Mas sem um vetor de injeção de instrução, o impacto residual é
limitado a: model ID inesperado para os agentes da máquina do desenvolvedor. Isso é uma superfície
de ataque real, porém em nível de severidade que suporta um ciclo próprio.

O argumento do ADR-2026-08-12 (falso-positivo como risco de primeira ordem) reforça o deferimento:
restringir o CWD→global path mudaria o comportamento para todos os usuários legítimos de
`update harness` (que dependem dessa leitura do CWD).

---

## Questão 5: achado novo?

Sim, um: a lacuna de U+2028/U+2029 descrita na Questão 2. Não é bloqueante por si — o impacto de
instrução injection é especulativo e dependente de loader externo. Registrado como dívida nomeada.

Nenhum outro achado novo.

---

## Resumo Executivo

| Item | Status |
|------|--------|
| Exploit original \n (chave duplicada) | FECHADO — medido, exit=1, arquivo intacto |
| Exploit original \n---\n (body injection) | FECHADO — medido, exit=1, arquivo intacto |
| Tab (0x09) | FECHADO — coberto por < 0x20 |
| CR (0x0D) | FECHADO — YAML normaliza para espaço; espaço é inócuo |
| Paridade 3 CLIs | CONFIRMADA — comportamento idêntico |
| Deferimento CWD→global | DEFENSÁVEL — impacto residual aceitável sem vector de instrução |
| Lacuna U+2028/U+2029 | DÍVIDA NOMEADA — não bloqueante; ciclo próprio necessário |

O bloqueio original foi emitido corretamente. A correção do ML-5A fecha a classe de defeito
bloqueante. A lacuna Unicode é nova, de severidade menor, e não reabre o bloqueio.

---

**Hades — Reverificação concluída:** 2026-08-21
