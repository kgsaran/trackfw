---
status: wip
date: 2026-09-24
req: "docs/req/REQ-2026-09-24-treze-rotulos-falham-no-censo-de-windows-e-cinco-sao-setup-que-aborta-o-cenario-inteiro.md"
squad: [hades-tf, ares-tf, artemis-tf]
---

# Roadmap: treze rótulos falham no censo de Windows, e cinco são `setup`

> Criado em: 2026-09-24 | Status: wip

REQ: `docs/req/REQ-2026-09-24-treze-rotulos-falham-no-censo-de-windows-e-cinco-sao-setup-que-aborta-o-cenario-inteiro.md`

## Diagnóstico

Primeira triagem do cluster de Windows com **número real**, não estimativa. Censo `36036473391`
(`main`, 8/8 shards, sem `TOTAL INCOMPLETO`): **OK=347 · FAIL=11 · 13 rótulos distintos em FAIL ·
3 ausentes**.

🔴 **A hipótese que organiza tudo, e que pode cair:** 5 dos 13 são `setup-*`, e um `setup` que falha
**aborta o cenário inteiro**. Se ela se confirmar, a população real é de poucas causas. Se cair,
isso fica escrito e a triagem segue rótulo a rótulo.

⚠️ **VM investiga, CI mede.** O mecanismo se acha na VM (`ssh powershell-vm`); **todo número que
virar afirmação sai do `windows-census.yml`**.

⚠️ **Custo de CPU:** teto de 2 agentes simultâneos, `go test` só do pacote tocado, `make quality`
apenas na barreira do arquiteto.

---

## Wave 0 — Triagem por mecanismo (1 ML, bloqueia tudo)

### ML-0A — Agrupar os 16 rótulos por causa, e medir a hipótese do `setup`
**Owner:** `hades-tf`
**Status:** ✅ Concluído — auditado em 2026-09-24
**Entregue:** `docs/seguranca/2026-09-25-triagem-cluster-windows.md`
**Arquivos afetados:** nenhum de produto — entrega `docs/seguranca/2026-09-25-triagem-cluster-windows.md`

**Ações:**
1. **Rode os gates na VM** e agrupe por **mecanismo**, não por nome de rótulo. Os 13 em `FAIL` e os
   3 ausentes estão na REQ.
2. 🔴 **Meça a hipótese do `setup`**: quantos dos 8 não-`setup` fecham quando o `setup` do mesmo
   cenário passa? Force o setup a passar (ou rode o cenário a partir do ponto seguinte) e veja o que
   sobra. **Se nada fechar, a hipótese cai e isso vira o achado principal.**
3. 🔴 **Confirme ou descarte o #307.** Ele afirma, com medição de terceiro, que
   `check-release-tag-parity.sh` falha no Windows porque o `ln -s` degrada para cópia e o `python3`
   copiado não inicia — e que **a guarda de vacuidade acusa o `git` quando o culpado é o `python3`**.
   4 dos 5 `setup-*` são desse gate. Se for o mesmo mecanismo, **absorva**; se não, escreva a
   diferença.
4. **#308 é candidato** para `git-branch-guard-dedup/*` (MSYS expandindo `{owner}/{repo}` ao
   reconstruir `argv` de pai nativo). Candidato, não membro.
5. Para cada grupo: a frase *"corrijo esta causa, exatamente estes rótulos fecham, e nenhum outro"*.
6. **Threat model:** vários rótulos são de controle de segurança (`credential-guard-*`,
   `git-branch-guard-*`). Para cada um, **qual garantia fica sem prova no Windows** e se há cobertura
   equivalente por outro caminho.

**Critérios de aceite:**
- [x] Tabela com **16 linhas**: 14 reais + 2 marcadas **fantasma**, com evidência. Cinco grupos:

