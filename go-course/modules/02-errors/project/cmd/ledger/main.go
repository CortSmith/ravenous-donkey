// Command ledger applies a file of bank transactions and prints the final
// balances. See package ledger for the file format and exit codes.
//
//	go run ./modules/02-errors/project/cmd/ledger modules/02-errors/project/ledger/testdata/transactions.txt
package main

import (
	"os"

	"gocourse/modules/02-errors/project/ledger"
)

func main() {
	os.Exit(ledger.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
