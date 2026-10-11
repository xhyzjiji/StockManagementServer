package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"

	"private/construction_warehouse/internal/model"
	"private/construction_warehouse/internal/stockmath"
)

const (
	maxWorkImages    = 20
	maxWorkMaterials = 200
	maxWorkQty       = 1e12
	maxWorkMinutes   = 1e9
)

// ListWorks 返回用户的作品列表（未删除），按创建时间倒序。
// ListWorks returns the user's non-deleted works, newest first.
func (s *Store) ListWorks(ctx context.Context, userID int64) ([]model.Work, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, remark, quantity, total_minutes, avg_minutes,
		        material_cost, unit_material_cost, cover_image, created_at
		 FROM work WHERE user_id=? AND deleted_at IS NULL ORDER BY created_at DESC, id DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []model.Work{}
	for rows.Next() {
		var w model.Work
		if err := rows.Scan(&w.ID, &w.Name, &w.Remark, &w.Quantity, &w.TotalMinutes, &w.AvgMinutes,
			&w.MaterialCost, &w.UnitMaterialCost, &w.CoverImage, &w.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, w)
	}
	return list, rows.Err()
}

// GetWorkDetail 返回作品详情（图片、材料、品次）。
// GetWorkDetail returns a work with images, materials and batches.
func (s *Store) GetWorkDetail(ctx context.Context, userID, id int64) (*model.WorkDetail, error) {
	w, err := s.getWork(ctx, s.db, userID, id, false)
	if err != nil {
		return nil, err
	}
	images, err := s.listWorkImages(ctx, s.db, id)
	if err != nil {
		return nil, err
	}
	materials, err := s.listWorkMaterials(ctx, s.db, id)
	if err != nil {
		return nil, err
	}
	batches, err := s.listWorkBatches(ctx, s.db, id)
	if err != nil {
		return nil, err
	}
	return &model.WorkDetail{Work: *w, Images: images, Materials: materials, Batches: batches}, nil
}

// CreateWork 创建作品：校验材料归属与用量，按当前均价锁定材料成本，转正图片，写入首个品次。
// 不扣减仓库库存——仅用于成本核算与材料关联。
//
// CreateWork creates a work: validates material ownership and usages, locks cost from current averages,
// promotes images and writes the first batch. Warehouse stock is not deducted — this is costing only.
func (s *Store) CreateWork(ctx context.Context, userID int64, in model.WorkInput) (int64, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return 0, model.Invalid("请输入作品名称")
	}
	if len(name) > 255 {
		return 0, model.Invalid("作品名称过长")
	}
	remark := strings.TrimSpace(in.Remark)
	if len(remark) > 1000 {
		return 0, model.Invalid("备注过长")
	}
	note := strings.TrimSpace(in.Note)
	if len(note) > 1000 {
		return 0, model.Invalid("备注过长")
	}
	if in.Quantity <= 0 || in.Quantity > maxWorkQty {
		return 0, model.Invalid("作品数量须大于 0")
	}
	if in.Minutes < 0 || in.Minutes > maxWorkMinutes {
		return 0, model.Invalid("制作用时无效")
	}
	if len(in.Images) == 0 {
		return 0, model.Invalid("请至少上传一张作品图片")
	}
	if len(in.Images) > maxWorkImages {
		return 0, model.Invalid(fmt.Sprintf("作品图片最多 %d 张", maxWorkImages))
	}
	if len(in.Materials) == 0 {
		return 0, model.Invalid("请至少选择一种材料")
	}
	if len(in.Materials) > maxWorkMaterials {
		return 0, model.Invalid("材料种类过多")
	}
	if err := validateWorkImagesCreate(userID, in.Images, s); err != nil {
		return 0, err
	}

	mats, materialCost, err := s.resolveWorkMaterials(ctx, userID, in.Materials)
	if err != nil {
		return 0, err
	}
	unitCost := stockmath.Round2(materialCost / in.Quantity)
	avgMinutes := in.Minutes / in.Quantity
	now := nowMillis()

	var workID int64
	var coverRel string
	committed := []string{}
	err = s.inTx(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx,
			`INSERT INTO work(user_id, name, remark, quantity, total_minutes, avg_minutes,
			                  material_cost, unit_material_cost, cover_image, created_at)
			 VALUES(?,?,?,?,?,?,?,?, '', ?)`,
			userID, name, remark, in.Quantity, in.Minutes, avgMinutes, materialCost, unitCost, now)
		if err != nil {
			return err
		}
		workID, err = res.LastInsertId()
		if err != nil {
			return err
		}
		for i, img := range in.Images {
			rel, err := s.images.CommitWork(userID, workID, img.StagedImage)
			if err != nil {
				return err
			}
			committed = append(committed, rel)
			if img.IsCover {
				coverRel = rel
			}
			isCover := 0
			if img.IsCover {
				isCover = 1
			}
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO work_image(work_id, path, sort_order, is_cover) VALUES(?,?,?,?)`,
				workID, rel, i, isCover); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, `UPDATE work SET cover_image=? WHERE id=?`, coverRel, workID); err != nil {
			return err
		}
		for _, m := range mats {
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO work_material(work_id, item_id, warehouse_id, warehouse_name, item_name, unit, quantity, unit_price, line_cost)
				 VALUES(?,?,?,?,?,?,?,?,?)`,
				workID, m.itemID, m.whID, m.whName, m.itemName, m.unit, m.qty, m.unitPrice, m.lineCost); err != nil {
				return err
			}
		}
		_, err = tx.ExecContext(ctx,
			`INSERT INTO work_batch(work_id, quantity, minutes, note, created_at) VALUES(?,?,?,?,?)`,
			workID, in.Quantity, in.Minutes, note, now)
		return err
	})
	if err != nil {
		for _, rel := range committed {
			s.images.Delete(rel)
		}
		if workID > 0 {
			s.images.DeleteWorkImages(workID)
		}
		return 0, err
	}
	return workID, nil
}

