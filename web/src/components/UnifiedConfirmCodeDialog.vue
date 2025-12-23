<template>
  <el-dialog
    v-model="confirm.visible"
    :title="confirm.config?.title"
    width="520px"
    :close-on-click-modal="false"
    @closed="onClosed"
  >
    <el-alert
      v-if="confirm.config?.alert"
      :title="confirm.config.alert"
      type="warning"
      :closable="false"
      show-icon
      class="mb"
    />

    <el-form label-width="110px" label-position="left">
      <el-form-item :label="t('confirmDialog.code')">
        <div class="code-line">
          <span class="mono code">{{ confirm.code || '-' }}</span>
          <el-button size="small" @click="confirm.generate" :loading="confirm.generating">
            {{ t('confirmDialog.generate') }}
          </el-button>
        </div>
      </el-form-item>

      <el-form-item :label="t('confirmDialog.countdown')">
        <span class="mono">{{ countdownText }}</span>
      </el-form-item>

      <el-form-item :label="t('confirmDialog.input')">
        <el-input
          v-model="confirm.input"
          maxlength="6"
          show-word-limit
          inputmode="numeric"
          autocomplete="one-time-code"
          class="mono"
        />
      </el-form-item>

      <el-alert v-if="expired" :title="t('confirmDialog.expired')" type="error" :closable="false" show-icon />
    </el-form>

    <template #footer>
      <el-button @click="confirm.close">{{ t('common.cancel') }}</el-button>
      <el-button
        type="primary"
        @click="submit"
        :disabled="submitDisabled"
        :loading="confirm.submitting"
      >
        {{ t('confirmDialog.submit') }}
      </el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ElMessage } from "element-plus";
import { computed, onBeforeUnmount, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { useRouter } from "vue-router";

import { useConfirmDialogStore } from "@/store/confirm";

const confirm = useConfirmDialogStore();
const { t } = useI18n();
const router = useRouter();

const nowMs = ref(Date.now());
let timer: number | null = null;

function startTimer() {
  if (timer != null) return;
  timer = window.setInterval(() => {
    nowMs.value = Date.now();
  }, 250);
}

function stopTimer() {
  if (timer == null) return;
  window.clearInterval(timer);
  timer = null;
}

const remainingSec = computed(() => {
  if (!confirm.expiresAt) return 0;
  const ms = confirm.expiresAt * 1000 - nowMs.value;
  return Math.max(0, Math.floor(ms / 1000));
});

const expired = computed(() => confirm.expiresAt != null && remainingSec.value <= 0);

const countdownText = computed(() => {
  if (!confirm.expiresAt) return "--:--";
  const s = remainingSec.value;
  const mm = String(Math.floor(s / 60)).padStart(2, "0");
  const ss = String(s % 60).padStart(2, "0");
  return `${mm}:${ss}`;
});

const submitDisabled = computed(() => {
  if (!confirm.code) return true;
  if (expired.value) return true;
  if (confirm.input !== confirm.code) return true;
  return false;
});

async function submit() {
  try {
    await confirm.submit();
    if (confirm.config?.successToast) ElMessage.success(confirm.config.successToast);
    confirm.close();
  } catch {
    // api interceptor shows message
  }
}

function onClosed() {
  stopTimer();
}

watch(
  () => confirm.visible,
  (v) => {
    if (v) startTimer();
    else stopTimer();
  },
  { immediate: true }
);

watch(
  () => router.currentRoute.value.fullPath,
  () => {
    if (confirm.visible) confirm.close();
  }
);

onBeforeUnmount(() => stopTimer());
</script>

<style scoped>
.mb {
  margin-bottom: 12px;
}

.code-line {
  width: 100%;
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 10px;
}

.code {
  font-size: 18px;
  letter-spacing: 0.15em;
}
</style>
