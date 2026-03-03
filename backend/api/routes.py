"""
FastAPI routes for Panama Power Grid Dashboard.

Provides REST API endpoints and WebSocket connections for:
- Real-time power flow results
- Stability analysis
- Market data
- Scenario simulation
"""

import asyncio
import json
from copy import deepcopy
from datetime import datetime

from fastapi import APIRouter, WebSocket, WebSocketDisconnect, Query
from fastapi.responses import JSONResponse

from ..engine.network_model import build_panama_network, BusType, get_generation_by_fuel
from ..engine.power_flow import newton_raphson_power_flow, run_power_flow
from ..engine.stability import (
    voltage_stability_analysis,
    vq_sensitivity_analysis,
    contingency_analysis,
    frequency_stability_assessment,
    transfer_capability_analysis,
    run_full_stability_assessment,
)
from ..engine.market import (
    calculate_lmp,
    merit_order_dispatch,
    siepac_export_analysis,
    demand_scenario_analysis,
    run_full_market_analysis,
)
from ..data.scraper import get_simulator
from ..data.cnd_collector import get_collector
from ..data.cache import get_cache

router = APIRouter()

# Data source labels used throughout the API and WebSocket
SOURCE_LIVE = "LIVE_CND"
SOURCE_CACHE = "CACHE"
SOURCE_SIMULATOR = "SIMULATOR"


def _get_realtime_generation():
    """
    Get generation data: prefer real CND data, fallback to simulator.

    Always tags the response with an explicit data_source label
    (LIVE_CND / CACHE / SIMULATOR) and includes degraded-state
    diagnostics when falling back.
    """
    collector = get_collector()
    status = collector.get_status()

    # Priority 1: Live CND data
    live = collector.get_latest()
    if live is not None:
        live["data_source"] = SOURCE_LIVE
        live["data_source_detail"] = {
            "label": SOURCE_LIVE,
            "degraded": False,
            "last_fetch": status["last_fetch"],
            "cnd_timestamp": live.get("timestamp"),
        }
        return live

    # Priority 2: SQLite cache
    cache = get_cache()
    cached = cache.get_latest()
    if cached is not None:
        cached["data_source"] = SOURCE_CACHE
        cached["data_source_detail"] = {
            "label": SOURCE_CACHE,
            "degraded": True,
            "reason": "CND live data unavailable, serving from cache",
            "last_live_fetch": status.get("last_live_fetch"),
            "last_error": status.get("last_error"),
            "last_error_time": status.get("last_error_time"),
            "consecutive_errors": status.get("consecutive_errors", 0),
        }
        return cached

    # Priority 3: Simulator (degraded mode)
    sim = get_simulator()
    data = sim.get_generation_data()
    data["data_source"] = SOURCE_SIMULATOR
    data["data_source_detail"] = {
        "label": SOURCE_SIMULATOR,
        "degraded": True,
        "reason": "No CND data available (live or cached). Using simulated data.",
        "last_error": status.get("last_error"),
        "last_error_time": status.get("last_error_time"),
        "last_http_status": status.get("last_http_status"),
        "consecutive_errors": status.get("consecutive_errors", 0),
        "error_count": status.get("error_count", 0),
    }
    return data


# ============================================================
# POWER FLOW ENDPOINTS
# ============================================================

@router.get("/api/power-flow")
async def get_power_flow():
    """Run power flow analysis and return results."""
    result = run_power_flow()
    return JSONResponse(content=result)


