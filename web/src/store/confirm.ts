import { defineStore } from "pinia";

export type ConfirmDialogConfig = {
  title: string;
  alert: string;
  generate: () => Promise<{ code: string; expiresAt: number }>;
  submit: (code: string) => Promise<void>;
  successToast?: string;
};

type State = {
  visible: boolean;
  config: ConfirmDialogConfig | null;
  code: string;
  expiresAt: number | null;
  input: string;
  generating: boolean;
  submitting: boolean;
};

export const useConfirmDialogStore = defineStore("confirm-dialog", {
  state: (): State => ({
    visible: false,
    config: null,
    code: "",
    expiresAt: null,
    input: "",
    generating: false,
    submitting: false,
  }),
  actions: {
    open(config: ConfirmDialogConfig) {
      this.visible = true;
      this.config = config;
      this.code = "";
      this.expiresAt = null;
      this.input = "";
      this.generating = false;
      this.submitting = false;
    },
    close() {
      this.visible = false;
      this.config = null;
      this.code = "";
      this.expiresAt = null;
      this.input = "";
      this.generating = false;
      this.submitting = false;
    },
    setInput(value: string) {
      this.input = value;
    },
    async generate() {
      if (!this.config || this.generating) return;
      this.generating = true;
      try {
        const { code, expiresAt } = await this.config.generate();
        this.code = code;
        this.expiresAt = expiresAt;
        this.input = "";
      } finally {
        this.generating = false;
      }
    },
    async submit() {
      if (!this.config || this.submitting) return;
      this.submitting = true;
      try {
        await this.config.submit(this.input);
      } finally {
        this.submitting = false;
      }
    },
  },
});
