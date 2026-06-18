/* trackfw dashboard — app.js
   Tecnologias: HTMX + marked.js + Chart.js + D3.js (todos via CDN)
   Sem bundler, sem npm no runtime.
*/

'use strict';

// ─── Estado global ────────────────────────────────────────────────────────────
let _currentView = 'board';
let _boardData   = null;     // cache do /api/board
let _chainData   = null;     // cache do /api/chain
let _donutChart  = null;     // instância Chart.js donut
let _burnChart   = null;     // instância Chart.js burndown
let _d3Sim       = null;     // simulação D3 force
let _drawerPath  = null;     // path atualmente aberto no drawer

// ─── Auto-refresh ─────────────────────────────────────────────────────────────
let _refreshInterval = parseInt(localStorage.getItem('trackfw_refresh_interval') || '60');
let _refreshPaused   = localStorage.getItem('trackfw_refresh_paused') === 'true';
let _refreshTimer    = null;

// ─── Atenção de agente ────────────────────────────────────────────────────────
let _attentionTimer     = null;
let _attentionDismissed = localStorage.getItem('trackfw_attention_dismissed') || null;

// Cores canonicas por estado
const STATE_COLORS = {
  wip:       '#3b82f6',
  backlog:   '#6b7280',
  blocked:   '#ef4444',
  done:      '#22c55e',
  abandoned: '#78716c',
};

// Cores canonicas por tipo de nó
const NODE_COLORS = {
  adr:     '#3b82f6',
  req:     '#f97316',
  roadmap: null, // usa STATE_COLORS[state]
};

// ─── Utilitários ──────────────────────────────────────────────────────────────

function truncate(str, max = 30) {
  if (!str) return '';
  return str.length > max ? str.slice(0, max - 1) + '…' : str;
}

function stateLabel(state) {
  const labels = {
    wip: 'WIP', backlog: 'BACKLOG', blocked: 'BLOCKED', done: 'DONE', abandoned: 'ABANDONED',
  };
  return labels[state] || state.toUpperCase();
}

function el(id) {
  return document.getElementById(id);
}

function show(id) {
  const e = el(id);
  if (e) e.classList.remove('hidden');
}

function hide(id) {
  const e = el(id);
  if (e) e.classList.add('hidden');
}

// ─── Navegação entre views ────────────────────────────────────────────────────

function switchView(view) {
  _currentView = view;

  // Esconder todas as sections
  document.querySelectorAll('.view-section').forEach(s => s.classList.add('hidden'));

  // Atualizar tabs
  document.querySelectorAll('.tab-btn').forEach(btn => {
    btn.classList.remove('active');
    btn.setAttribute('aria-pressed', 'false');
  });

  const activeTab = el('tab-' + view);
  if (activeTab) {
    activeTab.classList.add('active');
    activeTab.setAttribute('aria-pressed', 'true');
  }

  const activeSection = el('view-' + view);
  if (activeSection) activeSection.classList.remove('hidden');

  // Carregar dados conforme view
  if (view === 'board') {
    loadBoard();
  } else if (view === 'chain') {
    loadChain();
  } else if (view === 'metrics') {
    loadMetrics();
  }
}

// ─── View: Board / Kanban ─────────────────────────────────────────────────────

const COLUMNS_ORDER = ['backlog', 'wip', 'blocked', 'done', 'abandoned'];
const COLUMNS_LABEL = {
  wip: 'WIP', backlog: 'Backlog', blocked: 'Blocked', done: 'Done', abandoned: 'Abandoned',
};

