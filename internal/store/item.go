package store

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"

	"private/construction_warehouse/internal/model"
)

// ItemQuery 物品总览的筛选条件。
// ItemQuery holds the filters of the item overview.
type ItemQuery struct {
	// 0 表示全部类目
	// 0 means all categories.
	CategoryID int64
	Keyword    string
	// true 按标签搜索，false 按名称搜索
	// true searches tags, false searches names.
	ByTag bool
	// "" 录入时间新→旧；"days_desc" 在库天数长→短；"days_asc" 在库天数短→长
	// "" newest first; "days_desc" longest in stock first; "days_asc" shortest in stock first.
	Sort string
}

// escapeLike 转义 LIKE 通配符，使关键字按字面匹配。
// escapeLike escapes LIKE wildcards so the keyword matches literally.
func escapeLike(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(s)
}

func (s *Store) ListItems(ctx context.Context, userID, warehouseID int64, q ItemQuery) ([]model.ItemSummary, error) {
	if err := s.CheckWarehouse(ctx, userID, warehouseID); err != nil {
		return nil, err
	}
	query := `SELECT i.id, i.category_id, i.name, i.unit, i.quantity, i.avg_price, i.total_value, i.stock_since, i.created_at
		FROM item i JOIN category c ON c.id = i.category_id
		WHERE c.warehouse_id=? AND i.deleted_at IS NULL`
	args := []any{warehouseID}
	if q.CategoryID > 0 {
		query += ` AND i.category_id=?`
		args = append(args, q.CategoryID)
	}
	if kw := strings.TrimSpace(q.Keyword); kw != "" {
		if q.ByTag {
			query += ` AND EXISTS (SELECT 1 FROM attribute_value av JOIN attribute_def ad ON ad.id = av.attribute_def_id
				WHERE av.item_id = i.id AND ad.type='TAGS' AND av.value LIKE ?)`
			args = append(args, "%"+escapeLike(strings.ReplaceAll(kw, "|", ""))+"%")
		} else {
			query += ` AND i.name LIKE ?`
			args = append(args, "%"+escapeLike(kw)+"%")
		}
	}
	switch q.Sort {
	case "days_desc":
		query += ` ORDER BY i.stock_since IS NULL, i.stock_since ASC, i.id DESC`
	case "days_asc":
		query += ` ORDER BY i.stock_since IS NULL, i.stock_since DESC, i.id DESC`
	default:
		query += ` ORDER BY i.id DESC`
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []model.ItemSummary{}
	index := map[int64]int{}
	for rows.Next() {
		var it model.ItemSummary
		var since sql.NullInt64
		if err := rows.Scan(&it.ID, &it.CategoryID, &it.Name, &it.Unit, &it.Quantity, &it.AvgPrice,
			&it.TotalValue, &since, &it.CreatedAt); err != nil {
			return nil, err
		}
		it.StockSince = nullInt64Ptr(since)
		it.Tags = []string{}
		index[it.ID] = len(list)
		list = append(list, it)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return list, nil
	}
	if err := s.fillItemSummaries(ctx, warehouseID, list, index); err != nil {
		return nil, err
	}
	return list, nil
}

// fillItemSummaries 一次查询补齐属性摘要、标签与首张图片，避免 GROUP_CONCAT 长度截断。
// fillItemSummaries fills attribute summaries, tags and first images in one query, avoiding GROUP_CONCAT truncation.
func (s *Store) fillItemSummaries(ctx context.Context, warehouseID int64, list []model.ItemSummary, index map[int64]int) error {
	rows, err := s.db.QueryContext(ctx,
		`SELECT av.item_id, ad.name, ad.type, av.value
		 FROM attribute_value av
		 JOIN attribute_def ad ON ad.id = av.attribute_def_id
		 WHERE ad.warehouse_id=? AND av.value <> ''
		 ORDER BY ad.sort_order, ad.id`, warehouseID)
	if err != nil {
		return err
	}
	defer rows.Close()
	summaries := map[int64][]string{}
	seenTags := map[int64]map[string]bool{}
	for rows.Next() {
		var itemID int64
		var name, value string
		var typ model.AttrType
		if err := rows.Scan(&itemID, &name, &typ, &value); err != nil {
			return err
		}
		i, ok := index[itemID]
		if !ok {
			continue
		}
		it := &list[i]
		switch typ {
		case model.AttrImage:
			if it.FirstImage == "" {
				it.FirstImage = value
			}
		case model.AttrTags:
			if seenTags[itemID] == nil {
				seenTags[itemID] = map[string]bool{}
			}
			for _, t := range model.DecodeOptions(value) {
				if !seenTags[itemID][t] {
					seenTags[itemID][t] = true
					it.Tags = append(it.Tags, t)
				}
			}
		default:
			summaries[itemID] = append(summaries[itemID], name+": "+value)
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for id, parts := range summaries {
		list[index[id]].AttrSummary = strings.Join(parts, "  ")
	}
	return nil
}

func scanItem(row interface{ Scan(...any) error }) (*model.Item, error) {
	var it model.Item
	var since sql.NullInt64
	if err := row.Scan(&it.ID, &it.CategoryID, &it.Name, &it.Unit, &it.Quantity, &it.AvgPrice,
		&it.TotalValue, &since, &it.CreatedAt); err != nil {
		return nil, err
	}
	it.StockSince = nullInt64Ptr(since)
	return &it, nil
}

func (s *Store) GetItemDetail(ctx context.Context, userID, itemID int64) (*model.ItemDetail, error) {
	whID, err := warehouseOfItem(ctx, s.db, userID, itemID, false)
	if err != nil {
		return nil, err
	}
	it, err := scanItem(s.db.QueryRowContext(ctx,
		`SELECT id, category_id, name, unit, quantity, avg_price, total_value, stock_since, created_at
		 FROM item WHERE id=?`, itemID))
	if err != nil {
		return nil, notFoundIfNoRows(err)
	}
	defs, err := listAttrDefs(ctx, s.db, whID)
	if err != nil {
		return nil, err
	}
	values, err := attrValuesOfItem(ctx, s.db, itemID)
	if err != nil {
		return nil, err
	}
	attrs := make([]model.AttrWithValue, 0, len(defs))
	for _, d := range defs {
		attrs = append(attrs, model.AttrWithValue{Def: d, Value: values[d.ID]})
	}
	return &model.ItemDetail{Item: *it, WarehouseID: whID, Attrs: attrs}, nil
}

func attrValuesOfItem(ctx context.Context, q querier, itemID int64) (map[int64]string, error) {
	rows, err := q.QueryContext(ctx, `SELECT attribute_def_id, value FROM attribute_value WHERE item_id=?`, itemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := map[int64]string{}
	for rows.Next() {
		var defID int64
		var v string
		if err := rows.Scan(&defID, &v); err != nil {
			return nil, err
		}
		values[defID] = v
	}
	return values, rows.Err()
}

func validateItemInput(in *model.ItemInput) error {
	in.Name = strings.TrimSpace(in.Name)
	in.Unit = strings.TrimSpace(in.Unit)
	if in.Name == "" {
		return model.Invalid("请输入物品名称")
	}
	if len([]rune(in.Name)) > 255 {
		return model.Invalid("物品名称过长")
	}
	if len([]rune(in.Unit)) > 32 {
		return model.Invalid("计量单位过长")
	}
	if in.CategoryID <= 0 {
		return model.Invalid("请选择所属类目")
	}
	return nil
}

// SaveItem 新建（itemID 为 nil）或更新物品及其属性值；图片在事务成功后才清理旧文件。
// SaveItem creates (nil itemID) or updates an item with its attribute values; old images are pruned only after commit.
func (s *Store) SaveItem(ctx context.Context, userID int64, itemID *int64, in model.ItemInput) (int64, error) {
	if err := validateItemInput(&in); err != nil {
		return 0, err
	}
	type prune struct {
		itemID, defID int64
		keep          string
	}
	var newFiles []string
	var prunes []prune
	var id int64

	err := s.inTx(ctx, func(tx *sql.Tx) error {
		whID, err := warehouseOfCategory(ctx, tx, userID, in.CategoryID)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				return model.Invalid("所属类目不存在")
			}
			return err
		}
		if itemID == nil {
			res, err := tx.ExecContext(ctx,
				`INSERT INTO item(category_id, name, unit, quantity, created_at) VALUES(?, ?, ?, 0, ?)`,
				in.CategoryID, in.Name, in.Unit, nowMillis())
			if err != nil {
				return err
			}
			if id, err = res.LastInsertId(); err != nil {
				return err
			}
		} else {
			id = *itemID
			oldWh, err := warehouseOfItem(ctx, tx, userID, id, false)
			if err != nil {
				return err
			}
			if oldWh != whID {
				return model.Invalid("只能切换到同一仓库下的类目")
			}
			if _, err := tx.ExecContext(ctx,
				`UPDATE item SET name=?, unit=?, category_id=? WHERE id=?`,
				in.Name, in.Unit, in.CategoryID, id); err != nil {
				return err
			}
		}

		defs, err := listAttrDefs(ctx, tx, whID)
		if err != nil {
			return err
		}
		defByID := map[int64]model.AttributeDef{}
		for _, d := range defs {
			defByID[d.ID] = d
		}
		oldValues := map[int64]string{}
		if itemID != nil {
			if oldValues, err = attrValuesOfItem(ctx, tx, id); err != nil {
				return err
			}
		}
		for _, v := range in.Values {
			def, ok := defByID[v.DefID]
			if !ok {
				continue
			}
			// 候选项调整后，物品上未改动的旧单选值原样保留
			// Keep an unchanged old choice value even if it is no longer among the options.
			if def.Type == model.AttrChoice && v.Value != "" && strings.TrimSpace(v.Value) == oldValues[def.ID] {
				continue
			}
			if def.Type == model.AttrImage {
				switch {
				case v.StagedImage != "":
					rel, err := s.images.Commit(userID, id, def.ID, v.StagedImage)
					if err != nil {
						return err
					}
					newFiles = append(newFiles, rel)
					if err := upsertValue(ctx, tx, def.ID, id, rel); err != nil {
						return err
					}
					prunes = append(prunes, prune{id, def.ID, rel})
				case v.RemoveImage:
					if err := deleteValue(ctx, tx, def.ID, id); err != nil {
						return err
					}
					prunes = append(prunes, prune{id, def.ID, ""})
				}
				continue
			}
			value, err := normalizeValue(def, v.Value)
			if err != nil {
				return err
			}
			if value == "" {
				err = deleteValue(ctx, tx, def.ID, id)
			} else {
				err = upsertValue(ctx, tx, def.ID, id, value)
			}
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		for _, rel := range newFiles {
			s.images.Delete(rel)
		}
		return 0, err
	}
	for _, p := range prunes {
		s.images.PruneAttrDir(p.itemID, p.defID, p.keep)
	}
	return id, nil
}

// normalizeValue 按属性类型校验并规整属性值；返回空串表示清除该值。
// normalizeValue validates and normalizes a value by attribute type; an empty result clears it.
func normalizeValue(def model.AttributeDef, raw string) (string, error) {
	v := strings.TrimSpace(raw)
	if v == "" {
		return "", nil
	}
	switch def.Type {
	case model.AttrNumber:
		if _, err := strconv.ParseFloat(v, 64); err != nil {
			return "", model.Invalid("属性「" + def.Name + "」必须是数字")
		}
	case model.AttrChoice:
		found := false
		for _, o := range def.Options {
			if o == v {
				found = true
				break
			}
		}
		if !found {
			return "", model.Invalid("属性「" + def.Name + "」的选项无效")
		}
	case model.AttrTags:
		v = model.EncodeOptions(model.DecodeOptions(v))
	default:
		if len([]rune(v)) > 2000 {
			return "", model.Invalid("属性「" + def.Name + "」内容过长")
		}
	}
	return v, nil
}

func upsertValue(ctx context.Context, q querier, defID, itemID int64, value string) error {
	_, err := q.ExecContext(ctx,
		`INSERT INTO attribute_value(attribute_def_id, item_id, value) VALUES(?, ?, ?)
		 ON DUPLICATE KEY UPDATE value=VALUES(value)`, defID, itemID, value)
	return err
}

func deleteValue(ctx context.Context, q querier, defID, itemID int64) error {
	_, err := q.ExecContext(ctx, `DELETE FROM attribute_value WHERE attribute_def_id=? AND item_id=?`, defID, itemID)
	return err
}

// SoftDeleteItems 批量把仓库内的物品移入回收站（图片保留），返回实际移入数量。
// SoftDeleteItems moves items of the warehouse to the recycle bin (images kept) and returns how many moved.
func (s *Store) SoftDeleteItems(ctx context.Context, userID, warehouseID int64, ids []int64) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	if len(ids) > 1000 {
		return 0, model.Invalid("一次最多删除 1000 个物品")
	}
	if err := s.CheckWarehouse(ctx, userID, warehouseID); err != nil {
		return 0, err
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := []any{nowMillis(), warehouseID}
	for _, id := range ids {
		args = append(args, id)
	}
	res, err := s.db.ExecContext(ctx,
		`UPDATE item SET deleted_at=? WHERE deleted_at IS NULL
		   AND category_id IN (SELECT id FROM category WHERE warehouse_id=?)
		   AND id IN (`+placeholders+`)`, args...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (s *Store) ListDeletedItems(ctx context.Context, userID, warehouseID int64) ([]model.DeletedRow, error) {
	if err := s.CheckWarehouse(ctx, userID, warehouseID); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT i.id, i.name, c.name, i.deleted_at FROM item i JOIN category c ON c.id = i.category_id
		 WHERE c.warehouse_id=? AND i.deleted_at IS NOT NULL ORDER BY i.deleted_at DESC`, warehouseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []model.DeletedRow{}
	for rows.Next() {
		var r model.DeletedRow
		if err := rows.Scan(&r.ID, &r.Title, &r.Subtitle, &r.DeletedAt); err != nil {
			return nil, err
		}
		list = append(list, r)
	}
	return list, rows.Err()
}

// deletedItemOfUser 校验物品在回收站中且其仓库属于该用户、未被删除。
// deletedItemOfUser verifies the item is in the recycle bin and its warehouse is the user's and active.
func deletedItemOfUser(ctx context.Context, q querier, userID, itemID int64) error {
	var id int64
	err := q.QueryRowContext(ctx,
		`SELECT i.id FROM item i
		 JOIN category c ON c.id = i.category_id
		 JOIN warehouse w ON w.id = c.warehouse_id
		 WHERE i.id=? AND w.user_id=? AND w.deleted_at IS NULL AND i.deleted_at IS NOT NULL`,
		itemID, userID).Scan(&id)
	return notFoundIfNoRows(err)
}

func (s *Store) RestoreItem(ctx context.Context, userID, itemID int64) error {
	if err := deletedItemOfUser(ctx, s.db, userID, itemID); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `UPDATE item SET deleted_at=NULL WHERE id=?`, itemID)
	return err
}

// DestroyItem 彻底删除回收站中的物品与其图片；流水保留。
// DestroyItem permanently deletes a recycled item and its images; stock records are kept.
func (s *Store) DestroyItem(ctx context.Context, userID, itemID int64) error {
	if err := deletedItemOfUser(ctx, s.db, userID, itemID); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM item WHERE id=?`, itemID); err != nil {
		return err
	}
	s.images.DeleteItemImages(itemID)
	return nil
}
