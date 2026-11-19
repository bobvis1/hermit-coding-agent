// Package fsguard is the only code that touches the repository's files: root confinement, path normalization, protected, unreadable and allowlisted paths, file-type detection, and atomic and conditional writes. It imports nothing else from Hermit.
//
// Requirements: M8, M13, M21, M6 (REQUIREMENTS.md).
package fsguard
