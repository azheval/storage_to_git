package runner

import "fmt"

type User interface {
	ConnectionString() string
}

type IBUser struct {
	Name     string
	Password string
}

func (u *IBUser) ConnectionString() string {
	if u.Name == "" {
		return ""
	}
	if u.Password == "" {
		return fmt.Sprintf("/N %s", u.Name)
	}
	return fmt.Sprintf("/N %s /P %s", u.Name, u.Password)
}

type StorageUser struct {
	Name     string
	Password string
}

func (u *StorageUser) ConnectionString() string {
	if u.Password == "" {
		return fmt.Sprintf("/ConfigurationRepositoryN %s", u.Name)
	}
	return fmt.Sprintf("/ConfigurationRepositoryN %s /ConfigurationRepositoryP %s", u.Name, u.Password)
}
