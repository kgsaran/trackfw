---
status: Accepted
date: 2026-08-28
author: "trackfw_architect (Zeus)"
---

# ADR: Gate de CI gerado nasce pinado na versão que o gerou, e o `install.sh` honra `TRACKFW_VERSION`

> Date: 2026-08-28 | Status: Accepted

## Context

O workflow que o `trackfw init` gera instala a ferramenta assim:

```yaml
- name: Install trackfw
  run: |
    curl -sSfL https://github.com/kgsaran/trackfw/releases/latest/download/install.sh | sh
```

Duas propriedades desse trecho, ambas medidas:

**1. Não existe forma de pinar — nem para quem quer.** `scripts/install.sh:33-44` resolve a versão
por conta própria via `api.github.com/repos/${REPO}/releases/latest`, **ignorando de qual tag o
próprio script foi baixado**. Não há variável de ambiente, argumento posicional nem flag que aceite
uma versão. Trocar a URL de `releases/latest/download/` para `releases/download/v7.3.0/` não pina:
o script baixado da tag antiga continua consultando a API e instalando a release mais recente.

**2. O gate de governança é, portanto, não reprodutível por construção.** `trackfw validate` é
bloqueante de PR. Uma release nova com regra mais estrita passa a **reprovar PRs sem que nada no
repositório tenha mudado** — e o inverso também: um PR reprovado ontem passa hoje. O artefato que
deveria dar reprodutibilidade à entrega é ele próprio o ponto não reprodutível.

Isso já cobrou preço. Em 2026-08-27, no projeto cmdb, um `trackfw update` reescreveu
`.github/workflows/trackfw-gate.yml` e apagou o pin `TRACKFW_VERSION: "7.0.0"`, o
`timeout-minutes: 10` e o comentário que explicava por que o pin existia — trazendo de volta o
`install.sh | sh` sem pin. O usuário tinha resolvido o problema **saindo do caminho suportado**:
download direto do tarball, escrito à mão, fora de qualquer template. O `update` desfez.

A leitura errada desse incidente seria "o `update` não deveria sobrescrever". Mas o
`ADR-2026-08-27` já decidiu, e mantemos, que **a propriedade dos artefatos de scaffold é dada pelo
caminho** e que customização à mão não é suportada — o `update` sobrescrever é o comportamento
desenhado. O defeito não está no `update`. Está no **asset que ele reescreve**: se o template
gerado já fosse reprodutível, não haveria motivo para editar à mão, e não haveria o que desfazer.

A mesma linha não pinada aparece em `internal/generators/scaffold.go` (GitHub Actions e GitLab),
`npm/src/generators/init.js` (7 ocorrências, incluindo textos de CLAUDE.md) e
`pypi/trackfw/generators/init_gen.py` — nos três CLIs.

## Decision

**1. `scripts/install.sh` passa a honrar a variável de ambiente `TRACKFW_VERSION`.** Quando
definida e não vazia, é a versão instalada; quando ausente ou vazia, o script resolve a release mais
recente exatamente como hoje. **O comportamento padrão não muda** — nada quebra para quem não define
a variável.

**2. O valor é validado antes de compor qualquer URL**, contra `^v?[0-9]+\.[0-9]+\.[0-9]+$`. Um
valor fora desse formato aborta a instalação com mensagem explícita. `TRACKFW_VERSION` é
interpolada na URL de download; um workflow que a alimente a partir de entrada de PR não confiável
seria, sem essa validação, um caminho de injeção na etapa de instalação. A validação é requisito de
segurança, não higiene de formato.

**3. Os templates de CI gerados nascem pinados na versão do binário que os gerou**, e ganham
`timeout-minutes: 10`. `trackfw init` ou `trackfw update` rodando em 7.3.0 escreve
`TRACKFW_VERSION: "7.3.0"`. O bump do pin passa a ser um efeito de `trackfw update` — logo, um
**diff revisável no PR**, e não uma mudança silenciosa no dia da release alheia.

**4. Nada é registrado no `integrations-manifest.json`.** O `ADR-2026-08-27` fica inteiro de pé:
propriedade pelo caminho, sem migração, sem `unregistered-write` em massa.

