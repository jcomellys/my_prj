"""
Stability Analysis Module for Panama 230kV Power System.

Includes:
- Voltage stability analysis (P-V curves, V-Q sensitivity)
- N-1 contingency analysis
- Frequency stability estimation
- Transfer capability analysis
"""

import numpy as np
from copy import deepcopy
from .network_model import Bus, Branch, BusType, build_panama_network
from .power_flow import newton_raphson_power_flow, build_ybus

BASE_MVA = 100.0


def voltage_stability_analysis(buses, branches, load_increase_steps=20, max_increase_pct=80):
    """
    Perform voltage stability analysis using continuation power flow (P-V curves).

    Progressively increases load and records voltage profiles to identify
    the voltage collapse point (nose of the P-V curve).
    """
    pv_curves = {}
    for bus in buses:
        if bus.p_load_mw > 0:
            pv_curves[bus.id] = {
                "name": bus.name,
                "loading_pct": [],
                "voltage_pu": [],
                "province": bus.province,
            }

    collapse_point = None

    for step in range(load_increase_steps + 1):
        increase_factor = 1.0 + (max_increase_pct / 100.0) * (step / load_increase_steps)

        # Scale all loads
        scaled_buses = deepcopy(buses)
        for bus in scaled_buses:
            bus.p_load_mw *= increase_factor
            bus.q_load_mvar *= increase_factor

        # Also scale slack generation to balance
        total_load = sum(b.p_load_mw for b in scaled_buses)
        total_gen = sum(b.p_gen_mw for b in scaled_buses)
        slack_bus = next(b for b in scaled_buses if b.bus_type == BusType.SLACK)
        slack_bus.p_gen_mw += (total_load - total_gen) * 0.3  # Partial slack adjustment

        result = newton_raphson_power_flow(scaled_buses, branches, max_iter=100, tolerance=1e-5)

        if not result["converged"]:
            collapse_point = {
                "loading_pct": (increase_factor - 1) * 100,
                "step": step,
            }
            break

        loading_pct = (increase_factor - 1) * 100
        for bus_result in result["buses"]:
            if bus_result["id"] in pv_curves:
                pv_curves[bus_result["id"]]["loading_pct"].append(loading_pct)
                pv_curves[bus_result["id"]]["voltage_pu"].append(bus_result["v_mag_pu"])

    # Find weakest buses (lowest voltage at max loading)
    weakest_buses = []
    for bus_id, data in pv_curves.items():
        if data["voltage_pu"]:
            min_v = min(data["voltage_pu"])
            weakest_buses.append({
                "id": bus_id,
                "name": data["name"],
                "min_voltage_pu": min_v,
                "province": data["province"],
            })
    weakest_buses.sort(key=lambda x: x["min_voltage_pu"])

    return {
        "pv_curves": pv_curves,
        "collapse_point": collapse_point,
        "weakest_buses": weakest_buses[:10],
        "max_loading_tested_pct": max_increase_pct,
    }


def vq_sensitivity_analysis(buses, branches):
    """
    Compute V-Q sensitivity at each load bus.

    The V-Q sensitivity (dV/dQ) indicates how much voltage changes
    for a reactive power injection. Higher sensitivity = weaker bus.
    """
    Y, bus_idx = build_ybus(buses, branches)
    n = len(buses)

    # Get reduced B matrix for PQ buses
    pq_indices = [bus_idx[b.id] for b in buses if b.bus_type == BusType.PQ]

    if len(pq_indices) == 0:
        return {"sensitivities": []}

    B_reduced = np.imag(Y[np.ix_(pq_indices, pq_indices)])

    try:
        B_inv = np.linalg.inv(B_reduced)
    except np.linalg.LinAlgError:
        return {"sensitivities": [], "error": "Singular matrix"}

    sensitivities = []
    pq_buses = [b for b in buses if b.bus_type == BusType.PQ]

    for k, bus in enumerate(pq_buses):
        dv_dq = B_inv[k, k]
        sensitivities.append({
            "id": bus.id,
            "name": bus.name,
            "dv_dq": float(dv_dq),
            "province": bus.province,
            "vulnerable": dv_dq > 0.05,  # High sensitivity threshold
        })

    sensitivities.sort(key=lambda x: abs(x["dv_dq"]), reverse=True)

    return {"sensitivities": sensitivities}