// AppendWorkBatch 追加数量与用时，并重算平均制作用时。
// AppendWorkBatch appends quantity and time and recalculates average production minutes.
func (s *Store) AppendWorkBatch(ctx context.Context, userID, workID int64, in model.WorkBatchInput) error {
	if in.Quantity <= 0 || in.Quantity > maxWorkQty {
		return model.Invalid("追加数量须大于 0")
	}
	if in.Minutes < 0 || in.Minutes > maxWorkMinutes {
		return model.Invalid("制作用时无效")
	}
	note := strings.TrimSpace(in.Note)
	if len(note) > 1000 {
		return model.Invalid("备注过长")
	}
	return s.inTx(ctx, func(tx *sql.Tx) error {
		w, err := s.getWork(ctx, tx, userID, workID, true)
		if err != nil {
			return err
		}
		qty := w.Quantity + in.Quantity
		mins := w.TotalMinutes + in.Minutes
		avg := 0.0
		if qty > stockmath.Epsilon {
			avg = mins / qty
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE work SET quantity=?, total_minutes=?, avg_minutes=? WHERE id=?`,
			qty, mins, avg, workID); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx,
			`INSERT INTO work_batch(work_id, quantity, minutes, note, created_at) VALUES(?,?,?,?,?)`,
			workID, in.Quantity, in.Minutes, note, nowMillis())
		return err
	})
}

// DeleteWork 彻底删除作品及其图片（作品暂无回收站）。
// DeleteWork permanently deletes a work and its images (works have no recycle bin yet).
func (s *Store) DeleteWork(ctx context.Context, userID, workID int64) error {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM work WHERE id=? AND user_id=? AND deleted_at IS NULL`, workID, userID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return model.ErrNotFound
	}
	s.images.DeleteWorkImages(workID)
	return nil
}

// UpdateWork 编辑作品名称、备注、图片与材料；按当前均价重锁材料成本；不改累计数量/用时。
// UpdateWork edits name, remark, images and materials; re-locks cost from current averages; quantity/time unchanged.
func (s *Store) UpdateWork(ctx context.Context, userID, workID int64, in model.WorkUpdateInput) error {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return model.Invalid("请输入作品名称")
	}
	if len(name) > 255 {
		return model.Invalid("作品名称过长")
	}
	remark := strings.TrimSpace(in.Remark)
	if len(remark) > 1000 {
		return model.Invalid("备注过长")
	}
	if len(in.Images) == 0 {
		return model.Invalid("请至少上传一张作品图片")
	}
	if len(in.Images) > maxWorkImages {
		return model.Invalid(fmt.Sprintf("作品图片最多 %d 张", maxWorkImages))
	}
	if len(in.Materials) == 0 {
		return model.Invalid("请至少选择一种材料")
	}
	if len(in.Materials) > maxWorkMaterials {
		return model.Invalid("材料种类过多")
	}

	w, err := s.getWork(ctx, s.db, userID, workID, false)
	if err != nil {
		return err
	}
	oldImages, err := s.listWorkImages(ctx, s.db, workID)
	if err != nil {
		return err
	}
	oldPaths := map[string]bool{}
	for _, img := range oldImages {
		oldPaths[img.Path] = true
	}
	if err := validateWorkImagesUpdate(userID, workID, in.Images, oldPaths, s); err != nil {
		return err
	}

	mats, materialCost, err := s.resolveWorkMaterials(ctx, userID, in.Materials)
	if err != nil {
		return err
	}
	// 单件成本仍按首批数量折算，与创建时一致；累计产出件数不因编辑材料而改。
	// Per-piece cost uses the first batch qty (same as create); total produced qty is unchanged by material edits.
	var firstQty float64
	err = s.db.QueryRowContext(ctx,
		`SELECT quantity FROM work_batch WHERE work_id=? ORDER BY created_at ASC, id ASC LIMIT 1`, workID).Scan(&firstQty)
	if err != nil {
		return notFoundIfNoRows(err)
	}
	if firstQty <= stockmath.Epsilon {
		firstQty = w.Quantity
	}
	if firstQty <= stockmath.Epsilon {
		return model.Invalid("作品数量无效，无法重算单件成本")
	}
	unitCost := stockmath.Round2(materialCost / firstQty)

	var coverRel string
	newCommitted := []string{}
	keepPaths := map[string]bool{}
	err = s.inTx(ctx, func(tx *sql.Tx) error {
		if _, err := s.getWork(ctx, tx, userID, workID, true); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM work_image WHERE work_id=?`, workID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM work_material WHERE work_id=?`, workID); err != nil {
			return err
		}
		for i, img := range in.Images {
			var rel string
			if img.Path != "" {
				rel = img.Path
				keepPaths[rel] = true
			} else {
				r, err := s.images.CommitWork(userID, workID, img.StagedImage)
				if err != nil {
					return err
				}
				rel = r
				newCommitted = append(newCommitted, rel)
				keepPaths[rel] = true
			}
			if img.IsCover {
				coverRel = rel
			}
			isCover := 0
			if img.IsCover {
				isCover = 1
			}
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO work_image(work_id, path, sort_order, is_cover) VALUES(?,?,?,?)`,
				workID, rel, i, isCover); err != nil {
				return err
			}
		}
		for _, m := range mats {
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO work_material(work_id, item_id, warehouse_id, warehouse_name, item_name, unit, quantity, unit_price, line_cost)
				 VALUES(?,?,?,?,?,?,?,?,?)`,
				workID, m.itemID, m.whID, m.whName, m.itemName, m.unit, m.qty, m.unitPrice, m.lineCost); err != nil {
				return err
			}
		}
		_, err := tx.ExecContext(ctx,
			`UPDATE work SET name=?, remark=?, material_cost=?, unit_material_cost=?, cover_image=?
			 WHERE id=? AND user_id=? AND deleted_at IS NULL`,
			name, remark, materialCost, unitCost, coverRel, workID, userID)
		return err
	})
	if err != nil {
		for _, rel := range newCommitted {
			s.images.Delete(rel)
		}
		return err
	}
	// 事务成功后清理未再引用的旧图片文件
	// After commit, remove old image files that are no longer referenced.
	for p := range oldPaths {
		if !keepPaths[p] {
			s.images.Delete(p)
		}
	}
	return nil
}

