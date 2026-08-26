---
status: done
date: 2026-08-23
req: "docs/req/REQ-2026-08-23-titulo-de-roadmap-new-forja-secao-com-gate-que-o-barrier-executa.md"
squad: "hades-tf, apolo-tf"
---

# Roadmap: barrier nao executa gate de roadmap nao confiavel e roadmap new sanitiza o titulo

> Created: 2026-08-23 | Status: done

## Context

REQ: `docs/req/REQ-2026-08-23-titulo-de-roadmap-new-forja-secao-com-gate-que-o-barrier-executa.md`
ADR: `docs/adr/ADR-2026-08-23-barrier-nao-executa-gate-de-roadmap-nao-confiavel-e-roadmap-new-sanitiza-o-titulo.md`

`trackfw barrier` executa o bloco de gates via `sh -c`. Um roadmap que chega por **PR de terceiro**
faz o mantenedor executar shell que ele nunca aceitou. O título de `roadmap new`, interpolado sem
sanitizar newline, é o vetor plantável — o menor dos dois.

**Reproduzido:** `test -f /tmp/PWNED_TEST` → EXISTE, com `result: blocked` na mesma execução.


## Acceptance Criteria

- [ ] AC1–AC10 da REQ, integralmente
- [ ] 🔴 **AC5 é o critério que decide a entrega:** o fluxo normal de implementação (roadmap
      modificado e não commitado) não pode virar fricção que faça desligar o controle
- [ ] `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 make quality` exit 0 (exit code medido)

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Modelo de ameaça do discriminante de confiança
**Status:** ✅ Concluído · **Agente:** `hades-tf` (`subagent_type: hades-tf`)
**Escreve:** `docs/seguranca/2026-08-23-modelo-de-ameaca-do-gate-nao-confiavel.md`

**A decisão central do AC4 está em aberto e é sua:** qual é o discriminante de confiança, e como o
consentimento é dado no fluxo normal. Recomende **uma** forma, com o motivo e o que ela custa.

**Actions:**
1. Enumeration completeness — is the list of surfaces in this roadmap complete? Name what is missing, or show the list is closed. Do not limit the search to the files already named by the REQ — before declaring the list closed, search the repository for other places that emit the same artifact or the same pattern (for example, grep for the literal the final artifact contains).
2. Threat model — who empties this Wave 0 without breaking any written rule, and how?
3. Falsification targets in both directions — for each surface, what breaks when the behavior regresses, and what breaks when it regresses the opposite way?
4. Declared residual — what this design accepts not covering.
**Acceptance criteria:**
- [ ] The four sections above answered with evidence, not a one-line assertion
- [ ] No implementation line written for this ML

**Gates da wave:**
```bash
test -f docs/seguranca/2026-08-23-modelo-de-ameaca-do-gate-nao-confiavel.md
grep -q "Completude de enumera" docs/seguranca/2026-08-23-modelo-de-ameaca-do-gate-nao-confiavel.md
grep -q "Residual declarado" docs/seguranca/2026-08-23-modelo-de-ameaca-do-gate-nao-confiavel.md
grep -q "discriminante" docs/seguranca/2026-08-23-modelo-de-ameaca-do-gate-nao-confiavel.md
```


---

### Auditoria do ML-0A — aprovada, com um achado meu que virou AC13

**A recomendação resolve a tensão do AC5 sem ceder no vetor:** comparar contra **`origin/main`**, com
`--trust-local-gates` **injetado pelo slash command**.

- **`HEAD` não serve** — o roadmap do PR **está** commitado na branch do PR, então HEAD-comparison o
  marcaria como confiável: fecharia a usabilidade **sem fechar o vetor**.
- **Flag obrigatória universal** viraria costume de digitá-la sempre — o *"guard que o usuário
  desliga"* do `ADR-2026-08-17`.
- A separação é o ponto: **fluxo de agente** (dominante, slash command) sem fricção; **revisão de PR**
  (CLI direta) protegida por padrão.

**Ele mediu o caso dominante**, não supôs: o `barrier` roda **antes** do commit de conclusão do ML,
logo o roadmap está sempre modificado e não commitado — confirmado nos registros desta série.

**Confirmei a ordem de execução no código:** `barrier.go:506-525` compõe o veredito **depois** de
rodar os comandos. Por isso o gate da direção (b) tem de verificar **ausência do arquivo**, não só
exit code → **AC14**.

**Enumeração:** buscou `sh -c`, `exec.Command`, `subprocess.run(shell=True)` e
`spawnSync({shell:true})` nos 3 stacks; o `barrier` é o **único** ponto que executa shell derivado de
conteúdo de arquivo versionado. Os demais `exec` usam args estruturados.

#### 🔴 O que eu achei auditando a recomendação

