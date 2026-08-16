---
status: wip
date: 2026-08-16
req: "docs/req/REQ-2026-08-16-higiene-debitos-acumulados-na-entrega-de-remocao-de-plugins-e-release-7-0-0.md"
squad: "apolo-tf, hades-tf, hefesto-tf"
---

# Roadmap: Higiene — sete débitos acumulados da entrega de plugins e da release 7.0.0

> Created: 2026-08-16 | Status: wip

## Context

REQ: `docs/req/REQ-2026-08-16-higiene-debitos-acumulados-na-entrega-de-remocao-de-plugins-e-release-7-0-0.md`

Sete itens, nenhum bloqueante, cada um já com nota de vault ou registro em roadmap fechado.
**Dois deles (1 e 2) mexem num controle de segurança** — o `git-branch-guard` — e por isso exigem
barreira do `hades-tf` antes do merge.

### Mapa apurado (2026-08-16)

| Peça | Onde |
|---|---|
| Guard: gerador canônico | `internal/generators/scaffold.go:1116` (`GenerateGitBranchGuardScript`) + variante global |
| Guard: espelhos | `npm/src/generators/hooks.js`, `pypi/trackfw/generators/hooks.py` |
| Guard: cópia de referência | `scripts/trackfw-git-branch-guard.sh` |
| Guard: validadores | `internal/validator/validator_git_branch_guard.go`, `..._reference.go` |
| `ship` | `internal/commands/ship.go` + `npm/src/ship/runner.js` + `pypi/trackfw/ship/runner.py` |
| Root sem argumento | `internal/commands/root.go:76-100` (Go) e entrypoints Node/Python |
| Paridade do guard | `scripts/check-agent-hooks-parity.sh`, `check-harness-hooks-parity.sh` |

⚠️ **O guard tem 3 cópias sincronizadas** (gerador Go canônico + 2 espelhos) **mais** a cópia de
referência em `scripts/` **mais** validadores de integridade. Mudança no matcher precisa passar por
todas, senão `trackfw validate` acusa divergência.

## Acceptance Criteria
- [ ] AC1 — Itens 1 e 2 (guard) corrigidos, com **cenário de falsificação** conforme P4 do
      `ADR-2026-07-26-principios-de-design-de-gates-verificaveis`.
- [x] AC2 — Itens 3, 5 e 7 (divergências entre CLIs) corrigidos **e cobertos por paridade**, para
      não reaparecerem.
- [x] AC3 — Itens 4 e 6 (documentação) atualizados; ADR corrigido por **emenda**, nunca reescrita.
- [x] AC4 — `make quality` verde; `trackfw validate` sem novas violações.
- [x] AC5 — Qualquer item que não for corrigido é **declarado** como não-será-corrigido, com motivo.

---

## Wave 1 — Correções em árvores disjuntas (3 MLs em paralelo)
> Dependências: nenhuma.
> ⛔ **Nenhum ML desta wave toca `docs/cli-parity.md`** — é do ML-3A, sequencial, para não colidir.

