---
status: done
date: 2026-08-15
req: "docs/req/REQ-2026-08-15-remocao-do-subsistema-de-plugins-do-trackfw.md"
squad: "apolo-tf, hades-tf, hefesto-tf"
---

# Roadmap: Remoção do subsistema de plugins do trackfw

> Created: 2026-08-15 | Status: done

## Context

REQ: `docs/req/REQ-2026-08-15-remocao-do-subsistema-de-plugins-do-trackfw.md`
ADR: `docs/adr/ADR-2026-08-15-remocao-do-subsistema-de-plugins-em-vez-de-gate-de-binario-de-terceiro.md` (D1–D6)
Parecer que sustenta a decisão: `docs/seguranca/2026-08-15-gate-de-plugins-binario.md`

Remover por completo o subsistema de plugins — download, gestão e **execução**. Substitui o
roadmap de *gate*, abandonado em `docs/roadmaps/abandoned/`.

### Inventário exato do que sai (verificado no código, 2026-08-15)

| Item | Go | Node | Python |
|---|---|---|---|
| Pacote de download | `internal/plugins/` (`plugins.go`, `plugins_test.go`) | dentro de `npm/src/commands/plugins.js` | — (não existe) |
| Comando | `internal/commands/plugins.go` | `npm/src/commands/plugins.js` | `pypi/trackfw/commands/plugins.py` |
| Execução | `RunPlugin` (`plugins.go:111`) + fallback `root.go:71-74` | equivalente em `plugins.js` | `_cmd_run` (`plugins.py:91`) |
| Registro do comando | `internal/commands/root.go` | `npm/src/commands/index.js` | `pypi/trackfw/commands/__init__.py` (ou equivalente) |
| Testes | `internal/plugins/plugins_test.go` | **nenhum** | `pypi/tests/test_commands_extras.py` classe `TestPlugins` (:249) |

Referências em doc/gates a atualizar: `README.md:160-162`, `CLAUDE.md`, `docs/cli-parity.md`,
`scripts/check-cli-parity.sh:22` (lista `floor_commands` contém `plugins`).

## Acceptance Criteria
- [x] AC1 — Comandos e código de download/registry removidos nos 3 CLIs.
- [x] AC2 — Execução de plugin removida, incluindo o fallback de argumento desconhecido.
- [x] AC3 — Argumento desconhecido produz **erro de comando desconhecido**, mensagem idêntica nos 3 CLIs.
- [x] AC4 — Zero referências a `~/.trackfw/plugins` e ao `RegistryURL` em código de produto.
- [x] AC5 — `README.md`, `CLAUDE.md`, `docs/cli-parity.md` e `check-cli-parity.sh` atualizados.
- [x] AC6 — `make quality` verde, sem teste órfão.
- [x] AC7 — Breaking change no `CHANGELOG.md` + bump `7.0.0` — **PR próprio, fora deste roadmap**.

---

## Wave 1 — Remoção (paralelizável por stack)
> Dependências: nenhuma. ML-1A, ML-1B e ML-1C tocam árvores **disjuntas** → executam em paralelo.
> ⛔ Nenhum deles toca `scripts/`, `README.md`, `CLAUDE.md` ou `docs/cli-parity.md` — são do ML-2A,
> sequencial, para não colidir.

