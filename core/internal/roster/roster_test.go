package roster

import (
	"testing"

	"github.com/SeijiShii/link-self/core/internal/did"
)

func deviceEntry(t *testing.T, label string) DeviceEntry {
	t.Helper()
	id, err := did.Generate()
	if err != nil {
		t.Fatalf("generate device identity: %v", err)
	}
	return DeviceEntry{DeviceDID: id.DID, Label: label}
}

func TestBuildAndVerify(t *testing.T) {
	user, err := did.Generate()
	if err != nil {
		t.Fatal(err)
	}
	d1 := deviceEntry(t, "PC")
	d2 := deviceEntry(t, "Phone")
	r, err := Build(user, []DeviceEntry{d1, d2})
	if err != nil {
		t.Fatal(err)
	}
	if r.UserDID != user.DID {
		t.Errorf("UserDID = %q, want %q", r.UserDID, user.DID)
	}
	if !HasDevice(r, d1.DeviceDID) || !HasDevice(r, d2.DeviceDID) {
		t.Error("roster missing a device")
	}
	if !Verify(r) {
		t.Error("Verify() = false, want true")
	}
}

func TestSignatureIsOrderIndependent(t *testing.T) {
	user, _ := did.Generate()
	d1 := deviceEntry(t, "A")
	d2 := deviceEntry(t, "B")
	r1, _ := Build(user, []DeviceEntry{d1, d2})
	r2, _ := Build(user, []DeviceEntry{d2, d1})
	if string(r1.Sig) != string(r2.Sig) {
		t.Error("signatures differ for reordered devices")
	}
}

func TestDedupeLastWins(t *testing.T) {
	user, _ := did.Generate()
	d := deviceEntry(t, "old")
	r, err := Build(user, []DeviceEntry{d, {DeviceDID: d.DeviceDID, Label: "new"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Devices) != 1 {
		t.Fatalf("len(Devices) = %d, want 1", len(r.Devices))
	}
	if r.Devices[0].Label != "new" {
		t.Errorf("Label = %q, want new", r.Devices[0].Label)
	}
	if !Verify(r) {
		t.Error("Verify() = false")
	}
}

func TestRejectImpostorUserDID(t *testing.T) {
	user, _ := did.Generate()
	impostor, _ := did.Generate()
	r, _ := Build(user, []DeviceEntry{deviceEntry(t, "PC")})
	r.UserDID = impostor.DID // keep signature, claim a different user
	if Verify(r) {
		t.Error("Verify() = true for impostor userDID, want false")
	}
}

func TestRejectTamperedDevices(t *testing.T) {
	user, _ := did.Generate()
	r, _ := Build(user, []DeviceEntry{deviceEntry(t, "PC")})
	r.Devices = append(r.Devices, deviceEntry(t, "rogue"))
	if Verify(r) {
		t.Error("Verify() = true for tampered device list, want false")
	}
}

func TestWithAndWithoutDevice(t *testing.T) {
	user, _ := did.Generate()
	d1 := deviceEntry(t, "PC")
	d2 := deviceEntry(t, "Phone")

	r, _ := Build(user, []DeviceEntry{d1})
	r, err := WithDevice(user, r.Devices, d2)
	if err != nil {
		t.Fatal(err)
	}
	if !HasDevice(r, d2.DeviceDID) || !Verify(r) {
		t.Error("WithDevice failed")
	}

	r, err = WithoutDevice(user, r.Devices, d1.DeviceDID)
	if err != nil {
		t.Fatal(err)
	}
	if HasDevice(r, d1.DeviceDID) {
		t.Error("device not removed")
	}
	if !HasDevice(r, d2.DeviceDID) || !Verify(r) {
		t.Error("WithoutDevice failed")
	}
}

func TestMarshalRoundTrip(t *testing.T) {
	user, _ := did.Generate()
	r, _ := Build(user, []DeviceEntry{deviceEntry(t, "PC"), deviceEntry(t, "Phone")})
	data, err := Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Unmarshal(data)
	if err != nil {
		t.Fatal(err)
	}
	if got.UserDID != r.UserDID || len(got.Devices) != len(r.Devices) {
		t.Error("round-trip mismatch")
	}
	if !Verify(got) {
		t.Error("Verify() = false after round-trip")
	}
}
