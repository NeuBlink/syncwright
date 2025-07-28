# Security Policy

## Security Overview

Syncwright takes security seriously and implements multiple layers of protection to ensure safe operation in production environments. This document outlines our security practices, data handling policies, and guidelines for secure usage.

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| 1.x.x   | ✅ Active support  |
| 0.x.x   | ❌ No longer supported |

Security updates are provided for the latest major version. Please upgrade to the latest version to receive security patches.

## Security Features

### 1. Sensitive Data Protection

Syncwright automatically identifies and excludes sensitive files and data patterns:

**Automatically Filtered Files:**
- Credentials: `.env`, `.env.*`, `*.key`, `*.pem`, `*.p12`, `*.jks`
- Configuration: `*config*.json`, `*secret*`, `*password*`, `*token*`
- Cloud credentials: `*.aws`, `.azure`, `.gcp`, `service-account*.json`
- SSH keys: `id_rsa`, `id_dsa`, `*.pub`, `known_hosts`
- Database dumps: `*.sql`, `*.dump`, `*.backup`

**Content Pattern Filtering:**
- API keys (AWS, Google, GitHub, etc.)
- Database connection strings
- JWT tokens
- Private keys and certificates
- Password hashes

### 2. Data Minimization

Syncwright only processes conflict regions, not entire files:

```json
{
  "conflict": {
    "file": "main.go",
    "start_line": 15,
    "end_line": 23,
    "context_lines": 5,
    "ours_lines": ["func main() {"],
    "theirs_lines": ["func start() {"],
    "pre_context": ["package main", "import \"fmt\""],
    "post_context": ["fmt.Println(\"Hello\")"]
  }
}
```

**What is NOT sent to AI:**
- Complete file contents
- Unrelated code sections
- Binary files
- Generated files
- Build artifacts
- Dependencies (node_modules, vendor, etc.)

### 3. Safe Resolution Application

- **Backup Creation**: Automatic backups before any file modifications
- **Atomic Operations**: Changes are applied atomically to prevent partial updates
- **Surgical Precision**: Only conflict markers and their immediate content are modified
- **Validation**: Syntax checking and conflict marker removal verification
- **Rollback Capability**: Easy restoration from backups if needed

### 4. Confidence-Based Safety

All AI resolutions include confidence scores:

```json
{
  "resolution": {
    "confidence": 0.85,
    "reasoning": "Clear intent preservation based on surrounding context",
    "safe_application": true
  }
}
```

- **High confidence (0.8+)**: Generally safe for automatic application
- **Medium confidence (0.6-0.8)**: Review recommended
- **Low confidence (<0.6)**: Manual review required

## Data Handling Policy

### What Data is Processed

**Local Processing Only:**
- Git repository metadata
- File structure analysis
- Conflict detection
- Backup creation
- Validation checks

**Sent to AI Service (Claude Code API):**
- Conflict regions only (not full files)
- Minimal surrounding context (configurable, default 5 lines)
- Programming language metadata
- No personally identifiable information
- No sensitive credentials or secrets

### Data Retention

- **Local data**: Remains on your system, automatic cleanup of temporary files
- **AI service**: No data retention, requests are not stored or logged by Anthropic
- **Backups**: Stored locally with configurable retention (default: 7 days)

### Data Transmission

- **Encryption**: All API communications use TLS 1.3
- **Authentication**: OAuth2 tokens with scoped permissions
- **No logging**: API tokens are never written to logs or disk
- **Minimal payload**: Only necessary conflict data transmitted

## Security Best Practices

### 1. Token Management

**DO:**
```bash
# Store tokens in environment variables
export CLAUDE_CODE_OAUTH_TOKEN="your-secure-token"

# Use GitHub Actions secrets
claude_code_oauth_token: ${{ secrets.CLAUDE_CODE_OAUTH_TOKEN }}

# Use secure credential stores
syncwright ai-apply --token-from-keychain
```

**DON'T:**
```bash
# Never hardcode tokens
syncwright ai-apply --token "sk-1234567890"  # ❌ NEVER

# Never commit tokens to git
echo "TOKEN=sk-1234567890" >> .env  # ❌ NEVER

# Never pass tokens as arguments (visible in process list)
./script.sh sk-1234567890  # ❌ NEVER
```

### 2. File Exclusion

Create a `.syncwright-ignore` file for additional exclusions:

```
# Additional sensitive patterns
*.backup
*_secret.yml
internal/secrets/
config/production.json

# Company-specific patterns
*.company-internal
sensitive-data/
proprietary-algorithms/
```

### 3. Validation Workflow

Always validate AI resolutions before committing:

```bash
# Recommended security workflow
syncwright ai-apply --dry-run --verbose  # 1. Preview changes
syncwright ai-apply --confidence-threshold 0.8  # 2. Apply with high confidence
syncwright validate --security-check  # 3. Security validation
git diff --name-only  # 4. Review changed files
git diff  # 5. Review actual changes
```

### 4. GitHub Actions Security

```yaml
# Secure GitHub Actions configuration
jobs:
  syncwright:
    runs-on: ubuntu-latest
    permissions:
      contents: read  # Minimal permissions
      pull-requests: write  # Only if needed
    
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 1  # Minimal history
      
      - name: Syncwright resolution
        uses: neublink/syncwright@v1.0.2
        with:
          claude_code_oauth_token: ${{ secrets.CLAUDE_CODE_OAUTH_TOKEN }}
          timeout_seconds: 300     # Reasonable timeout for security
          max_retries: 3          # Limited retries to prevent abuse
          debug_mode: false       # Disable debug in production (never log tokens)
          # Never use: ${{ github.token }} for AI operations
```

### 5. Debug Mode Security

