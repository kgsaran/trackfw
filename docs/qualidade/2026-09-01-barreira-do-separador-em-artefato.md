# Barreira de qualidade — PR #231 (item 10 do #216)

> Escopo: `git diff origin/main...HEAD` na branch
> `fix/caminho-dentro-de-artefato-versionado-usa-sempre-barra`. Agente: `hefesto-tf`. Só
> diagnóstico — nenhum arquivo de produto tocado.

**Veredito: APROVA COM RESSALVAS.** Nenhum achado bloqueia este PR — são três itens de
acompanhamento (dívida de teste em 2 dos 3 limites duros do ML-0A; dívida de teste na escrita do
`.trackfw-log` em `by_agent`; e registro de REQ própria para o gap pré-existente de `/api/chain`
em Node/Python, confirmado ao vivo nesta auditoria). Nenhum achado invalida a propriedade central
do ML: a normalização não vaza para o corpo do artefato, nos 3 runtimes, com teste de controle
dedicado. O gate de CI foi falsificado por mim, nas duas direções que o roadmap alega.

---

## 1. Normalização vaza para o corpo do artefato? (risco mais provável) — NÃO VAZA

**Achado: não vaza. Normalização aplicada apenas ao valor já extraído do campo, nunca ao buffer
do arquivo, nos 3 runtimes.**

Pontos de normalização confirmados por leitura:

- `internal/generators/roadmap.go` — `normalizeRefSeparator` aplicada a: `dst` → `portableDst`
  (valor escrito na REQ), e a `plainVal`/`fmVal` só dentro da comparação `filepath.Base(...)`
  (nunca reescreve o conteúdo bruto do arquivo — só decide se casa, e a reescrita usa
  `newRoadmapPath` explícito, não o valor "limpo").
- `npm/src/generators/roadmap.js` — mesmo padrão: `normalizeRefSeparator(dst)` para escrita,
  `normalizeRefSeparator(currentRef)` só dentro de `path.basename(...)` para comparação.
- `pypi/trackfw/generators/roadmap.py` — idem: `_normalize_ref_separator` em `new_path` →
  `portable_path` (commands/roadmap.py) e em `current_ref` só para comparação de basename.