| grupo | n | mecanismo | categoria |
|---|---|---|---|
| **G1** | 6 | `check-release-tag-parity`: filho nativo morre no `NO_FORGE_PATH` curado (`0xC0000005`) | A — recusa ruidosa |
| **G2** | 4 | `check-update-parity` morre no `ln -sf` do Cenário 9 sob `set -euo pipefail` | A — recusa ruidosa |
| **G3** | 1 | `chmod 0644` não retira o bit — `ls -la` mostra `-rwxr-xr-x` **depois** | **C — passagem vacuosa** |
| **G4** | 2 | grafia MSYS de `HOME` cruza para o Go; `normalizeGuardPath` só converte com letra de unidade | 🔴 **D — produto** |
| **G5** | 1 | dreno `read -r -t 2 -d ''` não sustenta 200 KB; **guard sai 0** | 🔴 **D — produto** |
- [x] 🔴 **A hipótese CAIU, por dois lados.** Fonte: `falsify_fail_point` **retorna 0** sob
      `TRACKFW_FALSIFY_ENUMERATE=1`, o modo do censo — não aborta nada. Log: o shard 3 imprime **nove
      rótulos** depois de `FAIL [setup-s75]`, incluindo cenários inteiros.
      **O que o setup reprovado causa de fato:** o `echo OK` do `else` não sai — explica **os 3
      ausentes** e **zero** dos 11 FAIL. O que sobrevive é **causa comum**, não encadeamento:
      corrigir o braço de setup não fecharia os outros; **fechar o gate fecha os dois**
- [x] **#307 ABSORVIDO** — os dois bloqueios dele explicam **10 dos 14** (G1 e G2), com **três
      diferenças medidas** escritas. **#308 DESCARTADO** com a diferença: o G4 diverge **inteiramente
      dentro do Go**, sem `argv`, sem chaves e sem processo nativo no caminho
- [x] Uma por grupo, **com previsão de delta pareada** (G1: `FAIL −4` e `OK +2`, **não** −6)
- [x] **Quatro** categorias, não duas — a distinção *falha ruidosa* ≠ *garantia sem prova* se abre
      em A (recusa ruidosa, risco baixo), C (**passagem vacuosa**: `test -x` verdadeiro nos dois
      braços, o `OK` não prova nada) e D (**controle ausente no Windows**)
- [x] 🔴 Nenhuma implementação — zero arquivos em `internal/` ou `scripts/`
- [x] 🔴 `make quality` não rodado; **VM não usada** — todo o mecanismo saiu de fonte + log do censo

**Comandos de validação:** `trackfw barrier <roadmap> --wave 0`

**Auditoria do arquiteto (medida por mim):**

🔴 **O achado mais importante é uma correção a mim, e é o erro nº 11 do instrumento nesta campanha
— literalmente o que a minha própria memória descreve.** Eu enumerei os FAIL com
`grep -ao 'FAIL \[falsify/…'` **sem âncora**, e capturei texto **citado dentro** de mensagens
`PROOF …/non-vacuity`. Remedi com a âncora correta para o formato do `gh run view --log` (as linhas
começam com `job\tstep\ttimestamp`, então `^FAIL` dá **zero** — outra armadilha):

```
grep -aoE '[0-9]Z FAIL \[falsify/[^]]*\]'   →  11 ocorrências, 11 distintos
```

E `credential-guard-script-integrity/detected` aparece como **`OK`**. 🔴 **A inflação caiu
exatamente sobre a superfície de segurança** — os dois fantasmas eram os dois controles de
integridade de script, ambos **verdes**. Uma wave inteira teria sido desenhada para um buraco
inexistente.

**Decisões minhas, tomadas sobre as três que ele levantou:**

1. **#421 e o `.venv` ENTRAM na REQ.** As duas exclusões eram minhas e estavam erradas: o bit em
   NTFS é a **mesma causa** do G3 (e o censo **confirma a #421 em x64**, que a issue declarava não
   medido), e o Cenário 9 **não usa venv** — é symlink pendurado sintético, mesmo mecanismo do #307.
   *"Está fora do escopo declarado"* é exatamente o que a Regra Dura recusa.
2. **População corrigida para 14** na REQ, com a razão escrita — senão a recontagem procuraria um
   delta que nunca existiu.
3. **A Forma B vira ML próprio nesta REQ** (abaixo).

