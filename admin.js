// ==========================================================================
// Dmitry Zhdanov — Cyber Portfolio — admin.js
// ==========================================================================

/* ---------- Matrix rain background (standalone copy for admin page) ---------- */
(function matrixRain() {
  const canvas = document.getElementById('matrix-bg');
  if (!canvas) return;
  const ctx = canvas.getContext('2d');
  let w, h, columns, drops;
  const chars = '01アイウエオカキクケコサシスセソタチツテト{}<>/#$&*'.split('');

  function resize() {
    w = canvas.width = window.innerWidth;
    h = canvas.height = window.innerHeight;
    columns = Math.floor(w / 18);
    drops = new Array(columns).fill(1);
  }
  window.addEventListener('resize', resize);
  resize();

  function draw() {
    ctx.fillStyle = 'rgba(6,10,16,0.08)';
    ctx.fillRect(0, 0, w, h);
    ctx.fillStyle = '#00ff9d';
    ctx.font = '14px monospace';
    for (let i = 0; i < drops.length; i++) {
      const text = chars[Math.floor(Math.random() * chars.length)];
      ctx.fillText(text, i * 18, drops[i] * 18);
      if (drops[i] * 18 > h && Math.random() > 0.975) drops[i] = 0;
      drops[i]++;
    }
  }
  setInterval(draw, 55);
})();

/* ---------- Auth helpers ---------- */
const TOKEN_KEY = 'admin_token';
function getToken() { return localStorage.getItem(TOKEN_KEY); }
function setToken(t) { localStorage.setItem(TOKEN_KEY, t); }
function clearToken() { localStorage.removeItem(TOKEN_KEY); }

function authHeaders() {
  const t = getToken();
  return t ? { 'Authorization': 'Bearer ' + t } : {};
}

function showToast(msg, isErr) {
  const t = document.getElementById('toast');
  t.textContent = msg;
  t.className = 'toast show' + (isErr ? ' err' : '');
  setTimeout(() => t.classList.remove('show'), 2800);
}

function escapeHtml(s) {
  return String(s || '').replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
}

const loginScreen = document.getElementById('loginScreen');
const adminScreen = document.getElementById('adminScreen');

async function checkAuth() {
  if (!getToken()) { showLogin(); return; }
  const res = await fetch('/api/admin/ping', { headers: authHeaders() });
  if (res.ok) { showAdmin(); } else { clearToken(); showLogin(); }
}

function showLogin() { loginScreen.style.display = 'block'; adminScreen.style.display = 'none'; }
function showAdmin() {
  loginScreen.style.display = 'none';
  adminScreen.style.display = 'block';
  loadProjectsTable();
  loadProfile();
  loadLeads();
}

document.getElementById('loginBtn').addEventListener('click', doLogin);
document.getElementById('passInput').addEventListener('keydown', (e) => { if (e.key === 'Enter') doLogin(); });

async function doLogin() {
  const pass = document.getElementById('passInput').value;
  const errEl = document.getElementById('loginError');
  errEl.textContent = '';
  try {
    const res = await fetch('/api/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ password: pass })
    });
    if (!res.ok) { errEl.textContent = 'Неверный пароль.'; return; }
    const data = await res.json();
    setToken(data.token);
    showAdmin();
  } catch (e) {
    errEl.textContent = 'Ошибка соединения с сервером.';
  }
}

document.getElementById('logoutBtn').addEventListener('click', () => {
  clearToken();
  showLogin();
});

/* ---------- Tabs ---------- */
document.querySelectorAll('.tab-btn').forEach(btn => {
  btn.addEventListener('click', () => {
    document.querySelectorAll('.tab-btn').forEach(b => b.classList.remove('active'));
    document.querySelectorAll('.panel').forEach(p => p.classList.remove('active'));
    btn.classList.add('active');
    document.getElementById('tab-' + btn.dataset.tab).classList.add('active');
  });
});

/* ---------- Projects table ---------- */
let projectsCache = [];

async function loadProjectsTable() {
  const res = await fetch('/api/projects');
  projectsCache = await res.json();
  renderProjTable(projectsCache);
}

function renderProjTable(list) {
  const body = document.getElementById('projTableBody');
  body.innerHTML = list.map(p => `
    <tr>
      <td>${p.id}</td>
      <td>${escapeHtml(p.title)}</td>
      <td>${escapeHtml(p.category)}</td>
      <td>${escapeHtml((p.tech || []).join(', '))}</td>
      <td>
        <div class="row-actions">
          <button data-id="${p.id}" class="edit-btn">Изменить</button>
          <button data-id="${p.id}" class="danger delete-btn">Удалить</button>
        </div>
      </td>
    </tr>
  `).join('');
  body.querySelectorAll('.edit-btn').forEach(b => b.addEventListener('click', () => openEditModal(parseInt(b.dataset.id))));
  body.querySelectorAll('.delete-btn').forEach(b => b.addEventListener('click', () => deleteProject(parseInt(b.dataset.id))));
}

document.getElementById('projSearch').addEventListener('input', (e) => {
  const q = e.target.value.toLowerCase();
  renderProjTable(projectsCache.filter(p =>
    `${p.title} ${p.category} ${(p.tech || []).join(' ')}`.toLowerCase().includes(q)
  ));
});

