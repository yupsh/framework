package yup

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
)

// InputSource represents a source of input data
type InputSource struct {
	Reader   io.Reader
	Filename string
	File     *os.File // nil for stdin
}

// Close closes the underlying file if it exists
func (is InputSource) Close() error {
	if is.File != nil {
		return is.File.Close()
	}
	return nil
}

// ProcessorFunc is a function that processes a single input source
type ProcessorFunc func(source InputSource, output io.Writer) error

// FileProcessorOptions controls file processing behavior
type FileProcessorOptions struct {
	CommandName     string // e.g., "cat", "head" - used for error messages
	ShowHeaders     bool   // Show file headers for multiple files
	HeaderFormat    string // Format string for headers (default: "==> %s <==\n")
	BlankBetween    bool   // Add blank line between files
	ContinueOnError bool   // Continue processing other files on error
}

// ProcessFiles handles the common pattern of processing stdin or multiple files
func ProcessFiles(
	positionalArgs []string,
	stdin io.Reader,
	output, stderr io.Writer,
	options FileProcessorOptions,
	processor ProcessorFunc,
) error {
	// Set defaults
	if options.HeaderFormat == "" {
		options.HeaderFormat = "==> %s <==\n"
	}

	// If no files specified, read from stdin
	if len(positionalArgs) == 0 {
		source := InputSource{Reader: stdin, Filename: "stdin"}
		return processor(source, output)
	}

	multipleFiles := len(positionalArgs) > 1 && options.ShowHeaders
	var lastError error

	// Process each file
	for i, filename := range positionalArgs {
		var source InputSource

		if filename == "-" {
			source = InputSource{Reader: stdin, Filename: "stdin"}
		} else {
			file, err := os.Open(filename)
			if err != nil {
				ErrorF(stderr, options.CommandName, filename, err)
				if options.ContinueOnError {
					lastError = err
					continue
				}
				return err
			}
			source = InputSource{Reader: file, Filename: filename, File: file}
		}

		// Show header if needed
		if multipleFiles {
			if i > 0 && options.BlankBetween {
				_, _ = fmt.Fprintln(output)
			}
			_, _ = fmt.Fprintf(output, options.HeaderFormat, source.Filename)
		}

		// Process the source
		err := processor(source, output)

		// Close file if it was opened
		if closeErr := source.Close(); closeErr != nil && err == nil {
			err = closeErr
		}

		if err != nil {
			ErrorF(stderr, options.CommandName, source.Filename, err)
			if options.ContinueOnError {
				lastError = err
				continue
			}
			return err
		}
	}

	return lastError
}

// LineProcessor is a function that processes individual lines
type LineProcessor func(lineNum int, line string, output io.Writer) error

// ProcessLines reads lines from a reader and processes each one
func ProcessLines(reader io.Reader, output io.Writer, processor LineProcessor) error {
	scanner := bufio.NewScanner(reader)
	lineNum := 1

	for scanner.Scan() {
		if err := processor(lineNum, scanner.Text(), output); err != nil {
			return err
		}
		lineNum++
	}

	return scanner.Err()
}

// ReadAllLines reads all lines from a reader into a slice
func ReadAllLines(reader io.Reader) ([]string, error) {
	var lines []string
	scanner := bufio.NewScanner(reader)

	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	return lines, scanner.Err()
}

// CollectInputSources collects multiple input sources (stdin + files)
func CollectInputSources(positionalArgs []string, stdin io.Reader) ([]InputSource, error) {
	var sources []InputSource

	if len(positionalArgs) == 0 {
		sources = append(sources, InputSource{Reader: stdin, Filename: "stdin"})
		return sources, nil
	}

	for _, filename := range positionalArgs {
		if filename == "-" {
			sources = append(sources, InputSource{Reader: stdin, Filename: "stdin"})
		} else {
			file, err := os.Open(filename)
			if err != nil {
				return nil, fmt.Errorf("cannot open %s: %v", filename, err)
			}
			sources = append(sources, InputSource{Reader: file, Filename: filename, File: file})
		}
	}

	return sources, nil
}

// CloseInputSources closes all input sources that have files
func CloseInputSources(sources []InputSource) error {
	var lastError error
	for _, source := range sources {
		if err := source.Close(); err != nil {
			lastError = err
		}
	}
	return lastError
}

// ErrorF formats and prints an error message in the standard format
func ErrorF(stderr io.Writer, commandName, filename string, err error) {
	if filename == "" {
		_, _ = fmt.Fprintf(stderr, "%s: %v\n", commandName, err)
	} else {
		_, _ = fmt.Fprintf(stderr, "%s: %s: %v\n", commandName, filename, err)
	}
}

// ProcessSingleFile handles the common pattern of processing exactly one file or stdin
func ProcessSingleFile(
	positionalArgs []string,
	stdin io.Reader,
	commandName string,
	stderr io.Writer,
	processor func(io.Reader, string) error,
) error {
	if len(positionalArgs) == 0 {
		return processor(stdin, "stdin")
	}

	filename := positionalArgs[0]
	if filename == "-" {
		return processor(stdin, "stdin")
	}

	file, err := os.Open(filename)
	if err != nil {
		ErrorF(stderr, commandName, filename, err)
		return err
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			panic(err)
		}
	}(file)

	return processor(file, filename)
}

