# Parecer de segurança — gate de duas fases para `trackfw plugins add` (binário de terceiro)

> Data: 2026-08-15 | Autor: `hades-tf` (Security Reviewer) | ML-0A
> REQ: `docs/req/REQ-2026-08-15-gate-de-seguranca-para-trackfw-plugins-install-download-de-binario-de-terceiro-sem-parecer-previo.md`
> Roadmap: `docs/roadmaps/wip/ROADMAP-2026-08-15-gate-de-seguranca-para-trackfw-plugins-add-binario-de-terceiro.md`
> ADR de referência (padrão a reusar): `docs/adr/ADR-2026-08-15-gate-de-duas-fases-para-artefatos-de-terceiro-quarentena-parecer-vinculado-por-checksum-e-deteccao-por-proveniencia-versionada.md`
> Doutrina de detecção: `docs/adr/ADR-2026-08-12-nao-ha-prevencao-contra-agente-induzido-com-escrita-irrestrita-a-resposta-e-deteccao-ancorada-no-git.md`

> **Como ler:** cada seção termina em veredito explícito, nunca "depende". Onde há mais de uma
> resposta defensável, a alternativa rejeitada vem registrada com o custo da escolha.

---

## ⚠️ Sinalização prévia — AC6 não é implementável como está escrito

Antes de entrar nas perguntas: **AC6** do roadmap diz *"Detecção de plugin **sem proveniência**,
portada nos 3 CLIs"*. Essa frase descreve o que P1 chama de **ramo (i)** — enumerar o que está
instalado e sinalizar o que não tem entrada aprovada. Como o parecer estabelece abaixo (P1), esse
ramo **não é portável** para `~/.trackfw/plugins` sem produzir falso-positivo entre projetos não
relacionados na mesma máquina. Se o ML-0B copiar o texto de AC6 tal como está, o ML-3A vai construir
uma regra que erra em projetos que nunca instalaram o plugin em questão. **AC6 precisa ser reescrita
para nomear apenas o ramo (ii) (adulteração pós-aprovação), com o ramo (i) declarado estruturalmente
ausente** — não é um corte de escopo, é uma correção de enunciado. Isto não bloqueia a REQ; bloqueia
apenas o texto atual de AC6 seguir para o ADR sem correção.

---

## P1 — Onde mora a proveniência do plugin? ⭐ Pergunta central

**Veredito: existe camada de detecção, mas só para UM dos dois ramos que o gate markdown tem —
o outro é declarado estruturalmente ausente, não implementado por atalho.**

### Por que a pergunta se divide em dois ramos, e por que isso não aparecia no gate markdown

A regra `thirdparty_artifact_has_provenance` (`internal/validator/validator_thirdparty_provenance.go`)
tem dois ramos, ambos fatais:

1. **Ramo (i) — instalação sem aprovação:** todo destino no `integrations-manifest.json` com
   `Claim.Origin == "thirdparty"` precisa ter entrada em `thirdparty-provenance.json`. Funciona
   porque o `manifest.json` é **por projeto**, vive dentro do repo, e é a própria lista canônica
   de "o que este projeto instalou" — enumerar esse manifest nunca produz ruído de outro projeto.
2. **Ramo (ii) — adulteração pós-aprovação:** para toda entrada aprovada, o checksum do arquivo
   instalado precisa bater com `installed_sha256`.

Para o binário, o destino é sempre `~/.trackfw/plugins/<nome>` — **fixo, global, e compartilhado
por todo projeto que roda naquela máquina** (verificado em `internal/plugins/plugins.go:146-152`,
idêntico em `npm/src/commands/plugins.js:61-63`). Isso quebra a premissa que faz o ramo (i)
funcionar: não existe "o manifest deste projeto" para o binário, porque o binário não pertence a
nenhum projeto — pertence à máquina.

### Ramo (ii) — adulteração — EXISTE e é implementável

