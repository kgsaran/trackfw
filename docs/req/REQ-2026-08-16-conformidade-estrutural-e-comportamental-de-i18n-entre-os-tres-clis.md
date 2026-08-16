---
status: Open
date: 2026-08-16
author: "Zeus (Arquiteto)"
adr: ""
roadmap: ""
---

# REQ: Conformidade estrutural **e comportamental** de i18n entre os três CLIs

> Date: 2026-08-16 | Status: Open (backlog, sem roadmap)
| Linear Issue:
| Jira Issue:

## Motivation

Levantada a partir do ML-2A da REQ de higiene, que corrigiu **uma** chave órfã e, ao varrer o resto,
expôs um problema estrutural. O arquiteto mediu de forma independente antes de abrir esta REQ.

### O enquadramento importa: não é "alinhar chaves", é **a saída diverge**

Começou como divergência de contagem de chaves. Medindo o **comportamento**, virou algo mais sério:

```
$ trackfw roadmap move nao-existe wip

GO      Error: roadmap "nao-existe" not found in any state directory
NODE    roadmap "nao-existe" not found in any state directory
PYTHON  Erro: Roadmap "nao-existe" não encontrado em nenhum estado.
```

**Três CLIs, três saídas diferentes para a mesma operação** — e o Python responde **em português**.
Testado com `LANG`/`LC_ALL` em `en_US.UTF-8` e `pt_BR.UTF-8`: **a saída não muda**. Ou seja, não é
i18n funcionando; é **string hardcoded em português** (`pypi/trackfw/generators/roadmap.py:544`).

Isto viola diretamente a regra dura de paridade dos 3 CLIs.

### Chave presente ≠ comportamento presente

A chave `roadmap.move.notFound` **existe** nos locales de Node e Python e **não é consumida por
ninguém** — mesma natureza da `errors.notFound` já removida no ML-2A. Portanto **contagem de chaves
não mede conformidade**, e "igualar as chaves" pode dar sensação de conserto sem mudar nada do que o
usuário vê.

### Números medidos (`en-US.json`, 2026-08-16)

| stack | chaves |
|---|---|
| Go | 54 |
| Node | 80 |
| Python | 77 |
| união | 82 |
| **presentes nos 3** | **52** |

Divergências:

- **só no Go (2):** `discover.description`, `serve.description`
- **só no Node (3):** `req.new.prompt.adrScope`, `...adrScopeGlobal`, `...adrScopeLocal`
- **em Node+Python e ausentes no Go (25):** todo o bloco `adr.new.prompt.*` (6),
  `req.new.prompt.*` (8 + `criteria`/`motivation`/`title`), `init.prompt.projectType_*` (4),
  `metrics.export`, `metrics.since`, `log.header`, `req.new.adrWarning`, `req.new.resolveADRs`,
  `roadmap.move.notFound`

O bloco de 25 sugere que **o Go usa strings hardcoded** onde Node e Python passam por i18n. Não é
necessariamente bug — mas é assimetria não declarada, e é o terreno em que divergências de saída
como a de cima nascem sem ninguém perceber.

### Os gates atuais não pegam

`scripts/check-roadmap-move-parity.sh` **existe** e menciona o caminho de "not found", mas
evidentemente **não compara a mensagem byte a byte** — senão estaria vermelho. O gate cobre o
caminho feliz e deixa passar a divergência de texto.

## Acceptance Criteria

- [ ] **AC1 — Comportamento primeiro.** Para um conjunto acordado de caminhos de erro e de saída de
      usuário, os 3 CLIs produzem saída **byte-idêntica** sob locale fixo. O critério é a **saída**,
      não a contagem de chaves.
- [ ] **AC2 — Nenhuma string de usuário hardcoded em português** em nenhum dos 3 stacks. O idioma
      base do produto é definido pelo locale, não pelo stack que implementou.
- [ ] **AC3 — Zero chaves órfãs.** Chave de i18n sem consumidor é removida ou passa a ser usada.
      Verificação por **consumo real**, não por presença no JSON.
- [ ] **AC4 — Assimetria remanescente declarada.** Onde um stack legitimamente não tem uma chave
      (porque não tem a funcionalidade), isso vai documentado em `docs/cli-parity.md` como exceção
      intencional — não fica implícito.
- [ ] **AC5 — Gate que impede reincidência**, cobrindo os caminhos de erro comparados em AC1, com
      **cenário de falsificação** conforme P4 do `ADR-2026-07-26-principios-de-design-de-gates-verificaveis`.
- [ ] **AC6** — `make quality` verde.

### Achado adicional — 2026-08-16: `trackfw help <chave>` diverge no campo `Impact:`

Encontrado pelo `apolo-tf` no ML-3A da REQ de higiene e **confirmado por execução pelo arquiteto**.
É a mesma classe do caso acima — saída de usuário divergente — e não estava coberto:

```
$ trackfw help wip_limit
GO      Impact:  Aumentar reduz a frequência de bloqueio; diminuir exige mais disciplina.
NODE    Impact:  Aumentar reduz a frequência de bloqueio.
PY      Impact:  Aumentar reduz a frequência de bloqueio.

$ trackfw help roadmap_dir
GO      Impact:  Alterar muda onde o gate procura roadmaps em backlog/wip/blocked/done.
NODE    Impact:  Subtrees backlog/, wip/, blocked/, done/, abandoned/ são relativos a este diretório.
PY      Impact:  Afeta listagem, movimentação e validação de roadmaps.
```

Em `wip_limit` o Go diz mais que os outros dois. Em `roadmap_dir` **os três dizem coisas
diferentes** — não é omissão, é conteúdo divergente descrevendo o mesmo campo de configuração.

O comando em si e a sugestão para chave inexistente estão idênticos nos 3; a divergência é só no
corpo do `Impact:`. Como `trackfw help` documenta as chaves do `trackfw.yaml`, três descrições
diferentes do mesmo campo é pior que uma incompleta: o usuário não tem como saber qual vale.

## Escopo negativo

- **Não** é para "igualar as 82 chaves" mecanicamente: copiar chave sem consumidor cria órfã nova e
  dá falsa sensação de conformidade. Ver AC3.
- **Não** adiciona idiomas novos nem mexe no mecanismo de detecção de locale.
- **Não** traduz o que hoje é intencionalmente técnico e não traduzível (nomes de comando, chaves de
  config, mensagens de gate comparadas byte a byte por scripts de paridade — **atenção:** traduzir
  uma dessas quebraria os gates existentes).
- **Não** é bloqueante para release.

## Notas de execução

⚠️ **Risco principal deste trabalho:** vários gates de paridade comparam mensagens **byte a byte**.
Mexer em i18n pode quebrá-los de forma legítima (a mensagem mudou) ou ilegítima (o gate deixou de
valer). Qualquer alteração de mensagem coberta por gate precisa ser **acompanhada da atualização do
gate**, e o cenário de falsificação correspondente precisa continuar provando que ele reprova.

📎 Origem: ML-2A do `ROADMAP-2026-08-16-higiene-sete-debitos-acumulados-da-entrega-de-plugins-e-da-release-7-0-0`.

## Linked ADR

ADR: (a decidir — AC2 e AC4 podem exigir uma decisão registrada sobre qual é o idioma base e o que
é intencionalmente não traduzível)

## Blocked by ADRs
<!-- none -->

## Linked Roadmap

Roadmap: (a criar quando esta REQ sair do backlog — não iniciar sem REQ + roadmap em `wip`)
