---
status: wip
date: 2026-10-06
req: "docs/req/REQ-2026-10-06-credential-guard-nao-cobre-windsurf-e-amazon-q-e-a-premissa-da-adr-2026-08-05-que-excluiu-o-windsurf-caducou.md"
squad: ""
---

# Roadmap: credential guard nao cobre Windsurf e Amazon Q, e a premissa da ADR-2026-08-05 que excluiu o Windsurf caducou

> Created: 2026-10-06 | Status: wip

## Context
<!-- Derived from REQ: REQ-2026-10-06-credential-guard-nao-cobre-windsurf-e-amazon-q-e-a-premissa-da-adr-2026-08-05-que-excluiu-o-windsurf-caducou.md -->
REQ: docs/req/REQ-2026-10-06-credential-guard-nao-cobre-windsurf-e-amazon-q-e-a-premissa-da-adr-2026-08-05-que-excluiu-o-windsurf-caducou.md

## Acceptance Criteria
<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->
- [x] AC1 da REQ — eventos de hook do Windsurf e do Amazon Q remedidos com fonte oficial e data (ML-0B)
- [x] AC2 da REQ — adendo à ADR-2026-08-05 com a decisão por CLI (ML-1A)
- [x] AC3 da REQ — `init`/`update`/`update harness` emitem o credential guard para cada CLI decidido "instalar"; `validate` deixa de silenciar; teste nas duas direções (ML-1B)
- [x] AC5 da REQ — o guard lê o payload de cada CLI (ML-1C)
- [x] AC4 da REQ — prova de disparo real ou impossibilidade declarada com motivo (ML-2A)
- [x] `make quality` verde (arquiteto, sem `~/.local/bin` no PATH)

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat Model e remedição (2 MLs, mesmo despacho do hades-tf)
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model for this roadmap
**Status:** ✅ Concluído
**Squad:** hades-tf
**Files affected:** `docs/seguranca/2026-10-08-wave0-credential-guard-windsurf-amazonq.md` (novo)
**Actions:**
1. Enumeration completeness — todos os sítios que emitem, migram ou auditam o credential guard por CLI (gerador de projeto, `update`, `update harness`/`buildHarnessTargetIDs`, `harnessCatalogTargetOrder`, `validate`, detectores globais, `check-*` scripts, `docs/cli-parity.md`). Fechar a lista por grep do literal `trackfw guard credential` e de `credential-guard`, não só pelos arquivos da REQ.
2. Threat model — quem esvazia esta Wave 0 sem quebrar regra escrita, e como.
3. Falsification targets in both directions — por superfície: o que quebra se o guard deixar de ser emitido, e o que quebra se for emitido num evento que não bloqueia (falsa sensação de proteção) ou que bloqueia tudo (deny-all).
4. Declared residual.
**Acceptance criteria:**
- [x] The four sections above answered with evidence, not a one-line assertion
- [x] No implementation line written for this ML

### ML-0B — Remedição dos eventos de hook (AC1 da REQ)
**Status:** ✅ Concluído
**Squad:** hades-tf
**Files affected:** mesmo documento do ML-0A, seção "Remedição"
**Actions:** por CLI (Windsurf, Amazon Q CLI), com URL da documentação oficial e data de acesso: quais eventos existem antes de ler arquivo, escrever arquivo e executar comando; que payload chega no stdin (comando? caminho? conteúdo escrito?); exit code que bloqueia vs só avisa; se há escopo global (arquivo de usuário) além do de projeto. Confrontar com o que o gerador emite hoje.
**Acceptance criteria:**
- [x] Tabela por CLI × evento com fonte e data; cada célula "bloqueia/avisa/não existe" tem citação
- [x] Recomendação por CLI: instalar (em qual evento e com qual nome de guard) ou manter fora (motivo medido)

      Auditoria (2026-10-08): 1º parecer reprovado — payload de escrita inventado (`content`) nos dois CLIs; ML corretivo
      remediu com payload real (Windsurf `edits[].new_string`, Amazon Q `fs_write` tag `command`). Veredito mantido.
      Achados novos, conferidos pelo arquiteto com binário desta árvore em projeto `mode: block`: JWT + `> /dev/null` no
      conteúdo de `Write`/`Edit` do Claude Code e `fs_write` do Amazon Q → RC 0 (isenção efêmera aplicada ao payload
      inteiro); `cat "arquivo"` (aspas) passa na 2ª camada; Windsurf `command_line` não é lido. Mesma causa → ML-1C.

