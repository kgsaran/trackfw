# Revisão de segurança — guard global cabeado, no-op fora de projeto, integridade por existência

> Autor: `hades-tf` | Data: 2026-08-18 (ML-4A, Wave 4)
> Escopo: `docs/roadmaps/wip/ROADMAP-2026-08-17-guard-global-cabeado-com-no-op-fora-de-projeto-e-integridade-independente-de-fiacao.md`
> REQ: `docs/req/REQ-2026-08-17-guard-global-e-instalado-sem-fiacao-e-sua-integridade-nunca-e-verificada.md`
> ADR: `docs/adr/ADR-2026-08-17-guard-global-cabeado-com-no-op-fora-de-projeto-trackfw.md`
> Método: leitura do código + execução direta (`./scripts/trackfw-git-branch-guard.sh` real, `./bin/trackfw`
> compilado localmente) contra fixtures de `$HOME`/cwd isolados, nunca a máquina real de KG exceto para
> confirmar que a `validate` genuinamente acusa o script global dele (evidência independente de AC3).

## Veredito

**APROVAR, com um risco residual que rebaixo de "aceito implicitamente" para "nomeado e preciso ser
corrigido/reconhecido no ADR" — é a resposta à pergunta B, e é o achado central deste parecer.**

Os MLs 1A/1B/2A/2B/2C/3A/3B entregaram exatamente o que prometeram, e medi isso eu mesmo (seção
"O que medi", abaixo). Não bloqueio o roadmap. Mas encontrei um vetor de desarme duplo
(nem projeto, nem global) que **não é o mesmo** que o já aceito "`rm trackfw.yaml`" do ADR, tem custo
de exploração menor do que corromper o script, e nenhum ML/ADR desta série o nomeia. Não bloqueei
porque (a) ele explora a mesma superfície que o `ADR-2026-08-12` já declarou fora de alcance —
`$HOME` gravável pelo agente — e (b) fechá-lo é barato e não exige reabrir nenhum dos MLs já
concluídos. Ver "Risco residual nomeado" para o porquê da escolha e o que recomendo.

---

## A) O no-op é induzível dentro de um projeto trackfw?

**Medido, não inferido.** Bateria em `internal/generators`/execução direta do script, fixture próprio
(`proj/trackfw.yaml` presente, subdiretórios reais):

```
baseline (dentro de projeto)               git push -> exit 2   (confirmado antes de cada ataque)
subdiretorio profundo (proj/sub/deep)      git push -> exit 2

A1  rm trackfw.yaml                        exit 0   ESPERADO — decisão do ADR, "destrói a
                                                     governança inteira, não só o guard"
A2  mv trackfw.yaml trackfw.yaml.bak       exit 0   mesma classe de A1
A3  trackfw.yaml vira DIRETORIO            exit 0   mesma classe — test -f corretamente nao
                                                     conta diretorio como arquivo; ainda assim
                                                     e "apagar a governanca", nao bypass novo
A4  trackfw.yaml chmod 0000 (sem leitura)  exit 2   NAO desarma — test -f so precisa stat, nao
                                                     read; o guard continua bloqueando
A5  trackfw.yaml e symlink QUEBRADO        exit 0   mesma classe de A1 (arquivo "nao existe"
                                                     do ponto de vista de test -f)
A6  subdir com trackfw.yaml FALSO/vazio    exit 2   NAO desarma — ao contrario, um trackfw.yaml
    mais proximo que a raiz real                    forjado ainda REARMA o guard (o script so
                                                     verifica existencia, nao conteudo)
A7  cwd e SYMLINK para o projeto           exit 2   pwd -P resolve o fisico corretamente,
                                                     nao desarma
A8/A11  ancestral do cwd sem permissao de  cd falha ou getcwd falha — E git real TAMBEM
        execucao (chmod 444/000) apos cd            falha na mesma condicao ("fatal: Unable
                                                     to read current working directory:
                                                     Permission denied") — autodestrutivo,
                                                     nao e vetor pratico (quem quebra o guard
                                                     assim quebra o proprio git push)
```

**Conclusão A: nenhum vetor testado induz o no-op SEM também destruir a governança do projeto
(remover/mover/vazar `trackfw.yaml`) ou quebrar o próprio `git` no processo.** `A4` e `A6` são os
dois únicos casos que tentam manter o guard aceso via manipulação de permissão/arquivo-falso, e os
dois **falharam em desarmar** — o guard continuou bloqueando. Não encontrei escalada de privilégio
aqui: quem consegue apagar/renomear `trackfw.yaml` já tinha, por definição, escrita irrestrita no
repositório, que é exatamente o modelo de ameaça que o `ADR-2026-08-12` já declarou não coberto.

