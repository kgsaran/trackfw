# Parecer de segurança: mecanismo para impedir auto-silenciamento das regras de credential-guard

> ML-0A do roadmap
> `docs/roadmaps/wip/ROADMAP-2026-08-12-ancorar-rules-no-head-para-as-regras-de-credential-guard.md`.
> **Não modifica código.** Leitura de `internal/validator/validator.go`,
> `internal/validator/validator_credential_guard_integrity.go`, `internal/config/config.go`,
> `docs/adr/ADR-2026-08-12-nao-ha-prevencao-...-deteccao-ancorada-no-git.md` e `.gitignore`.

Convenção: **Verificado** cita código/config lido diretamente, com arquivo:linha. **Avaliação (Hades)**
é interpretação minha — decisão de desenho, trade-off, ou recomendação.

---

## 0. Achado que muda o escopo da pergunta original

**Verificado.** O roadmap enquadra o problema como "`rules:` em disco silencia a regra". Mas
`ruleSeverity()`/`rules:` **não é o único canal disco-e-não-commitado** que pode fazer a mesma
violação desaparecer do `trackfw validate`:

| Canal | Onde | Lido de | Commitável? |
|---|---|---|---|
| `rules: <nome>: off\|warning` | `trackfw.yaml` | disco (`config.Load()`, `validator.go:117`) | sim, mas não é exigido |
| `.trackfw-baseline.json` (ratchet) | arquivo próprio | disco (`LoadBaseline()`, `validator.go:28`) | **NÃO — `.gitignore:15` marca o arquivo como "não versionado" por desenho** |
| `governance_mode: lenient` (+ `lenient_until`) | `trackfw.yaml` | disco (`config.Load()`, `validator.go:264-290`) | sim, mas não é exigido |

