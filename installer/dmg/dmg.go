package dmg

import "github.com/itchio/butler/installer"

type Manager struct {
}

var _ installer.Manager = (*Manager)(nil)

func (m *Manager) Name() string {
	return "dmg"
}

func Register() {
	installer.RegisterManager(&Manager{})
}
