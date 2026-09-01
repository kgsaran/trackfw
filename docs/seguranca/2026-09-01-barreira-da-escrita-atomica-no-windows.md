---
date: 2026-09-01
author: hades-tf
status: barreira final
target: PR #234 (branch fix/escrita-atomica-do-cli-python-funciona-no-windows)
---

# Barreira final de segurança — escrita atômica do CLI Python no Windows (PR #234)

> Segunda passagem sobre a mesma REQ da Wave 0
> (`docs/seguranca/2026-09-01-modelo-de-ameaca-da-escrita-atomica-no-windows.md`). Esta barreira
> verifica se `git diff origin/main...HEAD` honrou o que aquele parecer exigiu, e se algo não
> previsto apareceu.

## Veredito

**APROVA COM RESSALVAS.**

Nenhum bloqueante. O fallback condicional está correto byte a byte nos três arquivos (confirmado que
são exatamente três, por grep independente da lista fixa da REQ/gate), é o mesmo texto nos três
(gate falsificado nas duas direções, ao vivo — evidência abaixo), os testes medem o resultado
observável no único site não-vácuo, e o contrato em `docs/cli-parity.md` não afirma o que é falso.
As ressalvas são três: um caminho de arquivo citado duas vezes que não existe; a execução real do
fallback em Windows depende de um passo de CI condicional a uma precondição já conhecida por falhar,
sem repro dedicado como rede de segurança; e o gate anti-divergência não detecta uma quarta cópia
futura, só a divergência entre as três atuais. Nenhuma bloqueia — todas nomeadas, não fechadas.

---

## 1. 🔴 O fallback condicional preserva a garantia em POSIX, byte a byte?

**Sim. Verificado nos três arquivos e ao vivo.**

`pypi/trackfw/identity/__init__.py`, `pypi/trackfw/integrations/manager.py` e
`pypi/trackfw/thirdparty/quarantine.py` têm exatamente o mesmo bloco:

```python
fchmod = getattr(os, "fchmod", None)
if fchmod is not None:
    fchmod(descriptor, mode)
else:
    os.chmod(temporary, mode)
```

Não há `getattr` mal escrito, sombreamento de `os.fchmod` por variável local, nem ordem de avaliação
que dispare o `else` com `os.fchmod` presente — o teste (`fchmod is not None`) é direto, sem
negação dupla nem short-circuit implícito.

**Falsificação ao vivo, direção "o fallback dispara por engano":** simulei exatamente o erro que
preocupa — substituí `if fchmod is not None:` por `if False:` em uma cópia de
`integrations/manager.py`, forçando o `else` (o `os.chmod` do fallback) a rodar mesmo com
`os.fchmod` presente. O teste de controle pegou:

```
FAILED test_manager_0o644_uses_fchmod_not_chmod_on_posix
AssertionError: os.fchmod must be used when present
assert 0 == 1
```

Arquivo restaurado logo em seguida (`git diff --stat` voltou vazio, confirmado). Isto é a
propriedade que a AC3(b) pede: um fallback que dispara indevidamente **não passa em silêncio** — ele
quebra um teste nomeado para exatamente esse modo de falha.

## 2. O controle mira o site certo?

**Sim.** `test_manager_0o644_fallback_produces_observable_mode`
(`pypi/tests/test_atomic_write_windows_fallback.py:78-92`) monta `IntegrationManager._atomic_write`
com `mode=0o644` (o único dos 7 sites onde o modo difere do `0o600` que `tempfile.mkstemp()` já
entrega por padrão) e verifica, sob `if os.name == "posix":`, `stat.S_IMODE(target.stat().st_mode)
== 0o644` — o **resultado no disco**, não a chamada. Rodei a suíte inteira localmente: os 7 testes
passam (`7 passed in 0.15s`, macOS/Python 3.14).

Os 3 testes que instrumentam a chamada (`test_*_uses_fchmod_not_chmod_on_posix`) existem para a
Direção (b) — "fchmod não foi contornado" — que é inerentemente sobre instrumentação, não sobre
efeito no disco (não há efeito observável diferente entre `fchmod(fd, 0o600)` e o `0o600` que o
`mkstemp` já entrega). Isso é coerente: instrumentar a chamada é vácuo quando usado para provar
"o modo certo foi aplicado" (era o erro do ML-0A), mas é o controle certo quando o que se quer provar
é "qual API foi usada". Os dois papéis não estão confundidos no diff.

