---
status: done
date: 2026-08-18
req: "docs/req/REQ-2026-08-17-doctor-detecta-artefato-em-disco-ausente-do-manifesto-apos-janela-de-gravacao-parcial.md"
adr: "docs/adr/ADR-2026-08-18-ordem-de-persistencia-inverte-para-manifesto-antes-dos-artefatos.md"
squad: "apolo-tf, hades-tf"
---

# Roadmap: `doctor` detecta artefato fora do manifesto, e a ordem de persistência inverte

> Created: 2026-08-18 | Status: done

> 🔴 **Entrega parcial deliberada.** A Wave 1 (inversão da ordem) foi para PR sozinha porque é
> autocontida e impede casos novos desde já. **A REQ NÃO está fechada:** o `doctor` (AC1–AC4) e a
> barreira ainda não existem, e instalações que já estão no estado ruim seguem sem detecção.

## Context

REQ: `docs/req/REQ-2026-08-17-doctor-detecta-artefato-em-disco-ausente-do-manifesto-apos-janela-de-gravacao-parcial.md`
ADR: `docs/adr/ADR-2026-08-18-ordem-de-persistencia-inverte-para-manifesto-antes-dos-artefatos.md`

Origem: bug de KG no CMDB — 12 arquivos em disco, 10 no manifesto, e o `agents update --force`
recusando com `unmanaged artifact`. O comportamento estava certo; o **estado** é que não deveria
existir.

Duas frentes, e a **ordem importa**: a inversão da frente 2 impede casos novos, mas **não** conserta
instalações que já estão no estado ruim. O `doctor` é o que revela essas.

## Acceptance Criteria
- [x] AC1 — Detecta **arquivo em disco ausente do manifesto** e o distingue de **arquivo modificado à mão**.
      ✅ **Refechado em 2026-08-19 pelo ML-2C, com três classes.** Histórico: desmarcado pela barreira do ML-3A — Existe um terceiro estado —
      `!Registered && StateModified` — que **nenhum `case` do `ClassifyDoctor` cobre**, e o comando
      responde `no mismatches found`. É exatamente o estado que faz o `agents install` recusar com
      `unmanaged artifact`, ou seja, o sintoma do CMDB que originou esta REQ. Fecha no ML-2C.
- [x] AC2 — A saída **nomeia o remédio**, com comando pronto para copiar.
- [x] AC3 — Paridade nos 3 CLIs, com **gate comparando saídas reais** — não por leitura de fonte.
- [x] AC4 — Cenário P4 reproduzindo a janela: artefato em disco sem registro, e prova de que acusa.
- [x] AC5 — Não-regressão: `update` **continua recusando** bytes unmanaged mesmo com `--force`.
      Evidência (verificada por mim, não pelo executor): `manager_test.go:149-152`, `legacy_test.go:120-124`,
      `npm/tests/agents-skills.test.js:79,178` — todos verdes no `make quality` desta auditoria.
