# Panama Power Grid Dashboard - SIN 230kV

Sistema de Analisis en Tiempo Real de Flujos de Potencia del Sistema Interconectado Nacional (SIN) de Panama a 230kV.

## Funcionalidades

### Analisis de Flujo de Potencia
- Motor Newton-Raphson completo para analisis AC
- 27 nodos del SIN a 230kV (subestaciones reductoras, seccionadoras, GIS, generacion)
- 30 lineas de transmision modeladas con impedancias reales
- 14 generadores (hidro, gas, carbon, eolica, solar, SIEPAC)
- Escenarios personalizados (carga, hidro, viento, solar, SIEPAC)

### Estabilidad Electrica
- **Contingencia N-1**: Analisis completo removiendo cada linea
- **Estabilidad de Voltaje**: Curvas P-V con punto de colapso
- **Sensibilidad V-Q**: Identificacion de nodos vulnerables
- **Estabilidad de Frecuencia**: Respuesta ante perdida de generacion
- **Capacidad de Transferencia**: Limite Oeste-Este y SIEPAC

### Mercado Electrico
- **Precios Marginales Locales (LMP)**: Energia + Congestion + Perdidas
- **Despacho por Orden de Merito**: Optimizacion economica
- **Analisis SIEPAC**: Export/import con Costa Rica y MER
- **Precios Regionales MER**: Guatemala a Panama

### Escenarios de Demanda
- Pico (verano/sequia)
- Valle (nocturno)
- Media (dia laboral)
- Alto crecimiento (+20%)
- Sequia extrema (El Nino)
- Temporada lluviosa

### Dashboard en Tiempo Real
- Mapa interactivo con Leaflet (todos los nodos y lineas)
- WebSocket para actualizaciones cada 5 segundos
- Graficos dinamicos con Chart.js
- Controles de escenario interactivos
- Alertas del sistema en tiempo real

## Nodos del SIN 230kV

| # | Subestacion | Tipo | Provincia |
|---|-------------|------|-----------|
| 1 | Panama | Reductora (Slack) | Panama |
| 2 | Panama II | Reductora | Panama |
| 3 | Chorrera | Reductora | Panama Oeste |
| 4 | Llano Sanchez | Reductora | Cocle |
| 5 | Mata de Nance | Reductora | Chiriqui |
| 6 | Progreso | Interconexion | Chiriqui |
| 7 | Charco Azul | Reductora | Chiriqui |
| 8 | Changuinola | Reductora | Bocas del Toro |
| 9 | Caldera | Reductora | Chiriqui |
| 10 | Boqueron I | Reductora | Chiriqui |
| 11 | San Bartolo | Reductora | Veraguas |
| 12 | Canazas | Reductora | Veraguas |
| 13 | Caceres | Seccionadora | Cocle |
| 14 | Santa Rita | Seccionadora | Cocle |
| 15 | Guasquitas | Seccionadora | Chiriqui |
| 16 | Veladero | Seccionadora | Veraguas |
| 17 | El Higo | Seccionadora | Cocle |
| 18 | Panama III | GIS | Panama |
| 19 | Sabanitas | GIS | Colon |
| 20 | Burunga | GIS | Panama Oeste |
| 21 | Chiriqui Grande | Reductora | Bocas del Toro |
| 22 | Fortuna | Generacion | Chiriqui |
| 23 | Bayano | Generacion | Panama |
| 24 | BLM (Bahia Las Minas) | Generacion | Colon |
| 25 | Costa Norte (AES/Gatun) | Generacion | Colon |
| 26 | Frontera Costa Rica (SIEPAC) | Interconexion | Chiriqui |
| 27 | 24 de Diciembre | Reductora | Panama |

## Instalacion

```bash
pip install -r requirements.txt
python main.py
```

Abrir en navegador: http://localhost:8000

API docs: http://localhost:8000/docs

## Arquitectura

```
backend/
  engine/
    network_model.py   # Modelo completo del SIN 230kV
    power_flow.py      # Motor Newton-Raphson
    stability.py       # Analisis de estabilidad
    market.py          # Analisis de mercado electrico
  data/
    scraper.py         # Ingestion de datos CND/EOR + simulacion
  api/
    routes.py          # FastAPI endpoints + WebSocket
frontend/
  index.html           # Dashboard principal
  css/dashboard.css    # Estilos
  js/dashboard.js      # Logica del dashboard
config/
  settings.py          # Configuracion del sistema
main.py                # Punto de entrada
```

## Fuentes de Datos

- [CND - Centro Nacional de Despacho](http://www.cnd.com.pa/) - Datos en tiempo real (SITR)
- [ETESA](https://www.etesa.com.pa/) - Plan de expansion y topologia
- [EOR - Ente Operador Regional](https://www.enteoperador.org/) - Precios nodales MER
- [ASEP PESIN](https://asep.gob.pa/) - Plan de expansion del sistema de transmision