async function loadBoard() {
  const container = el('board-container');

  // Se já temos dados em cache, apenas re-renderizar
  if (_boardData) {
    renderBoard(_boardData, el('agent-filter') ? el('agent-filter').value : '');
    return;
  }

  // Loading
  container.innerHTML = `
    <div id="board-loading" class="flex items-center gap-2 text-gray-500 text-sm">
      <svg class="animate-spin h-4 w-4 text-blue-500" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" aria-hidden="true">
        <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
        <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v8H4z"></path>
      </svg>
      Carregando board...
    </div>`;
  hide('board-error');

  try {
    const res = await fetch('/api/board');
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    _boardData = await res.json();

    // Popular dropdown de agentes
    const agents = _boardData.agents || [];
    const filterContainer = el('agent-filter-container');
    const agentSelect    = el('agent-filter');

    if (agents.length > 1 && filterContainer && agentSelect) {
      filterContainer.classList.remove('hidden');
      // Limpar opções extras (manter "Todos")
      while (agentSelect.options.length > 1) agentSelect.remove(1);
      agents.forEach(agent => {
        const opt = document.createElement('option');
        opt.value = agent;
        opt.textContent = agent;
        agentSelect.appendChild(opt);
      });
    }

    renderBoard(_boardData, '');
  } catch (err) {
    container.innerHTML = '';
    const errEl = el('board-error');
    if (errEl) {
      errEl.textContent = `Erro ao carregar board: ${err.message}`;
      errEl.classList.remove('hidden');
    }
  }
}

function renderBoard(data, agentFilter) {
  const container = el('board-container');
  if (!container) return;

  container.innerHTML = '';

  COLUMNS_ORDER.forEach(state => {
    let cards = (data.columns[state] || []);
    if (agentFilter) {
      cards = cards.filter(c => c.agent === agentFilter);
    }
    if (state === 'wip') {
      cards = [...cards].sort((a, b) => (b.active_ml ? 1 : 0) - (a.active_ml ? 1 : 0));
    }

    const col = document.createElement('div');
    col.className = 'kanban-column bg-white rounded-lg border border-gray-200 shadow-sm flex flex-col';
    col.setAttribute('aria-label', `Coluna ${COLUMNS_LABEL[state]}`);

    // Header da coluna
    const header = document.createElement('div');
    header.className = 'px-4 py-3 border-b border-gray-200 flex items-center justify-between';
    header.innerHTML = `
      <span class="text-xs font-semibold uppercase tracking-wider text-gray-500">${COLUMNS_LABEL[state]}</span>
      <span class="badge-${state} text-xs font-bold px-2 py-0.5 rounded-full">${cards.length}</span>`;
    col.appendChild(header);

    // Cards
    const cardsContainer = document.createElement('div');
    cardsContainer.className = 'p-3 flex flex-col gap-2 flex-1 overflow-y-auto';
    cardsContainer.style.maxHeight = 'calc(100vh - 160px)';

    if (cards.length === 0) {
      const empty = document.createElement('p');
      empty.className = 'text-xs text-gray-400 text-center py-4';
      empty.textContent = 'Nenhum item';
      cardsContainer.appendChild(empty);
    } else {
      cards.forEach(card => {
        cardsContainer.appendChild(createCard(card));
      });
    }

    col.appendChild(cardsContainer);
    container.appendChild(col);
  });
}

