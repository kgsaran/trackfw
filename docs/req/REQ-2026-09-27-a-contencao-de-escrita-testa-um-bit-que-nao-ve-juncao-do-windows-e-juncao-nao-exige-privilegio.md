---
status: Open
date: 2026-09-27
author: ""
adr: ""
roadmap: "docs/roadmaps/wip/ROADMAP-2026-09-27-a-contencao-de-escrita-testa-um-bit-que-nao-ve-juncao-do-windows-e-juncao-nao-exige-privilegio.md"
---

# REQ: a contencao de escrita testa um bit que nao ve juncao do Windows, e juncao nao exige privilegio

> Date: 2026-09-27 | Status: Open
| Linear Issue: 
| Jira Issue: 

## Motivation

Origem: **issue #444**, de consumidor externo, com medição de **seis braços e controle**.

A contenção de escrita entregue pelo **#441** recusa **symlink**. No Windows ela **não vê junção**
(`mklink /J`) — e a assimetria de privilégio é o que torna isto alcançável:

```
$ cmd /c "mklink /D link_dir alvo"
Você não tem privilégios suficientes para realizar esta operação.

$ cmd /c "mklink /J link_j alvo"
Junção criada para link_j <<===>> alvo
```

🔴 **A isca que a guarda recusa é a que o atacante não consegue criar; a que ele consegue, ela deixa
passar.** Sem Developer Mode, sem admin.

### O mecanismo, no fonte

`internal/pathguard/pathguard.go:88` decide por **um bit**:

```go
if err == nil && info.Mode()&os.ModeSymlink != 0 {
    return fmt.Errorf("refusing symlink path %q", current)
}
```

E o `Beneath` concorda que está tudo certo, porque **a string** `projeto/docs/adr` está mesmo sob
`projeto`. **A travessia acontece no sistema de arquivos, não no caminho.**

### O que o reportante provou, e que impede a leitura fácil

O braço **C2** da medição dele mostra a guarda **funcionando**: com `adr_dirs: ../vitima/adr` — escape
**visível no caminho** — o binário da `main` recusa com `rc=1`. Mesma árvore, mesmo comando: **recusa
quando o escape está no caminho, passa quando está atrás da junção**. A diferença não é a guarda
existir.

🔴 **Não é regressão do #441.** Aquele PR fechou o escape que mediu, e fechou — o C2 prova. Este é um
sítio que a medição dele não alcançou, porque o instrumento rodava onde symlink é symlink.

### Medição do arquiteto na VM de Windows — 2026-09-27

O reportante declarou explicitamente que **não** estava propondo a correção, porque o remédio óbvio
(recusar também `ModeIrregular`) tem custo na direção que o **AC13 da `REQ-2026-09-09`** nomeia:
*"falso-positivo aqui **paralisa**, não irrita"*.

Fui medir qual é esse custo. Dez objetos, `os.Lstat` em Go na VM:

| objeto | `ModeSymlink` | `ModeIrregular` |
|---|---|---|
| **junção** (`mklink /J`) | false | 🔴 **true** |
| symlink de diretório | true | false |
| diretório comum | false | false |
| arquivo comum | false | false |
| raiz do perfil (`C:\Users\Lab`) | false | false |
| `Documents` · `Desktop` · `Downloads` | false | false |
| `AppData` · `AppData\Local` | false | false |
| raiz do **OneDrive** | false | false |
| o próprio repositório e `repo/docs` | false | false |

**Só a junção acende.** Nos dez objetos medidos, `ModeIrregular` discrimina exatamente o caso do
defeito e não toca em nada legítimo.

⚠️ **A lacuna que NÃO consegui fechar, e declaro:** o OneDrive daquela VM está **vazio** — não havia
arquivo *cloud-only* (placeholder) para medir. Placeholders do OneDrive são reparse points, e são o
candidato mais forte a falso-positivo real. **A Wave 0 tem de fechar isso**; sem essa medição, a
decisão de recusar `ModeIrregular` está apoiada numa população incompleta.

## Acceptance Criteria

- [ ] **AC1 — A população de falso-positivo é medida, não presumida.** Em particular: arquivo
      *cloud-only* do OneDrive, Dev Drive e pasta de perfil redirecionada. 🔴 Se não for possível
      medir algum, isso fica **declarado**, não presumido em nenhuma das duas direções
- [ ] **AC2 — Decisão escrita sobre o predicado**: recusar `ModeIrregular`, recusar por
      *reparse point* específico, ou outra via — com o custo medido de cada uma
- [ ] **AC3 — O braço da junção passa a REPROVAR.** O teste que o #455 deixou com `t.Logf`
      (documentando o estado atual) vira expectativa
- [ ] **AC4 — 🔴 Contra-braço obrigatório:** nenhum dos objetos legítimos medidos passa a ser
      recusado. `make quality` e a suíte de Windows verdes **na VM e no CI**
- [ ] **AC5 — O braço C2 continua recusando.** A correção não pode afrouxar o escape visível que o
      #441 já fecha
- [ ] **AC6 — O que a guarda NÃO cobre fica declarado** no contrato, como o gate de CRLF passou a
      fazer. Limitação declarada é honesta; implícita é o defeito que esta REQ corrige

## Negative scope — o que esta REQ NÃO faz

- **Não** mede a frequência de junção dentro de projetos. O reportante foi explícito em não
  transformar "é criável sem privilégio" em estimativa de frequência, e eu também não vou.
- **Não** reabre o **#441**. Ele fechou o que mediu, e o braço C2 prova que continua fechado.
- **Não** trata os demais defeitos de Windows (#308, #421) — mecanismo diferente.

## Linked ADR
<!-- Reference the ADR that governs this requirement -->
ADR: 

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
<!-- Reference the roadmap that implements this requirement -->
Roadmap: docs/roadmaps/wip/ROADMAP-2026-09-27-a-contencao-de-escrita-testa-um-bit-que-nao-ve-juncao-do-windows-e-juncao-nao-exige-privilegio.md
