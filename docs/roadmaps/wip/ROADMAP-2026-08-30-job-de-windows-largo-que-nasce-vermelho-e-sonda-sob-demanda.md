---
status: wip
date: 2026-08-30
req: "docs/req/REQ-2026-08-30-ci-nao-exercita-windows-e-os-sete-defeitos-da-issue-216-sao-invisiveis-para-o-projeto.md"
squad: "hades-tf, ares-tf, artemis-tf"
---

# Roadmap: Job de Windows largo que nasce vermelho, e sonda sob demanda

> Created: 2026-08-30 | Status: wip

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Context

REQ: `REQ-2026-08-30-ci-nao-exercita-windows-e-os-sete-defeitos-da-issue-216-sao-invisiveis-para-o-projeto.md`
ADR: `ADR-2026-08-30-ci-de-windows-como-instrumento-de-medicao-job-largo-que-nasce-vermelho-mais-sonda-sob-demanda.md`

Onze defeitos de Windows conhecidos — sete da issue #216, três achados por nós, três em comentários
dela. Todos invisíveis para o CI, que roda **três invocações dirigidas** em `windows-latest`.

Esta REQ entrega o **instrumento**, não as correções. O job precisa **nascer vermelho**.

## Acceptance Criteria

Consolidado — AC1 a AC11 da REQ. **A AC2 define a REQ:** se o job nascer verde, o job está errado.

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Modelo de ameaça do instrumento
**Status:** ⬜ Pendente
**Agente:** `hades-tf`
**Files affected:** apenas este roadmap.
**Actions:**
1. **Completude de enumeração.** Os onze defeitos conhecidos estão listados na issue #216 e nas REQs
   abertas. **Quais deles o job largo NÃO vai expor**, e por quê? Um job que reprova por 4 dos 11
   dá falsa sensação de cobertura sobre os outros 7. Enumere item a item: cp1252, `$HOME`, bit de
   execução, gate de cobertura, CRLF na escrita, `isatty`, `sh -c`, `\` no destino,
   `ref_targets_exist` vácuo, separador em artefato, e os 12 testes de symlink sem privilégio.
2. **Modelo de ameaça do próprio instrumento.** A sonda é execução manual num runner; o que ela pode
   vazar em log público? O job largo roda suíte completa em Windows — algum teste **escreve fora do
   workspace** lá, dado que `$HOME` não é isolado (defeito 2/6 da issue) e os testes fazem
   `Setenv("HOME")` que não isola em Windows? **Este é o vetor que mais me preocupa: o job pode
   corromper o próprio runner e produzir resultado não reproduzível.**
3. **Alvos de falsificação nas duas direções.** O que quebra se o job regredir para dirigido de novo;
   e se regredir para o outro lado — `continue-on-error` esquecido depois das correções, ou `skip`
   virando incondicional e escondendo tudo.
4. **Residual declarado.** O que o runner **não** responde: junction, `core.symlinks=false`, console
   cp1252 real, e o que depende da máquina de quem clona.
**Critérios de aceite:**
- [ ] As quatro seções com evidência, não asserção
- [ ] A tabela do item 1 cobre os 11, dizendo para cada um se o job expõe e como
- [ ] Nenhuma linha de implementação

**Gates da wave:**
```bash
test -f docs/adr/ADR-2026-08-30-ci-de-windows-como-instrumento-de-medicao-job-largo-que-nasce-vermelho-mais-sonda-sob-demanda.md
```

## Wave 1 — O job largo (ML único)
> Dependências: Wave 0 aprovada.

### ML-1A — Job de Windows rodando as três suítes, não bloqueante
**Status:** ⬜ Pendente
**Agente:** `ares-tf`
**Files affected:** `.github/workflows/quality.yml` (job novo; **não alterar** o
`windows-integrations-resolve` existente).
**Actions:**
1. Job novo em `windows-latest` com as três suítes completas.
2. `continue-on-error: true` — **não bloqueante até a última correção** (AC4). Documentar no próprio
   YAML que é temporário e o que o remove.
3. Registrar o **tempo** de execução (AC10).
4. **Não corrigir nada.** Se um teste falhar, é a medição.
**Critérios de aceite:**
- [ ] As três suítes rodam por inteiro
- [ ] **O job REPROVA** — colar a saída no roadmap como linha de base (AC2)
- [ ] Mapeamento falha → item da issue #216 (AC3); falha sem correspondência é achado novo
- [ ] Demais jobs do CI seguem verdes; `make quality` em Linux inalterado

## Wave 2 — `skip` explícito (ML único)
> Dependências: Wave 1 concluída — precisamos ver os 12 vermelhos antes de silenciá-los.

### ML-2A — `skip` nomeando a garantia não exercitada
**Status:** ⬜ Pendente
**Agente:** `apolo-tf`
**Files affected:** testes de symlink dos 3 runtimes.
**Actions:**
1. Detectar falha de privilégio ao criar symlink e **pular**, com mensagem dizendo **qual garantia
   não foi exercitada** e que exige Developer Mode. A formulação é do autor da issue: *"é diferente
   de ficar em silêncio"*.
2. Vale para os 12 testes nos 3 runtimes.
**Critérios de aceite:**
- [ ] AC7, AC8 — com privilégio executam; sem privilégio pulam **com mensagem**
- [ ] `skip` **não** é incondicional — falsificar: em Linux os testes continuam executando
- [ ] Suítes verdes em Linux; job de Windows deixa de reportar esses 12 como falha

## Wave 3 — A sonda (ML único)
> Dependências: Wave 1 concluída.

### ML-3A — Workflow `workflow_dispatch` de sondagem
**Status:** ⬜ Pendente
**Agente:** `ares-tf`
**Files affected:** `.github/workflows/` (workflow novo).
**Actions:**
1. Sonda respondendo, com saída **bruta**: modo devolvido por `os.Stat` num arquivo comum e num
   `chmod +x`; `Lstat` diante de symlink e de junction (`mklink /J`); `isatty` sobre `NUL`; encoding
   do console; terminador de linha dos arquivos que os geradores escrevem; `sh`/`bash` no `PATH`.
2. Sem segredo, sem escrita no repositório, `workflow_dispatch` puro (AC9).
3. Documentar no próprio YAML que **não substitui** o job de regressão (AC6).
**Critérios de aceite:**
- [ ] AC5, AC6, AC9
- [ ] Saída legível e citável em REQ — é o propósito
- [ ] Tempo de execução em poucos minutos, não dezenas

## Barreira final
Revisão `hefesto-tf` e `hades-tf`, auditoria do arquiteto e `barrier --wave 3`. **Só declarar
concluído com o CI verde** — e aqui "verde" significa: os demais jobs verdes **e** o job de Windows
reprovando pelos motivos esperados e mapeados.
