<template>
  <el-menu
    ref="menuRef"
    class="sidebar"
    :collapse="app.sidebarCollapsed"
    :default-active="route.path"
    :unique-opened="true"
    @select="onSelect"
  >
    <el-menu-item index="/home">
      <el-icon><House /></el-icon>
      <span>{{ t("menu.updates") }}</span>
    </el-menu-item>

    <el-sub-menu index="manage">
      <template #title>
        <el-icon><Tools /></el-icon>
        <span>{{ t("menu.manage") }}</span>
      </template>
      <el-menu-item index="/bastions">{{ t("menu.bastions") }}</el-menu-item>
      <el-menu-item index="/mappings">{{ t("menu.mappings") }}</el-menu-item>
    </el-sub-menu>

    <el-sub-menu index="logs">
      <template #title>
        <el-icon><Document /></el-icon>
        <span>{{ t("menu.logs") }}</span>
      </template>
      <el-menu-item index="/logs/http">{{ t("menu.httpLogs") }}</el-menu-item>
      <el-menu-item index="/logs/errors">{{ t("menu.errorLogs") }}</el-menu-item>
    </el-sub-menu>
  </el-menu>
</template>

<script setup lang="ts">
import { Document, House, Tools } from "@element-plus/icons-vue";
import { ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { useRoute, useRouter } from "vue-router";

import { useAppStore } from "@/store/app";

const app = useAppStore();
const router = useRouter();
const route = useRoute();
const { t } = useI18n();

const menuRef = ref<any | null>(null);

function groupForPath(p: string): "manage" | "logs" | "" {
  if (p === "/bastions" || p === "/mappings") return "manage";
  if (p.startsWith("/logs/")) return "logs";
  return "";
}

function applyAutoOpen(p: string) {
  const group = groupForPath(p);
  if (!menuRef.value) return;

  for (const g of ["manage", "logs"] as const) {
    if (g === group) menuRef.value.open(g);
    else menuRef.value.close(g);
  }
}

watch(
  () => route.path,
  (p) => applyAutoOpen(p),
  { immediate: true }
);

function onSelect(index: string) {
  if (index.startsWith("/")) router.push(index);
}
</script>

<style scoped>
.sidebar {
  height: calc(100vh - 56px);
  border-right: none;
  background: transparent;
}
</style>
