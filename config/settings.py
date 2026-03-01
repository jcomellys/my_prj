"""
Configuration settings for Panama Power Grid Dashboard.
"""

# CND / EOR data source URLs
CND_BASE_URL = "https://www.cnd.com.pa"
CND_SITR_URL = "https://sitr.cnd.com.pa/m/pub/gen.html"
EOR_BASE_URL = "https://www.enteoperador.org"
EOR_NODAL_PRICES_URL = "https://info.enteoperador.org/PreciosNodalesMER/ConsultaPreciosNodales.php"

# System base values
BASE_MVA = 100.0  # System base power in MVA
BASE_KV = 230.0   # System base voltage in kV
BASE_IMPEDANCE = (BASE_KV ** 2) / BASE_MVA  # Ohms

# Power flow solver settings
PF_MAX_ITERATIONS = 50
PF_TOLERANCE = 1e-6

# Real-time update interval (seconds)
RT_UPDATE_INTERVAL = 5

# Server settings
API_HOST = "0.0.0.0"
API_PORT = 8000

# Stability analysis thresholds
VOLTAGE_MIN_PU = 0.95
VOLTAGE_MAX_PU = 1.05
LOADING_WARNING_PCT = 80.0
LOADING_CRITICAL_PCT = 100.0
FREQUENCY_NOMINAL_HZ = 60.0
FREQUENCY_TOLERANCE_HZ = 0.5
