# Triagem — Achados 12–16 do issue #216 (comentários de 2026-08-31)

> Autor: `hefesto-tf` | Data: 2026-09-01 | Tipo: parecer, não implementação. Nenhuma linha em
> `internal/`, `npm/`, `pypi/`, `scripts/`, `.github/`, `Makefile` foi tocada por este documento.

## Escopo

Quatro comentários novos de `@lourivalgarciajunior` no issue #216, todos de 2026-08-31, contendo
cinco achados (12 a 16). Não é o inventário de 11 defeitos original — são achados feitos **durante**
o trabalho de reprodução dos PRs #222–#225, fora da lista original.

Método: leitura do código atual (não do diff do comentário), reprodução local onde o achado não
depende de Windows, grep de REQs existentes para descartar duplicidade, e um `git blame`/leitura de
linha para confirmar que o trecho citado ainda corresponde ao `main` atual.

**Inventário prévio consultado** (para não reabrir o que já está resolvido/registrado):
`docs/analises/2026-08-31-aproveitamento-dos-prs-222-225.md`,
`docs/roadmaps/done/ROADMAP-2026-08-30-job-de-windows-largo-que-nasce-vermelho-e-sonda-sob-demanda.md`
(tabela de completude dos 11 itens originais + escopo negativo dos itens 8, 9, 11), `docs/req/`
(as 15 REQs abertas entre 2026-08-30 e 2026-09-01), `docs/cli-parity.md`.

**Resultado da checagem de duplicidade**: nenhum dos cinco achados novos duplica uma REQ existente.
Confirmado por grep — `fchmod`/`_atomic_write` (achado 13): zero ocorrências em `docs/req/` ou
`docs/roadmaps/`. `check-roadmap-barrier-contract`/`corpus-snapshot` (achado 16): só aparece
mencionado no roadmap `done` que o *criou*, nenhuma REQ aberta sobre ele. As REQs mais próximas por
tema — `REQ-2026-09-01-camada-2-mede-a-plataforma...` (achado 12/15 tocam código adjacente, tema
diferente: aquela é sobre os checks da sonda PowerShell medirem o SO, não o `trackfw`),
`REQ-2026-08-30-caminho-portavel-...` (item 10/11 antigos, separador de SO em artefato — não é o
tema de nenhum dos cinco novos), `REQ-2026-08-31-guarda-de-folha-...` (bypass de symlink por
ancestral, tema de segurança distinto de qualquer um dos cinco) e
`REQ-2026-08-30-fonte-unica-de-vetores-de-teste-...` (divergência de veredito entre os 3 CLIs por
lista de vetores escrita à mão — tema adjacente ao 12/14 mas não o mesmo mecanismo) — foram lidas
inteiras e nenhuma cobre o que segue.

---

## Achado 12 — asserção vácua por escape de JSON em `update_test.go:144`

**Novo.** Não coberto por nenhuma REQ.

```go
// internal/generators/update_test.go:144
if !strings.Contains(string(manifest), backendPath) || strings.Contains(string(manifest), frontendPath) {
    t.Fatalf("unexpected Codex ownership manifest:\n%s", manifest)
}
```

