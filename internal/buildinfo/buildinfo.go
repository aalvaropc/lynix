package buildinfo

import "fmt"

var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

func String() string {
	return fmt.Sprintf("lynix %s (commit=%s, date=%s)", Version, Commit, Date)
}
