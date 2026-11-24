package syntax

import "strconv"

// ErrorCode identifies a stable category of parsing failure.
type ErrorCode uint8

const (
	InvalidEncoding ErrorCode = iota + 1
	InvalidCharacter
	InvalidType
	InvalidScope
	InvalidSeparator
	InvalidDescription
	InvalidSectionSeparator
	InvalidFooterValue
)

func (c ErrorCode) String() string {
	switch c {
	case InvalidEncoding:
		return "InvalidEncoding"
	case InvalidCharacter:
		return "InvalidCharacter"
	case InvalidType:
		return "InvalidType"
	case InvalidScope:
		return "InvalidScope"
	case InvalidSeparator:
		return "InvalidSeparator"
	case InvalidDescription:
		return "InvalidDescription"
	case InvalidSectionSeparator:
		return "InvalidSectionSeparator"
	case InvalidFooterValue:
		return "InvalidFooterValue"
	default:
		return "ErrorCode(" + strconv.Itoa(int(c)) + ")"
	}
}

// ParseError locates a failure in the original input. Offset is a zero-based
// byte position and may equal the input length for missing content.
type ParseError struct {
	Code   ErrorCode
	Offset int
}

func (e *ParseError) Error() string {
	if e == nil {
		return "<nil>"
	}
	var reason string
	switch e.Code {
	case InvalidEncoding:
		reason = "invalid UTF-8"
	case InvalidCharacter:
		reason = "invalid character"
	case InvalidType:
		reason = "invalid type"
	case InvalidScope:
		reason = "invalid scope"
	case InvalidSeparator:
		reason = "expected colon and space"
	case InvalidDescription:
		reason = "expected nonblank description"
	case InvalidSectionSeparator:
		reason = "expected blank line before content"
	case InvalidFooterValue:
		reason = "expected nonblank footer value"
	default:
		reason = "invalid input"
	}
	return "syntax: " + reason + " at byte " + strconv.Itoa(e.Offset)
}
