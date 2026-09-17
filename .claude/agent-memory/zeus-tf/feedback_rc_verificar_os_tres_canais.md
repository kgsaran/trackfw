---
name: rc-verificar-os-tres-canais
description: Ao publicar um release candidate, verificar nos TRÊS canais o que passa a servir como estável — o GitHub é o que erra
metadata:
  type: feedback
---

Publicar um `-rc` não é só criar a tag: verificar, em cada canal, **o que passa a ser servido como
estável** para quem instala sem pedir versão.

**Why:** em 2026-09-16 KG apontou que o `install.sh` estava servindo `8.0.0-rc1` como latest — e
estava, havia 3 dias. npm e PyPI estavam certos sozinhos (dist-tag `rc`; PEP440 trata pré-release),
mas o GitHub não: o bloco `release:` do `.goreleaser.yaml` não declarava `prerelease`, e o default do
GoReleaser é `false`. Então **toda** tag `-rc` publica como estável e vira o `latest`. O `install.sh`
resolve por `/releases/latest`, que honra a flag — corrigir a origem resolve o consumidor.

**How to apply:** depois de qualquer tag de pré-release, medir os três, sem presumir:
`gh api repos/<r>/releases/latest --jq .tag_name` · `npm view <pkg> dist-tags` ·
`curl -s https://pypi.org/pypi/<pkg>/json | jq -r .info.version`.
O canal GitHub é o que falha por default; os outros dois falham por descuido de publicação.
Relacionado: [[push-e-pr-via-ship]].
