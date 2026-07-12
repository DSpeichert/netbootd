package config

// ServerHints carries process-global listener ports used by the TFTP and HTTP
// servers to build absolute URLs in manifest content templates (HttpBaseUrl,
// ApiBaseUrl, SyslogHost). These are server configuration, not manifest state,
// so they are passed to the servers directly rather than through the Store.
type ServerHints struct {
	HttpPort   int
	ApiPort    int
	SyslogPort int
}
