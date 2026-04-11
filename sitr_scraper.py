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
    """Obtiene datos generales del SIN: generacion, demanda, frecuencia.

    Estructura de sin.html:
      <h5>Generación total</h5>
      <h4>1832 MW</h4>
      ...
      <h5>Demanda Total</h5>
      <h4>1637 MW</h4>
      ...
      <h5>Intercambio neto</h5>
      <h4>194.76 MW</h4>
      ...
      <h5>Reserva rodante</h5>
      <h4>205.62 MW</h4>
    Frecuencia: div id="Fz" (cargado por JS)
    """
    try:
        soup = fetch_page("/m/pub/sin.html")

        data = {
            "generacion": 0.0,
            "demanda": 0.0,
            "frecuencia": 0.0,
            "intercambio_neto": 0.0,
            "reserva_rodante": 0.0,
        }

        # Buscar pares h5 (titulo) + h4 (valor) dentro de widgets
        h5_tags = soup.find_all("h5")
        for h5 in h5_tags:
            title = h5.get_text(strip=True).lower()
            # Buscar el h4 hermano mas cercano
            h4 = h5.find_next("h4")
            if not h4:
                continue
            val = parse_number(h4.get_text())

            if "generaci" in title and "total" in title:
                data["generacion"] = val
            elif "demanda" in title and "total" in title:
                data["demanda"] = val
            elif "intercambio neto" in title:
                data["intercambio_neto"] = val
            elif "reserva" in title:
                data["reserva_rodante"] = val

        # Frecuencia: buscar en scripts el valor del gauge (div id="Fz")
        for script in soup.find_all("script"):
            text = script.string or ""
            # Buscar patron de datos del gauge: valor tipo 60.XXX
            freq_patterns = [
                r"data:\s*\[\s*\{\s*value:\s*([\d.]+)",
                r"value:\s*(6[\d.]+)",
                r"(60\.\d{2,3})",
            ]
            for pattern in freq_patterns:
                match = re.search(pattern, text)
                if match:
                    val = float(match.group(1))
                    if 59 < val < 61:
                        data["frecuencia"] = val
                        break
            if data["frecuencia"] > 0:
                break

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
        bayano = {
            "bayano_1": 0.0,
            "bayano_2": 0.0,
            "bayano_3": 0.0,
        }
        changuinola = {
            "changuinola_1": 0.0,
            "changuinola_2": 0.0,
            "changuinola_3": 0.0,
        }
        termicas = {
            "gatun": 0.0,
            "cobre": 0.0,
            "costa_norte": 0.0,
        }

        # Palabras clave para clasificar por tipo
        HIDRO_KEYS = [
            "hidr", "agua", "chan", "bayano", "fortuna", "esti", "barro",
            "los valles", "gualaca", "caldera", "macho", "monte", "dolega",
            "bonyic", "bajo", "rio", "chiriqui", "piedra", "pando",
            "bugaba", "cochea", "concepci", "madden", "mendre", "pedregalito",
            "paso ancho", "lirio",
        ]
        TERM_KEYS = [
            "term", "gas", "diesel", "bunker", "carbon", "gnl", "lng",
            "bahia", "cobre", "pacora", "jinro", "thermal",
            "gatun", "gat\u00fan", "costa norte",
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
                    if "1" in name_lower:
                        bayano["bayano_1"] = value
                    elif "2" in name_lower:
                        bayano["bayano_2"] = value
                    elif "3" in name_lower:
                        bayano["bayano_3"] = value

                # Clasificar Changuinola
                if "changuinola" in name_lower:
                    if "1" in name_lower:
                        changuinola["changuinola_1"] = value
                    elif "2" in name_lower:
                        changuinola["changuinola_2"] = value
                    elif "3" in name_lower:
                        changuinola["changuinola_3"] = value

                # Clasificar termicas grandes
                if "gatun" in name_lower or "gatún" in name_lower:
                    termicas["gatun"] += value
                elif "cobre" in name_lower:
                    termicas["cobre"] += value
                elif "costa norte" in name_lower:
                    termicas["costa_norte"] += value

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
            "changuinola": changuinola,
            "termicas": termicas,
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
    """Obtiene niveles de embalses desde sin.html del SITR.

    La tabla tiene columnas: Embalse | Nivel minimo | Valor actual (con %) | Nivel maximo
    Embalses: Changuinola 1, Bonyic, Fortuna, La Estrella, Mendre_Presa,
              Mendre II, Esti_Chiriqui, Esti_Barrigon, Gualaca, Bayano, etc.
    """
    embalses = {}

    try:
        soup = fetch_page("/m/pub/sin.html")

        # Buscar filas con clase sitr-water (tabla de embalses)
        rows = soup.find_all("tr", class_="sitr-water")
        for row in rows:
            # Buscar nombre del embalse (span con texto no numerico)
            name = ""
            for span in row.find_all("span"):
                text = span.get_text(strip=True)
                if text and not re.match(r"^[\d.%]+$", text):
                    name = text
                    break

            if not name:
                continue
            name_lower = name.lower().replace("_", " ")

            # Porcentaje: dentro de div.progress-bar span
            pct = 0
            progress = row.find("div", class_="progress-bar")
            if progress:
                pct_span = progress.find("span")
                if pct_span:
                    pct_match = re.search(r"(\d+)%", pct_span.get_text())
                    if pct_match:
                        pct = int(pct_match.group(1))

            # Nivel actual: en <h6> (el numero coloreado)
            nivel_actual = 0.0
            h6 = row.find("h6")
            if h6:
                nivel_actual = parse_number(h6.get_text())

            # Nivel minimo: en td.text-right span
            nivel_min = 0.0
            td_right = row.find("td", class_="text-right")
            if td_right:
                span = td_right.find("span")
                if span:
                    nivel_min = parse_number(span.get_text())

            # Nivel maximo: ultimo td span (que no sea el nombre, min o actual)
            nivel_max = 0.0
            all_tds = row.find_all("td")
            if all_tds:
                last_td = all_tds[-1]
                span = last_td.find("span")
                if span:
                    val = parse_number(span.get_text())
                    if val > 0 and val != pct:
                        nivel_max = val

            # Mapear nombres
            key = None
            if "fortuna" in name_lower:
                key = "fortuna"
            elif "bayano" in name_lower:
                key = "bayano"
            elif "changuinola" in name_lower:
                key = "changuinola"
            elif "bonyic" in name_lower:
                key = "bonyic"
            elif "estrella" in name_lower:
                key = "la_estrella"
            elif "mendre" in name_lower and "presa" in name_lower:
                key = "mendre_presa"
            elif "mendre" in name_lower:
                key = "mendre_ii"
            elif "chiriqui" in name_lower or ("esti" in name_lower and "chir" in name_lower):
                key = "esti_chiriqui"
            elif "barrig" in name_lower or ("esti" in name_lower and "barr" in name_lower):
                key = "esti_barrigon"
            elif "gualaca" in name_lower:
                key = "gualaca"
            elif "lorena" in name_lower:
                key = "lorena"

            if key:
                embalses[key] = {
                    "nombre": name,
                    "nivel": nivel_actual,
                    "nivel_min": nivel_min,
                    "nivel_max": nivel_max,
                    "pct": pct,
                }

    except Exception as e:
        print(f"Error obteniendo datos de embalses: {e}")

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

        if sin["demanda"] == 0 and sin["generacion"] > 0:
            sin["demanda"] = round(sin["generacion"] * 0.95, 2)

    # Usar intercambio neto del SIN si el scraper de int.html fallo
    if sin and inter:
        if inter["interconexion_mw"] == 0 and sin.get("intercambio_neto", 0) > 0:
            inter["interconexion_mw"] = sin["intercambio_neto"]
            inter["tipo"] = "Exportando" if sin["intercambio_neto"] > 0 else "Importando"

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
