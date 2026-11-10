package clients

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"

	"github.com/KamogeloMahlake/pawchive_downloader/pawchive"
)

type Client struct {
	Client *http.Client
}

func (c *Client) fetch(url string)([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return []byte{}, err
	}

	resq, err := c.Client.Do(req)
	if err != nil {
		return []byte{}, err
	}

	defer resq.Body.Close()
	if resq.StatusCode != http.StatusOK {
		return []byte{}, fmt.Errorf("Unexpected status code: %d\n", resq.StatusCode)
	}

	data, err := io.ReadAll(resq.Body)
	if err != nil {
		return []byte{}, err
	} 
	return data, nil
}


func (c *Client) GetPosts(baseUrl, service string, userId int, posts *[]pawchive.Post) error {
	offset := 0
	for {
	  fullUrl := fmt.Sprintf("%s/%s/user/%d?o=%d", baseUrl, service, userId, offset)
	  data, err := c.fetch(fullUrl)	
		if err != nil {
			return err
		}
	
		var temp []pawchive.Post

	  if err := json.Unmarshal(data, &temp); err != nil {
		  return err
	  }
		if len(temp) == 0 {
			return nil
		}
		*posts = append(*posts, temp...)
		offset += 50
	}
}

func (c *Client) GetCreator(baseUrl, service string, userId int, creator *pawchive.Creator) error {
	fullUrl := fmt.Sprintf("%s/%s/user/%d/profile", baseUrl, service, userId)
	data, err := c.fetch(fullUrl)
	if err != nil {
		return err
	}

	if err = json.Unmarshal(data, creator); err != nil {
		return err
	}

	if reflect.ValueOf(creator).IsZero() {
		return fmt.Errorf("Creator not found")
	}
	return nil
}