**O slash command vive no repositório** — `.claude/commands/trackfw/barrier.md` está versionado aqui.
Um PR hostil pode **editar o próprio slash command** para incluir `--trust-local-gates` e recuperar a
execução. **A proteção guarda a chave dentro da porta que ela tranca.** → **AC13**: a entrega diz o
que impede isso, ou declara o residual com o motivo.

**Residuais dele, aceitos:** roadmap commitado e mergeado com gate hostil continua executando
(fronteira é revisão de código) · mantenedor que revisa PR pelo slash command contorna a proteção ·
store de hashes pré-aprovados descartado, com motivo.

---
## Wave 1 — Sanitização do título

> Dependências: ML-0A auditado. **Parte barata e independente da decisão do discriminante.**

### ML-1A — `roadmap new` sanitiza o título nos 3 CLIs
**Status:** ✅ Concluído · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`) · **Dep.:** ML-0A

`internal/generators/roadmap.go:150` e o caminho `--from-req`, mais os equivalentes Node e Python.
Newline e retorno de carro no título são entrada malformada.

**Critérios de aceite:** AC1, AC2 · fixture com o título forjado do exemplo · `make quality` exit 0

---

### Auditoria do ML-1A — aprovada; medi os dois lados

```
titulo forjado (\n + bloco de gates):
  Go/Node/Python  "Error: roadmap title must be a single line: newline and
                   carriage return are not allowed"      byte-identico
  arquivos criados: 0        <- rejeita ANTES de escrever

titulos legitimos:
  "Corrige acentuacao e c"             exit 0 · gates=1
  "feat: dois-pontos no titulo"        exit 0 · gates=1
  "com (parenteses) e hifen-composto"  exit 0 · gates=1

make quality (CI-exata, minha)  exit 0
validate                        16 warnings, 0 violations
```

**O falso-positivo — que era o que reprovaria — não existe.** Acento, `ç`, dois-pontos, parênteses e
hífen passam, e cada roadmap sai com **exatamente um** bloco de gates: o legítimo da Wave 0.

**Escolha dele que endosso:** rejeitar em vez de neutralizar em silêncio. Neutralizar deixaria o
usuário com título mutilado sem saber por quê — indistinguível de bug. Rejeição é contrato
verificável, e a mensagem saiu byte-idêntica nos 3 runtimes.

**Cuidado dele que não estava no meu handoff:** testou REQ com fim de linha **CRLF** no caminho
`--from-req`, para garantir que arquivo salvo no Windows não vire falso-positivo.

**Observação sobre o vetor `--from-req`:** ele nota que os parsers dos 3 CLIs são line-based por
construção, então critério extraído de REQ **não pode** conter `\n` embutido — a validação ali é
defense-in-depth, não a única barreira. Registrado como raciocínio dele, não como medição minha.

---

## Wave 2 — Discriminante de confiança no `barrier`

> Dependências: ML-1A auditado. **A forma vem da decisão do ML-0A.**

### ML-2A — `barrier` recusa gate de roadmap não confiável
**Status:** ✅ Concluído · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`) · **Dep.:** ML-1A

**Critérios de aceite:** AC3, AC4, AC5, AC6 · **prova de que o fluxo normal segue usável**

---

### Auditoria do ML-2A — aprovada; o vetor está fechado, provado com roadmap hostil meu

```
roadmap com gate 'touch /tmp/TRUST_REGRESSION', nao commitado em origin/main

CLI direta:
  gates: not_evaluated
    "roadmap is not committed in origin/main — pass --trust-local-gates to evaluate local gates"
  test -f /tmp/TRUST_REGRESSION  ->  NAO existe        <- ausencia de efeito, nao exit code
com --trust-local-gates:
  gates: passed                                         <- fluxo dominante intacto

make quality (CI-exata, minha)  exit 0
validate                        16 warnings, 0 violations
```

**Paridade conferida onde ela é contrato.** O **texto** dos 3 CLIs diverge bastante (cabeçalhos e
símbolos distintos) — e isso é **pré-existente e por desenho**: `check-barrier.sh` compara **JSON
normalizado**, não texto. O JSON bate:

```
go == node    True
go == py      difere so em started_at/finished_at (timestamps — o gate normaliza)
gates.status  not_evaluated
```

**AC13 respondido com honestidade, não com remendo.** O parecer dele em `cli-parity.md`:

> *"A flag vinda de um slash command hostil, de um `Makefile` hostil, de um passo de CI hostil e a
> invocação consciente do mantenedor são **indistinguíveis em `argv`**. Verificar o `barrier.md`
> contra `origin/main` não ajuda — o CLI nunca lê esse arquivo; quem lê e executa é o agente."*

É a resposta certa: a alternativa seria fingir uma proteção que o CLI não tem como oferecer. O slash
command passou a carregar o aviso de **não** usar a flag ao revisar roadmap de PR de terceiro.

