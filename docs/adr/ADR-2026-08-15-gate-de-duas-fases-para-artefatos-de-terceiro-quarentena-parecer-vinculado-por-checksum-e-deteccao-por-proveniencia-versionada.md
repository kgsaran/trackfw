---
status: Accepted
date: 2026-08-15
author: "Zeus (Arquiteto)"
---

# ADR: Gate de duas fases para artefatos de terceiro — quarentena, parecer vinculado por checksum e detecção por proveniência versionada

> Date: 2026-08-15 | Status: Accepted

REQ: `docs/req/REQ-2026-08-15-instalacao-de-skills-de-terceiro-via-url-para-agentes-especialistas.md`
Roadmap: `docs/roadmaps/wip/ROADMAP-2026-08-15-instalacao-de-skills-de-terceiro-via-url-para-agentes-especialistas.md`
Parecer de segurança (ML-0A, `hades-tf`): `docs/seguranca/2026-08-15-skills-de-terceiro-via-url.md`

## Context

O `trackfw` vai permitir instalar **artefato de terceiro** apontado por URL (skill, agent — e,
potencialmente, plugin) para compor conhecimento nos agentes especialistas. Diferente de instalar
uma dependência de código, o conteúdo baixado vira **instrução de sistema** carregada por um agente
que tem `Bash`/`Edit`/`Write`: pode tentar se auto-conceder autoridade de Git, desligar o gate de
governança, ou exfiltrar segredos. É a mesma classe de risco que já motivou o `credential-guard` e
o `git-branch-guard` neste projeto.

KG definiu a restrição central: a revisão do `hades-tf` **não é um evento único de desenho, é um
gate de runtime recorrente**, disparado a cada instalação, valendo tanto para o comando explícito
do `trackfw` quanto para o pedido em linguagem natural dentro da sessão.

Isso colide com um fato estrutural: **um comando de CLI não invoca um subagente.** O comando tem
que parar no meio do fluxo. Esse é o problema que este ADR resolve.

Duas doutrinas pré-existentes do projeto condicionam tudo abaixo:

- `ADR-2026-08-12-nao-ha-prevencao-contra-agente-induzido...`: **não existe prevenção técnica, no
  escopo do trackfw, contra um agente induzido com escrita irrestrita ao workspace**; o esforço vai
  para detecção ancorada no `HEAD` do git. Este ADR não tenta contornar isso — assume e constrói em
  cima.
- `ADR-2026-07-25-escopo-de-instalacao-selecionavel-para-agents-e-skills` D1: escopo default de
  instalação é `global`. **D4 abaixo abre uma exceção escopada a isso**, com justificativa.

## Decision

### D1 — Superfície do comando: subcomando `third-party` em `skills` e `agents`, não comando novo

O gate ganha um subcomando dentro do ciclo de vida já existente
(`newIntegrationsLifecycleCmd`, `internal/commands/integrations_flags.go`), disponível tanto em
`trackfw skills` quanto em `trackfw agents`, em **duas fases explícitas**:

- **Fase 1 — `trackfw <skills|agents> third-party fetch <url>`**: baixa, valida, coloca em
  quarentena, emite artefato de revisão. **Nunca instala.**
- **Fase 2 — `trackfw <skills|agents> third-party install --checksum <sha256>`**: consuma a
  instalação a partir da quarentena, exigindo prova de aprovação (D8c).

Confirmação obrigatória antes de gravar (AC1): a fase 2 exibe o conteúdo e o destino resolvido; em
modo não-interativo recusa por padrão, exigindo `--yes-i-trust-this-source` explícito. Sugestão de
agentes-alvo é permitida (por palavra-chave/nome); **decisão silenciosa é proibida** — o usuário
confirma a lista de agentes.

### D2 — A restrição "só o orquestrador invoca" é guardrail declarado, não controle

A env var de sessão (`TRACKFW_ORCHESTRATOR_SESSION`) é implementada, mas **documentada como
guardrail**: filtra o uso acidental, e não resiste a quem já tem `Bash` (basta exportá-la antes de
chamar o binário). Nenhuma mensagem de erro, doc ou teste pode apresentá-la como prevenção.

