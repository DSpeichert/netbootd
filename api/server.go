package api

import (
	"github.com/DSpeichert/netbootd/manifest"
	"github.com/DSpeichert/netbootd/store"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"net"
	"net/http"
	"time"
)

type Server struct {
	router     *mux.Router
	httpServer *http.Server

	logger zerolog.Logger
	store  *store.Store
}

func NewServer(store *store.Store) (server *Server, err error) {
	r := mux.NewRouter()

	server = &Server{
		router: r,
		httpServer: &http.Server{
			Handler: r,
			// Good practice: enforce timeouts for servers you create!
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

	r.HandleFunc("/api/manifests", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/yaml")
		w.WriteHeader(http.StatusOK)

		b, _ := yaml.Marshal(store.GetAll())
		w.Write(b)
	}).Methods("GET")

	r.HandleFunc("/api/manifests/{id}", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		m := store.Find(vars["id"])
		if m == nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/yaml")
		w.WriteHeader(http.StatusOK)
		b, _ := yaml.Marshal(m)
		w.Write(b)
	}).Methods("GET")

	r.HandleFunc("/api/manifests/{id}", func(w http.ResponseWriter, r *http.Request) {
		buf, _ := ioutil.ReadAll(r.Body)
		m, err := manifest.ManifestFromYaml(buf)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		_ = store.PutManifest(m)
		w.WriteHeader(http.StatusCreated)
	}).Methods("PUT")

	r.HandleFunc("/api/manifests/{id}", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		store.ForgetManifest(vars["id"])

		w.WriteHeader(http.StatusNoContent)
	}).Methods("DELETE")

	r.HandleFunc("/api/self/suspend-boot", func(w http.ResponseWriter, r *http.Request) {
		var ip net.IP
		if queryFirst(r, "spoof") != "" {
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
	})

	r.HandleFunc("/api/self/unsuspend-boot", func(w http.ResponseWriter, r *http.Request) {
		var ip net.IP
		if queryFirst(r, "spoof") != "" {
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
	})

	r.HandleFunc("/api/self/manifest", func(w http.ResponseWriter, r *http.Request) {
		var ip net.IP
		if queryFirst(r, "spoof") != "" {
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
		w.Header().Set("Content-Type", "text/yaml")
		w.WriteHeader(http.StatusOK)
		buf, _ := yaml.Marshal(m)
		w.Write(buf)
	})

	return server, nil
}

func (server *Server) Serve(l net.Listener) error {
	return server.httpServer.Serve(l)
}

func queryFirst(r *http.Request, k string) string {
	keys, ok := r.URL.Query()[k]
	if !ok || len(keys[0]) < 1 {
		return ""
	}
	return keys[0]
}
