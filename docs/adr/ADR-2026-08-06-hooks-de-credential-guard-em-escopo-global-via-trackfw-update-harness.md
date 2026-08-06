---
status: Proposed
date: 2026-08-06
author: "kg.saran@gmail.com"
---

# ADR: hooks de credential-guard em escopo global via trackfw update harness

> Date: 2026-08-06 | Status: Proposed

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
- Modo `credential_guard.mode` (warn/block) não tem uma fonte de config clara em escopo global até a
  Wave 1 do roadmap resolver isso — risco de o modo global sempre cair no default `warn` sem uma forma
  simples do usuário configurar `block` globalmente.
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
