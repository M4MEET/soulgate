package model

// BuildModelOptionsForProvider returns models for a specific provider
// from the central registry.
func BuildModelOptionsForProvider(provider string) []ModelOption {
	def, err := LookupProvider(provider)
	if err != nil {
		return []ModelOption{}
	}
	return def.Models
}

// ModelOption represents a model option
type ModelOption struct {
	ID          string
	Name        string
	Description string
}
