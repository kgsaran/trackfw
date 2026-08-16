---
status: done
date: 2026-08-16
roadmap: "docs/roadmaps/wip/ROADMAP-2026-08-15-remocao-do-subsistema-de-plugins-do-trackfw.md"
ml: "ML-3B"
agente: "hefesto-tf"
---

# Auditoria de qualidade — Remoção do subsistema de plugins (ML-3B)

> Branch: `refactor/remocao-do-subsistema-de-plugins-do-trackfw`
> Escopo: `git diff main...HEAD` (waves 1 e 2, commits `5dba110`..`023a269`)
> `make quality` verde no HEAD auditado — os achados abaixo são o que os gates automáticos não cobrem.

## Resumo

| Severidade | Qtd |
|---|---|
| Bloqueante | 0 |
| Alto | 0 |
| Médio | 2 |
| Baixo | 2 |
| Observação | 3 |

**Nenhum achado bloqueante para merge.** Os dois achados médios (M1, M2) são resíduo genuíno desta
remoção — o mesmo padrão em locais irmãos foi corrigido e um foi esquecido. `site/guide/commands.md`
foi investigado (ver O3) e **não** é resíduo desta entrega: é doc drift pré-existente e mais amplo,
tratado como observação com recomendação de REQ própria.

---

## Médio

### M1 — `internal/i18n/locales/{en-US,es-ES,pt-BR}.json`: chave `errors.pluginNotFound` órfã nos 3 locales Go

**Arquivos e linhas:**
- `internal/i18n/locales/en-US.json:63-65`
- `internal/i18n/locales/es-ES.json:64`
- `internal/i18n/locales/pt-BR.json:64`

**Problema:** o namespace `"errors"` em cada um dos 3 locales Go contém **só** a chave
`pluginNotFound` (`"plugin {{name}} not found"` / `"plugin {{name}} no encontrado"` / `"plugin
{{name}} não encontrado"`). `grep -rn "errors\." internal --include=*.go` (excluindo testes) não
retorna nenhuma ocorrência — nenhum código Go referencia mais esse namespace. É dado morto, único
consumidor era `internal/commands/plugins.go`, apagado na Wave 1.

**Assimetria com os outros dois CLIs (item 7 — consistência do estado final):** os locales
equivalentes de `npm/src/i18n/locales/en-US.json:124-126` e
`pypi/trackfw/i18n/locales/en-US.json:121-124` foram corretamente podados nesta mesma entrega — o
namespace `"errors"` neles ficou só com `notFound`/`downloadFailed` (chaves não relacionadas a
plugins, que já existiam antes e continuam em uso). Ou seja: o mesmo trabalho de limpeza de i18n foi
feito em Node e Python e **não** foi feito em Go — os 3 arquivos de locale Go são os únicos com
resíduo.

**Correção sugerida:** remover a chave `errors.pluginNotFound` dos 3 locales Go. Se o namespace
`"errors"` ficar vazio depois disso, remover o namespace inteiro (confirmar primeiro que nenhum
outro comando Go planeja usá-lo em curto prazo).

### M2 — `pypi/trackfw/thirdparty/fetch.py:32`: comentário desatualizado sobre "plugin binary download cap"

**Arquivo e linha:** `pypi/trackfw/thirdparty/fetch.py:32-33`

```python
# 2 MiB — deliberately smaller than the plugin binary download cap,
# because this is text, not a binary release asset.
MAX_CONTENT_SIZE = 2 << 20
```

**Problema:** o comentário ainda compara o limite com "the plugin binary download cap", um conceito
que não existe mais no código (o subsistema de plugins, incluindo seu próprio limite de tamanho de
download, foi removido na Wave 1). Os dois arquivos irmãos — `internal/thirdparty/fetch.go:15-20` e
o docstring do próprio módulo Python (linhas 6-9, poucas linhas acima do trecho não atualizado) —
**foram** corrigidos nesta mesma entrega para não mencionar mais plugins. Este comentário
especificamente ficou para trás: é o único dos três (`fetch.go`, docstring de `fetch.py`, comentário
de `MAX_CONTENT_SIZE` em `fetch.py`) que não foi tocado.

**Impacto:** nenhum impacto funcional — comentário morto, não afeta comportamento nem testes. É
exatamente o padrão "parou no meio do caminho" que este ML foi pedido para caçar: a mesma limpeza
foi aplicada em 2 de 3 lugares no mesmo arquivo/par de arquivos.

**Correção sugerida:** alinhar com `internal/thirdparty/fetch.go:15-18` — algo como `"2 MiB —
deliberately small, since this is text, not a binary release asset"`.

---

## Baixo

### B1 — `internal/commands/root.go:185-191`: comentário de `suggestCommand` com fraseado confuso (duplo sentido do "prefix")

**Arquivo e linha:** `internal/commands/root.go:187-190`

