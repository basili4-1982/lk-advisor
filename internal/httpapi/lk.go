package httpapi

import (
	"fmt"
	"net/http"
	"strings"
)

// lkClients lists clients that can be opened in the client-facing page.
type lkClient struct {
	ClientID   string `json:"client_id"`
	ClientType string `json:"client_type"`
	Industry   string `json:"industry"`
	Orders     int    `json:"orders"`
	ColdStart  bool   `json:"cold_start"`
}

func (h *handler) lkClients(w http.ResponseWriter, r *http.Request) {
	var audience, target string
	_ = h.db.QueryRowContext(r.Context(), `
		SELECT audience, to_id FROM experiments
		WHERE status = 'running' ORDER BY created_at DESC LIMIT 1`).Scan(&audience, &target)

	out := []lkClient{}
	audienceSQL := ""
	if audience == "b2b" {
		audienceSQL = " AND c.client_type IN ('ЮЛ', 'ИП')"
	} else if audience == "b2c" {
		audienceSQL = " AND c.client_type = 'ФЛ'"
	}

	// 40 clients with purchase history.
	warmQuery := `
		SELECT c.client_id, c.client_type, COALESCE(c.industry, ''), count(o.item_id) AS orders
		FROM lk_clients c
		JOIN lk_orders o ON o.client_id = c.client_id
		WHERE 1 = 1` + audienceSQL
	warmArgs := []any{}
	if target != "" {
		warmQuery += ` AND NOT EXISTS (
			SELECT 1 FROM lk_orders x
			WHERE x.client_id = c.client_id AND x.item_id = $1)`
		warmArgs = append(warmArgs, target)
	}
	warmQuery += fmt.Sprintf(" GROUP BY 1, 2, 3 ORDER BY orders DESC, c.client_id LIMIT $%d", len(warmArgs)+1)
	warmArgs = append(warmArgs, 40)
	if err := appendClients(r, h, warmQuery, warmArgs, &out); err != nil {
		writeError(w, http.StatusInternalServerError, "clients error")
		return
	}

	// 10 clients without history, so cold start is always demonstrable.
	coldQuery := `
		SELECT c.client_id, c.client_type, COALESCE(c.industry, ''), 0
		FROM lk_clients c
		WHERE NOT EXISTS (SELECT 1 FROM lk_orders x WHERE x.client_id = c.client_id)` + audienceSQL + `
		ORDER BY c.client_id LIMIT $1`
	if err := appendClients(r, h, coldQuery, []any{10}, &out); err != nil {
		writeError(w, http.StatusInternalServerError, "clients error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items": out, "experiment_target": target,
	})
}

func appendClients(r *http.Request, h *handler, query string, args []any, out *[]lkClient) error {
	rows, err := h.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var c lkClient
		if err := rows.Scan(&c.ClientID, &c.ClientType, &c.Industry, &c.Orders); err != nil {
			return err
		}
		c.ColdStart = c.Orders == 0
		*out = append(*out, c)
	}
	return rows.Err()
}

