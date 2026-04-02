'use strict';

var BASE_URL = window.location.origin;
var currentJobId = null;
var currentChainId = null;
var activePanel = 'jobs';
var refreshTimer = null;
var activeSSE = null;
var allJobs = [];
var allChains = [];
// Track which lazy sections have been loaded for the current job
var loadedSections = {};

// --- API helpers ---

function apiGet(path) {
	return fetch(BASE_URL + path, {
		headers: { 'Accept': 'application/json' }
	}).then(function(res) {
		if (!res.ok) {
			return res.text().then(function(t) { throw new Error(res.status + ': ' + t); });
		}
		return res.json();
	});
}

function apiPost(path, body) {
	return fetch(BASE_URL + path, {
		method: 'POST',
		headers: {
			'Content-Type': 'application/json',
			'Accept': 'application/json'
		},
		body: JSON.stringify(body || {})
	}).then(function(res) {
		if (!res.ok) {
			return res.text().then(function(t) { throw new Error(res.status + ': ' + t); });
		}
		return res.json();
	});
}

// --- Error display ---

function showError(msg) {
	var el = document.getElementById('error-banner');
	el.textContent = 'Error: ' + msg;
	el.classList.remove('hidden');
	setTimeout(function() { el.classList.add('hidden'); }, 6000);
}

function clearError() {
	document.getElementById('error-banner').classList.add('hidden');
}

// --- Badge helper ---

function badgeClass(status) {
	var s = (status || '').toLowerCase().replace(/ /g, '_');
	var map = {
		'running': 'badge-running',
		'done': 'badge-done',
		'completed': 'badge-completed',
		'failed': 'badge-failed',
		'waiting': 'badge-waiting',
		'waiting_worker': 'badge-waiting',
		'waiting_approval': 'badge-waiting',
		'paused': 'badge-paused',
		'planning': 'badge-planning',
		'cancelled': 'badge-cancelled',
		'canceled': 'badge-cancelled'
	};
	return map[s] || 'badge-default';
}

function makeBadge(status) {
	return '<span class="badge ' + badgeClass(status) + '">' + (status || 'unknown') + '</span>';
}

// --- Time formatter ---

function fmtTime(ts) {
	if (!ts) return '';
	try {
		var d = new Date(ts);
		return d.toLocaleTimeString();
	} catch(e) {
		return ts;
	}
}

// --- Mobile sidebar drawer ---

function toggleSidebar() {
	var sidebar = document.getElementById('sidebar');
	var overlay = document.getElementById('sidebar-overlay');
	var isOpen = sidebar.classList.toggle('sidebar-open');
	overlay.classList.toggle('hidden', !isOpen);
}

function closeSidebar() {
	var sidebar = document.getElementById('sidebar');
	var overlay = document.getElementById('sidebar-overlay');
	sidebar.classList.remove('sidebar-open');
	overlay.classList.add('hidden');
}

// --- Panel switching ---

function showPanel(name) {
	activePanel = name;
	document.getElementById('panel-jobs').classList.toggle('hidden', name !== 'jobs');
	document.getElementById('panel-chains').classList.toggle('hidden', name !== 'chains');
	document.getElementById('btn-jobs').classList.toggle('active', name === 'jobs');
	document.getElementById('btn-chains').classList.toggle('active', name === 'chains');

	if (name === 'jobs') {
		fetchJobs();
	} else {
		fetchChains();
	}
}

// --- Filter logic ---

function getFilterState() {
	var search = (document.getElementById('search-input').value || '').toLowerCase();
	var status = document.getElementById('status-filter').value.toLowerCase();
	return { search: search, status: status };
}

function matchesFilter(item, filter) {
	var id = (item.id || '').toLowerCase();
	var goal = (item.goal || '').toLowerCase();
	var itemStatus = (item.status || '').toLowerCase();

	if (filter.search && id.indexOf(filter.search) === -1 && goal.indexOf(filter.search) === -1) {
		return false;
	}

	if (filter.status) {
		// "waiting" matches waiting, waiting_worker, waiting_approval
		if (filter.status === 'waiting') {
			if (itemStatus.indexOf('waiting') === -1) return false;
		} else if (filter.status === 'done') {
			if (itemStatus !== 'done' && itemStatus !== 'completed') return false;
		} else if (filter.status === 'running') {
			if (itemStatus !== 'running') return false;
		} else if (filter.status === 'failed') {
			if (itemStatus !== 'failed') return false;
		} else if (filter.status === 'cancelled') {
			if (itemStatus !== 'cancelled' && itemStatus !== 'canceled') return false;
		} else {
			if (itemStatus.indexOf(filter.status) === -1) return false;
		}
	}

	return true;
}

