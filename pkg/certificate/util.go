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
