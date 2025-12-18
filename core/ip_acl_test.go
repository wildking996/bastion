package core

import (
	"net"
	"testing"
)

func TestIPAccessControl_Allows(t *testing.T) {
	acl, err := NewIPAccessControl([]string{"10.0.0.0/8"}, nil)
	if err != nil {
		t.Fatalf("NewIPAccessControl: %v", err)
	}
	if acl.Allows(net.ParseIP("10.1.2.3")) != true {
		t.Fatalf("expected allow for 10.1.2.3")
	}
	if acl.Allows(net.ParseIP("11.1.2.3")) != false {
		t.Fatalf("expected deny for 11.1.2.3")
	}
}

func TestIPAccessControl_DenyWins(t *testing.T) {
	acl, err := NewIPAccessControl([]string{"10.0.0.0/8"}, []string{"10.1.2.3"})
	if err != nil {
		t.Fatalf("NewIPAccessControl: %v", err)
	}
	if acl.Allows(net.ParseIP("10.1.2.3")) != false {
		t.Fatalf("expected deny for 10.1.2.3")
	}
	if acl.Allows(net.ParseIP("10.1.2.4")) != true {
		t.Fatalf("expected allow for 10.1.2.4")
	}
}

func TestIPAccessControl_NilAllowsAll(t *testing.T) {
	var acl *IPAccessControl
	if acl.Allows(net.ParseIP("1.2.3.4")) != true {
		t.Fatalf("expected allow when acl is nil")
	}
}
