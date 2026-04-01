package provider

import (
	"bytes"
	"encoding/json"
	"errors"
	"os/exec"
)

// minifyJSON compacts a JSON string to a single line.
// If the input is not valid JSON it is returned as-is.
func minifyJSON(s string) string {
	var buf bytes.Buffer
	if err := json.Compact(&buf, []byte(s)); err != nil {
		return s
	}
	return buf.String()
}

func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, exec.ErrNotFound) || errors.Is(err, exec.ErrDot) {
		return true
	}
	var execErr *exec.Error
	if errors.As(err, &execErr) {
		return errors.Is(execErr.Err, exec.ErrNotFound) || errors.Is(execErr.Err, exec.ErrDot)
	}
	return false
}
