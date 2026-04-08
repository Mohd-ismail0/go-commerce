package orders

import "errors"

var ErrInvalidStatusTransition = errors.New("invalid order status transition")
var ErrOptimisticLockFailed = errors.New("order was updated concurrently")
var ErrVoucherUnavailable = errors.New("voucher is unavailable")
