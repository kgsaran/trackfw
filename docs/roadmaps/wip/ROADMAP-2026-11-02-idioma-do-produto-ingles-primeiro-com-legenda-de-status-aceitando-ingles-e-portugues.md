---
status: wip
date: 2026-11-02
req: "docs/req/REQ-2026-11-02-idioma-do-produto-ingles-primeiro-com-legenda-de-status-aceitando-ingles-e-portugues.md"
squad: "hades-tf, apolo-tf, ares-tf"
---

# Roadmap: Idioma do produto: inglês primeiro, com legenda de status aceitando inglês e português

> Created: 2026-11-02 | Status: wip

## Context
<!-- Derived from REQ: REQ-2026-11-02-idioma-do-produto-ingles-primeiro-com-legenda-de-status-aceitando-ingles-e-portugues.md -->
REQ: docs/req/REQ-2026-11-02-idioma-do-produto-ingles-primeiro-com-legenda-de-status-aceitando-ingles-e-portugues.md
ADR: docs/adr/ADR-2026-11-08-contrato-de-marcadores-do-roadmap-bilingue-parser-aceita-ingles-e-portugues-antes-de-o-gerador-emitir-ingles.md

O produto passa a ser inglês primeiro. O texto em português cai em três grupos: (1) mensagens do CLI,
(2) conteúdo gerado no consumidor, (3) contrato que o binário lê de volta. O grupo 3 é maior do que a REQ
previa: `**Gates da wave:**`, `Caducou:` e as palavras de término só existem em português, e uma wave sem
cabeçalho de gates reconhecido conta zero gates e passa. Por isso a ordem é fixada pela ADR (D2): o leitor
bilíngue sai numa tag antes de o gerador emitir inglês.

Issues abertas com o mesmo mecanismo: nenhuma (varredura em 2026-11-08, zero issues abertas).

## Acceptance Criteria
- [ ] AC1 — inventário por grupo, cobrindo todo arquivo versionado (não só `.go`)
- [ ] AC2 — mensagens do CLI em inglês, com teste que reprova o retorno do português
- [ ] AC3 — conteúdo gerado em inglês, entregue pelo `trackfw update` sem perder edição local
- [ ] AC4 — marcadores de contrato bilíngues; saída de `barrier` e `validate` sobre `docs/roadmaps/` idêntica entre `main` e branch
- [ ] AC5 — decisão sobre presets escrita (ADR D6) e aplicada

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model e inventário por grupo (AC1)
**Status:** ✅ Concluído

*Auditoria (Zeus, 2026-11-08):* §3 (fail-open medido com o binário 11.5.1), §4 e os achados de falsificação aprovados; AC1 reprovado — inventário com 12 de 45 arquivos — e corrigido no ML-0B.
**Squad:** hades-tf
**Files affected:**
- `docs/seguranca/2026-11-08-wave0-idioma-ingles-primeiro.md` (novo; único arquivo que o ML escreve)

**Actions:**
1. **Inventário completo (AC1).** Varrer **todo arquivo versionado** em `internal/`, `cmd/`, `scripts/`,
   `Makefile`, `npm/`, `pypi/` e `docs/cli-parity.md`, de qualquer extensão (`.go`, `.js`, `.html`,
   `.md`, `.tmpl`, `.sh`, `.yaml`), com `git ls-files` + `/usr/bin/grep`. Não truncar saída com `head`
   ou `tail`. Classificar cada sítio no grupo 1, 2 ou 3 e dizer se é emitido, lido ou as duas coisas.
   Incluir o JavaScript estático do `serve` (go:embed), skills e comandos `/trackfw:*` e arquivos de
   agente que instruem o agente consumidor a **escrever** marcadores em português.
2. **Grupo 3 fechado.** Confirmar ou ampliar a tabela de marcadores da ADR-2026-11-08 (Context). Para cada
   marcador lido pelo binário, citar arquivo, linha e regex.
3. **Fail-open entre versões (ADR D2).** Medir com o binário da `main` (compilado em diretório temporário)
   o que o `barrier` faz com uma wave cujo cabeçalho é `**Wave gates:**`: quantos gates executa e qual o
   resultado. Medir também o que acontece hoje com **dois** cabeçalhos `**Gates da wave:**` na mesma wave
   (ADR D3).
4. **Quem afirma a forma antiga.** Listar os testes Go e os cenários de `scripts/check-gates-falsify.sh`
   (e outros scripts) que fixam `⬜ Pendente`, `**Gates da wave:**` ou outra string em português da
   saída do gerador. Esses mudam junto com o gerador na Wave 3.
5. **Threat model.** Quem esvazia esta Wave 0 sem quebrar regra escrita, e como? Alvos de falsificação nas
   duas direções para cada marcador (alias não reconhecido → gate perdido; alias largo demais → texto
   comum vira marcador, ex. `blocked` virando término).
6. **Residual declarado.**

**Restrições:** não editar nenhum outro arquivo; não rodar `trackfw init` na árvore; medições git só em
`t.TempDir`/diretório temporário fora do repo; não executar `git` de escrita no repositório real; ao fim,
`git status --short` e `git diff --cached --name-only` no relatório.

**Acceptance criteria:**
- [x] As seis seções respondidas com evidência (comando e resultado), não com afirmação de uma linha
- [x] Tabela de inventário com arquivo, linha, grupo e direção (emite/lê) para cada sítio
- [x] Item 3 medido com o binário da `main`, com o comando e a saída
- [x] Nenhuma linha de implementação escrita

### ML-0B — Corretivo: inventário completo e releitura do grupo 3
**Status:** ✅ Concluído

*Auditoria (Zeus, 2026-11-08):* tabela com 48 arquivos; conferidos no código os três leitores novos, todos já bilíngues — `config.go:398` (`AcceptanceMarkers`), `sync.go:202` (`## Motivation`/`## Motivação`), `generators/roadmap.go:661` (`NewRoadmapFromREQ`). Nenhum exige alias no ML-1A.
**Squad:** hades-tf
**Files affected:**
- `docs/seguranca/2026-11-08-wave0-idioma-ingles-primeiro.md`

*Por que existe:* a auditoria do ML-0A reprovou o AC1. A §1 do parecer buscou só palavras de marcador e
cobriu 12 arquivos; a medição do arquiteto de literais com acento em arquivos de produto achou 45 arquivos
e 269 linhas (help.go 45, scaffold.go 44, validator/validator.go 31, …). Com a lista incompleta, a
afirmação "nenhum leitor de grupo 3 fora de roadmapdoc/barrier/validator" não estava sustentada.

