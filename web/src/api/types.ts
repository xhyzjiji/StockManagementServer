// 接口数据类型，与后端 internal/model、internal/store 的 JSON 结构一致；时间均为毫秒时间戳。
// API data types matching the JSON of internal/model and internal/store; all times are millisecond timestamps.

export type AttrType = 'TEXT' | 'NUMBER' | 'CHOICE' | 'TAGS' | 'IMAGE';
export type StockType = 'IN' | 'OUT';
export type Pricing = 'WEIGHTED' | 'FIRST_PRICE' | 'CORRECT';
export type ImportMode = 'keep_both' | 'overwrite';

export interface User {
  id: number;
  username: string;
  created_at: number;
}

export interface AuthConfig {
  allow_register: boolean;
  token_ttl_seconds: number;
}

export interface Warehouse {
  id: number;
  name: string;
  remark: string;
  created_at: number;
  item_count: number;
}

export interface DeletedRow {
  id: number;
  title: string;
  subtitle: string;
  deleted_at: number;
}

export interface Category {
  id: number;
  warehouse_id: number;
  name: string;
  item_count: number;
}

export interface Item {
  id: number;
  category_id: number;
  name: string;
  unit: string;
  quantity: number;
  avg_price: number;
  total_value: number;
  stock_since: number | null;
  created_at: number;
}

export interface ItemSummary extends Item {
  tags: string[];
  attr_summary: string;
  first_image: string;
}

export interface AttributeDef {
  id: number;
  warehouse_id: number;
  name: string;
  type: AttrType;
  options: string[];
  sort_order: number;
}

export interface AttrWithValue {
  def: AttributeDef;
  value: string;
}

export interface ItemDetail {
  item: Item;
  warehouse_id: number;
  attrs: AttrWithValue[];
}

export interface StockRecord {
  id: number;
  warehouse_id: number;
  item_id: number;
  item_name: string;
  unit: string;
  type: StockType;
  quantity: number;
  before_qty: number;
  after_qty: number;
  total_price: number | null;
  note: string;
  created_at: number;
  item_exists: boolean;
}

export interface Page<T> {
  list: T[];
  total: number;
}

export interface PricePoint {
  time: number;
  price: number;
}

export interface WarehouseStats {
  item_kinds: number;
  total_value: number;
  in_days: Record<string, number>;
  out_days: Record<string, number>;
  since_day: number;
  tag_defs: AttributeDef[];
}

export interface TagSlice {
  label: string;
  quantity: number;
  unit: string;
}

export interface AttrDefDraft {
  id: number | null;
  name: string;
  type: AttrType;
  options: string[];
}

export interface AttrValueInput {
  def_id: number;
  value?: string;
  staged_image?: string;
  remove_image?: boolean;
}

export interface ItemInput {
  category_id: number;
  name: string;
  unit: string;
  values: AttrValueInput[];
}

export interface StockInput {
  type: StockType;
  quantity: number;
  total_price?: number;
  note: string;
  pricing?: Pricing;
  corrected_value?: number;
  occurred_at?: number;
}

export interface ImportPreview {
  warehouse_name: string;
  category_count: number;
  item_count: number;
  record_count: number;
  image_count: number;
  has_conflict: boolean;
  suggested_name: string;
}

export interface ImportResult {
  warehouse_id: number;
  final_name: string;
  category_count: number;
  item_count: number;
  record_count: number;
  image_count: number;
  missing_image_count: number;
  overwritten_count: number;
}

export interface ItemQuery {
  category_id?: number;
  q?: string;
  by?: 'name' | 'tag';
  sort?: '' | 'days_desc' | 'days_asc';
}

// ---------------- 作品 / Works ----------------

export interface Work {
  id: number;
  name: string;
  remark: string;
  quantity: number;
  total_minutes: number;
  avg_minutes: number;
  material_cost: number;
  unit_material_cost: number;
  cover_image: string;
  created_at: number;
}

export interface WorkImage {
  id: number;
  path: string;
  sort_order: number;
  is_cover: boolean;
}

export interface WorkMaterial {
  id: number;
  item_id: number;
  warehouse_id: number;
  warehouse_name: string;
  item_name: string;
  unit: string;
  quantity: number;
  unit_price: number;
  line_cost: number;
}

export interface WorkBatch {
  id: number;
  quantity: number;
  minutes: number;
  note: string;
  created_at: number;
}

export interface WorkDetail {
  work: Work;
  images: WorkImage[];
  materials: WorkMaterial[];
  batches: WorkBatch[];
}

export interface WorkImageInput {
  staged_image: string;
  is_cover: boolean;
}

export interface WorkMaterialInput {
  item_id: number;
  quantity: number;
}

export interface WorkInput {
  name: string;
  remark: string;
  images: WorkImageInput[];
  materials: WorkMaterialInput[];
  quantity: number;
  minutes: number;
  note: string;
}

export interface WorkBatchInput {
  quantity: number;
  minutes: number;
  note: string;
}

export interface MaterialPick {
  item_id: number;
  item_name: string;
  unit: string;
  quantity: number;
  avg_price: number;
  warehouse_id: number;
  warehouse_name: string;
  first_image: string;
}
