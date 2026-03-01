"""
Electricity Market Analysis Module.

Includes:
- Marginal cost calculation (Locational Marginal Prices - LMP)
- Merit order dispatch optimization
- SIEPAC import/export economics
- Demand scenario simulation
"""

import numpy as np
from copy import deepcopy
from .network_model import build_panama_network, BusType
from .power_flow import newton_raphson_power_flow


def calculate_lmp(buses, branches, generators):
    """
    Calculate Locational Marginal Prices (LMP) at each bus.

    LMP = Energy component + Congestion component + Loss component

    Uses sensitivity-based approach with power flow results.
    """
    # Run base power flow
    result = newton_raphson_power_flow(buses, branches)
    if not result["converged"]:
        return {"error": "Base power flow did not converge"}

    # Determine system marginal cost from merit order
    sorted_gens = sorted(generators, key=lambda g: g.cost_per_mwh)
    total_load = sum(b.p_load_mw for b in buses)

    cumulative_gen = 0
    system_lambda = 0  # System marginal cost

    for gen in sorted_gens:
        if gen.fuel_type == "Interconnection":
            continue
        cumulative_gen += gen.p_max_mw
        system_lambda = gen.cost_per_mwh
        if cumulative_gen >= total_load:
            break

    # Calculate loss sensitivity factors using perturbation
    lmp_results = []
    base_loss = result["total_p_loss_mw"]

    for bus in buses:
        bus_result = next(br for br in result["buses"] if br["id"] == bus.id)

        # Congestion component (based on line loading near the bus)
        congestion_cost = 0
        for lf in result["line_flows"]:
            if lf["from_bus"] == bus.id or lf["to_bus"] == bus.id:
                if lf["loading_pct"] > 80:
                    congestion_cost += (lf["loading_pct"] - 80) * 0.5  # $/MWh penalty

        # Loss component (approximation based on distance from generation center)
        loss_factor = 0
        if bus.p_load_mw > 0:
            # Approximate loss penalty factor
            v_deviation = abs(1.0 - bus_result["v_mag_pu"])
            loss_factor = v_deviation * system_lambda * 2

        lmp = system_lambda + congestion_cost + loss_factor

        lmp_results.append({
            "bus_id": bus.id,
            "name": bus.name,
            "lmp_usd_mwh": round(lmp, 2),
            "energy_component": round(system_lambda, 2),
            "congestion_component": round(congestion_cost, 2),
            "loss_component": round(loss_factor, 2),
            "province": bus.province,
            "p_load_mw": bus.p_load_mw,
            "voltage_pu": bus_result["v_mag_pu"],
        })

    # Weighted average LMP
    total_load_cost = sum(r["lmp_usd_mwh"] * r["p_load_mw"] for r in lmp_results)
    if total_load > 0:
        avg_lmp = total_load_cost / total_load
    else:
        avg_lmp = system_lambda

    return {
        "system_lambda_usd_mwh": system_lambda,
        "average_lmp_usd_mwh": round(avg_lmp, 2),
        "bus_lmps": lmp_results,
        "total_load_mw": total_load,
        "total_loss_mw": base_loss,
    }


def merit_order_dispatch(generators, total_demand_mw):
    """
    Economic dispatch using merit order (price-based stacking).

    Returns the optimal generation schedule and total cost.
    """
    sorted_gens = sorted(generators, key=lambda g: g.cost_per_mwh)

    dispatch = []
    remaining_demand = total_demand_mw
    total_cost = 0

    for gen in sorted_gens:
        if gen.fuel_type == "Interconnection":
            continue

        if remaining_demand <= 0:
            dispatch.append({
                "name": gen.name,
                "fuel": gen.fuel_type,
                "dispatch_mw": 0,
                "p_max_mw": gen.p_max_mw,
                "cost_per_mwh": gen.cost_per_mwh,
                "total_cost_usd_h": 0,
                "status": "standby",
            })
            continue

        dispatched = min(gen.p_max_mw, remaining_demand)
        cost = dispatched * gen.cost_per_mwh

        dispatch.append({
            "name": gen.name,
            "fuel": gen.fuel_type,
            "dispatch_mw": round(dispatched, 1),
            "p_max_mw": gen.p_max_mw,
            "cost_per_mwh": gen.cost_per_mwh,
            "total_cost_usd_h": round(cost, 2),
            "status": "dispatched" if dispatched > 0 else "standby",
        })

        remaining_demand -= dispatched
        total_cost += cost

    # Check if demand is met
    unserved = max(0, remaining_demand)

    return {
        "total_demand_mw": total_demand_mw,
        "total_dispatched_mw": round(total_demand_mw - unserved, 1),
        "unserved_mw": round(unserved, 1),
        "total_cost_usd_h": round(total_cost, 2),
        "marginal_cost_usd_mwh": dispatch[-1]["cost_per_mwh"] if dispatch else 0,
        "dispatch": dispatch,
    }


