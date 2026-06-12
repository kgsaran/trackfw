---
name: i18n-wizard-java-scaffold
description: Suporte multilingual, correções do wizard init e scaffold Java
metadata:
  type: project
status: backlog
---

# REQ: Melhorias v1.1.0 — i18n, Wizard init e Scaffold Java

> Criado em: 2026-06-12 | Status: ⬜ Backlog

## Motivação

Feedback do usuário após validação em ambiente Windows corporativo (primeiro uso real em produção):

1. **i18n**: O CLI é usado em equipes pt-BR, en-US e es-ES. Todas as mensagens, templates de artefatos (ADR, REQ, ROADMAP) e arquivos gerados (CLAUDE.md) devem respeitar o idioma do sistema operacional. Detecção automática via variáveis de ambiente (`LANG`, `LC_ALL`, `LANGUAGE`), fallback para `en-US`.

2. **Wizard init — projeto backend-only**: Quando o tipo de projeto é `backend` ou `governance`, o wizard não deve perguntar sobre frameworks de UI (React, Vue, Angular). Atualmente pergunta para todos os tipos.

3. **Wizard init — seleção de linguagem antes do framework**: Ao selecionar backend, a linguagem (Go, Java, Node.js, Python) deve ser selecionada primeiro; em seguida o wizard pergunta qual framework usar para a linguagem escolhida.

4. **Scaffold Java — pom.xml**: Ao selecionar Java como linguagem de backend, gerar um `pom.xml` básico com Spring Boot parent, dependências padrão (web, actuator, test) e plugin do Maven.

## Critérios de aceite

- [ ] `trackfw init` em sistema com `LANG=pt_BR.UTF-8` exibe todos os textos em português
- [ ] `trackfw init` em sistema com `LANG=en_US.UTF-8` exibe em inglês
- [ ] `trackfw init` em sistema com `LANG=es_ES.UTF-8` exibe em espanhol
- [ ] Templates de ADR, REQ e ROADMAP gerados no idioma detectado
- [ ] CLAUDE.md gerado no idioma detectado
- [ ] Projeto tipo `backend` NÃO pergunta sobre framework de UI
- [ ] Projeto tipo `backend` pergunta linguagem → depois framework da linguagem
- [ ] Projeto Java com `trackfw init` gera `pom.xml` válido com Spring Boot
- [ ] Paridade entre binário Go e pacote npm para todos os itens acima
