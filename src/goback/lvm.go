package main

import (
	"errors"
	"fmt"
	"os/exec"
	"reflect"
	"strings"
	"sudo"
)

type LVInfo struct {
	Volumes []*VolInfo
	ByName  map[VgName]*VolInfo
}

type VgName struct {
	VG string
	LV string
}

func (vn *VgName) TextName() string {
	return fmt.Sprintf("%s/%s", vn.VG, vn.LV)
}

func (vn *VgName) DevName() string {
	return fmt.Sprintf("/dev/mapper/%s-%s", vn.VG, vn.LV)
}

type VgNameSlice []VgName

func (p VgNameSlice) Len() int      { return len(p) }
func (p VgNameSlice) Swap(i, j int) { p[i], p[j] = p[j], p[i] }

func (p VgNameSlice) Less(i, j int) bool {
	if p[i].VG < p[j].VG {
		return true
	}

	if p[i].VG > p[j].VG {
		return false
	}

	return p[i].LV < p[j].LV
}

// This should match the order of the lvs command used.
type VolInfo struct {
	LV       string `key:"LV"`
	VG       string `key:"VG"`
	Attr     string `key:"Attr"`
	Lsize    string `key:"LSize"`
	Pool     string `key:"Pool"`
	Origin   string `key:"Origin"`
	Dataused string `key:"Data%"`
	Metaused string `key:"Meta%"`
	Move     string `key:"Move"`
	Log      string `key:"Log"`
	Cpysync  string `key:"Cpy%Sync"`
	Convert  string `key:"Convert"`
}

func (v *VolInfo) VgName() VgName {
	return VgName{VG: v.VG, LV: v.LV}
}

func GetLVM() (info *LVInfo, err error) {
	sudo.Setup()

	cmd := exec.Command("lvs", "--separator", "|")
	cmd = sudo.Sudoify(cmd)

	text, err := cmd.Output()
	if err != nil {
		return
	}

	var result LVInfo

	result.Volumes = make([]*VolInfo, 0, 10)
	result.ByName = make(map[VgName]*VolInfo)

	lines := strings.Split(string(text), "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	infoType := getInfoType()

	for i, line := range lines {
		if len(line) < 3 {
			err = errors.New(fmt.Sprintf("lvs output line is blank: %q", line))
			return
		}

		fields := strings.Split(line[2:len(line)], "|")

		if i == 0 {
			err = checkHeader(fields, infoType)
			if err != nil {
				return
			}
		} else {
			vol := decodeVol(fields)
			result.Volumes = append(result.Volumes, vol)

			key := vol.VgName()
			if _, ok := result.ByName[key]; ok {
				panic("Duplicate vg/lv back from lvs")
			}
			result.ByName[vol.VgName()] = vol
		}
	}

	info = &result
	return
}

func getInfoType() reflect.Type {
	var t *VolInfo
	ti := reflect.TypeOf(t)
	return ti.Elem()
}

// TODO: Allow more mismatches in this to account for changes across
// versions of LVM.
func checkHeader(fields []string, t reflect.Type) (err error) {
	if len(fields) != t.NumField() {
		err = errors.New(fmt.Sprintf("Field count mismatch in VolInfo(%d) and Lvm(%d)",
			len(fields), t.NumField()))
		return
	}

	for i := 0; i < len(fields); i++ {
		f := t.Field(i)
		if fields[i] != f.Tag.Get("key") {
			err = errors.New(fmt.Sprintf("Field order error, expecting: %s, got: %s",
				f.Tag.Get("key"), fields[i]))
			return
		}
	}

	return
}

func decodeVol(fields []string) *VolInfo {
	var result VolInfo
	v := reflect.Indirect(reflect.ValueOf(&result))

	if v.NumField() != len(fields) {
		panic("Extra rows from lvs have inconsitent number of fields")
	}

	for i := 0; i < len(fields); i++ {
		f := v.Field(i)
		f.SetString(fields[i])
	}

	return &result
}

func (lv *LVInfo) HasSnap(vg VgName) (present bool) {
	_, present = lv.ByName[vg]
	return
}
