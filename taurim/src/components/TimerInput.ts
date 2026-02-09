import { h } from 'preact';

interface TimerInputProps {
  value: number;
  onChange: (seconds: number) => void;
}

export function TimerInput({ value, onChange }: TimerInputProps) {
  const adjustTime = (delta: number) => {
    const newValue = Math.max(0, value + delta);
    onChange(newValue);
  };

  const adjustButtonClass = `
    px-4 py-2 rounded-lg text-sm font-medium
    bg-white/10 hover:bg-white/20 active:bg-white/30
    text-white border border-white/20
    transition-colors disabled:opacity-50
  `;

  return h('div', { className: 'flex flex-col items-center gap-3' },
    h('div', { className: 'flex gap-2' },
      h('button', {
        onClick: () => adjustTime(-600),
        disabled: value < 600,
        className: adjustButtonClass,
      }, '-10m'),
      h('button', {
        onClick: () => adjustTime(-60),
        disabled: value < 60,
        className: adjustButtonClass,
      }, '-1m'),
      h('button', {
        onClick: () => adjustTime(60),
        className: adjustButtonClass,
      }, '+1m'),
      h('button', {
        onClick: () => adjustTime(600),
        className: adjustButtonClass,
      }, '+10m')
    ),
    h('div', { className: 'flex gap-2' },
      h('button', {
        onClick: () => adjustTime(-10),
        disabled: value < 10,
        className: adjustButtonClass,
      }, '-10s'),
      h('button', {
        onClick: () => adjustTime(-1),
        disabled: value < 1,
        className: adjustButtonClass,
      }, '-1s'),
      h('button', {
        onClick: () => adjustTime(1),
        className: adjustButtonClass,
      }, '+1s'),
      h('button', {
        onClick: () => adjustTime(10),
        className: adjustButtonClass,
      }, '+10s')
    )
  );
}