function applyFilters() {
	var filter = getFilterState();
	if (activePanel === 'jobs') {
		renderJobList(allJobs.filter(function(j) { return matchesFilter(j, filter); }));
	} else {
		renderChainList(allChains.filter(function(c) { return matchesFilter(c, filter); }));
	}
}

// --- Aggregate token cost summary across all jobs ---

function computeAggregateTokens(jobs) {
	var totals = { input_tokens: 0, output_tokens: 0, total_tokens: 0, estimated_cost_usd: 0 };
	for (var i = 0; i < jobs.length; i++) {
		var u = jobs[i].token_usage;
		if (!u) continue;
		totals.input_tokens += (u.input_tokens || 0);
		totals.output_tokens += (u.output_tokens || 0);
		totals.total_tokens += (u.total_tokens || 0);
		totals.estimated_cost_usd += (u.estimated_cost_usd || 0);
	}
	return totals;
}

function renderStats(totals) {
	document.getElementById('stat-input').textContent = totals.input_tokens.toLocaleString();
	document.getElementById('stat-output').textContent = totals.output_tokens.toLocaleString();
	document.getElementById('stat-total').textContent = totals.total_tokens.toLocaleString();
	document.getElementById('stat-cost').textContent = '$' + totals.estimated_cost_usd.toFixed(4);
}

// --- Jobs ---

function fetchJobs() {
	apiGet('/jobs').then(function(jobs) {
		clearError();
		allJobs = jobs || [];
		var filter = getFilterState();
		renderJobList(allJobs.filter(function(j) { return matchesFilter(j, filter); }));
		renderStats(computeAggregateTokens(allJobs));
		if (currentJobId) {
			fetchJobDetail(currentJobId);
		}
	}).catch(function(err) {
		showError(err.message);
	});
}

function renderJobList(jobs) {
	var el = document.getElementById('job-list');
	if (!jobs.length) {
		el.innerHTML = '<p class="loading">No jobs found.</p>';
		return;
	}
	var html = '';
	for (var i = 0; i < jobs.length; i++) {
		var job = jobs[i];
		var sel = job.id === currentJobId ? ' selected' : '';
		html += '<div class="list-item' + sel + '" data-job-id="' + esc(job.id) + '">';
		html += '<div class="list-item-header">';
		html += '<span class="list-item-id">' + esc(job.id) + '</span>';
		html += makeBadge(job.status);
		html += '</div>';
		html += '<div class="list-item-goal">' + esc(truncate(job.goal, 80)) + '</div>';
		html += '</div>';
	}
	el.innerHTML = html;
}

function fetchJobDetail(id) {
	apiGet('/jobs/' + id).then(function(job) {
		clearError();
		currentJobId = id;
		renderJobDetail(job);
		openSSE(id, job);
	}).catch(function(err) {
		showError(err.message);
	});
}

function renderJobDetail(job) {
	document.getElementById('detail-placeholder').classList.add('hidden');
	document.getElementById('chain-detail').classList.add('hidden');
	var el = document.getElementById('job-detail');
	el.classList.remove('hidden');

	document.getElementById('detail-job-id').textContent = job.id || '';
	document.getElementById('detail-job-goal').textContent = job.goal || '';

	var statusEl = document.getElementById('detail-job-status');
	statusEl.className = 'badge ' + badgeClass(job.status);
	statusEl.textContent = job.status || 'unknown';

	// Contextual action buttons based on status
	var s = (job.status || '').toLowerCase();
	var needsApproval = s === 'waiting_approval';
	var isTerminal = s === 'done' || s === 'failed' || s === 'cancelled' || s === 'canceled';
	document.getElementById('btn-approve').classList.toggle('hidden', !needsApproval);
	document.getElementById('btn-reject').classList.toggle('hidden', !needsApproval);
	document.getElementById('btn-cancel').classList.toggle('hidden', isTerminal);
	document.getElementById('btn-retry').classList.toggle('hidden', s !== 'failed');
	document.getElementById('btn-resume').classList.toggle('hidden', s !== 'paused');
	document.getElementById('job-actions').classList.remove('hidden');

	// Token usage
	renderTokenUsage(job.token_usage);

	// Steps
	renderSteps(job.steps || []);

	// Steer history from events
	renderSteerHistory(job.events || []);

	// Events
	renderEvents(job.events || []);

	// Reset lazy-loaded sections on job switch
	loadedSections = {};
	document.getElementById('detail-artifacts').innerHTML = '<p class="secondary loading-small">Expand to load...</p>';
	document.getElementById('detail-planning').innerHTML = '<p class="secondary loading-small">Expand to load...</p>';
	document.getElementById('detail-evaluator').innerHTML = '<p class="secondary loading-small">Expand to load...</p>';

	// Mark selected in list
	var items = document.querySelectorAll('#job-list .list-item');
	for (var i = 0; i < items.length; i++) {
		items[i].classList.toggle('selected', items[i].getAttribute('data-job-id') === job.id);
	}
}

