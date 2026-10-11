// Package imagestore 管理本地磁盘上的图片文件，数据库只保存相对 Root 的路径。
//
// 目录结构：
//
//	items/{itemId}/{defId}/{name}.jpg   物品属性图片，随物品/属性删除按目录级联清理
//	tmp/{userId}/{name}                 上传暂存区（图片、导入数据包），保存物品后转正，定期清理
//
// Package imagestore manages image files on local disk; the database only stores paths relative to Root.
//
// Layout:
//
//	items/{itemId}/{defId}/{name}.jpg   item attribute images, removed per directory with the item/attribute
//	tmp/{userId}/{name}                 upload staging (images, import packages), promoted on item save, swept periodically
package imagestore

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"private/construction_warehouse/internal/model"
)

var ErrBadPath = errors.New("非法的文件路径")

var safeFileName = regexp.MustCompile(`^[A-Za-z0-9._-]{1,128}$`)

type Store struct {
	Root string
}

func New(root string) (*Store, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(abs, "tmp"), 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(abs, "items"), 0o755); err != nil {
		return nil, err
	}
	return &Store{Root: abs}, nil
}

// Abs 把相对路径转换为磁盘绝对路径，拒绝任何越出 Root 的路径。
// Abs converts a relative path into an absolute one and rejects anything escaping Root.
func (s *Store) Abs(rel string) (string, error) {
	clean := path.Clean("/" + rel)
	if clean == "/" || strings.Contains(rel, "\\") {
		return "", ErrBadPath
	}
	return filepath.Join(s.Root, filepath.FromSlash(clean[1:])), nil
}

func randomName(ext string) string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b) + ext
}

func tmpPrefix(userID int64) string { return fmt.Sprintf("tmp/%d/", userID) }

// SaveTmp 把上传内容写入用户暂存区，返回相对路径；超过 limit 字节返回错误。
// SaveTmp writes an upload into the user's staging area and returns its relative path; errors past limit bytes.
func (s *Store) SaveTmp(userID int64, src io.Reader, ext string, limit int64) (string, error) {
	rel := tmpPrefix(userID) + randomName(ext)
	if err := s.writeFile(rel, src, limit); err != nil {
		return "", err
	}
	return rel, nil
}

// IsUserTmp 判断相对路径是否位于该用户的暂存区内。
// IsUserTmp reports whether rel lives in this user's staging area.
func (s *Store) IsUserTmp(userID int64, rel string) bool {
	prefix := tmpPrefix(userID)
	return strings.HasPrefix(rel, prefix) && safeFileName.MatchString(strings.TrimPrefix(rel, prefix))
}

// Commit 把暂存图片转正到 items/{itemId}/{defId}/ 下并返回新的相对路径。
// 不清理目录中的旧图：调用方在数据库事务成功后再调用 PruneAttrDir，失败时删除新图即可回滚。
//
// Commit promotes a staged image into items/{itemId}/{defId}/ and returns the new relative path.
// Old images are kept: callers run PruneAttrDir after the DB transaction succeeds, or delete the new file to roll back.
func (s *Store) Commit(userID, itemID, defID int64, stagedRel string) (string, error) {
	if !s.IsUserTmp(userID, stagedRel) {
		return "", ErrBadPath
	}
	src, err := s.Abs(stagedRel)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(src); err != nil {
		return "", model.Invalid("暂存图片不存在或已过期，请重新上传")
	}
	rel := fmt.Sprintf("items/%d/%d/%s", itemID, defID, path.Base(stagedRel))
	dst, _ := s.Abs(rel)
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return "", err
	}
	if err := os.Rename(src, dst); err != nil {
		return "", err
	}
	return rel, nil
}

// ImportFile 导入数据包时把图片字节写入 items/{itemId}/{defId}/。
// ImportFile writes image bytes from an import package into items/{itemId}/{defId}/.
// 扩展名不是常见图片格式时统一改为 .jpg，保证访问时只会以图片类型返回。
// Non-image extensions are replaced by .jpg so the file is only ever served as an image.
func (s *Store) ImportFile(itemID, defID int64, fileName string, src io.Reader, limit int64) (string, error) {
	name := path.Base(fileName)
	ext := strings.ToLower(path.Ext(name))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".webp", ".gif":
	default:
		ext = ".jpg"
		name = randomName(ext)
	}
	if !safeFileName.MatchString(name) {
		name = randomName(ext)
	}
	rel := fmt.Sprintf("items/%d/%d/%s", itemID, defID, name)
	if err := s.writeFile(rel, src, limit); err != nil {
		return "", err
	}
	return rel, nil
}

func (s *Store) writeFile(rel string, src io.Reader, limit int64) error {
	dst, err := s.Abs(rel)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	n, err := io.Copy(f, io.LimitReader(src, limit+1))
	closeErr := f.Close()
	if err == nil && n > limit {
		err = model.Invalid(fmt.Sprintf("文件超过大小限制（%d MB）", limit>>20))
	}
	if err == nil {
		err = closeErr
	}
	if err != nil {
		os.Remove(dst)
	}
	return err
}

func (s *Store) Delete(rel string) {
	if p, err := s.Abs(rel); err == nil {
		os.Remove(p)
	}
}

func (s *Store) DeleteItemImages(itemID int64) {
	os.RemoveAll(filepath.Join(s.Root, "items", strconv.FormatInt(itemID, 10)))
}

func (s *Store) DeleteAttrDir(itemID, defID int64) {
	os.RemoveAll(filepath.Join(s.Root, "items", strconv.FormatInt(itemID, 10), strconv.FormatInt(defID, 10)))
}

// PruneAttrDir 删除 items/{itemId}/{defId}/ 下除 keepRel 以外的文件；keepRel 为空时删除整个目录。
// PruneAttrDir removes every file in items/{itemId}/{defId}/ except keepRel; an empty keepRel removes the directory.
func (s *Store) PruneAttrDir(itemID, defID int64, keepRel string) {
	if keepRel == "" {
		s.DeleteAttrDir(itemID, defID)
		return
	}
	dir := filepath.Join(s.Root, "items", strconv.FormatInt(itemID, 10), strconv.FormatInt(defID, 10))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	keep := path.Base(keepRel)
	for _, e := range entries {
		if e.Name() != keep {
			os.RemoveAll(filepath.Join(dir, e.Name()))
		}
	}
}

// CleanTmp 删除暂存区中早于 maxAge 的文件。
// CleanTmp removes staging files older than maxAge.
func (s *Store) CleanTmp(maxAge time.Duration) {
	cutoff := time.Now().Add(-maxAge)
	root := filepath.Join(s.Root, "tmp")
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if info, err := d.Info(); err == nil && info.ModTime().Before(cutoff) {
			os.Remove(p)
		}
		return nil
	})
}
