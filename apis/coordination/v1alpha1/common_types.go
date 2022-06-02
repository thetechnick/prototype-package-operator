package v1alpha1

// TargetAPI specifis an API to use for operations.
type TargetAPI struct {
	Group   string `json:"group"`
	Version string `json:"version"`
	Kind    string `json:"kind"`
}
