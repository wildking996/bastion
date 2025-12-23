import { apiJSON } from "../app/api.js";

const { ref, inject, computed, onMounted, onUnmounted } = Vue;

function useConfirmCodeDialog({ t, ElMessage }) {
  const visible = ref(false);
  const kind = ref(""); // 'update' | 'shutdown'

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
    generating.value = false;
    applying.value = false;
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

  const open = (nextKind) => {
    kind.value = nextKind;
    reset();
    visible.value = true;
  };

  const close = () => {
    visible.value = false;
    reset();
  };

  const title = computed(() => {
    if (kind.value === "update") return t.value.update;
    if (kind.value === "shutdown") return t.value.shutdown;
    return t.value.confirm;
  });

  const alertTitle = computed(() => {
    if (kind.value === "update") return t.value.updateConfirm;
    if (kind.value === "shutdown") return t.value.shutdownConfirm;
    return "";
  });

  const actionLabel = computed(() => {
    if (kind.value === "update") return t.value.applyUpdate;
    if (kind.value === "shutdown") return t.value.shutdownSystem;
    return t.value.confirm;
  });

  const actionType = computed(() => {
    if (kind.value === "update") return "warning";
    if (kind.value === "shutdown") return "danger";
    return "primary";
  });

  const generate = async ({ onGenerate }) => {
    generating.value = true;
    try {
      const data = await onGenerate();
      code.value = data.code || "";
      input.value = "";
      startCountdown(Number(data.expires_at || 0));
      ElMessage.success(t.value.codeGenerated);
    } catch (e) {
      ElMessage.error(e.message);
    } finally {
      generating.value = false;
    }
  };

  const apply = async ({ onApply }) => {
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

  onUnmounted(() => {
    stopTimer();
  });

  return {
    visible,
    kind,
    title,
    alertTitle,
    actionLabel,
    actionType,
    code,
    expiryText,
    input,
    generating,
    applying,
    canApply,
    open,
    close,
    generate,
    apply,
  };
}

export default {
  name: "ViewSystemActions",
  setup() {
    const t = inject("t");
    const ElMessage = ElementPlus.ElMessage;

    const checked = ref(false);

    const updateInfo = ref({});
    const updateChecking = ref(false);

    const updateProxyDetected = ref("");
    const updateProxyManual = ref("");
    const updateProxySaving = ref(false);
    const updateProxyCollapse = ref([]);

    const updateHelperLogPath = ref("");

    const confirmDlg = useConfirmCodeDialog({ t, ElMessage });

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
      confirmDlg.open("update");
    };

    const openShutdownConfirm = () => {
      confirmDlg.open("shutdown");
    };

    const onGenerateCode = async () => {
      if (confirmDlg.kind.value === "update") {
        return await apiJSON("/update/generate-code", "POST", {});
      }
      return await apiJSON("/shutdown/generate-code", "POST", {});
    };

    const onApplyAction = async ({ code }) => {
      if (confirmDlg.kind.value === "update") {
        const data = await apiJSON("/update/apply", "POST", { code });
        updateHelperLogPath.value = data.helper_log_path || "";
        ElMessage.success(t.value.updateStarted);
        confirmDlg.close();
        return;
      }

      await apiJSON("/shutdown/verify", "POST", { code });
      ElMessage.success(t.value.shutdownInitiated);
      confirmDlg.close();
    };

    onMounted(async () => {
      // do NOT auto check update (may hit GitHub API). Only load local proxy UI state.
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
      openShutdownConfirm,
      confirmDlg,
      onGenerateCode,
      onApplyAction,
    };
  },
  template: `
    <div style="max-width: 1100px; margin: 0 auto;">
      <el-row justify="space-between" align="middle" style="margin-bottom: 12px" :gutter="12">
        <el-col :span="24">
          <h2 style="margin: 0">{{ t.navUpdate }}</h2>
        </el-col>
      </el-row>

      <el-row :gutter="14" style="align-items: stretch;">
        <el-col :xs="24" :sm="24" :md="12">
          <el-card shadow="never" style="height: 100%">
            <template #header>
              <div style="display:flex;justify-content:space-between;align-items:center;gap:10px;flex-wrap:wrap;">
                <b>{{ t.update }}</b>
                <el-space wrap>
                  <el-button type="primary" :loading="updateChecking" @click="checkForUpdate">{{ t.checkUpdate }}</el-button>
                  <el-button type="warning" :disabled="!updateAvailable" @click="openUpdateConfirm">{{ t.applyUpdate }}</el-button>
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
        </el-col>

        <el-col :xs="24" :sm="24" :md="12">
          <el-card shadow="never" style="height: 100%">
            <template #header>
              <div style="display:flex;justify-content:space-between;align-items:center;gap:10px;flex-wrap:wrap;">
                <b>{{ t.shutdown }}</b>
                <el-button type="danger" @click="openShutdownConfirm">{{ t.shutdownSystem }}</el-button>
              </div>
            </template>

            <el-alert :title="t.shutdownConfirm" type="warning" :closable="false" show-icon></el-alert>
            <div class="muted" style="margin-top: 10px; font-size: 12px;">
              {{ t.shutdownInitiated }}
            </div>
          </el-card>
        </el-col>
      </el-row>

      <el-dialog v-model="confirmDlg.visible" :title="confirmDlg.title" width="520px" :close-on-click-modal="false" @close="confirmDlg.close">
        <el-alert type="warning" :closable="false" :title="confirmDlg.alertTitle" show-icon></el-alert>

        <div style="margin-top: 14px; display:flex; gap:10px; flex-wrap:wrap; align-items:center;">
          <el-button
            v-if="!confirmDlg.code"
            type="primary"
            :loading="confirmDlg.generating"
            @click="confirmDlg.generate({ onGenerate: onGenerateCode })"
          >
            {{ t.generateCode }}
          </el-button>

          <template v-else>
            <div class="muted">
              {{ t.confirmationCode }}
              <span v-if="confirmDlg.expiryText" class="muted">({{ t.expiresIn }}: {{ confirmDlg.expiryText }})</span>
            </div>
            <div class="code" style="font-size: 22px; font-weight: 700; color: var(--el-color-primary)">{{ confirmDlg.code }}</div>

            <el-input
              v-model="confirmDlg.input"
              maxlength="6"
              show-word-limit
              :placeholder="t.enterCode"
              style="width: 240px"
              @keyup.enter="confirmDlg.apply({ onApply: onApplyAction })"
            ></el-input>

            <el-button
              v-if="confirmDlg.canApply"
              :type="confirmDlg.actionType"
              :loading="confirmDlg.applying"
              @click="confirmDlg.apply({ onApply: onApplyAction })"
            >
              {{ confirmDlg.actionLabel }}
            </el-button>
          </template>
        </div>

        <template #footer>
          <el-button @click="confirmDlg.close">{{ t.close }}</el-button>
        </template>
      </el-dialog>
    </div>
  `,
};
