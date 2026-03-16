/**
 * Minimal JavaScript for Uptime Monitor
 *
 * Justification:
 * - WebSocket client is required for realtime updates (cannot be done with SSR only).
 *
 * Everything else (create/edit/delete) uses SSR forms and server redirects.
 */
(function () {
  'use strict';

  var ws = null;
  var reconnectAttempts = 0;
  var MAX_RECONNECT = 5;
  var RECONNECT_DELAY = 1000;

  function connect() {
    var protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    ws = new WebSocket(protocol + '//' + window.location.host + '/ws');

    ws.onopen = function () {
      reconnectAttempts = 0;
      setStatus('connected');
    };

    ws.onmessage = function (event) {
      try {
        var msg = JSON.parse(event.data);
        if (msg.type === 'health_check_update') {
          updateMonitorStatus(msg.data);
        }
      } catch (e) {
        console.error('WebSocket parse error:', e);
      }
    };

    ws.onclose = function () {
      setStatus('disconnected');
      if (reconnectAttempts < MAX_RECONNECT) {
        reconnectAttempts++;
        var delay = RECONNECT_DELAY * Math.pow(2, reconnectAttempts - 1);
        setStatus('reconnecting');
        setTimeout(connect, delay);
      } else {
        setStatus('failed');
      }
    };

    ws.onerror = function () {
      setStatus('error');
    };
  }

  function setStatus(status) {
    var indicator = document.getElementById('status-indicator');
    var text = document.getElementById('status-text');
    if (!indicator || !text) return;

    var labels = {
      connected: 'Connected',
      disconnected: 'Disconnected',
      reconnecting: 'Reconnecting…',
      error: 'Error',
      failed: 'Connection failed'
    };

    // Keep CSS responsibility in CSS; just toggle a coarse state class.
    var container = indicator.parentElement;
    if (container) container.className = 'status status--' + status;
    text.textContent = labels[status] || 'Connecting…';
  }

  function updateMonitorStatus(data) {
    if (!data || !data.monitor_id) return;
    var el = document.querySelector('[data-monitor-id="' + data.monitor_id + '"]');
    if (!el) return;

    var statusEl = el.querySelector('.monitor-status');
    if (statusEl && data.status) {
      statusEl.textContent = data.status;
      var cls =
        data.status === 'success'
          ? 'status-badge--success'
          : data.status === 'failure'
            ? 'status-badge--failure'
            : data.status === 'timeout'
              ? 'status-badge--timeout'
              : 'status-badge--unknown';
      statusEl.className = 'monitor-status status-badge ' + cls;
    }

    var rt = el.querySelector('.response-time');
    if (rt && data.response_time_ms != null) rt.textContent = String(data.response_time_ms) + 'ms';

    var lc = el.querySelector('.last-check');
    if (lc && data.checked_at) lc.textContent = new Date(data.checked_at).toLocaleString();
  }

  connect();
})();
