package infra

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// YAMLStore は YAML ファイルの読み書きを担う。
type YAMLStore interface {
	// Read はファイルを読み込み dest にデシリアライズする。
	// ファイルが存在しない場合はエラーを返さず、dest は変更されない。
	Read(path string, dest interface{}) error

	// Write はデータを YAML としてファイルに書き込む。
	// 親ディレクトリが存在しない場合は作成する。パーミッションは 0600。
	Write(path string, data interface{}) error

	// Exists はファイルが存在するかを返す。
	Exists(path string) bool
}

type yamlStore struct{}

// NewYAMLStore は YAMLStore の実装を返す。
func NewYAMLStore() YAMLStore {
	return &yamlStore{}
}

func (s *yamlStore) Read(path string, dest interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	return yaml.Unmarshal(data, dest)
}

func (s *yamlStore) Write(path string, data interface{}) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	buf, err := yaml.Marshal(data)
	if err != nil {
		return err
	}

	// アトミック書き込み: 一時ファイルに書き込み、その後リネームする。
	// これにより書き込み中のクラッシュでファイルが壊れることを防ぐ。
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, buf, 0600); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

func (s *yamlStore) Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
