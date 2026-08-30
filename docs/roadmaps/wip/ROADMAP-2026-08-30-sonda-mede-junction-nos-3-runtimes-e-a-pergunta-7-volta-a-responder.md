---
status: wip
date: 2026-08-30
req: "docs/req/REQ-2026-08-30-sonda-nao-responde-a-pergunta-7-e-nao-mede-junction-em-node-e-python-a-guarda-de-symlink-pode-estar-furada-nos-3-clis-no-windows.md"
squad: "hades-tf, ares-tf, zeus-tf"
---

# Roadmap: Sonda mede junction nos 3 runtimes e a pergunta 7 volta a responder

> Created: 2026-08-30 | Status: wip

## Context

REQ: `REQ-2026-08-30-sonda-nao-responde-a-pergunta-7-e-nao-mede-junction-em-node-e-python-a-guarda-de-symlink-pode-estar-furada-nos-3-clis-no-windows.md`

A sonda mediu, na primeira execução pós-merge (run `33338382066`), que **o `os.Lstat` do Go não
marca junction como `ModeSymlink` — marca como `ModeIrregular`** —, e que junction é criada por
`mklink /J` **sem privilégio algum**. Toda guarda que testa `Mode()&os.ModeSymlink` é cega para ela.

Duas lacunas impedem escrever a correção: a **pergunta 7 falhou** (vírgula do PowerShell virou
array em `git update-index --cacheinfo`), e **Node e Python nunca foram medidos**. Sob a regra dura
de paridade, "defeito só do Go" e "divergência entre os três" são correções diferentes.

**Este roadmap não corrige guarda nenhuma.** Ele produz o número que decide a correção.

## Acceptance Criteria

- [ ] A pergunta 7 responde, com prova de que o argumento chega íntegro (não só que parou de errar)
- [ ] Junction medida em Node e em Python, com valores crus
- [ ] Tabela comparativa `runtime × (arquivo | symlink | junction)` legível sem cruzar logs
- [ ] A sonda continua **sem veredito** — nenhum `exit 1` por causa do valor medido
- [ ] Nota de correção na `REQ-2026-08-29` e nota de vault
- [ ] `actionlint` limpo, `make quality` verde, `quality.yml` **não** alterado

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat Model
> Dependências: nenhuma. Bloqueia toda a implementação.

### ML-0A — Modelo de ameaça da extensão da sonda
**Status:** ⬜ Pendente
**Agente:** `hades-tf`
**Files affected:** nenhum (documento em `docs/seguranca/`)
**Contexto que importa:** a sonda **cria** links no runner (symlink via `os.Symlink`, junction via
`mklink /J`, e agora um symlink versionado via plumbing do git). Ela vai passar a criar mais. Roda
em `workflow_dispatch` com **log público**.
**Actions:**
1. **Completude da enumeração** — a lista de superfícies deste roadmap está completa? Não se limite
   aos arquivos citados pela REQ: procure no repositório outros pontos que criem reparse point,
   symlink ou junction, e outros que decidam por `ModeSymlink`/`isSymbolicLink`/`islink`. Mostre a
   busca, não a conclusão.
2. **Modelo de ameaça** — quem esvazia esta Wave 0 sem quebrar regra escrita, e como? Em especial:
   a sonda pode ser levada a criar link **fora** de `RUNNER_TEMP`/workspace? O log público passa a
   revelar algo novo?
3. **Alvos de falsificação nas duas direções** — para cada superfície, o que quebra quando o
   comportamento regride, e o que quebra quando regride ao contrário (ex.: a sonda ganhar veredito,
   ou passar a esconder um valor cru atrás de interpretação).
4. **Residual declarado** — o que este desenho aceita não cobrir.
**Critérios de aceite:**
- [ ] As quatro seções respondidas com evidência, não asserção de uma linha
- [ ] Nenhuma linha de implementação escrita neste ML
- [ ] Parecer em `docs/seguranca/2026-08-30-modelo-de-ameaca-da-extensao-da-sonda.md`

**Gates da wave:**
```bash
test -f docs/seguranca/2026-08-30-modelo-de-ameaca-da-extensao-da-sonda.md
! grep -qi "placeholder" docs/seguranca/2026-08-30-modelo-de-ameaca-da-extensao-da-sonda.md
grep -q "Residual" docs/seguranca/2026-08-30-modelo-de-ameaca-da-extensao-da-sonda.md
```

## Wave 1 — A medição (ML único)
> Dependências: Wave 0 completa. **ML único porque as três ações tocam `windows-probe.yml`** — dois
> agentes no mesmo arquivo é a colisão que este projeto proíbe.

