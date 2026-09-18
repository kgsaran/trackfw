# Threat Model — Severidade da Validação (Issue #387)

> Papel: Hades (Security Reviewer) · Data: 2026-09-17
> Entregável do ML-0A, Wave 0 do Roadmap #387.
> Pergunta central: quem continua conseguindo afrouxar a própria verificação DEPOIS desta REQ?

---

## 1. Completude da Enumeração — a lista de superfícies está fechada?

O roadmap nomeia dois interruptores: `governance_mode: lenient` e `rules: {<regra>: off}`.
A lista não está fechada. Foram encontrados cinco canais adicionais com evidência medida.

### Canal A — `governance_mode: lenient` sem `lenient_until` (nomeado na REQ)

`IsLenient()` em `validator.go:381-383`:
```
if gm.LenientUntil.IsZero() {
    return true
}
```
Prazo ausente retorna `true` imediatamente — falha aberta confirmada por leitura direta.
`discover.go:497-499` omite `lenient_until` ao escrever o arquivo; `scaffold.go:735` inclui.
Efeito: `validator.go:626` e `:942` executam `warnings = append(warnings, violations...); violations = nil` — incondicional nos dois sítios.
Veredito: **fecha** (AC2 + AC3).

### Canal B — `rules: {<regra>: off}` (nomeado na REQ)

`ruleSeverity()` em `validator.go:207-212` despacha para `diskRuleSeverity()` em `:218-227` para toda regra fora de `credentialGuardAnchoredRules`. `diskRuleSeverity` lê `cfg.Rules[name]` do disco — o arquivo que o PR edita.

`applyRule` em `validator.go:231-243` e `applyRuleTagged` em `validator.go:247-263`:
```
case "off":
    // silencioso
```
Regra com severidade `"off"` não produz nenhuma saída — confirmado por leitura do corpo das duas funções.

Efeito: qualquer regra não integrante de `credentialGuardAnchoredRules` pode ser silenciada por `rules: {<regra>: off}` em `trackfw.yaml`. Há ~21 regras nessa condição (todas exceto as 3 em `credentialGuardAnchoredRules`).
Veredito: **mitiga parcialmente** (AC1 fecha este canal para regras ancoradas; ver Eixo 2 e Eixo 3).

### Canal C — `req_dir` / `roadmap_dir` / `adr_dirs` apontando para diretórios existentes e vazios

**Medição realizada:** fixture com `governance_mode: strict`, `req_dir`, `roadmap_dir` e `adr_dirs` apontando para diretórios temporários existentes mas vazios. Resultado:

```
$ trackfw validate ; echo "RC=$?"
✓ No violations found.
RC=0
```

Zero violações em modo estrito, sem tocar `governance_mode` nem `rules:`. O validator não encontra artefatos para avaliar e conclui que está tudo em ordem. Este canal está inteiramente fora do alcance da AC1 (que ancora severidade por regra, não configuração de caminhos).

`config.go:413-480` mostra que `req_dir`, `roadmap_dir` e `adr_dirs` são lidos do mesmo arquivo que o PR edita — sem qualquer comparação com HEAD.

Veredito: **não toca** — AC1 ancora severidade de regra, não a configuração de `req_dir`. Vetor nomeado, não fechado por esta REQ.

### Canal D — `.trackfw-baseline.json` via force-add (gitignore contornado)

O arquivo `.trackfw-baseline.json` está listado no `.gitignore`. `LoadBaseline()` em `validator.go:27-41` lê do CWD sem nenhuma verificação de rastreamento git — confirmado pela leitura do código e pelo comentário em `:641-644`: "`.trackfw-baseline.json` is `.gitignore`'d on purpose... `require a commit` simply does not apply to a file the project decided never to version."

