package api

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestResolveImageReferenceRecognizesSupportedInputs(t *testing.T) {
	client := NewClient()
	cases := []struct {
		input string
		kind  ImageKind
		id    string
	}{
		{input: "BV1ABcsztEcY", kind: ImageKindVideo, id: "BV1ABcsztEcY"},
		{input: "cv42", kind: ImageKindArticle, id: "cv42"},
		{input: "ss62", kind: ImageKindBangumi, id: "ss62"},
		{input: "ep374717", kind: ImageKindBangumi, id: "ep374717"},
		{input: "md28231846", kind: ImageKindMedia, id: "md28231846"},
		{input: "https://space.bilibili.com/42", kind: ImageKindUser, id: "42"},
		{input: "https://live.bilibili.com/5440", kind: ImageKindLive, id: "5440"},
		{input: "https://www.bilibili.com/read/cv42", kind: ImageKindArticle, id: "cv42"},
		{input: "https://www.bilibili.com/bangumi/play/ep374717", kind: ImageKindBangumi, id: "ep374717"},
		{input: "123", kind: ImageKindUser, id: "123"},
		{input: "demo-up", kind: ImageKindUser, id: "demo-up"},
	}
	for _, testCase := range cases {
		reference, err := client.ResolveImageReference(context.Background(), testCase.input)
		if err != nil || reference.Kind != testCase.kind || reference.ID != testCase.id {
			t.Fatalf("ResolveImageReference(%q) = %#v, %v", testCase.input, reference, err)
		}
	}
	if _, err := client.ResolveImageReference(context.Background(), "https://example.com/video/BV1ABcsztEcY"); CodeOf(err) != CodeInvalidInput {
		t.Fatalf("unexpected unsupported URL error: %v", err)
	}
	if reference, err := client.ResolveImageReferenceAs(context.Background(), ImageKindLive, "5440"); err != nil || reference.ID != "5440" {
		t.Fatalf("ResolveImageReferenceAs() = %#v, %v", reference, err)
	}
	if _, err := client.ResolveImageReferenceAs(context.Background(), ImageKindLive, "https://www.bilibili.com/video/BV1ABcsztEcY"); CodeOf(err) != CodeInvalidInput {
		t.Fatalf("unexpected typed URL error: %v", err)
	}
}

func TestResolveImageReferenceFollowsB23Redirect(t *testing.T) {
	client := NewClient()
	client.HTTP = &http.Client{Transport: imageRoundTripper(func(request *http.Request) (*http.Response, error) {
		response := &http.Response{
			Header:  make(http.Header),
			Body:    io.NopCloser(strings.NewReader("")),
			Request: request,
		}
		switch request.URL.Host {
		case "b23.tv":
			response.StatusCode = http.StatusFound
			response.Header.Set("Location", "https://www.bilibili.com/video/BV1ABcsztEcY")
		case "www.bilibili.com":
			response.StatusCode = http.StatusOK
		default:
			t.Fatalf("unexpected host: %s", request.URL.Host)
		}
		return response, nil
	})}
	reference, err := client.ResolveImageReference(context.Background(), "https://b23.tv/demo")
	if err != nil || reference.Kind != ImageKindVideo || reference.ID != "BV1ABcsztEcY" {
		t.Fatalf("ResolveImageReference() = %#v, %v", reference, err)
	}
}

