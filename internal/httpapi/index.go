package httpapi

import "net/http"

const apiHTML = `<!doctype html>
<html lang="ru">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>LK Advisor — API</title>
<style>
  body { font-family: Arial, sans-serif; max-width: 760px; margin: 40px auto; padding: 0 16px; color: #222; line-height: 1.5; }
  h1 { font-size: 24px; margin-bottom: 4px; }
  .muted { color: #666; }
  code, pre { background: #f4f4f4; border-radius: 4px; }
  code { padding: 2px 5px; }
  pre { padding: 12px; overflow-x: auto; }
  li { margin: 4px 0; }
</style>
</head>
<body>
<h1>LK Advisor — API</h1>
<p class="muted">Бэкенд рекомендаций для личного кабинета. <a href="/">Открыть демо-кабинет</a></p>

<h2>Эндпоинты</h2>
<ul>
  <li><code>GET /healthz</code> — проверка живости</li>
  <li><code>POST /v1/recommendations</code> — top-3 рекомендации по клиенту</li>
  <li><code>POST /v1/ask</code> — вопрос по рекомендациям, инструменты и evidence</li>
</ul>

<h2>Примеры</h2>
<pre>curl -s localhost:8081/healthz

curl -s -X POST localhost:8081/v1/recommendations \
  -H 'Content-Type: application/json' \
  -d '{"client_id":"smb-002"}'

curl -s -X POST localhost:8081/v1/ask \
  -H 'Content-Type: application/json' \
  -d '{"client_id":"smb-002","question":"Почему векторный?"}'</pre>

<p class="muted">Демо целиком: <code>BASE=http://localhost:8081 ./docs/demo.sh</code></p>
</body>
</html>
`

func (h *handler) apiIndex(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(apiHTML))
}
