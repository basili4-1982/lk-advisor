package httpapi

import (
	"net/http"
	"strings"
)

// landingPage is the entry point of the demo: it routes to the client
// storefront, the admin and the relation graph instead of the legacy cabinet.
func (h *handler) landingPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(landingHTML))
}

var landingHTML = strings.TrimSpace(`
<!doctype html>
<html lang="ru">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>LK Advisor — демо</title>
<style>
:root{--ink:#0f172a;--muted:#64748b;--line:#e2e8f0;--blue:#2563eb}
*{box-sizing:border-box}
body{margin:0;background:#f6f7fb;color:var(--ink);font:16px/1.5 -apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,Arial,sans-serif}
.top{background:linear-gradient(135deg,#070b14 0%,#111827 60%,#1f2937 100%);color:#fff;padding:22px 0}
.wrap{max-width:1080px;margin:0 auto;padding:0 20px}
.brand{display:flex;align-items:center;gap:12px}
.logo{background:linear-gradient(135deg,#e11d48,#991b1b);color:#fff;font-weight:800;border-radius:11px;width:44px;height:44px;display:grid;place-items:center}
.brand b{display:block;font-size:19px}
.brand small{color:#94a3b8}
main{padding:32px 0 48px}
h1{font-size:26px;margin:0 0 6px}
.lead{color:var(--muted);margin:0 0 24px;max-width:760px}
.grid{display:grid;grid-template-columns:repeat(2,1fr);gap:16px}
.card{display:block;background:#fff;border:1px solid var(--line);border-radius:14px;padding:20px;text-decoration:none;color:inherit;box-shadow:0 4px 18px rgba(15,23,42,.05);transition:.15s}
.card:hover{transform:translateY(-2px);box-shadow:0 12px 30px rgba(15,23,42,.1);border-color:#c7d2fe}
.card b{display:block;font-size:18px;margin-bottom:6px}
.card span{color:var(--muted);font-size:14px}
.badge{display:inline-block;padding:3px 9px;border-radius:999px;background:#eef2ff;color:var(--blue);font-size:12px;font-weight:700;margin-top:12px}
.foot{color:#94a3b8;font-size:13px;margin-top:28px}
@media(max-width:820px){.grid{grid-template-columns:1fr}}
</style>
</head>
<body>
<header class="top"><div class="wrap"><div class="brand">
  <div class="logo">LK</div>
  <div><b>LK Advisor</b><small>связи товаров и проверка гипотез</small></div>
</div></div></header>
<main class="wrap">
  <h1>Демо-стенд</h1>
  <p class="lead">Закрытый показ на реальном датасете: витрина клиента, рабочий каталог связей и граф.</p>
  <div class="grid">
    <a class="card" href="/lk">
      <b>Витрина клиента</b>
      <span>Что видит клиент: топ-3, причины, клик и покупка. Системная информация — по отдельной кнопке.</span>
      <span class="badge">/lk</span>
    </a>
    <a class="card" href="/admin">
      <b>Админка</b>
      <span>Дашборд качества, каталог связей, гипотезы и решения.</span>
      <span class="badge">/admin</span>
    </a>
    <a class="card" href="/agent">
      <b>Как работает агент</b>
      <span>Сигналы из открытых источников, выбор типа связи и холодный старт.</span>
      <span class="badge">/agent</span>
    </a>
    <a class="card" href="/materials/">
      <b>Материалы</b>
      <span>One-pager, архитектура, метрики, API, инструкция и безопасность.</span>
      <span class="badge">/materials/</span>
    </a>
  </div>
  <div class="foot">Сырые данные датасета не публикуются. Демо доступно только на закрытом стенде.</div>
</main>
</body>
</html>
`)
