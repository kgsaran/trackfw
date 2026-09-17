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
**Status:** ✅ Concluído (auditado por Zeus em 2026-09-17 — AC5 reprovado e movido para o ML-1B) · **Papel:** `apolo-tf`
Cobre AC1 a AC8.

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
- [x] AC1 — `IsProducerGoMod(dir)` + `BuildDiscoverGitHubActionsWorkflowContent(isProducer bool)` — fixture com `module github.com/kgsaran/trackfw` no `go.mod` gera template com `go build`, fixture de consumidor gera template com `go install`
- [x] AC2 — os três consumidores (`discover.go:276`, `update.go:1976`, `scaffold_doctor.go:269`) passam `IsProducerGoMod(root)` para o builder; testes `TestRunScaffoldDoctor_DiscoverWorkflow_ProducerContext_Clean` e `TestRunScaffoldDoctor_DiscoverWorkflow_WrongContext_Divergent` cobrem o doctor; `TestWriteCIWorkflow_ProducerContext` cobre o braço produtor do discover; `TestRefreshDiscoverWorkflow_ProducerContext` cobre o braço produtor do update
- [x] AC3/AC4 — consumer: `checkout@v7`/`setup-go@v7`, `go-version: "1.25"`; producer: `go-version-file: go.mod`; gate `check_action_pins` detecta regressão para @v<7; job-id-collision gate atualizado para esperar 2 ocorrências
- [x] AC6 — `TestBuildDiscoverGitHubActionsWorkflowContent_Templates` falsifica nas duas direções com contra-braço: ambas as assinaturas diferem, consumer contém `run: go install`, producer não contém `run: go install`
- [x] AC7 — `trackfw doctor` na raiz **não** reporta `scaffold-divergent` **para o caminho
      `.github/workflows/trackfw-validate.yml`** · **medição que fecha o issue**.
      ⚠️ **Redação corrigida na auditoria:** o comando de validação escrito originalmente no roadmap
      era `doctor | grep -i "scaffold-divergent"` **sem filtro de caminho**, e esse comando **casa** —
      sobra 1 achado, para `trackfw-gate.yml` (tratado no ML-1B). A medição que de fato fecha o AC7 é
      com o caminho nomeado. Sem esta correção, um leitor futuro reexecuta o comando do roadmap, vê o
      acerto e conclui que o AC7 regrediu.
      Medido por Zeus: `doctor` → 1 `scaffold-divergent`, nenhum para `trackfw-validate.yml`.
- [x] AC8 — `bash scripts/check-ci-workflow-pin-parity.sh` sai 0 com 16 cenários; ambos os braços (`consumer-version-pin`, `producer-no-go-install`) verificados
- [x] nomes de jobs em `.github/workflows/trackfw-validate.yml` inalterados (`governance-go-install`)
- [ ] AC5 — 🔴 **REPROVADO na auditoria; movido para o ML-1B.** O ML-1A satisfez a redação anterior
      ("não usa `go install …@v`") e deixou a classe aberta: `.github/workflows/trackfw-gate.yml`
      obtém o binário por `install.sh` do release, e o required check `governance-install-script`
      valida o binário **publicado**, não o código do PR. O `doctor` continua reportando
      `scaffold-divergent` para esse arquivo, com `remedy: trackfw update`. O AC5 foi reescrito na REQ
      para nomear a **procedência do binário** em vez do comando, e o gate que ele exige — varredura
      dos workflows **commitados** — não existe. Ver ML-1B.
- [x] `go build ./...` RC=0; `make test` todos os 15 pacotes verdes; `bash scripts/check-ci-workflow-pin-parity.sh` RC=0 (16 cenários); `make quality` RC=0 (segunda execução, 2026-09-17)
- [x] `check-ci-workflow-job-id-collision.sh` atualizado para esperar 2 ocorrências de `governance-go-install:` em `scaffold_doctor.go` — os dois braços do template (produtor e consumidor) declaram o mesmo job-id porque ele é contrato de `required_status_checks`. O escopo original não previa esse gate porque `BuildDiscoverGitHubActionsWorkflowContent` tinha apenas um braço; ao acrescentar o segundo, a contagem passou de 1 para 2 e o gate precisou ser ajustado.
- [x] `.github/workflows/trackfw-gate.yml` comparado contra `buildGitHubActionsWorkflowContent(cfg)`: única divergência é `TRACKFW_VERSION: "8.0.0"` vs `"8.0.1"` (pin de versão desatualizado após v8.0.1, mecanismo `install.sh` inalterado). Causa distinta de #376 — reportado ao arquiteto como achado separado.

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


