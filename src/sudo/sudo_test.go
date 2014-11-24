// Test sudo.

// The sudo module is a little hard to test.  Easies seems to be to
// turn the logs on in the sudo module, and test various combinations
// of typing and not typing the password.

package sudo_test

import (
	"os/exec"
	"regexp"
	"strconv"
	"sudo"
	"testing"
	"time"
)

func TestSudo(t *testing.T) {
	// Make the ticker much faster.
	sudo.TickInterval = 1 * time.Second

	err := sudo.Setup()
	if err != nil {
		t.Errorf("Error starting sudo: %s", err)
		return
	}

	// Sleep a little bit.
	time.Sleep(5 * time.Second)

	// Have sudo forget.
	err = exec.Command("sudo", "-k").Run()
	if err != nil {
		t.Errorf("Error running sudo -k: %s", err)
		return
	}

	// Wait a bit more to make sure the ticker happens.
	time.Sleep(10 * time.Second)
}

var idRe = regexp.MustCompile(`id=(\d+)`)

func TestSudoRun(t *testing.T) {
	err := sudo.Setup()
	if err != nil {
		t.Errorf("Can't setup sudo: %s", err)
		return
	}

	cmd := exec.Command("id")
	cmd = sudo.Sudoify(cmd)
	out, err := cmd.Output()
	if err != nil {
		t.Errorf("Error running command: %s", err)
		return
	}

	t.Logf("Ran command: %q", string(out))

	// Search for the ID string in the command.
	places := idRe.FindSubmatch(out)
	if places == nil || len(places) != 2 {
		t.Errorf("Invalid return from regexp match")
		return
	}

	id, err := strconv.ParseInt(string(places[1]), 10, 0)
	if err != nil {
		t.Errorf("Unable to parse int uid in id command result: %s", err)
		return
	}

	if id != 0 {
		t.Errorf("Didn't run as 0: %d", id)
		return
	}
}
