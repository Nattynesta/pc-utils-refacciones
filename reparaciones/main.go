package main

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// ─── Config ────────────────────────────────────────────────────
const DB_PATH = "../pos/refacciones.db"
const PORT = "8082"

// ─── Embeds ────────────────────────────────────────────────────

//go:embed templates
var templateFS embed.FS

//go:embed static
var staticFS embed.FS

// ─── Globals ───────────────────────────────────────────────────

var db *sql.DB
var templates map[string]*template.Template
var csrfKey []byte
var csrfMux sync.Mutex

// ─── Types ─────────────────────────────────────────────────────

type Reparacion struct {
	ID               int
	Folio            int
	Token            string
	TipoEquipo       string
	MarcaNombre      string
	ModeloTexto      string
	NumeroSerie      string
	IMEI             string
	PasswordEquipo   string
	CondicionFisica  string
	ClienteNombre    string
	ClienteTelefono  string
	ClienteEmail     string
	FallaReportada   string
	Diagnostico      string
	Accesorios       string
	NotasCliente     string
	NotasInternas    string
	Status           string
	FechaIngreso     string
	FechaPrometida   string
	FechaEntrega     string
	CostoDiagnostico float64
	CostoReparacion  float64
	Total            float64
	Anticipo         float64
	TecnicoNombre    string
	Historial        []HistorialEntry
	PiezasUsadas     []PiezaUsada
	Archivos         []Archivo
	DiasEnStatus     int
}

type ReparacionRow struct {
	ID              int
	Folio           int
	Token           string
	TipoEquipo      string
	MarcaNombre     string
	ModeloTexto     string
	ClienteNombre   string
	ClienteTelefono string
	Status          string
	FechaIngreso    string
	Total           float64
	DiasEnStatus    int
	FallaReportada  string
}

type HistorialEntry struct {
	StatusAnterior string
	StatusNuevo    string
	Notas          string
	CreadoEn       string
	UsuarioNombre  string
}

type PiezaUsada struct {
	ID             int
	PiezaNombre    string
	PiezaCodigo    string
	Cantidad       int
	PrecioUnitario float64
	Subtotal       float64
}

type Archivo struct {
	ID   int
	Nombre string
	URL  string
	Tipo string
}

type DashboardStats struct {
	HoyRecibidos     int
	EnProceso        int
	EsperaAprobacion int
	ListosEntrega    int
	Atrasados        int
	TotalActivos     int
	CompletadosMes   int
	ValorPendiente   float64
}

type PageData struct {
	Title          string
	Active         string
	Error          string
	Success        string
	CSRFToken      string
	User           string
	Telefono       string
	Negocio        string
	Stats          DashboardStats
	Rows           []ReparacionRow
	Rep            Reparacion
	Query          string
	StatusFilter   string
	Marcas         []MarcaOption
	Piezas         []PiezaOption
	StatusCounts   map[string]int
	Data           any
	StatusSequence []string
}

type MarcaOption struct {
	ID     int
	Nombre string
}

type PiezaOption struct {
	ID     int
	Nombre string
	Codigo string
	Precio float64
	Stock  int
}

type SessionData struct {
	UserID   int
	Username string
	Role     string
}

// ─── Status Maps ────────────────────────────────────────────────

var statusLabels = map[string]string{
	"recibido": "Recibido", "diagnosticando": "Diagnosticando",
	"presupuestado": "Presupuestado", "aprobado": "Aprobado",
	"reparando": "Reparando", "reparado": "Reparado",
	"entregado": "Entregado", "cancelado": "Cancelado",
}

var statusColors = map[string]string{
	"recibido": "primary", "diagnosticando": "warning",
	"presupuestado": "secondary", "aprobado": "info",
	"reparando": "purple", "reparado": "success",
	"entregado": "success", "cancelado": "danger",
}

var statusIcons = map[string]string{
	"recibido": "inbox", "diagnosticando": "search",
	"presupuestado": "cash", "aprobado": "check-circle",
	"reparando": "tools", "reparado": "check-all",
	"entregado": "thumbs-up", "cancelado": "x-circle",
}

var statusEmojis = map[string]string{
	"recibido": "📥", "diagnosticando": "🔍",
	"presupuestado": "💰", "aprobado": "✅",
	"reparando": "🔧", "reparado": "✨",
	"entregado": "📦", "cancelado": "❌",
}

var tipoEquipoIcons = map[string]string{
	"laptop": "laptop", "pc": "pc", "celular": "phone", "audio": "speaker", "otro": "box",
}

var statusSequence = []string{"recibido", "diagnosticando", "presupuestado", "aprobado", "reparando", "reparado", "entregado"}

// ─── Main ──────────────────────────────────────────────────────