**Avaliação (Hades).** Os três canais convergem no mesmo defeito estrutural do achado do ML-3B
anterior: leitura do disco do CWD, nunca do `HEAD`. Um mecanismo que só ancora `rules:` **fecha um dos
três** e deixaria a alegação "auto-silenciamento resolvido" **falsa** se declarada sem qualificação. O
canal do baseline é o mais grave dos três: **não pode ser fechado por "exigir commit"**, porque o
arquivo é propositalmente ignorado pelo git (`.gitignore:14-15`, comentário "baseline local de
violations toleradas (nao versionado)") — não é um esquecimento de commit, é o desenho da feature de
ratchet (ML-3A/B/C do REQ-2026-06-13, tolerar débito legado sem versionar exceções). Isso significa
que **nenhum anchoring em `HEAD` fecha esse canal**: a única forma é tirar as duas regras de
credential-guard da elegibilidade de ratchet, por nome de regra, independente de onde o filtro lê.

As três perguntas abaixo respondem primeiro ao canal `rules:` (o que o roadmap nomeou), depois tratam
os outros dois como achado adicional com recomendação própria.

---

## 1. Mecanismos possíveis para o canal `rules:` — cada um com a resposta à recursão

### M1 — Meta-regra que compara `rules:` em disco × `HEAD` ❌ Rejeitado
Cria uma nova regra de validate (`rules_config_downgrade` ou similar) que dispara quando `rules:` em
disco é mais fraco que no `HEAD`.
**O que impede o adversário de desligar isto também?** Nada — a meta-regra passa pelo mesmo
`applyRule()`/`ruleSeverity()` que todas as outras, então é ela própria configurável via
`rules: rules_config_downgrade: off` na mesma edição não commitada. Empurra o problema um nível, é
exatamente a armadilha que o roadmap nomeia. **Rejeitado no próprio parecer.**

### M2 — Assinatura/hash externo do `trackfw.yaml` guardado fora do repo ❌ Rejeitado
Guarda um hash de referência em `~/.trackfw/` (escopo global) e compara contra o disco.
**O que impede o adversário de desligar isto também?** Reabre exatamente o debate que o
`ADR-2026-08-12` já fechou: a defesa dependeria do escopo global, que o próprio ADR mediu como **não
neutro** — o adversário induzido escreve dentro do workspace, não precisa alcançar `$HOME`, e mover a
defesa para lá troca visibilidade (auditável no repo) por custo sem fechar a via. Reabrir esse debate
está fora do escopo negativo do roadmap ("Não reabre prevenção × detecção"). **Rejeitado.**

### M3 — Ancorar `rules:` no `HEAD` **globalmente**, para as ~40 regras via `ruleSeverity()` ⚠️ Funciona, mas viola a restrição de escopo
Faz `ruleSeverity()` (a função compartilhada, `validator.go:117`) ler `rules:` do `HEAD` em vez do
disco, para **todas** as regras.
**O que impede o adversário de desligar isto também?** Nada precisa desligar — não há mais chave em
disco a consultar; a leitura em si é a defesa. A recursão termina de fato.
**Mas:** isso muda o comportamento de configuração de **todas as ~40 regras do validador** — inclusive
`wip_limit`, `stale_wip`, `adr_orphan`, etc. — para usuários que nunca tiveram relação com segurança,
forçando-os a commitar qualquer relaxamento de severidade antes que ele valha. É exatamente a mudança
de maquinaria compartilhada que o roadmap proíbe sem justificativa explícita (§"Restrição de escopo").
**Não recomendado como está** — ver M4 para a versão com blast radius zero.

### M4 — Ancorar `rules:` no `HEAD`, mas só para as 2 regras de credential-guard ✅ Recomendado
Mesmo princípio de M3 (severidade lida do `HEAD`, não do disco), mas dentro de `ruleSeverity()` como
um **branch guardado por nome de regra**, não uma mudança da função inteira:

```go
func ruleSeverity(name string) string {
    if name == "credential_guard_mode_downgrade" || name == "credential_guard_script_integrity" {
        return credentialGuardRuleSeverity(name) // lê do HEAD, ver §2
    }
    cfg := config.Load()
    if s, ok := cfg.Rules[name]; ok { return s }
    if d, ok := ruleDefaults[name]; ok { return d }
    return "error"
}
```
**O que impede o adversário de desligar isto também?** Para essas 2 regras, `cfg.Rules[name]` do disco
**deixa de ser consultado por completo**. Não há chave a flipar — a mesma razão de fundo que torna
"não configurável" não-recursivo (§5), mas sem abrir mão da configurabilidade **commitada** (§4). Para
as outras ~38 regras, o branch nem é alcançado — código textualmente idêntico ao de hoje.
**Recomendado.** Custo detalhado em §2.

---

## 2. Recomendação e custo em maquinaria compartilhada

**Recomendo M4**: branch guardado por nome de regra dentro de `ruleSeverity()`, não duplicação de
`applyRule`/`applyRuleTagged` (que multiplicaria os dois call sites e o risco de as duas cópias
divergirem) e não uma segunda função de dispatch paralela (que faria os dois call sites de
`credential_guard_*` — `validator.go:432,438,632,638` — terem que "saber" para qual chamar, reintroduzindo
o mesmo tipo de bug do gate cego documentado na Emenda 2 do ADR).

**Semântica de resolução — direcional, não substituição:** severidade efetiva = **a mais estrita**
entre a resolvida no `HEAD` e a resolvida no disco (`error > warning > off`), não "ignora o disco e usa
só o HEAD". Isso resolve de graça um caso de borda que uma leitura ingênua do M4 erraria: quando o
`HEAD` **não tem** `rules: credential_guard_mode_downgrade` (o caso normal, hoje, para praticamente
todo repositório) não existe "valor do HEAD para cair de volta ao disco" — o disco reentraria e o
buraco reabriria. Com a regra direcional, ausência de chave no `HEAD` resolve para o default
(`ruleDefaults`/`"error"`), que já é o mais estrito possível, então o disco só pode **igualar ou ser
mais fraco** — nunca mais fraco **vence**. E a mesma regra permite o ajuste ergonômico legítimo de
**subir** a severidade localmente sem commit (ex.: elevar de `warning` para `error` num experimento
local) sem quebrar nada.

**Custo declarado:**
- **Zero delta comportamental para as outras ~38 regras** — o branch é guardado por nome; o teste de
  regressão é objetivo: `ruleSeverity("wip_limit")` etc. continuam no mesmo caminho de código de hoje,
  provável por leitura estática (não há teste que precise ser inventado só pra provar isso — a suíte
  existente de `ruleSeverity`/`applyRule` já cobre as outras regras e não muda).
- **Reaproveita `headTrackfwYAML()`** (`validator_credential_guard_integrity.go:117`), já existente e
  já testado para o valor de `mode`. Precisa de um parser de `rules: <nome>: <valor>` sobre o conteúdo
  do `HEAD` — não existe hoje; é a peça nova. Como `internal/config` só expõe `Load()` (que sempre lê
  do CWD) e `parse()` (não exportado), a forma mais barata é expor uma função pequena
  `config.ParseRulesFromContent(content string) map[string]string` que reutiliza o `parse()` interno
  sobre uma string arbitrária — aditiva, não altera `Load()`/`parse()` existentes.
- **Paridade nos 3 CLIs:** Node.js já tem o equivalente de `headTrackfwYAML()` — verificado em
  `npm/src/validator/index.js:1533` (`execSync('git show HEAD:./trackfw.yaml', ...)`), herdado do ML
  anterior que ancorou `mode`. Python precisa ser conferido no Wave 1 (não verificado aqui — fora do
  escopo de leitura deste ML), mas o padrão já existe em pelo menos 2 dos 3 CLIs; o custo de paridade é
  incremental, não green-field.

---

## 3. Sem `HEAD` — posição e trade-off assumido

**Recomendação: cair no disco (comportamento de hoje) quando não há âncora, e aceitar o limite.**

Um "sem âncora" acontece em exatamente três casos, todos já enumerados por `headTrackfwYAML()`
(`validator_credential_guard_integrity.go:118-131`): não é worktree git, não há `HEAD` (zero commits),
ou `trackfw.yaml` não está rastreado no `HEAD`. Nesses três casos não há valor de referência para
comparar — "ignorar `rules:` por completo" quebraria a configuração legítima de qualquer projeto sem
`trackfw.yaml` versionado (um caso real e não hipotético: ferramentas de scaffolding que geram
`trackfw.yaml` fora do controle de versão, ou o instante entre `trackfw init` e o primeiro commit).

**Por que isso não é um bypass de graça, e sim um limite já aceito:** o adversário **não controla**
esse estado a partir de uma edição no workspace. `git show HEAD:./trackfw.yaml` não é afetado por
edição em disco, por `git add` (staged mas não commitado) nem por `git rm --cached` — só por destruir o
próprio `.git` ou por o arquivo genuinamente nunca ter sido commitado. Destruir `.git` é uma ação
**maximamente ruidosa** (não é uma edição de config, é sabotagem do próprio controle de versão) e cai
no mesmo limite que o `ADR-2026-08-12` já registra textualmente: *"Detecção via HEAD não cobre o que
nunca foi commitado — projeto antes do primeiro commit, ou alterações ainda não versionadas. Limite
conhecido."* (linhas 158-159). Este ML não descobre um limite novo — herda o já assumido, e a M4 não o
piora nem o esconde.

---

## 4. Desligamento legítimo e commitado — continua funcionando

Fluxo concreto sob M4:

1. Um mantenedor decide, por razão legítima (ex.: repositório de exemplo/tutorial sem credenciais
   reais), que `credential_guard_mode_downgrade` deve ficar `off`.
2. Edita `trackfw.yaml`: adiciona `rules: { credential_guard_mode_downgrade: off }`.
3. **Commita** essa mudança (`git commit`). Isso é o passo que M4 exige e que hoje é opcional — a
   diferença central deste mecanismo.
4. A partir desse commit, `git show HEAD:./trackfw.yaml` passa a conter `off` para essa regra.
   `credentialGuardRuleSeverity()` lê esse valor do `HEAD`, a comparação direcional (§2) encontra
   "`off` no HEAD" × "qualquer coisa no disco" e resolve para `off` — a regra fica silenciosa, exatamente
   como o mantenedor pretendia.
5. O que o revisor de PR/code review vê: um diff explícito `+  credential_guard_mode_downgrade: off`
   no `trackfw.yaml`, no mesmo commit — auditável por construção, o mesmo padrão de visibilidade que o
   `ADR-2026-08-12` já estabeleceu para o `mode` em si (linhas 40-50).

Nenhum passo extra além do que já é esperado de qualquer mudança de configuração versionada.

---

## 5. Caminho mais simples? — e os dois canais fora do `rules:`

**Para o canal `rules:`:** a alternativa mais simples do enunciado — "tornar a regra não configurável"
— foi avaliada e **rejeitada**: quebraria o critério de aceite "desligamento legítimo continua
funcionando" (§4), que é explícito no roadmap. A outra alternativa do enunciado — "reportar o
rebaixamento de `rules:` dentro da própria mensagem da regra de `mode`, em vez de regra separada" — na
prática **é o que M4 já faz de forma equivalente**: não cria regra nova (evitando a recursão do M1) e
a mensagem de `validateCredentialGuardModeDowngrade` já é única e não precisa de uma segunda regra para
descrever o rebaixamento de severidade — a severidade em si é o que passa a estar ancorada. Não há um
caminho mais simples que M4 para este canal especificamente.

**Para os dois canais adicionais (§0), a recomendação muda de forma:**

- **Baseline (`.trackfw-baseline.json`):** como o arquivo é propositalmente não versionado
  (`.gitignore:14-15`), ancorar no `HEAD` **não se aplica** — não há "commit" possível para esse
  arquivo sem reverter a decisão de design do ratchet. O mecanismo correto aqui **é** o mais simples do
  menu do enunciado: **tornar as duas regras de credential-guard não elegíveis para exclusão via
  baseline**, por nome de regra, no filtro que roda em `Validate()` (`validator.go:449-478`) e em
  `ValidateTagged()` (`validator.go:672-706`). Isso não reabre a discussão de configurabilidade — o
  ratchet nunca foi o canal sancionado para desligar controle de segurança, é uma ferramenta de dívida
  técnica; excluir 2 regras nomeadas dele é politicamente equivalente a "essas regras não são dívida
  técnica tolerável". Custo de implementação: o caminho `Validate()` hoje filtra por string de mensagem
  pura, sem tag de regra (`validator.go:449-478` não carrega `Rule`); as duas mensagens de
  credential-guard são constantes conhecidas
  (`credentialGuardModeDowngradeMessage()` e a mensagem fixa de `validateCredentialGuardScriptIntegrity`),
  então a exclusão pode ser feita por correspondência de conteúdo, ou — mais limpo — fazendo `Validate()`
  reaproveitar `validateUnfilteredTagged()` internamente e filtrar por `Rule` antes de descartar as tags,
  encerrando a divergência entre os dois caminhos.
- **`governance_mode: lenient`:** é o canal de blast radius mais amplo dos três — converte **toda** a
  saída de `validate` (todas as regras, não só credential-guard) em warnings com exit 0, e é lido do
  disco sem exigir commit (`validator.go:264-290`, `IsLenient()`). Ancorar isso no `HEAD` teria o mesmo
  problema de escopo do M3 rejeitado (afeta todas as regras) e não é uma decisão que cabe a este ML —
  seria coerente aplicar o mesmo princípio de M4 ("essas 2 regras não obedecem ao modo lenient",
  análogo ao carve-out do baseline), mas isso é uma mudança na semântica do modo lenient em si, uma
  feature com propósito próprio (grace period de adoção, com `lenient_until` como salvaguarda de prazo)
  e não deveria ser decidida como efeito colateral deste roadmap.

**🔴 Não vale a pena fechar os três no mesmo ML.** O canal `rules:` (M4) é barato, tem blast radius
zero e é o que o roadmap nomeou. O canal baseline é barato **e** necessário — sem ele, a alegação de
"fechado" no `docs/cli-parity.md`/README seria falsa mesmo depois de M4, porque o baseline continuaria
sendo um bypass total, silencioso e por desenho não commitável, para exatamente as mesmas 2 regras.
O canal `governance_mode: lenient` é o único dos três que legitimamente pede uma decisão separada de
Zeus — não porque seja caro, mas porque redefine o que "modo lenient" significa, e essa é uma decisão
de produto/ADR, não uma correção de bug.

---

## Recomendação acionável (para o ADR de Zeus)

1. **Fechar M4** — `ruleSeverity()` ganha um branch guardado por nome de regra
   (`credential_guard_mode_downgrade`, `credential_guard_script_integrity`) que resolve severidade pela
   **mais estrita** entre `HEAD` e disco. Sem alteração de comportamento para as demais ~38 regras.
   Nos 3 CLIs (Go pronto para reaproveitar `headTrackfwYAML()`; Node já tem o equivalente; Python a
   confirmar no Wave 1).
2. **Fechar o carve-out de baseline** no mesmo Wave 1, como parte do mesmo ML ou de um ML irmão — sem
   ele, a alegação "auto-silenciamento fechado" é falsa. Excluir as 2 regras de credential-guard da
   elegibilidade de `.trackfw-baseline.json`, nos dois caminhos (`Validate()`/`ValidateTagged()`) e nos
   3 CLIs.
3. **Não decidir `governance_mode: lenient` neste roadmap.** Documentar o canal como limite conhecido
   (mesmo padrão de honestidade do ADR-2026-08-12 sobre "não cobre o que nunca foi commitado") e deixar
   para Zeus abrir REQ/ADR próprio se decidir que vale fechar — é uma mudança de semântica de produto,
   não uma correção pontual.
4. **Sem `HEAD` (§3):** cair no disco, aceitar o limite documentado — não é um bypass acionável pelo
   adversário sem sabotar o próprio `.git`.
5. **Não é "documentar e parar".** M4 + carve-out de baseline são baratos, têm blast radius delimitado
   (2 regras nomeadas) e fecham os dois canais que o roadmap e a leitura adicional deste ML
   identificaram como realmente exploráveis por uma edição não commitada. A Barreira B0 deve liberar a
   Wave 1 com este escopo — **não** apenas o `rules:` original, e **não** o `governance_mode`.