## 3. O skip está condicionado a capacidade medida, não a plataforma?

**Sim, e a divisão é a que a barreira exigiu.** `_HAS_FCHMOD = hasattr(os, "fchmod")`
(`test_atomic_write_windows_fallback.py:48`) só é usado em `@pytest.mark.skipif` nos **3** testes
"Direção (b)" (`test_manager_0o644_uses_fchmod_not_chmod_on_posix`,
`test_identity_uses_fchmod_not_chmod_on_posix`, `test_quarantine_uses_fchmod_not_chmod_on_posix`) —
correto, porque não há nada para espionar (`monkeypatch.setattr(os, "fchmod", spy)`) numa plataforma
onde o atributo não existe.

Os **4** testes restantes (as 3 variantes `test_*_survives_missing_fchmod` + o
`test_manager_0o644_fallback_produces_observable_mode`) **não têm `skipif` nenhum** — rodam
incondicionalmente, em qualquer SO. Em Windows, `os.fchmod` já está ausente nativamente, então
`monkeypatch.delattr(os, "fchmod", raising=False)` é um no-op e o teste exercita o fallback real, não
uma simulação — exatamente o que o docstring do arquivo declara (linhas 13-17) e o que a barreira
pediu para confirmar.

**Correção depois de ler os passos reais do workflow (não só o comentário de cabeçalho):**
`.github/workflows/quality.yml` tem um job `windows-full-suites` (`runs-on: windows-latest`) cujo
passo `python -m pytest pypi/tests -q` (linha ~305) cobriria este arquivo — mas esse passo **não é
incondicional**. Ele tem `if: always() && steps.precondition.outcome == 'success'`, e a
precondição (AC12, linhas ~229-259) é a isolação de `HOME`/`USERPROFILE` nos três runtimes; se ela
falhar, um step dedicado (linha ~312) registra `::warning::Camada 1 ... foi pulada` e **a suíte
pytest inteira, este arquivo novo incluso, não roda naquela execução**. A nota do vault
`job-de-windows-largo-so-reproduz-2-dos-11-defeitos-2026-08-30` já documenta que essa isolação é
vácua nos três runtimes hoje — ou seja, a precondição já é conhecida por falhar em circunstâncias
reais, não é hipotética.

