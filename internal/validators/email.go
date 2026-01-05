package validators

import (
	"net"
	"strings"
)

// IsEmailDomainValid verifica se o domínio do e-mail tem DNS/MX válidos.
func IsEmailDomainValid(email string) bool {
	at := strings.LastIndex(email, "@")
	if at < 0 || at == len(email)-1 {
		return false
	}

	domain := email[at+1:]

	// tenta registros MX (servidores de e-mail)
	if mx, err := net.LookupMX(domain); err == nil && len(mx) > 0 {
		return true
	}

	// se não tiver MX, tenta ao menos A/AAAA
	if ips, err := net.LookupIP(domain); err == nil && len(ips) > 0 {
		return true
	}

	return false
}
