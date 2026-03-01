"""
Data Ingestion Layer for Panama Power Grid Dashboard.

Scrapes real-time and historical data from:
- CND (Centro Nacional de Despacho) - sitr.cnd.com.pa
- EOR (Ente Operador Regional) - enteoperador.org
- Simulates realistic data when sources are unavailable
"""

import asyncio
import random
import math
from datetime import datetime, timedelta
from typing import Optional

import numpy as np


class RealtimeDataSimulator:
    """
    Simulates realistic real-time power system data for Panama's SIN.

    When actual CND/EOR data is unavailable, this generates physically
    consistent data based on typical Panama grid operating conditions.
    """

    def __init__(self):
        self.base_time = datetime.now()
        self.hour_offset = 0

        # Typical Panama daily load curve (% of peak, hourly)
        # Based on CND post-dispatch data patterns
        self.daily_load_profile = [
            0.58, 0.54, 0.52, 0.51, 0.52, 0.56,  # 00:00-05:00
            0.62, 0.72, 0.82, 0.88, 0.92, 0.95,  # 06:00-11:00
            0.93, 0.96, 0.98, 1.00, 0.97, 0.94,  # 12:00-17:00
            0.92, 0.90, 0.85, 0.78, 0.70, 0.62,  # 18:00-23:00
        ]

        # Wind generation profile (typical for Penonomé area)
        self.wind_profile = [
            0.35, 0.38, 0.40, 0.42, 0.45, 0.48,
            0.50, 0.45, 0.35, 0.28, 0.22, 0.18,
            0.15, 0.12, 0.10, 0.12, 0.18, 0.25,
            0.32, 0.38, 0.42, 0.40, 0.38, 0.36,
        ]

        # Solar generation profile
        self.solar_profile = [
            0.00, 0.00, 0.00, 0.00, 0.00, 0.02,
            0.10, 0.30, 0.55, 0.75, 0.88, 0.95,
            1.00, 0.95, 0.85, 0.68, 0.45, 0.15,
            0.02, 0.00, 0.00, 0.00, 0.00, 0.00,
        ]

        # Monthly hydro availability factor (Panama rainy season June-Nov)
        self.monthly_hydro_factor = [
            0.60, 0.55, 0.50, 0.55, 0.65, 0.80,  # Jan-Jun
            0.90, 0.95, 1.00, 0.95, 0.85, 0.70,  # Jul-Dec
        ]

        # MER regional prices (USD/MWh) typical range by hour
        self.mer_price_base = [
            35, 32, 30, 29, 30, 33,
            38, 45, 52, 58, 62, 65,
            63, 65, 68, 70, 65, 58,
            55, 50, 45, 42, 40, 37,
        ]

    def get_current_conditions(self):
        """Get simulated current system conditions."""
        now = datetime.now()
        hour = now.hour
        minute = now.minute
        month = now.month - 1  # 0-indexed

        # Interpolate for sub-hourly resolution
        frac = minute / 60.0
        load_factor = self._interpolate(self.daily_load_profile, hour, frac)
        wind_factor = self._interpolate(self.wind_profile, hour, frac)
        solar_factor = self._interpolate(self.solar_profile, hour, frac)
        hydro_factor = self.monthly_hydro_factor[month]

        # Add realistic noise
        load_factor *= (1 + random.gauss(0, 0.02))
        wind_factor *= (1 + random.gauss(0, 0.08))
        solar_factor *= max(0, 1 + random.gauss(0, 0.05))

        # System peak demand ~ 2,300 MW for Panama
        system_peak_mw = 2300
        current_demand_mw = system_peak_mw * load_factor

        return {
            "timestamp": now.isoformat(),
            "hour": hour,
            "minute": minute,
            "load_factor": round(load_factor, 4),
            "wind_factor": round(wind_factor, 4),
            "solar_factor": round(solar_factor, 4),
            "hydro_factor": round(hydro_factor, 4),
            "current_demand_mw": round(current_demand_mw, 1),
            "system_peak_mw": system_peak_mw,
        }

    def get_generation_data(self):
        """Get simulated real-time generation by plant/fuel type."""
        conditions = self.get_current_conditions()

        # Base capacities
        plants = [
            {"name": "C.H. Fortuna", "fuel": "Hydro", "capacity": 300,
             "factor": conditions["hydro_factor"] * random.uniform(0.7, 0.95)},
            {"name": "C.H. Bayano", "fuel": "Hydro", "capacity": 260,
             "factor": conditions["hydro_factor"] * random.uniform(0.5, 0.9)},
            {"name": "C.H. Changuinola", "fuel": "Hydro", "capacity": 223,
             "factor": conditions["hydro_factor"] * random.uniform(0.6, 0.95)},
            {"name": "C.H. Estí", "fuel": "Hydro", "capacity": 120,
             "factor": conditions["hydro_factor"] * random.uniform(0.5, 0.9)},
            {"name": "C.H. Caldera", "fuel": "Hydro", "capacity": 60,
             "factor": conditions["hydro_factor"] * random.uniform(0.4, 0.85)},
            {"name": "C.H. Los Valles", "fuel": "Hydro", "capacity": 54,
             "factor": conditions["hydro_factor"] * random.uniform(0.4, 0.85)},
            {"name": "C.H. Bonyic", "fuel": "Hydro", "capacity": 31,
             "factor": conditions["hydro_factor"] * random.uniform(0.5, 0.9)},
            {"name": "C.T. Costa Norte", "fuel": "Natural Gas", "capacity": 380,
             "factor": random.uniform(0.6, 0.92)},
            {"name": "C.T. BLM", "fuel": "Coal/Pet Coke", "capacity": 180,
             "factor": random.uniform(0.5, 0.85)},
            {"name": "C.T. Pacora", "fuel": "Natural Gas", "capacity": 55,
             "factor": random.uniform(0.3, 0.8)},
            {"name": "P.E. Penonomé", "fuel": "Wind", "capacity": 215,
             "factor": conditions["wind_factor"]},
            {"name": "P.S. Coclé Solar", "fuel": "Solar", "capacity": 100,
             "factor": conditions["solar_factor"]},
            {"name": "P.S. Chiriquí Solar", "fuel": "Solar", "capacity": 50,
             "factor": conditions["solar_factor"]},
        ]

        total_gen = 0
        generation = []
        for plant in plants:
            output = round(plant["capacity"] * plant["factor"], 1)
            total_gen += output
            generation.append({
                "name": plant["name"],
                "fuel": plant["fuel"],
                "capacity_mw": plant["capacity"],
                "output_mw": output,
                "utilization_pct": round(output / plant["capacity"] * 100, 1),
            })

        # SIEPAC exchange
        demand = conditions["current_demand_mw"]
        siepac_flow = round(demand - total_gen, 1)  # Positive = import
        siepac_flow = max(-100, min(100, siepac_flow))  # Limit to realistic range

        return {
            "timestamp": conditions["timestamp"],
            "generation": generation,
            "total_generation_mw": round(total_gen, 1),
            "total_demand_mw": conditions["current_demand_mw"],
            "siepac_flow_mw": siepac_flow,
            "siepac_direction": "Import" if siepac_flow > 0 else "Export",
            "frequency_hz": round(60.0 + random.gauss(0, 0.015), 3),
        }

    def get_market_data(self):
        """Get simulated electricity market data."""
        now = datetime.now()
        hour = now.hour
        frac = now.minute / 60.0

        spot_price = self._interpolate(self.mer_price_base, hour, frac)
        spot_price *= (1 + random.gauss(0, 0.05))

        # Regional MER prices
        mer_prices = {
            "Panama": round(spot_price, 2),
            "Costa Rica": round(spot_price * random.uniform(0.85, 1.15), 2),
            "Nicaragua": round(spot_price * random.uniform(0.90, 1.20), 2),
            "Honduras": round(spot_price * random.uniform(0.95, 1.25), 2),
            "El Salvador": round(spot_price * random.uniform(0.88, 1.18), 2),
            "Guatemala": round(spot_price * random.uniform(0.82, 1.12), 2),
        }

        # Determine optimal trade direction
        panama_price = mer_prices["Panama"]
        cr_price = mer_prices["Costa Rica"]
        if panama_price > cr_price:
            trade_recommendation = "Import from Costa Rica"
            potential_saving = round((panama_price - cr_price) * 100, 2)  # for 100MW
        else:
            trade_recommendation = "Export to Costa Rica"
            potential_saving = round((cr_price - panama_price) * 100, 2)

        return {
            "timestamp": now.isoformat(),
            "spot_price_usd_mwh": round(spot_price, 2),
            "mer_nodal_prices": mer_prices,
            "trade_recommendation": trade_recommendation,
            "potential_saving_usd_h": potential_saving,
            "daily_avg_price_usd_mwh": round(sum(self.mer_price_base) / 24, 2),
        }

    def get_bus_voltages(self, num_buses=27):
        """Simulate voltage magnitudes at all buses."""
        now = datetime.now()
        conditions = self.get_current_conditions()

        voltages = []
        for i in range(1, num_buses + 1):
            # Base voltage with load-dependent sag
            base_v = 1.0 + random.gauss(0, 0.008)
            load_effect = (conditions["load_factor"] - 0.75) * 0.03
            v = base_v - load_effect
            v = max(0.92, min(1.08, v))

            voltages.append({
                "bus_id": i,
                "v_mag_pu": round(v, 4),
                "v_kv": round(v * 230, 2),
            })

        return {
            "timestamp": now.isoformat(),
            "voltages": voltages,
        }

    def get_historical_load(self, hours=24):
        """Generate historical load data for the last N hours."""
        now = datetime.now()
        data = []

        for i in range(hours * 4):  # 15-minute intervals
            t = now - timedelta(minutes=15 * (hours * 4 - i))
            hour = t.hour
            frac = t.minute / 60.0
            load_f = self._interpolate(self.daily_load_profile, hour, frac)
            load_f *= (1 + random.gauss(0, 0.015))

            data.append({
                "timestamp": t.isoformat(),
                "demand_mw": round(2300 * load_f, 1),
            })

        return data

    def _interpolate(self, profile, hour, frac):
        """Linear interpolation between hourly values."""
        next_hour = (hour + 1) % 24
        return profile[hour] * (1 - frac) + profile[next_hour] * frac


