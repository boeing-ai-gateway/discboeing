package hvsocket

import "testing"

func TestPortToServiceID(t *testing.T) {
	id, err := PortToServiceID(5000)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := id.String(), "00001388-facb-11e6-bd58-64006a7986d3"; got != want {
		t.Fatalf("service ID mismatch: got %s, want %s", got, want)
	}
}

func TestPortToServiceIDRejectsInvalidPorts(t *testing.T) {
	for _, port := range []int{0, -1, 0x80000000} {
		if _, err := PortToServiceID(port); err == nil {
			t.Fatalf("expected port %d to fail", port)
		}
	}
}
