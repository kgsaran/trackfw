---
date: 2026-08-27
roadmap: "ROADMAP-2026-08-27-doctor-cobre-artefatos-de-scaffold-por-comparacao-com-o-template.md"
ml: ML-0A
agente: hades-tf
---

# Modelo de Ameaça — Cobertura de Scaffold pelo `doctor`

> ML-0A · Wave 0 · 2026-08-27

---

## Completude de enumeração

**Pergunta:** o inventário listado na REQ está completo? O que está faltando ou incorreto?

### Metodologia

Enumerei os artefatos lendo diretamente o código-fonte dos três geradores:

- `internal/generators/scaffold.go` (Go) — `Scaffold()`, linha de chamadas
- `npm/src/generators/init.js` (Node.js) — `scaffold()`
- `pypi/trackfw/generators/init_gen.py` (Python) — `scaffold()`

O gate de paridade `scripts/check-slash-parity.sh` foi executado e retornou verde (9 commands × 3 runtimes). Os três geradores são byte-idênticos nos slash commands em HEAD.

### Artefatos que o Scaffold escreve — por superfície

| # | Artefato | Condicional? | Já coberto por `validate`? |
|---|---------- |-------------|---------------------------|
| 1 | `.claude/commands/trackfw/adr.md` | não | não |
| 2 | `.claude/commands/trackfw/req.md` | não | não |
| 3 | `.claude/commands/trackfw/validate.md` | não | não |
| 4 | `.claude/commands/trackfw/status.md` | não | não |
| 5 | `.claude/commands/trackfw/move.md` | não | não |
| 6 | `.claude/commands/trackfw/roadmap.md` | não | não |
| 7 | `.claude/commands/trackfw/barrier.md` | não | não |
| 8 | `.claude/commands/trackfw/architect.md` | não | não |
| 9 | `.claude/commands/trackfw/implement.md` | não | não |
| 10 | `scripts/trackfw-attention-signal.sh` | não | não |
| 11 | `scripts/trackfw-attention-cleanup.sh` | não | não |
| 12 | `scripts/trackfw-validate.sh` | conteúdo varia por cfg | não (cobertura só de hash) |
| 13 | `scripts/trackfw-credential-guard.sh` | não | **SIM** (`credential_guard_script_integrity`, severity=warning) |
| 14 | `scripts/trackfw-git-branch-guard.sh` | não | **SIM** (`git_branch_guard_script_integrity`, severity=warning) |
| 15 | `.github/workflows/trackfw-gate.yml` | `ci: github-actions` | não |
| 16 | `.gitlab-ci-trackfw.yml` | `ci: gitlab-ci` | não |
| 17 | Hook files (husky/lefthook) | `hooks: husky\|lefthook` | não |

**Itens na REQ não listados:** artefatos 16 e 17. A REQ lista `.github/workflows/trackfw-gate.yml` mas omite `.gitlab-ci-trackfw.yml` e os hook files condicionais (husky/lefthook). Eles existem no código-fonte dos três CLIs.

**Artefatos fora de escopo por design (REQ, negative scope):**
- `CLAUDE.md`, `AGENTS.md`, `GEMINI.md`, `.windsurfrules`, `.amazonq/developer/guidelines.md`, `.cursor/rules/trackfw.mdc` — editáveis pelo usuário; cobri-los é falso-positivo garantido.
- `vault/notes/index.md` — escrito por `init` e nunca por `update`; conteúdo de autoria do projeto.
- `trackfw.yaml` — escrito por `init` e nunca por `update`; configuração de autoria do projeto.

**Terceiro escritor — `discover --init`:** `InstallGates` (`internal/discover/discover.go:49`) escreve um **subconjunto** de scaffold: validate-script, CI workflow, attention scripts, credential-guard, git-branch-guard. **Não escreve slash commands.** Um projeto inicializado exclusivamente via `trackfw discover --init` legitimamente não tem `.claude/commands/trackfw/*.md`. O doctor deve conhecer esse estado.

**Conclusão:** o inventário da REQ está parcialmente correto. Itens 1–15 são confirmados. Itens 16 e 17 existem no código mas devem ser **incluídos com condicional explícita** no escopo do doctor (só reportar ausência se `trackfw.yaml` os declara). Hook files que dependem do framework (husky/lefthook) são de responsabilidade do integrador de hooks, não do doctor de scaffold — devem ser **excluídos do escopo** desta REQ.

---

## Modelo de ameaça

**Adversário:** o implementador apressado e o arquiteto otimista. Não um atacante externo.

### Como este Wave 0 é esvaziado sem violar nenhuma regra escrita

**Ameaça 1 — Inventário de template derivado de path, não de cfg.**
O implementador verifica se um arquivo existe no namespace (`scripts/trackfw-*.sh`) e compara byte a byte com um template fixo embutido. Não renderiza o template a partir do `trackfw.yaml` do projeto.

