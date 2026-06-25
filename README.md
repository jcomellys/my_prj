# Entorno Python — Ingeniería Eléctrica

Entorno de trabajo en Python para investigación en Ingeniería Eléctrica:
sistemas de potencia, control, RF, procesamiento de señales, machine/deep
learning y análisis electromagnético (FEM).

## Instalación

```bash
pip install -r requirements.txt
pip install --no-deps lcapy==1.26   # lcapy declara mal la dep obsoleta 'importlib'
```

En sesiones web (Claude Code on the web), el hook `SessionStart` definido en
`.claude/settings.json` ejecuta `.claude/hooks/setup-env.sh` y reinstala todo
automáticamente en el contenedor efímero.

## Herramientas incluidas

| Área | Paquetes |
|---|---|
| **Núcleo científico** | numpy, scipy, matplotlib, pandas, sympy |
| **Ingeniería Eléctrica** | control, scikit-rf, PySpice (+ngspice), lcapy, networkx |
| **Sistemas de Potencia** | pandapower, pypsa |
| **Señales / Datos** | PyWavelets, h5py |
| **Machine / Deep Learning** | scikit-learn, torch, torch-geometric (GNN), tensorflow, keras, optuna, tensorboard |
| **Electromagnetismo / FEM** | magpylib, scikit-fem, sfepy, gmsh, meshio |
| **Entorno** | jupyterlab, ipython, seaborn, tqdm |

## Notas importantes

- **GPU:** el entorno es **CPU**. PyTorch/TensorFlow funcionan, pero el
  entrenamiento de modelos grandes será lento. Para GPU hace falta un entorno
  con CUDA.
- **torch + tensorflow:** importarlos en el **mismo proceso** provoca un
  segfault (conflicto conocido). Usar uno por script/cuaderno.
- **numpy fijado a <2.4:** mantiene la compatibilidad de ABI con scipy y los
  demás binarios compilados. No subir numpy sin recompilar el resto.

### Análisis electromagnético de máquinas (transformadores, motores, generadores)

El stack FEM (magpylib, scikit-fem, sfepy, gmsh) es **nativo de Linux** y cubre
magnetostática, corrientes de Foucault y ecuaciones de Maxwell.

- **pyleecan** (diseño de máquinas eléctricas) y **FEMM/pyfemm** **NO** están
  instalados. Motivos:
  - pyleecan fija `numpy<=1.23.1` y `matplotlib<=3.3.4`, lo que degradaría y
    rompería el resto del entorno (torch, tensorflow, pandapower).
  - FEMM es nativo de **Windows** (en Linux requiere Wine).
- Si necesitas pyleecan/FEMM, lo recomendable es un **entorno virtual aislado**
  dedicado, separado de este stack.