@router.get("/api/power-flow/scenario")
async def get_power_flow_scenario(
    load_factor: float = Query(1.0, ge=0.3, le=2.0, description="Load scaling factor"),
    hydro_factor: float = Query(1.0, ge=0.0, le=1.5, description="Hydro availability factor"),
    siepac_mw: float = Query(0.0, ge=-300, le=300, description="SIEPAC flow (+ import, - export)"),
    wind_factor: float = Query(1.0, ge=0.0, le=1.5, description="Wind capacity factor"),
    solar_factor: float = Query(1.0, ge=0.0, le=1.5, description="Solar capacity factor"),
):
    """Run power flow with custom scenario parameters."""
    network = build_panama_network()
    buses = network["buses"]
    branches = network["branches"]
    generators = network["generators"]

    # Apply load scaling
    for bus in buses:
        bus.p_load_mw *= load_factor
        bus.q_load_mvar *= load_factor

    # Apply hydro factor to hydro generators
    for gen in generators:
        if gen.fuel_type == "Hydro":
            gen.p_max_mw *= hydro_factor

    # Apply wind/solar factors
    for gen in generators:
        if gen.fuel_type == "Wind":
            gen.p_max_mw *= wind_factor
        elif gen.fuel_type == "Solar":
            gen.p_max_mw *= solar_factor

    # Apply SIEPAC flow
    siepac_bus = next(b for b in buses if b.id == 26)
    if siepac_mw > 0:
        siepac_bus.p_gen_mw = siepac_mw
    else:
        siepac_bus.p_load_mw = abs(siepac_mw)

    # Rebalance slack
    total_load = sum(b.p_load_mw for b in buses)
    total_gen = sum(b.p_gen_mw for b in buses)
    slack = next(b for b in buses if b.bus_type == BusType.SLACK)
    slack.p_gen_mw += (total_load - total_gen)

    result = newton_raphson_power_flow(buses, branches)
    result["scenario"] = {
        "load_factor": load_factor,
        "hydro_factor": hydro_factor,
        "siepac_mw": siepac_mw,
        "wind_factor": wind_factor,
        "solar_factor": solar_factor,
    }

    return JSONResponse(content=result)


# ============================================================
# NETWORK MODEL ENDPOINTS
# ============================================================

@router.get("/api/network")
async def get_network():
    """Get the full network model (buses, branches, generators)."""
    network = build_panama_network()

    buses = [{
        "id": b.id, "name": b.name, "type": b.bus_type.name,
        "substation_type": b.substation_type.value,
        "voltage_kv": b.voltage_kv, "p_gen_mw": b.p_gen_mw,
        "p_load_mw": b.p_load_mw, "latitude": b.latitude,
        "longitude": b.longitude, "province": b.province,
    } for b in network["buses"]]

    branches = [{
        "id": br.id, "name": br.name, "from_bus": br.from_bus,
        "to_bus": br.to_bus, "rate_mva": br.rate_mva,
        "length_km": br.length_km, "circuits": br.circuits,
    } for br in network["branches"]]

    generators = [{
        "id": g.id, "name": g.name, "bus_id": g.bus_id,
        "fuel_type": g.fuel_type, "p_max_mw": g.p_max_mw,
        "cost_per_mwh": g.cost_per_mwh,
    } for g in network["generators"]]

    fuel_mix = get_generation_by_fuel(network["generators"])
    fuel_mix_serializable = {k: v for k, v in fuel_mix.items()}

    return JSONResponse(content={
        "buses": buses,
        "branches": branches,
        "generators": generators,
        "fuel_mix": fuel_mix_serializable,
        "total_capacity_mw": sum(g.p_max_mw for g in network["generators"]),
        "total_load_mw": sum(b.p_load_mw for b in network["buses"]),
    })


# ============================================================
# STABILITY ANALYSIS ENDPOINTS
# ============================================================

@router.get("/api/stability/voltage")
async def get_voltage_stability():
    """Run voltage stability analysis (P-V curves)."""
    network = build_panama_network()
    result = voltage_stability_analysis(
        network["buses"], network["branches"],
        load_increase_steps=15, max_increase_pct=60
    )
    return JSONResponse(content=result)


@router.get("/api/stability/vq-sensitivity")
async def get_vq_sensitivity():
    """Get V-Q sensitivity analysis."""
    network = build_panama_network()
    result = vq_sensitivity_analysis(network["buses"], network["branches"])
    return JSONResponse(content=result)


@router.get("/api/stability/contingency")
async def get_contingency_analysis():
    """Run N-1 contingency analysis."""
    network = build_panama_network()
    result = contingency_analysis(network["buses"], network["branches"])
    return JSONResponse(content=result)


@router.get("/api/stability/frequency")
async def get_frequency_stability(
    disturbance_mw: float = Query(380, ge=50, le=500, description="Generation loss in MW"),
):
    """Analyze frequency stability after generation trip."""
    network = build_panama_network()
    result = frequency_stability_assessment(
        network["buses"], network["generators"], disturbance_mw
    )
    return JSONResponse(content=result)


