---
status: parecer
date: 2026-08-12
author: "Hades (hades-tf)"
---

# Parecer: estado final da detecção de adulteração do credential-guard — ML-3B

> ML-3B do `docs/roadmaps/wip/ROADMAP-2026-08-12-deteccao-de-adulteracao-do-credential-guard-regra-de-validate.md`.
> Não modifica código. Avalia o resultado de ML-1A (Apolo) e ML-2A (Ártemis) contra as 5 perguntas do
> despacho. Roda em paralelo com ML-3A (Hefesto, `docs/cli-parity.md` + doc de usuário final) — nenhum
> arquivo desse escopo foi tocado ou lido como fonte de verdade aqui.

## Convenção deste parecer

Cada afirmação é marcada **[Verificado]** (lido diretamente no código/repo, comando reproduzível dado)
ou **[Avaliação]** (julgamento sobre o que foi verificado, não é fato observável).

---

## 1. As três vias estão cobertas de fato?

| Via | Regra | Arquivo:linha |
|---|---|---|
| Deleção do script | `credential_guard_hook_resolvable` (pré-existente, não tocada por este roadmap) | `internal/validator/validator_credential_guard.go:113` |
| Sobrescrita do script | `credential_guard_script_integrity` (novo) | `internal/validator/validator_credential_guard_integrity.go:40` |
| Downgrade de `credential_guard.mode` | `credential_guard_mode_downgrade` (novo) | `internal/validator/validator_credential_guard_integrity.go:145` |

**[Verificado]** As três existem e disparam para o caso central de cada via — li os testes unitários
(`validator_credential_guard_integrity_test.go`, `npm/tests/credential_guard_integrity.test.js`,
`pypi/tests/test_credential_guard_integrity.py`) e os cenários 49/50 de `check-gates-falsify.sh`, que
provam não-vacuidade com braço `off` (mesma sabotagem, regra desligada, o braço de detecção **falharia**).

**[Verificado] Cobertura da via "deleção" é condicional, não incondicional — a ressalva já estava no
roadmap ("⚠️ só quando `validate` roda") e é mais estreita do que isso.** `credential_guard_hook_resolvable`
só produz mensagem quando existe uma **entrada de hook que referencia** `trackfw-credential-guard.sh`
(`collectCredentialGuardCommands`, linha 84-99: varre os 6 arquivos de hook de projeto por valor-string
contendo o marcador). O comentário do próprio código é explícito sobre o motivo (linha 106-108):
"Ausência de entrada de guard é estado legítimo... nunca é violação por si só" — porque um guard global
instalado via `trackfw update harness` não deixa entrada de projeto, e isso não pode ser tratado como
adulteração. **Consequência:** um adversário que apaga `scripts/trackfw-credential-guard.sh` **e** remove
a entrada correspondente do arquivo de hook (ex.: `.claude/settings.json`) na mesma edição neutraliza
as três regras de uma vez — `hook_resolvable` não tem mais nada para iterar, `script_integrity` está
programada para silenciar quando o script está ausente (linha 44-47, delega a `hook_resolvable` de
propósito, para não duplicar o mesmo relato sob dois nomes de regra).

**[Avaliação]** Isto não é um bug deste ML — é um limite pré-existente de `hook_resolvable`, e a
alternativa (acusar ausência de hook mesmo quando é legítima) criaria falso positivo pior. Mas é
material o suficiente para não ficar implícito: **"cobertura da via deleção" deve ser lida como
"cobertura de deleção parcial, com wiring intacto" — deleção total (script + wiring) é indetectável
pelo `validate` hoje.** Isso precisa estar escrito na doc de usuário final (ver seção 5).

---

## 2. A cópia local do template no validador é aceitável?

**[Verificado]** A cópia existe nos 3 stacks (`credentialGuardScriptReference` em Go,
`CREDENTIAL_GUARD_SCRIPT_REFERENCE` em Node, `_CREDENTIAL_GUARD_SCRIPT_REFERENCE` em Python — 6 cópias
contando os dois lados: template do gerador × cópia do validador, ×3 stacks). O comentário em
`validator_credential_guard_integrity_reference.go:6-11` documenta a razão do ciclo de import
(`internal/generators/context.go` já importa `internal/validator`) e por que a exportação de uma
função string-returning do gerador não é opção neste ML (fora de escopo).

