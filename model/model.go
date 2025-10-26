package model

import "fmt"

type Installed struct {
	Name     string
	Version  string
	SysPath  string
	Location string
}

func (ins *Installed) Display() {
	fmt.Printf("Name: %s", ins.Name)
	fmt.Printf("Version: %s", ins.Version)
	fmt.Printf("SysPath: %s", ins.SysPath)
	fmt.Printf("Location: %s", ins.Location)
}
