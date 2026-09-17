# Threat Model — ML-0A: gerador aplica template de consumidor ao produtor
**REQ:** docs/req/REQ-2026-09-17-gerador-aplica-o-template-de-consumidor-ao-proprio-produtor-e-o-doctor-prescreve-desfazer-a-correcao.md
**Data:** 2026-09-17 | **Papel:** hades-tf | **Branch:** fix/gerador-aplica-o-template-de-consumidor

> Parecer de ameaça pré-implementação. Nenhuma wave de implementação é despachada sem este ser
> auditado. Todas as conclusões têm evidência de leitura ou execução identificada por [L] (leitura
> de arquivo) ou [V] (verificação executada); hipóteses não verificadas são marcadas [H].

---

## 1. Enumeration completeness — a lista de superfícies no roadmap está completa?

O roadmap nomeia três sítios que compartilham o mesmo builder:

| Sítio | Papel |
|---|---|
| `internal/discover/discover.go:276` | escreve o workflow no `init` |
| `internal/generators/update.go:1976` | reescreve no `update` — sem condição além de "existe e não é symlink" |
| `internal/generators/scaffold_doctor.go:269` | compara o disco contra o builder (gera falso-positivo permanente) |

A lista está correta e fechada para o defeito declarado. [L: grep `BuildDiscoverGitHubActionsWorkflowContent` em `internal/`; três ocorrências, nenhuma outra]

**Superfícies adicionais não cobertas pelos três sítios:**

A. **`governance_mode: lenient` permanente — amplificador de severidade (alta severidade).**
   O `trackfw.yaml` deste repositório declara `governance_mode: lenient` sem `lenient_until`.
   [L: trackfw.yaml linha 4]

   O código do validador confirma: `Validate()` chama `validateUnfilteredTagged()`, aplica
   `filterBaselineTagged()` (que tem carve-out para credential_guard), e então — **depois** — verifica
   `IsLenient()` sem nenhum carve-out:
   ```go
   // Modo lenient: mover violations para warnings, exit code 0
   if IsLenient() {
       warnings = append(warnings, violations...)
       violations = nil
   }
   ```
   [L: internal/validator/validator.go linha 625-628, leitura do bloco completo de `Validate()`]

   O mesmo padrão se repete em `ValidateTagged()` (linha 941-944), sem carve-out. Logo,
   **lenient silencia inclusive as 3 regras credential_guard** — a proteção de HEAD-anchoring
   existe mas pertence ao filtro de baseline, que corre antes, e a linha 625 desfaz qualquer
   violation que restou.

   Consequência: `trackfw validate` neste repositório **nunca retorna violação**, mesmo compilado
   do fonte. Tanto `governance-go-install` quanto `governance-install-script` são exit-0 por
   construção, independentemente de qualquer regra ou do binário usado. Esta superfície é um
   **amplificador** que torna o blast radius do defeito do gerador maior do que o roadmap declara.
   Não é do mesmo mecanismo que #366/#376 (ver Seção 4 — o terceiro sítio é outro).

B. **`check-ci-workflow-pin-parity.sh` quebra com a correção (nova, desta análise).** O script
   verifica que `BuildDiscoverGitHubActionsWorkflowContent()` emite `@v<versão>` e tem timeout.
   [L: scripts/check-ci-workflow-pin-parity.sh — a função `dump_go` gera um teste unitário que chama
   o builder sem argumentos e verifica a presença de `@v`]. Um template de produtor não terá
   `go install ...@v<versão>`, logo não terá `@v`. O script vai reprovar contra o template correto
   a menos que seja atualizado no mesmo ML (AC4 do roadmap). Esta superfície deve constar como AC
   obrigatório de ML-1A.

C. **`governance-install-script` usa script de `releases/latest` mas binário pinado.** [L:
   .github/workflows/trackfw-gate.yml linhas 7-20]. O job define `TRACKFW_VERSION: "8.0.0"` no env
   e o install.sh honra essa variável (linha 117: `if [ -n "${TRACKFW_VERSION:-}" ]`). O binário
   instalado é 8.0.0, não latest. Mas o **script em si** vem de `releases/latest/download/install.sh`
   — há uma assimetria entre a versão do script e a versão do binário que ele instala. Para a análise
   de bypass, o que importa é que o binário é publicado (v8.0.0), não o código do PR — isso confirma
   o bypass independentemente da versão.

