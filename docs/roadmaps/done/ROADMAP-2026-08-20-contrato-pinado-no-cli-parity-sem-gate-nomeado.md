---
status: done
date: 2026-08-20
req: "docs/req/REQ-2026-08-18-contrato-pinado-no-cli-parity-sem-gate-nomeado-e-contrato-nao-aplicado.md"
adr: "docs/adr/ADR-2026-08-20-anotacao-de-cobertura-de-contrato-no-cli-parity.md"
squad: "apolo-tf, hefesto-tf"
---

# Roadmap: contrato pinado no `cli-parity.md` sem gate nomeado

> Created: 2026-08-20 | Status: done

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
**Status:** ✅ Concluído · **Agente:** `apolo-tf` · **Dependência:** ML-1B-bis. **Antes do ML-2A.**
Fazer o checker cumprir a regra já escrita na Nota de parsing do ADR: chave desconhecida reprova
**em qualquer posição**, não só antes da primeira conhecida. Braço P4 com o caso `reson=` depois de
`reason=`. Precisa entrar antes da triagem — 177 anotações escritas contra um parser que engole
erro de digitação em silêncio seriam 177 a reconferir.


### Auditoria do ML-1B-ter — aprovada; testei a fronteira do heurístico, não o caminho feliz

O critério é: token minúsculo seguido de `=`, precedido por início ou espaço, a **distância de edição
≤ 1** de `gate`/`partial`/`reason`. Exercitei os dois lados eu mesmo:

```
DEVEM REPROVAR
  reason=motivo reson=typo      -> 1  ✓
  reason=motivo gatee=x         -> 1  ✓   (o relatorio dele dizia que escapava — nao escapa)
  reason=motivo raeson=x        -> 0  falso-negativo ACEITO (transposicao = distancia 2)
DEVEM PASSAR
  reason=... sob LANG=pt_BR ...      -> 0  ✓  (maiuscula)
  reason=a flag --scope=global ...   -> 0  ✓  (precedido por '-', nao espaco)
  reason=taxa rate=10 por segundo    -> 1  falso-positivo CONFIRMADO
```

**Duas correções ao relatório dele, ambas medidas:** `gatee=` **é** pego (ele disse que escapava — a
implementação é melhor que a descrição), e o falso-positivo do `rate=10` é **real**, não hipotético.

**Aceito o `rate=10`**, e o motivo: o dano é uma recusa com mensagem clara, e o autor reescreve o
motivo. O dano oposto — deixar `reson=` passar — é silencioso, e silêncio num verificador é a falha
que esta REQ inteira combate. A escolha de distância 1 em vez de 2 está justificada no próprio
script: distância 2 pegaria transposições, mas o crescimento de falsos-positivos supera o ganho.

**Escolha de desenho que endosso:** ele fez uma **segunda passada** independente do parser por
prefixo, varrendo inclusive dentro do valor já fatiado — que é onde o defeito morava. Tentar remendar
o parser primário teria misturado duas responsabilidades e provavelmente reintroduzido o problema.

`145 cenários` · invocação CI-exata exit 0 · `validate` exit 0.

**Wave 1 fechada.** O mecanismo está pronto e falsificado; a triagem pode começar contra um checker
que já cumpre o formato final.

## Wave 2 — Triagem (o grosso do trabalho)

### Fatiamento da triagem — 4 lotes sequenciais

177 seções, medidas com parsing ciente de blocos de código. **Sequenciais, não paralelos:** todos
editam `docs/cli-parity.md`, e dois agentes no mesmo arquivo é conflito garantido.

| lote | linhas | seções |
|---|---|---|
| ML-2A | 35–1209 | 44 |
| ML-2B | 1230–2668 | 44 |
| ML-2C | 2685–3652 | 44 |
| ML-2D | 3669–4545 | 45 |

**Por que fatiar:** triagem é julgamento, não mecânica. 177 seções num lote só não é auditável com o
mesmo rigor aplicado à Wave 1 — e auditoria fraca no lote mais importante anularia o cuidado de tudo
que veio antes. Cada lote fecha com o checker verde e é auditado antes do seguinte.

**O ML-2A é também o lote-piloto da triagem:** se o critério de classificação se mostrar ambíguo
demais na prática, o ajuste acontece depois de 44 seções, não de 177.

### ML-2A — Triagem, lote 1 (linhas 35–1209)
**Status:** 🔄 Em andamento — lote 1/4 concluído (linhas 35–1209, 44 seções); lotes 2–4 pendentes ·
**Agente:** `hefesto-tf` (`subagent_type: hefesto-tf`) · **Dependência:** ML-1B
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


### Auditoria do ML-2A — aprovada; **39% do primeiro quarto é contrato sem gate**