**5. O template de CI deixa de ser byte-estável entre releases — e isso é aceito.** É a
consequência direta da decisão 3, e está detalhada abaixo.

## Consequences

**Positivas**
- O gate de governança passa a ser reprodutível de fábrica, sem exigir que o usuário saiba do
  problema. É a diferença entre um default correto e um default corrigível.
- Some o motivo para editar o workflow à mão — que é a causa real do incidente de 2026-08-27, e a
  única forma que existia de contornar o problema.
- Atualizar a versão do gate vira um ato deliberado e auditável (`update` → diff → PR), no lugar de
  um efeito colateral da publicação de uma release nova.
- `TRACKFW_VERSION` serve a qualquer consumidor do `install.sh`, não só ao workflow gerado: Docker,
  devcontainer, provisionamento de máquina.

**Negativas e riscos aceitos**
- **O `doctor` passa a apontar `scaffold-divergent` no workflow a cada release**, em todo projeto,
  até que o `update` rode. Isso é ruído novo e recorrente. É aceito porque é literalmente a pergunta
  que o `ADR-2026-08-27` quis que o `doctor` respondesse — *"meu projeto está com os artefatos da
  versão que eu tenho instalada?"* — e aqui a resposta tem consequência real: o pin no CI está
  atrasado em relação ao binário local, o que significa que o gate local e o gate do PR rodam
  versões diferentes. O remédio impresso (`trackfw update`) resolve em um passo e converge.
- O template deixa de ser função só de `cfg` e passa a ser função de `(cfg, versão)`. Os doc-comments em
  `internal/generators/scaffold.go:1906` e `:1931` que declaram o builder cfg-independente precisam
  ser corrigidos: continuam cfg-independentes, mas não são mais version-independentes. (A redação
  original deste ADR apontava `scaffold_doctor.go:62`; a Wave 0 mediu e o comentário não está lá.) Um binário velho num projeto novo reporta
  divergência que é do binário — risco já registrado no `ADR-2026-08-27`, que esta decisão amplia.
- **O `update` do CLI Python não gerencia o alvo `ci-workflow`** (lacuna intencional e documentada em
  `pypi/trackfw/integrations/scaffold_doctor.py:25`). Projetos que só usam o CLI Python geram o
  workflow pinado no `init`, mas não recebem o bump automático depois. A lacuna não é fechada aqui;
  fica registrada como consequência conhecida.
- Um pin nunca atualizado envelhece em silêncio: o projeto fica preso a uma versão antiga do gate
  sem nada reclamar, a não ser o próprio `doctor`. Trocamos "quebra sozinho" por "congela sozinho" —
  e congelar é a falha mais segura das duas, além de ser a única visível no `doctor`.

## Alternatives Considered

**Pin vazio, opt-in** (`TRACKFW_VERSION: ""` com comentário ensinando a preencher). Mantém o
template byte-estável e não gera ruído novo no `doctor`. Rejeitada: o default continua não
reprodutível, e só quem já sabe do problema se protege. Corrige a *possibilidade* de pinar sem
corrigir o *comportamento padrão* — que é exatamente o estado em que o cmdb estava quando teve que
sair do caminho suportado.

**Fazer o `update` recusar sobrescrever workflow divergente** (com `--force` para forçar). Preserva
a customização à mão, mas contradiz o `ADR-2026-08-27` (customização de scaffold não é suportada,
propriedade é pelo caminho) e trata o sintoma: continuaria sendo necessário customizar, porque o
template continuaria errado.

**Registrar `.github/workflows/` no manifesto**, ganhando a classificação de três estados do
`doctor`. Rejeitada pelo mesmo motivo do `ADR-2026-08-27`: exige migração e faz todo projeto
existente reportar `unregistered-write` de uma vez.

**Publicar `install.sh` versionado por tag** (`releases/download/v7.3.0/install.sh`). Não funciona:
o script resolve a versão pela API de `latest` independentemente de onde foi baixado. Seria um pin
aparente — a pior categoria de controle, porque parece funcionar.
