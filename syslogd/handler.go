package syslogd

import "gopkg.in/mcuadros/go-syslog.v2/format"

func (server *Server) syslogHandleLog(logParts format.LogParts) {
	event := server.logger.Info()
	var msg string
	for key, value := range logParts {
		if m, ok := isMsg(key, value); ok {
			msg = m
			continue
		}
		event = event.Interface("syslog."+key, value)
	}
	if msg == "" {
		msg = "UNKNOWN"
	}
	event.Msg(msg)
}

func isMsg(key string, value any) (m string, ok bool) {
	if key != "content" && key != "message" {
		return "", false
	}
	m, ok = value.(string)
	if !ok {
		return "", false
	}
	return m, true
}