Verifiquei também o job irmão `windows-defect-reproduction` (`scripts/windows-repro/run.ps1`), que
roda os 11 itens da issue #216 item a item **independente** da precondição de `windows-full-suites`
— mas `grep -n "atomic\|fchmod\|chmod" scripts/windows-repro/run.ps1` não retorna nada: este defeito
(achado 13 da triagem, fora dos 11 originais da issue #216) **não tem item dedicado nesse job**. Ou
seja, a única cobertura de execução real em Windows para este fallback é o passo condicional de
`windows-full-suites` — não há uma segunda rede de segurança se aquele passo for pulado.

**Isso não muda o veredito** (a suíte roda hoje, na maioria das execuções, e prova o AC1 quando
roda), mas revisa a força da afirmação: não é "AC1 tem verificação real em CI" sem qualificação — é
"AC1 tem verificação real em CI, condicionada a uma precondição que já é sabidamente instável nos
três runtimes, e sem repro dedicado como rede de segurança". Ver Residual, item 1, abaixo.

## 3-bis. Existem só três sites de `os.fchmod`, e o Node só tem os sites já documentados?

**Confirmado, por grep independente da lista fixa do gate/REQ, não por confiança nela.**

`grep -rn "fchmod" pypi/trackfw/` retorna exatamente os três blocos de `identity/__init__.py`,
`integrations/manager.py` e `thirdparty/quarantine.py` — nenhum quarto site esquecido. Se houvesse
um quarto, o AC1 ("as três escritas atômicas funcionam em Windows") seria falso e este PR teria
deixado um crash de Windows sem corrigir; não é o caso.

`grep -rn "chmodSync\|fchmodSync" npm/src/` confirma os dois sites já nomeados em `docs/cli-parity.md`
(`quarantine.js:29`, `manager.js:95` e `97`) e nenhum `fchmodSync` real em lugar nenhum — os demais
`chmodSync` do Node (`update.js:120`, `generators/hooks.js`, `generators/init.js`) são restauração do
bit de execução em scripts de hook, uma classe de arquivo diferente da dos três `_atomic_write`, fora
do escopo desta REQ e não citados como se fossem parte dela.

## 3-ter. O gate cobre o quarto-site-futuro, ou só a não-divergência dos três atuais?

🟡 **Achado de acompanhamento, não bloqueante — a mesma classe do que a própria REQ nomeou.**
`FILES` em `scripts/check-atomic-write-anti-divergence.sh` é uma lista fixa de 3 caminhos. O gate
prova "estes três não divergem entre si"; não prova "só existem três". Se alguém adicionar uma quarta
cópia de `_atomic_write` em outro módulo Python amanhã (o mesmo padrão de replicação deliberada que
gerou as três atuais), o gate continua passando em silêncio para sempre — o modo de falha espelhado
do que o próprio gate foi desenhado para pegar ("corrigir duas de três e esquecer a terceira" vira
"esquecer de adicionar a quarta à lista do gate"). Mesma classe de gap que reportei em
`global-guard-dedup-and-hook-resolvable-never-validate-hook-structure-2026-08-18` (vault): controle
que verifica igualdade entre itens de uma lista fixa, não a completude da própria lista. Remédio
sugerido para REQ futura: gate adicional que varre `pypi/` por `import os` + uso de `os.fchmod` (ou
o padrão `getattr(os, "fchmod"`) e reprova se o número de ocorrências divergir de 3, nomeando o
arquivo extra.

## 3-quater. A anotação `trackfw-contract` nas duas seções novas parsa corretamente?

**Sim, confirmado por execução, não por leitura do formato.** Rodei
`scripts/check-parity-contract-coverage.sh docs/cli-parity.md .`: saída `OK — nenhuma anotação
inválida e nenhuma seção sem anotação`, exit 0. A seção principal (linha 6425) e a subseção
"Triplicação deliberada" têm `gate=` lido corretamente mesmo misturando o script
(`check-atomic-write-anti-divergence.sh`) com os três arquivos-fonte Python na mesma lista
separada por vírgula — confirmei no próprio checker (`scripts/check-parity-contract-coverage.sh:317`)
que `gate=` é sempre `split(",")` e cada item checado por `os.path.isfile`, sem exigir que sejam
todos scripts. Não é decoração; a anotação é lida de verdade pelo instrumento que a `parity:` do
`Makefile` invoca.

## 4. O gate anti-divergência mascara alguma diferença semanticamente relevante?

**Não, e falsifiquei nas duas direções fora do repo (cópias em `/tmp`, nunca no working tree):**

- **Positivo (as três iguais → passa):** `bash scripts/check-atomic-write-anti-divergence.sh .`
  contra a árvore real → `OK — 3 cópias ... com bloco de fallback idêntico após normalização`.
- **Negativo com divergência semântica real:** troquei `if fchmod is not None: fchmod(...) else:
  os.chmod(...)` por uma ordem invertida (`if fchmod is None: os.chmod(...) else: fchmod(...)`) em
  uma cópia de `identity/__init__.py` num diretório `/tmp` isolado → o gate reprovou nomeando o
  arquivo divergente (`DIVERGÊNCIA ... diverge: thirdparty/quarantine.py / integrations/manager.py`).
- **Negativo com quebra de âncora:** qualquer edição que toque o texto literal
  `os.chmod(temporary, mode)` (a borda de fim do bloco capturado) faz a extração falhar por âncora
  não encontrada, e o gate reprova por vacuidade ("esperava extrair 3 blocos, extraiu 2") em vez de
  comparar 2 de 3 silenciosamente — comportamento fail-safe, não fail-open.

`textwrap.dedent` só remove o prefixo de espaços **comum a todas as linhas do bloco capturado** —
não zera indentação por linha, então a estrutura relativa `if/else` (que é onde uma divergência
semântica viveria) sobrevive à normalização. A única coisa que a normalização absorve é o
deslocamento uniforme entre `@staticmethod` dentro de classe (`integrations/manager.py`, 12 espaços)
e função de módulo (`identity/__init__.py`, `thirdparty/quarantine.py`, 8 espaços) — que é
exatamente o motivo declarado no cabeçalho do script (linhas 36-58) e não esconde nenhuma diferença
de comportamento.

## 5. O contrato em `docs/cli-parity.md`

**Correto no que afirma sobre a garantia; duas citações de caminho quebradas.**

- **Não afirma o falso:** a seção nova diz explicitamente "**O contrato não é 'os 3 runtimes
  preservam a garantia de descritor' — isso seria falso hoje**" e apresenta a tabela com Node
  marcado `❌ janela aberta hoje, em produção, sem relação com Windows`. Nomeia
  `npm/src/thirdparty/quarantine.js:28-30` e `npm/src/integrations/manager.js:94-97` — **confirmei
  por leitura**: `quarantine.js` tem `chmodSync(tmp, mode)` na linha 29 seguido de `renameSync` na
  30; `manager.js` tem `writeFileSync` (94), `chmodSync(tmp, mode)` (95), `renameSync` (96) e uma
  **segunda** `chmodSync(file, mode)` depois do rename (97) — o "chmod duas vezes, a segunda depois
  do rename" que o contrato descreve é real, não hipérbole.
- **`partial=` honesto:** a anotação do `trackfw-contract` diz que o gate "cobre só a
  não-divergência das três cópias Python entre si; não mede Go nem Node, nem a janela pré-existente
  do `os.replace(path)`" — bate com o que o script realmente faz (só varre os 3 arquivos Python
  listados em `FILES`).