**Medição realizada:** baseline com mensagem verbatim suprimiu o aviso alvo (0 ocorrências na saída, confirmado por grep); RC permaneceu 0. O delta de contagem total (175 sem baseline → 176 com baseline, em vez do esperado 174) não foi explicado — a causa provável é que a presença do arquivo `.trackfw-baseline.json` no workspace dispara alguma verificação adicional, mas não foi medida. A supressão da mensagem-alvo por texto exato está confirmada; o delta total está não explicado.

A carve-out em `filterBaselineTagged` (`validator.go:636-676`) exclui as 3 regras de `credentialGuardAnchoredRules` — esse canal não alcança credential-guard. Todas as demais regras são suprimíveis por mensagem exata no baseline.

Vetor de ataque: PR que força-commita `.trackfw-baseline.json` com mensagens exatas das violações que quer suprimir pode silenciar qualquer violação não-credential-guard, incluindo carve-out rules que esta REQ pretende tornar permanentes (ver Eixo 4).
Veredito: **não toca** — esta REQ não aborda o canal de baseline. Vetor nomeado, não fechado.

### Canal E — `stale_wip_days` e `wip_limit` (knobs comportamentais)

`config.go` lê `stale_wip_days` e `wip_limit` do mesmo `trackfw.yaml`. Valores extremos (ex: `stale_wip_days: 999999`) silenciam as regras correspondentes sem passar pelo mecanismo de `rules:`. Não são afetados por AC1. Impacto limitado ao escopo das regras de WIP — não tocam `req_has_adr`, `req_has_roadmap`, `adr_orphan`.
Veredito: **não toca** — escopo restrito, não fechado por esta REQ. Vetor nomeado.

---

## 2. Ordem 3-antes-de-1 — apertar `lenient` sem ancorar severidade troca um buraco por outro

Confirmado por leitura do código. `ruleSeverity()` em `validator.go:207-212`:
```
func ruleSeverity(name string) string {
    if credentialGuardAnchoredRules[name] {
        return credentialGuardRuleSeverity(name)
    }
    return diskRuleSeverity(name)
}
```
Para toda regra fora de `credentialGuardAnchoredRules`, a severidade vem exclusivamente de `diskRuleSeverity()`, que lê `cfg.Rules[name]` do disco (`validator.go:218-227`).

Se AC2 (apertar `IsLenient()`) for implementada sem AC1 (ancoragem em HEAD para todas as regras), o resultado é:
- O interruptor global `governance_mode: lenient` é fechado
- Os ~21 interruptores por regra (`rules: {<regra>: off}`) permanecem abertos para leitura do disco
- Um PR que commita `rules: {req_has_adr: off}` silencia essa regra sem tocar `governance_mode`

A ADR nomeia isso explicitamente: "apertar o `lenient` sem ancorar a severidade por regra reabre a superfície `rules:` para as ~21 regras não ancoradas — troca-se um buraco por outro." A leitura do código confirma a mecânica.

Caminho não coberto pelo AC1 como especificado: veja Eixo 3 abaixo.

---

## 3. A ancoragem em HEAD é ela mesma contornável?

### Mecanismo atual (3 regras de credential-guard)

`headTrackfwYAML()` em `validator_credential_guard_integrity.go:128-150` executa:
```
git rev-parse --verify HEAD
git show HEAD:./trackfw.yaml
```
Retorna `ok=false` quando: repositório sem HEAD, `trackfw.yaml` não rastreado em HEAD, ou falha de git.

`credentialGuardRuleSeverity()` em `:293-310`: quando `ok=false`, retorna `diskSeverity` diretamente — **falha aberta** confirmada por leitura direta.

### O comentário do código documenta o escopo declarado

`validator.go:202-206` diz explicitamente:
> "those 3 rules can otherwise be silenced by the very same **uncommitted** edit they exist to catch (`rules: credential_guard_mode_downgrade: off` in trackfw.yaml, **never committed**)."

