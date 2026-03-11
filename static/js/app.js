/**
 * Minimal JavaScript for Uptime Monitor
 *
 * - WebSocket: real-time health check + alert updates (cannot be done server-side)
 * - Monitor modal: auto-open details elements with open-details class
 */
(function () {
  'use strict';

  // --- WebSocket ---
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
        var data = JSON.parse(event.data);
        if (data.type === 'health_check_update') {
          updateMonitorStatus(data.data);
        } else if (data.type === 'alert') {
          showAlertNotification(data.data);
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

    var colors = {
      connected: 'bg-teal-500',
      disconnected: 'bg-red-400',
      reconnecting: 'bg-gray-400 animate-pulse-dot',
      error: 'bg-red-400',
      failed: 'bg-red-400'
    };

    var labels = {
      connected: 'Connected',
      disconnected: 'Disconnected',
      reconnecting: 'Reconnecting…',
      error: 'Error',
      failed: 'Connection failed'
    };

    indicator.className = 'w-2 h-2 rounded-full ' + (colors[status] || 'bg-gray-300');
    text.textContent = labels[status] || 'Connecting…';
  }

  function updateMonitorStatus(data) {
    var el = document.querySelector('[data-monitor-id="' + data.monitor_id + '"]');
    if (!el) return;

    var statusEl = el.querySelector('.monitor-status');
    if (statusEl) {
      statusEl.textContent = data.status;
      statusEl.className = 'monitor-status inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium ';
      if (data.status === 'success') {
        statusEl.className += 'bg-teal-50 text-teal-700';
      } else if (data.status === 'failure') {
        statusEl.className += 'bg-red-50 text-red-700';
      } else {
        statusEl.className += 'bg-gray-100 text-gray-500';
      }
    }

    var rt = el.querySelector('.response-time');
    if (rt && data.response_time_ms) rt.textContent = data.response_time_ms + 'ms';

    var lc = el.querySelector('.last-check');
    if (lc && data.checked_at) lc.textContent = new Date(data.checked_at).toLocaleString();
  }

  function showAlertNotification(alert) {
    var node = document.createElement('div');
    node.className = 'fixed top-4 right-4 max-w-sm z-[60] bg-white border border-gray-200 rounded-lg shadow-lg p-4';
    node.innerHTML =
      '<p class="text-sm font-medium text-gray-900">' + (alert.type || 'Alert') + '</p>' +
      '<p class="text-sm text-gray-500 mt-1">' + (alert.message || '') + '</p>';
    document.body.appendChild(node);
    setTimeout(function () {
      if (node.parentNode) node.remove();
    }, 5000);
  }

  // --- Modal Logic ---
  window.openNewMonitorModal = function() {
    var titleEl = document.getElementById('modal-title');
    if (!titleEl) return; // Prevent errors if modal not on page
    
    titleEl.textContent = 'Add Monitor';
    document.getElementById('modal-submit-btn').textContent = 'Create';
    document.getElementById('monitor-id').value = '';
    document.getElementById('monitor-form').reset();
    hideModalError();
    
    // Close all details
    document.querySelectorAll('#monitor-modal details').forEach(function(el) {
      el.removeAttribute('open');
    });
    
    document.getElementById('monitor-modal').showModal();
  };

  window.openEditMonitorModal = function(id) {
    var titleEl = document.getElementById('modal-title');
    if (!titleEl) return;
    
    titleEl.textContent = 'Edit Monitor';
    document.getElementById('modal-submit-btn').textContent = 'Update';
    document.getElementById('monitor-id').value = id;
    document.getElementById('monitor-form').reset();
    hideModalError();
    
    // Fetch existing monitor data
    fetch('/api/v1/monitors/' + id)
      .then(function(res) { 
        if (!res.ok) throw new Error('Failed to fetch monitor');
        return res.json(); 
      })
      .then(function(data) {
        document.getElementById('monitor-name').value = data.name || '';
        document.getElementById('monitor-url').value = data.url || '';
        document.getElementById('monitor-interval').value = data.check_interval || '5';
        document.getElementById('monitor-enabled').checked = data.enabled;
        
        // Reset channels
        document.getElementById('telegram-enabled').checked = false;
        document.getElementById('telegram-bot-token').value = '';
        document.getElementById('telegram-chat-id').value = '';
        document.getElementById('email-enabled').checked = false;
        document.getElementById('email-to').value = '';
        document.getElementById('webhook-enabled').checked = false;
        document.getElementById('webhook-url').value = '';
        
        document.querySelectorAll('#monitor-modal details').forEach(function(el) {
          el.removeAttribute('open');
        });
        
        if (data.alert_channels) {
          data.alert_channels.forEach(function(ch) {
            if (ch.Type === 'telegram' || ch.type === 'telegram') {
              document.getElementById('telegram-enabled').checked = true;
              document.getElementById('telegram-bot-token').value = (ch.Config || ch.config || {}).bot_token || '';
              document.getElementById('telegram-chat-id').value = (ch.Config || ch.config || {}).chat_id || '';
              document.getElementById('telegram-details').setAttribute('open', '');
            } else if (ch.Type === 'email' || ch.type === 'email') {
              document.getElementById('email-enabled').checked = true;
              document.getElementById('email-to').value = (ch.Config || ch.config || {}).to || '';
              document.getElementById('email-details').setAttribute('open', '');
            } else if (ch.Type === 'webhook' || ch.type === 'webhook') {
              document.getElementById('webhook-enabled').checked = true;
              document.getElementById('webhook-url').value = (ch.Config || ch.config || {}).url || '';
              document.getElementById('webhook-details').setAttribute('open', '');
            }
          });
        }
      })
      .catch(function(err) {
        console.error('Failed to load monitor data', err);
        showModalError('Failed to load monitor data');
      });
      
    document.getElementById('monitor-modal').showModal();
  };

  window.closeMonitorModal = function() {
    var modal = document.getElementById('monitor-modal');
    if (modal) {
        modal.close();
        hideModalError();
    }
  };

  window.submitMonitorForm = function(event) {
    event.preventDefault();
    hideModalError();
    
    var id = document.getElementById('monitor-id').value;
    var isEdit = !!id;
    var method = isEdit ? 'PUT' : 'POST';
    var url = isEdit ? '/api/v1/monitors/' + id : '/api/v1/monitors';
    
    var payload = {
      name: document.getElementById('monitor-name').value,
      url: document.getElementById('monitor-url').value,
      check_interval: parseInt(document.getElementById('monitor-interval').value, 10),
      enabled: document.getElementById('monitor-enabled').checked,
      alert_channels: []
    };
    
    if (document.getElementById('telegram-enabled').checked) {
      payload.alert_channels.push({
        type: 'telegram',
        config: {
          bot_token: document.getElementById('telegram-bot-token').value,
          chat_id: document.getElementById('telegram-chat-id').value
        }
      });
    }
    
    if (document.getElementById('email-enabled').checked) {
      payload.alert_channels.push({
        type: 'email',
        config: {
          to: document.getElementById('email-to').value
        }
      });
    }
    
    if (document.getElementById('webhook-enabled').checked) {
      payload.alert_channels.push({
        type: 'webhook',
        config: {
          url: document.getElementById('webhook-url').value
        }
      });
    }
    
    fetch(url, {
      method: method,
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload)
    })
    .then(function(res) {
      if (!res.ok) {
        return res.text().then(function(text) { throw new Error(text || 'Operation failed'); });
      }
      return res.json();
    })
    .then(function() {
      closeMonitorModal();
      window.location.reload();
    })
    .catch(function(err) {
      showModalError(err.message);
    });
    
    return false;
  };

  function showModalError(msg) {
    var errDiv = document.getElementById('modal-error');
    var errText = document.getElementById('modal-error-text');
    if (errDiv && errText) {
      errText.textContent = msg;
      errDiv.classList.remove('hidden');
    }
  }

  function hideModalError() {
    var errDiv = document.getElementById('modal-error');
    if (errDiv) {
      errDiv.classList.add('hidden');
    }
  }

  connect();
})();
