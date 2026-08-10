package jsonschema

import (
	"reflect"
	"strconv"
	"strings"

	"golang.org/x/xerrors"
)

// Generator creates JSON Schema from Go structs
type Generator struct{}

// NewGenerator creates a new JSON Schema generator
func NewGenerator() *Generator {
	return &Generator{}
}

// Generate creates a JSON Schema from a struct instance
func (g *Generator) Generate(v interface{}) (map[string]interface{}, error) {
	t := reflect.TypeOf(v)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil, xerrors.Errorf("expected struct, got %s", t.Kind())
	}

	return g.generateObject(t)
}

func (g *Generator) generateObject(t reflect.Type) (map[string]interface{}, error) {
	schema := map[string]interface{}{
		"type":                 "object",
		"properties":           make(map[string]interface{}),
		"required":             []string{},
		"additionalProperties": false,
	}

	properties := schema["properties"].(map[string]interface{})
	var required []string
	dependentRequired := make(map[string][]string)

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		if !field.IsExported() {
			continue
		}

		jsonTag := field.Tag.Get("json")
		if jsonTag == "-" {
			continue
		}

		fieldName := field.Name
		parts := strings.Split(jsonTag, ",")
		if parts[0] != "" {
			fieldName = parts[0]
		}

		if !strings.Contains(jsonTag, "omitempty") {
			required = append(required, fieldName)
		}

		if depReq := field.Tag.Get("jsonschema_dependentRequired"); depReq != "" {
			dependencies := strings.Split(depReq, ",")
			for i := range dependencies {
				dependencies[i] = strings.TrimSpace(dependencies[i])
			}
			dependentRequired[fieldName] = dependencies
		}

		fieldSchema, err := g.generateFieldSchema(field)
		if err != nil {
			return nil, xerrors.Errorf("failed to generate schema for field %s: %w", field.Name, err)
		}

		properties[fieldName] = fieldSchema
	}

	if len(required) > 0 {
		schema["required"] = required
	} else {
		delete(schema, "required")
	}

	if len(dependentRequired) > 0 {
		schema["dependentRequired"] = dependentRequired
	}

	return schema, nil
}

func (g *Generator) generateFieldSchema(field reflect.StructField) (map[string]interface{}, error) {
	schema, err := g.generateTypeSchema(field.Type)
	if err != nil {
		return nil, err
	}

	if desc := field.Tag.Get("jsonschema_description"); desc != "" {
		schema["description"] = desc
	}

	g.addValidationTags(field, schema)

	return schema, nil
}

func (g *Generator) generateTypeSchema(t reflect.Type) (map[string]interface{}, error) {
	switch t.Kind() {
	case reflect.String:
		return map[string]interface{}{"type": "string"}, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return map[string]interface{}{"type": "integer"}, nil
	case reflect.Float32, reflect.Float64:
		return map[string]interface{}{"type": "number"}, nil
	case reflect.Bool:
		return map[string]interface{}{"type": "boolean"}, nil
	case reflect.Struct:
		return g.generateObject(t)
	case reflect.Ptr:
		return g.generateTypeSchema(t.Elem())
	case reflect.Slice, reflect.Array:
		itemSchema, err := g.generateTypeSchema(t.Elem())
		if err != nil {
			return nil, xerrors.Errorf("failed to generate array item schema: %w", err)
		}
		return map[string]interface{}{
			"type":  "array",
			"items": itemSchema,
		}, nil
	case reflect.Map:
		valueSchema, err := g.generateTypeSchema(t.Elem())
		if err != nil {
			return nil, xerrors.Errorf("failed to generate map value schema: %w", err)
		}
		return map[string]interface{}{
			"type":                 "object",
			"additionalProperties": valueSchema,
		}, nil
	case reflect.Interface:
		return map[string]interface{}{}, nil
	default:
		return nil, xerrors.Errorf("unsupported type: %s", t.Kind())
	}
}