**Gates da wave:**
```bash
test -s docs/seguranca/2026-10-08-wave0-credential-guard-windsurf-amazonq.md
grep -q "Veredito" docs/seguranca/2026-10-08-wave0-credential-guard-windsurf-amazonq.md
```

## Wave 1 — Decisão e implementação
> Dependencies: Wave 0 auditada. O detalhe dos MLs é fechado a partir do parecer da Wave 0.

### ML-1A — Adendo à ADR-2026-08-05 (AC2)
**Status:** ✅ Concluído
**Squad:** zeus-tf
**Files affected:** `docs/adr/ADR-2026-08-05-hook-de-guarda-contra-materializacao-de-credenciais-reais-por-subagentes.md`
**Acceptance criteria:**
- [x] Adendo datado revendo a premissa do Windsurf e avaliando o Amazon Q, decisão por CLI citando o ML-0B

### ML-1B — Gerador, update, harness e validate (AC3)
**Status:** ✅ Concluído
**Squad:** apolo-tf
**Files affected:** `internal/generators/agentfiles.go`, `internal/generators/update.go`, `internal/validator/validator_credential_guard.go`, `internal/validator/validator_guard_binary_probe.go` (se preciso), testes desses pacotes, `.windsurf/hooks.json`/`.amazonq/...` deste repo se existirem, `docs/cli-parity.md`. **Não toca `internal/guard/`** (ML-1C, paralelo).
**Eventos:** Windsurf `pre_run_command` + `pre_write_code`; Amazon Q `preToolUse` matchers `execute_bash` + `fs_write`. Nunca `pre_read_code`/`fs_read`. Harness `windsurf-credential-guard` (`~/.codeium/windsurf/hooks.json`); Amazon Q só projeto.
**Acceptance criteria:**
- [x] Teste: `init`/`update` emitem a linha D11 revista do credential guard no evento decidido, por CLI decidido
- [x] Teste: `validate` acusa o arquivo sem o credential guard (antes silenciado) e não acusa o arquivo correto
- [x] Falsificação nas duas direções, com a frase de reconciliação por teste novo
- [x] Configs deste repo e `docs/cli-parity.md` atualizados
      Auditoria (2026-10-08): 18 testes novos conferidos por nome; artefato real medido com o binário (init
      `--ai-tools windsurf,amazonq`, HOME isolado): guard em pre_run_command/pre_write_code e execute_bash/fs_write, ausente
      em pre_read_code; tirar pre_write_code → validate acusa; `update` repõe → 0 achados. Reprovado 1x: o validate não
      olhava o global do Windsurf (violação que o `update` nunca corrige) → ML-1D.

### ML-1C — O guard lê o payload de cada CLI (AC5)
**Status:** ✅ Concluído
**Squad:** apolo-tf (paralelo ao ML-1B: arquivos disjuntos)
**Por que o escopo original não previa:** achados da Wave 0 (ver ML-0B). A REQ dizia "não muda o guard"; a medição
mostrou que o guard não entende o payload dos CLIs que esta REQ cobre, e que a mesma causa já abria o Claude Code.
**Files affected:** `internal/guard/credential.go` e testes em `internal/guard/`. Nada fora de `internal/guard/`.
**Acceptance criteria:**
- [x] 2ª camada extrai o comando por JSON parse de `tool_input.command` e `tool_info.command_line` (e `command` no topo, se algum CLI usar); `cat "arquivo"` e `cat 'arquivo'` detectados
- [x] Isenção efêmera (`> /dev/null` etc.) avaliada só sobre o comando de shell extraído; payload de escrita (Write/Edit do Claude, `pre_write_code`, `fs_write`) com JWT + `> /dev/null` → 2 em `block`, nos escopos projeto e global
- [x] `echo <JWT> > /dev/null` num comando de shell continua isento (direção oposta)
- [x] Testes com payload real de Claude Code, Codex, Windsurf e Amazon Q; falsificação nas duas direções; frase de reconciliação por teste novo