function renderTokenUsage(usage) {
	var el = document.getElementById('detail-token-usage');
	if (!usage) {
		el.innerHTML = '<p class="secondary">No token data.</p>';
		return;
	}
	var fields = [
		['input_tokens', 'Input'],
		['output_tokens', 'Output'],
		['total_tokens', 'Total'],
		['estimated_cost_usd', 'Cost (USD)']
	];
	var html = '';
	for (var i = 0; i < fields.length; i++) {
		var key = fields[i][0];
		var label = fields[i][1];
		var val = usage[key];
		if (val === undefined || val === null) continue;
		var display = key === 'estimated_cost_usd' ? '$' + Number(val).toFixed(4) : val.toLocaleString();
		html += '<div class="token-card">';
		html += '<div class="token-value">' + esc(display) + '</div>';
		html += '<div class="token-label">' + esc(label) + '</div>';
		html += '</div>';
	}
	el.innerHTML = html || '<p class="secondary">No token data.</p>';
}

function renderSteps(steps) {
	var el = document.getElementById('detail-steps');
	if (!steps.length) {
		el.innerHTML = '<p class="secondary">No steps.</p>';
		return;
	}
	var html = '';
	for (var i = 0; i < steps.length; i++) {
		var step = steps[i];
		html += '<div class="step-item">';
		html += '<div class="step-header">';
		html += '<span class="step-index">#' + (step.index || i + 1) + '</span>';
		html += makeBadge(step.status);
		html += '</div>';
		if (step.task_text) {
			html += '<div class="step-task">' + esc(truncate(step.task_text, 200)) + '</div>';
		}
		html += '</div>';
	}
	el.innerHTML = html;
}

// Extract and render steer events from the job events array
function renderSteerHistory(events) {
	var el = document.getElementById('detail-steer-history');
	var steerEvents = (events || []).filter(function(ev) {
		return (ev.kind || '').indexOf('steer') !== -1;
	});
	if (!steerEvents.length) {
		el.innerHTML = '<p class="secondary">No steer messages.</p>';
		return;
	}
	var html = '';
	for (var i = 0; i < steerEvents.length; i++) {
		var ev = steerEvents[i];
		html += '<div class="steer-history-item">';
		html += '<div class="steer-history-meta">';
		html += '<span class="event-time">' + esc(fmtTime(ev.time)) + '</span>';
		html += '<span class="event-kind">' + esc(ev.kind || '') + '</span>';
		html += '</div>';
		html += '<div class="steer-history-msg">' + esc(ev.message || '') + '</div>';
		html += '</div>';
	}
	el.innerHTML = html;
}

function renderEvents(events) {
	var el = document.getElementById('detail-events');
	if (!events.length) {
		el.innerHTML = '<p class="secondary">No events.</p>';
		return;
	}
	var html = '';
	// Latest events first
	for (var i = events.length - 1; i >= 0; i--) {
		var ev = events[i];
		html += '<div class="event-item">';
		html += '<span class="event-time">' + esc(fmtTime(ev.time)) + '</span>';
		html += '<span class="event-kind">' + esc(ev.kind || '') + '</span>';
		html += '<span class="event-message">' + esc(ev.message || '') + '</span>';
		html += '</div>';
	}
	el.innerHTML = html;
}

// --- SSE Real-time Event Streaming ---

