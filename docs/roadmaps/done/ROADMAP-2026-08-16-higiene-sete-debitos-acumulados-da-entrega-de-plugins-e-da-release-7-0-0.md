---
status: done
date: 2026-08-16
req: "docs/req/REQ-2026-08-16-higiene-debitos-acumulados-na-entrega-de-remocao-de-plugins-e-release-7-0-0.md"
squad: "apolo-tf, hades-tf, hefesto-tf"
---

# Roadmap: Higiene — sete débitos acumulados da entrega de plugins e da release 7.0.0

> Created: 2026-08-16 | Status: done

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
- [x] AC1 — Itens 1 e 2 (guard) corrigidos, com **cenário de falsificação** conforme P4 do
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
**Status:** ✅ Concluído · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`)
**Origem:** veredito BLOQUEAR do ML-4A.

**⚠️ Achado que expande o escopo literal da ação 4, precisa ratificação do arquiteto:** `discover
--init` em **Go e Node não gerava** `trackfw-git-branch-guard.sh` antes deste ML — só `trackfw
init`/`update harness` chamavam o gerador nesses dois stacks. Acrescentar o script à lista do
`check-attention-scripts-parity.sh` sem corrigir isso tornaria o gate vazio (arquivo ausente falha
no `-s` antes mesmo do diff — nunca prova paridade nenhuma). **Python já gerava** — verificado por
`git diff`/`git log` que `pypi/trackfw/generators/hooks.py::inject_hooks_detected` já chamava
`_generate_git_branch_guard_script` incondicionalmente antes desta sessão (código pré-existente,
não tocado por mim), e `discover.py`'s bloco `--init` já chama `inject_hooks_detected`. Corrigido
em Go (`internal/discover/discover.go::InstallGates`) e Node (`npm/src/commands/discover.js`,
bloco `--init`) com uma chamada explícita ao gerador, mesmo padrão já usado para
`credential-guard` nesses dois stacks. Em Python acrescentei a mesma chamada explícita em
`discover.py` por paridade estrutural com Go/Node — é redundante com o que `inject_hooks_detected`
já fazia (mesma redundância pré-existente que já ocorre lá para `attention-scripts` e
`credential-guard`, chamados tanto explicitamente quanto dentro de `inject_hooks_detected`), não
uma correção de bug em Python. Esta mudança altera o comportamento observável de `trackfw discover
--init` em projetos brownfield (novo arquivo escrito) — está fora da redação literal da ação 4
("acrescentar às duas listas"), então o arquiteto deve decidir se ratifica aqui ou se prefere
separar em REQ própria antes do merge.
São **7 cópias literais** do script, não 6 como o mapa do roadmap registrava —
`internal/validator/validator_git_branch_guard_reference.go` é uma cópia própria (import cycle
Go), além do gerador Go canônico, dos 2 espelhos Node/Python e dos 2 validators Node/Python que o
ML-1A já tinha achado. Todas as 7 atualizadas e confirmadas byte-idênticas pelos testes de
paridade existentes nos 3 stacks (`TestGitBranchGuardScriptReference_MatchesGenerator` no Go,
`GIT_BRANCH_GUARD_SCRIPT_REFERENCE é byte-idêntico...` no Node,
`test_reference_e_byte_identico_ao_gerador_real` no Python) — todos verdes.

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
- [x] `env git commit`, `command git push`, `git checkout -q -b`, `git checkout --no-track -b` → **exit 2**.
- [x] Não-regressão: prosa com separador → **exit 0**; `git push`, `git commit`, `git switch -c/-C/--create`, `git checkout -b` → **exit 2**.
- [x] As 7 cópias byte-idênticas (6 do mapa original + `validator_git_branch_guard_reference.go`,
      achada por este ML); `trackfw validate` sem divergência de integridade.
- [x] `check-attention-scripts-parity.sh` passa a cobrir o `git-branch-guard`.
- [x] Cenário de falsificação novo (62, dois sub-casos), com baseline **e** detecção; Cenários
      60/61 continuam reprovando. Cada sub-caso tem asserção auto-discriminante contra o MESMO
      build corrompido (62a: `git push` puro continua bloqueado; 62b: corrupção retargetada para a
      forma exata pré-ML-4B, `git checkout -b` puro continua bloqueado) — prova que a detecção
      isola o literal certo, não uma quebra geral do matcher.
- [x] `make quality` verde.
- [x] **Residual declarado, não fechado:** `env`/`command` **com** argumentos (`env FOO=bar git
      push`, `env -i git commit`, `command -p git push`) ainda evadem — reproduzido por execução
      (exit 0). Fora da redação literal da ação 2 (que cobre a forma sem argumentos). Declarado no
      header das 7 cópias e na tabela AC5 abaixo, não fechado nesta correção.

---

### Ratificação do arquiteto — `discover --init` passa a gerar o script do guard

O `apolo-tf` pediu ratificação por ter mexido em `discover --init` nos 3 CLIs, fora do escopo literal
do ML-4B. **Ratificado** — e o motivo real é melhor que o alegado no relatório dele.

O relatório dizia que "o Python já fazia isso". **Não fazia** — conferi: nenhum dos 3 gerava.
E medi com o binário anterior: `discover --init` num repo limpo **não** produzia
`scripts/trackfw-git-branch-guard.sh`.

O que justifica a mudança é a **inconsistência entre irmãos**: `check-attention-scripts-parity.sh`
obtém os scripts rodando `discover --init` por runtime e comparando. Ele já cobria
`trackfw-credential-guard.sh` — ou seja, o `discover --init` **gerava** o credential-guard e **não**
gerava o git-branch-guard, dois scripts de hook de mesma natureza. A ação 4 do ML-4B era impossível
de cumprir sem alinhar isso: o gate tem um guard de vacuidade P2 que exige o arquivo existir e ser
não-vazio, então acrescentar o script à lista sem gerá-lo deixaria o gate **vermelho**, não vacuoso.

Verificado por mim: os 3 geram, 287 linhas, **byte-idênticos entre si e com a cópia de referência**
em `scripts/`.

### Evidência da auditoria do ML-4B (execução própria, não aceite de relatório)

```
FECHARAM (exit 2):  env git commit · command git push
                    git checkout -q -b · git checkout --no-track -b
