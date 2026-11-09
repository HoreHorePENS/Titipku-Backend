package apperror

import "strings"

// Stable, machine-readable codes for per-field validation errors.
//
// The human-readable messages in Error.Fields stay in English and may be
// reworded; clients must branch on these codes instead (see docs/api-errors.md).
const (
	FieldRequired      = "required"
	FieldInvalidFormat = "invalid_format"
	FieldInvalidLength = "invalid_length"
	FieldOutOfRange    = "out_of_range"
	FieldInvalidChoice = "invalid_choice"
	FieldInvalidType   = "invalid_type"
	FieldInvalidJSON   = "invalid_json"
	FieldUnknown       = "unknown_field"
	FieldDuplicate     = "duplicate"
	FieldInvalid       = "invalid"
)

// MessageUnknownField is the message used for JSON properties the endpoint does not accept.
const MessageUnknownField = "is not a recognized field"

// FieldCode derives the stable code for a validation message. Validation
// messages across services follow a small set of phrasings ("is required",
// "must be between 2 and 120 characters", "must be a valid email address", …);
// this keeps the codes consistent without touching every validator.
func FieldCode(message string) string {
	m := strings.ToLower(strings.TrimSpace(message))

	switch {
	case strings.HasPrefix(m, "is required"):
		return FieldRequired
	case m == MessageUnknownField:
		return FieldUnknown
	case strings.Contains(m, "invalid json"), strings.Contains(m, "single json object"):
		return FieldInvalidJSON
	case strings.HasPrefix(m, "must be of type"):
		return FieldInvalidType
	case strings.HasSuffix(m, "characters"), strings.HasSuffix(m, "bytes"):
		return FieldInvalidLength
	case strings.HasPrefix(m, "must be a valid"), strings.HasPrefix(m, "must use"),
		strings.HasPrefix(m, "must be a reasonable"):
		return FieldInvalidFormat
	case strings.HasPrefix(m, "must be one of"), strings.HasPrefix(m, "must be '"):
		return FieldInvalidChoice
	case strings.Contains(m, "duplicated"):
		return FieldDuplicate
	case strings.HasPrefix(m, "must be between"), strings.HasPrefix(m, "must be an integer between"),
		strings.HasPrefix(m, "must be greater than"), strings.HasPrefix(m, "must not be negative"),
		strings.HasPrefix(m, "must not be in the future"), strings.HasPrefix(m, "must contain at least"):
		return FieldOutOfRange
	default:
		return FieldInvalid
	}
}

// FieldCodes returns the stable code for every field in e.Fields, or nil when there are none.
func (e *Error) FieldCodes() map[string]string {
	if len(e.Fields) == 0 {
		return nil
	}

	codes := make(map[string]string, len(e.Fields))
	for field, message := range e.Fields {
		codes[field] = FieldCode(message)
	}
	return codes
}
