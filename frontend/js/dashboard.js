/**
 * Panama Power Grid Dashboard - Main JavaScript
 * Sistema Interconectado Nacional 230kV
 *
 * Handles:
 * - WebSocket real-time updates
 * - Map visualization (Leaflet)
 * - Charts (Chart.js)
 * - API calls to backend
 * - Tab navigation and scenario controls
 */

// ============================================================
// GLOBALS & STATE
// ============================================================
const API_BASE = '';
let ws = null;
let map = null;
let busMarkers = {};
let linePolylines = [];
let charts = {};
let networkData = null;

// Chart.js defaults
Chart.defaults.color = '#94a3b8';
Chart.defaults.borderColor = 'rgba(45, 55, 72, 0.5)';
Chart.defaults.font.family = "'Inter', sans-serif";
Chart.defaults.font.size = 11;
Chart.defaults.plugins.legend.labels.boxWidth = 12;

const FUEL_COLORS = {
    'Hydro': '#3b82f6',
    'Natural Gas': '#f97316',
    'Coal/Pet Coke': '#6b7280',
    'Wind': '#06b6d4',
    'Solar': '#f59e0b',
    'Interconnection': '#8b5cf6',
};

// ============================================================
// INITIALIZATION
// ============================================================
document.addEventListener('DOMContentLoaded', () => {
    initClock();
    initTabs();
    initMap();
    initScenarioControls();
    loadInitialData();
    connectWebSocket();
});

// ============================================================
// CLOCK
// ============================================================
function initClock() {
    const update = () => {
        const now = new Date();
        document.getElementById('clock').textContent = now.toLocaleTimeString('es-PA');
    };
    update();
    setInterval(update, 1000);
}

// ============================================================
// TAB NAVIGATION
// ============================================================
function initTabs() {
    document.querySelectorAll('.nav-tab').forEach(tab => {
        tab.addEventListener('click', () => {
            document.querySelectorAll('.nav-tab').forEach(t => t.classList.remove('active'));
            document.querySelectorAll('.tab-content').forEach(t => t.classList.remove('active'));
            tab.classList.add('active');
            document.getElementById('tab-' + tab.dataset.tab).classList.add('active');

            if (tab.dataset.tab === 'overview' && map) {
                setTimeout(() => map.invalidateSize(), 100);
            }
        });
    });
}

// ============================================================
// LEAFLET MAP
// ============================================================
function initMap() {
    map = L.map('map-container', {
        center: [8.8, -80.5],
        zoom: 8,
        zoomControl: true,
        attributionControl: false,
    });

    L.tileLayer('https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png', {
        maxZoom: 19,
    }).addTo(map);

    // Load network and populate map
    fetch(API_BASE + '/api/network')
        .then(r => r.json())
        .then(data => {
            networkData = data;
            populateMap(data);
        })
        .catch(() => console.warn('Could not load network data'));
}

function populateMap(data) {
    const busMap = {};
    data.buses.forEach(b => { busMap[b.id] = b; });

    // Draw transmission lines
    data.branches.forEach(br => {
        const from = busMap[br.from_bus];
        const to = busMap[br.to_bus];
        if (!from || !to) return;

        const line = L.polyline(
            [[from.latitude, from.longitude], [to.latitude, to.longitude]],
            {
                color: br.circuits > 1 ? '#3b82f6' : '#2d3748',
                weight: br.circuits > 1 ? 2.5 : 1.5,
                opacity: 0.6,
                dashArray: br.circuits > 1 ? null : '5,5',
            }
        ).addTo(map);

        line.bindPopup(`
            <div class="bus-popup">
                <h4>${br.name}</h4>
                <div class="info-row"><span class="info-label">Capacidad:</span><span class="info-value">${br.rate_mva} MVA</span></div>
                <div class="info-row"><span class="info-label">Longitud:</span><span class="info-value">${br.length_km} km</span></div>
                <div class="info-row"><span class="info-label">Circuitos:</span><span class="info-value">${br.circuits}</span></div>
            </div>
        `);

        linePolylines.push(line);
    });

    // Draw bus markers
    data.buses.forEach(bus => {
        const color = getBusColor(bus);
        const radius = getBusRadius(bus);

        const marker = L.circleMarker([bus.latitude, bus.longitude], {
            radius: radius,
            fillColor: color,
            color: '#fff',
            weight: 1,
            fillOpacity: 0.9,
        }).addTo(map);

        marker.bindPopup(`
            <div class="bus-popup">
                <h4>${bus.name}</h4>
                <div class="info-row"><span class="info-label">Tipo:</span><span class="info-value">${bus.substation_type}</span></div>
                <div class="info-row"><span class="info-label">Provincia:</span><span class="info-value">${bus.province}</span></div>
                <div class="info-row"><span class="info-label">Generacion:</span><span class="info-value">${bus.p_gen_mw} MW</span></div>
                <div class="info-row"><span class="info-label">Carga:</span><span class="info-value">${bus.p_load_mw} MW</span></div>
                <div class="info-row"><span class="info-label">Voltaje:</span><span class="info-value">${bus.voltage_kv} kV</span></div>
            </div>
        `);

        marker.bindTooltip(bus.name, {
            permanent: false,
            direction: 'top',
            className: 'bus-tooltip',
        });

        busMarkers[bus.id] = marker;
    });
}

