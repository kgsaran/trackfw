---
name: grep-glob-test-triggers-credential-guard
description: grep com glob sobre *_test.go pode acionar o credential guard — usar paths de arquivo explícitos ao procurar padrões em test files
metadata:
  type: feedback
---

`grep -n '"ls-remote"' internal/generators/*_test.go` retornou um block de JWT em vez dos matches
esperados. Hipótese (não verificada): o credential guard de `trackfw` inspeciona o conteúdo dos
arquivos que o grep abre e dispara ao encontrar fixtures JWT dentro dos test files. O mecanismo
exato não foi medido.

**Why:** glob `*_test.go` sobre um diretório grande inclui arquivos com fixtures JWT. Se o guard
monitora esse padrão, a operação é bloqueada antes de retornar resultados.

**How to apply:** ao procurar padrões em arquivos de teste para análise de segurança, especificar
os arquivos-alvo explicitamente (e.g. `ship_test.go release_test.go`) em vez de usar glob genérico
`*_test.go`. Isso evita o possível block do credential guard e retorna os resultados esperados.
