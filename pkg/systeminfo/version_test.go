package systeminfo

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

type VersionTestCase struct {
	version       string
	expectedIsDev bool
}

func name(in VersionTestCase) string {
	result := "Prod"
	if in.expectedIsDev {
		result = "Dev"
	}
	return fmt.Sprintf("%s is %s Version", in.version, result)
}

func TestVersionIsDevBuild(t *testing.T) {
	startVersion := coreRancherVersion

	testCases := []VersionTestCase{
		{version: "dev", expectedIsDev: true},
		{version: "2.13.2", expectedIsDev: false},
		{version: "v2.13.2", expectedIsDev: false},
		{version: "2.13.2+buildmeta.1", expectedIsDev: false},
		{version: "v2.13.2+buildmeta.1", expectedIsDev: false},
		{version: "2.13.2-rc.42", expectedIsDev: true},
		{version: "v2.13.2-rc.42", expectedIsDev: true},
		{version: "2.13.2+meta-with-hyphen", expectedIsDev: false},
		{version: "v2.13.2+meta-with-hyphen", expectedIsDev: false},
		{version: "2.13.2-rc.9999+meta-also", expectedIsDev: true},
		{version: "v2.13.2-rc.9999+meta-also", expectedIsDev: true},
		{version: "2.12-4f8fe4b5d-head", expectedIsDev: true},
		{version: "v2.12-4f8fe4b5d-head", expectedIsDev: true},
	}

	for _, tc := range testCases {
		t.Run(name(tc), func(t *testing.T) {
			// Test non-semVer versions used for dev builds
			coreRancherVersion = tc.version
			assert.Equal(t, tc.expectedIsDev, versionIsDevBuild())
			coreRancherVersion = startVersion
		})
	}
}