### ML-1A — Go
**Status:** ✅ Concluído (2026-08-15) — 3 arquivos apagados; fallback do root.go removido; falsificado com binário real no PATH · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`)
**Arquivos:** apagar `internal/plugins/` inteiro e `internal/commands/plugins.go`; editar
`internal/commands/root.go` (remover registro do comando **e** o fallback de `RunE`, linhas 71-74).
**Ações:**
1. Apagar o pacote e o comando.
2. Em `root.go`, `RunE` deixa de chamar `RunPlugin`. Argumento desconhecido deve produzir o erro
   padrão do cobra para comando desconhecido — **avaliar remover `root.Args = cobra.ArbitraryArgs`**,
   que é o que hoje permite argumento livre chegar ao `RunE`.
3. Sem argumento, o comportamento atual (imprimir help) **é preservado**.
**Aceite:**
- [ ] `go build ./...` e `go vet ./...` limpos; nenhuma referência residual a `plugins` em `internal/`.
- [ ] `trackfw` sem argumento → help (inalterado).
- [ ] `trackfw comando-inexistente` → **erro de comando desconhecido**, exit ≠ 0, e **não** tenta executar binário.
- [ ] Teste cobrindo o item acima (é o coração do AC2/AC3).

### ML-1B — Node
**Status:** ✅ Concluído (2026-08-15) — comando apagado; 4 testes novos; 590 testes verdes · **Agente:** `apolo-tf`
**Arquivos:** apagar `npm/src/commands/plugins.js`; editar o registro do comando em
`npm/src/commands/index.js`.
**Aceite:** `cd npm && npm test` verde; argumento desconhecido → erro, nunca execução; zero
referências a `plugins` em `npm/src/`.

### ML-1C — Python
**Status:** ✅ Concluído (2026-08-15) — comando e TestPlugins removidos; 1230 testes verdes · **Agente:** `apolo-tf`
**Arquivos:** apagar `pypi/trackfw/commands/plugins.py`; remover o registro do subparser; **remover
a classe `TestPlugins`** de `pypi/tests/test_commands_extras.py:249` (teste órfão — o código que ele
cobre deixa de existir).
**Aceite:** `python3 -m pytest pypi/tests -q` verde; argumento desconhecido → erro; zero
referências a `plugins`.

---

## Wave 2 — Docs, gates e paridade
> Dependências: Wave 1 completa. **Sequencial** — arquivos compartilhados.

### ML-2A — Docs + `check-cli-parity.sh` + paridade da mensagem de erro
**Status:** ✅ Concluído (2026-08-15) — mensagem canônica implementada nos 3 CLIs (Go via
`internal/commands/root.go`, Node via `npm/src/lib/unknown-command.js`, Python via
`pypi/trackfw/unknown_command.py` + `TrackfwArgumentParser`), `floor_commands` recalibrado,
`docs/cli-parity.md`/`README.md` atualizados, `scripts/check-unknown-command-parity.sh` novo
adicionado a `make parity`, byte-idêntico + exit 1 verificado (com e sem sugestão) + falsificação
com binário real `trackfw-vaildate` no PATH nos 3 CLIs; `make quality` exit 0 · **Agente:** `apolo-tf`
**Arquivos:** `README.md`, `CLAUDE.md`, `docs/cli-parity.md`, `scripts/check-cli-parity.sh`
**Ações:**
1. Remover `plugins` de `floor_commands` (`check-cli-parity.sh:22`).
   > ⚠️ **Precisão apurada na auditoria da Wave 1:** `floor_commands` é usado **apenas como guarda
   > de contagem** (`${#all_go_commands[@]} -lt ${#floor_commands[@]}`, linha 50), **não** como
   > checagem de pertencimento. Por isso `make quality` passou verde mesmo com `plugins` ainda
   > listado e já ausente do help. Remover a entrada mantém a guarda **calibrada** — não é conserto
   > de check quebrado.
2. Remover as três linhas de plugins da tabela de comandos do `README.md` (:160-162).
3. `docs/cli-parity.md`: registrar a remoção e **apagar** qualquer menção a plugins como exceção de
   paridade — a exceção deixa de existir (D4).
4. **Adicionar ao contrato de paridade** a checagem de que argumento desconhecido produz a **mesma
   mensagem de erro** nos 3 CLIs. É o comportamento novo que substitui a execução de plugin, e sem
   isso ele fica sem cobertura.
   > 🔴 **Divergência real medida na Wave 1 — este é o trabalho do ML-2A, não uma formalidade:**
   > ```
   > GO    exit 1  Error: unknown command "x" for "trackfw"
   > NODE  exit 1  error: unknown command 'x'   (+ "(Did you mean validate?)")
   > PY    exit 2  trackfw: error: argument COMMAND: invalid choice: 'x' (choose from ...)
   > ```
   > Divergem em **texto, aspas, exit code (1 vs 2) e sugestão**. Reconciliar exige customizar a
   > saída de erro do cobra, do commander e do argparse.
   > **Preservar a sugestão do tipo "Did you mean"** nos três: o typo que antes executava um binário
   > passar a sugerir o comando certo é ganho de UX, não ruído a remover.
5. **Divergência pré-existente, FORA de escopo, apenas documentar em `docs/cli-parity.md`:**
   `trackfw` **sem argumento** → Go sai `exit 0` com stdout; Node sai `exit 1` com o help em
   **stderr**. É default do commander (comando raiz sem `.action()`), não tem relação com plugins e
   **não deve ser "consertado" aqui**. Abrir REQ própria.
**Aceite:**
- [ ] `make quality` verde.
- [ ] `grep -rn "plugins" README.md CLAUDE.md scripts/` sem ocorrência que descreva o comando removido.
- [ ] Mensagem de comando desconhecido byte-idêntica nos 3 CLIs, coberta por script de paridade.

---

### ML-2B — Corretivo: cenário de falsificação para o gate novo (P4 do ADR-2026-07-26)
**Status:** ✅ Concluído (não commitado — devolvido ao `trackfw_architect` para auditoria/commit) · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`)
**Dependência:** ML-2A ✅ (commitado).
**Origem:** lacuna reportada pelo próprio ML-2A e confirmada pelo arquiteto como **violação de ADR
aceito**, não como refinamento opcional.

`docs/adr/ADR-2026-07-26-principios-de-design-de-gates-verificaveis.md` estabelece **P4**: todo ML
que mexe em gate exige um passo de falsificação, "com montagem e desmontagem do cenário negativo" —
o ônus da prova é de quem altera. O `scripts/check-unknown-command-parity.sh` nasceu **sem** cenário
em `scripts/check-gates-falsify.sh`, ao contrário dos outros 18 gates. Um gate que ninguém provou
ser capaz de falhar é exatamente o padrão "teste que não consegue falhar" que esta entrega caçou em
outros pontos.

**Arquivos afetados:** `scripts/check-gates-falsify.sh`
**Ações:**
1. Adicionar cenário(s) de falsificação provando que `check-unknown-command-parity.sh` **reprova**
   quando a paridade quebra. No mínimo: (a) divergência de **texto** da mensagem em um dos CLIs;
   (b) divergência de **exit code**; (c) **ausência** da linha de sugestão em um dos CLIs.
2. Seguir o padrão de montagem/desmontagem já usado pelos cenários existentes — o cenário negativo
   é temporário e o repo volta ao estado original.

**Critérios de aceite:**
- [x] Cada cenário novo, quando aplicado, faz `check-unknown-command-parity.sh` sair **≠ 0**; ao
      desmontar, volta a sair 0. (Cenários 55/56/57, cada um com braço baseline `assert_succeeds`
      exit 0 + braço de detecção `assert_fails_with` exit ≠ 0.)
- [x] Contagem de cenários em `check-gates-falsify.sh` sobe de 112 para 118 e a mensagem final
      reflete o novo total e descreve os 3 cenários novos.
- [x] `make quality` verde.

**Comando de validação:** `make quality`

---

## Wave 3 — Barreira final
### ML-3A — `hades-tf`: confirmar que a superfície sumiu
**Status:** ✅ Concluído (2026-08-16) — libera; vetor eliminado, não movido · **Agente:** `hades-tf` (`subagent_type: hades-tf`)
**Escreve:** seção apensada a `docs/seguranca/2026-08-15-gate-de-plugins-binario.md`
**Ações:** verificar que não sobrou caminho de download, de `chmod` de terceiro, nem de execução —
incluindo indiretos (`exec.Command`, `subprocess`, `child_process`) que possam invocar `trackfw-*`.
Confirmar que o débito D9 do ADR superseded está fechado. **Veredito explícito.**

### ML-3B — `hefesto-tf`: código órfão
**Status:** ✅ Concluído (2026-08-16) — 0 bloqueante, 2 médios → ML-3C · **Agente:** `hefesto-tf` (`subagent_type: hefesto-tf`)
**Escreve:** `docs/qualidade/2026-08-15-remocao-de-plugins.md`
**Ações:** procurar helpers, imports, constantes, fixtures e docs que ficaram sem uso após a
remoção — o risco típico de deleção é deixar meio caminho.

---

### ML-3C — Corretivo: resíduos da deleção apontados pela barreira
**Status:** ✅ Concluído (2026-08-16) · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`)
**Dependência:** ML-3A ✅ e ML-3B ✅.
**Origem:** achados médios do `hefesto-tf`, dois deles corroborados pelo `hades-tf`.

