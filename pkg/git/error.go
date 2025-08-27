// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package git

import "strings"

func isErrFailedToPushSomeRefs(err error) bool {
	if err != nil {
		return strings.Contains(err.Error(), "failed to push some refs")
	}
	return false
}
