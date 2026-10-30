# zeus — início: stream --json limpo depois do documento (REQ-2026-09-07, #290) (2026-10-30)

- **Começou:** KG escolheu o roadmap do `validate`. REQ revisada (seção 2026-10-30) e roadmap reescrito e em `wip/`; branch `fix/validate-suprime-usage-json-stderr`.
- **Medido (main `7b352543`, fixture strict fora da árvore):** `validate --json` com violação → rc=1, stdout JSON, stderr `1 violation(s) found`; `barrier --json` blocked → stderr 0 B. Mesma causa lida em `update.go:85` e `update_harness.go:106`. Uma primeira medição rodou em fixture errada (`ls | tail -1` escolheu por ordem alfabética) e foi descartada.
- **Decidido:** erro tipado com código de saída reconhecido pelo `root.go` (testes executam em processo, `os.Exit` no RunE os derrubaria); erro antes do documento é outra categoria, fora do escopo.
- **Pendente:** Wave 0 (hades-tf), Wave 1 (apolo-tf), Wave 2 red team + `make quality`.
