package main

import (
	"github.com/itchio/butler/installer/archive"
	"github.com/itchio/butler/installer/dmg"
	"github.com/itchio/butler/installer/iexpress"
	"github.com/itchio/butler/installer/inno"
	"github.com/itchio/butler/installer/msi"
	"github.com/itchio/butler/installer/naked"
	"github.com/itchio/butler/installer/nsis"
)

func init() {
	naked.Register()
	archive.Register()
	nsis.Register()
	inno.Register()
	msi.Register()
	dmg.Register()
	iexpress.Register()
}