```
44 secoes:  gate= 17 · gate+partial 9 · gap 17 · none 1 · invalidas 0
checker exit 0 · make quality (CI-exata) exit 0 · validate exit 0
```

**17 de 44 = 39%.** Se a proporção se mantiver, o documento inteiro tem por volta de 70 contratos
não-aplicados. A REQ deixou de ser hipótese na primeira medição real.

**Verifiquei dois `gap` por medição, não pelo relatório:**

```
gap #1  check-ship-parity.sh cobre lenient mode?  -> grep -c lenient = 0   CONFIRMADO
gap #9  check-artifact-parity.sh KINDS inclui schemas?
        KINDS=(req adr roadmap ... note note_index)  -> nao inclui        CONFIRMADO
```

O `gap #1` é o mais grave e ele acertou ao pô-lo em primeiro: a seção existe para fixar que o `ship`
**ignora** `governance_mode: lenient` enquanto o `validate` respeita — e nenhum cenário monta um
fixture com `lenient`. Uma regressão que fizesse o `ship` passar a respeitar o modo permissivo
atravessaria o gate inteiro. É o portão duro de governança dependendo de teste que não existe.

**A relação `none` = 1 é o sinal de que não houve conveniência.** O modo de falha previsível desta
REQ é inflar `none` para silenciar o checker; um único `none` em 44 seções, e num caso genuinamente
fora do domínio de comportamento de CLI (deriva de documentação de site), é o oposto disso.

#### Duas lições dele que valem para os 133 restantes — vão nos handoffs

**1. `none` só quando o assunto está fora do domínio de comportamento de CLI.** Ele marcou `none` na
primeira passada sempre que a seção dizia "isto não é requerido", e corrigiu 2 dos 3 na revisão:
prosa de disclaimer costuma esconder uma **exigência positiva** falsificável por trás.

**2. Rodar o gate isolado antes de declarar `gap` — não basta grepar o script.** Ele classificou
três seções como `gap` por leitura estática e depois descobriu que o `check-identity-parity.sh`
**deriva alvos do catálogo dinamicamente**, cobrindo mais do que o shell lista literalmente. Corrigiu
sozinho. Esse é o falso-positivo mais provável da triagem, e agora tem contramedida escrita.

**Nenhuma discordância com os 3 pilotos.**


### Auditoria do ML-2B — aprovada, e a **declaração de nível de verificação** vale registrar

```
lote 2 (44):  gate= 27 · partial 14 · gap 3 · none 0
acumulado (88): gate= 44 · partial 23 · gap 20 · none 1 · invalidas 0 · 89 sem anotacao
make quality (CI-exata) exit 0 · validate exit 0
```

**Ele declarou o próprio limite, e isso é o mais valioso do relatório.** Escreveu que os 27 `gate=`
foram confirmados no nível *"o cenário existe e cobre a topologia da seção"*, **não** mapeamento
alegação-a-asserção, e **pediu** que eu amostrasse — dizendo que, se eu achasse o padrão numa,
provavelmente há outras. Executor que expõe a fragilidade da própria conclusão em vez de deixá-la
implícita é o que torna a auditoria possível.

**Amostrei 2 dos 24 `gate=` plenos, por medição:**

```
linha 1349  "review_doc_config — requires a PROPER subset"
   -> grep inicial: 'review_doc_config' so aparece em COMENTARIO (linhas 316-317). Suspeita.
   -> leitura do trecho: existe Cenario (e) real, 'review-doc-config-only', com fixture de
      subconjunto PROPRIO e o contraste feat/doc-real. ANOTACAO CORRETA.
linha 1491  '## trackfw barrier' -> check-barrier.sh, com diff byte a byte. CORRETA.
```

A primeira parecia defeito e não era — o grep encontrava só o comentário porque o cenário nomeia o
label de outra forma. Registro o falso alarme de propósito: **grep sobre nome de regra não é prova
de cobertura, nos dois sentidos** — nem de ausência, nem de presença.

#### A divergência estatística é achado, não ruído

```
lote 1:  39% gap    ·  20% partial
lote 2:   7% gap    ·  32% partial
```

A explicação dele é convincente e verificável: os tópicos do lote 2 — `branch prune`, `barrier`,
`update`, fiação de hooks — são features recentes, que **já nasceram com gate cross-CLI dedicado**.
O `gap` alto do lote 1 vinha de contrato **antigo**, nunca gateado.

E a frase que resume o valor da REQ, dele: **"ter gate nomeado não é o mesmo que o gate cobrir cada
alegação da seção"**. O `partial` subindo de 20% para 32% é exatamente isso ficando visível.

