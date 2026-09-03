---
status: Open
date: 2026-09-03
author: "trackfw_architect (Zeus)"
adr: ""
roadmap: ""
---

# REQ: 73 das 246 falhas de Windows não medem nada, e contaminam qualquer estimativa

> Date: 2026-09-03 | Status: Open
| Linear Issue:
| Jira Issue:

## Motivation

Triagem completa do run `33742756936` (`windows-full-suites`), 2026-09-03:

```
Go      86 testes de topo (105 blocos com subtests), 7 pacotes
Node    56
Python  104
        246 falhas — nao ~160, como o arquiteto estimou lendo 20 linhas de log
```

**73 dessas 246 não são defeitos — são bloqueios de medição.** Nelas o código de produto **nunca
executou**:

**A — 17 testes Go: o binário de teste não tem `.exe`.** `filepath.Join(dir,"trackfw")` +
`exec.Command`; no Windows o `LookPath` exige extensão conhecida. O arquivo existe e não é
executável. `internal/commands/barrier_contract_test.go:63`, `barrier_test.go`, `root_test.go:178`.

**B — ~56 testes Python: `bash` devolve exit 1 uniforme**, com stderr vazio, **inclusive no caso que
deveria sair 0 na segunda linha** (`[ -f trackfw.yaml ] || exit 0`). `test_credential_guard.py`,
`test_git_branch_guard.py`, `test_credential_guard_sabotage.py`, `test_git_branch_guard_dedup.py`.
🔴 **Mecanismo não resolvido.**

### Por que isto vem antes de tudo

Qualquer estimativa feita sobre o run atual está contaminada: **30% das falhas não dizem nada sobre
o produto.** Corrigir defeito com base nessa contagem é priorizar por ruído.

A ordem correta é **desmascarar → re-rodar → re-contar**, e só então dimensionar. É o mesmo princípio
que a sprint inteira aplicou aos gates vácuos: *um veredito que não depende do estado real do alvo
não é medição.*

### O discriminante que a triagem já achou para B

**O Node roda o mesmo script pelo mesmo `bash`, com a mesma chamada `spawnSync('bash',[script])`, e
passa** — `credential_guard.test.js` reporta `22 passed, 2 failed` internamente, e os 2 são bit de
execução. **Isso mata a teoria ambiental e localiza o defeito no lado Python.**

Suspeitos **não verificados**, a medir sem presumir: `HOME` de sessão herdado pelo processo filho, e
tradução de newline por `text=True`.

🔴 **Não inventar mecanismo para o maior grupo.** Foi a recusa da triagem em fazer isso que tornou
este diagnóstico confiável.

## Acceptance Criteria

- [ ] **AC1** — Os 17 testes Go passam a **executar o binário** no Windows. Falsificação: com o
      remendo revertido, os 17 voltam a falhar pelo mesmo motivo.
- [ ] **AC2** — 🔴 O mecanismo de **B está identificado e escrito**, com a medição que o sustenta.
      **"Não sei ainda" é resultado válido**; **hipótese apresentada como causa, não.**
- [ ] **AC3** — Isolação de `HOME` deixa de ser vácua: os testes isolam **`HOME` e `%USERPROFILE%`**.
      Hoje `pypi/tests/conftest.py:30` isola só `HOME`, e o job já mediu (`ITEM 2 = REPRODUCED`) que
      no Windows **`%USERPROFILE%` vence nos 3 runtimes** — os testes veem o home sintético do job.
      Falsificação: sem a correção, o assert de home continua batendo no caminho errado.
- [ ] **AC4** — `.gitattributes` fixa `eol` para os goldens versionados, removendo a metade
      **fixture** do grupo CRLF. 🔴 **Só a metade fixture** — a metade de produto (parser de
      frontmatter cego a CRLF, que emitiu frontmatter duplicado em `TestRenderOpenCodeAgent`) é
      decisão de arquitetura e **não entra aqui**.
- [ ] **AC5** — O ambiente de CI para em quebrar por `git`/branch ausentes no home sintético
      (`git symbolic-ref --short HEAD exited with null`). Vive no `quality.yml`, **não nos 3 CLIs**.
- [ ] **AC6** — 🔴 **Re-rodar e re-contar.** A nova contagem por runtime é registrada e substitui a
      de hoje como base de estimativa. **Este é o entregável da REQ** — os remendos são o meio.
- [ ] **AC7** — 🔴 **Guarda contra o autoengano:** nenhum teste pode ser marcado `skip` para
      "desmascarar". Um teste pulado não mede mais que um teste que não executa — trocaria um
      bloqueio visível por um invisível. Toda supressão exige mensagem nomeando a garantia não
      exercitada.

## Negative Scope

- 🔴 **Não corrigir defeito de produto nesta REQ.** Ela é de instrumento. As causas de produto —
  separador, CRLF no parser, `IsAbs` de caminho POSIX, `agent_models` — têm ADR e REQ próprias.
  Misturar impediria saber qual contagem mudou por quê.
- **Não** decidir "o trackfw escreve separador POSIX nos artefatos que autora?" aqui. É ADR, e ela
  resolve três grupos de uma vez.
- **Não** tocar em `validator_credential_guard.go` nem `validator_git_branch_guard.go`: o grupo **G**
  (caminho POSIX ancorado classificado como relativo) é **de segurança** e **colide com a branch
  `fix/validate-detecta-hook-de-guard-na-forma-relativa-antiga`**.
- **Não** relitigar o bit de execução em NTFS: já decidido em
  `vault/notes/goos-guard-e-do-binario-nao-do-host-wsl-continua-protegido-2026-09-01`.
- **Não** prometer "gates de Windows verdes" como entregável. O entregável é **uma contagem
  confiável**.

## Linked ADR
<!-- Sem decisão de arquitetura: é correção de harness de teste e de ambiente de CI. As decisões
     (separador, CRLF no parser, IsAbs) são ADRs próprias, deliberadamente fora desta REQ. -->
ADR:

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap:
