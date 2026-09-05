# Triagem das sete issues (#273–#279) abertas por `lourivalgarciajunior` em 2026-09-05

> Investigação, sem correção. Nenhuma issue foi comentada, nenhum código foi alterado, nenhuma
> operação de git foi executada. Branch `fix/retarget-dos-checks-de-camada-2` (PR #280, aberto, não
> mergeado) foi apenas lida para checar sobreposição — não tocada.

## Tabela

| # | Afirmação central | Veredito | Evidência medida | Ação exigida |
|---|---|---|---|---|
| **273** | `branch_has_wip_roadmap` falha também na direção oposta (branch legítima rejeitada porque o slug não é substring do nome do roadmap) | **CONFIRMADO** (mecanismo); corpus do fork não verificável aqui | `strings.Contains(normalizeBranchSlug(name), branchSlug)` em `internal/validator/validator.go:2571` — citação exata, byte a byte, do código real. `docs/req/REQ-2026-08-20-branch-has-wip-roadmap-casa-por-substring-num-corpus-de-done-que-so-cresce.md` existe, `status: Open`, sem roadmap — confirma que não é achado novo, é medição do AC4 de uma REQ já aberta desde 20/08 | Nenhuma ação de produto aqui e agora — é insumo para o AC4 da REQ já existente. Mover a REQ para roadmap/wip continua sendo decisão do arquiteto, não desta triagem |
| **274** | `node --test` reporta `pass 0 / fail 1` tanto para "arquivo não carregou" quanto para "um teste reprovou" — CI não distingue | **CONFIRMADO** (mecanismo, reproduzido de verdade) | Reproduzi localmente: `require('modulo-que-nao-existe')` → `tests 1 · pass 0 · fail 1`, exit 1. Um teste real reprovando (2 testes, 1 falha) → `tests 2 · pass 1 · fail 1`, exit 1 também. O discriminante (`pass==0 && fail>=1`) é real e suficiente. Contraste com Go confirmado: pacote que não compila sai `[build failed]`, distinto de teste reprovando — reproduzi isso também. Não reproduzi o estado específico que ele viu em `npm/tests/validator.test.js` (aqui roda limpo, `pass 1 fail 0` — provavelmente por causa do `node_modules` ausente no worktree dele, que ele mesmo declara como causa em outro contexto) | Ação de **CI**: um passo pós-suíte Node que falhe com mensagem própria quando `pass==0 && fail>=1`. Baixo risco, mecanismo simples, vale small ML |
| **275** | O `continue-on-error` por contagem escondeu uma regressão real introduzida pela #271 (`TestPathIsAnchoredForHookConfig_ControlePOSIX`) | **CONFIRMADO**, e reforçado pelo nosso próprio histórico | `git log` mostra `87aded6` (#271) introduziu o teste comparando contra `filepath.IsAbs` (comportamento antigo) — genuinamente vermelho em Windows nesse commit. `c60a7f8` (#272) reescreveu o teste para pinar valores literais e resolveu — `git log -S"pinados LITERALMENTE"` aponta exatamente para esse commit, não para #271. Isso bate byte a byte com a numeração dele (`33→32` em #271, `32→12` em #272) e com o G0 já documentado em `docs/portabilidade/2026-09-04-retriagem-do-residuo-de-windows-por-mecanismo.md` linha 516+. **Ele mediu corretamente algo que o nosso próprio instrumento também já tinha capturado e nomeado — mas só porque um humano lembrou de diffar por nome, não porque o CI obrigava** | Decisão de **arquitetura de CI**: avaliar o ratchet por nome (ver análise abaixo) |
| **276** | Sexto sítio de `os.IsNotExist` engolindo `ENOTDIR` no Windows, fora de `internal/validator/` (`internal/integrations/manager.go:477`); e o comentário de `validator.go:2451` nomeia o próprio defeito | **CONFIRMADO** (código e mecanismo POSIX); comportamento Windows **NÃO VERIFICÁVEL AQUI** | Os dois sítios citados existem literalmente como descrito — `manager.go:477` (`if os.IsNotExist(err) { return nil }` engolindo falha de scan de colisão) e `validator.go:2451` (comentário cita `ENOTDIR` como caso que deve reportar, guardado por `if !os.IsNotExist(err)`). Testei em macOS: `os.IsNotExist(ENOTDIR) == false` aqui (comportamento correto, POSIX distingue `ENOENT` de `ENOTDIR`) — consistente com a afirmação dele de que Node/Python já usam predicados precisos e só o Go é largo demais. Não pude testar `ERROR_PATH_NOT_FOUND` no Windows (a divergência de plataforma é conhecida na literatura do Go stdlib, mas não medida por mim) | Correção de **produto** de baixo risco: trocar `os.IsNotExist(err)` por `errors.Is(err, fs.ErrNotExist)` nos 6 sítios (5 já na #269/#269-aberta, 1 novo). Requer confirmação em CI Windows antes de fechar |
| **277** | Gate `check-roadmap-barrier-contract.sh` continua acoplado à governança do próprio repo mantenedor (108 de 144 basenames do snapshot ausentes num fork consumidor) mesmo após a #257 | **CONFIRMADO** estruturalmente pela leitura do script; contagem 108/144 do fork **NÃO VERIFICÁVEL AQUI** | `scripts/testdata/roadmap-barrier-corpus-snapshot/*.md` tem exatamente **144** arquivos aqui — bate com o número dele. O script (`scripts/check-roadmap-barrier-contract.sh:485-500`) faz `find "$ROOT_DIR/docs/roadmaps" -type f -name "$base"` para cada basename do snapshot — confirma que a tripwire de disco depende do `docs/roadmaps/**` do clone local, exatamente o mecanismo descrito. O comentário do próprio script já documenta que a #257 separou "corpus/non-vacuous" e "basename-missing-from-disk" em vereditos distintos (era 6 falhas, virou 1) — bate com a leitura dele de "a #257 resolveu a cascata, não o acoplamento" | Decisão de **arquitetura**: mover o corpus para fixture sintética (`scripts/testdata/`) desacoplada de `docs/roadmaps/**` reais, e isolar a tripwire de disco atrás de flag tipo `TRACKFW_SELF_GOVERNED=1`. Não é urgente para o #216, é dívida de portabilidade do produto para consumidores |
| **278** | `contentHasMarker` só reconhece vazio por uma grafia literal (`marker + " \n"` / `" \r\n"`); 5 de 7 grafias naturais de vazio passam; no acervo real, o gate vê 11 REQs sem ADR onde há muito mais | **CONFIRMADO**, com contagem própria reproduzida de forma independente | Código citado é idêntico, linha a linha, a `internal/validator/validator.go:87-96`. Reproduzi as 7 grafias com o código real extraído: bate **exatamente** com a tabela da issue — só `"ADR: \n"` e `"ADR: \r\n"` (um espaço) são detectadas como vazias; as outras 5 passam. Rodei `trackfw validate` no próprio acervo: **11** REQs "has no linked ADR" — bate exatamente com o número dele. Escrevi um scanner independente (não o dele) sobre os 193 REQs deste repo: 193 arquivos (bate), 11 acusados (bate), e **63** REQs que passam com uma linha `ADR:` vazia sem conteúdo real (a definição dele deu 58 — diferença de método de classificação, mesma ordem de grandeza e mesma conclusão) | Correção de **produto**, risco baixo-médio: trocar a detecção de vazio por regex de valor (`^ADR:[ \t]*(\S.*)$`, ancorado em início de linha). A ressalva dele sobre casar `ADR:` em prosa é real e deve ser medida contra os 193 REQs antes de trocar |
| **279** | 9 `t.Skip` de classe plataforma sobraram do ML-4A (#269), em 4 arquivos que a #269 não tocou | **CONFIRMADO**, integralmente | `git log -1 -- <arquivo>` para os 4 arquivos citados confirma que só `internal/generators/update_test.go` foi tocado por `e6f0d83` (commit da #269) — e mesmo lá, `git show e6f0d83 -- update_test.go` mostra que a única mudança converteu **um** símbolo (`execBitRepresentavelPara`), não os 9 apontados. Os outros 3 arquivos (`scaffold_doctor_test.go`, `manager_persistence_order_test.go`, `manager_test.go`, `provenance_test.go`) têm seu último commit **anterior** à #269. `grep -n "t.Skip"` confirma as 9 linhas exatas nos 5 arquivos citados (a issue lista 4 mas na verdade são os mesmos 4 arquivos, 9 sítios — bate) com as mensagens exatas citadas | Correção de **teste**, mesmo padrão já usado pela #269/ML-4A (supressão nomeada — `t.Logf`/mensagem visível sem `-v`). Baixo risco, mecanismo já validado |

## Os dois destacados

### #279 — os 9 skips remanescentes, e a decisão do vault

Medido, linha por linha, contra a árvore atual (pós-`e6f0d83`/#269):

```
internal/generators/scaffold_doctor_test.go            :102,133,164,215   "execute bit not applicable on Windows (AC5)"
internal/generators/update_test.go                     :35                "guarda de escrita através de symlink não exercitada..." (JÁ nomeado)
internal/integrations/manager_persistence_order_test.go:121,201           "permission bits behave differently on windows"
internal/integrations/manager_test.go                  :217               "symlink creation is privilege-dependent on Windows"
internal/thirdparty/provenance_test.go                 :154               "permission bits behave differently on windows"
```

Os 8 restantes (fora do já-nomeado `update_test.go:35`) são `t.Skip("...")` cru, sem `t.Logf`
prévio nem asserção de distância visível sem `-v` — exatamente a forma que o ML-4A (#269) definiu
como inaceitável para os 22 sítios que tocou, e o critério que o próprio commit da #269 registrou
("toda supressão nomeia a garantia"; sem `-v`, 0 ocorrências, com `-v`, 13). `git log -1` confirma
que nenhum desses 4 arquivos estava no escopo da #269: só `update_test.go` foi tocado, e só um
símbolo dentro dele.

**Cruzando com `vault/notes/goos-guard-e-do-binario-nao-do-host-wsl-continua-protegido-2026-09-01.md`**
(lido na íntegra antes desta análise, conforme instrução): essa nota decide uma questão
**diferente e não relacionada** — se o guard `CurrentGOOS != "windows"` em
`validator_credential_guard.go`/`validator_git_branch_guard.go` enfraquece um controle de
segurança ao suprimir a checagem do bit de execução em binários Windows-nativos (decisão: não
enfraquece, porque `CurrentGOOS` reflete o binário, não o host, e WSL continua protegido). Os 9
skips do #279 são `runtime.GOOS == "windows"` dentro de **arquivos de teste** (`scaffold_doctor_test.go`,
`manager_test.go`, etc.), não guards de produto em código de validação de segurança — mecanismo,
arquivo e propósito diferentes. **Não relitigo essa decisão porque ela não se aplica aqui: o #279 não
contesta se a supressão é correta (a propriedade genuinamente não existe no Windows — bit NTFS,
symlink sem privilégio), contesta se a supressão é *visível* sem `-v`.** Isso é dentro do critério que
o próprio `e6f0d83` já estabeleceu para os outros 22 sítios, só que não aplicado exaustivamente aos 4
arquivos que a #269 nunca tocou. Achado real, correção mecânica de baixo risco, mesmo padrão já
provado.

### #275 — a proposta de ratchet por nome

A medição de que a contagem escondeu uma regressão real (`TestPathIsAnchoredForHookConfig_ControlePOSIX`,
#271→#272) é confirmada pelo nosso próprio histórico e pelo G0 já documentado. Isso valida a
premissa da proposta — mas a proposta em si (`scripts/testdata/windows-known-red.txt` como ratchet
substituindo `continue-on-error`) tem armadilhas reais que ele mesmo nomeia e que valem destacar:

- **A lista vira cemitério.** Sem processo de poda ativo, nomes acumulam e ninguém remove os que
  já foram corrigidos — o ratchet detecta regressão mas não força limpeza; precisa de um "avisa
  quando nome da lista deixa de falhar" com enforcement, não só aviso.
- **Testes parametrizados/subtests quebram casamento por nome.** `go test -run` e `t.Run` geram
  nomes como `TestFoo/caso_1` — se o parâmetro variar por SO (contagem de casos diferente), o nome
  completo muda e o ratchet trata como "nome novo" mesmo sem regressão real. Não medido por ele,
  vale medir antes de adotar.
- **Refactor renomeia testes.** Renomear uma função de teste (prática comum, incentivada) quebra o
  casamento por nome e o ratchet acusa falsamente uma regressão que não existe — precisa de
  processo (atualizar a lista no mesmo PR do rename) ou o ratchet vira fricção que ninguém respeita.
- **A lista dele não serve de conteúdo inicial** — ele mesmo declara isso (Windows sem WSL, sem
  bash no PATH nativo, nomes que não coincidem necessariamente com o CI real). Isso significa que
  populá-la exige rodar o CI real do Windows e capturar os nomes de lá, um passo que ainda não foi
  dado.

A proposta resolve o problema real (contagem esconde composição), mas troca "vermelho temporário
visível" por "lista que exige manutenção ativa" — decisão de arquitetura de CI, não um fix mecânico.
Vale considerar como sucessor do `continue-on-error` quando os itens do #216 fecharem (o próprio AC4
já prevê essa remoção), não como substituto imediato.

## Cruzamento com o PR #280 e com o trabalho de hoje

Nenhum dos 7 issues é resolvido pelo PR #280 (aberto, não mergeado) — os arquivos que ele toca
(`internal/generators/agentfiles.go`, `npm/src/generators/hooks.js`, `pypi/trackfw/generators/hooks.py`,
`scripts/windows-repro/**`, dedup de separador no guard) não sobrepõem nenhum dos mecanismos
citados nas 7 issues (`branch_has_wip_roadmap`, `node --test`, ratchet de CI, predicados
`os.IsNotExist`, corpus do barrier-contract, `contentHasMarker`, `t.Skip`). São trabalhos
ortogonais.

Nenhuma issue contradiz uma conclusão já registrada — a #275 **reforça** o G0 já documentado em
`docs/portabilidade/2026-09-04-retriagem-do-residuo-de-windows-por-mecanismo.md`, e a #279 opera
dentro da (não contra a) decisão do vault sobre `CurrentGOOS`.

## Ordenação por retorno

Objetivo declarado do usuário: fechar o #216 e chegar a uma release segura.

1. **#279** (9 skips remanescentes) — maior retorno por unidade de risco. Mecanismo já provado
   pela #269, correção mecânica, sem decisão de arquitetura, fecha uma inconsistência que **nós
   mesmos** criamos ao não varrer o acervo (só o diff da wave). É o único dos 7 que é regressão de
   qualidade do nosso próprio critério, não achado novo de superfície.
2. **#278** (`contentHasMarker`) — retorno alto, risco baixo-médio. Reproduzido byte a byte,
   contagem própria confirma a ordem de grandeza (63 vs. 58 dele), e a correção (regex ancorado)
   é pequena e testável contra os 193 REQs do próprio acervo antes de trocar.
3. **#276** (`os.IsNotExist`/`ENOTDIR`) — retorno médio-alto, risco baixo (`errors.Is(err, fs.ErrNotExist)`
   é troca mecânica, mesma classe já corrigida 5x nesta campanha). Precisa de confirmação em CI
   Windows para fechar de verdade, mas o fix em si é seguro de aplicar.
4. **#274** (`node --test` pass 0/fail 1) — retorno médio, risco baixo. Não é sobre o produto, é
   sobre o próprio CI enxergar corretamente o que já roda; reproduzido de verdade nesta sessão.
   Relevante para não repetir o "contagem mentindo" que a #275 documentou para o Go.
5. **#275** (ratchet por nome) — retorno potencialmente alto, mas é decisão de arquitetura de CI
   com armadilhas reais (cemitério, parametrização, rename) que exigem desenho antes de implementar.
   Não bloqueia o #216; é sucessor do AC4, não substituto agora.
6. **#273** (`branch_has_wip_roadmap` bidirecional) — é insumo de medição para uma REQ **já aberta**
   desde 20/08 (`REQ-2026-08-20`), fora do escopo do #216. Não compete por prioridade com os itens
   de portabilidade Windows.
7. **#277** (corpus do barrier-contract acoplado ao mantenedor) — menor retorno para o objetivo
   declarado: é dívida de portabilidade do **produto para consumidores externos**, não item da
   issue #216 nem bloqueio de release. Real e bem fundamentado, mas fora do caminho crítico agora.

## O que só o CI ou um Windows real fecha, por issue

- **#274**: o comportamento exato de `pass`/`fail` pode variar entre versões do `node --test`
  (ele mesmo declara não ter verificado estabilidade do formato entre versões) — só o CI real
  confirma se o passo proposto é robusto no runner.
- **#275**: os nomes de teste vermelhos no runner real do CI — a lista dele não é o conteúdo
  inicial válido, só o CI produz isso.
- **#276**: o comportamento de `os.IsNotExist(err)` sobre `ERROR_PATH_NOT_FOUND` no Windows real —
  não reproduzível em macOS/POSIX (aqui `IsNotExist(ENOTDIR) == false`, comportamento correto).
- **#277**: a contagem exata "108 de 144" depende do conteúdo de `docs/roadmaps/**` do fork dele —
  não reproduzível aqui.
- **#279**: se os 9 skips realmente dispararam em Windows real (ele declara não ter executado a
  suíte para confirmar isso) — a existência do código está confirmada, a execução não.

## Afirmações do handoff que a medição derrubou ou ajustou

Nenhuma. Todas as premissas do handoff se confirmaram:
- O handoff avisou que o goos-guard vault note trata de uma decisão diferente daquela que o #279
  poderia parecer contestar — confirmado: são mecanismos distintos (guard de produto em código de
  segurança vs. `t.Skip` cru em arquivo de teste), e o #279 não a relitiga.
- O handoff avisou que a #275 precisava ser avaliada como proposta, não só como medição — a
  medição bateu 100% com nosso próprio histórico (#271/#272/G0), mas a proposta (ratchet por nome)
  tem armadilhas reais não triviais, detalhadas acima.
- Nenhuma issue estava já resolvida pelo PR #280 — confirmado por leitura dos arquivos tocados.

A única correção que fiz em relação à minha própria contagem inicial: o scanner independente do
#278 deu 63 REQs vacuamente passando, não 58 como ele reportou — diferença de critério de
classificação (ele conferiu 3 à mão e declarou não ter verificado as 58), não uma refutação do
achado; a ordem de grandeza e a conclusão ("o gate vê 11 onde há dezenas") se mantêm.
