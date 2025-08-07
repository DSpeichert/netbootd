package tftpd

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"

	mfest "github.com/DSpeichert/netbootd/manifest"
	"github.com/DSpeichert/netbootd/static"
	"github.com/Masterminds/sprig"
	"github.com/pin/tftp"
)

func (server *Server) tftpReadHandler(filename string, rf io.ReaderFrom) error {
	raddr := rf.(tftp.OutgoingTransfer).RemoteAddr() // net.UDPAddr
	laddr := rf.(tftp.RequestPacketInfo).LocalIP()

	server.logger.Info().
		Str("path", filename).
		Str("client", raddr.IP.String()).
		Msg("new TFTP request")

	manifest := server.store.FindByIP(raddr.IP)
	if manifest == nil {
		server.logger.Info().
			Str("path", filename).
			Str("client", raddr.IP.String()).
			Msg("no manifest for client")
		return errors.New("no manifest for client: " + raddr.IP.String())
	}

	if manifest.Ipxe {
		f, err := static.Files.Open(filename)
		if err == nil {
			n, err := rf.ReadFrom(f.(io.ReadSeeker))
			server.logger.Info().
				Err(err).
				Str("path", filename).
				Str("client", raddr.IP.String()).
				Int64("sent", n).
				Msg("transfer finished")
			return nil
		}
	}

	mount, err := manifest.GetMount(filename)
	if err != nil {
		server.logger.Error().
			Err(err).
			Str("path", filename).
			Str("client", raddr.IP.String()).
			Msg("cannot find mount")
		return err
	}

	server.logger.Trace().
		Interface("mount", mount).
		Msg("found mount")

	if mount.Proxy != "" {
		url := mount.Proxy
		if mount.AppendSuffix {
			url = url + strings.TrimPrefix(filename, mount.Path)
		}

		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			server.logger.Error().
				Err(err).
				Msg("http request setup failed")
			return err
		}
		req.Header.Add("X-Forwarded-For", raddr.IP.String())
		req.Header.Add("X-TFTP-Port", fmt.Sprintf("%d", raddr.Port))
		req.Header.Add("X-TFTP-File", filename)
		resp, err := server.httpClient.Do(req)
		if err != nil {
			server.logger.Error().
				Err(err).
				Msg("http request setup failed")
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusNotFound {
			server.logger.Error().
				Str("url", url).
				Str("status", resp.Status).
				Str("path", filename).
				Str("client", raddr.IP.String()).
				Msg("upstream: not found")
			return errors.New("file not found")
		} else if resp.StatusCode != http.StatusOK {
			server.logger.Error().
				Msgf("http request returned status %s", resp.Status)
			return fmt.Errorf("HTTP request error: %s", resp.Status)
		}

		// Use ContentLength, if provided, to set TSize option
		if resp.ContentLength >= 0 {
			rf.(tftp.OutgoingTransfer).SetSize(resp.ContentLength)
		}

		n, err := rf.ReadFrom(resp.Body)
		if err != nil {
			server.logger.Error().
				Msgf("ReadFrom failed: %v", err)
			return err
		}

		server.logger.Info().
			Err(err).
			Str("path", filename).
			Str("url", url).
			Str("client", raddr.IP.String()).
			Int64("sent", n).
			Msg("transfer finished")
	} else if mount.Content != "" {
		tmpl, err := template.New("").Funcs(sprig.TxtFuncMap()).Parse(mount.Content)
		if err != nil {
			server.logger.Error().
				Err(err).
				Msg("failed to parse content template for mount")
			return err
		}

		buf := new(bytes.Buffer)

		err = tmpl.Execute(buf, mfest.ContentContext{
			LocalIP:  laddr,
			RemoteIP: raddr.IP,
			HttpBaseUrl: &url.URL{
				Scheme: "http",
				Host:   net.JoinHostPort(laddr.String(), strconv.Itoa(server.store.GlobalHints.HttpPort)),
			},
			ApiBaseUrl: &url.URL{
				Scheme: "http",
				Host:   net.JoinHostPort(laddr.String(), strconv.Itoa(server.store.GlobalHints.ApiPort)),
			},
			Manifest: manifest,
		})
		if err != nil {
			server.logger.Error().
				Err(err).
				Msg("failed to parse content template for mount")
			return err
		}

		rf.(tftp.OutgoingTransfer).SetSize(int64(buf.Len()))

		n, err := rf.ReadFrom(buf)
		if err != nil {
			server.logger.Error().
				Msgf("ReadFrom failed: %v", err)
			return err
		}

		server.logger.Info().
			Err(err).
			Str("path", filename).
			Str("client", raddr.IP.String()).
			Int64("sent", n).
			Msg("transfer finished")
	} else if mount.LocalDir != "" {
		path := filepath.Join(mount.LocalDir, mount.Path)

		if mount.AppendSuffix {
			path = filepath.Join(mount.LocalDir, strings.TrimPrefix(filename, mount.Path))
		}

		if !strings.HasPrefix(path, mount.LocalDir) {
			err := fmt.Errorf("requested path is invalid")
			server.logger.Error().
				Err(err).
				Msgf("Requested path is invalid: %q", path)
			return err
		}

		f, err := os.Open(path)
		if err != nil {
			server.logger.Error().
				Err(err).
				Msgf("Could not get file from local dir: %q", filename)

			return err
		}

		stat, err := f.Stat()
		if err != nil {
			server.logger.Error().
				Err(err).
				Msgf("Could not stat file: %q", path)
			return err
		}

		rf.(tftp.OutgoingTransfer).SetSize(int64(stat.Size()))

		n, err := rf.ReadFrom(f)
		if err != nil {
			server.logger.Error().
				Msgf("ReadFrom failed: %v", err)
			return err
		}

		server.logger.Info().
			Err(err).
			Str("path", filename).
			Str("client", raddr.IP.String()).
			Int64("sent", n).
			Msg("transfer finished")
	} else {
		// mount has neither .Path nor .Proxy defined
		server.logger.Error().
			Str("path", filename).
			Str("client", raddr.IP.String()).
			Str("mount", mount.Path).
			Msg("mount is empty")
		return errors.New("empty mount")
	}

	return nil
}