A defesa real é **detecção**, conforme o ADR-2026-08-12: nova regra em `trackfw validate`,
`thirdparty_artifact_has_provenance`, seguindo a convenção de nomes já existente
(`branch_has_wip_roadmap`, `credential_guard_hook_resolvable`), **bidirecional**:

1. destino gerenciado com origem de terceiro **sem** entrada em `thirdparty-provenance.json` →
   violação `error`;
2. entrada de proveniência cujo `checksum_sha256` **não bate** com o SHA-256 do conteúdo instalado
   → violação `error` (adulteração pós-instalação ou instalação fora do fluxo).

**Limite herdado e declarado:** a detecção só alcança o que foi commitado. Instalação que nunca é
commitada não é pega por esta regra — é pega pela revisão de PR, segunda camada que o ADR-2026-08-12
já assume para o resto do projeto.

### D3 — Critério objetivo de recusa: 6 marcadores, normalização de 5 passos, e o fence decidido

Recusa quando o conteúdo baixado apresenta, **como heading** de qualquer nível `#`–`######`,
qualquer um de: `Git authority`, `Mode lock`, `Governance prerequisite`, `Reporting boundary`,
`Scope boundary`, `Dispatch contract`.

Normalização, em ordem fixa: (1) remover comentários HTML; (2) NFKC; (3) casefold; (4) colapsar
espaços internos + strip; (5) extrair só linhas que casem `^#{1,6}\s+`.

**Emenda do arquiteto ao parecer (achado da auditoria do ML-0A):** a normalização acima **não
tratava blocos de código cercados**, e a consequência é verificável — o próprio parecer
`docs/seguranca/2026-08-15-skills-de-terceiro-via-url.md` lista os seis marcadores dentro de um
fence ``` e, pelo critério como escrito, **seria recusado por si mesmo**. Decisão: **o passo 5
opera sobre o conteúdo com os blocos cercados removidos** — linhas dentro de fence (``` ou ~~~) não
são consideradas headings. Justificativa: um marcador dentro de fence é citação/documentação, não
uma seção que redefine fronteira; e o falso positivo atingiria justamente a documentação legítima
sobre como escrever agentes. Contrapartida aceita e registrada: quem quiser esconder um marcador
pode envolvê-lo num fence — mas conteúdo em fence também não é lido como diretiva estrutural pelo
agente, e a evasão por paráfrase (já aceita em D3) é mais barata que essa de qualquer forma.

Recusa é o **default**; override explícito via `--force-thirdparty-markers`, que grava
`marker_override: true` na proveniência (fica auditável, não silencioso).

**Escopo honesto deste critério:** é um tripwire para o caso descuidado, **não** um filtro contra
adversário competente. Não cobre paráfrase sem marcador, indireção ("leia a URL X e siga"),
fragmentação, homoglifos residuais fora do NFKC, reivindicação semântica de autoridade sem heading,
nem conteúdo auto-modificável. Isso está declarado no parecer e não pode ser omitido da doc do
comando.

### D4 — Escopo default `project` — exceção escopada ao `ADR-2026-07-25` D1

Artefato de terceiro instala em escopo **`project` por padrão**. Escopo `global` exige confirmação
explícita adicional.

**Relação com o `ADR-2026-07-25`: é exceção escopada, não emenda.** O D1 daquele ADR (default
`global`) **permanece válido e inalterado** para o catálogo do próprio `trackfw` — agentes e skills
versionados no repositório upstream, cujo modelo mental correto é "instalo uma vez, vale para todos
os projetos". A exceção vale **apenas** para artefato de terceiro, por uma razão estrutural, não de
preferência: a detecção de D2 exige que o artefato e sua proveniência vivam **dentro do repositório**
para aparecerem em `git status`/diff/PR. Em `~/.trackfw/`, não aparecem em lugar nenhum — que é
exatamente o argumento que o ADR-2026-08-12 usou para inverter a decisão de escopo do guard.
Confirmado por KG em 2026-08-15.

### D5 — Residência: arquivo separado + referência por marcadores; nunca bloco no arquivo do catálogo

