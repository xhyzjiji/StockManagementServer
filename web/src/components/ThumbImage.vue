<script setup lang="ts">
defineProps<{ src?: string }>();
</script>

<template>
  <div class="thumb">
    <img v-if="src" :src="src" alt="" loading="lazy" />
    <van-icon v-else name="photo-o" />
  </div>
</template>
