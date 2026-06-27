package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// ─── Auth Handlers ────────────────────────────────────────────────

func handleLoginPage(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "admin/login.html", PageData{Title: "Login"})
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	usuario := r.FormValue("usuario")
	clave := r.FormValue("clave")

	var id int
	var username, rol, hash string
	err := db.QueryRow("SELECT id, usuario, rol, clave FROM usuarios WHERE usuario = ? AND activo = 't'", usuario).Scan(&id, &username, &rol, &hash)
	if err != nil || !verifyPassword(clave, hash) {
		renderTemplate(w, "admin/login.html", PageData{Title: "Login", Error: "Usuario o contraseña incorrectos"})
		return
	}

	sessionID := generateCSRFToken()[:16]
	db.Exec("INSERT INTO sessions (id, user_id, expires_at) VALUES (?, ?, datetime('now','+24 hours'))", sessionID, id)
	http.SetCookie(w, &http.Cookie{Name: "session", Value: sessionID, Path: "/", HttpOnly: true, MaxAge: 86400})
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func handleLogout(w http.ResponseWriter, r *http.Request) {
	c, _ := r.Cookie("session")
	if c != nil {
		db.Exec("DELETE FROM sessions WHERE id = ?", c.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: "session", Value: "", Path: "/", MaxAge: -1})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// ─── Portal Handlers ──────────────────────────────────────────────

func handlePortalHome(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	cats := queryCategorias()
	marcas := queryMarcas()

	rows, _ := db.Query("SELECT p.id, p.nombre, p.precio, p.stock, p.imagen_url, c.nombre FROM piezas p JOIN categorias c ON c.id = p.categoria_id WHERE p.activa = 1 ORDER BY p.created_at DESC LIMIT 8")
	var recientes []PiezaResumen
	for rows.Next() {
		var p PiezaResumen
		rows.Scan(&p.ID, &p.Nombre, &p.Precio, &p.Stock, &p.ImagenURL, &p.CategoriaNombre)
		recientes = append(recientes, p)
	}
	rows.Close()

	theme := getConfig("tema")
	if theme == "" { theme = "light" }

	renderTemplate(w, "portal/home.html", PageData{
		Title: "RefacCel — Refacciones de Celular",
		Categorias: cats, Marcas: marcas, Recientes: recientes, Theme: theme,
		Telefono: getConfig("telefono"), Negocio: getConfig("negocio_nombre"),
	})
}

func handlePortalBuscar(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	catSlug := r.URL.Query().Get("categoria")
	marcaNombre := r.URL.Query().Get("marca")
	precioMax := r.URL.Query().Get("precio_max")
	estado := r.URL.Query().Get("estado")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 { page = 1 }
	limit := 24
	offset := (page - 1) * limit

	where := []string{"p.activa = 1"}
	args := []any{}
	if q != "" {
		where = append(where, "(p.nombre LIKE ? OR p.codigo LIKE ? OR p.descripcion LIKE ?)")
		like := "%" + q + "%"
		args = append(args, like, like, like)
	}
	if catSlug != "" {
		where = append(where, "c.slug = ?")
		args = append(args, catSlug)
	}
	if marcaNombre != "" {
		where = append(where, "m.nombre = ?")
		args = append(args, marcaNombre)
	}
	if precioMax != "" {
		pm, _ := strconv.ParseFloat(precioMax, 64)
		if pm > 0 {
			where = append(where, "p.precio <= ?")
			args = append(args, pm)
		}
	}
	if estado != "" {
		where = append(where, "p.estado = ?")
		args = append(args, estado)
	}

	whereClause := strings.Join(where, " AND ")

	var total int
	db.QueryRow("SELECT COUNT(*) FROM piezas p JOIN categorias c ON c.id = p.categoria_id LEFT JOIN compatibilidad cp ON cp.pieza_id = p.id LEFT JOIN modelos mo ON mo.id = cp.modelo_id LEFT JOIN marcas m ON m.id = mo.marca_id WHERE "+whereClause, args...).Scan(&total)

	sql := "SELECT DISTINCT p.id, p.nombre, p.precio, p.stock, p.imagen_url, c.nombre FROM piezas p JOIN categorias c ON c.id = p.categoria_id LEFT JOIN compatibilidad cp ON cp.pieza_id = p.id LEFT JOIN modelos mo ON mo.id = cp.modelo_id LEFT JOIN marcas m ON m.id = mo.marca_id WHERE " + whereClause + " ORDER BY p.created_at DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)
	rows, _ := db.Query(sql, args...)
	var piezas []PiezaResumen
	for rows.Next() {
		var p PiezaResumen
		rows.Scan(&p.ID, &p.Nombre, &p.Precio, &p.Stock, &p.ImagenURL, &p.CategoriaNombre)
		piezas = append(piezas, p)
	}
	rows.Close()

	totalPages := (total + limit - 1) / limit
	theme := getConfig("tema")

	renderTemplate(w, "portal/buscar.html", PageData{
		Title: "Buscar", Query: q, Theme: theme,
		Piezas: piezas, Total: total, Page: page, TotalPages: totalPages,
		Categorias: queryCategorias(), Marcas: queryMarcas(),
		CategoriaSlug: catSlug, MarcaNombre: marcaNombre,
		PrecioMax: precioMax, Estado: estado,
		Telefono: getConfig("telefono"), Negocio: getConfig("negocio_nombre"),
	})
}

func handlePortalModelo(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("modelo")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	var mo Modelo
	err = db.QueryRow("SELECT mo.id, mo.nombre, mo.año_lanzamiento, m.nombre FROM modelos mo JOIN marcas m ON m.id = mo.marca_id WHERE mo.id = ?", id).Scan(&mo.ID, &mo.Nombre, &mo.AñoLanzamiento, &mo.MarcaNombre)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	rows, _ := db.Query("SELECT p.id, p.nombre, p.precio, p.stock, p.imagen_url, c.nombre FROM piezas p JOIN categorias c ON c.id = p.categoria_id JOIN compatibilidad cp ON cp.pieza_id = p.id WHERE cp.modelo_id = ? AND p.activa = 1 ORDER BY p.nombre", id)
	var piezas []PiezaResumen
	for rows.Next() {
		var p PiezaResumen
		rows.Scan(&p.ID, &p.Nombre, &p.Precio, &p.Stock, &p.ImagenURL, &p.CategoriaNombre)
		piezas = append(piezas, p)
	}
	rows.Close()

	theme := getConfig("tema")
	renderTemplate(w, "portal/modelo.html", PageData{
		Title: mo.Nombre, Theme: theme,
		ModeloNombre: mo.Nombre, MarcaNombre: mo.MarcaNombre,
		AñoLanzamiento: mo.AñoLanzamiento, Piezas: piezas,
		Telefono: getConfig("telefono"), Negocio: getConfig("negocio_nombre"),
	})
}

func handlePortalPieza(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	var p Pieza
	err = db.QueryRow("SELECT p.id, p.nombre, p.codigo, p.descripcion, p.precio, p.costo, p.stock, p.stock_minimo, p.imagen_url, p.imagenes_adicionales, p.estado, p.garantia_dias, p.proveedor, p.ubicacion, c.nombre, c.slug FROM piezas p JOIN categorias c ON c.id = p.categoria_id WHERE p.id = ? AND p.activa = 1", id).Scan(&p.ID, &p.Nombre, &p.Codigo, &p.Descripcion, &p.Precio, &p.Costo, &p.Stock, &p.StockMinimo, &p.ImagenURL, &p.ImagenesAdicionales, &p.Estado, &p.GarantiaDias, &p.Proveedor, &p.Ubicacion, &p.CategoriaNombre, &p.CategoriaSlug)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	rows, _ := db.Query("SELECT cp.modelo_id, mo.nombre, m.nombre FROM compatibilidad cp JOIN modelos mo ON mo.id = cp.modelo_id JOIN marcas m ON m.id = mo.marca_id WHERE cp.pieza_id = ?", id)
	for rows.Next() {
		var c Compatible
		rows.Scan(&c.ModeloID, &c.ModeloNombre, &c.MarcaNombre)
		p.Compatibles = append(p.Compatibles, c)
	}
	rows.Close()

	theme := getConfig("tema")
	renderTemplate(w, "portal/pieza.html", PageData{Title: p.Nombre, Pieza: p, Theme: theme,
		Telefono: getConfig("telefono"), Negocio: getConfig("negocio_nombre"),
	})
}

// ─── API Handlers ─────────────────────────────────────────────────

func handleAPIPiezas(w http.ResponseWriter, r *http.Request) {
	s := r.URL.Query().Get("s")
	var rows *sql.Rows
	if s != "" {
		like := "%" + s + "%"
		rows, _ = db.Query("SELECT p.id, p.nombre, p.precio, p.stock, p.codigo, COALESCE(p.imagen_url,'') FROM piezas p WHERE p.activa = 1 AND (p.nombre LIKE ? OR p.codigo LIKE ? OR p.descripcion LIKE ?) ORDER BY p.nombre LIMIT 20", like, like, like)
	} else {
		rows, _ = db.Query("SELECT p.id, p.nombre, p.precio, p.stock, p.codigo, COALESCE(p.imagen_url,'') FROM piezas p WHERE p.activa = 1 ORDER BY p.nombre LIMIT 50")
	}
	var items []map[string]any
	for rows.Next() {
		var id, stock int
		var nombre, codigo, img string
		var precio float64
		rows.Scan(&id, &nombre, &precio, &stock, &codigo, &img)
		items = append(items, map[string]any{"id": id, "nombre": nombre, "precio": precio, "stock": stock, "codigo": codigo, "imagen_url": img})
	}
	rows.Close()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(items)
}

func handleAPIModelos(w http.ResponseWriter, r *http.Request) {
	rows, _ := db.Query("SELECT mo.id, mo.nombre, m.nombre FROM modelos mo JOIN marcas m ON m.id = mo.marca_id WHERE mo.activo = 1 ORDER BY m.nombre, mo.nombre")
	var items []map[string]any
	for rows.Next() {
		var id int
		var nombre, marca string
		rows.Scan(&id, &nombre, &marca)
		items = append(items, map[string]any{"id": id, "nombre": nombre, "marca": marca})
	}
	rows.Close()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(items)
}

func handleAPIMarcas(w http.ResponseWriter, r *http.Request) {
	rows, _ := db.Query("SELECT nombre FROM marcas WHERE activa = 1 ORDER BY nombre")
	var items []map[string]string
	for rows.Next() {
		var nombre string
		rows.Scan(&nombre)
		items = append(items, map[string]string{"nombre": nombre})
	}
	rows.Close()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(items)
}

func handleAPICategorias(w http.ResponseWriter, r *http.Request) {
	rows, _ := db.Query("SELECT nombre, slug, icono FROM categorias WHERE activa = 1 ORDER BY orden")
	var items []map[string]string
	for rows.Next() {
		var nombre, slug, icono string
		rows.Scan(&nombre, &slug, &icono)
		items = append(items, map[string]string{"nombre": nombre, "slug": slug, "icono": icono})
	}
	rows.Close()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(items)
}

// ─── Admin Handlers ───────────────────────────────────────────────

func adminPD(r *http.Request) PageData {
	s := r.Context().Value("session").(*SessionData)
	return PageData{User: s.Username, Role: s.Role}
}

func handleAdminDashboard(w http.ResponseWriter, r *http.Request) {
	var stats DashboardStats
	db.QueryRow("SELECT COUNT(*) FROM piezas").Scan(&stats.TotalPiezas)
	db.QueryRow("SELECT COUNT(*) FROM modelos WHERE activo = 1").Scan(&stats.TotalModelos)
	db.QueryRow("SELECT COUNT(*) FROM marcas WHERE activa = 1").Scan(&stats.TotalMarcas)
	db.QueryRow("SELECT COUNT(*) FROM piezas WHERE stock <= stock_minimo AND stock > 0").Scan(&stats.StockBajo)

	rows, _ := db.Query("SELECT id, nombre, codigo, precio, stock FROM piezas ORDER BY created_at DESC LIMIT 5")
	for rows.Next() {
		var p PiezaResumen
		rows.Scan(&p.ID, &p.Nombre, &p.Codigo, &p.Precio, &p.Stock)
		stats.UltimasPiezas = append(stats.UltimasPiezas, p)
	}
	rows.Close()

	rows, _ = db.Query("SELECT id, nombre, stock, stock_minimo FROM piezas WHERE stock <= stock_minimo AND stock > 0 ORDER BY stock ASC LIMIT 5")
	for rows.Next() {
		var p PiezaResumen
		rows.Scan(&p.ID, &p.Nombre, &p.Stock, &p.StockMinimo)
		stats.StockBajoLista = append(stats.StockBajoLista, p)
	}
	rows.Close()

	pd := adminPD(r)
	pd.Title = "Dashboard"
	pd.Active = "dashboard"
	pd.Stats = stats
	renderTemplate(w, "admin/dashboard.html", pd)
}

func handleAdminPiezasList(w http.ResponseWriter, r *http.Request) {
	rows, _ := db.Query("SELECT p.id, p.codigo, p.nombre, p.precio, p.stock, p.stock_minimo, p.estado, c.nombre FROM piezas p JOIN categorias c ON c.id = p.categoria_id ORDER BY p.created_at DESC")
	var piezas []PiezaResumen
	for rows.Next() {
		var p PiezaResumen
		rows.Scan(&p.ID, &p.Codigo, &p.Nombre, &p.Precio, &p.Stock, &p.StockMinimo, &p.Estado, &p.CategoriaNombre)
		piezas = append(piezas, p)
	}
	rows.Close()
	pd := adminPD(r)
	pd.Title = "Piezas"
	pd.Active = "piezas"
	pd.Piezas = piezas
	renderTemplate(w, "admin/piezas_list.html", pd)
}

func handleAdminPiezaNuevaPage(w http.ResponseWriter, r *http.Request) {
	pd := adminPD(r)
	pd.Title = "Nueva Pieza"
	pd.Active = "piezas"
	pd.Categorias = queryCategorias()
	if s := r.URL.Query().Get("success"); s != "" {
		pd.Success = "✓ Pieza creada: " + s + ". ¡Agrega otra!"
	}
	renderTemplate(w, "admin/pieza_form.html", pd)
}

func handleAdminPiezaCrear(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	codigo := r.FormValue("codigo")
	nombre := r.FormValue("nombre")
	categoriaID, _ := strconv.Atoi(r.FormValue("categoria_id"))
	precio, _ := strconv.ParseFloat(r.FormValue("precio"), 64)
	costo, _ := strconv.ParseFloat(r.FormValue("costo"), 64)
	stock, _ := strconv.Atoi(r.FormValue("stock"))
	stockMinimo, _ := strconv.Atoi(r.FormValue("stock_minimo"))
	estado := r.FormValue("estado")
	if estado == "" { estado = "nuevo" }
	garantiaDias, _ := strconv.Atoi(r.FormValue("garantia_dias"))
	if garantiaDias == 0 { garantiaDias = 90 }

	_, err := db.Exec("INSERT INTO piezas (codigo, categoria_id, nombre, descripcion, precio, costo, stock, stock_minimo, imagen_url, estado, garantia_dias, proveedor, ubicacion) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		codigo, categoriaID, nombre, r.FormValue("descripcion"), precio, costo, stock, stockMinimo, r.FormValue("imagen_url"), estado, garantiaDias, r.FormValue("proveedor"), r.FormValue("ubicacion"))
	if err != nil {
		pd := adminPD(r)
		pd.Title = "Nueva Pieza"
		pd.Active = "piezas"
		pd.Error = err.Error()
		pd.Categorias = queryCategorias()
		renderTemplate(w, "admin/pieza_form.html", pd)
		return
	}
	if r.FormValue("_save_and_new") == "1" {
		http.Redirect(w, r, "/admin/piezas/nueva?success="+nombre, http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/admin/piezas", http.StatusSeeOther)
}

func handleAdminPiezaEditarPage(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, _ := strconv.Atoi(idStr)

	var p Pieza
	err := db.QueryRow("SELECT id, codigo, categoria_id, nombre, descripcion, precio, costo, stock, stock_minimo, imagen_url, estado, garantia_dias, proveedor, ubicacion FROM piezas WHERE id = ?", id).Scan(&p.ID, &p.Codigo, &p.CategoriaID, &p.Nombre, &p.Descripcion, &p.Precio, &p.Costo, &p.Stock, &p.StockMinimo, &p.ImagenURL, &p.Estado, &p.GarantiaDias, &p.Proveedor, &p.Ubicacion)
	if err != nil {
		http.Redirect(w, r, "/admin/piezas", http.StatusSeeOther)
		return
	}

	pd := adminPD(r)
	pd.Title = "Editar Pieza"
	pd.Active = "piezas"
	pd.Pieza = p
	pd.Categorias = queryCategorias()
	renderTemplate(w, "admin/pieza_form.html", pd)
}

func handleAdminPiezaActualizar(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	idStr := r.PathValue("id")
	id, _ := strconv.Atoi(idStr)

	categoriaID, _ := strconv.Atoi(r.FormValue("categoria_id"))
	precio, _ := strconv.ParseFloat(r.FormValue("precio"), 64)
	costo, _ := strconv.ParseFloat(r.FormValue("costo"), 64)
	stock, _ := strconv.Atoi(r.FormValue("stock"))
	stockMinimo, _ := strconv.Atoi(r.FormValue("stock_minimo"))
	garantiaDias, _ := strconv.Atoi(r.FormValue("garantia_dias"))

	_, err := db.Exec("UPDATE piezas SET codigo=?, categoria_id=?, nombre=?, descripcion=?, precio=?, costo=?, stock=?, stock_minimo=?, imagen_url=?, estado=?, garantia_dias=?, proveedor=?, ubicacion=? WHERE id=?",
		r.FormValue("codigo"), categoriaID, r.FormValue("nombre"), r.FormValue("descripcion"), precio, costo, stock, stockMinimo, r.FormValue("imagen_url"), r.FormValue("estado"), garantiaDias, r.FormValue("proveedor"), r.FormValue("ubicacion"), id)
	if err != nil {
		var p Pieza
		db.QueryRow("SELECT id, codigo, categoria_id, nombre, descripcion, precio, costo, stock, stock_minimo, imagen_url, estado, garantia_dias, proveedor, ubicacion FROM piezas WHERE id = ?", id).Scan(&p.ID, &p.Codigo, &p.CategoriaID, &p.Nombre, &p.Descripcion, &p.Precio, &p.Costo, &p.Stock, &p.StockMinimo, &p.ImagenURL, &p.Estado, &p.GarantiaDias, &p.Proveedor, &p.Ubicacion)
		pd := adminPD(r)
		pd.Title = "Editar Pieza"
		pd.Active = "piezas"
		pd.Error = err.Error()
		pd.Pieza = p
		pd.Categorias = queryCategorias()
		renderTemplate(w, "admin/pieza_form.html", pd)
		return
	}
	http.Redirect(w, r, "/admin/piezas", http.StatusSeeOther)
}

func handleAdminPiezaEliminar(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, _ := strconv.Atoi(idStr)
	_, err := db.Exec("DELETE FROM piezas WHERE id = ?", id)
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		json.NewEncoder(w).Encode(map[string]bool{"success": false})
	} else {
		json.NewEncoder(w).Encode(map[string]bool{"success": true})
	}
}

func handleAdminModelosList(w http.ResponseWriter, r *http.Request) {
	marcaID := r.URL.Query().Get("marca_id")
	q := r.URL.Query().Get("q")

	where := []string{"1=1"}
	args := []any{}
	if marcaID != "" {
		where = append(where, "mo.marca_id = ?")
		args = append(args, marcaID)
	}
	if q != "" {
		where = append(where, "mo.nombre LIKE ?")
		args = append(args, "%"+q+"%")
	}

	rows, _ := db.Query("SELECT mo.id, mo.nombre, mo.año_lanzamiento, mo.activo, m.nombre, (SELECT COUNT(*) FROM compatibilidad WHERE modelo_id = mo.id) FROM modelos mo JOIN marcas m ON m.id = mo.marca_id WHERE "+strings.Join(where, " AND ")+" ORDER BY m.nombre, mo.nombre", args...)
	var modelos []Modelo
	for rows.Next() {
		var mo Modelo
		rows.Scan(&mo.ID, &mo.Nombre, &mo.AñoLanzamiento, &mo.Activo, &mo.MarcaNombre, &mo.PiezasCount)
		modelos = append(modelos, mo)
	}
	rows.Close()

	pd := adminPD(r)
	pd.Title = "Modelos"
	pd.Active = "modelos"
	pd.Modelos = modelos
	pd.Marcas = queryMarcas()
	pd.MarcaID = marcaID
	pd.Query = q
	renderTemplate(w, "admin/modelos_list.html", pd)
}

func handleAdminModeloCrear(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	marcaID, _ := strconv.Atoi(r.FormValue("marca_id"))
	nombre := r.FormValue("nombre")
	año, _ := strconv.Atoi(r.FormValue("año"))
	db.Exec("INSERT INTO modelos (marca_id, nombre, año_lanzamiento) VALUES (?, ?, ?)", marcaID, nombre, año)
	http.Redirect(w, r, "/admin/modelos", http.StatusSeeOther)
}

func handleAdminCompatibilidadPage(w http.ResponseWriter, r *http.Request) {
	pd := adminPD(r)
	rows, _ := db.Query("SELECT p.id, p.codigo, p.nombre FROM piezas p WHERE p.activa = 1 ORDER BY p.nombre")
	var piezas []PiezaResumen
	for rows.Next() {
		var p PiezaResumen
		rows.Scan(&p.ID, &p.Codigo, &p.Nombre)
		piezas = append(piezas, p)
	}
	rows.Close()

	rows, _ = db.Query("SELECT mo.id, mo.nombre, m.nombre FROM modelos mo JOIN marcas m ON m.id = mo.marca_id WHERE mo.activo = 1 ORDER BY m.nombre, mo.nombre")
	var modelos []Modelo
	for rows.Next() {
		var mo Modelo
		rows.Scan(&mo.ID, &mo.Nombre, &mo.MarcaNombre)
		modelos = append(modelos, mo)
	}
	rows.Close()

	type CompRow struct {
		ID            int
		Notas         string
		PiezaNombre   string
		ModeloNombre  string
		MarcaNombre   string
	}
	rows, _ = db.Query("SELECT cp.id, COALESCE(cp.notas,''), p.nombre, mo.nombre, m.nombre FROM compatibilidad cp JOIN piezas p ON p.id = cp.pieza_id JOIN modelos mo ON mo.id = cp.modelo_id JOIN marcas m ON m.id = mo.marca_id ORDER BY cp.id DESC LIMIT 20")
	var comps []CompRow
	for rows.Next() {
		var c CompRow
		rows.Scan(&c.ID, &c.Notas, &c.PiezaNombre, &c.ModeloNombre, &c.MarcaNombre)
		comps = append(comps, c)
	}
	rows.Close()

	pd.Title = "Compatibilidad"
	pd.Active = "compatibilidad"
	pd.Piezas = piezas
	pd.Modelos = modelos
	pd.Compatibilidades = anySlice(comps)
	renderTemplate(w, "admin/compatibilidad.html", pd)
}

func handleAdminCompatibilidadGuardar(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	piezaID, _ := strconv.Atoi(r.FormValue("pieza_id"))
	notas := r.FormValue("notas")
	for _, idStr := range r.Form["modelo_ids"] {
		modeloID, _ := strconv.Atoi(idStr)
		db.Exec("INSERT OR IGNORE INTO compatibilidad (pieza_id, modelo_id, notas) VALUES (?, ?, ?)", piezaID, modeloID, notas)
	}
	http.Redirect(w, r, "/admin/compatibilidad", http.StatusSeeOther)
}

func handleAdminStats(w http.ResponseWriter, r *http.Request) {
	var stats DashboardStats
	db.QueryRow("SELECT COUNT(*) FROM piezas").Scan(&stats.TotalPiezas)
	db.QueryRow("SELECT COUNT(*) FROM piezas WHERE stock = 0").Scan(&stats.Agotadas)
	db.QueryRow("SELECT COALESCE(SUM(stock * costo), 0) FROM piezas").Scan(&stats.ValorInventario)
	db.QueryRow("SELECT COALESCE(SUM(stock * (precio - costo)), 0) FROM piezas").Scan(&stats.GananciaPotencial)
	db.QueryRow("SELECT COUNT(*) FROM ventas").Scan(&stats.TotalVentas)

	rows, _ := db.Query("SELECT c.nombre, COUNT(*) FROM piezas p JOIN categorias c ON c.id = p.categoria_id GROUP BY c.nombre ORDER BY COUNT(*) DESC")
	for rows.Next() {
		var pc PiezaCount
		rows.Scan(&pc.Nombre, &pc.Count)
		stats.PiezasPorCategoria = append(stats.PiezasPorCategoria, pc)
	}
	rows.Close()

	rows, _ = db.Query("SELECT estado, COUNT(*) FROM piezas GROUP BY estado")
	for rows.Next() {
		var pc PiezaCount
		rows.Scan(&pc.Estado, &pc.Count)
		stats.PiezasPorEstado = append(stats.PiezasPorEstado, pc)
	}
	rows.Close()

	pd := adminPD(r)
	pd.Title = "Estadísticas"
	pd.Active = "stats"
	pd.Stats = stats
	renderTemplate(w, "admin/stats.html", pd)
}

func handleAdminVentaCrear(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	var folio int
	db.QueryRow("SELECT COALESCE(MAX(folio), 0) + 1 FROM ventas").Scan(&folio)
	formaPago := r.FormValue("forma_pago")
	if formaPago == "" { formaPago = "efectivo" }

	piezaIDs := r.Form["pieza_id"]
	cantidades := r.Form["cantidad"]
	precios := r.Form["precio"]

	var total float64
	for i := range piezaIDs {
		cant, _ := strconv.Atoi(cantidades[i])
		precio, _ := strconv.ParseFloat(precios[i], 64)
		total += float64(cant) * precio
	}

	result, err := db.Exec("INSERT INTO ventas (folio, cliente_nombre, cliente_telefono, total, forma_pago, vendedor_id, notas) VALUES (?, ?, ?, ?, ?, ?, ?)",
		folio, r.FormValue("cliente_nombre"), r.FormValue("cliente_telefono"), total, formaPago, 1, r.FormValue("notas"))
	if err != nil {
		log.Printf("Error creating venta: %v", err)
		http.Redirect(w, r, "/admin/ventas", http.StatusSeeOther)
		return
	}
	ventaID, _ := result.LastInsertId()

	for i := range piezaIDs {
		piezaID, _ := strconv.Atoi(piezaIDs[i])
		cant, _ := strconv.Atoi(cantidades[i])
		precio, _ := strconv.ParseFloat(precios[i], 64)
		subtotal := float64(cant) * precio
		db.Exec("INSERT INTO ventas_detalle (venta_id, pieza_id, cantidad, precio_unitario, subtotal) VALUES (?, ?, ?, ?, ?)", ventaID, piezaID, cant, precio, subtotal)
		db.Exec("UPDATE piezas SET stock = stock - ? WHERE id = ?", cant, piezaID)
	}

	http.Redirect(w, r, "/admin/ventas", http.StatusSeeOther)
}

func handleAdminVentasList(w http.ResponseWriter, r *http.Request) {
	rows, _ := db.Query("SELECT v.id, v.folio, v.cliente_nombre, v.total, v.forma_pago, v.creado_en, COALESCE(u.nombre_completo, u.usuario) FROM ventas v LEFT JOIN usuarios u ON u.id = v.vendedor_id ORDER BY v.creado_en DESC LIMIT 100")
	var ventas []Venta
	for rows.Next() {
		var v Venta
		rows.Scan(&v.ID, &v.Folio, &v.ClienteNombre, &v.Total, &v.FormaPago, &v.CreadoEn, &v.VendedorNombre)
		ventas = append(ventas, v)
	}
	rows.Close()

	rows2, _ := db.Query("SELECT p.id, p.codigo, p.nombre, p.precio, p.stock FROM piezas p WHERE p.activa = 1 AND p.stock > 0 ORDER BY p.nombre")
	var piezas []PiezaResumen
	for rows2.Next() {
		var p PiezaResumen
		rows2.Scan(&p.ID, &p.Codigo, &p.Nombre, &p.Precio, &p.Stock)
		piezas = append(piezas, p)
	}
	rows2.Close()

	pd := adminPD(r)
	pd.Title = "Ventas"
	pd.Active = "ventas"
	pd.Ventas = ventas
	pd.Piezas = piezas
	renderTemplate(w, "admin/ventas_list.html", pd)
}

func handleAdminConfigPage(w http.ResponseWriter, r *http.Request) {
	pd := adminPD(r)
	pd.Title = "Configuración"
	pd.Active = "config"
	pd.Config = map[string]string{
		"NegocioNombre": getConfig("negocio_nombre"),
		"Tema":          getConfig("tema"),
	}
	renderTemplate(w, "admin/config.html", pd)
}

func handleAdminConfigGuardar(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	if v := r.FormValue("negocio_nombre"); v != "" {
		setConfig("negocio_nombre", v)
	}
	if v := r.FormValue("tema"); v != "" {
		setConfig("tema", v)
	}
	if u := r.FormValue("nuevo_usuario"); u != "" && r.FormValue("nueva_clave") != "" {
		db.Exec("INSERT INTO usuarios (usuario, clave, nombre_completo, rol) VALUES (?, ?, ?, ?)", u, hashPassword(r.FormValue("nueva_clave")), u, r.FormValue("nuevo_rol"))
	}

	http.Redirect(w, r, "/admin/config", http.StatusSeeOther)
}

// ─── Upload Handler ───────────────────────────────────────────────

func handleAdminUpload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 5<<20) // 5MB max
	if err := r.ParseMultipartForm(5 << 20); err != nil {
		json.NewEncoder(w).Encode(map[string]any{"error": "Archivo muy grande (máx 5MB)"})
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		json.NewEncoder(w).Encode(map[string]any{"error": "No se recibió archivo"})
		return
	}
	defer file.Close()

	ext := filepath.Ext(header.Filename)
	if ext == "" {
		ext = ".jpg"
	}
	ext = strings.ToLower(ext)
	allowed := map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".webp": true, ".gif": true}
	if !allowed[ext] {
		json.NewEncoder(w).Encode(map[string]any{"error": "Formato no permitido: " + ext})
		return
	}

	b := make([]byte, 8)
	rand.Read(b)
	filename := fmt.Sprintf("%d_%s%s", time.Now().UnixMilli(), hex.EncodeToString(b), ext)
	dst, err := os.Create(filepath.Join("uploads", filename))
	if err != nil {
		json.NewEncoder(w).Encode(map[string]any{"error": "Error al guardar archivo"})
		return
	}
	defer dst.Close()
	io.Copy(dst, file)

	json.NewEncoder(w).Encode(map[string]any{"url": "/uploads/" + filename})
}

