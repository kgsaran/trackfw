---
status: done
date: 2026-08-22
req: "docs/req/REQ-2026-08-21-validate-nao-detecta-hook-com-pwd-que-falha-fora-da-raiz.md"
adr: "docs/adr/ADR-2026-08-22-postura-do-validate-diante-de-formas-de-hook-nao-reconhecidas-classificar-por-ancoragem-nao-por-casamento-com-o-gerado.md"
squad: "apolo-tf, hades-tf"
---

# Roadmap: `validate` detecta hook com `$PWD` que falha fora da raiz

> Created: 2026-08-22 | Status: done

## Context

REQ: `docs/req/REQ-2026-08-21-validate-nao-detecta-hook-com-pwd-que-falha-fora-da-raiz.md`
ADR: `docs/adr/ADR-2026-08-22-postura-do-validate-diante-de-formas-de-hook-nao-reconhecidas-classificar-por-ancoragem-nao-por-casamento-com-o-gerado.md`

`$PWD/scripts/trackfw-credential-guard.sh` falha fora da raiz exatamente como a forma relativa pura,
e o `validate` fica **em silêncio**. Pior: é o erro que alguém comete **tentando consertar** depois
de receber a mensagem que a REQ-2026-08-17 introduziu.

**Motivo de estar na 7.2.0 (decisão do KG):** a mensagem que induz o erro sai nesta release; a
correção sai junto.

## Acceptance Criteria

- [ ] AC1–AC6 da REQ, integralmente
- [ ] **Caminho absoluto silencioso** — o falso-positivo que reprova a entrega
- [ ] `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 make quality` exit 0 (invocação CI-exata)
- [ ] `./bin/trackfw validate` sem violations novas

## A decisão já está tomada — este roadmap implementa, não delibera

O ADR decide: **o discriminante é a semântica de ancoragem, não o casamento com o que o gerador
emite.** Três classes, avaliadas só para os CLIs com `requiresVarOrShellPrefix=true`
(Claude Code, Codex CLI, Gemini CLI):

| Classe | Formas | Veredito |
|---|---|---|
| **1. Ancorado** | `$CLAUDE_PROJECT_DIR/…`, `$GEMINI_PROJECT_DIR/…`, `"$(git rev-parse --show-toplevel)/…"`, **caminho absoluto** | silêncio |
| **2. Dependente do cwd** | `$PWD/…`, `"$PWD/…"` (entre aspas), `./…`, `../…`, relativo puro | **acusar** |
| **3. Indecidível** | `$OUTRA_VAR/…`, `$UNDEFINED/…`, formas não reconhecidas | silêncio, residual declarado |

**O dado que fundamenta a classe 1 e não pode ser perdido na implementação:** caminho absoluto cai
hoje no `default: return "", false` de `resolveCredentialGuardHookPath`
(`internal/validator/validator_credential_guard.go:88`), porque a cláusula do relativo puro exige
`!filepath.IsAbs(raw)`. Ele é silencioso **e deve continuar sendo** — ancora, funciona de qualquer
cwd, é wiring customizado legítimo. Acusá-lo é o falso-positivo que reprova a entrega.

## Riscos que valem para todos os MLs

1. **Falso-positivo é o risco dominante.** Pelo `ADR-2026-08-17`, guard que atrapalha é guard que o
   usuário desliga. Classe 1 e classe 3 em silêncio são critério de aceite, não detalhe.
2. **Classe 2 é um predicado, não uma lista de literais.** O critério é *"expande a partir do
   diretório corrente"*. Escreva o teste exercendo a função de classificação — uma lista de strings
   é a "condição estreita demais" que esta série já nomeou nove vezes.
3. **Cuidado com o instrumento:** fixture com `settings.json` malformado dá falso negativo
   indistinguível de regra que não detecta. Use heredoc e **valide o JSON** antes de confiar — foi o
   que quase invalidou o ML-1B da REQ anterior.
