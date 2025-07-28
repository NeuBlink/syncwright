# Enhanced Go Conflict Resolution

This module provides advanced AI-powered conflict resolution specifically optimized for Go codebases. It leverages Claude AI's understanding of Go semantics, idioms, and best practices to provide high-quality, confident resolutions for complex merge conflicts.

## Key Features

### 🎯 Go-Specific Prompt Engineering
- **Function Signature Analysis**: Deep understanding of Go function signatures, method receivers, and parameter types
- **Import Management**: Intelligent handling of import statements, aliases, and package dependencies
- **Type System Awareness**: Advanced understanding of Go's type system, interfaces, and compatibility
- **Idiom Recognition**: Recognition and preservation of Go coding patterns and best practices
- **Error Handling Patterns**: Proper analysis of Go error handling conventions

### 🔄 Multi-Turn Conversation Refinement
- **Confidence-Based Refinement**: Automatically triggers additional AI analysis for low-confidence resolutions
- **Iterative Improvement**: Up to 3 refinement rounds to improve resolution quality
- **Context Preservation**: Maintains conversation context across turns for better understanding
- **Session Management**: Proper Claude CLI session handling for multi-turn interactions

### 📊 Enhanced Confidence Scoring
- **Go-Specific Validation**: Confidence adjustment based on Go syntax and semantic validation
- **Multiple Validation Layers**: Syntax checking, function signature validation, import consistency
- **Idiom Scoring**: Bonus points for following Go best practices and conventions
- **Semantic Analysis**: Deep code analysis for type safety and logical correctness

### 🔍 Comprehensive Context Extraction
- **Package Information**: Extraction of package declarations and context
- **Import Analysis**: Complete import statement analysis for dependency understanding
- **Function Signature Extraction**: Identification of function signatures involved in conflicts
- **Type Definition Analysis**: Recognition of struct and interface definitions

### ✅ Semantic Validation
- **Syntax Correctness**: Comprehensive Go syntax validation
- **Type Consistency**: Basic type compatibility checking
- **Error Handling Validation**: Verification of proper Go error handling patterns
- **Control Structure Analysis**: Validation of if/for/switch statement completeness

## Usage

### Basic Usage

```go
import (
    "context"
    "github.com/NeuBlink/syncwright/internal/claude"
    "github.com/NeuBlink/syncwright/internal/payload"
    "github.com/NeuBlink/syncwright/internal/gitutils"
)

// Configure enhanced resolver for Go projects
config := &claude.ConflictResolverConfig{
    ClaudeConfig: &claude.Config{
        CLIPath:          "claude",
        MaxTurns:         7,           // More turns for complex Go analysis
        TimeoutSeconds:   180,         // Extended timeout
        AllowedTools:     []string{"Read", "Write", "Bash(git*)", "Grep", "Glob"},
        OutputFormat:     "json",
        PrintMode:        true,
        Verbose:          true,
    },
    RepoPath:            "/path/to/go/repo",
    MinConfidence:       0.7,          // Higher threshold for Go code
    MaxBatchSize:        5,            // Focused analysis
    IncludeReasoning:    true,         // Essential for Go semantics
    Verbose:             true,
    EnableMultiTurn:     true,         // Enable refinement
    MaxTurns:            3,            // Up to 3 refinement rounds
    MultiTurnThreshold:  0.6,          // Refine below 60% confidence
}

// Create resolver
resolver, err := claude.NewConflictResolver(config)
if err != nil {
    log.Fatal(err)
}
defer resolver.Close()

// Get conflicts and resolve
report, err := gitutils.GetConflictReport("/path/to/go/repo")
if err != nil {
    log.Fatal(err)
}

conflictPayload, err := payload.BuildSimplePayload(report)
if err != nil {
    log.Fatal(err)
}

ctx := context.Background()
result, err := resolver.ResolveConflicts(ctx, conflictPayload)
if err != nil {
    log.Fatal(err)
}

// Process results
fmt.Printf("High confidence resolutions: %d\n", len(result.HighConfidence))
fmt.Printf("Low confidence resolutions: %d\n", len(result.LowConfidence))
fmt.Printf("Overall confidence: %.2f\n", result.OverallConfidence)
```

### Advanced Configuration

