import type { FormItemRule } from "element-plus";

type Translator = (key: string, params?: Record<string, any>) => string;

export function requiredTrimRule(
  t: Translator,
  fieldLabel: string,
  trigger: FormItemRule["trigger"] = "blur"
): FormItemRule {
  return {
    required: true,
    trigger,
    message: t("validation.required", { field: fieldLabel }),
    transform: (v: unknown) => (typeof v === "string" ? v.trim() : v),
  };
}

export function requiredNumberRule(
  t: Translator,
  fieldLabel: string,
  trigger: FormItemRule["trigger"] = "change"
): FormItemRule {
  return {
    required: true,
    type: "number",
    trigger,
    message: t("validation.required", { field: fieldLabel }),
  };
}
