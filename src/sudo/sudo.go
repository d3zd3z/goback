// Run commands through sudo if we aren't root.  Also manages a
// periodic invocation of sudo -v to keep the cookie alive.

package sudo

import (
	"log"
	"os"
	"os/exec"
	"time"
)

var needSudo = false
var TickInterval = 30 * time.Second
var running = false

func init() {
	id := os.Geteuid()
	needSudo = id != 0
}

func Setup() (err error) {
	if running {
		return
	}

	err = SudoKeeper()
	if err != nil {
		return
	}

	tick := time.NewTicker(TickInterval)
	go func() {
		for {
			_ = <-tick.C
			err = SudoKeeper()
			if err != nil {
				log.Printf("Warning: error running sudo: %s", err)
			}
		}
	}()

	running = true

	return
}

// Function called periodically that will enforce that sudo -v has
// been run.  This can also be invoked initially to
func SudoKeeper() (err error) {
	if !needSudo {
		return
	}

	// log.Printf("sudo tick: %s", time.Now().Format("15:04:05"))
	cmd := exec.Command("sudo", "-v")
	err = cmd.Run()

	return
}

// Modify this command so that the command is run using sudo instead
// of directly.  If we're already running as root, return the same
// command, otherwise return a newly allocated one copying the fields
// to run with Sudo.  The Path and Args fields will be freshly
// allocated, and the rest will just be copied over.
func Sudoify(cmd *exec.Cmd) *exec.Cmd {
	if !needSudo {
		log.Printf("Running: %#v", cmd)
		return cmd
	}

	ncmd := *cmd

	// log.Printf("Old cmd: %#v", &ncmd)

	// new arg[0] gets the sudo command.
	// new arg[1] gets the sudo executable
	// new arg[2] gets the original cmd.
	// old arg[0] is discarded.

	args := make([]string, len(cmd.Args)+1)
	args[0] = "sudo"
	args[1] = cmd.Path
	copy(args[2:], cmd.Args[1:])
	ncmd.Args = args
	var err error
	ncmd.Path, err = exec.LookPath("sudo")
	if err != nil {
		log.Fatalf("Unable to find sudo command: %s", err)
	}
	// log.Printf("Running: %#v", &ncmd)
	return &ncmd
}
