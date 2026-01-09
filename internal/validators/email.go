package validators

import (
	"net"
	"strings"
)

func IsEmailDomainValid(email string) bool {
	at := strings.LastIndex(email, "@")
	if at < 0 || at == len(email)-1 {
		return false
	}

	domain := email[at+1:]

	if mx, err := net.LookupMX(domain); err == nil && len(mx) > 0 {
		return true
	}

	if ips, err := net.LookupIP(domain); err == nil && len(ips) > 0 {
		return true
	}

	return false
}
