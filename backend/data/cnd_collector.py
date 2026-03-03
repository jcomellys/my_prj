"""
CND Real-Time Generation Collector.

Fetches and parses real-time generation data from:
  https://sitr.cnd.com.pa/m/pub/gen.html
  https://sitr.cnd.com.pa/m/pub/flow.html  (Flujo Occidente)

Architecture:
  [CND / SITR]  -->  [This collector, every 30-60s]  -->  [SQLite cache]
  Your app reads from the cache, never hitting CND directly on each request.

Source attribution: Data from ETESA/CND (sitr.cnd.com.pa).
"""

import re
import os
import logging
import asyncio
from datetime import datetime
from typing import Optional

import httpx
from bs4 import BeautifulSoup

logger = logging.getLogger(__name__)

GEN_URL = "https://sitr.cnd.com.pa/m/pub/gen.html"
FLOW_URL = "https://sitr.cnd.com.pa/m/pub/flow.html"

# Physical validation: Panama SIN total generation should be within this range
MIN_REASONABLE_MW = 300
MAX_REASONABLE_MW = 4000

# Map section headers from the HTML to internal keys (case-insensitive variants)
SECTION_MAP = {
    "Hidroeléctricas (MW)": "hidro",
    "Hidroelectricas (MW)": "hidro",
    "Térmicas (MW)": "termica",
    "Termicas (MW)": "termica",
    "Solares (MW)": "solar",
    "Eólicas (MW)": "eolica",
    "Eolicas (MW)": "eolica",
}

# Lowercase variants for fuzzy matching
SECTION_MAP_LOWER = {k.lower(): v for k, v in SECTION_MAP.items()}

# Headers that signal the end of a section
SECTION_TERMINATORS = {"Total", "Subtotal", "TOTAL", "Total SIN"}

# Persistent session headers
REQUEST_HEADERS = {
    "User-Agent": (
        "Mozilla/5.0 (Windows NT 10.0; Win64; x64) "
        "AppleWebKit/537.36 (KHTML, like Gecko) "
        "Chrome/122.0.0.0 Safari/537.36"
    ),
    "Accept": "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
    "Accept-Language": "es-PA,es;q=0.9,en;q=0.8",
    "Referer": "https://sitr.cnd.com.pa/m/",
    "Connection": "keep-alive",
}

# Known plant capacities (MW) for enriching CND data
KNOWN_CAPACITIES = {
    # Hydro
    "fortuna": 300, "fortuna 1": 100, "fortuna 2": 100, "fortuna 3": 100,
    "bayano": 260, "bayano 1": 130, "bayano 2": 130,
    "changuinola": 221, "chan 75": 221, "chan-75": 221,
    "estí": 120, "esti": 120, "el alto": 72,
    "caldera": 60, "bajo mina": 57, "los valles": 54,
    "monte lirio": 52, "la estrella": 48, "bonyic": 32,
    "lorena": 34, "pando": 33, "gualaca": 32,
    "barro blanco": 29, "algarrobos": 10,
    "gatún": 36, "madden": 36, "miraflores": 10,
    "proyecto gatún 1": 18, "proyecto gatún 2": 18,
    # Thermal
    "costa norte": 381, "costa norte 1": 190, "costa norte 2": 191,
    "termo colón": 150, "termocolon": 150, "gena": 150,
    "generadora gatún": 72, "cobre panamá": 300, "cobre panama": 300,
    "cobre panamá 1": 150, "cobre panama 1": 150,
    "cobre panamá 2": 150, "cobre panama 2": 150,
    "blm": 120, "bahía las minas": 120,
    "pan-am": 60, "pacora": 28, "pedregal": 27,
    # Wind
    "penonomé": 215, "penonome": 215, "toabré": 66, "toabre": 66,
    # Solar
    "antón solar": 10, "anton solar": 10,
}

DUMP_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "dumps")


def _clean_lines(text: str) -> list[str]:
    """Split text into non-empty stripped lines."""
    return [line.strip() for line in text.splitlines() if line.strip()]


def _parse_timestamp(lines: list[str]) -> Optional[str]:
    """Extract the CND timestamp. Format: 02-marzo-2026 14:06:32"""
    pattern = r"\d{2}-[A-Za-záéíóúñÁÉÍÓÚÑ]+-\d{4}\s+\d{2}:\d{2}:\d{2}"
    for line in lines:
        match = re.search(pattern, line)
        if match:
            return match.group(0)
    return None


