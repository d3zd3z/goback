package main

import (
	"fmt"
	"time"
)

// Name management within backups.
type Namer struct {
	date string
}

func newNamer() *Namer {
	var result Namer

	result.date = time.Now().Local().Format("2006.01.02")

	return &result
}

func (n *Namer) Snapvol(fs *FsInfo) string {
	return fmt.Sprintf("%s.%s", fs.Lvname, n.date)
}

func (n *Namer) Snapdev(fs *FsInfo) string {
	return fmt.Sprintf("/dev/mapper/%s-%s", fs.Volgroup, n.Snapvol(fs))
}

func (n *Namer) SnapVgName(fs *FsInfo) VgName {
	return VgName{VG: fs.Volgroup, LV: n.Snapvol(fs)}
}