### ML-1D — Corretivo da auditoria da Wave 1
**Status:** ✅ Concluído
**Squad:** apolo-tf
**Por que:** (1) o gerador pulava o guard de projeto do Windsurf com o global instalado, mas o validate exigia no
projeto; e "global instalado" contava só pre_run_command, deixando pre_write_code descoberto num global parcial;
(2) faltou o teste com payload do Codex pedido no AC5.
- [x] Validate respeita o global completo (pre_run_command E pre_write_code), mesma definição do gerador; 3 testes + falsificação
- [x] 3 testes de payload do Codex (cat, cat com aspas, echo > /dev/null isento)
- [x] `make quality` (arquiteto, máquina ociosa, sem `~/.local/bin`): EXIT=0, falsify 347 OK / 0 FAIL

## Wave 2 — Prova e red-team
> Dependencies: Wave 1 auditada.

### ML-2A — Prova de disparo real (AC4)
**Status:** ✅ Concluído
**Squad:** zeus-tf
**Acceptance criteria:**
- [x] Disparo real medido em ao menos um CLI, ou impossibilidade declarada (sem conta de Windsurf/Amazon Q, decisão do usuário)
      Impossibilidade declarada (2026-10-08): sem conta de Windsurf nem de Amazon Q (KG: "os demais não tenho assinatura").
      O que foi medido no lugar: o binário com o payload documentado de cada CLI (Wave 0 corrigida, ML-2D) e o artefato
      gerado por `init`/`update`/`validate` num projeto real (ML-1B). Não medido: o CLI real invocar o hook.

### ML-2B — Red-team do diff
**Status:** ✅ Concluído
**Squad:** hades-tf
**Acceptance criteria:**
- [x] Parecer sobre o diff da Wave 1 contra o threat model do ML-0A; achados corrigidos nesta REQ
      Veredito do hades-tf (`docs/seguranca/2026-10-08-red-team-credential-guard-windsurf-amazonq.md`): **não libera**.
      F1 (alto) validate aceita `echo` do marcador global do Windsurf; F2 (médio, anterior à REQ) isenção efêmera ignora
      sink secundário (`| tee arq > /dev/null`, `dd of=`, interpretador inline); F3 (alto, regressão) BOM faz o parse JSON
      falhar e desliga a 2ª camada — abaixo da main; F4 (baixo, regressão) `command` fora dos 4 caminhos fixos deixou de ser
      lido. Mesma causa (o guard e o validate lendo o payload/arquivo pelo que ele é) → ML-2C/2D aqui.

### ML-2C — Corretivo F1, F3, F4
**Status:** ✅ Concluído
**Squad:** apolo-tf (mesmo despacho do ML-2D, em sequência: ambos tocam `internal/guard/credential.go`)
- [x] F3: parse do payload sem BOM; teste com BOM em cada caso do ML-1C (projeto e global)
- [x] F4: 2ª camada e varredura de redirecionamento sobre TODOS os valores string de `command`/`command_line` em qualquer profundidade (exceto o enum do `fs_write`); `{"params":{"command":"cat arq"}}` → 2
- [x] F1: "global instalado" (validate) e presença no projeto exigem a linha de hook esperada (igualdade, como o gerador), não substring; `echo trackfw guard credential --global` não satisfaz
- [x] Falsificação de cada um, frase de reconciliação por teste

