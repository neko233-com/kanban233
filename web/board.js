const state = {
  groups: [],
  explore: [],
  currentGroup: null,
  currentBoard: null,
  editingCard: null,
  editingGroup: null,
  draggingCardId: null,
  activeMainView: 'research',
  groupSearch: '',
  charts: {},
};

const STATUS_LABELS = {
  active: '进行中',
  archived: '已归档',
  completed: '已完结',
};

if (!Auth.requireLogin()) throw new Error('redirect');

$('#username').textContent = Auth.user?.username || '';

$('#logout-btn').addEventListener('click', () => {
  Auth.clear();
  window.location.href = '/login.html';
});

document.querySelectorAll('.nav-btn').forEach((btn) => {
  btn.addEventListener('click', () => {
    document.querySelectorAll('.nav-btn').forEach((b) => b.classList.remove('active'));
    btn.classList.add('active');
    state.activeMainView = btn.dataset.view;
    if (state.activeMainView === 'research') {
      showResearchView();
    } else {
      showProjectsView();
    }
  });
});

function showResearchView() {
  $('#research-view').classList.remove('hidden');
  $('#projects-pane').classList.add('hidden');
  $('#projects-sidebar').classList.add('hidden');
  $('.workspace').classList.add('research-mode');
  loadResearchOverview();
}

function showProjectsView() {
  $('#research-view').classList.add('hidden');
  $('#projects-pane').classList.remove('hidden');
  $('#projects-sidebar').classList.remove('hidden');
  $('.workspace').classList.remove('research-mode');
  if (state.currentGroup && state.currentBoard) {
    $('#project-empty').classList.add('hidden');
    $('#board-view').classList.remove('hidden');
  } else {
    showProjectEmpty();
  }
}

function showProjectEmpty() {
  $('#project-empty').classList.remove('hidden');
  $('#board-view').classList.add('hidden');
}

function fuzzyMatch(text, query) {
  const src = String(text || '').toLowerCase();
  const q = String(query || '').trim().toLowerCase();
  if (!q) return true;
  if (src.includes(q)) return true;
  let i = 0;
  for (const ch of q) {
    i = src.indexOf(ch, i);
    if (i === -1) return false;
    i += 1;
  }
  return true;
}

async function loadResearchOverview() {
  const data = await api('/api/research/overview');
  const s = data.summary;
  $('#research-summary').innerHTML = `
    <span class="stat-pill"><span class="stat-value">${s.active_cards}</span><span class="stat-label">进行中</span></span>
    <span class="stat-pill"><span class="stat-value">${s.completed_cards}</span><span class="stat-label">已归档/完结</span></span>
    <span class="stat-pill"><span class="stat-value">${s.project_groups}</span><span class="stat-label">项目组</span></span>
    <span class="stat-pill"><span class="stat-value">${s.boards}</span><span class="stat-label">看板</span></span>`;
  loadResearchStats();
  const box = $('#research-content');
  box.innerHTML = '';
  data.groups.forEach((gv) => {
    const section = document.createElement('section');
    section.className = 'research-group';
    const joinLabel = gv.group.join_mode === 'apply' ? '申请加入' : '自由加入';
    const visBadge = gv.group.is_public
      ? '<span class="badge public">公开</span>'
      : '<span class="badge private">私有</span>';
    section.innerHTML = `<div class="research-group-head">
      <h3>${escapeHtml(gv.group.name)}</h3>
      <div class="badge-row">${visBadge}<span class="badge neutral">${joinLabel}</span></div>
    </div>`;
    gv.boards.forEach((bv) => {
      const boardBlock = document.createElement('div');
      boardBlock.className = 'research-board';
      const cardCount = bv.active_cards.length;
      boardBlock.innerHTML = `<h4>${escapeHtml(gv.group.name)} <span class="muted">(${cardCount} 进行中)</span></h4>`;
      const list = document.createElement('ul');
      list.className = 'research-cards';
      bv.active_cards.forEach((c) => {
        const li = document.createElement('li');
        li.textContent = c.title;
        li.addEventListener('click', async () => {
          document.querySelector('[data-view="projects"]').click();
          await selectGroup(gv.group.id);
        });
        list.appendChild(li);
      });
      if (!bv.active_cards.length) {
        list.innerHTML = '<li class="muted">暂无进行中任务</li>';
      }
      boardBlock.appendChild(list);
      section.appendChild(boardBlock);
    });
    if (!gv.boards.length) {
      section.innerHTML += '<p class="muted">暂无看板</p>';
    }
    box.appendChild(section);
  });
}