function getBusColor(bus) {
    if (bus.type === 'SLACK') return '#ef4444';
    if (bus.type === 'PV') return '#10b981';
    if (bus.substation_type === 'GIS') return '#06b6d4';
    if (bus.substation_type === 'Interconexion' || bus.substation_type === 'Interconexión') return '#8b5cf6';
    if (bus.substation_type === 'Generación') return '#f59e0b';
    return '#3b82f6';
}

function getBusRadius(bus) {
    const power = Math.max(bus.p_gen_mw, bus.p_load_mw);
    if (power > 300) return 10;
    if (power > 150) return 8;
    if (power > 50) return 6;
    return 5;
}

// ============================================================
// WEBSOCKET
// ============================================================
function connectWebSocket() {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    ws = new WebSocket(`${protocol}//${window.location.host}/ws/realtime`);

    ws.onopen = () => {
        document.getElementById('ws-status').style.background = '#10b981';
        document.getElementById('ws-status-text').textContent = 'Conectado';
    };

    ws.onmessage = (event) => {
        const data = JSON.parse(event.data);
        if (data.type === 'realtime_update') {
            updateKPIs(data);
            updateGenerationTable(data.generation);
            updateAlertsFromRT(data);
        }
    };

    ws.onclose = () => {
        document.getElementById('ws-status').style.background = '#ef4444';
        document.getElementById('ws-status-text').textContent = 'Desconectado';
        setTimeout(connectWebSocket, 5000);
    };

    ws.onerror = () => {
        document.getElementById('ws-status').style.background = '#f59e0b';
        document.getElementById('ws-status-text').textContent = 'Error';
    };
}

// ============================================================
// KPI UPDATES
// ============================================================
function updateKPIs(data) {
    if (data.generation) {
        document.getElementById('kpi-demand').textContent = data.generation.total_demand_mw.toFixed(0);
        document.getElementById('kpi-generation').textContent = data.generation.total_generation_mw.toFixed(0);
        document.getElementById('kpi-frequency').textContent = data.generation.frequency_hz.toFixed(3);
        document.getElementById('kpi-siepac').textContent =
            (data.generation.siepac_flow_mw > 0 ? '+' : '') + data.generation.siepac_flow_mw.toFixed(0);
    }
    if (data.market) {
        document.getElementById('kpi-marginal-cost').textContent = data.market.spot_price_usd_mwh.toFixed(1);
    }
}

// ============================================================
// GENERATION TABLE
// ============================================================
function updateGenerationTable(genData) {
    if (!genData || !genData.generation) return;
    const tbody = document.getElementById('generation-tbody');
    tbody.innerHTML = genData.generation.map(g => {
        const fuelClass = getFuelClass(g.fuel);
        return `<tr>
            <td><span class="fuel-dot ${fuelClass}"></span>${g.name}</td>
            <td class="${fuelClass}">${g.fuel}</td>
            <td class="num">${g.capacity_mw}</td>
            <td class="num">${g.output_mw}</td>
            <td class="num">${g.utilization_pct}%</td>
        </tr>`;
    }).join('');
}

function getFuelClass(fuel) {
    if (fuel.includes('Hydro')) return 'hydro';
    if (fuel.includes('Gas') || fuel.includes('Natural')) return 'gas';
    if (fuel.includes('Coal') || fuel.includes('Pet')) return 'coal';
    if (fuel.includes('Wind')) return 'wind';
    if (fuel.includes('Solar')) return 'solar';
    return 'siepac';
}

// ============================================================
// ALERTS
// ============================================================
function updateAlertsFromRT(data) {
    const container = document.getElementById('alerts-container');
    const alerts = [];

    if (data.generation) {
        const freq = data.generation.frequency_hz;
        if (freq < 59.8 || freq > 60.2) {
            alerts.push({level: 'warning', msg: `Frecuencia: ${freq.toFixed(3)} Hz (fuera de banda normal)`});
        }
        if (data.generation.siepac_flow_mw > 200) {
            alerts.push({level: 'warning', msg: `Alta importacion SIEPAC: ${data.generation.siepac_flow_mw} MW`});
        }
    }

    if (data.voltages) {
        data.voltages.voltages.forEach(v => {
            if (v.v_mag_pu < 0.95) {
                alerts.push({level: 'critical', msg: `Bajo voltaje Bus ${v.bus_id}: ${v.v_mag_pu.toFixed(4)} pu`});
            } else if (v.v_mag_pu > 1.05) {
                alerts.push({level: 'warning', msg: `Alto voltaje Bus ${v.bus_id}: ${v.v_mag_pu.toFixed(4)} pu`});
            }
        });
    }

    if (alerts.length === 0) {
        alerts.push({level: 'info', msg: 'Sistema operando dentro de parametros normales'});
    }

    document.getElementById('alert-count').textContent = alerts.filter(a => a.level !== 'info').length;

    container.innerHTML = alerts.slice(0, 6).map(a =>
        `<div class="alert-box ${a.level}">${a.msg}</div>`
    ).join('');
}

// ============================================================
// LOAD INITIAL DATA & CHARTS
// ============================================================
async function loadInitialData() {
    try {
        // Load overview charts in parallel
        const [genData, histData, pfData] = await Promise.all([
            fetch(API_BASE + '/api/realtime/generation').then(r => r.json()),
            fetch(API_BASE + '/api/realtime/history?hours=24').then(r => r.json()),
            fetch(API_BASE + '/api/power-flow').then(r => r.json()),
        ]);

        createGenMixChart(genData);
        createLoadCurveChart(histData);
        createVoltageProfileChart(pfData);
        updatePowerFlowResults(pfData);
        updateGenerationTable(genData);

        // Update loss KPI from power flow
        if (pfData.converged) {
            document.getElementById('kpi-losses').textContent = pfData.total_p_loss_mw.toFixed(1);
        }
    } catch (e) {
        console.warn('Error loading initial data:', e);
    }
}