O padrão foi projetado para detectar edições **não commitadas** — o atacante modelado é alguém que edita `trackfw.yaml` localmente para silenciar as regras antes de rodar validate. Edições commitadas nunca foram o alvo.

### AC1 generaliza o padrão — mas o mesmo limite se aplica

Se AC1 estender o mesmo mecanismo a todas as regras, ele fechará o mesmo canal que as 3 regras de credential-guard fecham: edições locais não commitadas. O canal de edições **commitadas** não é fechado.

### HEAD == disco para mudanças commitadas em CI

**Medição realizada:** simulação de PR merge commit. Em `actions/checkout@v7` sem `fetch-depth:`, o checkout padrão no evento `pull_request` usa `refs/pull/N/merge` — o commit de merge onde HEAD já incorpora as mudanças do PR:

```
disk = PR's version of trackfw.yaml
HEAD (git show HEAD:./trackfw.yaml) = merge commit's version = mesma que o disco
HEAD == disco → sem diferencial → ancoragem não tem efeito
```

Medido: PR que commita `rules: {req_has_adr: off}` → HEAD:./trackfw.yaml contém `req_has_adr: off` → diskRuleSeverity retorna `"off"` → applyRule silencia → nenhuma diferença detectada.

Os dois workflows de governança (`trackfw-gate.yml`, job `governance-install-script`; `trackfw-validate.yml`, job `governance-go-install`) usam `actions/checkout@v7` sem bloco `with:` — depth padrão 1, sem `git fetch origin main`.

### Shallow clone não quebra `git show HEAD`

**Medição realizada:** clone `--depth=1` de merge commit. `git show HEAD:./trackfw.yaml` retorna conteúdo correto (RC=0). Clone raso limita histórico de commits, não conteúdo da árvore em HEAD.

### `HEAD^1` é inacessível em clone raso — base-branch requer mudança de workflow

**Medição realizada:** em clone `--depth=1` de merge commit, `git rev-parse HEAD^1` retorna RC=128 (fatal: ambiguous argument). `git show HEAD^1:trackfw.yaml` retorna RC=128.

Para ancorar contra a base branch (comparar `trackfw.yaml` do PR contra o estado em `origin/main`), o implementador precisaria de:
1. `fetch-depth: 2` no checkout (para acessar `HEAD^1` no merge commit), OU
2. `git fetch origin main` explícito no workflow + `git show origin/main:./trackfw.yaml`

Nenhuma das duas medidas está nos workflows atuais. Se AC1 for implementada sem mudança de workflow, o canal de edições commitadas permanece aberto.

Veredito: AC1 como especificada fecha edições não commitadas. O canal de edições commitadas em CI (PR que commita `rules: {<regra>: off}`) permanece não fechado. Vetor nomeado — a ADR afirma "a mais estrita entre HEAD e disco prevalece", mas HEAD == disco para mudanças commitadas em CI.

---

## 4. O carve-out — quem produz inconsistência ativa fora da lista? As 157 históricas caem no carve-out?

### Estado atual das 8 ocorrências ativas (medidas em 2026-09-17, 175 avisos totais)

| Quantidade | Regra | Código | Rota |
|---|---|---|---|
| 6 | REQ aberta, roadmap em `done/` | `validator.go:~786-788` | **Hardcoded direto em `warnings`**, fora de `applyRuleTagged` |
| 2 | Caminho de estado obsoleto | `validator.go:~780` | Passa por `applyRuleTagged("ref_targets_exist", ...)` |

### As 6 ocorrências de `req_roadmap_lifecycle` estão hardwired como warnings e nunca passam por `ruleSeverity()`

`validateREQRoadmapLifecycle()` em `validator.go:2788` (função auxiliar) retorna mensagens como `[]string`. O chamador em `Validate()` em linha 482 as appenda diretamente a `warnings`. O chamador em `ValidateTagged()` em linhas 786-787:
```
for _, m := range reqLifecycleWarnings {
    warnings = append(warnings, TaggedMsg{Rule: "req_roadmap_lifecycle", Msg: m})
}
```
Ambos os sítios adicionam diretamente a `warnings`, **sem chamar `applyRuleTagged`**. A função `ruleSeverity("req_roadmap_lifecycle")` nunca é consultada em nenhum caminho.

