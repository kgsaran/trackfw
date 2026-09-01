# Barreira de qualidade — PR #234, escrita atômica do CLI Python no Windows

> Autor: `hefesto-tf` | Data: 2026-09-01 | Escopo: `git diff origin/main...HEAD` da branch
> `fix/escrita-atomica-do-cli-python-funciona-no-windows`. Parecer de qualidade, nenhuma linha de
> código de produto tocada por este documento.

## Veredito

**APROVA.**

Nenhum achado bloqueante. `make quality` completo: **`MAKE_EXIT=0`** (rodado até o fim nesta sessão,
ver seção "Execução"). Um achado de acompanhamento (não bloqueante) sobre o residual entre o literal
`0o644` do teste e a constante de produção em `manager.py:595`, registrado no item 1 abaixo.

---

## 1. O controle não é vácuo — confirmado, com uma precisão sobre o que exatamente é exercitado

**Pergunta central da tarefa: `manager.py:343`/`:358` (`mode=0o644`) é o alvo, e o resultado (não a
chamada) é o que se afirma?**

**Precisão necessária antes de responder:** essas duas linhas do roadmap (ML-0A, escrito antes do
ML-1A adicionar o doc-comment em `identity/__init__.py`, que deslocou o arquivo) já não correspondem
ao arquivo atual — hoje `_atomic_write` está definida em `manager.py:116`, e os dois call sites em
produção que a alimentam ficam em `manager.py:353` e `:368` (`self._atomic_write(destination/filename,
content, mode)`), não `:343`/`:358`. Isto é deriva de documentação do roadmap (números de linha que
andaram ~10-13 linhas por causa do doc-comment inserido depois), não um defeito de código — mas o
roadmap deveria ser atualizado para não citar números que já não batem, ver achado de acompanhamento
no fim desta seção.

O que o teste de fato exercita é a **função** `IntegrationManager._atomic_write` chamada com o
literal `mode=0o644` — não o call site de produção que fornece esse valor. Esse literal só tem
significado como controle não-vácuo porque reproduz o valor que `manager.py:595`
(`pending = (destination, plan["content"], 0o644)`) de fato usa hoje — mas, por leitura, nada no
teste deriva o valor de `:595` ou importa essa constante; a ligação entre os dois é só a coincidência
de hoje os dois usarem `0o644`. **Este é o residual real da não-vacuidade**: o controle prova "a
função, chamada com `0o644`, aplica `0o644` via fallback" — uma proposição verdadeira e não-vácua em
si — mas a ponte entre esse literal e a constante de produção não está testada por nada. Não é
bloqueante (o valor de hoje está correto e a mudança de `0o644` seria, por si, uma decisão de produto
rara e visível em qualquer revisão), mas é o tipo de acoplamento silencioso que a Wave 0 já tinha como
princípio evitar.

Confirmado por leitura de `pypi/tests/test_atomic_write_windows_fallback.py:78-92`
(`test_manager_0o644_fallback_produces_observable_mode`):

```python
monkeypatch.delattr(os, "fchmod", raising=False)
target = tmp_path / "artifact.txt"
IntegrationManager._atomic_write(target, b"payload", 0o644)
assert target.read_bytes() == b"payload"
if os.name == "posix":
    observed = stat.S_IMODE(target.stat().st_mode)
    assert observed == 0o644, f"fallback did not apply requested mode: got {oct(observed)}"
```

- O único site de chamada exercitado é `IntegrationManager._atomic_write` com `mode=0o644` — o único
  dos 7 onde `os.fchmod` teria efeito observável (os outros 6 pedem `0o600`, que `tempfile.mkstemp`
  já entrega por padrão; um teste que apenas verificasse "`fchmod` foi chamado" nesses 6 sites
  passaria sem provar nada, exatamente o vício que a tarefa pediu para procurar).
- A asserção é sobre o **resultado** (`stat.S_IMODE(target.stat().st_mode) == 0o644`), não sobre a
  chamada — é o oráculo certo: prova que o modo pedido efetivamente chegou ao disco pelo caminho de
  fallback (`os.chmod(temporary, mode)`), não que uma função qualquer foi invocada.
- A restrição a `os.name == "posix"` para a asserção de bits exatos é justificada e documentada no
  próprio arquivo (linha 31-34): NTFS só honra o bit de escrita, `0o644` pode legitimamente voltar
  como `0o666` lá — a asserção de bits ficaria falso-vermelha no Windows por um motivo alheio ao
  defeito. A asserção de "escreveu sem lançar `AttributeError`" (linha 89) permanece incondicional em
  todas as plataformas, cobrindo o Windows real.

