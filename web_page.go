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
.donutwrap{position:relative;width:88px;height:88px}
.donut{display:block;width:88px;height:88px}
.hole{position:absolute;inset:17px;border-radius:50%;background:var(--bg);display:flex;flex-direction:column;align-items:center;justify-content:center;line-height:1.2}
.hole b{font-family:var(--mono);font-size:14px;color:var(--ink)}
.hole i{font-style:normal;font-size:11px;color:var(--dim)}
.pdot{display:inline-block;width:9px;height:9px;border-radius:2px;margin-right:8px;vertical-align:-1px}
.pdot.un{background:var(--busy)}
.projrow{display:flex;flex-wrap:wrap;gap:4px 18px;margin:7px 0 0;font-size:12.5px}
.pl{color:var(--dim);white-space:nowrap}
.pl b{font-family:var(--mono);font-weight:600;color:var(--ink);margin-left:5px}
.pl i{font-style:normal;font-family:var(--mono);color:var(--full);margin-left:4px}
.sumblock{border:1px solid var(--line);border-radius:4px;padding:12px 16px 14px;margin:12px 0 18px;background:var(--panel)}
.sumblock .res-head{margin-top:10px}
.sumsel{margin:0}
.sel-label{color:var(--dim);font-size:12.5px;align-self:center;margin-right:2px}
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
.chip.series{border-style:dashed}
.chip.series.on{border-style:solid;border-color:var(--gold);color:var(--gold)}
.searchrow{display:flex;flex-wrap:wrap;gap:12px;align-items:center;margin:12px 0 6px}
#qsearch{background:var(--panel);border:1px solid var(--line);border-radius:3px;color:var(--ink);font:inherit;padding:7px 12px;width:320px;max-width:100%}
#qsearch:focus{outline:none;border-color:var(--gold)}
.mline{color:var(--dim);font-size:13px}
.hchart svg{display:block;width:100%;height:auto}
.hchart text{font-size:10px;font-family:var(--mono)}
.histread{min-height:20px;color:var(--dim);font-size:12.5px;margin-top:6px;font-family:var(--mono);word-break:break-all}
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
  <button id="tab-history" role="tab" aria-selected="false">历史回看</button>
