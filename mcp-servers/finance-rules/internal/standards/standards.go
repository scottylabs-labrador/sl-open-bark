// Package standards holds the reviewed reimbursement standards as data. The default set is embedded
// from reimbursement.json (version-controlled, finance-committee reviewed) so the deployed binary
// always has standards; an operator can override the file via the environment.
package standards

import _ "embed"

//go:embed reimbursement.json
var defaultJSON []byte

// Default returns the embedded, reviewed standards JSON.
func Default() []byte { return defaultJSON }
