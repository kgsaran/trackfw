# `ClassifyDoctor` fica silencioso para `!Registered && State == StateModified` — inclusive no próprio cenário do REQ, sem adversário

**Data:** 2026-08-19 · **Achado por:** Hades (revisão de segurança, ML-3A)
**Parecer completo:** `docs/seguranca/2026-08-18-revisao-do-doctor-e-da-inversao.md`

## O problema

`ClassifyDoctor` (`internal/integrations/doctor.go:68-100`, espelhado em `npm/src/integrations/
doctor.js` e `pypi/trackfw/integrations/doctor.py`) só tem dois `case`:

```go
case !inspection.Registered && inspection.State == StateCurrent:   // unregistered-write
case inspection.Managed && inspection.State == StateModified:      // hand-modified
```

Não existe `case` para `!Registered && State == StateModified`. Um artefato nesse estado cai no
`default` implícito e **não gera nenhum finding** — `doctor` reporta "no mismatches found".

## Por que isso não precisa de adversário

O comentário em `doctor.go:47-51` documenta, de propósito, que "conteúdo que não bate com o
catálogo e não tem entrada no manifesto não é do trackfw e nunca é reportado" — regra correta para
um arquivo que o trackfw *nunca* geriu (evita o falso-positivo dominante do roadmap). Mas o mesmo
estado é alcançável por um caminho inteiramente acidental, que é literalmente o cenário nomeado
pelo REQ ("artefato em disco ausente do manifesto após janela de gravação parcial"):

1. Uma interrupção comum (crash/`SIGKILL`/falta de energia — os mesmos eventos que o
   ADR-2026-08-18 já assume como não-tratáveis) deixa um artefato escrito pelo próprio trackfw sem
   entrada no manifesto.
2. O catálogo evolui depois disso (`CatalogVersion` novo, bytes de template novos — evento
   rotineiro). O conteúdo órfão deixa de bater com `desired`.
3. `LegacyHashes` (`internal/integrations/legacy.go`) é uma lista curada manualmente, só para os
   itens herdados do harness pessoal pré-migração — a maioria dos itens×alvos×escopos do catálogo
   não tem nenhuma entrada lá (confirmado por leitura: `claude/cli/project/agents/architect` não
   tem entrada; só a variante `global` do mesmo item tem). O hash órfão não bate com `desired` nem
   com `LegacyHashes`.
4. `inspectResolved` (`manager.go:639-645`) resolve isso como `StateModified`, `Registered=false` —
   e nenhum `case` do `switch` cobre a combinação.

Um artefato genuinamente do trackfw, órfão por interrupção comum, torna-se indistinguível de "nunca
existiu" assim que o catálogo evolui por cima dele. É o próprio AC1 falhando dentro do escopo que o
REQ declara — sem hipótese adversarial nenhuma.

## Medição (não é inferência de leitura)

Fixture descartável, `agents install` real, depois: adulterar conteúdo (gatilho mais simples para
reproduzir "conteúdo que não é nem `desired` nem `LegacyHashes`" sem precisar rodar duas versões
reais do catálogo — `ClassifyDoctor` não distingue a causa, só o resultado) e zerar
`manifest["artifacts"]`. `doctor --json` volta a `[]`: `"no mismatches found -- disk matches the
manifest for every catalog-managed artifact."` A leitura de `legacy.go` é a evidência do caminho de
produção sem edição manual (deriva de catálogo sobre item sem cobertura de legado).

Nota adicional: a mesma combinação também é alcançável por um agente induzido (ou adversário) que
adultera o conteúdo e apaga a entrada do manifesto — mas isso já é o caso que o
ADR-2026-08-12 aceita como não-prevenível; não é a base do achado, só reforça a prioridade do
conserto.

## Resolvido — ML-2C (2026-08-19)

Implementado por `apolo-tf`: terceira classe `DoctorUnknownContent` / `unknown-content` /
`UNKNOWN_CONTENT` (nomes por CLI) no `case !Registered && StateModified` dos 3 CLIs
(`internal/integrations/doctor.go`, `npm/src/integrations/doctor.js`,
`pypi/trackfw/integrations/doctor.py`). Remédio nomeia literalmente `unmanaged artifact` e
apresenta os dois ramos (arquivo alheio vs. artefato órfão do trackfw), em vez de escolher um
lado — a ambiguidade descrita acima neste achado não pôde ser resolvida por hash sozinho, então a
saída passou a declará-la honestamente.

**Cenário (d) do gate (`scripts/check-doctor-parity.sh`) foi reinterpretado, não apagado ou
duplicado.** Antes desta correção, (d) construía exatamente este estado (conteúdo alheio no
destino real do catálogo, nunca instalado) e afirmava `no mismatches found` — era, sem saber, o
teste que travava o bug em produção. O fixture não mudou; só a expectativa: agora espera
`[unknown-content]`. Se um agente futuro ler `docs/cli-parity.md` ou este vault e encontrar
referência à prosa antiga de (d) ("prova que o que não é do trackfw nunca é acusado"), essa prosa
descreve um comportamento **retirado deliberadamente**, não uma regressão a restaurar.

Foi adicionado um cenário novo, **(f)**, com claim retargetada + um byte anexado ao conteúdo
(`Registered=true, Managed=false, State=modified`) — é o único fixture do gate capaz de provar que
o discriminante correto da nova classe é `!Registered`, não `!Managed`: o cenário (e) já existente
não alcança esse `case` porque seu `State` fica `current`. Cenário 72 de
`scripts/check-gates-falsify.sh` sabota esse discriminante e usa (f) para provar vermelho,
espelhando o Cenário 71 para `unregistered-write`.

## Direção de correção (não implementada — decisão de produto, fora do escopo desta role)

Terceira classe de finding para `!Registered && State == StateModified` em destino catalog-conhecido
(o destino é sempre o caminho exato de um item específico do catálogo, não uma varredura genérica de
diretório — então "sem entrada" não é ambíguo da mesma forma que seria para um arquivo qualquer).
Remédio deve recomendar inspeção manual, nunca "adote automaticamente" — o sinal hash-only não
permite provar a causa (deriva de catálogo, escrita interrompida, ou edição manual de um arquivo
legitimamente independente do usuário no mesmo caminho).
