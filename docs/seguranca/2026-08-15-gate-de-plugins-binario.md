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

---

## Verificação pós-remoção (ML-3A, 2026-08-16)

> Autor: `hades-tf` (Security Reviewer) | Barreira final da
> `docs/roadmaps/wip/ROADMAP-2026-08-15-remocao-do-subsistema-de-plugins-do-trackfw.md`
> ADR verificado: `docs/adr/ADR-2026-08-15-remocao-do-subsistema-de-plugins-em-vez-de-gate-de-binario-de-terceiro.md`
> Branch: `refactor/remocao-do-subsistema-de-plugins-do-trackfw`, `HEAD` no momento da verificação.

Este apêndice não reavalia a decisão de remoção — já aceita no ADR — apenas verifica, **executando**,
que ela foi implementada por completo nos 3 CLIs, sem resíduo e sem regressão lateral. Metodologia:
`grep` estrutural nos 3 stacks para localizar todo caminho de rede/execução remanescente, seguido de
leitura de cada ocorrência para classificar propósito, e uma prova executável para o item mais crítico
(D9), não aceita por leitura de diff.

### 1 — Caminho de download de binário executável

**Veredito: eliminado.** Verificado nos 3 stacks, não só no Go:

- **Go** — `internal/thirdparty/fetch.go` restringe `Content-Type` a `allowedContentTypes`
  (`text/plain`, `text/markdown`, `text/x-markdown`; `fetch.go:43-100`), com comentário explícito
  (`fetch.go:19-20`) documentando que o antigo caminho de download de asset binário foi removido.
  Os demais usos de `net/http` no repo são servidor (`internal/serve/*`, `internal/server/server.go`
  — serve o dashboard, não faz fetch de terceiro) ou cliente de API de sync (`internal/sync/jira.go`,
  `internal/sync/linear.go` — Jira/Linear, dados textuais de issue, não artefato executável).
- **Node.js** — `npm/src/thirdparty/fetch.js:28` declara `ALLOWED_CONTENT_TYPES = new Set(['text/
  plain', 'text/markdown', 'text/x-markdown'])`, comentado como espelho literal do allowlist do Go.
  Os demais `fetch(...)` no repo Node são `npm/src/serve/static/app.js` — chamadas do dashboard a
  `/api/board`, `/api/attention`, `/api/chain`, `/api/metrics`, `/api/file` (mesma origem, não
  externas).
- **Python** — `pypi/trackfw/thirdparty/fetch.py:47` declara o mesmo `ALLOWED_CONTENT_TYPES` e
  recusa por `_content_type_allowed` (linha 70, aplicado em 119-123). `urllib` fora desse arquivo
  aparece em `commands/sync.py` (API Jira/Linear, mesmo caso do Go) e em módulos de parsing de URL
  (`urlparse`) sem rede.

