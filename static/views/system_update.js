import { apiJSON } from "../app/api.js";

const { ref, inject, computed, onMounted, onDeactivated } = Vue;

function useConfirmCodeFlow({ t, ElMessage, onGenerate, onApply }) {
  const code = ref("");
  const expiresAt = ref(0);
  const expiryText = ref("");
  const input = ref("");
  const generating = ref(false);
  const applying = ref(false);
  let timer = null;

  const stopTimer = () => {
    if (!timer) return;
    clearInterval(timer);
    timer = null;
  };

  const reset = () => {
    code.value = "";
    expiresAt.value = 0;
    expiryText.value = "";
    input.value = "";
    stopTimer();
  };

  const updateCountdown = () => {
    const now = Math.floor(Date.now() / 1000);
    const left = Math.max(0, expiresAt.value - now);
    if (left <= 0) {
      expiryText.value = t.value.codeExpired;
      stopTimer();
      return;
    }
    const minutes = String(Math.floor(left / 60)).padStart(2, "0");
    const seconds = String(left % 60).padStart(2, "0");
    expiryText.value = `${minutes}:${seconds}`;
  };

  const startCountdown = (unix) => {
    expiresAt.value = Number(unix || 0);
    updateCountdown();
    stopTimer();
    timer = setInterval(updateCountdown, 1000);
  };

  const canApply = computed(() => {
    return (
      code.value &&
      input.value &&
      input.value === code.value &&
      expiryText.value !== t.value.codeExpired
    );
  });

  const generate = async () => {
    generating.value = true;
    try {
      const data = await onGenerate();
      code.value = data.code || "";
      input.value = "";
      startCountdown(Number(data.expires_at || 0));
    } catch (e) {
      ElMessage.error(e.message);
    } finally {
      generating.value = false;
    }
  };

  const apply = async () => {
    if (!canApply.value) return;
    applying.value = true;
    try {
      await onApply({ code: input.value });
    } catch (e) {
      ElMessage.error(e.message);
    } finally {
      applying.value = false;
    }
  };

  onDeactivated(() => {
    stopTimer();
  });

  return {
    code,
    expiryText,
    input,
    generating,
    applying,
    canApply,
    reset,
    generate,
    apply,
  };
}

