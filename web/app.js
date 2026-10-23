'use strict';
const $ = s => document.querySelector(s);
const $$ = s => [...document.querySelectorAll(s)];

// ---------- theme ----------
const savedTheme = localStorage.getItem('ih_theme');
if (savedTheme) document.documentElement.dataset.theme = savedTheme;
else if (matchMedia('(prefers-color-scheme: dark)').matches) document.documentElement.dataset.theme = 'dark';
$('#themeBtn').onclick = () => {
  const t = document.documentElement.dataset.theme === 'dark' ? 'light' : 'dark';
  document.documentElement.dataset.theme = t;
  localStorage.setItem('ih_theme', t);
};

// ---------- state ----------
let ME = { admin: false, guest_pin_required: false, limits: {} };
let STATS = null;
let fmt = localStorage.getItem('ih_fmt') || 'url';
let galleryState = { page: 1, total: 0, items: [] };
let viewMode = 'grid';
let sessionUploads = JSON.parse(sessionStorage.getItem('ih_session_uploads') || '[]');

function toast(msg) {
  const t = $('#toast'); t.textContent = msg; t.classList.remove('hidden');
  clearTimeout(t._h); t._h = setTimeout(() => t.classList.add('hidden'), 2200);
}
async function copyText(txt, msg) {
  try { await navigator.clipboard.writeText(txt); }
  catch {
    const ta = document.createElement('textarea');
    ta.value = txt; document.body.appendChild(ta); ta.select();
    document.execCommand('copy'); ta.remove();
  }
  toast(msg || '已复制');
}
function fmtBytes(b) {
  if (b == null) return '-';
  const u = ['B', 'KB', 'MB', 'GB', 'TB']; let i = 0; let v = b;
  while (v >= 1024 && i < u.length - 1) { v /= 1024; i++; }
  return v.toFixed(v >= 100 || i === 0 ? 0 : 2) + ' ' + u[i];
}
function fmtTime(ts) {
  const d = new Date(ts * 1000);
  return d.getFullYear() + '-' + String(d.getMonth() + 1).padStart(2, '0') + '-' + String(d.getDate()).padStart(2, '0') +
    ' ' + String(d.getHours()).padStart(2, '0') + ':' + String(d.getMinutes()).padStart(2, '0');
}
function fmtCardTime(ts) {
  const d = new Date(ts * 1000);
  return String(d.getMonth() + 1).padStart(2, '0') + '-' + String(d.getDate()).padStart(2, '0');
}
function linkOf(item, f) {
  if (f === 'markdown') return item.markdown || `![${item.orig}](${item.url})`;
  if (f === 'html') return `<img src="${item.url}" alt="${item.orig}">`;
  if (f === 'bbcode') return `[img]${item.url}[/img]`;
  return item.url;
}
async function api(path, opts) {
  const res = await fetch(path, opts);
  let data = null;
  try { data = await res.json(); } catch { }
  if (!res.ok || (data && data.success === false)) {
    throw new Error((data && data.error) || ('请求失败 ' + res.status));
  }
  return data;
}

// ---------- format segmented controls ----------
function syncFmtGroups() {
  $$('[data-fmt-group]').forEach(g => {
    g.querySelectorAll('button').forEach(b => b.classList.toggle('on', b.dataset.fmt === fmt));
  });
}
$$('[data-fmt-group]').forEach(g => {
  g.querySelectorAll('button').forEach(b => b.onclick = () => {
    fmt = b.dataset.fmt; localStorage.setItem('ih_fmt', fmt); syncFmtGroups();
  });
});
syncFmtGroups();
$('#autoCopy').checked = localStorage.getItem('ih_autocopy') === '1';
$('#autoCopy').onchange = e => localStorage.setItem('ih_autocopy', e.target.checked ? '1' : '0');

// ---------- session uploads ----------
function renderSession() {
  const card = $('#sessionCard');
  if (!sessionUploads.length) { card.classList.add('hidden'); return; }
  card.classList.remove('hidden');
  $('#sessionGrid').innerHTML = sessionUploads.map((it, i) => `
    <div class="img-card">
      <img class="thumb" src="${it.url}" alt="" loading="lazy" data-act="zoom" data-i="${i}">
      <div class="img-meta"><div class="img-name" title="${escapeHtml(it.orig)}">${escapeHtml(it.orig)}</div>
      <div class="img-sub">${fmtBytes(it.size)}</div></div>
      <div class="img-actions"><button class="btn" data-act="copy" data-i="${i}">复制</button></div>
    </div>`).join('');
  $('#sessionGrid').querySelectorAll('[data-act]').forEach(el => {
    el.onclick = () => {
      const it = sessionUploads[+el.dataset.i];
      if (el.dataset.act === 'copy') copyText(linkOf(it, fmt));
      if (el.dataset.act === 'zoom') openDetail(it, false);
    };
  });
}
function escapeHtml(s) {
  return String(s).replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
}

