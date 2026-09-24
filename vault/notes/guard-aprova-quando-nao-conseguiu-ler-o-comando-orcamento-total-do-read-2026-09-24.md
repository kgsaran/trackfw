# O guard aprova quando não conseguiu ler o comando — e a culpa é do orçamento **total** do `read -t`

> Data: 2026-09-24 · Autor: `ares-tf` · ML-1B (G5) da
> `ROADMAP-2026-09-24-treze-rotulos-falham-no-censo-de-windows-e-cinco-sao-setup-que-aborta-o-cenario-inteiro.md`
> Sítio: `internal/generators/scaffold.go` (const `gitBranchGuardScript`, bloco `--- 0.`)
> Rótulo do censo que este achado explica:
> `falsify/git-branch-guard/stdin-drain-before-noop/baseline-writer-clean-large-payload`

## O achado, em uma frase

O `trackfw-git-branch-guard.sh` drenava o stdin com `IFS= read -r -t 2 -d ''` — um orçamento
**total** — e, quando o payload não cabia nesses 2 s, **saía 0**: aprovava um comando que **não
conseguiu ler**, sem emitir sinal nenhum.

🔴 É a pior das quatro categorias da triagem. Os outros grupos do cluster de Windows **reprovam
alto** — alguém lê vermelho. Este **aprova baixo**: o único observável é o `EPIPE`/`SIGPIPE` **no
escritor**, que em produção é o cliente de agente e pode engoli-lo em silêncio.

## Por que 200 KB não cabem em 2 s (e cabem no macOS)

O `read` do bash consome fd **não-seekable** 1 byte por `read(2)`. A syscall do MSYS é ~90x mais
cara que a do Darwin. Medido em 2026-09-24, mesmo payload de 200000 bytes, `bash 5.3`:

| | macOS (bash 5.3.20) | Git-Bash Windows (bash 5.3.15) |
|---|---|---|
| `read -t 600 -d ''` de **pipe** | 0,151 s, `rc=1`, `len=200000` | **13,6 s**, `rc=1`, `len=200000` |
| `read -t 2 -d ''` de **pipe** | 0,15 s, `rc=1`, `len=200000` | 🔴 **2,1 s, `rc=142`, `len=25397`** |
| `read -t 600 -d ''` de **arquivo** (fd seekable) | — | 0,618 s |

~1,3 MB/s contra **~14,7 KB/s**. É por isso que **este rótulo** é exclusivo do Windows: com payload
grande e escritor rápido, no macOS/Linux o mesmo código passa por folga. ⚠️ Isso **não** quer dizer
que o defeito seja exclusivo do Windows — ver a seção "O defeito não é exclusivo do Windows quando o
escritor é lento" abaixo.

🔴 **A semântica de `read -d ''` está CERTA** — ela lê até o EOF real e entrega os 200000 bytes
quando há tempo. A causa é **só** o orçamento. A triagem (`ML-0A`) declarou essa dúvida em aberto;
esta é a medição que a fecha.

## A armadilha: aumentar o `-t` é maquiagem

Qualquer valor de `-t` **total** passa em 200 KB e falha em 400 KB — o limite fica atado ao
tamanho do payload, que o guard não controla. A correção é mudar a **natureza** do orçamento:

- **total** → "quanto tempo a transferência inteira pode levar" (dependente de tamanho);
- **ocioso** → "quanto tempo o escritor pode ficar sem enviar **nada**" (independente de tamanho).

O laço renova os 2 s a cada byte que chega. 2 s de **ociosidade** é limite de **contrato**: é o que
separa *escritor lento* de *escritor que não vai escrever* — e foi o segundo caso (um chamador que
segura o descritor aberto sem escrever) que pendurou `make quality` por **1h05** no ML-2A da
ROADMAP-2026-09-09. Medido: o laço lê os 200000 bytes inteiros em 13,4 s no Git-Bash.

**Por que não despejar o stdin em arquivo temporário**, já que fd seekable é 22x mais rápido: o
despejo exige `cat` (ou equivalente) **sem limite** — é exatamente o travamento indefinido que o
ML-3A removeu.

## 🔴 Segundo achado, e é o que custa tempo amanhã: `read -t` no **bash 3.2** é outro animal

O comentário que estava no próprio código afirmava, como medição, que o `-t` *"preserva na variável
qualquer prefixo já lido antes do timeout"* em **bash 3.2 e 5.3**. **Falso para o 3.2** — que é o
bash padrão do macOS, e portanto o que roda o guard na maioria das máquinas de desenvolvimento.

Medido (mesmo cenário: escritor emite `abc` e segura o fd):

| bash | `rc` no timeout | conteúdo da variável | `rc` no EOF |
|---|---|---|---|
| 5.3 (macOS **e** Windows) | **142** (`128+SIGALRM`) | **preservado** (`abc`) | 1 |
| **3.2** (padrão do macOS) | **1** | 🔴 **vazio — prefixo descartado** | 1 |

Consequências práticas:

1. **Não existe discriminante de truncamento no bash 3.2** — `rc=1` significa *EOF* e *timeout* ao
   mesmo tempo. Qualquer lógica de fail-closed baseada em `rc > 128` é **condicional à plataforma**,
   e isso tem de ser **declarado**, não presumido.
2. `read -t` com `-d ''` no bash 3.2 **perde dados** no timeout. Um laço de acumulação escrito sem
   saber disso corromperia o payload em vez de completá-lo.
3. Na prática o risco é contido: sob bash 3.2 a vazão medida é 200 KB em 0,15 s, logo um timeout ali
   só pode significar *escritor parado*, nunca *payload grande*.

## O contrato escolhido: fail-closed, com o custo escrito

Quando o dreno termina **truncado** e **não há comando em argv**, o guard passa a recusar —
JSON de `deny` no stdout + razão no stderr + `exit 2`.

