package yup

import (
	"context"
	"io"
)

type Command interface {
	Execute(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer) error
}
