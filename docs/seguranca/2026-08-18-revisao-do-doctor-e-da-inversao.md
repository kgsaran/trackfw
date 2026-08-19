---
status: done
date: 2026-08-19
autor: "Hades (Security Reviewer)"
roadmap: "docs/roadmaps/wip/ROADMAP-2026-08-18-doctor-detecta-artefato-fora-do-manifesto-e-inverte-a-ordem-de-persistencia.md"
adr: "docs/adr/ADR-2026-08-18-ordem-de-persistencia-inverte-para-manifesto-antes-dos-artefatos.md"
req: "docs/req/REQ-2026-08-17-doctor-detecta-artefato-em-disco-ausente-do-manifesto-apos-janela-de-gravacao-parcial.md"
---

# Revisão de segurança — inversão da ordem de persistência + `trackfw doctor` (ML-3A)

## Veredito: **APROVADO COM RESSALVAS**

A ressalva (§1) é um buraco real na cobertura declarada do `doctor` (AC1) — não fecha por acidente
que é exatamente o cenário nomeado pelo REQ, sem precisar de nenhum adversário. Recomendo não
considerar o AC1 satisfeito até o `switch` de `ClassifyDoctor` cobrir o caso descrito, mas isso não
bloqueia a inversão de ordem (§2, aprovada) nem o gate (§4, aprovado) — ambos podem seguir. Ver
"Direção de correção sugerida" ao final; não implementei nada, é código de produto.

---

## 1. Ressalva — `ClassifyDoctor` fica em silêncio para o próprio cenário do REQ, sem precisar de adversário

**Severidade: alta.** Caminho de exploração medido por execução real, não inferido.

### O buraco no `switch`

`ClassifyDoctor` (`internal/integrations/doctor.go:68-100`, espelhado em `npm/src/integrations/
doctor.js` e `pypi/trackfw/integrations/doctor.py`) só tem dois `case`:

```go
case !inspection.Registered && inspection.State == StateCurrent:   // unregistered-write
case inspection.Managed && inspection.State == StateModified:      // hand-modified
```

Não há `case` para `!Registered && State == StateModified`. Esse estado cai no `default` implícito
do `switch` — nenhum finding é produzido.

### O caminho que não depende de adversário — é o próprio cenário do REQ

O REQ chama-se "doctor detecta artefato em disco ausente do manifesto **após janela de gravação
parcial**" — exatamente a janela que o ADR-2026-08-18 documenta e aceita como consequência
residual (nunca eliminada, só invertida de direção). O `doctor` existe para revelar instalações que
já ficaram nesse estado ruim antes da inversão (a própria motivação da Wave 2, roadmap linha 26-27).

Esse estado ruim não precisa ser "arquivo idêntico ao template atual" (o único caso que
`unregistered-write` cobre). Ele é igualmente alcançável assim, sem nenhuma ação maliciosa:

1. Uma interrupção (crash, `SIGKILL`, falta de energia — os mesmos eventos que o ADR já assume
   como não-tratáveis) deixa um artefato **escrito pelo próprio trackfw**, em disco, sem entrada no
   manifesto — o caso central do REQ.
2. Entre essa interrupção e a próxima execução do `doctor`, o **catálogo evolui** (novo
   `CatalogVersion`, novo conteúdo do template — evento rotineiro, não incidente). O conteúdo órfão
   em disco agora não bate mais com `desired` (o template atual).
3. `LegacyHashes` (`internal/integrations/legacy.go`) é uma lista **curada manualmente**, só para
   artefatos herdados do harness pessoal pré-migração — 10 entradas de escopo `global` e 12 do
   `codex`/`project`. Não cobre a maioria dos itens×alvos×escopos do catálogo (confirmado por
   leitura: o fixture usado abaixo, `claude/cli/project/agents/architect`, não tem nenhuma entrada
   em `legacyHashes` — só a variante `global` desse mesmo item tem). Então o hash órfão não bate com
   `desired` **nem** com nenhum `LegacyHashes`.