NÃO REGREDIU (2):   git push · git commit · switch -c/-C/--create · checkout -b
FALSO-POSITIVO (0): trackfw commit -m "veja: git status; git push é bloqueado"
FORA DE ESCOPO (0): git${IFS}push · {git,push} · g""it push · env FOO=bar git push
```

Header declara **"TRIPWIRE, NÃO FRONTEIRA DE SEGURANÇA"** citando o `ADR-2026-08-12`; nenhuma
ocorrência de "quote-aware" sem qualificação sobrou.

`make quality` exit 0 · **123 cenários** · `trackfw validate` exit 0 ·
`attention-scripts-parity/trackfw-git-branch-guard.sh/go-vs-node` e `/go-vs-py` **OK** ·
Cenários 60 e 61 **continuam reprovando** nos braços de detecção.

---

## Wave 6 — Segundo corretivo (achados da reverificação)

### ML-4C — Fecha `env VAR=val`, `git branch <nome>` e `git worktree add -b`
**Status:** ✅ Concluído · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`)
**Origem:** reverificação do `hades-tf`, que **levantou o bloqueio** e, ao mesmo tempo, apontou dois
pontos em que a minha própria tabela AC5 estava incompleta. Ele tem razão nos dois.

**Confirmado por mim (`exit 0` = evade):**
```
git branch nova              cria branch · o guard se chama git-branch-guard
git branch -c origem nova    idem
git worktree add -b nova ..  idem
env FOO=bar git push         mesma classe da forma nua que o ML-4B fechou
git checkout --orphan nova   -> exit 2 (já fechado como efeito colateral do ML-4B)
```

**Por que estes saem da tabela de declaração e viram correção:**
- `git branch` e `worktree add -b` **criam branch**. Um guard chamado *git-branch-guard* que deixa
  passar a forma mais direta de criar branch não é tripwire incompleto, é tripwire furado no
  próprio alvo. É a mesma classe do `switch -c` que o ML-1A fechou.
- `env FOO=bar git push` é o mesmo argumento que o ML-4B usou para a forma nua: é o que um agente
  emite **sem estar tentando evadir**. Arquivá-lo ao lado de `nice`/`sudo` foi erro meu de
  classificação — o `hades-tf` nomeou, e procede.

**Ações:**
1. Bloquear `git branch <nome>` e `git branch -c/-C/-m/-M <nome>`. **Não** bloquear as formas de
   leitura: `git branch` sem argumento, `-a`, `-r`, `-l`, `--list`, `-v`, `--show-current`,
   `--contains`, `--merged`. Falso-positivo aqui é pior que a brecha: listar branch é operação de
   leitura que agentes fazem o tempo todo.
