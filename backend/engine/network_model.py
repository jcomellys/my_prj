"""
Panama 230kV Power System Network Model.

Complete model of the Sistema Interconectado Nacional (SIN) at 230kV,
including all substations (buses), transmission lines, generators,
and loads based on ETESA/CND public data.
"""

import numpy as np
from dataclasses import dataclass, field
from enum import Enum
from typing import Optional


class BusType(Enum):
    SLACK = 1      # Reference bus
    PV = 2         # Generator bus (voltage controlled)
    PQ = 3         # Load bus


class SubstationType(Enum):
    REDUCTORA = "Reductora"
    SECCIONADORA = "Seccionadora"
    GIS = "GIS"
    GENERACION = "Generación"
    INTERCONEXION = "Interconexión"


@dataclass
class Bus:
    id: int
    name: str
    bus_type: BusType
    substation_type: SubstationType
    voltage_kv: float = 230.0
    v_mag_pu: float = 1.0
    v_ang_rad: float = 0.0
    p_gen_mw: float = 0.0
    q_gen_mvar: float = 0.0
    p_load_mw: float = 0.0
    q_load_mvar: float = 0.0
    q_min_mvar: float = -999.0
    q_max_mvar: float = 999.0
    latitude: float = 0.0
    longitude: float = 0.0
    province: str = ""
    # Shunt compensation
    b_shunt_mvar: float = 0.0


@dataclass
class Branch:
    id: int
    name: str
    from_bus: int
    to_bus: int
    r_pu: float          # Resistance in per-unit
    x_pu: float          # Reactance in per-unit
    b_pu: float = 0.0    # Total line charging susceptance in per-unit
    rate_mva: float = 500.0  # Thermal rating in MVA
    length_km: float = 0.0
    is_transformer: bool = False
    tap_ratio: float = 1.0
    is_active: bool = True
    circuits: int = 1     # Number of parallel circuits


@dataclass
class Generator:
    id: int
    name: str
    bus_id: int
    fuel_type: str
    p_max_mw: float
    p_min_mw: float = 0.0
    q_max_mvar: float = 100.0
    q_min_mvar: float = -100.0
    cost_per_mwh: float = 0.0  # Marginal cost USD/MWh
    ramp_rate_mw_min: float = 5.0
    inertia_constant_h: float = 4.0


