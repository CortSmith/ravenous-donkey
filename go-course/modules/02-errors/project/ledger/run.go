package ledger

import (
	"io"
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
	// TODO: Implement Run using flag.NewFlagSet.
	// Parse the -q flag and positional FILE argument.
	// - If FILE is empty or "-", read from stdin.
	// - Otherwise, open FILE with os.Open (return ExitIO on error).
	// - Create a ledger with New() and call Process.
	// - If Process returns an error, print it to stderr and return ExitIO.
	// - Unless -q, print each failure to stderr as "ledger: <error>".
	// - Print final balances to stdout, one line per account (sorted).
	// - Print a summary to stderr.
	// - Return ExitOK if no failures, ExitTxFailed if any failed.
	return 0
}
