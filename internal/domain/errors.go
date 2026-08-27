package domain

import "fmt"

type Violation struct {
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ValidationError struct {
	Violations []Violation `json:"violations"`
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation failed with %d violation(s)", len(e.Violations))
}

type StateError struct {
	Current  ApplicationState
	Expected string
}

func (e *StateError) Error() string {
	return fmt.Sprintf("state %q does not allow operation; expected %s", e.Current, e.Expected)
}

func violation(field, code, message string) Violation {
	return Violation{Field: field, Code: code, Message: message}
}
