package runner

import "fmt"

type Storage struct {
	Path string
	User *StorageUser
}

func (s *Storage) ConnectionString() string {
	return fmt.Sprintf("/ConfigurationRepositoryF %s %s", s.Path, s.User.ConnectionString())
}
