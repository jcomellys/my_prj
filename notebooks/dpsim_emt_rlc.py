"""
Prueba EMT con DPsim — Energización de un circuito RLC serie.

Caso clásico de transitorio de maniobra: una fuente de 230 V / 50 Hz
energiza una rama R-L-C en serie. Se observa la oscilación amortiguada
de la tensión en el capacitor (sobretensión de maniobra).

Resultado equivalente al que se obtendría en ATP/ATPDraw, pero todo en Python.
"""
import dpsim

# --- Componentes EMT monofásicos ---
EMT = dpsim.emt
ph1 = dpsim.emt.ph1

# Nodos (n3 = nodo del capacitor; gnd = referencia)
gnd = EMT.SimNode.gnd
n1 = EMT.SimNode("n1")
n2 = EMT.SimNode("n2")
n3 = EMT.SimNode("n3")

# Fuente de tensión: 230 V pico, 50 Hz
vs = ph1.VoltageSource("vs")
vs.set_parameters(V_ref=complex(230, 0), f_src=50)
vs.connect([gnd, n1])

# R = 1 Ω, L = 10 mH, C = 100 µF en serie
r = ph1.Resistor("r")
r.set_parameters(R=1.0)
r.connect([n1, n2])

l = ph1.Inductor("l")
l.set_parameters(L=10e-3)
l.connect([n2, n3])

c = ph1.Capacitor("c")
c.set_parameters(C=100e-6)
c.connect([n3, gnd])

# --- Topología del sistema ---
sys = dpsim.SystemTopology(50, [gnd, n1, n2, n3], [vs, r, l, c])

# --- Logger: registra la tensión del capacitor y la corriente ---
logger = dpsim.Logger("emt_rlc")
logger.log_attribute("v_cap", "v_intf", c)   # tensión en el capacitor
logger.log_attribute("i_rlc", "i_intf", r)   # corriente de la rama

# --- Simulación EMT ---
sim = dpsim.Simulation("emt_rlc")
sim.set_system(sys)
sim.set_domain(dpsim.Domain.EMT)        # dominio electromagnético transitorio
sim.set_time_step(50e-6)                # 50 µs
sim.set_final_time(0.1)                 # 100 ms
sim.add_logger(logger)
sim.run()

print("OK: simulacion EMT completada. Resultados en logs/emt_rlc.csv")