---

## 2. Threat model — quem esvazia esta Wave 0 sem quebrar regra escrita, e como

**Adversário neste contexto:** o implementador apressado e o arquiteto otimista — não um atacante
externo. Quem age de boa-fé mas não leu o parecer.

**Caminho de esvaziamento principal:**

1. O implementador implementa AC1 (distinguir produtor de consumidor via `go.mod`) e AC7 (doctor
   não acusa `scaffold-divergent`). Marca ML-1A como concluído.
2. Não atualiza `check-ci-workflow-pin-parity.sh` (AC4 implícito). O script quebra no CI.
3. Para fazer o CI passar, o implementador comenta ou enfraquece o script de paridade. Passou.
4. O resultado: o template produtor existe, o doctor está correto, mas a barreira que detecta
   regressão de template fica enfraquecida — exatamente o que o AC4/AC5 do roadmap deveria fechar.

**Segundo caminho (governance_mode: lenient):**

Toda a classe de ataques via `trackfw validate` está efetivamente desativada enquanto
`governance_mode: lenient` sem `lenient_until` estiver no `trackfw.yaml`. Um PR que enfraqueça
qualquer regra passa em `governance-go-install` e `governance-install-script` porque o validate
sai 0 por definição. O implementador que não lê o trackfw.yaml conclui que os dois jobs "validam
a governança" — mas eles apenas verificam que o binário existe e roda sem travar.

**Terceiro caminho (residual pós-correção):**

Mesmo com AC1-AC7 implementados e CI verde, um PR pode adicionar
`rules: {<regra>: off}` ao `trackfw.yaml`. Com lenient mode, isso é redundante. Mas se lenient
for removido no futuro sem que exista um AC que previna `rules: off` para regras críticas, a
superfície reabre sem aviso.

---

## 3. Falsification targets — por superfície, o que quebra e em qual direção

Legenda: [FN] = falso-negativo (bypass não detectado) | [FP] = falso-positivo (correção acusada
como defeito)

### Sítio 1 — `BuildDiscoverGitHubActionsWorkflowContent()` emite template errado

| Direção | Onde entra o dano | Qual gate deveria pegar | Status atual |
|---|---|---|---|
| [FP] template produtor marcado como divergente | `scaffold_doctor.go:269` compara disco contra builder | `trackfw doctor` na raiz (AC7) | ATIVO antes da correção |
| [FN] template consumidor no produtor | `governance-go-install` usa binário publicado | `go` + `windows-full-suites` (unit tests) | LATENTE (revertível por `trackfw update harness`) |

**Evidência [FP]:** confirmada na REQ: `trackfw doctor` na `main` atual reporta
`scaffold-divergent` para `.github/workflows/trackfw-validate.yml` com `remedy: trackfw update`.
**Evidência [FN]:** `governance_mode: lenient` no `trackfw.yaml` faz `governance-go-install` sair 0
independentemente. O único gate real para FN são os unit tests (`go`, `windows-full-suites`) e os
falsify scripts (`parity`).

### Sítio 2 — `update.go:1976` reescreve workflow sem condição

| Direção | Onde entra o dano | Qual gate deveria pegar | Status atual |
|---|---|---|---|
| [FN] `trackfw update harness` reverte arquivo corrigido | desenvolvedor roda manualmente ou CD o invoca | AC5 (scan de workflow) | ABERTO — nenhum gate hoje |
| [FP] impossível | update só age se arquivo existe e não é symlink | — | N/A |

**Evidência:** `update.go:1976` — a única proteção é `if err == nil && !isSymlink` [L confirmado
na sessão anterior]. CI nunca chama `trackfw update` [V: grep em workflows não encontrou chamada].
O vector é manual, mas a ferramenta imprime `remedy: trackfw update` como instrução ao usuário.

