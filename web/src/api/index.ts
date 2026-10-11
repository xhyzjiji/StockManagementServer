// 业务接口：每个后端路由对应一个带类型的函数，页面只依赖这里。
// Domain API: one typed function per backend route; pages depend only on this module.
import { http, qs } from './http';
import type {
  AttrDefDraft, AttributeDef, AuthConfig, Category, DeletedRow, ImportMode, ImportPreview, ImportResult,
  ItemDetail, ItemInput, ItemQuery, ItemSummary, MaterialPick, Page, PricePoint, StockInput, StockRecord,
  TagSlice, User, Warehouse, WarehouseStats, Work, WorkBatchInput, WorkDetail, WorkInput,
} from './types';

type Ok = { ok: boolean };
type Created = { id: number };

export const authApi = {
  config: () => http.get<AuthConfig>('/api/auth/config'),
  me: () => http.get<{ user: User }>('/api/auth/me'),
  login: (username: string, password: string) => http.post<{ user: User }>('/api/auth/login', { username, password }),
  register: (username: string, password: string) => http.post<{ user: User }>('/api/auth/register', { username, password }),
  logout: () => http.post<Ok>('/api/auth/logout'),
  changePassword: (oldPassword: string, newPassword: string) =>
    http.post<Ok>('/api/auth/password', { old_password: oldPassword, new_password: newPassword }),
};

export const warehouseApi = {
  list: () => http.get<Warehouse[]>('/api/warehouses'),
  get: (id: number) => http.get<Warehouse>(`/api/warehouses/${id}`),
  create: (name: string, remark: string) => http.post<Created>('/api/warehouses', { name, remark }),
  update: (id: number, name: string, remark: string) => http.put<Ok>(`/api/warehouses/${id}`, { name, remark }),
  remove: (id: number) => http.del<Ok>(`/api/warehouses/${id}`),
  exportUrl: (id: number) => `/api/warehouses/${id}/export`,
  listDeleted: () => http.get<DeletedRow[]>('/api/recycle/warehouses'),
  restore: (id: number) => http.post<Ok>(`/api/recycle/warehouses/${id}/restore`),
  destroy: (id: number) => http.del<Ok>(`/api/recycle/warehouses/${id}`),
  stats: (id: number, tzOffset: number, weeks: number) =>
    http.get<WarehouseStats>(`/api/warehouses/${id}/stats` + qs({ tz_offset: tzOffset, weeks })),
  tagStats: (id: number, defId: number, byItem: boolean) =>
    http.get<TagSlice[]>(`/api/warehouses/${id}/stats/tags` + qs({ def_id: defId, group: byItem ? 'item' : '' })),
  records: (id: number, offset: number, limit: number) =>
    http.get<Page<StockRecord>>(`/api/warehouses/${id}/records` + qs({ offset, limit })),
};

export const importApi = {
  prepare: (file: File) => http.upload<{ file: string; preview: ImportPreview }>('/api/import/prepare', file),
  commit: (file: string, mode: ImportMode) => http.post<ImportResult>('/api/import/commit', { file, mode }),
};

export const categoryApi = {
  list: (warehouseId: number) => http.get<Category[]>(`/api/warehouses/${warehouseId}/categories`),
  get: (id: number) => http.get<Category>(`/api/categories/${id}`),
  create: (warehouseId: number, name: string) => http.post<Created>(`/api/warehouses/${warehouseId}/categories`, { name }),
  rename: (id: number, name: string) => http.put<Ok>(`/api/categories/${id}`, { name }),
  remove: (id: number) => http.del<Ok>(`/api/categories/${id}`),
};

export const attrApi = {
  list: (warehouseId: number) => http.get<AttributeDef[]>(`/api/warehouses/${warehouseId}/attrs`),
  save: (warehouseId: number, drafts: AttrDefDraft[]) => http.put<AttributeDef[]>(`/api/warehouses/${warehouseId}/attrs`, drafts),
};

export const itemApi = {
  list: (warehouseId: number, q: ItemQuery = {}) =>
    http.get<ItemSummary[]>(`/api/warehouses/${warehouseId}/items` + qs({ ...q, category_id: q.category_id || '' })),
  get: (id: number) => http.get<ItemDetail>(`/api/items/${id}`),
  create: (input: ItemInput) => http.post<Created>('/api/items', input),
  update: (id: number, input: ItemInput) => http.put<Ok>(`/api/items/${id}`, input),
  removeMany: (warehouseId: number, ids: number[]) => http.post<Ok>(`/api/warehouses/${warehouseId}/items/delete`, { ids }),
  listDeleted: (warehouseId: number) => http.get<DeletedRow[]>(`/api/warehouses/${warehouseId}/recycle/items`),
  restore: (id: number) => http.post<Ok>(`/api/recycle/items/${id}/restore`),
  destroy: (id: number) => http.del<Ok>(`/api/recycle/items/${id}`),
  stock: (id: number, input: StockInput) => http.post<Ok>(`/api/items/${id}/stock`, input),
  records: (id: number, offset: number, limit: number) =>
    http.get<Page<StockRecord>>(`/api/items/${id}/records` + qs({ offset, limit })),
  prices: (id: number) => http.get<PricePoint[]>(`/api/items/${id}/prices`),
};

export const uploadApi = {
  image: (file: Blob) => http.upload<{ path: string; url: string }>('/api/uploads/image', file, 'image.jpg'),
};

export const workApi = {
  list: () => http.get<Work[]>('/api/works'),
  get: (id: number) => http.get<WorkDetail>(`/api/works/${id}`),
  create: (input: WorkInput) => http.post<Created>('/api/works', input),
  update: (id: number, name: string, remark: string) => http.put<Ok>(`/api/works/${id}`, { name, remark }),
  remove: (id: number) => http.del<Ok>(`/api/works/${id}`),
  appendBatch: (id: number, input: WorkBatchInput) => http.post<Ok>(`/api/works/${id}/batches`, input),
};

export const materialApi = {
  // 必须先指定仓库；搜索只在该仓库内进行
  // warehouse_id is required; search is scoped to that warehouse.
  list: (warehouseId: number, q = '') =>
    http.get<MaterialPick[]>('/api/materials' + qs({ warehouse_id: warehouseId, q })),
};
