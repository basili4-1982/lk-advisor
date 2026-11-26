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
main{display:grid;grid-template-columns:1fr 390px;height:calc(100vh - 72px)}
.graph-wrap{position:relative;min-width:0;overflow:hidden;height:100%}
canvas{display:block;position:absolute;inset:0;width:100%;height:100%;cursor:grab}
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
    <div class="seg">
      <button data-audience="b2b" class="active">B2B</button>
      <button data-audience="b2c">B2C</button>
    </div>
    <div class="timebox"><input id="time" type="range" min="0" max="0" value="0"><span id="period-label">Все время</span></div>
    <input id="search" type="search" placeholder="Найти товар…">
    <label class="toggle"><input id="solo" type="checkbox" checked> solo</label>
    <a class="link" href="/">Кабинет</a>
    <span class="status" id="status"></span>
  </div>
</header>
<main>
  <section class="graph-wrap" id="graph-wrap">
    <div class="stats" id="stats"></div>
    <canvas id="graph"></canvas>
    <div class="legend" id="legend"></div>
    <div class="hint">Клик по узлу — карточка и связи. Колесо — зум, перетаскивание — панорама.</div>
  </section>
  <aside class="panel" id="panel"></aside>
</main>
<div id="tooltip"></div>
<script>
const $=id=>document.getElementById(id);
const canvas=$('graph'), ctx=canvas.getContext('2d'), wrap=$('graph-wrap');
const W=1000,H=760,CENTER={x:W/2,y:H/2};
const COLORS={security:'#f87171',office:'#60a5fa',graphics:'#c084fc',backup:'#4ade80',devtools:'#fbbf24',training:'#2dd4bf',other:'#94a3b8'};
const CAT_RU={security:'Безопасность',office:'Офис',graphics:'Графика',backup:'Бэкап',devtools:'Разработка',training:'Обучение',other:'Другое'};
let DATA=null,nodes=[],edges=[],byId={},selected=null,hover=null;
let audience='b2b',soloOn=true,categoryFilter=null,search='';
let periods=[],currentPeriod='',initialPeriod=new URLSearchParams(location.search).get('period')||'';
let transform={x:0,y:0,k:1};
let seed=1234567;
function rnd(){seed=(seed*1664525+1013904223)>>>0;return seed/4294967296}
function esc(s){return String(s==null?'':s).replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]))}
function fmt(n){return Number(n||0).toLocaleString('ru-RU')}
function viewport(){return {w:wrap.clientWidth||900,h:wrap.clientHeight||700}}
function resize(){const vp=viewport(),dpr=window.devicePixelRatio||1;canvas.width=vp.w*dpr;canvas.height=vp.h*dpr;canvas.style.width=vp.w+'px';canvas.style.height=vp.h+'px'}
async function load(){
  $('status').textContent='Загружаю…';
  const r=await fetch('/v1/demo/graph?audience='+audience+'&period='+encodeURIComponent(initialPeriod)+'&solo_limit=1000');
  if(!r.ok) throw new Error('HTTP '+r.status);
  DATA=await r.json();
  periods=DATA.periods||[]; currentPeriod=DATA.period||'';
  build(); drawStats(); drawLegend(); render(); updateTimeControl(); selectInitial();
  $('status').textContent='';
}
function selectInitial(){
  const q=new URLSearchParams(location.search).get('select');
  if(!q)return;
  if(q==='top'){
    const n=nodes.filter(x=>!x.solo).sort((a,b)=>b.degree-a.degree)[0];
    if(n){showNode(n);centerOn(n)}
    return;
  }
  const n=byId[q];if(n){showNode(n);centerOn(n)}
}
function build(){
  nodes=DATA.nodes.map(n=>Object.assign({},n,{color:COLORS[n.category]||COLORS.other,r:n.solo?3:4+Math.min(9,Math.sqrt(Math.max(1,n.degree))*2.4)}));
  byId={}; nodes.forEach(n=>byId[n.id]=n);
  edges=DATA.edges.map(e=>Object.assign({},e,{a:byId[e.source],b:byId[e.target]})).filter(e=>e.a&&e.b);
  layout();
  showNode(null);
}
function layout(){
  seed=1234567;
  const counts={}; nodes.filter(n=>!n.solo).forEach(n=>{counts[n.category]=(counts[n.category]||0)+1});
  const cats=Object.keys(counts).sort((a,b)=>counts[b]-counts[a]);
  const centers={};
  cats.forEach((c,i)=>{const a=(i/Math.max(1,cats.length))*Math.PI*2-Math.PI/2;centers[c]={x:CENTER.x+Math.cos(a)*225,y:CENTER.y+Math.sin(a)*175}});
  const connected=nodes.filter(n=>!n.solo);
  connected.forEach(n=>{const c=centers[n.category]||CENTER;n.x=c.x+(rnd()-.5)*110;n.y=c.y+(rnd()-.5)*110;n.vx=0;n.vy=0});
  const solo=nodes.filter(n=>n.solo);
  solo.forEach((n,i)=>{const a=(i/Math.max(1,solo.length))*Math.PI*2;const rad=360+(i%7)*8;n.x=CENTER.x+Math.cos(a)*rad;n.y=CENTER.y+Math.sin(a)*rad*.72;n.vx=0;n.vy=0});
  for(let step=0;step<240;step++){
    const alpha=1-step/260;
    for(const n of connected){
      for(let s=0;s<26;s++){
        const o=connected[(rnd()*connected.length)|0];if(o===n)continue;
        const dx=n.x-o.x,dy=n.y-o.y,d2=dx*dx+dy*dy+.01;
        if(d2<40000){const d=Math.sqrt(d2),f=alpha*950/d2;n.vx+=dx/d*f;n.vy+=dy/d*f}
      }
      const c=centers[n.category]||CENTER;
      n.vx+=(c.x-n.x)*.0018*alpha;n.vy+=(c.y-n.y)*.0018*alpha;
    }
    for(const e of edges){
      const a=e.a,b=e.b;if(a.solo||b.solo)continue;
      const dx=b.x-a.x,dy=b.y-a.y,d=Math.sqrt(dx*dx+dy*dy)+.01;
      const desired=70+Math.min(80,Math.log(e.count+1)*18),f=(d-desired)*.0035*alpha;
      a.vx+=dx/d*f;a.vy+=dy/d*f;b.vx-=dx/d*f;b.vy-=dy/d*f;
    }
    for(const n of connected){n.x+=n.vx;n.y+=n.vy;n.vx*=.82;n.vy*=.82;n.x=Math.max(70,Math.min(W-70,n.x));n.y=Math.max(60,Math.min(H-60,n.y))}
  }
  fitView();
}
function fitView(){
  if(!nodes.length)return;
  const vp=viewport();let minX=1e9,minY=1e9,maxX=-1e9,maxY=-1e9;
  for(const n of nodes){minX=Math.min(minX,n.x);minY=Math.min(minY,n.y);maxX=Math.max(maxX,n.x);maxY=Math.max(maxY,n.y)}
  const bw=Math.max(100,maxX-minX),bh=Math.max(100,maxY-minY);
  const k=Math.min((vp.w-120)/bw,(vp.h-120)/bh,1.5);
  transform.k=Math.max(.15,k);
  transform.x=(vp.w-bw*transform.k)/2-minX*transform.k;
  transform.y=(vp.h-bh*transform.k)/2-minY*transform.k;
}
function visible(n){return (!n.solo||soloOn)&&(!categoryFilter||n.category===categoryFilter)}
function nodeRadius(n){
  if(DATA&&DATA.period){
    if(n.demand_hidden)return 2.6;
    return 3+Math.min(11,Math.sqrt(Math.max(1,n.period_buyers))*2.2);
  }
  return n.solo?3:4+Math.min(9,Math.sqrt(Math.max(1,n.degree))*2.4);
}
function updateTimeControl(){
  const t=$('time');
  t.max=Math.max(0,periods.length-1);
  let idx=periods.findIndex(p=>p.value===currentPeriod);
  if(idx<0)idx=0;
  t.value=idx;
  $('period-label').textContent=periods[idx]?periods[idx].label:'Все время';
}
function render(){
  const vp=viewport(),dpr=window.devicePixelRatio||1;
  ctx.setTransform(dpr,0,0,dpr,0,0);
  ctx.clearRect(0,0,vp.w,vp.h);
  ctx.save();ctx.translate(transform.x,transform.y);ctx.scale(transform.k,transform.k);
  const q=search.trim().toLowerCase();
  for(const e of edges){
    if(!visible(e.a)||!visible(e.b))continue;
    const hot=selected===e.a||selected===e.b||hover===e.a||hover===e.b;
    ctx.strokeStyle=hot?'rgba(96,165,250,.58)':'rgba(148,163,184,.16)';
    ctx.lineWidth=hot?1.8:Math.min(2.4,.5+Math.log(e.count+1)*.25);
    ctx.beginPath();ctx.moveTo(e.a.x,e.a.y);ctx.lineTo(e.b.x,e.b.y);ctx.stroke();
  }
  for(const n of nodes){
    if(!visible(n))continue;
    const match=q&&n.title.toLowerCase().indexOf(q)>=0;
    const hidePeriod=DATA&&DATA.period&&n.demand_hidden;
    const dim=(q&&!match)||(categoryFilter&&n.category!==categoryFilter)||hidePeriod;
    const r=nodeRadius(n);
    ctx.globalAlpha=dim?.22:1;
    if(DATA&&DATA.period&&!n.demand_hidden&&!n.growth_hidden&&Math.abs(n.growth)>=.15){
      ctx.beginPath();ctx.arc(n.x,n.y,r+3.5,0,Math.PI*2);
      ctx.strokeStyle=n.growth>0?'rgba(74,222,128,.8)':'rgba(248,113,113,.8)';ctx.lineWidth=1.6;ctx.stroke();
    }
    if(selected===n||hover===n){ctx.beginPath();ctx.arc(n.x,n.y,r+5,0,Math.PI*2);ctx.strokeStyle=n.color;ctx.lineWidth=2;ctx.stroke()}
    ctx.beginPath();ctx.arc(n.x,n.y,r,0,Math.PI*2);
    if(n.solo){ctx.fillStyle='rgba(71,85,105,.62)';ctx.fill();ctx.strokeStyle='rgba(148,163,184,.55)';ctx.lineWidth=1;ctx.stroke()}
    else{ctx.fillStyle=n.color;ctx.fill()}
    if(selected===n||hover===n||match){
      ctx.globalAlpha=dim?.25:.95;
      ctx.fillStyle='#e2e8f0';ctx.font=(selected===n?'600 ':'500 ')+(11/transform.k)+'px Inter, system-ui, sans-serif';
      const label=n.title.length>36?n.title.slice(0,35)+'…':n.title;
      ctx.fillText(label,n.x+r+4,n.y+3);
    }
    ctx.globalAlpha=1;
  }
  ctx.restore();
}
function drawStats(){
  const c=DATA.coverage;
  const cards=[['Публичные товары',c.public_items],['Публичные связи',c.public_pairs],['В связях',DATA.connected],['Solo',DATA.public_solo],['Скрыто k<3',c.suppressed_items],['Скрыто B2C k<5',c.suppressed_b2c_items]];
  if(DATA.period){cards.push(['Спрос за период',DATA.period_public_items]);cards.push(['Скрыто за период',DATA.period_hidden_items])}
  $('stats').innerHTML=cards.map(x=>'<div class="stat"><b>'+fmt(x[1])+'</b><span>'+esc(x[0])+'</span></div>').join('');
}
function drawLegend(){
  const counts={};nodes.forEach(n=>{counts[n.category]=(counts[n.category]||0)+1});
  let h='<button class="leg '+(categoryFilter===null?'active':'')+'" data-cat="all"><i style="background:#e2e8f0"></i>Все</button>';
  Object.keys(counts).sort((a,b)=>counts[b]-counts[a]).forEach(c=>{
    h+='<button class="leg '+(categoryFilter===c?'active':'')+'" data-cat="'+esc(c)+'"><i style="background:'+(COLORS[c]||COLORS.other)+'"></i>'+esc(CAT_RU[c]||c)+' · '+counts[c]+'</button>';
  });
  $('legend').innerHTML=h;
}
function showNode(n){
  selected=n||null;render();
  if(!n){$('panel').innerHTML='<div class="empty">Выберите товар на графе. Панель покажет покупателей, число связей и что покупают вместе с ним.</div>';return}
  const nb=edges.filter(e=>e.a===n||e.b===n).map(e=>({node:e.a===n?e.b:e.a,count:e.count,b2b:e.b2b_count,b2c:e.b2c_count,hidden:e.b2c_hidden})).sort((a,b)=>b.count-a.count);
  let h='<div class="panel-kicker">'+esc(CAT_RU[n.category]||n.category)+'</div><h2>'+esc(n.title)+'</h2>';
  h+='<div class="metrics"><div><b>'+fmt(n.buyers)+'</b><span>покупателей</span></div><div><b>'+fmt(n.degree)+'</b><span>связей</span></div><div><b>'+fmt(n.b2c_buyers)+'</b><span>B2C</span></div></div>';
  if(DATA.period){
    h+='<h3>Спрос за '+esc(DATA.period_label)+'</h3>';
    if(n.demand_hidden){h+='<div class="note">Меньше '+(audience==='b2c'?5:3)+' покупателей за период — цифра скрыта по k-порогу.</div>'}
    else{h+='<div class="metrics"><div><b>'+fmt(n.period_buyers)+'</b><span>покупателей</span></div><div><b>'+(n.growth_hidden?'—':((n.growth>=0?'+':'')+Math.round(n.growth*100)+'%'))+'</b><span>к прошлому</span></div><div><b>'+(n.growth_hidden?'—':fmt(n.prev_buyers))+'</b><span>было</span></div></div>'}
  }
  if(!n.renewal_hidden){h+='<h3>Продление</h3><div class="metrics"><div><b>'+Math.round(n.renewal_share*100)+'%</b><span>повторов</span></div><div><b>'+n.renewal_days+'</b><span>дней медиана</span></div></div>'}
  if(n.solo){h+='<div class="note">Публичных связей нет. Кандидат на достройку: co-client, семейство товаров, атрибуты или сервис.</div>'}
  else{h+='<h3>С этим покупают</h3><div>';nb.slice(0,14).forEach(x=>{h+='<button class="nb" data-id="'+esc(x.node.id)+'"><span>'+esc(x.node.title)+'</span><b>'+x.count+'</b></button>'});h+='</div>'}
  h+='<div class="foot">Показаны только публичные связи: у товара ≥3 покупателя, у B2B-пары ≥3 совместных покупки. B2C-счётчики меньше 5 скрыты.</div>';
  $('panel').innerHTML=h;
  $('panel').querySelectorAll('.nb').forEach(b=>b.onclick=()=>{const x=byId[b.dataset.id];if(x){showNode(x);centerOn(x)}});
}
function centerOn(n){const vp=viewport();transform.k=Math.max(1.1,transform.k);transform.x=vp.w/2-n.x*transform.k;transform.y=vp.h/2-n.y*transform.k;render()}
function toWorld(e){const r=canvas.getBoundingClientRect();return {x:(e.clientX-r.left-transform.x)/transform.k,y:(e.clientY-r.top-transform.y)/transform.k}}
function pick(x,y){
  let best=null,bd=1e9;
  for(const n of nodes){
    if(!visible(n))continue;
    const d=Math.hypot(n.x-x,n.y-y),hit=Math.max(nodeRadius(n)+6,10/transform.k);
    if(d<hit&&d<bd){bd=d;best=n}
  }
  return best;
}
function tooltip(e,n){
  const t=$('tooltip');
  if(!n){t.style.display='none';return}
  t.style.display='block';t.textContent=n.title;t.style.left=(e.clientX+14)+'px';t.style.top=(e.clientY+14)+'px';
}
let drag=null,moved=false;
canvas.addEventListener('mousedown',e=>{drag={x:e.clientX,y:e.clientY,tx:transform.x,ty:transform.y};moved=false;canvas.style.cursor='grabbing'});
window.addEventListener('mousemove',e=>{
  if(drag){const dx=e.clientX-drag.x,dy=e.clientY-drag.y;if(Math.abs(dx)+Math.abs(dy)>3)moved=true;transform.x=drag.tx+dx;transform.y=drag.ty+dy;render();return}
  const p=toWorld(e),n=pick(p.x,p.y);hover=n;canvas.style.cursor=n?'pointer':'grab';tooltip(e,n);render();
});
window.addEventListener('mouseup',e=>{
  if(!drag)return;
  const wasMoved=moved;drag=null;canvas.style.cursor='grab';
  if(!wasMoved){const p=toWorld(e),n=pick(p.x,p.y);showNode(n)}
});
canvas.addEventListener('wheel',e=>{
  e.preventDefault();
  const r=canvas.getBoundingClientRect(),mx=e.clientX-r.left,my=e.clientY-r.top;
  const factor=e.deltaY<0?1.12:.89,k2=Math.max(.15,Math.min(4,transform.k*factor));
  transform.x=mx-(mx-transform.x)*(k2/transform.k);transform.y=my-(my-transform.y)*(k2/transform.k);transform.k=k2;render();
},{passive:false});
function applyPeriod(d){
  const map={};d.nodes.forEach(n=>{map[n.id]=n});
  nodes.forEach(n=>{const x=map[n.id];if(!x)return;n.period_buyers=x.period_buyers;n.prev_buyers=x.prev_buyers;n.growth=x.growth;n.demand_hidden=x.demand_hidden;n.growth_hidden=x.growth_hidden;n.renewal_share=x.renewal_share;n.renewal_days=x.renewal_days;n.renewal_hidden=x.renewal_hidden});
  DATA.period=d.period;DATA.period_label=d.period_label;DATA.period_items=d.period_items;DATA.period_public_items=d.period_public_items;DATA.period_hidden_items=d.period_hidden_items;
  currentPeriod=d.period;initialPeriod=d.period;updateTimeControl();drawStats();render();if(selected)showNode(selected);
}
async function loadPeriod(value){
  $('status').textContent='Считаю…';
  const r=await fetch('/v1/demo/graph?audience='+audience+'&period='+encodeURIComponent(value)+'&solo_limit=1000');
  if(!r.ok)throw new Error('HTTP '+r.status);
  applyPeriod(await r.json());
  $('status').textContent='';
}
$('time').addEventListener('input',e=>{
  const p=periods[Number(e.target.value)];if(p)$('period-label').textContent=p.label;
});
$('time').addEventListener('change',e=>{
  const p=periods[Number(e.target.value)];if(!p)return;
  loadPeriod(p.value).catch(err=>{$('status').textContent='Ошибка: '+err.message});
});
$('search').addEventListener('input',e=>{search=e.target.value;render()});
$('search').addEventListener('keydown',e=>{
  if(e.key!=='Enter')return;
  const q=search.trim().toLowerCase();if(!q)return;
  const n=nodes.find(x=>x.title.toLowerCase().indexOf(q)>=0);
  if(n){showNode(n);centerOn(n)}
});
$('solo').addEventListener('change',e=>{soloOn=e.target.checked;render()});
$('legend').addEventListener('click',e=>{
  const b=e.target.closest('[data-cat]');if(!b)return;
  categoryFilter=b.dataset.cat==='all'?null:b.dataset.cat;drawLegend();render();
});
document.querySelectorAll('[data-audience]').forEach(b=>b.addEventListener('click',()=>{
  audience=b.dataset.audience;document.querySelectorAll('[data-audience]').forEach(x=>x.classList.toggle('active',x===b));
  selected=null;categoryFilter=null;load().catch(err=>{$('status').textContent='Ошибка: '+err.message});
}));
window.addEventListener('resize',()=>{resize();render()});
resize();
load().catch(err=>{$('status').textContent='Ошибка: '+err.message});
</script>
</body>
</html>`
