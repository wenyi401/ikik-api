package service

func cloneCredentials(credentials map[string]any) map[string]any {
	if credentials == nil {
		return nil
	}
	cloned := make(map[string]any, len(credentials))
	for key, value := range credentials {
		cloned[key] = value
	}
	return cloned
}
