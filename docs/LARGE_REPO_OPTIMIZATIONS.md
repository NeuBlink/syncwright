# Large Repository Optimizations

This document describes the performance optimizations implemented in Syncwright's GitHub Action for handling large repositories with >500 conflicted files.

## Overview

The Syncwright composite action has been optimized to handle large-scale conflict resolution scenarios efficiently while maintaining security and reliability. These optimizations address timeout issues, memory constraints, and API rate limiting that can occur with repositories containing hundreds of conflicted files.

## Key Optimizations

### 1. Intelligent Batching System

**Problem**: Processing all conflicts simultaneously overwhelms the Claude API and causes timeouts.

**Solution**: Automatic batch size calculation based on repository size:
- **Small repos** (<50 conflicts): Standard resolution mode
- **Medium repos** (50-500 conflicts): Batch size of 25
- **Large repos** (500-1000 conflicts): Batch size of 10  
- **Very large repos** (>1000 conflicts): Batch size of 5

```yaml
inputs:
  batch_size:
    description: 'Number of conflicts to process per batch (default: auto-calculated)'
    default: '0'  # 0 = auto-calculate
```

### 2. Enhanced Claude API Rate Limiting

**Problem**: API rate limits cause failures without proper backoff strategies.

**Solution**: Intelligent retry logic with exponential backoff:
- **Rate limit errors**: 5-second base delay with exponential backoff
- **Server errors**: 2-second base delay
- **Network errors**: 1-second base delay
- **Jitter**: ±25% randomization to prevent thundering herd
- **Maximum backoff**: Capped at 30 seconds

### 3. Parallel Processing with Concurrency Control

**Problem**: Sequential processing is too slow for large repositories.

**Solution**: Configurable concurrent batch processing:
- **Default concurrency**: 3 concurrent batches
- **Memory-aware scaling**: Reduces concurrency for very large repos
- **Secure token handling**: Each worker uses the same secure token

```yaml
inputs:
  concurrency:
    description: 'Number of concurrent batches to process'
    default: '3'
```

### 4. Streaming and Memory Optimization

**Problem**: Loading all conflicts into memory causes out-of-memory errors.

**Solution**: Streaming processing with memory management:
- **Automatic streaming**: Enabled for repos with >500 conflicts
- **Memory monitoring**: Real-time memory usage tracking
- **Context reduction**: Fewer context lines for very large repos
- **Garbage collection**: Forced cleanup after processing

```yaml
inputs:
  enable_streaming:
    description: 'Enable streaming processing for large repositories'
    default: 'true'
```

### 5. Enhanced Progress Reporting

**Problem**: Long-running operations lack visibility into progress.

**Solution**: Comprehensive progress tracking:
- **Real-time updates**: Progress messages during batch processing
- **Performance metrics**: Processing time, memory usage, batch counts
- **Detailed summaries**: Complete resolution statistics
- **Error context**: Clear error messages with suggested solutions

### 6. Configurable Timeouts and Thresholds

**Problem**: Default timeouts are insufficient for large repositories.

**Solution**: Flexible timeout and confidence configuration:
- **Extended timeouts**: Up to 30 minutes for very large repos
- **Confidence thresholds**: Configurable AI confidence requirements
- **Retry limits**: Configurable maximum retry attempts
- **Fallback strategies**: Alternative processing modes on failure

```yaml
inputs:
  timeout_minutes:
    description: 'Timeout in minutes for the entire resolution process'
    default: '30'
  confidence_threshold:
    description: 'Minimum confidence threshold for AI resolutions (0.0-1.0)'
    default: '0.7'
  max_retries:
    description: 'Maximum retry attempts for failed API requests'
    default: '3'
```

## Usage Examples

### Basic Large Repository Setup

```yaml
- name: Resolve conflicts in large repository
  uses: NeuBlink/syncwright@v1
  with:
    claude_code_oauth_token: ${{ secrets.CLAUDE_CODE_OAUTH_TOKEN }}
    merge_failed: true
    # Optimizations are applied automatically based on repository size
```

### Custom Configuration for Very Large Repositories

```yaml
- name: Resolve conflicts with custom optimization
  uses: NeuBlink/syncwright@v1
  with:
    claude_code_oauth_token: ${{ secrets.CLAUDE_CODE_OAUTH_TOKEN }}
    merge_failed: true
    batch_size: 5           # Small batches for memory efficiency
    concurrency: 2          # Reduced concurrency to avoid rate limits
    timeout_minutes: 45     # Extended timeout for complex processing
    confidence_threshold: 0.8  # Higher confidence for critical repositories
    max_retries: 5          # More retries for reliability
    enable_streaming: true  # Force streaming for memory efficiency
```

### Development and Testing Setup

```yaml
- name: Test conflict resolution
  uses: NeuBlink/syncwright@v1
  with:
    claude_code_oauth_token: ${{ secrets.CLAUDE_CODE_OAUTH_TOKEN }}
    merge_failed: true
    batch_size: 10          # Moderate batch size for testing
    timeout_minutes: 15     # Shorter timeout for development
    confidence_threshold: 0.6  # Lower confidence for experimentation
    run_validation: true    # Enable validation for development
```