def contingency_analysis(buses, branches):
    """
    N-1 contingency analysis.

    Removes each transmission line one at a time and runs power flow
    to identify critical contingencies.
    """
    # Base case
    base_result = newton_raphson_power_flow(buses, branches)
    if not base_result["converged"]:
        return {"error": "Base case did not converge", "contingencies": []}

    contingencies = []

    for br in branches:
        # Skip if already inactive
        if not br.is_active:
            continue

        # Create modified branch list with this branch out
        modified_branches = deepcopy(branches)
        for mod_br in modified_branches:
            if mod_br.id == br.id:
                mod_br.is_active = False
                break

        result = newton_raphson_power_flow(buses, modified_branches, max_iter=80, tolerance=1e-5)

        severity = "normal"
        violations = []

        if not result["converged"]:
            severity = "critical"
            violations.append("Power flow diverged - system instability")
        else:
            # Check voltage violations
            for bus_r in result["buses"]:
                if bus_r["v_mag_pu"] < 0.95:
                    severity = max(severity, "warning", key=lambda x: ["normal", "warning", "critical"].index(x))
                    violations.append(f"Low voltage at {bus_r['name']}: {bus_r['v_mag_pu']:.4f} pu")
                if bus_r["v_mag_pu"] > 1.05:
                    severity = max(severity, "warning", key=lambda x: ["normal", "warning", "critical"].index(x))
                    violations.append(f"High voltage at {bus_r['name']}: {bus_r['v_mag_pu']:.4f} pu")

            # Check overloads
            for lf in result["line_flows"]:
                if lf["loading_pct"] > 100:
                    severity = "critical"
                    violations.append(f"Overload on {lf['name']}: {lf['loading_pct']:.1f}%")
                elif lf["loading_pct"] > 80:
                    severity = max(severity, "warning", key=lambda x: ["normal", "warning", "critical"].index(x))
                    violations.append(f"High loading on {lf['name']}: {lf['loading_pct']:.1f}%")

        contingencies.append({
            "branch_id": br.id,
            "branch_name": br.name,
            "converged": result["converged"],
            "severity": severity,
            "violations": violations[:5],  # Limit to top 5 violations
            "total_loss_mw": result.get("total_p_loss_mw", 0),
        })

    # Sort by severity
    severity_order = {"critical": 0, "warning": 1, "normal": 2}
    contingencies.sort(key=lambda x: severity_order.get(x["severity"], 3))

    n_critical = sum(1 for c in contingencies if c["severity"] == "critical")
    n_warning = sum(1 for c in contingencies if c["severity"] == "warning")
    n_normal = sum(1 for c in contingencies if c["severity"] == "normal")

    return {
        "base_case_loss_mw": base_result["total_p_loss_mw"],
        "contingencies": contingencies,
        "summary": {
            "total": len(contingencies),
            "critical": n_critical,
            "warning": n_warning,
            "normal": n_normal,
            "n1_secure": n_critical == 0,
        },
    }


def frequency_stability_assessment(buses, generators, disturbance_mw=300):
    """
    Estimate frequency response after loss of largest generator.

    Uses swing equation approximation:
        df/dt = (P_mech - P_elec) / (2 * H_total * f0)
        f_nadir ≈ f0 - (disturbance * f0) / (2 * H_total * S_total)

    Parameters:
        disturbance_mw: Size of generation loss (MW)
    """
    f0 = 60.0  # Nominal frequency Hz

    # Calculate system inertia
    total_inertia_mj = 0
    total_gen_mva = 0

    gen_details = []
    for gen in generators:
        if gen.fuel_type == "Interconnection":
            continue
        if gen.p_max_mw > 0 and gen.inertia_constant_h > 0:
            # H is in MJ/MVA, we need total kinetic energy
            gen_mva = gen.p_max_mw / 0.85  # Approximate MVA rating
            ke = gen.inertia_constant_h * gen_mva  # MJ
            total_inertia_mj += ke
            total_gen_mva += gen_mva
            gen_details.append({
                "name": gen.name,
                "fuel": gen.fuel_type,
                "p_max_mw": gen.p_max_mw,
                "h_constant": gen.inertia_constant_h,
                "kinetic_energy_mj": ke,
            })

    if total_gen_mva == 0:
        return {"error": "No generators with inertia"}

    # System equivalent H
    h_system = total_inertia_mj / total_gen_mva

    # Rate of change of frequency (RoCoF)
    rocof = (disturbance_mw * f0) / (2 * total_inertia_mj)  # Hz/s

    # Frequency nadir (simplified estimation assuming governor response)
    # Assume primary frequency response starts at 0.5s with 5% droop
    t_nadir = 2.0  # seconds (approximate)
    governor_response_mw = disturbance_mw * 0.3  # 30% recovery by nadir

    f_nadir = f0 - (disturbance_mw - governor_response_mw) * f0 / (2 * total_inertia_mj) * t_nadir

    # Steady-state frequency
    total_droop_pct = 5.0
    total_capacity = sum(g.p_max_mw for g in generators if g.fuel_type != "Interconnection")
    f_ss = f0 - (disturbance_mw / total_capacity) * (total_droop_pct / 100) * f0

    # Generate time-domain approximation (simplified)
    t = np.linspace(0, 30, 300)
    f_response = np.zeros_like(t)
    for i, ti in enumerate(t):
        if ti < 0.1:
            f_response[i] = f0
        elif ti < t_nadir:
            # Inertial response phase
            gov_effect = governor_response_mw * (ti / t_nadir)
            f_response[i] = f0 - (disturbance_mw - gov_effect) * f0 / (2 * total_inertia_mj) * ti
        else:
            # Recovery phase (exponential approach to steady state)
            tau = 5.0  # Time constant
            f_response[i] = f_ss + (f_nadir - f_ss) * np.exp(-(ti - t_nadir) / tau)

    return {
        "disturbance_mw": disturbance_mw,
        "system_inertia_h": float(h_system),
        "total_kinetic_energy_mj": float(total_inertia_mj),
        "total_generation_mva": float(total_gen_mva),
        "rocof_hz_per_s": float(rocof),
        "frequency_nadir_hz": float(f_nadir),
        "steady_state_frequency_hz": float(f_ss),
        "nadir_time_s": t_nadir,
        "ufls_triggered": f_nadir < 59.0,  # Under-frequency load shedding
        "time_series": {
            "time_s": t.tolist(),
            "frequency_hz": f_response.tolist(),
        },
        "generators": gen_details,
    }


