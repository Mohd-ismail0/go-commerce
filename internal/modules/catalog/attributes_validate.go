package catalog

import (
	"encoding/json"
	"strconv"
	"strings"

	sharederrors "rewrite/internal/shared/errors"
)

func validateAttributeValue(inputType, value string, allowedJSON []byte) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", sharederrors.BadRequest("attribute value is required")
	}
	switch strings.ToLower(strings.TrimSpace(inputType)) {
	case "text":
		return value, nil
	case "number":
		f, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return "", sharederrors.BadRequest("invalid number attribute value")
		}
		return strconv.FormatFloat(f, 'f', -1, 64), nil
	case "boolean":
		switch strings.ToLower(value) {
		case "true", "1", "yes":
			return "true", nil
		case "false", "0", "no":
			return "false", nil
		default:
			return "", sharederrors.BadRequest("invalid boolean attribute value")
		}
	case "select":
		if len(allowedJSON) == 0 {
			return "", sharederrors.BadRequest("select attribute has no allowed_values")
		}
		var opts []string
		if err := json.Unmarshal(allowedJSON, &opts); err != nil {
			return "", sharederrors.BadRequest("invalid allowed_values on attribute")
		}
		if len(opts) == 0 {
			return "", sharederrors.BadRequest("select attribute allowed_values must be non-empty")
		}
		for _, o := range opts {
			if o == value {
				return value, nil
			}
		}
		return "", sharederrors.BadRequest("value not in allowed_values")
	default:
		return "", sharederrors.BadRequest("unknown attribute input_type")
	}
}

func validateCatalogAttributeInput(inputType string, allowedJSON []byte) error {
	t := strings.ToLower(strings.TrimSpace(inputType))
	switch t {
	case "text", "number", "boolean":
		if len(allowedJSON) > 0 {
			var raw any
			if err := json.Unmarshal(allowedJSON, &raw); err != nil {
				return sharederrors.BadRequest("allowed_values must be valid JSON when set")
			}
		}
		return nil
	case "select":
		var opts []string
		if err := json.Unmarshal(allowedJSON, &opts); err != nil || len(opts) == 0 {
			return sharederrors.BadRequest("select attributes require allowed_values as a non-empty JSON array of strings")
		}
		return nil
	default:
		return sharederrors.BadRequest("input_type must be text, number, boolean, or select")
	}
}
