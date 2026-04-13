---
name: fetch-sitr-panama
description: Obtiene datos en tiempo real del Sistema Interconectado Nacional (SIN) de Panama desde sitr.cnd.com.pa incluyendo generacion, demanda, frecuencia, reserva, plantas importantes (Fortuna, Bayano, Changuinola, Gatun, Cobre Panama, Costa Norte) y niveles de embalses.
---

# Fetch SITR Panama

## Instrucciones

Cuando el usuario pida un reporte del SIN de Panama, estado del sistema electrico panameno, datos de generacion, demanda, frecuencia, reserva operativa, plantas hidroelectricas, termicas, niveles de embalses, o palabras clave como "SITR", "CND", "Panama electrico", "Fortuna", "Bayano", "Changuinola":

1. Llama a la herramienta `run_js` con `index.html` y un string JSON vacio `{}` como `data`.
2. La herramienta devolvera un objeto JSON con todos los datos del SIN.
3. Con esos datos, genera un analisis operativo en este formato:

## RESUMEN EJECUTIVO
Dos o tres lineas sobre el estado general del sistema y el factor mas critico del momento.

## RIESGOS DETECTADOS
- Plantas importantes offline (Fortuna 1/2/3, Bayano 1/2/3, Changuinola 1/2/3, Gatun, Cobre Panama, Costa Norte)
- Reserva rodante baja (menos de 200 MW es critico, menos de 150 MW es alarma)
- Frecuencia fuera de rango (normal 59.95 a 60.05 Hz)
- Embalses bajos (menos de 30% es preocupante en epoca seca)
- Desbalance entre generacion y demanda

## CONTEXTO OPERATIVO
- Hora del reporte y tipo de carga esperada: punta (10-14h, 18-22h), valle (00-05h), resto
- Valores tipicos del SIN Panama: demanda pico 1800-2200 MW, reserva objetivo 200-300 MW
- Epoca del ano: seca (dic-abr) limita hidroelectricas; lluviosa (may-nov) favorece embalses

## RECOMENDACIONES
Acciones operativas sugeridas y puntos de atencion para las proximas horas.

## Reglas estrictas

- NO inventes datos. Si un campo devuelve 0 o vacio, dilo explicitamente.
- Se conciso y directo.
- Usa terminologia tecnica del sector electrico.
- Si todo esta normal, dilo claramente sin inventar problemas.
- Responde siempre en espanol.