function createCard(card) {
  const isActive = !!(card.active_ml) && card.state === 'wip';

  const div = document.createElement('div');
  div.className = `kanban-card bg-gray-50 border border-gray-200 rounded-md p-3 hover:bg-white${isActive ? ' card-active' : ''}`;
  div.setAttribute('tabindex', '0');
  div.setAttribute('role', 'button');
  div.setAttribute('aria-label', `Abrir: ${card.title || card.file}`);
  div.setAttribute('data-file', card.file || '');

  const total    = card.ml_total  || 0;
  const done     = card.ml_done   || 0;
  const activeML = card.active_ml || '';
  const pct      = total > 0 ? Math.round((done / total) * 100) : 0;

  let barColor = 'bg-blue-500';
  if (card.state === 'blocked') barColor = 'bg-red-400';
  else if (done === total && total > 0) barColor = 'bg-green-500';

  const progressHTML = total > 0 ? `
    <div class="mt-2 pt-2 border-t border-gray-100">
      <div class="flex items-center justify-between mb-1">
        <span class="text-xs text-gray-400">MLs</span>
        <span class="text-xs font-medium text-gray-600">${done}/${total}</span>
      </div>
      <div class="w-full bg-gray-200 rounded-full h-1.5">
        <div class="${barColor} h-1.5 rounded-full transition-all" style="width:${pct}%"></div>
      </div>
      ${activeML ? `<p class="text-xs text-blue-600 mt-1 leading-snug truncate" title="${escapeHtml(activeML)}">▶ ${escapeHtml(activeML)}</p>` : ''}
    </div>` : '';

  div.innerHTML = `
    ${isActive ? '<span class="live-dot" aria-hidden="true"></span>' : ''}
    <p class="text-xs font-semibold text-gray-800 mb-2 leading-snug">${escapeHtml(card.title || card.file)}</p>
    <div class="flex flex-wrap items-center gap-1">
      ${card.agent ? `<span class="inline-block text-xs bg-purple-100 text-purple-700 px-1.5 py-0.5 rounded font-medium">${escapeHtml(card.agent)}</span>` : ''}
      <span class="badge-${card.state} inline-block text-xs px-1.5 py-0.5 rounded font-medium">${stateLabel(card.state)}</span>
      ${isActive ? '<span class="inline-block text-xs bg-green-100 text-green-700 px-1.5 py-0.5 rounded font-medium">ATIVO</span>' : ''}
    </div>
    ${progressHTML}`;

  div.addEventListener('click', () => openDrawer(card.path));
  div.addEventListener('keydown', e => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); openDrawer(card.path); } });

  return div;
}

function filterByAgent(agent) {
  if (_boardData) {
    renderBoard(_boardData, agent);
  }
}

// ─── Auto-refresh ─────────────────────────────────────────────────────────────

function startRefreshTimer() {
  clearTimeout(_refreshTimer);
  if (_refreshPaused || _refreshInterval <= 0) return;
  _refreshTimer = setTimeout(() => {
    if (_currentView === 'board') {
      _boardData = null;
      loadBoard();
    }
    startRefreshTimer();
  }, _refreshInterval * 1000);
}

function setRefreshInterval(seconds) {
  _refreshInterval = seconds;
  localStorage.setItem('trackfw_refresh_interval', String(seconds));
  startRefreshTimer();
}

function toggleRefreshPause() {
  _refreshPaused = !_refreshPaused;
  localStorage.setItem('trackfw_refresh_paused', String(_refreshPaused));
  if (_refreshPaused) {
    clearTimeout(_refreshTimer);
  } else {
    startRefreshTimer();
  }
  updateRefreshUI();
}

function manualRefresh() {
  _boardData = null;
  loadBoard();
  startRefreshTimer();
}

function updateRefreshUI() {
  const btn = el('refresh-toggle');
  if (!btn) return;
  btn.textContent = _refreshPaused ? '▶ Retomar' : '⏸ Pausar';
}

function initRefreshUI() {
  const sel = el('refresh-interval');
  if (sel) sel.value = String(_refreshInterval);
  updateRefreshUI();
}

// ─── Atenção de agente ────────────────────────────────────────────────────────

function startAttentionPolling() {
  clearInterval(_attentionTimer);
  _attentionTimer = setInterval(pollAttention, 8000);
  pollAttention();
}

async function pollAttention() {
  try {
    const res = await fetch('/api/attention');
    if (!res.ok) return;
    const data = await res.json();
    if (data.active && data.timestamp !== _attentionDismissed) {
      showAttentionBanner(data);
    } else if (!data.active) {
      hideAttentionBanner();
    }
  } catch (_) { /* ignorar erros de rede */ }
}

