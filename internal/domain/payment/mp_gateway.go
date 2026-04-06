package payment

// MPPreference contém os dados da preferência criada no Mercado Pago.
type MPPreference struct {
	PreferenceID string
	InitPoint    string // URL de checkout (produção)
	SandboxPoint string // URL de checkout (sandbox)
}

// MPBackURLs define as URLs de retorno para o Mercado Pago redirecionar o usuário.
type MPBackURLs struct {
	Success string
	Pending string
	Failure string
}

// MPGateway é a interface para criação de preferências de pagamento no Mercado Pago.
type MPGateway interface {
	CreatePreference(
		amountCents int64,
		description string,
		externalReference string,
		notificationURL string,
		backURLs MPBackURLs,
	) (*MPPreference, error)
}
