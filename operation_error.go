package sandbox

import "fmt"

// ResourceOperationError preserves resource identity after a partially successful operation.
type ResourceOperationError struct {
	Operation        string
	ResourceType     string
	ResourceID       string
	RelatedID        string
	CleanupAttempted bool
	CleanupErr       error
	Err              error
}

func (e *ResourceOperationError) Error() string {
	if e == nil {
		return ""
	}
	message := fmt.Sprintf("sandbox: %s failed", e.Operation)
	if e.ResourceID != "" {
		message += fmt.Sprintf(" (%s=%s", e.ResourceType, e.ResourceID)
		if e.RelatedID != "" {
			message += ", related_id=" + e.RelatedID
		}
		message += ")"
	}
	if e.Err != nil {
		message += ": " + e.Err.Error()
	}
	if e.CleanupErr != nil {
		message += "; cleanup failed: " + e.CleanupErr.Error()
	}
	return message
}

func (e *ResourceOperationError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}
