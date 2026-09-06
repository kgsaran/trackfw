# TOCTOU — verificar pelo nome e usar pelo nome são **duas** resoluções

> 2026-09-06 · `trackfw_architect` (Zeus) · nasceu de uma pergunta do usuário: *"o que é o TOCTOU que
> você sempre usa?"* — eu tinha usado a sigla várias vezes sem nunca defini-la.

## O que é

**TOCTOU = Time-Of-Check to Time-Of-Use** — "hora da verificação até a hora do uso".

É uma corrida: você **verifica** uma propriedade de algo e depois **usa** esse algo. Entre as duas
operações existe uma janela em que ele pode mudar. Você verificou um objeto e usou **outro**.

## O caso concreto deste repositório (ML-1C, guard de config)

A forma ingênua de proteger contra FIFO seria:

```
1. stat(".claude/settings.json")   → "é arquivo regular, pode ler"
2. open(".claude/settings.json")   → lê
```

🔴 **O caminho é um NOME, não o objeto.** Os passos 1 e 2 fazem **duas resoluções independentes** do
mesmo nome. Trocar o arquivo por um FIFO entre eles faz o passo 1 aprovar um objeto e o passo 2 abrir
outro — e o `trackfw validate` **trava indefinidamente**, que era o defeito medido pela barreira.

## O remédio: inverter a ordem, verificar o DESCRITOR

```go
fd := open(path, O_NONBLOCK)   // abre PRIMEIRO
fstat(fd)                      // verifica o DESCRITOR, não o caminho
```

`internal/validator/regularfile_unix.go`.

Com a ordem invertida existe **uma única resolução**. O `fstat` age sobre o objeto que o kernel já
resolveu; o nome pode mudar depois, o descritor continua preso ao mesmo objeto. **Imune no POSIX.**

## 🔴 O que NÃO é imune, e por que isso ficou escrito

`regularfile_windows.go` faz `stat` e depois `open` — a janela **diminui, não fecha**.

Isso está declarado como **"raciocinado a partir da API, não medido"**, porque toda a medição da
sessão foi em macOS. *(Nota: em 2026-09-06 passamos a ter VM Windows ARM64 acessível por SSH —
esse é um dos primeiros candidatos a virar medição.)*

**Por que a distinção importa mais que a correção:** um guard de segurança que **afirma** imunidade
quando só reduziu a janela é **pior** que um que declara o limite — porque alguém vai confiar nele
exatamente no caso que ele não cobre. Declarar o limite é o que permite decidir; afirmar demais é o
que produz o silêncio falso que esta campanha inteira vem caçando.

## Onde isto reaparece

Toda vez que o código **verifica um caminho e depois o usa**: leitura de config, checagem de
permissão antes de escrever, guarda de contenção que resolve o pai imediato antes de criar.
A `REQ-2026-08-31-guarda-de-folha-faz-lstat-so-no-ultimo-componente...` tem a mesma família de
problema, com o agravante de resolver **só o pai imediato** e não o ancestral existente mais próximo.

## Regra de comunicação (pedido do usuário, 2026-09-06)

> *"Importante: sempre que você usar um acrônimo ou sigla pela primeira vez, explicar o que ele quer
> dizer."*

Vale para TOCTOU, mas também para **ML** (microlote), **AC** (critério de aceite), **ENOTDIR**,
**TTY**, **UNC**, **BOM**, **FIFO**, **fail-open/fail-closed**. Sigla não explicada transfere ao
leitor o custo de decifrar — e num relatório de decisão isso é o mesmo que não comunicar.