---

### ML-1B — o segundo builder não distingue produtor de consumidor, e o required check valida o binário publicado
**Status:** ⬜ Pendente · **Papel:** `apolo-tf`
Cobre o **AC5 reescrito**. Acrescentado pela auditoria do ML-1A em 2026-09-17.

🔴 **Por que está nesta REQ e não numa nova.** A Regra Dura de Causa Raiz diz que *"parser vs.
renderizador vs. escrita são superfícies diferentes do mesmo defeito"*, e que *"é superfície
diferente"* **não** justifica REQ nova. O ML-1A corrigiu
`BuildDiscoverGitHubActionsWorkflowContent`. O **segundo builder**,
`buildGitHubActionsWorkflowContent` (`internal/generators/scaffold.go:~1975`), tem o **mesmo
defeito e nenhum braço de produtor**: emite incondicionalmente

```
TRACKFW_VERSION: "<version>"
curl -sSfL https://github.com/kgsaran/trackfw/releases/latest/download/install.sh | sh
```

Mesmo mecanismo, mesma consequência, outro sítio → **novo ML na REQ vigente, no mesmo PR.**

**O que eu medi na auditoria, e é pior do que o relatório do ML-1A sugeria:**

1. `.github/workflows/trackfw-gate.yml` **deste repositório** roda o job `governance-install-script`,
   que é **required**. Ele instala o binário do **release publicado** e só então roda
   `trackfw validate`. 🔴 **Um PR que quebre a lógica de `trackfw validate` passa nesse check**, porque
   o verificador nunca executa o código do PR — é exatamente a frase que o comentário do
   `trackfw-validate.yml` já usava para justificar o braço de produtor.
2. `./bin/trackfw doctor` ainda reporta **1 `scaffold-divergent`** — agora para `trackfw-gate.yml` —
   com `remedy: trackfw update`. Aplicar essa remedy mantém o produtor no binário publicado. **É o
   sintoma do #376, intacto, no segundo builder.**
3. O `check-ci-workflow-pin-parity.sh` **não pega isto** e não é falha dele: ele verifica o
   **template que o builder emite**, nunca o **arquivo commitado**. Foi por essa fresta que o sítio
   sobreviveu à Wave 0 e ao ML-1A.

⚠️ **Composição com o #387, que torna o achado mais grave do que parece:** os dois checks de
governança obrigatórios são `governance-go-install` e `governance-install-script`. O #387 mostra que
ambos saem 0 por construção (`governance_mode: lenient`). Este ML mostra que o segundo, além disso,
**valida o binário errado**. Os dois defeitos são independentes e se somam: corrigir o `lenient`
sozinho deixa o check verde validando artefato publicado.


🔴 **Ação 0 — meça ANTES de editar o workflow, e pare se der vermelho.**
`governance-install-script` é **required** e hoje passa porque valida o binário **publicado**. Depois
deste ML ele passa a rodar o binário **do PR**. Os dois podem não concordar sobre esta árvore.
Primeira coisa, antes de qualquer edição:

```bash
go build -o /tmp/tfw ./cmd/trackfw && /tmp/tfw validate ; echo "RC=$?"
```

Se `RC≠0`, **não regenere o workflow**: escreva `docs/roadmaps/.trackfw-attention.json` e pare. Este
ML transformaria um required check verde em vermelho num PR aberto, e isso é decisão do arquiteto,
não sua. (Nota: o `governance_mode: lenient` deste repositório deve fazer o `validate` sair 0 — mas
"deve" não é medição.)

🔴 **Trava de contrato — o escopo negativo da REQ proíbe mexer em `D = R = W`.**
O conjunto `required-status-checks` foi fechado em 2026-09-16 com 8 nomes. O `W` (nomes derivados dos
workflows) depende **do job-id e também do `name:` do workflow**. Regenerar `trackfw-gate.yml`
mantendo só o job-id não basta: o `name: trackfw-gate` de topo também entra na derivação.
Depois de regenerar, **execute e mostre**:

