package store

import (
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"sync"

	"github.com/hashicorp/raft"
)

type Command struct {
	Op    string `json:"op,omitempty"`
	Key   string `json:"key,omitempty"`
	Value string `json:"value,omitempty"`
}

type Store struct {
	mu   sync.RWMutex
	data map[string]string
}

func NewStore() *Store {
	return &Store{
		data: make(map[string]string),
	}
}

func (s *Store) Get(key string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	val, ok := s.data[key]
	return val, ok
}

func (s *Store) Set(key, val string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = val
}

func (s *Store) Delete(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, key)
}

func (s *Store) Apply(log *raft.Log) any {
	var cmd Command
	if err := json.Unmarshal(log.Data, &cmd); err != nil {
		panic(fmt.Sprintf("failed to unmarshal command: %s", err.Error()))
	}
	switch cmd.Op {
	case "set":
		s.Set(cmd.Key, cmd.Value)
		return nil
	case "delete":
		s.Delete(cmd.Key)
		return nil
	default:
		panic(fmt.Sprintf("unrecognized command op: %s", cmd.Op))
	}
}

func (s *Store) Restore(reader io.ReadCloser) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	b, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	var data map[string]string
	if err := json.Unmarshal(b, &data); err != nil {
		return err
	}
	s.data = data
	return nil
}

type fsmSnapshot struct {
	data map[string]string
}

func (s *Store) Snapshot() (raft.FSMSnapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	o := make(map[string]string)
	maps.Copy(o, s.data)
	return &fsmSnapshot{data: o}, nil
}

func (f *fsmSnapshot) Persist(sink raft.SnapshotSink) error {
	err := func() error {
		b, err := json.Marshal(f.data)
		if err != nil {
			return err
		}
		if _, err := sink.Write(b); err != nil {
			return err
		}
		return sink.Close()
	}()
	if err != nil {
		sink.Cancel()
	}
	return err
}

func (f *fsmSnapshot) Release() {}
