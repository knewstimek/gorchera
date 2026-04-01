package provider

import (
	"fmt"

	"gorechera/internal/domain"
)

type ErrorKind string

const (
	ErrorKindMissingExecutable ErrorKind = "missing_executable"
	ErrorKindProbeFailed       ErrorKind = "probe_failed"
	ErrorKindCommandFailed     ErrorKind = "command_failed"
	ErrorKindInvalidResponse   ErrorKind = "invalid_response"
	ErrorKindUnsupportedPhase  ErrorKind = "unsupported_phase"
)

type ProviderError struct {
	Provider   domain.ProviderName
	Kind       ErrorKind
	Executable string
	Detail     string
	Err        error
}

func (e *ProviderError) Error() string {
	if e == nil {
		return "<nil>"
	}
	switch {
	case e.Executable != "" && e.Detail != "":
		return fmt.Sprintf("%s provider %s (%s): %s", e.Provider, e.Kind, e.Executable, e.Detail)
	case e.Executable != "":
		return fmt.Sprintf("%s provider %s (%s)", e.Provider, e.Kind, e.Executable)
	case e.Detail != "":
		return fmt.Sprintf("%s provider %s: %s", e.Provider, e.Kind, e.Detail)
	default:
		return fmt.Sprintf("%s provider %s", e.Provider, e.Kind)
	}
}

func (e *ProviderError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func missingExecutableError(provider domain.ProviderName, executable string, err error) error {
	return &ProviderError{
		Provider:   provider,
		Kind:       ErrorKindMissingExecutable,
		Executable: executable,
		Detail:     "CLI executable is not available on PATH",
		Err:        err,
	}
}

func probeFailedError(provider domain.ProviderName, executable string, err error) error {
	return &ProviderError{
		Provider:   provider,
		Kind:       ErrorKindProbeFailed,
		Executable: executable,
		Detail:     "CLI probe failed",
		Err:        err,
	}
}

func commandFailedError(provider domain.ProviderName, executable string, err error) error {
	return &ProviderError{
		Provider:   provider,
		Kind:       ErrorKindCommandFailed,
		Executable: executable,
		Detail:     "provider command failed",
		Err:        err,
	}
}

func invalidResponseError(provider domain.ProviderName, executable, detail string, err error) error {
	return &ProviderError{
		Provider:   provider,
		Kind:       ErrorKindInvalidResponse,
		Executable: executable,
		Detail:     detail,
		Err:        err,
	}
}

func unsupportedPhaseError(provider domain.ProviderName, executable, phase string) error {
	return &ProviderError{
		Provider:   provider,
		Kind:       ErrorKindUnsupportedPhase,
		Executable: executable,
		Detail:     fmt.Sprintf("provider does not support %s phase", phase),
	}
}
