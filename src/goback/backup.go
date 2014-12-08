package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"strings"
	"syscall"
	"time"

	"sudo"
)

// Management of backups themselves.

type Backup struct {
	conf    Config
	namer   *Namer
	host    *Host
	lvm     *LVInfo
	logFile *os.File
	time    time.Time
}

func (b *Backup) MakeSnap() (err error) {
	// Verify that today's snapshot doesn't exist.
	for _, fs := range b.host.Filesystems {
		// log.Printf("Checking %s", fs)
		snap := b.namer.SnapVgName(fs)

		if b.lvm.HasSnap(snap) {
			err = errors.New(fmt.Sprintf("Volume %s already present", snap))
			return
		}
	}

	// Now construct the snapshots.
	for _, fs := range b.host.Filesystems {
		base := fs.VgName()
		snap := b.namer.SnapVgName(fs)
		err = snapshot(base, snap)
		if err != nil {
			return
		}
	}

	return
}

// Invoke gosure on the snapshots.
func (b *Backup) GoSure() (err error) {
	for _, fs := range b.host.Filesystems {
		err = b.goSureOne(fs)
		if err != nil {
			return
		}
	}

	return
}

func (b *Backup) goSureOne(fs *FsInfo) (err error) {
	snap := b.namer.SnapVgName(fs)
	smount := b.snapName(fs)

	// Activate the VG.
	err = b.activate(snap)
	if err != nil {
		return
	}
	defer b.deactivate(snap)

	// for sanity sake, run a fsck.
	err = b.fsck(snap)
	if err != nil {
		return
	}

	// Mount it.
	err = b.mount(snap, smount, false)
	if err != nil {
		return
	}
	defer b.umount(snap)

	err = b.runGosure(fs)
	if err != nil {
		return
	}

	// Remount it rw.
	err = b.remount(smount, true)
	if err != nil {
		return
	}

	err = b.copyFile(path.Join(fs.Mount, "2sure.dat.gz"), b.snapName(fs))
	if err != nil {
		return
	}

	backPath := path.Join(fs.Mount, "2sure.bak.gz")
	exist, err := fileExists(backPath)
	if err != nil {
		return
	}
	if exist {
		err = b.copyFile(backPath, b.snapName(fs))
		if err != nil {
			return
		}
	}

	return
}

// Get the name of the mountpoint for this particular filesystem.
func (b *Backup) snapName(fs *FsInfo) string {
	return path.Join(b.host.Snapdir, fs.Lvname)
}

func (b *Backup) activate(vol VgName) (err error) {
	sudo.Setup()

	cmd := exec.Command("lvchange", "-ay", "-K", vol.DevName())
	cmd = sudo.Sudoify(cmd)
	showCommand(cmd)
	err = cmd.Run()
	return
}

func (b *Backup) deactivate(vol VgName) (err error) {
	sudo.Setup()

	cmd := exec.Command("lvchange", "-an", vol.DevName())
	cmd = sudo.Sudoify(cmd)
	showCommand(cmd)
	err = cmd.Run()
	return
}

func (b *Backup) mount(vol VgName, dest string, writable bool) (err error) {
	sudo.Setup()

	flags := make([]string, 0, 4)

	if !writable {
		flags = append(flags, "-r")
	}
	flags = append(flags, vol.DevName())
	flags = append(flags, dest)

	cmd := exec.Command("mount", flags...)
	cmd = sudo.Sudoify(cmd)
	showCommand(cmd)
	err = cmd.Run()
	return
}

func (b *Backup) remount(dest string, writable bool) (err error) {
	sudo.Setup()

	flag := "ro"
	if writable {
		flag = "rw"
	}
	flag = "remount," + flag

	cmd := exec.Command("mount", "-o", flag, dest)
	cmd = sudo.Sudoify(cmd)
	showCommand(cmd)
	err = cmd.Run()
	return
}

func (b *Backup) umount(vol VgName) (err error) {
	sudo.Setup()

	cmd := exec.Command("umount", vol.DevName())
	cmd = sudo.Sudoify(cmd)
	showCommand(cmd)
	err = cmd.Run()
	return
}

