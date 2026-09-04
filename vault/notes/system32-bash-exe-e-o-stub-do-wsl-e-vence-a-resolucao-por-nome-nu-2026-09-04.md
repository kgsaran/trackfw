---
title: "`bash` por nome nu no Windows é ambíguo — a CAUSA é o `lpApplicationName = NULL`; o stub do WSL é só um dos três desfechos"
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

## 🔴 CORREÇÃO (2026-09-04) — a assinatura não é a causa, e a primeira redação confundiu as duas

**Falsificado por `@lourivalgarciajunior` num Windows 11 real, fora do runner** (comentário no
PR #267). Naquela máquina:

```
System32\bash.exe      NAO EXISTE     <- sem WSL instalado
shutil.which("bash")   None           <- nao ha bash no %PATH% nativo
Git for Windows        C:\Program Files\Git\{bin,usr\bin}\bash.exe  (GNU bash 5.2.26)
```

**O sintoma lá NÃO é `exit 1` com `stderr` vazio — é `FileNotFoundError [WinError 2]`.**

A **causa** é a mesma: **nome nu entregue ao `CreateProcess` com `lpApplicationName = NULL`.** O stub
do WSL é **um dos desfechos**, não a causa. São três:

| ambiente | o que o nome nu produz |
|---|---|
| WSL instalado **sem distribuição** | `rc=1`, UTF-16 no `stdout` — o runner do CI |
| **sem** WSL, **sem** bash no `%PATH%` | `FileNotFoundError [WinError 2]` |
| **sem** WSL, **com** Git no `%PATH%` | funciona **por acaso**, e some quando o `%PATH%` muda |

🔴 **Por que a correção importa mais que o detalhe:** o título original vendia *"exit 1 com stderr
vazio é assinatura de processo que não é o script"*. Quem estiver na **segunda linha** da tabela lê
isso, vê uma exceção em vez de `exit 1`, e conclui *"não é o meu caso"* — **quando é o mesmo defeito
e o mesmo remédio**. **A causa é o discriminante; as assinaturas são sintomas dela.**

## Dois achados que o teste real trouxe

**1. A lista chumbada do Git for Windows não é "conveniência" — naquele ambiente é a ÚNICA fonte de
candidato.** O `shutil.which` devolve `None`, e a varredura do `%PATH%` não acharia nada. A docstring
do helper a descreve como *"conveniência, não garantia"*; onde não há bash no `%PATH%`, ela é a
garantia.

**2. 🔴 `env=` NÃO muda a resolução do nome nu no Windows** — e isto pode tornar um teste futuro
vácuo:

```
e = dict(os.environ); e["PATH"] = stub + PATH
subprocess.run(["bash","--version"], env=e)   ->  FileNotFoundError   <- o stub foi IGNORADO
os.environ["PATH"] = stub + PATH
subprocess.run(["bash","--version"])          ->  rc=1, UTF-16        <- agora vence
```

O `CreateProcess` procura no `%PATH%` do processo **pai**, não no ambiente passado ao filho. **Um
teste que tentasse forçar a ordem de resolução via `env=` estaria medindo nada e passaria verde por
vacuidade.**

**O portão de identidade foi provado independente da ordem:** com um stub sintético em **primeiro**
lugar na lista, o helper ainda resolveu para o Git for Windows. No arranjo real o portão nem é
exercitado — a lista chumbada vem antes —, então ele **forçou a ordem para medir o portão, e não o
acaso**.

**Ressalva do próprio autor:** o stub é **sintético** — reproduz a *assinatura*, não o binário da
Microsoft. Fecha o comportamento do portão diante daquela assinatura; **não** substitui o run de
Windows para provar que o `System32\bash.exe` real perde.

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
