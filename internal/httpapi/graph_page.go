package httpapi

const graphPageHTML = `<!doctype html>
<html lang="ru">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Граф связей — LK Advisor</title>
<style>
:root{--bg:#070b14;--panel:#0b1220;--line:rgba(148,163,184,.16);--text:#e2e8f0;--muted:#94a3b8;--blue:#60a5fa}
*{box-sizing:border-box}
html,body{height:100%}
body{margin:0;background:radial-gradient(1100px 700px at 30% 0%,#12203a 0%,var(--bg) 55%);color:var(--text);font-family:Inter,ui-sans-serif,system-ui,-apple-system,"Segoe UI",sans-serif;overflow:hidden}
.top{height:72px;display:flex;align-items:center;justify-content:space-between;gap:16px;padding:0 20px;border-bottom:1px solid var(--line);background:rgba(7,11,20,.82);backdrop-filter:blur(10px);position:relative;z-index:4}
.brand{display:flex;align-items:center;gap:12px;min-width:0}
.logo{width:38px;height:38px;border-radius:11px;background:linear-gradient(135deg,#c8102e,#7f1020);display:flex;align-items:center;justify-content:center;font-weight:800;font-size:13px;letter-spacing:.5px;box-shadow:0 0 24px rgba(200,16,46,.35)}
.brand h1{font-size:17px;margin:0;line-height:1.1}
.brand p{margin:3px 0 0;color:var(--muted);font-size:12px}
.controls{display:flex;align-items:center;gap:10px;flex-wrap:wrap;justify-content:flex-end}
.seg{display:flex;background:rgba(15,23,42,.8);border:1px solid var(--line);border-radius:9px;padding:3px}
.seg button{border:0;background:transparent;color:var(--muted);padding:6px 12px;border-radius:7px;font:inherit;font-size:13px;cursor:pointer}
.seg button.active{background:#1d4ed8;color:#fff;box-shadow:0 4px 18px rgba(29,78,216,.35)}
input[type=search]{background:rgba(15,23,42,.8);border:1px solid var(--line);border-radius:9px;color:var(--text);padding:8px 12px;font:inherit;font-size:13px;width:230px;outline:none}
input[type=search]:focus{border-color:rgba(96,165,250,.55)}
label.toggle{display:flex;align-items:center;gap:7px;color:var(--muted);font-size:13px;cursor:pointer}
label.toggle input{accent-color:#1d4ed8}
.status{color:var(--muted);font-size:12px;min-width:70px;text-align:right}
.link{color:var(--muted);text-decoration:none;font-size:13px;border:1px solid var(--line);padding:7px 11px;border-radius:9px}
.link:hover{color:#fff;border-color:rgba(96,165,250,.5)}
.filterbar{height:52px;display:flex;align-items:center;gap:8px;padding:0 20px;background:rgba(11,18,32,.9);border-bottom:1px solid var(--line);overflow-x:auto}
.filterbar .seg{flex:none}
.timebox{display:flex;align-items:center;gap:8px;background:rgba(15,23,42,.8);border:1px solid var(--line);border-radius:9px;padding:5px 10px;flex:none}
.timebox input[type=range]{width:120px;accent-color:#1d4ed8}
.timebox span{font-size:11.5px;color:var(--muted);white-space:nowrap;min-width:96px;text-align:right}
.filter-select{background:rgba(15,23,42,.8);border:1px solid var(--line);border-radius:9px;color:var(--text);padding:8px 10px;font:inherit;font-size:12.5px;max-width:190px;outline:none;flex:none}
.filter-select:focus{border-color:rgba(96,165,250,.55)}
.pager{position:absolute;bottom:14px;left:50%;transform:translateX(-50%);display:flex;gap:6px;align-items:center;background:rgba(11,18,32,.9);border:1px solid var(--line);border-radius:10px;padding:5px 7px;z-index:3}
.pager button{background:rgba(15,23,42,.9);border:1px solid var(--line);color:var(--text);border-radius:7px;padding:4px 9px;cursor:pointer;font:inherit;font-size:14px}
.pager button:disabled{opacity:.3;cursor:default}
.pager span{font-size:11px;color:var(--muted);min-width:52px;text-align:center}
.pager select{background:rgba(15,23,42,.9);border:1px solid var(--line);color:var(--muted);border-radius:7px;padding:4px 6px;font:inherit;font-size:11px}
.zoomctl{position:absolute;right:14px;bottom:56px;display:flex;flex-direction:column;gap:5px;z-index:3}
.zoomctl button{width:32px;height:32px;background:rgba(11,18,32,.9);border:1px solid var(--line);color:var(--text);border-radius:8px;cursor:pointer;font:inherit;font-size:17px;line-height:1}
.zoomctl button:hover{border-color:rgba(96,165,250,.6);color:#fff}
.zoomctl button.reset{font-size:13px}
main{display:grid;grid-template-columns:1fr 390px;height:calc(100vh - 124px)}
.graph-wrap{position:relative;min-width:0;overflow:hidden;height:100%}
#graph{position:absolute;inset:0}
#graph canvas{display:block;width:100%;height:100%}
.stats{position:absolute;top:14px;left:14px;display:flex;gap:8px;flex-wrap:wrap;max-width:calc(100% - 28px);z-index:2;pointer-events:none}
.stat{background:rgba(11,18,32,.84);border:1px solid var(--line);border-radius:10px;padding:8px 11px;min-width:96px;box-shadow:0 8px 26px rgba(0,0,0,.22)}
.stat b{display:block;font-size:16px;line-height:1.1}
.stat span{display:block;color:var(--muted);font-size:10.5px;margin-top:2px}
.legend{position:absolute;left:14px;bottom:14px;display:flex;gap:6px;flex-wrap:wrap;max-width:calc(100% - 28px);z-index:2}
.leg{display:flex;align-items:center;gap:6px;background:rgba(11,18,32,.84);border:1px solid var(--line);border-radius:999px;padding:5px 9px;color:var(--muted);font-size:11px;cursor:pointer;user-select:none}
.leg i{width:8px;height:8px;border-radius:50%;display:block}
.leg.active{color:#fff;border-color:rgba(96,165,250,.55);background:rgba(29,78,216,.22)}
.hint{position:absolute;right:14px;bottom:14px;color:#64748b;font-size:11px;max-width:300px;text-align:right;z-index:2}
.panel{background:linear-gradient(180deg,rgba(11,18,32,.98),rgba(7,11,20,.98));border-left:1px solid var(--line);padding:18px;overflow:auto;min-width:0}
.panel .empty{color:var(--muted);font-size:13px;line-height:1.6;border:1px dashed var(--line);border-radius:12px;padding:16px}
.panel-kicker{color:var(--blue);font-size:11px;text-transform:uppercase;letter-spacing:.12em;margin-bottom:8px}
.panel h2{font-size:18px;line-height:1.35;margin:0 0 14px;font-weight:650}
.panel h3{font-size:12px;text-transform:uppercase;letter-spacing:.1em;color:var(--muted);margin:20px 0 8px}
.metrics{display:grid;grid-template-columns:repeat(3,1fr);gap:8px;margin-bottom:14px}
.metrics div{background:rgba(15,23,42,.72);border:1px solid var(--line);border-radius:10px;padding:9px}
.metrics b{display:block;font-size:17px}
.metrics span{display:block;color:var(--muted);font-size:10px;margin-top:2px}
.note{background:rgba(251,191,36,.08);border:1px solid rgba(251,191,36,.22);color:#fde68a;border-radius:10px;padding:11px;font-size:12px;line-height:1.5}
.nb{display:flex;align-items:center;justify-content:space-between;gap:10px;width:100%;text-align:left;background:rgba(15,23,42,.55);border:1px solid var(--line);border-radius:9px;padding:9px 10px;color:var(--text);font:inherit;font-size:12px;cursor:pointer;margin-bottom:6px}
.nb:hover{border-color:rgba(96,165,250,.5);background:rgba(29,78,216,.14)}
.nb span{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.nb b{color:var(--blue);font-size:12px;flex:none}
.foot{color:#64748b;font-size:11px;line-height:1.5;margin-top:18px;border-top:1px solid var(--line);padding-top:12px}
#tooltip{position:fixed;display:none;z-index:20;background:rgba(2,6,23,.94);border:1px solid rgba(96,165,250,.35);border-radius:8px;padding:6px 9px;font-size:11.5px;max-width:340px;pointer-events:none;box-shadow:0 10px 30px rgba(0,0,0,.35)}
@media(max-width:980px){main{grid-template-columns:1fr}.panel{display:none}.stats .stat{min-width:80px}}
</style>
</head>
<body>
<header class="top">
  <div class="brand">
    <div class="logo">LK</div>
    <div><h1>Граф связей</h1><p>Публичный слой датасета v3 · совместные покупки</p></div>
  </div>
  <div class="controls">
    <input id="search" type="search" placeholder="Найти товар…">
    <label class="toggle"><input id="solo" type="checkbox"> solo</label>
    <a class="link" href="/">Кабинет</a>
    <span class="status" id="status"></span>
  </div>
</header>
<div class="filterbar">
  <div class="seg">
    <button data-audience="b2b" class="active">B2B</button>
    <button data-audience="b2c">B2C</button>
  </div>
  <select id="family-filter" class="filter-select" title="Семейство"></select>
  <select id="industry-filter" class="filter-select" title="Отрасль"></select>
  <select id="segment-filter" class="filter-select" title="Сегмент"></select>
  <div class="timebox"><input id="time" type="range" min="0" max="0" value="0"><span id="period-label">Все время</span></div>
</div>
<main>
  <section class="graph-wrap" id="graph-wrap">
    <div class="stats" id="stats"></div>
    <div id="graph"></div>
    <div class="legend" id="legend"></div>
    <div class="hint">Клик по узлу — карточка и связи. Вращение — мышь, колесо — масштаб.</div>
    <div class="pager"><button id="prev-page" title="Назад">‹</button><span id="page-label">1 / 1</span><button id="next-page" title="Вперёд">›</button><select id="page-size" title="Точек на странице"><option value="250">250</option><option value="500">500</option><option value="1000" selected>1000</option></select></div>
    <div class="zoomctl"><button id="zoom-in" title="Приблизить">+</button><button id="zoom-out" title="Отдалить">−</button><button id="zoom-reset" class="reset" title="Сбросить камеру">⌂</button></div>
  </section>
  <aside class="panel" id="panel"></aside>
</main>
<div id="tooltip"></div>
<script src="/static/3d-force-graph.min.js"></script>
<script>
const $=id=>document.getElementById(id);
const wrap=$('graph-wrap');
const COLORS={security:'#f87171',office:'#60a5fa',graphics:'#c084fc',backup:'#4ade80',devtools:'#fbbf24',training:'#2dd4bf',other:'#94a3b8'};
const CAT_RU={security:'Безопасность',office:'Офис',graphics:'Графика',backup:'Бэкап',devtools:'Разработка',training:'Обучение',other:'Другое'};
let DATA=null,nodes=[],edges=[],byId={},selected=null,hover=null;
let audience='b2b',soloOn=false,categoryFilter=null,search='',initialIndustry=new URLSearchParams(location.search).get('industry')||'',initialSegment=new URLSearchParams(location.search).get('segment')||'',initialFamily=new URLSearchParams(location.search).get('family')||'',industryFilter=initialIndustry,segmentFilter=initialSegment,familyFilter=initialFamily;
let periods=[],currentPeriod='',initialPeriod=new URLSearchParams(location.search).get('period')||'',page=Math.max(1,parseInt(new URLSearchParams(location.search).get('page')||'1',10)||1),pageSize=Math.max(250,Math.min(1000,parseInt(new URLSearchParams(location.search).get('page_size')||'1000',10)||1000)),totalPages=1;
let graph=null,autoFitDone=false,userInteracted=false;
function esc(s){return String(s==null?'':s).replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]))}
function fmt(n){return Number(n||0).toLocaleString('ru-RU')}
function sourceID(l){return l.source&&l.source.id?l.source.id:l.source}
function targetID(l){return l.target&&l.target.id?l.target.id:l.target}
function initGraph(){
  graph=new ForceGraph3D($('graph'),{controlType:'orbit'})
    .backgroundColor('#070b14')
    .showNavInfo(false)
    .nodeId('id')
    .nodeVal(nodeVal)
    .nodeColor(nodeColor)
    .nodeLabel(n=>n.title+(n.external_count?' · '+n.external_count+' за страницей':''))
    .nodeOpacity(.92)
    .nodeRelSize(7)
    .linkColor(linkColor)
    .linkWidth(linkWidth)
    .linkOpacity(.58)
    .linkVisibility(linkVisible)
    .nodeVisibility(nodeVisible)
    .onNodeClick(n=>{showNode(n);focusNode(n)})
    .onNodeHover(n=>{hover=n;refresh()});
  if(graph.d3Force){
    const charge=graph.d3Force('charge');if(charge&&charge.strength)charge.strength(-35);
    const link=graph.d3Force('link');if(link&&link.distance)link.distance(45);
  }
  if(graph.cooldownTicks)graph.cooldownTicks(220);
  resizeGraph();
}
function resizeGraph(){if(graph)graph.width(wrap.clientWidth).height(wrap.clientHeight)}
function nodeVisible(n){
  if(n.solo&&!soloOn)return false;
  if(categoryFilter&&n.category!==categoryFilter)return false;
  if(familyFilter&&n.family!==familyFilter)return false;
  return true;
}
function linkVisible(l){
  const a=byId[sourceID(l)],b=byId[targetID(l)];
  return !!(a&&b&&nodeVisible(a)&&nodeVisible(b));
}
function nodeVal(n){
  if(DATA&&DATA.period){
    if(n.demand_hidden)return 2;
    return 6+Math.min(24,(n.period_buyers||0)/6);
  }
  return n.solo?2:5+Math.min(22,Math.sqrt((n.degree||1))*4);
}
function nodeColor(n){
  const q=search.trim().toLowerCase();
  const dim=(q&&n.title.toLowerCase().indexOf(q)<0)||(categoryFilter&&n.category!==categoryFilter);
  if(selected&&selected.id===n.id)return '#ffffff';
  if(hover&&hover.id===n.id)return '#f8fafc';
  if(q&&n.title.toLowerCase().indexOf(q)>=0)return '#f8fafc';
  if(dim)return '#1e293b';
  if(n.external_count>0)return '#fbbf24';
  if(n.solo)return '#64748b';
  return COLORS[n.category]||COLORS.other;
}
function linkWidth(l){
  if(selected&&(sourceID(l)===selected.id||targetID(l)===selected.id))return 1.8;
  if(hover&&(sourceID(l)===hover.id||targetID(l)===hover.id))return 1.5;
  return Math.min(3.4,.8+Math.log((l.count||1)+1)*.35);
}
function linkColor(l){
  if(selected&&(sourceID(l)===selected.id||targetID(l)===selected.id))return 'rgba(96,165,250,.85)';
  if(hover&&(sourceID(l)===hover.id||targetID(l)===hover.id))return 'rgba(96,165,250,.55)';
  return 'rgba(148,163,184,.4)';
}
function refresh(){
  if(!graph)return;
  graph.nodeVal(nodeVal);graph.nodeColor(nodeColor);graph.nodeVisibility(nodeVisible);
  graph.linkColor(linkColor);graph.linkWidth(linkWidth);graph.linkVisibility(linkVisible);
  graph.refresh();
}
function fillFilter(id, items, allLabel, current){
  const s=$(id);
  s.innerHTML='<option value="">'+allLabel+'</option>'+(items||[]).map(o=>'<option value="'+esc(o.value)+'">'+esc(o.label)+' ('+o.count+')</option>').join('');
  s.value=current||'';
}
function populateFilters(){
  fillFilter('family-filter',DATA.families,'Все семейства',familyFilter);
  fillFilter('industry-filter',DATA.industries,'Все отрасли',industryFilter);
  fillFilter('segment-filter',DATA.segments,'Все сегменты',segmentFilter);
}
async function load(focusId){
  $('status').textContent='Загружаю…';
  const r=await fetch('/v1/demo/graph?audience='+audience+'&industry='+encodeURIComponent(industryFilter)+'&segment='+encodeURIComponent(segmentFilter)+'&period='+encodeURIComponent(initialPeriod)+'&page='+page+'&page_size='+pageSize+'&solo_limit=1000');
  if(!r.ok)throw new Error('HTTP '+r.status);
  DATA=await r.json();
  periods=DATA.periods||[];currentPeriod=DATA.period||'';
  page=DATA.page||1;totalPages=DATA.total_pages||1;pageSize=DATA.page_size||pageSize;
  populateFilters();updatePager();
  selected=null;hover=null;showNode(null);
  nodes=DATA.nodes.map(n=>Object.assign({},n));
  byId={};nodes.forEach(n=>byId[n.id]=n);
  edges=DATA.edges.map(e=>Object.assign({},e));
  if(!graph)initGraph();
  graph.graphData({nodes:nodes,links:edges});
  autoFitDone=false;userInteracted=false;
  drawStats();drawLegend();updateTimeControl();
  if(focusId&&byId[focusId]){showNode(byId[focusId]);setTimeout(()=>focusNode(byId[focusId]),700)}
  else selectInitial();
  setTimeout(()=>{if(!userInteracted&&!autoFitDone&&graph&&graph.zoomToFit){graph.zoomToFit(700,90,n=>nodeVisible(n));autoFitDone=true}},1200);
  $('status').textContent='';
}
function applyPeriod(d){
  const map={};d.nodes.forEach(n=>{map[n.id]=n});
  nodes.forEach(n=>{const x=map[n.id];if(!x)return;n.period_buyers=x.period_buyers;n.prev_buyers=x.prev_buyers;n.growth=x.growth;n.demand_hidden=x.demand_hidden;n.growth_hidden=x.growth_hidden;n.renewal_share=x.renewal_share;n.renewal_days=x.renewal_days;n.renewal_hidden=x.renewal_hidden});
  DATA.period=d.period;DATA.period_label=d.period_label;DATA.period_items=d.period_items;DATA.period_public_items=d.period_public_items;DATA.period_hidden_items=d.period_hidden_items;
  currentPeriod=d.period;initialPeriod=d.period;updateTimeControl();drawStats();refresh();if(selected)showNode(byId[selected.id]||selected);
}
async function loadPeriod(value){
  $('status').textContent='Считаю…';
  const r=await fetch('/v1/demo/graph?audience='+audience+'&industry='+encodeURIComponent(industryFilter)+'&segment='+encodeURIComponent(segmentFilter)+'&period='+encodeURIComponent(value)+'&page='+page+'&page_size='+pageSize+'&solo_limit=1000');
  if(!r.ok)throw new Error('HTTP '+r.status);
  applyPeriod(await r.json());
  $('status').textContent='';
}
function zoomBy(factor){
  if(!graph)return;
  userInteracted=true;autoFitDone=true;
  const cam=graph.cameraPosition();
  if(!cam)return;
  let target={x:0,y:0,z:0};
  if(graph.controls&&graph.controls()){const c=graph.controls();if(c.target&&typeof c.target.x==='number')target={x:c.target.x,y:c.target.y,z:c.target.z}}
  graph.cameraPosition({
    x: target.x+(cam.x-target.x)*factor,
    y: target.y+(cam.y-target.y)*factor,
    z: target.z+(cam.z-target.z)*factor
  }, target, 250);
}
function resetCamera(){userInteracted=true;autoFitDone=true;if(graph&&graph.zoomToFit)graph.zoomToFit(700,90,n=>nodeVisible(n))}
function updatePager(){
  $('page-label').textContent=page+' / '+totalPages;
  $('prev-page').disabled=page<=1;
  $('next-page').disabled=page>=totalPages;
  $('page-size').value=String(pageSize);
}
function drawStats(){
  const c=DATA.coverage;
  const segment=(DATA.industry||DATA.segment);
  const cards=segment
    ? [['Товары в сегменте',DATA.segment_items],['Публичные',DATA.public_items],['Связи',DATA.public_pairs],['В связях',DATA.connected],['Solo',DATA.public_solo],['Скрыто k',DATA.segment_hidden]]
    : [['Публичные товары',c.public_items],['Публичные связи',c.public_pairs],['В связях',DATA.connected],['Solo',DATA.public_solo],['Скрыто k<3',c.suppressed_items],['Скрыто B2C k<5',c.suppressed_b2c_items]];
  if(familyFilter){cards.push(['В семействе',nodes.filter(n=>n.family===familyFilter).length])}
  if((DATA.external_edges||[]).length){cards.push(['За страницей',DATA.external_edges.length])}
  if(DATA.period){cards.push(['Спрос за период',DATA.period_public_items]);cards.push(['Скрыто за период',DATA.period_hidden_items])}
  $('stats').innerHTML=cards.map(x=>'<div class="stat"><b>'+fmt(x[1])+'</b><span>'+esc(x[0])+'</span></div>').join('');
}
function drawLegend(){
  const counts={};nodes.forEach(n=>{counts[n.category]=(counts[n.category]||0)+1});
  let h='<button class="leg '+(categoryFilter===null?'active':'')+'" data-cat="all"><i style="background:#e2e8f0"></i>Все</button>';
  if((DATA.external_edges||[]).length)h+='<span class="leg"><i style="background:#fbbf24"></i>за страницей</span>';
  Object.keys(counts).sort((a,b)=>counts[b]-counts[a]).forEach(c=>{
    h+='<button class="leg '+(categoryFilter===c?'active':'')+'" data-cat="'+esc(c)+'"><i style="background:'+(COLORS[c]||COLORS.other)+'"></i>'+esc(CAT_RU[c]||c)+' · '+counts[c]+'</button>';
  });
  $('legend').innerHTML=h;
}
function showNode(n){
  selected=n||null;refresh();
  if(!n){$('panel').innerHTML='<div class="empty">Выберите товар на графе. Панель покажет покупателей, число связей и что покупают вместе с ним.</div>';return}
  const nb=edges.filter(e=>sourceID(e)===n.id||targetID(e)===n.id).map(e=>{
    const other=byId[sourceID(e)===n.id?targetID(e):sourceID(e)];
    return other?{node:other,count:e.count,b2b:e.b2b_count,b2c:e.b2c_count,hidden:e.b2c_hidden}:null;
  }).filter(Boolean).sort((a,b)=>b.count-a.count);
  let h='<div class="panel-kicker">'+esc(CAT_RU[n.category]||n.category)+(n.family?' · '+esc(n.family):'')+'</div><h2>'+esc(n.title)+'</h2>';
  h+='<div class="metrics"><div><b>'+fmt(n.buyers)+'</b><span>покупателей</span></div><div><b>'+fmt(n.degree)+'</b><span>связей</span></div><div><b>'+fmt(n.b2c_buyers)+'</b><span>B2C</span></div></div>';
  if(DATA.period){
    h+='<h3>Спрос за '+esc(DATA.period_label)+'</h3>';
    if(n.demand_hidden){h+='<div class="note">Меньше '+(audience==='b2c'?5:3)+' покупателей за период — цифра скрыта по k-порогу.</div>'}
    else{h+='<div class="metrics"><div><b>'+fmt(n.period_buyers)+'</b><span>покупателей</span></div><div><b>'+(n.growth_hidden?'—':((n.growth>=0?'+':'')+Math.round(n.growth*100)+'%'))+'</b><span>к прошлому</span></div><div><b>'+(n.growth_hidden?'—':fmt(n.prev_buyers))+'</b><span>было</span></div></div>'}
  }
  if(!n.renewal_hidden){h+='<h3>Продление</h3><div class="metrics"><div><b>'+Math.round(n.renewal_share*100)+'%</b><span>повторов</span></div><div><b>'+n.renewal_days+'</b><span>дней медиана</span></div></div>'}
  if(n.solo){h+='<div class="note">Публичных связей нет. Кандидат на достройку: co-client, семейство товаров, атрибуты или сервис.</div>'}
  else{h+='<h3>С этим покупают</h3><div>';nb.slice(0,14).forEach(x=>{h+='<button class="nb" data-id="'+esc(x.node.id)+'"><span>'+esc(x.node.title)+'</span><b>'+x.count+'</b></button>'});h+='</div>'}
  if(n.family){h+='<div class="actions" style="margin-top:10px"><button class="secondary" id="show-family">Показать семейство «'+esc(n.family)+'»</button></div>'}
  const ext=(DATA.external_edges||[]).filter(e=>e.from===n.id);
  if(ext.length){h+='<h3>За пределами страницы</h3><div>';ext.slice(0,12).forEach(e=>{h+='<button class="nb" data-ext-id="'+esc(e.to)+'" data-ext-page="'+e.target_page+'"><span>'+esc(e.title)+'</span><b>стр. '+e.target_page+'</b></button>'});h+='</div>'}
  h+='<div class="foot">Показаны только публичные связи: у товара ≥3 покупателя, у B2B-пары ≥3 совместных покупки. B2C-счётчики меньше 5 скрыты.</div>';
  $('panel').innerHTML=h;
  $('panel').querySelectorAll('.nb').forEach(b=>b.onclick=()=>{const x=byId[b.dataset.id];if(x){showNode(x);focusNode(x)}});
  const sf=$('show-family');if(sf)sf.onclick=()=>{familyFilter=n.family||'';fillFilter('family-filter',DATA.families,'Все семейства',familyFilter);drawStats();refresh()};
  $('panel').querySelectorAll('[data-ext-id]').forEach(b=>b.onclick=()=>{page=Number(b.dataset.extPage)||1;load(b.dataset.extId)});
}
function focusNode(n){
  if(!graph||!n)return;
  userInteracted=true;autoFitDone=true;
  const d=180,dist=Math.hypot(n.x||0,n.y||0,n.z||0)||1,ratio=1+d/dist;
  graph.cameraPosition({x:(n.x||0)*ratio,y:(n.y||0)*ratio,z:(n.z||0)*ratio},n,900);
}
function updateTimeControl(){
  const t=$('time');
  t.max=Math.max(0,periods.length-1);
  let idx=periods.findIndex(p=>p.value===currentPeriod);
  if(idx<0)idx=0;
  t.value=idx;
  $('period-label').textContent=periods[idx]?periods[idx].label:'Все время';
}
function selectInitial(){
  const q=new URLSearchParams(location.search).get('select');
  if(!q)return;
  let n=null;
  if(q==='top')n=nodes.filter(x=>!x.solo).sort((a,b)=>b.degree-a.degree)[0];
  else n=byId[q];
  if(n){showNode(n);setTimeout(()=>focusNode(n),700)}
}
$('zoom-in').addEventListener('click',()=>zoomBy(.8));
$('zoom-out').addEventListener('click',()=>zoomBy(1.25));
$('zoom-reset').addEventListener('click',resetCamera);
$('prev-page').addEventListener('click',()=>{if(page>1){page--;load()}});
$('next-page').addEventListener('click',()=>{if(page<totalPages){page++;load()}});
$('page-size').addEventListener('change',e=>{pageSize=Number(e.target.value)||1000;page=1;load()});
$('family-filter').addEventListener('change',e=>{familyFilter=e.target.value;drawStats();refresh();if(selected)showNode(selected)});
$('industry-filter').addEventListener('change',e=>{industryFilter=e.target.value;familyFilter='';initialPeriod='';load().catch(err=>{$('status').textContent='Ошибка: '+err.message})});
$('segment-filter').addEventListener('change',e=>{segmentFilter=e.target.value;familyFilter='';initialPeriod='';load().catch(err=>{$('status').textContent='Ошибка: '+err.message})});
$('time').addEventListener('input',e=>{const p=periods[Number(e.target.value)];if(p)$('period-label').textContent=p.label});
$('time').addEventListener('change',e=>{const p=periods[Number(e.target.value)];if(!p)return;loadPeriod(p.value).catch(err=>{$('status').textContent='Ошибка: '+err.message})});
$('search').addEventListener('input',e=>{search=e.target.value;refresh()});
$('search').addEventListener('keydown',e=>{if(e.key!=='Enter')return;const q=search.trim().toLowerCase();if(!q)return;const n=nodes.find(x=>x.title.toLowerCase().indexOf(q)>=0);if(n){showNode(n);focusNode(n)}});
$('solo').addEventListener('change',e=>{soloOn=e.target.checked;refresh()});
$('legend').addEventListener('click',e=>{const b=e.target.closest('[data-cat]');if(!b)return;categoryFilter=b.dataset.cat==='all'?null:b.dataset.cat;drawLegend();refresh()});
document.querySelectorAll('[data-audience]').forEach(b=>b.addEventListener('click',()=>{audience=b.dataset.audience;document.querySelectorAll('[data-audience]').forEach(x=>x.classList.toggle('active',x===b));industryFilter='';segmentFilter='';familyFilter='';initialPeriod='';load().catch(err=>{$('status').textContent='Ошибка: '+err.message})}));
window.addEventListener('resize',()=>{resizeGraph()});
wrap.addEventListener('pointerdown',()=>{userInteracted=true;autoFitDone=true},{passive:true});
wrap.addEventListener('wheel',()=>{userInteracted=true;autoFitDone=true},{passive:true});
initGraph();
load().catch(err=>{$('status').textContent='Ошибка: '+err.message});
</script>
</body>
</html>`