### ML-2D — Corretivo F2: isenção efêmera só para comando simples
**Status:** ✅ Concluído
**Squad:** apolo-tf
**Decisão do arquiteto:** a isenção existe porque `> /dev/null` não materializa nada. Ela só vale quando nada mais pode
materializar: um único comando de shell, simples (sem `|`, `;`, `&&`, `||`, `&`, `$(`, crase, `<(`/`>(`, quebra de linha),
argv[0] ∈ {echo, printf}, todos os redirecionamentos efêmeros, e o JWT dentro desse comando. Qualquer outra forma bloqueia
em `block` (fail-closed); em `warn` só avisa. Registrado no adendo da ADR-2026-08-05.
- [x] `echo <JWT> > /dev/null` continua isento; `| tee arq`, `dd of=`, `python3 -c ... > /dev/null`, `& writer` → 2
- [x] Falsificação nas duas direções
- [x] `make quality` (arquiteto)
      Auditoria (2026-10-08): 18 testes novos conferidos por nome; binário medido em 32 combinações (8 comandos × projeto/
      global × com/sem BOM: echo/printf para /dev/null → 0; tee, dd, python -c, &, ;, cat de arquivo → 2), sem divergência.
      F1 do lado do projeto já era coberto pela checagem de forma (`echo trackfw guard credential` → ✗, medido). A isenção
      ficou mais estrita que o pedido (só `/dev/null`, não `$(mktemp)`) — aceito, fail-closed. `make quality` EXIT=0,
      falsify 347 OK / 0 FAIL.

### ML-2E — Windows: payload inválido e caminho com barra invertida
**Status:** ✅ Concluído
**Squad:** apolo-tf
**Por que o escopo original não previa:** o `windows-full-suites` do PR #543 reprovou 13 testes do guard, incluindo 2
anteriores ao PR (`TestRunCredential_SecondLayerCatArg`, `TestRunCredential_SecondLayerRedirectFile`) que passavam na
main. Hipótese a medir: o teste monta o JSON concatenando `C:\Users\...` sem escapar → JSON inválido; o ML-1C trocou o
regex sobre o texto bruto (que tolerava isso) por JSON parse sem fallback. E, com JSON válido, o caminho decodificado
`C:\Users\...` pode perder as barras no tokenizador de shell da 2ª camada — o regex antigo acertava por acaso.
- [x] Causa medida no Windows (CI ou VM), escrita no relatório
- [x] Payload que não é JSON válido: mesma extração da main (sem regressão)
- [x] Payload JSON válido com caminho Windows (`C:\\Users\\...` escapado): 2ª camada acha o arquivo
- [x] Testes montam payload com `json.Marshal`; os 13 passam no `windows-full-suites`
      Causa medida na VM (Windows ARM64, binários de teste da main e da branch): H1 — os testes concatenavam `C:\Users\...`
      sem escape → JSON inválido → o ML-1C não tinha fallback e desligava a 2ª camada. H2 (tokenizador perde as barras)
      refutada: com JSON válido o caminho decodificado funciona. Correção: payload inválido volta à extração da main;
      testes com `json.Marshal`; VM 168 PASS / 0 FAIL. Auditoria: 2 nomes da tabela de reconciliação não existem
      (`TestRunCredential_BOMPrefix`, `_QuotedPath` — os testes descritos existem com outros nomes). `make quality` EXIT=0.

## Wave 3 — Reabertura pela issue #544 (forma de caminho Windows na 2ª camada)
> Dependencies: PR #543 mergeado. Reaberto em 2026-10-08.
**Por que o escopo original não previa:** o ML-1C criou `credResolveArg` resolvendo argumento com `filepath.IsAbs`, que
no Windows é false para `/c/Users/...` (forma natural do Git Bash, que o Claude Code usa no Windows) → caminho colado
no cwd, arquivo não achado, sem aviso. E o ML-2E registrou como limitação que `credRedirectRe` corta `C:\...` no `:`.
Mesma causa da REQ (o guard não lê o payload pelo que o CLI escreve) → mesma REQ. Relatado por @lourivalgarciajunior.