## Performance Characteristics

### Processing Times by Repository Size

| Repository Size | Conflicts | Processing Time | Memory Usage | API Calls |
|----------------|-----------|-----------------|--------------|-----------|
| Small          | <50       | 30s - 2min      | <100MB       | 1-5       |
| Medium         | 50-500    | 2min - 10min    | 100-300MB    | 5-25      |
| Large          | 500-1000  | 10min - 25min   | 200-500MB    | 25-100    |
| Very Large     | >1000     | 25min - 45min   | 300-800MB    | 100-500   |

### Scaling Characteristics

- **Linear scaling**: Processing time scales roughly linearly with conflict count
- **Memory efficiency**: Memory usage is bounded regardless of repository size
- **Rate limit compliance**: Automatic backoff prevents API quota exhaustion
- **Fault tolerance**: Robust error handling with multiple fallback strategies

## Monitoring and Observability

### Output Metrics

The optimized action provides comprehensive metrics:

```yaml
outputs:
  total_conflicts: "Total number of conflicts detected"
  resolved_conflicts: "Number of conflicts successfully resolved"
  processing_time: "Total processing time in seconds"
  ai_confidence: "Overall confidence score (0.0-1.0)"
  batches_processed: "Number of batches processed"
  files_modified: "Number of files modified"
```

### Summary Reports

Detailed GitHub Step Summary includes:
- **Resolution Summary**: Success rates, processing time, confidence scores
- **Configuration Used**: Batch size, concurrency, timeouts
- **Performance Metrics**: Memory usage, repository size, processing efficiency
- **Next Steps**: Actionable guidance for post-resolution activities

## Troubleshooting

### Common Issues and Solutions

#### Timeout Errors
**Symptom**: Action times out before completion
**Solution**: Increase `timeout_minutes` or reduce `batch_size`

#### Rate Limiting
**Symptom**: "429 Too Many Requests" errors
**Solution**: Reduce `concurrency` or increase `max_retries`

#### Memory Issues
**Symptom**: Out of memory errors or slow performance
**Solution**: Enable `enable_streaming` and reduce `batch_size`

#### Low Confidence Resolutions
**Symptom**: Many conflicts not resolved due to low confidence
**Solution**: Lower `confidence_threshold` or improve context in conflicts

### Debug Mode

Enable verbose logging for troubleshooting:

```yaml
- name: Debug large repository processing
  uses: NeuBlink/syncwright@v1
  with:
    claude_code_oauth_token: ${{ secrets.CLAUDE_CODE_OAUTH_TOKEN }}
    merge_failed: true
  env:
    SYNCWRIGHT_DEBUG: "true"
```

## Security Considerations

### Token Security
- **Secure storage**: OAuth tokens are handled securely throughout processing
- **No token logging**: Tokens are never logged or exposed in output
- **Worker isolation**: Each concurrent worker uses the same secure token reference
- **Memory cleanup**: Token references are cleared after processing

### API Security
- **Rate limit compliance**: Built-in rate limiting prevents quota abuse
- **Retry security**: Exponential backoff prevents API hammering
- **Error handling**: Secure error messages don't expose sensitive information
- **Session management**: Proper session cleanup after processing

## Best Practices

### Repository Preparation
1. **Clean repository**: Remove unnecessary files before conflict resolution
2. **Merge strategy**: Use appropriate merge strategies to minimize conflicts
3. **Branch hygiene**: Keep feature branches up-to-date with main branch
4. **File organization**: Organize code to minimize conflicting changes

### Action Configuration
1. **Start conservative**: Begin with default settings and adjust as needed
2. **Monitor performance**: Use output metrics to optimize configuration
3. **Test thoroughly**: Use the test workflow to validate configurations
4. **Document changes**: Record successful configurations for team use

### Continuous Improvement
1. **Monitor trends**: Track resolution success rates over time
2. **Optimize regularly**: Adjust settings based on repository evolution
3. **Update dependencies**: Keep Syncwright version up-to-date
4. **Team training**: Ensure team understands conflict resolution best practices

## Future Enhancements

### Planned Improvements
- **Adaptive batching**: Dynamic batch size adjustment during processing
- **ML optimization**: Machine learning-based parameter optimization  
- **Distributed processing**: Multi-runner parallel processing for extreme scale
- **Advanced caching**: Intelligent caching of AI responses for similar conflicts
- **Integration hooks**: Webhooks for external monitoring and alerting

### Community Contributions
We welcome contributions to improve large repository handling:
- **Performance testing**: Benchmarks on different repository types
- **Algorithm improvements**: Better batching and scheduling algorithms  
- **Monitoring enhancements**: Additional metrics and observability features
- **Documentation**: Usage examples and best practices

---

For questions or issues related to large repository optimizations, please:
1. Check the [troubleshooting section](#troubleshooting) above
2. Review the [GitHub Issues](https://github.com/NeuBlink/syncwright/issues) for similar problems
3. Create a new issue with detailed reproduction steps and repository characteristics
4. Consider contributing improvements back to the community