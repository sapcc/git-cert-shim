package git

import "strings"

func isErrFailedToPushSomeRefs(err error) bool {
	if err != nil {
		return strings.Contains(err.Error(), "failed to push some refs")
	}
	return false
}
