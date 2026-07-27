# Princípios de Design de Gates Verificáveis

> Extraídos do ADR `docs/adr/ADR-2026-07-26-principios-de-design-de-gates-verificaveis.md`.
> Aplica-se a todo gate do trackfw: scripts de paridade em `scripts/check-*.sh`, regras do
> validator e verificações de CI.

---

## Por que esses princípios existem

O trackfw vende **governança verificável**. Seus gates são o produto — não infraestrutura acessória.
Em 2026-07-26, quatro defeitos apareceram em três REQs consecutivas e nenhum foi detectado pelo CI.
Todos os quatro são casos concretos deste repositório, documentados abaixo como exemplos vinculantes.

| # | Gate afetado | Defeito | Princípio violado |
|:-:|---|---|---|
| D1 | `check-integration-cli-parity.sh` | `assert len(items) == (10 if kind == "agents" else 5)` — número mágico | P1 |
| D2 | `check-cli-parity.sh` | `argparse` do Python 3.13+ colore a saída; `grep` casava na descrição, não no nome do comando | P3 |
| D3 | `trackfw ship` npm / PyPI | `roadmap_dir` com valor divergente do Go; testes com injeção não exercitavam o caminho real | P2 + P3 |
| D4 | `branch_has_wip_roadmap` | Só enxerga `wip/`; mover o roadmap para `done/` — como a Definition of Done exige — reprovava | P2 (semântica) |

Notas de vault relacionadas:
- [`argparse-ansi-parity-gate-python313-2026-07-26.md`](../vault/notes/argparse-ansi-parity-gate-python313-2026-07-26.md) — causa raiz e fix do D2
- [`branch_has_wip_roadmap-conflita-com-a-definition-of-done-2026-07-26.md`](../vault/notes/branch_has_wip_roadmap-conflita-com-a-definition-of-done-2026-07-26.md) — análise do D4
- [`ship-roadmap-dir-default-divergencia-2026-07-27.md`](../vault/notes/ship-roadmap-dir-default-divergencia-2026-07-27.md) — causa raiz e fix do D3

---

## P1 — Nenhum número mágico

**Regra:** contagens e listas esperadas derivam da fonte de verdade (`catalog.json`,
`KnownAgentIDs`, config). Nunca de uma constante no corpo do gate.

**Por quê:** um gate que precisa ser editado sempre que o produto cresce será esquecido.
O D1 é a prova: `assert len(items) == (10 if kind == "agents" else 5)` quebrou ao acrescentar
dois agentes ao catálogo. Quebraria de novo ao acrescentar skills. E quebrou silenciosamente
nas sessões seguintes — o CI estava verde.

**Como aplicar:**
- Em shell: ler o catálogo com `jq` ou `grep -c` sobre os arquivos de fonte, não `== 12`.
- Em Go: usar `len(catalog.Agents)` ou equivalente, nunca uma constante literal.
- Em Node.js / Python: idem — importar ou ler o catálogo, não um número fixo.
- Se a lista de exceções (`go_only_commands`) precisar crescer, ela é uma variável
  documentada no topo do script, não um valor espalhado pelo grep.

**Exemplo corrigido:** `check-integration-cli-parity.sh` agora lê os targets diretamente
de `$CATALOG_FILE` via `jq`, tornando a lista imune ao crescimento do catálogo.

---

## P2 — Falha explícita, nunca degradação silenciosa

**Regra:** se o gate não consegue obter o que precisa para verificar (arquivo ausente, comando
indisponível, parse falho, diretório vazio), ele **falha dizendo o porquê**. Um gate que degrada
para "sempre passa" é pior que um gate quebrado: quebrado é visível.

**Por quê:** o D3 ilustra o caso extremo. O runner do npm/PyPI resolvia `roadmap_dir` com um
valor divergente do Go. Os testes injetavam o diretório correto, mascarando a divergência. Resultado:
o gate passava em CI, mas o comportamento real do `ship` estava errado. A degradação era silenciosa
porque o teste nunca exercitava o caminho de código que o usuário final chama.

O D4 tem uma variante semântica: a regra `branch_has_wip_roadmap` "funcionava" mas com uma definição
errada de sucesso — um roadmap em `done/` é governança concluída, não ausente.

**Como aplicar em shell:**

`set -euo pipefail` no topo **não** é suficiente. Dois casos que ele não cobre:

```bash
# (1) Substituição de comando — falha engolida
count=$(wc -l < arquivo_que_nao_existe)   # exit 1 de wc não propaga

# (2) Lado esquerdo de pipe — exit ignorado
find . | sort | wc -l   # se find falhar, sort e wc continuam com saída vazia
```

Para (1): capturar e testar explicitamente, ou usar a forma `|| { echo "erro"; exit 1; }`.
Para (2): usar `pipefail` (já obrigatório) **e** verificar se a saída não está vazia com
`vacuity guard`:

```bash
files=$(find "$dir" -name "*.json" | sort)
if [[ -z "$files" ]]; then
  echo "FAIL: nenhum arquivo encontrado em $dir — verifica o caminho" >&2
  exit 1
fi
```

**Como aplicar no validator:** erros de leitura de diretório (`listDir`, `Glob`) devem
propagar o erro, não retornar lista vazia e seguir como se o diretório estivesse vazio.
A regra `folder_status` e `filename_uniqueness` foram corrigidas exatamente por isso.

---

## P3 — Independência de ambiente

