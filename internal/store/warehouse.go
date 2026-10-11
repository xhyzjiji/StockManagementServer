package store

import (
	"context"
	"database/sql"

	"private/construction_warehouse/internal/model"
)

func (s *Store) ListWarehouses(ctx context.Context, userID int64) ([]model.Warehouse, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT w.id, w.name, w.remark, w.created_at,
		        (SELECT COUNT(*) FROM item i JOIN category c ON c.id = i.category_id
		          WHERE c.warehouse_id = w.id AND i.deleted_at IS NULL)
		 FROM warehouse w WHERE w.user_id=? AND w.deleted_at IS NULL ORDER BY w.id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []model.Warehouse{}
	for rows.Next() {
		var w model.Warehouse
		if err := rows.Scan(&w.ID, &w.Name, &w.Remark, &w.CreatedAt, &w.ItemCount); err != nil {
			return nil, err
		}
		list = append(list, w)
	}
	return list, rows.Err()
}

func (s *Store) GetWarehouse(ctx context.Context, userID, id int64) (*model.Warehouse, error) {
	var w model.Warehouse
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, remark, created_at FROM warehouse WHERE id=? AND user_id=? AND deleted_at IS NULL`,
		id, userID).Scan(&w.ID, &w.Name, &w.Remark, &w.CreatedAt)
	if err != nil {
		return nil, notFoundIfNoRows(err)
	}
	return &w, nil
}

func (s *Store) CreateWarehouse(ctx context.Context, userID int64, name, remark string) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO warehouse(user_id, name, remark, created_at) VALUES(?, ?, ?, ?)`,
		userID, name, remark, nowMillis())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) UpdateWarehouse(ctx context.Context, userID, id int64, name, remark string) error {
	if err := s.CheckWarehouse(ctx, userID, id); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `UPDATE warehouse SET name=?, remark=? WHERE id=?`, name, remark, id)
	return err
}

// SoftDeleteWarehouse 移入回收站（图片保留，彻底删除时才清理）。
// SoftDeleteWarehouse moves the warehouse to the recycle bin (images kept until permanent deletion).
func (s *Store) SoftDeleteWarehouse(ctx context.Context, userID, id int64) error {
	if err := s.CheckWarehouse(ctx, userID, id); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `UPDATE warehouse SET deleted_at=? WHERE id=?`, nowMillis(), id)
	return err
}

func (s *Store) ListDeletedWarehouses(ctx context.Context, userID int64) ([]model.DeletedRow, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, remark, deleted_at FROM warehouse
		 WHERE user_id=? AND deleted_at IS NOT NULL ORDER BY deleted_at DESC`, userID)
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

func (s *Store) RestoreWarehouse(ctx context.Context, userID, id int64) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE warehouse SET deleted_at=NULL WHERE id=? AND user_id=? AND deleted_at IS NOT NULL`, id, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return model.ErrNotFound
	}
	return nil
}

// DestroyWarehouse 彻底删除回收站中的仓库：类目、物品、属性、流水随外键级联删除，图片目录手动清理。
// DestroyWarehouse permanently deletes a recycled warehouse; rows cascade via FKs, image dirs are removed manually.
func (s *Store) DestroyWarehouse(ctx context.Context, userID, id int64) error {
	var itemIDs []int64
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		var found int64
		if err := tx.QueryRowContext(ctx,
			`SELECT id FROM warehouse WHERE id=? AND user_id=? AND deleted_at IS NOT NULL FOR UPDATE`,
			id, userID).Scan(&found); err != nil {
			return notFoundIfNoRows(err)
		}
		var err error
		if itemIDs, err = itemIDsOfWarehouse(ctx, tx, id); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `DELETE FROM warehouse WHERE id=?`, id)
		return err
	})
	if err != nil {
		return err
	}
	for _, itemID := range itemIDs {
		s.images.DeleteItemImages(itemID)
	}
	return nil
}