function showAttentionBanner(data) {
  const banner = el('attention-banner');
  if (!banner) return;

  const isAction = data.level !== 'info';
  banner.className = `attention-banner ${isAction ? 'attention-banner-action' : 'attention-banner-info'}`;
  banner.setAttribute('data-timestamp', data.timestamp || '');

  const iconEl = el('attention-icon');
  const labelEl = el('attention-label');
  const ctxEl = el('attention-context');
  const msgEl = el('attention-message');

  if (iconEl)  iconEl.textContent  = isAction ? '⚠️' : 'ℹ️';
  if (labelEl) labelEl.textContent = isAction ? 'Ação necessária' : 'Aviso';
  if (ctxEl)   ctxEl.textContent   = [data.roadmap, data.ml].filter(Boolean).join(' · ');
  if (msgEl)   msgEl.textContent   = data.message || '';

  if (data.roadmap) markCardAttention(data.roadmap);
}

function hideAttentionBanner() {
  const banner = el('attention-banner');
  if (banner) banner.className = 'hidden attention-banner attention-banner-action';
  clearCardAttention();
}

function dismissAttention() {
  const banner = el('attention-banner');
  if (!banner) return;
  const ts = banner.getAttribute('data-timestamp');
  if (ts) {
    _attentionDismissed = ts;
    localStorage.setItem('trackfw_attention_dismissed', ts);
  }
  hideAttentionBanner();
}

function markCardAttention(roadmapFile) {
  document.querySelectorAll('.kanban-card[data-file]').forEach(c => {
    if (c.getAttribute('data-file') !== roadmapFile) return;
    c.classList.add('card-attention');
    const live = c.querySelector('.live-dot');
    if (live) {
      live.className = 'attention-dot';
    } else {
      const dot = document.createElement('span');
      dot.className = 'attention-dot';
      dot.setAttribute('aria-hidden', 'true');
      c.appendChild(dot);
    }
  });
}

function clearCardAttention() {
  document.querySelectorAll('.card-attention').forEach(c => {
    c.classList.remove('card-attention');
    const dot = c.querySelector('.attention-dot');
    if (dot) {
      if (c.classList.contains('card-active')) {
        dot.className = 'live-dot';
      } else {
        dot.remove();
      }
    }
  });
}

// ─── View: Chain (D3 force-directed graph) ────────────────────────────────────

async function loadChain() {
  hide('chain-error');

  // Se já tem dados, re-renderizar
  if (_chainData) {
    renderChain(_chainData);
    return;
  }

  show('chain-loading');

  try {
    const res = await fetch('/api/chain');
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    _chainData = await res.json();
    hide('chain-loading');
    renderChain(_chainData);
  } catch (err) {
    hide('chain-loading');
    const errEl = el('chain-error');
    if (errEl) {
      errEl.textContent = `Erro ao carregar cadeia: ${err.message}`;
      errEl.classList.remove('hidden');
    }
  }
}

