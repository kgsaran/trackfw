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
**Status:** 🔄 Em andamento
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
- [ ] As seis seções respondidas com evidência (comando e resultado), não com afirmação de uma linha
- [ ] Tabela de inventário com arquivo, linha, grupo e direção (emite/lê) para cada sítio
- [ ] Item 3 medido com o binário da `main`, com o comando e a saída
- [ ] Nenhuma linha de implementação escrita

### ML-0B — Corretivo: inventário completo e releitura do grupo 3
**Status:** 🔄 Em andamento
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
- [ ] Todos os arquivos da medição do arquiteto presentes na tabela
- [ ] Cada arquivo da ação 2 com veredito leitor/não leitor e linha citada

**Gates da wave:**
```bash
test -s docs/seguranca/2026-11-08-wave0-idioma-ingles-primeiro.md
```

## Wave 1 — Leitor bilíngue (AC4, ADR D1/D3/D7)
> Dependencies: Wave 0 auditada. ML-1A e ML-1B tocam arquivos disjuntos e correm em paralelo.

### ML-1A — Aliases em inglês nos parsers
**Status:** ⬜ Pendente
**Squad:** apolo-tf
**Files affected:**
- `internal/roadmapdoc/roadmapdoc.go`, `internal/roadmapdoc/*_test.go`
- `internal/commands/barrier.go` e testes, se o inventário apontar marcador lido ali
- `internal/validator/*.go` e testes, idem
- demais leitores apontados pela Wave 0 (lista fechada no handoff)

**Actions:**
1. `GatesHeaderRe` aceita `**Gates da wave:**` e `**Wave gates:**`.
2. `LapsedContinuationRe` aceita `Caducou:` e `Lapsed:` com a mesma regra (≥2 espaços, não tab, não vazio).
3. `terminatedVocabulary` ganha `abandoned`; desempate do `❌` aceita `cancelled` e `canceled`.
4. Dois cabeçalhos de gates na mesma wave (fora de cerca/comentário), em qualquer combinação de língua,
   viram erro que nomeia as duas linhas (ADR D3).
5. Testes: braço positivo por alias novo; `❌ Blocked` e `❌ Bloqueado` continuam pendentes; os dois
   cabeçalhos dão erro; grafia fora do conjunto (`**Wave Gate:**`, `Lapsed :`) não é reconhecida.
   Mensagens novas em inglês.

**Acceptance criteria:**
- [ ] `go build ./...` sem erro
- [ ] `go test ./internal/roadmapdoc/ ./internal/commands/ ./internal/validator/` verde
- [ ] Cada teste novo com a frase do que afirma (Regra Dura de Reconciliação)

### ML-1B — Oráculo do acervo (ADR D7)
**Status:** ⬜ Pendente
**Squad:** ares-tf
**Files affected:**
- `scripts/check-marker-corpus-parity.sh` (novo)
- `Makefile` (ligar em `parity-rest`, com `--self-test` e execução)

**Actions:**
1. O script compila o binário de `origin/main` (via `git worktree` em diretório temporário fora do repo,
   removido no fim, trap com valor capturado na instalação) e o da árvore atual.
2. Para cada roadmap em `docs/roadmaps/**/*.md`, roda `trackfw barrier <roadmap> --wave <N> --dry-run`
   (ou o modo de listagem que exista; confirmar no `--help`) para cada wave, e `trackfw validate`, com os
   dois binários, e compara as saídas normalizadas. Diferença não vazia reprova e mostra o diff.
3. `--self-test` com braço positivo (fixture com divergência injetada reprova) e negativo; guarda de
   vacuidade (zero roadmaps → reprova).

**Acceptance criteria:**
- [ ] `bash scripts/check-marker-corpus-parity.sh --self-test` verde
- [ ] `bash scripts/check-marker-corpus-parity.sh` rc=0 com ML-1A aplicado
- [ ] `make parity-rest` verde

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
**Status:** ⬜ Pendente
**Squad:** hades-tf
**Files affected:**
- `docs/seguranca/2026-11-08-redteam-leitor-bilingue.md` (novo)

**Actions:** reimplementar a partir da leitura da ADR, sem conferir o diff: procurar alias que perde gate,
texto comum que vira marcador, e combinação de cabeçalhos que escapa de D3. Achado vira ML corretivo nesta
wave.

**Acceptance criteria:**
- [ ] Parecer com veredito e evidência por achado

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
**Files affected:** `internal/generators/scaffold.go`, `claudemd.go`, `agentfiles.go`, `update.go`,
`internal/identity/preset.go` e demais apontados pela Wave 0.
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
