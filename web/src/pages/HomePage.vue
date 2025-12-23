<template>
  <div class="app-page">
    <el-card class="app-card">
      <template #header>
        <div class="card-header">
          <div class="card-header__title">
            <el-button size="small" :icon="themeIcon" @click="toggleTheme">
              {{ t(`theme.${nextTheme}`) }}
            </el-button>
            <span>{{ t("home.statusCard") }}</span>
          </div>

          <div class="card-header__actions">
            <el-button :loading="checking" @click="checkUpdate">
              {{ t("home.checkUpdate") }}
            </el-button>
            <el-button
              type="primary"
              :disabled="!updateStatus?.update_available"
              @click="openConfirmUpdate"
            >
              {{ t("home.confirmUpdate") }}
            </el-button>
          </div>
        </div>
      </template>

      <el-descriptions class="desc-fixed" :column="2" border label-width="120px">
        <el-descriptions-item :label="t('home.currentVersion')">
          {{ updateStatus?.current_version ?? t("home.unknown") }}
        </el-descriptions-item>
        <el-descriptions-item :label="t('home.latestVersion')">
          {{ updateStatus?.latest_version ?? t("home.unknown") }}
        </el-descriptions-item>
        <el-descriptions-item :label="t('home.status')" :span="2">
          <el-tag v-if="updateStatus" :type="updateStatus.update_available ? 'warning' : 'success'">
            {{ updateStatus.update_available ? t("home.updateAvailable") : t("home.upToDate") }}
          </el-tag>
          <el-tag v-else type="info">{{ t("home.unknown") }}</el-tag>
        </el-descriptions-item>
      </el-descriptions>

      <div v-if="applyResult?.helper_log_path" class="helper">
        <div class="helper__label">{{ t("home.helperLogPath") }}</div>
        <div class="helper__value mono">{{ applyResult.helper_log_path }}</div>
      </div>
    </el-card>

    <el-card class="app-card">
      <template #header>
        <span>{{ t("home.proxy") }}</span>
      </template>

      <el-collapse>
        <el-collapse-item :title="t('home.proxyDetails')" name="proxy">

          <el-descriptions class="desc-fixed" :title="t('home.effectiveRule')" :column="2" border label-width="120px">
            <el-descriptions-item :label="t('home.effectiveProxy')">
              {{ proxyInfo?.effective_proxy || "-" }}
            </el-descriptions-item>
            <el-descriptions-item :label="t('home.source')">
              <el-tag :type="sourceTagType">{{ sourceLabel }}</el-tag>
              <span v-if="sourceHint" class="source-hint">{{ sourceHint }}</span>
            </el-descriptions-item>
          </el-descriptions>
          
          <el-divider />
          
          <el-descriptions class="desc-fixed" :title="t('home.proxyEnv')" :column="2" border label-width="120px">
            <el-descriptions-item label="HTTP_PROXY">
              {{ proxyInfo?.env_http_proxy || "-" }}
            </el-descriptions-item>
            <el-descriptions-item label="HTTPS_PROXY">
              {{ proxyInfo?.env_https_proxy || "-" }}
            </el-descriptions-item>
            <el-descriptions-item label="ALL_PROXY">
              {{ proxyInfo?.env_all_proxy || "-" }}
            </el-descriptions-item>
            <el-descriptions-item label="NO_PROXY">
              {{ proxyInfo?.env_no_proxy || "-" }}
            </el-descriptions-item>
          </el-descriptions>

          <el-divider content-position="left">{{ t('home.proxyManual') }}</el-divider>

          <el-form label-width="160px">
            <el-form-item :label="t('home.manualProxy')">
              <el-input v-model="manualProxy" placeholder="http(s):// or socks5(h)://">
                <template #append>
                  <el-button-group>
                    <el-button type="primary" :loading="savingProxy" @click="saveProxy">{{ t("common.save") }}</el-button>
                    <el-button :loading="savingProxy" @click="clearProxy">{{ t("common.clear") }}</el-button>
                  </el-button-group>
                </template>
              </el-input>
            </el-form-item>
          </el-form>
        </el-collapse-item>
      </el-collapse>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { Moon, Sunny } from "@element-plus/icons-vue";
import { ElMessage } from "element-plus";
import { computed, onMounted, ref } from "vue";
import { useI18n } from "vue-i18n";

