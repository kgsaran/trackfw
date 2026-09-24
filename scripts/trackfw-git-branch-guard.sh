#!/usr/bin/env bash
# trackfw git branch guard — bloqueia git commit/push/checkout -b/branch/worktree add -b
# brutos por subagente
#
# TRIPWIRE, NÃO FRONTEIRA DE SEGURANÇA: detecta o caso óbvio — comando git literal, sem
# indireção de shell — não é defesa contra um agente adversário competente. Evasões que
# exigem tokenizar como o bash (ex.: git${IFS}push, {git,push}, g""it push) permanecem
# abertas por decisão: ver docs/adr/ADR-2026-08-12-nao-ha-prevencao-contra-agente-induzido-
# com-escrita-irrestrita-a-resposta-e-deteccao-ancorada-no-git.md. O stripping de
# env/command abaixo reconhece as formas SEM argumentos antes de git (env git ...,
# command git ...) e o env seguido de uma sequência de atribuições CHAVE=valor
# (env FOO=bar git ..., env FOO=bar BAZ=qux git ...) — env com FLAGS (env -i git ...,
# env --ignore-environment git ...) e command com flags (command -p git ...) continuam
# evadindo; declarado, não fechado (ver AC5 do ML que adicionou esse stripping). A
# segmentação abaixo
# (quote_aware_split) evita falso-positivo em texto citado — não deve ser lida como imune a
# evasão por citação/tokenização do shell.
set -euo pipefail
set -f

