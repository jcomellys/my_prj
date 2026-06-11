#!/bin/bash
# Doble clic = prueba viva del agente. No hay que pegar nada:
# trae lo último, compila y arranca. Cierra la ventana para detenerlo.
cd "$HOME/my_prj" || { echo "No encuentro ~/my_prj"; read -r; exit 1; }
bash scripts/prueba.sh
echo ""
echo "── El agente terminó. Puedes cerrar esta ventana. ──"
read -r
