package policy

import (
	"encoding/json"
	"strings"
)

// sensitiveKeys are redacted from audit arguments. Untrusted/secret-ish fields never land in the
// audit log in the clear (design Section 10.2).
var sensitiveKeys = map[string]bool{
	"authorization": true, "token": true, "api_key": true, "apikey": true,
	"password": true, "secret": true, "access_token": true, "refresh_token": true,
}

// RedactArgs returns a copy of a JSON object argument with sensitive top-level values masked. If
// the payload is empty or not a JSON object, it is returned unchanged (an empty object for nil).
func RedactArgs(args json.RawMessage) json.RawMessage {
	if len(args) == 0 {
		return json.RawMessage("{}")
	}
	var obj map[string]any
	if err := json.Unmarshal(args, &obj); err != nil {
		return args // not an object; leave as-is (it is still recorded, just not field-redacted)
	}
	for k := range obj {
		if sensitiveKeys[strings.ToLower(k)] {
			obj[k] = "***"
		}
	}
	out, err := json.Marshal(obj)
	if err != nil {
		return json.RawMessage("{}")
	}
	return out
}