func (b *Backup) fsck(vol VgName) (err error) {
	sudo.Setup()

	cmd := exec.Command("fsck", "-p", "-f", vol.DevName())
	cmd = sudo.Sudoify(cmd)
	showCommand(cmd)
	err = cmd.Run()
	if err != nil {
		// Some unsuccessful results are fine.
		stat := cmd.ProcessState.Sys().(syscall.WaitStatus).ExitStatus()
		log.Printf("Status: %d", stat)
		if stat == 1 {
			err = nil
		}
	}
	// Successful error status is fine.
	return
}

func (b *Backup) runGosure(fs *FsInfo) (err error) {
	sudo.Setup()

	// TODO: Detect no 2sure.dat.gz file, and run a fresh gosure
	// instead of this scan.

	place := path.Join(fs.Mount, "2sure")

	cmd := exec.Command(gosurePath, "-file", place, "update")
	cmd = sudo.Sudoify(cmd)
	cmd.Dir = b.snapName(fs)
	showCommand(cmd)
	err = cmd.Run()
	if err != nil {
		return
	}

	// Run signoff and capture the output.
	b.message("sure of %s (%s) on %s", fs.Lvname, fs.Mount,
		b.time.Format("2006-01-02 15:04"))

	cmd = exec.Command(gosurePath, "-file", place, "signoff")
	cmd = sudo.Sudoify(cmd)
	cmd.Dir = b.snapName(fs)
	cmd.Stdout = b.logFile
	showCommand(cmd)
	err = cmd.Run()

	return
}

func (b *Backup) copyFile(from, to string) (err error) {
	sudo.Setup()

	cmd := exec.Command("cp", "-p", from, to)
	cmd = sudo.Sudoify(cmd)
	showCommand(cmd)
	err = cmd.Run()
	return
}

func (b *Backup) rsync(from, to string) (err error) {
	sudo.Setup()

	cmd := exec.Command("rsync", "-aXHi", "--delete", from, to)
	cmd = sudo.Sudoify(cmd)

	// TODO: Setup an rsync log as well.
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	showCommand(cmd)
	err = cmd.Run()
	return
}

func btrSnap(from, to string) (err error) {
	sudo.Setup()

	cmd := exec.Command("btrfs", "subvolume", "snapshot", "-r", from, to)
	cmd = sudo.Sudoify(cmd)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	showCommand(cmd)
	err = cmd.Run()
	return
}

func (b *Backup) LogRotate() (err error) {
	lname := b.host.Surelog
	bakname := lname + ".bak"

	err = os.Remove(bakname)
	if err != nil && !os.IsNotExist(err) {
		return
	}
	err = nil

	err = os.Rename(lname, bakname)
	if err != nil && !os.IsNotExist(err) {
		return
	}

	file, err := os.Create(lname)
	if err != nil {
		return
	}

	b.logFile = file

	return
}

func (b *Backup) message(format string, a ...interface{}) {
	text := fmt.Sprintf(format, a...)
	hyphens := strings.Map(func(a rune) rune { return '-' }, text)
	fmt.Fprintf(b.logFile, "%s\n", hyphens)
	fmt.Fprintf(b.logFile, "%s\n", text)
	fmt.Fprintf(b.logFile, "%s\n", hyphens)
}

// Return a list of all source volumes matching those specified in the
// backup.
func (b *Backup) GetSources() (src []VgName, err error) {
	src = make([]VgName, 0)

	for _, fs := range b.host.Filesystems {
		re := fs.MatchRe()

		for _, vol := range b.lvm.Volumes {
			if vol.VG == fs.Volgroup && re.FindString(vol.LV) != "" {
				src = append(src, vol.VgName())
			}
		}
	}

	return
}

func snapshot(base, snap VgName) (err error) {
	sudo.Setup()

	cmd := exec.Command("lvcreate", "-s",
		base.TextName(), "-n", snap.LV)
	cmd = sudo.Sudoify(cmd)

	showCommand(cmd)

	err = cmd.Run()

	return
}

func showCommand(cmd *exec.Cmd) {
	log.Printf("%s", strings.Join(cmd.Args, " "))

	if cmd.Dir != "" {
		log.Printf("  in dir: %q", cmd.Dir)
	}
}

func fileExists(path string) (exists bool, err error) {
	_, err = os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}

	return false, err
}
