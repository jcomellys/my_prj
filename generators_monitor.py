"""Taz - Monitor del SIN Panama con reporte por WhatsApp."""

import datetime
import schedule
import time
from whatsapp_sender import send_whatsapp_message
from sitr_scraper import get_all_data


def format_taz_report(data):
    """Formatea el reporte al estilo Taz."""
    now = datetime.datetime.now().strftime("%d/%m/%Y %H:%M")

    sin = data.get("sin") or {}
    gen = data.get("generation") or {}
    inter = data.get("interconnection") or {}

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

    inter_mw = inter.get("interconexion_mw", 0)
    inter_tipo = inter.get("tipo", "N/D")

    balance_sign = "+" if balance >= 0 else ""

    lines = [
        f"REPORTE SIN - {now}",
        "=" * 30,
        f"Generacion: *{generacion:.2f}* MW",
        f"Demanda: *{demanda:.2f}* MW",
        f"Balance: *{balance_sign}{balance:.2f}* MW",
        f"Frecuencia: *{frecuencia:.2f}* Hz",
        "",
        "Por Fuente:",
        f"  Hidrica: {hidrica:.2f} MW ({pct(hidrica)})",
        f"  Termica: {termica:.2f} MW ({pct(termica)})",
        f"  Solar: {solar:.2f} MW ({pct(solar)})",
        f"  Eolica: {eolica:.2f} MW ({pct(eolica)})",
        "",
        f"Fortuna (Total: {f_total:.2f} MW):",
        f"  Fortuna 1: {f1:.2f} MW",
        f"  Fortuna 2: {f2:.2f} MW",
        f"  Fortuna 3: {f3:.2f} MW",
        "",
        f"Interconexion: {inter_tipo} *{inter_mw:.2f}* MW",
    ]

    # Alertas
    alertas = []
    if balance < 200:
        reserva_pct = (balance / generacion * 100) if generacion > 0 else 0
        alertas.append(f"ALERTA: Reserva operativa baja: {reserva_pct:.0f}% ({balance:.0f} MW)")
    if f1 == 0:
        alertas.append("ALERTA: Fortuna 1 sin generacion")
    if f2 == 0:
        alertas.append("ALERTA: Fortuna 2 sin generacion")
    if f3 == 0:
        alertas.append("ALERTA: Fortuna 3 sin generacion")
    if frecuencia > 0 and (frecuencia < 59.95 or frecuencia > 60.05):
        alertas.append(f"ALERTA: Frecuencia fuera de rango: {frecuencia:.2f} Hz")

    if alertas:
        lines.append("")
        lines.append("ALERTAS:")
        lines.extend(alertas)

    lines.append("")
    lines.append("Taz - Monitoreo Continuo")

    return "\n".join(lines)


def send_taz_report():
    """Obtiene datos del SITR y envia reporte por WhatsApp."""
    print(f"[{datetime.datetime.now()}] Obteniendo datos del SITR...")
    data = get_all_data()

    if not data["sin"] and not data["generation"]:
        print("No se pudieron obtener datos del SITR.")
        send_whatsapp_message(
            "ALERTA Taz: No se pudieron obtener datos del SITR. "
            "Verificar conexion con sitr.cnd.com.pa"
        )
        return

    report = format_taz_report(data)
    print(report)
    print("-" * 30)
    send_whatsapp_message(report)
    print("Reporte Taz enviado por WhatsApp.")


def start_monitoring(interval_minutes=30):
    """Inicia el monitoreo periodico del SIN."""
    print(f"Taz iniciado - Reportes cada {interval_minutes} minutos")
    print("Enviando primer reporte...")
    send_taz_report()

    schedule.every(interval_minutes).minutes.do(send_taz_report)

    while True:
        schedule.run_pending()
        time.sleep(1)


if __name__ == "__main__":
    start_monitoring(interval_minutes=30)