Verificação: `./bin/trackfw validate --json` retorna `"rule": "req_roadmap_lifecycle"` para as 6 mensagens — o tag aparece porque `ValidateTagged()` envolve manualmente em `TaggedMsg` na linha 787, não porque passou pelo sistema de severidade. A presença do tag no JSON não implica que `applyRuleTagged` ou `ruleSeverity()` foi chamado.

Consequência 1: `rules: {req_roadmap_lifecycle: off}` não tem efeito — a regra está fora do sistema de severidade.
Consequência 2: o bloco de leniência em `:626` e `:942` move **violations** para warnings, não warnings para violations. As 6 mensagens já são warnings por construção — `lenient` não as move, não as silencia, não as eleva.
Consequência 3: **o carve-out AC4 é um no-op para essas 6 ocorrências** tal como especificado. AC4 define "conjunto nomeado e fechado de regras que o `lenient` não cobre" — mas essas 6 nunca foram cobertas pelo `lenient` (são hardcoded warnings). O carve-out precisaria primeiro rotear `req_roadmap_lifecycle` por `applyRuleTagged`, com default "error", para que o bloco lenient as converta — e então o carve-out as excetuaria dessa conversão.
Consequência 4: AC5 ("validate sai ≠ 0 apontando exatamente as 6 + os 2") não é atingível por ML-2A como escrito, porque as 6 nunca foram violations e o carve-out não as torna violations sem mudança de roteamento. Este é um pré-requisito de implementação para o ML-2A, não diagnosticado no roadmap.

### As 2 ocorrências de `ref_targets_exist` passam pelo sistema de severidade

`applyRuleTagged("ref_targets_exist", ...)` → `ruleSeverity("ref_targets_exist")` → `diskRuleSeverity` → não está em `ruleDefaults` → default "error" → violations → bloco lenient em `:942` converte para warnings.

Essas 2 são corretamente atingíveis pelo carve-out. `rules: {ref_targets_exist: off}` SILENCIARIA essas 2 (não são credentialGuard), o que é um vetor residual para o canal D (baseline) ou B (rules: off).

### As 157 históricas não caem no carve-out

Composição: 122 `req_has_adr` (REQ sem ADR vinculado), 36 `req_has_roadmap` (REQ sem roadmap vinculado), 7 `adr_orphan` (ADR sem REQ referenciando).

Critério de pertencimento ao carve-out: "contradição entre dois artefatos vivos." As 157 representam ausência de um segundo artefato, não contradição entre dois existentes. Uma REQ sem ADR não contradiz nenhum ADR — o ADR simplesmente não existe. Uma REQ sem roadmap não contradiz nenhum roadmap — o roadmap não existe. Um ADR órfão não contradiz nenhuma REQ — simplesmente não é referenciado.

Verificação: nenhuma das regras `req_has_adr`, `req_has_roadmap` ou `adr_orphan` detecta estado de artefato existente em conflito com outro artefato existente. São verificações de ausência. Nenhuma cairia no carve-out por acidente.

### Direção falso-negativa do carve-out: `req_roadmap_sync` não está na lista mas cabe no critério

`ruleDefaults` em `validator.go:193`: `"req_roadmap_sync": "warning"` — divergência entre `roadmap:` no frontmatter e `Roadmap:` no corpo quando ambos estão preenchidos mas apontam para basenames distintos. Pelo critério da ADR ("contradição entre dois artefatos vivos"), essa regra pertence ao carve-out: dois campos do mesmo documento em contradição ativa. Tem zero ocorrências hoje (por isso não entrou na calibração contra as 8 ativas), mas a calibração contra o corpus atual não é critério de pertencimento — o critério é a natureza da violação. Se ocorrer após a implementação do carve-out, não será bloqueante por não estar na lista fechada. Esta é a direção falso-negativa da especificação do carve-out.

