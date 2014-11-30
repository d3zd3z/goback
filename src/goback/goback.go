package main

import (
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

	info, ok := conf[host]
	if !ok {
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

	err = backup.LogRotate()
	if err != nil {
		log.Fatalf("Error rotating gosure log: %s", err)
	}

	err = backup.MakeSnap()
	if err != nil {
		// TODO: Undo the backup.
		log.Fatalf("Error making snapshot: %s", err)
	}

	err = backup.GoSure()
	if err != nil {
		log.Fatalf("Error running gosure: %s", err)
	}
}

// This probably should be in the config file.
var gosurePath = "/home/davidb/bin/gosure"
