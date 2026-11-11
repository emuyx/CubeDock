package core

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/df-mc/jsonc"

	"github.com/emuyx/cubedock/internal/config"
)

// ResourcePack is one entry shown in the console's limbo pack list.
type ResourcePack struct {
	File    string `json:"file"`
	Name    string `json:"name"`
	UUID    string `json:"uuid"`
	Version string `json:"version"`
	Size    int64  `json:"size"`
	Error   string `json:"error,omitempty"`
}

// ResourcePackList is the result of scanning the limbo resource pack directory.
type ResourcePackList struct {
	Dir   string         `json:"dir"`
	Packs []ResourcePack `json:"packs"`
}

// ResourcePackRefresh is returned after pulling packs from the backend server
// and removing exact duplicates.
type ResourcePackRefresh struct {
	ResourcePackList
	Pulled   int      `json:"pulled"`
	Skipped  int      `json:"skipped"`
	Removed  []string `json:"removed"`
	Warnings []string `json:"warnings,omitempty"`
	Output   string   `json:"output,omitempty"`
}

// resolveProfile returns the requested profile, falling back to the default
// when the caller does not name one.
func (m *Manager) resolveProfile(profileID string) (config.Profile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if strings.TrimSpace(profileID) == "" {
		profileID = m.cfg.DefaultProfileID()
	}
	profile, ok := m.cfg.Profile(profileID)
	if !ok {
		return config.Profile{}, fmt.Errorf("配置方案不存在")
	}
	return profile, nil
}

// ResourcePacks scans one profile's limbo resource pack directory. limbo loads
// packs only when it starts, so this list reflects the directory contents, not
// necessarily the running limbo instance.
func (m *Manager) ResourcePacks(profileID string) (ResourcePackList, error) {
	profile, err := m.resolveProfile(profileID)
	if err != nil {
		return ResourcePackList{}, err
	}
	dir, err := EnsureLimboPacksDir(m.dir, profile)
	if err != nil {
		return ResourcePackList{Dir: dir}, err
	}
	return scanResourcePacks(dir)
}

// RefreshResourcePacks pulls one profile's packs from its backend server as a
// bot, removes exact duplicates, and returns the resulting directory listing.
func (m *Manager) RefreshResourcePacks(ctx context.Context, profileID string) (ResourcePackRefresh, error) {
	var result ResourcePackRefresh
	profile, err := m.resolveProfile(profileID)
	if err != nil {
		return result, err
	}
	if profile.ConnectionMode != config.ConnectionTransfer {
		return result, fmt.Errorf("只有转服模式使用资源包")
	}
	dir, err := EnsureLimboPacksDir(m.dir, profile)
	if err != nil {
		return result, err
	}
	name, uid := m.packIdentity(profile.ID)
	output, err := m.pullFn(ctx, profile, name, uid)
	if err != nil {
		return result, err
	}
	result.Output = output
	var summary struct {
		Pulled  int `json:"pulled"`
		Skipped int `json:"skipped"`
	}
	for _, line := range strings.Split(output, "\n") {
		if json.Unmarshal([]byte(strings.TrimSpace(line)), &summary) == nil {
			break
		}
	}
	result.Pulled = summary.Pulled
	result.Skipped = summary.Skipped
	removed, warnings, err := dedupeResourcePacks(dir)
	if err != nil {
		return result, err
	}
	result.Removed = removed
	result.Warnings = warnings
	list, err := scanResourcePacks(dir)
	if err != nil {
		return result, err
	}
	result.ResourcePackList = list
	m.mu.Lock()
	m.preparedPacks[profile.ID] = true
	m.mu.Unlock()
	return result, nil
}

func scanResourcePacks(dir string) (ResourcePackList, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ResourcePackList{Dir: dir}, fmt.Errorf("读取资源包目录失败: %w", err)
	}
	list := ResourcePackList{Dir: dir, Packs: make([]ResourcePack, 0, len(entries))}
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		path := filepath.Join(dir, name)
		pack := ResourcePack{File: name, Name: name}
		size, manifest, err := readPackManifest(path, entry)
		pack.Size = size
		if err != nil {
			pack.Error = err.Error()
			list.Packs = append(list.Packs, pack)
			continue
		}
		if manifest.Header.Name != "" {
			pack.Name = manifest.Header.Name
		}
		pack.UUID = manifest.Header.UUID
		pack.Version = formatPackVersion(manifest.Header.Version)
		list.Packs = append(list.Packs, pack)
	}
	return list, nil
}

type packManifest struct {
	Header struct {
		Name    string `json:"name"`
		UUID    string `json:"uuid"`
		Version []int  `json:"version"`
	} `json:"header"`
}

func readPackManifest(path string, entry fs.DirEntry) (int64, *packManifest, error) {
	if entry.IsDir() {
		return readDirectoryPack(path)
	}
	return readZipPack(path)
}

