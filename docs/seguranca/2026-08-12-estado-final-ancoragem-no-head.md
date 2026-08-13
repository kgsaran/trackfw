---
status: final
date: 2026-08-12
author: "Hades (Segurança)"
---

# Estado final — ancoragem no HEAD para as regras de credential-guard (ML-3B)

> Revisão do resultado implementado do
> `ROADMAP-2026-08-12-ancorar-rules-no-head-para-as-regras-de-credential-guard.md`
> (ML-1A/Apolo, ML-2A/Ártemis). ADR:
> `docs/adr/ADR-2026-08-12-severidade-das-regras-de-credential-guard-resolvida-pela-mais-estrita-entre-head-e-disco.md`.

## Resumo do veredito

**Mecanismo M4 (`rules:`) e carve-out do baseline: implementados corretamente e conferidos
no código, não só no ADR.** Mas **existe brecha nova, real e empiricamente confirmada**:
`GIT_DIR`/`GIT_WORK_TREE` no ambiente do processo neutralizam por completo a âncora de `HEAD`
em que M4 se apoia, nos 3 CLIs. **Isso bloqueia o PR** até uma decisão explícita — ver
"Conclusão" no fim.

---

## Pergunta 1 — O mecanismo de fato impede o auto-silenciamento?

**Verificado no código.** `internal/validator/validator.go:122-127`:

```go
func ruleSeverity(name string) string {
	if credentialGuardAnchoredRules[name] {
		return credentialGuardRuleSeverity(name)
	}
	return diskRuleSeverity(name)
}
```

`credentialGuardAnchoredRules` (`validator_credential_guard_integrity.go:197-201`) lista
exatamente as três regras: `credential_guard_hook_resolvable`, `credential_guard_script_integrity`,
`credential_guard_mode_downgrade`. `credentialGuardRuleSeverity` (linhas 252-270) resolve a
severidade como a mais estrita entre `diskRuleSeverity(name)` e a severidade lida de
`headTrackfwYAML()` — reaproveitando `config.ParseRulesFromContent`. Confirmei que **os dois
únicos pontos de aplicação de severidade** (`applyRule`, `applyRuleTagged`, ambos em
`validator.go:146-178`) chamam `ruleSeverity()` — não há terceiro caminho de aplicação. As outras
~38 regras passam por `diskRuleSeverity`, que é **byte-idêntico** ao corpo que `ruleSeverity` tinha
antes do ADR (comentário confere isso explicitamente e o Cenário 53 prova ausência de vazamento).

Empiricamente, reproduzi o cenário decisivo (equivalente ao Cenário 51) num fixture isolado:
`trackfw.yaml` commitado no `HEAD` com `credential_guard.mode: block`, edição **não commitada** em
disco combinando `mode: warn` + `rules: credential_guard_mode_downgrade: off`. **A violação
continua sendo reportada** — confirma que M4 fecha o canal `rules:` como projetado.

**Veredito: sim, cobre todos os caminhos de resolução de severidade que existem hoje** — desde
que `HEAD` seja resolvido corretamente (ver Pergunta 3).

## Pergunta 2 — O carve-out do baseline está completo?

**Verificado no código.** `filterBaselineTagged` (`validator.go:512-548`) é o **único** ponto
de filtragem de baseline: usado tanto por `Validate()` (linha 483) quanto por `ValidateTagged()`
(linha 747) — que por sua vez são os únicos dois entry points de validação consumidos por
`internal/commands/validate.go` (texto e `--json`) e `internal/commands/barrier.go:525`. Não há
segunda cópia de filtro de baseline no Go.

O carve-out em si: `tolerated && !credentialGuardAnchoredRules[v.Rule]` (linha 528) — se a regra
é uma das 3 âncoras, a mensagem **nunca** é descartada, independente do conteúdo do baseline. A
condição está correta (não é `||` nem invertida).

**Paridade confirmada nos 3 CLIs**: `npm/src/validator/index.js:1801,1803` usa a mesma lógica
(`!baselineSet.has(v) || CREDENTIAL_GUARD_ANCHORED_RULES.has(...)`, equivalente por De Morgan);
`pypi/trackfw/validator.py:2019,2026` idem.

**Veredito: sim, completo — confirmo a afirmação do ML-1A.** Não encontrei segundo caminho de
supressão de baseline.

## Pergunta 3 — 🔴 Brecha nova?

**Sim — confirmada empiricamente, não é hipotética.**