4. `inspectResolved` (`manager.go:639-645`, ramo não-gerenciado) resolve isso como `StateModified`.
   `Registered=false`. Nenhum `case` de `ClassifyDoctor` cobre essa combinação → **nenhum finding**.

Ou seja: um artefato genuinamente escrito pelo trackfw, órfão por uma interrupção comum, e que o
catálogo simplesmente evoluiu por cima dele, é indistinguível — para o `doctor` — de "nunca existiu"
e nunca aparece no relatório. Isso é o próprio AC1 ("detecta artefato em disco ausente do manifesto")
falhando dentro do escopo que o REQ declara, sem precisar de nenhuma hipótese adversarial.

### Medição (fixture descartável, `/private/tmp/.../scratchpad/doctor-bypass`)

O mecanismo do `switch` foi confirmado assim (o gatilho usado para produzir "conteúdo que não é nem
`desired` nem `LegacyHashes`" foi uma edição manual, por ser a forma mais simples de reproduzir sem
depender de duas versões reais do catálogo — mas `ClassifyDoctor` não distingue a *causa* do
descompasso, só o resultado; a análise acima em `legacy.go` é a evidência de que a mesma combinação
`Registered=false` + `State=Modified` é alcançável por deriva de catálogo, sem edição manual):

```
1. trackfw agents install --items architect --targets claude --scope project
2. trackfw doctor --json                          -> []                              (baseline limpo)
3. echo "MALICIOUS INJECTED INSTRUCTION" >> trackfw-architect.md
   trackfw doctor --json                          -> [{"finding":"hand-modified", ...}]  (detectado,
                                                        correto — prova que a detecção funciona
                                                        quando o manifesto está intacto)
4. python3: manifest["artifacts"] = {}             (remove a entrada; conteúdo do passo 3 mantido)
   trackfw doctor --json                          -> []
   trackfw doctor                                 -> "no mismatches found -- disk matches the manifest
                                                        for every catalog-managed artifact."
```

O passo 4 prova o `default` silencioso do `switch`. A causa concreta e mais provável em produção
para chegar nesse mesmo estado (`Registered=false` + `State=Modified`) não é alguém editando o
manifesto — é a combinação interrupção-comum + catálogo evoluindo, ambas rotineiras.

### Nota sobre o cenário adversarial (fora do escopo do ADR-2026-08-12, registrado por transparência)

A mesma combinação também é alcançável se um agente induzido (ou um adversário com a mesma permissão
de escrita) adulterar o conteúdo e apagar a entrada do manifesto — o passo 4 acima é literalmente
essa demonstração. Não computo isso como achado independente: o ADR-2026-08-12 já aceita, como
premissa que não deve ser reaberta aqui, que não há prevenção contra agente induzido com essa
permissão. Cito apenas para que fique registrado que a mesma lacuna no `switch` também remove sinal
nesse caso — o que reforça a prioridade do conserto, mas não muda o veredito por si só.

### Direção de correção sugerida (não implementada — decisão de produto)

Uma terceira classe de finding para `!Registered && State == StateModified` em destino
catalog-conhecido, com redação que **não afirma nem nega** adulteração (o hash-only signal não
permite diferenciar deriva de catálogo, escrita interrompida ou edição manual de um arquivo
legitimamente independente do usuário no mesmo caminho — risco 1 do roadmap continua valendo) — por
exemplo `"unclaimed-drift"`, remédio que recomenda inspeção manual do conteúdo antes de qualquer
`--force`, nunca "adote automaticamente". Isso não fecha o gap de forma criptográfica (fora de
proporção, mesma lógica do ADR-2026-08-18 ao rejeitar WAL), mas troca silêncio por sinal — que é a
promessa mínima de uma ferramenta de "detecção ancorada", e fecha o AC1 dentro do próprio escopo que
o REQ declara.

---

## 2. Inversão da ordem de persistência (`mutate`) — aprovada

Lido e medido via leitura de `internal/integrations/manager.go` (`mutate`, `planArtifactWrite`,
`persistManifests`, `inspectResolved`), sem necessidade de reproduzir com `SIGKILL` porque a lógica
decide o caso por leitura direta do código (comportamento determinístico, sem I/O assíncrono
envolvido).

