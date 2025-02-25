package yup

import (
	"context"
	"io"
)

type Command interface {
	Execute(ctx context.Context, input io.Reader, output, stderr io.Writer) error
}
