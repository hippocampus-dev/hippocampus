package egosearch

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"
	"golang.org/x/xerrors"
)

type SlackResponse struct {
	Ok       bool   `json:"ok"`
	Query    string `json:"query"`
	Messages struct {
		Total      int `json:"total"`
		Pagination struct {
			TotalCount int `json:"total_count"`
			Page       int `json:"page"`
			PerPage    int `json:"per_page"`
			PageCount  int `json:"page_count"`
			First      int `json:"first"`
			Last       int `json:"last"`
		} `json:"pagination"`
		Paging struct {
			Count int `json:"count"`
			Total int `json:"total"`
			Page  int `json:"page"`
			Pages int `json:"pages"`
		} `json:"paging"`
		Matches []struct {
			Iid     string  `json:"iid"`
			Team    string  `json:"team"`
			Score   float64 `json:"score"`
			Channel struct {
				ID          string `json:"id"`
				IsChannel   bool   `json:"is_channel"`
				IsGroup     bool   `json:"is_group"`
				IsIm        bool   `json:"is_im"`
				IsMpim      bool   `json:"is_mpim"`
				IsShared    bool   `json:"is_shared"`
				IsOrgShared bool   `json:"is_org_shared"`
				IsExtShared bool   `json:"is_ext_shared"`
				IsPrivate   bool   `json:"is_private"`
				Name        string `json:"name"`
				User        string `json:"user"`
			} `json:"channel,omitempty"`
			Type     string `json:"type"`
			User     string `json:"user"`
			Username string `json:"username"`
			Ts       string `json:"ts"`
			Blocks   []struct {
				Type     string `json:"type"`
				BlockID  string `json:"block_id"`
				Elements []struct {
					Type     string `json:"type"`
					Elements []struct {
						Type string `json:"type"`
						Text string `json:"text"`
					} `json:"elements"`
				} `json:"elements"`
			} `json:"blocks"`
			Text        string `json:"text"`
			Permalink   string `json:"permalink"`
			NoReactions bool   `json:"no_reactions"`
			Files       []struct {
				ID                 string `json:"id"`
				Created            int    `json:"created"`
				Timestamp          int    `json:"timestamp"`
				Name               string `json:"name"`
				Title              string `json:"title"`
				Mimetype           string `json:"mimetype"`
				Filetype           string `json:"filetype"`
				PrettyType         string `json:"pretty_type"`
				User               string `json:"user"`
				UserTeam           string `json:"user_team"`
				Editable           bool   `json:"editable"`
				Size               int    `json:"size"`
				Mode               string `json:"mode"`
				IsExternal         bool   `json:"is_external"`
				ExternalType       string `json:"external_type"`
				IsPublic           bool   `json:"is_public"`
				PublicURLShared    bool   `json:"public_url_shared"`
				DisplayAsBot       bool   `json:"display_as_bot"`
				Username           string `json:"username"`
				URLPrivate         string `json:"url_private"`
				URLPrivateDownload string `json:"url_private_download"`
				Permalink          string `json:"permalink"`
				PermalinkPublic    string `json:"permalink_public"`
				EditLink           string `json:"edit_link"`
				Preview            string `json:"preview"`
				PreviewHighlight   string `json:"preview_highlight"`
				Lines              int    `json:"lines"`
				LinesMore          int    `json:"lines_more"`
				PreviewIsTruncated bool   `json:"preview_is_truncated"`
				IsStarred          bool   `json:"is_starred"`
				HasRichPreview     bool   `json:"has_rich_preview"`
				FileAccess         string `json:"file_access"`
			} `json:"files,omitempty"`
		} `json:"matches"`
	} `json:"messages"`
}

// https://en.wikipedia.org/wiki/ANSI_escape_code
var colorCodes = []int{31, 32, 33, 34, 35, 36, 36, 90, 91, 92, 93, 94, 95, 96, 9}
var defaultKeywords = []string{"hippocampus"}

