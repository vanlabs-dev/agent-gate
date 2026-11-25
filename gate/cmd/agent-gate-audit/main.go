// Command agent-gate-audit exports and verifies the gate audit log.
//
//	agent-gate-audit verify <file>
//
// verify exits 0 only when every hash, seq, prev_hash and checkpoint checks
// out. Otherwise it names the first failed seq and exits 1.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/vanlabs-dev/agent-gate/gate/internal/audit"
)

const usage = `usage:
  agent-gate-audit verify <file>
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	switch args[0] {
	case "verify":
		if len(args) != 2 {
			fmt.Fprint(stderr, usage)
			return 2
		}
		return verify(args[1], stdout, stderr)
	case "-h", "--help", "help":
		fmt.Fprint(stdout, usage)
		return 0
	default:
		fmt.Fprint(stderr, usage)
		return 2
	}
}

func verify(path string, stdout, stderr io.Writer) int {
	b, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	x, err := audit.Parse(b)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	r, err := audit.Verify(x)
	var ve *audit.VerifyError
	if errors.As(err, &ve) {
		fmt.Fprintf(stderr, "FAIL: %v\n", ve)
		return 1
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintf(stdout, "OK: %d entries, %d checkpoints, anchored head seq %d\n", r.Entries, r.Checkpoints, r.AnchoredHead)
	if r.Unanchored > 0 {
		fmt.Fprintf(stdout, "not anchored: %d entries after seq %d\n", r.Unanchored, r.AnchoredHead)
	}
	return 0
}