**[Verificado]** Existe teste de paridade que **executa o gerador real** em cada stack e compara
byte-a-byte contra a cópia:
- Go: `TestCredentialGuardScriptReference_MatchesGenerator`
  (`validator_credential_guard_integrity_external_test.go`, package externo `validator_test` — importa
  `internal/generators` e `internal/validator` sem reintroduzir o ciclo). Chama
  `generators.GenerateCredentialGuardScript(dir)` de verdade.
- Node: `'CREDENTIAL_GUARD_SCRIPT_REFERENCE é byte-idêntico ao que generateCredentialGuardScript
  emite'` (`npm/tests/credential_guard_integrity.test.js:243`) — chama
  `require('../src/generators/hooks').generateCredentialGuardScript(dir)` de verdade.
- Python: `test_reference_e_byte_identico_ao_gerador_real`
  (`pypi/tests/test_credential_guard_integrity.py:78`) — importa
  `from trackfw.generators.init_gen import _generate_credential_guard_script` e chama a função real, não
  um literal reconstruído no teste (confirmado lendo o import, não só o nome do teste).

Isto é **materialmente diferente** de `TestCredentialGuardScript_ParityAcrossStacks`
(a lacuna que a Emenda 2 do ADR documentou): aquele teste faz *regex-scraping* dos literais Node/Python
e nunca os executa. Estes três testes de paridade da referência **executam o gerador real** do próprio
stack. Isso fecha a lacuna estrutural que a Emenda 2 apontou, para o par gerador↔cópia (não para o par
entre os 3 stacks — esse é o ML-0B, gate de paridade cross-stack já concluído e fora do escopo deste
parecer).

**[Verificado] Existe também teste comportamental, que executa o script gerado com um payload real e
observa o efeito (exit code / mensagem), não só compara texto.** `internal/generators/credential_guard_sabotage_test.go`
(`TestSabotage_ClaudeCode_JWTInBashCommand_WarnMode`, `..._BlockMode`, e equivalentes Cursor/Kiro),
espelhados em `npm/tests/credential_guard_sabotage.test.js` e
`pypi/tests/test_credential_guard_sabotage.py` (`test_jwt_no_comando_bash_modo_warn`,
`..._modo_block`, via `subprocess`, checando `returncode`).

**[Avaliação] Isto responde a pergunta central: "existe modo de a cópia divergir sem o teste pegar?"**
Sim, existe um resíduo teórico, e é irredutível a testes de igualdade: **se alguém editar o template do
gerador e a cópia do validador simultaneamente, no mesmo PR, com o mesmo conteúdo novo e igualmente
errado** (ex.: um bloco novo que introduz uma regressão funcional, mas que os dois arquivos concordam em
ter), o teste de paridade (que só verifica **igualdade entre os dois**) passa — porque os dois lados
realmente são iguais. O teste de paridade prova "gerador e cópia não divergiram um do outro"; não prova
"o conteúdo é correto". **O teste comportamental (sabotage_test) reduz, mas não elimina, esse risco**:
ele cobre os comportamentos que já tinha cobertura antes deste ML (detecção de JWT/AWS key, modo
warn/block) — um bug novo introduzido igualmente nos dois lados só é pego se cair dentro de um desses
comportamentos já testados. Uma regressão fora desse invólucro (ex.: um bloco novo que quebra outro
aspecto do script, não relacionado a JWT/AWS/modo) passaria pelos dois testes sem ser detectada.
**Isto é um risco residual aceitável, não um defeito do desenho**: é o mesmo risco que existiria em
qualquer par gerador/consumidor com revisão de PR humana como última linha — reduzir mais exigiria uma
fonte única de verdade (resolver o ciclo de import), que está fora do escopo desta ROADMAP por decisão
explícita (ver comentário linha 9-11 do arquivo de referência).

**Posição:** a cópia é aceitável **com** os dois testes de paridade e comportamental presentes (estão).
Sem o teste de paridade que executa o gerador real, a cópia seria inaceitável — o risco residual descrito
acima é o piso, não o teto, do que se aceita hoje.

---

## 3. A severidade `warning` do script cria brecha explorável?

**[Verificado]** `ruleDefaults["credential_guard_script_integrity"] = "warning"`
(`internal/validator/validator.go:108`) e `applyRule`/`applyRuleTagged` só promovem mensagens de
`warning` para `violations` (linhas 126-158) — `internal/commands/validate.go` só falha o exit code em
`violations` (confirmado no comentário do cenário 49, linha 4050-4051: "warnings só imprimem ⚠").
**Um adversário sabe disto: `validate` em CI passa mesmo com o script adulterado, no default.**

