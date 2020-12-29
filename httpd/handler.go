package httpd

import (
	"bytes"
	_ "embed"
	mfest "github.com/DSpeichert/netbootd/manifest"
	"github.com/DSpeichert/netbootd/static"
	"github.com/Masterminds/sprig"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"text/template"
	"time"
)

type Handler struct {
	server *Server
}

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	raddr := net.ParseIP(ip)

	h.server.logger.Info().
		Str("path", r.RequestURI).
		Str("client", raddr.String()).
		Msg("incoming HTTP request")

	manifestRaddr := raddr
	spoofIPs, ok := r.URL.Query()["spoof"]
	if ok && len(spoofIPs[0]) > 0 {
		manifestRaddr = net.ParseIP(spoofIPs[0])
	}

	manifest := h.server.store.FindByIP(manifestRaddr)
	if manifest == nil {
		h.server.logger.Info().
			Str("path", r.RequestURI).
			Str("client", raddr.String()).
			Str("manifest_for", manifestRaddr.String()).
			Msg("no manifest for client")
		http.Error(w, "no manifest for client: "+raddr.String(), http.StatusNotFound)
		return
	}

	if manifest.Ipxe {
		f, err := static.Files.Open(strings.TrimLeft(r.URL.Path, "/"))
		if err == nil {
			fstat, _ := f.Stat()
			h.server.logger.Info().
				Err(err).
				Str("path", r.RequestURI).
				Str("client", raddr.String()).
				Str("manifest_for", manifestRaddr.String()).
				Msg("static download")

			http.ServeContent(w, r, fstat.Name(), fstat.ModTime(), f.(io.ReadSeeker))
			return
		}
	}

	mount, err := manifest.GetMount(r.URL.Path)
	if err != nil {
		h.server.logger.Error().
			Err(err).
			Str("path", r.URL.Path).
			Str("client", raddr.String()).
			Str("manifest_for", manifestRaddr.String()).
			Msg("cannot find mount")

		http.NotFound(w, r)
		return
	}

	h.server.logger.Trace().
		Interface("mount", mount).
		Msg("found mount")

	if mount.Content != "" {
		tmpl, err := template.New("").Funcs(sprig.TxtFuncMap()).Parse(mount.Content)
		if err != nil {
			h.server.logger.Error().
				Err(err).
				Msg("failed to parse content template for mount")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		buf := new(bytes.Buffer)

		err = tmpl.Execute(buf, mfest.ContentContext{
			RemoteIP: raddr,
			HttpBaseUrl: &url.URL{
				Scheme: "http",
				Host:   r.Host,
			},
			Manifest: manifest,
		})
		if err != nil {
			h.server.logger.Error().
				Err(err).
				Msg("failed to execute content template for mount")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		http.ServeContent(w, r, mount.Path, time.Time{}, bytes.NewReader(buf.Bytes()))

		h.server.logger.Info().
			Err(err).
			Str("path", r.RequestURI).
			Str("client", raddr.String()).
			Str("manifest_for", manifestRaddr.String()).
			Msg("transfer finished")
	} else if mount.Proxy != "" {
		d, err := mount.ProxyDirector()
		if err != nil {
			h.server.logger.Error().
				Err(err).
				Msg("failed to parse proxy URL")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		rp := httputil.ReverseProxy{
			Director: d,
		}
		rp.ServeHTTP(w, r)
		return
	} else {
		// mount has neither .Path nor .Proxy defined
		h.server.logger.Error().
			Str("path", r.RequestURI).
			Str("client", raddr.String()).
			Str("manifest_for", manifestRaddr.String()).
			Str("mount", mount.Path).
			Msg("mount is empty")

		http.Error(w, "empty mount", http.StatusInternalServerError)
		return

	}

	return
}