function renderChain(data) {
  const svgEl = el('chain-svg');
  if (!svgEl) return;

  // Limpar SVG anterior
  while (svgEl.firstChild) svgEl.removeChild(svgEl.firstChild);

  const width  = svgEl.clientWidth  || window.innerWidth;
  const height = svgEl.clientHeight || window.innerHeight - 60;

  svgEl.setAttribute('viewBox', `0 0 ${width} ${height}`);

  const svg = d3.select(svgEl);

  // Defs para a seta
  svg.append('defs').append('marker')
    .attr('id', 'arrowhead')
    .attr('viewBox', '-0 -5 10 10')
    .attr('refX', 22)
    .attr('refY', 0)
    .attr('orient', 'auto')
    .attr('markerWidth', 6)
    .attr('markerHeight', 6)
    .append('path')
    .attr('d', 'M 0,-5 L 10,0 L 0,5')
    .attr('fill', '#9ca3af');

  // Grupo raiz (para zoom/pan)
  const g = svg.append('g');

  // Zoom
  const zoom = d3.zoom()
    .scaleExtent([0.2, 4])
    .on('zoom', (event) => g.attr('transform', event.transform));
  svg.call(zoom);

  // Preparar nodes e edges (deep copy para D3 mutate)
  const nodes = (data.nodes || []).map(n => ({ ...n }));
  const edges = (data.edges || []).map(e => ({ ...e }));

  // Links com referência aos objetos de node
  const nodeById = {};
  nodes.forEach(n => { nodeById[n.id] = n; });

  const links = edges.map(e => ({
    source: nodeById[e.from],
    target: nodeById[e.to],
  })).filter(l => l.source && l.target);

  // Simulação
  if (_d3Sim) _d3Sim.stop();

  _d3Sim = d3.forceSimulation(nodes)
    .force('link', d3.forceLink(links).id(d => d.id).distance(120).strength(0.6))
    .force('charge', d3.forceManyBody().strength(-280))
    .force('center', d3.forceCenter(width / 2, height / 2))
    .force('collision', d3.forceCollide(35));

  // Arestas
  const link = g.append('g')
    .attr('class', 'links')
    .selectAll('line')
    .data(links)
    .enter().append('line')
    .attr('class', 'link-line')
    .attr('marker-end', 'url(#arrowhead)');

  // Grupos de nós
  const node = g.append('g')
    .attr('class', 'nodes')
    .selectAll('g')
    .data(nodes)
    .enter().append('g')
    .attr('class', 'node-group')
    .attr('cursor', 'pointer')
    .on('click', (_, d) => openDrawer(d.id))
    .call(
      d3.drag()
        .on('start', (event, d) => {
          if (!event.active) _d3Sim.alphaTarget(0.3).restart();
          d.fx = d.x; d.fy = d.y;
        })
        .on('drag', (event, d) => { d.fx = event.x; d.fy = event.y; })
        .on('end', (event, d) => {
          if (!event.active) _d3Sim.alphaTarget(0);
          d.fx = null; d.fy = null;
        })
    );

  // Círculo de cada nó
  node.append('circle')
    .attr('r', 18)
    .attr('fill', d => {
      if (d.type === 'adr') return NODE_COLORS.adr;
      if (d.type === 'req') return NODE_COLORS.req;
      return STATE_COLORS[d.state] || '#6b7280';
    })
    .attr('stroke', '#fff')
    .attr('stroke-width', 2);

  // Ícone/sigla dentro do círculo
  node.append('text')
    .attr('text-anchor', 'middle')
    .attr('dy', '0.35em')
    .attr('fill', '#fff')
    .attr('font-size', '9px')
    .attr('font-weight', '700')
    .attr('pointer-events', 'none')
    .text(d => {
      if (d.type === 'adr') return 'ADR';
      if (d.type === 'req') return 'REQ';
      return 'RM';
    });

  // Label abaixo do círculo
  node.append('text')
    .attr('class', 'node-label')
    .attr('text-anchor', 'middle')
    .attr('dy', '2.4em')
    .text(d => truncate(d.title || d.id, 28));

  // Tooltip básico via title SVG
  node.append('title').text(d => `${d.type.toUpperCase()}: ${d.title || d.id}\nEstado: ${d.state || '—'}`);

  // Tick
  _d3Sim.on('tick', () => {
    link
      .attr('x1', d => d.source.x)
      .attr('y1', d => d.source.y)
      .attr('x2', d => d.target.x)
      .attr('y2', d => d.target.y);

    node.attr('transform', d => `translate(${d.x},${d.y})`);
  });
}

// ─── View: Metrics ────────────────────────────────────────────────────────────

async function loadMetrics() {
  hide('metrics-error');
  hide('kpi-cards');
  hide('charts-container');
  show('metrics-loading');

  try {
    const res = await fetch('/api/metrics');
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    const data = await res.json();
    hide('metrics-loading');
    renderMetrics(data);
  } catch (err) {
    hide('metrics-loading');
    const errEl = el('metrics-error');
    if (errEl) {
      errEl.textContent = `Erro ao carregar métricas: ${err.message}`;
      errEl.classList.remove('hidden');
    }
  }
}

