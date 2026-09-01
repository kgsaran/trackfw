---
status: Open
date: 2026-09-01
author: "zeus-tf"
adr: ""
roadmap: ""
---

# REQ: CLI Node usa `chmodSync` no caminho em vez de `fchmodSync` no descritor, e reabre TOCTOU na escrita atômica

> Date: 2026-09-01 | Status: Open

## Motivation

Achado do `hades-tf` na Wave 0 da `REQ-2026-09-01-os-fchmod-...` — **fora do escopo original daquela
REQ, e mais grave que o defeito que ela investigava.**

Enquanto o Python **quebra barulhentamente** no Windows (`AttributeError` em `os.fchmod`), o **Node
funciona em todo lugar e silenciosamente não tem a garantia**:

| ponto | hoje | deveria |
|---|---|---|
| `npm/src/thirdparty/quarantine.js:28-30` | `chmodSync(path, mode)` | `fchmodSync(fd, mode)` |
| `npm/src/integrations/manager.js:94-97` | `chmodSync(path, mode)` | `fchmodSync(fd, mode)` |
| `npm/src/integrations/manager.js` (2ª chamada) | `chmodSync` **depois do `rename`** | não deveria existir |

**`fs.fchmodSync` existe no Node** — confirmado. Não é limitação de plataforma; é escolha de
primitiva.

`chmodSync(path)` opera no **caminho**, reabrindo a janela entre a criação do temporário e a
aplicação da permissão. E a segunda chamada, **após o `rename`**, é uma janela extra que o próprio
`npm/src/identity/config.js` do Node **não tem** — ou seja, a divergência existe **dentro do mesmo runtime**.

O `manager.js:362` usa o mesmo literal `0o644` do Python, então é o mesmo site sensível: o modo pedido
**difere** do padrão do temporário, e a permissão realmente muda.

## Por que isto não entrou na REQ do `os.fchmod`

Aquela REQ é sobre **Windows quebrando**. Esta é sobre **POSIX sem garantia**, num runtime que nunca
quebrou. Misturar faria a correção do Node herdar a verificação em Windows de lá, e faria a REQ de lá
crescer para um defeito que não é o dela.

**A consequência imediata:** a `AC6` daquela REQ ia escrever em `docs/cli-parity.md` que *"os 3
runtimes preservam a garantia de descritor"*. **Seria falso.** Contrato falso é pior que contrato
ausente — compra confiança que não existe. Aquela AC foi corrigida para nomear a exceção do Node e
apontar para esta REQ.

## Acceptance Criteria

- [ ] **AC1** — `fchmodSync(fd, mode)` no lugar de `chmodSync(path, mode)` nos dois pontos, com o
      descritor do temporário.
- [ ] **AC2** — A segunda chamada de `chmod` **após o `rename`** em `manager.js` é removida ou
      justificada por escrito. Se existe por um motivo, o motivo tem de estar no código.
- [ ] **AC3** — 🔴 **Controle no site certo, não na instrumentação.** O mesmo erro foi cometido e
      corrigido na REQ irmã: um teste que verifica *"`fchmodSync` foi chamado"* passa **vacuamente**
      onde o modo pedido coincide com o padrão do temporário. Mire o site de **`0o644`**
      (`manager.js:362`) e verifique o **resultado observável** — o modo final do arquivo.
- [ ] **AC4** — Falsificação da janela: PoC que demonstre a corrupção via `chmodSync(path)` e sua
      ausência via `fchmodSync(fd)`. O `hades-tf` já fez o equivalente em Python; replicar em Node.
- [ ] **AC5** — 🔴 **O remédio já existe dentro do próprio Node — adotem-no.**

      ⚠️ **A primeira redação desta AC estava apoiada em premissa falsa**, e a correção veio da
      barreira. Eu escrevi que *"`identity.js` não tem a segunda janela"*, citando um arquivo que
      **não existe**. O real é `npm/src/identity/config.js`, e ele não é apenas "menos ruim":

      ```javascript
      // npm/src/identity/config.js:77
      const fd = fs.openSync(temporaryName, 'w', mode)
      ```

      **O modo é aplicado na criação — zero janela**, nem a do `chmod` no descritor. É **mais forte
      que o `fchmod` do Python e que o `Chmod` do Go**.

      Logo esta REQ **não** é *"adote `fchmodSync`"*. É: **`quarantine.js` e `manager.js` adotam a
      forma que o `identity/config.js` já usa.** Remédio mais forte, com precedente dentro do
      próprio runtime — não é padrão importado de outra linguagem.

      **Como eu errei, e vale registrar:** grepei por `writeFileSync` e `chmod` — os **sintomas** —
      em vez de procurar a **capacidade** (escrever arquivo). `openSync` não bate com nenhum dos
      dois, então concluí que o módulo não escrevia. É a mesma falha que fez duas enumerações minhas
      errarem por uma ordem de grandeza nesta sessão, cometida por mim ao *verificar* uma ressalva.
- [ ] **AC6** — Depois desta REQ, o contrato de `docs/cli-parity.md` pode ser escrito **sem exceção**.

## Negative Scope

- **Não** tratar o `os.fchmod` do Python — é a REQ irmã.
- **Não** mexer no Go: usa `temporary.Chmod(mode)`, baseado em descritor, e já está correto.
- **Não** eliminar a janela do `os.replace`/`rename` em si — é residual declarado e comum aos três.

## Linked ADR

ADR: <!-- nenhum. Correção de primitiva; a decisão de usar descritor já está implícita no Go e no
Python. -->

## Linked Roadmap

Roadmap:
