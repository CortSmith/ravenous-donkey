package ledger

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"testing/iotest"
)

// runLedger calls Run and returns exit code, stdout and stderr.
func runLedger(t *testing.T, stdin string, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := Run(args, strings.NewReader(stdin), &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

const sampleBalances = "alice 86.75\nbob 29.50\ncarol 4.25\n"

func TestRunSampleFile(t *testing.T) {
	code, stdout, stderr := runLedger(t, "", filepath.Join("testdata", "transactions.txt"))
	if code != ExitTxFailed {
		t.Errorf("exit code = %d, want %d (ExitTxFailed)\nstderr:\n%s", code, ExitTxFailed, stderr)
	}
	if stdout != sampleBalances {
		t.Errorf("stdout =\n%s\nwant\n%s", stdout, sampleBalances)
	}
	for _, want := range []string{
		"ledger: line 14: withdraw carol: insufficient funds\n",
		"ledger: line 15: deposit dave: unknown account\n",
		"ledger: line 16: open alice: account already exists\n",
		"ledger: line 17: transfer bob: cannot transfer to the same account\n",
		"ledger: line 18: invalid amount \"-3\"\n",
		"ledger: line 19: syntax error: unknown command \"refund\"\n",
		"ledger: line 20: syntax error: usage: transfer FROM TO AMOUNT\n",
		"ledger: 8 applied, 7 failed\n",
	} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr missing %q; got:\n%s", want, stderr)
		}
	}
	if strings.Contains(stdout, "ledger:") {
		t.Errorf("diagnostics leaked to stdout:\n%s", stdout)
	}
}

func TestRunQuiet(t *testing.T) {
	code, stdout, stderr := runLedger(t, "", "-q", filepath.Join("testdata", "transactions.txt"))
	if code != ExitTxFailed {
		t.Errorf("exit code = %d, want %d", code, ExitTxFailed)
	}
	if stdout != sampleBalances {
		t.Errorf("stdout = %q, want %q", stdout, sampleBalances)
	}
	if want := "ledger: 8 applied, 7 failed\n"; stderr != want {
		t.Errorf("with -q, stderr = %q, want only %q", stderr, want)
	}
}

func TestRunStdin(t *testing.T) {
	input := "open b 2\nopen a 1.5\ndeposit a 1\n"
	for _, args := range [][]string{nil, {"-"}} {
		code, stdout, stderr := runLedger(t, input, args...)
		if code != ExitOK {
			t.Errorf("Run(%q) exit code = %d, want %d; stderr:\n%s", args, code, ExitOK, stderr)
		}
		if want := "a 2.50\nb 2.00\n"; stdout != want {
			t.Errorf("Run(%q) stdout = %q, want %q", args, stdout, want)
		}
		if want := "ledger: 3 applied, 0 failed\n"; stderr != want {
			t.Errorf("Run(%q) stderr = %q, want %q", args, stderr, want)
		}
	}
}

func TestRunUsageErrors(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantCode  int
		wantInErr string
	}{
		{"help", []string{"-h"}, ExitOK, "usage: ledger"},
		{"unknown flag", []string{"-bogus"}, ExitUsage, "flag provided but not defined"},
		{"too many args", []string{"a.txt", "b.txt"}, ExitUsage, "too many arguments"},
		{"missing file", []string{filepath.Join(t.TempDir(), "nope.txt")}, ExitIO, "no such file or directory"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, stdout, stderr := runLedger(t, "", tt.args...)
			if code != tt.wantCode {
				t.Errorf("Run(%q) exit code = %d, want %d; stderr:\n%s", tt.args, code, tt.wantCode, stderr)
			}
			if !strings.Contains(stderr, tt.wantInErr) {
				t.Errorf("Run(%q) stderr = %q, want it to contain %q", tt.args, stderr, tt.wantInErr)
			}
			if stdout != "" {
				t.Errorf("Run(%q) wrote to stdout: %q", tt.args, stdout)
			}
		})
	}
}

func TestRunReadError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(nil, iotest.ErrReader(errors.New("pipe broke")), &stdout, &stderr)
	if code != ExitIO {
		t.Errorf("exit code = %d, want %d (ExitIO)", code, ExitIO)
	}
	if want := "ledger: read transactions: pipe broke\n"; stderr.String() != want {
		t.Errorf("stderr = %q, want %q", stderr.String(), want)
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want nothing when input could not be read", stdout.String())
	}
}