import { api } from "@/api/client";
import type { UpdateApplyResponse, UpdateCheckResponse, UpdateProxyResponse } from "@/api/types";
import { useAppStore } from "@/store/app";
import { useConfirmDialogStore } from "@/store/confirm";

const { t } = useI18n();

const confirm = useConfirmDialogStore();
const app = useAppStore();

const nextTheme = computed(() => (app.theme === "dark" ? "light" : "dark"));
const themeIcon = computed(() => (nextTheme.value === "dark" ? Moon : Sunny));

function toggleTheme() {
  app.toggleTheme();
  ElMessage.success(t(app.theme === "dark" ? "toast.themeDark" : "toast.themeLight"));
}

const checking = ref(false);
const updateStatus = ref<UpdateCheckResponse | null>(null);

const applyResult = ref<UpdateApplyResponse | null>(null);

const proxyInfo = ref<UpdateProxyResponse | null>(null);
const manualProxy = ref(app.manualUpdateProxy ?? "");
const savingProxy = ref(false);

const sourceKey = computed(() => proxyInfo.value?.source || "-");

const sourceLabel = computed(() => {
  if (sourceKey.value === "manual") return t("home.sourceManual");
  if (sourceKey.value === "env") return t("home.sourceEnv");
  if (sourceKey.value === "none") return t("home.sourceNone");
  return sourceKey.value;
});

const sourceHint = computed(() => {
  if (sourceKey.value === "manual") return t("home.sourceHintManual");
  if (sourceKey.value === "env") return t("home.sourceHintEnv");
  if (sourceKey.value === "none") return t("home.sourceHintNone");
  return "";
});

const sourceTagType = computed(() => {
  if (sourceKey.value === "manual") return "success";
  if (sourceKey.value === "env") return "info";
  if (sourceKey.value === "none") return "warning";
  return "info";
});

async function checkUpdate() {
  checking.value = true;
  try {
    const res = await api.get<UpdateCheckResponse>("/update/check");
    updateStatus.value = res.data;
  } finally {
    checking.value = false;
  }
}

function openConfirmUpdate() {
  confirm.open({
    title: t("dialogs.updateTitle"),
    alert: t("dialogs.updateAlert"),
    async generate() {
      const res = await api.post("/update/generate-code");
      return { code: String(res.data.code), expiresAt: Number(res.data.expires_at) };
    },
    async submit(code: string) {
      const res = await api.post<UpdateApplyResponse>("/update/apply", { code });
      applyResult.value = res.data;
    },
    successToast: t("toast.updateStarted"),
  });
}

async function loadProxy() {
  const res = await api.get<UpdateProxyResponse>("/update/proxy");
  proxyInfo.value = res.data;
}

async function saveProxy() {
  savingProxy.value = true;
  try {
    const value = manualProxy.value.trim();
    await api.post("/update/proxy", { proxy_url: value });
    if (value) app.setManualUpdateProxy(value);
    else app.clearManualUpdateProxy();
    await loadProxy();
    ElMessage.success(t("common.save"));
  } catch {
    return;
  } finally {
    savingProxy.value = false;
  }
}

async function clearProxy() {
  manualProxy.value = "";
  await saveProxy();
}

onMounted(() => {
  loadProxy().catch(() => undefined);
});
</script>

<style scoped>
.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
}

.card-header__title {
  display: flex;
  align-items: center;
  gap: 10px;
}

.card-header__actions {
  display: flex;
  gap: 10px;
}

.helper {
  margin-top: 12px;
  padding: 10px;
  border: 1px solid var(--app-border);
  border-radius: 10px;
  background: var(--app-panel);
}

.helper__label {
  opacity: 0.8;
  margin-bottom: 6px;
}

.source-hint {
  margin-left: 10px;
  color: var(--app-text-muted);
}

.desc-fixed :deep(.el-descriptions__table) {
  width: 100%;
  table-layout: fixed;
}

.desc-fixed :deep(.el-descriptions__content) {
  word-break: break-word;
}

.saved-proxy {
  display: flex;
  gap: 8px;
  align-items: baseline;
  padding: 6px 2px 0;
  color: var(--app-text-muted);
}

.saved-proxy__label {
  white-space: nowrap;
}
</style>
