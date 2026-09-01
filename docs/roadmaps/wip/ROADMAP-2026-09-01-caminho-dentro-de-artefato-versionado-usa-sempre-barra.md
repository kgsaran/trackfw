---
status: wip
date: 2026-09-01
req: "docs/req/REQ-2026-08-30-caminho-portavel-montado-com-separador-do-sistema-vaza-para-dentro-de-artefato-versionado.md"
squad: "hades-tf, apolo-tf"
---

# Roadmap: Caminho dentro de artefato versionado usa sempre barra

> Created: 2026-09-01 | Status: wip

## Context

REQ: `docs/req/REQ-2026-08-30-caminho-portavel-montado-com-separador-do-sistema-vaza-para-dentro-de-artefato-versionado.md`

**Item 10 do issue #216.** `roadmap move` grava `roadmap: docs\roadmaps\wip\ROADMAP-x.md` no
frontmatter da REQ quando roda no Windows. No Windows resolve, porque o `os.Stat` aceita as duas
grafias. **Em Linux não resolve — e o arquivo vai para o git.** Basta commitar no Windows e alguém
dar checkout no Linux para a referência quebrar.

**Caminho dentro de arquivo não é caminho de sistema de arquivos — é dado portável, e tem que ser
sempre `/`.**

Medido no CI (PR #229): **reproduz nos 3 runtimes**, incluindo o caso misto no Python
(`docs/roadmaps\wip\ROADMAP-item10.md`).

## Acceptance Criteria

- [ ] Enumeração real dos pontos que montam caminho **escrito dentro** de conteúdo, nos 3 runtimes
- [ ] Escrita sempre com `/`, independentemente do SO
- [ ] 🔴 **Leitura tolerante**: artefato já gravado com `\` **continua resolvendo**
- [ ] Falsificação nas duas direções, incluindo o controle da leitura tolerante
- [ ] Camada 2 vai de **5** para **4** `REPRODUCED`, com a transição do item 10 explicada e o run citado
- [ ] `make quality` e **CI** verdes

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Enumeração e modelo de ameaça
> Dependências: nenhuma. Bloqueia a implementação.

### ML-0A — Enumerar os pontos que escrevem caminho dentro de conteúdo
**Status:** ⬜ Pendente
**Agente:** `hades-tf`
**Files affected:** nenhum (documento em `docs/seguranca/`)
**Por que a enumeração é o entregável, e não um preâmbulo:** a REQ nomeia **dois** pontos conhecidos
(o sync do `roadmap move`, e `pypi/trackfw/commands/roadmap.py:609` no `.trackfw-log`). **Trate essa
lista como conhecidamente incompleta.** Nesta mesma sessão, duas enumerações minhas erraram por uma
ordem de grandeza — 3 guardas contra 187 pontos de escrita, e 3 guardas de folha contra 228
candidatos. O padrão já não é acaso: **grep por sintoma encontra quem já trata o problema, não quem
o ignora.**
**Actions:**
1. Varra os **primitivos de junção** — `filepath.Join`, `path.join`, `os.path.join`, `Path(...) / ...`,
   e concatenação manual com separador — e **filtre pelos que têm o resultado escrito dentro de
   conteúdo de arquivo**, não usados para acessar o sistema de arquivos. **A distinção é a análise
   inteira:** `filepath.Join` para abrir um arquivo está **correto** e não deve ser tocado.
2. Modelo de ameaça: um caminho com `\` num artefato versionado é dado que atravessa máquinas. O que
   quebra, e para quem, quando ele chega ao Linux?
3. **Falsificação nas duas direções**, com atenção especial à direção simétrica: uma normalização
   agressiva demais que **quebre a leitura de artefato já gravado com `\`** troca um defeito por um
   pior — repositórios existentes quebram no upgrade. Nomeie o que **não** pode ser normalizado.
4. Residual declarado.
**Critérios de aceite:**
- [ ] Enumeração distingue "caminho escrito em conteúdo" de "caminho usado para acessar arquivo"
- [ ] A lista dos 2 pontos da REQ é tratada como ponto de partida, não como escopo
- [ ] Nenhuma linha de implementação escrita
- [ ] Parecer em `docs/seguranca/2026-09-01-modelo-de-ameaca-do-separador-em-artefato.md`

**Gates da wave:**
```bash
test -f docs/seguranca/2026-09-01-modelo-de-ameaca-do-separador-em-artefato.md
! grep -qi "placeholder" docs/seguranca/2026-09-01-modelo-de-ameaca-do-separador-em-artefato.md
grep -q "Residual" docs/seguranca/2026-09-01-modelo-de-ameaca-do-separador-em-artefato.md
```

## Wave 1 — A correção
> Dependências: Wave 0 completa. **O particionamento sai da enumeração** — escrever os MLs agora
> seria escopo inventado, pelo mesmo motivo que na REQ da guarda de ancestral.

## Wave 2 — Gate falsificável
> Dependências: Wave 1. O gate precisa provar a escrita **sem** máquina Windows — a REQ exige isso
> na AC5, e é o que torna a regressão detectável no CI Linux todo dia.
> 🔴 **O gate nasce ligado ao `Makefile` e com guarda de vacuidade** — contrato escrito em
> `docs/cli-parity.md` nesta sessão, depois de dois gates chegarem inertes.

## Verificação que só o CI fecha

A camada 2 indo de **5** para **4**. Ao contrário dos itens 2 e 3, **este check mede comportamento de
produto** — a saída do `roadmap move` no frontmatter da REQ —, então deve genuinamente virar.
Verifiquei o que ele mede antes de escrever o número, que foi exatamente o passo que faltou da última vez.

## Barreira final

`hefesto-tf` e `hades-tf`, auditoria do arquiteto, `barrier`. **CI verde**, não só verde local.
