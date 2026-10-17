package ledger

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
)

// Exit codes returned by Run.
const (
	ExitOK       = 0 // every transaction applied
	ExitTxFailed = 1 // input processed, but at least one transaction failed
	ExitUsage    = 2 // bad command-line usage
	ExitIO       = 3 // the input could not be opened or read
)

// Run is the testable body of the ledger command.
//
//	ledger [-q] [FILE]
//
// It reads transactions from FILE, or from stdin when FILE is absent or "-".
// Each failed transaction is reported on stderr as "ledger: line N: ...",
// unless -q is given. Final balances are printed to stdout, one
// "ACCOUNT AMOUNT" line per account in sorted order, followed on stderr by
// "ledger: A applied, F failed". -h prints usage and returns ExitOK.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("ledger", flag.ContinueOnError)
	fs.SetOutput(stderr)
	quiet := fs.Bool("q", false, "do not print individual failures")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "usage: ledger [-q] [FILE]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return ExitOK
		}
		return ExitUsage // the FlagSet has already printed the error and usage
	}
	if fs.NArg() > 1 {
		fmt.Fprintf(stderr, "ledger: too many arguments: %q\n", fs.Args())
		fs.Usage()
		return ExitUsage
	}

	in := stdin
	if name := fs.Arg(0); name != "" && name != "-" {
		f, err := os.Open(name)
		if err != nil {
			fmt.Fprintf(stderr, "ledger: %v\n", err)
			return ExitIO
		}
		defer f.Close() // read-only: a Close error cannot lose data
		in = f
	}

	l := New()
	res, err := l.Process(in)
	if err != nil {
		fmt.Fprintf(stderr, "ledger: %v\n", err)
		return ExitIO
	}

	if !*quiet {
		for _, f := range res.Failures {
			fmt.Fprintf(stderr, "ledger: %v\n", f)
		}
	}
	for _, name := range l.Accounts() {
		bal, _ := l.Balance(name) // name came from Accounts, so it exists
		fmt.Fprintf(stdout, "%s %s\n", name, FormatAmount(bal))
	}
	fmt.Fprintf(stderr, "ledger: %d applied, %d failed\n", res.Applied, len(res.Failures))

	if len(res.Failures) > 0 {
		return ExitTxFailed
	}
	return ExitOK
}
