# Modelo de Ameaça — guarda de folha resolve o caminho e afirma contenção antes de escrever

> Produzido por: `hades-tf` | Data: 2026-08-31
> REQ: `docs/req/REQ-2026-08-31-guarda-de-folha-faz-lstat-so-no-ultimo-componente-e-nunca-inspeciona-ancestral-escrita-fora-do-projeto-em-todo-so-e-todo-runtime.md`
> Roadmap: `docs/roadmaps/wip/ROADMAP-2026-08-31-guarda-de-folha-resolve-o-caminho-e-afirma-contencao-antes-de-escrever.md`
> ML: ML-0A, Wave 0 (bloqueante — nenhuma linha de implementação escrita aqui)

---

## Veredito antecipado

A lista de 3 guardas nomeada pela REQ (`update.go:1869`, `:1894`, `discover.go:268`) **está errada
por subestimação, não por erro de leitura** — é um subconjunto pequeno de uma população muito maior
de pontos de escrita sem checagem alguma de ancestral. Encontrei uma classe nova e maior que as três
já conhecidas: **escritas de escopo global (`trackfw update harness`, sob `$HOME`) e escritas de
escopo de projeto em geradores de artefato (`req new`, `roadmap new`, `adr new`, `note new`, hooks,
`init`) que não fazem checagem de link nenhuma — nem de folha, nem de ancestral.** Comprovei a
exploração ao vivo nas duas frentes com o binário Go real (seção 1, PoCs). A forma
resolver-e-afirmar-contenção proposta pela REQ está correta na *forma*, mas **subespecificada na
soletração por runtime de um jeito que quebra em dois pontos concretos, medidos**: (1) a primitiva de
resolução falha sobre caminho ainda inexistente — o caso dominante, já que a maioria destas escritas
é *criação* de arquivo novo — exigindo resolver o diretório-pai e só depois juntar a folha, não
resolver o caminho completo; e (2) comparar destino resolvido contra `root` **não resolvido** produz
falso positivo em qualquer projeto cujo caminho contenha um symlink legítimo de sistema (medido:
`/tmp` no macOS é um symlink para `/private/tmp` — cenário comum, não hipotético). Nenhum dos dois
invalida a forma; ambos têm de entrar na especificação da Wave 1 como requisito, não como nota de
rodapé.

---

## 1. Completude da enumeração

### 1.0 — Prova ao vivo antes da enumeração (a REQ pediu evidência, não asserção)

Dois PoCs rodados com o binário Go real (`go build ./cmd/trackfw`), fora da árvore do projeto, em
`/private/tmp/.../scratchpad/`:

**PoC A — escopo de projeto, `trackfw req new`:**
```
$ ln -s $SCRATCH/poc-outside poc-project/docs/req      # symlink ANCESTRAL, não a folha
$ cd poc-project && trackfw req new "poc ancestral symlink escreve fora do projeto"
created docs/req/REQ-2026-08-31-poc-ancestral-symlink-escreve-fora-do-projeto.md
$ ls $SCRATCH/poc-outside/
REQ-2026-08-31-poc-ancestral-symlink-escreve-fora-do-projeto.md   ← escreveu FORA da árvore
```
Sem aviso, sem recusa, `exit 0`.

**PoC B — escopo global, `trackfw update harness`:**
```
$ ln -s $SCRATCH/poc-outside2 poc-fakehome/.claude            # symlink pré-existente em $HOME
$ HOME=$SCRATCH/poc-fakehome trackfw update harness --targets claude-skill --install-missing
✓ claude-skill: updated (~/.claude/skills/trackfw/SKILL.md)
$ find $SCRATCH/poc-outside2 -type f
poc-outside2/skills/trackfw/SKILL.md                            ← escreveu FORA de $HOME
```
Sem aviso, sem recusa, `updated=1 failed=0`.

Nenhum dos dois toca as 3 guardas de folha já nomeadas pela REQ — não passam nem perto delas. São
famílias de escrita **totalmente sem checagem**.

### 1.1 — Varredura pelos primitivos de escrita (comandos executados)

```bash
grep -rn "os\.WriteFile(\|os\.Create(" internal/ | grep -v _test.go | wc -l     # → 80
grep -rn "writeFileSync" npm/src/ | wc -l                                       # → 78
grep -rn "write_text(\|open(.*['\"]w" pypi/trackfw/ | grep -v test | wc -l      # → 65
```

