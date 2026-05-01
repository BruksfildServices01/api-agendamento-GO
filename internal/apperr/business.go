package apperr

import "errors"

// BusinessError representa um erro de regra de negócio identificado por código.
// É usado em domain/, usecase/ e infra/ para sinalizar condições de negócio
// sem depender da camada HTTP.
type BusinessError struct {
	Code string
}

func (e BusinessError) Error() string {
	return e.Code
}

// ErrBusiness cria um BusinessError com o código fornecido.
func ErrBusiness(code string) error {
	return BusinessError{Code: code}
}

// IsBusiness verifica se err é um BusinessError com o código fornecido.
func IsBusiness(err error, code string) bool {
	var be BusinessError
	if errors.As(err, &be) {
		return be.Code == code
	}
	return false
}