// ============================================================
// CHART: Generation Mix (Doughnut)
// ============================================================
function createGenMixChart(genData) {
    const ctx = document.getElementById('chart-gen-mix');
    if (!ctx) return;

    const fuelTotals = {};
    genData.generation.forEach(g => {
        const key = g.fuel;
        fuelTotals[key] = (fuelTotals[key] || 0) + g.output_mw;
    });

    const labels = Object.keys(fuelTotals);
    const data = Object.values(fuelTotals);
    const colors = labels.map(l => FUEL_COLORS[l] || '#64748b');

    if (charts.genMix) charts.genMix.destroy();
    charts.genMix = new Chart(ctx, {
        type: 'doughnut',
        data: {
            labels: labels,
            datasets: [{
                data: data,
                backgroundColor: colors,
                borderColor: '#1a2332',
                borderWidth: 2,
            }],
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            plugins: {
                legend: { position: 'right', labels: { padding: 8, font: { size: 10 } } },
            },
            cutout: '60%',
        },
    });
}

// ============================================================
// CHART: Load Curve (24h)
// ============================================================
function createLoadCurveChart(histData) {
    const ctx = document.getElementById('chart-load-curve');
    if (!ctx) return;

    const labels = histData.map(d => {
        const t = new Date(d.timestamp);
        return t.getHours() + ':' + String(t.getMinutes()).padStart(2, '0');
    });

    if (charts.loadCurve) charts.loadCurve.destroy();
    charts.loadCurve = new Chart(ctx, {
        type: 'line',
        data: {
            labels: labels,
            datasets: [{
                label: 'Demanda (MW)',
                data: histData.map(d => d.demand_mw),
                borderColor: '#06b6d4',
                backgroundColor: 'rgba(6, 182, 212, 0.1)',
                fill: true,
                tension: 0.3,
                pointRadius: 0,
                borderWidth: 2,
            }],
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            scales: {
                x: { display: true, ticks: { maxTicksLimit: 12, font: { size: 9 } } },
                y: { display: true, title: { display: true, text: 'MW' } },
            },
            plugins: { legend: { display: false } },
            interaction: { intersect: false, mode: 'index' },
        },
    });
}

// ============================================================
// CHART: Voltage Profile (Overview)
// ============================================================
function createVoltageProfileChart(pfData) {
    const ctx = document.getElementById('chart-voltage-profile');
    if (!ctx || !pfData.buses) return;

    const labels = pfData.buses.map(b => b.name.substring(0, 10));
    const voltages = pfData.buses.map(b => b.v_mag_pu);
    const colors = voltages.map(v => {
        if (v < 0.95 || v > 1.05) return '#ef4444';
        if (v < 0.97 || v > 1.03) return '#f59e0b';
        return '#10b981';
    });

    if (charts.voltageOverview) charts.voltageOverview.destroy();
    charts.voltageOverview = new Chart(ctx, {
        type: 'bar',
        data: {
            labels: labels,
            datasets: [{
                label: 'Voltaje (pu)',
                data: voltages,
                backgroundColor: colors,
                borderRadius: 2,
            }],
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            indexAxis: 'y',
            scales: {
                x: { min: 0.90, max: 1.10, ticks: { font: { size: 9 } } },
                y: { ticks: { font: { size: 8 } } },
            },
            plugins: { legend: { display: false } },
        },
    });
}

// ============================================================
// SCENARIO CONTROLS
// ============================================================
function initScenarioControls() {
    // Power flow scenario sliders
    const controls = [
        { id: 'pf-load-factor', display: 'pf-load-value', format: v => parseFloat(v).toFixed(2) },
        { id: 'pf-hydro-factor', display: 'pf-hydro-value', format: v => parseFloat(v).toFixed(2) },
        { id: 'pf-siepac', display: 'pf-siepac-value', format: v => `${v} MW` },
        { id: 'pf-wind-factor', display: 'pf-wind-value', format: v => parseFloat(v).toFixed(2) },
        { id: 'pf-solar-factor', display: 'pf-solar-value', format: v => parseFloat(v).toFixed(2) },
    ];

    controls.forEach(ctrl => {
        const el = document.getElementById(ctrl.id);
        if (el) {
            el.addEventListener('input', () => {
                document.getElementById(ctrl.display).textContent = ctrl.format(el.value);
            });
        }
    });

    // Run power flow button
    document.getElementById('btn-run-pf').addEventListener('click', runPowerFlowScenario);

    // Stability buttons
    document.getElementById('btn-run-n1').addEventListener('click', runContingencyAnalysis);
    document.getElementById('btn-run-freq').addEventListener('click', runFrequencyAnalysis);
    document.getElementById('btn-run-pv').addEventListener('click', runVoltageStability);

    // Market buttons
    document.getElementById('btn-run-lmp').addEventListener('click', runLMPAnalysis);
    document.getElementById('btn-run-scenarios').addEventListener('click', runDemandScenarios);

    // SIEPAC buttons
    document.getElementById('btn-run-siepac').addEventListener('click', runSIEPACAnalysis);
    document.getElementById('btn-run-transfer').addEventListener('click', runTransferAnalysis);
}

