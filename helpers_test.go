package yup_test

import (
	"bufio"
	"context"
	"io"
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
		if err != context.Canceled {
			t.Errorf("Expected context.Canceled, got %v", err)
		}
	})

	t.Run("deadline exceeded", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer cancel()

		// Wait for timeout
		time.Sleep(1 * time.Millisecond)

		err := yup.CheckContextCancellation(ctx)
		if err != context.DeadlineExceeded {
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

		lines := []string{}
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
			if copyErr != context.Canceled {
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
		if err != context.Canceled {
			t.Errorf("Expected context.Canceled, got %v", err)
		}
	})
}

// Benchmark tests for performance
func BenchmarkCheckContextCancellation(b *testing.B) {
	ctx := context.Background()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		yup.CheckContextCancellation(ctx)
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
		yup.CopyWithContext(ctx, &dst, src)
	}
}