@router.get("/api/stability/transfer")
async def get_transfer_capability(
    direction: str = Query("west-east", description="Transfer direction: west-east or siepac"),
):
    """Analyze transfer capability between areas."""
    network = build_panama_network()

    if direction == "siepac":
        from_buses = [26]  # SIEPAC
        to_buses = [1, 2, 18, 27]  # Panama metro
    else:
        from_buses = [22, 15, 9, 8]  # Western hydro
        to_buses = [1, 2, 18, 27]  # Panama metro

    result = transfer_capability_analysis(
        network["buses"], network["branches"], from_buses, to_buses
    )
    return JSONResponse(content=result)


# ============================================================
# MARKET ANALYSIS ENDPOINTS
# ============================================================

@router.get("/api/market/lmp")
async def get_lmp():
    """Calculate Locational Marginal Prices."""
    network = build_panama_network()
    result = calculate_lmp(
        network["buses"], network["branches"], network["generators"]
    )
    return JSONResponse(content=result)


@router.get("/api/market/dispatch")
async def get_dispatch(
    demand_mw: float = Query(None, ge=500, le=4000, description="Total demand in MW"),
):
    """Get merit order dispatch for given demand."""
    network = build_panama_network()
    if demand_mw is None:
        demand_mw = sum(b.p_load_mw for b in network["buses"])
    result = merit_order_dispatch(network["generators"], demand_mw)
    return JSONResponse(content=result)


@router.get("/api/market/siepac")
async def get_siepac_analysis():
    """Analyze SIEPAC export/import scenarios."""
    network = build_panama_network()
    result = siepac_export_analysis(
        network["buses"], network["branches"], network["generators"]
    )
    return JSONResponse(content=result)


@router.get("/api/market/scenarios")
async def get_demand_scenarios():
    """Analyze multiple demand scenarios."""
    network = build_panama_network()
    result = demand_scenario_analysis(
        network["buses"], network["branches"], network["generators"]
    )
    return JSONResponse(content=result)


# ============================================================
# REAL-TIME DATA ENDPOINTS
# ============================================================

@router.get("/api/realtime/conditions")
async def get_realtime_conditions():
    """Get simulated real-time system conditions."""
    sim = get_simulator()
    return JSONResponse(content=sim.get_current_conditions())


@router.get("/api/realtime/generation")
async def get_realtime_generation():
    """Get real-time generation data (LIVE_CND > CACHE > SIMULATOR)."""
    data = _get_realtime_generation()
    return JSONResponse(content=data)


@router.get("/api/realtime/market")
async def get_realtime_market():
    """Get simulated real-time market data."""
    sim = get_simulator()
    return JSONResponse(content=sim.get_market_data())


@router.get("/api/realtime/voltages")
async def get_realtime_voltages():
    """Get simulated real-time bus voltages."""
    sim = get_simulator()
    return JSONResponse(content=sim.get_bus_voltages())


@router.get("/api/realtime/history")
async def get_load_history(hours: int = Query(24, ge=1, le=168)):
    """Get historical load data (from cache if available, else simulator)."""
    cache = get_cache()
    history = cache.get_history(hours)
    if history:
        return JSONResponse(content={
            "source": SOURCE_CACHE,
            "data": history,
        })
    sim = get_simulator()
    return JSONResponse(content={
        "source": SOURCE_SIMULATOR,
        "data": sim.get_historical_load(hours),
    })


# ============================================================
# CND DATA ENDPOINTS
# ============================================================

@router.get("/api/cnd/generation")
async def get_cnd_generation():
    """Get the latest CND generation data (detailed by plant)."""
    collector = get_collector()
    raw = collector.get_latest_raw()
    if raw:
        return JSONResponse(content={
            "source": "CND/SITR (sitr.cnd.com.pa)",
            "data": raw,
        })
    return JSONResponse(
        status_code=503,
        content={"error": "No CND data available yet", "hint": "Collector may still be starting"},
    )


