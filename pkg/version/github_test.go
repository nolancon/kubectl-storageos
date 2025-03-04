package version

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestFetchVersionsOrPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("panic not allowed: %v", r)
		}
	}()

	fakeUrl, close := startGithubServerMock(t)
	defer close()

	releases := fetchVersionsOrPanic(fakeUrl)

	if len(releases) != 31 {
		t.Error("not all the releases were parsed")
	}
}

func startGithubServerMock(t *testing.T) (string, func()) {
	ms := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		releases, err := os.ReadFile("test-data/cluster-operator-releases.json")
		if err != nil {
			t.Fatalf("failed to read testdata: %s", err.Error())
		}
		fmt.Fprintln(w, string(releases))
	}))

	return ms.URL, ms.Close
}

func TestSelectLatestVersionEnablePreReleasesFalse(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("panic not allowed: %v", r)
		}
	}()

	rawVersions, err := os.ReadFile("test-data/cluster-operator-releases.json")
	if err != nil {
		t.Fatalf("failed to read testdata: %s", err.Error())
	}

	releases := []GithubRelease{}
	err = json.Unmarshal(rawVersions, &releases)
	if err != nil {
		t.Fatalf("failed to parse testdata: %s", err.Error())
	}

	latest := selectLatestVersionOrPanic(releases)

	if latest != "v2.4.4" {
		t.Errorf("latest version doesn't match: v2.4.4 != %s", latest)
	}
}

func TestSelectLatestVersionEnablePreReleasesTrue(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("panic not allowed: %v", r)
		}
	}()

	enablePreReleases = true
	defer func() {
		enablePreReleases = false
	}()

	rawVersions, err := os.ReadFile("test-data/cluster-operator-releases.json")
	if err != nil {
		t.Fatalf("failed to read testdata: %s", err.Error())
	}

	releases := []GithubRelease{}
	err = json.Unmarshal(rawVersions, &releases)
	if err != nil {
		t.Fatalf("failed to parse testdata: %s", err.Error())
	}

	latest := selectLatestVersionOrPanic(releases)

	if latest != "v2.4.4" {
		t.Errorf("latest version doesn't match: v2.4.4 != %s", latest)
	}
}
