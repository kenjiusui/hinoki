package state

import (
	"encoding/json"
	"os"
)

type Entry struct {
	Rkey string `json:"rkey"`
	Hash string `json:"hash"`
}

type State struct {
	Documents map[string]Entry `json:"documents"`
}

func Load(path string) (*State, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &State{Documents: map[string]Entry{}}, nil
	}
	if err != nil {
		return nil, err
	}
	s := &State{}
	if err := json.Unmarshal(data, s); err != nil {
		return nil, err
	}
	if s.Documents == nil {
		s.Documents = map[string]Entry{}
	}
	return s, nil
}

func (s *State) Save(path string) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
