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
  var modalState = {
    isEdit: false,
    monitorID: ''
  };

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

  function getModalElements() {
    return {
      dialog: document.getElementById('monitor-modal'),
      form: document.getElementById('monitor-form'),
      monitorID: document.getElementById('monitor-id'),
      title: document.getElementById('modal-title'),
      submitBtn: document.getElementById('modal-submit-btn'),
      errorBox: document.getElementById('modal-error'),
      errorText: document.getElementById('modal-error-text'),
      name: document.getElementById('monitor-name'),
      url: document.getElementById('monitor-url'),
      interval: document.getElementById('monitor-interval'),
      enabled: document.getElementById('monitor-enabled'),
      telegramEnabled: document.getElementById('telegram-enabled'),
      telegramDetails: document.getElementById('telegram-details'),
      telegramBotToken: document.getElementById('telegram-bot-token'),
      telegramChatID: document.getElementById('telegram-chat-id'),
      emailEnabled: document.getElementById('email-enabled'),
      emailDetails: document.getElementById('email-details'),
      emailTo: document.getElementById('email-to'),
      webhookEnabled: document.getElementById('webhook-enabled'),
      webhookDetails: document.getElementById('webhook-details'),
      webhookURL: document.getElementById('webhook-url')
    };
  }

  function showModal(dialog) {
    if (!dialog) return;
    if (typeof dialog.showModal === 'function') {
      dialog.showModal();
      return;
    }
    dialog.setAttribute('open', 'open');
  }

  function hideModal(dialog) {
    if (!dialog) return;
    if (typeof dialog.close === 'function') {
      dialog.close();
      return;
    }
    dialog.removeAttribute('open');
  }

  function setChannelSection(checkbox, details, enabled) {
    if (!checkbox || !details) return;
    checkbox.checked = enabled;
    details.open = enabled;
  }

  function clearModalError(elements) {
    if (!elements.errorBox || !elements.errorText) return;
    elements.errorBox.classList.add('hidden');
    elements.errorText.textContent = '';
  }

  function showModalError(elements, message) {
    if (!elements.errorBox || !elements.errorText) return;
    elements.errorText.textContent = message || 'Something went wrong.';
    elements.errorBox.classList.remove('hidden');
  }

  function resetMonitorForm() {
    var elements = getModalElements();
    if (!elements.form) return elements;

    elements.form.reset();
    modalState.isEdit = false;
    modalState.monitorID = '';
    if (elements.monitorID) elements.monitorID.value = '';
    if (elements.title) elements.title.textContent = 'Add Monitor';
    if (elements.submitBtn) {
      elements.submitBtn.textContent = 'Create';
      elements.submitBtn.disabled = false;
    }
    if (elements.enabled) elements.enabled.checked = true;
    setChannelSection(elements.telegramEnabled, elements.telegramDetails, false);
    setChannelSection(elements.emailEnabled, elements.emailDetails, false);
    setChannelSection(elements.webhookEnabled, elements.webhookDetails, false);
    clearModalError(elements);
    return elements;
  }

  function findAlertChannel(channels, type) {
    if (!Array.isArray(channels)) return null;
    for (var i = 0; i < channels.length; i++) {
      if (channels[i] && channels[i].type === type) return channels[i];
    }
    return null;
  }

  function buildMonitorPayload(elements) {
    var payload = new URLSearchParams();

    payload.set('name', elements.name ? elements.name.value.trim() : '');
    payload.set('url', elements.url ? elements.url.value.trim() : '');
    payload.set('check_interval', elements.interval ? elements.interval.value : '');

    if (elements.enabled && elements.enabled.checked) {
      payload.set('enabled', 'on');
    }
    if (elements.telegramEnabled && elements.telegramEnabled.checked) {
      payload.set('telegram_enabled', 'on');
      payload.set('telegram_bot_token', elements.telegramBotToken ? elements.telegramBotToken.value.trim() : '');
      payload.set('telegram_chat_id', elements.telegramChatID ? elements.telegramChatID.value.trim() : '');
    }
    if (elements.emailEnabled && elements.emailEnabled.checked) {
      payload.set('email_enabled', 'on');
      payload.set('email_to', elements.emailTo ? elements.emailTo.value.trim() : '');
    }
    if (elements.webhookEnabled && elements.webhookEnabled.checked) {
      payload.set('webhook_enabled', 'on');
      payload.set('webhook_url', elements.webhookURL ? elements.webhookURL.value.trim() : '');
    }
    if (modalState.isEdit) {
      payload.set('_method', 'PUT');
    }

    return payload;
  }

  function submitPayload(action, payload) {
    var form = document.createElement('form');
    form.method = 'POST';
    form.action = action;
    form.style.display = 'none';

    payload.forEach(function (value, key) {
      var input = document.createElement('input');
      input.type = 'hidden';
      input.name = key;
      input.value = value;
      form.appendChild(input);
    });

    document.body.appendChild(form);
    form.submit();
  }

  function openNewMonitorModal() {
    var elements = resetMonitorForm();
    if (!elements.dialog) {
      window.location.href = '/monitors/new';
      return;
    }
    showModal(elements.dialog);
  }

  function openEditMonitorModal(monitorID) {
    if (!monitorID) return;

    var elements = resetMonitorForm();
    if (!elements.dialog) {
      window.location.href = '/monitors/' + encodeURIComponent(monitorID) + '/edit';
      return;
    }

    modalState.isEdit = true;
    modalState.monitorID = monitorID;
    if (elements.monitorID) elements.monitorID.value = monitorID;
    if (elements.title) elements.title.textContent = 'Edit Monitor';
    if (elements.submitBtn) elements.submitBtn.textContent = 'Save Changes';
    showModal(elements.dialog);

    fetch('/api/v1/monitors/' + encodeURIComponent(monitorID), {
      headers: {
        Accept: 'application/json'
      },
      credentials: 'same-origin'
    })
      .then(function (response) {
        if (!response.ok) {
          throw new Error('Failed to load monitor details.');
        }
        return response.json();
      })
      .then(function (monitor) {
        if (elements.name) elements.name.value = monitor.name || '';
        if (elements.url) elements.url.value = monitor.url || '';
        if (elements.interval) elements.interval.value = monitor.check_interval != null ? String(monitor.check_interval) : '';
        if (elements.enabled) elements.enabled.checked = !!monitor.enabled;

        var telegram = findAlertChannel(monitor.alert_channels, 'telegram');
        setChannelSection(elements.telegramEnabled, elements.telegramDetails, !!telegram);
        if (elements.telegramBotToken) elements.telegramBotToken.value = telegram && telegram.config ? telegram.config.bot_token || '' : '';
        if (elements.telegramChatID) elements.telegramChatID.value = telegram && telegram.config ? telegram.config.chat_id || '' : '';

        var email = findAlertChannel(monitor.alert_channels, 'email');
        setChannelSection(elements.emailEnabled, elements.emailDetails, !!email);
        if (elements.emailTo) elements.emailTo.value = email && email.config ? email.config.to || '' : '';

        var webhook = findAlertChannel(monitor.alert_channels, 'webhook');
        setChannelSection(elements.webhookEnabled, elements.webhookDetails, !!webhook);
        if (elements.webhookURL) elements.webhookURL.value = webhook && webhook.config ? webhook.config.url || '' : '';
      })
      .catch(function () {
        showModalError(elements, 'Unable to load monitor details right now.');
      });
  }

  function closeMonitorModal() {
    var elements = getModalElements();
    if (!elements.dialog) return;
    hideModal(elements.dialog);
    clearModalError(elements);
    if (elements.submitBtn) elements.submitBtn.disabled = false;
  }

  function submitMonitorForm(event) {
    if (event) event.preventDefault();

    var elements = getModalElements();
    if (!elements.form) return false;

    clearModalError(elements);
    if (elements.submitBtn) elements.submitBtn.disabled = true;

    var payload = buildMonitorPayload(elements);
    var action = modalState.isEdit && modalState.monitorID
      ? '/monitors/' + encodeURIComponent(modalState.monitorID)
      : '/monitors';

    submitPayload(action, payload);
    return false;
  }

  window.openNewMonitorModal = openNewMonitorModal;
  window.openEditMonitorModal = openEditMonitorModal;
  window.closeMonitorModal = closeMonitorModal;
  window.submitMonitorForm = submitMonitorForm;

  connect();
})();
