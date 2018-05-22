package ionic

import (
	"bufio"
	"fmt"
	"regexp"
	"strings"

	"github.com/bitrise-io/go-utils/command"
	ver "github.com/hashicorp/go-version"
	"github.com/pkg/errors"
)

// Version ...
func Version() (*ver.Version, error) {
	cmd := command.New("ionic", "-v")
	cmd.SetStdin(strings.NewReader("Y"))
	out, err := cmd.RunAndReturnTrimmedCombinedOutput()
	if err != nil {
		return nil, err
	}

	// fix for ionic-cli intercative version output: `[1000D[K3.2.0`
	pattern := `(?P<version>\d+\.\d+\.\d+)`

	reader := strings.NewReader(out)
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		if match := regexp.MustCompile(pattern).FindStringSubmatch(line); len(match) == 2 {
			versionStr := match[1]
			version, err := ver.NewVersion(versionStr)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to parse version: %s", versionStr)
			}
			return version, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return nil, fmt.Errorf("failed to find ionic version in output: %s", out)
}
