package payment

// MPCredentials representa as credenciais do Mercado Pago armazenadas
// criptografadas em barbershop_payment_providers.credentials_encrypted.
//
// Esta struct é estritamente interna:
//   - Nunca retornar em DTO ou resposta HTTP.
//   - Nunca logar nenhum campo.
//   - Nunca serializar para JSON em contexto externo.
type MPCredentials struct {
	AccessToken string `json:"access_token"`
	PublicKey   string `json:"public_key"`
}
