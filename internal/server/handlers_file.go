package server

import (
	"bytes"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"private/construction_warehouse/internal/model"
	"private/construction_warehouse/internal/store"
)

// imageExts 允许存储与访问的图片类型（按内容嗅探得到的 MIME → 扩展名）。
// imageExts are the image types allowed for storage and access (sniffed MIME → extension).
var imageExts = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
	"image/gif":  ".gif",
}

var imageMimes = map[string]string{
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".png":  "image/png",
	".webp": "image/webp",
	".gif":  "image/gif",
}

// filePart 返回 multipart 表单中名为 "file" 的部分。
// filePart returns the part named "file" of a multipart form.
func filePart(r *http.Request) (*multipart.Part, error) {
	mr, err := r.MultipartReader()
	if err != nil {
		return nil, model.Invalid("请使用 multipart/form-data 上传文件")
	}
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			return nil, model.Invalid("缺少文件字段 file")
		}
		if err != nil {
			return nil, err
		}
		if part.FormName() == "file" {
			return part, nil
		}
		part.Close()
	}
}

func (s *Server) handleUploadImage(w http.ResponseWriter, r *http.Request) {
	limit := s.cfg.Storage.MaxImageMB << 20
	r.Body = http.MaxBytesReader(w, r.Body, limit+1<<20)
	part, err := filePart(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	defer part.Close()
	head := make([]byte, 512)
	n, err := io.ReadFull(part, head)
	if err != nil && err != io.ErrUnexpectedEOF {
		if err == io.EOF {
			fail(w, r, model.Invalid("文件为空"))
			return
		}
		fail(w, r, err)
		return
	}
	ext, ok := imageExts[http.DetectContentType(head[:n])]
	if !ok {
		fail(w, r, model.Invalid("仅支持 JPG / PNG / WebP / GIF 图片"))
		return
	}
	rel, err := s.store.Images().SaveTmp(uid(r), io.MultiReader(bytes.NewReader(head[:n]), part), ext, limit)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"path": rel, "url": "/files/" + rel})
}

// handleFile 校验归属后返回图片：
// items/{itemId}/...、works/{workId}/... 需属于当前用户；tmp/{userId}/... 需为本人暂存。
// handleFile serves an image after an ownership check:
// items/{itemId}/... and works/{workId}/... must belong to the user; tmp/{userId}/... must be their staging.
func (s *Server) handleFile(w http.ResponseWriter, r *http.Request) {
	rel := r.PathValue("path")
	parts := strings.Split(rel, "/")
	mime, okExt := imageMimes[cleanExt(rel)]
	if len(parts) < 3 || !okExt {
		http.NotFound(w, r)
		return
	}
	owner, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	switch parts[0] {
	case "items":
		if !s.store.UserOwnsItemFiles(r.Context(), uid(r), owner) {
			http.NotFound(w, r)
			return
		}
	case "works":
		if !s.store.UserOwnsWorkFiles(r.Context(), uid(r), owner) {
			http.NotFound(w, r)
			return
		}
	case "tmp":
		if owner != uid(r) {
			http.NotFound(w, r)
			return
		}
	default:
		http.NotFound(w, r)
		return
	}
	abs, err := s.store.Images().Abs(rel)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	f, err := os.Open(abs)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil || !st.Mode().IsRegular() {
		http.NotFound(w, r)
		return
	}
	h := w.Header()
	h.Set("Content-Type", mime)
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Content-Security-Policy", "default-src 'none'; sandbox")
	// 文件名随机且内容不变，可长期缓存
	// File names are random and content never changes, so cache for long.
	h.Set("Cache-Control", "private, max-age=2592000, immutable")
	http.ServeContent(w, r, st.Name(), st.ModTime(), f)
}

// handleExport 先导出到临时文件再下发，失败时可以返回明确的错误而不是半截文件。
// handleExport writes to a temp file first so a failure yields a clear error instead of a truncated download.
func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	wh, err := s.store.GetWarehouse(r.Context(), uid(r), id)
	if err != nil {
		fail(w, r, err)
		return
	}
	tmp, err := os.CreateTemp("", "cw-export-*.zip")
	if err != nil {
		fail(w, r, err)
		return
	}
	defer func() {
		tmp.Close()
		os.Remove(tmp.Name())
	}()
	if err := s.store.ExportWarehouse(r.Context(), uid(r), id, tmp); err != nil {
		fail(w, r, err)
		return
	}
	size, err := tmp.Seek(0, io.SeekCurrent)
	if err != nil {
		fail(w, r, err)
		return
	}
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		fail(w, r, err)
		return
	}
	name := store.ExportFileName(wh.Name, time.Now())
	h := w.Header()
	h.Set("Content-Type", "application/zip")
	h.Set("Content-Length", strconv.FormatInt(size, 10))
	h.Set("Content-Disposition", `attachment; filename="export.zip"; filename*=UTF-8''`+url.PathEscape(name))
	h.Set("Cache-Control", "no-store")
	if _, err := io.Copy(w, tmp); err != nil {
		log.Printf("export copy: %v", err)
	}
}

func (s *Server) handleImportPrepare(w http.ResponseWriter, r *http.Request) {
	limit := s.cfg.Storage.MaxImportMB << 20
	r.Body = http.MaxBytesReader(w, r.Body, limit+1<<20)
	part, err := filePart(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	defer part.Close()
	images := s.store.Images()
	rel, err := images.SaveTmp(uid(r), part, ".zip", limit)
	if err != nil {
		fail(w, r, err)
		return
	}
	abs, _ := images.Abs(rel)
	preview, err := s.store.PrepareImport(r.Context(), uid(r), abs)
	if err != nil {
		images.Delete(rel)
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"file": rel, "preview": preview})
}

func (s *Server) handleImportCommit(w http.ResponseWriter, r *http.Request) {
	var req struct {
		File string `json:"file"`
		Mode string `json:"mode"`
	}
	if err := readJSON(w, r, &req); err != nil {
		fail(w, r, err)
		return
	}
	images := s.store.Images()
	if !images.IsUserTmp(uid(r), req.File) || !strings.HasSuffix(req.File, ".zip") {
		fail(w, r, model.Invalid("导入文件无效，请重新选择"))
		return
	}
	abs, err := images.Abs(req.File)
	if err != nil {
		fail(w, r, err)
		return
	}
	if _, err := os.Stat(abs); err != nil {
		fail(w, r, model.Invalid("导入文件已过期，请重新选择"))
		return
	}
	res, err := s.store.ExecuteImport(r.Context(), uid(r), abs, req.Mode, s.cfg.Storage.MaxImageMB<<20)
	if err != nil {
		fail(w, r, err)
		return
	}
	images.Delete(req.File)
	writeJSON(w, http.StatusOK, res)
}
