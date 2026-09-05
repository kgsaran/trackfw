---
title: "ML-7A — mecanismo do dedup `//` (G12): separador misto, não a barra dupla em si"
date: 2026-09-05
author: artemis-tf (QA — investigação, nenhuma correção aplicada)
status: investigação concluída — mecanismo identificado
---

# Por que `TestGBGDedup_Claude_SkipsProjectEntry_ToleratesDoubleSlashInStoredCommand` falha no Windows

> 🔴 Documento de investigação pura. Nenhum arquivo de produto, teste, fixture ou gate foi alterado.
> Nenhuma operação de git foi executada por este agente. `scripts/windows-repro/**` não foi tocado.

## Mecanismo, em uma frase

`normalizeGuardPath` (e os dois espelhos, `_normalize_guard_path` em Python e `normalizeGuardPath` em
Node) só colapsa sequências do caractere `/`; ele **nunca** converte `\` para `/` nem o inverso — e no
Windows, `filepath.Join`/`path.win32.join`/`ntpath.join` (usados para computar o caminho "já
instalado globalmente") **sempre** produzem separador nativo (`\`) em cada fronteira de segmento,
mesmo quando os argumentos de entrada não tinham nenhum. O valor **armazenado** no fixture (e num
`~/.claude/settings.json` editado à mão, ou escrito por uma versão antiga/outra ferramenta) mistura o
prefixo nativo do `$HOME` com sufixo em barras `/` — depois do colapso de `//`→`/`, ele continua tendo
barras `/` onde o comparando computado tem `\`. As duas strings normalizadas **nunca** ficam iguais no
Windows, então `samePathCommand` retorna `false`, `globalGitBranchGuardInstalledClaude()` (e as
contrapartes Codex/Gemini/Cursor/Copilot) retornam `false`, e `InjectClaudeHooks` reinsere a entrada de
projeto — exatamente o que o teste observa como falha ("project-scope git-branch-guard entry should
have been skipped").

**A barra dupla em si não é o problema — o colapso `//`→`/` funciona perfeitamente.** O nome do teste
descreve o sintoma errado. O defeito real é que `normalizeGuardPath` é **cego ao caractere `\`**: ele
nunca canoniza o *estilo* de separador, só a *redundância* dele.

## Medição (executada, não inferida — sem precisar de Windows real)

`filepath.Join`/`path.win32.join`/`ntpath.join` são módulos lexicais puros, sem chamada de SO — dá
para executar a forma **Windows** de cada um a partir de macOS/Linux, e depois aplicar a **cópia
literal** de cada `normalizeGuardPath`/`_normalize_guard_path` sobre o resultado. Feito para os 3
runtimes:

**Go** — via leitura do fonte instalado (`/usr/local/go/src/internal/filepathlite/path.go:65-138`):
`Clean` escreve `out.append(Separator)` em cada fronteira de segmento e finaliza com
`return FromSlash(out.string())` — os dois pontos garantem saída 100% `\` no Windows, mesmo que a
entrada tivesse `/`. Reproduzido com uma cópia byte-a-byte de `normalizeGuardPath`
(`internal/generators/agentfiles.go:1621`) alimentada com strings literais no formato Windows:

```
home              = C:\Users\RUNNER~1\AppData\Local\Temp\TestGBGDedup1234567
rawStoredCommand  = home + "//" + ".trackfw/scripts/trackfw-git-branch-guard.sh"
scriptPath        = home + "\.trackfw\scripts\trackfw-git-branch-guard.sh"   (forma que filepath.Join produziria)