type workMatRow struct {
	itemID, whID           int64
	whName, itemName, unit string
	qty, unitPrice, lineCost float64
}

func validateWorkImagesCreate(userID int64, images []model.WorkImageInput, s *Store) error {
	coverCount := 0
	seen := map[string]bool{}
	for _, img := range images {
		if strings.TrimSpace(img.Path) != "" {
			return model.Invalid("图片无效，请重新上传")
		}
		if !s.images.IsUserTmp(userID, img.StagedImage) {
			return model.Invalid("图片无效，请重新上传")
		}
		if seen[img.StagedImage] {
			return model.Invalid("图片重复")
		}
		seen[img.StagedImage] = true
		if img.IsCover {
			coverCount++
		}
	}
	if coverCount != 1 {
		return model.Invalid("请选择一张图片作为封面缩略图")
	}
	return nil
}

func validateWorkImagesUpdate(userID, workID int64, images []model.WorkImageInput, oldPaths map[string]bool, s *Store) error {
	coverCount := 0
	seen := map[string]bool{}
	for _, img := range images {
		path := strings.TrimSpace(img.Path)
		staged := strings.TrimSpace(img.StagedImage)
		if (path == "") == (staged == "") {
			return model.Invalid("图片无效，请重新上传")
		}
		key := path
		if staged != "" {
			key = staged
			if !s.images.IsUserTmp(userID, staged) {
				return model.Invalid("图片无效，请重新上传")
			}
		} else {
			if !s.images.IsWorkFile(workID, path) || !oldPaths[path] {
				return model.Invalid("图片无效，请重新上传")
			}
			abs, err := s.images.Abs(path)
			if err != nil {
				return model.Invalid("图片无效，请重新上传")
			}
			if _, err := os.Stat(abs); err != nil {
				return model.Invalid("图片文件缺失，请重新上传")
			}
		}
		if seen[key] {
			return model.Invalid("图片重复")
		}
		seen[key] = true
		if img.IsCover {
			coverCount++
		}
	}
	if coverCount != 1 {
		return model.Invalid("请选择一张图片作为封面缩略图")
	}
	return nil
}

