---
status: Accepted
date: 2026-08-17
author: "Zeus (Arquiteto)"
---

# ADR: guard global cabeado, com no-op fora de projeto trackfw

> Date: 2026-08-17 | Status: Accepted

## Context

Medido em 2026-08-17 na máquina de KG, ao fechar os PRs #183/#185:

- `trackfw update harness` **escreve** `~/.trackfw/scripts/trackfw-git-branch-guard.sh`
  (`internal/generators/update.go:493`), incondicionalmente;
- **nenhum** dos 6 arquivos de config global o referencia — o `credential-guard` tem 2 refs em cada
  um dos 4 que existem, o `git-branch-guard` tem **zero**;
- portanto o script fica no disco, executável, e **nada jamais o invoca**;
- e a regra `validateGuardGlobalScriptIntegrity` só avalia configs que **referenciam** o script, de
  modo que ela nunca roda para ele. Consequência medida: o script global ficou **3 versões
  atrasado** (123 linhas contra 369) atravessando ML-1A, ML-4B e ML-4C, com `validate` verde o tempo
  todo.

O `ADR-2026-08-12-nao-ha-prevencao-contra-agente-induzido…` decidiu que a defesa precisa morar no
escopo global — *"controle que mora onde o agente escreve não é controle"*. Para o `git-branch-guard`
isso **não se concretizou**.

### O fato que decidiu a questão

Medido num repositório **sem** `trackfw.yaml`:

```
~/.trackfw/scripts/trackfw-git-branch-guard.sh  ·  'git push'  ->  exit 2
```

O script **não tem no-op fora de projeto trackfw**. Cabeá-lo como está bloquearia `git push` e
`git commit` em **todo** repositório da máquina do usuário, inclusive os que nada têm a ver com o
trackfw.

Isso transforma "cabear ou não" numa escolha entre proteção e usabilidade — falsa, como se vê
abaixo.

## Decision

**Cabear o `git-branch-guard` no escopo global, nos mesmos CLIs do `credential-guard`, e tornar o
script um no-op quando não houver `trackfw.yaml` na raiz do repositório corrente.**

A separação que resolve o dilema: **a resistência à adulteração vem de onde a configuração mora, não
de onde ela se aplica.** São eixos independentes, e o desenho anterior os confundia.

| eixo | decisão | motivo |
|---|---|---|
| **onde a config mora** | escopo global (`~/…`) | o agente que o guard vigia tem escrita no repositório; config dentro do repo é desarmável por quem ela deveria conter |
| **onde o guard atua** | só em projeto trackfw | fora de projeto trackfw não há `trackfw ship` para oferecer como alternativa; bloquear ali é custo sem contrapartida |

### Consequência aceita, declarada

Um repositório **sem** `trackfw.yaml` não é protegido pelo guard. É deliberado: o guard existe para
forçar o trilho de governança do trackfw, e onde não há trilho não há o que forçar. Quem quiser
proteção precisa de um projeto trackfw — o que é o mesmo pré-requisito de todo o resto do produto.

### O que **não** muda

- O escopo de projeto continua existindo e sendo cabeado como hoje. A fiação global **soma**, não
  substitui — se o agente apagar a de projeto, a global continua de pé.
- O conteúdo do script segue **byte-idêntico** entre os escopos e entre os 3 CLIs. O no-op é
  comportamento do script, não uma variante global.
- Nada aqui muda o veredito do `ADR-2026-08-12`: continua não havendo prevenção contra agente
  induzido. O guard segue sendo **tripwire declarado**, como o header dele já afirma desde o ML-4B.

## Consequences

**Positivas**
- O `git-branch-guard` passa a ter, de fato, a proteção de escopo global que o `ADR-2026-08-12`
  prescreve — hoje ele tem zero.
- A regra de integridade global passa a rodar para ele **de graça**, porque a condição de disparo
  dela é justamente haver config referenciando o script. O ponto cego que deixou 3 versões passarem
  fecha como efeito colateral.
- Some o artefato órfão: script escrito e nunca invocado é confusão pura para quem audita.

**Negativas / riscos**
- **Falso-positivo em escopo global afeta todos os repositórios de uma vez.** É o risco dominante da
  implementação, e a razão de o no-op vir **antes** da fiação, nunca depois.
- O no-op cria um caminho de desarme trivial: `rm trackfw.yaml`. Aceito — quem apaga o `trackfw.yaml`
  destrói a governança inteira, não só o guard; não é escalada de privilégio.
- Detecção de "estou em projeto trackfw" precisa ser barata e robusta: roda em **toda** chamada de
  ferramenta. Subir diretórios até achar `trackfw.yaml` é o comportamento esperado, mas o custo tem
  de ser medido, não presumido.

## Alternatives Considered

- **Cabear sem no-op** — rejeitada: bloquear `git push` em todo repositório da máquina tem custo de
  UX alto e um efeito perverso previsível — o usuário desliga o guard por incômodo, e guard desligado
  protege zero. Trocar proteção real por proteção nominal é o pior resultado possível.
- **Parar de escrever o script global** (`update.go:493`) — rejeitada: elimina o artefato órfão e o
  ponto cego, mas abre mão da resistência à adulteração e contradiz o `ADR-2026-08-12` no ponto
  central dele. Seria escolher a solução limpa em vez da correta.
- **Manter como está** — rejeitada explicitamente: escrever o artefato, não cabeá-lo e não verificá-lo
  é o pior dos três mundos, e é o estado que esta ADR encerra.