**Delta declarado, não escondido:** o Go deu 80, a REQ cita 85. Não reconciliei a diferença de 5 —
pode ser `ioutil.WriteFile` (não encontrei nenhuma ocorrência) ou um grep levemente diferente do que
gerou o número da REQ. Node (78) e Python (65) batem exatamente com a REQ. A diferença de 5 no Go não
muda nenhuma conclusão abaixo porque toda a análise partiu de arquivo/função, não do número bruto.

### 1.2 — Classificação por família (a) escreve sob root/home controlável · (b) fixo/não controlável · (c) já inspeciona ancestral

| Família | Runtime : arquivo(s) | Nº sites | Checagem existente | Classe |
|---|---|---|---|---|
| Guarda de folha (já conhecida) | Go `update.go:1869,1894`, `discover.go:268` · Node `update.js:197,223`, `discover.js:360,603*` · Python `update.py:194,220`, `discover.py:499` | ~10 | `Lstat`/`lstat`/`islink` **só na folha** | (a) — furada, é o assunto da REQ |
| **Manager — instalação de catálogo (agents/skills), projeto E global** | Go `manager.go:684-690` `rejectSymlinks` · Node `manager.js:69` `assertNoSymlinks` · Python `manager.py:82` `_reject_symlinks` | 3 famílias | **Caminha o ancestral inteiro** até `root`, via `Lstat` em cada componente | (c) — correta, é o padrão a copiar |
| **Manager — limpeza de diretório vazio** | Go `manager.go:582` · Node `manager.js:420` · Python `manager.py:589` | 3 | Contenção geográfica (`beneath`/`path.isAbsolute`/`root in parents`), sem teste de tipo simétrico | fora de escopo (Classe 2 da REQ irmã de junction) — não mexer aqui |
| **NOVO — harness global, escrita direta (SKILL.md, settings.json, hooks.json, git-branch-guard)** | Go `internal/generators/update.go`, `sed -n '623,1720p' \| grep -c os\.WriteFile` **= 26** (funções `harness*`, faixa delimitada para excluir as guardas de folha em 1859+; `harnessCatalogTarget` linha 1721 fica de fora da faixa e é classe (c), não conta aqui) · Node `npm/src/commands/update-harness.js` `grep -c writeFileSync` **= 23** · Python `pypi/trackfw/commands/update_harness.py` `grep -c "write_text(\|open(...w"` **= 20** | **26 + 23 + 20 = 69** | **Nenhuma.** Nem folha, nem ancestral — `grep -c "Lstat\|ModeSymlink"` na mesma faixa Go **= 0**; zero também em Node/Python | (a) — furada, pior que a folha: nem tenta |
| **NOVO — geradores de artefato de projeto (req/roadmap/adr/note)** | Go `req.go`(4)+`roadmap.go`(3)+`adr.go`(2)+`note.go`(2) **= 11** · Node `req.js`(4)+`roadmap.js`(4)+`adr.js`(2)+`note.js`(2) **= 12** · Python `req.py`(4)+`roadmap.py`(4)+`adr.py`(1)+`note.py`(2) **= 11** | **11 + 12 + 11 = 34** | **Nenhuma** — `grep -c "Lstat\|ModeSymlink"`/`isSymbolicLink`/`islink` **= 0** em cada um dos 12 arquivos, verificado individualmente | (a) — furada, confirmada pelo PoC A |
| **NOVO — geradores de hook/script (credential-guard, git-branch-guard, husky, lefthook, CI workflow, `init`)** | Go `agentfiles.go`(12)+`scaffold.go`(18) **= 30** · Node `hooks.js`(9)+`init.js`(18) **= 27** · Python `hooks.py`(1)+`init_gen.py`(17) **= 18** | **30 + 27 + 18 = 75** | **Nenhuma** — `grep -c` de link/symlink/reject **= 0** em `scaffold.go`, `agentfiles.go`, `init.js` (checado à parte), `init_gen.py`, `hooks.py` | (a) — mesma família da REQ irmã, ancestral controlável (`.husky/`, `.lefthook/`, `.github/`) |
| Sites menores não citados pela REQ | Node `sync.js`(1)+`metrics.js`(1)+`configure.js`(1)+`thirdparty/quarantine.js`(1) **= 4** · Go `commands/discover.go`(1)+`commands/configure.go`(1) **= 2** · Python `sync.py`(1)+`metrics.py`(1)+`configure.py`(1) **= 3** | **4 + 2 + 3 = 9** | Nenhuma | (a), população pequena, mesma classe acima |
| `copyPath`/`_copy_path` (sandbox `--dry-run`) | `update.go:2323` / `update.py:667` | 2 | `Lstat` por nível (recursivo) — já discutido na Wave 0 da REQ irmã | (c) — não herda o buraco de ancestral por construção |
| `.trackfw-attention.json` | `api_attention.{go,js,py}` | 3 | Nenhuma, mas caminho fixo dentro de `roadmap_dir` já resolvido pelo processo, não por entrada de terceiro no momento da escrita | (b), risco residual pequeno — mencionado, não priorizado |

