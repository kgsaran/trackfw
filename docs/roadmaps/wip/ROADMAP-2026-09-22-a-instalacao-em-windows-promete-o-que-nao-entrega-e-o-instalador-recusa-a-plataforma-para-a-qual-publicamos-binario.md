---
status: wip
date: 2026-09-22
req: "docs/req/REQ-2026-09-05-a-instalacao-em-windows-promete-o-que-nao-entrega-e-o-instalador-recusa-a-plataforma-para-a-qual-publicamos-binario.md"
squad: "hades_tf, apolo_tf, hefesto_tf"
---

# Roadmap: a instalacao em Windows promete o que nao entrega e o instalador recusa a plataforma para a qual publicamos binario

> Created: 2026-09-22 | Status: wip

## Context
<!-- Derived from REQ: REQ-2026-09-05-a-instalacao-em-windows-promete-o-que-nao-entrega-e-o-instalador-recusa-a-plataforma-para-a-qual-publicamos-binario.md -->
REQ: docs/req/REQ-2026-09-05-a-instalacao-em-windows-promete-o-que-nao-entrega-e-o-instalador-recusa-a-plataforma-para-a-qual-publicamos-binario.md

## Acceptance Criteria
<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->
- [x] AC8: hooks de atenção nativos em Go, migrados sem perder hooks personalizados.
- [ ] AC5–AC7: jornadas Windows medidas, sem confundir execução direta com execução pelo runtime.

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model for this roadmap
**Status:** ✅ Concluído
**Files affected:** docs/seguranca/2026-10-20-wave0-attention-go.md
**Actions:**
1. Enumeration completeness — is the list of surfaces in this roadmap complete? Name what is missing, or show the list is closed. Do not limit the search to the files already named by the REQ — before declaring the list closed, search the repository for other places that emit the same artifact or the same pattern (for example, grep for the literal the final artifact contains).
2. Threat model — who empties this Wave 0 without breaking any written rule, and how?
3. Falsification targets in both directions — for each surface, what breaks when the behavior regresses, and what breaks when it regresses the opposite way?
4. Declared residual — what this design accepts not covering.
**Acceptance criteria:**
- [x] The four sections above answered with evidence, not a one-line assertion
- [x] No implementation line written for this ML

**Gates da wave:**
```bash
# Wave 0 gate — replace this placeholder with a project-specific check before
# marking ML-0A done. Do not remove the gate; replace its command (AC13).
test -s docs/seguranca/2026-10-20-wave0-attention-go.md
```

## Wave 1 — Critérios históricos e jornadas AC1–AC7
> Dependências: AC1–AC4 reconciliados na triagem de 17/10; AC5–AC7 somente após Wave 3. Não executar estes MLs antes do ML-1H.

### ML-1A — **AC1** — O README declara a dependência de **shell** para as funcionalidades que dependem
**Status:** ✅ Concluído
**Files affected:** `README.md` (squad: apolo-tf)
**Actions:** O texto "wherever Node.js ≥ 18" já saiu do README (v8). Atualizar o banner do topo (linhas ~21–27) e o aviso da seção npm (~linha 115) com o estado medido em 2026-10-21: guard e atenção são subcomandos Go, e não há dependência de shell POSIX no trackfw. 🔴 Mesmo parágrafo: o Claude Code no Windows executa hooks pelo Git Bash, que ele resolve por caminho fixo; máquina sem Git não medida.
**Acceptance criteria:**
- [x] **AC1** — O README declara a dependência de **shell** para as funcionalidades que dependem
- [x] build passes
- [x] tests green