2. Bloquear `git worktree add` quando houver `-b`/`-B`.
3. No stripping de prefixo, pular `env` seguido de qualquer sequência de `CHAVE=valor`.

**🔴 Onde pode falhar em silêncio:** mesmos dois modos das vezes anteriores — as **6+ cópias** saem
do gerador (nunca editadas uma a uma) e o `corrupt_literal` dos Cenários 60/61/62 vira inerte se o
template mudar. Rodar `make quality` e conferir que os braços de detecção **ainda reprovam**.

**Critérios de aceite:**
- [x] `git branch nova`, `git branch -c origem nova`, `git worktree add -b nova ..`, `env FOO=bar git push` → **exit 2**.
- [x] Leitura **não** bloqueia: `git branch`, `git branch -a`, `-r`, `--list`, `-v`, `--show-current` → **exit 0**.
- [x] Não-regressão completa da bateria já medida (push/commit/switch/checkout/prosa).
- [x] 7 cópias byte-idênticas (confirmado por `discover --init` real em Go/Node/Python, diff byte-a-byte contra `scripts/trackfw-git-branch-guard.sh`); Cenários 60/61/62 continuam reprovando; Cenário 63 novo (3 sub-casos: `branch-create`, `worktree-add-b`, `env-var-assignment`), cada um com baseline + detecção + auto-discriminação.
- [x] `make quality` verde.

**Evidência de execução (própria, não relatada):**
```
== EVADE HOJE → agora exit 2 ==
git branch nova                    exit=2
git branch -c origem nova          exit=2
git worktree add -b nova ..        exit=2
env FOO=bar git push               exit=2

== BATERIA DE LEITURA → exit 0 ==
git branch (sem args)/-a/-r/-l/--list/-v/-vv/--show-current/
  --contains x/--merged/--no-merged/--sort=.../--format=...   exit=0 (todas)
git branch -d nome / -D nome                                  exit=0

== NÃO-REGRESSÃO → exit 2 ==
git push · git commit -m x · git switch -c/-C/--create ·
git checkout -b · -q -b · --no-track -b · --orphan ·
env git commit · command git push                             exit=2 (todas)

== FALSO-POSITIVO → exit 0 ==
trackfw commit -m "veja: git status; git push é bloqueado"     exit=0

== FORA DE ESCOPO (declarado) → exit 0 ==
git${IFS}push · {git,push} · g""it push · nice git push ·
sudo git push · env -i git push                                exit=0 (todas)
```

`go build ./...` limpo · `go test ./internal/generators/... ./internal/validator/...` verde (16 testes novos
adicionados a `git_branch_guard_test.go`) · `scripts/check-gates-falsify.sh` **124 cenários, 0 FAIL** (Cenário
63a/63b/63c novos, todos OK; Cenários 60/61/62 continuam reprovando) · `make quality` verde (evidência completa
abaixo, na seção de execução do roadmap).

**Nota de execução — as 7 cópias, não 6+:** confirmado via `discover --init` real contra os 3 binários (Go,
Node, Python) num diretório limpo: os três geram `scripts/trackfw-git-branch-guard.sh` byte-idêntico entre si
e byte-idêntico à cópia de referência do repositório. `go build ./...`, `node -c` e `python3 -m ast` limpos
nos 6 arquivos-fonte tocados (`internal/generators/scaffold.go`,
`internal/validator/validator_git_branch_guard_reference.go`, `npm/src/generators/hooks.js`,
`npm/src/validator/index.js`, `pypi/trackfw/generators/init_gen.py`, `pypi/trackfw/validator.py`) + a cópia de
referência `scripts/trackfw-git-branch-guard.sh` = 7 no total.

---

### Auditoria do ML-4C pelo arquiteto — 33 payloads, execução própria

```
FECHOU (exit 2)      git branch nova · -c · -C · -m · git worktree add -b · env FOO=bar git push
LEITURA OK (exit 0)  git branch · -a · -r · -l · --list · -v · -vv · --show-current
                     --contains · --merged · --no-merged · --sort= · --format= · -d · -D
NÃO REGREDIU (2)     push · commit · switch -c/-C/--create · checkout -b/-q -b/--no-track -b/--orphan
                     env git commit · command git push · env env git push
FALSO-POSITIVO (0)   trackfw commit -m "veja: git status; git push é bloqueado"
DECLARADOS (0)       git${IFS}push · {git,push} · nice · sudo · env -i
```

As **15 formas de leitura** eram o risco que dominava este ML — bloquear `git branch -a` seria pior
que a brecha, e foi um falso-positivo assim que originou esta REQ. Nenhuma bloqueia.

