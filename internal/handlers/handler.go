// Package handlers implements the API contract as plain net/http handlers
// with the internal/platform ports injected. It never imports a cloud SDK or
// a runtime type; entrypoints under cmd adapt their runtime to it.
package handlers