@router.get("/api/cnd/generation/fortuna")
async def get_cnd_fortuna():
    """Get Fortuna hydroelectric units detail."""
    collector = get_collector()
    raw = collector.get_latest_raw()
    if raw and raw.get("hidro"):
        fortuna = {k: v for k, v in raw["hidro"].items() if "fortuna" in k.lower()}
        return JSONResponse(content={
            "source": "CND/SITR",
            "timestamp": raw.get("timestamp"),
            "fortuna_units": fortuna,
            "total_fortuna_mw": round(sum(fortuna.values()), 1),
        })
    return JSONResponse(
        status_code=503,
        content={"error": "No CND data available"},
    )


@router.get("/api/cnd/flow")
async def get_cnd_flow():
    """Get the latest Flujo Occidente data from flow.html."""
    collector = get_collector()
    flow = collector.get_latest_flow()
    if flow:
        return JSONResponse(content={
            "source": "CND/SITR (sitr.cnd.com.pa)",
            "data": flow,
        })
    return JSONResponse(
        status_code=503,
        content={"error": "No flow data available yet"},
    )


@router.get("/api/cnd/history")
async def get_cnd_history(hours: int = Query(24, ge=1, le=168)):
    """Get historical generation from the SQLite cache."""
    cache = get_cache()
    history = cache.get_history(hours)
    return JSONResponse(content={
        "source": SOURCE_CACHE,
        "snapshots": len(history),
        "data": history,
    })


@router.get("/api/cnd/status")
async def get_cnd_status():
    """Get the CND collector health status (full observability)."""
    collector = get_collector()
    cache = get_cache()
    status = collector.get_status()
    status["cache_snapshots"] = cache.get_snapshot_count()
    return JSONResponse(content=status)


# ============================================================
# WEBSOCKET FOR REAL-TIME UPDATES
# ============================================================

class ConnectionManager:
    def __init__(self):
        self.active_connections: list[WebSocket] = []

    async def connect(self, websocket: WebSocket):
        await websocket.accept()
        self.active_connections.append(websocket)

    def disconnect(self, websocket: WebSocket):
        self.active_connections.remove(websocket)

    async def broadcast(self, message: dict):
        for connection in self.active_connections:
            try:
                await connection.send_json(message)
            except Exception:
                pass


manager = ConnectionManager()


@router.websocket("/ws/realtime")
async def websocket_realtime(websocket: WebSocket):
    """WebSocket endpoint for real-time dashboard updates."""
    await manager.connect(websocket)
    sim = get_simulator()

    try:
        while True:
            # Use real CND data for generation if available
            generation = _get_realtime_generation()

            # These still come from the simulator (market, voltages, conditions)
            conditions = sim.get_current_conditions()
            market = sim.get_market_data()
            voltages = sim.get_bus_voltages()

            # If CND data is live, override demand with real total
            if generation.get("data_source") == SOURCE_LIVE:
                conditions["current_demand_mw"] = generation.get("total_demand_mw", conditions["current_demand_mw"])

            # Collector status for frontend observability
            collector = get_collector()
            collector_status = collector.get_status()

            # Flow data (Flujo Occidente)
            flow_data = collector.get_latest_flow()

            update = {
                "type": "realtime_update",
                "timestamp": datetime.now().isoformat(),
                "data_source": generation.get("data_source", SOURCE_SIMULATOR),
                "data_source_detail": generation.get("data_source_detail", {}),
                "cnd_timestamp": generation.get("timestamp"),
                "conditions": conditions,
                "generation": generation,
                "market": market,
                "voltages": voltages,
                "flow": flow_data,
                "collector_status": {
                    "consecutive_errors": collector_status.get("consecutive_errors", 0),
                    "last_error": collector_status.get("last_error"),
                    "last_error_time": collector_status.get("last_error_time"),
                    "last_http_status": collector_status.get("last_http_status"),
                    "last_live_fetch": collector_status.get("last_live_fetch"),
                    "fetch_count": collector_status.get("fetch_count", 0),
                },
            }

            await websocket.send_json(update)
            await asyncio.sleep(5)

    except WebSocketDisconnect:
        manager.disconnect(websocket)
    except Exception:
        manager.disconnect(websocket)
