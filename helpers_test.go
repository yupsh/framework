package yup_test

import (
	"bufio"
	`bytes`
	"context"
	`errors`
	"io"
	`reflect`
	"strings"
	"testing"
	"time"

	yup "github.com/yupsh/framework"
)

// Test CheckContextCancellation
func TestCheckContextCancellation(t *testing.T) {
	t.Run("active context", func(t *testing.T) {
		ctx := context.Background()
		err := yup.CheckContextCancellation(ctx)
		if err != nil {
			t.Errorf("Expected no error for active context, got %v", err)
		}
	})

	t.Run("cancelled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := yup.CheckContextCancellation(ctx)
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Expected context.Canceled, got %v", err)
		}
	})

	t.Run("deadline exceeded", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer cancel()

		// Wait for timeout
		time.Sleep(1 * time.Millisecond)

		err := yup.CheckContextCancellation(ctx)
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("Expected context.DeadlineExceeded, got %v", err)
		}
	})
}

// Test ScanWithContext
func TestScanWithContext(t *testing.T) {
	t.Run("normal scanning", func(t *testing.T) {
		ctx := context.Background()
		input := "line1\nline2\nline3"
		scanner := bufio.NewScanner(strings.NewReader(input))

		var lines []string
		for yup.ScanWithContext(ctx, scanner) {
			lines = append(lines, scanner.Text())
		}

		expected := []string{"line1", "line2", "line3"}
		if len(lines) != len(expected) {
			t.Errorf("Expected %d lines, got %d", len(expected), len(lines))
		}

		for i, line := range lines {
			if line != expected[i] {
				t.Errorf("Line %d: expected %q, got %q", i, expected[i], line)
			}
		}
	})

	t.Run("cancelled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		input := "line1\nline2\nline3"
		scanner := bufio.NewScanner(strings.NewReader(input))

		// Should return false immediately due to cancelled context
		result := yup.ScanWithContext(ctx, scanner)
		if result {
			t.Error("Expected ScanWithContext to return false for cancelled context")
		}
	})

	t.Run("empty input", func(t *testing.T) {
		ctx := context.Background()
		scanner := bufio.NewScanner(strings.NewReader(""))

		result := yup.ScanWithContext(ctx, scanner)
		if result {
			t.Error("Expected ScanWithContext to return false for empty input")
		}
	})
}

// Test CopyWithContext
func TestCopyWithContext(t *testing.T) {
	t.Run("normal copy", func(t *testing.T) {
		ctx := context.Background()
		src := strings.NewReader("test data for copying")
		var dst strings.Builder

		n, err := yup.CopyWithContext(ctx, &dst, src)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		expected := "test data for copying"
		if dst.String() != expected {
			t.Errorf("Expected %q, got %q", expected, dst.String())
		}

		if n != int64(len(expected)) {
			t.Errorf("Expected %d bytes copied, got %d", len(expected), n)
		}
	})

	t.Run("cancelled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())

		// Create a large input that would take time to copy
		largeData := strings.Repeat("test data ", 100000)
		src := strings.NewReader(largeData)
		var dst strings.Builder

		// Start copy in goroutine
		done := make(chan struct{})
		var copyErr error
		var bytesCopied int64

		go func() {
			defer close(done)
			bytesCopied, copyErr = yup.CopyWithContext(ctx, &dst, src)
		}()

		// Cancel after short delay
		time.Sleep(10 * time.Millisecond)
		cancel()

		// Wait for completion
		select {
		case <-done:
			if !errors.Is(copyErr, context.Canceled) {
				t.Errorf("Expected context.Canceled, got %v", copyErr)
			}
			// Should have copied some data before cancellation
			if bytesCopied == 0 {
				t.Error("Expected some bytes to be copied before cancellation")
			}
		case <-time.After(time.Second):
			t.Error("Copy operation did not respond to cancellation")
		}
	})

	t.Run("empty source", func(t *testing.T) {
		ctx := context.Background()
		src := strings.NewReader("")
		var dst strings.Builder

		n, err := yup.CopyWithContext(ctx, &dst, src)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		if n != 0 {
			t.Errorf("Expected 0 bytes copied, got %d", n)
		}

		if dst.String() != "" {
			t.Errorf("Expected empty destination, got %q", dst.String())
		}
	})
}

