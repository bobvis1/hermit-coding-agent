package tooldef

type Set struct{ names []string }

func NewSet(names ...string) Set { return Set{names: names} }

func (s Set) Without(name string) Set { return s }
