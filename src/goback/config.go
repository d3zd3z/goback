package main

import (
	"errors"
	"fmt"
	"regexp"

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
	Surelog     string
	Mirrors     []GeneralMirror
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

// Return a regular expression that will match the Lvname of snapshots
// of this filesystem.  Must match the format returned by
// (*Namer).Snapvol() and associated functions.
func (fs *FsInfo) MatchRe() *regexp.Regexp {
	q := regexp.QuoteMeta(fs.Lvname)
	text := fmt.Sprintf(`^%s\.\d\d\d\d\.\d\d\.\d\d$`, q)
	return regexp.MustCompile(text)
}

var dateRe = regexp.MustCompile(`\.\d\d\d\d\.\d\d\.\d\d$`)

// Given a lv name, remove the date suffix from it.
func undate(name string) string {
	return dateRe.ReplaceAllLiteralString(name, "")
}

// The general mirror type, just a mapping of keys and values.
type GeneralMirror map[string]string

// The General mirrors can be mapped into one that has actions
// associated with it.
type Mirror interface {
	Push(b *Backup) (err error)
}

// From a general mirror, get one specifically for a certain element.
func (m GeneralMirror) GetMirror() (result Mirror, err error) {
	switch m["style"] {
	case "lvm/ext4":
		vgname, ok := m["vgname"]
		if !ok {
			return nil, expecting(m["name"], "vgname")
		}

		prefix, ok := m["prefix"]
		if !ok {
			return nil, expecting(m["name"], "prefix")
		}

		return &lvmMirror{VgName: vgname, Prefix: prefix}, nil

	case "btrfs":
		prefix, ok := m["prefix"]
		if !ok {
			return nil, expecting(m["name"], "prefix")
		}

		return &btrMirror{Prefix: prefix}, nil

	default:
		msg := fmt.Sprintf("Unknown mirror style: %q", m["style"])
		err = errors.New(msg)
		return
	}
}

func expecting(name, key string) error {
	msg := fmt.Sprintf("Mirror configuration for %q needs %q key")
	return errors.New(msg)
}

// Within this host, look up a particular mirror returning its
// information.  Note that the mirror types should be expandable.