// ---------- upload ----------
const dz = $('#dropzone'), fi = $('#fileInput');
dz.onclick = () => fi.click();
dz.onkeydown = e => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); fi.click(); } };
['dragenter', 'dragover'].forEach(ev => dz.addEventListener(ev, e => { e.preventDefault(); dz.classList.add('drag'); }));
['dragleave', 'drop'].forEach(ev => dz.addEventListener(ev, e => { e.preventDefault(); dz.classList.remove('drag'); }));
dz.addEventListener('drop', e => handleFiles([...e.dataTransfer.files]));
document.addEventListener('paste', e => {
  const files = [...(e.clipboardData || {}).files || []];
  if (files.length) handleFiles(files);
});
fi.onchange = () => { handleFiles([...fi.files]); fi.value = ''; };

function handleFiles(files) {
  files = files.filter(f => f.size > 0);
  if (!files.length) return;
  const maxMB = ME.limits.max_file_mb || 10;
  files.forEach(f => {
    const row = document.createElement('div');
    row.className = 'q-item';
    row.innerHTML = `<img class="q-thumb" alt=""><div class="q-main"><div class="q-name">${escapeHtml(f.name || '剪贴板图片')}</div><div class="q-bar"><i></i></div></div><span class="q-state">上传中…</span>`;
    const thumb = row.querySelector('.q-thumb');
    if (f.type.startsWith('image/')) thumb.src = URL.createObjectURL(f);
    $('#queue').prepend(row);
    if (f.size > maxMB * 1024 * 1024) {
      row.querySelector('.q-state').textContent = '超过 ' + maxMB + ' MB';
      row.querySelector('.q-state').classList.add('fail');
      row.querySelector('.q-bar i').style.width = '0';
      return;
    }
    uploadOne(f, row);
  });
}

function uploadOne(file, row) {
  const fd = new FormData();
  fd.append('file', file);
  const pin = $('#pinInput').value.trim();
  if (pin) fd.append('pin', pin);
  const xhr = new XMLHttpRequest();
  xhr.open('POST', '/api/upload');
  xhr.upload.onprogress = e => {
    if (e.lengthComputable) row.querySelector('.q-bar i').style.width = Math.round(e.loaded / e.total * 100) + '%';
  };
  xhr.onload = () => {
    const st = row.querySelector('.q-state');
    let data = null;
    try { data = JSON.parse(xhr.responseText); } catch { }
    if (xhr.status === 200 && data && data.success && data.data && data.data.length) {
      st.textContent = '完成'; st.classList.add('ok');
      row.querySelector('.q-bar i').style.width = '100%';
      const item = data.data[0];
      sessionUploads.unshift(item);
      sessionStorage.setItem('ih_session_uploads', JSON.stringify(sessionUploads.slice(0, 60)));
      renderSession();
      if ($('#autoCopy').checked) copyText(item.markdown, '已复制 Markdown');
      if (ME.admin) { loadStats(); galleryState.page = 1; loadGallery(false); }
      setTimeout(() => row.remove(), 2600);
    } else {
      st.textContent = (data && data.error) || '上传失败';
      st.classList.add('fail');
    }
  };
  xhr.onerror = () => {
    const st = row.querySelector('.q-state');
    st.textContent = '网络错误'; st.classList.add('fail');
  };
  xhr.send(fd);
}

// ---------- auth ----------
function openLogin() { $('#loginModal').classList.remove('hidden'); $('#loginErr').classList.add('hidden'); setTimeout(() => $('#loginPwd').focus(), 50); }
function closeModals() { $$('.modal').forEach(m => m.classList.add('hidden')); }
$$('[data-close]').forEach(el => el.onclick = closeModals);
document.addEventListener('keydown', e => { if (e.key === 'Escape') closeModals(); });
$('#loginBtn').onclick = openLogin;
$('#loginBtn2').onclick = openLogin;
$('#loginForm').onsubmit = async e => {
  e.preventDefault();
  try {
    await api('/api/login', {
      method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ password: $('#loginPwd').value })
    });
    closeModals(); toast('登录成功'); await init();
  } catch (err) {
    const le = $('#loginErr'); le.textContent = err.message; le.classList.remove('hidden');
  }
};
$('#logoutBtn').onclick = async () => {
  await api('/api/logout', { method: 'POST' }).catch(() => { });
  sessionUploads = []; sessionStorage.removeItem('ih_session_uploads');
  await init(); toast('已退出');
};

