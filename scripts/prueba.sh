#!/bin/bash
# Prueba viva con un solo comando:
#   cd ~/my_prj && git pull origin claude/voice-mac-agent-V98uX && bash scripts/prueba.sh
#
# Por defecto usa LA MISMA configuración que ya funcionó en vivo con Codex
# (manos_libres + voz local macos_say). Para probar la voz neuronal nueva:
#   bash scripts/prueba.sh --voz
set -uo pipefail
cd "$(dirname "$0")/.."

BRANCH=claude/voice-mac-agent-V98uX
USAR_VOZ_NEURONAL=no
[ "${1:-}" = "--voz" ] && USAR_VOZ_NEURONAL=si

echo "── 1/4 Trayendo lo último de $BRANCH…"
ok_pull=no
for wait in 0 2 4 8 16; do
  sleep "$wait"
  if git pull origin "$BRANCH"; then ok_pull=si; break; fi
  echo "   (red caprichosa; reintentando…)"
done
[ "$ok_pull" = si ] || { echo "❌ No pude hacer git pull. Revisa la conexión y reintenta."; exit 1; }

echo "── 2/4 Compilando…"
go build ./... || { echo "❌ No compila. Pega este error a Claude."; exit 1; }

echo "── 3/4 Generando config de prueba (perfil manos_libres)…"
cp config.example.yaml config.prueba.yaml
sed -i.bak 's/^active_profile: .*/active_profile: manos_libres/' config.prueba.yaml
if [ "$USAR_VOZ_NEURONAL" = si ]; then
  echo "   Voz NEURONAL (openai_tts) activada para esta corrida."
  sed -i.bak 's/provider: macos_say/provider: openai_tts/' config.prueba.yaml
fi
rm -f config.prueba.yaml.bak

if [ ! -f .env ]; then
  echo "❌ Falta el archivo .env (con OPENAI_API_KEY) en $(pwd)."
  exit 1
fi

# Doctor: SOLO informativo. En este hardware el nivel de micrófono no se
# puede leer y marcaría ❌ aunque todo esté bien — nunca bloquea la prueba.
echo "── (info) Chequeo de entorno — ignora el ❌ del micrófono si la barra se mueve al hablar:"
go run ./cmd/agent --config config.prueba.yaml --env .env --doctor || true

echo ""
echo "── 4/4 Arrancando. ANTES DE HABLAR:"
echo "   • PDF EN INGLÉS (heading \"Introduction\") abierto en Vista Previa."
echo "   • Hotkey: Ctrl + Alt + Espacio (teclado Windows). Presiona, habla, espera."
echo "   • Frase 1: \"Léeme la sección introducción\"   → ~3s, en español."
echo "   • Frase 2: \"Explícame en breve qué es un transistor\""
echo "   • Para cortar la voz: la misma hotkey."
echo "   • Si la hotkey no responde: Ajustes → Privacidad y seguridad →"
echo "     Accesibilidad → activa Terminal. Luego reintenta."
echo "   • Log en prueba.log — pégalo a Claude si algo falla."
echo ""
go run ./cmd/agent --config config.prueba.yaml --env .env -v 2>&1 | tee prueba.log
