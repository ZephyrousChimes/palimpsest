package okf

import (
	"os"
	"testing"
)

func TestWrite(t *testing.T) {
	c := Concept{
		Id:          "concepts/test.md",
		Type:        "fact",
		Title:       "Test Concept",
		Description: "A concept created purely to test Write",
		Body:        "This is the body.",
	}

	err := c.Write()
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// verify the file actually exists on disk
	path := WikiRoot() + "/" + c.Id
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatalf("expected file at %s, but it does not exist", path)
	}

	os.RemoveAll(WikiRoot())
}

func TestWriteAndRead(t *testing.T) {
	c := Concept{
		Id:          "concepts/test.md",
		Type:        "fact",
		Title:       "Test Concept",
		Description: "A concept created purely to test Write",
		Body:        "This is the body.",
	}

	if werr := c.Write(); werr != nil {
		t.Fatalf("Write failed: %v", werr)
	}

	rc, rerr := Read("concepts/test.md")
	if rerr != nil {
		t.Fatalf("Read failed: %v", rerr)
	}

	if c.Id != rc.Id || c.Type != rc.Type || c.Title != rc.Title || c.Description != rc.Description || c.Body != rc.Body {
		t.Fatalf("expected: %+v\ngot: %+v", c, *rc)
	}

	os.RemoveAll(WikiRoot())
}