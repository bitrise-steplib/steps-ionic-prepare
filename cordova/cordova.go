package cordova

import (
	"github.com/bitrise-io/go-utils/command"
	ver "github.com/hashicorp/go-version"
)

// Version ...
func Version() (*ver.Version, error) {
	cmd := command.New("cordova", "-v")
	out, err := cmd.RunAndReturnTrimmedCombinedOutput()
	if err != nil {
		return nil, err
	}
	return ver.NewVersion(out)
}
