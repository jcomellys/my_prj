"""
Newton-Raphson Power Flow Solver for Panama 230kV System.

Implements full AC power flow analysis with:
- Y-bus admittance matrix construction
- Newton-Raphson iterative solution
- Power flow results (voltages, angles, line flows, losses)
"""

import numpy as np
from typing import Optional
from .network_model import Bus, Branch, BusType, build_panama_network

BASE_MVA = 100.0


def build_ybus(buses, branches):
    """Build the complex admittance matrix Y-bus."""
    n = len(buses)
    bus_idx = {b.id: i for i, b in enumerate(buses)}
    Y = np.zeros((n, n), dtype=complex)

    for br in branches:
        if not br.is_active:
            continue

        i = bus_idx[br.from_bus]
        j = bus_idx[br.to_bus]

        # Series admittance
        z = complex(br.r_pu, br.x_pu)
        y_series = 1.0 / z

        # For multiple circuits, multiply admittance
        y_series *= br.circuits

        # Line charging (total, split half-half)
        b_charging = br.b_pu * br.circuits

        if br.is_transformer:
            tap = br.tap_ratio
            Y[i, i] += y_series / (tap ** 2)
            Y[j, j] += y_series
            Y[i, j] -= y_series / tap
            Y[j, i] -= y_series / tap
        else:
            Y[i, i] += y_series + 1j * b_charging / 2
            Y[j, j] += y_series + 1j * b_charging / 2
            Y[i, j] -= y_series
            Y[j, i] -= y_series

    # Add shunt elements
    for bus in buses:
        i = bus_idx[bus.id]
        if bus.b_shunt_mvar != 0:
            Y[i, i] += 1j * bus.b_shunt_mvar / BASE_MVA

    return Y, bus_idx


def compute_power_injections(V, Y):
    """Compute complex power injections S = V * conj(Y * V)."""
    I = Y @ V
    S = V * np.conj(I)
    return S.real * BASE_MVA, S.imag * BASE_MVA  # P in MW, Q in MVAr