Um índice **versionado no projeto** — reusando o formato de `thirdparty-provenance.json`
(`internal/thirdparty/provenance.go`), com entradas **chaveadas pelo nome do plugin**, não pelo
caminho de destino absoluto (`filepath.Rel(root, destination)` da regra existente não consegue
expressar `~/.trackfw/plugins/x` como relativo a `root` — é por isso que a chave precisa mudar, não
é detalhe cosmético). Cada entrada carrega `repo` resolvido, `url` de asset (redigida), tag
resolvida, `checksum_sha256` (bytes brutos, aprovação), `installed_sha256` (bytes já com o `chmod`
aplicado — binário não tem "forma normalizada" como markdown, então aqui os dois domínios colapsam
em um só), `approved_by`, `review_reference`.

`trackfw validate` ganha uma nova checagem: para cada entrada que o **projeto declarou** (só as que
o próprio projeto aprovou — nunca varre `~/.trackfw/plugins` inteiro), calcula SHA-256 do arquivo em
`~/.trackfw/plugins/<nome>` e compara contra `installed_sha256`. Isso é leitura de disco local, não
leitura de manifest alheio, e é git-anchored porque o **valor esperado** está commitado — o mesmo
raciocínio da Emenda 1 do ADR-2026-08-12 (âncora por alvo: aqui o alvo é "o que este projeto
aprovou", não "tudo que existe no diretório").

### Ramo (i) — instalação sem aprovação — DECLARADO ESTRUTURALMENTE AUSENTE

Isto **não é uma lacuna a fechar depois**, é uma propriedade do desenho escolhido (KG manteve o
escopo global). Um plugin instalado por outro projeto na mesma máquina, ou por fora do fluxo,
**não é um fato do projeto atual** — é um fato da máquina. Um `trackfw validate` rodando no
projeto B não tem base para afirmar "este binário não foi aprovado" quando ele pode muito bem ter
sido aprovado pelo projeto A, na mesma máquina, ontem. Enumerar `~/.trackfw/plugins` e cruzar contra
o índice do projeto B produziria erro em **todo** plugin instalado para qualquer outro projeto —
falso-positivo sistemático, não ocasional.

### Alternativas avaliadas e rejeitadas, com custo

1. **Enumerar o diretório global e cruzar contra o índice do projeto** (a leitura ingênua de AC6).
   Rejeitada: gera falso-positivo cruzado entre projetos, treina o usuário a ignorar `validate` ou a
   rodar com bypass — exatamente a erosão que D5 do ADR de markdown foi escrito para evitar do lado
   do hash de manifest.
2. **Proveniência do lado do home** (`~/.trackfw/plugins-provenance.json`), aceitando que não há
   detecção. Rejeitada: não é git-anchored — `ADR-2026-08-12` §2 já estabeleceu como doutrina aceita
   que artefato fora do repo "não aparece em lugar nenhum" do fluxo de PR/review, e é o mesmo
   raciocínio que fez `D4-bis` recusar ler o manifest do home na regra de skills. Colocar o ledger lá
   move a única fonte de verdade para fora do alcance da detecção que o resto do gate assume.
3. **Diretório de plugins por projeto** (ex. `.trackfw/plugins/`), que recuperaria o ramo (i) por
   completo. **Não é uma alternativa desta REQ** — reabriria a decisão de KG de manter o escopo
   global "para não quebrar quem já usa", que a tarefa marca como não-reaberta. Registro aqui como
   **recomendação para REQ futura**, não como redesenho: se o produto algum dia quiser detecção
   completa de instalação-sem-aprovação para plugins, o caminho é escopo por projeto, com o mesmo
   trade-off de fricção que D4 já pagou para skills.

**Veredito P1 (para o ADR):** proveniência do plugin vive em `.trackfw/thirdparty-provenance.json`
(ou arquivo irmão dedicado, decisão de nomenclatura do ML-0B), **chaveada por nome de plugin**, no
projeto. `trackfw validate` ganha checagem do **ramo (ii)** apenas — adulteração pós-aprovação dos
plugins que o próprio projeto declarou. O **ramo (i)** — detectar instalação nunca aprovada — **é
declarado ausente por desenho**, consequência direta de KG ter mantido o escopo global; não é
esquecimento, é o mesmo trade-off que D4-bis já pagou para `--scope global` de skills, só que aqui
não existe alternativa de escopo `project` para compensar.

---

## P2 — Modelo de ameaça do binário, e o que muda em relação ao markdown

**Veredito: quatro diferenças materiais, nenhuma coberta por mecanismo já existente no gate de
markdown.**

1. **Execução sem camada de interpretação.** Markdown malicioso precisa de um agente LLM induzido
   para decidir agir sobre ele — há julgamento entre o download e o efeito. Um binário, quando
   invocado, **executa direto**; não há intermediação.
2. **Persistência é por-máquina, não por-projeto.** Um único plugin malicioso instalado uma vez fica
   disponível para **todo** projeto que aquele usuário abrir naquela máquina — estritamente pior que
   o `--scope global` de D4-bis para skills, porque lá ainda existe a opção `project` como default
   seguro; aqui **não existe alternativa de escopo** (KG manteve global como único caminho).
3. **Alcançável pelo fallback de subcomando desconhecido.** `internal/commands/root.go:70-76`:
   qualquer argumento não reconhecido pelo cobra vira `RunPlugin(args[0], args[1:])`
   (`internal/commands/plugins.go:111-125`), que executa `exec.Command(path, args...)` sem
   revalidar nada — sem checksum, sem proveniência, sem confirmação. Um typo de comando, ou um
   agente induzido sugerindo `trackfw <nome-do-plugin-malicioso> <args>`, executa o binário.
4. **Roda com privilégio total do usuário**, herdando `stdin`/`stdout`/`stderr` do processo `trackfw`
   (`cmd.Stdin = os.Stdin` etc., `plugins.go:121-123`) — pode imitar prompts do próprio `trackfw`
   para phishing de credencial digitada pelo usuário, um vetor que markdown, por não executar, não
   tem.

**Veredito P2:** o gate de markdown não cobre nenhum dos quatro — são específicos de artefato
executável. O handshake de duas fases (quarentena → parecer → `chmod` tardio) fecha o vetor de
**instalação**; os quatro pontos acima descrevem por que o vetor de **execução pós-instalação**
(P7 aprofunda isso) permanece fora do escopo desta REQ, e deve ficar nomeado como tal.

---

## P3 — `chmod` tardio: o que compra e o que não compra

**Veredito: compra o fechamento do caminho de execução padrão (`RunPlugin`/fallback); não compra
nada contra um ator com escrita no workspace, nem contra payload interpretável sem bit de execução.**

**Compra:** com o binário em quarentena a `0600`, `exec.Command` sobre esse caminho falha com
`EACCES` — é uma propriedade real e testável (o teste de AC4 verifica exatamente isso). Enquanto o
`chmod 0755` não roda (fase 2, só após aprovação), o fallback de subcomando desconhecido do
`root.go` não consegue invocar o binário, mesmo que ele já esteja fisicamente no destino final.
Fecha o vetor 3 de P2 durante a janela entre download e aprovação.

**Não compra:**
- **Nada contra o agente induzido do ADR-2026-08-12.** Quem já tem `Bash` no workspace roda
  `chmod +x` no arquivo e o executa manualmente — a mesma doutrina que já vale para
  `credential_guard.mode`: "a bytes de distância" não é defesa contra quem tem shell.
- **Nada contra payload interpretável sem bit de execução.** `sh arquivo` ou `python arquivo` não
  precisam de `+x`. O nome do asset (`trackfw-plugin-<nome>-<os>-<arch>`) sugere binário compilado,
  mas nada no caminho de download hoje **verifica** que os bytes são de fato um executável nativo —
  ver P4, onde isso vira critério de recusa concreto (assinatura mágica de arquivo).

**Veredito P3:** `0600` em quarentena é controle de **acesso pelo caminho de código padrão**, não
controle de **conteúdo**. É o mesmo tipo de garantia que D8(a) do ADR de markdown já declara para a
quarentena de texto ("ausência de caminho de código, não bloqueio ativo") — aqui, adicionalmente,
há um bloqueio de SO real (permissão de execução), mas só até o primeiro `chmod` manual.

---

## P4 — O que o revisor precisa ver para dar parecer sobre um binário

**Veredito: o parecer sobre binário não pode ser "estes bytes são seguros" — só pode ser "estes
bytes vieram do publicador resolvido X e eu aceito esse publicador", e o artefato de revisão precisa
carregar sete campos concretos para isso ser mais que carimbo.**

### Por que o formato de quarentena diverge de D8(b) do ADR markdown

D8(b) embute o conteúdo **integral em base64** dentro do JSON de quarentena, para fechar um segundo
TOCTOU por indireção de arquivo. Para markdown (teto de 2 MiB) isso é barato. Para um binário de até
50 MiB (`maxPluginSize`, `plugins.go:15`), base64 infla ~33% — ~67 MiB dentro de um único JSON — não
é um formato de arquivo viável de versionar e revisar. **A quarentena de binário diverge por
necessidade** (AC2 exige justificar divergências, não proibi-las): os **bytes brutos** ficam no
próprio arquivo de quarentena nomeado pelo checksum (`<checksum>.bin`, `0600`), e um **JSON sidecar**
carrega os metadados. A propriedade self-verifying de D8(a) (nome do arquivo = hash esperado) é
preservada; o TOCTOU de "trocar o arquivo referenciado" que D8(b) fecha por embutir continua fechado
por D8(c): se alguém trocar os bytes do `.bin`, `sha256(arquivo) ≠ <checksum-no-nome>`, e a fase 2
recusa por construção — não é preciso reler o conteúdo para pegar a troca.

### Os sete campos que tornam o parecer mais que carimbo

1. **Repo resolvido** (`user/repo`) — o que `ResolveRepo` de fato retornou, não a string digitada.
2. **String original digitada pelo usuário**, lado a lado com a resolvida — ver P5: sem isto, o
   revisor não tem contra o que comparar "o usuário esperava X" e a auditoria vira nominal.
3. **URL de asset resolvida** (redigida, D6-bis) — de onde os bytes vieram de fato.
4. **Tag efetivamente resolvida** — nunca a string `latest`: se o revisor aprova "latest" e a fase 2
   busca de novo mais tarde, os bytes podem ter mudado silenciosamente. É preciso registrar a tag
   concreta (ex. `v1.4.2`) que o fetch de fato baixou.
5. **Tamanho e SHA-256** dos bytes brutos.
6. **Plataforma declarada** (`<os>-<arch>` do nome do asset).
7. **Checagem de assinatura de arquivo (magic bytes)** — ELF/Mach-O/PE vs. script (`#!`) vs.
   arquivo comprimido, comparado contra o `<os>-<arch>` esperado pelo nome do asset. Um mismatch
   (ex. asset nomeado `-linux-amd64` mas os bytes começam com `MZ`, cabeçalho PE do Windows) é
   critério de recusa concreto — o análogo binário do `marker_check` de D3: `pass|fail`
   pré-computado pela fase 1, **insumo, não veredito**, exatamente como D8(b) já declara para o
   `marker_check` de markdown.

**Veredito P4:** o artefato de revisão do binário carrega os sete campos acima; a quarentena diverge
de D8(b) por necessidade de tamanho (bytes brutos em arquivo nomeado por checksum + JSON sidecar, em
vez de base64 embutido), com a mesma propriedade self-verifying preservada. O parecer do `hades-tf`
sobre um binário certifica **proveniência aceita**, nunca **ausência de malícia** — isso precisa
estar escrito no template do artefato de revisão para não ser lido como garantia que não existe.

---

## P5 — Registry não-pinado permanece fora de escopo?

**Veredito: sim, a exclusão se sustenta — e se sustenta por causa de AC9, não apesar dela.**

O discriminador: um registry comprometido pode redirecionar `trackfw plugins add foo` para
`attacker/repo` em vez do `kgsaran/repo` esperado — mas com AC9 em vigor, a quarentena **registra
`attacker/repo` explicitamente**, nunca apenas `foo`. O ataque deixa de ser invisível: o revisor vê
o repo resolvido antes de aprovar. Sem AC9 (ou antes desta REQ), a confusão do registry seria
silenciosa; com AC9, vira um dado auditável no artefato de revisão.

**Uma lacuna a fechar no ML-0B, não uma reabertura de escopo:** o texto de AC9 diz "repo e URL de
asset **resolvidos**" — não diz "junto com o que o usuário digitou". Sem a string original ao lado
da resolvida (item 2 de P4), o revisor vê `attacker/repo` mas não tem como saber que o usuário
pediu `foo` e não `attacker/repo` — a auditoria fica nominal (ele vê *um* repo, não sabe se é o
*esperado*). Isto é compatível com o texto de AC9 como está (que já pede "resolvidos", não proíbe
adicionar o original) — é uma especificação a completar, não uma mudança de decisão.

**Achado técnico relacionado, para registrar (não para reabrir P5):** `pluginName := filepath.Base(base)`
(`plugins.go:193`) deriva o **nome do arquivo em disco** a partir do `repo` já resolvido pelo
registry. `filepath.Base` impede escapar do diretório (`../../etc/passwd`), mas **não** impede que
uma entrada do registry resolva para um nome que colida com um plugin **já aprovado** anteriormente
— `os.Rename(tmpPath, dest)` sobrescreve incondicionalmente. É exatamente por isso que o ramo (ii)
de P1 (checksum de adulteração) precisa ter dentes: é a defesa que pega essa sobrescrita, não a
resolução do nome em si.

**Veredito P5:** exclusão do registry sustenta, condicionada a AC9 carregar repo/URL resolvidos —
adicionar a string originalmente digitada ao lado é o complemento necessário para a condição se
cumprir de fato, e vai para o ML-0B como especificação, não como reabertura.

---

## P6 — Fraquezas pré-existentes do Node: severidade e escopo

**Veredito: quatro achados, todos DoS/robustez local — nenhum abre caminho de execução novo — mas
com pesos diferentes por qual AC os compele.**

| Achado | Severidade | Por quê | Entra nesta REQ? |
|---|---|---|---|
| `installPlugin`: `res.arrayBuffer()` bufferiza o corpo inteiro **antes** de checar `content.length` (`plugins.js:103-104`); a checagem prévia de `content-length` (linha 98) é sobre um header que a origem pode omitir ou mentir | **Mais alta dos quatro** — entrada remota não autenticada, alocação não limitada, checagem prévia contornável | **Sim — compelida por AC3** ("teto de tamanho aplicado por stream... idêntico entre Go e Node"). Não dá para entregar AC3 sem substituir este caminho por leitura em stream com `io.LimitReader`-equivalente (ex. `stream.pipeline` com contador, recusando ao ultrapassar o teto, antes de materializar o buffer completo) |
| `fetchRegistry`: `https.get` cru — sem checagem de status, sem timeout, sem teto de tamanho (`plugins.js:13-21`) | Média — mesma classe (DoS local via resposta grande/lenta), mas é caminho **diferente** (`search`, não `add`) | **Não obrigatório aqui** — não é compelido por AC3, que fala do download de plugin, não do registry. Registrado aqui como achado nomeado com severidade; pode entrar como bônus no ML-2A se o agente achar barato, mas a REQ correta é uma REQ de robustez do registry (compartilhada com o `Search` do Go, que também não tem timeout dedicado além do `httpClient` global) |
| Zero testes de plugins no Node (`npm/tests/` não tem arquivo de plugins) | Estrutural, não é vulnerabilidade em si | É o que torna as duas linhas acima **não verificáveis** hoje e continuaria assim depois do fix sem teste | **Sim, indiretamente** — não é AC3 em si, mas é pré-requisito para **provar** AC3: sem teste de tamanho-por-stream, a alegação "idêntico entre Go e Node" não tem como ser checada em CI. ML-2A precisa nascer com os primeiros testes de plugins do Node |
| Ausência de `resolveRepo` equivalente ao Go (nome puro se comporta diferente) | Baixa como vulnerabilidade, alta como divergência de paridade | Comportamento observável diferente entre CLIs para a mesma entrada | Já nomeado no roadmap (ML-2A); não é achado novo deste parecer |

**Veredito P6:** `arrayBuffer()`-antes-do-teto entra nesta REQ porque AC3 o exige; `fetchRegistry`
sem status/timeout/teto é achado nomeado de severidade média, roteado para REQ própria de robustez
do registry (Go também tem exposição parcial ali, embora com teto — `Search`, `plugins.go:100-116`,
tem `maxRegistrySize` mas herda o `httpClient` de 30s sem revalidação de redirect); zero testes de
Node é bloqueante para **provar** AC3, não apenas para corrigi-lo.

---

## P7 — A exceção do Python: defensável, ou caminho de fuga?

**Veredito: defensável para o gate de instalação — mas mais estreita do que a formulação da REQ
sugere, e o `docs/cli-parity.md` precisa dizer o porquê ou vai ler como "Python tem menos
superfície" quando na verdade tem uma superfície diferente e igualmente aberta.**

`pypi/trackfw/commands/plugins.py` confirma o mapeamento do roadmap: não há `download`/`add`, só
`list` (varre `$PATH`) e `run` (`shutil.which` + `subprocess.run`). Não há o que gatear no sentido
de "download de binário de terceiro" — construir um caminho de instalação só para ter algo a gatear
inverteria a REQ, adicionando o próprio vetor que ela existe para fechar. **Nisto o veredito do
roadmap está correto.**

**O que falta nomear:** o Python **executa** plugins por um mecanismo diferente e **mais amplo** —
qualquer executável `trackfw-*` em **qualquer diretório do `$PATH`** do usuário, não só
`~/.trackfw/plugins`. Isso não é coberto nem pelo modelo de quarentena (que assume o destino fixo
`~/.trackfw/plugins`) nem pela regra de detecção de P1 (que também assume esse destino fixo). Um
binário `trackfw-malicioso` em qualquer lugar do `$PATH` do usuário — nem precisa ter sido "instalado
pelo trackfw" — é descoberto e executado pelo `plugins run` do Python sem revalidação alguma.

**Simetria com Go:** o `RunPlugin` do Go (`root.go:70-76`, fallback de subcomando desconhecido)
também não revalida nada no momento da execução — o gate desta REQ cobre a **instalação**
(`plugins add`), não a **execução** (`RunPlugin`/`plugins run`). Ou seja: **o gate cobre instalação
em Go e Node; execução fica ungated nos três CLIs**, só que com superfícies de descoberta
diferentes (`~/.trackfw/plugins` fixo em Go/Node; `$PATH` inteiro em Python).

**Veredito P7:** a exceção do Python **é defensável para o gate de instalação**, exatamente como o
roadmap concluiu — **não é caminho de fuga do gate**, porque não há "fuga" de algo que nunca existiu
naquele CLI. Mas é uma **divergência não nomeada de superfície de execução**, e precisa constar em
`docs/cli-parity.md`: "o gate de instalação (quarentena + parecer + `chmod` tardio) cobre Go e Node;
a execução pós-instalação não é revalidada em nenhum dos três CLIs, e o Python tem superfície de
descoberta adicional via `$PATH` que os outros dois não têm." Revalidação de proveniência em tempo
de execução não é sandboxing (que a REQ já exclui de escopo) — registrado aqui como recomendação
para REQ futura, pelo canal de discordância que a tarefa autoriza, não como redesenho desta.

---

## Resumo executivo (para consumo do ML-0B)

| # | Veredito |
|---|---|
| P1 | Ramo (ii) — adulteração pós-aprovação — implementável via índice do projeto chaveado por nome de plugin + `installed_sha256`, checado por `validate` contra o arquivo local. Ramo (i) — instalação sem aprovação — **declarado estruturalmente ausente**, consequência de KG ter mantido escopo global (`~/.trackfw/plugins` é por-máquina, não por-projeto; enumerar o dir produziria falso-positivo entre projetos). **AC6 precisa ser reescrito para só nomear o ramo (ii)**. |
| P2 | Quatro diferenças materiais em relação a markdown: execução sem camada de interpretação; persistência por-máquina (pior que `--scope global` de skills, sem alternativa de escopo); alcançável pelo fallback de subcomando desconhecido (`root.go:73`); roda com privilégio total herdando stdio (vetor de phishing que markdown não tem). |
| P3 | `chmod` tardio (`0600` em quarentena) fecha o caminho de execução padrão (`RunPlugin`/fallback) durante a janela pré-aprovação; não fecha nada contra agente com `Bash` (`chmod +x` manual) nem contra payload interpretável sem bit de execução — critério de assinatura de arquivo (P4) cobre parcialmente esse segundo ponto. |
| P4 | Quarentena diverge de D8(b) por necessidade de tamanho — bytes brutos em arquivo nomeado por checksum + JSON sidecar, não base64 embutido — com a propriedade self-verifying preservada e o TOCTOU fechado por D8(c), não por indireção. Parecer certifica proveniência aceita, nunca ausência de malícia. Sete campos obrigatórios: repo resolvido, string original digitada, URL de asset resolvida, tag efetivamente resolvida (nunca `latest` puro), tamanho, SHA-256, checagem de assinatura de arquivo (magic bytes vs. `<os>-<arch>` do nome). |
| P5 | Exclusão do registry não-pinado se sustenta — e se sustenta por causa de AC9 (repo/URL resolvidos ficam auditáveis), não apesar dela. Falta especificar em AC9 que a string original digitada também é registrada ao lado da resolvida, para a auditoria deixar de ser nominal. |
| P6 | `arrayBuffer()`-antes-do-teto em `installPlugin`: severidade mais alta, **entra nesta REQ** (compelido por AC3). `fetchRegistry` sem status/timeout/teto: severidade média, **REQ separada** (caminho diferente, não compelido por AC3). Zero testes de plugins no Node: bloqueante para **provar** AC3, não só para corrigi-lo — ML-2A nasce com os primeiros testes. |
| P7 | Exceção do Python é defensável para o **gate de instalação** (não há caminho de download, não há o que gatear) — não é caminho de fuga. Mas é superfície **de execução** diferente e mais ampla (`$PATH` inteiro via `shutil.which`, não só `~/.trackfw/plugins`), não coberta em nenhum dos três CLIs pelo gate de instalação. `docs/cli-parity.md` deve nomear essa divergência de execução, não só a de instalação. |

**Flag adicional para o ADR do ML-0B:** AC6, como escrito no roadmap ("Detecção de plugin sem
proveniência"), descreve o ramo (i), que P1 declara ausente por desenho. Reescrever para "Detecção
de adulteração pós-aprovação dos plugins declarados pelo projeto (ramo ii); ausência de detecção de
instalação-sem-aprovação declarada e justificada (ramo i)" antes de virar decisão `Dn` no ADR.