4. **Invocação CI-exata:** `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 make quality`.
5. Binário do `PATH` desatualizado; `--version` **não** distingue o build. Use `./bin/trackfw`.
6. Commits, branch e PR são exclusivos do `trackfw_architect`.

---

## Wave 1 — O classificador

> Dependências: nenhuma.
> **ML único, não paralelizável:** os 3 stacks precisam sair byte-idênticos. Dividir por linguagem é
> o que produziu as divergências das séries anteriores.

### ML-1A — Classificação por ancoragem nos 3 CLIs
**Status:** ✅ Concluído · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`)

**Arquivos afetados:**
- Go: `internal/validator/validator_credential_guard.go` + testes
- Node.js: o equivalente em `npm/src/validator/`
- Python: o equivalente em `pypi/trackfw/validator.py`
- **Proibido:** mudar a decisão de qual forma cada CLI usa (`credentialGuardHookFiles`), tocar nos
  geradores, ou alterar o `resolveCredentialGuardHookPath` para além do necessário à classificação.

**Ações:**
1. Introduzir a função de classificação (classe 1 / 2 / 3) nos 3 stacks, com o **mesmo nome** e a
   mesma semântica. `isRelativePureForGuard` passa a ser um caso da classe 2, não uma regra à parte.
2. Classe 2 acusa **apenas** para `requiresVarOrShellPrefix=true`. Cursor/Copilot/Kiro seguem
   intocados — a guarda por CLI continua sendo o primeiro operando, curto-circuitando antes de olhar
   o valor.
3. **Remover aspas envolventes ANTES de classificar.** A cláusula de hoje começa com
   `!strings.HasPrefix(raw, "\"")`, então `"$PWD/scripts/…"` cai no `default` e fica **silenciosa** —
   o mesmo erro do usuário, só que entre aspas, escaparia da correção inteira. É o achado D.3 da
   barreira anterior, e a decisão do ADR o cobre de graça: aspas são sintaxe, não semântica de
   ancoragem. **Não** confundir com a forma do Codex (`"$(git rev-parse --show-toplevel)/…"`), que
   é classe 1 e continua silenciosa.
4. **Mensagem específica por forma**, explicando por que não ancora. Para `$PWD`:
   *`$PWD` expande para o diretório corrente, não para a raiz do projeto* — dizer só "forma inválida"
   repete o engano que levou o usuário até ali. Mantenha o remédio (`trackfw update`) que a mensagem
   atual já traz.
5. Testes por stack cobrindo as três classes, incluindo **caminho absoluto silencioso** e
   **`$OUTRA_VAR/` silenciosa**.

**Critérios de aceite:**
- [ ] AC2 da REQ — `$PWD/scripts/…` acusado em Claude/Codex/Gemini
- [ ] AC3 da REQ — formas legítimas limpas nos **6** CLIs
- [ ] `"$PWD/…"` **entre aspas** também acusado (achado D.3), e a forma do Codex
      (`"$(git rev-parse --show-toplevel)/…"`) **continua silenciosa**
- [ ] **Não-regressão, não detecção nova:** `./…`, `../…` e `sh scripts/…` já são acusados hoje via
      `isRelativePureForGuard` (barreira de 2026-08-21, lista CAPTURADAS). Os testes provam que
      seguem acusados após a refatoração — passar neles não é evidência de trabalho novo
- [ ] **Caminho absoluto silencioso** — o falso-positivo que reprova a entrega
- [ ] `$OUTRA_VAR/…` silenciosa (classe 3), com o residual citado em comentário no código
- [ ] `make build` · `go test ./...` · testes Node e Python verdes
- [ ] Saída dos 3 runtimes comparada byte a byte nos casos acima, evidência colada
- [ ] `./bin/trackfw validate` — nenhum hook real deste repositório acusado

---

## Wave 2 — Gate

> Dependências: ML-1A completo e auditado.

### ML-2A — Paridade + falsificação nas duas direções
**Status:** ✅ Concluído · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`) · **Dep.:** ML-1A

