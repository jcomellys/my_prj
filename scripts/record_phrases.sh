#!/usr/bin/env bash
# Record a small corpus of voice samples for the whisper A/B test.
#
# Records 16kHz mono PCM WAVs that whisper-cli reads natively, using
# the same sox silence filter the agent runs at runtime. Output goes
# to samples/NN_<slug>.wav and a manifest.txt is written alongside.
# A short countdown avoids the operator speaking before the microphone is
# open, and a duration warning catches clipped samples early.
#
# Usage:
#   bash scripts/record_phrases.sh [output_dir]
#
# Then:
#   bash scripts/whisper_ab.sh samples/manifest.txt

set -euo pipefail

DIR="${1:-samples}"
mkdir -p "$DIR"

PHRASES=(
  "abre Google Chrome"
  "abre Mensajes"
  "abre Wikipedia y busca Isaac Newton"
  "léeme el primer párrafo de la página"
  "cierra esta pestaña"
  "qué hora es"
  "cuánto llevo gastado hoy"
  "navega a YouTube y busca Beethoven sinfonía nueve"
  "abre Word y escribe el título El sistema solar"
  "describe lo que hay en pantalla"
)

MIN_RECORDED_SECONDS="${MIN_RECORDED_SECONDS:-0.80}"

command -v sox >/dev/null || { echo "sox no instalado. brew install sox"; exit 1; }
command -v soxi >/dev/null || { echo "soxi no instalado. brew install sox"; exit 1; }

slug() {
  printf "%s" "$1" \
    | tr '[:upper:]' '[:lower:]' \
    | tr -c '[:alnum:]' '_' \
    | sed 's/__*/_/g; s/^_//; s/_$//' \
    | cut -c1-40
}

manifest="$DIR/manifest.txt"
: > "$manifest"

n=${#PHRASES[@]}
echo "Vamos a grabar $n frases. En cada una:"
echo "  1) Presiona Enter."
echo "  2) Espera la cuenta regresiva."
echo "  3) Habla solo cuando veas: HABLA AHORA."
echo "  4) Quédate en silencio ~1.5 s — la grabación se detiene sola."
echo

for i in "${!PHRASES[@]}"; do
  phrase="${PHRASES[$i]}"
  idx=$(printf "%02d" "$((i+1))")
  file="$DIR/${idx}_$(slug "$phrase").wav"

  echo
  echo "[$((i+1))/$n] di:  \"$phrase\""
  read -r -p "Enter para iniciar cuenta regresiva… " _
  for s in 3 2 1; do
    echo "$s..."
    sleep 1
  done
  echo "HABLA AHORA"
  sox -q -d -r 16000 -c 1 -b 16 "$file" \
    silence 1 0.1 3% 1 1.5 3%
  size=$(stat -f%z "$file" 2>/dev/null || stat -c%s "$file")
  dur=$(soxi -D "$file" 2>/dev/null || echo 0)
  printf "guardado: %s (%s bytes, %.2fs)\\n" "$file" "$size" "$dur"
  awk -v d="$dur" -v m="$MIN_RECORDED_SECONDS" 'BEGIN { exit !(d < m) }' && \
    echo "ADVERTENCIA: duración muy corta; probablemente no capturó la frase completa." || true
  echo "$file | $phrase" >> "$manifest"
done

echo
echo "Listo. Manifest en: $manifest"
echo
echo "Siguiente paso:"
echo "  bash scripts/whisper_ab.sh $manifest"