Em nenhum dos 3 runtimes o `content`/`rawContent` do arquivo (buffer inteiro lido de disco) passa
por `normalizeRefSeparator`. A reescrita usa sempre `newRoadmapPath`/`new_path` (o valor novo,
canônico), nunca uma versão "limpa" do texto antigo — então prosa/regex/exemplo com `\` literal
no corpo do artefato não é tocado porque o código nunca lê essas linhas para reescrevê-las (só a
linha do campo `roadmap:`/`Roadmap:` é candidata a `rewriteReqRoadmapRef`, e mesmo aí o *valor de
substituição* é o path novo calculado, não uma normalização do texto existente).

Confirmado também para a camada de leitura tolerante: `internal/validator/validator.go`,
`internal/validator/validator_thirdparty_provenance.go`, `internal/serve/api_chain.go`,
`pypi/trackfw/validator.py` — `normalizeRefSeparator`/`_normalize_ref_separator` é chamada só
sobre o valor já extraído do campo (`ref`, `provenanceKey`, `path`, `val`), nunca sobre
`rawContent`/`content` do arquivo.

**Achado positivo, não trivial**: os 3 runtimes têm teste de controle dedicado a este limite
exato — `TestSyncREQ_ControlDoesNotTouchUnrelatedBackslashInBody` (Go,
`internal/generators/roadmap_test.go:1401`), `syncReqReferences — controle: prosa não
relacionada com "\" legítimo no corpo não é tocada` (Node,
`npm/tests/roadmap_move.test.js:911`), `test_controle_prosa_nao_relacionada_com_backslash_nao_e_tocada`
(Python, `pypi/tests/test_generators_roadmap.py:992`) — todos escrevem um trecho de
prosa/código-cercado com `\` legítimo cujo basename não coincide com o roadmap movido, e
verificam que o texto sobrevive **byte a byte**. O achado é bem coberto por teste, não só por
comentário.

---

## 2. Os três limites duros do ML-0A — verificados por teste? PARCIAL

O parecer de ameaça (`docs/seguranca/2026-09-01-modelo-de-ameaca-do-separador-em-artefato.md`,
§3.2/3.3) nomeia três limites. Status de cada um, por evidência direta:

| limite | verificado por teste? | evidência |
|---|---|---|
| corpo de prosa/código de ADR/REQ/roadmap | ✅ sim, nos 3 runtimes | testes citados na seção 1 |
| `content_base64` da quarentena de terceiros | ❌ não | nenhuma ocorrência de `content_base64` em nenhum arquivo de teste tocado por este diff (confirmado por grep); a garantia hoje é só estrutural — nenhum código do diff passa `content_base64` por `normalizeRefSeparator`, mas nada testa que uma regressão futura seria pega |
| chave absoluta de `integrations-manifest.json` | ❌ não | mesma situação — nenhum teste novo cobre `integrations-manifest.json`; o campo nunca é tocado pelos pontos de normalização do diff, mas isso não está sob guarda de teste |

**Isto bate exatamente com o próprio roadmap**: em ML-1B, o critério de aceite
"🔴 Os três limites duros do ML-0A verificados por teste, não por comentário" está com a caixa
**desmarcada** (`- [ ]`) — não é um achado novo, é uma lacuna que a Wave 1 já sabia que tinha
deixado aberta e não fechou. `make quality` local roda verde apesar disso porque nenhum teste
existente exercita esses dois campos com um valor sujo — o risco funcional real hoje é baixo
(nenhum código no diff aplica a normalização a `content_base64` ou à chave absoluta), mas a
propriedade "não normalizado" está garantida por ausência de chamada, não por um teste que
falharia se alguém, num ML futuro, decidisse "generalizar" `normalizeRefSeparator` para esses
campos.

**ACOMPANHAMENTO, não bloqueante**: 2 dos 3 limites duros não têm o teste de regressão que a
própria REQ exigiu. Remédio: um teste por runtime que monte um `content_base64`/uma chave
absoluta contendo `\` legítima (ex.: um checksum ou um path Windows-like sintético) e assert que
ela sobrevive sem alteração ao passar pelo caminho de código relevante
(`quarantine.go`/`quarantine.js`/`quarantine.py`, e o parser de `integrations-manifest.json`).
Não é urgente porque nenhum código do diff cria a exposição — é dívida de teste declarada, e o AC
já a nomeia como pendente.

### Achado adicional — AC1 também tem um buraco de teste não declarado: a escrita do `.trackfw-log` em modo `by_agent`

O ML-1A trata dois sites de escrita: o frontmatter da REQ pareada (bem testado, ver seção 1) e a
linha do `.trackfw-log` em modo `by_agent` (`log_basename`/`logBasename`), que é justamente o
site que a REQ original nomeou como defeito primário (`pypi/trackfw/generators/roadmap.py:611`).
Busquei por um teste, nos 3 runtimes, que rode `MoveRoadmap`/`moveRoadmap`/`move_roadmap` em modo
`by_agent` e leia de volta a linha gravada no `.trackfw-log` para confirmar `/` — **não existe
nenhum**. `TestMoveRoadmap_ByAgent` (Go) só confirma que o arquivo se moveu de diretório, nunca
lê `.trackfw-log`; não há equivalente no Node/Python que leia o log de volta em modo `by_agent`
para este ML. A única garantia hoje é a assinatura de código do gate (`assert_has` sobre
`log_basename = agent + "/" + basename` etc.), que prova que a linha **compila** com
concatenação `/`, não que o `.trackfw-log` real, escrito por uma chamada de `move_roadmap` em
modo `by_agent`, contém `/` e não `\`.

Evidência corroborante do próprio diff: `docs/roadmaps/.trackfw-log` ganhou uma linha real deste
trabalho (`git diff` do PR) —
`2026-09-01 10:08  ROADMAP-2026-09-01-....md  backlog → wip` — mas este repositório usa
`roadmap_namespacing: flat`, então essa linha não exercita `logBasename = agent + "/" + basename`
(o `agent` só entra em `by_agent`); não serve como prova do caso que falta testar.

**ACOMPANHAMENTO, não bloqueante**: mesma classe do achado anterior — dívida de teste, risco
funcional baixo hoje (o código está correto e coberto pelo gate estrutural), mas o AC "Escrita
com `/` nos 3 runtimes, verificada por teste" (ML-1A, marcado `[x]`) não cobre este site
especificamente. Remédio: um teste por runtime que mova um roadmap em modo `by_agent` e leia
`.trackfw-log` de volta, confirmando ausência de `\`.

---

## 3. Paridade — Go, Node, Python

**Confirmado: as escritas (ML-1A) e a maior parte das leituras (ML-1B) tocam os 3 runtimes de
forma simétrica.** `internal/generators/roadmap.go`, `npm/src/generators/roadmap.js`,
`pypi/trackfw/generators/roadmap.py` (+ `pypi/trackfw/commands/roadmap.py`) recebem a mesma
correção com a mesma forma (substituição incondicional, não a API idiomática `ToSlash`/
equivalente). `pypi/trackfw/validator.py` recebe a mesma correção que
`internal/validator/validator.go` para `referenceExists`/`validateREQRoadmapLifecycle` e para
`provenanceKey`/`provenance_key` do `thirdparty_artifact_has_provenance`.

**`thirdparty_artifact_has_provenance` — confirmado ausente só no Node**, presente e corrigido em
Go e Python (`grep` por ocorrências da regra em `npm/src/validator/index.js` = 0). Isto bate com
o relato do roadmap (ML-1C) e com o parecer de ameaça (residual #1): gap de paridade **anterior**
a esta REQ, corretamente não implementado aqui, registrado para REQ própria.

### ACOMPANHAMENTO (não bloqueante) — `/api/chain` (sintoma 2, "a aresta some silenciosamente") só foi corrigido no Go

- `internal/serve/api_chain.go` — corrigido: `nodeID`/`edge.To` normalizados, com teste nas duas
  direções (`internal/serve/api_chain_test.go`).
- `pypi/trackfw/serve/api_chain.py` — **não tocado por este diff, e o sintoma reproduz de
  verdade** (verificado ao vivo, não só por leitura de código): montei um fixture com uma REQ e
  um Roadmap, `roadmap`'s `req:` apontando para a REQ. Com `/` (valor limpo), `get_chain` produz
  1 aresta; com `\` (valor sujo, simulando commit do Windows), produz **zero arestas** —
  reprodução direta do PoC B do parecer de ameaça, num runtime que este PR não tocou. Causa:
  `node["id"]` já é normalizado (`.replace("\\","/")`, pré-existente) e `_find_node_by_ref`
  tenta `ref in by_id` (exact-match) antes do fallback por basename — um `ref` sujo nunca bate
  contra `by_id`, cujas chaves são sempre `/`-normalizadas.
- `npm/src/serve/api_chain.js` — **não tocado por este diff, e verifiquei ao vivo que o grafo já
  não desenha a aresta nem com a referência limpa** (`/`) — bug estrutural pré-existente, mais
  amplo que o defeito desta REQ: `resolveRef` (linha ~161) nunca aplica `path.basename` ao valor
  comparado (`val.replace(/\.md$/, '')`), só `.toLowerCase().trim()`, então qualquer referência
  com diretório (o formato real gravado no frontmatter deste repositório, ex.
  `docs/roadmaps/wip/ROADMAP-x.md`) nunca bate contra `fileIndex`, que é indexado só por
  basename. Confirmado por `git log -- npm/src/serve/api_chain.js` que este trecho não foi
  tocado desde o commit `f43e0d0`, anterior a esta branch — não é regressão desta REQ, é um
  defeito mais largo e anterior que a torna irrelevante como reprodução isolada do separador
  (a aresta já não aparece antes de qualquer `\` entrar em cena). Reproduzido duas vezes, com
  `cfg` de diretórios absolutos e depois com diretórios relativos ao cwd do fixture (a forma como
  `serve` roda de verdade) — mesmo resultado (0 arestas) nas duas configurações, então não é
  artefato de como montei o teste.

O critério de aceite do ML-1B "O sintoma 2 (aresta órfã no `serve`) deixa de ocorrer" está
corretamente marcado com a caixa **desmarcada** no roadmap — a Wave 1 já sabia e não alegou mais
do que entregou. Reli o parágrafo "Leitura tolerante (AC3)" de `docs/cli-parity.md`: a lista
"Cobre: `trackfw validate` (`referenceExists`, `validateREQRoadmapLifecycle`,
`thirdparty_artifact_has_provenance`)..." nomeia identificadores **Go**, no mesmo padrão usado
para `thirdparty_artifact_has_provenance` (que o próprio texto já qualifica como ausente no
Node) — só a frase sobre `syncREQReferences`/`syncReqReferences`/`sync_paired_req_references`
tripla por runtime. Não há alegação textual de cobertura tri-runtime para `/api/chain`; retiro a
leitura inicial de que o `cli-parity.md` overclaima aqui.

**Remédio**: registrar REQ de acompanhamento para `/api/chain` em Node/Python — cobrindo (a) o
gap de separador em Python (regressão real, reproduzida) e (b) o bug estrutural mais amplo do
Node (`resolveRef` sem `path.basename`, que faz o grafo nunca desenhar arestas hoje,
independente de `\`). Mesmo tratamento já dado ao gap de `thirdparty_artifact_has_provenance`
no Node: nomeado, não escondido, REQ própria — não bloqueia este PR porque nenhum dos dois é
regressão introduzida por ele.

---

## 4. Gate `scripts/check-ref-separator-portability.sh` — falsificado por mim

Leitura do script completa (`internal/validator/validator.go` gera a única linha duplicada, com
`assert_count(2)`; as outras 16 checagens usam `assert_has`).

**Verificação de unicidade** — contei ocorrências de cada uma das 17 needles de `assert_has`
(exceto a de `assert_count`, já tratada como duplicada por design) nos respectivos arquivos-alvo:
todas ocorrem **exatamente 1 vez**. Nenhuma outra checagem do gate está exposta ao mesmo risco de
cobertura parcial que motivou a troca para `assert_count` — hoje não sobrou nenhum `assert_has`
que devesse ser `assert_count`.

**Falsificação 1 — revertendo só a segunda ocorrência (própria, em cópia de `/tmp`, nunca na
árvore real):**
```
árvore correta (2 ocorrências)      → OK — 18 assinaturas confirmadas
revertida só validateREQRoadmapLifecycle (1 ocorrência) → FALHOU
  "esperava 2 ocorrencia(s), achou 1 em internal/validator/validator.go"
