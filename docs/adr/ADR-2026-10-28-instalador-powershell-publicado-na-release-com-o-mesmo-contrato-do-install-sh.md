---
status: Accepted
date: 2026-10-28
author: "zeus (trackfw_architect), decisão do KG"
---

# ADR: Instalador PowerShell publicado na release com o mesmo contrato do install.sh

> Date: 2026-10-28 | Status: Accepted

## Context

A auditoria de documentação de 2026-10-28 mediu as rotas de instalação do trackfw 11.1.0 no
Windows:

| rota | estado |
|---|---|
| npm (`@trackfw-bin/win32-x64`, `win32-arm64`) | publicado; instalação a partir do registro medida em VM ARM64; x64 real não verificado |
| pip (wheels `win_amd64`, `win_arm64`) | publicado; ARM64 medido; x64 não verificado |
| `install.sh` | funciona em Git Bash (medido ARM64); MSYS2/Cygwin não medidos |
| PowerShell puro | **não existe rota** |

Quem não tem Node, Python nem Git Bash não tem como instalar sem extrair o `.tar.gz` à mão. Como
a direção do projeto é consolidação + adoção (Discussion #525, 2026-10-28), essa lacuna é atrito
de entrada justamente para o público que a v8 quis atender: o usuário de ambiente corporativo, que
quase sempre tem PowerShell.

O `install.sh` já tem um contrato endurecido e falsificado por três gates:

- `check-install-checksum.sh`: checksum SHA-256 obrigatório, conferido antes de extrair. Os casos
  ausente, duplicado e divergente abortam sem deixar binário.
- `check-install-version-pin.sh`: `TRACKFW_VERSION` validada no buffer inteiro, nunca por linha.
- `TRACKFW_ARCH` validado por lista fechada; aviso de outras cópias no PATH (#592).

## Decision

1. **`scripts/install.ps1` é publicado como asset da release**, ao lado do `install.sh`, pelo mesmo
   `extra_files` do `.goreleaser.yaml`. A instrução documentada é
   `irm https://github.com/kgsaran/trackfw/releases/latest/download/install.ps1 | iex`, mais a forma
   "baixe, leia, execute" para quem não aceita pipe para `iex`.
2. **Mesmo contrato do `install.sh`, cláusula por cláusula:**
   - checksum SHA-256 do `checksums.txt` da mesma tag, conferido antes de extrair; ausente,
     duplicado ou divergente → aborta sem deixar binário no destino;
   - `TRACKFW_VERSION` validada por regex ancorada no valor inteiro (`\A…\z`, nunca `^…$` com
     multiline);
   - `TRACKFW_ARCH` restrito a `amd64|arm64`, com detecção automática quando ausente;
   - `TRACKFW_INSTALL_DIR` sobrepõe o destino;
   - aviso de outras cópias de `trackfw.exe` no PATH, sem executá-las.
3. **Compatível com Windows PowerShell 5.1 e PowerShell 7.** Sem dependência de módulo externo.
   A detecção de arquitetura não confia só em `PROCESSOR_ARCHITECTURE`: um processo x64 emulado
   em ARM64 reporta `AMD64`. Nesse caso, consulta também `PROCESSOR_ARCHITEW6432` e a arquitetura
   do SO.
4. **Formato do arquivo inalterado:** o Windows continua recebendo `.tar.gz`. A extração usa o
   `tar.exe` nativo, presente desde o Windows 10 1803. Sem `tar.exe`, o instalador falha com
   mensagem que orienta para npm/pip. Trocar para `.zip` mexeria no `install.sh` e nas pipelines
   de npm/PyPI, sem ganho que justifique.
5. **Destino padrão `$env:LOCALAPPDATA\Programs\trackfw`**, por usuário e sem privilégio de
   administrador.
6. **O instalador acrescenta o destino ao PATH do usuário** (`HKCU\Environment`), somente se
   ainda não estiver lá. A gravação é feita direto no registro, preservando o tipo do valor:
   `REG_EXPAND_SZ` continua `ExpandString`. *Corrigido em 2026-10-28 pela Wave 0
   (`docs/seguranca/2026-10-28-wave0-docs-e-install-ps1.md` §2): a versão anterior desta
   decisão citava `[Environment]::SetEnvironmentVariable(..., 'User')`, que grava `REG_SZ` e
   perde a expansão de `%USERPROFILE%` e afins.* Fica
   desligado com `TRACKFW_NO_MODIFY_PATH=1`. Ele nunca toca o PATH de máquina e nunca reordena
   entradas existentes. Isto **diverge** do `install.sh`, que só imprime a instrução: no
   PowerShell, editar o PATH à mão é o atrito que esta ADR existe para remover.
7. **Evidência:**
   - gate permanente no CI `windows-latest` (x64), rodando o instalador sob `powershell` (5.1) e
     `pwsh` (7), com os mesmos cenários do `check-install-checksum.sh` (íntegro, adulterado,
     ausente, duplicado), a injeção de versão e o PATH;
   - medição em VM ARM64 antes do merge.
   - A documentação declara como "não verificado" o que nenhuma das duas medir.

## Consequences

- **Positivas:** existe rota de instalação sem Node, Python ou Git Bash, com a mesma garantia de
  integridade das outras.
- **Negativas:**
  - Mais um script para manter em contrato com o `install.sh`. Divergência entre os dois é
    defeito, e o gate precisa falsificar os dois braços.
  - `irm | iex` tem a mesma postura de confiança do `curl | sh`: quem controla a URL controla a
    execução. O checksum protege o binário, não o próprio script.
  - Alterar o PATH do usuário é efeito persistente fora do diretório de instalação. Ele precisa
    ser idempotente e reversível, com a reversão documentada.
- **Residual:** um `trackfw.exe` antigo que vem antes no PATH (por exemplo, o `~/bin` do Git Bash)
  continua respondendo no lugar do novo. O instalador avisa, mas não remove.

## Alternatives Considered

- **Só documentar a extração manual do `.tar.gz`:** rejeitado. Mantém o atrito que a direção de
  adoção quer remover.
- **winget / Scoop / Chocolatey:** adiado. Cada um é um canal novo com manifesto próprio e
  revisão de terceiro; o `install.ps1` é pré-requisito barato e não impede os três depois.
- **Assets `.zip` para Windows:** rejeitado pelo item 4.
- **Não alterar o PATH (paridade estrita com o `install.sh`):** rejeitado pelo item 6. O usuário
  que quiser esse comportamento usa `TRACKFW_NO_MODIFY_PATH=1`.
