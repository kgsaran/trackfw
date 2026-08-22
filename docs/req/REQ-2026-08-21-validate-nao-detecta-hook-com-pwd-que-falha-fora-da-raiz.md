---
status: done
date: 2026-08-21
author: "Zeus (Arquiteto)"
adr: "docs/adr/ADR-2026-08-22-postura-do-validate-diante-de-formas-de-hook-nao-reconhecidas-classificar-por-ancoragem-nao-por-casamento-com-o-gerado.md"
roadmap: "docs/roadmaps/done/ROADMAP-2026-08-22-validate-detecta-hook-com-pwd-que-falha-fora-da-raiz.md"
---

# REQ: `validate` não detecta hook com `$PWD` que falha fora da raiz

> Date: 2026-08-21 | Status: done

## Motivação

Débito nomeado pela barreira do ML-3A
(`docs/seguranca/2026-08-21-revisao-da-deteccao-de-hook-relativo.md`, achado D.1), **confirmado por
mim por medição**:

```
.claude/settings.json com "command": "$PWD/scripts/trackfw-credential-guard.sh"
  json valido: sim · script presente na raiz
  validate acusa?  0                                   <- NAO acusa
  hook fora da raiz -> No such file or directory       <- FALHA
```

É **o mesmo defeito** que a `REQ-2026-08-17` corrigiu, por outra forma: um caminho que parece
ancorado e não é. `$PWD` expande para o **diretório corrente**, não para a raiz do repositório.

## Por que esta forma é a que mais preocupa das três

A barreira nomeou três formas não capturadas — `$PWD/...`, `$UNDEFINED/...` e valor entre aspas. As
duas últimas são improváveis em campo.

**`$PWD` é diferente: é o erro que alguém comete tentando consertar.** Um usuário que recebe a
mensagem *"references ... with a bare relative path"* e edita o hook à mão pode escrever `$PWD/`
acreditando que está ancorando na raiz — e o `validate` **passa a ficar em silêncio**, confirmando o
engano.

A correção anterior **criou** o caminho para este erro, ao ensinar que o problema era "falta de
prefixo".

## Escopo

Reconhecer como não-ancoradas as formas que o `resolveCredentialGuardHookPath` hoje descarta com
`ok=false` por não casarem com nenhum prefixo gerado pelo trackfw — em particular `$PWD/`.

**Decidir a postura**, e é a parte que precisa de julgamento:
- **Acusar** o que não casa com nenhuma forma conhecida (fecha a classe, mas pode gerar
  falso-positivo em hook legítimo customizado); ou
- **Acusar apenas uma lista de formas sabidamente quebradas** (`$PWD`, `.`, `..`) — mais estreito,
  mas é o padrão "condição estreita demais" que esta série nomeou **nove** vezes.

A primeira é a que fecha a classe. A segunda é a que não incomoda ninguém. **O ADR precisa escolher
com o motivo**, não deixar implícito.

## O que **não** é escopo

- Reabrir a decisão de qual forma cada CLI usa.
- As formas D.2 (`$UNDEFINED/`) e D.3 (aspas), a menos que a decisão acima as cubra de graça.

## 🔴 Risco dominante

**Falso-positivo em hook customizado legítimo.** Um usuário pode ter razão para apontar o guard para
outro lugar. Acusar tudo que não casa com o gerado é a opção que fecha a classe **e** a que mais
arrisca incomodar — e pelo `ADR-2026-08-17`, guard que atrapalha é guard que o usuário desliga.

## Acceptance Criteria

- [ ] AC1 — Decisão registrada em **ADR**, com a alternativa descartada e o motivo.
- [ ] AC2 — `$PWD/scripts/...` em CLI que exige prefixo é **acusado** — provado.
- [ ] AC3 — Formas legítimas de cada CLI **continuam limpas** — provado nos 6.
- [ ] AC4 — Paridade nos 3 CLIs, com gate comparando saídas reais.
- [ ] AC5 — Cenário P4 **nas duas direções** — acusar de menos e acusar de mais.
- [ ] AC6 — `make quality` verde **e CI verde**.

## Riscos para quem executar

- **A mensagem precisa explicar por que `$PWD` não ancora** — dizer só "forma inválida" repete o
  engano que a levou lá.
- **Cuidado com o instrumento:** fixture com `settings.json` malformado dá falso negativo
  indistinguível de regra que não detecta. Use heredoc e **valide o JSON** antes de confiar.
- **Binário do `PATH` desatualizado**; `--version` não distingue o build.

## Linked ADR
ADR: `docs/adr/ADR-2026-08-22-postura-do-validate-diante-de-formas-de-hook-nao-reconhecidas-classificar-por-ancoragem-nao-por-casamento-com-o-gerado.md`

## Linked Roadmap
Roadmap: `docs/roadmaps/done/ROADMAP-2026-08-22-validate-detecta-hook-com-pwd-que-falha-fora-da-raiz.md`
