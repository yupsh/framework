# yupsh Framework Documentation

> **Developer Guide**: Understanding, building, and extending yupsh commands

yupsh is a modern framework for building Unix-style command-line tools in Go. This guide focuses on understanding the framework architecture, implementing new commands, and leveraging the powerful abstractions provided.

## 🎯 **For Developers**

This documentation is for developers who want to:
- **Understand** how yupsh commands work
- **Create** new commands using the framework
- **Contribute** to the yupsh ecosystem
- **Extend** existing functionality

> For end-user documentation, see the [main project README](../yupsh/README.md).

## 🏗️ **Framework Philosophy**

### **Core Principles**
1. **Composable Architecture**: Each command is an independent Go module
2. **Type Safety**: Strongly-typed flags prevent runtime errors
3. **Context Awareness**: All operations support cancellation and timeouts
4. **Memory Efficiency**: Streaming processing for files of any size
5. **Developer Experience**: Rich abstractions that eliminate boilerplate

## 🏗️ **Understanding the Framework**

### **Core Architecture**

```
yupsh/
├── framework/              # 🏗️ Core framework (this package)
│   ├── helpers.go          # Common utilities and abstractions
│   ├── pipeline.go         # Command composition and execution
│   └── opt/                # Type-safe flag system
├── cat/                   # 📝 Example command: file concatenation
├── grep/                  # 🔍 Example command: pattern matching
└── [other-commands]/      # 🛠️ Additional Unix commands
```

### **The Command Interface**

Every yupsh command implements this simple interface:

```go
type Command interface {
    Execute(ctx context.Context, input io.Reader, output, stderr io.Writer) error
}
```

**Key Design Decisions:**
- **`context.Context`**: Enables cancellation, timeouts, and graceful shutdown
- **`io.Reader/Writer`**: Stream-based I/O prevents memory issues with large files
- **Standard Error Separation**: Follows Unix conventions for output vs. error streams

### **Type-Safe Configuration**

Instead of string-based flags, yupsh uses Go's type system:

```go
// Traditional approach (error-prone)
command := "grep -i -n pattern file.txt"

// yupsh approach (type-safe)
command := grep.Grep("pattern", "file.txt",
    grep.IgnoreCase,    // Compile-time type checking
    grep.LineNumber)    // IDE autocompletion
```

## 🚀 **Creating a New Command**

### **Step 1: Project Structure**

Create the standard yupsh command structure:

```bash
mkdir mycommand
cd mycommand
go mod init github.com/yupsh/mycommand
```

```
mycommand/
├── go.mod               # Independent Go module
├── mycommand.go         # Main implementation
├── mycommand_test.go    # Example tests
└── opt/
    └── opt.go           # Strongly-typed flags
```

### **Step 2: Define Flags (opt/opt.go)**

```go
package opt

// Boolean flag types with constants
type VerboseFlag bool
const (
    Verbose   VerboseFlag = true
    NoVerbose VerboseFlag = false
)

// Custom parameter types
type Count int
type Format string

// Main flags structure
type Flags struct {
    Verbose VerboseFlag
    Count   Count
    Format  Format
}

// Configure methods for the opt system
func (f VerboseFlag) Configure(flags *Flags) { flags.Verbose = f }
func (c Count) Configure(flags *Flags) { flags.Count = c }
func (f Format) Configure(flags *Flags) { flags.Format = f }
```

### **Step 3: Implement the Command**

#### **Using StandardCommand (Recommended)**