**Varredura por vacuidade no restante do arquivo (7 testes, verificados um a um):**

| teste | o que afirma | vácuo? |
|---|---|---|
| `test_identity_atomic_write_survives_missing_fchmod` | conteúdo gravado corretamente com `fchmod` removido | Não — exercita o caminho de fallback ponta a ponta e confere o resultado |
| `test_quarantine_atomic_write_survives_missing_fchmod` | idem, em `quarantine.py` | Não |
| `test_manager_atomic_write_survives_missing_fchmod` | idem, em `manager.py` (site não-vácuo) | Não |
| `test_manager_0o644_fallback_produces_observable_mode` | `st_mode` final == `0o644` (POSIX) + escrita íntegra (todas) | Não — é o controle central, tratado acima |
| `test_manager_0o644_uses_fchmod_not_chmod_on_posix` | `os.fchmod` chamado exatamente 1x, `os.chmod` nunca, quando `fchmod` existe | Não — é a direção simétrica (b): fallback não deve disparar por engano em POSIX |
| `test_identity_uses_fchmod_not_chmod_on_posix` | idem, em `identity.py` | Não |
| `test_quarantine_uses_fchmod_not_chmod_on_posix` | idem, em `quarantine.py` | Não |

Nenhum dos sete é vácuo pelo mecanismo do achado 12 (asserção que passa sem nunca avaliar a garantia
real). Os três testes de "sobrevive à ausência" em `identity`/`quarantine` não têm o mesmo problema
dos 5 sites `0o600` porque não afirmam nada sobre o **modo** — afirmam apenas que o conteúdo chega ao
disco sem `AttributeError`, que é uma garantia real e diferente (a garantia de crash zero, não a de
modo). Rodei a suíte isolada: `pytest tests/test_atomic_write_windows_fallback.py -q` → `7 passed`.

**Dois achados de acompanhamento, não bloqueantes:**