normalizeGuardPath(rawStoredCommand) = ...TestGBGDedup1234567/.trackfw/scripts/trackfw-git-branch-guard.sh
normalizeGuardPath(scriptPath)       = ...TestGBGDedup1234567\.trackfw\scripts\trackfw-git-branch-guard.sh
EQUAL? = false
```

**Node** — executado de verdade, `path.win32.join` funciona em qualquer SO:
```
$ node -e "console.log(require('path').win32.join('C:\\Users\\foo\\Temp\\TestX','.trackfw','scripts','trackfw-git-branch-guard.sh'))"
C:\Users\foo\Temp\TestX\.trackfw\scripts\trackfw-git-branch-guard.sh
```
Aplicando a cópia literal de `normalizeGuardPath` (`npm/src/generators/hooks.js:1295`) ao resultado
acima e ao `raw = home + '//' + '.trackfw/scripts/trackfw-git-branch-guard.sh'`: `EQUAL? = false`,
mesmo padrão do Go (script fica todo `\`, raw mantém `/` depois do `home`).

**Python** — executado de verdade, `ntpath` funciona em qualquer SO:
```
$ python3 -c "import ntpath; print(ntpath.join('C:\\Users\\foo\\Temp\\TestX','.trackfw','scripts','trackfw-git-branch-guard.sh'))"
C:\Users\foo\Temp\TestX\.trackfw\scripts\trackfw-git-branch-guard.sh
```
Aplicando a cópia literal de `_normalize_guard_path` (`pypi/trackfw/generators/hooks.py:113`):
`EQUAL? = False`, mesmo padrão.

**Controle POSIX (medido, não fabricado):** o teste Go passa hoje nesta máquina (macOS):
```
$ go test ./internal/generators/... -run TestGBGDedup_Claude_SkipsProjectEntry_ToleratesDoubleSlashInStoredCommand -v
--- PASS
```
Consistente com o mecanismo: em POSIX, `filepath.Join`/`ntpath`-equivalente (`posixpath`/`path`) usa
`/` nativamente, então depois do colapso `//`→`/` as duas strings ficam byte-idênticas — não há
separador misto para expor o defeito.

## Discriminante adicional, gratuito, achado ao reler o próprio arquivo de teste

**Só a variante `//` falha; as variantes-irmãs de dedup passam.** `git_branch_guard_dedup_test.go`
tem `TestGBGDedup_Claude_SkipsProjectEntryWhenGlobalInstalled` e os pares Codex/Gemini/Cursor/Copilot
— todos usam **o mesmo** `dedupFixtureHome(t)` + `gbgDedupScriptPath(home)` (que já é o resultado de
`filepath.Join`, sem concatenação crua) para escrever o valor armazenado. Se a causa fosse resolução
de `$HOME`/`%USERPROFILE%` quebrada no Windows, ou algo estrutural no parsing/JSON, **todas** essas
variantes falhariam — G12 teria ~15 falhas (5 CLIs × 3 runtimes), não 3. O fato de **só** a variante
com concatenação crua falhar isola o separador como o único discriminante: nas variantes-irmãs, os
dois lados da comparação são produzidos por `Join`, então nunca há separador misto para expor o
defeito. Este argumento é mais forte que a identidade de fixture entre os 3 arquivos — não depende de
eu confiar que a fixture "faz o que o comentário diz", depende só de contar quantas variantes falham.

## O contrato documentado falha também no outro eixo que a doc-comment cita — não só no hand-edit

O comentário de `normalizeGuardPath` promete tolerar **"`$HOME` resolving with a trailing slash"**.
No Windows, esse cenário produz `home = C:\Users\foo\` (barra final nativa) e o código então concatena
`\.trackfw\...` via `Join` — gerando um `\\` duplicado na costura, o análogo Windows exato do caso
`//` que a função **trata corretamente** para `/`. Testado com a mesma cópia literal do algoritmo:

```
input : C:\Users\foo\\.trackfw\scripts\trackfw-git-branch-guard.sh
output: C:\Users\foo\\.trackfw\scripts\trackfw-git-branch-guard.sh
collapsed the doubled backslash? false
```

