---
name: parser-de-roadmap-mede-o-corpus
description: mudança em parser de roadmap (máscara, status, critério, gate, Files affected) exige medição main×branch sobre todo docs/roadmaps antes de auditar como concluída
metadata:
  type: feedback
---

Mudança em leitor de roadmap (`roadmapdoc`, `parseMLScopeFilesAffected`) só é auditada depois de medir
o corpus inteiro de `docs/roadmaps/**` com o código da `main` e o da branch, divergência por divergência.

**Why:** no #641 (2026-10-31), a máscara de comentário HTML passou em 4 rodadas de testes unitários e
contra-braços, e o barrier da Wave 3 reprovou no PRÓPRIO roadmap: um `` `<!--` `` citado em código inline
abriu "comentário" e escondeu o resto do documento. Fixtures sintéticas não tinham crase; o corpus real tinha.

**How to apply:** no handoff de qualquer ML que mude parsing de roadmap, exigir a medição de corpus como
critério de aceite (com cada divergência explicada) e rodar o barrier do próprio roadmap com o binário da
branch antes de marcar ✅. Ver [[medir-com-a-regra-nao-com-grep]].
