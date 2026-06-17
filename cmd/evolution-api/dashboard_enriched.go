package main

// enrichedDashboardHTML is the Phase 5 enhanced dashboard with UCB strategies
// and evolution decision history.
const enrichedDashboardHTML = `<!DOCTYPE html>
<html lang="fr">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>PADS – Tableau de bord enrichi</title>
    <script src="https://cdn.jsdelivr.net/npm/chart.js@4.4.0/dist/chart.umd.min.js"></script>
    <style>
        :root {
            --bg: #f5f7fa; --card-bg: #ffffff; --text: #2d3748;
            --primary: #3182ce; --success: #38a169; --danger: #e53e3e;
            --warning: #dd6b20; --border: #e2e8f0; --muted: #718096;
        }
        @media (prefers-color-scheme: dark) {
            :root {
                --bg: #1a202c; --card-bg: #2d3748; --text: #e2e8f0;
                --primary: #63b3ed; --success: #68d391; --danger: #fc8181;
                --warning: #f6ad55; --border: #4a5568; --muted: #a0aec0;
            }
        }
        * { box-sizing: border-box; }
        body { font-family: 'Segoe UI', system-ui, sans-serif; background: var(--bg); margin: 0; padding: 20px; color: var(--text); }
        .container { max-width: 1200px; margin: 0 auto; }
        h1 { margin-bottom: 8px; font-weight: 600; }
        h2 { font-size: 16px; font-weight: 600; margin: 0 0 12px 0; color: var(--text); }
        h3 { font-size: 14px; font-weight: 600; margin: 0 0 8px 0; color: var(--muted); text-transform: uppercase; letter-spacing: 0.5px; }
        .card { background: var(--card-bg); border-radius: 12px; padding: 20px; margin-bottom: 20px; box-shadow: 0 2px 4px rgba(0,0,0,0.06); }
        .card-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(280px, 1fr)); gap: 20px; }
        .row { display: flex; gap: 20px; flex-wrap: wrap; }
        .row > * { flex: 1; min-width: 280px; }
        .token-section { display: flex; gap: 10px; align-items: center; margin-bottom: 15px; }
        input, select, button { padding: 10px 14px; border: 1px solid var(--border); border-radius: 8px; font-size: 14px; background: var(--bg); color: var(--text); }
        button { background: var(--primary); color: white; border: none; cursor: pointer; font-weight: 500; }
        button:hover { opacity: 0.9; }
        button.secondary { background: var(--muted); }
        .stat-row { display: flex; gap: 15px; flex-wrap: wrap; margin-bottom: 15px; }
        .stat { background: var(--bg); border-radius: 8px; padding: 12px 16px; flex: 1; min-width: 100px; }
        .stat-label { font-size: 11px; color: var(--muted); text-transform: uppercase; letter-spacing: 0.5px; }
        .stat-value { font-size: 22px; font-weight: bold; color: var(--primary); margin-top: 4px; }
        .stat-value.success { color: var(--success); }
        .stat-value.danger { color: var(--danger); }
        .stat-value.warning { color: var(--warning); }
        .badge { display: inline-block; padding: 2px 8px; border-radius: 12px; font-size: 11px; font-weight: 600; }
        .badge.aceite { background: #c6f6d5; color: #22543d; }
        .badge.rejeitado { background: #fed7d7; color: #822727; }
        .badge.pendente { background: #feebc8; color: #744210; }
        table { width: 100%; border-collapse: collapse; margin-top: 10px; font-size: 13px; }
        th, td { padding: 8px 10px; border-bottom: 1px solid var(--border); text-align: left; }
        th { font-weight: 600; color: var(--muted); font-size: 11px; text-transform: uppercase; letter-spacing: 0.5px; }
        .mono { font-family: 'Consolas', monospace; font-size: 12px; }
        .strategy-bar { display: flex; align-items: center; gap: 10px; margin: 6px 0; padding: 8px 12px; background: var(--bg); border-radius: 6px; }
        .strategy-name { font-weight: 600; min-width: 120px; }
        .strategy-bar-fill { height: 8px; background: var(--primary); border-radius: 4px; flex: 1; min-width: 40px; }
        .strategy-bar-fill.winning { background: var(--success); }
        .strategy-stats { font-size: 11px; color: var(--muted); min-width: 80px; text-align: right; }
        .progress-bar { height: 8px; background: var(--border); border-radius: 4px; overflow: hidden; }
        .progress-fill { height: 100%; background: var(--primary); border-radius: 4px; transition: width 0.3s; }
        .error { color: var(--danger); }
        .success { color: var(--success); }
        .message { padding: 10px 15px; border-radius: 8px; margin-bottom: 15px; font-size: 14px; }
        .message.success { background: #c6f6d5; color: #22543d; }
        .message.error { background: #fed7d7; color: #822727; }
        .empty { color: var(--muted); font-style: italic; text-align: center; padding: 20px; }
        .tabs { display: flex; gap: 4px; margin-bottom: 15px; }
        .tab { padding: 8px 16px; border-radius: 6px; cursor: pointer; font-size: 13px; background: transparent; color: var(--muted); border: none; }
        .tab.active { background: var(--primary); color: white; }
        .tab:hover:not(.active) { background: var(--bg); }
        #message { position: fixed; top: 20px; right: 20px; z-index: 1000; }
    </style>
</head>
<body>
    <div class="container">
        <h1>🧬 PADS – Tableau de bord</h1>

        <!-- Token -->
        <div class="card">
            <div class="token-section">
                <input type="password" id="tokenInput" placeholder="Token d'accès" style="flex:1">
                <button onclick="saveToken()">Enregistrer</button>
                <button onclick="refreshAll()" class="secondary">↻ Actualiser</button>
            </div>
        </div>

        <!-- État global -->
        <div class="card">
            <h2>📊 État du système</h2>
            <div class="stat-row" id="statRow">
                <div class="stat">
                    <div class="stat-label">Stabilité</div>
                    <div class="stat-value" id="stabilityVal">—</div>
                </div>
                <div class="stat">
                    <div class="stat-label">Seuil</div>
                    <div class="stat-value" id="thresholdVal">—</div>
                </div>
                <div class="stat">
                    <div class="stat-label">Mode</div>
                    <div class="stat-value" id="modeVal">—</div>
                </div>
                <div class="stat">
                    <div class="stat-label">Bras UCB</div>
                    <div class="stat-value" id="ucbArmVal">—</div>
                </div>
                <div class="stat">
                    <div class="stat-label">Bras favori</div>
                    <div class="stat-value success" id="bestArmVal">—</div>
                </div>
            </div>
            <canvas id="stabilityChart" height="120"></canvas>
        </div>

        <!-- Stratégies UCB -->
        <div class="card">
            <h2>🎯 Stratégies d'agent (UCB)</h2>
            <div id="strategyList">
                <div class="empty">Chargement des stratégies...</div>
            </div>
        </div>

        <!-- Proposer évolution -->
        <div class="card">
            <h2>🚀 Nouvelle proposition</h2>
            <form id="evolveForm">
                <div style="display:grid; grid-template-columns:repeat(auto-fit, minmax(150px, 1fr)); gap:10px;">
                    <input type="number" id="candidate" placeholder="Score proposé" required>
                    <input type="number" id="current" placeholder="Score actuel" required>
                    <input type="number" id="weight" placeholder="Coefficient" step="0.1" value="1.0">
                    <select id="mode">
                        <option value="stable" selected>Stable</option>
                        <option value="bandit">Explorer</option>
                        <option value="locked">Verrouillé</option>
                    </select>
                </div>
                <button type="submit" style="margin-top:12px;">Proposer</button>
            </form>
        </div>

        <!-- Historique d'évolution -->
        <div class="card">
            <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:12px;">
                <h2>📋 Historique d'évolution</h2>
                <select id="historyFilter">
                    <option value="all">Tous</option>
                    <option value="accepted">Acceptés</option>
                    <option value="rejected">Rejetés</option>
                </select>
            </div>
            <table id="historyTable">
                <thead>
                    <tr>
                        <th>#</th>
                        <th>Score prop.</th>
                        <th>Score act.</th>
                        <th>Mode</th>
                        <th>Stratégie</th>
                        <th>Résultat</th>
                        <th>Raison</th>
                    </tr>
                </thead>
                <tbody id="historyBody">
                    <tr><td colspan="7" class="empty">Aucune évolution</td></tr>
                </tbody>
            </table>
        </div>

        <!-- Candidats agents récents -->
        <div class="card">
            <h2>🤖 Suggestions d'agents récentes</h2>
            <div id="agentCandidates">
                <div class="empty">Aucune suggestion d'agent</div>
            </div>
        </div>

        <div id="message"></div>
    </div>

    <script>
        let chart = null;
        let agentHistory = [];
        let evolutionHistory = [];

        function getToken() {
            return localStorage.getItem('evolutionToken') || '';
        }

        function saveToken() {
            const token = document.getElementById('tokenInput').value.trim();
            if (token) {
                localStorage.setItem('evolutionToken', token);
                document.getElementById('tokenInput').value = '';
                showMessage('Token enregistré', 'success');
                refreshAll();
            }
        }

        window.onload = () => {
            const stored = getToken();
            if (stored) document.getElementById('tokenInput').placeholder = 'Token chargé';
            refreshAll();
        };

        async function refreshAll() {
            await Promise.all([
                refreshState(),
                refreshAgentStatus(),
                refreshHistory()
            ]);
        }

        async function refreshState() {
            const token = getToken();
            if (!token) return;
            try {
                const resp = await fetch('/state', { headers: { 'Authorization': 'Bearer ' + token } });
                if (resp.ok) {
                    const state = await resp.json();
                    updateStateUI(state);
                }
            } catch(e) { console.error('state refresh error', e); }
        }

        async function refreshAgentStatus() {
            const token = getToken();
            if (!token) return;
            try {
                const resp = await fetch('/agent/status', { headers: { 'Authorization': 'Bearer ' + token } });
                if (resp.ok) {
                    const data = await resp.json();
                    updateUCBUI(data);
                }
            } catch(e) { console.error('agent status error', e); }
        }

        async function refreshHistory() {
            const token = getToken();
            if (!token) return;
            try {
                // Load evolution history from state endpoint
                const resp = await fetch('/state', { headers: { 'Authorization': 'Bearer ' + token } });
                if (resp.ok) {
                    const state = await resp.json();
                    updateHistoryUI(state);
                }
            } catch(e) { console.error('history error', e); }
        }

        function updateStateUI(state) {
            document.getElementById('modeVal').textContent = state.Mode || '-';
            document.getElementById('stabilityVal').textContent = state.StabilityScore ? state.StabilityScore.toFixed(2) : '0';
            document.getElementById('thresholdVal').textContent = (state.Gate?.Threshold || 0).toFixed(2);

            // Update chart
            const window = state.DetectorWindow || [];
            const labels = window.map((_, i) => i + 1);
            if (!chart) {
                const ctx = document.getElementById('stabilityChart').getContext('2d');
                chart = new Chart(ctx, {
                    type: 'line',
                    data: {
                        labels: labels,
                        datasets: [{
                            label: 'Stabilité',
                            data: window,
                            borderColor: '#3182ce',
                            backgroundColor: 'rgba(49,130,206,0.1)',
                            tension: 0.3,
                            pointRadius: 2,
                            fill: true
                        }]
                    },
                    options: {
                        responsive: true,
                        scales: { y: { beginAtZero: true, max: 100 } },
                        plugins: { legend: { display: false } }
                    }
                });
            } else {
                chart.data.labels = labels;
                chart.data.datasets[0].data = window;
                chart.update();
            }
        }

        function updateUCBUI(data) {
            const arm = data.selected_arm || '-';
            document.getElementById('ucbArmVal').textContent = arm;

            // Find best arm by avg reward
            let bestArm = arm;
            let bestAvg = -1;
            const arms = data.arms || {};
            for (const [name, stats] of Object.entries(arms)) {
                if (stats.avg_reward > bestAvg) {
                    bestAvg = stats.avg_reward;
                    bestArm = name;
                }
            }
            document.getElementById('bestArmVal').textContent = bestArm;

            // Render strategy bars
            const container = document.getElementById('strategyList');
            const strategyNames = Object.keys(arms);
            if (strategyNames.length === 0) {
                container.innerHTML = '<div class="empty">Aucune stratégie enregistrée</div>';
                return;
            }

            // Find max avg_reward for scaling
            let maxAvg = 0;
            let maxPulls = 0;
            for (const stats of Object.values(arms)) {
                if (stats.avg_reward > maxAvg) maxAvg = stats.avg_reward;
                if (stats.pull_count > maxPulls) maxPulls = stats.pull_count;
            }
            if (maxAvg === 0) maxAvg = 1;

            let html = '';
            for (const [name, stats] of Object.entries(arms)) {
                const isWinner = name === bestArm;
                const barWidth = (stats.avg_reward / maxAvg * 100).toFixed(0);
                html += '<div class="strategy-bar">';
                html += '<span class="strategy-name">' + name + '</span>';
                html += '<div class="progress-bar" style="flex:1;"><div class="progress-fill ' + (isWinner ? '' : '') + '" style="width:' + barWidth + '%"></div></div>';
                html += '<span class="strategy-stats">avg=' + stats.avg_reward.toFixed(2) + ' pulls=' + stats.pull_count + '</span>';
                html += '</div>';
            }
            container.innerHTML = html;
        }

        function updateHistoryUI(state) {
            const tbody = document.getElementById('historyBody');
            const filter = document.getElementById('historyFilter').value;
            const window = state.DetectorWindow || [];

            if (window.length === 0) {
                tbody.innerHTML = '<tr><td colspan="7" class="empty">Aucune évolution dans l\'historique</td></tr>';
                return;
            }

            let html = '';
            for (let i = 0; i < window.length; i++) {
                const score = window[i];
                const isAccepted = score >= (state.Gate?.Threshold || 50);
                const mode = state.Mode || 'stable';

                // Determine result based on score change
                const prev = i > 0 ? window[i-1] : score;
                const delta = score - prev;
                let reason = '';
                if (delta > 0) reason = 'Amélioration +' + delta;
                else if (delta < 0) reason = 'Dégradation ' + delta;
                else reason = 'Stable';

                let badge = '<span class="badge ' + (isAccepted ? 'aceite' : 'rejeitado') + '">' +
                    (isAccepted ? 'Accepté' : 'Rejeté') + '</span>';

                if (filter === 'accepted' && !isAccepted) continue;
                if (filter === 'rejected' && isAccepted) continue;

                html += '<tr>';
                html += '<td>' + (i+1) + '</td>';
                html += '<td class="mono">' + score + '</td>';
                html += '<td class="mono">' + prev + '</td>';
                html += '<td>' + mode + '</td>';
                html += '<td><span class="badge pendente">' + state.Mode + '</span></td>';
                html += '<td>' + badge + '</td>';
                html += '<td style="font-size:12px; color:var(--muted);">' + reason + '</td>';
                html += '</tr>';
            }
            tbody.innerHTML = html || '<tr><td colspan="7" class="empty">Aucune entrée</td></tr>';
        }

        function showMessage(msg, type) {
            const div = document.getElementById('message');
            div.textContent = msg;
            div.className = 'message ' + type;
            setTimeout(() => { div.textContent = ''; div.className = ''; }, 4000);
        }

        document.getElementById('evolveForm').addEventListener('submit', async (e) => {
            e.preventDefault();
            const token = getToken();
            if (!token) { showMessage('Token requis', 'error'); return; }
            const body = {
                candidate: parseInt(document.getElementById('candidate').value),
                current: parseInt(document.getElementById('current').value),
                weight: parseFloat(document.getElementById('weight').value),
                mode: document.getElementById('mode').value
            };
            try {
                const resp = await fetch('/evolve', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json', 'Authorization': 'Bearer ' + token },
                    body: JSON.stringify(body)
                });
                if (resp.ok) {
                    const data = await resp.json();
                    showMessage('Proposition envoyée (ID: ' + data.id + ')', 'success');
                    setTimeout(refreshAll, 500);
                } else {
                    showMessage('Erreur: ' + resp.status, 'error');
                }
            } catch(err) { showMessage('Erreur réseau', 'error'); }
        });

        document.getElementById('historyFilter').addEventListener('change', refreshHistory);

        // Polling: refresh every 5 seconds
        setInterval(refreshAll, 5000);
    </script>
</body>
</html>`