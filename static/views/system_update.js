import { apiJSON } from "../app/api.js";

const { ref, inject, onMounted, onDeactivated } = Vue;

export default {
  name: "ViewSystemUpdate",
  setup() {
    const t = inject("t");
    const ElMessage = ElementPlus.ElMessage;

    const updateInfo = ref({});
    const updateChecking = ref(false);
    const updateApplying = ref(false);
    const updateGenerating = ref(false);

    const updateCode = ref("");
    const updateCodeExpiry = ref("");
    const updateInputCode = ref("");
    let updateExpiryTimer = null;

    const updateProxyDetected = ref("");
    const updateProxyManual = ref("");
    const updateProxySaving = ref(false);
    const updateProxyCollapse = ref([]);

    const updateHelperLogPath = ref("");

    const stopUpdateTimer = () => {
      if (!updateExpiryTimer) return;
      clearInterval(updateExpiryTimer);
      updateExpiryTimer = null;
    };

    const resetUpdateCode = () => {
      updateCode.value = "";
      updateCodeExpiry.value = "";
      updateInputCode.value = "";
      stopUpdateTimer();
    };

    const startUpdateExpiryCountdown = (expiresAtUnix) => {
      stopUpdateTimer();

      const tick = () => {
        const now = Math.floor(Date.now() / 1000);
        const left = Math.max(0, expiresAtUnix - now);
        if (left <= 0) {
          updateCodeExpiry.value = t.value.codeExpired;
          stopUpdateTimer();
          return;
        }
        const minutes = String(Math.floor(left / 60)).padStart(2, "0");
        const seconds = String(left % 60).padStart(2, "0");
        updateCodeExpiry.value = `${minutes}:${seconds}`;
      };

      tick();
      updateExpiryTimer = setInterval(tick, 1000);
    };

    const checkForUpdate = async () => {
      updateChecking.value = true;
      try {
        const data = await apiJSON("/update/check", "GET");
        updateInfo.value = data || {};
        if (updateInfo.value.update_available !== true) {
          resetUpdateCode();
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
        const detected = data.env_https_proxy || data.env_http_proxy || data.env_all_proxy;
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

    const generateUpdateCode = async () => {
      updateGenerating.value = true;
      try {
        const data = await apiJSON("/update/generate-code", "POST", {});
        updateCode.value = data.code;
        updateInputCode.value = "";
        ElMessage.success(t.value.codeGenerated);
        startUpdateExpiryCountdown(Number(data.expires_at || 0));
      } catch (e) {
        ElMessage.error(e.message);
      } finally {
        updateGenerating.value = false;
      }
    };

    const applyUpdate = async () => {
      updateApplying.value = true;
      try {
        const data = await apiJSON("/update/apply", "POST", {
          code: updateInputCode.value,
        });
        updateHelperLogPath.value = data.helper_log_path || "";
        ElMessage.success(t.value.updateStarted);
      } catch (e) {
        ElMessage.error(`${t.value.updateFailed}: ${e.message}`);
      } finally {
        updateApplying.value = false;
      }
    };

    onMounted(async () => {
      resetUpdateCode();
      updateHelperLogPath.value = "";
      await loadUpdateProxy();
      await checkForUpdate();
    });

    onDeactivated(() => {
      stopUpdateTimer();
    });

    return {
      t,
      updateInfo,
      updateChecking,
      updateApplying,
      updateGenerating,
      updateCode,
      updateCodeExpiry,
      updateInputCode,
      updateProxyDetected,
      updateProxyManual,
      updateProxySaving,
      updateProxyCollapse,
      updateHelperLogPath,
      checkForUpdate,
      generateUpdateCode,
      applyUpdate,
      loadUpdateProxy,
      saveUpdateProxy,
      clearUpdateProxy,
    };
  },
  template: `
    <div>
      <h2 style="margin: 0 0 12px 0">{{ t.updateDialogTitle }}</h2>

      <div v-if="updateHelperLogPath" style="margin-bottom: 12px">
        <el-alert type="info" :closable="false" :title="t.updateHelperLog" show-icon>
          <template #default>
            <div class="code" style="word-break: break-all">{{ updateHelperLogPath }}</div>
          </template>
        </el-alert>
      </div>

      <el-card shadow="never">
        <div style="display:grid;grid-template-columns:1fr 1fr;gap:10px">
          <div>
            <div class="muted" style="font-size: 12px">{{ t.currentVersion }}</div>
            <div class="code" style="margin-top: 4px">{{ updateInfo.current_version || '-' }}</div>
          </div>
          <div>
            <div class="muted" style="font-size: 12px">{{ t.latestVersion }}</div>
            <div class="code" style="margin-top: 4px">{{ updateInfo.latest_version || '-' }}</div>
          </div>
        </div>

        <div style="margin-top: 12px">
          <el-alert v-if="updateInfo.update_available === true" type="success" :closable="false" :title="t.updateAvailable" show-icon></el-alert>
          <el-alert v-else-if="updateInfo.latest_version" type="info" :closable="false" :title="t.upToDate" show-icon></el-alert>
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

        <div v-if="updateInfo.update_available === true" style="margin-top: 12px">
          <el-alert type="warning" :closable="false" :title="t.updateConfirm" show-icon></el-alert>

          <div v-if="updateCode" style="margin-top: 12px">
            <div style="display:flex;gap:10px;align-items:center">
              <div class="muted">
                {{ t.confirmationCode }}
                <span v-if="updateCodeExpiry" class="muted">({{ t.expiresIn }}: {{ updateCodeExpiry }})</span>
              </div>
              <span class="code" style="font-size: 22px">{{ updateCode }}</span>
            </div>

            <div style="margin-top:12px;display:flex;align-items:center;gap:10px;">
              <el-input
                v-model="updateInputCode"
                :placeholder="t.enterCode"
                maxlength="6"
                show-word-limit
                style="width: 240px"
                @keyup.enter="applyUpdate"
              ></el-input>
              <el-button
                v-if="updateCode && updateInputCode && updateInputCode === updateCode && updateCodeExpiry !== t.codeExpired"
                type="warning"
                :loading="updateApplying"
                @click="applyUpdate"
              >
                {{ t.applyUpdate }}
              </el-button>
            </div>
          </div>

          <div v-else style="margin-top: 12px">
            <el-button type="primary" :loading="updateGenerating" @click="generateUpdateCode">{{ t.generateCode }}</el-button>
          </div>
        </div>

        <div style="margin-top: 12px;display:flex;gap:10px;flex-wrap:wrap;">
          <el-button type="primary" :loading="updateChecking" @click="checkForUpdate">{{ t.checkUpdate }}</el-button>
        </div>
      </el-card>
    </div>
  `,
};
