package yamlstore

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/yaml.v3"
)

// YAMLStore は YAML ファイルの読み書きを担う。
type YAMLStore interface {
	// Read はファイルを読み込み dest にデシリアライズする。
	// ファイルが存在しない場合はエラーを返さず、dest は変更されない。
	Read(path string, dest interface{}) error

	// Write はデータを YAML としてファイルに書き込む。
	// 親ディレクトリが存在しない場合は作成する。パーミッションは 0600。
	// 直前の Read が失敗していたファイルは、上書きの前に <path>.bak（0600）へ退避する。
	// 退避できない場合は元のファイルを失わないよう書き込みを拒否する。
	Write(path string, data interface{}) error

	// Exists はファイルが存在するかを返す。
	Exists(path string) bool
}

// backupSuffix は読み込めなかったファイルの退避先に付ける拡張子。
const backupSuffix = ".bak"

type yamlStore struct {
	mu         sync.Mutex
	unreadable map[string]error // 直近の Read が失敗したパスとそのエラー
}

// NewYAMLStore は YAMLStore の実装を返す。
func NewYAMLStore() YAMLStore {
	return &yamlStore{unreadable: make(map[string]error)}
}

func (s *yamlStore) setUnreadable(path string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err == nil {
		delete(s.unreadable, path)
		return
	}
	s.unreadable[path] = err
}

// backupIfUnreadable は直前の Read に失敗したファイルを <path>.bak へ退避する。
// 構文エラーなどで読めなかった設定を、デフォルト値での上書きで失わないための措置。
func (s *yamlStore) backupIfUnreadable(path string) error {
	s.mu.Lock()
	readErr, ok := s.unreadable[path]
	s.mu.Unlock()
	if !ok {
		return nil
	}

	data, err := os.ReadFile(path) //nolint:gosec // path はアプリケーション内部で管理されるファイルパス
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("refusing to overwrite unreadable %s (read error: %v): %w", path, readErr, err)
	}
	backupPath := path + backupSuffix
	if err := os.WriteFile(backupPath, data, 0600); err != nil { //nolint:gosec // backupPath はアプリケーション内部で管理される設定ファイルパスから導出
		return fmt.Errorf("refusing to overwrite %s (read error: %v): back up to %s: %w", path, readErr, backupPath, err)
	}
	slog.Warn("file could not be read; original backed up before overwriting",
		"path", path, "backup", backupPath, "read_error", readErr)
	return nil
}

func (s *yamlStore) Read(path string, dest interface{}) error {
	data, err := os.ReadFile(path) //nolint:gosec // path はアプリケーション内部で管理されるファイルパス
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			s.setUnreadable(path, nil)
			return nil
		}
		s.setUnreadable(path, err)
		return err
	}
	err = yaml.Unmarshal(data, dest)
	s.setUnreadable(path, err)
	return err
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
	if err := s.backupIfUnreadable(path); err != nil {
		return err
	}

	// アトミック書き込み: 一時ファイルに書き込み、その後リネームする。
	// これにより書き込み中のクラッシュでファイルが壊れることを防ぐ。
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, buf, 0600); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	s.setUnreadable(path, nil)
	return nil
}

func (s *yamlStore) Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