- [x] AC6 — Decisão sobre a janela registrada em ADR. ✅ **feito** — `ADR-2026-08-18`, inverter a ordem.
- [x] AC7 — Inversão implementada, com rollback preservado em erro normal. Evidência: ML-1A.
- [x] AC8 — `make quality` verde **e CI verde**.
      Evidência (PR #190, run 32266062707): `go`, `node`, `python (3.10)`, `python (3.12)`,
      `package-smoke`, `windows-integrations-resolve`, `governance` e — o que importava —
      **`parity` em Linux, 5m15s, pass**. É o job onde esta série já quebrou por diferença de
      plataforma invisível no macOS.

## 🔴 Riscos que valem para todos os MLs

1. **Falso-positivo é o risco dominante do `doctor`.** Acusar artefato legítimo treina o usuário a
   ignorar a saída. A comparação é **por conteúdo contra o template do catálogo**, não por presença
   de arquivo.
2. **A frente 2 é o caminho de escrita de TODO `install`/`update`.** Qualquer regressão afeta tudo.
3. **Fixture com manifesto de fato incompleto**, nunca mock — é o estado que se quer detectar.
4. **`make quality` verde localmente não fecha AC** — o AC8 exige CI. Já errei isso nesta série.
5. **AC3 não se fecha com teste por stack.** Exige gate comparando as três saídas reais; foi
   exatamente a lacuna que virou ML corretivo nas duas REQs anteriores.

---

## Wave 1 — Inversão da ordem (impede casos novos)

### ML-1A — Persistir manifesto antes dos artefatos
**Status:** ✅ Concluído · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`)
**Arquivos:** `internal/integrations/manager.go` (`mutate`) + espelhos Node/Python, testes dos 3.

**Ação:** trocar a ordem dos dois laços de `mutate` — persistir os manifestos **antes** de escrever
os bytes. Ver o ADR para o raciocínio: cada escrita já é atômica, a janela é só de ordem, e a
direção invertida é auto-reparável (`StateNotInstalled`) em vez de exigir humano (`unmanaged`).

**Decisão sobre `uninstall` (registrada com justificativa, conforme pedido no handoff):**
`uninstall` foi deliberadamente **NÃO invertido** — mantém a ordem pré-existente (bytes removidos
primeiro, manifesto persistido depois). Regra geral que decide os dois casos: persistir o lado que
torna o manifesto um **superset** do disco. Para install/update isso é manifesto-primeiro (uma
interrupção deixa o manifesto declarando um artefato ainda ausente → `StateNotInstalled`,
auto-reparável). Para uninstall, inverter da mesma forma (remover a entrada do manifesto antes de
remover os bytes) produziria a direção **ruim**: uma interrupção deixaria um arquivo íntegro em
disco, com conteúdo que ainda bate com o template do catálogo, mas **sem nenhum registro no
manifesto** — resolve para `StateCurrent`/`managed=false`, um artefato órfão que parece legítimo e
que nada detecta ou repara automaticamente. É exatamente a direção "disco à frente do manifesto"
que o ADR existe para eliminar. Comentário equivalente está no código dos 3 CLIs para não ser
"simetrizado" por engano depois.

**Critérios de aceite:**
- [x] Interrupção simulada entre as fases deixa **manifesto à frente**, e `install`/`update` repara sozinho.
- [x] **Rollback preservado**: erro normal no meio do lote restaura arquivos **e** manifestos.
- [x] Não-regressão: `install`, `update` e `uninstall` inalterados no caminho feliz, nos 3 CLIs.
- [x] Cenário P4 com baseline e detecção.
- [x] `make quality` verde.

---

### Auditoria do ML-1A — a assimetria do ADR, provada nos dois sentidos

Medida por mim em projeto real e descartável, **não por leitura**:

```
manifesto a frente (direcao nova)     agents install         -> REPARA sozinho
disco a frente + deriva (direcao ANTIGA, o caso do CMDB)
                                      agents install         -> RECUSA: "is modified; use force"
                                      agents install --force -> exige decisao humana
```

É exatamente o que o ADR afirmou. A primeira tentativa da minha auditoria falhou por **erro meu**
(usei `--install-missing`, que é flag do `trackfw update`, não do `agents update`), e a segunda não
reproduziu o caso ruim porque faltava a **deriva de conteúdo** — sem ela o `install` adota o arquivo.
O caso real do CMDB exige disco-à-frente **somado** a conteúdo que deixou de bater com o template.

**Decisão sobre o `uninstall`: melhor do que eu pedi.** Eu pedi que decidisse e justificasse; ele
derivou a **regra geral** que decide os dois casos — *persistir o lado que torna o manifesto um
superset do disco*. Para install/update isso é manifesto-primeiro; para uninstall, inverter
produziria a direção **ruim** (arquivo íntegro, sem registro, parecendo legítimo e sem reparo
automático). O comentário está no código dos 3 CLIs para não ser "simetrizado" por engano depois.

**Bug pré-existente corrigido de passagem:** o laço de rollback do Python **não** engolia erro por
item, ao contrário de Go e Node. Um restore falhando abortava antes de restaurar o resto — inclusive
o manifesto. Agora espelha os outros dois. Era exatamente o risco 2 do handoff: quebrar o rollback
trocaria uma falha rara de interrupção por uma falha comum de erro.

`make quality` exit 0 · 131 cenários · `validate` exit 0.

---

## Wave 2 — `doctor` (revela o estado ruim já existente)

> ⚠️ **Wave 2 fatiada em 2 lotes menores** (2026-08-18, a pedido de KG): janela semanal a 98%.
> Cada lote é autocontido e commitado assim que auditado, para nada ficar parado no meio.

### ML-2A — Detecção + comando `doctor` nos 3 CLIs
**Status:** ✅ Concluído · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`)
**Dependência:** ML-1A — a inversão muda o estado que o `doctor` vai encontrar.
**Escopo enxuto:** função de classificação + superfície do comando + testes unitários nos 3.
**Fora deste lote:** gate de paridade e cenário P4 — vão para o ML-2B.

