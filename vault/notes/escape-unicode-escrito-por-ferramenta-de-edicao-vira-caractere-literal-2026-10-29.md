# Escape `\uXXXX` escrito pela ferramenta de edição vira o caractere, não o texto

> Data: 2026-10-29 · Domínio: PowerShell, edição por agente · Origem: ML-4E da REQ-2026-10-28 (install.ps1)

## Sintoma
O ML-4E precisava que o `scripts/install.ps1` tivesse, como **texto ASCII**, a regex
`'[;;﹔；]'`. Era a defesa contra variantes Unicode de `;` (achado A-1 da reauditoria).
O agente relatou que, quando escreveu pelas ferramentas `Edit`/`Write`, a sequência `\uXXXX` foi
gravada como o **caractere** (bytes UTF-8), não como barra + `u` + quatro dígitos. O arquivo deixa de
ser ASCII, e a regex passa a conter o caractere cru, o que muda o significado num arquivo lido como
ANSI pelo Windows PowerShell 5.1.

## Causa
Relato do agente, não reproduzido pelo arquiteto: os parâmetros das ferramentas passam por
desescape do tipo JSON, e `;` é interpretado antes de chegar ao arquivo.

## Como escrever
- Gere o trecho por script, com escrita em bytes (`python3`, modo `wb`/`rb`), ou monte o caractere em
  tempo de execução no PowerShell (`[char]0xFF1B`) em vez de pôr o escape no fonte.
- Armadilha vizinha, também relatada: dentro de string com aspas simples no PowerShell, `''\', ''/'`
  não produz `\', /`. Para âncoras de sabotagem com esses trechos, use aspas duplas e escape `$` com
  crase.

## Como conferir (medido pelo arquiteto em 2026-10-29)
```bash
LC_ALL=C grep -c '[^ -~	]' scripts/install.ps1 scripts/check-install-ps1.ps1   # 0 e 0
grep -n 'u037E' scripts/install.ps1                                          # o escape aparece como texto
```
Os dois arquivos precisam continuar ASCII, porque o Windows PowerShell 5.1 lê um `.ps1` sem BOM na
página de código ANSI.

Relacionado: [powershell-funcao-simples-engole-parametro-desconhecido-em-args-2026-10-28](powershell-funcao-simples-engole-parametro-desconhecido-em-args-2026-10-28.md)
