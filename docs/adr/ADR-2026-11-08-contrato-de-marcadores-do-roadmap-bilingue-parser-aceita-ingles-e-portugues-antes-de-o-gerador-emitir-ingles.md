---
status: Accepted
date: 2026-11-08
author: "Zeus (trackfw_architect)"
---

# ADR: contrato de marcadores do roadmap bilíngue — o parser aceita inglês e português antes de o gerador emitir inglês

> Date: 2026-11-08 | Status: Accepted

## Context

Em 2026-11-02 o KG decidiu que o trackfw é **inglês primeiro** (REQ-2026-11-02, idioma do produto). Parte do
texto em português não é mensagem: é **contrato** que o binário lê de volta no roadmap e na REQ. Traduzir o
gerador sem tratar o leitor quebra o acervo, e traduzir na ordem errada abre uma falha silenciosa.

Leitura dos parsers em 2026-11-08 (`internal/roadmapdoc/roadmapdoc.go`, `internal/commands/barrier.go`,
`internal/validator/`):

| Marcador | Onde é lido | Hoje aceita |
|---|---|---|
| `**Acceptance criteria:**` / `**Critérios de aceite:**` | `roadmapdoc.CriteriaHeaderRe` | EN e PT |
| `**Files affected:**` / `**Arquivos afetados:**` | `barrier.go` (escopo do ML) | EN e PT |
| `## Acceptance Criteria` / `## Critérios de aceite` | `validator_req_done_criteria.go` | EN e PT |
| Status completo (`✅`, `done`, `concluido`) | `roadmapdoc.statusVocabulary` | EN e PT |
| `**Gates da wave:**` | `roadmapdoc.GatesHeaderRe` | **só PT** |
| `  Caducou:` (continuação de critério caducado) | `roadmapdoc.LapsedContinuationRe` | **só PT** |
| Término `abandonado`, `🚫`, `❌ Cancelado` | `roadmapdoc.StatusCategory` | **só PT** |

Os pendentes (`⬜`, `🔄`, `❌ Bloqueado`) são decididos pelo **primeiro token emoji**, então já são
independentes de idioma, exceto no desempate do `❌`, que lê o segundo token.

**A falha que define a ordem.** `ParseGatesLines` trata uma wave sem cabeçalho de gates reconhecido como
**zero gates, o que é legal** ("the barrier never invents one"). Se o gerador passar a emitir
`**Wave gates:**` e um binário anterior ler esse roadmap, ele não reconhece o cabeçalho, conclui "zero
gates" e o `barrier` passa **sem executar nada**. É um fail-open entre versões. O trackfw ainda não tem
usuários downstream, mas os hooks deste próprio repositório rodam o binário instalado, que costuma estar
atrasado: o caso acontece aqui.

## Decision

**D1 — Todo marcador de contrato é bilíngue no leitor, com conjunto fechado de grafias.** Os aliases
novos são:

- `**Wave gates:**` como alias de `**Gates da wave:**`;
- `  Lapsed:` como alias de `  Caducou:`, com a mesma regra (≥2 espaços, não tab, texto não vazio depois
  dos dois-pontos);
- término: `abandoned` entra em `terminatedVocabulary`; `cancelled` e `canceled` entram no desempate do `❌`.
  `blocked` **continua pendente**, e um teste afirma isso, como já afirma para `bloqueado`.

Nenhuma outra grafia entra (o conjunto continua fechado, como na ADR de 2026-08-29: "feito", "ok" e
"finalizado" seguem fora). O inventário da Wave 0 pode acrescentar marcadores a esta lista; acrescentar
exige emenda desta ADR, não decisão no ML.

**D2 — O leitor bilíngue sai numa release antes de o gerador emitir inglês.** As waves do roadmap são
separadas: primeiro o parser aceita as duas línguas, e essa mudança é publicada numa tag; o gerador só
passa a emitir inglês numa tag posterior. Assim, quem atualizar o binário recebe primeiro a capacidade de
ler e só depois o conteúdo novo.

**D3 — Dois cabeçalhos de gates na mesma wave é erro, não fusão.** Uma wave com `**Gates da wave:**` e
`**Wave gates:**` fora de cerca e de comentário faz o `barrier` falhar com mensagem que nomeia as duas
linhas. Fundir esconderia qual bloco o autor pretendia, e escolher um deles descartaria gates em silêncio.
A regra vale também para dois cabeçalhos na mesma língua, se hoje não valer (a Wave 0 mede).

