package mp

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"
)

// computeHMAC reproduz a assinatura esperada por VerifyWebhookSignature.
// Mesmo algoritmo do código de produção — usado para gerar casos válidos.
func computeHMAC(secret, dataID, xRequestID, ts string) string {
	manifest := fmt.Sprintf("id=%s&request-id=%s&ts=%s", dataID, xRequestID, ts)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(manifest))
	return hex.EncodeToString(mac.Sum(nil))
}

func buildXSignature(ts, v1 string) string {
	return fmt.Sprintf("ts=%s,v1=%s", ts, v1)
}

const (
	testSecret    = "webhook-secret-test-key"
	testDataID    = "1234567890"
	testRequestID = "req-abc-def-123"
	testTimestamp = "1700000000"
)

func validSignature() string {
	v1 := computeHMAC(testSecret, testDataID, testRequestID, testTimestamp)
	return buildXSignature(testTimestamp, v1)
}

// ── Casos positivos ────────────────────────────────────────────────────────────

func TestVerifyWebhookSignature_ValidSignature(t *testing.T) {
	if !VerifyWebhookSignature(testSecret, validSignature(), testRequestID, testDataID) {
		t.Error("assinatura HMAC válida deve ser aceita")
	}
}

func TestVerifyWebhookSignature_ExtraFieldsInSignatureAreIgnored(t *testing.T) {
	// MP pode adicionar campos extras no header no futuro — código atual ignora campos desconhecidos.
	v1 := computeHMAC(testSecret, testDataID, testRequestID, testTimestamp)
	sig := fmt.Sprintf("ts=%s,v1=%s,extra=ignored", testTimestamp, v1)
	if !VerifyWebhookSignature(testSecret, sig, testRequestID, testDataID) {
		t.Error("campos extras no header não devem invalidar assinatura válida")
	}
}

func TestVerifyWebhookSignature_FieldsInDifferentOrder(t *testing.T) {
	// Header pode vir com v1 antes de ts.
	v1 := computeHMAC(testSecret, testDataID, testRequestID, testTimestamp)
	sig := fmt.Sprintf("v1=%s,ts=%s", v1, testTimestamp)
	if !VerifyWebhookSignature(testSecret, sig, testRequestID, testDataID) {
		t.Error("campos em ordem diferente devem ser aceitos — parsing é baseado em prefixo")
	}
}

// ── Casos negativos — secret e assinatura ─────────────────────────────────────

func TestVerifyWebhookSignature_EmptySecret(t *testing.T) {
	// Comportamento documentado: secret vazio → false.
	// O bypass de "modo dev" é responsabilidade do handler (MPWebhookHandler),
	// não desta função.
	if VerifyWebhookSignature("", validSignature(), testRequestID, testDataID) {
		t.Error("secret vazio deve retornar false")
	}
}

func TestVerifyWebhookSignature_EmptySignatureHeader(t *testing.T) {
	// Header x-signature ausente → false. Sem assinatura não há o que validar.
	if VerifyWebhookSignature(testSecret, "", testRequestID, testDataID) {
		t.Error("assinatura vazia deve retornar false")
	}
}

func TestVerifyWebhookSignature_BothEmpty(t *testing.T) {
	if VerifyWebhookSignature("", "", "", "") {
		t.Error("tudo vazio deve retornar false")
	}
}

func TestVerifyWebhookSignature_WrongSecret(t *testing.T) {
	if VerifyWebhookSignature("wrong-secret", validSignature(), testRequestID, testDataID) {
		t.Error("secret errado deve retornar false")
	}
}

// ── Casos negativos — adulteração de campos ───────────────────────────────────

func TestVerifyWebhookSignature_TamperedDataID(t *testing.T) {
	if VerifyWebhookSignature(testSecret, validSignature(), testRequestID, "tampered-id") {
		t.Error("dataID adulterado deve invalidar a assinatura")
	}
}

func TestVerifyWebhookSignature_TamperedRequestID(t *testing.T) {
	if VerifyWebhookSignature(testSecret, validSignature(), "tampered-req-id", testDataID) {
		t.Error("xRequestID adulterado deve invalidar a assinatura")
	}
}

func TestVerifyWebhookSignature_TamperedTimestamp(t *testing.T) {
	// Assina com ts original, mas tenta validar com ts diferente no header.
	v1 := computeHMAC(testSecret, testDataID, testRequestID, testTimestamp)
	tamperedSig := buildXSignature("9999999999", v1) // ts diferente no header
	if VerifyWebhookSignature(testSecret, tamperedSig, testRequestID, testDataID) {
		t.Error("timestamp adulterado deve invalidar a assinatura")
	}
}

func TestVerifyWebhookSignature_TamperedV1Value(t *testing.T) {
	sig := buildXSignature(testTimestamp, "000000000000000000000000000000000000000000000000000000000000dead")
	if VerifyWebhookSignature(testSecret, sig, testRequestID, testDataID) {
		t.Error("valor HMAC adulterado deve retornar false")
	}
}

// ── Casos negativos — header malformado ───────────────────────────────────────

func TestVerifyWebhookSignature_MissingTsField(t *testing.T) {
	v1 := computeHMAC(testSecret, testDataID, testRequestID, testTimestamp)
	sig := fmt.Sprintf("v1=%s", v1) // sem ts=
	if VerifyWebhookSignature(testSecret, sig, testRequestID, testDataID) {
		t.Error("header sem campo ts deve retornar false")
	}
}

func TestVerifyWebhookSignature_MissingV1Field(t *testing.T) {
	sig := fmt.Sprintf("ts=%s", testTimestamp) // sem v1=
	if VerifyWebhookSignature(testSecret, sig, testRequestID, testDataID) {
		t.Error("header sem campo v1 deve retornar false")
	}
}

func TestVerifyWebhookSignature_MalformedHeaderNoPanic(t *testing.T) {
	// Nenhum desses casos pode gerar panic.
	inputs := []string{
		"garbage",
		"ts=,v1=",
		"=value",
		"ts=abc,v1=",
		",,,",
		"ts=1,v1=not-hex-value",
		"\x00\x01\x02",
		"ts=" + string(make([]byte, 10000)), // header gigante
	}
	for _, sig := range inputs {
		// Apenas verifica que não há panic — resultado false é esperado.
		_ = VerifyWebhookSignature(testSecret, sig, testRequestID, testDataID)
	}
}