def newton_raphson_power_flow(buses, branches, max_iter=50, tolerance=1e-6):
    """
    Solve power flow using Newton-Raphson method.

    Returns:
        dict with voltage magnitudes, angles, power injections, convergence info
    """
    n = len(buses)
    Y, bus_idx = build_ybus(buses, branches)

    # Initialize voltage vector
    V_mag = np.array([b.v_mag_pu for b in buses])
    V_ang = np.array([b.v_ang_rad for b in buses])

    # Scheduled power injections (generation - load) in per-unit
    P_sched = np.array([(b.p_gen_mw - b.p_load_mw) / BASE_MVA for b in buses])
    Q_sched = np.array([(b.q_gen_mvar - b.q_load_mvar) / BASE_MVA for b in buses])

    # Identify bus types
    slack_buses = [bus_idx[b.id] for b in buses if b.bus_type == BusType.SLACK]
    pv_buses = [bus_idx[b.id] for b in buses if b.bus_type == BusType.PV]
    pq_buses = [bus_idx[b.id] for b in buses if b.bus_type == BusType.PQ]

    # Indices for the Jacobian
    pq_pv = sorted(pv_buses + pq_buses)  # All non-slack buses (for P equations)
    pq_only = sorted(pq_buses)            # PQ buses only (for Q equations)

    n_p = len(pq_pv)
    n_q = len(pq_only)

    converged = False
    iterations = 0

    for iteration in range(max_iter):
        # Complex voltage
        V = V_mag * np.exp(1j * V_ang)

        # Compute current injections
        I_calc = Y @ V

        # Computed power
        S_calc = V * np.conj(I_calc)
        P_calc = S_calc.real
        Q_calc = S_calc.imag

        # Power mismatches
        dP = P_sched - P_calc
        dQ = Q_sched - Q_calc

        # Build mismatch vector
        mismatch = np.concatenate([dP[pq_pv], dQ[pq_only]])

        # Check convergence
        max_mismatch = np.max(np.abs(mismatch))
        if max_mismatch < tolerance:
            converged = True
            iterations = iteration + 1
            break

        # Build Jacobian
        J = _build_jacobian(Y, V, V_mag, V_ang, pq_pv, pq_only, n)

        # Solve J * dx = mismatch
        try:
            dx = np.linalg.solve(J, mismatch)
        except np.linalg.LinAlgError:
            break

        # Update variables
        d_theta = dx[:n_p]
        d_vmag = dx[n_p:]

        for k, idx in enumerate(pq_pv):
            V_ang[idx] += d_theta[k]

        for k, idx in enumerate(pq_only):
            V_mag[idx] += d_vmag[k]

        iterations = iteration + 1

    # Final computation
    V = V_mag * np.exp(1j * V_ang)
    S_calc = V * np.conj(Y @ V)

    # Compute line flows
    line_flows = _compute_line_flows(buses, branches, V, bus_idx)

    # Compute total losses
    total_p_loss = sum(lf["p_loss_mw"] for lf in line_flows)
    total_q_loss = sum(lf["q_loss_mvar"] for lf in line_flows)

    results = {
        "converged": converged,
        "iterations": iterations,
        "max_mismatch": float(max_mismatch) if not converged else float(np.max(np.abs(mismatch))),
        "buses": [],
        "line_flows": line_flows,
        "total_p_loss_mw": total_p_loss,
        "total_q_loss_mvar": total_q_loss,
        "total_generation_mw": float(np.sum(S_calc.real[S_calc.real > 0]) * BASE_MVA),
        "total_load_mw": float(sum(b.p_load_mw for b in buses)),
    }

    for bus in buses:
        idx = bus_idx[bus.id]
        results["buses"].append({
            "id": bus.id,
            "name": bus.name,
            "v_mag_pu": float(V_mag[idx]),
            "v_ang_deg": float(np.degrees(V_ang[idx])),
            "p_inject_mw": float(S_calc[idx].real * BASE_MVA),
            "q_inject_mvar": float(S_calc[idx].imag * BASE_MVA),
            "p_gen_mw": float(bus.p_gen_mw),
            "p_load_mw": float(bus.p_load_mw),
            "province": bus.province,
            "latitude": bus.latitude,
            "longitude": bus.longitude,
        })

    return results


