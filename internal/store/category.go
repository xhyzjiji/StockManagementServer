package store

import (
	"context"
	"database/sql"

	"private/construction_warehouse/internal/model"
)

func (s *Store) ListCategories(ctx context.Context, userID, warehouseID int64) ([]model.Category, error) {
	if err := s.CheckWarehouse(ctx, userID, warehouseID); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT c.id, c.warehouse_id, c.name,
		        (SELECT COUNT(*) FROM item i WHERE i.category_id = c.id AND i.deleted_at IS NULL)
		 FROM category c WHERE c.warehouse_id=? ORDER BY c.id`, warehouseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []model.Category{}
	for rows.Next() {
		var c model.Category
		if err := rows.Scan(&c.ID, &c.WarehouseID, &c.Name, &c.ItemCount); err != nil {
			return nil, err
		}
		list = append(list, c)
	}
	return list, rows.Err()
}

func (s *Store) GetCategory(ctx context.Context, userID, id int64) (*model.Category, error) {
	var c model.Category
	err := s.db.QueryRowContext(ctx,
		`SELECT c.id, c.warehouse_id, c.name,
		        (SELECT COUNT(*) FROM item i WHERE i.category_id = c.id AND i.deleted_at IS NULL)
		 FROM category c JOIN warehouse w ON w.id = c.warehouse_id
		 WHERE c.id=? AND w.user_id=? AND w.deleted_at IS NULL`, id, userID).
		Scan(&c.ID, &c.WarehouseID, &c.Name, &c.ItemCount)
	if err != nil {
		return nil, notFoundIfNoRows(err)
	}
	return &c, nil
}

func (s *Store) CreateCategory(ctx context.Context, userID, warehouseID int64, name string) (int64, error) {
	if err := s.CheckWarehouse(ctx, userID, warehouseID); err != nil {
		return 0, err
	}
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO category(warehouse_id, name, created_at) VALUES(?, ?, ?)`, warehouseID, name, nowMillis())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) RenameCategory(ctx context.Context, userID, id int64, name string) error {
	if _, err := warehouseOfCategory(ctx, s.db, userID, id); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `UPDATE category SET name=? WHERE id=?`, name, id)
	return err
}

// DeleteCategory 删除类目及其下全部物品（含回收站中的），与 App 行为一致。
// DeleteCategory deletes the category and all its items (recycled ones included), same as the App.
func (s *Store) DeleteCategory(ctx context.Context, userID, id int64) error {
	var itemIDs []int64
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		if _, err := warehouseOfCategory(ctx, tx, userID, id); err != nil {
			return err
		}
		var err error
		if itemIDs, err = queryIDs(ctx, tx, `SELECT id FROM item WHERE category_id=?`, id); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `DELETE FROM category WHERE id=?`, id)
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
