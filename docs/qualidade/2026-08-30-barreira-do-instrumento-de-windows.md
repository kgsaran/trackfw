# Barreira de Qualidade — Instrumento de Medição de Windows (PR #221)

> Agente: Hefesto (Code Quality) · 2026-08-30
> Artefatos de governança: REQ-2026-08-30-ci-nao-exercita-windows-e-os-sete-defeitos-da-issue-216-sao-invisiveis-para-o-projeto.md · ROADMAP-2026-08-30-job-de-windows-largo-que-nasce-vermelho-e-sonda-sob-demanda.md · ADR-2026-08-30-ci-de-windows-como-instrumento-de-medicao-job-largo-que-nasce-vermelho-mais-sonda-sob-demanda.md

---

## Veredito

**APROVA COM RESSALVAS**

Este PR não corrige defeitos de produto — entrega um instrumento de medição, e o critério de aceite
(job nascer vermelho) está satisfeito e medido num runner real (8/8 itens em escopo reproduzidos,
`vault/notes/job-de-windows-largo-so-reproduz-2-dos-11-defeitos-2026-08-30.md`,
`vault/notes/if-sem-funcao-de-status-tem-success-implicito-2026-08-30.md`). O trabalho de
autoauditoria (Wave 0 threat model, ML-1C, ML-1D) já é incomum em rigor e captou várias das próprias
falhas antes de mim. `actionlint` limpo, `go build`/`go vet` limpos, os testes falsificados (0 skipped
em Linux/macOS) confirmam o ML-2A.

Encontrei, porém, **uma classe de vacuidade** que a autoauditoria não cobriu: dois dos oito itens
verdes-por-ausência-de-sintoma (item 1, item 10) decidem `ABSENT` sem confirmar que o caminho medido
foi de fato alcançado — e um deles tem um gatilho concreto e presente **neste mesmo PR**, que já
provocou o exato modo de falha que ele reintroduz (ML-1D, duas commits atrás). Isso é bloqueante.
Os demais achados são de acompanhamento.

**Responsável pelo fix:** `trackfw_architect` para determinar se o item bloqueante impede tirar o PR
do rascunho; `ares-tf` para as correções, seguindo `os.symlink`→pattern já usado no resto da REQ
(mesma régua de "detectar a condição medida, não presumir").

---

## 1. Legibilidade do sinal

**Boa, com uma lacuna que amplifica os achados abaixo.** `run.ps1` imprime, por item, `## ITEM N —
Título`, o detalhe bruto e `RESULT: VEREDITO`; o cabeçalho do arquivo documenta a régua completa de
mapeamento dos 11 itens (linhas 11–46), com justificativa por que cada item fora de escopo está fora.
Um mantenedor em 3 meses consegue, na maioria dos casos, distinguir "defeito ainda presente" de
"instrumento quebrou" — mas só porque o roadmap guarda a linha de base (8/8 REPRODUCED) em texto
solto; **nada no próprio `run.ps1` compara a execução atual contra essa linha de base** (ver §2,
remédio 2). Sem esse pino, a leitura do sinal depende de o mantenedor lembrar de abrir o roadmap.

`$ErrorActionPreference = "Continue"` no topo de `run.ps1` (linha 48) faz com que **toda** exceção — 
inclusive exceções .NET não tratadas por cmdlet nenhum, como `Process.Start()` falhando — vire ruído
vermelho no log em vez de um erro de script claramente marcado. Verificado empiricamente (não por
leitura): um `Process.Start()` com binário inexistente não aborta `run.ps1`; a função retorna
`Stdout=''`/`ExitCode=$null` e o script segue até o fim. Isso **não** produz falso verde por si
(cai em `INCONCLUSIVE`, que já força saída 1 — ver §2), mas mistura "bug no instrumento" com "defeito
reproduzido" no mesmo veredito `RESULT: INCONCLUSIVE`, sem marcador visual diferenciando as duas
causas. Acompanhamento: um `trap` de topo que escreva `::error::run.ps1 crashed: $_` tornaria a
distinção explícita no log, sem mudar nenhum veredito.

## 2. Falso verde — achado principal

