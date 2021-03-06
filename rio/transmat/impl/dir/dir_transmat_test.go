package dir

import (
	"os"
	"path/filepath"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"go.polydawn.net/repeatr/lib/testutil"
	"go.polydawn.net/repeatr/rio/tests"
)

func TestCoreCompliance(t *testing.T) {
	Convey("Spec Compliance: Dir Transmat", t, testutil.WithTmpdir(func() {
		// scanning
		tests.CheckScanWithoutMutation(Kind, New)
		tests.CheckScanProducesConsistentHash(Kind, New)
		tests.CheckScanProducesDistinctHashes(Kind, New)
		tests.CheckScanEmptyIsCalm(Kind, New)
		tests.CheckScanWithFilters(Kind, New)
		tests.CheckMultipleCommit(Kind, New, "file://bounce", "relative")
		testutil.WithTmpdir(func() {
			os.Mkdir("bounce", 0755) // make the warehouse location
			tests.CheckMultipleCommit(Kind, New, "file+ca://bounce", "content-addressable")
		})(nil)

		// round-trip (with relative paths)
		tests.CheckRoundTrip(Kind, New, "file://bounce", "file literal", "relative (implicit)")
		tests.CheckRoundTrip(Kind, New, "file://./bounce", "file literal", "relative (dotted)")
		// round-trip (with absolute paths)
		cwd, _ := os.Getwd()
		tests.CheckRoundTrip(Kind, New, "file://"+filepath.Join(cwd, "bounce"), "file literal", "absolute")
		// round-trip using content-addressible "warehouse"
		testutil.WithTmpdir(func() {
			os.Mkdir("bounce", 0755) // make the warehouse location
			tests.CheckRoundTrip(Kind, New, "file+ca://bounce", "content-addressable")
		})(nil)
	}))
}