async function deleteProject(id) {
  if (!confirm('Удалить карточку #' + id + '?')) return;
  const res = await fetch('/api/projects/' + id, { method: 'DELETE', headers: authHeaders() });
  if (res.ok) { showToast('Карточка удалена'); loadProjectsTable(); }
  else showToast('Ошибка удаления', true);
}

/* ---------- Edit modal ---------- */
const editModal = document.getElementById('editModal');
document.getElementById('editModalClose').addEventListener('click', () => editModal.classList.remove('open'));
document.getElementById('newProjBtn').addEventListener('click', () => openEditModal(null));

let editingId = null;
function openEditModal(id) {
  editingId = id;
  const p = id ? projectsCache.find(x => x.id === id) : { title:'', category:'', niche:'', theme:'cyber-green', description:'', tech:[], features:[], demo_url:'' };
  document.getElementById('editModalTitle').textContent = id ? ('Редактирование #' + id) : 'Новая карточка';
  document.getElementById('f_title').value = p.title || '';
  document.getElementById('f_category').value = p.category || '';
  document.getElementById('f_niche').value = p.niche || '';
  document.getElementById('f_theme').value = p.theme || 'cyber-green';
  document.getElementById('f_description').value = p.description || '';
  document.getElementById('f_tech').value = (p.tech || []).join(', ');
  document.getElementById('f_features').value = (p.features || []).join(', ');
  document.getElementById('f_demo').value = p.demo_url || '';
  editModal.classList.add('open');
}

document.getElementById('saveProjBtn').addEventListener('click', async () => {
  const payload = {
    title: document.getElementById('f_title').value.trim(),
    category: document.getElementById('f_category').value.trim(),
    niche: document.getElementById('f_niche').value.trim(),
    theme: document.getElementById('f_theme').value,
    description: document.getElementById('f_description').value.trim(),
    tech: document.getElementById('f_tech').value.split(',').map(s => s.trim()).filter(Boolean),
    features: document.getElementById('f_features').value.split(',').map(s => s.trim()).filter(Boolean),
    demo_url: document.getElementById('f_demo').value.trim(),
    secure: true
  };
  if (!payload.title) { showToast('Укажите название', true); return; }

  const url = editingId ? ('/api/projects/' + editingId) : '/api/projects';
  const method = editingId ? 'PUT' : 'POST';
  const res = await fetch(url, {
    method, headers: { 'Content-Type': 'application/json', ...authHeaders() },
    body: JSON.stringify(payload)
  });
  if (res.ok) {
    showToast('Сохранено');
    editModal.classList.remove('open');
    loadProjectsTable();
  } else {
    showToast('Ошибка сохранения (проверьте авторизацию)', true);
  }
});

/* ---------- Profile ---------- */
async function loadProfile() {
  const res = await fetch('/api/profile');
  const p = await res.json();
  document.getElementById('p_name').value = p.name || '';
  document.getElementById('p_role').value = p.role || '';
  document.getElementById('p_email').value = p.email || '';
  document.getElementById('p_telegram').value = p.telegram || '';
  document.getElementById('p_lead').value = p.lead || '';
}

document.getElementById('saveProfileBtn').addEventListener('click', async () => {
  const payload = {
    name: document.getElementById('p_name').value.trim(),
    role: document.getElementById('p_role').value.trim(),
    email: document.getElementById('p_email').value.trim(),
    telegram: document.getElementById('p_telegram').value.trim(),
    lead: document.getElementById('p_lead').value.trim(),
  };
  const res = await fetch('/api/profile', {
    method: 'PUT', headers: { 'Content-Type': 'application/json', ...authHeaders() },
    body: JSON.stringify(payload)
  });
  if (res.ok) showToast('Профиль сохранён');
  else showToast('Ошибка сохранения (проверьте авторизацию)', true);
});

/* ---------- Leads ---------- */
async function loadLeads() {
  const res = await fetch('/api/leads', { headers: authHeaders() });
  if (!res.ok) return;
  const leads = await res.json();
  const body = document.getElementById('leadsTableBody');
  body.innerHTML = (leads || []).slice().reverse().map(l => `
    <tr>
      <td>${escapeHtml(l.created_at || '')}</td>
      <td>${escapeHtml(l.name || '')}</td>
      <td>${escapeHtml(l.contact || '')}</td>
      <td>${escapeHtml(l.type || '')}</td>
      <td>${escapeHtml(l.message || '')}</td>
    </tr>
  `).join('') || '<tr><td colspan="5">Заявок пока нет</td></tr>';
}

/* ---------- Password change ---------- */
document.getElementById('changePassBtn').addEventListener('click', async () => {
  const oldPass = document.getElementById('oldPass').value;
  const newPass = document.getElementById('newPass').value;
  if (!newPass || newPass.length < 6) { showToast('Новый пароль должен быть не короче 6 символов', true); return; }
  const res = await fetch('/api/admin/password', {
    method: 'POST', headers: { 'Content-Type': 'application/json', ...authHeaders() },
    body: JSON.stringify({ old_password: oldPass, new_password: newPass })
  });
  if (res.ok) {
    showToast('Пароль обновлён');
    document.getElementById('oldPass').value = '';
    document.getElementById('newPass').value = '';
  } else {
    showToast('Ошибка: проверьте текущий пароль', true);
  }
});

checkAuth();
