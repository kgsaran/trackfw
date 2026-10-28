---
status: Open
date: 2026-10-28
author: "zeus (trackfw_architect)"
adr: "docs/adr/ADR-2026-10-28-instalador-powershell-publicado-na-release-com-o-mesmo-contrato-do-install-sh.md"
roadmap: "docs/roadmaps/wip/ROADMAP-2026-10-28-documentacao-e-canais-refletem-o-produto-11-x-com-suporte-real-a-windows-e-instalador-powershell.md"
---

# REQ: Documentação e canais refletem o produto 11.x, com suporte real a Windows e instalador PowerShell

> Date: 2026-10-28 | Status: Open
| Linear Issue: 
| Jira Issue: 

## Motivation

A auditoria só de leitura de 2026-10-28 comparou a documentação de usuário com o código 11.1.0 e
achou 33 divergências. As de severidade alta levam o usuário a errar ao seguir a instrução:

1. **`README.md:96` → `brew install kgsaran/tap/trackfw` falha.** O tap é
   `kgsaran/homebrew-trackfw` (`.goreleaser.yaml:59`); `kgsaran/homebrew-tap` dá 404. O README sai
   igual na página do npm.
2. **A página do PyPI está vazia** (`description` de 0 caracteres na 11.1.0):
   `pypi/scripts/build_wheel.py` não grava o README no METADATA. O passo "Copia README para o
   pacote PyPI" (`release.yml:308`) não alimenta nada.
3. **A linha de hook do `README.md:220-225` não tem o `emulate -R sh`.** Quem a copia recria o
   fail-open sob zsh corrigido no #595 (10.1.0).
4. 🔴 **O `VISION.md:204-205` manda instalar a partir do namespace `trackfw` no GitHub**
   (`brew install trackfw/tap/trackfw`, `go install github.com/trackfw/trackfw/...`). Esse
   namespace **não está registrado**: `gh api users/trackfw` → 404. Qualquer um pode registrá-lo e
   servir código no lugar do nosso. É risco de cadeia de suprimento, não só documentação velha.
5. O site VitePress publicado (`site/`, `deploy-docs.yml`) parou em 17/08/2026, antes da v8: não
   traz pip, Windows, guard nem atenção.

**No Windows**, falta instalação sem Node, Python ou Git Bash, e a documentação não diz o que foi
medido e o que não foi. Pontos não verificados: pwsh 7 no guard, instalação x64 a partir do
registro, e cmd.exe com a linha PS/POSIX, que sai 1 e deixa passar.

O `docs/cli-parity.md` ainda descreve paridade entre três runtimes. Esta REQ **absorve** o
`ROADMAP-2026-10-18-reescrita-do-cli-parity-md-para-o-contrato-de-binario-unico-pos-v8` do
backlog: a causa é a mesma, documentação anterior à v8 (Regra Dura de Causa Raiz).

Direção que motiva a REQ: consolidação + adoção (Discussion #525, decisão do KG em 2026-10-28).

## Acceptance Criteria

- [ ] AC1 — Nenhuma documentação de usuário (README, CONTRIBUTING, VISION, cli-parity,
  gate-design-principles, site, cópias de npm/pypi) aponta para namespace que não é nosso:
  `git grep -nE 'github\.com/trackfw/|trackfw/tap|kgsaran/tap'` restrito a essas superfícies
  devolve vazio. Todo comando de instalação documentado foi executado ou conferido contra o
  registro.
- [ ] AC2 — A linha de hook documentada no README é byte a byte a linha atual de
  `internal/guardforms/guard_generations.go`, e um teste falha se as duas divergirem.
- [ ] AC3 — A página do PyPI de uma wheel construída por `build_wheel.py` traz o README:
  `Description-Content-Type: text/markdown` e corpo não vazio no METADATA, afirmado por gate.
  `pypi/pyproject.toml` morto e as cópias commitadas `npm/README.md` / `pypi/README.md` removidas
  ou geradas, sem cópia divergente versionada.
- [ ] AC4 — `scripts/install.ps1` existe, é publicado como asset da release e cumpre o contrato da
  ADR-2026-10-28. Um gate no CI `windows-latest` roda sob `powershell` (5.1) e `pwsh` (7) os
  cenários íntegro, adulterado, ausente, duplicado, versão injetada, arquitetura inválida, PATH
  idempotente e `TRACKFW_NO_MODIFY_PATH`. Cada cenário tem contra-braço.
- [ ] AC5 — A linha de hook PS/POSIX é medida sob `pwsh` 7 e `powershell` 5.1 no CI Windows x64,
  nos casos allow, deny, erro e ausente. O resultado entra na matriz de suporte.
- [ ] AC6 — O README tem uma seção de suporte a Windows com a matriz real: rota × arquitetura ×
  shell, cada célula **medido (fonte)** ou **não verificado**, mais as limitações vigentes. Nenhuma
  afirmação de suporte sem fonte.
- [ ] AC7 — `docs/cli-parity.md` sem afirmação de paridade entre runtimes no presente; nenhum
  contrato de comportamento perdido; todos os leitores mecânicos do arquivo continuam verdes; a
  tabela de comandos bate com `internal/commands/root.go`; os aliases inexistentes saem.
- [ ] AC8 — `VISION.md` e site (pt e en) descrevem a 11.x: um binário em três canais, instalação
  por Homebrew, npm, pip, `install.sh`, `install.ps1` e Go, com Go ≥ 1.25, e a direção
  consolidação + adoção. A tabela de comandos do README e a do site batem com o código.
- [ ] AC9 — Instalação do `install.ps1` medida em VM Windows 11 ARM64 sob PowerShell 5.1 antes do
  merge.
- [ ] AC10 — `make quality` verde com a máquina ociosa; `trackfw validate` verde.

## Negative scope

- Não cria canal novo (winget, Scoop, Chocolatey): adiado na ADR.
- Não muda o formato dos assets (`.tar.gz` continua).
- Não altera o comportamento do binário Go, exceto o necessário para um teste fixar a linha de
  hook documentada.
- Não reescreve docs datados (`docs/seguranca/`, `docs/qualidade/`, `docs/portabilidade/`, ADRs,
  REQs e roadmaps): são registro histórico.
- Não corrige a linha PS/POSIX sob cmd.exe: é documentada como limitação. Corrigir exige ADR
  própria, porque a causa é outra.
- Não corrige o #624.
- **Fora do repositório e sob responsabilidade do KG:** registrar defensivamente a organização
  `trackfw` no GitHub. A REQ remove as referências; o registro fecha a janela para quem copiou a
  instrução antiga.

## Linked ADR
ADR: docs/adr/ADR-2026-10-28-instalador-powershell-publicado-na-release-com-o-mesmo-contrato-do-install-sh.md

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
<!-- Reference the roadmap that implements this requirement -->
Roadmap: docs/roadmaps/wip/ROADMAP-2026-10-28-documentacao-e-canais-refletem-o-produto-11-x-com-suporte-real-a-windows-e-instalador-powershell.md