```go
package mycommand

import (
	"context"
	"fmt"
	"io"

	yup "github.com/yupsh/framework"
	"github.com/yupsh/framework/opt"
	localopt "github.com/yupsh/mycommand/opt"
)

// Command implementation using StandardCommand abstraction
type command struct {
	yup.StandardCommand[localopt.Flags]
}

// Constructor function
func MyCommand(parameters ...any) yup.Command {
	args := opt.Args[string, localopt.Flags](parameters...)
	return command{
		StandardCommand: yup.StandardCommand[localopt.Flags]{
			Positional: args.Positional,
			Flags:      args.Flags,
			Name:       "mycommand",
		},
	}
}

// Main execution logic
func (c command) Execute(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer) error {
	// Validate arguments using framework helper
	if err := c.RequireArgs(1, stderr); err != nil {
		return err
	}

	// Use framework abstraction for file processing
	return c.ProcessFiles(ctx, stdin, stdout, stderr,
		func(ctx context.Context, source yup.InputSource, output io.Writer) error {
			return c.processFile(ctx, source, output)
		},
	)
}

// Process individual files/streams
func (c command) processFile(ctx context.Context, source yup.InputSource, output io.Writer) error {
	// For line-by-line processing, use ProcessLinesSimple
	return yup.ProcessLinesSimple(ctx, source.Reader, output,
		func(ctx context.Context, lineNum int, line string, output io.Writer) error {
			// Your line processing logic here
			if bool(c.Flags.Verbose) {
				fmt.Fprintf(output, "Line %d: %s\n", lineNum, line)
			} else {
				fmt.Fprintln(output, line)
			}
			return nil
		},
	)
}
```

### **Step 4: Add Dependencies (go.mod)**

```go
module github.com/yupsh/mycommand

go 1.21

require github.com/yupsh/framework v0.1.0

// For local development
replace github.com/yupsh/framework => ../framework
```

### **Step 5: Write Tests (mycommand_test.go)**

```go
package mycommand_test

import (
    "context"
    "os"
    "strings"

    "github.com/yupsh/mycommand"
    "github.com/yupsh/mycommand/opt"
)

// Example test (serves as documentation)
func ExampleMyCommand() {
    ctx := context.Background()
    input := strings.NewReader("line 1\nline 2\nline 3")

    cmd := mycommand.MyCommand("input", opt.Verbose)
    cmd.Execute(ctx, input, os.Stdout, os.Stderr)
    // Output:
    // Line 1: line 1
    // Line 2: line 2
    // Line 3: line 3
}

func ExampleMyCommand_withCount() {
    ctx := context.Background()
    input := strings.NewReader("test data")

    cmd := mycommand.MyCommand(opt.Count(5), opt.Format("json"))
    cmd.Execute(ctx, input, os.Stdout, os.Stderr)
    // Output: expected output
}
```

## 🧠 **Framework Abstractions**

### **StandardCommand[F] - Base Command Type**

Provides common functionality for most commands:

```go
type command struct {
    yup.StandardCommand[Flags]  // Embed for automatic functionality
}

// Automatic methods available:
// - c.RequireArgs(min int, stderr io.Writer) error
// - c.RequireArgsExact(count int, stderr io.Writer) error
// - c.Error(stderr io.Writer, message string) error
// - c.ProcessFiles(ctx, input, output, stderr, processor) error
```

**Benefits:**
- Standardized error messages
- Argument validation
- Consistent file processing
- Reduced boilerplate

### **ProcessLinesSimple - Line Processing**

For commands that process input line-by-line:

```go
func (c command) processReader(ctx context.Context, reader io.Reader, output io.Writer) error {
    return yup.ProcessLinesSimple(ctx, reader, output,
        func(ctx context.Context, lineNum int, line string, output io.Writer) error {
            // Process each line with automatic context checking
            processedLine := transform(line)
            fmt.Fprintln(output, processedLine)
            return nil
        },
    )
}
```

**Features:**
- Automatic context cancellation checking
- No manual scanner management
- Focus on business logic
- Memory efficient streaming

### **OutputFormatter - Consistent Output**

```go
formatter := yup.OutputFormatter{
    ShowLineNumbers: bool(c.Flags.LineNumber),
    ShowFilenames:   true,
    Filename:        source.Filename,
    MultipleFiles:   len(c.Positional) > 1,
}

// Automatic prefix formatting (filename:linenum:)
formatter.WriteLine(output, lineNum, content)
formatter.WriteCount(output, matchCount)
```

## 🎨 **Common Patterns and Best Practices**

### **Pattern 1: Simple Line Processing**

```go
// Commands like: nl, rev, tr
func (c command) Execute(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer) error {
    return c.ProcessFiles(ctx, stdin, stdout, stderr,
        func(ctx context.Context, source yup.InputSource, stdout io.Writer) error {
            return yup.ProcessLinesSimple(ctx, source.Reader, stdout,
                func(ctx context.Context, lineNum int, line string, stdout io.Writer) error {
                    result := c.transformLine(line)  // Your logic here
                    fmt.Fprintln(stdout, result)
                    return nil
                },
            )
        },
    )
}
```

