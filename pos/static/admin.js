// ─── Toast ───
function showToast(msg, type) {
  type = type || 'info';
  const icons = {success:'bi-check-circle-fill text-success', error:'bi-x-circle-fill text-danger', info:'bi-info-circle-fill text-primary'};
  const c = document.querySelector('.admin-toast-container') || (() => {
    const el = document.createElement('div'); el.className = 'admin-toast-container'; document.body.appendChild(el); return el;
  })();
  const t = document.createElement('div');
  t.className = 'admin-toast ' + type;
  t.innerHTML = `<i class="bi ${icons[type]||icons.info}"></i><span>${msg}</span>`;
  c.appendChild(t);
  setTimeout(() => { t.style.opacity = '0'; t.style.transition = 'opacity 0.3s'; setTimeout(() => t.remove(), 300); }, 3000);
}

function eliminarPieza(id,tok){
    if(!confirm('¿Eliminar esta pieza?')) return;
    fetch('/admin/piezas/'+id,{method:'DELETE',headers:{'X-CSRF-Token':tok}})
    .then(r=>r.json()).then(d=>{if(d.success) location.reload(); else alert(d.error)});
}
function tableFilter(inputId, tableId) {
    const input = document.getElementById(inputId);
    if(!input) return;
    input.addEventListener('keyup', function(){
        const q = this.value.toLowerCase();
        document.querySelectorAll('#'+tableId+' tbody tr').forEach(r => {
            r.style.display = r.textContent.toLowerCase().includes(q) ? '' : 'none';
        });
    });
}
function exportCSV(tableId, filename) {
    const rows = document.querySelectorAll('#'+tableId+' tr');
    let csv = [];
    rows.forEach(r => {
        const cols = r.querySelectorAll('td,th');
        csv.push(Array.from(cols).map(c => '"'+c.textContent.trim().replace(/"/g,'""')+'"').join(','));
    });
    const blob = new Blob([csv.join('\n')], {type:'text/csv'});
    const a = document.createElement('a');
    a.href = URL.createObjectURL(blob);
    a.download = filename;
    a.click();
}
