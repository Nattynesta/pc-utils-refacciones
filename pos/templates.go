package main

import (
	"html/template"
	"io"
	"log"
)

var templates map[string]*template.Template

func initTemplates() {
	templates = make(map[string]*template.Template)
	funcMap := template.FuncMap{
		"add": func(a, b int) int { return a + b },
		"sub": func(a, b int) int { return a - b },
		"mul": func(a, b int) int { return a * b },
		"seq": func(n int) []int {
			s := make([]int, n)
			for i := range s { s[i] = i + 1 }
			return s
		},
		"statusLabel": func(s string) string {
			if v, ok := StatusLabels[s]; ok { return v }
			return s
		},
		"statusColor": func(s string) string {
			if v, ok := StatusColors[s]; ok { return v }
			return "secondary"
		},
		"statusIcon": func(s string) string {
			if v, ok := StatusIcons[s]; ok { return v }
			return "circle"
		},
		"tipoIcon": func(s string) string {
			if v, ok := TipoEquipoIcons[s]; ok { return v }
			return "box"
		},
		"truncate": func(s string, n int) string {
			runes := []rune(s)
			if len(runes) <= n { return s }
			return string(runes[:n]) + "..."
		},
		"dict": func(values ...any) map[string]any {
			m := make(map[string]any)
			for i := 0; i < len(values); i += 2 {
				if i+1 < len(values) {
					if key, ok := values[i].(string); ok {
						m[key] = values[i+1]
					}
				}
			}
			return m
		},
	}

	// Load portal base once
	baseContent := readFile("templates/portal/base.html")
	adminBaseContent := readFile("templates/admin/base.html")
	adminLoginContent := readFile("templates/admin/login.html")

	// Portal pages: base + page
	portalPages := []string{"home.html", "buscar.html", "modelo.html", "pieza.html", "reparacion.html"}
	for _, page := range portalPages {
		pageContent := readFile("templates/portal/" + page)
		tmpl := template.New("portal_" + page).Funcs(funcMap)
		tmpl.Parse(baseContent)
		tmpl.Parse(pageContent)
		templates["portal/"+page] = tmpl
	}

	// Admin login is standalone
	tmpl := template.New("admin_login").Funcs(funcMap)
	tmpl.Parse(adminLoginContent)
	templates["admin/login.html"] = tmpl

	// Admin pages: base + page
	adminPages := []string{"dashboard.html", "piezas_list.html", "pieza_form.html", "modelos_list.html", "compatibilidad.html", "ventas_list.html", "config.html", "stats.html",
		"reparaciones_list.html", "reparaciones_kanban.html", "reparacion_form.html", "reparacion_detail.html"}
	for _, page := range adminPages {
		pageContent := readFile("templates/admin/" + page)
		tmpl := template.New("admin_" + page).Funcs(funcMap)
		tmpl.Parse(adminBaseContent)
		tmpl.Parse(pageContent)
		templates["admin/"+page] = tmpl
	}

	log.Printf("Loaded %d templates", len(templates))
}

func readFile(path string) string {
	data, err := templateFS.ReadFile(path)
	if err != nil {
		log.Printf("Warning: template %s not found: %v", path, err)
		return ""
	}
	return string(data)
}

func renderTemplate(w io.Writer, name string, data PageData) {
	tmpl, ok := templates[name]
	if !ok {
		log.Printf("Template %s not found", name)
		return
	}
	data.CSRFToken = generateCSRFToken()

	if name == "admin/login.html" {
		tmpl.Execute(w, data)
		return
	}

	tmpl.ExecuteTemplate(w, "base", data)
}
