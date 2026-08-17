package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/azazo1/bilibili-cli/internal/api"
	"github.com/azazo1/bilibili-cli/internal/output"
)

func TestVideoDownloadCommandDownloadsBothStreamsInReadOnlyMode(t *testing.T) {
	server := newVideoDownloadTestServer(t)
	defer server.Close()

	app := newTestApp(t)
	app.API.BaseURL = server.URL
	app.API.HTTP = server.Client()
	app.Config.Safety.ReadOnly = true
	app.Out = &output.Writer{Stdout: io.Discard, Stderr: io.Discard, DefaultMode: "rich"}
	outDir := t.TempDir()
	root := NewRoot(app)
	root.SetArgs([]string{"video", "download", "BV1ABcsztEcY", "-o", outDir, "--no-merge"})
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"demo_audio.m4a", "demo_video.m4a"} {
		data, err := os.ReadFile(filepath.Join(outDir, name))
		if err != nil {
			t.Fatalf("missing %s: %v", name, err)
		}
		if len(data) == 0 {
			t.Fatalf("empty %s", name)
		}
	}
}

func TestVideoDownloadCommandRejectsConflictingStreamFlags(t *testing.T) {
	app := newTestApp(t)
	app.Out = &output.Writer{Stdout: io.Discard, Stderr: io.Discard, DefaultMode: "rich"}
	root := NewRoot(app)
	root.SetArgs([]string{"video", "download", "BV1ABcsztEcY", "--audio-only", "--video-only"})
	err := root.ExecuteContext(context.Background())
	var exitErr *ExitError
	if !errors.As(err, &exitErr) || exitErr.Code != 1 {
		t.Fatalf("conflicting flags returned: %v", err)
	}
	if api.CodeOf(err) != api.CodeInvalidInput {
		t.Fatalf("unexpected error code: %s", api.CodeOf(err))
	}
}

func TestVideoDownloadCommandHierarchy(t *testing.T) {
	root := NewRoot(newTestApp(t))
	command, _, err := root.Find([]string{"video", "download"})
	if err != nil || command == nil || command.Name() != "download" {
		t.Fatalf("download command not found: %v", err)
	}
	video, _, err := root.Find([]string{"video"})
	if err != nil {
		t.Fatal(err)
	}
	for _, child := range video.Commands() {
		if child.Name() == "audio" {
			t.Fatal("legacy audio command is still registered")
		}
	}
}

func TestReportVideoPagesRequiresExplicitSelection(t *testing.T) {
	app := newTestApp(t)
	app.Out = &output.Writer{Stdout: io.Discard, Stderr: io.Discard, DefaultMode: "rich"}
	err := reportVideoPages(app, "BV1ABcsztEcY", []api.VideoPage{{Page: 1, Title: "first"}, {Page: 2, Title: "second"}}, output.ModeRich)
	if api.CodeOf(err) != api.CodeInvalidInput {
		t.Fatalf("unexpected page selection error: %s", api.CodeOf(err))
	}
}

func TestVideoDownloadCommandDoesNotDownloadFirstPageForMultiPartVideo(t *testing.T) {
	mediaRequests := 0
	playURLRequests := 0
	infoRequests := 0
	var serverURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/x/web-interface/view":
			infoRequests++
			fmt.Fprint(w, `{"code":0,"data":{"title":"demo","duration":60}}`)
		case "/x/player/pagelist":
			fmt.Fprint(w, `{"code":0,"data":[{"cid":41,"part":"first"},{"cid":42,"part":"second"}]}`)
		case "/x/frontend/finger/spi":
			fmt.Fprint(w, `{"code":0,"data":{"b_3":"device3","b_4":"device4"}}`)
		case "/x/internal/gaia-gateway/ExClimbWuzhi":
			fmt.Fprint(w, `{"code":0}`)
		case "/x/web-interface/nav":
			fmt.Fprintf(w, `{"code":0,"data":{"wbi_img":{"img_url":"%s/wbi/0123456789abcdef0123456789abcdef.png","sub_url":"%s/wbi/fedcba9876543210fedcba9876543210.png"}}}`, serverURL, serverURL)
		case "/x/player/wbi/playurl":
			playURLRequests++
			fmt.Fprint(w, `{"code":0,"data":{"dash":{"audio":[{"base_url":"`+serverURL+`/audio.m4s"}],"video":[{"base_url":"`+serverURL+`/video.m4s"}]}}}`)
		case "/audio.m4s", "/video.m4s":
			mediaRequests++
			_, _ = w.Write([]byte("media"))
		default:
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
	}))
	defer server.Close()
	serverURL = server.URL

	app := newTestApp(t)
	app.API.BaseURL = server.URL
	app.API.HTTP = server.Client()
	app.Out = &output.Writer{Stdout: io.Discard, Stderr: io.Discard, DefaultMode: "rich"}
	root := NewRoot(app)
	root.SetArgs([]string{"video", "download", "BV1ABcsztEcY", "-o", t.TempDir()})
	err := root.ExecuteContext(context.Background())
	if api.CodeOf(err) != api.CodeInvalidInput {
		t.Fatalf("unexpected multi part error: %s", api.CodeOf(err))
	}
	if mediaRequests != 0 {
		t.Fatalf("downloaded media requests: %d", mediaRequests)
	}
	if playURLRequests != 0 {
		t.Fatalf("requested first page play URL: %d", playURLRequests)
	}
	if infoRequests != 0 {
		t.Fatalf("requested video info before page selection: %d", infoRequests)
	}
}

func newVideoDownloadTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	var serverURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/x/web-interface/view":
			fmt.Fprint(w, `{"code":0,"data":{"title":"demo","duration":60}}`)
		case "/x/player/pagelist":
			fmt.Fprint(w, `{"code":0,"data":[{"cid":42}]}`)
		case "/x/frontend/finger/spi":
			fmt.Fprint(w, `{"code":0,"data":{"b_3":"device3","b_4":"device4"}}`)
		case "/x/internal/gaia-gateway/ExClimbWuzhi":
			fmt.Fprint(w, `{"code":0}`)
		case "/x/web-interface/nav":
			fmt.Fprintf(w, `{"code":0,"data":{"wbi_img":{"img_url":"%s/wbi/0123456789abcdef0123456789abcdef.png","sub_url":"%s/wbi/fedcba9876543210fedcba9876543210.png"}}}`, serverURL, serverURL)
		case "/x/player/wbi/playurl":
			fmt.Fprint(w, `{"code":0,"data":{"dash":{"audio":[{"base_url":"`+serverURL+`/audio.m4s"}],"video":[{"base_url":"`+serverURL+`/video.m4s"}]}}}`)
		case "/audio.m4s":
			_, _ = w.Write([]byte("audio"))
		case "/video.m4s":
			_, _ = w.Write([]byte("video"))
		default:
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
	}))
	serverURL = server.URL
	return server
}

func TestMakeVideoDownloadItemsUsesNativeMP4WhenSeparateStreamsAreMissing(t *testing.T) {
	items, err := makeVideoDownloadItems(api.VideoDownloadURLs{CombinedURL: "https://cdn.example/video.mp4"}, "demo", ".", false, false)
	if err != nil || len(items) != 1 || items[0].path != "demo.mp4" {
		t.Fatalf("unexpected native MP4 plan: %#v, %v", items, err)
	}
}

func TestMakeVideoDownloadItemsHonorsStreamFlags(t *testing.T) {
	urls := api.VideoDownloadURLs{
		AudioURL: "https://cdn.example/audio.m4s",
		VideoURL: "https://cdn.example/video.m4s",
	}
	audioItems, err := makeVideoDownloadItems(urls, "demo", ".", true, false)
	if err != nil || len(audioItems) != 1 || audioItems[0].path != "demo_audio.m4a" {
		t.Fatalf("unexpected audio plan: %#v, %v", audioItems, err)
	}
	videoItems, err := makeVideoDownloadItems(urls, "demo", ".", false, true)
	if err != nil || len(videoItems) != 1 || videoItems[0].path != "demo_video.m4a" {
		t.Fatalf("unexpected video plan: %#v, %v", videoItems, err)
	}
}

func TestVideoDownloadFileTitleIncludesPageAndPart(t *testing.T) {
	title := videoDownloadFileTitle("demo", 2, 3, "第二集")
	if title != "demo_P02_第二集" {
		t.Fatalf("unexpected page title: %q", title)
	}
	if got := videoDownloadFileTitle("demo", 1, 1, "demo"); got != "demo" {
		t.Fatalf("unexpected single page title: %q", got)
	}
	longTitle := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if got := videoDownloadFileTitle(longTitle, 2, 3, "second"); len([]rune(got)) > 120 || got[len(got)-len("_P02_second"):] != "_P02_second" {
		t.Fatalf("page suffix was truncated: %q", got)
	}
}

func TestSeparateStreamPathsExcludesCombinedVideo(t *testing.T) {
	items := []videoDownloadItem{
		{stream: "audio", path: "demo_audio.m4a"},
		{stream: "video", path: "demo_video.m4a"},
	}
	audioPath, videoPath := separateStreamPaths(items)
	if audioPath != "demo_audio.m4a" || videoPath != "demo_video.m4a" {
		t.Fatalf("unexpected separate paths: %q, %q", audioPath, videoPath)
	}
	audioPath, videoPath = separateStreamPaths([]videoDownloadItem{{stream: "combined", path: "demo.mp4"}})
	if audioPath != "" || videoPath != "" {
		t.Fatalf("combined path was treated as separate streams: %q, %q", audioPath, videoPath)
	}
}
