---
status: parecer
date: 2026-08-12
author: "Hades (hades-tf)"
---

# Parecer: âncora de detecção de adulteração do credential-guard — `HEAD` × template do binário

> ML-0A do `docs/roadmaps/wip/ROADMAP-2026-08-12-deteccao-de-adulteracao-do-credential-guard-regra-de-validate.md`.
> Não modifica código. Recomendação para emenda do `ADR-2026-08-12-nao-ha-prevencao-...-deteccao-ancorada-no-git.md`.

## Convenção deste parecer

Cada afirmação é marcada **[Verificado]** (lido diretamente no código/repo, comando reproduzível dado)
ou **[Avaliação]** (julgamento sobre o que foi verificado, não é fato observável).

---

## 0. O que foi verificado antes de responder às 5 perguntas

**[Verificado]** O script de projeto e o script global são concatenações puras de `const` string em
Go (`internal/generators/scaffold.go:1097` e `:1104`), sem interpolação de variável (sem timestamp,
sem caminho absoluto, sem hash) — `GenerateCredentialGuardScript` e `GenerateGlobalCredentialGuardScript`
escrevem `[]byte(credentialGuardScript)` / `[]byte(globalCredentialGuardScript)` direto via
`os.WriteFile`. Ou seja: **para uma dada versão do binário, o conteúdo gerado é 100% determinístico** —
não existe nenhum campo por-projeto que impeça comparação byte-a-byte.

**[Verificado]** Node (`npm/src/generators/hooks.js:403-405`, `CREDENTIAL_GUARD_SCRIPT` /
`GLOBAL_CREDENTIAL_GUARD_SCRIPT`) e Python (`pypi/trackfw/generators/init_gen.py:1061`,
`_CREDENTIAL_GUARD_SH`) replicam a mesma composição de blocos (header + guarda de projeto + núcleo de
detecção + tail), como **três templates mantidos independentemente**, um por stack.

**[Verificado]** Não existe gate que compare os três templates entre si byte-a-byte. Os gates
existentes (`scripts/check-harness-hooks-parity.sh`, `scripts/check-agent-hooks-parity.sh`) verificam
que os *hooks* (`.claude/settings.json` etc.) referenciam `trackfw-credential-guard.sh` e que a
estrutura dos seis formatos de hook é idêntica — **não** verificam que o conteúdo do script gerado por
Go, Node e Python é idêntico entre si. `grep -rn` confirma que só existe verificação de "o hook aponta
para o script", não "o script dos 3 CLIs é o mesmo script".

**Consequência direta:** a âncora de template, tal como proposta, tem uma dependência não declarada —
**ela só é segura se os três templates forem byte-idênticos**. Hoje isso não é garantido por
nenhum gate. Um drift de uma linha entre `credentialGuardScript` (Go) e `_CREDENTIAL_GUARD_SH`
(Python) faz o **mesmo repositório** disparar violação num CLI e ficar silencioso no outro — quebra de
paridade que a própria regra nova estaria criando. **Isto precisa entrar no escopo do ML-1A como
pré-requisito, não como nota de rodapé**, ou a regra nasce com um bug de paridade day-one.

**[Verificado]** Este próprio repositório (`trackfw`) **não tem** `scripts/trackfw-credential-guard.sh`
no working tree nem em nenhum commit histórico (`git log --oneline -- scripts/trackfw-credential-guard.sh`
retorna vazio; `ls scripts/` não lista o arquivo). Isto é evidência direta e concreta do cenário "sem
âncora" descrito na pergunta 4 — não é hipotético.

**[Verificado]** `internal/version/version.go` define `var Version = "6.8.0"`, mas **nenhuma parte do
template do script interpola essa versão** — o conteúdo gerado por versões diferentes do binário não
carrega nenhum marcador que permita distinguir "script de versão antiga, nunca atualizado" de "script
adulterado". O template já mudou pelo menos duas vezes recentemente (ADR-2026-08-06 emendas 6 e 8,
que tocaram `credentialGuardDetectionCore`), então esse não é um risco teórico.

---

## 1. O raciocínio do ADR superseded se sustenta?

**[Avaliação] Cai — mas só parcialmente, e a forma exata de "cair" importa.**

O ADR superseded afirmou: integridade de conteúdo *"exige um valor de referência guardado fora do
arquivo gerado — ou seja, exige exatamente o escopo global."* Isso é **falso para o script** e
**verdadeiro para o `trackfw.yaml`**:

