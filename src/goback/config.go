package main

import (
	"fmt"
	"github.com/BurntSushi/toml"
)

func loadConfig() (conf Config, err error) {
	conf = make(Config)

	_, err = toml.DecodeFile("config.toml", conf)
	if err != nil {
		return
	}

	// log.Printf("Config: %#v", conf)
	// log.Printf("Meta: %#v", m)
	return
}

type Config map[string]*Host

type Host struct {
	Host        string
	Snapdir     string
	Filesystems []*FsInfo
}

type FsInfo struct {
	Volgroup string
	Lvname   string
	Mount    string
}

func (fs *FsInfo) VgName() VgName {
	return VgName{VG: fs.Volgroup, LV: fs.Lvname}
}

func (fs *FsInfo) String() string {
	return fmt.Sprintf("%s/%s (%s)", fs.Volgroup, fs.Lvname, fs.Mount)
}
