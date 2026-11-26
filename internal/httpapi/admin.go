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
<title>LK Advisor — настройка правил</title>
<style>
:root{--bg:#f6f7fb;--card:#fff;--ink:#111827;--muted:#6b7280;--line:#e5e7eb;--red:#c8102e;--blue:#1d4ed8;--green:#047857;--amber:#b45309;--purple:#6d28d9}
*{box-sizing:border-box}
body{margin:0;background:var(--bg);color:var(--ink);font:14px/1.45 -apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,Arial,sans-serif}
.top{background:linear-gradient(135deg,#070b14 0%,#111827 60%,#1f2937 100%);color:#fff;padding:18px 0;border-bottom:1px solid rgba(255,255,255,.07);box-shadow:0 8px 28px rgba(15,23,42,.16)}
.wrap{max-width:1240px;margin:0 auto;padding:0 24px}
.top .wrap{display:flex;align-items:center;justify-content:space-between;gap:20px}
.brand{display:flex;align-items:center;gap:12px}
.logo{background:linear-gradient(135deg,#e11d48,#991b1b);color:#fff;font-weight:800;border-radius:11px;width:42px;height:42px;display:grid;place-items:center;font-size:14px;letter-spacing:.5px;box-shadow:0 8px 22px rgba(225,29,72,.3)}
.brand b{display:block;font-size:18px}
.brand small{color:#94a3b8}
nav a{color:#cbd5e1;text-decoration:none;margin-left:18px}
nav a:hover{color:#fff}
main{padding:28px 0 60px}
h1{font-size:28px;margin:0 0 6px;letter-spacing:-.02em}
.lead{color:var(--muted);margin:0 0 20px;max-width:760px}
.stats{display:grid;grid-template-columns:repeat(4,1fr);gap:12px;margin-bottom:22px}
.stat{background:var(--card);border:1px solid var(--line);border-radius:12px;padding:14px 16px;box-shadow:0 3px 12px rgba(15,23,42,.035);transition:.15s}
.stat:hover{transform:translateY(-1px);box-shadow:0 10px 26px rgba(15,23,42,.07)}
.stat b{display:block;font-size:22px}
.stat span{color:var(--muted);font-size:12px}
.layout{display:grid;grid-template-columns:minmax(0,1fr) 340px;gap:20px;align-items:start}
.layout.registry-mode{grid-template-columns:minmax(0,1fr)}
.layout.registry-mode .side{display:none}
.panel,.card{background:var(--card);border:1px solid var(--line);border-radius:14px;box-shadow:0 8px 24px rgba(15,23,42,.045)}
.panel{padding:0;overflow:hidden}
.tabs{display:flex;gap:4px;padding:10px;border-bottom:1px solid var(--line);background:#fbfbfd;flex-wrap:wrap}
.tab{border:0;background:transparent;padding:9px 14px;border-radius:9px;cursor:pointer;color:var(--muted);font-weight:600}
.tab.on{background:#fff;color:var(--ink);box-shadow:0 1px 4px rgba(15,23,42,.08)}
.tabbody{padding:18px}
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
button.ghost{background:transparent;color:var(--red);padding:5px 8px}
button.small{padding:7px 10px;font-size:12px}
button:disabled{opacity:.5;cursor:not-allowed}
.row{display:flex;gap:10px;align-items:end;flex-wrap:wrap}
.row > *{min-width:0}
.formbox{background:#fafafa;border:1px solid var(--line);border-radius:12px;padding:14px;margin-bottom:16px}
.form-title{font-weight:800;margin-bottom:10px}
.toolbar{display:flex;gap:10px;margin-bottom:12px;flex-wrap:wrap}
.toolbar input{max-width:280px}
.toolbar select{max-width:220px}
.tablewrap{border:1px solid var(--line);border-radius:12px;overflow:auto;max-height:560px}
table{border-collapse:collapse;width:100%;font-size:13px}
table.registry{table-layout:fixed}
table.registry td{overflow:hidden;text-overflow:ellipsis}
table.registry td:nth-child(2),table.registry td:nth-child(3){white-space:nowrap}
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
.side{display:grid;gap:16px}
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
.toast{position:fixed;right:20px;bottom:20px;background:#111827;color:#fff;padding:11px 15px;border-radius:10px;opacity:0;transform:translateY(8px);transition:.2s;pointer-events:none;z-index:9}
.toast.show{opacity:1;transform:none}
.toast.err{background:#991b1b}
.hidden{display:none!important}
@media(max-width:980px){.layout{grid-template-columns:1fr}.stats{grid-template-columns:1fr 1fr}.g4{grid-template-columns:1fr 1fr}.top .wrap{align-items:flex-start}}
</style>
</head>
<body>
<header class="top"><div class="wrap">
  <div class="brand"><div class="logo">LK</div><div><b>LK Advisor</b><small>Настройка правил рекомендаций</small></div></div>
  <nav><a href="/">Кабинет</a><a href="/api">API</a></nav>
</div></header>
<main class="wrap">
  <h1>Правила рекомендаций</h1>
  <p class="lead">Связи товаров, ограничения и веса меняются здесь и сразу попадают в движок. ИИ-помощник переводит фразу в изменение, показывает его и применяет только после подтверждения.</p>
  <div class="stats">
    <div class="stat"><b id="st-products">—</b><span>товаров в каталоге</span></div>
    <div class="stat"><b id="st-relations">—</b><span>связей между товарами</span></div>
    <div class="stat"><b id="st-rules">—</b><span>правил и ограничений</span></div>
    <div class="stat"><b id="st-active">—</b><span>включённых связей</span></div>
  </div>
  <div class="layout">
    <section class="panel">
      <div class="tabs">
        <button class="tab on" data-tab="relations">Связи товаров</button>
        <button class="tab" data-tab="registry">Каталог v5</button>
        <button class="tab" data-tab="validation">Валидация <span class="pill q hidden" id="review-badge">0</span></button>
        <button class="tab" data-tab="types">Типы связей</button>
        <button class="tab" data-tab="rules">Правила</button>
        <button class="tab" data-tab="weights">Веса</button>
        <button class="tab" data-tab="dataset">Данные</button>
        <button class="tab" data-tab="experiments">Гипотезы</button>
        <button class="tab" data-tab="analytics">Аналитика</button>
        <button class="tab" data-tab="assistant">ИИ-помощник</button>
      </div>
      <div class="tabbody">
        <div id="tab-relations">
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
            <input id="registry-search" placeholder="Поиск по товару или id" style="max-width:220px">
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
            <button class="secondary small" data-view="all">Все</button>
            <button class="secondary small" data-view="disputed">Спорные</button>
            <button class="secondary small" data-view="expiring">Истекают</button>
            <button class="secondary small" data-view="structural">Структурные</button>
            <button id="registry-generate" class="small">Сгенерировать</button>
            <button id="registry-sweep" class="secondary small">Проверить TTL</button>
            <span class="hint" id="registry-count"></span>
          </div>
          <div class="tablewrap"><table class="registry" style="min-width:1180px">
            <colgroup>
              <col style="width:36px"><col style="width:250px"><col style="width:250px">
              <col style="width:90px"><col style="width:90px"><col style="width:100px">
              <col style="width:78px"><col style="width:200px"><col style="width:130px">
              <col style="width:90px"><col style="width:60px">
            </colgroup>
            <thead><tr>
              <th><input type="checkbox" id="registry-all" style="width:auto"></th>
              <th>Источник</th><th>Цель</th><th>Тип</th><th>Статус</th>
              <th>Провенанс</th><th>Уверенность</th><th>Доказательство</th>
              <th>Семейство</th><th>Действует до</th><th>Версия</th>
            </tr></thead>
            <tbody id="registry-body"></tbody>
          </table></div>
          <div class="toolbar" style="margin-top:10px">
            <input id="registry-note" placeholder="Причина решения (обязательна для отказа)" style="max-width:360px">
            <button id="registry-confirm" class="small">Подтвердить выбранные</button>
            <button id="registry-reject" class="secondary small">Отклонить</button>
            <button id="registry-defer" class="secondary small">Отложить</button>
          </div>
          <div id="registry-detail" class="formbox" style="margin-top:12px"></div>
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
            <div class="form-title">Шаблон связи</div>
            <p class="hint">Название и описание идут в промпт разметки. Поведение говорит движку, как использовать связь: показывать как допродажу, апгрейд, замену или только хранить.</p>
            <div class="grid g3">
              <div><label>Код</label><input id="type-code" placeholder="industry_addon"></div>
              <div><label>Название</label><input id="type-title" placeholder="Отраслевое дополнение"></div>
              <div><label>Поведение</label><select id="type-behavior"></select></div>
              <div><label>Направление</label><select id="type-direction"><option value="forward">От купленного к предлагаемому</option><option value="symmetric">Симметричная</option></select></div>
              <div><label>Уверенность по умолчанию, %</label><input id="type-confidence" type="number" min="0" max="100" value="80"></div>
              <div><label>Статус</label><select id="type-status"><option value="active">Включён</option><option value="draft">Черновик</option><option value="disabled">Отключён</option></select></div>
            </div>
            <div class="grid" style="margin-top:10px">
              <div><label>Что и как связывать</label><textarea id="type-description" placeholder="Опишите, когда связь уместна и чем она отличается от замены или апгрейда."></textarea></div>
              <div><label>Примеры и контрпримеры</label><textarea id="type-examples" placeholder="X → Y; A → B. Не связывать: C и D, потому что..."></textarea></div>
            </div>
            <div class="row" style="margin-top:10px"><button id="type-save">Сохранить шаблон</button><button id="type-reset" class="secondary">Новый шаблон</button></div>
            <p class="hint" id="type-edit-hint"></p>
          </div>
          <div id="type-list"></div>
          <div class="formbox">
            <div class="form-title">Промпт для разметки связей</div>
            <p class="hint">Этот блок добавляется в промпт LLM. В нём только активные шаблоны.</p>
            <pre class="prompt" id="type-prompt">Загружаю…</pre>
            <div class="row" style="margin-top:10px"><button id="type-prompt-refresh" class="secondary small">Обновить промпт</button><button id="type-prompt-copy" class="secondary small">Скопировать</button></div>
          </div>
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

        <div id="tab-dataset" class="hidden">
          <div class="formbox">
            <div class="form-title">Датасет v3 и co-purchase</div>
            <p class="hint">Реальные 1 079 товаров, 32k строк продаж, 19 162 клиента. Связи строим из совместных покупок, затем отправляем в карантин.</p>
            <div class="stats" id="dataset-cards" style="margin:12px 0"></div>
            <div class="toolbar">
              <button id="dataset-classify">Классифицировать топ-10 через конструктор</button>
              <button id="dataset-suggest" class="secondary">Быстрые: adjacent_task</button>
              <span class="hint">Классификация использует активные шаблоны связей и LLM; результат уходит в карантин.</span>
            </div>
          </div>
          <div class="formbox">
            <div class="form-title">Кандидаты связей из данных</div>
            <div id="dataset-candidates"></div>
          </div>
          <div class="formbox">
            <div class="form-title">Сильные совместные покупки</div>
            <div id="dataset-pairs"></div>
          </div>
        </div>

        <div id="tab-experiments" class="hidden">
          <div class="toolbar">
            <button id="experiment-seed" class="secondary">Создать демо-гипотезу</button>
            <button id="experiment-refresh" class="secondary small">Обновить</button>
            <input id="experiment-search" type="search" placeholder="Поиск по базе знаний…" style="max-width:220px">
            <select id="experiment-decision-filter" style="max-width:170px">
              <option value="">Все решения</option>
              <option value="promote">promote</option>
              <option value="reject">reject</option>
              <option value="extend">extend</option>
            </select>
            <button id="experiment-search-btn" class="secondary small">Найти</button>
            <span class="hint">Синтетика: 50 клиентов × 10 сессий. Назначение → exposure → метрики → решение.</span>
          </div>
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
var REG_PROV = {manual: 'Ручная', co_purchase: 'Co-purchase', llm: 'LLM', attribute: 'Атрибуты', rule: 'Правило', import: 'Импорт'};
var REG_SELECTED = {};
var REG_PAGE = {items: [], total: 0};
var REG_VIEW = '';

function esc(v){ return String(v == null ? '' : v).replace(/[&<>"']/g, function(c){ return {'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]; }); }
function el(id){ return document.getElementById(id); }
function title(id){ for (var i=0;i<state.products.length;i++) if (state.products[i].id===id) return state.products[i].title; return id; }
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
  loadTypePrompt();
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
  el('type-behavior').innerHTML = Object.keys(BEHAVIORS).map(function(k){ return '<option value="'+k+'">'+BEHAVIORS[k]+'</option>'; }).join('');
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
  var status = p.status==='llm_suggested' ? 'предложение ИИ' : 'на проверке';
  var ev = (p.evidence||[]).map(function(e){ return '<div class="hint">• '+esc(e.value)+'</div>'; }).join('');
  var html = '<div class="preview-item" style="border-color:#bfdbfe"><b>'+esc(p.title)+' · '+esc(p.type)+'</b>'+
    '<small>Уверенность '+Math.round((p.confidence||0)*100)+'% · '+status+' · источник: '+esc(p.source==='llm'?'ИИ':'эвристика')+'</small>'+
    '<div style="margin-top:6px">'+esc(p.reason)+'</div>'+ev+
    '<div style="margin-top:8px"><button class="small" data-agent-add="suggest">Добавить как предложение</button> '+
    '<button class="small secondary" data-agent-add="confirm">Добавить и подтвердить</button></div></div>';
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
  var status = mode==='confirm' ? 'confirmed' : (p.status==='llm_suggested'?'llm_suggested':'quarantine');
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
async function loadExperiments(){
  try {
    var d=await api('GET','/v1/admin/experiments');
    var rows=d.items||[];
    if(!rows.length){ el('experiments-list').innerHTML='<div class="empty">Гипотез пока нет — создайте демо-гипотезу.</div>'; return; }
    el('experiments-list').innerHTML=rows.map(function(x){
      var res=x.results||[];
      var c=res.find(function(r){return r.variant==='control'})||{exposures:0,clicks:0,purchases:0,ctr:0,conversion:0};
      var t=res.find(function(r){return r.variant==='treatment'})||{exposures:0,clicks:0,purchases:0,ctr:0,conversion:0};
      var uplift=c.conversion>0?(t.conversion-c.conversion)/c.conversion*100:0;
      var table='<div class="tablewrap"><table><thead><tr><th>Вариант</th><th>Exposure</th><th>Клиенты</th><th>Клики</th><th>Покупки</th><th>CTR</th><th>Conversion</th><th>Target shown</th><th>Target conv</th></tr></thead><tbody>'+
        [['control',c],['treatment',t]].map(function(pair){
          var r=pair[1];
          return '<tr><td><b>'+esc(pair[0])+'</b></td><td>'+r.exposures+'</td><td>'+r.clients+'</td><td>'+r.clicks+'</td><td>'+r.purchases+'</td><td>'+pct(r.ctr)+'</td><td>'+pct(r.conversion)+'</td><td>'+r.target_exposures+'</td><td>'+pct(r.target_conversion)+'</td></tr>';
        }).join('')+'</tbody></table></div>';
      var guard='<div class="hint" style="margin-top:8px">Guardrails: renewal '+pct(c.renewal_rate)+' → '+pct(t.renewal_rate)+', upsell '+pct(c.upsell_rate)+' → '+pct(t.upsell_rate)+', fallback '+pct(c.fallback_rate)+' → '+pct(t.fallback_rate)+'</div>';
      var g=x.guardrail||{};
      var reasons=(g.reasons||[]).join('; ');
      var gBadge=g.blocked
        ? '<span class="pill" style="background:#fef3c7;color:#92400e">guardrail: '+esc(g.suggested||'extend')+'</span>'
        : '<span class="pill" style="background:#dcfce7;color:#166534">guardrail ok</span>';
      var guardWarn=g.blocked?'<div class="hint" style="color:#92400e;margin-top:6px">Guardrail: '+esc(reasons||'сработал')+' → рекомендуем '+esc(g.suggested||'extend')+'. Promote остановлен.</div>':'';
      var promoteDisabled=g.blocked?' disabled':'';
      var decisions='<div class="toolbar" style="margin-top:8px">'+
        '<button data-exp-decision="promote" data-exp-id="'+esc(x.id)+'"'+promoteDisabled+'>Подтвердить</button> '+
        '<button class="ghost" data-exp-decision="reject" data-exp-id="'+esc(x.id)+'">Отклонить</button> '+
        '<button class="secondary" data-exp-decision="extend" data-exp-id="'+esc(x.id)+'">Продлить</button>'+
        '<span class="hint">Uplift treatment: '+(uplift>=0?'+':'')+uplift.toFixed(1)+'% · эффект в показе target: '+c.target_exposures+' → '+t.target_exposures+'</span></div>';
      var decided=x.decision?'<div class="hint" style="margin-top:6px">Решение: <b>'+esc(x.decision)+'</b> '+esc(x.decision_note||'')+'</div>':'';
      return '<div class="formbox" style="margin-bottom:12px">'+
        '<div style="display:flex;justify-content:space-between;gap:12px;align-items:flex-start">'+
          '<div><div class="form-title" style="margin:0">'+esc(x.title)+'</div>'+
          '<div class="hint">'+esc(x.hypothesis)+'</div></div>'+
          '<div style="display:flex;gap:6px;align-items:center;flex-wrap:wrap">'+gBadge+'<span class="pill">'+esc(x.status)+'</span></div></div>'+
        '<div class="hint" style="margin:8px 0">Скоуп: '+esc(x.audience||'все')+' / '+esc(x.industry||'все')+' / '+esc(x.strategy||'все')+' · '+
          esc(x.from_title||x.from_id)+' → '+esc(x.to_title||x.to_id)+' · '+esc(x.metric)+'</div>'+
        table+guard+guardWarn+decisions+decided+'</div>';
    }).join('');
  } catch(e){ el('experiments-list').innerHTML='<div class="hint" style="color:#b91c1c">'+esc(e.message)+'</div>'; }
}
async function loadKnowledge(){
  try {
    var q=encodeURIComponent((el('experiment-search')||{}).value||'');
    var dec=encodeURIComponent((el('experiment-decision-filter')||{}).value||'');
    var d=await api('GET','/v1/admin/experiments/knowledge?q='+q+'&decision='+dec);
    var rows=d.items||[];
    var title='<div class="form-title" style="margin-top:18px">База знаний</div>';
    if(!rows.length){ el('experiment-knowledge').innerHTML=title+'<div class="empty">Пока нет завершённых гипотез</div>'; return; }
    el('experiment-knowledge').innerHTML=title+rows.map(function(x){
      return '<div class="formbox" style="margin-bottom:8px">'+
        '<div style="display:flex;justify-content:space-between;gap:12px"><b>'+esc(x.title)+'</b><span class="pill">'+esc(x.decision)+'</span></div>'+
        '<div class="hint" style="margin-top:4px">'+esc(x.hypothesis)+'</div>'+
        '<div class="hint">'+esc(x.audience||'все')+' / '+esc(x.industry||'все')+' / '+esc(x.strategy||'все')+' · '+esc(x.from_title||x.from_id)+' → '+esc(x.to_title||x.to_id)+'</div>'+
        (x.decision_note?'<div class="hint">'+esc(x.decision_note)+'</div>':'')+
      '</div>';
    }).join('');
  } catch(e){ el('experiment-knowledge').innerHTML='<div class="hint" style="color:#b91c1c">'+esc(e.message)+'</div>'; }
}
async function loadRelationPerf(){
  try {
    var d=await api('GET','/v1/admin/relations/performance');
    var rows=d.items||[];
    el('relation-perf').innerHTML = rows.length ? '<div class="tablewrap"><table><thead><tr><th>Связь</th><th>Тип</th><th>Скоуп</th><th>Показ</th><th>Клик</th><th>CTR</th><th>Покупка</th><th>Статус</th><th>До</th></tr></thead><tbody>'+
      rows.map(function(x){ return '<tr><td>'+esc(x.title_from)+' → '+esc(x.title_to)+'</td><td><span class="pill type">'+esc(x.type)+'</span></td><td>'+esc(x.audience||'—')+' / '+esc(x.industry||'—')+'</td><td>'+x.shown+'</td><td>'+x.clicks+'</td><td>'+pct(x.ctr)+'</td><td>'+x.purchases+'</td><td>'+esc(x.status)+'</td><td>'+(x.valid_until?new Date(x.valid_until).toLocaleDateString('ru-RU'):'—')+'</td></tr>'; }).join('')+'</tbody></table></div>' :
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
  var n=(counts.quarantine||0)+(counts.llm_suggested||0);
  var b=el('review-badge'); b.textContent=n; b.classList.toggle('hidden', n===0);
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
async function loadDataset(){
  try {
    var c=await api('GET','/v1/admin/dataset/coverage');
    var cards = [
      ['Товары', c.items], ['Публичные', c.public_items],
      ['Скрыто k<3', c.suppressed_items], ['B2C товары', c.b2c_items],
      ['Скрыто B2C k<5', c.suppressed_b2c_items], ['Клиенты', c.clients],
      ['Заказы', c.orders], ['Инстансы заказов', c.order_instances],
      ['В нескольких неделях', c.multi_week_orders], ['Пересекают срез', c.cross_cut_orders],
      ['Пары', c.pairs], ['Публичные пары', c.public_pairs],
      ['Публичные B2C пары', c.public_b2c_pairs],
      ['Пары ≥2', c.frequent_pairs], ['Без связей', c.solo_items],
      ['Кандидаты', c.candidates]
    ];
    var notes = '';
    if (c.cross_cut_orders) notes += '<div class="hint" style="width:100%;margin:4px 0 0;color:#92400e">Заказы, пересекающие срез, исключаются из офлайн-оценки: иначе будущее попадает в train.</div>';
    if (c.suppressed_items || c.suppressed_b2c_items) notes += '<div class="hint" style="width:100%;margin:4px 0 0;color:#92400e">Товары с <3 покупателями и B2C-товары с <5 физлицами не показываются поштучно в публичных ответах.</div>';
    el('dataset-cards').innerHTML = cards.map(function(x){ return '<div class="stat"><b>'+esc(x[1])+'</b><span>'+esc(x[0])+'</span></div>'; }).join('') + notes;
    var p=await api('GET','/v1/admin/dataset/co-purchases?limit=50');
    var cnd=await api('GET','/v1/admin/dataset/candidates?limit=50');
    var rows=cnd.items||[];
    el('dataset-candidates').innerHTML = rows.length ? '<div class="tablewrap"><table><thead><tr><th>A</th><th>B</th><th>Тип</th><th>Conf.</th><th>Статус</th><th>Причина</th><th></th></tr></thead><tbody>'+
      rows.map(function(x){ return '<tr><td>'+esc(x.title_a)+'</td><td>'+esc(x.title_b)+'</td><td><span class="pill type">'+esc(x.type)+'</span></td><td>'+Math.round(x.confidence*100)+'%</td><td>'+esc(regStatusLabel(x.status))+'</td><td>'+esc(x.reason)+'</td><td><button class="small" data-cand-review="confirm" data-cand-id="'+x.id+'">✓</button> <button class="ghost small" data-cand-review="reject" data-cand-id="'+x.id+'">✕</button></td></tr>'; }).join('')+'</tbody></table></div>' :
      '<div class="empty">Кандидатов пока нет — нажмите «Классифицировать».</div>';
    var rows=p.items||[];
    el('dataset-pairs').innerHTML = rows.length ? '<div class="tablewrap"><table><thead><tr><th>A</th><th>B</th><th>Раз</th><th>B2B</th><th>B2C</th></tr></thead><tbody>'+
      rows.map(function(x){ return '<tr><td>'+esc(x.title_a)+'<div class="hint">'+esc(x.item_a)+'</div></td><td>'+esc(x.title_b)+'<div class="hint">'+esc(x.item_b)+'</div></td><td>'+x.count+'</td><td>'+x.b2b_count+'</td><td>'+(x.b2c_hidden?'<span class="hint">скрыто k<5</span>':x.b2c_count)+'</td></tr>'; }).join('')+'</tbody></table></div>' :
      '<div class="empty">Нет пар. Импортируйте датасет v3.</div>';
  } catch(e){ el('dataset-cards').innerHTML='<div class="hint" style="color:#b91c1c">'+esc(e.message)+'</div>'; }
}
el('dataset-suggest').addEventListener('click', async function(){
  try { var d=await api('POST','/v1/admin/dataset/co-purchases/suggest',{limit:100,min_count:2}); toast('Создано предложений: '+(d.created||0)); loadDataset(); }
  catch(e){ toast(e.message,true); }
});
el('dataset-classify').addEventListener('click', async function(){
  toast('Классифицирую топ-10 через шаблоны…');
  try { var d=await api('POST','/v1/admin/dataset/co-purchases/classify',{limit:10}); toast('Обработано: '+d.processed+', создано: '+d.created+', пропущено: '+d.skipped); loadDataset(); }
  catch(e){ toast(e.message,true); }
});
el('dataset-candidates').addEventListener('click', async function(ev){
  var mode=ev.target.dataset.candReview, id=ev.target.dataset.candId;
  if (!mode || !id) return;
  var note='';
  if (mode==='reject') {
    note=(prompt('Причина отказа')||'').trim();
    if (!note) { toast('Нужна причина отказа', true); return; }
  }
  try { await api('POST','/v1/admin/dataset/candidates/review',{id:Number(id),decision:mode,note:note}); toast(mode==='confirm'?'Связь подтверждена':'Связь отклонена'); loadDataset(); }
  catch(e){ toast(e.message,true); }
});
function regStatusLabel(s){ return REG_STATUS[s] || s || '—'; }
function regProvLabel(p){ return REG_PROV[p] || p || '—'; }
function regWhen(v){ if(!v) return '—'; var d=new Date(v); return isNaN(d.getTime()) ? '—' : d.toLocaleDateString('ru-RU'); }
function regEvidence(ev){
  if(!ev || !ev.length) return '—';
  return ev.map(function(e){
    var label = (e.kind ? e.kind+': ' : '') + (e.summary || '');
    return '<span class="pill type">'+esc(label)+'</span>';
  }).join(' ');
}
function registryQuery(){
  var p=[];
  function add(k,v){ if(v!=='' && v!=null) p.push(encodeURIComponent(k)+'='+encodeURIComponent(v)); }
  add('q', el('registry-search').value.trim());
  add('type', el('registry-type').value);
  add('status', el('registry-status').value);
  add('source', el('registry-source').value);
  add('family', el('registry-family').value);
  if (REG_VIEW==='disputed') { add('min_conf','0.5'); add('max_conf','0.75'); }
  if (REG_VIEW==='expiring') { add('expiring','30'); }
  add('limit','200');
  return p.length ? '?'+p.join('&') : '';
}
async function loadRegistry(){
  REG_SELECTED={};
  if (el('registry-all')) el('registry-all').checked=false;
  try {
    var data = await api('GET','/v1/admin/registry/relations'+registryQuery());
    var metrics = await api('GET','/v1/admin/registry/metrics');
    REG_PAGE.items = data.items || [];
    REG_PAGE.total = data.total || REG_PAGE.items.length;
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
  var cards=[['Связей', m.total], ['Активных', m.active], ['Кандидатов', m.candidates],
             ['Покрытие', (m.coverage_pct||0).toFixed(1)+'%'],
             ['Stale', (m.stale_share||0).toFixed(1)+'%'],
             ['Хабы top-1%', (m.hub_top1_pct||0).toFixed(1)+'%']];
  el('registry-metrics').innerHTML = cards.map(function(x){
    return '<div class="stat"><b>'+esc(x[1])+'</b><span>'+esc(x[0])+'</span></div>';
  }).join('');
}
function renderRegistry(){
  var rows=REG_PAGE.items;
  el('registry-count').textContent = rows.length + ' из ' + REG_PAGE.total;
  if (!rows.length) {
    el('registry-body').innerHTML = '<tr><td colspan="11" class="empty">Связей нет. Нажмите «Сгенерировать».</td></tr>';
    return;
  }
  el('registry-body').innerHTML = rows.map(function(r){
    return '<tr data-reg-id="'+r.id+'">'+
      '<td><input type="checkbox" data-reg-check="'+r.id+'" style="width:auto"'+(REG_SELECTED[r.id]?' checked':'')+'></td>'+
      '<td>'+esc(r.from_title||r.from_item_id)+'</td>'+
      '<td>'+esc(r.to_title||r.to_item_id)+'</td>'+
      '<td><span class="pill type">'+esc(typeTitle(r.type))+'</span></td>'+
      '<td><span class="pill '+(r.status==='active'?'on':(r.status==='rejected'?'off':'q'))+'">'+esc(regStatusLabel(r.status))+'</span></td>'+
      '<td>'+esc(regProvLabel(r.provenance_kind))+'</td>'+
      '<td>'+Math.round((r.confidence||0)*100)+'%</td>'+
      '<td>'+regEvidence(r.evidence)+'</td>'+
      '<td>'+esc(r.from_family||'—')+'</td>'+
      '<td>'+regWhen(r.valid_until)+'</td>'+
      '<td>v'+esc(r.version||1)+'</td>'+
    '</tr>';
  }).join('');
}
function registryItem(id){
  for (var i=0;i<REG_PAGE.items.length;i++) if (String(REG_PAGE.items[i].id)===String(id)) return REG_PAGE.items[i];
  return null;
}
function renderRegistryDetail(id){
  var r=registryItem(id); if (!r) return;
  el('registry-detail').innerHTML =
    '<div class="form-title">'+esc(r.from_title||r.from_item_id)+' → '+esc(r.to_title||r.to_item_id)+'</div>'+
    '<div class="hint">'+esc(typeTitle(r.type))+' · '+esc(regStatusLabel(r.status))+' · '+esc(regProvLabel(r.provenance_kind))+' · '+Math.round((r.confidence||0)*100)+'%</div>'+
    '<div style="margin:8px 0">'+regEvidence(r.evidence)+'</div>'+
    '<div class="hint">Семейство: '+esc(r.from_family||'—')+' → '+esc(r.to_family||'—')+' · версия '+esc(r.version||1)+
      (r.rejection_reason?' · причина отказа: '+esc(r.rejection_reason):'')+'</div>'+
    '<div class="toolbar" style="margin-top:8px">'+
      '<button class="small" data-reg-review="confirm" data-reg-review-id="'+r.id+'">Подтвердить</button>'+
      '<button class="secondary small" data-reg-review="reject" data-reg-review-id="'+r.id+'">Отклонить</button>'+
      '<button class="secondary small" data-reg-review="defer" data-reg-review-id="'+r.id+'">Отложить</button>'+
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
      '<td><span class="pill type">'+esc(typeTitle(r.type))+'</span></td>'+
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
      '<div style="margin-top:8px"><button class="secondary small" data-type-edit="'+esc(t.code)+'">Изменить</button> '+
      (t.builtin?'':'<button class="ghost small" data-type-del="'+esc(t.code)+'">Удалить</button>')+'</div></div>';
  }).join('');
}
function resetTypeForm(){
  el('type-code').value=''; el('type-title').value=''; el('type-behavior').value='cross_sell';
  el('type-direction').value='forward'; el('type-confidence').value=80; el('type-status').value='active';
  el('type-description').value=''; el('type-examples').value='';
  el('type-code').disabled=false; el('type-behavior').disabled=false; el('type-direction').disabled=false;
  el('type-confidence').disabled=false; el('type-edit-hint').textContent='';
}
function editType(code){
  var t=null; (state.relation_types||[]).forEach(function(x){ if(x.code===code) t=x; });
  if (!t) return;
  el('type-code').value=t.code; el('type-title').value=t.title; el('type-behavior').value=t.behavior;
  el('type-direction').value=t.direction; el('type-confidence').value=Math.round((t.default_confidence||0)*100);
  el('type-status').value=t.status; el('type-description').value=t.description||''; el('type-examples').value=t.examples||'';
  el('type-code').disabled=true;
  el('type-behavior').disabled=t.builtin; el('type-direction').disabled=t.builtin; el('type-confidence').disabled=t.builtin;
  el('type-edit-hint').textContent = t.builtin ? 'Встроенный шаблон: код, поведение и направление зафиксированы. Описание, примеры и статус можно менять.' : 'Пользовательский шаблон можно менять полностью.';
  el('type-title').focus();
}
async function loadTypePrompt(){
  try { var p=await api('GET','/v1/admin/relation-types/prompt'); el('type-prompt').textContent=p.prompt; }
  catch(e){ el('type-prompt').textContent='Не удалось загрузить промпт: '+e.message; }
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

document.querySelectorAll('.tab').forEach(function(b){
  b.addEventListener('click', function(){
    document.querySelectorAll('.tab').forEach(function(x){x.classList.remove('on');});
    document.querySelectorAll('.tabbody > div').forEach(function(x){x.classList.add('hidden');});
    b.classList.add('on'); el('tab-'+b.dataset.tab).classList.remove('hidden');
    document.querySelector('.layout').classList.toggle('registry-mode', b.dataset.tab === 'registry');
    if (b.dataset.tab === 'analytics') loadAnalytics();
    if (b.dataset.tab === 'dataset') loadDataset();
    if (b.dataset.tab === 'registry') loadRegistry();
    if (b.dataset.tab === 'experiments') { loadExperiments(); loadKnowledge(); }
    if (b.dataset.tab === 'validation') loadReview();
  });
});
el('registry-search').addEventListener('keyup', function(ev){ if (ev.key==='Enter') loadRegistry(); });
['registry-type','registry-status','registry-source','registry-family'].forEach(function(id){
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
el('registry-body').addEventListener('click', function(ev){
  if (ev.target.dataset.regCheck) return;
  var tr=ev.target.closest('tr[data-reg-id]');
  if (tr) renderRegistryDetail(tr.dataset.regId);
});
el('registry-detail').addEventListener('click', function(ev){
  var d=ev.target.dataset.regReview, id=ev.target.dataset.regReviewId;
  if (d && id) registryReview(id,d);
});
el('registry-confirm').addEventListener('click', function(){ registryBulk('confirm'); });
el('registry-reject').addEventListener('click', function(){ registryBulk('reject'); });
el('registry-defer').addEventListener('click', function(){ registryBulk('defer'); });
el('registry-generate').addEventListener('click', registryGenerate);
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
el('type-reset').addEventListener('click', function(){ resetTypeForm(); el('type-code').disabled=false; });
el('type-save').addEventListener('click', async function(){
  var body={
    code:el('type-code').value.trim(), title:el('type-title').value.trim(),
    behavior:el('type-behavior').value, direction:el('type-direction').value,
    default_confidence:Number(el('type-confidence').value)/100,
    status:el('type-status').value, description:el('type-description').value.trim(),
    examples:el('type-examples').value.trim()
  };
  try { await api('POST','/v1/admin/relation-types', body); toast('Шаблон сохранён'); resetTypeForm(); await load(); }
  catch(e){ toast(e.message,true); }
});
el('type-list').addEventListener('click', async function(ev){
  var edit=ev.target.dataset.typeEdit, del=ev.target.dataset.typeDel;
  if (edit) { editType(edit); return; }
  if (del) {
    if (!confirm('Удалить шаблон '+del+'? Связи этого типа останутся, но станут неизвестными.')) return;
    try { await api('DELETE','/v1/admin/relation-types/'+encodeURIComponent(del)); toast('Шаблон удалён'); resetTypeForm(); await load(); }
    catch(e){ toast(e.message,true); }
  }
});
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
el('experiment-seed').addEventListener('click', async function(){
  try { await api('POST','/v1/admin/experiments/demo',{}); toast('Демо-гипотеза создана'); loadExperiments(); loadKnowledge(); }
  catch(e){ toast(e.message,true); }
});
el('experiment-refresh').addEventListener('click', function(){ loadExperiments(); loadKnowledge(); });
el('experiment-search-btn').addEventListener('click', loadKnowledge);
el('experiment-decision-filter').addEventListener('change', loadKnowledge);
el('experiment-search').addEventListener('keydown', function(e){ if(e.key==='Enter') loadKnowledge(); });
el('experiments-list').addEventListener('click', async function(ev){
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
el('type-prompt-refresh').addEventListener('click', loadTypePrompt);
el('type-prompt-copy').addEventListener('click', async function(){
  try { await navigator.clipboard.writeText(el('type-prompt').textContent); toast('Промпт скопирован'); }
  catch(e){ toast('Не удалось скопировать', true); }
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
if (location.hash) {
  var initialTab = document.querySelector('.tab[data-tab="' + location.hash.slice(1) + '"]');
  if (initialTab) initialTab.click();
}
load().then(loadReview).catch(function(e){ toast('Не удалось загрузить состояние: '+e.message, true); });
</script>
</body>
</html>`
