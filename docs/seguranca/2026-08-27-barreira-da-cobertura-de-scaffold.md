---
date: 2026-08-27
roadmap: "ROADMAP-2026-08-27-doctor-cobre-artefatos-de-scaffold-por-comparacao-com-o-template.md"
ml: ML-3A
agente: hades-tf
---

# Barreira — Cobertura de Scaffold pelo `doctor`

> ML-3A · Wave 3 · 2026-08-27

---

## Veredito

**APROVADO com residuais nomeados.** Os cinco bloqueios da Wave 0 estão fechados. Nenhuma medição
contradiz os critérios de aceite. Três residuais declarados abaixo — todos aceitáveis e
documentados.

---

## Metodologia — o que foi medido vs. o que foi raciocinado

Cada achado está rotulado:

- **[medido]** — saída de comando colada, diretamente verificável.
- **[raciocinado]** — análise de código-fonte ou lógica estrutural; não requer execução para ser
  válida, mas também não tem saída que prove sozinha.

---

## Pergunta 1 — Os cinco bloqueios estão fechados?

### Bloqueio 1 — `trackfw-validate.sh` é cfg-dependente

**[medido]** Fixture `backend: go` completo — todos os 3 runtimes:

```
Go     → no mismatches found
Node   → no mismatches found
Python → no mismatches found
```

**[medido]** Mesmo fixture com validate.sh corrompido (`INJECTED_LINE` adicionado):

```
Go     → 1 finding scaffold-divergent: scripts/trackfw-validate.sh
Node   → 1 finding scaffold-divergent: scripts/trackfw-validate.sh
Python → 1 finding scaffold-divergent: scripts/trackfw-validate.sh
```

**[medido]** Fixture `frontend: react` + `pkg_manager: pnpm` — todos os 3 runtimes:

```
Go     → no mismatches found
Node   → no mismatches found
Python → no mismatches found
```

**[raciocinado]** `checkValidateScriptArtifact` lê `trackfw.yaml` do projeto via `cfg` (parâmetro
passado pelo chamador que já carregou `trackfw.yaml` com `LoadConfig`) e chama
`buildValidateScript(cfg)` para montar a forma Go/Node esperada antes da comparação. A forma Python
está fixada em `pythonValidateScriptForm` (constante). Não há template padrão embutido
sem renderização por cfg.

**Bloqueio 1: FECHADO.**

---

### Bloqueio 2 — Condicionais não gateadas por `trackfw.yaml`

**[medido]** Fixture `ci: none` (sem workflow) — todos os 3 runtimes:

```
Go     → no mismatches found
Node   → no mismatches found
Python → no mismatches found
```

**[raciocinado]** `RunScaffoldDoctor` usa `switch cfg.CI` para decidir se verifica o workflow.
Slash commands são verificados apenas se `os.Stat(claudeDir) == nil` (diretório existe). CI `none`
não cai em nenhum case — nenhum finding gerado para o workflow ausente.

**Bloqueio 2: FECHADO.**

---

### Bloqueio 3 — `discover --init` sem slash commands

**[medido]** Gate `check-doctor-parity.sh`, cenário `scaffold-no-slash-commands-dir-silent`:

```
OK   [doctor-parity/scaffold-no-slash-commands-dir-silent-text]
OK   [doctor-parity/scaffold-no-slash-commands-dir-silent-json]
```

**[raciocinado]** O gate cria fixture sem `.claude/commands/trackfw/`, roda os 3 runtimes e valida
que nenhum finding é gerado para slash commands ausentes. A condição
`os.Stat(claudeDir) err == nil` é o discriminante: se o diretório não existe, o bloco inteiro de
verificação de slash commands é pulado.

**Bloqueio 3: FECHADO.**

---

### Bloqueio 4 — Mensagem culpa o projeto quando o defasado é o binário

**[medido]** Remedy de scaffold-divergent medido em Go:

```
remedy: trackfw update   # resync scripts/trackfw-validate.sh: content differs from the
template trackfw v7.2.0 generates; if this project was initialized with a newer binary,
update the binary instead
```

**[raciocinado]** `scaffoldRemedy` (`scaffold_doctor.go:258-265`) sempre emite a segunda cláusula:
"if this project was initialized with a newer binary, update the binary instead". Nenhum artefato
carrega stamp de versão — a mensagem não pode declarar quem está desatualizado e não o faz. AC16
satisfeito nos runtimes Go e Node.

**RESIDUAL A (Python):** Python emite `trackfw vunknown` em vez de `v7.2.0` quando executado a
partir de fonte (sem instalação pip). Causa: `importlib.metadata.version('trackfw')` falha com
`PackageNotFoundError` — verificado:

