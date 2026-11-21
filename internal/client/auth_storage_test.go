package netease

import (
	"errors"
	"net/http"
	"net/http/cookiejar"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo"
)

// A NetEase login from an older build restores its cookies and sign-out deletes the old record.
func TestNetEaseRestoresLegacySessionCookies(t *testing.T) {
	t.Setenv("MYGO_ENV", "development")
	dir := t.TempDir()
	mygo.App.SetPath(mygo.PathUserData, dir)
	defer mygo.App.SetPath(mygo.PathUserData, "")
	path := filepath.Join(dir, "session")
	if err := os.WriteFile(path, []byte("MUSIC_U=music-session; __csrf=csrf-token"), 0o600); err != nil {
		t.Fatal(err)
	}
	oldJar := httpClient.Jar
	defer func() { httpClient.Jar = oldJar }()
	httpClient.Jar, _ = cookiejar.New(nil)
	if !SessionConfigured() || !RestoreSession() {
		t.Fatal("legacy NetEase session was not restored")
	}
	cookies := make(map[string]string)
	for _, cookie := range httpClient.Jar.Cookies(siteURL) {
		cookies[cookie.Name] = cookie.Value
	}
	if cookies["MUSIC_U"] != "music-session" || cookies["__csrf"] != "csrf-token" {
		t.Fatal("restored jar did not contain both login cookies")
	}
	ClearSession()
	if SessionConfigured() || len(httpClient.Jar.Cookies(siteURL)) != 0 {
		t.Fatal("sign-out retained the NetEase session")
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("legacy file after sign-out: %v", err)
	}
}

// A CSRF cookie alone cannot replace a previously saved login session.
func TestSaveSessionRequiresLoginCookie(t *testing.T) {
	t.Setenv("MYGO_ENV", "development")
	dir := t.TempDir()
	mygo.App.SetPath(mygo.PathUserData, dir)
	defer mygo.App.SetPath(mygo.PathUserData, "")
	path := filepath.Join(dir, "session")
	const saved = "MUSIC_U=verified-session"
	if err := os.WriteFile(path, []byte(saved), 0o600); err != nil {
		t.Fatal(err)
	}

	oldJar := httpClient.Jar
	defer func() { httpClient.Jar = oldJar }()
	httpClient.Jar, _ = cookiejar.New(nil)
	httpClient.Jar.SetCookies(siteURL, []*http.Cookie{{Name: "__csrf", Value: "csrf-only", Path: "/"}})
	if err := SaveSession(); err == nil {
		t.Fatal("saved a session without a login cookie")
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != saved {
		t.Fatalf("previous session was replaced: %v", err)
	}
}
