#!/bin/bash
# Doble clic = prueba viva con la VOZ NEURONAL nueva (openai_tts).
cd "$HOME/my_prj" || { echo "No encuentro ~/my_prj"; read -r; exit 1; }
bash scripts/prueba.sh --voz
echo ""
echo "── El agente terminó. Puedes cerrar esta ventana. ──"
read -r
