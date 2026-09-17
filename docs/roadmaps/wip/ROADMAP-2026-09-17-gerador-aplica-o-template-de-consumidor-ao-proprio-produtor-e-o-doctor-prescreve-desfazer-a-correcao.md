---
status: wip
date: 2026-09-17
req: "docs/req/REQ-2026-09-17-gerador-aplica-o-template-de-consumidor-ao-proprio-produtor-e-o-doctor-prescreve-desfazer-a-correcao.md"
squad: ""
---

# Roadmap: gerador aplica o template de consumidor ao proprio produtor e o doctor prescreve desfazer a correcao

> Created: 2026-09-17 | Status: wip


## Wave 0 — Threat model
> Dependências: nenhuma. 🔴 **Auditada antes de qualquer wave de implementação.**

### ML-0A — o que um PR consegue esconder quando a validação roda o binário publicado
**Status:** ⬜ Pendente · **Papel:** `hades-tf`

O enquadramento deste issue foi "falso-positivo do doctor". A pergunta de segurança é outra e é mais
séria: **o `governance-go-install` é required e, hoje, valida com o binário publicado.**

1. 🔴 **Bypass de verificação.** Um PR que altere `internal/validator/` — regras, severidade,
   mensagens — passa no check, porque o binário publicado não contém a alteração. **Enumere o que
   isso permite esconder**, e diga se algum outro required check pegaria (`go`, `parity`,
   `windows-full-suites`). Se outro pegar, o risco é menor do que parece; se nenhum pegar, o issue é
   mais grave do que está escrito.
2. **Superfície do `update`.** `update.go:1976` reescreve o workflow **sem condição** além de "existe
   e não é symlink". Um PR pode induzir reescrita de workflow? E o `discover.go:276`?
3. **O sinal proposto no AC1 é o `go.mod`.** Ele é confiável? O `go.mod` vem do próprio repositório —
   um consumidor hostil que declare `module github.com/kgsaran/trackfw` ganha alguma coisa ao ser
   tratado como produtor, ou é irrelevante por não haver privilégio associado?
4. **Precedente.** O #366 (gate escrevendo na árvore auditada) e este são a mesma família — código do
   repositório influenciando o que o CI executa. Diga se há **terceiro sítio** dessa família que
   nenhum dos dois issues cobre.

**Aceite:** parecer com os vetores enumerados e, por vetor, veredito (fecha / mitiga / não toca) com
evidência de leitura. 🔴 Vetor não fechado entra **nomeado**. Seção final sobre o que ficou em aberto.

**Gates da wave 0:**
```bash
P=docs/portabilidade/2026-09-17-threat-model-gerador-produtor-consumidor.md
test -s "$P" || { echo "FAIL: parecer ausente ou vazio: $P"; exit 1; }
for termo in "governance-go-install" "go.mod" "update" "required"; do
  grep -qi -- "$termo" "$P" || { echo "FAIL: parecer nao cobre: $termo"; exit 1; }
done
grep -qiE "residual|nao consegui|não consegui|indetermin" "$P" \
  || { echo "FAIL: parecer nao declara o que ficou em aberto"; exit 1; }
echo "OK [wave0/threat-model-gerador]: parecer presente e cobre os quatro eixos"
```

**Status da Wave 0:** ✅ **Auditada em 2026-09-17** — parecer em
`docs/portabilidade/2026-09-17-threat-model-gerador-produtor-consumidor.md`, gate acima executado
com RC=0. Dois achados do parecer foram **verificados por mim, no código**, não aceitos de palavra:

| achado | verificação minha | destino |
|---|---|---|
| Sítio 5 — `check-ci-workflow-pin-parity.sh` reprova a correção | `check_discover_pin` (l. 144-156) exige `grep -qF "@v${expected}"` e falha sem ele | **AC8 da REQ** — entra no ML-1A |
| `governance_mode: lenient` zera os dois required checks de governança | `trackfw validate` → RC=0 com 171 avisos e 0 violações; `validator.go` move todas as violations para warnings incondicionalmente | **causa distinta → issue #387** |

