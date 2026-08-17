package auth

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/azazo1/bilibili-cli/internal/api"
)

type Mode string

const (
	ModeOptional Mode = "optional"
	ModeRead     Mode = "read"
	ModeWrite    Mode = "write"
)

const credentialTTL = 7 * 24 * time.Hour

type Store struct {
	Client *api.Client
	Dir    string
	File   string
	Logger *slog.Logger
	Now    func() time.Time
}

func (s *Store) ensureDefaults() {
	if s.Logger == nil {
		s.Logger = slog.Default()
	}
	if s.Now == nil {
		s.Now = time.Now
	}
}

func NewStore(client *api.Client) *Store {
	dir := strings.TrimSpace(os.Getenv("BILI_CONFIG_DIR"))
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			home = "."
		}
		dir = filepath.Join(home, ".config", "bilibili-cli")
	}
	return &Store{
		Client: client,
		Dir:    dir,
		File:   filepath.Join(dir, "auth.json"),
		Logger: slog.Default(),
		Now:    time.Now,
	}
}

func (s *Store) LoadSaved() (*api.Credential, error) {
	data, err := os.ReadFile(s.File)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var credential api.Credential
	if err := json.Unmarshal(data, &credential); err != nil {
		return nil, err
	}
	if !credential.ValidForRead() {
		return nil, nil
	}
	return &credential, nil
}

func (s *Store) Save(credential *api.Credential) error {
	s.ensureDefaults()
	if credential == nil || !credential.ValidForRead() {
		return api.NewError(api.CodeInvalidInput, "保存凭证", "SESSDATA 不能为空")
	}
	if err := os.MkdirAll(s.Dir, 0o700); err != nil {
		return err
	}
	copy := *credential
	copy.SavedAt = float64(s.Now().Unix())
	data, err := json.MarshalIndent(&copy, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(s.File, data, 0o600); err != nil {
		return err
	}
	return os.Chmod(s.File, 0o600)
}

func (s *Store) Clear() error {
	err := os.Remove(s.File)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func (s *Store) IsStale() bool {
	s.ensureDefaults()
	credential, err := s.LoadSaved()
	if err != nil || credential == nil {
		return false
	}
	if credential.SavedAt <= 0 {
		return true
	}
	return s.Now().Unix()-int64(credential.SavedAt) > int64(credentialTTL.Seconds())
}

func (s *Store) GetCredential(ctx context.Context, mode Mode) (*api.Credential, error) {
	s.ensureDefaults()
	requireWrite := mode == ModeWrite
	saved, loadErr := s.LoadSaved()
	if loadErr != nil {
		s.Logger.Warn("读取凭证失败", "error", loadErr)
	}
	if saved != nil {
		if mode == ModeOptional {
			return saved, nil
		}
		if requireWrite && !saved.ValidForWrite() {
			s.Logger.Debug("保存的凭证缺少写权限")
		} else {
			if s.IsStale() {
				if fresh := s.extractBrowserCredential(); fresh != nil {
					valid, _ := s.validate(ctx, fresh, requireWrite)
					if valid {
						if err := s.Save(fresh); err != nil {
							s.Logger.Warn("保存浏览器凭证失败", "error", err)
						}
						return fresh, nil
					}
				}
			}
			valid, indeterminate := s.validate(ctx, saved, requireWrite)
			if valid || indeterminate {
				return saved, nil
			}
			_ = s.Clear()
		}
	}
	if mode == ModeOptional {
		return nil, nil
	}
	if browser := s.extractBrowserCredential(); browser != nil {
		valid, indeterminate := s.validate(ctx, browser, requireWrite)
		if valid || indeterminate {
			if err := s.Save(browser); err != nil {
				s.Logger.Warn("保存浏览器凭证失败", "error", err)
			}
			return browser, nil
		}
	}
	return nil, nil
}

func (s *Store) validate(ctx context.Context, credential *api.Credential, requireWrite bool) (valid bool, indeterminate bool) {
	if credential == nil || !credential.ValidForRead() {
		return false, false
	}
	if requireWrite && !credential.ValidForWrite() {
		return false, false
	}
	if s.Client == nil {
		return false, true
	}
	_, err := s.Client.GetSelfInfo(ctx, credential)
	if err == nil {
		return true, false
	}
	if api.IsNetwork(err) {
		return false, true
	}
	return false, false
}

func (s *Store) extractBrowserCredential() *api.Credential {
	if raw := strings.TrimSpace(os.Getenv("BILI_COOKIE")); raw != "" {
		return credentialFromCookies(parseCookieHeader(raw))
	}
	if path := strings.TrimSpace(os.Getenv("BILI_COOKIE_FILE")); path != "" {
		if cookies, err := readNetscapeCookies(path); err == nil {
			return credentialFromCookies(cookies)
		}
	}
	return nil
}

func parseCookieHeader(raw string) map[string]string {
	result := make(map[string]string)
	for _, part := range strings.Split(raw, ";") {
		pair := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(pair) != 2 {
			continue
		}
		result[pair[0]] = pair[1]
	}
	return result
}

func readNetscapeCookies(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	result := make(map[string]string)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) >= 7 && strings.Contains(fields[0], "bilibili.com") {
			result[fields[5]] = fields[6]
		}
	}
	return result, scanner.Err()
}

func credentialFromCookies(cookies map[string]string) *api.Credential {
	if strings.TrimSpace(cookies["SESSDATA"]) == "" {
		return nil
	}
	return &api.Credential{
		Sessdata:    cookies["SESSDATA"],
		BiliJct:     cookies["bili_jct"],
		AcTimeValue: cookies["ac_time_value"],
		Buvid3:      cookies["buvid3"],
		Buvid4:      cookies["buvid4"],
		DedeUserID:  cookies["DedeUserID"],
	}
}