func readZipPack(path string) (int64, *packManifest, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, nil, err
	}
	reader, err := zip.OpenReader(path)
	if err != nil {
		return info.Size(), nil, fmt.Errorf("不是有效的 .mcpack/.zip: %w", err)
	}
	defer reader.Close()
	for _, file := range reader.File {
		if !strings.EqualFold(filepath.Base(file.Name), "manifest.json") {
			continue
		}
		read, err := file.Open()
		if err != nil {
			return info.Size(), nil, err
		}
		data, err := io.ReadAll(io.LimitReader(read, 1<<20))
		_ = read.Close()
		if err != nil {
			return info.Size(), nil, err
		}
		manifest, err := parsePackManifest(data)
		return info.Size(), manifest, err
	}
	return info.Size(), nil, fmt.Errorf("未找到 manifest.json")
}

func readDirectoryPack(path string) (int64, *packManifest, error) {
	var manifestPath string
	var size int64
	err := filepath.WalkDir(path, func(current string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if info, err := entry.Info(); err == nil {
			size += info.Size()
		}
		if manifestPath == "" && strings.EqualFold(entry.Name(), "manifest.json") {
			manifestPath = current
		}
		return nil
	})
	if err != nil {
		return size, nil, err
	}
	if manifestPath == "" {
		return size, nil, fmt.Errorf("未找到 manifest.json")
	}
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return size, nil, err
	}
	manifest, err := parsePackManifest(data)
	return size, manifest, err
}

func parsePackManifest(data []byte) (*packManifest, error) {
	var manifest packManifest
	if err := jsonc.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("manifest.json 解析失败: %w", err)
	}
	if manifest.Header.UUID == "" && manifest.Header.Name == "" {
		return nil, fmt.Errorf("manifest.json 缺少 header")
	}
	return &manifest, nil
}

func formatPackVersion(version []int) string {
	if len(version) < 3 {
		return ""
	}
	return fmt.Sprintf("%d.%d.%d", version[0], version[1], version[2])
}

type packFileInfo struct {
	name string
	path string
	hash [32]byte
	mod  time.Time
}

// managedPackName matches the "<uuid>_<major>.<minor>.<patch>.mcpack" names
// CubeDock writes when it pulls packs. Dedupe only deletes files with this
// shape, so a user's own files in a shared pack directory are never removed.
var managedPackName = regexp.MustCompile(
	`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}_[0-9]+\.[0-9]+\.[0-9]+\.mcpack$`)

// dedupeResourcePacks removes exact duplicate zip packs (same manifest UUID,
// version and SHA-256), keeping the most recently modified file. Packs with
// the same UUID/version but different contents are kept and reported as a
// warning rather than deleted.
func dedupeResourcePacks(dir string) (removed, warnings []string, err error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, fmt.Errorf("读取资源包目录失败: %w", err)
	}
	groups := make(map[string][]packFileInfo)
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || strings.HasPrefix(name, ".") || entry.Type()&os.ModeSymlink != 0 {
			continue
		}
		path := filepath.Join(dir, name)
		_, manifest, err := readZipPack(path)
		if err != nil || manifest.Header.UUID == "" {
			continue
		}
		version := formatPackVersion(manifest.Header.Version)
		if version == "" {
			continue
		}
		file, err := os.Open(path)
		if err != nil {
			continue
		}
		hash := sha256.New()
		_, copyErr := io.Copy(hash, file)
		_ = file.Close()
		if copyErr != nil {
			continue
		}
		var sum [32]byte
		copy(sum[:], hash.Sum(nil))
		info, err := entry.Info()
		if err != nil {
			continue
		}
		key := manifest.Header.UUID + "_" + version
		groups[key] = append(groups[key], packFileInfo{
			name: name,
			path: path,
			hash: sum,
			mod:  info.ModTime(),
		})
	}
	for key, files := range groups {
		if len(files) < 2 {
			continue
		}
		byHash := make(map[[32]byte][]packFileInfo)
		for _, file := range files {
			byHash[file.hash] = append(byHash[file.hash], file)
		}
		if len(byHash) > 1 {
			warnings = append(warnings, fmt.Sprintf("%s 有 %d 个内容不同的资源包，未删除", key, len(byHash)))
		}
		canonical := key + ".mcpack"
		for _, same := range byHash {
			if len(same) < 2 {
				continue
			}
			sort.Slice(same, func(i, j int) bool {
				canonicalI, canonicalJ := same[i].name == canonical, same[j].name == canonical
				if canonicalI != canonicalJ {
					return canonicalI
				}
				return same[i].mod.After(same[j].mod)
			})
			for _, file := range same[1:] {
				if !managedPackName.MatchString(file.name) {
					continue
				}
				if err := os.Remove(file.path); err != nil {
					warnings = append(warnings, fmt.Sprintf("删除重复包 %s 失败: %v", file.name, err))
					continue
				}
				removed = append(removed, file.name)
			}
		}
	}
	sort.Strings(removed)
	sort.Strings(warnings)
	return removed, warnings, nil
}
