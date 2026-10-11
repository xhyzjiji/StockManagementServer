<script setup lang="ts">
// 我的仓库：仓库列表、新建 / 编辑 / 删除、导入导出数据包、账号菜单。
// My warehouses: list, create / edit / delete, package import & export, account menu.
import { computed, reactive, ref } from 'vue';
import { useRouter } from 'vue-router';
import PageLayout from '@/components/PageLayout.vue';
import LoadState from '@/components/LoadState.vue';
import MoreMenu from '@/components/MoreMenu.vue';
import WarehouseDialog from '@/components/WarehouseDialog.vue';
import PasswordDialog from '@/components/PasswordDialog.vue';
import type { MenuItem } from '@/components/types';
import { authApi, warehouseApi } from '@/api';
import type { Warehouse } from '@/api/types';
import { useUserStore } from '@/stores/user';
import { confirm, showError, toast } from '@/utils/feedback';
import { usePageData } from '@/utils/loader';
import { exportWarehouse, importPackage } from '@/utils/transfer';

const router = useRouter();
const store = useUserStore();
const { data: list, loading, reload } = usePageData(() => warehouseApi.list());

const dialog = reactive({
  show: false, title: '', name: '', remark: '',
  onOk: async (_name: string, _remark: string) => {},
});
const showPassword = ref(false);

const accountMenu = computed<MenuItem[]>(() => [
  { text: `当前账号：${store.user?.username ?? ''}`, onClick: () => {} },
  { text: '修改密码', onClick: () => { showPassword.value = true; } },
  { text: '退出登录', danger: true, onClick: logout },
]);

function cardMenu(w: Warehouse): MenuItem[] {
  return [
    { text: '导出数据包', onClick: () => exportWarehouse(w.id) },
    { text: '编辑', onClick: () => openEdit(w) },
    { text: '删除', danger: true, onClick: () => remove(w) },
  ];
}

function openCreate() {
  Object.assign(dialog, {
    show: true, title: '新建仓库', name: '', remark: '',
    onOk: async (name: string, remark: string) => {
      const res = await warehouseApi.create(name, remark);
      router.push(`/w/${res.id}`);
    },
  });
}

function openEdit(w: Warehouse) {
  Object.assign(dialog, {
    show: true, title: '编辑仓库', name: w.name, remark: w.remark,
    onOk: async (name: string, remark: string) => {
      await warehouseApi.update(w.id, name, remark);
      await reload();
    },
  });
}

async function remove(w: Warehouse) {
  if (!(await confirm(`删除仓库 "${w.name}"？将先移入回收站，可在回收站恢复或彻底删除。`))) return;
  try {
    await warehouseApi.remove(w.id);
    toast('已移入回收站');
    await reload();
  } catch (e) {
    showError(e);
  }
}

async function onImport() {
  if (await importPackage()) await reload();
}

async function logout() {
  try {
    await authApi.logout();
  } catch (e) {
    showError(e);
  }
  store.clear();
  router.replace('/login');
}
</script>

<template>
  <PageLayout title="我的仓库">
    <template #actions>
      <span class="nav-text" @click="onImport">导入</span>
      <van-icon name="delete-o" title="仓库回收站" @click="router.push('/bin')" />
      <MoreMenu :items="accountMenu" icon="user-o" />
    </template>

    <LoadState :loading="loading">
      <van-empty v-if="!list?.length" class="card pre-line" description="暂无仓库&#10;点击右下角按钮创建你的第一个仓库" />
      <div v-else class="wh-grid">
        <div v-for="w in list" :key="w.id" class="card wh-card" @click="router.push(`/w/${w.id}`)">
          <div class="wh-icon"><van-icon name="shop-o" /></div>
          <div class="grow">
            <h3 class="ellipsis">{{ w.name }}</h3>
            <div class="muted small ellipsis">{{ w.remark || `${w.item_count} 种物品` }}</div>
            <div v-if="w.remark" class="muted small">{{ w.item_count }} 种物品</div>
          </div>
          <div class="more" @click.stop><MoreMenu :items="cardMenu(w)" /></div>
        </div>
      </div>
    </LoadState>

    <template #footer>
      <button class="fab" @click="openCreate"><van-icon name="plus" /><span class="fab-label">新建仓库</span></button>
    </template>
  </PageLayout>

  <WarehouseDialog v-model:show="dialog.show" :title="dialog.title" :name="dialog.name" :remark="dialog.remark" :on-ok="dialog.onOk" />
  <PasswordDialog v-model:show="showPassword" />
</template>
