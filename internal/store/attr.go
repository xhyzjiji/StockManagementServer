package store

import (
	"context"
	"database/sql"
	"strings"

	"private/construction_warehouse/internal/model"
)

func (s *Store) ListAttrDefs(ctx context.Context, userID, warehouseID int64) ([]model.AttributeDef, error) {
	if err := s.CheckWarehouse(ctx, userID, warehouseID); err != nil {
		return nil, err
	}
	return listAttrDefs(ctx, s.db, warehouseID)
}

func listAttrDefs(ctx context.Context, q querier, warehouseID int64) ([]model.AttributeDef, error) {
	rows, err := q.QueryContext(ctx,
		`SELECT id, warehouse_id, name, type, options, sort_order FROM attribute_def
		 WHERE warehouse_id=? ORDER BY sort_order, id`, warehouseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []model.AttributeDef{}
	for rows.Next() {
		var d model.AttributeDef
		var opts string
		if err := rows.Scan(&d.ID, &d.WarehouseID, &d.Name, &d.Type, &opts, &d.SortOrder); err != nil {
			return nil, err
		}
		d.Options = model.DecodeOptions(opts)
		list = append(list, d)
	}
	return list, rows.Err()
}

func validateAttrDrafts(drafts []model.AttrDefDraft) error {
	seen := map[string]bool{}
	for i := range drafts {
		d := &drafts[i]
		d.Name = strings.TrimSpace(d.Name)
		if d.Name == "" {
			return model.Invalid("属性名不能为空")
		}
		if len([]rune(d.Name)) > 64 {
			return model.Invalid("属性名过长")
		}
		if seen[d.Name] {
			return model.Invalid("属性名重复：" + d.Name)
		}
		seen[d.Name] = true
		if !d.Type.Valid() {
			return model.Invalid("未知的属性类型：" + string(d.Type))
		}
		opts := model.DecodeOptions(model.EncodeOptions(d.Options))
		switch d.Type {
		case model.AttrChoice:
			if len(opts) < 2 {
				return model.Invalid("单选属性「" + d.Name + "」至少需要两个选项")
			}
		case model.AttrTags:
			if len(opts) < 1 {
				return model.Invalid("标签属性「" + d.Name + "」至少需要一个候选值")
			}
		default:
			opts = nil
		}
		d.Options = opts
	}
	return nil
}

// SaveAttrDefs 以整表覆盖的方式保存仓库属性定义（顺序即排序）。
// 删除定义或类型变更时，该仓库下所有物品对应的属性值与图片一并清除。
//
// SaveAttrDefs replaces the warehouse attribute definitions as a whole (list order is the sort order).
// Removing a definition or changing its type clears every item's value and images for it.
func (s *Store) SaveAttrDefs(ctx context.Context, userID, warehouseID int64, drafts []model.AttrDefDraft) error {
	if err := validateAttrDrafts(drafts); err != nil {
		return err
	}
	type imageDir struct{ itemID, defID int64 }
	var purgeDirs []imageDir

	err := s.inTx(ctx, func(tx *sql.Tx) error {
		var locked int64
		if err := tx.QueryRowContext(ctx,
			`SELECT id FROM warehouse WHERE id=? AND user_id=? AND deleted_at IS NULL FOR UPDATE`,
			warehouseID, userID).Scan(&locked); err != nil {
			return notFoundIfNoRows(err)
		}
		oldDefs, err := listAttrDefs(ctx, tx, warehouseID)
		if err != nil {
			return err
		}
		oldByID := map[int64]model.AttributeDef{}
		for _, d := range oldDefs {
			oldByID[d.ID] = d
		}
		kept := map[int64]bool{}
		for _, d := range drafts {
			if d.ID != nil {
				if _, ok := oldByID[*d.ID]; !ok {
					return model.Invalid("属性定义已不存在，请刷新后重试")
				}
				kept[*d.ID] = true
			}
		}

		purge := func(def model.AttributeDef) error {
			if def.Type == model.AttrImage {
				itemIDs, err := queryIDs(ctx, tx,
					`SELECT item_id FROM attribute_value WHERE attribute_def_id=?`, def.ID)
				if err != nil {
					return err
				}
				for _, itemID := range itemIDs {
					purgeDirs = append(purgeDirs, imageDir{itemID, def.ID})
				}
			}
			_, err := tx.ExecContext(ctx, `DELETE FROM attribute_value WHERE attribute_def_id=?`, def.ID)
			return err
		}

		for _, old := range oldDefs {
			if kept[old.ID] {
				continue
			}
			if err := purge(old); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `DELETE FROM attribute_def WHERE id=?`, old.ID); err != nil {
				return err
			}
		}
		for i, d := range drafts {
			opts := model.EncodeOptions(d.Options)
			if d.ID == nil {
				if _, err := tx.ExecContext(ctx,
					`INSERT INTO attribute_def(warehouse_id, name, type, options, sort_order) VALUES(?, ?, ?, ?, ?)`,
					warehouseID, d.Name, d.Type, opts, i); err != nil {
					return err
				}
				continue
			}
			old := oldByID[*d.ID]
			if old.Type != d.Type {
				if err := purge(old); err != nil {
					return err
				}
			}
			if _, err := tx.ExecContext(ctx,
				`UPDATE attribute_def SET name=?, type=?, options=?, sort_order=? WHERE id=?`,
				d.Name, d.Type, opts, i, old.ID); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	for _, d := range purgeDirs {
		s.images.DeleteAttrDir(d.itemID, d.defID)
	}
	return nil
}