export default {
  name: "ViewSystemActions",
  setup() {
    const t = inject("t");
    const ElMessage = ElementPlus.ElMessage;

    // Update
    const updateInfo = ref({});
    const updateChecking = ref(false);

    const updateProxyDetected = ref("");
    const updateProxyManual = ref("");
    const updateProxySaving = ref(false);
    const updateProxyCollapse = ref([]);

    const updateHelperLogPath = ref("");

    const checkForUpdate = async () => {
      updateChecking.value = true;
      try {
        const data = await apiJSON("/update/check", "GET");
        updateInfo.value = data || {};
        if (updateInfo.value.update_available !== true) {
          updateCodeFlow.reset();
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

    const updateCodeFlow = useConfirmCodeFlow({
      t,
      ElMessage,
      onGenerate: async () => {
        const data = await apiJSON("/update/generate-code", "POST", {});
        ElMessage.success(t.value.codeGenerated);
        return data;
      },
      onApply: async ({ code }) => {
        const data = await apiJSON("/update/apply", "POST", { code });
        updateHelperLogPath.value = data.helper_log_path || "";
        ElMessage.success(t.value.updateStarted);
      },
    });

    // Shutdown
    const shutdownCodeFlow = useConfirmCodeFlow({
      t,
      ElMessage,
      onGenerate: async () => {
        const data = await apiJSON("/shutdown/generate-code", "POST", {});
        ElMessage.success(t.value.codeGenerated);
        return data;
      },
      onApply: async ({ code }) => {
        await apiJSON("/shutdown/verify", "POST", { code });
        ElMessage.success(t.value.shutdownInitiated);
      },
    });

    onMounted(async () => {
      updateHelperLogPath.value = "";
      updateCodeFlow.reset();
      shutdownCodeFlow.reset();
      await loadUpdateProxy();
      await checkForUpdate();
    });

    return {
      t,
      updateInfo,
      updateChecking,
      updateProxyDetected,
      updateProxyManual,
      updateProxySaving,
      updateProxyCollapse,
      updateHelperLogPath,
      checkForUpdate,
      saveUpdateProxy,
      clearUpdateProxy,
      updateCodeFlow,
      shutdownCodeFlow,
    };
  },
  template: `
    <div style="max-width: 1100px; margin: 0 auto;">
      <el-row justify="space-between" align="middle" style="margin-bottom: 12px" :gutter="12">
        <el-col :span="24">
          <h2 style="margin: 0">{{ t.navUpdate }}</h2>
        </el-col>
      </el-row>

      <el-row :gutter="14">
        <el-col :xs="24" :sm="24" :md="16">
          <el-card shadow="never">
            <template #header>
              <div style="display:flex;justify-content:space-between;align-items:center;gap:10px;flex-wrap:wrap;">
                <b>{{ t.update }}</b>
                <el-button type="primary" :loading="updateChecking" @click="checkForUpdate">{{ t.checkUpdate }}</el-button>
              </div>
            </template>

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

              <div style="margin-top: 12px; display:flex; gap:10px; flex-wrap:wrap; align-items:center;">
                <el-button
                  v-if="!updateCodeFlow.code"
                  type="primary"
                  :loading="updateCodeFlow.generating"
                  @click="updateCodeFlow.generate"
                >
                  {{ t.generateCode }}
                </el-button>

                <template v-else>
                  <div class="muted">
                    {{ t.confirmationCode }}
                    <span v-if="updateCodeFlow.expiryText" class="muted">({{ t.expiresIn }}: {{ updateCodeFlow.expiryText }})</span>
                  </div>
                  <div class="code" style="font-size: 22px; font-weight: 700; color: var(--el-color-primary)">{{ updateCodeFlow.code }}</div>

                  <el-input
                    v-model="updateCodeFlow.input"
                    maxlength="6"
                    show-word-limit
                    :placeholder="t.enterCode"
                    style="width: 240px"
                    @keyup.enter="updateCodeFlow.apply"
                  ></el-input>

                  <el-button
                    v-if="updateCodeFlow.canApply"
                    type="warning"
                    :loading="updateCodeFlow.applying"
                    @click="updateCodeFlow.apply"
                  >
                    {{ t.applyUpdate }}
                  </el-button>
                </template>
              </div>
            </div>
          </el-card>
        </el-col>

        <el-col :xs="24" :sm="24" :md="8">
          <el-card shadow="never">
            <template #header>
              <b>{{ t.shutdown }}</b>
            </template>

            <el-alert :title="t.shutdownConfirm" type="warning" :closable="false" show-icon></el-alert>

            <div style="margin-top: 12px; display:flex; gap:10px; flex-wrap:wrap; align-items:center;">
              <el-button
                v-if="!shutdownCodeFlow.code"
                type="primary"
                :loading="shutdownCodeFlow.generating"
                @click="shutdownCodeFlow.generate"
              >
                {{ t.generateCode }}
              </el-button>

              <template v-else>
                <div class="muted">
                  {{ t.confirmationCode }}
                  <span v-if="shutdownCodeFlow.expiryText" class="muted">({{ t.expiresIn }}: {{ shutdownCodeFlow.expiryText }})</span>
                </div>
                <div class="code" style="font-size: 22px; font-weight: 700; color: var(--el-color-primary)">{{ shutdownCodeFlow.code }}</div>

                <el-input
                  v-model="shutdownCodeFlow.input"
                  maxlength="6"
                  show-word-limit
                  :placeholder="t.enterCode"
                  style="width: 240px"
                  @keyup.enter="shutdownCodeFlow.apply"
                ></el-input>

                <el-button
                  v-if="shutdownCodeFlow.canApply"
                  type="danger"
                  :loading="shutdownCodeFlow.applying"
                  @click="shutdownCodeFlow.apply"
                >
                  {{ t.shutdownSystem }}
                </el-button>
              </template>
            </div>
          </el-card>
        </el-col>
      </el-row>
    </div>
  `,
};