### Sítio 3 — `discover.go:276` escreve workflow no init

| Direção | Onde entra o dano | Qual gate deveria pegar | Status atual |
|---|---|---|---|
| [FN] `trackfw discover` reescreve com template consumidor | execução manual em worktree produtor | AC2 (builder distingue contexto) | ABERTO antes da correção |
| [FP] impossível | discover só age se arquivo ausente ou no init | — | N/A |

**Evidência:** `discover.go:276` compartilha o builder [L].

### Sítio 4 — `governance_mode: lenient` permanente (amplificador, não sítio do mesmo mecanismo)

| Direção | Onde entra o dano | Qual gate deveria pegar | Status atual |
|---|---|---|---|
| [FN] toda violation silenciada — validate sai 0 inclusive credential_guard | leitura de `trackfw.yaml` pelo validator | nenhum gate existente exige `lenient_until` | ATIVO — configuração fixa, não PR-introduzida |
| [FP] N/A | — | — | — |

**Evidência:** `internal/validator/validator.go:625-628` — bloco lenient corre após
`filterBaselineTagged` e não tem carve-out para credential_guard. [L confirmado pela leitura
completa de `Validate()` e `ValidateTagged()`, linhas 614-630 e 927-944]. `trackfw.yaml:4` [L].
Não há `lenient_until`. Nenhum AC do roadmap cobre esta superfície.

### Sítio 5 — `check-ci-workflow-pin-parity.sh` quebra com template correto

| Direção | Onde entra o dano | Qual gate deveria pegar | Status atual |
|---|---|---|---|
| [FP] template produtor reprovado por ausência de `@v` | `parity-other-gates` no CI | AC4 do roadmap (atualizar o script junto) | LATENTE — ML-1A deve cobrir |
| [FN] regressão de pin não detectada (já existente) | script só verifica presença de pin, não retrocesso de versão de action | AC4 deve detectar retrocesso | JÁ EXISTENTE, não novo |

**Evidência:** REQ declara: "O `check-ci-workflow-pin-parity.sh` não reprova: ele verifica que
existe pin, não que o pin não retrocedeu." [L: REQ, seção "Efeitos secundários"].

---

## 4. Resposta explícita aos 4 vetores do roadmap

### Vetor 1 — quais required checks pegam um PR que enfraquece uma regra do validador

**Contexto atual** (workflow manualmente corrigido, lenient ativo): `governance-go-install` compila
do fonte MAS `governance_mode: lenient` faz validate sair 0. Logo: **nenhum dos dois jobs de
validate bloqueia a PR**, independentemente do binário.

**Contexto latente** (workflow revertido por `trackfw update harness`): `governance-go-install`
usaria `go install ...@v8.0.0` (binário publicado). `governance-install-script` usa binário
v8.0.0 (pin via `TRACKFW_VERSION`). Com lenient ativo, ambos saem 0 de qualquer forma.

**Required checks e seu status para este vetor:**

| Required check | Pega enfraquecimento de regra? | Evidência |
|---|---|---|
| `governance-go-install` | NÃO — lenient mode faz exit 0 sempre | [L] validator.go:625-628 + trackfw.yaml:4 |
| `governance-install-script` | NÃO — binário publicado + lenient mode | [L] trackfw-gate.yml + validator.go:625-628 |
| `go` | SIM, se unit tests cobrem a regra enfraquecida e não são editados no mesmo PR | [L] quality.yml: `go test ./...` compila do fonte |
| `parity` (falsify shard + check-validate-rule-pins.sh) | SIM para regras cobertas pelos fixtures; NÃO para regras sem fixture | [L] scripts/check-validate-rule-pins.sh cobre 5 grupos; falsify cobre cenários adicionais |
| `windows-full-suites` | SIM, se unit tests cobrem a regra e não são editados | [L] quality.yml: `go test ./...` no Windows |
| `shim-byte-identity-gate` | NÃO — verifica identidade de byte do shim, não comportamento de regras | [L] nome e escopo |
| `package-smoke` | NÃO — smoke test de instalação, não de comportamento de regras | [L] escopo |
| `windows-integrations-resolve` | NÃO — verifica resolução de dependências, não comportamento | [L] escopo |

