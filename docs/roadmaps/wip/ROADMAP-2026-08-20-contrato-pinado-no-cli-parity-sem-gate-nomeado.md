---
status: wip
date: 2026-08-20
req: "docs/req/REQ-2026-08-18-contrato-pinado-no-cli-parity-sem-gate-nomeado-e-contrato-nao-aplicado.md"
adr: "docs/adr/ADR-2026-08-20-anotacao-de-cobertura-de-contrato-no-cli-parity.md"
squad: "apolo-tf, hefesto-tf"
---

# Roadmap: contrato pinado no `cli-parity.md` sem gate nomeado

> Created: 2026-08-20 | Status: wip

## Context

REQ: `docs/req/REQ-2026-08-18-contrato-pinado-no-cli-parity-sem-gate-nomeado-e-contrato-nao-aplicado.md`

A regra que falta, por analogia com o **P4** que o projeto já sustenta (*gate sem cenário de
falsificação é gate não-verificado*):

> **Contrato pinado sem gate nomeado é contrato não-aplicado.**

### Por que agora, e não como higiene de fim de sprint

A REQ foi aberta em 2026-08-18 com **duas** instâncias medidas. Entre aquela data e hoje, a lacuna
produziu mais evidência do que a própria REQ tinha quando foi escrita:

| evidência acumulada | onde |
|---|---|
| `--json` do `doctor`: Go emitia `null`, Node/Python `[]` | ML-2B do `doctor` |
| relatório de texto do `doctor`: linha em branco só no Go | ML-2B do `doctor` |
| `exec.Command().Output()` do Go descartava o stderr do filho | ML-1B do force-push |
| erro de git no fallback do Python divergente | ML-2A do `release tag` |
| timestamp com milissegundos no Node | ML-2A do `release tag` |

**Cinco divergências reais em três dias**, nenhuma detectável por teste por stack — cada runtime
concorda consigo mesmo. Todas apareceram só quando alguém escreveu um gate comparando as **três
saídas reais**. Enquanto não houver mecanismo que force a existência do gate, isso depende de alguém
lembrar.

### Medição de hoje (2026-08-20), refeita — não a da REQ

```
seções de topo (##) no cli-parity.md : 53
subseções (###)                      : 122
scripts check-*.sh                   : 27
```

A contagem de "seções que nomeiam gate" da REQ (18 de 52) precisa ser **refeita** pelo executor: o
documento cresceu desde então, e três seções novas entraram já nomeando o gate.

## 🔴 Riscos que valem para todos os MLs

1. **O modo de falha previsível é silenciar o checker** marcando tudo como não-contrato. Nenhuma
   mitigação impede o abuso; elas o tornam **visível**. É a mesma postura do `credential-guard`:
   detecção ancorada, não prevenção.
2. **Super-marcar como contrato** gera lacunas falsas e ruído que treina o leitor a ignorar. Em
   dúvida, marcar como contrato-sem-gate e deixar visível é a opção conservadora — mas dúvida
   sistemática é sinal de que o critério está mal definido, não de que se deve chutar.
3. **A triagem é julgamento, não mecânica.** É o grosso do trabalho e o produto mais valioso.
4. **Não testar por leitura.** O checker precisa ser exercitado contra seções reais do documento.
5. **O checker é um gate** — logo, ele mesmo precisa de cenário P4. Meta-checker sem falsificação
   seria a própria ironia.

---

## Wave 1 — Formato e mecanismo (2 MLs, sequenciais)

