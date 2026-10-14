// Package fixtures contains deterministic upstream responses shared by the suites.
package fixtures

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

//go:embed movies.json
var movieJSON []byte

type Movie struct {
	ID          int      `json:"id"`
	Title       string   `json:"title"`
	Year        int      `json:"year"`
	Director    string   `json:"director"`
	Description string   `json:"description"`
	Actors      []string `json:"actors"`
	Genres      []string `json:"genres"`
}

func MovieAt(id int) Movie {
	var movies []Movie
	if err := json.Unmarshal(movieJSON, &movies); err != nil {
		panic(err)
	}
	if id > 0 && id <= len(movies) {
		return movies[id-1]
	}
	return Movie{ID: id, Title: fmt.Sprintf("Movie %d", id), Year: 2020, Director: "Test Director"}
}
