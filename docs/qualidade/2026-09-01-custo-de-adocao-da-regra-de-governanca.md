# Custo de adoção da regra de governança para PRs externos — ML-0A

> Autor: `hefesto-tf` | Data: 2026-09-01 | Escopo: parecer sobre a REQ
> `docs/req/REQ-2026-09-01-projeto-nao-publica-a-exigencia-de-governanca-para-prs-e-nao-tem-contributing.md`
> e o roadmap `docs/roadmaps/wip/ROADMAP-2026-09-01-publicar-a-exigencia-de-governanca-para-prs-no-contributing.md`.
> Nenhuma linha de `CONTRIBUTING.md` escrita — isso é Wave 1 (`atena-tf`). Nenhum código de produto
> tocado.

## Veredito resumido

**Publicar a regra é a decisão certa, mas a régua interna não serve como está para quem chega de
fora, e a resposta à falsificabilidade (AC5) é o achado principal deste ML: a regra é
parcialmente detectável, e a parte que não é detectável é justamente a que mais importa —
`required_status_checks` não está configurado em `main`, o que torna os dois jobs de governança do
CI informativos, não bloqueantes.**

---

## 1. Onde a régua deve cair — os quatro casos concretos

A exceção de trivialidade do `~/.claude/CLAUDE.md` §7 foi escrita para mantenedores com contexto
implícito (sabem quais scripts são gate, quais strings são grepadas por teste, o que é "lógica de
negócio" neste projeto específico). Testada contra os quatro casos, ela **não** transporta sem
ajuste — não porque a fronteira esteja errada, mas porque falta a instrução de **como aplicá-la**
sem o contexto que um mantenedor já tem de cabeça.

### Caso A — correção de uma linha num gate de `scripts/` (PRs #238/#240)

**Não é trivial, mesmo em uma linha.** `scripts/` aqui não é "config sem efeito em runtime" — é
lógica que decide pass/fail em CI. A REQ da própria Wave 0 documenta que dois desses gates chegaram
"inertes" e custaram um microlote corretivo cada. O erro visível aqui não é a régua (uma linha em
lógica de gate já cai fora da exceção de §7 por natureza, não por tamanho) — é que **contagem de
linha é a heurística que um contribuidor de fora vai aplicar por padrão**, porque é a heurística
universal fora deste projeto. O `CONTRIBUTING.md` precisa dizer isto explicitamente e cedo: *tamanho
do diff não decide trivialidade; a pergunta é se o arquivo participa de uma decisão de pass/fail
em algum gate, teste ou CI* — com `scripts/*.sh`, `scripts/*.ps1` e qualquer coisa sob
`.github/workflows/` citados como exemplo positivo de "nunca trivial".

Peso adicional (ver §4): esses dois PRs específicos foram, na prática, **portados pelos mantenedores
com governança própria** — o contribuidor não escreveu REQ nem roadmap, e o trabalho dele passou por
duas barreiras sem bloqueante. Isso não muda o veredito de que a categoria não é trivial; muda a
recomendação de **quanto rigor de processo** exigir dela (ver a proposta de nível intermediário no
§4).

### Caso B — typo em mensagem de erro visível ao usuário

**Não cabe limpo em nenhuma das duas categorias existentes.** Não é "typo/renomear variável local"
(isso é para identificadores internos) nem é "doc-only" (a string não é markdown nem comentário —
é parte do contrato de saída do programa em runtime). O projeto tem um caso documentado e real desse
exato risco:
`docs/req/REQ-2026-08-04-make-quality-falha-sob-locale-pt-br-teste-fixa-literal-em-ingles-no-violations-found.md`
— um teste que gruda no literal em inglês de uma mensagem. Uma correção de "typo" numa string dessas
pode silenciosamente quebrar um `grep` de teste ou gate em outro lugar da árvore, e o autor de um
typo fix não tem motivo para saber disso.

**Recomendação para o `CONTRIBUTING.md`:** tratar como uma terceira categoria, não subsumida em
"doc-only" nem em "typo puro" — uma correção de string visível ao usuário exige que o autor rode
`grep -rn "<texto antigo>"` na árvore (comando exato, incluído no PR) e cole o resultado (vazio ou
não) na descrição do PR. Se vazio, trivial sem REQ. Se não-vazio, precisa de REQ porque a mudança
tem efeito em runtime fora do arquivo editado.

### Caso C — port de um teste de um runtime para outro

**Não é trivial, e a razão é a mais séria dos quatro casos.** Este projeto usa portabilidade de
teste como *mecanismo* de paridade entre os 3 CLIs (regra dura em `CLAUDE.md` deste repositório). Um
teste mal portado não é neutro — ele pode parecer cobertura adicionada e na prática não testar nada
(o roadmap cita, nesta mesma sessão, "um `skipif` que anularia a verificação inteira" como um dos
achados que a cadeia pegou e o CI verde não pegaria). Um port de teste é, por definição, código que
afirma equivalência comportamental entre dois runtimes — isso é a própria "lógica de negócio" do
contrato de paridade do projeto, não boilerplate.

**Recomendação:** port de teste sempre exige REQ, mesmo que leve — porque o risco não é "quebrar
build", é "parecer que cobre e não cobrir" (o mesmo padrão do achado #3 abaixo).

### Caso D — correção de documentação que descreve comportamento errado

**O artefato é doc-only, mas a afirmação corrigida não é.** A regra de §7 ("doc-only") mede o
*arquivo tocado*, não o *tipo da afirmação*. Esta mesma sessão encontrou "uma afirmação factualmente
errada em dois documentos" — exatamente esta categoria. Um contribuidor externo corrigindo um doc
sobre comportamento de runtime pode estar tão errado quanto o doc original, e nada na regra atual
pede evidência.

**Recomendação:** manter a isenção de processo (sem REQ) para doc-only, mas exigir no
`CONTRIBUTING.md` que, quando a correção afirma um fato sobre comportamento (não estilo/gramática),
o PR inclua a evidência que a sustenta — comando rodado, saída colada, ou link para o teste que já
prova a afirmação. Isto é uma condição de PR bem formado, não de REQ: mais barato que criar uma REQ
para cada typo de doc, e resolve o risco real sem reintroduzir o peso do processo pesado.

### Síntese do item 1

A fronteira de §7 **não precisa mudar de posição** — precisa de uma camada de "como aplicar sem o
contexto que um mantenedor tem". Os quatro casos mostram quatro formas diferentes de falha por falta
desse contexto: heurística errada de tamanho (A), categoria ausente para strings de runtime (B),
subestimação do risco de "parecer cobertura" (C), e nenhuma exigência de evidência para afirmações
sobre comportamento em documentos (D). O `CONTRIBUTING.md` (Wave 1) precisa nomear os quatro casos
explicitamente — não é redundante com §7, é a tradução dele para quem não tem o histórico de
incidentes que produziu cada exceção.

---

## 2. Falsificabilidade da regra (AC5) — verificado, não presumido

### O que existe hoje

Rodei os comandos, não presumi:

```
.github/workflows/trackfw-validate.yml → job "governance": go build + `trackfw validate`
.github/workflows/trackfw-gate.yml     → job "governance": curl install.sh (binário release) + `trackfw validate`
```

Ambos rodam em `pull_request`. Confirmado num PR real e **aberto** (**#238**, autor externo
`lourivalgarciajunior`; correção: uma primeira leitura minha classificou este PR como mergeado —
`gh pr view 238 --json mergeCommit,mergedAt` retorna `null` nos dois campos e
`gh pr list --state merged --author lourivalgarciajunior` só lista o **#233**, não o #238; o
`17:19:04Z` que citei pertence ao #233, não ao #238) via `gh pr view 238 --json statusCheckRollup`:
os dois jobs `governance` aparecem com `"conclusion":"SUCCESS"`. Isso já é suficiente para o que
esta seção precisa provar — que a regra **é computada** e produz veredito correto em PR real de
fora — sem depender de o PR ter sido mergeado. (Precisão adicional: o rollup completo do #238 tem
duas falhas, `windows-full-suites` e `windows-defect-reproduction`, ambas `continue-on-error: true`
por desenho — ver `.github/workflows/quality.yml` — então não invalidam o merge nem o ponto sobre
`governance`; mas "todos os checks verdes" seria impreciso, e eu tinha escrito isso numa versão
anterior deste parágrafo.)

### O que falta — e é o achado principal

```bash
gh api repos/kgsaran/trackfw/branches/main/protection
gh api repos/kgsaran/trackfw/rules/branches/main
gh api repos/kgsaran/trackfw/rulesets
```

A primeira resposta **não tem a chave `required_status_checks`** (confirmado com
`--jq '.required_status_checks'`, saída vazia); `required_pull_request_reviews.required_approving_review_count`
é `0`; `enforce_admins.enabled` é `false`. As outras duas — checadas porque required status checks
também podem vir de **rulesets** (mecanismo separado da branch protection clássica, e a `quality.yml`
comenta explicitamente sobre required-status-checks travando PR, o que levantou a hipótese de haver
algo configurado por fora) — retornam `[]` nos dois casos: **nenhum ruleset existe no repositório.**
Não há required status check configurado por nenhum dos dois mecanismos do GitHub.

**Isso significa que nenhum dos dois jobs `governance` é um required check.** O CI computa o
veredito corretamente — mas nada no GitHub bloqueia o merge se ele vier vermelho. Um PR pode ser
mergeado por qualquer mantenedor com permissão de escrita mesmo com `trackfw validate` reprovando,
sem aprovação de review, sem status check obrigatório. A regra publicada no `CONTRIBUTING.md` teria,
tecnicamente, **exatamente o mesmo status que o `docs/cli-parity.md` já critica por escrito sobre
gates de terceiro**: existe, roda, produz um veredito correto — e nada consome esse veredito de
forma vinculante.

### Veredito

**Parcialmente detectável.** Detectável no sentido de "o veredito existe e está correto" — não é
"com nada", como a REQ cogitava como pior caso. Mas não detectável no sentido que a REQ precisa
(AC5: "a regra é falsificável" implica consequência, não só medição) — hoje a consequência de um PR
sem REQ ligada, ou com `trackfw validate` vermelho, é **zero** do lado do GitHub. Depende
inteiramente de um humano olhar o check antes de clicar em "Merge".

**Recomendação:** este achado é do escopo de configuração de repositório (branch protection), não de
código — e a REQ tem Negative Scope explícito ("não endurecer o `validate`... implementar bloqueio é
decisão que merece REQ própria"). Registro aqui para que o `CONTRIBUTING.md` **não afirme
falsificabilidade que não existe** — ou o próprio documento vira o próximo caso do padrão que ele
existe para corrigir. Recomendo ao arquiteto uma REQ dedicada para configurar
`required_status_checks` com os dois jobs `governance` (e possivelmente `go`/`node`/`python` da
Quality) — fora do escopo deste roadmap, mas é o item que fecha o buraco que este ML mediu.

### Achado secundário — `branch_has_wip_roadmap` enfraquece com a idade do projeto

Ainda sobre "quão sólida é a regra por trás do gate": `internal/validator/validator.go:2419`
(`validateBranchHasWIPRoadmap`) casa o slug da branch por **substring** contra os nomes de arquivo em
`wip/` **e** `done/`. Existe uma REQ aberta, sem roadmap, documentando exatamente isso:
`docs/req/REQ-2026-08-20-branch-has-wip-roadmap-casa-por-substring-num-corpus-de-done-que-so-cresce.md`
— medido lá que um slug curto como `fix/guard` casa com 11 roadmaps não relacionados só em `done/`,
e `done/` só cresce. Não é uma hipótese: é `status: Open`, sem roadmap, hoje.

Efeito prático para este ML: mesmo quando o gate roda e "passa", passar não garante que a branch
tem uma REQ **relacionada ao que ela de fato muda** — pode estar casando por coincidência lexical com
um roadmap antigo e não relacionado. Isto reforça o achado do item 3 abaixo: o gate técnico prova
"existe algo com REQ perto", não "existe a REQ certa".

---

## 3. Falsificação nas duas direções — a simétrica (REQ decorativa)

**Direta** (regra publicada, ninguém segue, ninguém percebe): coberta pelo achado do item 2 —
sem `required_status_checks`, essa direção já é possível hoje, tecnicamente, sem nenhuma má-fé.

**Simétrica — REQ seguida ao pé da letra e vazia de conteúdo:** verifiquei o que os dois gates
mecânicos (`validateWIPHasREQ`, `internal/validator/validator.go:1397`, e
`validateREQsHaveRoadmap`) realmente checam. Ambos delegam a `contentHasMarker`
(`internal/validator/validator.go:87`), que faz **apenas** `strings.Contains(content, marker)` sobre
o campo de frontmatter (ex.: `req: "..."`) — confirma que o **campo não está vazio**, nunca que o
conteúdo apontado é substantivo. Uma REQ com uma frase de Motivation e um Acceptance Criteria
genérico ("melhorar qualidade") satisfaz o gate tão bem quanto a REQ mais detalhada deste próprio
roadmap.

**Não existe hoje nenhum gate mecânico capaz de distinguir REQ real de REQ decorativa** — e
provavelmente não deveria existir um totalmente automático: "a REQ explica por quê, não só o quê"
não é uma propriedade sintática. Isso não é uma lacuna a fechar com mais regex; é o ponto onde a
regra de governança **precisa** de revisão humana, e o `CONTRIBUTING.md` deveria dizer isso
explicitamente em vez de implicar que "REQ presente" resolve o problema.

**Como um revisor distingue as duas, na prática** (heurísticas, não gate):

1. **`Files affected` do roadmap bate com o diff real do PR?** Divergência é o sinal mais barato de
   checar e o mais forte de "REQ escrita depois, para satisfazer o portão" — se o roadmap lista
   arquivos que o PR não toca, ou não lista arquivos que o PR toca, a REQ não estava guiando o
   trabalho.
2. **O Acceptance Criteria cita algo falsificável** (arquivo+linha, comando, saída esperada) **ou é
   prosa genérica?** Este próprio roadmap usa "Gates da wave" com comandos `bash` executáveis — é o
   vocabulário próprio do projeto para "isto não é decorativo". Roadmap sem nenhum comando de
   validação é o padrão oposto.
3. **A Motivation explica por que a mudança é necessária, ou só reformula o diff/commit message?**
   Reformulação pura ("mudei X para Y") sem o "por quê" é o sintoma mais direto de artefato
   preenchido para passar no gate, não para guiar a mudança.
4. **A REQ existia antes do código, ou foi criada na mesma janela do PR sem nenhuma iteração
   visível?** Sinal mais fraco (REQs legítimas também nascem perto do código, como a própria REQ
   deste roadmap), mas combinado com os três anteriores ajuda a formar o quadro.

Nenhum desses quatro é gate — todos exigem um humano olhando o diff ao lado da REQ. O
`CONTRIBUTING.md`/`PULL_REQUEST_TEMPLATE.md` (Wave 1) deveria pedir isso explicitamente do revisor,
não só do autor.

---

## 4. O tensionamento que a tarefa pediu para não resolver fácil

**A favor da regra:** nesta mesma sessão, a cadeia de governança pegou dois critérios de aceite
vácuos, um contrato que seria falso, um `skipif` que anularia a verificação inteira, e uma afirmação
factualmente errada em dois documentos — nenhum apareceria em CI verde. Isso é real e é o argumento
mais forte a favor de publicar a regra sem enfraquecê-la.

**Contra a aplicação uniforme e pesada da regra:** os quatro PRs anteriores do mesmo contribuidor
externo (incluindo #238, verificado nesta sessão: mergeado com todos os checks de governança verdes,
sem um único bloqueante levantado pelas duas barreiras) tiveram qualidade que **não veio da cadeia
imposta a ele** — vieram portados pelos mantenedores, com REQ e roadmap escritos aqui, não por ele.
A cadeia mediu o trabalho dele; não foi o que produziu a qualidade dele.

**Como eu pesaria isto, sem escolher o lado confortável:** as duas evidências não estão em conflito
real — estão medindo coisas diferentes. A cadeia provou valor em trabalho **interno, de
instrumentação e verificação** (a suíte de Windows desta mesma sessão) — trabalho onde o "óbvio
visualmente correto" esconde vácuo com frequência mensurável. O contribuidor externo entregou
**fixes pontuais e localizados** (uma linha de gate, uma correção de comportamento) onde o "óbvio"
raramente esconde algo, porque o raio de efeito é pequeno e visível no diff.

**Não seria honesto concluir "dispensar a regra para contribuidores confiáveis"** — confiança não é
verificável a priori para quem chega de fora, e decidir caso a caso por reputação é exatamente o
tipo de julgamento ad hoc que fez a regra nunca ser publicada em primeiro lugar. Mas também não é
honesto ignorar que o Caso A (2 microlotes corretivos gastos portando REQ+roadmap completo para um
fix de uma linha) é custo real, medido, sem retorno equivalente medido.

**Recomendação concreta:** dois níveis de rigor, não um. REQ é sempre exigida para o não-trivial (a
lista dos §7 redesenhada no item 1), mas o roadmap-com-waves-e-microlotes só é exigido quando há
mais de um arquivo de lógica ou mais de um CLI envolvido. Para o Caso A (fix pontual de gate, um
arquivo, um CLI) uma REQ curta — motivação + evidência de antes/depois do gate — sem estrutura de
Wave/ML satisfaz `wip_has_req` e é proporcional ao risco. Isto é uma proposta para a Wave 1 decidir
formalmente (talvez mereça ADR, como a própria REQ já cogita), não uma implementação minha.

---

## 5. Residual declarado

- **`required_status_checks` ausente em `main`** — o achado de maior impacto deste ML. Sem ele, a
  regra publicada no `CONTRIBUTING.md` é tecnicamente idêntica em força a um gate inerte: correta
  quando roda, sem consequência garantida quando falha. Fora do Negative Scope desta REQ
  ("implementar bloqueio é decisão que merece REQ própria") — recomendo REQ dedicada.
- **`branch_has_wip_roadmap` casa por substring contra um corpus de `done/` que só cresce**
  (`REQ-2026-08-20`, `status: Open`, sem roadmap) — pré-existente, não introduzido por este ML, mas
  relevante porque o gate "passar" hoje não garante relação real entre branch e REQ.
- **Nenhum gate mede a qualidade do conteúdo de uma REQ** — `contentHasMarker` confirma só que o
  campo não está vazio. A distinção real/decorativa (item 3) permanece dependente de revisão humana;
  isto não é um defeito a corrigir com mais regex, é uma propriedade que este ML recomenda declarar
  explicitamente no `CONTRIBUTING.md`, para não prometer uma garantia que a ferramenta não dá.
- **O `PULL_REQUEST_TEMPLATE.md` (AC4, Wave 1) não terá enforcement técnico** — nada no CI hoje lê o
  corpo do PR. Os campos "REQ ligada / roadmap ligado / falsificação nas duas direções" no template
  são, e vão continuar sendo depois deste roadmap, prosa que depende do autor preencher e do revisor
  checar — não um gate. Isto deve estar explícito no próprio template ou no `CONTRIBUTING.md`, para
  não implicar uma garantia técnica que não existe.
- **Dois workflows redundantes fazem o mesmo `trackfw validate`** (`trackfw-validate.yml` builda do
  source; `trackfw-gate.yml` instala via `curl | sh` do último release) — não é bloqueante para este
  ML, mas é uma duplicação que confunde qual dos dois é "o" gate de governança ao configurar
  `required_status_checks` na recomendação acima; sinalizo para quem for fazer essa REQ.
- **A régua redesenhada no item 1 (casos A–D) é uma recomendação de conteúdo para a Wave 1, não uma
  decisão tomada por mim** — cabe à `atena-tf` e ao arquiteto aceitar, ajustar ou rejeitar cada um
  dos quatro tratamentos propostos.

---

## Execução

Nenhum comando de build/teste de produto foi necessário para este ML (parecer, sem código de
produto tocado). Comandos executados para produzir a evidência acima: `gh api
repos/kgsaran/trackfw/branches/main/protection`, `gh pr view 238 --json statusCheckRollup`, `gh pr
list --state merged --author lourivalgarciajunior`, leitura direta de
`internal/validator/validator.go` (`validateBranchHasWIPRoadmap`, `contentHasMarker`,
`validateWIPHasREQ`, `validateREQsHaveRoadmap`) e dos workflows em `.github/workflows/`.