Confirmado por leitura — o trecho ainda está em `main`, linha 144. No Windows, `backendPath` contém
`\`, o manifesto grava JSON com `\\` escapado, e `strings.Contains` compara a string crua contra o
conteúdo escapado — nunca casa. As duas metades do `||` ficam comprometidas, mas de formas
diferentes:

- A primeira (`Contains(manifest, backendPath)`) falha sempre no Windows — é ruído, o teste reprova
  onde não há defeito.
- A segunda (`!Contains(manifest, frontendPath)`, negada dentro do `||`) é o problema real: ela
  deveria provar que um agente Codex desconhecido **não** foi reivindicado no manifesto. No Windows
  ela é **sempre verdadeira independentemente do conteúdo real do manifesto**, porque a comparação
  crua nunca vai casar de qualquer forma — a garantia negativa fica sem verificação nenhuma.

**Defeito nosso ou uso de consumidor?** Nosso — é teste de produto (`internal/generators`), não
config de ambiente.

**Reproduz?** Não em macOS/Linux (não há `\` a escapar, a comparação crua funciona). Não simulei —
é um fato de codificação (Go `encoding/json` escapa `\` em string Windows), verificável por leitura,
não precisa de execução.

**Classe.** É a classe "mecanismo dá sinal verde enquanto o controle está inerte" — exatamente:
o teste passa (ou falha pelo motivo errado) sem nunca ter avaliado a garantia negativa que existe
para proteger.

**Severidade.** Baixa-média — é teste, não produto; mas é da mesma família que já custou caro nesta
sessão (item 5 da linha de base "sinal aparentemente presente, sem oráculo").

**Recomendação.** REQ nova pequena, ou ML dentro de uma REQ maior de saneamento de testes (ver
disposição consolidada abaixo — proponho agrupar com o achado 14, que é da mesma família, mas em
arquivo/mecanismo distintos, então pode entrar como um terceiro ML da mesma REQ ao invés de REQ
própria). Correção: comparar contra o **JSON-encoded** do caminho (`json.Marshal(backendPath)`) ou
decodificar o manifesto e comparar chaves, não string crua contra bytes serializados.

---

## Achado 13 — `os.fchmod` ausente no Windows quebra escrita atômica em 3 pontos críticos do Python

**Novo, e é o mais grave dos cinco.** Não coberto por nenhuma REQ.

```
pypi/trackfw/identity/__init__.py:97        _atomic_write
pypi/trackfw/integrations/manager.py:120    IntegrationManager._atomic_write
pypi/trackfw/thirdparty/quarantine.py:42    _atomic_write
```

Confirmado por leitura — as três linhas ainda chamam `os.fchmod(descriptor, mode)`, sem guarda de
plataforma nem fallback. `os.fchmod` é documentado pela própria stdlib do Python como
**Unix-only** — não existe em `win32`. Isto derruba `trackfw init --ai-tools`, `agents install`,
`skills install` e o install de terceiro no Windows, sempre, porque as três funções estão no
caminho de escrita crítico dessas quatro operações.

**Defeito nosso ou uso de consumidor?** Nosso — bug de portabilidade no produto, não do ambiente do
consumidor. Não há forma de contornar do lado do consumidor sem editar o pacote instalado.

**Reproduz?** **Não em macOS/Linux** — `os.fchmod` existe em ambos (confirmado: `hasattr(os,
'fchmod')` → `True` em Darwin). Isto bate exatamente com o relato do autor: é Windows-only por
ausência de API na plataforma, um fato de stdlib, não uma hipótese a testar aqui.

**Classe.** **Não** é "mecanismo verde / controle inerte" — é ausência pura de API na plataforma,
uma classe diferente e mais direta: crash imediato (`AttributeError`), não passagem silenciosa.

**Paridade dos 3 CLIs — nuance que exige cuidado antes de recomendar a correção do autor.** Fui além
do que o comentário mostra e li as três funções por inteiro (`_atomic_write` idêntica nos três
sítios): `tempfile.mkstemp()` cria o arquivo temporário com modo `0600` por padrão (comportamento da
stdlib). Os call sites de `identity/__init__.py` e `thirdparty/quarantine.py` sempre passam
`mode=0o600` — **sem alargamento**, o `fchmod` ali é redundante mas inofensivo. Já
`integrations/manager.py:585` passa `0o644` para artefatos de agente/skill/hook — **este é um
alargamento real** (0600 → 0644, leitura para grupo/outros).

A sugestão do autor — trocar `os.fchmod(descriptor, mode)` por `os.chmod(path, mode)` — alinha com
Go (`os.Chmod(path,...)`) e Node (`fs.chmodSync(tmp,...)`), mas **os dois outros runtimes já usam a
forma baseada em caminho, que é estruturalmente mais fraca**: entre `mkstemp()` retornar o caminho e
`os.chmod(path,...)` ser chamado, existe uma janela TOCTOU que `fchmod(descriptor,...)` (ligado ao
descritor, não ao caminho) elimina por construção. O diretório é criado com `mode=0o700` (só o dono
grava), e `mkstemp` gera nome único por processo — isto reduz o risco prático a "outro processo do
mesmo usuário", não elimina a diferença estrutural entre as duas formas. Regra do papel: nunca
recomendar enfraquecer um controle para fazer algo passar, mesmo que o risco residual seja baixo.

**Recomendação de correção (respondendo à pergunta direta do autor — ele pediu explicitamente qual
das duas ele deveria mandar):** **opção B — `getattr(os, "fchmod", None)` com fallback para
`os.chmod(path, mode)` só quando `fchmod` não existir (Windows)**, não a opção A (trocar
incondicionalmente para `os.chmod(path,...)` nos três). É estritamente não regressiva: mantém a
forma mais forte em todo lugar onde a plataforma a oferece (macOS, Linux — inclusive onde a
diferença de alargamento em `manager.py` importa), e só aceita a forma mais fraca onde a mais forte
nem existe (Windows) — não há ganho de simplicidade que justifique abrir mão da propriedade
race-free nos dois SOs onde ela está disponível de graça.

**Severidade.** **Alta.** Quebra total, determinística, no caminho de onboarding (`init
--ai-tools`) e nos comandos de instalação de agente/skill/terceiro, em toda máquina Windows, sem
exceção. Não é um gate ou uma sonda medindo a plataforma — é o produto real falhando na função real.

**Recomendação de disposição.** REQ própria, alta prioridade — ver seção de priorização abaixo.

---

## Achado 14 — testes acoplados a três arquivos reais do próprio repositório

**Novo.** Não coberto por nenhuma REQ.

```go
// internal/validator/validator_test.go:2109-2112 — TestExtractRefPath_TresREQsReaisDoRepositorio
```
```python
# pypi/tests/test_validator.py:540 — test_extract_ref_path_resolve_reqs_reais_com_backtick
```

Confirmado por leitura — os dois testes existem hoje, exatamente como descrito, e leem por caminho
fixo três arquivos de `docs/req/` do próprio repositório trackfw
(`REQ-2026-07-27-roadmap-move-sincroniza-o-status-do-artefato.md` e mais dois). **Node não tem
equivalente** — confirmei por grep, nenhum arquivo em `npm/tests/` referencia esses três nomes —
o que é, à parte do achado principal, uma quebra de paridade de cobertura entre os 3 CLIs
(Go+Python testam algo que Node não testa, ainda que indiretamente via os outros dois).

Não executei a remoção dos três arquivos para provar por execução — a primeira tentativa envolveria
mover `docs/req/*.md` reais do repositório, e isso está fora do meu escopo de escrita (não edito
REQ) e do princípio de não alterar estado do repositório fora do artefato deste parecer; o bloqueio
de segurança do próprio ambiente confirmou o limite antes que qualquer arquivo saísse do lugar
(nenhum arquivo foi de fato movido — verificado depois). O mecanismo, porém, é auto-evidente por
leitura: `filepath.Join("..", "..", "docs/req/<nome>.md")`, caminho fixo, sem fixture, sem `t.Skip`
se ausente — qualquer editor que renomeie/mova uma das três REQs (movimento normal de
`backlog→wip→done`, aliás — só que essas três já estão em `done`, então o risco real é edição de
conteúdo, não de estado) reprova o teste, e reprova de um jeito que não relaciona a mudança à
função testada (`extractRefPath`, um parser).

**Defeito nosso ou uso de consumidor?** Ambos, e a distinção importa aqui:
- Do lado do **produto/manutenção**: é fragilidade de teste — acoplamento a documento em vez de
  fixture, corrigível sem mudar comportamento.
- Do lado de **consumidor que forka o repositório-fonte** (não `pip install`): é uma consequência
  direta e inevitável de não carregar a governança do trackfw junto — o teste **sempre** vai
  reprovar lá, porque os três arquivos nunca vão existir no fork por decisão deliberada dele. Isto
  não é bug **nesse** sentido — é o teste assumindo, sem declarar, uma precondição que só o
  repositório de origem satisfaz.

**Reproduz?** Sim, e sem precisar de Windows — o defeito de acoplamento é independente de SO.

**Classe.** Adjacente a "mecanismo verde / controle inerte", não o caso central: aqui o oráculo
mede **o estado do repositório de documentação**, não o comportamento do parser — o parser pode
estar correto ou quebrado e o teste reprova/passa por um motivo alheio a ele.

**Severidade.** Média — não quebra produção, mas é friccão real e recorrente para qualquer um que
adote o padrão "forkar o código-fonte do trackfw" (ver achado 16, mesma população afetada).

**Recomendação.** Ver disposição consolidada — proponho a **mesma REQ do achado 16**, com um AC
comum: "um clone/fork fresco, com `docs/roadmaps/**` estranho ou vazio, roda `make quality` sem
reprovar por ausência de conteúdo do repositório de origem." Achado 14 vira o ML que troca os três
arquivos reais por fixture em tempdir (a correção que o próprio autor já escreveu e descreveu:
três casos — backtick, sem backtick, caminho puro) e adiciona o equivalente Node (fecha a lacuna
de paridade de cobertura). Alternativa mais barata, se preferir não portar a fixture agora:
`t.Skip`/equivalente nomeando a garantia quando o arquivo não existir — o próprio autor propôs isso
como fallback aceitável.

---

## Achado 15 — `validate_branch_has_wip_roadmap` (Python) devolve string onde as outras ~40 regras devolvem dict, e a isolação de `cwd` não isola

**Novo.** Não coberto por nenhuma REQ. Este é o único dos cinco que **não é específico de Windows**
— reproduz em qualquer SO, e reproduzi.

### Verificação 1 — o retorno é heterogêneo, confirmado por leitura e por execução

```python
# pypi/trackfw/validator.py:1638-1639
if not candidates:
    return [branch_governance_orientation(branch)]   # str
return [branch_no_matching_roadmap_message(branch, candidates)]   # str
```

`_enrich_items` (linha 257) só enriquece `dict`; `else: result.append(item)` deixa a string passar
crua. Reproduzi ponta a ponta, sem depender do relato: criei um repositório `/tmp` isolado, sem
nenhum roadmap, e chamei `validate_unfiltered(cwd)` com `TRACKFW_BRANCH=fix/sem-roadmap-nenhum`:

```
<class 'str'> 'branch "fix/sem-roadmap-nenhum" is a feat/fix/refactor branch but no roadmap...'
```

`result["violations"]` volta com esse item como `str` puro, enquanto todo outro item de violação no
mesmo array é `dict`. Confirmado também com `pytest`, exatamente como o autor relatou:

```
$ TRACKFW_BRANCH=fix/qualquer python3 -m pytest pypi/tests/test_credential_guard_integrity.py pypi/tests/test_git_branch_guard_validator.py
8 failed, 44 passed
TypeError: string indices must be integers, not 'str'
```

### Verificação 2 — refinamento do gatilho relatado pelo autor (diverge do relato, e por um motivo que fortalece o achado)

Testei **na branch atual do trabalho**, `fix/caminho-dentro-de-artefato-versionado-usa-sempre-barra`
(que começa com `fix/`), sem `TRACKFW_BRANCH`, esperando reprodução — e a suíte **passou** (27/27).
O relato do autor ("abrir uma branch `fix/` e rodar `pytest` já reprova 8 testes") não é preciso
neste ponto: o gatilho real, lido no código, é `branch_slug_matches_roadmap` **não achar candidato**
— nesta branch específica existe um roadmap em `wip/` cujo nome casa com o slug da branch
(`ROADMAP-2026-09-01-caminho-dentro-de-artefato-versionado-usa-sempre-barra.md`), então a função
retorna `[]` antes de chegar nas duas linhas problemáticas. O gatilho preciso é: **branch
`feat/fix/refactor/` sem nenhum roadmap de nome correspondente em `wip/` ou `done/`** — o que ainda
é um gatilho amplo e realista (branch cortada antes do roadmap ser nomeado, drift de slug, ou
layout `by_agent` — onde a regra procura em `docs/roadmaps/wip/` fixo enquanto o roadmap real vive
em `docs/roadmaps/<agente>/wip/`, terceira forma de disparar).

### Verificação 3 — paridade cruzada, e uma correção de framing importante

Chequei se isto é quebra de paridade Python-only ou dos 3 CLIs. **É Python-only, mas não pelo
motivo que a comparação ingênua sugere.** Go (`validator.go:2439`,
`return []string{...}`) e Node (`validator/index.js:1281/1283`, `return [str]`) **também** devolvem
string crua para esta regra — os três runtimes concordam na forma interna do item. A diferença é
arquitetural: Go e Node tratam **toda** regra internamente como string e só empacotam em
dict/objeto **centralizadamente**, na fronteira (`applyRuleTagged` em Go; `_setMeta` fora de banda
em Node) — nunca existiu "cada regra devolve seu próprio dict" nesses dois runtimes. O Python é o
único cujo convênio interno é "cada regra já devolve `list[dict]` com `message`", e é só dentro
**desse** convênio que `branch_has_wip_roadmap` é o único fora do padrão — as outras ~40 regras
Python seguem a convenção, esta não. Não é, portanto, "Python diverge de Go+Node"; é "esta regra
diverge da própria convenção interna do Python".

### Verificação 4 — o contrato público `trackfw validate --json` está protegido, ao contrário do que a leitura inicial sugeria

Fui conferir se isto vaza para a saída pública que um usuário do CLI vê. **Não vaza**:
`pypi/trackfw/commands/validate.py` já trata os dois formatos defensivamente em ambos os modos —
`_item_to_json_dict(item)` (linha 40-52) faz `isinstance(item, dict)` antes de extrair `message`, e
o modo texto (linha 111/118) faz `v["message"] if isinstance(v, dict) else str(v)`. **O comando
`trackfw validate` (texto ou `--json`) nunca quebra por causa disto** — a inconsistência de tipo é
absorvida na borda do comando. Isto reduz a superfície do achado: não é "quebra o contrato JSON
público do CLI para qualquer consumidor", é "quebra qualquer código que chame
`trackfw.validator.validate_unfiltered()`/`validate()` diretamente como biblioteca Python (uso
programático do pacote, não via subprocesso do binário) e itere `violations` assumindo
homogeneidade — e quebra a suíte de testes do próprio projeto, que é onde o efeito foi medido."

### Verificação 5 — a isolação de `cwd` de fato não isola, e por um motivo mais específico do que "não isola nunca"

```python
# pypi/trackfw/validator.py:3486
_apply_rule("branch_has_wip_roadmap", validate_branch_has_wip_roadmap(cfg), violations, warnings, cfg)
```

`validate_unfiltered(cwd)` recebe `cwd` mas **não repassa** para
`validate_branch_has_wip_roadmap(cfg)` — a função não tem como saber qual cwd o chamador pediu.
Internamente ela deriva `git_cwd = os.path.dirname(os.path.abspath(cfg["roadmap_dir"]))`, e como
`roadmap_dir` é relativo por padrão, `os.path.abspath` resolve contra o cwd **real do processo**, não
contra o `cwd` isolado que `validate_unfiltered` recebeu. Efeito prático: um teste que chama
`v.validate_unfiltered(self.tmp)` esperando isolamento total continua enxergando a branch git real
de quem roda o processo — é exatamente o vetor que produziu os 8 falsos-vermelhos.

Nota sobre impacto em uso normal de CLI (não testado pelo autor, verificado por mim): quando
`trackfw validate` roda via linha de comando normal, o processo já está com `cwd` == raiz do
repositório-alvo — não há isolamento a quebrar nesse caso, porque `cwd` do processo e o `cwd`
pretendido são o mesmo diretório. O defeito só se manifesta em cenários de **teste** e de
**embutimento como biblioteca chamando `validate_unfiltered(outro_cwd)` a partir de um processo cujo
cwd é outro diretório** (ex.: um `trackfw serve` hipotético validando múltiplos projetos sem
`chdir`) — não afeta o uso de terminal comum.

**Classe.** **Não** é "mecanismo verde/controle inerte" — é o oposto: um crash barulhento
(`TypeError`), não um passe silencioso. Importante não inflar essa contagem incluindo este achado
nela.

**Severidade.** **Alta**, apesar do contrato CLI estar protegido — porque (a) reproduz em qualquer
SO, sem depender de Windows, (b) está no `main` agora, não em hipótese, (c) derruba
deterministicamente a suíte de testes Python sempre que alguém trabalhar numa branch `fix/`\
`feat/`\`refactor/` cujo roadmap ainda não tenha nome correspondente — situação comum no início de
um ciclo de trabalho, e (d) a correção é de baixo custo e bem especificada.

**Recomendação.** REQ própria, com duas ACs independentes (como o autor já separou): (1) contrato —
`validate_branch_has_wip_roadmap` passa a devolver `list[dict]` com `message` como as outras 40
regras, **ou** `_enrich_items` normaliza `str` em `{"message": s}` — o autor recomenda a primeira
como "mais honesta"; concordo, porque a segunda mascara a próxima regra que cometer o mesmo erro
silenciosamente. (2) isolamento — `validate_branch_has_wip_roadmap` passa a receber e usar o `cwd`
que `validate_unfiltered(cwd)` já recebe, em vez de derivá-lo de `roadmap_dir`. Verificar também se
Go/Node têm o mesmo problema de isolamento em cenário de teste/embutimento equivalente (não
verifiquei isso — os dois têm a mesma forma de retorno de string, mas não confirmei se têm o mesmo
padrão de derivar cwd a partir de config em vez do parâmetro recebido).

---

## Achado 16 — o gate de snapshot do `barrier` reprova em qualquer fork que não herde a governança do trackfw

**Novo.** Não coberto por nenhuma REQ.

### O mecanismo, confirmado por leitura de `scripts/check-roadmap-barrier-contract.sh`

O gate faz duas coisas distintas dentro do mesmo laço (linhas ~483-527), e a confusão entre elas é
a causa raiz:

1. Para cada basename do snapshot congelado (144 arquivos, `scripts/testdata/roadmap-barrier-corpus-
   snapshot/`), verifica se ele **existe em algum lugar** de `docs/roadmaps/**` **na árvore de
   trabalho ao vivo** (`find "$ROOT_DIR/docs/roadmaps" -type f -name "$base"`, linha 490). Se não
   existir, marca como `MISSING_FROM_DISK` e **pula** (`continue`, linha 493) — **não incrementa
   `CORPUS_FILES`**.
2. Só para os basenames que passaram no passo 1, copia o **conteúdo do snapshot** (nunca do disco —
   `cp "$snapshot_path" ...`, linha 494) para um sandbox e roda o `barrier` real contra ele, somando
   nas contagens (`CORPUS_FILES`, `CORPUS_WAVES`, hash de veredito) que são comparadas contra os
   valores pinados.

**O ponto central: o conteúdo comparado contra os valores pinados sempre vem do snapshot — nunca do
disco.** A checagem de presença no disco (passo 1) não protege o conteúdo comparado (isso já está
imune, é bytes congelados) — ela é, na intenção documentada no próprio script (comentário linha
367), um **detector de deriva**: "um basename do snapshot AUSENTE do disco indica que um arquivo do
corpus congelado foi apagado/renomeado sem atualizar o snapshot". Essa é uma checagem legítima e
correta **dentro do repositório de origem**, onde `docs/roadmaps/**` é, por definição, o superconjunto
que originou o snapshot. Ela deixa de fazer sentido no instante em que `docs/roadmaps/**` não é mais
essa árvore — que é exatamente o caso de um fork com governança própria.

### Verificação — a alegação de que "`scripts/` é produto, sai no pacote" está incorreta, e isso muda o enquadramento

Chequei os manifestos de empacotamento:

```
npm/package.json        "files": ["bin/", "src/"]           ← scripts/ e Makefile NÃO incluídos
pyproject.toml          include = ["trackfw*"]              ← idem
```

**`scripts/` e `Makefile` não são distribuídos via `pip install trackfw` nem `npm install trackfw`.**
Só existem para quem clona/forka o **repositório-fonte** do trackfw — não para quem instala o pacote
publicado e usa `trackfw validate`/`status`/`context` no próprio projeto (que é o que os itens 1–11
originais da issue tratam). Isto não invalida o achado — o relato deixa claro que ele rodou
`internal/validator` (Go) e `pypi/tests` dentro do próprio repositório dele, ou seja, ele **forkou o
código-fonte do trackfw** como base do projeto dele, não apenas instalou o pacote. Mas **reduz o
raio de impacto**: afeta quem forka o repositório-fonte para desenvolver em cima dele (a mesma
população do achado 14), não todo consumidor do `pip`/`npm install`.

### Julgamento pedido — qual das três saídas, e se o gate deveria sequer rodar

Nenhuma das três como proposta, isolada, é a certa. A proposta do autor trata as sete sub-checagens
do gate (`files-count`, `waves-count`, `exit2-count`, `mls-complete-verdict-counts`,
`acceptance-evidence-verdict-counts`, `non-reclassification`, `basename-missing-from-disk`) como um
bloco monolítico — e elas não são: seis delas protegem uma propriedade do **parser do `barrier`**
(não reclassificar veredito de ML/AC ao evoluir), e uma (`basename-missing-from-disk`) protege uma
propriedade da **governança viva deste repositório** (não perder um arquivo do histórico sem
atualizar o congelamento). São duas garantias diferentes, com dois públicos diferentes, coincidindo
hoje no mesmo `continue` de uma única linha (493).

**Julgamento: desacoplar a contagem/hash da checagem de disco, não escolher uma das três saídas
propostas.** A mudança mínima: remover o `continue` no ramo "ausente do disco" para o cálculo de
`CORPUS_FILES`/`CORPUS_WAVES`/hash — essas seis checagens passam a rodar **sempre** sobre os 144
arquivos do snapshot, independentemente do que existe em `docs/roadmaps/**` (coerente com o fato de
que o conteúdo já vem 100% do snapshot). A sétima (`basename-missing-from-disk`) continua
comparando contra o disco, mas **degrada para skip nomeado** — não para reprovação — quando a
interseção snapshot∩disco for pequena (a forma da opção 2 do autor), porque nesse caso ela não está
detectando deriva, está detectando "este não é o repositório de onde o corpus foi extraído", que é
um fato válido, não um defeito. Isto é próximo da opção 1 do autor para as seis primeiras (ele
também chega em "varrer o próprio snapshot" como preferida) e da opção 2 para a sétima — mas
tratando-as como duas correções separadas, não uma escolha única entre três.

O gate **deveria** rodar num fork — a parte que verifica não-reclassificação do parser é uma
propriedade de código (`barrier.go`/equivalentes), válida em qualquer cópia do código-fonte,
inclusive um fork. O que não deveria rodar incondicionalmente é só a checagem de deriva de disco.

### A pergunta estrutural — existe fronteira entre gate "de desenvolvimento" e gate "de uso"?

**A fronteira já existe, mas só no nível de empacotamento, e não está documentada nem testada.**
`npm/package.json` e `pyproject.toml` já separam "o que vai para quem instala o pacote" (CLI:
`bin/`, `src/`/`trackfw*`) de "o que só existe para quem trabalha no código-fonte" (`scripts/`,
`Makefile`, `internal/validator/*_test.go`, `pypi/tests/`). Essa fronteira é implícita — nasce do
manifesto de empacotamento, não de uma decisão declarada em ADR/VISION sobre quem é o público de
`make quality`/`make parity`.

O achado real, mais estrutural que o gate específico: **não existe nenhum teste/gate que afirme "um
clone fresco do repositório-fonte, com `docs/roadmaps/**` estranho ou vazio, consegue rodar `make
quality` sem reprovar por ausência de conteúdo do repositório de origem"**. Essa ausência é o que
permitiu tanto o achado 14 (testes Go/Python acoplados a três REQs reais) quanto o achado 16 (gate
de snapshot acoplado a `docs/roadmaps/**` real) sobreviverem sem detecção — os dois são instâncias
do mesmo buraco, não dois problemas independentes. O achado 12 (asserção vácua por escaping) e o 13
(API ausente) **não** são instâncias dessa classe — são independentes, um de teste, um de produto.

Se "forkar o código-fonte do trackfw como base de um projeto próprio" é um modo de uso que o projeto
quer suportar, a fronteira precisa ser declarada (ADR ou seção em `docs/cli-parity.md`/`CLAUDE.md`
dizendo: "estes gates assumem o `docs/roadmaps/**` do trackfw; forks devem X") **e** testada (o
smoke test de fork acima). Se não é um modo de uso suportado, isso também precisa ser declarado, para
que o próximo achado desta classe não seja tratado como surpresa.

**Recomendação.** Uma REQ cobrindo achados 14 + 16 juntos, com a AC do smoke test de fork como
critério central e um ADR curto (ou seção nova em `docs/cli-parity.md`) declarando a fronteira
dev/uso — não porque a implementação dos dois compartilhe arquivo (não compartilha), mas porque
compartilham a mesma causa estrutural e o mesmo público afetado, e uma REQ só de "fork smoke test"
que force os dois a se resolverem junto é mais barata de auditar do que duas REQs paralelas
reinventando a mesma pergunta.

---

## Tabela de triagem consolidada

| Achado | Novo/Duplicado | Defeito nosso ou uso de consumidor | Severidade | Classe "verde/inerte"? | Recomendação | Onde entra |
|---|---|---|---|---|---|---|
| 12 — asserção vácua em `update_test.go:144` (escape de JSON no Windows) | Novo | Nosso (teste) | Baixa-média | **Sim** — a garantia negativa nunca é avaliada | ML dentro da REQ 14+16, ou REQ própria pequena se preferir isolar | Correção de teste, `internal/generators/update_test.go` |
| 13 — `os.fchmod` ausente no Windows quebra 3 escritas atômicas Python | Novo | Nosso (produto) | **Alta** | Não — crash imediato por API ausente | REQ própria — fallback `getattr(os,"fchmod",None)`→`os.chmod(path,...)` só quando ausente (opção B, não a A) | `pypi/trackfw/identity/`, `integrations/manager.py`, `thirdparty/quarantine.py` |
| 14 — testes Go/Python acoplados a 3 REQs reais do repositório | Novo | Ambos (fragilidade de teste + precondição de fork) | Média | Adjacente — oráculo mede doc, não parser | REQ conjunta com 16 (fixture em tempdir + `t.Skip` de fallback + fechar gap de paridade Node) | `internal/validator/validator_test.go`, `pypi/tests/test_validator.py`, novo em `npm/tests/` |
| 15 — `validate_branch_has_wip_roadmap` devolve `str`, isolamento de `cwd` não isola | Novo | Nosso (produto — mas não vaza para o contrato `--json` do CLI, confirmado por leitura de `commands/validate.py`) | **Alta** | Não — crash barulhento (`TypeError`), não passagem silenciosa | REQ própria, 2 ACs independentes (contrato + isolamento) | `pypi/trackfw/validator.py:1603-1639`, `:3486` |
| 16 — gate de snapshot do `barrier` reprova em fork sem o histórico de roadmaps | Novo | Nosso, com raio de impacto restrito a quem forka o código-fonte (não a quem `pip`/`npm install`) | Média | **Sim, na forma inversa** — `parity` ficou `skipped` meses, então "verde" era ausência de sinal | REQ conjunta com 14 + ADR/seção declarando a fronteira dev-gate/uso-gate | `scripts/check-roadmap-barrier-contract.sh`, novo ADR ou seção em `docs/cli-parity.md` |

## Julgamento sobre o gate de snapshot (resumo direto)

Nenhuma das três saídas propostas pelo autor, isolada, é a certa — o gate mistura duas garantias
diferentes numa mesma linha (`continue` no ramo "ausente do disco", linha ~493 de
`check-roadmap-barrier-contract.sh`). A correção certa é **desacoplar**: as seis sub-checagens de
contagem/hash (que já operam 100% sobre bytes do snapshot, nunca do disco) devem parar de depender
da presença em `docs/roadmaps/**` — rodam sempre, sobre o snapshot; só a sétima
(`basename-missing-from-disk`, que é de fato um detector de deriva do repositório de origem) deve
degradar para skip nomeado ("corpus não exercitado: este não é o repositório de origem") quando a
interseção snapshot∩disco for pequena. Isto não é a opção 1 nem a 2 do autor isoladamente — é as
duas, aplicadas a metades diferentes do gate, porque as duas metades protegem coisas diferentes.

O gate **deve** continuar rodando num fork — a parte que ele protege de fato (não-reclassificação do
parser do `barrier`) é uma propriedade de código, válida em qualquer cópia do fonte. A alegação do
autor de que "`scripts/` é produto, sai no pacote" está **factualmente incorreta** — `scripts/` e
`Makefile` não constam em `npm/package.json` (`files: ["bin/","src/"]`) nem em `pyproject.toml`
(`include: ["trackfw*"]`); não chegam a quem faz `pip install`/`npm install`. Isto não invalida o
achado (ele forkou o repositório-fonte, não instalou o pacote), mas restringe quem é afetado a essa
população específica — e é essa restrição que torna a pergunta estrutural real: **a fronteira entre
"gate de desenvolvimento do trackfw" e "gate de uso do trackfw" já existe implicitamente no nível de
empacotamento (o que vai em `files`/`include` vs. o que fica em `scripts/`/`Makefile`), mas nunca
foi declarada em ADR nem testada por um smoke test de fork.** Essa ausência de teste é a causa raiz
comum dos achados 14 e 16 — nenhum gate hoje afirma "um clone fresco, com `docs/roadmaps/**`
estranho ou vazio, roda `make quality` sem reprovar por ausência de conteúdo do repositório de
origem". É o achado estrutural que vale mais que o gate específico, como o enunciado da tarefa
antecipou.

## Priorização frente aos itens 4, 7 e 10 já na fila

**Os achados 13 e 15 superam os itens 4, 7 e 10 em prioridade.**

- **Item 13** derruba `init --ai-tools`, `agents install`, `skills install` e o install de
  terceiro — em toda máquina Windows, sempre, no caminho de onboarding. É quebra total e
  determinística de produto, não um gate ou uma sonda medindo a plataforma.
- **Item 15** reproduz em **qualquer sistema operacional** (medido em macOS, sem depender de runner
  Windows nenhum), está no `main` agora — não é hipótese —, e derruba a suíte de testes Python
  sempre que alguém trabalha numa branch `fix/`/`feat/`/`refactor/` sem roadmap de nome
  correspondente ainda, situação comum no início de qualquer ciclo.
- Em comparação: o **item 4** (gate de cobertura morre em cp1252) é um script auxiliar, não o
  produto em si; o **item 7** (`sh -c` hardcodado) está com reprodução **INCERTA mesmo em runner
  Windows real**, segundo a própria linha de base medida; o **item 10** (separador de SO em
  artefato versionado) é real mas **latente** — só se manifesta na combinação específica de um
  commit no Windows seguido de checkout no Linux, não no dia a dia isolado de uma única máquina.

Os achados **12, 14 e 16 ficam abaixo** dos itens 4/7/10 na fila — são fricção de teste e de
fork-maintenance, não quebra de produto em produção.