def build_panama_network():
    """
    Build the complete Panama 230kV network model.

    Returns a dict with buses, branches, and generators representing
    the Sistema Interconectado Nacional (SIN).

    Bus numbering:
      1-12:  Reductoras
      13-17: Seccionadoras
      18-21: New/Project substations
      22-25: Generation connection points
      26-27: International interconnection points
    """

    buses = [
        # === SUBESTACIONES REDUCTORAS 230/115 kV ===
        Bus(1, "Panamá", BusType.SLACK, SubstationType.REDUCTORA,
            p_load_mw=450, q_load_mvar=180,
            latitude=9.0000, longitude=-79.5000,
            province="Panamá"),

        Bus(2, "Panamá II", BusType.PQ, SubstationType.REDUCTORA,
            p_load_mw=380, q_load_mvar=150,
            latitude=9.0300, longitude=-79.5200,
            province="Panamá"),

        Bus(3, "Chorrera", BusType.PQ, SubstationType.REDUCTORA,
            p_load_mw=200, q_load_mvar=80,
            latitude=8.8800, longitude=-79.7800,
            province="Panamá Oeste"),

        Bus(4, "Llano Sánchez", BusType.PQ, SubstationType.REDUCTORA,
            p_load_mw=120, q_load_mvar=48,
            latitude=8.5500, longitude=-80.1200,
            province="Coclé"),

        Bus(5, "Mata de Nance", BusType.PQ, SubstationType.REDUCTORA,
            p_load_mw=80, q_load_mvar=32,
            latitude=8.4200, longitude=-81.7700,
            province="Chiriquí"),

        Bus(6, "Progreso", BusType.PQ, SubstationType.INTERCONEXION,
            p_load_mw=30, q_load_mvar=12,
            latitude=8.5100, longitude=-82.8700,
            province="Chiriquí"),

        Bus(7, "Charco Azul", BusType.PQ, SubstationType.REDUCTORA,
            p_load_mw=25, q_load_mvar=10,
            latitude=8.3700, longitude=-82.8600,
            province="Chiriquí"),

        Bus(8, "Changuinola", BusType.PQ, SubstationType.REDUCTORA,
            p_load_mw=35, q_load_mvar=14,
            latitude=9.4300, longitude=-82.5200,
            province="Bocas del Toro"),

        Bus(9, "Caldera", BusType.PV, SubstationType.REDUCTORA,
            p_gen_mw=118, v_mag_pu=1.02,
            q_max_mvar=60, q_min_mvar=-30,
            p_load_mw=15, q_load_mvar=6,
            latitude=8.6800, longitude=-82.3200,
            province="Chiriquí"),

        Bus(10, "Boquerón I", BusType.PQ, SubstationType.REDUCTORA,
            p_load_mw=55, q_load_mvar=22,
            latitude=8.4800, longitude=-82.6500,
            province="Chiriquí"),

        Bus(11, "San Bartolo", BusType.PQ, SubstationType.REDUCTORA,
            p_load_mw=60, q_load_mvar=24,
            b_shunt_mvar=60.0,  # 2x30 MVAr capacitor banks installed
            latitude=8.1100, longitude=-80.9600,
            province="Veraguas"),

        Bus(12, "Cañazas", BusType.PQ, SubstationType.REDUCTORA,
            p_load_mw=30, q_load_mvar=12,
            latitude=8.2700, longitude=-81.2200,
            province="Veraguas"),

        # === SUBESTACIONES SECCIONADORAS 230 kV ===
        Bus(13, "Cáceres", BusType.PQ, SubstationType.SECCIONADORA,
            latitude=8.3500, longitude=-80.3500,
            province="Coclé"),

        Bus(14, "Santa Rita", BusType.PQ, SubstationType.SECCIONADORA,
            latitude=8.6200, longitude=-80.5800,
            province="Coclé"),

        Bus(15, "Guasquitas", BusType.PV, SubstationType.SECCIONADORA,
            p_gen_mw=350, v_mag_pu=1.03,
            q_max_mvar=200, q_min_mvar=-100,
            latitude=8.6700, longitude=-82.1500,
            province="Chiriquí"),

        Bus(16, "Veladero", BusType.PQ, SubstationType.SECCIONADORA,
            latitude=8.4700, longitude=-81.1000,
            province="Veraguas"),

        Bus(17, "El Higo", BusType.PQ, SubstationType.SECCIONADORA,
            latitude=8.6000, longitude=-80.0200,
            province="Coclé"),

        # === NUEVAS SUBESTACIONES ===
        Bus(18, "Panamá III", BusType.PQ, SubstationType.GIS,
            p_load_mw=300, q_load_mvar=120,
            latitude=9.1000, longitude=-79.4500,
            province="Panamá"),

        Bus(19, "Sabanitas", BusType.PQ, SubstationType.GIS,
            p_load_mw=100, q_load_mvar=40,
            latitude=9.3600, longitude=-79.7500,
            province="Colón"),

        Bus(20, "Burunga", BusType.PQ, SubstationType.GIS,
            p_load_mw=150, q_load_mvar=60,
            latitude=8.9500, longitude=-79.6500,
            province="Panamá Oeste"),

        Bus(21, "Chiriquí Grande", BusType.PQ, SubstationType.REDUCTORA,
            p_load_mw=20, q_load_mvar=8,
            latitude=8.9600, longitude=-82.1200,
            province="Bocas del Toro"),

        # === NODOS DE GENERACIÓN (Connection points for major plants) ===
        Bus(22, "Fortuna", BusType.PV, SubstationType.GENERACION,
            p_gen_mw=300, v_mag_pu=1.04,
            q_max_mvar=150, q_min_mvar=-80,
            latitude=8.7500, longitude=-82.2000,
            province="Chiriquí"),

        Bus(23, "Bayano", BusType.PV, SubstationType.GENERACION,
            p_gen_mw=260, v_mag_pu=1.02,
            q_max_mvar=130, q_min_mvar=-60,
            latitude=9.1700, longitude=-78.9800,
            province="Panamá"),

        Bus(24, "BLM (Bahía Las Minas)", BusType.PV, SubstationType.GENERACION,
            p_gen_mw=255, v_mag_pu=1.01,
            q_max_mvar=130, q_min_mvar=-65,
            latitude=9.3700, longitude=-79.8500,
            province="Colón"),

        Bus(25, "Costa Norte (AES/Gatún)", BusType.PV, SubstationType.GENERACION,
            p_gen_mw=490, v_mag_pu=1.03,
            q_max_mvar=260, q_min_mvar=-130,
            latitude=9.2800, longitude=-79.9200,
            province="Colón"),

        # === INTERCONEXIÓN REGIONAL ===
        Bus(26, "Frontera Costa Rica (SIEPAC)", BusType.PQ, SubstationType.INTERCONEXION,
            latitude=8.5300, longitude=-82.9500,
            province="Chiriquí"),

        Bus(27, "24 de Diciembre", BusType.PQ, SubstationType.REDUCTORA,
            p_load_mw=180, q_load_mvar=72,
            latitude=9.0700, longitude=-79.4000,
            province="Panamá"),
    ]

    # Per-unit impedance calculations use Zbase = V^2/S = 230^2/100 = 529 ohms
    # Typical 230kV line: z = 0.02 + j0.08 pu/100km, b = 0.06 pu/100km
    branches = [
        # === TRONCAL PRINCIPAL (Oeste a Este) ===
        # Progreso - Mata de Nance
        Branch(1, "Progreso - Mata de Nance 230kV",
               6, 5, r_pu=0.0025, x_pu=0.0100, b_pu=0.0080,
               rate_mva=350, length_km=50),

        # Mata de Nance - Boquerón I
        Branch(2, "Mata de Nance - Boquerón I 230kV",
               5, 10, r_pu=0.0018, x_pu=0.0072, b_pu=0.0058,
               rate_mva=350, length_km=36),

        # Boquerón I - Veladero
        Branch(3, "Boquerón I - Veladero 230kV",
               10, 16, r_pu=0.0045, x_pu=0.0180, b_pu=0.0140,
               rate_mva=500, length_km=90),

        # Veladero - Llano Sánchez
        Branch(4, "Veladero - Llano Sánchez 230kV",
               16, 4, r_pu=0.0040, x_pu=0.0160, b_pu=0.0125,
               rate_mva=500, length_km=80),

        # Llano Sánchez - El Higo
        Branch(5, "Llano Sánchez - El Higo 230kV",
               4, 17, r_pu=0.0015, x_pu=0.0060, b_pu=0.0048,
               rate_mva=500, length_km=30),

        # El Higo - Chorrera
        Branch(6, "El Higo - Chorrera 230kV",
               17, 3, r_pu=0.0025, x_pu=0.0100, b_pu=0.0080,
               rate_mva=500, length_km=50),

        # Chorrera - Panamá
        Branch(7, "Chorrera - Panamá 230kV",
               3, 1, r_pu=0.0020, x_pu=0.0080, b_pu=0.0064,
               rate_mva=500, length_km=40),

        # === SEGUNDA LÍNEA (Guasquitas - Panamá II) ===
        # Guasquitas - Cáceres (doble circuito)
        Branch(8, "Guasquitas - Cáceres 230kV DC",
               15, 13, r_pu=0.0035, x_pu=0.0140, b_pu=0.0220,
               rate_mva=800, length_km=70, circuits=2),

        # Cáceres - Santa Rita
        Branch(9, "Cáceres - Santa Rita 230kV",
               13, 14, r_pu=0.0012, x_pu=0.0048, b_pu=0.0038,
               rate_mva=500, length_km=24),

        # Santa Rita - Panamá II
        Branch(10, "Santa Rita - Panamá II 230kV DC",
               14, 2, r_pu=0.0050, x_pu=0.0200, b_pu=0.0320,
               rate_mva=800, length_km=100, circuits=2),

        # === TERCERA LÍNEA (Sabanitas - Panamá III) ===
        Branch(11, "Sabanitas - Panamá III 230kV DC",
               19, 18, r_pu=0.0012, x_pu=0.0045, b_pu=0.0072,
               rate_mva=1000, length_km=46, circuits=2),

        # === RAMALES GENERACIÓN OCCIDENTAL ===
        # Fortuna - Guasquitas
        Branch(12, "Fortuna - Guasquitas 230kV",
               22, 15, r_pu=0.0010, x_pu=0.0040, b_pu=0.0032,
               rate_mva=500, length_km=20),

        # Guasquitas - Cañazas
        Branch(13, "Guasquitas - Cañazas 230kV",
               15, 12, r_pu=0.0020, x_pu=0.0080, b_pu=0.0064,
               rate_mva=350, length_km=40),

        # Guasquitas - Changuinola
        Branch(14, "Guasquitas - Changuinola 230kV",
               15, 8, r_pu=0.0035, x_pu=0.0140, b_pu=0.0110,
               rate_mva=350, length_km=70),

        # Caldera - Guasquitas
        Branch(15, "Caldera - Guasquitas 230kV",
               9, 15, r_pu=0.0015, x_pu=0.0060, b_pu=0.0048,
               rate_mva=350, length_km=30),

        # === RAMALES ZONA ESTE ===
        # Panamá - 24 de Diciembre
        Branch(16, "Panamá - 24 de Diciembre 230kV",
               1, 27, r_pu=0.0010, x_pu=0.0040, b_pu=0.0032,
               rate_mva=500, length_km=20),

        # 24 de Diciembre - Bayano
        Branch(17, "24 de Diciembre - Bayano 230kV",
               27, 23, r_pu=0.0030, x_pu=0.0120, b_pu=0.0096,
               rate_mva=500, length_km=60),

        # === GENERACIÓN COLÓN ===
        # Costa Norte - Sabanitas
        Branch(18, "Costa Norte - Sabanitas 230kV",
               25, 19, r_pu=0.0008, x_pu=0.0032, b_pu=0.0025,
               rate_mva=500, length_km=15),

        # BLM - Sabanitas
        Branch(19, "BLM - Sabanitas 230kV",
               24, 19, r_pu=0.0005, x_pu=0.0020, b_pu=0.0016,
               rate_mva=400, length_km=10),

        # === INTERCONEXIONES INTERNAS ===
        # Panamá II - Panamá
        Branch(20, "Panamá II - Panamá 230kV",
               2, 1, r_pu=0.0008, x_pu=0.0030, b_pu=0.0024,
               rate_mva=500, length_km=15),

        # Panamá III - Panamá
        Branch(21, "Panamá III - Panamá 230kV",
               18, 1, r_pu=0.0006, x_pu=0.0025, b_pu=0.0020,
               rate_mva=600, length_km=12),

        # Burunga - Chorrera
        Branch(22, "Burunga - Chorrera 230kV",
               20, 3, r_pu=0.0010, x_pu=0.0040, b_pu=0.0032,
               rate_mva=500, length_km=20),

        # Burunga - Panamá III
        Branch(23, "Burunga - Panamá III 230kV",
               20, 18, r_pu=0.0008, x_pu=0.0032, b_pu=0.0025,
               rate_mva=500, length_km=16),

        # San Bartolo - Veladero
        Branch(24, "San Bartolo - Veladero 230kV",
               11, 16, r_pu=0.0020, x_pu=0.0080, b_pu=0.0064,
               rate_mva=350, length_km=40),

        # Cañazas - San Bartolo
        Branch(25, "Cañazas - San Bartolo 230kV",
               12, 11, r_pu=0.0015, x_pu=0.0060, b_pu=0.0048,
               rate_mva=350, length_km=30),

        # === INTERCONEXIÓN SIEPAC ===
        # Progreso - Frontera Costa Rica
        Branch(26, "Progreso - Frontera CR (SIEPAC) 230kV",
               6, 26, r_pu=0.0010, x_pu=0.0040, b_pu=0.0032,
               rate_mva=300, length_km=20),

        # Charco Azul - Mata de Nance
        Branch(27, "Charco Azul - Mata de Nance 230kV",
               7, 5, r_pu=0.0012, x_pu=0.0048, b_pu=0.0038,
               rate_mva=350, length_km=24),

        # Mata de Nance - Boquerón I (via Boquerón III)
        Branch(28, "Mata de Nance - Progreso (via Boquerón III) 230kV",
               5, 6, r_pu=0.0040, x_pu=0.0160, b_pu=0.0125,
               rate_mva=350, length_km=80),

        # Chiriquí Grande - Changuinola
        Branch(29, "Chiriquí Grande - Changuinola 230kV",
               21, 8, r_pu=0.0025, x_pu=0.0100, b_pu=0.0080,
               rate_mva=350, length_km=50),

        # Panamá II - Panamá III
        Branch(30, "Panamá II - Panamá III 230kV",
               2, 18, r_pu=0.0005, x_pu=0.0020, b_pu=0.0016,
               rate_mva=600, length_km=10),
    ]

    generators = [
        # ============================================================
        # HYDROELECTRIC - Western Region (Chiriquí / Bocas del Toro)
        # Total: ~1,214 MW
        # ============================================================
        Generator(1, "C.H. Fortuna (ENEL)", 22, "Hydro",
                  p_max_mw=300, p_min_mw=30,
                  q_max_mvar=150, q_min_mvar=-80,
                  cost_per_mwh=15.0, inertia_constant_h=4.5),

        Generator(2, "C.H. Estí (AES)", 15, "Hydro",
                  p_max_mw=120, p_min_mw=10,
                  q_max_mvar=60, q_min_mvar=-30,
                  cost_per_mwh=16.0, inertia_constant_h=4.0),

        Generator(3, "C.H. Changuinola I (AES)", 8, "Hydro",
                  p_max_mw=221, p_min_mw=20,
                  q_max_mvar=110, q_min_mvar=-50,
                  cost_per_mwh=12.0, inertia_constant_h=4.0),

        Generator(4, "C.H. Changuinola Mini-Hidro", 8, "Hydro",
                  p_max_mw=10, p_min_mw=1,
                  q_max_mvar=5, q_min_mvar=-3,
                  cost_per_mwh=13.0, inertia_constant_h=2.0),

        Generator(5, "C.H. Bonyic", 8, "Hydro",
                  p_max_mw=32, p_min_mw=3,
                  q_max_mvar=16, q_min_mvar=-8,
                  cost_per_mwh=14.0, inertia_constant_h=3.0),

        Generator(6, "C.H. La Estrella (AES)", 15, "Hydro",
                  p_max_mw=48, p_min_mw=5,
                  q_max_mvar=24, q_min_mvar=-12,
                  cost_per_mwh=17.0, inertia_constant_h=3.5),

        Generator(7, "C.H. Los Valles (AES)", 15, "Hydro",
                  p_max_mw=54, p_min_mw=5,
                  q_max_mvar=27, q_min_mvar=-13,
                  cost_per_mwh=17.0, inertia_constant_h=3.5),

        Generator(8, "C.H. Caldera", 9, "Hydro",
                  p_max_mw=60, p_min_mw=5,
                  q_max_mvar=30, q_min_mvar=-15,
                  cost_per_mwh=18.0, inertia_constant_h=3.5),

        # Cuenca Río Chiriquí Viejo - connected via Guasquitas/Caldera
        Generator(9, "C.H. El Alto", 15, "Hydro",
                  p_max_mw=72, p_min_mw=7,
                  q_max_mvar=36, q_min_mvar=-18,
                  cost_per_mwh=16.0, inertia_constant_h=3.5),

        Generator(10, "C.H. Bajo Mina", 15, "Hydro",
                  p_max_mw=57, p_min_mw=5,
                  q_max_mvar=28, q_min_mvar=-14,
                  cost_per_mwh=16.5, inertia_constant_h=3.5),

        Generator(11, "C.H. Monte Lirio", 15, "Hydro",
                  p_max_mw=52, p_min_mw=5,
                  q_max_mvar=26, q_min_mvar=-13,
                  cost_per_mwh=17.0, inertia_constant_h=3.0),

        Generator(12, "C.H. Pando", 9, "Hydro",
                  p_max_mw=33, p_min_mw=3,
                  q_max_mvar=16, q_min_mvar=-8,
                  cost_per_mwh=17.5, inertia_constant_h=3.0),

        Generator(13, "C.H. Lorena (Alternegy)", 10, "Hydro",
                  p_max_mw=34, p_min_mw=3,
                  q_max_mvar=17, q_min_mvar=-8,
                  cost_per_mwh=18.0, inertia_constant_h=3.0),

        Generator(14, "C.H. Gualaca", 10, "Hydro",
                  p_max_mw=32, p_min_mw=3,
                  q_max_mvar=16, q_min_mvar=-8,
                  cost_per_mwh=18.0, inertia_constant_h=3.0),

        Generator(15, "C.H. Macho de Monte", 10, "Hydro",
                  p_max_mw=9, p_min_mw=1,
                  q_max_mvar=5, q_min_mvar=-3,
                  cost_per_mwh=20.0, inertia_constant_h=2.5),

        Generator(16, "C.H. Concepción", 10, "Hydro",
                  p_max_mw=11, p_min_mw=1,
                  q_max_mvar=5, q_min_mvar=-3,
                  cost_per_mwh=19.0, inertia_constant_h=2.5),

        Generator(17, "C.H. Cochea (Alto Valle)", 9, "Hydro",
                  p_max_mw=12, p_min_mw=1,
                  q_max_mvar=6, q_min_mvar=-3,
                  cost_per_mwh=19.0, inertia_constant_h=2.5),

        Generator(18, "C.H. Dolega", 10, "Hydro",
                  p_max_mw=3, p_min_mw=0,
                  q_max_mvar=2, q_min_mvar=-1,
                  cost_per_mwh=22.0, inertia_constant_h=2.0),

        Generator(19, "C.H. Paso Ancho", 9, "Hydro",
                  p_max_mw=7, p_min_mw=1,
                  q_max_mvar=3, q_min_mvar=-2,
                  cost_per_mwh=20.0, inertia_constant_h=2.5),

        Generator(20, "C.H. Bajos del Totuma", 9, "Hydro",
                  p_max_mw=6, p_min_mw=1,
                  q_max_mvar=3, q_min_mvar=-2,
                  cost_per_mwh=21.0, inertia_constant_h=2.0),

        # ============================================================
        # HYDROELECTRIC - Central Region (Veraguas / Coclé)
        # ============================================================
        Generator(21, "C.H. La Yeguada", 11, "Hydro",
                  p_max_mw=7, p_min_mw=1,
                  q_max_mvar=3, q_min_mvar=-2,
                  cost_per_mwh=20.0, inertia_constant_h=2.5),

        Generator(22, "C.H. Pedregalito I/II", 16, "Hydro",
                  p_max_mw=20, p_min_mw=2,
                  q_max_mvar=10, q_min_mvar=-5,
                  cost_per_mwh=19.0, inertia_constant_h=2.5),

        Generator(23, "C.H. Los Planetas", 12, "Hydro",
                  p_max_mw=5, p_min_mw=0,
                  q_max_mvar=3, q_min_mvar=-1,
                  cost_per_mwh=21.0, inertia_constant_h=2.0),

        Generator(24, "C.H. El Fraile", 16, "Hydro",
                  p_max_mw=4, p_min_mw=0,
                  q_max_mvar=2, q_min_mvar=-1,
                  cost_per_mwh=22.0, inertia_constant_h=2.0),

        Generator(25, "C.H. Antón I/II/III", 17, "Hydro",
                  p_max_mw=10, p_min_mw=1,
                  q_max_mvar=5, q_min_mvar=-3,
                  cost_per_mwh=20.0, inertia_constant_h=2.0),

        Generator(26, "C.H. Hidros Coclé (Agr.)", 4, "Hydro",
                  p_max_mw=15, p_min_mw=1,
                  q_max_mvar=8, q_min_mvar=-4,
                  cost_per_mwh=20.0, inertia_constant_h=2.5),

        # ============================================================
        # HYDROELECTRIC - Eastern Region (Panamá)
        # ============================================================
        Generator(27, "C.H. Bayano (AES)", 23, "Hydro",
                  p_max_mw=260, p_min_mw=25,
                  q_max_mvar=130, q_min_mvar=-60,
                  cost_per_mwh=14.0, inertia_constant_h=4.5),

        # Autogeneración Canal de Panamá (inyecta excedentes al SIN)
        Generator(28, "C.H. Gatún (ACP)", 25, "Hydro",
                  p_max_mw=36, p_min_mw=5,
                  q_max_mvar=18, q_min_mvar=-9,
                  cost_per_mwh=10.0, inertia_constant_h=3.5),

        Generator(29, "C.H. Madden (ACP)", 17, "Hydro",
                  p_max_mw=36, p_min_mw=5,
                  q_max_mvar=18, q_min_mvar=-9,
                  cost_per_mwh=10.0, inertia_constant_h=3.5),

        # ============================================================
        # THERMAL - Gas Natural (GNL)
        # Total: ~600 MW
        # ============================================================
        Generator(30, "C.T. Costa Norte (AES GNL)", 25, "Natural Gas",
                  p_max_mw=381, p_min_mw=100,
                  q_max_mvar=200, q_min_mvar=-100,
                  cost_per_mwh=48.0, inertia_constant_h=5.5),

        Generator(31, "C.T. Generadora Gatún (GNL)", 25, "Natural Gas",
                  p_max_mw=72, p_min_mw=20,
                  q_max_mvar=36, q_min_mvar=-18,
                  cost_per_mwh=50.0, inertia_constant_h=5.0),

        Generator(32, "C.T. Termo Colón (GENA CC)", 19, "Natural Gas",
                  p_max_mw=150, p_min_mw=40,
                  q_max_mvar=75, q_min_mvar=-38,
                  cost_per_mwh=52.0, inertia_constant_h=5.0),

        # ============================================================
        # THERMAL - Carbon / Pet Coke
        # ============================================================
        Generator(33, "C.T. BLM Vapor (Carbón)", 24, "Coal/Pet Coke",
                  p_max_mw=120, p_min_mw=40,
                  q_max_mvar=60, q_min_mvar=-30,
                  cost_per_mwh=65.0, inertia_constant_h=5.0),

        # ============================================================
        # THERMAL - Bunker / Diesel (MCI y Turbinas)
        # Total: ~850 MW
        # ============================================================
        Generator(34, "C.T. BLM Cativá (MCI)", 24, "Diesel/Bunker",
                  p_max_mw=87, p_min_mw=25,
                  q_max_mvar=44, q_min_mvar=-22,
                  cost_per_mwh=90.0, inertia_constant_h=4.0),

        Generator(35, "C.T. Pacora/Pedregal Power", 27, "Diesel/Bunker",
                  p_max_mw=55, p_min_mw=15,
                  q_max_mvar=28, q_min_mvar=-14,
                  cost_per_mwh=95.0, inertia_constant_h=4.0),

        Generator(36, "C.T. COPESA", 27, "Diesel/Bunker",
                  p_max_mw=40, p_min_mw=10,
                  q_max_mvar=20, q_min_mvar=-10,
                  cost_per_mwh=100.0, inertia_constant_h=3.5),

        Generator(37, "C.T. Pan-Am Generating", 3, "Diesel/Bunker",
                  p_max_mw=60, p_min_mw=15,
                  q_max_mvar=30, q_min_mvar=-15,
                  cost_per_mwh=92.0, inertia_constant_h=4.0),

        Generator(38, "C.T. Térmica del Caribe (Giral)", 24, "Diesel/Bunker",
                  p_max_mw=48, p_min_mw=12,
                  q_max_mvar=24, q_min_mvar=-12,
                  cost_per_mwh=88.0, inertia_constant_h=4.0),

        Generator(39, "C.T. Tropitérmica", 1, "Diesel/Bunker",
                  p_max_mw=40, p_min_mw=10,
                  q_max_mvar=20, q_min_mvar=-10,
                  cost_per_mwh=96.0, inertia_constant_h=3.5),

        Generator(40, "C.T. IDB Chilibre", 20, "Diesel/Bunker",
                  p_max_mw=30, p_min_mw=8,
                  q_max_mvar=15, q_min_mvar=-8,
                  cost_per_mwh=98.0, inertia_constant_h=3.5),

        # Canal de Panamá - Térmicas (autogeneración con excedentes al SIN)
        Generator(41, "C.T. Miraflores (ACP)", 1, "Diesel/Bunker",
                  p_max_mw=80, p_min_mw=20,
                  q_max_mvar=40, q_min_mvar=-20,
                  cost_per_mwh=85.0, inertia_constant_h=4.5),

        Generator(42, "C.T. ACP Térmica Adicional", 20, "Diesel/Bunker",
                  p_max_mw=152, p_min_mw=40,
                  q_max_mvar=76, q_min_mvar=-38,
                  cost_per_mwh=88.0, inertia_constant_h=4.5),

        # Térmicas menores distribuidas (agregado)
        Generator(43, "C.T. Dist. Panamá (Agr.)", 2, "Diesel/Bunker",
                  p_max_mw=80, p_min_mw=20,
                  q_max_mvar=40, q_min_mvar=-20,
                  cost_per_mwh=105.0, inertia_constant_h=3.5),

        Generator(44, "C.T. Dist. Colón (Agr.)", 19, "Diesel/Bunker",
                  p_max_mw=60, p_min_mw=15,
                  q_max_mvar=30, q_min_mvar=-15,
                  cost_per_mwh=102.0, inertia_constant_h=3.5),

        Generator(45, "C.T. Dist. Interior (Agr.)", 4, "Diesel/Bunker",
                  p_max_mw=50, p_min_mw=10,
                  q_max_mvar=25, q_min_mvar=-12,
                  cost_per_mwh=110.0, inertia_constant_h=3.0),

        # ============================================================
        # WIND - Eólica
        # Total: ~336 MW
        # ============================================================
        Generator(46, "P.E. Penonomé I/II/III", 14, "Wind",
                  p_max_mw=215, p_min_mw=0,
                  q_max_mvar=50, q_min_mvar=-50,
                  cost_per_mwh=0.0, inertia_constant_h=0.0),

        Generator(47, "P.E. Toabré Fase I", 14, "Wind",
                  p_max_mw=66, p_min_mw=0,
                  q_max_mvar=16, q_min_mvar=-16,
                  cost_per_mwh=0.0, inertia_constant_h=0.0),

        Generator(48, "P.E. AES Eólica (Nuevoleón)", 14, "Wind",
                  p_max_mw=55, p_min_mw=0,
                  q_max_mvar=14, q_min_mvar=-14,
                  cost_per_mwh=0.0, inertia_constant_h=0.0),

        # ============================================================
        # SOLAR - Fotovoltaica
        # Total: ~696 MW (49 instalaciones al cierre 2024)
        # ============================================================
        Generator(49, "P.S. Penonomé Solar (Avanzalia)", 13, "Solar",
                  p_max_mw=120, p_min_mw=0,
                  q_max_mvar=36, q_min_mvar=-36,
                  cost_per_mwh=0.0, inertia_constant_h=0.0),

        Generator(50, "P.S. Ecosolares (EISA)", 4, "Solar",
                  p_max_mw=50, p_min_mw=0,
                  q_max_mvar=15, q_min_mvar=-15,
                  cost_per_mwh=0.0, inertia_constant_h=0.0),

        Generator(51, "P.S. Baco (Enel)", 10, "Solar",
                  p_max_mw=30, p_min_mw=0,
                  q_max_mvar=9, q_min_mvar=-9,
                  cost_per_mwh=0.0, inertia_constant_h=0.0),

        Generator(52, "P.S. Madre Vieja (Enel)", 10, "Solar",
                  p_max_mw=30, p_min_mw=0,
                  q_max_mvar=9, q_min_mvar=-9,
                  cost_per_mwh=0.0, inertia_constant_h=0.0),

        Generator(53, "P.S. AES Solar (4 parques)", 4, "Solar",
                  p_max_mw=40, p_min_mw=0,
                  q_max_mvar=12, q_min_mvar=-12,
                  cost_per_mwh=0.0, inertia_constant_h=0.0),

        Generator(54, "P.S. Dist. Coclé/Herrera (Agr.)", 17, "Solar",
                  p_max_mw=130, p_min_mw=0,
                  q_max_mvar=39, q_min_mvar=-39,
                  cost_per_mwh=0.0, inertia_constant_h=0.0),

        Generator(55, "P.S. Dist. Chiriquí/Veraguas (Agr.)", 16, "Solar",
                  p_max_mw=100, p_min_mw=0,
                  q_max_mvar=30, q_min_mvar=-30,
                  cost_per_mwh=0.0, inertia_constant_h=0.0),

        Generator(56, "P.S. Dist. Panamá (Agr.)", 1, "Solar",
                  p_max_mw=90, p_min_mw=0,
                  q_max_mvar=27, q_min_mvar=-27,
                  cost_per_mwh=0.0, inertia_constant_h=0.0),

        Generator(57, "P.S. Dist. Azuero/Colón (Agr.)", 19, "Solar",
                  p_max_mw=106, p_min_mw=0,
                  q_max_mvar=32, q_min_mvar=-32,
                  cost_per_mwh=0.0, inertia_constant_h=0.0),

        # ============================================================
        # SIEPAC Interconnection (Import/Export)
        # ============================================================
        Generator(58, "SIEPAC Import/Export", 26, "Interconnection",
                  p_max_mw=300, p_min_mw=-300,
                  q_max_mvar=100, q_min_mvar=-100,
                  cost_per_mwh=40.0, inertia_constant_h=0.0),
    ]

    return {"buses": buses, "branches": branches, "generators": generators}


def get_bus_by_id(buses, bus_id):
    for bus in buses:
        if bus.id == bus_id:
            return bus
    return None


def get_total_generation(generators):
    return sum(g.p_max_mw for g in generators if g.fuel_type != "Interconnection")


def get_total_load(buses):
    return sum(b.p_load_mw for b in buses)


def get_generation_by_fuel(generators):
    fuel_mix = {}
    for g in generators:
        if g.fuel_type not in fuel_mix:
            fuel_mix[g.fuel_type] = {"capacity_mw": 0, "count": 0}
        fuel_mix[g.fuel_type]["capacity_mw"] += g.p_max_mw
        fuel_mix[g.fuel_type]["count"] += 1
    return fuel_mix