def siepac_export_analysis(buses, branches, generators, export_scenarios_mw=None):
    """
    Analyze SIEPAC export/import scenarios.

    Evaluates the economic and technical feasibility of different
    levels of energy export through the SIEPAC interconnection.
    """
    if export_scenarios_mw is None:
        export_scenarios_mw = [-300, -200, -100, -50, 0, 50, 100, 200, 300]

    results = []
    siepac_bus_id = 26  # Frontera Costa Rica

    for export_mw in export_scenarios_mw:
        modified_buses = deepcopy(buses)

        # Positive = export FROM Panama, Negative = import TO Panama
        siepac_bus = next(b for b in modified_buses if b.id == siepac_bus_id)
        if export_mw > 0:
            siepac_bus.p_load_mw = export_mw  # Export acts as load at border
        else:
            siepac_bus.p_gen_mw = abs(export_mw)  # Import acts as generation

        # Adjust slack to balance
        total_load = sum(b.p_load_mw for b in modified_buses)
        total_gen = sum(b.p_gen_mw for b in modified_buses)

        slack_bus = next(b for b in modified_buses if b.bus_type == BusType.SLACK)
        slack_bus.p_gen_mw += (total_load - total_gen)

        pf_result = newton_raphson_power_flow(modified_buses, branches)

        # Calculate economics
        if export_mw > 0:
            # Revenue from export (assume MER price)
            mer_price = 55.0  # USD/MWh average MER price
            revenue = export_mw * mer_price
        else:
            # Cost of import
            mer_price = 55.0
            revenue = export_mw * mer_price  # Negative = cost

        # Internal generation cost change
        internal_demand = total_load - max(0, -export_mw)
        dispatch = merit_order_dispatch(generators, internal_demand + max(0, export_mw))

        results.append({
            "export_mw": export_mw,
            "direction": "export" if export_mw > 0 else ("import" if export_mw < 0 else "balanced"),
            "converged": pf_result["converged"],
            "total_loss_mw": pf_result.get("total_p_loss_mw", 0),
            "revenue_usd_h": round(revenue, 2),
            "generation_cost_usd_h": dispatch["total_cost_usd_h"],
            "net_benefit_usd_h": round(revenue - dispatch["total_cost_usd_h"], 2),
            "marginal_cost_usd_mwh": dispatch["marginal_cost_usd_mwh"],
        })

    # Find optimal export level
    feasible = [r for r in results if r["converged"]]
    if feasible:
        optimal = max(feasible, key=lambda x: x["net_benefit_usd_h"])
    else:
        optimal = None

    return {
        "scenarios": results,
        "optimal_export": optimal,
        "mer_reference_price_usd_mwh": 55.0,
    }


