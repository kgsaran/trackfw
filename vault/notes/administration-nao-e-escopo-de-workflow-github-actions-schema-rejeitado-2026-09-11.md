# `administration` não é escopo de `permissions:` em GitHub Actions — schema rejeitado silenciosamente

**Data:** 2026-09-11
**Origem:** corretivo ML-4B (PR #317, branch `fix/ratchet-por-nome-e-classe-propria-para-suite-que-nao-carrega`)

## O defeito

`administration: read` foi declarado em `permissions:` de um job no `quality.yml` com a
intenção de permitir ao GITHUB_TOKEN chamar `/repos/{owner}/{repo}/branches/{branch}/protection`.

O GitHub **rejeitou o workflow inteiro no nível de schema**, criando 0 jobs — não falhou o job
individual. Resultado: todos os `required_status_checks` nunca reportam; todo PR fica pendente
para sempre. O YAML é sintaticamente válido (`yaml.safe_load` passa), mas o schema do GitHub
Actions não aceita o escopo.

## A causa

`administration` **não é** um escopo válido de `permissions:` em workflows GitHub Actions.
É um escopo de fine-grained PAT. Os escopos válidos de workflow são: `actions`,
`artifact-metadata`, `attestations`, `checks`, `contents`, `deployments`, `discussions`,
`id-token`, `issues`, `models`, `packages`, `pages`, `pull-requests`, `repository-projects`,
`security-events`, `statuses`.

Confirmado por actionlint:
```
.github/workflows/quality.yml:1069:7: unknown permission scope "administration".
all available permission scopes are "actions", "artifact-metadata", ...
```

## O modo de falha é o pior possível

GitHub valida o schema do workflow além da sintaxe YAML. Um escopo inválido em `permissions:`
não falha o job — **rejeita o workflow inteiro** antes de criar qualquer job. Isso faz todos os
checks obrigatórios (`required_status_checks`) nunca reportarem, e todo PR fica pendente para
sempre — o mesmo modo de falha `R\W` que o gate foi construído para detectar.

## O que o `yaml.safe_load` NÃO detecta

`yaml.safe_load` valida apenas sintaxe YAML, não o schema do GitHub Actions. Um workflow pode
ser YAML válido e ser rejeitado pelo GitHub. Para validação de schema:

```bash
actionlint .github/workflows/quality.yml
```

`actionlint` conhece os escopos válidos de `permissions:` e detecta esse erro antes de enviar
para o CI.

## Lição: wiring do actionlint no gate local

`make parity-rest` (e portanto `make quality`) não inclui `actionlint`. A falta desse step
foi a causa de o erro ter chegado ao CI. A solução mínima é adicionar `actionlint` ao
`parity-rest` — item registrado mas não implementado neste ML (requer REQ+roadmap próprios,
pois é mudança de scope de parity).

## Sobre a permissão de ler proteção de branch em CI

Medido em 2026-09-11:
- Chamada anônima a `/branches/main/protection` → HTTP 401 (autenticação requerida)
- Token pessoal de KG (scope `repo`) → HTTP 200 ✓
- GITHUB_TOKEN com `contents: read` em CI → **não confirmado** (não testável localmente;
  requer fine-grained PAT com `metadata: read` only ou run real de CI)

Se GITHUB_TOKEN não conseguir chamar o endpoint, as verificações D\R e R\W do gate não
ocorrem. O gate sai com exit 2 (nunca passa em silêncio). A solução para R em CI é
`secrets.REPO_ADMIN_TOKEN` (PAT com `repo` scope) — decisão de KG.

O defeito original do ML-4B (`windows-full-suites` ausente do `required_status_checks`) é
um defeito D\R e **não seria detectado** por um gate sem R.
