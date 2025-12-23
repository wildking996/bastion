<template>
  <div class="pager">
    <el-pagination
      :current-page="page"
      :page-size="pageSize"
      :page-sizes="[10, 20, 50, 100]"
      layout="total, sizes, prev, pager, next, jumper"
      :total="total"
      @update:current-page="(v) => emit('update:page', v)"
      @update:page-size="(v) => emit('update:pageSize', v)"
    />
  </div>
</template>

<script setup lang="ts">
const props = defineProps<{ page: number; pageSize: number; total: number }>();
const emit = defineEmits<{
  (e: "update:page", value: number): void;
  (e: "update:pageSize", value: number): void;
}>();
</script>

<style scoped>
.pager {
  display: flex;
  justify-content: flex-end;
  padding: 12px 0;
}
</style>