**Não colapsa.** A função só testa `r == '/'` — nunca `r == '\\'`. Isso significa que a tolerância
documentada está **inteiramente ausente no Windows nos dois eixos** (redundância de separador E
estilo de separador) para o cenário exato que a própria doc-comment nomeia como motivo de existir —
não é um cenário hipotético de "usuário talvez edite com `/`", é o cenário que a função já promete
cobrir e não cobre. Isso muda o veredito de "plausível" para "demonstrado com gatilho documentado".

## O discriminante

**Se o mecanismo estivesse errado** (causa real fosse outra — parsing de JSON, comparação de
`matcher`/`type`, resolução de `$HOME` vs `%USERPROFILE%`), eu esperaria:
1. O teste **também falhar em POSIX** — nada nessas outras hipóteses depende de SO. Não falha
   (medido acima: `PASS` em macOS).
2. Alimentar `normalizeGuardPath` com um `rawStoredCommand` **já em formato nativo Windows** (barras
   invertidas em toda a extensão, como um hand-edited config real escrito num editor Windows)
   **ainda falharia** sob a hipótese alternativa, mas sob a MINHA hipótese deveria **passar** — porque
   aí as duas strings já têm o mesmo estilo de separador. Testado (ver Direção B abaixo): passa.

Essas duas previsões são o que torna o mecanismo falsificável, e as duas batem com o observado.

## Falsificação proposta (não implementada)