// ---------- stats & gallery (admin) ----------
async function loadStats() {
  STATS = (await api('/api/stats')).data;
  $('#stUsed').textContent = fmtBytes(STATS.total_bytes);
  $('#stQuota').textContent = fmtBytes(STATS.total_quota);
  $('#stUsedBar').style.width = Math.min(100, STATS.total_bytes / STATS.total_quota * 100) + '%';
  $('#stCount').textContent = STATS.total_count;
  $('#stToday').textContent = STATS.today_count;
  $('#stGuest').textContent = fmtBytes(STATS.guest_bytes);
  $('#stGuestQuota').textContent = fmtBytes(STATS.guest_quota);
  $('#stGuestBar').style.width = Math.min(100, STATS.guest_bytes / STATS.guest_quota * 100) + '%';
  const up = STATS.uptime_sec;
  $('#stUptime').textContent = up >= 86400 ? Math.floor(up / 86400) + ' 天' : up >= 3600 ? Math.floor(up / 3600) + ' 小时' : Math.max(1, Math.floor(up / 60)) + ' 分钟';
  // PicGo card
  $('#pgServer').textContent = STATS.base_url;
  $('#pgToken').dataset.real = STATS.api_token;
  $('#pgToken').textContent = '••••••••••••••••';
  $('#pgToggle').textContent = '显示';
  const cfg = {
    picBed: {
      uploader: 'lskyplist',
      lskyplist: {
        version: 'V2',
        host: STATS.base_url,
        token: 'Bearer ' + STATS.api_token,
        strategyId: '',
        albumId: ''
      }
    },
    picgoPlugins: {}
  };
  $('#pgConfig').textContent = JSON.stringify(cfg, null, 2);
}

async function loadGallery(append) {
  const params = new URLSearchParams({
    q: $('#galSearch').value.trim(),
    owner: $('#galOwner').value,
    sort: $('#galSort').value,
    page: galleryState.page,
    page_size: 60
  });
  const data = (await api('/api/images?' + params)).data;
  galleryState.total = data.total;
  galleryState.items = append ? galleryState.items.concat(data.items) : data.items;
  $('#galCount').textContent = '共 ' + data.total + ' 张';
  $('#galEmpty').classList.toggle('hidden', galleryState.items.length > 0);
  $('#galMore').classList.toggle('hidden', galleryState.items.length >= data.total);
  const g = $('#gallery');
  g.classList.toggle('list-view', viewMode === 'list');
  g.innerHTML = galleryState.items.map((it, i) => `
    <div class="img-card">
      <img class="thumb" src="${it.url}" alt="" loading="lazy" data-act="zoom" data-i="${i}">
      <div class="img-meta">
        <div class="img-name" title="${escapeHtml(it.orig)}">${escapeHtml(it.orig)}</div>
        <div class="img-sub"><span>${fmtBytes(it.size)}</span><span>${fmtCardTime(it.ts)}</span>
        ${it.owner === 'guest' ? '<span class="badge-guest">游客</span>' : ''}</div>
      </div>
      <div class="img-actions">
        <button class="btn" data-act="copy" data-i="${i}">复制</button>
        <button class="btn btn-del" data-act="del" data-i="${i}">删除</button>
      </div>
    </div>`).join('');
  g.querySelectorAll('[data-act]').forEach(el => {
    el.onclick = async () => {
      const it = galleryState.items[+el.dataset.i];
      if (el.dataset.act === 'copy') copyText(linkOf(it, fmt));
      if (el.dataset.act === 'zoom') openDetail(it, true);
      if (el.dataset.act === 'del') await deleteImage(it);
    };
  });
}
$('#galSearch').addEventListener('input', () => { galleryState.page = 1; loadGallery(false); });
$('#galOwner').onchange = () => { galleryState.page = 1; loadGallery(false); };
$('#galSort').onchange = () => { galleryState.page = 1; loadGallery(false); };
$('#galMore').onclick = () => { galleryState.page++; loadGallery(true); };
$('#viewGrid').onclick = () => { viewMode = 'grid'; $('#viewGrid').classList.add('on'); $('#viewList').classList.remove('on'); loadGallery(false); };
$('#viewList').onclick = () => { viewMode = 'list'; $('#viewList').classList.add('on'); $('#viewGrid').classList.remove('on'); loadGallery(false); };

async function deleteImage(it) {
  if (!confirm('确定删除 ' + it.orig + ' 吗？删除后链接立即失效。')) return;
  await api('/api/images/' + it.id, { method: 'DELETE' });
  toast('已删除');
  closeModals();
  loadStats();
  galleryState.page = 1; loadGallery(false);
}