Não fechei o vetor `A8`/`A11` (ancestral sem permissão de execução) como achado porque ele é
**autodestrutivo**: reproduzi com `git status` real na mesma árvore corrompida e ele falha com o
mesmo erro do sistema operacional antes mesmo de chegar ao guard — não é um bypass, é quebrar a
própria ferramenta que se quer contornar.

## B) A fiação global criou caminho de desarme? — **SIM, achado central, medido e reproduzido**

Fiz exatamente o que a pergunta B pediu: tentei forjar uma entrada global que o **dedup reconheça**
mas que **não execute**. Consegui, e o resultado é pior do que "voltar ao estado anterior à REQ"
(nenhuma proteção) — é **pior do que nunca ter cabeado**, porque agora existe um mecanismo (dedup)
que ativamente **remove** a proteção de projeto com base numa entrada global que nunca dispara.

### O mecanismo, confirmado por leitura de `internal/generators/agentfiles.go`

- `mergeClaudeHookArray` (o **escritor**, linha 1497) sempre grava
  `{"type": "command", "command": <path>}`.
- `hookArrayHasCommand`/`globalGitBranchGuardInstalledClaude` (o **leitor** usado pelo dedup, linha
  1658) só compara o campo `"command"` (via `samePathCommand`, normalizado) — **nunca olha para
  `"type"`, nem para nenhum outro campo do objeto hook.**
- `validateGuardGlobalHookResolvable` (`internal/validator/validator_git_branch_guard.go:127`) usa
  `collectCommandsWithMarker`, que percorre a árvore JSON inteira coletando **qualquer string que
  contenha o marker** (`"trackfw-git-branch-guard.sh"`) — também sem olhar `"type"` nem posição
  estrutural — e só confirma existência+executabilidade do script no caminho absoluto encontrado.

