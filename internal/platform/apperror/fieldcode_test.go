package apperror

import "testing"

func TestFieldCode(t *testing.T) {
	cases := map[string]string{
		"is required": FieldRequired,
		"is required when payment amount is greater than zero": FieldRequired,
		MessageUnknownField:                                   FieldUnknown,
		"request body contains invalid JSON":                  FieldInvalidJSON,
		"body must contain a single JSON object":              FieldInvalidJSON,
		"must be of type string":                              FieldInvalidType,
		"must be between 2 and 120 characters":                FieldInvalidLength,
		"must be between 8 and 72 characters":                 FieldInvalidLength,
		"must be at most 500 characters":                      FieldInvalidLength,
		"must not exceed 100 characters":                      FieldInvalidLength,
		"must not exceed 1048576 bytes":                       FieldInvalidLength,
		"must be a valid email address":                       FieldInvalidFormat,
		"must be a valid Indonesian whatsapp number":          FieldInvalidFormat,
		"must be a valid UUID":                                FieldInvalidFormat,
		"must use YYYY-MM-DD format":                          FieldInvalidFormat,
		"must be a reasonable timestamp":                      FieldInvalidFormat,
		"must be one of cash, transfer, qris, other":          FieldInvalidChoice,
		"must be 'penitip' or 'pemilik_toko'":                 FieldInvalidChoice,
		"must not be duplicated":                              FieldDuplicate,
		"must be between -90 and 90":                          FieldOutOfRange,
		"must be between 1 and 3650":                          FieldOutOfRange,
		"must be an integer between 1 and 100":                FieldOutOfRange,
		"must be greater than zero":                           FieldOutOfRange,
		"must not be negative":                                FieldOutOfRange,
		"must not be in the future":                           FieldOutOfRange,
		"must contain at least one item":                      FieldOutOfRange,
		"store details are required for pemilik_toko account": FieldInvalid,
	}

	for message, want := range cases {
		if got := FieldCode(message); got != want {
			t.Errorf("FieldCode(%q) = %q, want %q", message, got, want)
		}
	}
}

func TestFieldCodesEmpty(t *testing.T) {
	if codes := Internal("X", "y").FieldCodes(); codes != nil {
		t.Fatalf("expected nil codes, got %v", codes)
	}
}