async function loadGroups() {
  state.groups = await api('/api/groups');
  state.explore = await api('/api/groups/explore');
  renderGroups();
  renderExplore();
}

function renderGroups() {
  const list = $('#group-list');
  list.innerHTML = '';
  const q = state.groupSearch.trim();
  const filtered = state.groups.filter((g) => fuzzyMatch(`${g.name} ${g.description || ''}`, q));
  if (!filtered.length) {
    list.innerHTML = `<li class="muted sidebar-empty">${q ? '无匹配项目组' : '暂无项目组'}</li>`;
    return;
  }
  filtered.forEach((g) => {
    const li = document.createElement('li');
    li.className = 'group-item' + (state.currentGroup?.id === g.id ? ' active' : '');
    const badge = g.is_public ? '<span class="badge public">公开</span>' : '<span class="badge private">私有</span>';
    const owned = g.is_owner ? '' : ' <span class="muted">(成员)</span>';
    const desc = g.description ? `<span class="group-item-desc muted">${escapeHtml(g.description)}</span>` : '';
    li.innerHTML = `<strong>${escapeHtml(g.name)}</strong>${badge}${owned}${desc}`;
    li.addEventListener('click', () => selectGroup(g.id));
    list.appendChild(li);
  });
}

$('#group-search').addEventListener('input', (e) => {
  state.groupSearch = e.target.value;
  renderGroups();
});

function renderExplore() {
  const list = $('#explore-list');
  list.innerHTML = '';
  state.explore.forEach((g) => {
    const li = document.createElement('li');
    li.className = 'group-item explore-item';
    const joinText = g.join_mode === 'apply' ? '申请' : '加入';
    li.innerHTML = `<strong>${escapeHtml(g.name)}</strong><br><span class="muted">${escapeHtml(g.description || '')}</span>`;
    const btn = document.createElement('button');
    btn.className = 'btn small';
    btn.textContent = g.join_pending ? '待审批' : joinText;
    btn.disabled = !!g.join_pending;
    btn.addEventListener('click', async (e) => {
      e.stopPropagation();
      try {
        const res = await api(`/api/groups/${g.id}/join`, { method: 'POST', body: '{}' });
        uiToast(res.join_pending ? '已提交加入申请' : '加入成功', { type: 'success' });
        await loadGroups();
        if (!res.join_pending) await selectGroup(g.id);
      } catch (err) {
        uiToast(err.message || '操作失败', { type: 'error' });
      }
    });
    li.appendChild(btn);
    list.appendChild(li);
  });
  if (!state.explore.length) {
    list.innerHTML = '<li class="muted" style="padding:0.5rem">暂无可加入项目</li>';
  }
}

async function selectGroup(groupId) {
  state.currentGroup = state.groups.find((g) => g.id === groupId) || null;
  if (!state.currentGroup) {
    await loadGroups();
    state.currentGroup = state.groups.find((g) => g.id === groupId) || null;
  }
  renderGroups();
  showProjectsView();
  try {
    state.currentBoard = await api(`/api/groups/${groupId}/kanban`);
    updateGroupToolbar();
    renderBoard();
    await loadGroupStats();
    $('#project-empty').classList.add('hidden');
    $('#board-view').classList.remove('hidden');
  } catch (err) {
    uiToast(err.message || '无法打开看板', { type: 'error' });
    showProjectEmpty();
  }
}

