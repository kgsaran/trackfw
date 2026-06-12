# REQ: REQ-Driven ADR Discovery

> Criado em: 2026-06-12 | Status: Backlog | Agente: Zeus (Arquiteto)

## Solicitação

Quando um usuário cria uma nova REQ via `trackfw req new`, o wizard deve conduzir uma entrevista contextual que detecta automaticamente domínios técnicos na intenção descrita e faz perguntas-chave para descobrir decisões arquiteturais pendentes. Cada decisão não-resolvida gera um rascunho de ADR (status `Draft`) vinculado à REQ. Um roadmap só pode ser criado para uma REQ quando todos os ADRs vinculados estão com status `Accepted`.

## Motivação

O fluxo atual (`ADR → REQ`) pressupõe maturidade arquitetural do usuário: ele precisa saber quais decisões existem *antes* de especificar um requisito. Usuários menos experientes criam REQs sem identificar as decisões arquiteturais latentes, gerando débito técnico e inconsistências de governança.

O novo fluxo (`REQ → guided ADR discovery`) guia qualquer usuário — técnico ou não — pelas decisões relevantes para o seu contexto, sem remover o caminho avulso para quem já tem maturidade.

## Escopo

### Wizard `trackfw req new` (nova Etapa 2 — probes contextuais)

1. Usuário descreve a intenção: "Tela de login para a aplicação"
2. Sistema detecta domínios por palavras-chave (autenticação, UI, banco, API, deploy, eventos)
3. Para cada domínio detectado, exibe perguntas de múltipla escolha
4. Opções com valor "ainda não decidido" → gera ADR Draft vinculado
5. Resumo final lista: REQ criada + ADRs gerados

### Catálogo de probes (domínios iniciais)

| Domínio | Keywords | ADRs possíveis |
|---|---|---|
| autenticação | login, auth, senha, sso, jwt, session, token | authentication-strategy, sso-provider, session-management |
| interface | tela, ui, frontend, componente, design, layout | ui-framework, design-system |
| persistência | banco, database, tabela, migração, modelo | database-engine, migration-strategy |
| api | endpoint, api, rest, grpc, graphql | api-protocol, api-versioning |
| deploy | cloud, container, kubernetes, docker, deploy | cloud-provider, container-strategy |
| eventos | kafka, fila, notificação, evento, pub/sub | event-broker |

### Modelo de dados — REQ

O frontmatter da REQ ganha campo `depends_on_adrs`:

```markdown
> Date: 2026-06-12 | Status: Open | Blocked by ADRs: ADR-authentication-strategy, ADR-ui-framework
```

### Mudanças em `trackfw validate`

Nova regra: REQ com `Status: Open` que tenha ADRs vinculados com `Status: Draft` → violação bloqueante com mensagem clara.

### Mudanças em `trackfw status`

Nova seção: "REQs bloqueadas por ADRs Draft" listando REQ + ADRs pendentes.

## Restrições

- Compatibilidade retroativa: REQs existentes sem ADRs vinculados não são afetadas
- Probes são opcionais — usuário pode pular ("nenhuma das opções acima") sem bloquear a criação da REQ
- `trackfw adr new` continua funcionando de forma avulsa (sem alterações)
- Detecção de domínio é por palavras-chave em português E inglês
- Keywords são case-insensitive

## Critérios de aceite

- [ ] `trackfw req new "tela de login"` → detecta domínios autenticação + UI → exibe probes
- [ ] Resposta "ainda não decidido" gera ADR Draft com título derivado da probe
- [ ] REQ gerada contém linha "Blocked by ADRs: ..." quando há ADRs Draft vinculados
- [ ] `trackfw validate` retorna violação para REQ Open com ADR Draft vinculado
- [ ] `trackfw status` exibe seção de REQs bloqueadas por ADRs Draft
- [ ] Usuário que pula todas as probes → REQ criada normalmente, sem ADRs vinculados
- [ ] `go test ./...` verde
- [ ] `go build ./...` sem erros
