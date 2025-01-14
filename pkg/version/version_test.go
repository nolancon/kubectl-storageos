package version

import (
	"regexp"
	"testing"

	"github.com/storageos/kubectl-storageos/pkg/consts"
	"github.com/stretchr/testify/require"
)

func TestRegexIsValid(t *testing.T) {
	t.Parallel()

	_, err := regexp.Compile(consts.VersionRegex)
	require.NoError(t, err, "version regex is not valid")

	_, err = regexp.Compile(consts.ShaVersionRegex)
	require.NoError(t, err, "sha version regex is not valid")
}

func TestIsDevelop(t *testing.T) {
	tests := map[string]struct {
		version  string
		expected bool
	}{
		"semver": {
			version: "1.17.3",
		},
		"develop": {
			version:  "develop",
			expected: true,
		},
		"test": {
			version:  "test",
			expected: true,
		},
		"sha": {
			version:  "24582d9a8f60c7f6d3ce7eea6833281f548826eacdd75122d842231bdf1fe89e",
			expected: true,
		},
		"notsha": {
			version: "24582d9a8f60c7f6d3ce7eea6833281f548826eacdd75122d842231bdf1fe89G",
		},
	}

	for name, test := range tests {
		tt := test

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			actual := IsDevelop(tt.version)

			if tt.expected != actual {
				t.Errorf("is develop value doesn't match: %t != %t", tt.expected, actual)
			}
		})
	}
}

func TestCleanupVersion(t *testing.T) {
	tests := map[string]struct {
		input    string
		expected string
	}{
		"already cleaned": {
			input:    "v21.42.43",
			expected: "v21.42.43",
		},
		"already cleaned versionless": {
			input:    "21.42.43",
			expected: "21.42.43",
		},
		"has postfix": {
			input:    "v21.42.43-alpha1",
			expected: "v21.42.43",
		},
	}

	for name, test := range tests {
		tt := test

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			actual := cleanupVersion(tt.input)

			if tt.expected != actual {
				t.Errorf("cleaned version doesn't match: %s != %s", tt.expected, actual)
			}
		})
	}
}

func TestIsSupported(t *testing.T) {
	tests := map[string]struct {
		haveVersion string
		wantVersion string
		expected    bool
	}{
		"less than": {
			haveVersion: "1.17.3",
			wantVersion: "1.18.0",
		},
		"greater than": {
			haveVersion: "1.18.3",
			wantVersion: "1.18.0",
			expected:    true,
		},
		"greater than prefixed": {
			haveVersion: "v1.18.3",
			wantVersion: "v1.18.0",
			expected:    true,
		},
		"greater than postfixed": {
			haveVersion: "1.18.3-gke.301",
			wantVersion: "1.18.0#tag",
			expected:    true,
		},
	}

	for name, test := range tests {
		tt := test

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			actual, err := IsSupported(tt.haveVersion, tt.wantVersion)

			if err != nil {
				t.Errorf("error not allowed: %s", err.Error())
			}

			if tt.expected != actual {
				t.Errorf("supported value doesn't match: %t != %t", tt.expected, actual)
			}
		})
	}
}
