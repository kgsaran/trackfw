#!/usr/bin/env bash
# M12 (REQ-2026-11-17, AC2): o Gemini CLI instalado ainda executa shell para conta individual (login Google/OAuth)?
# Sem chave de API e sem custo (decisão do KG em 2026-11-20). Contra-braço sem credencial nenhuma.
# Total de chamadas de modelo: no máximo 1/1 (o braço sem credencial não deve chegar ao modelo).
# Pré-requisito: `gemini` no PATH, com login Google já feito (rode `gemini` uma vez de forma interativa, se preciso).
# M12a usa o seu HOME real, porque é lá que o gemini guarda o login; o script não lê nem altera arquivos de ~/.gemini.
# M12b usa um HOME temporário, sem credencial. O projeto de teste fica num diretório temporário, apagado ao final.
set -u
command -v gemini >/dev/null || { echo "ABORT: gemini ausente do PATH"; exit 3; }
W=$(mktemp -d)
trap 'rm -rf "$W"' EXIT
mkdir -p "$W/home" "$W/proj" "$W/logs"
cd "$W/proj" || exit 1
git init -q
echo "gemini: $(command -v gemini) $(gemini --version 2>&1 | head -1)"
NOKEY="-u GEMINI_API_KEY -u GOOGLE_API_KEY -u GOOGLE_GENAI_USE_VERTEXAI -u GOOGLE_GENAI_USE_GCA -u GOOGLE_CLOUD_PROJECT"

run() {  # $1 = braço, $2 = HOME a usar
  a=$1; m="$W/proj/marker_$a.txt"
  p="Run exactly this shell command and nothing else, then stop: echo hello > $m"
  # shellcheck disable=SC2086
  env $NOKEY HOME="$2" gemini -p "$p" --approval-mode yolo -o text </dev/null >"$W/logs/$a.out" 2>"$W/logs/$a.err"
  rc=$?
  echo "$a gemini_rc=$rc marker=$([ -f "$m" ] && echo yes || echo no)"
  echo "  stdout: $(head -c 300 "$W/logs/$a.out" | tr '\n' ' ')"
  echo "  stderr: $(head -c 400 "$W/logs/$a.err" | tr '\n' ' ')"
}

run M12a-login-google "$HOME"
echo "  (marker=yes: o Gemini CLI ainda executa shell para conta individual;"
echo "   marker=no com erro de conta/plano no stderr: deixou de atender conta individual — também é resultado)"
run M12b-sem-credencial "$W/home"
echo "  (esperado: marker=no, com erro de autenticação)"