**D4 — Forma canônica emitida pelo gerador, depois de D2:**

- legenda: `⬜ Pending · 🔄 In progress · ✅ Done · ❌ Blocked`;
- status do ML: `**Status:** ⬜ Pending`;
- cabeçalhos: `**Files affected:**`, `**Actions:**`, `**Acceptance criteria:**`, `**Wave gates:**`.

**D5 — O acervo não é migrado.** Roadmaps e REQs existentes, deste repositório e de consumidores,
continuam válidos sem edição. A documentação em `docs/` continua em português (escopo negativo da REQ).
Não existe `--lang` nem i18n.

**D6 — Presets de identidade (AC5 da REQ).** Os **nomes** dos presets (`chaves`, `turma` e os nomes
próprios dos papéis) ficam como estão: são nomes próprios e identificadores gravados na configuração do
consumidor, e renomeá-los quebraria `trackfw agents update` para quem já os escolheu. O **texto
descritivo** do preset vai para inglês.

**D7 — O acervo é o oráculo do AC4.** O gate de não regressão compara a saída de `trackfw barrier` e de
`trackfw validate` sobre todo `docs/roadmaps/`, com o binário da `main` e com o da branch. A diferença
tem de ser vazia: nenhuma violação nova e nenhuma perdida.

**Emenda 2026-11-08 (Wave 0).** O inventário fechou o grupo 3: os únicos marcadores só em português são
os quatro de D1. Há três leitores já bilíngues fora das zonas da tabela acima — `config.AcceptanceMarkers`
(`internal/config/config.go`), `extractMotivation` (`internal/sync/sync.go`, `## Motivation`/`## Motivação`)
e `NewRoadmapFromREQ` (`internal/generators/roadmap.go`) — e nenhum exige alias. D2 vale também para os
meta-geradores (`claudemd.go`, `agentfiles.go`), que instruem o agente consumidor a escrever os marcadores:
eles mudam na mesma tag do gerador. `docs/cli-parity.md` documenta o contrato e muda junto com o leitor.

**Emenda 2026-11-08 (Wave 1), consequência de D3 no acervo.** 14 waves de roadmaps em `done/` têm dois
cabeçalhos de gates; até aqui o segundo bloco era ignorado em silêncio. Com D3, o `barrier` dessas waves
passa a reprovar. Elas não são reeditadas (D5): roadmap em `done/` não roda mais barrier, e o `validate`
só inspeciona gates de roadmaps em `wip/`. O oráculo de D7 confirma saída idêntica de `validate` e de
`roadmap show --json` nos 243 roadmaps.

**Emenda 2026-11-08 (Wave 2, red team) — D8, gate nunca some em silêncio.** O red team mostrou que, já
em português, o cabeçalho ou o bloco de gates podia ficar invisível (comentário HTML não fechado, cerca não
fechada) ou ser trocado por um bloco de exemplo, e a wave passava. Os aliases ampliam essa superfície. Por
isso: a busca do bloco respeita as mesmas máscaras do cabeçalho, e qualquer anomalia de máscara que possa
esconder o cabeçalho é erro, não zero gates. A razão de `Lapsed:`/`Caducou:` exige caractere visível.

**Emenda 2026-11-08 (Wave 2, auditoria do ML-2C) — D8 cobre também a máscara balanceada.** Cercas
balanceadas pela regra CommonMark podem cobrir o cabeçalho real (E3 do red team) sem nenhuma anomalia de
máscara. Por isso: uma wave sem cabeçalho de gates válido que tenha cabeçalho só dentro de cerca ou de
comentário HTML falha com erro que nomeia a linha. Com um cabeçalho válido presente, exemplos mascarados
continuam permitidos.

**Emenda 2026-11-08 (Wave 2, red team rodada 2) — D8 cobre o quase-cabeçalho.** A rodada 2 mediu que
`**Wave gates:**` com 1 a 3 espaços antes, como item de lista, em blockquote ou com homóglifo (`gаtes` em
cirílico) não casa `GatesHeaderRe`, e a wave conta zero gates. Os casos com espaço e com homóglifo são
visualmente idênticos ao cabeçalho válido, então a revisão humana não protege. A rodada 1 tinha aceitado
isso como residual "por design"; esta emenda revoga essa aceitação. Uma linha fora de máscara que não é
cabeçalho válido, mas cujo esqueleto (sem espaço inicial, sem marcador de lista ou blockquote, sem
caracteres invisíveis, homóglifos levados ao ASCII, minúsculas) começa com um dos dois cabeçalhos, é erro
que nomeia a linha. O conjunto de grafias aceitas continua fechado: o quase-cabeçalho é recusado, não
aceito. Na mesma linha, a razão de `Lapsed:`/`Caducou:` não pode ter só caracteres
`Other_Default_Ignorable_Code_Point` (preenchedores Hangul, de categoria Lo).