function renderMetrics(data) {
  // KPI cards
  const leadEl = el('kpi-lead-time');
  const cycleEl = el('kpi-cycle-time');
  const abandonEl = el('kpi-abandonment');

  if (leadEl)   leadEl.textContent   = `${(data.lead_time_avg_days  || 0).toFixed(1)} dias`;
  if (cycleEl)  cycleEl.textContent  = `${(data.cycle_time_avg_days || 0).toFixed(1)} dias`;
  if (abandonEl) abandonEl.textContent = `${(((data.abandonment_rate || 0) * 100)).toFixed(1)}%`;

  show('kpi-cards');

  // Donut chart — distribuição por estado
  const dist = data.state_distribution || {};
  const donutLabels = Object.keys(dist);
  const donutValues = Object.values(dist);
  const donutColors = donutLabels.map(s => STATE_COLORS[s] || '#9ca3af');

  const donutCanvas = el('donut-chart');
  if (donutCanvas) {
    if (_donutChart) { _donutChart.destroy(); _donutChart = null; }
    _donutChart = new Chart(donutCanvas, {
      type: 'doughnut',
      data: {
        labels: donutLabels.map(stateLabel),
        datasets: [{
          data: donutValues,
          backgroundColor: donutColors,
          borderWidth: 2,
          borderColor: '#fff',
        }],
      },
      options: {
        responsive: true,
        maintainAspectRatio: true,
        plugins: {
          legend: {
            position: 'bottom',
            labels: { font: { size: 11 }, padding: 12 },
          },
        },
      },
    });
  }

  // Burndown chart
  const burndown = data.burndown || [];
  const burndownCanvas = el('burndown-chart');
  const burndownEmpty  = el('burndown-empty');

  if (burndown.length === 0) {
    if (burndownCanvas) burndownCanvas.style.display = 'none';
    if (burndownEmpty) burndownEmpty.classList.remove('hidden');
  } else {
    if (burndownEmpty) burndownEmpty.classList.add('hidden');
    if (burndownCanvas) {
      burndownCanvas.style.display = '';
      if (_burnChart) { _burnChart.destroy(); _burnChart = null; }
      _burnChart = new Chart(burndownCanvas, {
        type: 'line',
        data: {
          labels: burndown.map(b => b.date),
          datasets: [
            {
              label: 'Abertos',
              data: burndown.map(b => b.open),
              borderColor: '#ef4444',
              backgroundColor: 'rgba(239,68,68,0.08)',
              pointRadius: 4,
              tension: 0.3,
              fill: true,
            },
            {
              label: 'Fechados',
              data: burndown.map(b => b.closed),
              borderColor: '#22c55e',
              backgroundColor: 'rgba(34,197,94,0.08)',
              pointRadius: 4,
              tension: 0.3,
              fill: true,
            },
          ],
        },
        options: {
          responsive: true,
          maintainAspectRatio: false,
          scales: {
            x: { ticks: { font: { size: 10 } } },
            y: { beginAtZero: true, ticks: { font: { size: 10 }, precision: 0 } },
          },
          plugins: {
            legend: { labels: { font: { size: 11 } } },
          },
        },
      });
    }
  }

  show('charts-container');
}

// ─── Drawer lateral ───────────────────────────────────────────────────────────