`*discover.js:603` é `writeCIWorkflowForce`, **código morto sem chamador** — já registrado no modelo
de ameaça da Wave 0 irmã (`docs/seguranca/2026-08-30-...md`). Mantenho aqui só para não reabrir a
mesma investigação.

### 1.3 — Por que isto muda o escopo da Wave 1

A REQ e o roadmap tratam a guarda de folha como o alvo. **A família (a) nova (harness global +
geradores de artefato + geradores de hook) é maior em número de sites (187, soma exata das 4 famílias acima) do que a guarda de
folha já conhecida (~10), e é estritamente pior**: a guarda de folha pelo menos recusa quando a
*folha* é link; estas famílias não recusam em nenhuma circunstância. O remédio
resolver-e-afirmar-contenção, se implementado só nos 3 pontos já nomeados pela REQ, deixa as duas
famílias novas — e os dois PoCs acima — intactas. Recomendo que a Wave 1 generalize o remédio para um
helper único por runtime (`assertContained(root, destination)`, chamado antes de todo `WriteFile`/
`writeFileSync`/`write_text` que deriva de `root`/`home`), não uma correção pontual nos 3 sites
citados. Isso é decisão do arquiteto ao particionar a Wave 1, não algo que eu decida aqui — só estou
nomeando o tamanho real do problema.

---

## 2. Modelo de ameaça

**Quem planta o ancestral, com que capacidade:**

1. **Symlink chegando num PR de terceiro.** Um colaborador externo submete um PR que inclui
   `.github` (ou `docs/req`, `vault/`, `.husky/`, `.lefthook/`) como symlink versionado (git suporta
   modo `120000`, exatamente como a sonda de Windows já usa via `update-index --cacheinfo 120000`).
   Se o mantenedor faz merge sem notar (symlinks não aparecem diferente de arquivo normal em muitos
   visualizadores de diff web), o próximo agente que rodar `trackfw req new`/`update`/`init` na
   árvore escreve fora do repositório. **Não precisa de execução de código, só de merge de um link.**
2. **Symlink pré-existente em `$HOME`, para os alvos de escopo global.** Qualquer processo prévio com
   permissão de escrita em `$HOME` (um instalador malicioso, um dotfile manager comprometido, um
   pacote npm de terceiro instalado globalmente) planta `~/.claude` como symlink. Na próxima vez que
   o usuário rodar `trackfw update harness --install-missing`, cada escrita de escopo global
   (SKILL.md, `settings.json` de Claude/Codex/Gemini/Cursor/Copilot/Kiro, os hooks de
   credential-guard e git-branch-guard) sai para onde o link aponta — **inclusive sobre-escrever um
   arquivo controlado pelo atacante que o próprio usuário depois vai executar** (os hooks são scripts
   `0o755`). Isto é escalonamento local low-to-high: quem só tinha escrita em `$HOME` ganha um script
   `chmod 755` com conteúdo que o trackfw escreveu, mas em local à escolha do atacante.
3. **Caso `--dry-run`/sandbox.** Não se aplica aqui: `copyPath`/`_copy_path` já é `Lstat` por nível,
   não herda o buraco de ancestral (item 1.2, classe (c)).

**O que o atacante ganha:** escrita arbitrária de conteúdo (na maioria dos casos, controlado pelo
próprio *template* do trackfw, não por entrada externa — então não é injeção de conteúdo arbitrário
de terceiro) em um caminho arbitrário fora da árvore esperada, escolhido pelo atacante via o alvo do
symlink. Para os alvos de hook (harness credential-guard/git-branch-guard), o conteúdo escrito é um
script executável `0o755` — o ganho é maior porque o *próximo* passo (o hook disparar) já é execução.

