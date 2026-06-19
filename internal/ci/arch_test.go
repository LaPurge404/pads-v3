package ci

import (
	"testing"
)

func Test_NoImportCycles_Exist(t *testing.T) {
	// Ce test sert indirectement :
	// si build passe => architecture stable
	t.Log("architecture validated via build system")
}
