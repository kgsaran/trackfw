---
status: done
date: 2026-08-12
req: "docs/req/REQ-2026-08-11-prova-negativa-dedicada-para-o-guard-de-vacuidade-credential-guard-present-do-check-agent-hooks-parity.md"
squad: "Ártemis, Hefesto"
---

# Roadmap: Prova negativa dedicada para o guard de vacuidade credential-guard-present

> Created: 2026-08-12 | Status: done

## Context

REQ: `docs/req/REQ-2026-08-11-prova-negativa-dedicada-para-o-guard-de-vacuidade-credential-guard-present-do-check-agent-hooks-parity.md`

`scripts/check-agent-hooks-parity.sh` tem **duas** camadas:

1. um **guard de vacuidade** (linhas ~190–202): para cada CLI × runtime, o arquivo de hook gerado
   não pode estar vazio **e** precisa referenciar `trackfw-credential-guard.sh`
   (`grep -q "trackfw-credential-guard.sh"`);
2. um **comparador estrutural** do JSON parseado, Go×Node e Go×Python.

O guard existe porque o comparador, sozinho, é vulnerável a falso verde: se os **três** stacks
removessem a entrada de credential-guard de forma **idêntica**, o diff cross-stack continuaria
passando e o gate diria "OK" sobre arquivos que perderam o controle de segurança.

**O problema:** o guard de vacuidade **não tem prova negativa própria** em
`scripts/check-gates-falsify.sh`. O Cenário 44 — única prova P4 hoje associada a esse gate —
falsifica apenas o **comparador estrutural** (corrompe o `matcher` do Kiro no Node e verifica que o
diff `go-vs-node` acusa). Se o guard de vacuidade parasse de funcionar, **nenhum cenário acusaria**.

Ou seja: o gate que existe para impedir falso verde é, ele próprio, não provado.

### Por que isto é prioridade, e não perfeccionismo

Este projeto já foi mordido duas vezes por exatamente esta classe de problema:

- **2026-08-08** — o guard de vacuidade capturou um falso negativo **ambiental**: o gate rodava
  `discover --init` sem isolar `$HOME`, e o credential-guard **global** instalado na máquina fazia o
  dedup pular a entrada de escopo de projeto. Corrigido isolando `$HOME` por runtime. Nota:
  `vault/notes/check-agent-hooks-parity-unisolated-home-false-failure-2026-08-08.md`.
- **2026-08-11** (ML-1A do `ROADMAP-2026-08-11`) — um teste aparentemente correto era **incapaz de
  distinguir** "migração ligada" de "migração ausente". Só a sabotagem revelou. A partir dali, prova
  negativa virou critério de aceite bloqueante.

O achado foi reportado por Hefesto em **duas sessões distintas** (2026-08-08 e 2026-08-11/ML-8A) sem
ser endereçado.

### 🔴 A armadilha que define o desenho do cenário

Esta é a parte que faz o ML dar errado se for ignorada.

**"Arquivo de hook sem entrada de credential-guard" é um estado LEGÍTIMO** em máquina com o
credential-guard **global** instalado: `globalCredentialGuardInstalled*()` (Go/Node/Python)
deliberadamente pula as entradas de escopo de projeto para o guard não rodar duas vezes por chamada.
Foi exatamente essa interação que produziu o falso negativo de 2026-08-08.

Consequências para o desenho:

- ❌ **Não funciona** sabotar apagando a entrada do **arquivo gerado**: o injector regenera o arquivo
  a cada execução do gate, e a sabotagem some. O cenário testaria nada.
- ✅ **Funciona** sabotar a **emissão nos geradores** — remover a emissão da entrada de
  credential-guard, **nos três stacks de forma idêntica** (é o cenário que o guard existe para
  pegar), em cópias do source, como o Cenário 44 já faz com `npm/`.
- ⚠️ O `$HOME` **tem de continuar isolado** durante o cenário. Sem isso, a máquina do desenvolvedor
  com guard global instalado faz o gate falhar pelo motivo **errado**, e o cenário vira um falso
  positivo que "passa" sem provar nada.

**O discriminante correto:** na árvore sabotada, o comparador estrutural **continua passando**
(os 3 stacks estão idênticos entre si) e é o **guard de vacuidade** que precisa falhar. Se o cenário
falhar por divergência estrutural, ele está testando o Cenário 44 de novo, não o guard.

## Acceptance Criteria

- [x] Existe cenário em `scripts/check-gates-falsify.sh` que falsifica **especificamente** o guard de
      vacuidade `credential-guard-present`.
- [x] O cenário segue o padrão do arquivo: **baseline** (gate passa na árvore íntegra) + **detecção**
      (gate falha na árvore sabotada). Não basta o gate passar.
- [x] A sabotagem remove a **emissão** do credential-guard nos **3 stacks identicamente**, de modo
      que o comparador estrutural continue satisfeito e apenas o guard de vacuidade acuse.