// RequireArguments checks that the required number of arguments are provided
func RequireArguments(args []string, min, max int, commandName string, stderr io.Writer) error {
	if len(args) < min {
		if min == max {
			ErrorF(stderr, commandName, "", fmt.Errorf("need exactly %d arguments", min))
		} else {
			ErrorF(stderr, commandName, "", fmt.Errorf("need at least %d arguments", min))
		}
		return fmt.Errorf("insufficient arguments")
	}

	if max > 0 && len(args) > max {
		ErrorF(stderr, commandName, "", fmt.Errorf("too many arguments"))
		return fmt.Errorf("too many arguments")
	}

	return nil
}

// CheckContextCancellation checks if the context has been cancelled and returns an error if so
func CheckContextCancellation(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

// ProcessorFuncWithContext is a function that processes a single input source with context support
type ProcessorFuncWithContext func(ctx context.Context, source InputSource, output io.Writer) error

// ProcessFilesWithContext handles the common pattern of processing stdin or multiple files with context cancellation support
func ProcessFilesWithContext(
	ctx context.Context,
	positionalArgs []string,
	stdin io.Reader,
	output, stderr io.Writer,
	options FileProcessorOptions,
	processor ProcessorFuncWithContext,
) error {
	// Check for cancellation before starting
	if err := CheckContextCancellation(ctx); err != nil {
		return err
	}

	// Set defaults
	if options.HeaderFormat == "" {
		options.HeaderFormat = "==> %s <==\n"
	}

	// If no files specified, read from stdin
	if len(positionalArgs) == 0 {
		source := InputSource{Reader: stdin, Filename: "stdin"}
		return processor(ctx, source, output)
	}

	multipleFiles := len(positionalArgs) > 1 && options.ShowHeaders
	var lastError error

	// Process each file
	for i, filename := range positionalArgs {
		// Check for cancellation before each file
		if err := CheckContextCancellation(ctx); err != nil {
			return err
		}

		var source InputSource

		if filename == "-" {
			source = InputSource{Reader: stdin, Filename: "stdin"}
		} else {
			file, err := os.Open(filename)
			if err != nil {
				ErrorF(stderr, options.CommandName, filename, err)
				if options.ContinueOnError {
					lastError = err
					continue
				}
				return err
			}
			source = InputSource{Reader: file, Filename: filename, File: file}
		}

		// Show header if needed
		if multipleFiles {
			if i > 0 && options.BlankBetween {
				_, _ = fmt.Fprintln(output)
			}
			_, _ = fmt.Fprintf(output, options.HeaderFormat, source.Filename)
		}

		// Process the source
		err := processor(ctx, source, output)

		// Close file if it was opened
		if closeErr := source.Close(); closeErr != nil && err == nil {
			err = closeErr
		}

		if err != nil {
			ErrorF(stderr, options.CommandName, source.Filename, err)
			if options.ContinueOnError {
				lastError = err
				continue
			}
			return err
		}
	}

	return lastError
}

// LineProcessorWithContext is a function that processes individual lines with context support
type LineProcessorWithContext func(ctx context.Context, lineNum int, line string, output io.Writer) error

// ProcessLinesWithContext reads lines from a reader and processes each one with context cancellation support
func ProcessLinesWithContext(ctx context.Context, reader io.Reader, output io.Writer, processor LineProcessorWithContext) error {
	scanner := bufio.NewScanner(reader)
	lineNum := 1

	for scanner.Scan() {
		// Check for cancellation before each line
		if err := CheckContextCancellation(ctx); err != nil {
			return err
		}

		if err := processor(ctx, lineNum, scanner.Text(), output); err != nil {
			return err
		}
		lineNum++
	}

	return scanner.Err()
}

// ProcessSingleFileWithContext handles the common pattern of processing exactly one file or stdin with context support
func ProcessSingleFileWithContext(
	ctx context.Context,
	positionalArgs []string,
	stdin io.Reader,
	commandName string,
	stderr io.Writer,
	processor func(ctx context.Context, reader io.Reader, filename string) error,
) error {
	// Check for cancellation before starting
	if err := CheckContextCancellation(ctx); err != nil {
		return err
	}

	if len(positionalArgs) == 0 {
		return processor(ctx, stdin, "stdin")
	}

	filename := positionalArgs[0]
	if filename == "-" {
		return processor(ctx, stdin, "stdin")
	}

	file, err := os.Open(filename)
	if err != nil {
		ErrorF(stderr, commandName, filename, err)
		return err
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			panic(err)
		}
	}(file)

	return processor(ctx, file, filename)
}

// ScanWithContext creates a scanner that checks for context cancellation on each scan
func ScanWithContext(ctx context.Context, scanner *bufio.Scanner) bool {
	// Check for cancellation before scanning
	if err := CheckContextCancellation(ctx); err != nil {
		return false
	}
	return scanner.Scan()
}

