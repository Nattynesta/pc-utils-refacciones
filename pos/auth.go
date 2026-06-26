package main

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"net/http"
	"os"
	"strings"
)

var db *sql.DB

type SessionData struct {
	UserID   int
	Username string
	Role     string
}

func loadCSRFKey() {
	keyPath := ".csrfkey"
	data, err := os.ReadFile(keyPath)
	if err != nil {
		key := make([]byte, 32)
		rand.Read(key)
		csrfKey = key
		os.WriteFile(keyPath, []byte(hex.EncodeToString(key)), 0600)
	} else {
		csrfKey, err = hex.DecodeString(strings.TrimSpace(string(data)))
		if err != nil {
			key := make([]byte, 32)
			rand.Read(key)
			csrfKey = key
		}
	}
}

func generateCSRFToken() string {
	mac := hmac.New(sha256.New, csrfKey)
	mac.Write([]byte("csrf-token"))
	return hex.EncodeToString(mac.Sum(nil))
}

func validateCSRFToken(token string) bool {
	mac := hmac.New(sha256.New, csrfKey)
	mac.Write([]byte("csrf-token"))
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(token), []byte(expected))
}

func withCSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cross := []string{"/api/tickets", "/api/chat/"}
		for _, p := range cross {
			if strings.HasPrefix(r.URL.Path, p) {
				next.ServeHTTP(w, r)
				return
			}
		}
		if r.Method == "POST" || r.Method == "PUT" || r.Method == "DELETE" || r.Method == "PATCH" {
			tok := r.Header.Get("X-CSRF-Token")
			if tok == "" {
				tok = r.FormValue("_csrf")
			}
			if tok == "" || !validateCSRFToken(tok) {
				http.Error(w, "CSRF token inválido", http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func getSession(db *sql.DB, r *http.Request) *SessionData {
	c, err := r.Cookie("session")
	if err != nil {
		return nil
	}
	var s SessionData
	err = db.QueryRow(
		"SELECT user_id, usuario, rol FROM sessions s JOIN usuarios u ON u.id = s.user_id WHERE s.id = ? AND s.expires_at > datetime('now','localtime')",
		c.Value,
	).Scan(&s.UserID, &s.Username, &s.Role)
	if err != nil {
		return nil
	}
	return &s
}

func requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s := getSession(db, r)
		if s == nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		r = r.WithContext(context.WithValue(r.Context(), "session", s))
		next(w, r)
	}
}