**17 reclassificações na autorrevisão**, todas do mesmo padrão: o gate prova que os 3 runtimes
concordam **entre si**, não que concordam com o texto literalmente pinado pela seção. É uma classe
de lacuna que eu não tinha antecipado ao escrever o ADR, e que só aparece quando alguém confere
alegação por alegação.


### Auditoria do ML-2C — aprovada; **respondi o item aberto que ele deixou**

```
lote 3 (44):   gate= 12 · partial 13 · gap 12 · none 7
acumulado(132): gate= 56 · partial 36 · gap 32 · none 8 · invalidas 0 · 45 sem anotacao
make quality (CI-exata) exit 0 · validate exit 0
```

**Item aberto dele, respondido por medição** — ele pediu que eu confirmasse que as 44 inserções não
mexeram na prosa, porque o anchor do `Edit` descarta anexar no lugar errado mas não um typo dentro
do trecho reproduzido:

```
git diff --numstat docs/cli-parity.md   ->  129 insercoes, 0 delecoes
linhas removidas que nao sejam anotacao ->  NENHUMA
```

**Zero deleções.** A prosa está byte-idêntica; toda a mudança é comentário inserido. Pedir essa
verificação em vez de afirmar que estava tudo bem foi a atitude certa — é o tipo de dano que passa
despercebido por meses num documento de 4,5 mil linhas.

#### O salto de `none` (1 → 0 → 7) eu fui olhar, porque é o modo de falha nomeado

Li os 7. **Todos legítimos, e o motivo é o conteúdo do quarto**: este trecho é o parecer de segurança
do credential-guard, e as seções são semântica de falha de **CLIs de terceiros** medida do fornecedor,
refutação de hipótese de ataque já descartada, declaração de escopo do que **não** foi medido,
referências cruzadas, e princípios de desenho dos próprios gates. Nada disso é comportamento do
trackfw. `none` aqui é a classificação correta, não conveniência.

#### O `gap` #1 dele é o mais grave dos três lotes

`branch_has_wip_roadmap` aceita, desde a `REQ-2026-07-26`, roadmap correspondente em `done/` além de
`wip/` — e **nenhum gate cross-CLI jamais põe um roadmap em `done/`**. Ele verificou os três
candidatos (`branch-new`, `commit`, `ship`) e o `check-validate-parity.sh`, que tem zero ocorrências
da regra. **O comportamento que define aquela REQ nunca foi testado entre os 3 CLIs** — e é a regra
que sustenta todo `branch`/`commit`/`ship` do projeto.

**Correções da autorrevisão dele que valem nota:** reclassificou uma seção de `none` para `gap` ao
perceber que "afirmar que algo **não** é usado" é alegação falsificável de ausência de saída; e
pegou um fechamento de comentário HTML malformado (`->` em vez de `-->`) que ele mesmo introduziu,
antes de me entregar.


### Auditoria do ML-2D — **triagem fechada, 177/177**

```
lote 4 (45):   gate= 16 · partial 15 · gap 10 · none 4
CONSOLIDADO:   gate= 72 (41%) · partial 51 (29%) · gap 42 (24%) · none 12 (7%)
               sem anotacao 0 · invalidas 0
diff: 90 insercoes, 0 delecoes — prosa byte-identica (conferido por mim)
make quality (CI-exata) exit 0 · validate exit 0
```

#### O achado #1 é de uma classe pior que lacuna, e verifiquei

O documento **afirma cobertura que não existe** para Windsurf e Amazon Q. Medido por mim:

```
grep -c 'windsurf|amazonq' em check-agent-hooks-parity.sh    -> 0
grep -c 'windsurf|amazonq' em check-harness-hooks-parity.sh  -> 0
e a tabela da secao lista os dois como "deny global" cabeados
```

**Ausência silenciosa é ruim; afirmação falsa é pior**, porque quem lê o documento para decidir se
pode confiar no wiring recebe uma garantia que não existe. É o único caso desse tipo nos quatro
lotes, e o executor acertou ao pô-lo no topo.

#### O que a distribuição diz — e é a conclusão útil da REQ

| lote | conteúdo | gap |
|---|---|---|
| 1 | contrato antigo, nunca gateado | **39%** |
| 2 | features recentes, nasceram com gate | 7% |
| 3 | parecer de segurança, prosa de investigação | 27% |
| 4 | REQs que nasceram com gate no mesmo ML | 22% |

**A prática do projeto melhorou.** O que entrou com gate no mesmo microlote está coberto; o passivo
é datado e não está crescendo. Isso muda o diagnóstico de *"estamos mal cobertos"* para *"há uma
dívida antiga identificada, e a prática atual não a aumenta"* — remédios diferentes.