**Actions:**
1. Tabela de §1 cobrindo todos os arquivos de produto com português, com grupo, direção e ML de destino.
2. Releitura de §2: abrir validator.go, config.go, discover.go, sync.go, thirdparty/markers.go, update.go,
   req_chain_ml4a.go, integrations/render.go, guard/writescope.go, validator_req_roadmap_cutoff.go e
   metrics.go e dizer, com linha, se algum literal português é lido de volta de arquivo do consumidor.

**Acceptance criteria:**
- [x] Todos os arquivos da medição do arquiteto presentes na tabela
- [x] Cada arquivo da ação 2 com veredito leitor/não leitor e linha citada

**Gates da wave:**
```bash
test -s docs/seguranca/2026-11-08-wave0-idioma-ingles-primeiro.md
```

## Wave 1 — Leitor bilíngue (AC4, ADR D1/D3/D7)
> Dependencies: Wave 0 auditada. ML-1A e ML-1B tocam arquivos disjuntos e correm em paralelo.

### ML-1A — Aliases em inglês nos parsers
**Status:** ✅ Concluído

*Auditoria (Zeus, 2026-11-08):* os 16 testes novos existem pelo nome e passam; `go test` de roadmapdoc, commands e validator rc=0. Medição do item 7 conferida por contagem independente (awk fora de cerca/comentário): 14 waves com dois cabeçalhos, todas em `done/`. Desvio: o apolo-tf escreveu nota no vault, fora da lista fechada; conteúdo válido, mantido. Residual apontado por ele vira o ML-1C.
**Squad:** apolo-tf
**Files affected:**
- `internal/roadmapdoc/roadmapdoc.go`, `internal/roadmapdoc/*_test.go`
- `internal/commands/barrier.go` e testes, se o inventário apontar marcador lido ali
- `docs/cli-parity.md` (seção "Wave gates contract" fixa `**Gates da wave:**` como única forma; Wave 0, achado 3)
- nenhum outro leitor: a Wave 0 fechou o grupo 3 em G3-1..G3-4 como os únicos só em português

**Actions:**
1. `GatesHeaderRe` aceita `**Gates da wave:**` e `**Wave gates:**`.
2. `LapsedContinuationRe` aceita `Caducou:` e `Lapsed:` com a mesma regra (≥2 espaços, não tab, não vazio).
3. `terminatedVocabulary` ganha `abandoned`; desempate do `❌` aceita `cancelled` e `canceled`.
4. Dois cabeçalhos de gates na mesma wave (fora de cerca/comentário), em qualquer combinação de língua,
   viram erro que nomeia as duas linhas (ADR D3).
5. Testes: braço positivo por alias novo; `❌ Blocked` e `❌ Bloqueado` continuam pendentes; os dois
   cabeçalhos dão erro; grafia fora do conjunto (`**Wave Gate:**`, `Lapsed :`) não é reconhecida.
   Mensagens novas em inglês.
6. Falsificação da Wave 0: `❌ cancellation-pending` continua pendente (comparação exata, nunca prefixo);
   `  Lapsed:` sem texto não conta; `**Wave gate:**` (singular) e `**Wave  gates:**` (dois espaços) não
   são reconhecidos.
7. Medir no acervo (`docs/roadmaps/**/*.md`) quantas waves têm dois ou mais cabeçalhos de gates fora de
   cerca e comentário: com D3, essas passam a dar erro no `barrier`, e o oráculo do ML-1B não as vê.

**Acceptance criteria:**
- [x] `go build ./...` sem erro
- [x] `go test ./internal/roadmapdoc/ ./internal/commands/ ./internal/validator/` verde
- [x] Cada teste novo com a frase do que afirma (Regra Dura de Reconciliação)

### ML-1B — Oráculo do acervo (ADR D7)
**Status:** ✅ Concluído

*Auditoria (Zeus, 2026-11-08):* `bash scripts/check-marker-corpus-parity.sh` com `origin/main` (9fd2d84b) × árvore com ML-1A: rc=0, 243 roadmaps, validate idêntico; nenhum worktree sobrando. Limitação: dois roadmaps (`architect-command-guidelines`, `attention-hooks-agent-clis`) têm nome ambíguo no `roadmap show` e só o `validate` os cobre. *Corretivo (2026-11-08):* o `make quality` do arquiteto reprovou em `check-unguarded-capture-rc` (captura `$(… | grep …)` sem guarda); o ares-tf guardou a captura, trocou dois `grep -q` em pipe e registrou a exceção do `check-orphan-gates.sh`, fora da lista fechada mas exigida pelo gate. `make parity-rest` rc=0.
**Squad:** ares-tf
**Files affected:**
- `scripts/check-marker-corpus-parity.sh` (novo; gate desta REQ, **não** ligado em `parity-rest`: depende de
  `origin/main` e compila dois binários)
- `scripts/check-orphan-gates.sh` (exceção com motivo para o script não ligado em Makefile; corretivo)

**Actions:**
1. O script compila o binário de `origin/main` (via `git worktree` em diretório temporário fora do repo,
   removido no fim, trap com valor capturado na instalação) e o da árvore atual.
2. Para cada roadmap em `docs/roadmaps/**/*.md`, roda `trackfw roadmap show <basename-sem-.md> --json` com os
   dois binários e compara; roda `trackfw validate` com os dois e compara. O `barrier` não entra: ele executa
   os gates e não tem modo de listagem. Diferença não vazia reprova e mostra o diff.
3. `--self-test` com braço positivo (fixture com divergência injetada reprova) e negativo; guarda de
   vacuidade (zero roadmaps comparados → reprova).

**Acceptance criteria:**
- [x] `bash scripts/check-marker-corpus-parity.sh --self-test` verde
- [x] `bash scripts/check-marker-corpus-parity.sh` rc=0 com ML-1A aplicado
- [x] Contra-braço: com um alias quebrado de propósito numa cópia temporária, o script reprova

### ML-1C — O validate nomeia o erro de cabeçalho duplicado (residual do ML-1A)
**Status:** ✅ Concluído

*Auditoria (Zeus, 2026-11-08):* os 3 testes novos do validator e o braço D3 de `TestWave0GateDiagnosis_NamesTheCause` existem e passam. O barrier já mostrava a mensagem do parser (exit 2). Residual apontado: cerca de gates não fechada também cai na mensagem fixa de "not followed by" → ML-1D.
**Squad:** apolo-tf
**Files affected:**
- `internal/validator/validator_roadmap_gates.go` e testes
- `internal/validator/validator_roadmap_gates_ml1c_test.go` (novo)
- `internal/roadmapdoc/wave0_gate_cause_test.go`
- `internal/roadmapdoc/roadmapdoc.go` (só `Wave0GateDiagnosis`/causas, se a causa nova morar lá) e testes
- `vault/notes/wave0gatediagnosis-d3-mensagem-errada-2026-11-08.md` (marcar como corrigido)

