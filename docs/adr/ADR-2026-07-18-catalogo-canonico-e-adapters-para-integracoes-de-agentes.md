---
status: Accepted
date: 2026-07-18
author: "Codex"
---

# ADR: Catálogo canônico e adapters para integrações de agentes

> Date: 2026-07-18 | Status: Accepted

## Context

O trackfw possui instaladores independentes e divergentes para ferramentas de IA.
O binário Go concentra templates completos para Claude, Gemini, Cursor, Copilot,
Windsurf e Amazon Q; Node.js oferece uma instalação parcial; Python expõe apenas
Codex. Os comandos `agents` e `skills` não possuem lifecycle (`list`, `install`,
`uninstall`, `update`) e não existe ownership que diferencie um arquivo gerenciado
pelo trackfw de uma customização do usuário.

Copiar o mesmo Markdown para todos os destinos não oferece compatibilidade nativa:
cada CLI possui paths, escopos, frontmatter, comandos e modelos de agente/skill
próprios. Antigravity e Kiro também precisam entrar na matriz suportada.

## Decision

1. Manter um catálogo canônico versionado com dois tipos de item:
   - `agents`: 10 especialidades (`architect`, `backend`, `frontend`, `qa`,
     `infra`, `security`, `code-quality`, `dba`, `ux`, `data`);
   - `skills`: 5 workflows de governança (`governance`, `plan`, `implement`,
     `review`, `release`).
2. Implementar adapters orientados a dados para `claude`, `codex`, `gemini`,
   `antigravity`, `cursor`, `copilot`, `windsurf`, `amazonq` e `kiro`. Cada target
   contém uma ou mais `surfaces` (IDE, CLI, current ou legacy), e cada surface
   define capacidades, paths, escopo, extensão, frontmatter, artefatos auxiliares
   e nível de suporte `native`, `fallback`, `legacy` ou `unsupported`.
3. Expor o mesmo contrato nos CLIs Go, Node.js e Python:
   - `trackfw agents|skills list [--target ...] [--json]`;
   - `trackfw agents|skills install [--targets ...] [--items ...] [--scope ...]`;
   - `trackfw agents|skills uninstall [...]`;
   - `trackfw agents|skills update [...]`.
4. Em TTY, `install`, `uninstall` e `update` apresentam seleção interativa. Em
   execução não interativa, usam flags determinísticas e falham com mensagem
   acionável quando faltar seleção.
5. Registrar ownership, versão e SHA-256 dos arquivos em manifesto trackfw por
   escopo. O estado de lifecycle será `not-installed`, `current`, `outdated` ou
   `modified`, combinado ao nível de suporte da surface. Um mesmo artefato físico
   pode possuir claims de múltiplos consumers; removê-lo de um target não apaga o
   arquivo enquanto outro claim permanecer ativo.
6. `update` sobrescreve somente arquivos cujo ownership seja comprovado e que não
   estejam modificados; `--force` será exigido para substituir customizações.
   `uninstall` remove somente arquivos pertencentes ao trackfw e limpa diretórios
   vazios, nunca arquivos do usuário.
7. Os assets Go serão a fonte canônica. Um sincronizador determinístico produzirá
   as cópias empacotadas por npm e PyPI, com paridade por hash verificada em CI.
8. Os comandos standalone antigos (`gemini`, `cursor`, `copilot`, `windsurf`,
   `amazonq`) permanecerão como aliases de compatibilidade e delegarão ao novo
   motor, com aviso de depreciação.

## Consequences

- Go, npm e PyPI passam a oferecer o mesmo contrato e os mesmos conteúdos.
- Novas CLIs podem ser adicionadas por adapter sem duplicar o lifecycle.
- Instalações existentes precisam de migração: o primeiro `list/update` adota
  arquivos legados somente quando path e hash correspondem a templates conhecidos.
- A matriz de testes e o volume de package-data aumentam.
- Algumas ferramentas não possuem conceito nativo de subagente; nesses casos o
  adapter materializa a especialidade no mecanismo nativo mais próximo e declara
  essa representação no resultado de `list --json`.
- Antigravity e Kiro exigem surfaces distintas para contratos atuais e legados;
  Amazon Q permanece suportado com indicação de migração para Kiro.

## Alternatives Considered

- **Manter instaladores separados:** rejeitado porque perpetua divergência entre
  runtimes e impossibilita lifecycle uniforme.
- **Usar um único Markdown em todos os CLIs:** rejeitado por ignorar contratos,
  escopos e frontmatter específicos.
- **Sobrescrever sempre no update:** rejeitado por risco de perda de customizações.
- **Suportar apenas Go e orientar cópias manuais:** rejeitado por violar o contrato
  de paridade dos três pacotes distribuídos.