**Ordem aceita: G4 e G5 primeiro.** O argumento dele é o certo — os outros três são gates que não
rodam e **falham alto**; inverter gastaria as primeiras waves deixando o CI verde enquanto dois
defeitos de **produto** seguem vivos.

⚠️ **E ele declarou o que não determinou:** a razão do `0xC0000005`, qual elo do G4 dispara primeiro,
e a causa do G5 entre orçamento de tempo e semântica de `read -d ''`. A ausência de cobertura
equivalente do G4 está marcada como **presunção, não medição** — é o primeiro item a verificar na
wave do G4.

---

### ML-0B — A Forma B: o rótulo afirma que o gate passou limpo, e o gate reprovou
**Owner:** `artemis-tf`
**Status:** ⬜ Pendente

Achado do ML-0A, **fora dos 14 e fora de qualquer REQ**: nos Cenários 87 (`:6066`) e 158 (`:6123`) o
`echo OK` está **fora do `if`**. A baseline reprova, e o rótulo que afirma *"o gate passa limpo"*
imprime `OK` na linha seguinte.

🔴 **É o único falso verde do cluster.** Os braços de detecção não são vazios — o que se perde é o
**discriminante de delta único**.

**Detector mecânico, já nomeado:** a Forma B **não chama** `falsify_count_success`, logo a contagem
de `^OK` no log **≠** tally. A divergência é medível sem inspeção.

**Critérios de aceite:**
- [ ] Os dois sítios corrigidos: o `OK` passa a ser condicional à baseline ter passado
- [ ] 🔴 **Varredura da forma** — `echo OK` fora do `if` que decide o veredito — com veredito por
      sítio; a divergência `^OK` vs tally é o instrumento
- [ ] Falsificação nas duas direções
- [ ] 🔴 Uma frase por teste novo
- [ ] 🔴 **NÃO rodar `make quality`**

---

## Wave 1 — Os dois grupos de produto (2 MLs em paralelo)

### ML-1A — G4 · **PREMISSA REFUTADA: não é produto, é fixture**
**Owner:** `apolo-tf`
**Status:** ✅ Concluído — auditado em 2026-09-24 · **nenhuma linha de produto alterada, com razão medida**

🔴 **O executor declinou a tarefa que recebeu, e estava certo.** Ele foi despachado para fechar a
gramática de `normalizeGuardPath`, e mediu que fechá-la **fecharia zero rótulos**.

**O que a Wave 0 concluiu por construção, sem executar:** que o `HOME` de grafia MSYS chega ao Go
**sem** letra de unidade, e que a divergência é de **separador**.

**O que a sonda na VM mediu:**
```
NORM_JSON = "/tmp/trackfw-probe2.gmIUJB/…"
NORM_COMP = "C:/Users/Lab/…"      ← 🔴 a conversão \ → / JÁ ACONTECEU
MATCH     = false
```

O MSYS **converte a variável de ambiente** ao lançar processo nativo, logo
`hasWindowsDriveLetterPrefix` é **verdadeiro**, o braço roda, e a divergência **sobrevive**. Não é
gramática de separador: é **espaço de nomes**. O heredoc do fixture grava `/tmp/…` porque
**conteúdo de arquivo o MSYS nunca converte**; o Go calcula `C:/Users/…`. Mesmo arquivo, duas
montagens.

🔴 **Consequência que fortalece o alerta original:** **nenhuma regra de string pode casar as duas
grafias**, porque a diferença é de montagem, não de sintaxe. Toda "correção" que as faça casar é
**necessariamente** afrouxamento semântico — o argumento contra comparar por *basename* fica mais
forte, não mais fraco.

**A/B com o binário real na VM, variável única (só o conteúdo do JSON muda):**

| braço | `command` no JSON | resultado |
|---|---|---|
| A | `/tmp/…/guard.sh` | entrada de projeto **gravada** — dedup não disparou · **reproduz o FAIL** |
| B | `C:/Users/Lab/…/guard.sh` | entrada **ausente** — dedup disparou · **o produto funciona** |

**Os três itens em aberto da Wave 0, respondidos:**

