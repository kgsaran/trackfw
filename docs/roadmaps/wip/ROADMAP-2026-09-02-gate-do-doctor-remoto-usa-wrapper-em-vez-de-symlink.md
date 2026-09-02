---
status: wip
date: 2026-09-02
req: "docs/req/REQ-2026-09-02-check-doctor-remote-parity-depende-de-ln-s-e-reprova-onde-deveria-dizer-que-nao-pode-medir.md"
squad: "lourivalgarciajunior"
---

# Roadmap: Gate do doctor remoto usa wrapper em vez de symlink

> Created: 2026-09-02 | Status: wip

## Context

REQ: `docs/req/REQ-2026-09-02-check-doctor-remote-parity-depende-de-ln-s-e-reprova-onde-deveria-dizer-que-nao-pode-medir.md`

O `RUNTIME_BIN` é populado com `ln -s`, que falha num Windows onde `python3` é o App Execution Alias
da Store. O gate morre antes do primeiro cenário, e reporta **reprovação** em vez de *"não pode ser
avaliado"*.

## Acceptance Criteria

- [x] `RUNTIME_BIN` populado sem `ln -s`, por wrapper com shebang absoluto
- [x] Isolamento preservado e **verificado**, não presumido
- [x] Falsificação nas duas direções
- [x] Comentário registra por que não é symlink, com o alvo medido

## Wave 1 — Correção

### ML-1A — `mk_runtime_shim` em vez de `ln -s`
**Status:** ✅ Concluído
**Files affected:** `scripts/check-doctor-remote-parity.sh`

**Actions:**
1. Função que escreve um wrapper de duas linhas com shebang absoluto, em vez de symlink.
2. Comentário com a medição do alvo que falha, para ninguém "simplificar" de volta.

**Acceptance criteria:**
- [x] O gate atravessa o `ln -s` e chega ao veredito
- [x] **Isolamento verificado por efeito**, não presumido
- [x] **Controle** — o que resolvia continua resolvendo, o que não resolvia continua não resolvendo

**Evidência — o defeito:**

```
command -v python3       ->  .../WindowsApps/python3  (109 bytes -> C:/Program Files/WindowsApps/)
ln -s para /bin/ls       ->  criou
ln -s para esse python3  ->  Permission denied
```

Não é Developer Mode: o `ln -s` funciona na mesma máquina para um alvo comum.

**Evidência — isolamento preservado, medido nas duas direções:**

```
PATH=<runtimebin>  python3 --version  ->  Python 3.12.4       resolve
PATH=<runtimebin>  node --version     ->  command not found   nao vaza
PATH=<runtimebin>  gh --version       ->  command not found   nao vaza
```

A terceira linha é o controle que importa: `gh` é a razão de o isolamento existir.

**Evidência — o gate deixa de morrer:**

```
antes   morre no ln -s, 0 cenarios avaliados
depois  33 cenarios avaliados: 26 OK, 7 FAIL
```

Os 7 restantes têm **outra causa, medida e declarada no escopo negativo da REQ** — o stub de `gh`
sem extensão é invisível para o `exec.LookPath` do Go no Windows. Verificado independente deste ML,
rodando o binário Go sozinho sem wrapper nenhum.

**Gates da wave:**
```bash
# O RUNTIME_BIN e populado sem symlink.
! grep -q 'ln -s "$REAL_' scripts/check-doctor-remote-parity.sh
```
