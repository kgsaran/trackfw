---
status: Accepted
date: 2026-09-04
author: "trackfw_architect (Zeus)"
---

# ADR: Caminho POSIX ancorado num config lido por CLI de agente é "absoluto", independente do SO host

> Date: 2026-09-04 | Status: Accepted

## Contexto

`filepath.IsAbs("/opt/foo/guard.sh")` devolve **`false`** no Windows — a definição de "absoluto" do
Go para Windows exige letra de unidade ou UNC.

Consequência medida: `classifyHookAnchorage`
(`internal/validator/validator_credential_guard.go`) classifica um caminho **ancorado** como
**relativo** (classe 2), e o validator **deixa de emitir a violation** de guard ausente ou malformado.

~14 das 217 falhas reais de Windows. **E é a única do lote que é de segurança:** a detecção de hook
de guard **enfraquece no Windows** — o controle que impede `git push` bruto por subagente reporta
saúde onde não olhou.

## Por que a definição do SO host é a errada aqui

O caminho em questão vive **dentro de um JSON de configuração lido por um CLI de agente**, e é
executado por **bash**:

```json
"command": "$CLAUDE_PROJECT_DIR/scripts/trackfw-git-branch-guard.sh"
```

Quem interpreta esse caminho é o **bash** — não o `filepath` do Go, não o Windows. Para o bash,
`/opt/foo/guard.sh` é **absoluto**, rode ele em Linux, macOS ou Git Bash sobre Windows.

Perguntar ao `filepath.IsAbs` é **perguntar à autoridade errada**. É o mesmo erro de categoria da
`ADR-2026-09-04-separador-posix-...`: **o consumidor decide, não o SO host.**

## Decisão

### D1 — Para caminho em config de agente, "ancorado" é a pergunta certa

A classificação usa um predicado de **ancoragem** — o caminho começa em `/`, em `~`, ou numa
variável de ambiente do harness (`$CLAUDE_PROJECT_DIR`) — e **não** `filepath.IsAbs`.

Uma letra de unidade (`C:\...`) também é ancorada. O predicado é a **união**, não a substituição.

### D2 — `filepath.IsAbs` continua para caminho que o SO abre

Nada aqui muda a classificação de caminho que vai a uma syscall. 🔴 **Trocar o predicado no lugar
errado quebraria a resolução real de caminho no Windows** — e o modo de falha seria intermitente.

Vale a mesma fronteira da ADR do separador: **emissão e classificação de config, sim; travessia de
sistema de arquivos, não.**

### D3 — O predicado é dos 3 CLIs, byte-idêntico

Regra dura de paridade. Node e Python têm o mesmo defeito por caminhos diferentes
(`path.isAbsolute`, `os.path.isabs`), e a mensagem de classificação já é contrato de paridade.

### D4 — 🔴 A mensagem tem de dizer o motivo certo

Achado do ML-2B: `cwdDependentReason` tem **dois ramos** — `$PWD` e *"bare relative path"* — e **não
há ramo para til**. Hoje `~usuario/` e `"~/"` caem no catch-all e recebem *"bare relative path"*, que
**não é o motivo real**: `~user/` **expande** em POSIX, e `"~/"` falha por **aspas**.

**Classificar certo e explicar errado manda o leitor investigar a coisa errada.** A mensagem entra
nesta decisão, não numa REQ de acompanhamento.

## Consequências

**A correção é de segurança, e por isso é sequencial e sozinha.** Não paralelizar com nenhum outro
ML. E **colide** com a branch `fix/validate-detecta-hook-de-guard-na-forma-relativa-antiga`, que
mexe nos mesmos arquivos: esta wave espera aquela fechar.

**Barreira com `hades-tf` é obrigatória.** É afrouxamento de um predicado de detecção: o risco
inverso — passar a classificar como ancorado algo que não é — precisa de falsificação explícita.

**Não fecha o ponto cego da fiação.** Remover a entrada `PreToolUse` do `settings.json` continua
indetectado (`REQ-2026-09-02-remover-a-entrada-pretooluse-...`). São defeitos distintos no mesmo
controle: um classifica errado o que existe, o outro não olha para o que falta.

## Verificação exigida de quem implementar

- Falsificação **nas duas direções**: `/opt/foo/guard.sh` sob `GOOS=windows` → classificado
  **ancorado**; um caminho genuinamente relativo (`scripts/guard.sh`) → **continua** classe 2.
- 🔴 **Controle de não-afrouxamento:** enumerar o que passou a ser aceito como ancorado e mostrar que
  **nenhum caminho relativo entrou no conjunto**. É o risco desta mudança.
- 🔴 **Controle POSIX:** em Linux e macOS a classificação de **todos** os casos existentes é
  idêntica à de hoje.
- Os 3 runtimes medidos separadamente.
