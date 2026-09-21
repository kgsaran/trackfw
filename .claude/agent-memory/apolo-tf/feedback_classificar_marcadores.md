---
name: classificar-marcadores-write-containment
description: Como classificar marcadores write-containment-allowed: verificar se o guard recebe o caminho exato do primitivo de escrita, lendo RejectSymlinks antes de vereditar
metadata:
  type: feedback
---

Antes de vereditar marcadores `write-containment-allowed`, sempre ler a implementação de `pathguard.RejectSymlinks`:
- Ela começa com `current = filename` — ou seja, verifica a FOLHA (o arquivo em si) na primeira iteração, depois sobe até root
- Uma folha ausente NÃO é erro (caso de criação de arquivo novo)
- Implicação: `rejectHarnessSymlink(home, filePath)` onde `filePath` é o arquivo PROTEGE a folha. Não é gap (B).

**Why:** O advisor detectou que a classificação inteira de 48 marcadores (A) dependia de um fato não verificado sobre se RejectSymlinks checa ou não a folha. Se começasse em `filepath.Dir(filename)`, TODOS seriam (B). Vale 10 minutos para ler o fonte.

**How to apply:** Em qualquer ML de classificação de marcadores, ler `internal/pathguard/pathguard.go` antes de qualquer veredito.