// ─── Repair Handlers ──────────────────────────────────────────────

// Dashboard principal
func handleAdminReparacionesList(w http.ResponseWriter, r *http.Request) {
	f := r.URL.Query().Get("f")
	q := r.URL.Query().Get("q")
	s := r.URL.Query().Get("s") // status filter

	where := []string{"r.activo = 1"}
	args := []any{}
	if s != "" && s != "todas" {
		where = append(where, "r.status = ?")
		args = append(args, s)
	}
	if q != "" {
		where = append(where, "(r.cliente_nombre LIKE ? OR r.modelo_texto LIKE ? OR r.falla_reportada LIKE ? OR r.numero_serie LIKE ?)")
		like := "%" + q + "%"
		args = append(args, like, like, like, like)
	}
	// Custom filter
	if f == "hoy" {
		where = append(where, "date(r.fecha_ingreso) = date('now')")
	} else if f == "atrasados" {
		where = append(where, "julianday('now') - julianday(r.fecha_ingreso) > 7")
	} else if f == "listos" {
		where = append(where, "r.status = 'reparado'")
	} else if f == "proceso" {
		where = append(where, "r.status IN ('recibido','diagnosticando')")
	}

	rows, _ := db.Query(`SELECT r.id, r.folio, r.token, r.tipo_equipo,
		COALESCE(m.nombre,'') as marca_nombre, r.modelo_texto,
		r.cliente_nombre, COALESCE(r.cliente_telefono,'') as cliente_telefono,
		r.status, r.fecha_ingreso, r.total
		FROM reparaciones r
		LEFT JOIN marcas m ON m.id = r.marca_id
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY r.fecha_ingreso DESC LIMIT 100`, args...)

	var reps []ReparacionResumen
	for rows.Next() {
		var r ReparacionResumen
		rows.Scan(&r.ID, &r.Folio, &r.Token, &r.TipoEquipo,
			&r.MarcaNombre, &r.ModeloTexto,
			&r.ClienteNombre, &r.ClienteTelefono,
			&r.Status, &r.FechaIngreso, &r.Total)
		reps = append(reps, r)
	}
	rows.Close()

	// Stats
	var stats ReparacionStats
	db.QueryRow(`SELECT
		COALESCE(SUM(CASE WHEN date(fecha_ingreso) = date('now') THEN 1 ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN status IN ('recibido','diagnosticando') THEN 1 ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN status = 'presupuestado' THEN 1 ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN status = 'reparado' THEN 1 ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN julianday('now')-julianday(fecha_ingreso)>7 AND status NOT IN ('entregado','cancelado') THEN 1 ELSE 0 END),0),
		COUNT(*),
		COALESCE(SUM(CASE WHEN status IN ('entregado','cancelado') AND strftime('%Y-%m',fecha_entrega)=strftime('%Y-%m','now') THEN 1 ELSE 0 END),0)
		FROM reparaciones WHERE activo=1`).Scan(
		&stats.HoyRecibidos, &stats.EnProceso, &stats.EsperaAprobacion,
		&stats.ListosEntrega, &stats.Atrasados, &stats.TotalActivos,
		&stats.CompletadosMes)

	pd := adminPD(r)
	pd.Title = "Reparaciones"
	pd.Active = "reparaciones"
	pd.Reparaciones = reps
	pd.ReparacionStats = stats
	pd.Query = q
	pd.ReparacionStatus = s
	pd.ReparacionFilter = f
	renderTemplate(w, "admin/reparaciones_list.html", pd)
}

