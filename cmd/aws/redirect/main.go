// Command redirect is the AWS Lambda entrypoint for GET /{short_code}. It
// wires the AWS adapters into the redirect handler from internal/handlers
// and holds no logic itself. It is its own function so the hot path gets
// provisioned concurrency and a read-only role.
package main

func main() {}
