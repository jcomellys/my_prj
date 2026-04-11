"""Monitor de estado de generadores de fortuna con reporte por WhatsApp."""

import random
import datetime
import schedule
import time
from whatsapp_sender import send_whatsapp_message


def get_generators_status():
    """Obtiene el estado de los generadores de fortuna.

    TODO: Reemplazar esta funcion con la conexion real a tus generadores.
    Actualmente genera datos de ejemplo.
    """
    generators = ["GEN-001", "GEN-002", "GEN-003"]
    statuses = []

    for gen_id in generators:
        status = {
            "id": gen_id,
            "state": random.choice(["ACTIVO", "INACTIVO", "MANTENIMIENTO"]),
            "output": round(random.uniform(0, 100), 2),
            "temperature": round(random.uniform(20, 80), 1),
        }
        statuses.append(status)

    return statuses


def format_status_report(statuses):
    """Formatea el reporte de estado para enviar por WhatsApp."""
    now = datetime.datetime.now().strftime("%Y-%m-%d %H:%M:%S")
    lines = [f"REPORTE DE GENERADORES DE FORTUNA", f"Fecha: {now}", ""]

    for s in statuses:
        emoji = "OK" if s["state"] == "ACTIVO" else "ALERTA"
        lines.append(f"[{emoji}] {s['id']}")
        lines.append(f"  Estado: {s['state']}")
        lines.append(f"  Output: {s['output']}%")
        lines.append(f"  Temp: {s['temperature']}C")
        lines.append("")

    active = sum(1 for s in statuses if s["state"] == "ACTIVO")
    lines.append(f"Resumen: {active}/{len(statuses)} generadores activos")

    return "\n".join(lines)


def send_status_report():
    """Obtiene el estado y lo envia por WhatsApp."""
    print("Generando reporte de estado...")
    statuses = get_generators_status()
    report = format_status_report(statuses)
    send_whatsapp_message(report)
    print("Reporte enviado.")


def start_monitoring(interval_minutes=60):
    """Inicia el monitoreo periodico de los generadores."""
    print(f"Iniciando monitoreo cada {interval_minutes} minutos...")
    print("Enviando primer reporte...")
    send_status_report()

    schedule.every(interval_minutes).minutes.do(send_status_report)

    while True:
        schedule.run_pending()
        time.sleep(1)


if __name__ == "__main__":
    start_monitoring(interval_minutes=60)