// Reference: https://json-schema.org/draft/2020-12/json-schema-validation
func (g *Generator) addValidationTags(field reflect.StructField, schema map[string]interface{}) {
	// 6.1 Validation Keywords for Any Instance Type
	// https://json-schema.org/draft/2020-12/json-schema-validation#section-6.1
	// Note: "type" is handled in generateTypeSchema
	if enum := field.Tag.Get("jsonschema_enum"); enum != "" {
		// 6.1.2 enum
		values := strings.Split(enum, ",")
		schema["enum"] = values
	}
	if constValue := field.Tag.Get("jsonschema_const"); constValue != "" {
		// 6.1.3 const
		schema["const"] = constValue
	}

	// 6.2 Validation Keywords for Numeric Instances
	// https://json-schema.org/draft/2020-12/json-schema-validation#section-6.2
	if multipleOf := field.Tag.Get("jsonschema_multipleOf"); multipleOf != "" {
		// 6.2.1 multipleOf
		if value, err := strconv.ParseFloat(multipleOf, 64); err == nil {
			schema["multipleOf"] = value
		}
	}
	if maximum := field.Tag.Get("jsonschema_maximum"); maximum != "" {
		// 6.2.2 maximum
		if value, err := strconv.ParseFloat(maximum, 64); err == nil {
			schema["maximum"] = value
		}
	}
	if exclusiveMax := field.Tag.Get("jsonschema_exclusiveMaximum"); exclusiveMax != "" {
		// 6.2.3 exclusiveMaximum
		if value, err := strconv.ParseFloat(exclusiveMax, 64); err == nil {
			schema["exclusiveMaximum"] = value
		}
	}
	if minimum := field.Tag.Get("jsonschema_minimum"); minimum != "" {
		// 6.2.4 minimum
		if value, err := strconv.ParseFloat(minimum, 64); err == nil {
			schema["minimum"] = value
		}
	}
	if exclusiveMin := field.Tag.Get("jsonschema_exclusiveMinimum"); exclusiveMin != "" {
		// 6.2.5 exclusiveMinimum
		if value, err := strconv.ParseFloat(exclusiveMin, 64); err == nil {
			schema["exclusiveMinimum"] = value
		}
	}

	// 6.3 Validation Keywords for Strings
	// https://json-schema.org/draft/2020-12/json-schema-validation#section-6.3
	if maxLen := field.Tag.Get("jsonschema_maxLength"); maxLen != "" {
		// 6.3.1 maxLength
		if value, err := strconv.Atoi(maxLen); err == nil {
			schema["maxLength"] = value
		}
	}
	if minLen := field.Tag.Get("jsonschema_minLength"); minLen != "" {
		// 6.3.2 minLength
		if value, err := strconv.Atoi(minLen); err == nil {
			schema["minLength"] = value
		}
	}
	if pattern := field.Tag.Get("jsonschema_pattern"); pattern != "" {
		// 6.3.3 pattern
		schema["pattern"] = pattern
	}

	// 6.4 Validation Keywords for Arrays
	// https://json-schema.org/draft/2020-12/json-schema-validation#section-6.4
	if maxItems := field.Tag.Get("jsonschema_maxItems"); maxItems != "" {
		// 6.4.1 maxItems
		if value, err := strconv.Atoi(maxItems); err == nil {
			schema["maxItems"] = value
		}
	}
	if minItems := field.Tag.Get("jsonschema_minItems"); minItems != "" {
		// 6.4.2 minItems
		if value, err := strconv.Atoi(minItems); err == nil {
			schema["minItems"] = value
		}
	}
	if uniqueItems := field.Tag.Get("jsonschema_uniqueItems"); uniqueItems == "true" {
		// 6.4.3 uniqueItems
		schema["uniqueItems"] = true
	}
	// TODO: 6.4.4 maxContains and 6.4.5 minContains require "contains" keyword support

	// 6.5 Validation Keywords for Objects
	// https://json-schema.org/draft/2020-12/json-schema-validation#section-6.5
	if maxProps := field.Tag.Get("jsonschema_maxProperties"); maxProps != "" {
		// 6.5.1 maxProperties
		if value, err := strconv.Atoi(maxProps); err == nil {
			schema["maxProperties"] = value
		}
	}
	if minProps := field.Tag.Get("jsonschema_minProperties"); minProps != "" {
		// 6.5.2 minProperties
		if value, err := strconv.Atoi(minProps); err == nil {
			schema["minProperties"] = value
		}
	}
	// Note: 6.5.3 required is handled in generateObject
	// Note: 6.5.4 dependentRequired is handled in generateObject

	// 7. Semantic Validation With "format"
	// https://json-schema.org/draft/2020-12/json-schema-validation#section-7
	if format := field.Tag.Get("jsonschema_format"); format != "" {
		// 7.3 Defined Formats
		// Common formats: date-time, date, time, duration, email,
		// idn-email, hostname, idn-hostname, ipv4, ipv6, uri, uri-reference,
		// iri, iri-reference, uuid, uri-template, json-pointer, relative-json-pointer, regex
		schema["format"] = format
	}

	// 8. A Vocabulary for the Contents of String-Encoded Data
	// https://json-schema.org/draft/2020-12/json-schema-validation#section-8
	if contentEncoding := field.Tag.Get("jsonschema_contentEncoding"); contentEncoding != "" {
		// 8.3 contentEncoding
		schema["contentEncoding"] = contentEncoding
	}
	if contentMediaType := field.Tag.Get("jsonschema_contentMediaType"); contentMediaType != "" {
		// 8.4 contentMediaType
		schema["contentMediaType"] = contentMediaType
	}
	// TODO: 8.5 contentSchema requires nested schema support

	// 9. A Vocabulary for Basic Meta-Data Annotations
	// https://json-schema.org/draft/2020-12/json-schema-validation#section-9
	if title := field.Tag.Get("jsonschema_title"); title != "" {
		// 9.1 title
		schema["title"] = title
	}
	// Note: 9.2 description is handled in generateFieldSchema
	if defaultValue := field.Tag.Get("jsonschema_default"); defaultValue != "" {
		// 9.3 default
		schema["default"] = defaultValue
	}
	if deprecated := field.Tag.Get("jsonschema_deprecated"); deprecated == "true" {
		// 9.4 deprecated
		schema["deprecated"] = true
	}
	if readOnly := field.Tag.Get("jsonschema_readOnly"); readOnly == "true" {
		// 9.5 readOnly
		schema["readOnly"] = true
	}
	if writeOnly := field.Tag.Get("jsonschema_writeOnly"); writeOnly == "true" {
		// 9.6 writeOnly
		schema["writeOnly"] = true
	}
	if examples := field.Tag.Get("jsonschema_examples"); examples != "" {
		// 9.7 examples
		exampleValues := strings.Split(examples, ",")
		schema["examples"] = exampleValues
	}

	// TODO: Conditional validation keywords (allOf, anyOf, oneOf, not, if, then, else)
	// are complex and would require significant structural changes to support
}
