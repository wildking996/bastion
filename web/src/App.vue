<template>
  <el-config-provider :locale="epLocale">
    <AppLayout />
  </el-config-provider>
</template>

<script setup lang="ts">
import { computed } from "vue";

import en from "element-plus/es/locale/lang/en";
import zhCn from "element-plus/es/locale/lang/zh-cn";

import AppLayout from "@/layout/AppLayout.vue";
import { useAppStore } from "@/store/app";

const app = useAppStore();

const epLocale = computed(() => (app.language === "zh" ? zhCn : en));
</script>