**A classe do problema:** quatro dos itens medidos por `run.ps1`/`checks.py` decidem `ABSENT` (o
defeito não se manifesta — verde) pela **ausência de um sintoma específico** no stderr/stdout, sem
verificar que o caminho de produção que supostamente produziria esse sintoma foi de fato alcançado.
Se o processo morrer **antes** de chegar ao ponto medido — por qualquer motivo não relacionado ao
defeito — o item reporta `ABSENT` por vacuidade, não porque o defeito foi corrigido.

### 2.1 — BLOQUEANTE: `pyyaml` hardcoded reintroduz o exato bug que o ML-1D corrigiu na camada 1

**Arquivo:** `.github/workflows/quality.yml:374` (job `windows-defect-reproduction`)
```yaml
- run: python -m pip install --upgrade pip pyyaml
```

Duas commits antes deste PR, o **ML-1D** (`243cd17`) corrigiu exatamente este padrão na camada 1
(`windows-full-suites`), trocando uma lista de dependências nomeadas por `pip install pypi/` — o
comentário ao lado da correção (linha ~184 do mesmo arquivo) documenta o motivo: *"zero drift, sem
hardcodar nomes de dependência aqui"*, e registra que a causa raiz medida foi
`ModuleNotFoundError: No module named 'yaml'` em **40 módulos de teste** quando a lista nomeada
ficou incompleta. A camada 2 (`windows-defect-reproduction`, o job que precisa nascer vermelho pelos
motivos certos) **não recebeu a mesma correção** — continua nomeando `pyyaml` à mão, desacoplado de
`pypi/pyproject.toml`.

**Por que isto é vacuidade, não só drift de dependência:** `checks.py` importa `trackfw.cli` via
`PYTHONPATH=$TRACKFW_PYPI_SRC` (source, não pacote instalado — `checks.py:36`), mas as dependências de
runtime desse import (`pyyaml`, e qualquer outra que `pyproject.toml` venha a declarar) só existem no
ambiente porque este `pip install` as nomeia manualmente. O dia em que uma segunda dependência de
runtime entrar em `pyproject.toml` (sem editar este `pip install`), o próximo `import trackfw.cli`
morre com `ModuleNotFoundError` **antes** de alcançar qualquer um dos quatro pontos medidos — e o
efeito não é uma falha visível de setup, é:
- **Item 1** (`checks.py:44-56`) — veredito é `"UnicodeEncodeError" in stderr` → senão `ABSENT`.
  `ModuleNotFoundError` não contém a string `UnicodeEncodeError` → **`VERDICT=ABSENT`**, reportando
  "cp1252 corrigido" sem o binário jamais ter chegado ao `--help`. `proc.returncode` é impresso mas
  **nunca consultado** na decisão do veredito.
- **Item 4** (`checks.py:59-92`) — mesmo mecanismo, mesma vacuidade: um `subprocess.run([...
  "print('\\u2192')"])` isolado não depende do import de `trackfw.cli`, então sobrevive; mas o
  `encoding_probe` que o precede é ruído irrelevante nesse cenário — o achado real está no item 1/5/6.
- **Itens 5 e 6** — já dependem de `from trackfw.cli import main` completar; cairiam em
  `INCONCLUSIVE` (mensagem "init não completou"), possivelmente confundido de novo com a cascata do
  item 1 que o ML-1C já resolveu uma vez.

**Evidência de que o gatilho é real, não hipotético:** é o mesmo `ModuleNotFoundError` que o ML-1D
já mediu no runner real duas commits atrás, para o mesmo pacote (`pypi/`), pela mesma causa
(dependência de runtime não instalada). A janela de exposição é qualquer PR futuro que adicione uma
dependência a `pyproject.toml` sem lembrar de tocar esta linha isolada em `quality.yml` — e é
precisamente o tipo de manutenção que ninguém vai pensar em revisar, porque o job já existe e "já
funciona".

**Remédio concreto:** trocar `pip install pyyaml` por `pip install pypi/` neste step (linha 374),
igual à correção já feita na linha 219 (camada 1) e documentada no comentário ao lado dela. Um único
`s/pip install --upgrade pip pyyaml/pip install --upgrade pip \&\& python -m pip install pypi\//`
resolve — mas revisar se algum item precisa também de `pytest` (não parece — `run.ps1` não invoca
`pytest` na camada 2, só `checks.py` diretamente).

### 2.2 — Item 10: `ABSENT` não confirma que a operação medida ocorreu

**Arquivo:** `scripts/windows-repro/run.ps1:287-294`

```powershell
$reqPath = Join-Path $fixture "docs\req\REQ-item10.md"
if (-not (Test-Path $reqPath)) {
    return [pscustomobject]@{ Runtime = $Runtime; Verdict = "INCONCLUSIVE"; ... }
}
$reqContent = Get-Content -Raw $reqPath
$roadmapLine = ($reqContent -split "`n" | Where-Object { $_ -match "^roadmap:" })
$hasBackslash = $roadmapLine -match "\\"
$verdict = if ($hasBackslash) { "REPRODUCED" } elseif ($r.ExitCode -eq 0) { "ABSENT" } else { "INCONCLUSIVE" }
```

O veredito `ABSENT` (defeito ausente — verde) só checa **(a)** que o arquivo REQ ainda existe e
**(b)** que a linha `roadmap:` não contém `\`. Nada confirma que `roadmap move` de fato **sincronizou**
o frontmatter — a fixture já grava `roadmap: docs/roadmaps/backlog/ROADMAP-item10.md` (com `/`, linha
268) **antes** de rodar o comando. Um `roadmap move` que falhe silenciosamente em localizar/atualizar
o REQ (bug de sync, não relacionado ao separador de SO) devolve `ExitCode=0` e deixa a linha original
intocada — sem `\`, porque nunca foi reescrita — e o item reporta `ABSENT` como se o defeito estivesse
corrigido, quando na verdade **a operação nunca aconteceu**.

**Remédio concreto:** exigir evidência positiva de que o `move` atualizou o estado, não só a ausência
de `\`. Ex.: também assertar que `docs\roadmaps\wip\ROADMAP-item10.md` existe (destino do move) e/ou
que a linha `roadmap:` mudou de `backlog` para `wip` em algum segmento — hoje o teste só provaria
"o caminho não tem `\`" mesmo que o caminho inteiro esteja errado.

### Remédios estruturais (fecham a classe, não só os dois casos acima)

1. **Piso de contagem.** `run.ps1` tem exatamente 11 chamadas a `Add-Result` e calcula
   `$results.Count` (linha 324) mas nunca o usa como gate. Adicionar
   `if ($results.Count -ne 11) { exit 1 }` antes da linha 339 barra qualquer execução que perca linhas
   por um `return` antecipado ou uma reorganização futura do arquivo — inclusive o caso degenerado do
   item 7 (§5).
2. **Pino de veredito esperado por item.** Nada compara a execução atual contra a linha de base 8/8
   já medida e registrada no roadmap. Fixar `expected = REPRODUCED` por item (até a REQ de correção
   correspondente fechar) e reportar uma mudança de veredito como *"item N: baseline REPRODUCED, agora
   ABSENT — ou foi corrigido (atualize o pino) ou a verificação quebrou"* é o mecanismo que torna a
   distinção "defeito corrigido" vs. "instrumento quebrou" legível **sem** depender de o mantenedor
   lembrar onde a linha de base vive.

## 3. Falso vermelho

Não encontrei falso vermelho por motivo ambiental além do que já está **declarado e correto** no
próprio roadmap (§ "Residual declarado", Wave 0): locale/codepage além de cp1252, Developer Mode não
habilitado no runner hospedado, `core.symlinks=false` de clone de terceiro, console interativo real.
A correção do `always()` implícito (ML-1C, `vault/notes/if-sem-funcao-de-status-tem-success-implicito-2026-08-30.md`)
já resolveu o caso mais sério dessa categoria (Node/Python pulados silenciosamente após o Go falhar).
Nenhum achado novo aqui.

## 4. ML-2A — `skip` condicionado à condição medida

**Confirma-se.** Os três runtimes detectam a falha pela **condição** (privilégio negado na chamada de
symlink), não por `runtime.GOOS`/`process.platform`/`sys.platform`:
- Go: `os.IsPermission(err)` OU `syscall.Errno(1314)` (`internal/generators/update_test.go:34-51`)
- Node: `err.code in ('EPERM','EACCES')` (`npm/tests/update_discover_symlink_guard.test.js:56-75`)
- Python: `err.winerror == 1314` OU `err.errno in (EPERM, EACCES)`
  (`pypi/tests/test_update_discover_symlink_guard.py:36-56`)

Falsificação reexecutada nesta auditoria (não apenas lida): `go test
./internal/generators/... -run TestUpdateNeverWritesThroughSymlink -v` → PASS; `node --test
npm/tests/update_discover_symlink_guard.test.js` → **10 passed, 0 skipped**. O guard exercita
normalmente quando o symlink é criado com sucesso — o teste não fica "elegantemente pulado sem testar
nada"; ele só pula no ramo de falha de privilégio, e nesse ramo a mensagem nomeia a garantia não
exercitada, conforme AC7/AC8.

**Nota menor, não bloqueante:** `os.IsPermission(err)` (Go) e `err.code in ('EPERM','EACCES')`
(Node)/`err.errno in (EPERM, EACCES)` (Python) são checagens amplas o bastante para também capturar
um erro de permissão **não relacionado a Developer Mode** (ex.: ACL do diretório de fixture) e
tratá-lo como "privilégio de symlink ausente", pulando em vez de falhar. O risco é baixo porque a
falha ocorre na criação da fixture, não no código sob teste — mas se algum dia um bug real de
permissão de diretório aparecer no ambiente de teste, ele seria mascarado como "sem Developer Mode".
Não é o caso hoje (falsificação confirma 0 skipped em Linux/macOS). O resíduo "o ramo de skip não é
exercitável no runner Windows hospedado (que já tem o privilégio)" já está corretamente declarado no
próprio roadmap (ML-1D, "Residual 1") — não repito como achado novo aqui.

## 5. Duplicação entre os 4 arquivos de checks (e entre `probe.go`/`checks.go`)

**Achado concreto, com gatilho identificado:** `GATE_QUOTE_COMMAND` (item 7) está **triplicado**,
byte a byte, em `go/checks.go:125`, `node/checks.js:20-21` e `python/checks.py:232-235`, com apenas um
comentário em cada arquivo dizendo "precisa ser EXATAMENTE o mesmo literal" — nenhuma checagem
automatizada garante isso. Confirmei nesta auditoria que os três literais **estão** idênticos hoje
(comparação byte a byte via script). O risco é o dia em que alguém editar **um** dos três (ex.: trocar
a aspa simples por dupla num ajuste de outro item no mesmo arquivo) sem replicar nos outros dois: o
item 7 passaria a reportar `REPRODUCED` **por divergência de literal**, indistinguível no log de uma
divergência real de semântica de shell entre os runtimes — o próprio mecanismo de comparação, cujo
propósito é detectar divergência real, não sabe distinguir "os três shells avaliam o mesmo texto
diferente" de "os três scripts não recebem mais o mesmo texto".

**Remédio concreto (a melhor opção, não um checker de sincronia):** `run.ps1` já é o orquestrador que
chama os três — ele deveria possuir o literal como única fonte de verdade e **passá-lo como argumento**
para `checks.go`/`checks.js`/`checks.py` (os três já leem `os.Args`/`process.argv`/`sys.argv`), em vez
de cada um hardcodar sua própria cópia. Isso elimina a classe inteira, não apenas detecta o desvio
depois que ele acontece.

**Duplicação secundária, cosmética:** o step "Fixar caches fora do home sintético (AC12)"
(`quality.yml`) é copiado verbatim entre `windows-full-suites` e `windows-defect-reproduction` — cinco
linhas idênticas em dois jobs. Baixo risco (é configuração estática, não lógica), mas é candidato a
composite action/reusable step se um terceiro job de Windows aparecer.

`probe.go` × `checks.go`: duplicação mínima e aceitável — só o padrão `printMode`/`fmt.Printf` de
formatação é repetido em espírito (não em código), e os dois arquivos têm propósitos declaradamente
distintos (sonda sem veredito vs. suíte com veredito). Não vejo drift esperado aqui.

## 6. Manutenibilidade do `run.ps1` (342 linhas)

Menos problemático do que o tamanho sugere: a estrutura por item, com `Add-Result`/`Run-Capture` como
únicos dois helpers e comentários de bloco por item citando a linha de produto correspondente, é
navegável — um mantenedor procurando "item 7" acha o bloco pelo comentário, não precisa ler o arquivo
inteiro. O ponto frágil real não é o tamanho, é a **dependência de parsing textual frágil**: todos os
vereditos de `checks.py` dependem de `-match "VERDICT=REPRODUCED"` contra uma string livre impressa
por `print()`. Isso falha **seguro** hoje (qualquer desvio de formatação cai em `INCONCLUSIVE`, que já
força vermelho — não em `ABSENT`), então não é falso verde; mas é um ponto de fragilidade a considerar
se a suíte crescer: uma saída estruturada (`VERDICT_JSON={"item":"1","verdict":"REPRODUCED"}`, ou um
arquivo JSON por checagem) seria mais robusta a mudanças de formatação futuras do que grep de texto
livre. Não bloqueante — é acompanhamento.

---

## Resumo por severidade

### Bloqueante (impede tirar o PR do rascunho)
1. **`quality.yml:374`** — `pip install pyyaml` nomeado à mão no job `windows-defect-reproduction`
   reintroduz o defeito de drift que o ML-1D já corrigiu na camada 1 duas commits atrás, com o mesmo
   mecanismo (`ModuleNotFoundError`) e o mesmo efeito colateral: os itens 1, 4, 5 e 6 vão para
   `ABSENT`/`INCONCLUSIVE` por vacuidade (nunca alcançam o código medido), não porque os defeitos
   foram corrigidos. **Remédio:** trocar por `pip install pypi/`, igual à linha 219.

### Acompanhamento (viram REQ/ML futuro, não bloqueiam este PR)
2. **`run.ps1:287-294` (item 10)** — `ABSENT` não confirma que `roadmap move` de fato sincronizou o
   frontmatter, só que a linha final não contém `\`. Adicionar assert positivo de que o destino
   (`wip/`) foi atualizado.
3. **`run.ps1` — piso de contagem ausente.** `if ($results.Count -ne 11) { exit 1 }` antes do bloco
   de saída fecha a classe de vacuidade por perda silenciosa de item (subsume o caso degenerado do
   item 7 abaixo).
4. **`run.ps1` — sem pino de veredito esperado.** Nada compara a execução atual contra a linha de
   base 8/8 registrada no roadmap; um flip de veredito não é sinalizado automaticamente como
   suspeito.
5. **`GATE_QUOTE_COMMAND` triplicado** (`go/checks.go:125`, `node/checks.js:20-21`,
   `python/checks.py:232-235`) — hoje sincronizado (confirmado byte a byte), sem checagem automática
   contra divergência futura. Melhor remédio: `run.ps1` passa o literal como argumento em vez de cada
   script hardcodar sua cópia.
6. **Item 7 degenerado (menor).** Se `go`, `node` e `python` falharem simultaneamente ao iniciar (não
   "sh ausente" — o próprio binário do runtime inacessível), `Get-GateQuoteToken` colapsa para o mesmo
   literal de fallback nos três, e o item reporta `ABSENT` em vez de sinalizar falha do instrumento.
   Não é falso verde de job (itens 1/4/5/6 cobririam com `INCONCLUSIVE`, ou bloqueante #1 acima já
   cobre a via mais provável), mas corrompe a leitura isolada do item 7. Resolvido pelo piso de
   contagem (#3) só parcialmente — recomendo também comparar `$goToken`/`$nodeToken`/`$pyToken`
   contra o padrão `<sem-STDOUT_BEGIN/END:` antes de concluir `ABSENT`.
7. **Duplicação cosmética** do step "Fixar caches fora do home sintético" entre os dois jobs de
   `quality.yml` — candidato a composite action se um terceiro job de Windows surgir.
8. **`os.IsPermission`/`EACCES` amplos demais (ML-2A)** — podem mascarar um erro de permissão de
   fixture não relacionado a Developer Mode como "skip esperado". Risco baixo, falsificação atual não
   o expõe.
9. **`run.ps1` sem `trap`** — `$ErrorActionPreference = "Continue"` absorve exceções .NET não
   tratadas sem marcador visual distinto de "instrumento crashou" vs. "defeito reproduzido"; ambos
   viram ruído dentro de `INCONCLUSIVE`. Não causa falso verde, prejudica legibilidade.
10. **Parsing textual frágil** (`-match "VERDICT=..."`) — falha seguro hoje, mas frágil a longo
    prazo; considerar saída estruturada se a suíte crescer.
