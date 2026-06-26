// ─── Toast System ───
function showToast(msg, type) {
    type = type || 'info';
    const icons = {success:'bi-check-circle-fill text-success', error:'bi-x-circle-fill text-danger', info:'bi-info-circle-fill text-primary'};
    const c = document.getElementById('toastContainer') || (()=>{
        const el = document.createElement('div'); el.id='toastContainer';
        el.className='toast-container'; document.body.appendChild(el); return el;
    })();
    const t = document.createElement('div');
    t.className = 'toast-custom ' + type + ' animate-fade-up';
    t.innerHTML = `<i class="bi ${icons[type]||icons.info}"></i><span>${msg}</span>`;
    c.appendChild(t);
    setTimeout(()=>{ t.style.opacity='0'; t.style.transition='opacity 0.3s'; setTimeout(()=>t.remove(),300); }, 3000);
}

// ─── Search Autocomplete ───
document.addEventListener('DOMContentLoaded', function(){
    const searchInput = document.getElementById('searchInput');
    const acDropdown = document.getElementById('acDropdown');
    if(!searchInput || !acDropdown) return;

    let acTimer;
    searchInput.addEventListener('input', function(){
        clearTimeout(acTimer);
        const q = this.value.trim();
        if(q.length < 2) { acDropdown.style.display = 'none'; return; }
        acTimer = setTimeout(() => {
            fetch('/api/piezas?s=' + encodeURIComponent(q))
            .then(r => r.json())
            .then(data => {
                if(!data.length) { acDropdown.style.display = 'none'; return; }
                acDropdown.innerHTML = data.slice(0,8).map(p =>
                    `<a href="/pieza/${p.id}" class="autocomplete-item text-decoration-none text-dark">
                        ${p.imagen_url ? `<img src="${p.imagen_url}" alt="">` : `<div style="width:36px;height:36px;background:#f0f0f0;border-radius:6px;display:flex;align-items:center;justify-content:center"><i class="bi bi-image text-muted small"></i></div>`}
                        <div><div class="small fw-semibold">${p.nombre}</div><small class="text-muted">${p.codigo}</small></div>
                        <div class="ac-price">$${parseFloat(p.precio).toFixed(2)}</div>
                    </a>`
                ).join('');
                acDropdown.style.display = 'block';
            });
        }, 300);
    });
    document.addEventListener('click', e => {
        if(!e.target.closest('#searchInput')) acDropdown.style.display = 'none';
    });

    // Load dropdowns
    fetch('/api/categorias').then(r=>r.json()).then(data=>{
        const dd = document.getElementById('categoriasDropdown');
        if(!dd) return;
        data.forEach(c => {
            const li = document.createElement('li');
            li.innerHTML = `<a class="dropdown-item" href="/buscar?categoria=${c.slug}"><i class="bi bi-${c.icono} me-2"></i>${c.nombre}</a>`;
            dd.appendChild(li);
        });
    });
    fetch('/api/marcas').then(r=>r.json()).then(data=>{
        const dd = document.getElementById('marcasDropdown');
        if(!dd) return;
        data.forEach(m => {
            const li = document.createElement('li');
            li.innerHTML = `<a class="dropdown-item" href="/buscar?marca=${encodeURIComponent(m.nombre)}">${m.nombre}</a>`;
            dd.appendChild(li);
        });
    });
});

// ─── WhatsApp ───
function whatsappPart(nombre, codigo, precio) {
    const msg = encodeURIComponent(`Hola, me interesa la pieza: ${nombre} (${codigo}) - $${precio}`);
    window.open(`https://wa.me/521${PHONE || ''}?text=${msg}`, '_blank');
}

// ─── Cart / Quote (simple) ───
let quoteItems = JSON.parse(localStorage.getItem('refacciones_quote') || '[]');
function updateQuoteBadge() {
    const b = document.getElementById('quoteBadge');
    if(b) { b.textContent = quoteItems.length; b.style.display = quoteItems.length ? '' : 'none'; }
}
function addToQuote(id, nombre, precio, img) {
    if(quoteItems.find(i => i.id === id)) { showToast('Ya tienes esta pieza en tu cotización', 'info'); return; }
    quoteItems.push({id, nombre, precio, img, cant: 1});
    localStorage.setItem('refacciones_quote', JSON.stringify(quoteItems));
    updateQuoteBadge();
    showToast(`${nombre} agregada a cotización`, 'success');
}
function removeFromQuote(id) {
    quoteItems = quoteItems.filter(i => i.id !== id);
    localStorage.setItem('refacciones_quote', JSON.stringify(quoteItems));
    updateQuoteBadge();
    renderQuoteModal();
}
function renderQuoteModal() {
    const tbody = document.getElementById('quoteTableBody');
    if(!tbody) return;
    if(!quoteItems.length) {
        tbody.innerHTML = '<tr><td colspan="5" class="text-center py-4 text-muted">No hay piezas en tu cotización</td></tr>';
        document.getElementById('quoteTotal').textContent = '$0.00';
        return;
    }
    let total = 0;
    tbody.innerHTML = quoteItems.map((item,i) => {
        const sub = item.precio * item.cant;
        total += sub;
        return `<tr>
            <td>${item.nombre}</td>
            <td>$${parseFloat(item.precio).toFixed(2)}</td>
            <td><input type="number" class="form-control form-control-sm" style="width:60px" value="${item.cant}" min="1" onchange="quoteItems[${i}].cant=parseInt(this.value);localStorage.setItem('refacciones_quote',JSON.stringify(quoteItems));renderQuoteModal()"></td>
            <td>$${sub.toFixed(2)}</td>
            <td><button class="btn btn-sm btn-outline-danger" onclick="removeFromQuote(${item.id})"><i class="bi bi-trash"></i></button></td>
        </tr>`;
    }).join('');
    document.getElementById('quoteTotal').textContent = '$' + total.toFixed(2);
}
function sendQuoteWhatsApp() {
    if(!quoteItems.length) return;
    const lines = quoteItems.map(i => `• ${i.nombre} x${i.cant} = $${(i.precio*i.cant).toFixed(2)}`);
    const total = quoteItems.reduce((s,i) => s + i.precio * i.cant, 0);
    const msg = encodeURIComponent(`Hola, quiero cotizar:\n${lines.join('\n')}\n\nTotal: $${total.toFixed(2)}`);
    window.open(`https://wa.me/521${PHONE || ''}?text=${msg}`, '_blank');
}
updateQuoteBadge();