*Por que existe:* `Wave0GateDiagnosis` mapeia todo erro de `ParseGates` para `Wave0GateMalformed`, e o
validate imprime "`**Gates da wave:**` is not followed by a ```bash fence". Com D3, um cabeçalho duplicado
na Wave 0 aparece com a mensagem errada. Mesma causa (contrato dos cabeçalhos), mesma REQ.

**Acceptance criteria:**
- [x] Wave 0 com dois cabeçalhos: o validate emite mensagem que diz "duplicate" e nomeia as duas linhas
- [x] Mensagem de fence ausente cita os dois cabeçalhos aceitos, não só o português
- [x] `go test ./internal/validator/ ./internal/roadmapdoc/` verde

### ML-1D — O validate repassa a causa real do bloco de gates malformado
**Status:** ✅ Concluído

*Auditoria (Zeus, 2026-11-08):* `TestValidateRoadmapGateCoverage_MalformedUnterminatedFence_UsesParserDetail` existe e passa; os 13 testes de `roadmap_gate_coverage` verdes. Observação para o red team: `ParseGatesLines` só fecha a cerca de gates com ```` ``` ```` exato, enquanto o `FenceMask` aceita fechamento mais longo; o efeito é erro (fail-closed), não gate perdido.
**Squad:** apolo-tf
**Files affected:** `internal/validator/validator_roadmap_gates.go`, `internal/roadmapdoc/roadmapdoc.go` (só `Wave0GateDiagnosisDetail`) e testes

*Por que existe:* residual do ML-1C. `Wave0GateMalformed` cobre cerca não fechada e o fail-closed do
`FenceMaskCheck`, mas a mensagem diz sempre "is not followed by a ```bash fence". É a mesma mensagem fixa
que esconde a causa do parser, tocada no ML-1C; defeito conhecido se corrige agora.

**Acceptance criteria:**
- [x] Cerca de gates não fechada na Wave 0: a mensagem do validate traz o texto do erro do parser
- [x] Bloco sem cerca continua com a mensagem atual (as duas formas de cabeçalho)
- [x] `go test ./internal/validator/ ./internal/roadmapdoc/` verde

**Gates da wave:**
```bash
go build ./...
go test ./internal/roadmapdoc/ ./internal/commands/ ./internal/validator/
bash scripts/check-marker-corpus-parity.sh --self-test
bash scripts/check-marker-corpus-parity.sh
```

## Wave 2 — Red team do leitor e primeira tag (ADR D2)
> Dependencies: Wave 1 completa.

### ML-2A — Red team do leitor bilíngue
**Status:** ✅ Concluído

