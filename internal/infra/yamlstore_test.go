package infra

import (
	"os"
	"path/filepath"
	"testing"
)

type testData struct {
	Name  string `yaml:"name"`
	Value int    `yaml:"value"`
}

func TestYAMLStore_WriteAndRead(t *testing.T) {
	store := NewYAMLStore()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.yaml")

	original := testData{Name: "hello", Value: 42}
	if err := store.Write(path, original); err != nil {
		t.Fatalf("Write: %v", err)
	}

	var got testData
	if err := store.Read(path, &got); err != nil {
		t.Fatalf("Read: %v", err)
	}

	if got.Name != original.Name || got.Value != original.Value {
		t.Errorf("Read = %+v, want %+v", got, original)
	}
}

func TestYAMLStore_ReadNonexistent(t *testing.T) {
	store := NewYAMLStore()
	dir := t.TempDir()
	path := filepath.Join(dir, "does-not-exist.yaml")

	var got testData
	if err := store.Read(path, &got); err != nil {
		t.Fatalf("Read nonexistent file should not error, got: %v", err)
	}

	// dest は変更されないことを確認
	if got.Name != "" || got.Value != 0 {
		t.Errorf("Read nonexistent file should leave dest unchanged, got %+v", got)
	}
}

func TestYAMLStore_WriteCreatesDirectories(t *testing.T) {
	store := NewYAMLStore()
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "deep", "test.yaml")

	data := testData{Name: "nested", Value: 1}
	if err := store.Write(path, data); err != nil {
		t.Fatalf("Write to nested dir: %v", err)
	}

	if !store.Exists(path) {
		t.Error("file should exist after Write")
	}
}

func TestYAMLStore_WriteFilePermissions(t *testing.T) {
	store := NewYAMLStore()
	dir := t.TempDir()
	path := filepath.Join(dir, "perms.yaml")

	data := testData{Name: "perms", Value: 0}
	if err := store.Write(path, data); err != nil {
		t.Fatalf("Write: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}

	perm := info.Mode().Perm()
	if perm != 0600 {
		t.Errorf("file permission = %o, want %o", perm, 0600)
	}
}

func TestYAMLStore_WriteIsAtomic(t *testing.T) {
	store := NewYAMLStore()
	dir := t.TempDir()
	path := filepath.Join(dir, "atomic.yaml")

	data := testData{Name: "atomic", Value: 99}
	if err := store.Write(path, data); err != nil {
		t.Fatalf("Write: %v", err)
	}

	// 書き込み成功後、一時ファイルが残っていないことを確認
	tmpPath := path + ".tmp"
	if _, err := os.Stat(tmpPath); err == nil {
		t.Error("temporary file should not remain after successful Write")
	}

	// 最終ファイルが正しい内容を持つことを確認
	var got testData
	if err := store.Read(path, &got); err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got.Name != data.Name || got.Value != data.Value {
		t.Errorf("Read = %+v, want %+v", got, data)
	}
}

func TestYAMLStore_Exists(t *testing.T) {
	store := NewYAMLStore()
	dir := t.TempDir()

	existingPath := filepath.Join(dir, "exists.yaml")
	if err := store.Write(existingPath, testData{Name: "exists"}); err != nil {
		t.Fatalf("Write: %v", err)
	}

	if !store.Exists(existingPath) {
		t.Error("Exists should return true for existing file")
	}

	nonExistingPath := filepath.Join(dir, "nope.yaml")
	if store.Exists(nonExistingPath) {
		t.Error("Exists should return false for non-existing file")
	}
}
