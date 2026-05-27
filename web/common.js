const Auth = {
  get token() { return localStorage.getItem('kanban_token') || ''; },
  get user() {
    try { return JSON.parse(localStorage.getItem('kanban_user') || 'null'); }
    catch { return null; }
  },
  set(token, user) {
    localStorage.setItem('kanban_token', token);
    localStorage.setItem('kanban_user', JSON.stringify(user));
  },
  clear() {
    localStorage.removeItem('kanban_token');
    localStorage.removeItem('kanban_user');
  },
  isLoggedIn() { return !!this.token; },
  requireLogin() {
    if (!this.isLoggedIn()) {
      window.location.href = '/login.html';
      return false;
    }
    return true;
  },
};

async function api(path, options = {}) {
  const headers = { ...(options.headers || {}) };
  if (!(options.body instanceof FormData)) {
    headers['Content-Type'] = 'application/json';
  }
  if (Auth.token) headers.Authorization = `Bearer ${Auth.token}`;
  const res = await fetch(path, { ...options, headers });
  const text = await res.text();
  let data = null;
  if (text) {
    try { data = JSON.parse(text); } catch { data = { error: text }; }
  }
  if (!res.ok) {
    const err = new Error(data?.error || data?.message || res.statusText);
    err.status = res.status;
    err.data = data;
    throw err;
  }
  return data;
}

function escapeHtml(str) {
  return String(str)
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;');
}

function downloadJSON(filename, obj) {
  const blob = new Blob([JSON.stringify(obj, null, 2)], { type: 'application/json' });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  a.click();
  URL.revokeObjectURL(url);
}

function formatTime(iso) {
  if (!iso) return '';
  return new Date(iso).toLocaleString('zh-CN');
}

const $ = (sel) => document.querySelector(sel);

function ensureUiDialogs() {
  if ($('#ui-prompt-dialog')) return;

  document.body.insertAdjacentHTML('beforeend', `
<dialog id="ui-prompt-dialog">
  <form method="dialog" id="ui-prompt-form">
    <h3 id="ui-prompt-title"></h3>
    <label id="ui-prompt-label">名称<input name="value" required></label>
    <div class="dialog-actions">
      <button type="button" class="btn ghost" id="ui-prompt-cancel">取消</button>
      <button type="submit" class="btn primary">确定</button>
    </div>
  </form>
</dialog>
<dialog id="ui-confirm-dialog">
  <form method="dialog" id="ui-confirm-form">
    <h3 id="ui-confirm-title">确认</h3>
    <p id="ui-confirm-message"></p>
    <div class="dialog-actions">
      <button type="button" class="btn ghost" id="ui-confirm-cancel">取消</button>
      <button type="submit" class="btn danger" id="ui-confirm-ok">确定</button>
    </div>
  </form>
</dialog>
<div id="ui-toast-host" class="toast-host" aria-live="polite"></div>`);
}

function uiPrompt({ title = '输入', label = '名称', defaultValue = '' } = {}) {
  ensureUiDialogs();
  const dlg = $('#ui-prompt-dialog');
  const form = $('#ui-prompt-form');
  $('#ui-prompt-title').textContent = title;
  $('#ui-prompt-label').childNodes[0].textContent = label;
  form.value.value = defaultValue;
  return new Promise((resolve) => {
    const done = (value) => {
      dlg.close();
      form.removeEventListener('submit', onSubmit);
      $('#ui-prompt-cancel').removeEventListener('click', onCancel);
      dlg.removeEventListener('cancel', onCancel);
      resolve(value);
    };
    const onSubmit = (e) => {
      e.preventDefault();
      done(form.value.value.trim());
    };
    const onCancel = () => done(null);
    form.addEventListener('submit', onSubmit);
    $('#ui-prompt-cancel').addEventListener('click', onCancel);
    dlg.addEventListener('cancel', onCancel);
    dlg.showModal();
    form.value.focus();
    form.value.select();
  });
}

function uiConfirm(message, { title = '确认', confirmText = '确定', danger = true } = {}) {
  ensureUiDialogs();
  const dlg = $('#ui-confirm-dialog');
  const form = $('#ui-confirm-form');
  $('#ui-confirm-title').textContent = title;
  $('#ui-confirm-message').textContent = message;
  const okBtn = $('#ui-confirm-ok');
  okBtn.textContent = confirmText;
  okBtn.classList.toggle('danger', danger);
  okBtn.classList.toggle('primary', !danger);
  return new Promise((resolve) => {
    const done = (value) => {
      dlg.close();
      form.removeEventListener('submit', onSubmit);
      $('#ui-confirm-cancel').removeEventListener('click', onCancel);
      dlg.removeEventListener('cancel', onCancel);
      resolve(value);
    };
    const onSubmit = (e) => {
      e.preventDefault();
      done(true);
    };
    const onCancel = () => done(false);
    form.addEventListener('submit', onSubmit);
    $('#ui-confirm-cancel').addEventListener('click', onCancel);
    dlg.addEventListener('cancel', onCancel);
    dlg.showModal();
  });
}

function uiToast(message, { type = 'info', duration = 3200 } = {}) {
  ensureUiDialogs();
  const host = $('#ui-toast-host');
  const el = document.createElement('div');
  el.className = `toast toast-${type}`;
  el.textContent = message;
  host.appendChild(el);
  requestAnimationFrame(() => el.classList.add('show'));
  setTimeout(() => {
    el.classList.remove('show');
    setTimeout(() => el.remove(), 200);
  }, duration);
}
