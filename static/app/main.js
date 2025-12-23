import {
  DEFAULT_ROUTE,
  ensureDefaultRoute,
  getCurrentRoute,
  navigate,
  onRouteChange,
} from "./router.js";
import { GROUPS, getViewByPath, getViewsByGroup } from "./view_registry.js";
import { loadSidebarCollapsed, saveSidebarCollapsed } from "./sidebar_state.js";
import { apiJSON } from "./api.js";
import { createConfirmCodeDialog } from "./confirm_code_dialog.js";

const {
  createApp,
  ref,
  shallowRef,
  computed,
  onMounted,
  onUnmounted,
  provide,
  nextTick,
} = Vue;

function titleForRoute(route, lang) {
  if (route.startsWith("/logs/http")) {
    return lang === "zh" ? i18nConfig.zh.titleLogs : i18nConfig.en.titleLogs;
  }
  if (route.startsWith("/logs/errors")) {
    return lang === "zh" ? i18nConfig.zh.titleErrors : i18nConfig.en.titleErrors;
  }
  return lang === "zh" ? i18nConfig.zh.titleIndex : i18nConfig.en.titleIndex;
}

const app = createApp({
  setup() {
    const currentLang = ref(i18n.getLanguage());
    const t = ref(i18n.getAll());

    const elLocale = computed(() => {
      if (currentLang.value === "zh") return window.ElementPlusLocaleZhCn;
      return window.ElementPlusLocaleEn;
    });

    const collapsed = ref(loadSidebarCollapsed());
    const asideWidth = computed(() => (collapsed.value ? "72px" : "260px"));

    const route = ref(DEFAULT_ROUTE);
    const activeComponent = shallowRef(null);
    const componentCache = new Map();

    const menuRef = ref(null);

    const activeView = computed(() => getViewByPath(route.value));
    const activeGroup = computed(() => {
      const view = activeView.value;
      if (!view) return null;
      return GROUPS.find((g) => g.key === view.group) || null;
    });

    const breadcrumbItems = computed(() => {
      const items = [{ label: t.value.console }];
      if (activeGroup.value) {
        items.push({ label: t.value[activeGroup.value.titleKey] });
      }
      if (activeView.value) {
        items.push({ label: t.value[activeView.value.titleKey] });
      }
      return items;
    });

    const groupIcons = {
      manage: "Tools",
      logs: "Document",
      system: "Setting",
    };

    const viewIcons = {
      "/bastions": "Connection",
      "/mappings": "Share",
      "/logs/http": "Document",
      "/logs/errors": "Warning",
      "/system/update": "Setting",
    };

    const refreshPage = () => window.location.reload();

    const switchLanguage = (lang) => {
      if (!lang || lang === currentLang.value) return;
      i18n.setLanguage(lang);
      currentLang.value = lang;
      t.value = i18n.getAll();
      document.title = titleForRoute(route.value, currentLang.value);
    };

    const toggleSidebar = () => {
      collapsed.value = !collapsed.value;
      saveSidebarCollapsed(collapsed.value);
    };

    const ElMessage = ElementPlus.ElMessage;
    const confirmDlg = createConfirmCodeDialog({ t, ElMessage });

    const confirmDlgVisible = confirmDlg.visible;
    const confirmDlgTitle = confirmDlg.title;
    const confirmDlgAlertTitle = confirmDlg.alertTitle;
    const confirmDlgActionType = confirmDlg.actionType;
    const confirmDlgCode = confirmDlg.code;
    const confirmDlgExpiryText = confirmDlg.expiryText;
    const confirmDlgInput = confirmDlg.input;
    const confirmDlgGenerating = confirmDlg.generating;
    const confirmDlgApplying = confirmDlg.applying;
    const confirmDlgCanApply = confirmDlg.canApply;

    const confirmDlgGenerate = confirmDlg.generate;
    const confirmDlgSubmit = confirmDlg.submit;
    const confirmDlgClose = confirmDlg.close;

    const openConfirmDialog = (opts) => confirmDlg.open(opts);

    const openShutdownConfirm = () => {
      openConfirmDialog({
        nextTitle: t.value.shutdown,
        nextAlertTitle: t.value.shutdownConfirm,
        nextActionType: "danger",
        nextOnGenerate: async () => await apiJSON("/shutdown/generate-code", "POST", {}),
        nextOnApply: async ({ code }) => {
          await apiJSON("/shutdown/verify", "POST", { code });
          ElMessage.success(t.value.shutdownInitiated);
        },
      });
    };


    let lastOpenedGroupKey = "";

    const syncOpenGroups = async ({ force = false } = {}) => {
      await nextTick();
      const menu = menuRef.value;
      if (!menu) return;

      const keep = activeGroup.value ? activeGroup.value.key : "";
      if (!force && keep === lastOpenedGroupKey) return;
      lastOpenedGroupKey = keep;

      for (const g of GROUPS) {
        if (g.key === keep) {
          if (typeof menu.open === "function") menu.open(g.key);
        } else {
          if (typeof menu.close === "function") menu.close(g.key);
        }
      }
    };

    const setRoute = async (nextRoute) => {
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

      confirmDlg.close();

      await syncOpenGroups();
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
    provide("openConfirmDialog", openConfirmDialog);

    let stopRouteListener = null;
    onMounted(async () => {
      ensureDefaultRoute();
      await setRoute(getCurrentRoute());
      stopRouteListener = onRouteChange(setRoute);
      document.title = titleForRoute(route.value, currentLang.value);
    });

    onUnmounted(() => {
      if (stopRouteListener) stopRouteListener();
    });

    return {
      elLocale,
      currentLang,
      t,
      collapsed,
      asideWidth,
      breadcrumbItems,
      groups: GROUPS,
      groupIcons,
      viewIcons,
      viewsByGroup,
      route,
      activeComponent,
      menuRef,
      onSelectMenu,
      refreshPage,
      switchLanguage,
      toggleSidebar,
      openShutdownConfirm,
      confirmDlgVisible,
      confirmDlgTitle,
      confirmDlgAlertTitle,
      confirmDlgActionType,
      confirmDlgCode,
      confirmDlgExpiryText,
      confirmDlgInput,
      confirmDlgGenerating,
      confirmDlgApplying,
      confirmDlgCanApply,
      confirmDlgGenerate,
      confirmDlgSubmit,
      confirmDlgClose,
    };
  },
  template: `
    <el-config-provider :locale="elLocale">
      <el-container class="app-shell">
        <el-header height="56px" style="border-bottom: 1px solid var(--el-border-color-lighter);">
          <el-row justify="space-between" align="middle" style="height: 100%;" :gutter="10">
            <el-col :span="12" style="min-width: 260px">
              <el-space :size="10" alignment="center">
                <el-button
                  :icon="collapsed ? 'Expand' : 'Fold'"
                  circle
                  @click="toggleSidebar"
                ></el-button>

                <el-space direction="vertical" :size="2">
                  <el-text size="large" tag="b">{{ t.console }}</el-text>
                  <el-breadcrumb separator="/">
                    <el-breadcrumb-item v-for="(b, i) in breadcrumbItems" :key="i">{{ b.label }}</el-breadcrumb-item>
                  </el-breadcrumb>
                </el-space>
              </el-space>
            </el-col>

            <el-col :span="12" style="display:flex;justify-content:flex-end">
              <el-space :size="10" alignment="center">
                <el-dropdown @command="switchLanguage">
                  <el-button circle aria-label="language">
                    <el-icon><ChatLineRound /></el-icon>
                  </el-button>
                  <template #dropdown>
                    <el-dropdown-menu>
                      <el-dropdown-item command="zh" :disabled="currentLang === 'zh'">中文</el-dropdown-item>
                      <el-dropdown-item command="en" :disabled="currentLang === 'en'">English</el-dropdown-item>
                    </el-dropdown-menu>
                  </template>
                </el-dropdown>

                <el-divider direction="vertical" />

                <el-tooltip :content="t.refresh" placement="bottom">
                  <template #reference>
                    <el-button circle @click="refreshPage" aria-label="refresh">
                      <el-icon><Refresh /></el-icon>
                    </el-button>
                  </template>
                </el-tooltip>

                <el-tooltip :content="t.shutdown" placement="bottom">
                  <template #reference>
                    <el-button circle type="danger" @click="openShutdownConfirm" aria-label="shutdown">
                      <el-icon><SwitchButton /></el-icon>
                    </el-button>
                  </template>
                </el-tooltip>
              </el-space>
            </el-col>
          </el-row>
        </el-header>

        <el-container class="app-body">
          <el-aside :width="asideWidth" class="app-aside">
            <el-scrollbar class="aside-scroll">
            <el-menu
              ref="menuRef"
              :default-active="route"
              :collapse="collapsed"
              unique-opened
              class="aside-menu"
              @select="onSelectMenu"
            >
              <el-sub-menu v-for="g in groups" :key="g.key" :index="g.key">
                <template #title>
                  <el-icon>
                    <component :is="groupIcons[g.key]" />
                  </el-icon>
                  <span>{{ t[g.titleKey] }}</span>
                </template>
                <el-menu-item
                  v-for="v in viewsByGroup[g.key]"
                  :key="v.path"
                  :index="v.path"
                >
                  <el-icon v-if="viewIcons[v.path]">
                    <component :is="viewIcons[v.path]" />
                  </el-icon>
                  <span>{{ t[v.titleKey] }}</span>
                </el-menu-item>
              </el-sub-menu>
            </el-menu>
            </el-scrollbar>
          </el-aside>

          <el-main class="app-main">
            <el-scrollbar class="main-scroll">
              <div class="main-inner">
                <keep-alive>
                  <component :is="activeComponent" />
                </keep-alive>
              </div>
            </el-scrollbar>
          </el-main>
        </el-container>
      </el-container>

      <el-dialog
        v-model="confirmDlgVisible"
        :title="confirmDlgTitle"
        width="520px"
        :close-on-click-modal="false"
        @close="confirmDlgClose"
      >
        <el-alert
          type="warning"
          :closable="false"
          :title="confirmDlgAlertTitle"
          show-icon
        ></el-alert>

        <div style="margin-top: 14px; display:flex; gap:10px; flex-wrap:wrap; align-items:center;">
          <el-button type="primary" :loading="confirmDlgGenerating" @click="confirmDlgGenerate">
            {{ t.generateCode }}
          </el-button>

          <template v-if="confirmDlgCode">
            <div class="muted">
              {{ t.confirmationCode }}
              <span v-if="confirmDlgExpiryText" class="muted">({{ t.expiresIn }}: {{ confirmDlgExpiryText }})</span>
            </div>
            <div class="code" style="font-size: 22px; font-weight: 700; color: var(--el-color-primary)">{{ confirmDlgCode }}</div>
          </template>

          <el-input
            v-model="confirmDlgInput"
            maxlength="6"
            :placeholder="t.enterCode"
            style="width: 240px"
            @keyup.enter="confirmDlgSubmit"
          ></el-input>

          <el-button
            :type="confirmDlgActionType"
            :loading="confirmDlgApplying"
            :disabled="!confirmDlgCanApply"
            @click="confirmDlgSubmit"
          >
            {{ t.submitCode }}
          </el-button>
        </div>

        <template #footer>
          <el-button @click="confirmDlgClose">{{ t.close }}</el-button>
        </template>
      </el-dialog>
    </el-config-provider>
  `,
});

for (const [key, component] of Object.entries(ElementPlusIconsVue)) {
  app.component(key, component);
}

app.use(ElementPlus).mount("#app");