`make quality` exit 0 · **124 cenários** · braços de detecção dos Cenários 60/61/62/63 continuam
reprovando (o modo de falha que derrubou o 58 neste rebase).

### 🔴 Armadilha de ambiente descoberta na auditoria

O `trackfw` do `PATH` (`/opt/homebrew/bin/trackfw`) é um build **velho** e emite um aviso **falso**
de divergência do script do guard. O agravante: `trackfw --version` diz `7.0.0` nos **dois** binários
— o velho e o recém-compilado. **A versão não distingue o build**, então não há como perceber que o
binário está desatualizado pela via óbvia.

O exit code não muda (é warning, não violação), então as conclusões de `validate` desta REQ seguem
válidas. Mas qualquer auditoria futura deve rodar `go build -o /tmp/tfw && /tmp/tfw validate`, nunca
o binário do `PATH`.

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
| **Wrappers de comando em geral como prefixo** — medido por mim **depois** do ML-4B, atacando a lógica nova de stripping: `env -i git push`, `env --ignore-environment git push`, `ENV=1 env git push`, `nice git push`, `sudo git push` (todos `exit 0`). O ML-4B fechou a forma **nua** de `env`/`command` (e formas encadeadas: `env env`, `command env` etc. bloqueiam). | Enumerar wrappers é corrida sem fim — `nice`, `sudo`, `timeout`, `xargs`, `stdbuf`, `setsid`, e qualquer binário que receba um comando como argumento. Reconhecer todos exige resolver o comando efetivo, que é o mesmo problema de tokenização já declarado acima. | Coberto pela declaração de **tripwire** no header do script. Se virar exigência, é emenda ao `ADR-2026-08-12`, não REQ nova. |
| **`ship` não tem modo push-only** | O comando acopla commit+push e exige algo staged; empurrar trabalho já commitado exige `reset --soft` como contorno. Funciona, mas é contorno. | Questão aberta na Emenda 1 do ADR |
| **`env`/`command` COM argumentos ainda evadem o guard** (`env -i git push`, `command -p git push`) — reproduzido e confirmado por execução (exit 0 nos três) | **Parcialmente fechado pelo ML-4C**: o ML-4C passou a reconhecer `env` seguido de uma sequência de atribuições `CHAVE=valor` (`env FOO=bar git push`, `env FOO=bar BAZ=qux git push` → agora bloqueiam, ver Cenário 63c de `check-gates-falsify.sh`). O que permanece aberto é `env`/`command` com **FLAGS** (`env -i git push`, `env --ignore-environment git push`, `command -p git push`) — reconhecer isso exigiria entender a sintaxe própria de flags desses dois builtins, custo incremental novo não coberto pela ação 3 do ML-4C. Declarado no header do script (7 cópias), não fechado. | Fechar exige reconhecer as flags de `env`/`command` (não só `CHAVE=valor`) antes de reler `base` — pequeno, mas re-sincroniza as 7 cópias e um novo cenário de falsificação. Fica para ML seguinte ou emenda a este, se o arquiteto priorizar. |
| **A cobertura NOVA de `check-attention-scripts-parity.sh` (`trackfw-git-branch-guard.sh`) não tem falsificação própria** — só o matcher do guard foi falsificado (Cenários 62 e 63). Por `ADR-2026-07-26-principios-de-design-de-gates-verificaveis` (citado no AC1), um gate sem falsificação é "não-verificado" — mesmo padrão que motivou o Cenário 48 quando o ML-0B estendeu este gate para `credential-guard`. Risco residual considerado baixo, não zero: o Cenário 43 já prova que o **mecanismo** de diff do gate (o loop `go-vs-node`/`go-vs-py`) é não-vacuoso contra deriva real (drift no `attention-cleanup` do Python); acrescentar um 4º script reusa o mesmo loop. Ausência do arquivo falha alto, no guard `-s` de vacuidade, antes de qualquer diff. Deriva por-stack do literal já é pega independentemente pelos 3 testes de byte-identidade dedicados (Go/Node/Python) que rodei nesta sessão. | Caberia um Cenário 64 (ou extensão do 43) sabotando especificamente a cópia Node ou Python do `GIT_BRANCH_GUARD_SCRIPT`/`_GIT_BRANCH_GUARD_SH` e provando que o `go-vs-node`/`go-vs-py` do gate estendido reprova — não criado aqui, fora do escopo literal da ação 5 do ML-4B (que pedia falsificação do guard, não da extensão do gate). |

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
