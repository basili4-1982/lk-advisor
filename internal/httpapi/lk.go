package httpapi

import (
	"fmt"
	"net/http"
	"strings"
)

// lkClients lists clients that can be opened in the client-facing page.
func (h *handler) lkClients(w http.ResponseWriter, r *http.Request) {
	limit := 50
	var audience, target string
	_ = h.db.QueryRowContext(r.Context(), `
		SELECT audience, to_id FROM experiments
		WHERE status = 'running' ORDER BY created_at DESC LIMIT 1`).Scan(&audience, &target)

	query := `
		SELECT c.client_id, c.client_type, COALESCE(c.industry, ''), count(o.item_id) AS orders
		FROM lk_clients c
		JOIN lk_orders o ON o.client_id = c.client_id
		WHERE 1 = 1`
	args := []any{}
	if audience == "b2b" {
		query += " AND c.client_type IN ('ЮЛ', 'ИП')"
	} else if audience == "b2c" {
		query += " AND c.client_type = 'ФЛ'"
	}
	if target != "" {
		query += ` AND NOT EXISTS (
			SELECT 1 FROM lk_orders x
			WHERE x.client_id = c.client_id AND x.item_id = $1)`
		args = append(args, target)
	}
	query += fmt.Sprintf(" GROUP BY 1, 2, 3 ORDER BY orders DESC, c.client_id LIMIT $%d", len(args)+1)
	args = append(args, limit)

	rows, err := h.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "clients error")
		return
	}
	defer rows.Close()
	type client struct {
		ClientID   string `json:"client_id"`
		ClientType string `json:"client_type"`
		Industry   string `json:"industry"`
		Orders     int    `json:"orders"`
	}
	out := []client{}
	for rows.Next() {
		var c client
		if err := rows.Scan(&c.ClientID, &c.ClientType, &c.Industry, &c.Orders); err != nil {
			writeError(w, http.StatusInternalServerError, "clients scan error")
			return
		}
		out = append(out, c)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": out, "experiment_target": target,
	})
}

// lkPage is a client-facing recommendation block: opening it is a real view.
func (h *handler) lkPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(lkHTML))
}