function openSSE(jobId, job) {
	closeSSE();

	var s = (job && job.status || '').toLowerCase();
	var isTerminal = s === 'done' || s === 'failed' || s === 'cancelled' || s === 'canceled';
	if (isTerminal) {
		setSSEIndicator('sse-off', 'Job finished');
		return;
	}

	var sseEl = document.getElementById('detail-sse-events');
	sseEl.innerHTML = '';
	setSSEIndicator('sse-connecting', 'Connecting...');

	var es = new EventSource(BASE_URL + '/jobs/' + jobId + '/events/stream');
	activeSSE = es;

	es.addEventListener('job_event', function(e) {
		try {
			var ev = JSON.parse(e.data);
			setSSEIndicator('sse-on', 'Live');

			var item = document.createElement('div');
			item.className = 'event-item sse-event-new';
			item.innerHTML =
				'<span class="event-time">' + esc(fmtTime(ev.time)) + '</span>' +
				'<span class="event-kind">' + esc(ev.kind || '') + '</span>' +
				'<span class="event-message">' + esc(ev.message || '') + '</span>';
			sseEl.insertBefore(item, sseEl.firstChild);

			// Cap live events at 50 to avoid DOM bloat
			while (sseEl.children.length > 50) {
				sseEl.removeChild(sseEl.lastChild);
			}

			// When the job reaches a terminal state via SSE, close and refresh
			var k = ev.kind || '';
			if (k === 'job_done' || k === 'job_failed' || k === 'job_cancelled') {
				closeSSE();
				fetchJobDetail(jobId);
			}
		} catch(ex) {}
	});

	es.onerror = function() {
		setSSEIndicator('sse-off', 'Disconnected');
	};
}

function closeSSE() {
	if (activeSSE) {
		activeSSE.close();
		activeSSE = null;
	}
	setSSEIndicator('sse-off', 'Disconnected');
}

function setSSEIndicator(cls, title) {
	var ind = document.getElementById('sse-indicator');
	if (!ind) return;
	ind.className = 'sse-indicator ' + cls;
	ind.title = title;
}

// --- Collapsible sub-views ---

function toggleSection(id) {
	var body = document.getElementById(id);
	var icon = document.getElementById('icon-' + id);
	if (!body) return;

	var isNowCollapsed = body.classList.toggle('collapsed');
	if (icon) {
		// right-arrow = collapsed, down-arrow = expanded
		icon.innerHTML = isNowCollapsed ? '&#9654;' : '&#9660;';
	}

	// Lazy-load data on first expand
	if (!isNowCollapsed && !loadedSections[id]) {
		loadedSections[id] = true;
		if (id === 'section-artifacts') loadArtifacts();
		if (id === 'section-planning') loadPlanning();
		if (id === 'section-evaluator') loadEvaluator();
	}
}

function loadArtifacts() {
	if (!currentJobId) return;
	var el = document.getElementById('detail-artifacts');
	el.innerHTML = '<p class="secondary">Loading...</p>';
	apiGet('/jobs/' + currentJobId + '/artifacts').then(function(artifacts) {
		if (!artifacts || !artifacts.length) {
			el.innerHTML = '<p class="secondary">No artifacts.</p>';
			return;
		}
		var html = '<ul class="artifact-list">';
		for (var i = 0; i < artifacts.length; i++) {
			html += '<li class="artifact-item">' + esc(artifacts[i]) + '</li>';
		}
		html += '</ul>';
		el.innerHTML = html;
	}).catch(function(err) {
		el.innerHTML = '<p class="secondary">Failed: ' + esc(err.message) + '</p>';
	});
}

function loadPlanning() {
	if (!currentJobId) return;
	var el = document.getElementById('detail-planning');
	el.innerHTML = '<p class="secondary">Loading...</p>';
	apiGet('/jobs/' + currentJobId + '/planning').then(function(data) {
		el.innerHTML = '<pre class="json-view">' + esc(JSON.stringify(data, null, 2)) + '</pre>';
	}).catch(function(err) {
		el.innerHTML = '<p class="secondary">Failed: ' + esc(err.message) + '</p>';
	});
}

function loadEvaluator() {
	if (!currentJobId) return;
	var el = document.getElementById('detail-evaluator');
	el.innerHTML = '<p class="secondary">Loading...</p>';
	apiGet('/jobs/' + currentJobId + '/evaluator').then(function(data) {
		el.innerHTML = '<pre class="json-view">' + esc(JSON.stringify(data, null, 2)) + '</pre>';
	}).catch(function(err) {
		el.innerHTML = '<p class="secondary">Failed: ' + esc(err.message) + '</p>';
	});
}

