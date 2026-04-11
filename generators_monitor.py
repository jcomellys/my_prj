"""Taz - Monitor del SIN Panama con reporte por WhatsApp."""

import datetime
import json
import os
import schedule
import time
from whatsapp_sender import send_whatsapp_message
from sitr_scraper import get_all_data

PREV_DATA_FILE = os.path.join(os.path.dirname(os.path.abspath(__file__)), ".taz_prev.json")


def load_previous():
    """Carga datos del reporte anterior."""
    try:
        with open(PREV_DATA_FILE, "r") as f:
            return json.load(f)
    except (FileNotFoundError, json.JSONDecodeError):
        return None


def save_current(data):
    """Guarda datos actuales para comparacion futura."""
    try:
        to_save = {
            "generacion": data.get("sin", {}).get("generacion", 0),
            "demanda": data.get("sin", {}).get("demanda", 0),
        }
        with open(PREV_DATA_FILE, "w") as f:
            json.dump(to_save, f)
    except Exception:
        pass


def format_taz_report(data, prev):
    """Formatea el reporte al estilo Taz con emojis."""
    now = datetime.datetime.now().strftime("%d/%m/%Y %H:%M")
    now_short = datetime.datetime.now().strftime("%H:%M")

    sin = data.get("sin") or {}
    gen = data.get("generation") or {}
    inter = data.get("interconnection") or {}
    embalses = data.get("embalses") or {}

    generacion = sin.get("generacion", 0)
    demanda = sin.get("demanda", 0)
    balance = generacion - demanda
    frecuencia = sin.get("frecuencia", 0)

    by_source = gen.get("by_source", {})
    fortuna = gen.get("fortuna", {})

    hidrica = by_source.get("hidrica", 0)
    termica = by_source.get("termica", 0)
    solar = by_source.get("solar", 0)
    eolica = by_source.get("eolica", 0)

    total_gen = generacion if generacion > 0 else (hidrica + termica + solar + eolica)

    def pct(val):
        return f"{(val / total_gen * 100):.2f}%" if total_gen > 0 else "0%"

    f1 = fortuna.get("fortuna_1", 0)
    f2 = fortuna.get("fortuna_2", 0)
    f3 = fortuna.get("fortuna_3", 0)
    f_total = f1 + f2 + f3

    bayano_data = gen.get("bayano", {})
    b1 = bayano_data.get("bayano_1", 0)
    b2 = bayano_data.get("bayano_2", 0)
    b3 = bayano_data.get("bayano_3", 0)
    bayano_total = b1 + b2 + b3

    chang_data = gen.get("changuinola", {})
    ch1 = chang_data.get("changuinola_1", 0)
    ch2 = chang_data.get("changuinola_2", 0)
    ch3 = chang_data.get("changuinola_3", 0)
    chang_total = ch1 + ch2 + ch3

    term_data = gen.get("termicas", {})
    gatun = term_data.get("gatun", 0)
    cobre = term_data.get("cobre", 0)
    costa_norte = term_data.get("costa_norte", 0)

    emb_fortuna = embalses.get("fortuna", {})
    emb_bayano = embalses.get("bayano", {})
    emb_chang = embalses.get("changuinola", {})

    inter_mw = inter.get("interconexion_mw", 0)
    inter_tipo = inter.get("tipo", "N/D")

    balance_sign = "+" if balance >= 0 else ""
    freq_str = f"*{frecuencia:.2f}* Hz" if frecuencia > 0 else "N/D"

    lines = [
        f"\U0001f4ca REPORTE SIN \u2014 {now}",
        "\u2501" * 23,
        f"\U0001f50b Generaci\u00f3n: *{generacion:.2f}* MW",
        f"\U0001f4ca Demanda: *{demanda:.2f}* MW",
        f"\u2696\ufe0f Balance: *{balance_sign}{balance:.2f}* MW",
        f"\U0001f504 Frecuencia: {freq_str}",
        "",
        "\U0001f3ed Por Fuente:",
        f"  \U0001f4a7 H\u00eddrica: {hidrica:.2f} MW ({pct(hidrica)})",
        f"  \U0001f525 T\u00e9rmica: {termica:.2f} MW ({pct(termica)})",
        f"  \u2600\ufe0f Solar: {solar:.2f} MW ({pct(solar)})",
        f"  \U0001f32c\ufe0f E\u00f3lica: {eolica:.2f} MW ({pct(eolica)})",
        "",
        f"\U0001f4a1 Fortuna (Total: {f_total:.2f} MW):",
        f"  \U0001f539 Fortuna 1: {f1:.2f} MW",
        f"  \U0001f539 Fortuna 2: {f2:.2f} MW",
        f"  \U0001f539 Fortuna 3: {f3:.2f} MW",
    ]

    # Embalse Fortuna
    if emb_fortuna.get("pct", 0) > 0:
        lines.append(f"  \U0001f4c8 Embalse: {emb_fortuna['pct']}% ({emb_fortuna.get('nivel', 0):.2f} msnm)")
    elif emb_fortuna.get("nivel", 0) > 0:
        lines.append(f"  \U0001f4c8 Embalse: {emb_fortuna['nivel']:.2f} msnm")

    lines.append("")
    lines.append(f"\U0001f4a1 Bayano (Total: {bayano_total:.2f} MW):")
    lines.append(f"  \U0001f539 Bayano 1: {b1:.2f} MW")
    lines.append(f"  \U0001f539 Bayano 2: {b2:.2f} MW")
    lines.append(f"  \U0001f539 Bayano 3: {b3:.2f} MW")

    # Embalse Bayano
    if emb_bayano.get("pct", 0) > 0:
        lines.append(f"  \U0001f4c8 Embalse: {emb_bayano['pct']}% ({emb_bayano.get('nivel', 0):.2f} msnm)")
    elif emb_bayano.get("nivel", 0) > 0:
        lines.append(f"  \U0001f4c8 Embalse: {emb_bayano['nivel']:.2f} msnm")

    # Changuinola
    lines.append("")
    lines.append(f"\U0001f4a7 Changuinola (Total: {chang_total:.2f} MW):")
    lines.append(f"  \U0001f539 Changuinola 1: {ch1:.2f} MW")
    lines.append(f"  \U0001f539 Changuinola 2: {ch2:.2f} MW")
    lines.append(f"  \U0001f539 Changuinola 3: {ch3:.2f} MW")

    # Embalse Changuinola
    if emb_chang.get("pct", 0) > 0:
        lines.append(f"  \U0001f4c8 Embalse: {emb_chang['pct']}% ({emb_chang.get('nivel', 0):.2f} msnm)")

    # Termicas grandes
    lines.append("")
    lines.append("\U0001f525 T\u00e9rmicas Principales:")
    lines.append(f"  \U0001f534 Gat\u00fan: {gatun:.2f} MW")
    lines.append(f"  \U0001f534 Cobre Panam\u00e1: {cobre:.2f} MW")
    lines.append(f"  \U0001f534 Costa Norte: {costa_norte:.2f} MW")

    if inter_mw > 0:
        lines.append("")
        lines.append(f"\U0001f517 Interconexi\u00f3n: {inter_tipo} *{inter_mw:.2f}* MW")

    # Alertas
    alertas = []
    info = []

    if generacion > 0 and balance < 200:
        reserva_pct = (balance / generacion * 100) if generacion > 0 else 0
        alertas.append(f"Reserva operativa baja: {reserva_pct:.0f}% ({balance:.0f} MW)")

    plantas_offline = []
    if f1 == 0:
        plantas_offline.append("Fortuna 1")
    if f2 == 0:
        plantas_offline.append("Fortuna 2")
    if f3 == 0:
        plantas_offline.append("Fortuna 3")
    if b1 == 0:
        plantas_offline.append("Bayano 1")
    if b2 == 0:
        plantas_offline.append("Bayano 2")
    if b3 == 0:
        plantas_offline.append("Bayano 3")
    if ch1 == 0:
        plantas_offline.append("Changuinola 1")
    if ch2 == 0:
        plantas_offline.append("Changuinola 2")
    if ch3 == 0:
        plantas_offline.append("Changuinola 3")
    if gatun == 0:
        plantas_offline.append("Gat\u00fan")
    if cobre == 0:
        plantas_offline.append("Cobre")
    if costa_norte == 0:
        plantas_offline.append("Costa Norte")
    if plantas_offline:
        alertas.append(f"Plantas importantes sin generaci\u00f3n: {', '.join(plantas_offline)}")

    if frecuencia > 0 and (frecuencia < 59.95 or frecuencia > 60.05):
        alertas.append(f"Frecuencia fuera de rango: {frecuencia:.2f} Hz")

    # Comparacion con reporte anterior
    if prev:
        prev_gen = prev.get("generacion", 0)
        prev_dem = prev.get("demanda", 0)
        if prev_gen > 0 and generacion > 0:
            diff_gen = generacion - prev_gen
            if abs(diff_gen) > 10:
                direction = "subi\u00f3" if diff_gen > 0 else "baj\u00f3"
                info.append(f"Generaci\u00f3n {direction} {abs(diff_gen):.0f} MW")
        if prev_dem > 0 and demanda > 0:
            diff_dem = demanda - prev_dem
            if abs(diff_dem) > 10:
                direction = "subi\u00f3" if diff_dem > 0 else "baj\u00f3"
                info.append(f"Demanda {direction} {abs(diff_dem):.0f} MW")

    if alertas or info:
        lines.append("")
        lines.append(f"\U0001f514 *ALERTA SIN \u2014 {now_short}*")
        for a in alertas:
            lines.append(f"\u26a0\ufe0f {a}")
        for i in info:
            lines.append(f"\u2139\ufe0f {i}")

    lines.append("")
    lines.append("\U0001f9ec Taz \u2014 Monitoreo Continuo")

    return "\n".join(lines)


