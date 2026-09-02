---
status: Accepted
date: 2026-09-02
author: "zeus-tf"
---

# ADR: O `doctor` ganha modalidade remota opcional, e ausência de credencial vira "não avaliado" — nunca aprovação

> Date: 2026-09-02 | Status: Accepted

## Contexto

A AC6 da `REQ-2026-09-01-o-repositorio-do-trackfw-nao-esta-sob-os-cuidados-do-trackfw` pede que o
`trackfw doctor` **acuse** as lacunas que encontramos: `required_status_checks` ausente,
`enforce_admins` desligado, `core.hooksPath` neutralizado.

Isso transforma o achado em **produto**: as demais waves consertaram **um** repositório; esta faz
qualquer projeto que adote o trackfw ganhar o mesmo diagnóstico.

**Mas o `doctor` de hoje é inteiramente local.** `runDoctor` (`internal/commands/doctor.go:74`) lê
catálogo, manager, identidade e scaffold — **tudo em disco**. Verificar branch protection exige
**API do GitHub: rede e token**.

O `hades-tf` nomeou isso na Wave 0:

> *"Não é 'adicionar mais um check ao doctor que já existe' — é dar ao `doctor` uma segunda
> modalidade de verificação (rede + autenticação) que ele nunca teve."*

## O problema real não é técnico

Uma verificação que depende de rede e credencial tem **três** resultados possíveis, não dois:

| resultado | significado |
|---|---|
| **ok** | verificado, e está correto |
| **finding** | verificado, e está errado |
| **não avaliado** | **não deu para verificar** — offline, sem token, sem permissão, rate limit |

**Colapsar o terceiro em "ok" é o defeito que esta sessão inteira perseguiu.** Nove instâncias, sempre
a mesma forma: *mecanismo dá sinal verde enquanto o controle está inerte*. Um `doctor` que roda sem
token e reporta limpo diria **exatamente a mentira mais cara possível** — "seu repositório está
protegido" quando ele não olhou.

E colapsar em "finding" é o erro simétrico: transformaria trabalho offline legítimo em alarme falso
recorrente, e alarme que sempre dispara é alarme que se aprende a ignorar.

## Decisão

**1. A modalidade remota é opcional e explícita.** O `doctor` continua funcionando offline, rápido e
sem credencial — que é o que o torna usável. A verificação remota é ativada por flag.

**2. Ausência de credencial ou de rede produz um resultado próprio — "não avaliado" — distinto de
aprovação e de falha.** E ele **nomeia o remédio**, como fizemos no `not_evaluated` do `barrier`.

**3. Reusar o vocabulário que já existe.** O `barrier` resolveu este mesmo problema nos 3 CLIs com
`not_evaluated`, distinto de `passed`/`blocked`. **O conceito passa a ter um nome só no projeto**, já
revisado — em vez de dois nomes para a mesma ideia.

## Consequências

- O `doctor` ganha um caminho de código que **só executa com rede**, e que ninguém consegue exercitar
  em CI offline. **É superfície nova, e o custo é real.**
- Projetos sem GitHub — GitLab, Gitea, local — recebem "não avaliado", **não** "reprovado". A
  verificação é específica de forja, e fingir universalidade seria falso.
- Um token com permissão insuficiente é caso **distinto** de token ausente, e a mensagem tem de
  separá-los: um se resolve dando escopo, o outro criando credencial.

## Alternativas descartadas

**Sempre verificar, exigindo token.** Quebraria o uso offline, que é a razão de o `doctor` ser rodado
com frequência. Um diagnóstico que exige preparo deixa de ser rodado.

**Comando separado (`trackfw audit-repo`).** Divide o diagnóstico em dois lugares e faz o usuário
lembrar de rodar o segundo — quem esquece é justamente quem mais precisa. O `doctor` é o ponto único
de "está tudo certo?".

**Reportar "ok" quando não dá para verificar.** É o defeito, não a alternativa. Está aqui **nomeado
de propósito**, porque é o caminho que um implementador toma sem perceber.

## Rastreamento

REQ: `REQ-2026-09-01-o-repositorio-do-trackfw-nao-esta-sob-os-cuidados-do-trackfw.md` (AC6)
