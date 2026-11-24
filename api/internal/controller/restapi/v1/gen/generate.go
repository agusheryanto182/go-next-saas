// Package gen holds code generated from openapi/openapi.yaml. Do not edit api.gen.go; run `make gen`.
package gen

//go:generate go tool -modfile=../../../../../tools/go.mod oapi-codegen -config oapi-codegen.yaml -o api.gen.go ../../../../../../openapi/openapi.yaml