**Ação:** detectar **arquivo cujo conteúdo bate com o template do catálogo e que está ausente do
manifesto** — isso não é adulteração, é escrita não registrada, e o remédio é diferente. Distinguir
de **arquivo modificado à mão**, que continua sendo o caso de `install --force`.

**Critérios de aceite:**
- [x] As duas classes são distinguidas e têm remédios diferentes; não podem ser fundidas.
- [x] A saída nomeia o remédio com comando pronto para copiar.
- [x] **Não acusa** artefato legítimo — risco 1.
- [x] `make quality` verde.

### ML-2B — Gate de paridade + cenário P4
**Status:** ✅ Concluído · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`)
**Dependência:** ML-2A.
**Arquivos:** `scripts/check-doctor-parity.sh` (novo), `Makefile` (alvo `parity`),
`scripts/check-gates-falsify.sh`, `docs/cli-parity.md` (seção nova, **nomeando o gate**).

**Critérios de aceite:**
- [x] Gate comparando as **três saídas reais** — teste por stack **não** fecha o AC3.
- [x] Gate cobre **as duas superfícies**: relatório de texto **e** `--json`.
- [x] Cenário P4 reproduzindo a janela: artefato em disco sem registro, e prova de que acusa.
- [x] `docs/cli-parity.md` ganha seção do `doctor` **nomeando o gate** que a protege.
- [x] `make quality` verde.

**Restrições duras do fixture** (cada uma já custou ciclo nesta série):
1. `HOME` **redirecionado** para o temp — o `doctor` varre escopo global; sem isso o gate lê o
   `~/.trackfw` real e a saída vira dependente de máquina.
2. Os dois estados construídos por **install-e-mutar**, nunca por bytes de template hardcoded
   (hardcode apodrece e fica vacuoso em silêncio — o modo de falha do Cenário 58).
3. **Identidade fixada** explicitamente: `identity.Load` decide os destinos; e identidade ausente
   faz os 3 errarem igual, fechando o AC3 **vacuamente**.
4. Edição do manifesto por `python3`, **nunca `sed -i`** (BSD vs GNU — foi a classe da falha de CI).

---

### Auditoria do ML-2A — aprovada

```
baseline limpo        "no mismatches found"        — sem falso-positivo
arquivo alheio        "no mismatches found"        — o que nao e nosso nao e acusado
architect sem registro [unregistered-write] remedio: install --force
                       "adopts it — content already matches ... only the manifest entry is missing"
backend hand-edit      [hand-modified]      remedio: install --force
                       "overwrites it ... you will lose the hand edit"
make quality exit 0 · validate exit 0
```

As duas classes têm **redação de remédio diferente**, não só rótulo: uma adota, a outra **avisa da
perda**. Era o ponto do lote — fundi-las faria o `doctor` aconselhar errado.

**Correção que o advisor do agente pegou antes de escrever código, e vale registrar:** a
classificação precisa usar `Registered` (existe **alguma** entrada para o destino) e não `Managed`
(entrada pertencente **àquela** claim). Sem isso, destino registrado sob outra claim seria reportado
como "escrita não registrada" — exatamente o falso-positivo dominante que o comando existe para
evitar.

---

### Auditoria do ML-2B — aprovada, com dois defeitos reais achados pelo próprio gate

Não auditei por leitura. **Reverti a correção do produto e exigi que o gate ficasse vermelho:**

```
findings := []DoctorFinding{}  ->  var findings []DoctorFinding    (delta de literal único)
GO_BIN=bin/trackfw scripts/check-doctor-parity.sh  ->  EXIT=1, 6 FAIL
    doctor-parity/baseline-clean-json/go-vs-node
    doctor-parity/alien-file-not-flagged-json/go-vs-node   (e os pares go-vs-py)
