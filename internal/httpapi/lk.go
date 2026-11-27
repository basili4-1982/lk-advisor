package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
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

	industries := []string{}
	indRows, err := h.db.QueryContext(r.Context(), `
		SELECT DISTINCT industry FROM lk_clients
		WHERE industry <> '' ORDER BY industry`)
	if err == nil {
		defer indRows.Close()
		for indRows.Next() {
			var name string
			if indRows.Scan(&name) == nil {
				industries = append(industries, name)
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": out, "industries": industries, "experiment_target": target,
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

// lkCreateClient adds a demo client profile from the storefront. It starts
// without purchase history, so it is a valid cold-start example.
func (h *handler) lkCreateClient(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ClientID    string `json:"client_id"`
		ClientType  string `json:"client_type"`
		Industry    string `json:"industry"`
		Segment     string `json:"segment"`
		ClientGroup string `json:"client_group"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	req.ClientType = strings.ToUpper(strings.TrimSpace(req.ClientType))
	switch req.ClientType {
	case "ЮЛ", "ИП", "ФЛ":
	default:
		writeError(w, http.StatusBadRequest, "client_type должен быть ЮЛ, ИП или ФЛ")
		return
	}
	req.ClientID = strings.TrimSpace(req.ClientID)
	if req.ClientID == "" {
		req.ClientID = "DEMO-" + time.Now().UTC().Format("20060102150405")
	}
	var exists int
	if err := h.db.QueryRowContext(r.Context(), `
		SELECT count(*) FROM lk_clients WHERE client_id = $1`, req.ClientID).Scan(&exists); err != nil {
		writeError(w, http.StatusInternalServerError, "client check error")
		return
	}
	if exists > 0 {
		writeError(w, http.StatusConflict, "клиент с таким ID уже существует")
		return
	}
	if _, err := h.db.ExecContext(r.Context(), `
		INSERT INTO lk_clients (client_id, client_type, industry, segment, client_group, source)
		VALUES ($1, $2, $3, $4, $5, 'lk_demo')`,
		req.ClientID, req.ClientType, strings.TrimSpace(req.Industry),
		strings.TrimSpace(req.Segment), strings.TrimSpace(req.ClientGroup)); err != nil {
		writeError(w, http.StatusInternalServerError, "client create error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"client_id": req.ClientID, "client_type": req.ClientType,
		"industry": req.Industry, "segment": req.Segment,
		"client_group": req.ClientGroup, "source": "lk_demo", "orders": 0,
	})
}

// lkClientDetail returns the client profile and purchase history behind the
// storefront block, so the operator can verify why recommendations appeared.
func (h *handler) lkClientDetail(w http.ResponseWriter, r *http.Request) {
	clientID := r.PathValue("id")
	var c struct {
		ClientID    string `json:"client_id"`
		ClientType  string `json:"client_type"`
		Industry    string `json:"industry"`
		Segment     string `json:"segment"`
		ClientGroup string `json:"client_group"`
		Source      string `json:"source"`
		Orders      int    `json:"orders"`
		Items       []struct {
			ItemID       string `json:"item_id"`
			Title        string `json:"title"`
			Category     string `json:"category"`
			ProductGroup string `json:"product_group"`
			Purchases    int    `json:"purchases"`
			LastOrder    string `json:"last_order"`
		} `json:"items"`
	}
	err := h.db.QueryRowContext(r.Context(), `
		SELECT client_id, client_type, COALESCE(industry, ''), COALESCE(segment, ''),
		       COALESCE(client_group, ''), COALESCE(source, '')
		FROM lk_clients WHERE client_id = $1`, clientID).Scan(
		&c.ClientID, &c.ClientType, &c.Industry, &c.Segment, &c.ClientGroup, &c.Source)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "client not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "client profile error")
		return
	}
	if err := h.db.QueryRowContext(r.Context(), `
		SELECT count(*) FROM lk_orders WHERE client_id = $1`, clientID).Scan(&c.Orders); err != nil {
		writeError(w, http.StatusInternalServerError, "client orders error")
		return
	}
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT i.item_id, i.title, i.category, COALESCE(i.product_group, ''),
		       count(*) AS purchases, to_char(max(o.order_week), 'YYYY-MM-DD')
		FROM lk_orders o
		JOIN lk_items i ON i.item_id = o.item_id
		WHERE o.client_id = $1
		GROUP BY 1, 2, 3, 4
		ORDER BY max(o.order_week) DESC, i.title
		LIMIT 60`, clientID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "client history error")
		return
	}
	defer rows.Close()
	c.Items = []struct {
		ItemID       string `json:"item_id"`
		Title        string `json:"title"`
		Category     string `json:"category"`
		ProductGroup string `json:"product_group"`
		Purchases    int    `json:"purchases"`
		LastOrder    string `json:"last_order"`
	}{}
	for rows.Next() {
		var it struct {
			ItemID       string `json:"item_id"`
			Title        string `json:"title"`
			Category     string `json:"category"`
			ProductGroup string `json:"product_group"`
			Purchases    int    `json:"purchases"`
			LastOrder    string `json:"last_order"`
		}
		if err := rows.Scan(&it.ItemID, &it.Title, &it.Category, &it.ProductGroup,
			&it.Purchases, &it.LastOrder); err != nil {
			writeError(w, http.StatusInternalServerError, "client history scan error")
			return
		}
		c.Items = append(c.Items, it)
	}
	writeJSON(w, http.StatusOK, c)
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
.top nav{display:flex;align-items:center;gap:18px}
.top nav a{color:#cbd5e1;text-decoration:none;font-size:14px}
.top nav a:hover{color:#fff}
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
.hidden{display:none!important}
.badge{display:inline-block;padding:3px 9px;border-radius:999px;font-size:12px;font-weight:700;background:#eef2ff;color:var(--blue)}
.badge.green{background:#dcfce7;color:#166534}

/* Всё системное скрыто, пока оператор не включил режим. */
.sys-panel{display:none;background:#0f172a;color:#e5e7eb;border-radius:12px;padding:14px;margin:0 0 16px;font:13px/1.55 ui-monospace,SFMono-Regular,Menlo,monospace}
body.sys-on .sys-panel{display:block}
.sys-panel b{color:#93c5fd;font-weight:700}
.sys-line{margin:2px 0}
.sys{display:none}
body.sys-on .sys{display:block}
.sys-card{margin-top:12px;padding-top:10px;border-top:1px dashed #cbd5e1;color:#475569;font:12px/1.5 ui-monospace,SFMono-Regular,Menlo,monospace}
.sys-card div{margin:2px 0}
.sys-table{width:100%;border-collapse:collapse;margin-top:8px;font:12px/1.45 ui-monospace,SFMono-Regular,Menlo,monospace;color:#cbd5e1}
.sys-table th{text-align:left;color:#93c5fd;border-bottom:1px solid #334155;padding:4px 6px;font-weight:700}
.sys-table td{padding:4px 6px;border-bottom:1px solid #1e293b;vertical-align:top}
.sys-title{font-size:14px;font-weight:800;color:#93c5fd;margin-bottom:10px}
.sys-panel label{display:grid;gap:4px;font-size:12px;color:#94a3b8}
.sys-panel input,.sys-panel select{background:#1e293b;color:#e5e7eb;border-color:#334155}
.section-label{margin:18px 0 -4px;color:#64748b;font-size:12px;font-weight:800;text-transform:uppercase;letter-spacing:.05em}
body.sys-on .card.forced{background:#f8fafc;border-color:#cbd5e1}
</style>
</head>
<body>
<header class="top"><div class="wrap">
  <div class="brand"><div class="logo">LK</div><div><b>Мои рекомендации</b><small>личный кабинет клиента</small></div></div>
  <nav><a href="/">Главная</a><a href="/admin">Админка</a><a href="/demo/graph">Граф</a><a href="/api">API</a></nav>
</div></header>
<main class="wrap">
  <h1>Что ещё может пригодиться</h1>
  <p class="lead">Подборка по вашим покупкам и задачам.</p>
  <div class="panel">
    <div class="toolbar" style="justify-content:flex-end;margin-bottom:0">
      <button id="sys-toggle" class="secondary">Показать системную информацию</button>
    </div>
    <div id="system" class="sys-panel">
      <div class="sys-title">Системная информация</div>
      <div style="display:flex;gap:10px;align-items:center;flex-wrap:wrap;margin-bottom:12px">
        <label for="client" style="color:#94a3b8">клиент</label>
        <select id="client" style="min-width:340px"></select>
        <button id="reload" class="secondary">Показать блок</button>
        <label style="display:flex;align-items:center;gap:6px;font-size:13px;color:#94a3b8;white-space:nowrap"><input type="checkbox" id="certified-only" style="width:auto"> только сертифицированные</label>
        <span id="certified-policy-hint" class="hint" style="color:#fbbf24"></span>
        <button id="new-client-toggle" class="secondary">＋ Создать клиента</button>
      </div>
      <div id="new-client" class="hidden" style="display:grid;gap:8px;margin-bottom:12px;padding:12px;border:1px solid #334155;border-radius:10px">
        <div style="display:grid;grid-template-columns:1fr 1fr 1fr;gap:8px">
          <label>ID (необязательно)<input id="new-client-id" placeholder="авто"></label>
          <label>Тип<select id="new-client-type"><option value="ЮЛ">ЮЛ</option><option value="ИП">ИП</option><option value="ФЛ">ФЛ</option></select></label>
          <label>Отрасль<select id="new-client-industry"></select></label>
        </div>
        <div style="display:grid;grid-template-columns:1fr 1fr;gap:8px">
          <label>Сегмент<select id="new-client-segment"><option value="">—</option><option value="small">small</option><option value="medium">medium</option><option value="corporate">corporate</option><option value="enterprise">enterprise</option></select></label>
          <label>Группа<input id="new-client-group" placeholder="необязательно"></label>
        </div>
        <div style="display:flex;gap:8px;align-items:center">
          <button id="new-client-save">Создать</button>
          <button id="new-client-cancel" class="secondary">Отмена</button>
          <span id="new-client-error" style="color:#fbbf24;font-size:12px"></span>
        </div>
      </div>
      <div id="sys-meta"></div>
      <div id="sys-client"></div>
    </div>
    <div class="sys section-label">Ниже — то, что видит клиент</div>
    <div id="cards" class="cards"><div class="empty">Выберите клиента и нажмите «Показать блок»</div></div>
  </div>
</main>
<script>
var trace = '', clientID = '', lastData = null, sysOn = false, LK_INDUSTRIES = [], LK_CLIENTS = {}, certifiedTouched = false;
var REGULATED_INDUSTRIES = {
  'Медицинские организации': true, 'Образование и наука': true,
  'Органы власти': true, 'Финансы (банки, страхование)': true
};
function applyCertifiedPolicy(){
  var cid = document.getElementById('client').value;
  var industry = (LK_CLIENTS[cid] || {}).industry || '';
  var regulated = !!REGULATED_INDUSTRIES[industry];
  if (!certifiedTouched) document.getElementById('certified-only').checked = regulated;
  document.getElementById('certified-policy-hint').textContent = regulated
    ? 'Для отрасли «'+industry+'» по умолчанию только сертифицированные'
    : '';
}
function esc(s){ return String(s==null?'':s).replace(/[&<>"']/g, function(c){ return {'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]; }); }
function typeLabel(t){
  return {upsell:'Апгрейд', cross_sell:'Дополнение', popular:'Популярное', alternative:'Альтернатива', personal:'Подборка'}[t] || t;
}
function clientReason(r){
  if (r.type === 'upsell') return 'Более широкая редакция вместо текущей';
  if (r.type === 'cross_sell') return 'Дополняет то, что вы уже купили';
  if (r.type === 'popular') return 'Часто выбирают клиенты с похожими задачами';
  if (r.type === 'alternative') return 'Альтернатива вашей текущей покупке';
  if (r.type === 'personal') return 'На основе ваших покупок';
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
  LK_INDUSTRIES = d.industries || [];
  var sel = document.getElementById('client');
  LK_CLIENTS = {};
  (d.items||[]).forEach(function(c){ LK_CLIENTS[c.client_id] = c; });
  sel.innerHTML = (d.items||[]).map(function(c){
    return '<option value="'+esc(c.client_id)+'">'+esc(c.client_id)+' · '+esc(c.client_type)+' · '+esc(c.industry||'без отрасли')+' · '+(c.cold_start?'без покупок':'покупок '+c.orders)+'</option>';
  }).join('');
  var ind = document.getElementById('new-client-industry');
  ind.innerHTML = '<option value="">— не указана —</option>'+LK_INDUSTRIES.map(function(x){ return '<option>'+esc(x)+'</option>'; }).join('');
  var wanted = new URLSearchParams(location.search).get('client');
  if (wanted && LK_CLIENTS[wanted]) sel.value = wanted;
}
function renderSystem(d){
  var rows = [];
  rows.push('<div class="sys-line"><b>client_id:</b> '+esc(d.client_id)+'</div>');
  rows.push('<div class="sys-line"><b>strategy:</b> '+esc(d.strategy || '—')+'</div>');
  rows.push('<div class="sys-line"><b>cold_start:</b> '+(d.cold_start?'true':'false')+'</div>');
  rows.push('<div class="sys-line"><b>experiment_id:</b> '+esc(d.experiment_id || '—')+'</div>');
  rows.push('<div class="sys-line"><b>variant:</b> '+esc(d.variant || '—')+'</div>');
  rows.push('<div class="sys-line"><b>trace_id:</b> '+esc(d.trace_id || '—')+'</div>');
  if (String(d.client_id||'').indexOf('COLD-') === 0) {
    rows.push('<div class="sys-line" style="color:#fbbf24">COLD-* — тестовый профиль без истории для холодного старта</div>');
  }
  rows.push('<div class="sys-line" style="margin-top:8px;color:#94a3b8">Каждое открытие страницы — один настоящий показ</div>');
  rows.push('<div class="sys-line"><a href="/admin" style="color:#93c5fd">Перейти в админку</a></div>');
  document.getElementById('sys-meta').innerHTML = rows.join('');
}
async function loadClientContext(d){
  var host = document.getElementById('sys-client');
  host.innerHTML = '<div class="sys-line" style="color:#94a3b8">История загружается…</div>';
  try {
    var c = await api('GET','/v1/lk/clients/'+encodeURIComponent(clientID));
    var sources = {};
    (d.recommendations||[]).forEach(function(r){ if (r.source_product_id) sources[r.source_product_id] = true; });
    var rows = (c.items||[]).map(function(it){
      return '<tr><td>'+esc(it.title)+'</td><td>'+esc(it.category||'—')+'</td><td>'+esc(it.last_order||'—')+'</td><td>'+it.purchases+'</td><td>'+(sources[it.item_id]?'источник блока':'')+'</td></tr>';
    }).join('');
    host.innerHTML =
      '<div class="sys-line" style="margin-top:12px"><b>Профиль:</b> '+esc(c.client_type||'—')+' · '+esc(c.industry||'без отрасли')+' · сегмент '+esc(c.segment||'—')+' · группа '+esc(c.client_group||'—')+' · источник '+esc(c.source||'—')+'</div>'+
      '<div class="sys-line"><b>История:</b> '+c.orders+' заказов, '+(c.items||[]).length+' товаров</div>'+
      (rows ? '<table class="sys-table"><thead><tr><th>Купленный товар</th><th>Категория</th><th>Последний заказ</th><th>Раз</th><th></th></tr></thead><tbody>'+rows+'</tbody></table>' : '<div class="sys-line">Покупок нет — холодный старт</div>');
  } catch(e){ host.innerHTML = '<div class="sys-line" style="color:#fbbf24">'+esc(e.message)+'</div>'; }
}
async function loadBlock(){
  var sel = document.getElementById('client');
  clientID = sel.value;
  var box = document.getElementById('cards');
  document.getElementById('sys-client').innerHTML = '';
  box.innerHTML = '<div class="empty">Считаю…</div>';
  try {
    var d = await api('POST','/v1/recommendations',{client_id:clientID, limit:3, certified_only: document.getElementById('certified-only').checked});
    lastData = d;
    trace = d.trace_id;
    renderSystem(d);
    var recs = d.recommendations || [];
    if (!recs.length){ box.innerHTML = '<div class="empty">Пока нечего предложить</div>'; return; }
    box.innerHTML = recs.map(function(r){
      var forced = (r.reason||'').indexOf('Гипотеза') === 0;
      var certs = r.fstec || [];
      var validCert = certs.some(function(c){ return !c.expired; });
      var certClaim = /сертифицирован|ФСТЭК/i.test(r.title || '');
      var certBadge = validCert ? '<span class="badge green">Сертифицировано ФСТЭК</span>'
        : (certClaim && r.cert_expired ? '<span class="badge" style="background:#fef3c7;color:#92400e">Сертификат истёк</span>' : '');
      var certSys = certs.length ? certs.map(function(c){
        return '<div><b>fstec:</b> №'+esc(c.number)+' · '+(c.expired?'<span style="color:#fbbf24">просрочен</span>':'до '+esc(c.valid_until||'—'))+'</div>';
      }).join('') : '<div><b>fstec:</b> нет</div>';
      return '<div class="card'+(forced?' forced':'')+'" data-product="'+esc(r.product_id)+'">'+
        '<div class="card-head"><div class="rank">#'+r.rank+' · '+esc(typeLabel(r.type))+'</div>'+
          certBadge+'</div>'+
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
          '<div><b>compatibility:</b> '+esc(r.compatibility || '—')+'</div>'+
          certSys+
          '<div><b>system_reason:</b> '+esc(r.reason || '—')+'</div>'+
        '</div>'+
      '</div>';
    }).join('');
    box.querySelectorAll('button[data-act]').forEach(function(b){
      b.addEventListener('click', function(){ sendEvent(b); });
    });
    loadClientContext(d);
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
document.getElementById('new-client-toggle').addEventListener('click', function(){
  document.getElementById('new-client').classList.toggle('hidden');
  document.getElementById('new-client-error').textContent = '';
});
document.getElementById('new-client-cancel').addEventListener('click', function(){
  document.getElementById('new-client').classList.add('hidden');
});
document.getElementById('new-client-save').addEventListener('click', async function(){
  var err = document.getElementById('new-client-error');
  err.textContent = '';
  var body = {
    client_id: document.getElementById('new-client-id').value.trim(),
    client_type: document.getElementById('new-client-type').value,
    industry: document.getElementById('new-client-industry').value,
    segment: document.getElementById('new-client-segment').value,
    client_group: document.getElementById('new-client-group').value.trim()
  };
  try {
    var created = await api('POST','/v1/lk/clients', body);
    await loadClients();
    var sel = document.getElementById('client');
    sel.value = created.client_id;
    if (!sel.value) {
      var o = document.createElement('option');
      o.value = created.client_id;
      o.textContent = created.client_id + ' · ' + created.client_type + ' · ' + (created.industry || 'без отрасли') + ' · без покупок';
      sel.insertBefore(o, sel.firstChild);
      sel.value = created.client_id;
    }
    document.getElementById('new-client').classList.add('hidden');
    document.getElementById('new-client-id').value = '';
    loadBlock();
  } catch(e){ err.textContent = e.message; }
});
document.getElementById('sys-toggle').addEventListener('click', function(){
  sysOn = !sysOn;
  document.body.classList.toggle('sys-on', sysOn);
  this.textContent = sysOn ? 'Скрыть системную информацию' : 'Системная информация';
});
document.getElementById('reload').addEventListener('click', loadBlock);
document.getElementById('certified-only').addEventListener('change', function(){ certifiedTouched = true; loadBlock(); });
document.getElementById('client').addEventListener('change', function(){ certifiedTouched = false; applyCertifiedPolicy(); loadBlock(); });
loadClients().then(function(){ applyCertifiedPolicy(); return loadBlock(); }).catch(function(e){ document.getElementById('cards').innerHTML = '<div class="empty">'+esc(e.message)+'</div>'; });
</script>
</body>
</html>
`)
