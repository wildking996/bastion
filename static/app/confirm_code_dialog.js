const { ref, computed, onUnmounted } = Vue;

export function createConfirmCodeDialog({ t, ElMessage }) {
  const visible = ref(false);

  const title = ref("");
  const alertTitle = ref("");
  const actionType = ref("primary");

  const code = ref("");
  const expiresAt = ref(0);
  const expiryText = ref("");
  const input = ref("");

  const generating = ref(false);
  const applying = ref(false);

  let onGenerate = async () => ({ code: "", expires_at: 0 });
  let onApply = async () => {};
  let timer = null;

  const stopTimer = () => {
    if (!timer) return;
    clearInterval(timer);
    timer = null;
  };

  const reset = () => {
    title.value = "";
    alertTitle.value = "";
    actionType.value = "primary";

    code.value = "";
    expiresAt.value = 0;
    expiryText.value = "";
    input.value = "";

    generating.value = false;
    applying.value = false;

    onGenerate = async () => ({ code: "", expires_at: 0 });
    onApply = async () => {};

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

  const open = ({
    nextTitle,
    nextAlertTitle,
    nextActionType,
    nextOnGenerate,
    nextOnApply,
  }) => {
    reset();
    title.value = nextTitle || t.value.confirm;
    alertTitle.value = nextAlertTitle || "";
    actionType.value = nextActionType || "primary";
    onGenerate = nextOnGenerate || onGenerate;
    onApply = nextOnApply || onApply;
    visible.value = true;
  };

  const close = () => {
    visible.value = false;
    reset();
  };

  const generate = async () => {
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

  const submit = async () => {
    if (!canApply.value) return;
    applying.value = true;
    try {
      await onApply({ code: input.value });
      close();
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
    title,
    alertTitle,
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
    submit,
  };
}