- **Script**: é um artefato **gerado**, com forma canônica reproduzível em memória a partir do
  próprio binário (`credentialGuardScript`/`CREDENTIAL_GUARD_SCRIPT`/`_CREDENTIAL_GUARD_SH`). O
  binário **é** a referência fora do arquivo gerado — não depende de `$HOME`, não depende de escopo
  global, não depende de commit algum. A premissa do ADR superseded está **incorreta** aqui.
- **`credential_guard.mode` em `trackfw.yaml`**: é um valor **autoral** (o usuário escreve `mode:
  block` ou `mode: warn` por decisão própria), sem forma canônica — não existe "o `trackfw.yaml`
  correto" que o binário possa gerar e comparar. Para este alvo, a premissa do ADR superseded
  **se sustenta**: falta mesmo um valor de referência fora do arquivo, e a única referência fora
  do arquivo disponível sem depender de escopo global é o `HEAD` do próprio git.

**Nuance que precisa constar na emenda, com todas as letras:** o ADR **Accepted**
(`ADR-2026-08-12-nao-ha-prevencao-...`), que já supersede o primeiro, **repete o mesmo erro** na sua
Decision item 3: *"[integridade] precisava de um valor de referência guardado fora do arquivo gerado
e, portanto, do próprio escopo global como pré-requisito"* — sem separar script de config. Este ADR
Accepted também precisa de emenda, não só o superseded. A formulação correta é: **a dependência do
escopo global era real para o `mode`, nunca foi real para o script.**

---

## 2. Qual âncora para o script? Qual para o `mode`? Por quê

**[Avaliação] Recomendação: template do binário para o script; `HEAD` do git para `credential_guard.mode`. Sem redundância — não usar `HEAD` também no script.**

**Script → template do binário.**
- Cobertura: cobre deleção **e** sobrescrita, e cobre **sempre que o binário rodar** — não tem gap
  de disponibilidade. Diferente do `HEAD`, não existe cenário "script sem `HEAD` para comparar" além
  de "arquivo não existe", que já é o caso trivial e silencioso (ver §4).
- Rejeitar `HEAD` como âncora adicional para o script: não agrega cobertura (o template já cobre as
  duas vias que atingem o script) e importa o falso positivo de edição legítima do script — que não
  deveria existir, já que o script não é para ser editado à mão (é gerado). Se um usuário editar o
  script manualmente por algum motivo, o comportamento correto é sinalizar divergência do template de
  qualquer forma — reforça, não enfraquece, usar só o template.
- **Pré-requisito não opcional**: os três templates (Go/Node/Python) precisam ser garantidamente
  idênticos, via gate novo de paridade byte-a-byte, ou a regra é inconsistente entre CLIs desde o
  primeiro dia. Este item entra no ML-1A.

**`credential_guard.mode` → `HEAD` do git, semântico e direcional, não byte-diff de `trackfw.yaml` inteiro.**
- Não existe forma canônica de `trackfw.yaml` — é editado o tempo todo por motivos legítimos (regras,
  diretórios, namespacing). Comparar o arquivo inteiro contra `HEAD` produziria ruído em toda edição
  não relacionada ao guard.
- A regra deve extrair especificamente o valor de `credential_guard.mode` em `HEAD` (via `git show
  HEAD:trackfw.yaml` + o mesmo parsing usado em runtime,
  `credentialGuardModeResolution`/equivalentes) e comparar **só esse valor** contra o valor lido do
  disco — e, mais especificamente ainda, disparar apenas quando a mudança é **direcional**: de
  `block` (em `HEAD`) para não-`block` (no disco). Ver §3 e §5 para o porquê da assimetria.

---

## 3. Falso positivo é o risco dominante — taxa esperada e discriminação, por âncora

**Script (template do binário).**
- **[Avaliação] Taxa esperada em uso normal: próxima de zero, sob uma condição.** A única fonte
  legítima de divergência é *drift* de versão: usuário gerou o script com binário vX, depois
  atualizou o binário para vY sem rodar `trackfw update` — o script em disco reflete o template de
  vX, mas o binário que roda o `validate` compara contra o template de vY. Isso **é** esperado em
  operação normal (usuários não atualizam na hora) e **não é** tampering.
- **Discriminação:** hoje **não há como discriminar** — nenhum marcador de versão está embutido no
  script (verificado acima). A consequência prática recomendada: (a) severidade **warn**, nunca
  **block**, para esta sub-regra; (b) mensagem causal-neutra — *"script diverge do template gerado por
  esta versão do trackfw — rode `trackfw update` para regenerar, ou verifique se o arquivo foi
  alterado manualmente"* — nunca a palavra "adulterado"/"tampered"; (c) o remédio prático é o mesmo
  nos dois casos (`trackfw update`), o que torna o warn acionável mesmo sem discriminar a causa.
  Recomenda-se **ao Zeus**, como item separado de roadmap (toca `internal/generators/`, fora do
  escopo deste ML), embutir um comentário de versão no template para que o *drift* passe a ser
  autoidentificável — isso reduziria a taxa de falso positivo a zero de fato, não só na severidade.