restaurado  ->  "All check-doctor-parity.sh scenarios passed."
```

O gate **não é vacuoso** — pega a divergência real, no par certo, na superfície certa.

**Dois defeitos de produto que só um gate de três saídas revelaria:**
1. `--json` com zero findings: Go emitia `null` (slice nil no `encoding/json`); Node e Python
   emitiam `[]`. Corrigido **no Go**, que era o divergente.
2. Relatório de texto: Go deixava linha em branco ao final; Node e Python já removiam.

Nenhum dos dois apareceria em teste por stack — cada runtime concordaria consigo mesmo. É
exatamente a lacuna que o AC3 existe para fechar, e a terceira REQ seguida em que ela se materializa
como defeito real, não como formalidade.

**Escopo ampliado pelo executor, e bem:** cenário (e) "registrado sob outra claim". Os 4 cenários
que especifiquei **não** distinguiam `Registered` de `Managed` — o gate passaria com o defeito
presente. O Cenário 71 sabota exatamente esse literal e prova que agora fica vermelho.

`make quality` exit 0 · 367 OK · 0 FAIL · 132 cenários · `validate` exit 0.

> 🔴 **Qualificação retroativa (2026-08-19).** A auditoria do ML-2A acima segue válida no que
> mediu, mas seus quatro cenários **nunca alcançaram** o estado `!Registered && StateModified`.
> Aprovar o ML-2A não provou o AC1 — ver a barreira do ML-3A e o ML-2C.

## Wave 3 — Barreira

### ML-3A — `hades-tf`: revisão da inversão e do diagnóstico
**Status:** ✅ Concluído · **Agente:** `hades-tf` (`subagent_type: hades-tf`)
**Escreve:** `docs/seguranca/2026-08-18-revisao-do-doctor-e-da-inversao.md`

**Ações:** a inversão mexe no caminho de escrita de tudo — avaliar se abre caminho para o produto
sobrescrever bytes que não escreveu, ou para o manifesto declarar como gerenciado algo que não é.
Avaliar se o `doctor` pode ser induzido a chamar de "escrita não registrada" um artefato adulterado
de fato — o que rebaixaria adulteração a acidente. **Veredito explícito; bloquear é saída legítima.**

---

### ML-3A — auditoria da barreira: ressalva **aceita**, e ela reprova o AC1

Veredito do `hades-tf`: **APROVADO COM RESSALVAS** — `docs/seguranca/2026-08-18-revisao-do-doctor-e-da-inversao.md`.
Nota de vault: `vault/notes/doctor-classifydoctor-silences-tampering-when-manifest-entry-removed-2026-08-19.md`.

**Confirmei a ressalva por leitura direta, não pelo relatório:**
`inspectResolved` (`internal/integrations/manager.go:638-645`) — com `!managed`, conteúdo diferente
do desejado e fora de `LegacyHashes`, o estado é `StateModified`. E `ClassifyDoctor`
(`internal/integrations/doctor.go:81-95`) só tem `case` para `!Registered && StateCurrent` e para
`Managed && StateModified`. O terceiro estado **cai fora do `switch` em silêncio**.

**O argumento que decide, e que a barreira não usou:** esse é precisamente o estado que faz o
`preflight` recusar com `unmanaged artifact`. Ou seja, o `doctor` responde **"no mismatches found"
para o usuário cujo `agents install` acabou de recusar** — um diagnóstico que contradiz a ferramenta
que ele existe para diagnosticar. Esse foi o sintoma do CMDB que originou a REQ. Não é feature
incompleta; é o AC1 não fechado.

**Não aceito a forma binária da ressalva.** Reportar todo `!Registered && StateModified` como
adulteração seria a acusação falsa que o risco 1 nomeia. A saída é uma **terceira classe** que
declara a ambiguidade honestamente — ver ML-2C.

**Volume verificado antes de decidir** (era o que definia se cabia num ML ou exigia REQ nova):
`RunDoctor` varre todo destino do catálogo, mas destino é **arquivo nomeado** (`.claude/agents/<slug>.md`),
não diretório. Só dispara para quem tem arquivo de mesmo nome de outra origem — que é exatamente a
colisão que se quer revelar. Volume limitado; a classe é segura.

**Observação não-bloqueante aceita:** a auto-cura que o ADR-2026-08-18 promete vale para instalação
nova interrompida, **não** para update de artefato já existente — nesse caso as duas ordens convergem
no mesmo `StateModified`. Registrada como **Emenda 1** no ADR (que é `Accepted`, portanto emendado,
nunca reescrito).

**Aprovado sem ressalva:** a inversão da ordem (sem caminho para sobrescrever bytes alheios, rollback
íntegro, assimetria do `uninstall` correta), o `doctor` não escrever nada, e o gate do ML-2B (`HOME`
inline por invocação, sem `export` global; `pwd -P` cobre o symlink de `$TMPDIR` no macOS).

---

## Wave 4 — Corretiva da barreira

### ML-2C — Terceira classe: conteúdo desconhecido em destino do catálogo
**Status:** 🔄 Em andamento · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`)
**Dependência:** ML-3A. **Fecha o AC1.**
**Arquivos:** `internal/integrations/doctor.go`, `npm/src/integrations/doctor.js`,
`pypi/trackfw/integrations/doctor.py` (+ testes dos 3), `scripts/check-doctor-parity.sh`,
`scripts/check-gates-falsify.sh`, `docs/cli-parity.md`.

