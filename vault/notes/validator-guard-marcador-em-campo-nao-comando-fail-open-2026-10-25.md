# Validador do guard: marcador em qualquer campo de string conta como fiação (fail-open e falso positivo)

> Domínio: validator / guard | REQ: REQ-2026-10-25 (#618) | Agente: zeus-tf (medição), hades-tf (Wave 0) | Data: 2026-10-25

## Código referenciado

`collectCommandsWithMarker` (`internal/validator/validator_credential_guard.go:332`) percorre o JSON recursivamente e
devolve **toda** string que contém o marcador, em qualquer chave. 14 chamadores:
- 12 recebem o arquivo inteiro (`parsed`);
- 2 recebem o array do evento (`extractCredentialGuardCommandsFromEvent`, L758 e L773).

## As duas direções, medidas no binário 11.0.1

| Direção | Fixture | Resultado |
|---|---|---|
| **Fail-open** | `.windsurf/hooks.json` com `{"command":"echo x","note":"<linha exata do guard>"}` nos 2 eventos | `validate` rc=0, nenhuma mensagem do Windsurf. O validador dá o guard como presente, e ele não roda |
| Controle | mesmo arquivo, sem o `note` | rc=1, "has no credential guard entry" |
| **Falso positivo** | `~/.claude/settings.json` com o marcador `.sh` em `autoMode.environment` | aviso de forma legada mandando rodar `update harness`, que não resolve |
| **Falso positivo** | `permissions.allow` com `Bash(trackfw guard credential …)` no projeto | violation, rc=1 |

## O que NÃO resolve

- **Restringir ao subárvore `hooks`:** fecha o caso `autoMode` e `permissions`, mas não o fail-open. O `note` está
  **dentro** do objeto do evento, e L758/L773 já passam só o evento e mesmo assim deixam passar.
- **Restringir à chave `command`:** cega o Copilot global, cujo comando fica na chave `bash` (`~/.copilot/settings.json`,
  `hooks.preToolUse[].bash`).

A correção precisa de um coletor que conheça o esquema de cada CLI: caminho do evento e chave de comando por CLI. A
tabela por CLI está em `docs/seguranca/2026-10-25-wave0-marcador-fora-de-hook.md` §1.

## Lacuna vizinha

`~/.codeium/windsurf/hooks.json` não está em `globalGuardConfigFiles`, e por isso o validador global não lê o Windsurf.