var lkHTML = strings.TrimSpace(`
<!doctype html>
<html lang="ru">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>Мои рекомендации</title>
<style>
:root{--ink:#0f172a;--muted:#64748b;--line:#e2e8f0;--bg:#f6f7fb;--blue:#2563eb;--green:#047857}
*{box-sizing:border-box}
body{margin:0;background:var(--bg);color:var(--ink);font:16px/1.5 -apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,Arial,sans-serif}
.top{background:linear-gradient(135deg,#070b14 0%,#111827 60%,#1f2937 100%);color:#fff;padding:18px 0}
.wrap{max-width:1080px;margin:0 auto;padding:0 20px}
.top .wrap{display:flex;align-items:center;justify-content:space-between;gap:16px}
.brand{display:flex;align-items:center;gap:12px}
.logo{background:linear-gradient(135deg,#e11d48,#991b1b);color:#fff;font-weight:800;border-radius:11px;width:42px;height:42px;display:grid;place-items:center}
.brand b{display:block;font-size:18px}
.brand small{color:#94a3b8}
main{padding:24px 0 40px}
h1{font-size:24px;margin:0 0 4px}
.lead{color:var(--muted);margin:0 0 18px}
.panel{background:#fff;border:1px solid var(--line);border-radius:14px;padding:18px;box-shadow:0 4px 18px rgba(15,23,42,.05)}
.toolbar{display:flex;gap:10px;align-items:center;flex-wrap:wrap;margin-bottom:14px}
select,input{font:inherit;padding:9px 11px;border:1px solid var(--line);border-radius:9px;background:#fff}
button{font:inherit;font-weight:700;border:0;border-radius:9px;padding:9px 14px;background:var(--blue);color:#fff;cursor:pointer}
button.secondary{background:#eef2ff;color:var(--blue)}
button.green{background:var(--green)}
button:disabled{opacity:.5;cursor:default}
.badge{display:inline-block;padding:4px 9px;border-radius:999px;font-size:12px;font-weight:700;background:#eef2ff;color:var(--blue)}
.badge.green{background:#dcfce7;color:#166534}
.badge.amber{background:#fef3c7;color:#92400e}
.badge.gray{background:#f1f5f9;color:#475569}
.cards{display:grid;gap:14px;margin-top:16px}
.card{border:1px solid var(--line);border-radius:14px;padding:16px}
.card.forced{border-color:#86efac;background:#f0fdf4}
.card-head{display:flex;justify-content:space-between;gap:12px;align-items:flex-start}
.rank{font-size:13px;color:var(--muted);font-weight:700}
.card h2{font-size:18px;margin:6px 0 8px;line-height:1.3}
.reason{color:#334155;margin:0 0 12px}
.meta{color:var(--muted);font-size:13px;margin-bottom:12px}
.actions{display:flex;gap:8px}
.empty{padding:24px;text-align:center;color:var(--muted)}
.trace{color:#94a3b8;font-size:12px;margin-top:12px;word-break:break-all}
.hint{color:var(--muted);font-size:13px}
</style>
</head>
<body>
<header class="top"><div class="wrap">
  <div class="brand"><div class="logo">LK</div><div><b>Мои рекомендации</b><small>личный кабинет клиента</small></div></div>
  <a href="/admin" style="color:#cbd5e1;text-decoration:none">Админка</a>
</div></header>
<main class="wrap">
  <h1>Что ещё может пригодиться</h1>
  <p class="lead">Блок собран по вашим покупкам и подтверждённым связям каталога. Каждое открытие страницы — настоящий показ.</p>
  <div class="panel">
    <div class="toolbar">
      <label class="hint" for="client">Клиент</label>
      <select id="client" style="min-width:340px"></select>
      <button id="reload" class="secondary">Показать блок</button>
      <span id="experiment" class="badge gray">—</span>
      <span id="cold" class="badge gray hidden">—</span>
    </div>
    <div id="cards" class="cards"><div class="empty">Выберите клиента и нажмите «Показать блок»</div></div>
    <div id="trace" class="trace"></div>
  </div>
</main>
<script>
var trace = '', clientID = '';
function esc(s){ return String(s==null?'':s).replace(/[&<>"']/g, function(c){ return {'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]; }); }
function typeLabel(t){
  return {upsell:'Апгрейд', cross_sell:'Дополнение', popular:'Популярное', alternative:'Альтернатива'}[t] || t;
}
async function api(method, path, body){
  var opt = {method:method, headers:{}};
  if (body !== undefined){ opt.headers['Content-Type']='application/json'; opt.body=JSON.stringify(body); }
  var r = await fetch(path, opt);
  var txt = await r.text();
  var data = {}; try { data = txt ? JSON.parse(txt) : {}; } catch(e){ data = {error:txt}; }
  if (!r.ok) throw new Error(data.error || ('HTTP '+r.status));
  return data;
}
async function loadClients(){
  var d = await api('GET','/v1/lk/clients?limit=50');
  var sel = document.getElementById('client');
  sel.innerHTML = (d.items||[]).map(function(c){
    return '<option value="'+esc(c.client_id)+'">'+esc(c.client_id)+' · '+esc(c.client_type)+' · '+esc(c.industry||'без отрасли')+' · покупок '+c.orders+'</option>';
  }).join('');
}
async function loadBlock(){
  var sel = document.getElementById('client');
  clientID = sel.value;
  var box = document.getElementById('cards');
  box.innerHTML = '<div class="empty">Считаю…</div>';
  try {
    var d = await api('POST','/v1/recommendations',{client_id:clientID, limit:3});
    trace = d.trace_id;
    var exp = document.getElementById('experiment');
    if (d.experiment_id){
      exp.textContent = 'Гипотеза: ' + (d.variant || '—');
      exp.className = 'badge ' + (d.variant==='treatment'?'green':'gray');
    } else {
      exp.textContent = 'Без эксперимента'; exp.className = 'badge gray';
    }
    var cold = document.getElementById('cold');
    cold.className = 'badge ' + (d.cold_start?'amber':'gray');
    cold.textContent = d.cold_start ? 'Холодный старт' : 'Есть история';
    var recs = d.recommendations || [];
    if (!recs.length){ box.innerHTML = '<div class="empty">Пока нечего предложить</div>'; return; }
    box.innerHTML = recs.map(function(r){
      var forced = (r.reason||'').indexOf('Гипотеза') === 0;
      return '<div class="card'+(forced?' forced':'')+'" data-product="'+esc(r.product_id)+'">'+
        '<div class="card-head"><div class="rank">#'+r.rank+' · '+esc(typeLabel(r.type))+'</div>'+
        (forced?'<span class="badge green">гипотеза</span>':'')+'</div>'+
        '<h2>'+esc(r.title)+'</h2>'+
        '<p class="reason">'+esc(r.reason)+'</p>'+
        '<div class="meta">product_id: '+esc(r.product_id)+'</div>'+
        '<div class="actions"><button data-act="clicked" class="secondary">Клик</button>'+
        '<button data-act="purchased" class="green">Купить</button></div>'+
      '</div>';
    }).join('');
    document.getElementById('trace').textContent = 'trace_id: ' + trace;
    box.querySelectorAll('button[data-act]').forEach(function(b){
      b.addEventListener('click', function(){ sendEvent(b); });
    });
  } catch(e){ box.innerHTML = '<div class="empty">'+esc(e.message)+'</div>'; }
}
async function sendEvent(btn){
  var card = btn.closest('.card');
  var product = card.getAttribute('data-product');
  var act = btn.getAttribute('data-act');
  try {
    await api('POST','/v1/events',{trace_id:trace, client_id:clientID, product_id:product, event:act});
    card.querySelectorAll('button').forEach(function(b){ b.disabled = true; });
    var mark = document.createElement('div');
    mark.className = 'hint';
    mark.style.marginTop = '8px';
    mark.textContent = act === 'purchased' ? 'Покупка записана' : 'Клик записан';
    card.appendChild(mark);
  } catch(e){ alert(e.message); }
}
document.getElementById('reload').addEventListener('click', loadBlock);
document.getElementById('client').addEventListener('change', loadBlock);
loadClients().then(loadBlock).catch(function(e){ document.getElementById('cards').innerHTML = '<div class="empty">'+esc(e.message)+'</div>'; });
</script>
</body>
</html>
`)
