# O `git` sai 128 no Windows porque o corpus de testdata estoura MAX_PATH

> 2026-09-24 · ML-2B da REQ-2026-09-23 (censo) · medido na VM Windows 11 ARM64, Git 2.55.0.windows.3

## O que estava em aberto

O shard 1 do censo de Windows morria com `chunk_rc=128` e **nenhum diagnóstico**. A Wave 0 eliminou
quase tudo e deixou duas hipóteses vivas, explicitamente **não** apresentadas como causa:
`core.longpaths`/MAX_PATH e `safe.directory`.

## A causa, falsificada nas duas direções no MESMO comprimento de caminho

```
longpaths=false  rc=128  len_win=66    error: open("internal/roadmapdoc/testdata/corpus/docs/
                                       roadmaps/done/ROADMAP-2026-08-04-json-marshalindent-…md"):
                                       Filename too long
longpaths=true   rc=0    len_win=66
```

**65 (prefixo Windows) + 1 + 195 (caminho relativo) = 261 > 260.** O repositório tem arquivos de
corpus com caminho relativo de **196, 195 e 190** caracteres:

```
internal/roadmapdoc/testdata/corpus/docs/roadmaps/done/ROADMAP-2026-09-17-jira-base-url-do-
repositorio-vira-destino-de-post-autenticado-e-um-pr-que-edita-so-a-config-exfiltra-a-credencial-do-ci.md
```

São **nomes de roadmap usados como fixture** — o próprio estilo de nomenclatura longa deste projeto,
versionado dentro de `testdata`.

## 🔴 Três coisas que tornam isto difícil de achar

**1. A falha é marginal, e o nome do diretório temporário decide.** Num teste meu, `lp.cW227I`
(9 chars) passou e `sonda-rc128.fEHVaE` (18) falhou — **o mesmo comando, o mesmo repositório**.
É por isso que a Wave 0 mediu "240-255, abaixo de 260" e classificou a hipótese como *enfraquecida*:
a medição estava certa para o prefixo que ela usou.

**2. Não é o MSYS.** O `bash` do Git for Windows cria e escreve num caminho de **916 caracteres**
sem reclamar (`mkdir -p` + `: > f.txt`, medido). Quem recusa é o **`git` nativo**, que usa a API
Win32 com `MAX_PATH=260` enquanto `core.longpaths=false` — o padrão.

**3. A "assinatura com stderr vazio" era artefato do próprio código.** O parecer da Wave 0 registrou
`rc=128` com stderr **vazio** como a assinatura a perseguir. Falso: o `git` **escreve** duas linhas
de `error:` e uma de `fatal:`. O que as apagava era o `2>/dev/null` no sítio
(`check-gates-falsify.sh:1559,1583`). 🔴 **A correção do ML-2A — parar de engolir o stderr — teria
revelado a causa sozinha.**

## Eliminado com medição

- **`safe.directory`**: `git rev-parse --show-toplevel` funciona na cópia → não é ownership.
- **Arquitetura ARM64**: o `uname` da VM é `…ARM64 3.6.9-….x86_64 … x86_64 Msys` — o runtime MSYS é
  **x86_64 emulado**, o mesmo build do runner. A arquitetura não participa.

## A correção

`git -c core.longpaths=true` nos comandos que operam sobre a cópia — o cenário controla os próprios
comandos e não depende da configuração do ambiente. **Não** encurtar os nomes do corpus: eles são o
dado sob teste, e encurtá-los mascararia a classe de defeito que o corpus existe para exercitar.

⚠️ **Todos os sítios, não um.** Qualquer `git` do falsify que opere sobre uma cópia do repositório
dentro de `$WORK` tem o mesmo risco — a distância até os 260 depende do `mktemp` daquele dia.