**Quem esvazia esta Wave 0 sem quebrar regra escrita:** alguém que implementasse a Wave 1 só nos 3
sites citados pela REQ (a leitura literal do texto, não da intenção) fecharia exatamente o caso que a
REQ nomeou e deixaria os 187 sites novos abertos — teria cumprido a letra da REQ sem fechar a
ameaça. É a mesma dinâmica descrita no precedente da REQ (`copyPath`, `writeCIWorkflowForce`): a
enumeração inicial sempre está incompleta porque foi feita pelo mecanismo errado (`grep ModeSymlink`
em vez de primitivo de escrita).

---

## 3. Falsificação nas duas direções — inclusive o controle

### 3.1 — O que quebra quando a guarda **não** existe (direção já provada, seção 1.0)

PoC A e PoC B, seção 1.0. Escrita silenciosa fora da árvore, `exit 0`, nenhuma mensagem.

### 3.2 — O que quebra quando a guarda **super-dispara** (a direção que a REQ pediu para eu nomear)

Duas causas concretas, ambas medidas nesta sessão, não hipotéticas:

**(i) Resolver o caminho completo falha quando o arquivo ainda não existe — e criar arquivo novo é o
caso dominante desta REQ (`req new`, `roadmap new`, `adr new`, `note new`, `claude-skill` no primeiro
`--install-missing`).**

Medido:
```go
filepath.EvalSymlinks("/private/tmp/.../realparent/new-file-does-not-exist.md")
// → erro: "no such file or directory" — MESMO com o diretório-pai existindo de verdade
filepath.EvalSymlinks(filepath.Dir("/private/tmp/.../realparent/new-file-does-not-exist.md"))
// → resolve normalmente, sem erro
```
Node reproduz o mesmo formato de falha:
```js
fs.realpathSync(fullPath)        // ENOENT quando a folha não existe
fs.realpathSync(path.dirname(fullPath))   // resolve sem erro
```
**Consequência para a Wave 1:** se a implementação chamar `EvalSymlinks`/`realpathSync` no caminho
completo do arquivo a ser criado — a leitura ingênua de "resolver o caminho" — toda criação de
arquivo novo passa a falhar com erro de I/O disfarçado de recusa de segurança, mesmo sem link algum
na árvore. **A especificação correta é: resolver o diretório-pai, depois juntar a folha antes de
comparar contra `root`.** Isto tem que estar escrito explicitamente na Wave 1, não inferido.

**(ii) Comparar destino resolvido contra `root` NÃO resolvido produz falso positivo em qualquer
projeto cujo caminho contém um symlink legítimo de sistema.**

Medido (macOS, mas o padrão — algum prefixo do path do usuário sendo symlink — não é exclusivo de
macOS: dotfile managers como `chezmoi`/`stow` fazem o mesmo em `$HOME` no Linux):
```
$ python3 -c "import os; print(os.path.islink('/tmp'))"
True
$ python3 -c "import os; print(os.path.realpath('/tmp'))"
/private/tmp
```
```python
root = '/tmp/claude-501/x/poc-project'                      # forma não resolvida (ex.: os.getcwd())
destination = os.path.realpath('/tmp/.../poc-project/docs/req/X.md')   # guarda resolve só o destino
destination.startswith(root)                    # → False — RECUSA operação 100% legítima
destination.startswith(os.path.realpath(root))   # → True  — correto, resolvendo os DOIS lados
```
**Consequência para a Wave 1:** a comparação de contenção tem que resolver `root` **e** `destination`
antes de comparar — nunca um resolvido contra o outro cru. Sem isto, todo desenvolvedor rodando o
trackfw a partir de um caminho sob `/tmp` no macOS (comum para projetos de teste/scratch), ou com
`$HOME` gerenciado por dotfile manager, recebe recusa em operação legítima, sem link malicioso algum
envolvido. **Este é o caso simétrico mais concreto que existe — não é hipotético, é o próprio
ambiente onde rodei os PoCs desta sessão.**

**(iii) Divergência de tolerância a caminho inexistente entre runtimes — risco para AC5 (paridade
exata).**

Medido:
```
Go   filepath.EvalSymlinks(caminho-completo-com-folha-inexistente)  → ERRO
Node fs.realpathSync(caminho-completo-com-folha-inexistente)        → ERRO (ENOENT)
Py   Path(caminho).resolve(strict=False)                            → NÃO erra, resolve o que existe
                                                                        e mantém a folha inexistente
```
Python **não precisa** do padrão "resolver só o diretório-pai" — `resolve(strict=False)` já tolera
folha inexistente nativamente. Go e Node precisam. **Se a Wave 1 usar a mesma sequência de passos nos
3 runtimes ("resolver o caminho completo, then comparar"), o Python se comporta OK e Go/Node quebram
em criação de arquivo novo — divergência que rebate direto no AC5 (mesma recusa, mesma mensagem nos
3 CLIs).** A REQ já previu isto ao nomear a tripla `EvalSymlinks`/`realpathSync`/`Path.resolve()`
runtime por runtime em vez de uma fórmula única — este achado é a evidência de que a diferença de
sequenciamento tem que estar escrita, não implícita.