### ML-1A — `git-branch-guard`: falso-positivo por prosa + brecha de contorno (itens 1 e 2)
**Status:** ✅ Concluído (aguardando barreira ML-4A/`hades-tf`) · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`)
**Arquivos:** `internal/generators/scaffold.go` (gerador canônico + variante global),
`npm/src/generators/hooks.js`, `pypi/trackfw/generators/hooks.py`,
`scripts/trackfw-git-branch-guard.sh`, `scripts/check-gates-falsify.sh`, + testes dos 3 stacks

**Ações:**
1. **Item 1 — falso-positivo por prosa.** Linha de mensagem de commit que **começa** com
   `git <sub>` é lida como comando. Correção sugerida: **descartar o conteúdo de `-m`/`--message` e
   de heredocs antes de segmentar**. Ver `vault/notes/git-branch-guard-falso-positivo-em-linha-de-mensagem-de-commit-2026-08-16.md`.
2. **Item 2 — brecha de contorno.** O guard casa `checkout -b` mas não a forma alternativa de criar
   branch. Estender o matcher.
3. **P4 obrigatório:** cenários em `check-gates-falsify.sh` provando que o guard **bloqueia** a forma
   alternativa e **não bloqueia** mensagem de commit com prosa. Braço baseline + braço detecção.

**Critérios de aceite:**
- [x] Commit cuja mensagem contém linha iniciada por `git commit`/`git push` **passa**.
- [x] A forma alternativa de criar branch (`git switch -c/-C/--create`) **é bloqueada**.
- [x] `git commit`/`git push`/`checkout -b` reais **continuam bloqueados** — não-regressão explícita.
- [x] As 3 cópias do script (gerador Go, espelhos Node/Python) e a de referência em `scripts/`
      permanecem **idênticas**; `trackfw validate` não acusa divergência de integridade.
- [x] Cenários de falsificação novos, com baseline e detecção — **Cenários 60 e 61** em
      `scripts/check-gates-falsify.sh`. Renumerados de 58/59 no rebase de 2026-08-16: a `main` já
      ocupava o 58 (vazamento de stack no Node, #181) e o 59 (loopback do `serve`, #182).

**Nota de execução:** foram encontradas mais 2 cópias do template do guard além das listadas nos
"Arquivos" (`pypi/trackfw/validator.py::_GIT_BRANCH_GUARD_SCRIPT_REFERENCE` e
`npm/src/validator/index.js::GIT_BRANCH_GUARD_SCRIPT_REFERENCE`, usadas por
`git_branch_guard_script_integrity`) — atualizadas também, todas as 6 cópias confirmadas
byte-idênticas via teste (`make quality` verde). Detalhe do design em
`vault/notes/git-branch-guard-quote-aware-segmentation-2026-08-16.md`.

### ML-1B — `ship`: mensagem e stream de erro divergentes (item 3)
**Status:** ✅ Concluído · **Agente:** `apolo-tf`
**Arquivos:** `internal/commands/ship.go`, `npm/src/ship/runner.js`, `pypi/trackfw/ship/runner.py`,
`scripts/check-ship-parity.sh`, + testes
**Ações:** unificar a mensagem de violação de `checkShipGovernance` (Go diz `"...wip/ nor done/..."`,
Node/Python dizem só `"...wip/..."`) e o stream/prefixo de erro do passo 1 (`ship.go` nunca seta
`SilenceErrors`). Ver `vault/notes/ship-checkgovernance-error-stream-wording-divergence-2026-08-16.md`.
**Aceite:** saída **byte-idêntica** nos 3 CLIs, mesmo stream e mesmo exit code, coberta por
`check-ship-parity.sh`.

### ML-1C — `trackfw` sem argumento: exit code e stream divergentes (item 7)
**Status:** ✅ Concluído · **Agente:** `apolo-tf`
**Arquivos:** `internal/commands/root.go`, entrypoint Node (`npm/src/commands/index.js`),
entrypoint Python (`pypi/trackfw/cli.py`), script de paridade, + testes
**Estado medido:** Go sai **exit 0** com help em **stdout**; Node sai **exit 1** com help em
**stderr**. Default do commander quando o comando raiz não tem `.action()`.
**Ação:** unificar. **Decisão do arquiteto: adotar o comportamento do Go como canônico** — `trackfw`
sem argumento é uso legítimo (pedir ajuda), não erro, então `exit 0` em `stdout`.
**Aceite:** os 3 CLIs saem `exit 0` com o help em stdout; coberto por paridade.

---

## Wave 2 — i18n e documentação (2 MLs em paralelo)
> Dependências: nenhuma em relação à Wave 1 (árvores disjuntas), mas mantidos aqui para não
> concorrer com os MLs de código pelos mesmos revisores.

### ML-2A — i18n: `errors.notFound` divergente (item 5)
**Status:** ✅ Concluído (2026-08-16) — órfã nos 3, removida; 31 divergências viraram REQ própria · **Agente:** `apolo-tf`
**Arquivos:** `internal/i18n/locales/*.json`, `npm/src/i18n/locales/*.json`,
`pypi/trackfw/i18n/locales/*.json`
**Ação:** a chave existe em Node e Python e **não** no Go. **Primeiro verificar se tem consumidor em
algum dos 3.** Se **órfã nos três** → remover dos três. Se **usada em algum** → adicionar ao Go.
**Reportar qual foi o caso** — a decisão depende da evidência, não de preferência.
**Aceite:** os 3 locales coerentes entre si; nenhuma chave órfã introduzida ou mantida.

### ML-2B — Deriva de documentação em `site/` (item 6)
**Status:** ✅ Concluído (2026-08-16) — 30 seções idênticas pt/en; falta só `trackfw help` · **Agente:** `apolo-tf`
**Arquivos:** `site/guide/commands.md`, `site/en/guide/commands.md`
**Ação:** remover `trackfw plugins` (não existe mais) e acrescentar `changelog` e `commit`, que
faltam. Conferir contra a saída real de `trackfw --help`, **não** contra o `README.md`.
**Aceite:** nenhum comando documentado que não exista; nenhum comando existente ausente.

### ML-2C — Item 8: `agents update` recusa artefato unmanaged sem dizer o remédio
**Status:** ✅ Concluído (2026-08-16) — corpo da mensagem idêntico nos 3; causa raiz encontrada · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`)
**Origem:** bug reportado por KG em uso real no projeto CMDB (2026-08-16). Acrescentado à REQ como
item 8, a pedido dele — REQ própria seria desperdício para um item deste tamanho.

**Arquivos:** `internal/integrations/manager.go` (mensagens em `:311` e `:422`),
`npm/src/integrations/manager.js:189`, equivalente em `pypi/trackfw/integrations/manager.py`, + testes.
**Não** tocar em `docs/cli-parity.md` (é do ML-3A).

**Contexto medido pelo arquiteto no ambiente real:** dois artefatos existiam no disco e **não** no
manifest. `trackfw agents update --force` falhava com `unmanaged artifact ... does not match a
trackfw template`. O comportamento **está correto** — `preflight` recusa bytes desconhecidos no
`update` **ignorando `--force` de propósito**, porque sobrescrever arquivo que o trackfw não escreveu
seria destrutivo. O defeito é de **diagnosticabilidade**: a mensagem não diz o remédio, e o help do
`--force` promete "replace or remove modified managed artifacts", levando o usuário a tentar
exatamente o que já falhou. `trackfw agents install --force` resolve, e isso não aparece em lugar
nenhum.

**Ações:**
1. A mensagem passa a **nomear o remédio**, com o comando pronto para copiar (item, target e escopo
   preenchidos a partir do plano em questão), nos 3 CLIs, byte-idêntica.
2. Revisar o texto do `--help` do `--force` para não prometer o que ele não faz no `update`.
3. **Investigação que faz parte do ML e pode ampliá-lo:** apurar *por que* os artefatos ficaram fora
   do manifest. Já verificado que **não é legado** — `iac`/`tooling` entraram no catálogo em
   2026-07-26 (#72) e o manifest existe desde 2026-07-19 (#50). Se a causa for gravação parcial
   ainda alcançável, **reportar antes de implementar**: aí a correção inclui detecção, não só
   mensagem, e o escopo muda.

**Critérios de aceite:**
- [x] Mensagem nomeia o remédio e é byte-idêntica nos 3 CLIs. Conferido no fonte dos 3
      (`manager.go:641`, `manager.js:326`, `manager.py:305-310` — no Python é concatenação
      multilinha, que um `grep` de uma linha faz parecer truncada).
- [x] Não-regressão: `update` **continua recusando** bytes unmanaged mesmo com `--force`. Coberto
      por teste nos **3** stacks, todos verdes no `make quality`:
      `TestManagerUpdateForceNeverAdoptsUnknownUnmanagedContent` (Go) ·
      `install force replaces unknown unmanaged content while update force never does` (Node) ·
      `test_update_force_never_claims_unknown_unmanaged_file` (Python). Somado ao teste
      end-to-end do Cenário 58, que roda o repro real e afirma `exit != 0`.
- [x] Conclusão da investigação registrada: **é padrão, não caso isolado** — janela de gravação
      parcial em `Manager.mutate()`. Nota de vault criada; correção exige detecção e ficou
      **fora** deste roadmap (vira o `doctor`).
- [x] `make quality` verde.
- [x] Ação 2 (help do `--force`) conferida por execução: `replace a modified managed artifact;
      never adopts unmanaged bytes — use 'install --force' for that` — não promete mais o que não faz.

> **Lacuna registrada, não corrigida aqui:** a identidade byte-a-byte da mensagem entre os 3 foi
> verificada por leitura do fonte, **não** por um gate que compare as três saídas reais. Os testes
> existentes afirmam o comportamento por stack, não a paridade entre stacks. Fica como observação
> para o ML-3A decidir se vira contrato em `cli-parity.md`.


---

> 📌 **Dois achados da Wave 2 que NÃO entram neste roadmap** (registrados para decisão de KG):
> 1. **Causa raiz do bug do CMDB encontrada:** em `Manager.mutate()`, **todos** os bytes do lote são
>    escritos em disco **antes** de qualquer manifest ser persistido. Interrupção entre os dois laços
>    deixa arquivos corretos sem registro — exatamente o sintoma observado (12 arquivos, mesmo
>    timestamp, 10 registrados). O `defer` de rollback cobre erro retornado normalmente, **não**
>    interrupção. Corrigir exige **detecção** (regra de `validate`/doctor) e/ou reordenar a
>    persistência — mudança de comportamento, fora de escopo aqui.
>    📎 `vault/notes/integrations-manifest-write-precedes-persist-janela-de-registro-parcial-2026-08-16.md`
> 2. **Wrapper de entrega de erro diverge no `integrations`**, medido pelo arquiteto: Go usa
>    `Error:`, Python usa `trackfw agents update:`, e o **Node imprime a linha de código-fonte do
>    `throw`** — stack vazando em erro esperado. É a mesma classe do item 3, que foi corrigido
>    apenas para o `ship`.

## Wave 3 — Consolidação (sequencial)

### ML-3A — `docs/cli-parity.md` + emenda ao ADR (itens 4 e consolidação)
**Status:** ✅ Concluído — auditado pelo arquiteto · **Agente:** `apolo-tf` (`cli-parity.md` + site);
**Emenda 1 do ADR escrita pelo arquiteto**

**Evidência da auditoria (verificada por execução própria, não por aceite do relatório):**
```
guard, prosa com separador   './scripts/trackfw-git-branch-guard.sh "trackfw commit -m \"veja: git status; git push...\""' -> exit 0
guard, comando real          'git commit -m "x"; git push'  -> exit 2
guard, git switch -c         'git switch -c nova'           -> exit 2
```
As duas remoções feitas no `cli-parity.md` (item 7 e a limitação residual do guard) descrevem
divergências que de fato **deixaram de existir** — conferido acima, não apenas relatado.

**Emenda 1 ao ADR do `ship`** (emenda, nunca reescrita — o ADR é aceito) registra o que mudou desde
2026-07-26, tudo medido no binário: tipos de branch passaram a incluir `chore|docs`; o gate tem duas
isenções novas (branch `chore/docs` e mudança doc-only); e **o gate aceita roadmap em `wip/` ou
`done/`**, embora o `--help` e a mensagem de erro digam só `wip/` — divergência registrada, não
corrigida (é string de usuário nos 3 CLIs). Também documentados `--no-pr`, o passo 4 bloqueante e o
contorno de `reset --soft` para empurrar trabalho já commitado.
**Ações:**
1. `docs/cli-parity.md`: registrar as divergências **eliminadas** nas Waves 1 e 2 — e **remover** as
   que estavam documentadas como conhecidas e deixaram de existir.
1-bis. Documentar `trackfw help` em `site/guide/commands.md` e `site/en/guide/commands.md` — é o
   único comando do binário ausente dos dois, e não é o `help` genérico do cobra: documenta as
   chaves de configuração do `trackfw.yaml`.
2. **(Arquiteto)** Emenda ao `docs/adr/ADR-2026-07-26-trackfw-ship-agnostico-de-forge.md`, cujo
   passo 1 ainda descreve o vocabulário antigo do `ship`. **Emenda, nunca reescrita** — o ADR é
   aceito.
**Aceite:** `docs/cli-parity.md` não descreve como conhecida nenhuma divergência já corrigida;
ADR emendado com data e motivo.

---

## Wave 5 — Corretivo da barreira (bloqueia o fechamento)

### ML-4B — Fecha o que a barreira nomeou: prefixos, flags do `checkout`, claim e gate de paridade
**Status:** ⬜ Pendente · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`)
**Origem:** veredito BLOQUEAR do ML-4A.

**A linha de corte não é "B1 vs resto" — é custo do conserto:**

| evasão | classe | entra aqui? |
|---|---|---|
| `env git …`, `command git …` | stripping de prefixo | ✅ sim — não exige tokenizador |
| `git checkout -q -b`, `--no-track -b` | matcher só olha o token seguinte | ✅ sim — o de `switch` já varre todos |
| `git${IFS}push`, `{git,push}`, `g""it push` | exige tokenizar como o bash | ❌ não — ver AC5 |

Corrigir só as flags do `checkout` e deixar `env git`/`command git` abertas seria incoerente: são o
mesmo custo e a mesma classe de "o agente emite sem estar tentando evadir".

**Ações:**
1. Matcher de `checkout` varre **todos** os tokens até achar `-b`/`-B`/`--orphan`, como o de `switch`.
2. Ignorar prefixos `env` e `command` antes de decidir se o comando é `git`.
3. **Header do script**: declarar que é **tripwire, não fronteira de segurança** — mesmo enquadramento
   que o `CLAUDE.md` já usa para o checker de markers de terceiro, e coerente com o
   `ADR-2026-08-12-nao-ha-prevencao-contra-agente-induzido…`. Nada de "quote-aware" sem qualificação.
4. **Gate de paridade**: acrescentar `trackfw-git-branch-guard.sh` às duas listas de
   `scripts/check-attention-scripts-parity.sh` (linhas ~133 e ~150), junto dos outros três scripts.
5. **Cenário de falsificação novo (62)** — ou extensão do 60 — cobrindo prefixo e flag do `checkout`.

**🔴 Onde este ML pode falhar em silêncio:**
- São **6 cópias** do script. Toda mudança sai **do gerador**, byte-idêntica nas 6 — nunca editando
  cópia a cópia. O próprio ML-1A achou 2 cópias além das 4 listadas.
- Os Cenários **60/61** usam `corrupt_literal` contra literais de `internal/generators/scaffold.go`.
  Mexer no template ali é **o mesmo modo de falha** que derrubou o Cenário 58 neste rebase: o literal
  deixa de casar e o cenário vira inerte. Depois de tocar o template, rode `make quality` e confirme
  que os dois braços de detecção **ainda reprovam** — exit code verde não basta.

**Critérios de aceite:**
- [ ] `env git commit`, `command git push`, `git checkout -q -b`, `git checkout --no-track -b` → **exit 2**.
- [ ] Não-regressão: prosa com separador → **exit 0**; `git push`, `git commit`, `git switch -c/-C/--create`, `git checkout -b` → **exit 2**.
- [ ] As 6 cópias byte-idênticas; `trackfw validate` sem divergência de integridade.
- [ ] `check-attention-scripts-parity.sh` passa a cobrir o `git-branch-guard`.
- [ ] Cenário de falsificação novo, com baseline **e** detecção; Cenários 60/61 continuam reprovando.
- [ ] `make quality` verde.

---

## Declaração de não-correção (AC5)

Itens tocados por esta REQ que **não** foram corrigidos aqui, cada um com motivo e destino. Nada
nesta lista é omissão silenciosa.

| o quê | por que não aqui | destino |
|---|---|---|
| **31 chaves de i18n divergentes** entre os 3 CLIs | O ML-2A corrigiu a chave órfã e, ao varrer o resto, expôs problema estrutural: a **saída** diverge, não só a contagem de chaves. Corrigir exige mudar strings de usuário nos 3 CLIs — escopo maior que higiene. | `REQ-2026-08-16-conformidade-estrutural-e-comportamental-de-i18n-entre-os-tres-clis.md` |
| **`trackfw help <chave>` diverge no `Impact:`** | Achado pelo `apolo-tf` no ML-3A, confirmado por execução pelo arquiteto. Em `roadmap_dir` os **três** CLIs dizem coisas diferentes sobre o mesmo campo. Mesma classe do item acima. | Registrado na mesma REQ de i18n |
| **Janela de gravação parcial em `Manager.mutate()`** | Causa raiz do bug do CMDB. Os bytes de todo o lote são escritos antes de qualquer manifest ser persistido; interrupção entre os dois laços deixa arquivo sem registro. Corrigir exige **detecção** e/ou reordenar persistência — mudança de comportamento, não de texto. | Vira o comando `doctor` (REQ ainda não criada) |
| **Wrapper de erro divergente no `integrations`** | Go usa `Error:`, Python usa `trackfw agents update:`, Node vazava a linha do `throw`. O vazamento do Node foi resolvido pelo handler global do #181; **a divergência de prefixo permanece**. | Mesma classe do item 3, corrigido só para o `ship` — fica para a REQ de i18n/saída |
| **Mensagem de artefato unmanaged sem gate de paridade** | A byte-identidade entre os 3 está provada por **leitura do fonte**; os testes afirmam comportamento por stack, não paridade entre stacks. Lacuna registrada em `cli-parity.md`. | Recomendado gate no estilo `check-ship-parity.sh` — não criado aqui |
| **`ship` diz `wip/` mas aceita `done/`** | Mensagem de erro e `--help` mais estritos que o código. Corrigir é mudar string de usuário nos 3 CLIs. | Emenda 1 do ADR do `ship` registra; sem REQ ainda |
| **Evasões que exigem tokenização do bash** (`git${IFS}push`, `{git,push}`, `g""it push`) | Reproduzidas pelo arquiteto e **pré-existentes** — o guard da `main` já evadia todas. Fechá-las exige tokenizar como o bash, e o `ADR-2026-08-12-nao-ha-prevencao-contra-agente-induzido-com-escrita-irrestrita…` já decidiu que prevenção contra agente induzido não é alcançável, investindo em **detecção ancorada no `HEAD`**. Uma REQ de "fazer o guard tokenizar" nasceria contra esse ADR. | O que falta decidir é se o guard vira **tripwire declarado** ou merece exceção — isso é **emenda ao ADR-2026-08-12**, não REQ nova. O ML-4B já declara o tripwire no header. |
| **`ship` não tem modo push-only** | O comando acopla commit+push e exige algo staged; empurrar trabalho já commitado exige `reset --soft` como contorno. Funciona, mas é contorno. | Questão aberta na Emenda 1 do ADR |

---

## Wave 4 — Barreira (só para os itens de segurança)
### ML-4A — `hades-tf`: revisão do guard
**Status:** ✅ Executado · **Veredito: 🔴 BLOQUEAR** · **Agente:** `hades-tf`
**Parecer:** `docs/seguranca/2026-08-16-revisao-do-git-branch-guard.md`

**Reproduzido pelo arquiteto, não aceito por relatório.** Sete evasões confirmadas (`exit 0` = evadiu):

```
git${IFS}push · {git,push} · g""it push · env git commit · command git push
git checkout -q -b nova · git checkout --no-track -b nova
```

**Mas nenhuma é regressão do ML-1A.** Medi o guard da `main` (pré-ML-1A) com a mesma bateria: as
**seis** primeiras já evadiam lá. E o ML-1A entregou o que prometeu — na `main`, prosa com separador
é bloqueada indevidamente (`exit 2`) e `git switch -c` passa (`exit 0`); nesta branch, o inverso.
O ML-1A é **estritamente aditivo**.

**Por que o bloqueio procede mesmo assim**, e é acatado: (a) descrever a segmentação nova como
"quote-aware" sem qualificar cria **confiança falsa** num guard trivialmente contornável; (b) não
existe gate de paridade 3-stacks para este script — confirmei que `check-attention-scripts-parity.sh`
cobre `attention-signal`, `attention-cleanup` e `credential-guard`, e **não** o `git-branch-guard`.

**Resolvido pelo ML-4B abaixo.** O ML-1A **não** é revertido.
**Escreve:** `docs/seguranca/2026-08-16-revisao-do-git-branch-guard.md`
**Ações:** o ML-1A mexe num **controle de segurança**. Verificar que a correção do falso-positivo
**não abriu** caminho para evasão real (ex.: esconder um comando dentro de algo que passe por
`-m`), que a brecha de contorno foi de fato fechada, e que as cópias seguem íntegras. **Veredito
explícito; bloquear é saída legítima.**

---

## Notas
- Itens de doc (`site/`, `cli-parity.md`) e de i18n **não** exigem barreira — só os do guard.
- Commits e branch são exclusivos do `trackfw_architect`.
