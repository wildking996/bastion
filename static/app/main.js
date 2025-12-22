import { apiJSON } from "./api.js";
import {
  DEFAULT_ROUTE,
  ensureDefaultRoute,
  getCurrentRoute,
  navigate,
  onRouteChange,
} from "./router.js";
import { GROUPS, getViewByPath, getViewsByGroup } from "./view_registry.js";
import { loadGroupState, saveGroupState } from "./sidebar_state.js";

const { createApp, ref, shallowRef, computed, onMounted, onUnmounted, provide } =
  Vue;

function titleForRoute(route, lang) {
  if (route.startsWith("/logs/http")) {
    return lang === "zh" ? i18nConfig.zh.titleLogs : i18nConfig.en.titleLogs;
  }
  if (route.startsWith("/logs/errors")) {
    return lang === "zh" ? i18nConfig.zh.titleErrors : i18nConfig.en.titleErrors;
  }
  return lang === "zh" ? i18nConfig.zh.titleIndex : i18nConfig.en.titleIndex;
}

function makeGroupDefaults() {
  const defaults = {};
  for (const group of GROUPS) defaults[group.key] = true;
  return defaults;
}

const app = createApp({
  setup() {
    const ElMessage = ElementPlus.ElMessage;

    const currentLang = ref(i18n.getLanguage());
    const t = ref(i18n.getAll());

    const elLocale = computed(() => {
      if (currentLang.value === "zh") return window.ElementPlusLocaleZhCn;
      return window.ElementPlusLocaleEn;
    });

    const groupState = ref(loadGroupState(makeGroupDefaults()));
    const defaultOpenGroups = computed(() =>
      Object.entries(groupState.value)
        .filter(([, opened]) => opened)
        .map(([key]) => key)
    );

    const route = ref(DEFAULT_ROUTE);
    const activeComponent = shallowRef(null);
    const componentCache = new Map();

    const shutdownDialogVisible = ref(false);
    const shutdownCode = ref("");
    const shutdownExpiresAt = ref(0);
    const shutdownCodeExpiry = ref("");
    const inputCode = ref("");
    const generating = ref(false);
    const shuttingDown = ref(false);
    let shutdownExpiryTimer = null;

    const stopShutdownTimer = () => {
      if (!shutdownExpiryTimer) return;
      clearInterval(shutdownExpiryTimer);
      shutdownExpiryTimer = null;
    };

    const updateShutdownCountdown = () => {
      const now = Math.floor(Date.now() / 1000);
      const left = Math.max(0, shutdownExpiresAt.value - now);
      if (left <= 0) {
        shutdownCodeExpiry.value = t.value.codeExpired;
        stopShutdownTimer();
        return;
      }
      const minutes = String(Math.floor(left / 60)).padStart(2, "0");
      const seconds = String(left % 60).padStart(2, "0");
      shutdownCodeExpiry.value = `${minutes}:${seconds}`;
    };

    const showShutdownDialog = () => {
      shutdownDialogVisible.value = true;
      shutdownCode.value = "";
      shutdownExpiresAt.value = 0;
      shutdownCodeExpiry.value = "";
      inputCode.value = "";
      stopShutdownTimer();
    };

    const generateShutdownCode = async () => {
      generating.value = true;
      try {
        const data = await apiJSON("/shutdown/generate-code", "POST");
        shutdownCode.value = data.code || "";
        shutdownExpiresAt.value = Number(data.expires_at || 0);
        inputCode.value = "";

        updateShutdownCountdown();
        stopShutdownTimer();
        shutdownExpiryTimer = setInterval(updateShutdownCountdown, 1000);

        ElMessage.success(t.value.codeGenerated);
      } catch (e) {
        ElMessage.error(e.message);
      } finally {
        generating.value = false;
      }
    };

    const verifyAndShutdown = async () => {
      if ((inputCode.value || "").length !== 6) {
        ElMessage.warning(t.value.enterCode);
        return;
      }
      shuttingDown.value = true;
      try {
        await apiJSON("/shutdown/verify", "POST", { code: inputCode.value });
        ElMessage.success(t.value.shutdownInitiated);
        shutdownDialogVisible.value = false;
        stopShutdownTimer();
      } catch (e) {
        ElMessage.error(e.message);
      } finally {
        shuttingDown.value = false;
      }
    };

    const refreshPage = () => window.location.reload();

    const switchLanguage = (lang) => {
      i18n.setLanguage(lang);
      currentLang.value = lang;
      t.value = i18n.getAll();
      document.title = titleForRoute(route.value, currentLang.value);
    };

    const onOpenGroup = (key) => {
      groupState.value[key] = true;
      saveGroupState(groupState.value);
    };

    const onCloseGroup = (key) => {
      groupState.value[key] = false;
      saveGroupState(groupState.value);
    };

    const setRoute = (nextRoute) => {
      const view = getViewByPath(nextRoute);
      if (!view) {
        navigate(DEFAULT_ROUTE);
        return;
      }

      route.value = view.path;
      if (!componentCache.has(view.path)) {
        componentCache.set(view.path, Vue.defineAsyncComponent(view.loader));
      }
      activeComponent.value = componentCache.get(view.path);
      document.title = titleForRoute(route.value, currentLang.value);
    };

    const onSelectMenu = (index) => {
      if (!index || typeof index !== "string") return;
      if (index.startsWith("/")) navigate(index);
    };

    const viewsByGroup = computed(() => {
      const grouped = {};
      for (const group of GROUPS) grouped[group.key] = getViewsByGroup(group.key);
      return grouped;
    });

    provide("t", t);
    provide("currentLang", currentLang);
    provide("navigate", navigate);

    let stopRouteListener = null;
    onMounted(() => {
      ensureDefaultRoute();
      setRoute(getCurrentRoute());
      stopRouteListener = onRouteChange(setRoute);
    });

    onUnmounted(() => {
      if (stopRouteListener) stopRouteListener();
      stopShutdownTimer();
    });

    return {
      elLocale,
      currentLang,
      t,
      groups: GROUPS,
      viewsByGroup,
      defaultOpenGroups,
      route,
      activeComponent,
      onSelectMenu,
      onOpenGroup,
      onCloseGroup,
      refreshPage,
      switchLanguage,
      shutdownDialogVisible,
      shutdownCode,
      shutdownCodeExpiry,
      inputCode,
      generating,
      shuttingDown,
      showShutdownDialog,
      generateShutdownCode,
      verifyAndShutdown,
    };
  },
  template: `
    <el-config-provider :locale="elLocale">
      <div class="wrap">
        <div class="topbar">
          <div class="topbar-left">
            <h2 style="margin: 0">{{ t.console }}</h2>
          </div>
          <div class="topbar-right">
            <el-button @click="showShutdownDialog" icon="SwitchButton" type="danger">{{ t.shutdown }}</el-button>
            <div class="lang-toggle">
              <button class="lang-btn" :class="{ active: currentLang === 'zh' }" @click="switchLanguage('zh')">中文</button>
              <button class="lang-btn" :class="{ active: currentLang === 'en' }" @click="switchLanguage('en')">English</button>
            </div>
            <el-button @click="refreshPage" icon="Refresh" circle></el-button>
          </div>
        </div>

        <div class="layout">
          <aside class="sidebar">
            <el-menu
              :default-active="route"
              :default-openeds="defaultOpenGroups"
              class="sidebar-menu"
              background-color="#0f1115"
              text-color="#cfd2d6"
              active-text-color="#409eff"
              @select="onSelectMenu"
              @open="onOpenGroup"
              @close="onCloseGroup"
            >
              <el-sub-menu v-for="g in groups" :key="g.key" :index="g.key">
                <template #title>
                  <span>{{ t[g.titleKey] }}</span>
                </template>
                <el-menu-item
                  v-for="v in viewsByGroup[g.key]"
                  :key="v.path"
                  :index="v.path"
                >
                  {{ t[v.titleKey] }}
                </el-menu-item>
              </el-sub-menu>
            </el-menu>
          </aside>

          <main class="main">
            <keep-alive>
              <component :is="activeComponent" />
            </keep-alive>
          </main>
        </div>
      </div>

      <el-dialog v-model="shutdownDialogVisible" :title="t.shutdown" width="500px">
        <div style="margin-bottom: 20px">
          <el-alert :title="t.shutdownConfirm" type="warning" :closable="false"></el-alert>
        </div>

        <div v-if="!shutdownCode" style="text-align: center">
          <el-button type="primary" @click="generateShutdownCode" :loading="generating">
            {{ t.generateCode }}
          </el-button>
        </div>

        <div v-else>
          <el-descriptions :column="1" border>
            <el-descriptions-item :label="t.confirmationCode">
              <span style="font-size: 24px; font-weight: bold; color: #409eff; font-family: monospace;">{{ shutdownCode }}</span>
            </el-descriptions-item>
            <el-descriptions-item :label="t.expiresIn">
              <el-tag type="info">{{ shutdownCodeExpiry }}</el-tag>
            </el-descriptions-item>
          </el-descriptions>

          <el-form style="margin-top: 20px" @submit.prevent="verifyAndShutdown">
            <el-form-item :label="t.enterCode">
              <el-input v-model="inputCode" maxlength="6" :placeholder="t.enterCode" clearable></el-input>
            </el-form-item>
            <el-form-item>
              <el-button
                type="danger"
                @click="verifyAndShutdown"
                :loading="shuttingDown"
                :disabled="(inputCode || '').length !== 6"
              >
                {{ t.shutdownSystem }}
              </el-button>
              <el-button @click="shutdownDialogVisible = false">{{ t.cancel }}</el-button>
            </el-form-item>
          </el-form>
        </div>
      </el-dialog>
    </el-config-provider>
  `,
});

for (const [key, component] of Object.entries(ElementPlusIconsVue)) {
  app.component(key, component);
}

app.use(ElementPlus).mount("#app");
