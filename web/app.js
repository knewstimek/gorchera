'use strict';

var BASE_URL = window.location.origin;
var currentJobId = null;
var currentChainId = null;
var activePanel = 'jobs';
var refreshTimer = null;

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

// --- Jobs ---

function fetchJobs() {
	apiGet('/jobs').then(function(jobs) {
		clearError();
		renderJobList(jobs || []);
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

	// Show/hide approve/reject buttons
	var actionsEl = document.getElementById('job-actions');
	var needsApproval = job.status === 'waiting_approval';
	actionsEl.classList.toggle('hidden', !needsApproval);

	// Token usage
	renderTokenUsage(job.token_usage);

	// Steps
	renderSteps(job.steps || []);

	// Events
	renderEvents(job.events || []);

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

function renderEvents(events) {
	var el = document.getElementById('detail-events');
	if (!events.length) {
		el.innerHTML = '<p class="secondary">No events.</p>';
		return;
	}
	var html = '';
	// Show latest events first
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

// --- Chains ---

function fetchChains() {
	apiGet('/chains').then(function(chains) {
		clearError();
		renderChainList(chains || []);
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
	var el = document.getElementById('chain-detail');
	el.classList.remove('hidden');

	document.getElementById('detail-chain-id').textContent = chain.id || '';
	document.getElementById('detail-chain-goal').textContent = chain.goal || '';

	var statusEl = document.getElementById('detail-chain-status');
	statusEl.className = 'badge ' + badgeClass(chain.status);
	statusEl.textContent = chain.status || 'unknown';

	// Progress
	var goals = chain.goals || [];
	var done = 0;
	for (var i = 0; i < goals.length; i++) {
		if (goals[i].status === 'done' || goals[i].status === 'completed') done++;
	}
	var progressEl = document.getElementById('detail-chain-progress');
	if (goals.length > 0) {
		var pct = Math.round((done / goals.length) * 100);
		progressEl.innerHTML = '<div class="chain-progress-bar"><div class="chain-progress-fill" style="width:' + pct + '%"></div></div>' +
			'<p class="secondary">' + done + ' of ' + goals.length + ' goals completed (' + pct + '%)</p>';
	} else {
		progressEl.innerHTML = '<p class="secondary">No goals.</p>';
	}

	// Goals list
	var goalsEl = document.getElementById('detail-chain-goals');
	if (!goals.length) {
		goalsEl.innerHTML = '<p class="secondary">No goals.</p>';
	} else {
		var html = '';
		for (var j = 0; j < goals.length; j++) {
			var g = goals[j];
			html += '<div class="chain-goal-item">';
			html += '<span class="chain-goal-index">' + (j + 1) + '</span>';
			html += '<span class="chain-goal-text">' + esc(g.goal || g.text || JSON.stringify(g)) + '</span>';
			html += makeBadge(g.status);
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

// --- Event delegation ---

document.addEventListener('click', function(e) {
	var jobItem = e.target.closest('[data-job-id]');
	if (jobItem) {
		fetchJobDetail(jobItem.getAttribute('data-job-id'));
		return;
	}
	var chainItem = e.target.closest('[data-chain-id]');
	if (chainItem) {
		fetchChainDetail(chainItem.getAttribute('data-chain-id'));
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