func (s *Store) resolveWorkMaterials(ctx context.Context, userID int64, materials []model.WorkMaterialInput) ([]workMatRow, float64, error) {
	mats := make([]workMatRow, 0, len(materials))
	seenItem := map[int64]bool{}
	var materialCost float64
	for _, m := range materials {
		if m.ItemID <= 0 || seenItem[m.ItemID] {
			return nil, 0, model.Invalid("材料选择无效或重复")
		}
		if m.Quantity <= 0 || m.Quantity > maxWorkQty {
			return nil, 0, model.Invalid("材料用量须大于 0")
		}
		seenItem[m.ItemID] = true
		var whID int64
		var whName, itemName, unit string
		var avgPrice float64
		err := s.db.QueryRowContext(ctx,
			`SELECT i.name, i.unit, i.avg_price, w.id, w.name
			 FROM item i
			 JOIN category c ON c.id = i.category_id
			 JOIN warehouse w ON w.id = c.warehouse_id
			 WHERE i.id=? AND w.user_id=? AND i.deleted_at IS NULL AND w.deleted_at IS NULL`,
			m.ItemID, userID).Scan(&itemName, &unit, &avgPrice, &whID, &whName)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil, 0, model.Invalid("所选材料已不存在或已被删除，请重新选择")
			}
			return nil, 0, notFoundIfNoRows(err)
		}
		line := stockmath.Round2(m.Quantity * avgPrice)
		materialCost = stockmath.Round2(materialCost + line)
		mats = append(mats, workMatRow{m.ItemID, whID, whName, itemName, unit, m.Quantity, avgPrice, line})
	}
	return mats, materialCost, nil
}