// --- Chains ---

function fetchChains() {
	apiGet('/chains').then(function(chains) {
		clearError();
		allChains = chains || [];
		var filter = getFilterState();
		renderChainList(allChains.filter(function(c) { return matchesFilter(c, filter); }));
		if (currentChainId) {
			fetchChainDetail(currentChainId);
		}
	}).catch(function(err) {
		showError(err.message);
	});
}

function renderChainList(chains) {
	var el = document.getElementById('chain-list');
	if (!chains.length) {
		el.innerHTML = '<p class="loading">No chains found.</p>';
		return;
	}
	var html = '';
	for (var i = 0; i < chains.length; i++) {
		var chain = chains[i];
		var sel = chain.id === currentChainId ? ' selected' : '';
		html += '<div class="list-item' + sel + '" data-chain-id="' + esc(chain.id) + '">';
		html += '<div class="list-item-header">';
		html += '<span class="list-item-id">' + esc(chain.id) + '</span>';
		html += makeBadge(chain.status);
		html += '</div>';
		var goals = chain.goals || [];
		var done = 0;
		for (var j = 0; j < goals.length; j++) {
			if (goals[j].status === 'done' || goals[j].status === 'completed') done++;
		}
		if (goals.length > 0) {
			var pct = Math.round((done / goals.length) * 100);
			html += '<div class="chain-progress-bar"><div class="chain-progress-fill" style="width:' + pct + '%"></div></div>';
			html += '<div class="list-item-goal">' + done + ' / ' + goals.length + ' goals</div>';
		} else {
			html += '<div class="list-item-goal">' + esc(truncate(chain.goal || '', 80)) + '</div>';
		}
		html += '</div>';
	}
	el.innerHTML = html;
}

function fetchChainDetail(id) {
	apiGet('/chains/' + id).then(function(chain) {
		clearError();
		currentChainId = id;
		renderChainDetail(chain);
	}).catch(function(err) {
		showError(err.message);
	});
}

function renderChainDetail(chain) {
	document.getElementById('detail-placeholder').classList.add('hidden');
	document.getElementById('job-detail').classList.add('hidden');
	closeSSE();
	var el = document.getElementById('chain-detail');
	el.classList.remove('hidden');

	document.getElementById('detail-chain-id').textContent = chain.id || '';
	document.getElementById('detail-chain-goal').textContent = chain.goal || '';

	var statusEl = document.getElementById('detail-chain-status');
	statusEl.className = 'badge ' + badgeClass(chain.status);
	statusEl.textContent = chain.status || 'unknown';

	// Overall chain progress bar
	var goals = chain.goals || [];
	var done = 0;
	for (var i = 0; i < goals.length; i++) {
		if (goals[i].status === 'done' || goals[i].status === 'completed') done++;
	}
	var progressEl = document.getElementById('detail-chain-progress');
	if (goals.length > 0) {
		var pct = Math.round((done / goals.length) * 100);
		progressEl.innerHTML =
			'<div class="chain-progress-bar"><div class="chain-progress-fill" style="width:' + pct + '%"></div></div>' +
			'<p class="secondary">' + done + ' of ' + goals.length + ' goals completed (' + pct + '%)</p>';
	} else {
		progressEl.innerHTML = '<p class="secondary">No goals.</p>';
	}

	// Per-goal progress bars with status badges
	var goalsEl = document.getElementById('detail-chain-goals');
	if (!goals.length) {
		goalsEl.innerHTML = '<p class="secondary">No goals.</p>';
	} else {
		var html = '';
		for (var j = 0; j < goals.length; j++) {
			var g = goals[j];
			var gDone = g.status === 'done' || g.status === 'completed';
			var gFailed = g.status === 'failed';
			var gRunning = g.status === 'running';
			var gPct = gDone ? 100 : (gFailed ? 100 : (gRunning ? 50 : 0));
			var fillColor = gFailed ? '#e94560' : (gDone ? '#4CAF50' : (gRunning ? '#0f3460' : '#1e2a4a'));

			html += '<div class="chain-goal-item">';
			html += '<span class="chain-goal-index">' + (j + 1) + '</span>';
			html += '<div class="chain-goal-body">';
			html += '<div class="chain-goal-header">';
			html += '<span class="chain-goal-text">' + esc(g.goal || g.text || JSON.stringify(g)) + '</span>';
			html += makeBadge(g.status);
			html += '</div>';
			html += '<div class="chain-progress-bar goal-progress-bar">';
			html += '<div class="chain-progress-fill" style="width:' + gPct + '%;background:' + fillColor + '"></div>';
			html += '</div>';
			html += '</div>';
			html += '</div>';
		}
		goalsEl.innerHTML = html;
	}

	// Mark selected in chain list
	var items = document.querySelectorAll('#chain-list .list-item');
	for (var k = 0; k < items.length; k++) {
		items[k].classList.toggle('selected', items[k].getAttribute('data-chain-id') === chain.id);
	}
}

