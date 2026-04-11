"""Scraper para obtener datos del SITR (CND Panama) - sitr.cnd.com.pa"""

import requests
from bs4 import BeautifulSoup
import re
import json

HEADERS = {
    "User-Agent": "Mozilla/5.0 (Linux; Android 13; SM-G991B) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Mobile Safari/537.36",
    "Accept": "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
    "Accept-Language": "es-PA,es;q=0.9,en;q=0.8",
    "Referer": "https://sitr.cnd.com.pa/m/",
    "X-Requested-With": "XMLHttpRequest",
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


def fetch_raw(path):
    """Obtiene el contenido raw de una URL del SITR."""
    url = f"{BASE_URL}{path}"
    resp = SESSION.get(url, timeout=15)
    resp.raise_for_status()
    return resp.text


def parse_number(text):
    """Extrae un numero de un texto."""
    if not text:
        return 0.0
    # Buscar el primer numero (con decimales) en el texto
    match = re.search(r"-?\d+\.?\d*", text.strip().replace(",", ""))
    if match:
        try:
            return float(match.group())
        except ValueError:
            return 0.0
    return 0.0


def try_api_endpoints():
    """Intenta descubrir y usar endpoints API del SITR."""
    api_paths = [
        "/api/sin",
        "/api/generation",
        "/api/datos",
        "/m/api/sin",
        "/m/api/gen",
        "/m/pub/api/sin",
        "/signalr/hubs",
        "/m/datos/sin.json",
        "/m/pub/sin.json",
        "/m/pub/datos.json",
        "/m/pub/sindata.ashx",
        "/m/pub/gendata.ashx",
        "/m/pub/data.ashx",
        "/m/pub/GetData.ashx",
        "/m/Handler.ashx",
        "/m/pub/Handler.ashx",
    ]
    for path in api_paths:
        try:
            url = f"{BASE_URL}{path}"
            resp = SESSION.get(url, timeout=5)
            if resp.status_code == 200 and len(resp.text) > 10:
                print(f"API encontrada: {path}")
                print(f"Respuesta: {resp.text[:500]}")
                return {"path": path, "content": resp.text}
        except Exception:
            continue
    return None


def extract_js_data(soup):
    """Extrae datos de variables JavaScript en la pagina."""
    data = {}
    scripts = soup.find_all("script")
    for script in scripts:
        text = script.string or ""
        # Buscar asignaciones de variables
        patterns = [
            (r"generacion\s*[=:]\s*([\d.]+)", "generacion"),
            (r"demanda\s*[=:]\s*([\d.]+)", "demanda"),
            (r"frecuencia\s*[=:]\s*([\d.]+)", "frecuencia"),
            (r"frequency\s*[=:]\s*([\d.]+)", "frecuencia"),
            (r"generation\s*[=:]\s*([\d.]+)", "generacion"),
            (r"demand\s*[=:]\s*([\d.]+)", "demanda"),
            (r"balance\s*[=:]\s*(-?[\d.]+)", "balance"),
            (r"interconexion\s*[=:]\s*(-?[\d.]+)", "interconexion"),
        ]
        for pattern, key in patterns:
            match = re.search(pattern, text, re.IGNORECASE)
            if match:
                data[key] = float(match.group(1))

        # Buscar objetos JSON en scripts
        json_matches = re.findall(r"\{[^{}]*generaci[^{}]*\}", text, re.IGNORECASE)
        for jm in json_matches:
            try:
                obj = json.loads(jm)
                data.update(obj)
            except (json.JSONDecodeError, ValueError):
                pass

    return data


def get_sin_data():
    """Obtiene datos generales del SIN: generacion, demanda, frecuencia."""
    try:
        soup = fetch_page("/m/pub/sin.html")

        data = {
            "generacion": 0.0,
            "demanda": 0.0,
            "frecuencia": 0.0,
        }

        # Intentar extraer de JavaScript
        js_data = extract_js_data(soup)
        if js_data:
            data.update({k: v for k, v in js_data.items() if k in data})

        # Buscar en tablas
        tds = soup.find_all("td")
        for i, td in enumerate(tds):
            td_text = td.get_text(strip=True).lower()
            if any(w in td_text for w in ["generaci", "gen total", "gen.", "generation"]):
                if i + 1 < len(tds):
                    val = parse_number(tds[i + 1].get_text())
                    if val > 0:
                        data["generacion"] = val
            elif any(w in td_text for w in ["demanda", "demand", "carga", "load"]):
                if i + 1 < len(tds):
                    val = parse_number(tds[i + 1].get_text())
                    if val > 0:
                        data["demanda"] = val
            elif any(w in td_text for w in ["frecuencia", "freq", "hz"]):
                if i + 1 < len(tds):
                    val = parse_number(tds[i + 1].get_text())
                    if val > 0:
                        data["frecuencia"] = val

        # Buscar en spans, divs con IDs
        for el in soup.find_all(["span", "div", "p", "label", "b", "strong"]):
            el_text = el.get_text(strip=True)
            el_id = (el.get("id") or "").lower()
            el_class = " ".join(el.get("class") or []).lower()
            attrs = el_id + " " + el_class

            if any(w in attrs for w in ["gen", "generacion", "generation"]):
                val = parse_number(el_text)
                if val > 100:
                    data["generacion"] = val
            elif any(w in attrs for w in ["dem", "demanda", "demand", "carga"]):
                val = parse_number(el_text)
                if val > 100:
                    data["demanda"] = val
            elif any(w in attrs for w in ["freq", "frec", "frecuencia"]):
                val = parse_number(el_text)
                if 59 < val < 61:
                    data["frecuencia"] = val

        return data
    except Exception as e:
        print(f"Error obteniendo datos SIN: {e}")
        return None


def get_generation_data():
    """Obtiene datos de generacion por planta y por fuente."""
    try:
        soup = fetch_page("/m/pub/gen.html")

        # Tambien intentar extraer de JavaScript
        js_data = extract_js_data(soup)

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
        bayano = {"bayano": 0.0}

        # Palabras clave para clasificar por tipo
        HIDRO_KEYS = [
            "hidr", "agua", "chan", "bayano", "fortuna", "esti", "barro",
            "los valles", "gualaca", "caldera", "macho", "monte", "dolega",
            "bonyic", "bajo", "rio", "chiriqui", "piedra", "pando",
        ]
        TERM_KEYS = [
            "term", "gas", "diesel", "bunker", "carbon", "gnl", "lng",
            "bahia", "cobre", "pacora", "jinro", "thermal",
        ]
        SOLAR_KEYS = ["solar", "foto", "pv", "photovoltaic"]
        EOLICA_KEYS = ["eol", "viento", "wind", "penonomé", "penonome"]

        rows = soup.find_all("tr")
        for row in rows:
            cols = row.find_all("td")
            if len(cols) >= 2:
                name = cols[0].get_text(strip=True)
                if not name or name.lower() in ["planta", "nombre", "central", "total"]:
                    continue

                value = parse_number(cols[-1].get_text())
                # Si hay mas de 2 columnas, el MW puede estar en otra posicion
                if value == 0 and len(cols) > 2:
                    for col in cols[1:]:
                        v = parse_number(col.get_text())
                        if v > 0:
                            value = v
                            break

                name_lower = name.lower()
                plants.append({"name": name, "mw": value})

                # Clasificar Fortuna
                if "fortuna" in name_lower:
                    if any(x in name_lower for x in ["1", " i ", "i\n"]) or name_lower.endswith("i") or name_lower.endswith("1"):
                        fortuna["fortuna_1"] = value
                    elif any(x in name_lower for x in ["2", " ii ", "ii\n"]) or name_lower.endswith("ii") or name_lower.endswith("2"):
                        fortuna["fortuna_2"] = value
                    elif any(x in name_lower for x in ["3", " iii", "iii\n"]) or name_lower.endswith("iii") or name_lower.endswith("3"):
                        fortuna["fortuna_3"] = value

                # Clasificar Bayano
                if "bayano" in name_lower:
                    bayano["bayano"] += value

                # Clasificar por fuente
                if any(w in name_lower for w in HIDRO_KEYS):
                    by_source["hidrica"] += value
                elif any(w in name_lower for w in TERM_KEYS):
                    by_source["termica"] += value
                elif any(w in name_lower for w in SOLAR_KEYS):
                    by_source["solar"] += value
                elif any(w in name_lower for w in EOLICA_KEYS):
                    by_source["eolica"] += value

        return {
            "plants": plants,
            "by_source": by_source,
            "fortuna": fortuna,
            "bayano": bayano,
        }
    except Exception as e:
        print(f"Error obteniendo datos de generacion: {e}")
        return None


def get_interconnection_data():
    """Obtiene datos de interconexion regional."""
    try:
        soup = fetch_page("/m/pub/int.html")

        js_data = extract_js_data(soup)
        data = {"interconexion_mw": 0.0, "tipo": ""}

        if "interconexion" in js_data:
            val = js_data["interconexion"]
            data["interconexion_mw"] = abs(val)
            data["tipo"] = "Exportando" if val > 0 else "Importando"
            return data

        # Buscar en todos los elementos
        for el in soup.find_all(["td", "span", "div", "p", "b", "strong", "label"]):
            el_text = el.get_text(strip=True).lower()
            el_id = (el.get("id") or "").lower()

            if any(w in el_text + el_id for w in ["inter", "export", "import", "flujo", "transfer"]):
                val = parse_number(el.get_text())
                if val != 0:
                    data["interconexion_mw"] = abs(val)
                    data["tipo"] = "Exportando" if val > 0 else "Importando"

        return data
    except Exception as e:
        print(f"Error obteniendo datos de interconexion: {e}")
        return None


def get_embalse_data():
    """Obtiene niveles de embalses (Fortuna, Bayano)."""
    embalses = {
        "fortuna": {"nivel": 0.0, "pct": 0.0},
        "bayano": {"nivel": 0.0, "pct": 0.0},
    }

    # Fortuna: max operativo ~1055 msnm, min ~1010 msnm
    FORTUNA_MAX = 1055.0
    FORTUNA_MIN = 1010.0
    # Bayano: max operativo ~62 msnm, min ~50 msnm
    BAYANO_MAX = 62.0
    BAYANO_MIN = 50.0

    # Intentar desde SITR (vert.html o sin.html)
    for path in ["/m/pub/vert.html", "/m/pub/sin.html"]:
        try:
            soup = fetch_page(path)

            # Buscar en JavaScript
            for script in soup.find_all("script"):
                text = script.string or ""
                for pattern in [r"fortuna.*?(\d{4}\.?\d*)", r"nivel.*?fortuna.*?(\d{4}\.?\d*)"]:
                    match = re.search(pattern, text, re.IGNORECASE)
                    if match:
                        val = float(match.group(1))
                        if 1000 < val < 1100:
                            embalses["fortuna"]["nivel"] = val
                for pattern in [r"bayano.*?(\d{2}\.?\d*)", r"nivel.*?bayano.*?(\d{2}\.?\d*)"]:
                    match = re.search(pattern, text, re.IGNORECASE)
                    if match:
                        val = float(match.group(1))
                        if 40 < val < 70:
                            embalses["bayano"]["nivel"] = val

            # Buscar en tablas y elementos
            for el in soup.find_all(["td", "span", "div", "p", "b", "label"]):
                el_text = el.get_text(strip=True).lower()
                el_id = (el.get("id") or "").lower()
                combined = el_text + " " + el_id

                if "fortuna" in combined and ("embalse" in combined or "nivel" in combined or "%" in el_text):
                    val = parse_number(el.get_text())
                    if 1000 < val < 1100:
                        embalses["fortuna"]["nivel"] = val
                    elif 0 < val <= 100:
                        embalses["fortuna"]["pct"] = val

                if "bayano" in combined and ("embalse" in combined or "nivel" in combined or "%" in el_text):
                    val = parse_number(el.get_text())
                    if 40 < val < 70:
                        embalses["bayano"]["nivel"] = val
                    elif 0 < val <= 100:
                        embalses["bayano"]["pct"] = val
        except Exception:
            continue

    # Intentar desde hidromet.com.pa como respaldo
    try:
        resp = SESSION.get("https://www.hidromet.com.pa/es/centrales-hidroelectricas", timeout=10)
        if resp.status_code == 200:
            soup = BeautifulSoup(resp.text, "html.parser")
            text = soup.get_text(" ", strip=True)

            # Buscar niveles
            fort_match = re.search(r"fortuna.*?(\d{4}\.?\d*)\s*msnm", text, re.IGNORECASE)
            if fort_match:
                embalses["fortuna"]["nivel"] = float(fort_match.group(1))

            bay_match = re.search(r"bayano.*?(\d{2}\.?\d*)\s*msnm", text, re.IGNORECASE)
            if bay_match:
                embalses["bayano"]["nivel"] = float(bay_match.group(1))
    except Exception:
        pass

    # Calcular porcentaje si tenemos nivel pero no porcentaje
    if embalses["fortuna"]["nivel"] > 0 and embalses["fortuna"]["pct"] == 0:
        nivel = embalses["fortuna"]["nivel"]
        embalses["fortuna"]["pct"] = round(
            (nivel - FORTUNA_MIN) / (FORTUNA_MAX - FORTUNA_MIN) * 100, 1
        )

    if embalses["bayano"]["nivel"] > 0 and embalses["bayano"]["pct"] == 0:
        nivel = embalses["bayano"]["nivel"]
        embalses["bayano"]["pct"] = round(
            (nivel - BAYANO_MIN) / (BAYANO_MAX - BAYANO_MIN) * 100, 1
        )

    return embalses


def get_all_data():
    """Obtiene todos los datos del SITR."""
    sin = get_sin_data()
    gen = get_generation_data()
    inter = get_interconnection_data()
    embalses = get_embalse_data()

    # Si no tenemos generacion total del SIN, calcularla desde las fuentes
    if sin and gen:
        by_source = gen.get("by_source", {})
        total_from_sources = sum(by_source.values())

        if sin["generacion"] == 0 and total_from_sources > 0:
            sin["generacion"] = round(total_from_sources, 2)

        # Estimar demanda como ~95% de generacion si no la tenemos
        if sin["demanda"] == 0 and sin["generacion"] > 0:
            sin["demanda"] = round(sin["generacion"] * 0.95, 2)

    return {
        "sin": sin,
        "generation": gen,
        "interconnection": inter,
        "embalses": embalses,
    }


def debug_pages():
    """Muestra estructura de las paginas para diagnostico."""
    for path in ["/m/pub/sin.html", "/m/pub/gen.html", "/m/pub/int.html"]:
        print(f"\n{'='*50}")
        print(f"PAGINA: {path}")
        print(f"{'='*50}")
        try:
            soup = fetch_page(path)
            # Mostrar scripts
            scripts = soup.find_all("script")
            for s in scripts:
                src = s.get("src", "")
                if src:
                    print(f"Script externo: {src}")
                elif s.string:
                    print(f"Script inline: {s.string[:300]}")

            # Mostrar elementos con IDs
            for el in soup.find_all(id=True):
                print(f"ID: {el.get('id')} -> {el.name}: {el.get_text(strip=True)[:100]}")

            # Mostrar tablas
            tables = soup.find_all("table")
            print(f"Tablas encontradas: {len(tables)}")
            for t_idx, table in enumerate(tables):
                rows = table.find_all("tr")
                print(f"  Tabla {t_idx}: {len(rows)} filas")
                for row in rows[:5]:
                    cells = [c.get_text(strip=True) for c in row.find_all(["td", "th"])]
                    print(f"    {cells}")
        except Exception as e:
            print(f"Error: {e}")


if __name__ == "__main__":
    # Modo diagnostico
    print("=== DIAGNOSTICO SITR ===")
    api = try_api_endpoints()
    if api:
        print(f"API encontrada: {api['path']}")
    else:
        print("No se encontraron API endpoints")

    print("\n=== ESTRUCTURA DE PAGINAS ===")
    debug_pages()

    print("\n=== DATOS EXTRAIDOS ===")
    data = get_all_data()
    print("SIN:", data["sin"])
    print("Generacion:", data["generation"])
    print("Interconexion:", data["interconnection"])
