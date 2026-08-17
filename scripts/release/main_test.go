package main

import (
	"path/filepath"
	"testing"
)

func TestFormatBuildVersion(t *testing.T) {
	tests := []struct {
		name     string
		explicit string
		exact    string
		nearest  string
		commit   string
		dirty    bool
		want     string
	}{
		{
			name:   "exact tag",
			exact:  "v1.2.3",
			nearest: "v1.2.3",
			commit: "a1b2c3d",
			want:   "v1.2.3",
		},
		{
			name:    "non tag commit",
			nearest: "v1.2.3",
			commit:  "a1b2c3d",
			want:    "v1.2.3-a1b2c3d",
		},
		{
			name:    "dirty worktree",
			nearest: "v1.2.3",
			commit:  "a1b2c3d",
			dirty:   true,
			want:    "v1.2.3^a1b2c3d",
		},
		{
			name:     "explicit release tag",
			explicit: "v2.0.0-rc.1",
			nearest:  "v1.2.3",
			commit:   "a1b2c3d",
			want:     "v2.0.0-rc.1",
		},
		{
			name:   "no release tag",
			commit: "a1b2c3d",
			want:   "devel-a1b2c3d",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := formatBuildVersion(test.explicit, test.exact, test.nearest, test.commit, test.dirty)
			if got != test.want {
				t.Fatalf("formatBuildVersion() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestVersionTagPattern(t *testing.T) {
	for _, tag := range []string{"v0.1.0", "v1.2.3-rc.1", "v2.0.0+build.7"} {
		if !versionTagPattern.MatchString(tag) {
			t.Fatalf("expected valid release tag: %s", tag)
		}
	}
	for _, tag := range []string{"1.2.3", "v01.2.3", "v1.2", "v1.2.3-rc.", "v1.2.3+build..7"} {
		if versionTagPattern.MatchString(tag) {
			t.Fatalf("expected invalid release tag: %s", tag)
		}
	}
}

func TestResolveTargetBuildsExpectedArchivePaths(t *testing.T) {
	tests := []struct {
		goos        string
		goarch      string
		binaryPath  string
		archivePath string
	}{
		{
			goos:        "linux",
			goarch:      "amd64",
			binaryPath:  filepath.Join("bin", "bili"),
			archivePath: filepath.Join("dist", "bilibili-cli-0.1.0-linux-x86_64.tar.gz"),
		},
		{
			goos:        "windows",
			goarch:      "arm64",
			binaryPath:  filepath.Join("bin", "bili.exe"),
			archivePath: filepath.Join("dist", "bilibili-cli-0.1.0-windows-aarch64.zip"),
		},
		{
			goos:        "darwin",
			goarch:      "arm64",
			binaryPath:  filepath.Join("bin", "bili"),
			archivePath: filepath.Join("dist", "bilibili-cli-0.1.0-macos-aarch64.tar.gz"),
		},
	}

	for _, test := range tests {
		t.Run(test.goos+"/"+test.goarch, func(t *testing.T) {
			target, err := resolveTarget(test.goos, test.goarch)
			if err != nil {
				t.Fatal(err)
			}
			metadata, err := buildMetadataForTest(target, "v0.1.0")
			if err != nil {
				t.Fatal(err)
			}
			if metadata.BinaryPath != test.binaryPath {
				t.Fatalf("binary path = %q, want %q", metadata.BinaryPath, test.binaryPath)
			}
			if metadata.ArchivePath != test.archivePath {
				t.Fatalf("archive path = %q, want %q", metadata.ArchivePath, test.archivePath)
			}
		})
	}
}

func buildMetadataForTest(target target, displayVersion string) (buildMetadata, error) {
	return buildMetadataForVersion(displayVersion, "", target)
}