- 🟡 **Achado — referência quebrada, repetida em dois documentos deste mesmo diff.** Tanto
  `docs/cli-parity.md` quanto `docs/req/REQ-2026-09-01-cli-node-usa-chmodsync-...md` (linha 27)
  citam **`npm/src/identity.js`** como o ponto do Node que "não tem a segunda janela". Esse arquivo
  **não existe** — confirmei com `find`/`ls`: o caminho real é
  `npm/src/identity/config.js` (função `atomicWrite`, linhas 72-84). A afirmação de fundo continua
  verdadeira (`config.js` usa `fs.openSync(temporaryName, 'w', mode)`, aplicando o modo na criação
  do descritor, sem `chmodSync` nenhum — na verdade uma primitiva **mais correta** que as outras duas
  cópias do Node, não só "sem a segunda janela"), mas o caminho citado vai fazer quem pegar a REQ
  `REQ-2026-09-01-cli-node-usa-chmodsync-...` procurar um arquivo que não existe. Não bloqueante
  para este PR (a REQ do Node é trabalho futuro, roadmap ainda vazio), mas deveria ser corrigido
  antes que vire ML — **é achado de origem minha, da Wave 0** (o texto da REQ é meu), então registro
  aqui para quem abrir aquela REQ não repetir a busca.
  **Severidade: baixa, não bloqueante. Remédio:** trocar `npm/src/identity.js` por
  `npm/src/identity/config.js` nos dois documentos, e atualizar a frase para registrar que
  `config.js` usa `openSync(..., mode)` (não `chmodSync` nenhum) — precisão maior que a REQ
  original presumia.

## 6. Residual e superfície nova

Aceito, com três pontos nomeados explicitamente (os itens 2 e 3 não estavam na Wave 0 e apareceram
nesta passagem):