```bash
make check-required-checks ; echo "RC=$?"
```

Se reprovar, o `W` mudou e **todo PR do repositório fica pendente para sempre**. É o discriminante
barato deste ML — não o pule.

**Achado resolvido de passagem (não abra issue):** `TRACKFW_VERSION: "8.0.0"` no
`trackfw-gate.yml` está defasado em relação à v8.0.1. É pin velho, e desaparece sozinho quando o
arquivo for regenerado a partir do builder. Registre no relatório como resolvido de passagem.
**Arquivos afetados:**
- `internal/generators/scaffold.go` (builder `buildGitHubActionsWorkflowContent`, ~l. 1975)
- `.github/workflows/trackfw-gate.yml` — regenerado a partir do braço de produtor
- `scripts/` — **gate novo** para o AC5 (ver Ação 3)
- `scripts/check-ci-workflow-pin-parity.sh` — braços do segundo builder
- testes em `internal/generators/`

**Ações:**
1. `buildGitHubActionsWorkflowContent` passa a receber o contexto, pelo **mesmo** discriminante já
   entregue no ML-1A (`IsProducerGoMod`) — não introduza um segundo critério. Braço de produtor
   compila do fonte; braço de consumidor mantém `install.sh` **inalterado**.
2. Regenerar e commitar `.github/workflows/trackfw-gate.yml`. 🔴 O job-id
   `governance-install-script` permanece **byte a byte idêntico** — é contrato com o
   `required_status_checks`, e renomear deixa todo PR pendente para sempre. O
   `check-ci-workflow-job-id-collision.sh` vai precisar do mesmo ajuste de contagem que o ML-1A fez
   para o outro builder.
3. 🔴 **Gate novo, sobre os arquivos commitados** — é o que o AC5 exige e o que não existe hoje.
   Varre `.github/workflows/*.yml` **deste repositório** e reprova qualquer workflow que obtenha o
   binário do trackfw de procedência que não seja o código do PR: `go install …@v`, `install.sh` de
   release, download de artefato, imagem pré-construída. Enumere por `find`/`git ls-files`, nunca por
   lista fixa — lista fixa fica vazia quando alguém acrescenta um workflow.
   Inclua **contra-braço**: um fixture com o `trackfw-gate.yml` **antigo** tem de reprovar. Gate que
   não reprova o defeito conhecido não mede nada.
4. Verificar se `trackfw doctor` deixa de reportar `scaffold-divergent` para `trackfw-gate.yml` —
   pelo braço correto, não porque a comparação foi afrouxada.

**Critérios de aceite:**
- [ ] `buildGitHubActionsWorkflowContent` tem os dois braços, com o discriminante do ML-1A
- [ ] `.github/workflows/trackfw-gate.yml` compila do fonte do PR; job-id inalterado
- [ ] Gate novo do AC5 existe, varre os workflows commitados e **reprova o `trackfw-gate.yml` antigo**
      num fixture (contra-braço demonstrado)
- [ ] `./bin/trackfw doctor` reporta **0 `scaffold-divergent`** na raiz deste repositório
- [ ] Ação 0 medida e reportada: RC do `validate` com o binário compilado do PR
- [ ] `make check-required-checks` RC=0 depois de regenerar o `trackfw-gate.yml` (D=R=W intacto)
- [ ] Reconciliação: uma frase por teste novo, dizendo qual conclusão deste ML ele afirma
- [ ] `go build ./...`, `make test`, `make quality` verdes, com exit code medido sem pipe

**Comandos de validação:**
```bash
go build ./... ; echo "RC=$?"
make test ; echo "RC=$?"
make build && ./bin/trackfw doctor | grep -c "scaffold-divergent"   # tem de ser 0
bash scripts/check-ci-workflow-pin-parity.sh ; echo "RC=$?"
bash scripts/check-ci-workflow-job-id-collision.sh ; echo "RC=$?"
make quality ; echo "RC=$?"
```
## Contexto

REQ: `docs/req/REQ-2026-09-17-gerador-aplica-o-template-de-consumidor-ao-proprio-produtor-e-o-doctor-prescreve-desfazer-a-correcao.md`
Issue: #376 · Causa separada descoberta na Wave 0: #387

## Legenda de status

⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado
