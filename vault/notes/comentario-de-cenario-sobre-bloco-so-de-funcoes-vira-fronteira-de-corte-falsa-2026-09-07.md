---
title: "Comentário '# Cenário N --' sobre um bloco só de funções vira fronteira de corte válida-mas-falsa no particionador de falsify"
date: 2026-09-07
squad: ares-tf
roadmap: docs/roadmaps/wip/ROADMAP-2026-09-06-perfil-e-aceleracao-do-check-gates-falsify-sem-perder-cobertura.md
ml: ML-2D (reentrega pós-reprovação)
---

# Comentário de cenário sobre bloco só-de-funções vira fronteira de corte falsa

## O que aconteceu

`scripts/gen-falsify-chunks.py` (ML-2D) particiona `check-gates-falsify.sh` em N chunks, cortando em
todo comentário `# Cenário N — ...`. A entrega anterior (reprovada em auditoria) media **77 rótulos
perdidos** (19% da suíte) e **1 FAIL** determinístico (`serve-chain-canonical-link/node/edge-baseline`,
`exit=2`), e atribuiu isso a grafia divergente de cabeçalho ("42 cabeçalhos usam 'Cenario' sem acento,
ou `--`"). **Essa causa não se confirmou.**

## Causa real, medida

`HDR_PAT` já casava acento opcional, plural e os dois travessões. A linha 913
(`# Cenario 166 -- Direcao A ...`) já casava o padrão — e virava a **primeira** fronteira
(`prelude_end_line=912`, não ~1160 como o ML-2C tinha registrado). O trecho que essa fronteira
introduz (913-1160) **não contém nenhuma asserção própria**: só define 5 funções
(`setup_s166_tree`, `write_req_adr_placeholder_fixture`, `write_req_roadmap_prose_fixture`,
`run_node_chain_probe`, `run_python_chain_probe`) usadas por cenários **quase 8000 linhas depois**
(9015-10927 do arquivo real).

Sem tratamento especial, esse bloco vira um segmento normal do particionador, cai num chunk
DIFERENTE do dos seus chamadores, e a função inexiste ali sob `set -euo pipefail` — tudo que roda
depois na mesma chunk morre em silêncio. Confirmado por grep direto nos chunks gerados: `chunk_0.sh`
definia `setup_s166_tree`, `chunk_6.sh` chamava — `command not found`.

Isso explica as duas pontas do relatório reprovado **ao mesmo tempo**, não são dois achados
separados: os 77 rótulos perdidos são a cauda do chunk que abortou; o único FAIL sobrevivente
(`serve-chain-canonical-link/node/edge-baseline`) chama `run_node_chain_probe`, uma das 5 funções do
mesmo bloco. Reproduzido isolado:

```bash
bash -c "$(declare -f run_node_chain_probe); run_node_chain_probe '$ROOT/npm/src' '$FIXDIR'"
```

Com a função indefinida, esse comando sai com **exit 2** — não 127 como se esperaria de "command not
found" ingênuo. `bash -c "STRING"` com `STRING` sintaticamente vazia na parte substituída
(`$(declare -f fn)` produz nada quando `fn` não existe) mais a chamada real da função ausente produz
exit 2, não 127 — o exit code exato do relatório original, medido, não hipótese.

## Por que a hipótese de grafia parecia plausível e não era

O gap entre "137 cabeçalhos" (contagem ingênua `grep -c '^# Cen'`) e "95 que casam o padrão estrito"
era, na maior parte, **prosa** — comentários que *mencionam* um cenário sem *ser* seu cabeçalho
(ex.: `# Cenários 14/16/17/20/21...`, `# Cenário 24 nunca precisou de...`). Afrouxar ainda mais o
padrão de fronteira teria piorado: casaria mais prosa como fronteira falsa e cortaria o preâmbulo
ainda mais cedo.

## A correção

Um segmento é SUPORTE (içado para um preâmbulo estendido, duplicado em todo chunk — definição de
função não tem ordem de execução, duplicar é sempre seguro) se e somente se (a) não contém nenhuma
asserção própria (nem `assert_*`, nem `echo "OK/FAIL [falsify/...]"` manual — buscado por
**substring**, não âncora em coluna 0, porque muitas chamadas reais vêm depois de `cd ... &&`) E
(b) todo statement de topo é definição de função (heredoc-aware, fecha por `}` solitário — mesmo
critério que o ML-2C já validou como seguro para este arquivo).

Um terceiro candidato encontrado na mesma varredura (`Cenários 60/61`, que faz `cd` num diretório de
fixture até o Cenário 64 restaurar) **não** foi içado — tem código de topo além de função, então falha
o critério (b) e continua tratado como segmento de asserção normal. Já era coberto pela fusão por
variável existente (`GBG_ORIGINAL_PWD`, referenciado no Cenário 64), sem precisar de código novo.

## Generalização — o que outro agente deveria saber

Um comentário estilo `# Cenário N — ...` **não implica** que o que vem depois dele contém uma
asserção. Ferramentas que particionam este arquivo por essa fronteira (ou fronteiras parecidas em
outros scripts de falsificação do projeto) precisam classificar o CONTEÚDO do segmento, não confiar
no rótulo do comentário — um bloco de suporte (helpers usados por cenários distantes) pode estar
"disfarçado" de cenário.

## Limitação declarada, não fechada

A guarda de conjunto do driver (`run-gates-falsify-parallel.sh`) — que compara rótulos esperados
(extraídos do próprio texto de cada chunk) contra os emitidos — cobre só chamadas `assert_*` (249
literais + 3 templates glob no arquivo real, ~255 de ~386 rótulos únicos do run serial). Cenários com
`echo "OK/FAIL [falsify/...]"` manual (ex. `no-repo-mutation`, `validate-ok-message/*`,
`status-inventory/*`) ficam fora do conjunto esperado. Muitos desses écos são **assimétricos por
design** (ex. rótulos `setup-sN` só imprimem em erro, nunca em sucesso) — incluí-los ingenuamente na
guarda geraria falso-positivo garantido em toda run limpa. Fechar isso com segurança exige
distinguir eco "sempre dispara" de "só no caminho de erro", não feito nesta entrega.