class CNDScraper:
    """
    Scraper for CND (Centro Nacional de Despacho) data.

    Target URLs:
    - Real-time generation: https://sitr.cnd.com.pa/m/pub/gen.html
    - Reports: https://www.cnd.com.pa/index.php/informes/categoria/informes-de-operaciones
    """

    def __init__(self):
        self.base_url = "https://www.cnd.com.pa"
        self.sitr_url = "https://sitr.cnd.com.pa/m/pub/gen.html"

    async def fetch_realtime_generation(self):
        """
        Attempt to fetch real-time generation data from SITR.

        Note: This requires network access and may be blocked by CORS/firewalls.
        Falls back to simulation if unavailable.
        """
        try:
            import httpx
            async with httpx.AsyncClient(timeout=10.0) as client:
                response = await client.get(self.sitr_url)
                if response.status_code == 200:
                    return self._parse_sitr_html(response.text)
        except Exception:
            pass

        return None

    def _parse_sitr_html(self, html):
        """Parse SITR HTML page for generation data."""
        try:
            from bs4 import BeautifulSoup
            soup = BeautifulSoup(html, "html.parser")

            # Extract generation table data
            # Structure varies - this is a best-effort parser
            tables = soup.find_all("table")
            if tables:
                data = []
                for table in tables:
                    rows = table.find_all("tr")
                    for row in rows:
                        cells = row.find_all(["td", "th"])
                        if len(cells) >= 2:
                            data.append({
                                "plant": cells[0].get_text(strip=True),
                                "output_mw": cells[1].get_text(strip=True),
                            })
                return data
        except Exception:
            pass
        return None