**Arquivos e correções exatas:**
1. `internal/i18n/locales/en-US.json`, `pt-BR.json`, `es-ES.json` — remover a chave órfã
   `errors.pluginNotFound` (linha ~64). **Verificado pelo arquiteto:** zero consumidores no Go, e
   Node e Python **já removeram** as suas nesta mesma branch. É assimetria entre os 3 CLIs, criada
   pela paralelização da Wave 1.
2. `pypi/trackfw/thirdparty/fetch.py:32` — comentário cita *"deliberately smaller than the plugin
   binary download cap"*, um teto que não existe mais. O equivalente em Go e o docstring do topo do
   próprio arquivo já foram corrigidos; esta linha escapou. **Apontado pelos dois auditores.**
3. `internal/commands/root_test.go:137` — `TestFormatUnknownCommandError_PluginsIsGone` usa
   `HasPrefix` enquanto os dois testes irmãos usam igualdade byte a byte. Alinhar ao mais estrito.

**Critérios de aceite:**
- [x] `grep -rn "pluginNotFound" internal/` → zero.
- [x] Nenhum comentário nos 3 stacks referencia teto/cap/download de plugin.
- [x] `TestFormatUnknownCommandError_PluginsIsGone` compara por igualdade exata.
- [x] `make quality` verde.