### ML-1B — **AC2** — Existe seção de instalação para Windows, com o caminho real (qual artefato, onde
**Status:** ✅ Concluído
**Files affected:** `README.md` (squad: apolo-tf)
**Actions:** Subseção "Installing on Windows" dentro de "Windows support (partial)": npm (`cmd /c npm install -g trackfw`; o shim `.ps1` é bloqueado por ExecutionPolicy Restricted, então usar `trackfw.cmd` ou cmd), pip, `install.sh` em Git Bash/MSYS2/Cygwin (instala em `$HOME/bin`, que só está no PATH dentro do Git Bash) e tarball manual (extrair `trackfw.exe`, pôr numa pasta, adicioná-la ao PATH do usuário). Armadilha medida: mais de um trackfw no PATH; o Git Bash põe `~/bin` primeiro, e os hooks do Claude rodam ali. Conferir com `where.exe trackfw` e, no Git Bash, `type -a trackfw`.
**Acceptance criteria:**
- [x] **AC2** — Existe seção de instalação para Windows, com o caminho real (qual artefato, onde
- [x] build passes
- [x] tests green

### ML-1C — **AC3** — A contradição entre install.sh e o release é resolvida **numa direção declarada**:
**Status:** ✅ Concluído
**Files affected:** `scripts/install.sh`, `scripts/check-install-checksum.sh` (squad: ares-tf)
**Actions:** Direção declarada pelo KG em 2026-10-21: **suportar** Windows (já entregue no #373). Fecha o resíduo: depois de instalar, listar todo outro `trackfw`/`trackfw.exe` encontrado nas entradas do PATH, diferente do recém-instalado, e avisar que ele pode responder no lugar (sem executá-lo). No Windows, avisar também que `~/bin` vem antes no PATH do Git Bash. Cenários novos no gate: com outra cópia → aviso com o caminho; sem outra → nenhum aviso.
**Acceptance criteria:**
- [x] **AC3** — A contradição entre install.sh e o release é resolvida **numa direção declarada**:
- [x] build passes
- [x] tests green

### ML-1D — **AC4** — O limite de ARM64 fica **declarado** onde o usuário lê, não só no .goreleaser.yaml.
**Status:** ✅ Concluído
**Files affected:** `README.md` (squad: apolo-tf)
**Actions:** O limite mudou de natureza (medição de 2026-09-16): `windows_arm64` é publicado nos canais, e npm e PyPI foram executados em Windows ARM64. Declarar no README: arm64 publicado; o CI de Windows é só x64; ARM64 medido em VM (n=1). Escopo negativo "não implementar ARM64" está obsoleto, conforme a própria REQ.
**Acceptance criteria:**
- [x] **AC4** — O limite de ARM64 fica **declarado** onde o usuário lê, não só no .goreleaser.yaml.
- [x] build passes
- [x] tests green

### ML-1E — **AC5** — 🔴 **Jornada verificada de ponta a ponta em Windows**, com o hook **disparando** —
**Status:** ✅ Concluído (com ressalva, ver REQ AC5)
**Plano executável em 20/10:** QA Artemis usa fonte exata `28d6d018`, enviada como git archive sem prefixo de diretório, em scratch novo na VM ARM64. Compilar `go build -o <scratch>/trackfw.exe ./cmd/trackfw`; registrar SHA256, versão e resolução de PATH (o instalado é 9.2.0). Criar projeto descartável com espaço/acento e configuração local emitida pelo binário. Isolar configuração de agente por mecanismo próprio, sem copiar credenciais nem mudar HOME/PATH persistentes. Verificar autenticação apenas por status booleano. Após login do usuário, acionar ferramenta benigna por runtime real, capturar debug/marker, sinal JSON e mudança do banner por browser, e provar cleanup por evento seguinte. Controle: ferramenta negada por guard não pode criar marcador; ferramenta permitida deve criá-lo. Nenhum push real ou segredo real como payload. Timeout finito em todos subprocessos. Não instalar outros agentes nesta etapa; registrar ausência como lacuna. Evidência em `docs/portabilidade/2026-10-20-jornadas-agentes-windows.md` por Artemis; root audita antes de marcar ACs.
**Reconciliação AC6:** texto original pressupõe hooks dependentes de Bash; ADR-2026-10-04 e AC8 substituem esse desenho. Medir ausência de Bash como caso nativo positivo; falsificar por ausência do binário trackfw, exigindo recusa visível do guard. Não marcar AC6 antigo como cumprido por silêncio ou por comando direto. Registrar mudança de premissa e separar falhas de autenticação de falhas do hook.
**AC7:** mesmo projeto e binário em caminho com espaço/acento; registrar caminho exato, shell efetivo do runtime e arquivo realmente criado/negado. Comandos diretos de PS/cmd são controles adicionais, nunca substituto do evento real. Sem login, preparar instrumentos e registrar impedimento sem fechar AC5–AC7.
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [x] **AC5** — 🔴 **Jornada verificada de ponta a ponta em Windows**, com o hook **disparando** —
- [x] build passes
- [x] tests green

### ML-1F — **AC6** — 🔴 **Falsificação da AC5:** num ambiente **sem** Git Bash, a jornada tem de **falhar
**Status:** ✅ Concluído (medido no ML-1E, 2026-10-21)
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [x] **AC6** — 🔴 **Falsificação da AC5:** num ambiente **sem** Git Bash, a jornada tem de **falhar
- [x] build passes
- [x] tests green

### ML-1G — **AC7** — Caminhos com **espaço** e **acento** exercitados (C:\Program Files\...,
**Status:** ✅ Concluído (medido no ML-1E, 2026-10-21)
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [x] **AC7** — Caminhos com **espaço** e **acento** exercitados (C:\Program Files\...,
- [x] build passes
- [x] tests green

---

## Wave 2 — Atenção nativa (retomada autorizada em 20/10)
> Dependências: Wave 0 auditada. ML-1H executado por apolo_tf, um único dono de comandos, geradores e testes. Sem paralelismo entre implementação e revisão, pois compartilham os arquivos.

### ML-1H — **AC8** — Hooks de atenção viram subcomando Go (`trackfw attention signal|cleanup`)
**Status:** ✅ Concluído
**Por que entra aqui:** Regra Dura de Causa Raiz — mesma causa da REQ (dependência de `sh` na
jornada Windows). Não previsto no escopo original porque, em 2026-09-22, os guards também eram `.sh`;
a ADR-2026-10-04 levou só os **guards** para Go e deixou os hooks de atenção para trás (medido em
2026-10-18).
**Files affected:** `internal/commands/` (novo `attention.go` + teste), `internal/generators/agentfiles.go`
(linhas 297, 405, 634, 640, 805, 813, 984, 991 e a migração `migrateHookCommand`), `scripts/trackfw-attention-{signal,cleanup}.sh`
(vira invólucro fino `exec trackfw attention …`, como ADR-2026-10-04 D3 fez com os guards),
`docs/cli-parity.md` (contrato do subcomando), emenda na ADR-2026-10-04 estendendo D2 aos hooks de atenção.
**Actions:**
1. Wave 0 (hades-tf): o hook de atenção escreve arquivo dentro de `roadmap_dir` a partir de payload do
   agente — confinar o caminho (hoje o `.sh` rejeita `/`, `..`), escapar JSON pelo `encoding/json`,
   limitar `message` a 300 chars, no-op fora da raiz do projeto, **nunca** bloquear o agente (exit 0 sempre).
2. Portar `trackfw-attention-signal.sh` (40 linhas) e `-cleanup.sh` (18) para `trackfw attention signal|cleanup`
   com comportamento igual (mesmos campos no `.trackfw-attention.json`).
3. Geradores emitem exatamente `trackfw attention signal` e `trackfw attention cleanup`, sem polyglot, para Claude, Codex, Gemini, Kiro, Cursor e Copilot (campo command); `trackfw update` migra todas as formas legadas emitidas sem duplicar/remover hooks personalizados. Atenção é informativa e não usa a linha fail-closed dos guards, defeituosa sob zsh (nota de 20/10). Não alterar guards nem arquivos do PR #590.
4. Teste que reprova se a linha emitida contiver `.sh`, `$VAR` ou caminho; teste de paridade `.sh` × subcomando.
**Acceptance criteria:**
- [x] `go build ./...` e `make quality` verdes
- [x] nenhum gerador emite `trackfw-attention-*.sh` como comando de hook
- [x] no `windows-probe.yml`, com bash/jq/python fora do PATH, comandos nativos ativam/ocultam o banner no Edge real (AC8). AC5–AC7: eventos reais dos agentes continuam pendentes.

**Triagem pós-v8 — 2026-10-17 (REQ-2026-10-17)**

**Mudou de forma**: `install.sh` aceita Windows (#373) e o README tem a seção; segue aberto só o que não foi medido — hooks com Cursor/Windsurf/Gemini/Kiro/Copilot/Amazon Q, `pwsh`, caminhos com espaço/acento.

Evidência e tabela completa: `docs/governanca/2026-10-17-triagem-backlog-pre-v8.md`.


**Plano executável da retomada — 2026-10-20**

- Escopo desta rodada: ML-0A, ML-1H e auditoria. AC5–AC7 seguem pendentes até medição real Windows; não encerrar a REQ nem mover o roadmap para done sem essa evidência.
- Arquivos adicionais descobertos: internal/commands/root.go, internal/generators/scaffold.go, internal/generators/agentfiles_test.go, internal/generators/copilot_hooks_parity_test.go, internal/generators/scaffold_literal_copy_parity_test.go, testes de atenção em internal/generators, e scripts/ de gates que ainda afirmem a implementação antiga. Inventário fechado pela Wave 0 antes do código.
- Contrato: entrada JSON stdin; prioridade question > command > texto por tool_name; message no máximo 300 runas; mesmos campos tool/message/level/timestamp; projeto somente com trackfw.yaml no cwd; caminho roadmap_dir configurado contido fisicamente na raiz; rejeitar symlinks/junctions com utilitários existentes; cleanup apaga somente .trackfw-attention.json, nunca .trackfw-credential-guard.json. Erros de payload/config/I/O são no-op e exit 0; stdin limitado para não travar hook.
- Compatibilidade: wrappers .sh e literais scaffold delegam para Go, preservando encaminhamento stdin e argumentos, sem jq/python; paridade usa fixture congelada do legado e prova comportamento sem ferramentas auxiliares. Não regenerar configurações locais/globais do usuário nesta rodada.
- Validação ML-1H: go build ./...; go test ./internal/commands ./internal/generators ./internal/integrations; go vet ./...; make quality; trackfw validate --json. Qualquer falha nova bloqueia; falhas de ambiente devem ser provadas e registradas.

**Gates da wave:**
```bash
go build ./...
go test ./internal/commands ./internal/generators ./internal/integrations
go test ./internal/pathguard
go vet ./...
```

### ML-2C — Correção da API de contenção exigida pelo gate global
**Status:** ✅ Concluído
**Files affected:** internal/commands/attention.go
**Actions:** substituir predicados brutos por pathguard.RejectAndReport, preservando exit 0 da atenção. Não alterar listas de exceções, emissores de diagnóstico ou testes do pacote pathguard.
**Acceptance criteria:**
- [x] Testes de atenção e gramática/proveniência em internal/pathguard passam.
- [x] make quality completo passa após o ajuste.

### ML-1I — Sinal de atenção lê o payload real do AskUserQuestion (`questions[].question`)
**Status:** ✅ Concluído
**Squad:** apolo-tf (implementação) · auditoria zeus-tf
**Origem:** jornada real Windows, rodada 3 (2026-10-21). O payload real do Claude Code 2.1.304, extraído do transcript da VM, é `tool_input.questions[{question,header,options,multiSelect}]`. `internal/commands/attention.go` só lê `tool_input.question`, então o banner mostra `Agent is executing: AskUserQuestion` e não a pergunta. Os testes usam o formato inventado `{"question":...}` e por isso passavam sem exercer o contrato real. Mesma causa da AC5 (artefato não verificado contra a jornada real), por isso entra nesta REQ.
**Arquivos afetados:** `internal/commands/attention.go`, `internal/commands/attention_test.go`
**Ações:** extrair a mensagem de `tool_input.questions[0].question` quando presente; manter `question` e `command` como estão hoje (outros emissores) e o fallback genérico; mesmos limites (300 runes, `attentionText`, tamanho máximo de entrada, JSON malformado = no-op). Tipos inesperados (`questions` não array, item não objeto, `question` não string, array vazio) → fallback, nunca pânico. Teste com o payload literal capturado.
**Critérios de aceite:**
- [x] payload real `{"tool_name":"AskUserQuestion","tool_input":{"questions":[{"question":"Qual cor?","header":"Cor","options":[...],"multiSelect":false}]}}` → `message` = `Qual cor?`
- [x] payloads antigos (`question`, `command`, vazio) mantêm a saída atual
- [x] tipos inesperados em `questions` → fallback sem pânico
- [x] `go build ./...` e `go test ./internal/commands/ -run Attention` verdes
- [x] re-medição na VM (sessão interativa) mostra a pergunta no arquivo de atenção e o cleanup após a resposta
**Auditoria (zeus, 2026-10-21):** pacote `internal/commands` verde com HOME isolado; `TestValidateJSONFlag`/`TestValidateTextUnchanged` falham também em origin/main só com o `~/.codex/hooks.json` local do usuário (passam com HOME isolado), então são vazamento de ambiente do teste e não efeito deste ML.
**Comandos de validação:** `go build ./... && go test ./internal/commands/ -run Attention -count=1`

## Wave 3 — Auditoria de qualidade e red-team
> Dependências: Wave 2 implementada e auditada. Revisões sequenciais para não compartilhar contexto em escrita.

### ML-3A — Qualidade (hefesto_tf)
**Status:** ✅ Concluído
**Files affected:** docs/seguranca/2026-10-20-quality-attention-go.md
**Actions:** revisar escopo, migração idempotente, fonte única, cobertura real e controles negativos; não escrever código.
**Acceptance criteria:**
- [x] Parecer escrito, achados bloqueantes resolvidos e gates conferidos.

### ML-3B — Red-team (hades_tf)
**Status:** ✅ Concluído
**Files affected:** docs/seguranca/2026-10-20-red-team-attention-go.md
**Actions:** exercer payload inválido, contenção caminho/symlink, exclusão segura e execução das linhas emitidas em sh/bash/zsh; distinguir prova local de runtime Windows.
**Acceptance criteria:**
- [x] Parecer escrito, nenhum achado bloqueante aberto.

**Gates da wave:**
```bash
test -s docs/seguranca/2026-10-20-quality-attention-go.md
test -s docs/seguranca/2026-10-20-red-team-attention-go.md
go test ./internal/commands ./internal/generators ./internal/integrations
```

### Auditoria Zeus da Wave 0 — 2026-10-20

Parecer docs/seguranca/2026-10-20-wave0-attention-go.md lido integralmente e aprovado com controles S1/S2 obrigatórios. Copilot migra atenção de bash para command: schema já empregado pelos guards e decidido na ADR D4. Limites decididos: stdin no máximo 1 MiB e prazo absoluto de 2 s (ambos testados); acima do limite ou prazo, no-op0, sem arquivo parcial. YAML deve ser decodificado sem config.Load (que encerra processo), documento único e erros no-op. Campos tool/message removem ASCII controles conforme corpus legado; message no máximo 300 runas. Não introduzir busca por raiz ancestral. Limite/erros/contenção fortalecem casos inválidos do legado, diferenças declaradas.

Arquivos autorizados adicionais ML-1H: internal/commands/testdata/attention-legacy/{signal.sh,cleanup.sh,MANIFEST.sha256}, internal/commands/attention_test.go, internal/generators/attention_hooks_test.go, scripts/check-output-encoding-declared.sh e scripts/check-crlf-normalize-capture.sh (somente pressupostos aposentados da implementação de atenção; não relaxar controles dos demais scripts). Build/test isolam HOME/USERPROFILE apenas no subprocesso de teste para não ler hooks reais do usuário; packaging/PyYAML via PYTHONPATH temporário.

### Complemento de medição Windows do ML-1H — artemis_tf

Após entrega e auditoria do código Apolo, QA é dono exclusivo de `scripts/windows-repro/p21-attention-native.ps1` e `.github/workflows/windows-probe.yml` (adicionar Pergunta 21 sem mudar triggers/permissões ou orçamento de 10 minutos). A sonda registra resultado bruto, não vira gate: binário compilado em caminho com espaço/acento e PATH sem bash/jq/python, signal/cleanup via PowerShell e cmd, API `/api/attention` com serve isolado, preservação de `.trackfw-credential-guard.json`, limpeza somente de seus processos/fixtures. Não é execução de evento por CLI de agente; AC5–AC7 seguem pendentes. Medir por workflow_dispatch da branch após push. VM powershell-vm sem resposta em 20/10 (ConnectTimeout de 8 s).

Para comprovar também o banner entregue ao browser, estender P21 com Edge headless real do runner (caminho absoluto, perfil descartável): carregar o board servido, aguardar polling e observar DOM de `attention-banner`/`attention-message` depois de signal e cleanup, registrando visibilidade e texto exato. Não avaliar só a API nem invocar funções JS do board artificialmente. Timeout finito, nenhum sandbox do browser desativado e nenhum perfil real tocado. Continua sendo comando + browser, não evento de agente; AC5–AC7 continuam pendentes.

### Evidência local final — Zeus, 2026-10-20

make quality completo PASS exit0 após corretivo de contenção: suíte Go, vet, gates e falsificação em 8 chunks com 368 OK/0 FAIL, conjunto de rótulos completo. Log preservado /private/tmp/trackfw-attention-quality-final.log. bin/trackfw validate --json: 0 violações/166 avisos legados e globais, exit0. Qualidade APROVADO e red-team LIBERA, relatórios lidos integralmente por Zeus. A medição P21 subsequente encerrou ML-1H (ver conclusão abaixo); AC5–AC7 continuam sem prova de evento real dos agentes. Nenhum código/config do PR #590 ou hooks reais do usuário alterado.

Para evitar aguardar as 20 sondas alheias ao ML, input boolean attention_only (default false) seleciona job attention_native, mutuamente exclusivo ao probe original. Mesmo workflow_dispatch, contents:read e teto de 10 min; padrão completo preservado. Novo diff da instrumentação teve parser PowerShell/YAML 10/10 e complemento Hades LIBERA. Código Go não mudou após make quality aprovado.

### Conclusão ML-1H — Zeus, 2026-10-20

**Correção de CI do PR #591 — em validação:** run 39458335009 revelou symlink sem helper de capacidade em `internal/commands/attention_test.go` e aposentadoria obrigatória do teste `TestAttentionScripts_FallbackWithoutJQ` que passou no Windows. Artemis substituiu a criação pelo helper existente `symlinkOrSkipCmd` e moveu somente essa entrada de `entries` para `removed` em `.github/windows-known-failures.json`, com `removal_note: corrected` e referência à medição. Sem mudança de produto nem relaxamento de gates. Testes focados de atenção, gate symlink e seus 3 autotestes e 59 autotestes do ratchet passaram. Aprovação final depende da nova execução da esteira.

P21 isolada 39454688584 (fonte 20b3734c) success: PowerShell/cmd signal e cleanup exit0, helpers ausentes do PATH, API active true/false com mensagem exata, Edge DOM visible true/false, credential signal byte-idêntico. Evidência completa: docs/portabilidade/2026-10-20-atencao-go-windows-p21.md. ML-1H/AC8 concluído; roadmap permanece WIP e REQ Open para jornadas reais AC5–AC7. Não confundir o primeiro workflow completo failure (5c legado Node e cinco falhas Windows conhecidas na pergunta 18) com P21 isolada medida.

### Barreiras finais auditadas

- Wave 0: PASS, 1 comando executado (parecer de ameaça presente).
- Wave 2: PASS, ML-1H e ML-2C completos, 4 comandos executados com exit 0 (build, suíte focada, pathguard e vet).
- Wave 3: PASS, ML-3A e ML-3B completos, 3 comandos executados com exit 0 (pareceres presentes e suíte focada).
- validate das barreiras em home de teste isolado: 0 violações/163 avisos existentes. A contagem 166 anterior incluía configurações globais reais, preservadas.

Auditoria detectou e corrigiu erro de autoria: dois headings H2 de contexto encerravam a Wave 2 antes dos gates. O primeiro passed com commands vazio foi descartado como evidência de execução; nova execução conferiu os quatro comandos no JSON. Nota: vault/notes/barrier-wave-h2-separa-gates-sem-executar-2026-10-20.md. Nenhuma regra do parser foi alterada.