### ML-1A — Pergunta 7 responde e junction é medida em Node e Python
**Status:** ⬜ Pendente
**Agente:** `ares-tf`
**Files affected:** `.github/workflows/windows-probe.yml`, `scripts/windows-repro/node/` (arquivo
novo de sonda), `scripts/windows-repro/python/` (arquivo novo de sonda). **Não tocar**
`quality.yml`, nem `run.ps1`, nem `checks.go`/`checks.js`/`checks.py` (esses são da camada 2, que é
regressão, não sonda).
**Actions:**
1. **Corrigir a pergunta 7.** `git update-index --add --cacheinfo 120000,$blob,mylink` falhou porque
   **em PowerShell a vírgula constrói array** — chegaram três argumentos ao git. Passar como
   **string única**. Provar que o argumento chega íntegro, não apenas que o comando parou de errar.
2. **Junction em Node**: `lstatSync()` sobre junction, symlink real e arquivo comum, imprimindo
   `isSymbolicLink()`, `isDirectory()`, `isFile()` **crus**. Mesmo formato comparativo do braço Go.
3. **Junction em Python**: `os.path.islink()`, `os.lstat().st_mode`, `stat.S_ISLNK()` e
   `os.readlink()` (com o erro, se levantar), sobre os mesmos três alvos.
4. **Tabela final** `runtime × (arquivo | symlink | junction)` — é o artefato que a REQ de correção
   vai citar. Sem ela, comparar exige cruzar log à mão.
**Critérios de aceite:**
- [ ] AC1, AC2, AC3, AC4, AC5 da REQ
- [ ] 🔴 **AC6 da REQ — a sonda continua SEM veredito.** Nenhuma pergunta nova emite pass/fail nem
      `exit 1` por causa do *valor* medido. Sonda com veredito vira job de regressão disfarçado.
- [ ] Todo link criado fica dentro de `RUNNER_TEMP`/workspace
- [ ] `actionlint` limpo; `make quality` verde; `quality.yml` byte-idêntico

**Gates da wave:**
```bash
actionlint .github/workflows/windows-probe.yml
git diff --quiet origin/main -- .github/workflows/quality.yml
! grep -nE "^\s*exit 1" .github/workflows/windows-probe.yml
```

## Wave 2 — Governança (ML único)
> Dependências: Wave 1 completa.

### ML-2A — Nota de correção na REQ-2026-08-29 e nota de vault
**Status:** ⬜ Pendente
**Agente:** arquiteto (`zeus-tf`)
**Files affected:**
`docs/req/REQ-2026-08-29-namespace-de-agente-nao-declarado-em-agents-fica-invisivel-e-o-validate-reporta-limpo-sem-olhar.md`,
`vault/notes/` (nota nova), `vault/notes/index.md`
**Actions:**
1. **Nota de correção** na `REQ-2026-08-29`, cujo **AC12** está `Done` afirmando *"a enumeração não
   segue symlink, verificável nos 3 CLIs"* — verdadeiro no Linux, **falso no Windows para
   junction**. Anexar nota com link para o run que mediu. **Não reabrir a REQ, não reescrever o AC
   original** — o histórico fica.
2. **Nota de vault** sobre `Lstat`/junction/`ModeIrregular`, contendo a separação que custou a
   primeira leitura errada: guarda de **diretório** (furada), guarda salva por acidente via
   `IsDir()`, e guarda de **folha que nunca olha ancestral** — esta última independente de
   plataforma, porque `Lstat` só não segue o **último** componente.
**Critérios de aceite:**
- [ ] AC7 e AC8 da REQ
- [ ] A nota separa as três classes de guarda — a versão ampla demais ("todas furadas") é errada
- [ ] Nota linkada em `vault/notes/index.md`

**Gates da wave:**
```bash
grep -q "junction" docs/req/REQ-2026-08-29-namespace-de-agente-nao-declarado-em-agents-fica-invisivel-e-o-validate-reporta-limpo-sem-olhar.md
grep -q "junction" vault/notes/index.md
```

## Verificação diferida para pós-merge — NÃO é critério de aceite de nenhum ML

`workflow_dispatch` só é acionável a partir da branch default, então **a sonda estendida não pode
ser executada nesta branch**. A AC9 da REQ é estruturalmente inverificável antes do merge.

| Ação | Gatilho | Dono | O que fecha |
|---|---|---|---|
| Disparar `windows-probe.yml` | merge deste PR em `main` | arquiteto | AC9 — produz a tabela `runtime × alvo` e decide se a correção é só do Go ou divergência dos 3 |

## Barreira final

Revisão `hefesto-tf` e `hades-tf`, auditoria do arquiteto, `barrier --wave 2`. **A medição só existe
depois do merge** — este PR entrega o instrumento, não o número.