Consequência: `scripts/trackfw-validate.sh` tem conteúdo que varia com `cfg.Backend` e `cfg.Frontend` (função `buildValidateScript` em `scaffold.go`). Um projeto com `backend: go` tem uma linha adicional `go build ./...` no script. Um projeto sem `backend:` não tem. Se o doctor usa o template do binário renderizado para `backend=""` contra um projeto com `backend: go`, reporta divergência onde não há — violando AC4.

Evidência: este repo tem `trackfw.yaml` sem chave `backend:`. O script em disco é o template base. `./bin/trackfw update --dry-run` retornou `validate-script: skipped` — zero divergência. Mas se um projeto com `backend: java` rodar contra este binário sem renderização por cfg, o resultado muda.

**Ameaça 2 — Escopo de ausência não gateado por `trackfw.yaml`.**
O implementador reporta artefatos condicionais como ausentes quando não estão configurados.

Consequência: projeto com `ci: none` recebe alerta de `.github/workflows/trackfw-gate.yml` ausente. Projeto sem husky nem lefthook recebe alerta de hook files ausentes. Isso é ruído puro — AC4 reprovado na primeira execução em qualquer projeto que não use GitHub Actions.

**Ameaça 3 — Ignorar o terceiro escritor (`discover --init`).**
O implementador implementa a cobertura assumindo que todo projeto trackfw foi inicializado via `trackfw init`. Projetos criados via `trackfw discover --init` legitimamente não têm slash commands. O doctor reporta 9 arquivos ausentes.

Consequência: AC2 ativado incorretamente para slash commands em projetos `discover --init`. AC4 violado.

**Ameaça 4 — Não distinguir binário antigo de projeto desatualizado.**
O implementador emite a mensagem "seu projeto está defasado" sem verificar a direção. Se o projeto tem um arquivo mais novo (gerado por uma versão futura) e o binário instalado é antigo, a mensagem culpa o projeto quando a culpa é do binário.

Consequência: AC5 não atendido. O usuário atualiza arquivos que não deveria e perde customizações.

**Ameaça 5 — Reuso ingênuo do `ClassifyDoctor` atual.**
`ClassifyDoctor` (`internal/integrations/doctor.go:68-100`) não tem case para `!Registered && State == StateModified`. Um artefato de scaffold que existe em disco mas difere do template cai no `default` implícito e não gera nenhum finding (documentado na nota `doctor-classifydoctor-silences-tampering-when-manifest-entry-removed-2026-08-19`). Se o implementador adiciona a comparação de template sem adicionar o case correspondente, o doctor fica silencioso para os artefatos mais críticos — os que foram modificados.

Consequência: FN total. AC1 não atendido.

---

## Alvos de falsificação nas duas direções

Para cada superfície: onde a sabotagem entra, qual gate deveria pegar, em qual direção.

### Superfície A — Slash commands (9 arquivos, `.claude/commands/trackfw/*.md`)

**Falso negativo (FN):** artefato divergente deixa de ser reportado.
- Sabotagem: stub de doctor não chama o comparador para o namespace `.claude/commands/trackfw/`.
- Gate: AC8(a) — teste injeta conteúdo divergente em `validate.md`, espera finding `divergent`.
- Como detectar: `grep -r "commands/trackfw" internal/integrations/doctor.go` deve encontrar a cobertura; se não encontrar, o case foi omitido.

**Falso positivo (FP):** artefato íntegro reportado como divergente.
- Sabotagem: comparador usa template gerado para runtime incorreto (ex.: conteúdo Node.js contra binário Go, antes do fix ML-5F). Hoje o gate de paridade `check-slash-parity.sh` passa — os três runtimes são byte-idênticos. Se a paridade quebrar no futuro e o doctor comparar com o template do Go enquanto o projeto foi inicializado com Python, há divergência artificial.
- Gate: AC4 — `update --dry-run` com `claude-commands: skipped` em projeto recém-atualizado.
- Residual: se os três runtimes voltarem a divergir (sem ML-5F equivalente), o doctor acusará projetos criados por um runtime contra o template do outro. Isso está fora do escopo desta REQ mas deve ser mencionado na documentação do AC7.

### Superfície B — `scripts/trackfw-validate.sh` (cfg-dependente)

**FN:** arquivo divergente não é reportado.
- Sabotagem: doctor compara apenas existência, não conteúdo.
- Gate: AC8(a) — teste injeta linha extra no script, espera finding.

**FP:** arquivo íntegro reportado como divergente.
- Sabotagem: template renderizado sem ler `trackfw.yaml` do projeto — usa `cfg.Backend=""` para um projeto com `backend: go`, gerando conteúdo diferente.
- Gate: AC4 — teste cria projeto com `backend: go`, roda doctor, espera `no mismatches`.
- Este é o FP mais provável e o que mais ameaça AC4.

### Superfície C — `.github/workflows/trackfw-gate.yml` (condicional `ci: github-actions`)

