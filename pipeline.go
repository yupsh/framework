package yup

import (
	"context"
	"io"
	"sync"

	"github.com/yupsh/framework/opt"
)

// Pipeline represents a sequence of commands connected by pipes
type Pipeline struct {
	commands []Command
	flags    ExecutionFlags
}

// ExecutionFlags controls how the pipeline is executed
type ExecutionFlags struct {
	PipeFail bool // Fail pipeline if any command fails
	Buffered bool // Use buffered I/O between commands
	Verbose  bool // Verbose execution logging
	DryRun   bool // Show commands without executing
	MaxProcs int  // Maximum number of parallel processes
}

// Execution flag types
type PipeFailFlag bool

const (
	PipeFail   PipeFailFlag = true
	NoPipeFail PipeFailFlag = false
)

type BufferedFlag bool

const (
	Buffered   BufferedFlag = true
	Unbuffered BufferedFlag = false
)

type VerboseFlag bool

const (
	Verbose VerboseFlag = true
	Quiet   VerboseFlag = false
)

type DryRunFlag bool

const (
	DryRun   DryRunFlag = true
	NoDryRun DryRunFlag = false
)

// MaxProcs represents max parallel processes
type MaxProcs int

// Flag configuration methods
func (f PipeFailFlag) Configure(flags *ExecutionFlags) { flags.PipeFail = bool(f) }
func (f BufferedFlag) Configure(flags *ExecutionFlags) { flags.Buffered = bool(f) }
func (f VerboseFlag) Configure(flags *ExecutionFlags)  { flags.Verbose = bool(f) }
func (f DryRunFlag) Configure(flags *ExecutionFlags)   { flags.DryRun = bool(f) }
func (m MaxProcs) Configure(flags *ExecutionFlags)     { flags.MaxProcs = int(m) }

// NewPipeline creates a new pipeline with the given commands
func NewPipeline(commands ...Command) *Pipeline {
	return &Pipeline{
		commands: commands,
		flags: ExecutionFlags{
			MaxProcs: 1, // Default to sequential execution
		},
	}
}

// WithFlags applies execution flags to the pipeline
func (p *Pipeline) WithFlags(configurers ...opt.Switch[ExecutionFlags]) *Pipeline {
	p.flags = configure(configurers...)
	return p
}

// configure is a helper function to apply configuration switches
func configure[T any](opts ...opt.Switch[T]) T {
	def := new(T)
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		opt.Configure(def)
	}
	return *def
}

// Execute runs the pipeline with the given input/output
func (p *Pipeline) Execute(ctx context.Context, input io.Reader, output, stderr io.Writer) error {
	if len(p.commands) == 0 {
		return nil
	}

	if len(p.commands) == 1 {
		return p.commands[0].Execute(ctx, input, output, stderr)
	}

	// Create pipes between commands
	pipes := make([]*io.PipeWriter, len(p.commands)-1)
	readers := make([]*io.PipeReader, len(p.commands)-1)

	for i := 0; i < len(p.commands)-1; i++ {
		readers[i], pipes[i] = io.Pipe()
	}

	// Error collection
	var wg sync.WaitGroup
	errChan := make(chan error, len(p.commands))

	// Execute commands
	for i, cmd := range p.commands {
		wg.Add(1)
		go func(i int, cmd Command) {
			defer wg.Done()

			var cmdInput io.Reader
			var cmdOutput io.Writer

			// Set input
			if i == 0 {
				cmdInput = input
			} else {
				cmdInput = readers[i-1]
			}

			// Set output
			if i == len(p.commands)-1 {
				cmdOutput = output
			} else {
				cmdOutput = pipes[i]
			}

			// Execute command
			err := cmd.Execute(ctx, cmdInput, cmdOutput, stderr)

			// Close output pipe if not the last command
			if i < len(p.commands)-1 {
				pipes[i].Close()
			}

			// Handle errors based on pipefail setting
			if err != nil {
				if p.flags.PipeFail {
					errChan <- err
				}
			}
		}(i, cmd)
	}

	// Wait for all commands to complete
	go func() {
		wg.Wait()
		close(errChan)
	}()

	// Collect errors
	var firstErr error
	for err := range errChan {
		if firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}

// Pipe creates a pipeline from multiple commands (convenience function)
func Pipe(commands ...Command) *Pipeline {
	return NewPipeline(commands...)
}

// Exec executes a single command (convenience function)
func Exec(cmd Command) *Pipeline {
	return NewPipeline(cmd)
}
