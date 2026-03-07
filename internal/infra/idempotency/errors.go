package idempotency

import "errors"

var ErrDuplicateRequest = errors.New("duplicate_request")
