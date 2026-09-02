---
status: Open
date: 2026-09-02
author: "kgsaran"
adr: ""
roadmap: ""
---

# REQ: O guard instalado emite schema de hook que o Claude Code rejeita, e a razão do bloqueio se perde — nos 3 CLIs

> Date: 2026-09-02 | Status: Open
| Linear Issue:
| Jira Issue:

## Motivation

### O sintoma, observado ao vivo

Na sessão de um subagente, repetidamente:

```
PreToolUse:Bash hook error
Hook JSON output validation failed — (root): Invalid input
```

`(root)` significa que o objeto inteiro é recusado na raiz — não é campo faltando, é forma errada.

### A causa, reproduzida

`scripts/trackfw-git-branch-guard.sh:559` e `internal/generators/scaffold.go:1855` emitem:

```bash
printf '{"decision":"block","reason":"%s"}\n' "$REASON"
```

Reproduzido nos dois caminhos:

```
comando permitido  -> rc=0, stdout vazio                  -> sem erro
comando bloqueado  -> rc=2, {"decision":"block",...}      -> erro de validacao
```

O schema aceito hoje pelo Claude Code para `PreToolUse` é:

```json
{
  "hookSpecificOutput": {
    "hookEventName": "PreToolUse",
    "permissionDecision": "deny",
    "permissionDecisionReason": "texto"
  }
}
```

**Não existe nenhuma ocorrência de `hookSpecificOutput` no repositório** — nem em `internal/`, nem em
`npm/src/`, nem em `pypi/trackfw/`. Verificado por `grep -rln`.

### 🔴 Não é furo de segurança — e isso está confirmado, não presumido

A documentação oficial (`code.claude.com/docs/en/hooks`) é explícita:

> *"Exit 2 blocks whether or not you print JSON: even a JSON `permissionDecision` of `"allow"` can't
> override it."*

E há evidência empírica direta desta mesma sessão: um `git checkout --` foi bloqueado pelo guard, com
a mensagem do trackfw visível. **O guard falha fechado.** A severidade é de usabilidade e ruído, não
de bypass.

### O que se perde de fato

A mesma documentação:

> *"The blocking message is the reason from your JSON's blocking decision when it makes one, and your
> stderr text otherwise."*

Nosso `REASON` vai para o **stdout**, dentro de um JSON que é descartado. O **stderr fica vazio**.
Ou seja, no caminho documentado o usuário recebe um "hook error" **sem a explicação do porquê** — e
a explicação é justamente o que ensina a usar `trackfw commit` em vez de `git commit`.

### 🔴 Duas fontes de terceiros afirmam o contrário, e estão erradas

Uma consulta de apoio devolveu, citando um gist e um blog, que o formato legado
`{"decision":"approve"|"block","reason":"..."}` *"permanece funcional"*. **A documentação oficial não
o menciona**, e a evidência primária — o nosso próprio hook sendo rejeitado — o contradiz
diretamente.

Registrar isto importa: quem for corrigir vai encontrar as mesmas fontes no topo da busca. **A
medição no ambiente real vence o gist.**

### Alcance

O `internal/generators/scaffold.go` **instala este script na máquina de quem adota o trackfw**. Não é
defeito do nosso repositório: é do produto, em todo adotante que usa Claude Code. É o terceiro
defeito de produto encontrado hoje pelo simples uso do trackfw no trackfw — junto com o
`.trackfw-log` que conflita sempre e o script de atenção sem codificação declarada.

### O comentário do gerador registra a decisão que envelheceu

`internal/generators/scaffold.go:1273` declara que o script *"emite SEMPRE os dois formatos de decisão
simultaneamente — `{"decision":"block",...}` no stdout (formato Claude/Gemini) e `exit 2` (formato
Codex/Windsurf/Cursor por exit-code)"*.

A estratégia dos dois formatos **continua certa** — é o que faz o guard funcionar em runtimes
diferentes. O que caducou é **qual** JSON representa o formato Claude. Mesmo padrão da REQ do job
`parity`: a decisão estava certa quando foi tomada, e nada a revalidou.

## Acceptance Criteria

- [ ] **AC1** — O guard emite `hookSpecificOutput` com `hookEventName`, `permissionDecision: "deny"`
      e `permissionDecisionReason` no stdout, e o Claude Code **não** registra erro de validação.
      Verificado por execução, não por leitura.
- [ ] **AC2** — 🔴 **A razão chega ao modelo:** o `REASON` também vai para o **stderr**, porque é de
      lá que a mensagem é lida quando o JSON não produz decisão. Falsificado: bloqueio com JSON
      propositalmente inválido ainda mostra a razão.
- [ ] **AC3** — 🔴 **O `exit 2` é preservado.** Ele é o que faz o guard funcionar em
      Codex/Windsurf/Cursor, e é o que garante fail-closed. Falsificação: com o JSON removido, o
      comando **continua bloqueado**.
- [ ] **AC4** — 🔴 **Controle — o guard não passa a bloquear o que antes permitia.** Comando
      permitido continua com `rc=0` e stdout vazio. Um guard que vira ruidoso demais é desligado
      pelo usuário, e aí não guarda nada.
- [ ] **AC5** — Paridade nos **3 CLIs**, literal byte-idêntico, verificada pelo
      `check-attention-scripts-parity.sh` (que já cobre `trackfw-git-branch-guard.sh`).
- [ ] **AC6** — `scripts/trackfw-git-branch-guard.sh` (a cópia *dogfooded* deste repo) é regenerada
      junto — mesmo descuido do ML-1A desta sprint, quando o script de atenção ficou defasado e
      **nada** o comparava com o gerador.
- [ ] **AC7** — O comentário de `scaffold.go:1273` é atualizado com o schema novo **e com a data**,
      para a próxima revisão saber quando a premissa foi vista pela última vez.
- [ ] **AC8** — Nota de vault registrando (a) que o `exit 2` é o que salva o fail-closed, e (b) que
      as fontes de terceiros afirmam que o formato legado funciona e **estão erradas**.

## Negative Scope

- **Não** trocar `exit 2` por `exit 0` com JSON, embora alguma fonte recomende. `exit 0` entrega o
  bloqueio **inteiramente** à validação do JSON: se ela falhar por qualquer motivo, o comando
  **executa**. Isso troca fail-closed por fail-open num guard de segurança. **Inaceitável.**
- **Não** mexer no `trackfw-credential-guard.sh` nesta REQ sem medir antes se ele tem o mesmo
  defeito. Se tiver, é o mesmo ML; se não, não inventar trabalho.
- **Não** alterar a detecção de comandos do guard (o que é bloqueado). Esta REQ é só sobre **como a
  decisão é comunicada**.
- **Não** tratar como incidente de segurança. Fail-closed está confirmado por documentação oficial
  **e** por observação direta. Escalar isso gastaria credibilidade que os achados reais precisam.

## Linked ADR
<!-- A estratégia de dois formatos já está registrada em
     ADR-2026-08-17-guard-global-cabeado-com-no-op-fora-de-projeto-trackfw.md. Esta REQ corrige o
     formato, não a estratégia — sem ADR nova. Confirmar ao escrever o roadmap. -->
ADR:

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
<!-- Roadmap não criado ainda: o ML-1A do .gitattributes está em execução e toca exatamente os 3
     geradores que esta REQ vai alterar (scaffold.go, init.js, init_gen.py). Despachar depois, para
     não pôr dois agentes no mesmo arquivo — colisão que já custou caro nesta sprint. -->
Roadmap:
