package config

// FlattenForTest exposes flatten, so a test can enumerate every config key the
// same way Load does.
func FlattenForTest(nested map[string]any) map[string]any {
	return flatten("", nested)
}
