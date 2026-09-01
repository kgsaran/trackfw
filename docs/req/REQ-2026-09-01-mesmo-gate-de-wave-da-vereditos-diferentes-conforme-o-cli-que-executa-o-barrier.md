---
status: Open
date: 2026-09-01
author: "zeus-tf"
adr: "docs/adr/ADR-2026-09-01-gate-de-wave-e-contrato-portavel-em-shell-posix-nao-script-do-sistema-operacional.md"
roadmap: "docs/roadmaps/wip/ROADMAP-2026-09-01-os-3-clis-executam-gate-de-wave-com-sh-c.md"
---

# REQ: Mesmo gate de wave dá vereditos diferentes conforme o CLI que executa o `barrier`

> Date: 2026-09-01 | Status: Open

## Motivation

**Item 7 do issue #216.** É o defeito mais grave da lista, e o único que **quebra a correção da
própria ferramenta de governança** — os outros dez são de artefato ou de plataforma.

| runtime | shell efetivo |
|---|---|
| Go (`barrier.go:729`) | `sh` POSIX, em qualquer SO |
| Node (`barrier.js:561`) | shell do SO — `cmd.exe` no Windows |
| Python (`barrier.py:582`) | shell do SO — `cmd.exe` no Windows |

**`trackfw barrier` pode aprovar uma wave para quem usa um CLI e reprová-la para quem usa outro**, no
mesmo repositório e no mesmo commit. Um gate é critério de aceite; um critério que muda de resposta
conforme quem pergunta não é critério.

## O reenquadramento que a medição forçou

Varri os **83 comandos** de `Gates da wave:` de todos os roadmaps do projeto:

```
35  grep/sed/awk     14  test / [      8  negação com !
 3  && / ||           3  $( )          2  pipe |
```

**Nenhum existe no `cmd.exe`.** Então no Windows o Node e o Python **não avaliam o gate de forma
diferente — falham em avaliá-lo**. O Go é o único que executa o que os roadmaps contêm.

Isso inverte a leitura "2 contra 1": **o divergente é o correto**, e a decisão está registrada no
ADR ligado.

## Acceptance Criteria

- [ ] **AC1** — `barrier.js` e `barrier.py` executam gate com `sh -c`, deixando de usar o shell do SO.
- [ ] **AC2** — 🔴 **Falsificação nas duas direções.** (a) um gate com idioma POSIX (`! grep -q`,
      `test -f`, `$( )`) produz **o mesmo veredito nos 3 CLIs**; (b) **controle:** um gate que
      **deve** reprovar continua reprovando nos três — não basta uniformizar para "passa".
- [ ] **AC3** — 🔴 **`sh` ausente falha nomeando o remédio.** Um usuário Windows sem shell POSIX tem
      de ler *"instale um shell POSIX"*, não `exec: "sh": executable file not found`. **Mensagem
      byte-idêntica nos 3 CLIs** — é o contrato de paridade de mensagem que o projeto já pratica.
- [ ] **AC4** — 🔴 **Distinguir "gate reprovou" de "gate não pôde ser avaliado".** Hoje ambos viram
      falha. Se o `sh` não existe, o resultado **não é** *"a wave não passou"* — é *"não deu para
      medir"*. Confundir os dois é a mesma classe de falha que atravessou esta REQ inteira: **tratar
      ausência de medição como medição negativa.**
- [ ] **AC5** — Gate falsificável que impeça regressão para `shell: true` / `shell=True` nos dois
      CLIs. **Nasce ligado ao `Makefile`, com guarda de vacuidade ancorada no mesmo cwd, `python3`
      nunca `python`** — contrato em `docs/cli-parity.md`.
- [ ] **AC6** — O contrato do ADR escrito em `docs/cli-parity.md`.
- [ ] **AC7** — 🔴 **O item 7 sai de `REPRODUCED` na camada 2** (de 4 para 3), com a transição
      explicada e o run citado. **Verificar o que o check mede antes de fixar o número** — o check
      compara o veredito do mesmo gate nos 3 runtimes, que **é** comportamento de produto, então deve
      genuinamente virar.
- [ ] **AC8** — `make quality` e **CI** verdes.

## Negative Scope

- **Não** editar gate de roadmap existente. Eles já são POSIX — é a premissa da decisão, e alterá-los
  invalidaria a evidência.
- **Não** implementar detecção de SO nem tradução de sintaxe. O ADR descarta explicitamente.
- **Não** tratar os itens 4, 8, 9 e 11 da issue.

## Linked ADR

ADR: `docs/adr/ADR-2026-09-01-gate-de-wave-e-contrato-portavel-em-shell-posix-nao-script-do-sistema-operacional.md`

## Linked Roadmap

Roadmap: `docs/roadmaps/wip/ROADMAP-2026-09-01-os-3-clis-executam-gate-de-wave-com-sh-c.md`