// Test CopyBufferWithContext
func TestCopyBufferWithContext(t *testing.T) {
	t.Run("custom buffer", func(t *testing.T) {
		ctx := context.Background()
		src := strings.NewReader("test data with custom buffer")
		var dst strings.Builder
		buffer := make([]byte, 8) // Small buffer to test chunking

		n, err := yup.CopyBufferWithContext(ctx, &dst, src, buffer)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		expected := "test data with custom buffer"
		if dst.String() != expected {
			t.Errorf("Expected %q, got %q", expected, dst.String())
		}

		if n != int64(len(expected)) {
			t.Errorf("Expected %d bytes copied, got %d", len(expected), n)
		}
	})

	t.Run("nil buffer auto-allocation", func(t *testing.T) {
		ctx := context.Background()
		src := strings.NewReader("test data")
		var dst strings.Builder

		n, err := yup.CopyBufferWithContext(ctx, &dst, src, nil)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		expected := "test data"
		if dst.String() != expected {
			t.Errorf("Expected %q, got %q", expected, dst.String())
		}

		if n != int64(len(expected)) {
			t.Errorf("Expected %d bytes copied, got %d", len(expected), n)
		}
	})
}

// Test ProcessFilesWithContext
func TestProcessFilesWithContext(t *testing.T) {
	processor := func(ctx context.Context, source yup.InputSource, output io.Writer) error {
		// Check context before processing
		if err := yup.CheckContextCancellation(ctx); err != nil {
			return err
		}

		// Simple processor that copies input to output
		_, err := io.Copy(output, source.Reader)
		return err
	}

	t.Run("stdin processing", func(t *testing.T) {
		ctx := context.Background()
		stdin := strings.NewReader("test input from stdin")
		var output, stderr strings.Builder

		options := yup.FileProcessorOptions{
			CommandName: "test",
		}

		err := yup.ProcessFilesWithContext(ctx, []string{}, stdin, &output, &stderr, options, processor)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		if output.String() != "test input from stdin" {
			t.Errorf("Expected stdin content, got %q", output.String())
		}
	})

	t.Run("cancelled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		stdin := strings.NewReader("test input")
		var output, stderr strings.Builder

		options := yup.FileProcessorOptions{
			CommandName: "test",
		}

		err := yup.ProcessFilesWithContext(ctx, []string{}, stdin, &output, &stderr, options, processor)
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Expected context.Canceled, got %v", err)
		}
	})
}

