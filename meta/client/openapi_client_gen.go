//go:build ignore

package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"go/format"
	"os"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/getkin/kin-openapi/openapi3"
	"gopkg.in/yaml.v3"
)

type openAPI struct {
	Paths      map[string]map[string]operation `yaml:"paths"`
	Components struct {
		Parameters map[string]parameter `yaml:"parameters"`
		Schemas    map[string]schema    `yaml:"schemas"`
	} `yaml:"components"`
}

type operation struct {
	OperationID   string       `yaml:"operationId"`
	DiscobotApply *applyConfig `yaml:"x-discobot-apply"`
	Parameters    []parameter  `yaml:"parameters"`
	RequestBody   *requestBody `yaml:"requestBody"`
}

type applyConfig struct {
	Resource          string   `yaml:"resource"`
	ListOperationID   string   `yaml:"listOperationId"`
	GetOperationID    string   `yaml:"getOperationId"`
	UpdateOperationID string   `yaml:"updateOperationId"`
	ScopeField        string   `yaml:"scopeField"`
	ScopeParam        string   `yaml:"scopeParam"`
	IDField           string   `yaml:"idField"`
	IDParam           string   `yaml:"idParam"`
	NameField         string   `yaml:"nameField"`
	MatchFields       []string `yaml:"matchFields"`
}

type requestBody struct {
	Required bool                 `yaml:"required"`
	Content  map[string]mediaType `yaml:"content"`
}

type mediaType struct {
	Schema schema `yaml:"schema"`
}

type schema struct {
	Ref                  string            `yaml:"$ref"`
	Type                 string            `yaml:"type"`
	Properties           map[string]schema `yaml:"properties"`
	Required             []string          `yaml:"required"`
	AllOf                []schema          `yaml:"allOf"`
	Items                *schema           `yaml:"items"`
	AdditionalProperties any               `yaml:"additionalProperties"`
}

type parameter struct {
	Ref      string `yaml:"$ref"`
	Name     string `yaml:"name"`
	In       string `yaml:"in"`
	Required bool   `yaml:"required"`
	Schema   struct {
		Type string `yaml:"type"`
	} `yaml:"schema"`
}

type route struct {
	Method      string
	Path        string
	OperationID string
	Params      []parameter
	BodyParams  []bodyParam
	Apply       *applyConfig
}

