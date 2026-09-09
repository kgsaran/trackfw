# Um gate que deriva sítios varrendo a árvore de trabalho conta artefato de build ignorado como defeito real — no CI, não localmente

> ROADMAP-2026-09-09-guard-emite-hookspecificoutput-e-a-razao-chega-ao-modelo-nos-3-clis.md, ML-4A.
> Achado no CI do PR #299, depois do ML-2A já mergeado localmente com `rc=0`.

## O sintoma

`Quality / parity-other-gates` reprovou no PR #299 com:

```
check-git-branch-guard-hook-schema: 9 sítio(s) derivado(s) (permissionDecisionReason, 3 stacks confirmados):
...
check-git-branch-guard-hook-schema: sítio(s) derivado(s) SEM cobertura ...
  pypi/build/lib/trackfw/generators/init_gen.py
  pypi/build/lib/trackfw/validator.py
make: *** [Makefile:64: parity-rest] Error 1
```

Localmente, o mesmo gate (`scripts/check-git-branch-guard-hook-schema.sh`) derivava 7 sítios e
passava. **O CI derivava 2 a mais — dois caminhos que nunca existem numa máquina de desenvolvimento.**

## A causa

`pypi/build/` está listado em `.gitignore` (linha 11). No workflow de CI, o step
`python -m pip install pypi/` (`.github/workflows/quality.yml`, múltiplos jobs) invoca o build
padrão do `setuptools`, que materializa `pypi/build/lib/trackfw/...` — **cópias literais do fonte**
em `pypi/trackfw/` — na árvore de trabalho do runner. Essas cópias contêm o texto
`permissionDecisionReason` porque são cópias byte-a-byte dos sítios reais.

`derive_sites()` (dentro do gate) fazia:

```bash
grep -rlas "permissionDecisionReason" --include='*.go' --include='*.js' --include='*.py' --include='*.sh' "$scan_root"
```

— uma varredura da **árvore de trabalho**, sem qualquer relação com o que o git versiona. No CI,
`pypi/build/lib/...` está no disco (o `.gitignore` não impede o `pip install` de escrever lá) e a
varredura os contava. O gate então checava os sítios derivados contra duas listas fechadas
(`EXECUTED_HERE`, `COVERED_BY_BYTE_IDENTITY_TEST`) — os dois caminhos novos não estavam em nenhuma,
e o gate reprovou nomeando-os, exatamente como projetado para fazer com qualquer sítio real sem
cobertura.

🔴 **Não era defeito do gate — era defeito da fonte de dados do gate.** A guarda de reconciliação do
ML-2A (`docs/roadmaps/wip/ROADMAP-2026-09-09-...md`, seção "Guarda de vacuidade 3") funcionou
exatamente como desenhada: acusou um sítio sem cobertura. Só que o sítio acusado não era um emissor
real do schema — era uma sombra do build, que o git nunca teria contado.

## Mesma causa do issue #288 (consumidor externo), mecanismo diferente

Já registrado (`issue #288`, fora deste vault): um consumidor externo via `cp -r` copiando conteúdo
**ignorado pelo git** para dentro de uma fixture de teste, corrompendo o resultado. Aqui é o mesmo
princípio — **o resultado de um processo depende de arquivos que existem no disco de uma máquina
específica, mas que o git não vê e o CI não reproduz de forma estável** — só que o mecanismo é
varredura (`grep -rl` sobre a árvore), não cópia (`cp -r`).

**Padrão a procurar em qualquer gate/script que "derive uma lista de arquivos" por `find`/`grep -rl`
sobre um diretório**: se esse diretório pode conter artefato de build/instalação (`build/`, `dist/`,
`.venv/`, `node_modules/`, `bin/`, ...) que o `.gitignore` já declara irrelevante, a varredura da
árvore de trabalho reintroduz exatamente o que o `.gitignore` existe para excluir.

## A correção

`derive_sites()` agora deriva de `git ls-files -- '*.go' '*.js' '*.py' '*.sh'` quando `scan_root`
está dentro de uma árvore git — exclui todo ignorado **por construção**, sem lista de exclusão por
nome para manter (excluir `pypi/build` por nome seria remendo: o próximo `dist/`, `.venv/`,
`node_modules/` ou `bin/` reproduziria o mesmo defeito e exigiria mais uma exceção).

Dois pontos que a mudança de fonte de dados força a decidir, explicitamente:

1. **Sítio versionado mas ausente no disco** (checkout esparso): ignorado — não é contado como
   sítio, e não reprova por si (mas pode acionar a guarda de vacuidade existente, se isso zerar um
   dos 3 stacks).
2. **`scan_root` fora de um repositório git**: `git ls-files` não tem o que responder. Fallback para
   a varredura antiga (`grep -rlas`), **declarado em stderr** — nunca em silêncio. Hoje só alcançado
   pelos diretórios sintéticos do `--self-test` do próprio gate (nenhum deles é repo git); a
   execução de produção sempre roda dentro do repo trackfw real, que é.

`-a` no `grep` de conteúdo continua preservado (um dos sítios reais, `npm/src/validator/index.js`,
tem byte NUL; sem `-a` o grep pula o arquivo em silêncio).

## Como reproduzir a condição do CI localmente, sem depender do CI para validar

```bash
mkdir -p pypi/build/lib/trackfw/generators
cp pypi/trackfw/generators/init_gen.py pypi/build/lib/trackfw/generators/init_gen.py
cp pypi/trackfw/validator.py pypi/build/lib/trackfw/validator.py
bash scripts/check-git-branch-guard-hook-schema.sh   # antes do fix: reprova nomeando os 2 caminhos
rm -rf pypi/build
```

Esse é o cenário mais direto de falsificação: reproduz literalmente o que o step `pip install pypi/`
faz no CI, sem precisar rodar o workflow.

## Levantamento — outros gates que varrem a árvore em vez de usar `git ls-files`

Medido nesta sessão (`grep -ln 'grep -rl\|grep -Rl\|find .*-type f' scripts/*.sh`), **reportado, não
corrigido** (fora do escopo deste ML): `scripts/check-artifact-closed-cycle.sh`,
`scripts/check-integration-assets.sh`, `scripts/check-identity-parity.sh`,
`scripts/check-static-assets.sh`, `scripts/check-update-parity.sh`,
`scripts/check-roadmap-barrier-contract.sh`, `scripts/smoke-integration-packages.sh`,
`scripts/sync-integration-assets.sh`. A maioria varre diretórios estreitos e controlados
(`internal/integrations/assets` canônico, projetos sintéticos sob `TMP_ROOT`/`mktemp -d`, tarballs
já empacotados, `docs/req`/`vault/notes` com padrão de nome fixo) — nenhum deles é obviamente
"árvore de fonte inteira + artefato de build instalável misturados", que foi a condição específica
deste defeito. **Não medido se algum ambiente de CI poderia colocar conteúdo ignorado dentro de um
desses diretórios estreitos** — fica como hipótese em aberto para quem tratar cada script.

## Ver também

- [git-branch-guard-schema-decision-block-rejeitado-pelo-claude-code-2026-09-09](git-branch-guard-schema-decision-block-rejeitado-pelo-claude-code-2026-09-09.md) — o ML-1A/1B/2A que introduziu este gate.
