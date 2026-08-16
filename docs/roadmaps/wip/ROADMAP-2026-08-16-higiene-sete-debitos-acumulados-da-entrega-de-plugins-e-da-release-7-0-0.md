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
- [ ] AC2 — Itens 3, 5 e 7 (divergências entre CLIs) corrigidos **e cobertos por paridade**, para
      não reaparecerem.
- [ ] AC3 — Itens 4 e 6 (documentação) atualizados; ADR corrigido por **emenda**, nunca reescrita.
- [ ] AC4 — `make quality` verde; `trackfw validate` sem novas violações.
- [ ] AC5 — Qualquer item que não for corrigido é **declarado** como não-será-corrigido, com motivo.

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
- [x] Cenários de falsificação novos, com baseline e detecção (Cenários 58/59 em
      `scripts/check-gates-falsify.sh`).

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
**Status:** ⬜ Pendente · **Agente:** `apolo-tf`
**Arquivos:** `internal/i18n/locales/*.json`, `npm/src/i18n/locales/*.json`,
`pypi/trackfw/i18n/locales/*.json`
**Ação:** a chave existe em Node e Python e **não** no Go. **Primeiro verificar se tem consumidor em
algum dos 3.** Se **órfã nos três** → remover dos três. Se **usada em algum** → adicionar ao Go.
**Reportar qual foi o caso** — a decisão depende da evidência, não de preferência.
**Aceite:** os 3 locales coerentes entre si; nenhuma chave órfã introduzida ou mantida.

### ML-2B — Deriva de documentação em `site/` (item 6)
**Status:** ⬜ Pendente · **Agente:** `apolo-tf`
**Arquivos:** `site/guide/commands.md`, `site/en/guide/commands.md`
**Ação:** remover `trackfw plugins` (não existe mais) e acrescentar `changelog` e `commit`, que
faltam. Conferir contra a saída real de `trackfw --help`, **não** contra o `README.md`.
**Aceite:** nenhum comando documentado que não exista; nenhum comando existente ausente.

### ML-2C — Item 8: `agents update` recusa artefato unmanaged sem dizer o remédio
**Status:** ⬜ Pendente · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`)
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
- [ ] Mensagem nomeia o remédio e é byte-idêntica nos 3 CLIs.
- [ ] Não-regressão: `update` **continua recusando** bytes unmanaged mesmo com `--force` — a
      correção é de texto, **não** de comportamento.
- [ ] Conclusão da investigação registrada: caso isolado ou padrão que exige detecção.
- [ ] `make quality` verde.


---

## Wave 3 — Consolidação (sequencial)

### ML-3A — `docs/cli-parity.md` + emenda ao ADR (itens 4 e consolidação)
**Status:** ⬜ Pendente · **Agente:** `apolo-tf` para `cli-parity.md`; **emenda do ADR é do arquiteto**
**Ações:**
1. `docs/cli-parity.md`: registrar as divergências **eliminadas** nas Waves 1 e 2 — e **remover** as
   que estavam documentadas como conhecidas e deixaram de existir.
2. **(Arquiteto)** Emenda ao `docs/adr/ADR-2026-07-26-trackfw-ship-agnostico-de-forge.md`, cujo
   passo 1 ainda descreve o vocabulário antigo do `ship`. **Emenda, nunca reescrita** — o ADR é
   aceito.
**Aceite:** `docs/cli-parity.md` não descreve como conhecida nenhuma divergência já corrigida;
ADR emendado com data e motivo.

---

## Wave 4 — Barreira (só para os itens de segurança)
### ML-4A — `hades-tf`: revisão do guard
**Status:** ⬜ Pendente · **Agente:** `hades-tf` (`subagent_type: hades-tf`)
**Escreve:** `docs/seguranca/2026-08-16-revisao-do-git-branch-guard.md`
**Ações:** o ML-1A mexe num **controle de segurança**. Verificar que a correção do falso-positivo
**não abriu** caminho para evasão real (ex.: esconder um comando dentro de algo que passe por
`-m`), que a brecha de contorno foi de fato fechada, e que as cópias seguem íntegras. **Veredito
explícito; bloquear é saída legítima.**

---

## Notas
- Itens de doc (`site/`, `cli-parity.md`) e de i18n **não** exigem barreira — só os do guard.
- Commits e branch são exclusivos do `trackfw_architect`.
