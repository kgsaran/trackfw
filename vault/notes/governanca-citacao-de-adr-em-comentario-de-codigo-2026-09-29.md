# Citação de ADR dentro de comentário de código não é decisão

> Domínio: governança · Data: 2026-09-29 · Origem: #451, REQ-2026-09-02

## O sintoma

Uma correção não avançava porque *"a coexistência dos dois workflows está decidida na
`ADR-2026-08-28`"*. A frase estava no roadmap, e o roadmap a herdara de um comentário de código.

## A causa raiz

A decisão **não existe**. Medido:

```
$ grep -rl 'trackfw-validate.yml' docs/adr/
docs/adr/ADR-2026-09-18-...-afirma-contencao-antes-de-escrever.md   ← só como caminho de exemplo
```

Zero ocorrências na `ADR-2026-08-28`, que decide outra coisa: template de CI pinado na versão que o
gerou, e `install.sh` honrando `TRACKFW_VERSION`.

A citação nasceu em `internal/generators/scaffold_doctor.go:333`:

```go
// trackfw-validate.yml (written by `trackfw discover --init`, InstallGates) is a
// separate artifact from trackfw-gate.yml above — both can coexist in the same
// project (ADR-2026-08-28).
```

## O mecanismo de propagação — é o que torna isto caro

```
comentário de código  →  lido como decisão  →  copiado para o roadmap  →  virou o motivo
                                                                          de não corrigir
```

Três semanas. E quem derrubou a citação foi um **consumidor externo** (`lourivalgarciajunior`, no
#451), que foi abrir a ADR. Nenhum dos nossos ciclos abriu.

🔴 O agravante: o comentário é **plausível e bem escrito**. Explica a assimetria, justifica o
tratamento condicional, cita fonte. É exatamente por ser bom que ninguém o conferiu.

## Regra operável

**Citação de ADR em comentário de código é afirmação a verificar, não decisão.** Ao encontrar uma
antes de construir sobre ela: abra o ADR e confira (`grep` pelo artefato citado, não pelo id do ADR).

Custo do check: um `grep`. Custo de não fazer: três semanas de correção travada, medido.

## Sinal de alerta

A frase *"é mudança de decisão, não de implementação"* num roadmap é o ponto onde o trabalho para.
Ela precisa apontar para uma ADR que **realmente** decida aquilo — se não aponta, o trabalho parou
por nada.

## Relacionado

- `REQ-2026-09-02-init-e-discover-geram-dois-workflows-...` (AC4-bis)
- #451 · #456