// BenchmarkCheckContextCancellation tests for performance
func BenchmarkCheckContextCancellation(b *testing.B) {
	ctx := context.Background()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		err := yup.CheckContextCancellation(ctx)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkScanWithContext(b *testing.B) {
	ctx := context.Background()
	testData := strings.Repeat("line of test data\n", 1000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		scanner := bufio.NewScanner(strings.NewReader(testData))
		for yup.ScanWithContext(ctx, scanner) {
			// Just scan, don't process
		}
	}
}

func BenchmarkCopyWithContext(b *testing.B) {
	ctx := context.Background()
	testData := strings.Repeat("benchmark test data ", 1000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		src := strings.NewReader(testData)
		var dst strings.Builder
		_, err := yup.CopyWithContext(ctx, &dst, src)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func TestProcessFiles(t *testing.T) {
	type args struct {
		positionalArgs []string
		stdin          io.Reader
		options        yup.FileProcessorOptions
		processor      yup.ProcessorFunc
	}
	tests := []struct {
		name       string
		args       args
		wantOutput string
		wantStderr string
		wantErr    bool
	}{
		{},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := &bytes.Buffer{}
			stderr := &bytes.Buffer{}
			err := yup.ProcessFiles(tt.args.positionalArgs, tt.args.stdin, output, stderr, tt.args.options, tt.args.processor)
			if (err != nil) != tt.wantErr {
				t.Errorf("ProcessFiles() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotOutput := output.String(); gotOutput != tt.wantOutput {
				t.Errorf("ProcessFiles() gotOutput = %v, want %v", gotOutput, tt.wantOutput)
			}
			if gotStderr := stderr.String(); gotStderr != tt.wantStderr {
				t.Errorf("ProcessFiles() gotStderr = %v, want %v", gotStderr, tt.wantStderr)
			}
		})
	}
}

func TestProcessLines(t *testing.T) {
	type args struct {
		reader    io.Reader
		processor yup.LineProcessor
	}
	tests := []struct {
		name       string
		args       args
		wantOutput string
		wantErr    bool
	}{
		{},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := &bytes.Buffer{}
			err := yup.ProcessLines(tt.args.reader, output, tt.args.processor)
			if (err != nil) != tt.wantErr {
				t.Errorf("ProcessLines() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotOutput := output.String(); gotOutput != tt.wantOutput {
				t.Errorf("ProcessLines() gotOutput = %v, want %v", gotOutput, tt.wantOutput)
			}
		})
	}
}

func TestReadAllLines(t *testing.T) {
	type args struct {
		reader io.Reader
	}
	tests := []struct {
		name    string
		args    args
		want    []string
		wantErr bool
	}{
		{},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := yup.ReadAllLines(tt.args.reader)
			if (err != nil) != tt.wantErr {
				t.Errorf("ReadAllLines() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ReadAllLines() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCollectInputSources(t *testing.T) {
	type args struct {
		positionalArgs []string
		stdin          io.Reader
	}
	tests := []struct {
		name    string
		args    args
		want    []yup.InputSource
		wantErr bool
	}{
		{},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := yup.CollectInputSources(tt.args.positionalArgs, tt.args.stdin)
			if (err != nil) != tt.wantErr {
				t.Errorf("CollectInputSources() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CollectInputSources() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProcessSingleFile(t *testing.T) {
	type args struct {
		positionalArgs []string
		stdin          io.Reader
		commandName    string
		processor      func(io.Reader, string) error
	}
	tests := []struct {
		name       string
		args       args
		wantStderr string
		wantErr    bool
	}{
		{},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stderr := &bytes.Buffer{}
			err := yup.ProcessSingleFile(tt.args.positionalArgs, tt.args.stdin, tt.args.commandName, stderr, tt.args.processor)
			if (err != nil) != tt.wantErr {
				t.Errorf("ProcessSingleFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotStderr := stderr.String(); gotStderr != tt.wantStderr {
				t.Errorf("ProcessSingleFile() gotStderr = %v, want %v", gotStderr, tt.wantStderr)
			}
		})
	}
}

func TestCloseInputSources(t *testing.T) {
	type args struct {
		sources []yup.InputSource
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := yup.CloseInputSources(tt.args.sources); (err != nil) != tt.wantErr {
				t.Errorf("CloseInputSources() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRequireArguments(t *testing.T) {
	type args struct {
		args        []string
		min         int
		max         int
		commandName string
	}
	tests := []struct {
		name       string
		args       args
		wantStderr string
		wantErr    bool
	}{
		{},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stderr := &bytes.Buffer{}
			err := yup.RequireArguments(tt.args.args, tt.args.min, tt.args.max, tt.args.commandName, stderr)
			if (err != nil) != tt.wantErr {
				t.Errorf("RequireArguments() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotStderr := stderr.String(); gotStderr != tt.wantStderr {
				t.Errorf("RequireArguments() gotStderr = %v, want %v", gotStderr, tt.wantStderr)
			}
		})
	}
}

func TestProcessSingleFileWithContext(t *testing.T) {
	type args struct {
		ctx            context.Context
		positionalArgs []string
		stdin          io.Reader
		commandName    string
		processor      func(ctx context.Context, reader io.Reader, filename string) error
	}
	tests := []struct {
		name       string
		args       args
		wantStderr string
		wantErr    bool
	}{
		{},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stderr := &bytes.Buffer{}
			err := yup.ProcessSingleFileWithContext(tt.args.ctx, tt.args.positionalArgs, tt.args.stdin, tt.args.commandName, stderr, tt.args.processor)
			if (err != nil) != tt.wantErr {
				t.Errorf("ProcessSingleFileWithContext() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotStderr := stderr.String(); gotStderr != tt.wantStderr {
				t.Errorf("ProcessSingleFileWithContext() gotStderr = %v, want %v", gotStderr, tt.wantStderr)
			}
		})
	}
}

func TestProcessLinesSimple(t *testing.T) {
	type args struct {
		ctx       context.Context
		reader    io.Reader
		processor yup.LineProcessorWithContext
	}
	tests := []struct {
		name       string
		args       args
		wantOutput string
		wantErr    bool
	}{
		{},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := &bytes.Buffer{}
			err := yup.ProcessLinesSimple(tt.args.ctx, tt.args.reader, output, tt.args.processor)
			if (err != nil) != tt.wantErr {
				t.Errorf("ProcessLinesSimple() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotOutput := output.String(); gotOutput != tt.wantOutput {
				t.Errorf("ProcessLinesSimple() gotOutput = %v, want %v", gotOutput, tt.wantOutput)
			}
		})
	}
}