**Ação:** terceira classe para `!Registered && State == StateModified`. **Não** é
`unregistered-write` (remédio errado: "adota, já bate") nem `hand-modified` (acusa adulteração sem
base). O remédio precisa **nomear a recusa**: `agents install` vai recusar este destino com
`unmanaged artifact`; se o arquivo é seu, remova-o; se é do trackfw e derivou, `install --force`.
Isso converte o achado de acusação em **explicação** — e é o que dissolve a objeção de falso-positivo.

**Critérios de aceite:**
- [x] Três classes distintas nos 3 CLIs, com remédios distintos; nenhuma pode ser fundida.
- [x] O remédio da classe nova **nomeia a recusa** `unmanaged artifact`.
- [x] Cenário (f) no `check-doctor-parity.sh`, **texto e `--json`**, comparando as 3 saídas reais.
- [x] Cenário P4 irmão do 71: sabota o `case` novo e prova que o gate fica vermelho — a classe
      **não pode voltar ao silêncio** sem gate reprovar.
- [x] Seção do `doctor` no `docs/cli-parity.md` atualizada.
- [x] `make quality` verde.

---

### Auditoria do ML-2C — aprovada, com **duas** sabotagens exigidas

Auditei por medição, não por leitura. Uma sabotagem só provaria metade:

```
1) discriminante:  !Registered  ->  !Managed        (delta de literal único)
   gate -> EXIT=1, 7 FAIL. Pega no cenário (f):
   "registered-under-different-claim-content-drifted/go: vacuity guard: stdout missing
    'no mismatches found'; stdout: ... 1 unknown-content"     <- FALSO-POSITIVO detectado

2) regressao ao silencio:  case false && !Registered && StateModified
   gate -> EXIT=1, 7 FAIL. Pega no cenário (d):
   "unknown-content-never-installed/go: vacuity guard: stdout missing '[unknown-content]';
    stdout: no mismatches found"                              <- SILÊNCIO detectado

restaurado -> "All check-doctor-parity.sh scenarios passed."
```

As duas direções do defeito ficam cobertas: **acusar demais** e **calar**. O cenário (f) é o que
prova que a classe se apoia em `Registered` e não em `Managed` — o executor o acrescentou por conta
própria, e tinha razão: o cenário (e) sozinho nunca alcança esse `case`, porque seu `State` fica
`current`.

**Cenário (d) reinterpretado, não duplicado.** Ele já montava exatamente o estado novo; só a
expectativa mudou, de "fica em silêncio" para `[unknown-content]`. Está correto — o que mudou foi o
veredito do produto sobre aquele estado, não o estado. Apagá-lo teria perdido cobertura.

**Verifiquei a veracidade do remédio**, que era o ponto delicado: `planArtifactWrite`
(`manager.go`) com `exists && !owned` e `force` de fato substitui os bytes. O texto promete o que o
produto cumpre.

`make quality` exit 0 · 371 OK · 0 FAIL · 133 cenários · `validate` exit 0.

**Pendências não-bloqueantes registradas pelo executor:** `npm/src/integrations/doctor.js` aparece
como binário no `git diff` por dois NUL literais **pré-existentes** em `.join('\0')` — cosmético,
fora do escopo, cabe numa limpeza futura.

## Notas
- **Fora de escopo, declarado:** WAL/journal cross-file — rejeitado no ADR por desproporção.
- **Fora de escopo:** afrouxar o `preflight`; ele recusar bytes desconhecidos é correto.
- Commits e branch são exclusivos do `trackfw_architect`.
