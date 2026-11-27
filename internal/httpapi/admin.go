package httpapi

import "net/http"

// adminPage renders the rule editor. It is intentionally a single static page:
// the demo needs no build step, and every change goes through the public API.
func (h *handler) adminPage(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(adminHTML))
}

const adminHTML = `<!doctype html>
<html lang="ru">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>LK Advisor — аналитика рекомендаций</title>
<style>
:root{--bg:#f6f7fb;--card:#fff;--ink:#111827;--muted:#6b7280;--line:#e5e7eb;--red:#c8102e;--blue:#1d4ed8;--green:#047857;--amber:#b45309;--purple:#6d28d9}
*{box-sizing:border-box}
body{margin:0;background:var(--bg);color:var(--ink);font:14px/1.45 -apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,Arial,sans-serif}
.top{background:linear-gradient(135deg,#070b14 0%,#111827 60%,#1f2937 100%);color:#fff;padding:18px 0;border-bottom:1px solid rgba(255,255,255,.07);box-shadow:0 8px 28px rgba(15,23,42,.16)}
.wrap{max-width:none;margin:0;padding:0}
.top .wrap{padding:0 16px}
.top .wrap{display:flex;align-items:center;justify-content:space-between;gap:20px}
.brand{display:flex;align-items:center;gap:12px}
.logo{background:linear-gradient(135deg,#e11d48,#991b1b);color:#fff;font-weight:800;border-radius:11px;width:42px;height:42px;display:grid;place-items:center;font-size:14px;letter-spacing:.5px;box-shadow:0 8px 22px rgba(225,29,72,.3)}
.brand b{display:block;font-size:18px}
.brand small{color:#94a3b8}
nav a{color:#cbd5e1;text-decoration:none;margin-left:18px}
nav a:hover{color:#fff}
main{padding:10px 0 20px}
h1{font-size:24px;margin:0 0 6px;letter-spacing:-.02em}
.lead{color:var(--muted);margin:0 0 14px;max-width:900px}
.hero-stats{display:none!important}
.stats{display:grid;grid-template-columns:repeat(4,1fr);gap:10px;margin-bottom:16px}
.stat{background:var(--card);border:1px solid var(--line);border-radius:12px;padding:10px 12px;box-shadow:0 3px 12px rgba(15,23,42,.035);transition:.15s}
.stat:hover{transform:translateY(-1px);box-shadow:0 10px 26px rgba(15,23,42,.07)}
.stat b{display:block;font-size:20px}
.stat span{color:var(--muted);font-size:12px}
.layout{display:grid;grid-template-columns:minmax(0,1fr);gap:20px;align-items:start}
.layout.registry-mode{grid-template-columns:minmax(0,1fr)}
body.registry-mode main{padding:0}
body.registry-mode h1{display:none}
body.registry-mode .top{display:none}
body.registry-mode .lead{display:none}
body.registry-mode .hero-stats{display:none}
body.registry-mode .panel{border-left:0;border-right:0;border-radius:0}
body.registry-mode .tabs{border-radius:0}
.layout.registry-mode .side{display:none!important;display:none}
.panel,.card{background:var(--card);border:1px solid var(--line);border-radius:14px;box-shadow:0 8px 24px rgba(15,23,42,.045)}
.panel{padding:0;overflow:hidden}
.tabs{display:flex;gap:3px;padding:4px;border-bottom:1px solid var(--line);background:#fbfbfd;flex-wrap:nowrap;overflow-x:auto;scrollbar-width:thin}
.tab{border:0;background:transparent;padding:5px 9px;font-size:13px;border-radius:8px;cursor:pointer;color:var(--muted);font-weight:600;white-space:nowrap}
.tab.on{background:#fff;color:var(--ink);box-shadow:0 1px 4px rgba(15,23,42,.08)}
.tabbody{padding:12px}
.card{padding:16px}
.card h3{margin:0 0 10px;font-size:15px}
.grid{display:grid;gap:10px}
.g2{grid-template-columns:1fr 1fr}
.g3{grid-template-columns:1fr 1fr 1fr}
.g4{grid-template-columns:1.2fr 1.2fr 1fr .8fr 1fr auto}
label{display:block;font-size:12px;color:var(--muted);margin-bottom:4px}
input,select,textarea{width:100%;border:1px solid #d1d5db;border-radius:9px;padding:9px 10px;background:#fff;color:var(--ink);font:inherit}
input[type=range]{padding:0}
textarea{min-height:92px;resize:vertical}
button{border:0;border-radius:9px;padding:10px 14px;font-weight:700;cursor:pointer;background:var(--red);color:#fff}
button.secondary{background:#eef2ff;color:var(--blue)}
button.success{background:#047857;color:#fff}
button.success:hover{background:#065f46}
button.ghost{background:transparent;color:var(--red);padding:5px 8px}
button.small{padding:4px 8px;font-size:12px}
button:disabled{opacity:.5;cursor:not-allowed}
.row{display:flex;gap:10px;align-items:end;flex-wrap:wrap}
.row > *{min-width:0}
.formbox{background:#fafafa;border:1px solid var(--line);border-radius:12px;padding:12px;margin-bottom:12px}
.form-title{font-weight:800;margin-bottom:10px}
.toolbar{display:flex;gap:4px;margin-bottom:4px;flex-wrap:wrap;align-items:center;justify-content:flex-start}
.toolbar input{flex:0 1 240px;max-width:240px;padding:4px 7px;font-size:12px}
#registry-note{flex:1 1 280px;max-width:none}
.toolbar select{flex:0 1 150px;padding:4px 6px;font-size:12px}
.conf-filter{display:inline-flex;align-items:center;gap:5px;font-size:11px;color:var(--muted);white-space:nowrap}
.conf-filter input[type=range]{width:130px;padding:0;accent-color:#1d4ed8}
#registry-conf-label{min-width:44px;color:var(--ink);font-weight:700;font-size:12px}
.toolbar-filters{display:flex;gap:6px;flex-wrap:wrap;flex:1 1 auto}
#registry-metrics{display:flex;gap:10px;flex-wrap:wrap;margin:2px 0 4px;font-size:12px;color:var(--muted)}
#registry-metrics b{color:var(--ink);font-size:13px}
#registry-metrics .sep{color:#cbd5e1}

.tablewrap{border:1px solid var(--line);border-radius:0;overflow:auto;max-height:calc(100vh - 100px);min-height:220px}
table{border-collapse:collapse;width:100%;font-size:13px}
table.registry{table-layout:fixed}
table.registry td{vertical-align:middle}
table.registry th,table.registry td{padding:4px 6px}
.clip{display:inline-block;max-width:260px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;vertical-align:bottom}
.clip-sm{max-width:180px}
.pair{display:block;line-height:1.3;min-width:0;cursor:pointer}
.pair-from,.pair-to{display:block;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.pair-from{color:#111827}
.pair-to{color:#111827;font-weight:600}
.pair-to .arrow{color:#9ca3af;margin-right:5px;font-weight:400}
.pair-text{cursor:pointer}
.pair-from.expanded,.pair-to.expanded{white-space:normal;overflow:visible}
.pair-text.full{white-space:normal;word-break:break-word;font-weight:600}
.alias-edit{font-size:12px;color:var(--muted);margin:4px 0}
.alias-edit b{color:var(--ink)}
.danger-zone{display:flex;justify-content:flex-end;align-items:center;gap:18px;margin:22px 0 4px;padding-top:14px;border-top:1px solid var(--line)}
.danger-copy{text-align:right}
.danger-copy b{color:#7f1d1d;font-size:13px}
.chart{display:grid;gap:14px;margin-top:16px}
.chart-row{display:grid;grid-template-columns:110px 1fr;gap:14px;align-items:end}
.chart-row>span{font-size:15px;font-weight:600;color:#374151;padding-bottom:8px}
.chart-bars{display:flex;gap:26px;align-items:flex-end;height:170px;border-bottom:2px solid var(--line);padding-bottom:6px}
.chart-col{display:flex;flex-direction:column;align-items:center;gap:5px;width:86px;height:100%}
.chart-col i{display:block;width:48px;border-radius:6px 6px 0 0;background:#c7d2fe;margin-top:auto}
.chart-col.ours i{background:#1d4ed8}
.chart-col b{font-size:16px}
.chart-col span{font-size:13px;color:#4b5563;white-space:nowrap}
.donuts{display:flex;gap:44px;flex-wrap:wrap;margin-top:18px}
.donut-wrap{display:flex;flex-direction:column;align-items:center;gap:6px;font-size:15px;font-weight:600}
.donut-wrap small{color:var(--muted);font-weight:400;font-size:13px}
.donut{width:128px;height:128px;border-radius:50%;background:conic-gradient(#6366f1 calc(var(--v)*1%), #22c55e 0);display:grid;place-items:center}
.donut .hole{width:84px;height:84px;background:#fff;border-radius:50%;display:grid;place-items:center;font-size:18px;font-weight:800}
#tab-experiments .tablewrap{min-height:0}
#tab-dataset .form-title{font-size:18px}
#tab-dataset .hint{font-size:14px}
#tab-dataset .stat b{font-size:30px}
#tab-dataset .stat span{font-size:14px}
#tab-dataset table{font-size:15px}
#tab-dataset th,#tab-dataset td{padding:11px 12px}
.method{display:grid;gap:4px;margin:8px 0 14px;font-size:13.5px;line-height:1.45;color:#374151}
.method b{color:#111827}
.method code{background:#f3f4f6;padding:0 4px;border-radius:4px;font-size:12.5px}
.exp-head{display:flex;justify-content:space-between;gap:12px;align-items:flex-start}
.exp-kpis{display:grid;grid-template-columns:repeat(4,1fr);gap:8px;margin:10px 0}
.exp-kpi{background:#f8fafc;border:1px solid var(--line);border-radius:10px;padding:9px 11px}
.exp-kpi b{display:block;font-size:22px}
.exp-kpi span{font-size:11px;color:var(--muted)}
.exp-bars{margin:8px 0}
.exp-bar{display:flex;align-items:center;gap:8px;margin:4px 0;font-size:12px}
.exp-bar>span{width:74px;color:var(--muted)}
.exp-bar .track{flex:1;height:11px;background:#eef2ff;border-radius:6px;overflow:hidden}
.exp-bar i{display:block;height:100%;background:#1d4ed8}
.exp-bar.control i{background:#94a3b8}
.exp-bar b{width:52px;text-align:right}
button.danger{background:#7f1d1d;color:#fff;padding:12px 22px;font-size:14px;border-radius:10px;font-weight:800}
button.danger:hover{background:#991b1b}
.pair-qual{display:inline-block;font-size:10px;font-weight:600;color:#6b7280;background:#f1f5f9;border-radius:4px;padding:0 4px;vertical-align:1px}
.pair-delta del{color:#b91c1c;text-decoration:line-through}
.pair-delta ins{color:#047857;text-decoration:none;font-weight:700}
.rel-pair{display:flex;align-items:center;gap:6px;min-width:0}
.rel-pair .clip{max-width:48%}
.rel-pair .arrow{color:#9ca3af;flex:0 0 auto}
.prov-cell{white-space:nowrap;font-size:12px;color:#374151}
.conf-meter{display:inline-flex;align-items:flex-end;gap:2px;height:11px;cursor:help;vertical-align:middle}
.conf-meter i{width:3px;border-radius:1px;background:#e5e7eb}
.conf-meter i:nth-child(1){height:4px}
.conf-meter i:nth-child(2){height:7px}
.conf-meter i:nth-child(3){height:10px}
.conf-meter.hi i{background:#047857}
.conf-meter.mid i:nth-child(-n+2){background:#b45309}
.conf-meter.lo i:nth-child(1){background:#b91c1c}
.type-chip{display:inline-flex;align-items:center;gap:5px;font-size:12px;font-weight:700;color:#374151;white-space:nowrap;cursor:help}
.type-chip i{width:8px;height:8px;border-radius:2px;background:var(--tc,#94a3b8);flex:0 0 auto}
.legend{display:none}
.legend .type-chip{font-size:11px;font-weight:600;color:var(--muted)}
.ev{display:inline-flex;align-items:center;gap:6px;max-width:100%}
.ev-dot{width:7px;height:7px;border-radius:50%;background:#94a3b8;flex:0 0 auto}
.ev-attribute{background:#6d28d9}
.ev-co_purchase{background:#1d4ed8}
.ev-llm{background:#b45309}
.ev-manual{background:#047857}
.ev-more{color:var(--muted);font-size:11px}
.ev-list{display:grid;gap:6px;margin:8px 0}
.ev-row{display:flex;gap:8px;align-items:flex-start;font-size:12px}
.ev-kind{color:var(--muted);min-width:92px;font-weight:600}
.ev-note{background:#f8fafc;border:1px solid var(--line);border-radius:8px;padding:8px 10px;font-size:12px;color:#374151;margin:8px 0}
th,td{padding:8px 10px;text-align:left;border-bottom:1px solid #f0f1f5;vertical-align:top}
th{background:#fafafa;color:var(--muted);font-weight:600;position:sticky;top:0;z-index:1}
tr:hover td{background:#fcfcff}
.pill{display:inline-block;padding:2px 8px;border-radius:999px;font-size:11px;font-weight:700;white-space:nowrap}
.pill.on{background:#e7f7ef;color:var(--green)}
.pill.q{background:#fff7ed;color:var(--amber)}
.pill.off{background:#f3f4f6;color:var(--muted)}
.pill.ai{background:#f3e8ff;color:var(--purple)}
.pill.type{background:#eef2ff;color:var(--blue)}
.rule{display:flex;justify-content:space-between;gap:12px;padding:12px 0;border-bottom:1px solid #f0f1f5}
.rule:last-child{border-bottom:0}
.rule b{display:block}
.rule small{color:var(--muted)}
.empty{padding:16px;color:var(--muted);text-align:center}
.weights .w{display:grid;grid-template-columns:150px 1fr 74px;gap:12px;align-items:center;margin:12px 0}
.weights .w b{font-weight:600;font-size:13px}
.weights .w output{text-align:right;font-variant-numeric:tabular-nums}
.sum{margin:12px 0;font-size:12px;color:var(--muted)}
.sum.bad{color:var(--red)}
.side{display:none!important;display:grid;gap:16px}
.hint{font-size:12px;color:var(--muted)}
.chips{display:flex;flex-wrap:wrap;gap:6px;margin:10px 0}
.chip{border:1px solid #dbe2ff;background:#f5f7ff;color:var(--blue);padding:6px 9px;border-radius:999px;font-size:12px;cursor:pointer}
.preview-item{border:1px solid var(--line);border-radius:10px;padding:10px;margin:8px 0}
.preview-item b{display:block;font-size:13px}
.preview-item small{color:var(--muted)}
.ops{margin-top:14px}
.op{border:1px solid var(--line);border-left:4px solid var(--green);border-radius:10px;padding:10px 12px;margin:8px 0;background:#fbfefc}
.op b{display:block}
.op small{color:var(--muted)}
pre.prompt{white-space:pre-wrap;background:#0f172a;color:#e5e7eb;border-radius:10px;padding:14px;max-height:360px;overflow:auto;font:12px/1.5 ui-monospace,SFMono-Regular,Menlo,monospace}
.type-head{display:flex;gap:8px;align-items:center;flex-wrap:wrap;margin-bottom:4px}
.type-card{border:1px solid var(--line);border-radius:12px;padding:12px;margin:10px 0}
.type-card .hint{margin-top:6px}
.modal{position:fixed;inset:0;background:rgba(15,23,42,.45);display:flex;align-items:center;justify-content:center;z-index:60}
.modal-box{background:#fff;border-radius:12px;padding:18px;max-width:480px;width:calc(100% - 32px);box-shadow:0 24px 64px rgba(0,0,0,.3)}
.exp-modal{max-width:1000px;max-height:96vh;display:flex;flex-direction:column}
.exp-modal .form-title{font-size:22px}
.exp-modal .hint{font-size:13px}
.exp-modal .toolbar button{font-size:14px;padding:10px 18px;border-radius:10px}
.exp-modal .exp-form{overflow:auto;min-height:0;padding-right:4px}
.exp-modal .toolbar{flex:0 0 auto}
.exp-modal .tablewrap{min-height:0}
.exp-form{display:grid;gap:12px;margin-top:14px}
.exp-form label{display:grid;gap:6px;font-size:14px;color:#475569}
.exp-form input,.exp-form select,.exp-form textarea{width:100%;border:1px solid var(--line);border-radius:10px;padding:10px 12px;font:inherit;font-size:15px;background:#fff}
.exp-form select[size]{height:80px;padding:6px}
.exp-form .row2{display:grid;grid-template-columns:1fr 1fr;gap:14px}
.exp-form .row4{display:grid;grid-template-columns:repeat(4,1fr);gap:12px}
.exp-guards{display:flex;gap:16px;align-items:center;flex-wrap:wrap;font-size:13px;color:#334155}
.exp-guards label{display:flex;align-items:center;gap:8px;font-size:14px;color:#334155}
.exp-guards input{width:auto}
.tip{position:fixed;z-index:50;max-width:380px;background:#111827;color:#fff;padding:8px 10px;border-radius:8px;font-size:12px;line-height:1.35;box-shadow:0 12px 32px rgba(0,0,0,.28);pointer-events:none;opacity:0;transform:translateY(4px);transition:opacity .12s}
.tip.on{opacity:1;transform:none}
.toast{position:fixed;right:20px;bottom:20px;background:#111827;color:#fff;padding:11px 15px;border-radius:10px;opacity:0;transform:translateY(8px);transition:.2s;pointer-events:none;z-index:9}
.toast.show{opacity:1;transform:none}
.toast.err{background:#991b1b}
.hidden{display:none!important}
@media(max-width:1350px){
  .clip{max-width:200px}
}
@media(max-width:980px){.layout{grid-template-columns:1fr}.stats{grid-template-columns:1fr 1fr}.g4{grid-template-columns:1fr 1fr}.top .wrap{align-items:flex-start}}
@media(max-width:760px){
  .wrap{padding:0}
  .top .wrap{padding:0 10px}
  h1{font-size:19px}
  .tabbody{padding:9px}
  .toolbar input,.toolbar select{max-width:none;flex:1 1 140px}
  .clip{max-width:150px}
  #registry-metrics .stat{flex:1 1 45%}
}
</style>
</head>
<body>
<header class="top"><div class="wrap">
  <div class="brand"><div class="logo">LK</div><div><b>LK Advisor</b><small>Настройка правил рекомендаций</small></div></div>
  <nav><a href="/">Кабинет</a><a href="/api">API</a></nav>
</div></header>
<main class="wrap">
  <h1>Аналитика рекомендаций</h1>
  <p class="lead">Качество, покрытие и структура рекомендаций по каталогу.</p>
  <div class="stats hero-stats">
    <div class="stat"><b id="st-products">—</b><span>товаров в каталоге</span></div>
    <div class="stat"><b id="st-relations">—</b><span>связей между товарами</span></div>
    <div class="stat"><b id="st-rules">—</b><span>правил и ограничений</span></div>
    <div class="stat"><b id="st-active">—</b><span>включённых связей</span></div>
  </div>
  <div class="layout">
    <section class="panel">
      <div class="tabs">
        <button class="tab on" data-tab="dataset">Дашборд</button>
        <button class="tab" data-tab="registry">Каталог</button>
        <button class="tab" data-tab="types">Типы связей</button>
        <button class="tab" data-tab="experiments">Гипотезы</button>
        <button class="tab" data-tab="analytics">Аналитика</button>
      </div>
      <div class="tabbody">
        <div id="tab-relations" class="hidden">
          <div class="formbox" style="border-color:#dbe2ff;background:#f8faff">
            <div class="form-title">Агент разметки</div>
            <p class="hint">Выберите пару товаров — агент предложит тип из активных шаблонов, уверенность и объяснение. Ничего не сохраняется без подтверждения.</p>
            <div class="grid g3">
              <div><label>Откуда (куплено)</label><select id="agent-from"></select></div>
              <div><label>Куда (предлагаем)</label><select id="agent-to"></select></div>
              <div><label>&nbsp;</label><button id="agent-run" class="secondary">Предложить связь</button></div>
            </div>
            <div id="agent-result"></div>
          </div>
          <div class="formbox">
            <div class="form-title">Новая или изменяемая связь</div>
            <div class="grid g4">
              <div><label>Откуда (куплено)</label><select id="rel-from"></select></div>
              <div><label>Куда (предлагаем)</label><select id="rel-to"></select></div>
              <div><label>Тип связи</label><select id="rel-type"></select></div>
              <div><label>Уверенность, %</label><input id="rel-conf" type="number" min="0" max="100" value="90"></div>
              <div><label>Статус</label><select id="rel-status"></select></div>
              <div><label>&nbsp;</label><button id="rel-save">Сохранить</button></div>
            </div>
            <p class="hint" id="rel-hint">Апгрейд — старшая версия, дополнение — другая задача, замена — конкурент. Замену движок покажет только как бюджетную альтернативу.</p>
          </div>
          <div class="toolbar">
            <input id="rel-search" placeholder="Поиск по названию или id">
            <select id="rel-filter-type"><option value="">Все типы</option></select>
            <select id="rel-filter-status"><option value="">Все статусы</option></select>
            <span class="hint" id="rel-count"></span>
          </div>
          <div class="tablewrap"><table>
            <thead><tr><th>Откуда</th><th>Куда</th><th>Тип</th><th>Увер.</th><th>Статус</th><th></th></tr></thead>
            <tbody id="rel-body"></tbody>
          </table></div>
        </div>

        <div id="tab-registry" class="hidden">
          <div class="stats" id="registry-metrics" style="margin-bottom:12px"></div>
          <div class="toolbar">
            <input id="registry-search" placeholder="Поиск по товару или id">
            <span class="conf-filter" title="Минимальная уверенность">уверенность
              <input id="registry-conf-min" type="range" min="0" max="100" step="1" value="0">
              <span id="registry-conf-label">любая</span>
            </span>
            <button id="registry-confirm-filter" class="small" title="Подтвердить всех кандидатов по текущему фильтру">Подтвердить все</button>
            <button id="registry-filters-toggle" class="secondary small">Фильтры</button>
            <div id="registry-filters" class="toolbar-filters hidden">
            <select id="registry-type"><option value="">Все типы</option></select>
            <select id="registry-status">
              <option value="">Все статусы</option>
              <option value="candidate">Кандидаты</option>
              <option value="active">Активные</option>
              <option value="rejected">Отклонённые</option>
              <option value="stale">Устаревшие</option>
              <option value="expired">Истёкшие</option>
              <option value="superseded">Заменённые</option>
            </select>
            <select id="registry-source">
              <option value="">Все источники</option>
              <option value="manual">Ручные</option>
              <option value="co_purchase">Co-purchase</option>
              <option value="llm">LLM</option>
              <option value="attribute">Атрибуты</option>
            </select>
            <select id="registry-family"><option value="">Все семейства</option></select>
            <label class="toggle" style="display:flex;align-items:center;gap:6px;white-space:nowrap"><input type="checkbox" id="registry-certified" style="width:auto"> с сертификатом ФСТЭК</label>
            <button id="registry-apply" class="small">Применить</button>
            </div>
            <button class="secondary small" data-view="all">Смысловые</button>
            <button class="secondary small" data-view="disputed">Спорные</button>
            <button class="secondary small" data-view="expiring">Истекают</button>
            <button class="secondary small" data-view="structural">Структурные</button>
            <button id="registry-generate" class="small">Сгенерировать</button>
            <button id="registry-import" class="secondary small">Импорт</button>
            <button id="registry-sweep" class="secondary small">Проверить TTL</button>
            <span class="hint" id="registry-count"></span>
          </div>
          <div id="registry-legend" class="legend"></div>
          <div class="tablewrap"><table class="registry">
            <colgroup>
              <col style="width:3%"><col style="width:54%">
              <col style="width:9%"><col style="width:8%">
              <col style="width:9%"><col style="width:17%">
            </colgroup>
            <thead><tr>
              <th><input type="checkbox" id="registry-all" style="width:auto"></th>
              <th>Связь</th>
              <th title="Что это за связь и как её использовать">Тип</th><th>Статус</th>
              <th title="Кто создал связь: человек, алгоритм совместных покупок, LLM или правило атрибутов">Откуда</th>
              <th title="Почему связь считается верной: совместные заказы, общая продуктовая группа, документ вендора">Доказательство</th>
            </tr></thead>
            <tbody id="registry-body"></tbody>
          </table></div>
          <div class="toolbar" style="margin-top:6px">
            <button id="registry-more" class="secondary small">Показать ещё</button>
            <span class="hint" id="registry-more-hint"></span>
          </div>
          <div class="toolbar" style="margin-top:6px">
            <input id="registry-note" placeholder="Причина решения (обязательна для отказа)">
            <button id="registry-confirm" class="small">Подтвердить выбранные</button>
            <button id="registry-reject" class="secondary small">Отклонить</button>
            <button id="registry-defer" class="secondary small">Отложить</button>
          </div>
          <div id="registry-detail" class="formbox" style="margin-top:12px"></div>
          <div class="danger-zone">
            <div class="danger-copy">
              <b>Опасное действие</b>
              <div class="hint">Удаляет все связи, события, алиасы и импортированные позиции. Остаётся исходный каталог.</div>
            </div>
            <button id="registry-reset" class="danger">Сбросить каталог</button>
          </div>
        </div>

        <div id="tab-validation" class="hidden">
          <div class="toolbar">
            <select id="review-filter">
              <option value="quarantine,llm_suggested">Все на проверке</option>
              <option value="quarantine">Только quarantine</option>
              <option value="llm_suggested">Только предложенные ИИ</option>
            </select>
            <input id="review-search" placeholder="Поиск по товару">
            <button id="review-refresh" class="secondary small">Обновить</button>
            <label class="hint" style="display:flex;align-items:center;gap:5px;margin:0"><input type="checkbox" id="review-all" style="width:auto"> Выбрать все</label>
            <button id="review-confirm" class="small">Подтвердить выбранные</button>
            <button id="review-reject" class="secondary small">Отклонить</button>
          </div>
          <div class="grid" style="margin-bottom:12px"><div><label>Заметка к решению</label><input id="review-note" placeholder="Почему подтверждаем или отклоняем"></div></div>
          <div id="review-list"></div>
        </div>

        <div id="tab-types" class="hidden">
          <div class="formbox">
            <div class="form-title">Шаблоны связей</div>
            <p class="hint">Справочник шаблонов. Редактирование отключено: набор, описания и примеры меняются только через релиз, чтобы промпт разметки нельзя было изменить на живую.</p>
          </div>
          <div id="type-list"></div>
        </div>

        <div id="tab-rules" class="hidden">
          <div class="formbox">
            <div class="form-title">Новое правило</div>
            <div class="grid g4">
              <div><label>Тип</label><select id="rule-type"></select></div>
              <div><label>Товар</label><select id="rule-product"></select></div>
              <div id="rule-other-wrap"><label>Второй товар</label><select id="rule-other"></select></div>
              <div id="rule-discount-wrap" class="hidden"><label>Максимальная скидка, %</label><input id="rule-discount" type="number" min="1" max="100" value="10"></div>
              <div><label>&nbsp;</label><button id="rule-save">Добавить</button></div>
            </div>
            <p class="hint" id="rule-hint"></p>
          </div>
          <div id="rule-list"></div>
        </div>

        <div id="tab-weights" class="hidden">
          <div class="formbox weights">
            <div class="form-title">Формула оценки</div>
            <p class="hint">Итоговый балл = сумма «фактор × вес». Веса нормализуются к 100%.</p>
            <div class="grid" style="margin-bottom:10px"><div><label>Сегмент</label><select id="weight-audience"><option value="default">Общие</option><option value="b2b">B2B</option><option value="b2c">B2C</option></select></div></div>
            <div id="weight-rows"></div>
            <div class="sum" id="weight-sum"></div>
            <button id="weight-save">Сохранить веса</button>
          </div>
          <p class="hint">Изменение весов влияет на порядок карточек, но не отменяет правила совместимости, аудитории и безопасности.</p>
        </div>

        <div id="tab-dataset">
          <div class="formbox">
            <div class="form-title">Качество рекомендаций · офлайн-бэктест</div>
            <p class="hint">Официальный протокол: basket = (order_id, order_week), train до 31.08.2026, test с 07.09.2026, Precision@3 по всем 817 item_id.</p>
            <div class="stats">
              <div class="stat"><b>11.37%</b><span>A · наша P@3</span></div>
              <div class="stat"><b>+102%</b><span>A · к baseline</span></div>
              <div class="stat"><b>14.53%</b><span>B · наша P@3</span></div>
              <div class="stat"><b>+142%</b><span>B · к baseline</span></div>
            </div>
            <div class="tablewrap"><table>
              <thead><tr><th>Группа</th><th>Popularity</th><th>Co-purchase</th><th>Наша модель</th><th>Рост к лучшему baseline</th></tr></thead>
              <tbody>
                <tr><td>A · B2B</td><td>3.75%</td><td>4.53%</td><td><b>10.38%</b></td><td>+129%</td></tr>
                <tr><td>A · B2C</td><td>20.00%</td><td>15.24%</td><td><b>20.00%</b></td><td>0% (малая выборка)</td></tr>
                <tr><td>A · все</td><td>5.44%</td><td>5.64%</td><td><b>11.37%</b></td><td>+102%</td></tr>
                <tr><td>B · B2B</td><td>5.22%</td><td>4.54%</td><td><b>13.61%</b></td><td>+200%</td></tr>
                <tr><td>B · B2C</td><td>20.00%</td><td>14.67%</td><td><b>20.00%</b></td><td>0% (малая выборка)</td></tr>
                <tr><td>B · все</td><td>7.36%</td><td>6.01%</td><td><b>14.53%</b></td><td>+142%</td></tr>
              </tbody>
            </table></div>
          </div>

          <div class="formbox">
            <div class="form-title">P@3: baseline против нашей модели</div>
            <div class="method">
              <div><b>P@3</b> = среднее по клиентам <code>hits / 3</code>. Рекомендуем ровно 3 новых item_id; если товаров меньше трёх — недостающие позиции считаются промахом. Считается по всем клиентам групп A и B; LLM-объяснения в метрике не участвуют.</div>
              <div><b>Срез:</b> train — заказы до 31.08.2026, test — с 07.09.2026; корзина = <code>(order_id, order_week)</code>.</div>
              <div><b>Группы:</b> A — есть покупки в train и хотя бы один новый товар в test (337 клиентов); B — подмножество A с ровно одним товаром в истории (172).</div>
              <div><b>Baseline:</b> popularity — число уникальных клиентов в train; co-purchase — пары из одной корзины, тай-брейк по популярности и item_id.</div>
              <div><b>Наша модель:</b> B2B — co-purchase (<code>3×count/√popularity</code>) + 0.5×popularity + 0.2×совпадение слов в названии; B2C — popularity.</div>
              <div><b>Variant share:</b> попадание считается вариантом, если <code>product_group</code> угаданного товара уже есть в истории клиента.</div>
            </div>
            <div class="chart">
              <div class="chart-row"><span>A · все</span><div class="chart-bars">
                <div class="chart-col"><b>5.44</b><i style="height:36%"></i><span>popularity</span></div>
                <div class="chart-col"><b>5.64</b><i style="height:38%"></i><span>co-purchase</span></div>
                <div class="chart-col ours"><b>11.37</b><i style="height:76%"></i><span>наша</span></div>
              </div></div>
              <div class="chart-row"><span>B · все</span><div class="chart-bars">
                <div class="chart-col"><b>7.36</b><i style="height:49%"></i><span>popularity</span></div>
                <div class="chart-col"><b>6.01</b><i style="height:40%"></i><span>co-purchase</span></div>
                <div class="chart-col ours"><b>14.53</b><i style="height:97%"></i><span>наша</span></div>
              </div></div>
              <div class="chart-row"><span>A · B2B</span><div class="chart-bars">
                <div class="chart-col"><b>3.75</b><i style="height:25%"></i><span>popularity</span></div>
                <div class="chart-col"><b>4.53</b><i style="height:30%"></i><span>co-purchase</span></div>
                <div class="chart-col ours"><b>10.38</b><i style="height:69%"></i><span>наша</span></div>
              </div></div>
              <div class="chart-row"><span>B · B2B</span><div class="chart-bars">
                <div class="chart-col"><b>5.22</b><i style="height:35%"></i><span>popularity</span></div>
                <div class="chart-col"><b>4.54</b><i style="height:30%"></i><span>co-purchase</span></div>
                <div class="chart-col ours"><b>13.61</b><i style="height:91%"></i><span>наша</span></div>
              </div></div>
            </div>
          </div>

          <div class="formbox">
            <div class="form-title">Структура попаданий: варианты и новые продукты</div>
            <p class="hint">Вариант — product_group уже есть в истории клиента. Новый продукт — такого product_group в истории не было. B2C variant share = 0 по данным.</p>
            <div class="donuts">
              <div class="donut-wrap"><div class="donut" style="--v:58.5"><div class="hole">58.5%</div></div><span>A · B2B</span><small>варианты</small></div>
              <div class="donut-wrap"><div class="donut" style="--v:65"><div class="hole">65.0%</div></div><span>B · B2B</span><small>варианты</small></div>
              <div class="donut-wrap"><div class="donut" style="--v:0"><div class="hole">0%</div></div><span>B2C</span><small>варианты</small></div>
            </div>
            <div class="hint">Baseline co-purchase variant share: A B2B 12.2%, B B2B 10.0%.</div>
          </div>

          <div class="formbox">
            <div class="form-title">Текущее состояние реестра</div>
            <div class="stats" id="dataset-registry"></div>
            <div class="toolbar" style="margin-top:8px">
              <button id="dataset-open-catalog" class="secondary small">Проверить кандидатов в Каталоге</button>
            </div>
          </div>

          <div class="formbox">
            <div class="form-title">Внешние знания</div>
            <div class="stats" id="vendor-stats"></div>
            <div class="toolbar" style="margin-top:8px">
              <input type="file" id="vendor-file" accept=".csv" style="max-width:220px">
              <button id="vendor-import" class="secondary small">Импорт CSV</button>
              <input type="file" id="registry-file" accept=".xml" style="max-width:220px">
              <button id="registry-import" class="secondary small">Импорт реестра XML</button>
              <button id="vendor-link" class="secondary small">Сопоставить с каталогом</button>
              <button id="vendor-candidates" class="secondary small">Сгенерировать связи</button>
            </div>
            <div class="hint">CSV: vendor, product, family, edition, platforms, registry_number, source_url, aliases. Списки — через <code>|</code>.</div>
            <div class="hint" id="vendor-result"></div>
            <div id="vendor-classes"></div>
            <div class="form-title" style="font-size:15px;margin:18px 0 6px">Источники знаний</div>
            <div id="vendor-sources"></div>
          </div>

          <div class="formbox">
            <div class="form-title">Источник данных</div>
            <div class="stats" id="dataset-cards"></div>
            <div class="hint" id="dataset-feed" style="margin-top:8px"></div>
            <div class="hint" style="margin-top:8px">Источник: текущий каталог, 817 товаров и 419 product_group. Исторического снимка фида нет — оцениваем товары текущего фида. Загруженные позиции не входят в офлайн-оценку.</div>
          </div>
        </div>

        <div id="tab-experiments" class="hidden">
          <div class="toolbar">
            <button id="experiment-seed" class="success">Добавить гипотезу</button>
            <button id="open-lk" class="secondary small">Витрина клиента</button>
            <button id="experiment-refresh" class="secondary small">Обновить</button>
            <input id="experiment-search" type="search" placeholder="Поиск по базе знаний…" style="max-width:220px">
            <select id="experiment-decision-filter" style="max-width:170px">
              <option value="">Все решения</option>
              <option value="promote">promote</option>
              <option value="reject">reject</option>
              <option value="extend">extend</option>
            </select>
            <button id="experiment-search-btn" class="secondary small">Найти</button>
            <span class="hint">Гипотеза создаётся на паре каталога; показы идут через <code>/lk</code>, клики и покупки — из ЛК через <code>/v1/events</code>.</span>
          </div>
          <div class="stats" id="experiments-summary"></div>
          <div id="experiments-list"></div>
          <div id="experiment-knowledge"></div>
        </div>

        <div id="tab-analytics" class="hidden">
          <div class="toolbar">
            <select id="analytics-days">
              <option value="1">Сегодня</option>
              <option value="7" selected>7 дней</option>
              <option value="30">30 дней</option>
              <option value="90">90 дней</option>
            </select>
            <button id="analytics-refresh" class="secondary small">Обновить</button>
            <span class="hint">Показы и пропуски пишутся автоматически. Клики и покупки — через <code>POST /v1/events</code>.</span>
          </div>
          <div class="stats" id="analytics-cards" style="margin-bottom:0"></div>
          <div class="formbox" style="margin-top:16px">
            <div class="form-title">Стратегии и авто-А/Б</div>
            <label style="display:flex;align-items:center;gap:8px;margin-bottom:8px"><input type="checkbox" id="auto-strategy" style="width:auto"> Авто-перераспределение трафика</label>
            <div id="analytics-strategies"></div>
          </div>
          <div class="formbox" style="margin-top:16px"><div class="form-title">Кандидаты на проверку связей <button id="health-flag" class="secondary small" style="margin-left:8px">Пометить связи</button></div><div id="analytics-health"></div></div>
          <div class="formbox" style="margin-top:16px"><div class="form-title">Работающие связи <button id="relation-auto" class="secondary small" style="margin-left:8px">Авто-решение бандита</button></div><div id="relation-perf"></div></div>
          <div class="formbox" style="margin-top:16px"><div class="form-title">Скоупы связей и TTL</div><div class="hint" style="margin-bottom:8px">Связь проверяется отдельно для B2B/B2C и отрасли. Подтверждённая связь живёт 30 дней (B2B) или 60 дней (B2C), затем нужен re-test.</div><div id="relation-scopes"></div></div>
          <div class="grid g2" style="margin-top:16px">
            <div class="formbox" style="margin:0"><div class="form-title">Что показываем</div><div id="analytics-products"></div></div>
            <div class="formbox" style="margin:0"><div class="form-title">Что не показываем</div><div id="analytics-skips"></div></div>
          </div>
        </div>

        <div id="tab-assistant" class="hidden">
          <div class="formbox">
            <div class="form-title">Опишите изменение обычной фразой</div>
            <textarea id="assistant-text" placeholder="Например: Сделай Кибер Бэкап дополнением к Kaiten"></textarea>
            <div class="chips" id="chips"></div>
            <button id="assistant-propose">Предложить изменения</button>
          </div>
          <div id="assistant-result"></div>
        </div>
      </div>
    </section>

    <aside class="side">
      <div class="card">
        <h3>Проверка на клиенте</h3>
        <label>Клиент</label>
        <select id="preview-client"></select>
        <button class="secondary" id="preview-run" style="margin-top:10px;width:100%">Показать топ-3</button>
        <div id="preview-out" class="hint" style="margin-top:10px">Выберите клиента, чтобы увидеть эффект правил.</div>
      </div>
      <div class="card">
        <h3>Как это работает</h3>
        <p class="hint" style="margin:0">Связи задают, что предлагать после покупки. Правила отсекают несовместимое и ограничивают скидки. Веса решают, какой из допустимых вариантов выше. Все изменения записываются в журнал.</p>
      </div>
      <div class="card">
        <h3>Последние изменения</h3>
        <div id="audit-list" class="hint">—</div>
      </div>
    </aside>
  </div>
</main>
<div class="toast" id="toast"></div>
<div class="tip" id="registry-tip" role="tooltip"></div>
<div class="modal hidden" id="registry-confirm-modal">
  <div class="modal-box">
    <div class="form-title">Подтвердить связи?</div>
    <div id="registry-confirm-text"></div>
    <div class="toolbar" style="margin-top:14px;justify-content:flex-end">
      <button id="registry-confirm-no" class="secondary small">Отмена</button>
      <button id="registry-confirm-yes" class="small">Подтвердить</button>
    </div>
  </div>
</div>
<div class="modal hidden" id="experiment-modal">
  <div class="modal-box exp-modal">
    <div class="form-title">Новая гипотеза</div>
    <div class="hint">Пара товаров из каталога, тип связи и условия эксперимента.</div>
    <div class="exp-form">
      <div class="row2">
        <label>Источник
          <input id="exp-from-search" placeholder="Фильтр по названию…">
          <select id="exp-from" size="3"></select>
        </label>
        <label>Цель
          <input id="exp-to-search" placeholder="Фильтр по названию…">
          <select id="exp-to" size="3"></select>
        </label>
      </div>
      <div class="row4">
        <label>Тип связи<select id="exp-type"></select></label>
        <label>Аудитория<select id="exp-audience"><option value="b2b">B2B</option><option value="b2c">B2C</option></select></label>
        <label>Primary metric<select id="exp-metric">
          <option value="conversion">conversion</option>
          <option value="target_conversion">target_conversion</option>
          <option value="ctr">ctr</option>
        </select></label>
        <label>Traffic split<select id="exp-split">
          <option value="0.5">50 / 50</option>
          <option value="0.7">70 / 30</option>
          <option value="0.8">80 / 20</option>
          <option value="0.9">90 / 10</option>
        </select></label>
      </div>
      <div class="row2">
        <label>Отрасль (необязательно)<input id="exp-industry" placeholder="например, образование"></label>
        <div>
          <div class="hint" style="margin-bottom:6px">Guardrails</div>
          <div class="exp-guards">
            <label><input type="checkbox" id="exp-g-renewal" checked> renewal</label>
            <label><input type="checkbox" id="exp-g-upsell" checked> upsell</label>
            <label><input type="checkbox" id="exp-g-fallback" checked> fallback</label>
          </div>
        </div>
      </div>
      <label>Название гипотезы<input id="exp-title" maxlength="200"></label>
      <label>Формулировка<textarea id="exp-hypothesis" rows="2" maxlength="400"></textarea></label>
      <div class="hint" id="exp-form-error" style="color:#b91c1c"></div>
    </div>
    <div class="toolbar" style="justify-content:flex-end;margin-top:14px">
      <button id="exp-cancel" class="secondary small">Отмена</button>
      <button id="exp-submit" class="success small">Создать гипотезу</button>
    </div>
  </div>
</div>
<div class="modal hidden" id="emulation-modal">
  <div class="modal-box exp-modal">
    <div class="form-title">Прогон на эмуляторе</div>
    <div class="hint" id="emu-hypothesis"></div>
    <div class="exp-form">
      <div class="row2">
        <label>Профиль клиента<select id="emu-profile"></select></label>
        <label>Количество показов<select id="emu-impressions">
          <option value="200">200</option>
          <option value="400" selected>400</option>
          <option value="1000">1000</option>
          <option value="2000">2000</option>
        </select></label>
      </div>
      <div id="emu-new" class="hidden" style="display:grid;gap:8px">
        <div class="row2">
          <label>Название профиля<input id="emu-name" placeholder="Клиника «Здоровье+»" maxlength="120"></label>
          <label>Аудитория<select id="emu-audience"><option value="b2b">B2B</option><option value="b2c">B2C</option></select></label>
        </div>
        <label>Отрасль<select id="emu-industry"></select></label>
        <label>Описание клиента словами<textarea id="emu-description" rows="3" placeholder="Например: клиника на 15 сотрудников, нужны почта, антивирус и защита данных, бюджет ограничен" maxlength="600"></textarea></label>
      </div>
      <div class="hint" id="emu-form-error" style="color:#b91c1c"></div>
      <div id="emu-result"></div>
    </div>
    <div class="toolbar" style="justify-content:flex-end;margin-top:14px">
      <button id="emu-cancel" class="secondary small">Закрыть</button>
      <button id="emu-run" class="success small">Запустить прогон</button>
    </div>
  </div>
</div>
<div class="modal hidden" id="registry-reset-modal">
  <div class="modal-box">
    <div class="form-title">Сбросить каталог?</div>
    <div class="hint">Будут удалены все связи, события, алиасы и импортированные позиции.
      Останется только исходный каталог — 817 позиций. Затем кандидаты будут собраны заново.
      Действие необратимо.</div>
    <div class="toolbar" style="margin-top:14px;justify-content:flex-end">
      <button id="registry-reset-no" class="secondary small">Отмена</button>
      <button id="registry-reset-yes" class="small">Сбросить</button>
    </div>
  </div>
</div>
<div class="modal hidden" id="registry-import-modal">
  <div class="modal-box">
    <div class="form-title">Импорт позиций</div>
    <div class="hint">CSV: <code>item_id, title, sku, product_group, available, category</code>.<br>
      Разделитель <code>,</code> или <code>;</code>. Excel: «Сохранить как CSV». Связи и алиасы не удаляются.</div>
    <input type="file" id="registry-import-file" accept=".csv,text/csv" style="margin:10px 0">
    <div id="registry-import-result" class="hint"></div>
    <div class="toolbar" style="margin-top:12px;justify-content:flex-end">
      <button id="registry-import-no" class="secondary small">Отмена</button>
      <button id="registry-import-yes" class="small">Загрузить</button>
    </div>
  </div>
</div>
<script>
var REL_TYPES = {
  upgrade: 'Апгрейд', adjacent_task: 'Дополнение', substitute: 'Замена',
  same_task: 'Та же задача', bundle: 'Комплект', accessory: 'Аксессуар',
  workflow_next_step: 'Следующий шаг', analog: 'Аналог', excludes: 'Несовместимость'
};
var REL_STATUS = {confirmed: 'Включена', llm_suggested: 'Предложена ИИ', quarantine: 'На проверке', rejected: 'Отключена'};
var RULE_TYPES = {excludes: 'Несовместимость', out_of_stock: 'Нет в наличии', manager_only: 'Согласование', max_discount: 'Лимит скидки', export_control: 'Экспортный контроль'};
var BEHAVIORS = {cross_sell: 'Допродажа', upsell: 'Апгрейд', bundle: 'Комплект', substitute: 'Замена', analog: 'Аналог снятого', info: 'Только справка'};
var TYPE_STATUS = {active: 'Включён', draft: 'Черновик', disabled: 'Отключён'};
var WEIGHT_ROWS = [
  ['relevance', 'Релевантность'], ['urgency', 'Срочность'], ['affinity', 'Аффинность'],
  ['margin', 'Маржа'], ['confidence', 'Уверенность']
];
var state = {products: [], relations: [], rules: [], weights: {}, clients: []};
var lastOps = [];
var REG_STATUS = {candidate: 'Кандидат', active: 'Активна', rejected: 'Отклонена', stale: 'Устарела', expired: 'Истекла', superseded: 'Заменена'};
var REG_TYPE_LABEL = {
  variant_of: 'Вариант продукта', upgrade: 'Апгрейд', adjacent_task: 'Дополняет по задаче',
  accessory: 'Аксессуар', alternative: 'Альтернатива', bundle: 'Комплект',
  compatibility: 'Совместимость', renewal: 'Продление', substitute: 'Замена',
  same_task: 'Та же задача', analog: 'Аналог снятого', excludes: 'Несовместимость',
  workflow_next_step: 'Следующий шаг', info: 'Справка'
};
var REG_TYPE_COLOR = {
  variant_of: '#6366f1', adjacent_task: '#2563eb', upgrade: '#059669',
  accessory: '#0891b2', alternative: '#d97706', bundle: '#7c3aed',
  compatibility: '#64748b', renewal: '#16a34a', substitute: '#dc2626',
  same_task: '#db2777', analog: '#78716c', excludes: '#991b1b'
};
function typeColor(code){ return REG_TYPE_COLOR[code] || '#94a3b8'; }
var REG_PROV = {manual: 'Вручную', co_purchase: 'Co-purchase', llm: 'LLM', attribute: 'Атрибуты', rule: 'Правило', import: 'Импорт'};
var REG_SELECTED = {};
var REG_PAGE = {items: [], total: 0, offset: 0, limit: 200, familyLabels: {}, familyCounts: {}};
var REG_VIEW = '';

function esc(v){ return String(v == null ? '' : v).replace(/[&<>"']/g, function(c){ return {'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]; }); }
function el(id){ return document.getElementById(id); }
function title(id){ for (var i=0;i<state.products.length;i++) if (state.products[i].id===id) return state.products[i].title; return id; }
function regTypeLabel(code){
  if (REG_TYPE_LABEL[code]) return REG_TYPE_LABEL[code];
  return typeTitle(code);
}
function typeDesc(code){
  var list = state.relation_types || [];
  for (var i=0;i<list.length;i++) if (list[i].code===code) return list[i].description || '';
  return '';
}
function typeTitle(code){
  var list = state.relation_types || [];
  for (var i=0;i<list.length;i++) if (list[i].code===code) return list[i].title;
  return REL_TYPES[code] || code;
}
function activeTypes(){ return (state.relation_types||[]).filter(function(t){ return t.status==='active'; }); }
function toast(msg, err){ var t=el('toast'); t.textContent=msg; t.className='toast show'+(err?' err':''); clearTimeout(t._h); t._h=setTimeout(function(){t.className='toast';}, 2600); }
async function api(method, path, body){
  var opt = {method: method, headers: {}};
  if (body !== undefined) { opt.headers['Content-Type']='application/json'; opt.body=JSON.stringify(body); }
  var r = await fetch(path, opt);
  var txt = await r.text();
  var data = {}; try { data = txt ? JSON.parse(txt) : {}; } catch(e) { data = {error: txt}; }
  if (!r.ok) throw new Error(data.error || ('HTTP '+r.status));
  return data;
}
function options(select, items, value, label){
  select.innerHTML = '';
  items.forEach(function(x){ var o=document.createElement('option'); o.value=x[value]; o.textContent=x[label]; select.appendChild(o); });
}
function byTitle(a,b){ return a.title.localeCompare(b.title, 'ru'); }

async function load(){
  state = await api('GET','/v1/admin/state');
  state.weights = state.weights || {};
  state.weights_by_audience = state.weights_by_audience || {default: state.weights};
  state.products.sort(byTitle);
  el('st-products').textContent = state.products.length;
  el('st-relations').textContent = state.relations.length;
  el('st-rules').textContent = state.rules.length;
  el('st-active').textContent = state.relations.filter(function(r){return r.status==='confirmed';}).length;
  fillStaticSelects();
  renderRelations();
  renderTypes();
  renderRules();
  renderWeights();
  renderAudit();
  el('preview-client').innerHTML = state.clients.map(function(c){ return '<option value="'+esc(c.id)+'">'+esc(c.id+' · '+c.name)+'</option>'; }).join('');
}
function fillStaticSelects(){
  options(el('rel-from'), state.products, 'id', 'title');
  options(el('rel-to'), state.products, 'id', 'title');
  options(el('agent-from'), state.products, 'id', 'title');
  options(el('agent-to'), state.products, 'id', 'title');
  options(el('rule-product'), state.products, 'id', 'title');
  options(el('rule-other'), state.products, 'id', 'title');
  var active = activeTypes();
  var allTypes = state.relation_types || [];
  el('rel-type').innerHTML = (active.length?active:allTypes).map(function(t){ return '<option value="'+esc(t.code)+'">'+esc(t.title)+'</option>'; }).join('') ||
    Object.keys(REL_TYPES).map(function(k){ return '<option value="'+k+'">'+REL_TYPES[k]+'</option>'; }).join('');
  el('rel-status').innerHTML = Object.keys(REL_STATUS).map(function(k){ return '<option value="'+k+'">'+REL_STATUS[k]+'</option>'; }).join('');
  el('rule-type').innerHTML = Object.keys(RULE_TYPES).map(function(k){ return '<option value="'+k+'">'+RULE_TYPES[k]+'</option>'; }).join('');
  el('rel-filter-type').innerHTML = '<option value="">Все типы</option>' + allTypes.map(function(t){ return '<option value="'+esc(t.code)+'">'+esc(t.title)+'</option>'; }).join('');
  el('rel-filter-status').innerHTML = '<option value="">Все статусы</option>' + Object.keys(REL_STATUS).map(function(k){ return '<option value="'+k+'">'+REL_STATUS[k]+'</option>'; }).join('');
  if (state.products.length > 1 && el('rel-from').value === el('rel-to').value) {
    el('rel-to').selectedIndex = (el('rel-from').selectedIndex + 1) % state.products.length;
  }
  if (state.products.length > 1 && el('agent-from').value === el('agent-to').value) {
    el('agent-to').selectedIndex = (el('agent-from').selectedIndex + 1) % state.products.length;
  }
  syncRuleFields();
}
function relationPill(status){
  var cls = status==='confirmed'?'on':(status==='rejected'?'off':(status==='llm_suggested'?'ai':'q'));
  return '<span class="pill '+cls+'">'+esc(REL_STATUS[status]||status)+'</span>';
}
var lastAgentProposal = null;
function renderAgentProposal(p){
  lastAgentProposal = p;
  if (!p || p.type === 'none') {
    el('agent-result').innerHTML = '<div class="preview-item"><b>Связь не предложена</b><small>'+esc((p&&p.reason)||'Нет оснований.')+'</small></div>';
    return;
  }
  var status = p.status==='confirmed' ? 'принята автоматически' : (p.status==='llm_suggested' ? 'кандидат · нужна проверка' : 'карантин');
  var ev = (p.evidence||[]).map(function(e){ return '<div class="hint">• '+esc(e.value)+'</div>'; }).join('');
  var actions = p.status==='confirmed'
    ? '<button class="small secondary" data-agent-add="confirm">Добавить в граф</button>'
    : '<button class="small" data-agent-add="suggest">Добавить как предложение</button> '+
      '<button class="small secondary" data-agent-add="confirm">Добавить и подтвердить</button>';
  var html = '<div class="preview-item" style="border-color:#bfdbfe"><b>'+esc(p.title)+' · '+esc(p.type)+'</b>'+
    '<small>Уверенность '+Math.round((p.confidence||0)*100)+'% · '+status+' · источник: '+esc(p.source==='llm'?'ИИ':'эвристика')+'</small>'+
    '<div style="margin-top:6px">'+esc(p.reason)+'</div>'+ev+
    '<div style="margin-top:8px">'+actions+'</div></div>';
  el('agent-result').innerHTML = html;
}
async function runRelationAgent(){
  var from=el('agent-from').value, to=el('agent-to').value;
  if (!from || !to || from===to) return toast('Выберите два разных товара', true);
  el('agent-result').innerHTML='<div class="hint">Агент смотрит карточки и шаблоны…</div>';
  try { renderAgentProposal(await api('POST','/v1/admin/relations/agent/propose',{from_id:from,to_id:to})); }
  catch(e){ el('agent-result').innerHTML='<div class="hint" style="color:#b91c1c">'+esc(e.message)+'</div>'; }
}
async function addAgentRelation(mode){
  var p=lastAgentProposal; if (!p || p.type==='none') return;
  var status = mode==='confirm' ? 'confirmed' : (p.status==='llm_suggested'?'llm_suggested':(p.status==='confirmed'?'confirmed':'quarantine'));
  try {
    await api('POST','/v1/admin/relations',{from_id:p.from_id,to_id:p.to_id,type:p.type,status:status,confidence:p.confidence,reason:p.reason,evidence:p.evidence,source:'agent'});
    toast(mode==='confirm'?'Связь добавлена и подтверждена':'Связь добавлена как предложение');
    el('agent-result').innerHTML='';
    await load();
  } catch(e){ toast(e.message,true); }
}
function pct(v){ return (Number(v||0)*100).toFixed(1)+'%'; }
function reasonTitle(r){
  var map={'below_top_3':'Не вошло в топ-3','rule:already_owned':'Уже куплено','rule:audience_mismatch':'Не подходит аудитории','rule:already_have_security':'Уже есть средство защиты','rule:compatibility':'Несовместимо с окружением','rule:excludes':'Исключено правилом','rule:export_control':'Экспортный контроль','rule:out_of_stock':'Нет в наличии','rule:manager_only':'Только через менеджера'};
  return map[r] || r;
}
function productTitle(id){ return title(id) + '<div class="hint">'+esc(id)+'</div>'; }
async function loadAnalytics(){
  var days=el('analytics-days').value;
  el('analytics-cards').innerHTML='<div class="hint">Считаю…</div>';
  try {
    var d=await api('GET','/v1/admin/analytics?days='+encodeURIComponent(days));
    el('analytics-cards').innerHTML = [
      ['Запросы', d.requests], ['Показы', d.shown], ['Пропуски', d.skipped],
      ['CTR', pct(d.ctr)], ['Покупки', d.purchases], ['Конверсия', pct(d.conversion)]
    ].map(function(x){ return '<div class="stat"><b>'+esc(x[1])+'</b><span>'+esc(x[0])+'</span></div>'; }).join('');
    var products=d.top_products||[];
    el('analytics-products').innerHTML = products.length ? '<table><thead><tr><th>Товар</th><th>Показ</th><th>Клик</th><th>Покупка</th><th>CTR</th></tr></thead><tbody>'+
      products.map(function(p){ return '<tr><td>'+productTitle(p.product_id)+'</td><td>'+p.shown+'</td><td>'+p.clicks+'</td><td>'+p.purchases+'</td><td>'+pct(p.ctr)+'</td></tr>'; }).join('')+'</tbody></table>' :
      '<div class="empty">Показов пока нет</div>';
    var reasons=d.skip_reasons||[];
    el('analytics-skips').innerHTML = reasons.length ? reasons.map(function(r){
      return '<div class="rule"><div><b>'+esc(reasonTitle(r.reason))+'</b><small>'+esc(r.sample||'')+'</small></div><div class="pill type">'+r.count+'</div></div>';
    }).join('') : '<div class="empty">Пропусков пока нет</div>';
  } catch(e){ el('analytics-cards').innerHTML='<div class="hint" style="color:#b91c1c">'+esc(e.message)+'</div>'; }
  loadStrategies();
  loadHealth();
  loadRelationPerf();
  loadExperiments();
  loadKnowledge();
  loadRelationScopes();
}
async function loadHealth(){
  try {
    var d=await api('GET','/v1/admin/analytics/health?days='+encodeURIComponent(el('analytics-days').value)+'&min_shown=50');
    var types=(d.types||[]).map(function(t){ return '<span class="pill type">'+esc(t.type)+' · CTR '+pct(t.ctr)+'</span>'; }).join(' ');
    var low=(d.low_click||[]).map(function(p){
      return '<div class="rule"><div><b>'+esc(title(p.product_id))+'</b><small>'+esc(p.product_id)+' · показов '+p.shown+', кликов 0</small></div><span class="pill q">в карантин?</span></div>';
    }).join('');
    el('analytics-health').innerHTML = '<div style="margin-bottom:8px">'+types+'</div>' + (low || '<div class="empty">Явных проблем нет</div>');
  } catch(e){ el('analytics-health').innerHTML='<div class="hint" style="color:#b91c1c">'+esc(e.message)+'</div>'; }
}
var EXP_STATUS = {running:'Идёт', completed:'Завершён', stopped:'Остановлен', paused:'Пауза', draft:'Черновик'};
var EXP_DECISION = {promote:'Внедрить', reject:'Отклонить', extend:'Продлить'};
var EXP_DECISION_STATE = {promote:'Внедрена', reject:'Отклонена', extend:'Продлена'};
function expStatusLabel(s){ return EXP_STATUS[s] || s || '—'; }
function decisionPill(d){
  if(!d) return '';
  var cls = d==='promote' ? 'on' : (d==='reject' ? 'off' : 'q');
  return '<span class="pill '+cls+'">'+esc(EXP_DECISION_STATE[d]||d)+'</span>';
}
var EXPERIMENTS=[];
async function loadExperiments(){
  try {
    var d=await api('GET','/v1/admin/experiments');
    var rows=d.items||[];
    EXPERIMENTS=rows;
    var running=rows.filter(function(x){return x.status==='running'&&!x.decision;}).length;
    var promoted=rows.filter(function(x){return x.decision==='promote';}).length;
    var rejected=rows.filter(function(x){return x.decision==='reject';}).length;
    var decided=rows.filter(function(x){return x.decision;}).length;
    el('experiments-summary').innerHTML = [
      ['Активных', running], ['Решений', decided], ['Внедрено', promoted], ['Отклонено', rejected]
    ].map(function(x){ return '<div class="stat"><b>'+x[1]+'</b><span>'+x[0]+'</span></div>'; }).join('');
    if(!rows.length){ el('experiments-list').innerHTML='<div class="empty">Гипотез пока нет — добавьте гипотезу.</div>'; return; }
    el('experiments-list').innerHTML=rows.map(function(x){
      var res=x.results||[];
      var c=res.find(function(r){return r.variant==='control'})||{exposures:0,clicks:0,purchases:0,ctr:0,conversion:0};
      var t=res.find(function(r){return r.variant==='treatment'})||{exposures:0,clicks:0,purchases:0,ctr:0,conversion:0};
      var uplift=c.conversion>0?(t.conversion-c.conversion)/c.conversion*100:null;
      var delta=(t.conversion-c.conversion)*100;
      var maxConv=Math.max(c.conversion,t.conversion,0.01);
      var g=x.guardrail||{};
      var gBadge=g.blocked
        ? '<span class="pill" style="background:#fef3c7;color:#92400e">guardrail: '+esc(g.suggested||'extend')+'</span>'
        : '<span class="pill" style="background:#dcfce7;color:#166534">guardrail ok</span>';
      var reasons=(g.reasons||[]).join('; ');
      var guardWarn=g.blocked?'<div class="hint" style="color:#92400e;margin-top:6px">Стоп: '+esc(reasons||'сработал guardrail')+' → рекомендуем '+esc(EXP_DECISION[g.suggested]||g.suggested||'продлить')+'. Внедрение заблокировано.</div>':'';
      var promoteDisabled=g.blocked?' disabled':'';
      var actions='';
      if(!x.decision){
        actions='<div class="toolbar" style="margin-top:10px">'+
          '<button data-exp-decision="promote" data-exp-id="'+esc(x.id)+'"'+promoteDisabled+'>Внедрить</button>'+
          '<button class="ghost" data-exp-decision="reject" data-exp-id="'+esc(x.id)+'">Отклонить</button>'+
          '<button class="secondary" data-exp-decision="extend" data-exp-id="'+esc(x.id)+'">Продлить</button>'+
          (x.status==='running'?'<button class="secondary" data-exp-emulate="'+esc(x.id)+'">Прогнать на эмуляторе</button>':'')+
          (x.status==='running'?'<button class="secondary small" data-exp-monitor="'+esc(x.id)+'">Проверить guardrails</button>':'')+
        '</div>';
      }
      var table='<div class="tablewrap" style="margin-top:10px"><table><thead><tr><th>Вариант</th><th>Exposure</th><th>Клиенты</th><th>Клики</th><th>Покупки</th><th>CTR</th><th>Conversion</th><th>Target shown</th><th>Target conv</th></tr></thead><tbody>'+
        [['control',c],['treatment',t]].map(function(pair){
          var r=pair[1];
          return '<tr><td><b>'+esc(pair[0])+'</b></td><td>'+r.exposures+'</td><td>'+r.clients+'</td><td>'+r.clicks+'</td><td>'+r.purchases+'</td><td>'+pct(r.ctr)+'</td><td>'+pct(r.conversion)+'</td><td>'+r.target_exposures+'</td><td>'+pct(r.target_conversion)+'</td></tr>';
        }).join('')+'</tbody></table></div>';
      var emuLine=x.emulation?'<div class="hint" style="margin-top:2px">Эмуляция: профиль «'+esc(x.emulation.profile_name)+'», '+x.emulation.impressions+' показов · <b>попадание '+pct(x.emulation.fit)+'</b></div>':'';
      var kpis='<div class="exp-kpis">'+
        '<div class="exp-kpi"><b>'+pct(c.conversion)+'</b><span>control · conversion</span></div>'+
        '<div class="exp-kpi"><b>'+pct(t.conversion)+'</b><span>treatment · conversion</span></div>'+
        '<div class="exp-kpi"><b style="color:'+(delta>=0?'#047857':'#b91c1c')+'">'+(delta>=0?'+':'')+delta.toFixed(1)+' п.п.</b><span>Δ к контролю</span></div>'+
        '<div class="exp-kpi"><b style="color:'+(uplift===null?'#64748b':(uplift>=0?'#047857':'#b91c1c'))+'">'+(uplift===null?'н/д':(uplift>=0?'+':'')+uplift.toFixed(1)+'%')+'</b><span>uplift</span></div>'+
      '</div>';
      var bars='<div class="exp-bars">'+
        '<div class="exp-bar control"><span>control</span><div class="track"><i style="width:'+(c.conversion/maxConv*100).toFixed(1)+'%"></i></div><b>'+pct(c.conversion)+'</b></div>'+
        '<div class="exp-bar"><span>treatment</span><div class="track"><i style="width:'+(t.conversion/maxConv*100).toFixed(1)+'%"></i></div><b>'+pct(t.conversion)+'</b></div>'+
      '</div>';
      return '<div class="formbox" style="margin-bottom:14px">'+
        '<div class="exp-head"><div><div class="form-title" style="margin:0">'+esc(x.title)+'</div>'+
          '<div class="hint">'+esc(x.hypothesis)+'</div></div>'+
          '<div style="display:flex;gap:6px;align-items:center;flex-wrap:wrap">'+gBadge+'<span class="pill">'+esc(expStatusLabel(x.status))+'</span>'+decisionPill(x.decision)+'</div></div>'+
        '<div class="hint" style="margin:8px 0">Скоуп: '+esc(x.audience||'все')+' / '+esc(x.industry||'все')+' / '+esc(x.strategy||'все')+' · '+
          esc(x.from_title||x.from_id)+' → '+esc(x.to_title||x.to_id)+' · метрика: '+esc(x.metric)+'</div>'+
        emuLine+kpis+bars+table+
        '<div class="hint" style="margin-top:8px">Guardrails: renewal '+pct(c.renewal_rate)+' → '+pct(t.renewal_rate)+
          ', upsell '+pct(c.upsell_rate)+' → '+pct(t.upsell_rate)+
          ', fallback '+pct(c.fallback_rate)+' → '+pct(t.fallback_rate)+'</div>'+
        guardWarn+actions+
        (x.decision?'<div class="hint" style="margin-top:6px">Решение: '+decisionPill(x.decision)+' '+esc(x.decision_note||'')+'</div>':'')+
      '</div>';
    }).join('');
  } catch(e){ el('experiments-list').innerHTML='<div class="hint" style="color:#b91c1c">'+esc(e.message)+'</div>'; }
}
async function loadKnowledge(){
  try {
    var q=encodeURIComponent((el('experiment-search')||{}).value||'');
    var dec=encodeURIComponent((el('experiment-decision-filter')||{}).value||'');
    var d=await api('GET','/v1/admin/experiments/knowledge?q='+q+'&decision='+dec);
    var rows=d.items||[];
    var title='<div class="form-title" style="margin-top:20px">База знаний</div>';
    if(!rows.length){ el('experiment-knowledge').innerHTML=title+'<div class="empty">Пока нет завершённых гипотез</div>'; return; }
    var counts={promote:0,reject:0,extend:0};
    rows.forEach(function(x){ if(counts[x.decision]!==undefined) counts[x.decision]++; });
    var summary='<div class="stats">'+
      '<div class="stat"><b>'+rows.length+'</b><span>завершённых гипотез</span></div>'+
      '<div class="stat"><b>'+counts.promote+'</b><span>внедрено</span></div>'+
      '<div class="stat"><b>'+counts.reject+'</b><span>отклонено</span></div>'+
      '<div class="stat"><b>'+counts.extend+'</b><span>продлено</span></div></div>';
    el('experiment-knowledge').innerHTML=title+summary+rows.map(function(x){
      return '<div class="formbox" style="margin-bottom:8px">'+
        '<div class="exp-head"><b>'+esc(x.title)+'</b>'+decisionPill(x.decision)+'</div>'+
        '<div class="hint" style="margin-top:4px">'+esc(x.hypothesis)+'</div>'+
        '<div class="hint">'+esc(x.audience||'все')+' / '+esc(x.industry||'все')+' / '+esc(x.strategy||'все')+' · '+esc(x.from_title||x.from_id)+' → '+esc(x.to_title||x.to_id)+'</div>'+
        (x.decision_note?'<div class="hint" style="margin-top:4px">'+esc(x.decision_note)+'</div>':'')+
      '</div>';
    }).join('');
  } catch(e){ el('experiment-knowledge').innerHTML='<div class="hint" style="color:#b91c1c">'+esc(e.message)+'</div>'; }
}
async function loadRelationPerf(){
  try {
    var d=await api('GET','/v1/admin/relations/performance');
    var rows=d.items||[];
    el('relation-perf').innerHTML = rows.length ? '<div class="tablewrap"><table><thead><tr><th>Связь</th><th>Тип</th><th>Скоуп</th><th>Показ</th><th>Клик</th><th>CTR</th><th>Вес</th><th>Покупка</th><th>Статус</th><th>До</th></tr></thead><tbody>'+
      rows.map(function(x){ return '<tr><td>'+esc(x.title_from)+' → '+esc(x.title_to)+'</td><td><span class="pill type">'+esc(x.type)+'</span></td><td>'+esc(x.audience||'—')+' / '+esc(x.industry||'—')+'</td><td>'+x.shown+'</td><td>'+x.clicks+'</td><td>'+pct(x.ctr)+'</td><td>'+Math.round((x.weight||0)*100)+'%</td><td>'+x.purchases+'</td><td>'+esc(x.status)+'</td><td>'+(x.valid_until?new Date(x.valid_until).toLocaleDateString('ru-RU'):'—')+'</td></tr>'; }).join('')+'</tbody></table></div>' :
      '<div class="empty">Пока нет показов с привязкой к связи</div>';
  } catch(e){ el('relation-perf').innerHTML='<div class="hint" style="color:#b91c1c">'+esc(e.message)+'</div>'; }
}
async function loadRelationScopes(){
  try {
    var d=await api('GET','/v1/admin/relations/scopes');
    var rows=d.items||[];
    el('relation-scopes').innerHTML = rows.length ? '<div class="tablewrap"><table><thead><tr><th>Связь</th><th>Тип</th><th>B2B/B2C</th><th>Отрасль</th><th>Статус</th><th>До</th><th>Проверено</th><th></th></tr></thead><tbody>'+
      rows.map(function(x){
        var attrs='data-scope-from="'+esc(x.from_id)+'" data-scope-to="'+esc(x.to_id)+'" data-scope-type="'+esc(x.type)+'" data-scope-audience="'+esc(x.audience)+'" data-scope-industry="'+esc(x.industry)+'"';
        var st=esc(x.status)+(x.expired?' <span class="pill" style="background:#fef3c7;color:#92400e">истёк</span>':'');
        return '<tr><td>'+esc(x.title_from)+' → '+esc(x.title_to)+'</td><td><span class="pill type">'+esc(x.type)+'</span></td><td>'+esc(x.audience||'все')+'</td><td>'+esc(x.industry||'все')+'</td><td>'+st+'</td><td>'+(x.valid_until?new Date(x.valid_until).toLocaleDateString('ru-RU'):'—')+'</td><td>'+esc(x.reviewed_by||'—')+'</td><td><button class="small" data-scope-review="confirm" '+attrs+'>✓</button> <button class="ghost small" data-scope-review="reject" '+attrs+'>✕</button> <button class="secondary small" data-scope-review="reopen" '+attrs+'>тест</button></td></tr>';
      }).join('')+'</tbody></table></div>' :
      '<div class="empty">Скоупов пока нет: связь живёт по глобальному статусу</div>';
  } catch(e){ el('relation-scopes').innerHTML='<div class="hint" style="color:#b91c1c">'+esc(e.message)+'</div>'; }
}
async function loadStrategies(){
  try {
    var d=await api('GET','/v1/admin/strategies');
    el('auto-strategy').checked=!!d.auto;
    var rows=d.stats||[];
    el('analytics-strategies').innerHTML = rows.length ? '<table><thead><tr><th>Сегмент</th><th>Стратегия</th><th>Показ</th><th>Клик</th><th>Покупка</th><th>CTR</th></tr></thead><tbody>'+
      rows.map(function(s){ return '<tr><td>'+esc(s.audience)+'</td><td><b>'+esc(s.strategy)+'</b></td><td>'+s.shown+'</td><td>'+s.clicks+'</td><td>'+s.purchases+'</td><td>'+pct(s.ctr)+'</td></tr>'; }).join('')+'</tbody></table>' :
      '<div class="empty">Пока нет данных по стратегиям</div>';
  } catch(e){ el('analytics-strategies').innerHTML='<div class="hint" style="color:#b91c1c">'+esc(e.message)+'</div>'; }
}
var reviewItems = [];
function reviewStatus(code){ return code==='quarantine'?'На проверке':(code==='llm_suggested'?'Предложена ИИ':code); }
function updateReviewBadge(counts){
  var b=el('review-badge');
  if (!b) return;
  var n=(counts.quarantine||0)+(counts.llm_suggested||0);
  b.textContent=n; b.classList.toggle('hidden', n===0);
}
function renderReview(){
  var q=el('review-search').value.trim().toLowerCase();
  var items=reviewItems.filter(function(r){
    if (!q) return true;
    return (r.from_title+' '+r.to_title+' '+r.from_id+' '+r.to_id).toLowerCase().indexOf(q)>=0;
  });
  el('review-all').checked=false;
  el('review-list').innerHTML = items.length ? items.map(function(r,i){
    var idx=reviewItems.indexOf(r);
    var ev=(r.evidence||[]).map(function(e){ return esc(e.value); }).join(' · ');
    var cls=r.status==='quarantine'?'q':'ai';
    return '<div class="type-card"><div class="type-head">'+
      '<input type="checkbox" data-review-check="'+idx+'" style="width:auto" checked> '+
      '<b>'+esc(r.from_title||r.from_id)+' → '+esc(r.to_title||r.to_id)+'</b>'+
      '<span class="pill type">'+esc(r.type_title||r.type)+'</span>'+
      '<span class="pill '+cls+'">'+esc(reviewStatus(r.status))+'</span>'+
      '<span class="hint">'+Math.round((r.confidence||0)*100)+'% · '+esc(r.source||'—')+'</span></div>'+
      (r.reason?'<div style="margin-top:6px">'+esc(r.reason)+'</div>':'')+
      (ev?'<div class="hint">Evidence: '+ev+'</div>':'')+
      '<div style="margin-top:8px"><button class="small" data-review-one="confirm" data-review-index="'+idx+'">Подтвердить</button> '+
      '<button class="ghost small" data-review-one="reject" data-review-index="'+idx+'">Отклонить</button></div></div>';
  }).join('') : '<div class="empty">Очередь пуста — все связи проверены</div>';
}
async function loadReview(){
  try {
    var status=el('review-filter').value;
    var d=await api('GET','/v1/admin/relations/review?limit=200&status='+encodeURIComponent(status));
    reviewItems=d.items||[];
    updateReviewBadge(d.counts||{});
    renderReview();
  } catch(e){ toast('Очередь валидации: '+e.message, true); }
}
async function reviewDecision(decision, items){
  if (!items.length) return toast('Ничего не выбрано', true);
  var payload={decision:decision, note:el('review-note').value.trim(), items:items.map(function(r){ return {from_id:r.from_id,to_id:r.to_id,type:r.type}; })};
  try {
    var d=await api('POST','/v1/admin/relations/review',payload);
    toast('Проверено связей: '+d.reviewed);
    el('review-note').value='';
    await load();
    await loadReview();
  } catch(e){ toast(e.message,true); }
}
async function loadVendorKnowledge(){
  try {
    var d=await api('GET','/v1/admin/vendor-products');
    var s=d.stats||{};
    el('vendor-stats').innerHTML=[
      ['Внешних продуктов', s.products||0], ['Сопоставлено', s.mapped_products||0],
      ['Покрыто SKU', s.covered_items||0], ['Из них без связей', s.solo_covered||0],
      ['Классов ПО', s.classes||0], ['Товаров с классом', s.classified_items||0]
    ].concat(d.fstec ? [
      ['Сертификаты ФСТЭК', d.fstec.certificates||0],
      ['Сопоставлено', d.fstec.matched_certificates||0],
      ['Товаров с сертификатом', d.fstec.covered_items||0],
      ['Просрочено', d.fstec.expired||0]
    ] : []).map(function(x){ return '<div class="stat"><b>'+x[1]+'</b><span>'+x[0]+'</span></div>'; }).join('');
    var classes=d.classes||[];
    el('vendor-classes').innerHTML = classes.length ? '<div class="tablewrap" style="margin-top:10px;min-height:0"><table><thead><tr><th>Код</th><th>Класс ПО</th><th>Продуктов</th><th>Товаров</th></tr></thead><tbody>'+
      classes.map(function(c){ return '<tr><td><b>'+esc(c.code)+'</b></td><td>'+esc(String(c.title||'').replace(/^[^-]+ - /,''))+'</td><td>'+c.products+'</td><td>'+c.items+'</td></tr>'; }).join('')+'</tbody></table></div>' : '';
    var sources=d.sources||[];
    var SRC_STATUS={connected:['Подключён','#dcfce7','#166534'],available:['Доступен','#eef2ff','#2563eb'],blocked:['Заблокирован','#fee2e2','#991b1b'],restricted:['Ограничен','#f1f5f9','#475569']};
    el('vendor-sources').innerHTML = sources.length ? '<div class="tablewrap" style="margin-top:4px;min-height:0"><table><thead><tr><th>Источник</th><th>Тип</th><th>Статус</th><th>Что даёт</th><th>Ограничение</th></tr></thead><tbody>'+
      sources.map(function(x){ var st=SRC_STATUS[x.status]||[x.status,'#f1f5f9','#475569']; var name=x.url?'<a href="'+esc(x.url)+'" target="_blank" rel="noopener">'+esc(x.name)+'</a>':esc(x.name); return '<tr><td><b>'+name+'</b></td><td>'+esc(x.kind)+'</td><td><span class="pill" style="background:'+st[1]+';color:'+st[2]+'">'+esc(st[0])+'</span></td><td>'+esc(x.gives)+'</td><td class="hint">'+esc(x.limitation)+'</td></tr>'; }).join('')+'</tbody></table></div>' : '';
  } catch(e){ el('vendor-stats').innerHTML='<div class="hint" style="color:#b91c1c">'+esc(e.message)+'</div>'; }
}
async function loadDataset(){
  try {
    var c=await api('GET','/v1/admin/dataset/coverage');
    var m=await api('GET','/v1/admin/registry/metrics');
    var f=await api('GET','/v1/admin/registry/feed');
    loadVendorKnowledge();
    var volumes = [
      ['Товары', c.items], ['Клиенты', c.clients],
      ['Заказы', c.orders], ['Инстансы заказов', c.order_instances],
      ['Co-purchase пары', c.pairs], ['Публичные пары', c.public_pairs],
      ['B2C товары', c.b2c_items], ['Без связей', c.solo_items]
    ];
    el('dataset-cards').innerHTML = volumes.map(function(x){
      return '<div class="stat"><b>'+esc(x[1])+'</b><span>'+esc(x[0])+'</span></div>';
    }).join('');
    var live = [
      ['Связей', m.total], ['Активных', m.active],
      ['Кандидатов', m.candidates], ['Покрытие', (m.coverage_pct||0).toFixed(1)+'%'],
      ['Stale', (m.stale_share||0).toFixed(1)+'%'], ['Хабы top-1%', (m.hub_top1_pct||0).toFixed(1)+'%']
    ];
    el('dataset-registry').innerHTML = live.map(function(x){
      return '<div class="stat"><b>'+esc(x[1])+'</b><span>'+esc(x[0])+'</span></div>';
    }).join('');
    var feedDate = f.updated_at ? new Date(f.updated_at).toLocaleDateString('ru-RU') : '';
    el('dataset-feed').innerHTML = 'Фид: доступно <b>'+f.available+'</b> из '+f.total+
      ' · обновлён: '+(feedDate || 'дата не передана (исторический снимок)');
  } catch(e){ el('dataset-cards').innerHTML='<div class="hint" style="color:#b91c1c">'+esc(e.message)+'</div>'; }
}
el('vendor-import').addEventListener('click', async function(){
  var f=el('vendor-file').files[0];
  if(!f){ el('vendor-result').textContent='Выберите CSV'; return; }
  var fd=new FormData(); fd.append('file',f);
  try {
    var r=await fetch('/v1/admin/vendor-products/import',{method:'POST',body:fd});
    var d=await r.json();
    if(!r.ok) throw new Error(d.error||('HTTP '+r.status));
    el('vendor-result').textContent='Импорт: добавлено '+d.added+', обновлено '+d.updated+', пропущено '+d.skipped;
    loadVendorKnowledge();
  } catch(e){ el('vendor-result').textContent=e.message; }
});
el('registry-import').addEventListener('click', async function(){
  var f=el('registry-file').files[0];
  if(!f){ el('vendor-result').textContent='Выберите XML реестра'; return; }
  var fd=new FormData(); fd.append('file',f);
  try {
    var r=await fetch('/v1/admin/vendor-products/import-registry',{method:'POST',body:fd});
    var d=await r.json();
    if(!r.ok) throw new Error(d.error||('HTTP '+r.status));
    el('vendor-result').textContent='Реестр: добавлено '+d.added+', обновлено '+d.updated+', пропущено '+d.skipped;
    loadVendorKnowledge();
  } catch(e){ el('vendor-result').textContent=e.message; }
});
el('vendor-candidates').addEventListener('click', async function(){
  try {
    var d=await api('POST','/v1/admin/vendor-products/candidates',{limit:50});
    el('vendor-result').textContent='Создано кандидатов: '+d.created+', пропущено: '+d.skipped;
    loadVendorKnowledge();
  } catch(e){ el('vendor-result').textContent=e.message; }
});
el('vendor-link').addEventListener('click', async function(){
  try {
    var d=await api('POST','/v1/admin/vendor-products/link',{});
    el('vendor-result').textContent='Сопоставлено: '+d.mappings+' связей, '+d.products+' продуктов, '+d.items+' SKU';
    loadVendorKnowledge();
  } catch(e){ el('vendor-result').textContent=e.message; }
});
el('dataset-open-catalog').addEventListener('click', function(){
  document.querySelector('[data-tab=registry]').click();
  el('registry-status').value='candidate';
  loadRegistry();
});
function regStatusLabel(s){ return REG_STATUS[s] || s || '—'; }
function familyLabel(id){ return REG_PAGE.familyLabels[id] || id || '—'; }
function familyCount(id){ return REG_PAGE.familyCounts[id] || 0; }
function updateConfLabel(){
  var v=parseInt(el('registry-conf-min').value,10)||0;
  el('registry-conf-label').textContent = v===0 ? 'любая' : '≥ '+v+'%';
}
function confMinValue(){
  var v=parseInt(el('registry-conf-min').value,10);
  if (isNaN(v) || v<0) v=0;
  if (v>100) v=100;
  return v/100;
}
function confBand(c){
  c = Number(c) || 0;
  if (c >= 0.80) return {label:'высокая', cls:'hi'};
  if (c >= 0.60) return {label:'средняя', cls:'mid'};
  return {label:'низкая', cls:'lo'};
}
function confMeter(c){
  var b=confBand(c);
  return '<span class="conf-meter '+b.cls+'" title="уверенность: '+b.label+', '+Number(c||0).toFixed(2)+'">'+
    '<i></i><i></i><i></i></span>';
}
function confText(c){
  var b=confBand(c);
  return b.label+' ('+Number(c||0).toFixed(2)+')';
}
function positionRegistryTip(t, x, y){
  var w=t.offsetWidth, h=t.offsetHeight;
  var left=Math.min(x+14, window.innerWidth-w-8);
  var top=y+16;
  if (top+h > window.innerHeight-8) top=y-h-10;
  t.style.left=Math.max(8,left)+'px';
  t.style.top=Math.max(8,top)+'px';
}
function showRegistryTip(text, x, y){
  var t=el('registry-tip');
  if(!text){ t.classList.remove('on'); return; }
  t.textContent=text;
  t.classList.add('on');
  positionRegistryTip(t, x, y);
}
function hideRegistryTip(){ el('registry-tip').classList.remove('on'); }
function regProvLabel(p){ return REG_PROV[p] || p || '—'; }
function regWhen(v){ if(!v) return '—'; var d=new Date(v); return isNaN(d.getTime()) ? '—' : d.toLocaleDateString('ru-RU'); }
var EV_KIND = {attribute: 'Атрибут', co_purchase: 'Co-purchase', llm: 'LLM', manual: 'Ручная', rule: 'Правило', spec: 'Спецификация', vendor_doc: 'Документация вендора', product_page: 'Страница товара'};
function evidenceText(e){
  if(!e) return '';
  var kind=e.kind||'', d=e.details||{};
  if(kind==='attribute'){
    var gid=d.product_group||'';
    if(!gid && e.summary){ var m=/product_group=([^;,\s]+)/.exec(e.summary); if(m) gid=m[1]; }
    var label=gid?familyLabel(gid):'';
    var n=gid?familyCount(gid):0;
    return 'Одна группа: '+(label||gid||e.summary||'—')+(n?' · '+n+' тов.':'');
  }
  if(kind==='co_purchase'){
    var orders=d.orders, b2b=d.b2b;
    if(orders) return 'Куплено вместе: '+orders+' зак.'+(b2b?', B2B '+b2b:'');
    return e.summary || 'совместные покупки';
  }
  if(kind==='manual') return e.summary ? 'Ручное подтверждение: '+e.summary : 'Ручное подтверждение';
  if(kind==='llm') return 'LLM: '+(e.summary || 'предложение модели');
  return (EV_KIND[kind]?EV_KIND[kind]+': ':'')+(e.summary || '');
}
function commonPair(from, to){
  var n=Math.min(from.length,to.length), p=0;
  while(p<n && from[p]===to[p]) p++;
  var s=0;
  while(s<n-p && from[from.length-1-s]===to[to.length-1-s]) s++;
  function boundary(ch){ return ch===' '||ch==='-'||ch==='/'||ch==='('||ch===')'||ch===','; }
  while(p>0 && !boundary(from[p-1])) p--;
  while(s>0 && !boundary(from[from.length-s])) s--;
  return {p:p, s:s};
}
var LEAD_PHRASES = [
  /^\([^)]*\)\s*/,
  /^предоставление доступа к сервисам\s+/i,
  /^предоставление права использования\s+/i,
  /^право на использование\s+/i,
  /^право использования программы для эвм\s+/i,
  /^лицензия на право использования\s+/i,
  /^неисключительное право (на )?использования\s+/i,
  /^дистрибутив программного обеспечения\s+/i,
  /^дистрибутив\s+/i,
  /^программа фиксации и контроля исходного состояния программного комплекса\s+/i,
  /^установочный комплект[.,]?\s*/i,
  /^сертификат на сопровождение по\s+/i,
  /^базовый пакет для сертифицированной версии программного комплекса\s+/i,
  /^базовый пакет для сертифицированной версии\s+/i,
  /^базовый пакет для сертифицированного\s+/i,
  /^средство создания модели системы разграничения доступа\s+/i,
  /^сертификат на годовую техническую поддержку\s*/i,
  /^сертификат на техническую поддержку\s*/i,
  /^сертификат на сопровождение\s*/i
];
var TAIL_PHRASES = [
  /\s*[-–]?\s*лицензия.*$/i,
  /\s*формуляр$/i,
  /\s*установочный комплект.*$/i,
  /\s*all lng.*$/i,
  /\s*download pack.*$/i,
  /\s*\d+-device.*$/i,
  /\s*\d+ year.*$/i,
  /\s*на \d+ учетн.*$/i,
  /,?\s*на \d+ месяц(а|ев)?.*$/i,
  /\s*на неограниченное количество.*$/i,
  /\s*на\s+dvd.*$/i,
  /\s*\.?\s*формуляр.*$/i,
  /,\s*продление.*$/i,
  /,\s*лицензия.*$/i,
  /,\s*клиентская.*$/i,
  /,\s*серверная.*$/i,
  /,\s*неисключительное право.*$/i,
  /,\s*простая.*$/i,
  /\s*право на использование.*$/i,
  /\s*[-–]?\s*неисключительное право.*$/i,
  /\s*[-–]?\s*неисключительное.*$/i,
  /\s*[-–]?\s*простая.*$/i
];
var WORD_REPLACEMENTS = [
  [/google workspace/gi,'GWS'], [/microsoft 365/gi,'M365'], [/microsoft/gi,'MS'],
  [/kaspersky endpoint security/gi,'KES'], [/kaspersky small office security/gi,'KSOS'],
  [/российская|russian/gi,''], [/edition/gi,''],
  [/версии|версия/gi,''], [/исполнения|исполнение/gi,''],
  [/для бизнеса|бизнеса/gi,'Bus.'],
  [/стандартный|standard/gi,'Std'],
  [/с расширенным функционалом|расширенным функционалом/gi,'Adv'],
  [/расширенный|расширенным|расширенного|advanced/gi,'Adv'],
  [/pack/gi,''],
  [/for desktops and mobiles/gi,''],
  [/(\d+)-mobile device/gi,'$1Mob'],
  [/(\d+)-desktop/gi,'$1Desk'],
  [/(\d+)-user/gi,'$1User'],
  [/\bprofessional\b/gi,'Pro'], [/\benterprise\b/gi,'Ent'],
  [/\bbusiness\b/gi,'Bus.'], [/\badditional\b/gi,'Add'],
  [/\bconcurrent\b/gi,'Conc'], [/\bcertified\b/gi,'Cert'],
  [/\bsecurity\b/gi,'Sec'],
  [/на сервере/gi,'Server'], [/на рабочем месте/gi,'Workstation']
];
function qualifierOf(t){
  t=String(t||'').trim();
  var qual='', m=t.match(/^\(([^)]*)\)\s*/);
  if (m){
    var inner=m[1];
    if (/\d+\s*\/\s*\d+|помесячн|первый год|второй год/i.test(inner)){
      var d=inner.match(/\d+\s*\/\s*\d+/);
      qual=d?d[0].replace(/\s+/g,''):'';
      t=t.slice(m[0].length);
    }
  }
  return {qual:qual, rest:t};
}
function editionTag(s){
  s=String(s||'').toLowerCase();
  if (/renewal|продление/.test(s)) return 'Renewal';
  if (/\bbase\b/.test(s)) return 'Base';
  if (/upgrade|апгрейд/.test(s)) return 'Upgrade';
  if (/subscription|подписк/.test(s)) return 'Subscr';
  return '';
}
function licenseKind(s){
  s=String(s||'').toLowerCase();
  if (/установочн|дистрибутив|download|dwnl/.test(s)) return 'distr';
  if (/клиентск/.test(s)) return 'client';
  if (/серверн/.test(s)) return 'server';
  if (/формуляр/.test(s)) return 'form';
  if (/лицензи/.test(s)) return 'license';
  return '';
}
function shortTitle(t){
  var original=String(t||'').replace(/[«»"]/g,' ').replace(/\s+/g,' ').trim();
  var text=original;
  if (/^сертификат на годовую техническую поддержку/i.test(text)) text=text.replace(/^сертификат на годовую техническую поддержку\s*/i,'Техподдержка 1 год ');
  else if (/^сертификат на техническую поддержку/i.test(text)) text=text.replace(/^сертификат на техническую поддержку\s*/i,'Техподдержка ');
  else if (/^сертификат на сопровождение/i.test(text)) text=text.replace(/^сертификат на сопровождение\s*/i,'Сопровождение ');
  LEAD_PHRASES.forEach(function(re){ text=text.replace(re,''); });
  WORD_REPLACEMENTS.forEach(function(pair){ text=text.replace(pair[0], pair[1]); });
  TAIL_PHRASES.forEach(function(re){ text=text.replace(re,''); });
  text=text.replace(/\(\s*([\d.]+)\s*\)/g,' $1').replace(/\(\s*[-–]\s*/g,'(').replace(/\s*[-–]\s*\)/g,')').replace(/\(\s*\)/g,'')
           .replace(/\s*[-–]\s*$/,'')
           .replace(/\s{2,}/g,' ')
           .trim();
  text=text.split(' ').filter(function(w,i,arr){
    function norm(x){ return x.toLowerCase().replace(/[.,;:]+$/,''); }
    return i===0 || norm(w)!==norm(arr[i-1]);
  }).join(' ');
  text=text.replace(/\s+([.,;:])/g,'$1').replace(/\.\s*\./g,'.').replace(/([A-ZА-ЯЁ]{2,})\./g,'$1').trim();
  text=text.split(' ').filter(function(w,i,arr){
    function norm(x){ return x.toLowerCase().replace(/[.,;:]+$/,''); }
    return i===0 || norm(w)!==norm(arr[i-1]);
  }).join(' ');
  if (!text) text=original;
  if (text.length > 40){
    var vt='', brand='';
    var BRAND_SKIP={'СКЗИ':1,'ПО':1,'ЭВМ':1,'ОС':1,'ПК':1,'АРМ':1,'БД':1,'СУБД':1,'API':1,'SDK':1,'CPU':1,'RAM':1,'РФ':1,'СС':1,'CSP':1,'CAL':1,'ESD':1,'OEM':1,'DVD':1,'CD':1,'USB':1,'BOX':1,'PDF':1,'DOC':1,'XLS':1};
    var parts=text.split(' ');
    for (var bi=0;bi<parts.length;bi++){
      var bk=parts[bi].replace(/^[.,;:()\-]+|[.,;:()\-]+$/g,'');
      if (/^\d+(\.\d+)+$/.test(bk) && !vt) vt=bk;
      if (/^[А-ЯЁA-Z]{3,10}$/.test(bk) && !BRAND_SKIP[bk]) brand=bk;
    }
    if (brand && vt) text=brand+' '+vt;
  }
  if (text.length > 52){
    text=text.slice(0,52);
    var cut=text.lastIndexOf(' ');
    if (cut > 20) text=text.slice(0,cut);
    text=text.trim()+'…';
  }
  return text;
}
function pairCell(from, to, fromShort, toShort){
  from=String(from||''); to=String(to||'');
  var full=esc(from)+' → '+esc(to);
  var qa=qualifierOf(from), qb=qualifierOf(to);
  var a=fromShort || shortTitle(qa.rest);
  var b=toShort || shortTitle(qb.rest);
  if (!fromShort && !toShort){
    var ea=editionTag(from), eb=editionTag(to);
    if (ea && a.toLowerCase().indexOf(ea.toLowerCase())<0) a+=' · '+ea;
    if (eb && b.toLowerCase().indexOf(eb.toLowerCase())<0) b+=' · '+eb;
    var ka=licenseKind(from), kb=licenseKind(to);
    if (ka && kb && ka!==kb){ a+=' · '+ka; b+=' · '+kb; }
    if (a===b){ a=qa.rest; b=qb.rest; }
  }
  var fromText='<span class="pair-text" data-full="'+esc(from)+'" data-short="'+esc(a)+'">'+esc(a)+'</span>';
  var toText='<span class="pair-text" data-full="'+esc(to)+'" data-short="'+esc(b)+'">'+esc(b)+'</span>';
  var fromLine, toLine;
  if (qa.qual && qa.qual===qb.qual){
    fromLine='<span class="pair-qual">'+esc(qa.qual)+'</span> '+fromText;
  } else {
    fromLine=(qa.qual?'<span class="pair-qual">'+esc(qa.qual)+'</span> ':'')+fromText;
    toText=(qb.qual?'<span class="pair-qual">'+esc(qb.qual)+'</span> ':'')+toText;
  }
  return '<span class="pair" data-tip="'+full+'"><span class="pair-from">'+fromLine+'</span><span class="pair-to"><span class="arrow">→</span>'+toText+'</span></span>';
}
function evidenceCell(ev){
  if(!ev || !ev.length) return '—';
  var first=ev[0], kind=first.kind||'';
  var more=ev.length>1 ? '<span class="ev-more">+'+(ev.length-1)+'</span>' : '';
  return '<span class="ev" data-tip="'+esc(evidenceText(first))+'" style="cursor:help"><i class="ev-dot ev-'+esc(kind)+'"></i><span class="clip clip-sm">'+esc(evidenceText(first))+'</span>'+more+'</span>';
}
function evidenceList(ev){
  if(!ev || !ev.length) return '<div class="hint">Доказательств нет — связь создана вручную.</div>';
  return '<div class="ev-list">'+ev.map(function(e){
    var text=esc(evidenceText(e));
    var url=e.details&&e.details.url;
    var body=url?'<a href="'+esc(url)+'" target="_blank" rel="noopener">'+text+'</a>':text;
    return '<div class="ev-row"><i class="ev-dot ev-'+esc(e.kind||'')+'"></i><span class="ev-kind">'+esc(EV_KIND[e.kind]||e.kind||'—')+'</span><span>'+body+'</span></div>';
  }).join('')+'</div>';
}
function evidenceNote(r){
  if(r.type==='variant_of') return 'Структурная связь: оба SKU входят в одну продуктовую группу, поэтому это вариант одного продукта, а не новый продукт.';
  if(r.type==='adjacent_task') return 'Связь предложена по совместным покупкам. Подтвердите, что это дополнение по задаче, а не случайное совпадение в заказе.';
  if(r.type==='upgrade') return 'Связь означает переход на старшую редакцию или тариф того же продукта.';
  if(r.type==='accessory') return 'Связь означает, что один товар нужен для работы другого.';
  return '';
}
function registryQuery(offset){
  var p=[];
  function add(k,v){ if(v!=='' && v!=null) p.push(encodeURIComponent(k)+'='+encodeURIComponent(v)); }
  add('q', el('registry-search').value.trim());
  add('type', el('registry-type').value);
  if (el('registry-type').value !== 'variant_of') add('exclude_variants','1');
  add('status', el('registry-status').value);
  add('source', el('registry-source').value);
  add('family', el('registry-family').value);
  if (REG_VIEW==='disputed') { add('min_conf','0.5'); add('max_conf','0.75'); }
  else {
    var minConf=confMinValue();
    if (minConf > 0) add('min_conf', minConf.toFixed(2));
  }
  if (REG_VIEW==='expiring') { add('expiring','30'); }
  if (el('registry-certified') && el('registry-certified').checked) add('certified','true');
  add('limit', String(REG_PAGE.limit));
  if (offset > 0) add('offset', String(offset));
  return p.length ? '?'+p.join('&') : '';
}
async function loadRegistry(append){
  if (!append){
    REG_PAGE.offset = 0;
    REG_PAGE.items = [];
    REG_SELECTED={};
    if (el('registry-all')) el('registry-all').checked=false;
  }
  try {
    var data = await api('GET','/v1/admin/registry/relations'+registryQuery(REG_PAGE.offset));
    var metrics = await api('GET','/v1/admin/registry/metrics');
    var batch = data.items || [];
    REG_PAGE.items = append ? REG_PAGE.items.concat(batch) : batch;
    REG_PAGE.offset += batch.length;
    REG_PAGE.total = data.total || REG_PAGE.items.length;
    REG_PAGE.familyLabels = metrics.family_labels || {};
    REG_PAGE.familyCounts = {};
    (metrics.family_options || []).forEach(function(g){
      if (!REG_PAGE.familyLabels[g.value]) REG_PAGE.familyLabels[g.value] = g.label;
      REG_PAGE.familyCounts[g.value] = g.count;
    });
    renderRegistryMetrics(metrics);
    fillRegistrySelects(metrics);
    renderRegistry();
  } catch(e){ toast(e.message,true); }
}
function fillRegistrySelects(metrics){
  var ts=el('registry-type'), cur=ts.value;
  if (ts.options.length <= 1) {
    (state.relation_types||[]).filter(function(t){ return t.status==='active'; }).forEach(function(t){
      var o=document.createElement('option'); o.value=t.code; o.textContent=t.title; ts.appendChild(o);
    });
    ts.value = cur;
  }
  var fs=el('registry-family'), fcur=fs.value;
  if (fs.options.length <= 1 && metrics.family_options && metrics.family_options.length) {
    var labelCount={};
    metrics.family_options.forEach(function(g){ labelCount[g.label]=(labelCount[g.label]||0)+1; });
    metrics.family_options.forEach(function(g){
      var o=document.createElement('option');
      o.value=g.value;
      o.textContent=g.label+(labelCount[g.label]>1?' · '+g.value:'')+' ('+g.count+')';
      fs.appendChild(o);
    });
    fs.value = fcur;
  } else if (fs.options.length <= 1 && metrics.families) {
    metrics.families.forEach(function(g){
      var o=document.createElement('option'); o.value=g; o.textContent=g; fs.appendChild(o);
    });
    fs.value = fcur;
  }
}
function renderRegistryMetrics(m){
  var cards=[['связей', m.total], ['активных', m.active], ['кандидатов', m.candidates],
             ['покрытие', (m.coverage_pct||0).toFixed(1)+'%'],
             ['stale', (m.stale_share||0).toFixed(1)+'%'],
             ['хабы top-1%', (m.hub_top1_pct||0).toFixed(1)+'%']];
  el('registry-metrics').innerHTML = cards.map(function(x){
    return '<span><b>'+esc(x[1])+'</b> '+esc(x[0])+'</span>';
  }).join('<span class="sep">·</span>');
}
function renderRegistryLegend(){
  var seen={}, order=[];
  REG_PAGE.items.forEach(function(r){ if(!seen[r.type]){ seen[r.type]=1; order.push(r.type); } });
  el('registry-legend').innerHTML = order.map(function(t){
    return '<span class="type-chip"><i style="background:'+typeColor(t)+'"></i>'+esc(typeTitle(t))+'</span>';
  }).join('');
}
function renderRegistry(){
  renderRegistryLegend();
  var rows=REG_PAGE.items;
  if (el('registry-more')){
    var more = rows.length < REG_PAGE.total;
    el('registry-more').classList.toggle('hidden', !more);
    el('registry-more-hint').textContent = more ? (REG_PAGE.total - rows.length)+' ещё' : '';
  }
  el('registry-count').textContent = rows.length + ' из ' + REG_PAGE.total;
  if (!rows.length) {
    el('registry-body').innerHTML = '<tr><td colspan="6" class="empty">Связей нет. Нажмите «Сгенерировать».</td></tr>';
    return;
  }
  el('registry-body').innerHTML = rows.map(function(r){
    return '<tr data-reg-id="'+r.id+'">'+
      '<td><input type="checkbox" data-reg-check="'+r.id+'" style="width:auto"'+(REG_SELECTED[r.id]?' checked':'')+'></td>'+
      '<td>'+pairCell(r.from_title||r.from_item_id, r.to_title||r.to_item_id, r.from_short, r.to_short)+'</td>'+
      '<td><span class="type-chip" style="--tc:'+typeColor(r.type)+'" data-tip="'+esc(typeDesc(r.type))+'"><i></i>'+esc(typeTitle(r.type))+'</span></td>'+
      '<td><span class="pill '+(r.status==='active'?'on':(r.status==='rejected'?'off':'q'))+'">'+esc(regStatusLabel(r.status))+'</span></td>'+
      '<td><span class="prov-cell">'+esc(regProvLabel(r.provenance_kind))+' '+confMeter(r.confidence)+'</span></td>'+
      '<td>'+evidenceCell(r.evidence)+'</td>'+
    '</tr>';
  }).join('');
}
function registryItem(id){
  for (var i=0;i<REG_PAGE.items.length;i++) if (String(REG_PAGE.items[i].id)===String(id)) return REG_PAGE.items[i];
  return null;
}
function certSummary(label, certs){
  if (!certs || !certs.length) return '<div class="hint">'+esc(label)+': ФСТЭК — нет</div>';
  return certs.map(function(c){
    var tail = c.expired ? ' · <span style="color:#b91c1c">просрочен</span>' : ' · до '+esc(String(c.valid_until||'').slice(0,10));
    return '<div class="hint">'+esc(label)+': ФСТЭК №'+esc(c.number)+tail+'</div>';
  }).join('');
}
function renderRegistryDetail(id){
  var r=registryItem(id); if (!r) return;
  el('registry-detail').innerHTML =
    '<div class="form-title">'+esc(r.from_title||r.from_item_id)+' → '+esc(r.to_title||r.to_item_id)+'</div>'+
    '<div class="hint">'+esc(regTypeLabel(r.type))+' · '+esc(regStatusLabel(r.status))+' · '+esc(regProvLabel(r.provenance_kind))+' · уверенность: '+confMeter(r.confidence)+' '+esc(confText(r.confidence))+'</div>'+
    evidenceList(r.evidence)+'<div class="ev-note">'+esc(evidenceNote(r))+'</div>'+
    '<div class="alias-edit">Источник: <b>'+esc(r.from_short||'—')+'</b> <button class="secondary small" data-alias-side="from" data-alias-id="'+r.id+'">Алиас</button></div>'+
    '<div class="alias-edit">Цель: <b>'+esc(r.to_short||'—')+'</b> <button class="secondary small" data-alias-side="to" data-alias-id="'+r.id+'">Алиас</button></div>'+
    certSummary('Источник', r.from_certs)+certSummary('Цель', r.to_certs)+
    '<div class="hint">Семейство: '+esc(familyLabel(r.from_family))+' → '+esc(familyLabel(r.to_family))+
      (r.valid_until?' · до '+esc(regWhen(r.valid_until)):'')+
      (r.rejection_reason?' · причина отказа: '+esc(r.rejection_reason):'')+'</div>'+
    '<div class="toolbar" style="margin-top:8px">'+
      '<button class="small" data-reg-review="confirm" data-reg-review-id="'+r.id+'">Подтвердить</button>'+
      '<button class="secondary small" data-reg-review="reject" data-reg-review-id="'+r.id+'">Отклонить</button>'+
      '<button class="secondary small" data-reg-review="defer" data-reg-review-id="'+r.id+'">Отложить</button>'+
      '<button class="secondary small" data-exp-create="'+r.id+'">Проверить гипотезой</button>'+
    '</div>';
}
async function registryReview(id, decision){
  var note=el('registry-note').value.trim();
  if (decision==='reject' && !note) { toast('Нужна причина отказа', true); return; }
  var item=registryItem(id) || {};
  try {
    await api('POST','/v1/admin/registry/relations/'+id+'/review',
      {decision:decision, note:note, expected_version:item.version||1});
    toast(decision==='confirm'?'Связь подтверждена':(decision==='reject'?'Связь отклонена':'Отложено'));
    el('registry-detail').innerHTML='';
    await loadRegistry();
  } catch(e){ toast(e.message,true); }
}
function registryFilter(){
  var f={statuses:['candidate']};
  var q=el('registry-search').value.trim(); if(q) f.query=q;
  var t=el('registry-type').value; if(t) f.types=[t];
  var s=el('registry-source').value; if(s) f.provenance=[s];
  var fam=el('registry-family').value; if(fam) f.family=fam;
  if (el('registry-type').value !== 'variant_of') f.exclude_variants=true;
  if (REG_VIEW==='disputed') { f.min_confidence=0.5; f.max_confidence=0.75; }
  else {
    var minConf=confMinValue();
    if (minConf > 0) f.min_confidence=minConf;
  }
  if (REG_VIEW==='expiring') f.expiring_days=30;
  if (el('registry-certified').checked) f.certified=true;
  return f;
}
var REG_PENDING_FILTER = null;
function filterSummary(f){
  var parts=[];
  if (f.min_confidence) parts.push('уверенность ≥ '+Math.round(f.min_confidence*100)+'%');
  if (f.max_confidence) parts.push('уверенность < '+Math.round(f.max_confidence*100)+'%');
  if (f.provenance && f.provenance.length) parts.push('источник: '+f.provenance.join(', '));
  if (f.types && f.types.length) parts.push('тип: '+f.types.join(', '));
  if (f.family) parts.push('семейство: '+f.family);
  if (f.query) parts.push('поиск: '+f.query);
  if (f.certified) parts.push('с сертификатом ФСТЭК');
  return parts.length ? '<div class="hint" style="margin-top:8px">'+esc(parts.join(' · '))+'</div>' : '';
}
function registryBulkFilter(){
  var f=registryFilter();
  if (!(f.min_confidence>0) && !(f.types && f.types.length)) {
    toast('Выберите уверенность или тип', true); return;
  }
  if (!REG_PAGE.total) { toast('По фильтру ничего не найдено', true); return; }
  REG_PENDING_FILTER=f;
  var limitNote = REG_PAGE.total > 500 ? ' За один раз будет обработано до 500.' : '';
  el('registry-confirm-text').innerHTML =
    'Будет подтверждено <b>'+REG_PAGE.total+'</b> кандидатов по текущему фильтру.'+limitNote+
    filterSummary(f);
  el('registry-confirm-modal').classList.remove('hidden');
}
async function registryConfirmAll(){
  var f=REG_PENDING_FILTER;
  el('registry-confirm-modal').classList.add('hidden');
  if (!f) return;
  var note=el('registry-note').value.trim();
  try {
    var out=await api('POST','/v1/admin/registry/relations/bulk-review',
      {filter:f, decision:'confirm', note:note});
    toast('Подтверждено: '+(out.reviewed||0));
    REG_PENDING_FILTER=null;
    await loadRegistry();
  } catch(e){ toast(e.message,true); }
}
async function registryBulk(decision){
  var ids=Object.keys(REG_SELECTED).filter(function(k){ return REG_SELECTED[k]; }).map(Number);
  var note=el('registry-note').value.trim();
  if (!ids.length) { toast('Ничего не выбрано', true); return; }
  if (decision==='reject' && !note) { toast('Нужна причина отказа', true); return; }
  try {
    var out=await api('POST','/v1/admin/registry/relations/bulk-review',
      {ids:ids, decision:decision, note:note});
    toast('Обработано: '+(out.reviewed||0));
    REG_SELECTED={};
    await loadRegistry();
  } catch(e){ toast(e.message,true); }
}
async function registryGenerate(){
  try {
    var out=await api('POST','/v1/admin/registry/generate',
      {min_co_purchase:3, limit:500, include_variants:true});
    toast('Co-purchase: '+out.co_purchase+', вариантов: '+out.variant+', отказов: '+out.skipped_rejected);
    await loadRegistry();
  } catch(e){ toast(e.message,true); }
}
function renderRelations(){
  var q = el('rel-search').value.trim().toLowerCase();
  var ft = el('rel-filter-type').value, fs = el('rel-filter-status').value;
  var rows = state.relations.filter(function(r){
    if (ft && r.type!==ft) return false;
    if (fs && r.status!==fs) return false;
    if (!q) return true;
    return (r.from_id+' '+r.to_id+' '+title(r.from_id)+' '+title(r.to_id)).toLowerCase().indexOf(q)>=0;
  });
  el('rel-count').textContent = rows.length + ' из ' + state.relations.length;
  el('rel-body').innerHTML = rows.map(function(r,i){
    var idx = state.relations.indexOf(r);
    return '<tr>'+
      '<td>'+esc(title(r.from_id))+'<div class="hint">'+esc(r.from_id)+'</div></td>'+
      '<td>'+esc(title(r.to_id))+'<div class="hint">'+esc(r.to_id)+'</div></td>'+
      '<td><span class="type-chip" style="--tc:'+typeColor(r.type)+'" data-tip="'+esc(typeDesc(r.type))+'"><i></i>'+esc(typeTitle(r.type))+'</span></td>'+
      '<td>'+Math.round((r.confidence||0)*100)+'%</td>'+
      '<td>'+relationPill(r.status)+'</td>'+
      '<td style="white-space:nowrap"><button class="ghost small" data-edit="'+idx+'">Изменить</button><button class="ghost small" data-del="'+idx+'">Удалить</button></td>'+
    '</tr>';
  }).join('') || '<tr><td colspan="6" class="empty">Ничего не найдено</td></tr>';
}
function renderTypes(){
  var list = state.relation_types || [];
  if (!list.length) { el('type-list').innerHTML='<div class="empty">Шаблонов пока нет</div>'; return; }
  el('type-list').innerHTML = list.map(function(t){
    var statusCls = t.status==='active'?'on':(t.status==='draft'?'q':'off');
    return '<div class="type-card"><div class="type-head"><b>'+esc(t.title)+'</b>'+
      '<span class="pill type">'+esc(t.code)+'</span>'+
      '<span class="pill '+(t.builtin?'ai':'on')+'">'+(t.builtin?'встроенный':'свой')+'</span>'+
      '<span class="pill '+statusCls+'">'+esc(TYPE_STATUS[t.status]||t.status)+'</span>'+
      '<span class="pill type">'+esc(BEHAVIORS[t.behavior]||t.behavior)+'</span>'+
      '<span class="hint">'+(t.direction==='symmetric'?'симметричная':'направленная')+' · '+Math.round((t.default_confidence||0)*100)+'%</span></div>'+
      '<div class="hint">'+esc(t.description)+'</div>'+
      (t.examples?'<div class="hint">Примеры: '+esc(t.examples)+'</div>':'')+
      '</div>';
  }).join('');
}
function renderRules(){
  if (!state.rules.length) { el('rule-list').innerHTML='<div class="empty">Правил пока нет</div>'; return; }
  el('rule-list').innerHTML = state.rules.map(function(r){
    return '<div class="rule"><div><b>'+esc(r.summary)+'</b><small>'+esc(r.id)+' · '+esc(r.condition)+'</small></div>'+
      '<div><span class="pill type">'+esc(RULE_TYPES[r.type]||r.type)+'</span> <button class="ghost small" data-rule-del="'+esc(r.id)+'">Удалить</button></div></div>';
  }).join('');
}
var AUDIT_ACTIONS = {'relation.upsert':'Связь сохранена','relation.delete':'Связь удалена','relation_type.upsert':'Шаблон связи сохранён','relation_type.delete':'Шаблон связи удалён','relation.review':'Связь проверена','strategy.auto':'Авто-А/Б изменён','relations.auto_flag':'Связи помечены для проверки','dataset.co_purchase_suggest':'Связи из co-purchase созданы','relations.auto_review':'Связи проверены бандитом','relation_scope.review':'Скоуп связи обновлён','experiment.seed_demo':'Демо-гипотеза создана','experiment.decision':'Решение по гипотезе','rule.create':'Правило добавлено','rule.delete':'Правило удалено','weights.update':'Веса изменены','agent.apply':'Применены изменения ИИ'};
function renderAudit(){
  var list = state.audit || [];
  if (!list.length) { el('audit-list').textContent='Пока нет изменений'; return; }
  el('audit-list').innerHTML = list.slice(0,8).map(function(a){
    var t = new Date(a.created_at).toLocaleString('ru-RU',{day:'2-digit',month:'2-digit',hour:'2-digit',minute:'2-digit'});
    return '<div style="padding:6px 0;border-bottom:1px solid #f0f1f5"><b style="color:#111827">'+esc(AUDIT_ACTIONS[a.action]||a.action)+'</b><div>'+esc(t)+' · '+esc(a.actor)+'</div></div>';
  }).join('');
}
function renderWeights(){
  var aud = el('weight-audience').value || 'default';
  var weights = (state.weights_by_audience && state.weights_by_audience[aud]) || state.weights || {};
  var sum = 0;
  el('weight-rows').innerHTML = WEIGHT_ROWS.map(function(row){
    var v = Math.round((weights[row[0]]||0)*1000)/10;
    sum += v;
    return '<div class="w"><b>'+row[1]+'</b><input type="range" min="0" max="100" step="1" value="'+v+'" data-weight="'+row[0]+'"><output id="wout-'+row[0]+'">'+v.toFixed(0)+'%</output></div>';
  }).join('');
  el('weight-sum').textContent = 'Сумма: ' + sum.toFixed(0) + '%';
  el('weight-sum').className = 'sum' + (Math.abs(sum-100)>1 ? ' bad' : '');
  document.querySelectorAll('[data-weight]').forEach(function(inp){
    inp.addEventListener('input', function(){
      el('wout-'+inp.dataset.weight).textContent = inp.value+'%';
      var s=0; document.querySelectorAll('[data-weight]').forEach(function(x){s+=Number(x.value);});
      el('weight-sum').textContent='Сумма: '+s+'%'; el('weight-sum').className='sum'+(Math.abs(s-100)>1?' bad':'');
    });
  });
}
function syncRuleFields(){
  var t = el('rule-type').value;
  el('rule-other-wrap').classList.toggle('hidden', t!=='excludes');
  el('rule-discount-wrap').classList.toggle('hidden', t!=='max_discount');
  var hints = {
    excludes: 'Не предлагать два товара вместе.',
    out_of_stock: 'Товар исчезнет из рекомендаций, но останется в каталоге.',
    manager_only: 'Карточка останется, но появится отметка «нужно согласование».',
    max_discount: 'Акционная скидка будет урезана до указанного значения.',
    export_control: 'Товар нельзя рекомендовать из-за экспортного контроля.'
  };
  el('rule-hint').textContent = hints[t]||'';
}

function tabForHash(hash){
  var v=String(hash||'').replace(/^#/,'');
  if (v==='hypotheses' || v==='hypothesis' || v==='гипотезы') v='experiments';
  return document.querySelector('.tab[data-tab="'+v+'"]');
}
document.querySelectorAll('.tab').forEach(function(b){
  b.addEventListener('click', function(){
    document.querySelectorAll('.tab').forEach(function(x){x.classList.remove('on');});
    document.querySelectorAll('.tabbody > div').forEach(function(x){x.classList.add('hidden');});
    b.classList.add('on'); el('tab-'+b.dataset.tab).classList.remove('hidden');
    if (location.hash !== '#'+b.dataset.tab) history.replaceState(null, '', '#'+b.dataset.tab);
    document.querySelector('.layout').classList.toggle('registry-mode', b.dataset.tab === 'registry');
    document.body.classList.toggle('registry-mode', b.dataset.tab === 'registry');
    if (b.dataset.tab === 'analytics') loadAnalytics();
    if (b.dataset.tab === 'dataset') loadDataset();
    if (b.dataset.tab === 'registry') loadRegistry();
    if (b.dataset.tab === 'experiments') { loadExperiments(); loadKnowledge(); }
  });
});
el('registry-filters-toggle').addEventListener('click', function(){ el('registry-filters').classList.toggle('hidden'); });
el('registry-conf-min').addEventListener('input', updateConfLabel);
el('registry-apply').addEventListener('click', function(){ REG_VIEW=''; loadRegistry(); });
updateConfLabel();
el('registry-search').addEventListener('keyup', function(ev){ if (ev.key==='Enter') loadRegistry(); });
['registry-type','registry-status','registry-source','registry-family','registry-conf-min'].forEach(function(id){
  el(id).addEventListener('change', loadRegistry);
});
document.querySelectorAll('#tab-registry [data-view]').forEach(function(b){
  b.addEventListener('click', function(){
    REG_VIEW=b.dataset.view;
    el('registry-search').value='';
    el('registry-type').value='';
    el('registry-status').value='';
    el('registry-source').value='';
    el('registry-family').value='';
    if (REG_VIEW==='disputed') el('registry-status').value='candidate';
    if (REG_VIEW==='structural') el('registry-type').value='variant_of';
    loadRegistry();
  });
});
el('registry-all').addEventListener('change', function(){
  var on=this.checked;
  REG_PAGE.items.forEach(function(r){ REG_SELECTED[r.id]=on; });
  renderRegistry();
});
el('registry-body').addEventListener('change', function(ev){
  var id=ev.target.dataset.regCheck;
  if (id) REG_SELECTED[id]=ev.target.checked;
});
el('registry-body').addEventListener('mousemove', function(ev){
  var n=ev.target.closest('[data-tip]');
  if (n) showRegistryTip(n.getAttribute('data-tip'), ev.clientX, ev.clientY);
  else hideRegistryTip();
});
el('registry-body').addEventListener('mouseleave', hideRegistryTip);
el('registry-body').addEventListener('click', function(ev){
  if (ev.target.dataset.regCheck) return;
  var pair=ev.target.closest('.pair');
  if (pair){
    var expanded=!pair.classList.contains('expanded');
    pair.classList.toggle('expanded', expanded);
    pair.querySelectorAll('.pair-text').forEach(function(t){
      t.textContent=expanded?t.dataset.full:t.dataset.short;
      t.classList.toggle('full', expanded);
    });
    pair.querySelectorAll('.pair-from,.pair-to').forEach(function(l){
      l.classList.toggle('expanded', expanded);
    });
    return;
  }
  var tr=ev.target.closest('tr[data-reg-id]');
  if (tr) renderRegistryDetail(tr.dataset.regId);
});
el('registry-detail').addEventListener('click', async function(ev){
  var make=ev.target.dataset.expCreate;
  if (make){
    var rel=registryItem(make);
    if(!rel) return;
    try {
      var out=await api('POST','/v1/admin/experiments',{
        from_id:rel.from_item_id, to_id:rel.to_item_id, relation_type:rel.type,
        audience:rel.audience, industry:rel.industry
      });
      toast('Гипотеза: '+out.id);
      document.querySelector('[data-tab=experiments]').click();
    } catch(e){ toast(e.message,true); }
    return;
  }
  var side=ev.target.dataset.aliasSide;
  if (side){
    var item=registryItem(ev.target.dataset.aliasId);
    if (!item) return;
    var itemID=side==='from'?item.from_item_id:item.to_item_id;
    var current=side==='from'?item.from_short:item.to_short;
    var title=side==='from'?item.from_title:item.to_title;
    var name=prompt('Короткое имя для '+title, current||'');
    if (name===null) return;
    try {
      await api('PUT','/v1/admin/registry/aliases/'+encodeURIComponent(itemID),{short_name:name.trim()});
      toast('Алиас сохранён');
      await loadRegistry();
      if (registryItem(item.id)) renderRegistryDetail(item.id);
    } catch(e){ toast(e.message,true); }
    return;
  }
  var d=ev.target.dataset.regReview, id=ev.target.dataset.regReviewId;
  if (d && id) registryReview(id,d);
});
el('registry-confirm').addEventListener('click', function(){ registryBulk('confirm'); });
el('registry-confirm-filter').addEventListener('click', registryBulkFilter);
el('registry-confirm-yes').addEventListener('click', registryConfirmAll);
el('registry-confirm-no').addEventListener('click', function(){ REG_PENDING_FILTER=null; el('registry-confirm-modal').classList.add('hidden'); });
el('registry-confirm-modal').addEventListener('click', function(ev){ if (ev.target === this){ REG_PENDING_FILTER=null; this.classList.add('hidden'); } });
el('registry-reject').addEventListener('click', function(){ registryBulk('reject'); });
el('registry-defer').addEventListener('click', function(){ registryBulk('defer'); });
el('registry-generate').addEventListener('click', registryGenerate);
el('registry-reset').addEventListener('click', function(){ el('registry-reset-modal').classList.remove('hidden'); });
el('registry-reset-no').addEventListener('click', function(){ el('registry-reset-modal').classList.add('hidden'); });
el('registry-reset-modal').addEventListener('click', function(ev){ if (ev.target === this) this.classList.add('hidden'); });
el('registry-reset-yes').addEventListener('click', async function(){
  el('registry-reset-modal').classList.add('hidden');
  try {
    var res=await api('POST','/v1/admin/registry/reset',{confirm:'RESET'});
    toast('Каталог сброшен: удалено '+res.items_removed+', связей '+res.relations);
    await loadRegistry();
  } catch(e){ toast(e.message,true); }
});
el('registry-import').addEventListener('click', function(){
  el('registry-import-result').textContent='';
  el('registry-import-file').value='';
  el('registry-import-modal').classList.remove('hidden');
});
el('registry-import-no').addEventListener('click', function(){ el('registry-import-modal').classList.add('hidden'); });
el('registry-import-modal').addEventListener('click', function(ev){ if (ev.target === this) this.classList.add('hidden'); });
el('registry-import-yes').addEventListener('click', async function(){
  var file=el('registry-import-file').files[0];
  if (!file){ el('registry-import-result').textContent='Выберите CSV-файл'; return; }
  var fd=new FormData(); fd.append('file', file);
  try {
    var res=await fetch('/v1/admin/registry/import/items',{method:'POST',body:fd});
    var data={}; try { data=await res.json(); } catch(e) { data={error:'сервер вернул не JSON'}; }
    if (!res.ok) throw new Error(data.error || ('HTTP '+res.status));
    el('registry-import-result').innerHTML='Добавлено: <b>'+data.added+'</b>, обновлено: <b>'+data.updated+'</b>, пропущено: '+data.skipped+
      (data.errors&&data.errors.length ? '<br><span style="color:#b91c1c">'+data.errors.map(esc).join('<br>')+'</span>' : '');
    toast('Импорт: +'+data.added+' / обновлено '+data.updated);
    await loadRegistry();
  } catch(e){
    el('registry-import-result').innerHTML='<span style="color:#b91c1c">'+esc(e.message)+'</span>';
  }
});
el('registry-more').addEventListener('click', function(){ loadRegistry(true); });
el('registry-sweep').addEventListener('click', async function(){
  try {
    var out=await api('POST','/v1/admin/registry/sweep',{});
    toast('Stale: '+out.stale+', истекло: '+out.expired);
    await loadRegistry();
  } catch(e){ toast(e.message,true); }
});
el('rel-search').addEventListener('input', renderRelations);
el('rel-filter-type').addEventListener('change', renderRelations);
el('rel-filter-status').addEventListener('change', renderRelations);
el('rule-type').addEventListener('change', syncRuleFields);
el('review-refresh').addEventListener('click', loadReview);
el('review-filter').addEventListener('change', loadReview);
el('review-search').addEventListener('input', renderReview);
el('review-all').addEventListener('change', function(){
  document.querySelectorAll('[data-review-check]').forEach(function(x){ x.checked=el('review-all').checked; });
});
el('review-confirm').addEventListener('click', function(){
  var items=[]; document.querySelectorAll('[data-review-check]:checked').forEach(function(x){ items.push(reviewItems[Number(x.dataset.reviewCheck)]); });
  reviewDecision('confirm', items);
});
el('review-reject').addEventListener('click', function(){
  var items=[]; document.querySelectorAll('[data-review-check]:checked').forEach(function(x){ items.push(reviewItems[Number(x.dataset.reviewCheck)]); });
  reviewDecision('reject', items);
});
el('review-list').addEventListener('click', function(ev){
  var mode=ev.target.dataset.reviewOne;
  if (!mode) return;
  var r=reviewItems[Number(ev.target.dataset.reviewIndex)];
  if (r) reviewDecision(mode, [r]);
});
var EXP_ITEMS = null;
var expTitleDirty = false, expHypDirty = false;
async function expItemList(){
  if (EXP_ITEMS) return EXP_ITEMS;
  var d = await api('GET','/v1/admin/registry/items?limit=5000');
  EXP_ITEMS = (d.items || []).slice().sort(function(a,b){
    var pa = a.title.charAt(0) === '(' ? 1 : 0, pb = b.title.charAt(0) === '(' ? 1 : 0;
    if (pa !== pb) return pa - pb;
    return a.title.localeCompare(b.title, 'ru');
  });
  return EXP_ITEMS;
}
var EXP_GENERIC_PREFIXES = [
  'Предоставление доступа к сервисам ',
  'Предоставление доступа к ',
  'Программа контроля полномочий доступа к информационным ресурсам ',
  'Средство создания модели системы разграничения доступа '
];
function expCleanTitle(s){
  s = String(s||'');
  var t = s.replace(/^\([^)]*\)\s*/, '').trim();
  for (var i=0;i<EXP_GENERIC_PREFIXES.length;i++){
    var p=EXP_GENERIC_PREFIXES[i];
    if (t.length > p.length && t.indexOf(p) === 0) { t = t.slice(p.length).trim(); break; }
  }
  return t || s;
}
function expFillItems(sel, filter){
  var q = String(filter||'').toLowerCase().trim();
  var prev = sel.value, out = [];
  for (var i=0;i<EXP_ITEMS.length;i++){
    var it = EXP_ITEMS[i];
    if (q && (it.title.toLowerCase().indexOf(q) < 0 &&
              String(it.product_group||'').toLowerCase().indexOf(q) < 0)) continue;
    out.push(it);
    if (out.length >= 900) break;
  }
  sel.innerHTML = '<option value="">— выберите товар —</option>' + out.map(function(it){
    return '<option value="'+esc(it.item_id)+'">'+esc(expCleanTitle(it.title))+'</option>';
  }).join('');
  if (prev) sel.value = prev;
}
function expClip(s, n){
  s = String(s||'');
  if (s.length <= n) return s;
  var cut = s.slice(0, n);
  var i = cut.lastIndexOf(' ');
  if (i > n/2) cut = cut.slice(0, i);
  return cut.replace(/[\s,.;:\-]+$/,'')+'…';
}
function expSelectedTitle(sel){
  var o = sel.options[sel.selectedIndex];
  return o ? o.textContent : '';
}
function expGenerateNames(){
  var ft = expSelectedTitle(el('exp-from')), tt = expSelectedTitle(el('exp-to'));
  if (!ft || !tt) return;
  var f = expClip(ft, 54), t = expClip(tt, 54);
  var aud = el('exp-audience').value === 'b2c' ? 'B2C' : 'B2B';
  if (!expTitleDirty) el('exp-title').value = 'Показ «'+t+'» после «'+f+'»';
  if (!expHypDirty) el('exp-hypothesis').value = 'Показ «'+t+'» после «'+f+'» повышает конверсию в '+aud;
}
function expFillTypes(){
  var sel = el('exp-type'), list = activeTypes(), opts = [];
  if (list.length) {
    list.forEach(function(t){ if (t.code !== 'variant_of') opts.push([t.code, t.title || t.code]); });
  }
  if (!opts.length) Object.keys(REL_TYPES).forEach(function(k){ opts.push([k, REL_TYPES[k]]); });
  sel.innerHTML = opts.map(function(p){ return '<option value="'+esc(p[0])+'">'+esc(p[1])+'</option>'; }).join('');
}
async function openExperimentForm(){
  try { await expItemList(); } catch(e){ toast(e.message, true); return; }
  expFillTypes();
  el('exp-from-search').value = ''; el('exp-to-search').value = '';
  expFillItems(el('exp-from'), ''); expFillItems(el('exp-to'), '');
  el('exp-audience').value = 'b2b';
  el('exp-industry').value = '';
  el('exp-metric').value = 'conversion';
  el('exp-split').value = '0.5';
  el('exp-g-renewal').checked = true; el('exp-g-upsell').checked = true; el('exp-g-fallback').checked = true;
  expTitleDirty = false; expHypDirty = false;
  el('exp-form-error').textContent = '';
  expGenerateNames();
  el('experiment-modal').classList.remove('hidden');
  el('exp-from-search').focus();
}
function closeExperimentForm(){ el('experiment-modal').classList.add('hidden'); }
el('experiment-seed').addEventListener('click', openExperimentForm);
el('exp-cancel').addEventListener('click', closeExperimentForm);
el('experiment-modal').addEventListener('click', function(ev){ if (ev.target === this) closeExperimentForm(); });
document.addEventListener('keydown', function(ev){ if (ev.key === 'Escape') { if (!el('experiment-modal').classList.contains('hidden')) closeExperimentForm(); if (!el('emulation-modal').classList.contains('hidden')) closeEmulation(); } });
el('exp-from-search').addEventListener('input', function(){ expFillItems(el('exp-from'), this.value); });
el('exp-to-search').addEventListener('input', function(){ expFillItems(el('exp-to'), this.value); });
el('exp-from').addEventListener('change', expGenerateNames);
el('exp-to').addEventListener('change', expGenerateNames);
el('exp-audience').addEventListener('change', expGenerateNames);
el('exp-title').addEventListener('input', function(){ expTitleDirty = true; });
el('exp-hypothesis').addEventListener('input', function(){ expHypDirty = true; });
el('exp-submit').addEventListener('click', async function(){
  var from = el('exp-from').value, to = el('exp-to').value;
  var err = el('exp-form-error');
  err.textContent = '';
  if (!from || !to) { err.textContent = 'Выберите источник и цель.'; return; }
  if (from === to) { err.textContent = 'Источник и цель должны различаться.'; return; }
  var guards = [];
  if (el('exp-g-renewal').checked) guards.push('renewal');
  if (el('exp-g-upsell').checked) guards.push('upsell');
  if (el('exp-g-fallback').checked) guards.push('fallback');
  try {
    var out = await api('POST','/v1/admin/experiments',{
      from_id: from, to_id: to, relation_type: el('exp-type').value,
      title: el('exp-title').value.trim(), hypothesis: el('exp-hypothesis').value.trim(),
      audience: el('exp-audience').value, industry: el('exp-industry').value.trim(),
      metric: el('exp-metric').value, guardrails: guards.join(', '),
      traffic_split: Number(el('exp-split').value)
    });
    closeExperimentForm();
    toast(out.created ? 'Гипотеза создана' : 'Гипотеза для этой пары уже идёт');
    document.querySelector('[data-tab=experiments]').click();
  } catch(e){ err.textContent = e.message; }
});
var EMU_PROFILES = [], EMU_INDUSTRIES = [], EMU_EXP = null;
function fitColor(f){ return f>=0.6?'#047857':(f>=0.35?'#b45309':'#b91c1c'); }
async function loadEmuProfiles(){
  var d = await api('GET','/v1/admin/client-profiles');
  EMU_PROFILES = d.items || []; EMU_INDUSTRIES = d.industries || [];
  var sel = el('emu-profile');
  sel.innerHTML = EMU_PROFILES.map(function(p){
    return '<option value="'+esc(p.id)+'">'+esc(p.name)+' · '+esc(String(p.audience||'').toUpperCase())+(p.industry?' / '+esc(p.industry):'')+'</option>';
  }).join('') + '<option value="__new">＋ Новый профиль</option>';
  el('emu-industry').innerHTML = '<option value="">— не указана —</option>'+
    EMU_INDUSTRIES.map(function(i){ return '<option>'+esc(i)+'</option>'; }).join('');
  emuToggleNew();
}
function emuToggleNew(){
  el('emu-new').classList.toggle('hidden', el('emu-profile').value !== '__new');
}
function openEmulation(id){
  var x = (EXPERIMENTS||[]).filter(function(e){ return e.id===id; })[0];
  if (!x) { toast('Гипотеза не найдена', true); return; }
  EMU_EXP = x;
  el('emu-hypothesis').innerHTML = '<b>'+esc(x.title)+'</b><br>'+esc(x.from_title||x.from_id)+' → '+esc(x.to_title||x.to_id);
  el('emu-result').innerHTML = '';
  el('emu-form-error').textContent = '';
  el('emulation-modal').classList.remove('hidden');
  loadEmuProfiles().catch(function(e){ toast(e.message,true); });
}
function closeEmulation(){ el('emulation-modal').classList.add('hidden'); }
function renderEmulation(r){
  var chips = (r.matched||[]).map(function(m){ return '<span class="chip">'+esc(m)+'</span>'; }).join('');
  var reasons = (r.reasons||[]).map(function(x){ return '<div class="hint">• '+esc(x)+'</div>'; }).join('');
  var rows = (r.sessions||[]).map(function(s){
    return '<tr><td>'+s.n+'</td><td>'+esc(s.variant)+'</td><td>'+esc(s.product_title)+'</td><td>'+pct(s.fit)+'</td><td>'+(s.clicked?'✓':'—')+'</td><td>'+(s.purchased?'✓':'—')+'</td></tr>';
  }).join('');
  var variants = [r.control, r.treatment].map(function(v){
    return '<tr><td><b>'+esc(v.variant)+'</b></td><td>'+v.exposures+'</td><td>'+v.clicks+'</td><td>'+v.purchases+'</td><td>'+pct(v.ctr)+'</td><td>'+pct(v.conversion)+'</td><td>'+pct(v.fit)+'</td></tr>';
  }).join('');
  el('emu-result').innerHTML = '<div class="formbox" style="margin-top:6px">'+
    '<div style="display:flex;align-items:baseline;gap:10px;flex-wrap:wrap"><span class="form-title" style="margin:0;color:'+fitColor(r.fit)+'">'+pct(r.fit)+'</span><span class="hint">попадание в профиль «'+esc(r.profile_name)+'»</span></div>'+
    '<div class="chips">'+chips+'</div>'+ reasons +
    '<div class="tablewrap" style="margin-top:8px"><table><thead><tr><th>Вариант</th><th>Exposure</th><th>Клики</th><th>Покупки</th><th>CTR</th><th>Conversion</th><th>Fit</th></tr></thead><tbody>'+variants+'</tbody></table></div>'+
    '<div class="hint" style="margin-top:6px">Uplift: <b>'+(r.uplift>=0?'+':'')+(r.uplift*100).toFixed(1)+'%</b> · control — «'+esc(r.control_title)+'», treatment — «'+esc(r.treatment_title)+'».</div>'+
    '<div class="form-title" style="font-size:14px;margin:12px 0 4px">Сессии</div>'+
    '<div class="tablewrap"><table><thead><tr><th>#</th><th>Вариант</th><th>Товар</th><th>Fit</th><th>Клик</th><th>Покупка</th></tr></thead><tbody>'+rows+'</tbody></table></div>'+
  '</div>';
}
el('emu-profile').addEventListener('change', emuToggleNew);
el('emu-cancel').addEventListener('click', closeEmulation);
el('emulation-modal').addEventListener('click', function(ev){ if (ev.target === this) closeEmulation(); });
el('emu-run').addEventListener('click', async function(){
  var err = el('emu-form-error'); err.textContent = '';
  var btn = this; btn.disabled = true;
  try {
    var profileID = el('emu-profile').value;
    if (!profileID) { err.textContent = 'Выберите профиль или создайте новый.'; return; }
    if (profileID === '__new') {
      var name = el('emu-name').value.trim(), desc = el('emu-description').value.trim();
      if (!name) { err.textContent = 'Укажите название профиля.'; return; }
      if (!desc) { err.textContent = 'Опишите клиента словами.'; return; }
      var p = await api('POST','/v1/admin/client-profiles',{
        name: name, audience: el('emu-audience').value,
        industry: el('emu-industry').value, description: desc
      });
      await loadEmuProfiles();
      el('emu-profile').value = p.id;
      emuToggleNew();
      profileID = p.id;
    }
    var r = await api('POST','/v1/admin/experiments/'+encodeURIComponent(EMU_EXP.id)+'/emulate',{
      profile_id: profileID, impressions: Number(el('emu-impressions').value)
    });
    renderEmulation(r);
    toast('Прогон готов: попадание '+pct(r.fit));
    loadExperiments();
  } catch(e){ err.textContent = e.message; }
  finally { btn.disabled = false; }
});
el('open-lk').addEventListener('click', function(){ window.open('/lk','_blank'); });
el('experiment-refresh').addEventListener('click', function(){ loadExperiments(); loadKnowledge(); });
el('experiment-search-btn').addEventListener('click', loadKnowledge);
el('experiment-decision-filter').addEventListener('change', loadKnowledge);
el('experiment-search').addEventListener('keydown', function(e){ if(e.key==='Enter') loadKnowledge(); });
el('experiments-list').addEventListener('click', async function(ev){
  var emu=ev.target.dataset.expEmulate;
  if(emu){ openEmulation(emu); return; }
  var mon=ev.target.dataset.expMonitor;
  if(mon){
    try { var r=await api('POST','/v1/admin/experiments/'+encodeURIComponent(mon)+'/monitor',{}); toast(r.stopped?'Остановлено: '+r.outcome:'Guardrails ok'); loadExperiments(); loadKnowledge(); }
    catch(e){ toast(e.message,true); }
    return;
  }
  var mode=ev.target.dataset.expDecision, id=ev.target.dataset.expId;
  if(!mode||!id)return;
  var note = mode==='promote'?'подтверждено на пилоте':(mode==='reject'?'гипотеза не подтверждена':'нужна большая выборка');
  try {
    var d=await api('POST','/v1/admin/experiments/'+encodeURIComponent(id)+'/decision',{decision:mode,note:note});
    toast('Решение: '+d.decision+(d.auto?' (guardrail)':''));
    loadExperiments();
    loadKnowledge();
  }
  catch(e){ toast(e.message,true); }
});
el('relation-auto').addEventListener('click', async function(){
  try { var d=await api('POST','/v1/admin/relations/auto-review',{min_shown:20,min_ctr:0.03}); toast('Продвинуто: '+d.promoted+', отклонено: '+d.rejected); loadRelationPerf(); loadRelationScopes(); }
  catch(e){ toast(e.message,true); }
});
el('relation-scopes').addEventListener('click', async function(ev){
  var mode=ev.target.dataset.scopeReview;
  if (!mode) return;
  var ds=ev.target.dataset;
  try {
    await api('POST','/v1/admin/relations/scopes/review',{
      from_id:ds.scopeFrom,to_id:ds.scopeTo,type:ds.scopeType,
      audience:ds.scopeAudience||'',industry:ds.scopeIndustry||'',
      decision:mode,note:'',ttl_days:0
    });
    toast(mode==='confirm'?'Связь подтверждена в скоупе':(mode==='reject'?'Связь отклонена в скоупе':'Связь возвращена в тест'));
    loadRelationScopes(); loadRelationPerf();
  } catch(e){ toast(e.message,true); }
});
el('analytics-refresh').addEventListener('click', loadAnalytics);
el('analytics-days').addEventListener('change', loadAnalytics);
el('health-flag').addEventListener('click', async function(){
  try { var d=await api('POST','/v1/admin/analytics/auto-review',{}); toast('Помечено связей: '+(d.flagged||0)); loadHealth(); }
  catch(e){ toast(e.message,true); }
});
el('auto-strategy').addEventListener('change', async function(){
  try { await api('PUT','/v1/admin/strategies/auto', {enabled: el('auto-strategy').checked}); toast(el('auto-strategy').checked?'Авто-А/Б включён':'Авто-А/Б выключен'); }
  catch(e){ toast(e.message,true); loadStrategies(); }
});

el('agent-run').addEventListener('click', runRelationAgent);
el('agent-result').addEventListener('click', function(ev){
  var mode=ev.target.dataset.agentAdd;
  if (mode) addAgentRelation(mode);
});
el('rel-save').addEventListener('click', async function(){
  try {
    await api('POST','/v1/admin/relations', {
      from_id: el('rel-from').value, to_id: el('rel-to').value,
      type: el('rel-type').value, status: el('rel-status').value,
      confidence: Number(el('rel-conf').value)/100
    });
    toast('Связь сохранена'); await load();
  } catch(e){ toast(e.message, true); }
});
el('rel-body').addEventListener('click', async function(ev){
  var edit = ev.target.dataset.edit, del = ev.target.dataset.del;
  if (edit !== undefined) {
    var r = state.relations[Number(edit)];
    el('rel-from').value=r.from_id; el('rel-to').value=r.to_id; el('rel-type').value=r.type;
    el('rel-status').value=r.status; el('rel-conf').value=Math.round((r.confidence||0)*100);
    document.querySelector('[data-tab=relations]').click(); el('rel-conf').focus();
  }
  if (del !== undefined) {
    var r = state.relations[Number(del)];
    if (!confirm('Удалить связь '+title(r.from_id)+' → '+title(r.to_id)+'?')) return;
    try { await api('DELETE','/v1/admin/relations', {from_id:r.from_id, to_id:r.to_id, type:r.type}); toast('Связь удалена'); await load(); }
    catch(e){ toast(e.message,true); }
  }
});
el('rule-save').addEventListener('click', async function(){
  var t=el('rule-type').value;
  var body={type:t, product_id:el('rule-product').value, other_product_id:el('rule-other').value, discount:Number(el('rule-discount').value)};
  if (t!=='excludes') body.other_product_id='';
  if (t!=='max_discount') body.product_id = t==='max_discount' ? '' : body.product_id;
  try { await api('POST','/v1/admin/rules', body); toast('Правило добавлено'); await load(); }
  catch(e){ toast(e.message,true); }
});
el('rule-list').addEventListener('click', async function(ev){
  var id = ev.target.dataset.ruleDel;
  if (!id) return;
  try { await api('DELETE','/v1/admin/rules/'+encodeURIComponent(id)); toast('Правило удалено'); await load(); }
  catch(e){ toast(e.message,true); }
});
el('weight-audience').addEventListener('change', renderWeights);
el('weight-save').addEventListener('click', async function(){
  var aud=el('weight-audience').value;
  var weights={};
  document.querySelectorAll('[data-weight]').forEach(function(x){ weights[x.dataset.weight]=Number(x.value)/100; });
  try {
    var saved=await api('PUT','/v1/admin/weights', {audience: aud==='default'?'':aud, weights: weights});
    state.weights_by_audience[aud]=saved;
    if (aud==='default') state.weights=saved;
    toast('Веса сохранены: '+aud); renderWeights();
  } catch(e){ toast(e.message,true); }
});
el('preview-run').addEventListener('click', async function(){
  var id=el('preview-client').value; if (!id) return;
  el('preview-out').innerHTML='Считаю…';
  try {
    var r = await api('POST','/v1/recommendations', {client_id:id});
    el('preview-out').innerHTML = (r.recommendations||[]).map(function(x){
      return '<div class="preview-item"><b>'+x.rank+'. '+esc(x.title)+'</b><small>'+esc(x.reason)+'</small></div>';
    }).join('') || '<div class="hint">Нет рекомендаций</div>';
  } catch(e){ el('preview-out').innerHTML='<span style="color:#b91c1c">'+esc(e.message)+'</span>'; }
});

var CHIPS = [
  'Сделай Кибер Бэкап дополнением к Kaiten',
  'Убери GStarCAD из рекомендаций',
  'Ограничь скидку 5%',
  'Согласование менеджера для Microsoft 365 бизнес премиум',
  'Релевантность 40, срочность 20, аффинность 20, маржа 10, уверенность 10'
];
el('chips').innerHTML = CHIPS.map(function(c){ return '<span class="chip">'+esc(c)+'</span>'; }).join('');
el('chips').addEventListener('click', function(ev){ if (ev.target.classList.contains('chip')) el('assistant-text').value=ev.target.textContent; });
el('assistant-propose').addEventListener('click', async function(){
  var text=el('assistant-text').value.trim(); if (!text) return toast('Опишите изменение', true);
  el('assistant-result').innerHTML='<div class="hint">Разбираю запрос…</div>';
  try {
    var p = await api('POST','/v1/admin/agent/propose', {text:text});
    lastOps = p.operations||[];
    var src = p.source==='llm' ? 'ИИ' : 'разбор по правилам';
    var html = '<div class="formbox"><b>Как понял запрос</b><p>'+esc(p.explanation)+'</p><div class="hint">Источник: '+src+'</div></div>';
    if (lastOps.length) {
      html += '<div class="ops">'+lastOps.map(function(op){ return '<div class="op"><b>'+esc(op.summary||op.action)+'</b><small>'+esc(op.action)+'</small></div>'; }).join('')+'</div>';
      html += '<button id="assistant-apply" style="margin-top:12px">Применить изменения</button>';
    }
    el('assistant-result').innerHTML=html;
    if (lastOps.length) el('assistant-apply').addEventListener('click', applyAssistant);
  } catch(e){ el('assistant-result').innerHTML='<div class="hint" style="color:#b91c1c">'+esc(e.message)+'</div>'; }
});
async function applyAssistant(){
  try {
    var r = await api('POST','/v1/admin/agent/apply', {operations:lastOps});
    toast('Применено изменений: '+r.applied); lastOps=[]; el('assistant-result').innerHTML='';
    await load();
  } catch(e){ toast(e.message,true); }
}
load().then(function(){
  var target = location.hash ? tabForHash(location.hash) : null;
  if (target) {
    target.click();
  } else {
    var on = document.querySelector('.tab.on');
    if (on && on.dataset.tab === 'registry') {
      document.body.classList.add('registry-mode');
      document.querySelector('.layout').classList.add('registry-mode');
      loadRegistry();
    } else if (on && on.dataset.tab === 'dataset') {
      loadDataset();
    }
  }
}).catch(function(e){ toast('Не удалось загрузить состояние: '+e.message, true); });
window.addEventListener('hashchange', function(){
  var target = tabForHash(location.hash);
  if (target) target.click();
});
</script>
</body>
</html>`