### 2.1 — Não abre caminho para sobrescrever bytes que o trackfw não escreveu

Testei a hipótese central por leitura de `planArtifactWrite` (`manager.go:517-575`) e `preflight`
(`manager.go:375-416`): mesmo com o manifesto reivindicando posse de um destino antes do byte
existir, uma escrita subsequente **nunca** sobrescreve conteúdo divergente sem `--force` explícito —
`writeDesired = actualHash != desiredHash` só é decidido no momento da própria escrita, e o
`preflight` já barra `StateModified` sem `--force` tanto para `install` quanto para `update`
(`manager.go:388`, `:404`). O único caminho sem `--force` é quando `actualHash == desiredHash`
(adoção silenciosa de conteúdo idêntico ao que o trackfw escreveria de qualquer forma) — esse
comportamento é **pré-existente** à inversão (é a mesma regra de "adoção de legado" que já existia
antes do ADR-2026-08-18) e não amplia superfície.

### 2.2 — O manifesto pode declarar como gerenciado algo que o trackfw não escreveu?

Só no sentido já aceito e documentado no próprio ADR ("consequência aceita, declarada"): durante a
janela, o manifesto pode declarar um artefato que ainda não está em disco. `inspectResolved`
(`manager.go:614-647`) lê o arquivo antes de classificar — ausência vira `StateNotInstalled`, nunca
`StateCurrent` falso — então essa reivindicação adiantada não produz diagnóstico incorreto nem
escrita indevida enquanto o arquivo continua ausente.

### 2.3 — Assimetria do `uninstall` (deliberadamente não invertido)

Concordo com a regra geral registrada no roadmap ("persistir o lado que torna o manifesto um
superset do disco"). Verifiquei que o `uninstall` (`manager.go:285-308`, `applyUninstall` em
`:482-502`) continua removendo bytes antes de persistir o manifesto, e que essa é a única direção
que evita a órfã "arquivo íntegro em disco, sem registro, parecendo legítimo". Nenhuma janela nova
surge da assimetria: as duas operações, cada uma na sua direção, convergem para o mesmo objetivo
(nunca deixar "disco à frente do manifesto" persistente).

### 2.4 — Rollback ainda restaura arquivos e manifestos em erro normal?

Confirmado por leitura: `snapshots` (`manager.go:233-265`) é populado tanto para os destinos de
`active` quanto para todo arquivo em `manifests`, e o `defer` (`manager.go:268-279`) restaura os
dois conjuntos quando `retErr != nil` e `!committed`. Não há regressão aqui.

### 2.5 — Observação não bloqueante: o benefício da inversão é mais estreito que o texto do ADR sugere

O ADR (`ADR-2026-08-18...md`, tabela da seção "Dois fatos medidos") descreve o benefício em termos
de "registrado e ausente do disco" — que é exatamente o caso de uma instalação **nova**
interrompida. Para um **update de conteúdo de um artefato já gerenciado e já existente**, uma
interrupção entre a persistência do manifesto (que já grava o hash novo, otimista) e a escrita dos
bytes deixa o disco com o conteúdo **antigo** e o manifesto com o hash **novo** — `inspectResolved`
classifica isso como `StateModified` (linha 630: `actual != entry.Hash`), e `preflight` exige
`--force` tanto para `install` quanto para `update` nesse caso (`manager.go:404`), exigindo
intervenção humana — o mesmo desfecho que o ADR credita à ordem antiga. Medindo a ordem antiga para
o mesmo cenário (bytes primeiro): a interrupção entre bytes-novos-escritos e manifesto-ainda-com-
hash-antigo também produz `StateModified` (mesma linha, direção oposta). Ou seja, para o caso de
*update de artefato existente*, as duas ordens convergem no mesmo desfecho — a auto-cura descrita no
ADR se aplica de fato ao caso de *instalação nova* (arquivo ausente do disco), não a toda
interrupção. Isso não é uma regressão de segurança — apenas o texto do ADR generaliza um pouco além
do que a tabela realmente prova. Não bloqueia; registro para não ser lido como "toda interrupção
agora é auto-reparável", o que não é o caso.

