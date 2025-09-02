// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package certificate

func checkSANs(commonName string, sans []string) []string {
	if sans == nil {
		sans = make([]string, 0)
	}

	if !ssliceContains(commonName, sans) {
		sans = append([]string{commonName}, sans...)
	}

	return sans
}

func ssliceContains(theString string, theStringSlice []string) bool {
	for _, s := range theStringSlice {
		if s == theString {
			return true
		}
	}
	return false
}
