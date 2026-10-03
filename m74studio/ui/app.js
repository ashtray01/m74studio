'use strict';
const $=id=>document.getElementById(id), esc=s=>String(s??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
const COLORS=['#73b5ff','#ecad64','#62d4be','#b89bf0','#ee869a','#cdd676'];
const presets={engine:['p004','p003','p008','p016'],mixture:['p013','p015','p016','p019'],air:['p008','p009','p011','p007'],electric:['p002','p004','p043','p065']};
const S={data:null,map:new Map(),charts:[...presets.engine],active:'p004',cursor:0,view:[0,1],selection:null,filter:'numbers',tab:'graphs',page:0,format:'csv',playing:false,playBase:0,playStart:0,drag:null};
const kpiDefs=[['p004','Обороты двигателя','об/мин',0],['p003','Охлаждающая жидкость','°C',1],['p002','Напряжение бортсети','В',1],['p008','Дроссельная заслонка','%',1],['p016','Коррекция впрыска','коэф.',3],['p005','Скорость автомобиля','км/ч',1]];
const qualityNames={complete:'Полный ответ ЭБУ',partial:'Неполный ответ · есть пропуски',invalid:'Повреждённый ответ · нет измерений',recovered:'Восстановлен префикс ответа'};
let toastTimer,drawPending=false,dragDepth=0;
const param=id=>S.data?.parameters[S.map.get(id)], val=(id,i=S.cursor)=>S.data?.frames[i]?.values[S.map.get(id)];
const color=id=>COLORS[Math.max(0,S.charts.indexOf(id))%COLORS.length];
function fmt(v,dec=2){if(v==null||!Number.isFinite(v))return '—';return v.toLocaleString('ru-RU',{minimumFractionDigits:dec,maximumFractionDigits:dec,useGrouping:false});}
function valueText(p,v){if(v==null)return '—';if(p.kind==='flag')return v?'ВКЛ':'ВЫКЛ';if(p.kind==='hex')return '0x'+v.toString(16).toUpperCase().padStart(4,'0');return fmt(v,p.decimals);}
function shortTime(t){t=Math.max(0,t);let m=Math.floor(t/60),s=t-m*60;return `${String(m).padStart(2,'0')}:${s.toFixed(1).padStart(4,'0')}`;}
function duration(t){return `${Math.floor(t/60)} мин ${Math.floor(t%60)} с`;}
function tickTime(t){if(!S.data)return '';let f=S.data.frames[0].time.split(':');let sec=+f[0]*3600+ +f[1]*60+parseFloat(f[2])+t;sec=((sec%86400)+86400)%86400;return `${Math.floor(sec/3600).toString().padStart(2,'0')}:${Math.floor(sec%3600/60).toString().padStart(2,'0')}:${Math.floor(sec%60).toString().padStart(2,'0')}`;}
function toast(message,error=false){clearTimeout(toastTimer);(document.querySelector('dialog[open]')||document.body).appendChild($('toast'));$('toast').textContent=message;$('toast').classList.toggle('error',error);$('toast').hidden=false;toastTimer=setTimeout(()=>$('toast').hidden=true,error?10000:6000);}
function status(text){$('statusText').textContent=text;}
async function openFile(){if($('busy').hidden===false)return;pause();try{const d=await window.openLog();if(d)load(d);}catch(e){toast('Не удалось открыть лог: '+String(e),true);}}
function load(d){
 if(!d?.frames?.length){toast('В файле нет кадров',true);return;}
 pause();S.data=d;S.map=new Map(d.parameters.map((p,i)=>[p.id,i]));S.charts=[...presets.engine];S.active='p004';S.cursor=0;S.view=[0,Math.max(d.duration,.001)];S.selection=null;S.page=0;S.filter='numbers';$('preset').value='engine';$('searchInput').value='';
 document.querySelectorAll('[data-filter]').forEach(b=>b.classList.toggle('active',b.dataset.filter===S.filter));
 $('fileName').textContent=d.file;$('fileName').title=d.file;$('fileDetail').textContent=`${d.date||'Дата не указана'}  ·  ${d.frames[0].time} — ${d.frames.at(-1).time}  ·  ${d.frames.length.toLocaleString('ru-RU')} кадров`;
 $('csvBtn').disabled=false;$('txtBtn').disabled=false;$('emptyState').hidden=true;$('timeline').hidden=false;
 $('channelCount').textContent=`${d.stats.filter(x=>x.count).length} / ${d.parameters.length}`;$('eventCount').textContent=d.events.length;
 const passport=[['Калибровка',d.meta['Калибровка']||'Не прочитана'],['Автомобиль',d.meta['Автомобиль']||'—'],['Номер ЭБУ',d.meta['Номер ЭБУ']||'—'],['Аппаратная версия',d.meta['Аппаратная версия']||'—'],['Дата производства',d.meta['Дата производства']||'—'],['VIN',d.meta.VIN||'Нет данных']];
 $('passportFields').innerHTML=passport.map(([k,v])=>`<dt>${esc(k)}</dt><dd>${esc(v)}</dd>`).join('');
 const q=d.quality,total=d.frames.length,pct=q.complete/total*100;
 $('qualitySummary').innerHTML=`${fmt(pct,1)}% <small>полных кадров</small>`;
 $('qualityMeter').innerHTML=[[q.complete,'#60d4be'],[q.partial,'#f3a653'],[q.invalid,'#ed7b7e'],[q.recovered,'#b89bf0']].map(([n,c])=>`<i style="width:${n/total*100}%;background:${c}"></i>`).join('');
 $('qualityFields').innerHTML=[['Полные',q.complete],['Неполные',q.partial],['Повреждённые',q.invalid],['Префикс восстановлен',q.recovered],['Интервал (медиана)',fmt(d.medianInterval*1000,0)+' мс']].map(([k,v])=>`<dt>${k}</dt><dd>${v}</dd>`).join('');
 $('durationLabel').textContent=duration(d.duration);renderKPIs();renderChannels();renderCharts();setTab('graphs');renderEvents();updateSelection();updateCursor();status(`Лог открыт · ${total} кадров · ${d.stats.filter(x=>x.count).length} параметра`);
}
let kpiIDs=kpiDefs.map(d=>d[0]), draggedKpi=null;
try{const saved=JSON.parse(localStorage.getItem('m74.kpis'));if(Array.isArray(saved)&&saved.every(id=>/^p\d{3}$/.test(id)))kpiIDs=[...new Set(saved)];}catch{}
function saveKPIs(){try{localStorage.setItem('m74.kpis',JSON.stringify(kpiIDs))}catch{}renderKPIs();if(S.data)updateCursor();}
function renderKPIs(){
 const ids=S.data?kpiIDs.filter(id=>param(id)):kpiIDs;
 $('kpis').innerHTML=ids.map((id,i)=>{const def=kpiDefs.find(d=>d[0]===id),p=param(id),name=def?.[1]||p?.name||id,unit=p?.unit||def?.[2]||'';
 return `<div class="kpi" data-kpi="${id}" style="--kpi-color:${COLORS[i%COLORS.length]}"><div class="kpi-top"><span class="kpi-label" title="${esc(name)}">${esc(name)}</span><button class="kpi-handle icon-button" draggable="true" data-drag-kpi="${id}" title="Перетащить; ← / → переместить" aria-label="Переместить ${esc(name)}">⋮⋮</button><button class="icon-button" data-remove-kpi="${id}" title="Убрать показание" aria-label="Убрать ${esc(name)}">−</button></div><div class="kpi-main"><b id="kpi-${id}">—</b><small>${esc(unit)}</small></div></div>`;
 }).join('')+`<button id="addKpi" class="kpi-add" title="Добавить показание" aria-label="Добавить показание" ${S.data?'':'disabled'}>+</button>`;
}
function moveKpi(id,target){const from=kpiIDs.indexOf(id),to=kpiIDs.indexOf(target);if(from<0||to<0||from===to)return;kpiIDs.splice(from,1);kpiIDs.splice(to,0,id);saveKPIs();}
$('kpis').onclick=e=>{const remove=e.target.closest('[data-remove-kpi]');if(remove){kpiIDs=kpiIDs.filter(id=>id!==remove.dataset.removeKpi);saveKPIs();return;}if(e.target.closest('#addKpi')&&S.data){const options=S.data.parameters.filter(p=>!kpiIDs.includes(p.id));$('kpiParameter').innerHTML=options.map(p=>`<option value="${p.id}">${esc(p.name)}${p.unit?' · '+esc(p.unit):''}</option>`).join('');$('confirmKpi').disabled=!options.length;$('kpiDialog').showModal();}};
$('closeKpi').onclick=()=>$('kpiDialog').close();
$('confirmKpi').onclick=()=>{const id=$('kpiParameter').value;if(param(id)&&!kpiIDs.includes(id)){kpiIDs.push(id);saveKPIs();}$('kpiDialog').close();};
$('kpis').ondragstart=e=>{const handle=e.target.closest('[data-drag-kpi]');if(!handle)return;draggedKpi=handle.dataset.dragKpi;e.dataTransfer.setData('text/plain',draggedKpi);e.dataTransfer.effectAllowed='move';};
$('kpis').ondragover=e=>{if(!draggedKpi)return;e.preventDefault();e.stopPropagation();e.dataTransfer.dropEffect='move';document.querySelectorAll('[data-kpi]').forEach(c=>c.classList.toggle('drop-target',c===e.target.closest('[data-kpi]')));};
$('kpis').ondrop=e=>{if(!draggedKpi)return;e.preventDefault();e.stopPropagation();const target=e.target.closest('[data-kpi]');if(target)moveKpi(draggedKpi,target.dataset.kpi);draggedKpi=null;};
$('kpis').ondragend=()=>{draggedKpi=null;document.querySelectorAll('.drop-target').forEach(c=>c.classList.remove('drop-target'));};
$('kpis').onkeydown=e=>{const handle=e.target.closest('[data-drag-kpi]');if(!handle||!['ArrowLeft','ArrowRight'].includes(e.key))return;e.preventDefault();e.stopPropagation();const id=handle.dataset.dragKpi,index=kpiIDs.indexOf(id),target=kpiIDs[index+(e.key==='ArrowLeft'?-1:1)];if(target){moveKpi(id,target);document.querySelector('[data-drag-kpi="'+id+'"]').focus();}};
function renderChannels(){
 if(!S.data)return;let needle=$('searchInput').value.toLocaleLowerCase('ru').trim(),html='',group='';
 const rows=S.data.parameters.map((p,j)=>({p,j})).filter(({p})=>(S.filter==='all'||(S.filter==='flags')===(p.kind==='flag'))&&(!needle||(p.name+' '+p.description+' '+p.id).toLocaleLowerCase('ru').includes(needle))).sort((a,b)=>a.p.group.localeCompare(b.p.group,'ru')||a.j-b.j);
 for(const {p,j} of rows){let selected=S.charts.includes(p.id),available=S.data.stats[j].count>0;if(group!==p.group){group=p.group;html+=`<div class="channel-group">${esc(group)}</div>`;}
  html+=`<div class="channel-row ${selected?'selected ':''}${p.id===S.active?'active ':''}${!available?'unavailable':''}" data-id="${p.id}" style="--signal:${color(p.id)}" title="${esc(p.description)}${p.unit?' · '+esc(p.unit):''}${available?'':' · Нет данных в записи'}" tabindex="${available?0:-1}" role="checkbox" aria-checked="${selected}" aria-label="${esc(p.name)}"><span class="channel-check"></span><span class="channel-name">${esc(p.name)}</span><span class="channel-value" data-value="${p.id}">${valueText(p,val(p.id))}</span></div>`;
 }
 $('channelList').innerHTML=html||'<div class="aside-empty">Ничего не найдено</div>';$('plottedCount').textContent=`Графиков: ${S.charts.length}`;
}
function toggleChannel(id){
 if(!S.data||!S.data.stats[S.map.get(id)]?.count)return;
 if(param(id).kind==='hex'){setActive(id);toast('Контрольная сумма доступна в статистике и полном экспорте');return;}
 const i=S.charts.indexOf(id);if(i>=0){S.charts.splice(i,1);if(S.active===id)S.active=S.charts[0]||id;}
 else{S.charts.push(id);S.active=id;}
 $('preset').value='custom';S.page=0;renderChannels();renderCharts();updateStats();if(S.tab==='table')renderTable();
}
function renderCharts(){
 if(!S.data)return;const options=S.data.parameters.filter((p,i)=>S.data.stats[i].count>0&&p.kind!=='hex');
 document.querySelectorAll('.chart-canvas').forEach(c=>observeCanvas.unobserve(c));
 $('graphs').innerHTML=S.charts.length?S.charts.map(id=>{let p=param(id);return `<article class="chart-card ${id===S.active?'active':''}" data-id="${id}" style="--signal:${color(id)}"><div class="chart-heading"><i class="signal-dot"></i><select data-chart-select="${id}" aria-label="Сигнал графика">${options.map(p2=>`<option value="${p2.id}" ${p2.id===id?'selected':''}>${esc(p2.name)}</option>`).join('')}</select><span class="chart-unit">${esc(p.unit)}</span><span class="chart-current" data-current="${id}">${valueText(p,val(id))}</span><button class="icon-button" data-remove="${id}" title="Убрать график">×</button></div><canvas class="chart-canvas" data-signal="${id}" aria-label="${esc(p.name)}"></canvas></article>`;}).join(''):'<div class="aside-empty">Выберите сигналы слева или готовый набор над графиками</div>';
 document.querySelectorAll('.chart-canvas').forEach(c=>{observeCanvas.observe(c);attachCanvas(c,false)});scheduleDraw();
}
function lowerBound(t){let lo=0,hi=S.data.frames.length;while(lo<hi){let mid=(lo+hi)>>1;if(S.data.frames[mid].t<t)lo=mid+1;else hi=mid;}return lo;}
function nearest(t){let i=lowerBound(t);if(i<=0)return 0;if(i>=S.data.frames.length)return S.data.frames.length-1;return t-S.data.frames[i-1].t<=S.data.frames[i].t-t?i-1:i;}
function bounds(a,b){let start=lowerBound(a),end=lowerBound(b);if(end>=S.data.frames.length||S.data.frames[end].t>b)end--;return [Math.min(start,S.data.frames.length-1),Math.max(-1,end)];}
function updateCursor(){
 if(!S.data)return;let f=S.data.frames[S.cursor];
 for(const id of kpiIDs){const el=$('kpi-'+id),p=param(id);if(el&&p)el.textContent=valueText(p,val(id));}
 document.querySelectorAll('[data-value]').forEach(el=>{let id=el.dataset.value;el.textContent=valueText(param(id),val(id))});
 document.querySelectorAll('[data-current]').forEach(el=>{let id=el.dataset.current;el.textContent=valueText(param(id),val(id))});
 $('cursorTime').textContent=f.time;$('frameIndex').textContent=`${S.cursor+1} / ${S.data.frames.length}`;$('frameStatus').textContent=qualityNames[f.quality]+(f.request?' · '+f.request:'');$('frameStatus').classList.toggle('warning',f.quality!=='complete');$('rawFrame').textContent=f.raw||'Ответ отсутствует';
 $('statusRange').textContent=`Кадр ${S.cursor+1} · ${f.time} · строка ${f.line}`;
 if(S.tab==='table')document.querySelectorAll('tr[data-frame]').forEach(row=>row.classList.toggle('current',+row.dataset.frame===S.cursor));scheduleDraw();
}
function setCursor(i,reveal=false){if(!S.data)return;S.cursor=Math.max(0,Math.min(S.data.frames.length-1,i));let t=S.data.frames[S.cursor].t;if(reveal&&(t<S.view[0]||t>S.view[1])){let span=S.view[1]-S.view[0];setView(t-span/2,t+span/2)}updateCursor();}
function setActive(id){S.active=id;document.querySelectorAll('.chart-card').forEach(el=>el.classList.toggle('active',el.dataset.id===id));document.querySelectorAll('.channel-row').forEach(el=>el.classList.toggle('active',el.dataset.id===id));updateStats();}
function setView(a,b){if(!S.data)return;let total=Math.max(.001,S.data.duration),span=Math.max(Math.min(total,Math.max(.1,S.data.medianInterval*2)),Math.min(total,b-a));a=Math.max(0,Math.min(total-span,a));S.view=[a,a+span];scheduleDraw();}
function zoom(factor,anchor){if(!S.data)return;let [a,b]=S.view;anchor??=(a+b)/2;let span=(b-a)*factor,ratio=(anchor-a)/(b-a);setView(anchor-span*ratio,anchor+span*(1-ratio));}
function fit(){if(!S.data)return;S.view=[0,Math.max(S.data.duration,.001)];scheduleDraw();}
function updateSelection(){
 let sel=S.selection;if(sel){let n=sel[1]-sel[0]+1,a=S.data.frames[sel[0]],b=S.data.frames[sel[1]];$('selectionLabel').textContent=`${a.time} → ${b.time} · ${fmt(b.t-a.t,3)} с · ${n} кадров`;}else $('selectionLabel').textContent='Выделение не задано';
 $('selectionLabel').parentElement.classList.toggle('selected',!!sel);$('zoomSelection').disabled=!sel;$('clearSelection').disabled=!sel;updateStats();scheduleDraw();
}
function updateStats(){
 if(!S.data)return;let p=param(S.active),j=S.map.get(S.active);if(!p)return;let st;
 if(S.selection){let min=Infinity,max=-Infinity,sum=0,n=0;for(let i=S.selection[0];i<=S.selection[1];i++){let v=S.data.frames[i].values[j];if(v==null)continue;min=Math.min(min,v);max=Math.max(max,v);sum+=v;n++;}st={min:n?min:null,max:n?max:null,mean:n?sum/n:null,count:n};}else st=S.data.stats[j];
 $('statName').textContent=p.name+(p.unit?' · '+p.unit:'');$('statsScope').textContent=S.selection?'ВЫДЕЛЕНИЕ':'ВСЯ ЗАПИСЬ';$('statMin').textContent=valueText(p,st.min);$('statMax').textContent=valueText(p,st.max);$('statMean').textContent=fmt(st.mean,p.kind==='flag'?3:Math.min(3,p.decimals));$('statCount').textContent=st.count.toLocaleString('ru-RU');
}
function scheduleDraw(){if(drawPending)return;drawPending=true;requestAnimationFrame(()=>{drawPending=false;if(!S.data)return;if(S.tab==='graphs')document.querySelectorAll('.chart-canvas').forEach(c=>drawChart(c,c.dataset.signal,false));drawChart($('overview'),'p004',true);});}
const observeCanvas=new ResizeObserver(scheduleDraw);
function setupCanvas(c){let r=c.getBoundingClientRect(),dpr=window.devicePixelRatio||1;if(r.width<2||r.height<2)return null;let w=Math.round(r.width*dpr),h=Math.round(r.height*dpr);if(c.width!==w||c.height!==h){c.width=w;c.height=h;}let ctx=c.getContext('2d');ctx.setTransform(dpr,0,0,dpr,0,0);ctx.clearRect(0,0,r.width,r.height);return {ctx,w:r.width,h:r.height};}
function yLabel(v,p){if(p.kind==='flag')return v===0?'0':v===1?'1':'';if(Math.abs(v)>=10000)return (v/1000).toFixed(1)+'k';return v.toLocaleString('ru-RU',{maximumFractionDigits:Math.abs(v)<2?3:Math.abs(v)<20?2:1,useGrouping:false});}
function drawChart(c,id,overview){
 let cv=setupCanvas(c);if(!cv||!S.data)return;let {ctx,w,h}=cv,p=param(id),j=S.map.get(id);if(!p)return;let left=overview?8:56,right=overview?8:18,top=overview?4:7,bottom=overview?17:21,pw=w-left-right,ph=h-top-bottom;if(ph<4||pw<4)return;
 let [a,b]=overview?[0,Math.max(S.data.duration,.001)]:S.view,frames=S.data.frames;
 let lo=Math.max(0,lowerBound(a)-1),hi=Math.min(frames.length-1,lowerBound(b)),min=Infinity,max=-Infinity;
 for(let i=lo;i<=hi;i++){let v=frames[i].values[j];if(v!=null){min=Math.min(min,v);max=Math.max(max,v)}}
 let has=Number.isFinite(min),ticks;
 if(!has){min=0;max=1;}
 if(p.kind==='flag'){min=-.15;max=1.15;ticks=[0,1];}
 else if(overview){min=0;max=Math.max(1,max*1.12);}
 else{let span=max-min;if(span===0)span=Math.max(Math.abs(max)*.1,.1);min-=span*.06;max+=span*.06;let rough=(max-min)/4,base=10**Math.floor(Math.log10(rough)),step=[1,2,2.5,5,10].find(n=>n*base>=rough)*base;min=Math.floor(min/step)*step;max=Math.ceil(max/step)*step;if(!p.signedComplement&&p.offset>=0&&min<0)min=0;ticks=[];for(let v=min;v<=max+step*.001;v+=step)ticks.push(Math.abs(v)<step*.001?0:v);}
 let x=t=>left+(t-a)/(b-a)*pw,y=v=>top+(max-v)/(max-min)*ph;
 ctx.font='9px Consolas';ctx.fillStyle='#5f7793';ctx.strokeStyle='#263243';ctx.lineWidth=1;
 if(!overview){
  const maxTicks=Math.max(2,Math.floor(ph/17)+1);if(ticks.length>maxTicks){ticks=Array.from({length:maxTicks},(_,i)=>ticks[Math.round(i*(ticks.length-1)/(maxTicks-1))]);}
  ticks.forEach(v=>{let yy=y(v);ctx.beginPath();ctx.moveTo(left,Math.round(yy)+.5);ctx.lineTo(w-right,Math.round(yy)+.5);ctx.stroke();ctx.textAlign='right';ctx.fillText(yLabel(v,p),left-8,yy+3)});
 }
 let steps=Math.max(2,Math.min(7,Math.floor(pw/110)));
 for(let k=0;k<=steps;k++){let t=a+(b-a)*k/steps,xx=x(t);ctx.strokeStyle='#223044';if(!overview){ctx.beginPath();ctx.moveTo(Math.round(xx)+.5,top);ctx.lineTo(Math.round(xx)+.5,h-bottom);ctx.stroke();}ctx.fillStyle='#5e748f';ctx.textAlign=k===0?'left':k===steps?'right':'center';ctx.fillText(tickTime(t),xx,h-5);}
 ctx.save();ctx.beginPath();ctx.rect(left,top,pw,ph);ctx.clip();
 if(S.selection){let [i,k]=S.selection;let sx=x(frames[i].t),ex=x(frames[k].t);ctx.fillStyle='#f3a65312';ctx.fillRect(sx,top,Math.max(1,ex-sx),ph);ctx.strokeStyle='#f3a65380';ctx.setLineDash([3,3]);for(let xx of [sx,ex]){ctx.beginPath();ctx.moveTo(xx,top);ctx.lineTo(xx,h-bottom);ctx.stroke()}ctx.setLineDash([]);}
 if(overview){let vx=x(S.view[0]),ex=x(S.view[1]);ctx.fillStyle='#69afff13';ctx.fillRect(vx,top,ex-vx,ph);ctx.strokeStyle='#6a98c866';ctx.strokeRect(vx,top,ex-vx,ph);}
 let stroke=overview?'#658bb1':color(id);ctx.strokeStyle=stroke;ctx.lineWidth=overview?1:1.45;ctx.beginPath();
 // Preserve extrema within a pixel column. Null values and time gaps always break the line.
 const gap=Math.max(1.5,S.data.medianInterval*6);let connected=false,lastT=0,lastY=0;
 function point(i){let f=frames[i],v=f.values[j];if(v==null){connected=false;return;}let xx=x(f.t),yy=y(v);if(!connected||f.t-lastT>gap)ctx.moveTo(xx,yy);else{if(p.kind==='flag')ctx.lineTo(xx,lastY);ctx.lineTo(xx,yy);}connected=true;lastT=f.t;lastY=yy;}
 if(hi-lo<pw*2||p.kind==='flag'){for(let i=lo;i<=hi;i++)point(i);}else{
  let start=lo;while(start<=hi){let f=frames[start];if(f.values[j]==null){point(start++);continue;}let pixel=Math.floor(x(f.t)),end=start,minI=start,maxI=start;while(end+1<=hi&&Math.floor(x(frames[end+1].t))===pixel&&frames[end+1].values[j]!=null&&frames[end+1].t-frames[end].t<=gap){end++;if(frames[end].values[j]<frames[minI].values[j])minI=end;if(frames[end].values[j]>frames[maxI].values[j])maxI=end;}[...new Set([start,minI,maxI,end])].sort((u,v)=>u-v).forEach(point);start=end+1;}
 }
 ctx.stroke();
 if(S.drag?.canvas===c&&S.drag.moved&&S.drag.mode!=='pan'){let sx=x(S.drag.startT),ex=x(S.drag.endT);ctx.fillStyle='#f3a65325';ctx.fillRect(Math.min(sx,ex),top,Math.max(1,Math.abs(ex-sx)),ph);}
 let cursor=frames[S.cursor],cx=x(cursor.t),v=cursor.values[j];
 if(cursor.t>=a&&cursor.t<=b){ctx.strokeStyle=overview?'#f3a653':'#a4b8d096';ctx.lineWidth=1;ctx.setLineDash([3,3]);ctx.beginPath();ctx.moveTo(cx,top);ctx.lineTo(cx,h-bottom);ctx.stroke();ctx.setLineDash([]);if(!overview&&v!=null){ctx.fillStyle=stroke;ctx.beginPath();ctx.arc(cx,y(v),3,0,Math.PI*2);ctx.fill();}}
 ctx.restore();
 if(!has&&!overview){ctx.textAlign='center';ctx.fillStyle='#5d7694';ctx.font='11px Segoe UI';ctx.fillText('Нет данных в этом интервале',left+pw/2,top+ph/2)}
 if(!overview&&has&&hi===lo&&frames[lo].values[j]!=null){ctx.fillStyle=stroke;ctx.beginPath();ctx.arc(x(frames[lo].t),y(frames[lo].values[j]),2.5,0,Math.PI*2);ctx.fill();}
}
function canvasTime(c,clientX,overview){let r=c.getBoundingClientRect(),l=overview?8:56,rr=overview?8:18,ratio=Math.max(0,Math.min(1,(clientX-r.left-l)/(r.width-l-rr))),[a,b]=overview?[0,Math.max(.001,S.data.duration)]:S.view;return a+ratio*(b-a);}
function attachCanvas(c,overview){
 // The overview is navigation only; range selection belongs to the main charts.
 const canPan=()=>overview;
 const resetDrag=()=>{if(S.drag?.canvas===c)S.drag=null;c.style.cursor=overview?'grab':'crosshair';scheduleDraw();};
 c.addEventListener('pointerdown',e=>{if(!S.data||e.button!==0)return;pause();let t=canvasTime(c,e.clientX,overview),mode=overview?'pan':'select';if(overview&&(t<S.view[0]||t>S.view[1])){const span=S.view[1]-S.view[0];setView(t-span/2,t+span/2);}if(!overview)setActive(c.dataset.signal);S.drag={canvas:c,startX:e.clientX,startT:t,endT:t,moved:false,mode,view:[...S.view]};c.setPointerCapture(e.pointerId);if(mode==='pan')c.style.cursor='grabbing';else setCursor(nearest(t));});
 c.addEventListener('pointermove',e=>{if(!S.data)return;let t=canvasTime(c,e.clientX,overview),d=S.drag;if(d?.canvas===c){d.endT=t;if(Math.abs(e.clientX-d.startX)>4)d.moved=true;if(d.mode==='pan'&&d.moved){const delta=t-d.startT;setView(d.view[0]+delta,d.view[1]+delta);}else scheduleDraw();}else{c.style.cursor=canPan(t)?'grab':'crosshair';if(!S.playing)setCursor(nearest(t));}});
 c.addEventListener('pointerup',e=>{let d=S.drag;if(!d||d.canvas!==c)return;if(d.mode==='pan'){if(!d.moved)setCursor(nearest(d.endT));}else if(d.moved){let i=nearest(Math.min(d.startT,d.endT)),j=nearest(Math.max(d.startT,d.endT));S.selection=[i,j];updateSelection();}else setCursor(nearest(d.endT));resetDrag();if(c.hasPointerCapture(e.pointerId))c.releasePointerCapture(e.pointerId);});
 c.addEventListener('pointercancel',resetDrag);
 c.addEventListener('lostpointercapture',resetDrag);
 if(overview)c.addEventListener('dblclick',()=>{S.drag=null;fit()});

}
function panWheel(e){const span=S.view[1]-S.view[0],delta=(e.deltaY||e.deltaX)*.001*span;setView(S.view[0]+delta,S.view[1]+delta);}
$('graphs').addEventListener('wheel',e=>{if(!S.data||!e.shiftKey)return;e.preventDefault();panWheel(e);},{passive:false});
$('timeline').addEventListener('wheel',e=>{if(!S.data)return;e.preventDefault();if(e.shiftKey)panWheel(e);else{const delta=e.deltaY||e.deltaX;if(delta)zoom(delta>0?1.2:1/1.2,e.target===$('overview')?canvasTime($('overview'),e.clientX,true):undefined);}},{passive:false});
function setTab(tab){
 S.tab=tab;document.querySelectorAll('[data-view]').forEach(b=>b.classList.toggle('active',b.dataset.view===tab));$('graphs').hidden=!S.data||tab!=='graphs';$('tableView').hidden=!S.data||tab!=='table';$('eventsView').hidden=!S.data||tab!=='events';
 if(tab==='table')renderTable();if(tab==='events')renderEvents();scheduleDraw();
}
function renderTable(){
 if(!S.data)return;let cols=S.charts.map(id=>S.map.get(id)),frames=S.data.frames,pageSize=80,pages=Math.ceil(frames.length/pageSize);S.page=Math.max(0,Math.min(pages-1,S.page));let start=S.page*pageSize;
 let html='<table><thead><tr><th>Кадр<small>№ / качество</small></th><th>Время<small>из лога</small></th>'+cols.map(j=>`<th>${esc(S.data.parameters[j].name)}<small>${esc(S.data.parameters[j].unit||'значение')}</small></th>`).join('')+'</tr></thead><tbody>';
 for(let i=start;i<Math.min(start+pageSize,frames.length);i++){let f=frames[i];html+=`<tr data-frame="${i}" class="${f.quality} ${i===S.cursor?'current':''}" title="${esc(qualityNames[f.quality])}"><td>${i+1}${f.quality!=='complete'?' · !':''}</td><td>${f.time}</td>${cols.map(j=>`<td>${valueText(S.data.parameters[j],f.values[j])}</td>`).join('')}</tr>`;}
 $('dataTable').innerHTML=html+'</tbody></table>';$('pageLabel').textContent=`${S.page+1} / ${pages}`;$('prevPage').disabled=S.page===0;$('nextPage').disabled=S.page===pages-1;
}
function renderEvents(){if(!S.data)return;let type=$('eventFilter').value,events=S.data.events.filter(e=>type==='all'||e.kind===type);$('eventList').innerHTML=events.length?events.map(e=>`<div class="event-row" data-event-frame="${e.frame}"><time>${e.time}</time><span class="event-kind ${e.kind}">${{state:'Состояние',quality:'Данные',gap:'Пауза'}[e.kind]}</span><div class="event-message">${esc(e.message)}<small>Кадр ${e.frame+1} · перейти к отсчёту ↗</small></div></div>`).join(''):'<div class="aside-empty">Событий этой категории нет</div>';}
function exportBounds(){let mode=$('exportRange').value;if(mode==='selection'&&S.selection)return S.selection;if(mode==='view')return bounds(...S.view);return [0,S.data.frames.length-1];}
function exportPreview(){if(!S.data)return;let [a,b]=exportBounds(),params=$('exportParams').value==='charts'?S.charts.length:S.data.stats.filter(x=>x.count).length,count=Math.max(0,b-a+1);$('exportPreview').textContent=`${count.toLocaleString('ru-RU')} кадров · ${params} параметров · ${S.data.frames[a]?.time||'—'} — ${S.data.frames[b]?.time||'—'}\nUTF-8 · пропуски сохранены`;$('confirmExport').disabled=!count||!params;}
function showExport(format){if(!S.data)return;pause();S.format=format;$('exportTitle').textContent=format==='csv'?'Таблица CSV':'Текстовый отчёт';$('exportDescription').textContent=format==='csv'?'Таблица с временем, качеством кадров, названиями параметров и единицами измерения.':'Паспорт ЭБУ, качество записи, статистика, события и полные данные TSV. Подходит для анализа нейросетью.';$('excelField').hidden=format!=='csv';$('exportRange').querySelector('[value="selection"]').disabled=!S.selection;$('exportRange').value=S.selection?'selection':'all';exportPreview();$('exportDialog').showModal();}
async function exportToFile(){let [start,end]=exportBounds(),parameters=$('exportParams').value==='charts'?[...S.charts]:[];let opts={start,end,parameters,format:S.format,excel:$('excelOption').checked};$('confirmExport').disabled=true;try{const path=await window.saveExport(opts);if(path){$('exportDialog').close();toast('Сохранено: '+path);status('Экспорт завершён · '+(end-start+1)+' кадров');}}catch(e){toast('Ошибка экспорта: '+String(e),true)}finally{exportPreview();}}
function pause(){S.playing=false;$('playBtn').textContent='▶';$('playBtn').title='Воспроизвести · пробел';}
function play(){if(!S.data)return;if(S.playing){pause();return;}if(S.cursor>=S.data.frames.length-1)setCursor(0);S.playing=true;S.playStart=performance.now();S.playBase=S.data.frames[S.cursor].t;$('playBtn').textContent='Ⅱ';$('playBtn').title='Пауза · пробел';playFrame();}
function playFrame(){if(!S.playing)return;let t=S.playBase+(performance.now()-S.playStart)/1000*Number($('playSpeed').value);setCursor(nearest(t),true);if(t>=S.data.duration){pause();return}requestAnimationFrame(playFrame);}
$('openBtn').onclick=openFile;$('emptyOpen').onclick=openFile;$('csvBtn').onclick=()=>showExport('csv');$('txtBtn').onclick=()=>showExport('txt');$('aboutBtn').onclick=()=>$('aboutDialog').showModal();$('closeAbout').onclick=()=>$('aboutDialog').close();
$('searchInput').oninput=renderChannels;
document.querySelectorAll('[data-filter]').forEach(b=>b.onclick=()=>{S.filter=b.dataset.filter;document.querySelectorAll('[data-filter]').forEach(el=>el.classList.toggle('active',el===b));renderChannels()});
$('channelList').onclick=e=>{let row=e.target.closest('[data-id]');if(row)toggleChannel(row.dataset.id)};
$('channelList').onkeydown=e=>{let row=e.target.closest('[data-id]');if(row&&(e.key==='Enter'||e.key===' ')){e.preventDefault();e.stopPropagation();toggleChannel(row.dataset.id)}};
$('graphs').onclick=e=>{let remove=e.target.closest('[data-remove]');if(remove){toggleChannel(remove.dataset.remove);return;}let card=e.target.closest('.chart-card');if(card)setActive(card.dataset.id)};
$('graphs').onchange=e=>{let old=e.target.dataset.chartSelect;if(!old)return;let id=e.target.value;if(S.charts.includes(id)){toast('Этот сигнал уже отображается');e.target.value=old;return;}S.charts[S.charts.indexOf(old)]=id;S.active=id;$('preset').value='custom';renderChannels();renderCharts();updateStats();};
document.querySelectorAll('[data-view]').forEach(b=>b.onclick=()=>setTab(b.dataset.view));
$('preset').onchange=()=>{if(!S.data)return;let p=presets[$('preset').value];if(!p)return;S.charts=p.filter(id=>S.data.stats[S.map.get(id)].count>0);S.active=S.charts[0]||'p004';S.page=0;renderChannels();renderCharts();updateStats();if(S.tab==='table')renderTable();};
$('clearCharts').onclick=()=>{if(!S.data)return;$('preset').value='engine';$('preset').dispatchEvent(new Event('change'));};
$('addChart').onclick=()=>{if(!S.data)return;let p=S.data.parameters.find((p,j)=>p.kind==='number'&&S.data.stats[j].count>0&&!S.charts.includes(p.id));if(p)toggleChannel(p.id);};
$('zoomIn').onclick=()=>zoom(.65);$('zoomOut').onclick=()=>zoom(1/.65);$('fitBtn').onclick=fit;$('zoomSelection').onclick=()=>{if(S.selection)setView(S.data.frames[S.selection[0]].t,S.data.frames[S.selection[1]].t)};$('clearSelection').onclick=()=>{S.selection=null;updateSelection()};
$('prevPage').onclick=()=>{S.page--;renderTable()};$('nextPage').onclick=()=>{S.page++;renderTable()};$('dataTable').onclick=e=>{let row=e.target.closest('[data-frame]');if(row)setCursor(+row.dataset.frame,true)};
$('eventFilter').onchange=renderEvents;$('eventList').onclick=e=>{let row=e.target.closest('[data-event-frame]');if(row){setCursor(+row.dataset.eventFrame,true);setTab('graphs')}};$('qualityBtn').onclick=()=>{$('eventFilter').value='quality';setTab('events')};
$('confirmExport').onclick=exportToFile;$('exportRange').onchange=exportPreview;$('exportParams').onchange=exportPreview;$('playBtn').onclick=play;$('playSpeed').onchange=()=>{if(S.playing){pause();play()}};
document.addEventListener('keydown',e=>{let typing=['INPUT','SELECT','TEXTAREA'].includes(document.activeElement.tagName);if(e.ctrlKey&&e.key.toLowerCase()==='o'){e.preventDefault();if(!document.querySelector('dialog[open]'))openFile();return;}if(document.querySelector('dialog[open]'))return;if(e.ctrlKey&&e.key.toLowerCase()==='e'){e.preventDefault();showExport('csv');return;}if(typing)return;if(e.key==='/'){e.preventDefault();$('searchInput').focus();return;}if(!S.data)return;if(e.key==='ArrowRight'||e.key==='ArrowLeft'){e.preventDefault();pause();setCursor(S.cursor+(e.key==='ArrowRight'?1:-1)*(e.shiftKey?10:1),true)}else if(e.key===' '){e.preventDefault();play()}else if(e.key==='Home'){e.preventDefault();setCursor(0,true)}else if(e.key==='End'){e.preventDefault();setCursor(S.data.frames.length-1,true)}else if(e.key==='Escape'){S.selection=null;updateSelection()}});
document.addEventListener('dragenter',e=>{e.preventDefault();if(e.dataTransfer.types.includes('Files')){dragDepth++;$('dropOverlay').hidden=false}});document.addEventListener('dragover',e=>{e.preventDefault();if(e.dataTransfer)e.dataTransfer.dropEffect='copy'});document.addEventListener('dragleave',e=>{e.preventDefault();if(--dragDepth<=0){dragDepth=0;$('dropOverlay').hidden=true}});
document.addEventListener('drop',async e=>{e.preventDefault();dragDepth=0;$('dropOverlay').hidden=true;let file=e.dataTransfer.files[0];if(!file)return;if(file.size>64*1024*1024){toast('Файл превышает 64 МБ',true);return;}pause();$('busy').hidden=false;try{let d=await window.importLogText(file.name,await file.text());load(d)}catch(err){toast('Не удалось прочитать файл: '+String(err),true)}finally{$('busy').hidden=true}});
renderKPIs();attachCanvas($('overview'),true);observeCanvas.observe($('overview'));
async function init(){if(typeof window.initialLog!=='function'){status('Откройте M74Studio.exe для работы с файлами');return;}$('busy').hidden=false;try{let d=await window.initialLog();if(d)load(d)}catch(e){toast('Ошибка загрузки: '+String(e),true)}finally{$('busy').hidden=true}}
init();