# --- 0. Drena o stdin ANTES de qualquer saída antecipada (ML-1B, ROADMAP-2026-08-17-guard-
# global-cabeado-com-no-op-fora-de-projeto-e-integridade-independente-de-fiacao.md): sem isso,
# quem escreve o payload JSON no pipe recebe EPIPE quando o no-op abaixo sai com 0 antes de ler
# — reprodutível em 100% das chamadas fora de projeto trackfw, não é corrida de timing. O dreno
# roda incondicionalmente, com orçamento de tempo (ML-3A, ver abaixo). O valor lido é reaproveitado no passo 1 abaixo — nunca há uma segunda leitura.
#
# ML-3A (ROADMAP-2026-09-09-guard-emite-hookspecificoutput-e-a-razao-chega-ao-modelo-nos-3-
# clis.md): o discriminante que existia aqui era "-t 0" -- separa "terminal interativo" de
# "pipe", não "vai receber EOF" de "não vai". Qualquer chamador não-tty que segure stdin aberta
# (ex.: um runtime de hook que nunca fecha o descritor) travava o antigo "cat" para sempre, sem
# limite -- foi o que pendurou "make quality" por 1h05 quando o gate de schema (ML-2A) chamou o
# guard sem "</dev/null" em 3 sítios. Substituído por dreno com ORÇAMENTO DE TEMPO via
# "read -t <segundos> -d ''": builtin do próprio bash desde a 3.0 (não do coreutils); o bash do
# MSYS2/Git-Bash do Windows é bash de verdade e traz o mesmo builtin. Evita depender de
# "timeout(1)", que NÃO existe no macOS base nem no MSYS2 mínimo. "-d ''" faz o read tentar ler
# até o EOF real (NUL nunca aparece em payload de hook).
#
# ML-1B (ROADMAP-2026-09-24-treze-rotulos-falham-no-censo-de-windows-e-cinco-sao-setup-que-aborta-
# o-cenario-inteiro.md, grupo G5 da triagem): o orçamento do ML-3A era TOTAL, e um orçamento total
# é dependente do TAMANHO do payload -- o que o torna arbitrário por construção. Medido no
# Git-Bash 5.3.15 do Windows (VM, 2026-09-24), lendo um payload de 200000 bytes de um PIPE:
#
#   read -t 600 -d ''  -> rc=1 (EOF), len=200000, 13.6 s     <- a semântica de -d '' está CERTA
#   read -t 2   -d ''  -> rc=142 (timeout), len=25397, 2.1 s <- o orçamento TOTAL é a causa
#   redirect de ARQUIVO (fd seekable), -t 600 -> 618 ms      <- o custo é do pipe, não do read
#
# São ~14,7 KB/s no pipe do MSYS contra ~1,3 MB/s no macOS (mesma medição, bash 5.3): o read do
# bash consome fd não-seekable 1 byte por read(2), e a syscall do MSYS é ~90x mais cara. Com o
# orçamento total, o guard lia 12,7% do payload, saía com 0 no no-op abaixo, e o escritor levava
# EPIPE -- o rótulo
# falsify/git-branch-guard/stdin-drain-before-noop/baseline-writer-clean-large-payload.
#
# 🔴 A correção NÃO é aumentar o "-t": qualquer valor passa em 200 KB e falha em 400 KB. O que
# muda é a NATUREZA do orçamento -- de TOTAL para OCIOSO. O laço abaixo renova os 2s toda vez que
# QUALQUER byte chega, então o limite deixa de ser "quanto tempo a transferência inteira pode
# levar" (dependente de tamanho) e passa a ser "quanto tempo o escritor pode ficar sem enviar
# nada" (independente de tamanho). 2s de ociosidade é limite de CONTRATO: é o que separa "escritor
# lento" de "escritor que não vai escrever", e foi o segundo caso -- um chamador que segura o
# descritor aberto sem escrever -- que pendurou "make quality" por 1h05 no ML-2A. Medido no mesmo
# Git-Bash: o laço lê os 200000 bytes inteiros (len=200000, truncado=0) em 13,4 s.
#
# Por que não despejar o stdin em arquivo temporário, já que fd seekable é 22x mais rápido: o
# despejo exige "cat" (ou equivalente) sem limite, e é exatamente o travamento indefinido que o
# ML-3A removeu.
#
# Dois efeitos do orçamento OCIOSO, medidos e declarados (não são regressão silenciosa):
#   (a) LATÊNCIA: com um escritor que manda alguns bytes e depois trava, o guard paga UMA janela
#       ociosa a mais que o orçamento total antigo -- medido 4,07 s contra 2,07 s (1 byte + fd
#       preso, macOS). É o preço de não cortar um escritor que ainda estava entregando.
#   (b) TERMINAÇÃO: um escritor que mande 1 byte a cada menos de 2 s indefinidamente mantém o
#       guard vivo indefinidamente. Aceito de propósito: esse escritor ESTÁ entregando o comando,
#       e cortá-lo é justamente o defeito que este ML corrige. A patologia que o ML-3A fechou --
#       chamador que segura o descritor e NÃO escreve nada (o "make quality" sem "</dev/null") --
#       continua cortada em 2 s, porque lá nenhuma janela tem progresso. Um teto absoluto adicional
#       seria um número sem razão de contrato (o mesmo chute que o "-t" maior), então não existe.
#
# Discriminante de truncamento: no bash >= 4 o read retorna rc > 128 (142 = 128+SIGALRM) quando
# estoura o "-t", e rc=1 no EOF -- medido nos dois bash 5.3 (macOS e Windows). ⚠️ No bash 3.2
# (padrão do macOS) NÃO existe esse discriminante: medido rc=1 no timeout, com a variável VAZIA
# (o prefixo já lido é DESCARTADO). O comentário anterior deste bloco afirmava o contrário
# ("preserva na variável qualquer prefixo já lido... bash 3.2 e 5.3") -- afirmação falsa,
# corrigida aqui por medição. Consequência declarada: sob bash 3.2 o laço roda uma iteração só e
# o truncamento fica INDETECTÁVEL (degrada exatamente para o comportamento de hoje). Não é
# remediável sem discriminante; sob bash 3.2 a vazão medida é 200 KB em 0,15 s, então um timeout
# ali só pode significar escritor parado, não payload grande.
_TRACKFW_STDIN=""
_TRACKFW_STDIN_TRUNCATED=0
while :; do
  _TRACKFW_STDIN_CHUNK=""
  _TRACKFW_STDIN_RC=0
  IFS= read -r -t 2 -d '' _TRACKFW_STDIN_CHUNK || _TRACKFW_STDIN_RC=$?
  _TRACKFW_STDIN="${_TRACKFW_STDIN}${_TRACKFW_STDIN_CHUNK}"
  if [ "$_TRACKFW_STDIN_RC" -le 128 ]; then
    break
  fi
  if [ -z "$_TRACKFW_STDIN_CHUNK" ]; then
    _TRACKFW_STDIN_TRUNCATED=1
    break
  fi