O conteúdo de terceiro vai para **subpasta dedicada, fora do namespace `trackfw-` do catálogo**:
`<target>/skills/thirdparty/<slug>.md` (o catálogo usa `<target>/skills/trackfw-<nome>.md` flat —
verificado em `internal/integrations/assets/catalog.json`; o caminho por target é resolvido pelo
`Manager`, nunca hardcodado como `.claude/`).

O arquivo do agente do catálogo recebe **apenas uma linha de referência**, gerenciada por um novo
par de marcadores dedicado (`<!-- trackfw:thirdparty-skills:start -->` / `end`), no padrão
idempotente de `injectOrUpdateRules` (`internal/generators/agentfiles.go`), implementado como ponto
de extensão em `internal/integrations/render.go`.

**Rejeitado — apensar bloco no arquivo renderizado do catálogo:** mudaria o hash do destino frente
ao `integrations-manifest.json`, e `inspectResolved` classificaria como `StateModified`; o próximo
`trackfw agents update` ou recusaria, ou treinaria o usuário a usar `--force` rotineiramente — o
oposto do que o hard-error de `Modified` existe para prevenir.

**Rejeitado — novo item de catálogo:** conteúdo de terceiro é dinâmico (URL escolhida em runtime);
entraria em `internal/integrations/assets/catalog.json`, que é fonte única espelhada e travada por
`scripts/check-integration-assets.sh`, exigindo exceção ad-hoc no gate de paridade.

O artefato de terceiro ganha **claim própria** no manifest — hash e proveniência isolados, sem
contaminar o hash do artefato do catálogo.

### D6 — Proveniência: JSON estruturado, fatal-on-failure, sem auto-update

`.trackfw/thirdparty-provenance.json`, `schema_version: 1`, objeto `entries` chaveado por destino,
com `url`, `checksum_sha256`, `installed_at`, `approved_by`, `review_reference`, `scope`,
`marker_override`. Irmão de `.trackfw/integrations-manifest.json`, escopo `project` (coerente com D4).

- Hash: **SHA-256 hex dos bytes brutos baixados**, antes de qualquer normalização, reusando
  `contentHash` de `internal/integrations/manager.go`.
- **Falha de escrita da proveniência é fatal** — sem registro, não há instalação. Isto diverge
  deliberadamente de `.trackfw-log`/`appendTransitionLog`, que é best-effort por desenho, e é por
  isso que o formato **não** é o log append-only: a regra de D2 precisa de lookup por destino.
- **Drift a montante:** `trackfw validate` **nunca faz fetch de rede** — compara o checksum
  registrado contra o conteúdo instalado localmente. Querer a versão nova da URL significa rodar a
  instalação de novo, passando pelo gate inteiro. Não existe `update` silencioso de artefato de
  terceiro.

### D7 — Política de rede

HTTPS-only (`url.Scheme == "https"` validado antes do primeiro `Get`; a URL vem inteira do usuário,
diferente de `plugins.go`, que monta a própria); timeout `30 * time.Second` (herdado de
`internal/plugins/plugins.go`); teto de **2 MiB** (`2 << 20`) — não os 50 MiB de plugins, porque é
texto; leitura via `io.LimitReader(body, max+1)` + comparação `> max` (padrão herdado); **máximo 3
redirects**, revalidando o esquema **a cada hop** (o default do `net/http` são 10 sem revalidar) via
`CheckRedirect` em client próprio — não reusar o `httpClient` compartilhado de `plugins.go`, para
não alterar o comportamento de plugins; **recusa** (não warn) se o `Content-Type` não for
`text/plain`, `text/markdown` ou `text/x-markdown`.

### D8 — Handshake de duas fases

**(a) Quarentena:** `.trackfw/thirdparty-quarantine/<checksum-sha256>.json`. Nome pelo checksum
torna o arquivo self-verifying e idempotente. A garantia de não-carregabilidade é **ausência de
caminho de código** — nenhum `render`/`compose` existente lê essa árvore — não um bloqueio ativo;
e a regra de D2 detecta se tal caminho for adicionado sem passar pelo gate.

