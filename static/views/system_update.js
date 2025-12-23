import { apiJSON } from "../app/api.js";

const { ref, inject, computed, onMounted } = Vue;

export default {
  name: "ViewSystemActions",
  setup() {
    const t = inject("t");
    const openConfirmDialog = inject("openConfirmDialog");
    const ElMessage = ElementPlus.ElMessage;

    const checked = ref(false);

    const updateInfo = ref({});
    const updateChecking = ref(false);

    const updateProxyDetected = ref("");
    const updateProxyManual = ref("");
    const updateProxySaving = ref(false);
    const updateProxyCollapse = ref([]);

    const updateHelperLogPath = ref("");

    const updateAvailable = computed(
      () => updateInfo.value && updateInfo.value.update_available === true
    );

    const checkForUpdate = async () => {
      updateChecking.value = true;
      try {
        const data = await apiJSON("/update/check", "GET");
        updateInfo.value = data || {};
        checked.value = true;
        if (updateInfo.value.update_available !== true) {
          updateHelperLogPath.value = "";
        }
      } catch (e) {
        ElMessage.error(`${t.value.updateFailed}: ${e.message}`);
      } finally {
        updateChecking.value = false;
      }
    };

    const loadUpdateProxy = async () => {
      try {
        const data = await apiJSON("/update/proxy", "GET");
        const detected =
          data.env_https_proxy || data.env_http_proxy || data.env_all_proxy;
        updateProxyDetected.value = detected || "";
        updateProxyManual.value = data.manual_proxy || "";
      } catch {
        updateProxyDetected.value = "";
      }
    };

    const saveUpdateProxy = async () => {
      updateProxySaving.value = true;
      try {
        await apiJSON("/update/proxy", "POST", {
          proxy_url: updateProxyManual.value || "",
        });
        ElMessage.success(t.value.updateProxySaved);
        await loadUpdateProxy();
      } catch (e) {
        ElMessage.error(e.message);
      } finally {
        updateProxySaving.value = false;
      }
    };

    const clearUpdateProxy = async () => {
      updateProxySaving.value = true;
      try {
        await apiJSON("/update/proxy", "POST", { proxy_url: "" });
        updateProxyManual.value = "";
        ElMessage.success(t.value.updateProxyCleared);
        await loadUpdateProxy();
      } catch (e) {
        ElMessage.error(e.message);
      } finally {
        updateProxySaving.value = false;
      }
    };

    const openUpdateConfirm = () => {
      if (!checked.value) {
        ElMessage.warning(t.value.updateNotChecked);
        return;
      }
      if (!updateAvailable.value) {
        ElMessage.info(t.value.upToDate);
        return;
      }
      if (typeof openConfirmDialog !== "function") {
        ElMessage.error("确认弹窗未就绪");
        return;
      }

      openConfirmDialog({
        nextTitle: t.value.update,
        nextAlertTitle: t.value.updateConfirm,
        nextActionType: "warning",
        nextOnGenerate: async () => await apiJSON("/update/generate-code", "POST", {}),
        nextOnApply: async ({ code }) => {
          const data = await apiJSON("/update/apply", "POST", { code });
          updateHelperLogPath.value = data.helper_log_path || "";
          ElMessage.success(t.value.updateStarted);
        },
      });
    };

    onMounted(async () => {
      await loadUpdateProxy();
    });

    return {
      t,
      checked,
      updateInfo,
      updateChecking,
      updateProxyDetected,
      updateProxyManual,
      updateProxySaving,
      updateProxyCollapse,
      updateHelperLogPath,
      updateAvailable,
      checkForUpdate,
      saveUpdateProxy,
      clearUpdateProxy,
      openUpdateConfirm,
    };
  },
  template: `
    <div style="max-width: 900px; margin: 0 auto;">
      <el-row justify="space-between" align="middle" style="margin-bottom: 12px" :gutter="12">
        <el-col :span="24">
          <h2 style="margin: 0">{{ t.navUpdate }}</h2>
        </el-col>
      </el-row>

      <el-card shadow="never">
        <template #header>
          <div style="display:flex;justify-content:space-between;align-items:center;gap:10px;flex-wrap:wrap;">
            <b>{{ t.update }}</b>
            <el-space wrap>
              <el-button type="primary" :loading="updateChecking" @click="checkForUpdate">{{ t.checkUpdate }}</el-button>
              <el-button type="warning" :disabled="!checked || !updateAvailable" @click="openUpdateConfirm">{{ t.confirmUpdate }}</el-button>
            </el-space>
          </div>
        </template>

        <div v-if="!checked" class="muted" style="margin-bottom: 12px;">
          {{ t.updateNotChecked }}
        </div>

        <div v-if="updateHelperLogPath" style="margin-bottom: 12px">
          <el-alert type="info" :closable="false" :title="t.updateHelperLog" show-icon>
            <template #default>
              <div class="code" style="word-break: break-all">{{ updateHelperLogPath }}</div>
            </template>
          </el-alert>
        </div>

        <el-row :gutter="12">
          <el-col :span="12">
            <div class="muted" style="font-size: 12px">{{ t.currentVersion }}</div>
            <div class="code" style="margin-top: 4px">{{ updateInfo.current_version || '-' }}</div>
          </el-col>
          <el-col :span="12">
            <div class="muted" style="font-size: 12px">{{ t.latestVersion }}</div>
            <div class="code" style="margin-top: 4px">{{ updateInfo.latest_version || '-' }}</div>
          </el-col>
        </el-row>

        <div style="margin-top: 12px">
          <el-alert v-if="updateInfo.update_available === true" type="success" :closable="false" :title="t.updateAvailable" show-icon></el-alert>
          <el-alert v-else-if="checked" type="info" :closable="false" :title="t.upToDate" show-icon></el-alert>
        </div>

        <div style="margin-top: 12px">
          <el-collapse v-model="updateProxyCollapse">
            <el-collapse-item name="proxy" :title="t.updateProxy">
              <el-form label-width="160px">
                <el-form-item :label="t.updateProxyDetected">
                  <el-input :model-value="updateProxyDetected || '-'" readonly></el-input>
                </el-form-item>
                <el-form-item :label="t.updateProxyManual">
                  <el-input v-model="updateProxyManual" :placeholder="t.updateProxyPlaceholder" clearable></el-input>
                </el-form-item>
                <el-form-item label=" ">
                  <el-button type="primary" :loading="updateProxySaving" @click="saveUpdateProxy">{{ t.updateProxySave }}</el-button>
                  <el-button :loading="updateProxySaving" @click="clearUpdateProxy">{{ t.updateProxyClear }}</el-button>
                </el-form-item>
              </el-form>
            </el-collapse-item>
          </el-collapse>
        </div>
      </el-card>
    </div>
  `,
};
