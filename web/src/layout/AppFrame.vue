<template>
  <el-container class="frame">
    <el-header class="frame__header">
      <TopBar />
    </el-header>
    <el-container>
      <el-aside :width="asideWidth" class="frame__aside">
        <SideBar />
      </el-aside>
      <el-main class="frame__main">
        <RouterView />
      </el-main>
    </el-container>

    <UnifiedConfirmCodeDialog />
  </el-container>
</template>

<script setup lang="ts">
import { computed, watch } from "vue";
import { RouterView } from "vue-router";

import UnifiedConfirmCodeDialog from "@/components/UnifiedConfirmCodeDialog.vue";
import SideBar from "@/layout/SideBar.vue";
import TopBar from "@/layout/TopBar.vue";
import { i18n } from "@/plugins/i18n";
import { useAppStore } from "@/store/app";

const app = useAppStore();

watch(
  () => app.language,
  (lang) => {
    i18n.global.locale.value = lang;
  },
  { immediate: true }
);

const asideWidth = computed(() => (app.sidebarCollapsed ? "64px" : "220px"));
</script>

<style scoped>
.frame {
  height: 100%;
}

.frame__header {
  padding: 0;
  border-bottom: 1px solid var(--app-border);
  background: var(--app-panel);
}

.frame__aside {
  border-right: 1px solid var(--app-border);
  background: var(--app-panel);
}

.frame__main {
  padding: 0;
}
</style>
