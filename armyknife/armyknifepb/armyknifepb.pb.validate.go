// Code generated by protoc-gen-validate. DO NOT EDIT.
// source: armyknifepb/armyknifepb.proto

package armyknifepb

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"net/mail"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/golang/protobuf/ptypes"
)

// ensure the imports are used
var (
	_ = bytes.MinRead
	_ = errors.New("")
	_ = fmt.Print
	_ = utf8.UTFMax
	_ = (*regexp.Regexp)(nil)
	_ = (*strings.Reader)(nil)
	_ = net.IPv4len
	_ = time.Duration(0)
	_ = (*url.URL)(nil)
	_ = (*mail.Address)(nil)
	_ = ptypes.DynamicAny{}
)

// define the regex for a UUID once up-front
var _armyknifepb_uuidPattern = regexp.MustCompile("^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$")

// Validate checks the field values on DelayMessage with the rules defined in
// the proto definition for this message. If any rules are violated, an error
// is returned.
func (m *DelayMessage) Validate() error {
	if m == nil {
		return nil
	}

	// no validation rules for Delay

	return nil
}

// DelayMessageValidationError is the validation error returned by
// DelayMessage.Validate if the designated constraints aren't met.
type DelayMessageValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e DelayMessageValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e DelayMessageValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e DelayMessageValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e DelayMessageValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e DelayMessageValidationError) ErrorName() string { return "DelayMessageValidationError" }

// Error satisfies the builtin error interface
func (e DelayMessageValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sDelayMessage.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = DelayMessageValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = DelayMessageValidationError{}

// Validate checks the field values on DelayResponse with the rules defined in
// the proto definition for this message. If any rules are violated, an error
// is returned.
func (m *DelayResponse) Validate() error {
	if m == nil {
		return nil
	}

	return nil
}

// DelayResponseValidationError is the validation error returned by
// DelayResponse.Validate if the designated constraints aren't met.
type DelayResponseValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e DelayResponseValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e DelayResponseValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e DelayResponseValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e DelayResponseValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e DelayResponseValidationError) ErrorName() string { return "DelayResponseValidationError" }

// Error satisfies the builtin error interface
func (e DelayResponseValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sDelayResponse.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = DelayResponseValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = DelayResponseValidationError{}

// Validate checks the field values on StatusMessage with the rules defined in
// the proto definition for this message. If any rules are violated, an error
// is returned.
func (m *StatusMessage) Validate() error {
	if m == nil {
		return nil
	}

	// no validation rules for Code

	return nil
}

// StatusMessageValidationError is the validation error returned by
// StatusMessage.Validate if the designated constraints aren't met.
type StatusMessageValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e StatusMessageValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e StatusMessageValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e StatusMessageValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e StatusMessageValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e StatusMessageValidationError) ErrorName() string { return "StatusMessageValidationError" }

// Error satisfies the builtin error interface
func (e StatusMessageValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sStatusMessage.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = StatusMessageValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = StatusMessageValidationError{}

// Validate checks the field values on StatusResponse with the rules defined in
// the proto definition for this message. If any rules are violated, an error
// is returned.
func (m *StatusResponse) Validate() error {
	if m == nil {
		return nil
	}

	return nil
}

// StatusResponseValidationError is the validation error returned by
// StatusResponse.Validate if the designated constraints aren't met.
type StatusResponseValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e StatusResponseValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e StatusResponseValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e StatusResponseValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e StatusResponseValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e StatusResponseValidationError) ErrorName() string { return "StatusResponseValidationError" }

// Error satisfies the builtin error interface
func (e StatusResponseValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sStatusResponse.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = StatusResponseValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = StatusResponseValidationError{}
