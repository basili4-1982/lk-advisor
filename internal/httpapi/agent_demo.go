package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/basili4-1982/lk-advisor/internal/catalog"
)

type agentEvidence struct {
	Kind    string `json:"kind"`
	Label   string `json:"label"`
	Summary string `json:"summary"`
	URL     string `json:"url,omitempty"`
}

type agentColdItem struct {
	Title  string `json:"title"`
	Type   string `json:"type"`
	Reason string `json:"reason"`
}

type agentDemo struct {
	FromID     string          `json:"from_id"`
	ToID       string          `json:"to_id"`
	FromTitle  string          `json:"from_title"`
	ToTitle    string          `json:"to_title"`
	Type       string          `json:"type"`
	Confidence float64         `json:"confidence"`
	Status     string          `json:"status"`
	Reason     string          `json:"reason"`
	Evidence   []agentEvidence `json:"evidence"`
	ColdStart  struct {
		ClientID  string          `json:"client_id"`
		Industry  string          `json:"industry"`
		Approach  string          `json:"approach"`
		ColdStart bool            `json:"cold_start"`
		Items     []agentColdItem `json:"items"`
	} `json:"cold_start"`
}

var evidenceLabels = map[string]string{
	"vendor_doc":   "Документация вендора",
	"product_page": "Страница товара",
	"attribute":    "Атрибут каталога",
	"co_purchase":  "Совместные покупки",
	"manual":       "Ручное подтверждение",
	"llm":          "Предложение модели",
	"spec":         "Спецификация",
	"rule":         "Правило",
}

// agentDemo returns the evidence and cold-start decision for one relation.
func (h *handler) agentDemo(w http.ResponseWriter, r *http.Request) {
	fromID := strings.TrimSpace(r.URL.Query().Get("from"))
	toID := strings.TrimSpace(r.URL.Query().Get("to"))
	if fromID == "" || toID == "" {
		fromID = "GAPPS-BUS-STD-1USER-1MO-First-Year"
		toID = "GAPPS-STARTER-1USER-1MO-First-Year"
	}
	var out agentDemo
	var evidenceRaw string
	err := h.db.QueryRowContext(r.Context(), `
		SELECT r.from_item_id, r.to_item_id, r.type, r.confidence::float8,
		       r.status, r.reason, r.evidence::text,
		       COALESCE(f.title, ''), COALESCE(t.title, '')
		FROM relations r
		LEFT JOIN lk_items f ON f.item_id = r.from_item_id
		LEFT JOIN lk_items t ON t.item_id = r.to_item_id
		WHERE r.from_item_id = $1 AND r.to_item_id = $2
		ORDER BY CASE r.status WHEN 'active' THEN 0 WHEN 'confirmed' THEN 0 ELSE 1 END
		LIMIT 1`, fromID, toID).Scan(
		&out.FromID, &out.ToID, &out.Type, &out.Confidence,
		&out.Status, &out.Reason, &evidenceRaw, &out.FromTitle, &out.ToTitle)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "relation not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "relation error")
		return
	}

	var raw []struct {
		Kind    string         `json:"kind"`
		Summary string         `json:"summary"`
		Details map[string]any `json:"details"`
	}
	if err := json.Unmarshal([]byte(evidenceRaw), &raw); err == nil {
		for _, e := range raw {
			item := agentEvidence{
				Kind:    e.Kind,
				Label:   evidenceLabels[e.Kind],
				Summary: e.Summary,
			}
			if item.Label == "" {
				item.Label = e.Kind
			}
			if e.Details != nil {
				if u, ok := e.Details["url"].(string); ok {
					item.URL = u
				}
			}
			out.Evidence = append(out.Evidence, item)
		}
	}

	// Cold start: an explicitly history-less profile gets popular items from its
	// industry and a plain explanation, without any personal history.
	const coldClient = "COLD-CLINIC"
	if _, industry, ok, _ := catalog.ClientAudience(r.Context(), h.db, coldClient); ok {
		out.ColdStart.ClientID = coldClient
		out.ColdStart.Industry = industry
		out.ColdStart.Approach = "Истории нет: популярные товары отрасли и совпадение по содержанию"
		if served, err := catalog.Serve(r.Context(), h.db, catalog.ServeOptions{
			ClientID: coldClient, Limit: 3,
		}); err == nil {
			out.ColdStart.ColdStart = served.ColdStart
			for _, it := range served.Items {
				out.ColdStart.Items = append(out.ColdStart.Items, agentColdItem{
					Title: it.Title, Type: it.Type, Reason: it.Reason,
				})
			}
		}
	}

	writeJSON(w, http.StatusOK, out)
}

