---
status: Accepted
date: 2026-08-06
author: "kg.saran@gmail.com"
---

# ADR: hooks de credential-guard em escopo global via trackfw update harness

> Date: 2026-08-06 | Status: Accepted

## Context

`REQ-2026-08-06-hooks-de-credential-guard-como-escopo-global-cross-project-via-trackfw-update-harness.md`
documenta uma lacuna real do hook de guarda contra materialização de credenciais (PR #141): ele herdou
escopo por-projeto do mecanismo de attention-signal (`InjectXHooks`/`InjectHooksDetected`) sem isso
ter sido uma decisão própria para esse propósito. O risco que o hook mitiga existe em qualquer
projeto que o usuário abra com um CLI de IA com hooks, com ou sem `trackfw.yaml` — proteção
por-projeto deixa o usuário vulnerável em todo repo que ele não lembrou de inicializar.

Pesquisa de 2026-08-06 confirma que os 6 CLIs da wave nativa (Claude Code, Codex, Gemini CLI, GitHub
Copilot, Cursor, Kiro) suportam hooks em nível de usuário/global, **mesclados** (não sobrescritos)
pelo nível de projeto:

| CLI | Nível global | Fonte | Ressalva |
|---|---|---|---|
| Claude Code | `~/.claude/settings.json` | confirmado informalmente em sessão anterior | — |
| Codex | `~/.codex/hooks.json`, prioridade 2 (project é prioridade 1, ambos rodam) | `developers.openai.com/codex/config-advanced` e fontes de terceiros | **Contradição a reconciliar na Wave 2**: uma fonte diz hooks habilitados por padrão (confirmado no PR #141/ML-2B via `developers.openai.com/codex/hooks`), outra diz que exigem `[features] codex_hooks = true` explícito e são silenciosos sem a flag — investigar qual é o comportamento real atual antes de implementar |
| Gemini CLI | `~/.gemini/settings.json` | `geminicli.com/docs/hooks/`, hierarquia System→Workspace→User documentada | — |
| GitHub Copilot | `~/.copilot/settings.json` (`hooks` inline) ou arquivos em diretório de hooks de usuário | `docs.github.com/en/copilot/reference/hooks-configuration` | — |
| Cursor | `~/.cursor/hooks.json` | confirmado informalmente em sessão anterior (pesquisa original do ML-2E já citava esse nível) | — |
| Kiro | `~/.kiro/hooks/` | `kiro.dev/changelog/cli/2-13/`, GitHub issues confirmando o recurso | **Só disponível em V3 (`kiro-cli --v3`)** — versão específica, não a padrão |

## Decision

1. **Opt-in explícito, só via `trackfw update harness`.** `trackfw init`/`trackfw update`
   (escopo de projeto) **não mudam de comportamento** — continuam instalando só o wiring por-projeto
   já existente (PR #141), sem sugerir ou instalar nada global automaticamente. A instalação global é
   uma ação deliberada do usuário, rodando `trackfw update harness` (com `--targets`/
   `--install-missing` já existentes, mesmo padrão dos alvos `claude-skill`/`<tool>-agents`/
   `<tool>-skills`).
2. **Script global em caminho estável fora do repositório.** Novo gerador (nos 3 stacks) escreve
   `~/.trackfw/scripts/trackfw-credential-guard.sh` — mesmo conteúdo canônico do script já existente
   (`credentialGuardScript`/`CREDENTIAL_GUARD_SCRIPT`/`_CREDENTIAL_GUARD_SH`, criado no ML-1A do PR
   #141), sem duplicar a lógica de detecção — só o destino de escrita muda. O script já lê
   `credential_guard.mode` de `trackfw.yaml` na raiz do projeto quando invocado a partir de um
   projeto; para invocação global, avaliar na Wave 1 do roadmap se precisa de uma fonte de config
   equivalente em `~/.trackfw/config.yaml` (ou aceitar que o modo global sempre usa o default `warn`
   até esse refinamento).
3. **Novos alvos em `trackfw update harness`**, um por CLI confirmado na tabela acima (nome sugerido:
   `<tool>-credential-guard`, seguindo o padrão de nomenclatura de `<tool>-agents`/`<tool>-skills` já
   usado em `HarnessTargetIDs`) — cada um escreve o wiring no arquivo de hooks global daquele CLI,
   apontando para o script em `~/.trackfw/scripts/`.
4. **Dedup: o projeto detecta instalação global e pula o wiring local do credential-guard
   especificamente.** `InjectXHooks` (escopo de projeto) passa a checar, em modo leitura (nunca
   escrita) no arquivo de hooks global do CLI correspondente, se a entrada de credential-guard
   apontando para `~/.trackfw/scripts/trackfw-credential-guard.sh` já existe — se sim, **não**
   adiciona a entrada de credential-guard por-projeto (evita rodar 2x por comando), mas continua
   adicionando normalmente as entradas de attention-signal/cleanup (que são inerentemente
   por-projeto, não fazem sentido em escopo global). Essa é uma dependência de leitura unidirecional
   (projeto lê estado global, nunca escreve nele) — não cria acoplamento de escrita entre os dois
   geradores.
5. **Kiro**: alvo condicionado à versão v3 confirmada em runtime (ou documentado como pré-requisito
   explícito no output do comando) — não instalar silenciosamente algo que só funciona numa versão
   específica sem avisar o usuário.

6. **Modo global passa de `warn` fixo para `block` por padrão, sem novo arquivo de config**
   (emenda de 2026-08-08, `REQ-2026-08-08-credential-guard-modo-block-cobertura-read-write-e-resolucao-de-arquivo-referenciado.md`).
   Resolve a lacuna deixada aberta na decisão 2/Consequences original ("avaliar na Wave 1 ... ou
   aceitar que o modo global sempre usa o default `warn`"). Em vez de criar `~/.trackfw/config.yaml`
   (rejeitado — adiciona uma segunda fonte de config só para isto), o script global reutiliza a MESMA
   leitura de `credential_guard.mode` que `credentialGuardProjectTail` já faz a partir do
   `trackfw.yaml` do projeto — quando o hook global é disparado a partir do cwd de um projeto que tem
   `trackfw.yaml` com `credential_guard.mode` explícito (`warn` ou `block`), esse valor é respeitado
   (preserva a decisão 5 original das Consequences — "nenhuma mudança de comportamento para quem já
   definiu mode: warn explicitamente"). Em qualquer outro caso — sem `trackfw.yaml`, ou com
   `trackfw.yaml` sem a chave `credential_guard.mode` — o fallback deixa de ser `warn` e passa a ser
   `block`. Um guard de segurança opt-in (decisão 1) que nunca bloqueia por padrão é uma armadilha de
   falsa sensação de proteção; o usuário que rodou `trackfw update harness` já demonstrou intenção
   explícita de ter o mecanismo ativo.
7. **Cobertura de Read/Write/Edit por CLI, matcher por matcher** (emenda de 2026-08-08, mesma REQ).
   O wiring de credential-guard (decisão 3) cobria só o tool de shell (`Bash`/`shell`/`bash`/
   `run_shell_command`) — extração via leitura direta do arquivo (`Read`) ou materialização via
   escrita (`Write`/`Edit`) nunca passava pelo hook. Confirmado por CLI (pesquisa 2026-08-08 contra a
   documentação oficial de cada um):

   | CLI | Matcher leitura | Matcher escrita/edição | Fonte / observação |
   |---|---|---|---|
   | Claude Code | `Read` | `Write\|Edit` | nomes nativos de tool, mesmo mecanismo já usado para `Bash` |
   | Codex | **não suportado** | `apply_patch` (aliases documentados: `Edit`, `Write`) | `learn.chatgpt.com/docs/hooks`: não há tool dedicado de leitura de arquivo interceptável por hook — limitação documentada, não implementada por workaround |
   | Gemini CLI | `read_file\|read_many_files` | `write_file\|replace` | `geminicli.com/docs/reference/tools` |
   | Kiro | `read` (alias de `fs_read`) | `write` (alias de `fs_write`) | `kiro.dev/docs/hooks/types` — wildcards de categoria já documentados na decisão 3 original |
   | GitHub Copilot | `view` (`preToolUse`/`postToolUse` minúsculo) ou `Read` (variante PascalCase) | `create\|edit` (minúsculo) ou `Write\|Edit` (PascalCase) | `docs.github.com/en/copilot/reference/hooks-reference` — doc confirma mapeamento `view→Read`, `create→Write`, `edit→Edit` |
   | Cursor | `Read` | `Write` | via os eventos genéricos `preToolUse`/`postToolUse` (não `beforeShellExecution`/`afterShellExecution`, que são Bash-only) — matcher já documentado no comentário de `InjectCursorHooks` |
   | Windsurf | fora de escopo | fora de escopo | credential-guard já não cobre Bash para Windsurf hoje (`InjectWindsurfHooks` não injeta a entrada) — consistente manter fora de escopo nesta emenda, não expandir escopo não solicitado |

   Onde o CLI não expuser um matcher de leitura dedicado (Codex), a limitação é documentada
   explicitamente em `docs/cli-parity.md` e no comentário da função de wiring correspondente — não
   silenciada.
8. **Segunda camada de detecção: conteúdo de arquivo referenciado, não só o payload do comando**
   (emenda de 2026-08-08, mesma REQ). `credentialGuardDetectionCore` passa a, além de escanear
   `$INPUT` (payload cru), também escanear o conteúdo de arquivos referenciados por um redirecionamento
   já capturado por `REDIRECTS` (extensão do que já existia para `is_ephemeral_target`) **e** por
   argumentos de comando que resolvem para um caminho de arquivo regular existente, quando o comando
   é um dos inspetores comuns (`cat`, `head`, `tail`, `jq`, `grep`) — cobre o padrão do incidente
   analisado (`head -c 50 /tmp/token.txt`) sem exigir um resolvedor de dataflow completo. Guarda de
   custo: só lê arquivos até um teto de tamanho (evita ler binários grandes/logs enormes a cada tool
   call); arquivos maiores são ignorados silenciosamente por essa camada (o payload do comando em si
   continua escaneado normalmente).

## Consequences

**Positivas:**
- Fecha a lacuna real de proteção cross-project — usuário protegido em todo projeto que abrir com um
  CLI onde instalou o credential-guard globalmente, sem depender de lembrar de rodar `trackfw init`
  em cada repo.
- Comportamento de projeto (`init`/`update`) permanece 100% retrocompatível — nenhuma surpresa para
  quem já usa o trackfw hoje.
- Dedup por leitura evita duplicidade de execução sem acoplar escrita entre os geradores global e
  por-projeto.

**Negativas / riscos aceitos:**
- Opt-in significa que usuários que não sabem que `trackfw update harness` existe continuam
  desprotegidos fora dos projetos onde já rodaram `trackfw init` — a lacuna original só é fechada para
  quem descobrir e rodar o comando. Mitigação parcial: `docs/cli-parity.md`/changelog do release devem
  destacar a feature.
- ~~Modo `credential_guard.mode` (warn/block) não tem uma fonte de config clara em escopo global até a
  Wave 1 do roadmap resolver isso — risco de o modo global sempre cair no default `warn` sem uma forma
  simples do usuário configurar `block` globalmente.~~ **Resolvido pela emenda 6** (2026-08-08):
  fallback global passa a `block`, sem nova fonte de config.
- Kiro com gate de versão (v3) é uma superfície de manutenção a mais — se a Kiro promover v3 para
  padrão ou descontinuar v2, o alvo precisa de revisão.
- Codex tem uma contradição de documentação não resolvida nesta ADR (flag `codex_hooks` habilitada por
  padrão vs. exigindo opt-in) — decisão de implementação adiada para a Wave 2 do roadmap, com
  investigação própria antes de escrever qualquer wiring.

## Alternatives Considered

- **`trackfw init`/`update` sugerindo ativamente a instalação global.** Rejeitado nesta ADR: decisão
  explícita do usuário foi manter `init`/`update` sem mudança de comportamento — instalação global é
  opt-in puro. Pode ser revisitado numa REQ futura se a adoção do `update harness` for baixa.
- **Deixar o hook rodar 2x quando ambos os escopos estão presentes (sem dedup).** Rejeitado: decisão
  explícita do usuário foi priorizar detecção e skip do wiring por-projeto quando o global já existe,
  mesmo aceitando a dependência de leitura entre os dois geradores.
- **Reescrever o mecanismo de escopo de `InjectXHooks` para ser genérico (global/projeto configurável
  por qualquer hook, não só credential-guard).** Rejeitado como fora de escopo desta ADR — o
  attention-signal/cleanup é inerentemente por-projeto (o sinal pertence a um roadmap/repo
  específico) e não há demanda para generalizar isso agora; a mudança fica escopada só ao
  credential-guard.
