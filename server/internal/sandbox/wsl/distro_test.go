package wsl

import "testing"

func TestParseDistroList(t *testing.T) {
	output := `  NAME                   STATE           VERSION
* Ubuntu                 Running         2
  discobot               Stopped         2
  Docker Desktop Data    Stopped         2
  My Custom Distro       Running         1
`

	distros, err := ParseDistroList(output)
	if err != nil {
		t.Fatalf("ParseDistroList() error = %v", err)
	}
	if len(distros) != 4 {
		t.Fatalf("ParseDistroList() len = %d, want 4", len(distros))
	}

	ubuntu := distros[0]
	if ubuntu.Name != "Ubuntu" || ubuntu.State != "Running" || ubuntu.Version != 2 || !ubuntu.IsDefault {
		t.Fatalf("unexpected Ubuntu parse result: %#v", ubuntu)
	}

	custom := distros[3]
	if custom.Name != "My Custom Distro" || custom.State != "Running" || custom.Version != 1 {
		t.Fatalf("unexpected custom distro parse result: %#v", custom)
	}
}

func TestParseDistroListSkipsNonEntries(t *testing.T) {
	output := "Windows Subsystem for Linux has no installed distributions.\n"
	distros, err := ParseDistroList(output)
	if err != nil {
		t.Fatalf("ParseDistroList() error = %v", err)
	}
	if len(distros) != 0 {
		t.Fatalf("ParseDistroList() len = %d, want 0", len(distros))
	}
}

func TestFindDistro(t *testing.T) {
	distros := []DistroInfo{{Name: "discobot", State: "Stopped", Version: 2}}

	got, ok := FindDistro(distros, "DiscoBot")
	if !ok {
		t.Fatal("FindDistro() ok = false, want true")
	}
	if got.Name != "discobot" {
		t.Fatalf("FindDistro() name = %q, want discobot", got.Name)
	}
}
