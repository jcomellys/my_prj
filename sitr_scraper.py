"""Scraper para obtener datos del SITR (CND Panama) - sitr.cnd.com.pa"""

import requests
from bs4 import BeautifulSoup
import re

HEADERS = {
    "User-Agent": "Mozilla/5.0 (Linux; Android 13; SM-G991B) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Mobile Safari/537.36",
    "Accept": "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
    "Accept-Language": "es-PA,es;q=0.9,en;q=0.8",
    "Referer": "https://sitr.cnd.com.pa/m/",
}

BASE_URL = "https://sitr.cnd.com.pa"
SESSION = requests.Session()
SESSION.headers.update(HEADERS)


def fetch_page(path):
    """Obtiene el contenido HTML de una pagina del SITR."""
    url = f"{BASE_URL}{path}"
    resp = SESSION.get(url, timeout=15)
    resp.raise_for_status()
    return BeautifulSoup(resp.text, "html.parser")


def parse_number(text):
    """Extrae un numero de un texto."""
    if not text:
        return 0.0
    cleaned = re.sub(r"[^\d.\-]", "", text.strip())
    try:
        return float(cleaned)
    except ValueError:
        return 0.0


def get_sin_data():
    """Obtiene datos generales del SIN: generacion, demanda, frecuencia."""
    try:
        soup = fetch_page("/m/pub/sin.html")
        text = soup.get_text(" ", strip=True)

        data = {
            "generacion": 0.0,
            "demanda": 0.0,
            "frecuencia": 0.0,
        }

        # Buscar patrones comunes en la pagina
        tables = soup.find_all("table")
        rows = soup.find_all("tr")
        tds = soup.find_all("td")

        # Intentar extraer datos de tablas
        for i, td in enumerate(tds):
            td_text = td.get_text(strip=True).lower()
            if "generaci" in td_text or "gen total" in td_text:
                if i + 1 < len(tds):
                    data["generacion"] = parse_number(tds[i + 1].get_text())
            elif "demanda" in td_text:
                if i + 1 < len(tds):
                    data["demanda"] = parse_number(tds[i + 1].get_text())
            elif "frecuencia" in td_text or "freq" in td_text:
                if i + 1 < len(tds):
                    data["frecuencia"] = parse_number(tds[i + 1].get_text())

        # Buscar tambien en spans y divs
        for el in soup.find_all(["span", "div", "p"]):
            el_text = el.get_text(strip=True)
            el_id = el.get("id", "").lower()
            el_class = " ".join(el.get("class", [])).lower()

            if "gen" in el_id or "generacion" in el_id:
                data["generacion"] = parse_number(el_text)
            elif "dem" in el_id or "demanda" in el_id:
                data["demanda"] = parse_number(el_text)
            elif "freq" in el_id or "frecuencia" in el_id or "frec" in el_id:
                data["frecuencia"] = parse_number(el_text)

        return data
    except Exception as e:
        print(f"Error obteniendo datos SIN: {e}")
        return None


def get_generation_data():
    """Obtiene datos de generacion por planta y por fuente."""
    try:
        soup = fetch_page("/m/pub/gen.html")

        plants = []
        by_source = {
            "hidrica": 0.0,
            "termica": 0.0,
            "solar": 0.0,
            "eolica": 0.0,
        }
        fortuna = {
            "fortuna_1": 0.0,
            "fortuna_2": 0.0,
            "fortuna_3": 0.0,
        }

        tables = soup.find_all("table")
        rows = soup.find_all("tr")

        for row in rows:
            cols = row.find_all("td")
            if len(cols) >= 2:
                name = cols[0].get_text(strip=True)
                value = parse_number(cols[-1].get_text())
                name_lower = name.lower()

                plants.append({"name": name, "mw": value})

                # Clasificar Fortuna
                if "fortuna" in name_lower:
                    if "1" in name_lower or "i" == name_lower.split()[-1]:
                        fortuna["fortuna_1"] = value
                    elif "2" in name_lower or "ii" == name_lower.split()[-1]:
                        fortuna["fortuna_2"] = value
                    elif "3" in name_lower or "iii" == name_lower.split()[-1]:
                        fortuna["fortuna_3"] = value

                # Clasificar por fuente
                if any(w in name_lower for w in ["hidr", "agua", "chan", "bayano", "fortuna", "esti", "barro"]):
                    by_source["hidrica"] += value
                elif any(w in name_lower for w in ["term", "gas", "diesel", "bunker", "carbon"]):
                    by_source["termica"] += value
                elif any(w in name_lower for w in ["solar", "foto"]):
                    by_source["solar"] += value
                elif any(w in name_lower for w in ["eol", "viento", "wind"]):
                    by_source["eolica"] += value

        return {
            "plants": plants,
            "by_source": by_source,
            "fortuna": fortuna,
        }
    except Exception as e:
        print(f"Error obteniendo datos de generacion: {e}")
        return None


def get_interconnection_data():
    """Obtiene datos de interconexion regional."""
    try:
        soup = fetch_page("/m/pub/int.html")
        tds = soup.find_all("td")

        data = {"interconexion_mw": 0.0, "tipo": ""}

        for i, td in enumerate(tds):
            td_text = td.get_text(strip=True).lower()
            if "inter" in td_text or "export" in td_text or "import" in td_text:
                if i + 1 < len(tds):
                    val = parse_number(tds[i + 1].get_text())
                    data["interconexion_mw"] = val
                    data["tipo"] = "Exportando" if val > 0 else "Importando"

        # Buscar en spans/divs tambien
        for el in soup.find_all(["span", "div"]):
            el_id = el.get("id", "").lower()
            if "inter" in el_id or "export" in el_id or "import" in el_id:
                val = parse_number(el.get_text())
                if val != 0:
                    data["interconexion_mw"] = abs(val)
                    data["tipo"] = "Exportando" if val > 0 else "Importando"

        return data
    except Exception as e:
        print(f"Error obteniendo datos de interconexion: {e}")
        return None


def get_all_data():
    """Obtiene todos los datos del SITR."""
    sin = get_sin_data()
    gen = get_generation_data()
    inter = get_interconnection_data()

    return {
        "sin": sin,
        "generation": gen,
        "interconnection": inter,
    }


if __name__ == "__main__":
    data = get_all_data()
    print("Datos SIN:", data["sin"])
    print("Generacion:", data["generation"])
    print("Interconexion:", data["interconnection"])