**Regra:** o resultado do gate não pode variar com versão de runtime, cor de terminal, locale,
`PATH`, presença de ferramenta externa, ordenação de sistema de arquivos ou fim de linha.
Onde a dependência é inevitável, ela é **neutralizada explicitamente** — não delegada a uma
variável de ambiente que o runtime pode ignorar.

**Por quê:** o D2 é o caso canônico. O Python 3.13+ passou a colorir a saída do `argparse`
por padrão. `NO_COLOR=1` existia, mas o argparse o ignorava. O `grep` casava na descrição
textual dos comandos `agents` e `skills` — que por acaso não têm decoração ANSI — e os
reportava como presentes. Os outros 19 comandos reprovavam em máquinas com Python 3.13+,
mas o CI usava 3.10/3.12, onde a saída nunca tinha cor. O gate estava verde em CI e vermelho
localmente, na direção inversa do esperado.

**Padrões de neutralização obrigatórios:**

| Dependência | Neutralização |
|---|---|
| Cor ANSI (argparse, chalk, lipgloss) | Exportar `NO_COLOR=1 TERM=dumb` **e** filtrar com `sed 's/\x1b\[[0-9;]*m//g'` antes de comparar |
| Ordenação de sistema de arquivos (`ls`, `find`) | Usar `sort` explicitamente; nunca assumir ordem de diretório |
| Fim de linha (CRLF vs LF) | Normalizar com `tr -d '\r'` ou equivalente antes de comparar conteúdo de arquivo |
| Locale (collation, case folding) | Exportar `LC_ALL=C` em comparações e sort |
| Ferramenta externa ausente | Verificar com `command -v <tool>` e falhar com mensagem clara se ausente |

**Fim de linha no validator:** a regra `contentHasMarker` foi corrigida nos 3 CLIs para
reconhecer tanto LF (`marker + " \n"`) quanto CRLF (`marker + " \r\n"`). Arquivos editados
no Windows passavam sem violação mesmo sem o marcador — o CRLF quebrava o match silenciosamente.

---

## P4 — Falsificabilidade obrigatória

**Regra:** todo gate novo ou corrigido só é aceito com **demonstração de que ele reprova**
o caso que deveria reprovar. "Ficou verde" não é critério de aceite. "Reprovou quando deveria
e passou quando deveria" é.

**Por quê:** todos os quatro defeitos existiam com CI verde. A ausência de prova de reprovação
é o que permite que um gate vire decoração — um check que nunca falha, mesmo quando deveria.

**Como registrar a prova:**

Para gates de script (paridade, assets, etc.): adicionar um cenário em
**`scripts/check-gates-falsify.sh`**. Este arquivo é o lugar canônico para provas negativas.
Cada cenário:

1. Monta uma árvore temporária com o defeito introduzido deliberadamente.
2. Afirma que o gate retorna `exit != 0` **e** que a saída contém o diagnóstico esperado.
3. Desmonta a árvore via `trap 'rm -rf "$WORK"' EXIT` — sem resíduo.

```bash
assert_fails_with "nome-do-cenario" \
  "fragmento do diagnóstico esperado" \
  bash "$scripts/check-alguma-coisa.sh"
```

O script roda como parte do alvo `parity` no `Makefile`, após os gates positivos.
**Um gate novo que não tem entrada em `check-gates-falsify.sh` não passou pela P4.**

Para regras do validator: adicionar um caso de teste negativo nas suítes existentes
(`internal/validator/validator_test.go`, `npm/tests/validator.test.js`,
`pypi/tests/test_validator.py`). O cenário monta o estado de violação e afirma que a regra
reporta a violação esperada — não apenas que não reporta quando está ok.

**Seis provas registradas na implementação desta REQ:**

| Cenário | Gate | Diagnóstico afirmado |
|---|---|---|
| `static-assets/byte-drift` | `check-static-assets.sh` | `"Static asset byte drift"` |
| `integration-assets/byte-drift` | `check-integration-assets.sh` | `"Integration asset byte drift"` |
| `identity-parity/slug-drift` | `check-identity-parity.sh` | `"slug vectors drift"` |
| `validate-parity/rule-removed` | `check-validate-parity.sh` | `"validate JSON contract differs between runtimes"` |
| `cli-parity/missing-command` | `check-cli-parity.sh` | `"node: missing command 'note'"` |
| `integration-cli-parity/missing-agents` | `check-integration-cli-parity.sh` | `"node: root help missing agents"` |

---

## Adicionando um gate novo

Checklist antes de commitar:

- [ ] **P1:** a contagem/lista esperada vem de uma fonte de verdade, não de uma constante.
- [ ] **P2:** o gate falha com mensagem clara se não consegue obter o que precisa para verificar.
      Verificar substituições de comando `$(...)` e lados esquerdos de pipe separadamente.
- [ ] **P3:** o resultado é o mesmo em Python 3.10, 3.13+, Node 18, Node 22, macOS e Linux,
      com e sem TTY, com qualquer locale.
- [ ] **P4:** há um cenário em `scripts/check-gates-falsify.sh` (para scripts de paridade)
      ou um caso de teste negativo (para regras do validator) provando que o gate reprova
      o cenário que deveria reprovar.
- [ ] O gate está integrado ao alvo correto do `Makefile` (`quality` ou `parity`).
- [ ] O contrato público (comandos, flags, saída JSON) está documentado em `docs/cli-parity.md`.
