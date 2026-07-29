package log

import (
	"fmt"
	"testing"
)

func TestBufferGroupAppendAndGet(t *testing.T) {
	bg := NewBufferGroup(10)

	bg.Append("api", Entry{Time: "t1", Level: "INFO", Message: "test", Count: 1})

	entries := bg.Entries(10, "", "api", "")
	if len(entries) != 1 {
		t.Fatalf("Entries count = %d, want 1", len(entries))
	}
	if entries[0].Message != "test" {
		t.Errorf("Message = %s, want test", entries[0].Message)
	}
}

func TestBufferGroupNonExistent(t *testing.T) {
	bg := NewBufferGroup(10)
	entries := bg.Entries(10, "", "nonexistent", "")
	if entries != nil {
		t.Errorf("Entries for non-existent should be nil, got %v", entries)
	}
}

func TestBufferGroupMergeAll(t *testing.T) {
	bg := NewBufferGroup(10)
	bg.Append("api", Entry{Time: "t1", Level: "INFO", Message: "api msg"})
	bg.Append("bot", Entry{Time: "t2", Level: "INFO", Message: "bot msg"})

	entries := bg.Entries(10, "", "", "")
	if len(entries) != 2 {
		t.Errorf("All entries count = %d, want 2", len(entries))
	}
}

func TestBufferGroupLevelFilter(t *testing.T) {
	bg := NewBufferGroup(10)
	bg.Append("api", Entry{Time: "t1", Level: "INFO", Message: "info msg"})
	bg.Append("api", Entry{Time: "t2", Level: "ERROR", Message: "error msg"})

	entries := bg.Entries(10, "ERROR", "api", "")
	if len(entries) != 1 {
		t.Fatalf("ERROR entries count = %d, want 1", len(entries))
	}
	if entries[0].Level != "ERROR" {
		t.Errorf("Level = %s, want ERROR", entries[0].Level)
	}
}

func TestBufferGroupLimit(t *testing.T) {
	bg := NewBufferGroup(10)
	for i := 0; i < 20; i++ {
		bg.Append("api", Entry{Time: "t", Level: "INFO", Message: "msg"})
	}

	entries := bg.Entries(5, "", "api", "")
	if len(entries) > 5 {
		t.Errorf("Entries count = %d, want <= 5", len(entries))
	}
}

func TestBufferGroupDedup(t *testing.T) {
	bg := NewBufferGroup(10)
	bg.Append("api", Entry{Time: "t1", Level: "INFO", Message: "same msg",
		Attrs: map[string]interface{}{"cmp": "api"}})
	bg.Append("api", Entry{Time: "t2", Level: "INFO", Message: "same msg",
		Attrs: map[string]interface{}{"cmp": "api"}})

	entries := bg.Entries(10, "", "api", "")
	if len(entries) != 1 {
		t.Fatalf("After dedup, entries count = %d, want 1", len(entries))
	}
	if entries[0].Count != 2 {
		t.Errorf("Count = %d, want 2", entries[0].Count)
	}
}

func TestBufferGroupGetComponents(t *testing.T) {
	bg := NewBufferGroup(10)
	bg.Append("api", Entry{Time: "t1", Level: "INFO", Message: "api msg"})
	bg.Append("bot", Entry{Time: "t2", Level: "INFO", Message: "bot msg"})

	components, backends := bg.GetComponents()
	if len(components) != 2 {
		t.Errorf("Components count = %d, want 2", len(components))
	}
	if len(backends) != 0 {
		t.Errorf("Backends count = %d, want 0", len(backends))
	}
}
func TestBufferCapacity(t *testing.T) {
	buf := NewBuffer(5)
	for i := 0; i < 10; i++ {
		buf.Append(Entry{Time: "t", Level: "INFO", Message: fmt.Sprintf("msg %d", i)})
	}
	entries := buf.Snap()
	if len(entries) != 5 {
		t.Errorf("After capacity overflow, entries = %d, want 5", len(entries))
	}
}

func TestBufferSnap(t *testing.T) {
	buf := NewBuffer(10)
	buf.Append(Entry{Time: "t1", Level: "INFO", Message: "first"})
	buf.Append(Entry{Time: "t2", Level: "INFO", Message: "second"})

	entries := buf.Snap()
	if len(entries) != 2 {
		t.Fatalf("Snap count = %d, want 2", len(entries))
	}
	if entries[0].Message != "first" || entries[1].Message != "second" {
		t.Errorf("Snap order wrong: %+v", entries)
	}
}

func TestBufferSnapEmpty(t *testing.T) {
	buf := NewBuffer(10)
	entries := buf.Snap()
	if entries != nil {
		t.Errorf("Snap of empty buffer = %v, want nil", entries)
	}
}

func TestNewBufferGroupDefaultCapacity(t *testing.T) {
	bg := NewBufferGroup(0)
	if bg.capacity != DefaultBufferCapacity {
		t.Errorf("capacity = %d, want %d", bg.capacity, DefaultBufferCapacity)
	}
}

func TestAttrsEqual(t *testing.T) {
	tests := []struct {
		name string
		a, b any
		want bool
	}{
		{"both nil", nil, nil, true},
		{"one nil", map[string]interface{}{"k": "v"}, nil, false},
		{"equal maps", map[string]interface{}{"k": "v"}, map[string]interface{}{"k": "v"}, true},
		{"different values", map[string]interface{}{"k": "v1"}, map[string]interface{}{"k": "v2"}, false},
		{"different keys", map[string]interface{}{"k1": "v"}, map[string]interface{}{"k2": "v"}, false},
		{"different lengths", map[string]interface{}{"k1": "v"}, map[string]interface{}{"k1": "v", "k2": "v2"}, false},
		{"slice values", map[string]interface{}{"args": []string{"a", "b"}}, map[string]interface{}{"args": []string{"a", "b"}}, true},
		{"different slices", map[string]interface{}{"args": []string{"a"}}, map[string]interface{}{"args": []string{"b"}}, false},
		{"not a map", "string", "string", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := attrsEqual(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("attrsEqual(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestWithComponentReplacesExisting(t *testing.T) {
	l := New("info")
	// Chain WithComponent - should replace, not accumulate
	l2 := l.WithComponent("main").WithComponent("api")
	if len(l2.preAttrs) != 1 {
		t.Errorf("preAttrs count = %d, want 1 (replaced)", len(l2.preAttrs))
	}
	if l2.preAttrs[0].Key != "cmp" || l2.preAttrs[0].Value.Any() != "api" {
		t.Errorf("preAttrs = %+v, want [{cmp api}]", l2.preAttrs)
	}
}