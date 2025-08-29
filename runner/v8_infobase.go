package runner

import "fmt"

type Infobase struct {
	Path           string
	User           *IBUser
}

func (ib *Infobase) ConnectionString() string {
	return fmt.Sprintf("/IBConnectionString %s %s", ib.Path, ib.User.ConnectionString())
}
