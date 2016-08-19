package main

import "testing"

func TestSplitCmdName(t *testing.T) {
	if pack, name, explicit := SplitCmdName("pack.name");
	!(pack == "pack" && name == "name" && explicit) {
		t.Errorf("Expected 'pack.name true', got '%s.%s %t'", pack, name, explicit)
	}

	if pack, name, explicit := SplitCmdName(".name");
	!(pack == "" && name == "name" && explicit) {
		t.Errorf("Expected '.name false', got '%s.%s %t'", pack, name, explicit)
	}

	if pack, name, explicit := SplitCmdName("name");
	!(pack == "" && name == "name" && !explicit) {
		t.Errorf("Expected '.name true', got '%s.%s %t'", pack, name, explicit)
	}
}

func TestCountVerbs(t *testing.T) {
	if n := CountVerbs("%s"); n != 1 {
		t.Error("Expected 1, got", n)
	}

	if n := CountVerbs("%[1]s"); n != 1 {
		t.Error("Expected 1, got", n)
	}

	if n := CountVerbs("%[2]s"); n != 2 {
		t.Error("Expected 2, got", n)
	}
}

func TestRemoveUnsupportedVerbs(t *testing.T) {
	if s := RemoveUnsupportedVerbs("%d"); !(s == "%s") {
		t.Error("Expected %s, got", s)
	}
}