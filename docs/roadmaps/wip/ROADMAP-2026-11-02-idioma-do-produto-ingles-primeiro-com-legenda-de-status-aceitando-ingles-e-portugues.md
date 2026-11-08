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
**Status:** 🔄 Em andamento
**Squad:** apolo-tf
**Files affected:**
- `internal/roadmapdoc/roadmapdoc.go`
- `internal/roadmapdoc/gates_lines_test.go`
- `internal/roadmapdoc/acceptance_lapsed_test.go`
- `internal/roadmapdoc/roadmapdoc_test.go`
- `docs/cli-parity.md`

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
- [ ] Um teste por achado (A2, A1, D1, E3, F com NBSP e ZWSP, G), cada um com contra-braço
- [ ] Medição do acervo reportada; nenhum roadmap em `wip/` passa a reprovar
- [ ] `go test ./internal/roadmapdoc/ ./internal/commands/ ./internal/validator/` verde

### ML-2D — Oráculo compara também os gates (lacuna do red team)
**Status:** ⬜ Pendente
**Squad:** apolo-tf
**Files affected:**
- `internal/generators/roadmap_show_json.go` e testes
- `scripts/check-marker-corpus-parity.sh`
- `docs/cli-parity.md`

**Actions:** `roadmap show --json` ganha, por wave, a lista de comandos de gate e o erro de parse (campo
aditivo). O oráculo compara esse campo quando os dois binários o emitem; enquanto a base não o tiver,
compara com o campo retirado e imprime a lista da head. Depende do ML-2C (mesmo `docs/cli-parity.md`).

**Acceptance criteria:**
- [ ] `go test` do pacote verde; `bash scripts/check-marker-corpus-parity.sh --self-test` e execução rc=0
- [ ] `make parity-rest` rc=0

### ML-2E — Segunda rodada do red team
**Status:** ⬜ Pendente
**Squad:** hades-tf
**Files affected:** `docs/seguranca/2026-11-08-redteam-leitor-bilingue.md` (seção nova "Rodada 2")
**Acceptance criteria:**
- [ ] Os sete achados da rodada 1 refeitos contra a branch, com veredito

### ML-2B — Merge e tag do leitor (Zeus)
**Status:** ⬜ Pendente
**Squad:** zeus-tf
**Actions:** PR das Waves 0–2, merge pelo KG, release minor com o leitor bilíngue via `trackfw release tag`.
O roadmap continua em `wip/` depois desse merge (ADR D2).

**Acceptance criteria:**
- [ ] Tag publicada nos três canais com o leitor bilíngue

**Gates da wave:**
```bash
test -s docs/seguranca/2026-11-08-redteam-leitor-bilingue.md
```

## Wave 3 — Gerador e mensagens em inglês (AC2, AC3, AC5)
> Dependencies: tag do ML-2B publicada. MLs detalhados a partir do inventário da Wave 0, com arquivos
> disjuntos e no máximo dois agentes em paralelo.

### ML-3A — Gerador de roadmap e REQ na forma canônica (ADR D4)
**Status:** ⬜ Pendente
**Squad:** apolo-tf
**Files affected:** `internal/generators/roadmap.go` e testes; lista final após a Wave 0.
**Acceptance criteria:**
- [ ] Roadmap gerado usa a legenda, o status e os cabeçalhos da ADR D4
- [ ] `scripts/check-gates-falsify.sh` e testes que fixavam a forma antiga atualizados no mesmo ML

### ML-3B — Mensagens do CLI (AC2)
**Status:** ⬜ Pendente
**Squad:** apolo-tf
**Files affected:** `internal/commands/help.go` e demais apontados pela Wave 0.
**Acceptance criteria:**
- [ ] Teste que reprova literal em português nas mensagens emitidas

### ML-3C — Conteúdo gerado no consumidor e presets (AC3, AC5)
**Status:** ⬜ Pendente
**Squad:** prometeu-tf
**Files affected:** `internal/generators/scaffold.go`, `claudemd.go` (linha ~36), `agentfiles.go` (linhas ~112-113),
`update.go`, `internal/identity/preset.go` e demais da tabela §1 do parecer da Wave 0. `claudemd.go` e
`agentfiles.go` são meta-geradores: instruem o agente consumidor a escrever `⬜ Pendente`; só mudam depois
da tag do leitor (Wave 0, achado 4).
**Acceptance criteria:**
- [ ] `trackfw update` entrega a versão nova sem perder edição local (teste)
- [ ] Nomes de preset mantidos, descrições em inglês (ADR D6)

**Gates da wave:**
```bash
go build ./...
go test ./internal/...
bash scripts/check-marker-corpus-parity.sh
```

## Wave 4 — Red team final e segunda tag
> Dependencies: Wave 3 completa.

### ML-4A — Red team do gerador e das mensagens
**Status:** ⬜ Pendente
**Squad:** hades-tf
**Files affected:** `docs/seguranca/2026-11-08-redteam-gerador-ingles.md` (novo)
**Acceptance criteria:**
- [ ] Parecer com veredito; roadmap novo gerado pela branch passa no `barrier` com gates executados

**Gates da wave:**
```bash
test -s docs/seguranca/2026-11-08-redteam-gerador-ingles.md
make quality
```