async function refreshKanban() {
  if (!state.currentGroup) return;
  state.currentBoard = await api(`/api/groups/${state.currentGroup.id}/kanban`);
  renderBoard();
  await loadGroupStats();
}

function cardPayloadFromForm(fd) {
  const start = String(fd.get('start_date') || '').trim();
  const end = String(fd.get('end_date') || '').trim();
  return {
    title: fd.get('title'),
    description: fd.get('description') || '',
    workers: String(fd.get('workers') || '').trim(),
    start_date: start || null,
    end_date: end || null,
  };
}

function formatCardDateRange(card) {
  if (!card.start_date && !card.end_date) return '';
  const s = card.start_date || '…';
  const e = card.end_date || '…';
  return `${s} → ${e}`;
}

function renderCardMeta(card) {
  const parts = [];
  if (card.workers) {
    parts.push(`<span class="card-tag worker">${escapeHtml(card.workers)}</span>`);
  }
  const range = formatCardDateRange(card);
  if (range) parts.push(`<span class="card-tag date">${escapeHtml(range)}</span>`);
  return parts.length ? `<div class="card-meta">${parts.join('')}</div>` : '';
}

async function loadGroupStats() {
  if (!state.currentGroup) return;
  try {
    const stats = await api(`/api/groups/${state.currentGroup.id}/stats`);
    renderWorkerChart('group-chart-workers', stats.by_worker, '工作人员任务分布');
    renderColumnChart('group-chart-column', stats.by_column);
    renderStatusChart('group-chart-status', stats.by_status);
  } catch { /* ignore */ }
}

async function loadResearchStats() {
  try {
    const stats = await api('/api/research/stats');
    renderWorkerChart('research-chart-workers', stats.by_worker, '全项目工作人员');
    renderStatusChart('research-chart-status', stats.by_status);
  } catch { /* ignore */ }
}

function disposeChart(id) {
  if (state.charts[id]) {
    state.charts[id].dispose();
    delete state.charts[id];
  }
}

function ensureChart(id) {
  const el = document.getElementById(id);
  if (!el || typeof echarts === 'undefined') return null;
  disposeChart(id);
  state.charts[id] = echarts.init(el, null, { renderer: 'canvas' });
  return state.charts[id];
}

const chartBase = {
  backgroundColor: 'transparent',
  textStyle: { color: '#98989d', fontFamily: 'system-ui, sans-serif' },
};

function renderWorkerChart(id, rows, title) {
  const chart = ensureChart(id);
  if (!chart) return;
  const data = (rows || []).slice(0, 12).map((w) => ({
    name: w.name,
    value: w.total,
    active: w.active,
    finished: w.finished,
  }));
  chart.setOption({
    ...chartBase,
    title: { text: title || '工作人员', left: 8, top: 4, textStyle: { color: '#ebebf5', fontSize: 13, fontWeight: 600 } },
    tooltip: {
      trigger: 'item',
      formatter: (p) => `${p.name}<br/>总计 ${p.value}（进行中 ${p.data.active} / 已结束 ${p.data.finished}）`,
    },
    series: [{
      type: 'pie',
      radius: ['42%', '68%'],
      center: ['50%', '58%'],
      itemStyle: { borderRadius: 6, borderColor: '#1c1c1e', borderWidth: 2 },
      label: { color: '#98989d', fontSize: 11 },
      data: data.length ? data : [{ name: '暂无数据', value: 1, itemStyle: { color: '#3a3a3c' } }],
    }],
  });
}