// ============================================================
// POWER FLOW SCENARIO
// ============================================================
async function runPowerFlowScenario() {
    const btn = document.getElementById('btn-run-pf');
    btn.textContent = 'Calculando...';
    btn.disabled = true;

    const params = new URLSearchParams({
        load_factor: document.getElementById('pf-load-factor').value,
        hydro_factor: document.getElementById('pf-hydro-factor').value,
        siepac_mw: document.getElementById('pf-siepac').value,
        wind_factor: document.getElementById('pf-wind-factor').value,
        solar_factor: document.getElementById('pf-solar-factor').value,
    });

    try {
        const res = await fetch(API_BASE + `/api/power-flow/scenario?${params}`);
        const data = await res.json();
        updatePowerFlowResults(data);
    } catch (e) {
        console.error('Power flow error:', e);
    }

    btn.textContent = 'Ejecutar Flujo de Potencia';
    btn.disabled = false;
}

function updatePowerFlowResults(data) {
    // Status badge
    const status = document.getElementById('pf-status');
    if (data.converged) {
        status.textContent = `Convergido (${data.iterations} iter)`;
        status.className = 'panel-badge badge-green';
    } else {
        status.textContent = 'No convergido';
        status.className = 'panel-badge badge-red';
    }

    // Bus results table
    const busTbody = document.getElementById('pf-bus-tbody');
    if (busTbody && data.buses) {
        busTbody.innerHTML = data.buses.map(b => {
            const vClass = (b.v_mag_pu < 0.95 || b.v_mag_pu > 1.05) ? 'severity-critical' :
                          (b.v_mag_pu < 0.97 || b.v_mag_pu > 1.03) ? 'severity-warning' : '';
            return `<tr>
                <td>${b.name}</td>
                <td>${b.province}</td>
                <td class="num ${vClass}">${b.v_mag_pu.toFixed(4)}</td>
                <td class="num">${b.v_ang_deg.toFixed(2)}</td>
                <td class="num">${b.p_inject_mw.toFixed(1)}</td>
                <td class="num">${b.q_inject_mvar.toFixed(1)}</td>
            </tr>`;
        }).join('');
    }

    // Line flows table
    const lineTbody = document.getElementById('pf-line-tbody');
    if (lineTbody && data.line_flows) {
        lineTbody.innerHTML = data.line_flows.map(lf => {
            const loadClass = lf.loading_pct > 100 ? 'severity-critical' :
                             lf.loading_pct > 80 ? 'severity-warning' : '';
            return `<tr>
                <td>${lf.name}</td>
                <td class="num">${lf.p_from_mw.toFixed(1)}</td>
                <td class="num">${lf.q_from_mvar.toFixed(1)}</td>
                <td class="num">${lf.p_loss_mw.toFixed(2)}</td>
                <td class="num ${loadClass}">${lf.loading_pct.toFixed(1)}%</td>
            </tr>`;
        }).join('');
    }

    // Voltage profile chart
    if (data.buses) {
        createPFVoltageChart(data.buses);
    }
    if (data.line_flows) {
        createPFLoadingChart(data.line_flows);
    }

    // Update N-1 KPI (simplified)
    document.getElementById('kpi-n1-status').textContent = data.converged ? 'OK' : 'ALERTA';
    document.getElementById('kpi-n1-status').className = 'kpi-value ' + (data.converged ? 'green' : 'red');

    // Update losses
    if (data.total_p_loss_mw !== undefined) {
        document.getElementById('kpi-losses').textContent = data.total_p_loss_mw.toFixed(1);
    }
}

function createPFVoltageChart(buses) {
    const ctx = document.getElementById('chart-pf-voltage');
    if (!ctx) return;

    const labels = buses.map(b => b.name);
    const voltages = buses.map(b => b.v_mag_pu);
    const colors = voltages.map(v => {
        if (v < 0.95 || v > 1.05) return 'rgba(239, 68, 68, 0.8)';
        if (v < 0.97 || v > 1.03) return 'rgba(245, 158, 11, 0.8)';
        return 'rgba(16, 185, 129, 0.8)';
    });

    if (charts.pfVoltage) charts.pfVoltage.destroy();
    charts.pfVoltage = new Chart(ctx, {
        type: 'bar',
        data: {
            labels: labels,
            datasets: [{
                label: 'Voltaje (pu)',
                data: voltages,
                backgroundColor: colors,
                borderRadius: 3,
            }],
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            scales: {
                x: { ticks: { maxRotation: 90, font: { size: 8 } } },
                y: {
                    min: 0.90, max: 1.10,
                    title: { display: true, text: 'Voltaje (pu)' },
                },
            },
            plugins: {
                legend: { display: false },
                annotation: {
                    annotations: {
                        low: { type: 'line', yMin: 0.95, yMax: 0.95, borderColor: '#ef4444', borderDash: [5, 5] },
                        high: { type: 'line', yMin: 1.05, yMax: 1.05, borderColor: '#ef4444', borderDash: [5, 5] },
                    },
                },
            },
        },
    });
}