- **Outros dois falsos positivos concretos a testar no ML-2A:** (1) normalização de fim de linha —
  um checkout com `core.autocrlf` no Windows produz CRLF onde o template Go/Node/Python usa LF;
  comparar sem normalizar quebra todo usuário Windows. (2) confundir as duas formas canônicas — o
  script de **projeto** (`credentialGuardScript`) e o **global** (`globalCredentialGuardScript`) têm
  conteúdo diferente por design (a guarda de projeto só existe na variante de projeto); comparar um
  script de projeto contra o template global é falso positivo garantido.

**`credential_guard.mode` (HEAD, semântico e direcional).**
- **[Avaliação] Taxa esperada: baixa, com a condicional direcional.** Editar `trackfw.yaml` é comum;
  editar especificamente `credential_guard.mode` é raro, e mudar de `block`→outro valor sem intenção
  de downgrade é mais raro ainda. Comparar só o **valor semântico** (não o texto bruto da linha —
  espaços, comentário, aspas não devem importar, replicando a tolerância que
  `credentialGuardModeResolution` já aplica em runtime via `sed`/`tr`) e só disparar na direção
  `block → não-block` elimina o ruído de reformatação e de mudanças em outras chaves do YAML.
- **Discriminação:** o disparo direcional já discrimina "downgrade de segurança" de "edição
  qualquer" — é o único braço desta regra em que a semântica do valor (não bytes) é o que importa,
  e isso é decisão de desenho, não acidente.
- **[Verificado — evidência local]** Este repositório não tem bloco `credential_guard:` em
  `trackfw.yaml`, e o fallback de projeto é `warn` (`credentialGuardProjectTail`,
  `DEFAULT_MODE="warn"`). Ou seja: no escopo de projeto, é comum não haver nada a comparar — reforça
  que "ausente em `HEAD` → silêncio" (não "ausente → violação") é o comportamento correto, e que a
  regra não vai disparar ruído neste tipo de repositório, o que é evidência direta a favor do
  critério de aceite "não dispara neste repositório".

---

## 4. O que fazer quando não há âncora

**[Avaliação] Silêncio — a convenção deste projeto vale aqui, e já tem precedente direto no código.**

A regra irmã, `credential_guard_hook_resolvable`
(`internal/validator/validator_credential_guard.go`), já estabelece o padrão a replicar,
textualmente no próprio comentário do código: `resolveCredentialGuardHookPath` retorna `ok=false`
para formas de caminho não reconhecidas e "o chamador **NÃO** deve tratar isso como violação"; hook
ausente e JSON inválido também são pulados em silêncio (`continue`). A regra nova deve seguir a mesma
forma.

Casos concretos de ausência de âncora a enumerar e testar explicitamente no ML-1A/ML-2A:

- **Script**: arquivo ausente no disco **e** nenhum hook o referencia → silêncio (guard global
  instalado é estado legítimo, já coberto pela regra irmã). Arquivo ausente **mas** referenciado por
  um hook → já é a violação existente de `credential_guard_hook_resolvable`, não desta regra nova.
- **`mode`**: repositório sem nenhum commit (`HEAD` inexistente) → silêncio. `credential_guard`
  ausente em `HEAD` → silêncio (não existe "downgrade" de um valor que nunca existiu). `trackfw.yaml`
  não versionado (`.gitignore`) → silêncio, pela mesma razão que não há árvore de referência.
- **Template**: se o binário rodando não reconhece a forma do script no disco o suficiente para
  decidir se é "projeto" ou "global" (por exemplo, um terceiro formato futuro) → silêncio, não
  adivinhar.

O efeito colateral positivo, já observado neste próprio repositório: com este critério, "não dispara
neste repositório" (critério de aceite do roadmap) é satisfeito **estruturalmente** pela ausência real
de âncora aqui, não por um caso especial hard-coded para este repo.

---

## 5. Um adversário que também adultera a âncora derrota a detecção?

**[Avaliação] Sim para as duas âncoras, mas o custo de ataque é qualitativamente diferente — e essa
diferença é o argumento mais forte para a divisão por alvo, mais forte que "alvo gerado × alvo
autoral".**