def demand_scenario_analysis(buses, branches, generators):
    """
    Analyze different demand scenarios:
    - Peak demand (summer/dry season)
    - Valley demand (nighttime)
    - Shoulder demand (typical weekday)
    - High growth scenario
    - Climate event (drought - reduced hydro)
    """
    base_load = sum(b.p_load_mw for b in buses)

    scenarios = {
        "peak": {
            "name": "Demanda Pico (Verano)",
            "load_factor": 1.15,
            "hydro_factor": 0.7,  # Dry season, less hydro
            "description": "Período seco con alta demanda",
        },
        "valley": {
            "name": "Demanda Valle (Nocturno)",
            "load_factor": 0.55,
            "hydro_factor": 1.0,
            "description": "Demanda mínima nocturna",
        },
        "shoulder": {
            "name": "Demanda Media (Día laboral)",
            "load_factor": 0.85,
            "hydro_factor": 0.9,
            "description": "Día laboral típico",
        },
        "high_growth": {
            "name": "Crecimiento Alto (+20%)",
            "load_factor": 1.20,
            "hydro_factor": 0.85,
            "description": "Escenario de alto crecimiento económico",
        },
        "drought": {
            "name": "Sequía Extrema",
            "load_factor": 1.0,
            "hydro_factor": 0.3,  # Severe drought
            "description": "El Niño - reducción severa de generación hidro",
        },
        "wet_season": {
            "name": "Temporada Lluviosa",
            "load_factor": 0.90,
            "hydro_factor": 1.2,  # Abundant hydro
            "description": "Temporada lluviosa con abundante generación hidro",
        },
    }

    results = {}

    for scenario_key, scenario in scenarios.items():
        modified_buses = deepcopy(buses)
        modified_gens = deepcopy(generators)

        # Scale loads
        for bus in modified_buses:
            bus.p_load_mw *= scenario["load_factor"]
            bus.q_load_mvar *= scenario["load_factor"]

        # Scale hydro generation
        for gen in modified_gens:
            if gen.fuel_type == "Hydro":
                gen.p_max_mw *= scenario["hydro_factor"]

        # Run economic dispatch
        scenario_load = sum(b.p_load_mw for b in modified_buses)
        dispatch = merit_order_dispatch(modified_gens, scenario_load)

        # Apply dispatch to buses for power flow
        for bus in modified_buses:
            for gen in modified_gens:
                if gen.bus_id == bus.id:
                    disp_entry = next((d for d in dispatch["dispatch"] if d["name"] == gen.name), None)
                    if disp_entry:
                        bus.p_gen_mw = disp_entry["dispatch_mw"]

        # Ensure slack balance
        total_gen_dispatched = sum(b.p_gen_mw for b in modified_buses)
        slack = next(b for b in modified_buses if b.bus_type == BusType.SLACK)
        slack.p_gen_mw += (scenario_load - total_gen_dispatched)

        # Run power flow
        pf_result = newton_raphson_power_flow(modified_buses, branches)

        # Identify issues
        voltage_violations = []
        overloads = []
        if pf_result["converged"]:
            for br in pf_result["buses"]:
                if br["v_mag_pu"] < 0.95 or br["v_mag_pu"] > 1.05:
                    voltage_violations.append({
                        "name": br["name"],
                        "voltage_pu": br["v_mag_pu"],
                    })
            for lf in pf_result["line_flows"]:
                if lf["loading_pct"] > 80:
                    overloads.append({
                        "name": lf["name"],
                        "loading_pct": lf["loading_pct"],
                    })

        # Generation mix
        fuel_mix = {}
        for d in dispatch["dispatch"]:
            fuel = d["fuel"]
            if fuel not in fuel_mix:
                fuel_mix[fuel] = 0
            fuel_mix[fuel] += d["dispatch_mw"]

        results[scenario_key] = {
            "name": scenario["name"],
            "description": scenario["description"],
            "total_demand_mw": round(scenario_load, 1),
            "load_factor": scenario["load_factor"],
            "converged": pf_result["converged"],
            "total_loss_mw": round(pf_result.get("total_p_loss_mw", 0), 1),
            "total_cost_usd_h": dispatch["total_cost_usd_h"],
            "marginal_cost_usd_mwh": dispatch["marginal_cost_usd_mwh"],
            "unserved_mw": dispatch["unserved_mw"],
            "fuel_mix_mw": fuel_mix,
            "voltage_violations": voltage_violations,
            "overloads": overloads,
            "dispatch": dispatch["dispatch"],
        }

    return {"base_load_mw": base_load, "scenarios": results}


def run_full_market_analysis():
    """Run complete market analysis."""
    network = build_panama_network()
    buses = network["buses"]
    branches = network["branches"]
    generators = network["generators"]

    lmp = calculate_lmp(buses, branches, generators)
    total_load = sum(b.p_load_mw for b in buses)
    dispatch = merit_order_dispatch(generators, total_load)
    siepac = siepac_export_analysis(buses, branches, generators)
    scenarios = demand_scenario_analysis(buses, branches, generators)

    return {
        "lmp": lmp,
        "dispatch": dispatch,
        "siepac": siepac,
        "demand_scenarios": scenarios,
    }
