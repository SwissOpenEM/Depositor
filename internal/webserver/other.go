package webserver

import (
	"log"
	"os"
)

// deferRemove schedules a file removal, logging any errors
func deferRemove(path string) {
	if err := os.Remove(path); err != nil {
		log.Printf("Warning: failed to remove temporary file %s: %v", path, err)
	}
}

type responseBody interface {
	~struct {
		Message *string `json:"message,omitempty"`
		Status  *string `json:"status,omitempty"`
	}
}

func getErrorResponse[T responseBody](
	status, message string,
) (T, error) {
	return T(ResponseType{
		Status:  &status,
		Message: &message,
	}), nil
}