`headTrackfwYAML()` (`validator_credential_guard_integrity.go:119-134`) e `isGitWorktree()`
(`validator.go:2114-2121`) chamam `exec.Command("git", ...)` **sem definir `cmd.Env`** — em Go,
isso herda `os.Environ()` do processo pai integralmente, inclusive `GIT_DIR` e `GIT_WORK_TREE`.
Essas duas variáveis têm precedência sobre a descoberta normal de repositório por `cwd`: git as
usa para decidir de qual repositório `rev-parse`/`show` leem, **independente** de `cmd.Dir`.

Provei isto com dois PoCs isolados (scripts descartáveis, nenhum código do trackfw alterado):

**PoC 1 — `credential_guard_mode_downgrade`:** repositório real com `mode: block` commitado no
`HEAD`, e a edição de disco não commitada (`mode: warn` + `rules: ...: off`) que o ADR existe
para pegar. Rodando `trackfw validate` normalmente, a violação **aparece** (confirma Pergunta 1).
Rodando com `GIT_DIR=<repo-alheio>/.git GIT_WORK_TREE=<repo-alheio>` apontando para **qualquer**
outro repositório git que não tenha `trackfw.yaml` versionado — **a violação desaparece por
completo**, `validate` retorna `✓ No violations found.`, exit 0.

**PoC 2 — `credential_guard_script_integrity`:** regra que, antes do M4, **nunca** dependia de
git (detecção 100% em disco, comparando o script contra o template embutido no binário). Fixture:
script adulterado em disco (dispara a regra, default `warning`), `rules:
credential_guard_script_integrity: off` em disco **sem commit**. Rodando normalmente, `HEAD` (sem
`rules:` para esta regra) resolve ao default `warning`, que vence a comparação "mais estrita"
contra o `off` do disco — **o warning aparece**, exatamente como M4 promete. Com
`GIT_DIR`/`GIT_WORK_TREE` redirecionados para um repositório alheio sem `trackfw.yaml`
versionado, `headTrackfwYAML()` retorna `ok=false`, `credentialGuardRuleSeverity` cai direto em
`diskRuleSeverity` sozinho (`off`) — **o warning desaparece por completo**, `✓ No violations
found.`.

**Isto é uma brecha nova, não pré-existente para 2 das 3 regras.** Antes do M4,
`credential_guard_script_integrity` e `credential_guard_hook_resolvable` nunca invocavam `git` —
sua severidade vinha só do disco (`diskRuleSeverity`), que não tem superfície de ataque via
variável de ambiente. M4 introduziu a dependência de `git` na resolução de severidade **destas
duas regras especificamente**, e essa dependência herda a mesma fragilidade que
`GIT_DIR`/`GIT_WORK_TREE` já morde nesta linha de trabalho (documentada em
`docs/cli-parity.md:2579-2588` para o script do hook, e em
`vault/notes/hooks-de-agente-falham-abertos-quando-o-script-nao-resolve-2026-08-12.md` para a
classe geral). Para `credential_guard_mode_downgrade`, a dependência de `headTrackfwYAML()` **já
existia** antes deste roadmap (a própria função de detecção da regra usa o mesmo helper) — M4 não
introduz a exposição aqui, mas também não a fecha; a reafirma dentro da resolução de severidade,
redundantemente.

A mesma fragilidade existe nos 3 CLIs: `npm/src/validator/index.js:1528-1533`
(`execSync('git show HEAD:./trackfw.yaml', {cwd: root, ...})`, sem `env` explícito → herda
`process.env`) e `pypi/trackfw/validator.py` (`_head_trackfw_yaml`, mesmo padrão via
`subprocess`). Não testei os PoCs no Node/Python diretamente, mas o padrão de invocação é
idêntico ao Go e a herança de ambiente é o comportamento padrão de `execSync`/`subprocess.run`
sem `env=` explícito — risco de paridade, não hipótese isolada do Go.

### **Se o `git` falha, o que acontece — falha aberta ou fechada?**

**Falha aberta (fail-open), por desenho explícito do ADR.** As 3 condições de "sem âncora"
(`headTrackfwYAML`, linhas 116-134: não é worktree git; sem commits; `trackfw.yaml` não rastreado
no `HEAD`) fazem `credentialGuardRuleSeverity` cair em `diskRuleSeverity` sozinho — o disco vence,
sem nenhum piso mínimo de severidade. O ADR (Decision point 4) aceita isto conscientemente para
o caso legítimo (projeto novo, sem `HEAD`, ou `trackfw.yaml` não versionado) — **mas a premissa
que justifica aceitar esse limite está errada**: o comentário do código
(`validator_credential_guard_integrity.go:248-251`) e o ADR afirmam textualmente que "none of
those 3 conditions can be reached by an uncommitted edit to trackfw.yaml alone" — **verdade
literal** (não é uma edição de `trackfw.yaml`), mas **falsa na prática**: as 3 condições SÃO
alcançáveis sem nenhum commit e sem tocar em `trackfw.yaml`, só definindo duas variáveis de
ambiente para a chamada de `trackfw validate`. O "sem âncora" deixou de ser um estado apenas
determinado pela topologia do repositório — passou a ser **um estado que o adversário aciona sob
demanda**, no mesmo processo, na mesma invocação que ele já está executando de qualquer forma.

