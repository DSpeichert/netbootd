package httpd

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"strings"
	"text/template"
	"time"

	mfest "github.com/DSpeichert/netbootd/manifest"
	"github.com/DSpeichert/netbootd/static"
	"github.com/Masterminds/sprig/v3"
)

type Handler struct {
	server *Server
}

func parseIPFromHostPort(hostPort string) (net.IP, error) {
	host, _, err := net.SplitHostPort(hostPort)
	if err != nil {
		return nil, err
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return nil, fmt.Errorf("%s: unable to parse ip", host)
	}
	return ip, nil
}

func parseRemoteIP(r *http.Request) (net.IP, error) {
	return parseIPFromHostPort(r.RemoteAddr)
}

func parseLocalIP(r *http.Request) (net.IP, error) {
	lip, ok := r.Context().Value(http.LocalAddrContextKey).(net.Addr)
	if !ok {
		return nil, errors.New("local address not found in request context")
	}
	if lip == nil {
		return nil, errors.New("nil local address")
	}
	return parseIPFromHostPort(lip.String())
}

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	raddr, err := parseRemoteIP(r)
	if err != nil {
		http.Error(w, "unable to determine remote address: "+err.Error(), http.StatusInternalServerError)
		return
	}
	laddr, err := parseLocalIP(r)
	if err != nil {
		http.Error(w, "unable to determine local address: "+err.Error(), http.StatusInternalServerError)
		return
	}

	h.server.logger.Info().
		Str("path", r.RequestURI).
		Str("client", raddr.String()).
		Msg("incoming HTTP request")

	manifestRaddr := raddr
	spoofIPs, ok := r.URL.Query()["spoof"]
	if ok && len(spoofIPs[0]) > 0 {
		manifestRaddr = net.ParseIP(spoofIPs[0])
		if manifestRaddr == nil {
			http.Error(w, "unable to determine host address: invalid ip: "+spoofIPs[0], http.StatusBadRequest)
			return
		}
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
			LocalIP:  laddr,
			RemoteIP: raddr,
			HttpBaseUrl: &url.URL{
				Scheme: "http",
				Host:   r.Host,
			},
			ApiBaseUrl: &url.URL{
				Scheme: "http",
				Host:   net.JoinHostPort(laddr.String(), strconv.Itoa(h.server.store.GlobalHints.ApiPort)),
			},
			SyslogHost: net.JoinHostPort(laddr.String(), strconv.Itoa(h.server.store.GlobalHints.SyslogPort)),
			Manifest:   manifest,
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
	} else if mount.LocalDir != "" {
		path := mount.HostPath(h.server.rootPath, r.URL.Path)

		if !mount.ValidateHostPath(h.server.rootPath, path) {
			h.server.logger.Error().
				Err(err).
				Msgf("Requested path is invalid: %q", path)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		f, err := os.Open(path)
		if err != nil {
			h.server.logger.Error().
				Err(err).
				Msgf("Could not get file from local dir: %q", path)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		stat, err := f.Stat()
		if err != nil {
			h.server.logger.Error().
				Err(err).
				Msgf("could not stat file: %q", path)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		http.ServeContent(w, r, r.URL.Path, stat.ModTime(), f)
		return
	} else {
		// mount has neither .Path, .Proxy nor .LocalDir defined
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