### ML-3A — Threat model da reabertura
**Status:** ✅ Concluído
**Squad:** hades-tf
- [x] Enumeração de todo sítio da 2ª camada que interpreta caminho (argumento, redirecionamento, glob, cwd do payload, `tool_info.cwd`) e de quais formas de caminho cada CLI escreve no Windows (Git Bash `/c/`, MSYS `/cygdrive/c/`?, `C:\`, `C:/`, UNC `\\server\share`, `~`)
- [x] Alvos de falsificação nas duas direções (forma não detectada; forma POSIX legítima `/c/...` num Linux real, onde `/c` é diretório de verdade)
- [x] Medição na VM com o binário de `main`, lendo stderr, em modo `warn` e `block`
      Parecer (`docs/seguranca/2026-10-08-wave3-credential-guard-caminho-windows.md`), medido na VM com o binário da main:
      BUG-1 `/c/`, `/C/`, `/cygdrive/c/` não detectados no argumento (sem cwd: `os.Stat("/c/...")` vira `C:\c\...`; com cwd:
      `IsAbs` false → join quebrado); BUG-2 `credRedirectRe` exclui `:` → `> C:\...` vira o arquivo `C` (prova: arquivo `C`
      no cwd é varrido). `C:\`, `C:/`, UNC e relativo detectam. macOS: `/c/...` absoluto, sem regressão possível se a
      tradução for só no Windows. Decisão do arquiteto: tradução só com GOOS windows e tool_name ≠ PowerShell (o PowerShell
      lê `/c/x` como `C:\c\x`); Codex manda "Bash" mesmo no PowerShell — traduzir ali é fail-closed (no pior caso um falso
      positivo). Auditoria: o agente escreveu fora da pasta autorizada da VM (tf-ml3a4..6, /tmp) e declarou; conferido limpo.

### ML-3B — Correção e testes
**Status:** ✅ Concluído
**Squad:** apolo-tf
- [x] Toda forma listada no ML-3A detectada no Windows (argumento e redirecionamento), projeto e global
- [x] POSIX sem regressão (`/c/...` continua absoluto)
- [x] Testes por forma de caminho; verdes no `windows-full-suites` e na VM; falsificação; frase de reconciliação
- [x] `make quality` (arquiteto)
      Auditoria (2026-10-08): o relatório citou 12 de 13 nomes de teste inexistentes; os testes reais
      (`TestCredNormalizeWindowsPath`, `TestCredRedirectRe_*`, `TestRunCredential_*_Windows`) rodados pelo arquiteto na VM:
      todos PASS, suíte do guard PASS. Falsificação na VM pelo arquiteto: sem a normalização em `credResolveArg`, reprovam
      SubcaseA, SubcaseB, Uppercase, Cygdrive e RedirectGitBash. `make quality` EXIT=0, 347 OK / 0 FAIL.

### ML-3D — Mesmo padrão no `credentialGuardDetectionCore` do scaffold
**Status:** ✅ Concluído
**Squad:** apolo-tf
**Premissa do arquiteto errada, corrigida pela medição:** o script gerado é um wrapper que chama `trackfw guard credential`;
o `credentialGuardDetectionCore` é código morto desde o ML-2A. Corrigido o padrão ali (alinhamento, sem efeito em runtime);
fixtures congeladas em `testdata/guard-sh-reference/` mantidas. Teste novo
`TestCredentialGuardScript_GeneratedScript_WindowsPathRedirectDetectsJWTInFile` (executa o script gerado; prova o ML-3B
através do wrapper). Remoção do código morto: fora do escopo, registrada.

### ML-3C — Red-team
**Status:** 🔄 Em andamento
**Squad:** hades-tf
- [ ] Parecer sobre o diff da Wave 3

