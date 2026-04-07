package utils

import (
	"net/http"

	sharederrors "rewrite/internal/shared/errors"
)

func WriteError(w http.ResponseWriter, err error) {
	apiErr, ok := err.(sharederrors.APIError)
	if !ok {
		JSON(w, http.StatusInternalServerError, sharederrors.Internal("internal server error"))
		return
	}
	JSON(w, apiErr.Status, apiErr)
}
