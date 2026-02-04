package ssh

import (
	"testing"
)

func TestManagerGenerateAlias(t *testing.T) {
	mgr := NewManager("", "/")
	defer mgr.Close()

	t.Run("generates user@host format", func(t *testing.T) {
		alias := mgr.generateAlias("admin", "server1.example.com")
		if alias != "admin@server1.example.com" {
			t.Errorf("expected 'admin@server1.example.com', got '%s'", alias)
		}
	})

	t.Run("generates unique aliases with suffix", func(t *testing.T) {
		// Add a mock connection to trigger suffix
		mgr.connections["admin@server1"] = &Client{}

		alias := mgr.generateAlias("admin", "server1")
		if alias != "admin@server1-2" {
			t.Errorf("expected 'admin@server1-2', got '%s'", alias)
		}

		// Add another
		mgr.connections["admin@server1-2"] = &Client{}
		alias = mgr.generateAlias("admin", "server1")
		if alias != "admin@server1-3" {
			t.Errorf("expected 'admin@server1-3', got '%s'", alias)
		}
	})
}

func TestManagerResolveTarget(t *testing.T) {
	mgr := NewManager("", "/")
	defer mgr.Close()

	t.Run("returns error when no connections", func(t *testing.T) {
		_, err := mgr.resolveTarget("primary")
		if err == nil {
			t.Error("expected error for no connections")
		}
	})

	t.Run("returns error for non-existent alias", func(t *testing.T) {
		mgr.connections["test"] = &Client{}
		mgr.primary = "test"

		_, err := mgr.resolveTarget("non-existent")
		if err == nil {
			t.Error("expected error for non-existent alias")
		}
	})

	t.Run("returns primary when target is 'primary'", func(t *testing.T) {
		mgr.connections["myconn"] = &Client{}
		mgr.primary = "myconn"

		alias, err := mgr.resolveTarget("primary")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if alias != "myconn" {
			t.Errorf("expected 'myconn', got '%s'", alias)
		}
	})

	t.Run("returns specific alias when provided", func(t *testing.T) {
		mgr.connections["specific"] = &Client{}

		alias, err := mgr.resolveTarget("specific")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if alias != "specific" {
			t.Errorf("expected 'specific', got '%s'", alias)
		}
	})
}

func TestManagerValidatePath(t *testing.T) {
	mgr := NewManager("", "/data")
	defer mgr.Close()

	t.Run("allows paths within allowed root", func(t *testing.T) {
		path, err := mgr.validatePath("/data/files/test.txt", "")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if path != "/data/files/test.txt" {
			t.Errorf("expected '/data/files/test.txt', got '%s'", path)
		}
	})

	t.Run("rejects paths outside allowed root", func(t *testing.T) {
		_, err := mgr.validatePath("/etc/passwd", "")
		if err == nil {
			t.Error("expected error for path outside allowed root")
		}
	})

	t.Run("resolves relative paths", func(t *testing.T) {
		// Default CWD is "/"
		path, err := mgr.validatePath("data/file.txt", "")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if path != "/data/file.txt" {
			t.Errorf("expected '/data/file.txt', got '%s'", path)
		}
	})
}

func TestManagerListConnections(t *testing.T) {
	mgr := NewManager("", "/")
	defer mgr.Close()

	t.Run("returns empty list when no connections", func(t *testing.T) {
		conns := mgr.ListConnections()
		if len(conns) != 0 {
			t.Errorf("expected empty list, got %d items", len(conns))
		}
	})

	t.Run("returns all connection aliases", func(t *testing.T) {
		mgr.connections["conn1"] = &Client{}
		mgr.connections["conn2"] = &Client{}
		mgr.connections["conn3"] = &Client{}

		conns := mgr.ListConnections()
		if len(conns) != 3 {
			t.Errorf("expected 3 connections, got %d", len(conns))
		}
	})
}

func TestConnectionError(t *testing.T) {
	testCases := []struct {
		errMsg   string
		expected bool
	}{
		{"connection reset by peer", true},
		{"broken pipe", true},
		{"EOF", true},
		{"connection refused", true},
		{"permission denied", false},
		{"file not found", false},
		{"", false},
	}

	for _, tc := range testCases {
		t.Run(tc.errMsg, func(t *testing.T) {
			var err error
			if tc.errMsg != "" {
				err = &testError{msg: tc.errMsg}
			}
			result := isConnectionError(err)
			if result != tc.expected {
				t.Errorf("isConnectionError(%q) = %v, want %v", tc.errMsg, result, tc.expected)
			}
		})
	}
}

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}
