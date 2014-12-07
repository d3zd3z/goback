package main

import (
	"errors"
	"log"
	"os"
	"time"
)

func main() {
	log.Printf("Godump!")

	var err error
	conf, err := loadConfig()
	if err != nil {
		log.Printf("Unable to load config file: %s", err)
		return
	}

	host, err := os.Hostname()
	if err != nil {
		log.Fatalf("Unable to get current hostname: %s", err)
	}
	log.Printf("Hostname: %q", host)

	var info *Host
	for _, hi := range conf {
		if hi.Host == host {
			info = hi
			break
		}
	}
	if info == nil {
		log.Fatalf("Host %q not found in config file", host)
	}

	namer := newNamer()

	// log.Printf("info: %#v", info)
	// for _, fs := range info.Filesystems {
	// 	log.Printf("fs: %#v", fs)
	// 	log.Printf("volname: %q", namer.Snapvol(fs))
	// 	log.Printf("dev: %q", namer.Snapdev(fs))
	// }

	lvm, err := GetLVM()
	if err != nil {
		log.Fatalf("Error getting lvm info: %s", err)
	}

	// The various parts of the backup.
	var backup Backup
	backup.conf = conf
	backup.namer = namer
	backup.host = info
	backup.lvm = lvm
	backup.time = time.Now()

	// Get the command.
	if len(os.Args) < 2 {
		log.Fatalf("Usage: %s command", os.Args[0])
	}

	cmd, ok := commands[os.Args[1]]
	if !ok {
		log.Fatalf("Unknown command: %q", os.Args[1])
	}

	err = cmd(&backup, os.Args[2:]...)
	if err != nil {
		log.Fatalf("Error running snapshot: %s", err)
	}
}

type command func(*Backup, ...string) error

var commands = map[string]command{
	"snap": (*Backup).SnapCmd,
	"push": (*Backup).PushCmd,
}

func (b *Backup) SnapCmd(args ...string) (err error) {

	if len(args) != 0 {
		err = errors.New("'snap' command not expecting additional arguments")
		return
	}

	err = b.LogRotate()
	if err != nil {
		return
	}

	err = b.MakeSnap()
	if err != nil {
		// TODO: Undo the backup.
		return
	}

	err = b.GoSure()
	if err != nil {
		return
	}

	return
}

func (b *Backup) PushCmd(args ...string) (err error) {
	if len(args) != 1 {
		err = errors.New("'push' command expects one argument")
		return
	}

	var info GeneralMirror
	for _, m := range b.host.Mirrors {
		if m["name"] == args[0] {
			info = m
			break
		}
	}

	if info == nil {
		err = errors.New("'push argument doesn't match a mirrors entry")
		return
	}

	m, err := info.GetMirror()
	if err != nil {
		return
	}

	err = m.Push(b)
	return
}

// This probably should be in the config file.
var gosurePath = "/home/davidb/bin/gosure"
