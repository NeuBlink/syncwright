# JSON Schema Validation

This package provides comprehensive JSON payload validation for the Syncwright CLI with a focus on security and reliability.

## Overview

The validation module protects against:
- **DoS attacks** through payload size limits
- **Path traversal attacks** via file path validation
- **Command injection** through content sanitization
- **Malformed data** causing processing failures
- **Resource exhaustion** through conflict size limits

## Key Features

### Security Validation
- **Payload Size Limits**: Maximum 10MB to prevent DoS attacks
- **Path Traversal Protection**: Blocks `../`, absolute paths to system directories
- **Content Sanitization**: Removes null bytes, dangerous control characters
- **Language Allowlisting**: Only permits supported programming languages
- **File Count Limits**: Maximum 1000 files, 100 conflicts per file

### Data Integrity
- **Struct-Level Validation**: Type safety with custom validation tags
- **Business Logic Checks**: Line range validation, duplicate detection
- **Context Size Limits**: Prevents excessive context data
- **JSON Schema Compliance**: Strict parsing with unknown field rejection

### Performance Optimization
- **Streaming Validation**: Memory-efficient processing of large payloads
- **Early Termination**: Fails fast on first validation error
- **Cached Validators**: Reusable validator instances

## Usage

```go
import "github.com/NeuBlink/syncwright/internal/validation"

// Create validator
validator := validation.NewPayloadValidator()

// Validate and sanitize payload
payload, result, err := validator.ValidateAndSanitize(jsonData)
if err != nil {
    log.Printf("Validation failed: %v", err)
    for _, validationError := range result.Errors {
        log.Printf("  - %s: %s", validationError.Field, validationError.Message)
    }
    return
}

// Use validated payload safely
fmt.Printf("Validated %d files with %d conflicts\n", 
    result.Summary.TotalFiles, result.Summary.TotalConflicts)
```

## Configuration Constants

```go
const (
    MaxPayloadSize      = 10 * 1024 * 1024 // 10MB
    MaxConflictFiles    = 1000             // Maximum files per payload
    MaxConflictsPerFile = 100              // Maximum conflicts per file
    MaxLineLength       = 10000            // Maximum characters per line
    MaxContextLines     = 50               // Maximum context lines
    MaxTotalConflicts   = 5000             // Maximum total conflicts
)
```

## Supported Languages

The validator accepts these programming languages:
- **Systems**: `go`, `c`, `cpp`, `rust`, `csharp`
- **Web**: `javascript`, `typescript`, `html`, `css`, `scss`
- **Scripting**: `python`, `ruby`, `php`, `shell`, `bash`
- **JVM**: `java`, `kotlin`, `scala`
- **Mobile**: `swift`, `kotlin`
- **Data**: `json`, `yaml`, `xml`, `sql`
- **Documentation**: `markdown`, `text`
- **DevOps**: `dockerfile`, `makefile`

## Validation Errors

The validator provides detailed error information:

```go
type ValidationError struct {
    Field       string `json:"field"`        // Field that failed validation
    Value       string `json:"value"`        // Validation rule that failed
    Tag         string `json:"tag"`          // Validation tag
    Message     string `json:"message"`      // Human-readable error message
    ActualValue string `json:"actual_value"` // The actual invalid value
}
```

## Integration

The validation is automatically integrated into:
- **ai-apply command**: Validates all incoming payloads before AI processing
- **Error reporting**: Provides detailed validation failure messages
- **Security logging**: Records validation failures for security monitoring

## Security Model

1. **Input Validation**: All external JSON payloads are validated before processing
2. **Content Sanitization**: Dangerous content is removed or escaped
3. **Resource Limits**: Prevents resource exhaustion attacks
4. **Allowlisting**: Only permits known-safe programming languages and file paths
5. **Fail-Safe Defaults**: Validation failures prevent processing, never allow through

## Testing

Comprehensive test coverage includes:
- **Security tests**: Path traversal, injection attempts, oversized payloads
- **Validation tests**: Invalid data types, missing required fields
- **Business logic tests**: Duplicate detection, line range validation
- **Performance tests**: Large payload handling, memory usage
- **Integration tests**: End-to-end CLI validation flow

Run tests with:
```bash
go test ./internal/validation/... -v
```

## Performance Characteristics

- **Validation time**: ~1ms for typical payloads (< 1MB)
- **Memory usage**: ~2x payload size during validation
- **CPU overhead**: Minimal impact on overall processing time
- **Scalability**: Linear performance with payload size

## Security Compliance

This validation implementation addresses:
- **OWASP Top 10**: Input validation, injection prevention
- **CWE-20**: Improper input validation
- **CWE-22**: Path traversal vulnerabilities
- **CWE-78**: Command injection prevention
- **CWE-400**: Resource exhaustion prevention