**Emenda 2026-11-08 (Wave 2, red team rodada 3) — modelo de ameaça e critério de convergência do D8.**
A rodada 3 achou 14 grafias novas que escapam. Depois das rodadas 1 e 2, ficou claro que reconhecer
grafias não converge, e que a meta das emendas anteriores ("homóglifo", "sem caracteres invisíveis") era
maior do que o mecanismo consegue entregar. A frase "sem caracteres invisíveis" estava errada como escrita:
os seletores de variação VS1–VS14 (categoria Mn) e o Braille em branco (U+2800, So) passavam.

1. **Modelo de ameaça.** Para a wave ≥1, zero gates é legal. Quem pode editar o roadmap pode apagar o
   cabeçalho e o bloco, e nenhuma regra de grafia impede isso. A detecção de quase-cabeçalho protege
   contra **acidente** (pessoa ou modelo de linguagem que escreve o cabeçalho de um jeito quase certo), não
   contra adversário. A defesa contra quem apaga gates é a revisão do diff do roadmap, não o parser.
2. **Âncora na cerca.** Um quase-cabeçalho só é erro se estiver **ligado a uma cerca** ```` ```bash ````
   pela mesma ligação que o cabeçalho válido usa: a próxima cerca ```` ```bash ```` fora de máscara
   antes do próximo `## `/`### `. Sem cerca ligada, nada se perde, porque o barrier roda zero gates nos
   dois casos. Isso também tira o falso positivo de prosa como "Wave gates: ver abaixo" sem cerca depois.
3. **Classes fechadas por categoria, não por grafia.** O esqueleto remove todo `Mn`, `Cf`,
   `Other_Default_Ignorable_Code_Point` e U+2800. Decodifica entidades HTML e remove tags `<…>`. Passa para
   minúsculas **antes** do mapa de homóglifos. Trata `:`, `：`, `∶` e `꞉` como dois-pontos. A forma sem
   dois-pontos casa só quando a linha inteira, reduzida, é o alvo (`**Wave gates**`, `### Wave gates`).
4. **Residual declarado.** Homóglifo fora da tabela fechada (scripts que não sejam latino, cirílico ou
   grego, e confusáveis raros desses três) e qualquer ofuscação deliberada. Pelo item 1, isso equivale a
   apagar o cabeçalho, que já é legal.
5. **Critério de aprovação do red team.** APROVA quando nenhum vetor **acidental** (digitável por pessoa
   ou gerável por modelo de linguagem sem intenção de evasão) conta zero gates em silêncio. Vetores da
   classe do item 4 não reprovam.

## Consequences

- O acervo continua passando; roadmaps novos nascem em inglês depois da segunda tag.
- São duas releases em vez de uma. É o custo de fechar o fail-open de D2.
- **Residual declarado:** um binário anterior à primeira tag, lendo roadmap gerado depois da segunda, ainda
  conta zero gates nas waves com `**Wave gates:**`. D2 reduz a janela, mas não a elimina para quem pular a
  primeira tag. Fechar isso exigiria que o binário antigo já soubesse o alias, o que é impossível. O aviso
  vai no CHANGELOG da segunda tag.
- Testes que afirmam `⬜ Pendente` ou `**Gates da wave:**` na saída do gerador mudam junto com o gerador,
  e os cenários de `scripts/check-gates-falsify.sh` que fixam a forma portuguesa são auditados na mesma
  wave.

## Alternatives Considered

- **Traduzir só o gerador e deixar o parser em português:** o roadmap gerado em inglês reprova ou, pior,
  passa sem gates. Rejeitada.
- **Migrar o acervo para inglês com um comando:** edita documentação que o escopo negativo mantém em
  português e não alcança roadmaps de consumidores sem pedir. Rejeitada.
- **Manter o gerador em português e traduzir só mensagens:** contraria a decisão de 2026-11-02, porque o
  roadmap gerado é a primeira coisa que o usuário novo lê. Rejeitada.
- **Leitor e gerador na mesma tag:** abre o fail-open de D2 para qualquer binário instalado anterior.
  Rejeitada.
