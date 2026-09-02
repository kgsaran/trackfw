# Modelo de ameaça — saída não-ASCII em console cp1252 (item 4/11 do issue #216)

> ML-0A (Wave 0), roadmap `ROADMAP-2026-09-02-saida-nao-ascii-declara-codificacao-em-script-gerado-e-em-gate.md`
> Autor: Hades (Security Reviewer) · Nenhuma linha de correção escrita — só varredura, medição e veredito.

## 1. Método da varredura

Parti do sintoma, não do mecanismo, e medi antes de generalizar.

**1.1 — Varredura irrestrita.** Todo byte não-ASCII em `scripts/**` (excluindo `testdata/` — corpus
de fixtures de roadmap, não código executável — e `windows-repro/`, que é o próprio harness de
reprodução do bug, não um alvo da varredura):

```
python3 os.walk("scripts") + contagem de bytes > 127 por arquivo
→ 204 arquivos com pelo menos 1 byte não-ASCII (a maioria é o corpus de fixtures em testdata/)
```

**1.2 — Restrito a `.sh` fora de `testdata/`, excluindo linhas de comentário** (`#` como primeiro
caractere não-branco): **746 linhas não-comentário com não-ASCII em 45 scripts**. Amostrei essas
746 linhas: a esmagadora maioria é `echo "...texto com — ou acentos..." >&2` — mensagens de
diagnóstico dos próprios gates, não heredoc Python.

**1.2-bis — o filtro "python3 presente + não-ASCII em algum lugar do arquivo" é superficial demais
para contar população, e eu apliquei esse filtro superficial em §1.4 sem perceber até refazer com
mais precisão.** "não-ASCII em algum lugar do arquivo" conta acentos em comentários bash e em
`echo` fora de qualquer bloco Python — não prova que o `print()`/heredoc Python realmente recebe ou
emite não-ASCII. Refiz de forma cirúrgica: extraí só o **corpo** de cada invocação `python3 - ...
<<'TAG' ... TAG` e `python3 -c '...'`/`"..."` e testei não-ASCII **dentro desse corpo**, separando
por destino do stdout (capturado via `VAR=$(...)`/redirecionado com `>`/`>>`, vs. livre —
stdout/stderr direto para o terminal ou CI):