1. **Qual elo dispara:** o **(b), a comparação**. O `readGlobalHookJSON` **sucede** (`err=<nil>`) —
   o `fail-open` de leitura não participa.
2. **Cobertura Go: a presunção CAI — e a razão vale mais que o veredito.** Há 6 referências e **34
   casos** em `guard_path_normalize_test.go`. O G4 ficou invisível **não por ausência de teste**, mas
   porque **um teste fixa a forma como comportamento intencional** (`:84`):
   ```go
   {"relative path with backslash, no drive letter, untouched", `scripts\guard.sh`, `scripts\guard.sh`},
   ```
   🔴 A lição *"teste que afirma o defeito como contrato defende o defeito na revisão"* continua
   valendo em geral — **mas não se aplica a este caso**, e propagá-la aqui teria levado alguém a
   remover uma garantia real.
3. **`fail-open`: NÃO mudar.** `false` → "não instalado" → entrada de projeto **gravada** → guard
   roda **duas vezes** (benigno). Inverter → entrada **pulada** → guard possivelmente **ausente**:
   exatamente o bypass. Aqui a direção permissiva é a **segura**. Se alguém inverter assim mesmo,
   passa a reprovar todo cenário sem `settings.json` global legível — incluindo dois braços hoje
   verdes.

🔴 **A linha que precisa ficar na tabela de risco, e independe de culpa:** com `MATCH=false`, a
garantia *"global instalado ⇒ entrada de projeto pulada"* **não tem prova nenhuma no Windows hoje**.

**Critérios de aceite:**
- [x] Regra **declinada com razão medida** — fechá-la mexeria em decisão aprovada por barreira e
      fecharia zero rótulos
- [x] Falsificação nas duas direções, por instrumento **comportamental na VM** — obrigatório, porque
      a divergência é propriedade de **montagem** e não existe num host POSIX
- [x] Presunção de cobertura **refutada**, com a razão
- [x] Veredito do `fail-open` escrito, com o que passa a reprovar se for invertido
- [x] `go test ./internal/generators/` RC=0 (linha de base intocada)
- [x] 🔴 **Zero testes novos** — o único concebível fixaria artefato de harness como contrato de
      produto, a mesma patologia medida no teste existente. Sem frase possível, sem teste

### ML-1B — G5 · O guard aprovava o que não conseguiu ler
**Owner:** `ares-tf`
**Status:** ✅ Concluído — auditado em 2026-09-24

🔴 **O defeito é mais grave do que a triagem dizia, e eu confirmei no macOS.** Escritor lento (JSON
em 5 pedaços de 1 s), com o dreno antigo:

```
len_lido=178  payload=446
🔴 COMANDO INVISIVEL -> guard aprova em silencio
```

O `"command":"git push"` fica **fora** do que o guard leu. **Não é defeito de Windows** — é um
controle de segurança que aprova o que não leu, em qualquer plataforma, quando o escritor é lento.
O Windows só tornava fácil disparar: o `read` de pipe no MSYS custa ~90x mais por syscall
(~14,7 KB/s contra ~1,3 MB/s).

**Causa separada como pedido — é orçamento, não semântica:**

| medição | macOS | Windows |
|---|---|---|
| `read -t 600 -d ''` de pipe | 0,151 s · 200000 B | **13,6 s** · 200000 B |
| `read -t 2 -d ''` de pipe | 0,15 s · 200000 B | 🔴 **2,1 s · rc=142 · 25397 B** |
| pipe × arquivo (fd seekable) | — | 13,6 s × **0,618 s** |

Com tempo, o `-d ''` lê até o EOF real. **A semântica está certa; o orçamento não.**

**Contrato escolhido, e o `-t` NÃO foi aumentado:** a janela de 2 s passou de **total** para
**ociosa**, renovada a cada byte. O valor é o mesmo; o que mudou é a natureza — *"quanto o escritor
pode ficar sem enviar nada"* em vez de *"quanto a transferência inteira pode levar"*. Mais
**fail-closed**: truncado + sem argv ⇒ `deny` + `exit 2`.

