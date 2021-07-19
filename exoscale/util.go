package exoscale

// defaultString returns the value of the string pointer s if not nil, otherwise the default value specified.
func defaultString(s *string, def string) string {
	if s != nil {
		return *s
	}

	return def
}