**Production Environments:**
```yaml
# NEVER enable debug mode in production
debug_mode: false  # Default and recommended for production

# Secure timeout settings
timeout_seconds: 300   # Prevent resource exhaustion
max_retries: 3        # Limit retry attempts
```

**Development/Testing Only:**
```yaml
# Only enable for debugging in secure environments
debug_mode: true      # Detailed logging for troubleshooting
timeout_seconds: 600  # Extended timeout for debugging
max_retries: 5       # More retries for testing reliability
```

**Debug Mode Risks:**
- May expose API request details in logs
- Could reveal repository structure information
- Increases log verbosity and storage requirements
- Should never be used with sensitive repositories

### 6. Network Security

**Allowed Domains:**
- `api.anthropic.com` (Claude Code API)
- `github.com` (for releases and updates)

**Firewall Configuration:**
```bash
# Allow only necessary outbound connections
iptables -A OUTPUT -d api.anthropic.com -p tcp --dport 443 -j ACCEPT
iptables -A OUTPUT -d github.com -p tcp --dport 443 -j ACCEPT
```

## Vulnerability Reporting

### How to Report

If you discover a security vulnerability, please report it responsibly:

1. **Email**: security@neublink.com
2. **Subject**: "Syncwright Security Vulnerability Report"
3. **Include**:
   - Description of the vulnerability
   - Steps to reproduce
   - Potential impact assessment
   - Suggested mitigation (if any)

### What to Expect

- **Acknowledgment**: Within 24 hours
- **Initial assessment**: Within 72 hours
- **Status updates**: Weekly until resolution
- **Fix timeline**: Critical issues within 7 days, others within 30 days

### Responsible Disclosure

Please follow responsible disclosure practices:

- **Don't** publicly disclose until we've had time to fix
- **Don't** test vulnerabilities against systems you don't own
- **Do** provide clear reproduction steps
- **Do** allow reasonable time for fixes (90 days standard)

## Security Hardening

### Production Deployment

```bash
# 1. Use read-only file systems where possible
docker run --read-only -v /tmp --tmpfs /tmp syncwright

# 2. Run with minimal privileges
useradd -r -s /bin/false syncwright
sudo -u syncwright syncwright detect

# 3. Network isolation
docker run --network none -v /repo:/repo syncwright detect --offline

# 4. Resource limits with timeout controls
docker run --memory=512m --cpus=1 \
  -e SYNCWRIGHT_TIMEOUT=300 \
  -e SYNCWRIGHT_MAX_RETRIES=3 \
  syncwright

# 5. Disable debug mode in production
docker run -e SYNCWRIGHT_DEBUG=false syncwright
```

### CI/CD Security

```yaml
# Security-hardened CI/CD pipeline
- name: Security scan before Syncwright
  run: |
    # Scan for secrets in codebase
    git-secrets --scan
    
    # Check for suspicious file changes
    git diff --name-only | grep -E '\.(key|pem|p12)$' && exit 1

- name: Syncwright with security controls
  uses: neublink/syncwright@v1.0.2
  with:
    claude_code_oauth_token: ${{ secrets.CLAUDE_CODE_OAUTH_TOKEN }}
    timeout_seconds: 300      # Prevent resource exhaustion
    max_retries: 2           # Limited retries for security
    debug_mode: false        # Never enable debug in production
    max_tokens: 8000         # Limit token usage
    confidence_threshold: 0.8  # High confidence only
    dry_run: true            # Preview mode for sensitive repos
  
- name: Post-resolution security check
  run: |
    # Verify no secrets were introduced
    git diff | grep -E '(password|secret|key|token)' && exit 1
    
    # Check for remaining conflict markers
    git ls-files -u | grep -q . && exit 1
```

### Monitoring and Auditing

```bash
# Log all Syncwright operations
export SYNCWRIGHT_AUDIT_LOG="/var/log/syncwright-audit.log"

# Monitor for suspicious patterns
tail -f /var/log/syncwright-audit.log | grep -E "(SECURITY|ERROR|FAIL)"

# Audit token usage
grep "API_CALL" /var/log/syncwright-audit.log | jq '.timestamp, .operation, .confidence'
```

## Security FAQ

### Q: Is my code sent to external services?

**A:** Only conflict regions (typically 10-20 lines) are sent to the Claude Code API. Complete files, sensitive data, and unrelated code remain local.

### Q: How is my API token protected?

**A:** Tokens are only used for API authentication and are never logged, cached, or written to disk. Use environment variables or secure secret management.

### Q: What happens if the AI suggests malicious code?

**A:** All suggestions include confidence scores and reasoning. Low-confidence suggestions require manual review. Additionally, Syncwright validates syntax and checks for obvious security anti-patterns.

### Q: Can Syncwright be used offline?

**A:** Yes, for detection, formatting, and validation. AI-powered resolution requires API access, but you can export conflicts and process them in a secure environment.

### Q: How do I audit Syncwright usage?

**A:** Enable audit logging with `SYNCWRIGHT_AUDIT_LOG` environment variable. All operations are logged with timestamps, user context, and operation details.

### Q: What compliance standards does Syncwright meet?

**A:** Syncwright is designed to support SOC 2, GDPR, and HIPAA compliance through data minimization, encryption in transit, and no data retention. Contact us for specific compliance documentation.

## Security Updates

Subscribe to security notifications:

- **GitHub Security Advisories**: [github.com/NeuBlink/syncwright/security](https://github.com/NeuBlink/syncwright/security)
- **Email notifications**: security-announce@neublink.com
- **RSS feed**: Available through GitHub releases

For the latest security information and updates, always refer to the [official documentation](https://github.com/NeuBlink/syncwright).

---

**Remember**: Security is a shared responsibility. While Syncwright provides robust security features, proper configuration and usage practices are essential for maintaining security in your environment.