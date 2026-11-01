# Fixture com origin em caminho local e `followRemoteHEAD never` exercita a cadeia sem a forge

**Data:** 2026-10-31
**Origem:** #633 (PR #640), sugestão do `apolo-tf` no ML-1A

## Fato

`resolveDefaultBranchWithSource` (`internal/commands/branch_prune_forge.go`) resolve a branch padrão na
ordem forge (`gh api`) → `refs/remotes/origin/HEAD` local → `git ls-remote --symref origin HEAD` →
`"main"`. Para testar as fontes depois da forge sem rede nem `gh`:

1. o `origin` da fixture é um **caminho local** (`git clone <dir>` ou `remote add origin <dir>`): a forge
   não reconhece o remoto e não responde, então a cadeia passa adiante;
2. `git config remote.origin.followRemoteHEAD never` impede que o `git fetch` do git ≥ 2.48 crie o
   `origin/HEAD` local, reproduzindo o git antigo do issue (o `symbolic-ref` falha e o `ls-remote` é
   consultado).

Sem o item 2, o teste passa no git novo pela fonte errada (symref local) e não prova nada sobre o
`ls-remote`. Exemplo: `TestBranchNew_DryRunFetchesMasterDefault` (`branch_new_base_e2e_test.go`).

## Armadilha

Com origin em caminho local, a forge **nunca** é exercitada. Teste que precisa afirmar a precedência da
forge tem de injetar o executor da forge (fake), não depender da fixture.
