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


---

## Adendo — 2026-09-06: a tabela fechou, e o desenho mudou

Cursor e Kiro deixaram de ser indeterminados. **Medido no bundle instalado**, numa VM Windows ARM64:

| CLI | shell no Windows | `.sh` dispara? |
|---|---|---|
| Gemini · Codex · **Cursor** | PowerShell | não |
| **Kiro** | **`cmd.exe`** (default do Node, `spawn(cmd,{shell:true})` via `%ComSpec%`) | não |
| Copilot | — (populamos o campo errado) | não |
| Claude Code | Git Bash se instalado | condicional |

**Nenhum dos seis executa o `.sh`**, exceto Claude Code com Git Bash. O Cursor **nunca** cai para
`cmd.exe` ou bash: tenta `pwsh`, depois `powershell.exe`, depois o caminho fixo, e lança exceção se
nenhum existir.

### 🔴 D6 — Emitir caminho de `.ps1` NÃO funciona. A `ExecutionPolicy` bloqueia

Medido na VM:

```
Get-ExecutionPolicy → Restricted        (padrão de Windows client)
powershell -File g.ps1  →  "não pode ser carregado porque a execução de scripts foi desabilitada"
```

Um `command` que aponte para um `.ps1` **falha em todos os CLIs de PowerShell**, não só no Kiro. Este
é o achado que teria produzido uma entrega quebrada se a implementação começasse pelo caminho óbvio.

### D7 — O `command` é uma **linha de invocação**, não um caminho — e ela é UNIFORME

```
powershell -NoProfile -ExecutionPolicy Bypass -File "<caminho>.ps1"
```

Medido nos dois mundos, com o **exit code preservado**, que é como o guard sinaliza bloqueio:

```
cmd /c powershell -NoProfile -ExecutionPolicy Bypass -File g.ps1   →  rodou, exit 2   (Kiro)
       powershell -NoProfile -ExecutionPolicy Bypass -File g.ps1   →  rodou, exit 2   (Cursor/Gemini/Codex)
```

**Consequência: o Kiro não precisa de tratamento especial nem de `.bat`.** Uma implementação `.ps1`,
uma linha de invocação, cinco CLIs. O que a D2 previa como "correção por CLI" se reduz a **um script
e um formato de comando** — mais Copilot, que continua sendo troca de campo, e Claude Code, que já
funciona.

🔴 **`-ExecutionPolicy Bypass` é decisão de segurança, não conveniência.** Ela existe porque a
alternativa — pedir ao usuário que afrouxe a política da máquina inteira — é pior: trocaria um hook
que não roda por uma máquina permanentemente mais permissiva. O `Bypass` vale **só para o processo
que invocamos**, e o script é gerado por nós, com integridade já verificada por
`credential_guard_script_integrity`.

### O que ainda NÃO foi medido
- Se o Kiro e os demais **propagam** o exit code do hook até a decisão de bloquear a ferramenta.
  Medimos que o exit code sobrevive à **cadeia de shell**; não que o CLI o **honra**.
- O comportamento com caminho contendo **espaço** (`C:\Program Files\...`) e **acento**.
