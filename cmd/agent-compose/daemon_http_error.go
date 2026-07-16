package main

import "fmt"

type daemonHTTPStatusError struct {
	Source      string
	SourceValue string
	StatusCode  int
	Body        string
}

func (e daemonHTTPStatusError) Error() string {
	return fmt.Sprintf("daemon via %s %q returned HTTP %d: %s", e.Source, e.SourceValue, e.StatusCode, e.Body)
}
