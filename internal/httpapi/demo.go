package httpapi

import "net/http"

const demoHTML = `<!doctype html>
<html lang="ru">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>LK Advisor — личный кабинет (демо)</title>
<style>
:root{--bg:#f4f6fa;--card:#fff;--text:#111827;--muted:#64748b;--line:#e2e8f0;--accent:#cc1e35;--blue:#2563eb;--green:#059669;--purple:#7c3aed;--shadow:0 12px 34px rgba(15,23,42,.07);--shadow-soft:0 4px 16px rgba(15,23,42,.05)}
*{box-sizing:border-box}
body{margin:0;background:radial-gradient(900px 500px at 18% -10%,#e8eeff 0,transparent 55%),var(--bg);color:var(--text);font-family:Inter,ui-sans-serif,system-ui,-apple-system,"Segoe UI",sans-serif;line-height:1.5;-webkit-font-smoothing:antialiased}
header{background:linear-gradient(135deg,#070b14 0%,#111827 58%,#1f2937 100%);color:#fff;padding:20px 0;border-bottom:1px solid rgba(255,255,255,.07);box-shadow:0 10px 34px rgba(15,23,42,.2)}
.wrap{max-width:1120px;margin:0 auto;padding:0 20px}
.header-row{display:flex;align-items:center;justify-content:space-between;gap:16px}
.brand{display:flex;align-items:center;gap:12px}
.admin-link{color:#cbd5e1;text-decoration:none;font-size:12.5px;background:rgba(255,255,255,.04);border:1px solid rgba(255,255,255,.14);padding:8px 12px;border-radius:10px;transition:.15s}
.admin-link:hover{color:#fff;border-color:rgba(255,255,255,.35);background:rgba(255,255,255,.08)}
.head-links{display:flex;gap:8px;align-items:center}
.logo{width:40px;height:40px;border-radius:12px;background:linear-gradient(135deg,#e11d48,#991b1b);display:grid;place-items:center;font-weight:800;font-size:13px;letter-spacing:.5px;box-shadow:0 8px 22px rgba(225,29,72,.35)}
h1{font-size:22px;margin:0;letter-spacing:-.02em}
.sub{color:#94a3b8;font-size:12.5px;margin-top:2px}
.tabs{display:flex;gap:10px;flex-wrap:wrap;margin:24px 0}
.client{background:linear-gradient(135deg,#fff 0%,#f8fafc 100%);border:1px solid var(--line);border-left:4px solid var(--accent);border-radius:14px;padding:18px 20px;margin-bottom:18px;display:flex;justify-content:space-between;gap:16px;flex-wrap:wrap;align-items:center;box-shadow:var(--shadow-soft)}
.client b{font-size:16px}
.client .ctx{color:var(--muted);font-size:13px;max-width:680px}
.profile-grid{display:grid;grid-template-columns:1.1fr 1fr;gap:24px}
@media(max-width:860px){.profile-grid{grid-template-columns:1fr}}
.profile-grid h3{font-size:11.5px;text-transform:uppercase;letter-spacing:.04em;color:var(--muted);margin:0 0 8px;white-space:nowrap}
.profile-grid ul{list-style:none;margin:0;padding:0;display:flex;flex-direction:column;gap:6px;font-size:13px}
.profile-grid li{display:flex;gap:6px;flex-wrap:wrap}
#purchases li{display:block;padding:7px 0;border-bottom:1px solid var(--line)}
#purchases li b{display:block;font-size:12.5px;line-height:1.3}
#purchases li .meta{display:block;margin-top:2px;font-size:11px;white-space:nowrap}
#purchases li:last-child{border-bottom:0}
.owned{list-style:none;margin:0;padding:0;display:flex;flex-direction:column;gap:8px;font-size:13px}
.owned li{display:flex;align-items:center;justify-content:space-between;gap:10px;border-bottom:1px solid var(--line);padding-bottom:8px}
.owned .rm{border:0;background:none;color:var(--accent);cursor:pointer;font:inherit;font-size:12px}
.add-row{display:flex;gap:8px;margin-top:10px}
.add-row select{flex:1;min-width:0;border:1px solid var(--line);border-radius:8px;padding:8px;font:inherit}
#profile-panel{margin-bottom:20px}
.card.top{border-color:rgba(204,30,53,.35);box-shadow:0 0 0 1px rgba(204,30,53,.12),0 12px 30px rgba(204,30,53,.06)}
.tab{border:1px solid var(--line);background:#fff;border-radius:12px;padding:10px 14px;cursor:pointer;font:inherit;text-align:left;transition:.15s;box-shadow:0 1px 2px rgba(15,23,42,.03)}
.tab:hover{border-color:#cbd5e1;transform:translateY(-1px)}
.tab b{display:block;font-size:12.5px}
.tab span{font-size:10.5px;color:var(--muted)}
.tab.active{border-color:var(--accent);box-shadow:0 0 0 2px #fde8ec,0 6px 18px rgba(204,30,53,.08)}
.grid{display:grid;grid-template-columns:1.4fr 1fr;gap:20px;align-items:start}
@media(max-width:860px){.grid{grid-template-columns:1fr}}
.panel{background:var(--card);border:1px solid var(--line);border-radius:16px;padding:20px;box-shadow:var(--shadow-soft)}
.panel h2{font-size:16px;margin:0 0 14px;letter-spacing:-.01em}
.card{border:1px solid var(--line);border-radius:14px;padding:16px;margin-bottom:14px;background:#fff;box-shadow:0 2px 10px rgba(15,23,42,.03);transition:.15s}
.card:hover{transform:translateY(-1px);box-shadow:0 10px 28px rgba(15,23,42,.07)}
.card h3{margin:8px 0 4px;font-size:18px;letter-spacing:-.01em;line-height:1.3}
.badge{display:inline-block;font-size:11px;font-weight:bold;padding:2px 8px;border-radius:999px;background:#eef2ff;color:var(--blue)}
.badge.upsell{background:#f3e8ff;color:var(--purple)}
.badge.cross_sell{background:#e7f7ef;color:var(--green)}
.badge.fallback{background:#f3f4f6;color:var(--muted)}
.badge.alternative{background:#fff4e5;color:#b45309}
.meta{color:var(--muted);font-size:13px}
.promo{display:inline-block;margin-left:8px;font-size:11px;font-weight:bold;color:#b45309;background:#fef3c7;padding:2px 8px;border-radius:999px}
.reason{margin:12px 0 10px;color:#334155}
.evidence{font-size:11px;color:var(--muted)}
.evidence summary{cursor:pointer;color:#64748b}
.evidence pre{white-space:pre-wrap;margin:8px 0 0;background:#f8fafc;border:1px solid var(--line);padding:10px;border-radius:8px;font-size:11px}
.actions{display:flex;gap:8px;margin-top:12px}
button.primary{background:var(--accent);color:#fff;border:0;border-radius:9px;padding:9px 14px;cursor:pointer;font:inherit;font-weight:600;box-shadow:0 5px 16px rgba(204,30,53,.2)}
button.primary:hover{background:#b91c1c}
button.ghost{background:#fff;border:1px solid var(--line);border-radius:9px;padding:9px 14px;cursor:pointer;font:inherit}
button.ghost:hover{border-color:#cbd5e1;background:#f8fafc}
.price-line{display:flex;align-items:center;gap:8px;flex-wrap:wrap;margin-top:2px}
.price{font-size:16px;font-weight:750;letter-spacing:-.01em}
.score{font-size:11px;color:var(--muted);background:#f1f5f9;border-radius:6px;padding:2px 6px}
.scorebar{height:4px;background:#f1f5f9;border-radius:999px;overflow:hidden;margin:8px 0 2px}
.scorebar i{display:block;height:100%;background:linear-gradient(90deg,#fb7185,#cc1e35);border-radius:999px}
.banner{background:#fff7ed;border:1px solid #fed7aa;color:#9a3412;padding:10px 12px;border-radius:8px;margin-bottom:12px;font-size:13px}
.messages{display:flex;flex-direction:column;gap:10px;max-height:420px;overflow:auto;margin-bottom:12px}
.msg{padding:10px 12px;border-radius:10px;font-size:14px;white-space:pre-wrap}
.msg.user{background:#eef2ff;align-self:flex-end;max-width:90%}
.msg.bot{background:#f9fafb;border:1px solid var(--line)}
.tools{margin-top:6px;display:flex;gap:6px;flex-wrap:wrap}
.tool{font-size:10px;background:#111827;color:#fff;border-radius:999px;padding:2px 7px}
.chat-form{display:flex;gap:8px}
.chat-form input{flex:1;border:1px solid var(--line);border-radius:8px;padding:10px;font:inherit}
.foot{color:var(--muted);font-size:12px;margin:12px 0 0}
a{color:var(--blue)}
</style>
</head>
<body>
<header><div class="wrap header-row"><div class="brand"><div class="logo">LK</div><div><h1>LK Advisor</h1><div class="sub">Личный кабинет · демо для жюри</div></div></div><div class="head-links"><a class="admin-link" href="/admin#experiments">Гипотезы</a><a class="admin-link" href="/admin">Настройка правил</a></div></div></header>
<main class="wrap">
  <div class="tabs" id="tabs"></div>
  <div class="client" id="client"></div>
  <section class="panel" id="profile-panel">
    <h2>Профиль клиента</h2>
    <div class="profile-grid">
      <div>
        <h3>У клиента уже есть</h3>
        <ul id="owned" class="owned"></ul>
        <div class="add-row">
          <select id="add-product"></select>
          <button class="ghost" id="add-btn" type="button">Добавить</button>
        </div>
        <div class="meta" style="margin-top:6px">Демо: добавьте или уберите продукт — рекомендация пересчитается.</div>
      </div>
      <div>
        <h3>История покупок</h3><ul id="purchases"></ul>
        <h3 style="margin-top:16px">Среда</h3><div id="environment" class="meta">—</div>
      </div>
    </div>
  </section>
  <section class="panel" id="audit-panel" style="margin-bottom:20px">
    <h2>Журнал изменений</h2>
    <ul id="audit" class="owned"></ul>
  </section>
  <div class="grid">
    <section class="panel">
      <h2 id="rec-title">Рекомендации</h2>
      <div id="recs"></div>
    </section>
    <section class="panel">
      <h2>Спросите по рекомендациям</h2>
      <div class="messages" id="messages"></div>
      <form class="chat-form" id="chat-form">
        <input id="question" placeholder="Почему это предложение?" autocomplete="off">
        <button class="primary" type="submit">Спросить</button>
      </form>
      <p class="foot">Числа возвращают инструменты, модель только формулирует. <a href="/api">API</a></p>
    </section>
  </div>
</main>
<script>
var scenarios = [
  {id:'smb-001', label:'Продление', note:'Kaspersky истекает через 14 дней', client:'ООО «Ромашка»', context:'SMB · активна лицензия Kaspersky Endpoint Security Cloud на 10 мест', sample:'Что будет, если не продлить?'},
  {id:'smb-002', label:'Апгрейд', note:'Visio Standard → Professional, акция 10%', client:'ИП Сергеев', context:'SMB · куплен Microsoft Visio Standard 2024, лицензий нет', sample:'Почему Microsoft Visio Professional 2024?'},
  {id:'smb-003', label:'Апгрейд', note:'Microsoft 365: бизнес стандарт → премиум', client:'ООО «Вектор»', context:'SMB · куплен Microsoft 365 бизнес стандарт, есть старший тариф', sample:'Почему пакет выгоднее?'},
  {id:'smb-005', label:'Допродажа', note:'Visio Professional → Project, нужно согласование', client:'ООО «Куб»', context:'SMB · куплен Visio Professional, доступен Microsoft Project Standard 2024', sample:'Почему нужен менеджер?'},
  {id:'smb-006', label:'Без истории', note:'fallback на популярное', client:'ООО «Новый клиент»', context:'SMB · истории покупок нет', sample:'Что предложить новому клиенту?'},
  {id:'lk-36612', label:'Реальный ЛК', note:'Kaspersky + Adobe expired + Dr.Web + Microsoft 365 Personal', client:'Компания 4', context:'B2B · тестовый ЛК Softline', sample:'Что предложить вместо Adobe?'}
];
var intents = {renewal:'Продление', upsell:'Апгрейд', cross_sell:'Допродажа', analog:'Замена', alternative:'Альтернатива', fallback:'Популярное'};
var current = (new URLSearchParams(location.search)).get('client') || 'smb-001';
var lastTrace = '';

function el(id){ return document.getElementById(id); }
function esc(s){ return String(s == null ? '' : s).replace(/[&<>"']/g, function(c){ return {'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]; }); }
function money(n){ if (!n || Number(n) <= 0) return 'цена по запросу'; return new Intl.NumberFormat('ru-RU').format(n) + ' ₽'; }

function scenario(id){ for (var i=0;i<scenarios.length;i++){ if (scenarios[i].id===id) return scenarios[i]; } return scenarios[0]; }

function renderClient(){
  var s = scenario(current);
  el('client').innerHTML = '<div><b>' + esc(s.client) + '</b><div class="ctx">' + esc(s.context) + '</div></div><div class="meta">' + esc(s.id) + '</div>';
}

function loadProfile(id){
  fetch('/v1/clients/' + encodeURIComponent(id) + '/profile')
    .then(function(res){ return res.json().then(function(d){ return {ok: res.ok, status: res.status, data: d}; }); })
    .then(function(out){
      if (!out.ok) {
        el('purchases').innerHTML = '<li class="meta">Профиль недоступен</li>';
        el('licenses').innerHTML = '';
        return;
      }
      renderProfile(out.data);
    })
    .catch(function(err){
      el('purchases').innerHTML = '<li class="meta">Сеть: ' + esc(err.message) + '</li>';
    });
}

function renderProfile(p){
  var s = scenario(current);
  var names = {business: 'бизнес', student: 'студент', consumer: 'частный'};
  var parts = (p.personas || []).map(function(s){
    return (names[s.type] || s.type) + ' ' + Math.round(s.score * 100) + '%';
  });
  var ctx = esc(p.segment) + ' · ' + esc(p.industry) + ' · ' + esc(p.size);
  if (parts.length) {
    ctx += ' · <b>' + esc(parts[0]) + '</b>';
    if (parts.length > 1) ctx += ' · ' + esc(parts.slice(1).join(' · '));
  } else {
    ctx += ' · сегмент не определён';
  }
  el('client').innerHTML = '<div><b>' + esc(s.client) + '</b><div class="ctx">' + ctx + '</div></div>' +
    '<div class="meta">' + esc(p.client_id) + '</div>';

  renderOwned(p);

  var pur = p.purchases || [];
  el('purchases').innerHTML = pur.length ? pur.map(function(x){
    return '<li><b>' + esc(x.title) + '</b><span class="meta">' + esc(x.date) + ' · ' + money(x.amount) + '</span></li>';
  }).join('') : '<li class="meta">Покупок нет</li>';

  var e = p.environment || {};
  var parts = [e.os, e.ram_gb ? e.ram_gb + ' ГБ' : '', e.cpu, e.gpu].filter(Boolean);
  el('environment').textContent = parts.length ? parts.join(' · ') : 'нет данных';
}

function renderOwned(p){
  var map = {};
  (p.purchases || []).forEach(function(x){
    map[x.product_id] = map[x.product_id] || {title: x.title, purchase: x.date, license: null};
  });
  (p.licenses || []).forEach(function(l){
    map[l.product_id] = map[l.product_id] || {title: l.title, purchase: null, license: null};
    if (l.status === 'active') { map[l.product_id].license = l; }
  });

  var ids = Object.keys(map);
  el('owned').innerHTML = ids.length ? ids.map(function(id){
    var x = map[id];
    var note = x.license ? ('лицензия до ' + x.license.end + ' · ' + x.license.seats + ' мест') : ('куплено ' + (x.purchase || ''));
    return '<li><span><b>' + esc(x.title) + '</b><br><span class="meta">' + esc(note) + '</span></span>' +
      '<button class="rm" data-id="' + esc(id) + '" type="button">Убрать</button></li>';
  }).join('') : '<li class="meta">Ничего нет</li>';

  Array.prototype.forEach.call(el('owned').querySelectorAll('.rm'), function(b){
    b.onclick = function(){ removeItem(b.getAttribute('data-id')); };
  });
}

function loadProducts(){
  fetch('/v1/products').then(function(r){ return r.json(); }).then(function(list){
    var sel = el('add-product');
    sel.innerHTML = '';
    var ph = document.createElement('option');
    ph.value = '';
    ph.textContent = 'Выберите продукт…';
    sel.appendChild(ph);
    (list || []).forEach(function(p){
      var o = document.createElement('option');
      o.value = p.id;
      o.textContent = p.title;
      sel.appendChild(o);
    });
  }).catch(function(){});
}

function addItem(){
  var sel = el('add-product');
  var id = sel.value;
  if (!id) return;
  var title = sel.selectedOptions && sel.selectedOptions[0] ? sel.selectedOptions[0].textContent : id;

  fetch('/v1/demo/clients/' + encodeURIComponent(current) + '/items', {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify({product_id: id})
  }).then(function(res){ return res.json().then(function(d){ return {ok: res.ok, status: res.status, data: d}; }); })
    .then(function(out){
      if (!out.ok) { addMsg('Не добавилось: ' + (out.data.error || out.status), 'bot'); return; }
      afterMutation('Добавили ' + title + '.');
    })
    .catch(function(err){ addMsg('Сеть: ' + err.message, 'bot'); });
}

function removeItem(id){
  fetch('/v1/demo/clients/' + encodeURIComponent(current) + '/items/' + encodeURIComponent(id), {method: 'DELETE'})
    .then(function(res){ return res.json().then(function(d){ return {ok: res.ok, status: res.status, data: d}; }); })
    .then(function(out){
      if (!out.ok) { addMsg('Не убралось: ' + (out.data.error || out.status), 'bot'); return; }
      afterMutation('Убрали продукт.');
    })
    .catch(function(err){ addMsg('Сеть: ' + err.message, 'bot'); });
}

function afterMutation(text){
  addMsg(text + ' Пересчитываю рекомендацию…', 'bot');
  loadProfile(current);
  loadAudit(current);
  loadRecs(current, false);
}

function loadAudit(id){
  fetch('/v1/demo/clients/' + encodeURIComponent(id) + '/audit')
    .then(function(r){ return r.json(); })
    .then(function(list){
      el('audit-panel').style.display = (list && list.length) ? '' : 'none';
      el('audit').innerHTML = (list && list.length) ? list.map(function(x){
        var verb = x.action === 'add' ? 'Добавили ' : 'Убрали ';
        return '<li><span><b>' + verb + esc(x.title) + '</b><br><span class="meta">' +
          esc(new Date(x.at).toLocaleString('ru-RU')) + '</span></span></li>';
      }).join('') : '<li class="meta">Пока без изменений</li>';
    })
    .catch(function(){});
}

function loadTrace(bot){
  fetch('/v1/demo/clients/' + encodeURIComponent(current) + '/agent-calls')
    .then(function(r){ return r.json(); })
    .then(function(list){
      if (!list || !list.length) return;
      var c = list[0];
      var d = document.createElement('details');
      d.className = 'evidence';
      var text = 'intent: ' + c.intent + '\nfallback: ' + c.fallback + '\nlatency: ' + c.latency_ms + ' ms\n' +
                 'base: ' + c.base_answer + '\nllm: ' + c.llm_answer + '\nguard: ' + c.guard_error;
      d.innerHTML = '<summary>Почему так</summary><pre>' + esc(text) + '</pre>';
      if (c.report) {
        var rd = document.createElement('details');
        rd.className = 'evidence';
        rd.innerHTML = '<summary>Отчёт из БД</summary><pre>' + esc(JSON.stringify(c.report, null, 1)) + '</pre>';
        d.appendChild(rd);
      }
      bot.appendChild(d);
    })
    .catch(function(){});
}

function renderTabs(){
  var box = el('tabs');
  box.innerHTML = '';
  scenarios.forEach(function(s){
    var b = document.createElement('button');
    b.className = 'tab' + (s.id === current ? ' active' : '');
    b.innerHTML = '<b>' + esc(s.label) + '</b><span>' + esc(s.id) + ' · ' + esc(s.note) + '</span>';
    b.onclick = function(){ loadClient(s.id, false); };
    box.appendChild(b);
  });
}

function renderRecs(data){
  var title = intents[data.intent] || data.intent;
  el('rec-title').textContent = 'Рекомендации · ' + title;
  var box = el('recs');
  box.innerHTML = '';

  lastTrace = data.trace_id || '';
  if (data.fallback) {
    var banner = document.createElement('div');
    banner.className = 'banner';
    banner.textContent = 'Истории нет — показываем популярное в сегменте.';
    box.appendChild(banner);
  }

  (data.recommendations || []).forEach(function(r, i){
    var card = document.createElement('article');
    card.className = 'card' + (i === 0 ? ' top' : '');

    var badge = r.type === 'fallback' ? 'fallback' : r.type;
    var promo = r.promo ? '<span class="promo">−' + esc(r.promo.discount) + '% до ' + esc(String(r.promo.valid_to).slice(0, 10)) + '</span>' : '';
    var cons = (r.constraints || []).indexOf('manager_approval') >= 0 ? ' · нужно согласование менеджера' : '';
    var src = {relation:'связь', license:'лицензия', promo:'акция', rule:'правило', fallback:'подбор'};
    var ev = (r.evidence || []).map(function(e){ return (src[e.source] || e.source) + ': ' + e.value; }).join('\n');

    card.innerHTML =
      '<span class="badge ' + esc(badge) + '">' + esc(intents[r.type] || r.type) + '</span>' +
      '<h3>' + esc(r.title) + '</h3>' +
      '<div class="price-line"><span class="price">' + money(r.price.amount) + '</span><span class="score">score ' + Number(r.score).toFixed(2) + '</span>' + cons + promo + '</div>' +
      '<div class="scorebar"><i style="width:' + Math.round(Math.max(0, Math.min(1, Number(r.score))) * 100) + '%"></i></div>' +
      '<p class="reason">' + esc(r.reason) + '</p>' +
      '<details class="evidence"><summary>Evidence</summary><pre>' + esc(ev) + '</pre></details>' +
      '<div class="actions"><button class="primary" type="button">Почему это?</button><button class="ghost" type="button">Купить</button></div>';

    var buttons = card.querySelectorAll('button');
    buttons[0].onclick = function(){ logEvent(r.product_id, 'clicked'); ask('Почему ' + r.title + '?'); };
    if (buttons[1]) buttons[1].onclick = function(){
      logEvent(r.product_id, 'purchased');
      buttons[1].textContent = 'Покупка записана';
      buttons[1].disabled = true;
    };
    box.appendChild(card);
  });
}

function logEvent(productId, event){
  if (!lastTrace) return;
  fetch('/v1/events', {
    method:'POST', headers:{'Content-Type':'application/json'},
    body: JSON.stringify({trace_id:lastTrace, client_id:current, product_id:productId, event:event})
  }).catch(function(){});
}
function addMsg(text, cls){
  var d = document.createElement('div');
  d.className = 'msg ' + cls;
  d.textContent = text;
  el('messages').appendChild(d);
  el('messages').scrollTop = 1000000;
  return d;
}

function loadRecs(id, withSample){
  el('recs').innerHTML = '<div class="meta">Загружаю…</div>';

  fetch('/v1/recommendations', {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify({client_id: id})
  }).then(function(res){ return res.json().then(function(d){ return {ok: res.ok, status: res.status, data: d}; }); })
    .then(function(out){
      if (!out.ok) { el('recs').innerHTML = '<div class="banner">Ошибка: ' + esc(out.data.error || out.status) + '</div>'; return; }
      renderRecs(out.data);
      if (withSample) { ask(scenario(id).sample); }
    })
    .catch(function(err){ el('recs').innerHTML = '<div class="banner">Сеть: ' + esc(err.message) + '</div>'; });
}

function loadClient(id, withSample){
  current = id;
  renderTabs();
  renderClient();
  loadProfile(id);
  loadAudit(id);
  el('messages').innerHTML = '';
  loadRecs(id, withSample);
}

function ask(question){
  question = (question || '').trim();
  if (!question) return;
  addMsg(question, 'user');
  el('question').value = '';
  var wait = addMsg('Думаю…', 'bot');

  fetch('/v1/ask', {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify({client_id: current, question: question})
  }).then(function(res){ return res.json().then(function(d){ return {ok: res.ok, status: res.status, data: d}; }); })
    .then(function(out){
      wait.remove();
      if (!out.ok) { addMsg('Ошибка: ' + (out.data.error || out.status), 'bot'); return; }
      var bot = addMsg(out.data.answer, 'bot');
      if (out.data.tool_calls && out.data.tool_calls.length) {
        var tools = document.createElement('div');
        tools.className = 'tools';
        out.data.tool_calls.forEach(function(c){
          var s = document.createElement('span');
          s.className = 'tool';
          s.textContent = c.tool;
          tools.appendChild(s);
        });
        bot.appendChild(tools);
      }
      loadTrace(bot);
    })
    .catch(function(err){ wait.remove(); addMsg('Сеть: ' + err.message, 'bot'); });
}

el('chat-form').addEventListener('submit', function(e){
  e.preventDefault();
  ask(el('question').value);
});

el('add-btn').addEventListener('click', addItem);
renderTabs();
loadProducts();
loadClient(current, true);
</script>
</body>
</html>
`

func (h *handler) demo(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(demoHTML))
}
