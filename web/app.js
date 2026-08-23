const out=document.querySelector('#out');
async function api(path,opts){const r=await fetch('/api/v1'+path,{headers:{'Content-Type':'application/json'},...opts});const v=await r.json();if(!r.ok)throw new Error(v.error?.message||'request failed');return v}
async function loadStats(){try{out.textContent=JSON.stringify(await api('/stats'),null,2)}catch(e){out.textContent=e.message}}
async function verify(){try{out.textContent=JSON.stringify(await api('/verify',{method:'POST',body:JSON.stringify({mode:'full'})}),null,2)}catch(e){out.textContent=e.message}}
async function appendEntry(){try{out.textContent=JSON.stringify(await api('/entries',{method:'POST',body:JSON.stringify({actor:actor.value,action:action.value,target:target.value,detail:{}})}),null,2);await loadStats()}catch(e){out.textContent=e.message}}
loadStats();
