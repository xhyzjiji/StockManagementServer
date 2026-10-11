<script setup lang="ts">
import GlobalDialogs from '@/components/GlobalDialogs.vue';
</script>

<template>
  <!-- 以 path 作为 key：参数变化时重建页面，仅 query 变化（如筛选条件）时保留状态 -->
  <!-- Keyed by path: pages are rebuilt when params change but keep state when only the query (filters) changes. -->
  <router-view :key="$route.path" />
  <GlobalDialogs />
</template>