function createPFLoadingChart(lineFlows) {
    const ctx = document.getElementById('chart-pf-loading');
    if (!ctx) return;

    // Sort by loading
    const sorted = [...lineFlows].sort((a, b) => b.loading_pct - a.loading_pct).slice(0, 15);

    const colors = sorted.map(lf => {
        if (lf.loading_pct > 100) return 'rgba(239, 68, 68, 0.8)';
        if (lf.loading_pct > 80) return 'rgba(245, 158, 11, 0.8)';
        return 'rgba(59, 130, 246, 0.8)';
    });

    if (charts.pfLoading) charts.pfLoading.destroy();
    charts.pfLoading = new Chart(ctx, {
        type: 'bar',
        data: {
            labels: sorted.map(lf => lf.name.replace(' 230kV', '').substring(0, 25)),
            datasets: [{
                label: 'Carga (%)',
                data: sorted.map(lf => lf.loading_pct),
                backgroundColor: colors,
                borderRadius: 3,
            }],
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            indexAxis: 'y',
            scales: {
                x: {
                    min: 0, max: 120,
                    title: { display: true, text: 'Carga (%)' },
                },
                y: { ticks: { font: { size: 9 } } },
            },
            plugins: { legend: { display: false } },
        },
    });
}

// ============================================================
// CONTINGENCY ANALYSIS (N-1)
// ============================================================
async function runContingencyAnalysis() {
    const btn = document.getElementById('btn-run-n1');
    btn.textContent = 'Analizando...';
    btn.disabled = true;

    try {
        const res = await fetch(API_BASE + '/api/stability/contingency');
        const data = await res.json();

        const tbody = document.getElementById('n1-tbody');
        tbody.innerHTML = data.contingencies.map(c => {
            const sevClass = `severity-${c.severity}`;
            return `<tr>
                <td>${c.branch_name}</td>
                <td class="${sevClass}">${c.severity.toUpperCase()}</td>
                <td>${c.converged ? 'Si' : 'No'}</td>
                <td>${c.violations.length > 0 ? c.violations[0] : '-'}</td>
            </tr>`;
        }).join('');

        // Update KPI
        const summary = data.summary;
        document.getElementById('kpi-n1-status').textContent = summary.n1_secure ? 'SEGURO' : 'RIESGO';
        document.getElementById('kpi-n1-status').className = 'kpi-value ' + (summary.n1_secure ? 'green' : 'red');
        document.getElementById('kpi-n1-detail').textContent =
            `${summary.critical} criticos, ${summary.warning} alertas`;
    } catch (e) {
        console.error('N-1 error:', e);
    }

    btn.textContent = 'Ejecutar N-1';
    btn.disabled = false;
}

// ============================================================
// FREQUENCY STABILITY
// ============================================================
async function runFrequencyAnalysis() {
    const btn = document.getElementById('btn-run-freq');
    btn.textContent = 'Analizando...';
    btn.disabled = true;

    try {
        const res = await fetch(API_BASE + '/api/stability/frequency?disturbance_mw=380');
        const data = await res.json();

        // Frequency response chart
        const ctx = document.getElementById('chart-frequency');
        if (charts.frequency) charts.frequency.destroy();
        charts.frequency = new Chart(ctx, {
            type: 'line',
            data: {
                labels: data.time_series.time_s.filter((_, i) => i % 3 === 0),
                datasets: [{
                    label: 'Frecuencia (Hz)',
                    data: data.time_series.frequency_hz.filter((_, i) => i % 3 === 0),
                    borderColor: '#3b82f6',
                    backgroundColor: 'rgba(59, 130, 246, 0.1)',
                    fill: true,
                    tension: 0.3,
                    pointRadius: 0,
                    borderWidth: 2,
                }],
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                scales: {
                    x: { title: { display: true, text: 'Tiempo (s)' } },
                    y: {
                        title: { display: true, text: 'Frecuencia (Hz)' },
                        suggestedMin: 58.5, suggestedMax: 60.5,
                    },
                },
                plugins: { legend: { display: false } },
            },
        });

        // Details
        document.getElementById('freq-details').innerHTML = `
            <div style="display:grid;grid-template-columns:1fr 1fr;gap:8px;font-size:12px;">
                <div><span style="color:var(--text-muted)">Perturbacion:</span> <strong>${data.disturbance_mw} MW</strong></div>
                <div><span style="color:var(--text-muted)">Inercia H:</span> <strong>${data.system_inertia_h.toFixed(2)} s</strong></div>
                <div><span style="color:var(--text-muted)">RoCoF:</span> <strong>${data.rocof_hz_per_s.toFixed(3)} Hz/s</strong></div>
                <div><span style="color:var(--text-muted)">Nadir:</span> <strong style="color:${data.frequency_nadir_hz < 59 ? '#ef4444' : '#10b981'}">${data.frequency_nadir_hz.toFixed(3)} Hz</strong></div>
                <div><span style="color:var(--text-muted)">Estacionario:</span> <strong>${data.steady_state_frequency_hz.toFixed(3)} Hz</strong></div>
                <div><span style="color:var(--text-muted)">UFLS:</span> <strong style="color:${data.ufls_triggered ? '#ef4444' : '#10b981'}">${data.ufls_triggered ? 'ACTIVADO' : 'No'}</strong></div>
            </div>
        `;
    } catch (e) {
        console.error('Frequency analysis error:', e);
    }

    btn.textContent = 'Analizar';
    btn.disabled = false;
}

