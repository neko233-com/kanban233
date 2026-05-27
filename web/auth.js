const authError = $('#auth-error');

function showError(msg) {
  authError.textContent = msg;
  authError.classList.remove('hidden');
}

async function loadConfig() {
  const cfg = await api('/api/config');
  if (!cfg.registration_open) {
    $('#register-tab').classList.add('hidden');
    $('#register-form').classList.add('hidden');
  }
}

document.querySelectorAll('.tab').forEach((tab) => {
  tab.addEventListener('click', () => {
    document.querySelectorAll('.tab').forEach((t) => t.classList.remove('active'));
    tab.classList.add('active');
    const name = tab.dataset.tab;
    $('#login-form').classList.toggle('hidden', name !== 'login');
    $('#register-form').classList.toggle('hidden', name !== 'register');
    authError.classList.add('hidden');
  });
});

$('#login-form').addEventListener('submit', async (e) => {
  e.preventDefault();
  const fd = new FormData(e.target);
  try {
    const data = await api('/api/auth/login', {
      method: 'POST',
      body: JSON.stringify({ username: fd.get('username'), password: fd.get('password') }),
    });
    Auth.set(data.token, data.user);
    window.location.href = '/board.html';
  } catch (err) {
    showError(err.message);
  }
});

$('#register-form').addEventListener('submit', async (e) => {
  e.preventDefault();
  const fd = new FormData(e.target);
  try {
    const data = await api('/api/auth/register', {
      method: 'POST',
      body: JSON.stringify({ username: fd.get('username'), password: fd.get('password') }),
    });
    Auth.set(data.token, data.user);
    window.location.href = '/board.html';
  } catch (err) {
    showError(err.message);
  }
});

(async function init() {
  if (Auth.isLoggedIn()) {
    window.location.href = '/board.html';
    return;
  }
  await loadConfig();
})();
