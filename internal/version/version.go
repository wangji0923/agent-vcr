package version

import "fmt"

var (
	Version = "0.2.0-dev"
	Commit  = "unknown"
	Date    = "unknown"
)

type Info struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Date    string `json:"date"`
}

func Get() Info {
	return Info{
		Version: Version,
		Commit:  Commit,
		Date:    Date,
	}
}

func String() string {
	return fmt.Sprintf("agent-vcr %s commit=%s date=%s", Version, Commit, Date)
}
