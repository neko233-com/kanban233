(async function initDevReload() {
  try {
    const cfg = await fetch('/api/config').then((r) => r.json());
    if (!cfg.dev) return;

    const es = new EventSource('/api/dev/reload');
    es.onmessage = (e) => {
      if (e.data === 'reload') {
        window.location.reload();
      }
    };
    es.onerror = () => {
      es.close();
      setTimeout(initDevReload, 3000);
    };
  } catch {
    // dev reload optional
  }
})();
