package main

import (
	"log"
	"sort"
)

// An extMirror is capable of mirroring the current local snapshots to
// snapshot-based volumegroup containing ext4 filesystems.
type lvmMirror struct {
	VgName string
	Prefix string
	backup *Backup
}

func (m *lvmMirror) Push(b *Backup) (err error) {
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
		base := VgName{VG: m.VgName, LV: undate(m.Prefix + vg.LV)}
		dest := VgName{VG: m.VgName, LV: m.Prefix + vg.LV}
		err = m.pushVol(vg, dest, base)
		if err != nil {
			return
		}

		// Make a snapshot.  This needs to be done outside of
		// the 'pushVol' function so that the volumes are
		// cleanly unmounted before making the snapshot.
		err = snapshot(base, dest)
		if err != nil {
			return
		}
	}

	return nil
}

// Given a list of source volumes, remove all that are present in the
// destination mirror, and return the result.
func (m *lvmMirror) filterSource(src []VgName) (result []VgName, err error) {

	sbits := make(map[VgName]bool)

	for _, svol := range src {
		sbits[svol] = true
	}

	for _, svol := range src {
		for _, vol := range m.backup.lvm.Volumes {
			if vol.VG != m.VgName {
				continue
			}

			if vol.LV == m.Prefix+svol.LV {
				delete(sbits, svol)
			}
		}
	}

	result = make([]VgName, 0, len(sbits))

	for k := range sbits {
		result = append(result, k)
	}

	return
}

// Mirror a single volume.
func (m *lvmMirror) pushVol(src, dest, base VgName) (err error) {
	log.Printf("Pushing %s to %s (base = %s)", src.TextName(), dest.TextName(), base.TextName())

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

	err = m.backup.mount(base, "/mnt/new", true)
	if err != nil {
		return
	}
	defer m.backup.umount(base)

	err = m.backup.rsync("/mnt/old/.", "/mnt/new")
	if err != nil {
		return
	}

	return
}
