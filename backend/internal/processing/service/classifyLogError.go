package service

import "strings"

type ErrorCategory string

const (
	AuthError           ErrorCategory = "AUTH_ERROR"
	DatabaseError       ErrorCategory = "DATABASE_ERROR"
	NetworkTimeout      ErrorCategory = "NETWORK_TIMEOUT"
	ThirdPartyAPIError  ErrorCategory = "THIRD_PARTY_API_ERROR"
	ValidationError     ErrorCategory = "VALIDATION_ERROR"
	InternalServerError ErrorCategory = "INTERNAL_SERVER_ERROR"
	UnknownError        ErrorCategory = "UNKNOWN_ERROR"
)

func ContainsAny(s string, keywords ...string) bool {
	for _, k := range keywords {
		if strings.Contains(s, k) {
			return true
		}
	}
	return false
}

func Classify(msg string) ErrorCategory {
	if ContainsAny(msg, "jwt", "token", "unauthorized", "forbidden") {
		return AuthError
	} else if ContainsAny(msg, "database", "sql", "connection pool", "deadlock", "transaction") {
		return DatabaseError
	} else if ContainsAny(msg, "timeout", "deadline exceeded", "connection refused", "connection reset", "eof") {
		return NetworkTimeout
	} else if ContainsAny(msg, "upstream", "third party", "external api") {
		return ThirdPartyAPIError
	} else if ContainsAny(msg, "validation failed", "invalid request", "bad request") {
		return ValidationError
	} else if ContainsAny(msg, "panic", "runtime error", "nil pointer", "index out of range") {
		return InternalServerError
	} else {
		return UnknownError
	}
}
