package validator

import (
	"net"
	"net/url"
	"strings"
)

// IsValidURL проверяет, что ссылка — HTTPS, без авторизации, с валидным хостом.
func IsValidURL(rawURL string) bool {
	u, err := url.ParseRequestURI(strings.TrimSpace(rawURL))
	if err != nil {
		return false
	}

	if u.Scheme != "https" || u.User != nil {
		return false
	}

	host := u.Hostname()
	if host == "" {
		return false
	}

	return net.ParseIP(host) != nil ||
		host == "localhost" ||
		strings.Contains(host, ".")
}
