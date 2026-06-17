package main

const dashboardHTML = `<!DOCTYPE html>
<html lang="fr">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>PADS – Tableau de bord</title>
    <script src="https://cdn.jsdelivr.net/npm/chart.js@4.4.0/dist/chart.umd.min.js"></script>
    <style>
        :root {
            --bg: #f5f7fa; --card-bg: #ffffff; --text: #2d3748;
            --primary: #3182ce; --success: #38a169; --danger: #e53e3e; --border: #e2e8f0;
        }
        @media (prefers-color-scheme: dark) {
            :root {
                --bg: #1a202c; --card-bg: #2d3748; --text: #e2e8f0;
                --primary: #63b3ed; --success: #68d391; --danger: #fc8181; --border: #4a5568;
            }
        }
        body { font-family: 'Segoe UI', system-ui, sans-serif; background: var(--bg); margin: 0; padding: 20px; color: var(--text); }
        .container { max-width: 1000px; margin: 0 auto; }
        h1 { margin-bottom: 10px; font-weight: 600; }
        .card { background: var(--card-bg); border-radius: 12px; padding: 20px; margin-bottom: 20px; box-shadow: 0 4px 6px rgba(0,0,0,0.05); }
        .token-section { display: flex; gap: 10px; align-items: center; margin-bottom: 15px; }
        input, select, button { padding: 10px 15px; border: 1px solid var(--border); border-radius: 8px; font-size: 14px; background: var(--bg); color: var(--text); }
        button { background: var(--primary); color: white; border: none; cursor: pointer; font-weight: 500; }
        button:hover { opacity: 0.9; }
        .status-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(140px, 1fr)); gap: 15px; }
        .stat { background: var(--bg); border-radius: 8px; padding: 15px; text-align: center; }
        .stat-value { font-size: 24px; font-weight: bold; color: var(--primary); }
        .error { color: var(--danger); }
        .success { color: var(--success); }
        table { width: 100%; border-collapse: collapse; margin-top: 10px; }
        th, td { padding: 8px 12px; border-bottom: 1px solid var(--border); text-align: left; }
        th { font-weight: 600; }
        .tooltip { position: relative; display: inline-block; cursor: help; }
        .tooltip .tooltiptext { visibility: hidden; width: 220px; background-color: #555; color: #fff; text-align: center; border-radius: 6px; padding: 5px; position: absolute; z-index: 1; bottom: 125%; left: 50%; margin-left: -110px; opacity: 0; transition: opacity 0.3s; }
        .tooltip:hover .tooltiptext { visibility: visible; opacity: 1; }
    </style>
</head>
<body>
    <div class="container">
        <h1>🧬 PADS – Évolution continue</h1>

        <!-- Token -->
        <div class="card">
            <h3>🔑 Connexion</h3>
            <div class="token-section">
                <input type="password" id="tokenInput" placeholder="Token d'accès" style="flex:1">
                <button onclick="saveToken()">Enregistrer</button>
            </div>
        </div>

        <!-- Environnement -->
        <div class="card" id="workspaceCard">
            <h3>📁 Projet</h3>
            <p>Branche : <strong id="gitBranch">-</strong></p>
            <pre id="gitStatus" style="max-height:100px;overflow-y:auto;background:var(--bg);padding:10px;border-radius:8px;"></pre>
            <p>Tests : <span id="testPassed">0</span> réussis, <span id="testFailed">0</span> échoués</p>
        </div>

        <!-- Évolution -->
        <div class="card">
            <h3>🚀 Proposer une nouvelle version</h3>
            <form id="evolveForm">
                <div style="display:grid; grid-template-columns:1fr 1fr; gap:10px;">
                    <input type="number" id="candidate" placeholder="Score version proposée" required>
                    <input type="number" id="current" placeholder="Score version actuelle" required>
                    <input type="number" id="weight" placeholder="Coefficient (1 par défaut)" step="0.1" value="1.0">
                    <select id="mode">
                        <option value="stable" selected>Stable</option>
                        <option value="bandit">Explorer (bandit)</option>
                        <option value="locked">Verrouillé</option>
                    </select>
                </div>
                <button type="submit" style="margin-top:10px;">Proposer</button>
            </form>
        </div>

        <!-- État système -->
        <div class="card">
            <h3>📊 Qualité du système</h3>
            <div class="status-grid" id="statusGrid">
                <div class="stat"><div class="tooltip">Stabilité<span class="tooltiptext">Plus c'est haut, plus le système est fiable</span></div><div class="stat-value" id="stabilityVal">-</div></div>
                <div class="stat"><div class="tooltip">Seuil adapt.<span class="tooltiptext">Score minimum pour accepter une nouvelle version</span></div><div class="stat-value" id="thresholdVal">-</div></div>
                <div class="stat"><div class="tooltip">Bras actif<span class="tooltiptext">Stratégie en cours (stable, bandit, locked)</span></div><div class="stat-value" id="modeVal">-</div></div>
                <div class="stat"><div class="tooltip">Apprentissage<span class="tooltiptext">Bras favori de l'UCB</span></div><div class="stat-value" id="ucbArmVal">-</div></div>
            </div>
            <canvas id="stabilityChart" height="200"></canvas>
        </div>

        <div class="card">
            <h3>📋 Historique</h3>
            <table id="historyTable">
                <thead><tr><th>#</th><th>Proposé</th><th>Actuel</th><th>Mode</th><th>Qualité</th></tr></thead>
                <tbody></tbody>
            </table>
        </div>
        <div id="message"></div>
    </div>

    <script>
        let chart;
        function getToken() {
            return localStorage.getItem('evolutionToken') || '';
        }

        function saveToken() {
            const token = document.getElementById('tokenInput').value.trim();
            if (token) {
                localStorage.setItem('evolutionToken', token);
                document.getElementById('tokenInput').value = '';
                showMessage('Token enregistré.', 'success');
                refreshAll();
            }
        }

        window.onload = () => {
            const stored = getToken();
            if (stored) document.getElementById('tokenInput').placeholder = 'Token chargé';
            refreshAll();
        };

        async function refreshAll() {
            refreshState();
            refreshWorkspace();
        }

        async function refreshState() {
            const token = getToken();
            if (!token) return;
            try {
                const resp = await fetch('/state', { headers: { 'Authorization': 'Bearer ' + token } });
                if (resp.ok) {
                    const state = await resp.json();
                    updateStateUI(state);
                } else {
                    showMessage('État non autorisé', 'error');
                }
            } catch(e) { showMessage('Erreur réseau', 'error'); }
        }

        async function refreshWorkspace() {
            const token = getToken();
            if (!token) return;
            try {
                const resp = await fetch('/workspace', { headers: { 'Authorization': 'Bearer ' + token } });
                if (resp.ok) {
                    const ws = await resp.json();
                    document.getElementById('gitBranch').textContent = ws.gitBranch;
                    document.getElementById('gitStatus').textContent = ws.gitStatus;
                    document.getElementById('testPassed').textContent = ws.testPassed;
                    document.getElementById('testFailed').textContent = ws.testFailed;
                }
            } catch(e) { /* silencieux */ }
        }

        function updateStateUI(state) {
            document.getElementById('modeVal').textContent = state.Mode;
            document.getElementById('stabilityVal').textContent = state.StabilityScore ? state.StabilityScore.toFixed(2) : '0';
            document.getElementById('thresholdVal').textContent = (state.Gate.Threshold || 0).toFixed(2);

            fetch('/select', { headers: { 'Authorization': 'Bearer ' + getToken() } })
                .then(r => r.json())
                .then(d => { document.getElementById('ucbArmVal').textContent = d.arm || '-'; });

            const window = state.DetectorWindow || [];
            const labels = window.map((_,i) => i+1);
            if (!chart) {
                const ctx = document.getElementById('stabilityChart').getContext('2d');
                chart = new Chart(ctx, {
                    type: 'line',
                    data: {
                        labels: labels,
                        datasets: [{
                            label: 'Qualité',
                            data: window,
                            borderColor: '#3182ce',
                            tension: 0.3,
                            pointRadius: 2
                        }]
                    },
                    options: { responsive: true, scales: { y: { beginAtZero: true } } }
                });
            } else {
                chart.data.labels = labels;
                chart.data.datasets[0].data = window;
                chart.update();
            }

            const tbody = document.querySelector('#historyTable tbody');
            tbody.innerHTML = '';
            if (state.Sequence > 0) {
                const row = tbody.insertRow();
                row.innerHTML = '<td>'+state.Sequence+'</td><td>'+ (state.DetectorWindow?.slice(-1)[0] || '-') +'</td><td>-</td><td>'+state.Mode+'</td><td>'+ (state.StabilityScore ? state.StabilityScore.toFixed(2) : '0') +'</td>';
            }
        }

        function showMessage(msg, type) {
            const div = document.getElementById('message');
            div.textContent = msg;
            div.className = type;
            setTimeout(() => { div.textContent = ''; }, 4000);
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
                    setTimeout(refreshState, 500);
                } else {
                    showMessage('Erreur', 'error');
                }
            } catch(err) { showMessage('Erreur réseau', 'error'); }
        });

        setInterval(refreshAll, 5000);
    </script>
</body>
</html>`
