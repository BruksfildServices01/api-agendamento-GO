package payment

// PagBankCredentials representa as credenciais PagBank armazenadas
// criptografadas em barbershop_payment_providers.credentials_encrypted.
//
// Esta struct é estritamente interna:
//   - Nunca retornar em DTO ou resposta HTTP.
//   - Nunca logar nenhum campo.
//   - Nunca serializar para JSON em contexto externo.
type PagBankCredentials struct {
	AccessToken string `json:"access_token"`
}
