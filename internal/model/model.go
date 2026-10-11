// Package model 定义业务实体与接口出入参。时间字段均为毫秒时间戳。
// Package model defines domain entities and API payloads. All times are millisecond timestamps.
package model

import (
	"errors"
	"strings"
)

// AttrType 属性类型：文本 / 数字 / 单选 / 多选标签 / 图片。
// AttrType is the attribute type: text / number / single choice / multi-select tags / image.
type AttrType string

const (
	AttrText   AttrType = "TEXT"
	AttrNumber AttrType = "NUMBER"
	AttrChoice AttrType = "CHOICE"
	AttrTags   AttrType = "TAGS"
	AttrImage  AttrType = "IMAGE"
)

func (t AttrType) Valid() bool {
	switch t {
	case AttrText, AttrNumber, AttrChoice, AttrTags, AttrImage:
		return true
	}
	return false
}

type StockType string

const (
	StockIn  StockType = "IN"
	StockOut StockType = "OUT"
)

var (
	ErrNotFound  = errors.New("记录不存在")
	ErrForbidden = errors.New("无权访问")
)

// ValidationError 参数校验失败，接口层映射为 400。
// ValidationError is an input validation failure, mapped to HTTP 400.
type ValidationError struct{ Msg string }

func (e *ValidationError) Error() string { return e.Msg }

func Invalid(msg string) error { return &ValidationError{Msg: msg} }

// InsufficientStockError 出库超量，接口层映射为 409。
// InsufficientStockError means an outbound exceeds stock, mapped to HTTP 409.
type InsufficientStockError struct {
	Available float64
	Requested float64
}

func (e *InsufficientStockError) Error() string { return "库存不足" }

// 单选/标签候选值内部以 | 分隔存储；用户输入支持逗号、顿号、竖线分隔。
// Choice/tag options are stored "|"-joined; user input may use commas, Chinese enumeration commas or pipes.

func EncodeOptions(options []string) string {
	var out []string
	for _, o := range options {
		if o = strings.TrimSpace(strings.ReplaceAll(o, "|", "")); o != "" {
			out = append(out, o)
		}
	}
	return strings.Join(out, "|")
}

func DecodeOptions(raw string) []string {
	out := []string{}
	for _, o := range strings.Split(raw, "|") {
		if o = strings.TrimSpace(o); o != "" {
			out = append(out, o)
		}
	}
	return out
}

type User struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	CreatedAt int64  `json:"created_at"`
}

type Warehouse struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Remark    string `json:"remark"`
	CreatedAt int64  `json:"created_at"`
	ItemCount int    `json:"item_count"`
}

// DeletedRow 回收站条目：仓库或物品。
// DeletedRow is a recycle bin entry: a warehouse or an item.
type DeletedRow struct {
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	Subtitle  string `json:"subtitle"`
	DeletedAt int64  `json:"deleted_at"`
}

type Category struct {
	ID          int64  `json:"id"`
	WarehouseID int64  `json:"warehouse_id"`
	Name        string `json:"name"`
	ItemCount   int    `json:"item_count"`
}

type Item struct {
	ID         int64   `json:"id"`
	CategoryID int64   `json:"category_id"`
	Name       string  `json:"name"`
	Unit       string  `json:"unit"`
	Quantity   float64 `json:"quantity"`
	AvgPrice   float64 `json:"avg_price"`
	TotalValue float64 `json:"total_value"`
	StockSince *int64  `json:"stock_since"`
	CreatedAt  int64   `json:"created_at"`
}

// ItemSummary 物品列表行：附带非图片/非标签属性摘要、标签与首张图片。
// ItemSummary is an item list row with a text summary of attributes, tags and the first image.
type ItemSummary struct {
	Item
	Tags        []string `json:"tags"`
	AttrSummary string   `json:"attr_summary"`
	FirstImage  string   `json:"first_image"`
}

type AttributeDef struct {
	ID          int64    `json:"id"`
	WarehouseID int64    `json:"warehouse_id"`
	Name        string   `json:"name"`
	Type        AttrType `json:"type"`
	Options     []string `json:"options"`
	SortOrder   int      `json:"sort_order"`
}

type AttrWithValue struct {
	Def   AttributeDef `json:"def"`
	Value string       `json:"value"`
}

type ItemDetail struct {
	Item        Item            `json:"item"`
	WarehouseID int64           `json:"warehouse_id"`
	Attrs       []AttrWithValue `json:"attrs"`
}

type StockRecord struct {
	ID          int64     `json:"id"`
	WarehouseID int64     `json:"warehouse_id"`
	ItemID      int64     `json:"item_id"`
	ItemName    string    `json:"item_name"`
	Unit        string    `json:"unit"`
	Type        StockType `json:"type"`
	Quantity    float64   `json:"quantity"`
	BeforeQty   float64   `json:"before_qty"`
	AfterQty    float64   `json:"after_qty"`
	TotalPrice  *float64  `json:"total_price"`
	Note        string    `json:"note"`
	CreatedAt   int64     `json:"created_at"`
	// ItemExists 物品是否仍存在且未删除（流水列表展示"物品已删除"用）
	// ItemExists tells whether the item still exists and is not deleted (for the "item deleted" hint).
	ItemExists bool `json:"item_exists"`
}

type PricePoint struct {
	Time  int64   `json:"time"`
	Price float64 `json:"price"`
}

// AttrDefDraft 属性设置页提交的定义草稿，ID 为空表示新增。
// AttrDefDraft is a definition draft from the attribute settings page; nil ID means new.
type AttrDefDraft struct {
	ID      *int64   `json:"id"`
	Name    string   `json:"name"`
	Type    AttrType `json:"type"`
	Options []string `json:"options"`
}

// AttrValueInput 物品编辑页提交的属性值。
// AttrValueInput is an attribute value submitted by the item edit page.
type AttrValueInput struct {
	DefID int64 `json:"def_id"`
	// 文本/数字/单选的值；标签为 | 分隔的已选值
	// Value for text/number/choice; tags are "|"-joined selections.
	Value string `json:"value"`
	// 图片类型新上传的暂存路径（tmp/...）
	// Staged path (tmp/...) of a newly uploaded image.
	StagedImage string `json:"staged_image"`
	// 图片类型要求清除已有图片
	// Asks to clear the existing image.
	RemoveImage bool `json:"remove_image"`
}

type ItemInput struct {
	CategoryID int64            `json:"category_id"`
	Name       string           `json:"name"`
	Unit       string           `json:"unit"`
	Values     []AttrValueInput `json:"values"`
}

type StockInput struct {
	Type           StockType `json:"type"`
	Quantity       float64   `json:"quantity"`
	TotalPrice     *float64  `json:"total_price"`
	Note           string    `json:"note"`
	Pricing        string    `json:"pricing"`
	CorrectedValue *float64  `json:"corrected_value"`
	OccurredAt     *int64    `json:"occurred_at"`
}
