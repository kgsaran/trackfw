# `insteadOf` reescreve de novo quando a URL já resolvida é passada ao `ls-remote`

> Data: 2026-11-26 · REQ-2026-11-26 (bootstrap de repositório novo) · achado D3-bypass do red team (ML-2B)

## Sintoma
A checagem "o remoto está vazio" do `trackfw push` (D3) consultava um destino vazio. O push, porém, chegava a
outro destino, que já tinha branches.

## Causa
- `git remote get-url --push origin` devolve a URL **já reescrita** por `pushInsteadOf`/`insteadOf`.
- Quando essa URL é passada a `git ls-remote --heads <url>`, o git aplica `insteadOf` **outra vez**.
- O push real (`git push origin`) aplica a reescrita uma vez só.

Duas regras bastam para os dois caminhos divergirem:
- **encadeadas:** `A→X`, depois `X→Y`;
- **combinadas:** `pushInsteadOf A→B` e `insteadOf B→C`.

`ls-remote origin` (pelo nome) também não serve, porque usa a URL de fetch, não a de push.

## Correção
Em `checkRemoteEmpty` (`internal/commands/push.go`), roda-se `git ls-remote --get-url <u>` para cada URL de push.
Se o resultado for diferente de `<u>`, a checagem é recusada (fail-closed).

Custo: há um falso positivo conhecido. Com `insteadOf HTTPS→SSH` e `insteadOf git@→ssh://` juntas, a correção recusa
uma configuração legítima. A saída é remover uma das regras durante o push de bootstrap.

## Regra geral
Para medir o que uma operação git vai fazer, consulte pela **mesma resolução** que a operação usa. Nunca passe a um
segundo comando git uma URL que já foi resolvida, porque ele resolve de novo.

Precondições de qualquer teste de `insteadOf`, antes de olhar o resultado:
- `get-url --push` dá a URL esperada;
- `ls-remote --get-url` dessa URL dá o destino esperado.

Se uma regra não casar, o teste não mede nada.

Ligações: `docs/seguranca/2026-11-26-red-team-bootstrap-de-repositorio-novo.md` · [[bootstrap-repo-novo-achados-wave0-2026-11-26]]
