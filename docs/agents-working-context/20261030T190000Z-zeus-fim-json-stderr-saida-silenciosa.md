# zeus — fim: stream --json limpo depois do documento (REQ-2026-09-07, #290) (2026-10-30)

- **Mudou:** `ExitCodeError` + `decideExit` no `root.go`: depois de escrever o documento, `validate --json`, `update --json` e `update harness --json` saem com o código sem imprimir (stderr 21/42/50 B → 0). Type assertion direta, fora do subtree `guard` (D7), só 1..255. Modo texto inalterado. Contrato em `docs/cli-parity.md` (`--json` stream contract). `windows-probe.yml` passou a comparar `context --format=json` e reprova rc≠0 com stdout vazio.
- **Medido:** binário em fixture strict: `validate --json` rc=1, stderr 0 B; mutações `errors.As` e sem D7 reprovam `TestJSONExit_DecideExit` (repetidas pelo arquiteto numa cópia). Barriers das Waves 0–3 passed; a da Wave 3 inclui `make quality` rc=0.
- **Decidido:** D-1 (a): diagnósticos do cálculo (pathguard, `agent_models`) seguem no stderr; P7 (contagem por grep) fora. Erro antes do documento é outra categoria.
- **Erro meu:** aprovei no ML-1A um teste que só afirmava a semântica do Go (F3); o red team pegou e a Wave 3 corrigiu.
- **Pendente:** PR citando (#290); após o merge, roadmap para `done` e REQ `Done`. Observação fora do escopo: `update harness` padrão emite 20 linhas de aviso de `agent_models` no stderr.
