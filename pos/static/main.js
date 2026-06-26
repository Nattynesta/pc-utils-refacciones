// Portal JS
document.addEventListener('DOMContentLoaded', function() {
    // Load dropdowns via API
    fetch('/api/categorias').then(r=>r.json()).then(data=>{
        let dd = document.getElementById('categoriasDropdown');
        if(!dd) return;
        data.forEach(c=>{
            let li = document.createElement('li');
            li.innerHTML = `<a class="dropdown-item" href="/buscar?categoria=${c.slug}"><i class="bi bi-${c.icono}"></i> ${c.nombre}</a>`;
            dd.appendChild(li);
        });
    });
    fetch('/api/marcas').then(r=>r.json()).then(data=>{
        let dd = document.getElementById('marcasDropdown');
        if(!dd) return;
        data.forEach(m=>{
            let li = document.createElement('li');
            li.innerHTML = `<a class="dropdown-item" href="/buscar?marca=${encodeURIComponent(m.nombre)}">${m.nombre}</a>`;
            dd.appendChild(li);
        });
    });
});