E os **51 `partial`** (29%) são a descoberta que eu não tinha antecipado ao escrever o ADR: o padrão
dominante não é ausência de gate, é **gate que prova que os 3 runtimes concordam entre si sem provar
que concordam com o texto que a seção fixa**.

**Autorrevisão dele:** corrigiu uma alegação exagerada própria e **22 violações de formato** —
`partial=` isolado sem `gate=`, e caminhos de gate com comentário embutido que quebravam a checagem
de existência. Todas pegas antes de me entregar, pelo próprio checker.

**Nível de verificação declarado:** majoritariamente leitura direta de cenário; alguns `gap`/`partial`
apoiados em grep negativo — mais fraco, e ele disse isso em vez de deixar implícito.

## Wave 3 — Tornar bloqueante

### ML-3A — Checker vira bloqueante
**Status:** ✅ Concluído · **Agente:** `apolo-tf` · **Dependência:** ML-2A
**Critérios de aceite:**
- [x] Checker reprova de verdade; `make quality` verde porque a triagem fechou, não porque o checker
      é permissivo
- [x] Seção nova sem anotação **reprova** — provado por cenário
- [x] CI verde

**Execução:** `scripts/check-parity-contract-coverage.sh` — o branch que contava e listava seção
sem anotação, sem reprovar, agora também acrescenta a violação (7º caso de reprovação, junto aos 6
já existentes das Emendas 1/2). Mensagens de relatório atualizadas ("bloqueante desde o ML-3A" em
vez de "modo relatório"). `Makefile`/gate `parity` já invocava o script (nenhuma mudança necessária
ali — a ADR previu isso desde o ML-1B).

`scripts/check-gates-falsify.sh`, Cenário 77: o fixture compartilhado `write_s77_fixture()` tinha
uma 5ª seção ("Unannotated section") propositalmente sem anotação para provar o modo relatório —
ela foi **removida** do fixture compartilhado (senão os 12 braços 77a-77o, que não testam esse
caso, passariam a reprovar pelo motivo errado) e ganhou fixture **dedicado**: 77p empilha essa
mesma seção sem anotação em cima do baseline 77a (que sozinho, com as 4 formas válidas, continua
passando) e prova, single-delta, que só a ausência de anotação derruba o exit code — com a mensagem
exata `seção sem anotação trackfw-contract`. Total: 146 cenários (era 145).

Contagem de hoje, confirmada por execução: `gate= 72 · gate+partial 51 · gap 42 · none 12 · sem
anotação 0 · inválidas 0` — 177/177, igual à contagem que fechou a triagem no ML-2D.

**Validações:**
```
bash scripts/check-parity-contract-coverage.sh          -> exit 0 (sem anotação: 0)
GO_BIN=bin/trackfw bash scripts/check-gates-falsify.sh   -> 146 cenários, exit 0
TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 make parity          -> exit 0
make quality                                             -> exit 0
./bin/trackfw validate                                   -> exit 0 (mesmos 18 warnings pré-existentes,
                                                             nenhum novo)
```

---

## Notas
- **Fora de escopo, declarado:** criar os gates faltantes. Esta REQ cria o **mecanismo que revela** a
  ausência; fechar cada lacuna é trabalho subsequente e priorizável, e provavelmente não vale para
  todas.
- **Fora de escopo:** exigir gate para tudo. Seção que descreve exceção intencional ou contexto deve
  ser marcada como não-contrato, não ganhar gate inventado.
- Commits e branch são exclusivos do `trackfw_architect`.

### Auditoria do ML-3A — aprovada; **o bloqueio é fato, não afirmação**

Provei eu mesmo, acrescentando uma seção sem anotação ao documento real:

```
com secao nova sem anotacao  ->  EXIT=1  "sem anotacao: 1"
                                 "-- secoes sem anotacao (reprovam — bloqueante desde o ML-3A) --"
restaurado                   ->  EXIT=0
146 cenarios · make quality (CI-exata) exit 0 · validate exit 0
```

**A decisão de fixture dele merece registro**, porque é do tipo que se erra em silêncio: o helper
compartilhado do Cenário 77 tinha uma 5ª seção **deliberadamente sem anotação**, ali para exercitar
o antigo modo relatório. Com o bloqueio ligado, ela faria **12 sub-cenários** falharem — todos pelo
motivo errado, a seção sem anotação em vez do delta pretendido. Ele removeu do fixture compartilhado
e criou o `77p`, que parte do baseline limpo e acrescenta **um** cabeçalho sem anotação. Delta único
preservado.

Se ele tivesse apenas "consertado até ficar verde", teríamos 12 cenários passando por acidente e um
gate novo sem prova. É exatamente o padrão que esta REQ existe para impedir, e ele o evitou dentro
da própria REQ.

**Wave 3 fechada. REQ pronta para PR.**

