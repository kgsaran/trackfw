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
**Status:** ✅ Concluído (auditado por Zeus em 2026-09-17) · **Papel:** `hades-tf`

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

**Critérios de aceite:**
- [x] Parecer entregue em `docs/portabilidade/2026-09-17-threat-model-gerador-produtor-consumidor.md`
- [x] Os quatro eixos respondidos com evidência de leitura, não asserção de uma linha
- [x] Vetores não fechados nomeados, com residual declarado em seção própria
- [x] Nenhuma linha de implementação escrita neste ML
- [x] Gate da wave 0 executado por Zeus com RC=0
- [x] Dois achados verificados por Zeus no código, não aceitos de palavra (ver tabela abaixo)


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
**Status:** 🔄 Em andamento · **Papel:** `apolo-tf`
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


**Arquivos afetados:**
- `internal/generators/scaffold_doctor.go` (builder, l. 53; comparação com o disco, l. 269)
- `internal/discover/discover.go` (l. 276 — escreve o workflow)
- `internal/generators/update.go` (l. 1976 — reescreve o workflow)
- `scripts/check-ci-workflow-pin-parity.sh` (`check_discover_pin` l. 144-156 **e** `dump_go`)
- `.github/workflows/trackfw-validate.yml` — 🔴 **o arquivo commitado, regenerado** (ver Ação 5)
- testes em `internal/generators/` e `internal/discover/`

**Ações:**
1. **AC1** — o builder passa a receber o contexto (produtor vs. consumidor). Sinal: o `go.mod` do
   alvo declarar `module github.com/kgsaran/trackfw`. Produtor compila do fonte; consumidor mantém
   a instalação a partir do release **inalterada**.
2. **AC2** — os três consumidores do builder (`discover.go:276`, `update.go:1976`,
   `scaffold_doctor.go:269`) passam o mesmo contexto. Se o `doctor` comparar contra o template do
   outro contexto, o falso-positivo apenas troca de sinal.
3. **AC3/AC4** — versão do Go e versões das actions deixam de ser literais desalinhados.
4. **AC8** — `check-ci-workflow-pin-parity.sh` passa a verificar **os dois braços**: exige a pinagem
   `@v<versão>` no template **de consumidor** e exige a **ausência** de instalação por release no
   template **de produtor**.
   ⚠️ **`dump_go` escreve `zz_dump_ci_workflow_pin_parity_test.go` chamando
   `BuildDiscoverGitHubActionsWorkflowContent` por nome e aridade.** Ao acrescentar o parâmetro de
   contexto (Ação 1), esse teste gerado **para de compilar** — e a falha lê como "gate instável", não
   como escopo faltando. Atualize `dump_go` na mesma entrega, emitindo os dois contextos e afirmando
   cada braço separadamente.
5. 🔴 **Regenerar e commitar `.github/workflows/trackfw-validate.yml`.** Mudar o builder **não** muda
   o arquivo em disco: sem este passo o `doctor` continua reportando `scaffold-divergent`, agora pelo
   motivo espelhado, e o **AC7 não fecha**. Restrição: os **nomes dos jobs** dentro do arquivo
   permanecem byte a byte idênticos — nome de job é contrato com o `required_status_checks`, e
   renomear deixa todo PR pendente para sempre (escopo negativo da REQ).
6. **AC6** — falsificação nas duas direções, com contra-braço: a versão sem a correção produz o mesmo
   template **nos dois** fixtures. Pela Regra Dura de Reconciliação, cada teste novo vem acompanhado
   da frase que diz qual conclusão deste ML ele afirma.

**Critérios de aceite:**
- [ ] AC1 — fixture cujo `go.mod` é o do trackfw gera template que compila do fonte
- [ ] AC2 — os três consumidores concordam quanto ao contexto
- [ ] AC3/AC4 — versão do Go e das actions alinhadas, sem literal desatualizado
- [ ] AC6 — as duas direções falsificadas, com contra-braço demonstrado
- [ ] AC7 — `trackfw doctor` na raiz **não** reporta `scaffold-divergent` para
      `.github/workflows/trackfw-validate.yml` · **é a medição que fecha o issue**
- [ ] AC8 — `bash scripts/check-ci-workflow-pin-parity.sh` sai 0 com os dois braços verificados
- [ ] nomes de jobs em `.github/workflows/trackfw-validate.yml` inalterados
- [ ] `go build ./...`, `make test` e `make quality` verdes

⚠️ **Não se apoie em `governance-go-install` nem em `governance-install-script` como evidência de
nada:** ambos são exit-0 por construção enquanto o `governance_mode: lenient` vigorar (issue #387).

**Comandos de validação:**
```bash
go build ./...
make test
bash scripts/check-ci-workflow-pin-parity.sh
./bin/trackfw doctor 2>&1 | grep -i "scaffold-divergent" && echo "FAIL: AC7 nao fechou" || echo "OK AC7"
make quality
```

---

## Contexto

REQ: `docs/req/REQ-2026-09-17-gerador-aplica-o-template-de-consumidor-ao-proprio-produtor-e-o-doctor-prescreve-desfazer-a-correcao.md`
Issue: #376 · Causa separada descoberta na Wave 0: #387

## Legenda de status

⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado
