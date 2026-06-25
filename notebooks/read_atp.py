"""
Puente ATP -> Python: lectores de resultados de ATP y comparación con DPsim.

Dos vías para traer resultados de ATP a Python:

  1) RUTA .mat (RECOMENDADA, robusta):
     En ATP conviertes el .pl4 a .mat con Pl42mat.exe (incluido en ATPDraw),
     y aquí lo cargas con scipy.io.loadmat. Sin sorpresas de formato binario.

  2) RUTA .pl4 (respaldo): parser del binario nativo de ATP. El formato PL4
     tiene variantes; VALIDA la primera lectura contra un resultado conocido.

Uso:
    from read_atp import load_mat, read_pl4, compare_with_dpsim
"""
import struct
import numpy as np
import pandas as pd


# ---------------------------------------------------------------------------
# RUTA 1 — .mat  (recomendada)
# ---------------------------------------------------------------------------
def load_mat(matfile):
    """Carga un .mat generado por Pl42mat.exe y devuelve un DataFrame.

    La primera columna suele ser el tiempo. Devuelve todas las señales
    indexadas por tiempo.
    """
    from scipy.io import loadmat
    raw = loadmat(matfile)
    # Descarta las claves meta de MATLAB (__header__, __version__, __globals__)
    data = {k: np.asarray(v).squeeze() for k, v in raw.items()
            if not k.startswith("__")}
    df = pd.DataFrame(data)
    # Si hay una columna de tiempo (t / time), la usamos como índice
    for tname in ("t", "time", "Time", "TIME"):
        if tname in df.columns:
            df = df.set_index(tname)
            df.index.name = "time"
            break
    return df


# ---------------------------------------------------------------------------
# RUTA 2 — .pl4  (respaldo; validar contra un caso conocido)
# ---------------------------------------------------------------------------
def read_pl4(pl4file):
    """Lee un archivo .pl4 binario de ATP.

    Devuelve (df, info) donde df tiene el tiempo como índice y una columna
    por variable, e info contiene deltat, nvar y steps.

    NOTA: el formato PL4 tiene variantes según la versión de ATP. Si los
    valores no cuadran, usa mejor la RUTA .mat (load_mat).
    """
    with open(pl4file, "rb") as f:
        raw = f.read()

    deltat = struct.unpack("<f", raw[40:44])[0]
    nvar = struct.unpack("<L", raw[48:52])[0] // 2
    pl4size = struct.unpack("<L", raw[56:60])[0] - 1
    steps = (pl4size - 5 * 16 - nvar * 16) // ((nvar + 1) * 4)

    # Cabecera: tipo + nombre origen/destino de cada variable
    names = []
    for i in range(nvar):
        pos = 5 * 16 + i * 16
        chunk = raw[pos:pos + 16]
        frm = chunk[3:9].decode("ascii", "replace").strip()
        to = chunk[9:15].decode("ascii", "replace").strip()
        names.append(f"{frm}-{to}".strip("-") or f"v{i}")

    # Bloque de datos: por cada paso -> (nvar+1) floats (tiempo + variables)
    data_start = 5 * 16 + nvar * 16
    block = np.frombuffer(
        raw[data_start:data_start + steps * (nvar + 1) * 4], dtype="<f4"
    )
    block = block.reshape(steps, nvar + 1)

    t = np.arange(steps) * deltat
    df = pd.DataFrame(block[:, 1:], columns=names, index=t)
    df.index.name = "time"
    info = {"deltat": deltat, "nvar": nvar, "steps": steps}
    return df, info


# ---------------------------------------------------------------------------
# Comparación ATP vs DPsim
# ---------------------------------------------------------------------------
def compare_with_dpsim(atp_df, atp_col, dpsim_csv="logs/emt_rlc.csv",
                       dpsim_col="v_cap", out="atp_vs_dpsim.png"):
    """Superpone una señal de ATP con una de DPsim y guarda la gráfica."""
    import matplotlib
    matplotlib.use("Agg")
    import matplotlib.pyplot as plt

    dp = pd.read_csv(dpsim_csv)
    dp.columns = [c.strip() for c in dp.columns]

    fig, ax = plt.subplots(figsize=(9, 5))
    ax.plot(atp_df.index * 1000, atp_df[atp_col], "k-", lw=1.5, label=f"ATP: {atp_col}")
    ax.plot(dp["time"] * 1000, dp[dpsim_col], "r--", lw=1.2, label=f"DPsim: {dpsim_col}")
    ax.set_xlabel("tiempo [ms]")
    ax.set_ylabel("magnitud")
    ax.set_title("Comparación EMT: ATP vs DPsim")
    ax.legend()
    ax.grid(True)
    fig.tight_layout()
    fig.savefig(out, dpi=110)
    print(f"Gráfica comparativa guardada en: {out}")
    return out


if __name__ == "__main__":
    import sys
    if len(sys.argv) < 2:
        print("Uso: python read_atp.py <archivo.pl4 | archivo.mat>")
        sys.exit(1)
    path = sys.argv[1]
    if path.lower().endswith(".mat"):
        df = load_mat(path)
        print("Señales:", list(df.columns))
        print(df.head())
    else:
        df, info = read_pl4(path)
        print("Info:", info)
        print("Señales:", list(df.columns))
        print(df.head())