**Arquivos afetados:**
- `scripts/check-validate-parity.sh` (estender os blocos CG e GBG existentes — **não** criar gate
  novo; a fronteira com `credential_guard_hook_resolvable` já é coberta ali)
- `scripts/check-gates-falsify.sh` (2 cenários; atualizar o total no echo final)
- `docs/cli-parity.md` (anotação `trackfw-contract` do bloco afetado)
- **Proibido:** qualquer arquivo do ML-1A.

**Ações:**
1. Casos byte-idênticos nos 3 CLIs: `$PWD` acusado · absoluto silencioso · `$OUTRA_VAR/` silenciosa ·
   Cursor com relativo silencioso (não-regressão do AC3 da REQ anterior). Guard de vacuidade em cada.
2. Falsificação **nas duas direções**:
   - direção A — suprimir a acusação de `$PWD` ⇒ o gate falha;
   - direção B — fazer a classe 1 (**caminho absoluto**) ser acusada ⇒ o gate falha.
   A direção B é a que protege o defeito caro desta entrega.
3. Atualizar a anotação de contrato; `check-parity-contract-coverage.sh` é bloqueante.

**Critérios de aceite:**
- [ ] AC4 e AC5 da REQ
- [ ] `bash scripts/check-validate-parity.sh` verde, com os casos novos
- [ ] Os 2 cenários com baseline verde **e** braço de detecção vermelho, saída colada
- [ ] `bash scripts/check-parity-contract-coverage.sh` exit 0
- [ ] `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 make quality` **exit 0** — reporte o **exit code**

---

### Auditoria do ML-1A e do ML-2A — aprovadas, com dois vermelhos meus no caminho

**Comportamento medido por mim, com fixtures próprios (não pelo relatório):**

```
Claude Code:
  $PWD/scripts/...                        acusa=1   <- deteccao nova
  "$PWD/scripts/..."  (entre aspas)       acusa=1   <- fecha o achado D.3
  ./scripts/...                           acusa=1   <- nao-regressao
  /opt/x/scripts/...  (absoluto)          acusa=0   <- o falso-positivo que reprovaria
  $MY_ROOT/scripts/... (classe 3)         acusa=0
  $CLAUDE_PROJECT_DIR/scripts/...         acusa=0
  "$(git rev-parse --show-toplevel)/..."  acusa=0   <- forma do Codex intacta
Cursor (os tres valores):                 acusa=0   <- imune por construcao
```

**Sabotagem própria, numa direção que os cenários dele NÃO cobrem** — neutralizei
`stripOuterQuotesForClassify`:

```
gate_exit=1
  claude-pwd-quoted/go: mensagem inesperada — esperava 'with a $PWD path',
  obteve 'with a bare relative path'
restaurado -> exit 0, arquivo IDENTICO ao entregue
```

O que isso revela: sem o strip, `"$PWD/…"` **ainda é acusado** (cai na cláusula do relativo puro),
mas com a **mensagem errada** — a que não explica que `$PWD` expande para o cwd. **O gate pina a
mensagem, não só o veredito.** Numa regra cuja função é ensinar o usuário a consertar, dizer a coisa
errada é o defeito.

#### Dois vermelhos que ele declarou verdes

1. **Cenário 159 invalidado pelo ML-1A.** O `sed` sabotava
   `hf.requiresVarOrShellPrefix && isRelativePureForGuard(...)`, símbolo que o refactor removeu. O
   **guard de vacuidade do próprio cenário** (`cmp -s`) recusou-se a fingir que provou algo:
   *"sed nao alterou validator_credential_guard.go — prova P4 invalida"*. Retargetado no ML-2A.