Isso é estritamente pior que o "buraco sem `HEAD`" que o ADR aceitou: aquele exige um repositório
genuinamente novo (condição de fato, fora do controle do adversário no momento do ataque); este
exige só `export GIT_DIR=... GIT_WORK_TREE=...` antes de chamar `trackfw validate` — ao alcance
de qualquer agente/processo com a mesma capacidade de shell que já é pré-requisito para editar
`trackfw.yaml` em disco sem commitar (o próprio ataque que o roadmap inteiro existe para fechar).

## Pergunta 4 — `governance_mode: lenient` anula o ganho?

**Não anula — degrada, mas não apaga.** Rastreei o fluxo em `Validate()` (`validator.go:477-498`)
e `ValidateTagged()` (linhas 740-759): a conversão lenient (`if IsLenient() { warnings =
append(warnings, violations...); violations = nil }`) acontece **depois** de
`filterBaselineTagged` — ou seja, depois que M4 e o carve-out já decidiram que a mensagem
**existe**. Em modo lenient, a violação vira warning e o exit code some (confirmado lendo
`internal/commands/validate.go:50-54`: laço de warnings imprime `⚠`, depois `return nil`). A
mesma reclassificação vale para `--json` (`ValidateTagged` já devolve `taggedV` vazio quando
lenient está ativo — `validate.go:36` não aciona o `return fmt.Errorf(...)`).

Ou seja: **o texto da violação continua aparecendo** (visível em `trackfw validate`,
`trackfw status`, e no JSON como warning) — o que M4 garante (não suprimível por `rules:`/
baseline) permanece verdadeiro. O que `lenient` remove é **o poder de bloqueio** (exit code,
gate de CI). Isso não é "M4 anulado", é "M4 opera uma camada abaixo de onde `lenient` atua" — a
visibilidade sobrevive, o enforcement automatizado não.

**Isso muda a prioridade da REQ pendente sobre `lenient`?** Sim, na minha avaliação — não porque
`lenient` esconda algo (não esconde), mas porque, com `rules:` e baseline fechados, `lenient` fica
sendo **o único canal restante que evita bloqueio de CI** para as 3 regras de credential-guard, e
ele é o mais barato de acionar (uma linha em `trackfw.yaml`, sem sequer precisar nomear a regra).
Combinado com a brecha do `GIT_DIR`/`GIT_WORK_TREE` (Pergunta 3), que sim é fail-open e silenciosa,
`lenient` deixa de ser a prioridade mais urgente — mas continua sendo a segunda, e o fato de os
dois canais mais fáceis (`rules:`, baseline) estarem fechados agora torna `lenient` proporcionalmente
mais atraente para quem quiser reduzir o *blast* de CI sem deixar rastro tão específico.

## Pergunta 5 — A consequência de migração é aceitável?

