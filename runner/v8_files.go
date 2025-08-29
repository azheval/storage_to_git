package runner

import (
	"path/filepath"
	"runtime"
)

type V8Files struct {
	ThickClient string
	ThinClient  string
	Ibcmd       string
	Rac         string
}

func NewV8Files(catalog1cv8 string) *V8Files {
	var exeSuffix string
	if runtime.GOOS == "windows" {
		exeSuffix = ".exe"
	}

	return &V8Files{
		ThickClient: filepath.Join(catalog1cv8, "1cv8"+exeSuffix),
		ThinClient:  filepath.Join(catalog1cv8, "1cv8c"+exeSuffix),
		Ibcmd:       filepath.Join(catalog1cv8, "ibcmd"+exeSuffix),
		Rac:         filepath.Join(catalog1cv8, "rac"+exeSuffix),
	}
}