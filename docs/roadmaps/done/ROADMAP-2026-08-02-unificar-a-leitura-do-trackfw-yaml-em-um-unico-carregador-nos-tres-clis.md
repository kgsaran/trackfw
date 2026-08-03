---
status: done
date: 2026-08-02
req: "docs/req/REQ-2026-08-02-unificar-a-leitura-do-trackfw-yaml-em-um-unico-carregador-nos-tres-clis.md"
squad: ""
---

# Roadmap: Unificar a leitura do trackfw.yaml em um unico carregador nos tres CLIs

> Created: 2026-08-02 | Status: done

## Context
<!-- Derived from REQ: REQ-2026-08-02-unificar-a-leitura-do-trackfw-yaml-em-um-unico-carregador-nos-tres-clis.md -->
REQ: docs/req/REQ-2026-08-02-unificar-a-leitura-do-trackfw-yaml-em-um-unico-carregador-nos-tres-clis.md
ADR: docs/adr/ADR-2026-08-02-caminho-unico-de-leitura-do-trackfw-yaml-com-namespaces-tipados.md

Cinco scanners artesanais de `trackfw.yaml` sobreviveram ao ciclo #106, em `update` e `sync`. E o
`update` do Python nunca teve leitor desses campos — lacuna funcional, não divergência de
implementação.

## Acceptance Criteria
<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->
- [ ] Nenhum módulo fora de `config` parseia `trackfw.yaml` nos 3 CLIs (AC1)
- [ ] Namespaces `Update` e `Sync` no contrato de config, chaves YAML inalteradas (AC2)
- [ ] Os 3 CLIs resolvem o mesmo valor para os 11 campos, provado por execução (AC3)
- [ ] YAML válido hoje rejeitado pelos scanners passa a funcionar (AC4)
- [ ] Precedência config→env e textos de erro de `sync`/`update` inalterados (AC5)
- [ ] `trackfw update` do Python lê e age sobre os 5 campos, como Go e Node (AC6)
- [ ] Cenários de falsificação discriminantes, um por CLI, em `check-gates-falsify.sh` (AC7)
- [ ] `docs/cli-parity.md` e docs de configuração listam os 11 campos (AC8)

## Nota de decomposição

O gerador propôs um ML por AC. **Descartado**: os ACs desta REQ são propriedades transversais
(paridade, ausência de segundo parser), não unidades de trabalho — um ML por AC produziria oito
lotes tocando os mesmos arquivos, todos sequenciais entre si.

A decomposição abaixo é **por arquivo**, e adota deliberadamente **executor único por wave,
cobrindo os 3 CLIs**. Nos ciclos de 2026-08-01/02, toda wave paralela com um agente por CLI
divergiu — e as divergências apareceram justamente nos casos que nenhuma fixture cobria. O primeiro
ciclo sem divergência foi o de executor único. O custo é perder paralelismo; o benefício é não
gastar um ML de reconciliação por wave.

## Wave 1 — Namespaces no carregador (1 ML)
> Dependências: nenhuma

### ML-1A — Adicionar `Update` e `Sync` ao contrato de config nos 3 CLIs
**Status:** ✅ Concluído (commits 853f1d3, 03c9206)
**Executor:** Apolo
**Arquivos afetados:** `internal/config/config.go`, `npm/src/config/index.js`,
`pypi/trackfw/config.py` (+ testes correspondentes). **Nenhum outro arquivo.**
**Ações:**
- Declarar os namespaces com os 11 campos, populados pelo `parse()` existente — sem segundo parse,
  sem segunda leitura de arquivo.
- Chaves permanecem **planas na raiz** do YAML, com os nomes atuais. O namespace é da estrutura em
  memória.
