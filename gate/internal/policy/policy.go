// Package policy decides a tool call: allow, deny or needs approval.
//
// The Decider interface hides the engine. The OPA implementation evaluates
// data.agentgate.decision over Rego files loaded once from a directory.
package policy

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/open-policy-agent/opa/v1/rego"
)

// Outcome is the result of a policy decision.
type Outcome string

const (
	Allow   Outcome = "allow"
	Deny    Outcome = "deny"
	Approve Outcome = "approve"
)

// Input is what policy reads for one call.
type Input struct {
	Tenant string `json:"tenant"`
	Agent  string `json:"agent"`
	Target string `json:"target"`
	Tool   string `json:"tool"`
	// Arguments is the decoded JSON arguments, or nil when absent.
	Arguments any `json:"arguments"`
	// Claims holds the identity metadata keys that the gateway forwards.
	Claims map[string]any `json:"claims"`
}

// Decision is one policy outcome with its reason.
type Decision struct {
	Outcome Outcome
	Reason  string
}

// Decider decides tool calls. Decide never fails: an error is a deny.
type Decider interface {
	Decide(ctx context.Context, in Input) Decision
	// Version identifies the loaded policy.
	Version() string
}

// Query is the decision document the gate evaluates.
const Query = "data.agentgate.decision"

// OPA is the Decider backed by embedded OPA.
type OPA struct {
	query   rego.PreparedEvalQuery
	version string
}

// Load reads each top-level *.rego file in dir, follows symlinks, skips names
// that start with ".", and prepares the query. It fails when a file does not
// compile or no policy file is found.
func Load(ctx context.Context, dir string) (*OPA, error) {
	files, err := readDir(dir)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("policy: no .rego file in %s", dir)
	}
	opts := []func(*rego.Rego){rego.Query(Query)}
	for _, f := range files {
		opts = append(opts, rego.Module(filepath.Join(dir, f.name), string(f.content)))
	}
	q, err := rego.New(opts...).PrepareForEval(ctx)
	if err != nil {
		return nil, fmt.Errorf("policy: %w", err)
	}
	return &OPA{query: q, version: version(files)}, nil
}

type file struct {
	name    string
	content []byte
}

// readDir returns the policy files of dir, sorted by name. A ConfigMap volume
// holds a "..data" link and a timestamped directory: both start with "." and
// are skipped, and the top-level links to them are followed.
func readDir(dir string) ([]file, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("policy: %w", err)
	}
	var files []file
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") || !strings.HasSuffix(name, ".rego") {
			continue
		}
		path := filepath.Join(dir, name)
		info, err := os.Stat(path) // follows symlinks
		if err != nil {
			return nil, fmt.Errorf("policy: %w", err)
		}
		if !info.Mode().IsRegular() {
			continue
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("policy: %w", err)
		}
		files = append(files, file{name, b})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].name < files[j].name })
	return files, nil
}

// version is "sha256:" over, for each file in name order: the base name, a NUL
// byte, the decimal content length, a NUL byte, and the content.
func version(files []file) string {
	h := sha256.New()
	for _, f := range files {
		h.Write([]byte(f.name))
		h.Write([]byte{0})
		h.Write([]byte(strconv.Itoa(len(f.content))))
		h.Write([]byte{0})
		h.Write(f.content)
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

// Version returns the digest of the loaded policy files.
func (o *OPA) Version() string { return o.version }

// Decide evaluates the decision document. An error, an undefined decision or a
// value that is not a valid decision is a deny.
func (o *OPA) Decide(ctx context.Context, in Input) Decision {
	rs, err := o.query.Eval(ctx, rego.EvalInput(in))
	if err != nil {
		return Decision{Deny, "policy error: " + err.Error()}
	}
	if len(rs) != 1 || len(rs[0].Expressions) != 1 {
		return Decision{Deny, "policy defines no decision"}
	}
	doc, ok := rs[0].Expressions[0].Value.(map[string]any)
	if !ok {
		return Decision{Deny, "policy decision is not an object"}
	}
	outcome, _ := doc["outcome"].(string)
	reason, ok := doc["reason"].(string)
	if !ok {
		return Decision{Deny, "policy decision has no reason"}
	}
	switch Outcome(outcome) {
	case Allow, Deny, Approve:
		return Decision{Outcome(outcome), reason}
	default:
		return Decision{Deny, fmt.Sprintf("policy returned invalid outcome %q", outcome)}
	}
}