def send_taz_report():
    """Obtiene datos del SITR y envia reporte por WhatsApp."""
    now = datetime.datetime.now().strftime("%H:%M")
    print(f"[{now}] Obteniendo datos del SITR...")

    try:
        data = get_all_data()
    except Exception as e:
        print(f"Error: {e}")
        send_whatsapp_message(f"\u26a0\ufe0f ALERTA Taz {now}: Error obteniendo datos - {e}")
        return

    gen = data.get("generation")
    if not gen or sum(gen.get("by_source", {}).values()) == 0:
        print("No se pudieron obtener datos del SITR.")
        send_whatsapp_message(
            f"\u26a0\ufe0f ALERTA Taz {now}: No se pudieron obtener datos del SITR. "
            "Verificar conexion con sitr.cnd.com.pa"
        )
        return

    prev = load_previous()
    report = format_taz_report(data, prev)
    print(report)
    print("-" * 30)
    send_whatsapp_message(report)
    save_current(data)
    print(f"[{now}] Reporte Taz enviado.")


def start_monitoring(interval_minutes=60):
    """Inicia el monitoreo cada hora."""
    print(f"\U0001f9ec Taz iniciado - Reportes cada {interval_minutes} minutos")
    print("Enviando primer reporte...")
    send_taz_report()

    schedule.every(interval_minutes).minutes.do(send_taz_report)

    print(f"Proximo reporte en {interval_minutes} minutos. Ctrl+C para detener.")
    while True:
        schedule.run_pending()
        time.sleep(1)


if __name__ == "__main__":
    start_monitoring(interval_minutes=60)