func main() {
	var err error
	loadCSRFKey()

	dbPath := os.Getenv("REFACCIONES_DB")
	if dbPath == "" {
		dbPath = DB_PATH
	}
	db, err = sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	initSchema()
	initTemplates()

	mux := http.NewServeMux()
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))
	mux.HandleFunc("/taller", func(w http.ResponseWriter, r *http.Request) {
		d, err := staticFS.ReadFile("static/tallerpro.html")
		if err != nil {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(d)
	})

	// Auth
	mux.HandleFunc("GET /login", handleLoginPage)
	mux.HandleFunc("POST /login", handleLogin)
	mux.HandleFunc("POST /logout", handleLogout)

	// Dashboard & repairs
	mux.HandleFunc("GET /{$}", requireAuth(handleDashboard))
	mux.HandleFunc("GET /kanban", requireAuth(handleKanban))
	mux.HandleFunc("GET /nueva", requireAuth(handleNuevaPage))
	mux.HandleFunc("POST /nueva", requireAuth(handleNuevaCrear))
	mux.HandleFunc("GET /reparacion/{id}", requireAuth(handleDetalle))
	mux.HandleFunc("POST /reparacion/{id}/status", requireAuth(handleStatusChange))
	mux.HandleFunc("POST /reparacion/{id}/editar", requireAuth(handleEditar))
	mux.HandleFunc("POST /reparacion/{id}/piezas", requireAuth(handleAgregarPieza))
	mux.HandleFunc("DELETE /reparacion/{id}/piezas/{pid}", requireAuth(handleQuitarPieza))
	mux.HandleFunc("POST /reparacion/{id}/archivos", requireAuth(handleSubirArchivo))
	mux.HandleFunc("DELETE /reparacion/{id}/archivos/{aid}", requireAuth(handleQuitarArchivo))
	mux.HandleFunc("POST /upload", requireAuth(handleUpload))
	mux.HandleFunc("GET /r/{token}", handlePublicPortal)

	// Uploads dir
	os.MkdirAll("uploads", 0755)
	mux.Handle("GET /uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir("uploads"))))

	log.Printf("📋 Reparaciones Dashboard en http://localhost:%s", PORT)
	log.Fatal(http.ListenAndServe(":"+PORT, withCSRF(mux)))
}

// ─── Schema ────────────────────────────────────────────────────

func initSchema() {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS marcas (id INTEGER PRIMARY KEY AUTOINCREMENT, nombre TEXT NOT NULL UNIQUE, activa INTEGER DEFAULT 1)`,
		`INSERT OR IGNORE INTO marcas (nombre) VALUES ('Apple'),('Samsung'),('Dell'),('HP'),('Lenovo'),('ASUS'),('Acer'),('Xiaomi'),('Motorola'),('Huawei'),('Sony'),('LG'),('Microsoft'),('JBL'),('Bose'),('Otras')`,
		`CREATE TABLE IF NOT EXISTS piezas (id INTEGER PRIMARY KEY AUTOINCREMENT, codigo TEXT NOT NULL UNIQUE, nombre TEXT NOT NULL, precio REAL NOT NULL DEFAULT 0, stock INTEGER NOT NULL DEFAULT 0, activa INTEGER DEFAULT 1)`,
		`CREATE TABLE IF NOT EXISTS usuarios (id INTEGER PRIMARY KEY AUTOINCREMENT, nombre_completo TEXT, usuario TEXT NOT NULL UNIQUE, clave TEXT NOT NULL, rol TEXT DEFAULT 'admin', activo TEXT DEFAULT 't')`,
		`CREATE TABLE IF NOT EXISTS sessions (id TEXT PRIMARY KEY, user_id INTEGER NOT NULL REFERENCES usuarios(id), created_at TEXT DEFAULT (datetime('now','localtime')), expires_at TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS config (clave TEXT PRIMARY KEY, valor TEXT NOT NULL DEFAULT '')`,
		`INSERT OR IGNORE INTO config (clave, valor) VALUES ('negocio_nombre','Taller'),('telefono','')`,
		`CREATE TABLE IF NOT EXISTS reparaciones (id INTEGER PRIMARY KEY AUTOINCREMENT, folio INTEGER NOT NULL, token TEXT NOT NULL UNIQUE DEFAULT (lower(hex(randomblob(16)))), tipo_equipo TEXT NOT NULL DEFAULT 'laptop', marca_id INTEGER REFERENCES marcas(id), modelo_texto TEXT, numero_serie TEXT, imei TEXT, password_equipo TEXT, condicion_fisica TEXT, cliente_nombre TEXT NOT NULL, cliente_telefono TEXT, cliente_email TEXT, falla_reportada TEXT NOT NULL, diagnostico TEXT, accesorios TEXT, notas_cliente TEXT, notas_internas TEXT, status TEXT NOT NULL DEFAULT 'recibido', fecha_ingreso TEXT DEFAULT (datetime('now','localtime')), fecha_prometida TEXT, fecha_entrega TEXT, costo_diagnostico REAL DEFAULT 0, costo_reparacion REAL DEFAULT 0, total REAL DEFAULT 0, anticipo REAL DEFAULT 0, tecnico_id INTEGER REFERENCES usuarios(id), activo INTEGER DEFAULT 1)`,
		`CREATE TABLE IF NOT EXISTS reparaciones_piezas (id INTEGER PRIMARY KEY AUTOINCREMENT, reparacion_id INTEGER NOT NULL REFERENCES reparaciones(id) ON DELETE CASCADE, pieza_id INTEGER NOT NULL REFERENCES piezas(id), cantidad INTEGER NOT NULL DEFAULT 1, precio_unitario REAL NOT NULL, subtotal REAL NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS reparaciones_archivos (id INTEGER PRIMARY KEY AUTOINCREMENT, reparacion_id INTEGER NOT NULL REFERENCES reparaciones(id) ON DELETE CASCADE, nombre TEXT NOT NULL, url TEXT NOT NULL, tipo TEXT DEFAULT 'foto', created_at TEXT DEFAULT (datetime('now','localtime')))`,
		`CREATE TABLE IF NOT EXISTS reparaciones_historial (id INTEGER PRIMARY KEY AUTOINCREMENT, reparacion_id INTEGER NOT NULL REFERENCES reparaciones(id) ON DELETE CASCADE, status_anterior TEXT, status_nuevo TEXT NOT NULL, usuario_id INTEGER REFERENCES usuarios(id), notas TEXT, creado_en TEXT DEFAULT (datetime('now','localtime')))`,
		`CREATE INDEX IF NOT EXISTS idx_rep_status ON reparaciones(status)`,
		`CREATE INDEX IF NOT EXISTS idx_rep_token ON reparaciones(token)`,
		`CREATE INDEX IF NOT EXISTS idx_rep_historial_rep ON reparaciones_historial(reparacion_id)`,
	}
	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			log.Printf("Schema: %v", err)
		}
	}
	var count int
	db.QueryRow("SELECT COUNT(*) FROM usuarios").Scan(&count)
	if count == 0 {
		db.Exec("INSERT INTO usuarios (usuario, clave, nombre_completo, rol) VALUES (?, ?, ?, ?)",
			"admin", hashPassword("admin"), "Administrador", "admin")
	}
}

// ─── Templates ─────────────────────────────────────────────────

func initTemplates() {
	templates = make(map[string]*template.Template)
	funcMap := template.FuncMap{
		"statusLabel": func(s string) string { return mapOr(s, statusLabels, s) },
		"statusColor": func(s string) string { return mapOr(s, statusColors, "secondary") },
		"statusIcon":  func(s string) string { return mapOr(s, statusIcons, "circle") },
		"statusEmoji": func(s string) string { return mapOr(s, statusEmojis, "") },
		"tipoIcon":    func(s string) string { return mapOr(s, tipoEquipoIcons, "box") },
		"truncate": func(s string, n int) string {
			r := []rune(s)
			if len(r) <= n {
				return s
			}
			return string(r[:n]) + "..."
		},
		"seq": func(n int) []int {
			s := make([]int, n)
			for i := range s {
				s[i] = i
			}
			return s
		},
		"add": func(a, b int) int { return a + b },
		"sub": func(a, b int) int { return a - b },
		"mul": func(a, b int) int { return a * b },
	}

	base := readFile("templates/base.html")
	adminPages := []string{"dashboard.html", "kanban.html", "ingreso.html", "detalle.html"}
	for _, page := range adminPages {
		t := template.New(page).Funcs(funcMap)
		t.Parse(base)
		t.Parse(readFile("templates/" + page))
		templates[page] = t
	}

	// Public portal
	portalBase := readFile("templates/portal_base.html")
	t := template.New("portal.html").Funcs(funcMap)
	t.Parse(portalBase)
	t.Parse(readFile("templates/portal.html"))
	templates["portal.html"] = t
}

func readFile(path string) string {
	d, err := templateFS.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(d)
}

func render(w io.Writer, name string, data PageData) {
	data.CSRFToken = generateCSRFToken()
	tmpl := templates[name]
	if tmpl == nil {
		return
	}
	tmpl.ExecuteTemplate(w, "layout", data)
}

// ─── Auth ──────────────────────────────────────────────────────

func hashPassword(pw string) string {
	h := sha256.Sum256([]byte(pw))
	return hex.EncodeToString(h[:])
}

func loadCSRFKey() {
	keyPath := ".csrfkey"
	d, err := os.ReadFile(keyPath)
	if err != nil {
		b := make([]byte, 32)
		rand.Read(b)
		d = []byte(hex.EncodeToString(b))
		os.WriteFile(keyPath, d, 0644)
	}
	csrfKey = d
}

func generateCSRFToken() string {
	csrfMux.Lock()
	defer csrfMux.Unlock()
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func withCSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" || r.Method == "PUT" || r.Method == "DELETE" {
			if r.Header.Get("X-CSRF-Token") == "" && r.FormValue("_csrf") == "" {
				http.Error(w, "CSRF required", http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if getSession(r) == nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		next(w, r)
	}
}

func getSession(r *http.Request) *SessionData {
	c, _ := r.Cookie("session")
	if c == nil || c.Value == "" {
		return nil
	}
	var s SessionData
	err := db.QueryRow("SELECT s.user_id, u.usuario, u.rol FROM sessions s JOIN usuarios u ON u.id = s.user_id WHERE s.id = ? AND s.expires_at > datetime('now','localtime')", c.Value).Scan(&s.UserID, &s.Username, &s.Role)
	if err != nil {
		return nil
	}
	return &s
}

func sessionUser(r *http.Request) int {
	s := getSession(r)
	if s != nil {
		return s.UserID
	}
	return 0
}

func handleLoginPage(w http.ResponseWriter, r *http.Request) {
	loginTpl := template.Must(template.New("login.html").Parse(readFile("templates/login.html")))
	err := r.URL.Query().Get("error")
	loginTpl.Execute(w, map[string]string{"Error": err})
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	u, p := r.FormValue("usuario"), r.FormValue("clave")
	var id int
	var hash string
	err := db.QueryRow("SELECT id, clave FROM usuarios WHERE usuario = ? AND activo = 't'", u).Scan(&id, &hash)
	if err != nil || !verifyPassword(p, hash) {
		http.Redirect(w, r, "/login?error=1", http.StatusSeeOther)
		return
	}
	b := make([]byte, 16)
	rand.Read(b)
	tok := hex.EncodeToString(b)
	db.Exec("INSERT INTO sessions (id, user_id, expires_at) VALUES (?, ?, datetime('now','+24 hours'))", tok, id)
	http.SetCookie(w, &http.Cookie{Name: "session", Value: tok, Path: "/", HttpOnly: true, MaxAge: 86400})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func handleLogout(w http.ResponseWriter, r *http.Request) {
	c, _ := r.Cookie("session")
	if c != nil {
		db.Exec("DELETE FROM sessions WHERE id = ?", c.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: "session", Value: "", Path: "/", MaxAge: -1})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func verifyPassword(pw, hash string) bool {
	return hashPassword(pw) == hash
}

// ─── Handlers ──────────────────────────────────────────────────

func handleDashboard(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	s := r.URL.Query().Get("s")

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

	rows, _ := db.Query(`SELECT r.id, r.folio, r.token, r.tipo_equipo,
		COALESCE(m.nombre,''), COALESCE(r.modelo_texto,''),
		r.cliente_nombre, COALESCE(r.cliente_telefono,''),
		r.status, r.fecha_ingreso, r.total
		FROM reparaciones r
		LEFT JOIN marcas m ON m.id = r.marca_id
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY r.fecha_ingreso DESC LIMIT 50`, args...)

	var rowsData []ReparacionRow
	for rows.Next() {
		var rd ReparacionRow
		rows.Scan(&rd.ID, &rd.Folio, &rd.Token, &rd.TipoEquipo,
			&rd.MarcaNombre, &rd.ModeloTexto,
			&rd.ClienteNombre, &rd.ClienteTelefono,
			&rd.Status, &rd.FechaIngreso, &rd.Total)
		rowsData = append(rowsData, rd)
	}
	rows.Close()

	// Get days in status for each
	for i := range rowsData {
		var days int
		db.QueryRow(`SELECT CAST(julianday('now') - julianday(COALESCE(
			(SELECT MAX(creado_en) FROM reparaciones_historial WHERE reparacion_id = ? AND status_nuevo = ?),
		fecha_ingreso)) AS INTEGER)`, rowsData[i].ID, rowsData[i].Status).Scan(&days)
		rowsData[i].DiasEnStatus = days
	}

	// Stats
	var st DashboardStats
	db.QueryRow(`SELECT
		COALESCE(SUM(CASE WHEN date(fecha_ingreso)=date('now') THEN 1 ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN status IN ('recibido','diagnosticando') THEN 1 ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN status='presupuestado' THEN 1 ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN status='reparado' THEN 1 ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN julianday('now')-julianday(fecha_ingreso)>7 AND status NOT IN ('entregado','cancelado') THEN 1 ELSE 0 END),0),
		COUNT(*),
		COALESCE(SUM(CASE WHEN status IN ('entregado','cancelado') AND strftime('%Y-%m',fecha_entrega)=strftime('%Y-%m','now') THEN 1 ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN status NOT IN ('entregado','cancelado') THEN total ELSE 0 END),0)
		FROM reparaciones WHERE activo=1`).Scan(
		&st.HoyRecibidos, &st.EnProceso, &st.EsperaAprobacion,
		&st.ListosEntrega, &st.Atrasados, &st.TotalActivos,
		&st.CompletadosMes, &st.ValorPendiente)

	pd := PageData{Title: "Dashboard", Active: "dashboard", Stats: st, Rows: rowsData, Query: q, StatusFilter: s}
	render(w, "dashboard.html", pd)
}

func handleKanban(w http.ResponseWriter, r *http.Request) {
	rows, _ := db.Query(`SELECT r.id, r.folio, r.token, r.tipo_equipo,
		COALESCE(m.nombre,''), COALESCE(r.modelo_texto,''),
		r.cliente_nombre, COALESCE(r.cliente_telefono,''),
		r.status, r.fecha_ingreso, r.total
		FROM reparaciones r
		LEFT JOIN marcas m ON m.id = r.marca_id
		WHERE r.activo = 1 AND r.status NOT IN ('entregado','cancelado')
		ORDER BY r.fecha_ingreso ASC`)
	defer rows.Close()

	byStatus := map[string][]ReparacionRow{}
	for rows.Next() {
		var rd ReparacionRow
		rows.Scan(&rd.ID, &rd.Folio, &rd.Token, &rd.TipoEquipo,
			&rd.MarcaNombre, &rd.ModeloTexto,
			&rd.ClienteNombre, &rd.ClienteTelefono,
			&rd.Status, &rd.FechaIngreso, &rd.Total)
		byStatus[rd.Status] = append(byStatus[rd.Status], rd)
	}

	pd := PageData{Title: "Kanban", Active: "kanban"}
	pd.Data = byStatus
	pd.StatusSequence = statusSequence
	render(w, "kanban.html", pd)
}

func handleNuevaPage(w http.ResponseWriter, r *http.Request) {
	pd := PageData{Title: "Nueva Reparación", Active: "nueva"}
	pd.Marcas = queryMarcas()
	if r.URL.Query().Get("success") != "" {
		pd.Success = "✓ Reparación registrada"
	}
	render(w, "ingreso.html", pd)
}

func handleNuevaCrear(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	mID, _ := strconv.Atoi(r.FormValue("marca_id"))

	var folio int
	db.QueryRow("SELECT COALESCE(MAX(folio),0)+1 FROM reparaciones").Scan(&folio)

	_, err := db.Exec(`INSERT INTO reparaciones (folio,tipo_equipo,marca_id,modelo_texto,numero_serie,imei,
		password_equipo,condicion_fisica,cliente_nombre,cliente_telefono,cliente_email,
		falla_reportada,diagnostico,accesorios,notas_cliente,notas_internas,
		status,fecha_prometida,costo_diagnostico,anticipo,tecnico_id)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		folio, r.FormValue("tipo_equipo"), mID, r.FormValue("modelo_texto"),
		r.FormValue("numero_serie"), r.FormValue("imei"), r.FormValue("password_equipo"),
		r.FormValue("condicion_fisica"), r.FormValue("cliente_nombre"),
		r.FormValue("cliente_telefono"), r.FormValue("cliente_email"),
		r.FormValue("falla_reportada"), r.FormValue("diagnostico"),
		r.FormValue("accesorios"), r.FormValue("notas_cliente"),
		r.FormValue("notas_internas"), "recibido",
		r.FormValue("fecha_prometida"), parseFloat(r.FormValue("costo_diagnostico")),
		parseFloat(r.FormValue("anticipo")), sessionUser(r))

	if err != nil {
		pd := PageData{Title: "Nueva Reparación", Active: "nueva", Error: err.Error()}
		pd.Marcas = queryMarcas()
		render(w, "ingreso.html", pd)
		return
	}
	if r.FormValue("_save_and_new") == "1" {
		http.Redirect(w, r, "/nueva?success=1", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func handleDetalle(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	rep := queryRepFull(id)
	if rep.ID == 0 {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	pd := PageData{Title: "Rep #" + strconv.Itoa(rep.Folio), Active: "detalle", Rep: rep, StatusSequence: statusSequence}
	pd.Marcas = queryMarcas()
	pd.Piezas = queryPiezas()
	render(w, "detalle.html", pd)
}

func handleStatusChange(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	r.ParseForm()
	ns := r.FormValue("status")
	notas := r.FormValue("notas")
	if ns == "" {
		json.NewEncoder(w).Encode(map[string]any{"error": "status requerido"})
		return
	}
	var old string
	db.QueryRow("SELECT status FROM reparaciones WHERE id=?", id).Scan(&old)
	if old == "" {
		json.NewEncoder(w).Encode(map[string]any{"error": "no encontrada"})
		return
	}
	if ns == "entregado" || ns == "cancelado" {
		db.Exec("UPDATE reparaciones SET status=?, fecha_entrega=datetime('now','localtime') WHERE id=?", ns, id)
	} else {
		db.Exec("UPDATE reparaciones SET status=? WHERE id=?", ns, id)
	}
	db.Exec("INSERT INTO reparaciones_historial (reparacion_id,status_anterior,status_nuevo,usuario_id,notas) VALUES(?,?,?,?,?)",
		id, old, ns, sessionUser(r), notas)
	json.NewEncoder(w).Encode(map[string]any{"success": true, "status": ns})
}

func handleEditar(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	r.ParseForm()
	db.Exec(`UPDATE reparaciones SET cliente_nombre=?,cliente_telefono=?,cliente_email=?,
		modelo_texto=?,numero_serie=?,imei=?,password_equipo=?,condicion_fisica=?,
		falla_reportada=?,diagnostico=?,accesorios=?,notas_cliente=?,notas_internas=?,
		fecha_prometida=?,costo_diagnostico=?,anticipo=?,total=?
		WHERE id=?`,
		r.FormValue("cliente_nombre"), r.FormValue("cliente_telefono"), r.FormValue("cliente_email"),
		r.FormValue("modelo_texto"), r.FormValue("numero_serie"), r.FormValue("imei"),
		r.FormValue("password_equipo"), r.FormValue("condicion_fisica"),
		r.FormValue("falla_reportada"), r.FormValue("diagnostico"), r.FormValue("accesorios"),
		r.FormValue("notas_cliente"), r.FormValue("notas_internas"),
		r.FormValue("fecha_prometida"), parseFloat(r.FormValue("costo_diagnostico")),
		parseFloat(r.FormValue("anticipo")), parseFloat(r.FormValue("total")), id)
	http.Redirect(w, r, "/reparacion/"+strconv.Itoa(id), http.StatusSeeOther)
}

func handleAgregarPieza(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	r.ParseForm()
	pid, _ := strconv.Atoi(r.FormValue("pieza_id"))
	cant, _ := strconv.Atoi(r.FormValue("cantidad"))
	if cant < 1 {
		cant = 1
	}
	var precio float64
	var nombre string
	db.QueryRow("SELECT precio, nombre FROM piezas WHERE id=? AND activa=1", pid).Scan(&precio, &nombre)
	if precio == 0 && nombre == "" {
		json.NewEncoder(w).Encode(map[string]any{"error": "Pieza no encontrada"})
		return
	}
	sub := precio * float64(cant)
	_, err := db.Exec("INSERT INTO reparaciones_piezas (reparacion_id,pieza_id,cantidad,precio_unitario,subtotal) VALUES(?,?,?,?,?)",
		id, pid, cant, precio, sub)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
		return
	}
	db.Exec("UPDATE piezas SET stock=stock-? WHERE id=?", cant, pid)
	db.Exec(`UPDATE reparaciones SET total=(SELECT COALESCE(SUM(subtotal),0) FROM reparaciones_piezas WHERE reparacion_id=?)+costo_diagnostico WHERE id=?`, id, id)
	json.NewEncoder(w).Encode(map[string]any{"success": true})
}

func handleQuitarPieza(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	pid, _ := strconv.Atoi(r.PathValue("pid"))
	var piezaID, cant int
	db.QueryRow("SELECT pieza_id, cantidad FROM reparaciones_piezas WHERE id=? AND reparacion_id=?", pid, id).Scan(&piezaID, &cant)
	db.Exec("UPDATE piezas SET stock=stock+? WHERE id=?", cant, piezaID)
	db.Exec("DELETE FROM reparaciones_piezas WHERE id=? AND reparacion_id=?", pid, id)
	db.Exec(`UPDATE reparaciones SET total=(SELECT COALESCE(SUM(subtotal),0) FROM reparaciones_piezas WHERE reparacion_id=?)+costo_diagnostico WHERE id=?`, id, id)
	json.NewEncoder(w).Encode(map[string]any{"success": true})
}

func handleSubirArchivo(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		json.NewEncoder(w).Encode(map[string]any{"error": "Archivo muy grande"})
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		json.NewEncoder(w).Encode(map[string]any{"error": "No se recibió archivo"})
		return
	}
	defer file.Close()
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" && ext != ".webp" && ext != ".gif" && ext != ".pdf" {
		json.NewEncoder(w).Encode(map[string]any{"error": "Formato no permitido"})
		return
	}
	b := make([]byte, 8)
	rand.Read(b)
	fn := fmt.Sprintf("rep%d_%s%s", id, hex.EncodeToString(b), ext)
	dst, _ := os.Create(filepath.Join("uploads", fn))
	defer dst.Close()
	io.Copy(dst, file)
	url := "/uploads/" + fn
	tipo := "foto"
	if ext == ".pdf" {
		tipo = "pdf"
	}
	db.Exec("INSERT INTO reparaciones_archivos (reparacion_id,nombre,url,tipo) VALUES(?,?,?,?)", id, header.Filename, url, tipo)
	json.NewEncoder(w).Encode(map[string]any{"success": true, "url": url})
}

func handleQuitarArchivo(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	aid, _ := strconv.Atoi(r.PathValue("aid"))
	var url string
	db.QueryRow("SELECT url FROM reparaciones_archivos WHERE id=? AND reparacion_id=?", aid, id).Scan(&url)
	if url != "" {
		os.Remove("." + url)
	}
	db.Exec("DELETE FROM reparaciones_archivos WHERE id=? AND reparacion_id=?", aid, id)
	json.NewEncoder(w).Encode(map[string]any{"success": true})
}

func handleUpload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 5<<20)
	if err := r.ParseMultipartForm(5 << 20); err != nil {
		json.NewEncoder(w).Encode(map[string]any{"error": "Archivo muy grande"})
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		json.NewEncoder(w).Encode(map[string]any{"error": "No se recibió archivo"})
		return
	}
	defer file.Close()
	ext := strings.ToLower(filepath.Ext(header.Filename))
	b := make([]byte, 8)
	rand.Read(b)
	fn := fmt.Sprintf("%d_%s%s", time.Now().UnixMilli(), hex.EncodeToString(b), ext)
	dst, _ := os.Create(filepath.Join("uploads", fn))
	defer dst.Close()
	io.Copy(dst, file)
	json.NewEncoder(w).Encode(map[string]any{"url": "/uploads/" + fn})
}

func handlePublicPortal(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if len(token) < 10 {
		render(w, "portal.html", PageData{Error: "Código de reparación inválido"})
		return
	}

	var rep Reparacion
	err := db.QueryRow(`SELECT r.id, r.folio, r.token, r.tipo_equipo,
		COALESCE(m.nombre,''), COALESCE(r.modelo_texto,''),
		COALESCE(r.cliente_nombre,''), COALESCE(r.cliente_telefono,''),
		COALESCE(r.falla_reportada,''), COALESCE(r.diagnostico,''),
		COALESCE(r.notas_cliente,''),
		r.status, r.fecha_ingreso, COALESCE(r.fecha_prometida,''), COALESCE(r.fecha_entrega,''),
		r.costo_diagnostico, r.total, r.anticipo
		FROM reparaciones r
		LEFT JOIN marcas m ON m.id = r.marca_id
		WHERE r.token=? AND r.activo=1`, token).Scan(
		&rep.ID, &rep.Folio, &rep.Token, &rep.TipoEquipo,
		&rep.MarcaNombre, &rep.ModeloTexto,
		&rep.ClienteNombre, &rep.ClienteTelefono,
		&rep.FallaReportada, &rep.Diagnostico,
		&rep.NotasCliente,
		&rep.Status, &rep.FechaIngreso, &rep.FechaPrometida, &rep.FechaEntrega,
		&rep.CostoDiagnostico, &rep.Total, &rep.Anticipo)

	if err != nil {
		render(w, "portal.html", PageData{Error: "No encontramos ninguna reparación con ese código."})
		return
	}

	rows, _ := db.Query(`SELECT status_anterior, status_nuevo, COALESCE(notas,''), creado_en
		FROM reparaciones_historial WHERE reparacion_id=? ORDER BY creado_en ASC`, rep.ID)
	for rows.Next() {
		var h HistorialEntry
		rows.Scan(&h.StatusAnterior, &h.StatusNuevo, &h.Notas, &h.CreadoEn)
		rep.Historial = append(rep.Historial, h)
	}
	rows.Close()

	telefono := getConfig("telefono")
	negocio := getConfig("negocio_nombre")
	pd := PageData{Title: "Reparación #" + strconv.Itoa(rep.Folio), Rep: rep, Telefono: telefono, Negocio: negocio}
	render(w, "portal.html", pd)
}

// ─── Queries ───────────────────────────────────────────────────

func queryRepFull(id int) Reparacion {
	var r Reparacion
	err := db.QueryRow(`SELECT r.id, r.folio, r.token, r.tipo_equipo,
		COALESCE(m.nombre,''), COALESCE(r.modelo_texto,''),
		COALESCE(r.numero_serie,''), COALESCE(r.imei,''), COALESCE(r.password_equipo,''),
		COALESCE(r.condicion_fisica,''),
		COALESCE(r.cliente_nombre,''), COALESCE(r.cliente_telefono,''), COALESCE(r.cliente_email,''),
		COALESCE(r.falla_reportada,''), COALESCE(r.diagnostico,''), COALESCE(r.accesorios,''),
		COALESCE(r.notas_cliente,''), COALESCE(r.notas_internas,''),
		r.status, r.fecha_ingreso, COALESCE(r.fecha_prometida,''), COALESCE(r.fecha_entrega,''),
		r.costo_diagnostico, r.costo_reparacion, r.total, r.anticipo,
		COALESCE(u.nombre_completo,'')
		FROM reparaciones r
		LEFT JOIN marcas m ON m.id=r.marca_id
		LEFT JOIN usuarios u ON u.id=r.tecnico_id
		WHERE r.id=?`, id).Scan(
		&r.ID, &r.Folio, &r.Token, &r.TipoEquipo,
		&r.MarcaNombre, &r.ModeloTexto,
		&r.NumeroSerie, &r.IMEI, &r.PasswordEquipo,
		&r.CondicionFisica,
		&r.ClienteNombre, &r.ClienteTelefono, &r.ClienteEmail,
		&r.FallaReportada, &r.Diagnostico, &r.Accesorios,
		&r.NotasCliente, &r.NotasInternas,
		&r.Status, &r.FechaIngreso, &r.FechaPrometida, &r.FechaEntrega,
		&r.CostoDiagnostico, &r.CostoReparacion, &r.Total, &r.Anticipo,
		&r.TecnicoNombre)
	if err != nil {
		return r
	}

	rows, _ := db.Query(`SELECT rh.status_anterior, rh.status_nuevo, COALESCE(rh.notas,''), rh.creado_en, COALESCE(u.nombre_completo,'')
		FROM reparaciones_historial rh LEFT JOIN usuarios u ON u.id=rh.usuario_id
		WHERE rh.reparacion_id=? ORDER BY rh.creado_en ASC`, id)
	for rows.Next() {
		var h HistorialEntry
		rows.Scan(&h.StatusAnterior, &h.StatusNuevo, &h.Notas, &h.CreadoEn, &h.UsuarioNombre)
		r.Historial = append(r.Historial, h)
	}
	rows.Close()

	rows2, _ := db.Query(`SELECT rp.id, p.nombre, p.codigo, rp.cantidad, rp.precio_unitario, rp.subtotal
		FROM reparaciones_piezas rp JOIN piezas p ON p.id=rp.pieza_id WHERE rp.reparacion_id=?`, id)
	for rows2.Next() {
		var p PiezaUsada
		rows2.Scan(&p.ID, &p.PiezaNombre, &p.PiezaCodigo, &p.Cantidad, &p.PrecioUnitario, &p.Subtotal)
		r.PiezasUsadas = append(r.PiezasUsadas, p)
	}
	rows2.Close()

	rows3, _ := db.Query("SELECT id, nombre, url, tipo FROM reparaciones_archivos WHERE reparacion_id=? ORDER BY created_at DESC", id)
	for rows3.Next() {
		var a Archivo
		rows3.Scan(&a.ID, &a.Nombre, &a.URL, &a.Tipo)
		r.Archivos = append(r.Archivos, a)
	}
	rows3.Close()

	return r
}

func queryMarcas() []MarcaOption {
	rows, _ := db.Query("SELECT id, nombre FROM marcas WHERE activa=1 ORDER BY nombre")
	defer rows.Close()
	var ms []MarcaOption
	for rows.Next() {
		var m MarcaOption
		rows.Scan(&m.ID, &m.Nombre)
		ms = append(ms, m)
	}
	return ms
}

func queryPiezas() []PiezaOption {
	rows, _ := db.Query("SELECT id, nombre, codigo, precio, stock FROM piezas WHERE activa=1 ORDER BY nombre")
	defer rows.Close()
	var ps []PiezaOption
	for rows.Next() {
		var p PiezaOption
		rows.Scan(&p.ID, &p.Nombre, &p.Codigo, &p.Precio, &p.Stock)
		ps = append(ps, p)
	}
	return ps
}

func getConfig(key string) string {
	var v string
	db.QueryRow("SELECT valor FROM config WHERE clave=?", key).Scan(&v)
	return v
}

func parseFloat(s string) float64 {
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

func mapOr[T any](key string, m map[string]T, def T) T {
	if v, ok := m[key]; ok {
		return v
	}
	return def
}
