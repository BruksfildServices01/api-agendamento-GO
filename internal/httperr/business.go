package httperr

import "errors"

type BusinessError struct {
	Code string
}

func (e BusinessError) Error() string {
	return e.Code
}

func ErrBusiness(code string) error {
	return BusinessError{Code: code}
}

func IsBusiness(err error, code string) bool {
	var be BusinessError
	if errors.As(err, &be) {
		return be.Code == code
	}
	return false
}