def transfer_capability_analysis(buses, branches, from_area_buses, to_area_buses, max_transfer_mw=500):
    """
    Analyze maximum transfer capability between two areas.

    Useful for SIEPAC export analysis and inter-regional transfers.
    """
    results = []
    transfer_limit = None

    for step in range(0, int(max_transfer_mw) + 1, 25):
        scaled_buses = deepcopy(buses)

        # Increase generation in from_area
        from_gen_buses = [b for b in scaled_buses if b.id in from_area_buses and b.p_gen_mw > 0]
        to_load_buses = [b for b in scaled_buses if b.id in to_area_buses and b.p_load_mw > 0]

        if not from_gen_buses or not to_load_buses:
            break

        # Distribute extra generation
        extra_per_gen = step / len(from_gen_buses)
        for bus in from_gen_buses:
            bus.p_gen_mw += extra_per_gen

        # Distribute extra load
        extra_per_load = step / len(to_load_buses)
        for bus in to_load_buses:
            bus.p_load_mw += extra_per_load

        result = newton_raphson_power_flow(scaled_buses, branches, max_iter=80, tolerance=1e-5)

        if not result["converged"]:
            transfer_limit = step - 25
            break

        # Check for violations
        has_violation = False
        for lf in result["line_flows"]:
            if lf["loading_pct"] > 100:
                has_violation = True
                break
        for br in result["buses"]:
            if br["v_mag_pu"] < 0.90 or br["v_mag_pu"] > 1.10:
                has_violation = True
                break

        results.append({
            "transfer_mw": step,
            "converged": True,
            "has_violation": has_violation,
            "total_loss_mw": result["total_p_loss_mw"],
        })

        if has_violation:
            transfer_limit = step
            break

    if transfer_limit is None and results:
        transfer_limit = max_transfer_mw

    return {
        "max_transfer_mw": transfer_limit,
        "steps": results,
    }


def run_full_stability_assessment():
    """Run complete stability assessment of the Panama 230kV system."""
    network = build_panama_network()
    buses = network["buses"]
    branches = network["branches"]
    generators = network["generators"]

    # Voltage stability
    vs = voltage_stability_analysis(buses, branches, load_increase_steps=15, max_increase_pct=60)

    # V-Q sensitivity
    vq = vq_sensitivity_analysis(buses, branches)

    # N-1 contingency
    n1 = contingency_analysis(buses, branches)

    # Frequency stability (loss of largest: Costa Norte 380MW)
    freq = frequency_stability_assessment(buses, generators, disturbance_mw=380)

    # SIEPAC transfer capability
    # Western generation to Panama City load
    west_gen_buses = [22, 15, 9, 8]  # Fortuna, Guasquitas, Caldera, Changuinola
    east_load_buses = [1, 2, 18, 27]  # Panama, Panama II, Panama III, 24 Dic
    tc = transfer_capability_analysis(buses, branches, west_gen_buses, east_load_buses)

    return {
        "voltage_stability": vs,
        "vq_sensitivity": vq,
        "contingency_n1": n1,
        "frequency_stability": freq,
        "transfer_capability": tc,
    }