**Custo, nomeado:** uma invocação legítima, **dentro** de projeto trackfw, por um runtime que só
passa o comando por stdin e que fica 2 s inteiros **sem enviar um único byte**, passa a ser
**bloqueada**. O usuário vê a razão e refaz. Antes, era aprovada sem o guard ter lido o que aprovava.

Dois limites deliberados desse custo:

- a recusa vem **depois** do probe de no-op — fora de projeto trackfw o guard continua saindo 0
  (`ADR-2026-08-17-guard-global-cabeado-com-no-op-fora-de-projeto-trackfw.md`). Ali ele nunca
  bloqueia nada, então não há controle a perder;
- **argv isenta** (`$# > 0`): o comando está completo e o stdin é irrelevante para a decisão.
  ⚠️ **`$TRACKFW_GIT_COMMAND` NÃO isenta**, de propósito: o script só recorre a ele quando o stdin
  rende vazio, e um payload **truncado** pode render um comando **não-vazio e errado** (um prefixo
  que não casa com `git push`) — pior que o vazio, porque desliga a isenção sem completar a
  informação.

## A/B medido, nas duas direções (Git-Bash 5.3.15 do Windows, mesma máquina, mesma fixture)

| cenário | script **antigo** | script **novo** |
|---|---|---|
| 200 KB, **fora** de projeto | `guard_rc=0`, 🔴 **`writer_rc=141`** (`128+SIGPIPE`) | `guard_rc=0`, **`writer_rc=0`** |
| 200 KB, **dentro** de projeto | `guard_rc=2`, 🔴 `writer_rc=141` | `guard_rc=2`, `writer_rc=0` |
| escritor **travado**, dentro | 🔴 **`guard_rc=0`, silencioso** | **`guard_rc=2`** + `deny` + razão |
| escritor travado, **fora** | `guard_rc=0` | `guard_rc=0` (ADR preservada) |
| `git push` normal, dentro | `guard_rc=2` | `guard_rc=2` |
| truncado **com argv** | `guard_rc=0` | `guard_rc=0` |

⚠️ **Reconciliação obrigatória:** depois da correção, 200 KB **não é mais** "payload que o guard não
consegue ler" — é só um payload lento (13,4 s). A segunda direção da falsificação teve de ser
reconstruída como **escritor travado**, que é o caso residual genuinamente ilegível. Comparar o
teste de direção 2 com o rótulo do censo sem ler isto leva a concluir, errado, que houve
substituição de cenário.

## Dois efeitos do orçamento ocioso, medidos e aceitos

- **Latência:** com um escritor que manda alguns bytes e depois trava, o guard paga **uma janela
  ociosa a mais** — medido **4,07 s** contra **2,07 s** do orçamento total antigo (macOS, 1 byte +
  fd preso, com e sem argv). É o preço de não cortar quem ainda estava entregando.
- **Terminação:** um escritor que mande 1 byte a cada menos de 2 s **indefinidamente** mantém o
  guard vivo indefinidamente. Aceito de propósito: esse escritor **está** entregando o comando. A
  patologia que o ML-3A fechou — chamador que segura o fd e **não escreve nada** — continua cortada
  em 2 s, porque lá nenhuma janela tem progresso. 🔴 Um teto absoluto adicional seria um número sem
  razão de contrato — o mesmo chute que o `-t` maior — então não existe.

## O defeito **não** é exclusivo do Windows quando o escritor é lento

O Windows expõe a dependência de **tamanho**; a dependência de **tempo** aparece em qualquer
plataforma. Medido no **macOS**, payload de 450 bytes entregue em 5 pedaços com 1 s de intervalo e
com `"command":"git push"` **no fim** do JSON:

| | script antigo | script novo |
|---|---|---|
| `git push` por stdin, escritor lento | 🔴 **`rc=0`** — aprova, porque o pedaço lido parou antes de `"command"` | **`rc=2`** |

É a prova de não-vacuidade do teste `SlowTricklingWriter`: ele reprova no código antigo.

## Regra de bolso para quem esbarrar nisto

- 🔴 **Timeout em leitura de stdin que precisa terminar é sempre orçamento OCIOSO, nunca TOTAL.**
  Orçamento total é dependente de tamanho, e tamanho é do chamador.
- 🔴 **Guard que não conseguiu ler o comando não pode sair 0.** Se a política for permitir, que seja
  explícita e com sinal; silêncio aqui é indistinguível de "nada a bloquear".
- **No Windows/MSYS, `read` de pipe custa ~90x o de Darwin.** Qualquer orçamento de tempo calibrado
  em macOS/Linux está calibrado errado para o MSYS.
- **`read -t` só tem `rc>128` e preservação de prefixo a partir do bash 4.** Em bash 3.2 os dois
  somem, sem aviso.

## Sítios acoplados (byte-identidade) — o dreno vive em 4 lugares

Mudar o dreno em `internal/generators/scaffold.go` **obriga** a mudar, na mesma entrega:

1. `internal/validator/validator_git_branch_guard_reference.go` — cópia de referência do
   `validate` (`TestGitBranchGuardScriptReference_MatchesGenerator` prova byte-identidade);
2. `scripts/trackfw-git-branch-guard.sh` — a cópia **instalada neste próprio repositório**;
3. `scripts/check-gates-falsify.sh` — o `corrupt_literal` do **Cenário 65** cita o literal antigo
   **inteiro** e aborta com `expected exactly 1 occurrence, got 0`.

Relacionadas: [[msys-converte-env-e-argv-mas-nunca-conteudo-de-arquivo-2026-09-24]],
[[overlay-json-com-caminho-posix-e-ignorado-em-silencio-pelo-go-no-windows-2026-09-24]].