async function openDrawer(path) {
  if (!path) return;
  _drawerPath = path;

  const drawer  = el('drawer');
  const overlay = el('drawer-overlay');

  // Exibir drawer e overlay
  drawer.style.display  = 'flex';
  overlay.classList.remove('hidden');
  drawer.classList.remove('hidden');

  // Forçar re-trigger da animação
  drawer.classList.remove('drawer-slide');
  void drawer.offsetWidth;
  drawer.classList.add('drawer-slide');

  // Filename no header
  const filenameEl = el('drawer-filename');
  if (filenameEl) filenameEl.textContent = path.split('/').pop();

  // Resetar conteúdo
  hide('drawer-content');
  hide('drawer-error');
  show('drawer-loading');

  const mdEl = el('drawer-markdown');
  const fmEl = el('drawer-frontmatter');
  if (mdEl) mdEl.innerHTML = '';
  if (fmEl) fmEl.classList.add('hidden');

  try {
    const res = await fetch(`/api/file?path=${encodeURIComponent(path)}`);
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    const raw = await res.text();

    hide('drawer-loading');

    // Extrair e renderizar frontmatter
    const { frontmatter, body } = parseFrontmatter(raw);
    if (frontmatter && Object.keys(frontmatter).length > 0) {
      renderFrontmatterTable(frontmatter);
    }

    // Renderizar markdown
    if (mdEl) {
      mdEl.innerHTML = marked.parse(body || raw);

      // Interceptar links internos .md
      mdEl.querySelectorAll('a[href]').forEach(a => {
        const href = a.getAttribute('href');
        if (!href) return;
        const isExternal = href.startsWith('http://') || href.startsWith('https://');
        const isMdLink   = href.endsWith('.md');
        if (!isExternal && isMdLink) {
          a.addEventListener('click', e => {
            e.preventDefault();
            openDrawer(href);
          });
        }
      });
    }

    show('drawer-content');
  } catch (err) {
    hide('drawer-loading');
    const errEl = el('drawer-error');
    if (errEl) {
      errEl.textContent = `Erro ao carregar arquivo: ${err.message}`;
      errEl.classList.remove('hidden');
    }
  }
}

function closeDrawer() {
  const drawer  = el('drawer');
  const overlay = el('drawer-overlay');
  if (drawer)  drawer.style.display = 'none';
  if (overlay) overlay.classList.add('hidden');
  _drawerPath = null;
}

// ─── Frontmatter YAML ─────────────────────────────────────────────────────────

/**
 * Extrai o bloco YAML entre --- e --- do início do markdown.
 * Retorna { frontmatter: Object, body: string }.
 */
function parseFrontmatter(md) {
  if (!md || !md.trimStart().startsWith('---')) {
    return { frontmatter: null, body: md };
  }

  const lines = md.split('\n');
  let endIndex = -1;

  // Pular a primeira linha (---) e procurar o fechamento
  for (let i = 1; i < lines.length; i++) {
    if (lines[i].trim() === '---') {
      endIndex = i;
      break;
    }
  }

  if (endIndex === -1) {
    return { frontmatter: null, body: md };
  }

  const yamlLines = lines.slice(1, endIndex);
  const body      = lines.slice(endIndex + 1).join('\n');
  const frontmatter = {};

  yamlLines.forEach(line => {
    const colonIdx = line.indexOf(':');
    if (colonIdx === -1) return;
    const key   = line.slice(0, colonIdx).trim();
    const value = line.slice(colonIdx + 1).trim().replace(/^['"]|['"]$/g, '');
    if (key) frontmatter[key] = value;
  });

  return { frontmatter, body };
}

/**
 * Renderiza o objeto frontmatter como tabela no drawer.
 */
function renderFrontmatterTable(frontmatter) {
  const container = el('drawer-frontmatter');
  const tbody     = document.querySelector('#frontmatter-table tbody');
  if (!container || !tbody) return;

  tbody.innerHTML = '';

  Object.entries(frontmatter).forEach(([key, value]) => {
    const tr = document.createElement('tr');
    const tdKey = document.createElement('td');
    const tdVal = document.createElement('td');
    tdKey.textContent = key;
    tdVal.textContent = value;
    tr.appendChild(tdKey);
    tr.appendChild(tdVal);
    tbody.appendChild(tr);
  });

  container.classList.remove('hidden');
}

// ─── Escape HTML ──────────────────────────────────────────────────────────────

function escapeHtml(str) {
  if (!str) return '';
  return str
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;');
}

// ─── Atalho de teclado: Escape fecha o drawer ─────────────────────────────────

document.addEventListener('keydown', e => {
  if (e.key === 'Escape') closeDrawer();
});

// ─── Init ─────────────────────────────────────────────────────────────────────

document.addEventListener('DOMContentLoaded', () => {
  switchView('board');
  initRefreshUI();
  startRefreshTimer();
  startAttentionPolling();
});