2. **`PWD_DEAD: unbound variable` no echo final.** Os 165 cenários passavam e o script morria no
   sumário: string entre aspas duplas contendo `$PWD_DEAD/`, expandida sob `set -u`. E o irmão
   silencioso, que eu mandei procurar: **4 ocorrências de `$PWD`**, que existe no ambiente e expandia
   **sem falhar**, corrompendo o texto do sumário para quem fosse ler o log amanhã.

Nas duas vezes o método dele foi `grep FAIL`, que não vê abort por variável não ligada. **Sexta
entrega seguida em que "0 linhas FAIL" é apresentado como exit code.** Registrado aqui porque é
padrão, não incidente.

#### Trabalho dele que eu não pedi e que ficou melhor que o pedido

- **Pré-flight de JSON dentro do gate** (`_validate_fixture_json`): parseia o fixture e confere que o
  `command` faz round-trip. É a resposta ao risco que quase me enganou hoje, agora no CI em vez da
  disciplina de quem escrever o próximo fixture.
- **Limite de vacuidade declarado, não maquiado:** para classes 1 e 3 o `resolveCredentialGuardHookPath`
  retorna `ok=false` **antes** de tocar o filesystem, então não existe irmão `-absent` que prove que
  o silêncio é real — só a sabotagem de sentido oposto (cenário 165). Está escrito no gate.

---

## Wave 3 — Barreira

> Dependências: Wave 2 completa e auditada.

### ML-3A — Revisão de segurança
**Status:** ✅ Concluído — **REPROVOU** · **Agente:** `hades-tf` (`subagent_type: hades-tf`)
**Escreve:** `docs/seguranca/2026-08-22-revisao-da-classificacao-por-ancoragem.md`

A regra decide se um guard está ativo. Avaliar:
(a) a classe 2 pode ser **contornada** por uma forma que expande do cwd e não foi classificada?
(b) a classe 1 ficou realmente livre de falso-positivo — em especial o caminho absoluto?
(c) a classe 3 é ponto cego **aceito e declarado**, ou virou porta de entrada nova?
**Veredito explícito**, distinguindo o que foi medido do que foi escolhido na ausência de dado.


---

### Auditoria do ML-3A — **REPROVADO**, e o achado é do tipo que a REQ existia para evitar

Parecer: `docs/seguranca/2026-08-22-revisao-da-classificacao-por-ancoragem.md`.

**Confirmei os três achados por medição própria**, com fixture validado:

```
~/.trackfw/scripts/trackfw-credential-guard.sh   -> ACUSADO  "bare relative path"   <- FALSO-POSITIVO
${PWD}/scripts/trackfw-credential-guard.sh       -> SILENCIO                         <- falso-negativo
sh -c "$PWD/scripts/..."                         -> acusado com a mensagem ERRADA
```

**O bloqueante:** `filepath.IsAbs("~/…")` é `false` em Go, então o til caiu na classe 2. Mas `~`
expande para `$HOME` em qualquer shell POSIX — a forma **ancora**. E o caminho que torna isso grave é
o do próprio trackfw: `~/.trackfw/scripts/trackfw-credential-guard.sh` é o **harness global**. Acusar
esse usuário o empurra a rodar `trackfw update`, que troca o comando por `$CLAUDE_PROJECT_DIR/…` —
inútil para hook global. **A regra criada para impedir que o guard fique inerte passaria a deixá-lo
inerte.**

**Correção de rota dele que merece registro:** na primeira passada ele mediu Node e Python com
caminhos errados (`npm/src/index.js` → module not found; `pypi/trackfw/cli.py` sem `PYTHONPATH`),
recebeu silêncio dos dois e **percebeu que o silêncio era do instrumento, não do produto**. Refez com
`node npm/bin/trackfw` e `PYTHONPATH=pypi python3 -m trackfw`. É a terceira vez nesta série que um
instrumento defeituoso quase vira conclusão — e a primeira em que o próprio agente pega.

---

## Wave 4 — Correção pós-barreira

> Dependências: ML-3A. Bloqueia o PR.