def _build_jacobian(Y, V, V_mag, V_ang, pq_pv, pq_only, n):
    """Build the Jacobian matrix for Newton-Raphson."""
    n_p = len(pq_pv)
    n_q = len(pq_only)
    J = np.zeros((n_p + n_q, n_p + n_q))

    G = Y.real
    B = Y.imag

    # J1 (dP/dTheta), J2 (dP/dV), J3 (dQ/dTheta), J4 (dQ/dV)
    for ki, i in enumerate(pq_pv):
        for kj, j in enumerate(pq_pv):
            if i == j:
                # Diagonal
                sum_p = 0
                sum_q = 0
                for m in range(n):
                    if m != i:
                        theta_im = V_ang[i] - V_ang[m]
                        sum_p += V_mag[m] * (G[i, m] * np.sin(theta_im) - B[i, m] * np.cos(theta_im))
                        sum_q += V_mag[m] * (G[i, m] * np.cos(theta_im) + B[i, m] * np.sin(theta_im))
                J[ki, kj] = -V_mag[i] * sum_p  # dPi/dThetai
            else:
                theta_ij = V_ang[i] - V_ang[j]
                J[ki, kj] = V_mag[i] * V_mag[j] * (G[i, j] * np.sin(theta_ij) - B[i, j] * np.cos(theta_ij))

    # J2: dP/dV (for PQ buses)
    for ki, i in enumerate(pq_pv):
        for kj, j in enumerate(pq_only):
            if i == j:
                sum_val = 0
                for m in range(n):
                    if m != i:
                        theta_im = V_ang[i] - V_ang[m]
                        sum_val += V_mag[m] * (G[i, m] * np.cos(theta_im) + B[i, m] * np.sin(theta_im))
                J[ki, n_p + kj] = sum_val + 2 * V_mag[i] * G[i, i]
            else:
                theta_ij = V_ang[i] - V_ang[j]
                J[ki, n_p + kj] = V_mag[i] * (G[i, j] * np.cos(theta_ij) + B[i, j] * np.sin(theta_ij))

    # J3: dQ/dTheta
    for ki, i in enumerate(pq_only):
        for kj, j in enumerate(pq_pv):
            if i == j:
                sum_val = 0
                for m in range(n):
                    if m != i:
                        theta_im = V_ang[i] - V_ang[m]
                        sum_val += V_mag[m] * (G[i, m] * np.cos(theta_im) + B[i, m] * np.sin(theta_im))
                J[n_p + ki, kj] = V_mag[i] * sum_val
            else:
                theta_ij = V_ang[i] - V_ang[j]
                J[n_p + ki, kj] = -V_mag[i] * V_mag[j] * (G[i, j] * np.cos(theta_ij) + B[i, j] * np.sin(theta_ij))

    # J4: dQ/dV (for PQ buses)
    for ki, i in enumerate(pq_only):
        for kj, j in enumerate(pq_only):
            if i == j:
                sum_val = 0
                for m in range(n):
                    if m != i:
                        theta_im = V_ang[i] - V_ang[m]
                        sum_val += V_mag[m] * (G[i, m] * np.sin(theta_im) - B[i, m] * np.cos(theta_im))
                J[n_p + ki, n_p + kj] = sum_val - 2 * V_mag[i] * B[i, i]
            else:
                theta_ij = V_ang[i] - V_ang[j]
                J[n_p + ki, n_p + kj] = V_mag[i] * (G[i, j] * np.sin(theta_ij) - B[i, j] * np.cos(theta_ij))

    return J


def _compute_line_flows(buses, branches, V, bus_idx):
    """Compute active and reactive power flows on all branches."""
    line_flows = []

    for br in branches:
        if not br.is_active:
            continue

        i = bus_idx[br.from_bus]
        j = bus_idx[br.to_bus]

        z = complex(br.r_pu, br.x_pu)
        y_series = br.circuits / z
        b_half = 1j * br.b_pu * br.circuits / 2

        # Current from i to j
        I_ij = (V[i] - V[j]) * y_series + V[i] * b_half
        # Current from j to i
        I_ji = (V[j] - V[i]) * y_series + V[j] * b_half

        # Power flows
        S_ij = V[i] * np.conj(I_ij) * BASE_MVA
        S_ji = V[j] * np.conj(I_ji) * BASE_MVA

        # Losses
        S_loss = S_ij + S_ji

        # Loading percentage
        flow_mag = max(abs(S_ij), abs(S_ji))
        loading_pct = (flow_mag / br.rate_mva) * 100 if br.rate_mva > 0 else 0

        from_bus = next(b for b in buses if b.id == br.from_bus)
        to_bus = next(b for b in buses if b.id == br.to_bus)

        line_flows.append({
            "branch_id": br.id,
            "name": br.name,
            "from_bus": br.from_bus,
            "from_name": from_bus.name,
            "to_bus": br.to_bus,
            "to_name": to_bus.name,
            "p_from_mw": float(S_ij.real),
            "q_from_mvar": float(S_ij.imag),
            "p_to_mw": float(S_ji.real),
            "q_to_mvar": float(S_ji.imag),
            "p_loss_mw": float(S_loss.real),
            "q_loss_mvar": float(S_loss.imag),
            "loading_pct": float(loading_pct),
            "rate_mva": br.rate_mva,
            "length_km": br.length_km,
        })

    return line_flows


def run_power_flow():
    """Convenience function to run power flow on the default Panama network."""
    network = build_panama_network()
    return newton_raphson_power_flow(
        network["buses"],
        network["branches"],
        max_iter=50,
        tolerance=1e-6,
    )
