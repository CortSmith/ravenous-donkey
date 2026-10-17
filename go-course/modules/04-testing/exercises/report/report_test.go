package report

import (
	"bytes"
	"testing"
)

// Write tests to expose the 3 bugs in report.go. Use golden files for complex output.
//
// TODO: Declare var update = flag.Bool("update", false, "update golden files")
//
// TODO: Write TestRenderGolden (table-driven):
//   - Cases: vary what a real bill varies: item count, the names, the
//     size of quantities and prices
//   - Compare with testdata/<name>.golden using -update flag
//
// TODO: Write TestMoney (table-driven):
//   - Unit test money() function directly
//
// TODO: Bonus: TestRenderLayoutRules and ExampleRender

func TestRenderSmoke(t *testing.T) {
	var buf bytes.Buffer
	if err := Render(&buf, nil); err != nil {
		t.Fatalf("Render(nil) error = %v", err)
	}
	if buf.Len() == 0 {
		t.Error("Render(nil) wrote nothing")
	}
}
