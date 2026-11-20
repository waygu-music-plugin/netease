package netease

import (
	"bytes"
	"crypto/aes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"testing"
)

type streamDigestResponse string
type streamTransport func(*http.Request) (*http.Response, error)

func (f streamTransport) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func (body streamDigestResponse) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Path != "/eapi/song/enhance/player/url/v1" && req.URL.Path != "/eapi/song/enhance/player/url" {
		return nil, fmt.Errorf("unexpected request path: %s", req.URL.Path)
	}
	return &http.Response{
		StatusCode: http.StatusOK, Header: make(http.Header),
		Body: io.NopCloser(strings.NewReader(string(body))), Request: req,
	}, nil
}

type streamParameters struct {
	IDs, Level, EncodeType string
	BR                     int
	Header                 map[string]string
}

func readStreamParameters(req *http.Request) (streamParameters, error) {
	var params streamParameters
	if err := req.ParseForm(); err != nil {
		return params, err
	}
	data, err := hex.DecodeString(req.Form.Get("params"))
	if err != nil || len(data) == 0 || len(data)%aes.BlockSize != 0 {
		return params, errors.New("invalid encrypted request")
	}
	block, _ := aes.NewCipher([]byte(eapiKey))
	for i := 0; i < len(data); i += aes.BlockSize {
		block.Decrypt(data[i:i+aes.BlockSize], data[i:i+aes.BlockSize])
	}
	parts := bytes.SplitN(data, []byte("-36cd479b6b5-"), 3)
	if len(parts) != 3 || json.Unmarshal(parts[1], &params) != nil {
		return params, errors.New("invalid song request")
	}
	return params, nil
}

// When higher quality returns no source, the resolver asks the same service for standard audio.
func TestSongURLFallsBackToStandard(t *testing.T) {
	original := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = original })
	var requests []string
	http.DefaultTransport = streamTransport(func(req *http.Request) (*http.Response, error) {
		params, err := readStreamParameters(req)
		if err != nil {
			return nil, err
		}
		if params.IDs != "[17]" || req.URL.Path != "/eapi/song/enhance/player/url/v1" {
			return nil, errors.New("invalid song request")
		}
		requests = append(requests, params.Level+"/"+params.EncodeType)
		var body streamDigestResponse = `{"code":200,"data":[{"id":17,"code":404,"url":null}]}`
		if params.Level == "standard" {
			body = `{"code":200,"data":[{"id":11,"code":200,"url":"https://example.test/other.mp3"},{"id":17,"code":200,"url":"http://example.test/record.mp3","type":"mp3","md5":"900150983cd24fb0d6963f7d28e17f72"}]}`
		}
		return body.RoundTrip(req)
	})
	stream, err := SongURL(t.Context(), 17, "lossless")
	if err != nil || stream == nil || stream.URL != "https://example.test/record.mp3" || stream.Level != "standard" ||
		stream.Digest == nil || !stream.Digest.Matches(strings.NewReader("abc")) {
		t.Fatalf("fallback returned %+v, %v", stream, err)
	}
	if got := strings.Join(requests, ","); got != "lossless/flac,standard/mp3" {
		t.Fatalf("unexpected quality requests: %s", got)
	}
}