done

# --- 0b. No-op fora de projeto trackfw (ADR-2026-08-17-guard-global-cabeado-com-no-op-fora-de-
# projeto-trackfw.md): sobe diretórios a partir do cwd FÍSICO (pwd -P, resolve symlink) até
# achar trackfw.yaml na raiz do projeto. Sem trackfw.yaml em nenhum ancestral, o guard não se
# aplica — fora de projeto trackfw não há trackfw ship como alternativa, e bloquear ali é custo
# sem contrapartida. Custo medido: só parameter expansion e test -f por nível, nenhum fork de
# processo; limitado pela profundidade do caminho.
_TRACKFW_ROOT_DIR=$(pwd -P)
_TRACKFW_FOUND=0
while :; do
  if [ -f "$_TRACKFW_ROOT_DIR/trackfw.yaml" ]; then
    _TRACKFW_FOUND=1
    break
  fi
  if [ "$_TRACKFW_ROOT_DIR" = "/" ]; then
    break
  fi
  _TRACKFW_ROOT_DIR="${_TRACKFW_ROOT_DIR%/*}"
  if [ -z "$_TRACKFW_ROOT_DIR" ]; then
    _TRACKFW_ROOT_DIR="/"
  fi
done
[ "$_TRACKFW_FOUND" -eq 1 ] || exit 0

# --- 0c. Contrato quando o stdin NÃO pôde ser lido por inteiro (ML-1B, ROADMAP-2026-09-24-treze-
# rotulos-..., grupo G5). Até aqui o guard era FAIL-OPEN nesse caso: com o payload truncado, o
# passo 1 extraía comando vazio, caía em "[ -n "$CMD_RAW" ] || exit 0" e APROVAVA -- em silêncio,
# sem nenhum sinal para quem chamou. É a pior das quatro categorias da triagem: os outros grupos
# reprovam alto, este aprova baixo.
#
# Decisão: FAIL-CLOSED. Se o dreno terminou truncado e NÃO há comando em argv, o guard recusa com
# a mesma resposta de bloqueio dos demais caminhos (JSON de deny no stdout + exit 2), nomeando o
# motivo. O que isso CUSTA, escrito: uma invocação legítima, DENTRO de projeto trackfw, feita por
# um runtime que só passa o comando por stdin e que fica 2s inteiros sem enviar um único byte,
# passa a ser BLOQUEADA -- o usuário vê a razão e refaz o comando. Antes, ela era aprovada sem o
# guard ter lido o que aprovava.
#
# Duas escolhas que limitam esse custo:
#   (a) a recusa vem DEPOIS do no-op de 0b -- fora de projeto trackfw o guard continua saindo 0,
#       como manda a ADR-2026-08-17; ali ele não tem autoridade para recusar nada, e não há perda
#       de controle porque ali ele também nunca bloqueia;
#   (b) argv isenta ($# > 0): o passo 1 prefere "$*" e o comando está completo -- o stdin é
#       irrelevante para a decisão. Já "$TRACKFW_GIT_COMMAND" NÃO isenta, de propósito: o passo 1
#       só recorre a ele quando o stdin rende vazio, e um payload truncado pode render um comando
#       NÃO-vazio e ERRADO (um prefixo que não casa com "git push") -- que é mais perigoso que o
#       vazio, porque desliga a isenção sem completar a informação.
if [ "$_TRACKFW_STDIN_TRUNCATED" -eq 1 ] && [ "$#" -eq 0 ]; then
  _TRACKFW_TRUNC_REASON='trackfw git-branch-guard: RECUSADO — o comando nao pode ser lido por inteiro do stdin (nenhum byte chegou em 2s de espera ociosa) e nao ha comando em argv. O guard recusa em vez de aprovar um comando que nao conseguiu ler. Reenvie o comando, ou use trackfw ship / trackfw branch new.'
  printf '{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"%s"}}\n' "$_TRACKFW_TRUNC_REASON"
  printf '%s\n' "$_TRACKFW_TRUNC_REASON" >&2
  exit 2