```
40 arquivos invocam python3
21 têm heredoc/-c localizável pelo extrator (os outros 19 usam sintaxe que o extrator não
   reconheceu — permanecem SEM classificação fina, ver ressalva de cobertura abaixo)
 5 dos 21 têm bytes não-ASCII em algum ponto do CORPO extraído (heredoc/-c inteiro, comentários
   Python inclusos):
   - check-roadmap-barrier-contract.sh   → não-ASCII em DADO impresso (f-string com evidência de
     roadmap), capturado/redirecionado (linha 516, >>"$CORPUS_LINES_FILE") — RISCO REAL, ver §3
   - check-atomic-write-anti-divergence.sh → não-ASCII em DADO impresso (mensagem de erro
     f-string), mas livre (print(..., file=sys.stderr), diagnóstico) — seguro por §4(a)
   - check-identity-parity.sh            → não-ASCII só num COMENTÁRIO `#` dentro do corpo Python;
     o dado de fato impresso (ids de catálogo) é ASCII — falso positivo à luz do sintoma real
   - check-parity-contract-coverage.sh   → idem, comentário Python ('—' em "or whitespace —
     avoids...") — falso positivo
   - check-validate-parity.sh            → o '…' está num comentário BASH acima do bloco Python,
     capturado pelo extrator por estar dentro da janela de busca, não dentro do corpo real —
     falso positivo do extrator
16 dos 21: python3 localizado, zero não-ASCII em qualquer parte do corpo (comentário ou dado)
19: sem detecção de corpo pelo extrator — não incluído em `scripts/trackfw-attention-signal.sh`?
   não: esse ESTÁ nos 21 detectados, com corpo limpo — confirma §2, o texto estático do script
   não tem não-ASCII no corpo Python; o risco de (a) é dado DINÂMICO, que um extrator estático
   nunca vê por natureza (só rodando com entrada real se observa).
```

**O que isso corrige:** dos "40 scripts com heredoc python3 + não-ASCII" do REQ, só **2** têm
não-ASCII genuíno em DADO efetivamente impresso pelo Python — `check-roadmap-barrier-contract.sh`
(risco real, capturado) e `check-atomic-write-anti-divergence.sh` (seguro, livre/stderr) — dentro
do subconjunto de 21 arquivos que meu extrator conseguiu isolar. Os outros 3 achados do filtro
original eram não-ASCII em comentário Python ou em comentário bash fora do corpo, nunca em dado
impresso. **Isso não significa que a correção deva cobrir só 2 scripts** — a REQ pede correção
*uniforme* (AC3/AC5) justamente para não depender de reclassificar cada script à mão a cada
mudança; o ponto é que a *urgência por conteúdo hoje comprovadamente exposto* é muito menor que
"40", e a lista de 40 continua sendo o alvo certo do gate de *prevenção* (AC5), não porque hoje
quebrem, mas porque qualquer um pode passar a imprimir dado não-ASCII amanhã sem que ninguém note
— ver §6. **Ressalva de cobertura, declarada e não fechada:** 19 dos 40 arquivos não tiveram o
corpo Python isolado pelo extrator (sintaxe de invocação fora do padrão `python3 - ... <<'TAG'` /
`python3 -c '...'` que cobri) — para esses, "sem não-ASCII no corpo" na tabela abaixo é uma
inferência mais fraca (herdada do filtro original de arquivo inteiro), não uma medição direta do
corpo. Não tive orçamento nesta sessão para escrever um segundo extrator mais tolerante; registrado
como residual (§5).

**1.3 — O achado que decide a classificação (medido, não inferido):** `echo`/`printf` em bash **não
encodam** — escrevem os bytes literais da fonte do script (UTF-8, já que os `.sh` são UTF-8) direto
para o descritor de saída. Não há passo de "codificar para o charset do console" no caminho de
`echo`. Medi isso diretamente:

```
$ LC_ALL=C LANG=C bash -c 'echo "seções relatório — ✓"; echo "exit=$?"'
seções relatório — ✓
exit=0
```

Nenhum crash, em nenhuma locale que testei. Isso significa que a varredura pelo sintoma bruto
("todo `echo`/`printf` não-ASCII") **infla a população sem mudar o risco de disponibilidade**: ela
encontra ~700 linhas de `echo` que nunca vão morrer por `UnicodeEncodeError`, porque bash não tem
esse mecanismo de falha. O que pode morrer é especificamente **`print()`/`sys.stdout` do Python**,
que faz encode estrito por padrão. Medido lado a lado:

```
$ PYTHONIOENCODING=cp1252 python3 -c "print('seções relatório — ✓')"
UnicodeEncodeError: 'charmap' codec can't encode character '✓' in position 19
exit=1

$ PYTHONIOENCODING=cp1252 python3 -c "
import sys; sys.stdout.reconfigure(encoding='utf-8', errors='replace')
print('seções relatório — ✓')"
seções relatório — ✓
exit=0
```

**Conclusão do método:** a varredura pelo sintoma (item 3 do pedido) foi feita e é mais ampla que a
lista original (~700 linhas de `echo` vs. 40 scripts com heredoc Python) — mas o único primitivo, no
ecossistema deste repositório, que encoda de forma estrita antes de escrever é `python3` via
`print()`/`sys.stdout` (`jq -r` também não crasha por encoding — escreve bytes, sem negociação de
charset). Refazer a varredura original — `python3` presente + não-ASCII em algum ponto do arquivo +
ausência de `reconfigure` — reproduz o mesmo número, 40, com a mesma lista do REQ, **mas essa
reprodução só confirma que o predicado é o mesmo, não que a população de risco real é 40** — ver
§1.2-bis, que refina o filtro para dentro do corpo Python de cada invocação e encontra só 2 scripts
com não-ASCII genuíno alcançando `print()` hoje. A varredura por sintoma segue sendo mais ampla que
a por mecanismo (achou o segundo artefato de produto do §2 e não achou nada crash-capaz fora dos
40), mas o número "40" não deve ser lido como "40 scripts que quebram hoje" — é "40 scripts que
usam o único primitivo capaz de quebrar", o que é a base certa para o gate de prevenção (AC5), não
uma medição de exposição atual.

**Risco residual não eliminado por este raciocínio:** não auditei `awk`/`sed`/`printf %s` com
locale não-C ativa (`LC_ALL` diferente de `C`/`POSIX` pode alterar comportamento de classes de
caractere `[[:alpha:]]` etc. em alguns `sed`/`awk`, mas isso é reordenamento/match, não um
encode-e-lance-exceção; não constitui o mesmo defeito). Ver §6.

## 2. Classificação produto × ferramenta

### (a) Produto — gerado e instalado na máquina de quem adota

Varri os três geradores (`internal/generators/*.go`, `npm/src/generators/*.js`,
`pypi/trackfw/generators/*.py`) por invocação de `python3` dentro de conteúdo que é escrito em
arquivo do projeto do adotante. Resultado: **`python3` aparece em exatamente 3 arquivos Go**
(`claudemd.go`, `scaffold.go`, `update.go`); dos três, só `scaffold.go` contém uma invocação de
`python3` cujo `print()` roda sobre dado, não é comentário — a de `claudemd.go:237` é um comando de
lint (`python3 -c "import pathlib, py_compile; ..."`) sugerido ao usuário em texto de instrução, sem
`print()` de não-ASCII. `update.go:2370` é comentário.

**A `attentionSignalScript` de `scaffold.go` é confirmada como a única fonte, nos três CLIs, de
conteúdo instalado no usuário que invoca `python3 print()`.** Confirmei paridade byte-a-byte da
linha que importa nos três runtimes:

```
internal/generators/scaffold.go:774   MSG=$(echo "$INPUT" | python3 -c "import sys,json; ... print((ti.get('question') or ti.get('command') or 'Agent is executing: '+d.get('tool_name','unknown'))[:300])" 2>/dev/null || echo "Agent needs attention")
npm/src/generators/hooks.js:148       — idêntica, byte a byte
pypi/trackfw/generators/init_gen.py:969-970 — idêntica, byte a byte (função ficava em init_gen.py, não em hooks.py — hooks.py só tem o comentário que a referencia; confirmar isso custou uma busca extra porque `grep hooks.py` deu falso-negativo)
```

**🔴 Correção ao enquadramento do REQ — medido, não assumido:** o REQ descreve "12 caracteres
não-ASCII: ã ç é ê ó ú — ✓" no `attentionSignalScript`. Extraí o literal exato do const Go (do
crase de abertura à de fechamento) e contei:

```
non-ascii count: 1  →  só o '—' da linha de comentário "# trackfw attention signal — PreToolUse/BeforeTool hook"
```

Um comentário `#` **nunca é executado nem impresso** — é bash, não Python, e nem chega a ser lido
pelo interpretador Python (está fora dos blocos `python3 -c "..."`). **O literal estático do script
não tem, hoje, nenhum caractere não-ASCII que passe por `print()`.** A REQ está descrevendo um risco
que não bate com o conteúdo atual — não encontrei outro trecho em `scaffold.go`/`hooks.js`/
`init_gen.py` que combine com "ã ç é ê ó ú" mais "✓" atribuído a este script; ou o número do issue
original mediu uma versão anterior do arquivo, ou mediu outra coisa. Não investiguei `git blame`
completo por estar fora do escopo de correção — deixo isso como item para quem pegar a Wave 1.

**O risco real em (a) não é estático, é dinâmico:** as duas linhas `python3 -c` de `scaffold.go:773-774`
fazem `print()` de **dado vindo do hook JSON em tempo de execução** — `tool_input.question` ou
`tool_input.command`, que é texto arbitrário produzido por um agente de IA (poderia ser "Preciso de
confirmação — seções não batem, ✓ falhou"). Reproduzi isso:

```
$ INPUT='{"tool_name":"Bash","tool_input":{"question":"Preciso de confirmação — seções não batem, ✓ falhou"}}'
$ echo "$INPUT" | PYTHONIOENCODING=cp1252 python3 -c "...print(...)"
# neste ambiente (macOS + PYTHONIOENCODING via env) NÃO reproduziu o crash isolado —
# json.load decodifica stdin sob a MESMA PYTHONIOENCODING, então bytes UTF-8 de entrada
# já chegam mal-decodificados ANTES do print, e o resultado mal-decodificado por acaso
# round-tripa de volta em cp1252 sem estourar. Ver §6 — isto NÃO é o mesmo caminho que
# um `print('literal-embutido-no-source')` puro, que estoura de forma limpa e reprodutível
# (medido em §1). A interação stdin-decode + stdout-encode sob a mesma env var precisa
# ser medida num Windows real ou com locale/console genuinamente cp1252, não simulada
# por env var isolada — registrado como residual, não fechado aqui.
```

**Mas mesmo no caminho que falha**, a linha já tem uma rede de segurança que o REQ não menciona:
`2>/dev/null || echo "Agent needs attention"`. Se o `python3` morrer, o `||` absorve o erro e `MSG`
vira uma string genérica ASCII fixa — **o script inteiro sobrevive** (`set -euo pipefail` no topo
não se propaga para fora do `$(...)`, porque a falha é capturada pelo `||` dentro da substituição de
comando). O efeito observável de um crash aqui não é "o `trackfw init` entrega um script quebrado
que morre" — é **degradação silenciosa da mensagem específica para um texto genérico**. É uma perda
de informação (o usuário vê "Agent needs attention" em vez do texto real do agente), não uma queda
de disponibilidade do hook. Isso muda a severidade de "o script morre" (como o REQ enquadra) para
"o conteúdo da mensagem se perde silenciosamente" — pior para depuração, mas não é o crash total que
o issue original descreveu para os 39 scripts de gate.

**Um segundo artefato de produto que a REQ não nomeou:** `scripts/trackfw-git-branch-guard.sh`
(gerado por `internal/generators/git_branch_guard*.go` e equivalentes Node/Python, instalado como
hook em todo repositório que adota o trackfw) tem **534 bytes não-ASCII** — muito mais que
`attentionSignalScript` — em comentários e nas mensagens `REASON="trackfw: git commit bruto
bloqueado... Nada antes deste comando foi executado"` etc. Mas: **zero invocações de `python3`**
neste arquivo — é bash puro, `REASON` é atribuída e (presumivelmente) impressa via `echo`/mensagem
de erro nativa do bash. Pelo fato medido em §1.3, isso **não crasha** em cp1252 — na pior hipótese,
produz mojibake visual (glifos errados) no console do usuário, sem matar o guard. **Resposta direta
ao pedido do roadmap: `attentionSignalScript` não é o único artefato de produto com não-ASCII, mas
é o único que usa `python3` (o único primitivo crash-capaz) — confirmei isso auditando as três
invocações de `python3` em todo `internal/generators/`.**

`trackfw-credential-guard.sh` e `trackfw-validate.sh` gerados: 0 invocações de `python3`, mesma
classe de risco (zero) que `git-branch-guard.sh`.

### (b) Ferramenta — os 39 gates restantes de `scripts/`

39 = 40 (população medida em §1.2-bis) menos `scripts/trackfw-attention-signal.sh`, que é a cópia
já materializada do artefato (a) dentro deste próprio repositório (dogfooding) — mesmo conteúdo,
mesma classificação, não é um gate. **Quem quebra aqui somos nós** — CI e desenvolvedores locais,
não o adotante. Urgência mais baixa que (a), mas correção uniforme ainda é obrigatória: 39 pontos
divergentes na próxima manutenção é o cenário que a AC3 da REQ já rejeita, e o gate de prevenção
(AC5) precisa cobrir os 39, não só os que hoje têm dado não-ASCII comprovado — amanhã qualquer um
pode passar a ter.

**Item a item** (extrator de corpo Python de §1.2-bis; ver ressalva de cobertura logo abaixo):

| script | classificação medida |
|---|---|
| `check-agent-hooks-parity.sh` | sem não-ASCII localizado dentro do corpo Python (heredoc/-c); risco é preventivo (AC5), não presente |
| `check-agent-models-parity.sh` | sem não-ASCII localizado dentro do corpo Python (heredoc/-c); risco é preventivo (AC5), não presente |
| `check-agent-namespace-union.sh` | sem não-ASCII localizado dentro do corpo Python (heredoc/-c); risco é preventivo (AC5), não presente |
| `check-artifact-parity.sh` | sem não-ASCII localizado dentro do corpo Python (heredoc/-c); risco é preventivo (AC5), não presente |
| `check-atomic-write-anti-divergence.sh` | não-ASCII real no corpo, mas livre (print para stderr, diagnóstico) — seguro por §4(a) |
| `check-attention-scripts-parity.sh` | sem não-ASCII localizado dentro do corpo Python (heredoc/-c); risco é preventivo (AC5), não presente |
| `check-audit-surface.sh` | sem não-ASCII localizado dentro do corpo Python (heredoc/-c); risco é preventivo (AC5), não presente |
| `check-barrier.sh` | sem não-ASCII localizado dentro do corpo Python (heredoc/-c); risco é preventivo (AC5), não presente |
| `check-branch-new-parity.sh` | sem não-ASCII localizado dentro do corpo Python (heredoc/-c); risco é preventivo (AC5), não presente |
| `check-branch-prune-parity.sh` | sem não-ASCII localizado dentro do corpo Python (heredoc/-c); risco é preventivo (AC5), não presente |
| `check-ci-workflow-pin-parity.sh` | sem não-ASCII localizado dentro do corpo Python (heredoc/-c); risco é preventivo (AC5), não presente |
| `check-cli-parity.sh` | sem não-ASCII localizado dentro do corpo Python (heredoc/-c); risco é preventivo (AC5), não presente |
| `check-commit-parity.sh` | sem não-ASCII localizado dentro do corpo Python (heredoc/-c); risco é preventivo (AC5), não presente |
| `check-doctor-parity.sh` | sem não-ASCII localizado dentro do corpo Python (heredoc/-c); risco é preventivo (AC5), não presente |
| `check-doctor-remote-parity.sh` | sem não-ASCII localizado dentro do corpo Python (heredoc/-c); risco é preventivo (AC5), não presente |
| `check-gates-falsify.sh` | sem não-ASCII localizado dentro do corpo Python (heredoc/-c); risco é preventivo (AC5), não presente |
| `check-harness-hooks-parity.sh` | sem não-ASCII localizado dentro do corpo Python (heredoc/-c); risco é preventivo (AC5), não presente |
| `check-homedir-parity.sh` | sem não-ASCII localizado dentro do corpo Python (heredoc/-c); risco é preventivo (AC5), não presente |
| `check-identity-parity.sh` | não-ASCII só em comentário Python; dado impresso é ASCII (ids de catálogo) — sem risco hoje |
| `check-integration-cli-parity.sh` | sem não-ASCII localizado dentro do corpo Python (heredoc/-c); risco é preventivo (AC5), não presente |
| `check-parity-contract-coverage.sh` | não-ASCII só em comentário Python fora do dado impresso — sem risco hoje |
| `check-push-force-parity.sh` | sem não-ASCII localizado dentro do corpo Python (heredoc/-c); risco é preventivo (AC5), não presente |
| `check-push-parity.sh` | sem não-ASCII localizado dentro do corpo Python (heredoc/-c); risco é preventivo (AC5), não presente |
| `check-python-writes-lf.sh` | sem não-ASCII localizado dentro do corpo Python (heredoc/-c); risco é preventivo (AC5), não presente |
| `check-release-tag-parity.sh` | sem não-ASCII localizado dentro do corpo Python (heredoc/-c); risco é preventivo (AC5), não presente |
| `check-roadmap-barrier-contract.sh` | não-ASCII real no corpo, CAPTURADO/hasheado — ver §3 (risco confirmado) |
| `check-roadmap-move-parity.sh` | sem não-ASCII localizado dentro do corpo Python (heredoc/-c); risco é preventivo (AC5), não presente |
| `check-rules-parity.sh` | sem não-ASCII localizado dentro do corpo Python (heredoc/-c); risco é preventivo (AC5), não presente |
| `check-serve-address-parity.sh` | sem não-ASCII localizado dentro do corpo Python (heredoc/-c); risco é preventivo (AC5), não presente |
| `check-shell-posix-portability.sh` | sem não-ASCII localizado dentro do corpo Python (heredoc/-c); risco é preventivo (AC5), não presente |
| `check-ship-force-parity.sh` | sem não-ASCII localizado dentro do corpo Python (heredoc/-c); risco é preventivo (AC5), não presente |
| `check-ship-parity.sh` | sem não-ASCII localizado dentro do corpo Python (heredoc/-c); risco é preventivo (AC5), não presente |
| `check-slash-parity.sh` | sem não-ASCII localizado dentro do corpo Python (heredoc/-c); risco é preventivo (AC5), não presente |
| `check-thirdparty-parity.sh` | sem não-ASCII localizado dentro do corpo Python (heredoc/-c); risco é preventivo (AC5), não presente |
| `check-tty-detection.sh` | sem não-ASCII localizado dentro do corpo Python (heredoc/-c); risco é preventivo (AC5), não presente |
| `check-unknown-command-parity.sh` | sem não-ASCII localizado dentro do corpo Python (heredoc/-c); risco é preventivo (AC5), não presente |
| `check-update-parity.sh` | sem não-ASCII localizado dentro do corpo Python (heredoc/-c); risco é preventivo (AC5), não presente |
| `check-validate-parity.sh` | não-ASCII só em comentário Python fora do dado impresso — sem risco hoje |
| `smoke-integration-packages.sh` | sem não-ASCII localizado dentro do corpo Python (heredoc/-c); risco é preventivo (AC5), não presente |

## 3. Saída que alimenta hash ou comparação

Busquei todo uso de `sha256sum`/`shasum`/`hashlib` nos 40 scripts (`grep -rln`) — **5 scripts**:
`check-agent-models-parity.sh`, `check-identity-parity.sh`, `check-roadmap-barrier-contract.sh`,
`check-thirdparty-parity.sh`, `check-update-parity.sh`. Inspecionei os cinco:

| script | o que é hasheado | codificação do hash | veredito |
|---|---|---|---|
| `check-agent-models-parity.sh:991` | `sys.stdin.buffer.read()` (bytes crus, sem decode) → `hexdigest()` (ASCII puro) | nunca passa por encoder de texto | **seguro** |
| `check-thirdparty-parity.sh:142,158` | idem — leitura binária de stdin, escreve só o hex | idem | **seguro** |
| `check-update-parity.sh:463` | `(sentinel + "\n").encode()` — `.encode()` sem argumento é **sempre UTF-8** em Python 3, independente de locale/console | não depende de `sys.stdout.encoding` | **seguro** |
| `check-identity-parity.sh:28` | heredoc grava `target`/`target=surface` — ids de catálogo (`claude`, `antigravity=legacy-cli`), texto de projeto, tipicamente ASCII | print() para arquivo redirecionado, mas conteúdo hoje é ASCII | **seguro hoje, frágil se o catálogo ganhar um id não-ASCII** — não é o achado do REQ, mas registro para quem tocar o gate |
| **`check-roadmap-barrier-contract.sh:513-542`** | **CONFIRMADO.** Heredoc Python (`python3 - "$out" "$base" "$label" >>"$CORPUS_LINES_FILE"`) faz `print(f"{base}\t{label}\t{c['name']}\tevidence\t{e}")` onde `e` é texto de evidência **extraído do conteúdo real de roadmaps em português** (`docs/roadmaps/**`, com acentos) — o `print()` vai para stdout **redirecionado para um arquivo** (`>>`), não para um terminal, e a saída redirecionada em Windows **ainda** usa o codepage do console por padrão (não vira UTF-8 automaticamente só por não ser um tty). `CORPUS_HASH` na linha 542 hasheia esse arquivo. | depende de `sys.stdout.encoding` no momento do `print()` | **⚠️ CONFIRMADO — o único ponto onde o achado do PR #238 se sustenta** |

**Medi o efeito, não inferi:**

```
$ python3 -c "
import hashlib
s = 'seções relatório — ✓'
print('utf-8 hash:        ', hashlib.sha256(s.encode('utf-8')).hexdigest()[:12])
print('cp1252-replace hash:', hashlib.sha256(s.encode('cp1252', errors='replace')).hexdigest()[:12])
"
utf-8 hash:         f3d8ae04b153
cp1252-replace hash: 963d8a0f993d
```

Mesma string-fonte, dois hashes diferentes, só por causa da codificação de saída. **Veredito: hash
divergente é pior que crash**, como o roadmap já antecipa — um crash é barulhento e para o CI com
um traceback apontando exatamente para a linha; um hash diferente passa como "o corpus mudou" e
manda quem investiga procurar uma alteração de roadmap que nunca existiu. Este é o único dos 5
pontos de hash que precisa de correção — os outros 4 já são seguros pela própria forma como
foram escritos (bytes crus ou `.encode()` sem argumento).

**"Ou comparação" — não só hash.** O pedido nomeia hash explicitamente mas pede "outros pontos
assim", e esta suíte é majoritariamente de **paridade** (Go vs. Node vs. Python) — o dano de um
byte-stream mangled por SO é o mesmo: uma divergência que não existe no produto passa a existir só
no gate. Os candidatos óbvios por volume de não-ASCII no arquivo inteiro são
`check-artifact-parity.sh` (1236 bytes), `check-validate-parity.sh` (1034) e `check-cli-parity.sh`
(104) — mas a tabela item-a-item de §2(b), que já isola o corpo Python de cada invocação, já
responde por esses três: **nenhum dos três tem não-ASCII dentro do corpo `python3 - <<TAG`/`-c`
que compara os três JSONs de saída** (`check-validate-parity.sh` teve só um comentário bash fora do
corpo; `check-artifact-parity.sh`/`check-cli-parity.sh` não tiveram nenhum). O volume de bytes
não-ASCII desses arquivos vem de `echo`/comentário de diagnóstico ao redor da comparação, não do
dado comparado — e por §1.3 isso não afeta o resultado da comparação (bash não recodifica). **Não
encontrei, na amostra que consegui isolar, um segundo ponto de comparação (fora do hash) onde a
codificação de saída do Python mude o resultado** — mas a ressalva de cobertura dos 19 arquivos
sem corpo isolado (acima) se aplica igual aqui: não é uma prova de ausência para o universo dos 40,
é a evidência que consegui produzir com o orçamento desta sessão.

## 4. Falsificação nas duas direções — `errors="replace"`

**(a) `reconfigure(encoding="utf-8", errors="replace")`** — força a codificação da stream para
UTF-8 e substitui o que não representar. Medido: nunca crasha, preserva o dado (UTF-8 cobre todo
Unicode). **Onde é aceitável:** nos 39 scripts de ferramenta — mensagem de diagnóstico impressa para
um humano/CI ler; se o console de fato exibir mojibake por não estar configurado para UTF-8, o pior
caso é uma leitura difícil, não um dado incorreto (o byte-stream é fielmente UTF-8; a exibição é
outro problema, do terminal, não do script). Disponibilidade > fidelidade visual aqui.

**(b) `reconfigure(errors="replace")` SEM forçar `encoding="utf-8"`** — mantém a codificação
detectada do console (ex.: cp1252) e só troca falhas por `?`. Medido:

```
$ PYTHONIOENCODING=cp1252 python3 -c "
import sys
sys.stdout.reconfigure(errors='replace')
print('seções relatório — ✓')"
se��es relat�rio � ?
```

**Corrompe o dado silenciosamente** — `ç`, `õ`, `—`, `✓` viram `?`/lixo, sem erro, sem aviso. **Onde
isso é inaceitável: exatamente no ponto do §3** — o `CORPUS_HASH`. Se a correção da REQ adotar
`errors="replace"` sem também fixar `encoding="utf-8"`, o hash **continua não-determinístico entre
SO** (`?` uniforme substitui caracteres diferentes de forma que dois corpora diferentes podem colidir
no mesmo `?`, e o mesmo corpus ainda hasheia diferente em cp1252 vs. UTF-8) — a correção pareceria
funcionar (não crasha mais) mas **não resolve o AC6 da REQ**. A correção certa para esse ponto
específico é `encoding="utf-8"` (com ou sem `errors="replace"` como cinto de segurança adicional,
já que UTF-8 cobre todo o alfabeto usado nos roadmaps e não deveria precisar de `replace` na
prática) — nunca manter a codificação do console.

**(c) Uma terceira opção que a REQ não cita e o roadmap pede que eu nomeie ("a simétrica decide o
remédio"): `PYTHONUTF8=1` (ou `python3 -X utf8`) fixado uma vez, fora do Python.** Ativa o modo
UTF-8 do CPython — mesmo efeito de `reconfigure(encoding="utf-8")` mas aplicado **sem editar
nenhum dos 40 scripts**: uma variável de ambiente no `Makefile`/no `env` do gate cobre todos de
uma vez, o que bate diretamente com a AC3 da REQ ("correção uniforme e verificável por gate, não
script a script na mão"). Contra-parte medida: **isso só resolve os 40 scripts deste
repositório** (que já rodam sob controle do próprio `Makefile`/CI) — não resolve o
`attentionSignalScript`, porque esse roda na máquina do **adotante**, invocado por um hook de
agente de IA (Claude/Codex/etc.), fora do `Makefile` do trackfw; ali a env var precisaria ser
setada dentro do próprio script gerado (`export PYTHONUTF8=1` antes da chamada a `python3`), o que
tem o mesmo efeito prático de `encoding="utf-8"` mas sem tocar o código Python em si — outra
variante da mesma família de remédio, não uma alternativa a ela.

**Confirmei que a exposição não está limitada a versões antigas do Python:** a partir do CPython
3.15 (PEP 686), o modo UTF-8 passa a ser o **padrão** — o que eliminaria esta classe de defeito
sem qualquer mudança de código, mas só em runtimes que o adotante ainda não tem hoje.
`pypi/pyproject.toml:11` fixa `requires-python = ">=3.10"` — **toda a faixa suportada (3.10–3.14)
está exposta**, porque o PEP só afeta 3.15+. Não é um caminho de mitigação disponível agora; é
contexto de quanto tempo esta classe de bug continuará viva em instalações reais mesmo depois de
qualquer correção nesta REQ (adotantes com Python antigo continuam expostos até atualizarem o
interpretador, independente do que o trackfw fizer).

**Bare `python` (sem o `3`) — a AC5 da REQ pede "python3 nunca python" como parte do gate.**
Varri `scripts/*.sh` por `python` sem o sufixo `3` fora de comentário: **nenhuma ocorrência** —
todas as invocações já usam `python3` explicitamente. A AC5 já está satisfeita neste eixo
especificamente; o gate ainda precisa impor isso para não regredir, mas não há correção pendente
aqui.

**Onde `errors="replace"` seria pior que o comportamento atual:** no `MSG=$(...)` de
`attentionSignalScript` (§2). Hoje, se `print()` falhar, o `||` externo troca a mensagem inteira por
um fallback genérico e limpo ("Agent needs attention") — o usuário sabe que perdeu informação. Se
alguém "corrigir" essas duas linhas adicionando só `sys.stdout.reconfigure(errors="replace")` (sem
`encoding="utf-8"`), o `print()` **passa a ter sucesso com conteúdo corrompido** (`?` no lugar de
acentos) e o `||` de fallback **nunca mais dispara** — o usuário passa a ver uma mensagem
que parece a original mas está silenciosamente truncada/malformada, sem sinal de que algo deu
errado. Isso é estritamente pior que o comportamento atual (fallback limpo). A correção certa aqui
também é `encoding="utf-8"`, não `errors="replace"` isolado.

## 5. Residual declarado

- **Não testei em Windows real** nem em console cp1252 genuíno — toda a medição usou
  `PYTHONIOENCODING=cp1252` como proxy, exatamente como a própria REQ instrui e como o
  `TestCliEmConsoleCp1252` do PR #223 já validou como método aceito. A interação entre
  **decodificação de stdin** e **codificação de stdout** sob a mesma env var (§2, o caso do JSON
  vindo pela pipe) não reproduziu o crash isolado neste ambiente — fica como medição pendente em
  Windows real ou com um console cp1252 genuíno (não simulado por env var isolada), porque a
  simulação por env var pode mascarar ou introduzir comportamento que um Windows real não tem.
  **Explicação mais provável da não-reprodução, sem fechar o caso:** `PYTHONIOENCODING=cp1252`
  governa tanto a decodificação de `stdin` quanto a codificação de `stdout` — bytes UTF-8 de
  entrada mal-decodificados como cp1252 antes do `print()` podem, dependendo dos caracteres
  específicos, (i) round-tripar de volta sem erro (o que observei), ou (ii) falhar já na leitura
  com `UnicodeDecodeError` em vez de `UnicodeEncodeError`, porque cp1252 tem posições de byte
  indefinidas (0x81, 0x8D, 0x8F, 0x90, 0x9D) que aparecem como bytes de continuação em sequências
  UTF-8 multibyte. Não teria testado isso sem o apontamento do revisor. Nas duas hipóteses de
  falha o resultado observável é o mesmo: `python3` sai não-zero, o `||` externo dispara, o
  fallback genérico entra — o que reforça, não enfraquece, a conclusão de severidade do §2 (a):
  o pior caso continua sendo degradação da mensagem, não morte do script.
- **Não fiz diff byte-a-byte completo dos três `attentionSignalScript`** (Go/Node/Python) —
  confirmei paridade da linha que importa (`MSG=$(...python3 -c...)`) e da primeira linha do bloco
  via grep direcionado, não um diff textual do bloco inteiro nos três arquivos (a extração
  automática do bloco em `hooks.js`/`init_gen.py` por string-matching falhou por heurística frágil
  de delimitação, não tentei uma segunda vez por custo/benefício).
- **Não audite `awk`/`sed` sob locale não-C/não-POSIX** quanto a comportamento de classes de
  caractere multibyte — é uma classe de defeito diferente (reordenamento/match incorreto, não
  crash-por-encode) e ficou fora do escopo desta varredura por não bater com o sintoma "morre".
- **Não confirmei se o número "12 caracteres" do REQ é herança de uma versão anterior de
  `scaffold.go`** (não rodei `git blame`/`git log -p` completo na linha) — reportei a discrepância
  medida contra o `HEAD` atual, não a causa da discrepância.
- **Não tentei localizar a origem exata do bug em `check-identity-parity.sh`** além de confirmar que
  hoje é seguro por o conteúdo hasheado ser ASCII — não é um achado do REQ, registrei como frágil
  para referência futura, não como item de correção desta REQ.
- **Extrator de corpo Python (§1.2-bis) cobriu só 21 dos 40 arquivos** — os outros 19 usam sintaxe
  de invocação de `python3` que o regex do extrator não reconheceu; para esses, a coluna
  "sem não-ASCII no corpo" da tabela de §2(b) é herdada do filtro de arquivo inteiro (mais fraco),
  não de uma medição direta do corpo Python. Não escrevi um segundo extrator mais tolerante por
  custo/benefício de tempo nesta sessão.
- Nenhuma linha de correção foi escrita; nenhuma operação de git foi executada; nenhum arquivo fora
  de `docs/seguranca/` e `docs/agents-working-context.md` foi tocado. **Correção ao registro
  original desta seção:** cheguei a escrever fixture sob o scratch da sessão
  (`$SCRATCH/hookpoc/trackfw.yaml` + cópia de `trackfw-attention-signal.sh`, mais os arquivos de
  varredura `sh_scan*.txt`/`python3_invocations.txt`/`python_regions_precise.txt`/`table39.txt`) —
  tudo sob `/private/tmp/claude-501/.../scratchpad`, nada fora dele, nada em `/tmp` bruto e nada
  no repositório real. A afirmação anterior de que "não houve necessidade" estava errada; corrijo
  aqui porque um erro sobre o próprio escopo de escrita, num relatório de segurança, é o tipo de
  erro que mais custa credibilidade se descoberto depois em vez de corrigido agora.

## 6. O que eu não previ

- Que a varredura "pelo sintoma" (§1.2, ~700 linhas de `echo`) fosse, na prática, **inofensiva** na
  imensa maioria dos casos — presumi, ao ler o pedido, que ampliar o escopo pelo sintoma
  necessariamente aumentaria a população de risco real. O fato de bash `echo`/`printf` nunca
  encodarem (§1.3) faz a varredura por sintoma **confirmar** o método por mecanismo em vez de
  substituí-lo — o valor da varredura ampla não foi achar mais scripts que quebram, foi **provar que
  não há mais scripts que quebram** além dos 40 já conhecidos.
- Que o `attentionSignalScript` citado como "12 caracteres não-ASCII" no REQ **não bate** com o
  conteúdo atual do repositório (achei 1, num comentário morto) — e que o risco real ali não é
  estático, é a impressão de **dado dinâmico agente-controlado**, já parcialmente amortecido por um
  `|| echo fallback` que o REQ não menciona.
- Que houvesse um segundo artefato de produto (`trackfw-git-branch-guard.sh`, 534 bytes não-ASCII,
  mais que o próprio `attentionSignalScript`) fora do radar da REQ — e que ele seja, ainda assim,
  seguro pelo mesmo motivo do §1.3 (zero `python3`).
- Que `errors="replace"` sozinho, sem `encoding="utf-8"`, seria **estritamente pior** que o
  comportamento atual do `attentionSignalScript` — troca um fallback limpo e visível por uma
  mensagem corrompida e silenciosa. Um "conserto" ingênuo copiado de outro ponto do código (ex.: um
  padrão só de `errors="replace"` aplicado uniformemente aos 40 scripts sem diferenciar os que
  hasheiam) resolveria o crash mas reintroduziria, de forma pior, o próprio problema do §3/§4 no
  ponto que mais importa.