// CopyWithContext copies from src to dst with context cancellation support
// It uses a buffer to copy in chunks and checks for cancellation periodically
func CopyWithContext(ctx context.Context, dst io.Writer, src io.Reader) (int64, error) {
	return CopyBufferWithContext(ctx, dst, src, nil)
}

// CopyBufferWithContext copies from src to dst using the provided buffer with context cancellation support
// If buf is nil, one is allocated. It checks for cancellation before each read/write cycle
func CopyBufferWithContext(ctx context.Context, dst io.Writer, src io.Reader, buf []byte) (int64, error) {
	// Check for cancellation before starting
	if err := CheckContextCancellation(ctx); err != nil {
		return 0, err
	}

	if buf == nil {
		size := 32 * 1024
		if l, ok := src.(*io.LimitedReader); ok && int64(size) > l.N {
			if l.N < 1 {
				size = 1
			} else {
				size = int(l.N)
			}
		}
		buf = make([]byte, size)
	}

	var written int64
	for {
		// Check for cancellation before each read
		if err := CheckContextCancellation(ctx); err != nil {
			return written, err
		}

		nr, er := src.Read(buf)
		if nr > 0 {
			nw, ew := dst.Write(buf[0:nr])
			if nw < 0 || nr < nw {
				nw = 0
				if ew == nil {
					ew = fmt.Errorf("invalid write result")
				}
			}
			written += int64(nw)
			if ew != nil {
				return written, ew
			}
			if nr != nw {
				return written, io.ErrShortWrite
			}
		}
		if er != nil {
			if er != io.EOF {
				return written, er
			}
			break
		}
	}
	return written, nil
}

// ProcessLinesSimple is a convenience wrapper for simple line processing
func ProcessLinesSimple(ctx context.Context, reader io.Reader, output io.Writer, processor LineProcessorWithContext) error {
	return ProcessLinesWithContext(ctx, reader, output, processor)
}

// StandardCommand provides common command functionality
type StandardCommand[F any] struct {
	Positional []string
	Flags      F
	Name       string
}

// RequireArgs validates minimum argument count with standardized error
func (c StandardCommand[F]) RequireArgs(min int, stderr io.Writer) error {
	if len(c.Positional) < min {
		return c.Error(stderr, fmt.Sprintf("missing operand (need at least %d)", min))
	}
	return nil
}

// RequireArgsExact validates exact argument count
func (c StandardCommand[F]) RequireArgsExact(count int, stderr io.Writer) error {
	if len(c.Positional) != count {
		return c.Error(stderr, fmt.Sprintf("need exactly %d arguments, got %d", count, len(c.Positional)))
	}
	return nil
}

// Error formats standardized error messages
func (c StandardCommand[F]) Error(stderr io.Writer, message string) error {
	ErrorF(stderr, c.Name, "", fmt.Errorf(message))
	return fmt.Errorf(message)
}

// ProcessFiles executes file processing with standard options
func (c StandardCommand[F]) ProcessFiles(
	ctx context.Context,
	input io.Reader,
	output, stderr io.Writer,
	processor ProcessorFuncWithContext,
) error {
	return ProcessFilesWithContext(
		ctx, c.Positional, input, output, stderr,
		FileProcessorOptions{
			CommandName:     c.Name,
			ContinueOnError: true,
		},
		processor,
	)
}

// OutputFormatter handles common output formatting patterns
type OutputFormatter struct {
	ShowLineNumbers bool
	ShowFilenames   bool
	ShowCounts      bool
	Prefix          string
	Filename        string
	MultipleFiles   bool
}

// WriteLine writes a line with appropriate formatting
func (of OutputFormatter) WriteLine(output io.Writer, lineNum int, content string) {
	prefix := of.buildPrefix(lineNum)
	_, _ = fmt.Fprintf(output, "%s%s\n", prefix, content)
}

// WriteCount writes a count with appropriate formatting
func (of OutputFormatter) WriteCount(output io.Writer, count int) {
	prefix := of.buildFilePrefix()
	_, _ = fmt.Fprintf(output, "%s%d\n", prefix, count)
}

// buildPrefix creates the line prefix (filename:linenum:)
func (of OutputFormatter) buildPrefix(lineNum int) string {
	var parts []string

	if of.Prefix != "" {
		parts = append(parts, of.Prefix)
	}

	if of.ShowFilenames && of.Filename != "" && of.MultipleFiles {
		parts = append(parts, of.Filename)
	}

	if of.ShowLineNumbers {
		parts = append(parts, fmt.Sprintf("%d", lineNum))
	}

	if len(parts) > 0 {
		return strings.Join(parts, ":") + ":"
	}
	return ""
}

// buildFilePrefix creates the file prefix (filename:)
func (of OutputFormatter) buildFilePrefix() string {
	if of.ShowFilenames && of.Filename != "" && of.MultipleFiles {
		return of.Filename + ":"
	}
	return ""
}
