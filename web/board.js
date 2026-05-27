const state = {
  groups: [],
  explore: [],
  currentGroup: null,
  boards: [],
  currentBoard: null,
  editingCard: null,
  editingGroup: null,
  draggingCardId: null,
  activeMainView: 'research',
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
  loadResearchOverview();
}

function showProjectsView() {
  $('#research-view').classList.add('hidden');
  $('#projects-pane').classList.remove('hidden');
  $('#projects-sidebar').classList.remove('hidden');
}

async function loadResearchOverview() {
  const data = await api('/api/research/overview');
  const s = data.summary;
  $('#research-summary').textContent =
    `进行中 ${s.active_cards} · 已完结 ${s.completed_cards} · ${s.project_groups} 个项目组 · ${s.boards} 个看板`;
  const box = $('#research-content');
  box.innerHTML = '';
  data.groups.forEach((gv) => {
    const section = document.createElement('section');
    section.className = 'research-group';
    const joinLabel = gv.group.join_mode === 'apply' ? '申请加入' : '自由加入';
    section.innerHTML = `<h3>${escapeHtml(gv.group.name)} <span class="muted">${gv.group.is_public ? '公开' : '私有'} · ${joinLabel}</span></h3>`;
    gv.boards.forEach((bv) => {
      const boardBlock = document.createElement('div');
      boardBlock.className = 'research-board';
      boardBlock.innerHTML = `<h4>${escapeHtml(bv.board.title)} <span class="muted">(${bv.active_cards.length} 进行中)</span></h4>`;
      const list = document.createElement('ul');
      list.className = 'research-cards';
      bv.active_cards.forEach((c) => {
        const li = document.createElement('li');
        li.textContent = c.title;
        li.addEventListener('click', async () => {
          document.querySelector('[data-view="projects"]').click();
          const g = state.groups.find((x) => x.id === gv.group.id) || gv.group;
          state.currentGroup = g;
          await selectGroup(gv.group.id);
          await openBoard(bv.board.id);
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
  state.groups.forEach((g) => {
    const li = document.createElement('li');
    li.className = 'group-item' + (state.currentGroup?.id === g.id ? ' active' : '');
    const badge = g.is_public ? '<span class="badge public">公开</span>' : '<span class="badge private">私有</span>';
    const owned = g.is_owner ? '' : ' <span class="muted">(成员)</span>';
    li.innerHTML = `<strong>${escapeHtml(g.name)}</strong>${badge}${owned}`;
    li.addEventListener('click', () => selectGroup(g.id));
    list.appendChild(li);
  });
}

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
        alert(res.join_pending ? '已提交加入申请' : '加入成功');
        await loadGroups();
        if (!res.join_pending) await selectGroup(g.id);
      } catch (err) {
        alert(err.message || '操作失败');
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
  state.currentGroup = state.groups.find((g) => g.id === groupId);
  state.boards = await api(`/api/groups/${groupId}/boards`);
  renderGroups();
  renderGroupView();
  $('#group-view').classList.remove('hidden');
  $('#board-view').classList.add('hidden');
  showProjectsView();
}

function renderGroupView() {
  const g = state.currentGroup;
  if (!g) return;
  $('#group-title').textContent = g.name;
  const joinLabel = g.join_mode === 'apply' ? '申请加入' : '自由加入';
  $('#group-meta').textContent = `${g.description || '暂无描述'} · ${g.is_public ? '公开' : '私有'} · ${joinLabel}`;
  const isOwner = g.is_owner || g.owner_id === Auth.user.id;
  $('#edit-group-btn').classList.toggle('hidden', !isOwner);
  $('#applications-btn').classList.toggle('hidden', !isOwner);
  $('#group-history-btn').classList.toggle('hidden', false);
  $('#new-board-btn').classList.toggle('hidden', !g.is_member && !isOwner);

  const list = $('#board-list');
  list.innerHTML = '';
  state.boards.forEach((b) => {
    const li = document.createElement('li');
    li.innerHTML = `<strong>${escapeHtml(b.title)}</strong><br><span class="muted">更新 ${formatTime(b.updated_at)}</span>`;
    li.addEventListener('click', () => openBoard(b.id));
    list.appendChild(li);
  });
}

$('#new-group-btn').addEventListener('click', () => openGroupDialog(null));
$('#edit-group-btn').addEventListener('click', () => openGroupDialog(state.currentGroup));
$('#new-board-btn').addEventListener('click', async () => {
  if (!state.currentGroup) return;
  const title = prompt('看板名称');
  if (!title) return;
  await api(`/api/groups/${state.currentGroup.id}/boards`, { method: 'POST', body: JSON.stringify({ title }) });
  await selectGroup(state.currentGroup.id);
});

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
  } else {
    await api('/api/groups', { method: 'POST', body: JSON.stringify(payload) });
  }
  groupDialog.close();
  await loadGroups();
  if (state.currentGroup) await selectGroup(state.currentGroup.id);
});

$('#delete-group-btn').addEventListener('click', async () => {
  if (!state.editingGroup || !confirm('确定删除该项目组及所有看板？')) return;
  await api(`/api/groups/${state.editingGroup.id}`, { method: 'DELETE' });
  groupDialog.close();
  state.currentGroup = null;
  await loadGroups();
});

async function openBoard(id) {
  state.currentBoard = await api(`/api/boards/${id}`);
  renderBoard();
  $('#group-view').classList.add('hidden');
  $('#board-view').classList.remove('hidden');
}

$('#back-btn').addEventListener('click', async () => {
  if (state.currentGroup) await selectGroup(state.currentGroup.id);
});

$('#add-column-btn').addEventListener('click', async () => {
  const title = prompt('列名称');
  if (!title) return;
  await api(`/api/boards/${state.currentBoard.board.id}/columns`, { method: 'POST', body: JSON.stringify({ title }) });
  await openBoard(state.currentBoard.board.id);
});

async function showHistory(url, title) {
  const items = await api(url);
  $('#history-title').textContent = title;
  $('#history-list').innerHTML = items.map((h) => `
    <div class="history-row">
      <strong>${escapeHtml(h.card.title)}</strong>
      <span class="muted">${escapeHtml(h.group_name)} / ${escapeHtml(h.board_title)}</span>
      <span class="muted">完结 ${formatTime(h.card.completed_at || h.card.updated_at)}</span>
      <p>${escapeHtml(h.card.description || '')}</p>
    </div>`).join('') || '<p class="muted">暂无完结任务</p>';
  $('#history-dialog').showModal();
}

$('#board-history-btn').addEventListener('click', () => {
  showHistory(`/api/boards/${state.currentBoard.board.id}/history`, '看板完结历史');
});
$('#group-history-btn').addEventListener('click', () => {
  if (state.currentGroup) showHistory(`/api/groups/${state.currentGroup.id}/history`, '项目组完结历史');
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
    alert(result.message || `导入成功：${result.action}`);
    if (result.detail) await openBoard(result.board_id);
    else if (state.currentGroup) await selectGroup(state.currentGroup.id);
  } catch (err) {
    alert(err.status === 409 ? (err.data?.message || '导入版本较旧') : (err.message || '导入失败'));
  }
  e.target.value = '';
});

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

function cardsForColumn(columnId) {
  return state.currentBoard.cards.filter((c) => c.column_id === columnId).sort((a, b) => a.position - b.position);
}

function renderBoard() {
  const { board, columns } = state.currentBoard;
  $('#board-title').textContent = board.title;
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
    await api(`/api/cards/${cardId}/complete`, { method: 'POST', body: '{}' });
    await openBoard(state.currentBoard.board.id);
  });
}

function renderCard(card) {
  const el = document.createElement('div');
  el.className = 'card';
  el.draggable = true;
  el.innerHTML = `<strong>${escapeHtml(card.title)}</strong>${card.description ? `<p>${escapeHtml(card.description)}</p>` : ''}`;
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
  await openBoard(state.currentBoard.board.id);
}

const cardDialog = $('#card-dialog');
const cardForm = $('#card-form');

function openCardDialog(card) {
  state.editingCard = card;
  $('#card-dialog-title').textContent = card.id ? '编辑卡片' : '新建卡片';
  cardForm.title.value = card.title || '';
  cardForm.description.value = card.description || '';
  $('#delete-card-btn').classList.toggle('hidden', !card.id);
  $('#complete-card-btn').classList.toggle('hidden', !card.id);
  cardDialog.showModal();
}

$('#complete-card-btn').addEventListener('click', async () => {
  if (!state.editingCard?.id) return;
  await api(`/api/cards/${state.editingCard.id}/complete`, { method: 'POST', body: '{}' });
  cardDialog.close();
  await openBoard(state.currentBoard.board.id);
});

cardForm.addEventListener('submit', async (e) => {
  e.preventDefault();
  const fd = new FormData(cardForm);
  const payload = { title: fd.get('title'), description: fd.get('description') || '' };
  if (state.editingCard.id) {
    await api(`/api/cards/${state.editingCard.id}`, { method: 'PUT', body: JSON.stringify(payload) });
  } else {
    await api(`/api/columns/${state.editingCard.column_id}/cards`, { method: 'POST', body: JSON.stringify(payload) });
  }
  cardDialog.close();
  await openBoard(state.currentBoard.board.id);
});

$('#delete-card-btn').addEventListener('click', async () => {
  if (!state.editingCard?.id) return;
  await api(`/api/cards/${state.editingCard.id}`, { method: 'DELETE' });
  cardDialog.close();
  await openBoard(state.currentBoard.board.id);
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
