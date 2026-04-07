package orders

import "errors"

var ErrInvalidStatusTransition = errors.New("invalid order status transition")
