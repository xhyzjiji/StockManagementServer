// 仓库数据包导入导出，数据包格式与 App 互通。
// Warehouse package import / export; the package format is interchangeable with the App.
import { showLoadingToast } from 'vant';
import { importApi, warehouseApi } from '@/api';
import type { ImportMode } from '@/api/types';
import { pickOption } from './dialogs';
import { showError, toast } from './feedback';
import { pickFile } from './image';

export function exportWarehouse(id: number): void {
  const a = document.createElement('a');
  a.href = warehouseApi.exportUrl(id);
  a.download = '';
  document.body.append(a);
  a.click();
  a.remove();
  toast('正在导出，浏览器将开始下载');
}

// importPackage 两步导入：上传并预检 → 同名冲突时选择处理方式 → 执行导入；成功返回 true。
// importPackage imports in two steps: upload & pre-check → resolve a name conflict → execute; resolves true on success.
export async function importPackage(): Promise<boolean> {
  const file = await pickFile('.zip,application/zip,application/x-zip-compressed');
  if (!file) return false;
  let loading = showLoadingToast({ message: '正在上传并解析…', forbidClick: true, duration: 0 });
  let prepared;
  try {
    prepared = await importApi.prepare(file);
  } catch (e) {
    loading.close();
    showError(e);
    return false;
  }
  loading.close();
  const p = prepared.preview;
  let mode: ImportMode = 'keep_both';
  if (p.has_conflict) {
    const pick = await pickOption(
      '同名仓库冲突',
      [`保留两者（自动改名为「${p.suggested_name}」导入）`, '覆盖同名仓库（原仓库移入回收站）'],
      `已存在同名仓库「${p.warehouse_name}」，请选择处理方式`,
      '取消导入',
    );
    if (pick < 0) return false;
    mode = pick === 0 ? 'keep_both' : 'overwrite';
  }
  loading = showLoadingToast({
    message: `正在导入「${p.warehouse_name}」\n${p.category_count} 个类目、${p.item_count} 件物品、${p.record_count} 条流水、${p.image_count} 张图片`,
    forbidClick: true,
    duration: 0,
  });
  try {
    const r = await importApi.commit(prepared.file, mode);
    loading.close();
    let msg = `已导入「${r.final_name}」：${r.category_count} 个类目、${r.item_count} 件物品、${r.record_count} 条流水`;
    if (r.missing_image_count > 0) msg += `，${r.missing_image_count} 张参考图缺失`;
    if (r.overwritten_count > 0) msg += '，原仓库已移入回收站';
    toast(msg, 4000);
    return true;
  } catch (e) {
    loading.close();
    showError(e);
    return false;
  }
}
