package main

import (
	"errors"
	"fmt"
	"os"
	"sort"
)

// A btrfs mirror mirrors to snapshots within a btrfs subvolume.
type btrMirror struct {
	Prefix string
	backup *Backup
}

func (m *btrMirror) Push(b *Backup) (err error) {
	m.backup = b

	src, err := m.backup.GetSources()
	if err != nil {
		return
	}

	src, err = m.filterSource(src)
	if err != nil {
		return
	}

	sort.Sort(VgNameSlice(src))

	for _, vg := range src {
		base := m.Prefix + "/" + undate(vg.LV)
		btr := m.Prefix + "/" + vg.LV
		fmt.Printf("Sync: %s to %s then %s\n",
			vg.TextName(), base, btr)

		err = m.pushVol(vg, base, btr)
		if err != nil {
			return
		}

		err = btrSnap(base, btr)
		if err != nil {
			return
		}
	}

	return
}

// Filter out the source volumes to only those that aren't present in
// the btr tree.
func (m *btrMirror) filterSource(src []VgName) (result []VgName, err error) {
	dvols, err := m.scanDest()
	if err != nil {
		return
	}

	result = make([]VgName, 0)

	for _, vol := range src {
		_, ok := dvols[vol.LV]
		if !ok {
			result = append(result, vol)
		}
	}

	return
}

func (m *btrMirror) scanDest() (names map[string]bool, err error) {
	fi, err := os.Stat(m.Prefix)
	if err != nil {
		return
	}
	if !fi.IsDir() {
		msg := fmt.Sprintf("%q is not a directory", m.Prefix)
		err = errors.New(msg)
		return
	}
	d, err := os.Open(m.Prefix)
	if err != nil {
		return
	}
	defer d.Close()

	nn, err := d.Readdirnames(-1)
	if err != nil {
		return
	}

	names = make(map[string]bool)

	for _, n := range nn {
		names[n] = true
	}

	return
}

func (m *btrMirror) pushVol(src VgName, base, dest string) (err error) {

	// Activate the source.
	err = m.backup.activate(src)
	if err != nil {
		return
	}
	defer m.backup.deactivate(src)

	err = m.backup.mount(src, "/mnt/old", false)
	if err != nil {
		return
	}
	defer m.backup.umount(src)

	err = m.backup.rsync("/mnt/old/.", base)
	if err != nil {
		return
	}

	return
}
