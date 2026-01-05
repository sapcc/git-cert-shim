// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package certificate

import "slices"

func checkSANs(commonName string, sans []string) []string {
	if sans == nil {
		sans = make([]string, 0)
	}

	if !slices.Contains(sans, commonName) {
		sans = append([]string{commonName}, sans...)
	}

	return sans
}