1. **Revisado nesta passagem — mais estreito do que eu tinha escrito na primeira versão.** O
   caminho `os.chmod(temporary, mode)` do fallback só executa em Windows — nenhuma máquina de
   desenvolvimento ou este ambiente de revisão consegue exercitá-lo nativamente. A cobertura real
   depende do passo `python -m pytest pypi/tests -q` do job `windows-full-suites`, mas esse passo
   **não é incondicional**: só roda se a precondição AC12 (isolação de `HOME`/`USERPROFILE` nos
   três runtimes) passar (ver item 3 acima) — e a própria nota do vault
   `job-de-windows-largo-so-reproduz-2-dos-11-defeitos-2026-08-30` documenta essa isolação como
   vácua hoje. O job irmão `windows-defect-reproduction` roda independente dessa precondição, mas
   `scripts/windows-repro/run.ps1` não tem item algum para este defeito (achado 13, fora dos 11
   originais da issue #216) — não existe hoje uma segunda rede de segurança. **Residual aceito, mas
   nomeado com mais precisão do que "depende do CI" — depende de um passo condicional específico,
   sem repro dedicado como fallback.**

2. **Achado de acompanhamento (não bloqueante):** o gate `check-atomic-write-anti-divergence.sh`
   prova que as três cópias não divergem entre si, mas não prova que só existem três — uma quarta
   cópia futura de `_atomic_write` não seria pega por esta lista fixa (item 3-ter acima). Mesma
   classe de gap documentada em `global-guard-dedup-and-hook-resolvable-never-validate-hook-structure-2026-08-18`.

3. **Novo nesta passagem — compõe com um achado anterior meu que não foi mencionado no pedido desta
   barreira.** A nota do vault `makedirs-mode-ignored-on-existing-and-intermediate-parents-2026-09-01`
   (também escrita por mim, na Wave 0, e já linkada no índice) mostra que `mode=0o700` em
   `os.makedirs(directory, exist_ok=True, mode=0o700)` — a chamada que precede o `_atomic_write` nos
   três arquivos — **não garante nada** se o diretório já existe (outro subsistema o criou antes sem
   `mode=`) ou se é um pai intermediário criado na mesma chamada. Isso não é regressão deste PR (a
   chamada já existia), mas **compõe diretamente com o fallback do Windows**: em POSIX, se o
   diretório-pai ficar em `0o755` em vez de `0o700` (cenário já comprovado ao vivo naquela nota), a
   janela do `os.chmod(path)` fica mais fácil de explorar por outro usuário local, porque o diretório
   deixou de ser exclusivo do dono. O residual "TOCTOU aceito só em Windows" presume implicitamente
   um diretório-pai restritivo — presunção que a própria Wave 0 já mostrou não se sustentar sempre.
   **Não bloqueante para este PR** (nem a REQ nem a AC pediam fechar esse gap; é da mesma família de
   achado, tratado como acompanhamento desde a Wave 0), mas registro a composição porque ninguém
   tinha juntado os dois até agora.

---

## Resumo para quem só quer o veredito

**APROVA COM RESSALVAS.** Sem bloqueantes.

- Fallback condicional correto nos 3 arquivos, idêntico entre si (confirmado por grep: exatamente 3
  sites de `os.fchmod` em `pypi/`, nenhum quarto esquecido), falsificado ao vivo nas duas direções
  (fallback dispara com `fchmod` presente → teste pega; fallback correto → teste passa).
- Controle no site certo (`st_mode` observável no único ponto não-vácuo), skip por capacidade
  (`hasattr`) e não por plataforma; 4 dos 7 testes não têm `skipif` e rodariam de verdade em Windows
  — mas essa execução real depende de um passo condicional do CI (ver Ressalva 2).
- Gate anti-divergência falsificado nas duas direções fora do repo; normalização não mascara
  diferença semântica, só indentação incidental. Anotação `trackfw-contract` das duas seções novas
  confirmada por execução do checker (`check-parity-contract-coverage.sh`, exit 0), não por leitura
  do formato.
- Contrato em `docs/cli-parity.md` não afirma garantia falsa para o Node; nomeia a exceção com
  precisão de linha, confirmada por leitura do código Node real (`quarantine.js:29`,
  `manager.js:95`/`97`, incluindo a segunda `chmodSync` pós-`rename`).
- **Ressalva 1 (baixa, não bloqueante):** `npm/src/identity.js` citado em `docs/cli-parity.md` e na
  REQ do Node não existe; caminho real é `npm/src/identity/config.js`. Corrigir antes de abrir a REQ
  como ML.
- **Ressalva 2 / risco residual aceito, revisado nesta passagem:** a única cobertura de execução real
  do fallback em Windows é o passo `pytest` de `windows-full-suites`, e esse passo é **condicional**
  a uma precondição (AC12, isolação de `HOME`/`USERPROFILE`) já documentada como vácua nos três
  runtimes por nota própria do vault; o job irmão `windows-defect-reproduction` não tem item para
  este defeito, então não há repro dedicado como rede de segurança se a precondição falhar. Compõe
  ainda com o gap de `makedirs(mode=)` já no vault, que pode deixar o diretório-pai menos restritivo
  do que a garantia POSIX presume. Nenhum dos dois é regressão deste PR; ambos acompanhamento.
- **Ressalva 3 (achado novo, não bloqueante):** o gate anti-divergência prova que as 3 cópias não
  divergem entre si, não que só existem 3 — uma quarta cópia futura não seria detectada pela lista
  fixa `FILES`. Mesma classe de gap já registrada em
  `global-guard-dedup-and-hook-resolvable-never-validate-hook-structure-2026-08-18` no vault.
- **Observação de processo (fora do meu escopo, para o arquiteto):** o roadmap
  `ROADMAP-2026-09-01-escrita-atomica-do-cli-python-funciona-no-windows.md` tem os 3 MLs marcados
  `✅ Concluído` mas ainda está em `docs/roadmaps/wip/`, não `done/`.
