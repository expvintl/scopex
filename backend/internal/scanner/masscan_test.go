package scanner

import (
	"reflect"
	"testing"
)

func TestExtraArgsStripsOutputAndConflictingOptions(t *testing.T) {
	got := extraArgs("--retries 3 -oJ /tmp/x.json --interface eth9 --wait 4")
	want := []string{"--retries", "3", "--wait", "4"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("extraArgs = %v, want %v", got, want)
	}
}

func TestBuildArgs(t *testing.T) {
	m := &Masscan{}
	m.SetOptions(MasscanOptions{
		Interface: "wlan0",
		RouterMAC: "aa:bb:cc:dd:ee:ff",
		ExtraArgs: "--wait 3",
	})
	got := m.buildArgs([]string{"192.168.6.0/24"}, []int{80, 443}, 1000)
	want := []string{
		"--interface", "wlan0",
		"--router-mac", "aa:bb:cc:dd:ee:ff",
		"192.168.6.0/24",
		"-p80,443", "--rate", "1000", "--wait", "2", "--open-only",
		"--wait", "3",
		"-oJ", "-",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("buildArgs = %v, want %v", got, want)
	}
}