// Kanban
func handleAdminReparacionesKanban(w http.ResponseWriter, r *http.Request) {
	rows, _ := db.Query(`SELECT r.id, r.folio, r.token, r.tipo_equipo,
		COALESCE(m.nombre,'') as marca_nombre, r.modelo_texto,
		r.cliente_nombre, COALESCE(r.cliente_telefono,''),
		r.status, r.fecha_ingreso, r.total
		FROM reparaciones r
		LEFT JOIN marcas m ON m.id = r.marca_id
		WHERE r.activo = 1 AND r.status NOT IN ('entregado','cancelado')
		ORDER BY r.fecha_ingreso ASC`)
	defer rows.Close()

	byStatus := map[string][]ReparacionResumen{}
	for rows.Next() {
		var r ReparacionResumen
		rows.Scan(&r.ID, &r.Folio, &r.Token, &r.TipoEquipo,
			&r.MarcaNombre, &r.ModeloTexto,
			&r.ClienteNombre, &r.ClienteTelefono,
			&r.Status, &r.FechaIngreso, &r.Total)
		byStatus[r.Status] = append(byStatus[r.Status], r)
	}

	pd := adminPD(r)
	pd.Title = "Kanban — Reparaciones"
	pd.Active = "reparaciones"
	pd.KanbanData = byStatus
	pd.KanbanStatuses = []string{"recibido", "diagnosticando", "presupuestado", "aprobado", "reparando", "reparado", "entregado"}
	renderTemplate(w, "admin/reparaciones_kanban.html", pd)
}

