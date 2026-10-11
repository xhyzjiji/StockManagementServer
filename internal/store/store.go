// Package store 是 MySQL 数据访问层；所有业务查询都按 userID 做归属校验。
// Package store is the MySQL data access layer; every business query is scoped by userID ownership.
package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"private/construction_warehouse/internal/imagestore"
	"private/construction_warehouse/internal/model"
)

type querier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type Store struct {
	db     *sql.DB
	images *imagestore.Store
}

func New(db *sql.DB, images *imagestore.Store) *Store {
	return &Store{db: db, images: images}
}

func (s *Store) DB() *sql.DB               { return s.db }
func (s *Store) Images() *imagestore.Store { return s.images }

func nowMillis() int64 { return time.Now().UnixMilli() }

// inTx 在事务中执行 fn；fn 返回错误或 panic 时回滚。
// inTx runs fn inside a transaction, rolling back on error or panic.
func (s *Store) inTx(ctx context.Context, fn func(tx *sql.Tx) error) (err error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p)
		}
		if err != nil {
			tx.Rollback()
		}
	}()
	if err = fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

func notFoundIfNoRows(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return model.ErrNotFound
	}
	return err
}

// CheckWarehouse 校验仓库属于该用户且未被删除。
// CheckWarehouse verifies the warehouse belongs to the user and is not deleted.
func (s *Store) CheckWarehouse(ctx context.Context, userID, warehouseID int64) error {
	return checkWarehouse(ctx, s.db, userID, warehouseID)
}

func checkWarehouse(ctx context.Context, q querier, userID, warehouseID int64) error {
	var id int64
	err := q.QueryRowContext(ctx,
		`SELECT id FROM warehouse WHERE id=? AND user_id=? AND deleted_at IS NULL`,
		warehouseID, userID).Scan(&id)
	return notFoundIfNoRows(err)
}

// warehouseOfCategory 返回类目所属的（属于该用户、未删除的）仓库 id。
// warehouseOfCategory returns the warehouse id of a category owned by the user in an active warehouse.
func warehouseOfCategory(ctx context.Context, q querier, userID, categoryID int64) (int64, error) {
	var whID int64
	err := q.QueryRowContext(ctx,
		`SELECT c.warehouse_id FROM category c JOIN warehouse w ON w.id = c.warehouse_id
		 WHERE c.id=? AND w.user_id=? AND w.deleted_at IS NULL`,
		categoryID, userID).Scan(&whID)
	return whID, notFoundIfNoRows(err)
}

// warehouseOfItem 返回物品所属仓库 id；includeDeleted 为 true 时允许回收站中的物品。
// warehouseOfItem returns the item's warehouse id; includeDeleted allows items in the recycle bin.
func warehouseOfItem(ctx context.Context, q querier, userID, itemID int64, includeDeleted bool) (int64, error) {
	query := `SELECT c.warehouse_id FROM item i
		JOIN category c ON c.id = i.category_id
		JOIN warehouse w ON w.id = c.warehouse_id
		WHERE i.id=? AND w.user_id=? AND w.deleted_at IS NULL`
	if !includeDeleted {
		query += ` AND i.deleted_at IS NULL`
	}
	var whID int64
	err := q.QueryRowContext(ctx, query, itemID, userID).Scan(&whID)
	return whID, notFoundIfNoRows(err)
}

// UserOwnsItemFiles 判断该用户是否可以读取物品图片（含回收站内的物品与仓库）。
// UserOwnsItemFiles reports whether the user may read an item's images (including recycled items and warehouses).
func (s *Store) UserOwnsItemFiles(ctx context.Context, userID, itemID int64) bool {
	var id int64
	err := s.db.QueryRowContext(ctx,
		`SELECT i.id FROM item i
		 JOIN category c ON c.id = i.category_id
		 JOIN warehouse w ON w.id = c.warehouse_id
		 WHERE i.id=? AND w.user_id=?`, itemID, userID).Scan(&id)
	return err == nil
}

func itemIDsOfWarehouse(ctx context.Context, q querier, warehouseID int64) ([]int64, error) {
	return queryIDs(ctx, q,
		`SELECT i.id FROM item i JOIN category c ON c.id = i.category_id WHERE c.warehouse_id=?`, warehouseID)
}

func queryIDs(ctx context.Context, q querier, query string, args ...any) ([]int64, error) {
	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func nullInt64Ptr(v sql.NullInt64) *int64 {
	if !v.Valid {
		return nil
	}
	return &v.Int64
}

func nullFloatPtr(v sql.NullFloat64) *float64 {
	if !v.Valid {
		return nil
	}
	return &v.Float64
}