### 3.3 — O que NÃO pode ser recusado (nomeado explicitamente, como a REQ pediu)

**A resposta honesta ao exemplo que o KG nomeou (`.github` apontando para um diretório
compartilhado) é: depende de PARA ONDE ele aponta, e a forma resolver-e-afirmar-contenção QUEBRA a
variante mais comum do padrão real.**

- **Variante que continua funcionando:** `.github`/`docs/req` symlink para um diretório **dentro do
  mesmo `root`** (ex.: monorepo com `docs/req` → `../shared/req`, ambos sob a mesma árvore
  gerenciada). Resolver-e-afirmar-contenção não recusa isto, porque o alvo resolvido continua sob
  `root` — comportamento correto, e é a razão pela qual a forma escolhida (contenção geográfica
  pós-resolução) é superior a "recusar qualquer `ModeSymlink` de ancestral, ponto".
- **Variante que a REQ tinha em mente e que QUEBRA:** o uso real mais comum de "`.github` compartilhado
  entre projetos" é um symlink apontando **para FORA da árvore de cada projeto individual** — um
  diretório central de templates de CI compartilhado entre múltiplos repositórios no mesmo disco
  (`~/org-templates/.github` linkado em cada checkout), ou um submódulo/worktree externo. **Sob
  resolver-e-afirmar-contenção, isto é indistinguível do PoC A da seção 1.0** — o destino resolvido
  não está sob `root`, então a guarda recusa. Não existe forma de a guarda diferenciar "link
  legítimo apontando para fora, mantido deliberadamente pelo usuário" de "link malicioso plantado
  por um PR de terceiro" só olhando o caminho resolvido — a informação que distingue os dois (quem
  colocou o link e com que intenção) não está disponível na chamada de sistema.
  **Esta é uma quebra de comportamento aceita, não um efeito colateral escondido.** A Wave 1 tem que
  decidir entre (a) aceitar a quebra e garantir que a recusa é audível e nomeia o caminho resolvido e
  o motivo (AC4 já exige isto), para que o usuário legítimo entenda o que aconteceu e possa decidir
  substituir o link por um mount real ou por conteúdo copiado; ou (b) um opt-out explícito (ex.: uma
  flag ou entrada de config que lista ancestrais confiáveis fora de `root`) — que é escopo novo, não
  implícito nesta REQ, e que o arquiteto deve aprovar antes de a Wave 1 implementar, porque abre uma
  segunda superfície (quem pode escrever a lista de exceções).
- **Todo projeto cujo `root` (ou algum prefixo do caminho até ele) é, ele mesmo, um symlink de
  sistema/gerenciador de dotfiles** — `/tmp`→`/private/tmp` no macOS, `$HOME` gerenciado por
  `chezmoi`/`stow`/`GNU Stow` no Linux, workspace de CI montado via bind-symlink. Coberto pelo
  achado 3.2(ii): resolver os dois lados antes de comparar. Esta variante **não pode** ser recusada e
  não há trade-off — resolver os dois lados corrige sem quebrar nada de legítimo.
- **Criação de arquivo novo em diretório existente e não-link** — o caso dominante de toda a
  população nova encontrada (seção 1.2). Coberto pelo achado 3.2(i): resolver o pai, juntar a folha.
  Também não há trade-off aqui — é puramente um bug de sequenciamento a evitar, não uma escolha.

---

## 4. Residual declarado

1. **TOCTOU entre resolver e escrever não é eliminado por esta forma — é aceito, não resolvido.** A
   REQ já descartou `Lstat` por componente por ter a mesma janela; resolver-e-afirmar-contenção tem a
   **mesma** janela (resolve → depois escreve; um atacante que já tem capacidade de trocar um
   ancestral por symlink entre as duas chamadas ainda vence). Dado o modelo de ameaça (seção 2): o
   atacante já precisa ter capacidade de escrever na árvore do projeto ou em `$HOME` para plantar o
   link em primeiro lugar — a mesma capacidade que TOCTOU exigiria para explorar a janela. Não é
   mitigação adicional necessária para fechar esta REQ; é residual aceito e deve ser dito assim no
   roadmap, não silenciado.
