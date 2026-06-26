// Admin JS
// eslint-disable-next-line no-unused-vars
function eliminarPieza(id){
    if(!confirm('¿Eliminar esta pieza?')) return;
    fetch('/admin/piezas/'+id, {method:'DELETE', headers:{'X-CSRF-Token': document.querySelector('[name=_csrf]')?.value || ''}})
    .then(r=>r.json()).then(d=>{if(d.success) location.reload();else alert(d.error);});
}