### ML-4A — `~/` é classe 1; `${PWD}/` é classe 2; mensagem certa por forma
**Status:** ✅ Concluído · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`)

**Critérios de aceite:**
- [x] `~/…` **não aspeado** → classe 1 (silêncio) nos 3 CLIs
- [x] `"~/…"` **aspeado** → **acusado**: o til não expande entre aspas duplas, então a forma quebra
- [x] `${PWD}/…` → classe 2 (acusado), mesmo predicado de `$PWD/…`
- [x] `sh -c "$PWD/…"` → acusado com a mensagem **do `$PWD`**, não a de relativo puro
- [x] Casos novos no `check-validate-parity.sh`, com guard de vacuidade
- [x] Nenhuma regressão nos casos já aprovados (absoluto, Codex, classe 3, Cursor)
- [x] `make quality` CI-exata **exit 0** (exit code medido) · `validate` 17 warnings

---

### Auditoria do ML-4A e reverificação da barreira — **APROVADO COM RESSALVAS**

**Medição própria, oito formas, fixture com round-trip validado:**

```
~/.trackfw/scripts/...        (sem aspas)  SILENCIO   <- era falso-positivo
"~/.trackfw/scripts/..."      (com aspas)  acusado    <- til nao expande entre aspas
${PWD}/scripts/...                         acusado, mensagem do $PWD
sh -c "$PWD/scripts/..."                   acusado, mensagem do $PWD   <- era a errada
$PWD/scripts/...                           acusado, mensagem do $PWD
/opt/x/scripts/...            (absoluto)   SILENCIO
"$(git rev-parse ...)/..."    (Codex)      SILENCIO
$OUTRA/scripts/...            (classe 3)   SILENCIO
```

O par til-aspeado × til-desaspeado é o núcleo: **a mesma aspa que faz `$PWD` continuar expandindo faz
o `~` parar de expandir.** Duas variáveis, comportamentos opostos sob a mesma sintaxe.

**A barreira reverificou nos 3 runtimes e levantou o bloqueio** — quem reprovou é quem aprova.
`docs/seguranca/2026-08-22-revisao-da-classificacao-por-ancoragem.md`, revisão 3.

**Segunda correção de instrumento dele, e vale mais que o veredito:** na primeira sessão ele usou
`--root`, flag que o `validate` **não aceita** — o que produziu *silêncio universal falso* nos três
CLIs. Reverificou com `cd "$tmpdir" && <cli> validate`, que é o caminho real, já que o validador usa
`os.Getwd()`. É a segunda vez nesta REQ que ele identifica o próprio instrumento como defeituoso
antes de concluir do produto.

**Residuais nomeados, todos não bloqueantes, todos candidatos a REQ futura:**
1. `~usuario/scripts/…` → acusado com mensagem "bare relative", factualmente errada. `~user` expande
   para o home de outro usuário; não é cwd-dependent. Probabilidade real ínfima em hook de agente.
2. `"~/…"` → mensagem imprecisa (*"only resolves from the project root"*; na verdade não resolve de
   lugar nenhum). **O débito é meu:** fui eu que restringi o handoff a duas mensagens. Veredito dele:
   aceitável para a 7.2.0, porque não induz a uma forma silenciosa.
3. `"$CLAUDE_PROJECT_DIR/…"` **com aspas externas** escapa dos checks de existência e
   executabilidade — o `resolve` recebe o valor **cru**, enquanto a classificação recebe o valor sem
   aspas. **Pré-existente**, não introduzido aqui, mas a assimetria ficou mais visível agora.

**Wave 4 fechada. Entrega completa.**

---

## Notas
- **Fora de escopo:** mudar a decisão de qual forma cada CLI usa; criar severidade/regra nova em
  `rules:` (o ADR rejeita, com o motivo).
- Esta entrega **entra na 7.2.0** — a mensagem que induz o erro sai nesta release, e a correção
  precisa sair com ela.
