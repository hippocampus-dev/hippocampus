package handler

import (
	"fmt"

	"golang.org/x/xerrors"
)

type NotFoundError struct {
	err   error
	frame xerrors.Frame
}

func NewNotFoundError(err error) *NotFoundError {
	return &NotFoundError{
		err:   err,
		frame: xerrors.Caller(1),
	}
}

func (e *NotFoundError) Error() string {
	return "handler is not found"
}

func (e *NotFoundError) Unwrap() error {
	return e.err
}

func (e *NotFoundError) Format(f fmt.State, c rune) {
	xerrors.FormatError(e, f, c)
}

func (e *NotFoundError) FormatError(p xerrors.Printer) error {
	p.Print(e.Error())
	e.frame.Format(p)
	return e.err
}