class EORScraper:
    """
    Scraper for EOR (Ente Operador Regional) data.

    Target URLs:
    - Nodal prices: https://info.enteoperador.org/PreciosNodalesMER/ConsultaPreciosNodales.php
    - Transaction reports: https://www.enteoperador.org/mer/gestion-comercial/
    """

    def __init__(self):
        self.base_url = "https://www.enteoperador.org"
        self.nodal_prices_url = "https://info.enteoperador.org/PreciosNodalesMER/ConsultaPreciosNodales.php"

    async def fetch_nodal_prices(self):
        """Attempt to fetch MER nodal prices."""
        try:
            import httpx
            async with httpx.AsyncClient(timeout=10.0) as client:
                response = await client.get(self.nodal_prices_url)
                if response.status_code == 200:
                    return self._parse_nodal_prices(response.text)
        except Exception:
            pass
        return None

    def _parse_nodal_prices(self, html):
        """Parse EOR nodal prices page."""
        try:
            from bs4 import BeautifulSoup
            soup = BeautifulSoup(html, "html.parser")
            # Best-effort parser for nodal price tables
            tables = soup.find_all("table")
            if tables:
                prices = []
                for table in tables:
                    rows = table.find_all("tr")
                    for row in rows[1:]:  # Skip header
                        cells = row.find_all("td")
                        if len(cells) >= 3:
                            prices.append({
                                "node": cells[0].get_text(strip=True),
                                "date": cells[1].get_text(strip=True),
                                "price": cells[2].get_text(strip=True),
                            })
                return prices
        except Exception:
            pass
        return None


# Singleton simulator instance
_simulator = RealtimeDataSimulator()
_cnd_scraper = CNDScraper()
_eor_scraper = EORScraper()


def get_simulator():
    return _simulator


def get_cnd_scraper():
    return _cnd_scraper


def get_eor_scraper():
    return _eor_scraper