### **Pattern 2: Complex File Processing**

```go
// Commands like: sort, comm, join
func (c command) Execute(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer) error {
    if err := c.RequireArgs(1, stderr); err != nil {
        return err
    }

    // Custom file processing logic
    return yup.ProcessFilesWithContext(
        ctx, c.Positional, stdin, stdout, stderr,
        yup.FileProcessorOptions{
            CommandName:     c.Name,
            ContinueOnError: true,
        },
        func(ctx context.Context, source yup.InputSource, stdout io.Writer) error {
            return c.complexProcessing(ctx, source, stdout)
        },
    )
}
```

### **Pattern 3: Output Formatting**

```go
// Commands like: grep, find
formatter := yup.OutputFormatter{
    ShowLineNumbers: bool(c.Flags.LineNumber),
    ShowFilenames:   len(c.Positional) > 1,
    Filename:        source.Filename,
    MultipleFiles:   len(c.Positional) > 1,
}

// In your line processor
if matches {
    formatter.WriteLine(output, lineNum, line)
}

// For counts
if c.Flags.Count {
    formatter.WriteCount(output, matchCount)
}
```

### **Pattern 4: Context Cancellation**

```go
// Automatic in ProcessLinesSimple, manual for custom loops
for i, item := range largeDataSet {
    // Check every 1000 iterations for efficiency
    if i%1000 == 0 {
        if err := yup.CheckContextCancellation(ctx); err != nil {
            return err
        }
    }
    processItem(item)
}
```

## 🎩 **Advanced Topics**

### **Type-Safe Flag System**

#### **Boolean Flags with Constants**
```go
type VerboseFlag bool
const (
    Verbose   VerboseFlag = true
    NoVerbose VerboseFlag = false
)

// Usage
command := mycommand.MyCommand("input.txt", opt.Verbose)
```

#### **Parameterized Flags**
```go
type Count int
type Format string

// Usage with type safety
command := mycommand.MyCommand(
    opt.Count(100),        // Compile-time type checking
    opt.Format("json"),    // No magic strings
)
```

### **Error Handling Best Practices**

```go
func (c command) Execute(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer) error {
    // Use framework helpers for consistent error messages
    if err := c.RequireArgs(1, stderr); err != nil {
        return err  // Standardized format
    }

    // Custom validation with framework error formatting
    if c.Flags.Count < 0 {
        return c.Error(stderr, "count must be non-negative")
    }

    // Handle context cancellation
    if err := yup.CheckContextCancellation(ctx); err != nil {
        return err
    }

    return nil
}
```

### **Memory Management**

```go
// ✅ Good: Streaming processing
return yup.ProcessLinesSimple(ctx, reader, output, processor)

// ❌ Avoid: Loading entire files
data, _ := io.ReadAll(reader)  // Can cause OOM on large files

// ✅ Good: Chunked processing for binary data
buf := make([]byte, 32*1024)  // 32KB chunks
for {
    n, err := reader.Read(buf)
    if err == io.EOF {
        break
    }
    // Process chunk...
}
```

### **Testing Strategies**

#### **Example Tests (Recommended)**
```go
func ExampleMyCommand() {
    ctx := context.Background()
    stdin := strings.NewReader("test input")

    cmd := mycommand.MyCommand("arg", opt.Verbose)
    cmd.Execute(ctx, stdin, os.Stdout, os.Stderr)
    // Output: expected output
}
```

#### **Unit Tests with Context Cancellation**
```go
func TestMyCommand_ContextCancellation(t *testing.T) {
    ctx, cancel := context.WithCancel(context.Background())
    input := strings.NewReader(strings.Repeat("line\n", 1000000))

    cmd := mycommand.MyCommand("input")

    // Cancel after a short time
    go func() {
        time.Sleep(10 * time.Millisecond)
        cancel()
    }()

    err := cmd.Execute(ctx, input, io.Discard, io.Discard)
    if err != context.Canceled {
        t.Errorf("Expected context.Canceled, got %v", err)
    }
}
```

