package serverapi

import _ "embed"

// OpenAPISpec is the embedded OpenAPI document served by the server for API documentation.
//
//go:embed openapi.json
var OpenAPISpec []byte
