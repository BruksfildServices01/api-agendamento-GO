package main

import (
	"fmt"

	"github.com/mercadopago/sdk-go/pkg/config"
	"github.com/mercadopago/sdk-go/pkg/payment"
)

func main() {
	cfg, err := config.New("APP_USR-3056706475935541-040522-585dc152a5191088e62e6470be0e9b05-3315715863")
	if err != nil {
		fmt.Println("erro ao criar config:", err)
		return
	}

	client := payment.NewClient(cfg)

	// Busca um pagamento inexistente só pra validar autenticação
	_, err = client.Get(nil, 0)
	if err != nil {
		fmt.Println("erro (esperado se ID=0, mas token inválido retorna 401):", err)
		return
	}

	fmt.Println("token válido, conexão com Mercado Pago OK")
}