**Residual que ele declara para a Wave 3:** os cenários cross-CLI de `not_evaluated` no
`check-barrier.sh` ainda não existem — as strings estão pinadas no `cli-parity.md` como `gap`, não
verificadas por gate. É exatamente o próximo ML.

---

## Wave 3 — Gate

### ML-3A — Paridade e falsificação nas duas direções
**Status:** ✅ Concluído · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`) · **Dep.:** ML-2A

**Critérios de aceite:** AC7, AC8, AC9, AC10

---

### Auditoria do ML-3A — aprovada; sabotagem minha acusa pelo motivo certo

```
sabotagem: roadmapTrustForGates() -> retorna trusted:true sempre
  check-barrier.sh -> EXIT 1
    FAIL [barrier/trust/not-committed/go]: hostile gate EXECUTED —
         sentinel was created; trust check failed to block execution
restaurado -> EXIT 0

make quality (CI-exata, minha)  exit 0, 172 cenarios
validate                        16 warnings, 0 violations
```

**A mensagem é a prova certa:** não "status divergiu" nem "exit code errado", mas **"o gate hostil
EXECUTOU"**, detectado pela criação do arquivo sentinela. É a única forma que serve, porque o defeito
original executava e **depois** reportava `blocked`. O cenário ainda verifica a ausência do sentinela
**antes** de cada runtime, para não confundir "não executou agora" com "o runtime anterior já criou".

#### O cenário 171 nasceu com o mesmo defeito do 170 — e o `assert_fails_with` recusou os dois

```
FAIL [falsify/ac2-sanitization/direction-a-detected]: saiu com 1 mas falta
  diagnostico 'expected 'roadmap title must be a single line''
  output real: FAIL [barrier/ac2-sanitization/go]: expected exit non-0 for forged title, got 0
```

**Causa raiz que ele nomeou na correção, e que explica as duas ocorrências:** o padrão mirava a
mensagem do **CLI**, mas o `assert_fails_with` observa a saída do **gate**, que emite diagnóstico
próprio. Dois níveis de mensagem; o padrão apontava para o de baixo.

**O valor do mecanismo ficou demonstrado duas vezes na mesma REQ:** uma prova que falha pelo motivo
errado não é prova, e sem o `assert_fails_with` teríamos dois cenários "verdes" atestando o que não
verificam.

**Residual declarado e aceito:** os cenários 171–172 são Go-only, porque a sabotagem é no binário Go
— mas o `check-barrier.sh`, que eles usam como alvo, verifica os **3** runtimes. A cobertura cross-CLI
vem por composição, não por triplicar a sabotagem.

---

## Wave 4 — Barreira

### ML-4A — Reverificação
**Status:** ✅ Concluído · **Agente:** `hades-tf` (`subagent_type: hades-tf`)

Quem escreveu a Wave 0 verifica se a implementação honra o que ela decidiu. **Veredito explícito.**

---

### Auditoria do ML-4A — **APROVADO**, com um achado adjacente que eu escalo

Parecer: `docs/seguranca/2026-08-23-barreira-do-gate-nao-confiavel.md`.

**O que ele mediu e passou:** o discriminante se comporta como a Wave 0 previu nos casos que ela
nomeou · `not_evaluated` não é confundível com `passed` em nenhum consumidor · a sanitização não tem
contorno por **U+2028/U+2029** — ele confirmou nos 3 runtimes que o parser usa `split("\n")` e não
`splitlines()`, então o separador Unicode não abre linha nova · `make quality` exit 0, 377 cenários.

**O AC5 — o critério que reprovaria mesmo com o vetor fechado — passa:** o fluxo dominante não ganhou
nenhuma interação a mais, porque o consentimento vem do slash command.

#### O achado adjacente, e é maior que o gate

Perguntei se o AC13 tinha **irmãos**. Tem: os **scripts de hook versionados** referenciados pelo
`.claude/settings.json` do projeto — mais `Makefile` e passos de CI.

**A superfície de hook é mais ampla que a do gate:** ela **não exige rodar `trackfw barrier`**. Um
checkout de PR hostil executa hook na máquina do mantenedor assim que ele usa a ferramenta. O gate
que fechamos exige uma ação; o hook não exige nenhuma.

Nomeei os irmãos no mesmo residual do `docs/cli-parity.md` — `check-parity-contract-coverage.sh`
exit 0. **Não corrigi nada além disso**, e é deliberado: a mitigação real dessa classe não é código,
é revisão de diff, e mudar isso é decisão de outro ciclo.

**Entrega completa.**

---

## Notas
- **Fora de escopo:** tudo listado no *Negative scope* da REQ.
- Commits, branch e PR são exclusivos do `trackfw_architect`.
