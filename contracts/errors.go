package contracts

import "errors"

const (
	ErrProtocolViolationCode = "ERR_PROTOCOL_VIOLATION"
	ErrToolExecutionCode     = "ERR_TOOL_EXECUTION"
	ErrProviderTimeoutCode   = "ERR_PROVIDER_TIMEOUT"
	ErrMemoryIndexCode       = "ERR_MEMORY_INDEX"
	ErrAgentSchemaCode       = "ERR_AGENT_SCHEMA_INVALID"
	ErrAdapterPersistCode    = "ERR_ADAPTER_PERSISTENCE"
	ErrPolicyDeniedCode      = "ERR_POLICY_DENIED"
	ErrSessionCreateCode     = "ERR_SESSION_CREATE"
	ErrAgentRegistryCode     = "ERR_AGENT_REGISTRY"
	ErrToolBackendCode       = "ERR_TOOL_BACKEND"
)

type RuntimeError struct {
	Code      string
	Message   string
	Retryable bool
	Cause     error
}

func (e *RuntimeError) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	return e.Code
}

func (e *RuntimeError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func NewRuntimeError(code string, message string, retryable bool) *RuntimeError {
	return &RuntimeError{
		Code:      code,
		Message:   message,
		Retryable: retryable,
	}
}

func WrapRuntimeError(code string, message string, retryable bool, cause error) *RuntimeError {
	return &RuntimeError{
		Code:      code,
		Message:   message,
		Retryable: retryable,
		Cause:     cause,
	}
}

func IsCode(err error, code string) bool {
	var runtimeErr *RuntimeError
	if !errors.As(err, &runtimeErr) {
		return false
	}
	return runtimeErr.Code == code
}
