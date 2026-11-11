package storage

import (
	"sort"
	"time"
)

//Index is the per-creator table of contents.
type Index struct {
	Service string `json:"service"`
	UserID string `json:"user_id"`
	Name string `json:"name"`
	LastSync time.Time `json:"last_sync"`
	Posts map[string]*PostRecord `json:"posts"`
}

func (index *Index) SortedPost() []*PostRecord{
	output := make([]*PostRecord, 0, len(index.Posts))
	for _, post := range index.Posts {
		output = append(output, post)
	}

	sort.Slice(output, func(i, j int) bool {
		if output[i].Published != output[j].Published {
			return output[i].Published > output[j].Published
		}
		return output[i].ID > output[j].ID
	})
	return output
}