// A temporary v1 refusal retries the same request before using the bitrate endpoint with the same session.
func TestSongURLRecoversFromRequestRefusal(t *testing.T) {
	originalTransport, originalJar := http.DefaultTransport, httpClient.Jar
	t.Cleanup(func() { http.DefaultTransport, httpClient.Jar = originalTransport, originalJar })
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	jar.SetCookies(siteURL, []*http.Cookie{{Name: "MUSIC_U", Value: "fixture-session"}})
	httpClient.Jar = jar
	for _, tt := range []struct {
		name         string
		refusals     int
		wantRequests string
	}{
		{"retry succeeds", 1, "v1,v1"},
		{"legacy serves full audio", 2, "v1,v1,legacy"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var requests []string
			http.DefaultTransport = streamTransport(func(req *http.Request) (*http.Response, error) {
				params, err := readStreamParameters(req)
				if err != nil {
					return nil, err
				}
				if params.IDs != "[17]" || params.Header["MUSIC_U"] != "fixture-session" ||
					!strings.Contains(req.Header.Get("Cookie"), "MUSIC_U=fixture-session") {
					return nil, errors.New("stream request did not retain the song and session")
				}
				switch req.URL.Path {
				case "/eapi/song/enhance/player/url/v1":
					if params.Level != "hires" || params.EncodeType != "flac" || params.BR != 0 {
						return nil, errors.New("v1 did not ask for the requested quality")
					}
					requests = append(requests, "v1")
					if len(requests) <= tt.refusals {
						return streamDigestResponse(`{"code":-460}`).RoundTrip(req)
					}
				case "/eapi/song/enhance/player/url":
					if params.BR != 999000 || params.Level != "" || params.EncodeType != "" {
						return nil, errors.New("legacy request did not use the equivalent bitrate")
					}
					requests = append(requests, "legacy")
				default:
					return nil, fmt.Errorf("unexpected playback endpoint: %s", req.URL.Path)
				}
				return streamDigestResponse(`{"code":200,"data":[{"id":17,"code":200,"url":"https://example.test/record.mp3","type":"mp3","br":320000,"md5":"900150983cd24fb0d6963f7d28e17f72"}]}`).RoundTrip(req)
			})
			stream, err := SongURL(t.Context(), 17, "hires")
			if err != nil || stream == nil || stream.URL != "https://example.test/record.mp3" ||
				stream.Digest == nil || !stream.Digest.Matches(strings.NewReader("abc")) {
				t.Fatalf("recovery returned %+v, %v", stream, err)
			}
			if strings.Contains(tt.wantRequests, "legacy") && stream.Level != "exhigh" {
				t.Fatalf("legacy MP3 advertised the wrong quality: %s", stream.Level)
			}
			headers := stream.Header()
			if headers.Get("Referer") != apiBase+"/" || headers.Get("User-Agent") == "" || headers.Get("Cookie") != "" {
				t.Fatal("audio requests require the service referer and no account cookies")
			}
			if got := strings.Join(requests, ","); got != tt.wantRequests {
				t.Fatalf("unexpected request sequence: %s", got)
			}
		})
	}
}

// An explicit refusal keeps its reason and a standard preview cannot become a full stream.
func TestSongURLReportsWhyAStreamWasRefused(t *testing.T) {
	original := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = original })
	for _, tt := range []struct {
		name, level string
		body        streamDigestResponse
		legacy      streamDigestResponse
		want        error
	}{
		{"login", "exhigh", `{"code":301}`, "", ErrLoginRequired},
		{"network", "exhigh", `{"code":-460}`, `{"code":-460}`, ErrNetworkRestricted},
		{"preview", "standard", previewResponse, "", ErrPreviewOnly},
		{"legacy preview", "standard", `{"code":-460}`, previewResponse, ErrPreviewOnly},
	} {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			http.DefaultTransport = streamTransport(func(req *http.Request) (*http.Response, error) {
				calls++
				if tt.legacy != "" && req.URL.Path == "/eapi/song/enhance/player/url" {
					return tt.legacy.RoundTrip(req)
				}
				return tt.body.RoundTrip(req)
			})
			stream, err := SongURL(t.Context(), 17, tt.level)
			wantCalls := 1
			if tt.legacy != "" {
				wantCalls = 3
			}
			if stream != nil || !errors.Is(err, tt.want) || calls != wantCalls {
				t.Fatalf("refused request returned %+v, %v after %d calls", stream, err, calls)
			}
		})
	}
}

const previewResponse = `{"code":200,"data":[{"id":17,"code":200,"url":"https://example.test/preview.mp3","freeTrialInfo":{"start":0,"end":30}}]}`

// The stream resolver checks a cached file against the digest of the selected audio.
func TestSongURLUsesContentDigest(t *testing.T) {
	original := http.DefaultTransport
	defer func() { http.DefaultTransport = original }()
	http.DefaultTransport = streamDigestResponse(`{"code":200,"data":[{"id":17,"url":"http://example.test/record.mp3","type":"mp3","level":"exhigh","size":3,"md5":"900150983cd24fb0d6963f7d28e17f72"}]}`)
	stream, err := SongURL(t.Context(), 17, "exhigh")
	if err != nil {
		t.Fatal(err)
	}
	if stream.VerifyCachedContent == nil || !stream.VerifyCachedContent(strings.NewReader("abc")) ||
		stream.VerifyCachedContent(strings.NewReader("abd")) {
		t.Fatal("resolved content check did not distinguish equal-sized files")
	}
	http.DefaultTransport = streamDigestResponse(`{"code":200,"data":[{"id":17,"url":"http://example.test/record.mp3","type":"mp3","level":"exhigh","size":3,"md5":"incorrect"}]}`)
	stream, err = SongURL(t.Context(), 17, "exhigh")
	if err != nil || stream.VerifyCachedContent != nil {
		t.Fatalf("invalid digest changed the stream request: %v", err)
	}
}
