<script setup lang="ts">
// 登录后主页：服务简介、快捷入口（仓库 / 作品 / 灵感）、功能亮点与仓库说明书。
// Post-login home: service intro, quick entries (warehouse / works / inspiration),
// feature highlights and the warehouse how-to.
import { computed } from 'vue';
import { useRouter } from 'vue-router';
import AppShell from '@/components/AppShell.vue';
import { useUserStore } from '@/stores/user';

const router = useRouter();
const store = useUserStore();

const greeting = computed(() => {
  const h = new Date().getHours();
  if (h < 6) return '夜深了';
  if (h < 12) return '上午好';
  if (h < 14) return '中午好';
  if (h < 18) return '下午好';
  return '晚上好';
});

const modules = [
  {
    key: 'warehouse', path: '/warehouses', icon: 'shop-o', color: '#1565C0', bg: '#E3F2FD',
    title: '我的仓库', badge: '已上线',
    desc: '管理材料库存、自定义属性、出入库流水，支持导入导出',
  },
  {
    key: 'works', path: '/works', icon: 'photo-o', color: '#00897B', bg: '#E0F2F1',
    title: '我的作品', badge: '已上线',
    desc: '录入成品、关联材料算成本，追加数量并统计平均用时',
  },
  {
    key: 'inspiration', path: '/inspiration', icon: 'bulb-o', color: '#E65100', bg: '#FFF3E0',
    title: '创作灵感', badge: '开发中',
    desc: '收集灵感碎片，随手记录想法与素材',
  },
] as const;

const features = [
  { icon: 'cluster-o', title: '多仓库组织', text: '仓库 → 类目 → 物品三级结构，材料分类清晰' },
  { icon: 'setting-o', title: '自定义属性', text: '文本 / 数字 / 单选 / 标签 / 图片，全仓库共用' },
  { icon: 'balance-o', title: '出入库记账', text: '加权均价、流水完整可查，支持修正未定价库存' },
  { icon: 'exchange', title: '数据可迁移', text: '与手机 App 互通的 zip 数据包，导入导出一键完成' },
];

const manual = [
  '在「我的仓库」右下角新建仓库，再点进仓库创建类目',
  '右上菜单「属性设置」统一定义属性，标签候选也在这里',
  '添加物品时填写名称、单位，勾选标签；详情页可出入库',
  '删除的仓库 / 物品先进回收站，可恢复；彻底删除才不可逆',
  '仓库列表可导出数据包；右上角「导入」可从 App 迁入数据',
];
</script>

<template>
  <AppShell>
    <section class="home-hero card">
      <div class="home-hello">
        <h1>{{ greeting }}，{{ store.user?.username || '创作者' }}</h1>
        <p>管好材料库，记录每一次创作</p>
      </div>
      <van-button type="primary" round icon="shop-o" @click="router.push('/warehouses')">进入我的仓库</van-button>
    </section>

    <section class="card section">
      <h2>服务简介</h2>
      <p class="home-intro">
        造物仓是面向手工、DIY 与小作坊的创作材料库。把布料、五金、颜料等材料按仓库管理起来，
        出入库有据可查；成品可关联材料核算成本、统计用时。网页端与手机 App 仓库数据互通，灵感收集正在路上。
      </p>
    </section>

    <section class="card section">
      <h2>功能模块</h2>
      <div class="home-modules">
        <button
          v-for="m in modules" :key="m.key" type="button" class="home-mod"
          @click="router.push(m.path)"
        >
          <div class="home-mod-icon" :style="{ color: m.color, background: m.bg }">
            <van-icon :name="m.icon" />
          </div>
          <div class="grow">
            <div class="row" style="gap: 8px">
              <b>{{ m.title }}</b>
              <span class="home-badge" :class="{ ready: m.badge === '已上线' }">{{ m.badge }}</span>
            </div>
            <div class="muted small" style="margin-top: 4px; line-height: 1.5">{{ m.desc }}</div>
          </div>
          <van-icon name="arrow" class="muted" />
        </button>
      </div>
    </section>

    <section class="card section">
      <h2>能力亮点</h2>
      <div class="home-features">
        <div v-for="f in features" :key="f.title" class="home-feat">
          <div class="home-feat-icon"><van-icon :name="f.icon" /></div>
          <div>
            <b>{{ f.title }}</b>
            <div class="muted small" style="margin-top: 4px; line-height: 1.5">{{ f.text }}</div>
          </div>
        </div>
      </div>
    </section>

    <section class="card section">
      <h2>「我的仓库」怎么用</h2>
      <ol class="home-manual">
        <li v-for="(step, i) in manual" :key="i">{{ step }}</li>
      </ol>
    </section>
  </AppShell>
</template>