**[Verificado]** Esta decisão **já está tomada e justificada por escrito**, não é uma lacuna deste ML:
ADR Emenda 3 (`docs/adr/ADR-2026-08-12-...md`, linhas 136-143) — o script não carrega marcador de
versão, então a regra **não consegue** discriminar drift legítimo (usuário não rodou `trackfw update`)
de adulteração real. A decisão explícita foi severidade `warn` + mensagem causalmente neutra, com
"embutir versão/hash no template" citado como o trabalho futuro que permitiria elevar para `error`.

**[Avaliação] O sinal no output basta hoje, com uma condição: quem revisa precisa olhar warnings, não
só o exit code.** Isso não é automático — CI que só checa `exit != 0` não vê a mensagem. **A regra não é
inútil**, porque:
1. É **configurável por alvo**: `rules: credential_guard_script_integrity: error` promove para violation
   (testado em `npm/tests/credential_guard_integrity.test.js:172`, `check-gates-falsify.sh` cenário 49
   usa exatamente esse override para poder usar `assert_fails_with`). Um projeto que aceita o custo de
   falso positivo por drift de versão pode ligar `error` hoje, sem esperar a versão/hash.
2. `trackfw validate` (interativo/local) imprime o `⚠` visível a quem roda manualmente, mesmo sem
   promover o exit code.

**Não recomendo mudar o default agora** — a ADR já pesou o trade-off (falso positivo por drift vs. sinal
fraco) e decidiu; reabrir sem o marcador de versão implementado repete o padrão que a Emenda 3 evitou.
**O que falta não é mudar o default, é a doc de usuário final dizer explicitamente que `warning` não
falha CI** — ver seção 5.

---

## 4. `mode_downgrade` pode ser contornado trivialmente?

| Cenário do despacho | Verificado no código | Resultado |
|---|---|---|
| Adversário **commita** o downgrade | `headTrackfwYAML()` lê `git show HEAD:./trackfw.yaml`; se o downgrade está commitado, `headMode` deixa de ser `"block"` e a regra silencia (linha 152: `if headMode != "block" { return nil, nil }`) | **Silencioso — por desenho, documentado no ADR e no comentário da regra (linha 140-144): "commit é o rastro auditável que esta detecção existe para forçar".** Não é bypass no sentido de "escapou sem deixar rastro" — deixa um commit no histórico. É bypass no sentido de "a regra para de proteger daquele ponto em diante". |
| Adversário **remove a chave `credential_guard` inteira** em vez de rebaixá-la | `extractCredentialGuardMode` retorna `ok=false` quando não há linha `credential_guard:`; no lado disco, isso vira `diskMode=""`, que **não** é `"block"` → dispara (linha 165-170, testado em `npm/tests/credential_guard_integrity.test.js:136` "dispara quando a chave some do disco") | **Detectado**, se não commitado. Se a remoção **também** for commitada, mesmo caso da linha acima — silencia a partir daí. |
| Adversário edita `trackfw.yaml` em **repositório sem commits** | `headTrackfwYAML()` roda `git rev-parse --verify HEAD`; falha → `ok=false` → silêncio (linha 121-124, testado em ambos os 3 stacks) | **Silencioso — sem âncora, é a condição explícita do critério de aceite da ROADMAP ("Não dispara sem âncora disponível"), não uma falha.** |

**[Verificado] — achado não levantado no despacho original, mais grave que os três acima: a regra
pode se auto-silenciar no mesmo commit/edição que faz o downgrade.** `ruleSeverity()`
(`internal/validator/validator.go:113-121`) lê a severidade via `config.Load()`, que **sempre lê o
`trackfw.yaml` em disco** (`internal/config/config.go`, confirmado: não há leitura de `HEAD` em
`config.Load` em nenhum ponto do pacote `internal/config`). Isso significa: um adversário que edita
`trackfw.yaml` para rebaixar `credential_guard.mode: block → warn` **e**, na mesma edição não commitada,
acrescenta `rules:\n  credential_guard_mode_downgrade: off`, silencia a regra usando o **mesmo arquivo**
que ela deveria estar inspecionando. `npm/tests/credential_guard_integrity.test.js:223-239`
(`'credential_guard_mode_downgrade: rules: off silencia totalmente'`) prova exatamente este caminho
funcionando — o teste existe para provar que o *knob* funciona, mas o efeito colateral é que o mesmo
knob é uma via de auto-silenciamento não coberta por nenhuma âncora no `HEAD`.