#### **Benchmark Tests**
```go
func BenchmarkMyCommand(b *testing.B) {
    ctx := context.Background()
    input := strings.NewReader("benchmark data")
    cmd := mycommand.MyCommand("arg")

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        cmd.Execute(ctx, input, io.Discard, io.Discard)
        input.Seek(0, 0)  // Reset for next iteration
    }
}
```

## 🛠️ **Framework Utilities**

### **Available Helper Functions**

```go
// Context and cancellation
yup.CheckContextCancellation(ctx) error
yup.ScanWithContext(ctx, scanner) bool

// File and stream processing
yup.ProcessFilesWithContext(ctx, files, input, output, stderr, options, processor)
yup.ProcessLinesSimple(ctx, reader, output, lineProcessor)

// I/O utilities
yup.CopyWithContext(ctx, dst, src) (int64, error)
yup.CopyBufferWithContext(ctx, dst, src, buf) (int64, error)

// Error formatting
yup.ErrorF(stderr, commandName, filename, err)
```

### **File Processing Options**

```go
type FileProcessorOptions struct {
    CommandName     string  // For error messages
    ShowHeaders     bool    // Show "==> filename <==" headers
    BlankBetween    bool    // Blank lines between files
    ContinueOnError bool    // Keep processing on file errors
}
```

## 🎓 **Learning from Examples**

### **Study Existing Commands**

1. **Simple**: `echo`, `yes` - Basic parameter handling
2. **Line Processing**: `cat`, `nl`, `rev` - Using ProcessLinesSimple
3. **Pattern Matching**: `grep`, `find` - Complex logic with formatting
4. **Data Processing**: `sort`, `uniq`, `wc` - Accumulation and transformation
5. **Advanced**: `xargs`, `split` - Parallel processing and chunking

### **Command Complexity Levels**

#### **Level 1: Basic Commands**
```go
// Simple output, minimal processing
// Examples: echo, yes
type command struct { yup.StandardCommand[Flags] }
```

#### **Level 2: Line Processors**
```go
// Process input line by line
// Examples: cat, nl, rev, tr
return yup.ProcessLinesSimple(ctx, reader, output, processor)
```

#### **Level 3: Complex Processing**
```go
// Custom algorithms, multiple phases
// Examples: sort, grep, find
return yup.ProcessFilesWithContext(ctx, files, input, output, stderr, options, processor)
```

## 📚 **Development Workflow**

### **Getting Started**

```bash
# 1. Set up development environment
git clone https://github.com/yupsh/yupsh.git
cd yupsh

# 2. Create your command
mkdir mycommand
cd mycommand
go mod init github.com/yupsh/mycommand

# 3. Copy a similar command as template
cp ../echo/echo.go mycommand.go
cp -r ../echo/opt .

# 4. Implement and test
go test -v
go run mycommand.go
```

### **Testing Your Command**

```bash
# Run example tests
go test -v -run Example

# Run all tests including benchmarks
go test -v -bench=.

# Test with race detection
go test -race

# Test context cancellation
go test -run TestCancel
```

## 🎆 **Contributing Guidelines**

### **Code Standards**
- Use `gofmt` and `go vet`
- Follow established naming conventions
- Include comprehensive example tests
- Support context cancellation
- Use framework abstractions

### **Performance Requirements**
- Support files of unlimited size (streaming)
- Respect context cancellation (< 100ms response)
- Memory usage should be O(1) for line processing
- Include benchmarks for performance-critical code

### **Documentation Standards**
- Example tests serve as documentation
- Comment complex algorithms
- Document flag behavior clearly
- Include usage examples in README

## 🔗 **Resources**

- **[Main Project](../yupsh/README.md)**: User-focused documentation
- **[Contributing Guide](../.github/CONTRIBUTING.md)**: Detailed contribution instructions
- **[Example Commands](../cat/README.md)**: Real command implementations
- **[Command Examples](../grep/README.md)**: Complex pattern matching examples

---

**Happy coding!** 🚀 The yupsh framework makes it easy to build reliable, performant Unix commands in Go.
