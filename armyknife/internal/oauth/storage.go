package oauth

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"

	"golang.org/x/oauth2"
)

func restoreToken(directory string, scope string) *oauth2.Token {
	b, err := os.ReadFile(filepath.Join(directory, url.PathEscape(scope)))
	if err != nil {
		return nil
	}
	var token oauth2.Token
	if err := json.Unmarshal(b, &token); err != nil {
		return nil
	}
	return &token
}

func saveToken(directory string, scope string, token *oauth2.Token) error {
	b, err := json.Marshal(token)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(directory, 0700); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(directory, url.PathEscape(scope)), b, 0600)
}

func deviceDirectory() string {
	return filepath.Join(oauthDirectory(), "device")
}

func pkceDirectory() string {
	return filepath.Join(oauthDirectory(), "pkce")
}

func oauthDirectory() string {
	home := os.Getenv("XDG_DATA_HOME")
	if home == "" {
		u, err := os.UserHomeDir()
		if err != nil {
			return os.TempDir()
		}
		home = filepath.Join(u, ".local", "share")
	}
	return filepath.Join(home, "oauth")
}