---

## 3. `trackfw doctor` — demais pontos

### 3.1 — O `doctor` não escreve nada

Confirmado por leitura de `internal/integrations/doctor.go` (só `ClassifyDoctor`/`RunDoctor`/
`doctorPlansForScope`, nenhuma chamada de escrita) e `internal/commands/doctor.go` (`runDoctor` só
lê catálogo, manager e identidade; `printDoctorReport` só escreve em `cmd.OutOrStdout()`). Nenhum
`os.WriteFile`/`atomicWrite`/equivalente em nenhum dos dois arquivos, nem nos espelhos
`npm/src/{integrations,commands}/doctor.js` e `pypi/trackfw/{integrations,commands}/doctor.py`
(mesma varredura). Confirma o texto do `--help` do comando.

### 3.2 — Escopo global (`~/.trackfw`, `~/.claude`) e vazamento em `--json`

`RunDoctor` varre `project` e `global` (`doctor.go:145`) e os `destination` reportados são caminhos
absolutos reais do disco do próprio usuário, tanto no texto quanto no `--json`
(`DoctorFinding.Destination`, `doctor.go:41`). Não há vazamento para terceiros — a saída vai para o
stdout do próprio usuário que rodou o comando, nas mesmas condições de qualquer outro comando do
CLI que já imprime caminhos absolutos (`agents list --json`, por exemplo). Não é um achado novo
desta mudança.

### 3.3 — `Registered` vs `Managed` (já auditado no ML-2A) — confirmado correto

Reconferido: `ClassifyDoctor` usa `inspection.Registered` (existe **alguma** entrada, de qualquer
claim) para `unregistered-write`, não `Managed` (entrada desta claim específica) — evita o falso
positivo de reportar como "escrita não registrada" um destino que só está registrado sob outra claim.
Consistente com a auditoria já registrada no roadmap para o ML-2A.

---

## 4. Gate `scripts/check-doctor-parity.sh` e Cenário 71 — aprovados

Lido o script inteiro. Cada invocação real do CLI (`agents install`, `doctor`, `agents list`) passa
`HOME="$home"` explicitamente na mesma linha da chamada (`check-doctor-parity.sh:107-123,319`) —
não há `export HOME=` global no script que pudesse vazar para um comando esquecido sem o prefixo, e
não encontrei nenhuma chamada real do CLI sem esse prefixo. `project`/`home` são `mktemp -d` sob
`$TMPDIR`, resolvidos com `pwd -P` (`:95-96`) — cobre a armadilha de symlink do macOS já documentada
no vault (`vies-do-tmp-ao-medir-sandbox-de-agente-2026-08-12.md`) e na nota de retomada do
`agents-working-context.md`. Nenhum `git commit`/`git branch`/`git push` no script. Nada toca o
`$HOME` real do usuário nem o repositório do projeto.

---

## Resumo para o handoff

| Área | Veredito |
|---|---|
| Inversão da ordem de persistência (`mutate`) | Aprovado — nenhuma escrita de bytes não autorizados encontrada; rollback e assimetria do `uninstall` corretos. Observação não bloqueante sobre o alcance do benefício (§2.5). |
| `doctor` não escreve nada | Confirmado por leitura, nos 3 CLIs |
| `doctor` — escopo global / `--json` | Sem vazamento além do já existente em outros comandos |
| `doctor` — `Registered` vs `Managed` | Correto |
| Gate `check-doctor-parity.sh` / Cenário 71 | Aprovado — `HOME` sempre redirecionado, sem toque no ambiente real |
| **`ClassifyDoctor` silencia `!Registered && State == StateModified`** | **RESSALVA** — §1, mecanismo medido por execução; caminho de produção (deriva de catálogo sobre artefato órfão) confirmado por leitura de `legacy.go`; AC1 não fecha até haver `case` para esse estado |