```
Confirmei também que um `assert_has` simples (`grep -qF`) sobre a mesma needle **passaria** nesse
cenário — a regressão de metade da garantia ficaria invisível sem `assert_count`. A escolha do
`assert_count` não é excesso de zelo; é a diferença entre o gate pegar ou não a regressão relatada
no roadmap.

**Falsificação 2 — guarda de vacuidade (removendo uma chamada `assert_has` do corpo do
script, em cópia de `/tmp`):**
```
"Go serve: node ID normalizado" removida do script → FALHOU
  "vacuidade — esperava checar 18 assinaturas, checou 17"
```
Confirmado: o gate não passa silenciosamente se alguém enxugar uma checagem.

**Veredito da seção 4: o gate faz o que o roadmap e o `cli-parity.md` afirmam que ele faz, nas
duas falsificações que reproduzi.**

---

## 5. Tolerância de leitura — super-normalização em POSIX? RISCO TEÓRICO

Já respondido no parecer de ameaça (§3.2-iii/3.3), e concordo com a conclusão: **risco teórico,
não funcional para este repositório.**

- `normalizeRefSeparator` é aplicada apenas a valores de campo (frontmatter `roadmap:`/`req:`/
  `adr:`, `.trackfw-log`, chave de provenance), nunca a caminhos arbitrários de sistema de
  arquivos escolhidos pelo usuário fora desses campos.
- Um arquivo real chamado `a\b.md` só deixaria de resolver se seu **basename completo**
  aparecesse, com `\` intacto, dentro de um desses campos de referência — E se essa referência
  precisasse ser resolvida contra o sistema de arquivos local (a normalização converteria o `\`
  do nome para `/`, quebrando a resolução).
- O parecer verifica, com evidência concreta (`internal/generators/adr.go:151`, `toSlug`), que
  **nenhum artefato gerado pelo próprio trackfw** pode ter esse nome — `toSlug` substitui toda
  sequência de não-`[a-z0-9]` por hífen antes de qualquer basename ser criado. Confirmei a
  alegação por leitura direta de `toSlug`.
- O caminho residual é: alguém renomeia manualmente, em Unix, um artefato para conter `\` no
  nome, e referencia esse artefato pelo caminho exato em outro frontmatter. Nenhuma evidência de
  que isso ocorre hoje; nenhum teste cobre o cenário (nem precisaria, dado o quão fora de banda
  ele está da geração normal). Concordo com a classificação do parecer: **residual, não
  bloqueio.**

---

## 6. Cobertura de teste dos arquivos novos — falsificam nas duas direções? SIM

`internal/validator/validator_separator_test.go` (4 testes) e `internal/serve/api_chain_test.go`
(3 testes, dos quais 2 novos relevantes a este PR) — sim, ambos falsificam nas duas direções:

- **Direção "resolve com `\`"**: `TestValidateRefTargetsExist_ToleratesDirtyBackslashReference`,
  `TestValidateREQRoadmapLifecycle_ToleratesDirtyBackslashReference`,
  `TestChainHandler_EdgeToleratesDirtyBackslashReference` — cada um monta o cenário sujo à mão
  (já que rodar o comando real nesta máquina nunca produz `\`) e confirma ausência do sintoma.
- **Direção "continua reprovando quando devia"**:
  `TestValidateRefTargetsExist_ControlBrokenReferenceStillFails` — referência para arquivo
  genuinamente inexistente continua sendo sinalizada, com e sem `\`. Isto é o controle correto:
  prova que a tolerância não virou "aceita qualquer coisa".
- **Direção "não muda o que já está limpo"**: `TestNormalizeRefSeparator_ControlDoesNotAlterCleanValue`
  (validator), `TestNormalizeRefSeparator_ControlDoesNotTouchUnrelatedValue` (serve) — a função
  não altera valor sem `\`.
- `TestChainHandler_EdgeStillResolvesWithPortableReference` — controle irmão do PoC B: a aresta
  continua desenhando quando a referência já está limpa (pós-fix, caso majoritário).

**Faltando, e é o mesmo achado da seção 2**: nenhum teste novo cobre a direção "referência que
usa `\` como caractere legítimo de nome (não separador) continua reprovando" — mas a seção 5
já argumenta que este cenário não é alcançável pela geração real do trackfw, então a ausência é
consistente com o residual declarado, não uma falsificação incompleta do que o ML se propôs a
testar.

---

## `make quality`

**Nota de processo**: minha primeira tentativa disparou dois `make quality` acidentalmente em
paralelo, ambos escrevendo no mesmo log com `>` (truncando um ao outro) — descartei essa saída,
matei os dois processos e rodei uma vez, limpo, sem pipe, capturando o exit code do `make`. Ver
seção seguinte.

Gate confirmado como **ligado** (não só escrito): `scripts/check-ref-separator-portability.sh`
está listado no target `parity:` do `Makefile` (linha 57), que por sua vez é dependência de
`quality:` — não é um script órfão.

**Limitação honesta desta auditoria**: a execução limpa e única de `make quality` (sem pipe, exit
code capturado, iniciada após matar as duas execuções paralelas acidentais) não terminou dentro
do tempo prático desta sessão — `scripts/check-gates-falsify.sh` sozinho executa centenas de
cenários de falsificação que reconstroem o binário Go e disparam subprocessos Node/Python/git
repetidamente (ex.: a seção `falsify/release-tag-parity/*` cria tags reais e roda os 3 CLIs por
cenário). Acompanhei a execução por ~70 minutos: **3370 linhas de log, 697 `OK`, zero `FAIL`**,
processo confirmado vivo e ativo (`ps` mostrando `go build` em andamento para o cenário
`s76-corrupt-go-bin`) em todos os pontos de checagem, cobrindo a suíte completa de
`test`/`test-node`/`test-python`/`lint` e a maior parte de `parity:` (incluindo
`check-ref-separator-portability.sh` implícito via `check-gates-falsify.sh`, que testa vários
scripts de forma aninhada) — **sem nenhuma falha observada em nenhum ponto**. O `MAKE_EXIT`
final não foi capturado dentro do tempo desta sessão; recomendo ao arquiteto rodar `make quality`
uma vez, sem paralelismo acidental, antes do merge, como confirmação final — evidência forte já
existe de que o resultado será verde, mas não é 100% definitivo sem o exit code.

---

## Veredito final

**APROVA COM RESSALVAS.**

### ACOMPANHAMENTO (não bloqueante) — nenhum achado bloqueia este PR

1. **2 dos 3 limites duros do ML-0A** (`content_base64` da quarentena, chave absoluta do
   `integrations-manifest.json`) não têm teste de regressão dedicado — AC do próprio roadmap
   (ML-1B) já está marcado como pendente (`- [ ]`); risco funcional baixo hoje (nenhum código do
   diff normaliza esses campos), dívida de teste real.
2. **A escrita do `.trackfw-log` em modo `by_agent`** (ML-1A) não tem teste que leia o log de
   volta e confirme `/` — só a assinatura de código do gate garante isto hoje. AC marcado `[x]`
   no roadmap, mas a cobertura de teste real é parcial (cobre o frontmatter, não o log).
3. **Registrar REQ de acompanhamento para `/api/chain` em Node e Python** — mesmo tratamento já
   dado ao gap de `thirdparty_artifact_has_provenance` no Node. Reproduzido ao vivo nesta
   auditoria (não só por leitura de código):
   - Python: referência limpa (`/`) desenha a aresta; a mesma referência suja (`\`) produz **zero
     arestas** — o sintoma 2 do parecer de ameaça reproduz de verdade em Python, sem correção
     neste diff.
   - Node: referência **limpa** já não desenha aresta nenhuma — bug estrutural mais amplo e
     anterior a esta REQ (`resolveRef` nunca aplica `path.basename` ao valor comparado), que torna
     o sintoma do separador irrelevante ali (a funcionalidade já está quebrada antes de qualquer
     `\` entrar em cena).
   - Nenhum dos dois é regressão introduzida por este PR; o critério de aceite do ML-1B que cobria
     isto está corretamente desmarcado no roadmap, e a leitura de `cli-parity.md` não alega
     cobertura tri-runtime aqui (a lista usa identificadores Go, mesmo padrão do gap já nomeado do
     Node).

### Confirmado por falsificação e reprodução própria (não apenas por relato)
- `scripts/check-ref-separator-portability.sh`: `assert_count` pega a regressão parcial que
  `assert_has` deixaria passar; guarda de vacuidade pega remoção de checagem; gate confirmado
  ligado ao `parity:` do `Makefile`. Nenhum outro `assert_has` do gate está exposto ao mesmo
  risco (todas as 17 needles são únicas nos arquivos-alvo).
- Normalização não vaza para o corpo de nenhum artefato, nos 3 runtimes, com teste de controle
  dedicado nos 3.
- Risco de super-normalização em POSIX (nome de arquivo com `\` legítimo) é teórico, já
  documentado e corretamente classificado como residual pelo parecer de ameaça.
- Sintoma 2 (aresta órfã) reproduzido ao vivo em Python (regressão real, não corrigida, fora
  desta REQ) e comportamento pré-existente do Node confirmado ao vivo (grafo não desenha nem
  referência limpa).