// Nueva reparación — form
func handleAdminReparacionNuevaPage(w http.ResponseWriter, r *http.Request) {
	pd := adminPD(r)
	pd.Title = "Nueva Reparación"
	pd.Active = "reparaciones"
	pd.Marcas = queryMarcas()
	pd.TipoEquipos = TipoEquipos
	renderTemplate(w, "admin/reparacion_form.html", pd)
}

// Crear reparación
func handleAdminReparacionCrear(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	marcaID, _ := strconv.Atoi(r.FormValue("marca_id"))

	var folio int
	db.QueryRow("SELECT COALESCE(MAX(folio), 0) + 1 FROM reparaciones").Scan(&folio)

	_, err := db.Exec(`INSERT INTO reparaciones
		(folio, tipo_equipo, marca_id, modelo_texto, numero_serie, imei,
		 password_equipo, condicion_fisica, cliente_nombre, cliente_telefono,
		 cliente_email, falla_reportada, diagnostico, accesorios,
		 notas_cliente, notas_internas, status, fecha_prometida,
		 costo_diagnostico, anticipo, tecnico_id)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		folio,
		r.FormValue("tipo_equipo"),
		marcaID,
		r.FormValue("modelo_texto"),
		r.FormValue("numero_serie"),
		r.FormValue("imei"),
		r.FormValue("password_equipo"),
		r.FormValue("condicion_fisica"),
		r.FormValue("cliente_nombre"),
		r.FormValue("cliente_telefono"),
		r.FormValue("cliente_email"),
		r.FormValue("falla_reportada"),
		r.FormValue("diagnostico"),
		r.FormValue("accesorios"),
		r.FormValue("notas_cliente"),
		r.FormValue("notas_internas"),
		"recibido",
		r.FormValue("fecha_prometida"),
		parseFloat(r.FormValue("costo_diagnostico")),
		parseFloat(r.FormValue("anticipo")),
		sessionUserID(r),
	)

	if err != nil {
		pd := adminPD(r)
		pd.Title = "Nueva Reparación"
		pd.Active = "reparaciones"
		pd.Error = err.Error()
		pd.Marcas = queryMarcas()
		renderTemplate(w, "admin/reparacion_form.html", pd)
		return
	}

	if r.FormValue("_save_and_new") == "1" {
		http.Redirect(w, r, "/admin/reparaciones/nueva?success=ok", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/admin/reparaciones", http.StatusSeeOther)
}

// Detalle de reparación
func handleAdminReparacionDetalle(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	rep := queryReparacionFull(id)
	if rep.ID == 0 {
		http.Redirect(w, r, "/admin/reparaciones", http.StatusSeeOther)
		return
	}
	pd := adminPD(r)
	pd.Title = "Rep #" + strconv.Itoa(rep.Folio) + " — " + rep.ClienteNombre
	pd.Active = "reparaciones"
	pd.Reparacion = rep
	pd.Marcas = queryMarcas()
	pd.Piezas = queryPiezasActivas()
	pd.KanbanStatuses = []string{"recibido", "diagnosticando", "presupuestado", "aprobado", "reparando", "reparado", "entregado"}
	renderTemplate(w, "admin/reparacion_detail.html", pd)
}

// Cambiar status
func handleAdminReparacionStatus(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	r.ParseForm()
	nuevoStatus := r.FormValue("status")
	notas := r.FormValue("notas")

	var statusActual string
	db.QueryRow("SELECT status FROM reparaciones WHERE id = ?", id).Scan(&statusActual)
	if statusActual == "" {
		json.NewEncoder(w).Encode(map[string]any{"error": "Reparación no encontrada"})
		return
	}

	if nuevoStatus == "entregado" || nuevoStatus == "cancelado" {
		db.Exec("UPDATE reparaciones SET status=?, fecha_entrega=datetime('now','localtime') WHERE id=?", nuevoStatus, id)
	} else {
		db.Exec("UPDATE reparaciones SET status=? WHERE id=?", nuevoStatus, id)
	}

	// Log manual (the trigger handles status_anterior/status_nuevo)
	uid := sessionUserID(r)
	if notas != "" {
		db.Exec("UPDATE reparaciones_historial SET notas=?, usuario_id=? WHERE reparacion_id=? AND status_nuevo=? AND notas IS NULL",
			notas, uid, id, nuevoStatus)
	}

	json.NewEncoder(w).Encode(map[string]any{"success": true, "status": nuevoStatus})
}

type StatusUpdateData struct {
	Status      string
	Label       string
	Color       string
	Icon        string
	StatusesForSelect []string
	StatusLabels      map[string]string
	StatusColors      map[string]string
	StatusIcons       map[string]string
	IsActive    bool
}

// Agregar pieza a reparación
func handleAdminReparacionPiezaAgregar(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	r.ParseForm()
	piezaID, _ := strconv.Atoi(r.FormValue("pieza_id"))
	cantidad, _ := strconv.Atoi(r.FormValue("cantidad"))
	if cantidad < 1 {
		cantidad = 1
	}

	var precio float64
	var nombre string
	err := db.QueryRow("SELECT precio, nombre FROM piezas WHERE id = ? AND activa = 1", piezaID).Scan(&precio, &nombre)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]any{"error": "Pieza no encontrada"})
		return
	}

	subtotal := precio * float64(cantidad)
	_, err = db.Exec("INSERT INTO reparaciones_piezas (reparacion_id, pieza_id, cantidad, precio_unitario, subtotal) VALUES (?,?,?,?,?)",
		id, piezaID, cantidad, precio, subtotal)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
		return
	}

	// Update repair total
	db.Exec(`UPDATE reparaciones SET total = (
		SELECT COALESCE(SUM(subtotal),0) FROM reparaciones_piezas WHERE reparacion_id = ?
	) + costo_diagnostico WHERE id = ?`, id, id)

	json.NewEncoder(w).Encode(map[string]any{"success": true, "pieza": nombre, "cantidad": cantidad})
}

// Quitar pieza de reparación
func handleAdminReparacionPiezaQuitar(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	pid, _ := strconv.Atoi(r.PathValue("pid"))
	db.Exec("DELETE FROM reparaciones_piezas WHERE id = ? AND reparacion_id = ?", pid, id)

	db.Exec(`UPDATE reparaciones SET total = (
		SELECT COALESCE(SUM(subtotal),0) FROM reparaciones_piezas WHERE reparacion_id = ?
	) + costo_diagnostico WHERE id = ?`, id, id)

	json.NewEncoder(w).Encode(map[string]any{"success": true})
}

// Subir archivo a reparación
func handleAdminReparacionArchivoSubir(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20) // 10MB max
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		json.NewEncoder(w).Encode(map[string]any{"error": "Archivo muy grande (máx 10MB)"})
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		json.NewEncoder(w).Encode(map[string]any{"error": "No se recibió archivo"})
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	allowed := map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".webp": true, ".gif": true, ".pdf": true}
	if !allowed[ext] {
		json.NewEncoder(w).Encode(map[string]any{"error": "Formato no permitido: " + ext})
		return
	}

	b := make([]byte, 8)
	rand.Read(b)
	filename := fmt.Sprintf("rep%d_%s%s", id, hex.EncodeToString(b), ext)
	dst, err := os.Create(filepath.Join("uploads", filename))
	if err != nil {
		json.NewEncoder(w).Encode(map[string]any{"error": "Error al guardar"})
		return
	}
	defer dst.Close()
	io.Copy(dst, file)

	url := "/uploads/" + filename
	tipo := "foto"
	if ext == ".pdf" {
		tipo = "pdf"
	}

	uid := sessionUserID(r)
	db.Exec("INSERT INTO reparaciones_archivos (reparacion_id, nombre, url, tipo, subido_por) VALUES (?,?,?,?,?)",
		id, header.Filename, url, tipo, uid)

	json.NewEncoder(w).Encode(map[string]any{"success": true, "url": url, "nombre": header.Filename})
}

// Quitar archivo
func handleAdminReparacionArchivoQuitar(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	aid, _ := strconv.Atoi(r.PathValue("aid"))
	var url string
	db.QueryRow("SELECT url FROM reparaciones_archivos WHERE id = ? AND reparacion_id = ?", aid, id).Scan(&url)
	if url != "" {
		os.Remove("." + url)
	}
	db.Exec("DELETE FROM reparaciones_archivos WHERE id = ? AND reparacion_id = ?", aid, id)
	json.NewEncoder(w).Encode(map[string]any{"success": true})
}

// Guardar notas
func handleAdminReparacionGuardar(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	r.ParseForm()
	db.Exec(`UPDATE reparaciones SET
		cliente_nombre=?, cliente_telefono=?, cliente_email=?,
		modelo_texto=?, numero_serie=?, imei=?, password_equipo=?, condicion_fisica=?,
		falla_reportada=?, diagnostico=?, accesorios=?,
		notas_cliente=?, notas_internas=?,
		fecha_prometida=?, costo_diagnostico=?, anticipo=?
		WHERE id=?`,
		r.FormValue("cliente_nombre"),
		r.FormValue("cliente_telefono"),
		r.FormValue("cliente_email"),
		r.FormValue("modelo_texto"),
		r.FormValue("numero_serie"),
		r.FormValue("imei"),
		r.FormValue("password_equipo"),
		r.FormValue("condicion_fisica"),
		r.FormValue("falla_reportada"),
		r.FormValue("diagnostico"),
		r.FormValue("accesorios"),
		r.FormValue("notas_cliente"),
		r.FormValue("notas_internas"),
		r.FormValue("fecha_prometida"),
		parseFloat(r.FormValue("costo_diagnostico")),
		parseFloat(r.FormValue("anticipo")),
		id)
	http.Redirect(w, r, "/admin/reparaciones/"+strconv.Itoa(id), http.StatusSeeOther)
}

// Portal público: el cliente ve su reparación
func handlePublicReparacionStatus(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")

	var rep Reparacion
	err := db.QueryRow(`SELECT r.id, r.folio, r.token, r.tipo_equipo,
		COALESCE(m.nombre,''), COALESCE(r.modelo_texto,''),
		COALESCE(r.numero_serie,''), COALESCE(r.imei,''),
		COALESCE(r.cliente_nombre,''), COALESCE(r.cliente_telefono,''),
		COALESCE(r.falla_reportada,''), COALESCE(r.diagnostico,''),
		COALESCE(r.notas_cliente,''),
		r.status, r.fecha_ingreso, COALESCE(r.fecha_prometida,''), COALESCE(r.fecha_entrega,''),
		r.costo_diagnostico, r.costo_reparacion, r.total, r.anticipo
		FROM reparaciones r
		LEFT JOIN marcas m ON m.id = r.marca_id
		WHERE r.token = ? AND r.activo = 1`, token).Scan(
		&rep.ID, &rep.Folio, &rep.Token, &rep.TipoEquipo,
		&rep.MarcaNombre, &rep.ModeloTexto,
		&rep.NumeroSerie, &rep.IMEI,
		&rep.ClienteNombre, &rep.ClienteTelefono,
		&rep.FallaReportada, &rep.Diagnostico,
		&rep.NotasCliente,
		&rep.Status, &rep.FechaIngreso, &rep.FechaPrometida, &rep.FechaEntrega,
		&rep.CostoDiagnostico, &rep.CostoReparacion, &rep.Total, &rep.Anticipo)

	if err != nil {
		renderTemplate(w, "portal/reparacion.html", PageData{
			Title: "Reparación no encontrada",
			Error: "No encontramos ninguna reparación con ese código. Verificá el enlace o contactanos.",
		})
		return
	}

	// Get history
	rows, _ := db.Query(`SELECT rh.status_anterior, rh.status_nuevo,
		COALESCE(rh.notas,''), rh.creado_en
		FROM reparaciones_historial rh
		WHERE rh.reparacion_id = ?
		ORDER BY rh.creado_en ASC`, rep.ID)
	for rows.Next() {
		var h ReparacionHistorial
		rows.Scan(&h.StatusAnterior, &h.StatusNuevo, &h.Notas, &h.CreadoEn)
		rep.Historial = append(rep.Historial, h)
	}
	rows.Close()

	theme := getConfig("tema")
	telefono := getConfig("telefono")
	negocio := getConfig("negocio_nombre")

	pd := PageData{
		Title:     "Reparación #" + strconv.Itoa(rep.Folio),
		Theme:     theme,
		Telefono:  telefono,
		Negocio:   negocio,
		Reparacion: rep,
	}
	renderTemplate(w, "portal/reparacion.html", pd)
}

// ─── Repair helpers ───────────────────────────────────────────────

func queryReparacionFull(id int) Reparacion {
	var r Reparacion
	err := db.QueryRow(`SELECT r.id, r.folio, r.token, r.tipo_equipo,
		COALESCE(r.marca_id,0), COALESCE(m.nombre,''), COALESCE(r.modelo_texto,''),
		COALESCE(r.numero_serie,''), COALESCE(r.imei,''), COALESCE(r.password_equipo,''),
		COALESCE(r.condicion_fisica,''),
		COALESCE(r.cliente_nombre,''), COALESCE(r.cliente_telefono,''), COALESCE(r.cliente_email,''),
		COALESCE(r.falla_reportada,''), COALESCE(r.diagnostico,''), COALESCE(r.accesorios,''),
		COALESCE(r.notas_cliente,''), COALESCE(r.notas_internas,''),
		r.status, r.fecha_ingreso, COALESCE(r.fecha_prometida,''), COALESCE(r.fecha_entrega,''),
		r.costo_diagnostico, r.costo_reparacion, r.total, r.anticipo,
		COALESCE(r.tecnico_id,0), COALESCE(u.nombre_completo,'')
		FROM reparaciones r
		LEFT JOIN marcas m ON m.id = r.marca_id
		LEFT JOIN usuarios u ON u.id = r.tecnico_id
		WHERE r.id = ?`, id).Scan(
		&r.ID, &r.Folio, &r.Token, &r.TipoEquipo,
		&r.MarcaID, &r.MarcaNombre, &r.ModeloTexto,
		&r.NumeroSerie, &r.IMEI, &r.PasswordEquipo,
		&r.CondicionFisica,
		&r.ClienteNombre, &r.ClienteTelefono, &r.ClienteEmail,
		&r.FallaReportada, &r.Diagnostico, &r.Accesorios,
		&r.NotasCliente, &r.NotasInternas,
		&r.Status, &r.FechaIngreso, &r.FechaPrometida, &r.FechaEntrega,
		&r.CostoDiagnostico, &r.CostoReparacion, &r.Total, &r.Anticipo,
		&r.TecnicoID, &r.TecnicoNombre)
	if err != nil {
		return r
	}
	r.Activo = true

	// History
	rows, _ := db.Query(`SELECT rh.id, COALESCE(rh.status_anterior,''), rh.status_nuevo,
		COALESCE(rh.usuario_id,0), COALESCE(u.nombre_completo,''),
		COALESCE(rh.notas,''), rh.creado_en
		FROM reparaciones_historial rh
		LEFT JOIN usuarios u ON u.id = rh.usuario_id
		WHERE rh.reparacion_id = ?
		ORDER BY rh.creado_en ASC`, id)
	for rows.Next() {
		var h ReparacionHistorial
		rows.Scan(&h.ID, &h.StatusAnterior, &h.StatusNuevo,
			&h.UsuarioID, &h.UsuarioNombre, &h.Notas, &h.CreadoEn)
		r.Historial = append(r.Historial, h)
	}
	rows.Close()

	// Parts used
	rows2, _ := db.Query(`SELECT rp.id, rp.reparacion_id, rp.pieza_id,
		p.nombre, p.codigo, rp.cantidad, rp.precio_unitario, rp.subtotal
		FROM reparaciones_piezas rp
		JOIN piezas p ON p.id = rp.pieza_id
		WHERE rp.reparacion_id = ?`, id)
	for rows2.Next() {
		var rp ReparacionPieza
		rows2.Scan(&rp.ID, &rp.ReparacionID, &rp.PiezaID,
			&rp.PiezaNombre, &rp.PiezaCodigo, &rp.Cantidad,
			&rp.PrecioUnitario, &rp.Subtotal)
		r.PiezasUsadas = append(r.PiezasUsadas, rp)
	}
	rows2.Close()

	// Files
	rows3, _ := db.Query(`SELECT id, nombre, url, tipo, subido_por, created_at
		FROM reparaciones_archivos WHERE reparacion_id = ? ORDER BY created_at DESC`, id)
	for rows3.Next() {
		var a ReparacionArchivo
		rows3.Scan(&a.ID, &a.Nombre, &a.URL, &a.Tipo, &a.SubidoPor, &a.CreadoEn)
		r.Archivos = append(r.Archivos, a)
	}
	rows3.Close()

	return r
}

func queryPiezasActivas() []PiezaResumen {
	rows, _ := db.Query("SELECT id, nombre, codigo, precio, stock FROM piezas WHERE activa = 1 ORDER BY nombre")
	defer rows.Close()
	var ps []PiezaResumen
	for rows.Next() {
		var p PiezaResumen
		rows.Scan(&p.ID, &p.Nombre, &p.Codigo, &p.Precio, &p.Stock)
		ps = append(ps, p)
	}
	return ps
}

// ─── Queries ──────────────────────────────────────────────────────

func queryCategorias() []Categoria {
	rows, _ := db.Query("SELECT id, nombre, slug, icono FROM categorias WHERE activa = 1 ORDER BY orden")
	var cats []Categoria
	for rows.Next() {
		var c Categoria
		rows.Scan(&c.ID, &c.Nombre, &c.Slug, &c.Icono)
		cats = append(cats, c)
	}
	rows.Close()
	return cats
}

func queryMarcas() []Marca {
	rows, _ := db.Query("SELECT id, nombre FROM marcas WHERE activa = 1 ORDER BY nombre")
	var marcas []Marca
	for rows.Next() {
		var m Marca
		rows.Scan(&m.ID, &m.Nombre)
		marcas = append(marcas, m)
	}
	rows.Close()
	return marcas
}

func getConfig(key string) string {
	var val string
	db.QueryRow("SELECT valor FROM config WHERE clave = ?", key).Scan(&val)
	return val
}

func setConfig(key, val string) {
	db.Exec("INSERT INTO config (clave, valor) VALUES (?, ?) ON CONFLICT(clave) DO UPDATE SET valor = ?", key, val, val)
}