2. **`root` em `rejectSymlinks`/`assertNoSymlinks` (o padrão já correto, classe (c)) não verifica se
   o próprio `root` é symlink** — o laço para quando `current == root` sem chamar `Lstat` nele. Não é
   o defeito desta REQ (o `root` normalmente vem de `os.Getwd()`/`os.homedir()`, não de entrada de
   terceiro no momento da chamada), mas é a mesma classe de "ancestral não olhado" um nível acima.
   Meu veredito: **fora de escopo desta REQ**, registrar como observação para o arquiteto decidir se
   vira REQ própria.
3. **Delta de contagem Go (80 medido vs 85 citado pela REQ) não reconciliado** — não muda nenhuma
   conclusão, mas devo declarar que não fechei os 5 números faltantes.
4. **`.trackfw-attention.json`** (classe (b), sem checagem, mas caminho fixo dentro de `roadmap_dir`
   já resolvido pelo processo) — risco residual pequeno, não priorizei investigação mais funda porque
   não é escrita repetida de artefato versionado, é sinal efêmero de UI. Nomeando para o arquiteto
   decidir se entra no escopo da Wave 1 ou fica de fora deliberadamente.
5. **Não tentei enumerar exaustivamente `internal/commands/` e os pares Node/Python além dos pontos
   já citados** (fiz varredura ampla + spot-check, não arquivo-por-arquivo dos ~228 sites brutos). A
   classificação (a)/(b)/(c) desta seção cobre as famílias, não cada um dos 228 pontos individuais —
   suficiente para decidir o escopo da Wave 1, mas a Wave 1 deve rodar o mesmo grep de novo ao
   particionar os MLs, não confiar cegamente nesta lista (mesmo aviso que a REQ me deu sobre a lista
   dela).
6. **PoCs rodados só em macOS** — não testei Linux nem Windows. O defeito da guarda de folha (REQ)
   está declarado como universal por natureza do `Lstat`, não por medição em 3 SOs; aceito a mesma
   inferência aqui, mas é inferência, não medição direta em Linux/Windows.

---

## Veredito sobre a decisão já tomada (resolver-e-afirmar-contenção)

**Não contesto a forma.** Compõe com `rejectSymlinks`/`assertNoSymlinks`/`_reject_symlinks` (classe
(c) já existente e correta) sem introduzir uma segunda checagem conflitante — a mesma razão pela qual
a REQ a escolheu se sustenta depois de eu ter procurado ativamente por um motivo para discordar.

**Contesto a soletração, com dois achados medidos que têm que virar requisito explícito da Wave 1,
não nota de rodapé:**

- **Resolver o diretório-pai da folha, depois juntar a folha — nunca resolver o caminho completo do
  arquivo a criar.** Medido: `EvalSymlinks`/`realpathSync` erram sobre folha inexistente, e criação
  de arquivo novo é o caso dominante de toda a população encontrada nesta Wave 0.
- **Resolver `root` e `destination` antes de comparar — nunca comparar um resolvido contra o outro
  cru.** Medido: `/tmp`→`/private/tmp` no macOS já produz falso positivo com um `root` não resolvido,
  sem link malicioso algum envolvido.

Um terceiro achado, de sequenciamento por runtime (Python tolera folha inexistente nativamente, Go e
Node não), não muda a forma — só confirma que a REQ acertou em nomear uma tripla de primitivas
diferentes por runtime em vez de uma fórmula única, e que a Wave 1 tem que soletrar a sequência de
passos por runtime, não assumir que "resolver o caminho" significa a mesma chamada nos 3.

**Achado adicional não previsto pela REQ:** a população de escrita sem checagem alguma (harness
global + geradores de artefato de projeto + geradores de hook, 187 sites, seção 1.2) é maior e mais
grave que a guarda de folha nomeada pela REQ, e comprovada explorável ao vivo nas duas frentes (seção
1.0). Recomendo ao arquiteto decidir explicitamente se a Wave 1 cobre só os 3 sites da folha (fecha a
letra da REQ, deixa os PoCs A e B abertos) ou generaliza para um helper único chamado por toda
escrita derivada de `root`/`home` (fecha a ameaça real, exige repartição maior da Wave 1). Esta
decisão de escopo não é minha para tomar aqui — é exatamente o tipo de coisa que o arquiteto decide
ao particionar a Wave 1 a partir desta enumeração.
