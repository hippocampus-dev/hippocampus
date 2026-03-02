import { h } from 'preact';

interface TimerControlsProps {
  isRunning: boolean;
  isCompleted: boolean;
  isAlarmPlaying: boolean;
  onStart: () => void;
  onPause: () => void;
  onReset: () => void;
  onStopAlarm: () => void;
}

export function TimerControls({
  isRunning,
  isCompleted,
  isAlarmPlaying,
  onStart,
  onPause,
  onReset,
  onStopAlarm,
}: TimerControlsProps) {
  const buttonBase = `
    px-8 py-3 rounded-full font-semibold text-lg
    transition-all duration-200 active:scale-95
  `;

  const primaryButton = `
    ${buttonBase}
    bg-white text-primary shadow-lg
    hover:shadow-xl
  `;

  const secondaryButton = `
    ${buttonBase}
    bg-white/20 text-white border border-white/30
    hover:bg-white/30
  `;

  // All timers completed - show Done (also stops alarm if playing)
  if (isCompleted) {
    return h('div', { className: 'flex gap-4 justify-center' },
      h('button', {
        onClick: onReset,
        className: primaryButton,
      }, 'Done')
    );
  }

  // Alarm playing (not last timer) - show Stop button
  if (isAlarmPlaying) {
    return h('div', { className: 'flex gap-4 justify-center' },
      h('button', {
        onClick: onStopAlarm,
        className: primaryButton,
      }, 'Stop')
    );
  }

  return h('div', { className: 'flex gap-4 justify-center' },
    isRunning
      ? h('button', {
          onClick: onPause,
          className: primaryButton,
        }, 'Pause')
      : h('button', {
          onClick: onStart,
          className: primaryButton,
        }, 'Start'),
    h('button', {
      onClick: onReset,
      className: secondaryButton,
    }, 'Reset')
  );
}
