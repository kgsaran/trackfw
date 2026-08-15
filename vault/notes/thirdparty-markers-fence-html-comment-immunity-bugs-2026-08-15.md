# D3 marker-check: unclosed fence and "remove" (not neutralize) HTML comment silently defeated the security scanner

**Data:** 2026-08-15
**Contexto:** ROADMAP-2026-08-15-instalacao-de-skills-de-terceiro-via-url-para-agentes-especialistas,
ML-4C (microlote corretivo pós-barreira da Wave 4).
**Achado por:** `hades-tf` + `hefesto-tf` (independentemente, achado do fence); só `hades-tf`
(achado do comentário HTML). Reproduzido e corrigido pelo `apolo-tf`.

## Sintoma

O checker de markers de `internal/thirdparty/markers.go` (e portes Node/Python) existe para recusar
artefatos de terceiro que tentem redefinir seções de fronteira (`## Git authority`, `## Mode lock`
etc.) via heading. Dois bypasses **triviais e mais baratos que qualquer evasão já documentada como
"NÃO cobre"** passavam despercebidos:

1. Um arquivo iniciado por ` ``` ` **sem fechamento** até EOF fazia o line-scanner de
   `removeFencedBlocks` tratar o documento inteiro, a partir do abridor, como "dentro do fence" —
   `CheckMarkers` retornava `[]` mesmo com `## Git authority` na linha seguinte. Custa 3 caracteres.
2. `<!-- ## Git authority -->` passava limpo porque o passo 1 (`htmlCommentPattern.ReplaceAllString`)
   **apagava** o comentário inteiro antes de escanear — o oposto da própria justificativa escrita do
   passo ("um agente LLM lê comentário HTML no fluxo de tokens", ou seja, o conteúdo *precisa*
   continuar sendo escaneado, só os delimitadores é que não deveriam contar como estrutura).

## Causa raiz

- **(1):** `removeFencedBlocks` mantinha um único booleano de estado ("dentro de fence: sim/não") e
  descartava linhas incondicionalmente enquanto esse estado fosse verdadeiro — nunca havia um branch
  para "cheguei ao EOF e o fence nunca fechou, então isso nunca foi um fence de verdade".
- **(2):** o regex de remoção de comentário (`<!--.*?-->`) substituía o match inteiro (delimitadores
  + conteúdo) por string vazia, em vez de substituir só pelo grupo capturado (o conteúdo).

## Por que isso não foi pego antes

Havia testes cobrindo exatamente esses dois casos — mas **testando e fixando o comportamento
errado**: `TestCheckMarkers_UnclosedFenceDropsRestOfDocument` e
`TestCheckMarkers_HTMLCommentStrippedBeforeMatch` afirmavam `len(matched) == 0` como o resultado
CORRETO. `make quality`/CI ficavam verdes porque a asserção codificava o bug como contrato. Isso só
foi pego porque `hades-tf`/`hefesto-tf` (Wave 4, revisão de segurança pós-implementação) tentaram
ativamente falsificar o critério de recusa com payloads de evasão — nenhum teste unitário pré-existente
teria pego isso sozinho, porque a intenção original (ML-1A) tratou "fence não fechado = documento
inteiro imune" e "comentário apagado" como comportamento intencional, não como bug.

## Correção

- Fence sem fechamento deixa de conceder imunidade: as linhas são **bufferizadas** enquanto o
  scanner está "dentro" de um possível fence; se o fence fechar propriamente, o buffer é descartado
  (comportamento antigo, preservado); se chegar ao EOF sem fechar, o buffer inteiro (abridor
  incluído) é **reincorporado** ao texto a escanear.
- Comentário HTML passa a ser **neutralizado**: `<!--(.*?)-->` → `$1` (grupo capturado), removendo só
  os delimitadores.
- **Não-regressão crítica:** a emenda original de D3 (fence fechado concede imunidade) precisa
  continuar valendo, senão o próprio parecer de segurança (que lista os 6 marcadores dentro de um
  fence fechado, como documentação) passaria a se recusar. Teste dedicado lê o arquivo real do
  parecer em disco e exige zero marcadores — não um fixture sintético que poderia divergir do
  arquivo de verdade com o tempo.

## Lição para o próximo agente

Ao portar/revisar um "tripwire" de segurança (regex/scanner que decide recusar conteúdo), **não
confie em testes existentes como prova de intenção correta** — leia a doutrina declarada (aqui, a
doc comment do próprio passo do pipeline) e confirme que o teste testa o que a doutrina diz, não
apenas o que o código já faz. Um teste verde pode estar fixando um bug.

Ver também: `docs/cli-parity.md` §"D3-ter — fence não fechado e comentário HTML deixaram de conceder
imunidade (ML-4C)".
