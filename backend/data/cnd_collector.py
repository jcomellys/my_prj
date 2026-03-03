"""
CND Real-Time Generation Collector.

Fetches and parses real-time generation data from:
  https://sitr.cnd.com.pa/m/pub/gen.html
  https://sitr.cnd.com.pa/m/pub/flow.html  (Flujo Occidente)

The page publishes generation data as text within HTML, organized by
sections (Hidroeléctricas, Térmicas, Solares, Eólicas) with individual
unit-level detail (e.g. "Fortuna 1", "Fortuna 2", "Fortuna 3").

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

# Map section headers from the HTML to internal keys
SECTION_MAP = {
    "Hidroeléctricas (MW)": "hidro",
    "Hidroelectricas (MW)": "hidro",
    "Térmicas (MW)": "termica",
    "Termicas (MW)": "termica",
    "Solares (MW)": "solar",
    "Eólicas (MW)": "eolica",
    "Eolicas (MW)": "eolica",
}

# Headers that signal the end of a section (totals, subtotals, etc.)
SECTION_TERMINATORS = {"Total", "Subtotal", "TOTAL", "Total SIN"}

# Persistent session headers — mimic a real browser visiting the SITR
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

# Directory for raw HTML dumps on failures
DUMP_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "dumps")


def _clean_lines(text: str) -> list[str]:
    """Split text into non-empty stripped lines."""
    return [line.strip() for line in text.splitlines() if line.strip()]


def _parse_timestamp(lines: list[str]) -> Optional[str]:
    """
    Extract the CND timestamp from the page text.
    Expected format: 02-marzo-2026 14:06:32
    """
    pattern = r"\d{2}-[A-Za-záéíóúñÁÉÍÓÚÑ]+-\d{4}\s+\d{2}:\d{2}:\d{2}"
    for line in lines:
        match = re.match(pattern, line)
        if match:
            return match.group(0)
    return None


def _parse_numeric(text: str) -> Optional[float]:
    """Try to parse a numeric value, handling commas and whitespace."""
    text = text.strip().replace(",", ".")
    try:
        return float(text)
    except (ValueError, TypeError):
        return None


def _extract_from_tables(soup, section_key: str, heading_texts: list[str]) -> dict:
    """
    Extract plant/value pairs from HTML tables that follow a section heading.

    Handles both:
    - <table> with <tr><td>name</td><td>value</td></tr> rows
    - Plain text lines "PlantName  value"
    """
    plants = {}

    # Strategy 1: Find tables near section headings
    for element in soup.find_all(string=re.compile("|".join(re.escape(h) for h in heading_texts))):
        # Walk forward from the heading to find the next table
        parent = element.find_parent()
        sibling = parent.find_next_sibling() if parent else None
        while sibling:
            if sibling.name == "table":
                for row in sibling.find_all("tr"):
                    cells = row.find_all(["td", "th"])
                    if len(cells) >= 2:
                        name = cells[0].get_text(strip=True)
                        val = _parse_numeric(cells[-1].get_text(strip=True))
                        if name and val is not None and not any(
                            name.startswith(t) for t in SECTION_TERMINATORS
                        ):
                            plants[name] = val
                break
            # Check if we hit another section heading
            sib_text = sibling.get_text(strip=True)
            if sib_text in SECTION_MAP:
                break
            sibling = sibling.find_next_sibling()

    return plants


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

        # Keep only the last 20 dumps to avoid disk bloat
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

    Uses two strategies:
    1. Direct HTML table parsing (more reliable for structured tables)
    2. Text-based line parsing (fallback for unstructured pages)

    Returns:
        {
            "timestamp": "02-marzo-2026 14:06:32",
            "source": "CND/SITR",
            "hidro": {"Fortuna 1": 95.2, "Fortuna 2": 88.1, ...},
            "termica": {"Costa Norte 1": 120.5, ...},
            "solar": {"Solar Park A": 45.0, ...},
            "eolica": {"Penonomé I": 33.2, ...},
            "totals": {"hidro": 850.3, "termica": 620.1, "solar": 180.5, "eolica": 78.9},
            "total_mw": 1729.8,
            "parse_errors": []
        }
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
    }

    # Strategy 1: Parse HTML tables directly
    section_headings = {
        "hidro": ["Hidroeléctricas (MW)", "Hidroelectricas (MW)"],
        "termica": ["Térmicas (MW)", "Termicas (MW)"],
        "solar": ["Solares (MW)"],
        "eolica": ["Eólicas (MW)", "Eolicas (MW)"],
    }

    for section, headings in section_headings.items():
        table_data = _extract_from_tables(soup, section, headings)
        if table_data:
            data[section] = table_data

    # Strategy 2: Text-based parsing (if tables didn't yield results)
    current_section = None
    for line in lines:
        # Check if this line is a section header
        if line in SECTION_MAP:
            current_section = SECTION_MAP[line]
            continue

        # Check for section terminators (total rows)
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

        # If we're in a section AND that section is still empty (table parse missed it),
        # try text-based extraction
        if current_section and not data[current_section]:
            # Pattern: plant name followed by spaces and a numeric value
            m = re.match(r"^(.+?)\s{2,}(-?\d+(?:[.,]\d+)?)\s*$", line)
            if not m:
                m = re.match(r"^(.*?)\s+(-?\d+(?:[.,]\d+)?)$", line)
            if m:
                name = m.group(1).strip()
                val = _parse_numeric(m.group(2))
                if name and val is not None:
                    data[current_section][name] = val

    # Compute totals if not provided by the page
    for section in ("hidro", "termica", "solar", "eolica"):
        if section not in data["totals"] and data[section]:
            data["totals"][section] = round(sum(data[section].values()), 2)

    data["total_mw"] = round(
        sum(data["totals"].get(s, 0) for s in ("hidro", "termica", "solar", "eolica")),
        2,
    )

    return data


def parse_flow_html(html: str) -> dict:
    """
    Parse the SITR flow.html page for Flujo Occidente data.

    Returns:
        {
            "timestamp": "...",
            "flujo_occidente_mw": 123.4,
            "limite_mw": 600.0,
            "carga_pct": 20.6,
            "parse_errors": [],
        }
    """
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
        # Look for "Flujo Occidente" value
        if "flujo" in lower and "occidente" in lower:
            m = re.search(r"(-?\d+(?:[.,]\d+)?)", line)
            if m:
                data["flujo_occidente_mw"] = _parse_numeric(m.group(1))
        # Look for "Limite" value
        if "limite" in lower or "límite" in lower:
            m = re.search(r"(-?\d+(?:[.,]\d+)?)", line)
            if m:
                data["limite_mw"] = _parse_numeric(m.group(1))

    # Also try table-based extraction
    for table in soup.find_all("table"):
        for row in table.find_all("tr"):
            cells = row.find_all(["td", "th"])
            if len(cells) >= 2:
                label = cells[0].get_text(strip=True).lower()
                val_text = cells[-1].get_text(strip=True)
                val = _parse_numeric(val_text)
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
    """
    Create a summary suitable for the dashboard API.

    Returns a structure compatible with the existing frontend expectations.
    """
    generation_list = []

    fuel_type_map = {
        "hidro": "Hydro",
        "termica": "Thermal",
        "solar": "Solar",
        "eolica": "Wind",
    }

    for section, fuel_label in fuel_type_map.items():
        for name, output_mw in data.get(section, {}).items():
            generation_list.append({
                "name": name,
                "fuel": fuel_label,
                "capacity_mw": 0,  # CND page doesn't report capacity
                "output_mw": round(output_mw, 1),
                "utilization_pct": 0,
            })

    return {
        "timestamp": data.get("timestamp"),
        "source": "CND/SITR (sitr.cnd.com.pa)",
        "generation": generation_list,
        "total_generation_mw": data.get("total_mw", 0),
        "totals_by_fuel": data.get("totals", {}),
        "total_demand_mw": data.get("total_mw", 0),  # Approx: gen ≈ demand
        "siepac_flow_mw": 0,
        "siepac_direction": "N/A",
        "frequency_hz": 60.0,
    }


class CNDCollector:
    """
    Background collector that periodically fetches generation data from CND.

    Uses a persistent httpx.AsyncClient with retry logic and proper session
    handling to maximize reliability against transient network issues.

    Usage:
        collector = CNDCollector(interval_seconds=60)
        asyncio.create_task(collector.start())

        # Later, from any endpoint:
        data = collector.get_latest()
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
        self._running = False
        self._fetch_count = 0
        self._error_count = 0
        self._consecutive_errors = 0
        self._validation_failures = 0
        self._client: Optional[httpx.AsyncClient] = None

    def _get_client(self) -> httpx.AsyncClient:
        """Get or create a persistent HTTP client (session reuse)."""
        if self._client is None or self._client.is_closed:
            self._client = httpx.AsyncClient(
                timeout=httpx.Timeout(20.0, connect=10.0),
                headers=REQUEST_HEADERS,
                follow_redirects=True,
                http2=False,
            )
        return self._client

    async def start(self):
        """Start the periodic collection loop."""
        self._running = True
        logger.info(
            "CND Collector started (interval=%ds, gen_url=%s, flow_url=%s)",
            self.interval, GEN_URL, FLOW_URL,
        )
        while self._running:
            await self._fetch_once()
            await asyncio.sleep(self.interval)

    def stop(self):
        """Stop the collection loop."""
        self._running = False
        logger.info("CND Collector stopped")

    async def close(self):
        """Close the HTTP client."""
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
                    wait = 2 ** attempt  # 1s, 2s, 4s
                    logger.warning(
                        "CND fetch attempt %d/%d failed for %s: %s — retrying in %ds",
                        attempt + 1, max_retries + 1, url, e, wait,
                    )
                    await asyncio.sleep(wait)
                    # Recreate client on persistent errors
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

            raw = parse_generation_html(html)

            # Physical validation: reject anomalous data
            if not is_reasonable_total(raw["total_mw"]):
                self._validation_failures += 1
                self._last_error = (
                    f"VALIDATION: total_mw={raw['total_mw']:.1f} outside "
                    f"[{MIN_REASONABLE_MW}, {MAX_REASONABLE_MW}] range"
                )
                self._last_error_time = datetime.now()
                logger.warning(
                    "CND data rejected: %s (plant_count=%d)",
                    self._last_error,
                    sum(len(raw[s]) for s in ("hidro", "termica", "solar", "eolica")),
                )
                _dump_html(html, "validation_fail")
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

            plant_count = sum(
                len(raw[s]) for s in ("hidro", "termica", "solar", "eolica")
            )
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

        # Also fetch flow data (best-effort, don't block on failure)
        await self._fetch_flow()

    async def _fetch_flow(self):
        """Fetch Flujo Occidente data from flow.html (best-effort)."""
        try:
            response = await self._fetch_with_retries(FLOW_URL, max_retries=1)
            flow_data = parse_flow_html(response.text)
            self._latest_flow = flow_data
        except Exception as e:
            logger.debug("Flow fetch failed (non-critical): %s", e)

    def get_latest_raw(self) -> Optional[dict]:
        """Get the latest raw parsed data (all sections with plant detail)."""
        return self._latest_raw

    def get_latest(self) -> Optional[dict]:
        """Get the latest summarized generation data for the API."""
        return self._latest_summary

    def get_latest_flow(self) -> Optional[dict]:
        """Get the latest Flujo Occidente data."""
        return self._latest_flow

    def get_status(self) -> dict:
        """Get collector health status with full observability."""
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
            "has_data": self._latest_raw is not None,
            "has_flow_data": self._latest_flow is not None,
            "source_url": GEN_URL,
            "flow_url": FLOW_URL,
            "interval_seconds": self.interval,
        }


# Module-level singleton
_collector: Optional[CNDCollector] = None


def get_collector() -> CNDCollector:
    """Get or create the singleton collector instance."""
    global _collector
    if _collector is None:
        _collector = CNDCollector(interval_seconds=60)
    return _collector