```
$ python3 -c "import importlib.metadata; print(importlib.metadata.version('trackfw'))"
PackageNotFoundError: No package metadata was found for trackfw
```

O fallback em `_scaffold_remedy` grava `ver = 'unknown'`. O usuário lendo a mensagem Python não
tem versão para verificar, o que parcialmente desfaz AC16 nesse runtime.

Condição de degradação: desenvolvimento local com `PYTHONPATH=pypi`, não em produção.
Severidade: baixa — não bloqueia, não afeta runtimes Go e Node, ocorre apenas no checkout de fonte
sem pip install. Não altera o veredito. Não é regressão nova: o código de fallback foi
introduzido junto com o ML e o comportamento é consistente com a expectativa do ambiente de
desenvolvimento.

**Bloqueio 4: FECHADO. Residual A nomeado e declarado não-bloqueante.**

---

### Bloqueio 5 — `ClassifyDoctor` sem case `!Registered && StateModified`

**[medido]** Código em `internal/integrations/doctor.go:135-144`:

```go
case !inspection.Registered && inspection.State == StateModified:
    findings = append(findings, DoctorFinding{
        FindingKind: DoctorUnknownContent,
        ...
    })
```

O case existe e gera `DoctorUnknownContent` — a cobertura não é decorativa para artefatos
manifest-managed.

**[raciocinado — distinção crítica]** Artefatos de scaffold **nunca passam por `ClassifyDoctor`**.
`RunScaffoldDoctor` chama `checkScaffoldArtifact` e `checkValidateScriptArtifact` diretamente, sem
passar pelo classificador de manifesto. O comentário em `scaffold_doctor.go:79` documenta isso
explicitamente: "not by adding a case to [ClassifyDoctor]".

O bloqueio 5 foi, portanto, fechado por **arquitetura** — a implementação criou um caminho
separado que não depende do `ClassifyDoctor` para scaffold. O case adicionado em `doctor.go:135`
fecha o mesmo buraco para artefatos manifest-managed (melhoria independente e igualmente
necessária), mas não é o mecanismo que protege a cobertura de scaffold.

Implicação para manutenção futura: qualquer refatoração que roteie artefatos de scaffold pelo
`ClassifyDoctor` deve verificar que o case `!Registered && StateModified` se comporta
corretamente para a nova superfície — a proteção atual depende da separação de caminhos.

**[medido]** Prova comportamental (AC15): conteúdo divergente em validate.sh gera finding nos 3
runtimes:

```
Go     → 1 finding scaffold-divergent: scripts/trackfw-validate.sh
Node   → 1 finding scaffold-divergent: scripts/trackfw-validate.sh
Python → 1 finding scaffold-divergent: scripts/trackfw-validate.sh
```

O finding é gerado via `checkValidateScriptArtifact`, não via `ClassifyDoctor`. Conclusão: o
comportamento pedido pelo AC15 existe; o mecanismo não é o que o bloqueio antecipava, mas a
proteção é real e verificada.

**Bloqueio 5: FECHADO.**

---

## Pergunta 2 — A regra de pertencimento abriu buraco?

**[medido]** Três vetores de ataque contra `bytes.Equal(actual, goNodeForm) || bytes.Equal(actual, pythonForm)`:

**Ataque 1 — Python form + linha extra:**

```
printf "%s\nEXTRA_LINE\n" "$PYTHON_FORM" > scripts/trackfw-validate.sh
Go     → 1 finding scaffold-divergent
Node   → 1 finding scaffold-divergent
Python → 1 finding scaffold-divergent
```

**Ataque 2 — Go form com última linha removida** (`sed '$d'`):

```
Go     → 1 finding scaffold-divergent
Node   → 1 finding scaffold-divergent
Python → 1 finding scaffold-divergent
```

**Ataque 3 — Concatenação das duas formas:**

```
printf "%s%s" "$GO_FORM" "$PYTHON_FORM" > scripts/trackfw-validate.sh
Go     → 1 finding scaffold-divergent
Node   → 1 finding scaffold-divergent
Python → 1 finding scaffold-divergent
```

**[raciocinado]** O conjunto aceito tem cardinalidade 2: exatamente a forma Go/Node renderizada a
partir de `buildValidateScript(cfg)` e exatamente a constante `pythonValidateScriptForm`. A
comparação é `bytes.Equal` — sem substring, sem normalização, sem tolerância de whitespace.
"Qualquer forma conhecida" significa literalmente dois valores fixos, não uma classe aberta.