</nav>
<main id="main"><div class="connecting">正在连接集群，等待第一批快照…</div></main>
<footer id="foot"></footer>
</div>
<script>
var STATE=null,FETCH_AT=0,LAST_SIG=null,RENDERED=false,TAB='capacity',QRES=null,QUERY='',MINE=[],OPEN={},SUMSEL=null,HIST={res:null,cluster:'all',hours:24,data:null,err:''};
function $(id){return document.getElementById(id)}
function esc(s){return String(s).replace(/[&<>"']/g,function(c){return {'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]})}
function now(){return STATE?STATE.nowMs+(Date.now()-FETCH_AT):Date.now()}
function fmtN(v){if(!isFinite(v))return '—';var r=Math.round(v*10)/10;return r.toLocaleString('zh-CN')}
function fmtRatio(v){if(!(v>=0))return '—';return Math.round(v*100)+'%'}
function ratioCls(v){return v>=1?'hot':(v>=0.8?'warn':'')}
// Project palette (ADR 0037): hues deliberately distinct from the reserved
// state colors (amber/red/green). A project keeps its color across views.
var PALETTE=['#4cb8d9','#5b8ee6','#9a7fe0','#c77bd9','#d97bb8','#a3c94d','#6fd0c0','#c98f6a'];
function hashStr(s){var h=0;for(var i=0;i<s.length;i++){h=(h*31+s.charCodeAt(i))>>>0}return h}
function pColor(name){return PALETTE[hashStr(name)%PALETTE.length]}
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
// stripAndPie builds the project-segmented meter and the donut for any
// resource-shaped figures (real or aggregated). Every color segment carries
// its own tooltip (ADR 0038).
function stripAndPie(res){
  var alloc=res.allocatable,run=res.running,pend=res.pending;
  var fill=alloc>0?Math.min(100,run/alloc*100):(run>0?100:0);
  var demand=run+pend;
  var demandEnd=alloc>0?Math.min(100,demand/alloc*100):(pend>0?100:0);
  var qW=Math.max(0,demandEnd-fill);
  var over=alloc>0&&demand>alloc?demand-alloc:0;
  // Task blocks inside project zones (ADR 0042): zone widths stay the
  // project view; each zone subdivides into its actual workloads.
  var segs='',cum=0,qcum=0;
  var byProjTask={};
  (res.tasks||[]).forEach(function(t){
    var k=t.project||'';
    (byProjTask[k]=byProjTask[k]||[]).push(t);
  });
  (res.projects||[]).forEach(function(p){
    var pk=p.name||'';
    var color=p.name?pColor(p.name):'#e2a23d';
    var qc=p.name?pColor(p.name):'#e0564f';
    var zname=p.name||'未标记';
    if(run>0&&p.running>0){
      var list=(byProjTask[pk]||[]).slice().sort(function(a,b){return b.run-a.run});
      if(!list.length)list=[{run:p.running,workload:'',cluster:''}];
      var shown=list.slice(0,8);
      var restRun=0,restN=list.length-shown.length;
      list.slice(8).forEach(function(t){restRun+=t.run});
      if(restN>0)shown=shown.concat([{run:restRun,workload:'__rest__',cluster:'',restN:restN}]);
      shown.forEach(function(t){
        var w=t.run/run*fill;
        var tt=t.workload==='__rest__'
          ?'其他 '+t.restN+' 个任务，共 '+fmtN(t.run)+' '+esc(res.unit)+'（'+zname+'）'
          :'任务 '+esc(t.workload)+'（'+zname+'，'+esc(cname(t.cluster))+'）：运行 '+fmtN(t.run)+' '+esc(res.unit);
        segs+='<div class="seg" data-w="'+w.toFixed(3)+'" style="width:0;left:'+cum.toFixed(3)+'%;background:'+color+'" title="'+tt+'"></div>';
        cum+=w;
      });
    }
    if(qW>0&&p.pending>0){
      var qlist=[];
      (res.clusters||[]).forEach(function(c){
        (c.queue||[]).forEach(function(g){
          if((g.project||'')===pk)qlist.push({workload:g.workload,demand:g.demand,cluster:c.cluster});
        });
      });
      qlist.sort(function(a,b){return b.demand-a.demand});
      var qshown=qlist.slice(0,6);
      var qrest=0,qrestN=qlist.length-qshown.length;
      qlist.slice(6).forEach(function(g){qrest+=g.demand});
      if(qrestN>0)qshown=qshown.concat([{workload:'__rest__',demand:qrest,cluster:'',restN:qrestN}]);
      qshown.forEach(function(g){
        var qw=g.demand/pend*qW;
        var tt=g.workload==='__rest__'
          ?'其他 '+g.restN+' 个排队任务，共 '+fmtN(g.demand)+' '+esc(res.unit)+'（'+zname+'）'
          :'任务 '+esc(g.workload)+'（'+zname+'，'+esc(cname(g.cluster))+'）：排队 '+fmtN(g.demand)+' '+esc(res.unit);
        segs+='<div class="seg" data-w="'+qw.toFixed(3)+'" style="width:0;left:'+(fill+qcum).toFixed(3)+'%;background:repeating-linear-gradient(45deg,'+hexA(qc,.6)+' 0 4px,transparent 4px 9px)" title="'+tt+'"></div>';
        qcum+=qw;
      });
    }
  });
  var cells='';for(var c=0;c<24;c++)cells+='<i class="cell"></i>';
  var strip='<div class="strip'+(alloc<=0?' nores':'')+'" title="实心段 = 运行中 '+fmtN(run)+'（占用 '+fmtRatio(res.runRatio)+'），每块一个任务，悬停看任务/项目/集群；斜纹段 = 排队 '+fmtN(pend)+' '+esc(res.unit)+'，同样按任务分块；斜纹铺满右缘即需求超过可分配">'+segs+'<div class="cells">'+cells+'</div></div>';
  var plist=(res.projects||[]).filter(function(p){return p.running>0});
  var pie=(run>0&&plist.length)?donutSVG(res,plist):'';
  return {strip:strip,pie:pie,over:over};
}
// donutSVG draws the running share as an SVG ring so each sector carries
// its own <title> tooltip (ADR 0038).
function donutSVG(res,plist){
  var run=res.running,parts='',pos=-0.25;
  plist.forEach(function(p){
    var share=p.running/run;
    if(share<=0)return;
    var color=p.name?pColor(p.name):'#e2a23d';
    var t=esc((p.name||'未标记')+'：运行 '+fmtN(p.running)+' '+res.unit+'（占运行中 '+fmtRatio(share)+'）');
    if(share>=0.99999){parts+='<circle cx="44" cy="44" r="44" fill="'+color+'"><title>'+t+'</title></circle>';return}
    var a0=pos*2*Math.PI,a1=(pos+share)*2*Math.PI;
    parts+='<path d="'+arcPath(44,44,17,a0,a1)+'" fill="'+color+'"><title>'+t+'</title></path>';
    pos+=share;
  });
  return '<div class="res-pie"><div class="donutwrap"><svg class="donut" viewBox="0 0 88 88" role="img" aria-label="项目占用环形图">'+parts+'</svg><div class="hole"><b>'+fmtRatio(res.runRatio)+'</b><i>占用</i></div></div></div>';
}
function arcPath(cx,R,r,a0,a1){
  function pt(rad,ang){return (cx+rad*Math.cos(ang)).toFixed(2)+','+(cx+rad*Math.sin(ang)).toFixed(2)}
  var big=(a1-a0)>Math.PI?1:0;
  return 'M'+pt(R,a0)+' A'+R+','+R+' 0 '+big+' 1 '+pt(R,a1)+' L'+pt(r,a1)+' A'+r+','+r+' 0 '+big+' 0 '+pt(r,a0)+' Z';
}
function numsFor(res,over){
  return '<div class="nums">'
    +'<span class="n"><i>总量</i><b>'+fmtN(res.total)+'</b></span>'
    +'<span class="n" title="占用 = 运行中 ÷ 可分配（与实心段一致）"><i>占用</i><b class="'+ratioCls(res.runRatio)+'">'+fmtRatio(res.runRatio)+'</b></span>'
    +'<span class="n" title="需求 =（运行中 + 排队）÷ 可分配，规划口径（ADR 0028）"><i>需求</i><b class="'+ratioCls(res.ratio)+'">'+fmtRatio(res.ratio)+'</b></span>'
    +'<span class="n"><i>运行中</i><b>'+fmtN(res.running)+'</b></span>'
    +'<span class="n"><i>排队</i><b class="'+(res.pending>0?'hot':'')+'">'+fmtN(res.pending)+'</b></span>'
    +'<span class="n"><i>可分配</i><b>'+fmtN(res.allocatable)+'</b></span>'
    +(over>0?'<span class="n" title="需求超出可分配的量 =（运行中+排队）− 可分配"><i>超配</i><b class="hot">+'+fmtN(over)+'</b></span>':'')
    +'</div>';
}
// projRow is the explicit per-resource project legend (ADR 0038).
function projRow(res){
  var ps=(res.projects||[]).filter(function(p){return p.running+p.pending>0});
  if(!ps.length)return '';
  return '<div class="projrow">'+ps.map(function(p){
    return '<span class="pl" title="'+esc(p.name||'未标记')+'：运行 '+fmtN(p.running)+' '+esc(res.unit)+'，排队 '+fmtN(p.pending)+' '+esc(res.unit)+'">'+projCell(p.name)+'<b>'+fmtN(p.running)+'</b>'+(p.pending>0?'<i>+'+fmtN(p.pending)+'</i>':'')+'</span>';
  }).join('')+'</div>';
}
// sumAggregate combines the reader's selected resource types at display
// time (ADR 0038); per-resource accounting stays untouched.
function sumAggregate(){
  var rs=STATE.resources;
  if(rs.length<2)return null;
  if(!SUMSEL){SUMSEL={}}
  rs.forEach(function(r){if(!(r.name in SUMSEL))SUMSEL[r.name]=true});
  var units={},t=0,alloc=0,run=0,pend=0,pm={};
  rs.forEach(function(r){
    if(!SUMSEL[r.name])return;
    units[r.unit]=1;
    t+=r.total;alloc+=r.allocatable;run+=r.running;pend+=r.pending;
    (r.projects||[]).forEach(function(p){
      var k=p.name||'',e=pm[k]||(pm[k]={name:k,running:0,pending:0});
      e.running+=p.running;e.pending+=p.pending;
    });
  });
  var projs=[];for(var k in pm)projs.push(pm[k]);
  projs.sort(function(a,b){var x=a.running+a.pending,y=b.running+b.pending;return x!==y?y-x:(a.name<b.name?-1:1)});
  var unit=Object.keys(units).length===1?Object.keys(units)[0]:'';
  return {name:'汇总',unit:unit,total:t,allocatable:alloc,running:run,pending:pend,projects:projs,
    runRatio:alloc>0?run/alloc:-1,ratio:alloc>0?(run+pend)/alloc:-1};
}
// seriesKey derives a hardware series from a resource name (ADR 0039):
// everything up to the last letter-or-dash that directly precedes the
// terminal variant. 910B1 → 910B, 910C560T → 910C.
function seriesKey(name){
  var m=/^(.*?[A-Za-z-])(\d.*)$/.exec(name);
  return m?m[1]:'';
}
function summaryPanel(){
  var sum=sumAggregate();
  if(!sum)return '';
  var sp=stripAndPie(sum);
  var groups={};
  STATE.resources.forEach(function(r){
    var k=seriesKey(r.name);
    if(!k)return;
    (groups[k]=groups[k]||[]).push(r.name);
  });
  var seriesButtons='';
  Object.keys(groups).sort().forEach(function(k){
    if(groups[k].length<2)return;
    var all=groups[k].every(function(n){return SUMSEL[n]});
    seriesButtons+='<button class="chip series'+(all?' on':'')+'" data-sumseries="'+esc(k)+'" title="'+esc('系列快捷：一次选中或清除全部 '+k+' 系列（'+groups[k].join('、')+'）')+'">'+esc(k)+'</button>';
  });
  var chips='<div class="chips sumsel">'
    +(seriesButtons?'<span class="sel-label">系列：</span>'+seriesButtons:'')
    +'<span class="sel-label">计入汇总：</span>'+STATE.resources.map(function(r){
    return '<button class="chip'+(SUMSEL[r.name]?' on':'')+'" data-sum="'+esc(r.name)+'">'+esc(r.name)+'</button>';
  }).join('')+'<button class="chip" data-sumall="1">全选</button><button class="chip" data-sumall="0">清空</button></div>';
  return '<section class="res sumblock">'+chips
    +'<div class="res-head"><div class="res-name" title="选中资源类型的合计占用；不同单位的资源按原始数值相加">汇总</div>'
    +sp.strip+sp.pie+numsFor(sum,sp.over)+'</div>'
    +projRow(sum)
    +'</section>';
}
function htmlCapacity(){
  if(STATE.resources.length===0)return emptyMsg();
  var h='<div class="legend"><span><i class="sw b"></i>运行中·未标记</span><span><i class="sw q"></i>排队·未标记</span><span><i class="sw pc"></i>项目色区（区内每块一个任务，悬停看任务/项目/集群）</span><span><i class="sw f"></i>空闲可分配</span><span class="leg-note">斜纹铺满右缘 = 排队需求已填满全部可分配</span></div>';
  h+=summaryPanel();
  STATE.resources.forEach(function(res,ri){
    var sp=stripAndPie(res);
    var kind=res.unit!=='张'?'<span class="kind">（'+esc(res.unit)+'）</span>':'';
    h+='<section class="res"><div class="res-head">'
      +'<div class="res-name" title="'+esc(res.name)+'">'+esc(res.name)+kind+'</div>'
      +sp.strip
      +sp.pie
      +numsFor(res,sp.over)
      +'</div>'
      +projRow(res)
      +'<details class="clusters" id="dp'+ri+'"'+(OPEN['dp'+ri]?' open':'')+'><summary>项目明细 · '+(res.projects||[]).length+'</summary><div class="scroll"><table>'
      +'<thead><tr><th>项目</th><th>运行中</th><th>排队</th><th>需求</th><th>需求占比</th></tr></thead><tbody>'
      +(res.projects||[]).map(function(p){
        return '<tr><td>'+projCell(p.name)+'</td><td>'+fmtN(p.running)+'</td><td class="'+(p.pending>0?'hot':'')+'">'+fmtN(p.pending)+'</td><td>'+fmtN(p.running+p.pending)+'</td><td>'+fmtRatio(res.running+res.pending>0?(p.running+p.pending)/(res.running+res.pending):0)+'</td></tr>';
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
// fetchHist loads the downsampled occupancy series for the current
// history controls and re-renders on completion (ADR 0040).
function fetchHist(){
  if(!HIST.res)return render();
  HIST.data=null;HIST.err='';
  var url='/api/history.json?res='+encodeURIComponent(HIST.res)+'&cluster='+encodeURIComponent(HIST.cluster)+'&hours='+HIST.hours;
  fetch(url,{cache:'no-store'}).then(function(r){return r.text()}).then(function(t){
    var s;
    try{s=JSON.parse(t)}catch(e){HIST.err='历史数据解析失败';return render()}
    if(s&&s.error){HIST.err=s.error;HIST.data=null}
    else{HIST.data=s;HIST.err=''}
    render();
  }).catch(function(){HIST.err='历史数据读取失败';render()});
}
function htmlHistory(){
  if(STATE.resources.length===0)return emptyMsg();
  if(!HIST.res){
    var first=null;
    STATE.resources.forEach(function(r){if(!first&&r.pending>0)first=r});
    HIST.res=(first||STATE.resources[0]).name;
  }
  var h='<div class="chips">'+STATE.resources.map(function(r){
    return '<button class="chip'+(r.name===HIST.res?' on':'')+'" data-hres="'+esc(r.name)+'">'+esc(r.name)+'</button>';
  }).join('')+'</div>';
  h+='<div class="chips"><span class="sel-label">集群：</span><button class="chip'+(HIST.cluster==='all'?' on':'')+'" data-hcl="all">全部</button>'
    +STATE.clusters.filter(function(c){return !c.err}).map(function(c){
      return '<button class="chip'+(HIST.cluster===c.name?' on':'')+'" data-hcl="'+esc(c.name)+'" title="'+esc(c.id)+'">'+esc(c.name)+'</button>';
    }).join('')+'</div>';
  var ranges=[[6,'6 小时'],[24,'24 小时'],[72,'3 天'],[168,'7 天'],[720,'30 天']];
  h+='<div class="chips"><span class="sel-label">时间范围：</span>'+ranges.map(function(rr){
    return '<button class="chip'+(HIST.hours===rr[0]?' on':'')+'" data-hh="'+rr[0]+'">'+rr[1]+'</button>';
  }).join('')+'</div>';
  if(HIST.err)h+='<div class="connecting">'+esc(HIST.err)+'</div>';
  else if(!HIST.data)h+='<div class="connecting">正在读取历史数据…</div>';
  else if(!HIST.data.points||!HIST.data.points.length)h+='<div class="connecting">这段时间内没有该资源的历史记录。</div>';
  else h+=histChart()+histLegend();
  h+='<p class="qinfo">历史为采样值（默认每分钟记录一次），悬停图表查看各时间点明细；采样粒度之内的瞬时波动不可见。</p>';
  return h;
}
// histChart draws the stacked SVG: solid areas = running by project,
// hatched areas on top = pending, dashed line = allocatable (ADR 0040).
function histChart(){
  var pts=HIST.data.points,W=1000,H=300,L=48,Rm=14,T=14,B=26;
  var pw=W-L-Rm,ph=H-T-B;
  var t0=pts[0].at,t1=pts[pts.length-1].at;
  if(t1<=t0)t1=t0+1;
  var ymax=1;
  pts.forEach(function(p){ymax=Math.max(ymax,p.A||0,(p.R||0)+(p.P||0))});
  ymax*=1.08;
  function X(t){return (L+(t-t0)/(t1-t0)*pw).toFixed(1)}
  function Y(v){return (T+ph-(v/ymax)*ph).toFixed(1)}
  var names={};
  pts.forEach(function(p){(p.p||[]).forEach(function(q){names[q.n||'']=1})});
  var order=Object.keys(names).sort();
  var byName={};
  pts.forEach(function(p){
    (p.p||[]).forEach(function(q){byName[q.n||'']=byName[q.n||'']||{};byName[q.n||''][p.at]={r:q.r,q:q.q}});
  });
  var runM=[],pendM=[],defs='',layers='';
  var runTop=[],pendTop=[];
  pts.forEach(function(p){
    runTop.push(p.R||0);pendTop.push((p.R||0)+(p.P||0));
  });
  order.forEach(function(n,pi){
    var runs=[],pends=[];
    pts.forEach(function(p){
      var e=byName[n][p.at];
      runs.push(e?e.r:0);pends.push(e?e.q:0);
    });
    var rc=[],pc=[];
    pts.forEach(function(p,i){
      var runBase=pi>0?runM[pi-1][i]:0;
      var pendBase=pi>0?pendM[pi-1][i]:(p.R||0);
      rc.push(runBase+runs[i]);pc.push(pendBase+pends[i]);
    });
    runM[pi]=rc;pendM[pi]=pc;
    var color=n?pColor(n):'#e2a23d';
    var qcolor=n?pColor(n):'#e0564f';
    defs+='<pattern id="hp'+hashStr(n)+'" width="6" height="6" patternUnits="userSpaceOnUse" patternTransform="rotate(45)"><line x1="0" y1="0" x2="0" y2="6" stroke="'+qcolor+'" stroke-width="2.5" opacity="0.55"/></pattern>';
    layers+=areaPath(pts,rc,pi>0?runM[pi-1]:null,X,Y,'fill:'+color+';stroke:none','run');
    layers+=areaPath(pts,pc,pi>0?pendM[pi-1]:runTop.map(function(v){return v}),X,Y,'fill:url(#hp'+hashStr(n)+');stroke:none','pend');
  });
  // reference lines and axes
  var grid='',ylab='',ylabN=4;
  for(var i=0;i<=ylabN;i++){
    var v=ymax*i/ylabN;
    var y=Y(v);
    grid+='<line x1="'+L+'" y1="'+y+'" x2="'+(W-Rm)+'" y2="'+y+'" stroke="#1d2b3c" stroke-width="1"/>';
    ylab+='<text x="'+(L-6)+'" y="'+(+y+4)+'" text-anchor="end" fill="#8290a3">'+fmtN(v)+'</text>';
  }
  var xlab='',xtN=6;
  for(var i=0;i<=xtN;i++){
    var t=t0+(t1-t0)*i/xtN;
    var d=new Date(t);
    var lab=HIST.hours<=48?d.toLocaleTimeString('zh-CN',{hour12:false,hour:'2-digit',minute:'2-digit'}):(d.getMonth()+1)+'-'+d.getDate();
    xlab+='<text x="'+X(t)+'" y="'+(H-8)+'" text-anchor="middle" fill="#8290a3">'+esc(lab)+'</text>';
    grid+='<line x1="'+X(t)+'" y1="'+T+'" x2="'+X(t)+'" y2="'+(T+ph)+'" stroke="#141d29" stroke-width="1"/>';
  }
  var alloc='',ax=pts.map(function(p){return X(p.at)+','+Y(p.A||0)});
  if(pts.some(function(p){return p.A>0}))alloc='<polyline points="'+ax.join(' ')+'" fill="none" stroke="#8290a3" stroke-width="1.2" stroke-dasharray="5 4"><title>可分配</title></polyline>';
  var demand='<polyline points="'+pendTop.map(function(v,i){return X(pts[i].at)+','+Y(v)}).join(' ')+'" fill="none" stroke="#e0564f" stroke-width="1.4"><title>需求（运行中+排队）</title></polyline>';
  return '<div class="hchart"><svg viewBox="0 0 '+W+' '+H+'" role="img" aria-label="占用历史">'
    +'<defs>'+defs+'</defs>'+grid+ylab+xlab+layers+alloc+demand
    +'<line id="histcur" x1="0" y1="'+T+'" x2="0" y2="'+(T+ph)+'" stroke="#d2a44e" stroke-width="1" opacity="0"/>'
    +'<rect id="histov" x="'+L+'" y="'+T+'" width="'+pw+'" height="'+ph+'" fill="transparent"/>'
    +'</svg><div class="histread" id="histread"> </div></div>';
}
function areaPath(pts,top,bot,X,Y,style,cls){
  var d='M'+X(pts[0].at)+','+Y(top[0]);
  for(var i=1;i<pts.length;i++)d+=' L'+X(pts[i].at)+','+Y(top[i]);
  if(bot){
    for(var i=pts.length-1;i>=0;i--)d+=' L'+X(pts[i].at)+','+Y(bot[i]);
  }else{
    var base=Y(0);
    d+=' L'+X(pts[pts.length-1].at)+','+base+' L'+X(pts[0].at)+','+base;
  }
  return '<path d="'+d+' Z" style="'+style+'" class="'+cls+'"/>';
}
function histLegend(){
  var pts=HIST.data.points,last=pts[pts.length-1];
  var names={};
  pts.forEach(function(p){(p.p||[]).forEach(function(q){names[q.n||'']=1})});
  var unit=(STATE.resources.filter(function(r){return r.name===HIST.res})[0]||{}).unit||'';
  var h='<div class="projrow">'+Object.keys(names).sort().map(function(n){
    var color=n?pColor(n):'#e2a23d';
    return '<span class="pl">'+(n?'<span class="pdot" style="background:'+color+'"></span>':'<span class="pdot un"></span>')+esc(n||'未标记')+'</span>';
  }).join('')+'<span class="pl"><span class="pdot" style="background:repeating-linear-gradient(45deg,rgba(224,86,79,.6) 0 2px,transparent 2px 5px)"></span>排队</span><span class="pl">— 可分配（'+fmtN(last.A)+' '+esc(unit)+'）</span></div>';
  return h;
}
// attachHistHover wires the chart's crosshair: nearest-point readout with
// the project breakdown, driven by mouse position in viewBox coordinates.
function attachHistHover(){
  var ov=$('histov'),cur=$('histcur');
  if(!ov||!cur||!HIST.data||!HIST.data.points||!HIST.data.points.length)return;
  var pts=HIST.data.points,t0=pts[0].at,t1=pts[pts.length-1].at;
  if(t1<=t0)t1=t0+1;
  var L=48,Rm=14,W=1000,pw=W-L-Rm;
  var unit=(STATE.resources.filter(function(r){return r.name===HIST.res})[0]||{}).unit||'';
  ov.onmousemove=function(e){
    var r=ov.getBoundingClientRect();
    var vx=L+(e.clientX-r.left)/r.width*pw;
    var t=t0+(vx-L)/pw*(t1-t0);
    var best=0,bd=Infinity;
    for(var i=0;i<pts.length;i++){
      var d=Math.abs(pts[i].at-t);
      if(d<bd){bd=d;best=i}
    }
    var p=pts[best];
    cur.setAttribute('x1',String(L+(p.at-t0)/(t1-t0)*pw));
    cur.setAttribute('x2',String(L+(p.at-t0)/(t1-t0)*pw));
    cur.setAttribute('opacity','0.8');
    var ps=(p.p||[]).slice().sort(function(a,b){return (b.r+b.q)-(a.r+a.q)});
    var projTxt=ps.slice(0,4).map(function(x){return (x.n||'未标记')+' '+fmtN(x.r)+'/'+fmtN(x.q)}).join('、')+(ps.length>4?'；等 '+ps.length+' 项':'');
    $('histread').innerHTML=new Date(p.at).toLocaleString('zh-CN',{hour12:false})+' · 运行 '+fmtN(p.R)+' · 排队 '+fmtN(p.P)+' · 需求 '+fmtN((p.R||0)+(p.P||0))+' · 可分配 '+fmtN(p.A)+(unit?' '+esc(unit):'')+(projTxt?' — '+esc(projTxt):'');
  };
  ov.onmouseleave=function(){
    cur.setAttribute('opacity','0');
    $('histread').textContent=' ';
  };
}
function render(){
  var main=$('main');
  var html=TAB==='capacity'?htmlCapacity():(TAB==='queue'?htmlQueue():htmlHistory());
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
  if(TAB==='history')attachHistHover();
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
    LAST_SIG=sig;
    // The history tab owns its own fetch cycle: live polls never rebuild
    // the chart out from under the user's hover.
    if(TAB==='history')return;
    try{render()}
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
  $('tab-history').setAttribute('aria-selected',TAB==='history');
}
$('tab-capacity').onclick=function(){TAB='capacity';syncTabs();render()};
$('tab-queue').onclick=function(){TAB='queue';syncTabs();render()};
$('tab-history').onclick=function(){TAB='history';syncTabs();render();fetchHist()};
$('main').addEventListener('click',function(e){
  var b=e.target.closest?e.target.closest('.chip'):null;
  if(!b)return;
  if(b.hasAttribute('data-hres')){HIST.res=b.getAttribute('data-hres');fetchHist();return}
  if(b.hasAttribute('data-hcl')){HIST.cluster=b.getAttribute('data-hcl');fetchHist();return}
  if(b.hasAttribute('data-hh')){HIST.hours=+b.getAttribute('data-hh');fetchHist();return}
  if(b.hasAttribute('data-sum')){
    var n=b.getAttribute('data-sum');
    SUMSEL[n]=!SUMSEL[n];
    render();return;
  }
  if(b.hasAttribute('data-sumseries')){
    var s=b.getAttribute('data-sumseries'),allOn=true;
    STATE.resources.forEach(function(r){if(seriesKey(r.name)===s&&!SUMSEL[r.name])allOn=false});
    STATE.resources.forEach(function(r){if(seriesKey(r.name)===s)SUMSEL[r.name]=!allOn});
    render();return;
  }
  if(b.hasAttribute('data-sumall')){
    var v=b.getAttribute('data-sumall')==='1';
    for(var k in SUMSEL)SUMSEL[k]=v;
    render();return;
  }
  QRES=b.getAttribute('data-res');render();
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