- **`HEAD` (para `mode`)**: um adversário que **commita** a mudança de `mode: block` para `mode: warn`
  derrota a detecção — mas paga um custo observável e permanente: o commit fica em `git log`, aparece
  em diff de PR, em code review. É o mesmo argumento central do ADR Accepted (§2 da Decision: "o
  escopo global é PIOR em visibilidade... um artefato dentro do repositório é auditável por
  construção"). Um agente induzido, restrito ao workspace, **consegue** commitar — mas commitar não é
  "sem custo": é a ação mais diffável das três vias, exatamente como o ADR Accepted já registra no
  item 5 da Decision.
- **Template (para o script)**: derrotar exige **tampering do binário instalado**
  (`/usr/local/bin/trackfw`, via `make install`, ou o equivalente `npm`/`pip` global) — fora do
  repositório, fora do workspace. É a **mesma classe de alcance** que a medição
  (`docs/pesquisa/2026-08-12-alcance-do-agente-ao-home.md`) já mediu para `$HOME`: Codex **não**
  alcança (sandbox bloqueia); Cursor e Claude alcançam tecnicamente, mas sem hesitação de alinhamento
  o suficiente para não contar como proteção garantida.

**Conclusão que muda a escolha:** para o script, um adversário **estritamente restrito ao
workspace** (sem alcançar o binário instalado) **não consegue** derrotar a âncora de template — não é
"mais caro", é **fora do alcance dado o mesmo modelo de ameaça já medido no ADR Accepted**. Essa é a
justificativa mais forte para não usar `HEAD` também no script: `HEAD` teria o mesmo limite de "custo
de um commit" que o `mode` tem, enquanto o template não tem esse limite — usá-lo só no script não é
perda de cobertura, é aproveitar a âncora mais forte disponível para o alvo em que ela existe.

---

## Recomendação acionável, por alvo

| Alvo | Âncora | Severidade | Condição de disparo | Pré-requisito não coberto por este parecer |
|---|---|---|---|---|
| **Script** (`scripts/trackfw-credential-guard.sh`) | Template compilado no binário (Go/Node/Python) | `warn` (não discrimina drift de versão vs. tampering) | script existe em disco **e** diverge byte-a-byte do template gerado pela versão corrente, após normalizar fim de linha, comparado contra a forma canônica correta (projeto × global) | Gate novo de paridade byte-a-byte entre os 3 templates (Go/Node/Python) — sem isso a regra nova nasce inconsistente entre CLIs |
| **`credential_guard.mode`** (`trackfw.yaml`) | `HEAD` do git | a decidir pelo Apolo/ML-1A, mas coerente com o fato de que a via sobrevivente é a mais diffável — sugestão: `warn` também, dado que é sinal e não bloqueio | valor em `HEAD` é `block` **e** valor no disco é diferente de `block` (comparação semântica, não textual) | — |
| **Sem âncora disponível** (script ausente + sem hook referenciando; `HEAD` inexistente; `credential_guard` ausente em `HEAD`; `trackfw.yaml` não versionado) | — | silêncio | nunca disparar | precedente já existe em `credential_guard_hook_resolvable` |

---

## Trade-offs que nem Zeus nem eu tínhamos mapeado antes deste parecer

1. **Gap de paridade entre os 3 templates** — a âncora de template só é segura se os três CLIs
   gerarem exatamente o mesmo script, e isso hoje não é gate nenhum. Precisa entrar no ML-1A como
   entregável, não como suposição.
2. **Falta de marcador de versão no template** — sem ele, a regra nunca discrimina *drift* de versão
   de tampering real; a mitigação viável agora é severidade `warn` + mensagem causal-neutra; a
   mitigação estrutural (embutir versão/hash no template) é trabalho separado, de escopo de
   `internal/generators/`, a propor como item de roadmap futuro, não deste ML.
3. **Duas formas canônicas do script** (projeto × global) — comparar contra a forma errada é falso
   positivo garantido; a regra precisa decidir qual forma esperar **por caminho de arquivo**, não por
   heurística de conteúdo.
4. **Normalização de fim de linha** — checkout Windows com `core.autocrlf` quebraria qualquer
   comparação byte-a-byte não normalizada.

## Emenda que o ADR precisa carregar

O ADR Accepted (`ADR-2026-08-12-nao-ha-prevencao-...`) precisa de emenda, não só o superseded: sua
Decision item 3 generaliza "integridade de conteúdo exige escopo global" para os dois alvos, quando a
dependência do escopo global só era real para `credential_guard.mode`. Para o script, o binário já é
a referência — sempre foi, e essa emenda é o que torna a divisão de âncora por alvo (§2) a decisão
correta, e não apenas uma hipótese razoável.
