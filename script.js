// ==========================================================================
// Dmitry Zhdanov — Cyber Portfolio — script.js
// ==========================================================================

document.getElementById('year').textContent = new Date().getFullYear();

/* ---------- Mobile nav ---------- */
const burger = document.getElementById('burger');
const navLinks = document.getElementById('navLinks');
burger.addEventListener('click', () => navLinks.classList.toggle('open'));
navLinks.querySelectorAll('a').forEach(a => a.addEventListener('click', () => navLinks.classList.remove('open')));

/* ---------- Matrix rain background ---------- */
(function matrixRain() {
  const canvas = document.getElementById('matrix-bg');
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

/* ---------- Projects: fetch, filter, render ---------- */
const state = { all: [], filtered: [], category: 'Все', query: '', visible: 12 };

async function loadProjects() {
  try {
    const res = await fetch('/api/projects');
    state.all = await res.json();
  } catch (e) {
    console.error('Не удалось загрузить проекты', e);
    state.all = [];
  }
  buildFilters();
  applyFilters();
}

function buildFilters() {
  const cats = ['Все', ...new Set(state.all.map(p => p.category))];
  const wrap = document.getElementById('filters');
  wrap.innerHTML = cats.map(c =>
    `<button class="filter-btn ${c === state.category ? 'active' : ''}" data-cat="${c}">${c}</button>`
  ).join('');
  wrap.querySelectorAll('.filter-btn').forEach(btn => {
    btn.addEventListener('click', () => {
      state.category = btn.dataset.cat;
      state.visible = 12;
      wrap.querySelectorAll('.filter-btn').forEach(b => b.classList.remove('active'));
      btn.classList.add('active');
      applyFilters();
    });
  });
}

function applyFilters() {
  const q = state.query.trim().toLowerCase();
  state.filtered = state.all.filter(p => {
    const matchCat = state.category === 'Все' || p.category === state.category;
    const hay = `${p.title} ${p.niche} ${(p.tech || []).join(' ')}`.toLowerCase();
    const matchQ = !q || hay.includes(q);
    return matchCat && matchQ;
  });
  renderGrid();
}

function renderGrid() {
  const grid = document.getElementById('projectsGrid');
  const slice = state.filtered.slice(0, state.visible);
  grid.innerHTML = slice.map(cardHtml).join('');
  grid.querySelectorAll('.project-card').forEach(card => {
    card.addEventListener('click', () => openModal(parseInt(card.dataset.id)));
  });
  document.getElementById('loadMoreBtn').style.display =
    state.visible >= state.filtered.length ? 'none' : 'inline-flex';
}

function cardHtml(p) {
  const tech = (p.tech || []).slice(0, 3).map(t => `<span>${escapeHtml(t)}</span>`).join('');
  const demoBtn = p.demo_url
    ? `<a href="${escapeHtml(p.demo_url)}" target="_blank" rel="noopener" class="demo-link-btn" onclick="event.stopPropagation()">Перейти на сайт →</a>`
    : '';
  return `
  <div class="project-card ${p.theme || 'cyber-green'}" data-id="${p.id}">
    <span class="cat-tag">${escapeHtml(p.category)} · #${p.id}</span>
    <h3>${escapeHtml(p.title)}</h3>
    <p>${escapeHtml(p.description || '')}</p>
    <div class="tags">${tech}</div>
    <span class="secure-tag">🛡 secure-by-default</span>
    ${demoBtn}
  </div>`;
}

function escapeHtml(s) {
  return String(s || '').replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
}

document.getElementById('searchInput').addEventListener('input', (e) => {
  state.query = e.target.value;
  state.visible = 12;
  applyFilters();
});

document.getElementById('loadMoreBtn').addEventListener('click', () => {
  state.visible += 12;
  renderGrid();
});

/* ---------- Modal ---------- */
const modalOverlay = document.getElementById('modalOverlay');
document.getElementById('modalClose').addEventListener('click', closeModal);
modalOverlay.addEventListener('click', (e) => { if (e.target === modalOverlay) closeModal(); });

function openModal(id) {
  const p = state.all.find(x => x.id === id);
  if (!p) return;
  const featuresHtml = (p.features || []).map(f => `<li>${escapeHtml(f)}</li>`).join('');
  const techHtml = (p.tech || []).map(t => `<span>${escapeHtml(t)}</span>`).join('');
  const demoBtn = p.demo_url
    ? `<a href="${escapeHtml(p.demo_url)}" target="_blank" rel="noopener" class="demo-link-btn">Перейти на сайт →</a>`
    : '';
  document.getElementById('modalContent').innerHTML = `
    <span class="cat-tag" style="color:var(--neon-green)">${escapeHtml(p.category)} · #${p.id}</span>
    <h3>${escapeHtml(p.title)}</h3>
    <p style="color:var(--text-1)">${escapeHtml(p.description || '')}</p>
    <div class="tags">${techHtml}</div>
    <b style="font-size:0.85rem">Опции:</b>
    <ul>${featuresHtml}</ul>
    <span class="secure-tag">🛡 базовая защита: XSS / CSRF / валидация форм</span>
    ${demoBtn}
  `;
  modalOverlay.classList.add('open');
}
function closeModal() { modalOverlay.classList.remove('open'); }

/* ---------- Contact form ---------- */
document.getElementById('contactForm').addEventListener('submit', async (e) => {
  e.preventDefault();
  const form = e.target;
  const status = document.getElementById('formStatus');
  const data = Object.fromEntries(new FormData(form).entries());

  if (!data.name || !data.contact || !data.message) {
    status.textContent = 'Заполните все обязательные поля.';
    status.className = 'form-status err';
    return;
  }

  status.textContent = 'Отправка...';
  status.className = 'form-status';
  try {
    const res = await fetch('/api/contact', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data)
    });
    if (!res.ok) throw new Error('bad status');
    status.textContent = '✓ Заявка отправлена. Отвечу в течение дня.';
    status.className = 'form-status ok';
    form.reset();
  } catch (err) {
    status.textContent = 'Ошибка отправки. Попробуйте позже или напишите напрямую.';
    status.className = 'form-status err';
  }
});

/* ---------- Load profile (email/telegram) ---------- */
async function loadProfile() {
  try {
    const res = await fetch('/api/profile');
    const p = await res.json();
    if (p.email) document.getElementById('emailText').textContent = p.email;
    if (p.telegram) document.getElementById('tgText').textContent = p.telegram;
  } catch (e) { /* keep defaults */ }
}

loadProjects();
loadProfile();
