package reconciler

import (
	"fmt"
	"strings"

	"github.com/blang/semver"
)

// ShouldTakeUpdatePath checks whether upgrade-type reconciliation should be executed. Returns error in case of downgrade
func ShouldTakeUpdatePath(targetVersion, currentVersion string, deploying bool) (bool, error) {

	if deploying {
		return false, nil
	}
	if targetVersion == currentVersion {
		return false, nil
	}

	// if no current version, then we can't perform semantic version comparison. But since the target version is not
	// empty, and since we are not deploying, then we're upgrading
	if currentVersion == "" {
		return true, nil
	}

	// semver doesn't like the 'v' prefix
	targetVersion = strings.TrimPrefix(targetVersion, "v")
	currentVersion = strings.TrimPrefix(currentVersion, "v")

	// our default position is that this is an update.
	// So if the target and current version do not
	// adhere to the semver spec, we assume by default the
	// update path is the correct path.
	shouldTakeUpdatePath := true
	target, err := semver.Make(targetVersion)
	if err == nil {
		current, err := semver.Make(currentVersion)
		if err == nil {
			if target.Compare(current) < 0 {
				err := fmt.Errorf("operator downgraded, will not reconcile")
				return false, err
			} else if target.Compare(current) == 0 {
				shouldTakeUpdatePath = false
			}
		}
	}

	return shouldTakeUpdatePath, nil
}
