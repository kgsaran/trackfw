---
status: Open
date: 2026-09-03
author: "trackfw_architect (Zeus)"
adr: ""
roadmap: ""
---

# REQ: As 217 falhas reais de Windows colapsam em poucas causas, e três delas exigem decisão antes de código

> Date: 2026-09-03 | Status: Open
| Linear Issue:
| Jira Issue:

## Motivation

A Wave 0 de desmascaramento removeu os bloqueios de medição. **A contagem agora significa algo**, e
foi medida no run `33810452454` da `main` — o primeiro com tudo dentro (4 PRs do reporter, a Wave 0,
e a regra de `eol`):

```
              ANTES     AGORA    delta
Go             86        64      -22
Node           56        52       -4
Python        104       101       -3
             ────      ────     ────
TOTAL         246       217      -29
```

🔴 **Eu estimei 73 desmascaradas. Foram 29.** Superestimei por um fator de **2,5** — e o ML-1C tinha
avisado: *"só ~4 falhas por runtime do grupo CRLF eram medição bloqueada; o resto é defeito de
produto"*. **As 217 restantes são defeito real.**

Node e Python quase não se moveram — **7 no total** —, o que confirma que o **grupo B** continua
inteiro.

### As causas, da triagem já feita (não rederivar)

| grupo | mecanismo | tamanho | produto ou teste? |
|---|---|---|---|
| **B** | `bash` do Python devolve exit 1 uniforme, **inclusive** no caso que deveria sair 0 | **~56** | 🔴 **mecanismo DESCONHECIDO** |
| separador | `tildeify` devolve `~\.claude\settings.json`; `provenanceKey` nativo no Node | ~45 | **produto** |
| CRLF no parser | parser de frontmatter cego a CRLF; emitiu frontmatter **duplicado** | ~14 | **produto** |
| `IsAbs` POSIX | `filepath.IsAbs("/opt/…")` é falso no Windows → guard classifica ancorado como relativo | ~14 | **produto, e é de segurança** |
| bit NTFS | `mode&0111` sempre 0 | ~22 | **teste** — já decidido no vault |
| resíduo | `WinError 32`, `.sh` sem `bash`, `stale_wip` off-by-one | ~15 | teste |

### Por que três decisões vêm antes de qualquer código

**Uma delas resolve três grupos de uma vez.** Se o trackfw escreve separador POSIX nos artefatos que
ele mesmo autora, os grupos de separador, de chave de proveniência e de caminho em JSON viram
**edição de fixture mais uma função por runtime** — e o tamanho do trabalho cai muito.

A evidência tende a **sim**: a chave de proveniência já é `/` por decisão documentada, `~\...` é
incoerente (`~` é POSIX-ismo e nenhum shell do Windows o expande), e um `command` bash com `\` é
mastigado pelo shell.

**Sem a decisão, cada agente escolhe sozinho** e a divergência entre os 3 CLIs volta por outro
caminho.

### 🔴 O grupo B é o maior risco isolado, e ninguém sabe o mecanismo

**~56 testes, 26% do total, uma única causa desconhecida.** O discriminante já foi achado pela
triagem, e ele **mata a teoria ambiental**:

> **O Node roda o mesmo script pelo mesmo `bash`, com a mesma chamada
> `spawnSync('bash',[script])`, e passa** — `credential_guard.test.js` reporta `22 passed, 2 failed`
> internamente, e os 2 são bit de execução.

O defeito está **no lado Python**. Suspeitos **não verificados**: `HOME` de sessão herdado pelo
filho, e tradução de newline por `text=True`.

🔴 **"Não sei ainda" é resultado válido. Hipótese apresentada como causa, não.** Foi a recusa da
triagem em inventar mecanismo para o maior grupo que tornou este diagnóstico confiável.

## Acceptance Criteria

- [ ] **AC1** — 🔴 O mecanismo do **grupo B** está identificado **e escrito**, com a medição que o
      sustenta. Se não for identificável nesta REQ, **vira REQ própria com o que foi eliminado** —
      não fica como suspeita.
- [ ] **AC2** — As 3 ADRs existem e estão `Accepted` **antes** do código do grupo que cada uma
      governa: separador POSIX em artefato autorado; CRLF no parser de frontmatter; caminho POSIX
      ancorado num config lido por CLI de agente.
- [ ] **AC3** — Cada grupo corrigido tem falsificação **nas duas direções** e **controle POSIX** com
      números de antes/depois. O remendo do `.exe` quebrou o POSIX na primeira tentativa — não se
      repete por acidente.
- [ ] **AC4** — 🔴 **Regra dura de paridade sem exceção:** é código de produto nos 3 CLIs. Todo ML
      lista os 3 stacks em "Files affected" **desde o início**.
- [ ] **AC5** — 🔴 **Nenhum teste marcado `skip`** para reduzir contagem. Um teste pulado não mede
      mais que um que não executa. Toda supressão exige mensagem nomeando a garantia não exercitada.
- [ ] **AC6** — 🔴 **Recontagem por wave, não só no fim.** A contagem por runtime é medida no CI
      após cada wave, e o delta é **atribuído** ao grupo corrigido. Sem isso não se sabe qual
      correção funcionou.
- [ ] **AC7** — 🔴 **Controle contra a armadilha do grupo CRLF:** nenhuma correção pode reduzir
      contagem **escondendo** defeito. O ML-1C mediu isso e **removeu** um pin que já tinha feito,
      porque apagaria a evidência. Cada ML declara o que deixou visível de propósito.
- [ ] **AC8** — `make quality` verde e os 9 `required_status_checks` verdes ao fim de cada wave.

## Negative Scope

- **Não** relitigar o bit de execução em NTFS: decidido em
  `vault/notes/goos-guard-e-do-binario-nao-do-host-wsl-continua-protegido-2026-09-01`.
- **Não** tocar o grupo `IsAbs` (segurança) na mesma wave que qualquer outro: ele **colide** com
  `fix/validate-detecta-hook-de-guard-na-forma-relativa-antiga` e é sequencial e sozinho.
- **Não** corrigir os checks da **camada 2** aqui — eles medem **réplicas dentro do harness**, não o
  produto, e são `REPRODUCED` permanentes por construção. REQ própria
  (`REQ-2026-09-01-camada-2-mede-a-plataforma-e-nao-o-produto...`), ampliada com o item 4.
- **Não** otimizar o `check-gates-falsify.sh` nem o alvo `parity` aqui — REQ própria, medição em
  curso.
- **Não** prometer "Windows verde" como entregável de uma wave. O entregável de cada wave é **um
  grupo fechado e a contagem re-medida**.

## Linked ADR
<!-- Três, a escrever antes do código de cada grupo. São do arquiteto e NÃO paralelizam. -->
ADR:

## Blocked by ADRs
<!-- As 3 acima. -->

## Linked Roadmap
Roadmap: docs/roadmaps/wip/ROADMAP-2026-09-03-fechar-os-grupos-de-falha-de-windows-por-causa-raiz.md
