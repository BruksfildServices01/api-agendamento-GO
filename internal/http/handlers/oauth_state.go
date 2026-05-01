package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// createOAuthState cria um state HMAC assinado para fluxos OAuth.
// Formato: base64url(barbershopID:timestamp:HMAC_SHA256)
func createOAuthState(barbershopID uint, secret string) string {
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	data := fmt.Sprintf("%d:%s", barbershopID, ts)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(data))
	sig := hex.EncodeToString(mac.Sum(nil))
	raw := fmt.Sprintf("%s:%s", data, sig)
	return base64.URLEncoding.EncodeToString([]byte(raw))
}

// parseOAuthState valida e extrai o barbershopID de um state OAuth.
// Retorna erro se a assinatura for inválida ou o state tiver expirado (10 minutos).
func parseOAuthState(state, secret string) (uint, error) {
	decoded, err := base64.URLEncoding.DecodeString(state)
	if err != nil {
		return 0, errors.New("invalid state encoding")
	}

	parts := strings.SplitN(string(decoded), ":", 3)
	if len(parts) != 3 {
		return 0, errors.New("invalid state format")
	}

	bid, ts, sig := parts[0], parts[1], parts[2]

	data := fmt.Sprintf("%s:%s", bid, ts)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(data))
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(sig), []byte(expected)) {
		return 0, errors.New("invalid state signature")
	}

	timestamp, err := strconv.ParseInt(ts, 10, 64)
	if err != nil || time.Now().Unix()-timestamp > 600 {
		return 0, errors.New("state expired")
	}

	id, err := strconv.ParseUint(bid, 10, 64)
	if err != nil {
		return 0, errors.New("invalid barbershop id in state")
	}
	return uint(id), nil
}
