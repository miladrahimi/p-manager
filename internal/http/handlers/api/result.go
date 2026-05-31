package api

import "github.com/labstack/echo/v4"

// httpResult is a deferred JSON error response produced inside a store
// critical section and written after the lock is released.
type httpResult struct {
	status  int
	message string
}

func (r *httpResult) write(c echo.Context) error {
	return c.JSON(r.status, map[string]string{"message": r.message})
}
