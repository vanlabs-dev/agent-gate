// Package canonical encodes tool call arguments with RFC 8785 (JSON
// Canonicalization Scheme) and computes the call and argument digests that
// approvals bind to.
package canonical

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"strconv"
	"strings"

	"github.com/gowebpki/jcs"
)

// CallSchema is the schema version inside the hashed bytes of a call digest.
const CallSchema = "agent-gate.call/v1"

// ErrUnsafeNumber is returned for an integer literal outside ±(2^53 − 1), or a
// number that overflows a 64-bit float. Two different integers of that kind
// can round to the same float, and so share a digest.
var ErrUnsafeNumber = errors.New("unsafe number in arguments")

var maxSafeInt = big.NewInt(1<<53 - 1)

// Arguments returns the RFC 8785 encoding of the arguments. Absent arguments
// (empty input) encode as null.
func Arguments(raw json.RawMessage) ([]byte, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return []byte("null"), nil
	}
	if err := checkNumbers(raw); err != nil {
		return nil, err
	}
	out, err := jcs.Transform(raw)
	if err != nil {
		return nil, fmt.Errorf("canonical encoding: %w", err)
	}
	return out, nil
}

// checkNumbers returns ErrUnsafeNumber for the first unsafe number in raw.
func checkNumbers(raw []byte) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("invalid arguments: %w", err)
		}
		n, ok := tok.(json.Number)
		if !ok {
			continue
		}
		s := n.String()
		if !strings.ContainsAny(s, ".eE") {
			i, ok := new(big.Int).SetString(s, 10)
			if !ok || i.CmpAbs(maxSafeInt) > 0 {
				return fmt.Errorf("%w: integer %s is outside ±(2^53 − 1)", ErrUnsafeNumber, short(s))
			}
			continue
		}
		if _, err := strconv.ParseFloat(s, 64); err != nil {
			return fmt.Errorf("%w: %s overflows a 64-bit float", ErrUnsafeNumber, short(s))
		}
	}
}

func short(s string) string {
	if len(s) > 32 {
		return s[:32] + "..."
	}
	return s
}

// Params reads the tool name and the arguments from the params of a
// tools/call. It ignores every other field, such as _meta.
func Params(raw []byte) (tool string, arguments json.RawMessage, err error) {
	var p struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return "", nil, fmt.Errorf("invalid params: %w", err)
	}
	return p.Name, p.Arguments, nil
}

// Digest returns "sha256:" and the lowercase hex SHA-256 of b.
func Digest(b []byte) string {
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// ArgsDigest returns the digest of canonical arguments from Arguments.
func ArgsDigest(canonicalArgs []byte) string {
	return Digest(canonicalArgs)
}

// Call identifies one tool call for a call digest.
type Call struct {
	Tenant string
	Agent  string
	Target string
	Tool   string
	// Arguments is the output of Arguments.
	Arguments []byte
}

// CallDigest returns the digest that an approval binds to.
func CallDigest(c Call) (string, error) {
	b, err := json.Marshal(struct {
		Schema    string          `json:"schema"`
		Tenant    string          `json:"tenant"`
		Agent     string          `json:"agent"`
		Target    string          `json:"target"`
		Tool      string          `json:"tool"`
		Arguments json.RawMessage `json:"arguments"`
	}{CallSchema, c.Tenant, c.Agent, c.Target, c.Tool, c.Arguments})
	if err != nil {
		return "", err
	}
	out, err := jcs.Transform(b)
	if err != nil {
		return "", err
	}
	return Digest(out), nil
}

// JCS returns the RFC 8785 encoding of v, marshalled with encoding/json.
func JCS(v any) ([]byte, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return jcs.Transform(b)
}