**Fora de escopo, vira REQ própria:** `site/guide/commands.md` e `site/en/guide/commands.md`
documentam `trackfw plugins`, mas o `git log` mostra que a deriva é **pré-existente** (arquivo
anterior a esta branch, também desatualizado quanto a `changelog` e `commit`). Não é resíduo desta
deleção e não infla este escopo.


### ML-3D — Corretivo final: `errors.downloadFailed` órfã no Python
**Status:** ✅ Concluído · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`)
**Dependência:** ML-3C ✅.
**Origem:** auditoria do arquiteto sobre o ML-3C — a correção fechou a assimetria do Go e deixou a
irmã de pé no Python.

Estado medido após o ML-3C:

| | bloco `errors` |
|---|---|
| Go | removido (só tinha `pluginNotFound`) ✅ |
| Node | `notFound` |
| Python | `notFound`, **`downloadFailed`** ← resíduo |

`errors.downloadFailed` (`"download failed: HTTP {{status}} for {{url}}"`) tem **zero consumidores**
no Python e é claramente de plugins — *download* só existia lá. O ML-1B removeu a equivalente do
Node; o ML-1C removeu só `pluginNotFound` do Python e deixou esta.

**Ações:** remover `errors.downloadFailed` de `pypi/trackfw/i18n/locales/{en-US,pt-BR,es-ES}.json`.
Validar o JSON depois de editar.

**Critérios de aceite:**
- [x] `grep -rn "downloadFailed" pypi/` → zero.
- [x] JSON dos 3 locales do Python válido.
- [x] `make quality` verde.

**FORA de escopo, documentado:** `errors.notFound` existe em Node e Python e **não** no Go — mas
`git show main:internal/i18n/locales/en-US.json` prova que o bloco `errors` do Go **já continha
apenas `pluginNotFound`** antes desta branch. Ou seja, a divergência de `notFound` é
**pré-existente** e não foi criada por esta entrega. Vira REQ própria junto com a deriva de
`site/guide/commands.md`.


## Notas
- Remoção é **breaking change**: bump `7.0.0` + `CHANGELOG.md` em **PR próprio**, após este.
- Roadmap anterior (gate) em `docs/roadmaps/abandoned/`, com o motivo registrado.
- Commits e branch são exclusivos do `trackfw_architect`.

---

## Evidência de fechamento (auditoria do arquiteto, 2026-08-16)

| AC | Evidência |
|---|---|
| AC1 | `internal/plugins/`, `internal/commands/plugins.go`, `npm/src/commands/plugins.js` e `pypi/trackfw/commands/plugins.py` apagados |
| AC2 | `RunPlugin` e o fallback `root.go:71-74` removidos; `hades-tf` varreu caminhos indiretos (`exec.Command`/`subprocess`/`child_process`) e confirmou que só resta toolchain fixa |
| AC3 | Mensagem canônica byte-idêntica nos 3 CLIs, verificada por `diff` do arquiteto, com e sem linha de sugestão, exit 1 nos três |
| AC4 | Zero ocorrências de `~/.trackfw/plugins` e `RegistryURL` em código de produto |
| AC5 | `README.md`, `CLAUDE.md`, `docs/cli-parity.md` e `check-cli-parity.sh` atualizados |
| AC6 | `make quality` exit 0; testes de plugins removidos junto com o código; nenhum teste órfão |
| AC7 | Breaking change → **PR próprio de release 7.0.0**, fora deste roadmap |

**Prova do vetor, feita com binário real e não por leitura de diff:** um executável
`trackfw-vaildate` colocado no `PATH`, imprimindo `EXECUTOU_PLUGIN_MALICIOSO`, **não é executado**
por nenhum dos 3 CLIs — todos recusam com a mensagem canônica. Verificado pelo arquiteto e,
independentemente, pelo `hades-tf` na barreira.

**Débitos deixados para trás, com REQ a abrir:**
- divergência **pré-existente** de `errors.notFound` entre os locales (existe em Node e Python, não
  no Go — comprovado em `git show main:`);
- deriva de `site/guide/commands.md` e `site/en/guide/commands.md`, que documentam `trackfw plugins`
  e também estão desatualizados quanto a `changelog` e `commit`;
- divergência **pré-existente** de `trackfw` **sem argumento**: Go sai `exit 0` com stdout, Node sai
  `exit 1` com help em stderr.
