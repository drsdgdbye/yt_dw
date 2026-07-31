package filestore

import (
	"io"
	"os"
)

// FileStore — интерфейс для работы с файлами на диске.
type FileStore interface {
	Open(name string) (io.ReadCloser, error)
	Remove(name string) error
}

// OSFileStore — реализация FileStore через os-пакет.
type OSFileStore struct {
	basePath string
}

// New создаёт OSFileStore с базовой директорией.
func New(basePath string) *OSFileStore {
	return &OSFileStore{basePath: basePath}
}

// Open открывает файл на чтение.
func (s *OSFileStore) Open(name string) (io.ReadCloser, error) {
	return os.Open(s.basePath + name)
}

// Remove удаляет файл.
func (s *OSFileStore) Remove(name string) error {
	return os.Remove(s.basePath + name)
}