func TestGetImageTargetSupportsAllKinds(t *testing.T) {
	var serverURL string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/x/space/wbi/acc/info":
			writer.Write([]byte(`{"code":0,"data":{"mid":42,"name":"demo user","face":"` + serverURL + `/avatar.jpg"}}`))
		case "/x/web-interface/view":
			writer.Write([]byte(`{"code":0,"data":{"title":"demo video","pic":"` + serverURL + `/video.webp","owner":{"mid":42,"face":"` + serverURL + `/video-avatar.png"}}}`))
		case "/x/article/viewinfo":
			writer.Write([]byte(`{"code":0,"data":{"title":"demo article","mid":42,"image_urls":["` + serverURL + `/article.jpg"]}}`))
		case "/pgc/view/web/season":
			writer.Write([]byte(`{"code":0,"result":{"season_id":62,"title":"demo bangumi","cover":"` + serverURL + `/bangumi.png","up_info":{"avatar":"` + serverURL + `/bangumi-avatar.jpg"}}}`))
		case "/pgc/review/user":
			writer.Write([]byte(`{"code":0,"result":{"media":{"title":"demo media","cover":"` + serverURL + `/media.webp","season_id":62}}}`))
		case "/room/v1/Room/get_info":
			writer.Write([]byte(`{"code":0,"data":{"room_id":5440,"uid":42,"title":"demo live","user_cover":"` + serverURL + `/live.jpg"}}`))
		default:
			t.Fatalf("unexpected request path: %s", request.URL.Path)
		}
	}))
	defer server.Close()
	serverURL = server.URL

	client := NewClient()
	client.BaseURL = server.URL
	client.LiveBaseURL = server.URL
	client.HTTP = server.Client()
	client.device = &Credential{Buvid3: "b3", Buvid4: "b4"}
	client.deviceExpires = time.Now().Add(time.Hour)
	client.wbiKey = strings.Repeat("a", 32)
	client.wbiExpires = time.Now().Add(time.Hour)
	client.webIDs = map[int64]webIDEntry{42: {Value: "web-id", Expiry: time.Now().Add(time.Hour)}}

	cases := []struct {
		reference ImageReference
		role      ImageAssetRole
		url       string
	}{
		{reference: ImageReference{Kind: ImageKindUser, ID: "42"}, role: ImageAssetAvatar, url: serverURL + "/avatar.jpg"},
		{reference: ImageReference{Kind: ImageKindVideo, ID: "BV1ABcsztEcY"}, role: ImageAssetCover, url: serverURL + "/video.webp"},
		{reference: ImageReference{Kind: ImageKindArticle, ID: "cv7"}, role: ImageAssetCover, url: serverURL + "/article.jpg"},
		{reference: ImageReference{Kind: ImageKindBangumi, ID: "ss62"}, role: ImageAssetCover, url: serverURL + "/bangumi.png"},
		{reference: ImageReference{Kind: ImageKindMedia, ID: "md8"}, role: ImageAssetCover, url: serverURL + "/media.webp"},
		{reference: ImageReference{Kind: ImageKindLive, ID: "1"}, role: ImageAssetCover, url: serverURL + "/live.jpg"},
	}
	for _, testCase := range cases {
		target, err := client.GetImageTarget(context.Background(), testCase.reference, nil)
		if err != nil || target.ImageRole != testCase.role || target.ImageURL != testCase.url {
			t.Fatalf("GetImageTarget(%#v) = %#v, %v", testCase.reference, target, err)
		}
	}
	article, err := client.GetImageTarget(context.Background(), ImageReference{Kind: ImageKindArticle, ID: "cv7"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if avatar, err := client.GetImageAvatar(context.Background(), article, nil); err != nil || avatar != serverURL+"/avatar.jpg" {
		t.Fatalf("GetImageAvatar(article) = %q, %v", avatar, err)
	}
	media, err := client.GetImageTarget(context.Background(), ImageReference{Kind: ImageKindMedia, ID: "md8"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if avatar, err := client.GetImageAvatar(context.Background(), media, nil); err != nil || avatar != serverURL+"/bangumi-avatar.jpg" {
		t.Fatalf("GetImageAvatar(media) = %q, %v", avatar, err)
	}
	live, err := client.GetImageTarget(context.Background(), ImageReference{Kind: ImageKindLive, ID: "1"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if avatar, err := client.GetImageAvatar(context.Background(), live, nil); err != nil || avatar != serverURL+"/avatar.jpg" {
		t.Fatalf("GetImageAvatar(live) = %q, %v", avatar, err)
	}
}

func TestClientDecodesResultEnvelope(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Write([]byte(`{"code":0,"result":{"value":"ok"}}`))
	}))
	defer server.Close()
	client := NewClient()
	client.BaseURL = server.URL
	client.HTTP = server.Client()
	var data map[string]any
	if err := client.Request(context.Background(), http.MethodGet, "/result", nil, nil, nil, &data); err != nil {
		t.Fatal(err)
	}
	if stringValue(data["value"]) != "ok" {
		t.Fatalf("result envelope was not decoded: %#v", data)
	}
}

type imageRoundTripper func(*http.Request) (*http.Response, error)

func (f imageRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}