// agentPage renders the "how the agent works" explainer.
func (h *handler) agentPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(agentHTML))
}

var agentHTML = strings.TrimSpace(`
<!doctype html>
<html lang="ru">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>Как агент находит связи</title>
<style>
:root{--ink:#0f172a;--muted:#64748b;--line:#e2e8f0;--bg:#f6f7fb;--blue:#2563eb;--green:#047857}
*{box-sizing:border-box}
body{margin:0;background:var(--bg);color:var(--ink);font:16px/1.5 -apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,Arial,sans-serif}
.top{background:linear-gradient(135deg,#070b14 0%,#111827 60%,#1f2937 100%);color:#fff;padding:20px 0}
.wrap{max-width:1100px;margin:0 auto;padding:0 20px}
.top .wrap{display:flex;align-items:center;justify-content:space-between;gap:16px}
.brand{display:flex;align-items:center;gap:12px}
.logo{background:linear-gradient(135deg,#e11d48,#991b1b);color:#fff;font-weight:800;border-radius:11px;width:44px;height:44px;display:grid;place-items:center}
.brand b{display:block;font-size:19px}
.brand small{color:#94a3b8}
main{padding:28px 0 48px}
h1{font-size:26px;margin:0 0 6px}
.lead{color:var(--muted);margin:0 0 20px;max-width:820px}
.toolbar{display:flex;gap:10px;flex-wrap:wrap;margin-bottom:18px}
button{font:inherit;font-weight:700;border:0;border-radius:9px;padding:9px 14px;background:#eef2ff;color:var(--blue);cursor:pointer}
button.on{background:var(--blue);color:#fff}
.steps{display:grid;gap:14px}
.step{background:#fff;border:1px solid var(--line);border-radius:14px;padding:18px;box-shadow:0 4px 18px rgba(15,23,42,.05)}
.step h2{font-size:17px;margin:0 0 4px}
.step .sub{color:var(--muted);font-size:13px;margin-bottom:12px}
.pair{font-size:18px;font-weight:800;margin:4px 0 8px}
.badge{display:inline-block;padding:3px 9px;border-radius:999px;font-size:12px;font-weight:700;background:#eef2ff;color:var(--blue);margin-right:6px}
.badge.green{background:#dcfce7;color:#166534}
.badge.amber{background:#fef3c7;color:#92400e}
.ev{display:grid;gap:8px}
.ev-row{display:grid;grid-template-columns:190px 1fr;gap:10px;padding:8px 0;border-bottom:1px solid #f1f5f9}
.ev-row:last-child{border-bottom:0}
.ev-kind{font-size:12px;font-weight:800;color:var(--muted);text-transform:uppercase;letter-spacing:.03em}
.ev-row a{color:var(--blue);text-decoration:none}
.ev-row a:hover{text-decoration:underline}
.cold{display:grid;gap:10px}
.cold-item{border:1px solid var(--line);border-radius:10px;padding:10px 12px}
.cold-item b{display:block}
.cold-item small{color:var(--muted)}
.empty{color:var(--muted)}
.foot{margin-top:20px;color:#94a3b8;font-size:13px}
@media(max-width:760px){.ev-row{grid-template-columns:1fr}}
</style>
</head>
<body>
<header class="top"><div class="wrap">
  <div class="brand"><div class="logo">LK</div><div><b>Как агент находит связи</b><small>сигналы → кандидат → проверка → холодный старт</small></div></div>
  <a href="/" style="color:#cbd5e1;text-decoration:none">На главную</a>
</div></header>
<main class="wrap">
  <h1>Разбор одной связи</h1>
  <p class="lead">Показываем, какие сигналы собраны, как агент выбрал тип связи и что он делает для клиента без истории.</p>
  <div class="toolbar">
    <button class="pair-btn on" data-from="GAPPS-BUS-STD-1USER-1MO-First-Year" data-to="GAPPS-STARTER-1USER-1MO-First-Year">Google Workspace: Business → Starter</button>
    <button class="pair-btn" data-from="17065" data-to="17067">Ревизор 1 → Ревизор 2</button>
  </div>
  <div class="steps">
    <section class="step">
      <h2>1. Сигналы</h2>
      <div class="sub">Документация вендора, страница товара, атрибут каталога и совместные покупки.</div>
      <div class="ev" id="evidence"><div class="empty">Загружаю…</div></div>
    </section>
    <section class="step">
      <h2>2. Кандидат</h2>
      <div class="sub">Агент выбирает тип связи и уверенность, а не просто «похожие товары».</div>
      <div id="candidate"><div class="empty">Загружаю…</div></div>
    </section>
    <section class="step">
      <h2>3. Проверка</h2>
      <div class="sub">Связь попадает в каталог со статусом: подтверждена или требует проверки.</div>
      <div id="status"><div class="empty">Загружаю…</div></div>
    </section>
    <section class="step">
      <h2>4. Холодный старт</h2>
      <div class="sub">У клиента нет истории: агент выбирает популярное в отрасли и объясняет, почему.</div>
      <div class="cold" id="cold"><div class="empty">Загружаю…</div></div>
    </section>
  </div>
  <div class="foot">Демо показывает заранее собранные сигналы. В продакшене агент обходит страницы вендора и каталог по расписанию.</div>
</main>
<script>
function esc(s){ return String(s==null?'':s).replace(/[&<>"']/g, function(c){ return {'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]; }); }
function typeLabel(t){ return {upgrade:'Апгрейд', adjacent_task:'Дополнение', accessory:'Аксессуар', alternative:'Альтернатива', substitute:'Замена'}[t]||t; }
async function load(from, to){
  var r = await fetch('/v1/agent/demo?from='+encodeURIComponent(from)+'&to='+encodeURIComponent(to));
  var d = await r.json();
  if(!r.ok){ document.getElementById('evidence').innerHTML='<div class="empty">'+esc(d.error||'Ошибка')+'</div>'; return; }
  var ev = (d.evidence||[]).map(function(e){
    var body = esc(e.summary||'');
    if(e.url) body = '<a href="'+esc(e.url)+'" target="_blank" rel="noopener">'+body+'</a>';
    return '<div class="ev-row"><div class="ev-kind">'+esc(e.label||e.kind)+'</div><div>'+body+'</div></div>';
  }).join('');
  document.getElementById('evidence').innerHTML = ev || '<div class="empty">Сигналов нет — связь создана вручную</div>';
  document.getElementById('candidate').innerHTML =
    '<div class="pair">'+esc(d.from_title||d.from_id)+' → '+esc(d.to_title||d.to_id)+'</div>'+
    '<span class="badge">'+esc(typeLabel(d.type))+'</span>'+
    '<span class="badge green">уверенность '+Math.round((d.confidence||0)*100)+'%</span>'+
    '<div class="sub" style="margin-top:8px">'+esc(d.reason||'')+'</div>';
  var confirmed = d.status==='active'||d.status==='confirmed';
  document.getElementById('status').innerHTML =
    '<span class="badge '+(confirmed?'green':'amber')+'">'+(confirmed?'Подтверждена':'Требует проверки')+'</span>'+
    '<span class="badge">'+esc(d.status||'—')+'</span>'+
    '<div class="sub" style="margin-top:8px">'+(confirmed?'Связь видна клиентам в блоке рекомендаций.':'Связь видна в каталоге, но не выдаётся клиентам до подтверждения.')+'</div>';
  var cold = d.cold_start||{};
  var items = (cold.items||[]).map(function(it){
    return '<div class="cold-item"><b>'+esc(it.title)+'</b><small>'+esc(typeLabel(it.type))+' · '+esc(it.reason)+'</small></div>';
  }).join('');
  document.getElementById('cold').innerHTML =
    '<div class="sub"><b>Клиент:</b> '+esc(cold.client_id||'—')+' · '+esc(cold.industry||'без отрасли')+' · '+(cold.cold_start?'истории нет':'есть история')+'</div>'+
    '<div class="sub"><b>Подход:</b> '+esc(cold.approach||'—')+'</div>'+(items||'<div class="empty">Нет рекомендаций</div>');
}
document.querySelectorAll('.pair-btn').forEach(function(b){
  b.addEventListener('click', function(){
    document.querySelectorAll('.pair-btn').forEach(function(x){ x.classList.remove('on'); });
    b.classList.add('on');
    load(b.dataset.from, b.dataset.to).catch(function(e){ document.getElementById('evidence').innerHTML='<div class="empty">'+esc(e.message)+'</div>'; });
  });
});
load('GAPPS-BUS-STD-1USER-1MO-First-Year','GAPPS-STARTER-1USER-1MO-First-Year').catch(function(e){ document.getElementById('evidence').innerHTML='<div class="empty">'+esc(e.message)+'</div>'; });
</script>
</body>
</html>
`)