🔴 **Por que o segundo não entra nesta REQ:** pela Regra Dura de Causa Raiz, mesma causa fica na mesma
REQ — e esta causa **é outra**. Aqui o defeito é o **gerador** não distinguir produtor de consumidor;
lá é o **`trackfw.yaml`, editável pelo PR, governar a severidade da verificação desse mesmo PR**.
Corrigir o gerador não fecha o #387, e corrigir o #387 não fecha este. O teste da regra
("se eu corrigir esta causa, exatamente estas falhas fecham — e nenhuma outra") separa os dois.

⚠️ **Consequência operacional para o ML-1A:** dois dos oito required checks
(`governance-go-install`, `governance-install-script`) **não podem reprovar** enquanto o #387 não for
tratado. A validação desta wave **não pode se apoiar neles** — o aceite é o `doctor` (AC7), o
`check-ci-workflow-pin-parity.sh` corrigido (AC8) e o `quality`.

---

## Wave 1 — Correção
> Dependências: **Wave 0 auditada.**

### ML-1A — gerador distingue produtor de consumidor, e o doctor compara contra o template certo
**Status:** ⬜ Pendente · **Papel:** `apolo-tf`
Cobre AC1 a AC7.

🔴 **O AC1 sozinho não fecha o issue.** Sem o AC2, o `doctor` continua acusando o arquivo correto como
divergente e prescrevendo `trackfw update` — a pressão para reverter permanece. Os três consumidores
do builder (`discover.go:276`, `update.go:1976`, `scaffold_doctor.go:269`) têm de concordar.

**A medição que fecha:** `trackfw doctor` na raiz deste repositório deixa de reportar
`scaffold-divergent` para `.github/workflows/trackfw-validate.yml` (AC7). Qualquer outra evidência é
indireta.

🔴 **Cobre também o AC8, acrescentado pela auditoria da Wave 0 — e ele é bloqueante.**
`scripts/check-ci-workflow-pin-parity.sh` exige hoje `@v<versão>` no template do `discover`
(`check_discover_pin`, l. 144-156). Um template de produtor que compila do fonte não tem essa string:
**o gate reprova a própria correção**, e ele roda no `quality`. Entregar o AC1 sem o AC8 deixa o PR
vermelho por construção.
O gate passa a exigir `@v<versão>` no template **de consumidor** e a **ausência** de `go install` no
template **de produtor** — os dois braços, no mesmo ML.

⚠️ **Sem o AC8, o AC1 e o AC5 se contradizem:** o AC5 proíbe `go install …@v` nos workflows deste
repositório; o gate atual o exige. Não declare nenhum dos dois atendido com o script intocado.

---

## Context
<!-- What problem does this roadmap solve? Link the REQ. -->
REQ: docs/req/REQ-2026-09-17-gerador-aplica-o-template-de-consumidor-ao-proprio-produtor-e-o-doctor-prescreve-desfazer-a-correcao.md

## Acceptance Criteria
<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->
- [ ]
- [ ]

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model for this roadmap
**Status:** ✅ Concluído (auditado em 2026-09-17 — ver veredito na Wave 0 acima)
**Files affected:**
**Actions:**
1. Enumeration completeness — is the list of surfaces in this roadmap complete? Name what is missing, or show the list is closed. Do not limit the search to the files already named by the REQ — before declaring the list closed, search the repository for other places that emit the same artifact or the same pattern (for example, grep for the literal the final artifact contains).
2. Threat model — who empties this Wave 0 without breaking any written rule, and how?
3. Falsification targets in both directions — for each surface, what breaks when the behavior regresses, and what breaks when it regresses the opposite way?
4. Declared residual — what this design accepts not covering.
**Acceptance criteria:**
- [ ] The four sections above answered with evidence, not a one-line assertion
- [ ] No implementation line written for this ML

**Gates da wave:**
```bash
# Wave 0 gate — replace this placeholder with a project-specific check before
# marking ML-0A done. Do not remove the gate; replace its command (AC13).
scripts/check-orphan-gates.sh
```

## Wave 1 — <name> (parallel MLs)
> Dependencies: none

### ML-1A — gerador aplica o template de consumidor ao proprio produtor e o doctor prescreve desfazer a correcao
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] build passes
- [ ] tests green
- [ ] validate passes