// --- Actions ---

function steerJob(id, message) {
	return apiPost('/jobs/' + id + '/steer', { message: message });
}

function approveJob(id) {
	return apiPost('/jobs/' + id + '/approve', {});
}

function rejectJob(id) {
	return apiPost('/jobs/' + id + '/reject', {});
}

function cancelJob(id) {
	return apiPost('/jobs/' + id + '/cancel', {});
}

function retryJob(id) {
	return apiPost('/jobs/' + id + '/retry', {});
}

function resumeJob(id) {
	return apiPost('/jobs/' + id + '/resume', {});
}

function steerCurrentJob() {
	if (!currentJobId) return;
	var msg = document.getElementById('steer-input').value.trim();
	if (!msg) { showError('Steer message cannot be empty.'); return; }
	steerJob(currentJobId, msg).then(function() {
		clearError();
		document.getElementById('steer-input').value = '';
		fetchJobDetail(currentJobId);
	}).catch(function(err) {
		showError(err.message);
	});
}

function approveCurrentJob() {
	if (!currentJobId) return;
	approveJob(currentJobId).then(function() {
		clearError();
		fetchJobDetail(currentJobId);
		fetchJobs();
	}).catch(function(err) {
		showError(err.message);
	});
}

function rejectCurrentJob() {
	if (!currentJobId) return;
	rejectJob(currentJobId).then(function() {
		clearError();
		fetchJobDetail(currentJobId);
		fetchJobs();
	}).catch(function(err) {
		showError(err.message);
	});
}

function cancelCurrentJob() {
	if (!currentJobId) return;
	if (!confirm('Cancel job ' + currentJobId + '?')) return;
	cancelJob(currentJobId).then(function() {
		clearError();
		fetchJobDetail(currentJobId);
		fetchJobs();
	}).catch(function(err) {
		showError(err.message);
	});
}

function retryCurrentJob() {
	if (!currentJobId) return;
	retryJob(currentJobId).then(function() {
		clearError();
		fetchJobDetail(currentJobId);
		fetchJobs();
	}).catch(function(err) {
		showError(err.message);
	});
}

function resumeCurrentJob() {
	if (!currentJobId) return;
	resumeJob(currentJobId).then(function() {
		clearError();
		fetchJobDetail(currentJobId);
		fetchJobs();
	}).catch(function(err) {
		showError(err.message);
	});
}

// --- Event delegation for list clicks ---

document.addEventListener('click', function(e) {
	var jobItem = e.target.closest('[data-job-id]');
	if (jobItem) {
		fetchJobDetail(jobItem.getAttribute('data-job-id'));
		closeSidebar();
		return;
	}
	var chainItem = e.target.closest('[data-chain-id]');
	if (chainItem) {
		fetchChainDetail(chainItem.getAttribute('data-chain-id'));
		closeSidebar();
		return;
	}
});

// --- Utility ---

function esc(str) {
	return String(str)
		.replace(/&/g, '&amp;')
		.replace(/</g, '&lt;')
		.replace(/>/g, '&gt;')
		.replace(/"/g, '&quot;');
}

function truncate(str, n) {
	if (!str) return '';
	return str.length > n ? str.slice(0, n) + '...' : str;
}

// --- Auto-refresh ---

function tick() {
	if (activePanel === 'jobs') {
		fetchJobs();
	} else {
		fetchChains();
	}
}

function startAutoRefresh() {
	if (refreshTimer) clearInterval(refreshTimer);
	refreshTimer = setInterval(tick, 10000);
}

// --- Init ---

fetchJobs();
startAutoRefresh();
