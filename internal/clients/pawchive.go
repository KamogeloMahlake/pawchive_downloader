package clients

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/KamogeloMahlake/pawchive_downloader/pawchive"
)

const (
	postPath = "%s/%s/user/%s" //list of posts, paged with ?o= <offset>
	profilePath = "%s/%s/user/%s/profile" // creator profile
)

var ErrorNotFound = errors.New("Not found")

type Client struct {
	Client *http.Client
	DownloadClient *http.Client
	BaseUrl string
	UserAgent string
	MaxRetries int
	RetryDelay  time.Duration
}

func NewClient(baseUrl string) *Client {
	return &Client{
		Client: &http.Client{Timeout: time.Second * 30},
		BaseUrl: baseUrl,
		DownloadClient: &http.Client{
			Transport: &http.Transport{
				Proxy: http.ProxyFromEnvironment,
				ResponseHeaderTimeout: time.Second * 30,
				TLSHandshakeTimeout: time.Second * 15,
			},
		},
		UserAgent: "pawchive_downloader/0.2",
		MaxRetries: 3,
		RetryDelay: time.Second * 2,

	}
}

func retryableStatus(code int) bool {
	return code == http.StatusTooManyRequests || code >= 500
}

func wait(ctx context.Context, d time.Duration) error{
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

//Returns the delay before retry number attempt when the server has sent one
func (c *Client) backoff(attempt int, response *http.Response) time.Duration{
	duration := c.RetryDelay * time.Duration(1<<(attempt-1))
	if response == nil {
		return duration
	}
	s := response.Header.Get("Retry-After")
	if s == "" {
		return duration
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return duration
	}
	if n > 60 {
		n = 60
	}
	return time.Duration(n) * time.Second
}

func (c *Client) fetch(ctx context.Context, url string)([]byte, error) {
	var lastError error
	var delay time.Duration

	for i := 0; i <= c.MaxRetries; i++ {
		if i > 0 {
			if delay == 0 {
				delay = c.backoff(i, nil)
			}
			if err := wait(ctx, delay); err != nil {
				return nil, err
			}
			delay = 0
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		
		req.Header.Set("User-Agent", c.UserAgent)
		req.Header.Set("Accept", "application/json")
		
		res, err := c.Client.Do(req)
		if err != nil {
			lastError = err
			if ctx.Err() != nil {
			  return nil, ctx.Err()
			}
			continue
		}
		data, readErr := io.ReadAll(res.Body)
		res.Body.Close()

		switch {
		case res.StatusCode == http.StatusOK:
			if readErr != nil {
				return nil, readErr
			}
			return data, nil
		case res.StatusCode == http.StatusNotFound:
			return nil, fmt.Errorf("%s: %w", url, ErrorNotFound)
		case retryableStatus(res.StatusCode):
			lastError = fmt.Errorf("Unexpected status code %d for %s", res.StatusCode, url)
			delay = c.backoff(i+1, res)
		default:
			return nil, fmt.Errorf("Unexpected status code %d for %s", res.StatusCode, url)
		}
	}	
	return nil, fmt.Errorf("Giving up after %d retries: %w", c.MaxRetries, lastError)
}


func (c *Client) GetPosts(ctx context.Context, service, userId string) ([]pawchive.Post, error) {
	var posts []pawchive.Post
	seen := map[string]bool{}
	offset := 0
	
	for {
	  url := fmt.Sprintf(postPath, c.BaseUrl, service, userId) + fmt.Sprintf("?o=%d", offset)
	  data, err := c.fetch(ctx, url)	
		if err != nil {
			return posts, err	
		}
	
		var temp []pawchive.Post

	  if err = json.Unmarshal(data, &temp); err != nil {
		  return posts, err
	  }
		if len(temp) == 0 {
			return posts, nil
		}

		added := 0 
		for _, post := range temp {
			if post.ID == "" || seen[post.ID] {
				continue
			}
			seen[post.ID] = true
			posts = append(posts, post)
			added++
		}

		if added == 0 {
			return posts, nil
		}
		offset += 50
	}
}

func (c *Client) GetCreator(ctx context.Context, service, userId string) (pawchive.Creator, error) {
	var creator pawchive.Creator
	url := fmt.Sprintf(profilePath, c.BaseUrl, service, userId)
	data, err := c.fetch(ctx, url)
	if err != nil {
		return creator, err
	}

	if err = json.Unmarshal(data, &creator); err != nil {
		return creator, err
	}

	if creator == (pawchive.Creator{}) {
		return creator, fmt.Errorf("Creator %s/%s: %w", service, userId, ErrorNotFound)
	}
	return creator, nil
}

func (c *Client) downloadOnce(ctx context.Context, rawUrl, dest string) (int64, bool, error) {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return 0, false, err
	}

	part := dest + ".part"
	var have int64
	if st, err := os.Stat(part); err == nil {
		have = st.Size()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawUrl, nil)
	if err != nil {
		return 0, false, nil
	}

	req.Header.Set("User-Agent", c.UserAgent)
	if have > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d", have))
	}

	res, err := c.DownloadClient.Do(req)
	if err != nil {
		return 0, ctx.Err() == nil, err
	}
	defer res.Body.Close()

	flags := os.O_CREATE | os.O_WRONLY
	switch res.StatusCode {
	case http.StatusOK:
		flags |= os.O_TRUNC
	case http.StatusPartialContent:
		flags |= os.O_APPEND
	case http.StatusRequestedRangeNotSatisfiable:
		os.Remove(part)
		return 0, true, fmt.Errorf("Range not satisfiable for %s, restarting", rawUrl)
	case http.StatusNotFound:
		return 0, false, fmt.Errorf("%s: %w", rawUrl, ErrorNotFound)
	default:
		return 0, retryableStatus(res.StatusCode), fmt.Errorf("Unexpected status code %d for %s", res.StatusCode, rawUrl)
	}

	file, err := os.OpenFile(part, flags, 0o644)
	if err != nil {
		return 0, false, err
	}

	written, err := io.Copy(file, res.Body)
	if err != nil {
		return 0, ctx.Err() == nil, err
	}
	err = file.Close()
	if err != nil {
		return 0, false, err
	}
	if res.ContentLength >= 0 && written != res.ContentLength {
		return 0, true, fmt.Errorf("Short download for %s: got %d of %d bytes", rawUrl, written, res.ContentLength)
	}
	if err = os.Rename(part, dest); err != nil {
		return 0, false, err
	}

	if st, err := os.Stat(dest); err != nil {
		return 0, false, err
	} else {
		return st.Size(), false, nil
	}
}

func (c *Client) DownloadFile(ctx context.Context, rawUrl, dest string) (int64, error) {
	var lastError error 
	for i := 0; i <= c.MaxRetries; i++ {
		if i > 0 {
			if err := wait(ctx, c.backoff(i, nil)); err != nil {
				return 0, err
			}
		}

		size, retry, err := c.downloadOnce(ctx, rawUrl, dest)
		if err == nil {
			return size, nil
		}
		lastError = err
		if !retry || ctx.Err() != nil {
			break
		}
	}
	return 0, lastError
}