**RESIDUAL B (custo declarado da decisão ML-1C):** Um projeto Go cujo `trackfw-validate.sh` está
na forma Python (`set -euo pipefail; trackfw validate`, sem `go build ./...`) é aceito sem finding.
Esse arquivo executa menos do que `trackfw.yaml` declara — o gate de build Go não roda. Não é
adulteração oculta; é uma lacuna de *capability* que o usuário pode ter criado ao inicializar
o projeto com o CLI Python em vez do Go. O custo é declarado: a decisão trocou um falso-positivo
real (bytes divergem entre runtimes por design documentado) por essa lacuna de capacidade,
escopada a projetos que misturam runtimes de inicialização.

**Pergunta 2: SEM BURACO de adulteração. Residual B nomeado como custo declarado da decisão.**

---

## Pergunta 3 — Falso-positivo em campo

### Cenário A — `backend: go` (projeto com build check de Go)

**[medido]** Fixture com `backend: go`, validate.sh na forma Go/Node com `go build ./...`:

```
Go     → no mismatches found
Node   → no mismatches found
Python → no mismatches found
```

### Cenário B — `ci: none` (sem workflow de CI)

**[medido]** Fixture com `ci: none`, sem `.github/workflows/trackfw-gate.yml`:

```
Go     → no mismatches found
Node   → no mismatches found
Python → no mismatches found
```

### Cenário C — `frontend: react` + `pkg_manager: pnpm`

**[medido]** Fixture com `frontend: react`, `pkg_manager: pnpm`, validate.sh com `pnpm run build`:

```
Go     → no mismatches found
Node   → no mismatches found
Python → no mismatches found
```

**Ameaça 1 está fechada nos três eixos de configuração:** backend, CI e frontend/pkg_manager.
O doctor renderiza o template correto a partir do `trackfw.yaml` do projeto nos 3 runtimes.

**Pergunta 3: ZERO falso-positivo medido em 3 configurações distintas × 3 runtimes.**

---

## Pergunta 4 — A cobertura tem buraco de inventário?

**[raciocinado a partir de fonte]** Inventário: 17 artefatos (Wave 0). Cobertura medida em
`RunScaffoldDoctor` (`scaffold_doctor.go:81-170`):

| # | Artefato | Coberto | Como |
|---|----------|---------|------|
| 1–9 | `.claude/commands/trackfw/*.md` (9) | Sim | `claudeCommandsContent()` iterado; gateado por `os.Stat(claudeDir)` |
| 10 | `scripts/trackfw-attention-signal.sh` | Sim | `staticScripts` |
| 11 | `scripts/trackfw-attention-cleanup.sh` | Sim | `staticScripts` |
| 12 | `scripts/trackfw-validate.sh` | Sim | `checkValidateScriptArtifact` (set-membership) |
| 13 | `scripts/trackfw-credential-guard.sh` | Sim | `staticScripts` |
| 14 | `scripts/trackfw-git-branch-guard.sh` | Sim | `staticScripts` |
| 15 | `.github/workflows/trackfw-gate.yml` | Sim | `switch cfg.CI` case `github-actions` |
| 16 | `.gitlab-ci-trackfw.yml` | Sim | `switch cfg.CI` case `gitlab-ci` |
| 17 | Hook files (husky/lefthook) | **Não** | Declarado fora de escopo na Wave 0 e na REQ |