fi

# --- 1. Obter o comando git bruto ------------------------------------------------------------
if [ "$#" -gt 0 ]; then
  CMD_RAW="$*"
else
  INPUT="$_TRACKFW_STDIN"
  TRIMMED=$(printf '%s' "$INPUT" | sed -e 's/^[[:space:]]*//')
  case "$TRIMMED" in
    \{*)
      CMD_RAW=""
      if command -v jq >/dev/null 2>&1; then
        CMD_RAW=$(printf '%s' "$INPUT" | jq -r '.tool_input.command // .command // .tool_info.command_line // .hook_input.command // empty' 2>/dev/null || true)
      fi
      if [ -z "$CMD_RAW" ] || [ "$CMD_RAW" = "null" ]; then
        CMD_RAW=$(printf '%s' "$INPUT" | sed -n 's/.*"tool_input"[[:space:]]*:[[:space:]]*{[^}]*"command"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -1)
      fi
      if [ -z "$CMD_RAW" ]; then
        CMD_RAW=$(printf '%s' "$INPUT" | sed -n 's/.*"tool_info"[[:space:]]*:[[:space:]]*{[^}]*"command_line"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -1)
      fi
      if [ -z "$CMD_RAW" ]; then
        CMD_RAW=$(printf '%s' "$INPUT" | sed -n 's/.*"hook_input"[[:space:]]*:[[:space:]]*{[^}]*"command"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -1)
      fi
      if [ -z "$CMD_RAW" ]; then
        CMD_RAW=$(printf '%s' "$INPUT" | sed -n 's/.*"command"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -1)
      fi
      ;;
    *)
      CMD_RAW="$INPUT"
      ;;
  esac
fi

if [ -z "$CMD_RAW" ]; then
  CMD_RAW="${TRACKFW_GIT_COMMAND:-}"
fi

[ -n "$CMD_RAW" ] || exit 0

# --- 2. Pré-processamento anti-falso-positivo: neutraliza separadores reais (';', '&&',
# '||', '|', quebra de linha) que estão DENTRO de aspas ou de corpo de heredoc, para que
# conteúdo de mensagem (ex.: `-m "linha 1\nlinha 2"`) nunca seja fatiado em pseudo-segmentos
# e lido como comando -------------------------------------------------------------------
#
# strip_heredoc_bodies: remove o CORPO de blocos heredoc (<<DELIM ... DELIM), preservando a
# linha de abertura e a linha terminadora — cobre o padrão `git commit -F- <<'EOF' ... EOF`
# (heredoc não citado, fora do escopo de quote_aware_split abaixo). Heurística por linha, não
# sintaxe completa de shell: só remove o corpo quando encontra a linha terminadora
# correspondente. Se o heredoc nunca fecha (terminador ausente ou não localizado), devolve o
# texto ORIGINAL sem qualquer alteração — lado seguro: mais restritivo é preferível a esconder
# um comando real atrás de um heredoc mal-formado.
strip_heredoc_bodies() {
  printf '%s' "$1" | awk '
    BEGIN {
      dq = sprintf("%c", 34)
      sq = sprintf("%c", 39)
      in_heredoc = 0
      delim = ""
      ok = 1
    }
    {
      raw = raw $0 "\n"
      if (in_heredoc) {
        trimmed = $0
        sub(/^[ \t]+/, "", trimmed)
        sub(/[ \t]+$/, "", trimmed)
        if (trimmed == delim) {
          in_heredoc = 0
          out = out $0 "\n"
        }
        next
      }
      if (match($0, /<<-?[ \t]*[^ \t]+/)) {
        d = substr($0, RSTART, RLENGTH)
        sub(/^<<-?[ \t]*/, "", d)
        gsub(dq, "", d)
        gsub(sq, "", d)
        if (d != "") {
          delim = d
          in_heredoc = 1
        }
      }
      out = out $0 "\n"
    }
    END {
      if (in_heredoc) ok = 0
      if (ok) { printf "%s", out } else { printf "%s", raw }
    }
  '
}

# quote_aware_split: emite o texto com ';' isolado, '&&', '||' e '|' isolado convertidos em
# quebra de linha — EXCETO quando ocorrem dentro de uma string entre aspas simples ou duplas,
# caso em que são preservados como texto e uma quebra de linha real dentro das aspas vira
# espaço (nunca gera um novo pseudo-segmento). Substitui o antigo `sed` cego, que não
# distinguia texto citado de sintaxe de comando — a causa raiz do falso-positivo de linha de
# mensagem de commit iniciada por "git ...". Aspas não fechadas até o fim da entrada
# permanecem "abertas" até o fim — mesma semântica do shell real: uma aspa não fechada nunca
# deixa o texto seguinte executar como comando novo, só torna o restante parte da mesma
# string.
quote_aware_split() {
  printf '%s' "$1" | awk '
    BEGIN {
      dq = sprintf("%c", 34)
      sq = sprintf("%c", 39)
      bs = sprintf("%c", 92)
      nl = sprintf("%c", 10)
    }
    { s = (NR == 1) ? $0 : s nl $0 }
    END {
      n = length(s)
      q = ""
      out = ""
      i = 1
      while (i <= n) {
        c = substr(s, i, 1)
        if (q != "") {
          if (q == dq && c == bs && i < n) {
            nx = substr(s, i + 1, 1)
            out = out c (nx == nl ? " " : nx)
            i += 2
            continue
          }
          if (c == q) {
            q = ""
            out = out c
            i++
            continue
          }
          out = out (c == nl ? " " : c)
          i++
          continue
        }
        if (c == dq || c == sq) {
          q = c
          out = out c
          i++
          continue
        }
        if (substr(s, i, 2) == "&&" || substr(s, i, 2) == "||") {
          out = out nl
          i += 2
          continue
        }
        if (c == ";" || c == "|") {
          out = out nl
          i++
          continue
        }
        out = out c
        i++
      }
      printf "%s", out
    }
  '
}

# match_subcommand — casa contra "git (commit|push|checkout -b|switch -c)", segmento por
# segmento. Cada segmento é um comando real, obtido depois do pré-processamento acima
# (strip_heredoc_bodies + quote_aware_split), que converte ';', '&&', '||', '|' fora de aspas
# em quebra de linha e neutraliza os mesmos separadores quando aparecem dentro de
# aspas/heredoc. "git" só conta se for o PRIMEIRO token do segmento (por basename, então
# /usr/bin/git também casa) — nunca uma ocorrência solta em qualquer posição da string
# inteira. Isso evita: (a) o segundo comando de uma cadeia escapar da checagem, (b) um path
# absoluto para o git escapar por comparação de igualdade exata, e (c) texto de prosa —
# inclusive linha de mensagem de commit que COMEÇA com "git <sub>" (ex.: uma tabela
# documentando comandos bloqueados) — ser tratado como comando, porque esse texto agora nunca
# produz um novo segmento. `git switch -c/-C/--create` (forma alternativa a `checkout -b`
# para criar branch) é reconhecido varrendo TODOS os tokens após o subcomando, não só o
# primeiro — cobre `git switch --track -c feat/x` (flag antes de -c).
# checkout -b é reconhecido do mesmo jeito: varre TODOS os tokens até achar -b/-B/--orphan,
# não só o primeiro. Prefixos env e command antes de git são descartados antes da checagem do
# basename — cobre env git push/command git push sem exigir tokenizar como o bash.
match_subcommand() {
  normalized=$(strip_heredoc_bodies "$1")
  normalized=$(quote_aware_split "$normalized")
  while IFS= read -r seg; do
    seg_trimmed=$(printf '%s' "$seg" | sed -e 's/^[[:space:]]*//')
    [ -n "$seg_trimmed" ] || continue

    set -- $seg_trimmed
    first="$1"
    base="${first##*/}"
    while [ "$base" = "env" ] || [ "$base" = "command" ]; do
      is_env="$base"
      shift
      [ "$#" -gt 0 ] || break
      if [ "$is_env" = "env" ]; then
        while [ "$#" -gt 0 ]; do
          case "$1" in
            -*)
              break
              ;;
            *=*)
              shift
              ;;
            *)
              break
              ;;
          esac
        done
        [ "$#" -gt 0 ] || break
      fi
      first="$1"
      base="${first##*/}"
    done
    [ "$base" = "git" ] || continue
    shift

    sub=""
    while [ "$#" -gt 0 ]; do
      tok="$1"
      case "$tok" in
        -C|-c|--work-tree|--git-dir|--namespace)
          if [ "$#" -ge 2 ]; then shift 2; else shift; fi
          continue
          ;;
        -*)
          shift
          continue
          ;;
        *)
          sub="$tok"
          shift
          break
          ;;
      esac
    done

    case "$sub" in
      commit)
        echo "commit"
        return 0
        ;;
      push)
        echo "push"
        return 0
        ;;
      checkout)
        for tok2 in "$@"; do
          case "$tok2" in
            -b|-B|--orphan|--orphan=*)
              echo "checkout-b"
              return 0
              ;;
          esac
        done
        # git checkout -- <path> | git checkout . descarta alterações não commitadas do
        # caminho indicado, de forma irreversível, no worktree compartilhado — bloqueia
        # quando '--' aparece em qualquer posição (forma explícita de pathspec) ou quando
        # '.' aparece como token isolado. 'git checkout <branch>' sem nenhum dos dois
        # segue liberado por decisão (distinguir branch de caminho sem '--' é ambíguo, e
        # adivinhar produziria falso-positivo).
        checkout_path=0
        for tok2 in "$@"; do
          case "$tok2" in
            --|.)
              checkout_path=1
              ;;
          esac
        done
        if [ "$checkout_path" = "1" ]; then
          echo "checkout-path"
          return 0
        fi
        ;;
      switch)
        for tok2 in "$@"; do
          case "$tok2" in
            -c|-C|--create|--create=*|--force-create|--force-create=*)
              echo "switch-c"
              return 0
              ;;
          esac
        done
        ;;
      stash)
        # git stash: liberado só para leitura (list/show) — bloqueia a forma bare
        # (equivale a "push"), push, save, clear e drop. Decisão de KG: bloquear a
        # classe inteira, não só os literais medidos (ver REQ). Repositório com um único
        # worktree compartilhado entre subagentes paralelos — um stash de um agente
        # remove as alterações não commitadas de todos os outros.
        stash_sub="${1:-}"
        case "$stash_sub" in
          list|show)
            ;;
          *)
            echo "stash"
            return 0
            ;;
        esac
        ;;
      reset)
        # Só --hard bloqueia, em qualquer posição de token — --soft/--mixed (inclusive
        # sem flag, que é --mixed implícito) seguem liberados: --soft é o contorno
        # padrão para reempurrar trabalho staged via `trackfw ship -m "..."` (ainda falta commitar após --soft).
        for tok2 in "$@"; do
          case "$tok2" in
            --hard)
              echo "reset-hard"
              return 0
              ;;
          esac
        done
        ;;
      clean)
        # Bloqueia qualquer forma com force (-f, -fd, -fx, --force) ou -x/-X, EXCETO
        # quando -n/--dry-run também está presente (dry-run nunca apaga nada).
        clean_dry=0
        clean_force=0
        for tok2 in "$@"; do
          case "$tok2" in
            -n|--dry-run)
              clean_dry=1
              ;;
            -f*|--force|--force=*|-x|-X)
              clean_force=1
              ;;
          esac
        done
        if [ "$clean_dry" != "1" ] && [ "$clean_force" = "1" ]; then
          echo "clean-force"
          return 0
        fi
        ;;
      restore)
        # git restore --staged SOZINHO nunca toca o working tree (mexe só no
        # index), então segue liberado mesmo com path. Mas --worktree/-W (com ou
        # sem --staged junto) SEMPRE afeta o working tree — inclusive
        # "--staged --worktree", que restaura os dois — então bloqueia sempre que
        # --worktree/-W aparecer, e também no caso padrão (sem --staged em
        # nenhuma forma) com um argumento posicional (o path).
        restore_staged=0
        restore_worktree=0
        restore_positional=0
        for tok2 in "$@"; do
          case "$tok2" in
            --staged)
              restore_staged=1
              ;;
            --worktree|-W)
              restore_worktree=1
              ;;
            -*)
              ;;
            *)
              restore_positional=1
              ;;
          esac
        done
        if [ "$restore_positional" = "1" ]; then
          if [ "$restore_worktree" = "1" ] || [ "$restore_staged" != "1" ]; then
            echo "restore-path"
            return 0
          fi
        fi
        ;;
      branch)
        # git branch é majoritariamente leitura (sem args, -a, -r, -l, --list, -v/-vv,
        # --show-current, --contains, --no-contains, --merged, --no-merged, --sort=,
        # --format=, --points-at, -d/-D/--delete) — bloquear leitura seria pior que a
        # brecha. Só bloqueia: (a) -c/-C/-m/-M/--copy/--move (cria/renomeia branch,
        # qualquer posição de token) ou (b) um argumento posicional puro (nome da branch a
        # criar), a menos que -d/-D/--delete também esteja presente (delete tem
        # posicional legítimo — o nome a apagar). Flags de valor conhecidas (--contains,
        # --no-contains, --sort, --format, --points-at, --merged, --no-merged) têm seu
        # valor seguinte pulado quando vem em token separado, para não ser lido como
        # posicional de criação.
        branch_action=0
        has_delete=0
        saw_positional=0
        skip_next=0
        for tok2 in "$@"; do
          if [ "$skip_next" = "1" ]; then
            skip_next=0
            continue
          fi
          case "$tok2" in
            -c|-C|-m|-M|--copy|--copy=*|--move|--move=*)
              branch_action=1
              ;;
            -d|-D|--delete|--delete=*)
              has_delete=1
              ;;
            --contains|--no-contains|--sort|--format|--points-at|--merged|--no-merged)
              skip_next=1
              ;;
            -*)
              ;;
            *)
              saw_positional=1
              ;;
          esac
        done
        if [ "$has_delete" != "1" ]; then
          if [ "$branch_action" = "1" ] || [ "$saw_positional" = "1" ]; then
            echo "branch-create"
            return 0
          fi
        fi
        ;;
      worktree)
        if [ "${1:-}" = "add" ]; then
          shift
          for tok2 in "$@"; do
            case "$tok2" in
              -b|-B)
                echo "worktree-add-b"
                return 0
                ;;
            esac
          done
        elif [ "${1:-}" = "remove" ]; then
          # git worktree remove SEM -f/--force já recusa sozinho quando há alteração não
          # commitada no worktree indicado — só a forma com force é irreversível o bastante
          # para bloquear aqui.
          shift
          for tok2 in "$@"; do
            case "$tok2" in
              -f|--force)
                echo "worktree-remove-force"
                return 0
                ;;
            esac
          done
        fi
        ;;
      update-ref)
        # git update-ref reescreve um ref (inclusive refs/remotes/origin/*) sem tocar o
        # objeto apontado nem exigir push — foi o mecanismo que tornou alcançável o exploit
        # descrito no ADR-2026-08-19-caminho-governado-para-push-forcado-e-tag-de-release.md
        # (Emenda 1): forjar origin/<base> localmente para desviar o commit-alvo de trackfw
        # release tag. Sem forma de leitura equivalente a bloquear seletivamente — a
        # subcommand inteira é escrita — bloqueia sempre, sem exceção de token.
        echo "update-ref"
        return 0
        ;;
      rm)
        # git rm -f/--force apaga do working tree e do index de forma irreversível, mesma
        # classe de git clean -f/git reset --hard já bloqueados acima — sem exceção para
        # --cached (destrancar do index sem -f já segue liberado por não precisar de force).
        for tok2 in "$@"; do
          case "$tok2" in
            -f*|--force|--force=*)
              echo "rm-force"
              return 0
              ;;
          esac
        done
        ;;
    esac
  done <<EOF
