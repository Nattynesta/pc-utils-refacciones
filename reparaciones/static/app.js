function cambiarStatus(id, status) {
  const fd = new FormData();
  fd.append('status', status);
  fetch('/reparacion/'+id+'/status', {method:'POST', body:fd})
  .then(r=>r.json()).then(d=>{
    if(d.success) location.reload();
    else toast(d.error||'Error','danger');
  }).catch(()=>toast('Error de conexión','danger'));
}

function toast(msg, type) {
  const c = document.getElementById('toast');
  if(!c) return;
  const el = document.createElement('div');
  el.className = 'toast-msg toast-'+type;
  el.textContent = msg;
  c.appendChild(el);
  setTimeout(()=>el.remove(), 3000);
}