// ---------- detail modal ----------
let detailItem = null;
function openDetail(it, admin) {
  detailItem = it;
  $('#dtImg').src = it.url;
  $('#dtMeta').innerHTML =
    `<span>${escapeHtml(it.orig)}</span><span>${fmtBytes(it.size)}</span>` +
    (it.ts ? `<span>${fmtTime(it.ts)}</span>` : '') +
    (it.owner ? `<span>来源：${it.owner === 'guest' ? '游客' : '管理员'}${it.ip ? ' · ' + escapeHtml(it.ip) : ''}</span>` : '');
  $('#dtLinks').innerHTML = ['url', 'markdown', 'html', 'bbcode'].map(f =>
    `<div class="kv-row"><span class="kv-k">${{ url: '直链', markdown: 'Markdown', html: 'HTML', bbcode: 'BBCode' }[f]}</span><code>${escapeHtml(linkOf(it, f))}</code><button class="btn btn-small" data-f="${f}">复制</button></div>`).join('');
  $('#dtLinks').querySelectorAll('button').forEach(b => b.onclick = () => copyText(linkOf(it, b.dataset.f)));
  $('#dtDelete').style.display = admin ? '' : 'none';
  $('#detailModal').classList.remove('hidden');
}
$('#dtDelete').onclick = () => { if (detailItem) deleteImage(detailItem); };

// ---------- picgo buttons ----------
$('#pgToggle').onclick = () => {
  const el = $('#pgToken');
  const shown = el.textContent !== '••••••••••••••••';
  el.textContent = shown ? '••••••••••••••••' : (el.dataset.real || '');
  $('#pgToggle').textContent = shown ? '显示' : '隐藏';
};
$('#pgCopy').onclick = () => copyText($('#pgToken').dataset.real || '', 'Token 已复制');
$$('[data-copy-target]').forEach(b => b.onclick = () => copyText($('#' + b.dataset.copyTarget).textContent));
$('#pgCfgCopy').onclick = () => copyText($('#pgConfig').textContent, '配置已复制');
$('#pgRegen').onclick = async () => {
  if (!confirm('重新生成 token 后，旧 token 立即失效，PicGo 需要更新配置。继续？')) return;
  const d = await api('/api/admin/token', { method: 'POST' });
  toast('新 token 已生成');
  loadStats();
};

// ---------- settings ----------
$('#pwdForm').onsubmit = async e => {
  e.preventDefault();
  try {
    await api('/api/admin/password', {
      method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ old: $('#pwdOld').value, new: $('#pwdNew').value })
    });
    $('#pwdOld').value = ''; $('#pwdNew').value = '';
    toast('密码已修改');
  } catch (err) { toast(err.message); }
};
$('#purgeBtn').onclick = async () => {
  if (!confirm('确定删除全部游客上传的图片吗？')) return;
  const d = await api('/api/admin/purge-guests', { method: 'POST' });
  toast('已清理 ' + d.data.removed + ' 张游客图片');
  loadStats(); galleryState.page = 1; loadGallery(false);
};

// ---------- init ----------
async function init() {
  ME = (await api('/api/me')).data;
  const L = ME.limits;
  $('#dzRules').textContent = `支持 jpg / png / gif / webp / avif / bmp / ico · 单张最大 ${L.max_file_mb} MB` +
    (ME.admin ? '' : ` · 游客每天最多 ${L.guest_daily} 张`);
  $('#pinRow').classList.toggle('hidden', !ME.guest_pin_required || ME.admin);
  $('#modeBadge').textContent = ME.admin ? '管理员' : '游客模式';
  $('#loginBtn').classList.toggle('hidden', ME.admin);
  $('#guestNote').classList.toggle('hidden', ME.admin);
  $('#logoutBtn').classList.toggle('hidden', !ME.admin);
  $('#statsRow').classList.toggle('hidden', !ME.admin);
  $('#galleryCard').classList.toggle('hidden', !ME.admin);
  $('#picgoCard').classList.toggle('hidden', !ME.admin);
  $('#settingsCard').classList.toggle('hidden', !ME.admin);
  $('#rulesList').innerHTML =
    `<li>单个文件最大 ${L.max_file_mb} MB，仅图片（服务端校验真实类型，不收 SVG）</li>
     <li>游客每天最多 ${L.guest_daily} 张，游客空间上限 ${L.guest_quota_gb} GB，总空间上限 ${L.total_quota_gb} GB</li>
     <li>游客上传口令：${ME.guest_pin_required ? '已开启' : '未开启（在服务端用 IMG_GUEST_PIN 设置）'}</li>`;
  renderSession();
  if (ME.admin) { await loadStats(); galleryState.page = 1; await loadGallery(false); }
}
init().catch(() => toast('初始化失败，请刷新页面'));
