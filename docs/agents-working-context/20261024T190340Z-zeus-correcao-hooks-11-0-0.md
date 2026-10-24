# zeus — correção: a remoção do guard pelo update estava errada (2026-10-24)

- **Corrige a entrada `*-zeus-hooks-11-0-0.md`,** que dizia que a remoção das entradas Bash de credential/git-branch estava certa por causa da D14. Não estava: os settings do projeto são versionados, o CI não tem o global, e o `validate` acusou 9 `guard_wiring_removed` (rc=1, local e CI). Eu medi com `grep | head` e não olhei o rc.
- **Mudou:** os três settings voltaram ao estado da main, mais o bloco do `write-scope`. `trackfw validate` rc=0.
- **Pendente:** a #614, defeito do `update` (D14 contra o `guard_wiring_removed`). Pela Regra Dura, ela volta para a REQ do #599. A decisão sobre uma 11.0.1 é do KG.
