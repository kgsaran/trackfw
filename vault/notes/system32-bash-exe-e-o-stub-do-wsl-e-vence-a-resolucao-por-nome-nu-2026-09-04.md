---
title: "`System32\\bash.exe` é o stub do WSL, vence a resolução por nome nu, e fala UTF-16 pelo stdout"
tags: [windows, wsl, pathext, subprocess, python, ci, diagnostico, causa-raiz]
date: 2026-09-04
related: [[exit-1-com-stderr-vazio-e-assinatura-de-processo-que-nao-e-o-script-2026-09-04]]
---

## Sintoma

**50 testes Python** (23% das 217 falhas de Windows) falhavam com **`exit 1` uniforme e `stderr`
vazio** — inclusive o caso que deveria sair **0** e o que deveria sair **2**. Os dois extremos
devolviam o mesmo 1.

## Causa raiz, medida (ITEM 12 da sonda, run `33875124523`)

O `bash` **existe** no runner. O problema é **qual** deles o Windows resolve:

```
shutil_which_bash   = 'C:\Program Files\Git\bin\bash.EXE'     <- GNU bash, rc=0 no --version
bash_candidates     = [Git\bin\bash.exe, System32\bash.exe, Git\usr\bin\bash.exe, ...]

bare_rc             = 1
bare_is_gnu_bash    = False
bare_out            = "W\x00i\x00n\x00d\x00o\x00w\x00s\x00 \x00S\x00u\x00b..."
                       ^ UTF-16: "Windows Subsystem for Linux has no installed distributions."
```

**`C:\Windows\System32\bash.exe` é o stub do WSL e VENCE a resolução por nome nu.** Sem distribuição
instalada, ele sai **1** e escreve em **UTF-16 pelo `stdout`**.

## Por que cada sintoma se explica de uma vez

| sintoma | explicação |
|---|---|
| `exit 1` uniforme | é o stub do WSL, **não o script** — por isso os dois extremos dão o mesmo 1 |
| `stderr` vazio | o stub fala por **`stdout`**, o canal que os 50 testes **descartam** |
| Go e Node passam | ambos entregam **caminho absoluto** ao `CreateProcess` |
| Python falha | CPython passa `lpApplicationName = NULL` → **ordem implícita**, e `System32` vem antes de `Git\bin` |

## As duas armadilhas que quase apagaram a medição

**1. `shutil.which` teria mascarado o defeito.** Ele varre o `%PATH%` **na ordem do PATH**, e
devolve `Git\bin\bash.EXE` — o binário **certo**. Usá-lo como braço de "caminho absoluto" faria os
dois braços concordarem e concluiríamos *"não é resolução"*. A ordem do `%PATH%` **não é** a ordem do
`CreateProcess` com `lpApplicationName=NULL`.

**2. Medir só o `stderr` mede o mesmo nada que os testes medem.** A única assinatura compatível com o
observado era *"algo saiu 1 falando por `stdout`"*. Uma sonda que replicasse a captura dos testes
não veria nada — e concluiríamos que o processo é mudo.

## Regra prática

**Em Windows, `bash` por nome nu é ambíguo por padrão.** `System32\bash.exe` existe em toda
instalação recente e é o stub do WSL. Todo lançamento de `bash` a partir de teste ou ferramenta deve
usar **caminho absoluto provado** — provado por `--version` contendo `GNU bash`, não por existência
do arquivo.

E o discriminante entre "não achou" e "achou o errado" **não é o exit code** — é a **identidade** do
processo que respondeu.

## Como foi descoberto

ML-0A reduziu o espaço a duas ramificações e **recusou-se a inventar mecanismo** para o maior grupo.
ML-0B construiu a sonda com três cuidados que decidiram o resultado: imprimir o `stdout`, recusar o
`shutil.which` como braço de caminho absoluto, e travar o rótulo de "defeito de segurança" atrás de
prova de identidade `GNU bash`.

**Sem o terceiro cuidado, o stub do WSL teria sido rotulado como o guard morrendo sob invocação
legítima** — defeito de harness virando alarme de segurança.