func main() {
	openapiPath := flag.String("openapi", "../api/openapi.yaml", "OpenAPI YAML path")
	outPath := flag.String("out", "client.gen.go", "generated output path")
	flag.Parse()

	data, err := os.ReadFile(*openapiPath)
	if err != nil {
		fatal(err)
	}
	if err := validateOpenAPI(data); err != nil {
		fatal(err)
	}
	var doc openAPI
	if err := yaml.Unmarshal(data, &doc); err != nil {
		fatal(err)
	}
	routes := collectRoutes(doc)
	src, err := generate(routes)
	if err != nil {
		fatal(err)
	}
	formatted, err := format.Source(src)
	if err != nil {
		_, _ = os.Stderr.Write(src)
		fatal(err)
	}
	if err := os.WriteFile(*outPath, formatted, 0o644); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	_, _ = fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

func validateOpenAPI(data []byte) error {
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromData(data)
	if err != nil {
		return fmt.Errorf("load OpenAPI: %w", err)
	}
	if err := doc.Validate(context.Background()); err != nil {
		return fmt.Errorf("validate OpenAPI: %w", err)
	}
	return nil
}

func collectRoutes(doc openAPI) []route {
	methods := map[string]string{
		"delete":  "DELETE",
		"get":     "GET",
		"head":    "HEAD",
		"options": "OPTIONS",
		"patch":   "PATCH",
		"post":    "POST",
		"put":     "PUT",
	}
	var routes []route
	for path, ops := range doc.Paths {
		for methodLower, op := range ops {
			method, ok := methods[strings.ToLower(methodLower)]
			if !ok || op.OperationID == "" {
				continue
			}
			params := make([]parameter, 0, len(op.Parameters))
			for _, p := range op.Parameters {
				params = append(params, resolveParam(doc, p))
			}
			routes = append(routes, route{Method: method, Path: path, OperationID: op.OperationID, Params: params, BodyParams: collectBodyParams(doc, op), Apply: op.DiscobotApply})
		}
	}
	sort.Slice(routes, func(i, j int) bool {
		if routes[i].Path == routes[j].Path {
			return routes[i].Method < routes[j].Method
		}
		return routes[i].Path < routes[j].Path
	})
	return routes
}

func resolveParam(doc openAPI, p parameter) parameter {
	if p.Ref == "" {
		return p
	}
	const prefix = "#/components/parameters/"
	name := strings.TrimPrefix(p.Ref, prefix)
	if resolved, ok := doc.Components.Parameters[name]; ok {
		return resolved
	}
	return p
}

func collectBodyParams(doc openAPI, op operation) []bodyParam {
	if op.RequestBody == nil {
		return nil
	}
	media, ok := op.RequestBody.Content["application/json"]
	if !ok {
		return nil
	}
	schema := resolveSchema(doc, media.Schema)
	if schema.Type != "object" || len(schema.Properties) == 0 {
		return nil
	}
	required := map[string]bool{}
	for _, name := range schema.Required {
		required[name] = true
	}
	names := make([]string, 0, len(schema.Properties))
	for name := range schema.Properties {
		names = append(names, name)
	}
	sort.Strings(names)

	params := make([]bodyParam, 0, len(names))
	for _, name := range names {
		params = append(params, bodyParam{Name: name, Required: required[name], Schema: resolveSchema(doc, schema.Properties[name])})
	}
	return params
}

func resolveSchema(doc openAPI, value schema) schema {
	if value.Ref != "" {
		const prefix = "#/components/schemas/"
		name := strings.TrimPrefix(value.Ref, prefix)
		if resolved, ok := doc.Components.Schemas[name]; ok {
			value = resolved
		}
	}
	if len(value.AllOf) == 0 {
		return value
	}
	merged := schema{Type: "object", Properties: map[string]schema{}, Required: append([]string(nil), value.Required...)}
	for _, part := range value.AllOf {
		resolved := resolveSchema(doc, part)
		if resolved.Type != "" {
			merged.Type = resolved.Type
		}
		for name, property := range resolved.Properties {
			merged.Properties[name] = property
		}
		merged.Required = append(merged.Required, resolved.Required...)
	}
	for name, property := range value.Properties {
		merged.Properties[name] = property
	}
	return merged
}

func generate(routes []route) ([]byte, error) {
	var out bytes.Buffer
	out.WriteString("// Code generated by go generate from meta/api/openapi.yaml; DO NOT EDIT.\n\n")
	out.WriteString("package client\n\n")
	out.WriteString("import (\n")
	out.WriteString("\t\"context\"\n")
	if hasApplyRoutes(routes) {
		out.WriteString("\t\"encoding/json\"\n")
		out.WriteString("\t\"fmt\"\n")
	}
	out.WriteString("\t\"net/http\"\n")
	out.WriteString("\t\"net/url\"\n")
	if needsStrconv(routes) {
		out.WriteString("\t\"strconv\"\n")
	}
	out.WriteString("\t\"strings\"\n")
	out.WriteString(")\n\n")

	for _, route := range routes {
		methodName := exportedName(route.OperationID)
		paramsName := methodName + "Params"
		params := routeParams(route)
		writeParams(&out, paramsName, params)
		writeMethod(&out, methodName, paramsName, route, params)
	}
	writeApplyMethods(&out, routes)
	return out.Bytes(), nil
}

func hasApplyRoutes(routes []route) bool {
	return len(collectApplyConfigs(routes)) > 0
}

type applyRoutes struct {
	Create route
	List   route
	Get    route
	Update route
	Config applyConfig
}

func collectApplyConfigs(routes []route) []applyRoutes {
	var applies []applyRoutes
	for _, create := range routes {
		if create.Apply == nil {
			continue
		}
		list := findRoute(routes, create.Apply.ListOperationID)
		get := findRoute(routes, create.Apply.GetOperationID)
		update := findRoute(routes, create.Apply.UpdateOperationID)
		if list == nil || get == nil || update == nil {
			continue
		}
		applies = append(applies, applyRoutes{
			Create: create,
			List:   *list,
			Get:    *get,
			Update: *update,
			Config: *create.Apply,
		})
	}
	return applies
}

func findRoute(routes []route, operationID string) *route {
	for i := range routes {
		if routes[i].OperationID == operationID {
			return &routes[i]
		}
	}
	return nil
}

func needsStrconv(routes []route) bool {
	for _, route := range routes {
		for _, p := range routeParams(route) {
			if p.In == "query" && (p.Type == "int" || p.Type == "*int" || p.Type == "bool" || p.Type == "*bool") {
				return true
			}
		}
	}
	return false
}

type methodParam struct {
	Name     string
	Field    string
	In       string
	Required bool
	Type     string
}

type bodyParam struct {
	Name     string
	Required bool
	Schema   schema
}

func routeParams(r route) []methodParam {
	seen := map[string]bool{}
	var params []methodParam
	for _, match := range pathParamRegexp.FindAllStringSubmatch(r.Path, -1) {
		name := match[1]
		seen["path:"+name] = true
		params = append(params, methodParam{Name: name, Field: exportedName(name), In: "path", Required: true, Type: "string"})
	}
	for _, p := range r.Params {
		if p.Name == "" || p.In == "path" && seen["path:"+p.Name] {
			continue
		}
		key := p.In + ":" + p.Name
		if seen[key] {
			continue
		}
		seen[key] = true
		params = append(params, methodParam{Name: p.Name, Field: exportedName(p.Name), In: p.In, Required: p.Required, Type: paramType(p)})
	}
	for _, p := range r.BodyParams {
		key := "body:" + p.Name
		if seen[key] {
			continue
		}
		seen[key] = true
		params = append(params, methodParam{Name: p.Name, Field: exportedName(p.Name), In: "body", Required: p.Required, Type: schemaType(p.Schema, p.Required)})
	}
	sort.SliceStable(params, func(i, j int) bool {
		if params[i].In == params[j].In {
			return params[i].Name < params[j].Name
		}
		return paramOrder(params[i].In) < paramOrder(params[j].In)
	})
	return params
}

func paramOrder(in string) int {
	switch in {
	case "path":
		return 0
	case "query":
		return 1
	case "body":
		return 2
	default:
		return 3
	}
}

func paramType(p parameter) string {
	switch p.Schema.Type {
	case "integer":
		if p.Required {
			return "int"
		}
		return "*int"
	case "boolean":
		if p.Required {
			return "bool"
		}
		return "*bool"
	default:
		if p.Required {
			return "string"
		}
		return "*string"
	}
}

func schemaType(schema schema, required bool) string {
	var typ string
	switch schema.Type {
	case "integer":
		typ = "int"
	case "number":
		typ = "float64"
	case "boolean":
		typ = "bool"
	case "array":
		typ = "[]" + schemaType(deref(schema.Items), true)
	case "object":
		typ = "map[string]any"
	default:
		typ = "string"
	}
	if required || strings.HasPrefix(typ, "[]") || strings.HasPrefix(typ, "map[") {
		return typ
	}
	return "*" + typ
}

func deref[T any](value *T) T {
	if value == nil {
		var zero T
		return zero
	}
	return *value
}

func writeParams(out *bytes.Buffer, name string, params []methodParam) {
	fmt.Fprintf(out, "// %s contains parameters for the generated client method.\n", name)
	fmt.Fprintf(out, "type %s struct {\n", name)
	for _, p := range params {
		fmt.Fprintf(out, "\t%s %s `json:%q yaml:%q`\n", p.Field, p.Type, p.Name+",omitempty", p.Name+",omitempty")
	}
	out.WriteString("}\n\n")
}

func writeMethod(out *bytes.Buffer, name, paramsName string, r route, params []methodParam) {
	fmt.Fprintf(out, "// %s calls %s %s.\n", name, r.Method, r.Path)
	fmt.Fprintf(out, "func (c *Client) %s(ctx context.Context, params %s, opts ...RequestOption) (*Response, error) {\n", name, paramsName)
	fmt.Fprintf(out, "\tpath := %q\n", r.Path)
	for _, p := range params {
		if p.In != "path" {
			continue
		}
		fmt.Fprintf(out, "\tpath = strings.ReplaceAll(path, %q, url.PathEscape(params.%s))\n", "{"+p.Name+"}", p.Field)
	}
	out.WriteString("\tquery := make(url.Values)\n")
	for _, p := range params {
		if p.In != "query" {
			continue
		}
		writeQueryParam(out, p)
	}
	bodyParams := filterParams(params, "body")
	if len(bodyParams) > 0 {
		out.WriteString("\tbody := struct {\n")
		for _, p := range bodyParams {
			fmt.Fprintf(out, "\t\t%s %s `json:%q`\n", p.Field, p.Type, p.Name+",omitempty")
		}
		out.WriteString("\t}{\n")
		for _, p := range bodyParams {
			fmt.Fprintf(out, "\t\t%s: params.%s,\n", p.Field, p.Field)
		}
		out.WriteString("\t}\n")
		out.WriteString("\topts = append([]RequestOption{WithJSONBody(body)}, opts...)\n")
	}
	fmt.Fprintf(out, "\treturn c.do(ctx, http.Method%s, path, query, opts...)\n", methodConstSuffix(r.Method))
	out.WriteString("}\n\n")
}

func writeApplyMethods(out *bytes.Buffer, routes []route) {
	applies := collectApplyConfigs(routes)
	if len(applies) == 0 {
		return
	}
	apply := applies[0]
	resource := apply.Config.Resource
	resourceName := exportedName(resource)
	createMethod := exportedName(apply.Create.OperationID)
	updateMethod := exportedName(apply.Update.OperationID)
	getMethod := exportedName(apply.Get.OperationID)
	listMethod := exportedName(apply.List.OperationID)
	createParams := routeParams(apply.Create)
	updateParams := routeParams(apply.Update)
	bodyParams := filterParams(createParams, "body")
	applyParamTypes := map[string]string{}
	for _, p := range bodyParams {
		applyParamTypes[p.Field] = applyParamType(p)
	}
	scopeField := exportedName(apply.Config.ScopeField)
	scopeParam := exportedName(apply.Config.ScopeParam)
	idField := exportedName(apply.Config.IDField)
	idParam := exportedName(apply.Config.IDParam)
	nameField := exportedName(apply.Config.NameField)
	primaryMatchField := ""
	if len(apply.Config.MatchFields) > 0 {
		primaryMatchField = exportedName(apply.Config.MatchFields[0])
	}

	out.WriteString("// ApplyResult describes the create-or-update operation performed by Apply.\n")
	out.WriteString("type ApplyResult struct {\n")
	out.WriteString("\tType string\n")
	out.WriteString("\tName string\n")
	out.WriteString("\tID string\n")
	out.WriteString("\tOperation string\n")
	out.WriteString("\tResponse *Response\n")
	out.WriteString("}\n\n")

	out.WriteString("// Apply creates or updates a declarative Meta resource.\n")
	out.WriteString("func (c *Client) Apply(ctx context.Context, resourceType string, params any, opts ...RequestOption) (*ApplyResult, error) {\n")
	out.WriteString("\tswitch normalizeApplyType(resourceType) {\n")
	fmt.Fprintf(out, "\tcase %q:\n", normalizeResourceType(resource))
	fmt.Fprintf(out, "\t\tvar typed Apply%sParams\n", resourceName)
	out.WriteString("\t\tif err := decodeApplyParams(params, &typed); err != nil { return nil, err }\n")
	fmt.Fprintf(out, "\t\treturn c.Apply%s(ctx, typed, opts...)\n", resourceName)
	out.WriteString("\tdefault:\n")
	out.WriteString("\t\treturn nil, fmt.Errorf(\"unsupported type %q\", resourceType)\n")
	out.WriteString("\t}\n")
	out.WriteString("}\n\n")

	fmt.Fprintf(out, "// Apply%s creates or updates a %s by id or name.\n", resourceName, resource)
	fmt.Fprintf(out, "func (c *Client) Apply%s(ctx context.Context, params Apply%sParams, opts ...RequestOption) (*ApplyResult, error) {\n", resourceName, resourceName)
	fmt.Fprintf(out, "\tscope := strings.TrimSpace(params.%s)\n", scopeField)
	fmt.Fprintf(out, "\tif scope == \"\" { return nil, fmt.Errorf(%q) }\n", apply.Config.ScopeField+" is required")
	fmt.Fprintf(out, "\tname := strings.TrimSpace(params.%s)\n", nameField)
	out.WriteString("\tif name == \"\" { return nil, fmt.Errorf(\"name is required\") }\n")
	matchArg := "\"\""
	if primaryMatchField != "" {
		matchArg = "params." + primaryMatchField
	}
	fmt.Fprintf(out, "\texistingID, err := c.find%s(ctx, scope, params.%s, name, %s, opts...)\n", resourceName, idField, matchArg)
	out.WriteString("\tif err != nil { return nil, err }\n")
	out.WriteString("\tif existingID == \"\" {\n")
	fmt.Fprintf(out, "\t\tcreateParams := %sParams{\n", createMethod)
	fmt.Fprintf(out, "\t\t\t%s: scope,\n", scopeParam)
	for _, p := range bodyParams {
		fmt.Fprintf(out, "\t\t\t%s: params.%s,\n", p.Field, p.Field)
	}
	out.WriteString("\t\t}\n")
	fmt.Fprintf(out, "\t\tresp, err := c.%s(ctx, createParams, opts...)\n", createMethod)
	fmt.Fprintf(out, "\t\tif err != nil { return &ApplyResult{Type: %q, Name: name, Operation: \"created\", Response: resp}, err }\n", resource)
	fmt.Fprintf(out, "\t\treturn &ApplyResult{Type: %q, Name: name, Operation: \"created\", Response: resp}, nil\n", resource)
	out.WriteString("\t}\n")
	fmt.Fprintf(out, "\tupdateParams := %sParams{\n", updateMethod)
	fmt.Fprintf(out, "\t\t%s: scope,\n", scopeParam)
	fmt.Fprintf(out, "\t\t%s: existingID,\n", idParam)
	for _, p := range filterParams(updateParams, "body") {
		fmt.Fprintf(out, "\t\t%s: %s,\n", p.Field, applyAssignment(p, applyParamTypes[p.Field]))
	}
	out.WriteString("\t}\n")
	fmt.Fprintf(out, "\tresp, err := c.%s(ctx, updateParams, opts...)\n", updateMethod)
	fmt.Fprintf(out, "\tif err != nil { return &ApplyResult{Type: %q, Name: name, ID: existingID, Operation: \"configured\", Response: resp}, err }\n", resource)
	fmt.Fprintf(out, "\treturn &ApplyResult{Type: %q, Name: name, ID: existingID, Operation: \"configured\", Response: resp}, nil\n", resource)
	out.WriteString("}\n\n")

	fmt.Fprintf(out, "// Apply%sParams contains declarative %s fields.\n", resourceName, resource)
	fmt.Fprintf(out, "type Apply%sParams struct {\n", resourceName)
	out.WriteString("\tType string `json:\"type,omitempty\" yaml:\"type,omitempty\"`\n")
	fmt.Fprintf(out, "\t%s string `json:%q yaml:%q`\n", idField, apply.Config.IDField+",omitempty", apply.Config.IDField+",omitempty")
	fmt.Fprintf(out, "\t%s string `json:%q yaml:%q`\n", scopeField, apply.Config.ScopeField+",omitempty", apply.Config.ScopeField+",omitempty")
	for _, p := range bodyParams {
		fmt.Fprintf(out, "\t%s %s `json:%q yaml:%q`\n", p.Field, applyParamType(p), p.Name+",omitempty", p.Name+",omitempty")
	}
	out.WriteString("}\n\n")

	fmt.Fprintf(out, "type %sApplyList struct {\n", unexportedName(resourceName))
	out.WriteString("\tItems []struct {\n")
	out.WriteString("\t\tID string `json:\"id\"`\n")
	out.WriteString("\t\tName string `json:\"name\"`\n")
	if primaryMatchField != "" {
		fmt.Fprintf(out, "\t\t%s string `json:%q`\n", primaryMatchField, apply.Config.MatchFields[0])
	}
	out.WriteString("\t} `json:\"items\"`\n")
	out.WriteString("}\n\n")

	matchParam := "match string"
	if primaryMatchField == "" {
		matchParam = "_ string"
	}
	fmt.Fprintf(out, "func (c *Client) find%s(ctx context.Context, scope, id, name string, %s, opts ...RequestOption) (string, error) {\n", resourceName, matchParam)
	out.WriteString("\tif id != \"\" {\n")
	fmt.Fprintf(out, "\t\tresp, err := c.%s(ctx, %sParams{%s: scope, %s: id}, opts...)\n", getMethod, getMethod, scopeParam, idParam)
	out.WriteString("\t\tif err == nil { return id, nil }\n")
	out.WriteString("\t\tif resp == nil || resp.StatusCode != http.StatusNotFound { return \"\", err }\n")
	out.WriteString("\t}\n")
	fmt.Fprintf(out, "\tresp, err := c.%s(ctx, %sParams{%s: scope}, opts...)\n", listMethod, listMethod, scopeParam)
	out.WriteString("\tif err != nil { return \"\", err }\n")
	fmt.Fprintf(out, "\tvar list %sApplyList\n", unexportedName(resourceName))
	out.WriteString("\tif err := resp.DecodeJSON(&list); err != nil { return \"\", err }\n")
	out.WriteString("\tvar matches []string\n")
	out.WriteString("\tfor _, item := range list.Items {\n")
	out.WriteString("\t\tif item.Name != name { continue }\n")
	if primaryMatchField != "" {
		fmt.Fprintf(out, "\t\tif match != \"\" && item.%s != match { continue }\n", primaryMatchField)
	}
	out.WriteString("\t\tmatches = append(matches, item.ID)\n")
	out.WriteString("\t}\n")
	fmt.Fprintf(out, "\tif len(matches) > 1 { return \"\", fmt.Errorf(\"multiple %s resources match name %%q; set id\", name) }\n", resource)
	out.WriteString("\tif len(matches) == 1 { return matches[0], nil }\n")
	out.WriteString("\treturn \"\", nil\n")
	out.WriteString("}\n\n")

	out.WriteString("func decodeApplyParams(input, output any) error {\n")
	out.WriteString("\tdata, err := json.Marshal(input)\n")
	out.WriteString("\tif err != nil { return err }\n")
	out.WriteString("\treturn json.Unmarshal(data, output)\n")
	out.WriteString("}\n\n")

	out.WriteString("func normalizeApplyType(value string) string {\n")
	out.WriteString("\tvalue = strings.ToLower(strings.TrimSpace(value))\n")
	out.WriteString("\tvalue = strings.ReplaceAll(value, \"-\", \"\")\n")
	out.WriteString("\tvalue = strings.ReplaceAll(value, \"_\", \"\")\n")
	out.WriteString("\treturn value\n")
	out.WriteString("}\n\n")

	out.WriteString("func optionalApplyString(value string) *string {\n")
	out.WriteString("\tif value == \"\" { return nil }\n")
	out.WriteString("\treturn &value\n")
	out.WriteString("}\n\n")
}

func applyParamType(p methodParam) string {
	return p.Type
}

func applyAssignment(p methodParam, applyType string) string {
	if p.Type == "*string" && applyType == "string" {
		return "optionalApplyString(params." + p.Field + ")"
	}
	return "params." + p.Field
}

func filterParams(params []methodParam, in string) []methodParam {
	var filtered []methodParam
	for _, p := range params {
		if p.In == in {
			filtered = append(filtered, p)
		}
	}
	return filtered
}

func writeQueryParam(out *bytes.Buffer, p methodParam) {
	field := "params." + p.Field
	switch p.Type {
	case "int":
		fmt.Fprintf(out, "\tquery.Set(%q, strconv.Itoa(%s))\n", p.Name, field)
	case "*int":
		fmt.Fprintf(out, "\tif %s != nil { query.Set(%q, strconv.Itoa(*%s)) }\n", field, p.Name, field)
	case "bool":
		fmt.Fprintf(out, "\tquery.Set(%q, strconv.FormatBool(%s))\n", p.Name, field)
	case "*bool":
		fmt.Fprintf(out, "\tif %s != nil { query.Set(%q, strconv.FormatBool(*%s)) }\n", field, p.Name, field)
	case "string":
		fmt.Fprintf(out, "\tquery.Set(%q, %s)\n", p.Name, field)
	default:
		fmt.Fprintf(out, "\tif %s != nil { query.Set(%q, *%s) }\n", field, p.Name, field)
	}
}

func methodConstSuffix(method string) string {
	return strings.Title(strings.ToLower(method))
}

var pathParamRegexp = regexp.MustCompile(`\{([^}]+)\}`)
var splitRegexp = regexp.MustCompile(`[._\-\s:]+`)

func exportedName(s string) string {
	parts := identifierWords(s)
	for i, part := range parts {
		parts[i] = wordName(part)
	}
	return strings.Join(parts, "")
}

func unexportedName(s string) string {
	if s == "" {
		return ""
	}
	runes := []rune(s)
	runes[0] = unicode.ToLower(runes[0])
	return string(runes)
}

func normalizeResourceType(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "-", "")
	value = strings.ReplaceAll(value, "_", "")
	return value
}