$normalized
EOF
  return 1
}

SUBCOMMAND=$(match_subcommand "$CMD_RAW") || exit 0

case "$SUBCOMMAND" in
  checkout-b)
    REASON="trackfw: git checkout -b bruto bloqueado. Use \`trackfw branch new <type>/<slug>\`. Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."
    ;;
  switch-c)
    REASON="trackfw: git switch -c bruto bloqueado. Use \`trackfw branch new <type>/<slug>\`. Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."
    ;;
  branch-create)
    REASON="trackfw: git branch bruto bloqueado. Use \`trackfw branch new <type>/<slug>\`. Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."
    ;;
  worktree-add-b)
    REASON="trackfw: git worktree add -b bruto bloqueado. Use \`trackfw branch new <type>/<slug>\`. Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."
    ;;
  commit)
    REASON="trackfw: git commit bruto bloqueado. Use \`trackfw commit -m '<mensagem>'\`. Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."
    ;;
  push)
    REASON="trackfw: git push bruto bloqueado. Use \`trackfw push\` (para empurrar commits já criados), \`trackfw ship\` (para commit+push+PR em uma etapa) ou \`trackfw release tag\` (para publicar uma tag de release). Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."
    ;;
  stash)
    REASON="trackfw: git stash bruto bloqueado — worktree compartilhado entre subagentes, um stash remove as alterações não commitadas de todos os outros. \`git stash list\`/\`git stash show\` seguem liberados; para guardar trabalho em progresso, use uma branch própria via \`trackfw branch new\` e commit nela. Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."
    ;;
  reset-hard)
    REASON="trackfw: git reset --hard bruto bloqueado — descarta de forma irreversível as alterações não commitadas de todo o worktree compartilhado. \`git reset --soft\`/\`--mixed\` seguem liberados (ex.: \`git reset --soft HEAD~1\` é o caminho padrão; use \`trackfw ship -m "..."\` para commitar e empurrar). Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."
    ;;
  clean-force)
    REASON="trackfw: git clean -f/-x bruto bloqueado — apaga arquivos não rastreados do worktree compartilhado, de forma irreversível. \`git clean -n\`/\`--dry-run\` segue liberado para revisar antes o que seria apagado. Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."
    ;;
  restore-path)
    REASON="trackfw: git restore <path> bruto bloqueado — descarta de forma irreversível as alterações não commitadas do caminho indicado. \`git restore --staged\` (não toca o working tree) segue liberado; para descartar de fato, confirme antes com o usuário. Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."
    ;;
  checkout-path)
    REASON="trackfw: git checkout -- <path>/git checkout . bruto bloqueado — descarta de forma irreversível as alterações não commitadas do caminho indicado. \`git checkout <branch>\`/\`git switch <branch>\` seguem liberados; para descartar de fato, confirme antes com o usuário. Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."
    ;;
  update-ref)
    REASON="trackfw: git update-ref bruto bloqueado — reescreve um ref (inclusive refs/remotes/origin/*) sem tocar o objeto apontado nem exigir push, o que permite forjar o commit-alvo que \`trackfw release tag\` publicaria. Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."
    ;;
  worktree-remove-force)
    REASON="trackfw: git worktree remove -f/--force bruto bloqueado — remove um worktree e descarta de forma irreversível qualquer alteração não commitada nele. \`git worktree remove\` sem force segue liberado (recusa sozinho quando há algo não commitado). Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."
    ;;
  rm-force)
    REASON="trackfw: git rm -f/--force bruto bloqueado — apaga arquivos do working tree e do index de forma irreversível, mesma classe de \`git clean -f\`/\`git reset --hard\` já bloqueados. Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."
    ;;
  *)
    exit 0
    ;;
esac

printf '{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"%s"}}\n' "$REASON"
echo "$REASON" >&2
exit 2
