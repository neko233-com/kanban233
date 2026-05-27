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