A Emenda 1 do ADR já assume o risco e propõe a saída (`rules: off` commitado). Na minha leitura,
**a saída é suficiente tecnicamente, mas o atrito é real e assimétrico**: um projeto que hoje
tolera `credential_guard_hook_resolvable` via `.trackfw-baseline.json` (arquivo local,
`.gitignore`d, editável sem revisão) passa a precisar de uma mudança **commitada e revisável** em
`trackfw.yaml` para obter o mesmo efeito. Isso é **exatamente o objetivo do carve-out** — trocar
tolerância silenciosa por tolerância auditável — não um efeito colateral indesejado. O risco real
não é "a saída não existe", é **descoberta**: quem depende do baseline hoje só vai encontrar essa
exigência quando `trackfw validate` quebrar depois de um `trackfw update`, sem aviso prévio no
momento da mudança de comportamento em si (a atualização do binário não avisa "isto vai parar de
ser tolerável"). O ADR já reconhece isto e pede que fique no `README.md`, não só no
`cli-parity.md` — **isso é necessário, mas note que não é suficiente por si só**: documentação
não é notificação; quem não lê o changelog de uma atualização de `trackfw` só descobre no
`validate` vermelho. Não vejo isso como bloqueante — é o preço correto de fechar um canal de
auto-silenciamento — mas deve estar destacado no `CHANGELOG.md`/notas de release da versão que
introduzir esta mudança, não só no README estático.

---

## Conclusão — pode seguir para PR?

**Não sem antes decidir explicitamente sobre a Pergunta 3.** M4 e o carve-out do baseline
**funcionam exatamente como o ADR descreve** dentro do modelo de ameaça que o ADR considerou
(adversário edita `trackfw.yaml`/`.trackfw-baseline.json` em disco, sem tocar em `git`). Mas o
modelo de ameaça desta linha de trabalho **já inclui manipulação de variáveis de ambiente**
(`GIT_DIR`/`GIT_WORK_TREE` documentado desde `docs/seguranca/2026-08-11-revisao-hooks-cwd.md` e
`docs/cli-parity.md`), e M4 introduz uma dependência nova de `git` cuja falha (inclusive falha
**induzida** via essas mesmas variáveis) é fail-open e silenciosa — sem log, sem diferença visível
no output além da ausência da mensagem. Isso derrota o mecanismo com **menos esforço** do que o
ataque original que o roadmap fecha (nenhum commit, nenhuma edição de `trackfw.yaml` sequer
necessária para `credential_guard_script_integrity`/`credential_guard_hook_resolvable` — só duas
variáveis de ambiente).

**Recomendação, ordenada por severidade:**

1. **Bloqueante para PR:** decidir conscientemente sobre `GIT_DIR`/`GIT_WORK_TREE` antes de
   fechar este roadmap — não porque a correção seja obrigatória agora (pode ser aceita como limite,
   como o ADR já faz para "sem HEAD"), mas porque a **premissa atual está factualmente incorreta**
   ("não alcançável por edição não commitada" — é alcançável, sem editar nada, provado acima) e
   isso precisa ser escrito corretamente antes de virar afirmação permanente em ADR/README. Se a
   decisão for "aceitar como limite conhecido", isso é legítimo — mas precisa ser **decisão
   consciente e documentada com a redação correta**, não herdada por engano de uma premissa que já
   nasceu errada.
2. **Recomendo, não bloqueante:** `headTrackfwYAML()`/`isGitWorktree()` (Go) e os equivalentes
   Node/Python poderiam limpar `GIT_DIR`/`GIT_WORK_TREE` do ambiente antes de invocar `git`
   (`cmd.Env = append(os.Environ() sem essas duas chaves, ...)` ou `git -c` equivalentes) — mitiga
   sem expandir escopo, mas é código, então fora do escopo deste ML de revisão; se aceito, vira
   roadmap próprio ou emenda ao ADR atual.
3. **Não bloqueante, mas deve entrar na documentação (ML-3A/Zeus):** a resposta correta à
   Pergunta 4 (`lenient` não esconde o texto, só remove o poder de bloqueio) e à Pergunta 5
   (migração do baseline é intencional, mas precisa de nota de release, não só README).

**Se a decisão for aceitar o limite do `GIT_DIR`/`GIT_WORK_TREE` como está** (mesma lógica que
já aceitou "sem HEAD"), o roadmap pode seguir para PR — mas a redação do ADR (Decision point 4) e
do comentário em `validator_credential_guard_integrity.go:248-251` **precisa ser corrigida antes**,
porque hoje ambos afirmam algo que os PoCs acima refutam.

## Verificado × Avaliação — resumo

| Item | Verificado (código/PoC) | Avaliação |
|---|---|---|
| M4 cobre as 3 regras, único caminho de aplicação | ✓ código lido, `applyRule`/`applyRuleTagged` únicos | Correto |
| Outras ~38 regras inalteradas | ✓ `diskRuleSeverity` idêntico ao corpo pré-ADR | Correto |
| Carve-out do baseline, único filtro | ✓ código lido, `filterBaselineTagged` único ponto | Correto |
| Paridade 3 CLIs (M4 + carve-out) | ✓ grep confirma presença e lógica equivalente nos 3 | Correto |
| Cenários 50-53 no falsify | ✓ lidos, desenho sólido (baseline+detecção+não-vacuidade+não-regressão) | Correto, mas não cobrem `GIT_DIR`/`GIT_WORK_TREE` |
| `GIT_DIR`/`GIT_WORK_TREE` derrota M4 | ✓ **PoC empírico, 2 regras testadas** | 🔴 Brecha nova, fail-open, silenciosa |
| `lenient` esconde a violação | ✓ código lido, texto sempre presente | Não esconde; remove só o bloqueio |
| Migração do baseline | N/A (decisão de produto) | Saída existe e é correta; falta aviso de release |
