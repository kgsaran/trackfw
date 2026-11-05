# Zeus — Wave 2 dos guards fechada, release 11.5.0 pronta para PR

- **Branch:** `fix/guards-do-primeiro-usuario-externo` · **REQ:** REQ-2026-11-05 (#662, #663)
- **Concluído:** MLs 2F a 2J auditados e commitados.
  - O guard de git extrai `$( )`, crase e `<( )` da string crua antes de dividir o comando (ML-2G).
  - `case`, aspas duplas, `\)` e comentário não fecham mais o corpo; parêntese desbalanceado bloqueia (ML-2I).
  - O red team final homologou com ressalvas (ML-2H), e a ressalva N-1 foi fechada no ML-2I.
  - Versão 11.5.0 com CHANGELOG (ML-2J).
  - `make quality` `MQ_RC=0`, barrier da wave 2 `rc=0`, `trackfw validate` `rc=0`.
- **Pendente:**
  - autorização do KG para `trackfw push` e PR com `Closes #662` e `Closes #663`;
  - após o merge: tag via `trackfw release tag`, conferência dos canais (GitHub, npm, PyPI), comentário nas issues (AC6), fechamento da governança e `trackfw branch prune --apply`;
  - depois disso, a REQ de CI (#664 opção 2, #665, porta fixa em `check-serve-address-parity.sh`), avisando na Discussion #664.
- **Residuais declarados:** shell lendo stdin, `-EncodedCommand`, expansão de variável, `python -c`/`node -e`, prefixos fora da lista, `pwsh git push`, e o guard de credencial lê só o `trackfw.yaml` do cwd.