// ============================================================
// VOLTAGE STABILITY (P-V CURVES)
// ============================================================
async function runVoltageStability() {
    const btn = document.getElementById('btn-run-pv');
    btn.textContent = 'Calculando...';
    btn.disabled = true;

    try {
        const [pvData, vqData] = await Promise.all([
            fetch(API_BASE + '/api/stability/voltage').then(r => r.json()),
            fetch(API_BASE + '/api/stability/vq-sensitivity').then(r => r.json()),
        ]);

        // P-V curves
        const ctx = document.getElementById('chart-pv-curves');
        if (charts.pvCurves) charts.pvCurves.destroy();

        const datasets = [];
        const colors = ['#3b82f6', '#ef4444', '#10b981', '#f59e0b', '#8b5cf6', '#06b6d4', '#f97316'];
        let colorIdx = 0;

        // Show the 5 weakest buses
        const weakest = pvData.weakest_buses.slice(0, 5);
        weakest.forEach(wb => {
            const curve = pvData.pv_curves[wb.id];
            if (curve) {
                datasets.push({
                    label: wb.name,
                    data: curve.loading_pct.map((l, i) => ({ x: l, y: curve.voltage_pu[i] })),
                    borderColor: colors[colorIdx % colors.length],
                    tension: 0.3,
                    pointRadius: 2,
                    borderWidth: 2,
                    fill: false,
                });
                colorIdx++;
            }
        });

        charts.pvCurves = new Chart(ctx, {
            type: 'scatter',
            data: { datasets: datasets },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                showLine: true,
                scales: {
                    x: { title: { display: true, text: 'Incremento de Carga (%)' } },
                    y: { title: { display: true, text: 'Voltaje (pu)' }, suggestedMin: 0.85, suggestedMax: 1.10 },
                },
                plugins: {
                    legend: { position: 'top', labels: { font: { size: 10 } } },
                },
            },
        });

        // V-Q Sensitivity chart
        const vqCtx = document.getElementById('chart-vq-sensitivity');
        if (charts.vqSens) charts.vqSens.destroy();

        const sensData = vqData.sensitivities.slice(0, 15);
        charts.vqSens = new Chart(vqCtx, {
            type: 'bar',
            data: {
                labels: sensData.map(s => s.name),
                datasets: [{
                    label: 'dV/dQ (pu/MVAr)',
                    data: sensData.map(s => Math.abs(s.dv_dq)),
                    backgroundColor: sensData.map(s => s.vulnerable ? 'rgba(239, 68, 68, 0.7)' : 'rgba(59, 130, 246, 0.7)'),
                    borderRadius: 3,
                }],
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                indexAxis: 'y',
                scales: {
                    x: { title: { display: true, text: '|dV/dQ| (pu/MVAr)' } },
                    y: { ticks: { font: { size: 9 } } },
                },
                plugins: { legend: { display: false } },
            },
        });
    } catch (e) {
        console.error('Voltage stability error:', e);
    }

    btn.textContent = 'Generar Curvas';
    btn.disabled = false;
}

// ============================================================
// LMP ANALYSIS
// ============================================================
async function runLMPAnalysis() {
    const btn = document.getElementById('btn-run-lmp');
    btn.textContent = 'Calculando...';
    btn.disabled = true;

    try {
        const [lmpData, dispatchData] = await Promise.all([
            fetch(API_BASE + '/api/market/lmp').then(r => r.json()),
            fetch(API_BASE + '/api/market/dispatch').then(r => r.json()),
        ]);

        // LMP chart
        const ctx = document.getElementById('chart-lmp');
        if (charts.lmp) charts.lmp.destroy();

        const sorted = [...lmpData.bus_lmps].sort((a, b) => b.lmp_usd_mwh - a.lmp_usd_mwh);
        charts.lmp = new Chart(ctx, {
            type: 'bar',
            data: {
                labels: sorted.map(b => b.name),
                datasets: [
                    {
                        label: 'Energia',
                        data: sorted.map(b => b.energy_component),
                        backgroundColor: 'rgba(59, 130, 246, 0.7)',
                    },
                    {
                        label: 'Congestion',
                        data: sorted.map(b => b.congestion_component),
                        backgroundColor: 'rgba(245, 158, 11, 0.7)',
                    },
                    {
                        label: 'Perdidas',
                        data: sorted.map(b => b.loss_component),
                        backgroundColor: 'rgba(239, 68, 68, 0.7)',
                    },
                ],
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                scales: {
                    x: { stacked: true, ticks: { maxRotation: 90, font: { size: 8 } } },
                    y: { stacked: true, title: { display: true, text: 'LMP (USD/MWh)' } },
                },
                plugins: { legend: { position: 'top' } },
            },
        });

        // Merit order chart
        const moCtx = document.getElementById('chart-merit-order');
        if (charts.meritOrder) charts.meritOrder.destroy();

        const dispatched = dispatchData.dispatch.filter(d => d.dispatch_mw > 0);
        charts.meritOrder = new Chart(moCtx, {
            type: 'bar',
            data: {
                labels: dispatched.map(d => d.name.replace('C.H. ', '').replace('C.T. ', '').replace('P.E. ', '').replace('P.S. ', '')),
                datasets: [{
                    label: 'Despacho (MW)',
                    data: dispatched.map(d => d.dispatch_mw),
                    backgroundColor: dispatched.map(d => FUEL_COLORS[d.fuel] || '#64748b'),
                    borderRadius: 3,
                }],
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                scales: {
                    x: { ticks: { maxRotation: 90, font: { size: 9 } } },
                    y: { title: { display: true, text: 'MW' } },
                },
                plugins: { legend: { display: false } },
            },
        });

        // Dispatch table
        const dtbody = document.getElementById('dispatch-tbody');
        dtbody.innerHTML = dispatchData.dispatch.map(d => `
            <tr>
                <td><span class="fuel-dot ${getFuelClass(d.fuel)}"></span>${d.name}</td>
                <td>${d.fuel}</td>
                <td class="num">${d.dispatch_mw}</td>
                <td class="num">${d.cost_per_mwh}</td>
                <td class="num">${d.total_cost_usd_h.toLocaleString()}</td>
            </tr>
        `).join('');

        // Update marginal cost KPI
        document.getElementById('kpi-marginal-cost').textContent = dispatchData.marginal_cost_usd_mwh.toFixed(1);

    } catch (e) {
        console.error('LMP error:', e);
    }

    btn.textContent = 'Calcular LMP';
    btn.disabled = false;
}