### Corpus drift: 177 avisos medidos (JSON), 172 no roadmap

O roadmap/REQ registram 172 avisos medidos em 2026-09-17 no momento de escrita. A medição por `validate --json` nesta sessão retorna 177, com composição: 122 `req_has_adr`, 36 `req_has_roadmap`, 8 `adr_orphan`, 6 `req_roadmap_lifecycle`, 2 `ref_targets_exist`, 1 `wip_has_req`, 1 `wip_acceptance`, 1 regra vazia (`frontmatter_presence` em ADR-001 appenda com tag vazio via `readFileForRule` em `validator.go:2302/2307`, fora de `applyRuleTagged`). Os avisos `wip_has_req` e `wip_acceptance` são do próprio roadmap desta REQ — instância ao vivo do problema do carve-out, produzidos durante este ML.

---

## 5. Residual Declarado

O que este desenho aceita não cobrir, dito claramente:

**R1. Edições commitadas de severidade em CI (principal residual).**
Um PR que commita `rules: {<qualquer-regra-não-credentialGuard>: off}` tem essa regra silenciada em CI mesmo após AC1 implementada, porque HEAD == disco no merge commit. Fechar este canal requer comparação contra `origin/main:trackfw.yaml`, que por sua vez requer mudança de workflow (`fetch-depth: 2` ou `git fetch origin main`). Os workflows atuais (`trackfw-gate.yml`, `trackfw-validate.yml`) não têm essa mudança. Nomeado, não fechado por esta REQ.

**R2. Bypass por caminhos vazios.**
`req_dir`, `roadmap_dir` ou `adr_dirs` apontando para diretórios existentes e vazios produz RC=0 em modo estrito — medido. AC1 ancora severidade de regra, não configuração de caminho. Nomeado, não fechado.

**R3. Baseline force-committed.**
`.trackfw-baseline.json` gitignored mas force-committable. Supressão por mensagem exata confirmada medida. Carve-out só para as 3 regras de credentialGuard. Nomeado, não fechado.

**R4. `lenient_until` com data arbitrariamente distante.**
AC2 fecha a ausência de `lenient_until`. Não restringe o valor máximo. `lenient_until: 9999-12-31` é válido e concede leniência efetivamente eterna, agora com campo preenchido. Nomeado, não fechado.

**R5. Knobs comportamentais (`stale_wip_days`, `wip_limit`).**
Silenciam regras específicas sem usar o mecanismo `rules:`. Fora do alcance de AC1. Impacto limitado ao escopo de WIP. Nomeado, não fechado.

**R6. `req_roadmap_lifecycle` fora do sistema de severidade.**
As 6 ocorrências ativas do carve-out são hardcoded warnings, não violations. O carve-out AC4 não as torna bloqueantes sem mudança de roteamento que não está no ML-2A como escrito. Este é um pré-requisito de implementação não diagnosticado. Nomeado — ML-2A precisará rotear `req_roadmap_lifecycle` por `applyRuleTagged` antes de poder invocar o carve-out sobre ela.

**O que este desenho fecha, ao ser implementado:**
- Canal A: `lenient` sem prazo concede permissão (AC2 + AC3)
- Canal B para edições não commitadas: `rules: off` em `trackfw.yaml` não commitado (AC1 + HEAD anchoring)
- O mecanismo de expiração que existia e era burlável por omissão

**O que não está no escopo e está documentado na Negative Scope da REQ:**
- Saldar as 157 pendências históricas
- Alterar branch protection ou required-status-checks
- Decidir versão do release

---

*Hades, 2026-09-17. Nenhuma linha de implementação foi escrita neste ML.*