def _parse_numeric(text: str) -> Optional[float]:
    """Try to parse a numeric value, handling commas and whitespace."""
    text = text.strip().replace(",", ".")
    # Remove any non-numeric chars except . and -
    cleaned = re.sub(r"[^\d.\-]", "", text)
    try:
        return float(cleaned) if cleaned else None
    except (ValueError, TypeError):
        return None


def _lookup_capacity(name: str) -> int:
    """Look up known capacity for a plant name."""
    lower = name.lower().strip()
    # Direct match
    if lower in KNOWN_CAPACITIES:
        return KNOWN_CAPACITIES[lower]
    # Partial match
    for key, cap in KNOWN_CAPACITIES.items():
        if key in lower or lower in key:
            return cap
    return 0


def _classify_fuel(name: str) -> Optional[str]:
    """Try to classify fuel type from plant name alone."""
    lower = name.lower()
    hydro_keywords = ["fortuna", "bayano", "chan", "estí", "esti", "caldera",
                      "mina", "valles", "lirio", "estrella", "bonyic", "lorena",
                      "pando", "gualaca", "barro", "algarrobos", "gatún", "gatun",
                      "madden", "miraflores", "hidroel", "c.h."]
    thermal_keywords = ["costa norte", "termo", "gena", "cobre", "blm", "bahía",
                        "pan-am", "pacora", "pedregal", "diesel", "gas", "c.t.",
                        "térmico", "termico"]
    solar_keywords = ["solar", "fotovolt", "p.s."]
    wind_keywords = ["eólico", "eolico", "penonomé", "penonome", "toabré",
                     "toabre", "viento", "p.e."]

    for kw in hydro_keywords:
        if kw in lower:
            return "hidro"
    for kw in thermal_keywords:
        if kw in lower:
            return "termica"
    for kw in solar_keywords:
        if kw in lower:
            return "solar"
    for kw in wind_keywords:
        if kw in lower:
            return "eolica"
    return None


def is_reasonable_total(total_mw: float) -> bool:
    """Validate that the total generation falls within physically plausible range."""
    return MIN_REASONABLE_MW <= total_mw <= MAX_REASONABLE_MW


def _dump_html(html: str, label: str):
    """Save raw HTML to disk for post-mortem diagnostics."""
    try:
        os.makedirs(DUMP_DIR, exist_ok=True)
        ts = datetime.now().strftime("%Y%m%d_%H%M%S")
        path = os.path.join(DUMP_DIR, f"{label}_{ts}.html")
        with open(path, "w", encoding="utf-8") as f:
            f.write(html)
        logger.info("Dumped raw HTML to %s (%d bytes)", path, len(html))
        dumps = sorted(
            [os.path.join(DUMP_DIR, f) for f in os.listdir(DUMP_DIR)
             if f.startswith(label)],
        )
        for old in dumps[:-20]:
            os.remove(old)
    except Exception:
        logger.warning("Failed to dump HTML for diagnostics", exc_info=True)


