#!/usr/bin/env bash
# M12 (REQ-2026-11-17, AC2): o Gemini CLI instalado ainda executa shell com chave de API? Contra-braço sem chave.
# Total de chamadas de modelo: 2/2 (o braço sem chave não deve chegar ao modelo).
# Pré-requisitos: `gemini` no PATH e GEMINI_API_KEY exportada no shell que roda o script.
# Isolamento: HOME temporário nos dois braços, para que credencial OAuth de ~/.gemini não seja usada.
# Não lê nem escreve em ~/.gemini. Tudo fica em um diretório temporário, apagado ao final.
set -u
command -v gemini >/dev/null || { echo "ABORT: gemini ausente do PATH"; exit 3; }
[ -n "${GEMINI_API_KEY:-}" ] || { echo "ABORT: exporte GEMINI_API_KEY antes de rodar"; exit 3; }
W=$(mktemp -d)
trap 'rm -rf "$W"' EXIT
mkdir -p "$W/home" "$W/proj" "$W/logs"
cd "$W/proj" || exit 1
git init -q
echo "gemini: $(command -v gemini) $(gemini --version 2>&1 | head -1)"

run() {  # $1 = braço, $2 = com|sem chave
  a=$1; m="$W/proj/marker_$a.txt"
  p="Run exactly this shell command and nothing else, then stop: echo hello > $m"
  if [ "$2" = com ]; then
    env HOME="$W/home" GOOGLE_API_KEY= GOOGLE_GENAI_USE_VERTEXAI= GOOGLE_CLOUD_PROJECT= \
      gemini -p "$p" --approval-mode yolo -o text </dev/null >"$W/logs/$a.out" 2>"$W/logs/$a.err"
  else
    env -u GEMINI_API_KEY -u GOOGLE_API_KEY -u GOOGLE_GENAI_USE_VERTEXAI -u GOOGLE_CLOUD_PROJECT HOME="$W/home" \
      gemini -p "$p" --approval-mode yolo -o text </dev/null >"$W/logs/$a.out" 2>"$W/logs/$a.err"
  fi
  rc=$?
  echo "$a gemini_rc=$rc marker=$([ -f "$m" ] && echo yes || echo no)"
  echo "  stdout: $(head -c 200 "$W/logs/$a.out" | tr '\n' ' ')"
  echo "  stderr: $(K="${GEMINI_API_KEY}" python3 -c 'import os,sys; print(sys.stdin.read().replace(os.environ["K"],"<REDACTED>")[:300].replace(chr(10)," "))' <"$W/logs/$a.err")"
}

run M12a-com-chave com
echo "  (esperado: marker=yes; se stderr mostrar erro de auth/quota, o braço é INCONCLUSIVO)"
run M12b-sem-chave sem
echo "  (esperado: marker=no, com erro de autenticação)"
