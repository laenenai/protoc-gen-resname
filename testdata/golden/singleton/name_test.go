package singleton

import "testing"

func TestSingleton(t *testing.T) {
	n, err := ParseUserConfigName("users/alice/config")
	if err != nil || n != (UserConfigName{User: "alice"}) {
		t.Fatalf("ParseUserConfigName = %+v, %v", n, err)
	}
	if n.String() != "users/alice/config" || n.Parent() != (UserName{User: "alice"}) {
		t.Fatalf("String()/Parent() = %q / %+v", n.String(), n.Parent())
	}
	for in, want := range map[string]string{
		"users/alice":          `invalid resource name: userConfig name "users/alice": expected 3 segments, got 2`,
		"users/alice/settings": `invalid resource name: userConfig name "users/alice/settings": expected "config" at segment 3, got "settings"`,
		"users//config":        `invalid resource name: userConfig name "users//config": segment {user} is empty`,
	} {
		if _, err := ParseUserConfigName(in); err == nil || err.Error() != want {
			t.Errorf("ParseUserConfigName(%q) = %v\n   want %s", in, err, want)
		}
	}
	if c, err := (&GetUserConfigRequest{Name: "users/bob/config"}).UserConfigName(); err != nil || c.User != "bob" {
		t.Fatalf("UserConfigName() = %+v, %v", c, err)
	}
}
