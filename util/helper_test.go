package util

import (
	"testing"
)

func TestContains(t *testing.T) {
	list := []string{"a", "b", "c"}

	if !Contains(list, "a") {
		t.Error("expected 'a' to be found")
	}
	if !Contains(list, "b") {
		t.Error("expected 'b' to be found")
	}
	if Contains(list, "d") {
		t.Error("expected 'd' to not be found")
	}
	if Contains([]string{}, "a") {
		t.Error("expected empty list to not contain anything")
	}
}

func TestQuoteCols(t *testing.T) {
	result := QuoteCols([]string{"id", "name", "email"})
	expected := `"id", "name", "email"`
	if result != expected {
		t.Errorf("expected %s, got %s", expected, result)
	}

	result = QuoteCols([]string{"single"})
	expected = `"single"`
	if result != expected {
		t.Errorf("expected %s, got %s", expected, result)
	}
}

func TestGetPort(t *testing.T) {
	ResetPorts()

	port1 := GetPort("node1")
	port2 := GetPort("node2")
	port1Again := GetPort("node1")

	if port1 != 5433 {
		t.Errorf("expected first port to be 5433, got %d", port1)
	}
	if port2 != 5434 {
		t.Errorf("expected second port to be 5434, got %d", port2)
	}
	if port1 != port1Again {
		t.Errorf("expected same node to return same port, got %d and %d", port1, port1Again)
	}
}