### ML-1A — Aplicar o formato em 3 seções-piloto
**Status:** ✅ Concluído · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`)
**Arquivos:** `docs/cli-parity.md` (apenas 3 seções-piloto).

> **O ADR foi escrito por mim, não delegado** — decisão de formato é do arquiteto, e o roadmap
> original o atribuía ao executor por engano. Formato decidido em
> `ADR-2026-08-20-anotacao-de-cobertura-de-contrato-no-cli-parity.md`:
> ```
> <!-- trackfw-contract: gate=scripts/check-doctor-parity.sh -->
> <!-- trackfw-contract: none reason=<motivo em uma linha> -->
> ```
> Resta ao ML-1A **aplicar** e provar que o formato aguenta os três casos.

**Ação:** decidir e registrar em **ADR** o formato pelo qual uma seção declara o gate que a protege,
e pelo qual uma seção se declara **não-contrato com motivo**. Aplicar em **3 seções-piloto** de
naturezas diferentes — uma com gate óbvio, uma sem gate, uma que é prosa — para provar que o formato
aguenta os três casos antes de virar 53.

**Critérios de aceite:**
- [x] Formato decidido em ADR, com o motivo da escolha — feito por mim
- [x] 3 seções-piloto anotadas, cobrindo os **três** casos: com gate, sem gate, não-contrato
- [x] A escolha de cada piloto é **justificada** — piloto fácil demais não prova nada
- [x] Nenhuma mudança de comportamento de CLI, nenhum gate criado

> **Achado do executor (Apolo), pendente de decisão do arquiteto antes do ML-2A:** o formato do
> ADR só define `gate=<caminho>` e `none reason=<motivo>` — não há forma explícita para
> "contrato sem gate", o caso mais valioso da REQ. Anotado como `gate=` (chave documentada, valor
> vazio) por não inventar sintaxe nem fabricar caminho de script inexistente. Ver
> `docs/agents-working-context.md`, sessão 2026-08-20 (Apolo), para a medição completa
> (`####` — 17 headers não contados na REQ/roadmap — e o exemplo de gate parcial em
> `## Vault de conhecimento`, que também revelou `note_orphan` ausente no validator do Node.js).

### ML-1B — Meta-checker
**Status:** ⬜ Pendente · **Agente:** `apolo-tf` · **Dependência:** ML-1A
**Arquivos:** `scripts/check-parity-contract-coverage.sh` (novo), `Makefile` (alvo `parity`),
`scripts/check-gates-falsify.sh`.

**Ação:** checker que reprova quando (a) seção de contrato **não nomeia** gate, (b) nomeia gate que
**não existe** no disco, (c) marcação de não-contrato **sem motivo**.

Enquanto a triagem da Wave 2 não terminar, o checker roda em **modo relatório** — conta e lista, sem
reprovar. Vira bloqueante no ML-3A. Sem isso o `make quality` fica vermelho durante toda a triagem, e
gate vermelho por semanas é gate que se aprende a ignorar.

**Critérios de aceite:**
- [ ] Reprova seção de contrato sem gate nomeado
- [ ] Reprova gate nomeado inexistente — **aponta para o vazio**
- [ ] Reprova não-contrato sem motivo
- [ ] Modo relatório enquanto a triagem não fecha; conta e lista
- [ ] Cenário P4 do próprio checker: baseline + detecção para os três casos
- [ ] Exercitado contra seções **reais** do documento, não fixture sintético
- [ ] `make quality` verde

---

### Auditoria do ML-1A — aprovada, e o piloto **pagou-se** antes de escalar

Era exatamente para isto que o lote existia: descobrir barato que o formato não aguentava. Descobriu.

**Confirmei as três medições por conta própria:**

```
niveis de titulo:  ## 53 · ### 122 · #### 17      <- o #### nao existia na REQ nem no roadmap
note_orphan:       Go 3 ocorrencias · Python 4 · Node ZERO
```

**Achado 1 — o formato tinha dois estados e o caso central da REQ é um terceiro.** Não havia forma
para *"isto é contrato e nada o protege"*. O executor contornou com `gate=` vazio e **sinalizou a
decisão em vez de escondê-la** — escolha certa diante da alternativa de inventar caminho de script,
que a própria ADR chama de carimbo. Mas valor vazio é indistinguível de **omissão**, e o checker não
separaria "declarei a lacuna" de "esqueci de preencher". Resolvido na **Emenda 1**: estado `gap`
próprio, greppável e **contável** — a contagem de `gap` é o produto da REQ e precisa ser um número
que se acompanhe cair.