func identifierWords(s string) []string {
	raw := splitRegexp.Split(s, -1)
	words := make([]string, 0, len(raw))
	for _, part := range raw {
		if part == "" {
			continue
		}
		words = append(words, splitCamel(part)...)
	}
	return words
}

func splitCamel(s string) []string {
	var words []string
	runes := []rune(s)
	start := 0
	for i := 1; i < len(runes); i++ {
		prev := runes[i-1]
		cur := runes[i]
		var next rune
		if i+1 < len(runes) {
			next = runes[i+1]
		}
		if unicode.IsLower(prev) && unicode.IsUpper(cur) || unicode.IsUpper(prev) && unicode.IsUpper(cur) && next != 0 && unicode.IsLower(next) {
			words = append(words, string(runes[start:i]))
			start = i
		}
	}
	words = append(words, string(runes[start:]))
	return words
}

func wordName(s string) string {
	upper := strings.ToUpper(s)
	switch upper {
	case "ID", "URL", "URI", "API", "JSON", "JWT", "OIDC", "OAUTH", "LLM", "CEK", "KMS", "HSM", "JWKS", "HTTP", "IP":
		if upper == "OAUTH" {
			return "OAuth"
		}
		return upper
	case "URIS":
		return "URIs"
	case "GITHUB":
		return "GitHub"
	}
	runes := []rune(strings.ToLower(s))
	if len(runes) == 0 {
		return ""
	}
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}
