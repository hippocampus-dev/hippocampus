import { h } from 'preact';
import { Timer, formatTime } from '../types/timer';
import { ProgressRing } from './ProgressRing';

interface TimerCardProps {
  timer: Timer;
  index: number;
  isRunning: boolean;
  onRemove?: (index: number) => void;
  showRemove?: boolean;
}

export function TimerCard({
  timer,
  index,
  isRunning,
  onRemove,
  showRemove = false,
}: TimerCardProps) {
  const progress = timer.durationSeconds > 0
    ? 1 - (timer.remainingSeconds / timer.durationSeconds)
    : 0;

  const statusColor = {
    idle: 'bg-white/10',
    running: 'bg-white/20',
    paused: 'bg-white/15',
    completed: 'bg-white/25',
  }[timer.status];

  return h('div', {
    className: `
      relative rounded-2xl p-6 backdrop-blur-sm
      ${statusColor}
      border border-white/20 shadow-xl
      transition-all duration-300
    `,
  },
    showRemove && h('button', {
      onClick: () => onRemove?.(index),
      className: `
        absolute top-2 right-2 w-8 h-8 rounded-full
        bg-red-500/50 hover:bg-red-500/70
        flex items-center justify-center
        text-white text-lg font-bold
        transition-colors
      `,
    }, '\u00d7'),
    timer.status === 'completed' && h('div', {
      className: `
        absolute top-2 left-2 w-8 h-8 rounded-full
        bg-green-500/70
        flex items-center justify-center
        text-white text-lg font-bold
      `,
    }, '\u2713'),
    h('div', { className: 'flex flex-col items-center justify-center py-8' },
      isRunning
        ? h('div', { className: 'relative' },
            h(ProgressRing, { progress, size: 160, strokeWidth: 6 }),
            h('div', {
              className: 'absolute inset-0 flex items-center justify-center',
            },
              h('span', {
                className: 'text-4xl font-mono font-bold text-white',
              }, formatTime(timer.remainingSeconds))
            )
          )
        : h('span', {
            className: 'text-5xl font-mono font-bold text-white',
          }, formatTime(timer.durationSeconds)),
      timer.label && h('span', {
        className: 'text-sm text-white/70 mt-2',
      }, timer.label)
    )
  );
}
