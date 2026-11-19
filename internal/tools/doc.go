// Package tools is the agent's fixed tool set, each tool a thin layer over fsguard, scan, scm and testresults. It tracks what the agent has read, so stale edits are refused.
//
// Requirements: M1, M2, M3, M9 (REQUIREMENTS.md).
package tools