**Achado 2 — três níveis de título, não dois.** E o estado de contrato **não acompanha a
profundidade**: há `####` de não-contrato dentro de `##` de contrato. O universo da triagem é **~192,
não 175**. O ML-2A está subdimensionado no roadmap e precisa ser refatiado.

**Achado 3 — cobertura parcial não era expressável.** Medido no piloto 2: o gate cobre a mecânica de
criação de nota mas não a semântica da regra. Colapsava em vazio. Emenda 1 acrescenta `partial=`.

**Achado 4 — regra de desempate.** Seção que se autodeclara não-contrato e mesmo assim fixa fato
falsificável. Emenda 1: **fato falsificável sobre comportamento de CLI ⇒ é contrato**; a
autodeclaração não prevalece.

#### O achado lateral é a melhor evidência que esta REQ podia ter

`note_orphan` existe em Go e Python e **está ausente do CLI Node**, com `cli-parity.md:147`
documentando-a como contrato. Violação viva da regra dura de paridade.

E o modo como apareceu é o argumento: **bastou alguém perguntar "qual gate protege esta seção?"**.
Não houve investigação — a pergunta que o mecanismo faz produziu a descoberta antes de o mecanismo
existir. Aberta a `REQ-2026-08-20-note-orphan-existe-em-go-e-python-e-esta-ausente-do-cli-node`
(backlog), com escopo negativo explícito: **não** varrer as outras regras à mão, porque é justamente
isso que o ML-2A vai fazer de forma sistemática.

`make quality` exit 0 · `validate` exit 0.


### Auditoria do ML-1A-bis — aprovada

```
linha  37  gate=scripts/check-cli-parity.sh
linha 139  gate=scripts/check-artifact-parity.sh partial=regra note_orphan nao comparada entre os 3 CLIs
linha 159  gap reason=... ver REQ-2026-08-16-conformidade-...-i18n-...
os dois caminhos de gate existem no disco (conferido)
make quality exit 0 · validate exit 0
```

**A decisão do piloto 2 é dele e está certa:** `partial`, não `gap`. Ele mediu que o
`check-artifact-parity.sh` exercita a mecânica de `note new` nos 3 CLIs — a cobertura **existe**, é
parcial, não zero. Marcar `gap` teria apagado a cobertura real e inflado a contagem de lacunas, que
é justamente o número que a REQ quer confiável.

**O piloto 3 sobreviveu ao primeiro teste real da regra de desempate.** Eu escrevi a regra a partir
do relato dele, sem olhar a seção, e pedi que discordasse se ela produzisse resultado ruim no caso
concreto. Ele aplicou e não discordou, com o argumento certo: a alternativa **manteria a
autodeclaração como juíza de si mesma**. E foi além do pedido — ligou o `gap` à
`REQ-2026-08-16-...-i18n-...`, que é onde a lacuna já está rastreada. Lacuna com destino é melhor
que lacuna apenas contada.

**Achado de parsing incorporado ao ADR**, sem mudar o formato: `reason=`/`partial=` são texto livre e
podem conter `=` e `,`. Restringir custaria expressividade onde ela mais importa. Muda a regra de
leitura — parser reconhece **prefixos de chave conhecidos** e consome até o próximo ou o fim da
linha. E chave desconhecida é **erro**, não texto: senão um `reson=` com erro de digitação viraria
parte do valor anterior e passaria em silêncio.

---

### ML-1A-bis — Reaplicar os 3 pilotos no formato da Emenda 1
**Status:** ✅ Concluído · **Agente:** `apolo-tf` · **Dependência:** Emenda 1 (feita).

