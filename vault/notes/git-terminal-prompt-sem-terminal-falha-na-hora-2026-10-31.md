# Sem terminal, o git falha na hora ao pedir credencial — `GIT_TERMINAL_PROMPT=0` não muda nada nesse caso

**Data:** 2026-10-31
**REQ:** `REQ-2026-08-30-debitos-de-higiene-package-lock-desatualizado-arvore-de-build-poluindo-grep-e-validate-script-sem-bit-de-execucao.md` (Motivation)

## Fato

O git pede usuário e senha abrindo **`/dev/tty`**, não lendo o stdin. Sem terminal de controle (Bash
de agente, runner de CI), o `open("/dev/tty")` falha e o git aborta na hora.

```
$ tty
not a tty
$ git -c credential.helper= ls-remote https://github.com/kgsaran/nao-existe-zz-641.git </dev/null
fatal: could not read Username for 'https://github.com': Device not configured     # 0 s
```

Com terminal, o git pergunta, e a pessoa responde.

## Consequência

A hipótese "sem credencial, o trackfw fica parado até o prazo de rede estourar" (parecer
`docs/seguranca/2026-10-30-wave0-branch-padrao-ls-remote.md`, V-2/P-5) **não se confirma** para o caso
sem terminal. A decisão do KG em 2026-10-31 ("desligar o pedido só sem terminal interativo") já é o
comportamento atual; nenhum código mudou.

## O que NÃO está coberto por esta medição

- **Credential helper com interface gráfica** (Git Credential Manager abrindo navegador): não depende de
  `/dev/tty` nem de `GIT_TERMINAL_PROMPT`.
- **SSH** (passphrase, confirmação de host key): é o `ssh` que pergunta, não o git; `GIT_TERMINAL_PROMPT`
  não o afeta (seria `-o BatchMode=yes`).

Se um travamento aparecer num desses dois casos, é outra causa: meça antes de reabrir.