- [x] O cenário **prova** que a falha veio do guard de vacuidade, não do comparador: a asserção casa
      a chave `agent-hooks-parity/<cli>/<runtime>/credential-guard-present`.
- [x] `$HOME` permanece isolado no cenário.
- [x] `docs/cli-parity.md` atualizado: remover a ressalva de que o guard não tem prova negativa.
- [x] `make quality` verde, com o total de cenários incrementado (hoje 103) e a string de resumo
      final do `check-gates-falsify.sh` atualizada.

### Escopo negativo

- **Não** altera o comportamento de `scripts/check-agent-hooks-parity.sh` — só acrescenta a prova de
  que ele não é vácuo.
- **Não** altera código de produto: `internal/`, `npm/src/`, `pypi/trackfw/` e os testes ficam
  intocados (a sabotagem opera sobre **cópias** em diretório temporário).
- **Não** endurece nenhum outro gate, nem mexe no Cenário 44.
- **Não** trata a semântica fail-open × fail-closed — é a outra REQ.

---

## Wave 1 — Cenário de falsificação (1 ML)
> Dependências: nenhuma.

### ML-1A — Cenário 46: falsificar o guard de vacuidade `credential-guard-present`
**Status:** ✅ Concluído (Ártemis; auditado e aprovado por Zeus em 2026-08-12)
**Agente:** Ártemis (`artemis-tf`)

**Arquivos afetados:**
- `scripts/check-gates-falsify.sh` — novo cenário + string de resumo final (linha ~3587)
- `docs/cli-parity.md` — remover a ressalva sobre ausência de prova negativa

**Ações:**
1. Ler o **Cenário 44** (`scripts/check-gates-falsify.sh`, linhas ~3493–3536) e reusar a mecânica:
   cópia do source em `$T`, sabotagem, `assert_fails_with`.
2. Escrever o **Cenário 46** com baseline + detecção, conforme o §Context (ler a subseção
   "A armadilha que define o desenho do cenário" **antes** de desenhar a sabotagem).
3. Escolher o alvo de sabotagem mais estável possível e **documentar no comentário do cenário qual
   literal está sendo fixado e por quê** — esse literal vira âncora de manutenção, como já
   aconteceu com os cenários 34 e 35 (`RETARGETED 2026-08-02`) quando o alvo original sumiu num
   refactor.
4. Atualizar a string de resumo final do `check-gates-falsify.sh` com o novo total e a descrição do
   cenário, no mesmo formato dos existentes.
5. Atualizar `docs/cli-parity.md` removendo a ressalva.

**Critérios de aceite:**
- [x] `bash scripts/check-gates-falsify.sh` passa, com total incrementado.
- [x] 🔴 **Prova de que o cenário não é vácuo:** desabilite o guard de vacuidade em
      `scripts/check-agent-hooks-parity.sh` (comente o bloco `grep -q "trackfw-credential-guard.sh"`),
      rode `check-gates-falsify.sh` e confirme que o **novo cenário falha**. Restaure. **Reporte o
      resultado.** Sem essa checagem, o cenário novo tem o mesmo defeito que ele existe para corrigir.
- [x] 🔴 **Prova de que o cenário não está testando o comparador:** na árvore sabotada, o comparador
      estrutural continua passando. Demonstre (ex.: a saída mostra o `FAIL` da chave
      `credential-guard-present` e **não** de `go-vs-node`/`go-vs-py`).
- [x] `internal/`, `npm/src/`, `pypi/trackfw/` e arquivos de teste **intocados**.
- [x] `make quality` → exit 0, 0 `FAIL`.

**Comandos de validação:**
```bash
bash scripts/check-gates-falsify.sh
bash scripts/check-agent-hooks-parity.sh
make quality
```

---

## Wave 2 — Barreira de qualidade (1 ML)
> Dependências: Wave 1.

### ML-2A — Auditoria de qualidade do cenário
**Status:** ✅ Concluído (Hefesto; auditado e aprovado por Zeus em 2026-08-12)
**Agente:** Hefesto (`hefesto-tf`)
**Arquivos afetados:** nenhum por padrão (revisão). Correções só se Zeus autorizar.

**Ações:** revisar o Cenário 46 quanto a: fragilidade da âncora escolhida, isolamento de `$HOME`,
risco de falso positivo em máquina com credential-guard global instalado, e coerência com o padrão
dos demais cenários. Rodar `make quality` em `$HOME` **não** isolado para checar se o cenário novo
introduz sensibilidade ambiental — foi exatamente esse o modo de falha de 2026-08-08.

**Critérios de aceite:**
- [x] Parecer escrito cobrindo os 4 pontos acima.
- [x] Confirmação de que o cenário não é ambientalmente sensível.
- [x] Achados reportados a Zeus, não corrigidos unilateralmente.