function renderColumnChart(id, rows) {
  const chart = ensureChart(id);
  if (!chart) return;
  const labels = (rows || []).map((r) => r.column);
  const values = (rows || []).map((r) => r.count);
  chart.setOption({
    ...chartBase,
    title: { text: '列分布（进行中）', left: 8, top: 4, textStyle: { color: '#ebebf5', fontSize: 13, fontWeight: 600 } },
    grid: { left: 48, right: 16, top: 40, bottom: 28 },
    xAxis: { type: 'category', data: labels.length ? labels : ['—'], axisLabel: { color: '#98989d', fontSize: 11 } },
    yAxis: { type: 'value', minInterval: 1, splitLine: { lineStyle: { color: '#2c2c2e' } }, axisLabel: { color: '#98989d' } },
    series: [{ type: 'bar', data: values.length ? values : [0], itemStyle: { color: '#0a84ff', borderRadius: [4, 4, 0, 0] }, barMaxWidth: 36 }],
    tooltip: { trigger: 'axis' },
  });
}

function renderStatusChart(id, rows) {
  const chart = ensureChart(id);
  if (!chart) return;
  const data = (rows || []).filter((r) => r.count > 0).map((r) => ({
    name: STATUS_LABELS[r.status] || r.status,
    value: r.count,
  }));
  const colors = { 进行中: '#0a84ff', 已归档: '#ff9f0a', 已完结: '#30d158' };
  chart.setOption({
    ...chartBase,
    title: { text: '任务状态', left: 8, top: 4, textStyle: { color: '#ebebf5', fontSize: 13, fontWeight: 600 } },
    tooltip: { trigger: 'item' },
    series: [{
      type: 'pie',
      radius: '58%',
      center: ['50%', '58%'],
      data: data.length ? data.map((d) => ({ ...d, itemStyle: { color: colors[d.name] } })) : [{ name: '暂无', value: 1, itemStyle: { color: '#3a3a3c' } }],
      label: { color: '#98989d' },
    }],
  });
}

window.addEventListener('resize', () => {
  Object.values(state.charts).forEach((c) => c.resize());
});

function updateGroupToolbar() {
  const g = state.currentGroup;
  if (!g) return;
  $('#board-title').textContent = g.name;
  const joinLabel = g.join_mode === 'apply' ? '申请加入' : '自由加入';
  $('#group-meta').textContent = `${g.description || '暂无描述'} · ${g.is_public ? '公开' : '私有'} · ${joinLabel}`;
  const isOwner = g.is_owner || g.owner_id === Auth.user.id;
  $('#edit-group-btn').classList.toggle('hidden', !isOwner);
  $('#applications-btn').classList.toggle('hidden', !isOwner);
}

$('#new-group-btn').addEventListener('click', () => openGroupDialog(null));
$('#edit-group-btn').addEventListener('click', () => openGroupDialog(state.currentGroup));

const groupDialog = $('#group-dialog');
const groupForm = $('#group-form');

function openGroupDialog(group) {
  state.editingGroup = group;
  $('#group-dialog-title').textContent = group ? '编辑项目组' : '新建项目组';
  groupForm.name.value = group?.name || '';
  groupForm.description.value = group?.description || '';
  groupForm.is_public.checked = !!group?.is_public;
  groupForm.join_mode.value = group?.join_mode || 'free';
  $('#delete-group-btn').classList.toggle('hidden', !group);
  groupDialog.showModal();
}

groupForm.addEventListener('submit', async (e) => {
  e.preventDefault();
  const fd = new FormData(groupForm);
  const payload = {
    name: fd.get('name'),
    description: fd.get('description') || '',
    is_public: fd.get('is_public') === 'on',
    join_mode: fd.get('join_mode') || 'free',
  };
  if (state.editingGroup) {
    await api(`/api/groups/${state.editingGroup.id}`, { method: 'PUT', body: JSON.stringify(payload) });
    groupDialog.close();
    await loadGroups();
    if (state.currentGroup?.id === state.editingGroup.id) await selectGroup(state.editingGroup.id);
    return;
  }
  const created = await api('/api/groups', { method: 'POST', body: JSON.stringify(payload) });
  groupDialog.close();
  await loadGroups();
  document.querySelector('[data-view="projects"]').click();
  await selectGroup(created.id);
});