Ou seja: **nem o dedup (grava/le), nem o `trackfw validate` (audita) verificam se o objeto do hook é
o que o harness-alvo (Claude Code, documentado em https://code.claude.com/docs/en/hooks) realmente
reconhece como um comando executável.** `"type":"command"` é campo documentado como parte da forma
válida do hook; sem ele, o Claude Code real não invoca o comando.

### Prova por execução (não hipotética)

Escrevi um teste Go descartável (`internal/generators/zz_hades_forge_test.go`, **removido após a
medição — não faz parte deste roadmap nem foi commitado**) que:

1. Gera o script global real e correto via `GenerateGlobalGitBranchGuardScript(home)` — o script no
   disco está **íntegro**, byte-idêntico ao template. Nenhuma sabotagem de conteúdo.
2. Escreve `~/.claude/settings.json` com uma entrada `PreToolUse[matcher:"Bash"]` cujo `hooks` tem
   `{"command": "<caminho real do script>"}` **sem** o campo `"type"`.
3. Mede:

```
globalGitBranchGuardInstalledClaude()                 -> true   (dedup ENGANADO)
InjectClaudeHooks(dir) — entrada de projeto            -> SKIPADA (dedup ativou o skip)
validator.ValidateUnfiltered() — mensagens sobre
  git-branch-guard/git_branch_guard                    -> ZERO   (validate SILENCIOSO)
```

**O resultado combinado:** o script global existe e está íntegro (integridade não acusa — está
correta em fazer isso, o script é válido). O hook_resolvable global não acusa (o caminho absoluto
existe e é executável — está correto nisso também, isoladamente). O dedup de projeto acredita que a
proteção global está de pé e **remove** a entrada de projeto. **Mas o Claude Code real nunca invoca
esse hook**, porque falta `"type":"command"`. Resultado líquido: **nem o escopo de projeto nem o
escopo global protegem esse repositório**, e `trackfw validate` reporta **zero violações,
zero warnings** sobre isso. É exatamente o "ficar sem nenhuma das duas" que a barreira pediu para
investigar.

### Por que isto é diferente do risco já aceito no ADR

O ADR aceita: "repo sem `trackfw.yaml` não é protegido" e "`rm trackfw.yaml` desarma tudo, mas
destrói a governança inteira — não é escalada de privilégio". Este vetor é distinto em dois eixos:

1. **Superfície de escrita menor.** Não precisa apagar nada do projeto nem tocar em `trackfw.yaml`
   — só precisa poder editar (ou ter editado incorretamente, por bug de terceiro, digitação manual,
   ou uma versão futura de `trackfw update harness` com uma regressão de escrita) **um campo de um
   arquivo de configuração do `$HOME`** que já é, por design desta REQ, o lugar onde a defesa mora.
2. **É silencioso duas vezes, não uma.** `rm trackfw.yaml` deixa rastro óbvio (o arquivo sumiu, e
   TODO o resto da governança para de funcionar também — é autodenunciante). A entrada malformada
   deixa `trackfw validate` **verde**, o script no disco **íntegro**, e o dedup **correto do ponto de
   vista da própria lógica dele** — só que a lógica dele nunca verificou a única coisa que importa:
   se o harness-alvo de fato executaria aquele objeto JSON.

### Por que não bloqueei mesmo assim

Continua sendo verdade que **produzir** essa entrada malformada exige escrita em `$HOME` — o mesmo
pré-requisito que o `ADR-2026-08-12` já reconhece como fora do modelo de defesa ("controle que mora
onde o agente escreve não é controle" pressupõe que o agente **não** escreve onde o controle mora; se
ele escreve, a garantia sempre foi condicional). Não é uma vulnerabilidade nova de execução de código
nem de escalada — é uma lacuna de **detecção**: o `validate` deveria ter avisado e não avisa. Por
isso não bloqueio, mas recuso classificar como "aceito implicitamente pelo ADR" — o ADR nunca discute
esse vetor especificamente, e a REQ inteira é sobre fechar pontos cegos de verificação exatamente
desta natureza (integridade condicionada a existência, não a fiação). Deixar esse de fora seria a
mesma classe de inconsistência que a REQ nomeia para o próprio `git-branch-guard`.

**Recomendação concreta para o próximo ML (não implementada aqui, fora do escopo desta barreira):**
`validateGuardGlobalHookResolvable`/`collectCommandsWithMarker` deveriam confirmar que o objeto que
contém o `command`/`bash`/`action.command` correspondente **também** tem a forma mínima que o
harness-alvo exige (`"type":"command"` para Claude/Codex; equivalente para os outros 4) antes de
considerar a entrada "resolvida" — não só que a string do comando aponta para um arquivo executável
existente. Mesmo custo de implementação da checagem de `"type"` já presente no escritor
(`mergeClaudeHookArray`), só espelhada no lado de leitura. Nomeio, não implemento — commits e
correção são do `trackfw_architect`.

## C) A integridade nova é vacuosa em algum caminho?

**Discriminante central confirmado, medido nesta sessão (independente da evidência do ML-3A):**

```
$ ./bin/trackfw validate   (na maquina real de KG, script global genuinamente desatualizado)
⚠ ~/.trackfw/scripts/trackfw-git-branch-guard.sh (global scope) content diverges from the
  template this version of trackfw generates — run `trackfw update harness` to regenerate it
```

Fixture próprio, isolado (`$HOME` sintético):

```
script tampered (1 byte), legivel        -> ACUSA (baseline reconfirmada)
script tampered, chmod 000 (ilegivel)    -> SILENCIOSO — nao acusa
```

O segundo resultado é um ponto cego real: `validateGuardGlobalScriptIntegrity`
(`internal/validator/validator_git_branch_guard.go:208`) trata **qualquer** erro de `os.ReadFile`
(não só `os.IsNotExist`) como "não instalado, silêncio" — diferente da contraparte de projeto
(`validateGitBranchGuardScriptIntegrity`, mesma arquivo, linha 27), que propaga erros que não sejam
"arquivo ausente". **Não considero isto explorável na prática**: reproduzi que um script shell sem
permissão de leitura também não é executável por `bash`/pelo shebang (o interpretador precisa abrir
e ler o arquivo) — então "script ilegível" e "script que ainda protege alguém" são, para scripts
shell, mutuamente exclusivos. Registro como débito de robustez/consistência de contrato de erro, não
como achado que bloqueia.

**Vetor adicional que testei e É genuíno, mas de baixa severidade — mesma família do achado B:**
`validateGuardGlobalScriptIntegrity` só verifica o conteúdo do caminho **canônico**
(`~/.trackfw/scripts/trackfw-git-branch-guard.sh`). `validateGuardGlobalHookResolvable`, por outro
lado, aceita **qualquer caminho absoluto cuja string contenha** o marker (`collectCommandsWithMarker`
é um `strings.Contains` recursivo sobre toda a árvore JSON — não valida que o caminho é exatamente o
canônico). Ou seja: um hook apontando para
`/home/x/.trackfw/scripts/trackfw-git-branch-guard-backup.sh` (nome diferente do canônico, mas
contendo o marker como substring) passaria pelo `hook_resolvable` (existe? executável? sim) sem
NUNCA ter seu conteúdo comparado por nenhuma regra de integridade — a integridade só olha o caminho
fixo. Mesma conclusão de escopo que o item B: exige `$HOME` já gravável, não é uma escalada nova, mas
é uma inconsistência de cobertura que vale nomear junto do corretivo do item B, porque as duas
lacunas têm a mesma causa raiz (comparação por substring/campo solto em vez de validação estrutural
do hook).

**Não é vacuidade:** script **ausente** → silêncio (correto, é o contrato declarado); script
**presente e íntegro** → silêncio (correto); `~/.trackfw/scripts` sendo symlink para outro diretório
→ `os.ReadFile` segue o link e compara o conteúdo real, então uma tentativa de indireção por symlink
**não** escapa da checagem — testei por leitura de código (não há `Lstat`/tratamento especial de
symlink em `validateGuardGlobalScriptIntegrity`, então o comportamento padrão do Go — seguir o link —
se aplica, e é o comportamento correto aqui).

## D) O isolamento de `$HOME` nos gates esconde algo?

**Ratifico, com verificação própria, não só leitura.** Conferi os 4 scripts citados
(`check-gates-falsify.sh`, `check-artifact-parity.sh`, `check-barrier.sh`,
`check-validate-parity.sh`) — todos fazem `export HOME="$WORK/home"` (ou equivalente) **antes** de
qualquer chamada ao binário. `check-harness-hooks-parity.sh` usa isolamento **por invocação**
(`HOME="$home_dir" "$GO_BIN" ...`) em vez de `export` global — funcionalmente equivalente, confirmado
por leitura de todas as chamadas no arquivo, nenhuma delas roda sem o prefixo `HOME=`.

Para os testes Go/Python, o relatório do ML-3A cita `internal/validator/main_test.go`
(`TestMain`, isola `$HOME` para toda a suíte) e `pypi/tests/conftest.py`
(fixture `session`+`autouse`) — confirmei que ambos existem e cobrem o pacote inteiro. Para Node.js,
não há um `TestMain` equivalente, mas **não é gap**: cada teste que toca `$HOME` usa um helper
`withEnv(overrides, fn)` (definido em `credential_guard_integrity.test.js` e
`git_branch_guard_hook_integrity.test.js`) que salva/restaura `process.env` em `try/finally` — testei
que o `finally` cobre o caso de asserção lançando exceção (não só o caminho feliz), então não há
vazamento de `$HOME` sintético para testes seguintes nem de `$HOME` real para dentro do teste.

Não encontrei nenhum gate ou suíte que deixasse de exercitar caminho real relevante por causa do
isolamento — o isolamento é estritamente aditivo aqui (torna os gates herméticos sem remover nenhuma
asserção).

## Não verificado por mim / fora do escopo desta barreira

- Não tentei reabrir A1–A4/B1 do parecer de 2026-08-16 (evasões de tokenização de shell,
  `${IFS}`, `{git,push}`, `env`/`command` com argumentos) — já registrado como risco residual aceito
  em duas rodadas anteriores, sem mudança nesta REQ.
- Não medi o custo de performance do walk do no-op (`~0,77ms/chamada` alegado pelo ML-1A) — o
  arquiteto já registrou isso como não verificado por ele também; seria fora do escopo de segurança
  desta barreira de qualquer forma.
- Não testei o caminho `--json`/`applyRuleTagged` das regras novas — mesma lacuna que os Cenários
  47/49/50 do próprio gate já documentam como não coberta, não introduzida por este roadmap.

## Resposta direta ao pedido específico de KG

Não encontrei nenhum AC fechado cedo demais **dos oito da REQ** — todos os 8 estão genuinamente
cobertos pelo que meço acima. O que encontrei é um **gap não coberto por nenhum AC**: nenhum dos 8
critérios de aceite pede que o dedup ou o `hook_resolvable` validem a **forma estrutural** do hook
(campo `"type"`), só a presença/caminho do `"command"`. Não é "moldura de tripwire declarado sendo
usada para justificar buraco que dava para fechar barato" no sentido em que usei essa frase em
2026-08-16 — lá, o texto do ML *afirmava* robustez que não tinha; aqui, nenhum texto desta REQ afirma
cobrir validação estrutural do hook, então não há alegação falsa, só uma lacuna nunca nomeada. Ainda
assim, concordo que é barato de fechar (mesma checagem que o escritor já faz, só espelhada no leitor)
e recomendo nomeá-la explicitamente — como um débito residual explícito no ADR, não como uma
linha perdida em algum roadmap futuro sem dono.
