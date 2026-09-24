# O MSYS converte **env var e `argv`**, mas **nunca conteúdo de arquivo** — e o G4 nasce nessa assimetria

> 2026-09-24 · ML-1A (Wave 1, G4) da REQ-2026-09-24 do cluster de Windows ·
> medido na VM Windows 11 ARM64, Git Bash (MSYS x86_64 emulado), `go1.27.0 windows/arm64`,
> com o **binário real** compilado de `977e3755`

## Por que a nota existe

A triagem do cluster (`docs/seguranca/2026-09-25-triagem-cluster-windows.md`, grupo **G4**)
concluiu, **por construção e sem executar**, que os dois rótulos
`git-branch-guard-dedup/baseline-skips-project-entry` e
`git-branch-guard-dedup/double-slash-tolerance` falham porque
`normalizeGuardPath` (`internal/generators/agentfiles.go`) só converte `\` → `/` quando
`hasWindowsDriveLetterPrefix` é verdadeiro — e um `HOME` de grafia MSYS *"não tem letra de
unidade"*, então `filepath.Join` produziria `\tmp\…` e a comparação divergiria por **separador**.

🔴 **A premissa é falsa.** O `HOME` que chega ao processo Go **tem** letra de unidade, porque o
MSYS converte a variável de ambiente ao lançar um processo **nativo**. A conversão de separador já
acontece hoje. Quem for corrigir `normalizeGuardPath` amanhã vai mexer em código de produto
aprovado por barreira, **fechar zero rótulos**, e não entender por quê.

## A assimetria, que é o achado transferível

| o que atravessa a fronteira MSYS→nativo | é convertido? |
|---|---|
| variável de ambiente (`HOME=…`) | ✅ sim — `/tmp/x` vira `C:\Users\…\AppData\Local\Temp\x` |
| `argv` | ✅ sim (medido também no censo: o splitter recebe `C:/Users/RUNNER~1/…`) |
| 🔴 **conteúdo de arquivo escrito pelo bash** (heredoc, `printf`, JSON) | ❌ **não** — fica `/tmp/x` |

O fixture grava o `command` do `settings.json` global com `cat <<EOF` a partir de `$T67_FAKE_HOME`
(`scripts/check-gates-falsify.sh:4722-4726`; braço 4 em `:4836`) — **conteúdo**, logo grafia MSYS.
O binário recebe `HOME` **convertido** e calcula o caminho com `filepath.Join` — grafia nativa.
As duas strings apontam para **o mesmo arquivo**, em **espaços de nomes diferentes**.

É a mesma família de
[[overlay-json-com-caminho-posix-e-ignorado-em-silencio-pelo-go-no-windows-2026-09-24]],
com a direção invertida: lá o caminho POSIX ia **para dentro** de um arquivo de configuração lido
por programa nativo; aqui o caminho POSIX **está** no arquivo e é comparado contra um caminho que o
MSYS já converteu. Regra de bolso que serve às duas: **caminho que sai do bash do MSYS por arquivo
precisa de `cygpath -m`; por env/argv, o MSYS já converteu.**

## A medição — sondas, e depois A/B com o binário real

Sonda 2 (reimplementação fiel de `normalizeGuardPath`, `HOME` POSIX no bash):

```
BASH_FH   = /tmp/trackfw-probe2.gmIUJB/s67-fake-home-installed
GO_HOME   = "C:\\Users\\Lab\\AppData\\Local\\Temp\\trackfw-probe2.gmIUJB\\s67-fake-home-installed"
LINK_A readGlobalHookJSON → err=<nil>        ← o elo (a) NÃO dispara: a leitura FUNCIONA
JSON_CMD  = "/tmp/trackfw-probe2.gmIUJB/s67-fake-home-installed/.trackfw/scripts/…"
COMPUTED  = "C:\\Users\\Lab\\…\\trackfw-probe2.gmIUJB\\s67-fake-home-installed\\.trackfw\\scripts\\…"
NORM_JSON = "/tmp/trackfw-probe2.gmIUJB/…"
NORM_COMP = "C:/Users/Lab/…"                 ← a conversão \→/ JÁ ACONTECEU
MATCH     = false
```

🔴 **`NORM_COMP` prova a refutação:** `hasWindowsDriveLetterPrefix` é **verdadeiro**, o braço de
letra de unidade rodou, e a divergência sobrevive. Não é gramática de separador.

A/B com o **binário real**, única variável = a grafia gravada no JSON global:

```
ARM A  command = /tmp/…/fake-home/.trackfw/scripts/trackfw-git-branch-guard.sh
       → ENTRADA DE PROJETO GRAVADA   (dedup NÃO disparou)  ← reproduz o FAIL do censo
ARM B  command = C:/Users/Lab/AppData/Local/Temp/…/fake-home/.trackfw/scripts/…
       → ENTRADA DE PROJETO AUSENTE   (dedup disparou)      ← o produto funciona
```

## Consequências

1. **O produto está certo.** A dedup de `git-branch-guard` funciona no Windows quando as duas
   pontas falam o mesmo espaço de nomes. O defeito é do **fixture**, não de `internal/`.
   A reclassificação do G4 é de **D (produto)** para **harness** — e o corretivo vive em
   `scripts/check-gates-falsify.sh`, não em `agentfiles.go`.
2. **O elo (a) está respondido** (item em aberto do parecer): `readGlobalHookJSON` **sucede**
   (`err=<nil>`), porque ele resolve o caminho pelo mesmo `homedir.Dir()` + `filepath.Join` — logo
   lê exatamente onde o MSYS escreveu. Quem decide é o elo (b), a comparação.
3. 🔴 **Nenhuma regra de string pode casar as duas grafias**, porque a diferença é de *montagem*,
   não de sintaxe. Toda "correção" que as faça casar é necessariamente um **afrouxamento
   semântico** — o que reforça, em vez de enfraquecer, o alerta da seção 6.4 do parecer contra
   comparar por *basename*.
4. **A garantia continua sem prova no Windows** enquanto o fixture não for corrigido: com
   `MATCH=false`, *"global instalado ⇒ entrada de projeto pulada"* nunca é exercitada lá.

## Por que a cobertura Go não pegou (a presunção do ML-0A, refutada)

A presunção era *"não há cobertura equivalente"*. **Há**:
`internal/generators/guard_path_normalize_test.go` tem 6 referências às funções e uma tabela de 22
casos. O motivo de o G4 ter ficado invisível **não é ausência de teste** — é um teste que **fixa a
forma como comportamento intencional**:

```go
{"relative path with backslash, no drive letter, untouched", `scripts\guard.sh`, `scripts\guard.sh`},
```

Teste que afirma o defeito como contrato é mais caro que teste ausente: ele **defende** o defeito na
revisão. (E, neste caso, o "defeito" que ele fixa nem é o mecanismo do G4.)

## O que NÃO foi medido

- Nada foi exercitado no **Windows CI** por mim — a VM é ARM64, o censo é x64. O que transfere do
  censo: `$WORK` sai `/tmp/trackfw-falsify.Q5DXgN` (grafia MSYS, linhas 5149/5237 do run
  `36036473391`) e `/tmp` mapeia para `C:\Users\RUNNER~1\AppData\Local\Temp` lá também.
- O `cat` do braço 1 despeja o `settings.json` **de projeto**, cujo comando é
  `$CLAUDE_PROJECT_DIR/…` — **não serve** de testemunha da grafia do `HOME`. O JSON global nunca é
  despejado no log.