// ListMaterials 在指定仓库内列出可选材料，支持按名称关键字过滤。
// ListMaterials lists pickable materials in one warehouse, optionally filtered by name keyword.
func (s *Store) ListMaterials(ctx context.Context, userID, warehouseID int64, keyword string) ([]model.MaterialPick, error) {
	if warehouseID <= 0 {
		return nil, model.Invalid("请先选择仓库")
	}
	if err := checkWarehouse(ctx, s.db, userID, warehouseID); err != nil {
		return nil, err
	}
	args := []any{userID, warehouseID}
	where := `w.user_id=? AND w.id=? AND w.deleted_at IS NULL AND i.deleted_at IS NULL`
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		where += ` AND i.name LIKE ? ESCAPE '\\'`
		args = append(args, "%"+escapeLike(keyword)+"%")
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT i.id, i.name, i.unit, i.quantity, i.avg_price, w.id, w.name
		 FROM item i
		 JOIN category c ON c.id = i.category_id
		 JOIN warehouse w ON w.id = c.warehouse_id
		 WHERE `+where+`
		 ORDER BY i.name, i.id
		 LIMIT 500`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []model.MaterialPick{}
	ids := []int64{}
	for rows.Next() {
		var m model.MaterialPick
		if err := rows.Scan(&m.ItemID, &m.ItemName, &m.Unit, &m.Quantity, &m.AvgPrice, &m.WarehouseID, &m.WarehouseName); err != nil {
			return nil, err
		}
		list = append(list, m)
		ids = append(ids, m.ItemID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return list, nil
	}
	// 首张图片：取该物品第一个 IMAGE 类型属性值
	// First image: the first IMAGE attribute value on the item.
	ph := strings.TrimRight(strings.Repeat("?,", len(ids)), ",")
	args2 := make([]any, len(ids))
	for i, id := range ids {
		args2[i] = id
	}
	imgRows, err := s.db.QueryContext(ctx,
		`SELECT av.item_id, av.value FROM attribute_value av
		 JOIN attribute_def ad ON ad.id = av.attribute_def_id
		 WHERE ad.type='IMAGE' AND av.value<>'' AND av.item_id IN (`+ph+`)
		 ORDER BY ad.sort_order, ad.id`, args2...)
	if err != nil {
		return nil, err
	}
	defer imgRows.Close()
	first := map[int64]string{}
	for imgRows.Next() {
		var itemID int64
		var path string
		if err := imgRows.Scan(&itemID, &path); err != nil {
			return nil, err
		}
		if _, ok := first[itemID]; !ok {
			first[itemID] = path
		}
	}
	if err := imgRows.Err(); err != nil {
		return nil, err
	}
	for i := range list {
		list[i].FirstImage = first[list[i].ItemID]
	}
	return list, nil
}

// UserOwnsWorkFiles 判断用户是否可读取作品图片（含软删除）。
// UserOwnsWorkFiles reports whether the user may read a work's images (including soft-deleted works).
func (s *Store) UserOwnsWorkFiles(ctx context.Context, userID, workID int64) bool {
	var id int64
	err := s.db.QueryRowContext(ctx,
		`SELECT id FROM work WHERE id=? AND user_id=?`, workID, userID).Scan(&id)
	return err == nil
}

func (s *Store) getWork(ctx context.Context, q querier, userID, id int64, forUpdate bool) (*model.Work, error) {
	query := `SELECT id, name, remark, quantity, total_minutes, avg_minutes,
	                 material_cost, unit_material_cost, cover_image, created_at
	          FROM work WHERE id=? AND user_id=? AND deleted_at IS NULL`
	if forUpdate {
		query += ` FOR UPDATE`
	}
	var w model.Work
	err := q.QueryRowContext(ctx, query, id, userID).Scan(
		&w.ID, &w.Name, &w.Remark, &w.Quantity, &w.TotalMinutes, &w.AvgMinutes,
		&w.MaterialCost, &w.UnitMaterialCost, &w.CoverImage, &w.CreatedAt)
	if err != nil {
		return nil, notFoundIfNoRows(err)
	}
	return &w, nil
}

func (s *Store) listWorkImages(ctx context.Context, q querier, workID int64) ([]model.WorkImage, error) {
	rows, err := q.QueryContext(ctx,
		`SELECT id, path, sort_order, is_cover FROM work_image WHERE work_id=? ORDER BY sort_order, id`, workID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []model.WorkImage{}
	for rows.Next() {
		var img model.WorkImage
		var cover int
		if err := rows.Scan(&img.ID, &img.Path, &img.SortOrder, &cover); err != nil {
			return nil, err
		}
		img.IsCover = cover != 0
		list = append(list, img)
	}
	return list, rows.Err()
}

func (s *Store) listWorkMaterials(ctx context.Context, q querier, workID int64) ([]model.WorkMaterial, error) {
	rows, err := q.QueryContext(ctx,
		`SELECT id, item_id, warehouse_id, warehouse_name, item_name, unit, quantity, unit_price, line_cost
		 FROM work_material WHERE work_id=? ORDER BY id`, workID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []model.WorkMaterial{}
	for rows.Next() {
		var m model.WorkMaterial
		if err := rows.Scan(&m.ID, &m.ItemID, &m.WarehouseID, &m.WarehouseName, &m.ItemName, &m.Unit,
			&m.Quantity, &m.UnitPrice, &m.LineCost); err != nil {
			return nil, err
		}
		list = append(list, m)
	}
	return list, rows.Err()
}

func (s *Store) listWorkBatches(ctx context.Context, q querier, workID int64) ([]model.WorkBatch, error) {
	rows, err := q.QueryContext(ctx,
		`SELECT id, quantity, minutes, note, created_at FROM work_batch WHERE work_id=? ORDER BY created_at DESC, id DESC`, workID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []model.WorkBatch{}
	for rows.Next() {
		var b model.WorkBatch
		if err := rows.Scan(&b.ID, &b.Quantity, &b.Minutes, &b.Note, &b.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, b)
	}
	return list, rows.Err()
}
