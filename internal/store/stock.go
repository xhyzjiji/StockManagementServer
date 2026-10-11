package store

import (
	"context"
	"database/sql"
	"math"
	"strings"
	"time"

	"private/construction_warehouse/internal/model"
	"private/construction_warehouse/internal/stockmath"
)

const maxQuantity = 1e12

// hasAtMost2Decimals 金额最多两位小数（元到分）。
// hasAtMost2Decimals reports whether a money amount has at most 2 decimals (cents).
func hasAtMost2Decimals(v float64) bool {
	return math.Abs(v*100-math.Round(v*100)) < 1e-6
}

func validMoney(v *float64, name string) error {
	if v == nil {
		return model.Invalid("请输入" + name)
	}
	if math.IsNaN(*v) || math.IsInf(*v, 0) || *v < 0 || *v > maxQuantity {
		return model.Invalid(name + "必须是非负数")
	}
	if !hasAtMost2Decimals(*v) {
		return model.Invalid(name + "最多两位小数")
	}
	return nil
}

// ApplyStock 事务化出入库：锁定物品行，更新库存、平均价、剩余总价与在库起点，并写入流水。
// 老库存未定价（有库存但总价为 0）时才使用 in.Pricing 指定的计价方式，否则按加权计算。
//
// ApplyStock performs an inbound/outbound in one transaction: locks the item row, updates quantity,
// average price, stock value and stock-since, and writes a record.
// in.Pricing only applies when existing stock is unpriced (quantity > 0, value 0); otherwise weighting is used.
func (s *Store) ApplyStock(ctx context.Context, userID, itemID int64, in model.StockInput) (*model.StockRecord, error) {
	if in.Type != model.StockIn && in.Type != model.StockOut {
		return nil, model.Invalid("未知的出入库类型")
	}
	if math.IsNaN(in.Quantity) || in.Quantity <= 0 || in.Quantity > maxQuantity {
		return nil, model.Invalid("请输入大于 0 的数量")
	}
	in.Note = strings.TrimSpace(in.Note)
	if len([]rune(in.Note)) > 1000 {
		return nil, model.Invalid("备注过长")
	}
	pricing, ok := stockmath.ParsePricing(in.Pricing)
	if !ok {
		return nil, model.Invalid("未知的计价方式")
	}
	if in.Type == model.StockIn {
		if err := validMoney(in.TotalPrice, "本次入库总价"); err != nil {
			return nil, err
		}
		if pricing == stockmath.Correct {
			if err := validMoney(in.CorrectedValue, "老库存总价"); err != nil {
				return nil, err
			}
		}
	}

	var rec model.StockRecord
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		var (
			name, unit             string
			before, avg, valBefore float64
			whID                   int64
		)
		err := tx.QueryRowContext(ctx,
			`SELECT i.name, i.unit, i.quantity, i.avg_price, i.total_value, c.warehouse_id
			 FROM item i
			 JOIN category c ON c.id = i.category_id
			 JOIN warehouse w ON w.id = c.warehouse_id
			 WHERE i.id=? AND w.user_id=? AND w.deleted_at IS NULL AND i.deleted_at IS NULL
			 FOR UPDATE`, itemID, userID).
			Scan(&name, &unit, &before, &avg, &valBefore, &whID)
		if err != nil {
			return notFoundIfNoRows(err)
		}

		now := time.Now().UnixMilli()
		at := now
		var after float64
		var va stockmath.ValueAfter
		var recordPrice float64
		if in.Type == model.StockIn {
			if in.OccurredAt != nil && *in.OccurredAt > 0 && *in.OccurredAt < now {
				at = *in.OccurredAt
			}
			after = before + in.Quantity
			unpriced := before > stockmath.Epsilon && valBefore <= 0
			switch {
			case unpriced && pricing == stockmath.FirstPrice:
				va = stockmath.AfterStockInFirstPrice(before, in.Quantity, *in.TotalPrice)
			case unpriced && pricing == stockmath.Correct:
				va = stockmath.AfterStockIn(before, *in.CorrectedValue, in.Quantity, *in.TotalPrice)
			default:
				va = stockmath.AfterStockIn(before, valBefore, in.Quantity, *in.TotalPrice)
			}
			recordPrice = stockmath.Round2(*in.TotalPrice)
		} else {
			after = before - in.Quantity
			if after < -stockmath.Epsilon {
				return &model.InsufficientStockError{Available: before, Requested: in.Quantity}
			}
			if after < stockmath.Epsilon {
				after = 0
			}
			va = stockmath.AfterStockOut(after, avg)
			// 出库金额 = 库存总价的实际扣减（出清时为全部剩余总价）
			// Outbound amount = actual decrease of stock value (all remaining value once cleared).
			recordPrice = stockmath.Round2(valBefore - va.TotalValue)
		}

		sinceExpr := "stock_since"
		var sinceArgs []any
		switch {
		case in.Type == model.StockOut && after < stockmath.Epsilon:
			sinceExpr = "NULL"
		case in.Type == model.StockIn && before < stockmath.Epsilon:
			sinceExpr = "?"
			sinceArgs = append(sinceArgs, at)
		}
		args := append([]any{after, va.AvgPrice, va.TotalValue}, sinceArgs...)
		args = append(args, itemID)
		if _, err := tx.ExecContext(ctx,
			`UPDATE item SET quantity=?, avg_price=?, total_value=?, stock_since=`+sinceExpr+` WHERE id=?`,
			args...); err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx,
			`INSERT INTO stock_record(warehouse_id, item_id, item_name, unit, type, quantity, before_qty, after_qty,
			                          total_price, note, created_at)
			 VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			whID, itemID, name, unit, in.Type, in.Quantity, before, after, recordPrice, in.Note, at)
		if err != nil {
			return err
		}
		recID, err := res.LastInsertId()
		if err != nil {
			return err
		}
		rec = model.StockRecord{
			ID: recID, WarehouseID: whID, ItemID: itemID, ItemName: name, Unit: unit, Type: in.Type,
			Quantity: in.Quantity, BeforeQty: before, AfterQty: after, TotalPrice: &recordPrice,
			Note: in.Note, CreatedAt: at, ItemExists: true,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &rec, nil
}

// Page 分页结果。
// Page is a paged result.
type Page[T any] struct {
	List  []T   `json:"list"`
	Total int64 `json:"total"`
}

func clampPage(offset, limit int) (int, int) {
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	return offset, limit
}

const recordColumns = `r.id, r.warehouse_id, r.item_id, r.item_name, r.unit, r.type, r.quantity, r.before_qty,
	r.after_qty, r.total_price, r.note, r.created_at, i.id IS NOT NULL`

func scanRecords(rows *sql.Rows) ([]model.StockRecord, error) {
	defer rows.Close()
	list := []model.StockRecord{}
	for rows.Next() {
		var r model.StockRecord
		var price sql.NullFloat64
		if err := rows.Scan(&r.ID, &r.WarehouseID, &r.ItemID, &r.ItemName, &r.Unit, &r.Type, &r.Quantity,
			&r.BeforeQty, &r.AfterQty, &price, &r.Note, &r.CreatedAt, &r.ItemExists); err != nil {
			return nil, err
		}
		r.TotalPrice = nullFloatPtr(price)
		list = append(list, r)
	}
	return list, rows.Err()
}

// ListWarehouseRecords 仓库流水（新→旧），附带物品是否仍存在。
// ListWarehouseRecords returns warehouse records (newest first) with whether the item still exists.
func (s *Store) ListWarehouseRecords(ctx context.Context, userID, warehouseID int64, offset, limit int) (*Page[model.StockRecord], error) {
	if err := s.CheckWarehouse(ctx, userID, warehouseID); err != nil {
		return nil, err
	}
	offset, limit = clampPage(offset, limit)
	var total int64
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM stock_record WHERE warehouse_id=?`, warehouseID).Scan(&total); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+recordColumns+` FROM stock_record r
		 LEFT JOIN item i ON i.id = r.item_id AND i.deleted_at IS NULL
		 WHERE r.warehouse_id=? ORDER BY r.created_at DESC, r.id DESC LIMIT ? OFFSET ?`,
		warehouseID, limit, offset)
	if err != nil {
		return nil, err
	}
	list, err := scanRecords(rows)
	if err != nil {
		return nil, err
	}
	return &Page[model.StockRecord]{List: list, Total: total}, nil
}

func (s *Store) ListItemRecords(ctx context.Context, userID, itemID int64, offset, limit int) (*Page[model.StockRecord], error) {
	whID, err := warehouseOfItem(ctx, s.db, userID, itemID, false)
	if err != nil {
		return nil, err
	}
	offset, limit = clampPage(offset, limit)
	var total int64
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM stock_record WHERE warehouse_id=? AND item_id=?`, whID, itemID).Scan(&total); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+recordColumns+` FROM stock_record r
		 LEFT JOIN item i ON i.id = r.item_id AND i.deleted_at IS NULL
		 WHERE r.warehouse_id=? AND r.item_id=? ORDER BY r.created_at DESC, r.id DESC LIMIT ? OFFSET ?`,
		whID, itemID, limit, offset)
	if err != nil {
		return nil, err
	}
	list, err := scanRecords(rows)
	if err != nil {
		return nil, err
	}
	return &Page[model.StockRecord]{List: list, Total: total}, nil
}
