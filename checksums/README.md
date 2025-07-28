# Binary Checksums for Security Verification

This directory contains SHA256 checksums for Syncwright binaries to ensure integrity and prevent tampering.

## Structure

- `syncwright-Linux-checksums.txt` - Linux binary checksums
- `syncwright-macOS-checksums.txt` - macOS binary checksums  
- `syncwright-Windows-checksums.txt` - Windows binary checksums

## Format

Each checksum file contains lines in the format:
```
<sha256_hash> <binary_name>
```

Example:
```
a1b2c3d4e5f6... syncwright
f6e5d4c3b2a1... syncwright.exe
```

## Generation

Checksums should be generated during the CI/CD build process using:

```bash
# Linux/macOS
sha256sum syncwright > checksums/syncwright-Linux-checksums.txt

# Windows (PowerShell)
Get-FileHash syncwright.exe | Format-List | Out-File syncwright-Windows-checksums.txt
```

## Security

- Checksums are verified automatically by the GitHub Action before binary execution
- Missing checksums will trigger a warning but won't fail the workflow
- Invalid checksums will fail the workflow for security protection