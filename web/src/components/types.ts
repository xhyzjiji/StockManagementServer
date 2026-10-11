import type { AttrType } from '@/api/types';

// AttrDraftRow 属性设置页的编辑草稿；id 为空表示新增，originalType 用于提示类型切换会清空数据。
// AttrDraftRow is an attribute-settings draft; null id means new, originalType warns that a type change clears data.
export interface AttrDraftRow {
  id: number | null;
  name: string;
  type: AttrType;
  options: string[];
  originalType: AttrType | null;
}

export type ItemView = 'list' | 'grid' | 'thumb';

export interface MenuItem {
  text: string;
  danger?: boolean;
  onClick: () => void;
}

export interface SelectOption<V> {
  text: string;
  value: V;
}
