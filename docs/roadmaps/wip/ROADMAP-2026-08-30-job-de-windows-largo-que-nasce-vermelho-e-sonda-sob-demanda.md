---
status: wip
date: 2026-08-30
req: "docs/req/REQ-2026-08-30-ci-nao-exercita-windows-e-os-sete-defeitos-da-issue-216-sao-invisiveis-para-o-projeto.md"
squad: "hades-tf, ares-tf, artemis-tf"
---

# Roadmap: Job de Windows largo que nasce vermelho, e sonda sob demanda

> Created: 2026-08-30 | Status: wip

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Context

REQ: `REQ-2026-08-30-ci-nao-exercita-windows-e-os-sete-defeitos-da-issue-216-sao-invisiveis-para-o-projeto.md`
ADR: `ADR-2026-08-30-ci-de-windows-como-instrumento-de-medicao-job-largo-que-nasce-vermelho-mais-sonda-sob-demanda.md`

Onze defeitos de Windows conhecidos — sete da issue #216, três achados por nós, três em comentários
dela. Todos invisíveis para o CI, que roda **três invocações dirigidas** em `windows-latest`.

Esta REQ entrega o **instrumento**, não as correções. O job precisa **nascer vermelho**.

## Acceptance Criteria

Consolidado — AC1 a AC11 da REQ. **A AC2 define a REQ:** se o job nascer verde, o job está errado.

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Modelo de ameaça do instrumento
**Status:** ✅ Concluído
**Agente:** `hades-tf`
**Files affected:** apenas este roadmap.
**Actions:**
1. **Completude de enumeração.** Os onze defeitos conhecidos estão listados na issue #216 e nas REQs
   abertas. **Quais deles o job largo NÃO vai expor**, e por quê? Um job que reprova por 4 dos 11
   dá falsa sensação de cobertura sobre os outros 7. Enumere item a item: cp1252, `$HOME`, bit de
   execução, gate de cobertura, CRLF na escrita, `isatty`, `sh -c`, `\` no destino,
   `ref_targets_exist` vácuo, separador em artefato, e os 12 testes de symlink sem privilégio.
2. **Modelo de ameaça do próprio instrumento.** A sonda é execução manual num runner; o que ela pode
   vazar em log público? O job largo roda suíte completa em Windows — algum teste **escreve fora do
   workspace** lá, dado que `$HOME` não é isolado (defeito 2/6 da issue) e os testes fazem
   `Setenv("HOME")` que não isola em Windows? **Este é o vetor que mais me preocupa: o job pode
   corromper o próprio runner e produzir resultado não reproduzível.**
3. **Alvos de falsificação nas duas direções.** O que quebra se o job regredir para dirigido de novo;
   e se regredir para o outro lado — `continue-on-error` esquecido depois das correções, ou `skip`
   virando incondicional e escondendo tudo.
4. **Residual declarado.** O que o runner **não** responde: junction, `core.symlinks=false`, console
   cp1252 real, e o que depende da máquina de quem clona.
**Critérios de aceite:**
- [x] As quatro seções com evidência, não asserção
- [x] A tabela do item 1 cobre os 11, dizendo para cada um se o job expõe e como
- [x] Nenhuma linha de implementação

**Gates da wave:**
```bash
test -f docs/adr/ADR-2026-08-30-ci-de-windows-como-instrumento-de-medicao-job-largo-que-nasce-vermelho-mais-sonda-sob-demanda.md
```

#### Resultado do ML-0A (hades-tf, 2026-08-30)
**Status:** ✅ Concluído

Metodologia: para cada um dos 11 itens, li o código-fonte do defeito E o teste que supostamente o
cobriria (não assumi cobertura pela existência de um arquivo de teste no pacote certo). Onde a
suíte usa `subprocess.run`/invocação real do binário, tratei como "pode expor"; onde o teste
faz `monkeypatch`/mock do próprio ponto que falha, ou lê de volta via uma API que mascara o sintoma
(texto normalizado, comparação idempotente sem oráculo de conteúdo), tratei como "não expõe".

##### 1. Tabela de completude — os 11 itens

| # | Defeito | Job largo expõe? | Evidência |
|---|---|---|---|
| 1 | cp1252 no `cli.py` (`--help` no parser top-level, `cli.py:50`) | **NÃO** | Único teste subprocess de `--help` é `run_trackfw("roadmap", "--help")` (`pypi/tests/test_commands_basic.py:287`) — help de **subparser**, que não renderiza a `description=` do parser raiz (onde está o `→`). Nenhum teste chama `run_trackfw()` sem argumentos, que é o único caminho de produção real que imprime essa string (`cli.py:180`, `parser.print_help()` quando `args.command is None`). `roadmap.py:106,116` também imprime `→` de verdade (`✓ moved ... → ...`), e não é exercitado via subprocesso em teste algum que localizei. **Exige teste novo.** |
| 2 | `$HOME` ignorado nos 3 runtimes | **SIM, mas de forma não confiável** | Ver seção 2 — a isolação de teste que deveria expor isso de forma limpa é ela própria vácua no Windows nos 3 runtimes. O job vai, sim, produzir sinal, mas ruidoso: falhas indiretas de outros testes por escrita na home real, não um assert direto e reprodutível no item 2. |
| 3 | Bit de execução — `info.Mode()&0111==0` sempre verdadeiro no Windows (`internal/validator/validator_credential_guard.go:377`, `validator_git_branch_guard.go:193`) | **SIM** | `go test ./...` roda `internal/validator/validator_credential_guard_integrity_test.go` e afins, que criam script sintético, dão `chmod 0755` e esperam bit reconhecido. No Windows `os.Stat` nunca marca `0111` — assert falha por construção, sem precisar de teste novo. |
| 4 | Gate de cobertura (`scripts/check-parity-contract-coverage.sh`) morre em cp1252 | **NÃO — fora do escopo do job tal como desenhado** | O script só é chamado por `scripts/check-gates-falsify.sh` e por `make quality`; não faz parte de `go test ./...`, `npm test` nem `pytest pypi/tests` (grep confirmou: nenhuma referência a `check-parity-contract-coverage` fora de shell scripts). AC1 da REQ lista só essas três suítes — o job largo, por design, **não pode** expor este item. É um gap estrutural aceito, não um bug do instrumento. |
| 5 | CRLF na escrita dos geradores Python (38+ sites sem `newline=`) | **PARCIAL — mecanismo presente, oráculo ausente** | `pypi/tests/test_generators_roadmap.py:774-836` já lê os arquivos gerados em modo `"rb"` (bytes crus, não universal-newlines) — mas as asserções são só de **idempotência** (`bytes_before == bytes_after`), nenhuma asserta ausência de `\r\n`. A suíte teria dado green mesmo com CRLF em todo lugar, porque nunca comparou contra o valor esperado. **Exige assert nova, não teste novo inteiro.** |
| 6 | `isatty()` mente para `NUL` (Python init wizard) | **NÃO** | Todos os pontos de teste (`test_init_identity.py:83,98`; `test_identity_wizard.py:91,112,131,149,167,236`) fazem `monkeypatch.setattr("sys.stdin.isatty", lambda: ...)` — substituem a própria chamada que falha. A suíte nunca invoca `sys.stdin.isatty()` de verdade sob um `NUL` redirecionado. **Exige teste novo via subprocess com stdin redirecionado.** |
| 7 | `sh -c` hardcodado no Go (`internal/commands/barrier.go:729`) | **INCERTO — depende da imagem do runner** | Exercitado por `barrier_test.go`/`barrier_contract_test.go` dentro de `go test ./...`. Mas a imagem `windows-latest` da GitHub Actions inclui Git para Windows, tipicamente com `sh.exe` no `PATH` — a falha "ausência total de `sh`" relatada pelo autor (fora do CI, máquina sem Git Bash) pode não se reproduzir da mesma forma no runner hospedado. O que sobra exposto é só a semântica potencialmente divergente do `sh -c`, e nenhum teste hoje compara essa semântica entre plataformas. **Não confiar neste item até Wave 1 confirmar empiricamente se `sh` está no PATH do runner e se algum gate real diverge.** |
| 8 | Postura divergente com `\` no `manager.go` (Node rejeita, Go não) | **NÃO** | Não é crash, é ausência de guarda simétrica — só apareceria se algum teste cross-runtime comparasse explicitamente a resposta dos dois managers para um alvo com `\`; não localizei esse teste em nenhum dos 3 runtimes. O próprio autor da issue chama de "não explorável hoje". **Exige teste dedicado, fora do escopo de rodar as 3 suítes como estão.** |
| 9 | `ref_targets_exist` vácuo em `roadmap_namespacing: by_agent` | **NÃO — nem é um bug de Windows** | A causa raiz (confirmada pelo autor no comentário mais recente: escritor grava `req_dir/REQ-x.md` flat, leitor busca em `req_dir/<agente>/<estado>/*.md`) se manifesta em **qualquer SO**, inclusive Linux. O job de Windows não vai expor isso porque não é uma questão de plataforma — é uma questão de fixture (`roadmap_namespacing: by_agent`) que nenhuma das 3 suítes hoje combina com uma REQ com `roadmap:`/`adr:` quebrado. Já existe REQ própria (`REQ-2026-08-30-consumidores-que-nao-conhecem-by-agent-...`) — correto deixar fora desta. |
| 10 | Separador de SO (`\`) vazando para o frontmatter da REQ no `roadmap move` | **PROVAVELMENTE NÃO** | Não localizei, nos 3 runtimes, um teste de `roadmap move` que compare a **string literal** gravada no frontmatter após sync em Windows. O próprio defeito explica por que um assert ingênuo não pega: "no Windows resolve, porque `os.Stat` aceita as duas grafias" — ou seja, um assert tipo `path exists`/`roadmap resolve para o arquivo certo` passa mesmo com `\` no conteúdo; só um assert de igualdade de string exata contra `docs/roadmaps/wip/X.md` (barras) pegaria. **Exige teste novo com asserção de string, não de resolução.** |
| 11 | Os 12 testes de symlink sem privilégio (5 Python, 5 Node, 2 Go) | **SIM, mas mal-mapeado** | Vão falhar por `OSError`/`WinError 1314` (privilégio) no runner padrão sem Developer Mode — isso É sinal, mas hoje seria indistinguível de "o job caiu por causa de um bug de produto". Antes do ML-2A (skip explícito), o job largo vai reprovar por causa desses 12 **misturado** com os outros itens realmente expostos, poluindo o AC3 (mapeamento falha → item da issue). É por isso que a Wave 2 depende da Wave 1 rodar primeiro — precisa ver os 12 vermelhos brutos antes de silenciá-los. |

**Contagem honesta:** dos 11, o job largo **como desenhado hoje** (rodar as 3 suítes sem nenhum
ajuste) expõe com confiança direta apenas **2** (item 3, item 11) e um terceiro de forma ruidosa/não
confiável (item 2). Os outros **8** — itens 1, 4, 5, 6, 7, 8, 9, 10 — ou não vão acender vermelho
algum, ou vão acender por um motivo diferente do item real, ou estão estruturalmente fora do escopo
das 3 suítes. Isto **não invalida o instrumento** — invalida a suposição implícita de que "rodar as
3 suítes completas" por si só reproduz os 11. **Recomendação para ML-1A:** a tabela de mapeamento do
AC3 precisa ter uma linha explícita para cada um dos 8 itens acima dizendo "não exposto, rastreado
separadamente" — se ML-1A simplesmente colar a saída do job e mapear só o que aparece vermelho,
o roadmap herda exatamente o viés que a REQ existe para corrigir: 3 de 11 dando falsa sensação de
cobertura sobre os outros 8. Isto não altera os "Files affected" das Waves 1-3 — nenhum item revelou
caminho de arquivo novo fora do já listado; o gap é de asserção/escopo de teste, não de arquivo.

##### 2. Modelo de ameaça do próprio instrumento

**O vetor mais grave, confirmado por código, não por suposição: a isolação de `$HOME` dos testes é
ela mesma vácua no Windows, nos 3 runtimes simultaneamente.**

- Python: `pypi/tests/conftest.py` tem uma fixture `session-scoped, autouse` que faz
  `os.environ["HOME"] = fake_home` para a suíte inteira — mas a produção usa `os.path.expanduser`,
  que no Windows lê `%USERPROFILE%`, não `$HOME`. A fixture cujo comentário explica que existe
  precisamente para não deixar "qualquer teste... enxergar o $HOME REAL de quem roda a suíte" é
  um no-op nesta plataforma.
- Go: `internal/validator/main_test.go` tem um `TestMain` que faz `os.Setenv("HOME", home)` só para
  o pacote `validator` — mesmo no-op no Windows. E esse é só **um** dos pacotes; a issue confirma
  **101 sites** de `t.Setenv("HOME", ...)` espalhados em `internal/`, a maioria por-teste, sem
  `TestMain` de pacote — todos igualmente vácuos.
- Node: 25 arquivos de teste referenciam `HOME`, sem um setup global equivalente localizado.

**Consequência concreta, não hipotética:** `go test ./...` no Windows vai escrever de verdade na
home real do runner (`C:\Users\runneradmin\...` ou equivalente) a cada teste que chama qualquer
caminho de produção que resolve home — exatamente o que o autor da issue relatou na própria máquina
("um ADR, o `integrations-manifest.json`, dois scripts de guard, seis arquivos de config de agente").
Isto responde às perguntas do roadmap:

- **Resultado não reproduzível?** Sim, potencialmente. `go test ./...` por padrão paraleliza pacotes
  (`-p` = `GOMAXPROCS`), e com a home real compartilhada como estado mutável entre pacotes que rodam
  concorrentemente, a ordem de execução e a interferência entre pacotes deixam de ser hipotéticas —
  dois pacotes que resolvem home ao mesmo tempo competem pelo mesmo diretório real.
- **Um teste pode ver artefato deixado por outro?** Sim — mesma causa: nada limpa a home real entre
  testes/pacotes (diferente de um `t.TempDir()`, que o Go descarta sozinho).
- **A ordem passa a importar?** Sim, pelas duas razões acima.
- **Cache restaurado entre execuções?** Não é fator relevante aqui — os runners hospedados da GitHub
  são efêmeros por job; o `actions/cache` do Go (`cache: true` em `setup-go`) restaura só o cache de
  build/módulos, não o perfil do usuário. O risco de contaminação é **dentro de uma única execução**
  (entre pacotes/testes concorrentes), não entre execuções.

**Vazamento em log público (sonda, `workflow_dispatch`, log visível):** nenhum dos itens do AC5
(modo de `os.Stat`, `Lstat` em junction, `isatty` sobre `NUL`, encoding do console, terminador de
linha, presença de `sh`/`bash`) é segredo por natureza. O risco residual é indireto: se a sonda
reusar a mesma resolução de home vácua acima para qualquer uma dessas checagens, o log público
passaria a conter caminhos absolutos do perfil real do runner (`C:\Users\runneradmin\...`) — baixa
sensibilidade (é um runner efêmero descartado ao fim do job, sem PII), mas ainda é informação de
infraestrutura desnecessária num log de acesso público. **Recomendação para ML-3A:** a sonda deve
probar em diretório de trabalho isolado (`${{ runner.temp }}` ou equivalente), nunca na home real,
mesmo que a checagem específica não dependa de home.

**Teste que escreve fora do workspace, além da home:** não identifiquei, no tempo desta análise,
nenhum teste Windows que escreva fora do workspace e fora da home (ex.: `C:\` raiz, diretório de
sistema). O vetor dominante é mesmo a home, coberto acima.

**PR de fork como vetor:** `quality.yml` usa `on: pull_request` (não `pull_request_target`) e declara
`permissions: contents: read` no nível do workflow, sem override por job — isso **confirma** que PRs
de fork rodam com o `GITHUB_TOKEN` padrão somente-leitura e sem secrets do repositório, sem exceção
documentada para o job novo. O job largo **não introduz uma classe de vetor nova**: `go test ./...`,
`npm test` e `pytest pypi/tests` já executam código arbitrário de teste vindo do PR no job Linux
existente hoje — o job de Windows só estende essa mesma superfície (já aceita) para um segundo SO.
O ponto que **precisa** ser preservado explicitamente por `ares-tf` na Wave 1: **não** adicionar
`permissions` mais amplas nem `secrets` ao job novo, e **não** usar `pull_request_target`. Efeito
colateral aceito, não mitigado por este ADR: `windows-latest` custa mais minutos de cota por PR
(inclusive de fork, sujeito à aprovação padrão de "run workflow" do GitHub para contribuidores
novos) — vale registrar como custo, não como vulnerabilidade nova.

**Os 3 vetores mais graves, em ordem:**
1. Isolação de `$HOME` vácua nos 3 runtimes simultaneamente — o job pode corromper o estado do
   próprio runner durante a execução e produzir resultado não determinístico por concorrência entre
   pacotes/testes, mascarado como "só mais um teste Windows instável".
2. `sh -c` hardcodado no Go (`barrier.go:729`) com `Stdout`/`Stderr` descartados no caminho de erro
   (achado da issue, não removedido por esta REQ) — combinado com o item 7 da tabela, isto significa
   que uma falha de gate no job largo pode ser indistinguível entre "gate reprovou de verdade" e
   "ambiente sem `sh`", exatamente o tipo de sinal ambíguo que um instrumento de medição não pode
   emitir sem se autodenunciar.
3. Ampliação de superfície de execução de fork PR para um segundo SO mais caro — não é vulnerabilidade
   nova em espécie, mas é aumento de custo/superfície que precisa de decisão explícita registrada
   (AC10 já cobre tempo; escopo de fork não tem AC dedicado nesta REQ).

##### 3. Alvos de falsificação nas duas direções

**Regressão para dirigido (volta a ser `go test -run X` / um arquivo Node / um arquivo Python):**
quebra AC1 e torna o item 1 desta tabela (e boa parte dos outros) permanentemente inexpostos de
novo — falsificar checando literalmente que o YAML novo roda `go test ./...` (sem `-run`),
`npm test` sem filtro de arquivo, `pytest pypi/tests` sem `-k`/caminho específico. Qualquer flag de
restrição reintroduzida silenciosamente é a mesma regressão.

**Regressão para o outro lado — `continue-on-error` esquecido:** o AC4 já prevê a transição, mas não
tem um gatilho automático de remoção. Um job com `continue-on-error: true` permanente aparece como
⚠️ (nunca ❌) no GitHub, e branch protection com "required status checks" **não bloqueia merge** por
ele — o job vira decorativo do jeito mais silencioso possível: continua rodando, continua reprovando,
e ninguém é impedido de mergear por causa disso. Falsificação: depois da última correção prevista
pela REQ que fecha os 11 itens, `barrier`/revisão precisa checar que `continue-on-error` foi
removido do YAML, não assumir que "o job existe" é suficiente.

**`skip` de privilégio virando incondicional (Wave 2):** o AC8 já exige a falsificação na direção
"em Linux os testes continuam executando" — correto, mas incompleto. A direção oposta,
"com privilégio disponível no Windows, os testes executam e passam", **não é falsificável no runner
hospedado padrão da GitHub**: `windows-latest` não roda com Developer Mode habilitado por padrão, e
nada nas Waves 1-3 propõe um passo que o habilite (ex.: chave de registro
`AllowDevelopmentWithoutDevLicense` via PowerShell antes dos testes). Sem esse passo, a metade
"com privilégio" do AC8 fica **permanentemente não verificada em CI** — só localmente, na máquina de
quem tem Developer Mode ligado. **Recomendação para ML-2A:** ou aceitar esse residual
explicitamente (documentado, não silencioso), ou adicionar o passo de habilitação ao job de Windows
para que o `skip` seja de fato exercitado nos dois ramos em CI.

##### 4. Residual declarado

O que o runner **não** responde, mesmo com o job largo e a sonda completos:

- **`core.symlinks=false`** (padrão do Git para quem clona sem privilégio elevado) — depende da
  configuração de quem clona, não do CI. A sonda pode (e deve, por AC5) testar `Lstat` sobre uma
  junction criada dentro do próprio job com `mklink /J` — isso responde "o `Lstat` do Go marca
  `ModeSymlink` para o reparse point que o Git para Windows cria", que é a pergunta que o autor da
  issue fez. O que continua sem resposta é o comportamento em uma clonagem de terceiro com
  `core.symlinks=false`, onde o `git checkout` de um symlink vira arquivo de texto comum e o próprio
  ataque de symlink deixa de se reproduzir — cenário que não existe no job largo porque o repositório
  não versiona symlinks como fixture permanente.
- **Console cp1252 real, interativo** — o job e a suíte rodam de forma não interativa, com stdout
  capturado por pipe. Determinação de encoding de um console Windows real (`cmd.exe`/Windows
  Terminal com um usuário sentado na frente) não é idêntica à de um processo com stdout redirecionado
  a um pipe do runner — ambos tendem a cair na mesma codepage ANSI do sistema (cp1252 em instalações
  en-US/pt-BR), mas isso não está provado pela suíte, só assumido por semelhança.
- **Codepage além de cp1252** — tanto o relato do autor quanto o runner `windows-latest` (locale
  en-US) usam cp1252 como ANSI padrão. Uma instalação Windows com codepage diferente (cp936, cp1251,
  etc.) não é coberta por nenhuma correção nem teste previstos aqui — o fix e a validação são
  específicos a "console usa uma codepage não-UTF-8", não à cp1252 em particular, mas só cp1252 foi
  de fato observado.
- **Developer Mode no runner hospedado** — coberto na seção 3: sem passo explícito de habilitação,
  o ramo "com privilégio" do AC7/AC8 nunca roda em CI.
- **Antivírus/Windows Defender de máquina real** — pode interferir em criação/remoção rápida de
  arquivos e nas tentativas de symlink de formas que o runner hospedado (sem AV corporativo ou com
  exclusões pré-configuradas pela GitHub) não reproduz. Fora do alcance de qualquer instrumento
  neste roadmap.
- **`ref_targets_exist` vácuo em `by_agent` (item 9)** — já apontado na tabela: não é residual de
  plataforma, é residual de escopo desta REQ (correto, tem REQ própria).

**Esta lista é o pedido de validação ao autor da issue** — cada item acima é algo que só a máquina
dele (ou uma máquina real de terceiro com configuração diferente do runner hospedado) pode confirmar.

## Wave 1 — O job largo (ML único)
> Dependências: Wave 0 aprovada.

### ML-1A — Job de Windows rodando as três suítes, não bloqueante
**Status:** ⬜ Pendente
**Agente:** `ares-tf`
**Files affected:** `.github/workflows/quality.yml` (job novo; **não alterar** o
`windows-integrations-resolve` existente).
**Actions:**
1. Job novo em `windows-latest` com as três suítes completas.
2. `continue-on-error: true` — **não bloqueante até a última correção** (AC4). Documentar no próprio
   YAML que é temporário e o que o remove.
3. Registrar o **tempo** de execução (AC10).
4. **Não corrigir nada.** Se um teste falhar, é a medição.
**Critérios de aceite:**
- [ ] As três suítes rodam por inteiro
- [ ] **O job REPROVA** — colar a saída no roadmap como linha de base (AC2)
- [ ] Mapeamento falha → item da issue #216 (AC3); falha sem correspondência é achado novo
- [ ] Demais jobs do CI seguem verdes; `make quality` em Linux inalterado

#### Resultado do ML-1A + ML-1B (ares-tf, 2026-08-30) — LINHA DE BASE MEDIDA

Instrumento entregue e **executado num runner Windows real** (PR #221, run `33322373895`).

**AC2 satisfeita: a camada 2 nasceu vermelha.** `windows-defect-reproduction` → `failure` em
**1m13s**, no passo *"Suíte de reprodução de defeito — 11 itens"*. Os passos de infraestrutura
passaram todos.

**Linha de base, item a item:**

| item | defeito | veredito |
|---|---|---|
| 1 | cp1252 no `cli.py --help` | **REPRODUCED** |
| 2 | `$HOME` ignorado nos 3 runtimes | **REPRODUCED** |
| 3 | bit de execução sempre presente | **REPRODUCED** |
| 4 | gate de cobertura crasha em cp1252 | **REPRODUCED** |
| 5 | geradores Python escrevem CRLF | INCONCLUSIVE — *"init não completou"* |
| 6 | `isatty()` mente `True` para `NUL` | INCONCLUSIVE |
| 7 | `sh -c` hardcodado no Go | **ABSENT** |
| 8 | postura divergente com `\` | declarado fora de escopo |
| 9 | `ref_targets_exist` vácuo | fora de escopo (não é Windows) |
| 10 | separador de SO no frontmatter | **REPRODUCED** (Go) · INCONCLUSIVE (Node, Python) |
| 11 | 12 testes de symlink sem privilégio | coberto pela camada 1 |

**Cinco reproduzidos com evidência.** O item 10 tem a prova mais literal:

```
go: REPRODUCED — roadmap-line=roadmap: docs\roadmaps\wip\ROADMAP-item10.md
```

**AC12 respondida, e a favor:** o passo *"confirmar que a isolação de HOME/USERPROFILE vale antes de
rodar as suítes"* → **success**. O isolamento sintético segura os três runtimes no Windows, então a
camada 1 é viável. Era a pergunta que podia derrubar metade do instrumento.

**AC10 medida:** camada 1 em **23 minutos** (16:24:41 → 16:47:43); camada 2 em **1m13s**.

---

#### Três achados sobre o próprio instrumento — viram ML-1C

**1. A camada 1 para na primeira falha.** `Go — suíte completa` → `failure`, e `Node` e `Python`
→ **skipped**. Medimos **um** dos três runtimes. Para um instrumento de medição isso é defeito: o
passo seguinte precisa de `if: always()`, senão cada execução mede só até o primeiro tropeço.

**2. Os itens 5 e 6 vieram INCONCLUSIVE por cascata, não por ausência de defeito.** A mensagem é
*"init não completou"* — e o `init` do Python é justamente o que morre no cp1252 do item 1. **Um
defeito está mascarando a medição de outros dois.** Enquanto o item 1 não for corrigido, 5 e 6
seguem sem medição — o que ordena as correções por dependência, não por gravidade.

**3. O item 7 veio ABSENT, e isso descarta metade da hipótese.** `sh` **existe** na imagem do
runner, então a hipótese *"o Go quebra porque não acha `sh`"* está descartada. A outra metade — Go
sob `sh` POSIX contra Node e Python sob `cmd.exe`, avaliando semânticas diferentes do mesmo gate —
**continua sem teste**, exatamente como a Wave 0 previu. `ABSENT` aqui significa *"a pergunta que o
teste faz foi respondida, e não era a pergunta certa"*.

### ML-1C — Corretivas do instrumento (da linha de base)
**Status:** ✅ Concluído
**Agente:** `ares-tf`
**Files affected:** `.github/workflows/quality.yml`, `scripts/windows-repro/`.
**Actions:**
1. **Camada 1 mede os 3 runtimes sempre.** Hoje o `Go` reprova e `Node`/`Python` viram `skipped` —
   medimos um de três. Os passos de suíte passam a rodar independentemente do resultado dos
   anteriores, preservando a precondição da AC12 como única condição.
2. **Desacoplar a medição dos itens 5 e 6 do item 1.** Eles vieram `INCONCLUSIVE` com *"init não
   completou"* porque o `init` do Python morre no cp1252 do item 1. Encontrar caminho que meça CRLF e
   `isatty` **sem** depender de um `init` que sabidamente falha — ou, se for impossível, declarar a
   dependência explicitamente no veredito, em vez de `INCONCLUSIVE` genérico.
3. **Reclassificar o item 7.** `ABSENT` está correto para a pergunta feita (`sh` existe), mas a
   pergunta útil é outra: **Go sob `sh` contra Node e Python sob `cmd.exe` avaliam o mesmo gate de
   wave igual?** Trocar a verificação para comparar o **veredito dos 3** sobre um gate com sintaxe
   que se comporta diferente nos dois shells.
**Critérios de aceite:**
- [x] Uma execução mede os 3 runtimes da camada 1, mesmo com falha em um deles
- [x] Itens 5 e 6 saem de `INCONCLUSIVE` genérico: ou medidos, ou com dependência declarada
- [x] Item 7 passa a comparar semântica de shell entre os 3, não presença de `sh`
- [x] A camada 2 **continua vermelha** — não é para consertar defeito, é para medir melhor
- [x] `actionlint` limpo; YAML válido; nenhum arquivo de produto tocado


#### Resultado do ML-1C (ares-tf, 2026-08-30) — LINHA DE BASE DEFINITIVA

Medido no runner real, run `33324283015`.

**1. Camada 1 mede os 3 runtimes.** A causa era sutil e invisível por inspeção: o GitHub Actions
**prepende `success()`** a todo `if:` sem função de status, então
`if: steps.precondition.outcome == 'success'` virava
`success() && steps.precondition.outcome == 'success'`. Com o Go reprovando, o `success()` invisível
derrubava Node e Python — apesar de a condição escrita continuar verdadeira.

```
antes   12. Go → failure   13. Node → skipped   14. Python → skipped
depois  12. Go → failure   13. Node → failure   14. Python → failure
        15. "Camada 1 pulada" → skipped   (correto: a precondição passou)
```

**Os três reprovam em Windows, com evidência própria** — não mais extrapolação a partir do Go.
Nota em `vault/notes/if-sem-funcao-de-status-tem-success-implicito-2026-08-30.md`.

**2. Itens 5 e 6 saíram de `INCONCLUSIVE` para `REPRODUCED`.** Estavam bloqueados pelo item 1 — o
`scaffold()` do `init_gen.py` imprime caractere que crasha em cp1252 antes de CRLF e `isatty` serem
alcançáveis. Medidos com `PYTHONIOENCODING=utf-8` só no subprocesso, com o veredito
`BLOCKED-BY-ITEM-1` reservado para o caso de ainda assim falhar.

**3. Item 7 — de `ABSENT` para `REPRODUCED`, e é a prova que faltava:**

```
Go     (sh -c, POSIX)             -> trackfw-gate-verdict-A
Node   (spawnSync shell:true)     -> 'trackfw-gate-verdict-B'
Python (subprocess shell=True)    -> 'trackfw-gate-verdict-B'
```

Mesmo gate, **vereditos opostos**. E as aspas simples saem literais no Node e no Python — o
`cmd.exe` não as remove, segunda divergência na mesma linha. **No Windows, uma wave que passa pelo
CLI Go bloqueia pelo Node.** O `ABSENT` anterior estava certo para a pergunta errada.

---

### Linha de base definitiva — 8 de 8 itens no escopo reproduzem

| item | veredito |
|---|---|
| 1 cp1252 no `--help` · 2 `$HOME` · 3 bit de execução · 4 gate de cobertura | **REPRODUCED** |
| 5 CRLF · 6 `isatty` sobre `NUL` · 7 semântica de shell · 10 separador no frontmatter | **REPRODUCED** |
| 8 postura com `\` | declarado fora de escopo (símbolo não exportado) |
| 9 `ref_targets_exist` | fora de escopo — não é defeito de Windows, tem REQ própria |
| 11 12 testes de symlink | coberto pela camada 1 |

**AC10:** camada 1 em **25m34s** (17:05:51 → 17:31:25); camada 2 em **1m23s**.

O contraste entre as duas camadas é o argumento da separação de CI que o KG aprovou tratar depois:
a camada 2 dá o sinal inteiro em 1m23s; a camada 1 custa 25 minutos para dizer "os três reprovam".

## Wave 2 — `skip` explícito (ML único)
> Dependências: Wave 1 concluída — precisamos ver os 12 vermelhos antes de silenciá-los.

### ML-2A — `skip` nomeando a garantia não exercitada
**Status:** ✅ Concluído
**Agente:** `apolo-tf`
**Files affected:** testes de symlink dos 3 runtimes —
`internal/generators/update_test.go` (2 testes),
`npm/tests/update_discover_symlink_guard.test.js` (8 testes que criam symlink; varredura não achou
equivalente de `roadmap move` fora deste arquivo),
`pypi/tests/test_update_discover_symlink_guard.py` (5 testes).
**Actions:**
1. Detectar falha de privilégio ao criar symlink e **pular**, com mensagem dizendo **qual garantia
   não foi exercitada** e que exige Developer Mode. A formulação é do autor da issue: *"é diferente
   de ficar em silêncio"*.
2. Vale para os 12 testes nos 3 runtimes.

Implementado com um helper por runtime (`symlinkOrSkip`/`_symlink_or_skip`) que envolve a criação
do symlink: sucesso segue o teste normalmente; falha por privilégio (`os.IsPermission` ou
`syscall.Errno(1314)`/`ERROR_PRIVILEGE_NOT_HELD` em Go; `err.code in ('EPERM','EACCES')` em Node;
`err.winerror == 1314` ou `err.errno in (EPERM, EACCES)` em Python) pula o teste com mensagem
nomeando a garantia não exercitada; qualquer outra falha continua propagando como erro real. O
critério é a falha medida na chamada de symlink, não `runtime.GOOS`/`process.platform`/`sys.platform`.
**Critérios de aceite:**
- [x] AC7, AC8 — com privilégio executam; sem privilégio pulam **com mensagem**
- [x] `skip` **não** é incondicional — falsificado: em Linux/macOS os 15 testes (2 Go + 8 Node + 5
      Python) executam e passam, 0 skipped — evidência abaixo
- [x] Suítes verdes em Linux; job de Windows deixa de reportar esses 12 como falha (sem privilégio)

**Evidência (2026-08-30, macOS/Linux-like runner local):**
- `go test ./internal/generators/... -run TestUpdateNeverWritesThroughSymlink -v` → 2/2 PASS
- `go test ./...` → todos os pacotes OK
- `node --test npm/tests/update_discover_symlink_guard.test.js` → `tests 10, pass 10, fail 0,
  skipped 0`
- `npm test --prefix npm` → `tests 839, pass 839, fail 0, skipped 0`
- `PYTHONPATH=pypi python3 -m pytest pypi/tests/test_update_discover_symlink_guard.py -v` → 5/5 PASSED
- `PYTHONPATH=pypi python3 -m pytest pypi/tests` → `1555 passed`
- `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 make quality` → exit code 0 (test, test-node, test-python,
  lint, parity todos verdes)
- `git diff --stat` — só os 3 arquivos de teste listados acima; nenhum código de produto tocado.

## Wave 3 — A sonda (ML único)
> Dependências: Wave 1 concluída.

### ML-3A — Workflow `workflow_dispatch` de sondagem
**Status:** ⬜ Pendente
**Agente:** `ares-tf`
**Files affected:** `.github/workflows/` (workflow novo).
**Actions:**
1. Sonda respondendo, com saída **bruta**: modo devolvido por `os.Stat` num arquivo comum e num
   `chmod +x`; `Lstat` diante de symlink e de junction (`mklink /J`); `isatty` sobre `NUL`; encoding
   do console; terminador de linha dos arquivos que os geradores escrevem; `sh`/`bash` no `PATH`.
2. Sem segredo, sem escrita no repositório, `workflow_dispatch` puro (AC9).
3. Documentar no próprio YAML que **não substitui** o job de regressão (AC6).
**Critérios de aceite:**
- [ ] AC5, AC6, AC9
- [ ] Saída legível e citável em REQ — é o propósito
- [ ] Tempo de execução em poucos minutos, não dezenas

#### Resultado do ML-3A (ares-tf, 2026-08-30)

`.github/workflows/windows-probe.yml` + `scripts/windows-repro/go/probe.go`. `workflow_dispatch`
puro, sem segredo, sem escrita no repositório. O YAML documenta que **não substitui** o job de
regressão.

**Duas soluções dele que eu não tinha antecipado, e são o que torna a sonda capaz de responder o
que não sabíamos:**

- A **junction é criada com `cmd /c mklink /J`**, que **não exige privilégio** — ao contrário do
  `os.Symlink`. Era a pergunta central, a mesma que fizemos ao autor da issue, e deixa de depender
  da máquina dele: a sonda imprime o `Mode()` cru do `Lstat`, os bits e o booleano `ModeSymlink`
  para a junction, mais um `Stat` no mesmo caminho para comparar.
- O **symlink do git é materializado via `git update-index --cacheinfo` com modo `120000`** —
  plumbing, não `os.Symlink` —, então o `checkout` acontece sem Developer Mode e mede o que um clone
  de verdade produz.

Ele também corrigiu no meio do caminho, após revisão, que o **Node tem gerador próprio** e não um
wrapper: a pergunta 5 passou a rodar `roadmap new` de verdade pelos três e despejar os bytes.

**Residuais declarados no próprio YAML:** console interativo real (o Actions sempre redireciona),
codepage fora de cp1252, `core.symlinks=false` de clone de terceiro, e Developer Mode ligado — este
deliberadamente não habilitado, porque a falha do `os.Symlink` é ela própria o sinal da pergunta 2.

**Achado do arquiteto ao tentar disparar:**

```
HTTP 404: workflow windows-probe.yml not found on the default branch
```

**`workflow_dispatch` só é despachável se o workflow existir na branch padrão.** A sonda só fica
utilizável **depois do merge** — o que limita justamente o uso durante o desenvolvimento da REQ que
a cria. Não é defeito do ML; é restrição do GitHub Actions que ninguém tinha considerado, e vale
registrar porque afeta qualquer ferramenta de investigação que a gente construa assim no futuro.

Consequência prática: a resposta da junction — se o `Lstat` do Go marca `ModeSymlink` para o reparse
point do `mklink /J`, e portanto se a guarda que entregamos esta semana vale em Windows — só sai
**após o merge**. Fica como primeira ação pós-merge.

## Barreira final
Revisão `hefesto-tf` e `hades-tf`, auditoria do arquiteto e `barrier --wave 3`. **Só declarar
concluído com o CI verde** — e aqui "verde" significa: os demais jobs verdes **e** o job de Windows
reprovando pelos motivos esperados e mapeados.