$('#delete-group-btn').addEventListener('click', async () => {
  if (!state.editingGroup || !await uiConfirm('确定删除该项目组及所有看板？')) return;
  await api(`/api/groups/${state.editingGroup.id}`, { method: 'DELETE' });
  groupDialog.close();
  state.currentGroup = null;
  state.currentBoard = null;
  await loadGroups();
  showProjectEmpty();
});

$('#add-column-btn').addEventListener('click', async () => {
  const title = await uiPrompt({ title: '添加列', label: '列名称' });
  if (!title) return;
  await api(`/api/boards/${state.currentBoard.board.id}/columns`, { method: 'POST', body: JSON.stringify({ title }) });
  await refreshKanban();
});

async function showHistory(url, title) {
  const items = await api(url);
  $('#history-title').textContent = title;
  $('#history-list').innerHTML = items.map((h) => {
    const status = STATUS_LABELS[h.card.status] || h.card.status;
    const workers = h.card.workers ? `<span class="muted"> · ${escapeHtml(h.card.workers)}</span>` : '';
    const dates = formatCardDateRange(h.card);
    const dateLine = dates ? `<span class="muted"> · ${escapeHtml(dates)}</span>` : '';
    return `
    <div class="history-row">
      <strong>${escapeHtml(h.card.title)}</strong>
      <span class="badge neutral">${escapeHtml(status)}</span>
      <span class="muted">${escapeHtml(h.group_name)} / ${escapeHtml(h.board_title)}</span>
      <span class="muted">${workers}${dateLine}</span>
      <span class="muted">归档 ${formatTime(h.card.completed_at || h.card.updated_at)}</span>
      <p>${escapeHtml(h.card.description || '')}</p>
    </div>`;
  }).join('') || '<p class="muted">暂无归档任务</p>';
  $('#history-dialog').showModal();
}

$('#board-history-btn').addEventListener('click', () => {
  showHistory(`/api/boards/${state.currentBoard.board.id}/history`, '看板归档历史');
});
$('#group-history-btn').addEventListener('click', () => {
  if (state.currentGroup) showHistory(`/api/groups/${state.currentGroup.id}/history`, '项目组归档历史');
});

$('#applications-btn').addEventListener('click', async () => {
  const apps = await api(`/api/groups/${state.currentGroup.id}/applications`);
  $('#applications-list').innerHTML = apps.map((a) => `
    <div class="app-row">
      <span>${escapeHtml(a.username)}</span>
      <span class="muted">${formatTime(a.joined_at)}</span>
      <button class="btn small" data-uid="${a.user_id}" data-act="approve">批准</button>
      <button class="btn small" data-uid="${a.user_id}" data-act="reject">拒绝</button>
    </div>`).join('') || '<p class="muted">暂无待审批申请</p>';
  $('#applications-list').onclick = async (e) => {
    const btn = e.target.closest('button[data-uid]');
    if (!btn) return;
    const uid = btn.dataset.uid;
    const act = btn.dataset.act;
    await api(`/api/groups/${state.currentGroup.id}/applications/${uid}/${act}`, { method: 'POST', body: '{}' });
    $('#applications-dialog').close();
    await loadGroups();
  };
  $('#applications-dialog').showModal();
});

$('#export-board-btn').addEventListener('click', async () => {
  const id = state.currentBoard.board.id;
  const res = await fetch(`/api/boards/${id}/export`, { headers: { Authorization: `Bearer ${Auth.token}` } });
  const blob = await res.blob();
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = `kanban-board-${state.currentBoard.board.export_key}.json`;
  a.click();
  URL.revokeObjectURL(url);
});

