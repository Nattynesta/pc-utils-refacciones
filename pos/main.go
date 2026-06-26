package main

import (
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	_ "modernc.org/sqlite"
)

//go:embed templates
var templateFS embed.FS

//go:embed static
var staticFS embed.FS

//go:embed schema.sql
var schemaSQL string

const DB_PATH = "refacciones.db"

type PageData struct {
	Title     string
	Active    string
	User      string
	UserID    int
	Role      string
	Error     string
	Success   string
	CSRFToken string
	Theme     string
	Query     string

	// Common dynamic data
	Categorias []Categoria
	Marcas     []Marca
	Piezas     []PiezaResumen
	Modelos    []Modelo
	Recientes  []PiezaResumen
	Stats      DashboardStats
	Ventas     []Venta
	Config     map[string]string
	Pieza      Pieza
	TotalPages int
	Total      int
	Page       int
	CategoriaSlug string
	MarcaNombre   string
	ModeloNombre  string
	Estado     string
	PrecioMax  string
	AñoLanzamiento int
	MarcaID    string
	Compatibilidades []any
	Telefono   string
	Negocio    string
	// Generic data fallback
	Data       any
}

var csrfKey []byte

func csrfToken() string {
	return generateCSRFToken()
}

func anySlice[T any](s []T) []any {
	r := make([]any, len(s))
	for i, v := range s { r[i] = v }
	return r
}

func main() {
	loadCSRFKey()

	var err error
	db, err = sql.Open("sqlite", DB_PATH+"?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := initSchema(db); err != nil {
		log.Fatal(err)
	}

	seedInitialData(db)

	initTemplates()

	mux := http.NewServeMux()

	// Static files
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	// Auth
	mux.HandleFunc("GET /login", handleLoginPage)
	mux.HandleFunc("POST /login", handleLogin)
	mux.HandleFunc("POST /logout", handleLogout)

	// Public customer portal (no auth)
	mux.HandleFunc("GET /", handlePortalHome)
	mux.HandleFunc("GET /buscar", handlePortalBuscar)
	mux.HandleFunc("GET /modelo/{modelo}", handlePortalModelo)
	mux.HandleFunc("GET /pieza/{id}", handlePortalPieza)
	mux.HandleFunc("GET /api/piezas", handleAPIPiezas)
	mux.HandleFunc("GET /api/modelos", handleAPIModelos)
	mux.HandleFunc("GET /api/marcas", handleAPIMarcas)
	mux.HandleFunc("GET /api/categorias", handleAPICategorias)

	// Admin
	mux.HandleFunc("GET /admin", requireAuth(handleAdminDashboard))
	mux.HandleFunc("GET /admin/piezas", requireAuth(handleAdminPiezasList))
	mux.HandleFunc("GET /admin/piezas/nueva", requireAuth(handleAdminPiezaNuevaPage))
	mux.HandleFunc("POST /admin/piezas", requireAuth(handleAdminPiezaCrear))
	mux.HandleFunc("GET /admin/piezas/{id}/editar", requireAuth(handleAdminPiezaEditarPage))
	mux.HandleFunc("PUT /admin/piezas/{id}", requireAuth(handleAdminPiezaActualizar))
	mux.HandleFunc("DELETE /admin/piezas/{id}", requireAuth(handleAdminPiezaEliminar))
	mux.HandleFunc("GET /admin/modelos", requireAuth(handleAdminModelosList))
	mux.HandleFunc("POST /admin/modelos", requireAuth(handleAdminModeloCrear))
	mux.HandleFunc("GET /admin/compatibilidad", requireAuth(handleAdminCompatibilidadPage))
	mux.HandleFunc("POST /admin/compatibilidad", requireAuth(handleAdminCompatibilidadGuardar))
	mux.HandleFunc("GET /admin/stats", requireAuth(handleAdminStats))
	mux.HandleFunc("POST /admin/ventas", requireAuth(handleAdminVentaCrear))
	mux.HandleFunc("GET /admin/ventas", requireAuth(handleAdminVentasList))
	mux.HandleFunc("GET /admin/config", requireAuth(handleAdminConfigPage))
	mux.HandleFunc("POST /admin/config", requireAuth(handleAdminConfigGuardar))

	// Uploads
	os.MkdirAll("uploads", 0755)
	mux.Handle("GET /uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir("uploads"))))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}
	log.Printf("Refacciones POS corriendo en http://localhost:%s", port)
	log.Fatal(http.ListenAndServe(":"+port, withCSRF(mux)))
}

func seedInitialData(db *sql.DB) {
	// Check if admin user exists
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM usuarios").Scan(&count)
	if err != nil || count > 0 {
		return
	}
	db.Exec("INSERT INTO usuarios (usuario, clave, nombre_completo, rol) VALUES (?, ?, ?, ?)",
		"admin", hashPassword("admin"), "Administrador", "admin")
	// Seed test data: some popular models
	db.Exec("INSERT OR IGNORE INTO modelos (marca_id, nombre, año_lanzamiento) VALUES ((SELECT id FROM marcas WHERE nombre='Apple'), 'iPhone 12', 2020)")
	db.Exec("INSERT OR IGNORE INTO modelos (marca_id, nombre, año_lanzamiento) VALUES ((SELECT id FROM marcas WHERE nombre='Apple'), 'iPhone 13', 2021)")
	db.Exec("INSERT OR IGNORE INTO modelos (marca_id, nombre, año_lanzamiento) VALUES ((SELECT id FROM marcas WHERE nombre='Apple'), 'iPhone 14', 2022)")
	db.Exec("INSERT OR IGNORE INTO modelos (marca_id, nombre, año_lanzamiento) VALUES ((SELECT id FROM marcas WHERE nombre='Apple'), 'iPhone 15', 2023)")
	db.Exec("INSERT OR IGNORE INTO modelos (marca_id, nombre, año_lanzamiento) VALUES ((SELECT id FROM marcas WHERE nombre='Samsung'), 'Galaxy S23', 2023)")
	db.Exec("INSERT OR IGNORE INTO modelos (marca_id, nombre, año_lanzamiento) VALUES ((SELECT id FROM marcas WHERE nombre='Samsung'), 'Galaxy S24', 2024)")
	db.Exec("INSERT OR IGNORE INTO modelos (marca_id, nombre, año_lanzamiento) VALUES ((SELECT id FROM marcas WHERE nombre='Xiaomi'), 'Redmi Note 12', 2023)")
	db.Exec("INSERT OR IGNORE INTO modelos (marca_id, nombre, año_lanzamiento) VALUES ((SELECT id FROM marcas WHERE nombre='Motorola'), 'Moto G84', 2023)")
}

func parseForm(r *http.Request) error {
	ctype := r.Header.Get("Content-Type")
	if strings.Contains(ctype, "application/json") {
		return nil
	}
	return r.ParseForm()
}

func formValue(r *http.Request, key string) string {
	if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		var data map[string]any
		json.NewDecoder(r.Body).Decode(&data)
		if v, ok := data[key]; ok {
			return fmt.Sprintf("%v", v)
		}
		return ""
	}
	return r.FormValue(key)
}
