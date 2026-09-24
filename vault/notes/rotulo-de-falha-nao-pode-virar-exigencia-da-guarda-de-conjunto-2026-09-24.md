# Rótulo de `FAIL` não pode virar exigência da guarda de conjunto — e o que a guarda de fato prova

> Data: 2026-09-24 · Autor: `artemis-tf` · ML-2C do
> `ROADMAP-2026-09-23-a-apuracao-do-censo-morre-no-shard-limpo-e-os-19-rotulos-ausentes-vem-de-um-unico-chunk-que-morre-em-silencio.md`
> Fonte medida: snapshot de `scripts/check-gates-falsify.sh` em `a5747fc8` (`sha256 b6f26a6a…`)

## 1. A armadilha: "colher OK|FAIL" é inimplementável como está escrito

O ML-2C nasceu com a instrução de fazer `extract_expected_labels` colher
`echo "OK|FAIL [falsify/<rótulo>]"`. **Medição:** 43 rótulos literais do arquivo existem
**somente** em linha `FAIL` — `setup`, `setup-s75`, `setup-s167`, …, `vacuity-guard`,
`credential-guard-git-env-bypass/{attack-inert,config-attack-inert,worktree-baseline}`,
`git-branch-guard-global-hook-resolvable/kiro-dedicated-file/{no-regression,no-double-report}`.

São **diagnósticos do caminho de falha**. Numa árvore correta eles nunca são emitidos. Exigi-los
faria a guarda de conjunto cobrar que uma falha aconteça → **gate permanentemente vermelho**.

A regra correta é `OK` + `PROOF` (caminho de sucesso). Comando que separa os conjuntos:

```bash
python3 - <<'PY'
import re
lines=open('scripts/check-gates-falsify.sh',encoding='utf-8').read().split('\n')
P=re.compile(r'''echo\s+(["'])(OK|FAIL|PROOF)\s+\[falsify/([^]"']*)\]''')
ok=set();fail=set()
for l in lines:
    if l.strip().startswith('#'): continue
    for m in P.finditer(l):
        lab=m.group(3)
        if '$' in lab and m.group(1)=='"': continue     # aspas duplas: $ interpola -> glob
        (ok if m.group(2) in ('OK','PROOF') else fail).add(lab)
print(len(ok), len(fail-ok))   # -> 68 43
PY
```

🔴 **Resíduo que fica aberto:** 42 desses 43 não são exigidos por via nenhuma (1 sobrevive porque
também vem de um `assert_*`). 36 são da família `setup*`; **6 são braços reais**, e pelo menos
3 são controle de segurança. Eles **passam em silêncio** — não existe emissão de sucesso para
cobrar. Fechar isso exige dar emissão de sucesso a esses braços; é mesma causa, mesma REQ.

## 2. O que a guarda de conjunto prova — e o que ela NÃO prova

Ela **não** detecta remoção de um controle do fonte. O manifesto é derivado do **mesmo** fonte que
gera o runtime (`gen-falsify-chunks.py` lê `check-gates-falsify.sh` em tempo de CI): apagar o
`echo` apaga a emissão **e** o requisito, junto.

O que ela prova é outra coisa, e é o defeito real do censo: **o rótulo está no fonte e não apareceu
no log** — chunk que morreu no meio, bloco pulado, cenário que abortou antes do veredito.

Quem precisa cobrir a remoção no fonte é pino de conteúdo (`check-parity-call-site-pins.sh`,
`corrupt_literal`), não esta guarda. Não confunda os dois contratos.

## 3. `weight_keys` ≠ rótulo exigido

`extract_expected_labels` alimenta **dois** consumidores: o manifesto da guarda **e** as
`weight_keys` da calibração de tempo (ML-2H). Somar os rótulos de `echo` aos dois introduziu ~67
chaves sem peso calibrado, cada uma caindo no peso **pessimista** (máximo já calibrado): a massa de
empacotamento passou de **4.122s → 7.713s** e o LPT piorou o balanceamento com números fictícios.

Correção: `extract_expected_labels(..., include_echo=False)` para `weight_keys`. **Calibração e
cobertura são contratos distintos.** Efeito final no empacotamento: +46,66s de massa e makespan
previsto **inalterado** (812,67s).

## 4. Artefato de shard antigo não pode ser reauditado com gerador novo

Rodar `check-falsify-shard-coverage.sh` com o gerador novo sobre os artefatos do run
`35872779844` (gerados com o particionamento antigo) acusa **148** rótulos ausentes. É ruído: o
rótulo é exigido do chunk `i` do particionamento **novo** e foi emitido pelo chunk `j` do **antigo**.

A comparação válida é por **união** de todos os shards: antigo **19** ausentes, novo **33** — e os
33 estão **todos** no chunk 1, o que morreu. Nenhum é skip de plataforma.
