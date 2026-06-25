#!/usr/bin/env bash
# =============================================================================
# Hook de SessionStart — prepara el entorno Python de Ingeniería Eléctrica.
# Reinstala las herramientas en contenedores efímeros (sesiones web).
# Es idempotente: si todo ya está instalado, termina en segundos.
# =============================================================================
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$REPO_ROOT"

echo "[setup-env] Verificando entorno Python de Ingeniería Eléctrica..."

# Comprobación rápida: si los paquetes clave importan, no reinstalamos nada.
if python3 -c "import numpy, scipy, pandapower, torch, torch_geometric, magpylib, skfem, dpsim" >/dev/null 2>&1; then
    echo "[setup-env] Entorno ya listo. Nada que hacer."
    exit 0
fi

echo "[setup-env] Instalando dependencias (esto puede tardar varios minutos)..."

# Librerías de sistema:
#   - ngspice / libngspice0-dev : motor de simulación para PySpice
#   - libglu1-mesa (libGLU.so.1) : requerido por gmsh (mallado FEM)
echo "[setup-env] Instalando librerías de sistema (ngspice, libGLU)..."
apt-get install -y ngspice libngspice0-dev libglu1-mesa >/dev/null 2>&1 || \
    echo "[setup-env] AVISO: no se pudieron instalar libs de sistema (sin permisos/red)."

# Stack Python. --ignore-installed packaging evita el conflicto con la versión
# de 'packaging' que instala Debian (RECORD file not found).
pip3 install --ignore-installed packaging -r requirements.txt

# lcapy se instala sin dependencias (declara por error la dep obsoleta 'importlib').
pip3 install --no-deps lcapy==1.26

echo "[setup-env] Entorno listo."
