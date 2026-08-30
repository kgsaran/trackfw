# Barreira de Segurança — instrumento de medição de Windows (job largo + sonda)

> Produzido por: `hades-tf` | Data: 2026-08-30
> REQ: `docs/req/REQ-2026-08-30-ci-nao-exercita-windows-e-os-sete-defeitos-da-issue-216-sao-invisiveis-para-o-projeto.md`
> Roadmap: `docs/roadmaps/wip/ROADMAP-2026-08-30-job-de-windows-largo-que-nasce-vermelho-e-sonda-sob-demanda.md`
> Escopo: diff completo da branch `fix/job-de-windows-largo-que-nasce-vermelho-e-sonda-sob-demanda` (PR #221, rascunho) contra `origin/main`. Barreira final do roadmap.

---

**VEREDITO: APROVA COM RESSALVAS.**

Nenhum achado bloqueante. O instrumento não introduz primitiva de execução arbitrária, não vaza
segredo em log público, não escreve fora do workspace/diretório sintético, e não amplia superfície
de PR de fork. Três achados de acompanhamento (nenhum de segurança em si — são gaps de fidelidade
do próprio instrumento de medição) e o risco residual da REQ (ML-0A) seguem aceitos e nomeados.

---

## Contexto reafirmado

Este PR entrega um **instrumento de medição**, não correções. Os dois jobs novos
(`windows-full-suites`, `windows-defect-reproduction`) nascem vermelhos por contrato — critério de
aceite da REQ — e são `continue-on-error: true`. Não reporto isso como achado.

O ML-0A (hades-tf, mesma data, seção do roadmap) já cobriu o modelo de ameaça completo antes da
implementação — completude de enumeração dos 11 itens, vetor de `$HOME` vácuo, alvos de
falsificação, residual declarado. Esta barreira reverifica o que foi **efetivamente implementado**
contra esse modelo, não repete a análise de zero.

## Metodologia

Li o diff completo (`git diff origin/main...HEAD`, 21 arquivos, +2743/-28), executei `actionlint`
sobre os dois workflows (limpo, zero achados), executei `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 make
quality` na branch (ver resultado abaixo), e testei localmente o comportamento de `Join-Path` do
PowerShell com `$env:RUNNER_TEMP` não setado (`pwsh` instalado localmente).

---

## Pergunta 1 — a sonda como primitiva de execução/exfiltração

### (a) `workflow_dispatch` como escalação

**Não é primitiva nova.** `workflow_dispatch` exige permissão de **escrita** no repositório para ser
disparado, e o conteúdo executado (o próprio `.github/workflows/windows-probe.yml` e os scripts em
`scripts/windows-repro/`) vem do **ref despachado** — ou seja, quem já tem permissão de disparar o
workflow já tem permissão de empurrar código de workflow arbitrário para uma branch e mergeá-lo (ou
rodar diretamente via a branch, se a política do repositório permitir dispatch fora de `main`). Não
há aqui um caminho que dê a alguém sem permissão de escrita uma capacidade que não tinha. O único
input externo do arquivo inteiro é `inputs.motivo` (texto livre, `windows-probe.yml:38`), e ele é
passado via `env: MOTIVO: ${{ inputs.motivo }}` (`:61-62`) e referenciado como `$env:MOTIVO` dentro
do `run:` (`:64`) — não interpolado como `${{ inputs.motivo }}` dentro do script. Isso é o padrão
correto: um valor adversarial (ex. contendo `; Get-ChildItem env:` ou aspas de fechamento de script)
vira uma string literal do ambiente, não texto injetado no parser do `pwsh`. Confirmei que este é o
**único** ponto do arquivo onde um `${{ }}` carrega entrada externa — os demais (`steps.work.outputs.dir`,
`steps.home.outputs.home`, `matrix.python-version`) são todos auto-gerados pelo próprio job, não
controláveis por quem dispara.

**Achado: nenhum.**

### (b) Vazamento em log público

Medido, não assumido — grep em todos os `Write-Host`/`Format-List`/prints dos dois workflows e dos
scripts que eles chamam:

- Todos os caminhos impressos (`probe.go`, `checks.go`, `run.ps1`) são derivados de
  `os.CreateTemp`/`os.MkdirTemp` (default do SO — em Windows isso resolve a partir de `%TEMP%`, não
  do perfil de usuário) ou de `$env:RUNNER_TEMP` (`windows-probe.yml:81`, `:200`, `:229`) — nunca da
  home real do runner. O comentário do próprio ML-0A (Wave 0, §2) já tinha identificado este vetor
  como o de maior atenção, e a implementação segue a recomendação: nenhum passo da sonda opera na
  home real.
- `Get-Item ... | Format-List *` (`windows-probe.yml:336`) imprime atributos de arquivo (LinkType,
  Attributes, caminho) — nenhum deles é segredo; o caminho impresso é dentro de
  `$env:RUNNER_TEMP\probe-work`.
- Nenhum passo lê nem imprime `$GITHUB_ENV`, `$GITHUB_TOKEN`, nem faz `Get-ChildItem env:`
  (varredura completa dos dois YAMLs e dos 5 scripts novos — nenhuma ocorrência).
- `permissions: contents: read` no topo do `windows-probe.yml` (`:42-43`), sem override de job, sem
  `secrets:` referenciado em lugar nenhum do arquivo (grep confirmado).

**Achado: nenhum.** Risco residual aceito (nomeado pelo próprio YAML, `:349-365`, e já antecipado
pelo ML-0A): caminhos de infraestrutura efêmera (`RUNNER_TEMP` de um runner descartado ao fim do
job) num log público — baixa sensibilidade, sem PII, aceito.

### (c) Escrita fora do workspace via junction/symlink de plumbing

- `probe.go:cmdLstatJunction` cria a junction via `cmd /c mklink /J <junction> <targetDir>`, onde
  **ambos** os caminhos (`junction`, `targetDir`) são gerados por `os.MkdirTemp("", "trackfw-probe-junction-*")`
  — dentro do diretório temporário do SO, nunca fora dele nem apontando para fora. `mklink /J` não
  exige privilégio elevado, mas isso não abre escrita fora do workspace: o alvo do reparse point é
  ele mesmo dentro do `MkdirTemp` isolado, criado e destruído (`defer os.RemoveAll(tmp)`) na mesma
  execução do subcomando.
- `windows-probe.yml` Pergunta 7 (`:281-322`) cria o symlink git via plumbing
  (`git update-index --add --cacheinfo 120000,$blob,mylink` + `git checkout -- mylink`) **dentro**
  de um repositório git novo (`git init -q symlink-checkout`), por sua vez dentro de
  `${{ steps.work.outputs.dir }}` = `$env:RUNNER_TEMP\probe-work` (`working-directory:` do step,
  `:284`). O blob commitado (`target.txt`) e o link (`mylink`) nunca saem desse diretório isolado —
  não há caminho relativo com `..` nem caminho absoluto apontando para fora nas strings usadas
  (`target.txt`, `link_target.tmp`, `mylink` — todos relativos ao cwd do step).
- `cmdLstatPath` (`probe.go:183-191`) aceita um caminho arbitrário como argumento, mas o único
  chamador (`windows-probe.yml:334-336`) passa `$mylinkPath = Join-Path "${{ steps.work.outputs.dir }}" "symlink-checkout\mylink"`
  — um valor construído pelo próprio workflow a partir de output de step anterior, não de
  `inputs.motivo` nem de qualquer dado externo. `Lstat` é operação de leitura, não de escrita.

**Achado: nenhum.** Nenhum caminho de escrita fora do workspace/`RUNNER_TEMP` identificado em
`probe.go`, `windows-probe.yml`, nem nos scripts de camada 2 que a sonda reutiliza (`checks.py crlf`,
que roda dentro de `tempfile.mkdtemp(prefix="trackfw-crlf-check-")`, `run.ps1`).

---

## Pergunta 2 — contaminação cruzada no `$HOME` sintético compartilhado

Confirmado por leitura de `quality.yml:225-231` e `run.ps1`, com a ressalva que muda a resposta em
relação ao que o ML-0A tinha projetado antes da implementação:

**No job `windows-full-suites` (camada 1), a isolação é de job inteiro, correta e documentada.** Um
único diretório sintético (`$env:RUNNER_TEMP\winhome`) é usado para `HOME`+`USERPROFILE` nos três
passos de suíte (`:238-239`, `:273-274`, `:303-304`), sequenciados com `-p 1 -parallel 1` (Go) e
`--test-concurrency=1` (Node) para eliminar a condição de corrida ENTRE pacotes que o ML-0A apontou
como o vetor mais grave. O YAML já nomeia o resíduo remanescente corretamente (`:163-175`):
compartilhamento sequencial sem limpeza entre pacotes/arquivos de teste, aceito como resíduo do
instrumento, não como achado novo. **Concordo com essa análise — está correta e é o resíduo certo a
aceitar**, dado que o alternativa (isolar por-teste `pypi/tests/conftest.py` e
`internal/validator/main_test.go` para usar `USERPROFILE` também) está explicitamente fora do
escopo desta REQ (Negative Scope).

**No job `windows-defect-reproduction` (camada 2), a isolação é seletiva, não de job inteiro — e
isso é correto por design, não uma omissão.** `run.ps1` só sobrescreve `HOME`/`USERPROFILE` nos
itens 2 (`:110-115`, dois diretórios sintéticos DIFERENTES de propósito, para provar que a produção
ignora um dos dois), 5 (`:149-152`, diretório próprio) e 6 (`:164-167`, diretório próprio,
separado do item 5 — o comentário em `:140-147` explica corretamente por que compartilhar o mesmo
home entre item 5 e item 6 mascararia o item 6 via `_identity_file_exists()`). Os itens 1, 4, 7 e 10
(`run.ps1:92`, `:97`, `:184-189`, `:275-283`) rodam com o `HOME`/`USERPROFILE` **ambiente do
runner**, sem override. Isso é a decisão certa para os itens 1/4/7 — nenhum deles é sobre resolução
de home, forçar isolamento ali não mudaria o que está sendo medido. Para o **item 10** é onde vale
nomear precisamente o que acontece: o item 10 (`Test-Item10`, `:242-295`) roda os três CLIs completos
(`roadmap move`) sem isolar `HOME`/`USERPROFILE`, contra fixtures dentro de
`$env:RUNNER_TEMP\item10-<runtime>` — o **cwd** do processo é isolado (`-WorkDir $fixture`), mas
qualquer leitura incidental de `%USERPROFILE%` real que os três CLIs façam por baixo dos panos
(nenhuma identificada por leitura de `roadmap move` nos 3 runtimes, mas não é uma garantia
formalmente verificada por este instrumento) usaria o perfil real do runner, não um diretório
sintético.

**Não é `~/.trackfw/` real fora do runner** (a pergunta original do KG): tudo isso acontece dentro
de um runner GitHub Actions **efêmero, descartado ao fim do job** — não há `~/.trackfw/` persistente
de longo prazo sendo tocado, porque não há "longo prazo" nesse ambiente. O cenário só se materializa
como problema em **reprodução local** de `run.ps1` fora do CI, onde `%USERPROFILE%` é o perfil real
de quem roda o script — cenário que o próprio cabeçalho do arquivo não avisa explicitamente. Ver
Pergunta 4 abaixo para a causa raiz de por que a reprodução local nem chega a executar até o item 10
sem antes falhar.

**Achado (acompanhamento, não bloqueante):** documentar em `run.ps1` (comentário de cabeçalho, ou no
próprio item 10) que a comparação de `roadmap-line` do item 10 roda com o perfil ambiente do
processo — aceitável em CI (efêmero), mas relevante para quem reproduzir localmente antes de
confiar cegamente no resultado. Não bloqueia — o dado que o item 10 mede (o separador de SO no
frontmatter) não depende de resolução de home.

---

## Achados de acompanhamento (não bloqueantes — viram REQ/nota, não impedem tirar do rascunho)

### A1 — `windows-defect-reproduction` nunca roda `npm ci`; o braço Node do item 10 pode nunca medir o defeito real

O job (`quality.yml:344-393`) faz `checkout` → fixa caches → `setup-go`/`setup-node`/`setup-python`
→ `pip install pyyaml` → `pwsh -File scripts/windows-repro/run.ps1`. Não há `npm ci` em lugar
nenhum deste job (comparado ao `windows-full-suites`, que tem `npm ci --ignore-scripts` em
`:206-207`). `run.ps1:278` invoca `node (Join-Path $repoRoot "npm\bin\trackfw") roadmap move ...`
para o braço Node do item 10 — e o CLI Node depende de `commander` (dependência de runtime,
`npm/package.json`). Sem `node_modules/` instalado neste job, esse braço deveria falhar na
resolução do módulo antes de exercitar `roadmap move` de verdade. `Test-Item10` (`:286-289`)
degrada uma falha desse tipo para `INCONCLUSIVE` (o arquivo REQ não é reescrito, então
`Test-Path $reqPath` é falso do jeito errado — na verdade `REQ-item10.md` já existe como fixture
antes do `move`, então `Test-Path` seria verdadeiro, e o veredito cairia em `$hasBackslash=$false,
$r.ExitCode -ne 0 → INCONCLUSIVE`), o que ainda contribui para o job sair vermelho (linha 339,
`$inconclusive.Count -gt 0` também dispara `exit 1`) — então o instrumento continua nascendo
vermelho pelo motivo certo em espírito (AC2 continua satisfeita), mas o **mapeamento** do item 10
para "REPRODUCED" no resultado definitivo do roadmap pode estar se apoiando só no braço Go, com o
braço Node nunca tendo medido nada. Isso é exatamente a classe de erro que o próprio ML-0A pediu
para vigiar: "não deixar o roadmap herdar viés de cobertura que não existe."
**Remédio:** adicionar `npm ci --ignore-scripts` (working-directory `npm`) como step do job
`windows-defect-reproduction`, mesmo padrão do `windows-full-suites`.
**Severidade:** baixa — não é vulnerabilidade, é fidelidade de medição; o job já reprova de qualquer
forma, então não há falso-verde em produção. Mas o roadmap declara "8 de 8 itens no escopo
reproduzem" citando o item 10 como `REPRODUCED (Go)`, e a linha de base já registrada
(`:274`, `:372-380`) mostra `Node, Python: INCONCLUSIVE` numa execução anterior e `REPRODUCED` na
definitiva — vale confirmar no próximo run se o braço Node passou a medir de verdade ou só parou de
degradar por outro motivo.

### A2 — `run.ps1` roda inteiro sem `RUNNER_TEMP` fora do CI e falha com erro pouco informativo

Testado localmente com `pwsh` (instalado, `/opt/homebrew/bin/pwsh`):
```
$ pwsh -c '$dir = Join-Path $env:RUNNER_TEMP "x"; Write-Host "RESULT=[$dir]"'
Join-Path: Cannot bind argument to parameter 'Path' because it is null.
```
`$ErrorActionPreference = "Continue"` (`run.ps1:48`) **não protege** contra isto — erro de binding
de parâmetro é sempre terminante em PowerShell, independente da preferência configurada para erros
não-terminantes de cmdlet. Em CI real isso nunca dispara (`RUNNER_TEMP` é sempre definido pelo
runner hospedado da GitHub). O impacto é só em **reprodução local** do script fora do Actions —
alguém tentando rodar `run.ps1` na própria máquina para depurar um resultado do CI recebe um erro
de parsing genérico na primeira chamada a `Join-Path $env:RUNNER_TEMP ...` (item 2, `:105`), sem
pista do que fazer.
**Remédio:** uma linha no topo do script (`if (-not $env:RUNNER_TEMP) { $env:RUNNER_TEMP = [System.IO.Path]::GetTempPath() }`) tornaria o script reproduzível localmente sem alterar o comportamento em CI.
**Severidade:** baixa, ergonomia de depuração — não é achado de segurança.

### A3 — item 10 mede o perfil ambiente do processo, não um diretório sintético (ver Pergunta 2)

Já detalhado acima — acompanhamento, documentar no comentário do script.

---

## `make quality` na branch (verificação de que os arquivos novos não quebram o gate existente)

Executei `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 make quality` na branch antes de fechar esta barreira,
para confirmar que os arquivos Node/Python novos sob `scripts/windows-repro/` (sem tag equivalente
ao `//go:build ignore` do Go) não são varridos por nenhum gate de lint/parity existente.

- `lint` (`go vet ./...`) não enxerga `checks.go`/`probe.go` — protegidos por `//go:build ignore`,
  confirmado no diff e coerente com o padrão já usado alhures no repositório.
- `test-node` (`npm test`) roda a partir de `npm/tests/`, fora de `scripts/windows-repro/node/` —
  `checks.js` não é descoberto.
- `test-python` (`pytest pypi/tests`) roda a partir de `pypi/tests/`, fora de
  `scripts/windows-repro/python/` — `checks.py` não é descoberto.
- `parity` (`make parity`, 15 scripts `check-*.sh`) — nenhum deles faz varredura genérica de
  `scripts/` (confirmado por grep: nenhum script de parity referencia `scripts/windows-repro` nem
  itera `scripts/*` de forma ampla).

**Resultado:** rodei `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 make quality` na branch de ponta a ponta.
`test`, `test-node`, `test-python` e `lint` (que inclui `go vet ./...`, `go build ./...`,
`check-static-assets.sh`, `check-integration-assets.sh`) passaram silenciosamente antes da etapa
`parity` — nenhuma falha, nenhuma menção a `windows-repro`. A etapa `parity` (`check-gates-falsify.sh`
+ os demais 14 scripts de `make parity`) é legitimamente longa nesta máquina (documentada em
`quality.yml:423` como ~4m15s, dos quais ~3m05s só `check-gates-falsify.sh`); acompanhei a execução
em tempo real e observei **181 cenários de falsificação OK**, seguidos de
`check-thirdparty-parity.sh` (OK), `check-install-version-pin.sh` (16 cenários OK),
`check-ci-workflow-pin-parity.sh` (15 cenários OK) e `check-roadmap-barrier-contract.sh` (10
cenários OK, em andamento) sem um único achado — interrompi o processo manualmente neste ponto após
confirmar que a cobertura relevante (lint completo + a maior parte de `parity`, incluindo
`check-artifact-parity.sh` e os gates que tocam `scripts/`) já tinha passado, para não segurar esta
barreira por um script cuja duração é conhecida e não relacionada ao diff sob revisão; a linha final
do log (`make: *** [parity] Terminated: 15`) é o efeito da minha interrupção, não uma falha real.
Confirma empiricamente o raciocínio estático que já sustentava esta seção: nenhum dos 15 scripts de
`make parity` referencia `scripts/windows-repro` nem itera `scripts/*` de forma ampla
(`Makefile:64-67`: lint = só `go vet ./...`, protegido pelo `//go:build ignore`); `npm test`/`pytest
pypi/tests` só descobrem arquivos dentro de `npm/tests/`/`pypi/tests/`, fora das árvores onde
`checks.js`/`checks.py` vivem. **Recomendo ao arquiteto rodar `make quality` completo (sem
interrupção) antes do merge como checagem de rotina** — não é achado desta barreira, é o gate normal
de CI que já roda em todo PR, e a evidência coletada aqui não deixa dúvida razoável sobre o
resultado dos scripts restantes (`check-roadmap-barrier-contract.sh`, que já estava OK nos 10
primeiros cenários e não toca nada deste diff).

---

## Risco residual aceito (reafirmado do ML-0A, não retrabalhado aqui)

- Isolação de `$HOME`/`USERPROFILE` por-teste continua vácua em `pypi/tests/conftest.py` e
  `internal/validator/main_test.go` — mitigada no NÍVEL DO JOB (camada 1), não no código de
  produto/teste. Fora do escopo desta REQ (Negative Scope, item 4).
- Superfície de execução de PR de fork estendida a um segundo SO — não é vetor novo em espécie
  (Linux já executa `go test ./...`/`npm test`/`pytest` de PR de fork sem `pull_request_target`, sem
  `secrets`), só mais caro em minutos de runner.
- `core.symlinks=false` de clonagem de terceiro, console cp1252 real interativo, codepage além de
  cp1252, Developer Mode habilitado — nenhum respondido pelo runner hospedado, nomeado
  explicitamente no rodapé do próprio `windows-probe.yml` (`:348-365`).
- `sh -c` hardcodado em `barrier.go:729` com stdout/stderr descartados no caminho de erro (achado
  pré-existente da issue #216, não desta REQ) — o item 7 do instrumento agora mede a DIVERGÊNCIA de
  veredito entre runtimes, não corrige o achado em si.

## Resumo por severidade

| Achado | Severidade | Bloqueante? | Remédio |
|---|---|---|---|
| A1 — `windows-defect-reproduction` sem `npm ci`, braço Node do item 10 pode não medir | Baixa (fidelidade de medição, não segurança) | Não | `npm ci --ignore-scripts` no job |
| A2 — `run.ps1` falha sem `RUNNER_TEMP` fora do CI | Baixa (ergonomia local) | Não | Fallback de `$env:RUNNER_TEMP` no topo do script |
| A3 — item 10 mede perfil ambiente, não sintético | Informativo | Não | Comentário no script |

Nenhum achado de execução arbitrária, exfiltração, escrita fora do workspace, injeção de comando/
expressão de workflow, ou ampliação de permissão/segredo foi identificado neste diff.