*Auditoria (Zeus, 2026-11-08):* veredito REPROVA, evidência conferida no parecer. Bloqueantes A2 (bloco ```bash dentro de cerca de exemplo escolhido como gate) e F (`Lapsed:` + NBSP/ZWSP conta como caducado). A1, D1, E3 e G têm o mesmo efeito (gate perdido ou término indevido) e são pré-existentes; entram aqui pela regra de defeito conhecido. Desvio do Hades: rodou `make quality` em background e criou um `.go` temporário na raiz, removido; árvore conferida limpa.
**Squad:** hades-tf
**Files affected:**
- `docs/seguranca/2026-11-08-redteam-leitor-bilingue.md` (novo)

**Actions:** reimplementar a partir da leitura da ADR, sem conferir o diff: procurar alias que perde gate,
texto comum que vira marcador, e combinação de cabeçalhos que escapa de D3. Achado vira ML corretivo nesta
wave.

**Acceptance criteria:**
- [x] Parecer com veredito e evidência por achado

### ML-2C — Gate nunca some em silêncio: correções do red team no parser
**Status:** ✅ Concluído
**Squad:** apolo-tf
**Files affected:**
- `internal/roadmapdoc/roadmapdoc.go`
- `internal/roadmapdoc/gates_lines_test.go`
- `internal/roadmapdoc/acceptance_lapsed_test.go`
- `internal/roadmapdoc/roadmapdoc_test.go`
- `docs/cli-parity.md`
- `internal/roadmapdoc/compare_baseline_test.go`

*Por que existe:* red team da Wave 2 (`docs/seguranca/2026-11-08-redteam-leitor-bilingue.md`). Os aliases
novos ampliaram superfícies que já falhavam em português.

**Actions:**
1. A2/A1: a busca da cerca ```bash depois do cabeçalho ignora linhas mascaradas (`fenced[j]`, `htmlMask[j]`).
2. D1/E3: anomalia de máscara que esconde ou pode esconder o cabeçalho de gates (comentário HTML não
   fechado; cerca não fechada até o fim do documento) faz `ParseGatesLines` retornar erro (fail-closed),
   nunca zero gates.
3. F: a razão de `Lapsed:`/`Caducou:` exige ao menos um caractere `\p{L}`, `\p{N}` ou `\p{P}`; NBSP, ZWSP e
   outros separadores/format chars sozinhos não contam.
4. G: `StatusCategory` aplica a mesma guarda de marca combinatória de `StatusIsComplete`; corrigir o
   comentário que afirma o contrário.
5. Medir no acervo (243 roadmaps) quantas waves mudam de seleção de gates ou passam a dar erro com 1 e 2,
   comparando a função antiga e a nova num teste temporário.

**Acceptance criteria:**
- [x] Um teste por achado (A2, A1, D1, E3, F com NBSP e ZWSP, G), cada um com contra-braço
- [x] Medição do acervo reportada; nenhum roadmap em `wip/` passa a reprovar
- [x] `go test ./internal/roadmapdoc/ ./internal/commands/ ./internal/validator/` verde

*Auditoria (Zeus, 2026-11-08):* 806 waves medidas entre HEAD~1 e a árvore, diff de seleção de gates vazio;
os 14 erros são D3 nas duas árvores. `compare_baseline_test.go` entrou no escopo porque o corpus congelado
de testdata tem uma cerca de três crases nunca fechada na wave 2, e o D8 passou a reprovar ali. O E3 ficou
com teste que **afirma o residual** (cerca balanceada que mascara o cabeçalho conta zero gates): a correção
não cabia no item 2, que trata só de máscara não fechada. Vai para o ML-2F. A forma NFC pré-composta
(`abandoñed` com U+00F1) não é divergência: `StatusIsComplete` e `StatusCategory` dobram o diacrítico do
mesmo jeito, que é a política da ADR de 2026-08-29 (`concluído` → `concluido`).

### ML-2F — E3: cabeçalho de gates só mascarado é erro
**Status:** ✅ Concluído
**Squad:** apolo-tf
**Files affected:**
- `internal/roadmapdoc/roadmapdoc.go`
- `internal/roadmapdoc/gates_lines_test.go`
- `internal/roadmapdoc/compare_baseline_test.go`

*Por que existe:* E3 do red team. Cerca de três crases com um ```` ```bash ```` interno fecha cedo pela
regra CommonMark; a cerca de "fechamento" abre outra que cobre o cabeçalho real. As cercas ficam
balanceadas, o D8 não dispara e a wave conta zero gates. O red team pediu REQ própria por ser outro
mecanismo (`scanFences`, não o laço do bloco), mas a ADR D8 governa o sintoma ("gate nunca some em
silêncio"), então fica nesta REQ.

**Actions:** em `ParseGatesLines`, uma wave sem cabeçalho de gates válido que tenha linha casando
`GatesHeaderRe` sob `fenced` ou `htmlMask` retorna erro que nomeia a linha. Medir antes no acervo quantas
waves caem na regra; se algum roadmap em `wip/` cair, parar e reportar.

**Acceptance criteria:**
- [x] O teste do residual E3 passa a afirmar erro, com contra-braço (wave sem cabeçalho nenhum continua zero gates sem erro)
- [x] Medição do acervo reportada; nenhum roadmap em `wip/` passa a reprovar
- [x] `go test ./internal/roadmapdoc/ ./internal/commands/ ./internal/validator/` verde

*Auditoria (Zeus, 2026-11-08):* acervo com 1 caso, `done/ROADMAP-2026-08-22-...` wave 2 (linha 226), e 0 em
`wip/`. A regra também inverteu três contratos que afirmavam "marcador mascarado sozinho conta zero gates"
(`TestParseGatesLines_F1_MaskedMarkerNotGate`, `TestParseGatesLines_HTMLComment_MarkerNotGate`,
`TestParseGatesLines_RealFile_Wave2MaskedOnlyReturnsError`). A inversão é intencional pela emenda D8. Bite proof
contra c76cd44d e mutações dos dois disjuntos (`fenced`, `htmlMask`) reprovam. `check-gates-falsify.sh`
(285) e `check-roadmap-barrier-contract.sh` (49) rc=0.

### ML-2D — Oráculo compara também os gates (lacuna do red team)
**Status:** ✅ Concluído
**Squad:** apolo-tf
**Files affected:**
- `internal/generators/roadmap_show_json.go`
- `internal/generators/roadmap_show_json_test.go`
- `scripts/check-marker-corpus-parity.sh`
- `docs/cli-parity.md`

**Actions:** `roadmap show --json` ganha, por wave, a lista de comandos de gate e o erro de parse (campo
aditivo). O oráculo compara esse campo quando os dois binários o emitem; enquanto a base não o tiver,
compara com o campo retirado e imprime a lista da head. Depende do ML-2C (mesmo `docs/cli-parity.md`). Roda em paralelo ao ML-2F (arquivos disjuntos) e documenta a regra do E3 em `docs/cli-parity.md`, porque o ML-2F não toca esse arquivo.

**Acceptance criteria:**
- [x] `go test` do pacote verde; `bash scripts/check-marker-corpus-parity.sh --self-test` e execução rc=0
- [x] `make parity-rest` rc=0

*Auditoria (Zeus, 2026-11-08):* campo `gates: {commands, error}` por wave. Reexecutei com a árvore em df216f07 mais o
diff do ML-2D: self-test 6/6, oráculo rc=0 em 243 roadmaps, e na head 188 waves com comandos e 15 com erro.
Listei as 15 uma a uma: 14 D3 e 1 E3, todas em `done/`; nenhuma em `wip/`. Corretivo (fim da Wave 2): a linha 435 usava `| grep -qF` sob pipefail e reprovou o
`make quality` (`check-no-grep-q-under-pipefail`), embora o relatório do ML afirmasse `make parity-rest` rc=0.
Corrigida para `| grep -F … >/dev/null`; o `make quality` rodado pelo Zeus com a máquina ociosa deu rc=0. Limite declarado: enquanto a `main`
não tiver o campo, o oráculo remove `gates` dos dois lados e só resume a head. A comparação de gates entre
base e head passa a valer depois que o leitor bilíngue chegar à `main` (ADR D2).

### ML-2E — Segunda rodada do red team
**Status:** ✅ Concluído
**Squad:** hades-tf
**Files affected:** `docs/seguranca/2026-11-08-redteam-leitor-bilingue.md` (seção nova "Rodada 2")
**Acceptance criteria:**
- [x] Os sete achados da rodada 1 refeitos contra a branch, com veredito

*Resultado:* REPROVA. Seis achados estão fechados. O F está PARCIAL, porque os preenchedores Hangul
(U+3164, U+115F, U+1160, U+FFA0, categoria Lo) ainda passam como razão. A sondagem (a) mediu o
quase-cabeçalho silencioso: espaço inicial, lista, blockquote e homóglifo. O red team o classificou como
residual aceito. O Zeus reclassificou: o sintoma é o mesmo do D8 (gate some em silêncio) e é defeito
conhecido, então entra nesta REQ (emenda da ADR). As duas correções ficam no ML-2G e a rodada 3 no ML-2H.

### ML-2G — Quase-cabeçalho de gates e razão só com preenchedor invisível são erro
**Status:** ✅ Concluído
**Squad:** apolo-tf
**Files affected:**
- `internal/roadmapdoc/roadmapdoc.go`
- `internal/roadmapdoc/gates_lines_test.go`
- `internal/roadmapdoc/acceptance_lapsed_test.go`
- `internal/roadmapdoc/compare_baseline_test.go`
- `docs/cli-parity.md`

**Actions:**
1. `lapsedLineHasContent`: uma runa só conta se também não estiver em `unicode.Other_Default_Ignorable_Code_Point`.
2. `ParseGatesLines`: uma linha fora de máscara na wave que não é cabeçalho válido, mas cujo esqueleto casa com um
   dos cabeçalhos, retorna erro que nomeia a linha (emenda da ADR, rodada 2). Medir o acervo antes; se algum
   roadmap em `wip/` cair na regra, parar e reportar.

**Acceptance criteria:**
- [x] Teste por vetor (1 a 3 espaços, lista, blockquote, homóglifo cirílico, maiúsculas, U+3164/U+115F/U+1160/U+FFA0), com contra-braço
- [x] Medição do acervo reportada; nenhum roadmap em `wip/` passa a reprovar
- [x] `go test ./internal/roadmapdoc/ ./internal/commands/ ./internal/validator/` verde

*Auditoria (Zeus, 2026-11-08):* o acervo não tem nenhum quase-cabeçalho nem nenhuma razão só com preenchedor.
Os 19 testes existem com os nomes reportados e passam; 14 deles reprovam contra 8e46da3e. Os gates de
falsificação (285), do contrato do barrier (49) e do oráculo (243) deram rc=0. O próprio ML declarou quatro
grafias que ainda escapam da regra: `**Wave gate:**` (singular), `**Wave gates**:` (dois-pontos fora do negrito),
`__Wave gates:__` e `**Wave gates :**`. Além delas, letras de largura total ou matemáticas não passam por NFKC.
São defeito conhecido do mesmo sintoma, então vão para o ML-2I. Achado colateral: o modelo de roadmap do
comando `/trackfw:roadmap` (`internal/generators/scaffold.go`, `roadmap.md`) está indentado em 3 espaços dentro
de uma cerca. Um agente que copie o modelo com a indentação agora recebe erro, em vez do zero gates silencioso
de antes, então a mudança não é regressão. A remoção da indentação entra no ML-3C.

### ML-2I — Quase-cabeçalho: grafias declaradas como residual pelo ML-2G
**Status:** ✅ Concluído
**Squad:** apolo-tf
**Files affected:**
- `internal/roadmapdoc/roadmapdoc.go`
- `internal/roadmapdoc/gates_lines_test.go`
- `internal/roadmapdoc/compare_baseline_test.go`
- `docs/cli-parity.md`

**Actions:** o esqueleto de `gatesNearMissSkeleton` passa por NFKC; `__` vira `**`; o espaço antes de `:` é removido;
`**x**:` vira `**x:**`. Os alvos aceitam `gate`/`gates` nas duas línguas. Medir o acervo antes e parar se
algum roadmap em `wip/` cair na regra.

**Acceptance criteria:**
- [x] Um teste por grafia (singular EN e PT, dois-pontos fora, `__`, espaço antes de `:`, largura total, negrito matemático), com contra-braço
- [x] Medição do acervo reportada; nenhum roadmap em `wip/` passa a reprovar
- [x] `go test ./internal/roadmapdoc/ ./internal/commands/ ./internal/validator/` verde

*Auditoria (Zeus, 2026-11-08):* a primeira entrega listava grafia por grafia e declarou mais 7 que escapavam.
No corretivo, o Zeus trocou a lista por um esqueleto só de letras: remove a marcação (`*_#|>-+~`) e os espaços
até o primeiro `:`, e compara com `wavegates:`, `wavegate:`, `gatesdawave:` e `gatedawave:`. A crase é
preservada, então a citação em prosa continua permitida. O `## Wave gates:` não é alcançável porque abre outra
seção; um teste confere o esqueleto. No acervo, 3 waves passam a dar erro, todas `**Gate da wave:**` no
singular, todas na Wave 0 e em `done/`. Esses gates nunca rodaram. Na head ficam 18 erros: 14 D3, 1 E3 e
3 quase-cabeçalho, nenhum em `wip/`. `check-gates-falsify.sh` 285 rc=0, rodado pelo Zeus em foreground. O WARN
`no-repo-mutation` de `check-channels-content.sh` não é falha: o laço de não mutação emite esse aviso
quando o script sai com rc≠0 sem alterar a árvore, e nenhum dos dois scripts difere da `main`. Contrato do
barrier 49 e oráculo 243 rc=0.

### ML-2H — Terceira rodada do red team
**Status:** ✅ Concluído
**Squad:** hades-tf
**Files affected:** `docs/seguranca/2026-11-08-redteam-leitor-bilingue.md` (seção nova "Rodada 3")
**Acceptance criteria:**
- [x] N-1 e os vetores da sondagem (a) refeitos contra a branch, com veredito; veredito final APROVA ou REPROVA

*Resultado:* REPROVA. N-1 e a sondagem (a) estão FECHADOS. A rodada achou 14 vetores novos: capitais
cirílicas e gregas (erro de ordem: o mapa de homóglifos roda antes da caixa baixa), VS1–VS14, Braille em
branco, forma sem dois-pontos, dois-pontos alternativos, HTML/entidade e `ɑ` latino. Também achou falso
positivo em prosa. O Zeus parou o ciclo de grafias: a emenda da ADR (rodada 3) fixa o modelo de ameaça
(acidente, não adversário), ancora a regra na cerca ligada e define o critério de convergência. Correção no
ML-2J e rodada 4 no ML-2K.

### ML-2J — Quase-cabeçalho ancorado na cerca, classes por categoria
**Status:** ✅ Concluído
**Squad:** apolo-tf
**Files affected:**
- `internal/roadmapdoc/roadmapdoc.go`
- `internal/roadmapdoc/gates_lines_test.go`
- `internal/roadmapdoc/compare_baseline_test.go`
- `docs/cli-parity.md`

**Actions:** itens 2 e 3 da emenda da ADR (rodada 3). A ligação cabeçalho→cerca vira função única, usada
pelo cabeçalho válido e pelo quase-cabeçalho. Medir o acervo antes e parar se algum roadmap em `wip/` cair
na regra.

**Acceptance criteria:**
- [x] Teste por vetor acidental da rodada 3 (capitais, VS1, U+2800, sem dois-pontos em negrito e em `###`, `∶`, `꞉`, `<strong>`, `&#58;`, `ɑ`), com contra-braço
- [x] Falso positivo da rodada 3 coberto: prosa "Wave gates: …" sem cerca ligada não dispara, com e sem cabeçalho válido
- [x] Medição do acervo reportada; nenhum roadmap em `wip/` passa a reprovar
- [x] `go test ./internal/roadmapdoc/ ./internal/commands/ ./internal/validator/` verde

*Auditoria (Zeus, 2026-11-08):* `boundGateFence` agora é a ligação única entre cabeçalho e cerca, usada pelo
cabeçalho válido e pelo quase-cabeçalho. Ela para em `## `/`### ` e, desde o corretivo, também em
`GatesHeaderRe`: uma cerca pertence ao cabeçalho mais próximo acima dela. Esse corretivo fecha o falso positivo
da prosa escrita antes de um cabeçalho válido. Os fixtures antigos de quase-cabeçalho, que não tinham cerca,
ganharam uma, e essa mudança está declarada. No acervo, um falso positivo de prosa em `done/` deixou de
disparar; 0 em `wip/`; 2 erros de quase-cabeçalho continuam em `done/`. Gates rodados pelo Zeus: falsificação
285, contrato do barrier 49 e `go test` dos três pacotes, todos rc=0. Os 22 testes novos existem pelos nomes
reportados.

### ML-2K — Quarta rodada do red team (critério de convergência)
**Status:** ✅ Concluído
**Squad:** hades-tf
**Files affected:** `docs/seguranca/2026-11-08-redteam-leitor-bilingue.md` (seção nova "Rodada 4")
**Acceptance criteria:**
- [x] Vetores da rodada 3 refeitos contra a branch, com veredito; vetores novos classificados como acidentais ou como residual do item 4 da emenda; veredito final pelo critério do item 5

*Resultado:* REPROVA. R3-A a R3-H estão FECHADOS. A rodada achou dois mecanismos acidentais novos. **A**: número
ou parênteses no rótulo (`**Wave 1 gates:**`, `**Gates da wave 0:**`, este com dois casos reais em `done/`).
**B**: a ligação pula a cerca com etiqueta diferente de `bash`, e o barrier pode executar um bloco de exemplo
posterior. Emenda da ADR (rodada 4); correção no ML-2L, verificação no ML-2M.

### ML-2L — A cerca ligada é a primeira cerca; rótulo tolera número e parênteses
**Status:** ✅ Concluído
**Squad:** apolo-tf
**Files affected:**
- `internal/roadmapdoc/roadmapdoc.go`
- `internal/roadmapdoc/gates_lines_test.go`
- `internal/roadmapdoc/compare_baseline_test.go`
- `docs/cli-parity.md`

**Actions:** emenda da ADR (rodada 4), mecanismos A e B. Medir o acervo antes e parar se algum roadmap em `wip/`
cair na regra.

**Acceptance criteria:**
- [x] Teste por vetor N2, N3, N4, N7, N8 e N9 da rodada 4, e por etiqueta (`sh`, `shell`, `Bash`, ` bash`, `~~~bash`), com contra-braço
- [x] Medição do acervo reportada; nenhum roadmap em `wip/` passa a reprovar
- [x] `go test ./internal/roadmapdoc/ ./internal/commands/ ./internal/validator/` verde

*Auditoria (Zeus, 2026-11-08):* no acervo, 4 waves de `done/` passam a dar erro e nenhuma de `wip/`. Três delas usam
`**Gates da wave 0:**`, e o corpus congelado registra os três casos. A quarta é
`ROADMAP-2026-09-28-trackfw-init-...` wave 0, cuja primeira cerca não tem etiqueta. O erro de gates aparece em
21 waves, todas em `done/`. O teste A2 mudou de sentido: uma cerca de exemplo entre o cabeçalho e o gate,
antes aceita, agora é erro, e o layout legal (gate primeiro, exemplo depois) tem teste próprio. Este próprio
roadmap continua sem erro nas cinco waves (`roadmap show --json`). Rodado pelo Zeus: falsificação 285,
contrato do barrier 49 e `go test` do pacote, todos rc=0; o hash do arquivo confere com o reportado.

### ML-2M — Verificação do red team (rodada 5, escopo fechado)
**Status:** ✅ Concluído
**Squad:** hades-tf
**Files affected:** `docs/seguranca/2026-11-08-redteam-leitor-bilingue.md` (seção nova "Rodada 5")
**Acceptance criteria:**
- [x] Vetores da rodada 4 refeitos com veredito; vetores novos só nos mecanismos A e B; veredito pelo critério do item 5 da emenda da rodada 3

*Resultado:* **APROVA**. N2 a N9 estão FECHADOS. No barrier ponta a ponta, ```` ```bash ```` com `exit 1` reprova
(rc=1) e ```` ```sh ```` sai com erro sem executar bloco nenhum (rc=2). Três fricções foram aceitas pelo Zeus
sem ML: cerca indentada 1–3 espaços e ```` ```bash ```` com espaço no fim são aceitas e o gate roda (nada some);
erro de parse sai com rc=2 em texto, sem JSON, mesmo com `--json`, que é o comportamento anterior desta REQ
para qualquer erro de parse. O ciclo de red team convergiu.

### ML-2B — Merge e tag do leitor (Zeus)

*Decisão do KG (2026-11-08):* dois PRs, com uma tag no meio. Este PR leva as Waves 0–2 (leitor bilíngue) e,
depois do merge, recebe a tag; as Waves 3–4 (gerador em inglês) vão num segundo PR. O KG também aceitou o
residual de ofuscação deliberada (emenda da ADR, rodada 3, item 4).
**Status:** ✅ Concluído
**Squad:** zeus-tf
**Files affected:**
- `CHANGELOG.md`
- `internal/version/version.go`
- `npm/package.json`
- `npm/package-lock.json`
- `pypi/pyproject.toml`

**Actions:** PR das Waves 0–2, merge pelo KG, release minor com o leitor bilíngue via `trackfw release tag`.
O roadmap continua em `wip/` depois desse merge (ADR D2).

**Acceptance criteria:**
- [x] Tag publicada nos três canais com o leitor bilíngue

**Gates da wave:**
```bash
test -s docs/seguranca/2026-11-08-redteam-leitor-bilingue.md
```

*Auditoria (Zeus, 2026-11-10):* o PR 1 (#677) e o PR de release (#678) entraram na `main`. `make check-required-full` deu OK (D\\R, R\\W e D\\W vazios). `trackfw release tag 11.6.0` publicou `v11.6.0` no commit `dc617634`. O run de release 42819567116 terminou em success, incluindo "Verify all release channels". GitHub: `v11.6.0` com pre=false; npm: `latest` = 11.6.0; PyPI: 11.6.0.

## Wave 3 — Gerador e mensagens em inglês (AC2, AC3, AC5)
> Dependencies: tag do ML-2B publicada. MLs detalhados a partir do inventário da Wave 0, com arquivos
> disjuntos e no máximo dois agentes em paralelo.

*Detalhamento (Zeus, 2026-11-10):* os MLs abaixo saem do inventário §1 e §4 do parecer da Wave 0
(`docs/seguranca/2026-11-08-wave0-idioma-ingles-primeiro.md`). O ML-3B original (cerca de 30 arquivos) foi
dividido em 3B1 e 3B2 por pacote, e o teste do AC2 virou o ML-3D, porque varre arquivos de todos os MLs e
só pode rodar depois deles. Ordem e paralelismo:
- **Par 1:** ML-3A (apolo-tf) em paralelo com ML-3C (prometeu-tf); arquivos disjuntos.
- **Par 2:** ML-3B1 (apolo-tf) em paralelo com ML-3B2 (apolo-tf); arquivos disjuntos.
- **Por último:** ML-3D (apolo-tf), sozinho.

`update.go` sai do ML-3B e fica inteiro no ML-3C, que tem o AC do `update`. `discover.go` fica inteiro no
ML-3B2. Os literais que **ficam em português** por desenho: as grafias PT dos leitores bilíngues (grupo 3,
ADR D1/D5); as palavras-chave de `probes.go` (`autenticação`, `migração`, `senha`, `banco`, `notificação`,
§1 nota 3); a sonda de caminho `"docs/requisições"` em `discover.go`; e os nomes dos presets (ADR D6).

### ML-3A — Gerador de roadmap e REQ na forma canônica (ADR D4)
**Status:** ✅ Concluído
**Squad:** apolo-tf
**Files affected:**
- `internal/generators/roadmap.go`
- `internal/generators/roadmap_test.go`
- `scripts/check-roadmap-barrier-contract.sh`
- `scripts/check-gates-falsify.sh` e `scripts/check-barrier.sh`, só se algum cenário consumir a **saída** do gerador

**Actions:** o gerador emite a forma D4: legenda `⬜ Pending · 🔄 In progress · ✅ Done · ❌ Blocked`,
`**Status:** ⬜ Pending`, `**Files affected:**`, `**Actions:**`, `**Acceptance criteria:**`, `**Wave gates:**`.
O leitor dentro de `roadmap.go` (G3-12) continua aceitando a forma PT. Os testes da tabela §4(i) passam a
afirmar a forma EN. `check-roadmap-barrier-contract.sh` (linha ~230) preenche o roadmap gerado com
`sed 's/^\*\*Status:\*\* ⬜ Pendente$/…/'`; passa a casar `⬜ Pending` e a escrever `✅ Done`. Cenários
que só **alimentam** PT ao parser (§4(ii)) ficam como estão.
**Acceptance criteria:**
- [x] Roadmap gerado usa a legenda, o status e os cabeçalhos da ADR D4
- [x] `scripts/check-gates-falsify.sh` e testes que fixavam a forma antiga atualizados no mesmo ML
- [x] Roadmap gerado passa no `barrier` com o gate `exit 1` reprovando (teste em `t.TempDir`)

*Auditoria (Zeus, 2026-11-10):* `roadmap.go` emite a forma D4 em 5 sítios, e o leitor G3-12 não foi tocado. Os 4 testes da §4(i) agora afirmam EN. Teste novo `TestNewRoadmap_ENGatesHeaderRecognizedByParser` (nome conferido): o gerador emite `**Wave gates:**` e `ParseGatesLines` extrai o gate `exit 1`; o executor provou que ele reprova com o gerador antigo. `check-roadmap-barrier-contract.sh`: 5 sítios que consomem a saída do gerador, 49 cenários OK. `go test ./internal/generators/ ./internal/identity/ ./internal/roadmapdoc/` rodado por Zeus: ok. Ficaram em PT mensagens G1 do próprio `roadmap.go` (linhas ~350–468); foram para o ML-3B2.

### ML-3C — Conteúdo gerado no consumidor e presets (AC3, AC5)
**Status:** ✅ Concluído
**Squad:** prometeu-tf
**Files affected:**
- `internal/generators/scaffold.go`
- `internal/generators/claudemd.go`
- `internal/generators/agentfiles.go`
- `internal/generators/update.go`
- `internal/identity/preset.go`
- `internal/generators/claudemd_test.go`
- `internal/generators/scaffold_test.go`
- `internal/generators/update_test.go`
- `.claude/commands/trackfw/*.md` (9 cópias versionadas, regeneradas a partir de `scaffold.go`)

`claudemd.go` e `agentfiles.go` são meta-geradores: instruem o agente consumidor a escrever `⬜ Pendente` e
passam a instruir a forma D4. Isso só é seguro depois da tag do leitor (Wave 0, achado 4), já publicada (v11.6.0).

*Acrescentado (auditoria do ML-2G):* o modelo de roadmap dentro de `roadmap.md` em `scaffold.go` perde a
indentação de 3 espaços, para que o cabeçalho de gates copiado fique na coluna 0. Teste: o modelo extraído da
cerca e passado a `ParseGatesLines` não dá erro na Wave 0 e retorna o gate `exit 1`.
**Acceptance criteria:**
- [x] `trackfw update` entrega a versão nova sem perder edição local (teste)
- [x] Nomes de preset mantidos, descrições em inglês (ADR D6)
- [x] Modelo de `roadmap.md` em coluna 0 e aceito por `ParseGatesLines` (teste)

*Auditoria (Zeus, 2026-11-10):* `scaffold.go`, `claudemd.go`, `agentfiles.go`, `update.go` e `preset.go` estão em EN, e o modelo de `roadmap.md` está na coluna 0 com `**Wave gates:**` e `⬜ Pending`. Testes novos, com nomes conferidos: `TestScaffold_RoadmapMD_GatesModelParseable` (o modelo passa em `ParseGatesLines` com `exit 1`) e `TestUpdate_AgentRulesBlockContainsENLifecycleMarkers` (o `update` entrega os marcadores EN e preserva o conteúdo fora do bloco gerenciado). Os nomes de preset não mudaram (D6). O texto descritivo dos presets ("Panteão grego…") está em `identity_wizard.go`, então o AC5 se completa no ML-3B1. Efeito colateral medido por Zeus: o cenário `falsify/update-config-loader/go-baseline` de `check-gates-falsify.sh` (~linha 3852) fixa a mensagem PT `trackfw-validate injetado` e reprova; a correção foi para o ML-3B2.

### ML-3B1 — Mensagens do CLI: pacote `commands` (AC2)
**Status:** ✅ Concluído
**Squad:** apolo-tf
**Files affected:**
- `internal/commands/help.go`
- `internal/commands/identity_wizard.go`
- `internal/commands/branch.go`
- `internal/commands/configure.go`
- `internal/commands/metrics.go`
- `internal/commands/agents_models.go`
- `internal/commands/integrations_flags.go`
- `internal/commands/commit.go`
- `internal/commands/req.go`
- `internal/commands/adr.go`
- `internal/commands/baseline.go`
- `internal/commands/barrier.go` (só a mensagem G1 da linha ~1842; o leitor G3-6 continua bilíngue)
- `internal/commands/doctor.go`, `internal/commands/init.go` (mensagens G1 achadas pelo grep do ML)
- `internal/commands/barrier_lapsed_test.go`
- `internal/commands/branch_new_base_e2e_test.go`
- `internal/commands/commit_test.go`
- `internal/commands/help_test.go`
**Acceptance criteria:**
- [x] Texto descritivo dos presets em `identity_wizard.go` em inglês, nomes mantidos (AC5, ADR D6)
- [x] Mensagens emitidas por esses arquivos em inglês; testes que afirmavam o texto PT atualizados

*Auditoria (Zeus, 2026-11-10):* o primeiro agente caiu por ENOTFOUND (falha de rede) e um segundo terminou a partir da árvore. O barrier emite `Lapsed:` e o leitor G3-6 não foi tocado. Os presets (`identity_wizard.go`) têm descrição EN, e os nomes e valores de opção foram mantidos; Chaves e Turma da Mônica são nomes próprios. O AC5 está fechado. Testes alterados, com nomes conferidos: `TestBarrierLapsed_JustificationInTextAndJSON`, `TestHelpUnknownTopicSuggestsNearMatch` e `TestCommit_Suggest_*`. `go test -count=1 ./internal/...` rodado por Zeus: rc=0.

### ML-3B2 — Mensagens do CLI: demais pacotes e dashboard (AC2)
**Status:** ✅ Concluído
**Squad:** apolo-tf
**Files affected:**
- `internal/validator/validator.go`
- `internal/validator/validator_git_branch_guard.go`
- `internal/validator/validator_credential_guard.go` (regra do `validate`; as mensagens de `internal/guard/` já estão em inglês pela REQ de primeira experiência, Done)
- `internal/validator/validator_roadmap_gates.go`
- `internal/validator/validator_req_done_criteria.go` (só a mensagem G1 da linha ~373)
- `internal/config/config.go` (só G1; `AcceptanceMarkers` continua bilíngue)
- `internal/discover/discover.go`
- `internal/generators/note.go`
- `internal/generators/nextsteps.go`
- `internal/generators/probes.go` (só mensagens; palavras-chave PT ficam)
- `internal/identity/identity.go`
- `internal/metrics/metrics.go`
- `internal/integrations/models.go`
- `internal/serve/static/app.js`
- `internal/serve/static/index.html`
- `internal/generators/roadmap.go` (só mensagens G1, ~linhas 350–468; o leitor G3-12 continua bilíngue)
- `scripts/check-gates-falsify.sh` (cenário `update-config-loader/go-baseline` e qualquer outro que fixe mensagem PT agora traduzida)
- `scripts/check-agent-models-parity.sh` (3 cópias congeladas de mensagens de `config.go`)
- `internal/identity/identity_test.go`
- `internal/validator/validator_git_branch_guard_test.go`
- `internal/validator/validator_req_contained_test.go`
**Acceptance criteria:**
- [x] Mensagens emitidas por esses arquivos em inglês; testes que afirmavam o texto PT atualizados
- [x] `bash scripts/check-gates-falsify.sh` com rc=0
- [x] Dashboard do `serve` verificado em navegador headless por Zeus (sem texto PT visível)

*Auditoria (Zeus, 2026-11-10):* o primeiro agente também caiu por ENOTFOUND e um segundo terminou. Os leitores PT estão preservados (`AcceptanceMarkers`, `reqACHeadingRe`, o leitor de `roadmap.go`, as palavras-chave de `probes.go` e a sonda `docs/requisições`). Medido por Zeus: `go test -count=1 ./internal/...` rc=0; `check-gates-falsify.sh` rc=0 (285 cenários); `check-agent-models-parity.sh` rc=0. Dashboard: `trackfw serve` com captura em Chrome headless; a interface (Board, Chain, Metrics, No items, Auto-refresh, Pause) está em EN, e os títulos de roadmap ficam em PT porque são dados de `docs/` (escopo negativo). Em `app.js`/`index.html` sobra PT só em comentários. Resta a mensagem da linha ~373 de `validator_req_done_criteria.go`, que está em EN mas cita só `Caducou:`; foi para o ML-3D.

### ML-3D — Teste que reprova o retorno do português (AC2)
**Status:** ✅ Concluído
**Squad:** apolo-tf
**Files affected:**
- `internal/i18nguard/pt_literals_test.go` e `internal/i18nguard/doc.go` (novos)
- Literais PT residuais achados pelo teste: `internal/commands/init.go`, `internal/commands/roadmap.go`, `internal/generators/roadmap.go`, `internal/generators/scaffold.go`, `internal/generators/scaffold_doctor.go`, com os goldens de `internal/generators/testdata/ci-trigger/` e os hashes AC6 de `consumer_install_test.go`
- `internal/generators/consumer_install_test.go`
- `internal/validator/validator_req_done_criteria.go` (mensagem da linha ~373 passa a citar `Lapsed:` com `Caducou:` como alternativa) e o teste que a afirmar

**Actions:** teste Go que percorre com `go/ast` os arquivos de produto de `internal/` e `cmd/` (sem
`_test.go` e `testdata/`) e reprova literal de string executável com palavra portuguesa. Lista de permissão
fechada e explícita: as grafias PT dos leitores bilíngues, as palavras-chave de `probes.go`, a sonda
`"docs/requisições"` e os nomes dos presets. Cada entrada traz o motivo.
**Acceptance criteria:**
- [x] O teste passa na branch e reprova quando um literal PT é reintroduzido (contraprova registrada)

*Auditoria (Zeus, 2026-11-10):* `TestNoPTLiteralsInProductGo` varre literais via `go/ast`. A lista de permissão é fechada, e `TestAllowlistIsExhaustive` reprova entrada obsoleta. Os arquivos citados na lista existem (conferido por Zeus). As classes são as quatro do ML mais uma quinta: mensagens EN que citam o marcador PT aceito; aceita porque nomeiam o leitor bilíngue. Contraprova do executor: `"Próximo passo"` em `init.go` reprova. Zeus achou um fail-open: o arquivo que não fazia parse era pulado em silêncio. O corretivo virou `t.Errorf`, com contraprova (`zz_contraprova.go` reprova; o arquivo foi apagado e o teste volta a passar). `go test -count=1 ./internal/...` rodado por Zeus: sem falha. Os nomes dos 4 testes foram conferidos.

**Gates da wave:**
```bash
go build ./...
go test ./internal/...
bash scripts/check-marker-corpus-parity.sh
```

## Wave 4 — Red team final e segunda tag
> Dependencies: Wave 3 completa.

### ML-4A — Red team do gerador e das mensagens
**Status:** 🔄 Em andamento
**Squad:** hades-tf
**Files affected:** `docs/seguranca/2026-11-08-redteam-gerador-ingles.md` (novo)
**Acceptance criteria:**
- [ ] Parecer com veredito; roadmap novo gerado pela branch passa no `barrier` com gates executados

**Gates da wave:**
```bash
test -s docs/seguranca/2026-11-08-redteam-gerador-ingles.md
make quality
```
