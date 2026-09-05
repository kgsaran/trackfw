---
status: Proposed
date: 2026-09-05
author: ""
---

## Contexto

O trackfw emite hooks de guard como scripts `.sh` e confia que o CLI de agente os execute.
**Medimos qual shell cada CLI usa no Windows** — leitura de código-fonte dos fornecedores e
documentação oficial, registrado em
`docs/portabilidade/2026-09-05-contrato-de-execucao-de-hook-por-cli-de-agente-no-windows.md`:

| CLI | shell no Windows | `.sh` dispara? | base |
|---|---|---|---|
| Gemini CLI | PowerShell, **sempre** | **não** | código-fonte do fornecedor |
| Codex CLI | PowerShell no caminho comum | **não** | código-fonte do fornecedor |
| GitHub Copilot CLI | — | **não** — populamos o campo errado | doc do fornecedor + nosso código |
| Claude Code | Git Bash se instalado, senão PowerShell | **condicional** | documentação do fornecedor |
| Cursor · Kiro | — | **indeterminado** | fechados, sem doc |

**PowerShell não interpreta shebang.** Nos CLIs marcados, o hook é escrito, o `validate` o reporta
instalado, **e ele nunca executa**.

🔴 **Para um produto de governança, este é o pior modo de falha que existe:** o controle que impede
`git push` bruto por subagente **reporta saúde sobre o que nunca inspecionou**. É a mesma família de
defeito que este repositório vem catalogando a semana toda — gate vácuo, teste verde por consequência
do bug, contagem que esconde regressão — só que na fronteira que o usuário confia.

E há um defeito que **não é de shell**: no Copilot populamos `"bash"` quando no Windows ele lê
`"powershell"` (ou `"command"`, cross-platform). `internal/generators/agentfiles.go:861`. Nem chega à
pergunta de execução: é **vácuo de configuração**.

## Decisão

### D1 — Hook de Windows roda **no Windows**. Git Bash não é requisito

A saída óbvia seria documentar "instale Git Bash". **Recusada**, e a razão é aritmética: **resolve um
CLI de seis.** Gemini, Codex e Copilot continuam quebrados com Git Bash instalado.

Exigir dependência externa que não resolve o problema é transferir custo ao usuário sem entregar o
controle. Um usuário corporativo em Windows instalaria o Git Bash e **continuaria sem guard**.

### D2 — A correção é **por CLI**, porque o defeito é por CLI

Não existe um remédio único. Cada linha da tabela tem causa própria:

- **Copilot** — trocar o campo do JSON. Não precisa de script novo, não precisa de Windows para
  verificar.
- **Gemini e Codex** — precisam de script **nativo** invocável por PowerShell.
- **Claude Code** — já funciona **se** houver Git Bash; a decisão é se passa a usar o caminho nativo
  também, para não depender de dependência externa.
- **Cursor e Kiro** — 🔴 **indeterminados: medir antes de emitir qualquer coisa.** Emitir script novo
  para CLI cujo mecanismo não conhecemos é inventar mecanismo, e é o que esta campanha recusa desde o
  grupo B das 56 falhas de Python.

### D3 — 🔴 Duas implementações do guard exigem **contrato de paridade comportamental**

O `git-branch-guard` tem **561 linhas** e decide se um `git push` bruto passa. Uma variante nativa é
uma **segunda implementação de controle de segurança** — e duas implementações divergem exatamente
onde ninguém testa.

Vale a mesma regra dura dos 3 CLIs: **comportamento verificado entre `.sh` e a variante nativa, por
gate, não por revisão.** Sem esse contrato, a variante nativa é dívida de segurança, não correção.

### D4 — 🔴 Escopo mínimo primeiro: **o que protege, não o que é conveniente**

`credential_guard` e `git_branch_guard` são **controle de segurança**. `attention-signal` e
`attention-cleanup` são conveniência.

Portar primeiro o que protege. Se o esforço se mostrar maior que o previsto, o corte cai na
conveniência — nunca no guard.

### D5 — O que **não** for portável fica **declarado no produto**, não só no README

Enquanto um CLI não tiver hook que execute no Windows, o `trackfw` deve dizê-lo **onde o usuário
olha** — no `validate`/`doctor`, não apenas na documentação.

🔴 Um `validate` que reporta hook "instalado" quando ele não pode executar **é o defeito**, não o
sintoma. Corrigir a emissão sem corrigir o relato deixaria o silêncio falso de pé.

## Consequências

**O suporte a Windows deixa de ser "quase" e passa a ser verificável por CLI.** A tabela vira o
contrato: cada linha tem um estado, e o estado é medido.

**Cresce a superfície de manutenção**: mais scripts, mais gates, mais cenários de falsificação — hoje
há **353 referências aos guards** só em `check-gates-falsify.sh`. É custo real e conhecido.

🔴 **Não cobre Cursor e Kiro sem medição nova.** A decisão vale para o que foi medido; para os dois
indeterminados, o próximo passo é **experimento**, não código.

## Verificação exigida de quem implementar

- Falsificação **nas duas direções, por CLI**: com o hook nativo, o guard **bloqueia** a operação que
  deve bloquear no Windows; sem ele, **não bloqueia**. Um hook que "instala" e não é exercitado não
  conta.
- 🔴 **A prova é o guard DISPARANDO**, não o arquivo existindo. Verificar que o arquivo foi escrito é
  exatamente o erro que produziu esta ADR.
- 🔴 **Controle POSIX:** Linux e macOS **inalterados**, medidos antes e depois. Nada aqui pode
  regredir o caminho que hoje funciona.
- Paridade `.sh` ↔ nativo exercitada por gate, com os mesmos cenários de bloqueio dos dois lados.
- Para Cursor e Kiro: **nenhuma emissão nova antes da medição**.