**Custo escrito:** invocação legítima que fique 2 s **sem enviar um único byte** passa a ser
bloqueada com razão visível — antes era **aprovada sem o guard ter lido o que aprovava**. A recusa
vem **depois** do no-op (ADR-2026-08-17 preservada) e **argv isenta**.

**E ele corrigiu uma alegação falsa no próprio código:** o comentário afirmava, como medição, que
`read -t` preserva o prefixo lido em bash 3.2. **Falso, medido** — no 3.2 o truncamento é
indetectável, e isso ficou declarado com a razão.

**Critérios de aceite:**
- [x] Causa medida: **orçamento**, não semântica — com o custo localizado no **pipe**
- [x] Contrato decidido e escrito, com o que ele custa
- [x] Falsificação nas duas direções, com **reconciliação obrigatória**: pós-correção, 200 KB deixou
      de ser "ilegível" e virou "lento" — a direção 2 teve de ser reconstruída como **escritor
      travado**. Sem essa frase, um auditor lê substituição de cenário
- [x] 🔴 A correção **não** é aumentar o `-t`
- [x] Efeito no consumidor declarado: quem tem o script antigo segue com fail-open até
      `trackfw agents update`; a regra de integridade **torna a pendência visível**
- [x] 6 testes novos, uma frase cada

### ML-1E — Os 2 sítios de `scripts/` que ficaram para trás
**Owner:** `artemis-tf`
**Status:** ✅ Concluído — auditado em 2026-09-24

O dreno vive em **4 sítios byte-idênticos**; o ML-1B atualizou os 2 de `internal/`.

🔴 **Ele corrigiu uma premissa do meu handoff:** eu disse que
`TestGitBranchGuardScriptReference_MatchesGenerator` provaria a regeneração. **Não prova** — ele
compara **duas constantes Go** e nunca olha o arquivo em disco. Quem prova é a regra
`git_branch_guard_script_integrity`, e ele falsificou nas duas direções (script antigo → aviso;
regenerado → aviso some).

**Critérios de aceite:**
- [x] Script **regenerado**, não editado à mão — **auditei**: `cmp` contra
      `GenerateGitBranchGuardScript` → **byte-idêntico**; bit de execução preservado
- [x] Sabotagem do Cenário 65 **ainda mede o que deve** — A/B: sabotagem real → `escritor_erro=1`;
      sabotagem-identidade → `FAIL … cenário vácuo`. A guarda de vacuidade **re-falsifica**
- [x] Literal alvo ocorre **exatamente 1 vez** no gerador, verificado com o mesmo `count()` do
      `corrupt_literal`
- [x] Comentário de `:4612` atualizado, **com a reconciliação** do que 200 KB passou a significar
- [x] `go test ./internal/generators/` e `./internal/validator/` RC=0
- [x] 🔴 Nenhum teste novo; a frase é do braço de detecção do Cenário 65
- [x] **Conferência extra que eu não pedi:** não há **quinto sítio** (varredura repo-wide), e o
      comentário novo **não perturbou o instrumento** — A/B do chunker: **266 rótulos, `diff` vazio**

**Auditoria do arquiteto — um susto que não era defeito:**

`trackfw validate` **acusava** divergência do script. Medido: o `cmp` contra o gerador dá
**byte-idêntico**, e com o binário **compilado da árvore** o aviso **some** (0 ocorrências). É o
**binário instalado** que carrega o template antigo — mesma versão `8.0.1`, compilada antes. Não há
divergência real.

### ML-1C — O fixture gravava grafia que o binário nunca produz
**Owner:** `artemis-tf`
**Status:** ✅ Concluído — auditado em 2026-09-24 · **os 2 rótulos do G4 passam na VM**

**Linha de base antes de editar** (VM, árvore intocada, `ENUMERATE=1` para que o braço 4 chegasse a
ser avaliado):
```
FAIL  git-branch-guard-dedup/baseline-skips-project-entry
FAIL  git-branch-guard-dedup/double-slash-tolerance
```
**Depois:** `CHUNK_RC=0`, os 4 `OK` + `PROOF …/non-vacuity`, e `CHUNK_COMPLETE`.

🔴 **Ele divergiu do meu handoff, com razão medida — e a minha instrução teria falhado no CI.** Eu
mandei converter **só o heredoc**, deixando o MSYS converter o `HOME`. Isso depende de as duas
conversões serem byte-iguais, e **não são**: a VM ARM64 dá nome longo
(`C:\Users\Lab\AppData\…`), o censo x64 dá **8.3** (`C:/Users/RUNNER~1/…`). **Minha versão
passaria na VM — satisfazendo o critério "medido na VM" — e seguiria vermelha no CI.** Ele controla
as **duas pontas a partir de uma string só**, tirando o MSYS da fronteira.

**Determinismo POSIX sobrevive por construção:** sem `cygpath`, `to_native_path` devolve a entrada
**inalterada**.

**E o `//` do braço 4 virou asserção, não presunção:** `cygpath` **colapsa** `//` embutido, então
ele converte a **base** e injeta o `//` depois — com guarda ancorada no **segmento**
(`//s67-fake-home-installed-slash`), nunca em `//` solto, que um prefixo `C:/` satisfaria por
acidente.

🔴 **Achado colateral, mesma causa:** `detection-catches-regression` aparecia **`OK` antes e depois**
— e antes era **vacuoso**. Compartilha o `$HOME` sintético do braço 1; com `MATCH=false` a entrada
de projeto reaparecia **independente** de o binário estar corrompido. **Regra transferível:** num par
baseline/detecção que compartilha fixture, **detector verde com baseline vermelho não prova nada**.

**Critérios de aceite:**
- [x] Grafia nativa nas duas pontas; determinismo POSIX preservado por construção
- [x] O `//` continua exercitado — **e agora é assertado**, com `PROOF …/non-vacuity`
- [x] Os 2 rótulos passam **na VM**, com linha de base antes/depois
- [x] A/B do conjunto de rótulos em **N=4, N=8 e N=24**: `265` vs `264` — **+1, 0 removidos**

### ML-1D — O teste-contrato: **comportamento desejado**, caso mantido, nome reescrito
**Owner:** `artemis-tf`
**Status:** ✅ Concluído — auditado em 2026-09-24

**Veredito: pina garantia, não acidente.** Sem âncora de letra de unidade, `\` é **byte de nome de
arquivo**; traduzi-lo faria `scripts\guard.sh` e `scripts/guard.sh` — dois arquivos genuinamente
diferentes em POSIX — compararem iguais, o falso *"já instalado"* que desarma o dedup em silêncio.

🔴 **A evidência que decide não é o agrupamento do teste, é o doc comment de `normalizeGuardPath`**,
que lista *"A relative path containing `\`"* entre os **known residuals**, declara *"Direction is
always TIGHTENS … never loosens"* e ainda avisa *"Do not treat rediscovering these three as a new
finding"*. **Aceito de propósito, não esquecido.**

Nome novo, declarando a garantia em vez da implementação:
```
"no drive-letter anchor: backslash is a filename byte, so two genuinely
 different relative paths never compare equal"
```

**Teste novo:** `TestSamePathCommand_MSYSAndNativeSpellingsMustNotMatch`, falsificado por
`go test -overlay` com `samePathCommand` afrouxado para comparar por **basename** — o afrouxamento
que o parecer proíbe: **`LOOSE_RC=1`**, duas asserções reprovam; árvore íntegra passa.

**Critérios de aceite:**
- [x] Veredito escrito, com a evidência do doc comment
- [x] Nome passa a dizer a garantia — **auditei na linha 96**
- [x] 🔴 `normalizeGuardPath` **não** relaxado — `git diff --name-only | grep -c agentfiles.go` → **0**
- [x] `go test ./internal/generators/` RC=0

**Auditoria do arquiteto:** confirmei o teste novo (`:173`), o nome reescrito (`:96`), o
`agentfiles.go` intocado e o pacote verde.

⚠️ **Duas ressalvas dele, registradas e não resolvidas aqui:** 3 testes reprovam na VM Windows, e ele
**provou** que são pré-existentes (`go test -overlay` com a versão `HEAD` → os mesmos 3). Causa
diferente, então **não** entram nesta REQ. E o `//` foi medido em **ARM64** enquanto o censo é x64 —
por isso a sobrevivência virou **asserção no script**: se o x64 divergir, o censo reporta **FAIL**
em vez de um `OK` vacuoso.

### ML-1C-antigo — O fixture do Cenário 67 (superseded) grava grafia que o binário nunca produz
**Owner:** `artemis-tf`
**Status:** ⬜ Pendente

Consequência direta do ML-1A. `scripts/check-gates-falsify.sh:4722-4726` (braço 1) e `:4836`
(braço 4) gravam o `command` do `settings.json` global em grafia MSYS, via heredoc — e **conteúdo de
arquivo o MSYS nunca converte**.

**Ação:** gravar o `command` em grafia nativa (`cygpath -m` quando disponível), de modo que o
fixture represente o que o binário de fato produz.

🔴 **Dois cuidados, ambos medidos:**
- o comentário do braço 1 diz que o `HOME` é *"deliberately corrupted … so this baseline stays
  deterministic across platforms"*. **Não quebre essa intenção** — o determinismo em POSIX precisa
  sobreviver;
- o braço 4 existe para testar **barra dupla** (`//`). A correção não pode apagar o caso que ele
  exercita.

**Critérios de aceite:**
- [ ] Os dois sítios gravam grafia nativa no Windows, mantendo o determinismo em POSIX
- [ ] O caso de **barra dupla** do braço 4 continua sendo exercitado
- [ ] Os 2 rótulos do G4 passam no Windows — medido, não presumido
- [ ] 🔴 Uma frase por teste novo, ou ausência declarada
- [ ] 🔴 **NÃO rodar `make quality`**

### ML-1D — O teste que fixa o defeito como contrato
**Owner:** `artemis-tf`
**Status:** ⬜ Pendente

`internal/generators/guard_path_normalize_test.go:84` afirma como **intencional** a forma que o ML-1A
mediu ser irrelevante para o G4 — e, pior, **defende-a na revisão**.

- [ ] Veredito: o caso pina comportamento desejado ou congela um acidente? **Com a razão escrita**
- [ ] Se for acidente, o caso sai ou muda de nome para dizer o que realmente afirma
- [ ] 🔴 Não relaxar `normalizeGuardPath` para "fazer passar" — o ML-1A mediu que nenhuma regra de
      string resolve

---

## Wave 1+ (waves seguintes) — Um grupo por wave
> Dependências: Wave 0 auditada. **O detalhamento é escrito depois do ML-0A** — ML detalhado sobre
> mecanismo não medido é o erro do `IsAbs`.

**Critérios que já valem, qualquer que seja o agrupamento:**
- [ ] Um **grupo** por wave, não um rótulo por wave
- [ ] Falsificação nas duas direções, exercitada no **Windows**
- [ ] 🔴 **Recontagem no CI ao fim da wave**, com o delta **atribuído** ao grupo — e se o delta não
      bater com o previsto, isso é achado, não ruído
- [ ] 🔴 Nenhum rótulo silenciado por `skip` para reduzir contagem. Supressão exige nomear a
      garantia não exercitada

## Barreira final

Revisão `hefesto-tf` e `hades-tf`, auditoria do arquiteto, `trackfw barrier`, CI verde.
---

## Wave 2 — G3, o único da categoria **C — passagem vacuosa**

### ML-2A — O cenário do bit passa a falhar alto em vez de aprovar sem medir
**Owner:** `ares-tf`
**Status:** ✅ Concluído — auditado em 2026-09-24 · Cenário 181

**Saída escolhida: falhar alto (C → A).** Uma sonda de representabilidade do bit — `0755` →
`test -x` → `0644` → `test -x`, construída **no mesmo mount das fixtures** — decide **antes** de
qualquer braço rodar. FS que não representa o bit ⇒ **dois** `FAIL` que **nomeiam a garantia não
exercitada**, e nenhum braço roda.

🔴 **`SKIP` foi descartado por razão MEDIDA, não estilística** — e a razão é bonita: o driver colhe
rótulos com `grep -oE '^(OK|FAIL|PROOF)…'` e **exige** por chunk todo literal extraído dos
`echo "OK|PROOF …"`. Uma linha `SKIP [falsify/<rótulo>]` **(a)** não é colhida → o rótulo vira
*"esperado AUSENTE"*, que é o diagnóstico de **chunk morto**; e **(b)** sai 0 → **a categoria C
reconstruída dentro da própria guarda**. Por isso os **dois** rótulos são emitidos no ramo
inconstruível: emitir só um deixaria o irmão como ausente.

**Prova de não-vacuidade com instrumento real, não stub** — ele reproduziu um FS que não representa
o bit **em darwin**, com imagem FAT32 esparsa de ~6 GB (200 MB morriam em `no space left` ao
compilar o binário sabotado):

| variante | resultado |
|---|---|
| `HEAD`, **braço de detecção removido** | `OK`, **rc=0 — verde inteiro** 🔴 a vacuidade, reproduzida |
| `HEAD`, completo | `OK` vácuo + `FAIL` — o estado do censo |
| **novo**, braço removido | 2× `FAIL` nomeado, **rc=1** |
| **novo**, completo | 2× `FAIL` nomeado, **rc=1** |

**POSIX (APFS):** `chunk_21` completo, **rc=0**, os dois braços `OK` — a garantia continua
exercitada de verdade.

**Custo escrito:** o censo passa a mostrar **2 FAIL** onde mostrava 1 `OK` vácuo + 1 `FAIL`.
Aceitável porque o `windows-census.yml` é `workflow_dispatch` + `continue-on-error` — **nunca
bloqueia merge**. 🔴 **Se o censo virar bloqueante, esta decisão precisa ser reaberta** — aí seria
vermelho permanente sem ação disponível.

**Auditoria do arquiteto (medida por mim):**

| afirmação | como confirmei |
|---|---|
| testemunha de CI existe | log do censo: `-rwxr-xr-x … /s181-det/…/trackfw-validate.sh` **depois** do `chmod 0644`, em `windows-latest` x64 |
| a atribuição ao `noacl` é presunção | `grep -ci 'noacl\|findmnt\|usertemp'` no log inteiro → **0** |
| conjunto de rótulos inalterado | `gen-falsify-chunks.py` N=8, `HEAD` × árvore → **265 = 265, diff vazio** |
| sintaxe | `bash -n` OK |

**Sobre a #421:** a testemunha **fecha o "não medido"** dela — o `chmod` não retira o bit também em
**x64 no runner**, não só na VM ARM64. Mas **não** atribui o mecanismo ao `noacl`.

**Critérios de aceite:**
- [x] O cenário distingue **fixture inconstruível** de **controle passou**, com custo escrito
- [x] 🔴 Sem skip silencioso — e o `SKIP` foi **descartado com medição**
- [x] A garantia continua exercitada em POSIX
- [x] Falsificação da **vacuidade**, com instrumento real
- [x] Veredito sobre a testemunha de CI: suficiente para o efeito, insuficiente para o mecanismo
- [x] 🔴 Nenhum teste Go novo; a frase é da sonda

---

### Dois itens que ficam registrados aqui, e são decisão minha

**1. Falta uma linha no censo, e ela fecha a atribuição da #421.** Uma linha `findmnt -T "$TMPDIR"`
(ou `mount`) no `windows-census.yml` tira o `noacl` de presunção para medição. **Entra no próximo ML
que tocar o workflow** — não vale um ML só para isso, e não vale esquecer.

**2. 🔴 O braço de baseline roda `$ROOT_DIR/bin/trackfw` — o binário COMMITADO.** Achado reportado
pelo ML-2A e **não corrigido, corretamente** (não expandiu escopo). O veredito do baseline atesta o
que estiver em `bin/`, **não** a árvore `internal/` — e o mesmo mecanismo aparece em **43 sítios** do
arquivo.

**Mecanismo distinto** de tudo nesta REQ (staleness de artefato, não fronteira MSYS), então **não
entra aqui** — mas é grande demais para virar só um parágrafo. Vira **issue própria**, com a
medição, depois do PR desta REQ.

