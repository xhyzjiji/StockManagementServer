package store

import (
	"archive/zip"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"regexp"
	"strings"
	"time"

	"private/construction_warehouse/internal/model"
)

// 仓库数据包格式与 App 的 WarehouseTransfer 保持一致，两端可互相导入：
// zip = manifest.json + images/items/{itemId}/{defId}/{file}。
//
// The package format matches the App's WarehouseTransfer so both sides can import each other's files:
// zip = manifest.json + images/items/{itemId}/{defId}/{file}.
const (
	transferFormat        = "stock-warehouse-export"
	transferFormatVersion = 1
	entryManifest         = "manifest.json"
	entryImagePrefix      = "images/"
	maxManifestBytes      = 64 << 20
)

var (
	imageEntryPattern = regexp.MustCompile(`^images/items/\d+/\d+/[^/]+$`)
	badFileNameChars  = regexp.MustCompile(`[\\/:*?"<>|]`)
	errBadPackage     = model.Invalid("文件无法识别，请选择本应用导出的仓库数据包")
)

type manifest struct {
	Format        string `json:"format"`
	FormatVersion int    `json:"formatVersion"`
	ExportedAt    int64  `json:"exportedAt"`
	Warehouse     struct {
		Name      string `json:"name"`
		Remark    string `json:"remark"`
		CreatedAt int64  `json:"createdAt"`
	} `json:"warehouse"`
	AttrDefs   []manifestDef      `json:"attrDefs"`
	Categories []manifestCategory `json:"categories"`
	Items      []manifestItem     `json:"items"`
	AttrValues []manifestValue    `json:"attrValues"`
	Records    []manifestRecord   `json:"records"`
}

type manifestDef struct {
	ID        int64    `json:"id"`
	Name      string   `json:"name"`
	Type      string   `json:"type"`
	Options   []string `json:"options"`
	SortOrder int      `json:"sortOrder"`
}

type manifestCategory struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	CreatedAt int64  `json:"createdAt"`
}

type manifestItem struct {
	ID         int64   `json:"id"`
	CategoryID int64   `json:"categoryId"`
	Name       string  `json:"name"`
	Unit       string  `json:"unit"`
	Quantity   float64 `json:"quantity"`
	AvgPrice   float64 `json:"avgPrice"`
	TotalValue float64 `json:"totalValue"`
	StockSince *int64  `json:"stockSince"`
	CreatedAt  int64   `json:"createdAt"`
}

type manifestValue struct {
	DefID  int64  `json:"defId"`
	ItemID int64  `json:"itemId"`
	Value  string `json:"value"`
}

type manifestRecord struct {
	ItemID     int64    `json:"itemId"`
	ItemName   string   `json:"itemName"`
	Unit       string   `json:"unit"`
	Type       string   `json:"type"`
	Quantity   float64  `json:"quantity"`
	BeforeQty  float64  `json:"beforeQty"`
	AfterQty   float64  `json:"afterQty"`
	TotalPrice *float64 `json:"totalPrice"`
	Note       string   `json:"note"`
	CreatedAt  int64    `json:"createdAt"`
}

// ExportFileName 导出的建议文件名，如 `五金仓_20260906_1530.zip`。
// ExportFileName is the suggested export file name, e.g. `五金仓_20260906_1530.zip`.
func ExportFileName(warehouseName string, now time.Time) string {
	clean := strings.TrimSpace(badFileNameChars.ReplaceAllString(warehouseName, "_"))
	if clean == "" {
		clean = "仓库"
	}
	return clean + "_" + now.Format("20060102_1504") + ".zip"
}

