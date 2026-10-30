<template>
  <div class="app-shell">
    <router-view />
    <van-tabbar route fixed v-if="showTabbar">
      <van-tabbar-item to="/menus" icon="notes-o">菜谱</van-tabbar-item>
      <van-tabbar-item to="/my/records" icon="orders-o">记录</van-tabbar-item>
      <van-tabbar-item to="/my" icon="user-o">我的</van-tabbar-item>
    </van-tabbar>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'

const route = useRoute()
// 登录/改密页不显示 Tabbar
const showTabbar = computed(() => !['/login', '/my/password'].includes(route.path))
</script>

<style>
body {
  margin: 0;
  background: #f7f8fa;
}
.app-shell {
  max-width: 480px;
  margin: 0 auto;
  min-height: 100vh;
  background: #f7f8fa;
}
.app-content {
  padding-bottom: 60px;
}
</style>
