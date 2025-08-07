package api

import (
	"encoding/json"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/DSpeichert/netbootd/manifest"
	"github.com/DSpeichert/netbootd/store"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v2"
)

type Server struct {
	router     *mux.Router
	httpServer *http.Server

	logger zerolog.Logger
	store  *store.Store
}

// NewServer set up HTTP API server instance
// If authorization is passed, requires privileged operation callers to present Authorization header with this content.
func NewServer(store *store.Store, authorization string) (server *Server, err error) {
	r := mux.NewRouter()

	server = &Server{
		router: r,
		httpServer: &http.Server{
			Handler:        r,
			WriteTimeout:   10 * time.Second,
			ReadTimeout:    10 * time.Second,
			MaxHeaderBytes: 1 << 20,
			IdleTimeout:    10 * time.Second,
		},
		logger: log.With().Str("service", "api").Logger(),
		store:  store,
	}

	// custom server header
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Server", "netbootd")
			next.ServeHTTP(w, r)
		})
	})

	// custom logging middleware
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			next.ServeHTTP(w, r)
			stop := time.Now()
			server.logger.Info().
				Int64("latency", stop.Sub(start).Microseconds()).
				Str("ip", r.RemoteAddr).
				Str("uri", r.RequestURI).
				Str("method", r.Method).
				Msg("request completed")
		})
	})

	// GET /api/manifests/{id}
	r.HandleFunc("/api/manifests/{id}", func(w http.ResponseWriter, r *http.Request) {
		if authorization != r.Header.Get("Authorization") {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		vars := mux.Vars(r)
		m := store.Find(vars["id"])
		if m == nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		var b []byte
		if strings.Contains(r.Header.Get("Accept"), "application/json") {
			w.Header().Set("Content-Type", "applications/json")
			b, _ = json.Marshal(m)
		} else {
			w.Header().Set("Content-Type", "text/yaml")
			b, _ = yaml.Marshal(m)
		}
		w.WriteHeader(http.StatusOK)
		w.Write(b)
	}).Methods("GET")

	// GET /api/manifests
	r.HandleFunc("/api/manifests", func(w http.ResponseWriter, r *http.Request) {
		if authorization != r.Header.Get("Authorization") {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		var b []byte
		if strings.Contains(r.Header.Get("Accept"), "application/json") {
			w.Header().Set("Content-Type", "applications/json")
			b, _ = json.Marshal(store.GetAll())
		} else {
			w.Header().Set("Content-Type", "text/yaml")
			b, _ = yaml.Marshal(store.GetAll())
		}
		w.WriteHeader(http.StatusOK)
		w.Write(b)
	}).Methods("GET")

	// PUT /api/manifests/{id}
	r.HandleFunc("/api/manifests/{id}", func(w http.ResponseWriter, r *http.Request) {
		if authorization != r.Header.Get("Authorization") {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		buf, _ := ioutil.ReadAll(r.Body)
		var m manifest.Manifest
		if r.Header.Get("Content-Type") == "application/json" {
			err = json.Unmarshal(buf, &m)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		} else {
			m, err = manifest.ManifestFromYaml(buf)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		}
		_ = store.PutManifest(m)
		w.WriteHeader(http.StatusCreated)
	}).Methods("PUT")

	// DELETE /api/manifests/{id}
	r.HandleFunc("/api/manifests/{id}", func(w http.ResponseWriter, r *http.Request) {
		if authorization != r.Header.Get("Authorization") {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		vars := mux.Vars(r)
		store.ForgetManifest(vars["id"])

		w.WriteHeader(http.StatusNoContent)
	}).Methods("DELETE")

	// GET|POST /api/self/suspend-boot
	r.HandleFunc("/api/self/suspend-boot", func(w http.ResponseWriter, r *http.Request) {
		var ip net.IP
		if queryFirst(r, "spoof") != "" {
			if authorization != r.Header.Get("Authorization") {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
			ip = net.ParseIP(queryFirst(r, "spoof"))
		} else {
			host, _, _ := net.SplitHostPort(r.RemoteAddr)
			ip = net.ParseIP(host)
		}

		m := server.store.FindByIP(ip)
		if m == nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		m.Suspended = true

		w.WriteHeader(http.StatusOK)
	}).Methods("GET", "POST")

	// GET|POST /api/self/unsuspend-boot
	r.HandleFunc("/api/self/unsuspend-boot", func(w http.ResponseWriter, r *http.Request) {
		var ip net.IP
		if queryFirst(r, "spoof") != "" {
			if authorization != r.Header.Get("Authorization") {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
			ip = net.ParseIP(queryFirst(r, "spoof"))
		} else {
			host, _, _ := net.SplitHostPort(r.RemoteAddr)
			ip = net.ParseIP(host)
		}

		m := server.store.FindByIP(ip)
		if m == nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		m.Suspended = false

		w.WriteHeader(http.StatusOK)
	}).Methods("GET", "POST")

	// GET /api/self/manifest
	r.HandleFunc("/api/self/manifest", func(w http.ResponseWriter, r *http.Request) {
		var ip net.IP
		if queryFirst(r, "spoof") != "" {
			if authorization != r.Header.Get("Authorization") {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
			ip = net.ParseIP(queryFirst(r, "spoof"))
		} else {
			host, _, _ := net.SplitHostPort(r.RemoteAddr)
			ip = net.ParseIP(host)
		}

		m := server.store.FindByIP(ip)
		if m == nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		var b []byte
		if strings.Contains(r.Header.Get("Accept"), "application/json") {
			w.Header().Set("Content-Type", "applications/json")
			b, _ = json.Marshal(store.GetAll())
		} else {
			w.Header().Set("Content-Type", "text/yaml")
			b, _ = yaml.Marshal(store.GetAll())
		}
		w.WriteHeader(http.StatusOK)
		w.Write(b)
	}).Methods("GET")

	return server, nil
}

func (server *Server) Serve(l net.Listener) error {
	return server.httpServer.Serve(l)
}

func (server *Server) ServeTLS(l net.Listener, certFile string, keyFile string) error {
	return server.httpServer.ServeTLS(l, certFile, keyFile)
}

func queryFirst(r *http.Request, k string) string {
	keys, ok := r.URL.Query()[k]
	if !ok || len(keys[0]) < 1 {
		return ""
	}
	return keys[0]
}