`grep -rn "download\|release\|assets" internal/commands/update.go npm/src/commands/update.js
pypi/trackfw/commands/update.py pypi/trackfw/commands/update_harness.py` retorna **zero**
ocorrências — o autoupdate do trackfw (candidato óbvio a esconder um caminho de "baixar e dar chmod
num binário") não tem nenhum mecanismo de download de asset; `update_harness.py:564` comenta
explicitamente que o alvo "does NOT attempt a subprocess version probe". Os pacotes
`internal/plugins/`, `npm/src/commands/plugins.js`, `pypi/trackfw/commands/plugins.py` não existem
mais no filesystem (confirmado por `ls`, não por grep negativo). A ocorrência de `plugins:` em
`internal/serve/static/app.js`/`npm/.../app.js`/`pypi/.../app.js` (linhas 805/860) é a chave de
configuração nativa do Chart.js (`options.plugins.legend`), sem relação com o subsistema removido —
falso positivo do grep, verificado por leitura de contexto.

### 2 — Caminho de execução de binário de terceiro (indireto incluído)

**Veredito: eliminado no caminho que importava (fallback de comando desconhecido); os `exec.Command`/
`subprocess`/`child_process` remanescentes são todos toolchain fixa do projeto, não terceiro
arbitrário.**

Inventariei todo `exec.Command`/`os/exec` (Go), `child_process`/`spawn`/`execFile`/`exec(` (Node),
`subprocess`/`shutil.which`/`os.system` (Python) fora de `_test`. Nenhum resolve um nome vindo de
artefato baixado da rede ou de `~/.trackfw/plugins`; todos resolvem para um conjunto fechado,
hardcoded no código:

| Comando invocado | Onde | Nome vem de |
|---|---|---|
| `git` | `ship.go`/`branch.go`/`commit.go`/`barrier.go`/`validator_git_exec.go`/forge `resolve.go` e equivalentes Node/Python | literal `"git"` |
| `gh`/`glab`/`az` | `ship.go:defaultExecForgeCLI`, `ship/runner.js`, `ship/runner.py` | `adapter.CLIName`, literal fixo por forge dentro de `NewAdapter` (`internal/forge/adapter.go:45,53,61`, confirmado por leitura — `CLIName` nunca é atribuído a partir de `resolution.Forge` ou de qualquer entrada de usuário/rede, só dos 3 literais `"gh"`/`"glab"`/`"az"` mais `""` para bitbucket/manual) |
| `npm`, `npx husky`, `lefthook install` | `internal/discover/discover.go`, `npm/src/commands/discover.js`, `pypi/trackfw/commands/discover.py:380,385,437` (`subprocess.run(["npm",...])`, `["npx","husky","init"]`, `["lefthook","install"]`, lidos linha a linha) | literais, para instalar hooks de governança do próprio trackfw |
| `sh -c <comando>` | `internal/commands/barrier.go:386` (`runGateCommand`) | comando de um bloco `` ```bash `` do **roadmap do próprio projeto** — artefato versionado no repo, revisado por PR, não artefato de terceiro baixado em runtime; fora do escopo desta remoção (é o "gates da wave" do `trackfw barrier`, mecanismo pré-existente e não tocado pelo ADR) |
| `open`/`start`/`xdg-open` | `serve.go`/`serve.js`/`serve.py` | literal por plataforma, argumento é `http://localhost:<port>` com `port` sempre numérico (`parseInt(...,10)\|\|8080` em Node; `int` em Go/Python) — sem espaço para injeção de shell |

Nenhuma dessas entradas é um binário `trackfw-*` de terceiro nem um artefato descoberto por varredura
de diretório. O `runGateCommand` do `barrier.go` é o único item que executa um "comando arbitrário",
mas a fonte é um arquivo de roadmap sob controle de versão e revisão do próprio projeto — categoria
diferente de "binário de terceiro baixado", e pré-existente à REQ de plugins (não nasceu nem foi
alargado por esta remoção). Registro aqui por completude do item 2, não como achado bloqueante.

### 3 — Débito D9 (fallback de comando desconhecido) — provado por execução

**Veredito: fechado, provado por execução real nos 3 CLIs, não por leitura de diff.**

`internal/commands/root.go` não contém mais nenhum `RunPlugin`/fallback: `newRootCmd()` registra uma
lista fechada de subcomandos (`root.go:37-63`) e qualquer argumento não reconhecido pelo cobra vira
erro canônico via `formatUnknownCommandError` (`root.go:94`, `root.go:130-143`) — nunca
`exec.Command`. Confirmado com `grep -rn "RunPlugin\|runPlugin\|run_plugin"` nos 3 stacks: zero
ocorrências.

Prova executável: criei um binário real `trackfw-vaildate` (script `#!/bin/sh` que imprime uma string
sentinela `EXECUTOU_PLUGIN_MALICIOSO_<CLI>`), coloquei-o no início do `$PATH`, e rodei `trackfw
vaildate` nos 3 CLIs a partir do `HEAD` da branch:

- **Go** — `go build -o /tmp/trackfw-go-bin ./cmd/trackfw` e execução: `Error: unknown command
  "vaildate" for "trackfw" / Did you mean "validate"? / Run 'trackfw --help' for usage.`, exit 1.
- **Node.js** — `node npm/bin/trackfw vaildate`: mensagem byte-idêntica, exit 1.
- **Python** — `python3 -c "sys.argv=['trackfw','vaildate']; from trackfw.cli import main;
  sys.exit(main())"` com `PYTHONPATH=pypi`: mensagem byte-idêntica, exit 1.

Em nenhum dos três a string sentinela apareceu no stdout/stderr — o binário malicioso nunca foi
invocado. Isto é exatamente o cenário que `internal/commands/root_test.go`
(`TestFormatUnknownCommandError_PluginsIsGone` e o teste de falsificação de `vaildate`) e os
equivalentes `npm/tests/unknown-command.test.js` / `pypi/tests/test_commands_basic.py::
TestUnknownCommand` já cobrem — a verificação aqui é a mesma prova, rodada de novo de forma
independente pelo revisor de segurança, fora do harness de teste do implementador, sobre o `HEAD`
real da branch.

### 4 — `chmod` de artefato de terceiro e escrita em `~/.trackfw/plugins`

**Veredito: eliminado.** `grep -rn "\.trackfw/plugins\|trackfw-plugins\|TRACKFW_PLUGIN"` nos 3 stacks
retorna zero ocorrências. Os `chmod`/`os.chmod`/`fs.chmodSync` remanescentes nos 3 stacks são todos de
domínios não relacionados a plugin: chaves de identidade (`internal/identity/identity.go`), o próprio
gate de skills markdown (`internal/integrations/manager.go`, `internal/thirdparty/quarantine.go` e
equivalentes Node/Python — o gate que este parecer já analisou em P3/P4, não tocado por esta remoção),
e scripts de hook/attention gerados pelo próprio trackfw (`update.go`/`update.py`/`init_gen.py`,
`0755` em arquivos que o trackfw mesmo escreveu, não terceiro).

### 5 — Superfície ampliada do Python (`$PATH` inteiro)

**Veredito: eliminada, confirmado pela mesma prova executável do item 3.** `pypi/trackfw/commands/
plugins.py` — que continha `list` (varredura de `$PATH`) e `run` (`shutil.which` + `subprocess.run`)
— não existe mais no filesystem. `grep -n "plugins" pypi/trackfw/cli.py` só retorna o comentário
referenciando o ADR de remoção; nenhum comando `plugins`/`run` está registrado. A prova de D9 (item 3)
já cobre o caso mais amplo: o binário sentinela estava no `$PATH`, que é exatamente a superfície que o
`plugins run`/`list` do Python varria — e mesmo assim não foi descoberto nem executado, porque o
mecanismo de descoberta em si foi removido, não apenas o comando `add`.

**Nota não-bloqueante:** `pypi/build/lib/trackfw/commands/plugins.py` é um artefato de build local
obsoleto (`pypi/build/`, `mtime` de 13/06, coberto por `pypi/.gitignore:11` — `pypi/build/` — e
**não rastreado pelo git**, confirmado via `git ls-files pypi/build` retornando vazio). Não é
publicado, não é parte do source tree, não afeta o pacote distribuído. Registrado por completude, não
altera o veredito — recomendo `rm -rf pypi/build` como housekeeping local, sem necessidade de ML.

**Nota não-bloqueante adicional:** `pypi/trackfw/thirdparty/fetch.py:32` mantém um comentário
("2 MiB — deliberately smaller than the plugin binary download cap") referenciando um teto de
download de plugin que não existe mais em lugar nenhum do código — comentário órfão, cosmético, não
um caminho de código. Reportado para quem tocar o arquivo depois, não bloqueia esta barreira.

### 6 — Regressão por caminho lateral (`update`, `init`, `serve`, `sync`, `ship`)

**Veredito: nenhuma regressão.** `grep -n "exec\." internal/commands/{update,sync,init,serve}.go`
não retorna nenhuma ocorrência — nenhum desses comandos executa processo externo no Go. Os
equivalentes Node (`update.js`, `sync.js`, `init.js`, `serve.js`) e Python (`update.py`,
`update_harness.py`, `sync.py`, `init.py`, `serve.py`) só invocam `open`/`start`/`xdg-open` com URL
local numérica (`serve`) — já coberto no item 2, sem relação com terceiro. `ship` (nos 3 CLIs) só
invoca `git` e o CLI de forge resolvido de tabela fixa — também já coberto no item 2. Nenhum desses
comandos ganhou, herdou ou reintroduziu um caminho de descoberta/execução de binário `trackfw-*` ou
de terceiro.

### 7 — Veredito explícito

**O vetor foi eliminado, não movido nem reduzido.** Os quatro pontos que P2 (seção original deste
parecer) apontou como não cobertos pelo gate de markdown — execução sem interpretação, persistência
por-máquina, alcançável pelo fallback de comando desconhecido, execução com privilégio total herdando
stdio — deixam de ser relevantes porque **não existe mais nenhum caminho no código que baixe, instale
ou execute um binário de terceiro**, nos 3 CLIs, confirmado por leitura exaustiva + prova executável
do caso mais crítico (D9). Os `exec`/`subprocess`/`chmod` que restam pertencem a três domínios
legítimos e não tocados por esta REQ: (a) a própria toolchain do trackfw (`git`, `gh`/`glab`/`az`,
`npm`/`npx`/`lefthook` para hooks), (b) o gate de skills markdown (fora de escopo, já analisado em
outro parecer), (c) o `runGateCommand` do `trackfw barrier`, que executa comandos de um roadmap
versionado no próprio repo — categoria distinta de "binário de terceiro baixado em runtime".

**Sem resíduo bloqueante.** O único achado registrado (`pypi/build/lib/trackfw/commands/plugins.py`,
item 5) é um artefato de build local, não rastreado, não publicado — não bloqueia merge.

**Libero esta barreira.** Nenhum item impede o merge do ML-3A. Não modifiquei nenhum arquivo de
código de produto nem de teste; este apêndice é o único artefato produzido, junto com a entrada em
`docs/agents-working-context.md`.