---

## Wave 3 — Microlote corretivo da barreira (1 ML)
> Dependências: ML-2A. Aberto **pelo achado do ML-2A**, não previsto no plano original.

### ML-1B — Tornar o braço de detecção do Cenário 46 autodiscriminante
**Status:** ✅ Concluído (Ártemis; auditado e aprovado por Zeus em 2026-08-12)
**Agente:** Ártemis (`artemis-tf`)

**Achado que origina este ML (ML-2A, severidade baixa segundo o parecer — elevado por Zeus):** o
braço de detecção do Cenário 46 assevera apenas que os 3 labels
`agent-hooks-parity/claude/{go,node,py}/credential-guard-present` **aparecem** na saída. Isso é
satisfazível pelo **próprio modo de falha ambiental de 2026-08-08**: um `$HOME` vazado, sem
isolamento, suprimiria a entrada do Claude nos 3 runtimes exatamente como a sabotagem faz, e o gate
sairia antes do comparador do mesmo jeito. Hoje o que impede esse falso-verde é apenas o **braço
baseline falhar primeiro** — proteção indireta, dependente de ordem.

**Por que Zeus não aceitou como "hardening opcional":** este roadmap existe para provar que um gate
não é vácuo. Entregar um cenário cujo braço de detecção pode ser satisfeito por um vazamento
ambiental é reproduzir, dentro da própria correção, a classe de problema que ela corrige. O custo de
fechar é baixo; o custo de descobrir depois é o de 2026-08-08 outra vez.

**Arquivos afetados:** `scripts/check-gates-falsify.sh` (apenas o braço de detecção do Cenário 46).

**Ações:** tornar o braço de detecção **autodiscriminante** — não basta ver o FAIL do Claude; é
preciso provar que a causa foi a **sabotagem**, e não um vazamento de ambiente.

⚠️ **Cuidado com o discriminante escolhido.** A sugestão do parecer (assertar que um CLI
não-sabotado, ex. `codex`, **não** aparece com `credential-guard-present`) só discrimina se o guard
global estiver instalado para aquele CLI na máquina. Numa máquina com guard global apenas para o
Claude, um vazamento suprimiria só o Claude e a asserção negativa passaria mesmo assim. Escolha um
discriminante que **não** dependa do que está instalado no `$HOME` real.

**Critérios de aceite:**
- [x] O braço de detecção falha se a causa do FAIL não for a sabotagem.
- [x] O discriminante **não** depende de quais guards globais existem no `$HOME` real da máquina.
- [x] 🔴 **Prova:** demonstre que o novo discriminante reprova num cenário de vazamento simulado
      (ex.: remover o isolamento de `$HOME` na cópia do gate dentro do fixture) — e restaure.
      Reporte a saída.
- [x] Prova de não-vacuidade do Cenário 46 continua valendo: desabilitar o guard de vacuidade em
      `check-agent-hooks-parity.sh` ainda faz o braço de detecção falhar.
- [x] `bash scripts/check-gates-falsify.sh` sem `FAIL`; `make quality` exit 0.
- [x] `internal/`, `npm/src/`, `pypi/trackfw/` e testes intocados.

---

## Verificações finais de Zeus (2026-08-12)

- `check-gates-falsify.sh`: **104 cenários**, 0 `FAIL`. Os 4 braços do Cenário 46 (`baseline`,
  `detected`, `discriminant`, `structural-comparator-not-reached`) todos `OK`.
- `make quality`: **exit 0**.
- **Prova de não-vacuidade reproduzida por Zeus** (ML-1A): desabilitando o bloco
  `grep -q "trackfw-credential-guard.sh"` em `check-agent-hooks-parity.sh`, o braço `detected`
  falha com *"saiu com 0, esperava != 0"*. Restaurado, `git diff --exit-code` limpo.
- **Limite de auditoria declarado:** a simulação de vazamento de `$HOME` do ML-1B **não** foi
  reproduzida por Zeus — a tentativa falhou por erro do próprio harness de auditoria (a cópia do
  gate resolve caminhos relativos à própria localização, quebrando `NODE_CLI`/`PY_ROOT`). O que Zeus
  verificou diretamente foi o **código dos dois lados**: o isolamento de `$HOME` por runtime
  (`check-agent-hooks-parity.sh:147–149`), a construção do `$HOME` sintético e o laço de
  exclusividade. A evidência empírica é de Ártemis, com labels concretos (`claude`+`codex`+`gemini`+
  `copilot` sob vazamento contra o `$HOME` real).

## Notas de execução

- **Autoridade de Git:** apenas Zeus cria branch, commita e faz push.
- **Sem paralelismo:** são 2 MLs sequenciais, o segundo audita o primeiro.
- **Regra de paridade dos 3 CLIs não se aplica** a este roadmap: ele não altera código de produto,
  apenas gates e documentação.