1. **Residual do literal `0o644` sem ponte testada até `manager.py:595`**, tratado acima — sugiro um
   ML pequeno de acompanhamento: **não** um teste que leia o literal de `:595` via `inspect`/AST e o
   compare textualmente a `0o644` (isso seria uma constante validada contra si mesma, restated —
   passa independentemente de o fallback funcionar, exatamente a forma "verifica a chamada/o texto,
   não o resultado" que o ML-0A já rejeitou). A correção com oráculo real é exercitar o caminho de
   produção ponta a ponta — `_plan_artifact_write` → o `pending` com `0o644` → `_atomic_write` — e
   assertar o `st_mode` resultante, como já se faz para a função isolada; assim a constante de
   produção é validada pelo efeito que produz, não por eco textual.
2. **Docstring do arquivo (linhas 9-17)** afirma que `monkeypatch.delattr(os, "fchmod",
   raising=False)` faz o teste ser "evidência real de CI no `windows-full-suites`" porque, no
   Windows, o delattr seria um no-op já que `os.fchmod` não existe lá. Verifiquei
   `pypi/tests/conftest.py` por inteiro: o único fixture ali (`_isolated_home_for_test_session`,
   session-scoped, autouse) isola `$HOME`, não toca `os.fchmod`/`os.chmod` — não há nada que
   interfira com a premissa do docstring. O raciocínio se sustenta. Sugestão de frase adicional
   (documentação, não teste): citar explicitamente que a premissa é "nenhum fixture anterior neste
   processo já monkeypatchou `os` antes desta suíte rodar" — evita que um futuro fixture global de
   spy quebre silenciosamente a alegação de "não é simulação no Windows".
3. **Atualizar os números de linha do roadmap** (`manager.py:343`/`:358` → `:353`/`:368` hoje) — a
   Wave 0 registrou coordenadas exatas como parte do veredito, e elas já ficaram obsoletas com o
   próprio ML-1A que o roadmap gerou.

---

## 2. O skip não anula a verificação — confirmado

`@pytest.mark.skipif(not _HAS_FCHMOD, reason=...)` está aplicado **por teste**, não a nível de
módulo, e apenas nos 3 testes da direção (b) — `test_manager_0o644_uses_fchmod_not_chmod_on_posix`,
`test_identity_uses_fchmod_not_chmod_on_posix`, `test_quarantine_uses_fchmod_not_chmod_on_posix`
(linhas 106, 132, 156). Os outros 4 (direção a, incluindo o controle central do item 1) não têm
nenhum decorador de skip — rodam incondicionalmente em qualquer plataforma, inclusive Windows.

Confirma a alegação da tarefa: os 4 restantes **rodam** em Windows — e, pelo mesmo mecanismo do item
1, **provam algo lá**: no Windows, `os.fchmod` já está ausente nativamente (não é simulação), então
`monkeypatch.delattr(..., raising=False)` é um no-op e os 4 testes exercitam o caminho de fallback
real da plataforma real, verificando (a) ausência de `AttributeError` nos três `_atomic_write` e (b)
que o conteúdo/modo aplicado pelo fallback está correto. Isto é evidência de CI genuína no runner
`windows-full-suites`, não uma simulação caveada.

O `hasattr(os, "fchmod")` como condição do skip (em vez de `sys.platform`/`os.name`) é a escolha
certa pela mesma razão nomeada no roadmap: é condição medida, não palpite de plataforma — cobre
também qualquer ambiente exótico onde `os.fchmod` esteja ausente por outro motivo que não "é
Windows".

**Confirmação de que `windows-full-suites` de fato coleta este arquivo (não é suficiente que ele não
tenha skip a nível de módulo — é preciso que o job o inclua na coleta).** A evidência real é
`.github/workflows/quality.yml:305`: o job roda `python -m pytest pypi/tests -q` — coleta o pacote
inteiro, sem `--ignore`/filtro de caminho que pudesse excluir o arquivo novo. A aritmética do
enunciado é corroboração, não prova por si: linha de base `145 failed / 1422 passed` (1567 coletados)
→ medido neste PR `103 failed / 1477 passed` (1580 coletados) — **+13 testes coletados**. Este PR
adiciona exatamente 7 testes Python novos (`git diff --stat`: um único arquivo de teste novo,
`pypi/tests/test_atomic_write_windows_fallback.py`, 177 linhas). Não identifiquei a origem dos outros
6 do delta — fora do escopo desta seção, e não invento a fonte: o fato verificável é que o delta é
compatível com "os 7 deste arquivo foram coletados" (não seria compatível com "0 novos" nem com um
delta negativo), não uma prova isolada da coleta — quem prova é a linha do workflow sem filtro.

---

## 3. O gate anti-divergência — falsificado de forma independente

Reproduzi as quatro direções em cópia isolada (`scratchpad/atomicgate`, nunca na árvore real do
repositório), sem depender do relato do roadmap:

| # | sabotagem | resultado observado | esperado |
|---|---|---|---|
| 1 | árvore correta (cópia fiel das três) | `OK — 3 cópias ... idêntico após normalização`, exit 0 | passa |
| 2 | diverge só o texto do comentário em `quarantine.py` ("without it" → "lacking it, e.g.") | `DIVERGÊNCIA ... referência: identity/__init__.py / diverge: thirdparty/quarantine.py`, exit 1 | reprova, nomeia o arquivo divergente corretamente **neste caso** |
| 3 | muda a indentação **relativa** do `if`/`fchmod(...)` dentro do bloco (2 espaços a mais só na linha do `fchmod(descriptor, mode)`) em `quarantine.py`, mantendo o deslocamento uniforme externo | `DIVERGÊNCIA`, mesmo formato, exit 1 | **este é o teste direto do risco "dedent mascara divergência real"** — resultado confirma que `textwrap.dedent` só remove o prefixo comum a **todas** as linhas do bloco; não normaliza indentação relativa entre linhas dentro do bloco, então uma mudança estrutural real (offset relativo do `if`/corpo) continua detectável |
| 4a | `ROOT` apontando para diretório vazio | nomeia os 3 arquivos ausentes, "não é possível comparar", exit 1 | reprova por vacuidade nomeada |
| 4b | âncora removida em `manager.py` (`fchmod = getattr(os, "fchmod", None)` trocado por `has_fchmod = hasattr(os, "fchmod")`) | `extração falhou em .../manager.py — âncora ... não encontrada` **+** `vacuidade — esperava extrair 3 blocos, extraiu 2`, exit 1 | duas mensagens independentes, reprova em vez de comparar 2 de 3 silenciosamente |

**Sobre o risco específico pedido ("a normalização com `textwrap.dedent` mascara alguma divergência
real"):** não encontrei nenhum caso em que mascare. `textwrap.dedent` calcula o prefixo de espaços
comum a **todas** as linhas não-vazias do bloco e o remove — isso é exatamente o que se precisa para
tolerar o deslocamento uniforme de `integrations/manager.py` (bloco a 12 espaços, `@staticmethod`
dentro de classe) contra as duas funções de módulo (8 espaços), sem apagar a estrutura relativa
`if`/`else`/corpo. A sabotagem 3 acima é o teste direto disso: uma mudança de indentação **relativa**
dentro do bloco (não um deslocamento uniforme) permanece detectável porque deixa de ser um prefixo
comum a todas as linhas — continua sendo comparada literalmente após o dedent e diverge. O único jeito
de escapar da comparação seria reescrever o bloco além da âncora reconhecível, e isso já cai na
guarda de vacuidade (direção 4b), não na comparação.

Concordo com a auto-crítica que o gate já registrou no seu próprio cabeçalho (linhas 36-58) sobre por
que não usar `assert_has` de string fixa (padrão de `check-ref-separator-portability.sh`, o mesmo
gate onde este papel já achou `assert_has` passando com 1 de 2): comparação três-contra-golden-fixo
seria estritamente mais fraca aqui — provaria "cada cópia bate com um texto congelado dentro do
gate", não "as três concordam entre si", e produziria falso positivo a cada reformatação inofensiva
do comentário nas três ao mesmo tempo. A comparação três-contra-as-outras-duas é a propriedade certa
para a garantia que a REQ pede.

**Achado de acompanhamento, não bloqueante — a mensagem "diverge: X" nomeia o comparado, não
necessariamente o editado.** O gate compara sempre contra `items[0]` (a primeira entrada do dict de
blocos, que segue a ordem de `FILES` no script — `identity/__init__.py`) como referência implícita, e
lista como "diverge" qualquer uma das outras duas que não bater com ela. Isso é preciso quando o
arquivo editado é um dos dois últimos (sabotagens 2 e 3 acima) — mas se o arquivo editado for
`identity/__init__.py` (o próprio baseline), a saída inverte: nomeia os **outros dois** como
divergentes, não o que de fato mudou. Reproduzi isso deliberadamente: diverg indo só
`identity/__init__.py`, a saída foi `referência: identity/__init__.py` / `diverge: quarantine.py` +
`diverge: manager.py` — o gate ainda reprova corretamente (nenhum falso `OK`), mas quem lê a mensagem
sem saber qual arquivo foi tocado por último pode gastar tempo inspecionando os dois arquivos
"apontados" antes de perceber que o terceiro (o baseline) é o que mudou. Não é vacuidade — é uma
imprecisão de diagnóstico, sem efeito na garantia de segurança que o gate protege (ele sempre reprova
quando há divergência real, goal da REQ). Sugestão de baixo custo: mencionar no `stderr` que a
referência é arbitrária (primeira da lista `FILES`), não necessariamente a versão "correta" — evita
que quem lê o erro presuma isso.

**Verificação adicional — o gate está de fato ligado e roda com o `ROOT` certo em produção.**
`Makefile:58` chama `scripts/check-atomic-write-anti-divergence.sh` sem argumento, dentro do alvo
`parity:` — `ROOT` fica `"."`, resolvido contra o cwd do `make` (raiz do repositório). Não há
divergência entre o `ROOT` usado pela guarda de existência e pela extração real (ambas usam a mesma
variável `ROOT` do script) — o defeito nomeado no cabeçalho do gate ("uma guarda olha um cwd, a
varredura real olha outro") não está presente aqui.

Nenhum achado bloqueante nesta seção.

---

## 4. O contrato em `docs/cli-parity.md`

Seção "Escrita atômica — chmod no descritor vs. chmod no caminho" (linhas 6425-6481) e subseção
"Triplicação deliberada no Python — não extraída, gateada" (6454-6481).

- **Afirma o Node não cumpre?** Sim, com arquivo e linha. Confirmado por leitura direta do código
  Node atual (não do relato):
  - `npm/src/thirdparty/quarantine.js:29` — `fs.chmodSync(tmp, mode)` (chmod sobre o **caminho**,
    nunca `fchmodSync(fd)`, que existe no runtime Node e não é usado). O contrato cita "28-30"; a
    chamada real está na linha 29 — dentro do intervalo citado, correto.
  - `npm/src/integrations/manager.js:95` e `:97` — `fs.chmodSync(tmp, mode)` seguido de
    `fs.chmodSync(file, mode)` **depois do `rename`** (linha 96). O contrato cita "94-97" e nomeia
    explicitamente a segunda chamada pós-rename como janela extra que `identity.js` não tem —
    confirmado: `identity.js` (grep) não tem chamada equivalente pós-rename.
  - A tabela (linha 6434-6438) marca Go ✅, Python ✅ (com a ressalva "em POSIX"), Node ❌ — não há
    nenhuma célula afirmando garantia que o código não sustenta.
  - **A célula Go é a afirmação mais forte da tabela ("✅ sem janela", sem ressalva de plataforma) —
    verifiquei as três cópias Go, não só a citada no diff.** `internal/identity/identity.go:99` e
    `internal/integrations/manager.go:731` usam `temporary.Chmod(mode)` (descritor). A terceira,
    `internal/thirdparty/quarantine.go`, **não aparece no diff desta REQ** (Go não foi tocado por
    este PR) e por isso eu não a tinha lido ainda neste parecer — grep direto confirma
    `quarantine.go:152`: `temporary.Chmod(mode)`, mesma forma baseada em descritor. As três cópias Go
    concordam com a célula ✅ da tabela; se uma delas usasse `os.Chmod(path, ...)` a tabela estaria
    fazendo uma afirmação falsa para Go inteiro (o mesmo padrão de erro que a AC6 original do ML-0A
    evitou por pouco para o Node) — não é o caso.
- **A anotação `partial=` declara honestamente o não-medido?** Sim. Duas anotações
  `trackfw-contract` na seção (linhas 6427 e 6456): a primeira nomeia explicitamente
  `partial=cobre só a não-divergência das três cópias Python entre si; não mede Go nem Node, nem a
  janela pré-existente do os.replace(path)` — isto é preciso: o gate realmente só compara as três
  cópias Python entre si (confirmado na seção 3), não toca Go/Node, e a seção "Residual aceito"
  (6448-6452) nomeia à parte a janela do `os.replace`/`fs.renameSync`/`os.Rename` final como
  pré-existente e fora do escopo desta REQ — consistente com o que o gate de fato verifica.
- `scripts/check-parity-contract-coverage.sh` (rodado dentro de `make quality`, ver seção "Execução"
  abaixo) valida estruturalmente que toda seção anotada bate com um gate real e todo gate tem
  anotação — não depende só da minha leitura.

Nenhum achado bloqueante nesta seção.

---

## 5. Cobertura e manutenibilidade

- **Triplicação do bloco de fallback nos três `_atomic_write`.** É deliberada (Wave 0 do roadmap;
  `quarantine.py:34-37` já documentava a razão antes desta REQ; `identity/__init__.py` ganhou o
  mesmo doc-comment nesta REQ, fechando a assimetria em que só uma das três explicava a não-extração)
  e está protegida por gate estrutural (seção 3). Não é dívida técnica silenciosa — é uma decisão
  registrada com controle compensatório. Sem achado aqui.
- **`getattr(os, "fchmod", None)` como guarda de capacidade** é o padrão correto para portabilidade
  condicionada à API, não ao nome da plataforma — consistente nas três cópias (confirmado pelo próprio
  gate, e por leitura direta do diff).
- **Doc-comment novo em `identity/__init__.py` (linhas 93-100 do diff).** Replica quase literalmente
  o texto de `quarantine.py`, nomeando as outras duas cópias por caminho — correto e no padrão já
  usado pelo `atomicWrite` do Go/Node (`internal/thirdparty/quarantine.go:87`,
  `npm/src/thirdparty/quarantine.js:1-3`) para o mesmo tipo de replicação deliberada. Nenhum achado.
- **`docs/req/REQ-2026-09-01-cli-node-usa-chmodsync-...`** existe e está referenciada corretamente a
  partir do contrato — não é uma REQ prometida e ausente. Confirmei por `ls docs/req/`.

Nenhum achado bloqueante nesta seção.

---

## Execução de `make quality`

Rodado sem pipe, exit code capturado diretamente pelo shell (`(make quality; echo "MAKE_EXIT=$?")`,
saída redigida para arquivo e monitorada até o processo terminar, sem interromper no meio).

```
$ make quality
...
GO_BIN=bin/trackfw scripts/check-roadmap-barrier-contract.sh
check-roadmap-barrier-contract: 53 cenários OK
scripts/check-ref-separator-portability.sh
check-ref-separator-portability: OK — 18 assinaturas de escrita/leitura portavel confirmadas
scripts/check-atomic-write-anti-divergence.sh
check-atomic-write-anti-divergence: OK — 3 cópias (pypi/trackfw/identity/__init__.py,
pypi/trackfw/thirdparty/quarantine.py, pypi/trackfw/integrations/manager.py) com bloco de fallback
idêntico após normalização
MAKE_EXIT=0
```

**`MAKE_EXIT=0` — acompanhei até o fim nesta sessão.** A cadeia completa (Go `go vet`/`go
build`/`go test`, `npm test`, `pytest pypi/tests`, e os mais de 30 scripts de `parity:`, incluindo
`check-cli-parity.sh`, `check-roadmap-barrier-contract.sh` e o gate novo
`check-atomic-write-anti-divergence.sh` na posição em que o `Makefile` o lista) terminou sem nenhum
`FAIL` no log e o gate-alvo desta REQ imprimiu `OK` dentro da própria execução de `make quality`
(último comando antes do `MAKE_EXIT=0`), não apenas isolado como nas seções 1-3 acima.

`git status --porcelain` após o run mostra só os artefatos deste parecer e do parecer paralelo de
`hades-tf` em `docs/seguranca/` (arquivo dele, não tocado por mim) — nenhum arquivo de `pypi/`,
`internal/`, `npm/`, `scripts/` ou `Makefile` foi mutado durante a janela do `make quality`, o que é
consistente com o gate `falsify/no-repo-mutation` não ter disparado.

---

## Resumo para o handoff

**Veredito: APROVA.**

Nenhum achado bloqueante nos cinco pontos pedidos:
1. Controle na função `IntegrationManager._atomic_write` (chamada com `mode=0o644`, o único site
   não-vácuo dos 7) não é vácuo — mira o resultado (`st_mode`), não a chamada; nenhum outro teste do
   arquivo é vácuo pelo mesmo mecanismo. Duas ressalvas de acompanhamento (não bloqueantes): (a) o
   literal `0o644` do teste não tem ponte testada até a constante de produção em `manager.py:595` —
   se ela mudar, o controle continua verde mas deixa de representar produção; (b) os números de linha
   `:343`/`:358` que o roadmap cita já não batem com o arquivo atual (`:353`/`:368` hoje, deriva por
   causa do próprio doc-comment que o ML-1A inseriu).
2. Skip escopado corretamente a 3 dos 7 testes (a direção simétrica, POSIX-only por natureza); os 4
   restantes rodam incondicionalmente, inclusive Windows, e provam algo lá (ausência de
   `AttributeError` + conteúdo/modo corretos pelo caminho de fallback real, não simulado). Confirmado
   que `windows-full-suites` de fato coleta o arquivo pela evidência real —
   `.github/workflows/quality.yml:305` roda `pytest pypi/tests -q` sem filtro de caminho —, com a
   aritmética do CI (1567 → 1580 testes coletados, +13) como corroboração compatível.
3. Gate falsificado de forma independente nas quatro direções pedidas, incluindo o teste direto do
   risco "dedent mascara divergência real" (indentação relativa alterada dentro do bloco continua
   detectável — dedent só remove prefixo comum a todas as linhas, não normaliza estrutura interna).
   Achado de acompanhamento (não bloqueante): a mensagem de divergência nomeia sempre os itens
   diferentes do primeiro arquivo da lista (`identity/__init__.py`, o baseline implícito) — se o
   arquivo editado for o próprio baseline, a mensagem aponta os outros dois em vez do editado; o gate
   ainda reprova corretamente em todos os casos, é só uma imprecisão de diagnóstico.
4. Contrato em `docs/cli-parity.md` nomeia a falha do Node com arquivo e linha exatos (confirmados
   por leitura do código Node atual), e a célula Go (✅ sem ressalva) foi verificada nas três cópias,
   incluindo `internal/thirdparty/quarantine.go:152` (não tocada por este diff, então não citada nele
   — confirmei por leitura direta que também usa `temporary.Chmod`, descritor). A anotação `partial=`
   é precisa quanto ao que o gate não mede.
5. Sem dívida de cobertura/manutenibilidade nova; triplicação é decisão registrada com controle
   compensatório.

`make quality` foi acompanhado até o fim nesta sessão: **`MAKE_EXIT=0`**, sem nenhum `FAIL` no log,
gate-alvo (`check-atomic-write-anti-divergence.sh`) impresso como `OK` dentro da própria execução.