```go
// suggestCommand picks a single closest candidate for typed, or reports none.
// Shared cross-CLI criterion (reimplemented identically in npm and pypi):
//   - a candidate is eligible when its case-insensitive Levenshtein distance to
//     typed is <= 2, OR it is a case-insensitive prefix of typed's target (i.e.
//     the candidate starts with the typed text);
```

**Problema:** "it is a case-insensitive prefix of typed's target" é uma frase que se lê, numa
primeira passada, como "typed é prefixo do candidato" — o oposto do que o código faz
(`strings.HasPrefix(lowerC, lowerTyped)`, ou seja, o **candidato** começa com o texto digitado). A
frase entre parênteses corrige a ambiguidade ("the candidate starts with the typed text"), mas só
depois de já ter introduzido a confusão. Os comentários equivalentes em
`npm/src/lib/unknown-command.js:44-48` e `pypi/trackfw/unknown_command.py:38-41` já usam o fraseado
mais direto ("prefix match (candidate starts with the typed text)"), sem a construção ambígua
"prefix of typed's target".

**Impacto:** nenhum — é o único dos 3 comentários com essa redação; os 3 códigos concordam
(confirmado por leitura linha a linha e pelo gate `check-unknown-command-parity.sh`). Puramente
uma questão de manutenibilidade para quem precisar portar esta lógica para uma eventual 4ª
implementação no futuro.

**Correção sugerida:** alinhar o comentário Go ao fraseado do Node/Python: "OR it is a
case-insensitive prefix match (the candidate starts with the typed text)".

### B2 — `internal/commands/root_test.go:122-140`: `TestFormatUnknownCommandError_PluginsIsGone` usa `HasPrefix` em vez de igualdade byte-a-byte, ao contrário dos dois testes irmãos

**Arquivo e linha:** `internal/commands/root_test.go:137`

```go
if !strings.HasPrefix(msg, `Error: unknown command "plugins" for "trackfw"`) {
```

**Problema:** `TestFormatUnknownCommandError_CanonicalMessage_WithSuggestion` (linha 88-93) e
`TestFormatUnknownCommandError_CanonicalMessage_NoSuggestion` (linha 112-116), os dois testes
imediatamente acima, comparam a mensagem inteira por igualdade (`msg != want`). Este terceiro teste,
para o caso "plugins", relaxa para `HasPrefix`, então não afirma nada sobre a linha 2/3 da mensagem
(presença ou ausência de "Did you mean" e da linha final "Run ... for usage.") — só confirma que a
primeira linha está certa.

**Por que não é um "teste que não consegue falhar" (item 5):** `"plugins"` tem distância Levenshtein
alta pra qualquer comando registrado e não é prefixo de nenhum, então na prática não deveria
sugerir nada — o teste continua fazendo algo útil e não passaria com uma função vazia (ele falharia
se o prefixo canônico sumisse). Mas é inconsistente em rigor com os dois vizinhos, e cobre menos do
que poderia com o mesmo esforço.

**Correção sugerida:** trocar por igualdade completa como os dois testes irmãos, fixando também
`"Run 'trackfw --help' for usage."` como última linha e a ausência de "Did you mean" (já que
"plugins" não deveria ter sugestão elegível).

---

## Observações (não são achados de risco — reconhecendo o que está bem feito)

### O1 — Triplicação da mensagem canônica está disciplinada

Item 4 do escopo pedia avaliar se a reimplementação 3x de Levenshtein + elegibilidade + desempate
nasceu divergente. **Não nasceu.** `internal/commands/root.go` (`suggestCommand`/
`levenshteinDistance`), `npm/src/lib/unknown-command.js` (`suggestCommand`/`levenshteinDistance`) e
`pypi/trackfw/unknown_command.py` (`suggest_command`/`levenshtein_distance`) usam:
- os mesmos nomes de função traduzidos idiomaticamente para cada linguagem (camelCase Go/JS,
  snake_case Python);
- a mesma ordem de operações (early-continue por elegibilidade, depois comparação de menor
  distância com desempate alfabético);
- comentários que se referenciam cruzadamente ("reimplemented IDENTICALLY in Go/Node.js/Python...").

A paridade é verificada byte-a-byte por `scripts/check-unknown-command-parity.sh`, que por sua vez
ganhou 3 cenários de falsificação dedicados (`check-gates-falsify.sh`, cenários 55/56/57 do ML-2B)
provando que uma divergência de texto, exit code ou supressão de sugestão em qualquer um dos 3
runtimes faz o gate reprovar. Essa é exatamente a estrutura que evita a triplicação divergir com o
tempo — próxima manutenção só precisa seguir o padrão já pinado.

### O3 — `site/guide/commands.md` / `site/en/guide/commands.md` documentam `trackfw plugins`, mas isso é doc drift pré-existente, não resíduo desta remoção

**Arquivos e linhas:** `site/en/guide/commands.md:644-662`, `site/guide/commands.md:659-688`.

