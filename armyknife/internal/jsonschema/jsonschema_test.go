package jsonschema_test

import (
	"reflect"
	"testing"

	"armyknife/internal/jsonschema"
)

func TestGenerator_Generate(t *testing.T) {
	tests := []struct {
		name    string
		input   interface{}
		want    map[string]interface{}
		wantErr bool
	}{
		// Basic struct tests
		{
			name: "simple struct",
			input: struct {
				Name string `json:"name"`
				Age  int    `json:"age"`
			}{},
			want: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{"type": "string"},
					"age":  map[string]interface{}{"type": "integer"},
				},
				"required":             []string{"name", "age"},
				"additionalProperties": false,
			},
			wantErr: false,
		},
		{
			name: "struct with omitempty",
			input: struct {
				Name     string `json:"name"`
				Optional string `json:"optional,omitempty"`
			}{},
			want: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name":     map[string]interface{}{"type": "string"},
					"optional": map[string]interface{}{"type": "string"},
				},
				"required":             []string{"name"},
				"additionalProperties": false,
			},
			wantErr: false,
		},
		{
			name: "struct with ignore tag",
			input: struct {
				Name   string `json:"name"`
				Ignore string `json:"-"`
			}{},
			want: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{"type": "string"},
				},
				"required":             []string{"name"},
				"additionalProperties": false,
			},
			wantErr: false,
		},
		{
			name: "struct with no json tag",
			input: struct {
				Name string
			}{},
			want: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"Name": map[string]interface{}{"type": "string"},
				},
				"required":             []string{"Name"},
				"additionalProperties": false,
			},
			wantErr: false,
		},
		{
			name: "struct with empty json tag",
			input: struct {
				Name string `json:""`
			}{},
			want: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"Name": map[string]interface{}{"type": "string"},
				},
				"required":             []string{"Name"},
				"additionalProperties": false,
			},
			wantErr: false,
		},
		{
			name: "pointer to struct",
			input: &struct {
				Name string `json:"name"`
			}{},
			want: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{"type": "string"},
				},
				"required":             []string{"name"},
				"additionalProperties": false,
			},
			wantErr: false,
		},
		{
			name: "struct with unexported field",
			input: struct {
				Name     string `json:"name"`
				internal string
			}{},
			want: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{"type": "string"},
				},
				"required":             []string{"name"},
				"additionalProperties": false,
			},
			wantErr: false,
		},
		{
			name: "all optional fields",
			input: struct {
				Name     string `json:"name,omitempty"`
				Age      int    `json:"age,omitempty"`
				Optional bool   `json:"optional,omitempty"`
			}{},
			want: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name":     map[string]interface{}{"type": "string"},
					"age":      map[string]interface{}{"type": "integer"},
					"optional": map[string]interface{}{"type": "boolean"},
				},
				"additionalProperties": false,
			},
			wantErr: false,
		},

		// Type tests
		{
			name: "all primitive types",
			input: struct {
				String  string  `json:"string"`
				Int     int     `json:"int"`
				Int8    int8    `json:"int8"`
				Int16   int16   `json:"int16"`
				Int32   int32   `json:"int32"`
				Int64   int64   `json:"int64"`
				Uint    uint    `json:"uint"`
				Uint8   uint8   `json:"uint8"`
				Uint16  uint16  `json:"uint16"`
				Uint32  uint32  `json:"uint32"`
				Uint64  uint64  `json:"uint64"`
				Float32 float32 `json:"float32"`
				Float64 float64 `json:"float64"`
				Bool    bool    `json:"bool"`
			}{},
			want: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"string":  map[string]interface{}{"type": "string"},
					"int":     map[string]interface{}{"type": "integer"},
					"int8":    map[string]interface{}{"type": "integer"},
					"int16":   map[string]interface{}{"type": "integer"},
					"int32":   map[string]interface{}{"type": "integer"},
					"int64":   map[string]interface{}{"type": "integer"},
					"uint":    map[string]interface{}{"type": "integer"},
					"uint8":   map[string]interface{}{"type": "integer"},
					"uint16":  map[string]interface{}{"type": "integer"},
					"uint32":  map[string]interface{}{"type": "integer"},
					"uint64":  map[string]interface{}{"type": "integer"},
					"float32": map[string]interface{}{"type": "number"},
					"float64": map[string]interface{}{"type": "number"},
					"bool":    map[string]interface{}{"type": "boolean"},
				},
				"required": []string{"string", "int", "int8", "int16", "int32", "int64",
					"uint", "uint8", "uint16", "uint32", "uint64", "float32", "float64", "bool"},
				"additionalProperties": false,
			},
			wantErr: false,
		},

		// Nested struct tests
		{
			name: "nested structs",
			input: struct {
				Name    string `json:"name"`
				Address struct {
					Street string `json:"street"`
					City   string `json:"city"`
				} `json:"address"`
			}{},
			want: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{"type": "string"},
					"address": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"street": map[string]interface{}{"type": "string"},
							"city":   map[string]interface{}{"type": "string"},
						},
						"required":             []string{"street", "city"},
						"additionalProperties": false,
					},
				},
				"required":             []string{"name", "address"},
				"additionalProperties": false,
			},
			wantErr: false,
		},

		// Pointer field tests
		{
			name: "pointer fields",
			input: struct {
				Name     string  `json:"name"`
				Optional *string `json:"optional,omitempty"`
				Count    *int    `json:"count"`
			}{},
			want: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name":     map[string]interface{}{"type": "string"},
					"optional": map[string]interface{}{"type": "string"},
					"count":    map[string]interface{}{"type": "integer"},
				},
				"required":             []string{"name", "count"},
				"additionalProperties": false,
			},
			wantErr: false,
		},

		// Slice and array tests
		{
			name: "slice fields",
			input: struct {
				Title string   `json:"title"`
				Tags  []string `json:"tags"`
				Nums  []int    `json:"nums,omitempty"`
			}{},
			want: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"title": map[string]interface{}{"type": "string"},
					"tags": map[string]interface{}{
						"type":  "array",
						"items": map[string]interface{}{"type": "string"},
					},
					"nums": map[string]interface{}{
						"type":  "array",
						"items": map[string]interface{}{"type": "integer"},
					},
				},
				"required":             []string{"title", "tags"},
				"additionalProperties": false,
			},
			wantErr: false,
		},
		{
			name: "array fields",
			input: struct {
				FixedStrings [3]string `json:"fixed_strings"`
				FixedInts    [5]int    `json:"fixed_ints"`
			}{},
			want: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"fixed_strings": map[string]interface{}{
						"type":  "array",
						"items": map[string]interface{}{"type": "string"},
					},
					"fixed_ints": map[string]interface{}{
						"type":  "array",
						"items": map[string]interface{}{"type": "integer"},
					},
				},
				"required":             []string{"fixed_strings", "fixed_ints"},
				"additionalProperties": false,
			},
			wantErr: false,
		},

		// Map tests
		{
			name: "map fields",
			input: struct {
				Settings map[string]string `json:"settings"`
				Counts   map[string]int    `json:"counts,omitempty"`
			}{},
			want: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"settings": map[string]interface{}{
						"type":                 "object",
						"additionalProperties": map[string]interface{}{"type": "string"},
					},
					"counts": map[string]interface{}{
						"type":                 "object",
						"additionalProperties": map[string]interface{}{"type": "integer"},
					},
				},
				"required":             []string{"settings"},
				"additionalProperties": false,
			},
			wantErr: false,
		},

		// dependentRequired tests
		{
			name: "simple dependentRequired",
			input: struct {
				CreditCard     string `json:"credit_card,omitempty" jsonschema_dependentRequired:"billing_address"`
				BillingAddress string `json:"billing_address,omitempty"`
			}{},
			want: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"credit_card":     map[string]interface{}{"type": "string"},
					"billing_address": map[string]interface{}{"type": "string"},
				},
				"additionalProperties": false,
				"dependentRequired": map[string][]string{
					"credit_card": {"billing_address"},
				},
			},
			wantErr: false,
		},
		{
			name: "multiple dependentRequired",
			input: struct {
				Name           string `json:"name"`
				CreditCard     string `json:"credit_card,omitempty" jsonschema_dependentRequired:"billing_address,billing_zip"`
				BillingAddress string `json:"billing_address,omitempty"`
				BillingZip     string `json:"billing_zip,omitempty"`
			}{},
			want: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name":            map[string]interface{}{"type": "string"},
					"credit_card":     map[string]interface{}{"type": "string"},
					"billing_address": map[string]interface{}{"type": "string"},
					"billing_zip":     map[string]interface{}{"type": "string"},
				},
				"required":             []string{"name"},
				"additionalProperties": false,
				"dependentRequired": map[string][]string{
					"credit_card": {"billing_address", "billing_zip"},
				},
			},
			wantErr: false,
		},
		{
			name: "multiple fields with dependentRequired",
			input: struct {
				Name          string   `json:"name"`
				HasSpouse     bool     `json:"has_spouse,omitempty" jsonschema_dependentRequired:"spouse_name"`
				SpouseName    string   `json:"spouse_name,omitempty"`
				HasChildren   bool     `json:"has_children,omitempty" jsonschema_dependentRequired:"children_names,children_count"`
				ChildrenNames []string `json:"children_names,omitempty"`
				ChildrenCount int      `json:"children_count,omitempty"`
			}{},
			want: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name":         map[string]interface{}{"type": "string"},
					"has_spouse":   map[string]interface{}{"type": "boolean"},
					"spouse_name":  map[string]interface{}{"type": "string"},
					"has_children": map[string]interface{}{"type": "boolean"},
					"children_names": map[string]interface{}{
						"type":  "array",
						"items": map[string]interface{}{"type": "string"},
					},
					"children_count": map[string]interface{}{"type": "integer"},
				},
				"required":             []string{"name"},
				"additionalProperties": false,
				"dependentRequired": map[string][]string{
					"has_spouse":   {"spouse_name"},
					"has_children": {"children_names", "children_count"},
				},
			},
			wantErr: false,
		},
		{
			name: "dependentRequired with spaces in tag",
			input: struct {
				PaymentMethod string `json:"payment_method" jsonschema_dependentRequired:"card_number, card_cvv, card_expiry"`
				CardNumber    string `json:"card_number,omitempty"`
				CardCvv       string `json:"card_cvv,omitempty"`
				CardExpiry    string `json:"card_expiry,omitempty"`
			}{},
			want: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"payment_method": map[string]interface{}{"type": "string"},
					"card_number":    map[string]interface{}{"type": "string"},
					"card_cvv":       map[string]interface{}{"type": "string"},
					"card_expiry":    map[string]interface{}{"type": "string"},
				},
				"required":             []string{"payment_method"},
				"additionalProperties": false,
				"dependentRequired": map[string][]string{
					"payment_method": {"card_number", "card_cvv", "card_expiry"},
				},
			},
			wantErr: false,
		},
		{
			name: "dependentRequired with nested struct",
			input: struct {
				UseShipping bool `json:"use_shipping" jsonschema_dependentRequired:"shipping"`
				Shipping    struct {
					Address string `json:"address"`
					City    string `json:"city"`
					Zip     string `json:"zip"`
				} `json:"shipping,omitempty"`
			}{},
			want: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"use_shipping": map[string]interface{}{"type": "boolean"},
					"shipping": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"address": map[string]interface{}{"type": "string"},
							"city":    map[string]interface{}{"type": "string"},
							"zip":     map[string]interface{}{"type": "string"},
						},
						"required":             []string{"address", "city", "zip"},
						"additionalProperties": false,
					},
				},
				"required":             []string{"use_shipping"},
				"additionalProperties": false,
				"dependentRequired": map[string][]string{
					"use_shipping": {"shipping"},
				},
			},
			wantErr: false,
		},
		{
			name: "dependentRequired with validation tags",
			input: struct {
				AccountType string `json:"account_type" jsonschema_enum:"personal,business" jsonschema_dependentRequired:"tax_id"`
				TaxId       string `json:"tax_id,omitempty" jsonschema_pattern:"^[0-9]{9}$"`
			}{},
			want: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"account_type": map[string]interface{}{
						"type": "string",
						"enum": []string{"personal", "business"},
					},
					"tax_id": map[string]interface{}{
						"type":    "string",
						"pattern": "^[0-9]{9}$",
					},
				},
				"required":             []string{"account_type"},
				"additionalProperties": false,
				"dependentRequired": map[string][]string{
					"account_type": {"tax_id"},
				},
			},
			wantErr: false,
		},

		// Interface tests
		{
			name: "interface fields",
			input: struct {
				Any interface{} `json:"any"`
			}{},
			want: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"any": map[string]interface{}{},
				},
				"required":             []string{"any"},
				"additionalProperties": false,
			},
			wantErr: false,
		},

		// Validation tag tests
		{
			name: "string validation tags",
			input: struct {
				Email string `json:"email" jsonschema_format:"email" jsonschema_minLength:"5" jsonschema_maxLength:"50" jsonschema_pattern:"^[a-zA-Z0-9]+@"`
			}{},
			want: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"email": map[string]interface{}{
						"type":      "string",
						"format":    "email",
						"minLength": 5,
						"maxLength": 50,
						"pattern":   "^[a-zA-Z0-9]+@",
					},
				},
				"required":             []string{"email"},
				"additionalProperties": false,
			},
			wantErr: false,
		},
		{
			name: "numeric validation tags",
			input: struct {
				Age    int     `json:"age" jsonschema_minimum:"0" jsonschema_maximum:"150"`
				Score  float64 `json:"score" jsonschema_minimum:"0" jsonschema_maximum:"100" jsonschema_multipleOf:"0.5"`
				Height float64 `json:"height" jsonschema_exclusiveMinimum:"0" jsonschema_exclusiveMaximum:"300"`
			}{},
			want: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"age": map[string]interface{}{
						"type":    "integer",
						"minimum": 0.0,
						"maximum": 150.0,
					},
					"score": map[string]interface{}{
						"type":       "number",
						"minimum":    0.0,
						"maximum":    100.0,
						"multipleOf": 0.5,
					},
					"height": map[string]interface{}{
						"type":             "number",
						"exclusiveMinimum": 0.0,
						"exclusiveMaximum": 300.0,
					},
				},
				"required":             []string{"age", "score", "height"},
				"additionalProperties": false,
			},
			wantErr: false,
		},
		{
			name: "array validation tags",
			input: struct {
				Tags    []string `json:"tags" jsonschema_minItems:"1" jsonschema_maxItems:"10" jsonschema_uniqueItems:"true"`
				Numbers []int    `json:"numbers" jsonschema_minItems:"0" jsonschema_maxItems:"5"`
			}{},
			want: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"tags": map[string]interface{}{
						"type":        "array",
						"items":       map[string]interface{}{"type": "string"},
						"minItems":    1,
						"maxItems":    10,
						"uniqueItems": true,
					},
					"numbers": map[string]interface{}{
						"type":     "array",
						"items":    map[string]interface{}{"type": "integer"},
						"minItems": 0,
						"maxItems": 5,
					},
				},
				"required":             []string{"tags", "numbers"},
				"additionalProperties": false,
			},
			wantErr: false,
		},
		{
			name: "object validation tags",
			input: struct {
				Config map[string]string `json:"config" jsonschema_minProperties:"1" jsonschema_maxProperties:"10"`
			}{},
			want: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"config": map[string]interface{}{
						"type":                 "object",
						"additionalProperties": map[string]interface{}{"type": "string"},
						"minProperties":        1,
						"maxProperties":        10,
					},
				},
				"required":             []string{"config"},
				"additionalProperties": false,
			},
			wantErr: false,
		},
		{
			name: "enum and const tags",
			input: struct {
				Status   string `json:"status" jsonschema_enum:"active,inactive,pending"`
				Version  string `json:"version" jsonschema_const:"1.0.0"`
				Priority int    `json:"priority" jsonschema_enum:"1,2,3"`
			}{},
			want: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"status": map[string]interface{}{
						"type": "string",
						"enum": []string{"active", "inactive", "pending"},
					},
					"version": map[string]interface{}{
						"type":  "string",
						"const": "1.0.0",
					},
					"priority": map[string]interface{}{
						"type": "integer",
						"enum": []string{"1", "2", "3"},
					},
				},
				"required":             []string{"status", "version", "priority"},
				"additionalProperties": false,
			},
			wantErr: false,
		},
		{
			name: "metadata tags",
			input: struct {
				Name        string `json:"name" jsonschema_title:"User Name" jsonschema_description:"The user's full name" jsonschema_default:"John Doe"`
				Email       string `json:"email" jsonschema_deprecated:"true" jsonschema_examples:"john@example.com,jane@example.com"`
				ReadOnlyID  string `json:"id" jsonschema_readOnly:"true"`
				WriteSecret string `json:"secret" jsonschema_writeOnly:"true"`
			}{},
			want: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{
						"type":        "string",
						"title":       "User Name",
						"description": "The user's full name",
						"default":     "John Doe",
					},
					"email": map[string]interface{}{
						"type":       "string",
						"deprecated": true,
						"examples":   []string{"john@example.com", "jane@example.com"},
					},
					"id": map[string]interface{}{
						"type":     "string",
						"readOnly": true,
					},
					"secret": map[string]interface{}{
						"type":      "string",
						"writeOnly": true,
					},
				},
				"required":             []string{"name", "email", "id", "secret"},
				"additionalProperties": false,
			},
			wantErr: false,
		},
		{
			name: "content validation tags",
			input: struct {
				Image string `json:"image" jsonschema_contentEncoding:"base64" jsonschema_contentMediaType:"image/png"`
				JSON  string `json:"json" jsonschema_contentMediaType:"application/json"`
			}{},
			want: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"image": map[string]interface{}{
						"type":             "string",
						"contentEncoding":  "base64",
						"contentMediaType": "image/png",
					},
					"json": map[string]interface{}{
						"type":             "string",
						"contentMediaType": "application/json",
					},
				},
				"required":             []string{"image", "json"},
				"additionalProperties": false,
			},
			wantErr: false,
		},
		{
			name: "format validation tags",
			input: struct {
				Date     string `json:"date" jsonschema_format:"date"`
				DateTime string `json:"datetime" jsonschema_format:"date-time"`
				Email    string `json:"email" jsonschema_format:"email"`
				UUID     string `json:"uuid" jsonschema_format:"uuid"`
				URI      string `json:"uri" jsonschema_format:"uri"`
				IPv4     string `json:"ipv4" jsonschema_format:"ipv4"`
				IPv6     string `json:"ipv6" jsonschema_format:"ipv6"`
			}{},
			want: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"date":     map[string]interface{}{"type": "string", "format": "date"},
					"datetime": map[string]interface{}{"type": "string", "format": "date-time"},
					"email":    map[string]interface{}{"type": "string", "format": "email"},
					"uuid":     map[string]interface{}{"type": "string", "format": "uuid"},
					"uri":      map[string]interface{}{"type": "string", "format": "uri"},
					"ipv4":     map[string]interface{}{"type": "string", "format": "ipv4"},
					"ipv6":     map[string]interface{}{"type": "string", "format": "ipv6"},
				},
				"required":             []string{"date", "datetime", "email", "uuid", "uri", "ipv4", "ipv6"},
				"additionalProperties": false,
			},
			wantErr: false,
		},
		{
			name: "invalid numeric tags ignored",
			input: struct {
				BadMin      int     `json:"bad_min" jsonschema_minimum:"not-a-number"`
				BadMax      int     `json:"bad_max" jsonschema_maximum:"not-a-number"`
				BadMultiple float64 `json:"bad_multiple" jsonschema_multipleOf:"not-a-number"`
			}{},
			want: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"bad_min":      map[string]interface{}{"type": "integer"},
					"bad_max":      map[string]interface{}{"type": "integer"},
					"bad_multiple": map[string]interface{}{"type": "number"},
				},
				"required":             []string{"bad_min", "bad_max", "bad_multiple"},
				"additionalProperties": false,
			},
			wantErr: false,
		},
		{
			name: "invalid integer tags ignored",
			input: struct {
				BadMinLen   string   `json:"bad_minlen" jsonschema_minLength:"not-a-number"`
				BadMaxLen   string   `json:"bad_maxlen" jsonschema_maxLength:"not-a-number"`
				BadMinItems []string `json:"bad_minitems" jsonschema_minItems:"not-a-number"`
				BadMaxItems []string `json:"bad_maxitems" jsonschema_maxItems:"not-a-number"`
			}{},
			want: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"bad_minlen":   map[string]interface{}{"type": "string"},
					"bad_maxlen":   map[string]interface{}{"type": "string"},
					"bad_minitems": map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
					"bad_maxitems": map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
				},
				"required":             []string{"bad_minlen", "bad_maxlen", "bad_minitems", "bad_maxitems"},
				"additionalProperties": false,
			},
			wantErr: false,
		},

		// Complex nested struct test
		{
			name: "complex nested structs",
			input: struct {
				ID       string `json:"id" jsonschema_format:"uuid"`
				Username string `json:"username" jsonschema_pattern:"^[a-zA-Z0-9_]+$"`
				Email    string `json:"email" jsonschema_format:"email"`
				Roles    []struct {
					Name        string `json:"name" jsonschema_minLength:"3"`
					Description string `json:"description,omitempty"`
					Permissions []struct {
						Name     string   `json:"name"`
						Resource string   `json:"resource"`
						Actions  []string `json:"actions" jsonschema_minItems:"1"`
					} `json:"permissions"`
				} `json:"roles"`
				Settings map[string]interface{} `json:"settings,omitempty"`
				Active   bool                   `json:"active"`
				Age      *int                   `json:"age,omitempty" jsonschema_minimum:"0" jsonschema_maximum:"150"`
			}{},
			want: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":   "string",
						"format": "uuid",
					},
					"username": map[string]interface{}{
						"type":    "string",
						"pattern": "^[a-zA-Z0-9_]+$",
					},
					"email": map[string]interface{}{
						"type":   "string",
						"format": "email",
					},
					"roles": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"name": map[string]interface{}{
									"type":      "string",
									"minLength": 3,
								},
								"description": map[string]interface{}{
									"type": "string",
								},
								"permissions": map[string]interface{}{
									"type": "array",
									"items": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"name":     map[string]interface{}{"type": "string"},
											"resource": map[string]interface{}{"type": "string"},
											"actions": map[string]interface{}{
												"type":     "array",
												"items":    map[string]interface{}{"type": "string"},
												"minItems": 1,
											},
										},
										"required":             []string{"name", "resource", "actions"},
										"additionalProperties": false,
									},
								},
							},
							"required":             []string{"name", "permissions"},
							"additionalProperties": false,
						},
					},
					"settings": map[string]interface{}{
						"type":                 "object",
						"additionalProperties": map[string]interface{}{},
					},
					"active": map[string]interface{}{
						"type": "boolean",
					},
					"age": map[string]interface{}{
						"type":    "integer",
						"minimum": 0.0,
						"maximum": 150.0,
					},
				},
				"required":             []string{"id", "username", "email", "roles", "active"},
				"additionalProperties": false,
			},
			wantErr: false,
		},

		// Error cases
		{
			name:    "non-struct input",
			input:   "string",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "slice input",
			input:   []string{"a", "b"},
			want:    nil,
			wantErr: true,
		},
		{
			name: "struct with channel field",
			input: struct {
				Ch chan int `json:"ch"`
			}{},
			want:    nil,
			wantErr: true,
		},
		{
			name: "struct with func field",
			input: struct {
				Fn func() `json:"fn"`
			}{},
			want:    nil,
			wantErr: true,
		},
		{
			name: "struct with slice of unsupported type",
			input: struct {
				Chans []chan int `json:"chans"`
			}{},
			want:    nil,
			wantErr: true,
		},
		{
			name: "struct with map of unsupported type",
			input: struct {
				Funcs map[string]func() `json:"funcs"`
			}{},
			want:    nil,
			wantErr: true,
		},
		{
			name: "struct with nested struct containing unsupported type",
			input: struct {
				Nested struct {
					Ch chan int `json:"ch"`
				} `json:"nested"`
			}{},
			want:    nil,
			wantErr: true,
		},
		{
			name: "struct with pointer to unsupported type",
			input: struct {
				ChPtr *chan int `json:"ch_ptr"`
			}{},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		name := tt.name
		input := tt.input
		want := tt.want
		wantErr := tt.wantErr
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			g := jsonschema.NewGenerator()
			got, err := g.Generate(input)
			if err != nil {
				if !wantErr {
					t.Errorf("Generator.Generate() error = %+v", err)
				}
				return
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("Generator.Generate() = %s, want %s", got, want)
			}
		})
	}
}
