import { apiJSON } from "./api.js";

const { ref, computed, onMounted } = Vue;

export function useUpdatePanel({ t, openConfirmDialog, ElMessage }) {
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

  const confirmUpdate = () => {
    if (!checked.value) {
      ElMessage.warning(t.value.updateNotChecked);
      return;
    }
    if (!updateAvailable.value) {
      ElMessage.info(t.value.upToDate);
      return;
    }
    if (typeof openConfirmDialog !== "function") {
      ElMessage.error(t.value.confirmDialogNotReady || "确认弹窗未就绪");
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
    // do NOT auto-check update (may hit GitHub API). Only load local proxy UI state.
    await loadUpdateProxy();
  });

  return {
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
    confirmUpdate,
  };
}