**Direção A — mutação que reproduz a falha, já demonstrada acima sem tocar o repositório:** alimentar
a cópia literal do algoritmo com os valores exatos que um runner Windows produziria (`home` nativo +
sufixo `/` vs. `home` nativo + sufixo `\` via `Join`) e mostrar que os dois valores normalizados
divergem. Script usado: `probe_normalize.go` (scratchpad, fora do repositório).

**Direção B — candidato de remédio faz a árvore (simulada) passar, sem afrouxar o caso POSIX:**
```go
func normalizeGuardPathCandidate(p string) string {
    p = strings.ReplaceAll(p, `\`, "/")
    return normalizeGuardPathCurrent(p)
}
```
Resultado medido (`probe_normalize2.go`):
```
Direção A (estado atual)     : EQUAL? = false   <- reproduz a falha
Direção B (candidato)        : EQUAL? = true    <- fecharia o teste
Controle POSIX, atual        : EQUAL? = true    <- já passava
Controle POSIX, candidato    : EQUAL? = true    <- continua passando, não afrouxa nada
```
🔴 **Ressalva que o candidato acima NÃO resolve sozinho, achada ao testar o caso adversarial:** um
caminho UNC (`\\server\share\...`) sob a tradução ingênua `\`→`/` vira `//server/share/...`, que o
colapso de barra dupla já existente reduz para `/server/share/...` — **perdendo a marca de UNC**
(duas barras iniciais viram uma). Isso não quebra o teste-alvo (que não usa UNC), mas qualquer
implementação real do candidato precisa tratar o prefixo UNC/`\\?\` como caso especial antes de
converter separadores — o mesmo cuidado que o ML-3B já teve que aplicar num predicado vizinho
(`docs/roadmaps/wip/...` Wave 3, braço UNC). Não avaliei se isso afeta algum caso real do dedup porque
nenhum sítio hoje grava caminho UNC nesse fluxo — registro como risco de implementação, não como parte
do mecanismo do defeito.

**O que só o CI/Windows real fecha:** instrumentar temporariamente
`globalGitBranchGuardInstalledClaude` com `t.Logf` para dois valores normalizados dentro do teste que
falha, num runner Windows real, e confirmar que batem byte a byte com a previsão acima. Não pude
executar isso — não tenho runner Windows disponível nesta sessão.

## Quantos runtimes, com a medição

**Os 3 — Go, Node, Python — pelo mesmo mecanismo, confirmado por execução real (não só leitura):**
- Go: mecanismo confirmado por leitura do código-fonte instalado do `path/filepath` (autoritativo,
  não amostragem) + reprodução da string exata com a cópia byte-a-byte de `normalizeGuardPath`.
- Node: `path.win32.join` **executado de verdade** nesta máquina (módulo cross-platform, não precisa
  de Windows) produzindo separador nativo `\`; `normalizeGuardPath` (cópia byte-a-byte de
  `npm/src/generators/hooks.js:1295`) aplicada ao resultado real. 🔴 Em runner Windows real,
  `path === path.win32` — a produção (`npm/src/generators/hooks.js:1441`, `path.join(home, ...)`)
  chama exatamente o módulo que o probe exercitou, não um proxy.
- Python: `ntpath.join` **executado de verdade** nesta máquina (módulo cross-platform); `_normalize_guard_path`
  (cópia byte-a-byte de `pypi/trackfw/generators/hooks.py:113`) aplicada ao resultado real. Em Windows,
  `os.path` **é** `ntpath` — mesmo argumento de não-proxy do Node.
- As três fixtures (`git_branch_guard_dedup_test.go:222`, `git_branch_guard_dedup.test.js:185`,
  `test_git_branch_guard_dedup.py:219`) constroem `rawStoredCommand`/`raw_stored_command` com a
  **mesma linha, byte a byte**: `home + '//' + '.trackfw/scripts/trackfw-git-branch-guard.sh'`. Não é
  hipótese por nome — é o mesmo texto de fixture nos 3 arquivos.
- 🔴 **Resolução de `$HOME` verificada nos 3, não só em Go** (o handoff pediu exatamente isso e a
  re-triagem G12 tinha deixado em aberto). `npm/src/homedir.js` e `pypi/trackfw/homedir.py`
  espelham `internal/homedir/homedir.go`: os 3 `home()`/`homedir()`/`home_dir()` chamados por
  `globalGitBranchGuardScriptPath` (`hooks.js:1441`) e sua contraparte Python (`hooks.py:317`)
  preferem `$HOME` no Windows, lido diretamente dos três arquivos-fonte — não é o mesmo defeito que
  o item 2/6 da issue #216 (já corrigido, `homedir.Dir()` prefere `$HOME`); os 3 caem no mesmo
  `$HOME` isolado que o fixture define via `t.Setenv`/`process.env.HOME`/equivalente.

**Discriminante de contagem que fecha isto, sem depender de eu confiar na leitura de código:** só a
variante `//` (concatenação crua) falha; as variantes-irmãs de dedup (`...SkipsProjectEntryWhenGlobalInstalled`,
Codex/Gemini/Cursor/Copilot — mesmo fixture, mesmo `$HOME` isolado, só sem a concatenação crua)
**passam nos 3 runtimes**. Se a causa fosse resolução de home quebrada ou algo estrutural em
JSON/matcher, todas as variantes-irmãs falhariam também — G12 teria ~15 falhas, não 3. Ver seção
"Discriminante adicional" acima.

Nenhuma diferença de mecanismo entre runtimes foi encontrada; os três caem no mesmo ponto (função de
normalização cega a `\`) pela mesma razão estrutural (o `Join`/`join` de cada linguagem sempre emite
separador nativo no Windows).

## Produto ou teste? — a pergunta mais importante

**PRODUTO — contrato documentado não cumprido no Windows, com gatilho demonstrado (não só
especulado), e nenhum sítio do trackfw produz esse gatilho hoje.**

`normalizeGuardPath` **promete no próprio comentário de documentação** tolerar "incidental formatting
(e.g. $HOME resolving with a trailing slash... or a hand-edited config file)". Medido, não os dois
cenários que ele cita falham no Windows:
- **`$HOME` com barra final** (o cenário que a doc-comment nomeia primeiro): produz `\\` duplicado na
  costura — a função só testa `r == '/'`, nunca colapsa. **Demonstrado acima** (seção "O contrato
  documentado falha também no outro eixo"), não hipotético — é o exato cenário que o comentário cita.
- **Hand-edited config com `/`** (o segundo cenário citado): plausível — JSON com `\` exige escapar
  cada barra (`\\`), então humanos e ferramentas de terceiros tendem a escrever caminhos Windows com
  `/` em strings JSON para evitar esse escaping — mas eu não tenho uma amostra real de um config
  assim; é o cenário que o teste do reporter simula, não um que eu tenha observado em uso real.

Se qualquer um dos dois ocorrer com o `command` do git-branch-guard global instalado por qualquer uma
das 6 CLIs suportadas, o dedup deixa de reconhecer a entrada como já instalada e **reinjeta a entrada
de projeto** — duplicando a execução do guard por chamada de `Bash`. É exatamente o cenário que o
comentário no topo de `git_branch_guard_dedup_test.go` (linhas 9-16) descreve como o motivo de o
dedup existir: "fires the guard twice per Bash call" é o efeito colateral ruim, não uma falha de
segurança (o guard ainda dispara, só que duplicado) — mas é um defeito de produto real contra o
contrato documentado, com pelo menos um dos dois gatilhos (`$HOME` com barra final) inteiramente
plausível em qualquer instalação Windows onde `%TEMP%`/perfil resolva com separador final — não
depende de edição manual.

**Verifiquei, e é relevante para a severidade:** hoje **nenhum sítio de produção** (nos 3 runtimes)
grava o comando global por concatenação crua de string — todos os sítios que escrevem
`~/.claude/settings.json`/`~/.codex/hooks.json`/etc. usam `filepath.Join`/`path.join`/`os.path.join`
(grep confirmado, zero ocorrência de `home + "/"` ou equivalente fora de arquivos `_test`). Ou seja:
**o próprio `trackfw update harness` nunca produz esse formato hoje.** O gatilho realista é
externo ao trackfw — hand-edited config, versão antiga do trackfw (antes de alguma normalização
anterior), ou outra ferramenta que escreve o mesmo arquivo de hooks. Isso não muda o veredito
(o dedup ainda falha silenciosamente contra seu próprio contrato quando o gatilho ocorre), mas muda
a frequência esperada: não é "toda instalação Windows duplica o hook", é "uma entrada global editada
à mão ou legada com barras `/` nunca é reconhecida como instalada, no Windows, pelos 3 CLIs".

## Premissas do handoff que esta medição derrubou ou confirmou

- **"O defeito de `//` que o nome descreve é real"** (citação do reporter no handoff) — **parcialmente
  confirmado, mas a causa não é a barra dupla.** O colapso `//`→`/` funciona corretamente nos 3
  runtimes, medido. O que falha é a ausência de canonização `\`↔`/`, um mecanismo adjacente que o
  nome do teste não nomeia. O sintoma (`//` na fixture) é a forma escolhida para expor o defeito, não
  a causa dele — se a fixture usasse uma única barra em vez de duas, o teste falharia da mesma forma
  no Windows.
- **"Espaço reduzido, mecanismo desconhecido" (ML-0A/G12, retriagem 2026-09-04)** — meu resultado
  substitui essa premissa: o mecanismo agora está identificado e medido (não apenas espaço reduzido).
- Nada no handoff afirmou uma causa incorreta que eu tenha precisado derrubar — ele explicitamente se
  absteve de apresentar hipótese, o que era o resultado certo naquele ponto da investigação.

## O que falta para fechar

Confirmação em Windows real (CI) dos valores normalizados exatos via instrumentação temporária
(`t.Logf`/`console.log`/`print` dentro do teste que falha, revertida depois) — o item que esta sessão
não pôde medir por falta de runner Windows. Se confirmado, o remédio mínimo é canonizar separador
antes do colapso de barra redundante nos 3 `normalizeGuardPath`/`_normalize_guard_path`, com o cuidado
de UNC/`\\?\` registrado acima como risco de implementação a testar explicitamente (corpus adversarial,
não só o caso feliz).