// ============================================================
// DEMAND SCENARIOS
// ============================================================
async function runDemandScenarios() {
    const btn = document.getElementById('btn-run-scenarios');
    btn.textContent = 'Analizando...';
    btn.disabled = true;

    try {
        const data = await fetch(API_BASE + '/api/market/scenarios').then(r => r.json());
        const scenarios = data.scenarios;

        // Render scenario cards
        const grid = document.getElementById('scenarios-grid');
        grid.innerHTML = Object.entries(scenarios).map(([key, s]) => {
            const statusColor = s.converged ? (s.voltage_violations.length > 0 ? 'yellow' : 'green') : 'red';
            const statusText = s.converged ? (s.voltage_violations.length > 0 ? 'Con Alertas' : 'Normal') : 'No Converge';

            return `
            <div class="panel">
                <div class="panel-header">
                    <span class="panel-title">${s.name}</span>
                    <span class="panel-badge badge-${statusColor}">${statusText}</span>
                </div>
                <div class="panel-body">
                    <p style="font-size:11px;color:var(--text-muted);margin-bottom:12px;">${s.description}</p>
                    <div style="display:grid;grid-template-columns:1fr 1fr;gap:6px;font-size:12px;">
                        <div><span style="color:var(--text-muted)">Demanda:</span><br><strong>${s.total_demand_mw} MW</strong></div>
                        <div><span style="color:var(--text-muted)">Costo Marginal:</span><br><strong>$${s.marginal_cost_usd_mwh}/MWh</strong></div>
                        <div><span style="color:var(--text-muted)">Perdidas:</span><br><strong>${s.total_loss_mw} MW</strong></div>
                        <div><span style="color:var(--text-muted)">Costo Total:</span><br><strong>$${s.total_cost_usd_h.toLocaleString()}/h</strong></div>
                        <div><span style="color:var(--text-muted)">No Servida:</span><br><strong style="color:${s.unserved_mw > 0 ? '#ef4444' : '#10b981'}">${s.unserved_mw} MW</strong></div>
                        <div><span style="color:var(--text-muted)">Violaciones V:</span><br><strong>${s.voltage_violations.length}</strong></div>
                    </div>
                    ${s.overloads.length > 0 ? `<div class="alert-box warning" style="margin-top:8px;font-size:10px;">Sobrecargas: ${s.overloads.map(o => o.name.replace(' 230kV','')).join(', ')}</div>` : ''}
                </div>
            </div>`;
        }).join('');

        // Cost comparison chart
        const costCtx = document.getElementById('chart-scenario-costs');
        if (charts.scenarioCosts) charts.scenarioCosts.destroy();

        const keys = Object.keys(scenarios);
        charts.scenarioCosts = new Chart(costCtx, {
            type: 'bar',
            data: {
                labels: keys.map(k => scenarios[k].name),
                datasets: [{
                    label: 'Costo Total (USD/h)',
                    data: keys.map(k => scenarios[k].total_cost_usd_h),
                    backgroundColor: 'rgba(59, 130, 246, 0.7)',
                    borderRadius: 4,
                }],
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                scales: {
                    x: { ticks: { maxRotation: 45, font: { size: 9 } } },
                    y: { title: { display: true, text: 'USD/h' } },
                },
                plugins: { legend: { display: false } },
            },
        });

        // Fuel mix stacked chart
        const mixCtx = document.getElementById('chart-scenario-mix');
        if (charts.scenarioMix) charts.scenarioMix.destroy();

        const fuelTypes = ['Hydro', 'Natural Gas', 'Coal/Pet Coke', 'Wind', 'Solar'];
        charts.scenarioMix = new Chart(mixCtx, {
            type: 'bar',
            data: {
                labels: keys.map(k => scenarios[k].name),
                datasets: fuelTypes.map(fuel => ({
                    label: fuel,
                    data: keys.map(k => scenarios[k].fuel_mix_mw[fuel] || 0),
                    backgroundColor: FUEL_COLORS[fuel],
                })),
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                scales: {
                    x: { stacked: true, ticks: { maxRotation: 45, font: { size: 9 } } },
                    y: { stacked: true, title: { display: true, text: 'MW' } },
                },
                plugins: { legend: { position: 'top', labels: { font: { size: 10 } } } },
            },
        });

    } catch (e) {
        console.error('Scenarios error:', e);
    }

    btn.textContent = 'Ejecutar Todos los Escenarios';
    btn.disabled = false;
}

