#!/bin/bash
# Prueba viva con un solo comando:
#   cd ~/my_prj && git pull origin claude/voice-mac-agent-V98uX && bash scripts/prueba.sh
#
# Hace todo: trae lo último, compila, verifica el entorno (--doctor),
# genera una config de prueba (perfil manos_libres + voz neuronal openai_tts)
# y arranca el agente con log en prueba.log. No toca tu config.yaml real.
set -euo pipefail
cd "$(dirname "$0")/.."

BRANCH=claude/voice-mac-agent-V98uX

echo "── 1/5 Trayendo lo último de $BRANCH…"
for wait in 0 2 4 8 16; do
  sleep "$wait"
  git pull origin "$BRANCH" && break
  echo "   (red caprichosa; reintento en $((wait*2 || 2))s)"
done

echo "── 2/5 Compilando…"
go build ./...

echo "── 3/5 Generando config de prueba (manos_libres + voz neuronal)…"
cp config.example.yaml config.prueba.yaml
sed -i.bak 's/^active_profile: .*/active_profile: manos_libres/' config.prueba.yaml
# Voz natural tipo app de Claude: openai_tts (usa tu OPENAI_API_KEY del .env).
# Si la red falla, el agente cae solo a la voz local — nunca se queda mudo.
sed -i.bak 's/provider: macos_say/provider: openai_tts/' config.prueba.yaml
rm -f config.prueba.yaml.bak

if [ ! -f .env ]; then
  echo "❌ Falta el archivo .env (con OPENAI_API_KEY) en $(pwd). Créalo y reintenta."
  exit 1
fi

echo "── 4/5 Chequeo de entorno (doctor)…"
go run ./cmd/agent --config config.prueba.yaml --env .env --doctor || {
  echo "❌ El doctor encontró problemas arriba. Corrígelos y reintenta."; exit 1; }

echo ""
echo "── 5/5 Arrancando. ANTES DE HABLAR:"
echo "   • Ten un PDF EN INGLÉS (con heading \"Introduction\") abierto en Vista Previa."
echo "   • Frase 1: \"Léeme la sección introducción\"   → debe tardar ~3s y leer en español."
echo "   • Frase 2: \"Explícame en breve qué es un transistor\" → escucha la voz nueva."
echo "   • Hotkey ⌃⌥Espacio corta la voz en <1s."
echo "   • Log completo en prueba.log (pégalo a Claude si algo falla)."
echo ""
go run ./cmd/agent --config config.prueba.yaml --env .env -v 2>&1 | tee prueba.log
