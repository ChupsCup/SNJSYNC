
(function(){

if(window.matchMedia('(max-width: 760px)').matches)return; 

const W=36,H=22,T=16,cv=document.getElementById('office'),g=cv.getContext('2d');

const mk=()=>[...Array(H)].map(()=>Array(W).fill(false));
const wall=mk(),blocked=mk(),objs=[],divs=[],agents=[],byID={},seats=[],col={};
const setW=(x,y)=>wall[y][x]=blocked[y][x]=true;
const add=(kind,x,y,w=1,h=1)=>{objs.push({kind,x,y,w,h});if(kind!=='chair'&&kind!=='rug')for(let i=0;i<w;i++)for(let j=0;j<h;j++)blocked[y+j][x+i]=true};
const rnd=n=>Math.floor(Math.random()*n),pick=a=>a[rnd(a.length)],now=()=>performance.now();
for(let x=0;x<W;x++){setW(x,0);setW(x,H-1)}
for(let y=0;y<H;y++){setW(0,y);setW(W-1,y)}
for(let y=0;y<=8;y++)if(y!==5)setW(25,y);
for(let y=13;y<H;y++)if(y!==17&&y!==18)setW(25,y);
for(let x=25;x<W;x++)if(x!==29&&x!==30){setW(x,8);setW(x,13)}
for(let x=1;x<=9;x++)setW(x,14);for(let y=14;y<H;y++)if(y!==17&&y!==18)setW(9,y);
const defs=[['imp','Import','#e879f9',1,2,['Import-Bot']],['wh','Warehouse','#26c6da',6,2,['WH-Bot']],['md','Master Data','#f97316',11,2,['MD-Admin']],['inv','Inventory','#3cb371',16,2,['Inventory-Bot']],['sales','Sales','#4d96ff',21,2,['Sales-Bot']],
['fin','Finance','#ee5253',1,9,['Finance-Bot']],['log','Logistik','#ec7fb8',6,9,['Logistik-Bot']],['it','IT','#a55eea',11,9,['IT-Bot']],['mgmt','Management','#8d9db6',16,9,['Mgmt-Bot']],['qc','QC','#1dd1a1',21,9,['QC-Bot']]];
for(const [id,name,color,ox,oy,ns] of defs){
  divs.push({id,name,color,ox,oy});col[id]=color;if(ns.length<2){add('panel',ox,oy+1,1,2);add('cabinet',ox+3,oy+1)}
  ns.forEach((n,i)=>{const dx=ox+(ns.length<2?1:i*2);add('desk',dx,oy+1,2,1);add('chair',dx,oy+2);
    const a={id:id+(i+1),n,d:id,x:dx,y:oy+2,dx,dy:oy+2,dir:'up',act:'work',face:'up',t:'',txt:'',mv:false,i:agents.length,desk:{x:dx,y:oy+2},path:[],script:[],dur:0,until:now()+1000+rnd(4000),bub:0,target:''};
    agents.push(a);byID[a.id]=a});
}
add('table',28,3,4,2);
for(let x=28;x<=31;x++){seats.push({x,y:2},{x,y:5});add('chair',x,2);add('chair',x,5)}
add('counter',26,14,3,1);add('fridge',34,14);add('table',31,17);add('chair',30,17);add('chair',32,17);add('printer',24,7);
for(const [x,y] of [[5,1],[10,1],[15,1],[20,1],[24,1],[5,13],[10,13],[15,13],[20,13],[24,13],[10,20],[24,20],[26,1],[34,1],[34,10],[26,12],[26,20],[34,20]])add('plant',x,y);
for(const x of[2,3,5,6])add('rack',x,15,1,2);
add('sofa',11,15,3,1);add('sofa',19,15,3,1);add('rug',12,17,10,2);add('reception',14,19,4,1);add('rug',16,20,2,1);add('cooler',23,15);
const coffee=[{x:26,y:15},{x:27,y:15},{x:28,y:15}],chat=[{x:30,y:17},{x:32,y:17},{x:31,y:16},{x:31,y:18}];
let nextMeet=now()+60000;

function bfs(f,t){
  if(f.x===t.x&&f.y===t.y)return[];
  const k=p=>p.y*W+p.x,prev=new Map([[k(f),null]]),q=[f];
  for(let h=0;h<q.length;h++){const c=q[h];
    for(const [dx,dy] of [[1,0],[-1,0],[0,1],[0,-1]]){const n={x:c.x+dx,y:c.y+dy};
      if(n.x<0||n.y<0||n.x>=W||n.y>=H||blocked[n.y][n.x]||prev.has(k(n)))continue;
      prev.set(k(n),c);q.push(n)}}
  if(!prev.has(k(t)))return[];
  const path=[];for(let c=t;c&&k(c)!==k(f);c=prev.get(k(c)))path.unshift(c);return path;
}
const faceOf=s=>s.y<4?'down':'up';
function dest(a,act,target){
  switch(act){
    case'work':return[a.desk,'up'];
    case'coffee':return[pick(coffee),'up'];
    case'chat':return[pick(chat),'down'];
    case'archive':return[pick([{x:3,y:17},{x:5,y:17}]),'up'];
    case'cooler':return[{x:23,y:16},'up'];
    case'printer':return[{x:24,y:8},'up'];
    case'meeting':{const s=pick(seats);return[s,faceOf(s)]}
    case'visit':{const t=byID[target];if(!t||t===a)return null;return[{x:t.desk.x+1,y:t.desk.y},'left']}
    case'wander':for(let i=0;i<50;i++){const p={x:1+rnd(23),y:1+rnd(19)};if(!blocked[p.y][p.x])return[p,'down']}
  }
  return null;
}
function plan(a,act,d,face,target,dur,txt){
  let from={x:Math.round(a.x),y:Math.round(a.y)},pre=[];
  if(a.path.length){from=a.path[0];pre=[from]}
  a.path=pre.concat(bfs(from,d));
  a.script=[];Object.assign(a,{act,face,target,dur,txt,until:0});
  if(!a.path.length)arrive(a);
}
function arrive(a){
  const n=now();a.dir=a.face;a.until=n+a.dur;a.bub=n+Math.min(a.dur,4000);
  if(a.act==='visit')talk(a,n);
}
function move(a){
  const t=a.path[0],dx=t.x-a.x,dy=t.y-a.y,d=Math.hypot(dx,dy);
  if(d>.01)a.dir=Math.abs(dx)>Math.abs(dy)?(dx>0?'right':'left'):(dy>0?'down':'up');
  if(d<=.3){a.x=t.x;a.y=t.y;a.path.shift()}else{a.x+=dx/d*.3;a.y+=dy/d*.3}
}
const WT={md:['Menjaga konsistensi master data','Menyelaraskan data lintas divisi','Memastikan kualitas data terjaga','Mendukung kebutuhan data seluruh tim'],imp:['Mengelola informasi barang impor','Menyiapkan dokumen pendukung','Berkoordinasi dengan supplier','Memantau proses kedatangan barang'],wh:['Mengelola data penyimpanan barang','Memantau aktivitas gudang','Menjaga ketertiban lokasi barang','Mendukung kelancaran operasional gudang'],inv:['Memantau ketersediaan persediaan','Menjaga akurasi data persediaan','Mengelola informasi status produk','Menyusun ringkasan persediaan'],sales:['Memantau informasi produk dan harga','Melayani kebutuhan pelanggan','Menyusun rencana penjualan','Berkoordinasi dengan tim terkait'],fin:['Menyusun laporan keuangan','Memantau data akuntansi','Mendukung proses pembayaran','Menjaga kelengkapan administrasi'],log:['Merencanakan distribusi barang','Berkoordinasi dengan mitra pengiriman','Memantau jadwal pengiriman','Mengoptimalkan rute distribusi'],it:['Menjaga stabilitas sistem ERP','Mendukung integrasi antar sistem','Memantau keamanan data','Mendukung kebutuhan pengguna'],mgmt:['Meninjau laporan kinerja','Menyusun strategi bisnis','Mengambil keputusan berbasis data','Berkoordinasi dengan seluruh divisi'],qc:['Memastikan standar mutu produk','Menyusun standar pemeriksaan','Memantau kualitas produk','Mendukung kepatuhan standar']};
const NEED={imp:'data barang impor',wh:'data penyimpanan barang',inv:'data persediaan',sales:'data produk dan harga',fin:'data akuntansi',log:'data distribusi',it:'data sistem',mgmt:'data untuk laporan',qc:'data standar mutu',md:'master data'};
const OWN={imp:'proses impor',wh:'operasional gudang',inv:'data persediaan',sales:'kegiatan penjualan',fin:'laporan keuangan',log:'jadwal pengiriman',it:'sistem ERP',mgmt:'laporan kinerja',qc:'standar mutu',md:'master data'};
const sku=()=>'SKU-'+(20000+rnd(9999));
const toMD=[
 (n)=>[`Selamat pagi, kami ingin berkoordinasi mengenai ${n}`,`Selamat pagi, dengan senang hati. Silakan disampaikan`,`Kami ingin memastikan informasinya selaras dengan master data`,`Baik, mari kita selaraskan bersama`],
 (n)=>[`Terima kasih atas dukungan Master Data selama ini`,`Sama-sama, kami senang bisa membantu`,`Semoga ${n} kami terus selaras dengan sistem`,`Tentu, kami siap membantu kapan pun dibutuhkan`],
 (n)=>[`Apakah ada informasi terbaru terkait ${n}?`,`Ada, informasinya sudah kami siapkan`,`Terima kasih, sangat membantu pekerjaan kami`,`Dengan senang hati, jangan sungkan untuk berdiskusi lagi`]];
const fromMD=[
 (n,o)=>[`Selamat siang, kami ingin berkoordinasi mengenai ${o}`,`Selamat siang, silakan, kami siap berdiskusi`,`Kami ingin memastikan informasinya selaras dengan master data`,`Baik, terima kasih atas koordinasinya`],
 (n,o)=>[`Terima kasih atas kerja sama tim Anda`,`Terima kasih kembali, kami senang bisa berkontribusi`,`Kolaborasi ini sangat membantu kualitas data`,`Mari kita terus menjaga sinergi yang baik`],
 (n,o)=>[`Boleh kami berdiskusi sebentar mengenai ${o}?`,`Tentu, silakan`,`Kami ingin memahami kebutuhan tim Anda dengan lebih baik`,`Terima kasih, kami senang bisa berkolaborasi`]];
const cross=[
 (n,o)=>[`Selamat pagi, semoga pekerjaan hari ini berjalan lancar`,`Selamat pagi, terima kasih, semoga lancar juga untuk Anda`,`Jika ada yang bisa kami bantu terkait ${o}, kabari kami ya`,`Baik, terima kasih atas tawarannya`],
 (n,o)=>[`Boleh kami berdiskusi sebentar mengenai ${o}?`,`Tentu, silakan`,`Kami ingin memahami kebutuhan tim Anda dengan lebih baik`,`Terima kasih, kami senang bisa berkolaborasi`],
 (n,o)=>[`Terima kasih atas dukungan tim Anda`,`Sama-sama, senang bisa bekerja sama`,`Sinergi antar divisi sangat membantu kami`,`Mari kita terus saling mendukung`]];
const NOBODY=['Baik, kami kembali lagi nanti','Kami sampaikan lewat pesan saja, terima kasih','Kami berkunjung kembali di lain waktu'];
const SOLO={coffee:['Mengambil kopi sebentar','Waktunya jeda kopi','Menyeduh kopi terlebih dahulu'],chat:['Beristirahat sejenak','Menyegarkan pikiran','Berbincang ringan dengan rekan'],printer:['Mencetak dokumen','Menyiapkan dokumen untuk rapat','Memindai dokumen'],archive:['Memeriksa arsip data','Mengambil dokumen referensi','Memastikan arsip tertata rapi'],cooler:['Mengambil air minum','Mengisi botol minum','Menyegarkan diri sejenak'],wander:['Peregangan sebentar','Berjalan sebentar','Menyegarkan pikiran']};
const MEET=[
 ['Selamat pagi, terima kasih telah hadir','Hari ini kita membahas keselarasan data antar divisi','Dari sisi Master Data, kami siap mendukung seluruh divisi','Terima kasih atas masukan yang telah disampaikan','Mari kita tetapkan jadwal koordinasi berikutnya','Setuju, kami akan menyesuaikan jadwal tim','Terima kasih semuanya, pertemuan yang produktif'],
 ['Mari kita tinjau perkembangan kolaborasi antar divisi','Secara umum koordinasi berjalan dengan baik','Kami mengapresiasi kerja sama seluruh tim','Masukan dari setiap divisi sangat berharga','Mari kita lanjutkan dengan semangat yang sama','Terima kasih atas diskusi yang konstruktif'],
 ['Mari kita bahas rencana kerja bulan depan','Prioritas kami adalah menjaga kelancaran operasional','Kami siap menyesuaikan dengan kebutuhan setiap divisi','Komunikasi yang terbuka sangat membantu kita semua','Baik, kita sepakat untuk saling mendukung','Terima kasih, rapat kita akhiri dengan hasil yang baik']];
const wt=a=>pick(WT[a.d]).replace(/#/g,()=>20000+rnd(9999));
function convo(v,t){const n=NEED[v.d],o=OWN[t.d];return pick(t.d==='md'?toMD:v.d==='md'?fromMD:cross)(n,o)}
function talk(v,n){
  const t=byID[v.target];v.txt='';
  if(!t||t.act!=='work'||t.path.length||t.script.length){v.script=[{at:n,txt:pick(NOBODY)}];v.until=n+4500;return}
  const L=convo(v,t),end=n+L.length*3200+800;
  L.forEach((ln,i)=>(i%2?t:v).script.push({at:n+i*3200,txt:ln}));
  v.until=end;t.until=Math.max(t.until,end);t.dir='right';
}
const visitTxt=['Permisi, ada yang perlu dikonfirmasi'];
function pickNext(a){
  let r=rnd(100);if(a.act!=='work'&&r<70)r=0;
  let act='work',txt=wt(a),target='',dur=(8+rnd(14))*1000;
  if(r>=94){act='wander';txt='...';dur=2000}
  else if(r>=89){act='cooler';txt='ambil air minum';dur=4000}
  else if(r>=83){act='archive';txt='ambil master data';dur=6000}
  else if(r>=80){act='printer';txt='ngeprint laporan';dur=4000}
  else if(r>=62){act='visit';txt=pick(visitTxt);dur=6000;if(a.d!=='md'&&Math.random()<.6)target=pick(agents.filter(x=>x.d==='md')).id;else do{target=pick(agents).id}while(byID[target].d===a.d)}
  else if(r>=55){act='chat';txt='istirahat dulu';dur=8000}
  else if(r>=40){act='coffee';txt='bikin kopi';dur=5000}
  if(SOLO[act])txt=pick(SOLO[act]);let res=dest(a,act,target);
  if(!res){act='work';txt=wt(a);res=dest(a,'work','')}
  plan(a,act,res[0],res[1],target,dur,txt);
}
function meeting(){
  const who=[...agents].sort(()=>Math.random()-.5).slice(0,3+rnd(3)),st=[...seats].sort(()=>Math.random()-.5);
  if(!who.some(a=>a.d==='md'))who[0]=pick(agents.filter(a=>a.d==='md'));
  who.forEach((a,i)=>plan(a,'meeting',st[i],faceOf(st[i]),'',30000,''));
  const t0=now()+12000;pick(MEET).slice(0,6).forEach((ln,i)=>who[i%who.length].script.push({at:t0+i*3300,txt:ln}));
}
setInterval(()=>{
  const n=now();
  for(const a of agents){
    if(a.path.length){move(a);if(!a.path.length)arrive(a)}
    else if(n>a.until)pickNext(a);
    if(a.script.length&&!a.path.length&&n>=a.script[0].at){const q=a.script.shift();a.txt=q.txt;a.bub=n+3100}
    a.mv=a.path.length>0;a.t=(n<a.bub&&!a.mv)?a.txt:'';
  }
  if(n>nextMeet){meeting();nextMeet=n+(180+rnd(120))*1000}
},100);


let S=1,OX=0,OY=0;
function topInset(){if(cv.dataset.bg!=='transparent')return 0;const tb=document.querySelector('.topbar');if(!tb)return 0;return Math.round(tb.getBoundingClientRect().bottom+8)}
function fit(){const d=devicePixelRatio||1;cv.width=innerWidth*d;cv.height=innerHeight*d;
  const ti=topInset();
  let rw=innerWidth,rh=innerHeight-ti;
  if(innerWidth>760)rw=Math.max(280,innerWidth-Math.max(24,innerWidth*.05)-400-28);else rh=Math.max(160,innerHeight-12-430-10)-ti;
  const pad=innerWidth>760?24:8;S=Math.min((rw-pad)*d/(W*T),(rh-pad)*d/(H*T));
  OX=(rw*d-W*T*S)/2;OY=ti*d+(rh*d-H*T*S)/2}
addEventListener('resize',fit);fit();
document.fonts&&document.fonts.load('8px "Press Start 2P"');
const X=v=>Math.round(v*S+OX),Y=v=>Math.round(v*S+OY);
const R=(c,x,y,w,h)=>{g.fillStyle=c;const x0=X(x),y0=Y(y);g.fillRect(x0,y0,X(x+w)-x0,Y(y+h)-y0)};
const AL=(a,f)=>{g.globalAlpha=a;f();g.globalAlpha=1};
const hash=(x,y)=>((x*73856093^y*19349663)>>>0)%100;
const hx=(c,k)=>{const n=parseInt(c.slice(1),16),f=v=>Math.max(0,Math.min(255,Math.round(v*k)));return`rgb(${f(n>>16)},${f(n>>8&255)},${f(n&255)})`};
const TXT=(s,x,y,c,sz)=>{g.font=`${sz*S}px "Press Start 2P",monospace`;g.fillStyle=c;g.fillText(s,X(x),Y(y))};
const TW=(s,sz)=>{g.font=`${sz*S}px "Press Start 2P",monospace`;return g.measureText(s).width/S};
const skin=['#f1c27d','#e0ac69','#c68642','#8d5524'],hairC=['#2b1b17','#6b4226','#d9a441','#b5382e','#1d1d27'];

const FF='system-ui,-apple-system,"Segoe UI",sans-serif';
function pill(t,x,y,c,hub){const d=devicePixelRatio||1;g.font=`700 ${10.5*d}px ${FF}`;
 const dot=c&&!hub?9*d:0,w=g.measureText(t).width+14*d+dot,h=17*d,px=X(x),py=Y(y)-h+3*d;
 rp(px,py,w,h,h/2);g.fillStyle=hub?c:'rgba(9,9,11,.82)';g.fill();
 if(dot){g.fillStyle=c;g.beginPath();g.arc(px+9*d,py+h/2,3.2*d,0,7);g.fill()}
 g.fillStyle=hub?'#18181b':'#fafafa';g.textBaseline='middle';g.fillText(t,px+7*d+dot,py+h/2+.5*d);g.textBaseline='alphabetic'}
const placed=[];
const rr=(c,x,y,w,h)=>{[3,1,1].forEach((n,i)=>{R(c,x+n,y+i,w-2*n,1);R(c,x+n,y+h-1-i,w-2*n,1)});R(c,x,y+3,w,h-6)};
const ell=(c,cx,cy,rx,ry)=>{for(let j=-ry;j<ry;j++){const k=Math.round(rx*Math.sqrt(1-((j+.5)/ry)**2));R(c,cx-k,cy+j,k*2,1)}};
const rp=(x,y,w,h,r)=>{g.beginPath();g.roundRect?g.roundRect(x,y,w,h,r):g.rect(x,y,w,h)};
const at=(pts,s)=>{for(let i=1;i<pts.length;i++){const a=pts[i-1],b=pts[i],l=Math.abs(b[0]-a[0])+Math.abs(b[1]-a[1]);if(s<=l){const t=l?s/l:0;return[a[0]+(b[0]-a[0])*t,a[1]+(b[1]-a[1])*t]}s-=l}return pts[pts.length-1]};
function lights(ts){
  if(th())return;const d=devicePixelRatio||1;g.save();g.globalCompositeOperation='lighter';
  const L=[];divs.forEach(v=>L.push([(v.ox+2)*T,(v.oy+2.5)*T,v.id==='md'?[249,115,22]:[255,240,210],v.id==='md'?.2:.09,52]));
  L.push([17*T,18*T,[255,235,200],.12,70],[30*T,4*T,[255,240,210],.12,70],[30*T,17*T,[255,240,210],.1,60],[5*T,18*T,[110,170,255],.1,60]);
  L.forEach((l,i)=>{const cx=X(l[0]),cy=Y(l[1]),gr=g.createRadialGradient(cx,cy,0,cx,cy,l[4]*S),a=l[3]*(.9+.1*Math.sin(ts/700+i));
    gr.addColorStop(0,`rgba(${l[2]},${a})`);gr.addColorStop(1,`rgba(${l[2]},0)`);g.fillStyle=gr;g.fillRect(cx-l[4]*S,cy-l[4]*S,l[4]*S*2,l[4]*S*2)});
  g.restore();
}
const QL={wf:'#7b7ea8',wh:'#9a9dc6',wm:'#6a6d96',wb:'#3a3c5e',ac:'#ea580c',wl:'#2a2b45',wc:'#262840',w1:'#946443',w2:'#9c6c49',ws:'#7a5236',p1:'#e8f3f1',p2:'#c9e0dc',pl:'#b4cfca',s1:'#2b3040',s2:'#323849',sl:'#3d4560',l1:'#e6e2da',l2:'#dcd7cd',ll:'#cdc8bc',f1:'#cfc2a9',f2:'#c7baa0',fs:'#b9ac92',fg:'#bdb096',bg:'#f4f4f5',pn:'#ffffff',bd:'#e4e4e7',vg:.2};
const QD={wf:'#3f3f46',wh:'#52525b',wm:'#35353b',wb:'#27272a',ac:'#f97316',wl:'#121214',wc:'#1d1d21',w1:'#3b2f29',w2:'#41352e',ws:'#2a211c',p1:'#2b3438',p2:'#273034',pl:'#20282b',s1:'#17171b',s2:'#1c1c21',sl:'#2c2c34',l1:'#2e2e34',l2:'#2a2a30',ll:'#222228',f1:'#2a2a30',f2:'#25252a',fs:'#232328',fg:'#28282d',bg:'#09090b',pn:'#18181b',bd:'#27272a',vg:.4};
let Q=QD;const th=()=>{try{return(parent!==window?parent:window).document.documentElement.dataset.theme==='light'}catch(e){return false}};
function drawMap(ts){
  for(let y=0;y<H;y++)for(let x=0;x<W;x++){
    const px=x*T,py=y*T,h=hash(x,y);
    if(wall[y][x]){
      if(y+1<H&&!wall[y+1][x]){R(Q.wf,px,py,T,T-4);R(Q.wh,px,py,T,1);R(Q.wm,px,py+7,T,1);R(Q.wb,px,py+T-4,T,4);R(Q.ac,px,py+T-4,T,1);R(Q.wl,px,py+T-1,T,1)}
      else R(Q.wc,px,py,T,T);
    }else if(x>=25&&y<9){
      for(let k=0;k<4;k++){R((y*4+k)%2?Q.w1:Q.w2,px,py+k*4,T,3);R(Q.ws,px,py+k*4+3,T,1);R(Q.ws,px+(x*5+y*3+k*7)%14,py+k*4,1,3)}
    }else if(x>=25&&y>=13){R((x+y)&1?Q.p1:Q.p2,px,py,T,T);R(Q.pl,px,py,T,1);R(Q.pl,px,py,1,T)}
    else if(y>14&&x<9){R((x+y)&1?Q.s1:Q.s2,px,py,T,T);R(Q.sl,px,py,T,1);if(x%2===0)R(ts%900<450?'#4f8':'#284',px+7,py+7,1,1)}
    else if(y>14&&x<25){R((x+y)&1?Q.l1:Q.l2,px,py,T,T);R(Q.ll,px,py,T,1)}
    else{R((x+y)&1?Q.f1:Q.f2,px,py,T,T);if(h<16)R(Q.fs,px+(h*3)%13,py+(h*7)%13,2,1);R(Q.fg,px,py,T,1);R(Q.fg,px,py,1,T)}
  }
  for(const wx of [2,7,12,17,22]){
    R('#e8edf5',wx*T+1,2,2*T-2,T-7);R('#79c4f2',wx*T+3,4,2*T-6,T-11);R('#a8e0ff',wx*T+3,4,2*T-6,3);
    R('#fff',wx*T+6+(ts/900%8),6,6,2);R('#e8edf5',wx*T+T-1,2,2,T-7);
    g.beginPath();g.moveTo(X(wx*T+2),Y(T-4));g.lineTo(X(wx*T+2*T-2),Y(T-4));g.lineTo(X(wx*T+2*T+16),Y(T*7));g.lineTo(X(wx*T+16),Y(T*7));g.closePath();AL(.09,()=>{g.fillStyle='#fff7c2';g.fill()});
  }
  R('#111',27*T+8,3,64,9);R('#24467a',27*T+9,4,62,7);
  for(let i=0;i<6;i++){const v=2+hash(i,Math.floor(ts/900))%5;R('#7ef',27*T+14+i*9,10-v,5,v)}
  for(const d of divs){
    const x=d.ox*T,y=d.oy*T,w=4*T,h=4*T;
    AL(.34,()=>rr(d.color,x,y,w,h));
    const hub=d.id==='md';pill(d.name,x+2,y-3,d.color,hub);if(hub){rr(d.color,x-2,y-2,w+4,h+4);rr(hx(d.color,.35),x,y,w,h)}
  }
  for(const [t,x,y] of [['MEETING ROOM',26*T,7.4*T],['PANTRY',26*T,19.6*T],['DATA ARCHIVE',1*T,19.6*T],['LOBBY',11*T,19.6*T]]){pill(t,x+2,y-1,null)}
}

function drawObj(o,ts){
  const x=o.x*T,y=o.y*T,w=o.w*T,hh=o.h*T;
  if(o.kind!=='chair'&&o.kind!=='rug')AL(.22,()=>R('#000',x+2,y+hh-2,w,4));
  switch(o.kind){
    case'desk':{
      const ow=agents.find(a=>a.desk.x===o.x&&a.desk.y===o.y+1),busy=agents.some(a=>a.act==='work'&&!a.mv&&Math.round(a.dx)===o.x&&Math.round(a.dy)===o.y+1);
      R('#b07a45',x,y+3,w,10);R('#c98f58',x,y+3,w,1);R('#7e5230',x,y+13,w,3);R('#5e3c20',x,y+15,w,1);
      R('#1d1d2b',x+3,y-6,13,10);R('#444',x+8,y+4,3,2);
      if(busy){R('#0e2a3a',x+4,y-5,11,7);const k=Math.floor(ts/250);for(let i=0;i<4;i++){const l=2+hash(i+k,i)%7;R(['#7ef','#9f9','#fc7','#f9c'][(i+k)%4],x+5,y-4+i*1.6,l,1)}}
      else{R('#243447',x+4,y-5,11,7);R('#3b5878',x+6,y-3,5,1)}
      R('#d8d8e0',x+4,y+8,11,3);R('#9a9aa8',x+4,y+10,11,1);
      R('#fff',x+22,y+5,4,4);R('#6b3f1d',x+22,y+5,4,1);R('#d8d8e0',x+17,y+6,4,5);
      if(ow)R(col[ow.d],x+18,y+14,8,1);break}
    case'rack':R('#161a28',x+1,y-2,14,hh+2);R('#232a40',x+1,y-2,14,2);for(let i=0;i<5;i++){R('#2d3550',x+3,y+i*5,10,3);R((ts/300+i+o.x)%3<1?'#f55':'#4f8',x+11,y+1+i*5,1,1);R('#6af',x+4,y+1+i*5,3,1)}break;
    case'sofa':R('#6b4a8c',x,y+2,w,12);R('#84609f',x+1,y+4,w-2,7);R('#55396f',x,y+2,w,2);R('#55396f',x,y+2,2,12);R('#55396f',x+w-2,y+2,2,12);break;
    case'cabinet':R('#8b95a3',x+1,y-6,14,T+6);R('#a9b3bd',x+1,y-6,14,2);for(let i=0;i<3;i++){R('#6f7a88',x+3,y-3+i*6,10,1);R('#4a525e',x+7,y-1+i*6,2,1)}break;
    case'panel':R('#58628a',x+5,y+2,6,hh-3);R('#7b86b0',x+5,y+2,6,2);R('#3d4468',x+5,y+hh-1,6,1);break;
    case'reception':R('#4a3a2a',x,y+2,w,14);R('#c9a46a',x,y+2,w,5);R('#fff',x+8,y-1,10,5);R('#d8d8e0',x+w-20,y+1,10,4);R(col.md,x+w/2-6,y+9,12,3);break;
    case'cooler':R('#cfd8e3',x+3,y-4,10,T+4);R('#7ec8ff',x+4,y-9,8,6);R('#a9b3bd',x+3,y+4,10,1);R('#4d96ff',x+7,y+6,2,2);break;
    case'rug':if(o.h===1){R('#7a3b3b',x+1,y+3,w-2,10);R('#e6c27a',x+3,y+7,w-6,2)}else{AL(.85,()=>{ell('#4b4f8a',x+w/2,y+hh/2,w/2,hh/2);ell('#5c61a8',x+w/2,y+hh/2,w/2-3,hh/2-3)})}break;
    case'chair':R('#2c2f48',o.x*T+2,y+6,12,3);R('#3d4268',o.x*T+3,y+4,10,8);R('#56608f',o.x*T+3,y+4,10,1);R('#2c2f48',o.x*T+2,y+12,12,2);break;
    case'table':
      if(o.w>1){ell('#5a3a1c',x+w/2,y+hh/2+1,w/2,hh/2);ell('#7a5230',x+w/2,y+hh/2-1,w/2-2,hh/2-3);
        for(const lx of[10,40]){R('#cfd3df',x+lx,y+6,10,6);R('#2a2d3f',x+lx+1,y+7,8,4)}R('#fff',x+26,y+8,6,4);R('#fff',x+52,y+20,4,4)}
      else{ell('#8b5a5a',x+8,y+9,7,5);ell('#c99',x+8,y+8,6,3)}break;
    case'counter':R('#8b95a3',x,y+4,w,12);R('#dfe3ea',x,y+2,w,4);R('#2f3240',x+6,y-8,11,12);R('#58607a',x+7,y-7,9,3);R(ts%800<400?'#6f6':'#3a3',x+8,y-3,2,1);R('#fff',x+11,y+1,3,3);
      for(let i=0;i<3;i++)AL(.55-i*.15,()=>R('#fff',x+11+Math.sin(ts/300+i)*1.5,y-4-((ts/120+i*4)%10),1,2));
      R('#fff',x+30,y,4,4);R('#fff',x+38,y,4,4);break;
    case'fridge':R('#f0f4f7',x+1,y-8,14,T+8);R('#a9b3bd',x+1,y,14,1);R('#9aa5b1',x+11,y-4,1,5);R('#ee5253',x+4,y-5,2,2);R('#4d96ff',x+7,y-3,2,2);break;
    case'printer':R('#8a8ea0',x+1,y+6,14,8);R('#e5e7ee',x+1,y+3,14,6);R('#fff',x+4,y,8,4);R(ts%1000<500?'#6f6':'#383',x+12,y+4,2,1);break;
    case'plant':{const s=Math.sin(ts/700)*.6;R('#7a3f20',x+4,y+10,8,6);R('#a0522d',x+4,y+10,8,2);R('#1f6b45',x+2+s,y+3,12,8);R('#2e8b57',x+3+s,y+1,10,7);R('#5bd08a',x+5+s,y,5,4);R('#5bd08a',x+9+s,y+4,3,3);break}
  }
}

function sprite(a,bx,by,f,sit,mono,ts){
  const C=c=>mono?'#1b1830':c,i=a.i,sk=skin[i%4],hr=hairC[i%5],sh=col[a.d],shd=hx(sh,.7),dir=a.dir;
  const typing=a.act==='work'&&sit,k=Math.floor(ts/110)%2;
  if(!sit){R(C('#2f3252'),bx+2,by+11,3,4-f);R(C('#2f3252'),bx+5,by+11,3,3+f);R(C('#16151f'),bx+2,by+15-f,3,1);R(C('#16151f'),bx+5,by+14+f,3,1)}
  R(C(sh),bx+1,by+7,8,5);R(C(shd),bx+1,by+10,8,2);
  const la=a.mv?f:0,ra=a.mv?1-f:0;
  if(typing){R(C(sh),bx,by+7,2,3);R(C(sh),bx+8,by+7,2,3);R(C(sk),bx+(k?0:1),by+6,2,2);R(C(sk),bx+(k?7:8),by+6,2,2)}
  else{R(C(sh),bx,by+7+la,2,4);R(C(sh),bx+8,by+7+ra,2,4);R(C(sk),bx,by+11+la,2,1);R(C(sk),bx+8,by+11+ra,2,1)}
  R(C(sk),bx+2,by+1,6,6);
  if(dir==='up')R(C(hr),bx+1,by,8,6);
  else{R(C(hr),bx+2,by,6,3);R(C(hr),bx+1,by+1,1,3);R(C(hr),bx+8,by+1,1,3);
    if(dir==='down'){R(C('#111'),bx+3,by+4,1,2);R(C('#111'),bx+6,by+4,1,2);R(C('#fff'),bx+4,by+8,2,2);0}
    else if(dir==='left'){R(C(hr),bx+6,by,2,5);R(C('#111'),bx+3,by+4,1,2)}
    else{R(C(hr),bx+2,by,2,5);R(C('#111'),bx+6,by+4,1,2)}}
}
function drawAgent(a,ts){
  const f=a.mv?Math.floor(ts/150)%2:0,sit=!a.mv&&(a.act==='work'||a.act==='meeting');
  const bx=a.dx*T+3,by=(a.dy+1)*T-16+(sit?3:0)-(a.mv&&f?1:0);
  AL(.28,()=>R('#000',bx,by+(sit?10:15),10,2));
  for(const[dx,dy]of[[-1,0],[1,0],[0,-1],[0,1]])sprite(a,bx+dx,by+dy,f,sit,true,ts);
  sprite(a,bx,by,f,sit,false,ts);a._by=by;a._bx=bx;
}
const wrap=(s,n)=>{const o=[];let c='';for(const w of s.split(' ')){if((c+' '+w).trim().length>n){o.push(c);c=w}else c=(c+' '+w).trim()}o.push(c);return o};
function label(a){
  const d=devicePixelRatio||1,cx=X(a._bx+5),top=Y(a._by),lo=X(0)+6*d,hi=X(W*T)-6*d;
  if(a.t){
    g.font=`600 ${10.5*d}px ${FF}`;
    const maxW=150*d,ls=[];let c='';
    for(const w of a.t.split(' ')){const t=c?c+' '+w:w;if(c&&g.measureText(t).width>maxW){ls.push(c);c=w}else c=t}ls.push(c);
    const tw=Math.max(...ls.map(l=>g.measureText(l).width)),px=8*d,lh=14*d,bw=tw+px*2,bh=ls.length*lh+8*d;
    const x=Math.min(Math.max(cx-bw/2,lo),hi-bw),y=top-7*d-bh;
    g.save();g.shadowColor='rgba(0,0,0,.35)';g.shadowBlur=8*d;g.shadowOffsetY=2*d;
    rp(x,y,bw,bh,8*d);g.fillStyle='#fafafa';g.fill();
    g.beginPath();g.moveTo(cx-4.5*d,y+bh-1);g.lineTo(cx,y+bh+5*d);g.lineTo(cx+4.5*d,y+bh-1);g.fill();g.restore();
    g.fillStyle='#18181b';g.textBaseline='middle';ls.forEach((l,i)=>g.fillText(l,x+px,y+4*d+lh*(i+.5)));g.textBaseline='alphabetic';
  }else{
    g.font=`600 ${8.5*d}px ${FF}`;const dot=7*d,w=g.measureText(a.n).width+11*d+dot,h=13*d,x=cx-w/2;let y=top-5*d-h;
    for(let i=0;i<5&&placed.some(r=>x<r.x+r.w&&x+w>r.x&&y<r.y+r.h&&y+h>r.y);i++)y-=h+2*d;
    placed.push({x,y,w,h});
    AL(.88,()=>{rp(x,y,w,h,h/2);g.fillStyle='#09090b';g.fill()});
    g.fillStyle=col[a.d];g.beginPath();g.arc(x+8*d,y+h/2,2.8*d,0,7);g.fill();
    g.fillStyle='#fafafa';g.textBaseline='middle';g.fillText(a.n,x+6*d+dot,y+h/2+.5*d);g.textBaseline='alphabetic';
  }
}
function frame(ts){
  g.setTransform(1,0,0,1,0,0);Q=th()?QL:QD;
  const isLogin=cv.dataset.bg==='transparent';
  if(isLogin){g.clearRect(0,0,cv.width,cv.height)}else{g.fillStyle=Q.bg;g.fillRect(0,0,cv.width,cv.height)}
  const rx=X(0),ry=Y(0),rw=X(W*T)-rx,rh=Y(H*T)-ry,dd=devicePixelRatio||1;
  g.save();g.shadowColor='rgba(0,0,0,.55)';g.shadowBlur=36*dd;g.shadowOffsetY=10*dd;rp(rx,ry,rw,rh,16*dd);g.fillStyle=Q.pn;g.fill();g.restore();
  g.save();rp(rx,ry,rw,rh,16*dd);g.clip();
  drawMap(ts);
  agents.forEach(a=>{a.dx+=(a.x-a.dx)*.3;a.dy+=(a.y-a.dy)*.3});
  const items=[...objs.map(o=>({y:o.kind==='rug'?-9:o.kind==='desk'||o.kind==='chair'?o.y-.5+(o.kind==='chair'?.3:0):o.y+o.h-1,o})),...agents.map(a=>({y:a.dy,a}))].sort((p,q)=>p.y-q.y);
  for(const it of items)it.o?drawObj(it.o,ts):drawAgent(it.a,ts);
  lights(ts);placed.length=0;
  [...agents].sort((p,q)=>!!p.t-!!q.t).forEach(label);
  const gr=g.createRadialGradient(rx+rw/2,ry+rh/2,rh*.35,rx+rw/2,ry+rh/2,rw*.7);
  gr.addColorStop(0,'rgba(10,10,30,0)');gr.addColorStop(1,'rgba(10,10,30,'+Q.vg+')');g.fillStyle=gr;g.fillRect(rx,ry,rw,rh);
  g.restore();rp(rx,ry,rw,rh,16*dd);g.strokeStyle=Q.bd;g.lineWidth=2*dd;g.stroke();
  requestAnimationFrame(frame);
}
requestAnimationFrame(frame);

})();