def parse_generation_html(html: str) -> dict:
    """
    Parse the SITR gen.html page and extract generation data.

    Uses multiple strategies in order:
    1. All <table> elements — extract name/value pairs from rows
    2. Section-header-based text parsing
    3. Global regex scan for "name  number" patterns
    """
    soup = BeautifulSoup(html, "html.parser")
    text = soup.get_text("\n", strip=True)
    lines = _clean_lines(text)

    data = {
        "timestamp": _parse_timestamp(lines),
        "source": "CND/SITR",
        "hidro": {},
        "termica": {},
        "solar": {},
        "eolica": {},
        "totals": {},
        "total_mw": 0.0,
        "parse_errors": [],
        "_debug_line_count": len(lines),
        "_debug_html_bytes": len(html),
    }

    # ── STRATEGY 1: Extract ALL table rows from ALL tables ──
    all_table_pairs = []
    for table in soup.find_all("table"):
        for row in table.find_all("tr"):
            cells = row.find_all(["td", "th"])
            if len(cells) >= 2:
                name = cells[0].get_text(strip=True)
                val_text = cells[-1].get_text(strip=True)
                val = _parse_numeric(val_text)
                if name and val is not None and len(name) > 1:
                    if not any(name.startswith(t) for t in SECTION_TERMINATORS):
                        all_table_pairs.append((name, val))

    # ── STRATEGY 2: Section-header-based text parsing ──
    current_section = None
    text_pairs_by_section = {"hidro": {}, "termica": {}, "solar": {}, "eolica": {}}

    for line in lines:
        # Check for section header (exact or fuzzy)
        if line in SECTION_MAP:
            current_section = SECTION_MAP[line]
            continue
        line_lower = line.lower()
        if line_lower in SECTION_MAP_LOWER:
            current_section = SECTION_MAP_LOWER[line_lower]
            continue
        # Fuzzy section detection
        if "hidroel" in line_lower and "(mw)" in line_lower:
            current_section = "hidro"
            continue
        if ("térmica" in line_lower or "termica" in line_lower) and "(mw)" in line_lower:
            current_section = "termica"
            continue
        if "solar" in line_lower and "(mw)" in line_lower:
            current_section = "solar"
            continue
        if ("eólica" in line_lower or "eolica" in line_lower) and "(mw)" in line_lower:
            current_section = "eolica"
            continue

        # Check for section terminators
        if any(line.startswith(term) for term in SECTION_TERMINATORS):
            m = re.match(
                r"^(?:Total|Subtotal|TOTAL|Total SIN)\s+(-?\d+(?:[.,]\d+)?)\s*$",
                line,
            )
            if m and current_section:
                val = _parse_numeric(m.group(1))
                if val is not None:
                    data["totals"][current_section] = val
            current_section = None
            continue

        # Extract name/value pairs from text lines
        if current_section:
            # Various patterns: "Name  123.4" or "Name 123,4" or tab-separated
            m = re.match(r"^(.+?)\s{2,}(-?\d+(?:[.,]\d+)?)\s*$", line)
            if not m:
                m = re.match(r"^(.+?)\t+(-?\d+(?:[.,]\d+)?)\s*$", line)
            if not m:
                m = re.match(r"^(.*?)\s+(-?\d+(?:[.,]\d+)?)$", line)
            if m:
                name = m.group(1).strip()
                val = _parse_numeric(m.group(2))
                if name and val is not None and len(name) > 1:
                    text_pairs_by_section[current_section][name] = val

    # ── STRATEGY 3: Classify table pairs by fuel type ──
    # First, use section-based text data
    for section in ("hidro", "termica", "solar", "eolica"):
        if text_pairs_by_section[section]:
            data[section] = text_pairs_by_section[section]

    # Then, enrich with table pairs (classify by name if no section)
    for name, val in all_table_pairs:
        # Skip if this is a known section header or metadata
        if any(kw in name.lower() for kw in ["(mw)", "total", "subtotal", "sitr",
                                               "cnd", "etesa", "seguimiento"]):
            continue

        # Check if already captured in a section
        already_found = False
        for section in ("hidro", "termica", "solar", "eolica"):
            if name in data[section]:
                already_found = True
                break
        if already_found:
            continue

        # Try to classify by plant name
        fuel = _classify_fuel(name)
        if fuel:
            data[fuel][name] = val
        else:
            # If we can't classify, check if it looks like a plant name
            # (not too short, not a date, not a header)
            if len(name) >= 3 and val > 0 and not re.match(r"^\d", name):
                # Default to termica if unknown
                data["termica"][name] = val
                data["parse_errors"].append(f"Unclassified plant: {name}={val}")

    # ── Compute totals ──
    for section in ("hidro", "termica", "solar", "eolica"):
        if section not in data["totals"] and data[section]:
            data["totals"][section] = round(sum(data[section].values()), 2)

    data["total_mw"] = round(
        sum(data["totals"].get(s, 0) for s in ("hidro", "termica", "solar", "eolica")),
        2,
    )

    return data


def parse_flow_html(html: str) -> dict:
    """Parse the SITR flow.html page for Flujo Occidente data."""
    soup = BeautifulSoup(html, "html.parser")
    text = soup.get_text("\n", strip=True)
    lines = _clean_lines(text)

    data = {
        "timestamp": _parse_timestamp(lines),
        "source": "CND/SITR",
        "flujo_occidente_mw": None,
        "limite_mw": None,
        "carga_pct": None,
        "parse_errors": [],
    }

    for line in lines:
        lower = line.lower()
        if "flujo" in lower and "occidente" in lower:
            m = re.search(r"(-?\d+(?:[.,]\d+)?)", line)
            if m:
                data["flujo_occidente_mw"] = _parse_numeric(m.group(1))
        if "limite" in lower or "límite" in lower:
            m = re.search(r"(-?\d+(?:[.,]\d+)?)", line)
            if m:
                data["limite_mw"] = _parse_numeric(m.group(1))

    for table in soup.find_all("table"):
        for row in table.find_all("tr"):
            cells = row.find_all(["td", "th"])
            if len(cells) >= 2:
                label = cells[0].get_text(strip=True).lower()
                val = _parse_numeric(cells[-1].get_text(strip=True))
                if val is not None:
                    if "flujo" in label:
                        data["flujo_occidente_mw"] = val
                    elif "limite" in label or "límite" in label:
                        data["limite_mw"] = val

    if data["flujo_occidente_mw"] is not None and data["limite_mw"] and data["limite_mw"] > 0:
        data["carga_pct"] = round(
            abs(data["flujo_occidente_mw"]) / data["limite_mw"] * 100, 1
        )

    return data