**Piloto 1 (`## Version output`):** revalidado, segue válido no formato final —
`gate=scripts/check-cli-parity.sh`, script existe no disco.

**Piloto 2 (`## Vault de conhecimento`):** o `gate=` vazio virou `partial`, não `gap`. Medição:
`scripts/check-artifact-parity.sh` de fato exercita a mecânica de `note new` nos 3 CLIs (cria
`vault/notes/<slug>-DATA.md` e a linha em `index.md` — cenário `note`/`note_index` do script,
comparado entre Go/Node/Python). O que esse gate **não** cobre é a semântica da regra de validate
`note_orphan` — nenhum script compara o comportamento dessa regra entre os 3 CLIs, e é exatamente
essa lacuna que expôs `note_orphan` ausente do validator do Node (ver achado lateral do ML-1A,
`REQ-2026-08-20-note-orphan-existe-em-go-e-python-e-esta-ausente-do-cli-node`). Como existe gate
cobrindo parte do contrato da seção, `partial=` é o estado correto, não `gap` (que é para "nada
protege"):
```
<!-- trackfw-contract: gate=scripts/check-artifact-parity.sh partial=regra note_orphan não comparada entre os 3 CLIs -->
```

**Piloto 3 (`## i18n locale keys`):** reclassificado de `none` para `gap` sob a regra de desempate.
A seção se autodeclara não-contrato ("cli-parity.md não documenta paridade de chaves i18n como
contrato"), mas fixa um fato falsificável e presente sobre o comportamento dos 3 CLIs: `errors.notFound`
está ausente das três árvores de locale e sem consumidor em nenhum runtime. Isso é testável por
grep hoje, e é exatamente o tipo de afirmação que a regra de desempate da Emenda 1 cobre — prosa que
afirma comportamento é contrato, rotulada ou não. Não há gate no repo que compare chaves de locale
entre runtimes (confirmado — `grep -rl locale scripts/*.sh` só acha `check-gates-falsify.sh`, que não
testa isso), logo o estado correto é `gap`, não `none`:
```
<!-- trackfw-contract: gap reason=a seção fixa fato falsificável (errors.notFound ausente e sem consumidor nos 3 CLIs) mas nenhum gate compara chaves de locale entre runtimes; ver REQ-2026-08-16-conformidade-estrutural-e-comportamental-de-i18n-entre-os-tres-clis -->
```
Não discordo da regra aplicada a este caso: a alternativa (manter `none`) exigiria que a própria
seção decidisse por autodeclaração se é contrato, que é precisamente o problema que a Emenda 1
resolve.

**Ambiguidade de parsing para o ML-1B (achado, não corrigido aqui):** nenhum dos 3 valores usados
acima tem `=` dentro do texto livre de `reason`/`partial`, nem vírgula dentro de um caminho de gate
único. Mas o formato **permite** ambas as coisas hoje — nada no ADR proíbe `reason=` conter `=` ou
`,`, e `partial=` aceita texto livre sem limite de tamanho. Um checker por regex ingênuo que
faça split em `,` para separar múltiplos `gate=` vai quebrar se um `reason`/`partial` contiver
vírgula (ex.: o texto do piloto 2 tem vírgula? Não, mas é fácil escrever um que tenha). Recomendo
ao ML-1B: (a) parsear pelo **prefixo da chave** (`gate=`/`partial=`/`reason=`) até o próximo prefixo
de chave conhecido ou fim de linha, nunca por split ingênuo em vírgula; (b) proibir explicitamente
`=` dentro de `reason`/`partial` livre não é necessário se o parser não depender de split por `=`.
Nenhuma seção do documento hoje tem duas anotações `trackfw-contract` na mesma linha nem comentário
HTML adicional colado na mesma linha — não há caso real disso a corrigir agora.

**Critérios de aceite:**
- [x] Os 3 pilotos no formato final da Emenda 1
- [x] Escolha entre `gap` e `partial` no piloto 2 justificada por medição (`check-artifact-parity.sh`
      existe e cobre a mecânica; `note_orphan` semântico não tem gate)
- [x] Piloto 3 reavaliado sob a regra de desempate, com veredito escrito (reclassificado `none` → `gap`)
- [x] Caminhos de gate nomeados existem no disco (`check-cli-parity.sh`, `check-artifact-parity.sh`)
- [x] `./bin/trackfw validate` exit 0 · `make quality` exit 0

---


### Auditoria do ML-1B — aprovada, com um 6º caso que ele achou e não corrigiu sozinho

```
5 classes de reprovacao provadas por execucao (Cenarios 77b-77g), delta unico cada
nao-vacuidade (77h): checker com os.path.isfile neutralizado fica em silencio
documento real: 1 gate · 1 gate+partial · 1 gap · 0 none · 174 sem anotacao · 0 invalidas
138 cenarios · make quality exit 0 · validate exit 0 · invocacao CI-exata exit 0
```

**Ele rodou a invocação CI-exata sem eu precisar cobrar.** Depois de três rodadas de CI perdidas
ontem por essa exata lacuna, é o tipo de coisa que quero ver virar hábito.

**Achado 1 — parsing ciente de blocos de código.** Minha medição por `grep` cru estava errada:
15 dos "cabeçalhos" são literais dentro de exemplos de template (`## Motivation`, `## Context`) para
`req new`/`adr new`/`roadmap new`. **O universo é 177, não ~192.** Confirmei por conta própria.
Numa triagem que é julgamento, 8% de falsos obrigaria o triador a decidir sobre seções que não
existem. Corrigido na Emenda 2 e aqui.

**Achado 2 — `partial=` vazio passa em silêncio, e ele sinalizou em vez de corrigir.** Foi a atitude
certa: não estava entre os 5 casos que eu documentei, e inventar o 6º por conta própria seria decidir
formato, que é meu. Verifiquei o furo:

```
gate=scripts/check-cli-parity.sh partial=     ->  exit 0, conta como cobertura parcial
                                                   SEM dizer o que fica de fora
```

É o mesmo argumento que matou o `gate=` vazio, e aqui é pior — a seção some do relatório que existe
para revelar lacunas. **Vira o 6º caso**, com regra geral na Emenda 2 para não precisar emendar a
cada chave nova: *toda chave presente exige valor não-vazio; para "não se aplica", omita a chave*.

---

### ML-1B-bis — 6º caso de reprovação
**Status:** 🔄 Em andamento (implementado, aguardando auditoria) · **Agente:** `apolo-tf` · **Dependência:** ML-1B.
Aplicar a regra geral da Emenda 2 no checker: chave presente com valor vazio reprova. Braço P4 para
o caso. Lote de minutos, mas precisa entrar **antes** do ML-2A — 177 seções anotadas contra um
checker permissivo seriam 177 anotações a reconferir.


### Auditoria do ML-1B-bis — aprovada, e ele achou **a mesma classe outra vez**

```
regra GERAL, nao if por chave: um laco sobre as chaves extraidas, antes da logica de estado
substituiu duas checagens ad-hoc (gate=, reason=) que viraram codigo morto
bracos 77i/77j/77k + 77l de nao-vacuidade (neutraliza o proprio laco)
142 cenarios · invocacao CI-exata exit 0 · validate exit 0
```

O `77l` é o braço que importa: ele neutraliza **o laço geral** e prova que é ele, e não alguma
checagem específica remanescente, que sustenta a detecção. Sem isso, a regra "geral" poderia estar
passando por acidente das antigas.

#### 🔴 O achado dele expõe uma lacuna de conformidade **do meu próprio ADR**

Ele sinalizou — de novo sem corrigir, de novo certo — que **chave desconhecida escrita depois de uma
conhecida** é absorvida no texto livre em vez de reprovar. Verifiquei:

```
<!-- trackfw-contract: gap reason=motivo qualquer reson=erro de digitacao -->
checker -> exit 0, e o relatorio lista "motivo qualquer reson=erro de digitacao" como motivo
```

Isto **não é decisão nova**: é exatamente o caso que eu escrevi na Nota de parsing do ADR, com estas
palavras — *"chave desconhecida é erro, não texto livre — senão um `reson=` com erro de digitação
viraria parte do valor anterior e o checker aceitaria em silêncio"*. O checker só inspeciona chave
desconhecida **antes** da primeira conhecida, então o cenário que motivou a regra é justamente o que
escapa.

**A regra está certa e escrita; a implementação não a cumpre.** É conformidade, não escopo novo.

**Padrão que vale nomear:** é a terceira vez seguida que o mesmo executor encontra uma lacuna, **não
a corrige por conta própria** e a devolve com o argumento. Nas três, a contenção foi correta — e nas
três a lacuna era real. Isso é o comportamento que eu quero: quem executa sinaliza, quem decide
decide.

---

### ML-1B-ter — Conformidade: chave desconhecida em qualquer posição
**Status:** ⬜ Pendente · **Agente:** `apolo-tf` · **Dependência:** ML-1B-bis. **Antes do ML-2A.**
Fazer o checker cumprir a regra já escrita na Nota de parsing do ADR: chave desconhecida reprova
**em qualquer posição**, não só antes da primeira conhecida. Braço P4 com o caso `reson=` depois de
`reason=`. Precisa entrar antes da triagem — 177 anotações escritas contra um parser que engole
erro de digitação em silêncio seriam 177 a reconferir.

## Wave 2 — Triagem (o grosso do trabalho)

### ML-2A — Triagem das seções
**Status:** ⬜ Pendente · **Agente:** `hefesto-tf` (`subagent_type: hefesto-tf`) · **Dependência:** ML-1B
**Escreve:** anotações em `docs/cli-parity.md` e o relatório de triagem.

**Ação:** classificar **cada** seção nos **três** estados da Emenda 1 (`gate=`, `gap`, `none`),
mais `partial=` onde couber. Refazer a contagem: a da REQ (18 de 52) está defasada.

🔴 **Refatiar antes de começar.** O universo medido é **177** (`##` 39 · `###` 121 · `####` 17) — ver Emenda 2:
o `grep` cru contava 15 cabeçalhos dentro de blocos de código de template, que não são seções. Triagem de 192 seções num ML só é grande demais para auditar bem — dividir por faixas do
documento, com cada lote auditável de forma independente.

**O produto mais valioso desta REQ é a lista de contratos SEM gate.** Ela não é subproduto da
triagem — é o entregável.

**Critérios de aceite:**
- [ ] Todas as seções classificadas
- [ ] Lista de contrato-sem-gate produzida e registrada, ordenada por risco
- [ ] Cada não-contrato tem motivo escrito
- [ ] Contagem de não-contratos reportada pelo checker — o abuso fica visível
- [ ] `make quality` verde

---

## Wave 3 — Tornar bloqueante

### ML-3A — Checker vira bloqueante
**Status:** ⬜ Pendente · **Agente:** `apolo-tf` · **Dependência:** ML-2A
**Critérios de aceite:**
- [ ] Checker reprova de verdade; `make quality` verde porque a triagem fechou, não porque o checker
      é permissivo
- [ ] Seção nova sem anotação **reprova** — provado por cenário
- [ ] CI verde

---

## Notas
- **Fora de escopo, declarado:** criar os gates faltantes. Esta REQ cria o **mecanismo que revela** a
  ausência; fechar cada lacuna é trabalho subsequente e priorizável, e provavelmente não vale para
  todas.
- **Fora de escopo:** exigir gate para tudo. Seção que descreve exceção intencional ou contexto deve
  ser marcada como não-contrato, não ganhar gate inventado.
- Commits e branch são exclusivos do `trackfw_architect`.
