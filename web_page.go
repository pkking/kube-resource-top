package main

// webPage is the dashboard markup, inline CSS and script with no external
// dependencies: internal cluster networks are frequently air-gapped, so the
// page ships inside the binary. The two tabs map to the two user types —
// capacity readers (资源总览) and task submitters (排队进度). The style is
// a machine-room LED inventory board: LED colors carry state semantics, one
// gold accent, tabular monospace figures, and per-resource chip strips as
// the hero. Data has no backticks so it can live in a raw string.
const webPage = `<!doctype html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<link rel="icon" href="data:,">
<title>kube-resource-top · 算力看板</title>
<style>
:root{--bg:#0b1017;--panel:#0f1520;--line:#1e2b3c;--ink:#d9e3ee;--dim:#8290a3;--gold:#d2a44e;--free:#39c97e;--busy:#e2a23d;--full:#e0564f;--mono:ui-monospace,SFMono-Regular,Menlo,Consolas,'Liberation Mono',monospace}
*{box-sizing:border-box}
body{margin:0;background:var(--bg);color:var(--ink);font:14px/1.65 "PingFang SC","Hiragino Sans GB","Microsoft YaHei","Noto Sans CJK SC",sans-serif}
.mono{font-family:var(--mono)}
.wrap{max-width:1200px;margin:0 auto;padding:22px 26px 64px}
header{display:flex;flex-wrap:wrap;justify-content:space-between;align-items:flex-end;gap:8px 16px;border-bottom:1px solid var(--line);padding-bottom:14px}
.brand{display:flex;align-items:baseline;gap:12px}
.brand .tool{font-family:var(--mono);color:var(--gold);font-size:13px}
.brand h1{margin:0;font-size:22px;font-weight:600;letter-spacing:.05em}
.live{display:flex;align-items:center;gap:10px;color:var(--dim);font-size:12.5px;font-family:var(--mono)}
.dot{width:8px;height:8px;border-radius:50%;background:var(--free);animation:pulse 2.4s ease-in-out infinite}
.dot.bad{background:var(--full)}
@keyframes pulse{50%{opacity:.35}}
.tabs{display:flex;gap:26px;border-bottom:1px solid var(--line)}
.tabs button{appearance:none;background:none;border:0;border-bottom:2px solid transparent;color:var(--dim);font:inherit;font-size:15px;padding:12px 2px;cursor:pointer}
.tabs button[aria-selected="true"]{color:var(--ink);border-bottom-color:var(--gold)}
.tabs button:focus-visible{outline:2px solid var(--gold);outline-offset:-2px}
main{min-height:180px}
.noanim .seg{transition:none}
.connecting{color:var(--dim);padding:46px 0;text-align:center}
.leg-note{color:var(--dim)}
.legend{display:flex;flex-wrap:wrap;gap:6px 22px;color:var(--dim);font-size:12.5px;margin:14px 0 4px}
.sw{display:inline-block;width:11px;height:11px;margin-right:6px;vertical-align:-1px;border-radius:2px}
.sw.b{background:var(--busy)}
.sw.q{background:repeating-linear-gradient(45deg,rgba(224,86,79,.6) 0 2px,transparent 2px 5px)}
.sw.pc{background:#4cb8d9}
.sw.f{border:1px solid #3a4c63}
.res{border-bottom:1px solid var(--line);padding:16px 0 12px}
.res-head{display:grid;grid-template-columns:240px 1fr 122px;gap:0 24px}
.res-name{grid-row:1/4;font-family:var(--mono);font-size:15px;color:var(--gold);overflow-wrap:anywhere}
.res-name .kind{color:var(--dim);font-size:12px}
.strip{position:relative;height:22px}
.strip .cells{position:absolute;inset:0;display:flex;gap:2px}
.strip .cell{flex:1;border:1px solid #223043;border-radius:2px}
.seg{position:absolute;top:0;bottom:0;transition:width .6s ease;border-right:1px solid rgba(11,16,23,.9)}
.strip.nores .cell{border-style:dashed}
.res-pie{grid-column:3;grid-row:1/3;align-self:center;justify-self:center}
.donut{width:88px;height:88px;border-radius:50%;position:relative}
.donut .hole{position:absolute;inset:17px;border-radius:50%;background:var(--bg);display:flex;flex-direction:column;align-items:center;justify-content:center;line-height:1.2}
.donut .hole b{font-family:var(--mono);font-size:14px;color:var(--ink)}
.donut .hole i{font-style:normal;font-size:11px;color:var(--dim)}
.pdot{display:inline-block;width:9px;height:9px;border-radius:2px;margin-right:8px;vertical-align:-1px}
.pdot.un{background:var(--busy)}
.nums{color:var(--dim);font-size:13px;display:flex;flex-wrap:wrap;gap:2px 20px}
.nums .n i{font-style:normal;margin-right:6px}
.nums .n b{font-family:var(--mono);font-weight:600;color:var(--ink);font-variant-numeric:tabular-nums}
.nums .n b.warn{color:var(--busy)}
.nums .n b.hot{color:var(--full)}
.nums .n b.ok{color:var(--free)}
details.clusters{margin-top:10px}
details.clusters>summary{cursor:pointer;color:var(--dim);font-size:13px;width:max-content}
details.clusters>summary:hover{color:var(--gold)}
details.clusters>summary:focus-visible{outline:2px solid var(--gold)}
details.nodes>summary{cursor:pointer;color:var(--dim);font-size:12.5px;width:max-content;margin:4px 0}
details.nodes>summary:hover{color:var(--gold)}
details.nodes>summary:focus-visible{outline:2px solid var(--gold)}
.scroll{overflow-x:auto}
tr.sub>td{border-bottom:none;padding:0 10px 10px}
tr.sub table{margin:0}
table{border-collapse:collapse;width:100%;font-size:13px}
th{color:var(--dim);font-weight:500;text-align:right;padding:4px 10px;border-bottom:1px solid var(--line);white-space:nowrap}
td{padding:4px 10px;text-align:right;border-bottom:1px solid #16202d;font-family:var(--mono);white-space:nowrap;font-variant-numeric:tabular-nums}
th:first-child,td:first-child{text-align:left}
td.dim{color:var(--dim)}
td.ok{color:var(--free)}
td.hot{color:var(--full)}
td.v{text-align:left}
td.none{text-align:center;color:var(--dim)}
.v.full{color:var(--full)}
.v.fragmented{color:var(--busy)}
.v.blocked{color:var(--gold)}
.v.absent{color:var(--dim)}
.v.empty{color:var(--free)}
.chips{display:flex;flex-wrap:wrap;gap:8px;margin:18px 0 4px}
.chip{display:inline-flex;align-items:center;gap:8px;background:var(--panel);border:1px solid var(--line);border-radius:3px;color:var(--ink);font:inherit;font-size:13px;padding:6px 12px;cursor:pointer}
.chip:focus-visible{outline:2px solid var(--gold)}
.chip.on{border-color:var(--gold)}
.chip .st{width:8px;height:8px;border-radius:50%;background:var(--free)}
.chip .st.warn{background:var(--busy)}
.chip .st.full{background:var(--full)}
.chip b{font-family:var(--mono);font-weight:600;color:var(--dim)}
.chip.on b{color:var(--ink)}
.searchrow{display:flex;flex-wrap:wrap;gap:12px;align-items:center;margin:12px 0 6px}
#qsearch{background:var(--panel);border:1px solid var(--line);border-radius:3px;color:var(--ink);font:inherit;padding:7px 12px;width:320px;max-width:100%}
#qsearch:focus{outline:none;border-color:var(--gold)}
.mline{color:var(--dim);font-size:13px}
.qsec{border-top:1px solid var(--line);margin-top:14px;padding-top:4px}
.qsec h2{font-size:15px;font-weight:600;margin:10px 0 8px}
.qsec h2 .abs{color:var(--dim);font-size:12.5px;font-weight:400;margin-left:10px}
.verdict{border-left:3px solid var(--dim);background:var(--panel);padding:8px 14px;margin:8px 0;font-size:13.5px;color:var(--ink)}
.verdict b{margin-right:8px}
.verdict.full{border-color:var(--full)}.verdict.full b{color:var(--full)}
.verdict.fragmented{border-color:var(--busy)}.verdict.fragmented b{color:var(--busy)}
.verdict.blocked{border-color:var(--gold)}.verdict.blocked b{color:var(--gold)}
.verdict.absent{border-color:var(--dim)}.verdict.absent b{color:var(--dim)}
.verdict.empty{border-color:var(--free)}.verdict.empty b{color:var(--free)}
.qt td:nth-child(3),.qt td:nth-child(4),.qt td:nth-child(5),.qt th:nth-child(3),.qt th:nth-child(4),.qt th:nth-child(5){text-align:left}
tr.mine td{background:rgba(210,164,78,.13)}
.qinfo{color:var(--dim);font-size:12.5px;margin:6px 2px 0}
footer{margin-top:36px;color:var(--dim);font-size:12.5px;border-top:1px solid var(--line);padding-top:12px}
footer p{margin:4px 0}
footer .err{color:var(--full)}
@media(max-width:720px){
  .res-head{grid-template-columns:1fr}
  .res-name{grid-row:auto;margin-bottom:2px}
  .res-pie{grid-column:auto;grid-row:auto;justify-self:start;margin-top:6px}
  .wrap{padding:16px 14px 48px}
}
@media(prefers-reduced-motion:reduce){
  .strip .seg{transition:none}
  .dot{animation:none}
}
</style>
</head>
<body>
<div class="wrap">
<header>
  <div class="brand"><span class="tool">kube-resource-top</span><h1>算力看板</h1></div>
  <div class="live"><span class="dot" id="dot"></span><span id="liveline">连接中</span><span id="clock"></span></div>
</header>
<nav class="tabs">
  <button id="tab-capacity" role="tab" aria-selected="true">资源总览</button>
  <button id="tab-queue" role="tab" aria-selected="false">排队进度</button>
</nav>
<main id="main"><div class="connecting">正在连接集群，等待第一批快照…</div></main>
<footer id="foot"></footer>
</div>
<script>
var STATE=null,FETCH_AT=0,LAST_SIG=null,RENDERED=false,TAB='capacity',QRES=null,QUERY='',MINE=[],OPEN={};
function $(id){return document.getElementById(id)}
function esc(s){return String(s).replace(/[&<>"']/g,function(c){return {'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]})}
function now(){return STATE?STATE.nowMs+(Date.now()-FETCH_AT):Date.now()}
function fmtN(v){if(!isFinite(v))return '—';var r=Math.round(v*10)/10;return r.toLocaleString('zh-CN')}
function fmtRatio(v){if(!(v>=0))return '—';return Math.round(v*100)+'%'}
function ratioCls(v){return v>=1?'hot':(v>=0.8?'warn':'')}
// Project palette (ADR 0037): hues deliberately distinct from the reserved
// state colors (amber/red/green). A project keeps its color across views.
var PALETTE=['#4cb8d9','#5b8ee6','#9a7fe0','#c77bd9','#d97bb8','#a3c94d','#6fd0c0','#c98f6a'];
function pColor(name){var h=0;for(var i=0;i<name.length;i++){h=(h*31+name.charCodeAt(i))>>>0}return PALETTE[h%PALETTE.length]}
function hexA(hex,a){return 'rgba('+parseInt(hex.slice(1,3),16)+','+parseInt(hex.slice(3,5),16)+','+parseInt(hex.slice(5,7),16)+','+a+')'}
function projCell(name){
  if(!name)return '<span class="pdot un"></span>未标记';
  return '<span class="pdot" style="background:'+pColor(name)+'"></span>'+esc(name);
}
function fmtAge(ms){if(!(ms>=0))ms=0;var m=Math.floor(ms/60000);if(m<1)return '刚刚';var mi=m%60,h=Math.floor(m/60)%24,d=Math.floor(m/1440);if(d>0)return d+'天'+h+'时';if(h>0)return h+'时'+(mi?mi+'分':'');return m+'分'}
function vWord(v){return {full:'卡已满',fragmented:'有空闲，但放不下队首',blocked:'有空闲，但未调度',absent:'集群没有这种资源',empty:'无排队'}[v]||v}
function vState(v){return {full:'满',fragmented:'碎片化',blocked:'未调度',absent:'无此资源',empty:'—'}[v]||v}
function cname(id){for(var i=0;i<STATE.clusters.length;i++){if(STATE.clusters[i].id===id)return STATE.clusters[i].name}return id}
function chipState(r){
  var queued=0,hard=0;
  r.clusters.forEach(function(c){if(c.queue.length){queued++;if(c.verdict==='full'||c.verdict==='absent')hard++}});
  if(!queued)return '';
  return hard===queued?'full':'warn';
}
function emptyMsg(){
  if(STATE.total===0)return '<div class="connecting">没有选中的集群，请在终端运行 kube-resource-top 选择上下文。</div>';
  if(STATE.live===0)return '<div class="connecting">正在连接集群，等待第一批快照…</div>';
  return '<div class="connecting">暂无资源数据：集群还没有上报可分配资源。</div>';
}
function htmlCapacity(){
  if(STATE.resources.length===0)return emptyMsg();
  var h='<div class="legend"><span><i class="sw b"></i>运行中·未标记</span><span><i class="sw q"></i>排队·未标记</span><span><i class="sw pc"></i>项目色（运行中实心、排队同色斜纹）</span><span><i class="sw f"></i>空闲可分配</span><span class="leg-note">斜纹铺满右缘 = 排队需求已填满全部可分配</span></div>';
  STATE.resources.forEach(function(res,ri){
    var alloc=res.allocatable,run=res.running,pend=res.pending;
    var fill=alloc>0?Math.min(100,run/alloc*100):(run>0?100:0);
    var demand=run+pend;
    var demandEnd=alloc>0?Math.min(100,demand/alloc*100):(pend>0?100:0);
    var qW=Math.max(0,demandEnd-fill);
    var over=alloc>0&&demand>alloc?demand-alloc:0;
    var cells='';for(var c=0;c<24;c++)cells+='<i class="cell"></i>';
    var kind=res.unit!=='张'?'<span class="kind">（'+esc(res.unit)+'）</span>':'';
    // Running and queued regions subdivide by project (ADR 0037); the
    // unattributed remainder keeps the neutral amber / red hatch.
    var segs='',cum=0,qcum=0;
    (res.projects||[]).forEach(function(p){
      if(run>0&&p.running>0){
        var w=p.running/run*fill;
        segs+='<div class="seg" data-w="'+w.toFixed(3)+'" style="width:0;left:'+cum.toFixed(3)+'%;background:'+(p.name?pColor(p.name):'#e2a23d')+'"></div>';
        cum+=w;
      }
      if(qW>0&&p.pending>0){
        var qw=p.pending/pend*qW;
        var qc=p.name?pColor(p.name):'#e0564f';
        segs+='<div class="seg" data-w="'+qw.toFixed(3)+'" style="width:0;left:'+(fill+qcum).toFixed(3)+'%;background:repeating-linear-gradient(45deg,'+hexA(qc,.6)+' 0 4px,transparent 4px 9px)"></div>';
        qcum+=qw;
      }
    });
    // Donut: the running share by project, matching the 占用 number.
    var plist=(res.projects||[]).filter(function(p){return p.running>0});
    var pie='';
    if(run>0&&plist.length){
      var stops=[],pos=0;
      plist.forEach(function(p){
        var share=p.running/run,color=p.name?pColor(p.name):'#e2a23d';
        stops.push(color+' '+(pos*100).toFixed(2)+'% '+((pos+share)*100).toFixed(2)+'%');
        pos+=share;
      });
      var dtitle='项目占用（运行中）：'+plist.map(function(p){return (p.name||'未标记')+' '+fmtN(p.running)+' '+esc(res.unit)+'（'+fmtRatio(p.running/run)+'）'}).join('；');
      pie='<div class="res-pie"><div class="donut" style="background:conic-gradient('+stops.join(',')+')" title="'+esc(dtitle)+'"><div class="hole"><b>'+fmtRatio(res.runRatio)+'</b><i>占用</i></div></div></div>';
    }
    h+='<section class="res"><div class="res-head">'
      +'<div class="res-name" title="'+esc(res.name)+'">'+esc(res.name)+kind+'</div>'
      +'<div class="strip'+(alloc<=0?' nores':'')+'" title="实心段 = 运行中 '+fmtN(run)+'（占用 '+fmtRatio(res.runRatio)+'）；斜纹段 = 排队 '+fmtN(pend)+' '+esc(res.unit)+'；斜纹铺满右缘即需求超过可分配">'
      +segs
      +'<div class="cells">'+cells+'</div></div>'
      +pie
      +'<div class="nums">'
      +'<span class="n"><i>总量</i><b>'+fmtN(res.total)+'</b></span>'
      +'<span class="n" title="占用 = 运行中 ÷ 可分配（与上方填充条一致）"><i>占用</i><b class="'+ratioCls(res.runRatio)+'">'+fmtRatio(res.runRatio)+'</b></span>'
      +'<span class="n" title="需求 =（运行中 + 排队）÷ 可分配，规划口径（ADR 0028），与红色需求线一致"><i>需求</i><b class="'+ratioCls(res.ratio)+'">'+fmtRatio(res.ratio)+'</b></span>'
      +'<span class="n"><i>运行中</i><b>'+fmtN(run)+'</b></span>'
      +'<span class="n"><i>排队</i><b class="'+(pend>0?'hot':'')+'">'+fmtN(pend)+'</b></span>'
      +'<span class="n"><i>可分配</i><b>'+fmtN(alloc)+'</b></span>'
      +(over>0?'<span class="n" title="需求超出可分配的量 =（运行中+排队）− 可分配"><i>超配</i><b class="hot">+'+fmtN(over)+'</b></span>':'')
      +'</div></div>'
      +'<details class="clusters" id="dp'+ri+'"'+(OPEN['dp'+ri]?' open':'')+'><summary>项目明细 · '+(res.projects||[]).length+'</summary><div class="scroll"><table>'
      +'<thead><tr><th>项目</th><th>运行中</th><th>排队</th><th>需求</th><th>需求占比</th></tr></thead><tbody>'
      +(res.projects||[]).map(function(p){
        return '<tr><td>'+projCell(p.name)+'</td><td>'+fmtN(p.running)+'</td><td class="'+(p.pending>0?'hot':'')+'">'+fmtN(p.pending)+'</td><td>'+fmtN(p.running+p.pending)+'</td><td>'+fmtRatio(demand>0?(p.running+p.pending)/demand:0)+'</td></tr>';
      }).join('')
      +'</tbody></table></div></details>'
      +'<details class="clusters" id="dc'+ri+'"'+(OPEN['dc'+ri]?' open':'')+'><summary>集群明细 · '+res.clusters.length+'</summary><div class="scroll"><table>'
      +'<thead><tr><th>集群</th><th>总量</th><th>运行中</th><th>排队</th><th>可分配</th><th>空闲</th><th>空闲机器</th><th>单机最大空闲</th><th>状态</th></tr></thead><tbody>';
    res.clusters.forEach(function(c,ci){
      h+='<tr><td>'+esc(cname(c.cluster))+'</td><td>'+fmtN(c.total)+'</td><td>'+fmtN(c.running)+'</td><td class="'+(c.pending>0?'hot':'')+'">'+fmtN(c.pending)+'</td><td>'+fmtN(c.allocatable)+'</td><td>'+fmtN(c.free)+'</td><td>'+fmtN(c.freeNodes)+'</td><td>'+fmtN(c.maxNodeFree)+'</td><td class="v '+c.verdict+'">'+vState(c.verdict)+'</td></tr>';
      if((c.nodes||[]).length>0){
        h+='<tr class="sub"><td colspan="9"><details class="nodes" id="dn'+ri+'-'+ci+'"'+(OPEN['dn'+ri+'-'+ci]?' open':'')+'><summary>机器明细 · '+c.nodes.length+'</summary><div class="scroll"><table><thead><tr><th>机器</th><th>IP</th><th>可分配</th><th>运行中</th><th>剩余</th></tr></thead><tbody>';
        c.nodes.forEach(function(n){
          h+='<tr><td>'+esc(n.name)+'</td><td class="dim">'+esc(n.ip||'')+'</td><td>'+fmtN(n.allocatable)+'</td><td>'+fmtN(n.running)+'</td><td class="'+(n.free>0?'ok':'')+'">'+fmtN(n.free)+'</td></tr>';
        });
        h+='</tbody></table></div></details></td></tr>';
      }
    });
    h+='</tbody></table></div></details></section>';
  });
  return h;
}
function htmlQueue(){
  if(STATE.resources.length===0)return emptyMsg();
  if(!QRES){
    var first=null;
    STATE.resources.forEach(function(r){if(!first&&r.pending>0)first=r});
    QRES=(first||STATE.resources[0]).name;
  }
  var h='<div class="chips">';
  STATE.resources.forEach(function(r){
    h+='<button class="chip'+(r.name===QRES?' on':'')+'" data-res="'+esc(r.name)+'"><span class="st '+chipState(r)+'"></span>'+esc(r.name)+(r.pending>0?'<b>排队 '+fmtN(r.pending)+'</b>':'<b>无排队</b>')+'</button>';
  });
  h+='</div>';
  var res=null;
  STATE.resources.forEach(function(r){if(r.name===QRES)res=r});
  if(!res)return emptyMsg();
  h+='<div class="searchrow"><input id="qsearch" type="text" placeholder="找我的任务（名称或命名空间）…" value="'+esc(QUERY)+'"><span class="mline" id="mline"></span></div>';
  var q=QUERY.trim().toLowerCase();
  MINE=[];
  res.clusters.forEach(function(c){
    var name=cname(c.cluster);
    h+='<section class="qsec"><h2>'+esc(name)+(c.absent?'<span class="abs">无此资源</span>':'')+'</h2>'
      +'<div class="verdict '+c.verdict+'"><b>'+vWord(c.verdict)+'</b>'+esc(c.note)+'</div>';
    var rows='',ahead=0;
    c.queue.forEach(function(g){
      if(q&&(g.namespace+' '+g.workload+' '+(g.project||'')).toLowerCase().indexOf(q)<0){ahead+=g.demand;return}
      if(q)MINE.push('「'+g.workload+'」在 '+name+' 排第 '+g.position+' 位（'+(g.project||'未标记')+'），前方还需 '+fmtN(ahead)+' '+res.unit+'，已等待 '+(g.oldestSince?fmtAge(now()-g.oldestSince):'未知时长'));
      rows+='<tr'+(q?' class="mine"':'')+'><td>#'+g.position+'</td><td data-since="'+g.oldestSince+'">'+(g.oldestSince?fmtAge(now()-g.oldestSince):'—')+'</td><td>'+esc(g.namespace)+'</td><td class="mono">'+esc(g.workload)+'</td><td>'+projCell(g.project)+'</td><td>'+fmtN(g.demand)+'</td><td>'+(g.position===1?'—':fmtN(ahead))+'</td></tr>';
      ahead+=g.demand;
    });
    if(q&&rows==='')rows='<tr><td colspan="7" class="none">没有匹配的任务</td></tr>';
    h+='<div class="scroll"><table class="qt"><thead><tr><th>排位</th><th>已等待</th><th>命名空间</th><th>任务</th><th>项目</th><th>需求（'+esc(res.unit)+'）</th><th>前方需求</th></tr></thead><tbody>'+rows+'</tbody></table></div>';
    if(c.queue.length>0)h+='<p class="qinfo">'+fmtN(c.queue.length)+' 个任务在排队，共需 '+fmtN(c.pending)+' '+esc(res.unit)+'。</p>';
    h+='</section>';
  });
  if(q&&MINE.length===0)MINE.push('没有找到匹配「'+esc(QUERY.trim())+'」的任务');
  return h;
}
function render(){
  var main=$('main');
  var html=TAB==='capacity'?htmlCapacity():htmlQueue();
  if(RENDERED)main.classList.add('noanim');
  main.innerHTML=html;
  var els=main.querySelectorAll('[data-w]');
  if(RENDERED){
    for(var i=0;i<els.length;i++)els[i].style.width=els[i].getAttribute('data-w')+'%';
    requestAnimationFrame(function(){main.classList.remove('noanim')});
  }else{
    requestAnimationFrame(function(){requestAnimationFrame(function(){for(var i=0;i<els.length;i++)els[i].style.width=els[i].getAttribute('data-w')+'%'})});
  }
  RENDERED=true;
  if(TAB==='queue'&&$('mline')){
    $('mline').textContent=MINE.slice(0,3).join('；')+(MINE.length>3?'；等 '+MINE.length+' 条':'');
  }
  renderFoot();
  tickAges();
}
function renderFoot(){
  var errs=[];
  STATE.clusters.forEach(function(c){if(c.err)errs.push(esc(c.name)+'：'+esc(c.err))});
  $('foot').innerHTML='<p>排位按 Pod 创建时间近似，优先级或亲和性可能改变实际调度顺序。没有历史释放数据，无法预测精确等待时长——请结合队首已等待时长与空闲卡数判断。</p>'
    +'<p>状态来自 Kubernetes Watch（无轮询），视图每 5 秒刷新。'+(errs.length?'<span class="err">连接异常：'+errs.join('；')+'</span>':'')+'</p>';
}
function updateLive(){
  if(!STATE)return;
  $('dot').className='dot'+(STATE.live<STATE.total?' bad':'');
  $('liveline').textContent=STATE.live+'/'+STATE.total+' 集群在线';
}
function tickAges(){
  $('clock').textContent=new Date().toLocaleTimeString('zh-CN',{hour12:false});
  if(!STATE)return;
  var t=now(),els=document.querySelectorAll('td[data-since]');
  for(var i=0;i<els.length;i++){
    var v=parseInt(els[i].getAttribute('data-since'),10);
    els[i].textContent=v>0?fmtAge(t-v):'—';
  }
}
function poll(){
  fetch('/api/state.json',{cache:'no-store'}).then(function(r){return r.text()}).then(function(t){
    var s;
    try{s=JSON.parse(t)}catch(e){return}
    STATE=s;FETCH_AT=Date.now();
    updateLive();
    var sig=JSON.stringify(s,function(k,v){return (k==='at'||k==='nowMs')?0:v});
    if(sig===LAST_SIG)return;
    try{render();LAST_SIG=sig}
    catch(e){
      // A failed render must never wedge the page on the connecting screen:
      // show the error and force the next poll to retry even if the data
      // is unchanged.
      LAST_SIG=null;
      $('main').innerHTML='<div class="connecting">渲染出错，5 秒后自动重试：'+esc(e&&e.message?e.message:String(e))+'</div>';
    }
  }).catch(function(){});
}
function syncTabs(){
  $('tab-capacity').setAttribute('aria-selected',TAB==='capacity');
  $('tab-queue').setAttribute('aria-selected',TAB==='queue');
}
$('tab-capacity').onclick=function(){TAB='capacity';syncTabs();render()};
$('tab-queue').onclick=function(){TAB='queue';syncTabs();render()};
$('main').addEventListener('click',function(e){
  var b=e.target.closest?e.target.closest('.chip'):null;
  if(b){QRES=b.getAttribute('data-res');render()}
});
$('main').addEventListener('input',function(e){
  if(e.target&&e.target.id==='qsearch'){
    QUERY=e.target.value;
    render();
    var i=$('qsearch');
    if(i){i.focus();try{i.setSelectionRange(i.value.length,i.value.length)}catch(x){}}
  }
});
$('main').addEventListener('toggle',function(e){
  if(e.target&&e.target.id&&e.target.tagName==='DETAILS')OPEN[e.target.id]=e.target.open;
},true);
setInterval(poll,5000);
setInterval(tickAges,1000);
poll();
</script>
</body>
</html>`