- Aplicar a mesma normalização para string na fronteira já em vigor (ADR de 2026-08-02).
- Default de campo ausente: string vazia, nos 3 CLIs.
**Critérios de aceite:**
- [ ] AC2 satisfeito
- [ ] `go build ./...`, `npm test`, `pytest` verdes
- [ ] Teste por CLI: fixture com os 11 campos → os 11 valores resolvidos como string
- [ ] Nenhum consumidor alterado nesta wave

## Wave 2 — Migrar consumidores e remover os scanners (1 ML)
> Dependências: Wave 1 completa

### ML-2A — Substituir os 5 scanners artesanais pelo carregador
**Status:** ✅ Concluído (commits f9168bb, 2b01905). Nota: YAML malformado passou a ser fatal
(exit 1) em `update`/`sync` nos 3 CLIs — antes os scanners liam `""` silenciosamente. Efeito
colateral esperado de usar o carregador único (que já tinha esse comportamento em `validate`/
`status`); revisar na barreira.
**Executor:** Apolo
**Arquivos afetados:** `internal/generators/update.go`, `internal/sync/linear.go`,
`internal/sync/jira.go`, `npm/src/commands/update.js`, `npm/src/commands/sync.js`,
`pypi/trackfw/commands/update.py`, `pypi/trackfw/commands/sync.py` (+ testes).
**Ações:**
- Remover `ReadUpdateConfig`, `readConfigField` (Go), `readUpdateConfig`, `readConfigField` (Node),
  `_read_config_field` (Python) e os helpers privados que só elas usavam (`splitKVupdate`,
  `splitLines`, `trimLeft`, `trim`).
- Consumir `cfg.Update.*` e `cfg.Sync.*`.
- **Preservar a precedência**: valor do arquivo primeiro, env var como fallback (AC5).
- **Preservar os textos de erro** de `sync` e `update` **literalmente**.
- **Python**: implementar a leitura dos 5 campos de `update` e agir sobre eles, alcançando Go e
  Node (AC6). Esta é a única mudança de comportamento observável autorizada.
**Critérios de aceite:**
- [ ] AC1: `grep -rn 'trackfw.yaml'` cruzado com `ReadFile`/`readFileSync`/`open(` retorna
      **exatamente 1** ocorrência por CLI — a do carregador
- [ ] AC4: os quatro casos da REQ resolvem corretamente nos 3 CLIs
- [ ] AC5: textos de erro byte-idênticos aos de `main`, verificados por diff de saída
- [ ] AC6: teste que **demonstra a mudança** — com `hooks: husky`, o `update` do Python produz o
      mesmo efeito observável que Go e Node. Teste que apenas continua verde **não** satisfaz.
- [ ] `make quality` verde
**Atenção:** `linear_api_key` e `jira_token` são roteados sem mudança de comportamento —
preservação mecânica, ver Negative Scope da REQ. Não alterar sua origem nem seu tratamento.

## Barreira — revisão especializada
> Bloqueia a Wave 3
**Status:** ✅ Aprovada (Hefesto + Hades, sem bloqueios).
- Hefesto: código morto/duplicação ausentes, `go vet`/parity scripts verdes. Achado não-bloqueante:
  `pypi update --dry-run/--json/--targets/--install-missing` (caminho `_run_project`) nunca chama
  `config.load()` — estrutural, pré-existente ao ML-2A, fora do escopo de AC6 (que fala do
  `trackfw update` **bare**, via `_run`, já correto). Nota:
  `vault/notes/python-update-run-project-bypassa-config-load-2026-08-03.md`. **Constraint para
  ML-3A:** o cenário de falsificação Python deve exercitar `trackfw update` sem flags — usar
  `--dry-run`/`--json`/`--targets` tornaria o cenário vazio (passa igual com scanner reintroduzido).
- Hades: sem vazamento de `linear_api_key`/`jira_token`. Achado informativo não-bloqueante:
  `trackfw serve` injeta `ProjectConfig` completo (incl. `Sync`) nos handlers HTTP de um processo de
  vida longa — nenhum handler lê `Sync` hoje, mas é superfície nova de reachability. Recomendação
  para ML de hardening futuro (fora deste roadmap): `json:"-"` em `SyncConfig` e equivalentes.

- **Hefesto** (code quality): remoção completa dos helpers órfãos, ausência de código morto,
  duplicação entre os 3 CLIs.
- **Hades** (segurança): material secreto passa a trafegar por um caminho novo. Confirmar que
  `linear_api_key`/`jira_token` não entram em log, mensagem de erro, saída de `--json` nem em
  `config` impresso para diagnóstico. **Reportar** — não corrigir; correção vem em microlote
  próprio.
- Zeus: auditoria do diff completo + `make quality` + `scripts/check-gates-falsify.sh` integral.

## Wave 3 — Proteção de falsificação (1 ML)
> Dependências: barreira aprovada

### ML-3A — Cenários que reprovam a volta do scanner artesanal
**Status:** ✅ Concluído (commits 47a5074, 7ffca28). 3 cenários novos (39/40/41), 99/99 OK na
suíte completa. Débito pré-existente registrado (não introduzido por este ML): Cenário 29 de
`check-gates-falsify.sh` depende de `LANG=C` implícito e reprova sob locale pt_BR — ver
`vault/notes/falsify-suite-locale-dependent-false-failure-2026-08-03.md`. `sync`'s
`readConfigField`/`_read_config_field` seguem sem cobertura de falsificação — fora do escopo de
3 cenários definido no roadmap (só `update`).
**Executor:** Ártemis
**Arquivos afetados:** `scripts/check-gates-falsify.sh` **apenas** (arquivo compartilhado — nenhum
outro ML pode tocá-lo em paralelo).
**Ações:**
- Um cenário por CLI, com braço de **baseline** (limpo → passa) e braço de **detecção**
  (corrompido → falha).
- Fixture **discriminante** obrigatória: valor que o scanner artesanal resolve **errado** e o
  carregador resolve **certo**. Candidatos em AC4 — chave aninhada homônima é o mais forte, porque
  o scanner casa a primeira linha em qualquer indentação.
- Guarda de vivacidade: após corromper, verificar que o defeito **aparece de fato** — não apenas
  que o arquivo mudou.
- Corrupção **determinística**, independente de filesystem e de ordem de leitura.
**Critérios de aceite:**
- [ ] AC7 satisfeito; contador de cenários atualizado no cabeçalho do script
- [ ] Suíte **integral** verde após a adição — não apenas os cenários novos
- [ ] Cada cenário novo, isolado, **falha** quando o scanner é reintroduzido
**Atenção:** rodar os cenários herdados **antes** de editar. `set -euo pipefail` aborta no primeiro
erro e pode esconder um segundo cenário quebrado atrás do primeiro — ver
`vault/notes/cenarios-de-falsificacao-quebram-em-refactor-do-alvo-2026-08-02.md`.

## Wave 4 — Documentação (1 ML)
> Dependências: Wave 3 completa

### ML-4A — Registrar os 11 campos no contrato de configuração
**Status:** ✅ Concluído (commit a63f5a3). Documentado em `docs/cli-parity.md` e `README.md`; não
havia entrada de exceção de paridade registrada para o gap do Python (só existia na REQ) — nada a
remover, só a fechar. Nota do vault de Hefesto sobre `_run_project` commitada junto.
**Executor:** Apolo
**Arquivos afetados:** `docs/cli-parity.md` e a documentação de configuração.
**Ações:**
- Listar os 11 campos, seus defaults e seus consumidores.
- Remover a lacuna do `update` do Python da lista de exceções, se registrada.
- Registrar o shell gerado (`grep`/`sed` sobre `roadmap_dir`) como exceção **intencional** e o
  motivo: roda sem o CLI presente.
**Critérios de aceite:**
- [ ] AC8 satisfeito
- [ ] `make quality` verde