**FN:** arquivo divergente não é reportado.
- Sabotagem: doctor não inclui CI workflow no conjunto de comparação.
- Gate: AC8(a) — teste corrompe o workflow, espera finding.

**FP:** arquivo ausente reportado como missing em projeto com `ci: none`.
- Sabotagem: doctor não lê `trackfw.yaml` para condicionar a cobertura.
- Gate: AC4 — teste cria projeto com `ci: none`, espera `no mismatches`.

### Superfície D — Guard scripts (credential-guard, git-branch-guard)

**FN:** arquivo divergente não é reportado pelo doctor (note: já coberto por `validate` com severity=warning, mas a cobertura do doctor é complementar).
- Sabotagem: doctor assume que `validate` cobre e pula a superfície.
- Gate: AC8(a) + verificar se a cobertura é aditiva (não exclusiva).

**FP:** guard script legítimo modificado pela equipe reportado como divergente.
- Sabotagem: nenhuma — o ADR aceita explicitamente este atrito; customização de guards não é suportada.
- Residual declarado: este FP é aceito pelo ADR. Deve ser documentado na saída do doctor como "hand-modified — run `trackfw update` to restore".

### Superfície E — Projetos criados por `discover --init` (subconjunto sem slash commands)

**FN:** slash commands ausentes não são reportados (correto para este tipo de projeto).
- Ameaça: o doctor reporta ausência quando não deveria — AC4 violado.
- Gate: AC4 — teste cria projeto via `discover --init` sem slash commands, espera `no mismatches` para essa superfície.

**FP:** doctor reporta ausência de slash commands em projeto `discover --init`.
- Como distinguir: o doctor precisa de um sinal de que o projeto foi inicializado via `discover --init` — o candidato mais direto é a presença de `trackfw.yaml` com um campo de origem, ou a ausência de `.claude/` combinada com presença de `scripts/trackfw-validate.sh`. A decisão de mecanismo é da implementação (ML-1A), não do Wave 0; o Wave 0 nomeia o problema.

### Superfície F — Binário antigo em projeto novo (AC5)

**FP:** binário desatualizado acusa o projeto.
- Sabotagem: mensagem de finding diz "projeto defasado" sem qualificar a direção.
- Gate: AC5 — não há gate automatizado para wording; o gate é revisão de código. A implementação deve usar linguagem neutra: "este artefato difere do template que o binário instalado (vX.Y.Z) geraria; se seu projeto foi inicializado com uma versão mais nova, atualize o binário".
- Ausência de stamp de versão nos artefatos confirma que a direção não pode ser inferida por conteúdo — a mensagem deve ser explicitamente agnóstica.

---

## Residual declarado

O que este design aceita não cobrir, dito claramente.

**1. Customizações deliberadas de artefatos de scaffold.**
Quem editou um slash command à mão verá `hand-modified`. Isso é informação correta — o `update` já sobrescreve essas edições — mas é atrito novo para quem não sabia que customizações não são suportadas. Aceito; documentado no ADR. Remédio nomeado: `trackfw update`.

**2. Projetos criados por `discover --init` sem slash commands.**
Doctor não pode, sem um sinal externo, distinguir "slash commands ausentes porque nunca foram instalados" de "slash commands ausentes porque foram deletados". O design aceita que a cobertura de AC2 (artefato ausente) para slash commands requer que o projeto tenha passado por `trackfw init` ou `trackfw update`. Projetos `discover --init` são excluídos da cobertura de AC2 para slash commands — ou o implementador deve definir o critério de elegibilidade explicitamente em ML-1A.

**3. Artefatos condicionais não configurados.**
`.gitlab-ci-trackfw.yml` e hook files (husky/lefthook) ficam fora do escopo de doctor quando `trackfw.yaml` não os declara. O doctor não os reporta como ausentes. Aceito.

**4. Drift de template entre runtimes futuro.**
Se Go, Node.js e Python voltarem a divergir nos templates de slash commands (como documentado na nota vault `slash-commands-cross-runtime-content-drift-2026-07-29`, corrigido em ML-5F), o doctor acusará projetos criados por um runtime contra o template do outro. O gate de paridade `check-slash-parity.sh` é a barreira — se ele falhar, o FP volta. Este risco é aceito como dependência do gate de paridade existente.

**5. Direção de defasagem (AC5) não inferível por conteúdo.**
Nenhum artefato de scaffold carrega stamp de versão. O doctor não pode determinar se o projeto é mais novo ou mais velho que o binário. A mensagem deve ser neutra quanto à culpa. Uma comparação que resulte em divergência pode ser culpa do projeto ou do binário — o usuário deve verificar. Aceito; não há mecanismo no design atual para resolver sem adicionar stamps.

**6. `vault/notes/index.md` e `trackfw.yaml` fora de escopo.**
Escritos por `init` e nunca por `update`. Conteúdo de autoria do projeto. Acusá-los seria FP garantido. Aceito.