func Run(a *Args) error {
	http.DefaultTransport.(*http.Transport).MaxIdleConnsPerHost = http.DefaultTransport.(*http.Transport).MaxIdleConns

	keywords := a.Keywords
	keywords = append(keywords, defaultKeywords...)

	var tr *regexp.Regexp
	if a.InvertMatch != "" {
		tr = regexp.MustCompile(a.InvertMatch)
	}

	eg := errgroup.Group{}

	stopCh := make(chan struct{}, 1)

	var files []string
	for _, keyword := range keywords {
		f, err := os.CreateTemp("", keyword)
		if err != nil {
			return xerrors.Errorf("failed to create temp file: %w", err)
		}
		files = append(files, f.Name())

		func(keyword string, f *os.File) {
			eg.Go(func() error {
				ticker := time.NewTicker(a.Interval)

				cache := restore(keyword)
				for {
					select {
					case <-stopCh:
						return nil
					case <-ticker.C:
						request, err := http.NewRequest("GET", fmt.Sprintf("https://slack.com/api/search.messages?query=%s&count=%d&sort=timestamp&pretty=1", url.QueryEscape(keyword), a.Count), nil)
						if err != nil {
							return xerrors.Errorf("failed to create request: %w", err)
						}
						request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", a.SlackToken))
						response, err := http.DefaultClient.Do(request)
						if err != nil {
							return xerrors.Errorf("failed to do request: %w", err)
						}
						defer func() {
							_ = response.Body.Close()
						}()

						var r SlackResponse
						if err := json.NewDecoder(response.Body).Decode(&r); err != nil {
							return xerrors.Errorf("failed to decode response body: %w", err)
						}
						for i := len(r.Messages.Matches); i > 0; i-- {
							match := r.Messages.Matches[i-1]

							if tr != nil && tr.MatchString(match.Text) {
								continue
							}

							a := strings.SplitN(match.Ts, ".", 2)
							sec, err := strconv.Atoi(a[0])
							if err != nil {
								return xerrors.Errorf("failed to convert string to int: %w", err)
							}
							nsec, err := strconv.Atoi(a[1])
							if err != nil {
								return xerrors.Errorf("failed to convert string to int: %w", err)
							}
							t := time.Unix(int64(sec), int64(nsec))

							if _, ok := cache[match.Ts]; ok {
								continue
							}
							if _, err := f.WriteString(fmt.Sprintf("\x1b[%dm#%s\x1b[0m %s [%s] \n%s\n%s\n\n", colorCodes[len(match.Channel.Name)%len(colorCodes)], match.Channel.Name, match.Username, t.String(), match.Text, match.Permalink)); err != nil {
								return xerrors.Errorf("failed to write string: %w", err)
							}
							cache[match.Ts] = struct{}{}
						}

						if len(cache) > a.Count {
							keys := make([]string, 0, len(cache))
							for k := range cache {
								keys = append(keys, k)
							}
							sort.Strings(keys)
							for i := 0; i < len(keys)-a.Count; i++ {
								delete(cache, keys[i])
							}
						}

						if err := save(keyword, cache); err != nil {
							return xerrors.Errorf("failed to save cache: %w", err)
						}
					}
				}
			})
		}(keyword, f)
	}

	fmt.Println(strings.Join(files, " "))

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT)
	go func() {
		<-quit
		close(stopCh)
	}()

	if err := eg.Wait(); err != nil {
		return xerrors.Errorf("failed to wait: %w", err)
	}

	for _, file := range files {
		if err := os.Remove(file); err != nil {
			return xerrors.Errorf("failed to remove file: %w", err)
		}
	}

	return nil
}

func restore(keyword string) map[string]struct{} {
	var cache map[string]struct{}
	b, err := os.ReadFile(filepath.Join(directory(), "cache", keyword))
	if err != nil {
		return map[string]struct{}{}
	}
	if err := json.Unmarshal(b, &cache); err != nil {
		return map[string]struct{}{}
	}
	return cache
}

func save(keyword string, cache map[string]struct{}) error {
	b, err := json.Marshal(cache)
	if err != nil {
		return xerrors.Errorf("failed to marshal cache: %w", err)
	}
	d := filepath.Join(directory(), "cache")
	if err := os.MkdirAll(d, 0755); err != nil {
		return xerrors.Errorf("failed to create directory: %w", err)
	}
	if err := os.WriteFile(filepath.Join(d, keyword), b, 0644); err != nil {
		return xerrors.Errorf("failed to write file: %w", err)
	}
	return nil
}

func directory() string {
	home := os.Getenv("XDG_DATA_HOME")
	if home == "" {
		u, err := os.UserHomeDir()
		if err != nil {
			return os.TempDir()
		}
		home = filepath.Join(u, ".local", "share")
	}
	return filepath.Join(home, "armyknife")
}