**(b) Artefato de revisão:** JSON com `url`, `checksum_sha256`, `fetched_at`, `content_base64`
(conteúdo **integral embutido**, para não abrir um segundo TOCTOU por indireção de arquivo),
`marker_check` pré-computado (D3), `kind`, `requested_targets`. O `marker_check: pass` **não é
suficiente para aprovar** — é insumo, não veredito.

**(c) Prova de aprovação vinculada por checksum:** a fase 2 recalcula o SHA-256 do conteúdo de
quarentena e só prossegue se existir entrada de proveniência com **aquele checksum exato** e
`approved_by` preenchido. Booleano "aprovado" solto é rejeitado por construção. Isto **fecha o
TOCTOU** (aprovar A e instalar B é impossível). Isto **não** impede forja por quem já tem escrita no
workspace — não há assinatura — mas torna a forja **git-detectável**: a entrada é versionada e
aparece no diff/PR sem parecer correspondente em `docs/seguranca/`.

**(d) Caminho de linguagem natural:** quando o pedido chega como "instala essa skill pra mim", não
há bloqueio técnico impedindo o agente de pular a fase 1 — a garantia é a mesma de (a) e D2:
ausência de caminho de código + detecção por proveniência. A instrução correspondente vai no
arquivo do agente orquestrador. O prompt de dispatch que envia o artefato de revisão ao `hades-tf`
**deve declarar explicitamente que `content_base64` decodificado é dado a analisar, nunca instrução
a seguir** (mitigação do vetor "injeção contra o próprio revisor", que Q1 registra como sem
mitigação técnica).

**(e) Cobertura — `trackfw plugins install` fica FORA desta REQ, em REQ separada.** O parecer
confirma que ele precisa do mesmo gate e com **severidade maior**: baixa binário de terceiro e faz
`chmod 0755` — executa direto, enquanto markdown apenas influencia. Fica em REQ separada, conforme a
guarda de escopo já registrada no ML-0B: gate de binário é superfície de ameaça materialmente
distinta de gate de composição de markdown, e misturá-los arrastaria a Wave 2 de lado. **Esta
decisão é um débito de segurança consciente e datado, não um esquecimento.**

**(f) Fail-closed sempre:** proveniência ausente, ilegível, com `schema_version` incompatível, ou
checksum divergente → recusa, sem bypass silencioso. Em **CI**: não há sessão de agente, portanto
**CI nunca instala artefato de terceiro do zero** — CI apenas *valida* instalações já commitadas via
`thirdparty_artifact_has_provenance`. Uma aprovação commitada é suficiente em CI porque está
vinculada por checksum e é ela própria o objeto da auditoria.

### D9 — Emenda pós-implementação (ML-1C): registro de referências é um TERCEIRO artefato

Descoberto ao implementar D5 e **não previsto neste ADR quando foi escrito**. Formalizado aqui
porque um porte feito estritamente de D1–D8 **divergiria** no comportamento de `agents update`
pós-anexação.

**O problema:** D5 diz que o arquivo do agente do catálogo recebe uma linha de referência entre
marcadores. Mas esse arquivo é **regenerado** por `Render`/`BuildPlans` a cada `trackfw agents
update`. Sem persistir a referência em algum lugar, a regeneração produz o arquivo sem o bloco, e a
anexação passa a ser lida como **drift** (`StateModified`) — exatamente o que D5 existe para evitar.

**A decisão:**

1. Terceiro schema versionado: **`.trackfw/thirdparty-references.json`**, `schema_version: 1`,
   `entries` chaveado por `<targetID>/<agentItemID>`, cada valor uma lista de referências
   (`slug`, caminho do arquivo de terceiro, etc.).
2. `BuildPlans` **deve** reaplicar as referências persistidas **depois** de `Render`, via
   `ApplyThirdPartyReferences`. Não pode viver dentro de `Render` porque `Render` não conhece o
   root do projeto.
3. O acoplamento é **opt-in por `PlanRequest.ProjectRoot`**: caller que não seta o campo recebe
   saída byte-idêntica à anterior. Isso preserva todo o comportamento existente do catálogo.

**Por que é um arquivo separado da proveniência (D6), e não um campo dentro dela** — a pergunta foi
levantada na auditoria e a resposta é o discriminante do desenho:

- proveniência é chaveada pelo **destino do arquivo de terceiro** e é **o objeto cuja integridade a
  regra `thirdparty_artifact_has_provenance` confere**;
- referências são chaveadas por **target + item de agente** e são lidas no **caminho de render**.

Fundir os dois obrigaria o render a **inverter o índice** (varrer toda a proveniência procurando
entradas aplicadas ao target X) a cada renderização de agente, e — mais grave — faria anexar ou
desanexar uma referência **mutar o registro de auditoria por um motivo que não é de auditoria**.
Não se escreve na coisa cuja integridade se está conferindo. Dois arquivos, duas chaves, dois
leitores.

### D7-bis — Emenda: o limite de redirect é "recusa o 3º", não "segue 3"

O texto de D7 dizia "máximo 3 redirects". **Impreciso.** Medido empiricamente pelo arquiteto
contra o `Fetch` do Go, com servidor de teste encadeado:

```
hops=1 → SEGUIU   hops=2 → SEGUIU   hops=3 → RECUSOU   hops=4 → RECUSOU
```

Causa: a semântica do `CheckRedirect` do `net/http` — o `via` conta **requests já completados**,
não redirects já seguidos; na primeira chamada `len(via)` já vale 1. Portanto `maxRedirects = 3`
**segue no máximo 2 hops**.

**Decisão: o comportamento fica como está (segue 2, recusa o 3º) e é o TEXTO que se corrige.**
Não há diferença de segurança entre 2 e 3 hops, e mudar o número exigiria alterar três CLIs já
implementados e verificados. O que não pode ficar é a divergência entre spec e código, que
convidaria um "conserto" futuro que quebraria a paridade.

Os três CLIs reproduzem **a mesma contagem**: Go via `CheckRedirect`, Node via `requestsCompleted`
(`npm/src/thirdparty/fetch.js`), Python via laço manual de redirect — o `max_redirections` do
`urllib` conta de forma diferente e foi deliberadamente abandonado.

⚠️ **Débito menor registrado:** o teste Go `TestFetch_RefusesFourthRedirect` tem nome que descreve
mal o próprio comportamento (recusa o 3º, não o 4º). Renomear no ML-3A.

Notas de vault: `vault/notes/node-https-redirect-checkredirect-off-by-one-2026-08-15.md`.

### D10 — Imprecisões deste ADR encontradas na implementação (registradas, não contornadas)

1. **Escopo do `--apply-to`.** D5/D8 nunca disseram em que escopo o artefato do agente do catálogo
   precisa estar, e D4 dá ao terceiro um default (`project`) **diferente** do default do catálogo
   (`global`, `ADR-2026-07-25` D1). Decisão: exigir que o agente esteja instalado, **owned** e não
   modificado à mão **no MESMO escopo** da skill, falhando com a remediação exata. Justificativa: um
   caminho de skill relativo ao projeto injetado num arquivo de agente de escopo global estaria
   quebrado para todos os outros projetos que compartilham aquele arquivo do home.
   ✅ **Consequência de UX DECIDIDA por KG em 2026-08-15:** no caminho 100% default (catálogo em
   `global`, terceiro em `project`), `--apply-to` **recusa** com mensagem de remediação — e fica
   assim. A recusa explícita com o comando exato de correção é preferível a instalar num escopo
   que produziria um link quebrado para os demais projetos.
2. **Quem escreve a entrada de proveniência.** `VerifyApproval` exige entrada preexistente, mas
   **nenhum comando a escreve** — o aprovador (`hades-tf`) grava direto no JSON versionado,
   **chaveado pelo destino RESOLVIDO**. Esse acoplamento era implícito e agora é normativo: sem ele,
   a Wave 2 não tem como portar o lado da escrita do handshake.
4. **`resolve_third_party_skill_destination` retorna 3 valores no Python, não 2.** O
   `PlannedArtifact` do Go não tem campo `Representation`, mas o `IntegrationManager.inspect` do
   Python acessa `plan["representation"]` incondicionalmente — o dict de plano do Python carrega
   mais campos que a struct do Go. Divergência de forma interna, **não** de comportamento
   observável; a paridade de saída é preservada.