// lkPage renders the client block. The technical context (variant, ids,
// relation internals) is hidden until the operator turns on system info.
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
.cards{display:grid;gap:14px;margin-top:16px}
.card{border:1px solid var(--line);border-radius:14px;padding:16px}
.card-head{display:flex;justify-content:space-between;gap:12px;align-items:flex-start}
.rank{font-size:13px;color:var(--muted);font-weight:700}
.card h2{font-size:18px;margin:6px 0 8px;line-height:1.3}
.reason{color:#334155;margin:0 0 12px}
.actions{display:flex;gap:8px}
.empty{padding:24px;text-align:center;color:var(--muted)}
.hint{color:var(--muted);font-size:13px}

/* Всё системное скрыто, пока оператор не включил режим. */
.sys-panel{display:none;background:#0f172a;color:#e5e7eb;border-radius:12px;padding:14px;margin:0 0 16px;font:13px/1.55 ui-monospace,SFMono-Regular,Menlo,monospace}
body.sys-on .sys-panel{display:block}
.sys-panel b{color:#93c5fd;font-weight:700}
.sys-line{margin:2px 0}
.sys{display:none}
body.sys-on .sys{display:block}
.sys-card{margin-top:12px;padding-top:10px;border-top:1px dashed #cbd5e1;color:#475569;font:12px/1.5 ui-monospace,SFMono-Regular,Menlo,monospace}
.sys-card div{margin:2px 0}
body.sys-on .card.forced{background:#f8fafc;border-color:#cbd5e1}
</style>
</head>
<body>
<header class="top"><div class="wrap">
  <div class="brand"><div class="logo">LK</div><div><b>Мои рекомендации</b><small>личный кабинет клиента</small></div></div>
  <span class="hint" style="color:#94a3b8">подобрано по вашим покупкам</span>
</div></header>
<main class="wrap">
  <h1>Что ещё может пригодиться</h1>
  <p class="lead">Подборка по вашим покупкам и задачам.</p>
  <div class="panel">
    <div class="toolbar" style="justify-content:flex-end;margin-bottom:0">
      <button id="sys-toggle" class="secondary">Системная информация</button>
    </div>
    <div id="system" class="sys-panel">
      <div style="display:flex;gap:10px;align-items:center;flex-wrap:wrap;margin-bottom:12px">
        <label for="client" style="color:#94a3b8">клиент</label>
        <select id="client" style="min-width:340px;background:#1e293b;color:#e5e7eb;border-color:#334155"></select>
        <button id="reload" class="secondary">Показать блок</button>
      </div>
      <div id="sys-meta"></div>
    </div>
    <div id="cards" class="cards"><div class="empty">Выберите клиента и нажмите «Показать блок»</div></div>
  </div>
</main>
<script>
var trace = '', clientID = '', lastData = null, sysOn = false;
function esc(s){ return String(s==null?'':s).replace(/[&<>"']/g, function(c){ return {'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]; }); }
function typeLabel(t){
  return {upsell:'Апгрейд', cross_sell:'Дополнение', popular:'Популярное', alternative:'Альтернатива'}[t] || t;
}
function clientReason(r){
  if (r.type === 'upsell') return 'Более широкая редакция вместо текущей';
  if (r.type === 'cross_sell') return 'Дополняет то, что вы уже купили';
  if (r.type === 'popular') return 'Часто выбирают клиенты с похожими задачами';
  if (r.type === 'alternative') return 'Альтернатива вашей текущей покупке';
  return 'Может пригодиться вместе с вашей покупкой';
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
    return '<option value="'+esc(c.client_id)+'">'+esc(c.client_id)+' · '+esc(c.client_type)+' · '+esc(c.industry||'без отрасли')+' · '+(c.cold_start?'без покупок':'покупок '+c.orders)+'</option>';
  }).join('');
}
function renderSystem(d){
  var rows = [];
  rows.push('<div class="sys-line"><b>client_id:</b> '+esc(d.client_id)+'</div>');
  rows.push('<div class="sys-line"><b>strategy:</b> '+esc(d.strategy || '—')+'</div>');
  rows.push('<div class="sys-line"><b>cold_start:</b> '+(d.cold_start?'true':'false')+'</div>');
  rows.push('<div class="sys-line"><b>experiment_id:</b> '+esc(d.experiment_id || '—')+'</div>');
  rows.push('<div class="sys-line"><b>variant:</b> '+esc(d.variant || '—')+'</div>');
  rows.push('<div class="sys-line"><b>trace_id:</b> '+esc(d.trace_id || '—')+'</div>');
  rows.push('<div class="sys-line" style="margin-top:8px;color:#94a3b8">Каждое открытие страницы — один настоящий показ</div>');
  rows.push('<div class="sys-line"><a href="/admin" style="color:#93c5fd">Перейти в админку</a></div>');
  document.getElementById('sys-meta').innerHTML = rows.join('');
}
async function loadBlock(){
  var sel = document.getElementById('client');
  clientID = sel.value;
  var box = document.getElementById('cards');
  box.innerHTML = '<div class="empty">Считаю…</div>';
  try {
    var d = await api('POST','/v1/recommendations',{client_id:clientID, limit:3});
    lastData = d;
    trace = d.trace_id;
    renderSystem(d);
    var recs = d.recommendations || [];
    if (!recs.length){ box.innerHTML = '<div class="empty">Пока нечего предложить</div>'; return; }
    box.innerHTML = recs.map(function(r){
      var forced = (r.reason||'').indexOf('Гипотеза') === 0;
      return '<div class="card'+(forced?' forced':'')+'" data-product="'+esc(r.product_id)+'">'+
        '<div class="card-head"><div class="rank">#'+r.rank+' · '+esc(typeLabel(r.type))+'</div></div>'+
        '<h2>'+esc(r.title)+'</h2>'+
        '<p class="reason">'+esc(clientReason(r))+'</p>'+
        '<div class="actions"><button data-act="clicked" class="secondary">Клик</button>'+
        '<button data-act="purchased" class="green">Купить</button></div>'+
        '<div class="sys sys-card">'+
          '<div><b>product_id:</b> '+esc(r.product_id)+'</div>'+
          '<div><b>relation_type:</b> '+esc(r.relation_type || '—')+'</div>'+
          '<div><b>forced_by_experiment:</b> '+(forced?'true':'false')+'</div>'+
          '<div><b>source_product_id:</b> '+esc(r.source_product_id || '—')+'</div>'+
          '<div><b>score:</b> '+Number(r.score||0).toFixed(3)+'</div>'+
          '<div><b>system_reason:</b> '+esc(r.reason || '—')+'</div>'+
        '</div>'+
      '</div>';
    }).join('');
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
document.getElementById('sys-toggle').addEventListener('click', function(){
  sysOn = !sysOn;
  document.body.classList.toggle('sys-on', sysOn);
  this.textContent = sysOn ? 'Скрыть системную информацию' : 'Системная информация';
});
document.getElementById('reload').addEventListener('click', loadBlock);
document.getElementById('client').addEventListener('change', loadBlock);
loadClients().then(loadBlock).catch(function(e){ document.getElementById('cards').innerHTML = '<div class="empty">'+esc(e.message)+'</div>'; });
</script>
</body>
</html>
`)