**Cobertura de regras por gate script** (medido via grep em `scripts/*.sh`):

| Grupo | Regras | Gate script | Unit tests |
|---|---|---|---|
| Cobertas por falsify | wip_has_req, req_has_adr, req_has_roadmap, wip_acceptance, wip_limit, ref_targets_exist, filename_uniqueness, note_orphan | check-gates-falsify.sh | sim |
| Cobertas por union | agent_namespace_undeclared, agent_namespace_hidden | check-agent-namespace-union.sh | 0 unit tests |
| Cobertas por thirdparty | thirdparty_artifact_has_provenance | check-thirdparty-parity.sh | 0 unit tests |
| Apenas unit tests (editáveis no PR) | blocked_has_req, req_roadmap_sync, adr_orphan, stale_wip, folder_status | nenhum gate script | 1-4 unit tests |

As 5 regras do último grupo (`blocked_has_req`, `req_roadmap_sync`, `adr_orphan`, `stale_wip`,
`folder_status`) podem ser enfraquecidas num PR que edite simultaneamente o código da regra e
seus unit tests — nenhum gate script as cobre independentemente.

[V: medição executada com grep por nome de regra em scripts/ e internal/validator/*_test.go]

**Conclusão vetor 1:** Os únicos required checks que podem bloquear um PR que enfraquece uma
regra são `go` e `windows-full-suites` (via unit tests, se não editados no mesmo PR) e `parity`
(via falsify/gate scripts, para regras com fixture). `governance-go-install` e
`governance-install-script` são bloqueados por `governance_mode: lenient`, não pelo binário.
As 5 regras sem gate script independente são silenciáveis com um PR que edite código + testes.

### Vetor 2 — `update.go` pode ser induzido durante o CI?

**Veredito: FECHA. O CI nunca chama `trackfw update`.** [V: grep em todos os workflows não
encontrou chamada a `trackfw update`]. A reescrita é manual (desenvolvedor ou script externo).
A mitigação AC5 (gate que detecta `go install ...@v` em workflows do repositório) fecha o
caminho de merge sem depender de vigilância manual.

**Ressalva:** `discover.go:276` escreve no `trackfw discover` — se um job de CI chamar
`trackfw discover`, o workflow seria reescrito. [V: grep em workflows não encontrou chamada a
`trackfw discover` em CI]. Não há vetor via CI hoje.

### Vetor 3 — o sinal `go.mod` é confiável?

**Veredito: FECHA com ressalva menor.** O `go.mod` declara
`module github.com/kgsaran/trackfw` — módulo único, nenhum fork externo usa esse nome de módulo.
Um consumidor que clonasse o repositório e declarasse o mesmo módulo seria tratado como produtor
e obteria o template "compila do fonte" — que é mais rigoroso, não menos. O false positive é
**auto-punitivo**: o CI do consumidor tentaria `go build ./cmd/trackfw` sem o código, falharia
imediatamente no job de build. Não há ganho para o consumidor.

**Ressalva:** se o repo for um fork do trackfw (contribuidor externo), ele é de fato produtor e
o template correto é o de compilar do fonte. O sinal é semanticamente correto neste caso.

### Vetor 4 — terceiro sítio da mesma família que #366 e #376

**Veredito: NÃO FECHA. Sítio identificado, não coberto por este roadmap.**

**Família da classe:** "código do repositório é lido pelo verificador durante o CI e influencia
o que ele verifica sobre o próprio PR — permitindo ao PR controlar seu resultado de governança".

**Sítio 4A — amplificador pré-existente (`governance_mode: lenient`), NÃO terceiro sítio:**
O `governance_mode: lenient` foi escrito por `trackfw discover` no onboarding (discover.go:499)
e está na `main` desde então — **é configuração fixa, igual para todo PR**. O mecanismo do #366
e do #376 é "o diff do PR muda o que o CI verifica sobre esse diff". Lenient não é introduzido
pelo PR adversário — ele já está lá. É um **amplificador**: torna a janela de exploração do
defeito do gerador maior do que o roadmap declara, mas corrigir lenient não fecha o gerador,
e corrigir o gerador não fecha lenient. São causas separadas.

**Sítio 4B — `rules: {<regra>: off}` no `trackfw.yaml` (terceiro sítio real, NÃO FECHA):**
Um PR pode adicionar `rules: {wip_has_req: off}` (ou qualquer regra não-credential-guard) ao
`trackfw.yaml`. O `governance-go-install`, quando compilar do fonte e com lenient desativado,
lê o arquivo local e silencia a regra via campo `rules:`. O PR controla o resultado da
verificação porque o verificador lê o repositório auditado para decidir o que verificar —
exatamente o mecanismo do #366 e do #376.

ADR-2026-08-12 já documentou este ponto (linha 204 do validator.go): as 3 regras
credential_guard foram tornadas HEAD-anchored **precisamente porque** `rules:
credential_guard_mode_downgrade: off` no `trackfw.yaml` as silenciaria. O comentário
diz "never committed" — mas nenhum gate existente previne que um PR as commite. As demais ~21
regras não têm proteção análoga. Nenhum AC do roadmap vigente endereça esta superfície.

**Nota:** com lenient ativo hoje, 4B é redundante — validate sai 0 de qualquer forma. 4B
se tornará exploitável quando (e se) lenient for removido sem que exista uma barreira nova.

---

## 5. Residual declarado — o que esta análise aceita não cobrir

1. **`governance_mode: lenient` permanente não é corrigido aqui.** O roadmap vigente não
   tem AC para exigir `lenient_until` ou remover o modo lenient. Toda a análise de bypass
   via `governance-go-install`/`governance-install-script` é dominada por este fato.
   Enquanto lenient estiver ativo, esses dois jobs são decorativos para detecção de
   enfraquecimento de regras.

2. **Sítio 4A e 4B precisam de REQ própria** (mesmo mecanismo, nova superfície — não
   cobertos pelo defeito do gerador). O presente roadmap não autoriza correção desses sítios.

3. **5 regras sem gate script independente: `blocked_has_req`, `req_roadmap_sync`, `adr_orphan`,
   `stale_wip`, `folder_status`.** Medido: nenhum script `.sh` em `scripts/` as referencia para
   fins de gate — apenas arquivos `.md` (documentação). A cobertura dessas regras descansa
   inteiramente nos unit tests, editáveis no mesmo PR que enfraquece a regra. [V: grep medido]

4. **Regressão de versão de action não detectada pelo `check-ci-workflow-pin-parity.sh`.**
   O script verifica presença de pin `@v`, não que o pin não retrocedeu. Uma regressão
   `v7` → `v4` passa. Este defeito é pré-existente e não é aberto por este roadmap.

5. **A assimetria script/binário em `governance-install-script`** (script de `releases/latest`,
   binário de v8.0.0) não foi classificada como vetor de risco porque o script honra
   `TRACKFW_VERSION`. Se em alguma versão futura o formato da variável mudar no script e não
   for refletido no job, a versão efetiva pode flutuar. Nenhum gate verifica que o script
   e o binário são da mesma versão.

---

## O que não consegui determinar

- **Se o `trackfw doctor` na raiz do repositório reporta `scaffold-divergent` neste momento.**
  A REQ afirma que sim (medido num worktree limpo da main antes da sessão). A verificação
  executada (`trackfw doctor` compilado do PR) não foi realizada nesta sessão — seria AC7,
  pertence a Apolo, não ao parecer.

- **Se `check-thirdparty-parity.sh` é invocado como required check ou apenas como gate script
  local.** O arquivo existe e referencia `thirdparty_artifact_has_provenance`, mas não foi
  verificado se está declarado em `.github/required-status-checks.txt`. Se não for required,
  `thirdparty_artifact_has_provenance` tem zero cobertura em required checks.
