# Setup de la wiki doctoral en macOS (Mac Mini M4)

El patrón de la wiki es solo filesystem + markdown, sin dependencias, así que
funciona igual en macOS que en la HP (Windows). Estos son los pasos para dejar
la Mac Mini operativa.

## 1. Instalar Claude Code

Instalador nativo (equivalente al de PowerShell usado en la HP):

```bash
curl -fsSL https://claude.ai/install.sh | bash
```

## 2. Ubicar la wiki

Descomprimir el zip de la wiki en el home:

```bash
mkdir -p ~/wiki-doctoral
unzip wiki-doctoral.zip -d ~/wiki-doctoral
```

## 3. Probar la compilación

```bash
cd ~/wiki-doctoral && claude
```

Y dentro de la sesión, pedir: **"compila el inbox"**.

## 4. Sincronización HP ↔ Mac (recomendado: Git)

Si el inbox se alimenta desde ambas máquinas, el directorio debe vivir en un
repo Git privado en GitHub en lugar de copiarse a mano. La wiki es markdown
puro: los diffs son legibles y el historial de compilaciones queda versionado
gratis.

Una vez, desde la máquina que tenga la versión más reciente de la wiki:

```bash
cd ~/wiki-doctoral
git init
git add -A && git commit -m "Estado inicial de la wiki"
git remote add origin git@github.com:<usuario>/<repo-privado>.git
git push -u origin main
```

En la otra máquina:

```bash
git clone git@github.com:<usuario>/<repo-privado>.git ~/wiki-doctoral
```

Flujo diario en cualquiera de las dos:

```bash
git pull          # antes de trabajar o compilar
# ... alimentar raw/_inbox/, compilar, etc.
git add -A && git commit -m "Compilación / capturas del día"
git push
```

## 5. Obsidian Web Clipper

En el navegador de la Mac, configurar la carpeta de salida del Web Clipper
apuntando directamente a `raw/_inbox/`, de modo que capturar una página deje
el material listo para la siguiente compilación en un solo paso.

## 6. Sinergia con fortuna-eng-7b

Tener la wiki en la Mac es conveniente porque ahí corre el pipeline
QLoRA/MLX: cuando la wiki madure, la pasada de generación de Q&A sintéticos
puede leer `wiki/` localmente sin mover datos.
