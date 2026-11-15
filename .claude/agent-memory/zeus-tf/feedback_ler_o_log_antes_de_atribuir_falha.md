---
name: ler-o-log-antes-de-atribuir-falha
description: job de CI vermelho só tem causa depois de ler o log do próprio job; atribuí o `go` ao timeout por 4 runs e era outro teste
metadata:
  type: feedback
---

Job de CI vermelho só tem causa atribuída depois de abrir o log daquele job
(`gh api --allow-escape-sequences repos/<o>/<r>/actions/jobs/<id>/logs`, filtrar `--- FAIL`, `##[error]`, `make: ***`).

**Why:** no PR #687 (D18, 2026-11-15), atribuí o `go` vermelho ao timeout de 2m que eu tinha medido localmente.
O job reprovava havia 4 runs em `D18ContraArm/sh-u`: no Linux `sh` é dash, e `dash -u` sai 2. O corretivo de
tempo (ML-2D) não resolveria nada disso. Pior: o ML-2E introduziu um `os.Symlink` sem guarda, que só o
`make quality` pegaria, e eu não rodei o `make quality` antes do push.

**How to apply:** ver um job vermelho → log primeiro, hipótese depois. Medição local explica o que roda local; o
CI de Linux tem `sh`=dash, e o de Windows pula braços inteiros (perfil via XDG). Depois de qualquer ML que toque
teste, `make quality` antes do push. Relaciona-se com [[o-instrumento-mente]] e
[[proibir-make-quality-cega-o-executor]].