def summarize_generation(data: dict) -> dict:
    """Create a summary suitable for the dashboard API."""
    generation_list = []

    fuel_type_map = {
        "hidro": "Hydro",
        "termica": "Thermal",
        "solar": "Solar",
        "eolica": "Wind",
    }

    for section, fuel_label in fuel_type_map.items():
        for name, output_mw in data.get(section, {}).items():
            cap = _lookup_capacity(name)
            util = round(output_mw / cap * 100, 1) if cap > 0 and output_mw > 0 else 0
            generation_list.append({
                "name": name,
                "fuel": fuel_label,
                "capacity_mw": cap,
                "output_mw": round(output_mw, 1),
                "utilization_pct": util,
            })

    return {
        "timestamp": data.get("timestamp"),
        "source": "CND/SITR (sitr.cnd.com.pa)",
        "generation": generation_list,
        "total_generation_mw": data.get("total_mw", 0),
        "totals_by_fuel": data.get("totals", {}),
        "total_demand_mw": data.get("total_mw", 0),
        "siepac_flow_mw": 0,
        "siepac_direction": "N/A",
        "frequency_hz": 60.0,
    }


class CNDCollector:
    """
    Background collector that periodically fetches generation data from CND.
    """

    def __init__(self, interval_seconds: int = 60):
        self.interval = interval_seconds
        self._latest_raw: Optional[dict] = None
        self._latest_summary: Optional[dict] = None
        self._latest_flow: Optional[dict] = None
        self._last_fetch: Optional[datetime] = None
        self._last_live_fetch: Optional[datetime] = None
        self._last_error: Optional[str] = None
        self._last_error_time: Optional[datetime] = None
        self._last_http_status: Optional[int] = None
        self._last_html: Optional[str] = None
        self._running = False
        self._fetch_count = 0
        self._error_count = 0
        self._consecutive_errors = 0
        self._validation_failures = 0
        self._client: Optional[httpx.AsyncClient] = None

    def _get_client(self) -> httpx.AsyncClient:
        """Get or create a persistent HTTP client."""
        if self._client is None or self._client.is_closed:
            self._client = httpx.AsyncClient(
                timeout=httpx.Timeout(20.0, connect=10.0),
                headers=REQUEST_HEADERS,
                follow_redirects=True,
                http2=False,
            )
        return self._client

    async def start(self):
        self._running = True
        logger.info(
            "CND Collector started (interval=%ds, gen_url=%s)",
            self.interval, GEN_URL,
        )
        while self._running:
            await self._fetch_once()
            await asyncio.sleep(self.interval)

    def stop(self):
        self._running = False
        logger.info("CND Collector stopped")

    async def close(self):
        if self._client and not self._client.is_closed:
            await self._client.aclose()

    async def _fetch_with_retries(self, url: str, max_retries: int = 3) -> httpx.Response:
        """Fetch a URL with exponential backoff retries."""
        client = self._get_client()
        last_exc = None
        for attempt in range(max_retries + 1):
            try:
                response = await client.get(url)
                self._last_http_status = response.status_code
                response.raise_for_status()
                return response
            except (httpx.HTTPStatusError, httpx.RequestError) as e:
                last_exc = e
                if isinstance(e, httpx.HTTPStatusError):
                    self._last_http_status = e.response.status_code
                if attempt < max_retries:
                    wait = 2 ** attempt
                    logger.warning(
                        "CND fetch attempt %d/%d failed for %s: %s — retrying in %ds",
                        attempt + 1, max_retries + 1, url, e, wait,
                    )
                    await asyncio.sleep(wait)
                    if isinstance(e, httpx.RequestError):
                        try:
                            await client.aclose()
                        except Exception:
                            pass
                        self._client = None
                        client = self._get_client()
        raise last_exc

    async def _fetch_once(self):
        """Fetch and parse the CND generation page once."""
        html = None
        try:
            response = await self._fetch_with_retries(GEN_URL)
            html = response.text
            self._last_html = html

            raw = parse_generation_html(html)
            plant_count = sum(
                len(raw[s]) for s in ("hidro", "termica", "solar", "eolica")
            )

            # Physical validation
            if not is_reasonable_total(raw["total_mw"]):
                self._validation_failures += 1
                self._last_error = (
                    f"VALIDATION: total_mw={raw['total_mw']:.1f} outside "
                    f"[{MIN_REASONABLE_MW}, {MAX_REASONABLE_MW}] range "
                    f"(found {plant_count} plants, {len(html)} bytes HTML)"
                )
                self._last_error_time = datetime.now()
                logger.warning("CND data rejected: %s", self._last_error)
                _dump_html(html, "validation_fail")

                # STILL store the raw parse result (marked as invalid)
                # so the debug endpoint can show what was parsed
                raw["_validation_failed"] = True
                self._latest_raw = raw
                self._latest_summary = summarize_generation(raw)
                self._latest_summary["_validation_failed"] = True
                return

            summary = summarize_generation(raw)

            self._latest_raw = raw
            self._latest_summary = summary
            self._last_fetch = datetime.now()
            self._last_live_fetch = datetime.now()
            self._last_error = None
            self._last_error_time = None
            self._last_http_status = 200
            self._fetch_count += 1
            self._consecutive_errors = 0

            logger.info(
                "CND fetch #%d OK: %d plants, %.1f MW total, ts=%s",
                self._fetch_count, plant_count,
                raw["total_mw"], raw["timestamp"],
            )

        except httpx.HTTPStatusError as e:
            self._last_error = f"HTTP {e.response.status_code}: {e.request.url}"
            self._last_error_time = datetime.now()
            self._error_count += 1
            self._consecutive_errors += 1
            logger.warning("CND fetch failed: %s", self._last_error)
            if html:
                _dump_html(html, "http_error")

        except httpx.RequestError as e:
            self._last_error = f"{type(e).__name__}: {e}"
            self._last_error_time = datetime.now()
            self._error_count += 1
            self._consecutive_errors += 1
            logger.warning("CND fetch error: %s", self._last_error)

        except Exception as e:
            self._last_error = f"{type(e).__name__}: {e}"
            self._last_error_time = datetime.now()
            self._error_count += 1
            self._consecutive_errors += 1
            logger.exception("CND fetch unexpected error")
            if html:
                _dump_html(html, "parse_error")

        # Also fetch flow data (best-effort)
        await self._fetch_flow()

    async def _fetch_flow(self):
        try:
            response = await self._fetch_with_retries(FLOW_URL, max_retries=1)
            self._latest_flow = parse_flow_html(response.text)
        except Exception as e:
            logger.debug("Flow fetch failed (non-critical): %s", e)

    def get_latest_raw(self) -> Optional[dict]:
        raw = self._latest_raw
        if raw and raw.get("_validation_failed"):
            return None  # Don't serve invalid data as "live"
        return raw

    def get_latest(self) -> Optional[dict]:
        summary = self._latest_summary
        if summary and summary.get("_validation_failed"):
            return None
        return summary

    def get_latest_flow(self) -> Optional[dict]:
        return self._latest_flow

    def get_debug_info(self) -> dict:
        """Full diagnostic info: raw HTML, parse results, everything."""
        raw = self._latest_raw or {}
        return {
            "status": self.get_status(),
            "last_html_bytes": len(self._last_html) if self._last_html else 0,
            "last_html_preview": self._last_html[:5000] if self._last_html else None,
            "last_html_text_lines": _clean_lines(
                BeautifulSoup(self._last_html, "html.parser").get_text("\n", strip=True)
            )[:100] if self._last_html else [],
            "parse_result": raw,
            "plant_counts": {
                s: len(raw.get(s, {}))
                for s in ("hidro", "termica", "solar", "eolica")
            },
            "total_mw": raw.get("total_mw", 0),
            "validation_passed": not raw.get("_validation_failed", False),
        }

    def get_status(self) -> dict:
        return {
            "running": self._running,
            "last_fetch": self._last_fetch.isoformat() if self._last_fetch else None,
            "last_live_fetch": self._last_live_fetch.isoformat() if self._last_live_fetch else None,
            "last_error": self._last_error,
            "last_error_time": self._last_error_time.isoformat() if self._last_error_time else None,
            "last_http_status": self._last_http_status,
            "fetch_count": self._fetch_count,
            "error_count": self._error_count,
            "consecutive_errors": self._consecutive_errors,
            "validation_failures": self._validation_failures,
            "has_data": self.get_latest_raw() is not None,
            "has_flow_data": self._latest_flow is not None,
            "source_url": GEN_URL,
            "flow_url": FLOW_URL,
            "interval_seconds": self.interval,
        }


# Module-level singleton
_collector: Optional[CNDCollector] = None


def get_collector() -> CNDCollector:
    global _collector
    if _collector is None:
        _collector = CNDCollector(interval_seconds=60)
    return _collector
