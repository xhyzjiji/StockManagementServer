// 全局 Promise 式弹窗的状态：选项面板（pickOption）与单行输入框（promptText），由 GlobalDialogs.vue 渲染。
// State for global promise-style dialogs, an option sheet (pickOption) and a single-line prompt (promptText),
// rendered by GlobalDialogs.vue.
import { reactive } from 'vue';

export interface OptionSheetState {
  show: boolean;
  title: string;
  description: string;
  options: string[];
  cancelText: string;
  resolve: ((index: number) => void) | null;
}

export const optionSheet = reactive<OptionSheetState>({
  show: false, title: '', description: '', options: [], cancelText: '取消', resolve: null,
});

// pickOption 打开底部选项面板，返回所选下标；取消返回 -1。
// pickOption opens a bottom option sheet and resolves to the chosen index, or -1 when cancelled.
export function pickOption(title: string, options: string[], description = '', cancelText = '取消'): Promise<number> {
  optionSheet.resolve?.(-1);
  return new Promise((resolve) => {
    Object.assign(optionSheet, { show: true, title, description, options, cancelText, resolve });
  });
}

export interface PromptOptions {
  title: string;
  label: string;
  value?: string;
  placeholder?: string;
  maxlength?: number;
  inputmode?: 'text' | 'decimal';
  // validate 返回错误文本时阻止提交
  // validate returning an error message blocks submission.
  validate?: (v: string) => string | null;
  // onOk 抛错时弹窗保持打开并提示错误
  // When onOk throws, the dialog stays open and shows the error.
  onOk?: (v: string) => Promise<void> | void;
}

export interface PromptState extends Required<Omit<PromptOptions, 'validate' | 'onOk'>> {
  show: boolean;
  error: string;
  validate: PromptOptions['validate'];
  onOk: PromptOptions['onOk'];
  resolve: ((v: string | null) => void) | null;
}

export const prompt = reactive<PromptState>({
  show: false, title: '', label: '', value: '', placeholder: '', maxlength: 128, inputmode: 'text',
  error: '', validate: undefined, onOk: undefined, resolve: null,
});

// promptText 打开单行输入弹窗，确认后返回输入值（已去首尾空格）；取消返回 null。
// promptText opens a single-line input dialog and resolves to the trimmed value, or null when cancelled.
export function promptText(opts: PromptOptions): Promise<string | null> {
  prompt.resolve?.(null);
  return new Promise((resolve) => {
    Object.assign(prompt, {
      value: '', placeholder: '', maxlength: 128, inputmode: 'text', validate: undefined, onOk: undefined,
      ...opts, show: true, error: '', resolve,
    });
  });
}
