package store

import (
	"context"
	"sort"

	"private/construction_warehouse/internal/model"
	"private/construction_warehouse/internal/stockmath"
)

const dayMillis = 24 * 60 * 60 * 1000

// WarehouseStats 统计页概览与出入库频率。
// WarehouseStats holds the stats overview and inbound/outbound activity.
type WarehouseStats struct {
	ItemKinds  int     `json:"item_kinds"`
	TotalValue float64 `json:"total_value"`
	// 本地纪元日（按客户端时区）→ 笔数
	// Local epoch day (client time zone) → record count.
	InDays  map[int64]int `json:"in_days"`
	OutDays map[int64]int `json:"out_days"`
	// 统计窗口起始的本地纪元日
	// Local epoch day where the stats window starts.
	SinceDay int64                `json:"since_day"`
	TagDefs  []model.AttributeDef `json:"tag_defs"`
}

// Stats tzOffsetMinutes 为客户端时区相对 UTC 的分钟偏移（东八区为 480）。
// Stats takes tzOffsetMinutes as the client's offset from UTC in minutes (480 for UTC+8).
func (s *Store) Stats(ctx context.Context, userID, warehouseID int64, tzOffsetMinutes int, weeks int) (*WarehouseStats, error) {
	if err := s.CheckWarehouse(ctx, userID, warehouseID); err != nil {
		return nil, err
	}
	if weeks <= 0 || weeks > 104 {
		weeks = 26
	}
	if tzOffsetMinutes < -14*60 || tzOffsetMinutes > 14*60 {
		tzOffsetMinutes = 0
	}
	offset := int64(tzOffsetMinutes) * 60 * 1000
	st := &WarehouseStats{InDays: map[int64]int{}, OutDays: map[int64]int{}}

	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*), COALESCE(SUM(i.total_value), 0) FROM item i JOIN category c ON c.id = i.category_id
		 WHERE c.warehouse_id=? AND i.deleted_at IS NULL`, warehouseID).Scan(&st.ItemKinds, &st.TotalValue); err != nil {
		return nil, err
	}
	st.TotalValue = stockmath.Round2(st.TotalValue)

	todayLocal := (nowMillis() + offset) / dayMillis
	st.SinceDay = todayLocal - int64(weeks*7) + 1
	sinceMillis := st.SinceDay*dayMillis - offset
	rows, err := s.db.QueryContext(ctx,
		`SELECT (created_at + ?) DIV ? AS d, type, COUNT(*) FROM stock_record
		 WHERE warehouse_id=? AND created_at >= ? GROUP BY d, type`,
		offset, dayMillis, warehouseID, sinceMillis)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var day int64
		var typ model.StockType
		var n int
		if err := rows.Scan(&day, &typ, &n); err != nil {
			return nil, err
		}
		if typ == model.StockOut {
			st.OutDays[day] += n
		} else {
			st.InDays[day] += n
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	defs, err := listAttrDefs(ctx, s.db, warehouseID)
	if err != nil {
		return nil, err
	}
	st.TagDefs = []model.AttributeDef{}
	for _, d := range defs {
		if d.Type == model.AttrTags {
			st.TagDefs = append(st.TagDefs, d)
		}
	}
	return st, nil
}

// TagSlice 饼图切片：标签值（或物品名）与剩余数量。
// TagSlice is a pie slice: a tag value (or item name) with its remaining quantity.
type TagSlice struct {
	Label    string  `json:"label"`
	Quantity float64 `json:"quantity"`
	// 按物品分组时的计量单位
	// Unit when grouped by item.
	Unit string `json:"unit"`
}

// TagDistribution 标签字段下在库物品的剩余数量分布；groupByItem 为 true 时每个物品一片。
// TagDistribution aggregates remaining quantity of in-stock items under a tag field; groupByItem gives one slice per item.
func (s *Store) TagDistribution(ctx context.Context, userID, warehouseID, defID int64, groupByItem bool) ([]TagSlice, error) {
	if err := s.CheckWarehouse(ctx, userID, warehouseID); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT i.id, i.name, i.unit, i.quantity, av.value
		 FROM attribute_value av
		 JOIN attribute_def ad ON ad.id = av.attribute_def_id
		 JOIN item i ON i.id = av.item_id
		 WHERE ad.id=? AND ad.warehouse_id=? AND ad.type='TAGS'
		   AND av.value <> '' AND i.deleted_at IS NULL AND i.quantity > 0`, defID, warehouseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var order []string
	byKey := map[string]*TagSlice{}
	add := func(key, label, unit string, qty float64) {
		sl, ok := byKey[key]
		if !ok {
			sl = &TagSlice{Label: label, Unit: unit}
			byKey[key] = sl
			order = append(order, key)
		}
		sl.Quantity += qty
	}
	for rows.Next() {
		var id int64
		var name, unit, value string
		var qty float64
		if err := rows.Scan(&id, &name, &unit, &qty, &value); err != nil {
			return nil, err
		}
		if groupByItem {
			add(name, name, unit, qty)
			continue
		}
		for _, tag := range model.DecodeOptions(value) {
			add(tag, tag, "", qty)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]TagSlice, 0, len(order))
	for _, k := range order {
		out = append(out, *byKey[k])
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Quantity > out[j].Quantity })
	return out, nil
}

// PricePoints 物品历次入库单价（时间升序），仅统计录入了总价的入库。
// PricePoints returns the item's inbound unit prices in time order, only for records with a total price.
func (s *Store) PricePoints(ctx context.Context, userID, itemID int64) ([]model.PricePoint, error) {
	whID, err := warehouseOfItem(ctx, s.db, userID, itemID, false)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT created_at, total_price, quantity FROM stock_record
		 WHERE warehouse_id=? AND item_id=? AND type='IN' AND total_price IS NOT NULL AND quantity > 0
		 ORDER BY created_at, id`, whID, itemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []model.PricePoint{}
	for rows.Next() {
		var at int64
		var total, qty float64
		if err := rows.Scan(&at, &total, &qty); err != nil {
			return nil, err
		}
		list = append(list, model.PricePoint{Time: at, Price: stockmath.Round2(total / qty)})
	}
	return list, rows.Err()
}