```go
// For production Go codebases with strict quality requirements
config := &claude.ConflictResolverConfig{
    ClaudeConfig: &claude.Config{
        CLIPath:          "claude",
        MaxTurns:         10,          // Maximum analysis depth
        TimeoutSeconds:   300,         // 5-minute timeout for complex conflicts
        AllowedTools:     []string{"Read", "Write", "Bash(git*)", "Grep", "Glob"},
        OutputFormat:     "json",
        PrintMode:        true,
        Verbose:          true,
    },
    RepoPath:            "/path/to/production/repo",
    MinConfidence:       0.85,         // Very high confidence required
    MaxBatchSize:        3,            // Deep analysis per file
    IncludeReasoning:    true,
    Verbose:             true,
    EnableMultiTurn:     true,
    MaxTurns:            5,            // Extensive refinement
    MultiTurnThreshold:  0.75,         // Refine anything below 75%
}
```

## Confidence Scoring System

The enhanced confidence scoring system provides accurate assessment of resolution quality:

### Score Ranges
- **0.9-1.0**: Confident resolution, clear semantic intent, perfect Go idioms
- **0.7-0.9**: Good resolution, minor ambiguity, mostly idiomatic
- **0.5-0.7**: Reasonable resolution, some uncertainty, basic correctness
- **0.3-0.5**: Uncertain resolution, significant ambiguity, may need review
- **0.0-0.3**: Low confidence, complex conflict, recommend manual review

### Validation Factors
- **Syntax Validity**: Go syntax correctness and compilation readiness
- **Function Signatures**: Consistency and compatibility of function definitions
- **Import Statements**: Proper import management and dependency handling
- **Go Idioms**: Adherence to Go best practices and conventions
- **Type Safety**: Basic type compatibility and interface satisfaction

## Multi-Turn Refinement Process

When a resolution has confidence below the threshold:

1. **Initial Analysis**: Standard conflict resolution with Go-specific prompting
2. **Confidence Assessment**: Evaluation using Go-specific validation rules
3. **Refinement Trigger**: If confidence < threshold, initiate multi-turn conversation
4. **Iterative Improvement**: Up to MaxTurns attempts to improve the resolution
5. **Quality Validation**: Continuous assessment and early termination on good results

### Refinement Strategies
- **Deeper Go Analysis**: More detailed examination of Go semantics
- **Alternative Approaches**: Exploration of different resolution strategies
- **Context Enhancement**: Additional context from surrounding code
- **Best Practice Application**: Explicit application of Go idioms and conventions

## Integration with Syncwright Pipeline

The enhanced resolver integrates seamlessly with the Syncwright workflow:

```bash
# Standard Syncwright usage automatically uses enhanced Go resolution
syncwright resolve --repo /path/to/go/project

# With explicit configuration
syncwright resolve \
  --repo /path/to/go/project \
  --min-confidence 0.8 \
  --enable-multi-turn \
  --max-turns 5 \
  --verbose
```

## Best Practices

### For Development Teams
1. **Set Appropriate Confidence Thresholds**: Use 0.7+ for production code
2. **Enable Multi-Turn for Complex Projects**: Use refinement for critical codebases
3. **Review Low-Confidence Resolutions**: Always manually review < 0.7 confidence
4. **Monitor Resolution Patterns**: Track common conflict types for process improvement

### For CI/CD Integration
1. **Automated High-Confidence Resolutions**: Auto-apply resolutions > 0.8 confidence
2. **Manual Review Workflows**: Route low-confidence resolutions to human review
3. **Quality Gates**: Implement pre-commit hooks for resolution validation
4. **Metrics Collection**: Track resolution success rates and confidence distributions

## Troubleshooting

### Common Issues

**Low Confidence Scores**
- Increase `MaxTurns` for more refinement attempts
- Lower `MultiTurnThreshold` to trigger refinement earlier
- Enable verbose mode to understand validation failures

**Timeout Issues**
- Increase `TimeoutSeconds` for complex codebases
- Reduce `MaxBatchSize` for more focused analysis
- Consider breaking large conflicts into smaller chunks

**Context Understanding Problems**
- Ensure Claude CLI has access to full repository context
- Verify import paths are correctly configured
- Check that Go modules are properly initialized

### Performance Optimization

**For Large Codebases**
- Use smaller batch sizes (3-5 files)
- Implement parallel processing for independent conflicts
- Cache resolution patterns for similar conflict types

**For Time-Critical Workflows**
- Set reasonable timeout limits (60-180 seconds)
- Use confidence thresholds to skip obvious resolutions
- Implement fallback strategies for timeout scenarios

## Examples and Use Cases

See `resolver_example.go` for comprehensive usage examples and validation patterns.

## Contributing

When contributing to Go conflict resolution:

1. **Test with Real Go Codebases**: Validate against actual merge conflicts
2. **Measure Confidence Accuracy**: Ensure confidence scores reflect reality
3. **Validate Go Semantics**: Verify that resolutions compile and pass tests
4. **Document Edge Cases**: Record unusual conflict patterns and solutions
5. **Performance Benchmarks**: Measure resolution time and quality metrics