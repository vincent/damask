package storage

import (
	"io"
	"os"
	"path/filepath"
	"strings"
)

type LocalStorage struct {
	base string
}

func NewLocalStorage(base string) (Storage, error) {
	if err := os.MkdirAll(base, 0750); err != nil {
		return nil, err
	}
	return &LocalStorage{base: base}, nil
}

func (s *LocalStorage) Put(key string, r io.Reader) error {
	dst := filepath.Join(s.base, filepath.FromSlash(key))
	if err := os.MkdirAll(filepath.Dir(dst), 0750); err != nil {
		return err
	}
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, r)
	return err
}

func (s *LocalStorage) Get(key string) (io.ReadCloser, error) {
	return os.Open(filepath.Join(s.base, filepath.FromSlash(key)))
}

func (s *LocalStorage) LocalPath(key string) string {
	return filepath.Join(s.base, filepath.FromSlash(key))
}

func (s *LocalStorage) Delete(key string) error {
	return os.RemoveAll(filepath.Join(s.base, filepath.FromSlash(key)))
}

func (s *LocalStorage) List(prefix string) ([]string, error) {
	root := filepath.Join(s.base, filepath.FromSlash(prefix))
	var keys []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			rel, _ := filepath.Rel(s.base, path)
			keys = append(keys, strings.ReplaceAll(rel, string(filepath.Separator), "/"))
		}
		return nil
	})
	if os.IsNotExist(err) {
		return []string{}, nil
	}
	return keys, err
}
