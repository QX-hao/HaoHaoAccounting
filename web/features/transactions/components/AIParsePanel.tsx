'use client';

import { formatMoney } from '@/lib/format';

type Props = {
  aiText: string;
  amount: string;
  onAITextChange: (value: string) => void;
  onParse: () => void;
};

export function AIParsePanel({ aiText, amount, onAITextChange, onParse }: Props) {
  return (
    <div className="card grid">
      <div>
        <span className="eyebrow">AI Assistant</span>
        <h3>对话记账</h3>
      </div>
      <textarea rows={7} value={aiText} onChange={(e) => onAITextChange(e.target.value)} placeholder="例如：今天午饭35" />
      <button className="secondary" type="button" onClick={onParse}>
        AI 解析
      </button>
      <div className="panel">
        <span className="eyebrow">Preview</span>
        <h3>{amount ? formatMoney(Number(amount)) : '等待解析'}</h3>
        <p className="muted">解析后填充左侧表单，你确认金额、分类、账户和时间后再保存。</p>
      </div>
    </div>
  );
}