As duas páginas do site (publicado via `.github/workflows/deploy-docs.yml`, vitepress build+deploy —
não é artefato morto) ainda têm a seção `## trackfw plugins` inteira, incluindo `trackfw plugins
search <keyword>` — subcomando que **nunca existiu** em nenhum dos 3 CLIs conforme o próprio
inventário do roadmap (`docs/roadmaps/wip/ROADMAP-2026-08-15-...md:25-29`, que lista só
`list`/`add`/`remove`). Isso já era um sinal de que a página estava desatualizada antes desta
remoção, não um efeito dela.

Confirmado com `git log --oneline -1 -- site/guide/commands.md` e `site/en/guide/commands.md`: o
último commit a tocar os dois arquivos é o PR #136 (`docs(vault): ... corrige site desatualizado`),
anterior a esta branch. Os dois arquivos também não foram atualizados pelas features mais recentes
que adicionaram comandos novos (`trackfw changelog`, PR #173; `trackfw commit --suggest`, PR #171) —
`grep -n "trackfw changelog\|trackfw commit"` não retorna nada em nenhuma das duas páginas. Ou seja,
o site está sistematicamente atrás do CLI, não especificamente atrás da remoção de plugins.

**Por que isso NÃO é achado desta auditoria (item 1-3 do escopo, "o que a remoção deixou pela
metade"):** o mesmo tratamento já existe como precedente no próprio `docs/cli-parity.md` desta
entrega, para a divergência Go/Node do "sem argumento" — "pre-existing divergence — out of scope,
not touched here... needs its own REQ". `site/` merece o mesmo enquadramento: **recomendação de REQ
própria de manutenção de docs do site** (escopo maior que plugins — cobrir também `changelog` e
`commit`), não um ML de correção amarrado a este roadmap.

**Checagem adicional (AC4 do roadmap):** confirmado que `~/.trackfw/plugins` e `RegistryURL` — os
dois literais que o AC4 pede para zerar em código de produto — retornam **zero ocorrências** em
`internal/`, `npm/src/` e `pypi/trackfw/`. As únicas ocorrências restantes desses literais estão em
`site/` (mesmo achado acima) e em `docs/roadmaps/{abandoned,done}/`, `docs/adr/`, `docs/seguranca/`,
`docs/req/` e nas entradas antigas de `docs/agents-working-context.md` — todos registros históricos
legítimos (ADRs superseded, roadmap abandonado, pareceres de segurança anteriores à remoção,
entradas de sessão anteriores à Wave 1), não ponteiros ativos que um agente seguiria hoje como
"como o código funciona agora". AC4 está satisfeito no código de produto.

### O2 — Testes de comportamento novo não são tautológicos

Verificação pontual do item 5 (padrão já encontrado duas vezes nesta sessão, por instrução do
prompt): os testes novos em `internal/commands/root_test.go`,
`internal/commands/agents_skills_test.go` (`TestRemovedIntegrationAliasesAreUnknownCommands`),
`npm/tests/unknown-command.test.js` e `pypi/tests/test_commands_basic.py::TestUnknownCommand`
afirmam texto exato ou exit code exato, incluindo cenário de falsificação com binário real
`trackfw-vaildate` no PATH nos 3 CLIs. Nenhum passaria com a função de sugestão retornando vazio ou
com o handler de erro sendo um no-op — a asserção de "não executou o binário externo" é o oposto de
vacuidade (prova ausência de efeito colateral, não presença de mensagem).

---

## Os 3 achados mais importantes

1. **M1 — `errors.pluginNotFound`** órfã nos 3 locales Go (`internal/i18n/locales/*.json`),
   enquanto os locales Node e Python equivalentes foram corretamente podados na mesma entrega —
   assimetria clara entre os 3 CLIs, exatamente o tipo de resíduo que uma remoção "com paridade"
   deveria ter evitado. É o achado mais bem confirmado (dupla verificação: ausência de qualquer
   `errors.` em código Go não-teste + comparação direta com os locales Node/Python).
2. **M2 — comentário de `fetch.py:32`** referenciando "the plugin binary download cap": o docstring
   do topo do mesmo arquivo e o `internal/thirdparty/fetch.go` irmão foram corrigidos nesta mesma
   entrega, mas este comentário específico, poucas linhas abaixo, ficou para trás — o exemplo mais
   literal de "parou no meio do caminho" encontrado nesta auditoria.
3. **B2 — `TestFormatUnknownCommandError_PluginsIsGone`** usa `HasPrefix` em vez de igualdade
   byte-a-byte como os 2 testes irmãos imediatamente acima dele no mesmo arquivo — não é vácuo, mas
   é rigor inconsistente dentro do mesmo bloco de testes novo.

**Nota sobre `site/guide/commands.md`/`site/en/guide/commands.md`:** investigados e rebaixados a
observação (O3) — confirmado via `git log` que a última alteração é do PR #136, anterior a esta
branch, e que a página já está atrasada em relação a features mais recentes não relacionadas a
plugins (`changelog`, `commit`). É doc drift pré-existente e mais amplo, não resíduo desta remoção;
tratamento recomendado é REQ própria de manutenção do site, no mesmo padrão que
`docs/cli-parity.md` já usa para a divergência Go/Node do "sem argumento".
