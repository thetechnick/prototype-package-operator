package probe

type NamedProbe struct {
	Interface
	Name string
}

func (np *NamedProbe) GetName() string {
	return np.Name
}