5. **Entradas de proveniência precisam de canonicalização de chaves na escrita (Python).** Como
   quem escreve a entrada é um chamador externo (o aprovador, D10.2) e não este pacote,
   `write_provenance` reordena os campos para a ordem da struct do Go antes de serializar — sem
   isso, a paridade byte-a-byte do JSON quebraria.
3. **`requested_targets` de D8b é ambíguo** — não distingue target de CLI (onde o arquivo cai) de
   item de agente do catálogo (quem recebe a referência). Por isso o CLI separa `--targets` de
   `--apply-to`.

### D11 — Como o `validate` identifica um destino "de origem de terceiro": campo `origin` na `Claim`

Decisão do arquiteto, tomada antes do ML-3A para não travá-lo no meio (KG confirmou o
comportamento de recusa de D10.1 em 2026-08-15; esta decisão é complementar).

`internal/integrations/manifest.go` ganha um campo na `Claim`:

```go
type Claim struct {
    Target  string   `json:"target"`
    Surface string   `json:"surface"`
    Scope   string   `json:"scope"`
    Kind    ItemKind `json:"kind"`
    Item    string   `json:"item"`
    Origin  string   `json:"origin,omitempty"` // "" = catálogo (default); "thirdparty" = artefato de terceiro
}
```

**Por que não as alternativas:**

- ❌ **Sniffing de caminho (`/thirdparty/`)** — heurística que quebra silenciosamente se o layout
  de D5 mudar, e que não distingue um diretório homônimo criado pelo usuário.
- ❌ **Usar a própria proveniência como índice** — **circular e logicamente impossível** para o ramo
  (i) da regra. Esse ramo detecta *"destino de terceiro **sem** entrada de proveniência"*; se a
  proveniência for o índice do que é de terceiro, um artefato ausente dela nunca é classificado
  como de terceiro e a violação **nunca dispara**. O ramo (i) deixaria de existir na prática.
- ✅ **Campo na `Claim`** — o manifest já é o registro canônico de posse por destino, é escrito no
  mesmo ato da instalação, e o zero-value (`""`) mantém **retrocompatibilidade total** com
  manifests existentes, que continuam lidos como catálogo. Sem migração.

**Limite honesto, declarado (coerente com o ADR-2026-08-12):** essa detecção cobre o que foi
instalado **pelo fluxo** (e portanto registrado no manifest). Um arquivo escrito à mão em
`<target>/skills/thirdparty/` não está no manifest, não é `Managed`, e **não é pego por esta
regra** — é pego pelo `git status`/diff/PR, a segunda camada que o projeto já assume. Não afirmar,
em doc ou mensagem, que a regra detecta instalação arbitrária fora do fluxo.

## Consequences

**Positivas**
- Não existe caminho de código que instale artefato de terceiro sem parecer prévio vinculado ao
  conteúdo exato — a propriedade que a emenda de KG exigia.
- O TOCTOU (aprovar um conteúdo, instalar outro) fica tecnicamente fechado, não apenas mitigado.
- A escolha de escopo `project` (D4) não é preferência: é o que torna a detecção possível.
- Artefatos do catálogo permanecem byte-limpos frente ao manifest (D5) — sem erosão do hard-error
  de `Modified`, sem treinar o usuário a usar `--force`.

**Negativas / riscos aceitos**
- **O critério de D3 é um tripwire, não um filtro.** Paráfrase e indireção passam. Assumido e
  documentado; qualquer doc que sugira o contrário é bug.
- **A prova de aprovação não é criptográfica.** Quem já tem escrita no workspace forja a entrada de
  proveniência. Coerente com o ADR-2026-08-12: a resposta é detecção via git, não prevenção.
- **`trackfw plugins install` segue sem gate** até a REQ separada de D8(e) ser executada — o vetor
  de maior severidade é o que fica aberto por mais tempo. Débito consciente.
- Duas fases significam mais fricção que `install <url>` de uma vez. É o custo do gate.
- Exceção de escopo (D4) cria duas regras de default coexistindo — catálogo `global`, terceiro
  `project`. Mitigado por declarar a relação explicitamente aqui e no `docs/cli-parity.md`.
