#!/usr/bin/env bash
# A/B test: whisper-small vs whisper-medium on the same audio inputs.
#
# Pure-STT test. No GPT, no API cost, no token spend. Isolates the
# STT layer so we can decide whisper-small (fast, ~480MB) vs
# whisper-medium (slower, ~1.5GB) for the manos_libres profile.
#
# Usage:
#   bash scripts/whisper_ab.sh [manifest.txt]
#
# Manifest format (one phrase per line):
#   path/to/recording.wav | expected transcript
# Lines starting with # are comments.
#
# Models expected at:
#   ~/.whisper-models/ggml-small.bin
#   ~/.whisper-models/ggml-medium.bin
#
# Download medium if missing:
#   curl -L -o ~/.whisper-models/ggml-medium.bin \
#     https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-medium.bin

set -euo pipefail

MODEL_SMALL="${MODEL_SMALL:-$HOME/.whisper-models/ggml-small.bin}"
MODEL_MEDIUM="${MODEL_MEDIUM:-$HOME/.whisper-models/ggml-medium.bin}"
MANIFEST="${1:-samples/manifest.txt}"
LANG_HINT="${LANG_HINT:-es}"

red()    { printf "\033[31m%s\033[0m" "$1"; }
green()  { printf "\033[32m%s\033[0m" "$1"; }
bold()   { printf "\033[1m%s\033[0m" "$1"; }

[ -f "$MODEL_SMALL"  ] || { red "Falta $MODEL_SMALL\n"; exit 1; }
[ -f "$MODEL_MEDIUM" ] || { red "Falta $MODEL_MEDIUM — descárgalo (ver cabecera del script).\n"; exit 1; }
[ -f "$MANIFEST"     ] || { red "Falta manifest: $MANIFEST\n"; exit 1; }
command -v whisper-cli >/dev/null || { red "whisper-cli no en PATH (brew install whisper-cpp)\n"; exit 1; }
command -v bc          >/dev/null || { red "bc no en PATH (brew install bc)\n"; exit 1; }

# normalize: lowercase, strip punctuation, collapse whitespace
norm() {
  printf "%s" "$1" \
    | tr '[:upper:]' '[:lower:]' \
    | tr -d '.,;:!?¡¿"\047' \
    | tr -s '[:space:]' ' ' \
    | sed 's/^ //; s/ $//'
}

run_one() {
  # args: model_path  wav_file
  local model="$1" wav="$2"
  local start end lat out
  start=$(date +%s.%N)
  out=$(whisper-cli -m "$model" -l "$LANG_HINT" -f "$wav" -nt -np 2>/dev/null \
        | tr '\n' ' ' | sed 's/  */ /g; s/^ //; s/ $//')
  end=$(date +%s.%N)
  lat=$(printf "%.2f" "$(echo "$end - $start" | bc -l)")
  printf "%s|%s" "$lat" "$out"
}

total=0
small_total_lat=0
medium_total_lat=0
small_matches=0
medium_matches=0

bold "Whisper A/B  —  small vs medium\n"
echo  "model_small : $MODEL_SMALL"
echo  "model_medium: $MODEL_MEDIUM"
echo  "manifest    : $MANIFEST"
echo  "lang        : $LANG_HINT"
echo

while IFS='|' read -r filename expected; do
  filename=$(echo "$filename" | xargs || true)
  expected=$(echo "$expected" | xargs || true)
  [ -z "$filename" ] && continue
  case "$filename" in \#*) continue;; esac
  if [ ! -f "$filename" ]; then
    red "skip $filename (no encontrado)\n"
    continue
  fi

  total=$((total + 1))
  echo
  bold "=== $(basename "$filename") ===\n"
  printf "  esperado : %s\n" "$expected"

  IFS='|' read -r s_lat s_txt <<< "$(run_one "$MODEL_SMALL"  "$filename")"
  IFS='|' read -r m_lat m_txt <<< "$(run_one "$MODEL_MEDIUM" "$filename")"

  small_total_lat=$(echo "$small_total_lat + $s_lat" | bc -l)
  medium_total_lat=$(echo "$medium_total_lat + $m_lat" | bc -l)

  s_norm=$(norm "$s_txt")
  m_norm=$(norm "$m_txt")
  e_norm=$(norm "$expected")
  s_match="❌"; [ "$s_norm" = "$e_norm" ] && { s_match="✅"; small_matches=$((small_matches + 1)); }
  m_match="❌"; [ "$m_norm" = "$e_norm" ] && { m_match="✅"; medium_matches=$((medium_matches + 1)); }

  printf "  small  %ss %s : %s\n" "$s_lat" "$s_match" "$s_txt"
  printf "  medium %ss %s : %s\n" "$m_lat" "$m_match" "$m_txt"
done < "$MANIFEST"

if [ "$total" -eq 0 ]; then
  red "\nNo se procesó ningún archivo.\n"
  exit 1
fi

s_avg=$(printf "%.2f" "$(echo "$small_total_lat  / $total" | bc -l)")
m_avg=$(printf "%.2f" "$(echo "$medium_total_lat / $total" | bc -l)")

echo
bold "Resumen\n"
printf "  small  : %d/%d coincidencias exactas, latencia media %ss\n" "$small_matches"  "$total" "$s_avg"
printf "  medium : %d/%d coincidencias exactas, latencia media %ss\n" "$medium_matches" "$total" "$m_avg"
echo
echo  "Nota: la 'coincidencia exacta' es estricta tras normalizar. Las diferencias"
echo  "menores (acentos, mayúsculas) no penalizan. Errores semánticos sí. Lee las"
echo  "líneas de cada archivo para juzgar calidad cualitativamente."