$('#export-all-btn').addEventListener('click', async () => {
  const res = await fetch('/api/export/all', { headers: { Authorization: `Bearer ${Auth.token}` } });
  const blob = await res.blob();
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = `kanban-all-${Auth.user.username}.json`;
  a.click();
  URL.revokeObjectURL(url);
});

$('#import-board-input').addEventListener('change', async (e) => {
  const file = e.target.files[0];
  if (!file) return;
  try {
    const payload = JSON.parse(await file.text());
    const result = await api('/api/boards/import', { method: 'POST', body: JSON.stringify(payload) });
    uiToast(result.message || `导入成功：${result.action}`, { type: 'success' });
    if (result.detail) {
      await loadGroups();
      if (result.detail.board?.project_group_id) await selectGroup(result.detail.board.project_group_id);
    } else if (state.currentGroup) await refreshKanban();
  } catch (err) {
    uiToast(err.status === 409 ? (err.data?.message || '导入版本较旧') : (err.message || '导入失败'), { type: 'error' });
  }
  e.target.value = '';
});

async function downloadAuditExport(format) {
  const res = await fetch(`/api/audit-logs/export?format=${format}`, {
    headers: { Authorization: `Bearer ${Auth.token}` },
  });
  if (!res.ok) {
    const text = await res.text();
    let msg = res.statusText;
    try { msg = JSON.parse(text).error || msg; } catch { /* ignore */ }
    throw new Error(msg);
  }
  const blob = await res.blob();
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  const stamp = new Date().toISOString().slice(0, 19).replace(/[-:T]/g, '');
  a.href = url;
  a.download = `kanban-audit-${stamp}.${format}`;
  a.click();
  URL.revokeObjectURL(url);
}

$('#audit-btn').addEventListener('click', async () => {
  const logs = await api('/api/audit-logs?limit=100');
  $('#audit-list').innerHTML = logs.map((l) => `
    <div class="audit-row">
      <span class="audit-time">${formatTime(l.created_at)}</span>
      <span class="audit-user">${escapeHtml(l.username)}</span>
      <span class="audit-action">${escapeHtml(l.action)}</span>
      <span class="audit-res">${escapeHtml(l.resource_type)}#${l.resource_id}</span>
      <span class="audit-detail">${escapeHtml(l.detail || '')}</span>
    </div>`).join('') || '<p class="muted">暂无日志</p>';
  $('#audit-dialog').showModal();
});

$('#export-audit-json-btn').addEventListener('click', async () => {
  try {
    await downloadAuditExport('json');
    uiToast('审计日志已导出 JSON', { type: 'success' });
  } catch (err) {
    uiToast(err.message || '导出失败', { type: 'error' });
  }
});

$('#export-audit-csv-btn').addEventListener('click', async () => {
  try {
    await downloadAuditExport('csv');
    uiToast('审计日志已导出 CSV', { type: 'success' });
  } catch (err) {
    uiToast(err.message || '导出失败', { type: 'error' });
  }
});

function cardsForColumn(columnId) {
  return state.currentBoard.cards.filter((c) => c.column_id === columnId).sort((a, b) => a.position - b.position);
}

function renderBoard() {
  if (!state.currentBoard) return;
  const { columns } = state.currentBoard;
  updateGroupToolbar();
  const kanban = $('#kanban');
  kanban.innerHTML = '';
  columns.sort((a, b) => a.position - b.position).forEach((column) => {
    const colEl = document.createElement('div');
    colEl.className = 'column';
    colEl.innerHTML = `<div class="column-header">${escapeHtml(column.title)}</div>`;
    const body = document.createElement('div');
    body.className = 'column-body';
    cardsForColumn(column.id).forEach((card) => body.appendChild(renderCard(card)));
    const addBtn = document.createElement('button');
    addBtn.className = 'btn add-card-btn';
    addBtn.textContent = '+ 添加卡片';
    addBtn.addEventListener('click', () => openCardDialog({ column_id: column.id }));
    body.addEventListener('dragover', (e) => { e.preventDefault(); body.classList.add('drag-over'); });
    body.addEventListener('dragleave', () => body.classList.remove('drag-over'));
    body.addEventListener('drop', (e) => onDrop(e, column.id, body));
    colEl.appendChild(body);
    colEl.appendChild(addBtn);
    kanban.appendChild(colEl);
  });
  setupCompleteDrop();
}