// ============================================================
// SIEPAC ANALYSIS
// ============================================================
async function runSIEPACAnalysis() {
    const btn = document.getElementById('btn-run-siepac');
    btn.textContent = 'Analizando...';
    btn.disabled = true;

    try {
        const [siepacData, marketData] = await Promise.all([
            fetch(API_BASE + '/api/market/siepac').then(r => r.json()),
            fetch(API_BASE + '/api/realtime/market').then(r => r.json()),
        ]);

        // SIEPAC analysis chart
        const ctx = document.getElementById('chart-siepac-analysis');
        if (charts.siepacAnalysis) charts.siepacAnalysis.destroy();

        const scenarios = siepacData.scenarios;
        charts.siepacAnalysis = new Chart(ctx, {
            type: 'bar',
            data: {
                labels: scenarios.map(s => `${s.export_mw} MW`),
                datasets: [
                    {
                        label: 'Ingreso (USD/h)',
                        data: scenarios.map(s => Math.max(0, s.revenue_usd_h)),
                        backgroundColor: 'rgba(16, 185, 129, 0.7)',
                    },
                    {
                        label: 'Costo Gen (USD/h)',
                        data: scenarios.map(s => s.generation_cost_usd_h),
                        backgroundColor: 'rgba(239, 68, 68, 0.5)',
                    },
                ],
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                scales: {
                    x: { title: { display: true, text: 'Exportacion/Importacion (MW)' } },
                    y: { title: { display: true, text: 'USD/h' } },
                },
            },
        });

        // Net benefit chart
        const bCtx = document.getElementById('chart-siepac-benefit');
        if (charts.siepacBenefit) charts.siepacBenefit.destroy();

        const feasible = scenarios.filter(s => s.converged);
        charts.siepacBenefit = new Chart(bCtx, {
            type: 'line',
            data: {
                labels: feasible.map(s => `${s.export_mw} MW`),
                datasets: [{
                    label: 'Beneficio Neto (USD/h)',
                    data: feasible.map(s => s.net_benefit_usd_h),
                    borderColor: '#8b5cf6',
                    backgroundColor: 'rgba(139, 92, 246, 0.1)',
                    fill: true,
                    tension: 0.3,
                    pointRadius: 4,
                    borderWidth: 2,
                }],
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                scales: {
                    x: { title: { display: true, text: 'Nivel de Exportacion (MW)' } },
                    y: { title: { display: true, text: 'USD/h' } },
                },
                plugins: { legend: { display: false } },
            },
        });

        // Recommendation
        const rec = document.getElementById('siepac-recommendation');
        if (siepacData.optimal_export) {
            const opt = siepacData.optimal_export;
            rec.innerHTML = `
                <div class="alert-box info">
                    <strong>Recomendacion:</strong> ${opt.direction === 'export' ? 'Exportar' : opt.direction === 'import' ? 'Importar' : 'Balanceado'}
                    ${Math.abs(opt.export_mw)} MW | Beneficio neto: $${opt.net_benefit_usd_h.toLocaleString()}/h |
                    Precio MER referencia: $${siepacData.mer_reference_price_usd_mwh}/MWh
                </div>
            `;
        }

        // Regional prices chart
        const rpCtx = document.getElementById('chart-regional-prices');
        if (charts.regionalPrices) charts.regionalPrices.destroy();

        const countries = Object.keys(marketData.mer_nodal_prices);
        const prices = Object.values(marketData.mer_nodal_prices);
        charts.regionalPrices = new Chart(rpCtx, {
            type: 'bar',
            data: {
                labels: countries,
                datasets: [{
                    label: 'Precio Nodal MER (USD/MWh)',
                    data: prices,
                    backgroundColor: countries.map(c =>
                        c === 'Panama' ? 'rgba(6, 182, 212, 0.8)' : 'rgba(59, 130, 246, 0.5)'
                    ),
                    borderRadius: 4,
                }],
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                scales: {
                    y: { title: { display: true, text: 'USD/MWh' } },
                },
                plugins: { legend: { display: false } },
            },
        });

    } catch (e) {
        console.error('SIEPAC error:', e);
    }

    btn.textContent = 'Analizar';
    btn.disabled = false;
}

// ============================================================
// TRANSFER CAPABILITY
// ============================================================
async function runTransferAnalysis() {
    const btn = document.getElementById('btn-run-transfer');
    btn.textContent = 'Calculando...';
    btn.disabled = true;

    try {
        const data = await fetch(API_BASE + '/api/stability/transfer?direction=west-east').then(r => r.json());

        const ctx = document.getElementById('chart-transfer');
        if (charts.transfer) charts.transfer.destroy();

        charts.transfer = new Chart(ctx, {
            type: 'line',
            data: {
                labels: data.steps.map(s => s.transfer_mw + ' MW'),
                datasets: [{
                    label: 'Perdidas (MW)',
                    data: data.steps.map(s => s.total_loss_mw),
                    borderColor: '#f59e0b',
                    backgroundColor: 'rgba(245, 158, 11, 0.1)',
                    fill: true,
                    tension: 0.3,
                    pointRadius: 3,
                    borderWidth: 2,
                }],
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                scales: {
                    x: { title: { display: true, text: 'Transferencia Oeste-Este (MW)' } },
                    y: { title: { display: true, text: 'Perdidas (MW)' } },
                },
                plugins: {
                    legend: { display: false },
                    title: {
                        display: true,
                        text: `Capacidad maxima de transferencia: ${data.max_transfer_mw} MW`,
                        color: '#06b6d4',
                    },
                },
            },
        });

    } catch (e) {
        console.error('Transfer error:', e);
    }

    btn.textContent = 'Calcular';
    btn.disabled = false;
}
