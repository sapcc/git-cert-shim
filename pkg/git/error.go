// Copyright 2020 SAP SE
// SPDX-License-Identifier: Apache-2.0

package git

import "strings"

func isErrFailedToPushSomeRefs(err error) bool {
	if err != nil {
		return strings.Contains(err.Error(), "failed to push some refs")
	}
	return false
}
