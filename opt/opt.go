package opt

import (
	"fmt"
	"log/slog"
)

type Inputs[T any, O any] struct {
	Positional []T
	Flags      O
}

type Switch[T any] interface {
	Configure(*T)
}

func configure[T any](opts ...Switch[T]) T {
	def := new(T)
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		opt.Configure(def)
	}
	return *def
}

func Args[T any, O any](parameters ...any) Inputs[T, O] {
	var (
		inputs  []T
		options []Switch[O]
	)
	for _, arg := range parameters {
		switch v := arg.(type) {
		case T:
			inputs = append(inputs, v)
		case Switch[O]:
			options = append(options, v)
		default:
			slog.Warn("Unknown argument type", "arg", v, "type", fmt.Sprintf("%T/%T", arg, v))
		}
	}
	return Inputs[T, O]{
		Positional: inputs,
		Flags:      configure(options...),
	}
}