**[Avaliação]** Isto **não é um bug de implementação** — é a mesma classe de limite que o ADR já aceita
("detecção, não prevenção; quem escreve o repositório derrota detecção com escopo no repositório").
`rules:` sempre foi lido do disco, para todas as regras do `validate`, não é novo deste ML. Mas é **o
achado mais afiado desta revisão** porque é simétrico e generalizável: qualquer regra nova que dependa
de `HEAD` para o dado protegido, mas de disco para a severidade, tem essa mesma abertura. **Não bloqueia
este PR** (seria pedir a este ML para resolver um problema estrutural do sistema de `rules:`, fora do
escopo do REQ). **Recomendo abrir um REQ novo, não deste roadmap**, para avaliar comparar `rules:`
também contra `HEAD` de forma direcional (mesma lógica já usada para `credential_guard.mode`) — mas
isso é trabalho futuro, não pré-requisito deste PR.

---

## 5. O conjunto entregue cria falso senso de segurança?

**[Verificado]** Depois deste roadmap existem 3 regras de `validate` com prefixo `credential_guard_`:
`credential_guard_hook_resolvable` (deleção/resolução, pré-existente), `credential_guard_script_integrity`
(sobrescrita, novo, `warning`), `credential_guard_mode_downgrade` (downgrade de modo, novo, `error`).
Alguém lendo só os nomes das 3 regras pode razoavelmente concluir "o guard está protegido contra
adulteração" sem as ressalvas acima.

**[Avaliação] O que a doc de usuário final (entregável do ML-3A, Hefesto, em paralelo) precisa dizer,
explicitamente, para não deixar essa leitura:**

1. **"Detecção ≠ prevenção", e só quando `validate` roda** — nada impede a ferramenta protegida
   (`trackfw-credential-guard.sh`) de já ter sido usada para vazar um segredo antes da próxima execução
   do `validate`; a regra não intercepta em tempo real.
2. **As condições de silêncio de cada regra, sem eufemismo**:
   - `credential_guard_hook_resolvable` / `credential_guard_script_integrity`: silenciam se o script
     **e** a entrada de hook correspondente forem removidos juntos (seção 1 deste parecer).
   - `credential_guard_mode_downgrade`: silencia sem repositório git, sem nenhum commit, ou com
     `trackfw.yaml` não versionado no `HEAD` — "sem âncora" é comportamento normal, não falha.
3. **Severidades default e o efeito real em CI**: `credential_guard_script_integrity` é `warning` por
   default e **não falha `trackfw validate`** (não derruba o exit code) — só `error` falha; quem quer
   bloquear CI nesse braço precisa configurar `rules: credential_guard_script_integrity: error`
   explicitamente. `credential_guard_mode_downgrade` já é `error` por default.
4. **Commitar a mudança, ou editar `rules:` no mesmo arquivo, desativa a detecção — por desenho.** Um
   downgrade de `mode` commitado, ou uma entrada `rules: <nome-da-regra>: off` no `trackfw.yaml` em
   disco, silenciam a regra correspondente a partir daquele ponto. Isto não é um defeito a ser corrigido
   por este roadmap — é a fronteira "quem tem escrita no repositório derrota detecção com escopo no
   repositório" que o ADR já assume.

---

## Conclusão

**Nada bloqueia o PR.** As três vias têm cobertura real e testada (unitária ×3 stacks + prova de
não-vacuidade nos cenários 49/50 do `check-gates-falsify.sh`); a cópia do template tem teste de paridade
que executa o gerador real nos 3 stacks e teste comportamental que executa o script gerado; a severidade
`warning` do braço de script é uma decisão já tomada e justificada por escrito na Emenda 3 do ADR, não
uma lacuna; e os limites de `mode_downgrade` (commit, sem-âncora, remoção-vs-downgrade) são o
comportamento esperado do desenho "detecção ancorada no `HEAD`", também documentado na ADR.

**Ressalvas que precisam estar escritas na doc de usuário final (ML-3A), não corrigidas neste PR:**
- Cobertura de "deleção" é condicional a sobreviver a entrada de hook (seção 1).
- `warning` não falha CI por default (seção 3).
- `mode_downgrade` silencia se o downgrade for commitado, se `rules:` for editado no mesmo arquivo em
  disco (achado novo, seção 4), ou sem âncora de git.

**Follow-up recomendado, fora deste roadmap:** REQ novo para avaliar ancorar `rules:` também no `HEAD`
de forma direcional (mesmo padrão de `credential_guard.mode`), fechando a via de auto-silenciamento
descrita na seção 4. Não é pré-requisito para este PR.

**Comandos usados nesta verificação:**
```
go build ./...
./bin/trackfw validate      # ✓ No violations found. — neste repositório
```