function setupCompleteDrop() {
  const zone = $('#complete-drop');
  zone.addEventListener('dragover', (e) => { e.preventDefault(); zone.classList.add('drag-over'); });
  zone.addEventListener('dragleave', () => zone.classList.remove('drag-over'));
  zone.addEventListener('drop', async (e) => {
    e.preventDefault();
    zone.classList.remove('drag-over');
    const cardId = Number(e.dataTransfer.getData('text/plain') || state.draggingCardId);
    if (!cardId) return;
    await api(`/api/cards/${cardId}/archive`, { method: 'POST', body: '{}' });
    await refreshKanban();
  });
}

function renderCard(card) {
  const el = document.createElement('div');
  el.className = 'card';
  el.draggable = true;
  el.innerHTML = `<strong>${escapeHtml(card.title)}</strong>${card.description ? `<p>${escapeHtml(card.description)}</p>` : ''}${renderCardMeta(card)}`;
  el.addEventListener('dragstart', (e) => {
    e.dataTransfer.setData('text/plain', String(card.id));
    state.draggingCardId = card.id;
  });
  el.addEventListener('click', () => openCardDialog(card));
  return el;
}

async function onDrop(e, columnId, body) {
  e.preventDefault();
  body.classList.remove('drag-over');
  const cardId = Number(e.dataTransfer.getData('text/plain') || state.draggingCardId);
  if (!cardId) return;
  await api(`/api/cards/${cardId}/move`, {
    method: 'POST',
    body: JSON.stringify({ column_id: columnId, position: cardsForColumn(columnId).length }),
  });
  await refreshKanban();
}

const cardDialog = $('#card-dialog');
const cardForm = $('#card-form');

function openCardDialog(card) {
  state.editingCard = card;
  $('#card-dialog-title').textContent = card.id ? '编辑卡片' : '新建卡片';
  cardForm.title.value = card.title || '';
  cardForm.description.value = card.description || '';
  cardForm.workers.value = card.workers || '';
  cardForm.start_date.value = card.start_date || '';
  cardForm.end_date.value = card.end_date || '';
  $('#delete-card-btn').classList.toggle('hidden', !card.id);
  $('#archive-card-btn').classList.toggle('hidden', !card.id);
  cardDialog.showModal();
}

$('#archive-card-btn').addEventListener('click', async () => {
  if (!state.editingCard?.id) return;
  await api(`/api/cards/${state.editingCard.id}/archive`, { method: 'POST', body: '{}' });
  cardDialog.close();
  await refreshKanban();
});

cardForm.addEventListener('submit', async (e) => {
  e.preventDefault();
  const payload = cardPayloadFromForm(new FormData(cardForm));
  if (state.editingCard.id) {
    await api(`/api/cards/${state.editingCard.id}`, { method: 'PUT', body: JSON.stringify(payload) });
  } else {
    await api(`/api/columns/${state.editingCard.column_id}/cards`, { method: 'POST', body: JSON.stringify(payload) });
  }
  cardDialog.close();
  await refreshKanban();
});

$('#delete-card-btn').addEventListener('click', async () => {
  if (!state.editingCard?.id) return;
  await api(`/api/cards/${state.editingCard.id}`, { method: 'DELETE' });
  cardDialog.close();
  await refreshKanban();
});

(async function init() {
  try {
    const me = await api('/api/auth/me');
    Auth.set(Auth.token, me);
    $('#username').textContent = me.username;
    await loadGroups();
    showResearchView();
  } catch {
    Auth.clear();
    window.location.href = '/login.html';
  }
})();
