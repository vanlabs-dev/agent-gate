// Command agent-gate-audit exports and verifies the gate audit log.
//
//	agent-gate-audit export --dsn <dsn> [--out <file>]
//	agent-gate-audit verify <file>
//
// export writes the entries and checkpoints of the log to a file, or to
// standard output.
//
// verify exits 0 only when every hash, seq, prev_hash and checkpoint checks
// out. Otherwise it names the first failed seq and exits 1.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/vanlabs-dev/agent-gate/gate/internal/audit"
	"github.com/vanlabs-dev/agent-gate/gate/internal/postgres"
)

const usage = `usage:
  agent-gate-audit export --dsn <dsn> [--out <file>]
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
	case "export":
		return export(args[1:], stdout, stderr)
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

func export(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("export", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dsn := fs.String("dsn", "", "Postgres connection string of the gate database")
	out := fs.String("out", "", "output file (default standard output)")
	if err := fs.Parse(args); err != nil || *dsn == "" || fs.NArg() != 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	conn, err := pgx.Connect(ctx, *dsn)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	defer conn.Close(ctx)
	x, err := postgres.Export(ctx, conn)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	b, err := json.MarshalIndent(x, "", "  ")
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	b = append(b, '\n')
	if *out == "" {
		_, err = stdout.Write(b)
	} else {
		err = os.WriteFile(*out, b, 0o600)
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}