// ExportWarehouse 把未删除的仓库导出为数据包写入 w；回收站中的物品不参与，流水全量带走。
// ExportWarehouse writes an active warehouse as a package to w; recycled items are skipped, all records are included.
func (s *Store) ExportWarehouse(ctx context.Context, userID, warehouseID int64, w io.Writer) error {
	// 单个一致性快照读取，避免导出过程中并发写入造成清单前后不一致
	// Read from one consistent snapshot so concurrent writes cannot make the manifest inconsistent.
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return err
	}
	defer tx.Rollback()

	m := manifest{Format: transferFormat, FormatVersion: transferFormatVersion, ExportedAt: nowMillis()}
	if err := tx.QueryRowContext(ctx,
		`SELECT name, remark, created_at FROM warehouse WHERE id=? AND user_id=? AND deleted_at IS NULL`,
		warehouseID, userID).Scan(&m.Warehouse.Name, &m.Warehouse.Remark, &m.Warehouse.CreatedAt); err != nil {
		return notFoundIfNoRows(err)
	}

	defs, err := listAttrDefs(ctx, tx, warehouseID)
	if err != nil {
		return err
	}
	imageDefs := map[int64]bool{}
	m.AttrDefs = []manifestDef{}
	for _, d := range defs {
		m.AttrDefs = append(m.AttrDefs, manifestDef{d.ID, d.Name, string(d.Type), d.Options, d.SortOrder})
		if d.Type == model.AttrImage {
			imageDefs[d.ID] = true
		}
	}

	m.Categories = []manifestCategory{}
	rows, err := tx.QueryContext(ctx,
		`SELECT id, name, created_at FROM category WHERE warehouse_id=? ORDER BY id`, warehouseID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var c manifestCategory
		if err := rows.Scan(&c.ID, &c.Name, &c.CreatedAt); err != nil {
			rows.Close()
			return err
		}
		m.Categories = append(m.Categories, c)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	m.Items = []manifestItem{}
	rows, err = tx.QueryContext(ctx,
		`SELECT i.id, i.category_id, i.name, i.unit, i.quantity, i.avg_price, i.total_value, i.stock_since, i.created_at
		 FROM item i JOIN category c ON c.id = i.category_id
		 WHERE c.warehouse_id=? AND i.deleted_at IS NULL ORDER BY i.id`, warehouseID)
	if err != nil {
		return err
	}
	for rows.Next() {
		it, err := scanItem(rows)
		if err != nil {
			rows.Close()
			return err
		}
		m.Items = append(m.Items, manifestItem{it.ID, it.CategoryID, it.Name, it.Unit, it.Quantity, it.AvgPrice,
			it.TotalValue, it.StockSince, it.CreatedAt})
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	// 图片值要求文件真实存在才带走
	// Image values are only exported when the file actually exists.
	m.AttrValues = []manifestValue{}
	var imagePaths []string
	rows, err = tx.QueryContext(ctx,
		`SELECT av.attribute_def_id, av.item_id, av.value FROM attribute_value av
		 JOIN item i ON i.id = av.item_id
		 JOIN category c ON c.id = i.category_id
		 WHERE c.warehouse_id=? AND i.deleted_at IS NULL`, warehouseID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var v manifestValue
		if err := rows.Scan(&v.DefID, &v.ItemID, &v.Value); err != nil {
			rows.Close()
			return err
		}
		if imageDefs[v.DefID] {
			p, err := s.images.Abs(v.Value)
			if v.Value == "" || err != nil {
				continue
			}
			if st, err := os.Stat(p); err != nil || !st.Mode().IsRegular() {
				continue
			}
			imagePaths = append(imagePaths, v.Value)
		}
		m.AttrValues = append(m.AttrValues, v)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	m.Records = []manifestRecord{}
	rows, err = tx.QueryContext(ctx,
		`SELECT item_id, item_name, unit, type, quantity, before_qty, after_qty, total_price, note, created_at
		 FROM stock_record WHERE warehouse_id=? ORDER BY id`, warehouseID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var r manifestRecord
		var price sql.NullFloat64
		if err := rows.Scan(&r.ItemID, &r.ItemName, &r.Unit, &r.Type, &r.Quantity, &r.BeforeQty, &r.AfterQty,
			&price, &r.Note, &r.CreatedAt); err != nil {
			rows.Close()
			return err
		}
		r.TotalPrice = nullFloatPtr(price)
		m.Records = append(m.Records, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	zw := zip.NewWriter(w)
	mw, err := zw.Create(entryManifest)
	if err != nil {
		return err
	}
	if err := json.NewEncoder(mw).Encode(&m); err != nil {
		return err
	}
	for _, rel := range imagePaths {
		if err := s.copyImageToZip(zw, rel); err != nil {
			return err
		}
	}
	return zw.Close()
}

func (s *Store) copyImageToZip(zw *zip.Writer, rel string) error {
	p, err := s.images.Abs(rel)
	if err != nil {
		return err
	}
	f, err := os.Open(p)
	if err != nil {
		return nil
	}
	defer f.Close()
	// 图片已是压缩格式，直接存储不再压缩
	// Images are already compressed; store them as-is.
	ew, err := zw.CreateHeader(&zip.FileHeader{Name: entryImagePrefix + rel, Method: zip.Store, Modified: time.Now()})
	if err != nil {
		return err
	}
	_, err = io.Copy(ew, f)
	return err
}

// ImportPreview 导入预检结果，供前端展示与冲突决策。
// ImportPreview is the pre-check result used by the UI to show counts and resolve name conflicts.
type ImportPreview struct {
	WarehouseName string `json:"warehouse_name"`
	CategoryCount int    `json:"category_count"`
	ItemCount     int    `json:"item_count"`
	RecordCount   int    `json:"record_count"`
	ImageCount    int    `json:"image_count"`
	HasConflict   bool   `json:"has_conflict"`
	// 冲突时"保留两者"将使用的名称
	// Name used by "keep both" when there is a conflict.
	SuggestedName string `json:"suggested_name"`
}

type ImportResult struct {
	WarehouseID       int64  `json:"warehouse_id"`
	FinalName         string `json:"final_name"`
	CategoryCount     int    `json:"category_count"`
	ItemCount         int    `json:"item_count"`
	RecordCount       int    `json:"record_count"`
	ImageCount        int    `json:"image_count"`
	MissingImageCount int    `json:"missing_image_count"`
	OverwrittenCount  int    `json:"overwritten_count"`
}

const (
	ImportKeepBoth  = "keep_both"
	ImportOverwrite = "overwrite"
)

func readManifest(zr *zip.Reader) (*manifest, error) {
	for _, f := range zr.File {
		if f.Name != entryManifest {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, errBadPackage
		}
		defer rc.Close()
		raw, err := io.ReadAll(io.LimitReader(rc, maxManifestBytes+1))
		if err != nil || len(raw) > maxManifestBytes {
			return nil, errBadPackage
		}
		var m manifest
		if err := json.Unmarshal(raw, &m); err != nil {
			return nil, errBadPackage
		}
		if err := validateManifest(&m); err != nil {
			return nil, err
		}
		return &m, nil
	}
	return nil, errBadPackage
}

func validateManifest(m *manifest) error {
	if m.Format != transferFormat || m.FormatVersion < 1 || m.FormatVersion > transferFormatVersion {
		return errBadPackage
	}
	m.Warehouse.Name = truncRunes(strings.TrimSpace(m.Warehouse.Name), 128)
	if m.Warehouse.Name == "" {
		return errBadPackage
	}
	cats := map[int64]bool{}
	for _, c := range m.Categories {
		cats[c.ID] = true
	}
	for _, it := range m.Items {
		if !cats[it.CategoryID] {
			return errBadPackage
		}
	}
	return nil
}

func truncRunes(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}

func orNow(ms int64) int64 {
	if ms <= 0 {
		return nowMillis()
	}
	return ms
}

func uniqueName(base string, taken map[string]bool) string {
	if !taken[base] {
		return base
	}
	for i := 2; ; i++ {
		if n := fmt.Sprintf("%s (%d)", base, i); !taken[n] {
			return n
		}
	}
}

func activeWarehouseNames(ctx context.Context, q querier, userID int64) (map[string]bool, error) {
	rows, err := q.QueryContext(ctx, `SELECT name FROM warehouse WHERE user_id=? AND deleted_at IS NULL`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	names := map[string]bool{}
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		names[n] = true
	}
	return names, rows.Err()
}

func openPackage(zipPath string) (*zip.ReadCloser, *manifest, error) {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, nil, errBadPackage
	}
	m, err := readManifest(&zr.Reader)
	if err != nil {
		zr.Close()
		return nil, nil, err
	}
	return zr, m, nil
}

// PrepareImport 读取数据包清单并检测同名冲突（仅限当前用户的仓库）。
// PrepareImport reads the package manifest and checks name conflicts among the user's warehouses.
func (s *Store) PrepareImport(ctx context.Context, userID int64, zipPath string) (*ImportPreview, error) {
	zr, m, err := openPackage(zipPath)
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	taken, err := activeWarehouseNames(ctx, s.db, userID)
	if err != nil {
		return nil, err
	}
	imageDefs := map[int64]bool{}
	for _, d := range m.AttrDefs {
		if model.AttrType(d.Type) == model.AttrImage {
			imageDefs[d.ID] = true
		}
	}
	images := 0
	for _, v := range m.AttrValues {
		if imageDefs[v.DefID] && v.Value != "" {
			images++
		}
	}
	return &ImportPreview{
		WarehouseName: m.Warehouse.Name,
		CategoryCount: len(m.Categories),
		ItemCount:     len(m.Items),
		RecordCount:   len(m.Records),
		ImageCount:    images,
		HasConflict:   taken[m.Warehouse.Name],
		SuggestedName: uniqueName(m.Warehouse.Name, taken),
	}, nil
}

// ExecuteImport 单事务重建「仓库 → 类目 → 物品」子树，id 全部重映射；失败整体回滚并清理已写出的图片。
// 冲突策略以执行时刻的库内状态为准：keep_both 自动改名，overwrite 把同名仓库移入回收站。
//
// ExecuteImport rebuilds the warehouse → category → item tree in one transaction with all ids remapped;
// on failure everything rolls back and written images are removed.
// Conflicts are resolved against the current state: keep_both renames, overwrite moves same-name warehouses to the bin.
func (s *Store) ExecuteImport(ctx context.Context, userID int64, zipPath, mode string, maxImageBytes int64) (*ImportResult, error) {
	if mode != ImportKeepBoth && mode != ImportOverwrite {
		return nil, model.Invalid("未知的冲突处理方式")
	}
	zr, m, err := openPackage(zipPath)
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	entries := map[string]*zip.File{}
	for _, f := range zr.File {
		if !f.FileInfo().IsDir() && imageEntryPattern.MatchString(f.Name) {
			entries[f.Name] = f
		}
	}

	var written []string
	res := &ImportResult{CategoryCount: len(m.Categories), ItemCount: len(m.Items), RecordCount: len(m.Records)}
	err = s.inTx(ctx, func(tx *sql.Tx) error {
		taken, err := activeWarehouseNames(ctx, tx, userID)
		if err != nil {
			return err
		}
		res.FinalName = m.Warehouse.Name
		if taken[m.Warehouse.Name] {
			if mode == ImportOverwrite {
				r, err := tx.ExecContext(ctx,
					`UPDATE warehouse SET deleted_at=? WHERE user_id=? AND name=? AND deleted_at IS NULL`,
					nowMillis(), userID, m.Warehouse.Name)
				if err != nil {
					return err
				}
				n, _ := r.RowsAffected()
				res.OverwrittenCount = int(n)
			} else {
				res.FinalName = uniqueName(m.Warehouse.Name, taken)
			}
		}

		r, err := tx.ExecContext(ctx,
			`INSERT INTO warehouse(user_id, name, remark, created_at) VALUES(?, ?, ?, ?)`,
			userID, res.FinalName, truncRunes(m.Warehouse.Remark, 1024), orNow(m.Warehouse.CreatedAt))
		if err != nil {
			return err
		}
		if res.WarehouseID, err = r.LastInsertId(); err != nil {
			return err
		}

		defIDs := map[int64]int64{}
		defTypes := map[int64]model.AttrType{}
		for _, d := range m.AttrDefs {
			typ := model.AttrType(d.Type)
			if !typ.Valid() {
				typ = model.AttrText
			}
			r, err := tx.ExecContext(ctx,
				`INSERT INTO attribute_def(warehouse_id, name, type, options, sort_order) VALUES(?, ?, ?, ?, ?)`,
				res.WarehouseID, truncRunes(d.Name, 128), typ, model.EncodeOptions(d.Options), d.SortOrder)
			if err != nil {
				return err
			}
			if defIDs[d.ID], err = r.LastInsertId(); err != nil {
				return err
			}
			defTypes[d.ID] = typ
		}

		catIDs := map[int64]int64{}
		for _, c := range m.Categories {
			r, err := tx.ExecContext(ctx,
				`INSERT INTO category(warehouse_id, name, created_at) VALUES(?, ?, ?)`,
				res.WarehouseID, truncRunes(c.Name, 128), orNow(c.CreatedAt))
			if err != nil {
				return err
			}
			if catIDs[c.ID], err = r.LastInsertId(); err != nil {
				return err
			}
		}

		itemIDs := map[int64]int64{}
		for _, it := range m.Items {
			r, err := tx.ExecContext(ctx,
				`INSERT INTO item(category_id, name, unit, quantity, avg_price, total_value, stock_since, created_at)
				 VALUES(?, ?, ?, ?, ?, ?, ?, ?)`,
				catIDs[it.CategoryID], truncRunes(it.Name, 255), truncRunes(it.Unit, 32), it.Quantity,
				it.AvgPrice, it.TotalValue, it.StockSince, orNow(it.CreatedAt))
			if err != nil {
				return err
			}
			if itemIDs[it.ID], err = r.LastInsertId(); err != nil {
				return err
			}
		}

		for _, v := range m.AttrValues {
			defID, ok1 := defIDs[v.DefID]
			itemID, ok2 := itemIDs[v.ItemID]
			if !ok1 || !ok2 || v.Value == "" {
				continue
			}
			value := v.Value
			if defTypes[v.DefID] == model.AttrImage {
				f := entries[entryImagePrefix+v.Value]
				if f == nil {
					res.MissingImageCount++
					continue
				}
				rel, err := s.importImage(f, itemID, defID, maxImageBytes)
				if err != nil {
					res.MissingImageCount++
					continue
				}
				written = append(written, rel)
				res.ImageCount++
				value = rel
			}
			if err := upsertValue(ctx, tx, defID, itemID, value); err != nil {
				return err
			}
		}

		for _, rec := range m.Records {
			// 映射不上的流水指向已删除物品：置负 id 保持悬空引用，与 App 一致
			// Records of items that no longer exist keep a negative dangling id, same as the App.
			itemID, ok := itemIDs[rec.ItemID]
			if !ok {
				itemID = -rec.ItemID
			}
			typ := model.StockIn
			if rec.Type == string(model.StockOut) {
				typ = model.StockOut
			}
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO stock_record(warehouse_id, item_id, item_name, unit, type, quantity, before_qty, after_qty,
				                          total_price, note, created_at)
				 VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				res.WarehouseID, itemID, truncRunes(rec.ItemName, 255), truncRunes(rec.Unit, 32), typ, rec.Quantity,
				rec.BeforeQty, rec.AfterQty, rec.TotalPrice, truncRunes(rec.Note, 1024), orNow(rec.CreatedAt)); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		for _, rel := range written {
			s.images.Delete(rel)
		}
		return nil, err
	}
	return res, nil
}

func (s *Store) importImage(f *zip.File, itemID, defID int64, limit int64) (string, error) {
	rc, err := f.Open()
	if err != nil {
		return "", err
	}
	defer rc.Close()
	return s.images.ImportFile(itemID, defID, path.Base(f.Name), rc, limit)
}
