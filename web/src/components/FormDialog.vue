<template>
  <el-dialog
    :model-value="modelValue"
    :title="title"
    :width="width"
    :close-on-click-modal="false"
    class="form-dialog"
    @update:model-value="(v) => emit('update:modelValue', v)"
    @closed="() => emit('closed')"
  >
    <div class="form-dialog__body" :style="bodyStyle">
      <slot />
    </div>

    <template #footer>
      <div class="form-dialog__footer">
        <slot name="footer" />
      </div>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { computed } from "vue";

const props = withDefaults(
  defineProps<{
    modelValue: boolean;
    title: string;
    width?: string | number;
    contentMaxWidth?: string;
  }>(),
  {
    width: "480px",
    contentMaxWidth: "440px",
  }
);

const emit = defineEmits<{
  (e: "update:modelValue", value: boolean): void;
  (e: "closed"): void;
}>();

const bodyStyle = computed(() => ({
  "--fd-max": props.contentMaxWidth,
}));
</script>

<style scoped>
.form-dialog :deep(.el-dialog__header) {
  padding: 16px 20px 10px;
}

.form-dialog :deep(.el-dialog__body) {
  padding: 0 20px 14px;
}

.form-dialog :deep(.el-dialog__footer) {
  padding: 10px 20px 16px;
}

.form-dialog__body {
  max-width: var(--fd-max);
  margin: 0 auto;
}

.form-dialog__footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}
</style>
