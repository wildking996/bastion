<template>
  <div class="topbar">
    <div class="topbar__left">
      <el-tooltip :content="t('menuToggle')" placement="bottom">
        <el-button
          :icon="app.sidebarCollapsed ? Expand : Fold"
          text
          @click="app.toggleSidebar()"
        />
      </el-tooltip>

      <div class="brand">
        <AppLogo />
        <span class="brand__name">Bastion</span>
      </div>
    </div>

    <div class="topbar__right">
      <el-tooltip :content="t('actions.refresh')" placement="bottom">
        <el-button :icon="Refresh" text @click="reload" />
      </el-tooltip>

      <el-tooltip :content="t('actions.shutdown')" placement="bottom">
        <el-button :icon="SwitchButton" type="danger" text @click="openShutdown" />
      </el-tooltip>

      <div class="lang">
        <span class="lang__label">{{ t("actions.language") }}</span>
        <el-button-group>
          <el-button
            size="small"
            :type="app.language === 'zh' ? 'primary' : 'default'"
            @click="setLanguage('zh')"
          >
            中文
          </el-button>
          <el-button
            size="small"
            :type="app.language === 'en' ? 'primary' : 'default'"
            @click="setLanguage('en')"
          >
            EN
          </el-button>
        </el-button-group>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { Expand, Fold, Refresh, SwitchButton } from "@element-plus/icons-vue";
import { ElMessage } from "element-plus";
import { useI18n } from "vue-i18n";

import { api } from "@/api/client";
import AppLogo from "@/components/AppLogo.vue";
import { useAppStore, type AppLanguage } from "@/store/app";
import { useConfirmDialogStore } from "@/store/confirm";

const app = useAppStore();
const confirm = useConfirmDialogStore();
const { t } = useI18n({
  messages: {
    zh: { menuToggle: "展开/收起侧边栏" },
    en: { menuToggle: "Toggle sidebar" },
  },
});

function setLanguage(lang: AppLanguage) {
  if (app.language === lang) return;
  app.setLanguage(lang);
  ElMessage.success(lang === "zh" ? "已切换为中文" : "Switched to English");
}

function reload() {
  window.location.reload();
}

function openShutdown() {
  confirm.open({
    title: t("dialogs.shutdownTitle"),
    alert: t("dialogs.shutdownAlert"),
    async generate() {
      const res = await api.post("/shutdown/generate-code");
      return { code: String(res.data.code), expiresAt: Number(res.data.expires_at) };
    },
    async submit(code: string) {
      await api.post("/shutdown/verify", { code });
    },
    successToast: "Shutdown initiated",
  });
}
</script>

<style scoped>
.topbar {
  height: 56px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 10px;
}

.topbar__left {
  display: flex;
  align-items: center;
  gap: 10px;
}

.brand {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 6px 10px;
  border-radius: 10px;
  border: 1px solid var(--app-border);
  background: var(--app-panel);
}

.brand__name {
  font-weight: 700;
  letter-spacing: 0.02em;
}

.topbar__right {
  display: flex;
  align-items: center;
  gap: 10px;
}

.lang {
  display: flex;
  align-items: center;
  gap: 8px;
}

.lang__label {
  opacity: 0.8;
  font-size: 12px;
}
</style>
