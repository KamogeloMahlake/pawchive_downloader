package clients

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
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
		UserAgent: `Mozilla/5.0 (Windows NT 10.0; Win64; x64)
AppleWebKit/537.36 (KHTML, like Gecko)
Chrome/124.0.0.0 Safari/537.36 KemonoSuite/2.0`,
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
		offset += len(temp)
	}
}

func (c *Client) GetCreator(ctx context.Context, service, userId string) (pawchive.Creator, error) {
	var creator pawchive.Creator
	url := fmt.Sprintf(profilePath, c.BaseUrl, service, userId)
	data, err := c.fetch(ctx, url)
	if err != nil {
		return creator, err
	}

	if err = json.Unmarshal(data, creator); err != nil {
		return creator, err
	}

	if creator == (pawchive.Creator{}) {
		return creator, fmt.Errorf("Creator %s/%s: %w", service, userId, ErrorNotFound)
	}
	return creator, nil
}