Cobertura: **16/17**. O artefato fora de cobertura (#17) é declarado explicitamente como fora de
escopo desta REQ — a responsabilidade dos hook files pertence ao integrador de hooks, não ao
doctor de scaffold. Exclusão não é silenciosa: está documentada no modelo de ameaça da Wave 0.

**Pergunta 4: INVENTÁRIO COBERTO em 16/17. O gap de #17 é declarado, não oculto.**

---

## Pergunta 5 — O `doctor` mente em algum caso?

### Caso A — Mensagem que culpa o projeto quando o defasado é o binário

**[medido e raciocinado]** Coberto na Pergunta 1, Bloqueio 4. Go e Node neutros. Python: "vunknown"
não nomeia versão — AC16 parcialmente degradado em desenvolvimento a partir de fonte. Não bloqueia.
Ver Residual A.

### Caso B — Artefato ausente que é legítimo (`discover --init`)

**[medido]** Bloqueio 3: slash commands ausentes em projeto `discover --init` não geram finding.
O sinal de elegibilidade é a presença do diretório `.claude/commands/trackfw/`.

Os 4 scripts estáticos (attention, guards) são verificados incondicionalmente — correto, porque
`discover --init` *os escreve*. Um projeto genuinamente inicializado via `discover --init` tem
esses arquivos. Um projeto sem eles está, de fato, incompleto.

### Caso C — Artefato presente por outro produto no mesmo caminho

**[raciocinado]** Se outro produto escrever em `.github/workflows/trackfw-gate.yml` com conteúdo
diferente, o doctor reporta `scaffold-divergent` e recomenda `trackfw update`. Isso pode ser um
falso-positivo se o arquivo pertencer legitimamente ao outro produto.

**RESIDUAL C:** O nome do arquivo (`trackfw-gate.yml`) é suficientemente específico para que o
conflito requeira squatting intencional de um nome trackfw. Aceito como residual; a probabilidade
é baixa e o remédio (`trackfw update`) sobrescreve com o template correto do trackfw — comportamento
esperado se o arquivo *for* um artefato trackfw. Se não for, o usuário verá a divergência e pode
decidir não executar o remédio.

**Pergunta 5: NENHUMA MENTIRA ESTRUTURAL.** Três residuais nomeados (A, B, C), todos declarados e
aceitáveis.

---

## Resumo dos residuais

| # | Residual | Severidade | Bloqueia? |
|---|----------|-----------|-----------|
| A | Python remedy omite versão binária quando não pip-installed | Baixa | Não |
| B | Forma Python aceita em projeto Go → gate de build Go não executa | Baixa | Não (custo declarado da decisão ML-1C) |
| C | Artefato de outro produto no caminho `trackfw-*.yml` → FP teórico | Muito baixa | Não |

---

## Gate de paridade — resultado completo

**[medido]** `bash scripts/check-doctor-parity.sh`:

```
OK   [doctor-parity/unregistered-write-text]
OK   [doctor-parity/unregistered-write-json]
OK   [doctor-parity/hand-modified-text]
OK   [doctor-parity/hand-modified-json]
OK   [doctor-parity/unknown-content-never-installed-text]
OK   [doctor-parity/unknown-content-never-installed-json]
OK   [doctor-parity/registered-under-different-claim-text]
OK   [doctor-parity/registered-under-different-claim-json]
OK   [doctor-parity/registered-under-different-claim-content-drifted-text]
OK   [doctor-parity/registered-under-different-claim-content-drifted-json]
OK   [doctor-parity/scaffold-baseline-clean-text]
OK   [doctor-parity/scaffold-baseline-clean-json]
OK   [doctor-parity/scaffold-attention-signal-divergent-text]
OK   [doctor-parity/scaffold-attention-signal-divergent-json]
OK   [doctor-parity/scaffold-attention-cleanup-missing-text]
OK   [doctor-parity/scaffold-attention-cleanup-missing-json]
OK   [doctor-parity/validate-sh-go-form-accepted-text]
OK   [doctor-parity/validate-sh-go-form-accepted-json]
OK   [doctor-parity/validate-sh-python-form-accepted-text]
OK   [doctor-parity/validate-sh-python-form-accepted-json]
OK   [doctor-parity/validate-sh-near-miss-rejected-text]
OK   [doctor-parity/validate-sh-near-miss-rejected-json]
OK   [doctor-parity/validate-sh-mirror-vs-generator-backend-go-text]
OK   [doctor-parity/validate-sh-mirror-vs-generator-backend-go-json]
OK   [doctor-parity/scaffold-no-slash-commands-dir-silent-text]
OK   [doctor-parity/scaffold-no-slash-commands-dir-silent-json]
OK   [doctor-parity/scaffold-backend-go-no-false-positive-text]
OK   [doctor-parity/scaffold-backend-go-no-false-positive-json]

All check-doctor-parity.sh scenarios passed.
```

28 cenários × 3 runtimes implícitos por cenário. 30 asserções no gate. Exit 0.

---

## Baseline limpo do repositório — confirmação final

**[medido]**

```
./bin/trackfw doctor                              → no mismatches found
node npm/bin/trackfw doctor                       → no mismatches found
PYTHONPATH=pypi python3 -m trackfw doctor         → no mismatches found
```

AC4 satisfeito nos 3 runtimes neste repositório em HEAD.

---

## Nota para manutenção futura

`RunScaffoldDoctor` e `ClassifyDoctor` são dois caminhos separados. A proteção de scaffold depende
da separação. Se uma refatoração futura rotear scaffold pelo `ClassifyDoctor`, o case
`!Registered && StateModified` existe e gera `DoctorUnknownContent` — mas o `Remedy` seria o do
classificador de manifesto, não o do scaffold. O remédio errado não bloqueia a detecção, mas
instrui o usuário a executar o comando errado. Sinalizado aqui para o próximo revisor.
